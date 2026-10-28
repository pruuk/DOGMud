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
	pin, next, exit, ok := m.nearestIn(1, inSet(9, 3))
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
		pin, _, _, ok := m.nearestIn(1, inSet(goal))
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
		if pin, _, exit, _ := m.nearestIn(1, inSet(2, 4)); pin != 2 || exit != "east" {
			t.Fatalf("tie went to %d via %q", pin, exit)
		}
	}
}

func TestNearestIn_NoneAndSelf(t *testing.T) {
	m := buildGridMapper()
	if _, _, _, ok := m.nearestIn(1, inSet(42)); ok {
		t.Error("a set with no reachable room must not resolve")
	}
	if pin, _, _, ok := m.nearestIn(1, inSet(1, 5)); !ok || pin != 5 {
		t.Errorf("the start room is never the answer: got %d %v", pin, ok)
	}
	if _, _, _, ok := m.nearestIn(77, inSet(5)); ok {
		t.Error("a start room the map lacks must not resolve")
	}
}

func TestNearestZoneHop_FewestZones(t *testing.T) {
	npStub(t)
	l, ok := nearestZoneHop("Docks", []string{"Noble", "Old Quarter"})
	if !ok || l.toZone != "Old Quarter" || l.exitRoom != 5520 {
		t.Fatalf("want the Old Quarter border, got %+v %v", l, ok)
	}
	if _, ok := nearestZoneHop("Docks", []string{"Docks"}); ok {
		t.Error("the start zone is not a hop")
	}
	if _, ok := nearestZoneHop("Docks", nil); ok {
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
