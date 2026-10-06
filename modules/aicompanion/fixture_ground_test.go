package aicompanion

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// R9, sibling paths: a fixture is part of the room, so the companion's room
// listing, her scene and her progress count leave it out like "On the Ground".
func TestCompanionGroundListsLeaveOutFixtures(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		999991: {ItemId: 999991, Name: "Arch Lantern", Type: items.Object, Fixture: items.FixtureLight},
		999992: {ItemId: 999992, Name: "Grey Pebble", Type: items.Object},
	}))
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	room.Items = []items.Item{items.New(999991), items.New(999992)}

	if got := strings.Join(roomThings(room), `, `); strings.Contains(got, "Lantern") || !strings.Contains(got, "Pebble") {
		t.Errorf("roomThings = %q, want only the pebble", got)
	}
	sc := buildScene(her, owner, &Profile{}, &Mind{}, 0)
	for _, th := range sc.Things {
		if th.Kind == `item` && strings.Contains(th.Name, "Lantern") {
			t.Errorf("scene lists the fixture: %+v", th)
		}
	}
	if n := snapshotOf(her).RoomItems; n != 1 {
		t.Errorf("RoomItems = %d, want 1 (the pebble)", n)
	}
}
