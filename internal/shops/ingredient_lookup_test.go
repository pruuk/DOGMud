package shops

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// seedBottles registers the four real bottle items, which all share
// component_tag "bottle" and differ in material tier. This is the collision
// that made the map-order resolver dangerous.
func seedBottles(t *testing.T) func() {
	t.Helper()
	return items.SeedItemsForTest(map[int]*items.ItemSpec{
		40043: {ItemId: 40043, Name: "clay flask", ComponentTag: "bottle", MaterialTier: 1},
		40006: {ItemId: 40006, Name: "glass vial", ComponentTag: "bottle", MaterialTier: 1},
		40044: {ItemId: 40044, Name: "sealed phial", ComponentTag: "bottle", MaterialTier: 3},
		40045: {ItemId: 40045, Name: "crystalline decanter", ComponentTag: "bottle", MaterialTier: 4},
	})
}

// TestFindStockedIngredientIsDeterministic is the whole reason this function
// exists. items.FindSpecByComponentTag ranges a Go map, so it answered
// differently between calls; two independent callers could therefore verify
// stock of one bottle and deduct another.
func TestFindStockedIngredientIsDeterministic(t *testing.T) {
	defer seedBottles(t)()

	inv := &ShopInventory{Stock: []StockEntry{
		{ItemId: 40045, Current: 5}, // decanter, tier 4
		{ItemId: 40043, Current: 5}, // clay flask, tier 1
		{ItemId: 40044, Current: 5}, // sealed phial, tier 3
	}}

	first := FindStockedIngredient(inv, "bottle")
	if first == nil {
		t.Fatal("no bottle resolved from a shop stocking three of them")
	}
	for i := 0; i < 300; i++ {
		got := FindStockedIngredient(inv, "bottle")
		if got == nil || got.ItemId != first.ItemId {
			t.Fatalf("resolution flipped between identical calls: %d then %v. "+
				"That is the map-order bug this function replaces.", first.ItemId, got)
		}
	}
	if first.ItemId != 40043 {
		t.Errorf("resolved item %d, want the tier-1 clay flask 40043 — cheapest "+
			"material first, matching player crafting", first.ItemId)
	}
}

// TestFindStockedIngredientOnlyConsidersStock pins the property that closes the
// free-craft hole: an item the shop does NOT stock can never be selected.
//
// Previously the resolver drew from the global item registry, so it could name
// an item with no stock entry; RemoveStockAtRound then removed 0 and the shop
// crafted for free.
func TestFindStockedIngredientOnlyConsidersStock(t *testing.T) {
	defer seedBottles(t)()

	// Stocks ONLY the decanter, even though cheaper bottles exist in the registry.
	inv := &ShopInventory{Stock: []StockEntry{{ItemId: 40045, Current: 2}}}

	got := FindStockedIngredient(inv, "bottle")
	if got == nil || got.ItemId != 40045 {
		t.Fatalf("resolved %v, want the decanter 40045 — the only bottle stocked. "+
			"Naming an unstocked item is how the shop crafted for free.", got)
	}
}

// TestFindStockedIngredientReportsNothingWhenUnstocked pins the nil contract.
// A caller treating nil as "fine" reintroduces the free craft.
func TestFindStockedIngredientReportsNothingWhenUnstocked(t *testing.T) {
	defer seedBottles(t)()

	inv := &ShopInventory{Stock: []StockEntry{{ItemId: 99999, Current: 5}}}
	if got := FindStockedIngredient(inv, "bottle"); got != nil {
		t.Fatalf("resolved %v from a shop stocking no bottles; want nil", got)
	}
	if got := FindStockedIngredient(nil, "bottle"); got != nil {
		t.Fatalf("resolved %v from a nil inventory; want nil", got)
	}
}
