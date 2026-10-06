package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// fireItemProc runs a proc event into the one worn item it reaches (item
// behaviour slice 3, Rule 21): a hit or a spell hit reaches the weapon, a
// block the offhand, a grapple the body armour (spec P3). owner is whoever
// the event belongs to, other the opponent, room nil where the caller does
// not know it. A kill does not come through here: it reaches every worn
// treed item through MobDeathItemProcs (ruling S4).
//
// ItemProcsEnabled is read first, so a disabled proc draws no number, and
// an item without a tree costs one spec lookup.
func fireItemProc(event behaviortree.EventContext, owner, other *characters.Character, room *rooms.Room, damage int) {
	if owner == nil || !behaviortree.ItemProcsOn() {
		return
	}
	var it items.Item
	var slot string
	switch event.EventType {
	case "on_hit", "on_spell_hit":
		it, slot = owner.Equipment.Weapon, "weapon"
	case "on_block":
		it, slot = owner.Equipment.Offhand, "offhand"
	case "on_grapple":
		it, slot = owner.Equipment.Body, "body"
	default:
		return
	}
	if it.ItemId <= 0 || !it.HasBehavior() {
		return
	}
	event.Proc = &behaviortree.ProcEvent{Other: other, Room: room, Damage: damage}
	fireItemEvent(event, it, slot, owner, owner.GetUserId(), owner.MobInstanceId)
}
