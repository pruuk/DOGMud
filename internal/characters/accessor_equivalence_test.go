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

// SetCast is the ONE writer that does not go through the targeting seam: it
// assigns Character.Aggro directly (spells.go:208) and never touches
// CombatPhase. So calling it while an engagement is live leaves the two stores
// disagreeing -- Aggro drops to zero ids, CombatPhase keeps the old target --
// and CurrentCombatTarget() reports the stale one.
//
// This is pinned as a KNOWN DISAGREEMENT rather than added to the table above,
// which it would fail. U12c-1 migrated ~241 reads onto the accessors on the
// strength of that table, so the exception has to be visible.
//
// It is not reachable in production today, but only because of a gate, not
// because the accessor is right:
//
//   - SetCast has exactly ONE production caller, mobcommands/aid.go:81.
//   - Aid requires room.IsCalm() -- no player and no mob in the room is
//     attacking -- so the caster cannot be mid-engagement when it fires.
//   - Nothing resolves a SetCast aggro into a spell effect. Aggro.SpellInfo is
//     read only by IsAggro, targeting.EngagementOf and
//     Death_InboundAggroCleanup; none of them apply the spell.
//
// U12c-2 owns the fix. Either SetCast moves onto the seam so both stores
// agree, or the SpellCast aggro type goes away with the field. If this test
// starts FAILING, that work happened and the assertion below should be
// inverted into an equivalence assertion, not deleted.
func TestAccessors_KnownDisagreement_SetCastOverALiveEngagement(t *testing.T) {
	c := New()
	c.SetAggro(0, 100, DefaultAttack)
	require.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId)

	c.SetCast(2, SpellAggroInfo{SpellId: "aidskill", TargetUserIds: []int{7}})

	require.NotNil(t, c.Aggro)
	assert.Equal(t, SpellCast, c.Aggro.Type)
	assert.Zero(t, c.Aggro.MobInstanceId, "SetCast writes a targetless Aggro")
	assert.Zero(t, c.Aggro.UserId, "SetCast writes a targetless Aggro")

	assert.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId,
		"KNOWN: CombatPhase still holds the pre-cast target, so the accessor "+
			"and the raw ids disagree. See this test's comment before changing it.")

	// The two accessors do not disagree about in-combat-ness, only about who.
	assert.Equal(t, c.Aggro != nil, c.IsInCombat())
}

// A cast from idle has nothing to go stale, so the accessors agree. Pinned as
// the boundary of the exception above: the disagreement needs a PRIOR
// engagement, it is not inherent to SetCast.
func TestAccessors_AgreeWhenSetCastComesFromIdle(t *testing.T) {
	c := New()
	c.SetCast(2, SpellAggroInfo{SpellId: "aidskill", TargetUserIds: []int{7}})

	require.NotNil(t, c.Aggro)
	assert.True(t, c.IsInCombat(), "a pending cast counts as in combat")
	assert.Equal(t, state.ActorRef{}, c.CurrentCombatTarget(),
		"and the accessor agrees the plain target is empty")

	// The aimed target lives in SpellInfo, which only IsAggro and
	// targeting.EngagementOf consult -- NOT CurrentCombatTarget.
	assert.True(t, c.IsAggro(7, 0), "IsAggro still sees the spell target")
}
