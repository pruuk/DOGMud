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
	"github.com/stretchr/testify/require"
)

// buildZoneMapFixture seeds the builder's view of zone BA (plane 0 unless
// noted; x across, y down):
//
//	          9986 BA (2,-1), reached only through BB
//	9980 9981 9982 9983 9985 9984     BA BA | BB BB BB BB (x 0..5)
//	          9987 BB (2,1) plane 1
//	     9988 BC (1,1), joined to nothing
//
// BA's box is x 0..2, y -1..0; the builder adds same-plane rooms of other
// zones within 2 cells of it.
func buildZoneMapFixture(t *testing.T) {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false)
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default"},
	}))
	mk := func(id int, zone string, x, y, plane int, exits map[string]exit.RoomExit) *rooms.Room {
		return &rooms.Room{RoomId: id, Zone: zone, Biome: "default", Title: "Room",
			X: x, Y: y, Plane: plane, Exits: exits}
	}
	rs := []*rooms.Room{
		mk(9980, "BA", 0, 0, 0, map[string]exit.RoomExit{"east": {RoomId: 9981}}),
		mk(9981, "BA", 1, 0, 0, map[string]exit.RoomExit{"west": {RoomId: 9980}, "east": {RoomId: 9982}}),
		mk(9986, "BA", 2, -1, 0, map[string]exit.RoomExit{"south": {RoomId: 9982}}),
		mk(9982, "BB", 2, 0, 0, map[string]exit.RoomExit{"west": {RoomId: 9981}, "east": {RoomId: 9983},
			"north": {RoomId: 9986}, "south": {RoomId: 9987}}),
		mk(9983, "BB", 3, 0, 0, map[string]exit.RoomExit{"west": {RoomId: 9982}, "east": {RoomId: 9985}}),
		mk(9985, "BB", 4, 0, 0, map[string]exit.RoomExit{"west": {RoomId: 9983}, "east": {RoomId: 9984}}),
		mk(9984, "BB", 5, 0, 0, map[string]exit.RoomExit{"west": {RoomId: 9985}}),
		mk(9987, "BB", 2, 1, 1, map[string]exit.RoomExit{"north": {RoomId: 9982}}),
		mk(9988, "BC", 1, 1, 0, map[string]exit.RoomExit{}),
	}
	byId := map[int]*rooms.Room{}
	ids := func(zone string) map[int]struct{} {
		out := map[int]struct{}{}
		for _, r := range rs {
			if r.Zone == zone {
				out[r.RoomId] = struct{}{}
			}
		}
		return out
	}
	for _, r := range rs {
		byId[r.RoomId] = r
	}
	t.Cleanup(rooms.SeedRoomsForTest(byId, map[string]*rooms.ZoneConfig{
		"BA": {Name: "BA", RoomId: 9980, RoomIds: ids("BA")},
		"BB": {Name: "BB", RoomId: 9982, RoomIds: ids("BB")},
		"BC": {Name: "BC", RoomId: 9988, RoomIds: ids("BC")},
	}))
	// Every map, as PreCacheMaps builds them at boot.
	for _, r := range rs {
		mapper.GetMapper(r.RoomId)
	}
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPOut`) })
}

// buildZoneMapGolden is the builder's Zone.Map for BA that master sent,
// recorded on 8534cf6a1, where the forced crawl from BA's root walked every
// room an exit reached: BA's rooms (9986 included, reached through BB), BB's
// plane-0 rooms within 2 cells of BA's box as foreign, not 9984 (outside),
// 9987 (plane 1) or 9988 (no exit joins it).
const buildZoneMapGolden = `[{"num":9980,"x":0,"y":0,"z":0,"plane":0,"symbol":"•","biome":"","name":"Room","exits":[{"to":9981,"dx":1,"dy":0,"dz":0,"kind":"normal"}]},{"num":9981,"x":1,"y":0,"z":0,"plane":0,"symbol":"•","biome":"","name":"Room","exits":[{"to":9980,"dx":-1,"dy":0,"dz":0,"kind":"normal"},{"to":9982,"dx":1,"dy":0,"dz":0,"kind":"normal","tozone":"BB"}]},{"num":9982,"x":2,"y":0,"z":0,"plane":0,"symbol":"•","biome":"","name":"Room","exits":[{"to":9981,"dx":-1,"dy":0,"dz":0,"kind":"normal","tozone":"BA"},{"to":9983,"dx":1,"dy":0,"dz":0,"kind":"normal"},{"to":9986,"dx":0,"dy":-1,"dz":0,"kind":"normal","tozone":"BA"},{"to":9987,"dx":0,"dy":1,"dz":0,"kind":"normal","stub":true}],"foreign":true,"ozone":"BB"},{"num":9983,"x":3,"y":0,"z":0,"plane":0,"symbol":"•","biome":"","name":"Room","exits":[{"to":9982,"dx":-1,"dy":0,"dz":0,"kind":"normal"},{"to":9985,"dx":1,"dy":0,"dz":0,"kind":"normal"}],"foreign":true,"ozone":"BB"},{"num":9985,"x":4,"y":0,"z":0,"plane":0,"symbol":"•","biome":"","name":"Room","exits":[{"to":9983,"dx":-1,"dy":0,"dz":0,"kind":"normal"},{"to":9984,"dx":1,"dy":0,"dz":0,"kind":"normal","stub":true}],"foreign":true,"ozone":"BB"},{"num":9986,"x":2,"y":-1,"z":0,"plane":0,"symbol":"•","biome":"","name":"Room","exits":[{"to":9982,"dx":0,"dy":1,"dz":0,"kind":"normal","tozone":"BB"}]}]`

// The builder's zone view shows rooms of neighbouring zones near the zone's
// edge, dimmed, so a builder does not build into them. Zone maps stop at
// their border (#375 phase 4), so they come from the neighbours' own maps;
// the payload must be master's.
func TestSendZoneMapFull_ForeignRoomsMatchMaster(t *testing.T) {
	buildZoneMapFixture(t)
	sendZoneMapFull(99989, "BA")

	var snap []mapper.SnapshotRoom
	for _, e := range events.DrainQueuedByTypeForTest(`GMCPOut`) {
		o := e.(GMCPOut)
		if o.UserId == 99989 && o.Module == `Zone.Map` {
			p := o.Payload.(GMCPZoneModule_Payload)
			snap = p.Rooms
		}
	}
	require.NotNil(t, snap, "a Zone.Map must be sent")
	sort.Slice(snap, func(i, j int) bool { return snap[i].RoomId < snap[j].RoomId })
	for i := range snap {
		ex := snap[i].Exits
		sort.Slice(ex, func(a, b int) bool { return ex[a].ToRoomId < ex[b].ToRoomId })
	}
	b, err := json.Marshal(snap)
	require.NoError(t, err)
	require.Equal(t, buildZoneMapGolden, string(b))
}
