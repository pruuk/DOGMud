package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mutations"
)

// U10b-1b renamed this from salvageChanceWithMutations: salvage is a CONTEST
// now, so the yield mutation scales the SCORE rather than a probability.
//
// 🔴 The 1.0 cap is deliberately gone, and its absence is asserted below. A
// probability had to be clamped; a score must NOT be, because clamping it would
// make the mutation stop helping exactly the master salvagers who invested in
// it. That was a real defect of the old form hiding behind a sensible-looking
// math.Min.
func TestSalvageScoreWithMutations_ProvidentHands(t *testing.T) {
	cleanup := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"salv-mut": {MutationId: "salv-mut", Name: "Salv", Rarity: 3,
			Pros: []mutations.MutationEffect{{Type: "salvage_yield_bonus", Value: 0.30}}},
	})
	defer cleanup()

	c := characters.New()
	// No mutation → unchanged.
	if got := salvageScoreWithMutations(c, 200); got != 200 {
		t.Fatalf("no mutation: got %v, want 200", got)
	}
	// With mutation → boosted (200 * 1.30 = 260).
	c.Mutations = map[string]int{"salv-mut": 1}
	if got := salvageScoreWithMutations(c, 200); got != 260 {
		t.Fatalf("with mutation: got %v, want 260", got)
	}
	// NO CAP. A high-scoring salvager must keep the full benefit; the old form
	// clamped at 1.0 and silently deleted the mutation for anyone good enough
	// to already be near-certain.
	if got := salvageScoreWithMutations(c, 10000); got != 13000 {
		t.Fatalf("high score: got %v, want 13000 (uncapped)", got)
	}
}
