package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

func exhaustedStepWorld(t *testing.T, stamina int) *mobs.Mob {
	t.Helper()
	// QuoteMobStep resolves the exit through FindExitByName, which reads the
	// direction aliases; a test binary has none loaded (same gap Task 7's
	// walkerWorld hit).
	t.Cleanup(keywords.SeedKeywordsForTest())
	// Test binaries load the Go default base (0.5), which prices a step under
	// 1 stamina: it is carried as a fraction, so an empty pool still walks.
	// Make a step cost whole stamina so the exhausted mob really cannot pay.
	cfg := configs.GetConfig()
	cfg.Balance.MovementBaseStaminaCost = 5
	configs.SetConfigForTest(t, cfg)
	room1 := &rooms.Room{RoomId: 1, Zone: "TestZone"}
	room2 := &rooms.Room{RoomId: 2, Zone: "TestZone", Exits: map[string]exit.RoomExit{"south": {RoomId: 1}}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{1: room1, 2: room2}, nil))

	caller := &mobs.Mob{InstanceId: 100, Character: characters.Character{RoomId: 1, Health: 50}}
	self := &mobs.Mob{InstanceId: 91101}
	self.Character.Name = "stepper"
	self.Character.RoomId = 2
	self.Character.Stamina = stamina
	self.Character.StaminaMax.Value = 100
	self.Character.Conditions = conditions.New()
	t.Cleanup(mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{100: caller, 91101: self}))
	t.Cleanup(func() { events.DrainQueuedInputsForTest(91101) })
	return self
}

// A single-step node quotes first and FAILS when the mob cannot pay the
// stamina, so a selector falls through to its next child.
func TestSingleStepNodesFailWhenExhausted(t *testing.T) {
	cases := []struct {
		name string
		run  func(ctx *EvalContext) Result
	}{
		{"go_to_caller_room", func(ctx *EvalContext) Result { return actGoToCallerRoom(nil, ctx) }},
		{"move", func(ctx *EvalContext) Result {
			return actMove(map[string]any{"direction": "south"}, ctx)
		}},
		// Out of combat, flee walks away through the same quote.
		{"flee out of combat", func(ctx *EvalContext) Result { return actFlee(nil, ctx) }},
	}
	for _, tc := range cases {
		t.Run(tc.name+" exhausted", func(t *testing.T) {
			exhaustedStepWorld(t, 0)
			ctx := &EvalContext{InstanceId: 91101, Event: EventContext{MobId: 100}}
			require.Equal(t, Failure, tc.run(ctx))
			require.Empty(t, events.InspectQueuedInputForTest(91101, "go "))
		})
		t.Run(tc.name+" rested", func(t *testing.T) {
			exhaustedStepWorld(t, 100)
			ctx := &EvalContext{InstanceId: 91101, Event: EventContext{MobId: 100}}
			require.Equal(t, Success, tc.run(ctx))
			require.NotEmpty(t, events.InspectQueuedInputForTest(91101, "go "))
		})
	}
}
