package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/require"
)

// A refused transition must refuse the whole commit. Before U12c-0b, SetAggro
// wrote Aggro and then discarded the transition error, so a vetoed commit left
// Aggro holding a target the machine had rejected -- the two stores
// disagreeing by construction.
func TestSetAggro_RefusedTransitionWritesNothing(t *testing.T) {
	c := New()
	c.SetAggro(0, 100, DefaultAttack)
	require.True(t, c.IsInCombat(), "the first commit must land")
	require.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId)

	// Veto every subsequent target, as a dead or non-combatant target would.
	c.CombatPhase.RegisterTargetLifeCheck(func(state.ActorRef) bool { return false })

	c.SetAggro(0, 200, DefaultAttack)

	require.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId,
		"a refused commit must leave the previous target intact, not overwrite it")
	require.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId,
		"and the two stores must still agree")
}

// The nil-CombatPhase path is the legacy/fixture one and must keep writing,
// or every test that builds a bare Character loses the ability to set a target.
func TestSetAggro_NilCombatPhaseStillWrites(t *testing.T) {
	c := New()
	c.CombatPhase = nil

	c.SetAggro(0, 100, DefaultAttack)

	require.True(t, c.IsInCombat(), "with no machine there is nothing to refuse")
	require.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId)
}
