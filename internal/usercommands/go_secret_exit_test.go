package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// darkenRoom makes the fixture room pitch dark (biome cave, SkyLight 0, no
// lamp) until the test ends.
func darkenRoom(t *testing.T, room *rooms.Room) {
	t.Helper()
	biome, lamp := room.Biome, room.Lamp
	room.Biome, room.Lamp = "cave", nil
	t.Cleanup(func() { room.Biome, room.Lamp = biome, lamp })
}

func drainedText(u *users.UserRecord) string {
	return strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "\n")
}

// #781 decision D5: in the dark the exits line would read "None", which is not
// true. `exits` answers as look does.
func TestExits_InTheDarkSaysItIsTooDark(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	user, room := getTestUserAndRoom(t)
	darkenRoom(t, room)
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Exits("", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	got := drainedText(user)
	require.Contains(t, got, "It is too dark to see.")
	require.NotContains(t, got, "None")
}

// #781: peering from a lit room into a pitch-dark one shows the far room as
// look does, but the exits line lists nothing at no sight there, so it must
// not claim "Exits: None". The line is left out, as the roster is.
func TestLook_PeerIntoADarkRoomClaimsNoExits(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	user, room := getTestUserAndRoom(t)
	// Bright enough here to see through an exit (LightExitsAbove).
	lamp := room.Lamp
	room.Lamp = rooms.LampPtr(100)
	t.Cleanup(func() { room.Lamp = lamp })
	far := rooms.LoadRoom(2)
	require.NotNil(t, far)
	darkenRoom(t, far)
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Look("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	got := drainedText(user)
	require.Contains(t, got, "A damp, dark cave.", "fixture premise: the peer reached the far room")
	require.NotContains(t, got, "Exits:")
}

// secretExit makes room's exit name secret (adding it, to room 2, when the
// room has none) until the test ends.
func secretExit(t *testing.T, room *rooms.Room, name string) {
	t.Helper()
	orig, had := room.Exits[name]
	ex := orig
	if !had {
		ex = exit.RoomExit{RoomId: 2}
	}
	ex.Secret = true
	room.Exits[name] = ex
	t.Cleanup(func() {
		if had {
			room.Exits[name] = orig
		} else {
			delete(room.Exits, name)
		}
	})
}

// Ruling 1 and the spec decision: an unfound secret exit answers a compass
// word exactly as no exit does.
func TestGo_AnUnfoundSecretExitBumpsLikeAWall(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	secretExit(t, room, "north")
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, user.Character.RoomId, "the player must not pass an unfound secret exit")
	require.Contains(t, drainedText(user), "You're bumping into walls.")
}

func TestGo_AFoundSecretExitWalks(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	secretExit(t, room, "north")
	user.Character.LearnSecretExit(1, "north")

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 2, user.Character.RoomId, "a found secret exit walks")
}

// A named secret exit answers "not recognized", as a word naming no exit does,
// and so does a prefix guess at it. Once found it is a move.
func TestTyped_AnUnfoundSecretNameIsNotRecognized(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	require.Equal(t, routeUnknown, fallbackRoute(user, room, "crack"), "premise: no crack exit at all")
	secretExit(t, room, "crack")
	for _, guess := range []string{"crack", "cra"} {
		require.Equal(t, routeUnknown, fallbackRoute(user, room, guess), "guess %q", guess)
		require.False(t, isKnownCommandLine(user, room, guess), "guess %q", guess)
	}
	handled, err := Go("crack", user, room, 0)
	require.False(t, handled, "go at an unfound named secret exit falls to not recognized")
	require.NoError(t, err)

	user.Character.LearnSecretExit(1, "crack")
	require.Equal(t, routeGo, fallbackRoute(user, room, "crack"))
}
