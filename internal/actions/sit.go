package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// Sitting (#637) is a real position, position.Sitting: `sit` and `rest` are
// one command, shared by players and mobs. A sitter regenerates faster
// (Balance.SitRegenMultiplier, below sleep), stands with `stand` at no cost,
// and is stood up by combat (Character.RiseForCombat, narrated by
// NarrateCombatRise) or by walking off (StandFromSitting in the go wrappers).

// Refusal lines the sit command sends a player. A mob actor's SendText does
// nothing, so a mob is refused silently.
const (
	SitRefuseAsleep     = `You're asleep. Type stand to wake up.`
	SitRefuseSitting    = `You're already sitting.`
	SitRefuseGrappled   = `You're locked in a grapple. You'll need to break free first.`
	SitRefuseOnTheFloor = `You're on the ground. Stand up first.`
)

// Sit seats actor: Standing to Sitting, with the actor's line and a sight-aware
// room line. Returns whether the actor sat. Refused while asleep, already
// sitting, fighting, grappled, or down on the floor.
func Sit(actor Actor) bool {
	c := actor.GetCharacter()
	if c == nil || c.Position == nil {
		return false
	}
	switch {
	case c.HasConditionFlag(conditions.Sleeping):
		actor.SendText(messaging.CategoryRefusal, SitRefuseAsleep)
		return false
	case c.IsSitting():
		actor.SendText(messaging.CategoryRefusal, SitRefuseSitting)
		return false
	case c.IsInCombat():
		refusal.WhileFighting(actor, "sit")
		return false
	case c.IsGrappling():
		actor.SendText(messaging.CategoryRefusal, SitRefuseGrappled)
		return false
	case !c.IsStanding():
		actor.SendText(messaging.CategoryRefusal, SitRefuseOnTheFloor)
		return false
	}
	if err := c.Position.TransitionToSitting(state.TransitionReason{
		Trigger: position.TriggerSitCommand,
		Actor:   ActorRefOf(actor),
	}); err != nil {
		refusal.Failed(actor, "sit", err, "userId", actor.GetUserId(), "mobInstanceId", actor.GetMobInstanceId())
		return false
	}
	narratePosture(actor, `You sit down.`, `%s sits down.`)
	return true
}

// StandFromSitting stands a sitter for free: the stand command's Sitting
// branch, and a sitter who walks off. Returns false, saying nothing, when the
// actor is not sitting.
func StandFromSitting(actor Actor) bool {
	c := actor.GetCharacter()
	if c == nil || c.Position == nil || !c.IsSitting() {
		return false
	}
	if err := c.Position.TransitionToStanding(state.TransitionReason{
		Trigger: position.TriggerStandCommand,
		Actor:   ActorRefOf(actor),
	}); err != nil {
		mudlog.Warn("StandFromSitting: TransitionToStanding failed", "userId", actor.GetUserId(),
			"mobInstanceId", actor.GetMobInstanceId(), "err", err)
		return false
	}
	narratePosture(actor, `You stand up.`, `%s stands up.`)
	return true
}

// NarrateCombatRise tells the room a sitter leapt up because a fight reached
// them. The transition itself is Character.RiseForCombat; the hooks package
// calls this from its Position observer on position.TriggerCombatRise.
func NarrateCombatRise(actor Actor) {
	narratePosture(actor, `You leap to your feet!`, `%s leaps up!`)
}

// narratePosture sends one posture change: the actor's own line, and the room
// line with the actor's name hidden from a reader who cannot see them
// (messaging.SendTrio). observerFmt takes the tagged name once.
func narratePosture(actor Actor, actorLine, observerFmt string) {
	nameTag := "mobname"
	if actor.IsPlayer() {
		nameTag = "username"
	}
	trio := messaging.Trio{
		Actor: messaging.Say(messaging.CategorySystem, actorLine),
		// Nobody is acted upon: a posture change is the actor's alone.
		Actee: messaging.NoLine,
		Observer: messaging.Say(messaging.CategoryMobEmote, fmt.Sprintf(observerFmt,
			fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, nameTag, actor.GetName()))),
	}
	aud := messaging.Audience{
		Actor:     actor,
		ActorId:   actor.GetUserId(),
		ActorName: actor.GetName(),
		ActeeName: messaging.NoName,
	}
	// A nil *rooms.Room must not reach the Broadcaster interface as a typed
	// nil, so the room is assigned only when there is one.
	if room := actor.GetRoom(); room != nil {
		aud.Room = room
	}
	messaging.SendTrio(trio, aud)
}
