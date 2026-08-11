package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/savequeue"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The autosave prepare pass (roadmap chunk 3.6b-1, review finding 36).
//
// This lives in hooks rather than beside the queue because it must call into
// BOTH internal/rooms and internal/users, and both of those import
// internal/savequeue. Putting it in savequeue would be an import cycle.

// autosaveQueue is the single process-wide pending set. Rooms owns the
// allocation only because something has to; users is pointed at the same one
// during init, and that sharing is load bearing rather than incidental (see
// PrepareAutosaveSet).
var autosaveQueue = rooms.AutosaveQueue()

func init() {
	users.SetAutosaveQueue(autosaveQueue)
	plugins.SetAutosaveQueue(autosaveQueue)
}

// AutosaveQueue exposes the pending set for shutdown and copyover flushes.
func AutosaveQueue() *savequeue.Queue { return autosaveQueue }

// PrepareAutosaveSet snapshots everything that needs persisting, in ONE pass.
//
// # Why one pass, and why it must not be split
//
// This is the single most important constraint in the amortised-autosave
// design, and it is the one most likely to be lost to a later "optimisation".
//
// Persisting a player picking up an item touches TWO files: the room loses the
// item, the character gains it. If the room is snapshotted before the pickup
// and the character after, the item is in BOTH files and reloading duplicates
// it. Reverse the order and the item is in NEITHER and it is destroyed. The
// same shape covers dropping, corpse looting, container contents, shop
// purchases and guild treasury transfers.
//
// A single pass under one lock hold is a consistent point-in-time view, so it
// cannot tear. Chunking it, or preparing rooms and then users, reintroduces
// exactly the hazard the atomic pass exists to prevent -- just with a shorter
// window, which makes it rarer and therefore harder to diagnose.
//
// This is why the pause floor for 3.6b-1 is the whole prepare pass rather than
// a few milliseconds. Removing that floor is 3.6b-2's job (a dirty set, so the
// pass is proportional to activity instead of world size), NOT something to
// solve here by cutting the pass into pieces.
//
// Caller must hold the world lock for the entire call.
func PrepareAutosaveSet() ([]savequeue.PendingWrite, error) {
	roomWrites, roomErr := rooms.PrepareAllInstanceWrites()
	userWrites, userErr := users.PrepareAllUserWrites()
	pluginWrites, pluginErr := plugins.PrepareAll()

	writes := make([]savequeue.PendingWrite, 0, len(roomWrites)+len(userWrites)+len(pluginWrites))
	writes = append(writes, roomWrites...)
	writes = append(writes, userWrites...)
	writes = append(writes, pluginWrites...)

	// A prepare failure for one entity must not discard the rest of the
	// snapshot: the other players' progress is still worth persisting. Report
	// the first problem and hand back everything that did prepare.
	if roomErr != nil {
		return writes, roomErr
	}
	if userErr != nil {
		return writes, userErr
	}
	return writes, pluginErr
}
