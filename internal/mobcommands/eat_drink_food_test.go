package mobcommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mfSteadyId  = 39431 // a potion; "Steady" contains "tea"
	mfHerbalId  = 39432 // a cooked drink
	mfBreadId   = 39433 // food
	mfTeaLeafId = 39434 // not edible
)

// mfObserver is the seeded player who stands in the mob's room.
const mfObserver = 1

var mfTag = regexp.MustCompile(`<[^>]*>`)

// mfScene is mob 100 in a lit room with the test items registered, an empty
// pack and bandolier, and a clean message queue for the observer.
func mfScene(t *testing.T) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		mfSteadyId:  {ItemId: mfSteadyId, Name: "Lake-tonic of Steady Hand", Type: items.Potion, Subtype: items.Drinkable, Uses: 1},
		mfHerbalId:  {ItemId: mfHerbalId, Name: "Herbal Tea", Type: items.Drink, Subtype: items.Edible, Uses: 1},
		mfBreadId:   {ItemId: mfBreadId, Name: "Test Bread", Type: items.Food, Subtype: items.Edible, Uses: 1},
		mfTeaLeafId: {ItemId: mfTeaLeafId, Name: "Tea Leaf", Type: items.Object, Subtype: items.Mundane},
	}))
	mob, room := getTestMobAndRoom(t)
	room.Lamp = rooms.LampPtr(90)
	mob.Character.Items = nil
	mob.Character.PotionItems = nil
	events.DrainQueuedMessagesForTest(mfObserver)
	return mob, room
}

// mfItems makes one item per id, in order, each older than the next, so the
// first of a bandolier is the one FindInPotions returns. Call it after mfScene:
// items.New reads the registered spec.
func mfItems(ids []int) []items.Item {
	made := make([]items.Item, 0, len(ids))
	for i, id := range ids {
		it := items.New(id)
		it.CraftedRound = uint64(i + 1)
		made = append(made, it)
	}
	return made
}

func mfCarries(mob *mobs.Mob, itemId int) bool {
	for _, it := range append(append([]items.Item{}, mob.Character.Items...), mob.Character.PotionItems...) {
		if it.ItemId == itemId {
			return true
		}
	}
	return false
}

func mfRoomRead() string {
	return mfTag.ReplaceAllString(strings.Join(events.DrainQueuedMessagesForTest(mfObserver), ""), "")
}

// A mob's `drink tea` could land on a potion and silently do nothing: the mob
// body had no edible-first pass. It finds the tea wherever the player finds it.
func TestMobDrinkLookupTakesTheEdibleMatch(t *testing.T) {
	for _, c := range []struct {
		name            string
		pack, bandolier []int
	}{
		{"edible past a potion in the pack", []int{mfSteadyId, mfHerbalId}, nil},
		{"edible past an older potion in the bandolier", nil, []int{mfSteadyId, mfHerbalId}},
		{"edible in the bandolier past a potion in the pack", []int{mfSteadyId}, []int{mfHerbalId}},
		{"edible in the bandolier past an inedible in the pack", []int{mfTeaLeafId}, []int{mfHerbalId}},
	} {
		t.Run(c.name, func(t *testing.T) {
			mob, room := mfScene(t)
			mob.Character.Items = mfItems(c.pack)
			mob.Character.PotionItems = mfItems(c.bandolier)
			handled, err := Drink("tea", mob, room)
			require.True(t, handled)
			require.NoError(t, err)
			assert.False(t, mfCarries(mob, mfHerbalId), "the tea is drunk")
			assert.Contains(t, mfRoomRead(), "drinks some Herbal Tea.")
		})
	}
}

// A potion aimed at the food command is left alone and the room reads nothing:
// a mob's refusal is silent, and it drinks a potion with quaff.
func TestMobEatAndDrinkLeaveAPotion(t *testing.T) {
	for _, c := range []struct {
		name            string
		pack, bandolier []int
	}{
		{"in the pack", []int{mfSteadyId}, nil},
		{"in the bandolier", nil, []int{mfSteadyId}},
	} {
		t.Run(c.name, func(t *testing.T) {
			mob, room := mfScene(t)
			mob.Character.Items = mfItems(c.pack)
			mob.Character.PotionItems = mfItems(c.bandolier)
			for _, cmd := range []func(string, *mobs.Mob, *rooms.Room) (bool, error){Eat, Drink} {
				_, err := cmd("steady", mob, room)
				require.NoError(t, err)
				assert.True(t, mfCarries(mob, mfSteadyId), "a potion is not consumed")
			}
			assert.Empty(t, mfRoomRead())
		})
	}
}

// The old mob Drink refused while acting; the food body lost that when drink
// became food. A busy mob eats nothing, as a busy player does.
func TestMobEatAndDrinkWaitWhileBusy(t *testing.T) {
	for name, cmd := range map[string]func(string, *mobs.Mob, *rooms.Room) (bool, error){"eat": Eat, "drink": Drink} {
		t.Run(name, func(t *testing.T) {
			mob, room := mfScene(t)
			mob.Character.Items = []items.Item{items.New(mfBreadId)}
			mob.Character.Activity = activity.NewMachine()
			require.NoError(t, mob.Character.Activity.TransitionToCrafting(
				activity.CraftingData{RecipeId: "test", RoundsTotal: 1},
				state.TransitionReason{Trigger: activity.TriggerCraftBegin},
			))
			t.Cleanup(func() { mob.Character.Activity = nil })

			_, err := cmd("bread", mob, room)
			require.NoError(t, err)
			assert.True(t, mfCarries(mob, mfBreadId), "a busy mob must not eat")
			assert.Empty(t, mfRoomRead())
		})
	}
}

// drink and eat are one food body for a mob too: the word typed does not
// choose the line, the item's type does.
func TestMobFoodRoomLineFollowsTheItem(t *testing.T) {
	for _, c := range []struct {
		name  string
		cmd   func(string, *mobs.Mob, *rooms.Room) (bool, error)
		item  int
		typed string
		want  string
	}{
		{"drink a cooked drink", Drink, mfHerbalId, "herbal tea", "drinks some Herbal Tea."},
		{"eat a cooked drink", Eat, mfHerbalId, "herbal tea", "drinks some Herbal Tea."},
		{"drink food", Drink, mfBreadId, "bread", "eats some Test Bread."},
		{"eat food", Eat, mfBreadId, "bread", "eats some Test Bread."},
	} {
		t.Run(c.name, func(t *testing.T) {
			mob, room := mfScene(t)
			mob.Character.Items = []items.Item{items.New(c.item)}
			_, err := c.cmd(c.typed, mob, room)
			require.NoError(t, err)
			assert.False(t, mfCarries(mob, c.item), "the item is consumed")
			assert.Contains(t, mfRoomRead(), c.want)
		})
	}
}

// A mob's quaff is the potion half of the same split: it drinks from the
// bandolier, oldest first, and it leaves a cooked drink alone.
func TestMobQuaffDrinksFromTheBandolierAndLeavesFood(t *testing.T) {
	mob, room := mfScene(t)
	mob.Character.Items = []items.Item{items.New(mfHerbalId)}
	mob.Character.PotionItems = []items.Item{items.New(mfSteadyId)}

	_, err := Quaff("herbal tea", mob, room)
	require.NoError(t, err)
	assert.True(t, mfCarries(mob, mfHerbalId), "quaff does not take a cooked drink")
	assert.Empty(t, mfRoomRead())

	_, err = Quaff("steady", mob, room)
	require.NoError(t, err)
	assert.False(t, mfCarries(mob, mfSteadyId), "quaff drinks a potion from the bandolier")
	assert.Contains(t, mfRoomRead(), "drinks Lake-tonic of Steady Hand.")
}
