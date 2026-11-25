package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// #720: a test that loads the shipped config with a bare configs.ReloadConfig
// leaks the shipped values into every later test in its binary, which then
// passes or fails by test order (AmbientListenerBudgetRounds 5 failed four
// TestSpeak* tests once). Tests load it through configs.ReloadConfigForTest,
// which restores the previous config at cleanup. The configs package's own
// tests of ReloadConfig are exempt: they isolate its globals themselves.
func TestTestsReloadTheShippedConfigOnlyThroughTheRestoringHelper(t *testing.T) {
	bareCall := regexp.MustCompile(`\bReloadConfig\(\)`)
	var offenders []string
	scanned := 0
	err := util.WalkDirTolerant(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			// .claude holds agent worktrees: whole second copies of the
			// repo whose exempt files sit at other paths.
			case ".git", ".claude", "node_modules", "_datafiles":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		slash := filepath.ToSlash(path)
		if strings.HasPrefix(slash, "internal/configs/") || slash == "internal/guards/config_reload_restore_guard_test.go" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		for i, line := range strings.Split(string(raw), "\n") {
			code := line
			if at := strings.Index(code, "//"); at >= 0 {
				code = code[:at]
			}
			if bareCall.MatchString(code) {
				offenders = append(offenders, slash+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 100 {
		t.Fatalf("scanned only %d test files; the walk is not seeing the repository", scanned)
	}
	for _, o := range offenders {
		t.Errorf("a test reloads the shipped config without restoring it; use configs.ReloadConfigForTest(t): %s", o)
	}
}
