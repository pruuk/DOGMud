package auththrottle

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestMain sets up the logger: a lockout logs a warning.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, ``, ``, false)
	os.Exit(m.Run())
}

// MaxFailures failures lock the source out; another source is unaffected;
// the lockout ends after LockoutDur.
func TestLocksOutAfterMaxFailures(t *testing.T) {
	tr := New("test")
	now := time.Now()
	for i := range MaxFailures - 1 {
		tr.NoteFailure("a", now)
		if locked, _ := tr.Throttled("a", now); locked {
			t.Fatalf("locked after %d failures, want %d", i+1, MaxFailures)
		}
	}
	tr.NoteFailure("a", now)
	if locked, _ := tr.Throttled("a", now); !locked {
		t.Fatalf("not locked after %d failures", MaxFailures)
	}
	if locked, _ := tr.Throttled("b", now); locked {
		t.Fatal("another source is locked")
	}
	if locked, _ := tr.Throttled("a", now.Add(LockoutDur+time.Second)); locked {
		t.Fatal("still locked after the lockout ended")
	}
}

// Clear forgets a source's failures (a successful login).
func TestClearForgets(t *testing.T) {
	tr := New("test")
	now := time.Now()
	for range MaxFailures - 1 {
		tr.NoteFailure("a", now)
	}
	tr.Clear("a")
	tr.NoteFailure("a", now)
	if locked, _ := tr.Throttled("a", now); locked {
		t.Fatal("Clear did not reset the count")
	}
}

// The tracker is keyed by attacker-controlled connection data, so it must be
// bounded rather than growing until the process dies.
func TestTrackerIsBounded(t *testing.T) {
	tr := New("test")
	now := time.Now()
	for i := range MaxEntries + 500 {
		tr.NoteFailure("10.0."+strconv.Itoa(i/256)+"."+strconv.Itoa(i%256), now)
	}
	if got := tr.Len(); got > MaxEntries {
		t.Fatalf("tracker grew to %d entries, cap is %d", got, MaxEntries)
	}
}
