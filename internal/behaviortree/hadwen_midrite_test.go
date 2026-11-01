package behaviortree

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// seedQuest30 gives the test binary quest 30's steps, so HasQuest("30-start")
// holds at 30-rite as it does in the game.
func seedQuest30(t *testing.T) {
	t.Helper()
	t.Cleanup(quests.SeedQuestsForTest(&quests.Quest{
		QuestId: 30, Name: "The Awakening",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "rite"}, {Id: "woken"}, {Id: "end"}},
	}))
}

// The rite marks itself on the player as it starts: quest step 30-rite, so
// Hadwen's dialogue (which cannot read the tree's per-player rite_running
// state) shows a mid-rite opener and the mid-rite topic rite_wait instead of
// the pre-rite opener and the begin line (playtest seat D, #375).
func TestHadwenRiteGrantsTheRiteStepAtOnce(t *testing.T) {
	seedQuest30(t)
	granted := map[int][]string{}
	origGrant := actionRegistry["grant_quest"]
	actionRegistry["grant_quest"] = func(params map[string]any, ctx *EvalContext) Result {
		granted[ctx.Event.UserId] = append(granted[ctx.Event.UserId], getStringParam(params, "quest"))
		return origGrant(params, ctx)
	}
	t.Cleanup(func() { actionRegistry["grant_quest"] = origGrant })
	loadHadwenTree(t)
	u, restore := seedRiteUser(t)
	t.Cleanup(restore)
	DrainAllDelayedActionsForTest(t)
	state := NewBehaviorState()

	require.GreaterOrEqual(t, askHadwen(state, "begin"), 10)
	require.Equal(t, "rite", u.Character.QuestProgress[30], "the rite step must land as the rite starts, before any delayed line")
	require.Equal(t, []string{"30-rite"}, granted[8142])

	// Mid-rite, the mid-rite topic gets only the hush line.
	require.LessOrEqual(t, askHadwen(state, "rite_wait"), 1, "rite_wait replayed the rite while it was running")

	DrainAllDelayedActionsForTest(t)
	require.Equal(t, []string{"30-rite", "30-woken"}, granted[8142])
}

// A rite cut short (the player logged out, or the server restarted: the tree
// state is not saved) leaves the player at 30-rite with the rite not
// running. The mid-rite topic then starts the rite again, so the player is
// never stuck without the begin line.
func TestHadwenRiteWaitRestartsACutShortRite(t *testing.T) {
	seedQuest30(t)
	said := []string{}
	origSay := actionRegistry["say"]
	actionRegistry["say"] = func(params map[string]any, ctx *EvalContext) Result {
		said = append(said, getStringParam(params, "text"))
		return Success
	}
	t.Cleanup(func() { actionRegistry["say"] = origSay })
	loadHadwenTree(t)
	u, restore := seedRiteUser(t)
	t.Cleanup(restore)
	u.Character.QuestProgress = map[int]string{30: "rite"}
	DrainAllDelayedActionsForTest(t)
	state := NewBehaviorState() // rite_running is empty: the rite is not running

	require.GreaterOrEqual(t, askHadwen(state, "rite_wait"), 10, "rite_wait on a cut-short rite must run the rite again")
	DrainAllDelayedActionsForTest(t)
	require.NotEmpty(t, said)
	require.True(t, strings.Contains(said[0], "went quiet"), "the restart must say why it begins again: %q", said[0])
}

// The pool's movement gate (room 5200) holds every player who is not yet
// Opened and names the next step for where they are (#375 phase 7): the
// offer before the quest, the begin line once it is taken, patience while
// the rite runs, and the hand-in once it is over. An Opened player walks.
func TestPoolGateNamesTheNextStep(t *testing.T) {
	seedQuest30(t)
	said := []string{}
	orig := actionRegistry["send_user_text"]
	actionRegistry["send_user_text"] = func(params map[string]any, ctx *EvalContext) Result {
		said = append(said, getStringParam(params, "text"))
		return Success
	}
	t.Cleanup(func() { actionRegistry["send_user_text"] = orig })

	_, room := sightScene(t, "city")
	u := users.NewTestUser(8146, "brann", "Brann", 98146)
	u.Character.RoomId = room.RoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8146: u}))
	room.AddPlayer(8146)
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	tree := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "behaviors", "rooms", "pothole_coulee", "5200.yaml")
	require.NoError(t, GetEngine().LoadRoomTree(room.RoomId, tree))
	t.Cleanup(func() { clearEngineRoomState(t, room.RoomId); clearRoomStates(t, room.RoomId) })

	walk := func(progress map[int]string) (bool, string) {
		said = said[:0]
		u.Character.QuestProgress = progress
		held := TryRoomBehavior(room.RoomId, EventContext{
			EventType: "room_command", UserId: 8146, RoomId: room.RoomId, Command: "south",
		})
		return held, strings.Join(said, " ")
	}
	for _, c := range []struct {
		name     string
		progress map[int]string
		want     string
	}{
		{"no quest", map[int]string{}, "[!]"},
		{"taken", map[int]string{30: "start"}, "begin the rite"},
		{"mid-rite", map[int]string{30: "rite"}, "under way"},
		{"woken", map[int]string{30: "woken"}, "tell him the rite is done"},
	} {
		held, text := walk(c.progress)
		require.True(t, held, "%s: the pool must hold the player", c.name)
		require.Contains(t, text, c.want, "%s", c.name)
	}
	// rite_running is not saved, so after a restart a player at 30-rite must
	// ask again. The gate has to say how.
	_, text := walk(map[int]string{30: "rite"})
	require.Contains(t, text, ">talk hadwen<", "a rite cut short must name the command")
	require.Contains(t, text, hadwenTopicLabel(t, "rite_wait"), "and the line to choose")

	held, _ := walk(map[int]string{30: "end"})
	require.False(t, held, "an Opened player walks on")
}

// hadwenMenu is the talk menu Hadwen's dialogue file gives a player at this
// progress, read from the shipped file, as topic key to label.
func hadwenMenu(t *testing.T, u *users.UserRecord) map[string]string {
	t.Helper()
	df := hadwenDialogue(t)
	menu := map[string]string{}
	for _, e := range df.Menu("Cleric Hadwen", &dialogue.PlayerState{
		QuestOpen:    func(int) bool { return true },
		HasQuest:     u.Character.HasQuest,
		HasItem:      func(int) bool { return false },
		CarriesItem:  func(int) bool { return false },
		GetQuestFlag: func(string) string { return "" },
		HasHeard:     func(string) bool { return false },
		MarkHeard:    func(string) {},
	}).Entries {
		menu[e.Id] = e.Label
	}
	return menu
}

// hadwenDialogue is Hadwen's shipped dialogue file.
func hadwenDialogue(t *testing.T) *dialogue.DialogueFile {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", "9100.yaml"))
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml.Unmarshal(data, &df))
	return &df
}

// hadwenTopicLabel is the label of one topic in Hadwen's dialogue file.
func hadwenTopicLabel(t *testing.T, key string) string {
	t.Helper()
	topic := hadwenDialogue(t).FindTopic(key)
	require.NotNil(t, topic, "topic %s", key)
	return topic.Label
}

// hadwenSpeechScene loads Hadwen's tree into a real room, with `say` and the
// ambient dice under the test's control. chances answers each random_chance in
// turn; past its end, the dice pass.
func hadwenSpeechScene(t *testing.T, chances ...bool) (*rooms.Room, *[]string) {
	t.Helper()
	seedQuest30(t)
	said := &[]string{}
	origSay := actionRegistry["say"]
	actionRegistry["say"] = func(params map[string]any, ctx *EvalContext) Result {
		*said = append(*said, getStringParam(params, "text"))
		return Success
	}
	t.Cleanup(func() { actionRegistry["say"] = origSay })
	origChance := conditionRegistry["random_chance"]
	conditionRegistry["random_chance"] = func(params map[string]any, ctx *EvalContext) Result {
		if len(chances) == 0 {
			return Success
		}
		next := chances[0]
		chances = chances[1:]
		if next {
			return Success
		}
		return Failure
	}
	t.Cleanup(func() { conditionRegistry["random_chance"] = origChance })
	_, room := sightScene(t, "city")
	loadHadwenTree(t)
	DrainAllDelayedActionsForTest(t)
	return room, said
}

// hadwenPoolStates is every state Hadwen's greeting and ambient line speak to,
// with the talk topic the line must name and any label it must not.
var hadwenPoolStates = []struct {
	name     string
	progress map[int]string
	topic    string
	notOn    string
}{
	{"no quest", map[int]string{28: "end"}, "rite_offer", ""},
	{"taken", map[int]string{30: "start"}, "begin", "The Awakening"},
	{"mid-rite", map[int]string{30: "rite"}, "rite_wait", "The Awakening"},
}

// A player at 30-start or 30-rite has no "The Awakening" line on the menu
// (rite_offer excludes 30-start, the hand-in needs 30-woken), so a greeting
// that sends them to it names something they cannot find.
func TestHadwenGreetingNamesALineOnThePlayersMenu(t *testing.T) {
	for _, c := range hadwenPoolStates {
		t.Run(c.name, func(t *testing.T) {
			room, said := hadwenSpeechScene(t)
			u := users.NewTestUser(8147, "wren", "Wren", 98147)
			u.Character.RoomId = room.RoomId
			u.Character.QuestProgress = c.progress
			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8147: u}))
			room.AddPlayer(8147)

			GetEngine().EvaluateEvent(9100, 91000, EventContext{
				EventType: "player_enter", UserId: 8147, RoomId: room.RoomId,
			}, NewBehaviorState())
			DrainAllDelayedActionsForTest(t)

			require.Len(t, *said, 1, "one greeting")
			line := (*said)[0]
			label, onMenu := hadwenMenu(t, u)[c.topic]
			require.True(t, onMenu, "%s is not on this player's menu", c.topic)
			require.Contains(t, line, label)
			require.Contains(t, line, ">talk hadwen<")
			if c.notOn != "" {
				require.NotContains(t, line, c.notOn)
			}
		})
	}

	// The states past the rite get no greeting: the rite's close and the gate
	// name the hand-in, and an Opened player has nothing to be told.
	for _, step := range []string{"woken", "end"} {
		t.Run(step, func(t *testing.T) {
			room, said := hadwenSpeechScene(t)
			u := users.NewTestUser(8147, "wren", "Wren", 98147)
			u.Character.RoomId = room.RoomId
			u.Character.QuestProgress = map[int]string{30: step}
			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8147: u}))
			room.AddPlayer(8147)
			GetEngine().EvaluateEvent(9100, 91000, EventContext{
				EventType: "player_enter", UserId: 8147, RoomId: room.RoomId,
			}, NewBehaviorState())
			DrainAllDelayedActionsForTest(t)
			require.Empty(t, *said)
		})
	}
}

// The ambient line has no triggering player, and the whole room hears it, so
// each state has its own line that says who it is for.
func TestHadwenAmbientNamesALineOnThePlayersMenu(t *testing.T) {
	for _, c := range hadwenPoolStates {
		t.Run(c.name, func(t *testing.T) {
			room, said := hadwenSpeechScene(t)
			u := users.NewTestUser(8147, "wren", "Wren", 98147)
			u.Character.RoomId = room.RoomId
			u.Character.QuestProgress = c.progress
			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8147: u}))
			room.AddPlayer(8147)

			GetEngine().EvaluateEvent(9100, 91000, EventContext{
				EventType: "mob_idle", RoomId: room.RoomId,
			}, NewBehaviorState())

			require.Len(t, *said, 1, "one ambient line")
			label, onMenu := hadwenMenu(t, u)[c.topic]
			require.True(t, onMenu, "%s is not on this player's menu", c.topic)
			require.Contains(t, (*said)[0], label)
			if c.notOn != "" {
				require.NotContains(t, (*said)[0], c.notOn)
			}
		})
	}
}

// Two players at the pool: A mid-rite, B with no quest. A holds 30-start and B
// lacks 30-rite, so two separate has/missing conditions would both pass and
// Hadwen would tell the room about a begin line neither player has. And when
// the first branch's dice fail, the next state present still gets its turn, so
// one player waiting at the pool cannot silence the line another needs.
func TestHadwenAmbientReadsOnePlayerAtATime(t *testing.T) {
	room, said := hadwenSpeechScene(t, false)
	a := users.NewTestUser(8147, "wren", "Wren", 98147)
	a.Character.RoomId = room.RoomId
	a.Character.QuestProgress = map[int]string{30: "rite"}
	b := users.NewTestUser(8148, "tam", "Tam", 98148)
	b.Character.RoomId = room.RoomId
	b.Character.QuestProgress = map[int]string{}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8147: a, 8148: b}))
	room.AddPlayer(8147)
	room.AddPlayer(8148)

	GetEngine().EvaluateEvent(9100, 91000, EventContext{
		EventType: "mob_idle", RoomId: room.RoomId,
	}, NewBehaviorState())

	require.Len(t, *said, 1, "the no-quest dice failed, so the mid-rite line speaks")
	require.Contains(t, (*said)[0], hadwenTopicLabel(t, "rite_wait"))
	require.NotContains(t, (*said)[0], hadwenTopicLabel(t, "begin"))
}

// player_in_room_has_quest with `missing` asks about one player: holding the
// token and lacking the other. Without it, the old any-player reading stands.
func TestPlayerInRoomHasQuest_MissingIsTheSamePlayer(t *testing.T) {
	seedQuest30(t)
	_, room := sightScene(t, "city")
	a := users.NewTestUser(8147, "wren", "Wren", 98147)
	a.Character.QuestProgress = map[int]string{30: "rite"}
	b := users.NewTestUser(8148, "tam", "Tam", 98148)
	b.Character.QuestProgress = map[int]string{}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8147: a, 8148: b}))
	room.AddPlayer(8147)
	room.AddPlayer(8148)
	ctx := &EvalContext{RoomId: room.RoomId}

	both := map[string]any{"quest": "30-start", "missing": "30-rite"}
	require.Equal(t, Failure, condPlayerInRoomHasQuest(both, ctx), "no one player holds 30-start and lacks 30-rite")
	require.Equal(t, Success, condPlayerInRoomHasQuest(map[string]any{"quest": "30-start"}, ctx), "without missing, any holder passes")

	b.Character.QuestProgress = map[int]string{30: "start"}
	require.Equal(t, Success, condPlayerInRoomHasQuest(both, ctx), "Tam is at 30-start")
}
