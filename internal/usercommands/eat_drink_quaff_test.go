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

	// The names that collide on "tea": "Steady" contains it too.
	edqSteadyId  = 39324 // a potion
	edqHerbalId  = 39325 // a cooked drink
	edqMintId    = 39326 // a cooked drink
	edqTeaLeafId = 39327 // not edible
	edqTeaBombId = 39328 // a throwable, which a bandolier also holds
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

		edqSteadyId:  {ItemId: edqSteadyId, Name: "Lake-tonic of Steady Hand", Type: items.Potion, Subtype: items.Drinkable, Uses: 1},
		edqHerbalId:  {ItemId: edqHerbalId, Name: "Herbal Tea", Type: items.Drink, Subtype: items.Edible, Uses: 1},
		edqMintId:    {ItemId: edqMintId, Name: "Mint Tea", Type: items.Drink, Subtype: items.Edible, Uses: 1},
		edqTeaLeafId: {ItemId: edqTeaLeafId, Name: "Tea Leaf", Type: items.Object, Subtype: items.Mundane},
		edqTeaBombId: {ItemId: edqTeaBombId, Name: "Tea Bomb", Type: items.Object, Subtype: items.Throwable, Uses: 1},
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

// edqItem is a new item made at craftedRound, so a test can say which of two
// bandolier items is the older one (FindInPotions takes the oldest match).
func edqItem(itemId int, craftedRound uint64) items.Item {
	it := items.New(itemId)
	it.CraftedRound = craftedRound
	return it
}

// edqFoodRun types `verb typed` with the given pack and bandolier, and
// returns what the player read.
func edqFoodRun(t *testing.T, user *users.UserRecord, room *rooms.Room, verb, typed string, pack, bandolier []items.Item) string {
	t.Helper()
	user.Character.Items = pack
	user.Character.PotionItems = bandolier
	events.DrainQueuedMessagesForTest(1)
	_, err := userCommands[verb].Func(typed, user, room, 0)
	require.NoError(t, err)
	return aimTold(1)
}

// An older potion in the bandolier must not hide the tea beside it: `drink
// tea` said "That is a potion. Try quaff tea." and following the hint drank
// the tonic, because "Steady" contains "tea".
func TestDrinkTakesBandolierTeaOverAnOlderTonic(t *testing.T) {
	user, room := edqScene(t)
	told := edqFoodRun(t, user, room, "drink", "tea", nil,
		[]items.Item{edqItem(edqSteadyId, 1), edqItem(edqHerbalId, 5)})
	assert.Contains(t, told, "You drink some of the Herbal Tea.")
	assert.False(t, edqCarries(user, edqHerbalId), "the tea is drunk")
	assert.True(t, edqCarries(user, edqSteadyId), "the tonic stays")
}

// A potion in the bandolier beats an inedible match in the pack: the player
// read "You can't drink Tea Leaf." and never heard of quaff.
func TestDrinkNamesQuaffOverAnInedibleInThePack(t *testing.T) {
	user, room := edqScene(t)
	told := edqFoodRun(t, user, room, "drink", "tea",
		[]items.Item{items.New(edqTeaLeafId)}, []items.Item{items.New(edqSteadyId)})
	assert.Contains(t, told, "That is a potion.")
	assert.Contains(t, told, "quaff tea")
	assert.NotContains(t, told, "can't drink")
	assert.True(t, edqCarries(user, edqTeaLeafId))
	assert.True(t, edqCarries(user, edqSteadyId))
}

func TestEatAndDrinkLeaveAPotionInTheBandolier(t *testing.T) {
	user, room := edqScene(t)
	for _, verb := range []string{"drink", "eat"} {
		told := edqFoodRun(t, user, room, verb, "test tonic", nil, []items.Item{items.New(edqTonicId)})
		assert.Contains(t, told, "That is a potion.", verb)
		assert.Contains(t, told, "quaff test tonic", verb)
		assert.True(t, edqCarries(user, edqTonicId), "%s must not consume a potion", verb)
	}
}

// The lookup order: edible in the pack, then edible in the bandolier, then a
// potion in either, then anything else carried, then nothing.
func TestFoodLookupOrder(t *testing.T) {
	user, room := edqScene(t)
	leaf, herbal, mint := items.New(edqTeaLeafId), items.New(edqHerbalId), items.New(edqMintId)

	told := edqFoodRun(t, user, room, "drink", "tea", []items.Item{mint}, []items.Item{herbal})
	assert.Contains(t, told, "You drink some of the Mint Tea.", "edible in the pack comes first")
	assert.True(t, edqCarries(user, edqHerbalId))

	told = edqFoodRun(t, user, room, "drink", "tea", []items.Item{leaf}, []items.Item{herbal})
	assert.Contains(t, told, "You drink some of the Herbal Tea.", "edible in the bandolier beats an inedible in the pack")
	assert.True(t, edqCarries(user, edqTeaLeafId))

	told = edqFoodRun(t, user, room, "drink", "tea", []items.Item{leaf, items.New(edqSteadyId)}, nil)
	assert.Contains(t, told, "That is a potion.", "a potion in the pack beats an inedible in the pack")

	told = edqFoodRun(t, user, room, "drink", "tea", []items.Item{leaf}, nil)
	assert.Contains(t, told, "You can't drink Tea Leaf.")

	told = edqFoodRun(t, user, room, "drink", "tea", nil, nil)
	assert.Contains(t, told, "You aren't carrying")
}

// A bandolier also holds throwables, and "You aren't carrying" was untrue of
// one: it is another item, so it gets the "can't" line.
func TestEatRefusesAThrowableInTheBandolierByName(t *testing.T) {
	user, room := edqScene(t)
	told := edqFoodRun(t, user, room, "eat", "tea bomb", nil, []items.Item{items.New(edqTeaBombId)})
	assert.Contains(t, told, "You can't eat Tea Bomb.")
	assert.True(t, edqCarries(user, edqTeaBombId))
}
