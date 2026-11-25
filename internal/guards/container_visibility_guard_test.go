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

// #442: "a hidden container is visible only once this character has
// discovered it in this room" lives in one place, Room.ContainerVisibleTo
// (and Room.FindVisibleContainerByName). It was written out by hand in about
// a dozen commands and the GMCP room view, so a change to the rule had to be
// made in every copy and a missed copy leaked a container or refused one.
// This fails on a non-test file that reads a container's Hidden field within
// a few lines of a HasDiscovery call. A condition flag (conditions.Hidden)
// is not a container and is not matched.
func TestHiddenContainerRuleIsWrittenOnce(t *testing.T) {
	hiddenField := regexp.MustCompile(`\b[a-zA-Z_][a-zA-Z0-9_]*\.Hidden\b`)
	const window = 4
	var offenders []string
	scanned := 0
	for _, root := range []string{"internal", "modules"} {
		err := util.WalkDirTolerant(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			slash := filepath.ToSlash(path)
			if slash == "internal/rooms/rooms.go" {
				return nil // the helper itself
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			lines := strings.Split(string(raw), "\n")
			for i, line := range lines {
				code := line
				if at := strings.Index(code, "//"); at >= 0 {
					code = code[:at]
				}
				m := hiddenField.FindString(code)
				if m == "" || strings.HasPrefix(m, "conditions.") {
					continue
				}
				for j := max(0, i-window); j <= min(len(lines)-1, i+window); j++ {
					if strings.Contains(lines[j], "HasDiscovery(") {
						offenders = append(offenders, slash+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
						break
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 500 {
		t.Fatalf("scanned only %d files; the walk is not seeing the source tree", scanned)
	}
	for _, o := range offenders {
		t.Errorf("hand-written hidden-container check; use room.ContainerVisibleTo or room.FindVisibleContainerByName: %s", o)
	}
}
