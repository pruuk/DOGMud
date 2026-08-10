// Round ticks for players
package hooks

import (
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

//
// Autosaves users/rooms every so often.
//
// Every stage below reports honestly. This hook used to broadcast "Done." to
// every connected player after each stage regardless of outcome, while the
// save functions themselves either returned nil unconditionally or returned
// nothing at all — so a total autosave failure was indistinguishable from a
// clean one, to players AND to the operator (review finding 35).
//

func AutoSave(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.NewTurn)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "NewTurn", "Actual Type", e.Type())
		return events.Cancel
	}

	if evt.TurnNumber%uint64(configs.GetTimingConfig().TurnsPerAutoSave()) == 0 {

		totalTimeStart := time.Now()
		defer func() {
			util.TrackTime(`AutoSave`, time.Since(totalTimeStart).Seconds())
		}()

		//////////////////////////////////////////
		// SAVE ALL USERS
		//////////////////////////////////////////
		events.AddToQueue(events.Broadcast{Text: `Saving users...`})

		if err := users.SaveAllUsers(true); err != nil {
			mudlog.Error("AutoSave", "stage", "users", "error", err)
			events.AddToQueue(events.Broadcast{
				Text:            `Saved with errors.` + term.CRLFStr,
				SkipLineRefresh: true,
			})
		} else {
			events.AddToQueue(events.Broadcast{
				Text:            `Done.` + term.CRLFStr,
				SkipLineRefresh: true,
			})
		}

		//////////////////////////////////////////
		// SAVE ALL ROOMS
		//////////////////////////////////////////
		events.AddToQueue(events.Broadcast{Text: `Saving rooms...`})

		if err := rooms.SaveAllRooms(); err != nil {
			mudlog.Error("AutoSave", "stage", "rooms", "error", err)
			events.AddToQueue(events.Broadcast{
				Text:            `Saved with errors.` + term.CRLFStr,
				SkipLineRefresh: true,
			})
		} else {
			events.AddToQueue(events.Broadcast{
				Text:            `Done.` + term.CRLFStr,
				SkipLineRefresh: true,
			})
		}

		//////////////////////////////////////////
		// SAVE ALL PLUGINS
		//////////////////////////////////////////
		events.AddToQueue(events.Broadcast{Text: `Saving other...`})
		// Save plugin states if applicable
		if err := plugins.Save(); err != nil {
			mudlog.Error("AutoSave", "stage", "plugins", "error", err)
			events.AddToQueue(events.Broadcast{
				Text:            `Saved with errors.` + term.CRLFStr,
				SkipLineRefresh: true,
			})
		} else {
			events.AddToQueue(events.Broadcast{
				Text:            `Done.` + term.CRLFStr,
				SkipLineRefresh: true,
			})
		}

	}

	return events.Continue
}
