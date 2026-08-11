# Amortise Plugin Saves — Design

**Chunk:** 4.7 (Adversarial Review Remediation Roadmap)
**Status:** Draft, awaiting review
**Depends on:** 2.8 (durable writes everywhere), 3.6b-1 (`internal/savequeue`, the
prepare/commit split), 4.6 (template cache)
**Supersedes nothing.** This closes the last remaining component of the
world-lock autosave pause.

## Why

`plugins.Save()` costs **20-22 ms** of every autosave cycle, against **3-5 ms**
for the entire room-and-user prepare. It is now the largest single component of
the pause by a wide margin.

**This is a regression I introduced in chunk 2.8.** 3.6a measured
`pluginsMs=0-1`. Chunk 2.8 routed plugin writes through `util.Save`, whose
comment reads:

```go
// Durable atomic write (chunk 2.8). This was a BARE write with no
// atomicity at all, over plugin state that includes auction history,
// leaderboards and weather simulation state.
```

Four plugins, one fsync each, ~5 ms apiece. The durability was correct and is
not being reverted; only its scheduling was wrong.

What is written every cycle, measured on the live server 2026-08-11:

| File | Size |
|---|---:|
| `auctions-v1.0/auctionhistory` | 136 B |
| `gmcp_mudlet-v1.0/mudlet_config` | 232 B |
| `leaderboards-v1.0/latest-leaderboards` | 2,980 B |
| `weather-v0.2.0/simstate` | 3,131 B |

**~6.5 KB costs 22 ms.** The cost is fsync count, not data volume.

## Goals

1. **Bound growth.** The cost is currently linear in plugin count with nothing
   capping it, which is how the room sweep reached 295 ms.
2. **Reclaim headroom.** 22 ms is ~44% of a 50 ms turn spent on 6.5 KB.

**Non-goal: tiering durability by data value.** Deciding per plugin whether its
data "deserves" an fsync is exactly how the pre-Wave-2 mess arose: some stores
hardened, some not, on no principle. Durability stays uniform. Only *when* the
write lands changes.

## Design

### Fold plugins into the one atomic prepare

`plugins.PrepareAll()` activates a **collector** on the registry, invokes every
plugin's existing `onSave` callback, and collects the marshalled bytes instead
of writing them. The resulting `savequeue.PendingWrite`s join the **same set**
as rooms and users and drain through the existing machinery.

```go
// PrepareAll runs every plugin's onSave with writes collected rather than
// committed. Caller must hold the world lock.
func PrepareAll() ([]savequeue.PendingWrite, error)
```

**`WriteStruct` and `WriteBytes` keep their exact signatures.** Module authors
write `m.plug.WriteStruct("key", m.data)`, and that is upstream GoMud's plugin
API — changing it would break third-party modules. Only the behaviour changes,
and only while a collector is active:

```go
func (p *Plugin) WriteBytes(identifier string, bytes []byte) error {
    // ... existing path computation, unchanged ...
    if c := activeCollector(p); c != nil {
        c.add(savequeue.PendingWrite{Kind: "plugin", Path: fullPath, Data: bytes, Careful: true})
        return nil
    }
    return util.Save(fullPath, bytes)   // unchanged synchronous path
}
```

There is **one collector for the whole registry**, not one per plugin: a
package-level variable in `internal/plugins`, set by `PrepareAll` and cleared
before it returns. `activeCollector(p)` reads that single variable; the plugin
argument is only used to attribute the write. A plain variable with no mutex,
for the same reason `internal/savequeue` has none: every caller is on
MainWorker. See that package's `context.md`.

`PendingWrite.Kind` is `"plugin"` and `Id` is unused (0), since plugins have no
numeric id. The plugin is still identifiable in failure reports because `Path`
contains its `name-vversion` folder.

### Why this is more correct, not just cheaper

A bid **deducts player gold and writes auction history** — two files in one
logical transaction. Preparing plugins in a separate pass from users carries the
same tear risk that guard G1 exists to prevent for rooms and users. Folding
plugins into the single atomic prepare closes that window as a side effect.

The existing callbacks already have the right shape for the split.
`auctions.save` **reads live state** before marshalling:

```go
mod.auctionMgr.WalletBalances = map[string]int{}
for _, b := range npcBuyers { ... b.Wallet() ... }   // live state
return mod.plug.WriteStruct(`auctionhistory`, mod.auctionMgr)
```

That gather must stay under the lock; only the write may defer. Which is exactly
what this design does.

### Out-of-cycle writes stay synchronous — this is load-bearing

A plugin may call `WriteBytes` outside an autosave. `weather.persistState()`
does, on every simulation tick (`TickEveryGameHours: 8`).

Those writes **must not** be enqueued, and the reason is guard G3: `Supersede`
**discards** an undrained set rather than merging it. A write enqueued from a
weather tick could be silently thrown away by the next cycle's prepare. That is
data loss, not a pause.

The collector being active only during `PrepareAll` gives this for free: outside
that window `WriteBytes` takes the synchronous path exactly as today. The design
depends on that, so it is stated rather than left implicit.

### Shutdown and copyover

`plugins.Save()` is unchanged and keeps its three callers (`copyover.go:85`,
`main.go:601`, and the autosave hook). With no collector active it writes
synchronously, so guard G4 holds without special-casing: copyover still aborts
on failure, shutdown still logs and proceeds.

The autosave hook stops calling `plugins.Save()` and calls `PrepareAll()`
instead, appending the result to the set it already builds from rooms and users.

## The ceiling this creates, and the guard rail it needs

Per new plugin, under this design:

| Cost | Amount |
|---|---|
| **In the lock** | its `onSave` gather + `yaml.Marshal` (~0.007 ms for a room-sized payload) |
| **Out of the lock** | one more entry in the drain queue |

At `AutosaveWritesPerTick: 3` across an 18,000-turn interval the queue absorbs
~54,000 writes per cycle. Today's set is ~30. Plugins will never approach it.

**But the ceiling moves rather than disappears**, from "5 ms of fsync per
plugin" to "whatever the plugin computes in `onSave`". `auctions.save` proves
that is a real pattern. A future plugin that walked every room or every player
would put that work under the world lock, and no amount of write-scheduling
would help.

Worse, it would be **invisible**: `pluginsMs` is a single aggregate, so one slow
plugin reads identically to four medium ones.

Two additions make the guarantee self-reporting:

1. **Per-plugin prepare timing.** Log a WARN naming any plugin whose prepare
   exceeds **5 ms**. That number is not arbitrary: 5 ms is what one fsync cost,
   so the threshold reads as "this plugin's prepare now costs as much as its
   write used to" — the point at which amortising it stopped helping. Fixed in
   code rather than a config knob: it is a diagnostic threshold tied to a
   measured physical cost, not a balance value an operator should tune.

   This plays the same role for plugin cost that the `savequeue.Supersede`
   warning plays for the drain budget: it turns a silent ceiling breach into a
   line in the log.
2. **Document the `onSave` contract** in the plugin API: gather and marshal;
   never do I/O or long computation, because it runs under the world lock.
   Nothing currently says so, which is why the risk exists at all.

## Accepted trade-off

Plugin state becomes **up to one drain-cycle staler on a hard crash**. Today
`plugins.Save()` returns only once all four files are on disk; afterwards they
are durable within seconds but not before the hook returns. Against a 15-minute
autosave interval that window is negligible, but it is a real change to when the
guarantee lands and is recorded here deliberately.

## Testing

**Unit**
- `PrepareAll` returns one `PendingWrite` per plugin that writes, with bytes
  byte-identical to what `plugins.Save()` writes today.
- A plugin whose `onSave` errors is reported and does not abort the others
  (matches current `plugins.Save` behaviour).
- With no collector active, `WriteBytes` writes synchronously — the
  out-of-cycle path.
- The collector is cleared even when a callback panics or errors, so a failure
  cannot leave the registry stuck in collecting mode.
- Per-plugin timing warns above the threshold and stays silent below it.

**Regression**
- `plugins.Save()` still writes synchronously for shutdown and copyover.
- Weather's tick-time `persistState` still writes immediately and is never
  enqueued.

**Acceptance**
- Live boot with autosave forced fast: `pluginsMs` falls from ~22 ms to
  approximately the marshal cost, `turnsDelayed` stays 0, and the four plugin
  files are still rewritten each cycle.

## Out of scope

- **Skipping writes whose bytes are unchanged.** Would remove ~3 of 4 writes
  most cycles, but it bounds nothing — a plugin whose state always changes still
  pays full cost — so it does not serve goal 1. Worth revisiting only if disk
  churn on the droplet becomes a concern.
- **A dedicated writer goroutine**, rejected on the same grounds as in 3.6b-1:
  shutdown drain, a rebuilt reporting contract, a locked cancellation protocol
  and a panic policy, in a codebase repeatedly bitten by concurrency.
- **Stale plugin-data directories.** `weather-v0.1.0/` and
  `weather-v0.2.0/geography` are left over from older module versions and are no
  longer written. Harmless; belongs with a plugin-data housekeeping pass.
