package ambient

// ResetForTest empties the budget, the visits and the telemetry and returns
// a restore func. For tests in any package.
func ResetForTest() func() {
	budgetMu.Lock()
	origBudget := budgetNext
	budgetNext = map[int]uint64{}
	budgetMu.Unlock()

	visitMu.Lock()
	origVisits := singleHeard
	singleHeard = map[int]int{}
	visitMu.Unlock()

	telMu.Lock()
	origRooms, origNext, origWindow, origKept := telRooms, telNextReport, telWindow, telKept.Load()
	telRooms, telNextReport, telWindow = map[int]*roomCounts{}, 0, 0
	telKept.Store(false)
	telMu.Unlock()

	return func() {
		budgetMu.Lock()
		budgetNext = origBudget
		budgetMu.Unlock()
		visitMu.Lock()
		singleHeard = origVisits
		visitMu.Unlock()
		telMu.Lock()
		telRooms, telNextReport, telWindow = origRooms, origNext, origWindow
		telKept.Store(origKept)
		telMu.Unlock()
	}
}

// ChargeForTest sets the round userId may next hear an ambient line.
func ChargeForTest(userId int, nextRound uint64) {
	budgetMu.Lock()
	budgetNext[userId] = nextRound
	budgetMu.Unlock()
}
