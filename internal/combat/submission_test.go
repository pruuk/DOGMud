package combat_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
)

func setupBalanceForSubmissionTests(t *testing.T) {
	// Ensure balance config is loaded so the sub roll picks up the
	// T3 knobs.
	cfg := configs.GetBalanceConfig()
	if float64(cfg.SubmissionAttemptAlpha) == 0 {
		t.Helper()
		t.Skip("balance config not initialized for tests")
	}
}

func newCharFor(t *testing.T, str, vit, unarmedSkill int) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Stats.Strength.Base = str
	c.Stats.Strength.ValueAdj = str
	c.Stats.Vitality.Base = vit
	c.Stats.Vitality.ValueAdj = vit
	if c.Skills == nil {
		c.Skills = map[string]int{}
	}
	c.Skills[string(skills.UnarmedCombat)] = unarmedSkill
	return c
}

func TestRollSubmissionAttempt_Structure(t *testing.T) {
	setupBalanceForSubmissionTests(t)
	atk := newCharFor(t, 120, 100, 30)
	def := newCharFor(t, 100, 100, 10)
	res := combat.RollSubmissionAttempt(atk, def, position.SubArmbar)
	assert.Equal(t, position.SubArmbar, res.SubType)
	assert.NotZero(t, res.AttackerScore)
	assert.NotZero(t, res.DefenderScore)
	// Tier is one of the four enum values
	assert.True(t, res.Tier >= combat.SubTierBad && res.Tier <= combat.SubTierCrit)
}

func TestRollSubmissionAttempt_BadTierOnVeryLowZ(t *testing.T) {
	setupBalanceForSubmissionTests(t)
	cfg := configs.GetBalanceConfig()
	assert.Equal(t, combat.SubTierBad,
		combat.ClassifySubmissionTier(false, false, float64(cfg.SubBadZThreshold)-0.5))
}

// UPDATED for U6b Task 13. The old shape of this test pinned the STUN-CRIT
// band to the attacker's self-relative z-score vs a config z-threshold — a
// pinned DEFECT: a self-relative z is opponent-blind, so stun-crit fired at a
// flat ~2% no matter how dominant the attempter was. The stun tier is now a
// margin crit vs CritBarFor (decided in RollSubmissionAttempt, delivered to
// the classifier as a boolean); only the bad band stays self-relative
// (SubBadZThreshold), because a fumble is the attempter's own blunder.
func TestClassifySubmissionTier_BoundaryConditions(t *testing.T) {
	setupBalanceForSubmissionTests(t)
	cfg := configs.GetBalanceConfig()
	bad := float64(cfg.SubBadZThreshold)

	// Failed and strictly below the bad threshold → Bad
	assert.Equal(t, combat.SubTierBad, combat.ClassifySubmissionTier(false, false, bad-0.1))
	// Equal to bad threshold and failed → Neutral
	assert.Equal(t, combat.SubTierNeutral, combat.ClassifySubmissionTier(false, false, bad))
	// Failed and above bad threshold → Neutral
	assert.Equal(t, combat.SubTierNeutral, combat.ClassifySubmissionTier(false, false, 0.0))
	// Success without a stun-crit → Success
	assert.Equal(t, combat.SubTierSuccess, combat.ClassifySubmissionTier(true, false, 0.0))
	// Success with a stun-crit → Crit
	assert.Equal(t, combat.SubTierCrit, combat.ClassifySubmissionTier(true, true, 0.0))
	// A stun-crit flag on a FAILED roll must never promote the tier
	assert.Equal(t, combat.SubTierNeutral, combat.ClassifySubmissionTier(false, true, 0.0))
}

// The old sub-only skill weight knob (1.5, a regime shared with nothing else)
// is DELETED by U6b Task 13: both sides of the sub roll now weight
// unarmed-combat by the global SkillWeight, same as every other additive
// contest score. Scores are pre-roll values, so this is deterministic.
func TestRollSubmissionAttempt_SkillWeightOnBothSides(t *testing.T) {
	setupBalanceForSubmissionTests(t)
	c := configs.GetConfig()
	c.Balance.SkillWeight = 3 // distinctive; shipped value is 2.0
	configs.SetConfigForTest(t, c)

	atk := newCharFor(t, 120, 100, 30)
	def := newCharFor(t, 100, 90, 10)

	res := combat.RollSubmissionAttempt(atk, def, position.SubArmbar)
	assert.InDelta(t, 120+30*3.0, res.AttackerScore, 0.001,
		"attacker score must be Str + unarmed rank x SkillWeight")
	assert.InDelta(t, 100+90+10*3.0, res.DefenderScore, 0.001,
		"defender score must be Str + Vit + unarmed rank x SkillWeight")
}

// The modelled stun-crit rise (a flat ~2% → 18-62% depending on matchup,
// accepted for playtest): stun-crit now derives from the normalized contest
// margin vs the unarmed pair bar, so a dominant attempter converts
// domination into stuns and a hopeless one essentially never stuns.
func TestRollSubmissionAttempt_StunCritTracksTheMatchup(t *testing.T) {
	setupBalanceForSubmissionTests(t)
	c := configs.GetConfig()
	c.Balance.ContestFloor = 0 // a floored sentinel win would muddy the rates
	configs.SetConfigForTest(t, c)
	const iterations = 2000

	t.Run("dominant attempter stun-crits far above the legacy ~2%", func(t *testing.T) {
		crits := 0
		for i := 0; i < iterations; i++ {
			atk := newCharFor(t, 1000, 100, 50)
			def := newCharFor(t, 10, 10, 1)
			if combat.RollSubmissionAttempt(atk, def, position.SubArmbar).Tier == combat.SubTierCrit {
				crits++
			}
		}
		assert.Greater(t, float64(crits)/iterations, 0.5,
			"a dominant attempter's margin clears the pair bar on most attempts; the old self-z capped this at ~2%")
	})

	t.Run("hopeless attempter essentially never stun-crits", func(t *testing.T) {
		crits := 0
		for i := 0; i < iterations; i++ {
			atk := newCharFor(t, 10, 10, 1)
			def := newCharFor(t, 1000, 100, 50)
			if combat.RollSubmissionAttempt(atk, def, position.SubArmbar).Tier == combat.SubTierCrit {
				crits++
			}
		}
		assert.Less(t, float64(crits)/iterations, 0.01,
			"a hopeless attempter's margin can never reach the bar")
	})
}
