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

// A mob's ref must carry MobInstanceId. This is the exact condition that made
// RecordInboundAttacker early-return on ActorRef.IsZero(), so that no mob
// attacking anyone was ever recorded.
func TestActorRef_Mob(t *testing.T) {
	c := &Character{MobInstanceId: 42}
	assert.Equal(t, state.ActorRef{MobInstanceId: 42}, c.ActorRef())
	assert.False(t, c.ActorRef().IsZero())
}

func TestActorRef_UnidentifiedIsZero(t *testing.T) {
	c := &Character{}
	assert.True(t, c.ActorRef().IsZero())
}

// Mobs receive MobInstanceId before Validate(), so Validate() is where a mob's
// identity reaches its machine.
func TestValidateRecordsMobSelf(t *testing.T) {
	c := &Character{MobInstanceId: 5150}
	c.IsMob = true
	require.NoError(t, c.Validate())

	// Guard against the assertion degenerating to nil == nil if Validate ever
	// stops constructing the machine.
	require.NotNil(t, c.CombatPhase)
	assert.Equal(t, state.ActorRef{MobInstanceId: 5150}, c.CombatPhase.Self())
}

// Players receive userId after Validate(), so SetUserId is where identity
// reaches the machine.
func TestSetUserIdRecordsSelfAfterValidate(t *testing.T) {
	c := &Character{}
	require.NoError(t, c.Validate())
	require.NotNil(t, c.CombatPhase)
	require.True(t, c.CombatPhase.Self().IsZero(), "no identity yet at Validate time")

	c.SetUserId(11)
	assert.Equal(t, state.ActorRef{UserId: 11}, c.CombatPhase.Self())
}

// CreateUser reaches SetUserId with no preceding Validate(), so the machine can
// still be nil. That must not panic; the later Validate() records it.
func TestSyncMachineSelf_NilMachineDoesNotPanic(t *testing.T) {
	c := &Character{}
	assert.NotPanics(t, func() { c.SetUserId(31) })

	c.CombatPhase = combatphase.NewMachine()
	c.SyncMachineSelf()
	assert.Equal(t, state.ActorRef{UserId: 31}, c.CombatPhase.Self())
}

// A zero ref must never be recorded as an identity.
func TestSyncMachineSelf_RefusesZeroRef(t *testing.T) {
	c := &Character{}
	c.CombatPhase = combatphase.NewMachine()
	c.SyncMachineSelf()
	assert.True(t, c.CombatPhase.Self().IsZero())
}
