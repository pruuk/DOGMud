package inputhandlers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// termTypeConn registers a test connection for the terminal type tests.
func termTypeConn(t *testing.T) connections.ConnectionId {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false)
	const id connections.ConnectionId = 97501
	connections.RegisterTestConnection(id)
	t.Cleanup(func() { connections.UnregisterTestConnection(id) })
	return id
}

// The usual cycle: name, terminal, MTTS. MTTS ends it and sets the bits.
func TestRecordTermType_MTTSEndsTheCycle(t *testing.T) {
	id := termTypeConn(t)
	require.True(t, recordTermType(id, `MUDLET`))
	require.True(t, recordTermType(id, `ANSI-TRUECOLOR`))
	require.False(t, recordTermType(id, `MTTS 2441`)) // 2441 = 2048+256+128+8+1, bit 64 clear
	cs := connections.GetClientSettings(id)
	require.True(t, cs.MTTSReported)
	require.False(t, cs.ScreenReader)
}

// The screen reader bit is read from an MTTS reply.
func TestRecordTermType_ScreenReaderBit(t *testing.T) {
	id := termTypeConn(t)
	require.False(t, recordTermType(id, `MTTS 73`)) // 64+8+1
	cs := connections.GetClientSettings(id)
	require.True(t, cs.MTTSReported)
	require.True(t, cs.ScreenReader)
}

// A client without MTTS repeats itself; the repeat ends the cycle, and the
// question will be asked.
func TestRecordTermType_RepeatEndsWithoutMTTS(t *testing.T) {
	id := termTypeConn(t)
	require.True(t, recordTermType(id, `XTERM`))
	require.False(t, recordTermType(id, `XTERM`))
	require.False(t, connections.GetClientSettings(id).MTTSReported)
}

// A client that never repeats is asked at most maxTermTypeReplies times.
func TestRecordTermType_Bounded(t *testing.T) {
	id := termTypeConn(t)
	asks := 0
	for i := range 10 {
		if recordTermType(id, fmt.Sprintf(`T%d`, i)) {
			asks++
		}
	}
	require.Equal(t, maxTermTypeReplies-1, asks)
}

// An over-long reply is kept short.
func TestRecordTermType_LongReplyTrimmed(t *testing.T) {
	id := termTypeConn(t)
	recordTermType(id, strings.Repeat(`A`, 500))
	require.Len(t, connections.GetClientSettings(id).TermTypeLast, maxTermTypeLen)
}

// Through the IAC handler: an MTTS reply is read, and nothing is sent back.
func TestTelnetIACHandler_MTTSReply(t *testing.T) {
	id := termTypeConn(t)
	in := append([]byte{term.TELNET_IAC, term.TELNET_SB, term.TELNET_OPT_TERM_TYPE, term.TELNET_TTYPE_IS}, []byte(`MTTS 64`)...)
	in = append(in, term.TELNET_IAC, term.TELNET_SE)
	TelnetIACHandler(&connections.ClientInput{ConnectionId: id, DataIn: in}, map[string]any{})
	require.True(t, connections.GetClientSettings(id).ScreenReader)
}

// A client that refuses terminal type changes nothing: the question is asked.
// (A regression guard: it passes before the handler branch exists too, since
// an unhandled WONT also changes nothing.)
func TestTelnetIACHandler_WontTermTypeChangesNothing(t *testing.T) {
	id := termTypeConn(t)
	before := connections.GetClientSettings(id)
	TelnetIACHandler(&connections.ClientInput{ConnectionId: id, DataIn: []byte{term.TELNET_IAC, term.TELNET_WONT, term.TELNET_OPT_TERM_TYPE}}, map[string]any{})
	require.Equal(t, before, connections.GetClientSettings(id))
}

// TestTelnetIACHandlerShortScreenSizeReport sends a NAWS report cut short after
// three bytes. It used to index past the payload and panic on the connection
// goroutine; DOGMud's recover in handleTelnetConnection kept the server up but
// still dropped the connection. Ported from upstream GoMud #647.
func TestTelnetIACHandlerShortScreenSizeReport(t *testing.T) {
	clientInput := &connections.ClientInput{
		ConnectionId: 1,
		DataIn:       []byte{term.TELNET_IAC, term.TELNET_SB, term.TELNET_OPT_NAWS, 0, 80, 0},
	}

	assert.NotPanics(t, func() {
		TelnetIACHandler(clientInput, map[string]any{})
	})
}
