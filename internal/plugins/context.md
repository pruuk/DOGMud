# internal/plugins

## Purpose

Registry and lifecycle for GoMud plugins (the `modules/` tree). Owns plugin
registration, per-plugin config namespacing, embedded-file lookup, the
cross-plugin exported-function table, and the persistence helpers plugins use
to store their own state (`WriteBytes`/`ReadBytes` and the struct-marshalling
wrappers over them).

**These are compile-time plugins, not loadable ones.** A module calls
`plugins.New(name, version)` from its `init()`, decorates the returned
`*Plugin`, and the engine later walks the registry. A new plugin must be
dropped into `modules/` and the server recompiled — there is no dynamic
loading, no `.so`, and no sandbox. This package is also why `modules/` is
invisible to grep from inside `internal/`: the dependency arrow points one
way only, and everything crosses back through this registry.

Deliberately does NOT decide *when* plugin state is written to disk. Before
chunk 4.7, `Save()` was the only path and every plugin wrote synchronously,
under the world lock, on every autosave. Since 4.7 the autosave hook drives
persistence through `PrepareAll` instead, and this package's job is only to
make that swap invisible to the plugin author: `WriteBytes` still looks like a
normal call.

## Files

- `plugins.go` — the registry (`pluginRegistry`, `registry`), `Plugin` and its
  registration/lifecycle methods, `WriteBytes`/`ReadBytes` and the struct
  helpers over them, `Load`, `Save`.
- `plugincallbacks.go` — `PluginCallbacks`, the callback set a plugin can
  register (`SetIACHandler`, `SetOnLoad`, `SetOnSave`, `SetOnNetConnect`), and
  the `NetConnection` interface passed to net-connect callbacks.
- `pluginconfig.go` — `PluginConfig`, a thin wrapper that namespaces a
  plugin's config reads/writes under `Modules.<name>.*`.
- `pluginfiles.go` — `PluginFiles`, an `fs.ReadFileFS`-shaped view over a
  plugin's `embed.FS`, keyed by the short (post-`datafiles/`) path.
- `webconfig.go` — `WebConfig`/`WebPage`, a plugin's nav links and served web
  pages.
- `collector.go` — chunk 4.7: `pendingCollector`, `PrepareAll`, the
  `autosaveQueue` handle, and the G2 cancellation seam (`cancelPending`).
- `collector_test.go` — coverage for the collector and the two `WriteBytes`
  modes.

## Core types

```go
type Plugin struct {
    name    string
    version string

    dependencies []dependency

    Callbacks PluginCallbacks

    exportedFunctions map[string]any

    Config PluginConfig

    files PluginFiles // helper for embedded files

    Web WebConfig
}

type dependency struct {
    name    string
    version string
}

type pluginRegistry []*Plugin

type PluginCallbacks struct {
    userCommands map[string]usercommands.CommandAccess
    mobCommands  map[string]mobcommands.CommandAccess

    iacHandler   func(uint64, []byte) bool
    onLoad       func()
    onSave       func() error
    onNetConnect func(NetConnection)
}
```

All fields above except `Callbacks`, `Config`, and `Web` are unexported —
plugins interact with a `*Plugin` only through its methods and those three
exported sub-structs.

## The `onSave` contract

`onSave` (registered via `Callbacks.SetOnSave`) runs **under the world lock**,
either directly from `Save()` or, since chunk 4.7, from `PrepareAll` as part
of the shared autosave prepare pass (`internal/hooks.PrepareAutosaveSet`). It
may gather live state and marshal it, but only work proportional to the
plugin's OWN state — never work proportional to the size of the world (all
rooms, all players, all mobs). It must not block on I/O itself; when called
from `PrepareAll`, the `WriteBytes`/`WriteStruct` calls inside it are
automatically redirected to an in-memory collector instead of hitting disk.

`modules/auctions`'s `save()` is the reference example: it snapshots six NPC
wallet balances from a fixed registry (`npcBuyers`) into a map and marshals.
Bounded, tiny, correct. A plugin that walked every room would have the
identical call shape and be catastrophic under the lock.

A single plugin's prepare taking longer than `slowPrepareThreshold` (5ms) is
logged at WARN, naming the plugin (`collector.go`'s `PrepareAll`). That
threshold is fixed in code, not a config knob — it is pinned to the fsync cost
this design amortised away, not a balance value.

## Public API

### Registration & lifecycle

```go
func New(name string, version string) *Plugin
func (p *Plugin) Requires(modname string, modversion string)
func (p *Plugin) ExportFunction(stringId string, f any)
func (p *Plugin) AddUserCommand(command string, handlerFunc usercommands.UserCommand, allowWhenDowned bool, isAdminOnly bool)
func (p *Plugin) AddMobCommand(command string, handlerFunc mobcommands.MobCommand, allowWhenDowned bool)
func (p *Plugin) AttachFileSystem(f embed.FS) error

func Load(dataFilesPath string)
func GetPluginRegistry() pluginRegistry
func (p pluginRegistry) GetExportedFunction(funcName string) (any, bool)
func ReadFile(dfPath string) ([]byte, error)
func OnNetConnect(n NetConnection)
```

`New` sanitizes `name` to `[a-zA-Z0-9_]` and registers the plugin; it returns
`nil` if called after `Load` has run (`registrationOpen` is closed). `Load`
resolves declared dependencies (dropping any plugin whose deps are unmet, via
an exact-string match on both `name` and `version` — the source carries a
`// Later improve version matching.` TODO next to it), registers user/mob
commands, applies each plugin's `data-overlays/config.yaml` as config
overrides, and fires `onLoad`. `ExportFunction` panics if `f` is not a
function, and panics if `stringId` is already claimed by another plugin —
exported ids are a flat global namespace across every plugin, so a silent
same-id collision would resolve to whichever plugin registered first.
Package-level `ReadFile` and `pluginRegistry.ReadFile`/`Open`/`Stat` return the
first plugin's matching embedded file — `pluginRegistry` implements
`fs.ReadFileFS`, plus `NavLinks`, `HandleIAC`, `WebRequest`, and
`AllFileSubSystems`, which together satisfy the `web.WebPlugin` interface so
web pages can be served straight out of a module's embedded FS.

### Web pages

```go
func (w *WebConfig) NavLink(name string, path string)
func (w *WebConfig) WebPage(name string, path string, file string, addToNav bool, dataFunc func(r *http.Request) map[string]any)
```

### Embedded file layout

`AttachFileSystem` walks the embed and builds a short-path → embed-path map,
recognising exactly two folder names: `datafiles/`, mapped with the prefix
**stripped** (a plugin's `datafiles/help/foo.md` is fetched as `help/foo.md`),
and `data-overlays/`, mapped with the prefix **kept** (overlays are looked up
by their full `data-overlays/...` path when merging onto existing engine
data). Paths use forward slashes unconditionally; `embed.FS` does, even on
Windows.

### Persistence

```go
func (p *Plugin) WriteBytes(identifier string, bytes []byte) error
func (p *Plugin) ReadBytes(identifier string) ([]byte, error)
func (p *Plugin) WriteStruct(identifier string, in any) error
func (p *Plugin) ReadIntoStruct(identifier string, out any) error

func Save() error
```

`WriteStruct`/`ReadIntoStruct` are YAML marshal/unmarshal wrappers over
`WriteBytes`/`ReadBytes`, so `WriteStruct` inherits the deferred-during-prepare
behaviour described below. `Save()` runs every registered `onSave` hook
synchronously and returns an aggregate error naming any plugin that failed;
callers must check it (previously the underlying callback signature was
`func()`, so a failed save had no way to report itself — review finding 35).

### Autosave integration (chunk 4.7)

```go
func PrepareAll() ([]savequeue.PendingWrite, error)
func SetAutosaveQueue(q *savequeue.Queue)
```

`PrepareAll` runs every plugin's `onSave` with writes COLLECTED rather than
committed, and returns them for the caller to hand to `savequeue`. Caller must
hold the world lock. A callback that errors has its own writes discarded (it
may have gathered only part of its state) and is named in the returned error;
the other plugins still prepare. `SetAutosaveQueue` points the package at the
shared pending set; called once at startup by
`internal/hooks/autosave_prepare.go`'s `init()`.

### Callback registration

```go
func (c *PluginCallbacks) SetIACHandler(f func(uint64, []byte) bool)
func (c *PluginCallbacks) SetOnLoad(f func())
func (c *PluginCallbacks) SetOnSave(f func() error)
func (c *PluginCallbacks) SetOnNetConnect(f func(NetConnection))
```

## Gotchas

**`ReadIntoStruct` distinguishes ABSENT from CORRUPT, and callers must too.**
It returns `util.ErrStateAbsent` when the plugin has never written that
identifier (the ordinary first-run case, seed defaults) and
`util.ErrStateCorrupt` when the file exists but does not parse. On corruption it
quarantines the file, resets `out` to its zero value, and logs at ERROR, so the
caller gets clean defaults rather than a half-applied hybrid and the next read
sees ABSENT.

Until 2026-08-11 it did the opposite: it returned an error for a merely-absent
file and `nil` for a corrupt one, so a damaged data file loaded silently as zero
values. Both callers ignored the return, which is why nobody noticed.


**`New` returns `nil` after registration closes.** Registration is only open
during startup — call it from `init()` or a module's early setup, never
lazily. A `nil` return will panic at the first method call on it, which is
the intended loud failure over a silent no-op.

**`WriteBytes` has two modes and the signature does not show it.** While an
autosave prepare (`PrepareAll`) is collecting, it captures the bytes and
returns `nil` without touching disk — `nil` means "accepted for a deferred
write", NOT "already on disk". Otherwise it writes synchronously and durably
via `util.Save`. The signature is frozen because it is the plugin API
third-party modules call; that is why the mode is implicit rather than a
parameter.

**Writes made OUTSIDE a prepare must stay synchronous.** They are not part of
the atomic set, and `Queue.Supersede` (guard G3, in `internal/savequeue`)
discards an undrained pending set on the next prepare — a write enqueued
outside `PrepareAll` could be silently thrown away before it commits. That is
data loss, not a delay. `modules/weather`'s `persistState` writes on its own
tick cadence (not only via `onSave`), so this is a live path, not a
theoretical one. The collector being active only during `PrepareAll` gives
the right behaviour automatically — plugin authors don't need to know which
mode they're in.

**A synchronous write cancels a pending one for the same path** (`cancelPending`,
guard G2), because the queued entry necessarily holds older bytes than the one
just written to disk.

**`collecting` and `autosaveQueue` carry no mutex, deliberately.** Every
caller runs on `World.MainWorker` — see `internal/savequeue/context.md` for
why a mutex here would be the wrong fix, not a missing one.

**`Save()` is still the synchronous path** and is what shutdown (`main.go`)
and copyover (`copyover.go`) call directly, outside the autosave hook.
`PrepareAll` is used ONLY by the autosave prepare pass.

**`ReadIntoStruct` swallows unmarshal errors.** Its body is
`if err = yaml.Unmarshal(b, out); err == nil { return err }` followed by
`return nil` — when `Unmarshal` fails, execution falls past the `if` and
returns `nil` anyway, discarding the real error. A plugin whose saved YAML is
corrupt sees a clean `nil` return with `out` left however `Unmarshal` left it
partially populated. Do not rely on this function's return value to detect a
bad read; check the byte length or the struct contents instead if that
matters.

**Writing before `Load()` runs is not durable.** `writeFolderPath` starts as
`os.TempDir()` and is only repointed at the real plugin-data directory inside
`Load`. A `WriteBytes` call before then logs a WARN and still "succeeds",
persisting into a location nothing will read back from on the next boot.

**Plugin names are silently sanitized.** `New` replaces any character outside
`[a-zA-Z0-9_]` with `_`; a plugin registered as `"gmcp.Automation"` becomes
`gmcp_Automation` on disk (folder name in `WriteBytes`'s path). Two plugins
whose names collide only after sanitization would silently share a data
folder — not currently a problem, since existing plugin names are already
`[a-zA-Z0-9_.]`-safe, but worth checking before adding a plugin with unusual
punctuation in its name.

## Dependencies

`internal/configs`, `internal/mobcommands`, `internal/usercommands`,
`internal/mudlog`, `internal/savequeue`, `internal/util` (`util.Save`,
`util.FilePath`), `gopkg.in/yaml.v2`. Standard library: `embed`, `io/fs`,
`net/http`, `net`, `os`, `path`, `reflect`, `regexp`, `strings`, `time`.

## Consumers

- `internal/hooks/autosave_prepare.go` — calls `SetAutosaveQueue` at `init()`
  and `PrepareAll` from `PrepareAutosaveSet`, as one leg of the atomic
  room+user+plugin autosave snapshot.
- `main.go` — `plugins.Load` at startup, `plugins.Save` on shutdown,
  `plugins.GetPluginRegistry` (wired into `templates`, `usercommands`,
  `actions`, `inputhandlers`, `web`, `keywords`), `plugins.OnNetConnect` on
  each new connection.
- `copyover.go` — calls `plugins.Save` before a copyover.
- Every module under `modules/` (`auctions`, `weather`, `gmcp` and its
  sub-files, `achievements`, `cleanup`, `follow`, `leaderboards`, `playtest`,
  `time`, `webhelp`) — each calls `plugins.New` to register itself, and most
  register `Callbacks.SetOnSave` and call `WriteBytes`/`WriteStruct` on their
  `*Plugin` to persist state.
