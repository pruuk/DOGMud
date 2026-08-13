package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dice"
)

// TestRunWithGlobalFloorsTracksDiceGlobals is the discriminating test for the
// trap in the U4 plan: config.yaml ships all three floor pairs at 0.05, so a
// wrapper wired to the WRONG pair is invisible in production. This moves the
// global pair to values nothing else uses and asserts the wrapper follows it.
func TestRunWithGlobalFloorsTracksDiceGlobals(t *testing.T) {
	origS, origR := dice.ContestFloors()
	t.Cleanup(func() { dice.SetContestFloors(origS, origR) })

	// A hopeless attacker: 10 against 1000 never wins on the roll, so the floor
	// is the entire signal.
	const hopeless, overwhelming = 10.0, 1000.0

	dice.SetContestFloors(0.5, 0)
	wins := 0
	for i := 0; i < 4000; i++ {
		if RunWithGlobalFloors(hopeless, overwhelming).Success {
			wins++
		}
	}
	if wins < 1600 || wins > 2400 {
		t.Errorf("floorSuccess=0.5 on a hopeless attack: got %d/4000 wins, want ~2000", wins)
	}

	dice.SetContestFloors(0, 0)
	wins = 0
	for i := 0; i < 4000; i++ {
		if RunWithGlobalFloors(hopeless, overwhelming).Success {
			wins++
		}
	}
	if wins > 40 {
		t.Errorf("floorSuccess=0 on a hopeless attack: got %d/4000 wins, want ~0", wins)
	}
}

// TestRunWithGlobalFloorsMatchesDiceOpposedRollStat is the no-op proof: the
// wrapper must be indistinguishable from the function the 17 sites call today.
//
// TOLERANCE. Revision 1 of this plan used a flat 3% and was blind to a floor
// that was wrong by 2x -- a halved pair moved every case by at most 2.5%. It
// also included a parity case (100 vs 100) with ZERO discriminating power: for
// any symmetric pair, p*(1-f) + (1-p)*f evaluates to exactly 0.5 at p=0.5, so
// that case cannot fail for any floor error at all.
//
// This version drops parity and uses a per-case 4-sigma bound on the difference
// of two proportions, which never flakes and does catch a halved floor.
func TestRunWithGlobalFloorsMatchesDiceOpposedRollStat(t *testing.T) {
	origS, origR := dice.ContestFloors()
	t.Cleanup(func() { dice.SetContestFloors(origS, origR) })
	dice.SetContestFloors(0.05, 0.05)

	cases := []struct {
		name     string
		atk, def float64
	}{
		{"outmatched", 100, 150}, // floor-success dominated
		{"favoured", 150, 100},   // floor-resist dominated
		{"rout", 100, 30},        // floor-resist is the whole signal
	}

	const n = 20000
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			oldWins := 0
			for i := 0; i < n; i++ {
				if ok, _, _, _ := dice.OpposedRollStat(c.atk, c.def); ok {
					oldWins++
				}
			}
			newWins := 0
			for i := 0; i < n; i++ {
				if RunWithGlobalFloors(c.atk, c.def).Success {
					newWins++
				}
			}

			p1 := float64(oldWins) / n
			p2 := float64(newWins) / n
			// Standard error of the difference of two independent proportions.
			se := math.Sqrt(p1*(1-p1)/n + p2*(1-p2)/n)
			if se == 0 {
				se = 1.0 / n // both arms saturated; any difference is real
			}
			z := math.Abs(p1-p2) / se
			if z > 4.0 {
				t.Errorf("atk=%.0f def=%.0f: dice=%.4f, combat=%.4f (z=%.1f) — not equivalent",
					c.atk, c.def, p1, p2, z)
			}
		})
	}
}

// TestRunWithGlobalFloorsDetectsAHalvedFloor proves the equivalence test above
// has real power. It deliberately compares the correct floor against a halved
// one and asserts the 4-sigma bound DOES fire, so a future tolerance loosening
// cannot quietly make the equivalence test vacuous.
func TestRunWithGlobalFloorsDetectsAHalvedFloor(t *testing.T) {
	origS, origR := dice.ContestFloors()
	t.Cleanup(func() { dice.SetContestFloors(origS, origR) })

	const n = 20000
	const atk, def = 100.0, 150.0 // outmatched: floor-success dominated

	dice.SetContestFloors(0.05, 0.05)
	correct := 0
	for i := 0; i < n; i++ {
		if RunWithGlobalFloors(atk, def).Success {
			correct++
		}
	}

	dice.SetContestFloors(0.025, 0.025)
	halved := 0
	for i := 0; i < n; i++ {
		if RunWithGlobalFloors(atk, def).Success {
			halved++
		}
	}

	p1, p2 := float64(correct)/n, float64(halved)/n
	se := math.Sqrt(p1*(1-p1)/n + p2*(1-p2)/n)
	z := math.Abs(p1-p2) / se
	if z <= 4.0 {
		t.Errorf("a halved floor pair must be detectable: correct=%.4f halved=%.4f z=%.1f, want z>4",
			p1, p2, z)
	}
}
