package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/stretchr/testify/require"
)

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
