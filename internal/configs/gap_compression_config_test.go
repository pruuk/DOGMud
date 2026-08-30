package configs

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 0 is the OFF value for this knob and is also what an absent key unmarshals
// to, so absence is safe by construction rather than by the usual guard.
//
// ⚠️ This is the OPPOSITE of the trap that pinned StealCooldown,
// StealHiddenBonus, ShadowCooldown, SneakFailCooldown and PackScatterRounds at
// zero. Those knobs have a nonzero intended default, so `if x < 0` could never
// repair an absent key. Here 0 IS the intended default, so the same shape is
// correct. Check which case you are in before copying either.
func TestContestGapSaturation_AbsentKeyIsIdentity(t *testing.T) {
	b := Balance{}
	b.Validate()
	assert.Equal(t, ConfigFloat(0), b.ContestGapSaturation,
		"an absent saturation must be 0 (no compression), so deleting the line "+
			"from config.yaml turns the feature off rather than doing something "+
			"unpredictable")
}

func TestContestGapSaturation_Clamped(t *testing.T) {
	for _, tc := range []struct {
		name     string
		in, want float64
	}{
		{"negative is nonsense", -1.0, 0},
		{"zero is off", 0.0, 0},
		{"shipped", 2.80, 2.80},
		{"mild", 1.0, 1.0},
		// ⚠️ NO upper bound on purpose: k is a saturation scale, not a
		// probability. Larger simply means flatter, and clamping it would
		// silently cap how hard the owner can dial the knob.
		{"large is legal", 50.0, 50.0},
	} {
		b := Balance{ContestGapSaturation: ConfigFloat(tc.in)}
		b.Validate()
		assert.Equal(t, ConfigFloat(tc.want), b.ContestGapSaturation, tc.name)
	}
}

// ⚠️ NaN fails EVERY ordinary comparison, so `if k < 0` and `if k <= 0` are
// both false for NaN and would let it through. ConfigFloat is a bare float64
// alias with no custom unmarshaller, so YAML `.nan` really does reach this
// struct. A NaN here makes every defence score NaN, `Margin > 0` false, and the
// attacker silently never wins a contest anywhere in the game.
func TestContestGapSaturation_RejectsNaN(t *testing.T) {
	b := Balance{ContestGapSaturation: ConfigFloat(math.NaN())}
	b.Validate()
	assert.False(t, math.IsNaN(float64(b.ContestGapSaturation)),
		"a NaN saturation must not survive validation")
	assert.Equal(t, ConfigFloat(0), b.ContestGapSaturation)
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
