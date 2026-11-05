package hooks

// NewRound_Bloom.go — per-round Bloom drug lifecycle for online players.
//
// Three duties per tick:
//  1. Crash onset:    Communion (condition 90) just expired → apply Crash (condition 91).
//  2. Addiction decay: one addiction point removed per BloomAddictionDecayRounds
//                     of abstinence; fully clean players have condition 92 cleared.
//  3. Withdrawal:     addict has abstained past BloomWithdrawalOnsetRounds →
//                     ensure Withdrawal (condition 92) is active; message on first apply.
//
// The Communion→Crash transition uses Character.BloomHadCommunion: true while
// condition 90 is on, and cleared the round it goes away. It is saved, so a
// Communion that ends on the first round after a login still crashes.
//
// BloomLastDoseRound is saved too (owner 2026-11-04): a relog must not stop
// withdrawal. util.SaveRoundCount keeps the round count across a restart, so a
// saved round stays meaningful, and rounds run while a player is away, so time
// away counts as abstinence. A save from before the round was stored, or a stamp
// ahead of the clock (a lost round file), starts its clock at the first round
// online: no false withdrawal, and never stuck with no clock at all.

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// BloomTick is the NewRound listener that drives the Bloom drug lifecycle.
// Registered in hooks.go after AutoHeal.
func BloomTick(e events.Event) events.ListenerReturn {
	evt := e.(events.NewRound)
	currentRound := evt.RoundNumber

	bal := configs.GetBalanceConfig()
	withdrawOnset := uint64(bal.BloomWithdrawalOnsetRounds)
	decayRounds := uint64(bal.BloomAddictionDecayRounds)

	for _, userId := range users.GetOnlineUserIds() {
		user := users.GetByUserId(userId)
		if user == nil {
			continue
		}
		// user.Character is *characters.Character — use directly.
		c := user.Character

		hasCommunion := c.HasCondition(90)

		// ── 1. Crash on Communion end ─────────────────────────────────────────
		// BloomHadCommunion was set to true last tick while condition 90 was active.
		// The moment it goes absent, apply the Crash and emit a grim message.
		// Scale: condition 91 baseline triggercount (75) * BloomCrashRoundsMult (default 2.5)
		// = 187 rounds of crash at default config.  Adjust both knobs together to
		// change the high:crash ratio without touching condition YAML.
		// Through the user record: condition 91 carries its own authored start line
		// ("The communion ends...") and only the event reaches the notice, so
		// applying it on the character left the crash unannounced. The
		// hand-rolled warning that used to sit here said the same thing in
		// different words, so it goes rather than double up on the authored one.
		if c.BloomHadCommunion && !hasCommunion {
			user.AddConditionScaled(91, float64(bal.BloomCrashRoundsMult), "bloom")
		}
		// Always mirror the current state for the next tick's detection.
		c.BloomHadCommunion = hasCommunion

		if c.BloomAddiction == 0 {
			continue
		}

		// An addict with no usable clock: a save from before the round was stored,
		// or a stamp ahead of the count, which the subtractions below would wrap
		// into a huge abstinence. The clock starts now.
		if c.BloomLastDoseRound == 0 || c.BloomLastDoseRound > currentRound {
			c.BloomLastDoseRound = currentRound
		}

		// ── 2. Addiction decay ────────────────────────────────────────────────
		// One addiction point per BloomAddictionDecayRounds of abstinence.
		// After each decrement, advance BloomLastDoseRound by one period so
		// the next decrement fires exactly BloomAddictionDecayRounds later. A
		// player back from a long absence settles it all in one tick, before the
		// withdrawal check, so someone clean by now is not sent into withdrawal
		// for one round first.
		for decayRounds > 0 && c.BloomAddiction > 0 && currentRound-c.BloomLastDoseRound >= decayRounds {
			c.AddBloomAddiction(-1)
			// Slide the reference point forward one period.
			c.BloomLastDoseRound += decayRounds
			if c.BloomAddiction == 0 {
				// Fully clean: remove any lingering Withdrawal condition.
				actions.LandBloomClean(actions.NewUserActor(user))
			}
		}

		// ── 3. Withdrawal ─────────────────────────────────────────────────────
		if c.BloomAddiction > 0 && withdrawOnset > 0 && currentRound-c.BloomLastDoseRound >= withdrawOnset {
			// Re-apply only when the condition has expired to avoid constant
			// timer resets that would make withdrawal permanent.
			// Through the user record, for the same reason as the crash
			// above: condition 92's authored start line is the one door, and the
			// duplicate warning that used to follow this call is gone.
			if !c.HasCondition(92) {
				user.AddConditionScaled(92, 1.0, "bloom") // baseline 200 rounds
			}
		}
	}

	return events.Continue
}
