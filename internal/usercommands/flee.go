package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	fleeIncludeSkillTempKey = "flee-include-skill"
	fleeShortageText        = "You break away on instinct rather than technique, too spent to use your training."
)

type fleeAdmission struct {
	includeSkill bool
	ready        bool
}

// TakeFleeAdmission atomically consumes the pending command's admission.
// The second result distinguishes a real command handoff from missing or
// already-consumed state; the round hook uses that to reject reentrant phase
// resolution while preserving true legacy fallback behavior.
func TakeFleeAdmission(user *users.UserRecord) (includeSkill bool, ok bool) {
	if user == nil {
		return false, false
	}
	// The command publishes a pending marker before asking CombatPhase to
	// transition. A round that observes Disengaging in that tiny handoff window
	// must leave the marker for the command to finish instead of consuming an
	// attempt whose cost decision is not ready yet.
	peek, ok := user.GetTempData(fleeIncludeSkillTempKey).(fleeAdmission)
	if !ok || !peek.ready {
		return false, false
	}
	admission, ok := user.TakeTempData(fleeIncludeSkillTempKey).(fleeAdmission)
	if !ok || !admission.ready {
		return false, false
	}
	return admission.includeSkill, true
}

// CancelFleeAdmission retracts either a pending or ready handoff. CombatPhase
// terminal-transition hooks use it when combat ends after the command was
// admitted but before the flee round can resolve.
func CancelFleeAdmission(user *users.UserRecord) bool {
	if user == nil {
		return false
	}
	_, ok := user.TakeTempData(fleeIncludeSkillTempKey).(fleeAdmission)
	return ok
}

func Flee(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	// Any command that is not the already-active attempt owns no pending
	// admission yet, so retract an orphan before any rejection path returns.
	// Do not clear the handoff for a genuine attempt still awaiting its round.
	if !user.Character.IsDisengaging() {
		user.SetTempData(fleeIncludeSkillTempKey, nil)
	}

	// A no-go root (e.g. a Jailed holding-cell buff — 5.1c) pins the player in
	// place; flee must honor it too, or it becomes a jail-escape hole (the
	// directional `go` block alone is bypassable via flee — smoke BUG-02).
	if user.Character.HasBuffFlag(buffs.NoMovement) {
		user.SendText(messaging.CategorySystem, `You're locked in — there's nowhere to flee to.`)
		return true, nil
	}

	// A no-flee state (Blood Frenzy, hamstrung, winded, tackled, …) forbids
	// retreat: you can still move and fight, but you can't break off to flee.
	if user.Character.HasBuffFlag(buffs.NoFlee) {
		user.SendText(messaging.CategorySystem, `You can't break off to flee right now — you can only fight.`)
		return true, nil
	}

	// A second flee while the first is still resolving used to print nothing at
	// all: the whole block below is skipped, so the player saw "You attempt to
	// flee..." once and then silence, and read the silence as the command being
	// swallowed. Say so instead.
	if user.Character.IsDisengaging() {
		user.SendText(messaging.CategorySystem, `You're already trying to break away. Give it a moment.`)
		return true, nil
	}
	// Input is accepted before command events are drained. A lethal combat
	// round can therefore kill, respawn, and force CombatPhase to Idle before a
	// queued flee reaches this handler. Reject that stale command before it
	// spends the revived character's stamina or publishes an attempt that no
	// round resolver can finish. This also gives ordinary out-of-combat use a
	// definitive response instead of a paid, outcome-less attempt.
	if !user.Character.IsInCombat() {
		user.SendText(messaging.CategorySystem, `You're not in combat; there's nothing to flee from.`)
		return true, nil
	}
	// Publish a pending handoff before the state transition. Cost and player-
	// facing attempt text belong only to an accepted Disengaging transition;
	// charging first lets a target-death or position veto create a paid attempt
	// that no round resolver can finish.
	user.SetTempData(fleeIncludeSkillTempKey, fleeAdmission{})
	if user.Character.CombatPhase == nil {
		CancelFleeAdmission(user)
		user.SendText(messaging.CategorySystem, `You can't break away just yet.`)
		return true, nil
	}
	if err := user.Character.CombatPhase.TransitionToDisengaging(state.TransitionReason{
		Trigger: combatphase.TriggerFleeCommand,
		Actor:   state.ActorRef{UserId: user.UserId},
	}); err != nil {
		CancelFleeAdmission(user)
		if !user.Character.IsInCombat() {
			user.SendText(messaging.CategorySystem, `You're not in combat; there's nothing to flee from.`)
		} else if user.Character.IsStandingGrapple() || user.Character.IsGroundGrapple() {
			user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't flee while grappled!</ansi>`)
		} else if !user.Character.IsStanding() {
			// The flee veto is IsStanding(), NOT grapple. Being knocked down
			// refuses a flee exactly as a grapple does, and until now it fell
			// through to the generic line below, which reads like a timing
			// problem. Knockdown is common (trips, bashes, sweeps, kicks and
			// double fumbles all cause it) and it is precisely when a player
			// most wants to run, so the one thing they need to know is that
			// standing up is what unblocks it.
			user.SendText(messaging.CategorySystem, `<ansi fg="red">You can't flee from the ground. Stand up first!</ansi>`)
		} else {
			user.SendText(messaging.CategorySystem, `You can't break away just yet.`)
		}
		return true, nil
	}

	// Quote and partially commit once. Flee remains life-preserving: shortage
	// never refuses the attempt, but its blocker contests lose Skullduggery.
	bal := configs.GetBalanceConfig()
	modifier := 1.0
	if mutations.IsFlying(user.Character.Mutations) {
		modifier = float64(bal.FlightFleeStaminaMult)
	}
	quote := user.Character.QuoteActionCost(characters.ActionCostRequest{
		Action:   costs.ActionFlee,
		Pool:     characters.PoolStamina,
		Base:     float64(bal.FleeStaminaCost),
		Modifier: modifier,
		Units:    1,
	})
	costResult := user.Character.CommitCost(quote, characters.CostPartial)
	user.SetTempData(fleeIncludeSkillTempKey, fleeAdmission{
		includeSkill: !costResult.Short(),
		ready:        true,
	})
	if costResult.Short() {
		user.SendText(messaging.CategorySystem, fleeShortageText)
	}

	user.SendText(messaging.CategorySystem, `You attempt to flee...`)

	return true, nil
}
