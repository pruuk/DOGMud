package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #375 phase 3b slice 4: the tracker payload carries the progress line of a
// step reached by a count trigger, and none for a plain step.
func TestCharQuests_CarriesCountProgress(t *testing.T) {
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&quests.Quest{
		QuestId: 900055, Name: "Root Gathering",
		Steps: []quests.QuestStep{
			{Id: "start", Description: "Dig up roots."},
			{Id: "carry", Description: "Bring them back."},
			{Id: "end", Description: "Done."},
		},
		Triggers: []quests.TriggerDef{{
			Event: "item_gain", Item: 9055, Count: 4,
			Conditions: quests.Conditions{Has: []string{"900055-start"}, Missing: []string{"900055-carry"}},
			Actions:    []quests.ActionDef{{Grant: "900055-carry"}},
		}},
	})
	u := users.NewTestUser(9763, "tracker", "Tracker", 97630)
	u.Character.QuestProgress = map[int]string{900055: "start"}
	u.Character.QuestStepCounts = map[string]int{"900055-start": 3}

	g := &GMCPCharModule{}
	data, name := g.GetCharNode(u, `Char.Quests`)
	require.Equal(t, `Char.Quests`, name)
	rows := data.([]GMCPCharModule_Payload_Quest)
	require.Len(t, rows, 1)
	assert.Equal(t, "Dig up roots.", rows[0].Description)
	assert.Equal(t, "Progress: 3 of 4", rows[0].Progress)

	u.Character.QuestProgress[900055] = "carry"
	data, _ = g.GetCharNode(u, `Char.Quests`)
	assert.Empty(t, data.([]GMCPCharModule_Payload_Quest)[0].Progress)
}
