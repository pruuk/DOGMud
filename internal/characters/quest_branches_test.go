package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// seedBranchedQuest11 seeds quest 11's shape (#958): one fork on the branch
// flag, two steps a side.
func seedBranchedQuest11(t *testing.T) {
	t.Helper()
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{
		QuestId: 11, Name: "The Windwarden's Dilemma",
		Flags: []quests.QuestFlagDef{{Key: "branch", Values: []string{"sylara", "rhett"}}},
		Steps: []quests.QuestStep{
			{Id: "start"},
			{Id: "sylara", Branch: "sylara"},
			{Id: "totem", Branch: "sylara"},
			{Id: "rhett", Branch: "rhett"},
			{Id: "samples", Branch: "rhett"},
			{Id: "end"},
		},
	}))
}

func rhettSider(step string) *Character {
	return &Character{
		QuestProgress: map[int]string{11: step},
		QuestFlags:    map[string]string{"11-branch": "rhett"},
	}
}

// A step off the player's path is never held: a Rhett player does not hold
// Sylara's steps, though they come earlier in the file.
func TestHasQuest_BranchedPath(t *testing.T) {
	seedBranchedQuest11(t)

	c := rhettSider("rhett")
	require.True(t, c.HasQuest("11-start"))
	require.True(t, c.HasQuest("11-rhett"))
	require.False(t, c.HasQuest("11-sylara"), "Sylara's fetch step is off a Rhett player's path")
	require.False(t, c.HasQuest("11-totem"), "Sylara's return step is off a Rhett player's path")
	require.False(t, c.HasQuest("11-samples"), "a later step is not held yet")

	c = rhettSider("end")
	require.True(t, c.HasQuest("11-samples"))
	require.False(t, c.HasQuest("11-totem"), "a finished Rhett player never held Sylara's steps")
}

// A grant off the player's path is refused, and a grant on it moves forward.
func TestGiveQuestToken_BranchedPath(t *testing.T) {
	seedBranchedQuest11(t)

	c := rhettSider("start")
	require.False(t, c.GiveQuestToken("11-sylara"), "a Rhett player cannot be granted Sylara's step")
	require.Equal(t, "start", c.QuestProgress[11])
	require.True(t, c.GiveQuestToken("11-rhett"))
	require.False(t, c.GiveQuestToken("11-totem"))
	require.True(t, c.GiveQuestToken("11-samples"))
	require.True(t, c.GiveQuestToken("11-end"))

	unsided := &Character{QuestProgress: map[int]string{11: "start"}}
	require.False(t, unsided.GiveQuestToken("11-rhett"), "no side chosen: no side's step")
	require.False(t, unsided.GiveQuestToken("11-sylara"), "no side chosen: no side's step")
}

// A save whose flag disagrees with its branched step keeps moving on its
// step's side, and LoadUser can name it.
func TestBranchedPath_FlagDisagreesWithStep(t *testing.T) {
	seedBranchedQuest11(t)

	c := &Character{
		QuestProgress: map[int]string{11: "rhett"},
		QuestFlags:    map[string]string{"11-branch": "sylara"},
	}
	require.Equal(t, []string{"11-rhett (flag 11-branch=sylara)"}, c.QuestBranchMismatches())
	require.True(t, c.HasQuest("11-rhett"))
	require.False(t, c.HasQuest("11-sylara"))
	require.False(t, c.GiveQuestToken("11-totem"), "the flag does not move it across")
	require.True(t, c.GiveQuestToken("11-samples"), "it keeps moving on its step's side")

	require.Empty(t, rhettSider("rhett").QuestBranchMismatches(), "an agreeing flag is no mismatch")
	require.Empty(t, rhettSider("start").QuestBranchMismatches(), "an unbranched step has no side to disagree with")
	require.Empty(t, (&Character{QuestProgress: map[int]string{11: "rhett"}}).QuestBranchMismatches(), "an unset flag is no mismatch")
}
