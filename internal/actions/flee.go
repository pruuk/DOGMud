package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/targeting"
)

// FleeRefusal says why a flee did not begin.
type FleeRefusal int

const (
	FleeOK FleeRefusal = iota
	FleeRefuseRooted
	FleeRefuseNoFlee
	FleeRefuseAlready
	FleeRefuseNotInCombat
	FleeRefuseGrappled
	FleeRefuseProne
	FleeRefuseNotReady
)

// FleeBegin reports one flee command. Accepted means the fleer is now
// Disengaging and the round will resolve the escape; Short means it could
// not pay in full and brings no Skullduggery to the blocker contest.
type FleeBegin struct {
	Accepted bool
	Short    bool
	Refusal  FleeRefusal
}

// FleeGate is every refusal a flee command can know before it transitions,
// in the order the player has always been told them. It is exported so a
// behaviour-tree action can decide to fire instead of kiting when a flee
// could not begin. Grapple and standing duplicate CombatPhase's position veto
// on purpose: the veto is registered by the hooks package, so without these
// two checks a flee's outcome would depend on which packages are linked.
func FleeGate(c *characters.Character) FleeRefusal {
	switch {
	case c.HasConditionFlag(conditions.NoMovement):
		return FleeRefuseRooted
	case c.HasConditionFlag(conditions.NoFlee):
		return FleeRefuseNoFlee
	case c.IsDisengaging():
		return FleeRefuseAlready
	case !c.IsInCombat():
		return FleeRefuseNotInCombat
	case c.IsStandingGrapple() || c.IsGroundGrapple():
		return FleeRefuseGrappled
	case !c.IsStanding():
		return FleeRefuseProne
	}
	return FleeOK
}

// BeginFlee is the flee command, shared by players and mobs: the gates, the
// pending admission, the Disengaging transition, then one quote and partial
// commit of the flee cost. Shortage never refuses a flee (it is the only way
// out of a fight); it drops Skullduggery from the contest instead. The escape
// itself happens on the next round, in ResolveFlee. preferredExit is carried
// to that round; "" means a random passable exit.
func BeginFlee(actor Actor, preferredExit string) FleeBegin {
	c := actor.GetCharacter()

	// Any command that is not the attempt already in flight owns no pending
	// admission, so retract an orphan before any refusal returns.
	if !c.IsDisengaging() {
		c.CancelFleeAdmission()
	}
	if r := FleeGate(c); r != FleeOK {
		return FleeBegin{Refusal: r}
	}

	// Publish a pending handoff before the transition. Cost belongs only to an
	// accepted Disengaging transition.
	c.PublishFleeAdmission(characters.FleeAdmission{PreferredExit: preferredExit})
	if c.CombatPhase == nil {
		c.CancelFleeAdmission()
		return FleeBegin{Refusal: FleeRefuseNotReady}
	}
	if err := c.CombatPhase.TransitionToDisengaging(state.TransitionReason{
		Trigger: combatphase.TriggerFleeCommand,
		Actor:   state.ActorRef{UserId: actor.GetUserId(), MobInstanceId: actor.GetMobInstanceId()},
	}); err != nil {
		c.CancelFleeAdmission()
		return FleeBegin{Refusal: fleeVetoReason(c)}
	}

	bal := configs.GetBalanceConfig()
	modifier := 1.0
	if mutations.IsFlying(c.Mutations) {
		modifier = float64(bal.FlightFleeStaminaMult)
	}
	quote := c.QuoteActionCost(characters.ActionCostRequest{
		Action:   costs.ActionFlee,
		Pool:     characters.PoolStamina,
		Base:     float64(bal.FleeStaminaCost),
		Modifier: modifier,
		Units:    1,
	})
	short := c.CommitCost(quote, characters.CostPartial).Short()
	c.PublishFleeAdmission(characters.FleeAdmission{
		IncludeSkill:  !short,
		Ready:         true,
		PreferredExit: preferredExit,
	})
	return FleeBegin{Accepted: true, Short: short}
}

// fleeVetoReason names a transition the machine refused after FleeGate let it
// through (a veto registered elsewhere, or combat ending in between).
func fleeVetoReason(c *characters.Character) FleeRefusal {
	switch {
	case !c.IsInCombat():
		return FleeRefuseNotInCombat
	case c.IsStandingGrapple() || c.IsGroundGrapple():
		return FleeRefuseGrappled
	case !c.IsStanding():
		return FleeRefuseProne
	}
	return FleeRefuseNotReady
}

// FleeOutcome reports one flee round. Fleeing is false when the actor was not
// Disengaging, so the round goes on as normal. Fleeing with Resolved false
// means another resolver already owns this attempt: skip the round and say
// nothing. A resolved flee ends one of four ways: Grappled, blocked (Blocker
// set), NoExit, or escaped through ExitName to ExitRoomId.
type FleeOutcome struct {
	Fleeing    bool
	Resolved   bool
	Grappled   bool
	Blocker    *combat.FleeBlocker
	NoExit     bool
	ExitName   string
	ExitRoomId int
}

// Escaped reports a flee that got away.
func (o FleeOutcome) Escaped() bool {
	return o.Resolved && !o.Grappled && o.Blocker == nil && !o.NoExit
}

// ResolveFlee is the flee round, shared by players and mobs: consume the
// admission, refuse a fleer grappled since the command, run the blocker
// contest, practise Skullduggery only when a contest happened AND the fleer
// paid in full, pick the exit, and settle CombatPhase (back to Engaged on any
// failure, Idle on success). It does not move the fleer; the wrapper does,
// because the player and mob moves differ (Look, charmed followers and room
// entry events for one; RelocateMob and mob_flee for the other).
func ResolveFlee(actor Actor, room *rooms.Room) FleeOutcome {
	c := actor.GetCharacter()
	if !c.IsDisengaging() {
		// A terminal transition can end Disengaging before this round runs.
		// Retract that orphan; an absent handoff is a harmless no-op.
		c.TakeFleeAdmission()
		return FleeOutcome{}
	}
	admission, admitted := c.TakeFleeAdmission()
	if !admitted {
		return FleeOutcome{Fleeing: true}
	}
	out := FleeOutcome{Fleeing: true, Resolved: true}

	if c.IsStandingGrapple() || c.IsGroundGrapple() {
		out.Grappled = true
		settleFlee(c, false)
		return out
	}

	blocker, contested := combat.ResolveFleeBlockers(c, room, admission.IncludeSkill)
	// Practise only when an opposed roll happened and the fleer brought the
	// skill to it (a short payment leaves it out). Won is "got away".
	if contested && admission.IncludeSkill {
		actor.AwardResolved(blocker == nil, c.CandidateFor(string(skills.Skullduggery)))
	}
	if blocker != nil {
		out.Blocker = blocker
		settleFlee(c, false)
		return out
	}

	exitName, exitRoomId := fleeExit(room, admission.PreferredExit, actor.GetUserId())
	if exitName == `` {
		out.NoExit = true
		settleFlee(c, false)
		return out
	}
	out.ExitName, out.ExitRoomId = exitName, exitRoomId

	targeting.Release(c, targeting.ReasonDisengage)
	settleFlee(c, true)
	return out
}

func settleFlee(c *characters.Character, success bool) {
	if c.CombatPhase != nil {
		c.CombatPhase.ResolveFlee(success)
	}
}

// fleeExit is the preferred exit when it is still there and passable, else a
// random passable one (secret and locked exits excluded). A routed exit (a
// housing or rift door) is passable only when its router lets this fleer
// through, and then leads where the router says: fleeing is not a way past a
// door that walking could not open. userId is zero for a mob.
func fleeExit(room *rooms.Room, preferred string, userId int) (exitName string, roomId int) {
	rooms.WhileFleeing(func() { exitName, roomId = fleeExitRouted(room, preferred, userId) })
	return exitName, roomId
}

// fleeExitRouted is fleeExit's choice, made inside rooms.WhileFleeing.
func fleeExitRouted(room *rooms.Room, preferred string, userId int) (string, int) {
	if preferred != `` {
		if info, ok := room.GetExitInfo(preferred); ok && !info.Lock.IsLocked() {
			if route, routed := rooms.RouteExit(userId, room.RoomId, preferred); !routed {
				return preferred, info.RoomId
			} else if route.RoomId != 0 {
				return preferred, route.RoomId
			}
		}
	}
	return room.GetRandomExitFor(userId)
}
