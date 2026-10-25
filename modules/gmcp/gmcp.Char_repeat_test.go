package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

func TestFinishedRepeatQuests(t *testing.T) {
	t.Cleanup(util.ResetRoundCountForTest)
	qs := map[int]*quests.Quest{
		53: {QuestId: 53, Name: "Cull the Wash", Repeatable: true, CooldownRounds: 300},
		54: {QuestId: 54, Name: "Back to the Forge", Repeatable: true, CooldownRounds: 300, RepeatLimit: 1},
		55: {QuestId: 55, Name: "Another Brew", Repeatable: true, CooldownRounds: 300},
		56: {QuestId: 56, Name: "Hidden", Repeatable: true, Secret: true, CooldownRounds: 300},
	}
	lookup := func(id int) *quests.Quest { return qs[id] }
	util.SetRoundCountForTest(1000)
	c := &characters.Character{
		QuestCompletions: map[int]int{53: 1, 54: 1, 55: 2, 56: 1},
		QuestProgress:    map[int]string{55: "start"}, // in progress again: an ordinary row
	}
	c.SetQuestCooldown(53, 300)

	rows := finishedRepeatQuests(c, lookup)
	require.Equal(t, []GMCPCharModule_Payload_Quest{
		{Id: 53, Name: "Cull the Wash", Repeatable: true, Status: questStatusWaiting},
	}, rows, "54 is done for good, 55 is in progress, 56 is secret")

	util.SetRoundCountForTest(1300)
	require.Equal(t, questStatusReady, finishedRepeatQuests(c, lookup)[0].Status)
}
