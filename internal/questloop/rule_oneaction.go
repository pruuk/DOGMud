package questloop

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// ruleOneAction is spec 5.2's one-action rule (P4). One finding per broken
// way, so the backlog names each.
func ruleOneAction(q *quests.Quest, g grants) []Finding {
	var out []Finding
	add := func(where, msg string) {
		out = append(out, Finding{Rule: "one-action", QuestId: q.QuestId, Where: where, Msg: msg})
	}
	for i := 1; i < len(q.Steps); i++ {
		tok, prev := token(q, i), token(q, i-1)
		ways := g[tok]
		if len(ways) == 0 {
			add(stepWhere(q, i), "nothing moves a player onto "+tok)
			continue
		}
		for _, w := range ways {
			if w.reward {
				add(w.where, "a completion reward grants "+tok)
				continue
			}
			switch {
			case w.topic != nil:
				if !slices.Contains(w.topic.QuestRequired, prev) {
					add(w.where, "grants "+tok+" without requiring "+prev)
				}
			case w.trigger != nil:
				c := w.trigger.Conditions
				if !slices.Contains(c.Has, prev) {
					add(w.where, "grants "+tok+" without requiring "+prev)
				}
				if c.HasItem > 0 || c.MissingItem > 0 || len(c.HasFlag) > 0 || len(c.MissingFlag) > 0 || c.HasGold > 0 {
					add(w.where, "an item, flag or gold condition hides a second action; make it its own step")
				}
				if w.nested {
					add(w.where, "grants "+tok+" inside a sequence; a step moves on one action")
				}
				if ownGrants(w.trigger, w.owner) > 1 {
					add(w.where, "grants more than one step of its quest at once")
				}
			}
		}
	}
	return dedupe(out)
}

// dedupe drops repeated keys (one trigger can break the rule two ways; the
// backlog needs it once), keeping the first message.
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
