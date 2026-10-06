package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: shapes_roster_test.go
//
// Lighting plan 5c playtest finding 1. A heat-sight caster reading SightShapes
// in a pitch-dark cave typed `look` and read "Also here: Midroad Scout (100%)".
// The roster itself is pinned at its source, rooms.GetDetails
// (internal/rooms/roomdetails_sight_test.go): the test binary here does not
// load the world's templates, so a rendered roster cannot be read back in
// this package. What is pinned here is what these commands say themselves:
// `who` at none refuses, and `look <creature>` at shapes names nobody.

// seedShapesRosterRoom is seedDarknessGateRoom with Bobrick (user 2, who
// starts in room 1) moved into the viewer's room, so there is somebody to name.
func seedShapesRosterRoom(t *testing.T, lamp, condition int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	viewer, room := seedDarknessGateRoom(t, lamp)
	bob := users.GetByUserId(2)
	require.NotNil(t, bob)
	rooms.LoadRoom(1).RemovePlayer(bob.UserId)
	bob.Character.RoomId = room.RoomId
	room.AddPlayer(bob.UserId)
	if condition != 0 {
		require.True(t, viewer.Character.Conditions.AddCondition(condition, true))
	}
	return viewer, room
}

const shapesLookHint = "You can only make out shapes here."

func TestLookAtCreature_ShapesViewerIsNotToldWhoItIs(t *testing.T) {
	viewer, room := seedShapesRosterRoom(t, 10, gateInfraConditionId)
	bob := users.GetByUserId(2)
	drainBob := func() string { return runGate(t, bob, func() (bool, error) { return true, nil }) }
	drainBob()

	out := runGate(t, viewer, func() (bool, error) { return Look("bobrick", viewer, room, 0) })
	require.Contains(t, out, shapesLookHint)
	require.NotContains(t, out, "Bobrick")

	// A name that matches nobody gets the same answer, so the reply is not
	// an oracle for who is standing there.
	out = runGate(t, viewer, func() (bool, error) { return Look("zzyzx", viewer, room, 0) })
	require.Contains(t, out, shapesLookHint)

	// Bobrick is not told he is being looked at by name: he was never found.
	require.NotContains(t, drainBob(), "is looking at you")
}

func TestLookAtCreature_ClearViewerStillLooks(t *testing.T) {
	viewer, room := seedShapesRosterRoom(t, 60, 0)

	out := runGate(t, viewer, func() (bool, error) { return Look("bobrick", viewer, room, 0) })
	require.NotContains(t, out, shapesLookHint)
	require.NotContains(t, out, "Look at what???")
}

func TestWho_ViewerInTheDarkIsRefused(t *testing.T) {
	viewer, room := seedShapesRosterRoom(t, 10, 0)

	out := runGate(t, viewer, func() (bool, error) { return Who("", viewer, room, 0) })
	require.Contains(t, out, tooDarkToSeeLine)
	require.NotContains(t, out, "Bobrick")
}

func TestWho_SightedViewerIsNotRefused(t *testing.T) {
	for name, c := range map[string]struct{ lamp, condition int }{
		"shapes": {10, gateInfraConditionId},
		"clear":  {60, 0},
	} {
		t.Run(name, func(t *testing.T) {
			viewer, room := seedShapesRosterRoom(t, c.lamp, c.condition)
			out := runGate(t, viewer, func() (bool, error) { return Who("", viewer, room, 0) })
			require.NotContains(t, out, tooDarkToSeeLine)
			require.NotContains(t, out, blindLookLine)
		})
	}
}
