package characters

import (
	"math/rand"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/stretchr/testify/assert"
)

// capBloomWorld seeds three plain mutations and sets MutationMaxCount to 2.
func capBloomWorld(t *testing.T) {
	t.Helper()
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"cap-a": {MutationId: "cap-a", Name: "Cap A", Rarity: 5},
		"cap-b": {MutationId: "cap-b", Name: "Cap B", Rarity: 5},
		"cap-c": {MutationId: "cap-c", Name: "Cap C", Rarity: 5},
	}))
	cfg := configs.GetConfig()
	cfg.Balance.MutationMaxCount = 2
	configs.SetConfigForTest(t, cfg)
}

// A wafer cannot add a mutation once the character holds MutationMaxCount,
// by either route: the dose hook's direct seed, or advance falling through to
// seed because every owned mutation is at its rank cap (owner, 2026-11-04).
func TestBloomSeedStopsAtMutationMaxCount(t *testing.T) {
	capBloomWorld(t)
	top := mutations.MaxRankOf("cap-a")

	capped := &Character{Mutations: map[string]int{"cap-a": top, "cap-b": top}}
	id, lvl := capped.BloomSeedNewMutation(rand.New(rand.NewSource(1)))
	assert.Equal(t, "", id, "a seed at the cap grants nothing")
	assert.Equal(t, 0, lvl)
	assert.Len(t, capped.Mutations, 2)

	id, lvl = capped.BloomAdvanceMutation(rand.New(rand.NewSource(1)))
	assert.Equal(t, "", id, "advance falling through to seed grants nothing at the cap")
	assert.Equal(t, 0, lvl)
	assert.Len(t, capped.Mutations, 2)
	assert.NotContains(t, capped.Mutations, "cap-c")

	// Control: one under the count can still take a new mutation.
	under := &Character{Mutations: map[string]int{"cap-a": top}}
	id, lvl = under.BloomSeedNewMutation(rand.New(rand.NewSource(1)))
	assert.NotEqual(t, "", id, "one under the cap still seeds")
	assert.Equal(t, 1, lvl)
	assert.Len(t, under.Mutations, 2)
}

// At the count, a wafer still deepens a mutation the character owns.
func TestBloomAdvanceStillDeepensAtMutationMaxCount(t *testing.T) {
	capBloomWorld(t)
	top := mutations.MaxRankOf("cap-a")
	if top < 2 {
		t.Skip("needs a mutation rank cap of at least 2")
	}

	c := &Character{Mutations: map[string]int{"cap-a": 1, "cap-b": top}}
	id, lvl := c.BloomAdvanceMutation(rand.New(rand.NewSource(1)))
	assert.Equal(t, "cap-a", id)
	assert.Equal(t, 2, lvl)
	assert.Equal(t, 2, c.Mutations["cap-a"])
	assert.Len(t, c.Mutations, 2)
	assert.NotContains(t, c.Mutations, "cap-c")
}
