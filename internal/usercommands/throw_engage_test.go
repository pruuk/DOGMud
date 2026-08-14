package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestThrower builds a bare player-side Character for the engage helper.
func newTestThrower() *characters.Character {
	c := &characters.Character{
		Name:      "Thrower",
		RoomId:    1,
		Health:    400,
		Buffs:     buffs.New(),
		Cooldowns: map[string]int{},
	}
	c.HealthMax.Value = 400
	return c
}

// Throwing used to be refused entirely out of combat, so a thrown explosive
// could never open a fight. Now that it can, the throw has to actually START
// the fight: without this the thrower bombs a room and neither side is
// engaged, which is strictly worse than the refusal it replaced.
func TestEngageAfterThrow_OpenerEngagesBothSides(t *testing.T) {
	const throwerUserId = 7

	thrower := newTestThrower()
	mobA := newTestMob(101)
	mobB := newTestMob(102)

	require.Nil(t, thrower.Aggro, "precondition: thrower is out of combat")
	require.Nil(t, mobA.Character.Aggro, "precondition: mob A is out of combat")

	engageAfterThrow(throwerUserId, thrower, []*mobs.Mob{mobA, mobB})

	require.NotNil(t, thrower.Aggro, "thrower did not enter combat after a hit")
	assert.Equal(t, 101, thrower.Aggro.MobInstanceId,
		"thrower should engage the first mob the throw hit")

	require.NotNil(t, mobA.Character.Aggro, "mob A did not retaliate")
	assert.Equal(t, throwerUserId, mobA.Character.Aggro.UserId)

	require.NotNil(t, mobB.Character.Aggro, "mob B did not retaliate")
	assert.Equal(t, throwerUserId, mobB.Character.Aggro.UserId)
}

// A throw landed mid-fight must not re-point anyone. Overwriting the thrower's
// aggro would yank them off their chosen target, and overwriting a mob's would
// pull it off whoever it was already fighting (a party member, say).
func TestEngageAfterThrow_DoesNotOverwriteExistingAggro(t *testing.T) {
	const throwerUserId = 7

	thrower := newTestThrower()
	thrower.SetAggro(0, 999, characters.DefaultAttack)

	mobA := newTestMob(101)
	mobA.Character.SetAggro(42, 0, characters.DefaultAttack)

	engageAfterThrow(throwerUserId, thrower, []*mobs.Mob{mobA})

	require.NotNil(t, thrower.Aggro)
	assert.Equal(t, 999, thrower.Aggro.MobInstanceId,
		"throw re-pointed the thrower at a different target")

	require.NotNil(t, mobA.Character.Aggro)
	assert.Equal(t, 42, mobA.Character.Aggro.UserId,
		"throw pulled the mob off the player it was already fighting")
}

// A throw that hits nothing is not an attack on anyone, so it must not drag
// the thrower into a fight.
func TestEngageAfterThrow_NoHitsLeavesThrowerOutOfCombat(t *testing.T) {
	thrower := newTestThrower()

	engageAfterThrow(7, thrower, nil)

	assert.Nil(t, thrower.Aggro, "a throw that hit nothing started a fight anyway")
}

func TestEngageAfterThrow_NilThrowerDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		engageAfterThrow(7, nil, []*mobs.Mob{newTestMob(101)})
	})
}
