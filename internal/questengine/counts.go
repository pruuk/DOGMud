package questengine

// StepCounter is implemented by action contexts that keep count: progress
// (#375 phase 3b slice 4). A context without it treats a count as reached on
// the first match.
type StepCounter interface {
	// BumpStepCount adds one to the count saved under key and returns the
	// new count, but only while the player's current step of questId is
	// step; otherwise it bumps nothing and ok is false.
	BumpStepCount(questId int, step, key string) (count int, ok bool)
}

// countResult is what one match does to a count: N trigger.
type countResult int

const (
	// countReached: the trigger fires (no count, no counter, or the Nth).
	countReached countResult = iota
	// countCounted: the match was counted, short of N.
	countCounted
	// countSkipped: the player is not on the trigger's own step, so the
	// match was not counted.
	countSkipped
)

// countMatch bumps a count: N trigger's own count (keyed by the trigger and
// its own step, quests.CountKey) and reports whether this match is the Nth.
// Each count trigger keeps its own count, so two on one step never credit
// each other.
func countMatch(t *indexedTrigger, ctx ActionContext) countResult {
	if t.def.Count < 2 {
		return countReached
	}
	sc, ok := ctx.(StepCounter)
	if !ok {
		return countReached
	}
	n, ok := sc.BumpStepCount(t.questId, t.countStep, t.countKey)
	if !ok {
		return countSkipped
	}
	if n >= t.def.Count {
		return countReached
	}
	return countCounted
}
