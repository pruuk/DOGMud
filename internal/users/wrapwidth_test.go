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

// The raw cols value of the websocket address is what the connect splash is
// drawn at: stored and clamped when it is a whole number, else 80 and nothing
// stored.
func TestStoreAddressCols(t *testing.T) {
	const conn uint64 = 97108
	connections.RegisterTestWebsocketConnection(conn)
	t.Cleanup(func() { connections.UnregisterTestConnection(conn) })
	stored := func() uint32 { return connections.GetClientSettings(conn).Display.ScreenWidth }

	for _, tt := range []struct {
		raw        string
		want       int
		wantStored uint32
	}{
		{"43", 43, 43}, {"12", 30, 30}, {"500", 80, 80}, {"-1", 30, 30},
	} {
		require.Equal(t, tt.want, StoreAddressCols(conn, tt.raw), "%q", tt.raw)
		require.Equal(t, tt.wantStored, stored(), "%q", tt.raw)
	}

	cs := connections.GetClientSettings(conn)
	cs.Display.ScreenWidth = 0
	connections.OverwriteClientSettings(conn, cs)
	for _, raw := range []string{"", "42px", "abc", "99999999999999999999999"} {
		require.Equal(t, 80, StoreAddressCols(conn, raw), "%q", raw)
		require.Equal(t, uint32(0), stored(), "%q stores nothing", raw)
	}
}

// A leaving connection's width: its logged-in user's WrapWidth (so set
// linewidth counts), else the login width.
func TestConnectionWrapWidth(t *testing.T) {
	const userConn, loginConn uint64 = 97109, 97110
	connections.RegisterTestWebsocketConnection(userConn)
	connections.RegisterTestWebsocketConnection(loginConn)
	t.Cleanup(func() {
		connections.UnregisterTestConnection(userConn)
		connections.UnregisterTestConnection(loginConn)
	})
	for _, c := range []uint64{userConn, loginConn} {
		cs := connections.GetClientSettings(c)
		cs.Display.ScreenWidth = 60
		connections.OverwriteClientSettings(c, cs)
	}
	u := NewTestUser(9713, "leaver", "Leaver", userConn)
	u.LineWidth = 40
	t.Cleanup(SeedUsersForTest(map[int]*UserRecord{u.UserId: u}))

	require.Equal(t, 40, ConnectionWrapWidth(userConn), "the user's set linewidth narrows the screen")
	require.Equal(t, 60, ConnectionWrapWidth(loginConn), "no user: the connection's stored columns")
}

// Before login there is no user, so the width comes from the connection: a
// websocket's stored columns, clamped again, or 80 when it stored none; a
// telnet connection is 80 whatever its terminal reported.
func TestLoginWrapWidth(t *testing.T) {
	const webConn, bareWebConn, telnetConn uint64 = 97105, 97106, 97107
	connections.RegisterTestWebsocketConnection(webConn)
	connections.RegisterTestWebsocketConnection(bareWebConn)
	connections.RegisterTestConnection(telnetConn)
	t.Cleanup(func() {
		for _, c := range []uint64{webConn, bareWebConn, telnetConn} {
			connections.UnregisterTestConnection(c)
		}
	})
	setWidth := func(id uint64, w uint32) {
		cs := connections.GetClientSettings(id)
		cs.Display.ScreenWidth = w
		connections.OverwriteClientSettings(id, cs)
	}

	require.Equal(t, 80, LoginWrapWidth(bareWebConn), "a web client that reported no columns")
	for _, tt := range []struct {
		stored uint32
		want   int
	}{{43, 43}, {42, 42}, {30, 30}, {80, 80}, {12, 30}, {200, 80}} {
		setWidth(webConn, tt.stored)
		require.Equal(t, tt.want, LoginWrapWidth(webConn), "web client stored %d", tt.stored)
	}
	setWidth(telnetConn, 40)
	require.Equal(t, 80, LoginWrapWidth(telnetConn), "a telnet terminal that reported 40 columns")
	require.Equal(t, 80, LoginWrapWidth(97199), "a connection that is not registered")
}
