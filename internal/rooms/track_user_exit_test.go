package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// findUserExit reads the player visitors of the rooms next door. It used to
// read the mob visitors and look their instance ids up as user ids, so a
// player's own trail was never found, and a mob whose instance id matched a
// user id was taken for that player.
func TestFindUserExit_ReadsThePlayerVisitors(t *testing.T) {
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	const here, north, east, quarry = 3390, 3391, 3392, 97901
	t.Cleanup(SeedRoomsForTest(map[int]*Room{
		here:  {RoomId: here, Exits: map[string]exit.RoomExit{"north": {RoomId: north}, "east": {RoomId: east}}},
		north: {RoomId: north},
		east:  {RoomId: east},
	}, map[string]*ZoneConfig{}))
	u := users.NewTestUser(quarry, "quill", "Quill", quarry)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{quarry: u}))
	m := &mobs.Mob{InstanceId: quarry}
	m.Character.Name = "Quill"
	mobs.SetInstanceForTest(quarry, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(quarry, nil) })

	// The player went north; a mob that shares the player's id went east.
	LoadRoom(north).MarkVisited(quarry, VisitorUser)
	LoadRoom(east).MarkVisited(quarry, VisitorMob)

	r := LoadRoom(here)
	require.Equal(t, "north", r.findUserExit(quarry, ""), "by id")
	require.Equal(t, "north", r.findUserExit(0, "Qui"), "by name")
}
