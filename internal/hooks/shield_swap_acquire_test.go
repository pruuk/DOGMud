package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #375 phase 3a: the broken-item replacement for a shattered offhand
// acquires through the seam. It lands with exactly one
// ItemOwnership{Gained, Reason: shield_swap}; when the defender cannot carry
// it, it drops to the floor and no gain is raised (it used to be raised
// anyway).

const (
	shieldAcqOffhand = 975301
	shieldAcqRoomId  = 975390
)

func seedShieldSwap(t *testing.T, brokenWeight float64) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	drainEventQueueOnCleanup(t)
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		shieldAcqOffhand: {ItemId: shieldAcqOffhand, Name: "cracked buckler", Type: items.Weapon, BreakChance: 100},
		20:               {ItemId: 20, Name: "broken item", Type: items.Object, Weight: brokenWeight},
	}))
	room := &rooms.Room{RoomId: shieldAcqRoomId, Lamp: rooms.LampPtr(90)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{shieldAcqRoomId: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(510, "shieldacq", "ShieldAcq", 5010)
	u.Character.RoomId = shieldAcqRoomId
	u.Character.Equipment.Offhand = items.New(shieldAcqOffhand)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{510: u}))
	return u, room
}

func TestOffhandBreak_ReplacementAcquiresWithShieldSwapReason(t *testing.T) {
	u, room := seedShieldSwap(t, 0.1)

	handleOffhandBreakUserDef(combat.AttackResult{Hit: true, CleanHit: true}, u, room)

	var gains []events.ItemOwnership
	for _, e := range events.DrainQueuedItemOwnershipForTest(510) {
		if e.Gained {
			gains = append(gains, e)
		}
	}
	require.Len(t, gains, 1, "exactly one gain: the broken item")
	assert.Equal(t, 20, gains[0].Item.ItemId)
	assert.Equal(t, "shield_swap", gains[0].Reason)
}

func TestOffhandBreak_ReplacementTooHeavyRaisesNoGain(t *testing.T) {
	u, room := seedShieldSwap(t, 2.0)
	u.Character.Stats.Strength.ValueAdj = 1

	handleOffhandBreakUserDef(combat.AttackResult{Hit: true, CleanHit: true}, u, room)

	for _, e := range events.DrainQueuedItemOwnershipForTest(510) {
		assert.False(t, e.Gained, "a replacement left on the floor is not a gain")
	}
	assert.Len(t, room.Items, 1, "the broken item drops to the floor")
}
