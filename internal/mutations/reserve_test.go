package mutations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// seedColossus seeds the three reserving mutations as #680 ships them, plus
// a same-cluster mutation with no units and an unrelated one.
func seedColossus(t *testing.T) {
	t.Helper()
	t.Cleanup(SeedMutationsForTest(map[string]*MutationSpec{
		"titan-growth": {MutationId: "titan-growth", Name: "Titan Growth", Rarity: 7,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1, 1, 1, 1},
			Pros: []MutationEffect{{Type: "size", Target: "large", Value: 3}}},
		"extra-arms": {MutationId: "extra-arms", Name: "Extra Arms", Rarity: 8, MaxRank: 4,
			Clusters: []string{"colossus", "ravener"}, ReserveUnits: []int{1, 1, 1, 2}},
		"colossus-form": {MutationId: "colossus-form", Name: "Colossus Form", Rarity: 9, MaxRank: 1,
			Clusters: []string{"colossus"}, ReserveUnits: []int{1},
			Prerequisites: []MutationPrereq{{Id: "dense-muscles", MinLevel: 1}, {Id: "titan-growth", MinLevel: 4}},
			Pros:          []MutationEffect{{Type: "flag", Target: "colossal-grip", Value: 1}}},
		"dense-muscles": {MutationId: "dense-muscles", Name: "Dense Muscles", Rarity: 3,
			Clusters: []string{"colossus"}},
		"padded-soles": {MutationId: "padded-soles", Name: "Padded Soles", Rarity: 3,
			Clusters: []string{"stalker"}},
	}))
}

func TestReserveUnitBudgetIsSixAtShippedKnobs(t *testing.T) {
	assert.Equal(t, 6, ReserveUnitBudget())
}

func TestOwnedReserveUnits(t *testing.T) {
	seedColossus(t)
	for _, tc := range []struct {
		owned map[string]int
		want  int
	}{
		{map[string]int{"titan-growth": 3, "extra-arms": 3}, 6},
		{map[string]int{"titan-growth": 4, "colossus-form": 1, "extra-arms": 1}, 6},
		{map[string]int{"extra-arms": 4}, 5},
		{map[string]int{"titan-growth": 3, "extra-arms": 2}, 5},
		{map[string]int{"dense-muscles": 4, "padded-soles": 2}, 0},
	} {
		assert.Equal(t, tc.want, OwnedReserveUnits(tc.owned), "%v", tc.owned)
	}
}

func TestFitsReserveBudget(t *testing.T) {
	seedColossus(t)
	assert.True(t, FitsReserveBudget(map[string]int{"titan-growth": 3, "extra-arms": 2}, "extra-arms", 3))
	assert.False(t, FitsReserveBudget(map[string]int{"titan-growth": 3, "extra-arms": 3}, "titan-growth", 4))
	assert.True(t, FitsReserveBudget(map[string]int{"titan-growth": 4, "colossus-form": 1}, "extra-arms", 1))
	assert.False(t, FitsReserveBudget(map[string]int{"extra-arms": 3, "titan-growth": 2}, "extra-arms", 4),
		"Arms 4 + Titan 2 is seven units")
	assert.True(t, FitsReserveBudget(map[string]int{"titan-growth": 3, "extra-arms": 3}, "dense-muscles", 2),
		"a rank with no units always fits")
}

// A body already past the budget (the knob was lowered, or an authored set
// skipped it) can still gain anything that adds no units, but never a rank
// that adds units.
func TestFitsReserveBudgetOverBudgetAllowsOnlyZeroDelta(t *testing.T) {
	seedColossus(t)
	over := map[string]int{"titan-growth": 4, "extra-arms": 4} // 9 units
	assert.True(t, FitsReserveBudget(over, "dense-muscles", 1), "a new zero-unit mutation")
	assert.True(t, FitsReserveBudget(over, "padded-soles", 3), "a zero-unit deepening")
	assert.False(t, FitsReserveBudget(over, "colossus-form", 1), "adds a unit while over")
	over["extra-arms"] = 3 // 8 units
	assert.False(t, FitsReserveBudget(over, "extra-arms", 4), "adds two units while over")
}

func TestGrowsLargeAtTitanThree(t *testing.T) {
	seedColossus(t)
	assert.False(t, GrowsLarge(map[string]int{"titan-growth": 2}))
	assert.True(t, GrowsLarge(map[string]int{"titan-growth": 3}))
	assert.True(t, GrowsLarge(map[string]int{"titan-growth": 4}))
	assert.False(t, GrowsLarge(map[string]int{"extra-arms": 4}))
}

func TestValidateRejectsBadReserveAndSize(t *testing.T) {
	neg := &MutationSpec{MutationId: "x", Name: "X", Rarity: 1, ReserveUnits: []int{1, -1}}
	assert.Error(t, neg.Validate())
	badSize := &MutationSpec{MutationId: "y", Name: "Y", Rarity: 1,
		Pros: []MutationEffect{{Type: "size", Target: "huge", Value: 3}}}
	assert.Error(t, badSize.Validate())
	zeroRank := &MutationSpec{MutationId: "z", Name: "Z", Rarity: 1,
		Pros: []MutationEffect{{Type: "size", Target: "large", Value: 0}}}
	assert.Error(t, zeroRank.Validate())
}

func TestDescribeSizeAndGrip(t *testing.T) {
	// The size step lands at rank 3 of 4, so the line must not promise it only
	// at full growth.
	assert.Equal(t, "In its later stages you grow large enough to wield a two-handed weapon in your weapon hand alone.",
		DescribeEffect(MutationEffect{Type: "size", Target: "large", Value: 3}))
	assert.NotEmpty(t, DescribeEffect(MutationEffect{Type: "flag", Target: "colossal-grip", Value: 1}))
}
