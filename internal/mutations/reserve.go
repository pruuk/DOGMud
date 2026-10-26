package mutations

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// ReserveUnitsAt returns the reserve units this mutation holds at the given
// rank (#680): the sum of the first `rank` entries of ReserveUnits.
func (s *MutationSpec) ReserveUnitsAt(rank int) int {
	total := 0
	for i := 0; i < rank && i < len(s.ReserveUnits); i++ {
		total += s.ReserveUnits[i]
	}
	return total
}

// OwnedReserveUnits returns the units every owned mutation holds at its rank.
func OwnedReserveUnits(owned map[string]int) int {
	total := 0
	for id, level := range owned {
		if spec := GetMutation(id); spec != nil {
			total += spec.ReserveUnitsAt(level)
		}
	}
	return total
}

// ReserveUnitBudget is how many units fit under the reservation ceiling. It is
// derived from PoolReservationCapPct and MutationReserveUnitPct, never a knob
// of its own, so the two can never disagree. The epsilon keeps 0.66 / 0.11
// from flooring to 5 on float error.
func ReserveUnitBudget() int {
	b := configs.GetBalanceConfig()
	unit := float64(b.MutationReserveUnitPct)
	if unit <= 0 {
		return 0
	}
	return int(math.Floor(float64(b.PoolReservationCapPct)/unit + 1e-9))
}

// FitsReserveBudget reports whether owning `id` at newRank keeps the owned
// units within ReserveUnitBudget. Gear and companions never count here: the
// budget is the body's alone (owner, 2026-10-26).
func FitsReserveBudget(owned map[string]int, id string, newRank int) bool {
	spec := GetMutation(id)
	if spec == nil {
		return true
	}
	units := OwnedReserveUnits(owned) - spec.ReserveUnitsAt(owned[id]) + spec.ReserveUnitsAt(newRank)
	return units <= ReserveUnitBudget()
}

// GrowsLarge reports whether an owned mutation's size effect has reached the
// rank it names (Titan Growth: 3). A size effect is a threshold, so it is
// read here and never through sumEffects or the level curve.
func GrowsLarge(owned map[string]int) bool {
	for id, level := range owned {
		spec := GetMutation(id)
		if spec == nil {
			continue
		}
		for _, p := range spec.Pros {
			if p.Type == "size" && p.Target == "large" && level >= int(p.Value) {
				return true
			}
		}
	}
	return false
}
