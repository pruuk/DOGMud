# guards: Package Documentation

## Overview

The repo-wide guard tests. Each one reads the whole source tree, the shipped
world data, or a golden, and fails the build when a rule the project depends
on is broken: rules a compiler cannot see and a loader does not enforce.

The package has no production code. `doc.go` exists only so the directory is
a package; every guard is a `_test.go` file. They all lived in the repository
root (package main) until they were moved here. None of them calls code in
package main.

## What lives here

- **Source-scan guards.** Walk every non-test Go file under `internal/` and
  `modules/` (some walk the whole repo) with `go/ast`, `go/scanner` or a
  regexp, and fail on a forbidden call, a second copy of a shared body, or a
  send on the wrong category. Examples: `pool_mutation_guard_test.go`,
  `contest_floor_guard_test.go`, `sight_penalty_guard_test.go`,
  `token_engine_guard_test.go`, `refusal_phrase_guard_test.go`,
  `messaging_surface_guard_test.go`, the `*_wrapper_guard_test.go` files
  (`drink_wrapper_guard_test.go` holds `TestQuaffWrappersDoNotReFork` and
  `TestFoodWrappersDoNotReFork`).
- **Content guards.** Read the shipped YAML under `_datafiles/world/dogmud`,
  directly or through the real loaders, and fail on content that would load
  but misbehave. Examples: `condition_notice_guard_test.go`,
  `shipped_narration_data_guard_test.go`, `shop_night_trade_guard_test.go`,
  `coulee_npc_light_guard_test.go`, `topic_manifest_test.go`.
- **Goldens.** Record a rendered or computed output and compare it byte for
  byte: the lighting goldens (`lighting_*_golden_test.go`),
  `wrap_live_render_golden_test.go` and `m2_routing_guard_test.go`. Their
  files live in `testdata/` here.
- **Copy guards.** House style for player text: `prose_dash_guard_test.go`,
  `go_prose_dash_guard_test.go`, `copy_no_dash_test.go`,
  `prose_tag_spacing_guard_test.go`, `combat_idiom_guard_test.go`,
  `html_entity_guard_test.go` (no `&lt;`/`&gt;` in Go player text: ansitags
  decodes no entity, #690).
- **CI guard.** `ci_run_patterns_guard_test.go` checks every `go test -run`
  pattern in the CI action, the workflows and the Makefile names a real test
  in the package it targets.

A guard that walks or globs must prove its input is not empty (a floor on
files scanned, or a probe it must find), so a broken path fails instead of
passing on zero files. Keep that when adding one.

## How the tests find the repository root

`testmain_test.go` holds the package's `TestMain` and its one root helper:

- `TestMain` walks up from the test's working directory (Go starts it in
  `internal/guards`) to the directory holding `go.mod`, stores it in
  `repoRootPath`, and changes the working directory to it. The guards were
  written in the root, and the production loaders they call read paths
  relative to the working directory (`configs.ReloadConfig` reads
  `_datafiles/config.yaml`; the shipped config names the world as
  `_datafiles/world/dogmud`). Running from the root keeps every relative path,
  every allowlist keyed by a repo-relative path, and every loader meaning what
  it meant before the move, and a walk of `internal` or `.` still covers the
  whole repository.
- `repoRoot(t)` returns that root. A guard that used to build a path from its
  own location with `runtime.Caller` uses `repoRoot(t)` instead, since
  `runtime.Caller` now names `internal/guards`.
- `testdataPath(name)` names a golden in `internal/guards/testdata`. A bare
  `"testdata"` would resolve against the root.
- Like the root's `TestMain`, it points `CONFIG_PATH` at a scratch file and
  fails the package if the real `_datafiles/world/dogmud/config-overrides.yaml`
  was created or changed.

A guard that must skip its own file finds it with `runtime.Caller(0)`, not by
a name relative to the working directory.

## How to run them

```
go test ./internal/guards/
go test ./internal/guards/ -run TestPoolMutationGoesThroughThePrimitives -v
```

CI runs them in `go test -race ./...`. Run `go test . ./internal/guards/` as
part of every task gate: several guards are line-number allowlists over
production files (`condition_apply_path_guard_test.go`), so an edit anywhere
can turn one red.

Golden re-record flags, each only for a change whose diff shape is predicted
first (see each test's failure message):

```
go test ./internal/guards/ -run TestLightingDayCycleAcrossSampleRounds -update-lighting-daycycle -v
go test ./internal/guards/ -run TestLightingParityAcrossEveryShippedRoom -update-lighting-parity -v
go test ./internal/guards/ -run TestLightingBalanceSpread -update-lighting-balance-spread -v
go test ./internal/guards/ -run TestLightingFixtureDayCycle -update-lighting-fixture-daycycle -v
go test ./internal/guards/ -run TestLiveRenderedOutputSurvivesThePipeline -update-live-render -v
M2_RECORD_ROUTING=1 go test ./internal/guards/ -run TestM2RoutingIsFrozen
```

`TestPowerScoreTable` is opt-in: set `DOGMUD_POWERSCORE_TABLE=<file>`.

## What stays in the root

Tests that call code in package main stay in the repository root until that
code moves into a package (issue #663): the boot smoke tests
(`boot_smoke_test.go`, which CI runs by name with `DOGMUD_BOOT_SMOKE=1`),
`move_route_stamina_test.go`, `mapper_zone_routes_test.go` (with its golden
in the root `testdata/`), `main_dialogue_test.go`,
`boot_config_log_test.go`, `conversation_wiring_test.go`,
`world_unhandled_mob_command_test.go` and `bauble_sweep_test.go`. Three guards
stay with them because they share helpers with a root test:
`item_walker_guard_test.go` defines `sweepRoots`, which `bauble_sweep_test.go`
needs; `bauble_sweep_guard_test.go` uses `sweepRoots` and
`identifierGuardSkipDir`; and `identifier_word_guard_test.go` defines
`identifierGuardSkipDir`.
