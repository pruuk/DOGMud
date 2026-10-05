package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// ItemRoundTick fires item_idle once a round for every item with a behaviour
// tree that it reaches (lighting 5e, item behaviour slice 1, Rule 5), in this
// order: every online player's worn slots then backpack; the indexed mobs'
// worn slots then backpack; the indexed rooms' floors. Never a world walk:
// players are bounded by who is online (the Pinnacle tick's precedent), and
// mobs and rooms come from the holder index (internal/items). A visited
// holder with nothing treed left drops out of the index. Container contents
// are not visited. Last, the state of every item not visited this round is
// evicted (Rule 4).
func ItemRoundTick(e events.Event) events.ListenerReturn {
	round := util.GetRoundCount()

	for _, user := range users.GetAllActiveUsers() {
		if user.Character != nil {
			tickHeldItems(user.Character, user.UserId, 0)
		}
	}
	for _, id := range items.MobHolders() {
		m := mobs.GetInstance(id)
		if m == nil || !tickHeldItems(&m.Character, 0, id) {
			items.DropMobHolder(id)
		}
	}
	for _, roomId := range items.RoomHolders() {
		if !tickFloorItems(roomId) {
			items.DropRoomHolder(roomId)
		}
	}

	behaviortree.EvictUnseenItemBTreeStates(round)
	return events.Continue
}

// EvaluateRoomFixtures runs the trees of every treed item on a room's floor
// now. RegisterListeners sets it as items.OnRoomHolderIndexed, so a room
// that joins the index (a fixture spawned, or a room loaded holding one) is
// lit before anyone reads it.
func EvaluateRoomFixtures(roomId int) {
	tickFloorItems(roomId)
}

func itemIdleEvent() behaviortree.EventContext {
	return behaviortree.EventContext{EventType: "item_idle"}
}

// tickHeldItems runs item_idle for a character's treed worn and backpack
// items, and reports whether it found any.
func tickHeldItems(c *characters.Character, userId, mobInstanceId int) bool {
	found := false
	for _, s := range c.Equipment.AllSlots() {
		if s.Item.ItemId <= 0 || !s.Item.HasBehavior() {
			continue
		}
		found = true
		behaviortree.TryItemBehavior(itemIdleEvent(), behaviortree.ItemSubject{
			UUID: s.Item.UUID, ItemId: s.Item.ItemId, UserId: userId, MobInstanceId: mobInstanceId,
			RoomId: c.RoomId, Slot: s.Key,
		})
	}
	for _, it := range c.GetAllBackpackItems() {
		if !it.HasBehavior() {
			continue
		}
		found = true
		behaviortree.TryItemBehavior(itemIdleEvent(), behaviortree.ItemSubject{
			UUID: it.UUID, ItemId: it.ItemId, UserId: userId, MobInstanceId: mobInstanceId,
			RoomId: c.RoomId,
		})
	}
	return found
}

// tickFloorItems runs item_idle for a loaded room's treed floor items, drops
// any fixture output whose item is no longer on the floor, and reports
// whether it found any treed item. An unloaded room loses its outputs and
// reports none.
func tickFloorItems(roomId int) bool {
	if !rooms.IsRoomLoaded(roomId) {
		itemlight.ClearRoom(roomId)
		return false
	}
	r := rooms.LoadRoom(roomId)
	if r == nil {
		return false
	}
	keep := map[uuid.UUID]bool{}
	for _, it := range append([]items.Item(nil), r.Items...) {
		if !it.HasBehavior() {
			continue
		}
		keep[it.UUID] = true
		behaviortree.TryItemBehavior(itemIdleEvent(), behaviortree.ItemSubject{
			UUID: it.UUID, ItemId: it.ItemId, RoomId: roomId, OnFloor: true,
		})
	}
	itemlight.Retain(roomId, keep)
	return len(keep) > 0
}
