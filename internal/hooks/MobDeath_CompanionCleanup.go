package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// CompanionCleanup removes a dead companion from the owner's Companions list
// and notifies the player when their companion falls in battle.
func CompanionCleanup(e events.Event) events.ListenerReturn {

	evt, ok := e.(events.MobDeath)
	if !ok {
		return events.Continue
	}

	// Check if the dead mob's instance was a companion before it was destroyed.
	// We rely on the CharmedMobs tracking on the player character to find the owner.
	allUsers := users.GetAllActiveUsers()
	for _, user := range allUsers {
		comp := user.Character.GetCompanionByInstanceId(evt.InstanceId)
		if comp == nil {
			continue
		}

		// We found the owner. Remove the companion from the list.
		companionName := comp.Name
		if companionName == "" {
			if spec := mobs.GetMobSpec(mobs.MobId(comp.MobId)); spec != nil {
				companionName = spec.Character.Name
			} else {
				companionName = "companion"
			}
		}

		// Chrysifier: a fallen homunculus reforges only after a delay — start the
		// respawn cooldown now (tickHomunculus reads it), before removal.
		if comp.MobId == homunculusMobId {
			user.Character.TryCooldown("homunculus-respawn", "10 rounds")
		}

		user.Character.RemoveCompanion(evt.InstanceId)
		user.Character.TrackCharmed(evt.InstanceId, false)

		// Releasing the record is not releasing the reservation. A fielded
		// companion holds a slice of its owner's Conviction, that slice is
		// DERIVED from the live companion list during RecalculateStats, and
		// Char.Vitals carries the figure to the web client as a PUSH-ONLY
		// snapshot -- so without the event the client keeps showing a phantom
		// reservation for a companion that just died. The prompt bar and
		// `status` never show it because both read GetPoolReservation live.
		//
		// This is a three-site invariant and death was the site that missed it:
		// dismiss.go has publishReleasedReservation, and the charm-expiry path in
		// NewRound_MobRoundTick.go makes the same two calls. The regen tick cannot
		// cover for any of them -- NewRound_AutoHeal republishes only when health
		// actually moved OR the player still has a companion, so losing your LAST
		// companion at full health is precisely the case that never self-corrects.
		user.Character.RecalculateStats()
		events.AddToQueue(events.CharacterVitalsChanged{UserId: user.UserId})

		user.SendText(messaging.CategoryDeath, fmt.Sprintf(
			`<ansi fg="red">Your %s has fallen.</ansi>`,
			companionName,
		))

		break
	}

	return events.Continue
}
