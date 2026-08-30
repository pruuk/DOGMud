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
//
// Uses a SENTINEL rather than asserting IsZero(): Self() starts zero, so
// `assert.True(Self().IsZero())` cannot tell "the guard returned early" from
// "SetSelf(zero) ran and wrote a zero". It passed with the guard deleted.
func TestSyncMachineSelf_RefusesZeroRef(t *testing.T) {
	c := &Character{}
	c.CombatPhase = combatphase.NewMachine()
	c.CombatPhase.SetSelf(state.ActorRef{UserId: 99})

	c.SyncMachineSelf() // c has no identity of its own

	assert.Equal(t, state.ActorRef{UserId: 99}, c.CombatPhase.Self(),
		"a zero ref must not overwrite a recorded identity")
}

// Two systems tell a companion to join a fight: the reactive
// SubscribeAttackersChange handler and the polling handleCharmedMobAssist.
// Both guard on !IsInCombat(), which is NOT sufficient -- Command() only
// enqueues, so the flag stays false until the command runs, and both paths
// passed the guard in the same window. A playtest saw the result: the same
// companion announcing "prepares to fight X" in two consecutive rounds.
func TestTryClaimAssistCommand_OnePerRound(t *testing.T) {
	c := &Character{}

	assert.True(t, c.TryClaimAssistCommand(100), "first claim in a round wins")
	assert.False(t, c.TryClaimAssistCommand(100), "second claim in the SAME round must be refused")
	assert.True(t, c.TryClaimAssistCommand(101), "a new round claims again")
}

// The claim is per-character: one companion claiming must not silence another.
func TestTryClaimAssistCommand_IsPerCharacter(t *testing.T) {
	a, b := &Character{}, &Character{}

	require.True(t, a.TryClaimAssistCommand(7))
	assert.True(t, b.TryClaimAssistCommand(7),
		"a different companion must still be able to be commanded this round")
}
