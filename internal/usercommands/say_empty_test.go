package usercommands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// #260: an empty `say` printed `You say, ""` and spoke a blank line to the
// room. It is refused now, and nobody else hears anything. Whitespace alone
// counts as empty.
func TestSay_EmptyIsRefused(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	for _, rest := range []string{"", "   "} {
		alice, _, room := speechWrapperScene(t)
		handled, err := Say(rest, alice, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		require.Equal(t, []string{"Say what?"}, speechWrapperHeard(1), "rest %q", rest)
		require.Empty(t, speechWrapperHeard(2), "rest %q: the room heard an empty say", rest)
	}
}
