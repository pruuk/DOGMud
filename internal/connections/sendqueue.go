package connections

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// maxQueuedBytes caps the output one connection may have waiting for its
// socket. A full Zone.Map for a well-explored character is about 271 KB and is
// sent on every room change, so 4 MiB is about 15 full maps: a client that
// far behind is not catching up, and holding more only grows server memory.
// The worst case is about 120 MB with 30 stalled clients, on a production
// droplet with 961 MB. Past the cap the connection is closed rather than
// letting anyone wait. (Owner decision 2026-10-24: lowered from 8 MiB.)
const maxQueuedBytes = 4 << 20

// ErrSendQueueFull is returned by Write when queueing the message would take
// the connection past maxQueuedBytes. The connection has already been closed
// and the WARN logged; the caller only has to Remove it.
var ErrSendQueueFull = errors.New("send queue over cap; connection closed")

// ErrConnectionClosing is returned by Write once Close has been called.
var ErrConnectionClosing = errors.New("connection is closing")

// sendQueue is a connection's outbound queue and the state of its one writer
// goroutine.
//
// Game code never touches the socket. Write (and so SendTo and Broadcast)
// appends to items and returns; run() is the only goroutine that writes data
// frames, one message at a time in queue order, each bounded by writeTimeout.
// A peer that stops reading therefore stalls only its own writer: before this
// queue the write happened on the game loop and froze every player for up to
// writeTimeout, then dropped the connection anyway.
type sendQueue struct {
	cd *ConnectionDetails

	mu    sync.Mutex
	items [][]byte
	// queuedBytes counts queued messages plus the one being written.
	queuedBytes int
	// idle is closed while nothing is queued or in flight, and replaced with
	// an open channel when work arrives. flush waits on it.
	idle   chan struct{}
	isIdle bool
	// closing: Close was called. The writer drains what is queued, bounded by
	// closeDeadline, then closes the socket.
	closing       bool
	closeDeadline time.Time
	// aborted: the queue went over cap or a write failed. Queued output is
	// dropped and the socket is already closed.
	aborted bool

	wake chan struct{} // capacity 1; a send means "look at the queue"
	done chan struct{} // closed when the writer goroutine has exited
}

func newSendQueue(cd *ConnectionDetails) *sendQueue {
	q := &sendQueue{
		cd:     cd,
		idle:   make(chan struct{}),
		isIdle: true,
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	close(q.idle)
	return q
}

// sendQueue returns the connection's queue, starting its writer on first use.
// Lazy so a ConnectionDetails built as a struct literal (tests, the copyover
// restore path) behaves like one from NewConnectionDetails.
func (cd *ConnectionDetails) sendQueue() *sendQueue {
	cd.outOnce.Do(func() {
		cd.out = newSendQueue(cd)
		go cd.out.run()
	})
	return cd.out
}

func (q *sendQueue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// setIdleLocked marks the queue idle. Caller holds q.mu.
func (q *sendQueue) setIdleLocked() {
	if !q.isIdle {
		q.isIdle = true
		close(q.idle)
	}
}

// enqueue appends p (already prepared; the queue owns it) without blocking.
func (q *sendQueue) enqueue(p []byte) error {
	if outputFrozen.Load() {
		return errOutputFrozen
	}

	q.mu.Lock()

	if q.aborted || q.closing {
		q.mu.Unlock()
		return ErrConnectionClosing
	}

	// A lone message larger than the cap is still sent to an otherwise idle
	// connection: the cap is about a client falling behind, not message size.
	if q.queuedBytes > 0 && q.queuedBytes+len(p) > maxQueuedBytes {
		queued := q.queuedBytes
		q.aborted = true
		q.items = nil
		q.queuedBytes = 0
		q.setIdleLocked()
		q.mu.Unlock()

		mudlog.Warn("SendQueueOverCap",
			"connectionId", q.cd.ConnectionId(),
			"remoteAddr", q.cd.remoteAddrString(),
			"queuedBytes", queued,
			"messageBytes", len(p),
			"capBytes", maxQueuedBytes,
		)
		// Closing the socket also unblocks a write in flight, so the writer
		// exits promptly. net.Conn.Close does not wait on the peer.
		q.cd.closeSocket()
		q.signal()
		return ErrSendQueueFull
	}

	q.items = append(q.items, p)
	q.queuedBytes += len(p)
	if q.isIdle {
		q.isIdle = false
		q.idle = make(chan struct{})
	}
	q.mu.Unlock()

	q.signal()
	return nil
}

// close asks the writer to deliver what is already queued and then close the
// socket. Never blocks. The drain is bounded: every write after close shares
// one deadline writeTimeout from now.
func (q *sendQueue) close() {
	q.mu.Lock()
	if !q.closing {
		q.closing = true
		q.closeDeadline = time.Now().Add(writeTimeout)
	}
	q.mu.Unlock()
	q.signal()
}

// flush waits up to timeout for everything queued so far to be written (or
// dropped by an abort). Reports whether the queue went idle in time.
func (q *sendQueue) flush(timeout time.Duration) bool {
	q.mu.Lock()
	ch := q.idle
	q.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ch:
		return true
	case <-timer.C:
		return false
	}
}

// outputFrozen, once set, makes every enqueue drop its output. Copyover sets
// it in CopyoverSave after the reconnect tokens are queued and before the
// final flush (fd_unix.go). Without it a reader goroutine could queue a prompt
// redraw after the flush, and the writer could be mid-write at execve, leaving
// half an ANSI, IAC or GMCP sequence on a telnet socket the new process then
// keeps writing to. Output queued before the freeze is still delivered.
var outputFrozen atomic.Bool

// errOutputFrozen is enqueue's internal "dropped" result; Write reports it as
// (0, nil) so SendTo and Broadcast do not Remove a connection that is about
// to be handed over.
var errOutputFrozen = errors.New("output frozen for copyover")

func freezeOutput() { outputFrozen.Store(true) }

// ThawOutputAfterFailedCopyover lifts the copyover output freeze. A copyover
// that succeeds replaces the process, so the freeze lasts its lifetime; one
// that fails after CopyoverSave returns to a live server, and without this
// every player would be left with no output at all.
func ThawOutputAfterFailedCopyover() { outputFrozen.Store(false) }

// idleNow reports, without waiting, whether nothing is queued or in flight.
func (q *sendQueue) idleNow() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.isIdle
}

// run is the connection's writer goroutine.
func (q *sendQueue) run() {
	defer close(q.done)
	defer q.finish()

	for {
		q.mu.Lock()
		for len(q.items) == 0 && !q.closing && !q.aborted {
			q.mu.Unlock()
			<-q.wake
			q.mu.Lock()
		}

		if q.aborted {
			// Whoever aborted closed the socket.
			q.items = nil
			q.queuedBytes = 0
			q.setIdleLocked()
			q.mu.Unlock()
			return
		}

		if len(q.items) == 0 {
			// Closing and fully drained.
			q.setIdleLocked()
			q.mu.Unlock()
			q.cd.closeSocket()
			return
		}

		p := q.items[0]
		q.items[0] = nil
		q.items = q.items[1:]

		deadline := time.Now().Add(writeTimeout)
		if q.closing && q.closeDeadline.Before(deadline) {
			deadline = q.closeDeadline
		}
		q.mu.Unlock()

		err := q.cd.writeNow(p, deadline)

		q.mu.Lock()
		q.queuedBytes -= len(p)

		if err != nil {
			wasClosing, wasAborted := q.closing, q.aborted
			q.aborted = true
			q.items = nil
			q.queuedBytes = 0
			q.setIdleLocked()
			q.mu.Unlock()

			q.cd.closeSocket()

			// A failure during a close drain, or after an abort, is expected
			// and already handled. Anything else is the slow or dead client
			// case and goes through the same Remove the game uses, so the
			// player goes linkdead exactly as a failed write always did.
			if !wasClosing && !wasAborted {
				mudlog.Warn("SendTo()",
					"connectionId", q.cd.ConnectionId(),
					"remoteAddr", q.cd.remoteAddrString(),
					"error", err,
				)
				_ = Remove(q.cd.ConnectionId())
			}
			return
		}

		if len(q.items) == 0 {
			q.setIdleLocked()
		}
		q.mu.Unlock()
	}
}

// finish stops the websocket heartbeat and waits for its goroutine, off the
// game loop, so no goroutine outlives the connection.
func (q *sendQueue) finish() {
	if hb := q.cd.heartbeat; hb != nil {
		hb.signalStop()
		hb.wait()
	}
}
