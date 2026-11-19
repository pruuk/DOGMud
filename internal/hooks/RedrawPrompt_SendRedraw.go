package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Checks whether their level is too high for a guide
func RedrawPrompt_SendRedraw(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.RedrawPrompt)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "RedrawPrompt", "Actual Type", e.Type())
		return events.Cancel
	}

	if user := users.GetByUserId(evt.UserId); user != nil {

		// Building the prompt counts the old one as erased (#922); a skipped
		// redraw below changes nothing on screen, so it restores the count.
		rowsOnScreen := user.PromptRows()
		newCmdPrompt := user.GetCommandPrompt()

		// When an interactive question is pending (character creation, confirm
		// prompts, password entry), the command prompt IS the full question text.
		// Message/broadcast senders fire RedrawPrompt WITHOUT OnlyIfChanged, so
		// every incidental line (join banners, periodic tips, channel chatter)
		// would re-print the entire multi-line question — the "route question
		// shows up twice/three times" report (2026-07-17). Re-rendering an
		// UNCHANGED pending question is never desirable, so force the change-dedup
		// in that case regardless of the event flag. Normal gameplay prompts are
		// unaffected (no pending question → behaves exactly as before).
		forceDedup := false
		if p := user.GetPrompt(); p != nil && p.GetNextQuestion() != nil {
			forceDedup = true
		}

		if evt.OnlyIfChanged || forceDedup {

			oldCmdPrompt := user.GetTempData(users.PromptDrawnTempKey)

			// If the prompt hasn't changed, skip redrawing
			// (compared without its leading erase, which depends on what the
			// screen showed before, #922)
			if s, ok := oldCmdPrompt.(string); ok && s == users.WithoutPromptErase(newCmdPrompt) {
				user.RestorePromptRows(rowsOnScreen)
				return events.Continue
			}

			// save the new prompt for next time we want to check
			user.SetTempData(users.PromptDrawnTempKey, users.WithoutPromptErase(newCmdPrompt))

		}

		pTxt := templates.AnsiParse(newCmdPrompt)
		connections.SendTo([]byte(pTxt), user.ConnectionId())

	}

	return events.Continue
}
