package main

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestAcceptDecision pins the accept loop's per-connection decision: shutdown
// ends the loop, a copyover pause turns the connection away but keeps the loop
// running, and otherwise the connection is admitted (#670).
func TestAcceptDecision(t *testing.T) {
	cases := []struct {
		name              string
		alive, paused     bool
		wantExit, wantRej bool
	}{
		{"normal accepts", true, false, false, false},
		{"copyover pause rejects and continues", true, true, false, true},
		{"shutdown exits", false, false, true, false},
		{"shutdown during a pause still exits", false, true, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exit, reject := acceptDecision(tc.alive, tc.paused)
			if exit != tc.wantExit || reject != tc.wantRej {
				t.Fatalf("acceptDecision(alive=%v, paused=%v) = (exit=%v, reject=%v), want (exit=%v, reject=%v)",
					tc.alive, tc.paused, exit, reject, tc.wantExit, tc.wantRej)
			}
		})
	}
}

// TestTelnetAcceptLoop_SurvivesFailedCopyover runs the real accept loop on a
// loopback listener. A connection that arrives during a copyover pause must be
// closed, and once the pause ends (as a failed copyover ends it) the loop must
// still accept. Before #670 that connection ended the loop for good, so no new
// telnet player could connect until a restart.
func TestTelnetAcceptLoop_SurvivesFailedCopyover(t *testing.T) {
	// The loop logs, and mudlog has no logger until one is set up.
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	prevAlive := serverAlive.Load()
	serverAlive.Store(true)
	t.Cleanup(func() {
		resumeAccepting()
		serverAlive.Store(prevAlive)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	admitted := make(chan net.Conn, 4)
	done := make(chan struct{})
	go func() {
		telnetAcceptLoop(ln, func(c net.Conn) { admitted <- c })
		close(done)
	}()

	dial := func() net.Conn {
		t.Helper()
		c, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return c
	}

	// 1. A copyover pauses accepting and a player connects mid-copyover.
	pauseAccepting()
	during := dial()
	defer during.Close()

	// The server tells the player why and closes that connection rather
	// than leaving it hanging.
	during.SetReadDeadline(time.Now().Add(2 * time.Second))
	got, err := io.ReadAll(during)
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			t.Error("connection made during the copyover pause was left open (read timed out)")
		}
	} else if string(got) != copyoverRejectMessage {
		t.Errorf("connection made during the copyover pause read %q, want %q", got, copyoverRejectMessage)
	}
	select {
	case <-admitted:
		t.Error("connection made during the copyover pause was admitted")
	default:
	}

	// 2. The copyover fails and accepting resumes.
	resumeAccepting()

	// 3. A new player can connect.
	after := dial()
	defer after.Close()
	select {
	case c := <-admitted:
		c.Close()
	case <-done:
		t.Fatal("accept loop exited after a failed copyover; no new connection can be accepted")
	case <-time.After(2 * time.Second):
		t.Fatal("connection after a failed copyover was not accepted")
	}

	// 4. Shutdown still ends the loop.
	serverAlive.Store(false)
	ln.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("accept loop did not exit on shutdown")
	}
}
