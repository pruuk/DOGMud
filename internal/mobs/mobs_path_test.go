package mobs

import "testing"

type peekStep struct {
	exit string
	room int
}

func (s peekStep) ExitName() string { return s.exit }
func (s peekStep) RoomId() int      { return s.room }
func (s peekStep) Waypoint() bool   { return false }

// Peek shows the next step and does NOT advance: a tired walker must be able
// to look at a step it will not take yet (movement parity 4b).
func TestPathQueuePeekDoesNotAdvance(t *testing.T) {
	var p PathQueue
	if p.Peek() != nil {
		t.Fatal("an empty queue peeks nil")
	}
	p.SetPath([]PathRoom{peekStep{"north", 2}, peekStep{"east", 3}}, false)
	if got := p.Peek(); got == nil || got.RoomId() != 2 {
		t.Fatalf("Peek = %v, want the north step", got)
	}
	if p.Len() != 2 || p.Current() != nil {
		t.Fatalf("Peek advanced the queue: Len %d Current %v", p.Len(), p.Current())
	}
	if got := p.Next(); got.RoomId() != 2 {
		t.Fatalf("Next after Peek = %v, want the same step", got)
	}
}
