package costs

import "github.com/GoMudEngine/GoMud/internal/configs"

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
func EncumbranceMultiplier(carried, capacity float64) float64 {
	if capacity <= 0 {
		return 1.0
	}

	bal := configs.GetBalanceConfig()

	knee := float64(bal.CostEncumbranceKnee)
	kneeMult := float64(bal.CostEncumbranceKneeMult)
	max := float64(bal.CostEncumbranceMax)

	// Guard against a misconfigured knob (zero, negative, or a knee at/above
	// 1.0) producing a divide-by-zero or a degenerate curve.
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
	return kneeMult + (max-kneeMult)*((r-knee)/(1-knee))
}
