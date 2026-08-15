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

// SkillCostMultiplier returns the cost multiplier for a given skill rank. It
// runs INVERSE to skill: a practised fighter spends less stamina (or
// conviction, or any other resource priced through this package) on the same
// action than an untrained one.
//
// Named SkillCostMultiplier, not SkillMultiplier, on purpose:
// combat.SkillMultiplier already exists with an IDENTICAL signature
// (func(rank int) float64) and scales DAMAGE UPWARD via a sqrt curve — the
// opposite direction from this one. Several U7 call sites live inside
// package combat, where an unqualified SkillMultiplier(...) call resolves to
// that function, not this one. At rank 100 the two return 3.0 and 0.40
// respectively, so a mix-up compiles clean, passes vet, and lands as a 7.5x
// cost error in the wrong direction. Do not rename this back to match, and
// do not merge the two functions — they are two straight-line segments
// scaling COST DOWNWARD versus a sqrt curve scaling DAMAGE UPWARD, different
// jobs entirely.
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
func SkillCostMultiplier(rank int) float64 {
	bal := configs.GetBalanceConfig()

	atZero := float64(bal.CostSkillMultAtZero)
	atMid := float64(bal.CostSkillMultAtMid)
	atCap := float64(bal.CostSkillMultAtCap)
	midRank := int(bal.CostSkillMidRank)
	capRank := int(bal.CostSkillCapRank)

	// Belt-and-braces, not a defence against a zero-value Balance:
	// GetBalanceConfig() calls ensureConfigValidated(), so a Balance reaching
	// this function has already been through validateProgression(), which
	// (after fix #3 below) rejects capRank <= midRank at load time. This
	// guard exists only in case a future caller constructs a Balance by hand
	// and skips validation — cheap insurance against a divide-by-zero.
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
