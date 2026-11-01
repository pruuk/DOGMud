package behaviortree

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Owner ruling 2026-10-30: a caught plant on the Pocket Dummy and a foiled
// opener on a mine creature earn one short good-natured line. Dace says the
// plant line when she is in the room, else the room carries it; the mine
// creatures carry the ambush line to the striker alone. A mine creature never
// uses {actor}: the striker may not be able to see it, and respond names the
// mob whatever the reader's sight.

func distinctCount(lines []string) int {
	seen := map[string]bool{}
	for _, l := range lines {
		seen[l] = true
	}
	return len(seen)
}

func TestPocketDummyMocksACaughtPlant(t *testing.T) {
	for _, tc := range []struct {
		name string
		dace bool
	}{{"dace on the jetty", true}, {"dace away", false}} {
		t.Run(tc.name, func(t *testing.T) {
			var said, narrated []string
			origSay, origRoom, origIn := actionRegistry["mob_say"], actionRegistry["send_room_text"], conditionRegistry["mob_in_room"]
			actionRegistry["mob_say"] = func(p map[string]any, ctx *EvalContext) Result {
				if getIntParam(p, "mob_id") != 9126 {
					return Failure
				}
				said = append(said, getStringParam(p, "text"))
				return Success
			}
			actionRegistry["send_room_text"] = func(p map[string]any, ctx *EvalContext) Result {
				narrated = append(narrated, getStringParam(p, "text"))
				return Success
			}
			conditionRegistry["mob_in_room"] = func(p map[string]any, ctx *EvalContext) Result {
				if tc.dace && getIntParam(p, "mob_id") == 9126 {
					return Success
				}
				return Failure
			}
			t.Cleanup(func() {
				actionRegistry["mob_say"], actionRegistry["send_room_text"], conditionRegistry["mob_in_room"] = origSay, origRoom, origIn
			})
			loadCouleeTree(t, 9127, "9127-pocket_dummy.yaml")
			DrainAllDelayedActionsForTest(t)
			state := NewBehaviorState()
			for i := 0; i < 60; i++ {
				GetEngine().EvaluateEvent(9127, 9127, EventContext{EventType: "caught_planting", UserId: 8603}, state)
			}
			DrainAllDelayedActionsForTest(t)

			lines := narrated
			if tc.dace {
				require.Empty(t, narrated, "Dace speaks; the room adds nothing")
				lines = said
			} else {
				require.Empty(t, said, "Dace is away and says nothing")
			}
			require.Len(t, lines, 60, "every caught plant earns a line")
			require.Equal(t, 3, distinctCount(lines), "all three lines can be picked")
			for _, l := range lines {
				require.LessOrEqual(t, utf8.RuneCountInString(l), 80, "line over 80 columns: %q", l)
			}
		})
	}
}

func TestMineCreaturesMockAFoiledAmbush(t *testing.T) {
	for _, tc := range []struct {
		mobId int
		file  string
	}{{9120, "9120-mine_crawler.yaml"}, {9121, "9121-tunnel_brute.yaml"}} {
		t.Run(tc.file, func(t *testing.T) {
			calls := recordFlavour(t)
			loadCouleeTree(t, tc.mobId, tc.file)
			DrainAllDelayedActionsForTest(t)
			state := NewBehaviorState()
			for i := 0; i < 60; i++ {
				GetEngine().EvaluateEvent(tc.mobId, tc.mobId, EventContext{EventType: "ambush_foiled", UserId: 8604}, state)
			}
			DrainAllDelayedActionsForTest(t)

			var lines []string
			for _, c := range *calls {
				require.Empty(t, c.room, "the striker alone hears it")
				lines = append(lines, c.user)
			}
			require.Len(t, lines, 60, "every foiled opener earns a line")
			require.Equal(t, 3, distinctCount(lines), "all three lines can be picked")
			for _, l := range lines {
				require.LessOrEqual(t, utf8.RuneCountInString(l), 80, "line over 80 columns: %q", l)
				require.False(t, strings.Contains(l, "{actor}"), "names a creature the striker may not see: %q", l)
			}
		})
	}
}

// A mob striker has no client: the real respond, the action the mine trees use,
// fails for user 0 and sends nothing, while a player striker is answered.
func TestRespond_AMobStrikerReadsNothing(t *testing.T) {
	const instanceId = 88120
	m := &mobs.Mob{InstanceId: instanceId, MobId: 9120}
	mobs.SetInstanceForTest(instanceId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instanceId, nil) })
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		8605: users.NewTestUser(8605, "striker", "Striker", 0),
	}))
	line := map[string]any{"user_text": "It felt you coming through the rock."}

	require.Equal(t, Failure, actRespond(line, &EvalContext{InstanceId: instanceId, Event: EventContext{EventType: "ambush_foiled"}}))
	require.Equal(t, Success, actRespond(line, &EvalContext{InstanceId: instanceId, Event: EventContext{EventType: "ambush_foiled", UserId: 8605}}))
}
