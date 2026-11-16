package actions

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Buying off the secondhand shelf (baubles slice D).

// shelfBuyFixture seeds the bauble sale harness (carrier, catalog, room 1),
// pins the shelf clock at now and the cap at limit, points DataFiles at a
// temp dir, and registers a living shop (TestZone, template 2, room 1) with
// an empty shelf.
func shelfBuyFixture(t *testing.T, limit int, now time.Time) *shops.ShopInventory {
	t.Helper()
	seedBaubleSale(t)
	t.Cleanup(seedSellRoom(t))
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	cfg.Balance.ShopAffixedStockCap = configs.ConfigInt(limit)
	configs.SetConfigForTest(t, cfg)
	orig := shops.ShelfNow
	shops.ShelfNow = func() time.Time { return now }
	t.Cleanup(func() { shops.ShelfNow = orig })
	shops.IsolateForTest(t)
	return shops.RegisterShop("TestZone", 2, 1, shops.ShopInventory{Gold: 1000, StartingGold: 1000, CraftSupport: shops.CraftSupportGeneral})
}

// shelfBuyer is a player standing in room 1 with gold and the strength to
// carry anything on a shelf; no Bartering, so prices are undiscounted.
func shelfBuyer(t *testing.T, userId int, gold int) *UserActor {
	t.Helper()
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)
	u := users.NewTestUser(userId, fmt.Sprintf("buyer%d", userId), fmt.Sprintf("Buyer%d", userId), uint64(userId))
	u.Character.RoomId = 1
	u.Character.Gold = gold
	u.Character.Conditions = conditions.New()
	u.Character.Stats.Strength.ValueAdj = 100
	return &UserActor{User: u, Room: room}
}

// Spec test 9: two Trinket rows at different prices with a held Trinket
// between them. `buy 2.trinket` takes and charges the second LISTED row and
// removes its own AffixedStock index; `buy trinket` then takes the first.
func TestBuy_Shelf_SameNameRowsAreChosenByPosition(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 12, now)
	first := newBauble(t, "Trinket", "trinket", 12, baubles.StatusFallback)
	held := newBauble(t, "Trinket", "trinket", 13, baubles.StatusFallback)
	second := newBauble(t, "Trinket", "trinket", 14, baubles.StatusFallback)
	si.AffixedStock = []shops.AffixedStockEntry{
		{Item: first, Price: 12, AddedAt: now.Add(-3 * time.Hour)},
		{Item: held, Price: 13, AddedAt: now.Add(-2 * time.Hour), HoldUntil: now.Add(time.Hour)},
		{Item: second, Price: 14, AddedAt: now.Add(-time.Hour)},
	}
	buyer := shelfBuyer(t, 1, 100)

	res := tryPurchaseFromInventory(buyer, "2.trinket", nil, si)
	require.True(t, res.Success, "res=%+v", res)
	assert.Equal(t, 86, buyer.GetCharacter().Gold, "charged the second listed row's own price")
	require.Len(t, si.AffixedStock, 2)
	assert.Equal(t, first.Bauble, si.AffixedStock[0].Item.Bauble, "the first row stays")
	assert.Equal(t, held.Bauble, si.AffixedStock[1].Item.Bauble, "the held entry between them is never counted")

	res = tryPurchaseFromInventory(buyer, "trinket", nil, si)
	require.True(t, res.Success, "res=%+v", res)
	assert.Equal(t, 74, buyer.GetCharacter().Gold, "then the first row, at its own price")
	require.Len(t, si.AffixedStock, 1)
	assert.Equal(t, held.Bauble, si.AffixedStock[0].Item.Bauble)
}

// #865: `buy pot` at Trader Onna bought the Small Red Potion, listed first,
// instead of the Rusty Pot. A whole word now beats a word that merely starts
// with the text; the potion is still there to buy by its own word.
func TestBuy_Shelf_WholeWordBeatsWordStart(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 12, now)
	gear := func(name string) items.Item {
		return items.Item{ItemId: sellTestItemId, Affixed: true, Spec: &items.ItemSpec{ItemId: sellTestItemId, Name: name, Type: items.Weapon, Value: 50}}
	}
	si.AffixedStock = []shops.AffixedStockEntry{
		{Item: gear("Small Red Potion"), Price: 20, AddedAt: now.Add(-2 * time.Hour)},
		{Item: gear("Rusty Pot"), Price: 30, AddedAt: now.Add(-time.Hour)},
	}
	buyer := shelfBuyer(t, 1, 100)

	res := tryPurchaseFromInventory(buyer, "pot", nil, si)
	require.True(t, res.Success, "res=%+v", res)
	assert.Equal(t, 70, buyer.GetCharacter().Gold, "charged the Rusty Pot's price")
	require.Len(t, si.AffixedStock, 1)
	assert.Equal(t, "Small Red Potion", si.AffixedStock[0].Item.GetSpec().Name, "the potion stays on the shelf")
}

// Spec tests 1 and 2 (buy half): a held entry cannot be bought by its name;
// once its hold ends (the shelf clock moves on) it can.
func TestBuy_Shelf_AHeldEntryIsNotForSaleUntilItsHoldEnds(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 12, now)
	dice := newBauble(t, "Bone Dice", "dice", 12, baubles.StatusReady)
	si.AffixedStock = []shops.AffixedStockEntry{{Item: dice, Price: 12, AddedAt: now, HoldUntil: now.Add(time.Hour)}}
	buyer := shelfBuyer(t, 1, 100)

	res := tryPurchaseFromInventory(buyer, "bone dice", nil, si)
	assert.Equal(t, BuyReasonNoMatch, res.Reason, "held: no row answers to its name")
	assert.Len(t, si.AffixedStock, 1)

	shops.ShelfNow = func() time.Time { return now.Add(time.Hour) }
	res = tryPurchaseFromInventory(buyer, "bone dice", nil, si)
	require.True(t, res.Success, "the hold is over: res=%+v", res)
	assert.Empty(t, si.AffixedStock)
}

// Spec test 5 (buy half): buy enforces the cap lazily once a hold has ended,
// evicting the entry listed earliest, and saves the trim even when nothing
// is bought.
func TestBuy_Shelf_EnforcesTheCapLazilyAndSavesTheTrim(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 1, now)
	gear := func(name string) items.Item {
		return items.Item{ItemId: sellTestItemId, Affixed: true, Spec: &items.ItemSpec{ItemId: sellTestItemId, Name: name, Type: items.Weapon, Value: 50}}
	}
	si.AffixedStock = []shops.AffixedStockEntry{
		{Item: gear("keen torc"), Price: 50, AddedAt: now.Add(-80 * time.Hour), HoldUntil: now.Add(-time.Hour)},
		{Item: gear("warding ring"), Price: 50, AddedAt: now.Add(-10 * time.Hour)},
	}
	buyer := shelfBuyer(t, 1, 0)

	res := tryPurchaseFromInventory(buyer, "nothing like this", nil, si)
	assert.Equal(t, BuyReasonNoMatch, res.Reason)
	require.Len(t, si.AffixedStock, 1, "over the cap once the hold ended: trimmed on buy")
	assert.Equal(t, "keen torc", si.AffixedStock[0].Item.GetSpec().Name, "the one listed earliest went, not the one shelved first")

	shops.ClearCache()
	reloaded := shops.GetShopInventory("TestZone", 2, 1)
	require.NotNil(t, reloaded, "the trim was saved though nothing was bought")
	assert.Len(t, reloaded.AffixedStock, 1)
}

// Spec test 11 (buy half) and ruling 4: a finder-only bauble on a shelf is
// matched and named in the buyer's own view. Its finder buys it by its own
// name and reads that name on the purchase line; anyone else buys a Trinket,
// and the hidden words match nothing for them.
func TestBuy_Shelf_AFinderOnlyBaubleIsBoughtInTheBuyersOwnView(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 12, now)
	const finderId, otherId = 1, 2
	rec, err := baubles.Create(baubles.Record{Name: "Painted Wooden Horse", NameSimple: "horse", Tier: baubles.TierAverage,
		Value: 12, WeightLbs: 0.6, Description: "A child's toy horse, its red paint flaking.", Status: baubles.StatusReady,
		Source: baubles.SourceSearch, PlayerKey: true, Moderated: false, FoundByUserId: finderId})
	require.NoError(t, err)
	shelve := func() {
		si.AffixedStock = []shops.AffixedStockEntry{{Item: items.Item{ItemId: items.BaubleItemId, Bauble: rec.Id}, Price: 12, AddedAt: now}}
	}

	other := shelfBuyer(t, otherId, 100)
	shelve()
	res := tryPurchaseFromInventory(other, "painted wooden horse", nil, si)
	assert.Equal(t, BuyReasonNoMatch, res.Reason, "the hidden words match nothing for anyone but the finder")
	events.DrainQueuedMessagesForTest(otherId)
	res = tryPurchaseFromInventory(other, "trinket", nil, si)
	require.True(t, res.Success, "res=%+v", res)
	out := strings.Join(events.DrainQueuedMessagesForTest(otherId), "\n")
	assert.Contains(t, out, "Trinket")
	assert.NotContains(t, out, "Horse")

	finder := shelfBuyer(t, finderId, 100)
	shelve()
	events.DrainQueuedMessagesForTest(finderId)
	res = tryPurchaseFromInventory(finder, "painted wooden horse", nil, si)
	require.True(t, res.Success, "the finder buys it by its own name: res=%+v", res)
	out = strings.Join(events.DrainQueuedMessagesForTest(finderId), "\n")
	assert.Contains(t, out, "You buy the")
	assert.Contains(t, out, "Painted Wooden Horse", "the purchase line shows the finder their own name")
}

// Spec test 12 (buy half): bought back off the shelf, a sold record returns
// to its unsold status (ruling 2), named to ready and generic to fallback,
// and its sale still counts.
func TestBuy_Shelf_ABuybackReturnsTheRecordToItsUnsoldStatus(t *testing.T) {
	now := stolenTestNow
	si := shelfBuyFixture(t, 12, now)
	named := newBauble(t, "Bone Dice", "dice", 12, baubles.StatusReady)
	baubles.Update(named.Bauble, func(r *baubles.Record) { r.Generator = baubles.GeneratorCorpus })
	generic := newBauble(t, "Brass Thimble", "thimble", 11, baubles.StatusFallback)
	since := time.Now().UTC().Add(-time.Second)
	require.True(t, baubles.MarkSold(named.Bauble, 6, 9))
	require.True(t, baubles.MarkSold(generic.Bauble, 6, 9))
	si.AffixedStock = []shops.AffixedStockEntry{{Item: named, Price: 12, AddedAt: now}, {Item: generic, Price: 11, AddedAt: now}}
	buyer := shelfBuyer(t, 1, 100)

	require.True(t, tryPurchaseFromInventory(buyer, "bone dice", nil, si).Success)
	require.True(t, tryPurchaseFromInventory(buyer, "brass thimble", nil, si).Success)
	r1, _ := baubles.Get(named.Bauble)
	r2, _ := baubles.Get(generic.Bauble)
	assert.Equal(t, baubles.StatusReady, r1.Status, "a named record is ready again")
	assert.Equal(t, baubles.StatusFallback, r2.Status, "a generic one is fallback again")
	n, _ := baubles.SalesSince(since)
	assert.Equal(t, 2, n, "a buyback does not erase the sale")
}
