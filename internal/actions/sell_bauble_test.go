package actions

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bauble selling (docs/baubles Phase 2). These run against the same legacy
// merchant the other sell tests use, plus a registered living-economy shop
// where a test needs one.

// seedBaubleSale seeds the carrier (and the normal sell item the legacy
// merchant stocks) and points the bauble catalog at a temp dir.
func seedBaubleSale(t *testing.T) {
	t.Helper()
	restore := items.SeedItemsForTest(map[int]*items.ItemSpec{
		items.BaubleItemId: {
			ItemId:     items.BaubleItemId,
			Name:       "Curious Trinket",
			NameSimple: "trinket",
			Type:       items.Object,
			Subtype:    items.Mundane,
			Weight:     0.2,
			Value:      1,
			NotSalable: true,
		},
		sellTestItemId: {
			ItemId: sellTestItemId,
			Name:   "iron sword",
			Type:   items.Weapon,
			Value:  100,
		},
	})
	baubles.SetDirForTest(t.TempDir())
	t.Cleanup(func() {
		restore()
		items.SetBaubleResolver(nil)
	})
}

// newBauble creates a catalog record and the item that points at it.
func newBauble(t *testing.T, name string, noun string, value int, status baubles.Status) items.Item {
	t.Helper()
	rec, err := baubles.Create(baubles.Record{
		Name:       name,
		NameSimple: noun,
		Tier:       baubles.TierAverage,
		Value:      value,
		WeightLbs:  0.6,
		Status:     status,
		Source:     baubles.SourceSearch,
	})
	require.NoError(t, err)
	it := items.New(items.BaubleItemId)
	it.Bauble = rec.Id
	return it
}

func TestBaublePrice(t *testing.T) {
	// BuyRatio defaults to 0.50; rounded up, at least 1.
	assert.Equal(t, 1, BaublePrice(1))
	assert.Equal(t, 3, BaublePrice(5))
	assert.Equal(t, 6, BaublePrice(12))
	assert.Equal(t, 100, BaublePrice(200))
	assert.Equal(t, 1, BaublePrice(0))
}

func TestSell_Bauble_LegacyMerchantPaysCatalogValueAndDoesNotStock(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	b := newBauble(t, "Painted Wooden Horse", "horse", 12, baubles.StatusFallback)
	require.True(t, char.StoreItem(b))
	merchantBefore := merchantInstance().Character.Gold

	res := Sell(seller, SellOptions{ItemName: "horse", Quantity: 1})

	require.Equal(t, SellStopSoldAll, res.Reason, "res=%+v", res)
	assert.Equal(t, 1, res.Sold)
	assert.Equal(t, 6, char.Gold, "paid Value*BuyRatio from the catalog")
	assert.Equal(t, merchantBefore-6, merchantInstance().Character.Gold, "player sale drains the merchant")
	assert.Equal(t, "Painted Wooden Horse", res.LastItemName)
	_, still := char.FindInBackpack("horse")
	assert.False(t, still, "the bauble leaves the seller")
	assert.Equal(t, 0, shopQty(merchantInstance(), items.BaubleItemId), "the carrier is never stocked")

	rec, _ := baubles.Get(b.Bauble)
	assert.Equal(t, baubles.StatusSold, rec.Status)
	assert.Equal(t, 6, rec.SoldValue)
}

func TestSell_Bauble_ByGenericKeyword(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(newBauble(t, "Chipped Clay Cup", "cup", 4, baubles.StatusReady)))

	res := Sell(seller, SellOptions{ItemName: "bauble", Quantity: 1})
	assert.Equal(t, 1, res.Sold, "every bauble answers to `bauble`")
	assert.Equal(t, 2, char.Gold)
}

func TestSell_Bauble_UnknownRecordIsRefused(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	orphan := items.New(items.BaubleItemId)
	orphan.Bauble = "B9999999"
	require.True(t, char.StoreItem(orphan))

	res := Sell(seller, SellOptions{ItemName: "bauble", Quantity: 1})
	assert.Equal(t, 0, res.Sold)
	assert.Equal(t, 0, char.Gold)
}

func TestSell_Bauble_AllBaublesAreMixed(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(newBauble(t, "Tarnished Copper Button", "button", 2, baubles.StatusFallback)))
	require.True(t, char.StoreItem(newBauble(t, "Painted Wooden Horse", "horse", 12, baubles.StatusReady)))

	res := Sell(seller, SellOptions{ItemName: "bauble", Quantity: UnlimitedSell})

	assert.Equal(t, 2, res.Sold)
	assert.Equal(t, 1+6, res.TotalGold)
	assert.True(t, res.Mixed, "two different baubles must not be pluralised as one name")
}

func TestSell_SameItemTwiceIsNotMixed(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()

	seller := newSellerActor(t, true, sellTestItemId, sellTestItemId)
	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: UnlimitedSell})
	assert.Equal(t, 2, res.Sold)
	assert.False(t, res.Mixed)
	assert.Equal(t, "iron sword", res.LastItemName)
}

func TestSell_Bauble_LivingShopByCraftSupport(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()

	shops.IsolateForTest(t)

	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000})

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	b := newBauble(t, "Painted Wooden Horse", "horse", 12, baubles.StatusReady)
	require.True(t, char.StoreItem(b))

	// A blacksmith does not buy trinkets.
	si.CraftSupport = shops.CraftSupportBlacksmithing
	res := Sell(seller, SellOptions{ItemName: "horse", Quantity: 1})
	assert.Equal(t, 0, res.Sold)
	assert.Equal(t, SellStopRejected, res.Reason)
	assert.Equal(t, 1000, si.Gold)

	// A general store does, from its own gold, and puts it on its shelf at
	// its catalog value (baubles slice D).
	si.CraftSupport = shops.CraftSupportGeneral
	res = Sell(seller, SellOptions{ItemName: "horse", Quantity: 1})
	require.Equal(t, 1, res.Sold, "res=%+v", res)
	assert.Equal(t, 6, char.Gold)
	assert.Equal(t, 994, si.Gold, "a living-economy sale drains the shop's gold")
	assert.Nil(t, si.GetStock(items.BaubleItemId), "the carrier is never stocked")
	require.Len(t, si.AffixedStock, 1, "an average bauble a player sells is shelved")
	assert.Equal(t, b.Bauble, si.AffixedStock[0].Item.Bauble)
	assert.Equal(t, 12, si.AffixedStock[0].Price, "relisted at its catalog value")
	assert.True(t, si.AffixedStock[0].HoldUntil.IsZero(), "an honest bauble is listed at once")
}

func TestSell_Bauble_LivingShopReserveRefuses(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()

	shops.IsolateForTest(t)

	// Starting gold 1000 keeps 500 in reserve; 503 on hand cannot pay 6.
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 503, StartingGold: 1000, CraftSupport: shops.CraftSupportJewelcrafting})

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(newBauble(t, "Painted Wooden Horse", "horse", 12, baubles.StatusReady)))

	res := Sell(seller, SellOptions{ItemName: "horse", Quantity: 1})
	assert.Equal(t, 0, res.Sold)
	assert.Equal(t, SellStopMerchantBroke, res.Reason)
	assert.Equal(t, 503, si.Gold)
}

func TestSell_Bauble_LegacyGetSellPriceNeverPricesIt(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()

	b := newBauble(t, "Painted Wooden Horse", "horse", 12, baubles.StatusReady)
	assert.Equal(t, 0, merchantInstance().GetSellPrice(b), "the ItemId-keyed path must never price a bauble")

	offer := BaubleOfferFrom(b, merchantInstance())
	assert.Equal(t, 6, offer.Price, "the bauble offer is the one to use")
}

// A crash can roll a seller's save back past a sale: the bauble is in the
// pack again while its record says sold. It sells again like any bauble, and
// the new sale is recorded (the catalog sweep keeps the record while it is
// held: internal/baubles/sweep.go).
func TestSell_Bauble_ASoldRecordSellsAgain(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	b := newBauble(t, "Painted Wooden Horse", "horse", 12, baubles.StatusFallback)
	longAgo := time.Now().UTC().Add(-40 * 24 * time.Hour)
	baubles.Update(b.Bauble, func(r *baubles.Record) {
		r.Status, r.SoldAt, r.SoldValue = baubles.StatusSold, longAgo, 6
	})
	require.True(t, char.StoreItem(b))

	res := Sell(seller, SellOptions{ItemName: "horse", Quantity: 1})

	require.Equal(t, SellStopSoldAll, res.Reason, "res=%+v", res)
	assert.Equal(t, 6, char.Gold, "paid from the catalog, as for any bauble")
	rec, _ := baubles.Get(b.Bauble)
	assert.True(t, rec.SoldAt.After(longAgo), "the new sale is recorded")
}

// Only a player's sale of a shelvable bauble to a living shop shelves it
// (baubles slice D, spec test 7): a mob's sale (ruling 7: the shop paid
// nothing), a cheap bauble (ruling 5) and a retired one (ruling 1) are
// destroyed, and each record is still marked sold.
func TestSell_Bauble_OnlyAPlayersShelvableSaleIsShelved(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()

	shops.IsolateForTest(t)
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, CraftSupport: shops.CraftSupportGeneral})

	mob := newSellerActor(t, false)
	mob.GetCharacter().Stats.Strength.ValueAdj = 100 // a bare mob fixture has no carrying capacity
	fromMob := newBauble(t, "Painted Wooden Horse", "horse", 12, baubles.StatusReady)
	require.True(t, mob.GetCharacter().StoreItem(fromMob))
	res := Sell(mob, SellOptions{ItemName: "horse", Quantity: 1})
	require.Equal(t, 1, res.Sold, "res=%+v", res)

	player := newSellerActor(t, true)
	cheap := newBauble(t, "Chipped Clay Cup", "cup", 4, baubles.StatusReady)
	retired := newBauble(t, "Rude Carving", "carving", 12, baubles.StatusRetired)
	require.True(t, player.GetCharacter().StoreItem(cheap))
	require.True(t, player.GetCharacter().StoreItem(retired))
	require.Equal(t, 1, Sell(player, SellOptions{ItemName: "cup", Quantity: 1}).Sold)
	require.Equal(t, 1, Sell(player, SellOptions{ItemName: "trinket", Quantity: 1}).Sold, "a retired bauble reads Trinket")

	assert.Empty(t, si.AffixedStock, "none of the three is shelved")
	for _, it := range []items.Item{fromMob, cheap, retired} {
		rec, _ := baubles.Get(it.Bauble)
		assert.Equal(t, baubles.StatusSold, rec.Status, "%s: the sale is recorded", it.Bauble)
	}
}

// The cheap-tier boundary (owner ruling 5, baubleShelvable): a bauble worth
// exactly the cheap tier's max is still cheap and must not be shelved; one
// worth one gold more crosses into average and must be. The boundary is read
// from the tier config, never hardcoded, so this pins the `>` in
// baubleShelvable against a `>=` regression.
func TestSell_Bauble_CheapTierBoundaryIsPinned(t *testing.T) {
	seedBaubleSale(t)
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 0)()

	shops.IsolateForTest(t)
	si := shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, CraftSupport: shops.CraftSupportGeneral})

	max := baubles.TierCheap.Range().Max

	player := newSellerActor(t, true)
	atMax := newBauble(t, "Chipped Clay Cup", "cup", max, baubles.StatusReady)
	require.True(t, player.GetCharacter().StoreItem(atMax))
	require.Equal(t, 1, Sell(player, SellOptions{ItemName: "cup", Quantity: 1}).Sold)
	assert.Empty(t, si.AffixedStock, "a bauble worth exactly the cheap tier max must not be shelved")

	aboveMax := newBauble(t, "Painted Wooden Horse", "horse", max+1, baubles.StatusReady)
	require.True(t, player.GetCharacter().StoreItem(aboveMax))
	require.Equal(t, 1, Sell(player, SellOptions{ItemName: "horse", Quantity: 1}).Sold)
	require.Len(t, si.AffixedStock, 1, "a bauble one gold above the cheap tier max must be shelved")
	assert.Equal(t, aboveMax.Bauble, si.AffixedStock[0].Item.Bauble)
}
