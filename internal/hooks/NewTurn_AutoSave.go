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

		// Chunk 3.6a instrumentation (finding 36). Autosave runs inside the
		// NewTurn event with the world lock held, so every millisecond here is
		// a millisecond no player acts. Capture the loaded-set sizes and the
		// per-stage cost so the pause can be budgeted against evidence rather
		// than argued about.
		loadedRooms := rooms.LoadedRoomCount()
		activeUsers := len(users.GetAllActiveUsers())
		var usersDur, roomsDur, pluginsDur time.Duration

		//////////////////////////////////////////
		// SAVE ALL USERS
		//////////////////////////////////////////
		events.AddToQueue(events.Broadcast{Text: `Saving users...`})

		usersStart := time.Now()
		userErr := users.SaveAllUsers(true)
		usersDur = time.Since(usersStart)
		util.TrackTime(`AutoSave.Users`, usersDur.Seconds())
		if err := userErr; err != nil {
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

		roomsStart := time.Now()
		roomErr := rooms.SaveAllRooms()
		roomsDur = time.Since(roomsStart)
		util.TrackTime(`AutoSave.Rooms`, roomsDur.Seconds())
		if err := roomErr; err != nil {
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
		pluginsStart := time.Now()
		pluginErr := plugins.Save()
		pluginsDur = time.Since(pluginsStart)
		util.TrackTime(`AutoSave.Plugins`, pluginsDur.Seconds())
		if err := pluginErr; err != nil {
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

		// One structured line per autosave. turnsDelayed is the player-visible
		// cost: the world lock is held for the whole pause, so this many turns
		// could not be processed. TurnMs is 50 by default, so 20 turns is one
		// second of frozen game.
		total := time.Since(totalTimeStart)
		turnMs := int64(configs.GetTimingConfig().TurnMs)
		turnsDelayed := int64(0)
		if turnMs > 0 {
			turnsDelayed = total.Milliseconds() / turnMs
		}
		mudlog.Info("AutoSave",
			"totalMs", total.Milliseconds(),
			"turnsDelayed", turnsDelayed,
			"loadedRooms", loadedRooms,
			"activeUsers", activeUsers,
			"usersMs", usersDur.Milliseconds(),
			"roomsMs", roomsDur.Milliseconds(),
			"pluginsMs", pluginsDur.Milliseconds(),
		)
	}

	return events.Continue
}
