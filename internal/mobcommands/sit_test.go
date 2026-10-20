package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/stretchr/testify/require"
)

// A mob sits with `sit` or `rest` and gets up with `stand` (#637), through
// the mob dispatcher. The seeded keywords carry no command aliases, so `rest`
// reaching Sit here proves the mob registry's own synonym.
func TestMobSitAndRestBothSeatAndStandGetsUp(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	require.NoError(t, mob.Character.Validate())

	for _, verb := range []string{"sit", "rest"} {
		handled, err := TryCommand(verb, "", 100)
		require.True(t, handled, verb)
		require.NoError(t, err, verb)
		require.True(t, mob.Character.IsSitting(), "%s must seat the mob, state %v", verb, mob.Character.Position.State())

		handled, err = TryCommand("stand", "", 100)
		require.True(t, handled)
		require.NoError(t, err)
		require.True(t, mob.Character.IsStanding(), "stand must get a sitting mob up")
	}
}
