package actions

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/merchantchests"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Stolen goods from merchant chests follow the stolen-bauble rules
// (sell_stolen.go): hot for three days in the area they were taken in,
// sellable elsewhere or once cooled, a fence's cut anywhere, recognised on
// the thief by their merchant anywhere and by a guard in that area.
// Built on the sale harness: room 1 in TestZone, merchant template 2
// (instance 301), player seller user 1.

// stolenSword is a sword taken from merchant template fromMob by userId in
// zone at the given time.
func stolenSword(fromMob int, userId int, zone string, at time.Time) items.Item {
	it := items.New(sellTestItemId)
	it.StolenFrom = "Smith Brindle"
	it.StolenFromMob = fromMob
	it.MarkTaken(userId, zone, at)
	return it
}

// hotSword is stolen an hour ago here, from someone else's chest.
func hotSword() items.Item {
	return stolenSword(99, 1, "TestZone", stolenTestNow.Add(-time.Hour))
}

func TestSell_Stolen_HotHereHonestMerchantRefuses(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(hotSword()))

	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})

	assert.Equal(t, SellStopRejected, res.Reason, "res=%+v", res)
	assert.Equal(t, 0, char.Gold)
	_, still := char.FindInBackpack("iron sword")
	assert.True(t, still, "a refused item stays with the seller")
	assert.Equal(t, 1000, merchantInstance().Character.Gold)
}

func TestSell_Stolen_ElsewhereOrCooledSellsHonestly(t *testing.T) {
	cases := map[string]func() items.Item{
		"stolen in another town": func() items.Item { return stolenSword(99, 1, "Faraway", stolenTestNow.Add(-time.Hour)) },
		"three days cold":        func() items.Item { return stolenSword(99, 1, "TestZone", stolenTestNow.Add(-73*time.Hour)) },
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			defer seedSellItemSpecs()()
			defer seedSellRoom(t)()
			defer seedSellMerchant(t, 1000)()
			pinStolenClock(t, stolenTestNow)

			seller := newSellerActor(t, true)
			char := seller.GetCharacter()
			require.True(t, char.StoreItem(mk()))

			res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})
			assert.Equal(t, 1, res.Sold, "res=%+v", res)
			assert.Positive(t, char.Gold, "an honest sale at the honest price")
		})
	}
}

func TestSell_Stolen_FenceBuysHotOrColdAtItsCut(t *testing.T) {
	for name, mk := range map[string]func() items.Item{
		"hot":  hotSword,
		"cold": func() items.Item { return stolenSword(99, 1, "TestZone", stolenTestNow.Add(-100*time.Hour)) },
	} {
		t.Run(name, func(t *testing.T) {
			defer seedSellItemSpecs()()
			defer seedSellRoom(t)()
			defer seedSellMerchant(t, 1000)()
			pinStolenClock(t, stolenTestNow)
			merchantInstance().Groups = []string{"fence"}
			defer func() { merchantInstance().Groups = nil }()

			seller := newSellerActor(t, true)
			char := seller.GetCharacter()
			require.True(t, char.StoreItem(mk()))
			qtyBefore := shopQty(merchantInstance(), sellTestItemId)

			res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})

			require.Equal(t, SellStopSoldAll, res.Reason, "res=%+v", res)
			assert.Equal(t, FencePrice(100), char.Gold, "a fence pays its cut of the value")
			assert.Equal(t, 1000-FencePrice(100), merchantInstance().Character.Gold)
			assert.Equal(t, qtyBefore, shopQty(merchantInstance(), sellTestItemId), "the fence network moves it on; it is never shelved")
		})
	}
}

func TestSell_Stolen_CleanCopySellsFirstThenHotOneIsRefused(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(hotSword()))
	require.True(t, char.StoreItem(items.New(sellTestItemId)))

	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: UnlimitedSell})

	assert.Equal(t, 1, res.Sold, "only the clean sword sells (res=%+v)", res)
	assert.Equal(t, SellStopRejected, res.Reason, "then the hot one is refused")
	left, ok := char.FindInBackpack("iron sword")
	require.True(t, ok)
	assert.True(t, left.IsStolen(), "the stolen sword is the one left")
}

func TestSell_Stolen_OrdinalPicksExactlyThatCopy(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(hotSword()))
	require.True(t, char.StoreItem(items.New(sellTestItemId)))

	res := Sell(seller, SellOptions{ItemName: "1.iron sword", Quantity: 1})
	assert.Equal(t, SellStopRejected, res.Reason, "`1.iron sword` is the first sword, the stolen one (res=%+v)", res)

	res = Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})
	assert.Equal(t, 1, res.Sold, "by plain name the clean one sells")
}

func TestSellSweep_SellsTheStolenCopyToTheFence(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)
	merchantInstance().Groups = []string{"fence"}
	defer func() { merchantInstance().Groups = nil }()

	seller := newSellerActor(t, false)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(hotSword()))

	res := Sell(seller, SellOptions{SellAllSellable: true})
	assert.Equal(t, 1, res.Sold, "res=%+v", res)
	assert.Equal(t, FencePrice(100), char.Gold)
}

func TestStolenGoodsRefusal_OwnerKnowsItsOwn(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()

	own := StolenGoodsRefusal(merchantInstance(), stolenSword(2, 1, "TestZone", stolenTestNow))
	other := stolenSword(99, 1, "TestZone", stolenTestNow)
	other.StolenFrom = "A Market Hawker"
	assert.Contains(t, own, "That's mine!")
	assert.Contains(t, StolenGoodsRefusal(merchantInstance(), other), "the market hawker", "a leading article reads mid-sentence")
}

func TestMarkTaken_StartsTheHeatOnce(t *testing.T) {
	it := items.New(sellTestItemId)
	it.MarkTaken(1, "TestZone", stolenTestNow)
	assert.False(t, it.IsStolen(), "not a merchant's goods: nothing to mark")

	it.StolenFrom, it.StolenFromMob = "Smith Brindle", 2
	assert.True(t, it.IsMerchantGoods())
	assert.False(t, it.IsStolen(), "in the chest it is not yet stolen")
	it.MarkTaken(1, "TestZone", stolenTestNow)
	assert.True(t, it.IsStolen())
	it.MarkTaken(7, "Elsewhere", stolenTestNow.Add(time.Hour))
	assert.Equal(t, 1, it.StolenBy, "the first theft stands")
	assert.Equal(t, "TestZone", it.StolenZone)
}

// ─── Recognition and returns ────────────────────────────────────────────────

// addGuard stands a town guard (template 5, instance 305) in room 1.
func addGuard(t *testing.T) *mobs.Mob {
	t.Helper()
	g := &mobs.Mob{MobId: 5, InstanceId: 305, HomeRoomId: 1, Zone: "TestZone", Groups: []string{"guard"},
		Character: characters.Character{Name: "Town Guard", RoomId: 1, Conditions: conditions.New()}}
	g.Character.HealthMax.Value = 100
	g.Character.Health = 100
	mobs.SetInstanceForTest(305, g)
	room := rooms.LoadRoom(1)
	room.AddMob(305)
	t.Cleanup(func() { room.RemoveMob(305); mobs.SetInstanceForTest(305, nil) })
	return g
}

func TestStolenGoods_OwnerRecognisesThemOnceATheft(t *testing.T) {
	h := setupRecognition(t, true)
	sword := stolenSword(2, 1, "Faraway", stolenTestNow.Add(-time.Hour)) // taken elsewhere: owners know it anywhere
	require.True(t, h.thief.GetCharacter().StoreItem(sword))

	recognizeIn(h.room, 1, 0)
	require.Len(t, h.caught, 1)
	assert.Equal(t, merchantInstance(), h.caught[0], "caught by the merchant it was taken from")
	carried, _ := h.thief.GetCharacter().FindInBackpack("iron sword")
	assert.True(t, carried.StolenSeen)

	recognizeIn(h.room, 1, 0)
	assert.Len(t, h.caught, 1, "once per theft")
}

// reportedTo swaps the report a guard or lookout makes for a recorder.
func reportedTo(t *testing.T) *[]*mobs.Mob {
	t.Helper()
	var got []*mobs.Mob
	orig := stolenReported
	stolenReported = func(_ Actor, m *mobs.Mob, _ *rooms.Room) { got = append(got, m) }
	t.Cleanup(func() { stolenReported = orig })
	return &got
}

func TestStolenGoods_GuardReportsThemOnlyWhereTheyAreHot(t *testing.T) {
	for name, tc := range map[string]struct {
		zone     string
		reported bool
	}{
		"taken in this town": {"TestZone", true},
		"taken in another":   {"Faraway", false},
	} {
		t.Run(name, func(t *testing.T) {
			h := setupRecognition(t, true)
			reported := reportedTo(t)
			guard := addGuard(t)
			require.True(t, h.thief.GetCharacter().StoreItem(stolenSword(99, 1, tc.zone, stolenTestNow.Add(-time.Hour))))

			recognizeIn(h.room, 1, 0)
			assert.Empty(t, h.caught, "a guard reports the theft; it does not start a fight (thiefCaught)")
			if tc.reported {
				require.Len(t, *reported, 1)
				assert.Equal(t, guard, (*reported)[0])
			} else {
				assert.Empty(t, *reported, "word of the theft has not reached this town's guards")
			}
		})
	}
}

func TestStolenGoods_ACatalogLookoutReportsThemLikeAGuard(t *testing.T) {
	h := setupRecognition(t, true)
	reported := reportedTo(t)
	defer merchantchests.SetForTest(merchantchests.Catalog{
		Settings: merchantchests.Settings{SleepingPerceptionMult: 0.5, LookoutMobs: []int{5}},
		Generic:  merchantchests.Chest{Name: "chest", Description: "A chest."},
		Chests:   []merchantchests.Chest{{MobId: 2, RoomId: 1}},
	})()
	constable := addGuard(t)
	constable.Groups = []string{"np_dockfolk"} // not a `guard`: known only through the catalog
	require.True(t, h.thief.GetCharacter().StoreItem(stolenSword(99, 1, "TestZone", stolenTestNow.Add(-time.Hour))))

	recognizeIn(h.room, 1, 0)
	require.Len(t, *reported, 1)
	assert.Equal(t, constable, (*reported)[0])
}

func TestStolenGoods_WornGoodsAreRecognisedToo(t *testing.T) {
	h := setupRecognition(t, true)
	char := h.thief.GetCharacter()
	char.Equipment.Weapon = stolenSword(2, 1, "TestZone", stolenTestNow.Add(-time.Hour))

	recognizeIn(h.room, 1, 0)
	require.Len(t, h.caught, 1, "wielding it does not hide it from the merchant it was taken from")
	assert.True(t, char.Equipment.Weapon.StolenSeen, "marked on the wielded copy")
}

func TestSell_Stolen_AFenceWillNotBuyBackItsOwnGoods(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 1000)()
	pinStolenClock(t, stolenTestNow)
	merchantInstance().Groups = []string{"fence"}
	defer func() { merchantInstance().Groups = nil }()

	seller := newSellerActor(t, true)
	char := seller.GetCharacter()
	require.True(t, char.StoreItem(stolenSword(2, 1, "Faraway", stolenTestNow.Add(-100*time.Hour)))) // its own, even cold and far away

	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})
	assert.Equal(t, SellStopRejected, res.Reason, "res=%+v", res)
	assert.Equal(t, 0, char.Gold)
}

func TestStolenGoods_NoRecognitionWithoutCause(t *testing.T) {
	cases := map[string]func() items.Item{
		"cold":                   func() items.Item { return stolenSword(2, 1, "TestZone", stolenTestNow.Add(-73*time.Hour)) },
		"someone else's goods":   func() items.Item { return stolenSword(99, 1, "Faraway", stolenTestNow.Add(-time.Hour)) },
		"carried by a bystander": func() items.Item { return stolenSword(2, 42, "TestZone", stolenTestNow.Add(-time.Hour)) },
		"still a merchant's good": func() items.Item {
			i := items.New(sellTestItemId)
			i.StolenFrom, i.StolenFromMob = "Merchant", 2
			return i
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			h := setupRecognition(t, true)
			require.True(t, h.thief.GetCharacter().StoreItem(mk()))
			recognizeIn(h.room, 0, 301)
			assert.Empty(t, h.caught)
		})
	}
}

func TestStolenGoods_GivenBackToTheirMerchantAreNoLongerStolen(t *testing.T) {
	h := setupRecognition(t, true)
	sword := stolenSword(2, 1, "TestZone", stolenTestNow.Add(-time.Hour))
	m := merchantInstance()
	require.True(t, m.Character.StoreItem(sword)) // give transfers first, then calls the handler

	assert.True(t, StolenBaubleGiven(h.thief, m, sword))
	held, ok := m.Character.FindInBackpack("iron sword")
	require.True(t, ok)
	assert.False(t, held.IsStolen())
	assert.False(t, held.IsMerchantGoods())

	other := stolenSword(99, 1, "TestZone", stolenTestNow.Add(-time.Hour))
	assert.False(t, StolenBaubleGiven(h.thief, m, other), "given to someone it was not taken from, it is just a gift")
}

// ─── Sleeping merchants ─────────────────────────────────────────────────────

func TestSleepingMerchantPerceptionMult(t *testing.T) {
	defer seedSleepCondition(t)()
	defer merchantchests.SetForTest(merchantchests.Catalog{
		Settings: merchantchests.Settings{SleepingPerceptionMult: 0.5},
		Generic:  merchantchests.Chest{Name: "chest", Description: "A chest."},
		Chests:   []merchantchests.Chest{{MobId: 2, RoomId: 1}},
	})()

	merchant := &characters.Character{
		Name:       "Merchant",
		IsMob:      true,
		Conditions: conditions.New(),
		Shop:       characters.Shop{{ItemId: sellTestItemId}},
	}
	townsfolk := &characters.Character{Name: "Sleeper", Conditions: conditions.New()}

	assert.Equal(t, 1.0, sleepingMerchantPerceptionMult(merchant), "an awake merchant sees in full")

	merchant.AddCondition(15, false)
	townsfolk.AddCondition(15, false)
	require.True(t, merchant.HasConditionFlag(conditions.Sleeping))

	assert.Equal(t, 0.5, sleepingMerchantPerceptionMult(merchant), "a sleeping merchant counts half")
	assert.Equal(t, 1.0, sleepingMerchantPerceptionMult(townsfolk), "only merchants: no shop list, no change")
	assert.Equal(t, 1.0, sleepingMerchantPerceptionMult(nil))

	playerShop := &characters.Character{Name: "Shopkeeping Player", Conditions: conditions.New(),
		Shop: characters.Shop{{ItemId: sellTestItemId}}}
	playerShop.AddCondition(15, false)
	assert.Equal(t, 1.0, sleepingMerchantPerceptionMult(playerShop), "a player's shop is not a merchant NPC")
}
