package actions

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// RenderRoster renders the room roster ("Also here: ...", the
// descriptions/who template) for userId and wraps it to that user's line
// width (80 when there is no user record).
//
// The roster goes out as CategoryRoomDescription (look) or CategorySystem
// (search), and the messaging pipeline wraps neither: one carries the
// side-by-side description and minimap block, the other tables. So a busy
// room printed one line well past 80 columns (#430). Every roster send goes
// through here.
func RenderRoster(details rooms.RoomTemplateDetails, userId int) string {
	text, _ := templates.Process("descriptions/who", details, userId)
	if text == "" {
		return ""
	}
	return messaging.WrapAnsi(text, users.GetByUserId(userId).GetLineWidth())
}
