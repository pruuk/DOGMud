package connections

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// These tests pin the outbound send queue: game code enqueues, one writer
// goroutine per connection does the socket write. The property that matters
// is that nothing a game-loop caller does can wait on a socket.

// testLogTee captures every log line so a test can assert a WARN was written.
// Installed by TestMain (write_deadline_test.go).
type testLogTee struct {
	mu    sync.Mutex
	lines []string
}

func (l *testLogTee) Println(level string, v ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, level+" "+ansiRegexp.ReplaceAllString(fmt.Sprint(v...), ``))
}

func (l *testLogTee) contains(subs ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range l.lines {
		all := true
		for _, s := range subs {
			if !strings.Contains(line, s) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

var testLogs = &testLogTee{}

// fastCallLimit is how long one SendTo may take. A blocking socket write
// costs the full writeTimeout (5 s). A queued 256 KiB send takes about 1 ms
// normally, but under -race on a loaded machine one took 92 ms (2026-10-24),
// so the limit is 250 ms: still 20 times under a single blocked write.
const fastCallLimit = 250 * time.Millisecond

// waitUntil polls cond until it holds or limit passes. It waits on a
// condition rather than sleeping a guessed amount.
func waitUntil(t *testing.T, limit time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for: %s", limit, what)
		}
		<-tick.C
	}
}

// addPipeConnection registers a logged-in telnet connection whose client end
// is returned. Nothing reads the client end unless the test starts a reader.
func addPipeConnection(t *testing.T) (ConnectionId, net.Conn) {
	t.Helper()
	serverSide, clientSide := net.Pipe()
	cd := Add(serverSide, nil)
	cd.SetState(LoggedIn)
	id := cd.ConnectionId()
	t.Cleanup(func() {
		_ = Remove(id)
		_ = clientSide.Close()
	})
	return id, clientSide
}

// pipeReader drains a client end into a buffer.
type pipeReader struct {
	mu  sync.Mutex
	buf bytes.Buffer
	eof bool
}

func startPipeReader(c net.Conn) *pipeReader {
	r := &pipeReader{}
	go func() {
		chunk := make([]byte, 64*1024)
		for {
			n, err := c.Read(chunk)
			r.mu.Lock()
			r.buf.Write(chunk[:n])
			if err != nil {
				r.eof = true
			}
			r.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return r
}

func (r *pipeReader) text() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

func (r *pipeReader) closed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.eof
}

// bigMessage is a size-byte message whose first bytes are a sequence header.
func bigMessage(i, size int) []byte {
	head := fmt.Sprintf("<MSG%05d>", i)
	return []byte(head + strings.Repeat("x", size-len(head)))
}

// headersInOrder reports how many <MSGnnnnn> headers appear in text, and
// whether they appear as 0, 1, 2, ... with none missing or reordered.
func headersInOrder(text string) (int, bool) {
	count := 0
	pos := 0
	for {
		i := strings.Index(text[pos:], "<MSG")
		if i < 0 {
			return count, true
		}
		pos += i
		if pos+10 > len(text) {
			return count, true
		}
		want := fmt.Sprintf("<MSG%05d>", count)
		if text[pos:pos+10] != want {
			return count, false
		}
		count++
		pos += 10
	}
}

// A peer that never reads must not hold up the caller. Before the queue,
// the first SendTo blocked for the whole writeTimeout.
func TestSendQueue_SendToReturnsQuicklyWhenPeerNeverReads(t *testing.T) {
	id, _ := addPipeConnection(t)

	const msgSize = 256 * 1024
	for i := 0; i < 20; i++ { // 5 MiB: under the cap, so the connection stays
		msg := bigMessage(i, msgSize)
		var elapsed time.Duration
		_ = waitFor(t, writeTimeout+5*time.Second, "SendTo", func() error {
			start := time.Now()
			SendTo(msg, id)
			elapsed = time.Since(start)
			return nil
		})
		if elapsed > fastCallLimit {
			t.Fatalf("SendTo #%d to an unread peer took %s (limit %s): the caller waited on the socket", i, elapsed, fastCallLimit)
		}
	}

	if Get(id) == nil {
		t.Fatal("a connection under the queue cap was closed")
	}
}

// A slow peer is closed once its queue passes the cap, with a WARN naming it.
// A healthy connection fed by the same SendTo calls gets every message, in
// order, and is never held up.
func TestSendQueue_SlowConnectionClosedAtCapHealthyKeepsReceiving(t *testing.T) {
	slowId, _ := addPipeConnection(t)
	healthyId, healthyClient := addPipeConnection(t)
	healthy := startPipeReader(healthyClient)

	const msgSize = 256 * 1024
	const total = 40 // 10 MiB: past the 8 MiB cap for the slow peer
	for i := 0; i < total; i++ {
		msg := bigMessage(i, msgSize)
		var elapsed time.Duration
		_ = waitFor(t, writeTimeout+5*time.Second, "SendTo", func() error {
			start := time.Now()
			SendTo(msg, slowId, healthyId)
			elapsed = time.Since(start)
			return nil
		})
		if elapsed > fastCallLimit {
			t.Fatalf("SendTo #%d took %s (limit %s)", i, elapsed, fastCallLimit)
		}
	}

	if Get(slowId) != nil {
		t.Fatalf("slow connection %d is still registered after %d MiB queued", slowId, total*msgSize>>20)
	}
	if !testLogs.contains("WARN", "SendQueueOverCap", fmt.Sprintf("connectionId=%d ", slowId), "queuedBytes=") {
		t.Fatal("no SendQueueOverCap WARN naming the slow connection was logged")
	}

	waitUntil(t, 10*time.Second, "healthy connection receives all messages", func() bool {
		n, _ := headersInOrder(healthy.text())
		return n == total
	})
	if n, ok := headersInOrder(healthy.text()); !ok || n != total {
		t.Fatalf("healthy connection got %d messages, in order=%v; want %d in order", n, ok, total)
	}
	if Get(healthyId) == nil {
		t.Fatal("healthy connection was closed")
	}
}

// N messages to one telnet connection arrive in the order they were sent.
func TestSendQueue_TelnetOrderPreserved(t *testing.T) {
	id, client := addPipeConnection(t)
	r := startPipeReader(client)

	const total = 2000
	for i := 0; i < total; i++ {
		SendTo([]byte(fmt.Sprintf("<MSG%05d>", i)), id)
	}

	waitUntil(t, 10*time.Second, "all telnet messages arrive", func() bool {
		n, _ := headersInOrder(r.text())
		return n == total
	})
	if n, ok := headersInOrder(r.text()); !ok || n != total {
		t.Fatalf("got %d messages, in order=%v; want %d in order", n, ok, total)
	}
}

// N messages to one websocket connection arrive in the order they were sent,
// one websocket frame per message.
func TestSendQueue_WebsocketOrderPreserved(t *testing.T) {
	upgrader := websocket.Upgrader{}
	serverConn := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConn <- c
	}))
	defer srv.Close()

	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	var wsConn *websocket.Conn
	select {
	case wsConn = <-serverConn:
	case <-time.After(5 * time.Second):
		t.Fatal("server never saw the websocket upgrade")
	}

	cd := Add(nil, wsConn)
	cd.SetState(LoggedIn)
	id := cd.ConnectionId()
	t.Cleanup(func() { _ = Remove(id) })

	const total = 500
	for i := 0; i < total; i++ {
		SendTo([]byte(fmt.Sprintf("<MSG%05d>", i)), id)
	}

	_ = client.SetReadDeadline(time.Now().Add(10 * time.Second))
	for i := 0; i < total; i++ {
		_, msg, err := client.ReadMessage()
		if err != nil {
			t.Fatalf("read message %d: %v", i, err)
		}
		if want := fmt.Sprintf("<MSG%05d>", i); string(msg) != want {
			t.Fatalf("message %d = %q, want %q", i, msg, want)
		}
	}
}

// The writer keeps the per-write deadline: a write the peer never takes
// closes the connection through Remove, with a WARN, after writeTimeout.
func TestSendQueue_WriterDeadlineClosesStalledConnection(t *testing.T) {
	id, _ := addPipeConnection(t)

	start := time.Now()
	SendTo([]byte("nobody reads this"), id)

	waitUntil(t, writeTimeout+5*time.Second, "stalled connection is removed", func() bool {
		return Get(id) == nil
	})
	if elapsed := time.Since(start); elapsed < writeTimeout/2 {
		t.Fatalf("removed after only %s: the write deadline does not look applied", elapsed)
	}
	if !testLogs.contains("WARN", "i/o timeout") {
		t.Fatal("no WARN with the write timeout error was logged")
	}
}

// Remove delivers output queued before it (a goodbye line), then closes.
func TestSendQueue_RemoveDeliversQueuedOutputThenCloses(t *testing.T) {
	id, client := addPipeConnection(t)
	r := startPipeReader(client)

	SendTo([]byte("first line"), id)
	SendTo([]byte("goodbye"), id)
	if err := Remove(id); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	waitUntil(t, writeTimeout+5*time.Second, "peer sees goodbye then EOF", func() bool {
		return r.closed()
	})
	if got := r.text(); got != "first linegoodbye" {
		t.Fatalf("peer received %q, want %q", got, "first linegoodbye")
	}
}

// Remove unblocks the connection's reader at once, even while queued output
// is still draining to a slow peer, so a kicked player's input stops.
func TestSendQueue_RemoveUnblocksReaderWhileDraining(t *testing.T) {
	id, _ := addPipeConnection(t)
	cd := Get(id)

	SendTo([]byte("the peer never reads this"), id)

	readErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 64)
		_, err := cd.Read(buf)
		readErr <- err
	}()

	if err := Remove(id); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	select {
	case err := <-readErr:
		if err == nil {
			t.Fatal("Read returned no error after Remove")
		}
	case <-time.After(time.Second):
		t.Fatal("Read still blocked one second after Remove")
	}
}

// Cleanup (shutdown) waits for each writer to finish, so no writer goroutine
// outlives it, and output queued before it is delivered.
func TestSendQueue_CleanupFlushesAndStopsWriters(t *testing.T) {
	id, client := addPipeConnection(t)
	r := startPipeReader(client)
	cd := Get(id)

	SendTo([]byte("server going down"), id)

	_ = waitFor(t, writeTimeout+5*time.Second, "Cleanup", func() error {
		Cleanup()
		return nil
	})

	select {
	case <-cd.sendQueue().done:
	default:
		t.Fatal("writer goroutine still running after Cleanup returned")
	}
	waitUntil(t, 2*time.Second, "peer sees the shutdown line", func() bool {
		return strings.Contains(r.text(), "server going down")
	})
}

// flush reports true once queued output is written, false on timeout.
func TestSendQueue_FlushWaitsForQueuedOutput(t *testing.T) {
	id, client := addPipeConnection(t)
	cd := Get(id)

	SendTo([]byte("held by a peer that is not reading yet"), id)
	if cd.sendQueue().flush(100 * time.Millisecond) {
		t.Fatal("flush reported drained while the peer had read nothing")
	}

	startPipeReader(client)
	if !cd.sendQueue().flush(5 * time.Second) {
		t.Fatal("flush did not report drained after the peer started reading")
	}
}
