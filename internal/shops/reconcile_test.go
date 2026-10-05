package shops

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// A saved shop follows its template: the renamed craft support, stock the
// template gained since the save, and no forged tools on the shelf.
func TestReconcileShop(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		10: {ItemId: 10, Name: "pelt"},
		30: {ItemId: 30, Name: "crude knife", Tool: &items.ToolSpec{Type: items.ToolKnife, Tier: items.ToolTierCrude}},
		40: {ItemId: 40, Name: "steel knife", Tool: &items.ToolSpec{Type: items.ToolKnife, Tier: items.ToolTierSteel}},
	}))

	saved := &ShopInventory{
		CraftSupport: "carpentry",
		Stock: []StockEntry{
			{ItemId: 10, RestockQty: 0, MaxStock: 10, Current: 4}, // walk-in pelts players sold
			{ItemId: 40, RestockQty: 5, MaxStock: 10, Current: 3}, // a forged tool from an older template
		},
	}
	template := ShopInventory{
		CraftSupport: CraftSupportWoodwork,
		Stock: []StockEntry{
			{ItemId: 30, RestockQty: 5, MaxStock: 20},
			{ItemId: 40, RestockQty: 5, MaxStock: 20},
		},
	}
	if !reconcileShop(saved, template) {
		t.Fatal("reconcileShop reported no change")
	}
	if saved.CraftSupport != CraftSupportWoodwork {
		t.Errorf("craft support = %q, want woodwork", saved.CraftSupport)
	}
	got := map[int]StockEntry{}
	for _, e := range saved.Stock {
		got[e.ItemId] = e
	}
	if e, ok := got[10]; !ok || e.Current != 4 {
		t.Errorf("walk-in stock must be kept as it was, got %+v", got[10])
	}
	if _, ok := got[40]; ok {
		t.Error("a forged tool must come off the shelf")
	}
	if e, ok := got[30]; !ok || e.Current <= 0 {
		t.Errorf("the crude knife the template gained must be stocked, got %+v", got[30])
	}
	if reconcileShop(saved, template) {
		t.Error("a second reconcile should change nothing")
	}
}
