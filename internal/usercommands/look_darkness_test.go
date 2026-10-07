package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/stretchr/testify/require"
)

// File: look_darkness_test.go
//
// #364. A player who sees nothing because the room is too dark is told that
// darkness is the reason, and what would help; a blinded player keeps the
// plain refusal, since light would not help them. (`who` shared this until
// #421 made it an alias of `online`.)

const (
	tooDarkToSeeLine = "It is too dark to see. You need light, or eyes that do not need it."
	blindLookLine    = "You can't see anything!"
)

func TestLook_TooDarkNamesTheDarkness(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 0)
	out := runGate(t, user, func() (bool, error) { return Look("", user, room, 0) })
	require.Contains(t, out, tooDarkToSeeLine, "look in a pitch-dark room")
	require.NotContains(t, out, blindLookLine, "look in a pitch-dark room")
}

func TestLook_BlindedKeepsThePlainRefusal(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 90)
	orig := user.Character.Perception
	t.Cleanup(func() { user.Character.Perception = orig })
	user.Character.Perception = characters.New().Perception
	require.NoError(t, user.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))

	out := runGate(t, user, func() (bool, error) { return Look("", user, room, 0) })
	require.Contains(t, out, blindLookLine, "look while blinded in a lit room")
	require.NotContains(t, out, tooDarkToSeeLine, "look while blinded in a lit room")
}
