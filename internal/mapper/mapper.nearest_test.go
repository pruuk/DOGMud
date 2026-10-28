package mapper

import "testing"

// A 3x3 grid, ids 1..9 row by row, every neighbour linked both ways:
//
//	1 2 3
//	4 5 6
//	7 8 9
func buildGridMapper() *mapper {
	m := NewMapper(1)
	m.crawledRooms = map[int]*mapNode{}
	for id := 1; id <= 9; id++ {
		x, y := (id-1)%3, (id-1)/3
		exits := map[string]nodeExit{}
		if x > 0 {
			exits["west"] = nodeExit{RoomId: id - 1}
		}
		if x < 2 {
			exits["east"] = nodeExit{RoomId: id + 1}
		}
		if y > 0 {
			exits["north"] = nodeExit{RoomId: id - 3}
		}
		if y < 2 {
			exits["south"] = nodeExit{RoomId: id + 3}
		}
		m.crawledRooms[id] = &mapNode{RoomId: id, Pos: positionDelta{x: x, y: y}, Exits: exits}
	}
	return m
}

func inSet(ids ...int) func(int) bool {
	s := map[int]bool{}
	for _, id := range ids {
		s[id] = true
	}
	return func(id int) bool { return s[id] }
}

func TestNearestIn_PicksTheClosestMember(t *testing.T) {
	m := buildGridMapper()
	pin, next, exit, ok := m.nearestIn(1, inSet(9, 3), nil)
	if !ok || pin != 3 || next != 2 || exit != "east" {
		t.Fatalf("from 1 toward {3,9}: got pin %d next %d %q ok %v; want 3 via 2 east", pin, next, exit, ok)
	}
}

func TestNearestIn_MatchesFindPathDistance(t *testing.T) {
	m := buildGridMapper()
	for goal := 2; goal <= 9; goal++ {
		path, err := m.findPath(1, goal)
		if err != nil {
			t.Fatal(err)
		}
		pin, _, _, ok := m.nearestIn(1, inSet(goal), nil)
		if !ok || pin != goal {
			t.Fatalf("goal %d: nearestIn found %d", goal, pin)
		}
		if got := m.distanceForTest(1, goal); got != len(path) {
			t.Errorf("goal %d: breadth-first distance %d, A* path %d", goal, got, len(path))
		}
	}
}

func TestNearestIn_TiesResolveTheSameWay(t *testing.T) {
	m := buildGridMapper()
	// 2 and 4 are both one step from 1; exits are taken in name order, east before south.
	for i := 0; i < 20; i++ {
		if pin, _, exit, _ := m.nearestIn(1, inSet(2, 4), nil); pin != 2 || exit != "east" {
			t.Fatalf("tie went to %d via %q", pin, exit)
		}
	}
}

func TestNearestIn_NoneAndSelf(t *testing.T) {
	m := buildGridMapper()
	if _, _, _, ok := m.nearestIn(1, inSet(42), nil); ok {
		t.Error("a set with no reachable room must not resolve")
	}
	if pin, _, _, ok := m.nearestIn(1, inSet(1, 5), nil); !ok || pin != 5 {
		t.Errorf("the start room is never the answer: got %d %v", pin, ok)
	}
	if _, _, _, ok := m.nearestIn(77, inSet(5), nil); ok {
		t.Error("a start room the map lacks must not resolve")
	}
}

func TestNearestZoneHop_FewestZones(t *testing.T) {
	npStub(t)
	l, ok := nearestZoneHop("Docks", []string{"Noble", "Old Quarter"}, nil)
	if !ok || l.toZone != "Old Quarter" || l.exitRoom != 5520 {
		t.Fatalf("want the Old Quarter border, got %+v %v", l, ok)
	}
	if _, ok := nearestZoneHop("Docks", []string{"Docks"}, nil); ok {
		t.Error("the start zone is not a hop")
	}
	if _, ok := nearestZoneHop("Docks", nil, nil); ok {
		t.Error("no zones, no hop")
	}
}

// The start zone's map is crawled only when the set touches that zone; a
// cross-zone set goes straight to the zone hop (#375 p4 review: a full crawl
// per move otherwise).
func TestSearchesHere_OnlyWhenTheSetTouchesTheStartZone(t *testing.T) {
	if !searchesHere("Docks", []string{"Docks", "Noble"}) {
		t.Error("a set touching the start zone must search it")
	}
	if searchesHere("Docks", []string{"Noble", "Old Quarter"}) {
		t.Error("a set outside the start zone must not crawl it")
	}
	if searchesHere("Docks", nil) {
		t.Error("no zones: nothing to search")
	}
}

// distanceForTest is the breadth-first step count from a to b (-1 if none).
func (r *mapper) distanceForTest(a, b int) int {
	dist := map[int]int{a: 0}
	queue := []int{a}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == b {
			return dist[cur]
		}
		for _, ex := range r.crawledRooms[cur].Exits {
			if _, seen := dist[ex.RoomId]; !seen {
				dist[ex.RoomId] = dist[cur] + 1
				queue = append(queue, ex.RoomId)
			}
		}
	}
	return -1
}

// zonedGrid is buildGridMapper with every room in zone za except those
// named, which are in zb; an exit into a zb room records zb as its ToZone.
// It stands for a map that holds a room of another zone (one built before
// maps stopped at their border, or an instance copy).
func zonedGrid(foreign ...int) *mapper {
	m := buildGridMapper()
	isForeign := inSet(foreign...)
	for id, n := range m.crawledRooms {
		n.Zone = "za"
		if isForeign(id) {
			n.Zone = "zb"
		}
	}
	for _, n := range m.crawledRooms {
		for name, e := range n.Exits {
			e.ToZone = m.crawledRooms[e.RoomId].Zone
			n.Exits[name] = e
		}
	}
	return m
}

// The destination search never steps into a room of another zone, even one
// its map holds (#375 p4 Task 17): from 4 to 6 the two-step way runs through
// 5, which is in zb, so the search goes round by 1, 2 and 3.
func TestNearestIn_NeverEntersAnotherZone(t *testing.T) {
	m := zonedGrid(5)
	if pin, _, _, ok := m.nearestIn(4, inSet(5), nil); ok {
		t.Fatalf("a room of another zone must not be the answer, got %d", pin)
	}
	pin, next, exit, ok := m.nearestIn(4, inSet(6), nil)
	if !ok || pin != 6 || next != 1 || exit != "north" {
		t.Fatalf("from 4 to 6 round zb's room 5: got pin %d next %d %q ok %v; want 6 via 1 north", pin, next, exit, ok)
	}
}

// Doubling the rooms outside the player's zone leaves the rooms the search
// visits the same (#375 p4 Task 17): its cost is bounded by the zone. The
// counting set never matches, so the search visits every room it can reach.
func TestNearestIn_CostDoesNotGrowOutsideTheZone(t *testing.T) {
	visits := func(m *mapper, from int) int {
		n := 0
		m.nearestIn(from, func(int) bool { n++; return false }, nil)
		return n
	}

	// A map holding a chain of foreign rooms east of room 3.
	handBuilt := func(outside int) int {
		m := zonedGrid()
		prev := 3
		for i := 0; i < outside; i++ {
			id := 100 + i
			m.crawledRooms[id] = &mapNode{RoomId: id, Zone: "zb", Exits: map[string]nodeExit{
				"west": {RoomId: prev, ToZone: "za"}}}
			m.crawledRooms[prev].Exits["east"] = nodeExit{RoomId: id, ToZone: "zb"}
			prev = id
		}
		return visits(m, 1)
	}
	if a, b := handBuilt(10), handBuilt(20); a != b || a != 8 {
		t.Errorf("map holding foreign rooms: %d visits with 10 outside, %d with 20; want 8 both", a, b)
	}

	// The shipped pipeline: written rooms, a zone's map from GetMapper.
	crawled := func(t *testing.T, outside int) int {
		isolateMapperCaches(t)
		rs := []fixtureRoom{
			{id: 9701, x: 0, zone: "zd_a", exits: map[string]int{"east": 9702}},
			{id: 9702, x: 1, zone: "zd_a", exits: map[string]int{"west": 9701, "east": 9710}},
		}
		for i := 0; i < outside; i++ {
			id := 9710 + i
			exits := map[string]int{"west": id - 1}
			if i == 0 {
				exits["west"] = 9702
			}
			if i < outside-1 {
				exits["east"] = id + 1
			}
			rs = append(rs, fixtureRoom{id: id, x: 2 + i, zone: "zd_b", exits: exits})
		}
		writeFixtureWorld(t, rs)
		return visits(GetMapper(9701), 9701)
	}
	var a, b int
	t.Run("10 outside", func(t *testing.T) { a = crawled(t, 10) })
	t.Run("20 outside", func(t *testing.T) { b = crawled(t, 20) })
	if a != b || a != 1 {
		t.Errorf("crawled map: %d visits with 10 outside, %d with 20; want 1 both", a, b)
	}
}

// borderPairWorld is two zones joined by two borders:
//
//	9611 [b] 9612 [b] 9613 [b] 9614 [b] 9615 [b]
//	9601 [a] 9602 [a] 9603 [a] 9604 [a] 9605 [a]
//
// 9601 and 9605 are the border rooms of zone zn_a toward zn_b.
func borderPairWorld(t *testing.T) {
	t.Helper()
	isolateMapperCaches(t)
	var rs []fixtureRoom
	for i := 0; i < 5; i++ {
		for _, row := range []struct {
			base, y int
			zone    string
			cross   string
		}{{9601, 1, "zn_a", "north"}, {9611, 0, "zn_b", "south"}} {
			id := row.base + i
			exits := map[string]int{}
			if i > 0 {
				exits["west"] = id - 1
			}
			if i < 4 {
				exits["east"] = id + 1
			}
			if i == 0 || i == 4 {
				other := 9611 + i
				if row.zone == "zn_b" {
					other = 9601 + i
				}
				exits[row.cross] = other
			}
			rs = append(rs, fixtureRoom{id: id, x: i, y: row.y, zone: row.zone, exits: exits})
		}
	}
	writeFixtureWorld(t, rs)
	GetMapper(9601)
	GetMapper(9611)
	stubZoneGraph(t, nil) // restores the package's graph afterwards
	InvalidateZoneGraph()
}

// The zone graph keeps every border link of a zone pair (#375 p4 Task 17).
func TestBuildZoneGraph_KeepsEveryBorderLink(t *testing.T) {
	borderPairWorld(t)
	ensureZoneGraph()
	zoneGraphMu.RLock()
	defer zoneGraphMu.RUnlock()
	exitRooms := map[int]string{}
	for _, l := range zoneGraph["zn_a"] {
		if l.toZone == "zn_b" {
			exitRooms[l.exitRoom] = l.exitName
		}
	}
	if len(exitRooms) != 2 || exitRooms[9601] != "north" || exitRooms[9605] != "north" {
		t.Fatalf("zn_a -> zn_b links %v; want both borders, 9601 and 9605 north", exitRooms)
	}
}

// A cross-zone destination's first hop leads to the NEAREST border room
// toward the next zone, and standing on a border room it is the crossing
// (#375 p4 Task 17).
func TestNearestOf_HeadsForTheNearestBorder(t *testing.T) {
	borderPairWorld(t)
	toB := inSet(9613)
	cases := []struct {
		from, next int
		exit       string
	}{
		{9602, 9601, "west"},
		{9604, 9605, "east"},
		{9601, 9611, "north"},
		{9605, 9615, "north"},
	}
	for _, c := range cases {
		pin, next, exit, ok := NearestOf(c.from, toB, []string{"zn_b"}, nil)
		if !ok || pin != 0 || next != c.next || exit != c.exit {
			t.Errorf("from %d: got pin %d next %d %q ok %v; want %d via %q", c.from, pin, next, exit, ok, c.next, c.exit)
		}
		if next, exit, ok := crossZoneHop(c.from, "zn_a", "zn_b"); !ok || next != c.next || exit != c.exit {
			t.Errorf("crossZoneHop from %d: got %d %q %v; want %d via %q", c.from, next, exit, ok, c.next, c.exit)
		}
	}
}
