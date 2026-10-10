package items

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// #277: one spoilage check, shared by player and mob eat.
func TestIsSpoiledFood(t *testing.T) {
	aging := AgingThresholds{FermentRounds: 10, PeakRounds: 20, DecayRounds: 30, SpoilRounds: 40}
	food := Item{ItemId: 999951, Spec: &ItemSpec{ItemId: 999951, Type: Food, Subtype: Edible, Aging: aging}, CraftedRound: 100}

	assert.False(t, food.IsSpoiledFood(120), "inside its life")
	assert.True(t, food.IsSpoiledFood(140), "at its spoil point")

	unknownAge := food
	unknownAge.CraftedRound = 0
	assert.False(t, unknownAge.IsSpoiledFood(100000), "no craft round: never spoils")

	plain := Item{ItemId: 999952, Spec: &ItemSpec{ItemId: 999952, Type: Food, Subtype: Edible}, CraftedRound: 1}
	assert.False(t, plain.IsSpoiledFood(100000), "no aging data: never spoils")
}
