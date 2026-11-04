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

// The #375 phase 7 tier 4 pass (Greenford, Cascade Pass Road and the Eastern
// Highlands) was done by the disc line redesign, #667
// (docs/superpowers/specs/2026-11-03-disc-line-design.md). A loop finding on
// one of its quests is fixed in content, never listed.
var tier4Quests = map[int]bool{75: true, 76: true, 77: true}

func tier4BacklogLines(t *testing.T, data []byte) []string {
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
		if id, err := strconv.Atoi(fields[1]); err == nil && tier4Quests[id] {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestTier4QuestsHaveNoBacklogLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt"))
	if err != nil {
		t.Fatalf("read backlog: %v", err)
	}
	if bad := tier4BacklogLines(t, data); len(bad) > 0 {
		t.Errorf("loop-backlog.txt names a tier 4 quest; fix the finding in content instead of listing it:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

func TestTier4QuestsHaveNoBacklogLines_GuardCanFail(t *testing.T) {
	fixture := []byte("# header\nhint-length 76 step journal\nchain 77 mob 9553 topic truth_in\none-action 63 step report:none\n")
	if got := tier4BacklogLines(t, fixture); len(got) != 2 {
		t.Fatalf("guard saw %d tier 4 lines, want 2: %v", len(got), got)
	}
}
