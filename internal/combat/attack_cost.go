package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
)

// attackCostPerSwing prices ONE swing through the shared U7 formula:
//
//	AttackBaseStaminaCost x encumbrance x inverse-skill x AttackCostModifier
//
// Same composition the five defences use, from the same registry, so the two
// sides of an exchange cannot drift apart the way they did when the attacker
// read a per-weapon item field and the defender read a config base.
//
// The skill rank comes from GetCombatSkillLevel, which picks the skill matching
// the EQUIPPED weapon (weapon-combat, unarmed-combat or ranged-combat) rather
// than the registry's nominal skills.WeaponCombat. That is deliberate: an
// unarmed brawler's practice should discount their swings, and reading
// weapon-combat off them would charge a master the novice price. It also returns
// a minimum of 1, so a fresh character lands on the rank-1 multiplier (1.096)
// rather than the rank-0 one (1.10) -- a four-thousandths difference, noted only
// so the arithmetic in the tests reads correctly.
//
// costs.SkillCostMultiplier -- NOT combat.SkillMultiplier, which lives in this
// same package with an identical signature and scales DAMAGE UPWARD. The mix-up
// compiles clean. costs.Calc does the reading here, which is the safe way round.
func attackCostPerSwing(attacker *characters.Character) float64 {
	bal := configs.GetBalanceConfig()
	spec := costs.SpecFor(costs.ActionAttack)

	return costs.Calc(costs.Input{
		Base:      float64(bal.AttackBaseStaminaCost),
		Carried:   attacker.GetCarriedWeight(),
		Capacity:  attacker.CarryCapacity(),
		Physical:  spec.Physical,
		SkillRank: attacker.GetCombatSkillLevel(),
		HasSkill:  spec.HasSkill,
		Modifier:  float64(bal.AttackCostModifier),
	})
}

// ChargeAttackCost charges an attacker for the swings they actually threw this
// round, and reports what the charge did.
//
// Before U7 Task 7 the four combat wrappers each called DeductAttackStamina
// exactly ONCE per round, whatever the round contained: a six-armed character
// throwing twelve swings paid the same as someone throwing one, while the
// defender on the other side paid on every one of those twelve incoming blows.
// That asymmetry is the single largest reason offence was effectively free next
// to defence. Cost now scales with swings, so both sides are priced per action.
//
// Charged through ApplyCostFloat, not ApplyCostPartial. The per-swing cost is a
// product of four config floats and is rarely a whole number; rounding each
// round to an integer would eat the encumbrance and skill terms this task exists
// to introduce. ApplyCostFloat banks the sub-integer remainder so the average
// converges. It also rejects NaN and infinity, which matters here because
// swings multiplies whatever costs.Calc returns.
//
// A nil attacker or a non-positive swing count charges NOTHING and reports the
// zero CostResult. Zero swings is a real state -- a round where the attacker had
// no weapons to swing, or the defender was already gone -- and must not be
// charged as if it were one. It is also not Short: nothing was demanded.
//
// The returned CostResult is what U8 reads to strip the skill term from an
// attacker who could not pay in full. Discarding it is safe today and will not
// be later.
func ChargeAttackCost(attacker *characters.Character, swings int) characters.CostResult {
	if attacker == nil || swings <= 0 {
		return characters.CostResult{}
	}

	return attacker.ApplyCostFloat(characters.PoolStamina, attackCostPerSwing(attacker)*float64(swings))
}
