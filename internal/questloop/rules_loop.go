package questloop

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// ruleStart: the first step's token is granted by at least one offer topic
// and by nothing else (an external grant included: see way.external).
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
// and by nothing else (an external grant included). A one-step quest's only token is also its start, so
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

// HintMaxRunes caps a step hint (owner ruling P8, 2026-10-24): a hint says
// where, then the one action; the minimap and the quest panel's "head <dir>"
// line carry the route. Counted in runes after whitespace runs collapse to
// one space. The web quest panel uses the same number to offer "more".
const HintMaxRunes = 200

// ruleHintLength: no step's hint runs past HintMaxRunes (P8). A step with
// no hint is the hint rule's finding, not this one's.
func ruleHintLength(q *quests.Quest) []Finding {
	var out []Finding
	for i := range q.Steps {
		hint := strings.Join(strings.Fields(q.Steps[i].Hint), " ")
		if n := utf8.RuneCountInString(hint); n > HintMaxRunes {
			out = append(out, Finding{Rule: "hint-length", QuestId: q.QuestId, Where: stepWhere(q, i),
				Msg: fmt.Sprintf("this hint is %d characters; the cap is %d (say where, then the one action)", n, HintMaxRunes)})
		}
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
