package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func TestCompletedQuestRows(t *testing.T) {
	qs := map[int]*quests.Quest{
		10: {QuestId: 10, Name: "One Off"},
		12: {QuestId: 12, Name: "Capped", Repeatable: true, RepeatLimit: 1},
	}
	lookup := func(id int) *quests.Quest { return qs[id] }
	c := &characters.Character{
		QuestProgress:    map[int]string{10: "end"},
		QuestCompletions: map[int]int{12: 1},
	}
	require.Equal(t, []GMCPCharModule_Payload_Quest{
		{Id: 10, Name: "One Off", Status: questStatusDone},
		{Id: 12, Name: "Capped", Repeatable: true, Status: questStatusDone},
	}, completedQuestRows(c, lookup))
}
