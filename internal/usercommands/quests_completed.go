package usercommands

import (
	"sort"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// CompletedQuestIds are the quests c has finished for good, sorted, for the
// web client's Completed tab (plan 7, owner P4): a quest whose progress is at
// its end step, and a repeatable at its repeat cap, neither shelved. Secret
// quests are left out, as plain `quests all` leaves them out. A repeatable
// that can be taken again belongs to the Repeat tab (gmcp
// finishedRepeatQuests). TestCompletedQuestIds_MatchesQuestsAll holds this to
// `quests all`.
func CompletedQuestIds(c *characters.Character, lookup func(int) *quests.Quest) []int {
	seen := map[int]bool{}
	progress := c.GetQuestProgress()
	for id, step := range progress {
		if step != `end` || c.IsShelved(id) {
			continue
		}
		if q := lookup(id); q != nil && !q.Secret {
			seen[id] = true
		}
	}
	for id, n := range c.QuestCompletions {
		if n < 1 || c.IsShelved(id) {
			continue
		}
		if _, inProgress := progress[id]; inProgress {
			continue
		}
		q := lookup(id)
		if q == nil || q.Secret || !q.Repeatable || !c.RepeatCapReached(q) {
			continue
		}
		seen[id] = true
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}
