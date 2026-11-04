package guards

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questloop"
)

// The #375 phase 7 tier 5 pass (New Plymouth). A loop finding on one of its
// quests is fixed in content, never listed.
var tier5Quests = map[int]bool{63: true, 65: true, 66: true, 67: true}

func tier5BacklogLines(t *testing.T, data []byte) []string {
	t.Helper()
	b, err := questloop.ParseBacklog(data)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for line := range b {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if id, err := strconv.Atoi(fields[1]); err == nil && tier5Quests[id] {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestTier5QuestsHaveNoBacklogLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt"))
	if err != nil {
		t.Fatalf("read backlog: %v", err)
	}
	if bad := tier5BacklogLines(t, data); len(bad) > 0 {
		t.Errorf("loop-backlog.txt names a tier 5 quest; fix the finding in content instead of listing it:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

func TestTier5QuestsHaveNoBacklogLines_GuardCanFail(t *testing.T) {
	fixture := []byte("# header\nhint-length 66 step start\none-action 63 step report:none\nchain 76 mob 9553 topic x\n")
	if got := tier5BacklogLines(t, fixture); len(got) != 2 {
		t.Fatalf("guard saw %d tier 5 lines, want 2: %v", len(got), got)
	}
}
