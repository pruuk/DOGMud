package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #371: walking into an instance that was not opened for the player is an
// ordinary refusal. MoveToRoom tells the player why; go must not add the
// server-fault line on top of it.
func TestGo_InstanceDenialIsNotAFault(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, _ := getTestUserAndRoom(t)

	const ephId = 2_000_000_123
	require.True(t, rooms.IsEphemeralRoomId(ephId), "fixture premise: the target id is ephemeral")

	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		1: {RoomId: 1, Zone: "TestZone", Title: "Porch",
			Exits: map[string]exit.RoomExit{"north": {RoomId: ephId}}},
		ephId: {RoomId: ephId, Zone: "Vault", Title: "Vault"},
	}, map[string]*rooms.ZoneConfig{
		"TestZone": {Name: "TestZone", RoomId: 1, RoomIds: map[int]struct{}{1: {}}},
		"Vault":    {Name: "Vault", RoomId: ephId, RoomIds: map[int]struct{}{ephId: {}}},
	}))
	room := rooms.LoadRoom(1)
	room.AddPlayer(user.UserId)
	user.Character.RoomId = 1
	user.Character.ActionPoints = 100

	inst := &rooms.ZoneInstance{
		InstanceId:      ephId,
		TemplateZone:    "Vault",
		AuthorizedUsers: []int{user.UserId + 1000},
		OwnerUserId:     user.UserId + 1000,
		PortalDuration:  "1m",
		OverworldRoomId: 1,
		EntryRoomId:     ephId,
		RoomIdMap:       map[int]int{1: ephId},
	}
	rooms.GetInstanceRegistry().Add(inst)
	t.Cleanup(func() { rooms.GetInstanceRegistry().Remove(inst) })

	hoodTestText(user.UserId)
	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, user.Character.RoomId, "fixture: the portal pushed the player back")

	got := hoodTestText(user.UserId)
	require.Contains(t, got, "It wasn't opened for you.", "the player is told why")
	require.NotContains(t, got, refusal.FailedText("move"), "a denial is not a server fault")
	require.NotContains(t, got, "bumping into walls", "the exit exists; the denial is the whole answer")
}
