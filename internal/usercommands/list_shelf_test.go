package usercommands

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The secondhand shelf in `list` (baubles slice D). The table's text cannot
// be read here (no templates filesystem in this test binary, see
// list_sight_test.go), so these check the rows, whether a table is shown,
// and the shop's saved state.

var shelfListNow = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

// shelfListFixture is listSightRoom's lit shop (merchant 841, room 8410,
// shopper 8411) with a registered living shop, the shelf clock pinned at
// shelfListNow, the cap pinned, and DataFiles and the catalog in temp dirs.
func shelfListFixture(t *testing.T, limit int) (*users.UserRecord, *rooms.Room, *shops.ShopInventory) {
	t.Helper()
	user, room := listSightRoom(t, "city")
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(t.TempDir())
	cfg.Balance.ShopAffixedStockCap = configs.ConfigInt(limit)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		84001: {ItemId: 84001, Name: "tin cup", Type: items.Object, Value: 50},
		items.BaubleItemId: {ItemId: items.BaubleItemId, Name: "Curious Trinket", NameSimple: "trinket",
			Type: items.Object, Subtype: items.Mundane, Weight: 0.2, Value: 1, NotSalable: true},
	}))
	baubles.SetDirForTest(t.TempDir())
	t.Cleanup(func() { items.SetBaubleResolver(nil) })
	orig := shops.ShelfNow
	shops.ShelfNow = func() time.Time { return shelfListNow }
	t.Cleanup(func() { shops.ShelfNow = orig })
	shops.IsolateForTest(t)
	si := shops.RegisterShop("TestZone", 841, 8410, shops.ShopInventory{Gold: 100, StartingGold: 100, CraftSupport: shops.CraftSupportGeneral})
	return user, room, si
}

// shelfGear is affixed gear on the shelf.
func shelfGear(name string, price int, addedAt, holdUntil time.Time) shops.AffixedStockEntry {
	return shops.AffixedStockEntry{
		Item:  items.Item{ItemId: 84001, Affixed: true, Spec: &items.ItemSpec{ItemId: 84001, Name: name, Type: items.Object, Value: price}},
		Price: price, AddedAt: addedAt, HoldUntil: holdUntil,
	}
}

func plainRow(row []string) string {
	return listSightTag.ReplaceAllString(strings.Join(row, " | "), "")
}

// Spec test 1 (list half): the shelf lists in shelf order, unsorted, without
// the held entry, at each entry's own price; with only held entries nothing
// is shown, so the "nothing to sell" line can fire.
func TestListShelf_ShowsListedEntriesInShelfOrder(t *testing.T) {
	user, _, si := shelfListFixture(t, 12)
	// Shelf order Zinc, Amber, Copper differs from every sort a renderer
	// might slip in: by name (Amber, Copper, Zinc), by price ascending
	// (Amber 10, Copper 20, Zinc 30) and by price descending (Zinc, Copper,
	// Amber).
	si.AffixedStock = []shops.AffixedStockEntry{
		shelfGear("Zinc Ring", 30, shelfListNow.Add(-3*time.Hour), time.Time{}),
		shelfGear("Held Torc", 90, shelfListNow.Add(-2*time.Hour), shelfListNow.Add(time.Hour)),
		shelfGear("Amber Brooch", 10, shelfListNow.Add(-time.Hour), time.Time{}),
		shelfGear("Copper Pin", 20, shelfListNow.Add(-time.Minute), time.Time{}),
	}

	rows := buildShelfRows(si, user.UserId, shelfListNow)
	require.Len(t, rows, 3, "the held entry is out of sight")
	for i, want := range []struct{ name, price string }{{"Zinc Ring", "30"}, {"Amber Brooch", "10"}, {"Copper Pin", "20"}} {
		assert.Contains(t, plainRow(rows[i]), want.name, "row %d: shelf order, not sorted by name or price", i)
		assert.Equal(t, want.price, rows[i][2], "row %d: its own price", i)
	}
	for _, r := range rows {
		assert.NotContains(t, plainRow(r), "Held Torc")
		assert.Len(t, r, 3, "Name, Type, Price")
	}
	assert.True(t, renderShelfListing(user, si, "Keeper", shelfListNow))

	si.AffixedStock = si.AffixedStock[1:2] // only the held one
	assert.False(t, renderShelfListing(user, si, "Keeper", shelfListNow), "nothing listed: no table, so list may say it has nothing")
}

// The "nothing to sell" say fires only when both tables are empty: a shop
// with no stock but a listed shelf says nothing, and one whose shelf holds
// only held entries says it.
func TestListShelf_NothingToSellOnlyWhenBothTablesAreEmpty(t *testing.T) {
	user, room, si := shelfListFixture(t, 12)
	si.AffixedStock = []shops.AffixedStockEntry{shelfGear("Zinc Ring", 30, shelfListNow, time.Time{})}
	events.DrainQueuedInputsForTest(8412)
	_, err := List("", user, room, 0)
	require.NoError(t, err)
	for _, in := range events.DrainQueuedInputsForTest(8412) {
		assert.NotContains(t, in, "nothing to sell", "a listed shelf is something to sell")
	}

	si.AffixedStock = []shops.AffixedStockEntry{shelfGear("Held Torc", 90, shelfListNow, shelfListNow.Add(time.Hour))}
	_, err = List("", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, strings.Join(events.DrainQueuedInputsForTest(8412), "\n"), "nothing to sell", "only held entries: nothing to show")
}

// Spec tests 4 and 5 (list half): list enforces the cap lazily once a hold
// has ended, evicting the entry listed earliest (not the one shelved first),
// and saves the trimmed shop.
func TestListShelf_EnforcesTheCapLazilyAndSaves(t *testing.T) {
	user, room, si := shelfListFixture(t, 2)
	si.AffixedStock = []shops.AffixedStockEntry{
		shelfGear("Old Torc", 50, shelfListNow.Add(-80*time.Hour), shelfListNow.Add(-time.Hour)), // shelved first, listed an hour ago
		shelfGear("Bone Ring", 40, shelfListNow.Add(-10*time.Hour), time.Time{}),
		shelfGear("Jet Pin", 30, shelfListNow.Add(-5*time.Hour), time.Time{}),
	}

	handled, err := List("", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	names := func(inv *shops.ShopInventory) []string {
		out := []string{}
		for _, e := range inv.AffixedStock {
			out = append(out, e.Item.GetSpec().Name)
		}
		return out
	}
	assert.Equal(t, []string{"Old Torc", "Jet Pin"}, names(si), "Bone Ring listed earliest and goes")

	shops.ClearCache()
	reloaded := shops.GetShopInventory("TestZone", 841, 8410)
	require.NotNil(t, reloaded, "the trim was saved")
	assert.Equal(t, []string{"Old Torc", "Jet Pin"}, names(reloaded))
}

// Spec test 11 (list half): a finder-only bauble on the shelf reads its own
// name to its finder and Trinket to anyone else.
func TestListShelf_AFinderOnlyBaubleReadsByViewer(t *testing.T) {
	user, _, si := shelfListFixture(t, 12)
	rec, err := baubles.Create(baubles.Record{Name: "Painted Wooden Horse", NameSimple: "horse", Tier: baubles.TierAverage,
		Value: 12, WeightLbs: 0.5, Description: "A child's toy horse, its red paint flaking.", Status: baubles.StatusReady,
		PlayerKey: true, Moderated: false, FoundByUserId: user.UserId})
	require.NoError(t, err)
	horse := items.New(items.BaubleItemId)
	horse.Bauble = rec.Id
	si.AffixedStock = []shops.AffixedStockEntry{{Item: horse, Price: 12, AddedAt: shelfListNow}}

	mine := buildShelfRows(si, user.UserId, shelfListNow)
	theirs := buildShelfRows(si, user.UserId+1000, shelfListNow)
	require.Len(t, mine, 1)
	require.Len(t, theirs, 1)
	assert.Contains(t, plainRow(mine[0]), "Painted Wooden Horse", "the finder reads their own name")
	assert.Contains(t, plainRow(theirs[0]), "Trinket")
	assert.NotContains(t, plainRow(theirs[0]), "Horse", "nobody else reads the hidden words")
	assert.Equal(t, "12", theirs[0][2], "the price is the same in both views")
}
