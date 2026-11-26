package mobs

import (
	"fmt"
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)

	// The Go default for FilePaths.DataFiles is the relative
	// `_datafiles/world/default`, so instance saves and the summons nuke
	// tests wrote under internal/mobs/_datafiles/ and removed it again while
	// guards in other packages were walking the repo (#953). Give the
	// package its own folder outside the repo, as bounties, facts and
	// knowledge do.
	tmp, err := os.MkdirTemp("", "mobs-test-datafiles-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mobs: mkdirtemp:", err)
		os.Exit(1)
	}
	if err := configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": tmp}); err != nil {
		fmt.Fprintln(os.Stderr, "mobs: redirect DataFiles:", err)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}
