package guards

import (
	"os"
	"regexp"
	"testing"
)

// TestMoveWrappersDoNotReFork: the step price and the arrival detection live
// in internal/actions/move.go (movement parity 4b). If a command wrapper
// prices, charges or rolls detection itself, the player and mob paths have
// forked again (mobs walked free and never rolled detection until 4b).
func TestMoveWrappersDoNotReFork(t *testing.T) {
	forbidden := regexp.MustCompile(`GetMovementStaminaCost|ApplyCostFloatOrRefuse|CalcSneakScoreVsObserver|CalcDetectionScore|RunContest`)
	for _, path := range []string{"internal/usercommands/go.go", "internal/mobcommands/go.go"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if loc := forbidden.FindIndex(b); loc != nil {
			t.Errorf("%s prices or detects a step itself (%q); call actions.ChargeMove / actions.EntryDetection instead", path, b[loc[0]:loc[1]])
		}
	}
}
