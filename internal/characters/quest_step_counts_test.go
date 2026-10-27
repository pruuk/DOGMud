package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestQuestStepCount_Bumps(t *testing.T) {
	c := &Character{}
	require.Equal(t, 0, c.QuestStepCount("53-hunt"))
	require.Equal(t, 1, c.BumpQuestStepCount("53-hunt"))
	require.Equal(t, 2, c.BumpQuestStepCount("53-hunt"))
	require.Equal(t, 3, c.BumpQuestStepCount("53-hunt"))
	require.Equal(t, 3, c.QuestStepCount("53-hunt"))
}

// Moving a quest's progress clears every count of that quest, and only that
// quest: a count belongs to the step the player was on.
func TestQuestStepCount_ClearedOnProgress(t *testing.T) {
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{
		QuestId: 53, Name: "Hunt",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "hunt"}, {Id: "end"}},
	}))
	c := &Character{QuestProgress: map[int]string{53: "hunt"}}
	// Keys are per trigger (quests.CountKey): every trigger's count goes.
	c.BumpQuestStepCount(quests.CountKey(53, "hunt", 0))
	c.BumpQuestStepCount(quests.CountKey(53, "hunt", 1))
	c.BumpQuestStepCount(quests.CountKey(530, "hunt", 0))
	require.False(t, c.GiveQuestToken("53-start"), "a step behind is no progress")
	require.Equal(t, 1, c.QuestStepCount("53-hunt-t0"), "no progress, no clear")
	require.True(t, c.GiveQuestToken("53-end"))
	require.Equal(t, 0, c.QuestStepCount("53-hunt-t0"))
	require.Equal(t, 0, c.QuestStepCount("53-hunt-t1"))
	require.Equal(t, 1, c.QuestStepCount("530-hunt-t0"), "another quest's count stays")

	c.QuestProgress[530] = "hunt"
	c.ClearQuestToken("530-hunt")
	require.Equal(t, 0, c.QuestStepCount("530-hunt-t0"), "clearing a quest clears its counts")
	require.Nil(t, c.QuestStepCounts, "an emptied map is dropped")
}

func TestQuestStepCount_SurvivesYAMLRoundTrip(t *testing.T) {
	c := &Character{Name: "Mira"}
	c.BumpQuestStepCount("53-hunt")
	c.BumpQuestStepCount("53-hunt")
	out, err := yaml.Marshal(c)
	require.NoError(t, err)
	require.Contains(t, string(out), "queststepcounts:")
	var back Character
	require.NoError(t, yaml.Unmarshal(out, &back))
	require.Equal(t, 2, back.QuestStepCount("53-hunt"))

	out, err = yaml.Marshal(&Character{Name: "Mira"})
	require.NoError(t, err)
	require.NotContains(t, string(out), "queststepcounts")
}
