package questloop

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// ruleStart: the first step's token is granted by at least one offer topic
// and by nothing else.
func ruleStart(q *quests.Quest, g grants) []Finding {
	tok := token(q, 0)
	var out []Finding
	offers := 0
	for _, w := range g[tok] {
		if w.topic != nil && w.topic.Kind == dialogue.KindOffer {
			offers++
			continue
		}
		out = append(out, Finding{Rule: "start", QuestId: q.QuestId, Where: w.where,
			Msg: tok + " is granted here; only an NPC offer may start a quest"})
	}
	if offers == 0 {
		out = append(out, Finding{Rule: "start", QuestId: q.QuestId, Where: stepWhere(q, 0),
			Msg: "no offer topic grants " + tok})
	}
	return out
}

// ruleEnd: the final step's token is granted by at least one hand-in topic
// and by nothing else. A one-step quest's only token is also its start, so
// it cannot pass both start and end: intended (spec 5.2).
func ruleEnd(q *quests.Quest, g grants) []Finding {
	last := len(q.Steps) - 1
	tok := token(q, last)
	var out []Finding
	handins := 0
	for _, w := range g[tok] {
		if w.topic != nil && w.topic.Kind == dialogue.KindHandIn {
			handins++
			continue
		}
		out = append(out, Finding{Rule: "end", QuestId: q.QuestId, Where: w.where,
			Msg: tok + " is granted here; only an NPC hand-in may finish a quest"})
	}
	if handins == 0 {
		out = append(out, Finding{Rule: "end", QuestId: q.QuestId, Where: stepWhere(q, last),
			Msg: "no hand-in topic grants " + tok})
	}
	return out
}

// ruleHint: every step a player can stand on (all but the final one) has a hint.
func ruleHint(q *quests.Quest) []Finding {
	var out []Finding
	for i := 0; i < len(q.Steps)-1; i++ {
		if strings.TrimSpace(q.Steps[i].Hint) == "" {
			out = append(out, Finding{Rule: "hint", QuestId: q.QuestId, Where: stepWhere(q, i), Msg: "this step has no hint"})
		}
	}
	return out
}

// Stubs: Tasks 12 and 13 replace these with the real rules.
func ruleRepeat(*quests.Quest) []Finding                        { return nil }
func ruleReward(*quests.Quest, quests.RewardNames) []Finding    { return nil }
func ruleRefs(*quests.Quest, *quests.QuestValidators) []Finding { return nil }
func ruleChain(World) []Finding                                 { return nil }
