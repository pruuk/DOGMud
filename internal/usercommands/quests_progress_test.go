package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func countQuestForLog() *quests.Quest {
	return &quests.Quest{
		QuestId: 900054, Name: "Root Gathering",
		Steps: []quests.QuestStep{
			{Id: "start", Description: "Dig up roots in the grove."},
			{Id: "end", Description: "Done."},
		},
		Triggers: []quests.TriggerDef{{
			Event: "item_gain", Item: 9054, Count: 5, Label: "Roots gathered",
			Conditions: quests.Conditions{Has: []string{"900054-start"}, Missing: []string{"900054-end"}},
			Actions:    []quests.ActionDef{{Grant: "900054-end"}},
		}},
	}
}

// #375 phase 3b slice 4: a step reached by a count trigger shows its
// progress under the description in the quest log.
func TestQuestLog_ShowsCountProgress(t *testing.T) {
	useDogmudTemplates(t)
	t.Cleanup(questengine.ResetEngineForTest())
	q := countQuestForLog()
	questengine.GetEngine().RegisterQuest(q)

	u := users.NewTestUser(9762, "questlog", "Questlog", 97620)
	u.Character.QuestStepCounts = map[string]int{"900054-start-t0": 2}

	rec, _ := activeQuestRecord(u, q, "start")
	assert.Equal(t, "Dig up roots in the grove.", rec.Description)
	assert.Equal(t, "Roots gathered: 2 of 5", rec.Progress)

	out, err := templates.Process("character/quests", map[string]any{
		"QuestsFound": 1, "QuestsTotal": 1, "Records": []questRecord{rec},
	}, 0)
	require.NoError(t, err)
	assert.Contains(t, out, "Roots gathered: 2 of 5")

	rec, _ = activeQuestRecord(u, q, "end")
	assert.Empty(t, rec.Progress, "a step no count trigger leads from has no progress line")
}
