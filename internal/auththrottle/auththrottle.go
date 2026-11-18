// Package auththrottle counts failed logins per source and locks a source out
// after too many. The /admin Basic Auth check and the game login each keep
// their own Tracker, keyed by the client IP, and ask it BEFORE any bcrypt
// work so a lockout also saves the CPU.
package auththrottle

import (
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

const (
	// MaxFailures failures inside FailureWindow lock a source out for
	// LockoutDur.
	MaxFailures   = 5
	FailureWindow = 5 * time.Minute
	LockoutDur    = 1 * time.Minute

	// MaxEntries caps a tracker. Its keys come from attacker-controlled
	// connection data, so it must never grow without bound: entries are
	// swept on every write and, if the cap is still hit, the
	// least-recently-failed entry is evicted to make room.
	MaxEntries = 4096
)

type record struct {
	count       int
	windowStart time.Time
	lastFail    time.Time
	lockedUntil time.Time
}

// Tracker is one independent set of failure counts. It is safe for
// concurrent use.
type Tracker struct {
	name     string
	mu       sync.Mutex
	failures map[string]*record
}

// New returns a tracker whose lockout warnings are logged under name.
func New(name string) *Tracker {
	return &Tracker{name: name, failures: map[string]*record{}}
}

// Throttled reports whether key is locked out at now, and for how long.
func (t *Tracker) Throttled(key string, now time.Time) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.failures[key]
	if !ok {
		return false, 0
	}
	if now.Before(rec.lockedUntil) {
		return true, rec.lockedUntil.Sub(now)
	}
	return false, 0
}

// NoteFailure records a failed attempt and locks key out once it reaches
// MaxFailures inside FailureWindow.
func (t *Tracker) NoteFailure(key string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sweepLocked(now)

	rec, ok := t.failures[key]
	if !ok {
		if len(t.failures) >= MaxEntries {
			t.evictOldestLocked()
		}
		rec = &record{windowStart: now}
		t.failures[key] = rec
	}

	// A quiet period longer than the window starts a fresh count.
	if now.Sub(rec.windowStart) > FailureWindow {
		rec.count = 0
		rec.windowStart = now
	}

	rec.count++
	rec.lastFail = now

	if rec.count >= MaxFailures {
		rec.lockedUntil = now.Add(LockoutDur)
		mudlog.Warn(t.name, "ip", key, "failures", rec.count, "lockedForSeconds", int(LockoutDur.Seconds()))
	}
}

// Clear drops the record for a source that just logged in.
func (t *Tracker) Clear(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.failures, key)
}

// Len is the number of sources tracked.
func (t *Tracker) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.failures)
}

// ResetForTest forgets every source.
func (t *Tracker) ResetForTest() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failures = map[string]*record{}
}

// sweepLocked drops records outside the failure window that are not locked
// out. Caller holds t.mu.
func (t *Tracker) sweepLocked(now time.Time) {
	for key, rec := range t.failures {
		if now.Before(rec.lockedUntil) {
			continue
		}
		if now.Sub(rec.lastFail) > FailureWindow {
			delete(t.failures, key)
		}
	}
}

// evictOldestLocked makes room when the tracker is full. Caller holds t.mu.
func (t *Tracker) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	for key, rec := range t.failures {
		if oldestKey == `` || rec.lastFail.Before(oldest) {
			oldestKey = key
			oldest = rec.lastFail
		}
	}
	if oldestKey != `` {
		delete(t.failures, oldestKey)
	}
}
