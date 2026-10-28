package mapper

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Zone maps stop at their zone's border (#375 phase 4, spec ruling 18,
// section 10): a crawl records only its own zone's rooms and keeps each
// border exit with the zone it leads to, every room belongs to its own zone's
// map whichever map was built first, and GetPath searches across maps so a
// route is the one a single map of the whole graph would give.

// isolateMapperCaches gives a test empty map and path caches and restores
// the package's own afterwards.
func isolateMapperCaches(t *testing.T) {
	t.Helper()
	prevZone, prevRoom := mapperZoneCache, roomIdToMapperCache
	ClearCache()
	pathCache.Purge()
	t.Cleanup(func() {
		mapperZoneCache, roomIdToMapperCache = prevZone, prevRoom
		pathCache.Purge()
	})
}

// fixtureRoom is one room of a written fixture world.
type fixtureRoom struct {
	id, x, y int
	zone     string
	exits    map[string]int
	symbol   string // mapsymbol, legend "Mark"; "" for the zone's biome
}

// writeFixtureWorld writes rooms to a temp data tree and seeds the zones they
// name, so LoadRoom and the crawl read them as they read shipped rooms.
func writeFixtureWorld(t *testing.T, rs []fixtureRoom) {
	t.Helper()
	dataDir := useTempDataFilesTB(t)
	zones := map[string]*rooms.ZoneConfig{}
	for _, r := range rs {
		if _, ok := zones[r.zone]; !ok {
			zones[r.zone] = &rooms.ZoneConfig{Name: r.zone, RoomId: r.id, DefaultBiome: "plains", RoomIds: map[int]struct{}{}}
		}
	}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{}, zones))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default", Name: "Default"},
		"plains":  {BiomeId: "plains", Name: "Plains", Symbol: "\""},
	}))
	for _, r := range rs {
		room := &rooms.Room{RoomId: r.id, Zone: r.zone, Title: fmt.Sprintf("Room %d", r.id), X: r.x, Y: r.y,
			Exits: map[string]exit.RoomExit{}}
		if r.symbol != "" {
			room.MapSymbol, room.MapLegend = r.symbol, "Mark"
		}
		for dir, to := range r.exits {
			room.Exits[dir] = ex(to, "")
		}
		writeRoomTemplate(t, dataDir, room)
	}
}

// A two-zone line: 9101 - 9102 | 9103 - 9104, the border between 9102 (zone
// zb_a) and 9103 (zone zb_b).
func twoZoneLine(t *testing.T) {
	writeFixtureWorld(t, []fixtureRoom{
		{id: 9101, x: 0, zone: "zb_a", exits: map[string]int{"east": 9102}},
		{id: 9102, x: 1, zone: "zb_a", exits: map[string]int{"west": 9101, "east": 9103}},
		{id: 9103, x: 2, zone: "zb_b", exits: map[string]int{"west": 9102, "east": 9104}},
		{id: 9104, x: 3, zone: "zb_b", exits: map[string]int{"west": 9103}},
	})
}

func sortedCrawl(m *mapper) []int {
	ids := m.CrawledRoomIds()
	sort.Ints(ids)
	return ids
}

// (a) The crawl holds only its own zone's rooms and keeps the border exit,
// with the zone it leads to.
func TestCrawlStopsAtZoneBorder(t *testing.T) {
	isolateMapperCaches(t)
	twoZoneLine(t)

	m := NewMapper(9101)
	m.Start()
	if got := sortedCrawl(m); fmt.Sprint(got) != "[9101 9102]" {
		t.Fatalf("zone zb_a's crawl holds %v, want [9101 9102]: it must stop at the border", got)
	}
	border, ok := m.crawledRooms[9102].Exits["east"]
	if !ok || border.RoomId != 9103 {
		t.Fatalf("border room 9102 lost its east exit to 9103: %+v", m.crawledRooms[9102].Exits)
	}
	if border.ToZone != "zb_b" {
		t.Fatalf("border exit ToZone = %q, want zb_b", border.ToZone)
	}
	if in := m.crawledRooms[9101].Exits["east"].ToZone; in != "zb_a" {
		t.Fatalf("in-zone exit ToZone = %q, want zb_a", in)
	}
}

// (b) A border room belongs to its own zone's map whichever map is built
// first.
func TestBorderRoomsOwnedByTheirOwnZone(t *testing.T) {
	for _, first := range []int{9101, 9104} {
		t.Run(fmt.Sprintf("first %d", first), func(t *testing.T) {
			isolateMapperCaches(t)
			twoZoneLine(t)

			second := 9104
			if first == 9104 {
				second = 9101
			}
			GetMapper(first)
			GetMapper(second)

			a, b := GetMapperIfExists(9102), GetMapperIfExists(9103)
			if a == nil || b == nil {
				t.Fatalf("both border rooms must belong to a map: 9102 %v, 9103 %v", a != nil, b != nil)
			}
			if a == b {
				t.Fatalf("9102 (zb_a) and 9103 (zb_b) share one map %v; each zone needs its own", sortedCrawl(a))
			}
			if got := sortedCrawl(a); fmt.Sprint(got) != "[9101 9102]" {
				t.Fatalf("zb_a's map holds %v, want [9101 9102]", got)
			}
			if got := sortedCrawl(b); fmt.Sprint(got) != "[9103 9104]" {
				t.Fatalf("zb_b's map holds %v, want [9103 9104]", got)
			}
			if GetMapperIfExists(9101) != a || GetMapperIfExists(9104) != b {
				t.Fatal("every room must be claimed by its own zone's map")
			}
		})
	}
}

// crossingWorld is a three-zone world whose shortest routes cross borders
// back and forth. Rows (x across, y down); zone in brackets:
//
//	base+1 [a]  base+11 [b]  base+12 [b]  base+5 [a]  base+21 [c]
//	base+2 [a]  base+3  [a]  base+4  [a]  base+6 [a]
//
// From base+1 to base+5 the short way (3 steps) runs through zone b; the
// long way (5 steps) stays in zone a. With single set, every room is in one
// zone "zs", the same graph as one map.
func crossingWorld(base int, single bool) []fixtureRoom {
	z := func(name string) string {
		if single {
			return "zs"
		}
		return name
	}
	id := func(n int) int { return base + n }
	return []fixtureRoom{
		{id: id(1), x: 0, y: 0, zone: z("zc_a"), exits: map[string]int{"east": id(11), "south": id(2)}},
		{id: id(11), x: 1, y: 0, zone: z("zc_b"), exits: map[string]int{"west": id(1), "east": id(12)}},
		{id: id(12), x: 2, y: 0, zone: z("zc_b"), exits: map[string]int{"west": id(11), "east": id(5)}},
		{id: id(5), x: 3, y: 0, zone: z("zc_a"), exits: map[string]int{"west": id(12), "south": id(6), "east": id(21)}},
		{id: id(21), x: 4, y: 0, zone: z("zc_c"), exits: map[string]int{"west": id(5)}},
		{id: id(2), x: 0, y: 1, zone: z("zc_a"), exits: map[string]int{"north": id(1), "east": id(3)}},
		{id: id(3), x: 1, y: 1, zone: z("zc_a"), exits: map[string]int{"west": id(2), "east": id(4)}},
		{id: id(4), x: 2, y: 1, zone: z("zc_a"), exits: map[string]int{"west": id(3), "east": id(6)}},
		{id: id(6), x: 3, y: 1, zone: z("zc_a"), exits: map[string]int{"west": id(4), "north": id(5)}},
	}
}

// pathShape renders a path relative to base, so two copies of one graph
// under different ids compare equal.
func pathShape(p []pathStep, base int) string {
	s := ""
	for _, st := range p {
		s += fmt.Sprintf("%s>%d,", st.exitName, st.roomId-base)
	}
	return s
}

// (c) GetPath across zones returns the path a single map of the same graph
// gives, waypoint legs included.
func TestGetPathAcrossZonesMatchesSingleMap(t *testing.T) {
	isolateMapperCaches(t)
	const multi, single = 9200, 9300
	writeFixtureWorld(t, append(crossingWorld(multi, false), crossingWorld(single, true)...))
	// Build every zone's map up front, as PreCacheMaps does at boot.
	for _, id := range []int{multi + 1, multi + 11, multi + 21, single + 1} {
		GetMapper(id)
	}

	// Every case has one shortest route: A* takes exits in map order, so two
	// routes of equal length would compare by chance.
	cases := []struct {
		name string
		from int
		to   []int
	}{
		{"a to a through b", 1, []int{5}},
		{"a to c through b and a", 1, []int{21}},
		{"c back to a", 21, []int{3}},
		{"b to a", 12, []int{4}},
		{"waypoints across three zones", 2, []int{11, 21, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			to := func(base int) []int {
				out := make([]int, len(c.to))
				for i, n := range c.to {
					out[i] = base + n
				}
				return out
			}
			want, err := GetPath(single+c.from, to(single)...)
			if err != nil {
				t.Fatalf("single-map path failed: %v", err)
			}
			got, err := GetPath(multi+c.from, to(multi)...)
			if err != nil {
				t.Fatalf("cross-zone GetPath failed: %v", err)
			}
			if pathShape(got, multi) != pathShape(want, single) {
				t.Fatalf("cross-zone path %s differs from the single-map path %s",
					pathShape(got, multi), pathShape(want, single))
			}
			for i := range got {
				if got[i].waypoint != want[i].waypoint {
					t.Fatalf("waypoint flags differ at step %d", i)
				}
			}
		})
	}
	if GetMapperIfExists(multi + 1).HasRoom(multi + 11) {
		t.Fatal("fixture premise: zone zc_a's map must not hold zone zc_b's rooms")
	}
}

// gridWorld writes a cols x rows grid of rooms at authored (x, y), id
// base + y*100 + x + 1, every room linked to its four neighbours. cell
// gives each cell's zone and symbol; ok false leaves the cell empty. Give
// each test its own id range: the template zone index (rooms.TemplateRoomZone,
// which the crawl reads) outlives a test, so a reused id keeps the zone an
// earlier test gave it.
func gridWorld(t *testing.T, base, cols, rows int, cell func(x, y int) (zone, symbol string, ok bool)) {
	t.Helper()
	id := func(x, y int) int { return base + y*100 + x + 1 }
	has := func(x, y int) bool {
		if x < 0 || y < 0 || x >= cols || y >= rows {
			return false
		}
		_, _, ok := cell(x, y)
		return ok
	}
	var rs []fixtureRoom
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			zone, symbol, ok := cell(x, y)
			if !ok {
				continue
			}
			exits := map[string]int{}
			for dir, d := range map[string][2]int{"east": {1, 0}, "west": {-1, 0}, "north": {0, -1}, "south": {0, 1}} {
				if has(x+d[0], y+d[1]) {
					exits[dir] = id(x+d[0], y+d[1])
				}
			}
			rs = append(rs, fixtureRoom{id: id(x, y), x: x, y: y, zone: zone, symbol: symbol, exits: exits})
		}
	}
	writeFixtureWorld(t, rs)
}

// renderText is a map render as text lines plus its legend, sorted.
func renderText(m mapRender) string {
	s := ""
	for _, line := range m.Render {
		s += string(line) + "|\n"
	}
	keys := make([]string, 0, len(m.legend))
	for r, name := range m.legend {
		keys = append(keys, fmt.Sprintf("%c=%s", r, name))
	}
	sort.Strings(keys)
	return s + fmt.Sprint(keys)
}

// The ASCII map's output for a two-zone fixture, recorded on the tree before
// #375 p4 Task 19 changed how GetLimitedMap reads rooms: zone zl_a
// (x 0-4, the biome's ") and zone zl_b (x 5-9, #), a hole at (3,1), the
// player at (3,2). The map still draws across the border, from the zone's
// map alone (the far zone's rooms loaded) or with every zone's map built.
func TestGetLimitedMapGolden(t *testing.T) {
	const base = 40000
	world := func(t *testing.T) {
		isolateMapperCaches(t)
		gridWorld(t, base, 10, 5, func(x, y int) (string, string, bool) {
			if x == 3 && y == 1 {
				return "", "", false
			}
			if x >= 5 {
				return "zl_b", "#", true
			}
			return "zl_a", "", true
		})
	}
	center := base + 2*100 + 3 + 1
	configs := []struct {
		name string
		c    Config
		want string
	}{
		{"minimap", Config{ZoomLevel: 1, Width: 5, Height: 5}, goldenMinimap},
		{"zoom 0", Config{ZoomLevel: 0, Width: 21, Height: 9}, goldenZoom0},
		{"zoom 1", Config{ZoomLevel: 1, Width: 21, Height: 9}, goldenZoom1},
	}
	for _, built := range []string{"own zone's map", "every zone's map"} {
		for _, cfg := range configs {
			t.Run(built+"/"+cfg.name, func(t *testing.T) {
				world(t)
				m := GetMapper(center)
				if built == "every zone's map" {
					GetMapper(base + 9 + 1)
				}
				c := cfg.c
				c.OverrideSymbol(center, '@', `You`)
				if got := renderText(m.GetLimitedMap(center, c)); got != cfg.want {
					t.Fatalf("render changed:\n%s\nwant:\n%s", got, cfg.want)
				}
			})
		}
	}
}

// Recorded on bac109a5f, before Task 19.
const goldenMinimap = `"   "|
│   │|
"─@─"|
│ │ │|
"─"─"|
["=Plains @=You]`

const goldenZoom0 = `                     |
                     |
       """""#####    |
       """ "#####    |
       """@"#####    |
       """""#####    |
       """""#####    |
                     |
                     |
["=Plains #=Mark @=You]`

const goldenZoom1 = `    "─"─"─"─"─#─#─#─#|
    │ │ │   │ │ │ │ │|
    "─"─"   "─#─#─#─#|
    │ │ │   │ │ │ │ │|
    "─"─"─@─"─#─#─#─#|
    │ │ │ │ │ │ │ │ │|
    "─"─"─"─"─#─#─#─#|
    │ │ │ │ │ │ │ │ │|
    "─"─"─"─"─#─#─#─#|
["=Plains #=Mark @=You]`

// GetLimitedMap's walk loads no room once every zone's map is built (#375 p4
// Task 19). It still walks every room reachable from the player, as on
// master (bounding it by the canvas changed shipped renders; see the comment
// in GetLimitedMap), but it reads a room across a border from that room's
// own zone's map, as master read every room from its one big map. Before,
// it reloaded every cold room across the border on each look and map.
func TestGetLimitedMapLoadsNoRoomWhenMapsAreBuilt(t *testing.T) {
	const base = 41000
	isolateMapperCaches(t)
	gridWorld(t, base, 12, 9, func(x, y int) (string, string, bool) {
		if x >= 2 {
			return "zr_b", "", true
		}
		return "zr_a", "", true
	})
	center := base + 4*100 + 1 + 1
	m := GetMapper(center)
	GetMapper(base + 2 + 1) // zr_b's map, as PreCacheMaps builds it
	var far []int
	for y := 0; y < 9; y++ {
		for x := 2; x < 12; x++ {
			far = append(far, base+y*100+x+1)
		}
	}
	for _, id := range far {
		rooms.UnloadRoomForTest(id)
	}

	got := renderText(m.GetLimitedMap(center, Config{ZoomLevel: 1, Width: 41, Height: 21}))
	loaded := 0
	for _, id := range far {
		if rooms.IsRoomLoaded(id) {
			loaded++
		}
	}
	if loaded != 0 {
		t.Fatalf("the walk loaded %d of zr_b's %d cold rooms; it must read them from zr_b's map", loaded, len(far))
	}
	if strings.Count(got, "\"") != 9*12+1 { // each room's biome mark and the legend's
		t.Fatalf("the map must still draw both zones' rooms:\n%s", got)
	}
}

// zoneGraphFromFixture rebuilds the zone graph from the written fixture and
// restores the package's graph afterwards.
func zoneGraphFromFixture(t *testing.T) {
	t.Helper()
	stubZoneGraph(t, nil)
	InvalidateZoneGraph()
}

// FindRoomsInDistance (call for help) finds rooms across a zone border within
// its radius, as it did when one map held both zones (#375 p4 Task 19).
func TestFindRoomsInDistanceCrossesZoneBorder(t *testing.T) {
	isolateMapperCaches(t)
	twoZoneLine(t)
	GetMapper(9101)
	GetMapper(9104)
	zoneGraphFromFixture(t)

	m := GetMapper(9102)
	for _, c := range []struct {
		radius int
		want   string
	}{{1, "[9101 9103]"}, {2, "[9101 9103 9104]"}} {
		got := m.FindRoomsInDistance(9102, c.radius, 0)
		sort.Ints(got)
		if fmt.Sprint(got) != c.want {
			t.Errorf("radius %d from 9102: %v, want %s", c.radius, got, c.want)
		}
	}
}

// Called with no z radius it searches the room's own level and does not
// panic (pre-existing: it indexed the empty option list).
func TestFindRoomsInDistanceWithoutZRadius(t *testing.T) {
	isolateMapperCaches(t)
	twoZoneLine(t)
	m := GetMapper(9101)
	var got []int
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("FindRoomsInDistance without a z radius panicked: %v", r)
			}
		}()
		got = m.FindRoomsInDistance(9101, 1)
	}()
	if fmt.Sprint(got) != "[9102]" {
		t.Fatalf("radius 1 from 9101: %v, want [9102]", got)
	}
}

// The distance is |dx| + |dy| (pre-existing: |dx + dy|, so a room at dx +2,
// dy -2 counted as distance 0).
func TestFindRoomsInDistanceIsManhattan(t *testing.T) {
	isolateMapperCaches(t)
	const base = 43000
	gridWorld(t, base, 5, 5, func(x, y int) (string, string, bool) { return "zm_one", "", true })
	id := func(x, y int) int { return base + y*100 + x + 1 }
	m := GetMapper(id(2, 2))
	got := map[int]bool{}
	for _, r := range m.FindRoomsInDistance(id(2, 2), 2, 0) {
		got[r] = true
	}
	for _, xy := range [][2]int{{4, 0}, {0, 4}, {0, 0}, {4, 4}, {4, 1}} {
		if got[id(xy[0], xy[1])] {
			t.Errorf("room at (%d,%d) is %d steps from (2,2), outside radius 2", xy[0], xy[1], absInt(xy[0]-2)+absInt(xy[1]-2))
		}
	}
	for _, xy := range [][2]int{{4, 2}, {2, 0}, {3, 1}, {1, 3}, {3, 3}, {1, 1}, {2, 3}} {
		if !got[id(xy[0], xy[1])] {
			t.Errorf("room at (%d,%d) is within radius 2 of (2,2) and was not found", xy[0], xy[1])
		}
	}
	if len(got) != 12 {
		t.Errorf("found %d rooms within radius 2 of a grid's centre, want 12", len(got))
	}
}
