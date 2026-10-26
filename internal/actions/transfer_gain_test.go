package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The transfer helpers route the gaining side through the acquire seam
// (internal/acquire, #375 phase 3a): one gain
// event with the route's reason, and the giver's loss event unchanged.
func TestTransferItemToBackpack_GainCarriesGetReason(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	char := newTestChar()
	room := newTestRoom()
	itm := newTestItem(37504)
	room.Items = append(room.Items, itm)

	err := TransferItemToBackpack(itm, char, 3753, 0,
		func(i items.Item) { room.RemoveItem(i, false) },
		func(i items.Item) { room.AddItem(i, false) })
	require.NoError(t, err)

	got := events.DrainQueuedItemOwnershipForTest(0)
	require.Len(t, got, 1, "exactly one ownership event")
	assert.True(t, got[0].Gained)
	assert.Equal(t, 3753, got[0].UserId)
	assert.Equal(t, "get", got[0].Reason)
}

func TestTransferItemToBackpack_RefusedQueuesNothing(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	char := newWeakChar()
	room := newTestRoom()
	itm := newHeavyTestItem(37505)
	room.Items = append(room.Items, itm)

	err := TransferItemToBackpack(itm, char, 3754, 0,
		func(i items.Item) { room.RemoveItem(i, false) },
		func(i items.Item) { room.AddItem(i, false) })
	assert.ErrorIs(t, err, ErrInventoryFull)
	assert.Empty(t, events.DrainQueuedItemOwnershipForTest(0))
}

func TestTransferItemBetweenChars_OneLossOneGainWithGiveReason(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	charA := newTestChar()
	charB := newTestChar()
	itm := newTestItem(37506)
	charA.Items = append(charA.Items, itm)

	require.NoError(t, TransferItemBetweenChars(itm, charA, 3755, 0, charB, 3756, 0, acquire.AcquireGive))

	lost := events.DrainQueuedItemOwnershipForTest(3755)
	require.Len(t, lost, 1, "giver: exactly one event")
	assert.False(t, lost[0].Gained)
	assert.Equal(t, "", lost[0].Reason, "a loss carries no reason")

	gained := events.DrainQueuedItemOwnershipForTest(3756)
	require.Len(t, gained, 1, "receiver: exactly one event")
	assert.True(t, gained[0].Gained)
	assert.Equal(t, "give", gained[0].Reason)
}

// A mob receiver keeps its MobInstanceId on the gain event, as before.
func TestTransferItemBetweenChars_MobReceiverKeepsInstanceId(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	charA := newTestChar()
	charB := newTestChar()
	itm := newTestItem(37507)
	charA.Items = append(charA.Items, itm)

	require.NoError(t, TransferItemBetweenChars(itm, charA, 3757, 0, charB, 0, 88, acquire.AcquireGive))

	all := events.DrainQueuedItemOwnershipForTest(0)
	require.Len(t, all, 2)
	var gains []events.ItemOwnership
	for _, e := range all {
		if e.Gained {
			gains = append(gains, e)
		}
	}
	require.Len(t, gains, 1)
	assert.Equal(t, 88, gains[0].MobInstanceId)
	assert.Equal(t, 0, gains[0].UserId)
}

func TestTransferItemBetweenChars_RefusedQueuesNothing(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	charA := newTestChar()
	charB := newWeakChar()
	itm := newHeavyTestItem(37508)
	charA.Items = append(charA.Items, itm)

	err := TransferItemBetweenChars(itm, charA, 3758, 0, charB, 3759, 0, acquire.AcquireGive)
	assert.ErrorIs(t, err, ErrInventoryFull)
	assert.Empty(t, events.DrainQueuedItemOwnershipForTest(0))
}
