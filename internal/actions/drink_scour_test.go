package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// drinkTestUser builds a bare player in a bare room for the drink tests
// that moved here from usercommands with drink path unification. Species 0
// is seeded with no intrinsic mutations, so a scour leaves nothing behind.
func drinkTestUser(t *testing.T, userId int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "Human", Size: species.Medium, Selectable: true},
	}))
	c := characters.New()
	c.Name = "Drinker"
	c.RoomId = 99999
	return &users.UserRecord{UserId: userId, Character: c}, newEmptyTestRoom(t)
}

// countDrinkItemsById counts the items in a slice with the given id.
func countDrinkItemsById(itemSlice []items.Item, id int) int {
	n := 0
	for _, it := range itemSlice {
		if it.ItemId == id {
			n++
		}
	}
	return n
}

// TestDrink_CatalystOfUnmakingScoursMutations locks the #22 crash-site
// delivery: drinking the Catalyst of Unmaking (item 30067) runs
// Character.ScourMutations, stripping all acquired mutations down to species
// intrinsics and granting reroll charges that bias re-acquisition toward rare.
//
// It exercises the REAL command path (Drink -> special-case block -> the same
// ScourMutations the engine calls), not a shim, so a regression in the drink.go
// wiring (wrong item id, missing block, wrong charge count) fails here.
func TestDrink_CatalystOfUnmakingScoursMutations(t *testing.T) {
	// Register the scour potion in the item registry so items.New/GetSpec
	// resolve it. Only 30067 is needed for the drink path.
	itemCleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		30067: {
			ItemId:     30067,
			Name:       "Catalyst of Unmaking",
			NameSimple: "catalyst",
			Type:       items.Potion,
			Subtype:    items.Drinkable,
			Uses:       1,
		},
	})
	defer itemCleanup()

	user, room := drinkTestUser(t, 7201)
	user.Character.Conditions = conditions.New()
	user.Character.SpeciesId = 0 // Human: no intrinsic mutations seeded

	// Give the player two non-intrinsic mutations and some progress.
	user.Character.Mutations = map[string]int{"keen-eyes": 2, "large": 1}
	user.Character.MutationProgress = 0.7
	user.Character.MutationRerollBonus = 0

	// Seed the potion into the player's backpack.
	require.True(t, user.Character.StoreItem(items.New(30067)),
		"failed to seed Catalyst of Unmaking in player's backpack")

	res := Quaff(NewUserActorInRoom(user, room).(DrinkActor), "catalyst")
	assert.True(t, res.Drank, "the Catalyst must be drunk")

	// Mutations scoured to species intrinsics only (none for Human 0).
	assert.Empty(t, user.Character.Mutations,
		"drinking the Catalyst must clear acquired mutations to species intrinsics")
	// Reroll charges granted (rare bias while > 0).
	assert.Greater(t, user.Character.MutationRerollBonus, 0,
		"drinking the Catalyst must grant reroll charges")
	// Progress reset.
	assert.Equal(t, float64(0), user.Character.MutationProgress,
		"drinking the Catalyst must reset mutation progress")

	// Potion consumed.
	assert.Equal(t, 0, countDrinkItemsById(user.Character.Items, 30067),
		"the Catalyst must be consumed on drink")
}

// TestDrink_PhialOfSecondBirthScoursAndGrantsRareMutation locks the pinnacle
// remort potion (item 40181): drinking it runs Character.ScourMutations(0)
// (no reroll charges: the grant below is immediate) and then grants exactly
// one mutation from the rarity-floored pool (>= phialRarityFloor).
func TestDrink_PhialOfSecondBirthScoursAndGrantsRareMutation(t *testing.T) {
	mutCleanup := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"common-1": {MutationId: "common-1", Name: "Common One", Rarity: 2},
		"common-2": {MutationId: "common-2", Name: "Common Two", Rarity: 2},
		"rare-1":   {MutationId: "rare-1", Name: "Rare One", Rarity: 7},
		"rare-2":   {MutationId: "rare-2", Name: "Rare Two", Rarity: 7},
	})
	defer mutCleanup()

	itemCleanup := items.SeedItemsForTest(map[int]*items.ItemSpec{
		40181: {
			ItemId:     40181,
			Name:       "Phial of Second Birth",
			NameSimple: "phial",
			Type:       items.Potion,
			Subtype:    items.Drinkable,
			Uses:       1,
		},
	})
	defer itemCleanup()

	user, room := drinkTestUser(t, 7202)
	user.Character.Conditions = conditions.New()
	user.Character.SpeciesId = 0 // Human: no intrinsic mutations seeded

	// Give the player two non-intrinsic mutations and some progress.
	user.Character.Mutations = map[string]int{"common-1": 1, "common-2": 1}
	user.Character.MutationProgress = 0.7
	user.Character.MutationRerollBonus = 0

	// Seed the potion into the player's backpack.
	require.True(t, user.Character.StoreItem(items.New(40181)),
		"failed to seed Phial of Second Birth in player's backpack")

	res := Quaff(NewUserActorInRoom(user, room).(DrinkActor), "phial")
	assert.True(t, res.Drank, "the Phial must be drunk")

	// Exactly one new mutation present, at or above the rarity floor.
	require.Len(t, user.Character.Mutations, 1,
		"drinking the Phial must leave exactly one mutation granted")
	for id := range user.Character.Mutations {
		spec := mutations.GetMutation(id)
		require.NotNil(t, spec)
		assert.GreaterOrEqual(t, spec.Rarity, phialRarityFloor,
			"the granted mutation must be at or above the phial's rarity floor")
	}

	// No reroll charges granted: the phial's grant is immediate, not charge-based.
	assert.Equal(t, 0, user.Character.MutationRerollBonus,
		"drinking the Phial must not grant reroll charges")

	// Progress reset.
	assert.Equal(t, float64(0), user.Character.MutationProgress,
		"drinking the Phial must reset mutation progress")

	// Potion consumed.
	assert.Equal(t, 0, countDrinkItemsById(user.Character.Items, 40181),
		"the Phial must be consumed on drink")
}
