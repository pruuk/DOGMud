package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dice"
)

// Chunk 5.11g follow-through — margin-derived crit for the spell and conviction
// channels, which 5.11d converted only for melee.
//
// SIGN WARNING. dice.OpposedRoll returns margin = attackRoll.Value -
// defenseRoll.Value, which is ATTACK-positive. That is the OPPOSITE of
// bestDefenseResult.margin, which runBestOfAllDefense builds defence-positive
// and which normalizedAttackMargin therefore has to negate. Copying that
// negation to this path would put crit on the losing side and still compile.
// These tests exist mostly to pin that down.

func TestContestCrit_ParityDoesNotCrit(t *testing.T) {
	// A dead-even contest has margin ~0. Nothing about that is decisive.
	roll := dice.RollResult{StdDev: 15.0, ZScore: 0.0}
	if ContestCrit(0.0, roll) {
		t.Error("a zero margin must not crit")
	}
}

func TestContestCrit_DecisiveWinCrits(t *testing.T) {
	// The threshold is 2.0 standard deviations of the DIFFERENCE, and both
	// sides roll with the same stdDev, so the difference has stdDev*sqrt(2).
	stdDev := 15.0
	justOver := 2.0*stdDev*math.Sqrt2 + 0.01
	justUnder := 2.0*stdDev*math.Sqrt2 - 0.01

	roll := dice.RollResult{StdDev: stdDev, ZScore: 0.0}
	if !ContestCrit(justOver, roll) {
		t.Error("a margin just past 2 sigma must crit")
	}
	if ContestCrit(justUnder, roll) {
		t.Error("a margin just short of 2 sigma must not crit")
	}
}

func TestContestCrit_LosingSideNeverCrits(t *testing.T) {
	// The sign trap. A large NEGATIVE margin means the tested side was
	// annihilated. If the sign were inverted this is the case that would crit
	// on every swing.
	roll := dice.RollResult{StdDev: 15.0, ZScore: 0.0}
	for _, m := range []float64{-1, -50, -1000} {
		if ContestCrit(m, roll) {
			t.Errorf("margin %v is a LOSS and must never crit", m)
		}
	}
}

func TestContestCrit_FloorSentinelDoesNotCrit(t *testing.T) {
	// dice.OpposedRollStatWithFloors overwrites margin with +-1 when a contest
	// floor fires, per the 5.9 convention. That 1 is a sentinel, not a real
	// margin. A hit handed out by the floor must not also be a critical hit.
	roll := dice.RollResult{StdDev: 15.0, ZScore: 3.5} // roll itself looks great
	if ContestCrit(1.0, roll) {
		t.Error("a floor-granted success (margin sentinel 1) must not crit")
	}
}

func TestContestCrit_FallsBackWhenStdDevUnusable(t *testing.T) {
	// With stdDev 0 there is no scale to normalise against and the margin
	// carries no information. Fall back to the legacy self-relative z-score
	// rather than dividing by zero.
	if !ContestCrit(0.0, dice.RollResult{StdDev: 0, ZScore: 2.5}) {
		t.Error("stdDev 0 must fall back to the self-relative z-score (2.5 >= 2.0)")
	}
	if ContestCrit(0.0, dice.RollResult{StdDev: 0, ZScore: 1.0}) {
		t.Error("stdDev 0 fallback must still respect the threshold")
	}
}

// critRateAt samples the crit rate for an attacker of `atk` against a defender
// of `def`, driving the real dice path the spell and taunt sites use.
func critRateAt(atk, def float64, samples int) float64 {
	crits := 0
	for i := 0; i < samples; i++ {
		_, margin, atkRoll, _ := dice.OpposedRollStatRaw(atk, def)
		if ContestCrit(margin, atkRoll) {
			crits++
		}
	}
	return float64(crits) / float64(samples)
}

func TestContestCrit_RateRespondsToTheMatchup(t *testing.T) {
	// THE point of the change. Under the old self-relative z-score, crit rate
	// was statistically independent of the opponent: a caster obliterating a
	// rat critted at the same 2.3% as one duelling an archmage, because the
	// z-score only measured the roll against its OWN mean. These three rates
	// were identical before; now they must separate, and in the right order.
	const samples = 100000

	outclassed := critRateAt(60, 140, samples)
	parity := critRateAt(100, 100, samples)
	dominant := critRateAt(140, 60, samples)

	if !(outclassed < parity && parity < dominant) {
		t.Errorf("crit rate must rise with advantage; got outclassed=%.4f parity=%.4f dominant=%.4f",
			outclassed, parity, dominant)
	}

	// And the separation must be substantial, not a rounding artifact — this
	// is what would still fail if the margin were normalised but then scaled
	// so weakly that the matchup barely registered.
	if dominant < 4*parity {
		t.Errorf("dominant crit rate %.4f is not meaningfully above parity %.4f", dominant, parity)
	}
	if outclassed > parity/2 {
		t.Errorf("outclassed crit rate %.4f should be well below parity %.4f", outclassed, parity)
	}
}

func TestContestCrit_ReproducesLegacyRateAtParity(t *testing.T) {
	// The 5.11d promise, restated for this channel: threshold 2.0 reproduces
	// the legacy ~2.3% crit rate at parity BY CONSTRUCTION, so evenly matched
	// fights need no retune. This is the test that would catch a missing
	// sqrt(2) — dividing by stdDev alone inflates the margin by ~41% and would
	// push this rate to roughly 8%.
	const (
		samples = 200000
		stat    = 100.0
	)

	crits := 0
	for i := 0; i < samples; i++ {
		_, margin, atkRoll, _ := dice.OpposedRollStatRaw(stat, stat)
		if ContestCrit(margin, atkRoll) {
			crits++
		}
	}

	rate := float64(crits) / samples
	if math.Abs(rate-0.023) > 0.005 {
		t.Errorf("parity crit rate = %.4f, want ~0.023 (legacy z>=2.0 rate)", rate)
	}
}
