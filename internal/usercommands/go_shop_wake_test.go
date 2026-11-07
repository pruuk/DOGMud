package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

const (
	goShopWakeSleepCond = 9871 // Sleeping
	goShopWakeLightCond = 9872 // a carried light, literal strength 60
)

// goShopWakeScene: the seeded mob 100 becomes a shopkeeper asleep in their
// own shop (room 1), and Aliceia waits in the cave (room 2) to walk south
// into it. The RNG is pinned so the sneak contest on entry is repeatable.
func goShopWakeScene(t *testing.T) (*mobs.Mob, *users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		goShopWakeSleepCond: {ConditionId: goShopWakeSleepCond, Name: "Test Sleep", Flags: []conditions.Flag{conditions.Sleeping}},
		goShopWakeLightCond: {ConditionId: goShopWakeLightCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 60}}},
	}))
	t.Cleanup(util.SetRandForTest(7))

	keeper := mobs.GetInstance(100)
	keeper.AutoAggro = false
	keeper.Groups = nil
	keeper.Character.Name = "Marn"
	keeper.Character.Shop = characters.Shop{{ItemId: 10001, Quantity: 1}}
	require.Equal(t, 1, keeper.HomeRoomId, "fixture: room 1 is the keeper's shop")
	require.True(t, keeper.Character.Conditions.AddCondition(goShopWakeSleepCond, true), "fixture: the keeper is asleep")

	// Both seeded players leave the shop for the cave: Bobrick would
	// otherwise stand in the shop and could spot a sneaking mover.
	room1, cave := rooms.LoadRoom(1), rooms.LoadRoom(2)
	mover := users.GetByUserId(1)
	for _, u := range []*users.UserRecord{mover, users.GetByUserId(2)} {
		room1.RemovePlayer(u.UserId)
		u.Character.RoomId = 2
		cave.AddPlayer(u.UserId)
	}
	mover.Character.ActionPoints = 100
	events.DrainQueuedMessagesForTest(mover.UserId)
	return keeper, mover, cave
}

// A customer walking in, not sneaking, wakes the keeper: the control for the
// sneak test below.
func TestGo_EntryWakesASleepingShopkeeper(t *testing.T) {
	keeper, mover, cave := goShopWakeScene(t)
	handled, err := Go("south", mover, cave, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, mover.Character.RoomId, "fixture: the mover arrived")
	require.False(t, keeper.Character.HasConditionFlag(conditions.Sleeping))
	require.Contains(t, hoodTestText(mover.UserId), "Marn</ansi> wakes as you come in.")
}

// Review 2026-11-07: a player sneaking in (the misc-data flag the sneak
// command sets before the Hidden condition lands) leaves the keeper asleep.
func TestGo_SneakingEntryLeavesTheShopkeeperAsleep(t *testing.T) {
	keeper, mover, cave := goShopWakeScene(t)
	mover.Character.SetMiscData(`sneaking`, true)
	require.False(t, mover.Character.IsHidden(), "fixture: only the misc-data flag says sneaking")
	handled, err := Go("south", mover, cave, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, mover.Character.RoomId, "fixture: the mover arrived")
	sneaking, _ := mover.Character.GetMiscData(`sneaking`).(bool)
	require.True(t, sneaking, "fixture: the sneak survived entry detection")
	require.True(t, keeper.Character.HasConditionFlag(conditions.Sleeping), "a sneaking entry woke the keeper")
}

// Review 2026-11-07: the light-arrival wake treats a keeper in their own
// shop as wakeKeeper does (lantern relit, the room told), not a bare cancel.
// A sneaking player carrying a light skips the customer wake, so the light
// wake is the one that fires.
func TestGo_LitArrivalWakesAKeeperThroughWakeKeeper(t *testing.T) {
	keeper, mover, cave := goShopWakeScene(t)
	mover.Character.SetMiscData(`sneaking`, true)
	require.True(t, mover.Character.Conditions.AddCondition(goShopWakeLightCond, false))
	require.True(t, mover.Character.EmitsLight(), "fixture: the mover carries a light")
	handled, err := Go("south", mover, cave, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, mover.Character.RoomId, "fixture: the mover arrived")
	sneaking, _ := mover.Character.GetMiscData(`sneaking`).(bool)
	require.True(t, sneaking, "fixture: the sneak survived entry detection")
	require.False(t, keeper.Character.HasConditionFlag(conditions.Sleeping), "the light woke the keeper")
	got := hoodTestText(mover.UserId)
	require.Contains(t, got, "Marn</ansi> wakes up.", "a keeper woken by the light says so")
	require.NotContains(t, got, "come in", "the light woke the keeper, not a customer's entry")
}
