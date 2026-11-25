package gmcp

import (
	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// pressButton runs a touch button the player pressed (#781, the movement
// slice of the button protocol, owner rulings 7 and 8). The client sends only
// the id (Char.Button.Press {"id": ...}); buttons.Resolve turns it back into
// the line the player would have typed, and it is queued as typed input at
// the current turn, through the same per-user input queue as typing: the same
// parser, and a menu or a page stop treats it as typing. It also carries
// events.CmdButton, so world.processInput drops it if a prompt has opened by
// the time it runs.
//
// An id the server did not issue to this player for this room is refused as
// stale, with one line that never repeats the id, so a crafted, foreign or
// old id all read the same. The refusal queues events.RoomExitsChanged so
// Room.Info goes out again and the next press is current. That event merges
// per player while queued, and a stale press that finds one already waiting
// adds no second line, so a flood of stale presses reads one line and builds
// Room.Info once.
//
// While a prompt is open a press does nothing: a prompt takes the next typed
// line as its answer, and a button is never an answer (D13).
func pressButton(u *users.UserRecord, id string) {
	if u.GetPrompt() != nil {
		return
	}
	line, ok := buttons.Resolve(u.UserId, u.Character.RoomId, id)
	if !ok {
		refresh := events.RoomExitsChanged{UserId: u.UserId}
		if !events.IsQueued(refresh) {
			refusal.StaleButton(u)
		}
		events.AddToQueue(refresh)
		return
	}
	u.CommandFlagged(line, events.CmdTyped|events.CmdButton)
}
