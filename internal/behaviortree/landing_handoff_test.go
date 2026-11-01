package behaviortree

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// landingScene loads The Landing's shipped tree with Dewey and one player in
// the room, and returns the room, the player and what Dewey says there.
func landingScene(t *testing.T) (*rooms.Room, *users.UserRecord, *[]string) {
	t.Helper()
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{
		QuestId: 28, Name: "Waking to Gaius",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "attacked"}, {Id: "tripped"}, {Id: "fled"}, {Id: "end"}},
	}))
	said := &[]string{}
	orig := actionRegistry["mob_say"]
	actionRegistry["mob_say"] = func(params map[string]any, ctx *EvalContext) Result {
		*said = append(*said, getStringParam(params, "text"))
		return Success
	}
	t.Cleanup(func() { actionRegistry["mob_say"] = orig })

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
	t.Cleanup(func() {
		DrainAllDelayedActionsForTest(t)
		events.DrainQueuedQuestsForTest()
		clearEngineRoomState(t, room.RoomId)
		clearRoomStates(t, room.RoomId)
	})
	return room, u, said
}

// The Landing (room 6467) since #375 phase 7: quest 28 ends at Dewey's
// hand-in topic, so the room no longer takes `talk dewey` itself. A talk,
// say or ask goes on to the command (and Dewey's talk menu); a walk is still
// refused while the quest is open, since the room has no exits. A player who
// already holds 28-end but is still here (the send-off's move did not
// happen) is sent on by any talk or walk.
func TestLandingLeavesTalkToDewey(t *testing.T) {
	room, u, _ := landingScene(t)

	try := func(cmd, rest string) bool {
		clearRoomStates(t, room.RoomId)
		return TryRoomBehavior(room.RoomId, EventContext{
			EventType: "room_command", UserId: 8141, RoomId: room.RoomId, Command: cmd, Rest: rest,
		})
	}

	u.Character.QuestProgress = map[int]string{28: "fled"}
	for _, c := range []struct{ cmd, rest string }{
		{"talk", "dewey"}, {"talk", "to dewey"}, {"say", "hello"}, {"ask", "dewey quest"},
	} {
		require.False(t, try(c.cmd, c.rest), "%s %q must reach the command, which opens Dewey's talk menu", c.cmd, c.rest)
	}
	require.True(t, try("north", ""), "a walk is refused while quest 28 is open")

	u.Character.QuestProgress = map[int]string{28: "end"}
	require.True(t, try("talk", "dewey"), "a player who finished but is still here is sent on")
	DrainAllDelayedActionsForTest(t)
}

// Only a flee reaches The Landing, so a player who holds quest 28 here has
// done the flee lesson whichever lesson they ran from. A resume from the shelf
// brings the old step back with no flee to raise, so what the player types
// next counts it. None of this intercepts talk: the menu must open, and show
// the hand-in.
func TestLandingCountsTheFleeForAHeldQuest(t *testing.T) {
	room, u, _ := landingScene(t)

	command := func(cmd string) bool {
		return TryRoomBehavior(room.RoomId, EventContext{
			EventType: "room_command", UserId: 8141, RoomId: room.RoomId, Command: cmd,
		})
	}

	t.Run("arriving", func(t *testing.T) {
		for _, step := range []string{"start", "attacked", "tripped"} {
			u.Character.QuestProgress = map[int]string{28: step}
			TryRoomBehavior(room.RoomId, EventContext{EventType: "room_enter", UserId: 8141, RoomId: room.RoomId})
			require.Equal(t, "fled", u.Character.QuestProgress[28], "arriving at %s", step)
		}
	})

	for _, c := range []struct {
		cmd         string
		intercepted bool
	}{
		{"talk", false}, {"ask", false}, {"say", false}, {"hint", false}, {"quests", false},
		{"north", true}, {"go", true}, {"down", true},
	} {
		t.Run(c.cmd, func(t *testing.T) {
			u.Character.QuestProgress = map[int]string{28: "attacked"}
			require.Equal(t, c.intercepted, command(c.cmd))
			require.Equal(t, "fled", u.Character.QuestProgress[28])
		})
	}

	t.Run("a command the lesson does not read", func(t *testing.T) {
		u.Character.QuestProgress = map[int]string{28: "attacked"}
		require.False(t, command("inventory"))
		require.Equal(t, "attacked", u.Character.QuestProgress[28])
	})

	t.Run("the abandoned hold nothing to count", func(t *testing.T) {
		u.Character.QuestProgress = map[int]string{}
		u.Character.ShelvedQuests = map[int]string{28: "attacked"}
		t.Cleanup(func() { u.Character.ShelvedQuests = nil })
		TryRoomBehavior(room.RoomId, EventContext{EventType: "room_enter", UserId: 8141, RoomId: room.RoomId})
		require.False(t, command("talk"))
		require.True(t, command("north"))
		require.Empty(t, u.Character.QuestProgress)
	})

	t.Run("a finished quest is not moved back", func(t *testing.T) {
		u.Character.QuestProgress = map[int]string{28: "end"}
		TryRoomBehavior(room.RoomId, EventContext{EventType: "room_enter", UserId: 8141, RoomId: room.RoomId})
		require.Equal(t, "end", u.Character.QuestProgress[28])
	})
}

// Each line at The Landing names the mark that is really on the player's
// screen: a player who holds the quest finishes it at [?], and one who set it
// aside takes it up again at [!].
func TestLandingLinesNameTheMarkOnScreen(t *testing.T) {
	cases := []struct {
		name      string
		progress  map[int]string
		mark, not string
	}{
		{"held at the flee step", map[int]string{28: "fled"}, "[?]", "[!]"},
		{"resumed on an early step", map[int]string{28: "attacked"}, "[?]", "[!]"},
		{"abandoned", map[int]string{}, "[!]", "[?]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			room, u, said := landingScene(t)
			set := func() {
				*said = (*said)[:0]
				u.Character.QuestProgress = map[int]string{}
				for k, v := range c.progress {
					u.Character.QuestProgress[k] = v
				}
			}
			check := func(when string) {
				t.Helper()
				DrainAllDelayedActionsForTest(t)
				joined := strings.Join(*said, " ")
				require.Contains(t, joined, c.mark, "%s: %q", when, joined)
				require.NotContains(t, joined, c.not, "%s: %q", when, joined)
			}

			set()
			TryRoomBehavior(room.RoomId, EventContext{EventType: "room_enter", UserId: 8141, RoomId: room.RoomId})
			check("arriving")

			set()
			require.True(t, TryRoomBehavior(room.RoomId, EventContext{
				EventType: "room_command", UserId: 8141, RoomId: room.RoomId, Command: "north",
			}))
			check("walking")

			// room_idle has no player. The counter speaks on the seventh round.
			set()
			for i := 0; i < 8; i++ {
				TryRoomBehavior(room.RoomId, EventContext{EventType: "room_idle", RoomId: room.RoomId})
			}
			check("re-prompting")
		})
	}
}

// Dewey's send-off (#375 phase 7): choosing the `ready` hand-in fires his
// tree, which moves the player to the Awakening Pool (5200). The hand-in
// grants 28-end itself, so the tree grants nothing. A second choice in the
// moment before 28-end lands does not replay it, and a player who left
// before the move is not moved.
func TestDeweySendOffMovesThePlayerToThePool(t *testing.T) {
	const (
		landing   = 6467
		pool      = 5200
		elsewhere = 6262
		deweyInst = 94910
	)
	run := func(t *testing.T, leave bool) int {
		DrainAllDelayedActionsForTest(t)
		events.DrainQueuedQuestsForTest()
		t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
			landing:   {RoomId: landing, Zone: "TestZone", Title: "The Landing", Exits: map[string]exit.RoomExit{}},
			pool:      {RoomId: pool, Zone: "TestZone", Title: "The Awakening Pool", Exits: map[string]exit.RoomExit{}},
			elsewhere: {RoomId: elsewhere, Zone: "TestZone", Title: "The Proving", Exits: map[string]exit.RoomExit{}},
		}, map[string]*rooms.ZoneConfig{}))
		t.Cleanup(seedTestMob(t, 9491, deweyInst, landing, "Dewey"))
		t.Cleanup(seedTestUser(t, 1, "verenna", "Verenna", landing))
		u := requireUser(t, 1)

		tree, err := LoadTreeFromFile("../../_datafiles/world/dogmud/behaviors/newcomer_antechamber/9491-dewey.yaml")
		require.NoError(t, err)
		state := NewBehaviorState()
		ctx := func() *EvalContext {
			return &EvalContext{
				Event:    EventContext{EventType: "player_topic", UserId: 1, Text: "ready", RoomId: landing},
				MobState: state, MobId: 9491, InstanceId: deweyInst, RoomId: landing, MobName: "Dewey",
			}
		}
		require.Equal(t, Success, tree.Evaluate(ctx()), "the ready topic must reach the send-off branch")
		require.NotEqual(t, Success, tree.Evaluate(ctx()), "a second choice must not replay the send-off")
		if leave {
			u.Character.RoomId = elsewhere
		}
		DrainAllDelayedActionsForTest(t)
		require.Empty(t, questGrantsFor(1), "the hand-in grants 28-end; the tree grants nothing")
		return u.Character.RoomId
	}

	t.Run("stayed", func(t *testing.T) {
		require.Equal(t, pool, run(t, false), "the player is moved to the Awakening Pool")
	})
	t.Run("left", func(t *testing.T) {
		require.Equal(t, elsewhere, run(t, true), "a player who left is not moved")
	})
}
