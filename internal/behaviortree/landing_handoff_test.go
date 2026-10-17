package behaviortree

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// TestLandingHandoffNeedsARealTalk loads the shipped tree of The Landing
// (room 6467), where talking to Dewey ends the Waking to Gaius tutorial, and
// checks only a talk, say or ask the command itself would carry out takes the
// handoff. The 2026-10-17 playtest saw a bare `talk` end the tutorial; the
// commands refuse a bare verb, so it must fall through to that refusal. This
// is the class quest 28 fixed for its earlier steps by listening on success.
func TestLandingHandoffNeedsARealTalk(t *testing.T) {
	m, room := sightScene(t, "city")
	m.MobId = 9491
	m.Character.Name = "Dewey"
	room.AddMob(m.InstanceId)

	u := users.NewTestUser(8141, "verenna", "Verenna", 98141)
	u.Character.RoomId = room.RoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8141: u}))
	room.AddPlayer(8141)

	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	tree := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "behaviors", "rooms", "newcomer_antechamber", "6467.yaml")
	require.NoError(t, GetEngine().LoadRoomTree(room.RoomId, tree))
	t.Cleanup(func() { clearEngineRoomState(t, room.RoomId); clearRoomStates(t, room.RoomId) })

	try := func(cmd, rest string) bool {
		clearRoomStates(t, room.RoomId) // a fresh `leaving` flag for every attempt
		return TryRoomBehavior(room.RoomId, EventContext{
			EventType: "room_command", UserId: 8141, RoomId: room.RoomId, Command: cmd, Rest: rest,
		})
	}

	for _, c := range []struct{ cmd, rest string }{
		{"talk", ""}, {"talk", "to"}, {"talk", "nobody"},
		{"say", ""}, {"say", "   "},
		{"ask", ""}, {"ask", "dewey"}, {"ask", "nobody world"},
	} {
		require.False(t, try(c.cmd, c.rest), "%s %q is refused by the command, so it must not end the tutorial", c.cmd, c.rest)
	}
	for _, c := range []struct{ cmd, rest string }{
		{"talk", "dewey"}, {"talk", "to dewey"}, {"say", "hello"}, {"ask", "dewey quest"},
	} {
		require.True(t, try(c.cmd, c.rest), "%s %q is the real thing and must take the handoff", c.cmd, c.rest)
	}
}
