package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// pinContestFloorOff removes Balance.ContestFloor for the duration of the
// calling test.
//
// U6 made this necessary, and it is the single most likely reason a
// previously-solid contest test starts flaking. Before U6 the maneuver and
// spell floors read configs.GetBalanceConfig(), which a Go test binary never
// loads from _datafiles/config.yaml, so they measured 0 and a test asserting
// that an overwhelming attacker WINS was deterministic for free.
// Balance.Validate replaces a zero or out-of-range ContestFloor with 0.125, so
// the floor is live in every test binary now and hands the hopeless side a
// 12.5% save. Any test that asserts a single lopsided contest resolves the
// obvious way must pin it or it fails about one run in eight.
//
// configs.SetConfigForTest assigns without validating, which is why a zero
// survives here, and it self-registers the restore.
func pinContestFloorOff(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.ContestFloor = 0
	configs.SetConfigForTest(t, c)
}

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
