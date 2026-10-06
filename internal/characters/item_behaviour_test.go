package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

const (
	treedLanternItem = 999950
	plainRockItem    = 999951
)

func seedTreedItems(t *testing.T) {
	t.Helper()
	t.Cleanup(seedMediumSpecies())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		treedLanternItem: {ItemId: treedLanternItem, Name: "test keeper lantern", Type: items.Light, Subtype: items.Wearable,
			Behavior: "keeper_lantern"},
		plainRockItem: {ItemId: plainRockItem, Name: "test rock", Type: items.Object},
	}))
	t.Cleanup(items.ResetHolderIndexForTest())
}

func indexedMobs() map[int]bool {
	out := map[int]bool{}
	for _, id := range items.MobHolders() {
		out[id] = true
	}
	return out
}

// Rule 5: a mob joins the item tick's index when it stores or wears a
// treed item, and through IndexTreedItems at spawn. A player never does
// (the tick walks every online player), and an untreed item indexes nobody.
func TestAMobHandedATreedItemJoinsTheIndex(t *testing.T) {
	seedTreedItems(t)

	stored := New()
	stored.MobInstanceId = 41
	stored.Stats.Strength.ValueAdj = 100
	stored.StoreItem(items.New(treedLanternItem))

	worn := New()
	worn.MobInstanceId = 42
	worn.Stats.Strength.ValueAdj = 100
	if _, ok, why := worn.Wear(items.New(treedLanternItem)); !ok {
		t.Fatalf("could not wear the lantern: %s", why)
	}

	spawned := New()
	spawned.MobInstanceId = 43
	spawned.Equipment.Light = items.New(treedLanternItem)
	spawned.IndexTreedItems()

	rock := New()
	rock.MobInstanceId = 44
	rock.Stats.Strength.ValueAdj = 100
	rock.StoreItem(items.New(plainRockItem))
	rock.IndexTreedItems()

	player := New()
	player.Stats.Strength.ValueAdj = 100
	player.StoreItem(items.New(treedLanternItem))

	got := indexedMobs()
	for _, id := range []int{41, 42, 43} {
		if !got[id] {
			t.Errorf("mob %d holds a treed item but is not indexed (index %v)", id, got)
		}
	}
	if got[44] || len(got) != 3 {
		t.Errorf("index %v, want exactly mobs 41, 42 and 43", got)
	}
}
