// Package savequeue holds durable writes that have been prepared but not yet
// committed, so autosave can stop being a single long pause.
//
// The split it exists to support: everything that reads live game state (the
// reflection diff, the marshal) happens in ONE atomic pass under the world
// lock and produces immutable bytes; the durable write, which is 95% of the
// cost and touches nothing but those bytes, is spread across later ticks at a
// bounded rate.
//
// See docs/superpowers/specs/2026-08-10-autosave-async-writes-design.md
// (roadmap chunk 3.6b-1, review finding 36).
package savequeue

import (
	"fmt"
	"os"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// PendingWrite is an immutable unit of deferred work.
//
// It holds BYTES, not a Room or a UserRecord, and that is the whole reason
// deferral is safe. A Room holds maps and slices; copying the struct shares
// them, so a deferred writer would marshal state that the game is mutating
// underneath it. Marshalling first removes the hazard by construction: a
// []byte cannot be changed by the game. Nothing in this package may ever hold
// a reference to live game state.
//
// Careful is captured at prepare time rather than read at commit time, so the
// unit is fully self-describing and this package does not depend on configs.
type PendingWrite struct {
	Kind    string // "room" | "user" — for reporting only
	Id      int    // room id or user id — for reporting only
	Path    string // absolute or data-relative target path
	Data    []byte // nil means DELETE the file at Path instead of writing it
	Careful bool   // route through the atomic+durable path (ignored for deletes)
}

// IsDelete reports whether committing this entry removes the file.
//
// Rooms use this: when a room's instance overlay diffs to nothing, the stale
// overlay must be removed, because otherwise it is re-applied on the next room
// load and resurrects state the room no longer has.
func (p PendingWrite) IsDelete() bool { return p.Data == nil }

// Outcome is the result of committing one entry.
type Outcome struct {
	Write PendingWrite
	Err   error
}

// CycleResult summarises one prepare-and-drain cycle. It is produced exactly
// once, on the drain that empties the queue.
//
// This exists to preserve review finding 35: the autosave hook must not report
// success before the work has happened. With commits spread over many ticks,
// the only honest moment to report is when the last one lands.
type CycleResult struct {
	Total     int   // entries the cycle started with
	Succeeded int   // committed without error
	Failed    int   // committed with an error
	Cancelled int   // superseded by a synchronous save or a deletion (G2)
	FirstErr  error // first failure, for the log line
}

// Err returns a single error describing the cycle, or nil if nothing failed.
func (c CycleResult) Err() error {
	if c.Failed == 0 {
		return nil
	}
	return fmt.Errorf("savequeue: %d of %d write(s) failed; first error: %w",
		c.Failed, c.Total, c.FirstErr)
}

// DrainResult is what one call to Drain accomplished.
type DrainResult struct {
	Committed int
	Failed    int
	Outcomes  []Outcome
	// Cycle is non-nil on EXACTLY the drain that finishes the current cycle,
	// and nil on every other call. Callers report the cycle when it is set.
	Cycle *CycleResult
}

type entry struct {
	w         PendingWrite
	cancelled bool
}

// Queue is the pending set.
//
// NOT SAFE FOR CONCURRENT USE, AND DELIBERATELY SO. Every caller runs on
// World.MainWorker: the autosave hook runs inside EventLoop, and every
// synchronous save path that cancels into this queue is reached from a
// MainWorker select case. That single-goroutine precondition is what lets
// cancellation (guard G2) be a map delete rather than a locked cancellation
// protocol, and it is the main reason this design was chosen over a background
// writer goroutine.
//
// If you are about to call a Queue method from a new goroutine: don't. Post the
// work to MainWorker instead. Adding a mutex here would make the code look
// correct while silently reintroducing every hazard the amortised design was
// picked to avoid.
type Queue struct {
	entries []entry
	byPath  map[string]int // path -> index into entries; only LIVE entries
	cursor  int            // next index to consider committing
	cycle   CycleResult
	active  bool // a cycle is in progress and has not been reported
}

// New returns an empty queue.
func New() *Queue {
	return &Queue{byPath: map[string]int{}}
}

// Pending returns how many entries are still waiting to commit.
func (q *Queue) Pending() int { return len(q.byPath) }

// Supersede replaces the pending set with a freshly prepared one.
//
// Guard G3. If the previous set had not drained, the leftovers are DISCARDED
// rather than merged, and the fact is logged at WARN with the count. Merging
// would be wrong: the new set was prepared from the same live state, so it
// already contains a newer payload for every path the old set covered, and
// keeping the old entries would let a stale payload land after a fresh one.
//
// The WARN is the operator's signal that WritesPerTick is too low for the
// world size, because it means a whole autosave interval elapsed without the
// queue emptying.
func (q *Queue) Supersede(writes []PendingWrite) {
	if remaining := q.Pending(); remaining > 0 {
		mudlog.Warn("savequeue.Supersede",
			"message", "previous autosave set had not finished committing; discarding it",
			"discarded", remaining,
			"committedLastCycle", q.cycle.Succeeded,
			"newSetSize", len(writes),
			"hint", "raise Timing.AutosaveWritesPerTick or lower Timing.RoundsPerAutoSave",
		)
	}

	q.entries = make([]entry, 0, len(writes))
	q.byPath = make(map[string]int, len(writes))
	q.cursor = 0
	q.cycle = CycleResult{Total: len(writes)}
	// Always active, even for an empty set. A cycle that had nothing to write
	// still completed, and the caller still owes the player a completion
	// message; making the empty case report through the same path keeps that
	// special case out of the hook.
	q.active = true

	for _, w := range writes {
		// A prepared set should not contain two entries for one path, but if it
		// does, the later one wins — it was prepared from the same instant and
		// keeping both would commit the same path twice for no reason.
		if idx, dup := q.byPath[w.Path]; dup {
			q.entries[idx].cancelled = true
			q.cycle.Cancelled++
		}
		q.entries = append(q.entries, entry{w: w})
		q.byPath[w.Path] = len(q.entries) - 1
	}
}

// Cancel drops any pending write for path.
//
// Guard G2. Call this from every path that writes or REMOVES the file itself:
// a synchronous save (which has newer bytes than the pending entry), a template
// or zone deletion, or a cache purge. Without it a stale payload lands after a
// newer one, or a deleted room's overlay is recreated.
//
// Deletion is the case that is easy to miss, because those paths never call the
// save function: they call os.Remove / os.RemoveAll directly.
//
// Returns true if something was actually cancelled.
func (q *Queue) Cancel(path string) bool {
	idx, ok := q.byPath[path]
	if !ok {
		return false
	}
	q.entries[idx].cancelled = true
	delete(q.byPath, path)
	q.cycle.Cancelled++

	// Deliberately does NOT complete the cycle here, even when this was the last
	// live entry. maybeCompleteCycle hands the result to exactly one caller and
	// then disarms; Cancel returns a bool and has nowhere to put it, so calling
	// it here SWALLOWED the completion and the hook waited forever for a signal
	// that had already fired. Leave it to the next Drain, which runs every turn.
	return true
}

// Drain commits up to n entries and returns what happened.
//
// n is bounded by the caller (a config knob) so one tick's added cost stays
// inside the turn budget. A value below 1 is treated as 1: a queue that never
// drains persists nothing while the game reports itself healthy, which is a
// worse failure than any pause this design exists to fix.
func (q *Queue) Drain(n int) DrainResult {
	if n < 1 {
		n = 1
	}

	res := DrainResult{}
	for q.cursor < len(q.entries) && res.Committed+res.Failed < n {
		e := q.entries[q.cursor]
		q.cursor++

		if e.cancelled {
			continue
		}

		err := Commit(e.w)
		delete(q.byPath, e.w.Path)

		res.Outcomes = append(res.Outcomes, Outcome{Write: e.w, Err: err})
		if err != nil {
			res.Failed++
			q.cycle.Failed++
			if q.cycle.FirstErr == nil {
				q.cycle.FirstErr = err
			}
			mudlog.Error("savequeue.Drain",
				"kind", e.w.Kind, "id", e.w.Id, "path", e.w.Path, "error", err)
			continue
		}
		res.Committed++
		q.cycle.Succeeded++
	}

	if c := q.maybeCompleteCycle(); c != nil {
		res.Cycle = c
	}
	return res
}

// FlushAll commits every remaining entry synchronously and reports the result.
//
// Guard G4. Shutdown and copyover call this before the process goes away.
// Losing it would be new data loss in exactly the area the persistence work
// spent eight findings fixing, so the two callers are expected to differ in how
// they react: copyover must ABORT on a non-nil error (it re-execs, so the
// pending set would be gone for good), while shutdown logs and proceeds because
// refusing to stop is not useful.
func (q *Queue) FlushAll() error {
	var firstErr error
	failed := 0

	for q.cursor < len(q.entries) {
		e := q.entries[q.cursor]
		q.cursor++
		if e.cancelled {
			continue
		}
		err := Commit(e.w)
		delete(q.byPath, e.w.Path)
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
			mudlog.Error("savequeue.FlushAll",
				"kind", e.w.Kind, "id", e.w.Id, "path", e.w.Path, "error", err)
			continue
		}
		q.cycle.Succeeded++
	}

	q.cycle.Failed += failed
	if q.cycle.FirstErr == nil {
		q.cycle.FirstErr = firstErr
	}
	q.maybeCompleteCycle()

	if failed > 0 {
		return fmt.Errorf("savequeue.FlushAll: %d write(s) failed; first error: %w", failed, firstErr)
	}
	return nil
}

// maybeCompleteCycle returns the cycle result exactly once, on the transition
// to empty. Every later call returns nil until a new set is superseded in.
func (q *Queue) maybeCompleteCycle() *CycleResult {
	if !q.active || q.Pending() > 0 {
		return nil
	}
	q.active = false
	result := q.cycle
	return &result
}

// Commit performs one prepared write, or removes the file when Data is nil.
//
// It is exported because the synchronous save paths compose prepare+Commit to
// keep their existing "save this now, tell me if it failed" contract.
func Commit(p PendingWrite) error {
	if p.IsDelete() {
		// A failed delete is NOT harmless: the stale overlay is re-applied on the
		// next load, resurrecting state the room no longer has. This error used
		// to be discarded entirely (review finding 35).
		if err := os.Remove(p.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("savequeue: remove stale overlay %s: %w", p.Path, err)
		}
		return nil
	}
	return util.Save(p.Path, p.Data, p.Careful)
}
