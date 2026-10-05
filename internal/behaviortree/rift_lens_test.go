package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Obelisk's Lenses (rift mobs 9835-9839) each fight by their own tree.
// Every one must compile: an unknown action or condition is a load error, and
// a tree that fails to load leaves the mob with no brain at all in game.
func TestRiftLensTrees_Compile(t *testing.T) {
	for _, f := range []string{
		`9835-glint_stalker.yaml`,
		`9836-spine_lattice.yaml`,
		`9837-splitlight.yaml`,
		`9838-the_watching_obelisk.yaml`,
		`9839-facet_hunter.yaml`,
	} {
		_, err := LoadTreeFromFile(`../../_datafiles/world/dogmud/behaviors/rift_obelisk/` + f)
		assert.NoError(t, err, f)
	}
}

// vanish: a mob in a fight breaks off and tries to hide where it stands. Its
// own target is dropped, the player fighting it lets go of it, and it sneaks
// (the Awareness machine: with nobody else to spot it, it is hidden). A
// second vanish while hidden does nothing.
func TestActVanish_BreaksOffAndHides(t *testing.T) {
	const roomId, instId, userId = 20001, 93000, 7

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		hiddenConditionForTest: {ConditionId: hiddenConditionForTest, Name: `Hidden`},
	}))

	stalker := &mobs.Mob{MobId: 9835, InstanceId: instId, Character: *characters.New()}
	stalker.Character.Name = `Glint Stalker`
	stalker.Character.RoomId = roomId
	stalker.Character.Health = 50
	stalker.Character.Stamina = 100
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{9835: stalker}, map[int]*mobs.Mob{instId: stalker}))

	u := users.NewUserRecord(userId, 0)
	u.Character = characters.New()
	u.Character.Name = `Tester`
	u.Character.RoomId = roomId
	u.Character.SetUserId(userId)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{userId: u}))

	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{roomId: {RoomId: roomId, Title: `Facets`}}, map[string]*rooms.ZoneConfig{}))
	room := rooms.LoadRoom(roomId)
	room.AddPlayer(userId)
	room.AddMob(instId)

	targeting.Commit(&stalker.Character, state.ActorRef{UserId: userId}, targeting.ReasonAttack)
	targeting.Commit(u.Character, state.ActorRef{MobInstanceId: instId}, targeting.ReasonAttack)
	require.True(t, stalker.Character.IsInCombat())
	require.True(t, u.Character.IsInCombat())

	ctx := &EvalContext{InstanceId: instId, RoomId: roomId, Event: EventContext{EventType: `mob_combat_round`}}
	assert.Equal(t, Success, actVanish(nil, ctx), `breaking off spends the round`)
	assert.False(t, stalker.Character.IsInCombat(), `it dropped its target`)
	assert.NotEqual(t, instId, u.Character.CurrentCombatTarget().MobInstanceId, `the player let go of it`)

	if stalker.Character.IsHidden() {
		assert.Equal(t, Failure, actVanish(nil, ctx), `already hidden`)
	}
}

const hiddenConditionForTest = 9
