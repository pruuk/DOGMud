package behaviortree

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 (Pothole Coulee pass, owner ruling 2026-10-30): once a player
// holds all eight "Report to Toke" bridges (90-end to 97-end), Crier Toke and
// Warden Esk greet them on arrival, one time in four, with a line about one
// teacher's repeatable work. The line goes to that player alone (respond
// with user_text only), and a player missing any bridge hears nothing.

type flavourCall struct{ user, room string }

func recordFlavour(t *testing.T) *[]flavourCall {
	t.Helper()
	var calls []flavourCall
	orig := actionRegistry["respond"]
	actionRegistry["respond"] = func(params map[string]any, ctx *EvalContext) Result {
		calls = append(calls, flavourCall{getStringParam(params, "user_text"), getStringParam(params, "room_text")})
		return Success
	}
	t.Cleanup(func() { actionRegistry["respond"] = orig })
	return &calls
}

func loadCouleeTree(t *testing.T, mobId int, file string) {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "behaviors", "pothole_coulee", file)
	require.NoError(t, GetEngine().LoadTree(mobId, path))
	t.Cleanup(func() { GetEngine().EvictTree(mobId) })
}

// bridgeUser builds a user holding the bridges 90 to lastBridge. SeedUsersForTest
// replaces the whole user table, so a test seeds all its users in one call.
func bridgeUser(userId int, lastBridge int) *users.UserRecord {
	u := users.NewTestUser(userId, "bridge", "Bridge", 0)
	u.Character.QuestProgress = map[int]string{}
	for q := 90; q <= lastBridge; q++ {
		u.Character.QuestProgress[q] = "end"
	}
	return u
}

func TestCouleeFlavourTrees_SpeakOnlyToAPlayerWithEveryBridge(t *testing.T) {
	for _, tc := range []struct {
		mobId  int
		file   string
		givers []string
	}{
		{9106, "9106-crier_toke.yaml", []string{"Garve", "Falv", "Orrin", "Dace"}},
		{9107, "9107-warden_esk.yaml", []string{"Ovell", "Delk", "Sere", "Bryn"}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			calls := recordFlavour(t) // before the load, so the tree's nodes call it
			loadCouleeTree(t, tc.mobId, tc.file)
			DrainAllDelayedActionsForTest(t)
			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
				8601: bridgeUser(8601, 97), // all eight
				8602: bridgeUser(8602, 96), // missing 97-end
			}))
			state := NewBehaviorState()
			enter := func(userId int) {
				GetEngine().EvaluateEvent(tc.mobId, tc.mobId, EventContext{EventType: "player_enter", UserId: userId}, state)
			}

			for i := 0; i < 400; i++ {
				enter(8602)
			}
			DrainAllDelayedActionsForTest(t)
			require.Empty(t, *calls, "a player missing a bridge hears nothing")

			for i := 0; i < 400; i++ {
				enter(8601)
			}
			DrainAllDelayedActionsForTest(t)
			require.NotEmpty(t, *calls, "one arrival in four speaks")
			require.Less(t, len(*calls), 400, "not every arrival speaks")

			heard := map[string]bool{}
			for _, c := range *calls {
				require.Empty(t, c.room, "the line goes to the player alone")
				named := ""
				for _, g := range tc.givers {
					if strings.Contains(c.user, g) {
						named = g
					}
				}
				require.NotEmpty(t, named, "each line names a teacher: %q", c.user)
				heard[named] = true
			}
			require.Len(t, heard, len(tc.givers), "every line can be picked")
		})
	}
}
