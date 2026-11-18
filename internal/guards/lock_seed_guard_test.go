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
func TestLockSequencesUseTheBootSeed(t *testing.T) {
	calls, files := walkGoCalls(t, map[string]bool{
		"GetLockSequence": true,
		"LockSequence":    true,
	})
	viaBootSeed := 0
	var problems []string
	for _, c := range calls {
		switch {
		case c.name == "LockSequence":
			viaBootSeed++
		case c.file == "internal/util/lockseed.go":
			// LockSequence itself.
		default:
			problems = append(problems, fmt.Sprintf("%s:%d calls GetLockSequence; use util.LockSequence", c.file, c.line))
		}
	}
	if files < 100 || viaBootSeed < 5 {
		t.Fatalf("scanned %d files and found %d LockSequence calls; the walk is not reaching the tree, so a clean result proves nothing", files, viaBootSeed)
	}
	for _, p := range problems {
		t.Error(p)
	}
}
