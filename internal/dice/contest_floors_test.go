package dice

import (
	"math"
	"testing"
)

// Chunk 5.9a. Combat bounded both ends of its hit/avoid contest; every other
// consumer of OpposedRollStat was unbounded, so a much weaker initiator
// essentially could not succeed and a much stronger one essentially could not
// fail. These pin the floors that close that gap.

func withFloors(t *testing.T, success, resist float64) {
	t.Helper()
	prevS, prevR := ContestFloors()
	SetContestFloors(success, resist)
	t.Cleanup(func() { SetContestFloors(prevS, prevR) })
}

// rate runs the contest many times and returns the observed success rate.
func rate(atk, def float64, n int) float64 {
	wins := 0
	for i := 0; i < n; i++ {
		if ok, _, _, _ := OpposedRollStatFloored(atk, def); ok {
			wins++
		}
	}
	return float64(wins) / float64(n)
}

// THE gap. Unfloored, a stat-100 initiator against a stat-150 resister wins
// about 0.9% of the time. The floor must lift that to roughly the floor value.
func TestOpposedRollStatFloored_OutmatchedInitiatorKeepsAChance(t *testing.T) {
	withFloors(t, 0.05, 0.05)

	const n = 40000
	got := rate(100, 150, n)

	if got < 0.03 {
		t.Errorf("outmatched initiator won %.2f%% of contests; the floor is not applying", got*100)
	}
	// Unfloored this is ~0.9%; floored it should be near 5% plus the small raw
	// chance. Generous upper bound so this cannot flake.
	if got > 0.09 {
		t.Errorf("outmatched initiator won %.2f%%; far above the 5%% floor", got*100)
	}
}

// The other end: an overwhelming initiator must not be a certainty.
func TestOpposedRollStatFloored_OverwhelmingInitiatorIsNotCertain(t *testing.T) {
	withFloors(t, 0.05, 0.05)

	const n = 40000
	got := rate(200, 100, n)

	if got > 0.97 {
		t.Errorf("overwhelming initiator won %.2f%% of contests; the resist floor is not applying", got*100)
	}
	if got < 0.90 {
		t.Errorf("overwhelming initiator won only %.2f%%; the resist floor is too strong", got*100)
	}
}

// At parity the floors cancel and the contest stays even.
func TestOpposedRollStatFloored_ParityStaysEven(t *testing.T) {
	withFloors(t, 0.05, 0.05)

	got := rate(100, 100, 40000)
	if math.Abs(got-0.5) > 0.03 {
		t.Errorf("parity contest won %.2f%%, want ~50%%", got*100)
	}
}

// Zero disables an end, which must reproduce the raw behaviour exactly.
func TestOpposedRollStatFloored_ZeroDisablesTheFloor(t *testing.T) {
	withFloors(t, 0, 0)

	got := rate(100, 150, 40000)
	if got > 0.03 {
		t.Errorf("with floors disabled the outmatched initiator won %.2f%%; want the raw ~0.9%%", got*100)
	}
}

// A floor save is a BARE success. Callers that scale an effect by margin must
// not read a last-resort save as a decisive one.
func TestOpposedRollStatFloored_FlippedOutcomeHasMinimalMargin(t *testing.T) {
	// 0.5 is the maximum the setter accepts -- above that a "floor" would be the
	// dominant term rather than a last resort. At a 3x stat gap the raw success
	// chance is effectively zero, so any success here IS a floor save.
	withFloors(t, 0.5, 0)

	flips := 0
	for i := 0; i < 400; i++ {
		success, margin, atkRoll, defRoll := OpposedRollStatFloored(100, 300)
		if !success {
			continue
		}
		flips++
		if margin != 1 {
			t.Fatalf("flipped margin = %v, want 1: a floor save is a bare success, not a rout", margin)
		}
		if !atkRoll.Success || defRoll.Success {
			t.Fatal("roll result flags disagree with the flipped outcome")
		}
		if defRoll.Margin != -margin {
			t.Fatalf("defender margin = %v, want %v", defRoll.Margin, -margin)
		}
	}

	if flips == 0 {
		t.Fatal("no contest was rescued at a 0.5 floor; the test proves nothing")
	}
}

func TestSetContestFloors_ClampsOutOfRangeValues(t *testing.T) {
	withFloors(t, 0.05, 0.05)

	SetContestFloors(-1, 99)
	s, r := ContestFloors()
	if s != 0 {
		t.Errorf("negative floor = %v, want 0", s)
	}
	if r != 0.5 {
		t.Errorf("floor above range = %v, want 0.5 (a floor must stay a last resort)", r)
	}
}
