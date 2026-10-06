package rooms

import "github.com/GoMudEngine/GoMud/internal/items"

// A room's floor joins the item tick's holder index (lighting 5e, Rule 5)
// when a treed item lands on it (AddItem, Prepare's spawn append) and when
// the room loads into memory holding one (addRoomToMemory). It leaves when
// it unloads (removeRoomFromMemory) or when the tick finds nothing treed
// left on it. Stashed items are never visited, so a stash does not index.

// noteTreedFloorItem enters this room in the index when the item names a
// behaviour tree.
func (r *Room) noteTreedFloorItem(i items.Item) {
	if i.HasBehavior() {
		items.IndexRoomHolder(r.RoomId)
	}
}

// indexTreedFloor enters this room in the index when any floor item names a
// behaviour tree.
func (r *Room) indexTreedFloor() {
	for _, it := range r.Items {
		if it.HasBehavior() {
			items.IndexRoomHolder(r.RoomId)
			return
		}
	}
}

// IndexTreedFloors enters every room already in memory that holds a treed
// floor item. main.go calls it once, after item specs load: a room loaded
// before then (a faction holding cell) could not tell which items are
// treed.
func IndexTreedFloors() {
	for _, r := range roomManager.rooms {
		r.indexTreedFloor()
	}
}
