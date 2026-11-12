package users

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// The pipeline reads WrapWidth: a narrow web reader's narration folds there.
func TestSendText_WrapsAtWrapWidth(t *testing.T) {
	const webConn uint64 = 97103
	connections.RegisterTestWebsocketConnection(webConn)
	t.Cleanup(func() { connections.UnregisterTestConnection(webConn) })
	cs := connections.GetClientSettings(webConn)
	cs.Display.ScreenWidth = 40
	connections.OverwriteClientSettings(webConn, cs)
	u := NewTestUser(9712, "reader", "Reader", webConn)
	events.DrainQueuedMessagesForTest(u.UserId)

	u.SendText(messaging.CategoryTip, "Healing up slow? Some rooms are sanctuaries, temples and certain camps, and they regenerate your health much faster than ordinary rooms do.")

	msgs := events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, msgs, 1)
	lines := strings.Split(strings.TrimRight(msgs[0], "\n"), "\n")
	require.Greater(t, len(lines), 2)
	for _, line := range lines {
		require.LessOrEqual(t, util.VisibleWidth(line), 40, "%q", line)
	}
}

func TestWrapWidth(t *testing.T) {
	const webConn, telnetConn uint64 = 97101, 97102
	connections.RegisterTestWebsocketConnection(webConn)
	connections.RegisterTestConnection(telnetConn)
	t.Cleanup(func() {
		connections.UnregisterTestConnection(webConn)
		connections.UnregisterTestConnection(telnetConn)
	})

	tests := []struct {
		name      string
		conn      uint64
		cols      uint32 // Display.ScreenWidth; 0 is "never reported"
		lineWidth int    // UserRecord.LineWidth; 0 is "unset"
		want      int
	}{
		{"telnet, nothing saved", telnetConn, 0, 0, 80},
		{"telnet, saved width above 80 still works", telnetConn, 0, 120, 120},
		{"telnet, saved narrow width", telnetConn, 0, 40, 40},
		{"telnet, a screen size report is not a wrap width", telnetConn, 132, 0, 80},
		{"web, nothing reported or saved", webConn, 0, 0, 80},
		{"web, narrow report", webConn, 42, 0, 42},
		{"web, saved width narrows a wider report", webConn, 60, 40, 40},
		{"web, saved width cannot widen a report", webConn, 42, 120, 42},
		{"web, saved 100 is capped at 80", webConn, 80, 100, 80},
		{"web, report below the floor", webConn, 12, 0, 30},
		{"web, saved width below the floor", webConn, 60, 20, 30},
		{"web, report above the cap", webConn, 200, 0, 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := connections.GetClientSettings(tt.conn)
			cs.Display.ScreenWidth = tt.cols
			connections.OverwriteClientSettings(tt.conn, cs)

			u := NewTestUser(9710, "wrapper", "Wrapper", tt.conn)
			u.LineWidth = tt.lineWidth
			require.Equal(t, tt.want, u.WrapWidth())
		})
	}
}

func TestWrapWidth_NilAndUnknownConnection(t *testing.T) {
	var nobody *UserRecord
	require.Equal(t, 80, nobody.WrapWidth(), "a nil user gets the default, as GetLineWidth does")

	gone := NewTestUser(9711, "gone", "Gone", 97199)
	gone.LineWidth = 55
	require.Equal(t, 55, gone.WrapWidth(), "a connection that is not registered is not a websocket")
}

// The web client's websocket address carries its columns (/ws?cols=57) so
// the connect splash is drawn to them. Only a whole number counts.
func TestParseReportedCols(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want int
		ok   bool
	}{
		{"57", 57, true},
		{"30", 30, true},
		{"5", 5, true},
		{"5000", 5000, true},
		{"-7", -7, true},
		{"", 0, false},
		{"42.5", 0, false},
		{"42px", 0, false},
		{" 42", 0, false},
		{"abc", 0, false},
		{"99999999999999999999999", 0, false},
		{"+57", 57, true},
	} {
		got, ok := ParseReportedCols(tt.in)
		require.Equal(t, tt.ok, ok, "%q", tt.in)
		require.Equal(t, tt.want, got, "%q", tt.in)
	}
}

// A reported column count is clamped to 30..80 and stored on the connection
// only when it differs; the width stored is returned.
func TestStoreReportedCols(t *testing.T) {
	const conn uint64 = 97104
	connections.RegisterTestWebsocketConnection(conn)
	t.Cleanup(func() { connections.UnregisterTestConnection(conn) })
	stored := func() uint32 { return connections.GetClientSettings(conn).Display.ScreenWidth }

	for _, tt := range []struct{ cols, want int }{
		{57, 57}, {5, 30}, {-7, 30}, {30, 30}, {80, 80}, {81, 80}, {5000, 80}, {42, 42},
	} {
		require.Equal(t, tt.want, StoreReportedCols(conn, tt.cols), "cols %d", tt.cols)
		require.Equal(t, uint32(tt.want), stored(), "cols %d", tt.cols)
	}
}
