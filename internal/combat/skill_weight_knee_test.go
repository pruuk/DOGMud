package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// Skill weight knee (owner 2026-10-31): with the knee on (SkillWeight 5, knee
// 65, above-knee 1.0) a rank-89 score drops by exactly 96 against the
// straight line (445 -> 349), and a rank below the knee does not move.

func setKneeForTest(t *testing.T, on bool) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.SkillWeight = 5
	cfg.Balance.SkillWeightKneeRank = 0
	cfg.Balance.SkillWeightAboveKnee = 1
	if on {
		cfg.Balance.SkillWeightKneeRank = 65
	}
	configs.SetConfigForTest(t, cfg)
}

func TestCalcAttackScore_KneeCutsRankAboveTheKnee(t *testing.T) {
	score := func(t *testing.T, rank int, on bool) float64 {
		setKneeForTest(t, on)
		attacker := characters.New()
		attacker.Stats.Dexterity.ValueAdj = 100
		attacker.SetSkill(string(skills.UnarmedCombat), rank)
		defender := characters.New()
		ctx := combatContext{sourceSight: messaging.SightFull, targetSight: messaging.SightFull}
		return calcAttackScore(attacker, defender, items.Item{}, 0, ctx)
	}

	if d := score(t, 89, false) - score(t, 89, true); math.Abs(d-96) > 1e-6 {
		t.Errorf("rank 89 attack score dropped by %v with the knee on, want 96", d)
	}
	if d := score(t, 50, false) - score(t, 50, true); math.Abs(d) > 1e-6 {
		t.Errorf("rank 50 attack score moved by %v with the knee on, want 0", d)
	}
}

func TestAttackSideScore_KneeCutsRankAboveTheKnee(t *testing.T) {
	side := func(t *testing.T, rank int, on bool) float64 {
		setKneeForTest(t, on)
		return AttackSide{Stat: 100, SkillRank: rank}.score()
	}

	if d := side(t, 89, false) - side(t, 89, true); math.Abs(d-96) > 1e-9 {
		t.Errorf("rank 89 side score dropped by %v with the knee on, want 96", d)
	}
	if d := side(t, 50, false) - side(t, 50, true); d != 0 {
		t.Errorf("rank 50 side score moved by %v with the knee on, want 0", d)
	}
}
