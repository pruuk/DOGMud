package shops

import (
	"path/filepath"
	"testing"
)

// IsolateForTest gives one test its own shops folder and an empty shop cache,
// and restores both when the test ends. Every test that registers or saves a
// shop calls it first.
//
// RegisterShop and every trade save the shop to disk, and ClearCache empties
// only the in-memory cache, so a shop saved by one test came back in a later
// one: under `go test -count=2` an offer test's sale made the next pass's list
// test read a stale shop (#896). Test binaries also resolve the default data
// folder against their working directory, so those saves landed in the
// repository's own _datafiles tree. A folder per test fixes both.
//
// It changes package state, so a test that calls it cannot run in parallel.
func IsolateForTest(t testing.TB) {
	t.Helper()
	prev := shopsDirOverride
	shopsDirOverride = filepath.Join(t.TempDir(), "shops")
	ClearCache()
	t.Cleanup(func() {
		ClearCache()
		shopsDirOverride = prev
	})
}
