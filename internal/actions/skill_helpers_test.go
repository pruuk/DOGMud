package actions

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Task 16 (U6b): opposed stealth/detection linearization.
//
// CalcSneakScore converts in place to the linear stat + rank*SkillWeight
// regime (all of its consumers are opposed contests). CalcDetectionScore is
// the NEW linear observer-side score for opposed detection sites.
// CalcSearchScore does NOT convert: its remaining consumers (forage yield,
// search flat thresholds, track) are Category B and must move ZERO.
// ---------------------------------------------------------------------------

// TestCalcSneakScore_LinearSkillTerm verifies the sneak skill term is now
// linear: Dex + rank*SkillWeight (dark sneaker, dark room = no modifier).
func TestCalcSneakScore_LinearSkillTerm(t *testing.T) {
	sw := float64(configs.GetBalanceConfig().SkillWeight)
	require.Greater(t, sw, 0.0, "SkillWeight must be defaulted > 0")

	for _, rank := range []int{0, 5, 25, 50} {
		char := newTestChar()
		char.Stats.Dexterity.ValueAdj = 100
		char.Skills[string(skills.Skullduggery)] = rank

		got := CalcSneakScore(char, false /* effectiveLit */)
		want := 100.0 + float64(rank)*sw // no mutations on a fresh test char
		require.Equal(t, want, got,
			"CalcSneakScore rank %d must be Dex + rank*SkillWeight", rank)
	}
}

// TestCalcSneakScore_LightModifiersSurviveConversion pins that the lit-room
// modifier still applies multiplicatively on the linear base.
func TestCalcSneakScore_LightModifiersSurviveConversion(t *testing.T) {
	char := newTestChar()
	char.Stats.Dexterity.ValueAdj = 100
	char.Skills[string(skills.Skullduggery)] = 50

	dark := CalcSneakScore(char, false)
	lit := CalcSneakScore(char, true)
	require.InDelta(t, dark*0.9, lit, 0.0001,
		"lit-room modifier must remain 0.9x of the (linear) dark baseline")
}

// TestCalcDetectionScore_Linear verifies the new observer-side score:
// Perception + rank(search)*SkillWeight.
func TestCalcDetectionScore_Linear(t *testing.T) {
	sw := float64(configs.GetBalanceConfig().SkillWeight)

	for _, rank := range []int{0, 5, 25, 50} {
		char := newTestChar()
		char.Stats.Perception.ValueAdj = 100
		char.Skills[string(skills.Search)] = rank

		got := CalcDetectionScore(char)
		want := 100.0 + float64(rank)*sw
		require.Equal(t, want, got,
			"CalcDetectionScore rank %d must be Per + rank*SkillWeight", rank)
	}
}

// TestCalcSearchScore_RegressionFrozen asserts CalcSearchScore's output is
// byte-identical to its pre-Task-16 values at several rank points. Its
// remaining consumers — forage.go (forage YIELD), search.go (flat
// thresholds), track.go — are Category B: forage yields and search/track
// rates must move ZERO under U6b.
func TestCalcSearchScore_RegressionFrozen(t *testing.T) {
	bal := configs.GetBalanceConfig()
	// The frozen expectations below assume the default sqrt-curve knobs.
	require.Equal(t, 1.0, float64(bal.SkillMultiplierBase))
	require.Equal(t, 3.0, float64(bal.SkillMultiplierMax))
	require.Equal(t, 50.0, float64(bal.SkillSoftCap))

	// want = 100 + (base + (max-base)*sqrt(rank/softCap)) * 25
	cases := []struct {
		rank int
		want float64
	}{
		{0, 125.0}, // 100 + 1.0*25
		{5, 100.0 + (1.0+2.0*math.Sqrt(5.0/50.0))*25.0},   // ~140.8114
		{25, 100.0 + (1.0+2.0*math.Sqrt(25.0/50.0))*25.0}, // ~160.3553
		{50, 175.0}, // 100 + 3.0*25
	}

	for _, tc := range cases {
		char := newTestChar()
		char.Stats.Perception.ValueAdj = 100
		char.Skills[string(skills.Search)] = tc.rank

		got := CalcSearchScore(char)
		require.Equal(t, tc.want, got,
			"CalcSearchScore rank %d must be byte-identical to the frozen "+
				"sqrt-curve value (Category B: forage/search/track must move zero)",
			tc.rank)
	}
}
