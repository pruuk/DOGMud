package dice

import (
	"sync"
	"testing"
)

// #576: SetRandForTest pins every kind of dice draw, and restore puts the
// previous source back.
func TestSetRandForTest_ReplaysEveryDrawKind(t *testing.T) {
	draw := func() []float64 {
		out := make([]float64, 0, 60)
		for i := 0; i < 10; i++ {
			ok, roll := Percentile(50)
			_ = ok
			_, _, a, d := OpposedRollStatWithFloors(100, 100, 0.2, 0.2)
			out = append(out, Roll(100, 15).Value, roll, RollBetween(0, 1),
				float64(RollTable([]int{1, 2, 3})), a.Value, d.Value)
		}
		return out
	}

	restore := SetRandForTest(11)
	first := draw()
	restore()
	restore = SetRandForTest(11)
	second := draw()
	restore()

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("draw %d: %v then %v; the same seed must replay the same sequence", i, first[i], second[i])
		}
	}
	if source.Load() != nil {
		t.Fatal("restore must put the production (nil) source back")
	}
}

// The seeded generator is shared by every goroutine that rolls, so it must
// be safe for concurrent draws. Meaningful under -race.
func TestSetRandForTest_ConcurrentDrawsAreRaceFree(t *testing.T) {
	t.Cleanup(SetRandForTest(3))
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				Roll(100, 15)
				Intn(10)
				Float64()
			}
		}()
	}
	wg.Wait()
}
