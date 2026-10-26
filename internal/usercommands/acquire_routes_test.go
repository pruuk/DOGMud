package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #375 phase 3a: instant crafts (both sites) and mail receipt acquire
// through the seam, raising exactly one ItemOwnership{Gained} with their
// reason; a refused mail item raises nothing.

func gainsFor(userId int) []events.ItemOwnership {
	var gains []events.ItemOwnership
	for _, e := range events.DrainQueuedItemOwnershipForTest(userId) {
		if e.Gained {
			gains = append(gains, e)
		}
	}
	return gains
}

func TestCraft_InstantCraftAcquiresWithCraftReason(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	recipe := craftQuestRecipe(t, "test-craft-acquire-instant", 0)
	user.Character.KnownRecipes = map[string]int{recipe.RecipeId: 1}
	t.Cleanup(func() { user.Character.KnownRecipes = nil })
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	// Craft()'s ImmediateComplete branch (actions.InitiateCraft).
	_, err := Craft(recipe.Name, user, room, 0)
	require.NoError(t, err)
	gains := gainsFor(user.UserId)
	require.Len(t, gains, 1, "the instant craft raises one gain")
	assert.Equal(t, "craft", gains[0].Reason)
	assert.Equal(t, 10001, gains[0].Item.ItemId)

	// completeCraft, the enchanting instant path.
	completeCraft(user, room, recipe)
	gains = gainsFor(user.UserId)
	require.Len(t, gains, 1, "completeCraft raises one gain")
	assert.Equal(t, "craft", gains[0].Reason)
}

func TestApplyMailReceipt_AcquiresWithMailReason(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		975501: {ItemId: 975501, Name: "parcel", Type: items.Object, Weight: 0.1},
		975502: {ItemId: 975502, Name: "crate", Type: items.Object, Weight: 2.0},
	}))
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })

	u := &users.UserRecord{UserId: 9755, Character: characters.New()}
	itm := items.New(975501)
	require.True(t, applyMailReceipt(u, &users.Message{Item: &itm}))
	gains := gainsFor(u.UserId)
	require.Len(t, gains, 1)
	assert.Equal(t, "mail", gains[0].Reason)

	weak := &users.UserRecord{UserId: 9756, Character: characters.New()}
	weak.Character.Stats.Strength.ValueAdj = 1
	heavy := items.New(975502)
	assert.False(t, applyMailReceipt(weak, &users.Message{Item: &heavy}), "a refused item defers the message")
	assert.Empty(t, events.DrainQueuedItemOwnershipForTest(0), "a refused mail item raises nothing")
}
