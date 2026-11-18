// Package auththrottle counts login attempts per source and locks a source
// out after too many failures. The /admin Basic Auth check and the game login
// each keep their own Tracker, keyed by the client IP.
//
// Call Attempt BEFORE any password work: it refuses a locked-out source and
// counts the attempt in one step, so a burst of parallel connections cannot
// all pass a check before any failure is counted. A login that succeeds
// gives back only its own count (Refund); it never wipes the source's other
// failures, or an attacker could reset the count by logging into an account
// of their own between guesses.
package auththrottle

import (
	"net"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

const (
	// MaxFailures is how many failed attempts inside FailureWindow lock a
	// source out.
	MaxFailures = 5
	// FailureWindow is how far back attempts count; a quieter source starts
	// a fresh count.
	FailureWindow = 5 * time.Minute
	// LockoutDur is how long a locked-out source is refused.
	LockoutDur = 1 * time.Minute

	// MaxEntries caps a tracker. Its keys come from attacker-controlled
	// connection data, so it must never grow without bound: entries are
	// swept on every write and, if the cap is still hit, the
	// least-recently-failed entry that is not locked out is evicted.
	MaxEntries = 4096
)

// record is one source's counted attempts in the current window.
type record struct {
	count       int
	windowStart time.Time
	lastFail    time.Time
	lockedUntil time.Time
}

// Tracker is one independent set of counts. It is safe for concurrent use.
type Tracker struct {
	name     string
	mu       sync.Mutex
	failures map[string]*record
}

// New returns a tracker whose lockout warnings are logged under name.
func New(name string) *Tracker {
	return &Tracker{name: name, failures: map[string]*record{}}
}

// SourceKey is the key for a client address: an IPv4 address as is, an IPv6
// address reduced to its /64 (one host usually holds a whole /64, so a key
// per address would let it rotate past the limit), anything else as given.
func SourceKey(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() != nil {
		return ip
	}
	return parsed.Mask(net.CIDRMask(64, 128)).String() + `/64`
}

// Throttled reports whether the source at ip is locked out at now, and for
// how long. It counts nothing.
func (t *Tracker) Throttled(ip string, now time.Time) (bool, time.Duration) {
	key := SourceKey(ip)
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

// Attempt refuses a source (false) or counts one attempt for it (true),
// atomically. A source may have MaxFailures attempts in FailureWindow that
// no success gave back; the next one is refused and locks it out for
// LockoutDur. A success must call Refund (or Clear) to give its count back,
// so a good login is never what trips the lock.
func (t *Tracker) Attempt(ip string, now time.Time) bool {
	key := SourceKey(ip)
	t.mu.Lock()
	defer t.mu.Unlock()

	if rec, ok := t.failures[key]; ok {
		if now.Before(rec.lockedUntil) {
			return false
		}
		if now.Sub(rec.windowStart) <= FailureWindow && rec.count >= MaxFailures {
			rec.lockedUntil = now.Add(LockoutDur)
			mudlog.Warn(t.name, "source", key, "failures", rec.count, "lockedForSeconds", int(LockoutDur.Seconds()))
			return false
		}
	}
	t.countLocked(key, now, false)
	return true
}

// NoteFailure counts a known failure that Attempt did not count, and locks
// the source out on reaching MaxFailures.
func (t *Tracker) NoteFailure(ip string, now time.Time) {
	key := SourceKey(ip)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.countLocked(key, now, true)
}

// Refund gives back the count of one attempt that succeeded. It never
// lifts a lockout already in force.
func (t *Tracker) Refund(ip string) {
	key := SourceKey(ip)
	t.mu.Lock()
	defer t.mu.Unlock()
	if rec, ok := t.failures[key]; ok && rec.count > 0 {
		rec.count--
	}
}

// Clear forgets a source entirely. Only the /admin login uses it, where a
// success needs an admin's own credentials.
func (t *Tracker) Clear(ip string) {
	key := SourceKey(ip)
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

// countLocked counts one attempt for key. With lockAtMax it also locks the
// key out on reaching MaxFailures (a known failure); Attempt locks on the
// attempt after instead, so a success can give its count back first.
// Caller holds t.mu.
func (t *Tracker) countLocked(key string, now time.Time, lockAtMax bool) {
	t.sweepLocked(now)

	rec, ok := t.failures[key]
	if !ok {
		if len(t.failures) >= MaxEntries {
			t.evictOldestLocked(now)
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

	if lockAtMax && rec.count >= MaxFailures {
		rec.lockedUntil = now.Add(LockoutDur)
		mudlog.Warn(t.name, "source", key, "failures", rec.count, "lockedForSeconds", int(LockoutDur.Seconds()))
	}
}

// sweepLocked drops records outside the window that are not locked out.
// Caller holds t.mu.
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

// evictOldestLocked makes room when the tracker is full: the
// least-recently-failed record that is not locked out, so a flood of new
// sources can never lift a lockout. If every record is locked out, the
// oldest goes. Caller holds t.mu.
func (t *Tracker) evictOldestLocked(now time.Time) {
	var oldestKey, oldestLockedKey string
	var oldest, oldestLocked time.Time
	for key, rec := range t.failures {
		if now.Before(rec.lockedUntil) {
			if oldestLockedKey == `` || rec.lastFail.Before(oldestLocked) {
				oldestLockedKey, oldestLocked = key, rec.lastFail
			}
			continue
		}
		if oldestKey == `` || rec.lastFail.Before(oldest) {
			oldestKey, oldest = key, rec.lastFail
		}
	}
	if oldestKey == `` {
		oldestKey = oldestLockedKey
	}
	if oldestKey != `` {
		delete(t.failures, oldestKey)
	}
}
