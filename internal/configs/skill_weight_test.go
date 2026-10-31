package configs

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
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

func loadShippedBalance(t *testing.T) Balance {
	t.Helper()
	_, self, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(self), "..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	cfg, err := loadConfig(raw)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	return cfg.Balance
}

func TestSkillWeightKnee_ShippedValues(t *testing.T) {
	b := loadShippedBalance(t)
	if b.SkillWeight != 5.0 || b.SkillWeightKneeRank != 65 || b.SkillWeightAboveKnee != 1.0 ||
		b.CritDamagePerSkill != 0.025 || b.CritDamageRankCap != 65 {
		t.Errorf("shipped SkillWeight/KneeRank/AboveKnee/CritDamagePerSkill/CritDamageRankCap = %v/%v/%v/%v/%v, want 5/65/1/0.025/65",
			b.SkillWeight, b.SkillWeightKneeRank, b.SkillWeightAboveKnee, b.CritDamagePerSkill, b.CritDamageRankCap)
	}
}

func TestSkillWeightKnee_AbsentKeysKeepTodaysBehaviour(t *testing.T) {
	b := loadAndValidate(t, "Balance:\n  StandMinStamina: 0.15\n")
	if b.SkillWeightKneeRank != 0 || b.CritDamageRankCap != 0 {
		t.Errorf("absent knee and cap must load 0/0 (off), got %v/%v", b.SkillWeightKneeRank, b.CritDamageRankCap)
	}
	if b.CritDamagePerSkill != 0.05 {
		t.Errorf("absent CritDamagePerSkill must load the 0.05 Go default, got %v", b.CritDamagePerSkill)
	}
	// With the knee off, SkillWeightAboveKnee is never read.
	pinKnee(t, float64(b.SkillWeight), float64(b.SkillWeightKneeRank), float64(b.SkillWeightAboveKnee))
	if got, want := WeightedSkill(89), 89*float64(b.SkillWeight); got != want {
		t.Errorf("WeightedSkill(89) with absent keys = %v, want the straight line %v", got, want)
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
