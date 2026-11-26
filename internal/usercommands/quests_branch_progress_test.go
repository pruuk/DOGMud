package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func branchedQuestForLog() *quests.Quest {
	return &quests.Quest{
		QuestId: 900058, Name: "Forked",
		Flags: []quests.QuestFlagDef{{Key: "branch", Values: []string{"sylara", "rhett"}}},
		Steps: []quests.QuestStep{
			{Id: "start", Description: "Choose a side."},
			{Id: "sylara", Description: "Fetch the totem.", Branch: "sylara"},
			{Id: "totem", Description: "Bring the totem back.", Branch: "sylara"},
			{Id: "rhett", Description: "Fetch the samples.", Branch: "rhett"},
			{Id: "samples", Description: "Bring the samples back.", Branch: "rhett"},
			{Id: "end", Description: "Done."},
		},
	}
}

// #958: the quest log counts the player's own side only. A Rhett player at
// rhett is on step 2 of 4 (start, rhett, samples, end), not 4 of 6; an
// unsided player at start is on 1 of 2.
func TestQuestLog_BranchedProgressCountsOwnSide(t *testing.T) {
	useDogmudTemplates(t)
	t.Cleanup(questengine.ResetEngineForTest())
	q := branchedQuestForLog()
	questengine.GetEngine().RegisterQuest(q)

	u := users.NewTestUser(9764, "questfork", "Questfork", 97640)
	u.Character.QuestFlags = map[string]string{"900058-branch": "rhett"}

	rec, completion := activeQuestRecord(u, q, "rhett")
	require.Equal(t, 0.5, completion, "step 2 of 4")
	require.Equal(t, "50%", rec.Completion)
	require.Equal(t, "Fetch the samples.", rec.Description)

	u.Character.QuestFlags = nil
	_, completion = activeQuestRecord(u, q, "start")
	require.Equal(t, 0.5, completion, "step 1 of 2")

	_, completion = activeQuestRecord(u, q, "all+")
	require.Equal(t, 0.0, completion, "the admin all+ row keeps its own count")
}
