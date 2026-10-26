package acquire

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The acquire seam (#375 phase 3a): an item that lands raises exactly one
// ItemOwnership{Gained} carrying its reason; an item that does not land
// raises nothing.

func countCharItems(c *characters.Character) int {
	return len(c.Items) + len(c.ComponentItems) + len(c.PotionItems)
}

func newTestItem(id int) items.Item {
	return items.Item{ItemId: id, Spec: &items.ItemSpec{ItemId: id, Name: "test item", Weight: 0.1}}
}

// newHeavyTestItem weighs 2 pounds. With Strength 1 and the default
// CarryCapacityMultiplier (0.65) StoreItem refuses above 1.3 pounds, so one
// of these is refused by newWeakChar.
func newHeavyTestItem(id int) items.Item {
	return items.Item{ItemId: id, Spec: &items.ItemSpec{ItemId: id, Name: "heavy item", Weight: 2.0}}
}

func newWeakChar() *characters.Character {
	c := characters.New()
	c.Stats.Strength.ValueAdj = 1
	return c
}

func TestAcquireItem_StoresAndQueuesOneGainWithReason(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	user := &users.UserRecord{UserId: 3751, Character: characters.New()}
	itm := newTestItem(37501)

	require.True(t, AcquireItem(user, itm, AcquireGet))
	assert.Equal(t, 1, countCharItems(user.Character), "the item is carried")

	got := events.DrainQueuedItemOwnershipForTest(user.UserId)
	require.Len(t, got, 1, "exactly one ownership event")
	assert.True(t, got[0].Gained)
	assert.Equal(t, "get", got[0].Reason)
	assert.Equal(t, itm.ItemId, got[0].Item.ItemId)
	assert.Equal(t, 0, got[0].MobInstanceId)
}

// StoreItem refuses past twice carry capacity (the "crushed" tier, see
// questengine/bridge.go GiveItem). A refused store raises nothing.
func TestAcquireItem_RefusedStoreQueuesNothing(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	user := &users.UserRecord{UserId: 3752, Character: newWeakChar()}

	assert.False(t, AcquireItem(user, newHeavyTestItem(37502), AcquireGet))
	assert.Equal(t, 0, countCharItems(user.Character), "nothing is carried")
	assert.Empty(t, events.DrainQueuedItemOwnershipForTest(0), "no ownership event")
}

func TestAcquireItem_NilUserRefuses(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	assert.False(t, AcquireItem(nil, newTestItem(37503), AcquireGet))
	assert.Empty(t, events.DrainQueuedItemOwnershipForTest(0))
}

// A mob receiver keeps its MobInstanceId on the gain event.
func TestAcquireItemForChar_KeepsBothIds(t *testing.T) {
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	require.True(t, AcquireItemForChar(characters.New(), 0, 88, newTestItem(37509), AcquireGive))
	got := events.DrainQueuedItemOwnershipForTest(0)
	require.Len(t, got, 1)
	assert.Equal(t, 88, got[0].MobInstanceId)
	assert.Equal(t, "give", got[0].Reason)
}
