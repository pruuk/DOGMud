package usercommands

import (
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// 2026-10-17 pre-deploy playtest: `offer token` at Trader Onna quoted
// "I can give you 1 gold for that Smooth Grey Token." and `sell token` then
// said "I'm not interested in that." Offer priced through the legacy
// mob.GetSellPrice, sell through the living-economy shop
// (shops.EvaluateBuyRules), which refuses an item with no vendor_categories.
// Both now say actions.QuoteSale.

const (
	agreeUntaggedId = 84001 // listSightRoom's tin cup: stocked, no vendor_categories
	agreeTaggedId   = 84002 // a tagged item the general shop buys
)

// agreeShop is listSightRoom (lit "city") with a living-economy general shop
// registered for its merchant Keeper (mob 841, home room 8410), the shape
// every shipped merchant has (mobs.RegisterMobShop).
func agreeShop(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	user, room := listSightRoom(t, "city")
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		agreeUntaggedId: {ItemId: agreeUntaggedId, Name: "tin cup", Type: items.Object, Value: 50},
		agreeTaggedId: {ItemId: agreeTaggedId, Name: "brass bell", Type: items.Object, Value: 40,
			VendorCategories: []string{"trinket"}},
	}))
	shops.ClearCache()
	t.Cleanup(shops.ClearCache)
	shops.RegisterShop("TestZone", 841, 8410, shops.ShopInventory{Gold: 100000, CraftSupport: shops.CraftSupportGeneral})
	room.AddPlayer(user.UserId) // so the merchant's spoken lines reach the seller
	t.Cleanup(func() { room.RemovePlayer(user.UserId) })
	events.DrainQueuedMessagesForTest(8411)
	return user, room
}

func agreeLines(t *testing.T) string {
	t.Helper()
	return strings.Join(listSightPlain(events.DrainQueuedMessagesForTest(8411)), "\n")
}

// An item the shop will not buy: offer names no price, and says the same
// reason sell gives.
func TestOffer_NeverQuotesWhatSellRefuses(t *testing.T) {
	user, room := agreeShop(t)
	user.Character.StoreItem(items.New(agreeUntaggedId))

	_, err := Offer("tin cup", user, room, 0)
	require.NoError(t, err)
	offerOut := agreeLines(t)
	require.NotContains(t, offerOut, "gold for that", "offer quoted a price sell refuses")

	_, err = Sell("tin cup", user, room, 0)
	require.NoError(t, err)
	sellOut := agreeLines(t)
	require.Contains(t, sellOut, "No shop will buy it", "sell says why it refuses")
	require.Equal(t, sellOut, offerOut, "offer and sell give the same answer")
	require.Equal(t, 0, user.Character.Gold, "nothing was sold")
}

// An item the shop buys: offer's price is the gold sell pays.
func TestOffer_QuotesWhatSellPays(t *testing.T) {
	user, room := agreeShop(t)
	user.Character.StoreItem(items.New(agreeTaggedId))
	user.Character.Gold = 0

	_, err := Offer("brass bell", user, room, 0)
	require.NoError(t, err)
	offerOut := agreeLines(t)
	require.Contains(t, offerOut, "gold for that")

	_, err = Sell("brass bell", user, room, 0)
	require.NoError(t, err)
	_ = agreeLines(t)
	require.Greater(t, user.Character.Gold, 0, "the bell sold")
	require.Contains(t, offerOut, "give you "+strconv.Itoa(user.Character.Gold)+" gold for that",
		"offer quoted the gold sell paid")
}
