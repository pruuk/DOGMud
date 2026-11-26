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
// A press is a command from the player's client (events.CmdFromClient), as a
// typed line and a Char.Action.Try are: it counts against the player's
// pending inputs (Network.MaxPendingInputs) and runs at the client pace, one
// per tick. At the cap it is not queued, and events.TryAddToQueue reports the
// refusal (refusal.InputQueueFull, sent by inputQueueHandler); the press
// carries CmdTyped, so it is reported as a refused typed line is. A refused
// press is never retried: the player may have moved by then, and the web
// client greys the deck while the queue is full (Char.Input.Queue).
//
// An id the server did not issue to this player for this room is refused as
// stale, with one line that never repeats the id, so a crafted, foreign or
// old id all read the same. The refusal queues events.RoomExitsChanged so
// Room.Info goes out again and the next press is current. That event merges
// per player while queued, and a stale press that finds one already waiting
// adds no second line, so a run of stale presses reads one line and builds
// Room.Info once.
//
// While a prompt is open a press does nothing: a prompt takes the next typed
// line as its answer, and a button is never an answer (D13).
//
// It reports whether the press was queued.
func pressButton(u *users.UserRecord, id string) bool {
	if u.GetPrompt() != nil {
		return false
	}
	line, ok := buttons.Resolve(u.UserId, u.Character.RoomId, id)
	if !ok {
		refresh := events.RoomExitsChanged{UserId: u.UserId}
		if !events.IsQueued(refresh) {
			refusal.StaleButton(u)
		}
		events.AddToQueue(refresh)
		return false
	}
	return u.CommandFlagged(line, events.CmdTyped|events.CmdFromClient|events.CmdButton)
}
