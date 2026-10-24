package guards

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// M6 slice 2 (ledger rows 48 to 53, owner 2026-10-12): six idioms that do not
// translate for players reading English as a second language are retired
// from combat text. Replacements: barely hits, misses / flies past, dodge
// most of it, hurts a little, stay on your feet, lose your balance, runs
// over / runs right past.
var retiredCombatIdiom = regexp.MustCompile(`(?i)\bclip(s|ped|ping)?\b|goes wide|went wide|\bslips? most\b|slipped most|\bsliver\b|\bfooting\b|\boverruns?\b|carries straight past`)

func TestCombatTextHasNoRetiredIdioms(t *testing.T) {
	root := filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud")
	dirs := []string{"combat-messages", "defense-messages", filepath.Join("narration", "special-moves"), "messaging"}
	scanned := 0
	for _, d := range dirs {
		files, err := filepath.Glob(filepath.Join(root, d, "*.yaml"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no yaml under %s (err %v); the guard would check nothing", d, err)
		}
		for _, f := range files {
			fh, err := os.Open(f)
			if err != nil {
				t.Fatal(err)
			}
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
				if m := retiredCombatIdiom.FindString(line); m != "" {
					t.Errorf("%s:%d uses retired idiom %q: %s", f, n, m, strings.TrimSpace(line))
				}
			}
			fh.Close()
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no lines")
	}
}
