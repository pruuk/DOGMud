package auththrottle

import (
	"os"
	"strconv"
	"sync"
	"sync/atomic"
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

// A burst of parallel attempts from one source lets exactly MaxFailures
// through: the check and the count are one step.
func TestAttemptIsAtomic(t *testing.T) {
	tr := New("test")
	now := time.Now()
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for range 200 {
		wg.Go(func() {
			if tr.Attempt("198.51.100.1", now) {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()
	if got := allowed.Load(); got != MaxFailures {
		t.Fatalf("%d parallel attempts got through, want %d", got, MaxFailures)
	}
}

// Refund gives back one attempt but never lifts a lockout in force.
func TestRefundKeepsALockout(t *testing.T) {
	tr := New("test")
	now := time.Now()
	for range MaxFailures {
		tr.Attempt("a", now)
	}
	if tr.Attempt("a", now) {
		t.Fatal("the attempt past the limit was allowed")
	}
	tr.Refund("a")
	if tr.Attempt("a", now) {
		t.Fatal("a refund lifted the lockout")
	}
}

// IPv6 addresses in one /64 share a count; IPv4 addresses do not.
func TestSourceKeyGroupsIPv6By64(t *testing.T) {
	if SourceKey("2001:db8:1:2::1") != SourceKey("2001:db8:1:2:ffff::9") {
		t.Fatal("two addresses in one /64 got different keys")
	}
	if SourceKey("2001:db8:1:2::1") == SourceKey("2001:db8:1:3::1") {
		t.Fatal("two /64s got the same key")
	}
	if SourceKey("198.51.100.1") == SourceKey("198.51.100.2") {
		t.Fatal("two IPv4 addresses got the same key")
	}
	tr := New("test")
	now := time.Now()
	for i := range MaxFailures {
		tr.Attempt("2001:db8:1:2::"+strconv.Itoa(i+1), now)
	}
	if tr.Attempt("2001:db8:1:2::99", now) {
		t.Fatal("rotating addresses inside one /64 got past the limit")
	}
}

// A flood of new sources filling the tracker never evicts a locked-out one.
func TestEvictionKeepsLockouts(t *testing.T) {
	tr := New("test")
	now := time.Now()
	for range MaxFailures + 1 { // the last one is refused and locks
		tr.Attempt("locked", now)
	}
	later := now.Add(time.Second)
	for i := range MaxEntries + 100 {
		tr.NoteFailure("10.1."+strconv.Itoa(i/256)+"."+strconv.Itoa(i%256), later)
	}
	if locked, _ := tr.Throttled("locked", later); !locked {
		t.Fatal("a flood of new sources lifted a lockout")
	}
}
