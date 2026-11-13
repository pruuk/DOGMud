package gmcp

import (
	"encoding/json"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func init() {
	registerClientDisplayListener()
}

// registerClientDisplayListener wires the handler to the event bus. Split from
// init so a test can register it against a swapped bus and drive it through
// DoListeners.
func registerClientDisplayListener() {
	events.RegisterListener(GMCPClientDisplay{}, clientDisplayHandler)
}

// GMCPClientDisplay carries a client's Client.Display report from the
// connection goroutine (HandleIAC) to MainWorker, where clientDisplayHandler
// judges it and stores the width on the connection.
type GMCPClientDisplay struct {
	ConnectionId uint64
	Payload      []byte
}

func (g GMCPClientDisplay) Type() string { return `GMCPClientDisplay` }

// clientDisplayHandler stores a web client's reported column count. The
// payload comes from the client: only a websocket connection is listened to,
// only a whole number counts, the value is clamped before it is stored, and
// it is written only when it differs. A client can change only its own
// connection, so a phone never changes the player's desktop or saved setting.
func clientDisplayHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(GMCPClientDisplay)
	if !ok {
		mudlog.Error("Event", "Expected Type", "GMCPClientDisplay", "Actual Type", e.Type())
		return events.Cancel
	}
	if !connections.IsWebsocket(evt.ConnectionId) {
		return events.Continue
	}
	cols, ok := reportedCols(evt.Payload)
	if !ok {
		mudlog.Debug("gmcp.ClientDisplay", "connectionId", evt.ConnectionId, "ignored", "payload is not a whole column count")
		return events.Continue
	}
	users.StoreReportedCols(evt.ConnectionId, cols)
	return events.Continue
}

// reportedCols reads {"cols": n}. A fraction, a string, a null, a missing key
// and malformed JSON all fail to decode into the *int and are refused.
func reportedCols(payload []byte) (int, bool) {
	var decoded struct {
		Cols *int `json:"cols"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil || decoded.Cols == nil {
		return 0, false
	}
	return *decoded.Cols, true
}
