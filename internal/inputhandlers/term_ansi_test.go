package inputhandlers

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/stretchr/testify/require"
)

// ansiSizeConn registers a telnet (or websocket) test connection for the
// ANSI screen size tests.
func ansiSizeConn(t *testing.T, websocket bool) connections.ConnectionId {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false)
	const id connections.ConnectionId = 97601
	if websocket {
		connections.RegisterTestWebsocketConnection(id)
	} else {
		connections.RegisterTestConnection(id)
	}
	t.Cleanup(func() { connections.UnregisterTestConnection(id) })
	return id
}

// sendAnsiCursorReport sends the reply a terminal gives to the connect-time
// cursor query: ESC [ row ; col R.
func sendAnsiCursorReport(id connections.ConnectionId, report string) {
	AnsiHandler(&connections.ClientInput{ConnectionId: id, DataIn: []byte("\x1b[" + report + "R")}, map[string]any{})
}

// sendNAWS sends a telnet NAWS window size report.
func sendNAWS(id connections.ConnectionId, w, h int) {
	in := []byte{term.TELNET_IAC, term.TELNET_SB, term.TELNET_OPT_NAWS, byte(w >> 8), byte(w), byte(h >> 8), byte(h), term.TELNET_IAC, term.TELNET_SE}
	TelnetIACHandler(&connections.ClientInput{ConnectionId: id, DataIn: in}, map[string]any{})
}

func screenSize(id connections.ConnectionId) (int, int) {
	d := connections.GetClientSettings(id).Display
	return int(d.ScreenWidth), int(d.ScreenHeight)
}

// A telnet client without NAWS gets the size its cursor report gives. Before
// #915 an inverted err check threw every report away.
func TestAnsiCursorReport_StoresTelnetSize(t *testing.T) {
	id := ansiSizeConn(t, false)
	sendAnsiCursorReport(id, "50;132")
	w, h := screenSize(id)
	require.Equal(t, 132, w)
	require.Equal(t, 50, h)
}

// NAWS owns the size once it has reported: a cursor report after it changes
// nothing.
func TestAnsiCursorReport_NAWSFirstWins(t *testing.T) {
	id := ansiSizeConn(t, false)
	sendNAWS(id, 100, 40)
	sendAnsiCursorReport(id, "50;132")
	w, h := screenSize(id)
	require.Equal(t, 100, w)
	require.Equal(t, 40, h)
}

// A NAWS report after the cursor report overwrites it.
func TestAnsiCursorReport_NAWSLaterWins(t *testing.T) {
	id := ansiSizeConn(t, false)
	sendAnsiCursorReport(id, "50;132")
	sendNAWS(id, 100, 40)
	w, h := screenSize(id)
	require.Equal(t, 100, w)
	require.Equal(t, 40, h)
}

// A modified F3 key has the cursor report's shape (Ctrl+F3 is ESC[1;5R) and
// must not become a 5 column screen; nor may a narrow or huge report.
func TestAnsiCursorReport_IgnoresWhatIsNotAScreen(t *testing.T) {
	for _, report := range []string{"1;5", "1;2", "50;20", "5;80", "70000;80", "50;70000", "-50;80"} {
		id := ansiSizeConn(t, false)
		sendAnsiCursorReport(id, report)
		w, h := screenSize(id)
		require.Zero(t, w, "report %q set the width", report)
		require.Zero(t, h, "report %q set the height", report)
	}
}

// A websocket's width is the web client's clamped report; a cursor report
// never touches it.
func TestAnsiCursorReport_IgnoresAWebsocket(t *testing.T) {
	id := ansiSizeConn(t, true)
	sendAnsiCursorReport(id, "50;132")
	w, h := screenSize(id)
	require.Zero(t, w)
	require.Zero(t, h)
}
