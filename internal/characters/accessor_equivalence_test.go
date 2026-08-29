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
