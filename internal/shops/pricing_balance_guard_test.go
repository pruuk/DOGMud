package shops

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Live pricing reads its knobs from config.yaml through
// PricingConfigFromBalance. DefaultPricingConfig holds the Go defaults, which
// are only the fallback PricingConfigFromBalance starts from; a production path
// that calls it directly ignores every retune of the shop knobs. The NPC
// crafter's craft and salvage decisions once did exactly that.
func TestDefaultPricingConfig_OnlyCalledByPricingConfigFromBalance(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	allowed := filepath.Join(repoRoot, "internal", "shops", "pricing.go")

	scanned := 0
	var offenders []string
	for _, dir := range []string{"internal", "modules"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			scanned++
			if path == allowed {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(src), "DefaultPricingConfig(") {
				offenders = append(offenders, path)
			}
			return nil
		})
		require.NoError(t, err)
	}
	// The scan must have been able to find something.
	require.Greater(t, scanned, 100, "walked too few files; is the repo root right?")
	src, err := os.ReadFile(allowed)
	require.NoError(t, err)
	require.Contains(t, string(src), "DefaultPricingConfig(", "the probe string must match the real call")
	require.Empty(t, offenders, "use shops.PricingConfigFromBalance() so config.yaml knobs apply")
}
