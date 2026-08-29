package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// U12c-1 depends entirely on this: IsInCombat() and CurrentCombatTarget() must
// agree with the raw .Aggro reads they are about to replace at ~241 sites.
//
// They did NOT agree before U12c-0 (a retarget left CombatPhase stale) or
// before U12c-0b (a vetoed commit left Aggro holding a rejected target). This
// test is what makes the migration mechanical rather than behavioural.
func TestAccessors_AgreeWithRawAggroReads(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Character)
	}{
		{"idle", func(c *Character) {}},
		{"engaged with a mob", func(c *Character) {
			c.SetAggro(0, 100, DefaultAttack)
		}},
		{"engaged with a player", func(c *Character) {
			c.SetAggro(7, 0, DefaultAttack)
		}},
		{"after a retarget", func(c *Character) {
			c.SetAggro(0, 100, DefaultAttack)
			for i := 0; i < 10; i++ {
				c.CombatPhase.OnRoundTick()
			}
			c.SetAggro(0, 200, DefaultAttack)
		}},
		{"after a retarget mid wind-up", func(c *Character) {
			c.SetAggro(0, 100, DefaultAttack)
			c.SetAggro(0, 200, DefaultAttack)
		}},
		{"after a release", func(c *Character) {
			c.SetAggro(0, 100, DefaultAttack)
			c.EndAggro()
		}},
		{"surprise engagement", func(c *Character) {
			c.SetAggro(0, 100, SurpriseAttack)
		}},
		{"no combat phase machine", func(c *Character) {
			c.CombatPhase = nil
			c.SetAggro(0, 100, DefaultAttack)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := New()
			tc.setup(c)

			assert.Equal(t, c.Aggro != nil, c.IsInCombat(),
				"IsInCombat() must agree with `Aggro != nil`")

			want := state.ActorRef{}
			if c.Aggro != nil {
				want = state.ActorRef{
					UserId:        c.Aggro.UserId,
					MobInstanceId: c.Aggro.MobInstanceId,
				}
			}
			assert.Equal(t, want, c.CurrentCombatTarget(),
				"CurrentCombatTarget() must agree with the raw Aggro ids")
		})
	}
}

// A vetoed commit is the case U12c-0b fixed. Pinned separately because it is
// the one where the two stores could previously disagree while both non-zero,
// which the table above cannot express.
func TestAccessors_AgreeAfterAVetoedCommit(t *testing.T) {
	c := New()
	c.SetAggro(0, 100, DefaultAttack)
	c.CombatPhase.RegisterTargetLifeCheck(func(state.ActorRef) bool { return false })

	c.SetAggro(0, 200, DefaultAttack)

	require.NotNil(t, c.Aggro)
	assert.Equal(t, 100, c.Aggro.MobInstanceId, "the refused commit changed nothing")
	assert.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId,
		"and the accessor agrees")
}

// U12c-2 landed: SetCast now records the cast on the Activity machine and no
// longer assigns Aggro, so the disagreement this used to pin is GONE.
//
// It was pinned as TestAccessors_KnownDisagreement_SetCastOverALiveEngagement,
// whose comment said: "If this test starts FAILING, that work happened and the
// assertion below should be inverted into an equivalence assertion, not
// deleted." This is that inversion.
//
// What used to happen: SetCast assigned c.Aggro directly and never touched
// CombatPhase, so calling it over a live engagement dropped Aggro to zero ids
// while CombatPhase kept the old target, and CurrentCombatTarget() reported the
// stale one.
func TestAccessors_AgreeAfterSetCastOverALiveEngagement(t *testing.T) {
	c := New()
	c.SetAggro(0, 100, DefaultAttack)
	require.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId)

	require.True(t, c.SetCast(2, SpellAggroInfo{SpellId: "aidskill", TargetUserIds: []int{7}}),
		"precondition: the cast was recorded")

	assert.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId,
		"the engagement is untouched by starting a cast; both stores agree")
	assert.True(t, c.IsInCombat())
	assert.True(t, c.IsCasting(), "and the cast is recorded on the Activity machine")

	// The aim lives in CastingData, never in the combat target.
	cd, ok := c.CastingData()
	require.True(t, ok)
	assert.Equal(t, []int{7}, cd.TargetUserIds)
	assert.True(t, c.IsAggro(7, 0), "IsAggro still sees the spell target")
}

// A cast from idle has nothing to go stale, so the accessors agree. Pinned as
// the boundary of the exception above: the disagreement needs a PRIOR
// engagement, it is not inherent to SetCast.
func TestAccessors_AgreeWhenSetCastComesFromIdle(t *testing.T) {
	c := New()
	require.True(t, c.SetCast(2, SpellAggroInfo{SpellId: "aidskill", TargetUserIds: []int{7}}))

	assert.True(t, c.IsCasting(), "the cast is recorded")
	assert.Equal(t, state.ActorRef{}, c.CurrentCombatTarget(),
		"a cast from idle sets no combat target")

	// ⚠️ U12c-2 BEHAVIOUR CHANGE, deliberate: a pending cast no longer counts
	// as "in combat". It used to, only because SetCast assigned Aggro and
	// IsInCombat fell back to `Aggro != nil`. Casting is an Activity, not a
	// combat phase, and conflating them is what let a cast look like an
	// engagement with no target -- the stale state ValidateAggro had a special
	// exemption for.
	assert.False(t, c.IsInCombat(), "casting is an activity, not an engagement")

	// The aim lives in CastingData, which only IsAggro and
	// targeting.EngagementOf consult -- NOT CurrentCombatTarget.
	assert.True(t, c.IsAggro(7, 0), "IsAggro still sees the spell target")
}
