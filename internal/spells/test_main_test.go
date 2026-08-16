package spells

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestMain initialises mudlog for the whole package.
//
// Without it, mudlog.Warn dereferences a nil slog.Logger and panics, so any
// test that drives SpellData.Validate down one of its warning branches crashed
// the test binary instead of failing an assertion. Doing it once here covers
// every test in the package.
//
// Same trap, same fix as internal/dialogue, internal/moderation and
// internal/bounties.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}
