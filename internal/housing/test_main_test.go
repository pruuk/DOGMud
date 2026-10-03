package housing

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// mudlog.Error dereferences a nil logger, so without this every save-failure
// and quarantine path would crash the test binary instead of failing an
// assertion (dogmud-persistence skill).
func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}
