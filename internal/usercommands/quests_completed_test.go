package usercommands

import (
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Plan 7, owner P4: the Quests panel's Completed tab.
func completedFixture() (map[int]*quests.Quest, *characters.Character) {
	// Two steps each, so `quests all` (activeQuestRecord) can compute a completion.
	steps := []quests.QuestStep{{Id: "start", Description: "Begin."}, {Id: "end", Description: "Done."}}
	qs := map[int]*quests.Quest{
		10: {QuestId: 10, Name: "One Off", Steps: steps},
		11: {QuestId: 11, Name: "Secret One Off", Secret: true, Steps: steps},
		12: {QuestId: 12, Name: "Capped", Repeatable: true, RepeatLimit: 1, Steps: steps},
		13: {QuestId: 13, Name: "Again", Repeatable: true, Steps: steps},
		14: {QuestId: 14, Name: "Working", Steps: steps},
		15: {QuestId: 15, Name: "Shelved Capped", Repeatable: true, RepeatLimit: 1, Steps: steps},
		16: {QuestId: 16, Name: "Secret Capped", Repeatable: true, RepeatLimit: 1, Secret: true, Steps: steps},
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

// quests all and the Completed tab agree on what is finished for good. The
// one-off half is read from what `quests all` computes for each row, as
// Quests() loops: not secret (plain `all`), and activeQuestRecord's completion
// at 1. The repeatable half is the rows it marks repeatDoneLine.
func TestCompletedQuestIds_MatchesQuestsAll(t *testing.T) {
	qs, c := completedFixture()
	var list []*quests.Quest
	for _, q := range qs {
		list = append(list, q)
	}
	t.Cleanup(quests.SeedQuestsForTest(list...))

	user := users.NewTestUser(1, "completed", "Completed", 1)
	user.Character = c

	var fromQuestsAll []int
	for questId, questStep := range c.GetQuestProgress() {
		questInfo := quests.GetQuest(quests.PartsToToken(questId, questStep))
		if questInfo == nil || questInfo.Secret {
			continue
		}
		if _, completion := activeQuestRecord(user, questInfo, questStep); completion >= 1 {
			fromQuestsAll = append(fromQuestsAll, questId)
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
