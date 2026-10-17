package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

// 2026-10-17 pre-deploy playtest: `eat meat` on Wild Hare Meat read "You
// can't eat Wild Hare Meat." with no pointer to cooking. A cooking ingredient
// now says to cook it; any other inedible thing keeps the plain refusal.
func TestEat_CookingIngredientPointsToCooking(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	const meatId, stoneId = 84064, 84065
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		meatId: {ItemId: meatId, Name: "Raw Test Meat", NameSimple: "meat", Type: items.Object,
			Subtype: items.Mundane, ComponentTag: "test-raw-meat", IsComponent: true},
		stoneId: {ItemId: stoneId, Name: "Grey Pebble", NameSimple: "pebble", Type: items.Object,
			Subtype: items.Mundane},
	}))
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{
		RecipeId: "test-meat-stew", Name: "Test Meat Stew", Skill: "cooking",
		Ingredients: []crafting.RecipeIngredient{{ItemTag: "test-raw-meat", Quantity: 1}},
	})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest("test-meat-stew") })

	meat, pebble := items.New(meatId), items.New(stoneId)
	require.True(t, user.Character.StoreItem(meat))
	require.True(t, user.Character.StoreItem(pebble))
	t.Cleanup(func() {
		user.Character.RemoveItem(meat)
		user.Character.RemoveItem(pebble)
	})

	events.DrainQueuedMessagesForTest(user.UserId)
	_, err := Eat("meat", user, room, 0)
	require.NoError(t, err)
	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "")
	require.Contains(t, out, "Cook it first.")
	require.Contains(t, out, `<ansi fg="command">help cooking</ansi>`)

	_, err = Eat("pebble", user, room, 0)
	require.NoError(t, err)
	out = strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "")
	require.Contains(t, out, `You can't eat <ansi fg="itemname">Grey Pebble</ansi>.`)
	require.NotContains(t, out, "cooking")
}
