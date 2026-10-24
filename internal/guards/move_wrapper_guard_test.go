package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestMoveWrappersDoNotReFork: the step price and the arrival detection live
// in internal/actions/move.go (movement parity 4b). If a command wrapper
// prices, charges or rolls detection itself, the player and mob paths have
// forked again (mobs walked free and never rolled detection until 4b).
func TestMoveWrappersDoNotReFork(t *testing.T) {
	forbidden := regexp.MustCompile(`DeductActionPoints|GetMovementStaminaCost|ApplyCostFloatOrRefuse|CalcSneakScoreVsObserver|CalcDetectionScore|RunContest`)
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

// moveAPSpenders are the only production files allowed to name
// DeductActionPoints: its definition and actions.ChargeMove's home. Whole
// files, not packages, so a second spender anywhere else in internal/actions
// or internal/characters still fails.
var moveAPSpenders = map[string]bool{
	"internal/characters/resources.go": true, // the definition
	"internal/actions/move.go":         true, // ChargeMove
}

// TestOnlyActionsSpendActionPoints: action points are spent by
// actions.ChargeMove alone. Any other production caller is a second price.
func TestOnlyActionsSpendActionPoints(t *testing.T) {
	scanned := 0
	allowedSeen := map[string]bool{}
	for _, root := range []string{"internal", "modules"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			scanned++
			slash := filepath.ToSlash(path)
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !strings.Contains(string(b), "DeductActionPoints(") {
				return nil
			}
			if moveAPSpenders[slash] {
				allowedSeen[slash] = true
				return nil
			}
			t.Errorf("%s spends action points itself; only actions.ChargeMove may", slash)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if scanned < 500 {
		t.Fatalf("scanned only %d files; the walk is not seeing the tree, so a pass proves nothing", scanned)
	}
	// A stale allow-list row would hide a moved spender: each allowed file
	// must still name DeductActionPoints.
	for path := range moveAPSpenders {
		if !allowedSeen[path] {
			t.Errorf("%s is allowed to spend action points but no longer names DeductActionPoints; update moveAPSpenders", path)
		}
	}
}
