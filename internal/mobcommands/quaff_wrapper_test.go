package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

// TestMobQuaffRunsTheSharedBody pins that the mob command drinks through
// actions.Quaff (drink path unification): the potion's toxicity lands and
// a special potion applies in full. The mob path used to be a 49-line copy
// that charged no toxicity and ran no special potion, so both fail on it.
func TestMobQuaffRunsTheSharedBody(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	const brewId, catalystId = 39401, 30067
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		brewId: {ItemId: brewId, Name: "test brew", Type: items.Potion, Subtype: items.Drinkable,
			Uses: 1, Toxicity: 1},
		catalystId: {ItemId: catalystId, Name: "catalyst of unmaking", Type: items.Potion,
			Subtype: items.Drinkable, Uses: 1},
	}))

	mob, room := getTestMobAndRoom(t)
	mob.Character.Stats.Vitality.Base = 300
	mob.Character.Stats.Vitality.Recalculate()
	mob.Character.Toxicity = 0
	require.True(t, mob.Character.StoreItem(items.New(brewId)))
	require.True(t, mob.Character.StoreItem(items.New(catalystId)))

	handled, err := Quaff("test brew", mob, room)
	require.True(t, handled)
	require.NoError(t, err)
	require.InDelta(t, 1.0, mob.Character.Toxicity, 1e-9, "a mob's drink must charge the potion's toxicity")

	mob.Character.Mutations = map[string]int{"keen-eyes": 1}
	_, err = Quaff("catalyst", mob, room)
	require.NoError(t, err)
	require.Empty(t, mob.Character.Mutations, "the Catalyst must scour a mob's mutations too")
	require.Equal(t, 3, mob.Character.MutationRerollBonus, "the Catalyst grants a mob its reroll charges")
}
