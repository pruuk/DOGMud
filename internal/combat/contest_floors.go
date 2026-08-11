package combat

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
)

// Contest floors for combat maneuvers (roadmap chunk 5.9c).
//
// The floor value tracks the COST OF A SINGLE FAILURE, which is the principle
// the whole 5.9 arc settled on:
//
//	melee swing        fraction of a round, many per fight   0.15
//	maneuver / spell   consumes the WHOLE round              0.05
//	out-of-combat      one shot, plus a consequence          0.05
//
// So maneuvers sit with spells, not with the swing they superficially resemble.
// A grapple that fails costs the round; a swing that misses costs a slice of one.
func ManeuverFloors() (hit, resist float64) {
	bal := configs.GetBalanceConfig()
	return float64(bal.MinManeuverHitChance), float64(bal.MinManeuverResistChance)
}

// SpellFloors is the spell pair, for combat-side abilities that resolve against
// a spell rather than against a maneuver -- TrySpellDeflection is one. Using the
// maneuver pair there would be a coincidence of value, not a shared rule.
func SpellFloors() (hit, resist float64) {
	bal := configs.GetBalanceConfig()
	return float64(bal.MinSpellHitChance), float64(bal.MinSpellResistChance)
}
