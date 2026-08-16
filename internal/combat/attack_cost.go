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
// zero CostResult. It is also not Short: nothing was demanded.
//
// THE ZERO-SWING GUARD IS DEFENSIVE AGAINST A STATE NOTHING CURRENTLY REACHES.
// An earlier version of this comment claimed zero swings was "a real state --
// the attacker had no weapons, or the defender was already gone"; neither is
// true today. collectAttackWeapons ends in an unconditional
// `if len(attackWeapons) == 0 { append(fist) }` fallback, so it never returns an
// empty list; calcSwingCount floors its result at 1; and the swing loop in
// calculateCombat contains no break or early return, so it cannot stop short on
// a defender who died mid-round. All four wrappers pass
// attackResult.SwingsThrown straight out of calculateCombat, so the argument
// here is always at least 1.
//
// Keep the guard. Four separate edits would make it live again: removing that
// weapons fallback, removing calcSwingCount's floor, adding the death break the
// swing loop does not yet have, or a future caller computing its own count (an
// AoE splitting a round's swings across targets is the obvious one). A negative
// count is a caller bug either way and must never credit stamina back.
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
