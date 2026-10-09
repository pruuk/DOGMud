package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4, closing playtest 2026-10-08: a mob that flees
// mid-cast broke its concentration on the visual channel only, so a reader
// who saw nothing got no line at all. A break is heard. mobSpeechRoom lights
// room 1 exactly: Aliceia (user 1) sees clearly, Bobrick (user 2) is blinded.
// The line is the shared spell-disruption wording ("X's concentration
// breaks."), not the flee's own "breaks their concentration."
func TestFlee_CastingMobBreakIsSeenAndHeard(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)
	mob.Character.SetAggro(1, 0, characters.DefaultAttack)
	startTestCast(t, mob)

	_, err := Flee("", mob, room)
	require.NoError(t, err)

	sighted, blind := mobSpeechHeard(1), mobSpeechHeard(2)
	// Each reader gets exactly one of the two lines.
	require.Equal(t, []string{"Skeleton's concentration breaks."}, sighted)
	require.Equal(t, []string{messaging.SoundChantBreaksOff}, blind)
}

// The line names the mob as the hooks package's mob room lines do: a second
// Skeleton in the room reads with its duplicate number ("Skeleton #2").
func TestFlee_CastingDuplicateMobBreakCarriesItsNumber(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	_, room := mobSpeechRoom(t)
	second := mobs.GetInstance(200)
	require.NotNil(t, second)
	second.Character.Name = "Skeleton"
	require.Equal(t, 2, room.GetMobDuplicateIndex(second.InstanceId))
	second.Character.SetAggro(1, 0, characters.DefaultAttack)
	startTestCast(t, second)

	_, err := Flee("", second, room)
	require.NoError(t, err)

	require.Equal(t, []string{"Skeleton #2's concentration breaks."}, mobSpeechHeard(1))
}

// A hidden caster is unnamed to every reader, a full-sight one included, as
// mobSubjectName reads a hidden mob in the hooks package.
func TestFlee_CastingHiddenMobBreakReadsSomething(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)
	mob.Character.SetAggro(1, 0, characters.DefaultAttack)
	reason := state.TransitionReason{Trigger: "flee_concentration_sight_test"}
	require.NoError(t, mob.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	mob.Character.Awareness.ResolveConcealment(true, reason)
	require.True(t, mob.Character.IsHidden())
	startTestCast(t, mob)

	_, err := Flee("", mob, room)
	require.NoError(t, err)

	require.Equal(t, []string{"Something's concentration breaks."}, mobSpeechHeard(1))
}
