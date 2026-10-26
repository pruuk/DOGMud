package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #375 phase 3a: a quest trigger's give_item acquires through the seam, so
// the item lands with exactly one ItemOwnership{Gained, Reason: quest_give}
// and a refused store raises nothing.

const (
	bridgeAcqLightItem = 975101
	bridgeAcqHeavyItem = 975102
)

func seedBridgeAcquireItems(t *testing.T) {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		bridgeAcqLightItem: {ItemId: bridgeAcqLightItem, Name: "brass key", Type: items.Object, Weight: 0.1},
		bridgeAcqHeavyItem: {ItemId: bridgeAcqHeavyItem, Name: "anvil", Type: items.Object, Weight: 2.0},
	}))
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
}

func TestGameBridgeGiveItem_AcquiresWithQuestGiveReason(t *testing.T) {
	seedBridgeAcquireItems(t)
	u := users.NewTestUser(9751, "bridgeacq", "Bridgeacq", 97510)

	require.NoError(t, NewGameBridge(u, 1).GiveItem(bridgeAcqLightItem))

	got := events.DrainQueuedItemOwnershipForTest(u.UserId)
	require.Len(t, got, 1, "exactly one ownership event")
	assert.True(t, got[0].Gained)
	assert.Equal(t, "quest_give", got[0].Reason)
	assert.Equal(t, bridgeAcqLightItem, got[0].Item.ItemId)
}

func TestGameBridgeGiveItem_RefusedStoreRaisesNothing(t *testing.T) {
	seedBridgeAcquireItems(t)
	u := users.NewTestUser(9752, "bridgeacq2", "Bridgeacq2", 97520)
	u.Character.Stats.Strength.ValueAdj = 1

	require.Error(t, NewGameBridge(u, 1).GiveItem(bridgeAcqHeavyItem))
	assert.Empty(t, events.DrainQueuedItemOwnershipForTest(0))
}
