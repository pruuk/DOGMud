package usercommands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The tutorial starts newcomers at medium, except a screen reader user,
// who keeps light (spec 2026-11-17, ruling 5).
func TestNewcomerCombatVerbosity(t *testing.T) {
	require.Equal(t, `medium`, newcomerCombatVerbosity(``, false))
	require.Equal(t, `medium`, newcomerCombatVerbosity(`light`, false))
	require.Equal(t, `light`, newcomerCombatVerbosity(`light`, true))
}
