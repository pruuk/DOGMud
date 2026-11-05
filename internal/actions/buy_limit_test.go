package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// The per-character limit on buying wafers, unit by unit. The shipped dealer in
// the real room is internal/guards/tier5_bloom_buy_test.go.

const (
	limitPerDay      = 3
	limitWindowHours = 24
	limitSellerId    = 7451
)

func pinBuyLimit(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.BloomDealerWafersPerDay = limitPerDay
	cfg.Balance.BloomDealerWindowHours = limitWindowHours
	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		bloomWaferItemId: {ItemId: bloomWaferItemId, Name: "Bloom Wafer", Type: items.Potion, Subtype: items.Drinkable, Uses: 1, Value: 120, Weight: 0.1},
		sellTestItemId:   {ItemId: sellTestItemId, Name: "iron sword", Type: items.Weapon, Value: 100},
	}))
	shops.ClearCache()
	t.Cleanup(shops.ClearCache)
	t.Cleanup(util.ResetRoundCountForTest)
	gametime.ClearDateCacheForTest()
}

func limitBuyer(gold int) *MobActor {
	b := newBuyItemIdBuyer()
	b.Mob.Character.Gold = gold
	return b
}

func limitSeller() (*mobs.Mob, *shops.ShopInventory) {
	inv := shops.RegisterShop("LimitZone", 9802, 5530, shops.ShopInventory{Gold: 500, StartingGold: 500, CraftSupport: shops.CraftSupportAlchemy,
		Stock: []shops.StockEntry{
			{ItemId: bloomWaferItemId, RestockQty: 5, MaxStock: 50, Current: 50},
			{ItemId: sellTestItemId, RestockQty: 5, MaxStock: 50, Current: 50},
		}})
	return &mobs.Mob{InstanceId: limitSellerId, Character: characters.Character{Name: "Sefton"}}, inv
}

// buyOnce is one unit, as Buy's closure runs it, success bookkeeping included.
func buyOnce(buyer Actor, seller *mobs.Mob, inv *shops.ShopInventory, item string) BuyResult {
	r := tryPurchaseFromInventory(buyer, item, seller, inv)
	if r.Success {
		recordLimitedBuy(buyer.GetCharacter(), r.ItemId)
	}
	return r
}

func TestBuyLimit_RefusesPastTheLimitAndChangesNothing(t *testing.T) {
	pinBuyLimit(t)
	buyer := limitBuyer(100000)
	seller, inv := limitSeller()
	events.DrainQueuedInputsForTest(limitSellerId)

	for i := 0; i < limitPerDay; i++ {
		require.True(t, buyOnce(buyer, seller, inv, "bloom wafer").Success, "unit %d", i+1)
	}
	require.Empty(t, events.DrainQueuedInputsForTest(limitSellerId), "he says nothing while he is selling")

	gold, stock, sales := buyer.Mob.Character.Gold, inv.GetStock(bloomWaferItemId).Current, inv.SalesCount
	shopGold, held := inv.Gold, countDrinkItemsById(buyer.Mob.Character.Items, bloomWaferItemId)

	res := buyOnce(buyer, seller, inv, "bloom wafer")
	assert.False(t, res.Success)
	assert.Equal(t, BuyReasonLimit, res.Reason)
	assert.Equal(t, gold, buyer.Mob.Character.Gold, "charges nothing")
	assert.Equal(t, stock, inv.GetStock(bloomWaferItemId).Current, "stock is unchanged")
	assert.Equal(t, sales, inv.SalesCount)
	assert.Equal(t, shopGold, inv.Gold)
	assert.Equal(t, held, countDrinkItemsById(buyer.Mob.Character.Items, bloomWaferItemId), "moves nothing")

	said := strings.Join(events.DrainQueuedInputsForTest(limitSellerId), " ")
	assert.Contains(t, said, "That is enough from me for one day. Come back tomorrow, if you still want to.")

	assert.True(t, buyOnce(buyer, seller, inv, "iron sword").Success, "the limit is on the wafer and on nothing else")
}

// One character's limit is that character's.
func TestBuyLimit_IsPerCharacter(t *testing.T) {
	pinBuyLimit(t)
	first, second := limitBuyer(100000), limitBuyer(100000)
	seller, inv := limitSeller()
	for i := 0; i < limitPerDay; i++ {
		require.True(t, buyOnce(first, seller, inv, "bloom wafer").Success)
	}
	assert.Equal(t, BuyReasonLimit, buyOnce(first, seller, inv, "bloom wafer").Reason)
	assert.True(t, buyOnce(second, seller, inv, "bloom wafer").Success)
}

// A rolling window: each purchase frees its own place when it ages out. Waiting
// for the first purchase does not free the ones bought later.
func TestBuyLimit_WindowRolls(t *testing.T) {
	pinBuyLimit(t)
	buyer := limitBuyer(100000)
	seller, inv := limitSeller()

	start := util.GetRoundCount()
	window := gametime.GetDate(start).AddPeriod("24 real hours") - start
	require.Greater(t, window, uint64(1000), "24 real hours is a real length of rounds")

	require.True(t, buyOnce(buyer, seller, inv, "bloom wafer").Success)
	util.SetRoundCountForTest(start + window/2)
	gametime.ClearDateCacheForTest()
	require.True(t, buyOnce(buyer, seller, inv, "bloom wafer").Success)
	require.True(t, buyOnce(buyer, seller, inv, "bloom wafer").Success)
	assert.Equal(t, BuyReasonLimit, buyOnce(buyer, seller, inv, "bloom wafer").Reason)

	// Just past the first purchase's window: one place, no more.
	util.SetRoundCountForTest(start + window + 2)
	gametime.ClearDateCacheForTest()
	assert.True(t, buyOnce(buyer, seller, inv, "bloom wafer").Success, "the first purchase has aged out")
	assert.Equal(t, BuyReasonLimit, buyOnce(buyer, seller, inv, "bloom wafer").Reason, "the other two have not")
}

// The window is a config knob, read when the purchase is made.
func TestBuyLimit_ReadsTheKnobs(t *testing.T) {
	pinBuyLimit(t)
	cfg := configs.GetConfig()
	cfg.Balance.BloomDealerWafersPerDay = 1
	cfg.Balance.BloomDealerWindowHours = 2
	configs.SetConfigForTest(t, cfg)

	buyer := limitBuyer(100000)
	seller, inv := limitSeller()
	require.True(t, buyOnce(buyer, seller, inv, "bloom wafer").Success)
	assert.Equal(t, BuyReasonLimit, buyOnce(buyer, seller, inv, "bloom wafer").Reason, "a limit of one")

	start := util.GetRoundCount()
	short := gametime.GetDate(start).AddPeriod("2 real hours") - start
	util.SetRoundCountForTest(start + short + 2)
	gametime.ClearDateCacheForTest()
	assert.True(t, buyOnce(buyer, seller, inv, "bloom wafer").Success, "a window of two hours")
}

// The count is saved with the character, so a relog waits out nothing.
func TestBuyLimit_SurvivesASaveAndLoad(t *testing.T) {
	pinBuyLimit(t)
	buyer := limitBuyer(100000)
	seller, inv := limitSeller()
	for i := 0; i < limitPerDay; i++ {
		require.True(t, buyOnce(buyer, seller, inv, "bloom wafer").Success)
	}

	out, err := yaml.Marshal(&buyer.Mob.Character)
	require.NoError(t, err)
	loaded := characters.New()
	require.NoError(t, yaml.Unmarshal(out, loaded))
	assert.True(t, BuyLimitReached(loaded, bloomWaferItemId))

	buyer.Mob.Character = *loaded
	buyer.Mob.Character.Gold = 100000
	buyer.Mob.Character.Stats.Strength.ValueAdj = 100
	assert.Equal(t, BuyReasonLimit, buyOnce(buyer, seller, inv, "bloom wafer").Reason)
}

// Both purchase paths ask: a legacy shop mob, and a player's own stall.
func TestBuyLimit_HoldsOnTheLegacyPath(t *testing.T) {
	pinBuyLimit(t)
	buyer := limitBuyer(100000)
	for i := 0; i < limitPerDay; i++ {
		recordLimitedBuy(&buyer.Mob.Character, bloomWaferItemId)
	}
	shop := characters.Shop{{ItemId: bloomWaferItemId, Price: 150, Quantity: 2, QuantityMax: 2}}

	seller := &mobs.Mob{InstanceId: limitSellerId, Character: characters.Character{Name: "Sefton", Shop: shop}}
	events.DrainQueuedInputsForTest(limitSellerId)
	res := tryPurchaseLegacy(buyer, "bloom wafer", seller, nil)
	assert.Equal(t, BuyReasonLimit, res.Reason)
	assert.Equal(t, 2, seller.Character.Shop[0].Quantity)
	assert.Equal(t, 100000, buyer.Mob.Character.Gold)
	assert.Contains(t, strings.Join(events.DrainQueuedInputsForTest(limitSellerId), " "), "That is enough from me")

	vendor := users.NewUserRecord(9051, 0)
	vendor.Character.Name = "Vendor"
	vendor.Character.Shop = characters.Shop{{ItemId: bloomWaferItemId, Price: 150, Quantity: 2, QuantityMax: 2}}
	res = tryPurchaseLegacy(buyer, "bloom wafer", nil, vendor)
	assert.Equal(t, BuyReasonLimit, res.Reason)
	assert.Equal(t, 2, vendor.Character.Shop[0].Quantity)
}

// The list keeps showing the stock, and says when this viewer can buy no more.
func TestBuyLimitNote_NamesTheItemAtTheLimitOnly(t *testing.T) {
	pinBuyLimit(t)
	c := characters.New()
	stock := characters.Shop{{ItemId: sellTestItemId, Quantity: 1}, {ItemId: bloomWaferItemId, Quantity: 5}}

	assert.Empty(t, BuyLimitNote(c, stock, "Sefton"))
	for i := 0; i < limitPerDay; i++ {
		recordLimitedBuy(c, bloomWaferItemId)
	}
	note := BuyLimitNote(c, stock, "Sefton")
	assert.Contains(t, note, "Sefton")
	assert.Contains(t, note, "Bloom Wafer")
	assert.NotRegexp(t, `[0-9]`, strings.ReplaceAll(note, `fg="`, ``), "no numbers")

	assert.Empty(t, BuyLimitNote(c, characters.Shop{{ItemId: sellTestItemId, Quantity: 1}}, "Sefton"),
		"a merchant with no wafer has nothing to say of it")
}
