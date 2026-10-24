package guards

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDamageInterruptLivesAtTheHarmSeam keeps the craft and salvage damage
// interrupt at Character.ApplyHarm (#514). Hand-placed cancels at damage sites
// once missed charges, kicks, spells and thrown blades. The trigger may be
// named only where it is defined and raised (internal/characters,
// internal/state/activity) and where the two ended hooks compare against it.
func TestDamageInterruptLivesAtTheHarmSeam(t *testing.T) {
	const needle = "TriggerDamageInterrupt"
	allowedPrefixes := []string{"internal/characters/", "internal/state/activity/"}
	allowedFiles := map[string]bool{
		"internal/hooks/Activity_CraftEnded.go":   true,
		"internal/hooks/Activity_SalvageEnded.go": true,
	}
	found := 0
	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			rel := filepath.ToSlash(path)
			if d.IsDir() || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if !bytes.Contains(src, []byte(needle)) {
				return nil
			}
			allowed := allowedFiles[rel]
			for _, p := range allowedPrefixes {
				if strings.HasPrefix(rel, p) {
					allowed = true
				}
			}
			sc := bufio.NewScanner(bytes.NewReader(src))
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			n := 0
			for sc.Scan() {
				n++
				line := sc.Text()
				if !strings.Contains(line, needle) {
					continue
				}
				found++
				if !allowed {
					t.Errorf("%s:%d names %s: %s\n  damage interrupts work at Character.ApplyHarm; do not cancel at a damage site",
						rel, n, needle, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	// Proves the walk can find a match at all: the seam and both hooks name it.
	if found < 3 {
		t.Fatalf("found %d uses of %s, want at least 3; the walk or the needle is broken", found, needle)
	}
}
