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

// #896: a test that loads the shipped species roster with a bare
// species.LoadDataFiles() leaves it in place for every later test in its
// binary. sell_rusk_dagger_test.go did, so on `go test -count=2` the next
// pass's characters.New() met the shipped species 0 (ghostly spirit, ten
// disabled slots) and six equipment tests counted ten phantom items. Tests
// load the roster through species.LoadForTest(t), which restores it.
//
// Exempt: internal/species itself, and internal/guards, whose binary loads
// the shipped world for every test on purpose.
func TestTestsLoadTheSpeciesRosterOnlyThroughTheRestoringHelper(t *testing.T) {
	bare := regexp.MustCompile(`\bspecies\.LoadDataFiles\(\)`)
	var offenders []string
	scanned := 0
	for _, root := range []string{"internal", "modules"} {
		err := util.WalkDirTolerant(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			slash := filepath.ToSlash(path)
			if strings.HasPrefix(slash, "internal/species/") || strings.HasPrefix(slash, "internal/guards/") {
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
				if bare.MatchString(code) {
					offenders = append(offenders, slash+":"+strconv.Itoa(i+1))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 500 {
		t.Fatalf("scanned only %d test files; the walk is not seeing the source tree", scanned)
	}
	for _, o := range offenders {
		t.Errorf("%s loads the species roster without restoring it; use species.LoadForTest(t)", o)
	}
}
