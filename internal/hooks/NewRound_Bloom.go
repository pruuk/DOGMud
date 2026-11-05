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
// Only time online counts (owner 2026-11-04): a player must not be able to log
// out and wait off a craving or a withdrawal. BloomAbstinentRounds counts the
// rounds spent online since the last dose. This hook is the only thing that ticks
// it, a dose resets it, and it is saved, so a relog neither stops it nor moves it.
// A save with addiction and no count reads as 0: the count starts at the first
// round online, with no false withdrawal, and cannot get stuck.

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// BloomTick is the NewRound listener that drives the Bloom drug lifecycle.
// Registered in hooks.go after AutoHeal.
func BloomTick(e events.Event) events.ListenerReturn {
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

		// This loop covers online users only, so this is the one place the count
		// moves: a round spent logged out is not a round of abstinence.
		c.BloomAbstinentRounds++

		// ── 2. Addiction decay ────────────────────────────────────────────────
		// One addiction point per BloomAddictionDecayRounds of abstinence.
		// After each decrement, take one period off the count so the next
		// decrement fires exactly BloomAddictionDecayRounds later.
		if decayRounds > 0 && c.BloomAbstinentRounds >= decayRounds {
			c.AddBloomAddiction(-1)
			c.BloomAbstinentRounds -= decayRounds
			if c.BloomAddiction == 0 {
				// Fully clean: remove any lingering Withdrawal condition.
				actions.LandBloomClean(actions.NewUserActor(user))
			}
		}

		// ── 3. Withdrawal ─────────────────────────────────────────────────────
		if c.BloomAddiction > 0 && withdrawOnset > 0 && c.BloomAbstinentRounds >= withdrawOnset {
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
