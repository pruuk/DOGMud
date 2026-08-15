package costs

import "github.com/GoMudEngine/GoMud/internal/configs"

// Input is everything Calc needs to price one action. Callers assemble it from
// the actor and the registry rather than passing a Character, which is what
// keeps this package a leaf: internal/costs must never import
// internal/characters, so the numbers come in already extracted.
type Input struct {
	Base      float64 // flat config base for the action
	Carried   float64 // actor's carried weight
	Capacity  float64 // actor's carry capacity
	Physical  bool    // physical actions take the encumbrance multiplier
	SkillRank int     // rank in the action's governing skill
	HasSkill  bool    // false for actions with no associated skill
	Modifier  float64 // per-action tuning knob
}

// Calc composes one action's cost:
//
//	cost = Base x encumbrance x skill x modifier
//
// Each multiplier is conditional. Encumbrance applies only to physical actions
// (a mental or social action costs the same laden or empty). The inverse-skill
// multiplier applies only when the action has a governing skill. The per-action
// Modifier applies only when it is positive, so a caller that leaves it at the
// zero value gets a neutral 1.0 rather than a free action.
//
// The clamp — CostTotalMultiplierMax, default 6.0 — applies to the PRODUCT of
// the three multipliers, NOT to each factor in turn. This matters. Clamping
// factors individually still lets a capacity-laden encumbrance of 5.0, a rank-0
// skill penalty of 1.10 and a defence premium of 1.25 stack to 6.875x, and a
// laden novice priced at nearly seven times base cannot pay for anything. That
// does not read to the player as "expensive"; it reads as autofail-everything,
// because every action they attempt is refused or partially charged. One ceiling
// on the composed product is the only place the guarantee can actually live.
//
// Base itself is outside the clamp: it is the authored price of the action, and
// capping it would silently flatten the difference between a cheap action and an
// expensive one.
func Calc(in Input) float64 {
	mult := 1.0

	if in.Physical {
		mult *= EncumbranceMultiplier(in.Carried, in.Capacity)
	}
	if in.HasSkill {
		mult *= SkillCostMultiplier(in.SkillRank)
	}
	if in.Modifier > 0 {
		mult *= in.Modifier
	}

	max := float64(configs.GetBalanceConfig().CostTotalMultiplierMax)
	if max <= 0 {
		max = 6.0
	}
	if mult > max {
		mult = max
	}

	return in.Base * mult
}
