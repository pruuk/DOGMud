package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/stretchr/testify/require"
)

const eatProbeMealId = 999964

// Eating raises the eat command event, naming the food (#375 phase 7): the
// Hunt (quest 42) asks the player to eat the meal they cooked, and a step can
// only listen for an event the command raises. eat and drink are one command,
// so drinking the same food raises the same event.
func TestEat_RaisesTheEatCommandEvent(t *testing.T) {
	for _, verb := range []string{"eat", "drink"} {
		t.Run(verb, func(t *testing.T) {
			t.Cleanup(seedAllRegistries())
			t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
				eatProbeMealId: {ItemId: eatProbeMealId, Name: "Probe Stew", NameSimple: "stew",
					Type: items.Food, Subtype: items.Edible, Uses: 1},
			}))
			user, room := getTestUserAndRoom(t)
			orig := user.Character.Items
			t.Cleanup(func() { user.Character.Items = orig })
			user.Character.Items = []items.Item{items.New(eatProbeMealId)}
			events.DrainQueuedMessagesForTest(user.UserId)

			restore := questengine.ResetEngineForTest()
			t.Cleanup(restore)
			questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
				QuestId: 999303,
				Name:    "eat probe",
				Steps:   []questengine.QuestStep{{Id: "start"}},
				Triggers: []questengine.TriggerDef{{
					Event:   "command",
					Command: "eat",
					Item:    eatProbeMealId,
					Actions: []questengine.ActionDef{{Grant: "999303-start"}},
				}},
			})
			t.Cleanup(func() { user.Character.ClearQuestToken("999303-start") })

			if verb == "eat" {
				Eat("stew", user, room, 0)
			} else {
				Drink("stew", user, room, 0)
			}

			require.Contains(t, sentTo(user), "You eat some of the", "the meal must actually be eaten for the probe to mean anything")
			require.True(t, user.Character.HasQuest("999303-start"), "eating must raise the eat command event naming the food")
		})
	}
}
