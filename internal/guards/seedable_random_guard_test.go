package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// #594: every random draw in the game goes through the one seedable source
// (internal/dice, which util.Rand also reads), so a test can replay a run with
// util.SetRandForTest. A package that imports math/rand draws around it, and
// its draws cannot be pinned (condition tick variance, the Bloom rolls and
// mutation pick, event debug sampling were found doing so).
//
// Allowed: internal/dice itself, and gametime/zodiac.go, which builds its own
// generator from the world's configured seed on purpose (the sky is the same
// for every run of one world).
func TestOnlyTheDiceSourceImportsMathRand(t *testing.T) {
	allowed := map[string]bool{
		"internal/dice/source.go":     true,
		"internal/gametime/zodiac.go": true,
	}
	var offenders []string
	scanned := 0
	for _, root := range []string{"internal", "modules"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			slash := filepath.ToSlash(path)
			if strings.Contains(string(raw), `"math/rand"`) && !allowed[slash] {
				offenders = append(offenders, slash)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 500 {
		t.Fatalf("scanned only %d files; the walk is not seeing the source tree", scanned)
	}
	for _, f := range offenders {
		t.Errorf("%s imports math/rand; draw through dice (dice.Intn, dice.Float64) or util.Rand so tests can seed it", f)
	}
}
