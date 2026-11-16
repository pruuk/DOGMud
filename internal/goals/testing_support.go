package goals

import (
	"path/filepath"
	"testing"
)

// goalsDirEnv names the environment variable goalsBaseDir reads before it
// falls back to the configured data folder.
const goalsDirEnv = "DOGMUD_GOALS_DIR_OVERRIDE"

// IsolateForTest gives one test its own goals folder and an empty goals
// cache, and restores both when the test ends. Every test that stores a goal
// (Add, AddIndividual, Recompute, or a path that reaches them) calls it first.
//
// Goals are saved to disk on every change, and ClearCache empties only the
// in-memory cache. A folder shared by the whole test binary therefore lets a
// goal saved by one run come back on the next: `go test -count=2` failed seven
// behaviortree tests that way (#896). A folder per test cannot leak.
//
// It uses t.Setenv, so a test that calls it cannot also call t.Parallel; the
// goals cache is package state, so such a test could not run in parallel
// anyway.
func IsolateForTest(t testing.TB) {
	t.Helper()
	t.Setenv(goalsDirEnv, filepath.Join(t.TempDir(), "goals"))
	ClearCache()
	t.Cleanup(ClearCache)
}
