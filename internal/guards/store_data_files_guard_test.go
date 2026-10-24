package guards

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Each data file has exactly one Go owner that names it. M3 item 7 moved the
// gossip templates out of hook globals and renamed the hints broadcast to tips;
// a second reader would reintroduce the silent, unvalidated load both stores
// replaced, and a stray hints.yaml would read a file that no longer exists.
var storeDataFileOwners = []struct{ file, owner string }{
	{"gossip_templates.yaml", "internal/gossip/"},
	{"tips.yaml", "internal/tips/"},
}

func TestStoreDataFilesAreNamedOnlyByTheirStore(t *testing.T) {
	inside := map[string]int{}
	var problems []string

	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			rel := filepath.ToSlash(path)
			for i, line := range strings.Split(string(src), "\n") {
				if strings.Contains(line, "hints.yaml") {
					problems = append(problems, fmt.Sprintf("%s:%d names hints.yaml, which was renamed to tips.yaml: %s", rel, i+1, strings.TrimSpace(line)))
				}
				for _, o := range storeDataFileOwners {
					if !strings.Contains(line, o.file) {
						continue
					}
					if strings.HasPrefix(rel, o.owner) {
						inside[o.file]++
						continue
					}
					problems = append(problems, fmt.Sprintf("%s:%d names %s outside %s: %s", rel, i+1, o.file, o.owner, strings.TrimSpace(line)))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	for _, o := range storeDataFileOwners {
		if inside[o.file] == 0 {
			t.Errorf("%s is named nowhere inside %s; the guard is blind, not the tree clean", o.file, o.owner)
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}
