package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
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
