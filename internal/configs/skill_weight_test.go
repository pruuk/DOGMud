package configs

import (
	"math"
	"testing"
)

func pinKnee(t *testing.T, sw, knee, above float64) {
	t.Helper()
	c := Config{}
	c.Balance.SkillWeight = ConfigFloat(sw)
	c.Balance.SkillWeightKneeRank = ConfigFloat(knee)
	c.Balance.SkillWeightAboveKnee = ConfigFloat(above)
	c.validated = true
	SetConfigForTest(t, c)
}

func TestWeightedSkill(t *testing.T) {
	cases := []struct {
		name            string
		sw, knee, above float64
		rank, want      float64
	}{
		{"off 0", 5, 0, 1, 0, 0},
		{"off 50", 5, 0, 1, 50, 250},
		{"off 89", 5, 0, 1, 89, 445},
		{"off 101", 5, 0, 1, 101, 505},
		{"knee 50", 5, 65, 1, 50, 250},
		{"knee at 65", 5, 65, 1, 65, 325},
		{"knee 66", 5, 65, 1, 66, 326},
		{"knee 89", 5, 65, 1, 89, 349},
		{"knee 101", 5, 65, 1, 101, 361},
		{"flat above", 5, 65, 0, 89, 325},
		{"fractional", 5, 65, 1, 70.5, 330.5},
		{"negative", 5, 65, 1, -3, 0},
		{"negative knee off", 5, 0, 1, -3, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pinKnee(t, c.sw, c.knee, c.above)
			if got := WeightedSkill(c.rank); math.Abs(got-c.want) > 1e-9 {
				t.Errorf("WeightedSkill(%v) = %v, want %v", c.rank, got, c.want)
			}
		})
	}
}

func TestSkillWeightKneeValidation(t *testing.T) {
	b := Balance{}
	b.SkillWeight = 5
	b.SkillWeightKneeRank = -4
	b.SkillWeightAboveKnee = -1
	b.CritDamageRankCap = -2
	b.validateMisc()
	b.validateCombat()
	if b.SkillWeightKneeRank != 0 {
		t.Errorf("negative knee should reset to 0, got %v", b.SkillWeightKneeRank)
	}
	if b.SkillWeightAboveKnee != 5 {
		t.Errorf("negative above-knee should reset to SkillWeight, got %v", b.SkillWeightAboveKnee)
	}
	if b.CritDamageRankCap != 0 {
		t.Errorf("negative crit cap should reset to 0, got %v", b.CritDamageRankCap)
	}
}
