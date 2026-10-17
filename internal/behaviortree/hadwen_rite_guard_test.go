package behaviortree

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Cleric Hadwen's Awakening Rite (mob 9100) is a paced sequence of delayed
// actions about 37 seconds long, and 30-end is granted only at its close. The
// 2026-10-17 audit (S6) found that `ask hadwen begin` asked again mid-rite
// passed every gate (the asker still lacks 30-end) and queued a second full
// rite, so the asker heard it twice and was granted twice. The tree now holds
// a `rite_running` flag for the length of the rite, the way The Landing
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
	before := hadwenQueueLen()
	GetEngine().EvaluateEvent(9100, 91000, EventContext{
		EventType: "player_ask", UserId: 8142, Text: text, RoomId: 5200,
	}, state)
	return hadwenQueueLen() - before
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
	require.Equal(t, "", state.GetString("rite_running"), "the rite-running flag must clear when the rite ends")
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
	require.Equal(t, "", state.GetString("rite_running"), "an interrupted rite must not leave the flag stuck")

	// The next newcomer starts the rite as normal.
	u.Character.QuestProgress = map[int]string{30: "start"}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8142: u}))
	require.GreaterOrEqual(t, askHadwen(state, "begin"), 10, "after the flag clears the rite must start again")
	DrainAllDelayedActionsForTest(t)
}
