package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const shopWakeSleepId = 9850

// shopScene: Marn (mob instance 851), a shopkeeper asleep in shop room 9851,
// and a customer Kesh (9852).
func shopScene(t *testing.T) (*rooms.Room, *mobs.Mob, *users.UserRecord) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		shopWakeSleepId: {ConditionId: shopWakeSleepId, Name: "Test Sleep", Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	lit := 60
	shop := &rooms.Room{RoomId: 9851, Zone: "ShopZone", Lamp: rooms.LampPtr(lit)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9851: shop},
		map[string]*rooms.ZoneConfig{"ShopZone": {Name: "ShopZone", RoomId: 9851}}))
	marn := &mobs.Mob{MobId: 85, InstanceId: 851, Zone: "ShopZone"}
	marn.Character.Name = "Marn"
	marn.Character.RoomId = 9851
	marn.Character.Conditions = conditions.New()
	marn.Character.Shop = characters.Shop{{ItemId: 1, Quantity: 1}}
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{85: {MobId: 85, Zone: "ShopZone"}}, map[int]*mobs.Mob{851: marn}))
	shop.AddMob(851)
	require.True(t, marn.Character.Conditions.AddCondition(shopWakeSleepId, true), "precondition: Marn asleep")
	require.True(t, marn.Character.HasConditionFlag(conditions.Sleeping), "precondition: Marn asleep")
	kesh := users.NewTestUser(9852, "kesh", "Kesh", 99852)
	kesh.Character.RoomId = 9851
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9852: kesh}))
	shop.AddPlayer(9852)
	return shop, marn, kesh
}

func hideForTest(t *testing.T, u *users.UserRecord) {
	t.Helper()
	c := u.Character
	c.Awareness = awareness.NewMachine()
	reason := state.TransitionReason{Trigger: "shop_wake_test"}
	if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatalf("concealing: %v", err)
	}
	c.Awareness.ResolveConcealment(true, reason)
	require.True(t, c.IsHidden(), "fixture is not hidden")
}

// Spec: a customer walking in wakes a sleeping shopkeeper.
func TestEntryWakesASleepingShopkeeper(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	wakeKeepersOnEntry(shop, kesh)
	require.False(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}

// Spec: a hidden player walking in does not.
func TestHiddenEntryLeavesTheShopkeeperAsleep(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	hideForTest(t, kesh)
	wakeKeepersOnEntry(shop, kesh)
	require.True(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}

// Spec: a shop command wakes the keeper and goes on, instead of refusing.
func TestShopCommandWakesTheKeeperAndServes(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	hideForTest(t, kesh)
	require.True(t, shopOpenFor(shop, kesh), "the shop is open once its keeper wakes")
	require.False(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}
