package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// U6b Task 13 — grapple scoring joins the SkillWeight regime and grapple crit
// derives from the normalized contest margin vs CritBarFor, not the "Stage
// 8.4" self-relative z-score.
// ---------------------------------------------------------------------------

func newGrappleMarginChar(dex, unarmed int) *characters.Character {
	c := characters.New()
	c.Stats.Dexterity.ValueAdj = dex
	c.Skills["unarmed-combat"] = unarmed
	return c
}

// The grapple skill terms were pinned at x1 — a defect: every other additive
// contest score on this branch (flee, defence admission, submission after this
// task) weights skill by the global SkillWeight knob. AttackScore/DefenseScore
// are pre-roll values, so this is deterministic.
func TestAttemptGrapple_SkillTermsUseSkillWeight(t *testing.T) {
	c := configs.GetConfig()
	c.Balance.SkillWeight = 3 // distinctive; shipped value is 2.0
	configs.SetConfigForTest(t, c)

	atk := newGrappleMarginChar(100, 20)
	def := newGrappleMarginChar(80, 10)

	res := AttemptGrapple(atk, def)

	assert.InDelta(t, float64(atk.GetEffectiveDexterity())+20*3, res.AttackScore, 0.001,
		"attack score must be Dex + unarmed rank x SkillWeight")
	assert.InDelta(t, float64(def.GetEffectiveDexterity())+10*3, res.DefenseScore, 0.001,
		"defense score must be Dex + unarmed rank x SkillWeight")
}

// The two prone literals moved into config, and an earlier plan draft SWAPPED
// them. Pin the knobs to distinguishable values and assert each side's score
// takes ITS OWN knob: defender-prone scales DefenseScore by
// GrappleProneDefenderMod, attacker-prone scales AttackScore by
// GrappleProneAttackerMod.
func TestAttemptGrapple_ProneKnobsKeepTheirSides(t *testing.T) {
	c := configs.GetConfig()
	c.Balance.SkillWeight = 2
	c.Balance.GrappleProneAttackerMod = 0.9
	c.Balance.GrappleProneDefenderMod = 0.1
	configs.SetConfigForTest(t, c)

	t.Run("defender prone → DefenseScore x defender knob", func(t *testing.T) {
		atk := newGrappleMarginChar(100, 25)
		def := newGrappleMarginChar(100, 25)
		setCombatPositionParallel(def, position.Prone)

		res := AttemptGrapple(atk, def)

		base := float64(def.GetEffectiveDexterity()) + 25*2
		assert.InDelta(t, base*0.1, res.DefenseScore, 0.001,
			"defender-prone must apply GrappleProneDefenderMod to the DEFENSE score")
		assert.InDelta(t, float64(atk.GetEffectiveDexterity())+25*2, res.AttackScore, 0.001,
			"attacker score must be untouched when only the defender is prone")
	})

	t.Run("attacker prone → AttackScore x attacker knob", func(t *testing.T) {
		atk := newGrappleMarginChar(100, 25)
		def := newGrappleMarginChar(100, 25)
		setCombatPositionParallel(atk, position.Prone)

		res := AttemptGrapple(atk, def)

		base := float64(atk.GetEffectiveDexterity()) + 25*2
		assert.InDelta(t, base*0.9, res.AttackScore, 0.001,
			"attacker-prone must apply GrappleProneAttackerMod to the ATTACK score")
		assert.InDelta(t, float64(def.GetEffectiveDexterity())+25*2, res.DefenseScore, 0.001,
			"defender score must be untouched when only the attacker is prone")
	})
}

// The crit inputs the grapple_move ladder consumes must ride on GrappleResult
// — converting the roll while the ladder still read the old z-score would be
// a half-conversion that compiles. The bar is CritBarFor over BOTH sides'
// unarmed-combat ranks; NormalizedMargin is the attack-positive contest
// margin in std-dev units, so with the floor pinned off its sign must agree
// with Success.
func TestAttemptGrapple_ThreadsMarginAndPairBar(t *testing.T) {
	pinContestFloorOff(t)

	atk := newGrappleMarginChar(100, 40)
	def := newGrappleMarginChar(100, 10)

	res := AttemptGrapple(atk, def)

	assert.Equal(t, CritBarFor(40, 10), res.CritBar,
		"CritBar must be CritBarFor over the unarmed-combat pair")
	if res.Success {
		assert.Greater(t, res.NormalizedMargin, 0.0,
			"a won contest (floor off) must carry a positive normalized margin")
	} else {
		assert.Less(t, res.NormalizedMargin, 0.0,
			"a lost contest (floor off) must carry a negative normalized margin")
	}
}

// The "Stage 8.4" crit read the attacker's SELF-RELATIVE z (> 2.0), which
// crits ~2.3% regardless of the opponent: a dominant grappler could not
// convert domination into crits, and a hopeless one still critted at the
// parity rate. Margin-vs-CritBarFor makes the crit rate track the matchup.
func TestAttemptGrapple_CritDerivesFromMarginNotSelfZScore(t *testing.T) {
	pinContestFloorOff(t)
	const iterations = 2000

	t.Run("dominant attacker crits far above the legacy 2.3%", func(t *testing.T) {
		crits := 0
		for i := 0; i < iterations; i++ {
			atk := newGrappleMarginChar(1000, 50)
			def := newGrappleMarginChar(10, 1)
			if AttemptGrapple(atk, def).Crit {
				crits++
			}
		}
		assert.Greater(t, float64(crits)/iterations, 0.5,
			"a dominant attacker's margin clears the pair bar on most attempts; the old self-z capped this at ~2.3%")
	})

	t.Run("hopeless attacker essentially never crits", func(t *testing.T) {
		crits := 0
		for i := 0; i < iterations; i++ {
			atk := newGrappleMarginChar(10, 1)
			def := newGrappleMarginChar(1000, 50)
			if AttemptGrapple(atk, def).Crit {
				crits++
			}
		}
		assert.Less(t, float64(crits)/iterations, 0.01,
			"a hopeless attacker's margin can never reach the bar; the old self-z still critted ~2.3% here")
	})
}

// The failure-side defence penalty used the same self-relative z (< 0.5), so
// an outclassed attacker only ate the penalty on ~69% of failures — the
// roll's own wobble, not the contest. On the normalized margin, a decisively
// lost failure always lands under the same 0.5 line.
func TestExecuteGrappleMove_WeakBandReadsTheMargin(t *testing.T) {
	pinContestFloorOff(t)

	failures, penalties := 0, 0
	for i := 0; i < 400; i++ {
		atk := newGrappleMarginChar(10, 1)
		def := newGrappleMarginChar(1000, 50)
		res := ExecuteGrappleMove(atk, def, 1, nil)
		if !res.Success {
			failures++
			if res.DefensePenalty {
				penalties++
			}
		}
	}
	require.Greater(t, failures, 300, "a hopeless attacker with the floor off should almost always fail")
	assert.GreaterOrEqual(t, float64(penalties)/float64(failures), 0.99,
		"every decisively lost failure must land under the 0.5 margin line; the old self-z hit only ~69%")
}
