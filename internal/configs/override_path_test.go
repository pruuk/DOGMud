package configs

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// ReloadConfig logs through mudlog, which is nil until set up.
var setupLoggerOnce sync.Once

// isolateReloadGlobals snapshots every package global ReloadConfig writes and
// restores it when the test ends, then seeds configData with the UNLOADED
// config, which is what the first ReloadConfig of a boot sees.
func isolateReloadGlobals(t *testing.T) {
	t.Helper()
	setupLoggerOnce.Do(func() { mudlog.SetupLogger(nil, "", "", false) })
	SetConfigForTest(t, newUnloadedConfig())
	origOverrides := overrides
	origModuleKeys := moduleOverlayKeys
	origKeyLookups := keyLookups
	origTypeLookups := typeLookups
	t.Cleanup(func() {
		overrides = origOverrides
		moduleOverlayKeys = origModuleKeys
		keyLookups = origKeyLookups
		typeLookups = origTypeLookups
	})
}

// writeWorld builds <dir>/_datafiles/config.yaml naming world/x as DataFiles,
// and world/x/config-overrides.yaml setting NextRoomId.
func writeWorld(t *testing.T, dir string) {
	t.Helper()
	mustWrite(t, filepath.Join(dir, "_datafiles", "config.yaml"),
		"FilePaths:\n  DataFiles: world/x\n")
	mustWrite(t, filepath.Join(dir, "world", "x", "config-overrides.yaml"),
		"Server:\n  NextRoomId: 424242\n")
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The first ReloadConfig of a boot runs while configData is still the unloaded
// default, whose DataFiles validates to _datafiles/world/default. The override
// file must be read from the world config.yaml names, not from that default.
func TestReloadConfig_ReadsOverridesFromLoadedDataFiles(t *testing.T) {
	isolateReloadGlobals(t)
	t.Setenv("CONFIG_PATH", "")
	dir := t.TempDir()
	writeWorld(t, dir)
	t.Chdir(dir)

	if err := ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	if got := GetConfig().Server.NextRoomId; got != 424242 {
		t.Fatalf("NextRoomId = %d, want 424242: world/x/config-overrides.yaml was not loaded", got)
	}
}

// A config.yaml with no FilePaths.DataFiles key reads its overrides from the
// default world, the value FilePaths.Validate fills in.
func TestReloadConfig_NoDataFilesKeyReadsDefaultWorld(t *testing.T) {
	isolateReloadGlobals(t)
	t.Setenv("CONFIG_PATH", "")
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "_datafiles", "config.yaml"),
		"Server:\n  MudName: test\n")
	mustWrite(t, filepath.Join(dir, "_datafiles", "world", "default", "config-overrides.yaml"),
		"Server:\n  NextRoomId: 515151\n")
	t.Chdir(dir)

	if err := ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	if got := GetConfig().Server.NextRoomId; got != 515151 {
		t.Fatalf("NextRoomId = %d, want 515151: _datafiles/world/default/config-overrides.yaml was not loaded", got)
	}
}

// #720: ReloadConfigForTest loads the shipped config for one test and puts
// the previous config, lookups and overrides back at cleanup, so the shipped
// values do not leak into later tests.
func TestReloadConfigForTest_RestoresWhatItReplaced(t *testing.T) {
	isolateReloadGlobals(t)
	t.Setenv("CONFIG_PATH", "")
	dir := t.TempDir()
	writeWorld(t, dir)
	t.Chdir(dir)

	before := GetConfig()
	before.Server.NextRoomId = 1234
	SetConfigForTest(t, before)

	t.Run("reload", func(t *testing.T) {
		if err := ReloadConfigForTest(t); err != nil {
			t.Fatalf("ReloadConfigForTest: %v", err)
		}
		if got := GetConfig().Server.NextRoomId; got != 424242 {
			t.Fatalf("inside the test NextRoomId = %d, want the loaded 424242", got)
		}
	})

	if got := GetConfig().Server.NextRoomId; got != 1234 {
		t.Fatalf("after the test NextRoomId = %d, want 1234 back: the reload leaked", got)
	}
}

// CONFIG_PATH still wins over the DataFiles-derived path.
func TestReloadConfig_ConfigPathEnvWins(t *testing.T) {
	isolateReloadGlobals(t)
	dir := t.TempDir()
	writeWorld(t, dir)
	mustWrite(t, filepath.Join(dir, "elsewhere.yaml"), "Server:\n  NextRoomId: 777\n")
	t.Chdir(dir)
	t.Setenv("CONFIG_PATH", "elsewhere.yaml")

	if err := ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	if got := GetConfig().Server.NextRoomId; got != 777 {
		t.Fatalf("NextRoomId = %d, want 777 from CONFIG_PATH", got)
	}
}
