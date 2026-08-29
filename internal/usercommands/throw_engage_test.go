package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const throwerUserId = 7

// newTestThrower builds a bare player-side user for the engage helper.
func newTestThrower() *users.UserRecord {
	c := &characters.Character{
		Name:      "Thrower",
		RoomId:    1,
		Health:    400,
		Buffs:     buffs.New(),
		Cooldowns: map[string]int{},
	}
	c.HealthMax.Value = 400
	return &users.UserRecord{UserId: throwerUserId, Character: c}
}

func newTestThrowRoom() *rooms.Room {
	return &rooms.Room{RoomId: 1}
}

// Throwing used to be refused entirely out of combat, so a thrown explosive
// could never open a fight. Now that it can, the throw has to actually START
// the fight: without this the thrower bombs a room and neither side is
// engaged, which is strictly worse than the refusal it replaced.
func TestEngageAfterThrow_OpenerEngagesBothSides(t *testing.T) {
	user := newTestThrower()
	mobA := newTestMob(101)
	mobB := newTestMob(102)

	require.False(t, user.Character.IsInCombat(), "precondition: thrower is out of combat")
	require.False(t, mobA.Character.IsInCombat(), "precondition: mob A is out of combat")

	engageAfterThrow(user, newTestThrowRoom(), []*mobs.Mob{mobA, mobB})

	require.True(t, user.Character.IsInCombat(), "thrower did not enter combat after a hit")
	assert.Equal(t, 101, user.Character.CurrentCombatTarget().MobInstanceId,
		"thrower should engage the first mob the throw hit")

	require.True(t, mobA.Character.IsInCombat(), "mob A did not retaliate")
	assert.Equal(t, throwerUserId, mobA.Character.CurrentCombatTarget().UserId)

	require.True(t, mobB.Character.IsInCombat(), "mob B did not retaliate")
	assert.Equal(t, throwerUserId, mobB.Character.CurrentCombatTarget().UserId,
		"every mob caught in the blast must retaliate, not just the first")
}

// A throw landed mid-fight must not re-point anyone. Overwriting the thrower's
// aggro would yank them off their chosen target, and overwriting a mob's would
// pull it off whoever it was already fighting (a party member, say).
func TestEngageAfterThrow_DoesNotOverwriteExistingAggro(t *testing.T) {
	user := newTestThrower()
	user.Character.SetAggro(0, 999, characters.DefaultAttack)

	mobA := newTestMob(101)
	mobA.Character.SetAggro(42, 0, characters.DefaultAttack)

	engageAfterThrow(user, newTestThrowRoom(), []*mobs.Mob{mobA})

	require.True(t, user.Character.IsInCombat())
	assert.Equal(t, 999, user.Character.CurrentCombatTarget().MobInstanceId,
		"throw re-pointed the thrower at a different target")

	require.True(t, mobA.Character.IsInCombat())
	assert.Equal(t, 42, mobA.Character.CurrentCombatTarget().UserId,
		"throw pulled the mob off the player it was already fighting")
}

// A throw that hits nothing is not an attack on anyone, so it must not drag
// the thrower into a fight.
func TestEngageAfterThrow_NoHitsLeavesThrowerOutOfCombat(t *testing.T) {
	user := newTestThrower()

	engageAfterThrow(user, newTestThrowRoom(), nil)

	assert.False(t, user.Character.IsInCombat(), "a throw that hit nothing started a fight anyway")
}

// Mixed blast: one mob already fighting the thrower, one not. The already
// engaged mob keeps its aggro; the fresh one retaliates. This is the per-mob
// freshness rule that the single-target moves cannot express, since the
// attacker's own aggro can only point at one target at a time.
func TestEngageAfterThrow_MixedFreshnessPerMob(t *testing.T) {
	user := newTestThrower()

	alreadyFighting := newTestMob(101)
	alreadyFighting.Character.SetAggro(throwerUserId, 0, characters.DefaultAttack)
	bystander := newTestMob(102)

	engageAfterThrow(user, newTestThrowRoom(), []*mobs.Mob{alreadyFighting, bystander})

	require.True(t, alreadyFighting.Character.IsInCombat())
	assert.Equal(t, throwerUserId, alreadyFighting.Character.CurrentCombatTarget().UserId)

	require.NotNil(t, bystander.Character.Aggro,
		"a mob newly caught in the blast must be pulled into the fight")
	assert.Equal(t, throwerUserId, bystander.Character.CurrentCombatTarget().UserId)
}

func TestEngageAfterThrow_NilUserDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		engageAfterThrow(nil, newTestThrowRoom(), []*mobs.Mob{newTestMob(101)})
	})
}
