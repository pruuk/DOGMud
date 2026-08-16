package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllByName_FiltersOnTheName reproduces the playtest report that
// `drop all stone`, carrying three stones and one Healing Salve, dropped the
// salve as well, and `get all stone` then picked the whole floor back up.
//
// The cause was that only the DOT form was ever filtered. util.GetMatchNumber
// understands "all.sword" and knows nothing about "all sword", and both
// commands tested `args[0] == "all"` and returned from a grab-everything /
// drop-everything fallback long before the dot-form handler could run. A
// command that names exactly one thing was silently acting on the player's
// entire inventory, quest items included.
//
// Both spellings must filter, and must filter identically.
func TestAllByName_FiltersOnTheName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	// The two spellings are asserted to behave the same way, so the table runs
	// the identical scenario through each and compares against one expectation.
	for _, form := range []struct {
		name string
		drop string
		get  string
	}{
		{name: "space_form", drop: "all sword", get: "all sword"},
		{name: "dot_form", drop: "all.sword", get: "all.sword"},
	} {
		t.Run(form.name, func(t *testing.T) {
			user, room := getTestUserAndRoom(t)

			room.Items = nil
			user.Character.Items = []items.Item{
				items.New(10001), // Iron Sword
				items.New(10001), // Iron Sword
				items.New(10001), // Iron Sword
				items.New(30001), // Healing Potion — the bystander
			}

			handled, err := Drop(form.drop, user, room, 0)
			assert.True(t, handled)
			require.NoError(t, err)

			require.Len(t, user.Character.Items, 1,
				"only the three swords should have been dropped")
			assert.Equal(t, 30001, user.Character.Items[0].ItemId,
				"the potion must still be in the pack")
			assert.Len(t, room.Items, 3, "three swords should be on the floor")

			// And the mirror: sweeping them back up must not also sweep up
			// anything else lying in the room.
			room.AddItem(items.New(30001), false) // a second potion, on the floor

			handled, err = Get(form.get, user, room, 0)
			assert.True(t, handled)
			require.NoError(t, err)

			assert.Len(t, user.Character.Items, 4,
				"the three swords should be back, and nothing else")
			require.Len(t, room.Items, 1, "the floor potion must be left behind")
			assert.Equal(t, 30001, room.Items[0].ItemId)
		})
	}
}

// TestDropAllGold_DispatchesOnTheWordNotTheBalance covers the second half of
// the same defect. `drop all gold` used to require `Gold > 0` to be recognised
// AS a gold command; a player with no gold fell through to the bare-"drop all"
// fallback and emptied their entire pack instead of being told they had no
// gold.
func TestDropAllGold_DispatchesOnTheWordNotTheBalance(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)
	room.Items = nil
	user.Character.Gold = 0
	user.Character.Items = []items.Item{items.New(10001), items.New(30001)}

	handled, err := Drop("all gold", user, room, 0)
	assert.True(t, handled)
	require.NoError(t, err)

	assert.Len(t, user.Character.Items, 2,
		"a penniless `drop all gold` must not drop the player's items")
	assert.Empty(t, room.Items)
}
