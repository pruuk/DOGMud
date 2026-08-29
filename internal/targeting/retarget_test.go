package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/require"
)

// TestCommit_RetargetKeepsTheStoresInAgreement is the U12c-0 fix seen from
// where it matters. CurrentCombatTarget is what the {target} and
// {targethealth} prompt tokens render (users/userrecord.prompt.go:541, :557),
// and before this slice it kept naming the PREVIOUS enemy after a successful
// target switch.
func TestCommit_RetargetKeepsTheStoresInAgreement(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{MobInstanceId: 100}, ReasonAttack)
	for i := 0; i < 10; i++ {
		c.CombatPhase.OnRoundTick()
	}
	require.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId)

	Commit(c, state.ActorRef{MobInstanceId: 200}, ReasonAttack)

	require.Equal(t, 200, c.Aggro.MobInstanceId, "Aggro holds the new target")
	require.Equal(t, 200, c.CurrentCombatTarget().MobInstanceId,
		"CurrentCombatTarget must follow the retarget; this is the prompt bug")
	require.Equal(t, 200, EngagementOf(c).Target.MobInstanceId,
		"and the seam's own query must agree")
}

// CommitTaunt takes the same path, and taunt is the game's most frequent
// retargeting mechanic, so it gets its own assertion rather than being
// assumed to follow.
func TestCommitTaunt_RetargetKeepsTheStoresInAgreement(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{UserId: 7}, ReasonAttack)
	for i := 0; i < 10; i++ {
		c.CombatPhase.OnRoundTick()
	}
	require.Equal(t, 7, c.CurrentCombatTarget().UserId)

	CommitTaunt(c, state.ActorRef{UserId: 9}, 4)

	require.Equal(t, 9, c.Aggro.UserId)
	require.Equal(t, 9, c.CurrentCombatTarget().UserId,
		"a taunt must pull the CombatPhase target too, not just Aggro")
}
