package characters

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// NextPowerOfTwo returns the smallest power of 2 >= n (minimum 2).
func NextPowerOfTwo(n int) int {
	if n <= 2 {
		return 2
	}
	p := 2
	for p < n {
		p <<= 1
	}
	return p
}

// CalcFoldsPerRound returns max(1, round((primaryStat + skillLevel*SpellFoldsSkillFactor) / 100)).
// Example: stat=50, skill=2 → round(100/100)=1; stat=100, skill=4 → round(200/100)=2
func CalcFoldsPerRound(primaryStat, skillLevel int) int {
	b := configs.GetBalanceConfig()
	skillFactor := int(b.SpellFoldsSkillFactor)
	weightedSkill := int(math.Round(float64(skillLevel) * float64(b.SkillWeight)))
	result := int(math.Round(float64(primaryStat+weightedSkill*skillFactor) / 100.0))
	if result < 1 {
		return 1
	}
	return result
}

// CalcInitiationChance was deleted in roadmap U0. It returned
// clamp(base + willpower/divisor + level*factor, 10, 95), and the clamp was the
// problem: a maxed caster's computed value was 1372 against a ceiling of 95, so
// no amount of skill could beat it and every caster failed one cast in twenty
// forever. Concentration break already covers the intent and does respond to
// skill. Do not reintroduce a flat initiation gate.
//
// NOTE: SpellInitiationWillpowerDivisor survives despite its name because
// CalcConcentrationChance below also reads it. Roadmap U9 removes it when
// concentration is rebuilt as a proper contest.

// CalcConcentrationChance returns the % chance to maintain concentration
// when struck for damagePct percent of max health.
// Formula: clamp(base + willpower/divisor - damagePct, 5, 95)
func CalcConcentrationChance(willpower, damagePct int) int {
	b := configs.GetBalanceConfig()
	base := int(b.SpellConcentrationBase)
	divisor := int(b.SpellInitiationWillpowerDivisor)
	chance := base + willpower/divisor - damagePct
	if chance < 5 {
		return 5
	}
	if chance > 95 {
		return 95
	}
	return chance
}

// CalcSpellAttack returns the attacker score for a spell contest: the mean the
// contest core rolls the caster's side against. U2 moved the spell sites off
// dice.OpposedRoll* and onto that core (reached then via contest.RunWithFloors
// plus a pair of private floor accessors in internal/hooks); U3 deleted those
// accessors, and U6 collapsed the wrappers that replaced them, so the core is
// now reached through combat.RunContest.
// Higher willpower and spellcasting level increase spell offense.
// Formula: willpower + round(spellcastingLevel * SkillWeight) * SpellAttackSkillFactor
func CalcSpellAttack(willpower, spellcastingLevel int) float64 {
	b := configs.GetBalanceConfig()
	factor := int(b.SpellAttackSkillFactor)
	weightedSkill := int(math.Round(float64(spellcastingLevel) * float64(b.SkillWeight)))
	return float64(willpower + weightedSkill*factor)
}
