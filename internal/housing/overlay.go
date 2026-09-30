package housing

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// The overlay lays a house's own state over its unit rooms. Exits, title,
// description, nouns and coordinates are instance:"skip" room fields: an
// instance save never writes them and every load restores them from the
// template. So the house record is their only source of truth, and the
// overlay reapplies it every time a unit room is built (rooms.SetRoomOverlay)
// and whenever the house changes while the room is loaded.
//
// What it does to a room of a house:
//   - adds a doorway exit for every link touching the room;
//   - for any room but the entry: removes the door back to the alley (an
//     extension is reached through the house, not from the street), removes
//     the door noun, and uses the building's extension title and text;
//   - uses the owner's own description, if they wrote one;
//   - places the room relative to the entry room's authored coordinates, so
//     the map draws the house the way its doorways say.
//
// It is idempotent, and it never loads another room.

// ApplyOverlay is the rooms.RoomOverlay. Rooms that are not in a house are
// left exactly as their template made them.
func ApplyOverlay(r *rooms.Room) {
	if r == nil {
		return
	}
	mu.RLock()
	h, owned := roomHouse[r.RoomId]
	if !owned {
		mu.RUnlock()
		return
	}
	house := h.clone()
	b := buildings[house.BuildingId]
	base, hasBase := unitCoords[house.EntryRoom()]
	mu.RUnlock()
	if b == nil {
		return
	}
	overlayRoom(r, house, *b, base, hasBase)
}

func overlayRoom(r *rooms.Room, house House, b Building, base [4]int, hasBase bool) {
	if r.Exits == nil {
		r.Exits = map[string]exit.RoomExit{}
	}
	for dir, to := range house.exitsOf(r.RoomId) {
		r.Exits[dir] = exit.RoomExit{RoomId: to}
	}

	if r.RoomId != house.EntryRoom() {
		delete(r.Exits, b.DoorExit)
		delete(r.Nouns, b.DoorExit)
		r.Title = b.ExtensionTitle
		r.Description = b.ExtensionDescription
	}
	if text, ok := house.Descriptions[r.RoomId]; ok && text != `` {
		r.Description = text
	}

	if hasBase {
		off := house.offsets()[r.RoomId]
		r.X, r.Y, r.Z, r.Plane = base[0]+off[0], base[1]+off[1], base[2]+off[2], base[3]
	}
}

// applyAllOverlays lays every house over its rooms. LoadDataFiles calls it
// after houses load, because the building validation and the rest of boot may
// already have loaded unit rooms before any house was known.
func applyAllOverlays() {
	for _, h := range AllHouses() {
		for _, roomId := range h.RoomIds {
			ApplyOverlay(rooms.LoadRoom(roomId))
		}
	}
}

// refreshHouse reapplies the overlay to a changed house's loaded rooms and
// asks for its map to be rebuilt from both the entry and the newest room, so
// every room of the house resolves to a map that includes the new one.
func refreshHouse(h House, newestRoom int) {
	for _, roomId := range h.RoomIds {
		ApplyOverlay(rooms.LoadRoom(roomId))
	}
	events.AddToQueue(events.RebuildMap{MapRootRoomId: newestRoom})
	events.AddToQueue(events.RebuildMap{MapRootRoomId: h.EntryRoom()})
}
