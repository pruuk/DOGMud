package quests

import (
	"fmt"
	"slices"
)

// Quest branches (#958). A step may carry `branch: <value>`, a value of one of
// the quest's declared flags; every branched step of a quest uses the same
// flag (one fork per quest), and the first and last steps are never branched.
//
// A player's path is the quest's steps in file order, keeping the unbranched
// steps and those whose branch is the player's side. The side is the branch of
// the player's current step when that step is branched, and only otherwise the
// value of the branch flag, so a save resting on a branched step stays on its
// side whatever its flag says. With the flag unset on an unbranched step the
// path is the unbranched steps only. A quest with no branched steps has one
// path, all of its steps, and behaves as quests always have.
//
// Every reader of step order (Character.HasQuest and GiveQuestToken through
// IsTokenPassedOnPath and IsTokenAfterOnPath, the trigger engine's off-path
// skip, the quest log through StepProgress) works from these helpers. The
// order checks run on indexes and build no slice, since HasQuest runs on
// every trigger check.
//
// flag arguments read a player's quest flags by registered key
// ("<questId>-<key>", see flagKey); nil reads every flag as unset.

// validateBranches refuses a branch that is not a value of exactly one
// declared flag, branched steps on more than one flag, a value of that flag
// no step branches on, a branched first or last step, and branches on a
// repeatable quest (a repeat clears tokens, not
// flags, so a second run would be locked to the first run's side).
func (r *Quest) validateBranches() error {
	flagIdx := -1
	for i, s := range r.Steps {
		if s.Branch == `` {
			continue
		}
		if r.Repeatable {
			return fmt.Errorf("step %q has branch %q, but a repeatable quest cannot branch (a repeat clears steps, not flags, so the next run would be locked to this run's side)", s.Id, s.Branch)
		}
		if i == 0 {
			return fmt.Errorf("first step %q cannot be branched: every side starts on it", s.Id)
		}
		if i == len(r.Steps)-1 {
			return fmt.Errorf("last step %q cannot be branched: every side ends on it", s.Id)
		}
		owner := -1
		for fi, f := range r.Flags {
			if !slices.Contains(f.Values, s.Branch) {
				continue
			}
			if owner >= 0 {
				return fmt.Errorf("step %q: branch %q is a value of more than one flag (%q and %q); a branch must name one flag's value", s.Id, s.Branch, r.Flags[owner].Key, f.Key)
			}
			owner = fi
		}
		if owner < 0 {
			return fmt.Errorf("step %q: branch %q is not a value of any of the quest's flags", s.Id, s.Branch)
		}
		if flagIdx >= 0 && owner != flagIdx {
			return fmt.Errorf("step %q: branched steps use flags %q and %q; a quest forks on one flag", s.Id, r.Flags[flagIdx].Key, r.Flags[owner].Key)
		}
		flagIdx = owner
	}
	// Every value of the branch flag is a side with its own steps: a value no
	// step uses would be a path of the unbranched steps alone, on which a
	// player stands at the first step with no way on.
	if flagIdx >= 0 {
		f := r.Flags[flagIdx]
		for _, v := range f.Values {
			if !slices.ContainsFunc(r.Steps, func(s QuestStep) bool { return s.Branch == v }) {
				return fmt.Errorf("value %q of branch flag %q is the branch of no step: every value of the flag the steps branch on must be a side with its own steps; record any other choice in a separate flag", v, f.Key)
			}
		}
	}
	return nil
}

// branchCache is what Validate works out once about a quest's fork, so the
// hot readers (HasQuest on every trigger check) build no string. A quest that
// never went through Validate (a test literal) works it out on each call, and
// so does one whose id or step count changed since (cached). An in-place edit
// that keeps both (a step id renamed, a step's Branch changed, Flags edited)
// is not seen: whoever edits Steps or Flags of a validated quest calls
// Validate again, which refills the cache. Checking more on every read would
// cost the hot readers.
type branchCache struct {
	ready   bool
	questId int      // QuestId when filled
	steps   int      // len(Steps) when filled
	flag    int      // index in Flags of the branch flag, -1 when unbranched
	key     string   // that flag's registered key, "<questId>-<key>"
	tokens  []string // each step's token, "<questId>-<stepId>", by step index
}

// cached reports whether the branch cache still describes the quest.
func (r *Quest) cached() bool {
	return r.branch.ready && r.branch.questId == r.QuestId && r.branch.steps == len(r.Steps)
}

// prepareBranches fills the branch cache. Validate calls it.
func (r *Quest) prepareBranches() {
	c := branchCache{ready: true, questId: r.QuestId, steps: len(r.Steps), flag: r.findBranchFlag()}
	if c.flag >= 0 {
		c.key = flagKey(r.QuestId, r.Flags[c.flag].Key)
	}
	c.tokens = make([]string, len(r.Steps))
	for i, s := range r.Steps {
		c.tokens[i] = PartsToToken(r.QuestId, s.Id)
	}
	r.branch = c
}

// branchFlag is the index in Flags of the flag the branched steps use, or -1
// for a quest with no branched steps.
func (r *Quest) branchFlag() int {
	if r.cached() {
		return r.branch.flag
	}
	return r.findBranchFlag()
}

// branchKey is the branch flag's registered key; fi is branchFlag().
func (r *Quest) branchKey(fi int) string {
	if r.cached() {
		return r.branch.key
	}
	return flagKey(r.QuestId, r.Flags[fi].Key)
}

// stepToken is step i's token.
func (r *Quest) stepToken(i int) string {
	if r.cached() {
		return r.branch.tokens[i]
	}
	return PartsToToken(r.QuestId, r.Steps[i].Id)
}

// findBranchFlag works out branchFlag from the steps and flags.
func (r *Quest) findBranchFlag() int {
	for _, s := range r.Steps {
		if s.Branch == `` {
			continue
		}
		for fi, f := range r.Flags {
			if slices.Contains(f.Values, s.Branch) {
				return fi
			}
		}
		return -1
	}
	return -1
}

// BranchFlagKey is the registered key ("<questId>-<key>") of the flag the
// quest's branched steps use, or blank for a quest with no branched steps.
func (r *Quest) BranchFlagKey() string {
	if fi := r.branchFlag(); fi >= 0 {
		return r.branchKey(fi)
	}
	return ``
}

// Step is the quest's step with id, and whether it has one.
func (r *Quest) Step(id string) (QuestStep, bool) {
	if i := r.stepIndex(id); i >= 0 {
		return r.Steps[i], true
	}
	return QuestStep{}, false
}

// stepIndex is the index of step id in Steps, or -1.
func (r *Quest) stepIndex(id string) int {
	for i, s := range r.Steps {
		if s.Id == id {
			return i
		}
	}
	return -1
}

// Side is the side of a player standing at stepId: the step's own branch when
// it has one, else the branch flag's value. It is blank for a quest with no
// branched steps, and for a player on an unbranched step with the flag unset.
func (r *Quest) Side(stepId string, flag func(string) string) string {
	fi := r.branchFlag()
	if fi < 0 {
		return ``
	}
	if i := r.stepIndex(stepId); i >= 0 && r.Steps[i].Branch != `` {
		return r.Steps[i].Branch
	}
	if flag == nil {
		return ``
	}
	return flag(r.branchKey(fi))
}

// GrantOnPath reports whether granting stepId keeps a player on their own
// path: true for an unbranched step (or one the quest lacks), and for a
// branched step of the player's side. It sees the player only through has
// (HasQuest on this quest's tokens) and flag, as the trigger engine does.
// Holding any branched step of the grant's side opens it; holding only the
// other side's shuts it; holding none, the flag decides. For a holder on the
// path helper (Character.HasQuest) this is Side; a holder that reads steps
// as one straight line, and so "holds" both sides' earlier steps, still gets
// its own side's next step.
func (r *Quest) GrantOnPath(stepId string, has func(string) bool, flag func(string) string) bool {
	i := r.stepIndex(stepId)
	if i < 0 || r.Steps[i].Branch == `` {
		return true
	}
	want := r.Steps[i].Branch
	heldOther := false
	for j, s := range r.Steps {
		if s.Branch == `` || !has(r.stepToken(j)) {
			continue
		}
		if s.Branch == want {
			return true
		}
		heldOther = true
	}
	if heldOther {
		return false
	}
	fi := r.branchFlag()
	if fi < 0 || flag == nil {
		return false
	}
	return flag(r.branchKey(fi)) == want
}

// onSide reports whether step i lies on side's path.
func (r *Quest) onSide(i int, side string) bool {
	b := r.Steps[i].Branch
	return b == `` || b == side
}

// stepBefore reports whether step a comes strictly before step b on the path
// of a player standing at step at. Both must exist and lie on that path.
func (r *Quest) stepBefore(at, a, b string, flag func(string) string) bool {
	ia, ib := r.stepIndex(a), r.stepIndex(b)
	if ia < 0 || ib <= ia {
		return false
	}
	side := r.Side(at, flag)
	return r.onSide(ia, side) && r.onSide(ib, side)
}

// IsStepAfterOnPath reports whether granting step next to a player standing
// at step cur moves them forward on their path, as GiveQuestToken asks. A
// blank cur is a player who has not begun the quest: they enter only at
// "start", or at the only step of a one-step quest. IsTokenAfterOnPath is the
// token form; a reader that holds the quest itself (the walkthrough's player)
// calls this one.
func (r *Quest) IsStepAfterOnPath(cur, next string, flag func(string) string) bool {
	if cur == `` {
		return next == `start` || (next == `end` && len(r.Steps) == 1)
	}
	return cur != next && r.stepBefore(cur, cur, next, flag)
}

// IsStepPassedOnPath reports whether a player standing at step cur has passed
// step on their path, as HasQuest asks of every step but the current one. The
// token form is IsTokenPassedOnPath.
func (r *Quest) IsStepPassedOnPath(step, cur string, flag func(string) string) bool {
	return step != cur && r.stepBefore(cur, step, cur, flag)
}

// Path is the steps of a player standing at stepId, in order. It builds a
// slice: hot paths use the order checks instead.
func (r *Quest) Path(stepId string, flag func(string) string) []QuestStep {
	return r.pathFor(r.Side(stepId, flag))
}

// pathFor is the steps on side's path, in order.
func (r *Quest) pathFor(side string) []QuestStep {
	out := make([]QuestStep, 0, len(r.Steps))
	for i, s := range r.Steps {
		if r.onSide(i, side) {
			out = append(out, s)
		}
	}
	return out
}

// Paths is every path through the quest: one per declared value of the branch
// flag, in the order the flag lists them, or the single path of all steps for
// a quest with no branched steps.
func (r *Quest) Paths() [][]QuestStep {
	sides := r.Sides()
	out := make([][]QuestStep, 0, len(sides))
	for _, v := range sides {
		out = append(out, r.pathFor(v))
	}
	return out
}

// Sides is the side of each path Paths returns, in the same order: the branch
// flag's declared values, or one blank side for a quest with no branched
// steps. A walk of path k ends with the branch flag set to Sides()[k].
func (r *Quest) Sides() []string {
	fi := r.branchFlag()
	if fi < 0 {
		return []string{``}
	}
	return slices.Clone(r.Flags[fi].Values)
}

// StepProgress is how far a player standing at stepId is along their path:
// done counts the path's steps up to and including stepId, total the path's
// length. A step the quest does not have reads as done (done == total), as the
// quest log always read it.
func (r *Quest) StepProgress(stepId string, flag func(string) string) (done, total int) {
	side := r.Side(stepId, flag)
	cur := r.stepIndex(stepId)
	for i := range r.Steps {
		if !r.onSide(i, side) {
			continue
		}
		total++
		if i <= cur {
			done++
		}
	}
	if cur < 0 {
		done = total
	}
	return done, total
}
