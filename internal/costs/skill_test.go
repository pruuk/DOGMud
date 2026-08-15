package costs

import (
	"math"
	"testing"
)

// The band is deliberately asymmetric: a gentle penalty for the untrained, a
// large reward for mastery.
func TestSkillMultiplierBand(t *testing.T) {
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
		got := SkillMultiplier(c.rank)
		if math.Abs(got-c.want) > 0.0005 {
			t.Errorf("rank %d: got %.4f, want %.4f", c.rank, got, c.want)
		}
	}
}

// Monotonically decreasing: more skill must never cost more.
func TestSkillMultiplierIsMonotonic(t *testing.T) {
	prev := SkillMultiplier(0)
	for r := 1; r <= 120; r++ {
		got := SkillMultiplier(r)
		if got > prev+0.0001 {
			t.Fatalf("rank %d: multiplier rose from %.4f to %.4f", r, prev, got)
		}
		prev = got
	}
}

// A negative rank cannot be cheaper than rank zero.
func TestSkillMultiplierNegativeRankClampsToZero(t *testing.T) {
	if SkillMultiplier(-5) != SkillMultiplier(0) {
		t.Fatalf("negative rank must clamp to the rank-0 penalty")
	}
}

// Midpoint of each segment, to pin the shape rather than only its endpoints.
func TestSkillMultiplierSegmentMidpoints(t *testing.T) {
	// Halfway from 0 to the neutral rank: halfway from 1.10 to 1.00.
	if got := SkillMultiplier(12); math.Abs(got-1.052) > 0.002 {
		t.Errorf("rank 12: got %.4f, want about 1.052", got)
	}
	// Halfway from neutral to the cap: halfway from 1.00 to 0.40.
	if got := SkillMultiplier(62); math.Abs(got-0.704) > 0.002 {
		t.Errorf("rank 62: got %.4f, want about 0.704", got)
	}
}
