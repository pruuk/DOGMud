package hooks

import (
	"os"
	"strings"
	"testing"
)

// #241: a declared arrest lapses when the player walks out of the room it
// was declared in. That rides on justice.LapseArrestStampOnMove hearing every
// RoomChange; unregistered, a player could step out and back and be hauled
// on the old declaration with no new word.
func TestArrestStampLapseIsRegistered(t *testing.T) {
	src, err := os.ReadFile("hooks.go")
	if err != nil {
		t.Fatalf("reading hooks.go: %v", err)
	}
	if !strings.Contains(string(src), "events.RegisterListener(events.RoomChange{}, justice.LapseArrestStampOnMove)") {
		t.Error("hooks.go no longer registers justice.LapseArrestStampOnMove on RoomChange")
	}
}
