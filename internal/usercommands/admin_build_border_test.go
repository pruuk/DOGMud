package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// `build room <dir>` and `teleport <dir>` look for a room already in that
// direction on the map. Zone maps stop at their border (#375 phase 4), so a
// room of a neighbouring zone in the cell must still be found, as master's
// one map of the connected world found it (Task 20):
//
//	9963 UB (0,-1)   9962 UB (1,-1)
//	9960 UA (0,0)    9961 UA (1,0)
//
// 9960 has no north exit; 9963, north of it, is in zone UB.
func borderBuildScene(t *testing.T) *users.UserRecord {
	t.Helper()
	lit := func(id int, zone string, x, y int, exits map[string]exit.RoomExit) *rooms.Room {
		return &rooms.Room{RoomId: id, Zone: zone, Title: "Room", Description: "A room.",
			Lamp: rooms.LampPtr(60), X: x, Y: y, Exits: exits}
	}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		9960: lit(9960, "UA", 0, 0, map[string]exit.RoomExit{"east": {RoomId: 9961}}),
		9961: lit(9961, "UA", 1, 0, map[string]exit.RoomExit{"west": {RoomId: 9960}, "north": {RoomId: 9962}}),
		9962: lit(9962, "UB", 1, -1, map[string]exit.RoomExit{"south": {RoomId: 9961}, "west": {RoomId: 9963}}),
		9963: lit(9963, "UB", 0, -1, map[string]exit.RoomExit{"east": {RoomId: 9962}}),
	}, map[string]*rooms.ZoneConfig{
		"UA": {Name: "UA", RoomId: 9960, RoomIds: map[int]struct{}{9960: {}, 9961: {}}},
		"UB": {Name: "UB", RoomId: 9962, RoomIds: map[int]struct{}{9962: {}, 9963: {}}},
	}))
	// Every map, as PreCacheMaps builds them at boot.
	for _, id := range []int{9960, 9961, 9962, 9963} {
		mapper.GetMapper(id)
	}
	admin := users.NewTestUser(9964, "bld", "Builda", 99964)
	admin.Role = users.RoleAdmin
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9964: admin}))
	admin.Character.RoomId = 9960
	rooms.LoadRoom(9960).AddPlayer(admin.UserId)
	return admin
}

func TestBuildRoom_ConnectsToANeighbouringZonesRoomInThatCell(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	admin := borderBuildScene(t)

	_, err := Build(`room north`, admin, rooms.LoadRoom(9960), 0)
	require.NoError(t, err)

	// The room already in the cell is used, not a new one built over it. (The
	// exits ConnectRoom adds are written to room files, which these seeded
	// rooms do not have, so the move is what this checks.)
	require.Equal(t, 9963, admin.Character.RoomId, "the admin is moved to the room already there")
}

// Owner ruling 18 (2026-11-26): every exit name fits a deck button, and the
// boot refuses a longer one, so `build room <name>` and `room exit` refuse a
// name longer than exit.MaxNameLetters before anything is built or renamed.
func TestBuilders_RefuseAnExitNameLongerThanAButton(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	admin := borderBuildScene(t)
	room := rooms.LoadRoom(9960)

	_, err := Build(`room outlook`, admin, room, 0)
	require.NoError(t, err)
	require.Equal(t, 9960, admin.Character.RoomId, "nothing was built, so the admin did not move")
	require.NotContains(t, room.Exits, "outlook")

	_, err = adminRoom_Exit([]string{"exit", "east", "outlook"}, admin, room)
	require.NoError(t, err)
	require.Contains(t, room.Exits, "east", "the rename was refused")
	require.NotContains(t, room.Exits, "outlook")

	_, err = adminRoom_Exit([]string{"exit", "east", "ledge"}, admin, room)
	require.NoError(t, err)
	require.Contains(t, room.Exits, "ledge", "fixture check: a six-letter rename goes through")
}

func TestTeleportDirection_FindsANeighbouringZonesRoom(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	admin := borderBuildScene(t)

	_, err := Teleport(`north`, admin, rooms.LoadRoom(9960), 0)
	require.NoError(t, err)
	require.Equal(t, 9963, admin.Character.RoomId)
}
