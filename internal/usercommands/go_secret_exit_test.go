package usercommands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
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

// gmcpCapture is a FunctionExporter whose SendGMCPEvent records each send.
type gmcpCapture struct{ sent *[]string }

func (g gmcpCapture) GetExportedFunction(name string) (any, bool) {
	if name != `SendGMCPEvent` {
		return nil, false
	}
	return func(userId int, module string, payload any) {
		*g.sent = append(*g.sent, fmt.Sprintf("gmcp %d %s %v", userId, module, payload))
	}, true
}

// bumpNorth runs Go("north") for user 1 in a fresh fixture whose room 1 has
// no north exit (secret false) or a secret one user 1 has not found, and
// returns everything the move sent: the player's text and GMCP, the other
// player's text, the room's messages and where the player stands.
func bumpNorth(t *testing.T, secret bool) string {
	t.Helper()
	cleanup := seedAllRegistries()
	defer cleanup()
	var sent []string
	exporters := functionExporters
	functionExporters = []FunctionExporter{gmcpCapture{sent: &sent}}
	defer func() { functionExporters = exporters }()

	user, room := getTestUserAndRoom(t)
	if secret {
		north := room.Exits["north"]
		north.Secret = true
		room.Exits["north"] = north
	} else {
		delete(room.Exits, "north")
	}
	events.DrainQueuedMessageEventsForTest(1)
	events.DrainQueuedMessageEventsForTest(2)
	events.DrainQueuedRoomMessagesForTest(1)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := []string{fmt.Sprintf("room %d", user.Character.RoomId)}
	for _, uid := range []int{1, 2} {
		for _, m := range events.DrainQueuedMessageEventsForTest(uid) {
			out = append(out, fmt.Sprintf("user %d: %+v", uid, m))
		}
	}
	for _, m := range events.DrainQueuedRoomMessagesForTest(1) {
		out = append(out, fmt.Sprintf("room: %+v", m))
	}
	out = append(out, sent...)
	return strings.Join(out, "\n")
}

// Ruling 1 and the spec decision: an unfound secret exit answers a compass
// word exactly as no exit does, every byte of text and GMCP alike.
func TestGo_AnUnfoundSecretExitBumpsLikeAWall(t *testing.T) {
	wall := bumpNorth(t, false)
	require.Contains(t, wall, "You're bumping into walls.", "premise: the wall bump is captured")
	require.Contains(t, wall, "gmcp 1 Room.WrongDir", "premise: the GMCP is captured")
	require.Equal(t, wall, bumpNorth(t, true), "an unfound secret exit must answer as no exit")
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

// Ruling 2: a party member who moves through a secret exit behind the leader
// learns it. Ruling 13: a companion moving with its owner learns it too.
func TestGo_FollowersLearnTheSecretExitTheLeaderTakes(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	secretExit(t, room, "north")
	user.Character.LearnSecretExit(1, "north")

	member := users.GetByUserId(2)
	p := parties.New(1)
	require.NotNil(t, p)
	t.Cleanup(p.Disband)
	require.True(t, p.InvitePlayer(2))
	require.True(t, p.AcceptInvite(2))
	companion := mobs.GetInstance(100)
	require.NotNil(t, companion)
	companion.Character.Charmed = characters.NewCharm(1, -1, "")
	require.False(t, member.Character.KnowsSecretExit(1, "north"), "premise")
	require.False(t, companion.Character.KnowsSecretExit(1, "north"), "premise")
	events.DrainQueuedUserInputsForTest(2)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 2, user.Character.RoomId)
	require.True(t, member.Character.KnowsSecretExit(1, "north"), "the party member walked behind the leader")
	require.Contains(t, events.DrainQueuedUserInputsForTest(2), "north", "the member's follow step is queued")
	require.True(t, companion.Character.KnowsSecretExit(1, "north"), "the companion walked behind its owner")
}
