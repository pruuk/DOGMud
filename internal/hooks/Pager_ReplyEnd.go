package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Pager_ReplyEnd ends the reply to one typed line (#842). InputDone pops
// after every event that writes the line's text or opens a menu, so the
// whole reply has been held (Message_SendMessage) by now; the queue closed
// the reply window as it popped InputDone. It sends the reply whole, or its
// first page with the stop line drawn as the prompt.
func Pager_ReplyEnd(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.InputDone)
	if !ok {
		mudlog.Error("Event", "Expected Type", "InputDone", "Actual Type", e.Type())
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}
	if out := user.EndReply(); out != `` {
		connections.SendTo([]byte(user.PromptErase()+out), user.ConnectionId())
		events.AddToQueue(events.RedrawPrompt{UserId: user.UserId}, 100)
	}
	return events.Continue
}
