package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
)

// RunContest is the ONLY place ContestFloor is read. Everything else in the
// game reaches the floor through it.
func TestRunContest_AppliesTheConfiguredFloor(t *testing.T) {
	const iterations = 20000
	wins := 0
	for i := 0; i < iterations; i++ {
		if RunContest(10, []contest.Entry{{Score: 10000}}).Success {
			wins++
		}
	}
	rate := float64(wins) / iterations
	if rate < 0.10 || rate > 0.15 {
		t.Fatalf("hopeless attacker should be floored to about 0.125, got %v", rate)
	}
}

func TestRunContest_UncontestedIsUntouched(t *testing.T) {
	res := RunContest(100, nil)
	if res.Contested {
		t.Fatal("no entries means no contest")
	}
	if res.Floored {
		t.Fatal("an uncontested result has no outcome to flip")
	}
}
