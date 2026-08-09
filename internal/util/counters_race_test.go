package util

import (
	"sync"
	"testing"
)

// Finding 4: roundCount and turnCount were plain package-level uint64s
// incremented by the world loop while asynchronous consumers read them with no
// synchronization. internal/llm/cache.go drives cache expiry off
// GetRoundCount from its own goroutines, so a stale or torn read could expire
// an entry early, late, or never.
//
// These run under -race in CI (`go test -race ./...`), which is where a
// regression to a plain uint64 would be caught. They also assert the counting
// itself stays exact under contention.

func TestRoundCount_ConcurrentIncrementIsExact(t *testing.T) {
	ResetRoundCountForTest()
	start := GetRoundCount()

	const goroutines = 16
	const perGoroutine = 500

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				IncrementRoundCount()
			}
		}()
	}
	wg.Wait()

	want := start + goroutines*perGoroutine
	if got := GetRoundCount(); got != want {
		t.Errorf("round count = %d, want %d; increments were lost to a race", got, want)
	}

	ResetRoundCountForTest()
}

func TestTurnCount_ConcurrentIncrementIsExact(t *testing.T) {
	turnCount.Store(0)

	const goroutines = 16
	const perGoroutine = 500

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				IncrementTurnCount()
			}
		}()
	}
	wg.Wait()

	if got, want := GetTurnCount(), uint64(goroutines*perGoroutine); got != want {
		t.Errorf("turn count = %d, want %d; increments were lost to a race", got, want)
	}

	turnCount.Store(0)
}

// The real shape: the world loop increments while async consumers read, which
// is exactly what internal/llm does. Readers must never observe a value that
// goes backwards.
func TestRoundCount_ConcurrentReadersSeeMonotonicValues(t *testing.T) {
	ResetRoundCountForTest()

	done := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() { // the "world loop"
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			IncrementRoundCount()
		}
		close(done)
	}()

	for r := 0; r < 8; r++ { // async consumers
		wg.Add(1)
		go func() {
			defer wg.Done()
			last := uint64(0)
			for {
				select {
				case <-done:
					return
				default:
					cur := GetRoundCount()
					if cur < last {
						t.Errorf("round count went backwards: %d after %d", cur, last)
						return
					}
					last = cur
				}
			}
		}()
	}

	wg.Wait()
	ResetRoundCountForTest()
}

// Save/Load and the copyover round-trip must still work through the atomics.
func TestRoundCount_SetAndResetRoundTrip(t *testing.T) {
	ResetRoundCountForTest()
	if got := GetRoundCount(); got != RoundCountMinimum {
		t.Errorf("after reset = %d, want RoundCountMinimum (%d)", got, RoundCountMinimum)
	}

	SetRoundCount(RoundCountMinimum + 9999)
	if got := GetRoundCount(); got != RoundCountMinimum+9999 {
		t.Errorf("after SetRoundCount = %d, want %d", got, RoundCountMinimum+9999)
	}

	SetRoundCountForTest(RoundCountMinimum + 5)
	if got := GetRoundCount(); got != RoundCountMinimum+5 {
		t.Errorf("after SetRoundCountForTest = %d, want %d", got, RoundCountMinimum+5)
	}

	ResetRoundCountForTest()
}
