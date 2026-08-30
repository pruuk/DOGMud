package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRegisteredMob builds a mob instance that is BOTH in the mobs instance map
// and bound in the state-machine registry. Both halves are required:
// Attackers() reads the registry, but recoveryContest resolves each ref back to
// a Character through mobs.GetInstance.
func newRegisteredMob(t *testing.T, instanceId, roomId, health int) *mobs.Mob {
	t.Helper()

	m := &mobs.Mob{}
	m.InstanceId = instanceId
	m.Character.MobInstanceId = instanceId
	m.Character.RoomId = roomId
	require.NoError(t, m.Character.Validate())
	m.Character.Health = health

	mobs.SetInstanceForTest(instanceId, m)
	t.Cleanup(func() { mobs.DestroyInstance(instanceId) })

	return m
}

// engage drives the REAL production path: TransitionToEngaging looks the
// target up in the machine registry and calls RecordInboundAttacker on it.
// Calling RecordInboundAttacker directly would bypass lookupMachine and prove
// nothing about the wiring -- which is exactly what an earlier version of this
// test did, and it passed with registration disabled.
func engage(t *testing.T, attacker, defender *mobs.Mob) {
	t.Helper()
	require.NoError(t, attacker.Character.CombatPhase.TransitionToEngaging(
		combatphase.EngagingData{Target: defender.Character.ActorRef()},
		state.TransitionReason{
			Trigger: combatphase.TriggerAttackCommand,
			Actor:   attacker.Character.ActorRef(),
			Target:  defender.Character.ActorRef(),
		},
	))
}

// Until U11 this returned nil for everyone, always, because
// Character.Attackers() was always empty -- and recoveryContest documents nil
// as a FREE STAND. Prone auto-recovery had therefore never been contested.
// This test fails if the registry ever goes inert again.
func TestRecoveryContestIsLiveWithARegisteredAttacker(t *testing.T) {
	defender := newRegisteredMob(t, 7201, 4242, 40)
	attacker := newRegisteredMob(t, 7202, 4242, 50)

	engage(t, attacker, defender)
	require.Len(t, defender.Character.Attackers(), 1,
		"registry must be live: engaging a target must record the attacker on "+
			"it through lookupMachine. Attackers() was permanently empty before U11")

	assert.NotNil(t, recoveryContest(&defender.Character),
		"a living same-room attacker must produce a contest, not a free stand")
}

// The contest is deliberately narrow: only a LIVING holder counts.
func TestRecoveryContestFreeStandWhenAttackerIsDead(t *testing.T) {
	defender := newRegisteredMob(t, 7205, 4242, 40)
	attacker := newRegisteredMob(t, 7206, 4242, 50)

	// Engage while alive -- a dead mob is vetoed from engaging at all -- then
	// kill it, which is the order this happens in during a real fight.
	engage(t, attacker, defender)
	require.Len(t, defender.Character.Attackers(), 1, "precondition: recorded")
	attacker.Character.Health = 0

	assert.Nil(t, recoveryContest(&defender.Character),
		"a dead holder must not contest the stand")
}

// ...and only a SAME-ROOM holder counts.
func TestRecoveryContestFreeStandWhenAttackerIsInAnotherRoom(t *testing.T) {
	defender := newRegisteredMob(t, 7207, 4242, 40)
	attacker := newRegisteredMob(t, 7208, 9999, 50)

	engage(t, attacker, defender)
	require.Len(t, defender.Character.Attackers(), 1, "precondition: recorded")

	assert.Nil(t, recoveryContest(&defender.Character),
		"a holder in another room must not contest the stand")
}

// An unresolvable ref is a free stand, not a panic.
func TestRecoveryContestFreeStandWhenAttackerIsUnresolvable(t *testing.T) {
	defender := newRegisteredMob(t, 7203, 4242, 40)

	defender.Character.CombatPhase.RecordInboundAttacker(state.ActorRef{MobInstanceId: 999999})

	assert.Nil(t, recoveryContest(&defender.Character),
		"an unresolvable holder must still be a free stand")
}
