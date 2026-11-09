package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/stretchr/testify/require"
)

// clientDisplayConn registers a websocket connection whose stored screen width
// is start, and returns its id.
func clientDisplayConn(t *testing.T, start uint32) uint64 {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false) // the handler logs what it ignores
	const id uint64 = 97201
	connections.RegisterTestWebsocketConnection(id)
	t.Cleanup(func() { connections.UnregisterTestConnection(id) })
	cs := connections.GetClientSettings(id)
	cs.Display.ScreenWidth = start
	connections.OverwriteClientSettings(id, cs)
	return id
}

func storedWidth(id uint64) uint32 {
	return connections.GetClientSettings(id).Display.ScreenWidth
}

func reportCols(id uint64, payload string) events.ListenerReturn {
	return clientDisplayHandler(GMCPClientDisplay{ConnectionId: id, Payload: []byte(payload)})
}

// HandleIAC runs on the connection goroutine, so it only queues the report,
// and the queued payload is a copy: the read buffer is reused after it returns.
func TestHandleIAC_ClientDisplayIsQueuedWithACopiedPayload(t *testing.T) {
	mudlog.SetupLogger(nil, ``, ``, false) // HandleIAC logs each command
	events.DrainQueuedByTypeForTest(`GMCPClientDisplay`)
	t.Cleanup(func() { events.DrainQueuedByTypeForTest(`GMCPClientDisplay`) })

	frame := gmcpFrame(`Client.Display {"cols":42}`)
	require.True(t, gmcpModule.HandleIAC(97202, frame))
	for i := range frame {
		frame[i] = 'x' // the read loop reuses this buffer
	}

	queued := events.DrainQueuedByTypeForTest(`GMCPClientDisplay`)
	require.Len(t, queued, 1)
	got := queued[0].(GMCPClientDisplay)
	require.Equal(t, uint64(97202), got.ConnectionId)
	require.Equal(t, `{"cols":42}`, string(got.Payload))
}

func TestClientDisplay_StoresAWholeNumberInRange(t *testing.T) {
	id := clientDisplayConn(t, 80)
	require.Equal(t, events.Continue, reportCols(id, `{"cols":42}`))
	require.Equal(t, uint32(42), storedWidth(id))
}

func TestClientDisplay_ClampsOutOfRange(t *testing.T) {
	for _, tt := range []struct {
		payload string
		want    uint32
	}{
		{`{"cols":5}`, 30},
		{`{"cols":0}`, 30},
		{`{"cols":-7}`, 30},
		{`{"cols":30}`, 30},
		{`{"cols":80}`, 80},
		{`{"cols":81}`, 80},
		{`{"cols":5000}`, 80},
	} {
		id := clientDisplayConn(t, 55)
		reportCols(id, tt.payload)
		require.Equal(t, tt.want, storedWidth(id), tt.payload)
	}
}

// Everything that is not a whole number leaves the stored width alone.
func TestClientDisplay_IgnoresABadPayload(t *testing.T) {
	for _, payload := range []string{
		``,
		`not json`,
		`{"cols":`,
		`{}`,
		`{"cols":null}`,
		`{"cols":"42"}`,
		`{"cols":42.5}`,
		`{"cols":[42]}`,
		`{"cols":99999999999999999999}`,
		`[42]`,
	} {
		id := clientDisplayConn(t, 55)
		require.Equal(t, events.Continue, reportCols(id, payload), payload)
		require.Equal(t, uint32(55), storedWidth(id), "payload %q changed the width", payload)
	}
}

func TestClientDisplay_IgnoresANonWebsocketConnection(t *testing.T) {
	mudlog.SetupLogger(nil, ``, ``, false)
	const id uint64 = 97203
	connections.RegisterTestConnection(id)
	t.Cleanup(func() { connections.UnregisterTestConnection(id) })
	cs := connections.GetClientSettings(id)
	cs.Display.ScreenWidth = 132
	connections.OverwriteClientSettings(id, cs)

	reportCols(id, `{"cols":42}`)

	require.Equal(t, uint32(132), storedWidth(id), "a telnet client's width comes from NAWS only")
}

// A report equal to the effective width writes nothing. The stored 0 reads as
// 80 through GetScreenWidth, so a write of 80 would be visible here.
func TestClientDisplay_UnchangedValueWritesNothing(t *testing.T) {
	id := clientDisplayConn(t, 0)
	reportCols(id, `{"cols":80}`)
	require.Equal(t, uint32(0), storedWidth(id))
}

// The handler reaches the width through the event bus, or the feature is dead
// however green the direct calls are.
func TestClientDisplay_ListenerIsRegistered(t *testing.T) {
	id := clientDisplayConn(t, 80)
	// No swap and no register call: the listener must already be on the bus
	// from the package's init, or this fails.

	events.DoListeners(GMCPClientDisplay{ConnectionId: id, Payload: []byte(`{"cols":61}`)})

	require.Equal(t, uint32(61), storedWidth(id))
}
