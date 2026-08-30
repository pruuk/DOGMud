package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLiveMob builds a mob instance that is installed in the mobs instance map,
// which is what makes it resolvable. There is no registry to populate: since
// 2026-08-30 combatphase resolves an ActorRef straight from mobs/users, so
// being in that map IS being registered.
func newLiveMob(t *testing.T, instanceId, roomId, health int) *mobs.Mob {
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

// engage drives the FULL production path: SetAggro builds the actor ref,
// TransitionToEngaging resolves the target and calls RecordInboundAttacker.
//
// It deliberately does NOT hand-build a TransitionReason. An earlier version of
// this test did, and that skipped the very line the branch changed
// (engagement_storage.go's Actor: c.ActorRef()) -- reverting that line failed
// nothing. Going through SetAggro covers ActorRef carrying MobInstanceId AND
// SetAggro using it.
func engage(t *testing.T, attacker, defender *mobs.Mob) {
	t.Helper()
	attacker.Character.SetAggro(0, defender.InstanceId, characters.DefaultAttack)
}

// Before 2026-08-30 this returned nil for everyone, always, because
// Character.Attackers() was permanently empty -- and recoveryContest documents
// nil as a FREE STAND. Prone auto-recovery had never been contested.
func TestRecoveryContestIsLiveWithARealAttacker(t *testing.T) {
	defender := newLiveMob(t, 7201, 4242, 40)
	attacker := newLiveMob(t, 7202, 4242, 50)

	engage(t, attacker, defender)

	require.Equal(t, []state.ActorRef{{MobInstanceId: 7202}}, defender.Character.Attackers(),
		"SetAggro must record the attacker on the target, carrying MobInstanceId")

	assert.NotNil(t, recoveryContest(&defender.Character),
		"a living same-room attacker must produce a contest, not a free stand")
}

// The contest is deliberately narrow: only a LIVING holder counts.
func TestRecoveryContestFreeStandWhenAttackerIsDead(t *testing.T) {
	defender := newLiveMob(t, 7205, 4242, 40)
	attacker := newLiveMob(t, 7206, 4242, 50)

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
	defender := newLiveMob(t, 7207, 4242, 40)
	attacker := newLiveMob(t, 7208, 4242, 50)

	engage(t, attacker, defender)
	require.Len(t, defender.Character.Attackers(), 1, "precondition: recorded")
	attacker.Character.RoomId = 9999

	assert.Nil(t, recoveryContest(&defender.Character),
		"a holder in another room must not contest the stand")
}

// A despawned attacker resolves to nothing and is a free stand, not a panic.
// This is the case that used to accumulate: nothing clears a stale inbound
// entry when its owner despawns without dying.
func TestRecoveryContestFreeStandWhenAttackerDespawned(t *testing.T) {
	defender := newLiveMob(t, 7209, 4242, 40)
	attacker := newLiveMob(t, 7210, 4242, 50)

	engage(t, attacker, defender)
	require.Len(t, defender.Character.Attackers(), 1, "precondition: recorded")
	mobs.DestroyInstance(7210)

	assert.Nil(t, recoveryContest(&defender.Character),
		"an unresolvable holder must be a free stand")
}
