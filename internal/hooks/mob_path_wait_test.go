package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

type walkerTestStep struct {
	exit string
	room int
}

func (s walkerTestStep) ExitName() string { return s.exit }
func (s walkerTestStep) RoomId() int      { return s.room }
func (s walkerTestStep) Waypoint() bool   { return false }

const walkerCliff = "walkertestcliff"

// walkerWorld: room 7101 with a north exit to 7102, a 2.5 cliff.
func walkerWorld(t *testing.T, stamina, staminaMax int) *mobs.Mob {
	t.Helper()
	// QuoteMobStep resolves the exit through FindExitByName, which reads the
	// direction aliases; a test binary has none loaded.
	t.Cleanup(keywords.SeedKeywordsForTest())
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		walkerCliff: {BiomeId: walkerCliff, MovementCost: 2.5},
	}))
	from := &rooms.Room{RoomId: 7101, Exits: map[string]exit.RoomExit{"north": {RoomId: 7102}}}
	to := &rooms.Room{RoomId: 7102, Biome: walkerCliff}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{7101: from, 7102: to}, map[string]*rooms.ZoneConfig{}))

	m := &mobs.Mob{InstanceId: 7150}
	m.Character = *characters.New()
	m.Character.RoomId = 7101
	m.Character.StaminaMax.Value = staminaMax
	m.Character.Stamina = stamina
	m.Path.SetPath([]mobs.PathRoom{walkerTestStep{"north", 7102}}, false)
	events.DrainQueuedInputsForTest(7150)
	t.Cleanup(func() { events.DrainQueuedInputsForTest(7150) })
	return m
}

// A tired mob keeps its path and issues nothing; it tries again next round.
func TestAdvanceMobPath_TiredMobWaits(t *testing.T) {
	m := walkerWorld(t, 0, 100)
	require.True(t, advanceMobPath(m), "a waiting mob is busy with its path")
	require.Equal(t, 1, m.Path.Len(), "the step stays queued")
	require.Nil(t, m.Path.Current(), "Next was not called")
	require.Empty(t, events.InspectQueuedInputForTest(7150, "north"))
	require.True(t, mobPathStepWaiting(m))
}

// A rested mob takes the step.
func TestAdvanceMobPath_RestedMobSteps(t *testing.T) {
	m := walkerWorld(t, 100, 100)
	require.True(t, advanceMobPath(m))
	require.Equal(t, 0, m.Path.Len())
	require.NotEmpty(t, events.InspectQueuedInputForTest(7150, "north"))
	require.False(t, mobPathStepWaiting(m))
}

// A step the mob could never pay clears the path, handing the mob to the
// schedule and patrol fallbacks instead of parking it forever.
func TestAdvanceMobPath_NeverAffordableClears(t *testing.T) {
	m := walkerWorld(t, 0, 1) // empty, and a pool of 1 can never cover a 1.37 cliff step
	require.False(t, advanceMobPath(m), "a cleared path leaves the mob idle")
	require.Equal(t, 0, m.Path.Len())
	require.Empty(t, events.InspectQueuedInputForTest(7150, "north"))
}

// A patrol mob resting mid-path does not count a failed path attempt, so a
// long rest never trips the home fallback.
func TestApplyPatrolPlan_WaitingIsNotFailing(t *testing.T) {
	registerTestPatrol(t)
	m := walkerWorld(t, 0, 100)
	plan := patrolTickPlan(m, "test_patrol")
	require.True(t, plan.WantsPath, "fixture premise: 7101 is not a waypoint")
	applyPatrolPlan(m, plan, "test_patrol")
	if got := m.Character.GetMiscData("patrol_path_fail_count"); got != nil {
		require.Equal(t, 0, got.(int))
	}
}

// Real IdleMobs ticks: a tired patrol mob mid-path neither spins (no re-path,
// no step) nor counts toward its home fallback, however long it rests. Once
// rested it takes the same step it was waiting on.
func TestIdleMobs_TiredPatrolMobWaitsWithoutSpinning(t *testing.T) {
	registerTestPatrol(t)
	m := walkerWorld(t, 0, 100)
	m.PatrolId = "test_patrol"
	mobs.SetInstanceForTest(7150, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(7150, nil) })

	// More ticks than the default retry cap, so a counted rest would trip it.
	for i := range 30 {
		IdleMobs(events.NewRound{RoundNumber: uint64(i + 1)})
	}
	require.Empty(t, events.DrainQueuedInputsForTest(7150), "a tired mob queues no step, pathto or home")
	require.Equal(t, 1, m.Path.Len(), "the path is intact")
	require.Nil(t, m.Path.Current())
	require.Equal(t, 0, getMiscDataInt(&m.Character, "patrol_path_fail_count"))

	m.Character.Stamina = 100
	IdleMobs(events.NewRound{RoundNumber: 31})
	require.Equal(t, []string{"north"}, events.DrainQueuedInputsForTest(7150))
	require.Equal(t, 0, m.Path.Len())
}
