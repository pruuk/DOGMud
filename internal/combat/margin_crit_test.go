package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Chunk 5.11d — crit derives from the normalized margin of the opposed roll.
//
// Crit was keyed on the attacker's SELF-RELATIVE z-score,
// zScore = (value - mean) / stdDev, which is statistically independent of the
// opponent. Winning decisively could not make you crit, so every situational
// modifier had to be hand-injected into calcCritThreshold — a single scalar with
// about 1.0 of headroom where skill, Accuracy, Blink and grapple all collided
// and saturated.
//
// Traps guarded here (see the 5.11 spec):
//
//	T1  best.margin is DEFENCE-positive; the attacker's margin is its negation.
//	    An inverted sign puts crit on the LOSING side and still compiles, so the
//	    tests below are deliberately ASYMMETRIC — a symmetric test passes under
//	    inversion and proves nothing.
//	T2  best.margin is math.Inf(-1) when no defence was attempted. Under
//	    margin-derivation that reads as an infinitely decisive attack and would
//	    crit EVERY swing. Detect via defenseType, never by testing the margin.
//	T3  Both rolls use the attacker's stdDev, so the difference has standard
//	    deviation stdDev*sqrt(2) — that, not stdDev, is the normaliser.
// ---------------------------------------------------------------------------

// bestWith builds a bestDefenseResult with a defence attempted.
// marginDefencePositive follows the engine's own sign convention: positive means
// the DEFENCE won by that much.
func bestWith(marginDefencePositive, stdDev float64) bestDefenseResult {
	return bestDefenseResult{
		margin:      marginDefencePositive,
		defenseType: "dodge",
		hitRoll:     dice.RollResult{StdDev: stdDev},
		defRoll:     dice.RollResult{StdDev: stdDev},
	}
}

// T1 — the attacker won decisively, so the ATTACKER crits.
func TestNormalizedAttackMargin_AttackerDominanceIsPositive(t *testing.T) {
	// Defence lost by 30 => margin is -30 under the defence-positive convention.
	z, ok := normalizedAttackMargin(bestWith(-30, 10))
	assert.True(t, ok)
	assert.Greater(t, z, 0.0, "a decisive ATTACKER win must yield a positive margin")
}

// T1 — the mirror. A decisive DEFENDER win must NOT read as attacker dominance.
// Asymmetric on purpose: this is the assertion a sign inversion breaks.
func TestNormalizedAttackMargin_DefenderDominanceIsNegative(t *testing.T) {
	z, ok := normalizedAttackMargin(bestWith(30, 10))
	assert.True(t, ok)
	assert.Less(t, z, 0.0, "a decisive DEFENDER win must yield a negative attacker margin")
}

// T3 — the normaliser is stdDev*sqrt(2), not stdDev. A missing sqrt(2) inflates
// this by ~41%.
func TestNormalizedAttackMargin_UsesSqrt2Normaliser(t *testing.T) {
	z, ok := normalizedAttackMargin(bestWith(-10, 10))
	assert.True(t, ok)
	assert.InDelta(t, 10.0/(10.0*math.Sqrt2), z, 1e-9,
		"margin must be divided by stdDev*sqrt(2)")
}

// T2 — THE trap. No defence attempted leaves best.margin at math.Inf(-1).
// Negating that gives +Inf, which would crit every single swing.
func TestNormalizedAttackMargin_NoDefenceAttempted(t *testing.T) {
	b := bestDefenseResult{
		margin:      math.Inf(-1), // exactly what runBestOfAllDefense initialises
		defenseType: "",           // nothing was attempted
		hitRoll:     dice.RollResult{StdDev: 10},
	}
	z, ok := normalizedAttackMargin(b)
	assert.False(t, ok, "no defence attempted must report no usable margin")
	assert.False(t, math.IsInf(z, 0), "must never hand back an infinite margin")
}

// A zero or negative stdDev must not divide by zero.
func TestNormalizedAttackMargin_ZeroStdDev(t *testing.T) {
	z, ok := normalizedAttackMargin(bestWith(-30, 0))
	assert.False(t, ok)
	assert.False(t, math.IsInf(z, 0))
	assert.False(t, math.IsNaN(z))
}

// Calibration: at parity the normalized margin is standard normal, so a 2.0
// threshold reproduces the legacy ~2.3% crit rate. This is what makes the change
// safe for even fights without a retune.
func TestMarginCrit_ParityRateMatchesLegacy(t *testing.T) {
	const (
		trials = 200000
		stat   = 100.0
	)
	sd := dice.StdDevFor(stat)

	crits := 0
	for i := 0; i < trials; i++ {
		atk := dice.Roll(stat, sd)
		def := dice.Roll(stat, sd)
		b := bestDefenseResult{
			margin:      def.Value - atk.Value, // engine's defence-positive convention
			defenseType: "dodge",
			hitRoll:     atk,
			defRoll:     def,
		}
		if z, ok := normalizedAttackMargin(b); ok && z >= 2.0 {
			crits++
		}
	}

	rate := float64(crits) / float64(trials)
	assert.InDelta(t, 0.0228, rate, 0.004,
		"at parity a 2.0 margin threshold must reproduce the legacy ~2.3%% crit rate, got %.4f", rate)
}

// A large skill/stat advantage must produce materially MORE crits than parity.
// This is the whole point of the change and cannot happen under the old
// self-relative z-score, which is blind to the opponent.
func TestMarginCrit_AdvantageRaisesCritRate(t *testing.T) {
	const trials = 100000

	rateFor := func(atkStat, defStat float64) float64 {
		sd := dice.StdDevFor(atkStat)
		crits := 0
		for i := 0; i < trials; i++ {
			a := dice.Roll(atkStat, sd)
			d := dice.Roll(defStat, sd)
			b := bestDefenseResult{
				margin: d.Value - a.Value, defenseType: "dodge",
				hitRoll: a, defRoll: d,
			}
			if z, ok := normalizedAttackMargin(b); ok && z >= 2.0 {
				crits++
			}
		}
		return float64(crits) / float64(trials)
	}

	parity := rateFor(100, 100)
	dominant := rateFor(150, 100)

	assert.Greater(t, dominant, parity*3,
		"a 1.5x score advantage must crit far more than parity (parity=%.4f dominant=%.4f)",
		parity, dominant)
}
