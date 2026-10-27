package questloop

import "github.com/GoMudEngine/GoMud/internal/quests"

// ruleTalkOnQuest: a dialogue topic grants only a quest's first step (an
// offer) or its final step (a hand-in). A topic granting any step between
// is a talk step the quest file does not own; the quest takes it with a
// topic_heard trigger on that topic instead (#375 phase 3b slice 1), so the
// whole chain of steps reads from the quest file. A revoke ("-" token) is
// not a grant and is not indexed.
func ruleTalkOnQuest(q *quests.Quest, g grants) []Finding {
	var out []Finding
	for i := 1; i < len(q.Steps)-1; i++ {
		for _, w := range g[token(q, i)] {
			if w.topic == nil {
				continue
			}
			out = append(out, Finding{Rule: "talk-on-quest", QuestId: q.QuestId, Where: w.where,
				Msg: "a topic grants a mid-quest step; give the quest a topic_heard trigger for it instead"})
		}
	}
	return out
}
