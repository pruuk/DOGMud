package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func shelfQuest(t *testing.T) {
	t.Helper()
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{
		QuestId: 42, Name: "The Hunt",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "track"}, {Id: "hunt"}, {Id: "end"}},
	}))
}

// Abandoning moves the step to the shelf: the quest is not held, its flags
// and counts stay, and focus leaves it (spec section 1, owner ruling 1).
func TestShelveQuest_MovesTheStep(t *testing.T) {
	shelfQuest(t)
	c := &Character{QuestProgress: map[int]string{42: "track"}, LastQuestId: 42,
		QuestFlags: map[string]string{"42-x": "y"}}
	c.BumpQuestStepCount(quests.CountKey(42, "track", 0))

	require.True(t, c.ShelveQuest(42))
	require.False(t, c.HasQuest("42-start"))
	_, held := c.GetQuestProgress()[42]
	require.False(t, held)
	require.True(t, c.IsShelved(42))
	require.Equal(t, "y", c.GetQuestFlag("42-x"))
	require.Equal(t, 1, c.QuestStepCount(quests.CountKey(42, "track", 0)))
	require.Equal(t, 0, c.LastQuestId)

	require.False(t, c.ShelveQuest(42), "not held any more")
	require.False(t, c.ShelveQuest(7), "never held")
}

func TestShelveQuest_RefusesAFinishedQuest(t *testing.T) {
	shelfQuest(t)
	c := &Character{QuestProgress: map[int]string{42: "end"}}
	require.False(t, c.ShelveQuest(42))
	require.False(t, c.IsShelved(42))
}

// Any grant of N-start resumes the shelved step and keeps the counts.
func TestGiveQuestToken_ResumesTheShelf(t *testing.T) {
	shelfQuest(t)
	c := &Character{QuestProgress: map[int]string{42: "track"}}
	c.BumpQuestStepCount(quests.CountKey(42, "track", 0))
	require.True(t, c.ShelveQuest(42))

	require.True(t, c.GiveQuestToken("42-start"))
	require.Equal(t, "track", c.GetQuestProgress()[42])
	require.False(t, c.IsShelved(42))
	require.Equal(t, 42, c.LastQuestId)
	require.Equal(t, 1, c.QuestStepCount(quests.CountKey(42, "track", 0)), "a resume keeps counts")
	require.True(t, c.TakeResumed(42))
	require.False(t, c.TakeResumed(42), "taken once")
}

// A grant of a later token while shelved is not a resume: the quest is not
// held, and only an accept (N-start) takes it back up.
func TestGiveQuestToken_LaterTokenWhileShelvedDoesNothing(t *testing.T) {
	shelfQuest(t)
	c := &Character{QuestProgress: map[int]string{42: "track"}}
	require.True(t, c.ShelveQuest(42))
	require.False(t, c.GiveQuestToken("42-hunt"))
	require.True(t, c.IsShelved(42))
	_, held := c.GetQuestProgress()[42]
	require.False(t, held)
}

func TestShelvedQuests_SurviveYAMLRoundTrip(t *testing.T) {
	c := &Character{Name: "Mira", ShelvedQuests: map[int]string{42: "track"}}
	out, err := yaml.Marshal(c)
	require.NoError(t, err)
	require.Contains(t, string(out), "shelvedquests:")
	var back Character
	require.NoError(t, yaml.Unmarshal(out, &back))
	require.True(t, back.IsShelved(42))

	out, err = yaml.Marshal(&Character{Name: "Mira"})
	require.NoError(t, err)
	require.NotContains(t, string(out), "shelvedquests")
}
