package questloop

import (
	"slices"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// ruleOneAction is spec 5.2's one-action rule (P4). One finding per broken
// way and reason, so each defect has its own key: the way's Where takes a
// suffix, :none (nothing moves a player onto the step), :reward (a
// completion reward grants it), :prev (the previous step is not required),
// :cond (an item, flag or gold condition), :seq (granted inside a sequence)
// or :double (one trigger grants two steps of its quest).
//
// A branched quest (#958) is checked one path at a time (quests.Paths): a
// way into a step must require the step before it on some path that holds
// the step, and lie on that path (requiresOnPath). Where the paths reach a
// step from different steps (the join after a fork), each of those steps
// needs a way from it, or "step <id> from <prev id>:none" names the side
// left with no way on. A quest with no branched steps has one path, so its
// findings are those of the straight-line rule.
func ruleOneAction(q *quests.Quest, g grants) []Finding {
	var out []Finding
	add := func(where, reason, msg string) {
		out = append(out, Finding{Rule: "one-action", QuestId: q.QuestId, Where: where + ":" + reason, Msg: msg})
	}
	paths := q.Paths()
	for i := 1; i < len(q.Steps); i++ {
		tok := token(q, i)
		ways := g[tok]
		if len(ways) == 0 {
			add(stepWhere(q, i), "none", "nothing moves a player onto "+tok)
			continue
		}
		lines := stepLines(q, paths, i)
		prevs := linePrevs(lines)
		covered := map[string]bool{}
		missing := "grants " + tok + " without requiring " + strings.Join(prevs, " or ")
		if len(prevs) > 1 {
			missing = "grants " + tok + " without requiring the step before it on a path it lies on (" + strings.Join(prevs, " or ") + ")"
		}
		for _, w := range ways {
			if w.reward {
				add(w.where, "reward", "a completion reward grants "+tok)
				continue
			}
			switch {
			case w.external != nil:
				// A tree grant or item questtoken: a way in with no
				// conditions to check (see way.external), from any side.
				for _, l := range lines {
					covered[l.prev] = true
				}
			case w.topic != nil:
				if !requiresPrev(q, w.topic.QuestRequired, lines, covered) {
					add(w.where, "prev", missing)
				}
			case w.trigger != nil:
				c := w.trigger.Conditions
				if !requiresPrev(q, c.Has, lines, covered) {
					add(w.where, "prev", missing)
				}
				if c.HasItem > 0 || c.MissingItem > 0 || len(c.MissingItems) > 0 || len(c.HasFlag) > 0 || len(c.MissingFlag) > 0 || c.HasGold > 0 {
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
		if len(prevs) < 2 {
			continue // one way on: every way above already answered for it
		}
		for _, prev := range prevs {
			if !covered[prev] {
				_, prevStep := quests.TokenToParts(prev)
				add(stepWhere(q, i)+" from "+prevStep, "none", "nothing moves a player on from "+prev+" onto "+tok+": that side has no way on")
			}
		}
	}
	return dedupe(out)
}

// stepLine is one path through step i, and the step before it there.
type stepLine struct {
	prev string // the token of the step before
	path []quests.QuestStep
}

// stepLines lists, for each path that holds step i, the step before it.
func stepLines(q *quests.Quest, paths [][]quests.QuestStep, i int) []stepLine {
	var out []stepLine
	id := q.Steps[i].Id
	for _, p := range paths {
		k := slices.IndexFunc(p, func(s quests.QuestStep) bool { return s.Id == id })
		if k > 0 {
			out = append(out, stepLine{prev: quests.PartsToToken(q.QuestId, p[k-1].Id), path: p})
		}
	}
	return out
}

// linePrevs is the distinct steps before, in path order.
func linePrevs(lines []stepLine) []string {
	var out []string
	for _, l := range lines {
		if !slices.Contains(out, l.prev) {
			out = append(out, l.prev)
		}
	}
	return out
}

// requiresPrev reports whether req (a way's has or quest_required) requires
// the step before on some line and lies on that line's path, marking each
// such step covered.
func requiresPrev(q *quests.Quest, req []string, lines []stepLine, covered map[string]bool) bool {
	ok := false
	for _, l := range lines {
		if slices.Contains(req, l.prev) && requiresOnPath(q, req, l.path) {
			covered[l.prev] = true
			ok = true
		}
	}
	return ok
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
