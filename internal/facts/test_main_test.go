package facts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func TestMain(m *testing.M) {
	// Use a temp dir so tests don't pollute real data files.
	tmp, err := os.MkdirTemp("", "facts-test-*")
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

	// Create the facts directory structure
	factsDir := filepath.Join(tmp, "facts.awareness")
	if err := os.MkdirAll(factsDir, 0o755); err != nil {
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// resetCaches wipes the in-memory caches and the saved registry and awareness
// files, now and again when the test ends, so each test starts clean.
//
// The awareness files must be part of it: RecordHeardEvent saves a mob's
// awareness to disk, and the data folder TestMain makes is shared by every pass
// of the binary. Removing only the registry file let the second pass of
// `go test -count=2` load what TestHeardEvent saved on the first, so a mob
// had already heard the event the test expects it not to know.
func resetCaches(t testing.TB) {
	t.Helper()
	wipe := func() {
		registryMu.Lock()
		registry = nil
		registryMu.Unlock()

		awarenessCacheMu.Lock()
		awarenessCache = make(map[int]*Awareness)
		awarenessCacheMu.Unlock()

		_ = os.Remove(registryFilePath())
		awarenessDir := filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "facts.awareness")
		_ = os.RemoveAll(awarenessDir)
		_ = os.MkdirAll(awarenessDir, 0o755)
	}
	wipe()
	t.Cleanup(wipe)
}
