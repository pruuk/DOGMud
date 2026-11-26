package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func pinTrackTiming(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
}

// findUserExit reads the player visitors of the rooms next door. It used to
// read the mob visitors and look their instance ids up as user ids, so a
// player's own trail was never found, and a mob whose instance id matched a
// user id was taken for that player.
func TestFindUserExit_ReadsThePlayerVisitors(t *testing.T) {
	pinTrackTiming(t)
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
	tracker := &characters.Character{}
	require.Equal(t, "north", r.findUserExit(tracker, quarry, ""), "by id")
	require.Equal(t, "north", r.findUserExit(tracker, 0, "Qui"), "by name")
}

// #781 owner ruling 17: the room render's "Tracking X... They went <exit>"
// follows a trail through a secret exit the tracker has found; one the
// tracker has not found loses the trail, as before; an ordinary exit is
// unchanged. Mob and player quarries alike, through GetDetails, so the
// tracker passed in is the viewer.
func TestTrackingString_FollowsOnlyAFoundSecretExit(t *testing.T) {
	const quarryMob, quarryUser, trackCond = 97951, 97952, 86
	cases := []struct {
		name, via string
		found     bool
		want      string
	}{
		{"unfound secret exit", "west", false, "You lost the trail of"},
		{"found secret exit", "west", true, "They went"},
		{"ordinary exit", "north", false, "They went"},
	}
	for _, quarry := range []string{"tracking-mob", "tracking-user"} {
		for _, tc := range cases {
			t.Run(quarry+"/"+tc.name, func(t *testing.T) {
				r, _ := knownExitsRoom(t, true) // west is secret, to room 5; north to room 2
				pinTrackTiming(t)
				t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
					trackCond: {ConditionId: trackCond, Name: "Tracking", TriggerCount: 1000000},
				}))
				m := &mobs.Mob{InstanceId: quarryMob}
				m.Character.Name = "Quill"
				mobs.SetInstanceForTest(quarryMob, m)
				t.Cleanup(func() { mobs.SetInstanceForTest(quarryMob, nil) })
				q := users.NewTestUser(quarryUser, "quinn", "Quinn", 97952)
				t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{quarryUser: q}))

				dest := r.Exits[tc.via].RoomId
				var vType VisitorType = VisitorMob
				id, name := quarryMob, "Quill"
				if quarry == "tracking-user" {
					vType, id, name = VisitorUser, quarryUser, "Quinn"
				}
				r.MarkVisited(id, vType)
				LoadRoom(dest).MarkVisited(id, vType)

				viewer := users.NewTestUser(7812, "tracker", "Tracker", 97812)
				require.NoError(t, viewer.Character.AddCondition(trackCond, true))
				viewer.Character.SetMiscData(quarry, name)
				if tc.found {
					viewer.Character.LearnSecretExit(1, tc.via)
				}

				got := GetDetails(r, viewer).TrackingString
				require.Contains(t, got, tc.want, "trail through %s", tc.via)
				if tc.want == "They went" {
					require.Contains(t, got, tc.via)
				}
			})
		}
	}
}
