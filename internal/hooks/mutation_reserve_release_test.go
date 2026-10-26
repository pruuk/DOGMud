package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/stretchr/testify/assert"
)

func seedTitan(t *testing.T) {
	t.Helper()
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"titan-growth": {MutationId: "titan-growth", Name: "Titan Growth", Rarity: 7, MaxRank: 4,
			ReserveUnits: []int{1, 1, 1, 1}},
		"extra-arms": {MutationId: "extra-arms", Name: "Extra Arms", Rarity: 8, MaxRank: 4,
			ReserveUnits: []int{1, 1, 1, 2}},
	}))
}

func TestReleaseNewestCompanionsUntilUnderCeiling(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.Mutations = map[string]int{"titan-growth": 3, "extra-arms": 2} // 5 units = 220
	ch.Companions = []characters.CompanionInfo{
		{Name: "old wolf", InstanceId: 1, ConvictionReserve: 30},
		{Name: "new wolf", InstanceId: 2, ConvictionReserve: 30},
	}
	ch.ConvictionMax.Value = 400 // ceiling 264
	released := releaseCompanionsOverCeiling(ch)
	// 220 + 60 = 280 > 264: drop the newest (250 fits).
	if assert.Len(t, released, 1) {
		assert.Equal(t, "new wolf", released[0].Name)
	}
	if assert.Len(t, ch.Companions, 1) {
		assert.Equal(t, "old wolf", ch.Companions[0].Name)
	}
}

func TestNoReleaseWithoutReservingMutations(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.Companions = []characters.CompanionInfo{{Name: "wolf", InstanceId: 1, ConvictionReserve: 90}}
	ch.ConvictionMax.Value = 100
	assert.Empty(t, releaseCompanionsOverCeiling(ch), "D4: an overage with no body share is grandfathered")
	assert.Len(t, ch.Companions, 1)
}

func TestBondedCompanionsAreNeverReleased(t *testing.T) {
	seedTitan(t)
	ch := characters.New()
	ch.Mutations = map[string]int{"titan-growth": 4, "extra-arms": 3} // 7 units, clamped to 264
	ch.Companions = []characters.CompanionInfo{{Name: "Jev", InstanceId: 1, SourceType: characters.CompanionBonded}}
	ch.ConvictionMax.Value = 400
	assert.Empty(t, releaseCompanionsOverCeiling(ch))
	assert.Len(t, ch.Companions, 1)
}
