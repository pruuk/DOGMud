package buffs

import "testing"

// Ids clear of seedRegistry's 100/101/106 and of other packages' test fixtures.
const (
	refreshTestCadenceBuffId = 9301 // RoundInterval 2, TriggerCount 5
	refreshTestPermaBuffId   = 9302 // RoundInterval 1, TriggerCount 3, granted permanent
	refreshTestUnheldBuffId  = 9303 // never added
	refreshTestDeadBuffId    = 9304 // held (indexed by Validate) but no live spec
)

// RefreshBuff must top TriggersLeft back up without resetting RoundCounter.
// AddBuff resets both, which would starve any buff whose RoundInterval is
// above one: refreshed every round, RoundCounter would be zeroed every round
// and Trigger's `RoundCounter % RoundInterval == 0` would never see anything
// but 0 % N == 0 on round 1, then reset again before round 2 ever accumulates.
func TestRefreshBuff_KeepsCadenceAcrossRepeatedRefreshes(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		refreshTestCadenceBuffId: {BuffId: refreshTestCadenceBuffId, Name: "Test Cadence", TriggerCount: 5, RoundInterval: 2},
	})
	defer restore()

	bs := New()
	if !bs.AddBuff(refreshTestCadenceBuffId, false) {
		t.Fatal("precondition: could not grant the cadence buff")
	}

	triggerTotal := 0
	for round := 1; round <= 6; round++ {
		triggered := bs.Trigger()
		triggerTotal += len(triggered)

		if !bs.RefreshBuff(refreshTestCadenceBuffId) {
			t.Fatalf("round %d: RefreshBuff returned false for a held buff", round)
		}

		idx := bs.buffIds[refreshTestCadenceBuffId]
		if got := bs.List[idx].TriggersLeft; got != 5 {
			t.Fatalf("round %d: TriggersLeft = %d, want 5 (topped back up)", round, got)
		}
	}

	if triggerTotal != 3 {
		t.Errorf("triggered %d times across 6 rounds at RoundInterval 2, want 3 (rounds 2, 4, 6)", triggerTotal)
	}
}

// A permanent held buff (isPermanent true on AddBuff) must stay permanent
// through a refresh: RefreshBuff must not read the spec's finite TriggerCount
// over top of TriggersLeftUnlimited, and must not touch PermaBuff.
func TestRefreshBuff_LeavesAPermanentBuffPermanent(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		refreshTestPermaBuffId: {BuffId: refreshTestPermaBuffId, Name: "Test Perma", TriggerCount: 3, RoundInterval: 1},
	})
	defer restore()

	bs := New()
	if !bs.AddBuff(refreshTestPermaBuffId, true) {
		t.Fatal("precondition: could not grant the permanent buff")
	}

	if !bs.RefreshBuff(refreshTestPermaBuffId) {
		t.Fatal("RefreshBuff returned false for a held permanent buff")
	}

	idx := bs.buffIds[refreshTestPermaBuffId]
	if !bs.List[idx].PermaBuff {
		t.Error("RefreshBuff cleared PermaBuff on a permanent buff")
	}
	if got := bs.List[idx].TriggersLeft; got != TriggersLeftUnlimited {
		t.Errorf("TriggersLeft = %d, want TriggersLeftUnlimited", got)
	}
}

// An id never granted is not held, so nothing to refresh.
func TestRefreshBuff_UnheldIdReturnsFalse(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		refreshTestUnheldBuffId: {BuffId: refreshTestUnheldBuffId, Name: "Test Unheld", TriggerCount: 3, RoundInterval: 1},
	})
	defer restore()

	bs := New()
	if bs.RefreshBuff(refreshTestUnheldBuffId) {
		t.Error("RefreshBuff returned true for an id never added")
	}
}

// A save can carry a buff id whose content was later removed. Buffs.Validate
// indexes it into buffIds before checking whether GetBuffSpec finds anything,
// so the id is "held" by the buffIds map despite having no live spec. Refresh
// must decline rather than guess a TriggerCount.
func TestRefreshBuff_HeldDeadIdReturnsFalse(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{})
	defer restore()

	bs := Buffs{List: []*Buff{{BuffId: refreshTestDeadBuffId, TriggersLeft: 3}}}
	bs.Validate()

	if _, ok := bs.buffIds[refreshTestDeadBuffId]; !ok {
		t.Fatal("precondition: Validate should have indexed the dead id anyway")
	}
	if bs.RefreshBuff(refreshTestDeadBuffId) {
		t.Error("RefreshBuff returned true for a held buff with no live spec")
	}
}
