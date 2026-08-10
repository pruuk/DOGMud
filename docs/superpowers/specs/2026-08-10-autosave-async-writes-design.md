# Autosave Without a World Freeze — Design

**Chunk:** 3.6b (Adversarial Review Remediation Roadmap), decomposed into 3.6b-1 / 3.6b-2 / 3.6b-3
**Status:** Reviewed and revised 2026-08-10. Ready for an implementation plan.
**Depends on:** 3.6a (measurement, done), 2.1 (living-state persistence contract), 2.7 (autosave outcome reporting), **2.8 (contract adopted repo-wide — this changed the numbers below)**
**Activated by:** 3.6a evidence — remediation required

## Goal

Autosave must stop being a full-simulation stop, and its cost must stop
scaling with the size of the world.

**Target: 100 concurrent players.** Not because that is today's load, but
because it is a realistic ceiling for a MUD that finds an audience, and
because this repository is a fork others may build on. A fork's world may be
much larger than ours, so "fast enough for 1386 rooms" is not the bar. The bar
is that cost tracks **activity**, not **world size**.

## Evidence (3.6a)

Autosave runs inside the `NewTurn` event with the global world lock held.
Measured per dirty room:

| Phase | Per room | Share | Reads live state? |
|---|---:|---:|---|
| `LoadRoomTemplate` (disk read) | 0.110 ms | 3.0% | No — immutable authored data |
| reflection diff | ~0.055 ms | 1.5% | **Yes** |
| `yaml.Marshal` | 0.007 ms | 0.2% | **Yes** |
| `util.Save` (temp + fsync + rename) | **3.459 ms** | **95.3%** | **No** — input is bytes |

Live: 1386 rooms, 0 players → **295 ms** per cycle (5–6 turns). Rounds are 4 s.

### Users, and what chunk 2.8 changed (added 2026-08-10)

3.6a reported `usersMs=0`, but with **zero players connected** — unmeasured,
not proven cheap. Benchmarked since, on a realistic 48KB established-character
file (`internal/util/livingstate_bench_test.go`, `-bench UserSave`):

| User save | Per file | 100 players |
|---|---:|---:|
| Old hand-rolled `.new`+rename, no fsync | 0.696 ms | 70 ms |
| Through `util.Save` (**what 2.8 shipped**) | 3.873 ms | **387 ms** |

Users were cheap **because they were not durable**. Chunk 2.8 fixed that, which
is correct and is not being reverted — but it means user saves are now the
**larger** half of the autosave pause at the 100-player target, ahead of the
236 ms room prepare. The old scope limit ("rooms only, revisit users later") was
written against the 70 ms figure and no longer holds.

## The constraint that decides the design

**Corrected 2026-08-10 during adversarial review.** This section previously said
player input arrives on `case wi := <-w.worldInput` *inside* MainWorker. It does
not: `worldInput` is read by **`InputWorker`** (`world.go:919`), a separate
goroutine, which does nothing but wrap the text in an `events.Input` and queue
it. The conclusion below survives, but the reason is different, and anyone
re-deriving it from the old wording would have got it wrong.

`World.MainWorker` (`world.go:732`) is a **single `select` loop** in which every
case takes `util.LockMud()` for its body. The cases that matter here:

- `case <-turnTimer.C` (every `TurnMs`, 50 ms) — **queues** `events.NewTurn`.
- `case <-eventLoopTimer.C` (every **1 ms**) — calls `w.EventLoop()` →
  `events.ProcessEvents()`, which is where `NewTurn` listeners (including
  `AutoSave`) and queued player `Input` events actually **run**.

So input is *accepted* off-loop but only *processed* in a MainWorker case.
**Therefore releasing the world lock does not let players act.** While MainWorker
is inside `EventLoop` running autosave, the next `eventLoopTimer` case cannot
run, so no queued input is processed, lock or no lock. Any design whose benefit
is "the lock is free sooner" helps only *other* goroutines contending on the
lock — which, after chunk 3.5 moved admin auth out of it, is nearly nothing.

One consequence worth keeping: because `eventLoopTimer` is 1 ms and `turnTimer`
is 50 ms, `NewTurn` events are processed individually and promptly. A design that
does per-turn work in a `NewTurn` listener therefore gets one execution per turn,
not a burst of several in one `EventLoop` pass.

The work must either leave MainWorker entirely, or be spread across ticks so
MainWorker returns to its select promptly.

## Decomposition

| Sub-chunk | Delivers | Pause at 100 players | Scales with |
|---|---|---:|---|
| **3.6b-1** Amortised commit, **rooms + users** | Prepare stays atomic; commit spread across ticks | ~236 ms | world size |
| **3.6b-2** Dirty-set write-behind | Removes the periodic sweep entirely | ~0 | **activity** |
| **3.6b-3** Transactional value transfers | Closes the two-file tear window | — | — |

3.6b-1 is not throwaway work: its pending-set, cancellation, supersede and
flush-on-shutdown machinery is exactly what 3.6b-2 consumes.

3.6b-3 is conditional on a separate decision (see its section).

---

# 3.6b-1 — Amortised commit

## Design

### Split `SaveRoomInstance` into prepare and commit

```go
// pendingWrite is an immutable unit of work. It holds BYTES, not a Room or a
// User, which is what makes it safe to defer.
type pendingWrite struct {
    kind string // "room" | "user" — for reporting
    id   int
    path string
    data []byte // nil means "delete the overlay instead"
}

// prepareRoomInstanceWrite does everything that reads live state.
// Caller must hold the world lock.
func prepareRoomInstanceWrite(r Room) (pendingWrite, error)

// commitPendingWrite performs the durable write (or delete).
// Touches only its own bytes and path.
func commitPendingWrite(p pendingWrite) error
```

**Users use the same two-phase split**, and the same type. `SaveUser`'s marshal
happens under the lock and its `util.Save` becomes a `pendingWrite` — the
identical shape, because the property that makes deferral safe (the payload is
already immutable bytes) is identical. `pendingWrite` deliberately lives in one
place with a `kind` discriminator rather than being duplicated per subsystem;
G2–G5 then apply to both by construction instead of being reimplemented.

One difference worth stating: users are **not** deleted by this path, so the
`data == nil` delete case is rooms-only. A user file's removal is account
deletion, which is not an autosave concern.

`SaveRoomInstance` remains as the synchronous composition of the two, because
unload, the builder, shutdown and copyover all legitimately want "save this
now, tell me if it failed".

**Why bytes, not a `Room` copy.** A `Room` holds maps and slices; copying the
struct shares them, and a deferred writer would marshal state mutating
underneath it. Marshalling *first* removes the hazard by construction — a
`[]byte` cannot be mutated by the game. This is why marshal stays under the
lock despite being 0.2% of the cost, and it is why the roadmap's
"unsafe shallow snapshot" risk does not arise here.

### Prepare is ONE atomic pass. Commit is amortised.

```
tick N:      under the lock, prepare every loaded non-ephemeral room
             -> []pendingWrite            (~0.17 ms/room, 236 ms @ 1386)
tick N..N+k: commit WritesPerTick entries per tick, on MainWorker
```

`WritesPerTick` is a config knob sized so a tick's added cost stays well inside
`TurnMs` (50 ms). At 3.46 ms/write, 3 writes/tick ≈ 10 ms.

**Amortised is not free, and the spec should not imply it is.** 10 ms of every
50 ms turn is a **sustained 20% occupancy of MainWorker** for as long as the set
is draining: 1386 entries at 3/tick is 462 turns, about **23 seconds** per cycle.
That is a much better trade than a 295 ms full stop — nothing is frozen, and
the game stays responsive — but it is a real, measurable degradation with a
duration, not a disappearance of the cost.

It also scales the wrong way. A fork with 10,000 rooms drains for ~167 s
(≈2.8 min) of every cycle at 20% occupancy. That is the strongest argument for
treating **3.6b-2 as the actual fix** rather than an optional follow-up: only
the dirty set makes the drain proportional to activity instead of world size.

**Prepare must NOT be chunked.** See Failure Mode 1 — this is the single most
important constraint in the design.

### What prepare actually costs, and why template caching is not optional

Adversarial review (2026-08-10) found the 236 ms figure internally inconsistent
with deferring template caching. `SaveRoomInstance` calls `LoadRoomTemplate`
(`save_and_load.go:238`), which is an **uncached disk read plus YAML parse, per
room, per cycle** — `loadRoomFromFile` every time, no memoisation.

| Prepare phase | Per room | Share of prepare |
|---|---:|---:|
| `LoadRoomTemplate` (disk read + parse) | 0.110 ms | **64%** |
| reflection diff | 0.055 ms | 32% |
| `yaml.Marshal` | 0.007 ms | 4% |
| **prepare total** | **0.172 ms** | 100% |

So the pass this sub-chunk keeps under the lock is **dominated by uncached file
I/O**, not by the diff. Two things follow:

1. Describing template caching as "a further ~65% off prepare" understated it.
   It is the majority of the remaining cost, and 236 ms → ~84 ms is the
   difference between "a noticeable hitch" and "barely visible".
2. **0.110 ms is a warm-page-cache local-SSD number.** On the droplet with a
   cold cache, 1386 file reads and parses could be materially worse, and unlike
   `util.Save` this cost cannot be amortised — it is inside the atomic pass by
   construction, because the diff needs the template.

Caching is still kept out of 3.6b-1 (the builder rewrites templates at runtime,
so invalidation is its own design problem, and mixing it in would put a
correctness-shaped change inside a scheduling-shaped one). But it is promoted
from "a nice further optimisation" to **the named prerequisite for the prepare
figure holding in production**, and the 236 ms target is explicitly a
warm-cache local number rather than a prod guarantee. The prod `AutoSave` log
line is the real check.

### Guards

| ID | Guard | Prevents |
|---|---|---|
| **G1** | Prepare is one atomic pass under a single lock hold | Cross-entity item/gold duplication or loss |
| **G2** | Any synchronous save **or deletion** of a path cancels a pending write for that path | Stale write clobbering a newer one; orphan files for deleted rooms |
| **G3** | A new cycle supersedes the previous pending set, and logs if it had not drained | Stale/reordered writes; the log is the signal `WritesPerTick` is too low |
| **G4** | Shutdown and copyover flush all pending writes synchronously before exit, and **copyover aborts if the flush fails** | Data loss at process exit |
| **G5** | Outcomes accumulate across the cycle and are reported at cycle end; success is never claimed early | Regressing finding 35 |

**G2 is nearly free here.** Everything runs on MainWorker, so the pending set is
a plain `map[string]pendingWrite` with **no mutex and no race**; a synchronous
save is `delete(pending, path)`. Under a background writer the same guard needs
a lock and a cancellation protocol.

**Invariant to assert:** every `SaveRoomInstance` call site is MainWorker-only.
Three exist today — `removeRoomFromMemory` (`roommanager.go:561`, unload),
`SaveRoomTemplate` (`save_and_load.go:258`, builder rewrite) and `SaveAllRooms`
(`save_and_load.go:494`) — and all three appear to be. Add a check rather than
trust it. (Line numbers re-verified 2026-08-10; the three previously listed here
had all drifted.)

**G2 does NOT come for free from the save call sites — review finding.** The
original wording ("any synchronous save cancels") silently assumed deletion also
flows through `SaveRoomInstance`. It does not. Three paths delete room data
without ever calling it:

| Path | What it does |
|---|---|
| `DeleteRoomTemplate` (`save_and_load.go:376`) | `os.Remove` on the template, then `ClearRoomCache` |
| `DeleteZone` (`zone_lifecycle.go:157`) | `os.RemoveAll` on **every** zone directory, including `rooms.instances/<zone>/` |
| `ClearRoomCache` | drops the room from memory with no save at all |

Left uncancelled, a pending write for a room in a just-deleted zone tries to
write into a directory that no longer exists. `util.SafeSave` does not `MkdirAll`,
so the realistic outcome is a spurious ERROR at commit rather than a resurrected
file — but it is an error attributed to the save system for something the
builder did, and if the directory happens to still exist it *is* an orphan
instance file for a room that no longer has a template.

These three are explicit G2 call sites, not an afterthought.

## Failure modes

### Safe, no guard needed

| Scenario | Why |
|---|---|
| Player walks room → room | `Room.players` is **unexported** (`rooms.go:114`), so reflection skips it. Room files never record occupancy. Location is one field in the user file. |
| Admin teleport | Same — only `Character.RoomId` changes. |
| Builder changes an exit | `Exits` is `instance:"skip"` — template state, different file, different write path. |

Footnote: `DefusedExits` is instance state and names exits by string, so
deleting an exit can strand an entry. Pre-existing; not introduced here.

### 1. Item or gold moving between a room and a player — **the reason prepare is atomic**

Pick up an item. If the room is snapshotted *before* the pickup and the user
*after*, the item is in **both** files → **duplication** on reload. Reverse the
order → **loss**. The same shape applies to dropping, corpse looting, container
contents, shop purchases (shop file ↔ user) and guild treasury.

A single atomic prepare is a consistent point-in-time view and cannot tear.
**Guard G1.** This is why the pause floor is ~236 ms in this sub-chunk rather
than ~10 ms.

**Users make this constraint sharper, not weaker.** The room half and the user
half of a pickup live in different files, so G1 only holds if rooms and users
are prepared in the **same** lock hold — one pass producing one pending set,
not "prepare rooms, then prepare users". Preparing them separately reintroduces
exactly the tear the atomic prepare exists to prevent, just with a shorter
window. This is the one place where adding users to the scope changes the
design rather than just extending it.

### 2–8. Commit-phase hazards

| # | Hazard | Guard |
|---|---|---|
| 2 | Room unloaded between prepare and commit; unload writes fresh bytes, stale write lands after | G2 |
| 3 | Builder rewrites the room via `SaveRoomTemplate` → `SaveRoomInstance` | G2 |
| 4 | Room or zone deleted; pending commit recreates an orphan instance file | G2 (extended to deletes) |
| 5 | Next cycle starts while commits pending | G3 |
| 6 | Shutdown/copyover with pending commits | G4 |
| 7 | Commit failures invisible | G5 |
| 8 | Room modified after prepare, before commit | **Accepted.** Writes slightly stale bytes; the next cycle corrects it; nothing is lost because live state is still in memory. |

## Scope: rooms AND users (revised 2026-08-10)

This originally said "rooms only, revisit users before claiming the 100-player
target." That was written when a user save cost 0.696 ms. Chunk 2.8 made user
saves durable and they now cost 3.873 ms, so at the 100-player target users are
**387 ms** — more than the entire 236 ms room prepare.

Deferring users would mean shipping 3.6b-1, measuring, and finding the pause had
gone *up*: the sub-chunk that exists to bound the pause would have bounded the
smaller half of it while the larger half grew. Users are in scope for 3.6b-1.

The cost is small because the machinery is shared: users need the same prepare/
commit split, and they join the same pending set, so G2–G5 cover them without
new guards. The additional work is the two-phase split of `SaveUser` and its
call-site audit, not a second mechanism.

**Still deferred:** a *load* measurement with real connected players. The
benchmark gives a per-file cost and the arithmetic to 100 players is
straightforward, but it does not capture whatever else scales with player count.
See "Measuring under load" below.

## Test strategy

**Unit**
- `prepareRoomInstanceWrite` produces byte-identical output to what
  `SaveRoomInstance` writes today, for clean, dirty and delete cases.
- The same for the user split: prepare + commit is byte-identical to today's
  `SaveUser`, and mutating the character after prepare does not change the
  payload.
- Rooms and users prepared in ONE pass: a test that mutates a room and a user
  between what would be two passes must show the pending set is internally
  consistent (the G1 regression test).
- Mutating the room after prepare does not change the payload (pins
  immutability — the shallow-snapshot guard).
- `commitPendingWrite` with `data == nil` removes the overlay, and a failed
  removal is an error (preserves 2.7).
- G2: a synchronous save cancels the pending write for that path.
- G3: a new cycle supersedes an undrained set and logs.
- G4: flush completes every pending write.
- G5: a failed commit produces an aggregate error and a non-success broadcast.

**Regression**
- All existing `SaveAllRooms` tests pass unchanged.
- `TestSaveAllRooms_ReportsFailureInsteadOfReturningNil` still fails loudly.

**Benchmark (acceptance)**
- Prepare ≤ **0.3 ms/room**, i.e. a fully dirty world costs about what a fully
  clean one costs today.
- Per-tick commit cost ≤ **10 ms** at the shipped `WritesPerTick`.

**Boot**
- Real world, autosave forced fast: `turnsDelayed` must fall, and no cycle may
  report an undrained predecessor.

## Completion criteria (3.6b-1)

1. Lock-held cost is independent of how many rooms **and users** are dirty.
2. No single tick's autosave contribution exceeds the per-tick budget.
3. A failed write still produces ERROR and a non-success broadcast.
4. Shutdown and copyover get a synchronous, definite outcome for both kinds.
5. Rooms and users are prepared in a single lock hold (G1 holds across kinds).
6. Full suite, lint, clean boot.

---

# 3.6b-2 — Dirty-set write-behind for rooms

## Why this is the real fix

3.6b-1 leaves a pause proportional to **world size**: the prepare pass walks
every loaded room to discover which changed. A fork with 10,000 rooms pays
~1.7 s regardless of whether anyone is playing.

If rooms mark themselves dirty on mutation, the sweep is unnecessary. Cost then
tracks **activity**: a 10,000-room world with 10 players costs the same as a
100-room world with 10 players. That is what makes the 100-player target
survivable in someone else's world.

## Correction to record

3.6a measured dirty tracking as worth 1.5% and this design earlier dismissed it
on that basis. **That measurement answered a narrower question than it was
applied to.** It measured dirty tracking as a way to skip the *diff* inside the
existing periodic design. It said nothing about using a dirty set to eliminate
the *sweep*. In that role it is the central mechanism, not a 1.5% saving.

## Sketch (to be specified properly when this sub-chunk is selected)

- Rooms mark dirty on mutation of instance-relevant state.
- A bounded number of dirty rooms are prepared+committed per tick, reusing
  3.6b-1's pending set, cancellation, supersede and flush machinery.
- **Coalescing is mandatory.** A room changing 20× per interval must not write
  20×; mark-and-flush-at-most-once-per-N-ticks. Without this, write-behind can
  be *worse* than the sweep.
- The periodic sweep is retained initially as a safety net at a long interval,
  then removed once the dirty set is trusted.

**Open risk:** correctness now depends on every mutation site marking dirty. A
missed site means silent non-persistence — which is worse than a slow save. The
safety-net sweep exists to make that detectable (it would find and write rooms
the dirty set missed, and can log when it does).

---

# 3.6b-3 — Transactional value transfers (conditional)

Persisting a room↔user transfer touches **two files**, and there is no
atomicity across two files. A crash between them duplicates or loses the item.
3.6b-1's atomic prepare removes the *snapshot* tearing window; it does not make
two file writes atomic.

Closing it requires a small write-ahead log: record the transfer durably, write
both files, clear the record, repair on boot.

**This is a separate decision**, not implied by the others. The tier below
frames it:

| Tier | Examples | Loss impact | Policy |
|---|---|---|---|
| **1 — value transfer** | items/gold room↔user, user↔user, shop↔user, guild treasury | Duplication or loss of value | Candidate for WAL |
| **2 — accumulated progression** | skills, stats, quest tokens, mutation progress | Lose ≤ one interval of grind | Periodic is fine |
| **3 — transient** | health, stamina, aggro, buffs | Regenerates, or should not survive restart | Do not persist |

The codebase already believes this split: `MobInstanceData` states that combat
state is "intentionally excluded — mobs respawn at full resources".

Recommendation: **do not build this yet.** The window is milliseconds and
requires a crash inside it. Revisit if duplication is ever observed, or when
the economy is valuable enough that the risk outweighs the complexity.

---

# Rejected alternatives

### Background writer goroutine

Moves commits off MainWorker entirely, so turns continue during the write.
Rejected for 3.6b-1:

- **Shutdown/copyover would need a drain**, and losing it is new data loss in
  precisely the area Wave 2 spent eight findings fixing.
- **Finding 35's reporting contract must be rebuilt** (no success at dispatch,
  outcomes collected within a cycle).
- **G2 becomes a locked cancellation protocol** rather than a map delete.
- **A panic policy is required.** MainWorker deliberately re-panics rather than
  run a frozen world; a silently dead writer is the same class of failure.
- This codebase has been bitten by concurrency repeatedly — finding 8 was
  `concurrent map writes` killing the server, finding 22 a listener deadlock.
  Adding a concurrent writer to the persistence path immediately after making
  persistence correct is the highest-regret option available.

Amortised commit reaches the same pause target with **zero new concurrency**.

### Sync-after-unlock (release the lock, then commit on MainWorker)

Rejected outright: MainWorker is a single select loop, so players cannot act
while it commits regardless of the lock. It optimises contention that chunk 3.5
already largely removed.

### Chunked prepare

Rejected: tears cross-entity item/gold invariants (Failure Mode 1).

---

# Review resolutions (2026-08-10)

1. **~236 ms every 15 minutes is acceptable** as 3.6b-1's landing point, with
   3.6b-2 as the follow-up that removes it. 3.6b-1 first.
2. **`WritesPerTick` ships at 3** (~10 ms/tick). Config knob, retunable without
   a rebuild per the balance-lives-in-config rule.
3. **Users are IN SCOPE for 3.6b-1**, not deferred — see "Scope" above. Chunk
   2.8 made them the larger half of the pause, which inverted the original
   answer to this question.
4. **Template caching is a separate slice.** The builder rewrites templates at
   runtime, so invalidation needs its own design; folding it in would mix a
   correctness-shaped change into a scheduling-shaped one.

## Measuring under load

Still owed, and deliberately not a blocker for 3.6b-1. The per-file cost is
benchmarked and the arithmetic to 100 players is direct, but a real load run
would catch anything else that scales with player count.

Two options, in preference order:

- **Headless, no login limit.** Drive `SaveAllUsers` against N synthetic
  in-memory users in a benchmark. No network, no AI agents, no config change,
  and it isolates the thing being measured. This is the better instrument for
  *this* question, because the number wanted is "what does the save pass cost at
  N users", not "how does the server behave with N clients".
- **Raise the local AI login cap 20 → 100 and connect harness agents.** Higher
  fidelity for whole-server behaviour, much slower to run, and it measures many
  things at once. Worth doing once as a sanity check on the whole 3.6b arc, not
  as the per-change instrument.

Recommendation: the benchmark for 3.6b-1's acceptance criteria; the harness run
once, at the end of 3.6b, before claiming the 100-player target is met.
