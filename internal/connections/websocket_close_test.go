package connections

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// pipeListener is a net.Listener whose connections are net.Pipe pairs. A pipe
// has no buffer at all, so a write blocks until the peer reads: the test
// decides exactly when the client takes each byte, and a loopback TCP buffer
// cannot swallow the race under test (Windows loopback absorbed 32 MiB in one
// earlier attempt; see write_deadline_test.go).
type pipeListener struct {
	conns     chan net.Conn
	closeOnce sync.Once
	closed    chan struct{}
}

func newPipeListener() *pipeListener {
	return &pipeListener{conns: make(chan net.Conn), closed: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return nil
}

func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

func (l *pipeListener) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	serverSide, clientSide := net.Pipe()
	select {
	case l.conns <- serverSide:
		return clientSide, nil
	case <-l.closed:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// productionShapedWSHandler mirrors the production websocket path:
// internal/web/web.go's /ws handler (Upgrade, then `defer conn.Close()`, then
// the game's handler) around main.go's HandleWebSocketConnection (connections.Add,
// a reader looping on conn.ReadMessage, and a deferred connections.Remove).
func productionShapedWSHandler(t *testing.T, ids chan<- ConnectionId) http.HandlerFunc {
	upgrader := websocket.Upgrader{}
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close() // web.go

		cd := Add(nil, conn)
		cd.SetState(LoggedIn)
		defer func() { // main.go HandleWebSocketConnection
			_ = Remove(cd.ConnectionId())
			if !cd.WaitWriter(WriterDrainTimeout) {
				t.Errorf("writer still running %s after Remove", WriterDrainTimeout)
			}
		}()

		ids <- cd.ConnectionId()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}
}

// Output queued before a Kick reaches a websocket client before the socket
// closes. The game queues a goodbye and Kicks; Close unblocks the reader, the
// handler returns, and before 2026-10-24 web.go's deferred conn.Close shut the
// socket while the writer was still draining, so the goodbye was lost (and
// silently, because a write failure during a close drain is expected).
func TestSendQueue_WebsocketKickDeliversQueuedGoodbye(t *testing.T) {
	pl := newPipeListener()
	ids := make(chan ConnectionId, 1)
	srv := httptest.NewUnstartedServer(productionShapedWSHandler(t, ids))
	srv.Listener = pl
	srv.Start()
	defer srv.Close()

	dialer := websocket.Dialer{NetDialContext: pl.dial, HandshakeTimeout: 5 * time.Second}
	client, _, err := dialer.Dial("ws://pipe/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	var id ConnectionId
	select {
	case id = <-ids:
	case <-time.After(5 * time.Second):
		t.Fatal("server never registered the websocket")
	}

	big := bigMessage(0, 256*1024)
	SendTo(big, id)
	SendTo([]byte("goodbye"), id)
	if err := Kick(id, "test"); err != nil {
		t.Fatalf("Kick: %v", err)
	}

	// The client is slow: it starts reading only after the server side has
	// had ample time to run its exit path.
	time.Sleep(200 * time.Millisecond)

	_ = client.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msg, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("first message: %v (the socket closed before the queued output was written)", err)
	}
	if !strings.HasPrefix(string(msg), "<MSG00000>") || len(msg) != len(big) {
		t.Fatalf("first message: %d bytes starting %q, want the %d-byte big message", len(msg), msg[:min(len(msg), 10)], len(big))
	}
	_, msg, err = client.ReadMessage()
	if err != nil {
		t.Fatalf("goodbye: %v (the socket closed before the goodbye was written)", err)
	}
	if string(msg) != "goodbye" {
		t.Fatalf("second message = %q, want %q", msg, "goodbye")
	}
	if _, _, err := client.ReadMessage(); err == nil {
		t.Fatal("socket still open after the goodbye")
	} else if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		t.Fatalf("socket not closed after the goodbye: %v", err)
	}
}
