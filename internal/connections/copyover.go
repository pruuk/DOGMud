package connections

import (
	"time"

	"github.com/GoMudEngine/GoMud/internal/copyover"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// flushAllForCopyover waits, budget in total, for every connection's queued
// output to reach its socket. Called by the Unix CopyoverSave (fd_unix.go);
// here so its logic is tested on every platform. The caller holds the
// registry lock; that is safe because a writer marks its queue idle before it
// calls Remove on a failed write, so a failing writer cannot hold this wait up.
func flushAllForCopyover(conns map[ConnectionId]*ConnectionDetails, budget time.Duration) {
	deadline := time.Now().Add(budget)
	for id, cd := range conns {
		if cd.wsConn == nil && cd.conn == nil {
			continue
		}
		q := cd.sendQueue()
		// Checked before the budget: a connection with nothing queued is
		// fine at handoff however late in the loop it is reached.
		if q.idleNow() {
			continue
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || !q.flush(remaining) {
			// mudlog cuts a message at 24 characters, so the detail is an attr.
			mudlog.Warn("CopyoverFlush", "connectionId", id, "error", "output still queued at handoff")
		}
	}
}

// IssueWebSocketReconnectToken is set by main to avoid an import cycle. It is
// called for each WebSocket connection during CopyoverSave (Unix only),
// returning a one-time token the client uses to skip the login prompt after
// reconnecting. Declared here (not the Unix-only fd file) so main can reference
// it on every platform.
var IssueWebSocketReconnectToken func(connectionId ConnectionId) (string, error)

type connectionRecord struct {
	ConnectionId   ConnectionId   `json:"connection_id"`
	State          ConnectState   `json:"state"`
	Fd             int            `json:"fd"`
	ClientSettings ClientSettings `json:"client_settings"`
	// ConnType and StripAnsi carry the listening port's settings across the
	// handoff, so a connection from the AI port stays an AI connection (its
	// per-round command budget, its plain-text output). Absent in a record
	// from an older binary, which restores as a human connection, as before.
	ConnType  ConnType `json:"conn_type,omitempty"`
	StripAnsi bool     `json:"strip_ansi,omitempty"`
}

// restoredConnType is the type a restored connection gets from its record: a
// value this binary does not know restores as ConnHuman.
func (r connectionRecord) restoredConnType() ConnType {
	switch r.ConnType {
	case ConnHuman, ConnAI:
		return r.ConnType
	}
	return ConnHuman
}

type connectionsState struct {
	ConnectCounter uint64             `json:"connect_counter"`
	Records        []connectionRecord `json:"records"`
}

type connectionsCopyoverContributor struct{}

func (c *connectionsCopyoverContributor) CopyoverName() string {
	return "connections"
}

// CopyoverContributor returns the connections contributor for registration.
func CopyoverContributor() copyover.Contributor {
	return &connectionsCopyoverContributor{}
}
