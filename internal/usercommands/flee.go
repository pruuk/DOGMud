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

func Flee(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

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

	// Fleeing costs stamina — a flyer breaks away far more easily (Winged Flight).
	//
	// Priced through costs.Calc like every other action rather than as the flat
	// int it used to be. FleeStaminaCost stays the BASE, so the knob keeps
	// meaning what it says, and the registry supplies the encumbrance and
	// inverse-skill terms. Before this, escaping under a crushing load cost
	// exactly what escaping empty-handed cost, which quietly undid the premise
	// that what you carry is a decision.
	bal := configs.GetBalanceConfig()
	spec := costs.SpecFor(costs.ActionFlee)
	fleeStaminaCost := costs.Calc(costs.Input{
		Base:      float64(bal.FleeStaminaCost),
		Carried:   user.Character.GetCarriedWeight(),
		Capacity:  user.Character.CarryCapacity(),
		Physical:  spec.Physical,
		SkillRank: user.Character.GetSkillLevel(spec.Skill),
		HasSkill:  spec.HasSkill,
	})
	if mutations.IsFlying(user.Character.Mutations) {
		// No "never below 1" clamp any more: the charge is fractional and its
		// remainder is banked, so a small cost is deferred rather than rounded
		// away, and clamping would have made a flyer's discount vanish at
		// exactly the light loads it is supposed to help.
		fleeStaminaCost *= float64(bal.FlightFleeStaminaMult)
	}
	// U5b-2: flee charges what it can and NEVER refuses. go.go refuses all
	// movement while in combat, so fleeing is the only player-initiated
	// disengage; refusing it at zero stamina would leave no alternative action
	// that changes the character's situation.
	//
	// ApplyCostFloat preserves that exactly: it banks the sub-1 remainder and
	// then delegates the whole part to ApplyCostPartial, which takes whatever is
	// in the pool and never refuses. It is the fractional form of the same
	// primitive, NOT the all-or-nothing one movement uses.
	//
	// The old "You're too exhausted to flee!" message is deleted rather than
	// left unreachable (standing rule 4). U8 reads CostResult.Short to strip the
	// skill term from fleeScore's skullduggery contribution; this chunk discards
	// it.
	_ = user.Character.ApplyCostFloat(characters.PoolStamina, fleeStaminaCost)

	user.SendText(messaging.CategorySystem, `You attempt to flee...`)

	// Task 15: use CombatPhase.TransitionToDisengaging instead of the legacy
	// Aggro{Type:Flee} sentinel. The round driver's handlePlayerFlee checks
	// IsDisengaging() which reads CombatPhase state directly.
	// Veto errors (e.g., grappled) are silently ignored here — the grapple check
	// in handlePlayerFlee catches them a moment later.
	// TODO Task 18: no legacy fallback needed; Aggro field is gone.
	if user.Character.CombatPhase != nil {
		_ = user.Character.CombatPhase.TransitionToDisengaging(state.TransitionReason{
			Trigger: combatphase.TriggerFleeCommand,
			Actor:   state.ActorRef{UserId: user.UserId},
		})
	}

	return true, nil
}
