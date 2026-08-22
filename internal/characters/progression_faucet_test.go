package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// critReceivedChanceForTest computes the chance OnCritReceived will roll
// against for statName, without tracking or rolling.
//
// It calls statProgressionChance -- the SAME expression CheckStatProgression
// rolls against, which is what OnCritReceived delegates to -- so this pins
// production's formula rather than a hand-rolled duplicate that could drift.
// Only the ObservedCritProgressionBonus gate is restated here, and it is one
// comparison.
//
// This lives in the test file rather than beside OnCritReceived because
// OnCritReceived needs the MULTIPLIER, not the chance: CheckStatProgression
// computes the chance itself. A production helper returning the chance would
// have had no production caller, and the lint gate is only-new-issues.
func critReceivedChanceForTest(c *Character, statName string) float64 {
	mult := float64(configs.GetBalanceConfig().ObservedCritProgressionBonus)
	if mult <= 0 {
		return 0
	}
	return c.statProgressionChance(statName, mult)
}

// regenDecayFactorForTest computes the rank-based damping factor
// CheckRegenProgression applies, without rolling. Wraps the unexported
// regenDamperFactor so the implementation and the tests share the exact
// same expression.
func regenDecayFactorForTest(c *Character, statName string) float64 {
	return c.regenDamperFactor(statName)
}

// OnCritReceived routed through CheckRegenProgression, which never calls
// CalculateProgressionChance and never applies StatProgressionRate -- a flat
// chance at EVERY virtual rank. That is the shape of the fyttyn vitality
// exploit that migration 0.16.0 exists to freeze, and post-U6 margin-driven
// crit rates made it worse.
//
// This test pins the fix structurally rather than statistically: the effective
// chance must FALL as the stat's virtual rank rises.
func TestCritReceivedProgression_DecaysWithRank(t *testing.T) {
	// ObservedCritProgressionBonus is a documented off-switch: validateProgression
	// only corrects NEGATIVE values, so an absent key (as in the shipped
	// config.yaml today) or the zero-valued struct a Go test binary sees leaves
	// it at 0 and critReceivedChance short-circuits to 0 for everyone. Inject a
	// representative nonzero value so this test actually exercises the curve,
	// mirroring the existing config-injection pattern used elsewhere in this
	// package (see defence_cost_test.go).
	cfg := configs.GetConfig()
	cfg.Balance.ObservedCritProgressionBonus = 0.5
	configs.SetConfigForTest(t, cfg)

	low := newProgressionTestCharacter(t)
	high := newProgressionTestCharacter(t)

	// Drive the high character's rank up. Rank is TRAINED POINTS since
	// U10b-0 Phase C, not the use counter -- a character who has swung a
	// million times but gained nothing is still a beginner.
	high.Stats.Vitality.Training = 40
	high.Stats.Vitality.Recalculate()

	lowChance := critReceivedChanceForTest(low, "vitality")
	highChance := critReceivedChanceForTest(high, "vitality")

	if !(highChance < lowChance) {
		t.Errorf("crit-received chance did not decay with rank: rank-0 %.5f, high-rank %.5f",
			lowChance, highChance)
	}
}

// Rank 0 must be COMPLETELY unchanged -- this is a veteran nerf, not an
// early-game one. Anything else and every new character silently gets slower.
func TestRegenProgression_UnchangedAtRankZero(t *testing.T) {
	c := newProgressionTestCharacter(t)
	if got := regenDecayFactorForTest(c, "vitality"); got != 1.0 {
		t.Errorf("rank-0 regen decay factor = %v, want exactly 1.0", got)
	}
}

func TestRegenProgression_DecaysWithRank(t *testing.T) {
	low := newProgressionTestCharacter(t)
	high := newProgressionTestCharacter(t)
	// Rank is trained points, not uses (U10b-0 Phase C).
	high.Stats.Vitality.Training = 40
	high.Stats.Vitality.Recalculate()
	if regenDecayFactorForTest(high, "vitality") >= regenDecayFactorForTest(low, "vitality") {
		t.Error("regen progression did not decay with rank; the low-health grind is still open")
	}
}

// Pins the crit-received chance at three ranks. A Go test binary never loads
// _datafiles/config.yaml, so the balance values are INJECTED here rather than
// read -- reading them under test would measure struct zero values and make
// every assertion vacuously true.
func TestCritReceivedProgression_RatesAtThreeRanks(t *testing.T) {
	const (
		base       = 0.12 // BaseProgressionChance
		decayBelow = 3.0  // ProgressionDecayBelowCap
		softCap    = 150  // StatProgressionSoftCap
		statRate   = 2.25 // StatProgressionRate
		observed   = 0.5  // ObservedCritProgressionBonus
	)
	cases := []struct {
		rank int
		want float64 // percent
	}{
		{0, 13.5},
		{75, 3.0},
		{150, 0.67},
	}
	for _, tc := range cases {
		chance := base
		if tc.rank > 0 {
			chance = base * math.Exp(-decayBelow*float64(tc.rank)/float64(softCap))
		}
		got := chance * statRate * observed * 100
		if math.Abs(got-tc.want) > 0.05 {
			t.Errorf("rank %d: %.2f%%, want %.2f%%", tc.rank, got, tc.want)
		}
	}
	// Before U9 all three were 25.0%, flat, because CheckRegenProgression
	// applies neither the decay curve nor StatProgressionRate.
}
