package costs

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// EncumbranceMultiplier returns the cost multiplier for how much weight the
// actor is carrying, as a fraction of carry capacity. It applies to PHYSICAL
// actions only — mental and social actions never take it.
//
// The curve is smooth (owner ruling 2026-11-16, #401): with r the load as a
// fraction of capacity, clamped to [0, 1], the multiplier is
// 1 + (CostEncumbranceMax - 1) * r^CostEncumbranceShape. It is 1.0 when
// empty and CostEncumbranceMax at and above capacity.
//
// A shape above 1 stays light for most of the range and climbs late, the
// job the old 75% knee did with two straight segments. It replaced that knee
// when action points were removed and movement came to be paid in stamina
// alone.
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

	shape := float64(bal.CostEncumbranceShape)
	maxMult := float64(bal.CostEncumbranceMax)

	// Belt-and-braces, not a defence against a zero-value Balance:
	// GetBalanceConfig() validates at load, so a shape at or below 0 or a top
	// below 1 cannot reach here. This guard exists only in case a caller
	// builds a Balance by hand and skips validation; a non-positive shape
	// would make r^shape flat or infinite, and a top below 1 would make
	// loading up cheaper.
	if shape <= 0 {
		shape = 2.5
	}
	if maxMult < 1.0 {
		maxMult = 1.0
	}

	r := carried / capacity
	if r < 0 {
		r = 0
	}
	if r > 1.0 {
		r = 1.0
	}

	return 1.0 + (maxMult-1.0)*math.Pow(r, shape)
}
