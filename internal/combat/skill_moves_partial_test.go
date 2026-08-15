package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/stretchr/testify/assert"
)

// makePartialTestParams builds SkillMoveParams with a MODERATE defender
// advantage (attacker score ~100 vs defender score ~130). With RollSpread
// 0.15 this produces frequent bare/partial defensive wins alongside
// occasional defensive crits, so a loop of a few hundred iterations sees
// both a defended attempt with damage > 0 (the partial) and a bare miss.
//
// Do NOT copy the plan's original 100-vs-5000 gap: at that gap every
// defensive win is a defensive crit (normalized margin far past the 2.0
// threshold), the multiplier is 0.0 every time, and "sometimes deals
// partial damage" can never pass.
func makePartialTestParams(attacker, defender *characters.Character) SkillMoveParams {
	return SkillMoveParams{
		Attacker: attacker, Defender: defender,
		AttackStat: 100, AttackSkill: 0, DefenseStat: 130, DefenseSkill: 0,
		DamagePercent:   1.0,
		KnockdownChance: 100, // maximal, so a leaked status/knockdown on a
		// defended attempt would be caught rather than hidden by a low roll.
		SkillRank:            0,
		DamageStat:           100,
		MitigationMultiplier: 1.0,
	}
}

// TestExecuteSkillMove_DefendedAttemptDealsPartialDamageWithoutStatus is the
// Task 13 regression: a defended maneuver (Hit == false) must sometimes deal
// damage > 0 (the shared partial mechanism) while NEVER applying the binary
// status effect (StatusApplied / KnockedDown).
func TestExecuteSkillMove_DefendedAttemptDealsPartialDamageWithoutStatus(t *testing.T) {
	const iterations = 2000

	foundPartial := false

	for i := 0; i < iterations; i++ {
		attacker := characters.New()
		defender := characters.New()
		defender.HealthMax.Value = 100000
		defender.Health = 100000

		healthBefore := defender.Health

		res := ExecuteSkillMove(makePartialTestParams(attacker, defender))

		if res.Hit {
			continue
		}

		// The status effect stays binary: a defended attempt must never
		// apply status or knock the defender down, regardless of damage.
		assert.False(t, res.StatusApplied, "a defended attempt must never set StatusApplied")
		assert.False(t, res.KnockedDown, "a defended attempt must never set KnockedDown")

		if res.Damage > 0 {
			foundPartial = true
			// Damage reported must have actually been applied to the pool,
			// not just computed and dropped.
			assert.Less(t, defender.Health, healthBefore,
				"a reported partial-damage amount must be reflected in the defender's health pool")
		}
	}

	assert.True(t, foundPartial,
		"expected at least one defended attempt to deal partial damage across %d iterations", iterations)
}
