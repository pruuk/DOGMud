package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// #896: goals and shops save to disk on every change, and their ClearCache
// empties only memory. A test that saves one into a folder it shares with
// later tests leaks it into them: under `go test -count=2` seven behaviortree
// tests, most of internal/goals and two usercommands list tests failed on the
// second pass. Each package now has IsolateForTest(t), which gives the test its
// own folder; a test file that saves goals or shops must call it.
//
// The check is per file, not per test: a file that calls IsolateForTest in one
// test and saves in another passes. That is the trade for a guard that needs
// no type information; review covers the rest.
func TestTestsThatSaveGoalsOrShopsIsolateTheirFolder(t *testing.T) {
	rules := []struct {
		pkg     string         // the package whose own tests call it unqualified
		saves   *regexp.Regexp // a call that writes the package's saves
		isolate string
	}{
		// Group 1 is the qualifier, or the one character before an
		// unqualified call (never a dot, so wg.Add does not match).
		{"internal/goals", regexp.MustCompile(`(goals\.|[^\w.])(Add|AddIndividual|Recompute)\(`), "IsolateForTest("},
		{"internal/shops", regexp.MustCompile(`(shops\.|[^\w.])(RegisterShop|SaveShop)\(`), "IsolateForTest("},
	}
	var offenders []string
	scanned := 0
	for _, root := range []string{"internal", "modules"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			slash := filepath.ToSlash(path)
			if strings.HasPrefix(slash, "internal/guards/") {
				return nil
			}
			src := string(raw)
			for _, r := range rules {
				own := filepath.ToSlash(filepath.Dir(path)) == r.pkg
				qualifier := filepath.Base(r.pkg) + "."
				calls := false
				for _, m := range r.saves.FindAllStringSubmatch(src, -1) {
					if own || m[1] == qualifier {
						calls = true
						break
					}
				}
				if !calls {
					continue
				}
				want := qualifier + r.isolate
				if own {
					want = r.isolate
				}
				if !strings.Contains(src, want) {
					offenders = append(offenders, slash+": saves "+qualifier[:len(qualifier)-1]+" but never calls "+want+"t)")
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 500 {
		t.Fatalf("scanned only %d test files; the walk is not seeing the source tree", scanned)
	}
	for _, o := range offenders {
		t.Error(o)
	}
}
