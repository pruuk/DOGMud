package questloop

import (
	"fmt"
	"slices"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// ruleChain: an offer gated on quest N's final token sits on an NPC that
// takes N's hand-in (P1), so the hand-in can pitch it.
func ruleChain(w World) []Finding {
	finals := map[string]int{} // final token -> quest id
	for _, q := range w.Quests {
		if q != nil && len(q.Steps) > 0 {
			finals[token(q, len(q.Steps)-1)] = q.QuestId
		}
	}
	handinMobs := map[int][]int{} // quest id -> mobs with a hand-in for it
	for _, df := range w.Dialogue {
		if df == nil {
			continue
		}
		for _, t := range df.Topics {
			if t.Kind == dialogue.KindHandIn && t.Quest > 0 && !slices.Contains(handinMobs[t.Quest], df.MobId) {
				handinMobs[t.Quest] = append(handinMobs[t.Quest], df.MobId)
			}
		}
	}
	var out []Finding
	for _, df := range w.Dialogue {
		if df == nil {
			continue
		}
		for _, t := range df.Topics {
			if t.Kind != dialogue.KindOffer {
				continue
			}
			for _, req := range t.QuestRequired {
				prior, ok := finals[req]
				if !ok || prior == t.Quest || slices.Contains(handinMobs[prior], df.MobId) {
					continue
				}
				out = append(out, Finding{Rule: "chain", QuestId: prior,
					Where: fmt.Sprintf("mob %d topic %s", df.MobId, t.Key),
					Msg:   fmt.Sprintf("offers quest %d after quest %d, but this NPC does not take quest %d's hand-in; add a bridge quest to here", t.Quest, prior, prior)})
			}
		}
	}
	return out
}

// ruleRepeat: a repeatable has a cooldown; repeat_limit is not negative and
// only appears on a repeatable (P3).
func ruleRepeat(q *quests.Quest) []Finding {
	var out []Finding
	add := func(msg string) {
		out = append(out, Finding{Rule: "repeat", QuestId: q.QuestId, Where: fmt.Sprintf("quest %d", q.QuestId), Msg: msg})
	}
	if q.Repeatable && q.CooldownRounds <= 0 {
		add("a repeatable quest needs cooldown_rounds above zero")
	}
	if q.RepeatLimit < 0 {
		add("repeat_limit cannot be negative")
	}
	if q.RepeatLimit > 0 && !q.Repeatable {
		add("repeat_limit is set on a quest that does not repeat")
	}
	return dedupe(out)
}

// ruleReward: an authored summary names every item, recipe and spell the
// quest pays, unless the reward is a surprise; a surprise needs a summary (P2).
func ruleReward(q *quests.Quest, n quests.RewardNames) []Finding {
	r := q.Rewards
	where := fmt.Sprintf("quest %d rewards summary", q.QuestId)
	summary := strings.TrimSpace(r.Summary)
	if r.Surprise && summary == "" {
		return []Finding{{Rule: "reward", QuestId: q.QuestId, Where: where, Msg: "surprise is set but there is no summary to show"}}
	}
	if summary == "" || r.Surprise {
		return nil
	}
	low := strings.ToLower(summary)
	var missing []string
	for _, name := range quests.RewardPaidNames(q, n) {
		if !strings.Contains(low, strings.ToLower(name)) {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []Finding{{Rule: "reward", QuestId: q.QuestId, Where: where,
		Msg: "the summary does not name " + strings.Join(missing, ", ") + "; name them or set surprise"}}
}

// ruleRefs: ValidateQuestRefs' refusals, so its checks run at every door.
// Its warnings stay with the editor.
func ruleRefs(q *quests.Quest, v *quests.QuestValidators) []Finding {
	if v == nil {
		return nil
	}
	errs, _ := quests.ValidateQuestRefs(*q, *v)
	var out []Finding
	for _, e := range errs {
		out = append(out, Finding{Rule: "refs", QuestId: q.QuestId, Where: fmt.Sprintf("quest %d refs", q.QuestId), Msg: e})
	}
	return out
}
