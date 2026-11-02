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

// The #375 phase 7 Pothole Coulee pass (spec
// docs/superpowers/specs/2026-10-30-quest-arc-phase7-pothole-coulee-design.md,
// "Done when") cleared every loop finding on its quests. This guard keeps it
// that way: no loop-backlog.txt line may name quest 28 to 59, 76, 88, 89 or
// 90 to 101. A finding on one of them is fixed in content, never listed.

func couleeQuest(id int) bool {
	return (id >= 28 && id <= 59) || id == 76 || id == 88 || id == 89 || (id >= 90 && id <= 101)
}

// couleeBacklogLines returns the backlog lines ("rule questId where") that
// name a Coulee quest.
func couleeBacklogLines(t *testing.T, data []byte) []string {
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
		if id, err := strconv.Atoi(fields[1]); err == nil && couleeQuest(id) {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestCouleeQuestsHaveNoBacklogLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud", "quests", "loop-backlog.txt"))
	if err != nil {
		t.Fatalf("read backlog: %v", err)
	}
	if bad := couleeBacklogLines(t, data); len(bad) > 0 {
		t.Errorf("loop-backlog.txt names a Pothole Coulee quest; fix the finding in content instead of listing it:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

// TestCouleeQuestsHaveNoBacklogLines_GuardCanFail proves the guard sees a
// Coulee line and ignores another zone's.
func TestCouleeQuestsHaveNoBacklogLines_GuardCanFail(t *testing.T) {
	fixture := []byte("# header\n" +
		"end 89 quest 89 trigger room_enter room=468 grants=89-end\n" +
		"walk-dead-end 101 step lifted\n" +
		"chain 73 mob 9465 topic q74_grant\n" +
		"one-action 7 mob 89 topic dornhandoff:prev\n")
	if got := couleeBacklogLines(t, fixture); len(got) != 2 {
		t.Fatalf("guard saw %d Coulee lines, want 2 (quests 89 and 101): %v", len(got), got)
	}
}
