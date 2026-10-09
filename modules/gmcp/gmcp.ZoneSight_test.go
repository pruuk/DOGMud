package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #382 playtest: Zone.Map always added the player's current room, so a
// character who logged in already in the dark read "Cave Mouth" off the map
// payload. The saved fog map (D2) already refused a room seen in the dark;
// the snapshot must agree.
func TestZoneMapVisited_DarkCurrentRoomIsNotAdded(t *testing.T) {
	viewer := roomSightFixture(t, 0)
	room := rooms.LoadRoom(viewer.Character.RoomId)
	require.NotNil(t, room)
	visited := zoneMapVisited(viewer, room, func(int) bool { return true })
	require.NotContains(t, visited, 9720, "a room the player sees nothing in must not reach the map")
}

func TestZoneMapVisited_ShapesCurrentRoomIsAdded(t *testing.T) {
	viewer := roomSightFixture(t, 30)
	room := rooms.LoadRoom(viewer.Character.RoomId)
	require.NotNil(t, room)
	visited := zoneMapVisited(viewer, room, func(int) bool { return true })
	require.Contains(t, visited, 9720, "a room the player can make out belongs on the map")
}

func TestZoneMapVisited_KeepsRoomsAlreadyMapped(t *testing.T) {
	viewer := roomSightFixture(t, 0)
	viewer.Character.MarkRoomVisited("SightZone", 9721)
	room := rooms.LoadRoom(viewer.Character.RoomId)
	require.NotNil(t, room)
	visited := zoneMapVisited(viewer, room, func(int) bool { return true })
	require.Contains(t, visited, 9721, "the dark keeps rooms already seen (R5)")
	require.NotContains(t, visited, 9720)
}
