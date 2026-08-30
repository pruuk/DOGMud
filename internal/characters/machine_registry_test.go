package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActorRef_Player(t *testing.T) {
	c := &Character{}
	c.SetUserId(7)
	assert.Equal(t, state.ActorRef{UserId: 7}, c.ActorRef())
	assert.False(t, c.ActorRef().IsZero())
}

// A mob's ref must be non-zero. This is the exact condition that made
// RecordInboundAttacker early-return on ActorRef.IsZero() and is why a
// repaired registry alone would still never have recorded a mob attacker.
func TestActorRef_Mob(t *testing.T) {
	c := &Character{MobInstanceId: 42}
	assert.Equal(t, state.ActorRef{MobInstanceId: 42}, c.ActorRef())
	assert.False(t, c.ActorRef().IsZero())
}

func TestActorRef_UnidentifiedIsZero(t *testing.T) {
	c := &Character{}
	assert.True(t, c.ActorRef().IsZero())
}

func TestSyncMachineRegistry_RegistersUnderNonZeroRef(t *testing.T) {
	c := &Character{}
	c.CombatPhase = combatphase.NewMachine()
	c.SetUserId(11)

	got := combatphase.LookupMachineForTest(state.ActorRef{UserId: 11})
	assert.Same(t, c.CombatPhase, got)

	c.UnregisterMachines()
}

// The whole point of the guard. A zero ref is a SINGLE map key, so admitting
// it would alias every unidentified character onto one entry and hand combat
// another character's state machines.
func TestSyncMachineRegistry_RefusesZeroRef(t *testing.T) {
	c := &Character{}
	c.CombatPhase = combatphase.NewMachine()
	c.syncMachineRegistry()

	assert.Nil(t, combatphase.LookupMachineForTest(state.ActorRef{}))
}

func TestSyncMachineRegistry_RekeysOnIdentityChange(t *testing.T) {
	c := &Character{}
	c.CombatPhase = combatphase.NewMachine()
	c.SetUserId(21)
	c.SetUserId(22)

	assert.Nil(t, combatphase.LookupMachineForTest(state.ActorRef{UserId: 21}),
		"old binding must not leak")
	assert.Same(t, c.CombatPhase, combatphase.LookupMachineForTest(state.ActorRef{UserId: 22}))

	c.UnregisterMachines()
}

// CreateUser reaches SetUserId with no preceding Validate(), so the machines
// can still be nil. That must not panic; the later Validate() re-syncs.
func TestSyncMachineRegistry_NilMachinesDoNotPanic(t *testing.T) {
	c := &Character{}
	assert.NotPanics(t, func() { c.SetUserId(31) })

	c.CombatPhase = combatphase.NewMachine()
	c.syncMachineRegistry()
	assert.Same(t, c.CombatPhase, combatphase.LookupMachineForTest(state.ActorRef{UserId: 31}))

	c.UnregisterMachines()
}

func TestUnregisterMachines_RemovesBinding(t *testing.T) {
	c := &Character{}
	c.CombatPhase = combatphase.NewMachine()
	c.SetUserId(41)
	c.UnregisterMachines()

	assert.Nil(t, combatphase.LookupMachineForTest(state.ActorRef{UserId: 41}))
}

// Mobs get their identity BEFORE Validate(), so Validate() is where a mob's
// machines become registerable.
func TestValidateRegistersMobInstance(t *testing.T) {
	c := &Character{MobInstanceId: 5150}
	c.IsMob = true
	require.NoError(t, c.Validate())

	assert.Same(t, c.CombatPhase,
		combatphase.LookupMachineForTest(state.ActorRef{MobInstanceId: 5150}))

	c.UnregisterMachines()
}

// A mob TEMPLATE has MobInstanceId 0, so a binding leaked from a template
// would land on the zero key -- the mob-side twin of the player aliasing bug.
func TestValidateDoesNotRegisterMobTemplate(t *testing.T) {
	tmpl := &Character{}
	tmpl.IsMob = true
	require.NoError(t, tmpl.Validate())

	assert.Nil(t, combatphase.LookupMachineForTest(state.ActorRef{}))
}

// ResetForMobInstance runs on a shallow copy of the template. If it did not
// clear registeredRef, an instance would inherit the template's binding and
// unregister it out from under the template on despawn.
func TestResetForMobInstanceClearsRegisteredRef(t *testing.T) {
	c := &Character{}
	c.CombatPhase = combatphase.NewMachine()
	c.SetUserId(61)
	require.False(t, c.registeredRef.IsZero())

	c.ResetForMobInstance()
	assert.True(t, c.registeredRef.IsZero())

	combatphase.UnregisterMachine(state.ActorRef{UserId: 61})
}
