package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Finding 3 / chunk 5.2 — `target` is a third harmful-action entry point.
//
// It blocked companions but not non-combatants and not player_attack_immune
// mobs, so a player already in combat could switch their swings onto a
// protected NPC that `attack` refuses outright.
// ---------------------------------------------------------------------------

// seedTargetTestMob registers a mob instance in the room and returns cleanup.
func seedTargetTestMob(t *testing.T, room *rooms.Room, instanceId int, name string, mutate func(*mobs.Mob)) func() {
	t.Helper()

	m := &mobs.Mob{
		MobId:      5,
		InstanceId: instanceId,
		HomeRoomId: room.RoomId,
		Character: characters.Character{
			Name:   name,
			RoomId: room.RoomId,
			Health: 100,
			Buffs:  buffs.New(),
		},
	}
	m.Character.HealthMax.Value = 100
	if mutate != nil {
		mutate(m)
	}

	mobs.SetInstanceForTest(instanceId, m)
	room.AddMob(instanceId)

	return func() {
		room.RemoveMob(instanceId)
		mobs.SetInstanceForTest(instanceId, nil)
	}
}

func TestTarget_RefusesAttackImmuneMob(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)

	defer seedTargetTestMob(t, room, 310, "Bandit", nil)()
	defer seedTargetTestMob(t, room, 311, "Caravan Guard", func(m *mobs.Mob) {
		m.PlayerAttackImmune = true
	})()

	// Engage the ordinary mob so the switch path is reachable.
	user.Character.SetAggro(0, 310, characters.DefaultAttack)
	defer user.Character.EndAggro()
	require.True(t, user.Character.IsInCombat())

	handled, err := Target("caravan guard", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	require.True(t, user.Character.IsInCombat())
	assert.Equal(t, 310, user.Character.CurrentCombatTarget().MobInstanceId,
		"target switch onto an attack-immune mob must be refused")
}

func TestTarget_RefusesNonCombatant(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)

	defer seedTargetTestMob(t, room, 312, "Bandit", nil)()
	defer seedTargetTestMob(t, room, 313, "Barkeep", func(m *mobs.Mob) {
		m.NonCombatant = true
	})()

	user.Character.SetAggro(0, 312, characters.DefaultAttack)
	defer user.Character.EndAggro()

	handled, err := Target("barkeep", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	require.True(t, user.Character.IsInCombat())
	assert.Equal(t, 312, user.Character.CurrentCombatTarget().MobInstanceId,
		"target switch onto a non-combatant must be refused")
}
