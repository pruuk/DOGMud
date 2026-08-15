package costs

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// The band is deliberately asymmetric: a gentle penalty for the untrained, a
// large reward for mastery.
func TestSkillCostMultiplierBand(t *testing.T) {
	cases := []struct {
		rank int
		want float64
	}{
		{0, 1.10},
		{25, 1.00},
		{100, 0.40},
		{150, 0.40}, // clamped beyond the cap rank
	}
	for _, c := range cases {
		got := SkillCostMultiplier(c.rank)
		if math.Abs(got-c.want) > 0.0005 {
			t.Errorf("rank %d: got %.4f, want %.4f", c.rank, got, c.want)
		}
	}
}

// Monotonically decreasing: more skill must never cost more.
func TestSkillCostMultiplierIsMonotonic(t *testing.T) {
	prev := SkillCostMultiplier(0)
	for r := 1; r <= 120; r++ {
		got := SkillCostMultiplier(r)
		if got > prev+0.0001 {
			t.Fatalf("rank %d: multiplier rose from %.4f to %.4f", r, prev, got)
		}
		prev = got
	}
}

// A negative rank cannot be cheaper than rank zero.
func TestSkillCostMultiplierNegativeRankClampsToZero(t *testing.T) {
	if SkillCostMultiplier(-5) != SkillCostMultiplier(0) {
		t.Fatalf("negative rank must clamp to the rank-0 penalty")
	}
}

// Midpoint of each segment, to pin the shape rather than only its endpoints.
func TestSkillCostMultiplierSegmentMidpoints(t *testing.T) {
	// Halfway from 0 to the neutral rank: halfway from 1.10 to 1.00.
	if got := SkillCostMultiplier(12); math.Abs(got-1.052) > 0.002 {
		t.Errorf("rank 12: got %.4f, want about 1.052", got)
	}
	// Halfway from neutral to the cap: halfway from 1.00 to 0.40.
	if got := SkillCostMultiplier(62); math.Abs(got-0.704) > 0.002 {
		t.Errorf("rank 62: got %.4f, want about 0.704", got)
	}
}

// The four tests above all use today's shipped-default band, so an
// implementation that hardcoded those numbers instead of reading config
// would pass every one of them. This test sets a deliberately non-default
// balance and asserts the curve moves with it, pinning that the function
// actually reads its knobs. Pattern follows
// TestDiscoveryChance_ReadsConfiguredBalance (internal/configs/discovery_test.go).
func TestSkillCostMultiplier_ReadsConfiguredBalance(t *testing.T) {
	c := configs.GetConfig()
	c.Balance.CostSkillMultAtZero = 3.0
	c.Balance.CostSkillMultAtMid = 2.0
	c.Balance.CostSkillMultAtCap = 0.10
	c.Balance.CostSkillMidRank = 10
	c.Balance.CostSkillCapRank = 40
	configs.SetConfigForTest(t, c)

	if got := SkillCostMultiplier(0); math.Abs(got-3.0) > 0.0005 {
		t.Errorf("rank 0: got %.4f, want 3.0000 (configured atZero)", got)
	}
	if got := SkillCostMultiplier(10); math.Abs(got-2.0) > 0.0005 {
		t.Errorf("rank 10 (configured mid): got %.4f, want 2.0000", got)
	}
	if got := SkillCostMultiplier(40); math.Abs(got-0.10) > 0.0005 {
		t.Errorf("rank 40 (configured cap): got %.4f, want 0.1000", got)
	}
	if got := SkillCostMultiplier(100); math.Abs(got-0.10) > 0.0005 {
		t.Errorf("rank 100 (past configured cap): got %.4f, want 0.1000 (clamped)", got)
	}
	// Old shipped-default values must NOT appear anywhere on this curve now.
	if got := SkillCostMultiplier(25); math.Abs(got-1.00) < 0.0005 {
		t.Errorf("rank 25: got %.4f, matches the OLD default — config was not read", got)
	}
}
