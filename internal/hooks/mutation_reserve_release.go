package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// releaseCompanionsOverCeiling drops companion records, newest first, to
// cover the body's GROWTH since it was last acknowledged (#680, ruling 6).
// Character.MutationReserveHeld marks the units already acknowledged.
// When the owned units have not grown past it (a login re-price, a zero-unit
// gain, a scour), nothing is released and the mark follows the units down.
// When they have, only the overage the growth itself added is released: the
// overage that was there before (gear, re-priced bonds) stays grandfathered
// (D4). A save from before the rule holds 0, so its first check treats the
// whole body share as new growth. Bonded companions hold no reserve and are
// never released. Gear is never removed. Returns the records it removed.
//
// It reads the pool maximum as it stands and does not recalculate stats; the
// caller (releaseCompanionsForUser) does that first.
func releaseCompanionsOverCeiling(ch *characters.Character) []characters.CompanionInfo {
	units := mutations.OwnedReserveUnits(ch.Mutations)
	held := ch.MutationReserveHeld
	ch.MutationReserveHeld = units
	if units <= held {
		return nil
	}
	increase := ch.MutationReservePoints(units) - ch.MutationReservePoints(held)
	grandfathered := ch.ReservationOverages().Conviction - increase
	if grandfathered < 0 {
		grandfathered = 0
	}
	var released []characters.CompanionInfo
	for ch.ReservationOverages().Conviction > grandfathered {
		idx := -1
		for i := len(ch.Companions) - 1; i >= 0; i-- {
			if ch.Companions[i].ConvictionReserve > 0 {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		comp := ch.Companions[idx]
		ch.Companions = append(ch.Companions[:idx], ch.Companions[idx+1:]...)
		released = append(released, comp)
	}
	return released
}

// releaseCompanionsForUser runs the release for a live player and makes each
// one real: a charmed wild creature leaves peacefully (no grudge: the owner's
// body changed, nobody betrayed anyone), a summoned, raised or conjured one
// dissolves, the player is told, and the pool is republished.
//
// The records are already gone when the mobs are touched, which is the order
// dismiss.go and tickMobCharmState keep: the companion record goes before
// anything that can trigger room-wide combat logic. At login the companions
// are not yet spawned (InstanceId is 0), so only the records are dropped.
func releaseCompanionsForUser(user *users.UserRecord) {
	user.Character.RecalculateStats()
	released := releaseCompanionsOverCeiling(user.Character)
	if len(released) == 0 {
		return
	}
	for _, comp := range released {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`Your changing body claims more of your will. <ansi fg="mobname">%s</ansi> slips its bond and leaves you.`,
			comp.Name))
		if comp.InstanceId == 0 {
			continue
		}
		user.Character.TrackCharmed(comp.InstanceId, false)
		mob := mobs.GetInstance(comp.InstanceId)
		if mob == nil {
			continue
		}
		cmd := ``
		if mob.Character.Charmed != nil {
			cmd = mob.Character.Charmed.ExpiredCommand
		}
		if mob.Character.IsCharmed(user.UserId) {
			mob.Character.RemoveCharm()
		}
		if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
			room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
				`<ansi fg="mobname">%s</ansi> slips its bond and leaves.`, comp.Name), user.UserId)
		}
		if comp.SourceType == characters.CompanionCharmed {
			runCharmExpiredCommand(mob, cmd)
			continue
		}
		mob.Command("despawn")
	}
	user.Character.RecalculateStats()
	events.AddToQueue(events.CharacterVitalsChanged{UserId: user.UserId})
}

// MutationGained_ReleaseCompanions runs the release after every player rank
// gain, including the silent Bloom and Phial grants.
func MutationGained_ReleaseCompanions(e events.Event) events.ListenerReturn {
	evt, ok := e.(mutations.Gained)
	if !ok {
		return events.Continue
	}
	if user := users.GetByUserId(evt.UserId); user != nil {
		releaseCompanionsForUser(user)
	}
	return events.Continue
}
