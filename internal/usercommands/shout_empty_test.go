package usercommands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// #260: an empty `shout` broadcast a blank shout to this room and the next.
// It is refused now, as `say` is, and nobody else hears anything. Whitespace
// alone counts as empty.
func TestShout_EmptyIsRefused(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	for _, rest := range []string{"", "   "} {
		alice, _, room := speechWrapperScene(t)
		handled, err := Shout(rest, alice, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		require.Equal(t, []string{"Shout what?"}, speechWrapperHeard(1), "rest %q", rest)
		require.Empty(t, speechWrapperHeard(2), "rest %q: the room heard an empty shout", rest)
	}
}
