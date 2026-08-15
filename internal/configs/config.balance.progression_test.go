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
