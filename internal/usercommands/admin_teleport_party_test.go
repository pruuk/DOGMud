package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #428 review: `teleport <player> <room>` moved the party members standing in
// the ADMIN's room (and their charmed mobs) instead of the target's, because
// the member loop read the room the command was typed in. It now reads the
// room the target left.
//
// Rooms: A (10) holds the admin Adminna (3) and Carrow (4); B (11) holds the
// leader Bobrick (2) and his member Aliceia (1); C (12) is the destination.
// Carrow is in Bobrick's party too, but stands in A, away from the leader.
const (
	tpRoomA = 10
	tpRoomB = 11
	tpRoomC = 12
)

func teleportPartyScene(t *testing.T) (admin, leader, member, bystander *users.UserRecord) {
	t.Helper()
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		tpRoomA: {RoomId: tpRoomA, Zone: "TestZone", Title: "Room A", Lamp: rooms.LampPtr(60)},
		tpRoomB: {RoomId: tpRoomB, Zone: "TestZone", Title: "Room B", Lamp: rooms.LampPtr(60)},
		tpRoomC: {RoomId: tpRoomC, Zone: "TestZone", Title: "Room C", Lamp: rooms.LampPtr(60)},
	}, map[string]*rooms.ZoneConfig{
		"TestZone": {Name: "TestZone", RoomId: tpRoomA, RoomIds: map[int]struct{}{tpRoomA: {}, tpRoomB: {}, tpRoomC: {}}},
	}))

	member = users.NewTestUser(1, "alice", "Aliceia", 1001)
	leader = users.NewTestUser(2, "bob", "Bobrick", 1002)
	admin = users.NewTestUser(3, "adm", "Adminna", 1003)
	admin.Role = users.RoleAdmin
	bystander = users.NewTestUser(4, "cara", "Carrow", 1004)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: member, 2: leader, 3: admin, 4: bystander}))

	for _, u := range []*users.UserRecord{member, leader, admin, bystander} {
		roomId := tpRoomA
		if u == member || u == leader {
			roomId = tpRoomB
		}
		u.Character.RoomId = roomId
		rooms.LoadRoom(roomId).AddPlayer(u.UserId)
	}
	return admin, leader, member, bystander
}

func TestTeleport_TargetLeaderBringsTheirOwnRoomsPartyNotTheAdmins(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	admin, leader, member, bystander := teleportPartyScene(t)

	party := parties.New(leader.UserId)
	require.NotNil(t, party)
	defer party.Disband()
	for _, uid := range []int{member.UserId, bystander.UserId} {
		require.True(t, party.InvitePlayer(uid))
		require.True(t, party.AcceptInvite(uid))
	}

	_, err := Teleport(`Bobrick 12`, admin, rooms.LoadRoom(tpRoomA), 0)
	require.NoError(t, err)

	require.Equal(t, tpRoomC, leader.Character.RoomId, "the target lands")
	require.Equal(t, tpRoomC, member.Character.RoomId, "the member who stood with the leader follows")
	require.Equal(t, tpRoomA, bystander.Character.RoomId, "a member in the admin's room, away from the leader, stays")
	require.Equal(t, tpRoomA, admin.Character.RoomId, "the admin stays")
}

func TestTeleport_SelfTeleportStillMovesTheAdminsOwnParty(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	admin, _, _, bystander := teleportPartyScene(t)

	party := parties.New(admin.UserId)
	require.NotNil(t, party)
	defer party.Disband()
	require.True(t, party.InvitePlayer(bystander.UserId))
	require.True(t, party.AcceptInvite(bystander.UserId))

	_, err := Teleport(`12`, admin, rooms.LoadRoom(tpRoomA), 0)
	require.NoError(t, err)

	require.Equal(t, tpRoomC, admin.Character.RoomId, "the admin lands")
	require.Equal(t, tpRoomC, bystander.Character.RoomId, "the admin's member follows")
}
