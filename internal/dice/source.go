package dice

import (
	"math/rand"
	"sync"
	"sync/atomic"
)

// The random source every draw in the game comes from (#576, #524).
//
// Production draws from the math/rand top-level functions, exactly as dice
// always has: the hot path adds one atomic load of a nil pointer and nothing
// else, so the distribution and the lock-free global generator are unchanged.
//
// A test can swap in a seeded generator with SetRandForTest. util.Rand draws
// through this same seam, so one seed pins every Normal roll, contest floor,
// percentile and util.Rand pick a run makes, in the order the run makes them.
// The seeded generator sits behind a mutex, so a test whose draws happen on
// several goroutines is still race-free (though only a single-goroutine test
// replays the same sequence every time).

type seededRand struct {
	mu sync.Mutex
	r  *rand.Rand
}

var source atomic.Pointer[seededRand]

// SetRandForTest makes every dice draw (and every util.Rand draw) come from a
// generator seeded with seed, until the returned restore func runs. Pair it
// with t.Cleanup(restore) or defer restore().
func SetRandForTest(seed int64) func() {
	prev := source.Swap(&seededRand{r: rand.New(rand.NewSource(seed))})
	return func() { source.Store(prev) }
}

func normFloat64() float64 {
	if s := source.Load(); s != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.r.NormFloat64()
	}
	return rand.NormFloat64()
}

// Float64 returns a uniform draw in [0, 1) from the dice source. Use it for
// a plain chance check (`dice.Float64() < p`) so the draw is seedable.
func Float64() float64 {
	if s := source.Load(); s != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.r.Float64()
	}
	return rand.Float64()
}

// Intn returns a uniform int in [0, n) from the dice source. It panics if
// n <= 0, as math/rand.Intn does.
func Intn(n int) int {
	if s := source.Load(); s != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.r.Intn(n)
	}
	return rand.Intn(n)
}
