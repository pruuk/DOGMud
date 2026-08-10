# Implementation Plan — 3.6b-1 Amortised Commit (rooms + users)

**Spec:** `docs/superpowers/specs/2026-08-10-autosave-async-writes-design.md`
**Roadmap chunk:** 3.6b-1 (finding 36)
**Branch:** `fix/chunk-3.6b1-amortised-autosave-commit`
**Revised:** 2026-08-10 after an adversarial review of this plan (10 findings,
3 material). See "What the adversarial review changed" below.
**Status: COMPLETE 2026-08-10.** All 9 tasks. Results and the two benchmark bugs
found along the way are recorded in the roadmap's 3.6b-1 section.
**Depends on merged work:** 2.1 (persistence contract), 2.7 (autosave outcome
reporting), 2.8 (contract adopted repo-wide), 3.5 (admin lock scope), 3.6a
(measurement)

## What this delivers

Autosave stops being a single long pause. The part that must read live state
(diff + marshal, for rooms **and** users) runs in one atomic pass under the
world lock; the part that only touches immutable bytes (the durable write) is
spread across subsequent ticks at a bounded rate.

Target: the lock-held cost becomes independent of how many rooms and users are
*dirty*, and no single tick's autosave contribution exceeds ~10 ms.

## Verified against the code (2026-08-10)

Checked with codegraph before planning, because the spec's line numbers had
already drifted:

| Symbol | Real location | Note |
|---|---|---|
| `SaveRoomInstance(r Room) error` | `internal/rooms/save_and_load.go:395` | diff → marshal → `util.Save`; `len(instanceSaveData)==0` means *delete the overlay* |
| `SaveAllRooms() error` | `internal/rooms/save_and_load.go:494` | skips `IsEphemeral`, aggregates errors (2.7) |
| `SaveUser(u *UserRecord, isAutoSave ...bool) error` | `internal/users/users.go:704` | marshal → `util.Save` since 2.8 |
| `SaveAllUsers(isAutoSave ...bool) error` | `internal/users/users.go:361` | snapshots under `userManager.mu.RLock()`, aggregates errors |
| `AutoSave(e events.Event)` | `internal/hooks/NewTurn_AutoSave.go:27` | 3 stages, gated on `TurnsPerAutoSave()` |

**`SaveRoomInstance` call sites** (the spec listed stale line numbers; identities
are right): `SaveRoomTemplate` (`save_and_load.go:258`), `SaveAllRooms`
(`:494`), `removeRoomFromMemory` (`roommanager.go:561`).

**`SaveUser` call sites:** `SaveAllUsers` (`:361`), `LogOutUserByConnectionId`
(`:387`), `CreateUser` (`:442`), `loadUserFromPath` (`:489`).

Those seven call sites are what G2 (cancel-on-synchronous-save) must cover.

## Design decisions that came out of planning

### The AutoSave hook's reporting has to change shape

This is the part the spec did not spell out and it is the main risk of quietly
regressing finding 35. Today the hook broadcasts `Saving users...` → outcome →
`Saving rooms...` → outcome, because each stage finishes before the next starts.
Under amortised commit the writes are not finished when the hook returns.

Rule: **the outcome broadcast moves to drain completion.** At the prepare tick
the hook announces that a save has started; the `Done.` / `Saved with errors.`
line is emitted once, when the last pending write of that cycle commits. A
success broadcast at prepare time would be exactly the "claims success before
the work happened" defect 2.7 removed.

`plugins.Save()` stays synchronous and keeps its own stage. It measured 0–1 ms
and is out of scope.

**The UX consequence needs a decision, not just a rule.** A cycle drains over
~23 seconds, so keeping today's shape literally produces `Saving users...` and
then `Done.` twenty-three seconds later, which reads as a hang. Decision: emit
**one** line at prepare (`Saving...`) and **one** at drain completion, and drop
the per-stage progress chatter for the amortised kinds. Plugins keep their own
synchronous line because they genuinely finish immediately.

### Drain runs on every turn, not only on autosave turns

`AutoSave` already listens to `NewTurn` and gates the body on
`TurnNumber % TurnsPerAutoSave() == 0`. The drain must run on *every* turn, so
the hook becomes: drain up to `WritesPerTick` pending entries first, then, if
this is an autosave turn, prepare a new set.

Ordering matters: draining before preparing means a new cycle's supersede check
(G3) sees the true remaining backlog.

### One pass, not two

Rooms and users are prepared in the **same** lock hold into the **same** pending
set. Preparing rooms and then users reintroduces the item/gold tear the atomic
prepare exists to prevent (spec Failure Mode 1). This is the single constraint
most likely to be lost in implementation, so it gets its own test.

### Where `pendingWrite` lives — and where the prepare pass does NOT

Not in `internal/rooms` (users would import it) and not in `internal/users`
(rooms would). New small package **`internal/savequeue`**, depending only on
`util` and `mudlog`, holding the type, the pending map, the drain, the guards
and the outcome accumulator. Rooms and users each expose a `prepare…` that
returns a `savequeue.PendingWrite`.

**The one-pass orchestration cannot live in `savequeue`.** Review finding: the
obvious-looking home for `PrepareAutosaveSet()` is beside the queue, but that
function must call into *both* `internal/rooms` and `internal/users`, and both
of those import `savequeue` — an import cycle. It belongs in
**`internal/hooks`**, which already imports rooms, users, plugins and events for
`NewTurn_AutoSave.go`. `savequeue` stays a leaf: it holds bytes and knows
nothing about what produced them.

This also settles where the G1 test lives: `internal/hooks`, the only package
that can see a room and a user at once.

Per the repo convention, `internal/savequeue/context.md` ships with it.

## Tasks

Each task is independently verifiable and leaves the build green.

### Task 1 — `internal/savequeue`: the queue, guards, and outcome accumulator

New package. No callers yet, so it lands with only its own tests.

- `PendingWrite{Kind, Id, Path, Data}`; `Data == nil` means delete.
- `Queue` with `Supersede([]PendingWrite)`, `Drain(n int) []Outcome`,
  `Cancel(path string)`, `FlushAll() error`, `Pending() int`.
- Plain map, no mutex: everything runs on MainWorker. Document that
  precondition loudly in `context.md` — it is what makes G2 free, and it is
  invalidated the moment someone calls it from a goroutine.
- G3: `Supersede` logs at WARN when the previous set had not drained, including
  how many were left. That log line is the signal `WritesPerTick` is too low.
- G5: outcomes accumulate per cycle; the queue reports the cycle result exactly
  once, at drain completion.

**Tests:** supersede-with-backlog logs and replaces; cancel removes exactly one
path; drain respects `n`; drain of a delete removes the file and a failed
removal is an error (preserves 2.7); `FlushAll` writes everything;
cycle outcome is reported once and only at completion.

### Task 2 — Config knob `WritesPerTick`

`internal/configs/config.balance.go` field + default in the right sibling file,
plus the `_datafiles/config.yaml` entry with a comment explaining the tradeoff.
Ships at **3** (~10 ms/tick at the measured 3.46 ms/write).

Balance lives in config, so this must be retunable without a rebuild — which
means it must also survive being set badly. **Clamp to a minimum of 1 in the
validator.** At `0` or negative the queue never drains: the set grows without
bound, G3 warns every cycle forever, and nothing is ever persisted while the
game reports itself healthy. That is a worse failure than any pause this chunk
exists to fix, and it is one typo away.

### Task 3 — Split `SaveRoomInstance` into prepare + commit

- `prepareRoomInstanceWrite(r Room) (savequeue.PendingWrite, error)` — the diff,
  the marshal, the delete decision. Everything that reads live state.
- `SaveRoomInstance` becomes prepare + immediate commit, so unload, the builder,
  shutdown and copyover keep their synchronous "save now, tell me if it failed"
  contract unchanged.
- Every synchronous `SaveRoomInstance` cancels any pending write for that path
  (G2).
- **G2 must also cover the three DELETION paths that never call
  `SaveRoomInstance`** — this was missed in the first draft of this plan:
  `DeleteRoomTemplate` (`save_and_load.go:376`), `DeleteZone`
  (`zone_lifecycle.go:157`, which `os.RemoveAll`s `rooms.instances/<zone>/`
  wholesale) and `ClearRoomCache`. Without cancellation, a pending write for a
  room whose zone the builder just deleted commits into a directory that no
  longer exists; `util.SafeSave` does not `MkdirAll`, so it surfaces as a
  spurious save-system ERROR for something the builder did — or, if the
  directory survives, as a genuine orphan instance file for a room with no
  template.

**Tests:** prepare output is byte-identical to what `SaveRoomInstance` writes
today for clean / dirty / delete; mutating the room after prepare does not
change the payload (the immutability guard); a synchronous save cancels a
pending write for that path; **`DeleteZone` with a pending write for one of its
rooms produces no orphan file and no commit error.**

### Task 4 — Split `SaveUser` the same way

- `prepareUserWrite(u *UserRecord) (savequeue.PendingWrite, error)`.
- `SaveUser` = prepare + commit, so the three non-autosave call sites
  (`LogOutUserByConnectionId`, `CreateUser`, `loadUserFromPath`) are unchanged
  and each cancels a pending write for that path.
- Keep the existing `defer mudlog.Info(...)` progress line honest: it currently
  reports `wrote-file` / `completed`, and those must mean what they say in both
  the synchronous and the deferred path.
- No delete case: users are never removed by autosave.

**Tests:** prepare + commit is byte-identical to today's `SaveUser`; mutating
the character after prepare does not change the payload; a synchronous save
cancels the pending write.

### Task 5 — One atomic prepare pass

`PrepareAutosaveSet()` **in `internal/hooks`** (not `savequeue` — see the
import-cycle note above), producing one `[]PendingWrite` covering every
non-ephemeral loaded room and every active user, in a single lock hold.

**Test (the G1 regression test):** with an item moved between a room and a
player, the prepared set must contain a consistent point-in-time view — the item
appears in exactly one of the two payloads, never both and never neither. This
is the test that fails if someone later "optimises" prepare into two passes.

### Task 6 — Rewire the AutoSave hook

- Drain up to `WritesPerTick` every turn.
- On an autosave turn: supersede with a freshly prepared set.
- Move the outcome broadcast to drain completion; announce at prepare.
- Keep the 3.6a structured log line, extended with `pendingRemaining` and
  `prepareMs` so prod stays self-measuring across the change.

### Task 7 — Shutdown and copyover flush (G4)

`triggerCopyover` (`copyover.go:44`) and the shutdown path call `FlushAll()`
before exit and propagate its error. Losing this is new data loss in exactly the
area Wave 2 spent eight findings fixing, so it gets an explicit test.

**"Propagate the error" is not a policy, and the first draft stopped there.**
The two callers need different answers:

- **Copyover ABORTS on flush failure.** Copyover re-execs the process; carrying
  on after a failed flush discards the pending set permanently. A refused
  copyover is recoverable and visible; a completed one that ate a cycle of saves
  is neither.
- **Shutdown logs at ERROR and proceeds.** The operator asked the process to
  stop and refusing is not useful, but the failure must be loud enough to find
  in the log afterwards.

### Task 8 — Benchmarks and acceptance

- Extend `internal/rooms/autosave_bench_test.go` with a prepare-only benchmark;
  acceptance is **≤ 0.3 ms/room**, i.e. a fully dirty world costs about what a
  fully clean one costs today. **State plainly that this is a warm-page-cache
  local-SSD number**: 64% of prepare is `LoadRoomTemplate` doing an uncached
  read and parse per room, so the droplet figure is unknown until the prod
  `AutoSave` log line reports it. Do not present the local benchmark as a prod
  guarantee.
- Add a `SaveAllUsers`-shaped benchmark over N synthetic in-memory users — the
  headless instrument the spec chose over raising the local AI login cap.
- Per-tick commit cost ≤ **10 ms** at the shipped `WritesPerTick`.

### Task 9 — Docs, verification, ship

- `internal/savequeue/context.md`; update `internal/rooms/context.md` and
  `internal/users/context.md` for the new prepare functions.
- Roadmap 3.6b-1 → DONE with what was measured.
- `docs/PATCH_NOTES.md`: player-facing framing, no raw numbers, no em dashes.
- Pre-push SOP: `gofmt -l internal/ modules/`, `go build ./...`, full
  `go test ./internal/...`, `golangci-lint run --new-from-rev=HEAD`,
  `Logging.LogToFile: false`, boot test in an isolated detached worktree
  (exit 124 is the success case), then PR against `pruuk/DOGMud`.

## Risks

| Risk | Mitigation |
|---|---|
| Prepare gets split into two passes later, tearing item/gold | Task 5's G1 test fails loudly; the reason is stated in the test, not just the spec |
| Someone calls the queue off MainWorker, and the lock-free map races | `context.md` states the precondition; consider a cheap goroutine-identity assert in debug builds |
| Success broadcast drifts back to prepare time | Task 6 test asserts no outcome broadcast before drain completion |
| `WritesPerTick` too low, sets never drain | G3 WARN with the remaining count; visible in prod logs immediately |
| Deploy makes the pause *worse* if only half lands | 2.8 is merged but not deployed; both ship to the droplet together |
| `WritesPerTick` misconfigured to 0 — nothing ever persists while the game looks healthy | Validator clamps to ≥ 1 (Task 2) |
| Copyover completes after a failed flush, silently eating a cycle | Copyover aborts on flush failure (Task 7) |
| Prepare is slower on the droplet than the local benchmark suggests | 64% of prepare is uncached template I/O; prod log line is the check, and template caching is the named prerequisite |
| Pending set holds every payload at once | ~5 MB at our scale (100 users x 48 KB); note it, revisit only at fork scale |

## Explicitly out of scope

- **3.6b-2** dirty-set write-behind — the fix that makes cost track activity
  rather than world size. This sub-chunk's machinery is built to be consumed by
  it.
- **3.6b-3** transactional value transfers (write-ahead log). Separate decision;
  spec recommends not yet.
- **Template caching** (~65% more off prepare). Separate slice: the builder
  rewrites templates at runtime, so invalidation needs its own design.
- **A real 100-player load run.** Owed once, at the end of 3.6b, before the
  100-player target is claimed met.

## What the adversarial review changed (2026-08-10)

Reviewed after the plan was approved, on the principle that a plan nobody
attacked is a plan whose assumptions are still guesses. Ten findings; three
changed the work.

**Material:**

1. **The spec's central piece of evidence was factually wrong.** It claimed
   player input arrives on a `case wi := <-w.worldInput` *inside* MainWorker.
   That case is in **`InputWorker`** (`world.go:919`), a separate goroutine. The
   conclusion — "releasing the lock does not let players act" — survives, but
   because input is only *processed* in MainWorker's `eventLoopTimer` case, not
   because MainWorker reads the channel. Corrected in the spec. Anyone
   re-deriving the design from the old wording would have reached the right
   answer by luck.

2. **The 236 ms prepare figure is inconsistent with deferring template
   caching.** `LoadRoomTemplate` is an uncached disk read and YAML parse per
   room per cycle, and at 0.110 ms of a 0.172 ms prepare it is **64% of the pass
   this chunk keeps under the lock**. Caching stays out of 3.6b-1 for good
   reasons, but it is now recorded as the named prerequisite for the figure
   holding in production rather than as a further optimisation, and the
   acceptance number is explicitly warm-cache-local.

3. **G2 had a hole: three deletion paths never call `SaveRoomInstance`.**
   `DeleteRoomTemplate`, `DeleteZone` and `ClearRoomCache` remove room data
   directly, so "any synchronous save cancels" would not have cancelled anything
   when a builder deleted a zone with writes pending. Added as explicit G2 call
   sites with a test.

**Corrections to this plan:**

4. `PrepareAutosaveSet()` cannot live in `savequeue` — it must call both
   `rooms` and `users`, which both import `savequeue`. Moved to `internal/hooks`,
   which also settles where the G1 test can live.
5. `WritesPerTick` needs a validator clamp; at 0 the queue never drains and
   nothing is persisted while the game reports itself healthy.
6. "Propagate the flush error" was not a policy. Copyover **aborts**; shutdown
   logs and proceeds.
7. The broadcast rule produced bad UX (`Saving users...` then `Done.` 23 s
   later). One line at prepare, one at completion.

**Recorded, not acted on:**

8. **Amortised is not free.** 10 ms of every 50 ms turn is a sustained **20%
   MainWorker occupancy for ~23 seconds** per cycle (≈2.8 minutes at 10,000
   rooms). Far better than a 295 ms freeze, but it is a degradation with a
   duration, not a disappearance. This is the strongest argument for treating
   3.6b-2 as the real fix rather than an optional follow-up.
9. `isAutoSave` is a **dead parameter** — threaded through `SaveAllUsers` into
   `SaveUser` and never read in either. Tempting to delete while splitting
   `SaveUser`, but that is unrelated cleanup; note it and leave it.
10. The pending set holds every payload simultaneously (~5 MB at our scale).
    Fine here; worth remembering for a fork with 1000 players.

**Checked and found sound:** the one-atomic-pass requirement; bytes-not-structs
as the deferral-safety property; the MainWorker-only precondition that makes the
queue lock-free (`LogOutUserByConnectionId` reaches `SaveUser` via
`w.logoutConnectionId` and via event handlers, both of which run on MainWorker);
`eventLoopTimer` at 1 ms against `turnTimer` at 50 ms, so a `NewTurn` listener
really does get one execution per turn rather than a burst.
