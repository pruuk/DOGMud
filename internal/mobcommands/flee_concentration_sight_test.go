package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
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
	require.Contains(t, sighted, "Skeleton's concentration breaks.")
	require.Contains(t, blind, messaging.SoundChantBreaksOff)
	for _, line := range blind {
		require.NotContains(t, line, "Skeleton", "a reader who sees nothing must not learn who broke off")
	}
}
