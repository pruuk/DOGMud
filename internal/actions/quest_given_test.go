package actions

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
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
	assert.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(seller.GetUserId()), "\n"), SellTellQuestGiven)
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
	assert.NotContains(t, strings.Join(events.DrainQueuedMessagesForTest(seller.GetUserId()), "\n"), SellTellQuestGiven,
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
	assert.Equal(t, SellTellQuestGiven, q.Tell)

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

// Quest gear fades only when the plant succeeds, the moment it would have
// left the thief; nothing reaches the target. A plant that fails keeps it, as
// it keeps any item. A bought copy is planted as before.
func TestQuestGear_Plant(t *testing.T) {
	questItem := func() items.Item {
		itm := plantTestItem()
		itm.QuestGiven = true
		return itm
	}
	attempt := func(t *testing.T, perception, dex, rank int, itm items.Item) (PlantResult, *stubActorWithId, *mobs.Mob) {
		t.Helper()
		target := newPlantTestMob(testPlantMobInstId, perception)
		mobs.SetInstanceForTest(testPlantMobInstId, target)
		t.Cleanup(func() { mobs.SetInstanceForTest(testPlantMobInstId, nil) })
		actor := newPlantPlayerActor(dex, rank)
		actor.char.Items = append(actor.char.Items, itm) // carried, whatever it weighs
		return Plant(actor, PlantOptions{TargetMobInstanceId: testPlantMobInstId, ItemNoun: "!1"}), actor, target
	}

	t.Run("a successful plant fades quest gear", func(t *testing.T) {
		faded := 0
		for range 20 {
			res, actor, target := attempt(t, 1, 200, 8, questItem())
			assert.Empty(t, target.Character.Items, "quest gear never reaches the target")
			assert.False(t, res.Succeeded)
			if res.Reason == "quest gear faded" {
				faded++
				assert.Empty(t, actor.char.Items)
			}
		}
		assert.Greater(t, faded, 10, "a strong thief succeeds, and the gear fades")
	})
	t.Run("a failed plant keeps quest gear", func(t *testing.T) {
		caught := 0
		for range 20 {
			res, actor, _ := attempt(t, 100000, 1, 0, questItem())
			if res.Detected {
				caught++
				require.Len(t, actor.char.Items, 1, "the thief keeps it")
				assert.True(t, actor.char.Items[0].IsQuestGiven())
			}
		}
		assert.Greater(t, caught, 10, "a weak thief is caught")
	})
	t.Run("a target that cannot carry it leaves it with the thief", func(t *testing.T) {
		t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
			plantTestItemId: {ItemId: plantTestItemId, Name: "Anvil", Type: items.Object, Weight: 500},
		}))
		kept := 0
		for range 20 {
			res, actor, target := attempt(t, 1, 200, 8, questItem())
			assert.NotEqual(t, "quest gear faded", res.Reason, "a full target never takes it, so it never leaves")
			assert.Empty(t, target.Character.Items)
			if res.Reason == "target full" {
				kept++
				require.Len(t, actor.char.Items, 1, "the thief keeps it (#696)")
				assert.True(t, actor.char.Items[0].IsQuestGiven())
			}
		}
		assert.Greater(t, kept, 10)
	})
	t.Run("a bought copy is planted", func(t *testing.T) {
		planted := 0
		for range 20 {
			res, _, target := attempt(t, 1, 200, 8, plantTestItem())
			assert.NotEqual(t, "quest gear faded", res.Reason)
			if res.Succeeded {
				planted++
				assert.Len(t, target.Character.Items, 1)
			}
		}
		assert.Greater(t, planted, 10)
	})
}

// ─── Floors reached without RemoveItem ──────────────────────────────────────

// questHammerId is a worn weapon too heavy to go back in a weak pack.
const questHammerId = 94501

func seedHammer(t *testing.T) {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		questHammerId:     {ItemId: questHammerId, Name: "Anvil Hammer", NameSimple: "hammer", Type: items.Weapon, Hands: 1, Weight: 500},
		questHammerId + 1: {ItemId: questHammerId + 1, Name: "Twig", NameSimple: "twig", Type: items.Weapon, Hands: 2, Weight: 0.1}, // two hands: it displaces the hammer
	}))
}

// A worn item taken off by a character over twice its carry capacity falls to
// the floor; quest gear fades instead.
func TestQuestGear_RemoveWhileOverloadedFades(t *testing.T) {
	defer seedSellRoom(t)()
	seedHammer(t)
	room := rooms.LoadRoom(1)
	for _, quest := range []bool{true, false} {
		room.Items = nil
		char := characters.New()
		char.Stats.Strength.ValueAdj = 1
		char.Equipment.Weapon = items.Item{ItemId: questHammerId, UUID: items.NewItemUUID(), QuestGiven: quest}
		actor := &stubActor{char: char, room: room}

		res := RemoveEquipment(actor, "hammer")
		require.True(t, res.Removed, "quest=%v", quest)
		assert.Equal(t, 0, char.Equipment.Weapon.ItemId)
		assert.Empty(t, char.Items, "too heavy for the pack")
		want := 1
		if quest {
			want = 0
		}
		assert.Equal(t, want, countId(room.Items, questHammerId), "quest=%v", quest)
	}
}

// Gear displaced by an equip that will not fit the pack falls to the floor;
// quest gear fades instead.
func TestQuestGear_DisplacedByEquipFades(t *testing.T) {
	defer seedSellRoom(t)()
	seedHammer(t)
	room := rooms.LoadRoom(1)
	for _, quest := range []bool{true, false} {
		room.Items = nil
		char := characters.New()
		char.Stats.Strength.ValueAdj = 1
		char.Equipment.Weapon = items.Item{ItemId: questHammerId, UUID: items.NewItemUUID(), QuestGiven: quest}
		char.Items = []items.Item{items.New(questHammerId + 1)}
		actor := &stubActor{char: char, room: room}

		res := EquipItem(actor, "twig")
		require.True(t, res.Equipped, "quest=%v: %+v", quest, res)
		assert.Equal(t, questHammerId+1, char.Equipment.Weapon.ItemId)
		want := 1
		if quest {
			want = 0
		}
		assert.Equal(t, want, countId(room.Items, questHammerId), "quest=%v", quest)
		assert.Equal(t, 0, countId(char.Items, questHammerId))
	}
}

// TransferItemBetweenChars holds the leave rule itself (#940), so no caller
// can skip it: a player to another player fades; a tree handing back what it
// was given does not.
func TestQuestGear_TransferHoldsTheRule(t *testing.T) {
	defer seedSellItemSpecs()()
	from := characters.New()
	to := characters.New()
	gear := questGiven(sellTestItemId)
	from.Items = []items.Item{gear}

	err := TransferItemBetweenChars(gear, from, 9421, 0, to, 9422, 0, acquire.AcquireGive)
	assert.ErrorIs(t, err, ErrQuestGivenVanished)
	assert.Empty(t, from.Items)
	assert.Empty(t, to.Items)

	npc := characters.New()
	npc.Items = []items.Item{gear}
	player := characters.New()
	err = TransferItemBetweenChars(gear, npc, 0, 9423, player, 9421, 0, acquire.AcquireTreeGive)
	require.NoError(t, err)
	require.Len(t, player.Items, 1)
	assert.True(t, player.Items[0].IsQuestGiven())
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

// ─── Salvage ────────────────────────────────────────────────────────────────

// sayingActor records what its holder reads.
type sayingActor struct {
	*stubActorWithId
	said []string
}

func (a *sayingActor) SendText(_ messaging.Category, s string) { a.said = append(a.said, s) }

// Owner ruling 2026-11-22 (option A): salvaging a quest's copy destroys it and
// recovers nothing, with no roll, no skill award and no success. A bought copy
// salvages as before.
func TestQuestGear_SalvageYieldsNothing(t *testing.T) {
	seedRouteAcquire(t)
	const gearId = 94801
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		gearId: {ItemId: gearId, Name: "Drillyard Cudgel", Type: items.Weapon,
			SalvageReturns: []items.SalvageReturn{{ItemTag: routeAcqTag, Quantity: 1}}},
		routeAcqLight: {ItemId: routeAcqLight, Name: "scrap", Type: items.Object, Weight: 0.1, ComponentTag: routeAcqTag},
	}))
	for _, quest := range []bool{true, false} {
		actor := &sayingActor{stubActorWithId: newStealPlayerActor(100, 0)}
		itm := items.New(gearId)
		itm.QuestGiven = quest
		actor.char.Items = []items.Item{itm}
		events.DrainAllQueuedEventsForTest()

		res := salvageItem(actor, itm.UUID.String(), false, 1e9)

		assert.Equal(t, 0, countId(actor.char.Items, gearId), "quest=%v: the item is destroyed either way", quest)
		if quest {
			assert.False(t, res.Succeeded)
			assert.False(t, res.RollHappened, "no roll")
			assert.Empty(t, res.MaterialIds, "nothing recovered")
			assert.Empty(t, actor.awards, "no skill award")
			assert.Empty(t, routeGains(actor.userId), "nothing acquired")
			assert.Equal(t, []string{fmt.Sprintf(QuestGearSalvageLine, "Drillyard Cudgel")}, actor.said)
		} else {
			assert.True(t, res.Succeeded)
			assert.True(t, res.RollHappened)
			assert.Len(t, actor.awards, 1, "one award, as before")
		}
	}
}
