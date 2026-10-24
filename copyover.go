package main

import (
	"fmt"
	"os"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/apiframework"
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/caravan"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/copyover"
	"github.com/GoMudEngine/GoMud/internal/forager"
	"github.com/GoMudEngine/GoMud/internal/hooks"
	"github.com/GoMudEngine/GoMud/internal/knowledge"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/warehouse"
)

// triggerCopyoverLocked acquires the world lock, then performs a copyover.
//
// Use this from anywhere OUTSIDE MainWorker's tick loop — notably the SIGUSR1
// handler, which runs on its own goroutine. Without the lock, that handler
// serialised rooms, users and shops to disk while the tick loop was concurrently
// mutating them: at best a torn save that corrupts the copyover handoff, at
// worst a "concurrent map iteration and map write" fatal error during YAML
// marshal. The graceful-shutdown path already wraps the identical save sequence
// in this lock (world.go).
func triggerCopyoverLocked() error {
	util.LockMud()
	defer util.UnlockMud()

	return triggerCopyover()
}

// triggerCopyover flushes all state to disk and re-execs the same binary,
// handing off active telnet sockets so players stay connected. It is a no-op on
// Windows (copyover.Execute returns an error there).
//
// The caller MUST already hold the world lock. The admin `copyover` command
// satisfies this because commands execute inside MainWorker's locked EventLoop
// (world.go: LockMud / EventLoop / UnlockMud), and util's mutex is not
// reentrant — locking here would deadlock that path. Callers outside the tick
// loop must use triggerCopyoverLocked instead.
func triggerCopyover() error {
	binaryPath, err := os.Executable()
	if err != nil {
		return err
	}

	// Turn new telnet connections away for the handoff. A copyover that
	// succeeds never returns here (Execute re-execs), so the deferred resume
	// runs only when this copyover fails: the save-flush abort, an Execute
	// error, or a panic. Each leaves the server running, and it must keep
	// accepting players (#670). Shutdown has its own flag, serverAlive.
	pauseAccepting()
	defer resumeAccepting()

	// Finds a player was already told about ("Something glints...") and
	// pickpockets in their pause are on goroutines that need this lock to
	// finish: finish them here, before rooms and players are saved, or the
	// re-exec loses them.
	actions.FlushBaubleDeliveries()
	actions.FlushPocketAttempts()

	// Guard G4 (chunk 3.6b-1). Autosave now spreads its writes across ticks, so
	// a cycle may be mid-flight. Those pending writes exist ONLY in memory, and
	// copyover re-execs the process, so anything left in the queue is gone for
	// good.
	//
	// Unlike shutdown, this ABORTS. Copyover is an optional convenience and a
	// refused one is recoverable and obvious; a completed one that silently ate
	// a cycle of saves is neither. This is the only place in the codebase where
	// a save failure stops an operation rather than being logged past.
	//
	// NPC knowledge is write-behind, so its dirty files go first: SaveAll
	// cancels any older queued copy, which FlushAll would otherwise land.
	// Logged, not an abort: it is NPC memory, and the queue flush below is
	// still the gate for player and world state.
	if err := knowledge.SaveAll(); err != nil {
		mudlog.Error("copyover", "action", "knowledge.SaveAll", "error", err)
	}
	if err := hooks.AutosaveQueue().FlushAll(); err != nil {
		mudlog.Error("copyover", "action", "FlushPendingWrites", "error", err,
			"message", "aborting copyover rather than discarding unwritten saves")
		return fmt.Errorf("copyover aborted: pending saves could not be flushed: %w", err)
	}

	// Logged rather than discarded: this is the last chance to persist room
	// state before the process re-execs, so a failure here silently loses it.
	if err := rooms.SaveAllRooms(); err != nil {
		mudlog.Error("copyover", "action", "SaveAllRooms", "error", err)
	}
	if err := users.SaveAllUsers(); err != nil {
		mudlog.Error("copyover", "action", "SaveAllUsers", "error", err)
	}
	// Living-economy dirty-state persists only on graceful shutdown, so flush it
	// here too — otherwise a copyover silently rewinds it. Keeps the reboot seamless.
	shops.SaveAllShops()
	warehouse.SaveAll()
	// The bauble sweeper is not stopped here: copyover holds the mud lock a
	// sweep may be waiting on, so waiting for it would deadlock. Its catalog
	// writes are atomic (util.Save), so the re-exec cutting a sweep off loses
	// nothing; the next boot sweeps again.
	baubles.SaveAll()
	apiframework.SaveBudget()
	forager.SaveAllThroughputs()
	caravan.SaveAllThroughputs()
	opinions.SaveAllOpinions()
	if err := plugins.Save(); err != nil {
		mudlog.Error("copyover", "action", "plugins.Save", "error", err)
	}

	// On success Execute never returns. If it fails after the connections
	// contributor froze output for the handoff, the server keeps running, so
	// lift the freeze or every player is left with no output. The deferred
	// resumeAccepting above reopens the telnet listeners.
	if err := copyover.Execute(binaryPath, os.Args[1:]); err != nil {
		connections.ThawOutputAfterFailedCopyover()
		return err
	}
	return nil
}
