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
// NarrateCombatRise) or, silently, by any room move
// (Character.StandForMove in rooms.MoveToRoom and Room.AddMob).

// Refusal lines the sit command sends a player. A mob actor's SendText does
// nothing, so a mob is refused silently.
const (
	// SitRefuseAsleep: `wake` is a command alias of noop, so typing it runs
	// the player dispatcher's cancel-on-action wake. sit and rest are exempt
	// from that wake (usercommands.keepsSleep), or this line could never be
	// reached by a player (owner ruling 2026-10-19).
	SitRefuseAsleep     = `You're asleep. Type 'wake' to wake up.`
	SitRefuseSitting    = `You're already sitting.`
	SitRefuseGrappled   = `You're locked in a grapple. You'll need to break free first.`
	SitRefuseOnTheFloor = `You're on the ground. Stand up first.`
)

// HiddenPostureNote tells a hidden actor that their sit or stand reached no
// one, the posture twin of the emote command's note: the room line goes
// through SendSeen, which is silent for a hidden actor (#274, owner R3).
const HiddenPostureNote = `No one sees it; you are hidden.`

// Sit seats actor: Standing to Sitting, with the actor's line and a sight-aware
// room line. Returns whether the actor sat. Refused, players and mobs alike,
// while asleep (sit does not wake a sleeper, owner ruling 2026-10-19), busy
// (casting, crafting, salvaging), already sitting, fighting, grappled, or
// down on the floor.
func Sit(actor Actor) bool {
	c := actor.GetCharacter()
	if c == nil || c.Position == nil {
		return false
	}
	switch {
	case c.HasConditionFlag(conditions.Sleeping):
		actor.SendText(messaging.CategoryRefusal, SitRefuseAsleep)
		return false
	case c.IsActing():
		refusal.WhileBusy(actor, "sit")
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
	narratePosture(actor, `You sit down.`, `%s sits down.`, true)
	return true
}

// StandFromSitting stands a sitter for free, narrated: the Sitting branch of
// both stand commands. Returns false, saying nothing, when the actor is not
// sitting. A room move stands a sitter silently instead (StandForMove).
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
	narratePosture(actor, `You stand up.`, `%s stands up.`, true)
	return true
}

// NarrateCombatRise tells the room a sitter leapt up because a fight reached
// them. The transition itself is Character.RiseForCombat; the hooks package
// calls this from its Position observer on position.TriggerCombatRise. A
// sitter still hidden when it rises (an area attack does not reveal) leaps up
// unseen, with no note: the fight is the news, not the posture.
func NarrateCombatRise(actor Actor) {
	narratePosture(actor, `You leap to your feet!`, `%s leaps up!`, false)
}

// narratePosture sends one posture change: the actor's own line, and the room
// line through SendSeen, which hides the name from a reader who cannot see the
// actor and sends nothing at all for a hidden actor. hiddenNote adds
// HiddenPostureNote for a hidden actor so the silence is not a mystery.
// observerFmt takes the tagged name once.
func narratePosture(actor Actor, actorLine, observerFmt string, hiddenNote bool) {
	nameTag := "mobname"
	if actor.IsPlayer() {
		nameTag = "username"
	}
	actor.SendText(messaging.CategorySystem, actorLine)
	SendSeen(actor, messaging.CategoryMobEmote, fmt.Sprintf(observerFmt,
		fmt.Sprintf(`<ansi fg="%s">%s</ansi>`, nameTag, actor.GetName())), false)
	if c := actor.GetCharacter(); hiddenNote && c != nil && c.IsHidden() {
		actor.SendText(messaging.CategorySystem, HiddenPostureNote)
	}
}
