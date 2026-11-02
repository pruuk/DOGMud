package questengine_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Hook 1 reads the item with the same reach as a carriesItem offer: anywhere on
// the player. HasItem stays backpack only (has_item and consume_item use it).
func TestGameBridge_CarriesItemReachesWornComponentAndBandolier(t *testing.T) {
	u := newBridgeUser(1)
	u.Character.Equipment.Offhand = items.Item{ItemId: 501}
	u.Character.ComponentItems = append(u.Character.ComponentItems, items.Item{ItemId: 502})
	u.Character.PotionItems = append(u.Character.PotionItems, items.Item{ItemId: 503})
	u.Character.Items = append(u.Character.Items, items.Item{ItemId: 504})
	b := questengine.NewGameBridge(u, 1)

	for _, id := range []int{501, 502, 503, 504} {
		assert.True(t, b.CarriesItem(id), "carries %d", id)
	}
	assert.False(t, b.CarriesItem(505))
	assert.True(t, b.HasItem(504))
	assert.False(t, b.HasItem(501), "HasItem is unchanged: backpack only")
}

func TestHeld_AWornItemAdvancesTheStepThroughTheBridge(t *testing.T) {
	mudlog.SetupLogger(nil, "", "", false)
	// HasQuest on a step past start reads step order from the legacy registry.
	defer quests.SeedQuestsForTest(&quests.Quest{QuestId: 341, Name: "Fetch",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "fetch"}, {Id: "end"}}})()
	u := newBridgeUser(1)
	u.Character.Equipment.Offhand = items.Item{ItemId: 77}
	u.Character.QuestProgress = map[int]string{341: "start"}
	b := questengine.NewGameBridge(u, 1)
	require.False(t, b.HasItem(77), "precondition: not in the backpack")

	e := questengine.NewEngine()
	e.RegisterQuest(&questengine.QuestDef{
		QuestId: 341, Name: "Fetch",
		Steps: []questengine.QuestStep{{Id: "start"}, {Id: "fetch"}, {Id: "end"}},
		Triggers: []questengine.TriggerDef{
			{Event: "command", Command: "begin",
				Conditions: questengine.Conditions{Has: []string{"341-start"}, Missing: []string{"341-fetch"}},
				Actions:    []questengine.ActionDef{{Grant: "341-fetch"}}},
			{Event: "item_gain", Item: 77,
				Conditions: questengine.Conditions{Has: []string{"341-fetch"}, Missing: []string{"341-end"}},
				Actions:    []questengine.ActionDef{{Grant: "341-end"}}},
		},
	})
	e.Notify("command", questengine.EventDetails{UserId: 1, Command: "begin", TargetKind: "none"}, b, b)

	assert.True(t, u.Character.HasQuest("341-end"), "the worn item satisfied the step as it started")
}
