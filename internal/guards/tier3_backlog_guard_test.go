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

// The #375 phase 7 tier 3 pass (spec
// docs/superpowers/specs/2026-11-02-quest-arc-phase7-tier3-design.md) cleared
// every loop finding on these quests. A finding on one is fixed in content,
// never listed. 3b adds the quests it retires.
var tier3Quests = map[int]bool{73: true, 74: true}

func tier3BacklogLines(t *testing.T, data []byte) []string {
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
		if id, err := strconv.Atoi(fields[1]); err == nil && tier3Quests[id] {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestTier3QuestsHaveNoBacklogLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt"))
	if err != nil {
		t.Fatalf("read backlog: %v", err)
	}
	if bad := tier3BacklogLines(t, data); len(bad) > 0 {
		t.Errorf("loop-backlog.txt names a tier 3 quest; fix the finding in content instead of listing it:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

func TestTier3QuestsHaveNoBacklogLines_GuardCanFail(t *testing.T) {
	fixture := []byte("# header\nhint-length 74 step record\nchain 73 mob 9465 topic q74_grant\none-action 61 step reach:none\n")
	if got := tier3BacklogLines(t, fixture); len(got) != 2 {
		t.Fatalf("guard saw %d tier 3 lines, want 2: %v", len(got), got)
	}
}
