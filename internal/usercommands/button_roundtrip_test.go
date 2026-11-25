package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Guard 1 of the touch arc (round trip), movement slice (#781): every
// movement id the server issues, in a room holding a compass, a vertical, a
// diagonal, a named, a temporary, a locked and a found secret exit, and the
// six fixed ids the dark offers, becomes a line the real parser accepts and
// that reaches the exit it was issued for.
func TestButtons_EveryIssuedMovementIdRoundTrips(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	t.Cleanup(func() { buttons.Forget(user.UserId) })
	origExits, origTemp := room.Exits, room.ExitsTemp
	room.Exits = map[string]exit.RoomExit{
		"north":     {RoomId: 2},
		"up":        {RoomId: 2},
		"northeast": {RoomId: 2},
		"out":       {RoomId: 2},
		"east":      {RoomId: 2, Lock: gamelock.Lock{Difficulty: 10}},
		"west":      {RoomId: 2, Secret: true},
	}
	room.ExitsTemp = nil
	t.Cleanup(func() { room.Exits, room.ExitsTemp = origExits, origTemp })
	user.Character.LearnSecretExit(1, "west")
	require.True(t, room.AddTemporaryExit("portal", exit.TemporaryRoomExit{RoomId: 2}))

	known := rooms.KnownExits(room, user.Character)
	require.False(t, known.Dark, "premise: room 1 is lit")
	ids := buttons.MovementIds(user.UserId, room, known)
	require.Len(t, ids, 7)
	for name, id := range ids {
		line, ok := buttons.Resolve(user.UserId, room.RoomId, id)
		require.True(t, ok, "%s: id %q must resolve", name, id)
		require.True(t, isKnownCommandLine(user, room, line), "%s: %q must be a line the parser accepts", name, line)
		verb, rest := SplitInput(line)
		word := verb
		if verb == "go" {
			word = rest
		}
		got, _ := room.FindKnownExitByName(user.Character, word)
		require.Equal(t, name, got, "%s: %q must reach the exit it was issued for", name, line)
	}
	for dir, id := range buttons.FixedIds() {
		line, ok := buttons.Resolve(user.UserId, room.RoomId, id)
		require.True(t, ok)
		require.Equal(t, dir, line)
		require.True(t, isKnownCommandLine(user, room, line), "the dark's %s must parse", dir)
	}
}

// The round trip can fail: a line the parser does not take, and a word that
// reaches no exit, are caught by the same checks.
func TestButtons_RoundTripGuardCanFail(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	require.False(t, isKnownCommandLine(user, room, "gox north"))
	got, _ := room.FindKnownExitByName(user.Character, "southwest")
	require.NotEqual(t, "southwest", got)
}
