package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makePartialTestParams builds SkillMoveParams with a MODERATE defender
// advantage (attacker score ~100 vs a dodge score ~130 from the defender's
// Dexterity — the caller sets Dexterity 130). With RollSpread 0.15 this
// produces frequent bare/partial defensive wins alongside occasional
// defensive crits, so a loop of a few hundred iterations sees both a
// defended attempt with damage > 0 (the partial) and a bare miss.
//
// U6b Task 7: converted off the legacy scalar-defence shape (the flat
// DefenseStat 130 at x1 weights — a pinned defect) onto the seam, where the
// defender's side is their real equipment-gated defence set (dodge only for
// this bare-handed defender), charged and progressed per contest.
//
// Do NOT copy the plan's original 100-vs-5000 gap: at that gap every
// defensive win is a defensive crit (normalized margin far past the 2.0
// threshold), the multiplier is 0.0 every time, and "sometimes deals
// partial damage" can never pass.
func makePartialTestParams(attacker, defender *characters.Character) SkillMoveParams {
	return SkillMoveParams{
		Attacker: attacker, Defender: defender,
		Channel: ChannelMelee,
		Attack: AttackSide{
			Stat: 100, StatName: "strength",
			Skill: skills.WeaponCombat, SkillRank: 0,
			Mult: 1.0,
		},
		DamagePercent:   1.0,
		KnockdownFactor: 2.0, // maximal, so a leaked status/knockdown on a
		// defended attempt would be caught rather than hidden by a low roll.
		DamageStat:           100,
		MitigationMultiplier: 1.0,
	}
}

// TestExecuteSkillMove_DefendedAttemptDealsPartialDamageWithoutStatus is the
// Task 13 regression: a defended maneuver (Hit == false) must sometimes deal
// damage > 0 (the shared partial mechanism) while NEVER applying the binary
// status effect (StatusApplied / KnockedDown).
func TestExecuteSkillMove_DefendedAttemptDealsPartialDamageWithoutStatus(t *testing.T) {
	pinDefenceAdmissionConfig(t)

	const iterations = 2000

	foundPartial := false

	for i := 0; i < iterations; i++ {
		attacker := characters.New()
		defender := characters.New()
		// The defender's contest side is now their REAL dodge (seam), so the
		// moderate advantage lives on the character: Dexterity 130 vs the
		// attacker's flat 100 AttackSide score.
		setStatBase(t, &defender.Stats.Dexterity, 130)
		// Direct .Value/.Health writes are deliberate here, not the usual
		// "set .Base then Recalculate()" convention: nothing in this test
		// path recalculates stats mid-run, so there is no derived value to
		// go stale. Stamina funds the (now-charged) dodges across the loop.
		defender.HealthMax.Value = 100000
		defender.Health = 100000
		defender.StaminaMax.Value = 200
		defender.Stamina = 200

		healthBefore := defender.Health

		res := ExecuteSkillMove(makePartialTestParams(attacker, defender))

		if res.Hit {
			continue
		}

		// The status effect stays binary: a defended attempt must never
		// apply status or knock the defender down, regardless of damage.
		// require, not assert: a leak should fail the loop once, not once
		// per remaining iteration (up to ~2000 duplicate failures).
		require.False(t, res.StatusApplied, "a defended attempt must never set StatusApplied")
		require.False(t, res.KnockedDown, "a defended attempt must never set KnockedDown")

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
