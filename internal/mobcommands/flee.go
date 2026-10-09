package mobcommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

// Flee starts a mob's flee through the player's rules (actions.BeginFlee):
// it must be fighting, standing, unrooted and not frenzied, it pays the flee
// cost, and it enters Disengaging. The escape resolves on the next round in
// hooks.handleMobFlee. rest, when given, is the exit the mob would rather
// take (a kiting archer passes the exit toward home).
func Flee(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	// Non-combatant mobs never flee (they should never be in combat).
	if mob.IsNonCombatant() {
		return true, nil
	}

	// A flee breaks a fold-cast when there is a live combat to flee, before
	// the flee gates run, exactly as the player's fold-casting intercept in
	// usercommands.go does. Without this the mob finishes its spell first,
	// because handleMobFoldCasting runs ahead of handleMobFlee. An
	// out-of-combat flee is refused and must not destroy the cast.
	if mob.Character.Activity != nil && mob.Character.Activity.IsCasting() && mob.Character.IsInCombat() {
		_ = mob.Character.Activity.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerCastCancel,
			Actor:   state.ActorRef{MobInstanceId: mob.InstanceId},
		})
		// Seen by sight, heard by a reader who sees nothing, in the shared
		// spell-disruption wording (#242, owner ruling R4).
		// The mob is named as the hooks package's mob room lines name it:
		// with its duplicate number ("Skeleton 2"), and unnamed when hidden.
		subject := mob.Character.GetMobNameIndexed(0, room.GetMobDuplicateIndex(mob.InstanceId)).String()
		if mob.Character.IsHidden() {
			subject = messaging.HideNames(`<ansi fg="mobname">`+mob.Character.Name+`</ansi>`,
				[]string{mob.Character.Name}, messaging.SightNone)
		}
		room.SendTextVisualWithAudio(messaging.CategorySpellDisruption,
			subject+"'s concentration breaks.", messaging.SoundChantBreaksOff)
	}

	begin := actions.BeginFlee(actions.NewMobActorInRoom(mob, room), strings.TrimSpace(rest))

	// Every refusal is silent for a mob except the grapple: the player holding
	// it needs to see the hold working.
	if begin.Refusal == actions.FleeRefuseGrappled {
		room.SendTextVisual(messaging.CategoryGrappleFlow,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> tries to break free but you've got them locked down!`, mob.Character.Name))
	}
	return true, nil
}
