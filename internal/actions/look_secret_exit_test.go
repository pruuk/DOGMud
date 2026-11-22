package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 1: `look north` at a secret exit the looker has not found
// reads as if no exit were there; once found it looks through it.
func TestResolveLook_AnUnfoundSecretExitIsNotAnExit(t *testing.T) {
	actor := scanLockScene(t, 90, false, false, true)
	here := rooms.LoadRoom(scanLockHereId)
	north := here.Exits["north"]
	north.Secret = true
	here.Exits["north"] = north

	res := ResolveLook(actor, "north")
	require.NotEqual(t, LookExit, res.Kind, "an unfound secret exit must not be looked through")
	require.Equal(t, "", res.ExitName)

	actor.char.LearnSecretExit(scanLockHereId, "north")
	res = ResolveLook(actor, "north")
	require.Equal(t, LookExit, res.Kind)
	require.Equal(t, "north", res.ExitName)
}
