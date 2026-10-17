package util

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dice"
)

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

// #576, #524: one seed must pin the dice too. Before this, dice drew from the
// unseedable math/rand global, so a seeded test still rolled every contest
// live and a contest-driven assertion flaked at the fumble rate.
func TestSetRandForTestAlsoReplaysDiceRolls(t *testing.T) {
	draw := func() []float64 {
		out := make([]float64, 0, 40)
		for i := 0; i < 10; i++ {
			out = append(out,
				dice.Roll(100, 15).Value,
				dice.Float64(),
				float64(dice.RollBetweenInt(1, 6)),
				float64(Rand(100)), // interleaved: one shared sequence
			)
		}
		return out
	}

	restore := SetRandForTest(7)
	first := draw()
	restore()

	restore = SetRandForTest(7)
	second := draw()
	restore()

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("draw %d: %v then %v; util.SetRandForTest must seed dice as well", i, first[i], second[i])
		}
	}
}
