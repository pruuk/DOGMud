package guards

import (
	"sort"
	"strings"
	"testing"
)

// TestRoomMappedRaisesExplore holds every caller of
// actions.MarkRoomMappedIfSeen to also call questengine.NotifyRoomMapped
// (#375 phase 3b slice 4). A room joining the map is one of the two moments
// an explore step is read; a new mapping site that skips the call leaves the
// step unfinished until the player maps another room of the area. The
// pairing is checked per file: the call goes where the true result is seen.
func TestRoomMappedRaisesExplore(t *testing.T) {
	calls, _ := walkGoCalls(t, map[string]bool{"MarkRoomMappedIfSeen": true, "NotifyRoomMapped": true})
	marks, notifies := map[string]bool{}, map[string]bool{}
	for _, c := range calls {
		if c.file == "internal/actions/map_visit.go" {
			continue // the definition's own doc
		}
		if c.name == "MarkRoomMappedIfSeen" {
			marks[c.file] = true
		} else {
			notifies[c.file] = true
		}
	}
	if len(marks) < 2 {
		t.Fatalf("the scan saw %d MarkRoomMappedIfSeen callers, want at least 2 (go.go, gmcp.Room.go): the guard cannot find calls", len(marks))
	}
	var missing []string
	for f := range marks {
		if !notifies[f] {
			missing = append(missing, f)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("these files map rooms without questengine.NotifyRoomMapped on a true result:\n  %s",
			strings.Join(missing, "\n  "))
	}
}
