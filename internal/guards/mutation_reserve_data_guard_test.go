package guards

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// #680: every reserve_units list covers every rank, and every authored
// mutation set (species intrinsics, mob spawn lists, the Homunculus preset)
// fits the reserve budget. Authored sets may skip prerequisites (owner,
// 2026-10-26); they may not skip the budget.
func TestMutationReserveData(t *testing.T) {
	mobsById := loadFightWorld(t)

	for _, spec := range mutations.AllSpecs() {
		if len(spec.ReserveUnits) > 0 && len(spec.ReserveUnits) != mutations.MaxRankOf(spec.MutationId) {
			t.Errorf("%s: reserve_units has %d entries, max rank is %d",
				spec.MutationId, len(spec.ReserveUnits), mutations.MaxRankOf(spec.MutationId))
		}
	}
	for _, want := range []string{"titan-growth", "extra-arms", "colossus-form"} {
		if s := mutations.GetMutation(want); s == nil || len(s.ReserveUnits) == 0 {
			t.Errorf("%s must declare reserve_units", want)
		}
	}

	budget := mutations.ReserveUnitBudget()
	check := func(who string, set map[string]int) {
		if u := mutations.OwnedReserveUnits(set); u > budget {
			t.Errorf("%s: authored mutations hold %d units, budget %d", who, u, budget)
		}
	}
	for _, sp := range species.GetAllSpecies() {
		check("species "+sp.Name, sp.IntrinsicMutations)
	}
	for id, m := range mobsById {
		set := map[string]int{}
		for _, mid := range m.SpawnMutations {
			set[mid] = 1
		}
		if sp := species.GetSpecies(m.Character.SpeciesId); sp != nil {
			for mid, r := range sp.IntrinsicMutations {
				set[mid] += r
			}
		}
		check("mob "+itoa(id), set)
	}
	check("homunculus preset", map[string]int{"titan-growth": 2})
}
