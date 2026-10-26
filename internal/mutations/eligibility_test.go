package mutations

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
)

// pinShippedMaxLevel sets MutationMaxLevel to its shipped 4. A bare test
// binary reads the Go default of 3, under which Titan Growth (no max_rank of
// its own) could never reach rank 4 and a budget refusal would pass for the
// max-rank reason instead.
func pinShippedMaxLevel(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.MutationMaxLevel = 4
	configs.SetConfigForTest(t, cfg)
}

func TestCanGainColossusNeedsTitanFour(t *testing.T) {
	seedColossus(t)
	assert.False(t, CanGain(map[string]int{"titan-growth": 3, "dense-muscles": 1}, "colossus-form", 1, nil))
	assert.True(t, CanGain(map[string]int{"titan-growth": 4, "dense-muscles": 1}, "colossus-form", 1, nil))
}

func TestCanGainRefusesOverBudgetAndPastMaxRank(t *testing.T) {
	seedColossus(t)
	pinShippedMaxLevel(t)
	assert.True(t, CanGain(map[string]int{"titan-growth": 3, "extra-arms": 2}, "titan-growth", 4, nil),
		"rank 4 is reachable, so the next refusal is the budget's")
	full := map[string]int{"titan-growth": 3, "extra-arms": 3}
	assert.False(t, CanGain(full, "titan-growth", 4, nil), "seven units")
	assert.True(t, CanGain(full, "dense-muscles", 1, nil), "no units, fits")
	assert.False(t, CanGain(map[string]int{"colossus-form": 1}, "colossus-form", 2, nil), "max_rank 1")
	assert.False(t, CanGain(map[string]int{"titan-growth": 2}, "titan-growth", 4, nil), "ranks go one at a time")
}

func TestWeightedPoolHonoursPrereqsAndBudget(t *testing.T) {
	seedColossus(t)
	pool := GetWeightedPool(map[string]int{"titan-growth": 1, "dense-muscles": 1}, nil)
	assert.NotContains(t, pool, "colossus-form", "prerequisites now apply to the weighted pool")

	// Every prerequisite met (Titan Growth 4, Dense Muscles 1) and six units
	// owned: Colossus Form's one unit is refused by the budget alone. The
	// control one unit lighter proves nothing else refuses it.
	pinShippedMaxLevel(t)
	full := GetWeightedPool(map[string]int{"titan-growth": 4, "dense-muscles": 1, "extra-arms": 2}, nil)
	assert.NotContains(t, full, "colossus-form", "seven units would pass the budget of six")
	assert.Contains(t, full, "padded-soles", "a zero-unit mutation still fits")
	roomy := GetWeightedPool(map[string]int{"titan-growth": 4, "dense-muscles": 1, "extra-arms": 1}, nil)
	assert.Contains(t, roomy, "colossus-form", "control: at five units Colossus Form fits")
}

func TestRelatedFitPrefersTheRefusedPicksCluster(t *testing.T) {
	seedColossus(t)
	assert.Equal(t, "dense-muscles", RelatedFit("titan-growth", []string{"dense-muscles", "padded-soles"}))
	assert.Equal(t, "padded-soles", RelatedFit("titan-growth", []string{"padded-soles"}))
	assert.Equal(t, "", RelatedFit("titan-growth", nil))
}

func TestRollDeepeningNeverPicksAnOverBudgetRank(t *testing.T) {
	seedColossus(t)
	pinShippedMaxLevel(t)
	owned := map[string]int{"titan-growth": 3, "extra-arms": 3, "dense-muscles": 1, "padded-soles": 1}
	sawDense := false
	for i := 0; i < 300; i++ {
		got := RollDeepening(owned)
		assert.Contains(t, []string{"dense-muscles", "padded-soles"}, got)
		sawDense = sawDense || got == "dense-muscles"
	}
	assert.True(t, sawDense)
}

func TestCanDeepenFalseWhenOnlyOverBudgetRanksRemain(t *testing.T) {
	seedColossus(t)
	pinShippedMaxLevel(t)
	assert.False(t, CanDeepen(map[string]int{"titan-growth": 3, "extra-arms": 3}))
	assert.True(t, CanDeepen(map[string]int{"titan-growth": 3, "extra-arms": 2}))
}
