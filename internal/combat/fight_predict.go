package combat

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// FightPrediction is one character's read of a one-on-one fight against
// another as both stand now (#301, layer 2). Damage rates are net: each
// side's output less the other side's heals, never negative.
type FightPrediction struct {
	YourDamagePerRound  float64 // net damage you deal them each round
	TheirDamagePerRound float64 // net damage they deal you each round
	RoundsToKillThem    float64 // math.Inf(1) when YourDamagePerRound is 0
	RoundsToKillYou     float64 // math.Inf(1) when TheirDamagePerRound is 0
	Cost                float64 // share of your current health the fight takes
	Dent                float64 // share of their current health you take before you drop
}

// PredictFight reads you against them from layer 1: melee and spell output
// each way, then each side's lifesteal (on its melee), Regenerating record and
// health ticks (heal-over-time less damage-over-time).
func PredictFight(you, them *characters.Character) FightPrediction {
	yourMelee := meleeDamagePerRound(you, them, sourceTypeOf(you))
	theirMelee := meleeDamagePerRound(them, you, sourceTypeOf(them))
	yourOut := yourMelee + spellDamagePerRound(you, them)
	theirOut := theirMelee + spellDamagePerRound(them, you)

	p := FightPrediction{
		YourDamagePerRound:  nonNegative(yourOut - healPerRound(them, theirMelee)),
		TheirDamagePerRound: nonNegative(theirOut - healPerRound(you, yourMelee)),
	}
	finish(&p, float64(max(you.Health, 1)), float64(max(them.Health, 1)))
	return p
}

// healPerRound is what c regains each combat round: lifesteal on its own
// melee damage, a heal spell's Regenerating record, and health ticks
// (negative when damage-over-time outweighs heal-over-time). Lifesteal counts
// only when lifesteal_pct is above zero, the same gate live combat uses in
// NewRound_DoCombat_unified.go.
func healPerRound(c *characters.Character, meleeOut float64) float64 {
	h := 0.0
	if pct := c.StatMod("lifesteal_pct"); pct > 0 {
		h = meleeOut * float64(pct) / 100.0
	}
	h += c.CombatHealthRegenPerRound()
	h += c.Conditions.PoolTickPerRound("health")
	return h
}

// finish derives rounds, cost and dent from the two net rates. Zero damage is
// infinite rounds, handled here rather than by a division by zero.
func finish(p *FightPrediction, yourHealth, theirHealth float64) {
	p.RoundsToKillThem = roundsToKill(theirHealth, p.YourDamagePerRound)
	p.RoundsToKillYou = roundsToKill(yourHealth, p.TheirDamagePerRound)

	switch {
	case p.TheirDamagePerRound == 0:
		p.Cost = 0
	case math.IsInf(p.RoundsToKillThem, 1):
		p.Cost = math.Inf(1)
	default:
		p.Cost = p.RoundsToKillThem * p.TheirDamagePerRound / yourHealth
	}

	switch {
	case p.YourDamagePerRound == 0:
		p.Dent = 0
	case math.IsInf(p.RoundsToKillYou, 1):
		p.Dent = math.Inf(1)
	default:
		p.Dent = p.RoundsToKillYou * p.YourDamagePerRound / theirHealth
	}
}

// nonNegative floors x at zero; !(x > 0) also sends a NaN from layer 1 to 0.
func nonNegative(x float64) float64 {
	if !(x > 0) {
		return 0
	}
	return x
}

func roundsToKill(health, perRound float64) float64 {
	if perRound <= 0 {
		return math.Inf(1)
	}
	return health / perRound
}
