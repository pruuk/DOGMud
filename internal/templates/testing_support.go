package templates

import (
	"io/fs"
	"testing"
)

// SetFSForTest replaces the registered template filesystems with exactly the
// ones given, for the duration of the calling test, and restores the prior
// registrations via t.Cleanup. Exported (like configs.SetConfigForTest) so a
// package outside templates that needs Process to actually read real
// on-disk templates in a test (rather than the untouched, empty-fileSystems
// default) can get that without permanently leaking a RegisterFS call into
// every later test sharing the same test binary process.
//
// RegisterFS only appends and has no matching unregister: once called, its
// filesystem stays registered for the rest of that test binary's run. That
// matters because readFile's zero-registrations behavior is not "always
// fail" but "always vacuously succeed with zero bytes" (see the comment on
// TestMain in process_test.go), which many callers across the codebase rely
// on in tests that never set up a real templates filesystem or a matching
// FilePaths.DataFiles. A single unscoped RegisterFS call flips that default
// for everything downstream in the same run, so anything that needs a real,
// disk-backed render in one test must use this instead.
//
// Use this in tests instead of RegisterFS, which appends and leaks. This is
// not safe with parallel tests (t.Parallel()): fileSystems is an unlocked
// package-level var, so two tests racing SetFSForTest step on each other's
// registrations and cleanups. It also forgets which narrow forms exist, on
// the way in and out, since they were found on the old filesystems.
func SetFSForTest(t *testing.T, filesystems ...fs.ReadFileFS) {
	t.Helper()
	original := fileSystems
	fileSystems = filesystems
	formCache.Clear()
	t.Cleanup(func() {
		fileSystems = original
		formCache.Clear()
	})
}

// ProcessAtWidth renders fname as Process does for a reader whose screen is
// width columns wide, without a screen reader. For guards that hold every
// template to the width it is shown at (internal/guards).
func ProcessAtWidth(fname string, data any, width int) (string, error) {
	return process(fname, data, templateConfig{}, width)
}
