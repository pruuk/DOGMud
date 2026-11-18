package gmcp

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Char.Input.Done tells the web client the server has finished one of its
// typed lines: {"seq": n, "menu": bool}. seq is the line's number (both ends
// count text frames from the socket's open); menu is whether that left a game
// menu or prompt open, or left a long reply waiting at a page stop (#842),
// where the player is expected to press Enter. The phone keyboard waits for
// it and stays up when a menu opened (plan 6, spec 2026-11-14 R2, R6). It
// never carries the line's text, so an answer typed into the password prompt
// is not sent back (R7). Web client only: telnet GMCP clients get nothing new.
func init() {
	registerInputDoneListener()
}

// registerInputDoneListener registers inputDoneHandler as a Last listener, so
// it runs after hooks.Pager_ReplyEnd (registered later, from main.go) has cut
// the line's reply into pages, and the menu flag sees those pages. Last only
// orders the listeners of this one event; the frame is still queued in the
// same InputDone pop, at the same priority, as before.
func registerInputDoneListener() {
	events.RegisterListener(events.InputDone{}, inputDoneHandler, events.Last)
}

func inputDoneHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.InputDone)
	// Seq 0: a telnet line or a web client's macro line (#842). Only the
	// pager needs those; the web client counts only its own lines.
	if !ok || evt.UserId == 0 || evt.Seq == 0 {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}
	connId := user.ConnectionId()
	if !isWebClientConnection(connId) || !isGMCPEnabled(connId) {
		return events.Continue
	}
	// Default priority 0: InputDone popped at events.InputDonePriority, after
	// the command's text and menu events, and the frame follows their frames
	// and the page Pager_ReplyEnd wrote directly; it pops before the prompt
	// redraw at 100.
	events.AddToQueue(GMCPOut{
		UserId:  evt.UserId,
		Module:  `Char.Input.Done`,
		Payload: map[string]any{"seq": evt.Seq, "menu": user.MenuOrPromptOpen() || user.PagesWaiting()},
	})
	return events.Continue
}
