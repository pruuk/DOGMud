package connections

import (
	"fmt"
	"testing"
	"time"
)

// handoffWarned reports whether flushAllForCopyover logged its "output still
// queued at handoff" WARN for this connection. Pinned to the id: testLogs is
// shared by every test in the package.
func handoffWarned(id ConnectionId) bool {
	return testLogs.contains("WARN", "CopyoverFlush", fmt.Sprintf("connectionId=%d ", id), "output still queued at handoff")
}

// A connection with nothing queued is not reported as "output still queued"
// just because the flush budget ran out on an earlier, stalled connection.
// Budget 0 puts every connection past the budget whatever the map order.
func TestFlushAllForCopyover_IdleConnectionsPastBudgetAreNotReported(t *testing.T) {
	stalledId, _ := addPipeConnection(t) // nobody reads it
	idleA, _ := addPipeConnection(t)
	idleB, _ := addPipeConnection(t)

	SendTo([]byte("held by a peer that never reads"), stalledId)

	conns := map[ConnectionId]*ConnectionDetails{
		stalledId: Get(stalledId),
		idleA:     Get(idleA),
		idleB:     Get(idleB),
	}
	flushAllForCopyover(conns, 0)

	if !handoffWarned(stalledId) {
		t.Fatalf("no handoff WARN for stalled connection %d", stalledId)
	}
	for _, id := range []ConnectionId{idleA, idleB} {
		if handoffWarned(id) {
			t.Errorf("idle connection %d was reported as having output queued at handoff", id)
		}
	}
}

// Within budget, a connection whose peer drains is flushed and not reported.
func TestFlushAllForCopyover_DrainedConnectionNotReported(t *testing.T) {
	id, client := addPipeConnection(t)
	startPipeReader(client)
	SendTo([]byte("delivered before the handoff"), id)

	flushAllForCopyover(map[ConnectionId]*ConnectionDetails{id: Get(id)}, 5*time.Second)

	if handoffWarned(id) {
		t.Fatalf("drained connection %d was reported as having output queued at handoff", id)
	}
}
