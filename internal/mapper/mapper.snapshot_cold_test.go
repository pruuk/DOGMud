package mapper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"gopkg.in/yaml.v2"
)

// Zone.Map is rebuilt on every room change a web-client player makes, over
// every visited room the zone crawl reached (790 to 1,209 rooms on the
// overworld). Snapshot used to call rooms.LoadRoom per room, so a room that
// idle unloading had dropped was read and parsed from disk on the event loop:
// ~150 ms locally for 960 cold rooms, and the owner's "lag when moving area
// to area" on the 1-vCPU droplet. These tests pin that a snapshot never loads
// a room, and that the payload is byte-identical to what the loading version
// produced, cold or warm.

// The room loader logs through mudlog, which panics on a nil logger.
func init() {
	mudlog.SetupLogger(nil, "", "", false)
}

// useTempDataFilesTB points FilePaths.DataFiles at a temp tree for the test.
func useTempDataFilesTB(tb testing.TB) string {
	tb.Helper()
	dir := tb.TempDir()
	prev := configs.GetFilePathsConfig()
	if err := configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": dir}); err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		_ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": prev.DataFiles.String()})
	})
	return dir
}

func writeRoomTemplate(tb testing.TB, dataDir string, r *rooms.Room) {
	tb.Helper()
	if r.Description == "" {
		r.Description = "A fixture room." // the loader rejects an empty description
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		tb.Fatal(err)
	}
	p := filepath.Join(dataDir, "rooms", r.Zone, fmt.Sprintf("%d.yaml", r.RoomId))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0644); err != nil {
		tb.Fatal(err)
	}
}

func ex(to int, dir string) exit.RoomExit { return exit.RoomExit{RoomId: to, MapDirection: dir} }

var zoneMapFixtureIds = []int{9001, 9002, 9003, 9004, 9005, 9006}

// setupZoneMapFixture writes a small two-zone world to disk, crawls each
// zone's map (which loads every room, as the boot-time PreCacheMaps does), and
// returns zone zm_a's map (from 9001), zone zm_b's map (from 9005) and the
// player's fog set.
//
//	9006 Far            (unvisited: fog stub from 9002)
//	9002 Bank           isbank, biome forest
//	9005 9001 9003      9005 is zone zm_b (cross-zone exits both ways);
//	                    9001 has a mapsymbol; 9003 isstorage, locked exit back
//	9004 Hidden         behind a SECRET exit from 9001, unvisited (omitted)
//
// A zone's map stops at its border (#375 phase 4, ruling 18), so 9005 is in
// zm_b's map only and each side of the 9001/9005 border is a stub in its own
// zone's map, still labelled with the far zone.
func setupZoneMapFixture(t *testing.T) (*mapper, *mapper, map[int]struct{}) {
	t.Helper()
	dataDir := useTempDataFilesTB(t)

	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{}, map[string]*rooms.ZoneConfig{
		"zm_a": {Name: "zm_a", RoomId: 9001, DefaultBiome: "plains", RoomIds: map[int]struct{}{}},
		"zm_b": {Name: "zm_b", RoomId: 9005, RoomIds: map[int]struct{}{}},
	}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default", Name: "Default"},
		"plains":  {BiomeId: "plains", Name: "Plains", Symbol: "\""},
		"forest":  {BiomeId: "forest", Name: "Forest", Symbol: "♣"},
	}))

	square := &rooms.Room{RoomId: 9001, Zone: "zm_a", Title: "Square", MapSymbol: "T",
		Exits: map[string]exit.RoomExit{
			"north": ex(9002, ""), "east": ex(9003, ""), "west": ex(9005, ""),
			"south": {RoomId: 9004, Secret: true},
		}}
	bank := &rooms.Room{RoomId: 9002, Zone: "zm_a", Title: "Bank", IsBank: true, Biome: "forest", Y: -1,
		Exits: map[string]exit.RoomExit{"south": ex(9001, ""), "north": ex(9006, "")}}
	store := &rooms.Room{RoomId: 9003, Zone: "zm_a", Title: "Store", IsStorage: true, X: 1,
		Exits: map[string]exit.RoomExit{"west": {RoomId: 9001, Lock: gamelock.Lock{Difficulty: 20}}}}
	hidden := &rooms.Room{RoomId: 9004, Zone: "zm_a", Title: "Hidden", Y: 1,
		Exits: map[string]exit.RoomExit{"north": ex(9001, "")}}
	neighbour := &rooms.Room{RoomId: 9005, Zone: "zm_b", Title: "Neighbour", Biome: "forest", X: -1,
		Exits: map[string]exit.RoomExit{"east": ex(9001, "")}}
	far := &rooms.Room{RoomId: 9006, Zone: "zm_a", Title: "Far", Y: -2,
		Exits: map[string]exit.RoomExit{"south": ex(9002, "")}}
	for _, r := range []*rooms.Room{square, bank, store, hidden, neighbour, far} {
		writeRoomTemplate(t, dataDir, r)
	}

	m := NewMapper(9001)
	m.Start()
	if got := fmt.Sprint(sortedCrawl(m)); got != "[9001 9002 9003 9004 9006]" {
		t.Fatalf("zone zm_a's crawl reached %s, want [9001 9002 9003 9004 9006]", got)
	}
	mb := NewMapper(9005)
	mb.Start()
	if got := fmt.Sprint(sortedCrawl(mb)); got != "[9005]" {
		t.Fatalf("zone zm_b's crawl reached %s, want [9005]", got)
	}
	return m, mb, map[int]struct{}{9001: {}, 9002: {}, 9003: {}, 9005: {}}
}

// zoneMapSnapshot is both zones' snapshots for the fixture's fog set, every
// room of the fixture the player has seen.
func zoneMapSnapshot(m, mb *mapper, visited map[int]struct{}) []SnapshotRoom {
	return append(m.Snapshot(visited), mb.Snapshot(visited)...)
}

// canonicalSnapshot sorts rooms and exits (Snapshot iterates maps) and returns
// the JSON the client would receive for them.
func canonicalSnapshot(t *testing.T, snap []SnapshotRoom) string {
	t.Helper()
	sort.Slice(snap, func(i, j int) bool { return snap[i].RoomId < snap[j].RoomId })
	for i := range snap {
		ex := snap[i].Exits
		sort.Slice(ex, func(a, b int) bool { return ex[a].ToRoomId < ex[b].ToRoomId })
	}
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// zoneMapGolden is the payload the pre-fix Snapshot (rooms.LoadRoom per room
// and per exit destination) produced for this fixture, captured on master
// 5ab3d54c2 before the change. Fog: 9004 (secret, unvisited) is omitted, 9006
// is a stub. Cross-zone: 9001 <-> 9005 carry tozone.
//
// Since zone maps stop at their border (#375 phase 4, ruling 18) the payload
// is the two zones' snapshots together, and the two border exits 9001 -> 9005
// and 9005 -> 9001 gain "stub":true (the far room is in the other zone's map);
// every other byte is the loading version's. Their tozone comes from the zone
// the exit recorded at crawl time, so it survives the far room being unloaded.
const zoneMapGolden = `[{"num":9001,"x":0,"y":0,"z":0,"plane":0,"symbol":"T","biome":"Plains","name":"Square","exits":[{"to":9002,"dx":0,"dy":-1,"dz":0,"kind":"normal"},{"to":9003,"dx":1,"dy":0,"dz":0,"kind":"normal"},{"to":9005,"dx":-1,"dy":0,"dz":0,"kind":"normal","stub":true,"tozone":"zm_b"}]},{"num":9002,"x":0,"y":-1,"z":0,"plane":0,"symbol":"♣","biome":"Forest","name":"Bank","tags":["bank"],"exits":[{"to":9001,"dx":0,"dy":1,"dz":0,"kind":"normal"},{"to":9006,"dx":0,"dy":-1,"dz":0,"kind":"normal","stub":true}]},{"num":9003,"x":1,"y":0,"z":0,"plane":0,"symbol":"\"","biome":"Plains","name":"Store","tags":["storage"],"exits":[{"to":9001,"dx":-1,"dy":0,"dz":0,"kind":"normal","locked":true}]},{"num":9005,"x":-1,"y":0,"z":0,"plane":0,"symbol":"♣","biome":"Forest","name":"Neighbour","exits":[{"to":9001,"dx":1,"dy":0,"dz":0,"kind":"normal","stub":true,"tozone":"zm_a"}]}]`

// TestSnapshot_LoadedRoomReadsLiveState pins that a room still in memory is
// read live, not from the crawl-time summary: the shop tag comes from the mobs
// standing in the room right now, so a loaded room must keep that path. A
// change made only to the in-memory room shows; the disk and node never saw it.
func TestSnapshot_LoadedRoomReadsLiveState(t *testing.T) {
	m, _, visited := setupZoneMapFixture(t)
	live := rooms.LoadRoom(9001) // still loaded from the crawl: a memory hit
	live.Title = "Square (live)"
	live.IsBank = true

	var sq *SnapshotRoom
	snap := m.Snapshot(visited)
	for i := range snap {
		if snap[i].RoomId == 9001 {
			sq = &snap[i]
		}
	}
	if sq == nil {
		t.Fatal("room 9001 missing from snapshot")
	}
	if sq.Name != "Square (live)" || len(sq.Tags) != 1 || sq.Tags[0] != "bank" {
		t.Fatalf("loaded room must be read live; got name %q tags %v", sq.Name, sq.Tags)
	}
}

func TestSnapshot_WarmPayloadMatchesGolden(t *testing.T) {
	m, mb, visited := setupZoneMapFixture(t)
	if got := canonicalSnapshot(t, zoneMapSnapshot(m, mb, visited)); got != zoneMapGolden {
		t.Fatalf("warm Zone.Map payload changed.\n got: %s\nwant: %s", got, zoneMapGolden)
	}
}

// TestSnapshot_ColdRoomsStayUnloaded is the lag regression: a snapshot over
// rooms idle unloading has dropped must not read them back from disk, and must
// still send exactly the payload the loading version sent.
func TestSnapshot_ColdRoomsStayUnloaded(t *testing.T) {
	m, mb, visited := setupZoneMapFixture(t)
	for _, id := range zoneMapFixtureIds {
		rooms.UnloadRoomForTest(id)
	}

	got := canonicalSnapshot(t, zoneMapSnapshot(m, mb, visited))

	var loaded []string
	for _, id := range zoneMapFixtureIds {
		if rooms.IsRoomLoaded(id) {
			loaded = append(loaded, fmt.Sprint(id))
		}
	}
	if len(loaded) > 0 {
		t.Errorf("Snapshot loaded unloaded room(s) %s from disk; it must read the mapper's own nodes",
			strings.Join(loaded, ", "))
	}
	if got != zoneMapGolden {
		t.Errorf("cold Zone.Map payload differs from the loading version.\n got: %s\nwant: %s", got, zoneMapGolden)
	}
}

// BenchmarkSnapshotCold960 times one Zone.Map snapshot over 960 visited rooms
// that are all unloaded, the shape of an overworld move ~30 minutes after the
// player last passed through. Room files carry a ~1 KB description so a
// disk reload costs about what a real room does.
func BenchmarkSnapshotCold960(b *testing.B) {
	dataDir := useTempDataFilesTB(b)
	b.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{}, map[string]*rooms.ZoneConfig{
		"bench": {Name: "bench", RoomId: 20000, DefaultBiome: "plains", RoomIds: map[int]struct{}{}},
	}))
	b.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default", Name: "Default"},
		"plains":  {BiomeId: "plains", Name: "Plains", Symbol: "\""},
	}))

	const w, h = 32, 30
	desc := strings.Repeat("The grass bends under a steady wind off the steppe. ", 20)
	id := func(x, y int) int { return 20000 + y*w + x }
	ids := []int{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := &rooms.Room{RoomId: id(x, y), Zone: "bench", Title: fmt.Sprintf("Steppe %d,%d", x, y),
				Description: desc, X: x, Y: y, Exits: map[string]exit.RoomExit{}}
			if x > 0 {
				r.Exits["west"] = ex(id(x-1, y), "")
			}
			if x < w-1 {
				r.Exits["east"] = ex(id(x+1, y), "")
			}
			if y > 0 {
				r.Exits["north"] = ex(id(x, y-1), "")
			}
			if y < h-1 {
				r.Exits["south"] = ex(id(x, y+1), "")
			}
			writeRoomTemplate(b, dataDir, r)
			ids = append(ids, r.RoomId)
		}
	}

	m := NewMapper(id(0, 0))
	m.Start()
	visited := map[int]struct{}{}
	for _, rid := range ids {
		visited[rid] = struct{}{}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		for _, rid := range ids {
			rooms.UnloadRoomForTest(rid)
		}
		b.StartTimer()
		if snap := m.Snapshot(visited); len(snap) != len(ids) {
			b.Fatalf("snapshot rooms = %d, want %d", len(snap), len(ids))
		}
	}
}
