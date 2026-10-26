package guards

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// #680: Character.Size() (internal/characters/size.go) is the one reader of
// species size. A second reader would ignore a size a mutation grants.
var speciesSizeRead = regexp.MustCompile(`\.Size\s*(==|!=)\s*species\.|switch\s+[\w.]+\.Size\s*\{`)

func TestSpeciesSizeReadOnlyInSize(t *testing.T) {
	root := repoRoot(t)
	var bad []string
	for _, dir := range []string{"internal", "modules"} {
		filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "internal/species/") || rel == "internal/characters/size.go" {
				return nil
			}
			src, _ := os.ReadFile(path)
			for n, line := range strings.Split(string(src), "\n") {
				if speciesSizeRead.MatchString(line) {
					bad = append(bad, rel+":"+itoa(n+1)+": "+strings.TrimSpace(line))
				}
			}
			return nil
		})
	}
	if len(bad) > 0 {
		t.Fatalf("species size read outside Character.Size():\n%s", strings.Join(bad, "\n"))
	}
}
