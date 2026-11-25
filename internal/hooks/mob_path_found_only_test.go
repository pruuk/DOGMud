package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

type waypointTestStep struct {
	exit string
	room int
}

func (s waypointTestStep) ExitName() string { return s.exit }
func (s waypointTestStep) RoomId() int      { return s.room }
func (s waypointTestStep) Waypoint() bool   { return true }

// displacedMob is a mob in room 7121 whose path's current step is room 7122,
// a waypoint: it has been moved off its path, so advanceMobPath re-paths.
func displacedMob(t *testing.T, foundOnly bool) *mobs.Mob {
	t.Helper()
	const inst = 7160
	m := &mobs.Mob{InstanceId: inst}
	m.Character = *characters.New()
	m.Character.RoomId = 7121
	m.Path.SetPath([]mobs.PathRoom{waypointTestStep{"north", 7122}}, foundOnly)
	m.Path.Next()
	events.DrainQueuedInputsForTest(inst)
	t.Cleanup(func() { events.DrainQueuedInputsForTest(inst) })
	return m
}

// #781, owner ruling 13: a planner's errand (`pathto found`) that is knocked
// off its path re-paths as an errand; a scripted route re-paths as written.
func TestAdvanceMobPath_RepathKeepsFoundOnly(t *testing.T) {
	m := displacedMob(t, true)
	require.True(t, advanceMobPath(m))
	require.Equal(t, []string{"pathto " + mobs.PathtoFoundOnly + " 7122"}, events.DrainQueuedInputsForTest(m.InstanceId))

	m = displacedMob(t, false)
	require.True(t, advanceMobPath(m))
	require.Equal(t, []string{"pathto 7122"}, events.DrainQueuedInputsForTest(m.InstanceId))
}
