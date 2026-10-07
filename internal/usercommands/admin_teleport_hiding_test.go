package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #428: the admin teleport arrival line went out on plain Room.SendText, so
// a shapes-only viewer (heat sight in an unlit room) read the arriver's
// name; and a party member's arrival was announced on the room they LEFT.
// Both now go through the destination room's name-hiding sight path.
//
// Aliceia (user 1, admin) leads a party with Bobrick (user 2) from room 1 to
// room 2, where Carrow (user 3) sees by heat only.
func TestTeleport_ArrivalHidesNamesFromAShapesViewer(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	useDogmudTemplates(t)
	darkenCraftRoom(t, 2)

	leader := users.GetByUserId(1)
	leader.Role = users.RoleAdmin
	member := users.GetByUserId(2)
	watcher := users.NewTestUser(3, "cara", "Carrow", 1003)
	watcher.Character.RoomId = 2
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: leader, 2: member, 3: watcher})
	defer restoreUsers()
	rooms.LoadRoom(2).AddPlayer(3)
	require.True(t, watcher.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))

	party := parties.New(1)
	require.NotNil(t, party)
	defer party.Disband()
	require.True(t, party.InvitePlayer(2))
	require.True(t, party.AcceptInvite(2))

	craftPlainLines(1)
	craftPlainLines(2)
	craftPlainLines(3)

	_, err := Teleport(`2`, leader, rooms.LoadRoom(1), 0)
	require.NoError(t, err)
	require.Equal(t, 2, leader.Character.RoomId, "fixture: the leader must land")
	require.Equal(t, 2, member.Character.RoomId, "fixture: the party must follow")

	seen := craftPlainLines(3)
	require.Equal(t, 0, craftCountContaining(seen, "Aliceia"), "arrival named the leader: %v", seen)
	require.Equal(t, 0, craftCountContaining(seen, "Bobrick"), "arrival named the member: %v", seen)
	require.Equal(t, 2, craftCountContaining(seen, "figure appears in a flash of light!"),
		"both arrivals land in the destination room as figures: %v", seen)
}
