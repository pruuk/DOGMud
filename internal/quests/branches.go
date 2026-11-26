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
// declared flag, branched steps on more than one flag, a branched first or
// last step, and branches on a repeatable quest (a repeat clears tokens, not
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
	return nil
}

// branchFlag is the index in Flags of the flag the branched steps use, or -1
// for a quest with no branched steps.
func (r *Quest) branchFlag() int {
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
		return flagKey(r.QuestId, r.Flags[fi].Key)
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
	return flag(flagKey(r.QuestId, r.Flags[fi].Key))
}

// GrantOnPath reports whether granting stepId keeps a player on their own
// path: true for an unbranched step (or one the quest lacks), and for a
// branched step of the player's side. It sees the player only through has
// (HasQuest on this quest's tokens) and flag, as the trigger engine does, and
// reads the same side as Side: a held branched step is held only on its own
// side's path, so its branch is the side; holding none, the flag decides.
func (r *Quest) GrantOnPath(stepId string, has func(string) bool, flag func(string) string) bool {
	i := r.stepIndex(stepId)
	if i < 0 || r.Steps[i].Branch == `` {
		return true
	}
	for _, s := range r.Steps {
		if s.Branch != `` && has(PartsToToken(r.QuestId, s.Id)) {
			return s.Branch == r.Steps[i].Branch
		}
	}
	fi := r.branchFlag()
	if fi < 0 || flag == nil {
		return false
	}
	return flag(flagKey(r.QuestId, r.Flags[fi].Key)) == r.Steps[i].Branch
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
	fi := r.branchFlag()
	if fi < 0 {
		return [][]QuestStep{slices.Clone(r.Steps)}
	}
	out := make([][]QuestStep, 0, len(r.Flags[fi].Values))
	for _, v := range r.Flags[fi].Values {
		out = append(out, r.pathFor(v))
	}
	return out
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
