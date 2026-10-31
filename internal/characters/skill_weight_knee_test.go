package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Skill weight knee (owner 2026-10-31): with the knee on (SkillWeight 5, knee
// 65, above-knee 1.0) a rank-89 defence score drops by exactly 96 against the
// straight line (445 -> 349), and a rank below the knee does not move.
func TestGetDefenseScoreFor_KneeCutsRankAboveTheKnee(t *testing.T) {
	score := func(t *testing.T, rank int, on bool) float64 {
		cfg := configs.GetConfig()
		cfg.Balance.SkillWeight = 5
		cfg.Balance.SkillWeightKneeRank = 0
		cfg.Balance.SkillWeightAboveKnee = 1
		if on {
			cfg.Balance.SkillWeightKneeRank = 65
		}
		configs.SetConfigForTest(t, cfg)

		c := New()
		c.Stats.Willpower.Base = 100
		c.Stats.Willpower.Recalculate()
		c.SetSkill(string(skills.Spellcasting), rank)
		return c.GetDefenseScoreFor(combatvocab.DefenceQuell, true)
	}

	if d := score(t, 89, false) - score(t, 89, true); math.Abs(d-96) > 1e-9 {
		t.Errorf("rank 89 defence score dropped by %v with the knee on, want 96", d)
	}
	if d := score(t, 50, false) - score(t, 50, true); d != 0 {
		t.Errorf("rank 50 defence score moved by %v with the knee on, want 0", d)
	}
}
