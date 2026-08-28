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

// U10b-2: the two hidden-detection sites fire through the progression seam, on
// BOTH outcomes, instead of calling OnSkillUse only when the observer won.
//
// U10b-1b gave this contest a real opposed roll against the hider's sneak score
// but left its FIRING alone, so the award stayed inside the `if success` branch
// -- the exact win-only shape the firing convention exists to remove. The seam
// guard carried a row exempting this file, printed with a reason that had
// stopped being true. That row is gone and must not come back.
//
// Both assertions are proven to flip: against the pre-change file the first
// counts 2 and the second counts 0.
func TestGo_HiddenDetectionFiresThroughTheSeamOnBothOutcomes(t *testing.T) {
	src := goSource(t)

	require.NotContains(t, src, "OnSkillUse(string(skills.Search)",
		"go.go calls the OnSkillUse primitive directly again; hidden detection must fire through AwardResolved (the seam guard no longer exempts this file)")

	// Pinned on `success` as the won argument, not merely on AwardResolved
	// being present: moving the award back inside the `if success` block would
	// pass a literal true, which is the win-only regression this guards.
	// go.go has a THIRD AwardResolved call (the movement-trains-search one,
	// which correctly passes true because walking is not a contest), so
	// counting bare "AwardResolved(" would not distinguish them.
	require.Equal(t, 2, strings.Count(src, "AwardResolved(user.UserId, success,"),
		"expected both hidden-detection sites (players and mobs) to pass the contest result as the won argument; a literal true here means the award moved back inside the success branch")
}
