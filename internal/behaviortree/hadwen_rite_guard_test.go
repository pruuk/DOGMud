package behaviortree

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Cleric Hadwen's Awakening Rite (mob 9100) is a paced sequence of delayed
// actions about 37 seconds long, and 30-end is granted only at its close. The
// 2026-10-17 audit (S6) found that `ask hadwen begin` asked again mid-rite
// passed every gate (the asker still lacks 30-end) and queued a second full
// rite, so the asker heard it twice and was granted twice. The tree now holds
// a per-player `rite_running` flag (BehaviorState key "rite_running:<user
// id>", from `per_player: true`) for the length of the rite, the way The Landing
// (room 6467) holds `leaving` for its handoff.

func hadwenQueueLen() int {
	e := GetEngine()
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.queue)
}

func loadHadwenTree(t *testing.T) {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	tree := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "behaviors", "pothole_coulee", "9100-cleric_hadwen.yaml")
	require.NoError(t, GetEngine().LoadTree(9100, tree))
	t.Cleanup(func() { GetEngine().EvictTree(9100) })
}

func seedRiteUser(t *testing.T) (*users.UserRecord, func()) {
	t.Helper()
	u := users.NewTestUser(8142, "ilse", "Ilse", 98142)
	u.Character.RoomId = 5200
	u.Character.QuestProgress = map[int]string{30: "start"}
	restore := users.SeedUsersForTest(map[int]*users.UserRecord{8142: u})
	return u, restore
}

func askHadwen(state *BehaviorState, text string) int {
	return askHadwenAs(state, 8142, text)
}

func askHadwenAs(state *BehaviorState, userId int, text string) int {
	before := hadwenQueueLen()
	GetEngine().EvaluateEvent(9100, 91000, EventContext{
		EventType: "player_ask", UserId: userId, Text: text, RoomId: 5200,
	}, state)
	return hadwenQueueLen() - before
}

// Owner ruling 2026-10-17: a mutation can fire mid-tutorial, as long as
// Hadwen explains it. His mutation line reads one way to a player the
// chrysalis has already changed and another to one it has not, and both
// point at the mutations command.
func TestHadwenRiteExplainsMutationsEitherWay(t *testing.T) {
	said := map[int][]string{}
	origSay := actionRegistry["say"]
	actionRegistry["say"] = func(params map[string]any, ctx *EvalContext) Result {
		said[ctx.Event.UserId] = append(said[ctx.Event.UserId], getStringParam(params, "text"))
		return Success
	}
	t.Cleanup(func() { actionRegistry["say"] = origSay })
	loadHadwenTree(t)

	changed := users.NewTestUser(8144, "vey", "Vey", 98144)
	changed.Character.RoomId = 5200
	changed.Character.QuestProgress = map[int]string{30: "start"}
	changed.Character.Mutations = map[string]int{"hollow-bones": 1}
	fresh := users.NewTestUser(8145, "pell", "Pell", 98145)
	fresh.Character.RoomId = 5200
	fresh.Character.QuestProgress = map[int]string{30: "start"}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8144: changed, 8145: fresh}))
	DrainAllDelayedActionsForTest(t)
	state := NewBehaviorState()

	askHadwenAs(state, 8144, "begin")
	askHadwenAs(state, 8145, "begin")
	DrainAllDelayedActionsForTest(t)

	line := func(uid int) string {
		for _, s := range said[uid] {
			if strings.Contains(s, "mutations</ansi>") {
				return s
			}
		}
		return ""
	}
	require.Contains(t, line(8144), "The chrysalis has already changed you", "a player with a mutation hears that it has happened")
	require.Contains(t, line(8145), "In time the chrysalis will change you", "a player with none hears that it will")
	for _, uid := range []int{8144, 8145} {
		require.NotContains(t, line(uid), "A change may already have found you", "the vague line is retired")
	}
}

// The guard is per player (2026-10-17 newbie-duo playtest): two newcomers
// asking at once each get their own rite and both are Opened. The mob-wide
// flag it replaced answered the second asker "Hush, now" and left them at
// 30-start with no hint to ask again.
func TestHadwenRiteRunsForTwoPlayersAskingAtOnce(t *testing.T) {
	// Record every grant the rite makes. The test binary loads no quest
	// data, so the token itself cannot be read back off the character; the
	// recorder is installed before the tree loads, so the tree's grant nodes
	// call it.
	granted := map[int][]string{}
	origGrant := actionRegistry["grant_quest"]
	actionRegistry["grant_quest"] = func(params map[string]any, ctx *EvalContext) Result {
		granted[ctx.Event.UserId] = append(granted[ctx.Event.UserId], getStringParam(params, "quest"))
		return origGrant(params, ctx)
	}
	t.Cleanup(func() { actionRegistry["grant_quest"] = origGrant })
	loadHadwenTree(t)
	a := users.NewTestUser(8142, "ilse", "Ilse", 98142)
	a.Character.RoomId = 5200
	a.Character.QuestProgress = map[int]string{30: "start"}
	b := users.NewTestUser(8143, "odo", "Odo", 98143)
	b.Character.RoomId = 5200
	b.Character.QuestProgress = map[int]string{30: "start"}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8142: a, 8143: b}))
	DrainAllDelayedActionsForTest(t)
	state := NewBehaviorState()

	first := askHadwenAs(state, 8142, "begin")
	require.GreaterOrEqual(t, first, 10, "the first asker's rite must run")
	second := askHadwenAs(state, 8143, "begin")
	require.Equal(t, first, second,
		"a second player asking during the first player's rite queued %d actions, not their own rite of %d", second, first)

	// Each asker's own re-ask mid-rite still gets only the hush line.
	require.LessOrEqual(t, askHadwenAs(state, 8142, "begin"), 1, "the first asker's re-ask replayed the rite")
	require.LessOrEqual(t, askHadwenAs(state, 8143, "begin"), 1, "the second asker's re-ask replayed the rite")

	DrainAllDelayedActionsForTest(t)
	require.Equal(t, []string{"30-end", "29-start"}, granted[8142], "the first asker must be Opened exactly once")
	require.Equal(t, []string{"30-end", "29-start"}, granted[8143], "the second asker must be Opened exactly once")
}

func TestHadwenRiteDoesNotReplayMidRite(t *testing.T) {
	loadHadwenTree(t)
	_, restore := seedRiteUser(t)
	t.Cleanup(restore)
	DrainAllDelayedActionsForTest(t) // start from an empty queue
	state := NewBehaviorState()

	first := askHadwen(state, "begin")
	require.GreaterOrEqual(t, first, 10, "the first ask must queue the paced rite; the tree did not run it")

	second := askHadwen(state, "begin")
	require.LessOrEqual(t, second, 1,
		"asking again mid-rite queued %d actions (the first ask queued %d): the rite replays and grants twice", second, first)

	DrainAllDelayedActionsForTest(t)
	require.Equal(t, "", state.GetString("rite_running:8142"), "the rite-running flag must clear when the rite ends")
}

// The clear is queued with the rite, so it runs even when the asker is gone
// before the rite ends (logged out mid-rite): the flag never stays stuck and
// the next newcomer can begin.
func TestHadwenRiteFlagClearsWhenTheAskerLeaves(t *testing.T) {
	loadHadwenTree(t)
	u, restore := seedRiteUser(t)
	DrainAllDelayedActionsForTest(t)
	state := NewBehaviorState()

	require.GreaterOrEqual(t, askHadwen(state, "begin"), 10)
	restore() // the asker is no longer online
	DrainAllDelayedActionsForTest(t)
	require.Equal(t, "", state.GetString("rite_running:8142"), "an interrupted rite must not leave the flag stuck")

	// The next newcomer starts the rite as normal.
	u.Character.QuestProgress = map[int]string{30: "start"}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8142: u}))
	require.GreaterOrEqual(t, askHadwen(state, "begin"), 10, "after the flag clears the rite must start again")
	DrainAllDelayedActionsForTest(t)
}
