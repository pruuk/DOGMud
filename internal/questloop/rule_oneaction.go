package questloop

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// ruleOneAction is spec 5.2's one-action rule (P4). One finding per broken
// way and reason, so the backlog names each defect: the way's Where takes a
// suffix, :none (nothing moves a player onto the step), :reward (a
// completion reward grants it), :prev (the previous step is not required),
// :cond (an item, flag or gold condition), :seq (granted inside a sequence)
// or :double (one trigger grants two steps of its quest).
func ruleOneAction(q *quests.Quest, g grants) []Finding {
	var out []Finding
	add := func(where, reason, msg string) {
		out = append(out, Finding{Rule: "one-action", QuestId: q.QuestId, Where: where + ":" + reason, Msg: msg})
	}
	for i := 1; i < len(q.Steps); i++ {
		tok, prev := token(q, i), token(q, i-1)
		ways := g[tok]
		if len(ways) == 0 {
			add(stepWhere(q, i), "none", "nothing moves a player onto "+tok)
			continue
		}
		for _, w := range ways {
			if w.reward {
				add(w.where, "reward", "a completion reward grants "+tok)
				continue
			}
			switch {
			case w.topic != nil:
				if !slices.Contains(w.topic.QuestRequired, prev) {
					add(w.where, "prev", "grants "+tok+" without requiring "+prev)
				}
			case w.trigger != nil:
				c := w.trigger.Conditions
				if !slices.Contains(c.Has, prev) {
					add(w.where, "prev", "grants "+tok+" without requiring "+prev)
				}
				if c.HasItem > 0 || c.MissingItem > 0 || len(c.HasFlag) > 0 || len(c.MissingFlag) > 0 || c.HasGold > 0 {
					add(w.where, "cond", "an item, flag or gold condition hides a second action; make it its own step")
				}
				if w.nested {
					add(w.where, "seq", "grants "+tok+" inside a sequence; a step moves on one action")
				}
				if ownGrants(w.trigger, w.owner) > 1 {
					add(w.where, "double", "grants more than one step of its quest at once")
				}
			}
		}
	}
	return dedupe(out)
}

// dedupe drops repeated keys, keeping the first message. Different reasons
// have different keys; a repeat is the same defect reached twice, such as a
// trigger that grants one step in two of its actions (the step's way is
// listed once per grant) or a double grant reported from both steps.
func dedupe(fs []Finding) []Finding {
	seen := map[string]bool{}
	var out []Finding
	for _, f := range fs {
		if seen[f.Key()] {
			continue
		}
		seen[f.Key()] = true
		out = append(out, f)
	}
	return out
}
