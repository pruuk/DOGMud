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
}

// TakeFleeAdmission atomically consumes the pending command's admission.
// The second result distinguishes a real command handoff from missing or
// already-consumed state; the round hook uses that to reject reentrant phase
// resolution while preserving true legacy fallback behavior.
func TakeFleeAdmission(user *users.UserRecord) (includeSkill bool, ok bool) {
	if user == nil {
		return false, false
	}
	admission, ok := user.TakeTempData(fleeIncludeSkillTempKey).(fleeAdmission)
	if !ok {
		return false, false
	}
	return admission.includeSkill, true
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
	if costResult.Short() {
		user.SendText(messaging.CategorySystem, fleeShortageText)
	}
	// Publish before the transition so even synchronous/reentrant transition
	// observers see the attempt-scoped admission. A veto atomically retracts it.
	user.SetTempData(fleeIncludeSkillTempKey, fleeAdmission{
		includeSkill: !costResult.Short(),
	})

	user.SendText(messaging.CategorySystem, `You attempt to flee...`)

	// Task 15: use CombatPhase.TransitionToDisengaging instead of the legacy
	// Aggro{Type:Flee} sentinel. The round driver's handlePlayerFlee checks
	// IsDisengaging() which reads CombatPhase state directly.
	// A veto retracts the handoff because no round resolver owns the rejected
	// transition.
	// TODO Task 18: no legacy fallback needed; Aggro field is gone.
	if user.Character.CombatPhase != nil {
		if err := user.Character.CombatPhase.TransitionToDisengaging(state.TransitionReason{
			Trigger: combatphase.TriggerFleeCommand,
			Actor:   state.ActorRef{UserId: user.UserId},
		}); err != nil {
			user.TakeTempData(fleeIncludeSkillTempKey)
		}
	} else {
		user.TakeTempData(fleeIncludeSkillTempKey)
	}

	return true, nil
}
