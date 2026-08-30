package configs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ⚠️ An absent key unmarshals to 0, and |gap|^0 == 1 -- every mismatch in the
// game would collapse to a one-point gap. So this validator must key on <= 0,
// never < 0. That exact trap left StealCooldown, StealHiddenBonus,
// ShadowCooldown, SneakFailCooldown and PackScatterRounds pinned at zero.
func TestContestGapCompression_AbsentKeyIsIdentity(t *testing.T) {
	b := Balance{}
	b.Validate()
	assert.Equal(t, ConfigFloat(1.0), b.ContestGapCompression,
		"an absent exponent must be 1.0 (no compression). If this is ever 0, "+
			"deleting the line from config.yaml collapses every mismatched "+
			"contest in the game to a one-point gap, with no error anywhere.")
}

func TestContestGapCompression_Clamped(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{-1.0, 1.0}, // nonsense
		{0.0, 1.0},  // reads as unset
		{1.5, 1.0},  // this knob only compresses, never expands
		{0.80, 0.80},
		{0.5, 0.5},
		{1.0, 1.0},
	} {
		b := Balance{ContestGapCompression: ConfigFloat(tc.in)}
		b.Validate()
		assert.Equal(t, ConfigFloat(tc.want), b.ContestGapCompression, "input %v", tc.in)
	}
}

// Absent weights must reproduce TODAY's 80/20 archetype split, not 0/0. A zero
// weight pair would make every distribution degenerate.
func TestArchetypeWeights_AbsentKeysKeepTodaysSplit(t *testing.T) {
	b := Balance{}
	b.Validate()
	assert.Equal(t, ConfigFloat(0.80/3), b.ArchetypePrimaryStatWeight,
		"absent primary weight must reproduce today's 26.7% per primary stat")
	assert.Equal(t, ConfigFloat(0.20/3), b.ArchetypeSecondaryStatWeight,
		"absent secondary weight must reproduce today's 6.7% per non-primary stat")
}

func TestArchetypeWeights_RejectNonPositive(t *testing.T) {
	b := Balance{
		ArchetypePrimaryStatWeight:   ConfigFloat(-1),
		ArchetypeSecondaryStatWeight: ConfigFloat(0),
	}
	b.Validate()
	assert.Equal(t, ConfigFloat(0.80/3), b.ArchetypePrimaryStatWeight)
	assert.Equal(t, ConfigFloat(0.20/3), b.ArchetypeSecondaryStatWeight)
}

// A secondary weight above the primary would invert the archetype. Refuse it.
func TestArchetypeWeights_SecondaryMayNotExceedPrimary(t *testing.T) {
	b := Balance{
		ArchetypePrimaryStatWeight:   ConfigFloat(0.15),
		ArchetypeSecondaryStatWeight: ConfigFloat(0.25),
	}
	b.Validate()
	assert.Equal(t, b.ArchetypePrimaryStatWeight, b.ArchetypeSecondaryStatWeight,
		"a secondary weight above the primary inverts the archetype; clamp to equal")
}
