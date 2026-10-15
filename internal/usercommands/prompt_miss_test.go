package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// M7 slice 2: a typed name that matches nothing reads the slice 1
// not-here line, never a question back at the player.
func TestPromptMisses_ReadNotHere(t *testing.T) {
	cases := []struct {
		name string
		cmd  UserCommand
		rest string
		want string
	}{
		{"ask", Ask, "zzyzx to sit", refusal.NotHereCreatureLine},
		{"talk", Talk, "zzyzx", refusal.NotHereCreatureLine},
		{"look", Look, "zzyzx", refusal.NotHereThingLine},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user, room := aimScene(t, aimFull)
			handled, err := c.cmd(c.rest, user, room, 0)
			require.NoError(t, err)
			require.True(t, handled)
			out := aimTold(1)
			assert.Contains(t, out, c.want)
			assert.NotContains(t, out, "zzyzx", "no echo")
			assert.NotContains(t, out, "?", "a miss is not a question")
		})
	}
}

// Bobrick (user 2) is a player in the fixture room.
func TestTalkToAPlayerSaysUseSay(t *testing.T) {
	user, room := aimScene(t, aimFull)
	handled, err := Talk("bobrick", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	// Raw, not aimTold: that helper strips anything shaped <...>, which
	// would eat the "<message>" placeholder in the line.
	raw := strings.Join(events.DrainQueuedMessagesForTest(1), "")
	assert.Contains(t, raw, refusal.TalkToPlayerLine)
}
