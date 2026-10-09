package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
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

	// Exactly the sound line: no visual line, and no name.
	require.Equal(t, []string{messaging.SoundChantBreaksOff}, speechWrapperHeard(2))
}

// A player hidden in a fight and then casting is reached only through an
// admin's setcondition 9 (hideForStealthRecord has no combat check), since
// engaging reveals and sneak refuses in combat. The break line then reads
// "Something", as the mob flee line and the hooks player lines do.
func TestFlee_HiddenCastingPlayerBreakDoesNotNameThem(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, _, _ := speechWrapperScene(t)

	alice.Character.SetAggro(0, 100, characters.DefaultAttack)
	reason := state.TransitionReason{Trigger: "flee_concentration_sight_test"}
	require.NoError(t, alice.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	alice.Character.Awareness.ResolveConcealment(true, reason)
	require.True(t, alice.Character.IsHidden())
	defer alice.Character.Awareness.ForceVisible(reason)
	events.DrainQueuedMessagesForTest(2)

	alice.Character.Activity = activity.NewMachine()
	require.NoError(t, alice.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "sparks", ConvictionSpent: 3, FoldsNeeded: 4},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	))
	handled, err := TryCommand("flee", "", alice.UserId, events.CmdSkipScripts)
	require.True(t, handled)
	require.NoError(t, err)

	got := speechWrapperHeard(2)
	require.Contains(t, got, "Something's concentration breaks.")
	for _, line := range got {
		require.NotContains(t, line, "Aliceia", "the hidden caster is not named: %v", got)
	}
}

func TestFlee_CastingPlayerBreakNamesTheCasterAtClearSight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, _, _ := speechWrapperScene(t)

	fleeMidCast(t, alice)

	require.Equal(t, []string{"Aliceia's concentration breaks."}, speechWrapperHeard(2))
}
