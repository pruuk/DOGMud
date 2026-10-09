package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4, closing playtest 2026-10-08: a player who flees
// mid-cast broke concentration on the visual channel only ("A figure breaks
// their concentration."), so a reader who saw nothing heard nothing. A
// break is heard, in the shared spell-disruption wording.

// fleeMidCast starts Aliceia casting in a fight with mob 100 and has her
// flee. The caller's seedAllRegistries cleanup restores the users and rooms.
func fleeMidCast(t *testing.T, alice *users.UserRecord) {
	t.Helper()
	alice.Character.SetAggro(0, 100, characters.DefaultAttack)
	alice.Character.Activity = activity.NewMachine()
	require.NoError(t, alice.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "sparks", ConvictionSpent: 3, FoldsNeeded: 4},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	))

	handled, err := TryCommand("flee", "", alice.UserId, events.CmdSkipScripts)
	require.True(t, handled)
	require.NoError(t, err)
	require.True(t, alice.Character.Activity.IsFree(), "the flee must drop the cast")
}

func TestFlee_CastingPlayerBreakIsHeardByTheBlind(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, bob, _ := speechWrapperScene(t)
	blindForSpeechTest(t, bob)

	fleeMidCast(t, alice)

	heard := speechWrapperHeard(2)
	require.Contains(t, heard, messaging.SoundChantBreaksOff)
	for _, line := range heard {
		require.NotContains(t, line, "Aliceia", "a reader who sees nothing must not learn who broke off")
	}
}

func TestFlee_CastingPlayerBreakNamesTheCasterAtClearSight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, _, _ := speechWrapperScene(t)

	fleeMidCast(t, alice)

	require.Contains(t, speechWrapperHeard(2), "Aliceia's concentration breaks.")
}
