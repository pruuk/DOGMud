package usercommands

import (
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// Plan 7, owner P4: the Quests panel's Completed tab.
func completedFixture() (map[int]*quests.Quest, *characters.Character) {
	qs := map[int]*quests.Quest{
		10: {QuestId: 10, Name: "One Off"},
		11: {QuestId: 11, Name: "Secret One Off", Secret: true},
		12: {QuestId: 12, Name: "Capped", Repeatable: true, RepeatLimit: 1},
		13: {QuestId: 13, Name: "Again", Repeatable: true},
		14: {QuestId: 14, Name: "Working"},
		15: {QuestId: 15, Name: "Shelved Capped", Repeatable: true, RepeatLimit: 1},
		16: {QuestId: 16, Name: "Secret Capped", Repeatable: true, RepeatLimit: 1, Secret: true},
	}
	c := &characters.Character{
		QuestProgress:    map[int]string{10: "end", 11: "end", 14: "start"},
		QuestCompletions: map[int]int{12: 1, 13: 1, 15: 1, 16: 1},
		ShelvedQuests:    map[int]string{15: "start"},
	}
	return qs, c
}

func TestCompletedQuestIds(t *testing.T) {
	qs, c := completedFixture()
	lookup := func(id int) *quests.Quest { return qs[id] }
	require.Equal(t, []int{10, 12}, CompletedQuestIds(c, lookup),
		"a finished one-off and a capped repeatable; not secret, not shelved, not one that can be taken again, not one in progress")
}

// quests all and the Completed tab agree on what is finished for good: the
// one-offs at their end step plus the repeatables quests all marks done.
func TestCompletedQuestIds_MatchesQuestsAll(t *testing.T) {
	qs, c := completedFixture()
	var list []*quests.Quest
	for _, q := range qs {
		list = append(list, q)
	}
	t.Cleanup(quests.SeedQuestsForTest(list...))

	var fromQuestsAll []int
	for id, step := range c.GetQuestProgress() {
		if q := quests.GetQuestById(id); step == "end" && q != nil && !q.Secret && !c.IsShelved(id) {
			fromQuestsAll = append(fromQuestsAll, id)
		}
	}
	for _, row := range finishedRepeatRows(c, c.GetQuestProgress(), false) {
		if row.Description == repeatDoneLine {
			fromQuestsAll = append(fromQuestsAll, row.Id)
		}
	}
	sort.Ints(fromQuestsAll)
	require.Equal(t, fromQuestsAll, CompletedQuestIds(c, quests.GetQuestById))
}
