package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
)

// #465: a crit skips mitigation up to the curve's knee and loses the part of
// effective mitigation above it (MitigationCurve.CritReduction). These tests
// pin that on the shared spell, taunt, throw and skill-move resolver;
// crit_overflow_melee_test.go pins the melee path.

// pinShippedCurve pins the shipped curve on all three channels, restored on
// cleanup, so these tests do not depend on what an earlier test left behind.
func pinShippedCurve(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	b := &cfg.Balance
	b.PhysicalMitigationKnee, b.PhysicalMitigationCap = 0.50, 0.90
	b.PhysicalMitigationAim, b.PhysicalMitigationBend = 0.05, 0.40
	b.MagicalMitigationKnee, b.MagicalMitigationCap = 0.50, 0.90
	b.MagicalMitigationAim, b.MagicalMitigationBend = 0.05, 0.40
	b.ConvictionMitigationKnee, b.ConvictionMitigationCap = 0.50, 0.90
	b.ConvictionMitigationAim, b.ConvictionMitigationBend = 0.05, 0.40
	cfg.GamePlay.UseSkillProgression = false
	configs.SetConfigForTest(t, cfg)
}

// critMeanOver is the mean crit damage of the shared resolver at one raw
// mitigation.
func critMeanOver(samples int, rawDmg float64, rank int, mitig float64) float64 {
	total := 0.0
	for i := 0; i < samples; i++ {
		total += float64(CritOrMitigatedDamage(rawDmg, rank, true, mitig, shippedCurveForTest))
	}
	return total / float64(samples)
}

// Below the knee a crit is unchanged: it lands on the same mean as a crit
// against no mitigation at all.
func TestCritOrMitigatedDamage_BelowTheKneeIsUnchanged(t *testing.T) {
	const samples, rawDmg, rank = 20000, 60.0, 40
	bare := critMeanOver(samples, rawDmg, rank, 0.0)
	light := critMeanOver(samples, rawDmg, rank, 0.30)
	assert.InDelta(t, 1.0, light/bare, 0.03, "a crit against 30%% raw mitigation must equal a crit against none")
}

// Above the knee a crit loses the overflow: raw 100% bends to 75% effective,
// 25 points over the knee, so the crit keeps three quarters.
func TestCritOrMitigatedDamage_AboveTheKneeLosesTheOverflow(t *testing.T) {
	const samples, rawDmg, rank = 20000, 60.0, 40
	bare := critMeanOver(samples, rawDmg, rank, 0.0)
	heavy := critMeanOver(samples, rawDmg, rank, 1.00)
	assert.InDelta(t, 0.75, heavy/bare, 0.03, "a crit against 100%% raw mitigation keeps 1 - (0.75 - 0.50)")

	// At the cap the crit keeps 1 - (0.90 - 0.50) = 0.60, and is still worth
	// several normal hits: a normal hit there keeps 0.10.
	capped := critMeanOver(samples, rawDmg, rank, 10.0)
	assert.InDelta(t, 0.60, capped/bare, 0.03)
}
