package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// #958: the Quests panel counts the player's own side only, as the quest log
// does (quests.Quest.StepProgress). A Rhett player at rhett reads 2 of 4.
func TestCharQuests_BranchedCompletionCountsOwnSide(t *testing.T) {
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&quests.Quest{
		QuestId: 9958, Name: "Forked",
		Flags: []quests.QuestFlagDef{{Key: "branch", Values: []string{"sylara", "rhett"}}},
		Steps: []quests.QuestStep{
			{Id: "start"},
			{Id: "sylara", Branch: "sylara"},
			{Id: "totem", Branch: "sylara"},
			{Id: "rhett", Description: "Fetch the samples.", Hint: "Rhett's tent.", Branch: "rhett"},
			{Id: "samples", Branch: "rhett"},
			{Id: "end"},
		},
	})

	u := menuTestUser(t, 9833, 98833, true)
	u.Character.QuestProgress = map[int]string{9958: "rhett"}
	u.Character.QuestFlags = map[string]string{"9958-branch": "rhett"}

	data, _ := (&GMCPCharModule{}).GetCharNode(u, `Char.Quests`)
	rows := data.([]GMCPCharModule_Payload_Quest)
	require.Len(t, rows, 1)
	require.Equal(t, 50, rows[0].Completion, "step 2 of 4")
	require.Equal(t, "Fetch the samples.", rows[0].Description)
	require.Equal(t, "Rhett's tent.", rows[0].Hint)
}
