package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// These literals are derived directly from each documented defence formula at
// SkillWeight 2.5. A regression that removes anything except the governing
// skill addend from the short-funded score fails the no-skill column, while a
// regression that changes the legacy score fails both the full and wrapper
// columns.
func TestGetDefenseScoreFor_OmitsOnlyTheGoverningSkillAddend(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.SkillWeight = 2.5
	configs.SetConfigForTest(t, cfg)

	c := New()
	c.Stats.Dexterity.Base = 120
	c.Stats.Dexterity.Recalculate()
	c.Stats.Strength.Base = 80
	c.Stats.Strength.Recalculate()
	c.Stats.Willpower.Base = 150
	c.Stats.Willpower.Recalculate()
	c.SetSkill(string(skills.UnarmedCombat), 7)
	c.SetSkill(string(skills.WeaponCombat), 11)
	c.SetSkill(string(skills.Spellcasting), 13)
	c.SetSkill(string(skills.Rhetoric), 17)
	c.Equipment.Weapon = items.Item{ItemId: 1, Spec: &items.ItemSpec{ParryRating: 9}}
	c.Equipment.Offhand = items.Item{ItemId: 2, Spec: &items.ItemSpec{
		Type:        items.Offhand,
		BlockRating: 17,
	}}

	tests := []struct {
		defence string
		without float64
		with    float64
	}{
		{DefenseDodge, 120, 137.5},
		{DefenseParry, 129, 156.5},
		{DefenseBlock, 117, 144.5},
		{DefenseQuell, 150, 182.5},
		{DefenseDefy, 150, 192.5},
	}

	for _, tc := range tests {
		t.Run(tc.defence, func(t *testing.T) {
			if got := c.GetDefenseScoreFor(tc.defence, false); math.Abs(got-tc.without) > 1e-9 {
				t.Fatalf("GetDefenseScoreFor(%q, false) = %.4f, want %.4f", tc.defence, got, tc.without)
			}
			if got := c.GetDefenseScoreFor(tc.defence, true); math.Abs(got-tc.with) > 1e-9 {
				t.Fatalf("GetDefenseScoreFor(%q, true) = %.4f, want %.4f", tc.defence, got, tc.with)
			}
			if got := c.GetDefenseScore(tc.defence); math.Abs(got-tc.with) > 1e-9 {
				t.Fatalf("GetDefenseScore(%q) = %.4f, want legacy full score %.4f", tc.defence, got, tc.with)
			}
		})
	}
}

func TestGetDefenseScoreFor_UnknownDefenceRemainsZero(t *testing.T) {
	c := New()
	if got := c.GetDefenseScoreFor("not-a-defence", false); got != 0 {
		t.Fatalf("unknown no-skill defence score = %.4f, want 0", got)
	}
	if got := c.GetDefenseScoreFor("not-a-defence", true); got != 0 {
		t.Fatalf("unknown full defence score = %.4f, want 0", got)
	}
}
