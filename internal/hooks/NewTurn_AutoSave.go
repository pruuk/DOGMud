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
// Chunk 3.6b-1 changed the SHAPE of that reporting, and this is the part most
// at risk of quietly regressing finding 35.
//
// Autosave used to be three synchronous stages, so each one could announce its
// own outcome the moment it finished. It is now:
//
//	prepare tick:   one atomic snapshot of every room and user  ->  pending set
//	every tick:     commit AutosaveWritesPerTick entries from that set
//
// The writes are NOT finished when this hook returns on the prepare tick, so
// the completion message cannot be emitted there. It moves to the drain that
// empties the queue, which is the only moment the outcome is actually known.
// Announcing success at prepare time would be finding 35 all over again, in a
// new costume.
//
// One consequence, accepted deliberately: a cycle drains over roughly 23
// seconds at 1386 rooms and the shipped budget. So players see one "Saving..."
// line and one completion line well after it, rather than the old per-stage
// chatter. Plugins keep their own synchronous line because they genuinely do
// finish immediately.
//

// preparedAt records when the current cycle's snapshot was taken, so the
// completion line can report how long the whole cycle actually took rather than
// just how long the last write took.
var (
	preparedAt   time.Time
	cyclePrepMs  int64
	cycleWaiting bool
)

func AutoSave(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.NewTurn)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "NewTurn", "Actual Type", e.Type())
		return events.Cancel
	}

	// Drain FIRST, on every turn, not only on autosave turns.
	//
	// Order matters: draining before a possible Supersede means the supersede
	// check (G3) sees the true remaining backlog rather than a backlog this
	// turn was about to reduce.
	drainPendingWrites()

	if evt.TurnNumber%uint64(configs.GetTimingConfig().TurnsPerAutoSave()) == 0 {
		prepareAutosaveCycle()
	}

	return events.Continue
}

// drainPendingWrites commits a bounded number of entries and, on the drain that
// empties the queue, reports the cycle.
func drainPendingWrites() {

	q := AutosaveQueue()
	if q.Pending() == 0 && !cycleWaiting {
		return
	}

	perTick := int(configs.GetTimingConfig().AutosaveWritesPerTick)

	start := time.Now()
	res := q.Drain(perTick)
	if res.Committed+res.Failed > 0 {
		util.TrackTime(`AutoSave.Commit`, time.Since(start).Seconds())
	}

	if res.Cycle == nil {
		return
	}
	cycleWaiting = false

	// The cycle is finished and only NOW is the outcome known.
	total := time.Since(preparedAt)
	if err := res.Cycle.Err(); err != nil {
		mudlog.Error("AutoSave", "stage", "commit", "error", err)
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

	// One structured line per autosave cycle. prepareMs is the part that still
	// holds the world lock in one block and is therefore the player-visible
	// stall; turnsDelayed is that cost expressed in turns nobody could act.
	// drainSeconds is how long the amortised tail ran, during which the game was
	// responsive but MainWorker was busier than usual.
	turnMs := int64(configs.GetTimingConfig().TurnMs)
	turnsDelayed := int64(0)
	if turnMs > 0 {
		turnsDelayed = cyclePrepMs / turnMs
	}
	mudlog.Info("AutoSave",
		"prepareMs", cyclePrepMs,
		"turnsDelayed", turnsDelayed,
		"drainSeconds", int64(total.Seconds()),
		"written", res.Cycle.Succeeded,
		"failed", res.Cycle.Failed,
		"cancelled", res.Cycle.Cancelled,
		"total", res.Cycle.Total,
	)
}

// prepareAutosaveCycle takes the atomic snapshot and hands it to the queue.
func prepareAutosaveCycle() {

	totalTimeStart := time.Now()
	defer func() {
		util.TrackTime(`AutoSave`, time.Since(totalTimeStart).Seconds())
	}()

	loadedRooms := rooms.LoadedRoomCount()
	activeUsers := len(users.GetAllActiveUsers())

	events.AddToQueue(events.Broadcast{Text: `Saving...`})

	//////////////////////////////////////////
	// SNAPSHOT ROOMS AND USERS — ONE PASS
	//////////////////////////////////////////
	prepStart := time.Now()
	writes, prepErr := PrepareAutosaveSet()
	prepDur := time.Since(prepStart)
	util.TrackTime(`AutoSave.Prepare`, prepDur.Seconds())

	if prepErr != nil {
		// Some entities could not be snapshotted. The rest still can be, so the
		// cycle continues with what prepared — but the failure is reported now,
		// because it happened now and will not be visible in the commit results.
		mudlog.Error("AutoSave", "stage", "prepare", "error", prepErr, "prepared", len(writes))
	}

	preparedAt = time.Now()
	cyclePrepMs = prepDur.Milliseconds()
	cycleWaiting = true
	AutosaveQueue().Supersede(writes)

	//////////////////////////////////////////
	// SAVE ALL PLUGINS
	//////////////////////////////////////////
	// Still synchronous: plugin saves measured 0-1ms in 3.6a, so amortising
	// them would add machinery for nothing.
	pluginsStart := time.Now()
	pluginErr := plugins.Save()
	pluginsDur := time.Since(pluginsStart)
	util.TrackTime(`AutoSave.Plugins`, pluginsDur.Seconds())
	if pluginErr != nil {
		mudlog.Error("AutoSave", "stage", "plugins", "error", pluginErr)
		events.AddToQueue(events.Broadcast{
			Text:            `Saved with errors.` + term.CRLFStr,
			SkipLineRefresh: true,
		})
	}

	mudlog.Info("AutoSave",
		"stage", "prepared",
		"prepareMs", prepDur.Milliseconds(),
		"pendingWrites", len(writes),
		"loadedRooms", loadedRooms,
		"activeUsers", activeUsers,
		"pluginsMs", pluginsDur.Milliseconds(),
	)
}
