package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const sightHeatConditionId = 9781 // infra reach 50, no nightvision: the Phantom's heat sense shape

// Lighting plan 5d, ruling D8: a mob acts on a figure it can make out. In a
// DIM room (the shapes band) with a player present, a mob with no
// infravision passes mobCanSee and condPlayersInRoom, where before 5d it saw
// nobody. Proven able to fail by reverting mobCanSee to SightFull only.
func TestMobActsOnShapesInADimRoom(t *testing.T) {
	m, room := sightScene(t, "cave")
	room.Lamp = rooms.LampPtr(30) // shapes for normal eyes: 25 <= 30 < 50
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(&m.Character, room), "fixture: the room must read shapes")

	u := users.NewTestUser(8130, "dim", "Dim", 98130)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8130: u}))
	room.AddPlayer(8130)

	require.True(t, mobCanSee(m, room), "a mob that can make out a shape acts on it")
	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Success, condPlayersInRoom(nil, ctx), "a dim room's mob finds the player it can make out")
}

// Heat reads shapes below any light: a heat-sensing mob acts in darkness its
// reach covers, from no light at all down to minus its reach.
func TestMobActsOnHeatInTheDark(t *testing.T) {
	m, room := sightScene(t, "cave")
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		sightHeatConditionId: {ConditionId: sightHeatConditionId, Name: "Test Heat Sense",
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 50}},
			Flags:   []conditions.Flag{conditions.InfraredVision}},
	}))
	require.False(t, mobCanSee(m, room), "fixture: without heat the unlit cave blinds it")
	require.NoError(t, m.Character.AddCondition(sightHeatConditionId, true))
	require.True(t, mobCanSee(m, room), "heat shows the room's shapes at light 0")
}

// A genuinely dark room, below the shapes floor, with no infravision on
// either side, still blinds a mob: D8 widens shapes, not darkness.
func TestMobStillSeesNothingInTrueDarkness(t *testing.T) {
	m, room := sightScene(t, "cave")
	u := users.NewTestUser(8131, "dark", "Dark", 98131)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8131: u}))
	room.AddPlayer(8131)

	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(&m.Character, room), "fixture: the cave must read nothing")
	require.False(t, mobCanSee(m, room))
	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Failure, condPlayersInRoom(nil, ctx))
}

// The combat darkness penalty still keys on full sight: D8 widens only mob
// decisions, never messaging.CanSeeSightImpairedOnly.
func TestCombatSightStaysFullOnly(t *testing.T) {
	m, room := sightScene(t, "cave")
	room.Lamp = rooms.LampPtr(30)
	require.True(t, mobCanSee(m, room))
	require.False(t, messaging.CanSeeSightImpairedOnly(&m.Character, room),
		"the combat predicate must stay SightFull only; D8 widens mob decisions alone")
}
