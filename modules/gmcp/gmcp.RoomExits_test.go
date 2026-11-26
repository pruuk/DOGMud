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
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutators"
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

// A mutator exit is listed like a room exit, and a mutator exit of a room
// exit's name takes its place (GetExitInfo's precedence, D6).
func TestRoomInfo_MutatorExitsAreListed(t *testing.T) {
	viewer, _ := exitsFixture(t, 60)
	t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
		MutatorId: "test-roominfo-bridge",
		Exits: map[string]exit.RoomExit{
			"bridge": {RoomId: 9721},
			"north":  {RoomId: 9721, Lock: gamelock.Lock{Difficulty: 5}},
		},
	}))
	require.True(t, rooms.GetZoneConfig("SightZone").Mutators.Add("test-roominfo-bridge"), "fixture premise")

	p := roomInfoFor(t, viewer)
	require.Equal(t, 9721, p.Exits["bridge"])
	require.Equal(t, "named", p.ExitsV2["bridge"].Kind)
	line, ok := buttons.Resolve(viewer.UserId, 9720, p.ExitsV2["bridge"].Id)
	require.True(t, ok)
	require.Equal(t, "go bridge", line)
	require.Contains(t, p.ExitsV2["north"].Details, "locked", "the mutator's locked north replaces the room's open one")
}

// A temporary exit named like a permanent one is reached by the permanent
// one's name (Room.FindExitByName), so Room.Info lists it once, as the
// permanent exit.
func TestRoomInfo_ATemporaryExitSharingAPermanentNameIsListedOnce(t *testing.T) {
	viewer, room := exitsFixture(t, 60)
	require.True(t, room.AddTemporaryExit("north", exit.TemporaryRoomExit{RoomId: 9720}))

	p := roomInfoFor(t, viewer)
	require.Equal(t, 9721, p.Exits["north"])
	require.Equal(t, "compass", p.ExitsV2["north"].Kind)
	require.Equal(t, 9721, p.ExitsV2["north"].RoomId)
	require.Equal(t, "m1", p.ExitsV2["north"].Id)
}

// One sight verdict per Room.Info: the builder uses the sight GetRoomNode
// read, not a second ParticipantSight of its own.
func TestRoomInfo_ExitsFollowTheSightPassedIn(t *testing.T) {
	viewer, room := exitsFixture(t, 60)
	info := buildRoomExitsInfo(viewer, room, messaging.SightNone)
	require.True(t, info.Dark)
	require.Empty(t, info.Exits)
	require.Empty(t, info.ExitsV2)
}

// pinRelockTiming makes a lock's relock hour a real span of rounds and
// returns the round it set. Test binaries load Go timing defaults, not
// config.yaml, and there an hour is less than a round; this pins the shipped
// timing (as gamelock_test.go and the rooms KnownExits test do) and starts
// past RoundCountMinimum as the game does.
func pinRelockTiming(t *testing.T) uint64 {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	start := uint64(util.RoundCountMinimum) + 1000
	util.SetRoundCountForTest(start)
	t.Cleanup(util.ResetRoundCountForTest)
	return start
}

// Ruling 3: `locked` follows the door's real state: lock, unlock, and a
// relock on its timer.
func TestRoomInfo_LockedMeansLockedNow(t *testing.T) {
	start := pinRelockTiming(t)
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

// Owner ruling 18 and R2: an instance portal's button reads its zone, not
// the number, and is wired to that portal's own key; a second portal the
// viewer may enter gets a button too, and only the viewer's own carries the
// own_portal detail the client sorts first by. (Neither destination is an
// instance room here, so the viewer may enter both; the refusal case is
// rooms.TestForDeck_*.)
func TestRoomInfo_InstancePortalsAreLabelledByZoneAndTheViewersOwnIsMarked(t *testing.T) {
	viewer, room := exitsFixture(t, 60)
	own := exit.InstancePortalKey("arena", viewer.UserId)
	theirs := exit.InstancePortalKey("arena", viewer.UserId+1)
	require.True(t, room.AddTemporaryExit(own, exit.TemporaryRoomExit{RoomId: 9721, UserId: viewer.UserId}))
	require.True(t, room.AddTemporaryExit(theirs, exit.TemporaryRoomExit{RoomId: 9721, UserId: viewer.UserId + 1}))

	p := roomInfoFor(t, viewer)
	for _, key := range []string{own, theirs} {
		require.Equal(t, "arena", p.ExitsV2[key].Label, key)
		line, ok := buttons.Resolve(viewer.UserId, 9720, p.ExitsV2[key].Id)
		require.True(t, ok, key)
		require.Equal(t, "go "+key, line)
	}
	require.Contains(t, p.ExitsV2[own].Details, "own_portal")
	require.NotContains(t, p.ExitsV2[theirs].Details, "own_portal")
	require.Empty(t, p.ExitsV2["out"].Label, "an exit shown by its own name sends no label")
	require.NotContains(t, p.ExitsV2["portal"].Details, "own_portal", "a plain temporary exit is nobody's portal")
}

// R2 through Room.Info: an exit into an instance that does not authorize the
// viewer stays in the exit maps, as on the exits line, with no button and no
// label: another player's portal, a rift under its own name, and the
// viewer's own portal after their access was revoked (ejected death policy),
// which then is not marked own_portal either.
func TestRoomInfo_AnExitIntoAnInstanceTheViewerMayNotEnterHasNoButton(t *testing.T) {
	viewer, room := exitsFixture(t, 60)
	entryTheirs, entryOwn := 2_000_000_000, 2_000_000_100
	require.True(t, rooms.IsEphemeralRoomId(entryTheirs))
	require.True(t, rooms.IsEphemeralRoomId(entryOwn))
	reg := rooms.GetInstanceRegistry()
	instTheirs := &rooms.ZoneInstance{InstanceId: entryTheirs, EntryRoomId: entryTheirs, AuthorizedUsers: []int{viewer.UserId + 1}}
	instOwn := &rooms.ZoneInstance{InstanceId: entryOwn, EntryRoomId: entryOwn, AuthorizedUsers: []int{}}
	reg.Add(instTheirs)
	reg.Add(instOwn)
	t.Cleanup(func() {
		reg.Remove(instTheirs)
		reg.Remove(instOwn)
	})
	own := exit.InstancePortalKey("arena", viewer.UserId)
	theirs := exit.InstancePortalKey("arena", viewer.UserId+1)
	require.True(t, room.AddTemporaryExit(own, exit.TemporaryRoomExit{RoomId: entryOwn, UserId: viewer.UserId}))
	require.True(t, room.AddTemporaryExit(theirs, exit.TemporaryRoomExit{RoomId: entryTheirs, UserId: viewer.UserId + 1}))
	require.True(t, room.AddTemporaryExit("rift", exit.TemporaryRoomExit{RoomId: entryTheirs}))

	p := roomInfoFor(t, viewer)
	for _, key := range []string{own, theirs, "rift"} {
		require.Contains(t, p.Exits, key, "the exits line still lists it")
		require.Contains(t, p.ExitsV2, key)
		require.Empty(t, p.ExitsV2[key].Id, key)
		require.Empty(t, p.ExitsV2[key].Label, key)
	}
	require.NotContains(t, p.ExitsV2[own].Details, "own_portal", "a portal with no button is not marked")
	require.NotEmpty(t, p.ExitsV2["portal"].Id, "an exit the viewer may enter keeps its button")
}
