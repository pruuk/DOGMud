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

// A knee multiplier ABOVE the maximum makes costs.EncumbranceMultiplier's
// second segment run backwards: 3.00 at the knee falling to 1.50 at capacity,
// so loading further toward capacity would get CHEAPER. Both fields are
// individually positive, so the old per-field guards passed this through.
func TestValidateProgression_CostEncumbranceMaxBelowKneeMultIsCorrected(t *testing.T) {
	b := &Balance{CostEncumbranceKneeMult: 3.0, CostEncumbranceMax: 1.5}
	b.validateProgression()
	if float64(b.CostEncumbranceMax) <= float64(b.CostEncumbranceKneeMult) {
		t.Errorf("CostEncumbranceMax = %.2f, CostEncumbranceKneeMult = %.2f; max must be strictly greater than the knee multiplier after validation",
			float64(b.CostEncumbranceMax), float64(b.CostEncumbranceKneeMult))
	}
}

// Equal pair (degenerate, not just inverted) must also be corrected: it flattens
// the steep segment to a constant, so the curve stops responding to load at all
// past the knee.
func TestValidateProgression_CostEncumbranceMaxEqualToKneeMultIsCorrected(t *testing.T) {
	b := &Balance{CostEncumbranceKneeMult: 2.0, CostEncumbranceMax: 2.0}
	b.validateProgression()
	if float64(b.CostEncumbranceMax) <= float64(b.CostEncumbranceKneeMult) {
		t.Errorf("CostEncumbranceMax = %.2f, CostEncumbranceKneeMult = %.2f; max must be strictly greater than the knee multiplier after validation",
			float64(b.CostEncumbranceMax), float64(b.CostEncumbranceKneeMult))
	}
}

// A well-formed encumbrance pair must be left exactly as authored.
func TestValidateProgression_CostEncumbrancePairPreservedWhenValid(t *testing.T) {
	b := &Balance{CostEncumbranceKneeMult: 1.5, CostEncumbranceMax: 5.0}
	b.validateProgression()
	if float64(b.CostEncumbranceKneeMult) != 1.5 {
		t.Errorf("CostEncumbranceKneeMult = %.2f, want 1.50 (explicit value preserved)", float64(b.CostEncumbranceKneeMult))
	}
	if float64(b.CostEncumbranceMax) != 5.0 {
		t.Errorf("CostEncumbranceMax = %.2f, want 5.00 (explicit value preserved)", float64(b.CostEncumbranceMax))
	}
}

// A knee multiplier BELOW 1.0 inverts the first segment: the curve would run
// from 1.0 at empty down to the knee, making a loaded character cheaper to act
// than an empty one. Positive, so the old `<= 0` guard let it through.
func TestValidateProgression_CostEncumbranceKneeMultBelowOneIsCorrected(t *testing.T) {
	b := &Balance{CostEncumbranceKneeMult: 0.5, CostEncumbranceMax: 5.0}
	b.validateProgression()
	if float64(b.CostEncumbranceKneeMult) < 1.0 {
		t.Errorf("CostEncumbranceKneeMult = %.2f; a value below 1.0 inverts the gentle segment and must be corrected at validation",
			float64(b.CostEncumbranceKneeMult))
	}
	if float64(b.CostEncumbranceMax) <= float64(b.CostEncumbranceKneeMult) {
		t.Errorf("CostEncumbranceMax = %.2f is not above the corrected knee multiplier %.2f",
			float64(b.CostEncumbranceMax), float64(b.CostEncumbranceKneeMult))
	}
}
