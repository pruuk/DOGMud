package scavenger

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestMain installs a logger: mudlog dereferences a nil one, and resolvePool
// logs the rooms it drops.
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}
