package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A completed player sale notifies the quest engine with the command "sell",
// the item sold and the shop mob that bought it (R22, #375: selling Smith Rusk
// the crafted dagger turns in First Heat). These tests register a probe quest
// whose single trigger is a sell trigger and look for its grant on the seller.

const (
	sellProbeToken  = "99-start" // a fresh character may only start a quest
	sellProbeQuest  = 99
	sellProbeMobId  = 2 // seedSellMerchant's merchant MobId
	sellProbeRoomId = 1
)

// sellProbeSetup seeds the world, the merchant (with the given gold), a player
// holding sellTestItemId, and an engine whose only trigger is the given one.
// It returns the seller's user record and a cleanup.
func sellProbeSetup(t *testing.T, merchantGold int, trig quests.TriggerDef) (*users.UserRecord, Actor, func()) {
	t.Helper()
	undo := []func(){seedSellItemSpecs(), seedSellRoom(t), seedSellMerchant(t, merchantGold)}
	seller := newSellerActor(t, true, sellTestItemId)
	u := seller.(*UserActor).User
	undo = append(undo, users.SeedUsersForTest(map[int]*users.UserRecord{u.UserId: u}))

	undo = append(undo, questengine.ResetEngineForTest())
	trig.Event = "command"
	trig.Command = "sell"
	trig.Actions = []quests.ActionDef{{Grant: sellProbeToken}}
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId:  sellProbeQuest,
		Name:     "Sell Probe",
		Triggers: []quests.TriggerDef{trig},
	})
	return u, seller, func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
	}
}

func TestSell_NotifiesQuestEngineWithItemAndShopMob(t *testing.T) {
	u, seller, done := sellProbeSetup(t, 1000, quests.TriggerDef{
		Item: sellTestItemId, Mob: sellProbeMobId, Room: sellProbeRoomId,
	})
	defer done()

	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})

	require.Equal(t, 1, res.Sold, "the sale itself must complete")
	assert.True(t, u.Character.HasQuest(sellProbeToken),
		"a completed sale must notify the quest engine with the item and the shop mob")
}

func TestSell_OtherItemDoesNotFireItemTrigger(t *testing.T) {
	u, seller, done := sellProbeSetup(t, 1000, quests.TriggerDef{
		Item: sellTestItemId + 500, Mob: sellProbeMobId,
	})
	defer done()

	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})

	require.Equal(t, 1, res.Sold)
	assert.False(t, u.Character.HasQuest(sellProbeToken),
		"selling a different item must not fire a trigger keyed to another item")
}

func TestSell_OtherShopDoesNotFireMobTrigger(t *testing.T) {
	u, seller, done := sellProbeSetup(t, 1000, quests.TriggerDef{
		Item: sellTestItemId, Mob: sellProbeMobId + 500,
	})
	defer done()

	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})

	require.Equal(t, 1, res.Sold)
	assert.False(t, u.Character.HasQuest(sellProbeToken),
		"selling to a different shop mob must not fire a trigger keyed to Rusk's mob")
}

func TestSell_RefusedSaleDoesNotNotify(t *testing.T) {
	// A merchant with no gold refuses the sale (SellStopMerchantBroke).
	u, seller, done := sellProbeSetup(t, 0, quests.TriggerDef{
		Item: sellTestItemId, Mob: sellProbeMobId,
	})
	defer done()

	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 1})

	require.Equal(t, 0, res.Sold, "the broke merchant must refuse")
	assert.False(t, u.Character.HasQuest(sellProbeToken),
		"a refused sale must not notify the quest engine")
}
