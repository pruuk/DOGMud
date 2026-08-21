package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// The concentration floor must flip both tails: a hopeless caster still
// holds ~2% of the time, a master still breaks ~2% of the time.
func TestRunConcentrationContest_FloorFlipsBothTails(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.ConcentrationFloor = 0.02
	configs.SetConfigForTest(t, cfg)

	const n = 20000
	holdsHopeless, holdsMaster := 0, 0
	for i := 0; i < n; i++ {
		if RunConcentrationContest(50, 700).Success {
			holdsHopeless++
		}
		if RunConcentrationContest(700, 50).Success {
			holdsMaster++
		}
	}
	// 2% of 20000 = 400; allow ~5 sigma (+-100).
	if holdsHopeless < 300 || holdsHopeless > 500 {
		t.Errorf("hopeless caster held %d/20000, want ~400 (floor 0.02)", holdsHopeless)
	}
	if breaks := n - holdsMaster; breaks < 300 || breaks > 500 {
		t.Errorf("master broke %d/20000, want ~400 (flip 0.02)", breaks)
	}
}
