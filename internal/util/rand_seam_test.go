package util

import "testing"

// SetRandForTest replays one seeded sequence, so a golden can pin every
// random draw a run makes (item behaviour slice 2, spec X19).
func TestSetRandForTestReplaysASeededSequence(t *testing.T) {
	draw := func() []int {
		out := make([]int, 20)
		for i := range out {
			out[i] = Rand(100)
		}
		return out
	}

	restore := SetRandForTest(42)
	first := draw()
	restore()

	restore = SetRandForTest(42)
	second := draw()
	restore()

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("draw %d: %d then %d; the same seed must replay the same sequence", i, first[i], second[i])
		}
	}
	if Rand(0) != 0 {
		t.Fatal("Rand(0) must stay 0 whatever the source")
	}
}
