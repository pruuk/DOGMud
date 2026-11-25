package gmcp

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

var perRoomIdInJSON = regexp.MustCompile(`"x[0-9a-f]{8}"`)

// exitsFixture is roomSightFixture's cellar (9720, north to 9721) at lamp,
// given one more exit of every kind: up, northeast, out, a secret west and a
// temporary portal.
func exitsFixture(t *testing.T, lamp int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	viewer := roomSightFixture(t, lamp)
	t.Cleanup(func() { buttons.Forget(viewer.UserId) })
	room := rooms.LoadRoom(9720)
	room.Exits["up"] = exit.RoomExit{RoomId: 9721}
	room.Exits["northeast"] = exit.RoomExit{RoomId: 9721}
	room.Exits["out"] = exit.RoomExit{RoomId: 9721}
	room.Exits["west"] = exit.RoomExit{RoomId: 9721, Secret: true}
	require.True(t, room.AddTemporaryExit("portal", exit.TemporaryRoomExit{RoomId: 9721}))
	return viewer, room
}

func payloadJSON(t *testing.T, p GMCPRoomModule_Payload) string {
	t.Helper()
	b, err := json.Marshal(p)
	require.NoError(t, err)
	return string(b)
}

// Guard 5 (sight) and "nothing leaks in the dark": dark is set, both exit
// maps are present and empty, and the only ids are the fixed T ids, which are
// the same everywhere.
func TestRoomInfo_TheDarkLeaksNoExitAndOffersOnlyTheFixedIds(t *testing.T) {
	viewer, _ := exitsFixture(t, 0)
	viewer.Character.LearnSecretExit(9720, "west")
	p := roomInfoFor(t, viewer)
	require.True(t, p.Dark)
	require.NotNil(t, p.Exits)
	require.Empty(t, p.Exits)
	require.NotNil(t, p.ExitsV2)
	require.Empty(t, p.ExitsV2)
	require.Equal(t, buttons.FixedIds(), p.Moves)
	js := payloadJSON(t, p)
	require.False(t, perRoomIdInJSON.MatchString(js), "the dark must carry no per-room id: %s", js)
	for _, leak := range []string{"northeast", "portal", `"out"`} {
		require.NotContains(t, js, leak)
	}
}

// Ruling 1 and the kinds and ids of every exit: an unfound secret exit is
// never sent; T directions carry fixed ids, others per-room ids that resolve
// to their exit.
func TestRoomInfo_KindsIdsAndOnlyKnownExits(t *testing.T) {
	viewer, _ := exitsFixture(t, 60)
	p := roomInfoFor(t, viewer)
	require.False(t, p.Dark)
	require.NotContains(t, p.Exits, "west")
	require.NotContains(t, p.ExitsV2, "west")
	for name, kind := range map[string]string{
		"north": "compass", "up": "vertical", "northeast": "diagonal", "out": "named", "portal": "temporary",
	} {
		require.Equal(t, kind, p.ExitsV2[name].Kind, name)
	}
	require.Equal(t, "m1", p.ExitsV2["north"].Id)
	require.Equal(t, "m5", p.ExitsV2["up"].Id)
	for _, name := range []string{"northeast", "out", "portal"} {
		id := p.ExitsV2[name].Id
		require.Regexp(t, `^x[0-9a-f]{8}$`, id, name)
		line, ok := buttons.Resolve(viewer.UserId, 9720, id)
		require.True(t, ok, name)
		require.Equal(t, "go "+name, line)
	}
	require.Equal(t, 9721, p.Exits["portal"])

	viewer.Character.LearnSecretExit(9720, "west")
	p = roomInfoFor(t, viewer)
	require.Contains(t, p.ExitsV2["west"].Details, "secret")
	require.Equal(t, "m4", p.ExitsV2["west"].Id)
}

// Ruling 3: `locked` follows the door's real state: lock, unlock, and a
// relock on its timer.
func TestRoomInfo_LockedMeansLockedNow(t *testing.T) {
	// Test binaries load Go timing defaults, not config.yaml, and there an
	// hour is less than a round; pin the shipped timing so the relock hour is
	// a real span of rounds (as gamelock_test.go and the rooms KnownExits
	// test do), and start past RoundCountMinimum as the game does.
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	start := uint64(util.RoundCountMinimum) + 1000
	util.SetRoundCountForTest(start)
	t.Cleanup(util.ResetRoundCountForTest)
	viewer, room := exitsFixture(t, 60)
	north := room.Exits["north"]
	north.Lock = gamelock.Lock{Difficulty: 5}
	room.Exits["north"] = north

	require.Contains(t, roomInfoFor(t, viewer).ExitsV2["north"].Details, "locked")
	room.SetExitLock("north", false)
	require.NotContains(t, roomInfoFor(t, viewer).ExitsV2["north"].Details, "locked", "unlocked now")
	util.SetRoundCountForTest(gametime.GetDate(start).AddPeriod(gamelock.DefaultRelockTime))
	require.Contains(t, roomInfoFor(t, viewer).ExitsV2["north"].Details, "locked", "relocked on its timer")
}

// Ruling 4: the dark marks ride in Room.Info while the room is dark.
func TestRoomInfo_DarkMarksRideOnlyInTheDark(t *testing.T) {
	viewer, room := exitsFixture(t, 0)
	buttons.RecordDarkMark(viewer.UserId, 9720, "north", buttons.MarkWall)
	require.Equal(t, map[string]string{"north": buttons.MarkWall}, roomInfoFor(t, viewer).DarkMarks)
	room.Lamp = rooms.LampPtr(60)
	p := roomInfoFor(t, viewer)
	require.NotNil(t, p.DarkMarks)
	require.Empty(t, p.DarkMarks)
}
