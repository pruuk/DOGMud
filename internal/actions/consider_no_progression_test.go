package actions

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sourceDir returns the directory holding THIS test file.
//
// Do not use a bare relative path here: economy_test.go:28 calls os.Chdir to
// the repo root, and Go runs every test in a package in one binary, so the CWD
// a given test observes depends on whether that init has already run. Anchoring
// on runtime.Caller is CWD-independent and therefore order-independent.
func sourceDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate the source tree")
	}
	return filepath.Dir(thisFile)
}

func readSource(t *testing.T, rel string) string {
	t.Helper()
	path := filepath.Join(sourceDir(t), rel)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// Consider must not award progression.
//
// look and consider were the ONLY stat faucets in the game with no cooldown and
// no gate: at one command per second that is ~3,600 perception uses/hour,
// against forage's 150/hour ceiling. Measured live in the Phase D session --
// the very first `consider` issued printed "STATISTIC INCREASED / perception".
//
// Perception is now fed by search/forage, the perception crafts (alchemy,
// cooking, enchanting), salvage and ranged-combat, all via SkillPrimaryStats.
// Re-adding a progression call here reopens a ~24x exploit, so this test guards
// the file structurally rather than behaviourally -- there is no seam to assert
// on, since the whole point is the ABSENCE of a call.
func TestConsider_AwardsNoProgression(t *testing.T) {
	src := readSource(t, "consider.go")
	for _, banned := range []string{
		"OnStatUse",
		"OnSkillUse",
		"CheckStatProgression",
		"CheckSkillProgression",
	} {
		if strings.Contains(src, banned) {
			t.Errorf("consider.go calls %s; U10b-0 Phase D removed the ungated "+
				"perception faucet and it must not return", banned)
		}
	}
}

// The same guard for the look command's examine branch. It lives in
// internal/usercommands, which imports this package rather than the reverse, so
// this reads it by path instead of importing it.
func TestLook_AwardsNoPerceptionOnExamine(t *testing.T) {
	src := readSource(t, filepath.Join("..", "usercommands", "look.go"))
	if strings.Contains(src, `OnStatUse("perception"`) {
		t.Error(`look.go calls OnStatUse("perception"); U10b-0 Phase D removed ` +
			`the ungated perception faucet and it must not return`)
	}
}
