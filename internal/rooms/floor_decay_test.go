package rooms

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"gopkg.in/yaml.v2"
)

// The daily chance is 10% for a lone item plus 5% for each other one, held to
// 100: the user's worked example is 5 items at 30% each.
func TestFloorDecayChancePct(t *testing.T) {
	cases := []struct{ n, want int }{
		{0, 0}, {1, 10}, {2, 15}, {5, 30}, {10, 55}, {19, 100}, {40, 100},
	}
	for _, c := range cases {
		if got := FloorDecayChancePct(c.n, 10, 5); got != c.want {
			t.Errorf("FloorDecayChancePct(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

// litter builds n ordinary floor items with distinct ids.
func litter(n int) []items.Item {
	out := make([]items.Item, n)
	for i := range out {
		out[i] = items.Item{ItemId: 70000 + i}
	}
	return out
}

// fixedRoll returns v for every roll and counts the calls.
func fixedRoll(v int, calls *int) func(int) int {
	return func(int) int {
		*calls++
		return v
	}
}

var decayNow = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

// Each item rolls once against the pile's chance: a roll just under 30 takes
// every one of five items, a roll of 30 takes none.
func TestDecayFloor_EachItemRollsAtThePileChance(t *testing.T) {
	today := DecayDay(decayNow)

	calls := 0
	r := &Room{RoomId: 10, Items: litter(5), FloorDecayDay: today - 1}
	removed, processed := r.decayFloor(decayNow, fixedRoll(29, &calls))
	if !processed || removed != 5 || len(r.Items) != 0 || calls != 5 {
		t.Fatalf("roll 29 vs 30%%: processed=%v removed=%d left=%d calls=%d", processed, removed, len(r.Items), calls)
	}

	calls = 0
	r = &Room{RoomId: 10, Items: litter(5), FloorDecayDay: today - 1}
	removed, _ = r.decayFloor(decayNow, fixedRoll(30, &calls))
	if removed != 0 || len(r.Items) != 5 {
		t.Fatalf("roll 30 vs 30%%: removed=%d left=%d", removed, len(r.Items))
	}
	if r.FloorDecayDay != today {
		t.Fatalf("stamped %d, want today %d", r.FloorDecayDay, today)
	}
}

// A room is decayed at most once a day, however often the check runs.
func TestDecayFloor_OncePerDay(t *testing.T) {
	calls := 0
	r := &Room{RoomId: 10, Items: litter(3), FloorDecayDay: DecayDay(decayNow) - 1}
	r.decayFloor(decayNow, fixedRoll(99, &calls))
	calls = 0
	if _, processed := r.decayFloor(decayNow.Add(6*time.Hour), fixedRoll(0, &calls)); processed || calls != 0 {
		t.Fatalf("second pass the same day: processed=%v calls=%d", processed, calls)
	}
	if len(r.Items) != 3 {
		t.Fatalf("items lost on the second check: %d", len(r.Items))
	}
}

// A room unloaded for several days owes one pass per missed day, capped.
func TestDecayFloor_CatchesUpMissedDays(t *testing.T) {
	today := DecayDay(decayNow)

	calls := 0
	r := &Room{RoomId: 10, Items: litter(2), FloorDecayDay: today - 3}
	r.decayFloor(decayNow, fixedRoll(99, &calls))
	if calls != 3*2 {
		t.Fatalf("3 missed days x 2 items: %d rolls", calls)
	}

	calls = 0
	r = &Room{RoomId: 10, Items: litter(1), FloorDecayDay: today - 400}
	r.decayFloor(decayNow, fixedRoll(99, &calls))
	if calls != floorDecayMaxCatchUpDays {
		t.Fatalf("catch-up cap: %d rolls, want %d", calls, floorDecayMaxCatchUpDays)
	}

	// Never decayed before (a pile from before this system): one pass.
	calls = 0
	r = &Room{RoomId: 10, Items: litter(4)}
	r.decayFloor(decayNow, fixedRoll(99, &calls))
	if calls != 4 {
		t.Fatalf("first-ever pass: %d rolls, want 4", calls)
	}
}

// Later passes of a catch-up roll at the smaller pile's lower chance.
func TestDecayFloor_ChanceShrinksWithThePile(t *testing.T) {
	today := DecayDay(decayNow)
	// Two items: 15% on day one. A roll of 12 takes both (12 < 15). Then the
	// floor is clean and day two has nothing to roll.
	calls := 0
	r := &Room{RoomId: 10, Items: litter(2), FloorDecayDay: today - 2}
	removed, _ := r.decayFloor(decayNow, fixedRoll(12, &calls))
	if removed != 2 || calls != 2 {
		t.Fatalf("removed=%d calls=%d", removed, calls)
	}

	// Two items and a roll of 12 each time, but the first roll removes one
	// and the second must then beat the lone-item 10%: 12 does not.
	seq := []int{12, 99, 12}
	i := 0
	r = &Room{RoomId: 10, Items: litter(2), FloorDecayDay: today - 2}
	removed, _ = r.decayFloor(decayNow, func(int) int { v := seq[i]; i++; return v })
	if removed != 1 || len(r.Items) != 1 || i != 3 {
		t.Fatalf("removed=%d left=%d rolls=%d", removed, len(r.Items), i)
	}
}

// Nothing vanishes in front of a player; the pass waits until they leave.
func TestDecayFloor_DeferredWhileOccupied(t *testing.T) {
	calls := 0
	r := &Room{RoomId: 10, Items: litter(3), FloorDecayDay: DecayDay(decayNow) - 1, players: []int{1}}
	if _, processed := r.decayFloor(decayNow, fixedRoll(0, &calls)); processed || calls != 0 || len(r.Items) != 3 {
		t.Fatalf("occupied room decayed: processed=%v calls=%d", processed, calls)
	}
	r.players = nil
	if removed, processed := r.decayFloor(decayNow, fixedRoll(0, &calls)); !processed || removed != 3 {
		t.Fatalf("after they left: processed=%v removed=%d", processed, removed)
	}
}

// Rooms a scavenger keeps, and ephemeral rooms, are never decayed.
func TestDecayFloor_ExemptAndEphemeralRooms(t *testing.T) {
	defer SetFloorDecayExempt(nil)
	SetFloorDecayExempt(func(roomId int) bool { return roomId == 10 })

	calls := 0
	r := &Room{RoomId: 10, Items: litter(3)}
	if _, processed := r.decayFloor(decayNow, fixedRoll(0, &calls)); processed || calls != 0 {
		t.Fatal("an exempt room was decayed")
	}

	eph := &Room{RoomId: ephemeralRoomIdMinimum + 1, Items: litter(3)}
	if _, processed := eph.decayFloor(decayNow, fixedRoll(0, &calls)); processed || calls != 0 {
		t.Fatal("an ephemeral room was decayed")
	}
}

// What the world put on the floor (template items, SpawnInfo items) is not
// litter: never rolled, and not counted toward the pile's chance.
func TestDecayFloor_AuthoredItemsAreKeptAndNotCounted(t *testing.T) {
	calls := 0
	token := items.Item{ItemId: 2}
	flower := items.Item{ItemId: 3}
	junk := items.Item{ItemId: 70000}
	r := &Room{
		RoomId:          10,
		Items:           []items.Item{token, flower, junk},
		SpawnInfo:       []SpawnInfo{{ItemId: 3}},
		authoredItemIds: map[int]bool{2: true},
		FloorDecayDay:   DecayDay(decayNow) - 1,
	}
	// One litter item: 10%. A roll of 9 takes it; the other two are never
	// rolled.
	removed, _ := r.decayFloor(decayNow, fixedRoll(9, &calls))
	if removed != 1 || calls != 1 || len(r.Items) != 2 {
		t.Fatalf("removed=%d calls=%d left=%+v", removed, calls, r.Items)
	}
	for _, it := range r.Items {
		if it.ItemId == 70000 {
			t.Fatal("the litter survived")
		}
	}

	// A floor holding only authored items is left alone, unstamped.
	r.FloorDecayDay = 0
	if _, processed := r.decayFloor(decayNow, fixedRoll(0, &calls)); processed || r.FloorDecayDay != 0 {
		t.Fatal("an authored-only floor was processed")
	}
}

// A found bauble still lying untaken keeps its own clock, and a household's
// bauble is the house's: neither is litter.
func TestFloorItemIsLitter_Baubles(t *testing.T) {
	defer func() { floorDecayNow = time.Now }()
	floorDecayNow = func() time.Time { return decayNow }

	untaken := items.Item{ItemId: items.BaubleItemId, Bauble: `B0000001`}
	untaken.LeaveBaubleAt(``, 0, decayNow.Add(-time.Hour))
	household := items.Item{ItemId: items.BaubleItemId, Bauble: `B0000002`}
	household.LeaveBaubleAt(`on the shelf`, 10, decayNow.Add(-time.Hour))
	carried := items.Item{ItemId: items.BaubleItemId, Bauble: `B0000003`}

	r := &Room{RoomId: 10}
	if r.FloorItemIsLitter(untaken) || r.FloorItemIsLitter(household) {
		t.Fatal("an untaken find counted as litter")
	}
	if !r.FloorItemIsLitter(carried) {
		t.Fatal("a carried-and-dropped bauble is ordinary litter")
	}
}

// The clock starts when litter lands on a clean floor, so a new item is not
// rolled for days it was not there; more litter does not restart it.
func TestAddItem_StartsTheDecayClock(t *testing.T) {
	defer func() { floorDecayNow = time.Now }()
	floorDecayNow = func() time.Time { return decayNow }
	today := DecayDay(decayNow)

	r := &Room{RoomId: 10, FloorDecayDay: today - 50}
	r.AddItem(items.Item{ItemId: 70000}, false)
	if r.FloorDecayDay != today {
		t.Fatalf("first litter stamped %d, want %d", r.FloorDecayDay, today)
	}

	r.FloorDecayDay = today - 1
	r.AddItem(items.Item{ItemId: 70001}, false)
	if r.FloorDecayDay != today-1 {
		t.Fatal("a second item restarted a running clock")
	}

	// Stash and authored items leave the clock alone.
	r2 := &Room{RoomId: 11, FloorDecayDay: today - 50, authoredItemIds: map[int]bool{2: true}}
	r2.AddItem(items.Item{ItemId: 70000}, true)
	r2.AddItem(items.Item{ItemId: 2}, false)
	if r2.FloorDecayDay != today-50 {
		t.Fatal("stash or authored item started the clock")
	}
}

// DecayLoadedFloors visits every loaded room and reports what it did.
func TestDecayLoadedFloors(t *testing.T) {
	today := DecayDay(decayNow)
	a := &Room{RoomId: 1, Items: litter(19), FloorDecayDay: today - 1} // 100%: all go
	b := &Room{RoomId: 2, Items: litter(2), FloorDecayDay: today}      // already done today
	c := &Room{RoomId: 3}                                              // clean floor
	defer SeedRoomsForTest(map[int]*Room{1: a, 2: b, 3: c}, map[string]*ZoneConfig{})()

	processed, removed := DecayLoadedFloors(decayNow)
	if processed != 1 || removed != 19 || len(a.Items) != 0 || len(b.Items) != 2 {
		t.Fatalf("processed=%d removed=%d a=%d b=%d", processed, removed, len(a.Items), len(b.Items))
	}
}

// Loading a room over its instance save keeps the template's own floor items
// marked as authored (the save replaced Items, but not what the template
// lays down), and the decay day rides the save.
func TestLoadRoomInstance_KeepsAuthoredIdsAndDecayDay(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()

	tempDir := t.TempDir()
	prev := configs.GetFilePathsConfig()
	if err := configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": tempDir}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": prev.DataFiles.String()})
	}()

	template := &Room{RoomId: 90104, Zone: "test_zone", Title: "Decay Test", Description: "A test room.",
		Exits: map[string]exit.RoomExit{}, Items: []items.Item{{ItemId: 2}}}
	writeYAML(t, filepath.Join(tempDir, "rooms", "test_zone", "90104.yaml"), template)
	writeYAML(t, filepath.Join(tempDir, "rooms.instances", "test_zone", "90104.yaml"), map[string]any{
		"items":         []map[string]any{{"itemid": 2}, {"itemid": 70000}},
		"floordecayday": 20000,
	})
	roomManager.setCachedFilePath(90104, "test_zone/90104.yaml")

	loaded := LoadRoomInstance(90104)
	if loaded == nil {
		t.Fatal("room did not load")
	}
	if loaded.FloorDecayDay != 20000 {
		t.Fatalf("FloorDecayDay %d, want 20000 from the save", loaded.FloorDecayDay)
	}
	if loaded.FloorItemIsLitter(loaded.Items[0]) || !loaded.FloorItemIsLitter(loaded.Items[1]) {
		t.Fatalf("authored token must not be litter, the other item must: %+v", loaded.Items)
	}
}

func writeYAML(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := yaml.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
