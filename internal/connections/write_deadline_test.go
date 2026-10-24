package connections

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestMain initializes mudlog. Broadcast/SendTo log a warning on a failed
// write, and mudlog's package-level slog instance is nil until SetupLogger
// runs - without this, exercising the failure path panics inside the logger
// instead of testing the connection layer.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(testLogs, `L`, ``, false)
	os.Exit(m.Run())
}

// waitFor runs fn in a goroutine and returns its error, or fails the test if it
// has not returned within limit. Every test in this file goes through it so a
// regression shows up as a failure, never as a hung test binary.
func waitFor(t *testing.T, limit time.Duration, what string, fn func() error) error {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	timer := time.NewTimer(limit)
	defer timer.Stop()

	select {
	case err := <-done:
		return err
	case <-timer.C:
		t.Fatalf("%s did not return within %s - it is blocking indefinitely", what, limit)
		return nil
	}
}

func isTimeout(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// TestWriteDeadlineBoundsBlockedWrite is the core proof for the deadline fix.
//
// net.Pipe() is a fully synchronous, zero-buffer connection: a write blocks
// until the peer reads. Nothing ever reads here, so without a write deadline
// this Write would block forever - which is exactly what a real client with a
// full TCP receive window did to the game loop.
func TestWriteDeadlineBoundsBlockedWrite(t *testing.T) {

	clientSide, peerSide := net.Pipe()
	defer clientSide.Close()
	defer peerSide.Close()

	cd := &ConnectionDetails{
		connectionId: 90001,
		conn:         clientSide,
	}

	start := time.Now()

	// Generous ceiling: we are asserting "bounded", not "exactly 5s".
	// writeNow is the writer goroutine's socket write; the deadline it is
	// given is the one the writer computes (now + writeTimeout).
	err := waitFor(t, writeTimeout+10*time.Second, "ConnectionDetails.writeNow", func() error {
		return cd.writeNow([]byte("this write has no reader on the other end"), time.Now().Add(writeTimeout))
	})

	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected a timeout error writing to an unread pipe, got nil after %s", elapsed)
	}
	if !isTimeout(err) {
		t.Fatalf("expected a net timeout error, got %T: %v", err, err)
	}
	if elapsed < writeTimeout/2 {
		t.Fatalf("write returned after only %s - deadline does not look like it was applied", elapsed)
	}
	t.Logf("blocked write returned %v after %s (writeTimeout=%s)", err, elapsed, writeTimeout)
}

// NOTE: a real-TCP variant of the test above was tried and removed. Windows
// loopback absorbed a 32 MiB write into socket buffers in ~30ms even with
// SetWriteBuffer/SetReadBuffer shrunk to 2 KiB, so the assertion proved
// nothing there. net.Pipe is the stronger seam anyway: it has no buffer at
// all, and the deadline call under test (cd.conn.SetWriteDeadline) is on the
// net.Conn interface, so it is transport-agnostic.

// TestWriteToDetachedConnectionDoesNotPanic covers the snapshot-then-write
// hazard: Broadcast/SendTo now resolve *ConnectionDetails under the lock and
// write after releasing it, so the connection can be torn down in between.
func TestWriteToDetachedConnectionDoesNotPanic(t *testing.T) {

	// No socket at all (the shape RegisterTestConnection produces).
	cd := &ConnectionDetails{connectionId: 90003}

	if _, err := cd.Write([]byte("hello")); !errors.Is(err, ErrNoConnection) {
		t.Fatalf("expected ErrNoConnection writing to a socketless connection, got %v", err)
	}

	// Also safe to Close - Remove() calls it.
	cd.Close()

	if got := cd.remoteAddrString(); got != "unknown" {
		t.Fatalf("expected remoteAddrString()==%q for a socketless connection, got %q", "unknown", got)
	}

	// A closed socket must error rather than panic.
	clientSide, peerSide := net.Pipe()
	peerSide.Close()
	clientSide.Close()

	closedCd := &ConnectionDetails{connectionId: 90004, conn: clientSide}
	err := waitFor(t, 5*time.Second, "Write to closed pipe", func() error {
		return closedCd.writeNow([]byte("hello"), time.Now().Add(writeTimeout))
	})
	if err == nil {
		t.Fatal("expected an error writing to a closed connection, got nil")
	}
}

// TestSendToReleasesLockBeforeWriting: while a connection's write is parked on
// a peer that never reads, the package lock must be free, or a wedged client
// could not even be kicked. Since 2026-10-24 the parked write is on the
// connection's writer goroutine and SendTo itself returns at once.
func TestSendToReleasesLockBeforeWriting(t *testing.T) {

	clientSide, peerSide := net.Pipe()
	defer clientSide.Close()
	defer peerSide.Close()

	const testId ConnectionId = 90005

	lock.Lock()
	netConnections[testId] = &ConnectionDetails{
		connectionId: testId,
		state:        LoggedIn,
		conn:         clientSide,
	}
	lock.Unlock()

	t.Cleanup(func() {
		lock.Lock()
		delete(netConnections, testId)
		lock.Unlock()
	})

	sendDone := make(chan struct{})
	go func() {
		defer close(sendDone)
		SendTo([]byte("a message nobody will read"), testId)
	}()

	// Wait until the writer has taken the message and is inside the write.
	lock.RLock()
	q := netConnections[testId].sendQueue()
	lock.RUnlock()
	waitUntil(t, 2*time.Second, "writer is mid-write", func() bool {
		q.mu.Lock()
		defer q.mu.Unlock()
		return len(q.items) == 0 && q.queuedBytes > 0
	})

	// Remove() takes the exclusive package lock. If anything held it across
	// the write, this would block for the full writeTimeout.
	waitFor(t, 2*time.Second, "Remove() while a write is parked", func() error {
		Remove(999999) // not a real connection; we only care that it can lock
		return nil
	})

	select {
	case <-sendDone:
	case <-time.After(fastCallLimit):
		t.Fatal("SendTo did not return at once")
	}
}
