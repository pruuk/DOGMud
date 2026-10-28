package mapper

import (
	"fmt"
	"sort"
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
