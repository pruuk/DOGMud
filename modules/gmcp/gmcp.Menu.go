package gmcp

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Char.Menu tells the client whether a game menu or prompt is open, so the
// web client queues ticks and triggers until it closes (#375). Telnet
// clients with GMCP can read the same signal in their own scripts.
func init() {
	events.RegisterListener(events.MenuChanged{}, menuChangedHandler)
}

func menuChangedHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.MenuChanged)
	if !ok || evt.UserId == 0 {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil || !isGMCPEnabled(user.ConnectionId()) {
		return events.Continue
	}
	events.AddToQueue(GMCPOut{
		UserId:  evt.UserId,
		Module:  `Char.Menu`,
		Payload: map[string]bool{"open": evt.Open},
	})
	return events.Continue
}
