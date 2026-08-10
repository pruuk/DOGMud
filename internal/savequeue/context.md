# internal/savequeue

## Purpose

Holds durable writes that have been *prepared* but not yet *committed*, so
autosave can stop being a single long pause.

Autosave runs inside the `NewTurn` event with the global world lock held, and
3.6a measured a dirty room's save as **95.3% `util.Save`** (temp file, fsync,
rename) and only 1.7% diff-and-marshal. The expensive part touches nothing but
bytes. So the work splits: everything that reads live game state happens in one
atomic pass under the lock and produces immutable bytes, and the durable write
is spread across later ticks at a bounded rate.

**This package deliberately does not:**

- know what a `Room` or a `UserRecord` is (it holds `[]byte` and a path)
- decide *what* to save or *when* to prepare (that is `internal/hooks`)
- run anything on its own goroutine (see Gotchas)

Design: `docs/superpowers/specs/2026-08-10-autosave-async-writes-design.md`
(roadmap chunk 3.6b-1, review finding 36).

## Files

- `savequeue.go` — the whole package: `PendingWrite`, `Queue`, `Commit`.
- `savequeue_test.go` — guard-by-guard coverage, including the failure paths.

## Core types

```go
type PendingWrite struct {
	Kind    string // "room" | "user" — reporting only
	Id      int    // room or user id — reporting only
	Path    string
	Data    []byte // nil means DELETE the file at Path
	Careful bool   // route through the atomic+durable path
}

type CycleResult struct {
	Total, Succeeded, Failed, Cancelled int
	FirstErr                            error
}

type DrainResult struct {
	Committed, Failed int
	Outcomes          []Outcome
	Cycle             *CycleResult // non-nil ONLY on the drain that empties the queue
}
```

## Public API

Building and committing one unit:

```go
func (p PendingWrite) IsDelete() bool
func Commit(p PendingWrite) error   // write, or remove when Data == nil
```

The queue:

```go
func New() *Queue
func (q *Queue) Supersede(writes []PendingWrite)  // G3: replace the pending set
func (q *Queue) Cancel(path string) bool          // G2: drop a pending write
func (q *Queue) Drain(n int) DrainResult          // commit up to n
func (q *Queue) FlushAll() error                  // G4: commit everything now
func (q *Queue) Pending() int
func (c CycleResult) Err() error
```

## Gotchas

**NOT SAFE FOR CONCURRENT USE, AND DELIBERATELY SO.** Every caller runs on
`World.MainWorker`: the autosave hook runs inside `EventLoop`, and every
synchronous save path that cancels into this queue is reached from a MainWorker
select case. That single-goroutine precondition is what lets cancellation be a
map delete instead of a locked cancellation protocol, and it is the main reason
the amortised design was chosen over a background writer goroutine. **Do not add
a mutex to make it callable from a goroutine** — that would make the code look
correct while reintroducing every hazard the design was picked to avoid. Post
the work to MainWorker instead.

**`Cancel` must be called from DELETION paths too, not just save paths.** The
easy mistake is assuming everything that removes a file goes through the save
function. It does not: `DeleteRoomTemplate`, `DeleteZone` (which `os.RemoveAll`s
`rooms.instances/<zone>/` wholesale) and `ClearRoomCache` all remove room data
directly. A pending write left uncancelled then commits into a directory that no
longer exists.

**`Cancel` never completes a cycle, even when it removes the last live entry.**
`Drain` is the only thing that reports completion. `maybeCompleteCycle` hands the
result to exactly one caller and then disarms, and `Cancel` returns a `bool` with
nowhere to put it — calling it there swallowed the completion and left the hook
waiting for a signal that had already fired. Since the hook drains every turn,
the report is at most one turn late.

**`DrainResult.Cycle` is non-nil exactly once per cycle.** This is the finding-35
contract: with commits spread over many ticks, the only honest moment to report
success is when the last one lands. Do not report at prepare time.

**`Supersede` discards leftovers rather than merging them.** The new set was
prepared from the same live state, so it already holds a newer payload for every
path the old set covered; merging would let a stale payload land after a fresh
one. It logs at WARN when it discards, and that log line is the operator's
signal that `WritesPerTick` is too low for the world size.

**`Drain(n)` treats `n < 1` as 1.** A queue that never drains persists nothing
while the game reports itself healthy, which is worse than any pause this design
exists to fix, and it is one config typo away. The config validator clamps too;
this is the second line of defence.

**`Commit` does not create parent directories.** It matches `util.Save`, which
also does not. A path whose directory is missing is a bug upstream in prepare,
not something to paper over here.

**A failed delete is an error, not a no-op.** A stale overlay left on disk is
re-applied on the next room load and resurrects state the room no longer has.
An *absent* file is fine (`os.IsNotExist` is swallowed); a failed removal is not.

## Dependencies

`internal/util` (the one hardened durable write), `internal/mudlog`.

Nothing else, on purpose — it must not import `rooms` or `users`, because both
of those import it.

## Consumers

- `internal/hooks` — `NewTurn_AutoSave.go` prepares, drains, and reports.
- `internal/rooms`, `internal/users` — produce `PendingWrite`s and `Cancel` on
  their synchronous save and deletion paths.
- shutdown and copyover — `FlushAll` before the process goes away.
