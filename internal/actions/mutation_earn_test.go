package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

func seedEarnMutations(t *testing.T) {
	t.Helper()
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"titan-growth": {MutationId: "titan-growth", Name: "Titan Growth", Rarity: 7, MaxRank: 4,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1, 1, 1, 1}},
		"extra-arms": {MutationId: "extra-arms", Name: "Extra Arms", Rarity: 8, MaxRank: 4,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1, 1, 1, 2}},
	}))
}

func TestEarnMutationKeepsProgressWhenNothingFits(t *testing.T) {
	seedEarnMutations(t)
	u := users.NewTestUser(9801, "earner", "Earner", uint64(9801))
	u.Character.Mutations = map[string]int{"titan-growth": 3, "extra-arms": 3}
	u.Character.MutationProgress = 42
	u.Character.ClusterAffinity = map[string]float64{"colossus": 1.0}
	assert.Equal(t, "", EarnMutation(u))
	assert.Equal(t, 42.0, u.Character.MutationProgress)
	assert.Equal(t, 3, u.Character.Mutations["titan-growth"])
	assert.Equal(t, 1.0, u.Character.ClusterAffinity["colossus"], "drift fades only on a grant")
}

func TestEarnMutationSpendsProgressOnAGrant(t *testing.T) {
	seedEarnMutations(t)
	u := users.NewTestUser(9802, "earner2", "Earner2", uint64(9802))
	u.Character.Mutations = map[string]int{"titan-growth": 2}
	u.Character.MutationProgress = 42
	// Strong colossus drift so the acquire branch (rolled 30% of the time)
	// always finds Extra Arms; without it that branch grants nothing and the
	// test is a coin flip.
	u.Character.ClusterAffinity = map[string]float64{"colossus": 1e6}
	got := EarnMutation(u)
	assert.NotEqual(t, "", got)
	assert.Equal(t, 0.0, u.Character.MutationProgress)
}
