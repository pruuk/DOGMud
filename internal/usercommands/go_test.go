package usercommands

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// goSource reads internal/usercommands/go.go, anchored on this file's own
// location rather than the working directory -- test binaries do not reliably
// start in the package dir.
func goSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller(0) failed")
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "go.go"))
	require.NoError(t, err)
	return string(src)
}

// U10b-1 Task 19: the stranded mob-follow roll is gone and must stay gone.
//
// It was a bare util.Rand(100) against 20 + Charisma + speedDelta, entirely off
// the contest core, deciding whether an engaged mob chased a leaving player.
// The arc's ruling is that mob pursuit is AUTHORED BEHAVIOUR, not a roll.
//
// Pinned on the roll's own distinctive parts rather than on "does go.go call
// util.Rand", because that file rolls for other things and always will.
func TestGo_TheMobFollowRollIsDeleted(t *testing.T) {
	src := goSource(t)

	for _, fragment := range []string{
		"Mob Follow",
		"rooms.FindFightingPlayer",
		"speedDelta",
	} {
		require.NotContains(t, src, fragment,
			"go.go still contains %q; the mob-follow roll was deleted by U10b-1 Task 19 and pursuit is authored behaviour now", fragment)
	}
}

// The DESTINATION room_enter behaviour survives the deletion.
//
// ⚠️ This is the assertion the plan warned about. go.go calls TryRoomBehavior
// TWICE -- once early on the departure room, once on the destination room right
// after the deleted loop -- so asserting merely that "TryRoomBehavior" appears
// would still pass if the destination call were deleted along with the roll it
// sat beside. The call is therefore pinned by its ARGUMENT.
func TestGo_TheDestinationRoomEnterBehaviourSurvives(t *testing.T) {
	src := goSource(t)

	require.Contains(t, src, "behaviortree.TryRoomBehavior(destRoom.RoomId",
		"the destination room_enter behaviour was deleted along with the mob-follow roll it sat beside")

	// And prove the weaker assertion really would not have caught it: the file
	// carries a second, unrelated TryRoomBehavior call.
	require.GreaterOrEqual(t, strings.Count(src, "behaviortree.TryRoomBehavior("), 2,
		"fixture premise: go.go must have more than one TryRoomBehavior call for the argument-pinning to matter")
}

// The `if !isSneaking` wrapper the roll lived inside is also kept: the ambush
// and player_enter blocks below still depend on it.
func TestGo_TheSneakingWrapperSurvives(t *testing.T) {
	require.Contains(t, goSource(t), "if !isSneaking {",
		"the !isSneaking wrapper was deleted with the mob-follow roll; the ambush and player_enter blocks need it")
}
