package actions

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #940 (owner ruling 2026-11-22): quest gear fades whenever it leaves its
// holder, shops never buy it and a trade-in never takes it. Each case below
// pairs the quest-given copy with a bought copy of the same item, which
// behaves exactly as before.

func questGiven(id int) items.Item {
	itm := items.New(id)
	itm.QuestGiven = true
	return itm
}

func countId(list []items.Item, id int) int {
	n := 0
	for _, itm := range list {
		if itm.ItemId == id {
			n++
		}
	}
	return n
}

// ─── Sell ───────────────────────────────────────────────────────────────────

func sellerWith(t *testing.T, copies ...items.Item) Actor {
	t.Helper()
	seller := newSellerActor(t, true)
	for _, itm := range copies {
		require.True(t, seller.GetCharacter().StoreItem(itm))
	}
	events.DrainQueuedMessagesForTest(seller.GetUserId())
	return seller
}

func TestQuestGear_SellByNameIsRefused(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 100000)()

	seller := sellerWith(t, questGiven(sellTestItemId))
	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})

	assert.Equal(t, 0, res.Sold)
	assert.Equal(t, SellStopRejected, res.Reason)
	assert.Equal(t, 0, seller.GetCharacter().Gold)
	assert.Equal(t, 1, countId(seller.GetCharacter().Items, sellTestItemId), "the seller keeps it")
	assert.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(seller.GetUserId()), "\n"), sellTellQuestGiven)
}

func TestQuestGear_SellByNameBoughtCopySells(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 100000)()

	seller := sellerWith(t, items.New(sellTestItemId))
	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})

	assert.Equal(t, 1, res.Sold)
	assert.Greater(t, seller.GetCharacter().Gold, 0)
	assert.Equal(t, 0, countId(seller.GetCharacter().Items, sellTestItemId))
}

// With both copies carried, `sell iron sword` sells the bought one whichever
// comes first, and `sell all iron sword` stops at the quest copy as a
// completed sale.
func TestQuestGear_SellPrefersTheBoughtCopy(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 100000)()

	seller := sellerWith(t, questGiven(sellTestItemId), items.New(sellTestItemId))
	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})
	assert.Equal(t, 1, res.Sold)
	left := seller.GetCharacter().Items
	require.Equal(t, 1, countId(left, sellTestItemId))
	assert.True(t, left[0].IsQuestGiven(), "the quest copy stays")

	seller = sellerWith(t, questGiven(sellTestItemId), items.New(sellTestItemId), items.New(sellTestItemId))
	res = Sell(seller, SellOptions{ItemName: "iron sword", Quantity: UnlimitedSell})
	assert.Equal(t, 2, res.Sold)
	assert.Equal(t, SellStopSoldAll, res.Reason)
	assert.NotContains(t, strings.Join(events.DrainQueuedMessagesForTest(seller.GetUserId()), "\n"), sellTellQuestGiven,
		"selling the rest is a completed sale, not a refusal")
	require.Equal(t, 1, countId(seller.GetCharacter().Items, sellTestItemId))
	assert.True(t, seller.GetCharacter().Items[0].IsQuestGiven())
}

// The inventory sweep (mob `sell all`) skips quest gear and sells the rest.
func TestQuestGear_SellSweepSkipsIt(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 100000)()

	seller := sellerWith(t, questGiven(sellTestItemId), items.New(sellTestItemId))
	res := Sell(seller, SellOptions{SellAllSellable: true})
	assert.Equal(t, 1, res.Sold)
	require.Equal(t, 1, countId(seller.GetCharacter().Items, sellTestItemId))
	assert.True(t, seller.GetCharacter().Items[0].IsQuestGiven())
}

// Offer and appraise read the same quote.
func TestQuestGear_QuoteSaleTells(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 100000)()

	q := QuoteSale(nil, rooms.LoadRoom(1), merchantInstance(), nil, questGiven(sellTestItemId), true)
	assert.Equal(t, 0, q.Price)
	assert.Equal(t, sellTellQuestGiven, q.Tell)

	q = QuoteSale(nil, rooms.LoadRoom(1), merchantInstance(), nil, items.New(sellTestItemId), true)
	assert.Greater(t, q.Price, 0, "a bought copy is quoted as before")
}

// ─── Trade-in ───────────────────────────────────────────────────────────────

func TestQuestGear_TradeInNeverTakesIt(t *testing.T) {
	defer seedSellItemSpecs()()

	const wantId = sellTestComponentId // bought by trading in an iron sword
	shopItem := characters.ShopItem{ItemId: wantId, TradeItemId: sellTestItemId, Quantity: 5, QuantityMax: 5}
	buyerWith := func(copies ...items.Item) (*MobActor, *mobs.Mob) {
		m := &mobs.Mob{}
		m.Character.Name = "Buyer"
		m.Character.Gold = 1000
		m.Character.Conditions = conditions.New()
		m.Character.Stats.Strength.ValueAdj = 100
		for _, itm := range copies {
			require.True(t, m.Character.StoreItem(itm))
		}
		shop := &mobs.Mob{Character: characters.Character{Name: "Merchant", Shop: characters.Shop{shopItem}}}
		return &MobActor{Mob: m}, shop
	}

	t.Run("only quest gear: nothing to trade", func(t *testing.T) {
		buyer, shop := buyerWith(questGiven(sellTestItemId))
		_, reason, ok := validatePurchase(buyer, shop, nil, shopItem, map[int]int{wantId: 0}, nil)
		assert.False(t, ok)
		assert.Equal(t, BuyReasonMissingTradeItem, reason)
		assert.Equal(t, 1, countId(buyer.Mob.Character.Items, sellTestItemId))
	})
	t.Run("a bought copy pays, the quest copy stays", func(t *testing.T) {
		buyer, shop := buyerWith(questGiven(sellTestItemId), items.New(sellTestItemId))
		_, _, ok := validatePurchase(buyer, shop, nil, shopItem, map[int]int{wantId: 0}, nil)
		require.True(t, ok)
		left := buyer.Mob.Character.Items
		require.Equal(t, 1, countId(left, sellTestItemId))
		assert.True(t, left[0].IsQuestGiven())
	})
}

// ─── Drop and give (the actions seams) ──────────────────────────────────────

// A companion holding quest gear lets it go too: a mob's drop fades it.
func TestQuestGear_MobDropFades(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	room := rooms.LoadRoom(1)

	for _, tc := range []struct {
		name  string
		itm   items.Item
		fades bool
	}{
		{"quest gear fades", questGiven(sellTestItemId), true},
		{"bought copy drops", items.New(sellTestItemId), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			room.Items = nil
			m := &mobs.Mob{InstanceId: 9401, Character: characters.Character{Name: "Hound", RoomId: 1, Conditions: conditions.New()}}
			m.Character.Stats.Strength.ValueAdj = 100
			require.True(t, m.Character.StoreItem(tc.itm))
			res := DropItem(&MobActor{Mob: m, Room: room}, "iron sword")
			require.True(t, res.Found)
			assert.Equal(t, tc.fades, errors.Is(res.Err, ErrQuestGivenVanished))
			assert.Empty(t, m.Character.Items)
			want := 1
			if tc.fades {
				want = 0
			}
			assert.Equal(t, want, countId(room.Items, sellTestItemId))
		})
	}
}

// A charmed mob hands quest gear back to its master and the copy stays quest
// gear; given to anyone else it fades.
func TestQuestGear_CompanionGivesItBackToItsMaster(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	room := rooms.LoadRoom(1)

	master := characters.New()
	master.Stats.Strength.ValueAdj = 100
	other := characters.New()
	other.Stats.Strength.ValueAdj = 100

	companion := func() *MobActor {
		m := &mobs.Mob{InstanceId: 9402, Character: characters.Character{Name: "Hound", RoomId: 1, Conditions: conditions.New()}}
		m.Character.Stats.Strength.ValueAdj = 100
		require.True(t, m.Character.StoreItem(questGiven(sellTestItemId)))
		m.Character.Charm(9411, 100, ``)
		return &MobActor{Mob: m, Room: room}
	}

	res := GiveItemToChar(companion(), "iron sword", master, 9411, 0)
	require.NoError(t, res.Err)
	require.Equal(t, 1, countId(master.Items, sellTestItemId))
	assert.True(t, master.Items[0].IsQuestGiven())

	res = GiveItemToChar(companion(), "iron sword", other, 9412, 0)
	assert.ErrorIs(t, res.Err, ErrQuestGivenVanished)
	assert.Equal(t, 0, countId(other.Items, sellTestItemId))
}

// ─── Plant ──────────────────────────────────────────────────────────────────

// Quest gear fades before any contest: no attempt, no award, nothing planted.
// A bought copy goes on to the contest as before.
func TestQuestGear_Plant(t *testing.T) {
	target := newPlantTestMob(testPlantMobInstId, 1)
	mobs.SetInstanceForTest(testPlantMobInstId, target)
	defer mobs.SetInstanceForTest(testPlantMobInstId, nil)

	t.Run("quest gear fades", func(t *testing.T) {
		actor := newPlantPlayerActor(200, 8)
		itm := plantTestItem()
		itm.QuestGiven = true
		actor.char.StoreItem(itm)
		target.Character.Items = nil

		res := Plant(actor, PlantOptions{TargetMobInstanceId: testPlantMobInstId, ItemNoun: "!1"})
		assert.False(t, res.Succeeded)
		assert.Equal(t, "quest gear faded", res.Reason)
		assert.Empty(t, actor.char.Items)
		assert.Empty(t, target.Character.Items)
		assert.Empty(t, actor.awards, "no attempt was made")
		assert.True(t, actor.char.CooldownReady(skills.Skullduggery.String("steal")), "no cooldown spent")
	})
	t.Run("bought copy is attempted", func(t *testing.T) {
		actor := newPlantPlayerActor(200, 8)
		seedPlantItem(actor)
		target.Character.Items = nil

		res := Plant(actor, PlantOptions{TargetMobInstanceId: testPlantMobInstId, ItemNoun: "!1"})
		assert.NotEqual(t, "quest gear faded", res.Reason)
		assert.NotEmpty(t, actor.awards, "the contest ran")
	})
}

// ─── The helper ─────────────────────────────────────────────────────────────

func TestVanishQuestGiven_LeavesOrdinaryItemsAlone(t *testing.T) {
	c := characters.New()
	plain := items.Item{ItemId: 1}
	c.Items = []items.Item{plain}
	assert.False(t, VanishQuestGiven(c, 0, 0, plain, nil))
	assert.Len(t, c.Items, 1)

	gear := items.Item{ItemId: 2, QuestGiven: true}
	c.Items = append(c.Items, gear)
	assert.True(t, VanishQuestGiven(c, 0, 0, gear, nil))
	assert.Len(t, c.Items, 1)
	assert.Equal(t, 1, c.Items[0].ItemId)
}

func TestQuestGearFadesLineFitsOneScreen(t *testing.T) {
	line := fmt.Sprintf(QuestGearFadesLine, "Drillyard Cudgel")
	plain := strings.NewReplacer(`<ansi fg="itemname">`, ``, `</ansi>`, ``).Replace(line)
	assert.Equal(t, "The Drillyard Cudgel fades away. Quest gear lasts only while you carry it.", plain)
	assert.LessOrEqual(t, len(plain), 80)
}
