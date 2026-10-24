package guards

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Words and tags need a space between them. A tag is invisible, so
// "you<ansi fg=...>FUMBLE</ansi>" renders "youFUMBLE". The dash rewrite once
// dropped these spaces in dozens of lines. The same rewrite also produced
// the joined words "youare", "theyare" and "weare".
var (
	wordTagJoin  = regexp.MustCompile(`[A-Za-z]<ansi\b|</ansi>[A-Za-z]`)
	joinedAreRun = regexp.MustCompile(`\b(you|they|we)are\b`)
	// "theirjaws", "yourvitality": a possessive run into the noun after it.
	// Go's regexp has no lookahead, so the real possessives are filtered in
	// runTogetherPossessive.
	possessiveRun = regexp.MustCompile(`\b(their|your)[a-z]+\b`)
)

var realPossessiveWords = map[string]bool{
	"theirs": true, "yours": true, "yourself": true, "yourselves": true,
}

// runTogetherPossessive returns the first "their..." or "your..." word in s
// that is not a real English word, or "" when there is none.
func runTogetherPossessive(s string) string {
	for _, w := range possessiveRun.FindAllString(s, -1) {
		if !realPossessiveWords[w] {
			return w
		}
	}
	return ""
}

func TestProseTagSpacing(t *testing.T) {
	root := filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud")
	scanned := 0
	for _, d := range proseDashDirs {
		base := filepath.Join(root, d)
		if _, err := os.Stat(base); err != nil {
			t.Fatalf("%s: %v; the guard would check nothing there", base, err)
		}
		err := filepath.WalkDir(base, func(path string, de os.DirEntry, err error) error {
			if err != nil || de.IsDir() || filepath.Ext(path) != ".yaml" {
				return err
			}
			fh, err := os.Open(path)
			if err != nil {
				return err
			}
			defer fh.Close()
			sc := bufio.NewScanner(fh)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			n := 0
			for sc.Scan() {
				n++
				line := sc.Text()
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				scanned++
				if wordTagJoin.MatchString(line) {
					t.Errorf("%s:%d word joined to a tag (needs a space): %s", path, n, strings.TrimSpace(line))
				}
				if w := runTogetherPossessive(line); w != "" {
					t.Errorf("%s:%d run-together possessive %q: %s", path, n, w, strings.TrimSpace(line))
				}
				if joinedAreRun.MatchString(line) {
					t.Errorf("%s:%d joined words (you are / they are / we are): %s", path, n, strings.TrimSpace(line))
				}
			}
			return sc.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no lines")
	}
}
