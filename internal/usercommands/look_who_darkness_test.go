package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/stretchr/testify/require"
)

// File: look_who_darkness_test.go
//
// #364. A player who sees nothing because the room is too dark is told that
// darkness is the reason, and what would help; a blinded player keeps the
// plain refusal, since light would not help them.

const (
	tooDarkToSeeLine = "It is too dark to see. You need light, or eyes that do not need it."
	blindLookLine    = "You can't see anything!"
)

func TestLookAndWho_TooDarkNamesTheDarkness(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 0)
	for verb, cmd := range map[string]func() (bool, error){
		"look": func() (bool, error) { return Look("", user, room, 0) },
		"who":  func() (bool, error) { return Who("", user, room, 0) },
	} {
		out := runGate(t, user, cmd)
		require.Contains(t, out, tooDarkToSeeLine, "%s in a pitch-dark room", verb)
		require.NotContains(t, out, blindLookLine, "%s in a pitch-dark room", verb)
	}
}

func TestLookAndWho_BlindedKeepsThePlainRefusal(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 90)
	orig := user.Character.Perception
	t.Cleanup(func() { user.Character.Perception = orig })
	user.Character.Perception = characters.New().Perception
	require.NoError(t, user.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))

	for verb, cmd := range map[string]func() (bool, error){
		"look": func() (bool, error) { return Look("", user, room, 0) },
		"who":  func() (bool, error) { return Who("", user, room, 0) },
	} {
		out := runGate(t, user, cmd)
		require.Contains(t, out, blindLookLine, "%s while blinded in a lit room", verb)
		require.NotContains(t, out, tooDarkToSeeLine, "%s while blinded in a lit room", verb)
	}
}
