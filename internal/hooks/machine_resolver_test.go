package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The whole feature is gated on this package's init() side effect. If the
// resolver is not installed -- an import trimmed, the init edited, a
// mutation-test revert left in by accident -- combatphase silently returns to
// its pre-2026-08-30 state: Attackers() empty, prone recovery a free stand,
// companion assist dead. Nothing else in the suite notices, because every
// other test that needs it would just see "no attacker recorded" and pass.
//
// This test exists because exactly that happened: a mutation experiment was
// committed and survived two commits with a fully green suite.
func TestResolverIsInstalledByInit(t *testing.T) {
	u := users.NewTestUser(5150, "installcheck", "Installcheck", 9)
	u.Character.SetUserId(5150)
	restore := users.SeedUsersForTest(map[int]*users.UserRecord{5150: u})
	t.Cleanup(restore)

	// Goes through combatphase's OWN lookup path, not the local function, so
	// this fails if SetMachineResolver was never called.
	m := combatphase.LookupForTest(state.ActorRef{UserId: 5150})
	require.NotNil(t, m,
		"combatphase has no machine resolver installed: internal/hooks init() "+
			"must call combatphase.SetMachineResolver")
	assert.Same(t, u.Character.CombatPhase, m)
}
