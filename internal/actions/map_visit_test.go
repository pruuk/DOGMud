package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// mapVisitRoom seeds one sky-less room lit by exactly lamp (0 is pitch dark)
// and a character standing in it.
func mapVisitRoom(t *testing.T, lamp int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	zero := 0.0
	room := &rooms.Room{RoomId: 9730, Zone: "MapZone", SkyLight: &zero}
	if lamp > 0 {
		room.Lamp = rooms.LampPtr(lamp)
	}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9730: room},
		map[string]*rooms.ZoneConfig{"MapZone": {Name: "MapZone", RoomId: 9730, RoomIds: map[int]struct{}{9730: {}}}},
	))
	u := users.NewTestUser(9731, "mapper", "Mapper", 97731)
	u.Character.RoomId = 9730
	return u, room
}

// #252, R5: a room walked through in pitch dark does not join the map.
func TestMarkRoomMappedIfSeen_DarkRoomStaysOffTheMap(t *testing.T) {
	u, room := mapVisitRoom(t, 0)
	require.False(t, MarkRoomMappedIfSeen(u.Character, room))
	require.False(t, u.Character.HasVisitedRoom("MapZone", 9730))
}

// A room the character makes out, even as shapes, joins the map.
func TestMarkRoomMappedIfSeen_ShapesRoomJoinsTheMap(t *testing.T) {
	u, room := mapVisitRoom(t, 30)
	require.True(t, MarkRoomMappedIfSeen(u.Character, room))
	require.True(t, u.Character.HasVisitedRoom("MapZone", 9730))
}

// A room already on the map stays there when the character returns blind.
func TestMarkRoomMappedIfSeen_DarkReturnKeepsAMappedRoom(t *testing.T) {
	u, room := mapVisitRoom(t, 0)
	u.Character.MarkRoomVisited("MapZone", 9730)
	require.False(t, MarkRoomMappedIfSeen(u.Character, room))
	require.True(t, u.Character.HasVisitedRoom("MapZone", 9730))
}
