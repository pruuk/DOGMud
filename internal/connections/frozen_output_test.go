package connections

import (
	"strings"
	"testing"
	"time"
)

// Once output is frozen (copyover, after the reconnect tokens are queued),
// Write drops new output without an error, so SendTo does not Remove the
// connection, while output queued before the freeze is still delivered by
// the flush. A failed copyover thaws output again.
func TestSendQueue_FrozenOutputDropsNewWritesKeepsQueued(t *testing.T) {
	id, client := addPipeConnection(t)
	cd := Get(id)

	SendTo([]byte("<queued before freeze>"), id)

	freezeOutput()
	t.Cleanup(ThawOutputAfterFailedCopyover)

	if n, err := cd.Write([]byte("<prompt redraw after freeze>")); err != nil || n != 0 {
		t.Fatalf("Write while frozen = (%d, %v), want (0, nil)", n, err)
	}
	SendTo([]byte("<sendto after freeze>"), id)
	Broadcast([]byte("<broadcast after freeze>"))
	if Get(id) == nil {
		t.Fatal("a write while frozen removed the connection")
	}

	r := startPipeReader(client)
	if !cd.sendQueue().flush(5 * time.Second) {
		t.Fatal("output queued before the freeze did not drain")
	}
	// The pipe write returns once read, slightly before the reader goroutine
	// appends what it read, so wait for it rather than reading at once.
	waitUntil(t, 5*time.Second, "output queued before the freeze arrives", func() bool {
		return strings.Contains(r.text(), "<queued before freeze>")
	})
	if got := r.text(); got != "<queued before freeze>" {
		t.Fatalf("peer received %q, want only the output queued before the freeze", got)
	}

	ThawOutputAfterFailedCopyover()
	SendTo([]byte("<after thaw>"), id)
	waitUntil(t, 5*time.Second, "output sent after a thaw arrives", func() bool {
		return strings.Contains(r.text(), "<after thaw>")
	})
}
