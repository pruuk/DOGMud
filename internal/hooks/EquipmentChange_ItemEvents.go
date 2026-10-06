package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ItemEquipEvents fires on_equip into the tree of every treed item an
// equipment change put on, and on_unequip into every one it took off
// (item behaviour slice 2, spec X18, closes #222). Players and mobs alike:
// actions.equipItem and actions.removeWorn queue the one EquipmentChange
// both share, and internal/actions cannot reach a tree itself. An item put
// on is found in its slot by identity; an item taken off is in the
// backpack (or spilled to the floor), so it fires with no slot.
func ItemEquipEvents(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.EquipmentChange)
	if !ok || (len(evt.ItemsWorn) == 0 && len(evt.ItemsRemoved) == 0) {
		return events.Continue
	}
	c := equipmentChangeHolder(evt)
	if c == nil {
		return events.Continue
	}
	for _, it := range evt.ItemsWorn {
		if !it.HasBehavior() {
			continue
		}
		for _, s := range c.Equipment.AllSlots() {
			if s.Item.ItemId == it.ItemId && s.Item.UUID == it.UUID {
				fireItemEvent(behaviortree.EventContext{EventType: "on_equip"}, *s.Item, s.Key, c, evt.UserId, evt.MobInstanceId)
				break
			}
		}
	}
	for _, it := range evt.ItemsRemoved {
		if it.HasBehavior() {
			fireItemEvent(behaviortree.EventContext{EventType: "on_unequip"}, it, ``, c, evt.UserId, evt.MobInstanceId)
		}
	}
	return events.Continue
}

// equipmentChangeHolder is the character an equipment change happened to.
func equipmentChangeHolder(evt events.EquipmentChange) *characters.Character {
	if evt.UserId > 0 {
		if u := users.GetByUserId(evt.UserId); u != nil {
			return u.Character
		}
		return nil
	}
	if evt.MobInstanceId > 0 {
		if m := mobs.GetInstance(evt.MobInstanceId); m != nil {
			return &m.Character
		}
	}
	return nil
}

// fireWornItemEvent fires an event into the tree of every treed item c
// wears, in slot order.
func fireWornItemEvent(event behaviortree.EventContext, c *characters.Character, userId, mobInstanceId int) {
	for _, s := range c.Equipment.AllSlots() {
		if s.Item.ItemId > 0 && s.Item.HasBehavior() {
			fireItemEvent(event, *s.Item, s.Key, c, userId, mobInstanceId)
		}
	}
}

// fireItemEvent runs one item's tree for an event and reports whether the
// tree handled it.
func fireItemEvent(event behaviortree.EventContext, it items.Item, slot string, c *characters.Character, userId, mobInstanceId int) bool {
	return behaviortree.TryItemBehavior(event, behaviortree.ItemSubject{
		UUID: it.UUID, ItemId: it.ItemId, UserId: userId, MobInstanceId: mobInstanceId,
		RoomId: c.RoomId, Slot: slot,
	})
}
