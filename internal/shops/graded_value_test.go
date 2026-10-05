package shops

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

func TestGradedValue(t *testing.T) {
	cleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		40070: {ItemId: 40070, Name: "Pack-Hide Pelt", Value: 5, IsComponent: true},
	})
	defer cleanup()

	if got := GradedValue(items.Item{ItemId: 40070}); got != 5 {
		t.Errorf("ungraded pelt = %d, want spec value 5", got)
	}
	std := GradedValue(items.Item{ItemId: 40070, Quality: items.QualityStandard})
	fine := GradedValue(items.Item{ItemId: 40070, Quality: items.QualityFine})
	crude := GradedValue(items.Item{ItemId: 40070, Quality: items.QualityCrude})
	pristine := GradedValue(items.Item{ItemId: 40070, Quality: items.QualityPristine})
	if !(crude < std && std < fine && fine < pristine) {
		t.Errorf("grades should order crude<standard<fine<pristine, got %d %d %d %d", crude, std, fine, pristine)
	}
	if crude < 1 {
		t.Error("a graded item never prices below 1")
	}
}

// The walk-in bug: the second unit of an item the shop does not stock used to
// price on the scarcity curve at about four times the first.
func TestWalkInBuyPrice_NeverRisesAndSlidesGently(t *testing.T) {
	cfg := DefaultPricingConfig()
	first := WalkInBuyPrice(100, 0, cfg)
	second := WalkInBuyPrice(100, 1, cfg)
	tenth := WalkInBuyPrice(100, 9, cfg)
	if second > first {
		t.Errorf("second unit (%d) must not pay more than the first (%d)", second, first)
	}
	if tenth >= first || tenth < first/2 {
		t.Errorf("the tenth unit (%d) should pay a little less than the first (%d), not crash", tenth, first)
	}
	if floor := WalkInBuyPrice(100, 1000, cfg); floor < 1 {
		t.Error("never below 1")
	}
}

func TestEvaluateBuyRules_WalkInEntryUsesFlatSlope(t *testing.T) {
	cleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		500: {ItemId: 500, Name: "Wolf Pelt", Value: 20, Type: items.Object, IsComponent: true, VendorCategories: []string{"tailoring"}},
	})
	defer cleanup()
	shop := &ShopInventory{Gold: 100000, StartingGold: 100000, CraftSupport: CraftSupportTailoring}
	cfg := DefaultPricingConfig()
	first := EvaluateBuyRules(items.Item{ItemId: 500}, shop, "", false, cfg, nil).Price
	shop.AddStock(500, 1) // the shop now holds the first pelt (RestockQty 0)
	second := EvaluateBuyRules(items.Item{ItemId: 500}, shop, "", false, cfg, nil).Price
	if first == 0 || second == 0 {
		t.Fatalf("the tailor should buy pelts: %d, %d", first, second)
	}
	if second > first {
		t.Errorf("second pelt paid %d, more than the first (%d)", second, first)
	}
}

// Review fix: a shop must never pay more for a walk-in good than BuyRatio of
// what it would charge for that same unit once it holds it, or a player buys
// one back cheap and sells it again at a profit.
func TestEvaluateBuyRules_WalkInNoBuyBackProfit(t *testing.T) {
	cleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		500: {ItemId: 500, Name: "Wolf Pelt", Value: 200, Type: items.Object, IsComponent: true, VendorCategories: []string{"tailoring"}},
	})
	defer cleanup()
	shop := &ShopInventory{Gold: 1000000, StartingGold: 1000000, CraftSupport: CraftSupportTailoring}
	cfg := DefaultPricingConfig()
	for held := 0; held < 19; held++ {
		pays := EvaluateBuyRules(items.Item{ItemId: 500}, shop, "", false, cfg, nil).Price
		entry := shop.GetStock(500)
		charges := CalcSellPrice(200, held+1, PricingBaseline(entry, cfg), cfg)
		if pays >= charges {
			t.Fatalf("holding %d the shop pays %d but resells for %d", held, pays, charges)
		}
		shop.AddStock(500, 1)
	}
}

// Review fix: goods a shop scraps (forged tools, broken gear) still slide in
// price per recent unit, and the memory wears off with time.
func TestScrapSlidesAndWearsOff(t *testing.T) {
	cleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		501: {ItemId: 501, Name: "Steel Pick", Value: 200, Type: items.Object, VendorCategories: []string{"blacksmithing"}},
	})
	defer cleanup()
	shop := &ShopInventory{Gold: 1000000, StartingGold: 1000000, CraftSupport: CraftSupportBlacksmithing}
	cfg := DefaultPricingConfig()
	first := EvaluateBuyRules(items.Item{ItemId: 501}, shop, "", false, cfg, nil).Price
	for i := 0; i < 10; i++ {
		shop.AddScrap(501, 0)
	}
	if got := shop.ScrapHeld(501, 0); got != 10 {
		t.Fatalf("ten scrap buys held, got %d", got)
	}
	if later := EvaluateBuyRules(items.Item{ItemId: 501}, shop, "", false, cfg, nil).Price; first == 0 || later >= first {
		t.Errorf("the eleventh pick should pay less than the first: %d vs %d", later, first)
	}
	if got := shop.ScrapHeld(501, 1000000); got != 0 {
		t.Errorf("scrap memory should wear off, still %d", got)
	}
}
