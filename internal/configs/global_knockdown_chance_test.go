package configs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// An ABSENT key unmarshals to 0. For this knob 0 would mean "no contested
// knockdown ever lands", so a `< 0` validator would let a missing line silently
// disable every knockdown in the game.
//
// That is not hypothetical: StealCooldown, StealHiddenBonus, ShadowCooldown,
// SneakFailCooldown and PackScatterRounds were all found pinned at zero on
// 2026-08-30 for exactly this reason. This knob must fail SAFE instead.
func TestGlobalKnockdownChance_AbsentKeyIsUnchangedNotDisabled(t *testing.T) {
	b := Balance{} // every field at its zero value, i.e. nothing configured
	b.Validate()

	assert.Equal(t, ConfigFloat(1.0), b.GlobalKnockdownChance,
		"an absent GlobalKnockdownChance must read as 1.0 (unchanged). If this "+
			"is ever 0, deleting the line from config.yaml disables every "+
			"knockdown in the game with no error anywhere.")
}

func TestGlobalKnockdownChance_OutOfRangeIsClamped(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{-1.0, 1.0}, // nonsense
		{0.0, 1.0},  // reads as unset, deliberately not an off-switch
		{1.5, 1.0},  // this knob only reduces
		{0.5, 0.5},  // a real setting survives
		{1.0, 1.0},
	} {
		b := Balance{GlobalKnockdownChance: ConfigFloat(tc.in)}
		b.Validate()
		assert.Equal(t, ConfigFloat(tc.want), b.GlobalKnockdownChance,
			"input %v", tc.in)
	}
}
