package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
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

// sendInitialMap gives u's web client the whole map, before any find, and
// checks nothing shows the west exit yet.
func sendInitialMap(t *testing.T, g *GMCPZoneModule, u *users.UserRecord, study *rooms.Room) {
	t.Helper()
	g.buildAndSend(GMCPZoneUpdate{UserId: u.UserId})
	before := drainZoneMaps(t, u.UserId)
	require.Len(t, before, 1)
	require.True(t, before[0].Full)
	require.False(t, studyExitTo(before[0], 9787), "an unfound secret exit is not drawn")
	require.NotContains(t, roomInfoFor(t, u).ExitsV2, "west")
	require.NotContains(t, rooms.GetDetails(study, u).VisibleExits, "west")
}

// runFindRefresh plays the queue after a find, as MainWorker would: the one
// queued RoomExitsChanged goes to the handler, and the GMCPRoomUpdate and
// GMCPZoneUpdate it queued (the drained events themselves, not ones the test
// builds) go to their senders. It returns the Room.Info and Zone.Map sent.
func runFindRefresh(t *testing.T, g *GMCPZoneModule, u *users.UserRecord) (GMCPRoomModule_Payload, GMCPZoneModule_Payload) {
	t.Helper()
	queued := events.DrainQueuedRoomExitsChangedForTest(u.UserId)
	require.Len(t, queued, 1, "a find queues one refresh")
	require.True(t, queued[0].SecretExitFound)
	(&GMCPRoomModule{}).roomExitsChangedHandler(queued[0])

	roomUpdates := events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`)
	require.Len(t, roomUpdates, 1, "Room.Info goes out at once")
	zoneReqs := events.DrainQueuedByTypeForTest(`GMCPZoneUpdate`)
	require.Len(t, zoneReqs, 1, "Zone.Map goes out at once")
	(&GMCPRoomModule{}).buildAndSendGMCPPayload(roomUpdates[0])
	g.buildAndSend(zoneReqs[0])

	var info *GMCPRoomModule_Payload
	var zone *GMCPZoneModule_Payload
	for _, e := range events.DrainQueuedByTypeForTest(`GMCPOut`) {
		o := e.(GMCPOut)
		require.Equal(t, u.UserId, o.UserId)
		switch o.Module {
		case `Room.Info`:
			p := o.Payload.(GMCPRoomModule_Payload)
			require.Nil(t, info, "one Room.Info")
			info = &p
		case `Zone.Map`:
			p := o.Payload.(GMCPZoneModule_Payload)
			require.Nil(t, zone, "one Zone.Map")
			zone = &p
		}
	}
	require.NotNil(t, info, "Room.Info was sent")
	require.NotNil(t, zone, "Zone.Map was sent")
	return *info, *zone
}

// requireAllShowWest is the spec's "the exits line, Zone.Map and Room.Info
// agree" check, after one find. The map is a partial (the #650 diff); here,
// with the whole map already sent, that is the changed study alone.
func requireAllShowWest(t *testing.T, u *users.UserRecord, study *rooms.Room, info GMCPRoomModule_Payload, zone GMCPZoneModule_Payload) {
	t.Helper()
	require.False(t, zone.Full, "a find sends a partial map, not the whole map")
	require.Equal(t, []int{9785}, zoneMapRoomIds(zone), "only the study changed")
	require.True(t, studyExitTo(zone, 9787), "Zone.Map draws the found exit")
	require.Contains(t, info.ExitsV2, "west", "Room.Info lists it")
	require.Contains(t, rooms.GetDetails(study, u).VisibleExits, "west", "the exits line lists it")
}

// #781 Task 11b and the spec's "the exits line, Zone.Map and Room.Info
// agree" check, on one find: a new find queues one refresh, which sends
// Room.Info and Zone.Map at once (not at the next round or move), and the
// exits line, Room.Info and Zone.Map all show the exit.
func TestSecretExitFound_ExitsLineRoomInfoAndZoneMapAgreeAtOnce(t *testing.T) {
	u, study := foundFixture(t)
	g := &GMCPZoneModule{zoneSent: map[int]*zoneSentState{}}
	sendInitialMap(t, g, u, study)

	u.Character.LearnSecretExit(9785, "west")
	info, zone := runFindRefresh(t, g, u)
	requireAllShowWest(t, u, study, info, zone)
}

// End to end: a real `search` (actions.Search, through the player's own
// actor) finds the exit, and the events it queued produce a Room.Info and a
// Zone.Map that show it. The roll is pinned from outside actions by the
// shared seeded dice (util.SetRandForTest, the hook
// TestSearch_AFoundSecretExitIsNotRolledAgain uses), with a perception the
// study's one secret exit cannot resist under that seed.
func TestSecretExitFound_ARealSearchRefreshesTheMapAndButtons(t *testing.T) {
	u, study := foundFixture(t)
	g := &GMCPZoneModule{zoneSent: map[int]*zoneSentState{}}
	sendInitialMap(t, g, u, study)
	t.Cleanup(util.SetRandForTest(630))
	u.Character.Stats.Perception.ValueAdj = 100000

	result := actions.Search(actions.NewUserActorInRoom(u, study), actions.SearchOptions{})
	require.Equal(t, []string{"west"}, result.HiddenExitsFound, "the search finds the exit")
	info, zone := runFindRefresh(t, g, u)
	requireAllShowWest(t, u, study, info, zone)
}

// Two Zone.Map requests for one player in one tick (a find while following,
// plus the move) merge into one, so the snapshot is built and sent once.
// Another player's request is kept.
func TestGMCPZoneUpdate_TwoInOneTickSendOnce(t *testing.T) {
	u, study := foundFixture(t)
	g := &GMCPZoneModule{zoneSent: map[int]*zoneSentState{}}
	sendInitialMap(t, g, u, study)

	u.Character.LearnSecretExit(9785, "west")
	events.DrainQueuedRoomExitsChangedForTest(u.UserId)
	events.AddToQueue(GMCPZoneUpdate{UserId: u.UserId})
	events.AddToQueue(GMCPZoneUpdate{UserId: u.UserId})
	events.AddToQueue(GMCPZoneUpdate{UserId: 9789})
	reqs := events.DrainQueuedByTypeForTest(`GMCPZoneUpdate`)
	require.ElementsMatch(t, []events.Event{GMCPZoneUpdate{UserId: u.UserId}, GMCPZoneUpdate{UserId: 9789}}, reqs)

	for _, r := range reqs {
		g.buildAndSend(r)
	}
	maps := drainZoneMaps(t, u.UserId)
	require.Len(t, maps, 1, "one send")
	require.True(t, studyExitTo(maps[0], 9787))
}

// A dark mark refreshes Room.Info alone: the map draws no marks, so it is not
// re-sent.
func TestSecretExitFound_ADarkMarkSendsNoZoneMap(t *testing.T) {
	u, _ := foundFixture(t)
	(&GMCPRoomModule{}).roomExitsChangedHandler(events.RoomExitsChanged{UserId: u.UserId})
	require.Equal(t, 1, roomInfoUpdates(u.UserId))
	require.Zero(t, zoneUpdates(u.UserId))
}
