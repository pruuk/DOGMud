package guards

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// spellPhaseNarrationRE matches a site rendering a spell phase: a Narrate or
// Narration call with a spells.Phase constant, or (inside the body) with the
// phase parameter.
var spellPhaseNarrationRE = regexp.MustCompile(`\.(Narrate|Narration)\((spells\.Phase\w+|phase)\b`)

// TestSpellPhaseNarrationLivesInOneBody keeps a caster's own spell lines in
// actions.NarrateSpellPhase (messaging M6 slice 4). Seven sites once rendered
// them under four rules; a new site must call the body, not render a phase.
func TestSpellPhaseNarrationLivesInOneBody(t *testing.T) {
	const home = "internal/actions/spell_phase_lines.go"
	inside := 0
	for _, root := range messagingSurfaceGoRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			rel := filepath.ToSlash(path)
			if d.IsDir() || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") ||
				strings.HasPrefix(rel, "internal/spells/") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			sc := bufio.NewScanner(bytes.NewReader(src))
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			n := 0
			for sc.Scan() {
				n++
				if !spellPhaseNarrationRE.MatchString(sc.Text()) {
					continue
				}
				if rel == home {
					inside++
					continue
				}
				t.Errorf("%s:%d renders a spell phase itself: %s\n  call actions.NarrateSpellPhase instead",
					rel, n, strings.TrimSpace(sc.Text()))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if inside == 0 {
		t.Errorf("the pattern matched nothing in %s; the guard is blind, not the tree clean", home)
	}
}
