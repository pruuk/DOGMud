package guards

import (
	"fmt"
	"testing"
)

// TestLockSequencesUseTheBootSeed: every lock sequence comes from
// util.LockSequence, which uses a secret seed made at every boot (owner
// ruling 2026-11-18: a lock solution is good only until a restart).
// GetLockSequence takes its seed as an argument, and the old callers passed
// Server.Seed, which ships in the public config; anyone could then work
// every lock out offline. Only util.LockSequence may call it.
//
// And every lock's combination comes from gamelock.Lock.Sequence (owner ruling
// 2026-11-25: every relock gets a new combination). It builds the sequence at
// Lock.Rotation, which covers a relock on the timer as well as SetLocked; a
// caller passing RotationSeed itself would miss the timer relock. So
// util.LockSequence is called only there, and by lockseed.go's own test
// helper.
func TestLockSequencesUseTheBootSeed(t *testing.T) {
	calls, files := walkGoCalls(t, map[string]bool{
		"GetLockSequence": true,
		"LockSequence":    true,
	})
	viaLock := 0
	var problems []string
	for _, c := range calls {
		switch {
		case c.name == "LockSequence" && c.file == "internal/gamelock/gamelock.go":
			viaLock++
		case c.file == "internal/util/lockseed.go":
			// LockSequence itself, and RebootLocksUntilRotatedForTest.
		case c.name == "LockSequence":
			problems = append(problems, fmt.Sprintf("%s:%d calls util.LockSequence; use the lock's Sequence (gamelock.Lock.Sequence)", c.file, c.line))
		default:
			problems = append(problems, fmt.Sprintf("%s:%d calls GetLockSequence; use the lock's Sequence (gamelock.Lock.Sequence)", c.file, c.line))
		}
	}
	if files < 100 || viaLock < 1 {
		t.Fatalf("scanned %d files and found %d LockSequence calls in gamelock; the walk is not reaching the tree, so a clean result proves nothing", files, viaLock)
	}
	for _, p := range problems {
		t.Error(p)
	}
}
