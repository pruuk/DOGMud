package mobs

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Rule 5: a mob spawned holding a treed item (a keeper's lantern in the
// light slot) joins the item tick's holder index; one with none does not.
func TestSpawnIndexesAMobHoldingATreedItem(t *testing.T) {
	t.Cleanup(seedRegistry())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		999970: {ItemId: 999970, Name: "test keeper lantern", Type: items.Light, Subtype: items.Wearable,
			Behavior: "keeper_lantern"},
	}))
	t.Cleanup(items.ResetHolderIndexForTest())

	mobs[1].Character.Equipment.Light = items.Item{ItemId: 999970}
	t.Cleanup(func() { mobs[1].Character.Equipment.Light = items.Item{} })

	keeper := NewMobByIdFresh(MobId(1), 4242)
	if keeper == nil {
		t.Fatal("spawn returned nil")
	}
	defer DestroyInstance(keeper.InstanceId)
	plain := NewMobByIdFresh(MobId(2), 4242)
	if plain == nil {
		t.Fatal("spawn returned nil")
	}
	defer DestroyInstance(plain.InstanceId)

	got := items.MobHolders()
	if len(got) != 1 || got[0] != keeper.InstanceId {
		t.Errorf("MobHolders = %v, want only the keeper %d", got, keeper.InstanceId)
	}
}
