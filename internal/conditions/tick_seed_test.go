package conditions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// #594: the tick variance draws from the seedable source, so a seeded test
// replays it. It drew from math/rand and could not be pinned.
func TestComputeTickAmount_VarianceReplaysUnderASeed(t *testing.T) {
	run := func(seed int64) []int {
		defer util.SetRandForTest(seed)()
		out := make([]int, 20)
		for i := range out {
			out[i] = ComputeTickAmount(1000, 0.05, 0.10, 1, 1.0)
		}
		return out
	}
	a, b := run(594), run(594)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("draw %d differs under one seed: %d then %d", i, a[i], b[i])
		}
	}
	varied := false
	for _, v := range a[1:] {
		varied = varied || v != a[0]
	}
	if !varied {
		t.Fatalf("precondition: the variance must vary the amount across draws: %v", a)
	}
}
