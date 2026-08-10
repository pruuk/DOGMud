package moderation

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestMain initialises mudlog for the whole package.
//
// Without it, mudlog.Error dereferences a nil slog.Logger and panics, so any
// test that drives a failure path here crashed the test binary rather than
// failing an assertion. That is very likely why the save-failure paths in this
// package had no coverage at all before chunk 2.3 — they were untestable in
// practice, not merely untested.
//
// Same pattern as internal/bounties/test_main_test.go.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}
