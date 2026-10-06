package main

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

// realOverridesPath is the live world's engine-written overrides file. In a
// developer checkout that runs a server it holds NextRoomId and CurrentVersion,
// so no root test may touch it.
const realOverridesPath = `_datafiles/world/dogmud/config-overrides.yaml`

// TestMain points CONFIG_PATH at a scratch file before any root test runs.
// Room creation persists Server.NextRoomId through configs.SetEngineVal, which
// writes to CONFIG_PATH when set and otherwise to the real overrides file under
// the world's data folder. After the run it fails the package if the real
// file was created or changed.
func TestMain(m *testing.M) {
	before, beforeErr := os.ReadFile(realOverridesPath)

	scratch, err := os.MkdirTemp("", "dogmud-root-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "root test: mkdirtemp:", err)
		os.Exit(1)
	}
	// os.Setenv, not t.Setenv: this must hold for every test in the package.
	os.Setenv(`CONFIG_PATH`, scratch+`/config-overrides.yaml`)

	code := m.Run()

	os.RemoveAll(scratch)

	after, afterErr := os.ReadFile(realOverridesPath)
	if (beforeErr == nil) != (afterErr == nil) || !bytes.Equal(before, after) {
		fmt.Fprintf(os.Stderr, "root tests wrote %s (existed before: %v); route config writes through CONFIG_PATH\n", realOverridesPath, beforeErr == nil)
		code = 1
	}
	os.Exit(code)
}
