package buttons

import (
	"reflect"
	"testing"
)

// Ruling 4: a direction tried in the dark keeps its mark until the player
// leaves the room or the room is lit for them.
func TestDarkMarks(t *testing.T) {
	t.Cleanup(func() { Forget(1) })
	if !RecordDarkMark(1, 5, "north", MarkWall) {
		t.Fatal("a new mark must report a change")
	}
	if RecordDarkMark(1, 5, "north", MarkWall) {
		t.Error("the same mark again is no change")
	}
	if !RecordDarkMark(1, 5, "east", MarkLocked) {
		t.Fatal("a second mark must report a change")
	}
	if RecordDarkMark(1, 5, "northeast", MarkWall) {
		t.Error("only the six T directions are marked")
	}
	want := map[string]string{"north": MarkWall, "east": MarkLocked}
	if got := DarkMarks(1, 5, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("marks %v, want %v", got, want)
	}
	if got := DarkMarks(1, 6, true); len(got) != 0 {
		t.Fatalf("another room shows %v", got)
	}
	if got := DarkMarks(1, 5, true); len(got) != 0 {
		t.Fatalf("reading another room cleared nothing: %v", got)
	}

	RecordDarkMark(1, 5, "north", MarkWall)
	if got := DarkMarks(1, 5, false); len(got) != 0 {
		t.Fatalf("a lit room shows %v", got)
	}
	if got := DarkMarks(1, 5, true); len(got) != 0 {
		t.Fatalf("light must clear the marks, still %v", got)
	}

	RecordDarkMark(1, 5, "north", MarkWall)
	ClearDarkMarks(1)
	if got := DarkMarks(1, 5, true); len(got) != 0 {
		t.Fatalf("ClearDarkMarks left %v", got)
	}
}
