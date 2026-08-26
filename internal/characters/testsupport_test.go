package characters

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// repoRootForTest resolves the repository root from this file's own location.
//
// Test binaries do NOT reliably start in the package directory: all tests share
// one binary, so a relative path passes or fails depending on which package ran
// first. Anchor on runtime.Caller instead.
//
// Duplicated per package because Go test helpers are not visible across
// packages.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

// pinConfigForTest enables progression for a test.
//
// A test binary does NOT see an all-zero config: ensureConfigValidated applies
// every <=0-idiom default on the first read. What stays false are the
// ConfigBools, and those are the two that gate progression entirely, so a test
// that forgets them asserts against a path that can never advance anything.
//
// It also pins ProgressionFailureFraction, which no <=0-idiom default can
// supply: 0 is a legal explicit off-switch for that knob, so its absent-key
// default is seeded by configs.newUnloadedConfig() at LOAD time. A test binary
// never loads config.yaml, so the sentinel never runs and the field reads a
// bare 0. Left unpinned, every loss-award assertion in the firing convention
// would silently compare against zero and pass for the wrong reason.
func pinConfigForTest(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.UseSkillProgression = true
	cfg.Balance.MobProgressionEnabled = true
	cfg.Balance.ProgressionFailureFraction = 0.35
	configs.SetConfigForTest(t, cfg)
}
