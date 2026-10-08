package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	equipLightTestCond  = 9771 // a carried light, literal strength 40
	equipLightTestTorch = 999975
	equipLightTestHat   = 999976
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
	require.False(t, conditions.AnyLightSource(nil))
	require.False(t, conditions.AnyLightSource([]int{987654}), "an unknown id is not a light")
}
