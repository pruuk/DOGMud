package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	edqBreadId = 39321 // food
	edqTeaId   = 39322 // a cooked drink
	edqTonicId = 39323 // a potion
)

// edqScene is user 1 in a lit room, with the three test items registered and
// an empty pack and bandolier.
func edqScene(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	user, room := aimScene(t, aimFull)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		edqBreadId: {ItemId: edqBreadId, Name: "test bread", Type: items.Food, Subtype: items.Edible, Uses: 1},
		edqTeaId:   {ItemId: edqTeaId, Name: "test tea", Type: items.Drink, Subtype: items.Edible, Uses: 1},
		edqTonicId: {ItemId: edqTonicId, Name: "test tonic", Type: items.Potion, Subtype: items.Drinkable, Uses: 1},
	}))
	user.Character.Items = nil
	user.Character.PotionItems = nil
	return user, room
}

func edqCarries(user *users.UserRecord, itemId int) bool {
	for _, it := range append(append([]items.Item{}, user.Character.Items...), user.Character.PotionItems...) {
		if it.ItemId == itemId {
			return true
		}
	}
	return false
}

// eat and drink are one command (#375 phase 7, owner ruling 2026-10-30):
// both take food and cooked drinks, and the line says eat or drink by the
// item, not by the word typed.
func TestEatAndDrinkTakeFoodAndCookedDrinks(t *testing.T) {
	user, room := edqScene(t)
	for _, c := range []struct {
		verb   string
		itemId int
		typed  string
		line   string
	}{
		{"eat", edqBreadId, "test bread", "You eat some of the"},
		{"drink", edqBreadId, "test bread", "You eat some of the"},
		{"drink", edqTeaId, "test tea", "You drink some of the"},
		{"eat", edqTeaId, "test tea", "You drink some of the"},
	} {
		require.True(t, user.Character.StoreItem(items.New(c.itemId)), c.verb)
		events.DrainQueuedMessagesForTest(1)
		handled, err := userCommands[c.verb].Func(c.typed, user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		told := aimTold(1)
		assert.Contains(t, told, c.line, "%s %s", c.verb, c.typed)
		assert.False(t, edqCarries(user, c.itemId), "%s %s must consume it", c.verb, c.typed)
	}
}

// drink or eat on a potion consumes nothing and names quaff.
func TestEatAndDrinkLeaveAPotion(t *testing.T) {
	user, room := edqScene(t)
	require.True(t, user.Character.StoreItem(items.New(edqTonicId)))
	for _, verb := range []string{"drink", "eat"} {
		events.DrainQueuedMessagesForTest(1)
		_, err := userCommands[verb].Func("test tonic", user, room, 0)
		require.NoError(t, err)
		told := aimTold(1)
		assert.Contains(t, told, "That is a potion.", verb)
		assert.Contains(t, told, "quaff test tonic", verb)
		assert.True(t, edqCarries(user, edqTonicId), "%s must not consume a potion", verb)
	}
}

// quaff drinks a potion and refuses a cooked drink by naming drink.
func TestQuaffTakesPotionsOnly(t *testing.T) {
	user, room := edqScene(t)
	require.True(t, user.Character.StoreItem(items.New(edqTonicId)))
	require.True(t, user.Character.StoreItem(items.New(edqTeaId)))

	events.DrainQueuedMessagesForTest(1)
	_, err := Quaff("test tea", user, room, 0)
	require.NoError(t, err)
	told := aimTold(1)
	assert.Contains(t, told, "That is not a potion.")
	assert.Contains(t, told, "drink test tea")
	assert.True(t, edqCarries(user, edqTeaId))

	events.DrainQueuedMessagesForTest(1)
	_, err = Quaff("test tonic", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, aimTold(1), "You drink the")
	assert.False(t, edqCarries(user, edqTonicId))
}

// Herbal Tea was a potion until #375 phase 7, so a player may hold one in a
// bandolier. drink still finds it there.
func TestDrinkFindsACookedDrinkInTheBandolier(t *testing.T) {
	user, room := edqScene(t)
	user.Character.PotionItems = []items.Item{items.New(edqTeaId)}
	events.DrainQueuedMessagesForTest(1)
	_, err := Drink("test tea", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, aimTold(1), "You drink some of the")
	assert.Empty(t, user.Character.PotionItems)
}
