// Package costs prices every action in the game through one formula:
//
//	cost = base x encumbrance(actor) x skill(actor) x modifier(action)
//
// Encumbrance applies to physical actions only; the skill multiplier applies to
// any action with an associated skill, mental and social included.
//
// This package is a config-only LEAF. It must never import internal/characters:
// callers assemble the inputs and pass plain numbers in.
package costs

import "github.com/GoMudEngine/GoMud/internal/configs"

// SkillMultiplier returns the cost multiplier for a given skill rank. It runs
// INVERSE to skill: a practised fighter spends less stamina (or conviction,
// or any other resource priced through this package) on the same action than
// an untrained one.
//
// This is NOT combat.SkillMultiplier. That function is a sqrt curve scaling
// DAMAGE UPWARD with skill rank. This one is two straight-line segments
// scaling COST DOWNWARD with skill rank. Same name, opposite direction,
// different job — do not merge them and do not reuse one in place of the
// other.
//
// The curve is two linear segments joined at the neutral rank
// (CostSkillMidRank, default 25, multiplier 1.00):
//
//   - rank 0                       -> CostSkillMultAtZero (default 1.10)
//   - rank CostSkillMidRank        -> CostSkillMultAtMid   (default 1.00)
//   - rank CostSkillCapRank+       -> CostSkillMultAtCap   (default 0.40)
//
// clamped flat below rank 0 and at/above CostSkillCapRank.
//
// The band is deliberately asymmetric: a wide penalty at rank 0 would drain a
// new player's resources in their first exchange, while a deep discount at
// the cap is what makes grinding a skill toward mastery worth it.
func SkillMultiplier(rank int) float64 {
	bal := configs.GetBalanceConfig()

	atZero := float64(bal.CostSkillMultAtZero)
	atMid := float64(bal.CostSkillMultAtMid)
	atCap := float64(bal.CostSkillMultAtCap)
	midRank := int(bal.CostSkillMidRank)
	capRank := int(bal.CostSkillCapRank)

	// Guard against a misconfigured knob (zero, negative, or inverted ranks)
	// dividing by zero below.
	if midRank <= 0 {
		midRank = 25
	}
	if capRank <= midRank {
		capRank = midRank + 1
	}

	if rank <= 0 {
		return atZero
	}
	if rank <= midRank {
		// Linear interpolation from (0, atZero) to (midRank, atMid).
		t := float64(rank) / float64(midRank)
		return atZero + (atMid-atZero)*t
	}
	if rank >= capRank {
		return atCap
	}
	// Linear interpolation from (midRank, atMid) to (capRank, atCap).
	t := float64(rank-midRank) / float64(capRank-midRank)
	return atMid + (atCap-atMid)*t
}
