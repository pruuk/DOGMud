package guards

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// repoRootPath is the repository root (the directory holding go.mod). TestMain
// sets it before any test runs and then changes the working directory to it.
var repoRootPath string

// realOverridesPath is the live world's engine-written overrides file. In a
// developer checkout that runs a server it holds NextRoomId and CurrentVersion,
// so no guard may touch it. Relative to the repo root.
const realOverridesPath = `_datafiles/world/dogmud/config-overrides.yaml`

// findRepoRoot walks up from dir to the first directory holding go.mod.
func findRepoRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above " + dir)
		}
		dir = parent
	}
}

// repoRoot returns the repository root. Every guard that builds a path from
// its own location uses this instead of runtime.Caller, which would now name
// internal/guards rather than the root.
func repoRoot(t testing.TB) string {
	t.Helper()
	if repoRootPath == "" {
		t.Fatal("repoRootPath is unset: TestMain did not run")
	}
	return repoRootPath
}

// testdataPath names a golden this package owns, under internal/guards/testdata.
// The working directory is the repo root (see TestMain), so a bare "testdata"
// would name the root's folder instead.
func testdataPath(name string) string {
	return filepath.Join(repoRootPath, "internal", "guards", "testdata", name)
}

// TestMain finds the repository root and makes it the working directory for
// every test in this package.
//
// Go runs a package's tests with the package directory as the working
// directory. These guards were written in the repo root, and so were the
// production loaders they call: configs.ReloadConfig reads
// `_datafiles/config.yaml`, and the shipped config names its world as
// `_datafiles/world/dogmud`, both relative to the working directory. Changing
// to the root once keeps every relative path in the guards, every allowlist
// keyed by a repo-relative path, and every loader meaning exactly what it
// meant in the root, and a walk of "internal" or "." still covers the whole
// repository.
//
// It also points CONFIG_PATH at a scratch file, as the root's TestMain does:
// room creation persists Server.NextRoomId through configs.SetEngineVal, which
// writes to CONFIG_PATH when set and otherwise to the real overrides file.
// After the run it fails the package if the real file was created or changed.
func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "guards: getwd:", err)
		os.Exit(1)
	}
	root, err := findRepoRoot(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "guards:", err)
		os.Exit(1)
	}
	repoRootPath = root
	if err := os.Chdir(root); err != nil {
		fmt.Fprintln(os.Stderr, "guards: chdir to the repo root:", err)
		os.Exit(1)
	}

	before, beforeErr := os.ReadFile(realOverridesPath)

	scratch, err := os.MkdirTemp("", "dogmud-guards-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "guards: mkdirtemp:", err)
		os.Exit(1)
	}
	// os.Setenv, not t.Setenv: this must hold for every test in the package.
	os.Setenv(`CONFIG_PATH`, scratch+`/config-overrides.yaml`)

	code := m.Run()

	os.RemoveAll(scratch)

	after, afterErr := os.ReadFile(realOverridesPath)
	if (beforeErr == nil) != (afterErr == nil) || !bytes.Equal(before, after) {
		fmt.Fprintf(os.Stderr, "guard tests wrote %s (existed before: %v); route config writes through CONFIG_PATH\n", realOverridesPath, beforeErr == nil)
		code = 1
	}
	os.Exit(code)
}
