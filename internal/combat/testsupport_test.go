package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// pinConfigForTest enables progression for a test.
//
// A test binary does NOT see an all-zero config: ensureConfigValidated applies
// every <=0-idiom default on the first read. What stays false are the
// ConfigBools, and those are the two that gate progression entirely.
//
// repoRootForTest is NOT defined here; this package already has one in
// coup_de_grace_test.go.
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
