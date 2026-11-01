package behaviortree

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
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
	held, _ := walk(map[int]string{30: "end"})
	require.False(t, held, "an Opened player walks on")
}
