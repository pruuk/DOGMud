package ambient

// ResetForTest empties the budget, the visits, the repeat memory and the telemetry and returns
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

	repeatMu.Lock()
	origRepeat := repeatSeen
	repeatSeen = map[int]map[string][]uint64{}
	repeatMu.Unlock()

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
		repeatMu.Lock()
		repeatSeen = origRepeat
		repeatMu.Unlock()
		telMu.Lock()
		telRooms, telNextReport, telWindow = origRooms, origNext, origWindow
		telKept.Store(origKept)
		telMu.Unlock()
	}
}

// MovesForTest returns a copy of the mob moves telemetry has counted in
// roomId this window, by kind. Empty when none were.
func MovesForTest(roomId int) map[MoveKind]int {
	telMu.Lock()
	defer telMu.Unlock()
	out := map[MoveKind]int{}
	if rc := telRooms[roomId]; rc != nil {
		for k, v := range rc.moves {
			out[k] = v
		}
	}
	return out
}

// RepeatKeysForTest returns how many line keys the repeat limit remembers
// for userId.
func RepeatKeysForTest(userId int) int {
	repeatMu.Lock()
	defer repeatMu.Unlock()
	return len(repeatSeen[userId])
}

// ChargeForTest sets the round userId may next hear an ambient line.
func ChargeForTest(userId int, nextRound uint64) {
	budgetMu.Lock()
	budgetNext[userId] = nextRound
	budgetMu.Unlock()
}
