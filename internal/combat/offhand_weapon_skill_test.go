package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// An offhand FIST is scored with unarmed-combat, not with whatever the main
// hand is holding.
//
// The bug: calcAttackScore took its skill term from GetCombatSkillLevel(),
// which resolves its tag from c.Equipment.Weapon ALONE, while calculateCombat
// swings every weapon in the plan through that one helper. So a character
// holding a sword and punching with the free hand rolled the PUNCH at their
// weapon-combat rank. This is a hit-chance bug, not a progression one: the
// score feeds the opposed contest.
//
// The fixture makes the two ranks maximally far apart so the assertion cannot
// pass by accident, and asserts the ORDER rather than an exact figure, because
// the stamina multiplier and position modifiers scale both sides.
func TestCalcAttackScore_OffhandFistUsesUnarmedNotTheMainHandWeaponSkill(t *testing.T) {
	attacker := characters.New()
	attacker.Stats.Dexterity.ValueAdj = 100
	attacker.SetSkill(string(skills.WeaponCombat), 40)
	attacker.SetSkill(string(skills.UnarmedCombat), 1)

	defender := characters.New()
	defender.Stats.Dexterity.ValueAdj = 100

	sword := items.Item{ItemId: 1, Spec: &items.ItemSpec{
		Subtype:          items.Slashing,
		DamageMultiplier: 1.0,
		SpeedMultiplier:  1.0,
	}}
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	swordScore := calcAttackScore(attacker, defender, sword, 0, ctx)
	fistScore := calcAttackScore(attacker, defender, items.Item{}, 0, ctx)

	if fistScore >= swordScore {
		t.Fatalf("a bare-handed swing scored %.3f against the sword's %.3f. "+
			"The fist must roll on unarmed-combat (rank 1), not on weapon-combat "+
			"(rank 40). Equal scores mean calcAttackScore is resolving the skill "+
			"from the main-hand slot again", fistScore, swordScore)
	}
}

// And the seam underneath it: the level is resolved from the weapon HANDED IN,
// not from the equipped main hand. Guards the character-side half directly, so
// a regression there is not diagnosed only through combat arithmetic.
func TestGetCombatSkillLevelFor_ResolvesFromTheWeaponPassedIn(t *testing.T) {
	c := characters.New()
	c.SetSkill(string(skills.WeaponCombat), 40)
	c.SetSkill(string(skills.UnarmedCombat), 7)
	c.SetSkill(string(skills.RangedCombat), 22)

	sword := items.Item{ItemId: 1, Spec: &items.ItemSpec{Subtype: items.Slashing}}
	bow := items.Item{ItemId: 2, Spec: &items.ItemSpec{Subtype: items.Shooting}}
	c.Equipment.Weapon = sword // the main hand is deliberately the SWORD throughout

	cases := []struct {
		name   string
		weapon items.Item
		want   int
	}{
		{"bare hands", items.Item{}, 7},
		{"sword", sword, 40},
		{"bow", bow, 22},
	}
	for _, tc := range cases {
		if got := c.GetCombatSkillLevelFor(tc.weapon); got != tc.want {
			t.Errorf("GetCombatSkillLevelFor(%s) = %d, want %d; it is reading the equipped main hand rather than the weapon passed in", tc.name, got, tc.want)
		}
	}

	// The no-argument form must still mean "the main hand", which is the sword.
	if got := c.GetCombatSkillLevel(); got != 40 {
		t.Errorf("GetCombatSkillLevel() = %d, want 40; the delegating form changed meaning", got)
	}
}
