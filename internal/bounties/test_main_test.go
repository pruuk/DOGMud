package bounties

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)

	// Use a temp dir so tests don't pollute real data files.
	tmp, err := os.MkdirTemp("", "bounties-test-*")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	// Store the previous config to restore it after tests
	prev := configs.GetFilePathsConfig()
	configs.AddOverlayOverrides(map[string]any{
		"FilePaths.DataFiles": tmp,
	})
	defer func() {
		configs.AddOverlayOverrides(map[string]any{
			"FilePaths.DataFiles": prev.DataFiles.String(),
		})
	}()

	// Create the bounties directory structure
	bountiesDir := tmp
	if err := os.MkdirAll(bountiesDir, 0o755); err != nil {
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// resetCache gives a test an empty bounty registry: it wipes the in-memory
// cache and deletes the saved registry file, now and again when the test ends.
//
// Declare saves the registry to disk, and the data folder TestMain makes is
// shared by every pass of the binary. Clearing only the cache let the file the
// last test of one pass saved come back as the first test's registry on the
// next pass, so `go test -count=2` saw bounty id 2 where it expected 1.
func resetCache(t testing.TB) {
	t.Helper()
	wipeRegistry := func() {
		registryMu.Lock()
		registry = nil
		registryMu.Unlock()
		_ = os.Remove(registryFilePath())
	}
	wipeRegistry()
	t.Cleanup(wipeRegistry)
}
