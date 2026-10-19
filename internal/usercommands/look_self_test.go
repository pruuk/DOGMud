package usercommands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// #606: looking at yourself by name shows your description but does not tell
// you "<you> is looking at you." in the third person.
func TestLook_AtYourselfSendsNoLookingAtYouLine(t *testing.T) {
	viewer, room := seedDarknessGateRoom(t, 90)

	out := runGate(t, viewer, func() (bool, error) { return Look("aliceia", viewer, room, 0) })
	require.NotContains(t, out, "is looking at you")
	require.NotContains(t, out, "is looking at")
}
