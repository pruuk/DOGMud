package rooms

import (
	"fmt"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// Untaken baubles (docs/baubles). A found bauble left lying in a room (a
// household's, or one its finder could not carry or was gone for) carries the
// time it was left (items.Item.BaubleLeftAt). Once it has lain untaken for
// BaubleUntakenHours it vanishes: someone tidied it away, or it was lost
// underfoot. A bauble anyone has carried is an ordinary possession and never
// vanishes (carrying clears the mark).
//
// Swept on the room's round tick, which only runs while players are in the
// room, when the room is prepared for a visitor after lying empty, and when
// a room is loaded from its instance file (LoadRoomInstance), so an expired
// bauble is never shown to someone walking in and never left for the bauble
// catalog sweep to count as a reference it should no longer see.

// removeUntakenBaubles removes the baubles that have lain untaken for the
// limit, records them as vanished, and tells anyone present. It returns how
// many it removed.
func (r *Room) removeUntakenBaubles(now time.Time) int {
	if len(r.Items) == 0 {
		return 0
	}
	// A player's own room (a housing lodging) is not a roadside: anything
	// on its floor stays until someone takes it.
	if IsPrivateRoom(r.RoomId) {
		return 0
	}
	limit := baubles.UntakenLimit()
	if limit <= 0 {
		return 0
	}
	expired := func(itm items.Item) bool {
		age, untaken := itm.BaubleUntakenFor(now)
		return untaken && age >= limit
	}
	anyExpired := false
	for _, itm := range r.Items {
		if expired(itm) {
			anyExpired = true
			break
		}
	}
	if !anyExpired {
		return 0
	}

	// A fresh slice: never compact r.Items in place, in case a caller holds
	// the old one.
	removed := 0
	kept := make([]items.Item, 0, len(r.Items))
	for _, itm := range r.Items {
		if expired(itm) {
			baubles.MarkVanished(itm.Bauble, now)
			if len(r.players) > 0 {
				r.SendText(messaging.CategoryRoomDescription, fmt.Sprintf(
					`The <ansi fg="itemname">%s</ansi> is no longer%s. Someone must have tidied it away.`,
					itm.DisplayName(), untakenWhere(itm.BaubleSpot)))
			}
			removed++
			continue
		}
		kept = append(kept, itm)
	}
	r.Items = kept
	return removed
}

// untakenWhere finishes "is no longer...": " on the bookshelf", or " here".
func untakenWhere(spot string) string {
	if spot == `` {
		return ` here`
	}
	return ` ` + spot
}
