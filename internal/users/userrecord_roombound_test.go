package users

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
)

func TestCloseMenusForRoomChange_ClearsARoomBoundPrompt(t *testing.T) {
	u := menuUser()
	builderPrompt(u)
	u.GetPrompt().RoomBound = true
	events.DrainQueuedMessagesForTest(u.UserId)

	u.CloseMenusForRoomChange()
	if u.GetPrompt() != nil {
		t.Fatal("a room-bound prompt survived a room change")
	}
	msgs := strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "")
	if !strings.Contains(msgs, "You left the room, so the edit is cancelled.") {
		t.Fatalf("messages %q", msgs)
	}
}

func TestCloseMenusForRoomChange_KeepsAModalPromptNotBoundToTheRoom(t *testing.T) {
	u := menuUser()
	builderPrompt(u)
	events.DrainQueuedMessagesForTest(u.UserId)

	u.CloseMenusForRoomChange()
	if u.GetPrompt() == nil {
		t.Fatal("a modal prompt not bound to the room was cleared")
	}
	if msgs := events.DrainQueuedMessagesForTest(u.UserId); len(msgs) != 0 {
		t.Fatalf("messages %q", msgs)
	}
}
