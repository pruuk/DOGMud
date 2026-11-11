package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// An AI tool call's own markup once landed at the top of a help file
// (help/serpents-edge, from e8c5eeec0, removed 2026-11-11), so every player
// who typed `help serpents-edge` saw two lines of tool syntax, one naming a
// path on the developer's machine. Nothing a player reads may carry that
// markup. The pattern is assembled so this file does not match itself.
var leakedToolText = regexp.MustCompile(`<` + `(/?parameter|invoke)\b[^>]*>`)

// leakedToolTextDirs hold everything the game serves: world data, and each
// module's embedded files.
var leakedToolTextDirs = []string{"_datafiles", "modules"}

func TestGameDataHasNoLeakedToolMarkup(t *testing.T) {
	if !leakedToolText.MatchString(`<` + `parameter name="content">x`) {
		t.Fatal("probe: the pattern cannot match tool markup")
	}
	root := repoRoot(t)
	scanned := 0
	for _, dir := range leakedToolTextDirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || strings.HasSuffix(path, ".go") {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".template", ".yaml", ".yml", ".md", ".txt", ".html", ".js", ".json":
			default:
				return nil
			}
			b, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			scanned++
			for i, line := range strings.Split(string(b), "\n") {
				if leakedToolText.MatchString(line) {
					rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(os.PathSeparator)))
					t.Errorf("%s:%d: AI tool markup in game data: %.80s", rel, i+1, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	if scanned < 1000 {
		t.Fatalf("scanned only %d data files; the walk cannot be finding the game data", scanned)
	}
}
