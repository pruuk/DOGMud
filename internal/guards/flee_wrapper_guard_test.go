package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestFleeWrappersDoNotReFork: the flee rules live in actions.BeginFlee and
// actions.ResolveFlee. If a command wrapper transitions CombatPhase, prices
// or charges the flee, or runs the blocker contest itself, the player and mob
// flees have forked again (until slice 4a the mob's was free, instant and
// ignored standing).
func TestFleeWrappersDoNotReFork(t *testing.T) {
	forbidden := regexp.MustCompile(`TransitionToDisengaging|QuoteActionCost|CommitCost|FleeStaminaCost|ResolveFleeBlockers`)
	for _, path := range []string{"internal/usercommands/flee.go", "internal/mobcommands/flee.go"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if loc := forbidden.FindIndex(b); loc != nil {
			t.Errorf("%s applies flee rules itself (%q); call actions.BeginFlee instead", path, b[loc[0]:loc[1]])
		}
	}
}

// TestFleeResolutionStaysInActions: only actions (and the packages that own
// the primitives) may run the blocker contest or settle a flee's phase.
func TestFleeResolutionStaysInActions(t *testing.T) {
	resolve := regexp.MustCompile(`ResolveFleeBlockers\(|CombatPhase\.ResolveFlee\(`)
	allowed := []string{"internal/actions/", "internal/combat/", "internal/state/combatphase/"}
	checked := 0
	for _, root := range []string{"internal", "modules"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			p := filepath.ToSlash(path)
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			for _, a := range allowed {
				if strings.HasPrefix(p, a) {
					return nil
				}
			}
			checked++
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if loc := resolve.FindIndex(b); loc != nil {
				t.Errorf("%s resolves a flee itself (%q); call actions.ResolveFlee instead", p, b[loc[0]:loc[1]])
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if checked == 0 {
		t.Fatal("walked no files; the guard is broken, not the code")
	}
}
