package gmcp

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Char.Input.Queue tells a client its player's pending inputs filled or freed
// (Network.MaxPendingInputs, owner ruling 2026-11-25): {"depth": n, "full":
// bool}. It is sent only on a change between full and not full, never once
// per command. The web client greys out the controls that send commands while
// full is true.
type GMCPInput_Queue struct {
	Depth int  `json:"depth"`
	Full  bool `json:"full"`
}

// actionQueueFullReason answers a Char.Action.Try refused because the
// player's pending inputs are full. It goes with "deferred", so the client
// tries the action again later.
const actionQueueFullReason = `queue full`

func init() {
	events.RegisterListener(events.InputQueueChanged{}, inputQueueHandler)
}

// inputQueueHandler sends Char.Input.Queue to a GMCP client when the queue
// fills or frees. A client other than the web client, which is the only one
// that reads Char.Input.Queue today, gets one plain line on the first command
// refused in a full spell instead (events.InputQueueChanged.Refused).
func inputQueueHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.InputQueueChanged)
	if !ok {
		mudlog.Error("Event", "Expected Type", "InputQueueChanged", "Actual Type", e.Type())
		return events.Cancel
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}
	connId := user.ConnectionId()

	if evt.Refused {
		if !isWebClientConnection(connId) {
			refusal.InputQueueFull(user)
		}
		return events.Continue
	}

	if !isGMCPEnabled(connId) {
		return events.Continue
	}
	events.AddToQueue(GMCPOut{
		UserId:  evt.UserId,
		Module:  `Char.Input.Queue`,
		Payload: GMCPInput_Queue{Depth: evt.Depth, Full: evt.Full},
	})
	return events.Continue
}
