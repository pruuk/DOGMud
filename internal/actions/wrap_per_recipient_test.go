package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The reason Speech left the senders' hands: one line to a room, two readers
// at two widths. Clara reads at telnet's 80; Dena is on the web at 40 columns.
func TestSay_OneLineWrapsForEachReader(t *testing.T) {
	sc := newSpeechScene(t)
	webConn := users.GetByUserId(speechDeafId).ConnectionId()
	connections.RegisterTestWebsocketConnection(webConn)
	t.Cleanup(func() { connections.UnregisterTestConnection(webConn) })
	cs := connections.GetClientSettings(webConn)
	cs.Display.ScreenWidth = 40
	connections.OverwriteClientSettings(webConn, cs)

	text := `The pool answers first, it is older than the town and more patient than I am, so look into the water and wait.`
	Say(sc.speaker(true, sc.lit), text)

	heard := func(uid int) []string {
		msgs := events.DrainQueuedMessagesForTest(uid)
		require.Len(t, msgs, 1, "user %d hears the line once", uid)
		return wrapGoldenVisible(strings.TrimRight(msgs[0], "\n"))
	}
	telnet := heard(speechClearId)
	web := heard(speechDeafId)

	require.Len(t, telnet, 2, "at 80 the line takes two lines: %q", telnet)
	require.Greater(t, len(web), 3, "at 40 the same line takes more than three: %q", web)
	for _, line := range telnet {
		require.LessOrEqual(t, len([]rune(line)), 80, "telnet line: %q", line)
	}
	for _, line := range web {
		require.LessOrEqual(t, len([]rune(line)), 40, "web line: %q", line)
	}
	require.Equal(t, strings.Join(telnet, " "), strings.Join(web, " "), "the same words reach both")
}
