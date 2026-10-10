package shops

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
)

// #346: the reserve gate read the global config, so a test could not pin it
// through cfg. It now reads cfg.GoldReserveRatio.
func TestBuyRules_GoldReserveComesFromPricingConfig(t *testing.T) {
	item := makeItem(items.ItemSpec{
		ItemId:           100,
		Value:            200,
		Type:             items.Object,
		VendorCategories: []string{"alchemy"},
	})
	shop := baseShop() // 1000 gold, started with 1000
	shop.CraftSupport = CraftSupportAlchemy
	cfg := DefaultPricingConfig()

	// Unstocked, so the flat price: 200 x 0.5 = 100. Keeping half back
	// leaves 500 to spend.
	assert.Equal(t, 100, EvaluateBuyRules(item, shop, "", false, cfg, nil).Price)

	// Keeping 95% back leaves 50 to spend: the same offer is refused.
	cfg.GoldReserveRatio = 0.95
	assert.Equal(t, 0, EvaluateBuyRules(item, shop, "", false, cfg, nil).Price)
}

func TestPricingConfigFromBalance_CarriesGoldReserveRatio(t *testing.T) {
	c := configs.GetConfig()
	c.Balance.ShopGoldReserveRatio = 0.8
	configs.SetConfigForTest(t, c)
	assert.Equal(t, 0.8, PricingConfigFromBalance().GoldReserveRatio)
}
