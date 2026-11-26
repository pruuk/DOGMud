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
		got, gotRoom := room.FindKnownExitByName(user.Character, word)
		require.Equal(t, name, got, "%s: %q must reach the exit it was issued for", name, line)
		require.Equal(t, issuedFor(t, known, name).RoomId, gotRoom, "%s: %q must lead where the button says", name, line)
	}
	for dir, id := range buttons.FixedIds() {
		line, ok := buttons.Resolve(user.UserId, room.RoomId, id)
		require.True(t, ok)
		require.Equal(t, dir, line)
		require.True(t, isKnownCommandLine(user, room, line), "the dark's %s must parse", dir)
	}
}

// issuedFor is the exit a button named name stands for: the first of that
// name in known, the one buttons.MovementIds and Room.Info keep when a
// temporary exit shares a permanent exit's name.
func issuedFor(t *testing.T, known rooms.KnownExitList, name string) rooms.KnownExit {
	t.Helper()
	for _, ke := range known.Exits {
		if ke.Name == name {
			return ke
		}
	}
	t.Fatalf("no known exit %q", name)
	return rooms.KnownExit{}
}

// Guard 1, a shared name: a temporary exit keyed like a room exit gets no
// button of its own, and the one button leads where typing the name does.
// With the room exit known, that is the room exit; with it an unfound
// secret, typing reaches the temporary exit, and so does the button.
func TestButtons_ASharedNameLeadsWhereTypingDoes(t *testing.T) {
	for name, tc := range map[string]struct {
		secret   bool
		wantRoom int
	}{
		"room exit known":             {secret: false, wantRoom: 2},
		"room exit an unfound secret": {secret: true, wantRoom: 3},
	} {
		t.Run(name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			user, room := getTestUserAndRoom(t)
			t.Cleanup(func() { buttons.Forget(user.UserId) })
			origExits, origTemp := room.Exits, room.ExitsTemp
			room.Exits = map[string]exit.RoomExit{"out": {RoomId: 2, Secret: tc.secret}}
			room.ExitsTemp = nil
			t.Cleanup(func() { room.Exits, room.ExitsTemp = origExits, origTemp })
			require.True(t, room.AddTemporaryExit("out", exit.TemporaryRoomExit{RoomId: 3}))

			known := rooms.KnownExits(room, user.Character)
			ids := buttons.MovementIds(user.UserId, room, known)
			require.Len(t, ids, 1, "one button for the shared name")
			line, ok := buttons.Resolve(user.UserId, room.RoomId, ids["out"])
			require.True(t, ok)
			require.Equal(t, "go out", line)
			_, rest := SplitInput(line)
			_, typedRoom := room.FindKnownExitByName(user.Character, rest)
			require.Equal(t, tc.wantRoom, typedRoom, "premise: where typing goes")
			require.Equal(t, typedRoom, issuedFor(t, known, "out").RoomId, "the button leads where typing does")
		})
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
