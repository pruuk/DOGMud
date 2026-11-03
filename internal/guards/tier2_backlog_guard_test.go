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

// The #375 phase 7 tier 2 pass (spec
// docs/superpowers/specs/completed/2026-11-02-quest-arc-phase7-tier2-design.md) cleared
// every loop finding on these quests. A finding on one is fixed in content,
// never listed.
var tier2Quests = map[int]bool{2: true, 16: true, 17: true, 18: true, 19: true, 20: true, 60: true, 61: true}

func tier2BacklogLines(t *testing.T, data []byte) []string {
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
		if id, err := strconv.Atoi(fields[1]); err == nil && tier2Quests[id] {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestTier2QuestsHaveNoBacklogLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt"))
	if err != nil {
		t.Fatalf("read backlog: %v", err)
	}
	if bad := tier2BacklogLines(t, data); len(bad) > 0 {
		t.Errorf("loop-backlog.txt names a tier 2 quest; fix the finding in content instead of listing it:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

func TestTier2QuestsHaveNoBacklogLines_GuardCanFail(t *testing.T) {
	fixture := []byte("# header\nhint-length 20 step start\none-action 61 step reach:none\nchain 73 mob 9465 topic q74_grant\n")
	if got := tier2BacklogLines(t, fixture); len(got) != 2 {
		t.Fatalf("guard saw %d tier 2 lines, want 2: %v", len(got), got)
	}
}
