package gamelock

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// pinRelockTiming pins the shipped timing (test binaries load Go defaults,
// where an hour is less than a round) and starts the clock past
// RoundCountMinimum, as the game does. It returns the start round.
func pinRelockTiming(t *testing.T) uint64 {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	start := uint64(util.RoundCountMinimum) + 1000
	util.SetRoundCountForTest(start)
	t.Cleanup(util.ResetRoundCountForTest)
	return start
}

// openAndLetRelock opens l at the current round and moves the clock to the
// round its timer relocks it.
func openAndLetRelock(t *testing.T, l *Lock) {
	t.Helper()
	l.SetUnlocked()
	util.SetRoundCountForTest(relockRound(util.GetRoundCount(), l.RelockInterval))
	if !l.IsLocked() {
		t.Fatal("premise: the timer relocked the lock")
	}
}

// Owner ruling 2026-11-25: every relock gets a new combination, the timer's
// included, and a lock still open keeps its rotation.
func TestLock_RotationChangesAtATimerRelock(t *testing.T) {
	pinRelockTiming(t)
	const lockId = `5-north`
	l := Lock{Difficulty: 32, RotationSeed: 3}

	if l.Rotation() != 3 {
		t.Fatalf("a never-opened lock's rotation is %d, want its RotationSeed 3", l.Rotation())
	}
	before := l.Sequence(lockId)

	l.SetUnlocked()
	if l.Rotation() != 3 {
		t.Fatalf("an open lock's rotation is %d, want its RotationSeed 3", l.Rotation())
	}

	util.SetRoundCountForTest(relockRound(util.GetRoundCount(), l.RelockInterval))
	if !l.IsLocked() {
		t.Fatal("premise: the timer relocked the lock")
	}
	first := l.Sequence(lockId)
	if first == before {
		t.Fatal("a timer relock kept the combination the lock had before it opened")
	}
	if l.Rotation()&(1<<63) == 0 {
		t.Fatalf("a timer relock's rotation %d does not have its top bit set, so SetLocked could reach it", l.Rotation())
	}

	// A second open-then-relock cycle gets its own combination. Difficulty
	// 32 makes a chance match one in four billion.
	openAndLetRelock(t, &l)
	second := l.Sequence(lockId)
	if second == first || second == before {
		t.Fatal("a second timer relock repeated an earlier combination")
	}

	// SetLocked after a timer relock still rotates: the seed moves on by one
	// (the seed carries whatever the last opening folded into it).
	seed := l.RotationSeed
	l.SetLocked()
	if l.Rotation() != seed+1 {
		t.Fatalf("SetLocked after a timer relock left rotation %d, want RotationSeed %d", l.Rotation(), seed+1)
	}
	if s := l.Sequence(lockId); s == second || s == first || s == before {
		t.Fatal("SetLocked after a timer relock repeated an earlier combination")
	}
}

// Opening a lock after a timer relock keeps the timer relock's combination:
// SetUnlocked overwrites UnlockedRound, so the rotation is folded into
// RotationSeed first, and the combination from before the relock stays
// retired while the lock is open and after it is locked again by the timer.
func TestLock_OpeningAfterATimerRelockKeepsItsCombination(t *testing.T) {
	pinRelockTiming(t)
	const lockId = `5-north`
	l := Lock{Difficulty: 32}
	original := l.Sequence(lockId)

	openAndLetRelock(t, &l)
	timer := l.Sequence(lockId)
	if timer == original {
		t.Fatal("premise: the timer relock changed the combination")
	}

	l.SetUnlocked()
	if got := l.Sequence(lockId); got != timer {
		t.Fatal("opening the lock after a timer relock fell back to an older combination")
	}
	if l.Sequence(lockId) == original {
		t.Fatal("opening the lock after a timer relock brought back the original combination")
	}
}
