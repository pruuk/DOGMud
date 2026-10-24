package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// MarkRoomMappedIfSeen records room on the character's fog-of-war map
// (characters.Character.MarkRoomVisited, the Zone.Map set) when the character
// makes out anything there, and reports whether that changed the map: true
// only when the room was newly added. A room walked through in the dark stays
// off the map until the character sees it (#252, owner ruling R5); rooms
// already on the map stay, and seeing them again reports false.
//
// The sight-change GMCP path re-sends Zone.Map on true. Until 2026-10-24 this
// returned true whenever the room was visible, so every light-band change
// re-sent an unchanged map (about 271 KB for an explored character): 73
// Zone.Map sends for 60 moves.
//
// The template id goes on the map, never an ephemeral instance id: the raw
// instance id (e.g. 1000000000) is unauthorable and would leak onto the
// Zone.Map snapshot. rooms.OriginalRoomId returns a normal room's own id.
func MarkRoomMappedIfSeen(c *characters.Character, room *rooms.Room) bool {
	if c == nil || room == nil {
		return false
	}
	if messaging.ParticipantSight(c, room) == messaging.SightNone {
		return false
	}
	templateId, _ := rooms.OriginalRoomId(room.RoomId)
	if c.HasVisitedRoom(room.Zone, templateId) {
		return false
	}
	c.MarkRoomVisited(room.Zone, templateId)
	return true
}
