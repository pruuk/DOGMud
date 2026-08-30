package mobs

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The registry is process-global and lives for the life of the server. Without
// teardown, every mob instance ever spawned is retained forever.
func TestDestroyInstanceUnregistersMachines(t *testing.T) {
	m := &Mob{}
	m.Character.MobInstanceId = 9001
	m.InstanceId = 9001
	require.NoError(t, m.Character.Validate())

	ref := state.ActorRef{MobInstanceId: 9001}
	require.NotNil(t, combatphase.LookupMachineForTest(ref), "precondition: registered")

	mobInstancesMu.Lock()
	mobInstances[9001] = m
	mobInstancesMu.Unlock()

	DestroyInstance(9001)

	assert.Nil(t, combatphase.LookupMachineForTest(ref),
		"DestroyInstance must drop the registry binding or the map leaks")
}
