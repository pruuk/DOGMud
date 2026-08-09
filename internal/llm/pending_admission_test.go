package llm

import (
	"sync"
	"sync/atomic"
	"testing"
)

// Finding 21: admission used a separately-locked isPending() check followed
// later by setPending(true). Each call locked correctly, but the gap between
// them did not, so two goroutines could both observe "not pending" and both
// proceed. That meant duplicate model requests and duplicate callbacks
// mutating dialogue state for the same mob.

// Exactly one caller may hold the slot at a time.
func TestTryMarkPending_OnlyOneWinnerPerMob(t *testing.T) {
	const mobId = 1234
	clearPending(mobId)

	const goroutines = 64
	var winners atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // maximize the overlap
			if tryMarkPending(mobId) {
				winners.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := winners.Load(); got != 1 {
		t.Errorf("winners = %d, want exactly 1; concurrent requests for one mob both got admitted", got)
	}

	clearPending(mobId)
}

// Releasing the slot lets the next caller in.
func TestTryMarkPending_ClearReleasesSlot(t *testing.T) {
	const mobId = 5678
	clearPending(mobId)

	if !tryMarkPending(mobId) {
		t.Fatal("first claim should succeed")
	}
	if tryMarkPending(mobId) {
		t.Error("second claim should fail while the slot is held")
	}

	clearPending(mobId)

	if !tryMarkPending(mobId) {
		t.Error("claim should succeed again after clearPending")
	}
	clearPending(mobId)
}

// Different mobs are independent; one mob's in-flight request must not block
// another mob's.
func TestTryMarkPending_DistinctMobsIndependent(t *testing.T) {
	const mobA, mobB = 111, 222
	clearPending(mobA)
	clearPending(mobB)

	if !tryMarkPending(mobA) {
		t.Fatal("mobA claim should succeed")
	}
	if !tryMarkPending(mobB) {
		t.Error("mobB claim should succeed while mobA is pending")
	}

	clearPending(mobA)
	clearPending(mobB)
}

// Sustained churn across many mobs: never more than one winner per mob per
// round, and no concurrent map access fault on the pending map.
func TestTryMarkPending_ConcurrentChurnAcrossMobs(t *testing.T) {
	const mobs = 16
	const rounds = 50

	for r := 0; r < rounds; r++ {
		var winners [mobs]atomic.Int32
		var wg sync.WaitGroup
		start := make(chan struct{})

		for g := 0; g < mobs*4; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				mobId := g % mobs
				<-start
				if tryMarkPending(mobId) {
					winners[mobId].Add(1)
				}
			}(g)
		}
		close(start)
		wg.Wait()

		for m := 0; m < mobs; m++ {
			if got := winners[m].Load(); got != 1 {
				t.Fatalf("round %d mob %d: winners = %d, want 1", r, m, got)
			}
			clearPending(m)
		}
	}
}
