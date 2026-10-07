package shops

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
)

func TestDefaultPricingConfig_BaselineQty(t *testing.T) {
	assert.Equal(t, 3, DefaultPricingConfig().DefaultBaselineQty)
}

func TestPricingBaseline_UsesRestockQtyWhenPositive(t *testing.T) {
	assert.Equal(t, 7, PricingBaseline(&StockEntry{RestockQty: 7, MaxStock: 20}, DefaultPricingConfig()))
}

func TestPricingBaseline_UsesDefaultWhenZero(t *testing.T) {
	// not MaxStock/2 (=10) — the unified baseline.
	assert.Equal(t, 3, PricingBaseline(&StockEntry{RestockQty: 0, MaxStock: 20}, DefaultPricingConfig()))
}

func TestPricingBaseline_NilEntryUsesDefault(t *testing.T) {
	assert.Equal(t, 3, PricingBaseline(nil, DefaultPricingConfig()))
}

func TestPricingBaseline_ClampsDefaultToOne(t *testing.T) {
	cfg := DefaultPricingConfig()
	cfg.DefaultBaselineQty = 0
	assert.Equal(t, 1, PricingBaseline(&StockEntry{RestockQty: 0, MaxStock: 20}, cfg))
}

func TestScarcityMultiplier_Baseline3_ModestStockIsAffordable(t *testing.T) {
	cfg := DefaultPricingConfig()
	// 5 units at baseline 3 → ratio 1.67 → comfortably below base price.
	assert.Less(t, ScarcityMultiplier(5, 3, cfg), 1.5)
}

func TestScarcityMultiplier_Baseline3_NineUnitsHitsFloor(t *testing.T) {
	cfg := DefaultPricingConfig()
	// 9 units at baseline 3 → ratio 3.0 == AbundanceThreshold → floor.
	assert.Equal(t, cfg.PriceFloor, ScarcityMultiplier(9, 3, cfg))
}

func TestScarcityMultiplier_Baseline3_LastUnitBounded(t *testing.T) {
	cfg := DefaultPricingConfig()
	// A single unit still carries a premium, but never exceeds the ceiling.
	m := ScarcityMultiplier(1, 3, cfg)
	assert.Greater(t, m, 1.0)
	assert.LessOrEqual(t, m, cfg.PriceCeiling)
}

func TestScarcityMultiplier_OutOfStock(t *testing.T) {
	cfg := DefaultPricingConfig()
	mult := ScarcityMultiplier(0, 5, cfg)
	assert.Equal(t, 1.5, mult)
}

func TestScarcityMultiplier_FullyAbundant(t *testing.T) {
	cfg := DefaultPricingConfig()
	mult := ScarcityMultiplier(15, 5, cfg) // ratio = 3.0 = threshold
	assert.Equal(t, 0.75, mult)
}

func TestScarcityMultiplier_OverAbundant(t *testing.T) {
	cfg := DefaultPricingConfig()
	mult := ScarcityMultiplier(20, 5, cfg) // ratio = 4.0 > threshold
	assert.Equal(t, 0.75, mult)
}

func TestScarcityMultiplier_AtRestock(t *testing.T) {
	cfg := DefaultPricingConfig()
	mult := ScarcityMultiplier(5, 5, cfg) // ratio = 1.0
	assert.Greater(t, mult, cfg.PriceFloor)
	assert.Less(t, mult, cfg.PriceCeiling)
}

func TestScarcityMultiplier_Monotonic(t *testing.T) {
	cfg := DefaultPricingConfig()
	prev := ScarcityMultiplier(0, 5, cfg)
	for stock := 1; stock <= 15; stock++ {
		curr := ScarcityMultiplier(stock, 5, cfg)
		assert.LessOrEqual(t, curr, prev, "stock=%d should have lower or equal mult than stock=%d", stock, stock-1)
		prev = curr
	}
}

func TestScarcityMultiplier_ZeroRestock(t *testing.T) {
	cfg := DefaultPricingConfig()
	// Should not panic, treats as restockQty=1
	mult := ScarcityMultiplier(3, 0, cfg)
	assert.Greater(t, mult, 0.0)
}

func TestCalcSellPrice_OutOfStock(t *testing.T) {
	cfg := DefaultPricingConfig()
	assert.Equal(t, 15, CalcSellPrice(10, 0, 5, cfg))
}

func TestCalcBuyPrice_OutOfStock(t *testing.T) {
	cfg := DefaultPricingConfig()
	assert.Equal(t, 8, CalcBuyPrice(10, 0, 5, cfg))
}

func TestCalcBuyPrice_Abundant(t *testing.T) {
	cfg := DefaultPricingConfig()
	price := CalcBuyPrice(10, 15, 5, cfg)
	assert.Equal(t, 4, price)
}

func TestCalcSellPrice_MinimumOne(t *testing.T) {
	cfg := DefaultPricingConfig()
	// Even a 0-value item should cost at least 1
	price := CalcSellPrice(0, 10, 5, cfg)
	assert.GreaterOrEqual(t, price, 1)
}

func TestBarterSellDiscount(t *testing.T) {
	assert.Equal(t, 85, ApplyBarterSellDiscount(100, 0.15))
}

func TestBarterBuyBonus(t *testing.T) {
	assert.Equal(t, 115, ApplyBarterBuyBonus(100, 0.15))
}

// The bonus adds whole gold only: 15% of a 1-gold offer is not a gold, so the
// offer stays 1 instead of rounding up to 2 (a 100% bonus under a 15% cap).
func TestBarterBuyBonus_SmallOfferNeverExceedsCap(t *testing.T) {
	assert.Equal(t, 1, ApplyBarterBuyBonus(1, 0.15))
	assert.Equal(t, 6, ApplyBarterBuyBonus(6, 0.15), "0.9 gold of bonus is not a whole gold")
	assert.Equal(t, 8, ApplyBarterBuyBonus(7, 0.15), "1.05 gold of bonus adds one gold")
	for price := 1; price <= 200; price++ {
		got := ApplyBarterBuyBonus(price, 0.15)
		assert.LessOrEqual(t, float64(got), float64(price)*1.15+1e-9, "price %d", price)
		assert.GreaterOrEqual(t, got, price, "price %d", price)
	}
}

func TestBarterBuyBonus_LargeOfferKeepsBonus(t *testing.T) {
	assert.Equal(t, 1150, ApplyBarterBuyBonus(1000, 0.15))
	assert.Equal(t, 130, ApplyBarterBuyBonus(100, 0.30))
}

func TestBarterSellDiscount_MinimumOne(t *testing.T) {
	assert.Equal(t, 1, ApplyBarterSellDiscount(1, 0.99))
}

func TestSpread_BuyAlwaysLessThanSell(t *testing.T) {
	cfg := DefaultPricingConfig()
	for stock := 0; stock <= 20; stock++ {
		sell := CalcSellPrice(10, stock, 5, cfg)
		buy := CalcBuyPrice(10, stock, 5, cfg)
		assert.LessOrEqual(t, buy, sell, "buy should be <= sell at stock=%d", stock)
	}
}

// TestPricingConfigFromBalance_ShippedBand pins the shop scarcity band the
// shipped config.yaml carries (owner ruling 2026-10-07: 0.75 to 1.5). Test
// binaries load Go defaults, not config.yaml, so this loads the real file;
// without that the Go default and the shipped value could drift apart unseen.
func TestPricingConfigFromBalance_ShippedBand(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	t.Chdir(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	cfg := PricingConfigFromBalance()
	assert.Equal(t, 0.75, cfg.PriceFloor)
	assert.Equal(t, 1.5, cfg.PriceCeiling)
	assert.Equal(t, 3.0, cfg.AbundanceThreshold)
	assert.Equal(t, 0.50, cfg.BuyRatio)
}
