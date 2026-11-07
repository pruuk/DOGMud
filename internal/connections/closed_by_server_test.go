package connections

import (
	"net"
	"testing"
)

// The read loop logs a failed read as INFO, not WARN, when the server closed
// the connection itself (owner 2026-11-07): a refused login closes it, and
// the pending read then fails with an i/o timeout that is no fault.
func TestClosedByServerIsSetOnlyByClose(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	cd := Add(server, nil)
	defer Remove(cd.ConnectionId())

	if cd.ClosedByServer() {
		t.Fatal("a fresh connection must not read as closed by the server")
	}
	cd.Close()
	if !cd.ClosedByServer() {
		t.Fatal("after Close the connection must read as closed by the server")
	}
}
