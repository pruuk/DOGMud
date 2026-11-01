package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/stretchr/testify/require"
)

// 2026-10-17 pre-deploy playtest: a bare `drink` read "You aren't carrying
// that. Try inventory." while get, drop and sell ask "<Verb> what?". Every
// verb that acts on one of the player's items now asks when none is named.
func TestBareItemVerbsAskWhat(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	for _, verb := range []string{"drink", "eat", "quaff", "read", "use", "remove", "stash"} {
		events.DrainQueuedMessagesForTest(user.UserId)
		handled, err := userCommands[verb].Func("", user, room, 0)
		require.NoError(t, err, verb)
		require.True(t, handled, verb)
		require.Equal(t, refusal.PromptText(verb, refusal.What, verb+" <item>")+"\n",
			refusalPlain(events.DrainQueuedMessagesForTest(user.UserId)), verb)
	}
}

// refusalPlain is the one refusal line sent, unwrapped from its category tag.
func refusalPlain(lines []string) string {
	if len(lines) != 1 {
		return ""
	}
	const open, close = `<ansi fg="refusal">`, `</ansi>`
	l := lines[0]
	if len(l) > len(open)+len(close) && l[:len(open)] == open {
		l = l[len(open):]
		if i := len(l) - len(close) - 1; i >= 0 && l[i:] == close+"\n" {
			l = l[:i] + "\n"
		}
	}
	return l
}
