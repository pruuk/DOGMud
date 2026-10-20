package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// MenuCloseOnRoomChange closes a player's game menu and any non-modal prompt
// when they change rooms: walked, fled, or pulled by a leader (owner rule
// 2026-10-20, #375). A follow pull has already closed it with the follow
// lines, so this finds nothing open then.
func MenuCloseOnRoomChange(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.RoomChange)
	if !ok || evt.UserId == 0 || evt.FromRoomId == evt.ToRoomId {
		return events.Continue
	}
	if u := users.GetByUserId(evt.UserId); u != nil {
		u.CloseMenusForRoomChange()
	}
	return events.Continue
}
