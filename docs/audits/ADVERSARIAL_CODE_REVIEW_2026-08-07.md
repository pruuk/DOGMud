# DOGMud Adversarial Code Review

**Date:** 2026-08-07
**Scope:** Go server, gameplay commands, persistence, loaders, CI/tooling, tests,
and browser client
**Method:** Fresh source inspection, call-site tracing, targeted searches,
`go vet`, `go test`, and `golangci-lint`. Existing audit/review reports were not
consulted or used as input.

## Executive summary

The codebase has substantial test coverage and several good defensive patterns,
but those patterns are applied inconsistently. The most serious risks are:

1. Combat is represented by both legacy `Aggro` state and a newer combat state
   machine; failed state-machine transitions can leave the legacy state active.
2. Several persistence paths mutate memory before saving, write non-atomically,
   or treat corrupt state as absent state. Failures can therefore become silent
   resets or reappear after restart.
3. Important test suites are skipped or probabilistic, while release CI omits
   gates present on pull requests.
4. Static analysis currently reports 105 issues, including two plausible nil
   dereferences and multiple ignored persistence errors.

This review found **9 high, 18 medium, and 5 low-priority valid findings**.
Finding 1 was invalidated during remediation when a direct Git-tree check proved
that the supposedly missing asset is tracked in both `HEAD` and
`origin/master`. The first remediation pass should address findings 2–10 before
maintainability cleanup.

## Verification baseline

- `go vet ./...`: passed.
- `go mod verify`: passed.
- `golangci-lint run ./...`: failed with 105 findings:
  - 50 unchecked errors
  - 31 ineffectual assignments
  - 19 staticcheck findings
  - 2 vet findings
  - 3 unnecessary conversions
- `golangci-lint run --enable-only=dupl ./...`: found extensive duplication in
  production and test code.
- `go test ./...`: all packages that executed passed, but Windows Defender
  blocked the generated `internal/relationships.test.exe` as potentially
  unwanted software. The run is therefore **inconclusive**, not green.

The review preserved the user's existing uncommitted room changes.

---

## Invalidated

### 1. The xterm runtime is already tracked and packaged

**Evidence**

- `_datafiles/html/public/webclient-pure.html:215-219` loads
  `static/js/xterm.4.19.0.js`.
- `git ls-tree HEAD` and `git ls-tree origin/master` both resolve that exact
  path to blob `fbe149ccd58871e9df6301108a4cbedd7ad9a8a4`.
- The blob is 387,768 bytes and has SHA-256
  `f5d4d231cd6a3f6e9fb49d899427fa9409d7e4dc2344b0a3ee3a8fca15093f4b`.
- Release CI copies the containing `_datafiles` tree directly at
  `.github/workflows/build-and-release.yml:39-40`.

**Correction**

The original absence claim came from indexed file-search results that omitted
the large minified asset. Direct Git-tree inspection is authoritative and
disproves the finding. Windows `core.autocrlf=true` changes the working-tree
copy's sole LF to CRLF, but the committed blob and Linux release checkout retain
the canonical bytes.

**Disposition**

Invalidated. No release repair is required. A general asset-manifest validator
could be proposed separately as optional hardening, but this finding does not
justify adding one.

---

## High severity

### 2. Combat can start while crafting or salvaging remains active

**Evidence**

- `internal/hooks/CombatPhase_Vetoes.go:21-29` rejects combat entry while
  crafting or salvaging.
- `internal/characters/combat_state_compat.go:121-148` writes `c.Aggro` first,
  then intentionally ignores a failed `CombatPhase.TransitionToEngaging`.
- `attack` does not use the shared busy/activity gate used by special moves.
- `Character.IsInCombat` falls back to `Aggro != nil`.

**Impact**

`attack` can leave the character simultaneously crafting/salvaging and in
legacy combat state. Downstream code can disagree about whether combat started,
and crafting only stops later if damage happens to land.

**Remediation**

Make combat entry transactional: validate the state-machine transition before
publishing `Aggro`, or roll `Aggro` back on transition failure. Also apply the
shared activity gate to `attack` and `target`.

### 3. Harmful spells bypass `PlayerAttackImmune`

**Evidence**

- Melee blocks both non-combatants and `PlayerAttackImmune` mobs in
  `internal/usercommands/attack.go:166-169`.
- Harmful single-target casting checks only charm and `IsNonCombatant` in
  `internal/actions/cast.go:89-107`.
- The spell-resolution path does not re-check `PlayerAttackImmune`.

**Impact**

Quest/tutorial NPCs protected from melee can still be damaged by spells,
potentially breaking progression and creating inconsistent player-visible
rules.

**Remediation**

Centralize harmful-target authorization and call it from melee, spells, steal,
and special attacks. Re-check at spell resolution because a folded spell lands
later than target selection.

### 4. The global round counter is read and written without synchronization

**Evidence**

- `internal/util/util.go:108-127` directly reads/writes package-level
  `roundCount`.
- The world loop increments it while asynchronous LLM goroutines read it in
  `internal/llm/cache.go:28-59`.

**Impact**

This is a Go data race. Cache expiry can be early, late, or undefined under the
memory model. Other asynchronous consumers of `GetRoundCount` inherit the same
risk.

**Remediation**

Use `atomic.Uint64` for round/turn counters, or require every access to use the
same lock. Add a focused `-race` test with concurrent increments and reads.

### 5. Mob instance persistence can turn a torn write into silent progression loss

**Evidence**

- `internal/mobs/instance_save.go:148-163` writes directly with
  `os.WriteFile`, bypassing the repository's careful-save path.
- `internal/mobs/instance_save.go:173-181` treats read failure as no instance
  and malformed YAML as `nil`.

**Impact**

A crash or disk interruption can truncate a mob instance file. The next load
silently falls back to the template, discarding stats, skills, inventory, gold,
and planner state.

**Remediation**

Use atomic temp-write/rename through the shared careful-save helper. Distinguish
not-found from corrupt, quarantine corrupt files, and emit an operationally
visible error.

### 6. Corrupt shop state is treated as a brand-new shop

**Evidence**

- `internal/shops/persistence.go:333-347` returns `nil` for both missing files
  and YAML parse failures.
- Registration interprets `nil` as a fresh shop and seeds template defaults.

**Impact**

One malformed living-economy file resets stock, merchant gold, and restock
timers. The reset is destructive and looks like normal initialization.

**Remediation**

Return `(inventory, state, error)` or equivalent so “not found” and “corrupt”
cannot collapse into one value. Refuse reseeding on corruption and preserve or
quarantine the bad file.

### 7. Guild and moderation operations can report success after persistence fails

**Evidence**

- Guild membership mutates registry maps before `Save` in
  `internal/guilds/registry.go:123-139,162-177`.
- Guild, ban, and petition files use direct `os.WriteFile` in
  `internal/guilds/persistence.go:36-46`,
  `internal/moderation/bans.go:95-116`, and
  `internal/moderation/petitions.go:112-126`.
- `ban` and `unban` discard moderation errors and print success in
  `internal/usercommands/ban.go:55-56,75-85` and
  `internal/usercommands/unban.go:24-31`.

**Impact**

Memory and disk diverge. An unban can appear successful and then return after
restart; a failed guild join can still leave in-memory membership; a torn file
can erase moderation or guild state.

**Remediation**

Persist an immutable candidate snapshot atomically, then publish it to memory.
Never discard save errors in administrative commands. Add rollback/failure
tests using an unwritable directory.

### 8. Room path cache writes are not protected by a room-manager mutex

**Evidence**

- `internal/rooms/roommanager.go:80-94` reads and writes
  `roomIdToFileCache` without package-local synchronization.
- Room loads can occur from multiple connection goroutines under a shared MUD
  read lock, so the global lock does not serialize these writes.

**Impact**

Concurrent uncached room loads can trigger a fatal concurrent-map write or
produce duplicate room objects outside the manager cache.

**Remediation**

Give `RoomManager` an explicit mutex and use a read-then-write double check, or
require uncached loading on the single world worker. Add a parallel first-load
race test.

### 9. The position test matrix is mostly phantom coverage

**Evidence**

- `internal/state/position/position_test.go` contains 100 test functions and 87
  unconditional `t.Skip` calls.
- Forty skips claim coverage in
  `internal/state/position/control_test.go`, which does not exist.
- Examples are at `position_test.go:615-649`.

**Impact**

CI presents a large behavior matrix without executing it. Removed APIs are
still named as supposed replacement coverage, hiding regression gaps in a
high-risk combat-state subsystem.

**Remediation**

Delete placeholder tests or rewrite them against the current outcomes and role
APIs. Enforce a CI budget for unconditional skips and require skip reasons to
point to an existing test or issue.

### 10. Master/release CI omits lint and coverage gates

**Evidence**

- Pull-request CI runs lint and enforces 28% coverage in
  `.github/workflows/run-tests.yml:14-50`.
- Master/tag CI only runs the shared codegen/test action before building in
  `.github/workflows/build-and-release.yml:21-31`.

**Impact**

Direct pushes and release builds can ship regressions that a pull request would
reject. This matters because the current full lint backlog already includes
plausible correctness defects.

**Remediation**

Create a reusable validation workflow and require the same lint, tests,
coverage, generated-file drift check, and asset check for pull requests,
master, and tags.

---

## Medium severity

### 11. `wander loot` and `wander players` compute a filter and ignore it

`internal/mobcommands/wander.go:44-73` fills `exitOptions` with qualifying
exits, then calls `room.GetRandomExit()` instead. The modes therefore wander
randomly rather than toward loot or players.

**Fix:** select from `exitOptions`, with an explicit fallback when it is empty.
Add deterministic tests for both modes.

### 12. Compact gold-give syntax resolves one amount and transfers another

`internal/usercommands/give.go:42-45` parses through `len-5`, while
`giveObjectResolves` correctly trims the final four characters at
`give.go:339-345`. For example, compact `50gold` can resolve as 50 but execute
as 5.

**Fix:** use one strict gold parser for resolution and execution.

### 13. Zone creation reports success after ignored save failures

`modules/gmcp/gmcp.Build.go:783-808` ignores errors from
`SaveRoomTemplate` and `SaveZoneConfig`, updates in-memory plane state, and
returns `BuildResult{Ok: true}`.

**Impact:** the browser builder can claim success while leaving a partially
created zone that changes or disappears after restart.

**Fix:** fail the operation on either save error and roll back the created
artifacts.

### 14. Dialogue parse errors are cached as “no dialogue”

`internal/dialogue/loader.go:36-52` stores a nil sentinel for missing files and
YAML failures. A corrected file cannot be loaded until process restart.

**Fix:** cache only confirmed not-found results, not read/parse errors. Parse
all dialogue during production boot or expose an explicit reload invalidation.

### 15. Room instance loading continues after YAML failure

`internal/rooms/save_and_load.go:117-121` logs unmarshal failure but continues
with a potentially partial overlay.

**Impact:** players can enter a template/runtime hybrid with corrupt items,
gold, signs, containers, or defused exits.

**Fix:** reject the entire overlay on parse failure and use template state, or
fail the room load loudly.

### 16. Quest flag validation silently skips unreadable dialogue

`internal/questengine/loader.go:76-82` continues past dialogue read/unmarshal
errors while collecting flag references.

**Impact:** the validator can claim success without inspecting files that may
contain undeclared or misspelled quest flags.

**Fix:** aggregate these errors and fail validation.

### 17. Admin economy data is inserted through raw `innerHTML`

`_datafiles/html/admin/economy/index.html:313-371` concatenates discipline,
shop, and craft-support names into table HTML.

**Impact:** authored or persisted names containing markup execute in an
administrator's browser, creating a stored-XSS surface.

**Fix:** construct cells with `textContent`; reserve HTML only for trusted,
locally generated fragments.

### 18. Global keyboard capture breaks non-mouse dashboard controls

`_datafiles/html/public/webclient-pure.html:2098-2117` focuses the command input
for almost every document keydown outside a small set of form fields. Panel
buttons and tabs lose usable keyboard focus.

**Fix:** scope command shortcuts to explicit game keys and use real
`<button>` controls with labels.

### 19. Hot GMCP paths rebuild whole DOM subtrees

- `_datafiles/html/public/static/js/gmcp.js:309-329,466-502` rebuilds map room
  tokens and edges on each zone snapshot.
- `_datafiles/html/public/webclient-pure.html:919-920` wipes/recreates the
  inventory grid.
- `webclient-pure.html:1987-2057` rebuilds status condition HTML.

**Impact:** movement and combat create avoidable SVG/DOM churn, especially in
large zones and on mobile.

**Fix:** key and diff snapshots, update changed nodes only, and apply zoom once
per map batch.

### 20. HTTP servers lack defensive timeouts and trust the Host header

`internal/web/web.go:461-495` creates HTTP(S) servers without
`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, or `IdleTimeout`.
`web.go:505-516` constructs HTTPS redirects from unvalidated `r.Host`.

**Impact:** slow clients can retain connections cheaply, and forged Host
headers can produce attacker-controlled redirects.

**Fix:** configure bounded server timeouts and redirect to a configured
canonical host rather than request input.

### 21. LLM in-flight suppression has a check-then-set race

`internal/llm/client.go:48-68` calls separately locked `isPending` and
`setPending`. Two requests for one mob can both pass the check.

**Impact:** duplicate model requests and duplicate callbacks can mutate dialogue
state twice.

**Fix:** replace both calls with one locked `tryMarkPending` operation or
`LoadOrStore`.

### 22. Event dispatch invokes arbitrary callbacks while holding its registry lock

`internal/events/listeners.go:195-224` holds an exclusive `listenerLock` while
calling listeners. A listener that registers/unregisters another listener
deadlocks; all registration is also blocked by a slow handler.

**Fix:** copy listener slices under a read lock, release it, then invoke.

### 23. Production combat/position implementations are heavily duplicated

The duplication scan found, among others:

- `internal/actions/combat_rally.go:1-105` and
  `combat_warcry.go:1-107`
- `internal/state/position/disruption.go:23-87` and two corresponding blocks in
  `modifiers.go`
- `internal/mobcommands/drain.go`, `maul.go`, and `rake.go`
- `internal/mobcommands/howl.go` and `taunt.go`
- `internal/actions/plant.go:217-247` and `steal.go:277-307`

**Impact:** policy and bug fixes drift between near-identical moves. Existing
differences become difficult to distinguish from accidental omissions.

**Fix:** extract parameterized execution kernels while keeping message/power
configuration data-driven. Consolidate duplicated test fixtures separately.

### 24. Probabilistic tests skip assertions when randomness misses

`combat_drain_test.go`, `combat_throttle_test.go`, and
`spell_drainarea_test.go` retry random hit paths and call `t.Skip` if no hit is
observed.

**Impact:** CI can pass without executing the behavior under test.

**Fix:** inject deterministic RNG/roll functions and fail if a forced path does
not occur.

### 25. Generated-file and JavaScript checks are absent from CI

The shared CI action runs `go generate`, but no `git diff --exit-code` verifies
committed generated artifacts. `make test` depends on `js-lint`, while GitHub
Actions never runs it.

**Fix:** add generated-drift and JavaScript syntax/lint checks to the shared
validation workflow.

### 26. Local Makefile targets do not match the current project

- `Makefile:24` uses Go 1.21.3 while `go.mod` requires Go 1.25.
- `Makefile:67-70` deletes instance directories under `world/default` and
  `world/empty`, not the live `world/dogmud` directories.

**Impact:** local Docker behavior differs from CI/production, and `run-new`
does not perform the instance cleanup its name promises.

**Fix:** derive the Go version and world name from one source of truth; clean
both DOGMud room and mob instance directories.

### 27. Parser adoption is partial, preserving multiple argument grammars

The shared composition parser is used by `get`, while `put`, `steal`, and
`loot` retain separate multi-word container/corpse splitting and resolution.

**Impact:** fixes for ambiguous or multi-word input do not propagate across
commands.

**Fix:** migrate composition-heavy commands to the shared parser and keep
authorization/ownership gates in command handlers.

### 28. Static analysis identifies two plausible nil dereferences

- `internal/combat/combat_helpers.go:259-275` dereferences
  `raceInfo.UnarmedName` before checking `raceInfo != nil`.
- `internal/usercommands/look.go:608-615` uses `user.UserId` before a later
  `user != nil` check.

**Impact:** malformed species references or non-user look call paths can panic.

**Fix:** move nil checks before dereferences and add malformed-input tests.

---

## Low-priority debt

### 29. Dead progression hook remains exported

`internal/characters/progression.go:365-374` marks `OnLowResource` deprecated;
there are no call sites. “Kept for potential reuse” is not a maintenance
contract.

**Fix:** remove it or move the concept to a tracked design document.

### 30. Deprecated config remains shipped and documented as live

`CrafterMaterialRestockRate` is marked for removal in
`internal/configs/config.balance.go:452-455`, but is still defaulted and present
in `_datafiles/config.yaml`; `internal/mobs/context.md` still describes it as
the active cadence.

**Fix:** complete the migration and update package documentation, or explicitly
declare a compatibility window.

### 31. Dead corpse lookup arrays remain in `look`

`internal/usercommands/look.go:421-447` appends to `mobCorpses` and
`playerCorpses`, but the slices are never consumed. Staticcheck flags both.

**Fix:** remove them or use one shared disambiguation index.

### 32. `WrapAnsi` panic fallback does not return the original text

`internal/messaging/wrap.go:23-31` uses an unnamed return and a deferred
`recover`. If a panic occurs, the function returns the zero string, contrary to
its comment that the original text is returned.

**Fix:** use a named result initialized to the original text, then replace it
only after successful wrapping.

### 33. Dual YAML major versions create inconsistent persistence semantics

Both `gopkg.in/yaml.v2` and `gopkg.in/yaml.v3` are direct dependencies, and
production persistence/loaders use both.

**Fix:** either finish a validated migration or document and test explicit
subsystem boundaries. Do not perform a blind mechanical migration.

---

## Recommended remediation order

### Immediate: prevent broken releases and state loss

1. Make combat entry transactional and centralize harmful-target checks.
2. Convert round counters to atomics.
3. Introduce one atomic persistence helper and migrate mobs, guilds, moderation,
   and other living-state stores.
4. Distinguish missing, corrupt, and unreadable persisted state.
5. Propagate GMCP builder and admin-command save errors.

### Next: make the safety net trustworthy

6. Replace phantom/skipping tests with deterministic current-API tests.
7. Unify PR/master/tag validation, including lint, coverage, JS, generated
   drift, and shipped-asset checks.
8. Fix the two nil-dereference candidates and the ignored wander filter.
9. Add race tests around room loading, round counters, and LLM pending state.

### Then: reduce recurrence

10. Consolidate duplicated combat/mob command kernels.
11. Finish parser migration for composition-heavy commands.
12. Diff hot GMCP DOM updates.
13. Remove dead hooks, stale config, and misleading package documentation.
14. Burn down the 105-item lint backlog and switch full bug-finding lint from
    “new issues only” to a clean enforced baseline.

## Positive observations

- `go vet` and dependency verification are clean.
- The repository has broad package-level unit coverage and good focused tests
  in combat, persistence, state machines, and data validation.
- Several newer stores already use serialized temp-write/rename persistence;
  those implementations provide a pattern for older stores.
- Schedule and patrol loaders have strong validation and fail-fast behavior.
- Event-listener panic isolation, connection write deadlines, and queue
  synchronization are deliberate and tested.
- Mob template getters return copies, reducing accidental template mutation.

The central engineering issue is not an absence of good patterns; it is that
legacy and newer subsystems apply different contracts. Consolidating state
entry, persistence, validation, and CI around one contract will remove more
risk than isolated cosmetic cleanup.
