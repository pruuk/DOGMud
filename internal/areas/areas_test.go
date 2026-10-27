package areas

import (
	"strings"
	"testing"
)

func testRooms() map[int]string { // room id -> zone
	return map[int]string{5311: "pothole_coulee", 5312: "pothole_coulee", 5313: "pothole_coulee", 100: "thornwall", 101: "thornwall"}
}

func TestResolveZonesAndRooms(t *testing.T) {
	a := Area{Key: "grove", Name: "the Grove", Zones: []string{"thornwall"}, Rooms: []int{5311, 5313}}
	set, warns, err := resolve(a, testRooms())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{100, 101, 5311, 5313} {
		if !set[id] {
			t.Errorf("room %d missing from %v", id, set)
		}
	}
	if set[5312] {
		t.Errorf("5312 is not in the area")
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings %v", warns)
	}
}

func TestResolveRefusesAndWarns(t *testing.T) {
	for _, c := range []struct {
		a    Area
		want string
	}{
		{Area{Key: "x", Name: "X", Zones: []string{"nowhere"}}, `unknown zone "nowhere"`},
		{Area{Key: "x", Name: "X", Rooms: []int{9}}, "unknown room 9"},
		{Area{Key: "x", Name: "X"}, "resolves to no rooms"},
		{Area{Key: "x"}, "has no name"},
	} {
		if _, _, err := resolve(c.a, testRooms()); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: want %q, got %v", c.a, c.want, err)
		}
	}
	_, warns, err := resolve(Area{Key: "x", Name: "X", Zones: []string{"thornwall"}, Rooms: []int{100}}, testRooms())
	if err != nil || len(warns) != 1 || !strings.Contains(warns[0], "room 100 is already in zone thornwall") {
		t.Errorf("want one overlap warning, got %v %v", warns, err)
	}
}

func TestIndexAndContains(t *testing.T) {
	r := newRegistry()
	r.add(Area{Key: "grove", Name: "the Grove", Rooms: []int{5311, 5312}}, map[int]bool{5311: true, 5312: true})
	r.add(Area{Key: "wood", Name: "the Wood", Rooms: []int{5312}}, map[int]bool{5312: true})
	if !r.contains("grove", 5311) || r.contains("wood", 5311) {
		t.Fatal("contains wrong")
	}
	if got := r.areasOf(5312); len(got) != 2 {
		t.Fatalf("areasOf(5312) = %v, want both", got)
	}
}
