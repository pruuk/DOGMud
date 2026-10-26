package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #375 phase 3a: salvage, buy and steal acquire through the seam (internal/
// acquire). Each landed item raises one ItemOwnership{Gained} with its
// reason; an item the actor cannot carry raises no gain. Before the seam a
// purchase or a theft queued its gain whether or not the store landed.

const (
	routeAcqLight = 975601
	routeAcqHeavy = 975602
	routeAcqTag   = "test-acquire-scrap"
)

func seedRouteAcquire(t *testing.T) {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		routeAcqLight: {ItemId: routeAcqLight, Name: "scrap", Type: items.Object, Weight: 0.1, ComponentTag: routeAcqTag},
		routeAcqHeavy: {ItemId: routeAcqHeavy, Name: "millstone", Type: items.Object, Weight: 2.0},
	}))
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
}

func routeGains(userId int) []events.ItemOwnership {
	var gains []events.ItemOwnership
	for _, e := range events.DrainQueuedItemOwnershipForTest(userId) {
		if e.Gained {
			gains = append(gains, e)
		}
	}
	return gains
}

func TestSalvage_RecoveredMaterialsAcquireWithSalvageReason(t *testing.T) {
	seedRouteAcquire(t)
	actor := newStealPlayerActor(100, 0)

	storeRecovered(actor, []crafting.RecipeIngredient{{ItemTag: routeAcqTag, Quantity: 2}}, &SalvageResult{})

	gains := routeGains(actor.userId)
	require.Len(t, gains, 2, "one gain per recovered material")
	for _, g := range gains {
		assert.Equal(t, "salvage", g.Reason)
	}
}

func TestBuy_PurchasedItemAcquiresWithBuyReason(t *testing.T) {
	seedRouteAcquire(t)
	buyer := newStealPlayerActor(100, 0)

	executePurchaseItem(buyer, nil, nil, characters.ShopItem{ItemId: routeAcqLight}, 0, "nothing")

	gains := routeGains(buyer.userId)
	require.Len(t, gains, 1)
	assert.Equal(t, "buy", gains[0].Reason)
}

func TestBuy_RefusedStoreRaisesNoGain(t *testing.T) {
	seedRouteAcquire(t)
	buyer := newStealPlayerActor(100, 0)
	buyer.char.Stats.Strength.ValueAdj = 1

	executePurchaseItem(buyer, nil, nil, characters.ShopItem{ItemId: routeAcqHeavy}, 0, "nothing")

	assert.Empty(t, routeGains(0), "a purchase that did not land raises no gain")
}

func TestSteal_TakenItemAcquiresWithStealReason(t *testing.T) {
	seedRouteAcquire(t)
	m := newStealTestMob(testMobInstId, 0, 1)
	m.Character.Items = []items.Item{items.New(routeAcqLight)}
	thief := newStealPlayerActor(150, 5)

	res := takeFromMob(thief, m, nil)
	require.Equal(t, routeAcqLight, res.StoleItemId)

	gains := routeGains(thief.userId)
	require.Len(t, gains, 1)
	assert.Equal(t, "steal", gains[0].Reason)
}

func TestSteal_RefusedStoreRaisesNoGain(t *testing.T) {
	seedRouteAcquire(t)
	m := newStealTestMob(testMobInstId, 0, 1)
	m.Character.Items = []items.Item{items.New(routeAcqHeavy)}
	thief := newStealPlayerActor(150, 5)
	thief.char.Stats.Strength.ValueAdj = 1

	takeFromMob(thief, m, nil)

	assert.Empty(t, routeGains(0), "a theft that did not land raises no gain")
}
