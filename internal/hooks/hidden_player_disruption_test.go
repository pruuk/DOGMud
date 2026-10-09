package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A player can be hidden while casting: sneak first, then cast. The Activity
// veto (Awareness_Vetoes.go) only stops a caster from starting to sneak, and
// nothing in the cast command reveals. Such a caster's fizzle, falter or
// broken concentration named them to the room, while the mob twins read
// "Something" for a hidden caster (mobSubjectName). A player caster reads
// alike now.

// hidePlayer drives u's Awareness into Hidden the way the sneak command does.
func hidePlayer(t *testing.T, u *users.UserRecord) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "hidden_player_disruption_test"}
	require.NoError(t, u.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	u.Character.Awareness.ResolveConcealment(true, reason)
	require.True(t, u.Character.IsHidden())
	t.Cleanup(func() { u.Character.Awareness.ForceVisible(reason) })
}

func TestHiddenPlayerCaster_DisruptionLinesDoNotNameThem(t *testing.T) {
	cases := []struct {
		name string
		send func(caster *users.UserRecord, room *rooms.Room)
		want string
	}{
		{"fizzle", func(c *users.UserRecord, r *rooms.Room) { sendPlayerSpellFailed(c, r, "fizzles") },
			"Something's spell fizzles."},
		{"falter", func(c *users.UserRecord, r *rooms.Room) { sendPlayerSpellFailed(c, r, "falters") },
			"Something's spell falters."},
		{"break", sendPlayerConcentrationBroke,
			"Something's concentration breaks."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := seedFallbackRoom(t, 60, heatEyesConditionId)
			caster := users.GetByUserId(2)
			hidePlayer(t, caster)
			drainPlain(1)

			tc.send(caster, room)

			require.Equal(t, []string{tc.want}, drainPlain(1),
				"a full-sight reader is not told who the hidden caster is")
		})
	}
}
