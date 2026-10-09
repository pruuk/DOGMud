package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	equipLightTestCond  = 9771 // a carried light, literal strength 40
	equipLightTestDark  = 9772 // a darkness, literal strength 50
	equipLightTestTorch = 999975
	equipLightTestHat   = 999976
	equipLightTestStub  = 999977 // a light-slot item that sheds nothing
)

// equipLightFixture seeds a torch-shaped light-slot item that sheds light and
// a plain wearable that does not, and returns user 1 holding both.
func equipLightFixture(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "human", Size: species.Medium},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		equipLightTestCond: {ConditionId: equipLightTestCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 40}}},
		equipLightTestDark: {ConditionId: equipLightTestDark, Name: "Test Darkness", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 50}}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		equipLightTestTorch: {ItemId: equipLightTestTorch, Name: "test torch", NameSimple: "torch",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{equipLightTestCond}},
		equipLightTestHat: {ItemId: equipLightTestHat, Name: "test hat", NameSimple: "hat",
			Type: items.Head, Subtype: items.Wearable},
	}))
	user := users.GetByUserId(1)
	require.NotNil(t, user)
	user.Character.SpeciesId = 0
	user.Character.Stats.Strength.ValueAdj = 100
	user.Character.StoreItem(items.Item{ItemId: equipLightTestTorch})
	user.Character.StoreItem(items.Item{ItemId: equipLightTestHat})
	return user
}

// #260: `equip torch` said only "You wear your Torch." Putting on something
// that sheds light now says so; putting on anything else does not.
func TestEquippingALightSaysItCastsLight(t *testing.T) {
	user := equipLightFixture(t)
	_, room := getTestUserAndRoom(t)
	events.DrainQueuedMessagesForTest(user.UserId)

	_, err := Equip("test torch", user, room, 0)
	require.NoError(t, err)
	require.Equal(t, equipLightTestTorch, user.Character.Equipment.Light.ItemId, "fixture: the torch must be worn")
	got := hoodTestText(user.UserId)
	require.Contains(t, got, "You wear your")
	require.Contains(t, got, "It casts light around you.")

	_, err = Equip("test hat", user, room, 0)
	require.NoError(t, err)
	require.NotContains(t, hoodTestText(user.UserId), "casts light", "a hat sheds no light")
}

// AnyLightSource is the light twin of AnyDarknessSource: a darkness is never
// a light (lighting plan 5d, ruling D1), and an unknown id is skipped.
func TestAnyLightSource(t *testing.T) {
	equipLightFixture(t)
	require.True(t, conditions.AnyLightSource([]int{equipLightTestCond}))
	require.False(t, conditions.AnyLightSource([]int{equipLightTestDark}), "a darkness is never a light")
	require.False(t, conditions.AnyLightSource(nil))
	require.False(t, conditions.AnyLightSource([]int{987654}), "an unknown id is not a light")
}

// equipLightRoomFixture is equipLightFixture in cave room 2 with no sky and
// no lamp, so the torch is the room's only light, and user 2 watching. It
// adds a stub: a light-slot item that sheds nothing.
func equipLightRoomFixture(t *testing.T) (user, observer *users.UserRecord, room *rooms.Room) {
	t.Helper()
	user = equipLightFixture(t)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		equipLightTestTorch: {ItemId: equipLightTestTorch, Name: "test torch", NameSimple: "torch",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{equipLightTestCond}},
		equipLightTestHat: {ItemId: equipLightTestHat, Name: "test hat", NameSimple: "hat",
			Type: items.Head, Subtype: items.Wearable},
		equipLightTestStub: {ItemId: equipLightTestStub, Name: "test stub", NameSimple: "stub",
			Type: items.Light, Subtype: items.Wearable},
	}))
	user.Character.StoreItem(items.Item{ItemId: equipLightTestStub})
	room = rooms.LoadRoom(2)
	require.NotNil(t, room)
	room.Biome = "cave"
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = nil
	rooms.LoadRoom(user.Character.RoomId).RemovePlayer(user.UserId)
	user.Character.RoomId = 2
	room.AddPlayer(user.UserId)
	observer = users.GetByUserId(2)
	require.NotNil(t, observer)
	rooms.LoadRoom(observer.Character.RoomId).RemovePlayer(observer.UserId)
	observer.Character.RoomId = 2
	room.AddPlayer(observer.UserId)
	require.Less(t, room.LightLevel(), 25, "fixture: the room must be dark with no torch")
	events.DrainQueuedMessagesForTest(user.UserId)
	events.DrainQueuedMessagesForTest(observer.UserId)
	return user, observer, room
}

// #447: a light's equip and remove lines were judged after the change. The
// line announcing a light lands with the room as it was before (owner rule,
// 2026-10-05).
func TestLightLines_JudgedBeforeTheChange(t *testing.T) {
	t.Run("lighting a torch in the dark is not seen", func(t *testing.T) {
		user, observer, room := equipLightRoomFixture(t)
		_, err := Equip("test torch", user, room, 0)
		require.NoError(t, err)
		require.GreaterOrEqual(t, room.LightLevel(), 25, "fixture: the torch must light the room")
		got := hoodTestText(observer.UserId)
		require.NotContains(t, got, "puts on their", "the observer was blind when it went on")
		require.NotContains(t, got, user.Character.Name)
	})
	t.Run("taking off the only light is seen", func(t *testing.T) {
		user, observer, room := equipLightRoomFixture(t)
		_, err := Equip("test torch", user, room, 0)
		require.NoError(t, err)
		hoodTestText(observer.UserId)
		_, err = Remove("test torch", user, room, 0)
		require.NoError(t, err)
		require.Less(t, room.LightLevel(), 25, "fixture: the room must go dark")
		require.Contains(t, hoodTestText(observer.UserId), "removes their",
			"the observer saw the torch taken off before the dark fell")
	})
	t.Run("a displaced light is seen going and the stub coming", func(t *testing.T) {
		user, observer, room := equipLightRoomFixture(t)
		_, err := Equip("test torch", user, room, 0)
		require.NoError(t, err)
		hoodTestText(observer.UserId)
		_, err = Equip("test stub", user, room, 0)
		require.NoError(t, err)
		require.Equal(t, equipLightTestStub, user.Character.Equipment.Light.ItemId, "fixture: the stub must displace the torch")
		require.Less(t, room.LightLevel(), 25, "fixture: the room must go dark")
		got := hoodTestText(observer.UserId)
		require.Contains(t, got, "removes their", "the displaced torch, seen by its own light")
		require.Contains(t, got, "puts on their", "the stub, seen by the torch it replaced")
	})
}
