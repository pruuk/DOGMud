package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// The food room lines name the eater and go out the way the potion room lines
// do (drink_room_line_sight_test.go): an observer who makes out shapes reads
// "a figure" and one who sees nothing reads nothing. The scene is the drink
// scene with food added. A name tag alone is anonymized by any visual sender,
// so the two "named after" cases are what tell SendTextVisualHidingNames from
// SendTextVisual: the eater's name also stands, untagged, in the item's.

const (
	foodSightTeaId   = 39441
	foodSightBreadId = 39442
	foodSightNamedId = 39443
)

func seedFoodSightItems(t *testing.T) {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		foodSightTeaId:   {ItemId: foodSightTeaId, Name: "Test Tea", Type: items.Drink, Subtype: items.Edible, Uses: 2},
		foodSightBreadId: {ItemId: foodSightBreadId, Name: "Test Bread", Type: items.Food, Subtype: items.Edible, Uses: 2},
		foodSightNamedId: {ItemId: foodSightNamedId, Name: "Tea of Drinker", Type: items.Drink, Subtype: items.Edible, Uses: 2},
	}))
}

func requireFoodLineAtShapes(t *testing.T, want string) {
	t.Helper()
	requireOneLine(t, "infrared observer", drinkSightPlain(events.DrainQueuedMessagesForTest(drinkSightInfraId)), want)
	if got := events.DrainQueuedMessagesForTest(drinkSightBlindId); len(got) != 0 {
		t.Errorf("an observer who cannot see got %q", got)
	}
}

func TestFoodRoomLine_PlayerHiddenFromShapes(t *testing.T) {
	drinker, room := drinkSightScene(t, "cave")
	seedFoodSightItems(t)
	drinker.Character.Items = []items.Item{items.New(foodSightTeaId)}

	ConsumeFood(NewUserActorInRoom(drinker, room), `drink`, "test tea")

	requireFoodLineAtShapes(t, "A figure drinks some Test Tea.")
	for _, l := range drinkSightPlain(events.DrainQueuedMessagesForTest(drinkSightDrinkerId)) {
		if strings.Contains(l, "drinks") {
			t.Errorf("the eater read their own room line %q", l)
		}
	}
}

func TestFoodRoomLine_MobHiddenFromShapes(t *testing.T) {
	_, room := drinkSightScene(t, "cave")
	seedFoodSightItems(t)
	mob := drinkSightMob(room)
	mob.GetCharacter().Items = []items.Item{items.New(foodSightBreadId)}

	ConsumeFood(mob, `eat`, "test bread")

	requireFoodLineAtShapes(t, "A figure eats some Test Bread.")
}

func TestFoodRoomLine_PlayerNameInTheItemHiddenFromShapes(t *testing.T) {
	drinker, room := drinkSightScene(t, "cave")
	seedFoodSightItems(t)
	drinker.Character.Items = []items.Item{items.New(foodSightNamedId)}

	ConsumeFood(NewUserActorInRoom(drinker, room), `drink`, "tea of drinker")

	requireFoodLineAtShapes(t, "A figure drinks some Tea of a figure.")
}

func TestFoodRoomLine_MobNameInTheItemHiddenFromShapes(t *testing.T) {
	_, room := drinkSightScene(t, "cave")
	seedFoodSightItems(t)
	mob := drinkSightMob(room)
	mob.GetCharacter().Items = []items.Item{items.New(foodSightNamedId)}

	ConsumeFood(mob, `drink`, "tea of drinker")

	requireFoodLineAtShapes(t, "A figure drinks some Tea of a figure.")
}

// The control: in a lit room both observers read the name, so the hiding above
// is the sight tier and not a line that never names anyone.
func TestFoodRoomLine_LitObserversReadTheName(t *testing.T) {
	_, room := drinkSightScene(t, "city")
	seedFoodSightItems(t)
	mob := drinkSightMob(room)
	mob.GetCharacter().Items = []items.Item{items.New(foodSightTeaId)}

	ConsumeFood(mob, `drink`, "test tea")

	for _, id := range []int{drinkSightInfraId, drinkSightBlindId} {
		requireOneLine(t, "lit observer", drinkSightPlain(events.DrainQueuedMessagesForTest(id)),
			"Drinker drinks some Test Tea.")
	}
}
