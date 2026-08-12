package hooks

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// Chunk 5.11g — skill-scaled crit damage, magical channel.
//
// A spell crit bypasses mitigation, exactly like a melee crit did before this
// chunk, so against an UNMITIGATED target a crit and a normal hit used to land
// on the same mean. That equality is what makes this test able to isolate the
// multiplier: with mitigation held at zero, crit/normal is the multiplier and
// nothing else. Every other term in calcSpellDamageForCharacter — willpower,
// SkillMultiplier, weapon spell bonus, conviction penalty — is common to both
// branches and cancels.

// spellTestCaster builds a caster whose only interesting property is its
// spellcasting rank. Conviction is kept full so ResourceMultiplier contributes
// a flat 1.0 to both branches.
func spellTestCaster(spellcasting int) *characters.Character {
	c := &characters.Character{}
	c.Skills = map[string]int{string(skills.Spellcasting): spellcasting}
	// Base is the input; Value and ValueAdj are DERIVED from it by Recalculate.
	// CalcRawDamage reads ValueAdj, so assigning Value directly (and skipping
	// Recalculate) leaves raw damage at 0 — both branches then floor at 1
	// damage and the ratio under test comes out a meaningless 1.0.
	c.Stats.Willpower.Base = 100
	c.Stats.Willpower.Recalculate()
	c.Conviction = 100
	c.ConvictionMax.Value = 100
	return c
}

func meanSpellDamage(t *testing.T, sd *spells.SpellData, caster, target *characters.Character, isCrit bool, samples int) float64 {
	t.Helper()
	total := 0.0
	for i := 0; i < samples; i++ {
		total += float64(calcSpellDamageForCharacter(sd, caster, target, 0, isCrit))
	}
	return total / float64(samples)
}

func TestCalcSpellDamage_CritScalesWithSpellcastingSkill(t *testing.T) {
	const samples = 20000

	// TargetDefenseType "" yields mitPct 0, so the normal branch is unmitigated
	// and equals the crit branch's pre-multiplier mean.
	sd := &spells.SpellData{SpellId: "test-harm", DamageMultiplier: 1.0}
	target := &characters.Character{}

	for _, rank := range []int{1, 30, 60} {
		caster := spellTestCaster(rank)

		normal := meanSpellDamage(t, sd, caster, target, false, samples)
		crit := meanSpellDamage(t, sd, caster, target, true, samples)

		want := combat.CritDamageMultiplier(rank)
		got := crit / normal
		if math.Abs(got-want) > 0.05*want {
			t.Errorf("spellcasting rank %d: crit/normal = %f, want ~%f", rank, got, want)
		}
	}
}

func TestCalcSpellDamage_NormalHitUnaffectedBySkillCritMultiplier(t *testing.T) {
	// Guards the leak in the other direction: the crit multiplier must not
	// reach the normal-damage branch. Normal damage still rises with skill via
	// SkillMultiplier, but that curve clamps at the soft cap (50), so rank 60
	// and rank 100 must produce the SAME normal damage. If the linear crit
	// multiplier leaked in, they would not.
	const samples = 20000

	sd := &spells.SpellData{SpellId: "test-harm", DamageMultiplier: 1.0}
	target := &characters.Character{}

	at60 := meanSpellDamage(t, sd, spellTestCaster(60), target, false, samples)
	at100 := meanSpellDamage(t, sd, spellTestCaster(100), target, false, samples)

	if math.Abs(at100-at60) > 0.05*at60 {
		t.Errorf("normal spell damage rank 60 = %f vs rank 100 = %f; crit multiplier leaked into normal hits", at60, at100)
	}
}
