package configs

import (
	"math/rand"
	"testing"
)

// The weighted pick must bias AWAY from difficulty without excluding anything.
// Exclusion is the skill gate's job; this only shades what surfaces first.
//
// ⚠️ A test binary does not load config.yaml, so this runs at the Go DEFAULT
// DiscoveryDifficultyWeightScale of 0.02.
func TestWeightedDiscoveryPick_BiasesAwayFromDifficulty(t *testing.T) {
	// Two candidates: trivial and difficulty 50. At scale 0.02 their weights
	// are 1/1.0 and 1/2.0, so the easy one should come up about twice as often.
	difficulties := []int{0, 50}

	rng := rand.New(rand.NewSource(1))
	counts := [2]int{}
	const trials = 200000
	for range trials {
		counts[WeightedDiscoveryPick(difficulties, rng.Intn)]++
	}

	// Neither is excluded.
	if counts[0] == 0 || counts[1] == 0 {
		t.Fatalf("neither candidate may be excluded; got %v", counts)
	}

	ratio := float64(counts[0]) / float64(counts[1])
	if ratio < 1.85 || ratio > 2.15 {
		t.Fatalf("difficulty-0 should be drawn about 2x as often as difficulty-50 at scale 0.02, got ratio %.3f (%v)", ratio, counts)
	}
}

// A single candidate is returned without consulting the RNG at all, and an
// empty pool returns -1 rather than panicking a caller that would index with it.
func TestWeightedDiscoveryPick_Degenerate(t *testing.T) {
	if got := WeightedDiscoveryPick(nil, func(int) int { t.Fatal("RNG must not be consulted"); return 0 }); got != -1 {
		t.Fatalf("empty pool must return -1, got %d", got)
	}
	if got := WeightedDiscoveryPick([]int{99}, func(int) int { t.Fatal("RNG must not be consulted"); return 0 }); got != 0 {
		t.Fatalf("single candidate must return 0, got %d", got)
	}
}

// Every index must be reachable. A wheel that walks weights in order can
// silently starve the last entry if the accumulator comparison is wrong.
func TestWeightedDiscoveryPick_AllIndicesReachable(t *testing.T) {
	difficulties := []int{0, 10, 25, 45, 75}
	rng := rand.New(rand.NewSource(7))
	seen := map[int]bool{}
	for range 100000 {
		seen[WeightedDiscoveryPick(difficulties, rng.Intn)] = true
	}
	for i := range difficulties {
		if !seen[i] {
			t.Fatalf("index %d (difficulty %d) was never picked", i, difficulties[i])
		}
	}
	if seen[-1] {
		t.Fatal("a non-empty pool must never return -1")
	}
}
