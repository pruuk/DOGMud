package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// foundFixture seeds a lit, mapped zone: the study 9785 with the landing 9786
// to its north and a secret west exit to the vault 9787. The returned user
// stands in the study, has mapped it alone, has not found the west exit, and
// has a web client connection (the client Zone.Map diffs are sent to).
func foundFixture(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false) // the mapper logs when it builds a zone
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default"},
	}))
	zero := 0.0
	lit := func(id int, title string, exits map[string]exit.RoomExit) *rooms.Room {
		return &rooms.Room{RoomId: id, Zone: "FoundZone", Biome: "default", SkyLight: &zero,
			Lamp: rooms.LampPtr(60), Title: title, Exits: exits}
	}
	study := lit(9785, "Study", map[string]exit.RoomExit{
		"north": {RoomId: 9786},
		"west":  {RoomId: 9787, Secret: true},
	})
	landing := lit(9786, "Landing", map[string]exit.RoomExit{"south": {RoomId: 9785}})
	vault := lit(9787, "Vault", map[string]exit.RoomExit{"east": {RoomId: 9785}})
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9785: study, 9786: landing, 9787: vault},
		map[string]*rooms.ZoneConfig{"FoundZone": {Name: "FoundZone", RoomId: 9785,
			RoomIds: map[int]struct{}{9785: {}, 9786: {}, 9787: {}}}},
	))
	require.NotNil(t, mapper.GetMapper(9785, true), "the fixture zone must map")

	u := users.NewTestUser(9788, "finder", "Finder", 97788)
	u.Character.RoomId = 9785
	u.Character.MarkRoomVisited("FoundZone", 9785)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9788: u}))
	web := GMCPSettings{GMCPAccepted: true, HelloReceived: true}
	web.Client.Name = `WebClient`
	gmcpModule.cache.Add(97788, web)
	t.Cleanup(func() { gmcpModule.cache.Remove(97788) })
	forgetSentRoomExits(u.UserId)
	t.Cleanup(func() { forgetSentRoomExits(u.UserId) })
	study.AddPlayer(9788)

	drain := func() {
		events.DrainQueuedRoomExitsChangedForTest(0)
		for _, typ := range []string{`GMCPOut`, `GMCPRoomUpdate`, `GMCPZoneUpdate`} {
			events.DrainQueuedByTypeForTest(typ)
		}
	}
	drain()
	t.Cleanup(drain)
	return u, study
}

// studyExitTo reports whether the Zone.Map rooms draw an exit from the study
// to roomId.
func studyExitTo(p GMCPZoneModule_Payload, roomId int) bool {
	for _, r := range p.Rooms {
		if r.RoomId != 9785 {
			continue
		}
		for _, e := range r.Exits {
			if e.ToRoomId == roomId {
				return true
			}
		}
	}
	return false
}

// zoneUpdates drains the queued Zone.Map requests and counts userId's.
func zoneUpdates(userId int) int {
	n := 0
	for _, e := range events.DrainQueuedByTypeForTest(`GMCPZoneUpdate`) {
		if u, ok := e.(GMCPZoneUpdate); ok && u.UserId == userId {
			n++
		}
	}
	return n
}

// #781 Task 11b and the spec's "the exits line, Zone.Map and Room.Info
// agree" check, on one find: a new find queues one refresh, which sends
// Room.Info and Zone.Map at once (not at the next round or move), and the
// exits line, Room.Info and Zone.Map all show the exit. The Zone.Map is the
// changed study alone, not the whole map.
func TestSecretExitFound_ExitsLineRoomInfoAndZoneMapAgreeAtOnce(t *testing.T) {
	u, study := foundFixture(t)
	g := &GMCPZoneModule{zoneSent: map[int]*zoneSentState{}}

	// Before: the web client holds the whole map, and nothing shows west.
	g.buildAndSend(GMCPZoneUpdate{UserId: u.UserId})
	before := drainZoneMaps(t, u.UserId)
	require.Len(t, before, 1)
	require.True(t, before[0].Full)
	require.False(t, studyExitTo(before[0], 9787), "an unfound secret exit is not drawn")
	require.NotContains(t, roomInfoFor(t, u).ExitsV2, "west")
	require.NotContains(t, rooms.GetDetails(study, u).VisibleExits, "west")

	u.Character.LearnSecretExit(9785, "west")
	queued := events.DrainQueuedRoomExitsChangedForTest(u.UserId)
	require.Len(t, queued, 1, "a find queues one refresh")
	(&GMCPRoomModule{}).roomExitsChangedHandler(queued[0])
	require.Equal(t, 1, roomInfoUpdates(u.UserId), "Room.Info goes out at once")
	require.Equal(t, 1, zoneUpdates(u.UserId), "Zone.Map goes out at once")

	g.buildAndSend(GMCPZoneUpdate{UserId: u.UserId})
	after := drainZoneMaps(t, u.UserId)
	require.Len(t, after, 1)
	require.False(t, after[0].Full, "a find sends what changed, not the whole map")
	require.Equal(t, []int{9785}, zoneMapRoomIds(after[0]), "only the study changed")
	require.True(t, studyExitTo(after[0], 9787), "Zone.Map draws the found exit")
	require.Contains(t, roomInfoFor(t, u).ExitsV2, "west", "Room.Info lists it")
	require.Contains(t, rooms.GetDetails(study, u).VisibleExits, "west", "the exits line lists it")
}

// A dark mark refreshes Room.Info alone: the map draws no marks, so it is not
// re-sent.
func TestSecretExitFound_ADarkMarkSendsNoZoneMap(t *testing.T) {
	u, _ := foundFixture(t)
	(&GMCPRoomModule{}).roomExitsChangedHandler(events.RoomExitsChanged{UserId: u.UserId})
	require.Equal(t, 1, roomInfoUpdates(u.UserId))
	require.Zero(t, zoneUpdates(u.UserId))
}
