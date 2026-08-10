package dialogue

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestMain initialises mudlog for the whole package.
//
// Without it, mudlog.Error dereferences a nil slog.Logger and panics, so any
// test that drives a load failure crashed the test binary instead of failing an
// assertion. save_test.go worked around this by calling SetupLogger inside one
// of its helpers; doing it once here covers every test and stops the next
// person rediscovering it.
//
// Same trap, same fix as internal/moderation and internal/bounties.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}
