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
func pinConfigForTest(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.UseSkillProgression = true
	cfg.Balance.MobProgressionEnabled = true
	// Pinned explicitly: a Config built from scratch skips the sentinel. See
	// newUnloadedConfig in internal/configs/configs.go.
	cfg.Balance.ProgressionFailureFraction = 0.35
	configs.SetConfigForTest(t, cfg)
}
