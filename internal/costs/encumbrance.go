package costs

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// EncumbranceMultiplier returns the cost multiplier for how much weight the
// actor is carrying, as a fraction of carry capacity. It applies to PHYSICAL
// actions only — mental and social actions never take it.
//
// The curve is two linear segments joined at the knee (CostEncumbranceKnee,
// default 0.75 of capacity):
//
//   - r = 0        -> 1.0 (empty, no penalty)
//   - r = knee      -> CostEncumbranceKneeMult (default 1.5)
//   - r >= 1.0      -> CostEncumbranceMax (default 5.0), clamped
//
// Gentle from empty to the knee, then steep from the knee to capacity, flat
// at and above capacity. A realistically equipped character (roughly 35-45%
// of capacity) lands near 1.25.
//
// This deliberately replaces the encumbrance term inline in
// GetMovementStaminaCost, which was flat 1.0 until the actor EXCEEDED
// capacity and only ramped to 5.0 at double capacity — a shape that priced
// nothing for anyone not deliberately overloaded. That call site is migrated
// to this function separately; this package does not touch it.
//
// A non-finite carried weight or capacity returns the neutral 1.0, matching how
// a non-positive capacity is handled. NaN fails every comparison, so it would
// otherwise sail through the clamps below and come out as a NaN multiplier,
// which characters.ApplyCostFloat would then bank into a pool's cost carry and
// make that pool free for the rest of the session. Fail at the source.
func EncumbranceMultiplier(carried, capacity float64) float64 {
	if math.IsNaN(carried) || math.IsInf(carried, 0) ||
		math.IsNaN(capacity) || math.IsInf(capacity, 0) {
		return 1.0
	}
	if capacity <= 0 {
		return 1.0
	}

	bal := configs.GetBalanceConfig()

	knee := float64(bal.CostEncumbranceKnee)
	kneeMult := float64(bal.CostEncumbranceKneeMult)
	maxMult := float64(bal.CostEncumbranceMax)

	// Belt-and-braces, not a defence against a zero-value Balance:
	// GetBalanceConfig() calls ensureConfigValidated(), so a Balance reaching
	// this function has already been validated at load time and cannot carry a
	// knee of zero, a negative knee, or a knee at/above 1.0. This guard exists
	// only in case a future caller constructs a Balance by hand and skips
	// validation -- cheap insurance against a divide-by-zero or a degenerate
	// curve. It is not the primary defence and is not expected to fire.
	if knee <= 0 || knee >= 1.0 {
		knee = 0.75
	}

	r := carried / capacity
	if r < 0 {
		r = 0
	}
	if r > 1.0 {
		r = 1.0
	}

	if r <= knee {
		return 1.0 + (kneeMult-1.0)*(r/knee)
	}
	return kneeMult + (maxMult-kneeMult)*((r-knee)/(1-knee))
}
