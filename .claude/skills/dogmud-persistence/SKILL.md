---
name: dogmud-persistence
description: Use when adding state that must survive a restart, writing a data migration, or deciding whether a directory is safe to wipe. Covers the four rules of the living-state contract, which directories are living state that must never be wiped by the instance-save cleanup (shops, guilds, moderation) versus which are instance overrides that must be, the instance:"skip" tag exception, and the trap where a character-scoped migration marker re-runs per alt.
---

This skill covers what makes DOGMud state durable: which directories are
living state that a wipe would destroy permanently, which are disposable
instance overrides, the one tagged-field exception to the shadowing rule, and
a migration-marker trap that corrupts a shared bank once per alt.

The instance-save wipe command itself, and the rest of the smoke-test
ritual, live in the `dogmud-shipping` skill. This skill does not restate that
procedure; it covers the reasoning that decides which directories the wipe
is allowed to touch.

## Living state versus instance override

Two categories of on-disk world data exist, and they are opposite: one MUST
be wiped before a local smoke test, the other MUST NEVER be wiped.

**Instance overrides**: `_datafiles/world/dogmud/mobs.instances/` and
`_datafiles/world/dogmud/rooms.instances/`. These shadow authored YAML
templates. They are reproducible: wipe them and the engine rebuilds them
from the (possibly updated) template on the next load. They are not
deployed to prod and are exactly what the `dogmud-shipping` smoke-test wipe
targets.

**Living state**: `_datafiles/world/dogmud/shops/`, `guilds/`, and
`moderation/` (`petitions.yaml`, `bans.yaml`). CLAUDE.md's "Shop Persistence"
section states shop economic state (stock levels, NPC gold, restock timers)
lives at `_datafiles/world/dogmud/shops/{zone}/{mobid}-room{roomid}.yaml`,
is completely separate from `rooms.instances/` and `mobs.instances/`, and is
NOT cleaned by the instance-save cleanup SOP. Deleting a shop file resets
that merchant to template defaults (500g starting gold, base stock levels)
rather than shadowing it. CLAUDE.md's "Moderation Persistence" section
states moderation data is `.gitignore`d, kept on the prod droplet, and must
NOT be wiped by the instance-save smoke-test SOP, "like `shops/` and
`guilds/`". Guild files are runtime-generated per-guild YAML
(`guilds/<tag>.yaml`); a malformed one logs and skips at boot rather than
panicking, unlike authored content, and moderation files behave the same
way, mirroring the guilds loader.

**Deciding for a directory neither list names:** ask whether the directory
holds data the engine can reproduce by re-reading authored content, or data
that accumulated at runtime and has no other source of truth. If a wipe
would just cause a rebuild from YAML on next load, it is an instance
override and belongs in the wipe. If a wipe would erase something that only
ever existed because players or the economy acted on it, and there is no
template to rebuild it from, it is living state, and the
[[reference_living_state_persistence_contract]] memory file's write-path
rules apply to it. When still unsure, check whether the directory is
`.gitignore`d and excluded from deploy the way `shops/`, `guilds/`, and
`moderation/` are.

## The instance:"skip" exception

Not every field in an overlaid struct is actually shadowed by a stale
instance save. Fields tagged `instance:"skip"` are the exception:
`SaveRoomInstance` skips them when writing, and `restoreSkipTaggedFields`
(`internal/rooms/save_and_load.go`) copies them back from the template
after the instance overlay is applied, so a stale save cannot override
them.

`Room.SpawnInfo` (room spawn lists) is in this category. CLAUDE.md records
that it was wrongly listed as shadowed until 2026-07-25: a spawn-list edit
takes effect on the next room load with no wipe needed.

Check the struct tag before assuming a field is shadowed by a stale
instance save. A field tagged `instance:"skip"` does not need the
`dogmud-shipping` wipe to pick up a template edit.

## The living-state contract

[[reference_living_state_persistence_contract]] establishes four rules for
persisting living state (users, mobs, shops, guilds, bans), adopted
repo-wide in roadmap chunk 2.8 and now enforced by
`durable_write_guard_test.go` at the repo root, which walks the AST of every
non-test `.go` file and fails on `os.WriteFile` outside a reasoned exemption
list, or on any file that hand-rolls a `.tmp`/`.new` sibling and renames it.
Adding a writer means either using `util.Save` or writing down why the data
is not living state.

1. **Write atomically AND durably**: use `util.Save` / `util.SafeSave`, which
   fsyncs the temp file before rename and syncs the directory after.
2. **Never conflate absent with corrupt**: `util.ReadLivingState` returns
   `util.ErrStateAbsent` vs `util.ErrStateCorrupt`. Absent legitimately means
   "first run, seed defaults"; treating corrupt as absent silently destroys
   real data.
3. **On corruption: quarantine, log ERROR, continue**: `util.QuarantineCorrupt`
   moves the file aside (nanosecond-stamped, never deletes) and leaves the
   path reading as absent so the normal seed-defaults path takes over. One
   bad byte must not take a live game offline.
4. **Persist before publishing**: build the new state as a value, write it,
   mutate the in-memory registry only after the write returns nil.

The memory file also records a testing gotcha worth carrying: `mudlog.Error`
dereferences a nil logger and panics when a package has no `TestMain`, so any
test driving a save-failure path used to crash the test binary instead of
failing an assertion, making those paths untestable rather than merely
untested.

## Migrations

Two migration frameworks exist for different scopes. The versioned framework
in `internal/migration/`, keyed on `Server.CurrentVersion`, runs as a
startup world sweep and is the right tool for world state (mob instances,
shops, room containers). The per-load MiscData-marker pattern in
`internal/characters/migrations.go` runs on character load.

[[reference-alt-characters-break-character-scoped-migrations]] documents the
trap in the second pattern: **scope the marker to the data it guards, not to
the character.** Alts are separate `Character` values, each with its own
MiscData, so each carries its own copy of a Character-scoped marker. But
`ItemStorage` (the bank) lives on `UserRecord`, not `Character`, one per
account and shared by every alt. A migration guarded only by a Character
marker therefore re-runs against the shared bank once per alt: migrate,
swap to an alt, log in again, and the new active character has no marker,
so the guard passes and the bank is migrated a second time. For a
multiplicative migration this is silent, permanent data corruption per alt.
The fix used for the bow-detune migration was an account-scoped marker,
`Storage.MigrationsDone map[string]bool` on `Storage`
(`internal/users/storage.go`), with `HasMigration`/`MarkMigration`
accessors, shared against one unguarded pure sweep function rather than
duplicating the sweep per caller.

Character collections (backpack, component bag, potion bandolier, equipped)
correctly use a Character-scoped marker. The bank does not, and needs an
account-scoped one instead.

Note also that a user-save migration reached from character load does not
touch mob equipment (`mobs.instances/**`), shop resale stock (`shops/**`),
or room containers and corpses (`rooms.instances/**`); those populations
need the versioned world-sweep framework, not a per-character hook.

## Autosave cost

[[reference-autosave-lock-cost]] establishes that autosave's world-lock-held
cost tracks activity (how many rooms/users are dirty and need a durable
write), not the size of the world. The template-cache optimization (chunk
4.6) brought the warm-path lock hold from 220ms down to 7ms for a 1386-room
world by caching the immutable authored template instead of re-reading and
re-parsing it every cycle; a dirty-set approach (3.6b-2) was evaluated and
downgraded rather than taken, because its correctness surface (roughly 80
mutation sites across 10+ packages) was far larger than the actual win at
this world's scale.

## Sources

Lifted from CLAUDE.md, file-location and do-not-wipe halves only (the
dynamic-pricing formula and shop config knobs at CLAUDE.md lines 214-218
were deliberately left in CLAUDE.md as subsystem detail for
`internal/shops/context.md`):
- "Shop Persistence (Living Economy)" (lines 206-213)
- "Moderation Persistence" (lines 220-230)

Folded memory files (rule stated inline above, cited here):
- [[reference_living_state_persistence_contract]]
- [[reference-alt-characters-break-character-scoped-migrations]]
- [[reference-autosave-lock-cost]]

Cited, not folded (a fact that drives no procedure here):
- [[reference-user-save-location]]: player/character saves live at
  `_datafiles/world/dogmud/users/<userid>.yaml`.
