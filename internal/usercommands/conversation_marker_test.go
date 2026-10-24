package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func rosterFor(t *testing.T, user *users.UserRecord) string {
	t.Helper()
	room := mustRoom(t, user)
	return strings.Join(rooms.GetDetails(room, user).VisibleMobs, "\n")
}

func TestMarker_RosterShowsInfoMarkerUntilHeard(t *testing.T) {
	user, _, done := talkFixture(t)
	defer done()
	roster := rosterFor(t, user)
	require.Contains(t, roster, "Skeleton")
	require.Contains(t, roster, "[*]")

	user.Character.MarkHeard("1.secret")
	roster = rosterFor(t, user)
	require.Contains(t, roster, "Skeleton")
	require.NotContains(t, roster, "[*]")
}

func TestMarker_NoMarkerWithoutFullSight(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	require.Contains(t, rosterFor(t, user), "[*]", "precondition: marker shows in the lit room")

	const darkRoom = 2
	lit := mustRoom(t, user)
	dark := rooms.LoadRoom(darkRoom)
	require.NotNil(t, dark)
	litId := lit.RoomId
	lit.RemovePlayer(user.UserId)
	lit.RemoveMob(mob.InstanceId)
	user.Character.RoomId = darkRoom
	mob.Character.RoomId = darkRoom
	dark.AddPlayer(user.UserId)
	dark.AddMob(mob.InstanceId)
	defer func() {
		dark.RemovePlayer(user.UserId)
		dark.RemoveMob(mob.InstanceId)
		user.Character.RoomId = litId
		mob.Character.RoomId = litId
		lit.AddPlayer(user.UserId)
		lit.AddMob(mob.InstanceId)
	}()

	// Positive controls: the move really happened, the viewer does not see
	// the room in full, and the mob is in room 2's list.
	require.NotEqual(t, messaging.SightFull, messaging.ParticipantSight(user.Character, dark))
	require.Contains(t, dark.GetMobs(), mob.InstanceId)
	require.Equal(t, darkRoom, user.Character.RoomId)
	require.Equal(t, "[*]", conversation.Marker(user.UserId, mob), "the mob still has a marker to hide")

	require.NotContains(t, rosterFor(t, user), "[*]")
}

func TestMarker_NpcMarkerHookValue(t *testing.T) {
	user, mob, done := talkFixture(t)
	defer done()
	require.NotNil(t, rooms.NpcMarker, "internal/conversation installs the hook")
	require.Equal(t, "[*]", rooms.NpcMarker(user.UserId, mob))
	user.Character.MarkHeard("1.secret")
	require.Equal(t, "", rooms.NpcMarker(user.UserId, mob))
}
