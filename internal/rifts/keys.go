package rifts

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// keys.go: the Facet Key (a profile's key_item_id). It is generic: any one of
// them opens any locked door of the run it is used in, and is used up doing
// so. It exists only inside a rift: it is purged when its holder leaves the
// rift, dies, logs out or logs back in somewhere else. The profile's light
// (the Glowstone) is purged the same way: like the rest of a rift room's
// placements it does not leave the rift.

func isKey(itm items.Item, p *Profile) bool {
	return p != nil && itm.ItemId == p.KeyItemId
}

func hasKey(u *users.UserRecord, p *Profile) bool {
	if u == nil {
		return false
	}
	for _, itm := range u.Character.GetAllBackpackItems() {
		if isKey(itm, p) {
			return true
		}
	}
	return false
}

// takeKey removes one key from u's pack. False if they had none.
func takeKey(u *users.UserRecord, p *Profile) bool {
	if u == nil {
		return false
	}
	for _, itm := range u.Character.GetAllBackpackItems() {
		if !isKey(itm, p) {
			continue
		}
		if u.Character.RemoveItem(itm) {
			events.AddToQueue(events.ItemOwnership{UserId: u.UserId, Item: itm, Gained: false})
			return true
		}
	}
	return false
}

// giveKey puts a key in u's pack, or at their feet when the pack is full.
func giveKey(u *users.UserRecord, p *Profile, room *rooms.Room) {
	itm := items.New(p.KeyItemId)
	if !itm.IsValid() {
		return
	}
	if u.Character.StoreItem(itm) {
		events.AddToQueue(events.ItemOwnership{UserId: u.UserId, Item: itm, Gained: true})
		return
	}
	if room != nil {
		room.AddItem(itm, false)
	}
}

// GiveKeyTo gives u one of run's keys (admin and tests).
func GiveKeyTo(u *users.UserRecord, run *Run) {
	if u == nil || run == nil {
		return
	}
	giveKey(u, run.Profile, rooms.LoadRoom(u.Character.RoomId))
}

// giveItem is giveKey for any item id (a puzzle's ore reward).
func giveItem(u *users.UserRecord, itemId int, room *rooms.Room) (name string, carried bool) {
	itm := items.New(itemId)
	if !itm.IsValid() {
		return ``, false
	}
	if u.Character.StoreItem(itm) {
		events.AddToQueue(events.ItemOwnership{UserId: u.UserId, Item: itm, Gained: true})
		return itm.DisplayName(), true
	}
	if room != nil {
		room.AddItem(itm, false)
	}
	return itm.DisplayName(), false
}

// riftOnlyItemIds are the items that exist only inside a rift: every
// profile's key and its light (the Glowstone).
func riftOnlyItemIds() map[int]bool {
	ids := map[int]bool{}
	for _, p := range profiles {
		if p.KeyItemId != 0 {
			ids[p.KeyItemId] = true
		}
		if p.LightItemId != 0 {
			ids[p.LightItemId] = true
		}
	}
	return ids
}

// PurgeKeys removes every rift-only item (keys and lights, of any profile)
// from u: their pack, what they wear or hold, and the packs of the creatures
// they have charmed (a companion follows them out). It returns how many were
// removed.
func PurgeKeys(u *users.UserRecord) int {
	if u == nil {
		return 0
	}
	ids := riftOnlyItemIds()
	if len(ids) == 0 {
		return 0
	}
	n := 0
	for _, itm := range u.Character.GetAllWornItems() {
		if ids[itm.ItemId] && u.Character.RemoveFromBody(itm) {
			events.AddToQueue(events.ItemOwnership{UserId: u.UserId, Item: itm, Gained: false})
			n++
		}
	}
	for _, itm := range u.Character.GetAllBackpackItems() {
		if ids[itm.ItemId] && u.Character.RemoveItem(itm) {
			events.AddToQueue(events.ItemOwnership{UserId: u.UserId, Item: itm, Gained: false})
			n++
		}
	}
	for _, mobInstanceId := range u.Character.GetCharmIds() {
		m := mobs.GetInstance(mobInstanceId)
		if m == nil {
			continue
		}
		for _, itm := range m.Character.GetAllWornItems() {
			if ids[itm.ItemId] && m.Character.RemoveFromBody(itm) {
				n++
			}
		}
		for _, itm := range m.Character.GetAllBackpackItems() {
			if ids[itm.ItemId] && m.Character.RemoveItem(itm) {
				n++
			}
		}
	}
	return n
}
