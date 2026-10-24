package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestQuestOpen_NonRepeatableAlwaysOpen(t *testing.T) {
	c := &Character{}
	require.True(t, c.QuestOpen(&quests.Quest{QuestId: 9}))
}

func TestQuestOpen_CooldownAndCap(t *testing.T) {
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(1000)
	q := &quests.Quest{QuestId: 53, Repeatable: true, CooldownRounds: 300, RepeatLimit: 2}
	c := &Character{}
	require.True(t, c.QuestOpen(q))

	c.RecordCompletion(q)
	require.Equal(t, 1, c.QuestCompletions[53])
	require.False(t, c.QuestOpen(q), "on cooldown")

	util.SetRoundCountForTest(1300)
	require.True(t, c.QuestOpen(q), "cooldown over, one run left")
	require.True(t, c.LastRepeat(q), "the next run is the last")

	c.RecordCompletion(q)
	util.SetRoundCountForTest(5000)
	require.False(t, c.QuestOpen(q), "at the cap")
	require.True(t, c.RepeatCapReached(q))
}

func TestQuestOpen_NoLimitNeverCaps(t *testing.T) {
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(1000)
	q := &quests.Quest{QuestId: 54, Repeatable: true, CooldownRounds: 1}
	c := &Character{}
	for i := 0; i < 50; i++ {
		c.RecordCompletion(q)
	}
	util.SetRoundCountForTest(2000)
	require.True(t, c.QuestOpen(q))
	require.False(t, c.LastRepeat(q))
}

// The tests above set the round count below util.RoundCountMinimum; each
// must put it back so a later test in the package does not run on a clock
// the game can never show.
func TestQuestOpen_TestsRestoreTheRoundCount(t *testing.T) {
	for name, fn := range map[string]func(*testing.T){
		"CooldownAndCap":   TestQuestOpen_CooldownAndCap,
		"NoLimitNeverCaps": TestQuestOpen_NoLimitNeverCaps,
	} {
		util.ResetRoundCountForTest()
		t.Run(name, fn)
		require.Equal(t, uint64(util.RoundCountMinimum), util.GetRoundCount(), "%s left the round count changed", name)
	}
}

func TestQuestCompletions_YAMLRoundTrip(t *testing.T) {
	c := &Character{QuestCompletions: map[int]int{53: 4}}
	out, err := yaml.Marshal(c)
	require.NoError(t, err)
	require.Contains(t, string(out), "questcompletions:")
	var back Character
	require.NoError(t, yaml.Unmarshal(out, &back))
	require.Equal(t, 4, back.QuestCompletions[53])
}

func TestRenameQuestSteps(t *testing.T) {
	c := &Character{QuestProgress: map[int]string{53: "firstkill", 7: "deliver"}}
	c.RenameQuestSteps(map[string]string{"53-firstkill": "53-scout", "53-scout": "53-scoutdown"})
	require.Equal(t, "scoutdown", c.QuestProgress[53], "chains are followed")
	require.Equal(t, "deliver", c.QuestProgress[7])
}
