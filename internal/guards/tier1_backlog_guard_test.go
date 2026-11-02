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

// The #375 phase 7 tier 1 pass (spec
// docs/superpowers/specs/completed/2026-11-02-quest-arc-phase7-tier1-design.md) cleared
// every loop finding on quests 4 to 15. A finding on one of them is fixed in
// content, never listed.
func tier1BacklogLines(t *testing.T, data []byte) []string {
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
		if id, err := strconv.Atoi(fields[1]); err == nil && id >= 4 && id <= 15 {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestTier1QuestsHaveNoBacklogLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt"))
	if err != nil {
		t.Fatalf("read backlog: %v", err)
	}
	if bad := tier1BacklogLines(t, data); len(bad) > 0 {
		t.Errorf("loop-backlog.txt names a tier 1 quest; fix the finding in content instead of listing it:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

func TestTier1QuestsHaveNoBacklogLines_GuardCanFail(t *testing.T) {
	fixture := []byte("# header\none-action 14 step report:none\nhint-length 6 step start\nchain 73 mob 9465 topic q74_grant\n")
	if got := tier1BacklogLines(t, fixture); len(got) != 2 {
		t.Fatalf("guard saw %d tier 1 lines, want 2: %v", len(got), got)
	}
}
