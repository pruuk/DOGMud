package combat

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// shippedCurveForTest is the curve config.yaml ships on every channel (#465):
// knee 0.50, cap 0.90, aim 0.05, bend 0.40. Written out here, not read from
// config, so the pure function is tested against the owner's numbers even in
// a test binary that never loads config.yaml.
var shippedCurveForTest = MitigationCurve{Knee: 0.50, Cap: 0.90, Aim: 0.05, Bend: 0.40}

// The spec's table (section 1): raw mitigation in, effective out, within
// half a percentage point (the table is rounded to whole percents).
func TestEffectiveMitigation_MatchesTheSpecTable(t *testing.T) {
	rows := []struct{ raw, want float64 }{
		{0.50, 0.50},
		{0.75, 0.67},
		{0.91, 0.73},
		{1.00, 0.75},
		{1.64, 0.83},
		{1.81, 0.84},
		{2.70, 0.88},
		{3.30, 0.89},
		{3.70, 0.90},
	}
	for _, r := range rows {
		got := EffectiveMitigation(r.raw, shippedCurveForTest)
		assert.InDelta(t, r.want, got, 0.005, "raw %.2f", r.raw)
	}
}

// Below the knee the curve is the identity: every point of raw mitigation
// counts in full, as it did under the old clamp.
func TestEffectiveMitigation_LinearUpToTheKnee(t *testing.T) {
	for _, raw := range []float64{0.01, 0.10, 0.25, 0.40, 0.4999, 0.50} {
		assert.InDelta(t, raw, EffectiveMitigation(raw, shippedCurveForTest), 1e-12, "raw %.4f", raw)
	}
}

// The curve has no step at the knee: a hair above it reads a hair above it.
func TestEffectiveMitigation_ContinuousAtTheKnee(t *testing.T) {
	c := shippedCurveForTest
	below := EffectiveMitigation(c.Knee-1e-9, c)
	at := EffectiveMitigation(c.Knee, c)
	above := EffectiveMitigation(c.Knee+1e-9, c)
	assert.InDelta(t, at, below, 1e-8)
	assert.InDelta(t, at, above, 1e-8)
}

// More raw mitigation never means less protection, and nothing exceeds the
// cap, across a sweep well past any shipped build (raw 0 to 10).
func TestEffectiveMitigation_IncreasingAndNeverAboveTheCap(t *testing.T) {
	c := shippedCurveForTest
	prev := 0.0
	for i := 0; i <= 10000; i++ {
		raw := float64(i) / 1000.0
		got := EffectiveMitigation(raw, c)
		assert.GreaterOrEqual(t, got, prev, "raw %.3f fell below raw %.3f", raw, raw-0.001)
		assert.LessOrEqual(t, got, c.Cap, "raw %.3f above the cap", raw)
		prev = got
	}
}

// Aim above 0 makes the cap reachable; aim 0 only approaches it.
func TestEffectiveMitigation_AimZeroNeverReachesTheCap(t *testing.T) {
	c := shippedCurveForTest
	assert.Equal(t, c.Cap, EffectiveMitigation(10.0, c), "aim 0.05 reaches the cap")

	c.Aim = 0
	for _, raw := range []float64{1, 10, 100, 1e6} {
		assert.Less(t, EffectiveMitigation(raw, c), c.Cap, "aim 0 at raw %.0f", raw)
	}
}

// Nothing negative and nothing not-a-number reaches the damage roll: both read
// as no mitigation (the guard compressContestGap uses).
func TestEffectiveMitigation_NaNAndNegativeReadAsNone(t *testing.T) {
	c := shippedCurveForTest
	assert.Equal(t, 0.0, EffectiveMitigation(-0.5, c))
	assert.Equal(t, 0.0, EffectiveMitigation(0, c))
	assert.Equal(t, 0.0, EffectiveMitigation(math.NaN(), c))
	assert.Equal(t, c.Cap, EffectiveMitigation(math.Inf(1), c))
}

// A zero curve (never built by MitigationCurveFor, which reads validated
// config) mitigates nothing rather than dividing by zero.
func TestEffectiveMitigation_ZeroCurveMitigatesNothing(t *testing.T) {
	assert.Equal(t, 0.0, EffectiveMitigation(0.6, MitigationCurve{}))
}

// The crit reduction is the part of effective mitigation above the knee:
// none at or below it, the overflow above it.
func TestCritReduction_IsTheOverflowAboveTheKnee(t *testing.T) {
	c := shippedCurveForTest
	assert.Equal(t, 0.0, c.CritReduction(0.30))
	assert.Equal(t, 0.0, c.CritReduction(0.50))
	assert.InDelta(t, EffectiveMitigation(1.00, c)-0.50, c.CritReduction(1.00), 1e-12)
	assert.InDelta(t, 0.25, c.CritReduction(1.00), 1e-12)
	assert.InDelta(t, c.Cap-c.Knee, c.CritReduction(10.0), 1e-12)
	assert.Equal(t, 0.0, c.CritReduction(math.NaN()))
}
