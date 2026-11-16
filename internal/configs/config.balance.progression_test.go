package configs

import "testing"

// An inverted rank pair (cap at or below mid) must be corrected at
// validation time, not silently repaired per call by costs.SkillCostMultiplier.
func TestValidateProgression_CostSkillCapRankBelowMidRankIsCorrected(t *testing.T) {
	b := &Balance{CostSkillMidRank: 100, CostSkillCapRank: 50}
	b.validateProgression()
	if int(b.CostSkillCapRank) <= int(b.CostSkillMidRank) {
		t.Errorf("CostSkillCapRank = %d, CostSkillMidRank = %d; cap must be strictly greater than mid after validation",
			int(b.CostSkillCapRank), int(b.CostSkillMidRank))
	}
}

// Equal rank pair (degenerate, not just inverted) must also be corrected.
func TestValidateProgression_CostSkillCapRankEqualToMidRankIsCorrected(t *testing.T) {
	b := &Balance{CostSkillMidRank: 30, CostSkillCapRank: 30}
	b.validateProgression()
	if int(b.CostSkillCapRank) <= int(b.CostSkillMidRank) {
		t.Errorf("CostSkillCapRank = %d, CostSkillMidRank = %d; cap must be strictly greater than mid after validation",
			int(b.CostSkillCapRank), int(b.CostSkillMidRank))
	}
}

// A well-formed pair must be left exactly as authored.
func TestValidateProgression_CostSkillRankPairPreservedWhenValid(t *testing.T) {
	b := &Balance{CostSkillMidRank: 25, CostSkillCapRank: 100}
	b.validateProgression()
	if int(b.CostSkillMidRank) != 25 {
		t.Errorf("CostSkillMidRank = %d, want 25 (explicit value preserved)", int(b.CostSkillMidRank))
	}
	if int(b.CostSkillCapRank) != 100 {
		t.Errorf("CostSkillCapRank = %d, want 100 (explicit value preserved)", int(b.CostSkillCapRank))
	}
}

// A top below 1.0 inverts the curve: loading up would make every physical
// action cheaper. It is corrected to the default, as an absent key (0) is.
func TestValidateProgression_CostEncumbranceMaxBelowOneIsCorrected(t *testing.T) {
	for _, bad := range []float64{0, 0.5} {
		b := &Balance{CostEncumbranceShape: 2.5, CostEncumbranceMax: ConfigFloat(bad)}
		b.validateProgression()
		if float64(b.CostEncumbranceMax) != 5.5 {
			t.Errorf("CostEncumbranceMax %.2f validated to %.2f, want the default 5.5", bad, float64(b.CostEncumbranceMax))
		}
	}
}

// A shape at or below 0 makes the curve flat or undefined; it falls back to
// the default.
func TestValidateProgression_CostEncumbranceShapeNotPositiveIsCorrected(t *testing.T) {
	for _, bad := range []float64{0, -1} {
		b := &Balance{CostEncumbranceShape: ConfigFloat(bad), CostEncumbranceMax: 5.5}
		b.validateProgression()
		if float64(b.CostEncumbranceShape) != 2.5 {
			t.Errorf("CostEncumbranceShape %.2f validated to %.2f, want the default 2.5", bad, float64(b.CostEncumbranceShape))
		}
	}
}

// A well-formed pair is left exactly as authored, including a top of exactly
// 1 (a flat curve, which tests use to price load out).
func TestValidateProgression_CostEncumbrancePairPreservedWhenValid(t *testing.T) {
	for _, c := range []struct{ shape, max float64 }{{2.5, 5.5}, {1, 1}, {4, 8}} {
		b := &Balance{CostEncumbranceShape: ConfigFloat(c.shape), CostEncumbranceMax: ConfigFloat(c.max)}
		b.validateProgression()
		if float64(b.CostEncumbranceShape) != c.shape || float64(b.CostEncumbranceMax) != c.max {
			t.Errorf("(%.2f, %.2f) validated to (%.2f, %.2f), want them preserved", c.shape, c.max,
				float64(b.CostEncumbranceShape), float64(b.CostEncumbranceMax))
		}
	}
}

// Zero must SURVIVE validation on both knobs. The neighbouring progression
// knobs validate with `<= 0`, which silently replaces a deliberate 0 with the
// default. These two are documented off-switches, so they validate on `< 0`.
func TestProgressionBonuses_ZeroIsLegal(t *testing.T) {
	b := &Balance{CritProgressionBonus: 0, ObservedCritProgressionBonus: 0}
	b.validateProgression()
	if float64(b.CritProgressionBonus) != 0 {
		t.Errorf("CritProgressionBonus = %.2f, want 0 (explicit off-switch preserved)", float64(b.CritProgressionBonus))
	}
	if float64(b.ObservedCritProgressionBonus) != 0 {
		t.Errorf("ObservedCritProgressionBonus = %.2f, want 0 (explicit off-switch preserved)", float64(b.ObservedCritProgressionBonus))
	}
}

// A negative value is not a meaningful configuration (unlike 0, which is a
// deliberate off-switch) and must be corrected back to the documented default.
func TestProgressionBonuses_NegativeIsCorrected(t *testing.T) {
	b := &Balance{CritProgressionBonus: -1, ObservedCritProgressionBonus: -1}
	b.validateProgression()
	if float64(b.CritProgressionBonus) != 2.0 {
		t.Errorf("CritProgressionBonus = %.2f, want 2.0 (default after negative correction)", float64(b.CritProgressionBonus))
	}
	if float64(b.ObservedCritProgressionBonus) != 0.5 {
		t.Errorf("ObservedCritProgressionBonus = %.2f, want 0.5 (default after negative correction)", float64(b.ObservedCritProgressionBonus))
	}
}
