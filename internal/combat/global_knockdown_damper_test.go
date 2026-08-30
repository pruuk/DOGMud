package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The damper was chosen over a score multiplier precisely because it is EXACT
// and LINEAR: 0.5 must mean half as many knockdowns, not "somewhat fewer".
//
// A score multiplier was measured at score parity (40k trials) and rejected:
// below ~0.75 every move collapsed onto ContestFloor (12.5%), above ~1.5 every
// move saturated at the ceiling (87.5%), and at both ends bash, trip and kick
// became indistinguishable, flattening the per-move ordering U10 tuned.
func TestGlobalKnockdownDamper_IsLinear(t *testing.T) {
	const trials = 60000

	for _, chance := range []float64{0.25, 0.5, 0.75} {
		kept := 0
		for i := 0; i < trials; i++ {
			if rollKnockdownDamper(chance) {
				kept++
			}
		}
		got := float64(kept) / float64(trials)
		assert.InDelta(t, chance, got, 0.02,
			"damper at %v must keep that exact fraction; got %.4f", chance, got)
	}
}

// 1.0 and above must be a true pass-through, not a 99.99% approximation: the
// default must never silently drop a knockdown the contest won.
func TestGlobalKnockdownDamper_DefaultKeepsEverything(t *testing.T) {
	for i := 0; i < 10000; i++ {
		require.True(t, rollKnockdownDamper(1.0),
			"a chance of 1.0 must keep every knockdown")
	}
}
