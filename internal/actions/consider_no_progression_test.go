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

// Consider trains, but only behind the shared special-move cooldown (#301).
//
// look and consider were once the only stat faucets with no cooldown and no
// gate: ~3,600 perception uses an hour against forage's 150 (U10b-0 Phase D).
// consider now competes with special moves and spells for the cooldown, which
// is the gate. This guard keeps the training call (the resolved-contest
// applier, AwardResolved) after the claim, and keeps raw progression
// primitives out.
func TestConsider_TrainsOnlyBehindTheCooldown(t *testing.T) {
	src := readSource(t, "consider.go")
	claim := strings.Index(src, "ClaimSpecialMove(")
	train := strings.Index(src, "AwardResolved(")
	if claim < 0 || train < 0 || train < claim {
		t.Fatalf("consider.go must claim the special-move cooldown before it trains (claim at %d, train at %d)", claim, train)
	}
	for _, banned := range []string{"OnSkillUse", "OnStatUse", "CheckStatProgression", "CheckSkillProgression"} {
		if strings.Contains(src, banned) {
			t.Errorf("consider.go calls %s; it trains only through AwardResolved behind the cooldown", banned)
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
