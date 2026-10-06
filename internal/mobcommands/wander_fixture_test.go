package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// R9 sibling: a room holding only a fixture has nothing to scavenge.
func TestLootWorthRoomIgnoresFixtures(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		999991: {ItemId: 999991, Name: "Arch Lantern", Type: items.Object, Fixture: items.FixtureLight},
		999992: {ItemId: 999992, Name: "Grey Pebble", Type: items.Object},
	}))
	if hasLootItems([]items.Item{items.New(999991)}) {
		t.Error("a fixture alone counts as loot")
	}
	if !hasLootItems([]items.Item{items.New(999991), items.New(999992)}) {
		t.Error("a pebble does not count as loot")
	}
}
