package conditions

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/dice"
)

// ComputeTickAmount calculates the per-tick heal/damage amount.
// maxPool: target's max HP/SP/CP for the relevant pool.
// percent: base percentage (positive=heal, negative=damage).
// variance: random variance added to percent (0=none).
// minAmount: minimum absolute value (1 if not specified).
// scalingMult: spell skill multiplier (1.0 for potions/non-spell).
// Returns the signed tick amount (positive=heal, negative=damage).
func ComputeTickAmount(maxPool int, percent float64, variance float64, minAmount int, scalingMult float64) int {
	if percent == 0 {
		return 0
	}
	if minAmount < 1 {
		minAmount = 1
	}

	effectivePercent := percent
	if variance > 0 {
		effectivePercent += dice.Float64() * variance // seedable (#594)
	}

	base := float64(maxPool) * effectivePercent
	scaled := base * scalingMult
	amount := max(int(math.Round(math.Abs(scaled))), minAmount)

	if percent < 0 {
		return -amount
	}
	return amount
}
