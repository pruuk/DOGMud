package guards

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// House style (dogmud-player-copy): no em dash, no en dash, and no spaced
// hyphen or spaced double hyphen (" - ", " -- ") standing in for one, in
// prose a player reads. M6 slice 2 (ledger row 55). Quests, dialogue and
// conversations are exempt until the quest and dialogue rework arc (#375).
//
// Quests, dialogue and conversations are not listed: their dashes wait for
// the quest and dialogue rework arc (#375).
var proseDashDirs = []string{
	"combat-messages",
	"defense-messages",
	"narration",
	"messaging",
	"rooms",
	"mobs",
	"taunt-messages",
	"spells",
}

// proseDashFiles are bare player-text files at the world root.
var proseDashFiles = []string{
	"casting-messages.yaml",
	"crafting-messages.yaml",
}

// The characters around a spaced dash may be a tag bracket or a token brace
// ("{actor}</ansi> - their jaws", "opening - <ansi ...>"): a tag is invisible
// in play, so a dash beside one reads as a dash all the same.
var (
	emOrEnDash   = regexp.MustCompile("[—–]")
	spacedHyphen = regexp.MustCompile(`[A-Za-z0-9,.!?'")>}] --? [A-Za-z0-9<{'"]`)
	// Holes a folded or double-quoted multi-line string opens: a dash after a
	// `\ ` escaped-space continuation, a dash that ends a line, and a double
	// hyphen that starts a continuation line.
	contHyphen     = regexp.MustCompile(`\\ --? [A-Za-z<{]`)
	trailingHyphen = regexp.MustCompile(`[^\s] --?\\?$`)
	leadingDouble  = regexp.MustCompile(`^\s+-- `)
)

func TestPlayerProseHasNoDashes(t *testing.T) {
	root := filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud")
	scanned := 0
	scanFile := func(path string) error {
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
			if emOrEnDash.MatchString(line) || spacedHyphen.MatchString(line) ||
				contHyphen.MatchString(line) || trailingHyphen.MatchString(line) || leadingDouble.MatchString(line) {
				t.Errorf("%s:%d uses a dash: %s", path, n, strings.TrimSpace(line))
			}
		}
		return sc.Err()
	}
	for _, d := range proseDashDirs {
		base := filepath.Join(root, d)
		if _, err := os.Stat(base); err != nil {
			t.Fatalf("%s: %v; the guard would check nothing there", base, err)
		}
		err := filepath.WalkDir(base, func(path string, de os.DirEntry, err error) error {
			if err != nil || de.IsDir() || filepath.Ext(path) != ".yaml" {
				return err
			}
			return scanFile(path)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range proseDashFiles {
		path := filepath.Join(root, f)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v; the guard would check nothing there", path, err)
		}
		if err := scanFile(path); err != nil {
			t.Fatal(err)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no lines")
	}
}
