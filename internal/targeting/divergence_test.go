package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two stores are kept in sync by a dual-write inside SetAggro, an
// invariant held by convention with nothing enforcing it. U12c deletes one of
// them; until then this is what stops them drifting while 88 call sites move.
func TestCommitAndRelease_KeepTheStoresInAgreement(t *testing.T) {
	cases := []struct {
		name string
		act  func(*characters.Character)
		// inCombat is what CombatPhase must report afterwards.
		inCombat bool
	}{
		{"commit to a mob", func(c *characters.Character) {
			Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)
		}, true},
		{"commit to a player", func(c *characters.Character) {
			Commit(c, state.ActorRef{UserId: 7}, ReasonAttack)
		}, true},
		{"commit a surprise", func(c *characters.Character) {
			Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonSurprise)
		}, true},
		{"commit after a wait", func(c *characters.Character) {
			CommitAfter(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack, 1)
		}, true},
		{"commit a taunt", func(c *characters.Character) {
			CommitTaunt(c, state.ActorRef{UserId: 7}, 4)
		}, true},
		{"release", func(c *characters.Character) {
			Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)
			Release(c, ReasonDisengage)
		}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := characters.New()
			require.NotNil(t, c.CombatPhase)

			tc.act(c)

			assert.Equal(t, tc.inCombat, c.CombatPhase.IsInCombat(),
				"CombatPhase must agree with the Aggro write")
			assert.Equal(t, tc.inCombat, c.Aggro != nil,
				"Aggro must agree with the CombatPhase transition")
			if tc.inCombat {
				e := EngagementOf(c)
				assert.False(t, e.Target.IsZero(),
					"a committed engagement must report a target")
			}
		})
	}
}
