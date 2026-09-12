package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestCondPlayersInRoomNeedsSight(t *testing.T) {
	m, room := sightScene(t, "cave")
	u := users.NewTestUser(8110, "kesh", "Kesh", 98110)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8110: u}))
	room.AddPlayer(8110)

	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Failure, condPlayersInRoom(nil, ctx), "a blind mob finds nobody")

	require.NoError(t, m.Character.AddBuff(sightNightVisionBuffId, true))
	require.Equal(t, Success, condPlayersInRoom(nil, ctx), "night vision finds them")
}

// The case that keeps the Ironwind cave bosses working: a light carried by
// anyone lifts the darkness for everyone, so a mob with no night vision of its
// own still sees. If this fails, the gate is reading the biome instead of
// Room.GetVisibility.
func TestCondPlayersInRoomSeesWhenSomeoneCarriesLight(t *testing.T) {
	m, room := sightScene(t, "cave")
	u := users.NewTestUser(8111, "lume", "Lume", 98111)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8111: u}))
	room.AddPlayer(8111)
	require.NoError(t, u.Character.AddBuff(sightIlluminationBuffId, true))

	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Success, condPlayersInRoom(nil, ctx), "someone carrying light lets the mob see")
}

func TestCondMultipleEnemiesNeedsSight(t *testing.T) {
	m, room := sightScene(t, "cave")
	a := users.NewTestUser(8112, "kesh", "Kesh", 98112)
	b := users.NewTestUser(8113, "ordel", "Ordel", 98113)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8112: a, 8113: b}))
	room.AddPlayer(8112)
	room.AddPlayer(8113)

	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Failure, condMultipleEnemies(nil, ctx), "a blind mob counts no enemies")

	require.NoError(t, m.Character.AddBuff(sightNightVisionBuffId, true))
	require.Equal(t, Success, condMultipleEnemies(nil, ctx), "night vision counts both")
}

// Quest givers must NOT be sight-gated. Dewey (9491) and Cleric Hadwen (9100)
// have to keep offering their quests in an unlit room, so this condition stays
// ungated on purpose (spec ruling 5). If someone "consistently" gates it later,
// this test is what says no.
func TestQuestGiverConditionIsNotSightGated(t *testing.T) {
	m, room := sightScene(t, "cave")
	u := users.NewTestUser(8114, "kesh", "Kesh", 98114)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8114: u}))
	room.AddPlayer(8114)

	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	params := map[string]any{"quest": "slice-f-fixture-token"}
	require.Equal(t, Success, condPlayerInRoomMissingQuest(params, ctx),
		"a quest giver must still offer its quest in the dark")
}
