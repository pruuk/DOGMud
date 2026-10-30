package ambient

// ResetForTest empties the budget and the visits and returns a restore
// func. For tests in any package.
func ResetForTest() func() {
	budgetMu.Lock()
	origBudget := budgetNext
	budgetNext = map[int]uint64{}
	budgetMu.Unlock()

	visitMu.Lock()
	origVisits := singleHeard
	singleHeard = map[int]int{}
	visitMu.Unlock()

	return func() {
		budgetMu.Lock()
		budgetNext = origBudget
		budgetMu.Unlock()
		visitMu.Lock()
		singleHeard = origVisits
		visitMu.Unlock()
	}
}

// ChargeForTest sets the round userId may next hear an ambient line.
func ChargeForTest(userId int, nextRound uint64) {
	budgetMu.Lock()
	budgetNext[userId] = nextRound
	budgetMu.Unlock()
}
