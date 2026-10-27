package questengine

// StepCounter is implemented by action contexts that keep count: progress
// (#375 phase 3b slice 4). A context without it treats a count as reached on
// the first match.
type StepCounter interface {
	BumpStepCount(questId int) int // bumps the player's current step of questId, returns the new count
}

// countReached bumps a count: N trigger's progress and reports whether this
// match is the Nth. A trigger without a count, or a context that keeps no
// counts, is reached at once.
func countReached(t *indexedTrigger, ctx ActionContext) bool {
	if t.def.Count < 2 {
		return true
	}
	sc, ok := ctx.(StepCounter)
	if !ok {
		return true
	}
	return sc.BumpStepCount(t.questId) >= t.def.Count
}
