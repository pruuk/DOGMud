package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #375 phase 3a: the behaviour tree's item grants acquire through the seam.
// give_item and give_item_multiple raise one ItemOwnership{Gained, Reason:
// tree_give} per item that lands; return_item's hand-back carries tree_give.

const (
	treeAcqItem   = 975401
	treeAcqUserId = 9754
	treeAcqRoomId = 975490
)

func seedTreeAcquire(t *testing.T) *users.UserRecord {
	t.Helper()
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		treeAcqItem: {ItemId: treeAcqItem, Name: "river stone", Type: items.Object, Weight: 0.1},
	}))
	t.Cleanup(seedTestRoom(t, treeAcqRoomId, "Testzone"))
	t.Cleanup(seedTestUser(t, treeAcqUserId, "treeacq", "Treeacq", treeAcqRoomId))
	u := users.GetByUserId(treeAcqUserId)
	require.NotNil(t, u)
	return u
}

func treeAcqGains(t *testing.T) []events.ItemOwnership {
	t.Helper()
	var gains []events.ItemOwnership
	for _, e := range events.DrainQueuedItemOwnershipForTest(treeAcqUserId) {
		if e.Gained {
			gains = append(gains, e)
		}
	}
	return gains
}

func TestActGiveItem_AcquiresWithTreeGiveReason(t *testing.T) {
	seedTreeAcquire(t)
	ctx := &EvalContext{Event: EventContext{UserId: treeAcqUserId}}

	require.Equal(t, Success, actGiveItem(map[string]any{"item_id": treeAcqItem}, ctx))

	gains := treeAcqGains(t)
	require.Len(t, gains, 1)
	assert.Equal(t, "tree_give", gains[0].Reason)
	assert.Equal(t, treeAcqItem, gains[0].Item.ItemId)
}

func TestActGiveItemMultiple_OneGainPerItemWithTreeGiveReason(t *testing.T) {
	seedTreeAcquire(t)
	ctx := &EvalContext{Event: EventContext{UserId: treeAcqUserId}}

	require.Equal(t, Success, actGiveItemMultiple(map[string]any{"item_id": treeAcqItem, "count": 3}, ctx))

	gains := treeAcqGains(t)
	require.Len(t, gains, 3)
	for _, g := range gains {
		assert.Equal(t, "tree_give", g.Reason)
	}
}

func TestActReturnItem_HandBackCarriesTreeGiveReason(t *testing.T) {
	cleanup := seedReturnItemFixture(t)
	defer cleanup()
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	mob := mobs.GetInstance(riInstanceId)
	require.NotNil(t, mob)
	given := items.New(riItemId)
	require.True(t, mob.Character.StoreItem(given))
	ctx := &EvalContext{
		InstanceId: riInstanceId, MobId: riTemplateId, RoomId: riRoomId, MobName: "test guard",
		Event: EventContext{EventType: "player_give", UserId: riUserId, ItemId: riItemId, ItemUUID: given.UUID, RoomId: riRoomId},
	}

	require.Equal(t, Success, actReturnItem(nil, ctx))

	gains := 0
	for _, e := range events.DrainQueuedItemOwnershipForTest(riUserId) {
		if e.Gained {
			gains++
			assert.Equal(t, "tree_give", e.Reason)
		}
	}
	assert.Equal(t, 1, gains)
}
