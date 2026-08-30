package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/stretchr/testify/assert"
)

// The spec's headline claim, measured against LIVE DICE rather than the model.
//
// Neither change ships alone: compression leaves the Elemental Queen high, and
// redistribution ALONE strips Dexterity from every fighting mob and makes the
// royal fighters worse. This pins the combined outcome.
//
// ⚠️ ContestFloor is 0.125 and flips that fraction of ALL contested outcomes,
// and a floored result cannot crit (`!res.Floored` gates it). So 0.875 is the
// ceiling on any win rate and crit rates run below the unfloored model.
func TestOasisCritRatesAfterCompression(t *testing.T) {
	const trials = 40000
	const meirokAttack = 455.0

	critRate := func(def, p float64) float64 {
		SetContestGapCompressionForTest(t, p)
		crits := 0
		for i := 0; i < trials; i++ {
			res := RunContest(meirokAttack, []contest.Entry{{Score: def}})
			if !res.Floored && AttackContestCritAt(res.Margin, res.AttackRoll, 1.5) {
				crits++
			}
		}
		return float64(crits) / trials
	}

	// 86 defence: a storm elemental AFTER redistribution.
	stormNow := critRate(86, 1.0)
	stormAfter := critRate(86, 0.80)
	assert.Greater(t, stormNow, 0.70, "uncompressed, a weak defence is crit constantly")
	assert.Less(t, stormAfter, stormNow*0.75, "compression must pull that back materially")

	// 276 defence: royal fighters after redistribution, the tier already behaving.
	royalNow := critRate(276, 1.0)
	royalAfter := critRate(276, 0.80)
	assert.Less(t, royalAfter, royalNow, "compression must not make a competitive matchup worse")

	// The Queen after redistribution must land clear of saturation.
	assert.Less(t, critRate(168, 0.80), 0.50,
		"the Queen after redistribution plus compression must be well off the crit ceiling")

	// Parity is invariant: this is the property that makes the knob safe.
	assert.InDelta(t, critRate(455, 1.0), critRate(455, 0.80), 0.02,
		"an even fight must be untouched at any exponent")
}
