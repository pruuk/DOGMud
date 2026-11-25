package usercommands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
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

// moveOnlookersTo2 puts Bobrick and the skeleton (mob 100) in room 2, lit at
// the faces band, for an arrival test.
func moveOnlookersTo2(t *testing.T) (*users.UserRecord, *mobs.Mob, *rooms.Room) {
	t.Helper()
	room1, room2 := rooms.LoadRoom(1), rooms.LoadRoom(2)
	lamp := room2.Lamp
	room2.Lamp = rooms.LampPtr(60)
	t.Cleanup(func() { room2.Lamp = lamp })
	watcher, skeleton := users.GetByUserId(2), mobs.GetInstance(100)
	require.NotNil(t, skeleton)
	room1.RemovePlayer(2)
	watcher.Character.RoomId = 2
	room2.AddPlayer(2)
	room1.RemoveMob(100)
	skeleton.Character.RoomId = 2
	room2.AddMob(100)
	return watcher, skeleton, room2
}

// Owner ruling 14 (#781): a player and an NPC who watch someone leave through
// a secret exit learn it.
func TestGo_OnlookersLearnTheSecretExitTheyWatchSomeoneLeaveBy(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	secretExit(t, room, "north")
	user.Character.LearnSecretExit(1, "north")
	watcher, skeleton := users.GetByUserId(2), mobs.GetInstance(100)
	require.Equal(t, messaging.SightFull, room.ParticipantSight(2), "fixture: the square is lit")
	require.False(t, watcher.Character.KnowsSecretExit(1, "north"), "premise")
	require.False(t, skeleton.Character.KnowsSecretExit(1, "north"), "premise")
	events.DrainQueuedMessagesForTest(2)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 2, user.Character.RoomId)
	got := drainedText(watcher)
	require.Contains(t, got, "leaves", "premise: the watcher read the leave line")
	require.Contains(t, got, "north", "premise: the line names the exit")
	require.True(t, watcher.Character.KnowsSecretExit(1, "north"), "the watcher saw the exit used")
	require.True(t, skeleton.Character.KnowsSecretExit(1, "north"), "the NPC onlooker saw it too")
}

// Ruling 14: onlookers who watch someone arrive learn the secret exit the
// arrive line names, the one in their room leading back. The mover's own
// knowledge is not changed by it.
func TestGo_OnlookersLearnTheSecretExitTheyWatchSomeoneArriveBy(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	watcher, skeleton, room2 := moveOnlookersTo2(t)
	secretExit(t, room2, "south")
	require.Equal(t, messaging.SightFull, room2.ParticipantSight(2), "fixture: room 2 is lit")
	events.DrainQueuedMessagesForTest(2)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 2, user.Character.RoomId)
	got := drainedText(watcher)
	require.Contains(t, got, "enters from", "premise: the watcher read the arrive line")
	require.Contains(t, got, "south", "premise: the line names the exit")
	require.True(t, watcher.Character.KnowsSecretExit(2, "south"), "the watcher saw the exit used")
	require.True(t, skeleton.Character.KnowsSecretExit(2, "south"), "the NPC onlooker saw it too")
	require.False(t, user.Character.KnowsSecretExit(2, "south"), "the mover's own knowledge is not this rule's")
}

// Ruling 14: an onlooker who cannot see the mover (a pitch-dark room) hears
// the move and learns nothing, leaving or arriving.
func TestGo_OnlookersInTheDarkLearnNothing(t *testing.T) {
	t.Run("leaving", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		user, room := getTestUserAndRoom(t)
		secretExit(t, room, "north")
		user.Character.LearnSecretExit(1, "north")
		darkenRoom(t, room)
		watcher, skeleton := users.GetByUserId(2), mobs.GetInstance(100)
		require.Equal(t, messaging.SightNone, room.ParticipantSight(2), "fixture: the square is dark")
		events.DrainQueuedMessagesForTest(2)

		handled, err := Go("north", user, room, 0)
		require.True(t, handled)
		require.NoError(t, err)
		require.Equal(t, 2, user.Character.RoomId)
		require.Contains(t, drainedText(watcher), "You hear someone leave the room.", "premise: the sound line")
		require.False(t, watcher.Character.KnowsSecretExit(1, "north"))
		require.False(t, skeleton.Character.KnowsSecretExit(1, "north"))
	})
	t.Run("arriving", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		user, room := getTestUserAndRoom(t)
		watcher, skeleton, room2 := moveOnlookersTo2(t)
		secretExit(t, room2, "south")
		darkenRoom(t, room2)
		require.Equal(t, messaging.SightNone, room2.ParticipantSight(2), "fixture: room 2 is dark")
		events.DrainQueuedMessagesForTest(2)

		handled, err := Go("north", user, room, 0)
		require.True(t, handled)
		require.NoError(t, err)
		require.Equal(t, 2, user.Character.RoomId)
		require.Contains(t, drainedText(watcher), "You hear someone enter the room.", "premise: the sound line")
		require.False(t, watcher.Character.KnowsSecretExit(2, "south"))
		require.False(t, skeleton.Character.KnowsSecretExit(2, "south"))
	})
}

// Ruling 14 needs a line to read: a sneaking mover sends no leave or arrive
// line, so Bobrick, left behind, and the skeleton waiting in room 2 learn
// nothing about the secret exits on either side.
func TestGo_ASneakingMoverTeachesNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	secretExit(t, room, "north")
	user.Character.LearnSecretExit(1, "north")
	watcher, skeleton := users.GetByUserId(2), mobs.GetInstance(100)
	room2 := rooms.LoadRoom(2)
	lamp := room2.Lamp
	room2.Lamp = rooms.LampPtr(60)
	t.Cleanup(func() { room2.Lamp = lamp })
	secretExit(t, room2, "south")
	room.RemoveMob(100)
	skeleton.Character.RoomId = 2
	room2.AddMob(100)
	user.Character.SetMiscData(`sneaking`, true)
	t.Cleanup(func() { user.Character.SetMiscData(`sneaking`, nil) })
	events.DrainQueuedMessagesForTest(2)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 2, user.Character.RoomId, "fixture: the move happened")
	require.NotContains(t, drainedText(watcher), "leaves", "premise: no leave line for a sneak")
	require.False(t, watcher.Character.KnowsSecretExit(1, "north"))
	require.False(t, skeleton.Character.KnowsSecretExit(2, "south"))
}

// Ruling 14 touches secret exits only: watching someone use an ordinary exit
// records nothing.
func TestGo_WatchingAnOrdinaryExitRecordsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	watcher, skeleton := users.GetByUserId(2), mobs.GetInstance(100)
	events.DrainQueuedMessagesForTest(2)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Contains(t, drainedText(watcher), "leaves", "premise: the watcher read the leave line")
	require.Empty(t, watcher.Character.Discoveries)
	require.Empty(t, skeleton.Character.Discoveries)
}

// Ruling 4: a T direction tried in the dark is marked, typed or tapped (both
// run Go): a cross after the bump line, a padlock after the locked line, and
// Room.Info goes out at once. A lit room marks nothing.
func TestGo_DarkTriesLeaveMarks(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	util.SetRoundCountForTest(1000)
	t.Cleanup(util.ResetRoundCountForTest)
	user, room := getTestUserAndRoom(t)
	t.Cleanup(func() { buttons.Forget(user.UserId) })
	orig := room.Exits["north"]
	locked := orig
	locked.Lock = gamelock.Lock{Difficulty: 10}
	room.Exits["north"] = locked
	t.Cleanup(func() { room.Exits["north"] = orig })

	_, _ = Go("west", user, room, 0)
	require.Empty(t, buttons.DarkMarks(user.UserId, room.RoomId, true), "a lit bump marks nothing")

	darkenRoom(t, room)
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(user.Character, room), "premise: the room is dark")
	events.DrainQueuedRoomExitsChangedForTest(user.UserId)

	_, _ = Go("west", user, room, 0)
	_, _ = Go("north", user, room, 0)
	require.Equal(t, 1, user.Character.RoomId, "premise: the locked door held")
	require.Equal(t,
		map[string]string{"west": buttons.MarkWall, "north": buttons.MarkLocked},
		buttons.DarkMarks(user.UserId, room.RoomId, true))
	require.NotEmpty(t, events.DrainQueuedRoomExitsChangedForTest(user.UserId), "Room.Info must go out at once")
}

// Task 4's rule reaches the marks: a guess in the dark at a secret exit the
// player has not found marks a cross, as a wall does, even when that exit is
// locked. A padlock there would tell them a door stands in the wall.
func TestGo_ADarkGuessAtAnUnfoundSecretExitMarksAWall(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	util.SetRoundCountForTest(1000)
	t.Cleanup(util.ResetRoundCountForTest)
	user, room := getTestUserAndRoom(t)
	t.Cleanup(func() { buttons.Forget(user.UserId) })
	t.Cleanup(func() { events.DrainQueuedRoomExitsChangedForTest(user.UserId) })
	secretExit(t, room, "north")
	north := room.Exits["north"]
	north.Lock = gamelock.Lock{Difficulty: 10}
	room.Exits["north"] = north
	darkenRoom(t, room)
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(user.Character, room), "premise: the room is dark")

	_, _ = Go("north", user, room, 0)
	_, _ = Go("west", user, room, 0)
	require.Equal(t, 1, user.Character.RoomId, "premise: the guess went nowhere")
	require.Equal(t,
		map[string]string{"north": buttons.MarkWall, "west": buttons.MarkWall},
		buttons.DarkMarks(user.UserId, room.RoomId, true),
		"the unfound secret exit must mark exactly as the plain wall beside it")
}

// darkWithDirectionAliases is the fixture for the alias tests: user 1 in a
// pitch dark room 1 (north to room 2, no west exit), with the shipped
// direction aliases n and w. Cleanups run in the right order through the
// returned func, which the caller defers.
func darkWithDirectionAliases(t *testing.T) (*users.UserRecord, *rooms.Room, func()) {
	t.Helper()
	cleanup := seedAllRegistries()
	cleanupKeywords := keywords.SeedKeywordsForTest(keywords.Aliases{
		DirectionAliases: map[string]string{"n": "north", "w": "west"},
	})
	util.SetRoundCountForTest(1000)
	user, room := getTestUserAndRoom(t)
	darkenRoom(t, room)
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(user.Character, room), "premise: the room is dark")
	return user, room, func() {
		events.DrainQueuedRoomExitsChangedForTest(user.UserId)
		buttons.Forget(user.UserId)
		util.ResetRoundCountForTest()
		cleanupKeywords()
		cleanup()
	}
}

// The typed short alias reaches the marks through the real dispatcher:
// TryCommand expands w to west (actions.ExpandAliases), and the bump marks.
func TestTryCommand_ATypedDirectionAliasInTheDarkMarksAWall(t *testing.T) {
	user, room, done := darkWithDirectionAliases(t)
	defer done()

	handled, err := TryCommand("w", "", user.UserId, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, user.Character.RoomId, "premise: there is no west exit")
	require.Equal(t, map[string]string{"west": buttons.MarkWall},
		buttons.DarkMarks(user.UserId, room.RoomId, true))
}

// `go w` answers as `w` does: Go expands a direction alias before it asks
// whether the word is a compass direction, so the bump line and the mark
// come either way.
func TestGo_ADirectionAliasBumpsAndMarksAsTypingItDoes(t *testing.T) {
	user, room, done := darkWithDirectionAliases(t)
	defer done()
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Go("w", user, room, 0)
	require.True(t, handled, "go w must be handled, as w is")
	require.NoError(t, err)
	require.Contains(t, drainedText(user), "You're bumping into walls.")
	require.Equal(t, map[string]string{"west": buttons.MarkWall},
		buttons.DarkMarks(user.UserId, room.RoomId, true))
}

// The alias expansion leaves every other word alone: `go gate` takes a named
// exit, `go portal` a temporary one, and `go n` still walks north.
func TestGo_NamedAndTemporaryExitsAreUnchangedByAliasExpansion(t *testing.T) {
	for _, tc := range []struct{ word, exitName string }{
		{"gate", "gate"}, {"portal", "portal"}, {"n", "north"},
	} {
		t.Run(tc.word, func(t *testing.T) {
			// seedAllRegistries builds room 1 afresh, so these edits end
			// with the test.
			user, room, done := darkWithDirectionAliases(t)
			defer done()
			room.ExitsTemp = map[string]exit.TemporaryRoomExit{}
			switch tc.exitName {
			case "gate":
				room.Exits = map[string]exit.RoomExit{"gate": {RoomId: 2}}
			case "portal":
				room.Exits = map[string]exit.RoomExit{}
				require.True(t, room.AddTemporaryExit("portal", exit.TemporaryRoomExit{RoomId: 2, Expires: "1 day"}))
			}

			handled, err := Go(tc.word, user, room, 0)
			require.True(t, handled)
			require.NoError(t, err)
			require.Equal(t, 2, user.Character.RoomId, "go %s walks through the %s exit", tc.word, tc.exitName)
			require.Empty(t, buttons.DarkMarks(user.UserId, room.RoomId, true), "a move marks nothing")
		})
	}
}
