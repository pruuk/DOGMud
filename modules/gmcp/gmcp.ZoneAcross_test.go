package gmcp

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// zoneAcrossFixture seeds two joined zones and one apart, all lit:
//
//	9956 - 9955 | 9950 - 9951        9958 (zone WebC, no exit to the rest)
//	   WebB     |    WebA
//
// The player stands in 9950 and has visited every room. 9951 has a north exit
// to 9952 (WebA, unvisited), so the fog stub is in the fixture too.
func zoneAcrossFixture(t *testing.T) *users.UserRecord {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false)
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default"},
	}))
	zero := 0.0
	lit := func(id int, zone string, x int, exits map[string]exit.RoomExit) *rooms.Room {
		return &rooms.Room{RoomId: id, Zone: zone, Biome: "default", SkyLight: &zero,
			Lamp: rooms.LampPtr(60), Title: "Room", X: x, Exits: exits}
	}
	a0 := lit(9950, "WebA", 0, map[string]exit.RoomExit{"west": {RoomId: 9955}, "east": {RoomId: 9951}})
	a1 := lit(9951, "WebA", 1, map[string]exit.RoomExit{"west": {RoomId: 9950}, "north": {RoomId: 9952}})
	a2 := lit(9952, "WebA", 1, map[string]exit.RoomExit{"south": {RoomId: 9951}})
	a2.Y = -1
	b0 := lit(9955, "WebB", -1, map[string]exit.RoomExit{"east": {RoomId: 9950}, "west": {RoomId: 9956}})
	b1 := lit(9956, "WebB", -2, map[string]exit.RoomExit{"east": {RoomId: 9955}})
	c0 := lit(9958, "WebC", 5, map[string]exit.RoomExit{})
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9950: a0, 9951: a1, 9952: a2, 9955: b0, 9956: b1, 9958: c0},
		map[string]*rooms.ZoneConfig{
			"WebA": {Name: "WebA", RoomId: 9950, RoomIds: map[int]struct{}{9950: {}, 9951: {}, 9952: {}}},
			"WebB": {Name: "WebB", RoomId: 9955, RoomIds: map[int]struct{}{9955: {}, 9956: {}}},
			"WebC": {Name: "WebC", RoomId: 9958, RoomIds: map[int]struct{}{9958: {}}},
		},
	))
	// Every zone's map, as PreCacheMaps builds them at boot.
	for _, id := range []int{9950, 9955, 9958} {
		require.NotNil(t, mapper.GetMapper(id, true))
	}

	u := users.NewTestUser(9957, "across", "Across", 97957)
	u.Character.RoomId = 9950
	for _, v := range []struct {
		zone string
		id   int
	}{{"WebA", 9950}, {"WebA", 9951}, {"WebB", 9955}, {"WebB", 9956}, {"WebC", 9958}} {
		u.Character.MarkRoomVisited(v.zone, v.id)
	}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9957: u}))
	web := GMCPSettings{GMCPAccepted: true, HelloReceived: true}
	web.Client.Name = `WebClient`
	gmcpModule.cache.Add(97957, web)
	t.Cleanup(func() { gmcpModule.cache.Remove(97957) })
	a0.AddPlayer(9957)
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPOut`) })
	return u
}

// zoneAcrossGolden is the Zone.Map rooms master sent for zoneAcrossFixture,
// recorded on 8534cf6a1 (one map crawled every zone an exit reached): the
// explored rooms of WebB with WebA's, the border 9950 <-> 9955 a full edge
// labelled with the far zone, and WebC (joined to nothing) left out.
const zoneAcrossGolden = `[{"num":9950,"x":0,"y":0,"z":0,"plane":0,"symbol":"•","biome":"","name":"Room","exits":[{"to":9951,"dx":1,"dy":0,"dz":0,"kind":"normal"},{"to":9955,"dx":-1,"dy":0,"dz":0,"kind":"normal","tozone":"WebB"}]},{"num":9951,"x":1,"y":0,"z":0,"plane":0,"symbol":"•","biome":"","name":"Room","exits":[{"to":9950,"dx":-1,"dy":0,"dz":0,"kind":"normal"},{"to":9952,"dx":0,"dy":-1,"dz":0,"kind":"normal","stub":true}]},{"num":9955,"x":-1,"y":0,"z":0,"plane":0,"symbol":"•","biome":"","name":"Room","exits":[{"to":9950,"dx":1,"dy":0,"dz":0,"kind":"normal","tozone":"WebA"},{"to":9956,"dx":-1,"dy":0,"dz":0,"kind":"normal"}]},{"num":9956,"x":-2,"y":0,"z":0,"plane":0,"symbol":"•","biome":"","name":"Room","exits":[{"to":9955,"dx":1,"dy":0,"dz":0,"kind":"normal"}]}]`

// The owner's 2026-07-30 rule: the web map shows every room the player has
// explored whatever zone it is in. Zone maps stop at their border (#375
// phase 4, ruling 18), so the snapshot gathers the rooms from each joined
// zone's own map; the payload must be master's.
func TestZoneMap_ExploredRoomsOfAJoinedZoneMatchMaster(t *testing.T) {
	u := zoneAcrossFixture(t)
	g := &GMCPZoneModule{zoneSent: map[int]*zoneSentState{}}

	g.buildAndSend(GMCPZoneUpdate{UserId: u.UserId})
	maps := drainZoneMaps(t, u.UserId)
	require.Len(t, maps, 1)
	require.True(t, maps[0].Full)
	require.Equal(t, "WebA", maps[0].Zone)

	snap := maps[0].Rooms
	sort.Slice(snap, func(i, j int) bool { return snap[i].RoomId < snap[j].RoomId })
	for i := range snap {
		ex := snap[i].Exits
		sort.Slice(ex, func(a, b int) bool { return ex[a].ToRoomId < ex[b].ToRoomId })
	}
	b, err := json.Marshal(snap)
	require.NoError(t, err)
	require.Equal(t, zoneAcrossGolden, string(b))
}
