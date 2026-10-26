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

// releaseCompanionsOverCeiling drops companion records, newest first, while
// the Conviction reservation is past the ceiling (#680). It acts only when
// the body holds part of the pool: a gained rank of Titan Growth, Colossus
// Form or Extra Arms crowds out bonds, while an overage from re-pricing or
// gear alone stays grandfathered (D4). Bonded companions hold no reserve and
// are never released. Gear is never removed. Returns the records it removed.
//
// It reads the pool maximum as it stands and does not recalculate stats; the
// caller (releaseCompanionsForUser) does that first.
func releaseCompanionsOverCeiling(ch *characters.Character) []characters.CompanionInfo {
	if mutations.OwnedReserveUnits(ch.Mutations) == 0 {
		return nil
	}
	var released []characters.CompanionInfo
	for ch.ReservationOverages().Conviction > 0 {
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
