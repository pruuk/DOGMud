package scavenger

import (
	"strings"
	"testing"
	"time"
)

// fakeWorld is a two-zone world: zone "Town" holds rooms 1-6, zone "Wilds"
// room 9. Room 6 cannot be reached from anywhere.
func fakeWorld() World {
	biomes := map[int]string{
		1: `city_thoroughfare`, 2: `city_backstreet`, 3: `interior`, 4: `cave`, 5: `water`, 6: `interior`,
		9: `forest`,
	}
	return World{
		ZoneExists: func(z string) bool { return z == `Town` || z == `Wilds` },
		ZoneRooms: func(z string) []int {
			if z == `Town` {
				return []int{1, 2, 3, 4, 5, 6}
			}
			return []int{9}
		},
		RoomBiome: func(id int) (string, bool) { b, ok := biomes[id]; return b, ok },
		Reachable: func(from, to int) bool { return to != 6 },
		MobExists: func(id int) bool { return id == 9820 || id == 9821 },
	}
}

const goodLines = `
    pickup_lines: ["{actor} takes the {item}."]
    gold_lines: ["{actor} pockets {gold}."]
    reset_lines: ["{actor} hands in the day's haul."]`

// The pool is the zone's rooms in the default city biomes, minus the
// excluded and the unreachable.
func TestParse_ResolvesPoolFromZonesAndBiomes(t *testing.T) {
	raw := `
scavengers:
  - mobid: 9820
    city: Town
    zones: [Town]
    home_room: 1
    exclude_rooms: [3]` + goodLines

	ps, err := Parse([]byte(raw), fakeWorld())
	if err != nil {
		t.Fatal(err)
	}
	// 1, 2 are streets; 3 is excluded; 4 is a cave and 5 water (not default
	// biomes); 6 is unreachable.
	if got := ps[0].Pool(); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("pool %v, want [1 2]", got)
	}
}

// Named biomes replace the default set; include_rooms replaces the zone scan.
func TestParse_BiomesAndIncludeRooms(t *testing.T) {
	raw := `
scavengers:
  - mobid: 9820
    city: Quays
    zones: [Town]
    biomes: [water, city_thoroughfare]
    home_room: 1` + goodLines + `
  - mobid: 9821
    city: Valley
    zones: [Town]
    include_rooms: [1, 4, 9]
    home_room: 4` + goodLines

	ps, err := Parse([]byte(raw), fakeWorld())
	if err != nil {
		t.Fatal(err)
	}
	if got := ps[0].Pool(); len(got) != 2 || got[0] != 1 || got[1] != 5 {
		t.Fatalf("quays pool %v, want [1 5]", got)
	}
	if got := ps[1].Pool(); len(got) != 3 || got[0] != 1 || got[1] != 4 || got[2] != 9 {
		t.Fatalf("valley pool %v, want [1 4 9]", got)
	}
}

// Authoring mistakes fail loudly.
func TestParse_RejectsBadProfiles(t *testing.T) {
	cases := map[string]string{
		`unknown mob`: `
  - mobid: 1234
    zones: [Town]
    home_room: 1` + goodLines,
		`unknown zone`: `
  - mobid: 9820
    zones: [Nowhere]
    home_room: 1` + goodLines,
		`home outside pool`: `
  - mobid: 9820
    zones: [Town]
    home_room: 4` + goodLines,
		`home excluded`: `
  - mobid: 9820
    zones: [Town]
    exclude_rooms: [1]
    home_room: 1` + goodLines,
		`duplicate mob`: `
  - mobid: 9820
    zones: [Town]
    home_room: 1` + goodLines + `
  - mobid: 9820
    zones: [Town]
    home_room: 1` + goodLines,
		`line without the item`: `
  - mobid: 9820
    zones: [Town]
    home_room: 1
    pickup_lines: ["{actor} takes something."]
    gold_lines: ["{actor} pockets {gold}."]
    reset_lines: ["{actor} hands in the day's haul."]`,
		`unknown key`: `
  - mobid: 9820
    zones: [Town]
    home_room: 1
    wander_speed: 3` + goodLines,
		`pool too small`: `
  - mobid: 9820
    zones: [Town]
    include_rooms: [1]
    home_room: 1` + goodLines,
	}
	for name, body := range cases {
		if _, err := Parse([]byte("scavengers:"+body), fakeWorld()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Loaded pools are what the floor decay exempts.
func TestInstall_PatrolledRooms(t *testing.T) {
	p := &Profile{MobId: 9820, HomeRoom: 1}
	p.SetPoolForTest([]int{1, 2})
	defer SetProfilesForTest(p)()

	if !IsPatrolledRoom(2) || IsPatrolledRoom(3) {
		t.Fatal("patrolled rooms are the pools")
	}
	if ProfileFor(9820) != p || ProfileFor(1) != nil {
		t.Fatal("ProfileFor")
	}
	if a := AnchorRooms(); len(a) != 1 || a[0] != 1 {
		t.Fatalf("anchors %v", a)
	}
}

func TestStepDelay_InRange(t *testing.T) {
	if d := StepDelay(30, 150, func(int) int { return 0 }); d != 30*time.Second {
		t.Fatalf("low end %v", d)
	}
	if d := StepDelay(30, 150, func(n int) int { return n - 1 }); d != 150*time.Second {
		t.Fatalf("high end %v", d)
	}
	if d := StepDelay(60, 10, func(n int) int { return n - 1 }); d != 60*time.Second {
		t.Fatalf("max below min holds at min: %v", d)
	}
}

func TestPickTarget_NeverTheCurrentRoom(t *testing.T) {
	for i := 0; i < 3; i++ {
		if got := PickTarget([]int{1, 2, 3}, 2, func(int) int { return i % 2 }); got == 2 || got == 0 {
			t.Fatalf("picked %d", got)
		}
	}
	if got := PickTarget([]int{5}, 5, func(int) int { return 0 }); got != 0 {
		t.Fatalf("nowhere else to go: %d", got)
	}
}

// A step that went nowhere twice in a row gives up the target.
func TestWalk_NoteArrival(t *testing.T) {
	w := &Walk{Target: 9, LastFrom: 1, Expect: 2}
	w.NoteArrival(2)
	if w.Fails != 0 || w.Expect != 0 || w.Target != 9 {
		t.Fatalf("a good step: %+v", w)
	}

	w.LastFrom, w.Expect = 2, 3
	w.NoteArrival(2)
	if w.Fails != 1 || w.Target != 9 {
		t.Fatalf("one failed step: %+v", w)
	}
	w.LastFrom, w.Expect = 2, 3
	w.NoteArrival(2)
	if w.Fails != 0 || w.Target != 0 {
		t.Fatalf("two failed steps drop the target: %+v", w)
	}

	// Nothing pending: nothing to settle.
	w = &Walk{Target: 9, Fails: 1}
	w.NoteArrival(4)
	if w.Fails != 1 {
		t.Fatal("settled a step that was never taken")
	}
}

func TestLine_FillsPlaceholders(t *testing.T) {
	got := Line([]string{`{actor} takes the {item} and {gold}.`}, func(int) int { return 0 }, `Old Mags`, `rusty nail`, 4)
	for _, want := range []string{`>Old Mags<`, `>rusty nail<`, `>4 gold<`} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q missing %q", got, want)
		}
	}
	if Line(nil, func(int) int { return 0 }, `x`, ``, 0) != `` {
		t.Fatal("no lines, no text")
	}
}

// Player homes never join a pool, even when the pool's zone and biome would
// take them in, and a home room that is one fails the load.
func TestParse_DropsPrivateRooms(t *testing.T) {
	w := fakeWorld()
	w.Private = func(id int) bool { return id == 3 }

	raw := `
scavengers:
  - mobid: 9820
    city: Town
    zones: [Town]
    home_room: 1` + goodLines
	ps, err := Parse([]byte(raw), w)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ps[0].Pool() {
		if id == 3 {
			t.Fatalf("private room 3 in the pool %v", ps[0].Pool())
		}
	}

	raw = `
scavengers:
  - mobid: 9820
    city: Town
    include_rooms: [1, 2, 3]
    home_room: 3` + goodLines
	if _, err := Parse([]byte(raw), w); err == nil {
		t.Fatal("a private home room was accepted")
	}
}
