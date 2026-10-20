package usercommands

import (
	"os"
	"strings"
	"testing"
)

// TestRoomEditPromptsAreRoomBound guards #375: a builder prompt that edits
// the room (or zone) the admin stands in must be RoomBound, so a follow step
// or forced move cancels it instead of letting the next answer edit the new
// room. Every StartPrompt in these files must be followed by RoomBound.
func TestRoomEditPromptsAreRoomBound(t *testing.T) {
	files := []string{"admin.room.go", "admin.room.exits.go", "admin.zone.go"}
	found := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		lines := strings.Split(string(src), "\n")
		for i, line := range lines {
			if !strings.Contains(line, "StartPrompt(") {
				continue
			}
			found++
			if i+1 >= len(lines) || !strings.Contains(lines[i+1], ".RoomBound = true") {
				t.Errorf("%s:%d: StartPrompt is not followed by RoomBound = true", f, i+1)
			}
		}
	}
	// The scan must be able to see the prompts it guards.
	if found < 4 {
		t.Fatalf("found %d StartPrompt calls, want at least 4", found)
	}
}
