package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// MarkRoomMappedIfSeen records room on the character's fog-of-war map
// (characters.Character.MarkRoomVisited, the Zone.Map set) when the character
// makes out anything there, and reports whether it did. A room walked through
// in the dark stays off the map until the character sees it (#252, owner
// ruling R5); rooms already on the map stay.
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
	c.MarkRoomVisited(room.Zone, templateId)
	return true
}
