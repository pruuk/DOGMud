# Lighting Plan 6 Balance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lighting plan 6 (#372): rebuild the light arithmetic so no room reads below 0 without magical darkness, floor any real light at `LightRealMinimum` 3, light the city street lamps only at night and while the clear sky is too dim to read a face, surface every lighting knob in `config.yaml` (with `LightExitsAbove` at 55), let infravision show shapes through an exit, give the rooms that promise light their light, and make dense forest cost 1.4 to cross.

**Architecture:** `internal/lightscale` moves every operation onto linear brightness, `B(p) = 2^(p/step) - 1` and `p(B) = step * log2(1 + B)`: `Combine` is `p(sum of B)`, `Attenuate` is `p(fraction * B)`, and `Trim`/`TrimDarkness` solve the linear sum. `Absent()` stays only as a marker that reads `B = 0`. `Room.composeWithFixtures` loses its `-Inf` branch and raises any real light to `LightRealMinimum` on `LightTerms.Light` before darkness is subtracted. `gametime.LampsLitAt(night, celestial, dimBelow)` is the lamplighter's rule and `gametime.LampsLit()` reads it on the current round; `BiomeInfo.StreetLamp` (`streetlamp: true`) gates the biome lamp through `BiomeInfo.LampAt(lampsLit)`, threaded through the composition as a `lampsLit` argument, and the behaviour condition `time_of_day period: lamplit` drives the North Gate arch lantern on the same test. `messaging.SensesHeatThroughExit` gives `scan` and a new `actions.LookExitShapes` the roster's anonymous figures when the exit light test fails. Rooms and the biome are data; a root test pins them.

**Tech Stack:** Go 1.25.

**Spec (binding):** `docs/superpowers/specs/2026-10-07-lighting-plan-6-balance-design.md` (owner-approved 2026-10-07, amended after the dry run), with the corrections under "Where the spec could not be implemented as written" below.

**Branch:** implementation branch `feat/lighting-plan-6`, cut from master AFTER the docs branch `docs/lighting-plan-6-spec` (the spec and this plan) merges, in the worktree `C:/tmp/dogmud-plan6`:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feat/lighting-plan-6 C:/tmp/dogmud-plan6 origin/master
```

All paths are relative to the worktree. Throwaway output goes in `$TMP`, never `C:/tmp`. Edits use the Edit and Write tools only, never a Python read-modify-write and never GNU `sed -i` (it turns CRLF into LF here). **An Edit whose old or new text ends in a space loses that space**: anchor on whole lines and run `gofmt -l`. Every commit names its paths (never `git add -A` or `git add .`) and ends with a blank line and then exactly `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; the `-m "..." -m "Co-Authored-By: ..."` form below produces that. Subagent model: opus for Tasks 2 and 4; sonnet for Tasks 1, 3, 5, 6, 7 and 8.

**How the code blocks read.** "Create" blocks are the whole file. "Modify" blocks are unified diffs against the file as the previous task left it: apply each hunk with the Edit tool (`-` lines old, `+` lines new, context lines locate it). Golden files (`testdata/*.golden`) are never inlined: each task gives the re-record command, and the spec check covers them byte for byte. The spec check for each task is `git diff <checkpoint> -- <the task's paths>` from the implementation worktree, run after staging with `--cached` as well (an untracked file shows as a deletion before it is staged); it must print nothing. The checkpoints live in the dry-run worktree `C:/tmp/dogmud-plan6-dry` on the branch `dry/plan6-clean`, which shares the repository's object store, so they are reachable by SHA with no fetch (`git cat-file -t 8d7fa2267` prints `commit`). Keep that worktree and branch until this branch merges.

**Dry run (2026-10-07).** An exploratory pass in `C:/tmp/dogmud-plan6-dry` (eleven commits from `f3144f81a` to `e56fe3c42`, the last reworking the street lamp after the owner's amended O4 ruling) was replayed onto `dry/plan6-clean` as one checkpoint per task: Task 1 `3cdbc28fc`, Task 2 `542437562`, Task 3 `93850add5`, Task 4 `adac03187`, Task 5 `a6c68e295`, Task 6 `a16cc8457`, Task 7 `eb8d826ff`, Task 8 `8d7fa2267`. `git diff e56fe3c42 8d7fa2267` shows only three comment rewrites, each made at the checkpoint that introduces the file: the bound "at or above 37" in `lighting_balance_spread_golden_test.go` (doc comment and failure message, Tasks 1 and 4), the gap note in `internal/lightscale/lightscale_test.go` and the 1.49 note in `internal/rooms/light_floor_test.go` (Task 2). The first replay without them is kept as `dry/plan6-clean-v1`. Every block below is the diff between two checkpoints. Each failing-first test was run against the previous checkpoint's code and failed as its step says. At every checkpoint `go build ./...`, `go vet ./...` and `go test ./... -count=1` gave 129 packages `ok` and no `FAIL`, and `TestEveryShopkeeperCanTradeAtNightWhileAwake` passed (before Task 4 because the biome lamps still burn at all hours). At the end: `gofmt -l internal/ modules/` clean; `golangci-lint run --new-from-merge-base=f3144f81a` `0 issues.` (one stale-cache warning naming another worktree); `python tools/context_md_audit.py` 27 phantoms in 14 packages at `f3144f81a` and at `8d7fa2267`, its output identical, the only package this plan touches that it lists being `internal/configs` with its two older entries.

---

## Facts verified against source (`f3144f81a`, 2026-10-07)

`f3144f81a` is the docs branch head; its code is master `48007844c`, where the spec's own facts table was read. Re-read for this plan by `git grep -n` and `git show` at `f3144f81a`:

| # | Fact | Where |
|---|---|---|
| G1 | `Absent()` returns `math.Inf(-1)`; `present(v)` excludes both infinities and NaN | `internal/lightscale/lightscale.go:27`, `:43` |
| G2 | `Combine(step, terms...)` is the log-domain sum (brightest plus `step*log2(sum 2^((t-best)/step))`) and returns `Absent()` for no terms; `Attenuate(step, light, fraction)` is `light + step*log2(fraction)` and returns `Absent()` for a fraction at or below 0 | `lightscale.go:55`, `:97` |
| G3 | `Trim(step, others, max, target)` and `TrimDarkness(step, light, otherDark, max, floor)` solve the log combine | `internal/lightscale/trim.go:30`, `:67` |
| G4 | `composeWithFixtures(cfg, celestial, skyFilter float64, carried, dark, fixtureLight, fixtureDark []float64) LightTerms` reads a `-Inf` light as 0 (`v := out.Light`, `v = 0`) and subtracts darkness only when it is not `-Inf` | `internal/rooms/lighting.go:137`, `:176-182`, `:195` |
| G5 | `type LightTerms struct` documents `Light`, `Dark`, `Sky`, `Fixture` and `CarriedLight` as `lightscale.Absent()` when empty | `internal/rooms/lighting.go:63` |
| G6 | Other `lightscale` users: `Character.InfraReach` combines reach (`vision.go:45`, `Combine` at `:61`); `SunLight` returns `Absent()` below the horizon and `CelestialLight` combines sun and moons; `TrimLightFor` reads `terms.Dark` through a `math.IsInf(dark, -1)` branch; item light uses `Absent()` as "off"; `conditions/effects.go` comments say "log-sums" | `internal/characters/vision.go:45,61`; `internal/gametime/celestial.go:111,114,122,189,208`; `internal/rooms/light_trim.go:71,78,81,84`; `internal/behaviortree/actions_item_light.go:127,188`; `internal/conditions/effects.go:73,276` |
| G7 | `lightnotice.attribute` blames the lamp when `HasLamp`, `Lamp` or `Fixture` moved, before the sky; `termMoved` follows | `internal/lightnotice/tracker.go:150`, `:167`, `:184` |
| G8 | `IsNight()`; `GameDate.Night`; `ReCalculate` computes night inline from `NightHoursAt(configs.GetLightingConfig().WorldLatitude, int(day))` and `hourOfDay >= nightStartHour \|\| hourOfDay < nightEndHour` | `internal/gametime/gametime.go:212`, `:104`, `:288`, `:293` |
| G9 | `condTimeOfDay` handles `range`, then `period: after_dusk`, then reads `gametime.IsNight()` and switches on `night` / `day`; any other period fails, and nothing validates period names at load | `internal/behaviortree/conditions_state.go:73`, `:122`, `:138-148` |
| G10 | The arch lantern's tree `dusk_to_dawn` lights at 52 on `period: night` | `_datafiles/world/dogmud/behaviors/items/dusk_to_dawn.yaml:14`; `items/other-0/55-arch_lantern.yaml` |
| G11 | Biome lamps: `city_thoroughfare` 52, `city_backstreet` 35, `interior` 50, `ether` 60, `spiderweb` 45 | `_datafiles/world/dogmud/biomes/city_thoroughfare.yaml:10`, `city_backstreet.yaml:10`, `interior.yaml:8`, `ether.yaml:8`, `spiderweb.yaml:7` |
| G12 | `BiomeInfo.MovementCost` (`movementcost`), `SkyLight *float64` (`skylight`), `Lamp *int` (`lamp`), `GetMovementCost`; `Room.SkyLight` and `Room.Lamp` carry `instance:"skip"` | `internal/rooms/biomes.go:22,35,50,121`; `internal/rooms/rooms.go:102-103` |
| G13 | `dense_forest` ships `movementcost: 1.0`; `forest` 1.0; `snow` 1.4 | `biomes/dense_forest.yaml:10`, `forest.yaml:10`, `snow.yaml:9` |
| G14 | `LightBlindBelow`, `LightDimBelow`, `LightExitsAbove` (default 65) are declared together; `LightDoublingStep`, `WorldLatitude`, `LightEquinoxNoon` below them; no `LightRealMinimum` or `RealMinimum` exists anywhere under `internal/` (the same `git grep` finds `LightDimBelow`) | `internal/configs/config.balance.go:1161-1163`, `:1207`, `:1233`, `:1244` |
| G15 | `validateLighting`; an out-of-range `LightExitsAbove` falls back to 65, or to `LightBlindBelow` | `internal/configs/config.balance.lighting.go:21`, `:80-83` |
| G16 | `configs.Lighting` and `GetLightingConfig()` mirror the knobs for callers | `internal/configs/config.lighting_accessor.go:11`, `:45` |
| G17 | The claim that no lighting knob appears in `config.yaml` is in five places, plus a knob count paragraph | `config.balance.go:1220`; `config.balance.lighting.go:126`; `config.lighting_accessor_test.go:12`, `:50`; `internal/gametime/context.md:162`; `internal/configs/context.md:756` |
| G18 | The committed `config.yaml` blob carries `DazzleCap` (`:939`), `LightDazzleAbove` (`:946`) and `LightInfraReachCap` (`:980`) in its lighting block and none of the twelve knobs the spec lists as absent | `git show f3144f81a:_datafiles/config.yaml` |
| G19 | `SeesThroughExit(observer, room)` gates `look <exit>` and `scan`; `ExitThroughWindow(light, strength, exitsAbove)`; own-room infravision in `SightThroughWindow` is `reach > 0 && light >= -reach` | `internal/messaging/predicates.go:96`; `window.go:72`, `:34`, `:60`; callers `internal/actions/look.go:102` (`LookExitTooDark` at `:20`) and `internal/actions/scan.go:110` |
| G20 | `UnseenFigure(d SightDecision)` is the roster's anonymous figure; `Character.Perceives(other)` | `internal/messaging/hidenames.go:39`; `internal/characters/character.go:965` |
| G21 | No Go field or flag marks a body warm or cold: `cold-blooded` is a retired mutation id and the rest is flavour text | `internal/migration/0.14.0.go:27`; `git grep -i "cold.\?blooded"` |
| G22 | Golden flags: `-update-lighting-daycycle`, `-update-lighting-parity`, `-update-lighting-fixture-daycycle`; the day-cycle golden samples days 356, 81, 172 at hours 0, 6, 12, 18 | `lighting_daycycle_golden_test.go:20`, `:41-45`; `lighting_parity_golden_test.go:21`; `lighting_fixture_daycycle_golden_test.go:20` |
| G23 | The #207 guard `TestEveryShopkeeperCanTradeAtNightWhileAwake` samples days 356, 81, 172 at every hour | `shop_night_trade_guard_test.go:33`, `:75`, `:203` |
| G24 | Cell 5105 is `biome: dungeon`, `skylight: 0.1` | `_datafiles/world/dogmud/rooms/thornwall_city/5105.yaml:11-12` |
| G25 | `gametime.ClearCelestialMemoForTest` and `ClearDateCacheForTest` exist for tests | `internal/gametime/celestial.go:229`; `gametime.go:60` |
| G26 | The main checkout's `_datafiles/config.yaml` shows `S` (skip-worktree) in `git ls-files -v`; the dry-run worktree showed `H` | `git ls-files -v _datafiles/config.yaml` |

## Where the spec could not be implemented as written

1. **The #207 guard went red under an IsNight-only lamp.** The dry run's first street lamp joined only while `gametime.IsNight()` (biome flag `lampatnight`). `TestEveryShopkeeperCanTradeAtNightWhileAwake` then failed for 12 street stall keepers at midwinter 08:00 and 16:00: day by `IsNight`, a clear sky near 40, below the faces edge, and the lamps already out. Owner ruling (O4 as amended): a lamp burns while it is night OR while the clear-sky celestial light reads below `LightDimBelow`. Task 4 builds the ruling directly, as `gametime.LampsLitAt` and `LampsLit`. Crossings on the shipped clock: midwinter out at 08.51h, lit at 15.49h; equinox 06.64h and 17.39h; midsummer 04.85h and 19.20h.
2. **The half-point bound holds only above about 37, not above 25.** The gap to the old arithmetic is `step/ln2 * (n-1) * 2^(-p/step)`; two equal terms at 17 read 25 before and 23.6 now. `TestCombineMatchesTheOldScaleWellAboveTheDimEnd` pins 37. One shipped sample moves more than half a point at or above 25: forest 4029 under a full moon, 25.784 to 26.445, level 26 unchanged.
3. **Cell 5105 at a new-moon midnight reads 1.49, not 2.4** (2.4 is `0.1 * 2^(10/8)` without the minus one of `B`). It floors to 3 either way.
4. **Equinox noon reads about 2 lower than the first draft said.** At the shipped `LightEquinoxNoon` 70 a street reads 69.4 by day, not about 72, and the nine new sky fractions read 62, 62, 58, 54, 54, 54, 52, 48, 44 at equinox noon, about 2 under the review page's figures.
5. **`LightRealMinimum` 0 means unset.** A missing key reads 0 and Go cannot tell it from an authored 0, so validation turns 0 or less into 3 and clamps a value at or above `LightBlindBelow` to `LightBlindBelow - 1` (0, no floor, when `LightBlindBelow` is 1 or below).
6. **There is no warm or cold body to filter on** (G21). Heat through an exit counts every occupant the roster would list (every mob not hidden, every player the viewer perceives), as own-room infravision already does.
7. **5255's `dark` noun ("the abrupt death of the daylight") needed rewording too**, not only its description.
8. **The stale "no lighting knobs in `config.yaml`" claim is in five places** (G17), not one. Task 5 rewrites the four in code and tests; Task 8 rewrites `internal/gametime/context.md` and the knob paragraph in `internal/configs/context.md`.
9. **A test fixture pinned a negative lamp.** `seedDarknessGateRoom` (`internal/usercommands/darkness_gates_sight_test.go`) set `room.Lamp` below 0 to make a dark room; after Task 2 a light term below 0 reads 0, so a negative pin becomes an unlit room under an `itemlight` darkness fixture of that strength.
10. **The floor sits on `LightTerms.Light`**, not only on the net reading: `Light` carries the floored value because it is the light darkness subtracts from and the light a darkness trim solves against (`TestTheFloorIsTheLightDarknessSubtractsFrom`).
11. **`lightnotice` needed `lampAgrees`.** Once street lamps follow the sky, a lamp comes on while a room darkens (noon to midnight on a backstreet), and the old rule blamed the lamp ("the lamplight fades") for a darker room. The lamp is now named only when it moved the same way as the room; otherwise the change falls through to the sky.
12. **The flag is `StreetLamp` / `streetlamp: true`**, not `lampatnight` (renamed with the ruling).
13. **`period: lamplit` has no validation list to join** (G9): `condTimeOfDay` reads period names at run time and an unknown one fails.
14. **A one-round 49 dip at lamp-out on a main street, accepted.** At the morning crossing the clear sky reaches 50 and the lamp goes out, and the street reads that sky through its 0.95 fraction, just under 50, for about a round before the sky climbs. Accepted under the ruling: the lamplighter reads the clear sky, not the street.
15. **A backstreet reads about 52 just after its lamps light, accepted.** In the evening, with the clear sky just under 50, a backstreet combines that sky with its lamp 35, above its night reading of 36 to 43, until the sky fades.

## Player-visible lines

| Where | Before | After |
|---|---|---|
| A main street by day | Daylight plus lamp 52 at all hours (midsummer noon dazzles at 75) | Daylight alone once the clear sky shows faces (equinox noon about 69, solstice noon about 74), lamps lit at night and while the sky is dim |
| The arch lantern's description (item 55) | It is lit at dusk and snuffed at first light. | It is lit when the light fails and snuffed once the day is bright. |
| A backstreet at dusk | The lamplight blamed for the darker street | The sky blamed (`lampAgrees`) |
| `look <exit>` with infravision, too dark to see through | It's too dark to see anything in that direction. | You peer toward the south. / It's too dark to see that way, but you sense the warmth of: / `  a figure, a figure` (an empty room: It's too dark to see that way, and nothing warm moves there.) |
| `scan` with infravision, too dark to see out | too dark to make anything out | `a figure` per occupant |
| `look <exit>` on a lamplit street at night | Refused (edge 65) | Still refused at 52 to 54 (edge 55); a candle (55) or better sees through |
| 5255 Mine Head | "the daylight dies behind you ... the way a door shuts", "The world you came from lies back east" | Grey light from the mouth above thins a few steps on; the way back climbs up to the mine mouth |
| 6405 | "at the far edge of your lamplight" | "the shadows along the walkway's far edge" |
| `docs/PATCH_NOTES.md` | | "2026-10-07: Light, balanced" |

## File map

| Task | Files |
|---|---|
| 1 | `internal/rooms/test_helpers.go`; `lighting_balance_spread_golden_test.go` (new), `testdata/lighting_balance_spread.golden` (new) |
| 2 | `internal/characters/vision.go`, `vision_test.go`; `internal/conditions/effects.go`; `internal/configs/config.balance.go`, `config.balance.lighting.go`, `config.lighting_accessor.go`, `config.lighting_accessor_test.go`, `config_lighting_real_minimum_test.go` (new); `internal/lightnotice/tracker.go`; `internal/lightscale/lightscale.go`, `lightscale_test.go`, `trim.go`, `trim_test.go`; `internal/rooms/biomes.go`, `carried_light_test.go`, `darkness_compose_test.go`, `darkness_trim_test.go`, `fixture_compose_test.go`, `light_floor_test.go` (new), `light_terms_test.go`, `light_trim.go`, `lighting.go`, `lighting_model_test.go`, `weather_occlusion_test.go`; `internal/usercommands/darkness_gates_sight_test.go`; `testdata/lighting_balance_spread.golden`, `lighting_daycycle.golden`, `lighting_parity.golden` |
| 3 | `lighting_no_natural_negative_test.go` (new) |
| 4 | `_datafiles/world/dogmud/behaviors/items/dusk_to_dawn.yaml`, `biomes/city_backstreet.yaml`, `biomes/city_thoroughfare.yaml`, `items/other-0/55-arch_lantern.yaml`, `narration/light-notices/lamp.yaml`; `internal/behaviortree/conditions_after_dusk_test.go`, `conditions_lamplit_test.go` (new), `conditions_state.go`, `shipped_item_trees_test.go`; `internal/gametime/celestial.go`, `gametime.go`, `lamps_lit_test.go` (new); `internal/lightnotice/integration_test.go`, `tracker.go`; `internal/rooms/biomes.go`, `fixture_compose_test.go`, `light_terms_test.go`, `light_trim.go`, `lighting.go`, `lighting_model_test.go`, `room_light_override_test.go`, `sky_filter_test.go`, `street_lamp_test.go` (new), `test_helpers.go`; `lighting_balance_spread_golden_test.go`, `lighting_no_natural_negative_test.go`; `testdata/lighting_balance_spread.golden`, `lighting_daycycle.golden` |
| 5 | `_datafiles/config.yaml`; `internal/configs/config.balance.go`, `config.balance.lighting.go`, `config.lighting_accessor_test.go`; `lighting_config_surface_test.go` (new) |
| 6 | `internal/actions/look.go`, `scan.go`, `scan_heat_test.go` (new); `internal/messaging/predicates.go`, `window.go`; `internal/mobcommands/look.go`; `internal/usercommands/darkness_gates_sight_test.go`, `look.go`, `look_exit_heat_test.go` (new) |
| 7 | `_datafiles/world/dogmud/biomes/dense_forest.yaml`; 24 room files under `_datafiles/world/dogmud/rooms/` (the 23 relit rooms 3101, 3102, 3109, 301, 310, 314, 317, 6032, 6403, 6404, 6407, 6411, 5255, 4127, 204, 6200, 488, 490, 493, 496, 497, 498, 503, and 6405, text only); `lighting_promised_light_rooms_test.go` (new); `testdata/lighting_balance_spread.golden`, `lighting_daycycle.golden`, `lighting_parity.golden` |
| 8 | `docs/PATCH_NOTES.md`, `docs/schemas/behavior.md`; `internal/actions/context.md`, `internal/behaviortree/context.md`, `internal/configs/context.md`, `internal/gametime/context.md`, `internal/lightnotice/context.md`, `internal/lightscale/context.md`, `internal/messaging/context.md`, `internal/rooms/context.md` |

---

### Task 1: golden baseline before the rebuild

Checkpoint `3cdbc28fc`. Model: sonnet.

This task records the arithmetic as it is today, so Task 2's moves can be read against it. Nothing in the game changes.

- [ ] **Step 1: The test first**

Apply both blocks below, then run `go test . -run TestLightingBalanceSpread -count=1`: it fails with `read golden: open testdata\lighting_balance_spread.golden: The system cannot find the file specified. (record it with -update-lighting-balance-spread)`, because the golden does not exist yet.

**Modify `internal/rooms/test_helpers.go`:**

````diff
@@ -1,5 +1,7 @@
 package rooms
 
+import "github.com/GoMudEngine/GoMud/internal/configs"
+
 // SeedRoomsForTest replaces the global roomManager with a fresh instance
 // populated from the supplied maps and returns a cleanup function.
 // Intended for cross-package integration tests (hooks, commands).
@@ -51,3 +53,13 @@ func SeedBiomesForTest(biomeMap map[string]*BiomeInfo) func() {
 // doc comments), so a plain literal cannot express "authored as 0".
 func SkyLightPtr(f float64) *float64 { return &f }
 func LampPtr(n int) *int             { return &n }
+
+// LightTermsAtForTest composes this room's light with the celestial term and
+// the weather sky filter supplied, rather than read from the clock and the
+// active mutators. A cross-package golden uses it to pin moon states that no
+// single round of the real clock can produce (every moon new, or every moon
+// full). Everything else (lamp, fixtures, carried light) is read as
+// LightTerms reads it.
+func (r *Room) LightTermsAtForTest(celestial, skyFilter float64) LightTerms {
+	return r.composeLight(configs.GetLightingConfig(), celestial, skyFilter)
+}
````

**Create `lighting_balance_spread_golden_test.go`:**

````go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

var updateBalanceSpread = flag.Bool("update-lighting-balance-spread", false,
	"re-record testdata/lighting_balance_spread.golden")

// balanceSpreadReviewRooms are the 45 rooms of the plan 6 review page (rooms
// whose text promises light), plus 5105 and 5106 (the holding cells whose
// night reading went negative under the old arithmetic), 5803 (a room whose
// text says the street lamps are lit in the evening) and 4111 (the North Gate,
// home of the arch lantern).
var balanceSpreadReviewRooms = []int{
	// Room lamp proposals.
	3109, 301, 310, 314, 317, 6032, 204, 6200, 488, 493, 496, 497, 498, 503,
	// Sky fraction proposals.
	3101, 3102, 6403, 6404, 6407, 6411, 5255, 4127, 490,
	// Left as they are.
	3106, 3108, 304, 6024, 6025, 6034, 6039, 6405, 6406, 6409, 6410, 6413,
	6417, 6419, 5257, 5258, 5259, 5261, 6468, 6469, 6204, 507,
	// Named in the spec.
	5105, 5106, 5803, 4111,
}

// balanceSpreadSample is one moment the golden composes every spread room at:
// a day of the year, an hour, and every moon new or every moon full.
type balanceSpreadSample struct {
	Label string
	Doy   int
	Hour  float64
	Moons float64
}

func balanceSpreadSamples() []balanceSpreadSample {
	out := []balanceSpreadSample{}
	for _, d := range []struct {
		name string
		doy  int
	}{{"midwinter", 356}, {"equinox", 81}, {"midsummer", 172}} {
		for _, h := range []struct {
			name string
			hour float64
		}{{"midnight", 0}, {"dawn", 6}, {"noon", 12}, {"dusk", 18}} {
			for _, m := range []struct {
				name string
				full float64
			}{{"new", 0}, {"full", 1}} {
				out = append(out, balanceSpreadSample{
					Label: d.name + "-" + h.name + "-moons-" + m.name,
					Doy:   d.doy, Hour: h.hour, Moons: m.full,
				})
			}
		}
	}
	return out
}

// TestLightingBalanceSpread is lighting plan 6's before-and-after record
// (spec, Testing). The day-cycle golden records integer levels at whatever
// moons each round happens to give; this one records the RAW composed light
// to a thousandth over a fixed spread of rooms, with the moons pinned new and
// full and the weather clear, so the arithmetic rebuild can be checked for
// "nothing at or above 37 moves 0.5 or more" rather than eyeballed through
// rounding.
//
// The spread is every biome's lowest-numbered room, every room with its own
// lamp or skylight override, and balanceSpreadReviewRooms. The celestial term
// is composed here from gametime.SunLight and gametime.MoonLight on the same
// operator gametime.CelestialLight uses, because no round of the real clock
// gives all three moons new (or all full) at once.
//
// It is allowed to move, but only by a diff whose shape was predicted first:
// see the failure message.
func TestLightingBalanceSpread(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	// The real shipped config, as the day-cycle golden reads it. A bare test
	// binary would otherwise walk the `default` fixture world.
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()
	conditions.LoadDataFiles()
	mutators.LoadDataFiles()

	ids := rooms.GetAllRoomIds()
	if len(ids) < 1000 {
		t.Fatalf("loaded only %d rooms: the walk is not seeing the world, so a green run proves nothing", len(ids))
	}
	sort.Ints(ids)

	want := map[int]string{}
	for _, id := range balanceSpreadReviewRooms {
		want[id] = "review"
	}
	firstOfBiome := map[string]bool{}
	for _, id := range ids {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load: the golden would silently lose coverage", id)
		}
		biome := ``
		if b := r.GetBiome(); b != nil {
			biome = b.BiomeId
		}
		if !firstOfBiome[biome] {
			firstOfBiome[biome] = true
			if _, ok := want[id]; !ok {
				want[id] = "biome"
			}
		}
		if r.Lamp != nil || r.SkyLight != nil {
			if _, ok := want[id]; !ok {
				want[id] = "override"
			}
		}
	}
	spread := make([]int, 0, len(want))
	for id := range want {
		spread = append(spread, id)
	}
	sort.Ints(spread)

	cfg := configs.GetLightingConfig()
	var b strings.Builder
	for _, s := range balanceSpreadSamples() {
		celestial := lightscale.Combine(cfg.DoublingStep,
			gametime.SunLight(cfg, s.Doy, s.Hour),
			gametime.MoonLight(cfg, s.Moons, s.Moons, s.Moons),
		)
		fmt.Fprintf(&b, "== %s\n", s.Label)
		for _, id := range spread {
			r := rooms.LoadRoom(id)
			if r == nil {
				t.Fatalf("room %d failed to load", id)
			}
			biome := ``
			if bi := r.GetBiome(); bi != nil {
				biome = bi.BiomeId
			}
			terms := r.LightTermsAtForTest(celestial, 1)
			fmt.Fprintf(&b, "room %d %s biome=%s raw=%.3f level=%d\n", id, want[id], biome, terms.Raw, terms.Level)
		}
	}
	got := b.String()

	goldenPath := filepath.Join("testdata", "lighting_balance_spread.golden")
	if *updateBalanceSpread {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("recorded %s", goldenPath)
		return
	}
	wantGolden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (record it with -update-lighting-balance-spread)", err)
	}
	if got != string(wantGolden) {
		t.Errorf("lighting balance spread golden moved.\n\n" +
			"This golden was recorded before lighting plan 6 rebuilt the light " +
			"arithmetic. It is allowed to move, but every move must be one you " +
			"predicted: the rebuild only raises the dim end (a reading at or above " +
			"37 moves by under 0.5), the night-only street lamp drops street rooms " +
			"to daylight alone by day, and the room data pass relights the rooms " +
			"the review page approved.\n\n" +
			"Work out the expected diff first, compare, and only then:\n" +
			"  go test . -run TestLightingBalanceSpread -update-lighting-balance-spread -v")
	}
}
````

- [ ] **Step 2: Record the golden and run the packages**

```bash
go test . -run TestLightingBalanceSpread -update-lighting-balance-spread -v -count=1
go test . -run TestLightingBalanceSpread -count=1
gofmt -l internal/
go build ./...
go vet ./internal/rooms .
go test ./internal/rooms . -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: `recorded testdata\lighting_balance_spread.golden`, then `ok`; then nothing from `gofmt`; then two `ok` lines. The golden holds 2304 lines: 24 samples (`== midwinter-midnight-moons-new` and so on), each listing 95 rooms as `room <id> <review|biome|override> biome=<id> raw=<x.xxx> level=<n>`.

- [ ] **Step 3: Spec check and commit**

```bash
P="internal/rooms/test_helpers.go lighting_balance_spread_golden_test.go testdata/lighting_balance_spread.golden"
git add $P
git diff --cached 3cdbc28fc -- $P
git commit -m "test(lighting): light balance spread golden before the rebuild (#372)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

The diff must print nothing: it compares the recorded golden with the dry run's byte for byte.

---

### Task 2: rebuild the light arithmetic on linear brightness

Checkpoint `542437562`. Model: opus (the arithmetic, its consumers and three goldens move together, so no commit carries a red test).

- [ ] **Step 1: The tests first**

Apply the test blocks first: `internal/characters/vision_test.go`, `internal/configs/config.lighting_accessor_test.go`, `internal/configs/config_lighting_real_minimum_test.go`, `internal/lightscale/lightscale_test.go`, `internal/lightscale/trim_test.go`, the eight `internal/rooms/*_test.go` blocks, and `internal/usercommands/darkness_gates_sight_test.go`. Against Task 1's code:

- `go vet ./internal/lightscale ./internal/configs ./internal/rooms` fails to build: `lightscale_test.go:55:13: undefined: level`; `config.lighting_accessor_test.go:122:12: c.Balance.LightRealMinimum undefined (type Balance has no field or method LightRealMinimum)`; `light_floor_test.go:64:6: cfg.RealMinimum undefined (type configs.Lighting has no field or method RealMinimum)` (and `lighting_model_test.go:11:49: unknown field RealMinimum in struct literal of type configs.Lighting`).
- With throwaway `brightness` and `level` declarations in a scratch file in `internal/lightscale` (the two functions from the `lightscale.go` block below; delete the file before Step 2), `go test ./internal/lightscale -count=1` fails ten tests against the old `Combine`, `Attenuate` and `Trim`: `TestCombineOfNothingReadsZero` (`Combine of no terms = -Inf, want 0`), `TestTwoZerosAreStillZero` (`Combine(0, 0) = 8, want 0`), `TestTwoLanternsReadSixty` (`Combine(52, 52) = 60, want about 59.94`), `TestTwoEqualSourcesApproachOneStep` (`x 5: two equal terms add 8, want under 8 ...`), `TestTwoSmallReachesReadLower` (`Combine(5, 5) = 13, want about 8.47`), `TestAttenuateNeverReadsBelowZero` (`Attenuate(0, 1e-09) = -239.1788228318901, below 0` and on), `TestAttenuateByZeroReadsZero` (`want 0, got -Inf`), `TestNegativeLightTermsAddNothing` (`Combine(-20, 10) = 10.827448753398535, want 10`), `TestTrimRoundTripsOnTheLinearSum` (`others 1 target 1.5: Trim is Absent, want a term`), `TestTrimDarknessSolvesTheDarknessCombine` (`TrimDarkness(light 50, other 20, floor 25) = 12.935406579451715, want about 16.2`).
- `go test ./internal/characters -run TestInfraReachCombinesAndCaps -count=1` fails: `5 + 5 = 13, want 8 (the linear sum; 13 under the old combine)` and `19.29 + 26 = 31, want 30`.
- `go test ./internal/usercommands -count=1` passes before and after: its block only moves the negative-lamp fixture onto a darkness fixture, which reads the same under both arithmetics.

Then apply the code blocks.

**Modify `internal/characters/vision.go`:**

````diff
@@ -37,11 +37,12 @@ func (c *Character) NightVisionStrength() int {
 //
 // Unlike nightvision it COMBINES its sources (lighting plan 5c, owner ruling):
 // every held condition's reach and every mutation's rank-scaled reach are
-// log-summed through lightscale.Combine at the light scale's doubling step,
-// the rule room light already uses, so two equal sources read one step above
-// one and a much weaker source adds almost nothing. The result is capped at
-// LightInfraReachCap and rounded once. A bare infrared flag with no number
-// still reads zero.
+// combined through lightscale.Combine at the light scale's doubling step,
+// the rule room light already uses (since lighting plan 6 a sum of linear
+// brightness), so a single source reads its own value, two equal sources
+// read a little under one step above one, and a much weaker source adds
+// almost nothing. The result is capped at LightInfraReachCap and rounded
+// once. A bare infrared flag with no number still reads zero.
 func (c *Character) InfraReach() int {
 	var vals []float64
 	for _, v := range c.Conditions.EffectValues(conditions.EffectInfraReach) {
````

**Modify `internal/characters/vision_test.go`:**

````diff
@@ -175,7 +175,7 @@ func TestInfraReachCombinesAndCaps(t *testing.T) {
 		t.Errorf("one source 30 = %d, want 30", got)
 	}
 	if got := seed(30, 30).InfraReach(); got != 38 {
-		t.Errorf("two equal sources 30 = %d, want 38 (one doubling step)", got)
+		t.Errorf("two equal sources 30 = %d, want 38 (a little under one doubling step)", got)
 	}
 	if got := seed(40, 30, 25).InfraReach(); got != 46 {
 		t.Errorf("40 + 30 + 25 = %d, want 46", got)
@@ -183,4 +183,14 @@ func TestInfraReachCombinesAndCaps(t *testing.T) {
 	if got := seed(48, 48).InfraReach(); got != 50 {
 		t.Errorf("48 + 48 = %d, want the cap 50", got)
 	}
+	// Lighting plan 6: the combine is a sum of linear brightness, so small
+	// reaches add less than the old log-domain step did (13 before, 8.5 now).
+	if got := seed(5, 5).InfraReach(); got != 8 {
+		t.Errorf("5 + 5 = %d, want 8 (the linear sum; 13 under the old combine)", got)
+	}
+	// Heat Sight from a new caster (19.29) beside a fresh Pitsense Tincture
+	// (26): 31 before plan 6, 30 now.
+	if got := seed(19.29, 26).InfraReach(); got != 30 {
+		t.Errorf("19.29 + 26 = %d, want 30", got)
+	}
 }
````

**Modify `internal/conditions/effects.go`:**

````diff
@@ -29,7 +29,7 @@ const (
 	// scale (lighting plan 5c). Independent of strength: a creature can sense
 	// heat deeply while being no better than anyone else at using faint
 	// light. Effect() aggregates it by MAX (isMax) for any reader that calls
-	// Effect(); Character.InfraReach does not, it log-sums every held
+	// Effect(); Character.InfraReach does not, it combines every held
 	// source's value through Conditions.EffectValues instead (owner ruling:
 	// reach sources combine).
 	EffectInfraReach EffectKind = `infra_reach`
@@ -273,7 +273,7 @@ func (bs *Conditions) Effect(kind EffectKind) float64 {
 // EffectValues returns every held, unexpired record's value for one kind,
 // magnitude-aware exactly as Effect reads it, in list order. It exists for a
 // reader that combines values some other way than Effect's own rule:
-// Character.InfraReach log-sums reach through lightscale.Combine (lighting
+// Character.InfraReach combines reach through lightscale.Combine (lighting
 // plan 5c). Records that do not declare the kind contribute nothing.
 func (bs *Conditions) EffectValues(kind EffectKind) []float64 {
 	var out []float64
````

**Modify `internal/configs/config.balance.go`:**

````diff
@@ -1162,6 +1162,20 @@ type Balance struct {
 	LightDimBelow   ConfigInt `yaml:"LightDimBelow"`   // Below this a normal observer reads shapes only (default 50)
 	LightExitsAbove ConfigInt `yaml:"LightExitsAbove"` // At or above this, exits into adjacent rooms are visible (default 65)
 
+	// LightRealMinimum is the least a room with ANY real light reads, before
+	// darkness is subtracted (lighting plan 6, owner ruling O3). The light
+	// arithmetic already cannot go below 0 without a darkness source; this is
+	// a rounding guard on top, so a sliver of starlight through a crack never
+	// rounds to the same 0 as a sealed cave. A room with no light at all still
+	// reads 0.
+	//
+	// Zero means unset and is coerced to the default, the house idiom for a
+	// knob a test binary would otherwise see as zero (LightDefaultVisionStrength
+	// above). The accepted authored range is therefore [1, LightBlindBelow-1]:
+	// at or above LightBlindBelow a sliver of light would let a normal
+	// observer see, which is not a rounding guard but a lamp.
+	LightRealMinimum ConfigInt `yaml:"LightRealMinimum"` // Least reading of a room with any real light (default 3)
+
 	// LightDazzleAbove is where the comfortable band ends and too-bright
 	// begins for a normal observer; a vision ability moves it down by its
 	// strength. It was the constant windowDazzleEdge until plan 5b gave dazzle
````

**Modify `internal/configs/config.balance.lighting.go`:**

````diff
@@ -85,6 +85,20 @@ func (b *Balance) validateLighting() {
 		b.LightExitsAbove = exitsFallback
 	}
 
+	// LightRealMinimum, checked after the blind/dim pair so it sees the final
+	// LightBlindBelow. Zero or negative is unset and takes the default 3. At
+	// or above LightBlindBelow it is clamped to one point below it, the
+	// LightExitsAbove precedent of clamping to the nearest valid value rather
+	// than reverting; with LightBlindBelow at 1 or below (the never-blind
+	// escape hatch) that leaves 0, no floor at all, which is the only value
+	// that cannot let a sliver of light grant sight.
+	if b.LightRealMinimum <= 0 {
+		b.LightRealMinimum = 3
+	}
+	if b.LightRealMinimum >= b.LightBlindBelow {
+		b.LightRealMinimum = max(b.LightBlindBelow-1, 0)
+	}
+
 	// LightDefaultVisionStrength is clamped to [0, 24] first, then zero
 	// (whether authored directly or reached by clamping a negative) is
 	// defaulted to 12. Doing it in that order means the accepted AUTHORED
````

**Modify `internal/configs/config.lighting_accessor.go`:**

````diff
@@ -14,6 +14,9 @@ type Lighting struct {
 	ExitsAbove            int
 	DazzleAbove           int
 	DefaultVisionStrength int
+	// RealMinimum is Balance.LightRealMinimum: the least a room with any real
+	// light reads before darkness (lighting plan 6).
+	RealMinimum int
 
 	DoublingStep  float64
 	WorldLatitude float64
@@ -55,6 +58,7 @@ func GetLightingConfig() Lighting {
 		ExitsAbove:            int(b.LightExitsAbove),
 		DazzleAbove:           int(b.LightDazzleAbove),
 		DefaultVisionStrength: int(b.LightDefaultVisionStrength),
+		RealMinimum:           int(b.LightRealMinimum),
 
 		DoublingStep:  float64(b.LightDoublingStep),
 		WorldLatitude: float64(b.WorldLatitude),
````

**Modify `internal/configs/config.lighting_accessor_test.go`:**

````diff
@@ -119,6 +119,7 @@ func TestGetLightingConfigMirrorsBalance(t *testing.T) {
 	c.Balance.LightDimBelow = 31
 	c.Balance.LightExitsAbove = 32
 	c.Balance.LightDefaultVisionStrength = 13
+	c.Balance.LightRealMinimum = 7
 	c.Balance.LightDoublingStep = 11
 	c.Balance.WorldLatitude = 12.5
 	c.Balance.LightEquinoxNoon = 14.5
@@ -140,6 +141,7 @@ func TestGetLightingConfigMirrorsBalance(t *testing.T) {
 		{"DimBelow", float64(got.DimBelow), 31},
 		{"ExitsAbove", float64(got.ExitsAbove), 32},
 		{"DefaultVisionStrength", float64(got.DefaultVisionStrength), 13},
+		{"RealMinimum", float64(got.RealMinimum), 7},
 		{"DoublingStep", got.DoublingStep, 11},
 		{"WorldLatitude", got.WorldLatitude, 12.5},
 		{"EquinoxNoon", got.EquinoxNoon, 14.5},
````

**Create `internal/configs/config_lighting_real_minimum_test.go`:**

````go
package configs

import "testing"

// LightRealMinimum (lighting plan 6, owner ruling O3) is the least a room with
// any real light reads. A test binary never loads config.yaml, so a zero must
// land on the default 3, and the knob must stay below LightBlindBelow so a
// sliver of light can never grant sight.
func TestLightRealMinimumValidation(t *testing.T) {
	cases := []struct {
		name  string
		blind ConfigInt
		dim   ConfigInt
		set   ConfigInt
		want  ConfigInt
	}{
		{"unset takes the default", 0, 0, 0, 3},
		{"negative takes the default", 0, 0, -4, 3},
		{"an authored 1 survives", 0, 0, 1, 1},
		{"an authored 24 survives under blind 25", 0, 0, 24, 24},
		{"at blind it clamps one below", 0, 0, 25, 24},
		{"above blind it clamps one below", 30, 60, 90, 29},
		{"blind 2 leaves 1", 2, 50, 3, 1},
		{"blind 1 leaves no floor", 1, 50, 3, 0},
		{"a negative blind leaves no floor", -10, 50, 3, 0},
	}
	for _, c := range cases {
		b := Balance{LightBlindBelow: c.blind, LightDimBelow: c.dim, LightRealMinimum: c.set}
		b.Validate()
		if b.LightRealMinimum != c.want {
			t.Errorf("%s: LightRealMinimum = %v, want %v (blind %v)", c.name, b.LightRealMinimum, c.want, b.LightBlindBelow)
		}
	}
}
````

**Modify `internal/lightnotice/tracker.go`:**

````diff
@@ -178,7 +178,9 @@ func attribute(prev record, now observation) Cause {
 	return CauseEyes
 }
 
-// termMoved reports whether a light-scale term changed: appeared, went Absent,
+// termMoved reports whether a light-scale term changed: appeared, went out
+// (0 since lighting plan 6, when a combine of nothing reads 0; Absent is still
+// accepted from a caller that passes one),
 // or moved by more than float noise. The sky, the darkness, the carried light
 // and the fixtures share it.
 func termMoved(a, b float64) bool {
````

**Modify `internal/lightscale/lightscale.go`:**

````diff
@@ -1,14 +1,25 @@
 // Package lightscale holds the arithmetic of DOGMud's graded light scale.
 //
-// The scale runs -100 to 100 and is PERCEPTUAL, not linear. Zero is the darkest
-// light that naturally occurs, roughly an unlit cave; negative is magical
-// darkness, light actively removed. Because the scale is logarithmic, one
-// constant relates it to physical light: the doubling step, which is how many
-// scale points twice as much light is worth.
+// The scale runs -100 to 100 and is PERCEPTUAL. Zero is the darkest a place
+// can be without active magical darkness, roughly an unlit cave; negative is
+// magical darkness, light actively removed, and nothing else (lighting plan 6,
+// owner ruling O1). One constant relates the scale to physical light: the
+// doubling step, which is how many scale points twice as much light is worth.
 //
-// 🔑 The same step governs three things, which is why it is ONE config knob:
+// 🔑 Every operation works on LINEAR brightness, not on points (lighting plan
+// 6, ruling O2). A light term of p points has brightness
+//
+//	B(p) = 2^(p/step) - 1
+//
+// and a brightness reads back as p(B) = step * log2(1 + B). B(0) is exactly 0,
+// so "no light" and "a light of 0" are the same statement, and no sum or
+// fraction of real light can read below 0. Darkness is the only way a room goes
+// negative, and the room subtracts it in points; this package never does.
+//
+// The same step governs three things, which is why it is ONE config knob:
 // combining sources, applying a sky fraction, and the shape of the daylight
-// curve. See docs/superpowers/specs/2026-09-23-graded-room-lighting-amendment-celestial.md.
+// curve. See docs/superpowers/specs/2026-09-23-graded-room-lighting-amendment-celestial.md
+// and docs/superpowers/specs/2026-10-07-lighting-plan-6-balance-design.md.
 //
 // This package is deliberately pure: no config reads, no globals, no locks. Its
 // callers own the config read, the same discipline messaging.SightThroughWindow
@@ -17,92 +28,95 @@ package lightscale
 
 import "math"
 
-// Absent is a light term that is not present at all, as distinct from a term
-// that is present and dark.
+// Absent is the marker for a term that is not there at all: a sun below the
+// horizon, a trimmed source switched off, an unlit fixture.
 //
-// 🔑 The distinction is load-bearing. A cave has no sky, which is not the same
-// as a sky contributing zero: on a logarithmic scale two terms at zero
-// legitimately combine to something brighter than one, so a "zero" sky would
-// make a cave brighter for having a sky it does not have.
+// Since lighting plan 6 it carries no arithmetic weight of its own. Combine and
+// Attenuate read it exactly as a term of 0, because a term of 0 has brightness
+// 0 and adds nothing. It survives as a MARKER, not a value: a trim result of
+// Absent means "switch this source off" (conditions.SetLightOutput lands it on
+// LightOff), and an unlit fixture records it so a look can still say "unlit".
+// Combine and Attenuate never return it; a combination of nothing reads 0.
 func Absent() float64 { return math.Inf(-1) }
 
 // present reports whether a term should take part in a combination. Only finite
-// values do; -Inf means absent and NaN means a caller made an arithmetic
-// mistake, which must not silently poison the whole room.
+// values do; -Inf is Absent and NaN means a caller made an arithmetic mistake,
+// which must not silently poison the whole room.
 //
 // ⚠️ +Inf is folded into "absent" too, which is deliberate but is NOT the same
-// judgement as the other two. -Inf is a legitimate value this package produces
-// on purpose, and NaN is excluded so one bad term cannot poison a whole room.
-// +Inf is neither: no caller can currently produce it, and if one ever does it
-// is a bug upstream, most likely a division by zero. Skipping it means that bug
-// would vanish without trace rather than reddening a test. It is folded in here
-// only because a term of infinite brightness has no sane reading on a bounded
-// -100..100 scale, so there is nothing better to do with it locally. If plans 4
-// or 5 add a source whose magnitude is computed by division, give +Inf its own
-// branch and make it loud.
+// judgement as the other two. No caller can currently produce it, and if one
+// ever does it is a bug upstream, most likely a division by zero. It is folded
+// in here only because a term of infinite brightness has no sane reading on a
+// bounded -100..100 scale. If a source whose magnitude is computed by division
+// is ever added, give +Inf its own branch and make it loud.
 func present(v float64) bool { return !math.IsInf(v, 0) && !math.IsNaN(v) }
 
-// Combine returns the light produced by every present term together.
-//
-//	combined = brightest + step * log2( sum over i of 2^((s_i - brightest)/step) )
-//
-// Two equal terms read one step brighter than one; four read two steps brighter;
-// a term far below the brightest contributes almost nothing. Absent terms are
-// skipped, and a combination of nothing is Absent.
-//
-// It is computed relative to the brightest term rather than from an absolute
-// origin so that large scale values cannot overflow the exponential.
-func Combine(step float64, terms ...float64) float64 {
+// coerceStep turns a non-positive or NaN step into 1 rather than dividing by
+// zero.
+func coerceStep(step float64) float64 {
 	if !(step > 0) {
-		step = 1
+		return 1
 	}
-	best := math.Inf(-1)
-	for _, t := range terms {
-		if present(t) && t > best {
-			best = t
-		}
+	return step
+}
+
+// brightness is B(p) = 2^(p/step) - 1 for one light term. Every term that is
+// not real light (Absent, NaN, +Inf, or at or below 0 points) reads 0. A light
+// term cannot be negative: below 0 is darkness, and darkness is never a light
+// term.
+func brightness(step, p float64) float64 {
+	if !present(p) || !(p > 0) {
+		return 0
 	}
-	if math.IsInf(best, -1) {
-		return Absent()
+	return math.Exp2(p/step) - 1
+}
+
+// level is p(B) = step * log2(1 + B), the inverse of brightness. A brightness
+// at or below 0 reads 0.
+func level(step, b float64) float64 {
+	if !(b > 0) {
+		return 0
 	}
+	return step * math.Log2(1+b)
+}
+
+// Combine returns the light produced by every term together: the sum of their
+// brightnesses, read back in points.
+//
+//	combined = p( sum over i of B(t_i) )
+//
+// A single term reads its own value. Two equal terms read a little under one
+// step brighter than one (52 and 52 read 59.9 at step 8), and the gap closes to
+// a whole step as the terms grow; a term far below the brightest contributes
+// almost nothing. Absent terms, and terms at or below 0, add nothing. A
+// combination of nothing reads 0, never Absent.
+func Combine(step float64, terms ...float64) float64 {
+	step = coerceStep(step)
 	sum := 0.0
 	for _, t := range terms {
-		if present(t) {
-			sum += math.Exp2((t - best) / step)
-		}
-	}
-	// Unreachable as written, and kept as a backstop rather than removed. Once
-	// best is finite, the very term that set it is re-encountered by this loop
-	// and contributes Exp2(0/step) = 1 exactly, so sum is always at least 1. It
-	// stays because the guarantee depends on the two loops agreeing about which
-	// terms are present: change the best-selection loop without changing the
-	// summation loop and sum could reach here as zero, whose Log2 is -Inf.
-	if !(sum > 0) {
-		return best
+		sum += brightness(step, t)
 	}
-	return best + step*math.Log2(sum)
+	return level(step, sum)
 }
 
 // Attenuate applies a transmission fraction to one light term: the share of the
 // light that gets through a canopy, a roof, a drain-cap or a blizzard.
 //
-// 🔑 On a logarithmic scale a MULTIPLIER is a SUBTRACTION. Letting half the
-// light through is minus one doubling step, whatever the light was. That is why
-// canopy, roof and weather are one operator rather than three, and why a
-// blizzard is devastating at midnight and merely gloomy at noon: it removes the
-// same number of points in both cases, but the sight bands are absolute.
+//	attenuated = p( fraction * B(light) )
 //
-// A fraction at or below zero returns Absent, not a very dark value, because "no
-// sky reaches here" is a different statement from "very little does".
+// It never reads below 0, for any fraction. Well above the dim end a halving is
+// very nearly one doubling step down, which is why canopy, roof and weather are
+// one operator rather than three; near 0 the cut shrinks toward nothing,
+// because there is almost no light left to take. A fraction at or below 0, or
+// an Absent light, reads 0: no sky reaching here and no light are the same
+// statement.
 func Attenuate(step, light, fraction float64) float64 {
-	if !present(light) || !(fraction > 0) {
-		return Absent()
+	if !present(light) || !(light > 0) || !(fraction > 0) {
+		return 0
 	}
 	if fraction >= 1 {
 		return light
 	}
-	if !(step > 0) {
-		step = 1
-	}
-	return light + step*math.Log2(fraction)
+	step = coerceStep(step)
+	return level(step, fraction*brightness(step, light))
 }
````

**Modify `internal/lightscale/lightscale_test.go`:**

````diff
@@ -5,49 +5,102 @@ import (
 	"testing"
 )
 
-func TestCombineOfNothingIsAbsent(t *testing.T) {
-	if got := Combine(8, Absent(), Absent()); !math.IsInf(got, -1) {
-		t.Fatalf("want -Inf, got %v", got)
+// oldCombine and oldAttenuate are the pre-plan-6 log-domain operators, pinned
+// as literals from master 48007844c, so the property tests can measure how far
+// the rebuild moved each reading.
+func oldCombine(step float64, terms ...float64) float64 {
+	best := math.Inf(-1)
+	for _, t := range terms {
+		if t > best {
+			best = t
+		}
 	}
+	sum := 0.0
+	for _, t := range terms {
+		sum += math.Exp2((t - best) / step)
+	}
+	return best + step*math.Log2(sum)
+}
+
+func oldAttenuate(step, light, fraction float64) float64 {
+	return light + step*math.Log2(fraction)
 }
 
-func TestCombineOfOneTermIsThatTerm(t *testing.T) {
-	if got := Combine(8, 35, Absent()); got != 35 {
-		t.Fatalf("want 35, got %v", got)
+func TestCombineOfNothingReadsZero(t *testing.T) {
+	if got := Combine(8); got != 0 {
+		t.Fatalf("Combine of no terms = %v, want 0", got)
+	}
+	if got := Combine(8, Absent(), Absent()); got != 0 {
+		t.Fatalf("Combine of two Absent terms = %v, want 0", got)
 	}
 }
 
-// Two equal sources read exactly one doubling step brighter. This is the
-// definition of the step, so it is the load-bearing test of the whole scale.
-func TestTwoEqualSourcesAddOneStep(t *testing.T) {
-	got := Combine(8, 35, 35)
-	if math.Abs(got-43) > 1e-9 {
-		t.Fatalf("want 43, got %v", got)
+// B(0) is 0, so two zeros are still zero. The old log-domain combine read
+// them one step brighter, which is why the Absent sentinel had to exist.
+func TestTwoZerosAreStillZero(t *testing.T) {
+	if got := Combine(8, 0, 0); got != 0 {
+		t.Fatalf("Combine(0, 0) = %v, want 0", got)
 	}
 }
 
-func TestFourEqualSourcesAddTwoSteps(t *testing.T) {
-	got := Combine(8, 35, 35, 35, 35)
-	if math.Abs(got-51) > 1e-9 {
-		t.Fatalf("want 51, got %v", got)
+// A single term reads its own value: p(B(x)) = x for every x >= 0.
+func TestASingleTermReadsItself(t *testing.T) {
+	for x := 0.0; x <= 100; x += 0.5 {
+		if got := Combine(8, x); math.Abs(got-x) > 1e-9 {
+			t.Errorf("Combine(%v) = %v", x, got)
+		}
+		if got := Combine(8, x, Absent()); math.Abs(got-x) > 1e-9 {
+			t.Errorf("Combine(%v, Absent) = %v", x, got)
+		}
+		if got := level(8, brightness(8, x)); math.Abs(got-x) > 1e-9 {
+			t.Errorf("p(B(%v)) = %v", x, got)
+		}
 	}
 }
 
-// A source five doublings weaker than the brightest is negligible: it
-// contributes something, but under half a point. This is what stops a
-// lantern mattering at noon, which the halving rule the spec originally
-// used could not express.
-//
-// Exact value at step 8 with terms 70 and 30:
-//
-//	70 + 8*log2(1 + 2^((30-70)/8)) = 70.35515295486763
-func TestFarWeakerSourceBarelyContributes(t *testing.T) {
-	got := Combine(8, 70, 30)
-	if got <= 70 {
-		t.Fatalf("weaker source contributed nothing: got %v, want above 70", got)
+// Two oil lanterns: 52 and 52 read 59.9, which rounds to 60 (spec section 1).
+func TestTwoLanternsReadSixty(t *testing.T) {
+	got := Combine(8, 52, 52)
+	if math.Abs(got-59.94) > 0.01 {
+		t.Fatalf("Combine(52, 52) = %v, want about 59.94", got)
 	}
-	if got-70 >= 0.5 {
-		t.Fatalf("weaker source contributed too much: got %v, want within 0.5 of 70", got)
+	if math.Round(got) != 60 {
+		t.Fatalf("Combine(52, 52) rounds to %v, want 60", math.Round(got))
+	}
+}
+
+// Two equal sources read a little under one doubling step brighter, closing to
+// a whole step as the sources grow.
+func TestTwoEqualSourcesApproachOneStep(t *testing.T) {
+	prevGap := 0.0
+	for _, x := range []float64{5, 25, 50, 75} {
+		gap := Combine(8, x, x) - x
+		if !(gap < 8) || !(gap > prevGap) {
+			t.Errorf("x %v: two equal terms add %v, want under 8 and more than at the smaller x (%v)", x, gap, prevGap)
+		}
+		prevGap = gap
+	}
+}
+
+// Combine never reads below its brightest term, and never above the old
+// log-domain combine.
+func TestCombineNeverBelowTheBrightestTerm(t *testing.T) {
+	sets := [][]float64{
+		{0, 0}, {3, 0}, {10, 10}, {5, 5}, {70, 30}, {52, 35, 12}, {1, 2, 3, 4, 5},
+		{100, 100}, {24, 24, 24, 24},
+	}
+	for _, s := range sets {
+		got := Combine(8, s...)
+		best := 0.0
+		for _, v := range s {
+			best = math.Max(best, v)
+		}
+		if got < best-1e-9 {
+			t.Errorf("Combine%v = %v, below the brightest term %v", s, got, best)
+		}
+		if old := oldCombine(8, s...); got > old+1e-9 {
+			t.Errorf("Combine%v = %v, above the old combine %v", s, got, old)
+		}
 	}
 }
 
@@ -59,10 +112,71 @@ func TestCombineIsOrderIndependent(t *testing.T) {
 	}
 }
 
-func TestAttenuateHalfIsMinusOneStep(t *testing.T) {
-	got := Attenuate(8, 60, 0.5)
-	if math.Abs(got-52) > 1e-9 {
-		t.Fatalf("want 52, got %v", got)
+// A source five doublings weaker than the brightest is negligible.
+func TestFarWeakerSourceBarelyContributes(t *testing.T) {
+	got := Combine(8, 70, 30)
+	if got <= 70 || got-70 >= 0.5 {
+		t.Fatalf("Combine(70, 30) = %v, want within (70, 70.5)", got)
+	}
+}
+
+// The two reaches of 5 the spec quotes: 13 under the old combine, 8.5 now.
+func TestTwoSmallReachesReadLower(t *testing.T) {
+	if got := oldCombine(8, 5, 5); math.Abs(got-13) > 1e-9 {
+		t.Fatalf("old Combine(5, 5) = %v, want 13", got)
+	}
+	if got := Combine(8, 5, 5); math.Abs(got-8.47) > 0.01 {
+		t.Fatalf("Combine(5, 5) = %v, want about 8.47", got)
+	}
+}
+
+// Well above the dim end the rebuild is invisible: the gap to the old reading
+// shrinks as 2^(-p/step). The bound is step/ln2 * (n-1) * 2^(-p/step) for n
+// terms, which is under 0.5 for a two-term reading at or above 37 at step 8.
+//
+// Below 37 the gap can pass half a point: at 25 two equal terms move about
+// 1.4 points (17 and 17 read 25 before, 23.6 now).
+func TestCombineMatchesTheOldScaleWellAboveTheDimEnd(t *testing.T) {
+	for a := 0.0; a <= 100; a += 1 {
+		for b := 0.0; b <= a; b += 1 {
+			got, old := Combine(8, a, b), oldCombine(8, a, b)
+			if old < 37 {
+				continue
+			}
+			if math.Abs(got-old) >= 0.5 {
+				t.Errorf("Combine(%v, %v) = %v, old %v: moved %v", a, b, got, old, old-got)
+			}
+		}
+	}
+}
+
+func TestAttenuateNeverReadsBelowZero(t *testing.T) {
+	for light := 0.0; light <= 100; light += 5 {
+		for _, f := range []float64{1e-9, 0.001, 0.01, 0.1, 0.15, 0.25, 0.5, 0.95, 1} {
+			if got := Attenuate(8, light, f); got < 0 {
+				t.Errorf("Attenuate(%v, %v) = %v, below 0", light, f, got)
+			}
+		}
+	}
+}
+
+// Well above the dim end a halving is very nearly one step down, and a window
+// at noon reads what it did before to within half a point.
+func TestAttenuateMatchesTheOldScaleWellAboveTheDimEnd(t *testing.T) {
+	for light := 0.0; light <= 100; light += 1 {
+		for _, f := range []float64{0.01, 0.1, 0.15, 0.25, 0.35, 0.45, 0.5, 0.75, 0.95} {
+			old := oldAttenuate(8, light, f)
+			if old < 37 {
+				continue
+			}
+			if got := Attenuate(8, light, f); math.Abs(got-old) >= 0.5 {
+				t.Errorf("Attenuate(%v, %v) = %v, old %v", light, f, got, old)
+			}
+		}
+	}
+	// Equinox noon (72) through an interior's 0.15 window.
+	if got, old := Attenuate(8, 72, 0.15), oldAttenuate(8, 72, 0.15); math.Abs(got-old) >= 0.5 {
+		t.Errorf("window at noon: %v, old %v", got, old)
 	}
 }
 
@@ -72,25 +186,37 @@ func TestAttenuateByOneIsIdentity(t *testing.T) {
 	}
 }
 
-// A sky fraction of zero is not "contributes zero", it is "there is no sky
-// here". A cave must not receive a term at all.
-func TestAttenuateByZeroIsAbsent(t *testing.T) {
-	if got := Attenuate(8, 60, 0); !math.IsInf(got, -1) {
-		t.Fatalf("want -Inf, got %v", got)
+// No sky and no light are the same statement now: both read 0.
+func TestAttenuateByZeroReadsZero(t *testing.T) {
+	if got := Attenuate(8, 60, 0); got != 0 {
+		t.Fatalf("want 0, got %v", got)
+	}
+	if got := Attenuate(8, Absent(), 0.5); got != 0 {
+		t.Fatalf("Attenuate of Absent = %v, want 0", got)
+	}
+}
+
+// A negative light term is not light: it reads 0 and adds nothing.
+func TestNegativeLightTermsAddNothing(t *testing.T) {
+	if got := Combine(8, -20, 10); math.Abs(got-10) > 1e-9 {
+		t.Fatalf("Combine(-20, 10) = %v, want 10", got)
+	}
+	if got := Attenuate(8, -20, 0.5); got != 0 {
+		t.Fatalf("Attenuate(-20, 0.5) = %v, want 0", got)
 	}
 }
 
-func TestAttenuateOfAbsentIsAbsent(t *testing.T) {
-	if got := Attenuate(8, Absent(), 0.5); !math.IsInf(got, -1) {
-		t.Fatalf("want -Inf, got %v", got)
+func TestNaNTermsAreSkipped(t *testing.T) {
+	if got := Combine(8, math.NaN(), 10); math.Abs(got-10) > 1e-9 {
+		t.Fatalf("Combine(NaN, 10) = %v, want 10", got)
 	}
 }
 
 func TestNonPositiveStepDoesNotPanicOrNaN(t *testing.T) {
-	if got := Combine(0, 10, 10); math.IsNaN(got) {
-		t.Fatalf("NaN from zero step")
+	if got := Combine(0, 10, 10); math.IsNaN(got) || math.IsInf(got, 0) {
+		t.Fatalf("Combine at step 0 = %v", got)
 	}
-	if got := Attenuate(-4, 10, 0.5); math.IsNaN(got) {
-		t.Fatalf("NaN from negative step")
+	if got := Attenuate(-4, 10, 0.5); math.IsNaN(got) || math.IsInf(got, 0) {
+		t.Fatalf("Attenuate at step -4 = %v", got)
 	}
 }
````

**Modify `internal/lightscale/trim.go`:**

````diff
@@ -6,19 +6,18 @@ import "math"
 // smallest cut from its full strength that keeps the combined light at or
 // below target.
 //
-// others is the combine of every other term the source joins, Absent when
-// there is none. max and the result are light-scale terms fed to Combine.
+// others is the combine of every other term the source joins, Absent (or 0)
+// when there is none. max and the result are light-scale terms fed to Combine.
 //
-// The result solves Combine(others, out) == target analytically; in floating
-// point to rounding, capped at max. A non-positive max is passed through
-// unchanged, because 0 (or below) is a legitimate light term and the caller,
-// not Trim, is responsible for refusing a strengthless source. The result is
-// Absent when others already reaches target without this source, or when the
-// term the arithmetic needs would fall below 0, the darkest light that occurs
-// naturally: such a source would have to be darker than an unlit cave to
-// matter, so it is not needed at all rather than "lit" at a meaningless
-// negative value. A linear "target - others" is wrong here: on a log scale
-// adding a source does not add its value. With others Absent the result is
+// The result solves Combine(others, out) == target on the linear sum,
+// B(out) = B(target) - B(others), analytically; in floating point to rounding,
+// capped at max. A non-positive max is passed through unchanged, because the
+// caller, not Trim, is responsible for refusing a strengthless source. The
+// result is Absent when others already reaches target without this source, or
+// when the term needed has no brightness at all: the source is not needed, so
+// it is switched off rather than "lit" at nothing. A linear "target - others"
+// is wrong here: adding a source does not add its value in points. With no
+// other light (others Absent, or at or below 0) the result is
 // min(target, max).
 //
 // A NaN target or max cannot produce a meaningful term; Trim returns Absent
@@ -31,17 +30,16 @@ func Trim(step, others, max, target float64) float64 {
 	if math.IsNaN(target) || math.IsNaN(max) {
 		return Absent()
 	}
-	if !(step > 0) {
-		step = 1
-	}
-	if !present(others) {
+	step = coerceStep(step)
+	bo := brightness(step, others)
+	if bo == 0 {
 		return math.Min(target, max)
 	}
 	if others >= target {
 		return Absent()
 	}
-	need := target + step*math.Log2(1-math.Exp2((others-target)/step))
-	if need < 0 {
+	need := level(step, brightness(step, target)-bo)
+	if !(need > 0) {
 		return Absent()
 	}
 	return math.Min(need, max)
@@ -52,16 +50,16 @@ func Trim(step, others, max, target float64) float64 {
 // that keeps light - Combine(otherDark, out) >= floor.
 //
 // light is the room's combined light (Absent reads 0, an unlit cave),
-// otherDark the combine of every other darkness in the room (Absent when
+// otherDark the combine of every other darkness in the room (Absent or 0 when
 // there is none), max the source's full strength and floor its bearer's trim
 // target, one point inside the bottom of the bearer's usable range
 // (messaging.DarknessTrimTarget).
 //
 // Keeping the room at or above floor means Combine(otherDark, d) <= light -
 // floor, which is Trim with others = otherDark and target = light - floor:
-// darkness sources combine among themselves by the same halving rule lights
-// do (lighting plan 5d, owner decision 1). A budget at or below 0 means the
-// room already sits at or below floor without this source, so it is not
+// darkness sources combine among themselves on the same linear sum lights do
+// (lighting plan 5d, owner decision 1; plan 6). A budget at or below 0 means
+// the room already sits at or below floor without this source, so it is not
 // needed and the result is Absent. That is the caller-side floor Trim's low
 // targets need, applied once here.
 func TrimDarkness(step, light, otherDark, max, floor float64) float64 {
````

**Modify `internal/lightscale/trim_test.go`:**

````diff
@@ -37,18 +37,62 @@ func TestTrimLightGoesDarkWhenTheRoomIsAlreadyBright(t *testing.T) {
 	}
 }
 
-// A room within a hair of the target needs a term below zero, the darkest
-// natural light, so the source is not needed at all rather than "lit" at a
-// meaningless negative value.
-func TestTrimLightJustBelowTargetIsAbsent(t *testing.T) {
+// A room within a hair of the target needs only a sliver of light. On the
+// linear sum that sliver is a small positive term, never a negative one (the
+// old log-domain solve needed a term below zero here and switched the source
+// off). Either way the combine lands on the target, and when float rounding
+// leaves no brightness to supply at all, the source is off.
+func TestTrimLightJustBelowTargetNeedsOnlyASliver(t *testing.T) {
 	for _, d := range []float64{1e-3, 1e-9, 1e-14, 1e-15} {
-		if got := Trim(8, 74-d, 90, 74); !math.IsInf(got, -1) {
-			t.Errorf("others 74-%v: Trim = %v, want Absent", d, got)
+		got := Trim(8, 74-d, 90, 74)
+		if math.IsInf(got, -1) {
+			continue
+		}
+		if !(got > 0 && got < 1) {
+			t.Errorf("others 74-%v: Trim = %v, want a sliver in (0, 1) or Absent", d, got)
+		}
+		if c := Combine(8, 74-d, got); math.Abs(c-74) > 1e-6 {
+			t.Errorf("others 74-%v: Combine(others, Trim) = %v, want 74", d, c)
+		}
+	}
+	if got := Trim(8, 70, 90, 74); !(got > 0 && got < 74) {
+		t.Errorf("others 70: Trim = %v, want a term in (0, 74)", got)
+	}
+}
+
+// The solve round-trips on the linear sum for every other light and target.
+func TestTrimRoundTripsOnTheLinearSum(t *testing.T) {
+	for others := 1.0; others < 90; others += 3 {
+		for target := others + 0.5; target <= 100; target += 7 {
+			out := Trim(8, others, 200, target)
+			if math.IsInf(out, -1) {
+				t.Errorf("others %v target %v: Trim is Absent, want a term", others, target)
+				continue
+			}
+			if out < 0 {
+				t.Errorf("others %v target %v: Trim = %v, below 0", others, target, out)
+			}
+			if got := Combine(8, others, out); math.Abs(got-target) > 1e-9 {
+				t.Errorf("others %v target %v: Combine(others, Trim) = %v", others, target, got)
+			}
 		}
 	}
-	// Just far enough below that a real (non-negative) term is needed.
-	if got := Trim(8, 70, 90, 74); !(got >= 0 && got < 74) {
-		t.Errorf("others 70: Trim = %v, want a term in [0, 74)", got)
+}
+
+// TrimDarkness round-trips the same way: the room it leaves sits on the floor.
+func TestTrimDarknessRoundTripsOnTheFloor(t *testing.T) {
+	for light := 0.0; light <= 90; light += 10 {
+		for _, otherDark := range []float64{0, 5, 12} {
+			for _, floor := range []float64{-50, -30, 1, 25} {
+				out := TrimDarkness(8, light, otherDark, 200, floor)
+				if math.IsInf(out, -1) {
+					continue
+				}
+				if got := light - Combine(8, otherDark, out); math.Abs(got-floor) > 1e-9 {
+					t.Errorf("light %v otherDark %v floor %v: room = %v", light, otherDark, floor, got)
+				}
+			}
+		}
 	}
 }
 
@@ -126,12 +170,12 @@ func TestTrimDarknessKeepsTheRoomOnTheFloor(t *testing.T) {
 	}
 }
 
-// Other darkness below the budget: the source solves the halving rule, not a
-// linear cut. Two darknesses of 20 and 12.9 combine to 25, not 32.9.
+// Other darkness below the budget: the source solves the darkness combine, not
+// a point cut. Two darknesses of 20 and 16.2 combine to 25, not 36.2.
 func TestTrimDarknessSolvesTheDarknessCombine(t *testing.T) {
 	out := TrimDarkness(8, 50, 20, 50, 25)
-	if !(out > 12.9 && out < 13.0) {
-		t.Fatalf("TrimDarkness(light 50, other 20, floor 25) = %v, want about 12.9", out)
+	if !(out > 16.1 && out < 16.3) {
+		t.Fatalf("TrimDarkness(light 50, other 20, floor 25) = %v, want about 16.2", out)
 	}
 	if got := 50 - Combine(8, 20, out); math.Abs(got-25) > 1e-9 {
 		t.Errorf("room after = %v, want exactly the floor 25", got)
````

**Modify `internal/rooms/biomes.go`:**

````diff
@@ -24,7 +24,8 @@ type BiomeInfo struct {
 	// SkyLight is the fraction of the open sky's light that reaches this
 	// biome's floor: 1.0 for a desert dune, about 0.45 under forest canopy,
 	// 0.0 at the back of a cave. It is applied as an attenuation on the
-	// logarithmic light scale, so it is the SAME operator weather occlusion
+	// light scale (a multiplier on linear brightness since lighting plan 6),
+	// so it is the SAME operator weather occlusion
 	// uses in plan 4. Canopy, roof, drain-cap and blizzard are one idea.
 	//
 	// 🔑 It is a POINTER because zero is meaningful. A cave's sky fraction is
````

**Modify `internal/rooms/carried_light_test.go`:**

````diff
@@ -28,10 +28,10 @@ func TestCarriedSourcesEachJoinTheCombine(t *testing.T) {
 	if want := lightscale.Combine(cfg.DoublingStep, 56, 56); math.Abs(two.Raw-want) > 1e-9 {
 		t.Errorf("Raw = %v, want %v", two.Raw, want)
 	}
-	// Since lighting plan 5d, Raw is the net light (Absent light reads 0), and
+	// Since lighting plan 5d, Raw is the net light; since plan 6 no light reads 0, and
 	// the light combine alone is Light.
-	if empty := cave.composeWith(cfg, 60, 1, nil, nil); !math.IsInf(empty.Light, -1) || empty.Raw != 0 || empty.Level != 0 {
-		t.Errorf("an empty cave: Light %v Raw %v Level %d, want -Inf, 0 and 0", empty.Light, empty.Raw, empty.Level)
+	if empty := cave.composeWith(cfg, 60, 1, nil, nil); empty.Light != 0 || empty.Raw != 0 || empty.Level != 0 {
+		t.Errorf("an empty cave: Light %v Raw %v Level %d, want 0, 0 and 0", empty.Light, empty.Raw, empty.Level)
 	}
 }
 
````

**Modify `internal/rooms/darkness_compose_test.go`:**

````diff
@@ -52,19 +52,19 @@ func TestDarknessTermsAreReported(t *testing.T) {
 	cave := Room{SkyLight: &zero}
 
 	dark := cave.composeWith(cfg, 60, 1, nil, []float64{50, 50})
-	if !math.IsInf(dark.Light, -1) || dark.Carried {
-		t.Errorf("darkness alone: Light %v Carried %v, want -Inf and false", dark.Light, dark.Carried)
+	if dark.Light != 0 || dark.Carried {
+		t.Errorf("darkness alone: Light %v Carried %v, want 0 and false", dark.Light, dark.Carried)
 	}
 	if want := lightscale.Combine(cfg.DoublingStep, 50, 50); math.Abs(dark.Dark-want) > 1e-9 || !dark.Darkened {
 		t.Errorf("darkness alone: Dark %v Darkened %v, want %v and true", dark.Dark, dark.Darkened, want)
 	}
 	if math.Abs(dark.Raw-(-dark.Dark)) > 1e-9 {
-		t.Errorf("darkness alone: Raw %v, want %v (Absent light reads 0)", dark.Raw, -dark.Dark)
+		t.Errorf("darkness alone: Raw %v, want %v (no light reads 0)", dark.Raw, -dark.Dark)
 	}
 
 	lit := cave.composeWith(cfg, 60, 1, []float64{56}, nil)
-	if !lit.Carried || lit.Darkened || !math.IsInf(lit.Dark, -1) || lit.Light != 56 || lit.Raw != 56 {
-		t.Errorf("a torch alone: %+v, want Carried, not Darkened, Dark -Inf, Light and Raw 56", lit)
+	if !lit.Carried || lit.Darkened || lit.Dark != 0 || lit.Light != 56 || lit.Raw != 56 {
+		t.Errorf("a torch alone: %+v, want Carried, not Darkened, Dark 0, Light and Raw 56", lit)
 	}
 
 	both := cave.composeWith(cfg, 60, 1, []float64{56}, []float64{50})
````

**Modify `internal/rooms/darkness_trim_test.go`:**

````diff
@@ -108,7 +108,7 @@ func TestUmbralLanternTrimsToItsBearersEyes(t *testing.T) {
 		{"infravision 30, cave", 0, []int{darkInfra30Id}, 0, false, 29, -29},
 		{"infravision 50, cave", 0, []int{darkInfra50Id}, 0, false, 49, -49},
 		{"infravision 50, light 50: full", 50, []int{darkInfra50Id}, 0, false, 50, 0},
-		{"normal eyes, light 50, other darkness 20", 50, nil, 20, false, 9.83, 26},
+		{"normal eyes, light 50, other darkness 20", 50, nil, 20, false, 13.93, 26},
 		{"normal eyes, light 50, other darkness 30: off", 50, nil, 30, true, 0, 20},
 	}
 	for i, c := range cases {
````

**Modify `internal/rooms/fixture_compose_test.go`:**

````diff
@@ -5,7 +5,6 @@ import (
 	"testing"
 
 	"github.com/GoMudEngine/GoMud/internal/itemlight"
-	"github.com/GoMudEngine/GoMud/internal/lightscale"
 	"github.com/GoMudEngine/GoMud/internal/uuid"
 )
 
@@ -25,16 +24,16 @@ func TestFixtureComposition(t *testing.T) {
 		carried, dark    []float64
 		fxLight, fxDark  []float64
 		want             int
-		wantFixture      float64 // Absent when none
-		wantCarriedLight float64 // Absent when none
+		wantFixture      float64 // 0 when none (lighting plan 6: Combine of nothing reads 0)
+		wantCarriedLight float64 // 0 when none (lighting plan 6: Combine of nothing reads 0)
 	}{
-		{"a fixture alone", cave, nil, nil, []float64{52}, nil, 52, 52, lightscale.Absent()},
-		{"5000: lamp 38, stones at their trough", tavern, nil, nil, []float64{20}, nil, 40, 20, lightscale.Absent()},
-		{"5000: lamp 38, stones at their crest", tavern, nil, nil, []float64{36}, nil, 45, 36, lightscale.Absent()},
+		{"a fixture alone", cave, nil, nil, []float64{52}, nil, 52, 52, 0},
+		{"5000: lamp 38, stones at their trough", tavern, nil, nil, []float64{20}, nil, 40, 20, 0},
+		{"5000: lamp 38, stones at their crest", tavern, nil, nil, []float64{36}, nil, 45, 36, 0},
 		{"a fixture and a carried torch", cave, []float64{56}, nil, []float64{52}, nil, 62, 52, 56},
-		{"two fixtures", cave, nil, nil, []float64{52, 52}, nil, 60, 60, lightscale.Absent()},
-		{"a darkness fixture under a lamp", tavern, nil, nil, nil, []float64{30}, 8, lightscale.Absent(), lightscale.Absent()},
-		{"a darkness fixture and a carried darkness", cave, nil, []float64{50}, nil, []float64{50}, -58, lightscale.Absent(), lightscale.Absent()},
+		{"two fixtures", cave, nil, nil, []float64{52, 52}, nil, 60, 60, 0},
+		{"a darkness fixture under a lamp", tavern, nil, nil, nil, []float64{30}, 8, 0, 0},
+		{"a darkness fixture and a carried darkness", cave, nil, []float64{50}, nil, []float64{50}, -58, 0, 0},
 	}
 	for _, c := range cases {
 		got := c.room.composeWithFixtures(cfg, 60, 1, c.carried, c.dark, c.fxLight, c.fxDark)
````

**Create `internal/rooms/light_floor_test.go`:**

````go
package rooms

import (
	"math"
	"testing"
)

// The floor (lighting plan 6, owner ruling O3): any real light reads at least
// LightRealMinimum before darkness is subtracted, and a room with none reads
// exactly 0.
func TestRealLightReadsAtLeastTheFloor(t *testing.T) {
	cfg := modelCfg() // RealMinimum 3
	zero := 0.0
	tenth := 0.1
	cave := Room{SkyLight: &zero}
	cell := Room{SkyLight: &tenth} // 5105's sky fraction

	cases := []struct {
		name      string
		room      Room
		celestial float64
		carried   []float64
		dark      []float64
		wantLevel int
		wantRaw   float64 // NaN: not checked
	}{
		{"a sealed cave reads 0", cave, 10, nil, nil, 0, 0},
		{"a cell under starlight alone is floored to 3", cell, 10, nil, nil, 3, 3},
		{"a carried sliver is floored to 3", cave, 10, []float64{0.4}, nil, 3, 3},
		{"light at the floor is untouched", cave, 10, []float64{3}, nil, 3, 3},
		{"light above the floor is untouched", cave, 10, []float64{4.6}, nil, 5, 4.6},
		{"darkness subtracts from the floored light", cell, 10, nil, []float64{10}, -7, -7},
		{"darkness alone in a cave reads below 0", cave, 10, nil, []float64{10}, -10, -10},
	}
	for _, c := range cases {
		got := c.room.composeWith(cfg, c.celestial, 1, c.carried, c.dark)
		if got.Level != c.wantLevel {
			t.Errorf("%s: Level = %d, want %d", c.name, got.Level, c.wantLevel)
		}
		if !math.IsNaN(c.wantRaw) && math.Abs(got.Raw-c.wantRaw) > 1e-9 {
			t.Errorf("%s: Raw = %v, want %v", c.name, got.Raw, c.wantRaw)
		}
	}
}

// Light carries the floored value: it is what darkness subtracts from, so a
// darkness trim solving against it leaves the room exactly on its floor.
func TestTheFloorIsTheLightDarknessSubtractsFrom(t *testing.T) {
	tenth := 0.1
	cell := Room{SkyLight: &tenth}
	got := cell.composeWith(modelCfg(), 10, 1, nil, []float64{2})
	if got.Light != 3 {
		t.Errorf("Light = %v, want the floored 3", got.Light)
	}
	if math.Abs(got.Raw-(got.Light-got.Dark)) > 1e-9 {
		t.Errorf("Raw = %v, want Light - Dark = %v", got.Raw, got.Light-got.Dark)
	}
}

// With the knob at 0 (only reachable with LightBlindBelow at 1 or below) there
// is no floor.
func TestAFloorOfZeroIsNoFloor(t *testing.T) {
	cfg := modelCfg()
	cfg.RealMinimum = 0
	zero := 0.0
	cave := Room{SkyLight: &zero}
	if got := cave.composeWith(cfg, 10, 1, []float64{0.4}, nil); math.Abs(got.Raw-0.4) > 1e-9 {
		t.Errorf("Raw = %v, want 0.4 with no floor", got.Raw)
	}
}

// Cell 5105 at night with no moons: the old arithmetic read -16.6, by
// construction a natural negative. Now the sky reads about 1.49, a tenth of
// starlight's brightness B(10) = 2^(10/8) - 1 read back in points, and is
// floored to 3.
func TestTheHoldingCellAtANewMoonMidnightIsBarelyLit(t *testing.T) {
	tenth := 0.1
	cell := Room{SkyLight: &tenth}
	got := cell.composeWith(modelCfg(), 10, 1, nil, nil)
	if math.Abs(got.Sky-1.49) > 0.01 {
		t.Errorf("Sky = %v, want about 1.49", got.Sky)
	}
	if got.Level != 3 {
		t.Errorf("Level = %d, want 3", got.Level)
	}
}
````

**Modify `internal/rooms/light_terms_test.go`:**

````diff
@@ -46,7 +46,7 @@ func TestLightTermsReportEachTerm(t *testing.T) {
 	}
 
 	cave := Room{SkyLight: &zero}
-	if c := cave.composeLight(cfg, 60, 1); !math.IsInf(c.Sky, -1) || c.HasLamp {
-		t.Errorf("a cave has no sky term and no lamp, got Sky=%v HasLamp=%v", c.Sky, c.HasLamp)
+	if c := cave.composeLight(cfg, 60, 1); c.Sky != 0 || c.HasLamp {
+		t.Errorf("a cave has no sky light (0) and no lamp, got Sky=%v HasLamp=%v", c.Sky, c.HasLamp)
 	}
 }
````

**Modify `internal/rooms/light_trim.go`:**

````diff
@@ -1,7 +1,6 @@
 package rooms
 
 import (
-	"math"
 	"slices"
 
 	"github.com/GoMudEngine/GoMud/internal/characters"
@@ -77,11 +76,9 @@ func (r *Room) TrimLightFor(c *characters.Character) {
 		if t.dark {
 			out = lightscale.TrimDarkness(cfg.DoublingStep, terms.Light, terms.Dark, full, darkFloor)
 		} else {
-			dark := terms.Dark
-			if math.IsInf(dark, -1) {
-				dark = 0
-			}
-			out = lightscale.Trim(cfg.DoublingStep, terms.Light, full, lightTarget+dark)
+			// terms.Dark is 0 when the room holds no darkness (lighting
+			// plan 6: a combine of nothing reads 0, never Absent).
+			out = lightscale.Trim(cfg.DoublingStep, terms.Light, full, lightTarget+terms.Dark)
 		}
 		if out >= full {
 			// No cut at all: the source runs at full strength, so it is not "trimmed".
````

**Modify `internal/rooms/lighting.go`:**

````diff
@@ -15,18 +15,20 @@ import (
 
 // LightLevel reports the room's light on the graded -100 to 100 scale.
 //
-// Three kinds of term compose it, all on one logarithmic operator:
+// Three kinds of term compose it, all on one operator, the sum of linear
+// brightness (lightscale.Combine, lighting plan 6):
 //
 //  1. The sky, which is the celestial term attenuated by this room's sky
-//     fraction. A room with no sky receives no term at all, which is not the
-//     same as receiving a term of zero.
+//     fraction. A room with no sky receives a term of 0, which adds nothing.
 //  2. The room's own lamp, if it has one, joining the combine rather than
 //     acting as a floor, so a lantern-lit tavern plus a carried torch does not
 //     double-count.
 //  3. Everything anyone in the room carries, one term per light.
 //
-// Every carried darkness is then combined on the same operator and taken
-// away from the result, so a room can read below 0 (lighting plan 5d).
+// Any real light then reads at least LightRealMinimum (plan 6, ruling O3).
+// Every carried darkness is combined on the same operator and taken away
+// from the result in points, which is the only way a room reads below 0
+// (lighting plan 5d; plan 6, ruling O1).
 //
 // Weather attenuates the SKY only, through each active mutator's skylight
 // fraction: a blizzard does not dim a lantern.
@@ -63,20 +65,19 @@ func (r *Room) lightLevelWithSkyFilter(cfg configs.Lighting, celestial, skyFilte
 type LightTerms struct {
 	// Level is exactly LightLevel(): both come from composeLight.
 	Level int
-	// Raw is the net light Level rounds and clamps: Light (read as 0 when
-	// Absent) minus Dark (read as 0 when Absent). An unlit room with no
-	// darkness is 0 (lighting plan 5d, ruling D3).
+	// Raw is the net light Level rounds and clamps: Light minus Dark. An
+	// unlit room with no darkness is 0 (lighting plan 5d, ruling D3).
 	Raw float64
 	// Light is the combined light of the sky, the lamp and every carried
-	// light; lightscale.Absent() when nothing lights the room. A light's trim
-	// solves against it.
+	// light, raised to LightRealMinimum when it is above 0 but below it
+	// (lighting plan 6, ruling O3); 0 when nothing lights the room, and never
+	// below 0. A light's or a darkness's trim solves against it.
 	Light float64
-	// Dark is the combined darkness every carried darkness takes away, by the
-	// same halving rule; lightscale.Absent() when nobody carries one
-	// (lighting plan 5d).
+	// Dark is the combined darkness every carried darkness takes away, on
+	// the same combine; 0 when nobody carries one (lighting plan 5d).
 	Dark float64
 	// Sky is the sky term after the sky fraction and the weather filter, in
-	// light-scale units; lightscale.Absent() when the room has no sky.
+	// light-scale units; 0 when the room has no sky.
 	Sky float64
 	// SkyFilter is the fraction of the sky active weather lets through: the
 	// product of the active mutators' skylight values, 1 when clear.
@@ -89,12 +90,12 @@ type LightTerms struct {
 	// Darkened reports that someone in the room carries a darkness.
 	Darkened bool
 	// Fixture is the combined light of the room's lit light fixtures (items
-	// with `fixture: light`, internal/itemlight); lightscale.Absent() when
+	// with `fixture: light`, internal/itemlight); 0 when
 	// none is lit. A fixture is part of the room, never a carried light, so
 	// it never sets Carried (lighting 5e, X5).
 	Fixture float64
-	// CarriedLight is the combine of carried light alone;
-	// lightscale.Absent() when nobody here carries a lit light. A lantern
+	// CarriedLight is the combine of carried light alone; 0 when nobody
+	// here carries a lit light. A lantern
 	// dimming while still lit, or a second light arriving, moves it where
 	// Carried does not (lighting 5e, X5).
 	CarriedLight float64
@@ -128,10 +129,10 @@ func (r *Room) composeWith(cfg configs.Lighting, celestial, skyFilter float64, c
 
 // composeWithFixtures is the composition itself, with every term supplied.
 //
-// Lights combine as they always have; darknesses combine among themselves by
-// the same halving rule; the net light is the combined light (0 when none)
-// minus the combined darkness (0 when none), clamped to [-100, 100]
-// (lighting plan 5d, owner decision 1). A lit light fixture is one term in
+// Lights combine on the linear sum; darknesses combine among themselves the
+// same way; the net light is the combined light (0 when none, at least
+// LightRealMinimum when any) minus the combined darkness (0 when none),
+// clamped to [-100, 100] (lighting plan 5d, owner decision 1; plan 6). A lit light fixture is one term in
 // the light combine and a darkness fixture one in the darkness combine,
 // beside what people carry (lighting 5e, Rule 10). Fixtures never trim.
 func (r *Room) composeWithFixtures(cfg configs.Lighting, celestial, skyFilter float64, carried, dark, fixtureLight, fixtureDark []float64) LightTerms {
@@ -144,11 +145,11 @@ func (r *Room) composeWithFixtures(cfg configs.Lighting, celestial, skyFilter fl
 	terms := make([]float64, 0, 2+len(fixtureLight)+len(carried))
 
 	// 1. The sky, attenuated by this room's fraction and then by any weather
-	// filtering it. A filter multiplies the fraction, which on this log scale
-	// is a fixed subtraction: 0.5 removes one doubling step at any hour, so a
-	// storm is merely gloomy at noon and blinding at midnight. Attenuate
-	// returns Absent for a fraction of zero, so a cave contributes no term
-	// rather than a term of zero.
+	// filtering it. A filter multiplies the fraction, and the fraction
+	// multiplies the sky's brightness: 0.5 removes very nearly one doubling
+	// step by day and less at night, where there is less light to take, so a
+	// storm is merely gloomy at noon and blinding at midnight. A fraction of
+	// zero reads 0, which adds nothing, so a cave needs no special case.
 	out.Sky = lightscale.Attenuate(step, celestial, r.skyLightFraction()*skyFilter)
 	terms = append(terms, out.Sky)
 
@@ -172,18 +173,24 @@ func (r *Room) composeWithFixtures(cfg configs.Lighting, celestial, skyFilter fl
 		terms = append(terms, carried...)
 	}
 
+	// The combined light. It cannot read below 0 (lighting plan 6): no light
+	// at all reads exactly 0, an unlit cave, and only magical darkness goes
+	// below it.
+	//
+	// The floor (lighting plan 6, owner ruling O3): any real light reads at
+	// least LightRealMinimum before darkness is subtracted, so a sliver of
+	// starlight through a crack never rounds to the same 0 as a sealed cave.
+	// Light carries the floored value, because it is the light darkness
+	// subtracts from, and so the light a darkness trim must solve against.
 	out.Light = lightscale.Combine(step, terms...)
-	v := out.Light
-	if math.IsInf(v, -1) {
-		// No light of any kind. Zero is the darkest light that NATURALLY
-		// occurs, which is what an unlit cave is. Only magical darkness goes
-		// below it.
-		v = 0
+	if floor := float64(cfg.RealMinimum); out.Light > 0 && out.Light < floor {
+		out.Light = floor
 	}
+	v := out.Light
 
 	// 5. Every darkness anyone here carries (lighting plan 5d), and every
-	// darkness fixture (lighting 5e), combined among themselves by the same
-	// halving rule and taken away from the light. Two darknesses of 50 take
+	// darkness fixture (lighting 5e), combined among themselves on the same
+	// linear sum and taken away from the light in points. Two darknesses of 50 take
 	// 58, not 100. Darkened still means a CARRIED darkness.
 	darkTerms := make([]float64, 0, len(dark)+len(fixtureDark))
 	darkTerms = append(darkTerms, dark...)
@@ -192,9 +199,7 @@ func (r *Room) composeWithFixtures(cfg configs.Lighting, celestial, skyFilter fl
 	if len(dark) > 0 {
 		out.Darkened = true
 	}
-	if !math.IsInf(out.Dark, -1) {
-		v -= out.Dark
-	}
+	v -= out.Dark
 	out.Raw = v
 
 	n := int(math.Round(v))
````

**Modify `internal/rooms/lighting_model_test.go`:**

````diff
@@ -8,7 +8,7 @@ import (
 
 func modelCfg() configs.Lighting {
 	return configs.Lighting{
-		BlindBelow: 25, DimBelow: 50, ExitsAbove: 65,
+		BlindBelow: 25, DimBelow: 50, ExitsAbove: 65, RealMinimum: 3,
 		DoublingStep: 8, WorldLatitude: 46.5, EquinoxNoon: 70,
 		Starlight: 10, MoonsFull: 35,
 		MoonWeightSwiftmoon: 4, MoonWeightWanderer: 1, MoonWeightEye: 0.5,
````

**Modify `internal/rooms/weather_occlusion_test.go`:**

````diff
@@ -89,9 +89,13 @@ func TestStormNeverHidesFacesAtNoon(t *testing.T) {
 	}
 }
 
-// Heavy weather takes one full step off every night, so a night readable in
-// clear weather goes blind under a storm unless the moons are near their
-// brightest.
+// Heavy weather halves the night sky, so a night readable in clear weather
+// goes blind under a storm unless the moons are near their brightest.
+//
+// Since lighting plan 6 a halving is a whole doubling step only well above the
+// dim end: on the linear sum it takes less as the light nears 0, because there
+// is less light left to take (a clear 20 halves to about 14, a clear 10 to
+// about 6). So a night storm takes between 3 and 8 points, never more.
 func TestStormTakesAStepOffTheNight(t *testing.T) {
 	room := openSkyUnder(t)
 	blind := configs.GetLightingConfig().BlindBelow
@@ -103,8 +107,8 @@ func TestStormTakesAStepOffTheNight(t *testing.T) {
 			clear := room.LightLevel()
 			setWeather(t, room, "weather-storm")
 			stormy := room.LightLevel()
-			if drop := clear - stormy; clear > 8 && (drop < 7 || drop > 9) {
-				t.Errorf("day %d midnight: storm took %d points, want 8", d, drop)
+			if drop := clear - stormy; clear > 8 && (drop < 3 || drop > 8) {
+				t.Errorf("day %d midnight: storm took %d points from %d, want 3 to 8", d, drop, clear)
 			}
 			if clear >= blind && stormy < blind {
 				flipped++
````

**Modify `internal/usercommands/darkness_gates_sight_test.go`:**

````diff
@@ -6,8 +6,10 @@ import (
 
 	"github.com/GoMudEngine/GoMud/internal/conditions"
 	"github.com/GoMudEngine/GoMud/internal/events"
+	"github.com/GoMudEngine/GoMud/internal/itemlight"
 	"github.com/GoMudEngine/GoMud/internal/rooms"
 	"github.com/GoMudEngine/GoMud/internal/users"
+	"github.com/GoMudEngine/GoMud/internal/uuid"
 	"github.com/stretchr/testify/require"
 )
 
@@ -53,7 +55,16 @@ func seedDarknessGateRoom(t *testing.T, lamp int) (*users.UserRecord, *rooms.Roo
 	room := rooms.LoadRoom(2)
 	require.NotNil(t, room)
 	room.Biome = "cave"
-	room.Lamp = rooms.LampPtr(lamp)
+	if lamp >= 0 {
+		room.Lamp = rooms.LampPtr(lamp)
+	} else {
+		// Since lighting plan 6 a light term below 0 is not light: it reads
+		// 0, and only darkness takes a room below 0 (owner ruling O1). A
+		// negative pin is an unlit room under a darkness of that strength.
+		room.Lamp = nil
+		t.Cleanup(itemlight.ResetForTest())
+		itemlight.Set(room.RoomId, uuid.New(), itemlight.Darkness, float64(-lamp))
+	}
 
 	rooms.LoadRoom(1).RemovePlayer(user.UserId)
 	user.Character.RoomId = 2
````

- [ ] **Step 2: Packages, then the goldens move**

```bash
gofmt -l internal/
go build ./...
go vet ./...
go test ./internal/lightscale ./internal/rooms ./internal/configs ./internal/characters ./internal/lightnotice ./internal/usercommands ./internal/gametime ./internal/behaviortree -count=1
go test . -run "TestLightingBalanceSpread|TestLightingDayCycleAcrossSampleRounds|TestLightingParityAcrossEveryShippedRoom" -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: nothing from `gofmt`, all packages `ok`, then three `--- FAIL` lines: the three root lighting goldens moved, as predicted. Re-record them:

```bash
go test . -run TestLightingBalanceSpread -update-lighting-balance-spread -count=1
go test . -run TestLightingDayCycleAcrossSampleRounds -update-lighting-daycycle -count=1
go test . -run TestLightingParityAcrossEveryShippedRoom -update-lighting-parity -count=1
git diff --stat -- testdata
```

Expected: `testdata/lighting_balance_spread.golden` 676 lines changed each way, `testdata/lighting_daycycle.golden` 951, `testdata/lighting_parity.golden` 2. What the dry run read in those diffs:

- **Spread golden**: 2280 room entries, 676 raw readings moved, 112 levels moved. Every negative reading is gone (28 before, 0 after). Moves by band, before to after: below 0, 28 entries, largest 19.575; 0 to 10, 28, largest 4.786; 10 to 25, 14, largest 1.559; 25 to 37, 33, largest 0.661; 37 and above, 573, largest 0.401. Seven entries at or above 25 moved 0.5 or more, all room 4029 (forest) under a full moon, 25.784 to 26.445, level 26 unchanged.
- **Day-cycle golden**: 951 of 15660 entries moved. Every move from a level of 25 or more is a one-point rounding flip; below 25 rooms rise by up to 11; natural negatives go from 6 to 0.
- **Parity golden**: only the nightvision observer's line in cells 5105 and 5106 changed (both now see; none moved to shapes).

Infra reach, measured with `Character.InfraReach` before and after (owner ruling O10 accepts every move):

| Reach sources | Before | After |
|---|---|---|
| 85 innate | 30 | 30 |
| 133 Phantom Heat Sense | 50 | 50 |
| 129 Heat Sight: new caster, mid, endgame (capped) | 19.29, 33.57, 50 | unchanged |
| 130 Pitsense Tincture: fresh, peak | 26, 50 | unchanged |
| Heat Sight new + Pitsense fresh | 31.13 | 30.32 |
| Heat Sight mid + Pitsense fresh | 38.39 | 37.97 |
| 85 + Heat Sight new | 33.85 | 33.21 |
| 85 + Pitsense fresh | 36.17 | 35.66 |
| 85 + Heat Sight mid | 39.92 | 39.55 |
| Pitsense raw 20 + Heat Sight new | 27.65 | 26.55 (the only move past a point; accepted, O10) |
| two reaches of 5 | 13.00 | 8.48 |

- [ ] **Step 3: The full suite**

```bash
go test ./... -count=1 2>&1 | grep -E "^(--- FAIL|FAIL|panic)"
```

Expected: nothing (the dry run: 129 packages `ok`). `TestEveryShopkeeperCanTradeAtNightWhileAwake` stays green here and in Task 3: the biome lamps still burn at all hours until Task 4.

- [ ] **Step 4: Spec check and commit**

```bash
P="internal/characters/vision.go internal/characters/vision_test.go internal/conditions/effects.go internal/configs/config.balance.go internal/configs/config.balance.lighting.go internal/configs/config.lighting_accessor.go internal/configs/config.lighting_accessor_test.go internal/configs/config_lighting_real_minimum_test.go internal/lightnotice/tracker.go internal/lightscale/lightscale.go internal/lightscale/lightscale_test.go internal/lightscale/trim.go internal/lightscale/trim_test.go internal/rooms/biomes.go internal/rooms/carried_light_test.go internal/rooms/darkness_compose_test.go internal/rooms/darkness_trim_test.go internal/rooms/fixture_compose_test.go internal/rooms/light_floor_test.go internal/rooms/light_terms_test.go internal/rooms/light_trim.go internal/rooms/lighting.go internal/rooms/lighting_model_test.go internal/rooms/weather_occlusion_test.go internal/usercommands/darkness_gates_sight_test.go testdata/lighting_balance_spread.golden testdata/lighting_daycycle.golden testdata/lighting_parity.golden"
git add $P
git diff --cached 542437562 -- $P
git commit -m "feat(lighting): rebuild the light arithmetic on linear brightness, floor real light at LightRealMinimum (#372)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

The diff must print nothing; it covers the three goldens byte for byte.

---

### Task 3: world guard, no natural negative light

Checkpoint `93850add5`. Model: sonnet.

- [ ] **Step 1: The guard, and proof it can fail**

Create the file below. Against Task 1's code it does not build (`lighting_no_natural_negative_test.go:72:23: cfg.RealMinimum undefined (type configs.Lighting has no field or method RealMinimum)`). Against Task 2's code it is green: it pins what Task 2 built, so it is a guard rather than a failing-first test. Prove it can fail: in `internal/rooms/lighting.go` change `if floor := float64(cfg.RealMinimum); out.Light > 0 && out.Light < floor {` to `if floor := float64(cfg.RealMinimum); false && out.Light > 0 && out.Light < floor {`, run `go test . -run TestNoShippedRoomReadsBelowZeroWithoutDarkness -count=1`, and read `room 139 sample 0 filter 0.001: Raw 0.016 Level 0 (want 0, or at least 3)` and more. Restore the line.

**Create `lighting_no_natural_negative_test.go`:**

````go
package main

import (
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// TestNoShippedRoomReadsBelowZeroWithoutDarkness is lighting plan 6's world
// guard (owner rulings O1 and O2): 0 is the darkest a room can be without
// active magical darkness, so with no darkness source present no shipped room
// may read below 0 at any hour, season, moon state or weather.
//
// It loads the real world the way the night-trade guard
// (shop_night_trade_guard_test.go) and the lighting goldens do, and composes
// every room through rooms.LightTermsAtForTest with the celestial term built
// on the same operator gametime.CelestialLight uses. Nothing is prepared, so
// no mob, carried source or fixture (light or darkness) is in any room: the
// only terms are the sky, the room's own lamp and the weather.
//
// Samples: three days (both solstices and an equinox), every hour, the moons
// all new, all half and all full, and every distinct skylight fraction a
// shipped weather mutator declares, plus clear sky, the worst two stacked,
// and a near-total 0.001 as a probe past anything shipped.
//
// It also pins the O3 floor: a room reads either exactly 0 (no light at all)
// or at least LightRealMinimum.
func TestNoShippedRoomReadsBelowZeroWithoutDarkness(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()
	conditions.LoadDataFiles()
	mutators.LoadDataFiles()

	ids := rooms.GetAllRoomIds()
	if len(ids) < 1000 {
		t.Fatalf("loaded only %d rooms: the walk is not seeing the world, so a green run proves nothing", len(ids))
	}
	sort.Ints(ids)

	filterSet := map[float64]bool{1: true, 0.001: true}
	worst := 1.0
	for _, spec := range mutators.GetAllMutatorSpecs() {
		if spec.SkyLight != nil {
			filterSet[*spec.SkyLight] = true
			worst = min(worst, *spec.SkyLight)
		}
	}
	if len(filterSet) < 4 {
		t.Fatalf("found only %d sky filters: the mutator walk is not seeing the shipped weather", len(filterSet))
	}
	second := 1.0
	for f := range filterSet {
		if f > worst && f < 1 && f != 0.001 {
			second = min(second, f)
		}
	}
	filterSet[worst*second] = true

	cfg := configs.GetLightingConfig()
	floor := float64(cfg.RealMinimum)
	type sample struct {
		celestial float64
	}
	var samples []sample
	for _, doy := range []int{356, 81, 172} {
		for hour := 0; hour < 24; hour++ {
			for _, m := range []float64{0, 0.5, 1} {
				samples = append(samples, sample{
					celestial: lightscale.Combine(cfg.DoublingStep,
						gametime.SunLight(cfg, doy, float64(hour)),
						gametime.MoonLight(cfg, m, m, m)),
				})
			}
		}
	}

	checked, failures := 0, 0
	for _, id := range ids {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load", id)
		}
		for si, s := range samples {
			for f := range filterSet {
				terms := r.LightTermsAtForTest(s.celestial, f)
				checked++
				bad := terms.Raw < 0 || terms.Level < 0 || (terms.Raw > 0 && terms.Raw < floor)
				if bad {
					failures++
					if failures <= 20 {
						t.Errorf("room %d sample %d filter %v: Raw %.3f Level %d (want 0, or at least %v)",
							id, si, f, terms.Raw, terms.Level, floor)
					}
				}
			}
		}
	}
	if failures > 20 {
		t.Errorf("... and %d more", failures-20)
	}
	t.Logf("checked %d room readings", checked)
}
````

- [ ] **Step 2: Run and commit**

```bash
gofmt -l lighting_no_natural_negative_test.go
go vet .
go test . -run TestNoShippedRoomReadsBelowZeroWithoutDarkness -count=1 -v 2>&1 | grep -E "checked|^(--- |ok|FAIL)"
P="lighting_no_natural_negative_test.go"
git add $P
git diff --cached 93850add5 -- $P
git commit -m "test(lighting): world guard, no shipped room reads below 0 without darkness (#372)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected: nothing from `gofmt`; `checked 1409400 room readings`, `--- PASS`, `ok`; an empty diff.

---

### Task 4: street lamps lit at night and while the sky is dim

Checkpoint `adac03187`. Model: opus (a new clock rule threaded through the room composition, a behaviour condition, the light notices and two goldens).

- [ ] **Step 1: The tests first**

Apply the test blocks first: `internal/behaviortree/conditions_after_dusk_test.go`, `conditions_lamplit_test.go`, `shipped_item_trees_test.go`; `internal/gametime/lamps_lit_test.go`; `internal/lightnotice/integration_test.go`; `internal/rooms/fixture_compose_test.go`, `light_terms_test.go`, `lighting_model_test.go`, `room_light_override_test.go`, `sky_filter_test.go`, `street_lamp_test.go`; and the root `lighting_balance_spread_golden_test.go` and `lighting_no_natural_negative_test.go`. (`internal/rooms/test_helpers.go` is not a test file; it goes with the code.) Against Task 3's code:

- `go vet ./internal/gametime` fails: `lamps_lit_test.go:34:13: undefined: LampsLitAt`.
- `go vet ./internal/behaviortree` fails: `conditions_lamplit_test.go:44:15: undefined: gametime.LampsLit`. With that one file set aside, `go test ./internal/behaviortree -count=1` fails `TestShippedFixtureTrees`: `midwinter 08:00: the arch lantern reads -Inf, want 52 with the street lamps`.
- `go vet ./internal/rooms` fails: `fixture_compose_test.go:39:85: too many arguments in call to c.room.composeWithFixtures`.
- `go vet .` fails: `lighting_balance_spread_golden_test.go:152:24: undefined: gametime.LampsLitAt`.
- `go test ./internal/lightnotice -count=1` builds and fails `TestDuskTakesFacesFromTheBackstreetButNotTheThoroughfare`: `thoroughfare at midnight: faces kept from noon, want no line, got ["<ansi fg=\"light\">The day softens; your eyes stop aching.</ansi> "]` (today the lamp dazzles the street at noon, so midnight reads as the glare easing).

Then apply the code and data blocks.

**Modify `_datafiles/world/dogmud/behaviors/items/dusk_to_dawn.yaml`:**

````diff
@@ -1,8 +1,11 @@
-# dusk_to_dawn: a fixed lamp lit from dusk to dawn (lighting 5e, item
-# behaviour slice 1). Item 55, the North Gate's arch lantern (room 4111).
-# 52 is the Oil Lantern's rung. A fixture's light lives in
-# internal/itemlight and never trims.
-notes: Lit at the oil lantern's 52 from dusk to dawn, dark by day.
+# dusk_to_dawn: a fixed lamp lit with the street lamps (lighting 5e, item
+# behaviour slice 1; lighting plan 6). Item 55, the North Gate's arch lantern
+# (room 4111). `period: lamplit` is gametime.LampsLit, the same test the
+# city's biome street lamps read: lit at night and while the clear sky is too
+# dim to read a face, so a dim midwinter morning keeps it, and it changes in
+# the same round as every street lamp. 52 is the Oil Lantern's rung. A
+# fixture's light lives in internal/itemlight and never trims.
+notes: Lit at the oil lantern's 52 with the street lamps, at night and while the sky is too dim to read a face; dark otherwise.
 tree:
   type: selector
   event: item_idle
@@ -11,7 +14,7 @@ tree:
       children:
         - type: condition
           check: time_of_day
-          period: night
+          period: lamplit
         - type: action
           do: set_light
           level: 52
````

**Modify `_datafiles/world/dogmud/biomes/city_backstreet.yaml`:**

````diff
@@ -8,6 +8,9 @@ description: The side streets, lanes, alleys, courts and yards off a city's
   why those who hunt thieves do too.
 skylight: 0.95
 lamp: 35
+# Street lamps, lit and put out with the main streets' (lighting plan 6):
+# at night and while the clear sky is too dim to read a face.
+streetlamp: true
 requireditemid: 0
 usesitem: false
 burns: false
````

**Modify `_datafiles/world/dogmud/biomes/city_thoroughfare.yaml`:**

````diff
@@ -8,6 +8,11 @@ description: The main ways through a city, its squares, markets, gates and
   go unseen.
 skylight: 0.95
 lamp: 52
+# Street lamps (lighting plan 6): lit at night and while the clear sky is
+# too dim to read a face, like a lamplighter working by eye, so a dim
+# midwinter morning keeps them. Once the sky shows faces the street reads its
+# daylight alone. Weather never lights them.
+streetlamp: true
 requireditemid: 0
 usesitem: false
 burns: false
````

**Modify `_datafiles/world/dogmud/items/other-0/55-arch_lantern.yaml`:**

````diff
@@ -2,11 +2,13 @@ itemid: 55
 name: Arch Lantern
 namesimple: lantern
 # A fixture (lighting 5e): hangs from the North Gate's arch (room 4111),
-# placed there by spawninfo, and cannot be taken. Its tree lights it from
-# dusk to dawn at the oil lantern's rung.
+# placed there by spawninfo, and cannot be taken. Its tree lights it with
+# the street lamps (period: lamplit) at the oil lantern's rung: at night and
+# while the sky is too dim to read a face.
 description: A small iron lantern hung from the arch's crossbeam, the glass
-  smoke-blackened along its upper rim. It is lit at dusk and snuffed at
-  first light. Even unlit, the smell of old oil lingers faintly around it.
+  smoke-blackened along its upper rim. It is lit when the light fails and
+  snuffed once the day is bright. Even unlit, the smell of old oil lingers
+  faintly around it.
 type: object
 subtype: mundane
 weight: 0.1
````

**Modify `_datafiles/world/dogmud/narration/light-notices/lamp.yaml`:**

````diff
@@ -1,7 +1,10 @@
-# The room's own light: its lamp, and its fixtures (lighting 5e). A room's
-# lamp does not change at runtime, but a fixture does: the North Gate's arch
-# lantern lit at dusk and snuffed at dawn, a pulsing stone. These lines fire
-# when one moves the observer's band.
+# The room's own light: its lamp, and its fixtures (lighting 5e). A street
+# lamp is lit when the clear sky grows too dim to read a face and put out
+# once it is bright again, at night and on a dim winter morning or evening
+# alike (lighting plan 6, gametime.LampsLit), and the North Gate's arch
+# lantern with it; a pulsing stone rises and falls. These lines fire when one
+# moves the observer's band, so none may name the hour: each must read true
+# at a dim morning as well as at nightfall.
 cause: lamp
 transitions:
   darker_faces:
````

**Modify `internal/behaviortree/conditions_after_dusk_test.go`:**

````diff
@@ -21,10 +21,14 @@ func pinAfterDuskClock(t *testing.T) uint64 {
 	c.Balance.Validate()
 	configs.SetConfigForTest(t, c)
 	gametime.ClearDateCacheForTest()
+	// The celestial memo is keyed on the round alone, like the date cache;
+	// period: lamplit reads it (lighting plan 6).
+	gametime.ClearCelestialMemoForTest()
 	prev := util.GetRoundCount()
 	t.Cleanup(func() {
 		util.SetRoundCountForTest(prev)
 		gametime.ClearDateCacheForTest()
+		gametime.ClearCelestialMemoForTest()
 	})
 	for r := uint64(355*900 + 450); r < 356*900; r++ {
 		if gametime.GetDate(r).Night {
````

**Create `internal/behaviortree/conditions_lamplit_test.go`:**

````go
package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// period: lamplit is true while the street lamps burn (gametime.LampsLit):
// at night and while the clear sky is too dim to read a face. Midwinter
// 08:00 is day, so `period: night` fails there, but its sky is dim and the
// lamps (and so a lamplit lantern) still burn.
func TestCondTimeOfDay_LampLit(t *testing.T) {
	pinAfterDuskClock(t)
	lamplit := map[string]any{"period": "lamplit"}
	night := map[string]any{"period": "night"}

	const midwinter = 355 * 900 // the first round of day 356
	for _, c := range []struct {
		name      string
		round     uint64
		lit       Result
		nightWant Result
	}{
		{"midnight", midwinter, Success, Success},
		{"08:00, day but a dim sky", midwinter + 300, Success, Failure},
		{"noon", midwinter + 450, Failure, Failure},
	} {
		util.SetRoundCountForTest(c.round)
		if got := condTimeOfDay(lamplit, nil); got != c.lit {
			t.Errorf("%s: lamplit got %v, want %v", c.name, got, c.lit)
		}
		if got := condTimeOfDay(night, nil); got != c.nightWant {
			t.Errorf("%s: night got %v, want %v; the probe is off", c.name, got, c.nightWant)
		}
	}

	// Every round of the day it is exactly gametime.LampsLit, the test the
	// biome street lamps read, so a lamplit fixture changes in the same round.
	for r := uint64(midwinter); r < midwinter+900; r++ {
		util.SetRoundCountForTest(r)
		want := Failure
		if gametime.LampsLit() {
			want = Success
		}
		if got := condTimeOfDay(lamplit, nil); got != want {
			t.Fatalf("round %d: lamplit %v, gametime.LampsLit says %v", r, got, want)
		}
	}
}

// The period name is case-insensitive, as day and night are.
func TestCondTimeOfDay_LampLitIgnoresCase(t *testing.T) {
	pinAfterDuskClock(t)
	util.SetRoundCountForTest(355 * 900)
	if got := condTimeOfDay(map[string]any{"period": "LampLit"}, nil); got != Success {
		t.Errorf("period LampLit at midnight: got %v, want Success", got)
	}
}
````

**Modify `internal/behaviortree/conditions_state.go`:**

````diff
@@ -135,6 +135,18 @@ func condTimeOfDay(params map[string]any, ctx *EvalContext) Result {
 		return Failure
 	}
 
+	// period: lamplit (lighting plan 6, owner ruling O4 as amended): true
+	// while the street lamps burn, at night or while the clear sky is too
+	// dim to read a face. It is the same gametime.LampsLit the biome street
+	// lamps read, so a lantern fixture on this tree lights and goes out in
+	// the same round as every street lamp.
+	if strings.ToLower(period) == "lamplit" {
+		if gametime.LampsLit() {
+			return Success
+		}
+		return Failure
+	}
+
 	isNight := gametime.IsNight()
 	switch strings.ToLower(period) {
 	case "night":
````

**Modify `internal/behaviortree/shipped_item_trees_test.go`:**

````diff
@@ -122,8 +122,9 @@ func TestShippedSunstoneFollowsTheSun(t *testing.T) {
 	}
 }
 
-// The arch lantern (55) is lit at 52 from dusk to dawn; the Rift Stone (56)
-// pulses within 20 to 36.
+// The arch lantern (55) is lit at 52 while the street lamps burn (period:
+// lamplit: at night, and while the clear sky is too dim to read a face, so a
+// midwinter 08:00 keeps it lit); the Rift Stone (56) pulses within 20 to 36.
 func TestShippedFixtureTrees(t *testing.T) {
 	dusk := loadShippedItemWorld(t)
 	idle := EventContext{EventType: "item_idle"}
@@ -140,6 +141,11 @@ func TestShippedFixtureTrees(t *testing.T) {
 	if v, _ := itemlight.Get(engineProbeRoomId, arch.UUID); v != 52 {
 		t.Errorf("dusk: the arch lantern reads %v, want 52", v)
 	}
+	util.SetRoundCountForTest(355*900 + 300) // 08:00, day but a dim sky
+	TryItemBehavior(idle, arch)
+	if v, _ := itemlight.Get(engineProbeRoomId, arch.UUID); v != 52 {
+		t.Errorf("midwinter 08:00: the arch lantern reads %v, want 52 with the street lamps", v)
+	}
 	for r := uint64(0); r < 24; r++ {
 		util.SetRoundCountForTest(dusk + r)
 		TryItemBehavior(idle, stone)
````

**Modify `internal/gametime/celestial.go`:**

````diff
@@ -81,6 +81,39 @@ func NightHoursAt(latitudeDegrees float64, dayOfYear int) float64 {
 	return 24 - 2*halfDayHours(latitudeDegrees, dayOfYear)
 }
 
+// NightAt reports whether an hour of a day of the year is night at a latitude:
+// the night is centred on midnight and NightHoursAt long. It is the one
+// boundary GameDate.Night (and so IsNight) uses, exposed so a caller holding a
+// day and an hour (a light golden, the street lamp's tests) can ask
+// without moving the round counter.
+func NightAt(latitudeDegrees float64, dayOfYear int, hour float64) bool {
+	halfNight := NightHoursAt(latitudeDegrees, dayOfYear) / 2
+	return hour >= 24-halfNight || hour < halfNight
+}
+
+// LampsLitAt reports whether the world's street lamps burn, given whether it
+// is night, the clear-sky celestial light (CelestialLight: sun and moons
+// before any sky fraction or weather) and the faces edge LightDimBelow. A
+// lamp burns while it is night OR while that clear sky reads below the faces
+// edge, the way a lamplighter works by eye (lighting plan 6, owner ruling
+// O4 as amended): a midwinter morning still too dim to read a face keeps its
+// lamps. The input is the clear sky, never the weathered one, so a storm
+// lights no lamp and every lamp in the world changes at the same moment.
+//
+// An Absent celestial (-Inf) is below any edge; a NaN one, which only an
+// arithmetic mistake produces, leaves the lamps to the night alone.
+func LampsLitAt(night bool, celestial float64, dimBelow int) bool {
+	return night || celestial < float64(dimBelow)
+}
+
+// LampsLit is LampsLitAt on the current round: IsNight, CelestialLight and
+// the shipped LightDimBelow. The street lamps (rooms.BiomeInfo.StreetLamp)
+// and the `time_of_day period: lamplit` behaviour condition both read it, so
+// a biome lamp and a lantern fixture light and go out together.
+func LampsLit() bool {
+	return LampsLitAt(IsNight(), CelestialLight(), configs.GetLightingConfig().DimBelow)
+}
+
 // solarSinAltitude is the sine of the sun's altitude above the horizon, which
 // is also the share of its light falling on level ground. Negative means below
 // the horizon.
````

**Modify `internal/gametime/gametime.go`:**

````diff
@@ -285,12 +285,13 @@ func (g *GameDate) ReCalculate() {
 	// nearest hour and lose up to half an hour of night at each end.
 	hourOfDay := float64(roundOfDay) / float64(g.RoundsPerDay) * 24
 
-	nightHours := NightHoursAt(configs.GetLightingConfig().WorldLatitude, int(day))
+	latitude := configs.GetLightingConfig().WorldLatitude
+	nightHours := NightHoursAt(latitude, int(day))
 	halfNight := nightHours / 2
 	nightStartHour := 24 - halfNight
 	nightEndHour := halfNight
 
-	night := hourOfDay >= nightStartHour || hourOfDay < nightEndHour
+	night := NightAt(latitude, int(day), hourOfDay)
 
 	ampm := `AM`
 	if hour >= 12 {
````

**Create `internal/gametime/lamps_lit_test.go`:**

````go
package gametime

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The lamplighter's rule (lighting plan 6, owner ruling O4 as amended): the
// street lamps burn at night, and by day while the clear sky reads below the
// faces edge.
func TestLampsLitAt(t *testing.T) {
	for _, c := range []struct {
		name      string
		night     bool
		celestial float64
		dim       int
		want      bool
	}{
		{"night under a bright sky", true, 70, 50, true},
		{"night with no sky at all", true, lightscale.Absent(), 50, true},
		{"a day sky below the faces edge", false, 40, 50, true},
		{"a day sky just below the edge", false, 49.999, 50, true},
		{"a day sky exactly at the edge", false, 50, 50, false},
		{"a day sky above the edge", false, 70, 50, false},
		{"an Absent sky by day", false, lightscale.Absent(), 50, true},
		{"a NaN sky leaves it to the night: day", false, math.NaN(), 50, false},
		{"a NaN sky leaves it to the night: night", true, math.NaN(), 50, true},
		{"the edge is the configured one", false, 55, 60, true},
	} {
		if got := LampsLitAt(c.night, c.celestial, c.dim); got != c.want {
			t.Errorf("%s: LampsLitAt(%v, %v, %d) = %v, want %v", c.name, c.night, c.celestial, c.dim, got, c.want)
		}
	}
}

// LampsLit reads the live clock: midwinter 08:00 is day (IsNight false) but
// its clear sky is below the faces edge, so the lamps still burn; noon puts
// them out; midnight lights them.
func TestLampsLitOnTheClock(t *testing.T) {
	pinTiming(t, 46.5)
	ClearCelestialMemoForTest()
	t.Cleanup(ClearCelestialMemoForTest)
	original := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCount(original) })
	dim := configs.GetLightingConfig().DimBelow

	at := func(doy int, hour float64) (night bool, celestial float64, lit bool) {
		util.SetRoundCount(roundFor(doy, hour))
		ClearDateCacheForTest()
		ClearCelestialMemoForTest()
		return IsNight(), CelestialLight(), LampsLit()
	}

	if night, cel, lit := at(356, 8); night || !(cel < float64(dim)) || !lit {
		t.Errorf("midwinter 08:00: night=%v celestial=%.2f lit=%v, want day, a sky below %d, lamps lit",
			night, cel, lit, dim)
	}
	for _, doy := range []int{356, 81, 172} {
		if _, _, lit := at(doy, 12); lit {
			t.Errorf("day %d noon: lamps lit, want out", doy)
		}
		if _, _, lit := at(doy, 0); !lit {
			t.Errorf("day %d midnight: lamps out, want lit", doy)
		}
	}

	// Every round of midwinter day: LampsLit is exactly the pure rule on
	// the clock's own reads, and there is dim daylight where it differs from
	// IsNight alone (else this test proves nothing about the new half).
	dimDay := 0
	for r := roundFor(356, 0); r < roundFor(357, 0); r++ {
		util.SetRoundCount(r)
		night, cel := IsNight(), CelestialLight()
		if got, want := LampsLit(), LampsLitAt(night, cel, dim); got != want {
			t.Fatalf("round %d: LampsLit %v, LampsLitAt on the same reads %v", r, got, want)
		}
		if !night && LampsLit() {
			dimDay++
		}
	}
	if dimDay == 0 {
		t.Errorf("midwinter has no dim daylight round with the lamps lit; the sky-dim half is untested")
	}
}
````

**Modify `internal/lightnotice/integration_test.go`:**

````diff
@@ -76,16 +76,20 @@ func logLightAt(t *testing.T, label string, roomId int) {
 }
 
 // A backstreet loses faces between noon and midnight on midsummer day; a
-// thoroughfare never drops below faces, but it does not stay comfortable
-// either: measured against the real biome files, a normal observer on a main
-// street at midsummer noon reads BandDazzled (sky 73 combined with the
-// thoroughfare's lamp 52 clears the 75 dazzle edge). The owner ruled this is
-// intended, not a bug: a normal observer CAN be dazzled by a torch or a main
-// street at midsummer noon, which is what makes a hooded lantern or a spell
-// worth having. So the thoroughfare's dusk is not silent: the glare easing as
-// dazzle gives way to faces at midnight is exactly the DarkerFaces notice, and
-// faces are kept the rest of the night (never DarkerShapes or DarkerDark).
-func TestDuskTakesFacesFromTheBackstreetButOnlyGlareFromTheThoroughfare(t *testing.T) {
+// thoroughfare keeps faces from noon to midnight and says nothing.
+//
+// Before lighting plan 6 the street lamps burned at all hours, so a main
+// street at midsummer noon read sky 73 combined with lamp 52, past the 75
+// dazzle edge, and its dusk was the glare easing (DarkerFaces). Since plan 6
+// (owner ruling O4 as amended) the lamps burn only while it is night or the
+// clear sky is too dim to read a face: at noon the street
+// reads its daylight alone, 73, faces and not dazzled, and at midnight its
+// lamp, 54, still faces. No band moves, so no line.
+//
+// The backstreet's lamp lights between the two checks while its band drops:
+// a lamp that brightened cannot have darkened the room, so the line blames
+// the sky (lampAgrees), not "the lamplight fades".
+func TestDuskTakesFacesFromTheBackstreetButNotTheThoroughfare(t *testing.T) {
 	withShippedWorld(t)
 	seedCity(t)
 	lane := users.NewTestUser(1, "alice", "Aliceia", 1001)
@@ -114,13 +118,8 @@ func TestDuskTakesFacesFromTheBackstreetButOnlyGlareFromTheThoroughfare(t *testi
 		t.Fatalf("backstreet at midnight: want one sky darker_shapes line, got %q", got)
 	}
 
-	gotStreet := drainStreet()
-	if len(gotStreet) != 1 || !containsAny(gotStreet[0], Pool(CauseSky, DarkerFaces, false)) {
-		t.Fatalf("thoroughfare at midnight: want one sky darker_faces (glare easing) line, got %q", gotStreet)
-	}
-	if containsAny(gotStreet[0], Pool(CauseSky, DarkerShapes, false)) ||
-		containsAny(gotStreet[0], Pool(CauseSky, DarkerDark, false)) {
-		t.Fatalf("thoroughfare at midnight: faces are kept all night, got a darker-than-faces line %q", gotStreet)
+	if gotStreet := drainStreet(); len(gotStreet) != 0 {
+		t.Fatalf("thoroughfare at midnight: faces kept from noon, want no line, got %q", gotStreet)
 	}
 
 	Check(lane, TriggerCommand)
````

**Modify `internal/lightnotice/tracker.go`:**

````diff
@@ -163,8 +163,15 @@ func attribute(prev record, now observation) Cause {
 	// (lighting 5e, X5).
 	case a.Carried != b.Carried || termMoved(a.CarriedLight, b.CarriedLight):
 		return CauseCarried
-	// The room's own light: its lamp, and its fixtures (lighting 5e).
-	case a.HasLamp != b.HasLamp || a.Lamp != b.Lamp || termMoved(a.Fixture, b.Fixture):
+	// The room's own light: its lamp, and its fixtures (lighting 5e). Since
+	// street lamps light when the clear sky grows too dim to read a face and
+	// go out once it is bright again (lighting plan 6, gametime.LampsLit), the
+	// lamp can move the opposite way to the band across a long gap (noon to
+	// midnight: the sky went out, the lamp came on). A lamp that brightened
+	// cannot have darkened the room, nor one that dimmed lightened it, so
+	// such a move falls through to the sky.
+	case (a.HasLamp != b.HasLamp || a.Lamp != b.Lamp || termMoved(a.Fixture, b.Fixture)) &&
+		lampAgrees(a, b):
 		return CauseLamp
 	// Exact comparison is safe while the shipped fractions are 0.5 and 0.7:
 	// their products are identical in any order. Two different fractions
@@ -178,6 +185,37 @@ func attribute(prev record, now observation) Cause {
 	return CauseEyes
 }
 
+// lampAgrees reports whether the room's own light (its lamp and its light
+// fixtures) moved the same way as the room: a lamp that only brightened
+// cannot explain a darker room, and one that only dimmed cannot explain a
+// lighter one. A move both ways (one fixture up, the lamp out) agrees, so the
+// lamp keeps the blame it had before lighting plan 6.
+func lampAgrees(a, b rooms.LightTerms) bool {
+	lampOf := func(t rooms.LightTerms) float64 {
+		if !t.HasLamp {
+			return 0
+		}
+		return float64(t.Lamp)
+	}
+	la, lb := lampOf(a), lampOf(b)
+	fa, fb := a.Fixture, b.Fixture
+	if math.IsInf(fa, -1) {
+		fa = 0
+	}
+	if math.IsInf(fb, -1) {
+		fb = 0
+	}
+	up := lb > la || fb > fa
+	down := lb < la || fb < fa
+	switch {
+	case b.Level < a.Level:
+		return down || !up
+	case b.Level > a.Level:
+		return up || !down
+	}
+	return true
+}
+
 // termMoved reports whether a light-scale term changed: appeared, went out
 // (0 since lighting plan 6, when a combine of nothing reads 0; Absent is still
 // accepted from a caller that passes one),
````

**Modify `internal/rooms/biomes.go`:**

````diff
@@ -50,6 +50,22 @@ type BiomeInfo struct {
 	// inn should do.
 	Lamp *int `yaml:"lamp,omitempty"`
 
+	// StreetLamp makes the biome's Lamp a street lamp, lit and put out by a
+	// lamplighter working by eye: it joins the room's light while
+	// gametime.LampsLit() is true, which is at night OR while the clear sky
+	// (the celestial light before any sky fraction or weather) reads below
+	// LightDimBelow, so a midwinter morning too dim to read a face keeps its
+	// lamps (lighting plan 6, owner ruling O4 as amended). The North Gate
+	// arch lantern's dusk_to_dawn tree reads the same test (`time_of_day
+	// period: lamplit`), and weather never enters it, so every street lamp
+	// in the world lights and goes out at the same moment. Unset, the lamp
+	// burns at all hours, which is right for an inn's lamps, a cave's glow
+	// or the ether.
+	//
+	// It governs the BIOME lamp only. A room's own `lamp:` override is an
+	// all-hours lamp whatever its biome says.
+	StreetLamp bool `yaml:"streetlamp,omitempty"`
+
 	// Indoor marks a room as sheltered from weather; outdoor-only mutators
 	// don't render here.
 	//
@@ -94,7 +110,8 @@ func (bi *BiomeInfo) SkyLightFraction() float64 {
 	return *bi.SkyLight
 }
 
-// LampValue is the biome's own light source and whether it declares one at all.
+// LampValue is the biome's own light source and whether it declares one at
+// all, whatever the hour. LampAt is the lamp as it burns at a given hour.
 func (bi *BiomeInfo) LampValue() (int, bool) {
 	if bi.Lamp == nil {
 		return 0, false
@@ -102,6 +119,16 @@ func (bi *BiomeInfo) LampValue() (int, bool) {
 	return *bi.Lamp, true
 }
 
+// LampAt is the biome's lamp as it burns when the street lamps are (or are
+// not) lit, the value gametime.LampsLit reports: a StreetLamp lamp is out
+// while they are out, every other lamp burns at all hours.
+func (bi *BiomeInfo) LampAt(lampsLit bool) (int, bool) {
+	if bi.StreetLamp && !lampsLit {
+		return 0, false
+	}
+	return bi.LampValue()
+}
+
 // HasLamp reports whether this biome carries a light source that actually
 // produces light.
 //
````

**Modify `internal/rooms/fixture_compose_test.go`:**

````diff
@@ -36,7 +36,7 @@ func TestFixtureComposition(t *testing.T) {
 		{"a darkness fixture and a carried darkness", cave, nil, []float64{50}, nil, []float64{50}, -58, 0, 0},
 	}
 	for _, c := range cases {
-		got := c.room.composeWithFixtures(cfg, 60, 1, c.carried, c.dark, c.fxLight, c.fxDark)
+		got := c.room.composeWithFixtures(cfg, 60, true, 1, c.carried, c.dark, c.fxLight, c.fxDark)
 		if got.Level != c.want {
 			t.Errorf("%s: Level = %d, want %d", c.name, got.Level, c.want)
 		}
@@ -59,7 +59,7 @@ func TestComposeWithIsFixtureFree(t *testing.T) {
 	zero := 0.0
 	cave := Room{SkyLight: &zero, Lamp: LampPtr(50)}
 	a := cave.composeWith(cfg, 60, 1, []float64{56}, []float64{20})
-	b := cave.composeWithFixtures(cfg, 60, 1, []float64{56}, []float64{20}, nil, nil)
+	b := cave.composeWithFixtures(cfg, 60, true, 1, []float64{56}, []float64{20}, nil, nil)
 	if a != b {
 		t.Errorf("composeWith %+v != composeWithFixtures with none %+v", a, b)
 	}
````

**Modify `internal/rooms/light_terms_test.go`:**

````diff
@@ -30,7 +30,7 @@ func TestLightTermsReportEachTerm(t *testing.T) {
 	lamp := 40
 
 	lit := Room{SkyLight: &open, Lamp: &lamp}
-	got := lit.composeLight(cfg, 60, 0.25)
+	got := lit.composeLight(cfg, 60, true, 0.25)
 	if !got.HasLamp || got.Lamp != 40 {
 		t.Errorf("lamp = (%v, %d), want (true, 40)", got.HasLamp, got.Lamp)
 	}
@@ -46,7 +46,7 @@ func TestLightTermsReportEachTerm(t *testing.T) {
 	}
 
 	cave := Room{SkyLight: &zero}
-	if c := cave.composeLight(cfg, 60, 1); c.Sky != 0 || c.HasLamp {
+	if c := cave.composeLight(cfg, 60, true, 1); c.Sky != 0 || c.HasLamp {
 		t.Errorf("a cave has no sky light (0) and no lamp, got Sky=%v HasLamp=%v", c.Sky, c.HasLamp)
 	}
 }
````

**Modify `internal/rooms/light_trim.go`:**

````diff
@@ -56,6 +56,7 @@ func (r *Room) TrimLightFor(c *characters.Character) {
 
 	cfg := configs.GetLightingConfig()
 	celestial := gametime.CelestialLight()
+	lampsLit := gametime.LampsLitAt(gametime.IsNight(), celestial, cfg.DimBelow)
 	skyFilter := r.mutatorSkyFilter()
 	strength := c.NightVisionStrength()
 	lightTarget := messaging.LightTrimTarget(strength, cfg.DazzleAbove)
@@ -70,7 +71,7 @@ func (r *Room) TrimLightFor(c *characters.Character) {
 		t.rec.SetLightOutput(lightscale.Absent())
 	}
 	for _, t := range todo {
-		terms := r.composeLightExcluding(cfg, celestial, skyFilter, t.rec)
+		terms := r.composeLightExcluding(cfg, celestial, lampsLit, skyFilter, t.rec)
 		full := t.rec.LightMax(t.spec)
 		var out float64
 		if t.dark {
````

**Modify `internal/rooms/lighting.go`:**

````diff
@@ -33,7 +33,7 @@ import (
 // Weather attenuates the SKY only, through each active mutator's skylight
 // fraction: a blizzard does not dim a lantern.
 func (r *Room) LightLevel() int {
-	return r.lightLevel(configs.GetLightingConfig(), gametime.CelestialLight())
+	return r.lightLevel(configs.GetLightingConfig(), gametime.CelestialLight(), gametime.LampsLit())
 }
 
 // IsLit reports whether a normal observer can see anything at all here.
@@ -44,19 +44,21 @@ func (r *Room) LightLevel() int {
 // reads the narrow lighting config once.
 func (r *Room) IsLit() bool {
 	cfg := configs.GetLightingConfig()
-	return r.lightLevel(cfg, gametime.CelestialLight()) >= cfg.BlindBelow
+	return r.lightLevel(cfg, gametime.CelestialLight(), gametime.LampsLit()) >= cfg.BlindBelow
 }
 
-// lightLevel is LightLevel with its two reads injected, so it is testable
-// without global state and so a caller holding both can avoid reading twice.
-func (r *Room) lightLevel(cfg configs.Lighting, celestial float64) int {
-	return r.lightLevelWithSkyFilter(cfg, celestial, r.mutatorSkyFilter())
+// lightLevel is LightLevel with its clock reads (the celestial light and
+// whether the street lamps are lit, gametime.LampsLit) injected, so it is
+// testable without global state and so a caller holding both can avoid
+// reading twice.
+func (r *Room) lightLevel(cfg configs.Lighting, celestial float64, lampsLit bool) int {
+	return r.lightLevelWithSkyFilter(cfg, celestial, lampsLit, r.mutatorSkyFilter())
 }
 
 // lightLevelWithSkyFilter is the composition itself, with the active mutators'
 // sky filter already multiplied out, so tests can drive it directly.
-func (r *Room) lightLevelWithSkyFilter(cfg configs.Lighting, celestial, skyFilter float64) int {
-	return r.composeLight(cfg, celestial, skyFilter).Level
+func (r *Room) lightLevelWithSkyFilter(cfg configs.Lighting, celestial float64, lampsLit bool, skyFilter float64) int {
+	return r.composeLight(cfg, celestial, lampsLit, skyFilter).Level
 }
 
 // LightTerms is the room's light broken into the terms LightLevel combines,
@@ -104,27 +106,30 @@ type LightTerms struct {
 // LightTerms reports the terms behind LightLevel, from the same single
 // computation.
 func (r *Room) LightTerms() LightTerms {
-	return r.composeLight(configs.GetLightingConfig(), gametime.CelestialLight(), r.mutatorSkyFilter())
+	return r.composeLight(configs.GetLightingConfig(), gametime.CelestialLight(), gametime.LampsLit(), r.mutatorSkyFilter())
 }
 
 // composeLight is the one computation behind LightLevel and LightTerms.
-func (r *Room) composeLight(cfg configs.Lighting, celestial, skyFilter float64) LightTerms {
-	return r.composeLightExcluding(cfg, celestial, skyFilter, nil)
+func (r *Room) composeLight(cfg configs.Lighting, celestial float64, lampsLit bool, skyFilter float64) LightTerms {
+	return r.composeLightExcluding(cfg, celestial, lampsLit, skyFilter, nil)
 }
 
 // composeLightExcluding is composeLight with one carried record left out of
 // whichever combine it belongs to, which is the room a trimming source sees:
 // everything except itself.
-func (r *Room) composeLightExcluding(cfg configs.Lighting, celestial, skyFilter float64, exclude *conditions.Condition) LightTerms {
+func (r *Room) composeLightExcluding(cfg configs.Lighting, celestial float64, lampsLit bool, skyFilter float64, exclude *conditions.Condition) LightTerms {
 	carried, dark := r.carriedTerms(exclude)
 	fxLight, fxDark := itemlight.Terms(r.RoomId)
-	return r.composeWithFixtures(cfg, celestial, skyFilter, carried, dark, fxLight, fxDark)
+	return r.composeWithFixtures(cfg, celestial, lampsLit, skyFilter, carried, dark, fxLight, fxDark)
 }
 
 // composeWith is the composition with the carried light and darkness terms
-// supplied and no fixtures, so a test needs no users, mobs or items.
+// supplied and no fixtures, so a test needs no users, mobs or items. It
+// composes with every street lamp lit, which is what every test written
+// before the street lamp's hours (lighting plan 6) assumed; a test of the
+// lamp's hours calls composeWithFixtures with lampsLit set.
 func (r *Room) composeWith(cfg configs.Lighting, celestial, skyFilter float64, carried, dark []float64) LightTerms {
-	return r.composeWithFixtures(cfg, celestial, skyFilter, carried, dark, nil, nil)
+	return r.composeWithFixtures(cfg, celestial, true, skyFilter, carried, dark, nil, nil)
 }
 
 // composeWithFixtures is the composition itself, with every term supplied.
@@ -135,7 +140,7 @@ func (r *Room) composeWith(cfg configs.Lighting, celestial, skyFilter float64, c
 // clamped to [-100, 100] (lighting plan 5d, owner decision 1; plan 6). A lit light fixture is one term in
 // the light combine and a darkness fixture one in the darkness combine,
 // beside what people carry (lighting 5e, Rule 10). Fixtures never trim.
-func (r *Room) composeWithFixtures(cfg configs.Lighting, celestial, skyFilter float64, carried, dark, fixtureLight, fixtureDark []float64) LightTerms {
+func (r *Room) composeWithFixtures(cfg configs.Lighting, celestial float64, lampsLit bool, skyFilter float64, carried, dark, fixtureLight, fixtureDark []float64) LightTerms {
 	step := cfg.DoublingStep
 	if !(step > 0) {
 		step = 1
@@ -154,7 +159,7 @@ func (r *Room) composeWithFixtures(cfg configs.Lighting, celestial, skyFilter fl
 	terms = append(terms, out.Sky)
 
 	// 2. The room's own lamp.
-	if lamp, ok := r.lampValue(); ok {
+	if lamp, ok := r.lampValue(lampsLit); ok {
 		out.Lamp, out.HasLamp = lamp, true
 		terms = append(terms, float64(lamp))
 	}
@@ -279,14 +284,17 @@ func (r *Room) skyLightFraction() float64 {
 	return 1.0
 }
 
-// lampValue is this room's own light source, and whether it has one at all.
-// Nil-safe for the same reason as skyLightFraction.
-func (r *Room) lampValue() (int, bool) {
+// lampValue is this room's own light source as it burns when the street lamps
+// are (or are not) lit, and whether it burns at all. A room's own `lamp:`
+// override burns at all hours; a biome lamp marked streetlamp only while
+// gametime.LampsLit (lighting plan 6, owner ruling O4 as amended). Nil-safe
+// for the same reason as skyLightFraction.
+func (r *Room) lampValue(lampsLit bool) (int, bool) {
 	if r.Lamp != nil {
 		return *r.Lamp, true
 	}
 	if b := r.GetBiome(); b != nil {
-		return b.LampValue()
+		return b.LampAt(lampsLit)
 	}
 	return 0, false
 }
````

**Modify `internal/rooms/lighting_model_test.go`:**

````diff
@@ -21,7 +21,7 @@ func modelCfg() configs.Lighting {
 func TestUnlitCaveIsZeroNotNegative(t *testing.T) {
 	zero := 0.0
 	r := Room{SkyLight: &zero}
-	if got := r.lightLevel(modelCfg(), 70); got != 0 {
+	if got := r.lightLevel(modelCfg(), 70, true); got != 0 {
 		t.Errorf("unlit cave = %d, want 0", got)
 	}
 }
@@ -30,7 +30,7 @@ func TestUnlitCaveIsZeroNotNegative(t *testing.T) {
 func TestSkyFractionCostsOneStepPerHalving(t *testing.T) {
 	half := 0.5
 	r := Room{SkyLight: &half}
-	if got := r.lightLevel(modelCfg(), 60); got != 52 {
+	if got := r.lightLevel(modelCfg(), 60, true); got != 52 {
 		t.Errorf("half sky under a 60 sky = %d, want 52", got)
 	}
 }
@@ -41,7 +41,7 @@ func TestLampJoinsTheCombineRatherThanFlooring(t *testing.T) {
 	open := 1.0
 	lamp := 36
 	r := Room{SkyLight: &open, Lamp: &lamp}
-	if got := r.lightLevel(modelCfg(), 36); got != 44 {
+	if got := r.lightLevel(modelCfg(), 36, true); got != 44 {
 		t.Errorf("lamp 36 under a 36 sky = %d, want 44", got)
 	}
 }
@@ -52,7 +52,7 @@ func TestLanternIsNearlyIrrelevantAtNoon(t *testing.T) {
 	open := 1.0
 	lamp := 55
 	r := Room{SkyLight: &open, Lamp: &lamp}
-	got := r.lightLevel(modelCfg(), 70)
+	got := r.lightLevel(modelCfg(), 70, true)
 	if got < 70 || got > 74 {
 		t.Errorf("lantern 55 at noon 70 = %d, want 70 to 74", got)
 	}
@@ -62,7 +62,7 @@ func TestLightIsClampedToTheScale(t *testing.T) {
 	open := 1.0
 	lamp := 100
 	r := Room{SkyLight: &open, Lamp: &lamp}
-	if got := r.lightLevel(modelCfg(), 100); got > 100 {
+	if got := r.lightLevel(modelCfg(), 100, true); got > 100 {
 		t.Errorf("light = %d, above the scale ceiling", got)
 	}
 }
````

**Modify `internal/rooms/room_light_override_test.go`:**

````diff
@@ -30,11 +30,11 @@ func TestRoomOverridesBiomeSkyAndLamp(t *testing.T) {
 		t.Errorf("overridden skylight = %v, want 1.0", got)
 	}
 
-	if got, ok := r.lampValue(); ok {
+	if got, ok := r.lampValue(true); ok {
 		t.Errorf("cave room lamp = (%v,%v), want none", got, ok)
 	}
 	r.Lamp = &lamp
-	got, ok := r.lampValue()
+	got, ok := r.lampValue(true)
 	if !ok || got != 55 {
 		t.Errorf("overridden lamp = (%v,%v), want (55,true)", got, ok)
 	}
````

**Modify `internal/rooms/sky_filter_test.go`:**

````diff
@@ -16,18 +16,18 @@ func TestSkyFilterSubtractsFromTheSkyOnly(t *testing.T) {
 	lamp := 40
 
 	sky := Room{SkyLight: &open}
-	clear := sky.lightLevelWithSkyFilter(cfg, 60, 1)
+	clear := sky.lightLevelWithSkyFilter(cfg, 60, true, 1)
 	for _, c := range []struct {
 		filter float64
 		drop   int
 	}{{0.7, 4}, {0.5, 8}, {0.35, 12}} {
-		if got := clear - sky.lightLevelWithSkyFilter(cfg, 60, c.filter); got != c.drop {
+		if got := clear - sky.lightLevelWithSkyFilter(cfg, 60, true, c.filter); got != c.drop {
 			t.Errorf("filter %v dropped the sky by %d, want %d", c.filter, got, c.drop)
 		}
 	}
 
 	lampOnly := Room{SkyLight: &zero, Lamp: &lamp}
-	if a, b := lampOnly.lightLevelWithSkyFilter(cfg, 60, 1), lampOnly.lightLevelWithSkyFilter(cfg, 60, 0.35); a != b {
+	if a, b := lampOnly.lightLevelWithSkyFilter(cfg, 60, true, 1), lampOnly.lightLevelWithSkyFilter(cfg, 60, true, 0.35); a != b {
 		t.Errorf("a filter moved a lamp: %d clear, %d filtered", a, b)
 	}
 }
````

**Create `internal/rooms/street_lamp_test.go`:**

````go
package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The street lamp (lighting plan 6, owner ruling O4 as amended): a biome
// marked streetlamp lights only while the street lamps are lit
// (gametime.LampsLit); every other lamp, and every room's own lamp override,
// burns at all hours.
func TestStreetLampJoinsOnlyWhileLampsAreLit(t *testing.T) {
	cleanup := SeedBiomesForTest(map[string]*BiomeInfo{
		"default":           {BiomeId: "default", Name: "Default"},
		"city_thoroughfare": {BiomeId: "city_thoroughfare", Name: "Street", SkyLight: SkyLightPtr(0.95), Lamp: LampPtr(52), StreetLamp: true},
		"interior":          {BiomeId: "interior", Name: "Interior", SkyLight: SkyLightPtr(0.15), Lamp: LampPtr(50)},
	})
	t.Cleanup(cleanup)
	cfg := modelCfg()

	street := Room{Biome: "city_thoroughfare"}
	inn := Room{Biome: "interior"}
	lampPost := Room{Biome: "city_thoroughfare", Lamp: LampPtr(40)}

	for _, c := range []struct {
		name     string
		room     Room
		lit      bool
		wantLamp bool
		lamp     int
	}{
		{"a street with the lamps out reads its daylight alone", street, false, false, 0},
		{"a street with the lamps lit has its lamp", street, true, true, 52},
		{"an inn's lamp burns with the lamps out", inn, false, true, 50},
		{"an inn's lamp burns with the lamps lit", inn, true, true, 50},
		{"a room's own lamp burns on a street with the lamps out", lampPost, false, true, 40},
		{"a room's own lamp burns on a street with the lamps lit", lampPost, true, true, 40},
	} {
		got := c.room.composeWithFixtures(cfg, 70, c.lit, 1, nil, nil, nil, nil)
		if got.HasLamp != c.wantLamp || got.Lamp != c.lamp {
			t.Errorf("%s: lamp (%v, %d), want (%v, %d)", c.name, got.HasLamp, got.Lamp, c.wantLamp, c.lamp)
		}
	}

	// With the lamps out the street is its sky alone; lit, the lamp joins it.
	day := street.composeWithFixtures(cfg, 70, false, 1, nil, nil, nil, nil)
	if day.Light != day.Sky {
		t.Errorf("street with the lamps out: Light %v, want the sky alone %v", day.Light, day.Sky)
	}
	night := street.composeWithFixtures(cfg, 10, true, 1, nil, nil, nil, nil)
	if night.Level != 52 {
		t.Errorf("street at a new-moon midnight = %d, want 52", night.Level)
	}
}

// setRound moves the world to an exact round, clearing the per-round memos.
func setRound(r uint64) {
	util.SetRoundCountForTest(r)
	gametime.ClearDateCacheForTest()
	gametime.ClearCelestialMemoForTest()
}

// The street lamp on the real clock. Every round of three sample days: the
// lamp is lit exactly while it is night or the clear sky reads below
// LightDimBelow, so it is lit all night. At each end of the day the lamp
// changes in the round the clear sky crosses the faces edge, not the round
// night ends or begins: lit one round before the morning crossing and out at
// it, out one round before the evening crossing and lit at it, and both
// crossings fall in daylight (IsNight false), which is the lamplighter's
// half of the rule that IsNight alone would get wrong.
func TestStreetLampFollowsTheDimSkyAtTheBoundary(t *testing.T) {
	withShippedBiomesAndClock(t)
	requireBiome(t, "city_thoroughfare")
	dim := float64(configs.GetLightingConfig().DimBelow)
	street := Room{Biome: "city_thoroughfare"}

	const roundsPerDay = 900
	for _, doy := range []int{356, 81, 172} {
		base := uint64(doy-1) * roundsPerDay
		type reading struct {
			night, dimSky, lamp bool
		}
		day := make([]reading, roundsPerDay)
		for i := range day {
			setRound(base + uint64(i))
			day[i] = reading{
				night:  gametime.IsNight(),
				dimSky: gametime.CelestialLight() < dim,
				lamp:   street.LightTerms().HasLamp,
			}
			if want := day[i].night || day[i].dimSky; day[i].lamp != want {
				t.Errorf("day %d round %d: lamp lit %v, want %v (night %v, clear sky below %v: %v)",
					doy, i, day[i].lamp, want, day[i].night, dim, day[i].dimSky)
			}
			if day[i].night && !day[i].lamp {
				t.Errorf("day %d round %d: night with the street lamp out", doy, i)
			}
		}

		// The morning crossing: the first round of the day the clear sky
		// reaches the faces edge. The evening one: the first round after
		// noon it is below the edge again.
		dawn, dusk := -1, -1
		for i := 1; i < roundsPerDay/2; i++ {
			if day[i-1].dimSky && !day[i].dimSky {
				dawn = i
				break
			}
		}
		for i := roundsPerDay / 2; i < roundsPerDay; i++ {
			if !day[i-1].dimSky && day[i].dimSky {
				dusk = i
				break
			}
		}
		if dawn < 0 || dusk < 0 {
			t.Fatalf("day %d: no crossing of the faces edge found (dawn %d, dusk %d)", doy, dawn, dusk)
		}
		if !day[dawn-1].lamp || day[dawn].lamp {
			t.Errorf("day %d morning: lamp %v one round before the crossing and %v at it, want lit then out",
				doy, day[dawn-1].lamp, day[dawn].lamp)
		}
		if day[dusk-1].lamp || !day[dusk].lamp {
			t.Errorf("day %d evening: lamp %v one round before the crossing and %v at it, want out then lit",
				doy, day[dusk-1].lamp, day[dusk].lamp)
		}
		if day[dawn-1].night || day[dusk].night {
			t.Errorf("day %d: a crossing fell in the night (morning %v, evening %v); the sky-dim half is untested",
				doy, day[dawn-1].night, day[dusk].night)
		}
	}

	// The case that forced the ruling: midwinter 08:00 is day, its clear sky
	// too dim to read a face, and its street lamps burn.
	setClock(356, 8)
	if gametime.IsNight() {
		t.Fatalf("midwinter 08:00 reads as night; the probe is off")
	}
	if !street.LightTerms().HasLamp {
		t.Errorf("midwinter 08:00: street lamp out, want lit (clear sky %.2f, faces edge %v)",
			gametime.CelestialLight(), dim)
	}
}

// The shipped biome files: the two street biomes carry street lamps, and the
// indoor and magical lamps do not (spec section 2).
func TestShippedStreetLamps(t *testing.T) {
	withShippedBiomesAndClock(t)
	for id, want := range map[string]bool{
		"city_thoroughfare": true, "city_backstreet": true,
		"interior": false, "ether": false, "spiderweb": false,
	} {
		b, ok := GetBiome(id)
		if !ok {
			t.Fatalf("biome %q is not shipped", id)
		}
		if b.StreetLamp != want {
			t.Errorf("%s: streetlamp = %v, want %v", id, b.StreetLamp, want)
		}
		if _, has := b.LampValue(); !has {
			t.Errorf("%s: declares no lamp at all", id)
		}
	}
}
````

**Modify `internal/rooms/test_helpers.go`:**

````diff
@@ -54,12 +54,13 @@ func SeedBiomesForTest(biomeMap map[string]*BiomeInfo) func() {
 func SkyLightPtr(f float64) *float64 { return &f }
 func LampPtr(n int) *int             { return &n }
 
-// LightTermsAtForTest composes this room's light with the celestial term and
-// the weather sky filter supplied, rather than read from the clock and the
-// active mutators. A cross-package golden uses it to pin moon states that no
-// single round of the real clock can produce (every moon new, or every moon
-// full). Everything else (lamp, fixtures, carried light) is read as
-// LightTerms reads it.
-func (r *Room) LightTermsAtForTest(celestial, skyFilter float64) LightTerms {
-	return r.composeLight(configs.GetLightingConfig(), celestial, skyFilter)
+// LightTermsAtForTest composes this room's light with the celestial term,
+// whether the street lamps are lit (gametime.LampsLitAt on the caller's own
+// night and celestial) and the weather sky filter supplied, rather than read
+// from the clock and the active mutators. A cross-package golden uses it to
+// pin moon states that no single round of the real clock can produce (every
+// moon new, or every moon full). Everything else (lamp, fixtures, carried
+// light) is read as LightTerms reads it.
+func (r *Room) LightTermsAtForTest(celestial float64, lampsLit bool, skyFilter float64) LightTerms {
+	return r.composeLight(configs.GetLightingConfig(), celestial, lampsLit, skyFilter)
 }
````

**Modify `lighting_balance_spread_golden_test.go`:**

````diff
@@ -147,6 +147,9 @@ func TestLightingBalanceSpread(t *testing.T) {
 			gametime.SunLight(cfg, s.Doy, s.Hour),
 			gametime.MoonLight(cfg, s.Moons, s.Moons, s.Moons),
 		)
+		// The lamps follow the sky being composed: the lamplighter sees
+		// this sample's moons, not the live clock's.
+		lampsLit := gametime.LampsLitAt(gametime.NightAt(cfg.WorldLatitude, s.Doy, s.Hour), celestial, cfg.DimBelow)
 		fmt.Fprintf(&b, "== %s\n", s.Label)
 		for _, id := range spread {
 			r := rooms.LoadRoom(id)
@@ -157,7 +160,7 @@ func TestLightingBalanceSpread(t *testing.T) {
 			if bi := r.GetBiome(); bi != nil {
 				biome = bi.BiomeId
 			}
-			terms := r.LightTermsAtForTest(celestial, 1)
+			terms := r.LightTermsAtForTest(celestial, lampsLit, 1)
 			fmt.Fprintf(&b, "room %d %s biome=%s raw=%.3f level=%d\n", id, want[id], biome, terms.Raw, terms.Level)
 		}
 	}
@@ -180,8 +183,8 @@ func TestLightingBalanceSpread(t *testing.T) {
 			"This golden was recorded before lighting plan 6 rebuilt the light " +
 			"arithmetic. It is allowed to move, but every move must be one you " +
 			"predicted: the rebuild only raises the dim end (a reading at or above " +
-			"37 moves by under 0.5), the night-only street lamp drops street rooms " +
-			"to daylight alone by day, and the room data pass relights the rooms " +
+			"37 moves by under 0.5), the street lamp drops street rooms to daylight " +
+			"alone while the clear sky shows faces, and the room data pass relights the rooms " +
 			"the review page approved.\n\n" +
 			"Work out the expected diff first, compare, and only then:\n" +
 			"  go test . -run TestLightingBalanceSpread -update-lighting-balance-spread -v")
````

**Modify `lighting_no_natural_negative_test.go`:**

````diff
@@ -72,15 +72,19 @@ func TestNoShippedRoomReadsBelowZeroWithoutDarkness(t *testing.T) {
 	floor := float64(cfg.RealMinimum)
 	type sample struct {
 		celestial float64
+		lampsLit  bool
 	}
 	var samples []sample
 	for _, doy := range []int{356, 81, 172} {
 		for hour := 0; hour < 24; hour++ {
 			for _, m := range []float64{0, 0.5, 1} {
+				celestial := lightscale.Combine(cfg.DoublingStep,
+					gametime.SunLight(cfg, doy, float64(hour)),
+					gametime.MoonLight(cfg, m, m, m))
 				samples = append(samples, sample{
-					celestial: lightscale.Combine(cfg.DoublingStep,
-						gametime.SunLight(cfg, doy, float64(hour)),
-						gametime.MoonLight(cfg, m, m, m)),
+					celestial: celestial,
+					lampsLit: gametime.LampsLitAt(
+						gametime.NightAt(cfg.WorldLatitude, doy, float64(hour)), celestial, cfg.DimBelow),
 				})
 			}
 		}
@@ -94,7 +98,7 @@ func TestNoShippedRoomReadsBelowZeroWithoutDarkness(t *testing.T) {
 		}
 		for si, s := range samples {
 			for f := range filterSet {
-				terms := r.LightTermsAtForTest(s.celestial, f)
+				terms := r.LightTermsAtForTest(s.celestial, s.lampsLit, f)
 				checked++
 				bad := terms.Raw < 0 || terms.Level < 0 || (terms.Raw > 0 && terms.Raw < floor)
 				if bad {
````

- [ ] **Step 2: Packages, then two goldens move**

```bash
gofmt -l internal/
go build ./...
go vet ./...
go test ./internal/gametime ./internal/behaviortree ./internal/lightnotice ./internal/rooms ./internal/itemlight -count=1
go test . -run "TestLightingBalanceSpread|TestLightingDayCycleAcrossSampleRounds|TestLightingParityAcrossEveryShippedRoom|TestLightingFixtureDayCycle" -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: nothing from `gofmt`, all packages `ok`, then `--- FAIL` for exactly `TestLightingBalanceSpread` and `TestLightingDayCycleAcrossSampleRounds` (the parity and fixture day-cycle goldens hold). Re-record the two:

```bash
go test . -run TestLightingBalanceSpread -update-lighting-balance-spread -count=1
go test . -run TestLightingDayCycleAcrossSampleRounds -update-lighting-daycycle -count=1
git diff --stat -- testdata
```

Expected: the spread golden 30 lines changed each way, the day-cycle golden 802. What the dry run read: in the spread golden 30 entries moved, all in the three city rooms of the spread (200, 202, 5803), all daytime samples, all down (the lamp is out and the street reads its sky alone). In the day-cycle golden 802 entries moved, every one a city room in a daytime sample, every one down. The dim-sky half of the rule moves no golden entry: every golden samples 00:00, 06:00, 12:00 and 18:00 on days 356, 81 and 172, where `LampsLit` equals `IsNight`; `TestStreetLampFollowsTheDimSkyAtTheBoundary` and `TestLampsLitOnTheClock` cover the crossings instead.

- [ ] **Step 3: The full suite, with the #207 guard**

```bash
go test . -run TestEveryShopkeeperCanTradeAtNightWhileAwake -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL)"
go test ./... -count=1 2>&1 | grep -E "^(--- FAIL|FAIL|panic)"
```

Expected: `--- PASS: TestEveryShopkeeperCanTradeAtNightWhileAwake`, `ok`; then nothing (the dry run: 129 packages `ok`). If the guard reports keepers "refused at 2 awake samples, first midwinter 08:00 light 40", the lamp is reading `IsNight` alone (see "Where the spec could not be implemented as written", item 1).

- [ ] **Step 4: Spec check and commit**

```bash
P="_datafiles/world/dogmud/behaviors/items/dusk_to_dawn.yaml _datafiles/world/dogmud/biomes/city_backstreet.yaml _datafiles/world/dogmud/biomes/city_thoroughfare.yaml _datafiles/world/dogmud/items/other-0/55-arch_lantern.yaml _datafiles/world/dogmud/narration/light-notices/lamp.yaml internal/behaviortree/conditions_after_dusk_test.go internal/behaviortree/conditions_lamplit_test.go internal/behaviortree/conditions_state.go internal/behaviortree/shipped_item_trees_test.go internal/gametime/celestial.go internal/gametime/gametime.go internal/gametime/lamps_lit_test.go internal/lightnotice/integration_test.go internal/lightnotice/tracker.go internal/rooms/biomes.go internal/rooms/fixture_compose_test.go internal/rooms/light_terms_test.go internal/rooms/light_trim.go internal/rooms/lighting.go internal/rooms/lighting_model_test.go internal/rooms/room_light_override_test.go internal/rooms/sky_filter_test.go internal/rooms/street_lamp_test.go internal/rooms/test_helpers.go lighting_balance_spread_golden_test.go lighting_no_natural_negative_test.go testdata/lighting_balance_spread.golden testdata/lighting_daycycle.golden"
git add $P
git diff --cached adac03187 -- $P
git commit -m "feat(lighting): street lamps lit at night and while the sky is too dim to read a face (#372)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

The diff must print nothing.

---

### Task 5: surface every lighting knob in `config.yaml`

Checkpoint `a6c68e295`. Model: sonnet.

- [ ] **Step 1: The test first**

Create `lighting_config_surface_test.go` (block below) and run `go test . -run TestShippedConfigSurfacesEveryLightingKnob -count=1`. Against Task 4's tree it fails with thirteen lines of the form `config.yaml does not set LightBlindBelow: it runs its Go default unseen` (`LightBlindBelow`, `LightDimBelow`, `LightExitsAbove`, `LightRealMinimum`, `LightDefaultVisionStrength`, `LightDoublingStep`, `WorldLatitude`, `LightEquinoxNoon`, `LightStarlight`, `LightMoonsFull`, `LightMoonWeightSwiftmoon`, `LightMoonWeightWanderer`, `LightMoonWeightEye`), then `shipped LightExitsAbove = 65, want 55`. The `config.lighting_accessor_test.go` block only rewrites comments: `go test ./internal/configs -count=1` passes before and after.

**`config.yaml` carries skip-worktree in some checkouts.** First run `git ls-files -v _datafiles/config.yaml`:

- `H` (normal): apply the `_datafiles/config.yaml` hunk below with the Edit tool and stage it with the other paths in Step 3.
- `S` (skip-worktree): build the change from the committed blob, never from disk (the `dogmud-balance-config` skill, `.claude/skills/dogmud-balance-config`). `git show HEAD:_datafiles/config.yaml > "$TMP/config.yaml"`; apply the hunk to `$TMP/config.yaml` with the Edit tool; `B=$(git hash-object -w "$TMP/config.yaml")`; `git update-index --cacheinfo 100644,$B,_datafiles/config.yaml`; then `git update-index --skip-worktree _datafiles/config.yaml`, because `--cacheinfo` clears the bit; `git ls-files -v _datafiles/config.yaml` must read `S` again. The tests read the file on disk, so for the runs in Step 2 also apply the same hunk to the disk copy.

**Modify `_datafiles/config.yaml`:**

````diff
@@ -939,11 +939,50 @@ Balance:
   DazzleCap: 0.80
 
   # ── LIGHT: THRESHOLDS ─────────────────────────────────────────────────────
+  # The sight bands of a normal observer on the -100..100 light scale. Below
+  #   LightBlindBelow they see nothing; below LightDimBelow shapes but not
+  #   faces. The pair must be ordered; an invalid pair reverts both to 25/50.
+  LightBlindBelow: 25
+  LightDimBelow: 50
+  # LightExitsAbove: at or above this light a normal observer can look and
+  #   scan through an exit into the next room (lighting plan 6: 55, so a
+  #   lamplit street at night needs a candle or better). Never below
+  #   LightBlindBelow; out of range reverts to 65.
+  LightExitsAbove: 55
+  # LightRealMinimum: the least a room with any real light reads before
+  #   magical darkness is subtracted (lighting plan 6), so a sliver of light
+  #   never rounds to the same 0 as a sealed cave. 1 to LightBlindBelow - 1;
+  #   zero reverts to 3.
+  LightRealMinimum: 3
   # LightDazzleAbove: where the comfortable band ends and too-bright begins
   #   for a normal observer, on the same -100..100 scale. Must sit above
-  #   LightDimBelow (default 50; not set in this file) and at most 100; zero
-  #   reverts to 75.
+  #   LightDimBelow and at most 100; zero reverts to 75.
   LightDazzleAbove: 75
+  # LightDefaultVisionStrength: the window shift of a night vision flag that
+  #   declares no strength of its own. 1 to 24; zero reverts to 12.
+  LightDefaultVisionStrength: 12
+
+  # ── LIGHT: SCALE, SUN AND MOONS ───────────────────────────────────────────
+  # LightDoublingStep: scale points per doubling of light. It governs how
+  #   sources combine, how a sky fraction or weather dims, and the daylight
+  #   curve. Zero or negative reverts to 8.
+  LightDoublingStep: 8
+  # WorldLatitude: degrees north the whole world sits at; it sets day length
+  #   and the seasons. Zero means unset and reverts to 46.5.
+  WorldLatitude: 46.5
+  # LightEquinoxNoon: the open sky's light at noon on an equinox, the sun's
+  #   one calibration point. Zero reverts to 70.
+  LightEquinoxNoon: 70
+  # LightStarlight / LightMoonsFull: the night sky with every moon new and
+  #   with every moon full. Starlight must sit below the full value; an
+  #   invalid pair reverts both to 10/35.
+  LightStarlight: 10
+  LightMoonsFull: 35
+  # The three moons' relative light at full, The Wanderer as 1.0. All three
+  #   at zero reverts the set to 4.0 / 1.0 / 0.5.
+  LightMoonWeightSwiftmoon: 4.0
+  LightMoonWeightWanderer: 1.0
+  LightMoonWeightEye: 0.5
 
   # ── LIGHT: SPELL SCALING (lighting plan 5a) ─────────────────────────────────
   # A light spell (condition with light_strength: magnitude) is cast at
````

**Modify `internal/configs/config.balance.go`:**

````diff
@@ -1208,9 +1208,10 @@ type Balance struct {
 	// LightDoublingStep is how many points on the -100..100 light scale are
 	// worth TWICE as much physical light. It is the single constant relating
 	// the perceptual scale to real light, and it governs three things at once:
-	// combining sources (two equal lamps read one step brighter), applying a
-	// sky fraction (half the light is minus one step) and the shape of the
-	// daylight curve.
+	// combining sources (since lighting plan 6 they add as linear brightness,
+	// so two equal lamps read nearly one step brighter), applying a sky
+	// fraction (half the light is nearly minus one step well above the dim
+	// end) and the shape of the daylight curve.
 	//
 	// Eight means the 100-point span covers about 12.5 doublings, and the
 	// 25-point sight bands are about three doublings wide, so climbing from
@@ -1230,10 +1231,11 @@ type Balance struct {
 	//
 	// 🔴 An earlier draft honoured zero as "this world has no latitude", with
 	// night length falling back to Timing.NightHours. That could not work. Go
-	// cannot distinguish an unset float from an authored zero, and none of the
-	// lighting knobs appear in config.yaml, so the shipped configuration IS a
-	// bare Balance. Honouring zero would have shipped DOGMud at no latitude:
-	// a flat eight-hour night, no seasons, and this entire model unreachable.
+	// cannot distinguish an unset float from an authored zero, and until
+	// lighting plan 6 surfaced it this key was absent from config.yaml, so the
+	// shipped configuration read a bare zero. Honouring zero would have
+	// shipped DOGMud at no latitude: a flat eight-hour night, no seasons, and
+	// this entire model unreachable.
 	//
 	// There is therefore NO path from day length back to Timing.NightHours.
 	// That knob still exists for upstream compatibility but nothing reads it
````

**Modify `internal/configs/config.balance.lighting.go`:**

````diff
@@ -137,10 +137,12 @@ func (b *Balance) validateLighting() {
 	// 🔴 An earlier draft had zero HONOURED, meaning "this world has no
 	// latitude, fall back to Timing.NightHours". That was incoherent, and the
 	// incoherence was not academic. Go cannot distinguish an unset float from
-	// an authored zero, and none of these knobs appear in config.yaml, so the
-	// shipped configuration IS a bare Balance. Honouring zero would therefore
-	// have shipped DOGMud at no latitude: a flat eight-hour night, no seasons,
-	// and the entire celestial model built and never once reached.
+	// an authored zero, and until lighting plan 6 surfaced it, WorldLatitude
+	// was absent from config.yaml, so the shipped configuration read a bare
+	// zero. Honouring zero would therefore have shipped DOGMud at no
+	// latitude: a flat eight-hour night, no seasons, and the entire celestial
+	// model built and never once reached. Any config file that omits the key
+	// is in the same position today.
 	//
 	// So zero is unset, and the NightHours fallback is deleted rather than
 	// repaired. An operator who wants an equator-like world of twelve-hour
````

**Modify `internal/configs/config.lighting_accessor_test.go`:**

````diff
@@ -8,9 +8,10 @@ func TestLightingDefaultsAreTheShippedCalibration(t *testing.T) {
 	if b.LightDoublingStep != 8 {
 		t.Errorf("LightDoublingStep = %v, want 8", b.LightDoublingStep)
 	}
-	// 🔴 This assertion is the one that matters most in this test. None of the
-	// lighting knobs appear in config.yaml, so a bare Balance is not a test
-	// fixture, it is the SHIPPED configuration. If this ever reads zero,
+	// 🔴 This assertion is the one that matters most in this test. A config
+	// file that omits WorldLatitude (as config.yaml did until lighting plan 6)
+	// arrives at a bare Balance, so this is not only a test fixture. If this
+	// ever reads zero,
 	// DOGMud is running with no latitude: a flat night, no seasons, and the
 	// whole celestial model unreachable.
 	if b.WorldLatitude != 46.5 {
@@ -47,8 +48,8 @@ func TestDoublingStepRejectsNonPositive(t *testing.T) {
 // reverts. Zero means UNSET and is coerced to the default.
 //
 // 🔴 This is the load-bearing assertion of the whole celestial model, not a
-// boundary nicety. None of the lighting knobs appear in config.yaml, so the
-// SHIPPED configuration is a bare Balance, whose WorldLatitude is zero. If zero
+// boundary nicety. A config file that omits WorldLatitude (as config.yaml did
+// until lighting plan 6) runs a bare Balance, whose WorldLatitude is zero. If zero
 // were honoured as "no latitude", DOGMud would ship with a flat night, no
 // seasons, and the entire model unreachable. Do not "fix" this test by making
 // zero survive.
````

**Create `lighting_config_surface_test.go`:**

````go
package main

import (
	"os"
	"regexp"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestShippedConfigSurfacesEveryLightingKnob is lighting plan 6's config
// surfacing guard (spec section 7). Every lighting knob is present in the
// shipped _datafiles/config.yaml, so none of them silently runs its Go
// default, and the two the plan retunes ship at their new values:
// LightExitsAbove 55 (owner ruling O5) and LightRealMinimum 3 (O3).
func TestShippedConfigSurfacesEveryLightingKnob(t *testing.T) {
	raw, err := os.ReadFile("_datafiles/config.yaml")
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	for _, key := range []string{
		"LightBlindBelow", "LightDimBelow", "LightExitsAbove", "LightRealMinimum",
		"LightDazzleAbove", "LightDefaultVisionStrength", "LightDoublingStep",
		"WorldLatitude", "LightEquinoxNoon", "LightStarlight", "LightMoonsFull",
		"LightMoonWeightSwiftmoon", "LightMoonWeightWanderer", "LightMoonWeightEye",
		"LightInfraReachCap", "LightInfraPenaltyFloor", "DarknessCombatPenalty", "DazzleCap",
	} {
		if !regexp.MustCompile(`(?m)^\s+` + key + `:\s*\S`).Match(raw) {
			t.Errorf("config.yaml does not set %s: it runs its Go default unseen", key)
		}
	}

	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	cfg := configs.GetLightingConfig()
	for _, c := range []struct {
		name      string
		got, want float64
	}{
		{"LightBlindBelow", float64(cfg.BlindBelow), 25},
		{"LightDimBelow", float64(cfg.DimBelow), 50},
		{"LightExitsAbove", float64(cfg.ExitsAbove), 55},
		{"LightRealMinimum", float64(cfg.RealMinimum), 3},
		{"LightDazzleAbove", float64(cfg.DazzleAbove), 75},
		{"LightDefaultVisionStrength", float64(cfg.DefaultVisionStrength), 12},
		{"LightDoublingStep", cfg.DoublingStep, 8},
		{"WorldLatitude", cfg.WorldLatitude, 46.5},
		{"LightEquinoxNoon", cfg.EquinoxNoon, 70},
		{"LightStarlight", cfg.Starlight, 10},
		{"LightMoonsFull", cfg.MoonsFull, 35},
		{"LightMoonWeightSwiftmoon", cfg.MoonWeightSwiftmoon, 4},
		{"LightMoonWeightWanderer", cfg.MoonWeightWanderer, 1},
		{"LightMoonWeightEye", cfg.MoonWeightEye, 0.5},
	} {
		if c.got != c.want {
			t.Errorf("shipped %s = %v, want %v", c.name, c.got, c.want)
		}
	}
}
````

- [ ] **Step 2: Packages and the root guards**

```bash
gofmt -l internal/ lighting_config_surface_test.go
go build ./...
go vet ./internal/configs .
go test ./internal/configs -count=1
go test . -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: nothing from `gofmt`; `ok`; `ok`. Test binaries read Go defaults unless a test reloads the shipped file (the `dogmud-writing-tests` skill); the Go default of `LightExitsAbove` stays 65, and only the shipped file says 55.

- [ ] **Step 3: Spec check and commit**

```bash
P="_datafiles/config.yaml internal/configs/config.balance.go internal/configs/config.balance.lighting.go internal/configs/config.lighting_accessor_test.go lighting_config_surface_test.go"
git add internal/configs/config.balance.go internal/configs/config.balance.lighting.go internal/configs/config.lighting_accessor_test.go lighting_config_surface_test.go
git ls-files -v _datafiles/config.yaml
git diff --cached a6c68e295 -- $P
git commit -m "feat(config): surface every lighting knob in config.yaml, exits need 55 (#372)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

With `H`, also `git add _datafiles/config.yaml` before the diff; with `S` it is already staged from the blob. The diff must print nothing: it compares the staged `config.yaml`, never the disk copy, against the checkpoint.

---

### Task 6: infravision shows shapes through exits

Checkpoint `a16cc8457`. Model: sonnet.

- [ ] **Step 1: The tests first**

Apply `internal/actions/scan_heat_test.go`, `internal/usercommands/look_exit_heat_test.go` and the `internal/usercommands/darkness_gates_sight_test.go` block. They build against Task 5's code and fail:

- `go test ./internal/actions -run Heat -count=1`: three subtests of `TestScan_HeatShowsShapesThroughAnExit` ("infra, too dark to see out: a figure by heat", "infra into an unlit cave: a figure by heat", "infra into darkness within reach: a figure") fail with `figure = false, want true: "You scan the surrounding area...\n\n  <ansi fg=\"exit\">north</ansi> (<ansi fg=\"room-title\"></ansi>): too dark to make anything out\n"` and `too dark = true, want false: ...`. The other three subtests (no infra, darkness beyond reach, light enough to see out) pass before and after: they guard what heat must not do.
- `go test ./internal/usercommands -count=1`: `TestLookExit_HeatShowsFiguresThroughAnExit/infravision,_too_dark_to_see_through:_two_figures_by_heat` fails with `"<ansi fg=\"system\">It's too dark to see anything in that direction.</ansi>\n" does not contain "You peer toward the south."`, and `TestLookExit_HeatInAnEmptyRoomFindsNothingWarm` with `... does not contain "nothing warm moves there"`. The `darkness_gates_sight_test.go` block passes before and after (its infravision case still refuses the light test; it now also accepts the heat line).

Then apply the code blocks.

**Modify `internal/actions/look.go`:**

````diff
@@ -5,6 +5,7 @@ import (
 
 	"github.com/GoMudEngine/GoMud/internal/keywords"
 	"github.com/GoMudEngine/GoMud/internal/messaging"
+	"github.com/GoMudEngine/GoMud/internal/rooms"
 	"github.com/GoMudEngine/GoMud/internal/state/perception"
 )
 
@@ -19,6 +20,7 @@ const (
 	LookExit                        // an exit the looker can see through
 	LookExitTooDark                 // an exit, but too dark to see through it
 	LookExitLocked                  // an exit that is locked
+	LookExitShapes                  // an exit too dark to see through, whose occupants heat shows as shapes (lighting plan 6)
 	LookOther                       // anything else: each wrapper's own objects, in its own order
 )
 
@@ -32,7 +34,7 @@ type LookResolution struct {
 	Target         Actor  // LookCreature
 	LookAt         string // the target, after a direction alias resolved to an exit
 	ExitName       string // the LookExit kinds
-	ExitRoomId     int    // LookExit
+	ExitRoomId     int    // LookExit, LookExitShapes
 	// PetUserId is the owner of a pet the looker may name here (clear sight
 	// only), 0 otherwise. It is not a kind because the player resolves the
 	// pet AFTER carried items and room nouns; each wrapper reads it at its
@@ -98,9 +100,18 @@ func ResolveLook(actor Actor, lookAt string) LookResolution {
 	res.ExitName, res.ExitRoomId = exitName, exitRoomId
 
 	// Seeing THROUGH an exit needs more light than seeing the room you are
-	// standing in (messaging.SeesThroughExit; infra reach does not help).
+	// standing in (messaging.SeesThroughExit). When the light refuses, heat
+	// may still show the next room's occupants as shapes (lighting plan 6,
+	// owner ruling O6), through an exit that is not locked.
 	if !messaging.SeesThroughExit(char, room) {
 		res.Kind = LookExitTooDark
+		if next := rooms.LoadRoom(exitRoomId); next != nil && messaging.SensesHeatThroughExit(char, room, next) {
+			if info, _ := room.GetExitInfo(exitName); info.Lock.IsLocked() {
+				res.Kind = LookExitLocked
+			} else {
+				res.Kind = LookExitShapes
+			}
+		}
 		return res
 	}
 	if info, _ := room.GetExitInfo(exitName); info.Lock.IsLocked() {
````

**Modify `internal/actions/scan.go`:**

````diff
@@ -4,6 +4,7 @@ import (
 	"fmt"
 	"strings"
 
+	"github.com/GoMudEngine/GoMud/internal/characters"
 	"github.com/GoMudEngine/GoMud/internal/messaging"
 	"github.com/GoMudEngine/GoMud/internal/mobs"
 	"github.com/GoMudEngine/GoMud/internal/rooms"
@@ -113,11 +114,19 @@ func Scan(actor Actor, opts ScanOptions) ScanResult {
 			// 5c): out through the exit from here, then into the next room.
 			// With faces, names; with shapes, the anonymous figure the room
 			// roster uses, one per creature and uncolored so a mob and a
-			// player read alike; with neither, nobody. The structured
-			// result is left whole for the mob callers.
+			// player read alike; with neither, nobody. When the light here
+			// refuses, heat may still show the next room's occupants as
+			// shapes (lighting plan 6, owner ruling O6); it never upgrades a
+			// view the light grants. The structured result is left whole
+			// for the mob callers.
 			sight := messaging.SightNone
-			if adjRoom := rooms.LoadRoom(s.RoomId); seesOut && adjRoom != nil {
-				sight = messaging.ParticipantSight(viewer, adjRoom)
+			if adjRoom := rooms.LoadRoom(s.RoomId); adjRoom != nil {
+				switch {
+				case seesOut:
+					sight = messaging.ParticipantSight(viewer, adjRoom)
+				case messaging.SensesHeatThroughExit(viewer, room, adjRoom):
+					sight = messaging.SightShapes
+				}
 			}
 			parts := []string{}
 			for _, m := range s.Mobs {
@@ -161,3 +170,35 @@ func Scan(actor Actor, opts ScanOptions) ScanResult {
 
 	return result
 }
+
+// FiguresSensedIn is one anonymous figure (messaging.UnseenFigure at
+// SightShapes, the room roster's shapes vocabulary) per creature in room that
+// a viewer would list there: every mob that is not hidden and every player the
+// viewer perceives, leaving out selfUserId. It is what heat shows through an
+// exit (lighting plan 6): a count of bodies, never a name.
+func FiguresSensedIn(viewer *characters.Character, room *rooms.Room, selfUserId int) []string {
+	if room == nil {
+		return nil
+	}
+	n := 0
+	for _, id := range room.GetMobs(rooms.FindAll) {
+		if m := mobs.GetInstance(id); m != nil && !m.Character.IsHidden() {
+			n++
+		}
+	}
+	for _, id := range room.GetPlayers(rooms.FindAll) {
+		if id == selfUserId {
+			continue
+		}
+		u := users.GetByUserId(id)
+		if u == nil || (viewer != nil && !viewer.Perceives(u.Character)) {
+			continue
+		}
+		n++
+	}
+	out := make([]string, 0, n)
+	for range n {
+		out = append(out, messaging.UnseenFigure(messaging.SightShapes))
+	}
+	return out
+}
````

**Create `internal/actions/scan_heat_test.go`:**

````go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// Lighting plan 6, owner ruling O6: infravision shows the next room's
// occupants as shapes through an exit when the light here is too poor to see
// through it, down to minus the reach in the next room. Never a name, and
// never in place of a view the light grants.
const (
	scanHeatHereId  = 9494
	scanHeatThereId = 9495
	scanHeatScoutId = 9496
	scanHeatInfraId = 9497
)

// scanHeatSent scans from a cave lit at hereLamp into one lit at thereLamp
// and darkened by thereDark (0 for none), with or without infra reach 30.
func scanHeatSent(t *testing.T, hereLamp, thereLamp int, thereDark float64, infra bool) string {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		scanHeatInfraId: {ConditionId: scanHeatInfraId, Name: "Test Heat Sight", RoundInterval: 1, TriggerCount: 10,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	here := &rooms.Room{RoomId: scanHeatHereId, Zone: "ScanHeat", Biome: "cave", Lamp: rooms.LampPtr(hereLamp),
		Exits: map[string]exit.RoomExit{"north": {RoomId: scanHeatThereId}}}
	there := &rooms.Room{RoomId: scanHeatThereId, Zone: "ScanHeat", Biome: "cave", Lamp: rooms.LampPtr(thereLamp),
		Exits: map[string]exit.RoomExit{"south": {RoomId: scanHeatHereId}}}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{scanHeatHereId: here, scanHeatThereId: there},
		map[string]*rooms.ZoneConfig{"ScanHeat": {Name: "ScanHeat", RoomId: scanHeatHereId,
			RoomIds: map[int]struct{}{scanHeatHereId: {}, scanHeatThereId: {}}}},
	))
	t.Cleanup(itemlight.ResetForTest())
	if thereDark > 0 {
		itemlight.Set(scanHeatThereId, uuid.New(), itemlight.Darkness, thereDark)
	}
	scout := newScanTestMob(scanHeatScoutId, "Midroad Scout", scanHeatThereId)
	mobs.SetInstanceForTest(scanHeatScoutId, scout)
	t.Cleanup(func() { mobs.SetInstanceForTest(scanHeatScoutId, nil) })
	there.AddMob(scanHeatScoutId)

	actor := newScanFakeActor("Scanner", here, true, 9498)
	if infra {
		if !actor.char.Conditions.AddCondition(scanHeatInfraId, true) {
			t.Fatal("could not give the scanner infra reach")
		}
		if got := actor.char.InfraReach(); got != 30 {
			t.Fatalf("scanner infra reach = %d, want 30", got)
		}
	}
	Scan(actor, ScanOptions{})
	return strings.Join(actor.sent, "\n")
}

func TestScan_HeatShowsShapesThroughAnExit(t *testing.T) {
	cases := []struct {
		name            string
		here, there     int
		thereDark       float64
		infra           bool
		named, figure   bool
		tooDarkToMakeIt bool
	}{
		{"no infra, too dark to see out: nobody", 30, 30, 0, false, false, false, true},
		{"infra, too dark to see out: a figure by heat", 30, 30, 0, true, false, true, false},
		{"infra into an unlit cave: a figure by heat", 30, 0, 0, true, false, true, false},
		{"infra into darkness within reach: a figure", 30, 0, 20, true, false, true, false},
		{"infra into darkness beyond reach: nobody", 30, 0, 40, true, false, false, true},
		{"infra, light here sees out: the name, never downgraded", 90, 90, 0, true, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sent := scanHeatSent(t, c.here, c.there, c.thereDark, c.infra)
			if got := strings.Contains(sent, "Midroad Scout"); got != c.named {
				t.Errorf("named = %v, want %v: %q", got, c.named, sent)
			}
			if got := strings.Contains(sent, "a figure"); got != c.figure {
				t.Errorf("figure = %v, want %v: %q", got, c.figure, sent)
			}
			if got := strings.Contains(sent, "too dark to make anything out"); got != c.tooDarkToMakeIt {
				t.Errorf("too dark = %v, want %v: %q", got, c.tooDarkToMakeIt, sent)
			}
		})
	}
}
````

**Modify `internal/messaging/predicates.go`:**

````diff
@@ -103,6 +103,31 @@ func SeesThroughExit(observer *characters.Character, room RoomVisibility) bool {
 	return ExitThroughWindow(room.LightLevel(), observer.NightVisionStrength(), configs.GetLightingConfig().ExitsAbove)
 }
 
+// SensesHeatThroughExit reports whether an observer whose light test through
+// an exit failed (SeesThroughExit false) still makes out the next room's
+// occupants as shapes by their heat (lighting plan 6, owner ruling O6). It
+// needs infra reach, some sight here (ParticipantSight not SightNone, so a
+// blinded observer senses nothing), and the next room's light at or above
+// minus the reach, the same depth own-room infravision reads to
+// (SightThroughWindow).
+//
+// Heat shows bodies, never names, a room's description or its items, and it
+// never upgrades a view: a caller asks it only after SeesThroughExit has
+// refused. A nil observer or room senses nothing.
+func SensesHeatThroughExit(observer *characters.Character, here, next RoomVisibility) bool {
+	if observer == nil || here == nil || next == nil {
+		return false
+	}
+	reach := observer.InfraReach()
+	if reach <= 0 {
+		return false
+	}
+	if ParticipantSight(observer, here) == SightNone {
+		return false
+	}
+	return next.LightLevel() >= -reach
+}
+
 // FixedLight is a RoomVisibility at one light value. A caller judging many
 // observers in one room reads room.LightLevel() once and passes FixedLight,
 // rather than recomposing the room's light per observer.
````

**Modify `internal/messaging/window.go`:**

````diff
@@ -66,8 +66,9 @@ func SightThroughWindow(light, strength, reach int, blindBelow, dimBelow int) Si
 // ExitThroughWindow reports whether an observer sees THROUGH an exit into the
 // next room at a given light. exitsAbove (Balance.LightExitsAbove) is the edge
 // for normal eyes; night-vision strength moves it down exactly as it moves the
-// blind and dim edges, clamped the same way. Infra reach plays no part: heat
-// shows shapes in the observer's own room, not in the room beyond an exit.
+// blind and dim edges, clamped the same way. Infra reach plays no part in this
+// light test; heat has its own path through an exit, shapes only
+// (SensesHeatThroughExit, lighting plan 6).
 // The caller still refuses first when the observer reads nothing at all here.
 func ExitThroughWindow(light, strength, exitsAbove int) bool {
 	return light >= exitsAbove-clampShift(strength)
````

**Modify `internal/mobcommands/look.go`:**

````diff
@@ -40,7 +40,7 @@ func Look(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
 	// observer's sight, as the player's do.
 	res := actions.ResolveLook(actions.NewMobActorInRoom(mob, room), rest)
 	switch res.Kind {
-	case actions.LookBlind, actions.LookTooDark, actions.LookExitTooDark, actions.LookExitLocked:
+	case actions.LookBlind, actions.LookTooDark, actions.LookExitTooDark, actions.LookExitLocked, actions.LookExitShapes:
 		return true, nil
 
 	case actions.LookRoom:
````

**Modify `internal/usercommands/darkness_gates_sight_test.go`:**

````diff
@@ -133,8 +133,9 @@ func TestDarknessGates_ReadSightNotTheNightVisionFlag(t *testing.T) {
 
 // Seeing THROUGH an exit keeps LightExitsAbove as its threshold for normal
 // eyes. Nightvision shifts that edge down exactly as it shifts the blind and
-// dim edges (by its strength, capped at the window shift cap); infra reach
-// reads heat in the observer's own room and does not reach the next one.
+// dim edges (by its strength, capped at the window shift cap). Infra reach
+// does not pass the light test; since lighting plan 6 (owner ruling O6) heat
+// shows the next room's occupants as figures instead (look_exit_heat_test.go).
 func TestLookDirection_ExitThresholdShiftsWithNightVisionStrength(t *testing.T) {
 	const tooDark = "too dark to see anything in that direction"
 	cases := []struct {
@@ -146,7 +147,7 @@ func TestLookDirection_ExitThresholdShiftsWithNightVisionStrength(t *testing.T)
 		{"normal eyes at 45 are refused (edge 65)", 45, 0, true},
 		{"strength 24 at 45 peers (edge 41)", 45, gateNightConditionId, false},
 		{"bare flag at 45 is refused (edge 53)", 45, gateNightFlagOnlyId, true},
-		{"infravision at 45 is refused (heat does not reach the next room)", 45, gateInfraConditionId, true},
+		{"infravision at 45 fails the light test and senses heat instead", 45, gateInfraConditionId, true},
 		{"strength 24 at light 0 is refused (it could see nothing here)", 0, gateNightConditionId, true},
 	}
 	for _, c := range cases {
@@ -157,7 +158,8 @@ func TestLookDirection_ExitThresholdShiftsWithNightVisionStrength(t *testing.T)
 			}
 			out := runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
 			if c.refused {
-				require.True(t, strings.Contains(out, tooDark) || strings.Contains(out, tooDarkToSeeLine),
+				require.True(t, strings.Contains(out, tooDark) || strings.Contains(out, tooDarkToSeeLine) ||
+					strings.Contains(out, "It's too dark to see that way"),
 					"must refuse; got:\n%s", out)
 			} else {
 				require.NotContains(t, out, tooDark)
````

**Modify `internal/usercommands/look.go`:**

````diff
@@ -153,6 +153,24 @@ func Look(rest string, user *users.UserRecord, room *rooms.Room, flags events.Ev
 		user.SendText(messaging.CategorySystem, fmt.Sprintf("The %s exit is locked.", res.ExitName))
 		return true, nil
 
+	case actions.LookExitShapes:
+		// Too dark to see through, but heat shows the next room's
+		// occupants (lighting plan 6, owner ruling O6): the roster's
+		// anonymous figures, never a name, the room's description or its
+		// items.
+		user.SendText(messaging.CategorySystem, fmt.Sprintf("You peer toward the %s.", res.ExitName))
+		if !isSneaking {
+			room.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> peers toward the %s.`, user.Character.Name, res.ExitName), []string{user.Character.Name}, user.UserId)
+		}
+		figures := actions.FiguresSensedIn(user.Character, rooms.LoadRoom(res.ExitRoomId), user.UserId)
+		if len(figures) == 0 {
+			user.SendText(messaging.CategorySystem, `It's too dark to see that way, and nothing warm moves there.`)
+		} else {
+			user.SendText(messaging.CategorySystem, `It's too dark to see that way, but you sense the warmth of:`)
+			user.SendText(messaging.CategorySystem, `  `+strings.Join(figures, `, `))
+		}
+		return true, nil
+
 	case actions.LookExit:
 		user.SendText(messaging.CategorySystem, fmt.Sprintf("You peer toward the %s.", res.ExitName))
 		if !isSneaking {
````

**Create `internal/usercommands/look_exit_heat_test.go`:**

````go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Lighting plan 6, owner ruling O6: `look <exit>` with infravision, when the
// light here is too poor to see through the exit, shows the next room's
// occupants as the roster's anonymous figures: never a name, the room's
// title, description or items. With light enough to see through, the look is
// the ordinary one, names and all.
//
// The fixture (seedDarknessGateRoom) puts Aliceia alone in the Dark Cave at a
// pinned lamp; south is the Town Square, lit at 60, holding Bobrick and the
// Skeleton.
func TestLookExit_HeatShowsFiguresThroughAnExit(t *testing.T) {
	cases := []struct {
		name      string
		lamp      int
		condition int
		want      []string
		wantNot   []string
	}{
		{
			name: "infravision, too dark to see through: two figures by heat",
			lamp: 30, condition: gateInfraConditionId,
			want:    []string{"You peer toward the south.", "but you sense the warmth of:", "a figure</ansi>, <ansi fg=\"combat-anon\">a figure"},
			wantNot: []string{"Bobrick", "Skeleton", "Town Square", "bustling", "too dark to see anything in that direction"},
		},
		{
			name: "normal eyes, too dark to see through: nothing",
			lamp: 30, condition: 0,
			want:    []string{"too dark to see anything in that direction"},
			wantNot: []string{"Bobrick", "Skeleton", "a figure", "warmth"},
		},
		{
			name: "infravision, light enough to see through: the ordinary look",
			lamp: 90, condition: gateInfraConditionId,
			// The test world renders no room template, so the ordinary look
			// is told by its peer line and the room-description block.
			want:    []string{"You peer toward the south.", "room-description"},
			wantNot: []string{"warmth", "too dark", "a figure"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user, room := seedDarknessGateRoom(t, c.lamp)
			if c.condition != 0 {
				require.True(t, user.Character.Conditions.AddCondition(c.condition, true))
			}
			out := runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
			for _, w := range c.want {
				require.Contains(t, out, w)
			}
			for _, w := range c.wantNot {
				require.NotContains(t, out, w)
			}
		})
	}
}

// An empty next room reads as empty to heat, not as too dark.
func TestLookExit_HeatInAnEmptyRoomFindsNothingWarm(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 30)
	require.True(t, user.Character.Conditions.AddCondition(gateInfraConditionId, true))
	out := runGate(t, user, func() (bool, error) { return Look("north", user, room, 0) })
	// The Dark Cave has only a south exit; north is not an exit at all.
	require.NotContains(t, out, "warmth")

	square := rooms.LoadRoom(1)
	require.NotNil(t, square)
	for _, id := range square.GetPlayers(rooms.FindAll) {
		square.RemovePlayer(id)
	}
	for _, id := range square.GetMobs(rooms.FindAll) {
		square.RemoveMob(id)
	}
	out = runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
	require.Contains(t, out, "nothing warm moves there")
	require.NotContains(t, out, "a figure")
}
````

- [ ] **Step 2: Packages and the root guards**

```bash
gofmt -l internal/
go build ./...
go vet ./internal/actions ./internal/messaging ./internal/usercommands ./internal/mobcommands
go test ./internal/actions ./internal/messaging ./internal/usercommands ./internal/mobcommands -count=1
go test . -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: nothing from `gofmt`; four `ok` lines (`internal/actions` takes about a minute); `ok`.

- [ ] **Step 3: Spec check and commit**

```bash
P="internal/actions/look.go internal/actions/scan.go internal/actions/scan_heat_test.go internal/messaging/predicates.go internal/messaging/window.go internal/mobcommands/look.go internal/usercommands/darkness_gates_sight_test.go internal/usercommands/look.go internal/usercommands/look_exit_heat_test.go"
git add $P
git diff --cached a16cc8457 -- $P
git commit -m "feat(look): infravision shows shapes through an exit, never names (#372)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

The diff must print nothing.

---

### Task 7: rooms that promise light, and dense forest

Checkpoint `eb8d826ff`. Model: sonnet. Follow `dogmud-authoring-content` and `dogmud-player-copy` for the YAML and the two rewritten texts (80-column wrap, no numbers).

- [ ] **Step 1: The test first**

Create `lighting_promised_light_rooms_test.go` (block below) and run `go test . -run TestRoomsThatPromiseLightCarryTheApprovedLight -count=1`. Against Task 6's data it fails with one line per room and one for the biome, in map order: `room 317 lamp = <nil>, want 50` and the thirteen other lamps, `room 4127 skylight = <nil>, want 0.25` and the eight other fractions, and `dense_forest movement cost = 1, want 1.4`. Then apply the data blocks.

**Modify `_datafiles/world/dogmud/biomes/dense_forest.yaml`:**

````diff
@@ -7,4 +7,4 @@ skylight: 0.25
 requireditemid: 0
 usesitem: false
 burns: true
-movementcost: 1.0
+movementcost: 1.4 # Close-grown timber and deadfall, like wading snow
````

**Modify `_datafiles/world/dogmud/rooms/ironwind_steppe/3101.yaml`:**

````diff
@@ -10,6 +10,7 @@ description: A broad cave entrance opens in the cliff face, wide
   cool to the touch, and the ceiling curves overhead in a natural
   vault. The passage leads deeper into darkness.
 biome: cave
+skylight: 0.50
 coord:
   x: 13
   y: 7
````

**Modify `_datafiles/world/dogmud/rooms/ironwind_steppe/3102.yaml`:**

````diff
@@ -9,6 +9,7 @@ description: A long, narrow passage where the wind is channeled to
   the wind stings exposed skin. The passage curves gently, and
   the light from outside is reduced to a dim glow.
 biome: cave
+skylight: 0.20
 coord:
   x: 12
   y: 8
````

**Modify `_datafiles/world/dogmud/rooms/ironwind_steppe/3109.yaml`:**

````diff
@@ -10,6 +10,7 @@ description: A damp, warm cave chamber where pale fungi grow in
   and movement. The air is thick with spores and smells of
   damp earth and decay.
 biome: cave
+lamp: 35
 coord:
   x: 13
   y: 10
````

**Modify `_datafiles/world/dogmud/rooms/labyrinth_of_low_tunnels/301.yaml`:**

````diff
@@ -9,6 +9,7 @@ description: The descent levels out into a cramped junction where the tunnel spl
   bodies and rancid fat. This is not a natural cave. Something lives here, and has
   for a long time.
 biome: cave
+lamp: 28
 coord:
   x: -12
   y: 7
````

**Modify `_datafiles/world/dogmud/rooms/labyrinth_of_low_tunnels/310.yaml`:**

````diff
@@ -10,6 +10,7 @@ description: Clusters of pale fungi cover the walls and ceiling of this wider ch
   thick with spores, warm and humid and cloying, and every breath tastes of rot and
   mushroom.
 biome: cave
+lamp: 35
 coord:
   x: -12
   y: 6
````

**Modify `_datafiles/world/dogmud/rooms/labyrinth_of_low_tunnels/314.yaml`:**

````diff
@@ -11,6 +11,7 @@ description: The tunnels open into the largest chamber you have encountered down
   is so thick with smoke, sweat, and the reek of communal habitation that it feels
   like breathing through a damp cloth pressed against your face.
 biome: cave
+lamp: 40
 coord:
   x: -12
   y: 4
````

**Modify `_datafiles/world/dogmud/rooms/labyrinth_of_low_tunnels/317.yaml`:**

````diff
@@ -13,6 +13,7 @@ description: Beyond the barricade, the tunnel opens into a chamber that feels al
   is a palpable presence -- the ceiling so low and the walls so close that the chamber
   feels like the inside of a clenched fist.
 biome: cave
+lamp: 50
 coord:
   x: -12
   y: 2
````

**Modify `_datafiles/world/dogmud/rooms/new_plymouth_old_quarter/6032.yaml`:**

````diff
@@ -21,6 +21,7 @@ description: >
   recording the flood's highest historical reach in a
   thin pale band of mineral deposit.
 biome: dungeon
+lamp: 45
 coord:
   x: -23
   y: 81
````

**Modify `_datafiles/world/dogmud/rooms/new_plymouth_sewers/6403.yaml`:**

````diff
@@ -14,6 +14,7 @@ description: >
   The vault carries on north and south into a darkness the drain-light
   does not negotiate with.
 biome: sewer
+skylight: 0.35
 coord:
   x: -15
   y: 90
````

**Modify `_datafiles/world/dogmud/rooms/new_plymouth_sewers/6404.yaml`:**

````diff
@@ -13,6 +13,7 @@ description: >
   thin ghost of music from some high room. Whatever goes on up there,
   it goes on without reference to you. The only way out is back south.
 biome: sewer
+skylight: 0.10
 coord:
   x: -15
   y: 91
````

**Modify `_datafiles/world/dogmud/rooms/new_plymouth_sewers/6405.yaml`** (one line of context, so the hunk starts below the description's first lines):

````diff
@@ -11,5 +11,4 @@ description: >
   the <ansi fg="itemname">gnawed timber</ansi> of an old shoring post
-  has been chiseled nearly through by generations of teeth, and at
-  the far edge of your lamplight the shadows along the walkway's
-  edge are never quite empty.
+  has been chiseled nearly through by generations of teeth, and the
+  shadows along the walkway's far edge are never quite empty.
 biome: sewer
````

**Modify `_datafiles/world/dogmud/rooms/new_plymouth_sewers/6407.yaml`:**

````diff
@@ -14,6 +14,7 @@ description: >
   very far at once, the way a shore feels from under water. The trunk
   continues north, and a lower passage angles away southwest.
 biome: sewer
+skylight: 0.15
 coord:
   x: -15
   y: 87
````

**Modify `_datafiles/world/dogmud/rooms/new_plymouth_sewers/6411.yaml`:**

````diff
@@ -15,6 +15,7 @@ description: >
   appointment. The trunk continues north; everything south of here
   belongs to the sea twice a day.
 biome: sewer
+skylight: 0.25
 coord:
   x: -17
   y: 83
````

**Modify `_datafiles/world/dogmud/rooms/pothole_coulee/5255.yaml`:**

````diff
@@ -2,18 +2,21 @@ roomid: 5255
 zone: Pothole Coulee
 title: Mine Head
 description: >
-  The adit punches through the talus and the daylight dies behind you
-  between one step and the next -- not fading but cut, the way a door
-  shuts. The tunnel is raw basalt, crack-edged and close, just wide
-  enough to work by, and the footing beneath it is broken stone and
-  wet grit, uneven and treacherous at every step. Old score-marks on
-  the walls show where picks bit at this rock a long time ago; a
-  split timber overhead has been shored up with a wedge of basalt and
+  The adit punches down through the talus, and the daylight follows
+  you only a little way: grey light from the mouth above lies across
+  the first stretch of tunnel and thins to nothing a few steps on.
+  The tunnel is raw basalt, crack-edged and close, just wide enough
+  to work by, and the footing beneath it is broken stone and wet
+  grit, uneven and treacherous at every step. Old score-marks on the
+  walls show where picks bit at this rock a long time ago; a split
+  timber overhead has been shored up with a wedge of basalt and
   holds -- for now. The air has already changed, cool and mineral and
   with the faint of old powder at the back of it. Somewhere deeper
-  in the dark a footfall echoes that is not your own. The world you
-  came from lies back east; the mine goes west.
+  in the dark a footfall echoes that is not your own. The way back
+  to the world you came from climbs up to the mine mouth; the mine
+  goes west.
 biome: cave
+skylight: 0.50
 coord:
   x: 35
   y: -1
@@ -29,11 +32,11 @@ nouns:
     sits level; every step finds a different angle. Moving fast in
     here means falling, and falling means bleeding on rock that does
     not give.
-  dark: The abrupt death of the daylight past the adit mouth -- not
-    a gradual dimming but a hard threshold where the outside simply
-    ends. Past it the eye finds nothing it can use, only shapes
-    suggested by the cold air moving. Whatever lives deeper in has
-    lived in this a long time and can navigate it better than you.
+  dark: Past the grey wash of light from the mouth above, the dark
+    begins -- not a wall but a quick thinning, a few steps where
+    shapes still hold, then nothing the eye can use, only the cold
+    air moving. Whatever lives deeper in has lived in this a long
+    time and can navigate it better than you.
 spawninfo:
 - mobid: 9120
   message: A pale mine crawler pours out of a crack in the wall, too many legs finding purchase.
````

**Modify `_datafiles/world/dogmud/rooms/stillwater/4127.yaml`:**

````diff
@@ -15,6 +15,7 @@ description: >
   deeper darkness, and a smaller side passage opens west,
   the water there shallower.
 biome: cave
+skylight: 0.25
 coord:
   x: -14
   y: 6
````

**Modify `_datafiles/world/dogmud/rooms/test_arena/204.yaml`:**

````diff
@@ -7,6 +7,7 @@ description: A deep, stone-walled pit serves as the proving ground for the arena
   eyes locked on anyone who dares enter. This is no place for the unprepared. The
   training yard lies to the east.
 biome: dungeon
+lamp: 60
 exits:
   east:
     roomid: 201
````

**Modify `_datafiles/world/dogmud/rooms/the_confluence/6200.yaml`:**

````diff
@@ -12,6 +12,7 @@ description: >
   and carries the smell of still water from the west.
   The stair goes back up; the undercroft opens west.
 biome: dungeon
+lamp: 30
 coord:
   x: 10
   y: -78
````

**Modify `_datafiles/world/dogmud/rooms/thornwall_city/488.yaml`:**

````diff
@@ -10,6 +10,7 @@ description: >
   for maintenance access. The passage continues east through a low archway
   and south into a narrowing tunnel.
 biome: cave
+lamp: 30
 coord:
   x: 1
   y: -3
````

**Modify `_datafiles/world/dogmud/rooms/thornwall_city/490.yaml`:**

````diff
@@ -9,6 +9,7 @@ description: >
   someone has climbed up and down here before, but there is no rope now.
   The shaft is a dead end without equipment.
 biome: cave
+skylight: 0.25
 coord:
   x: 3
   y: -3
````

**Modify `_datafiles/world/dogmud/rooms/thornwall_city/493.yaml`:**

````diff
@@ -9,6 +9,7 @@ description: >
   counting hours on watch. A narrow passage opens east into a storage
   alcove, and the main tunnel continues south.
 biome: cave
+lamp: 30
 coord:
   x: 2
   y: -5
````

**Modify `_datafiles/world/dogmud/rooms/thornwall_city/496.yaml`:**

````diff
@@ -9,6 +9,7 @@ description: >
   worn smooth by the passage of heavy loads dragged along it. The tunnel
   continues south toward a faint glow.
 biome: cave
+lamp: 28
 coord:
   x: 1
   y: -6
````

**Modify `_datafiles/world/dogmud/rooms/thornwall_city/497.yaml`:**

````diff
@@ -9,6 +9,7 @@ description: >
   the outer office of whoever runs the operation -- a place for receiving
   reports and issuing orders before visitors are allowed further in.
 biome: cave
+lamp: 60
 coord:
   x: 1
   y: -7
````

**Modify `_datafiles/world/dogmud/rooms/thornwall_city/498.yaml`:**

````diff
@@ -10,6 +10,7 @@ description: >
   sits beneath the table, secured with a brass lock. This is not a hideout
   -- it is an office, run with the efficiency of a legitimate business.
 biome: cave
+lamp: 64
 coord:
   x: 1
   y: -8
````

**Modify `_datafiles/world/dogmud/rooms/thornwall_city/503.yaml`:**

````diff
@@ -12,6 +12,7 @@ description: >
   the chest for itself. A rough stone shelf protrudes from the east wall,
   holding a scatter of small objects barely visible under the fungi.
 biome: cave
+lamp: 45
 coord:
   x: 2
   y: -1
````

**Create `lighting_promised_light_rooms_test.go`:**

````go
package main

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// TestRoomsThatPromiseLightCarryTheApprovedLight pins lighting plan 6's room
// data (spec section 4, owner ruling O7): the 14 room lamps and 9 sky
// fractions the review page approved, and dense_forest's movement cost (O8).
// A typo in a room YAML (a lamp on the wrong room, a fraction off by a digit)
// otherwise passes every other test.
func TestRoomsThatPromiseLightCarryTheApprovedLight(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()

	lamps := map[int]int{
		3109: 35, 310: 35, 503: 45, 488: 30, 317: 50, 314: 40, 6032: 45,
		204: 60, 497: 60, 498: 64, 493: 30, 6200: 30, 301: 28, 496: 28,
	}
	skies := map[int]float64{
		3101: 0.50, 5255: 0.50, 6403: 0.35, 6411: 0.25, 490: 0.25,
		4127: 0.25, 3102: 0.20, 6407: 0.15, 6404: 0.10,
	}
	for id, want := range lamps {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load", id)
		}
		if r.Lamp == nil || *r.Lamp != want {
			t.Errorf("room %d lamp = %v, want %d", id, r.Lamp, want)
		}
		if r.SkyLight != nil {
			t.Errorf("room %d gained a skylight %v; the review approved a lamp only", id, *r.SkyLight)
		}
	}
	for id, want := range skies {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load", id)
		}
		if r.SkyLight == nil || *r.SkyLight != want {
			t.Errorf("room %d skylight = %v, want %v", id, r.SkyLight, want)
		}
		if r.Lamp != nil {
			t.Errorf("room %d gained a lamp %d; the review approved a sky fraction only", id, *r.Lamp)
		}
	}

	b, ok := rooms.GetBiome("dense_forest")
	if !ok {
		t.Fatal("dense_forest biome is not shipped")
	}
	if got := b.GetMovementCost(); got != 1.4 {
		t.Errorf("dense_forest movement cost = %v, want 1.4", got)
	}
}
````

- [ ] **Step 2: The test, then three goldens move**

```bash
go test . -run TestRoomsThatPromiseLightCarryTheApprovedLight -count=1
go test . -run "TestLightingBalanceSpread|TestLightingDayCycleAcrossSampleRounds|TestLightingParityAcrossEveryShippedRoom|TestLightingFixtureDayCycle" -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: `ok`; then `--- FAIL` for `TestLightingBalanceSpread`, `TestLightingDayCycleAcrossSampleRounds` and `TestLightingParityAcrossEveryShippedRoom` (the fixture day-cycle golden holds). Re-record the three:

```bash
go test . -run TestLightingBalanceSpread -update-lighting-balance-spread -count=1
go test . -run TestLightingDayCycleAcrossSampleRounds -update-lighting-daycycle -count=1
go test . -run TestLightingParityAcrossEveryShippedRoom -update-lighting-parity -count=1
git diff --stat -- testdata
```

Expected: the spread golden 552 lines changed each way, the day-cycle golden 276, the parity golden 44. What the dry run read: exactly the 23 relit rooms move and every move is up. In the spread golden 552 entries (23 rooms by 24 samples); in the day-cycle golden 276 (23 rooms by 12 samples); in the parity golden 44 observer lines in the same 23 rooms, 37 from `sight=none` and 7 from `sight=shapes` to 20 `sight=full` and 24 `sight=shapes`.

- [ ] **Step 3: The full suite**

```bash
go test ./... -count=1 2>&1 | grep -E "^(--- FAIL|FAIL|panic)"
```

Expected: nothing (the dry run: 129 packages `ok`).

- [ ] **Step 4: Spec check and commit**

```bash
P="_datafiles/world/dogmud/biomes/dense_forest.yaml _datafiles/world/dogmud/rooms/ironwind_steppe/3101.yaml _datafiles/world/dogmud/rooms/ironwind_steppe/3102.yaml _datafiles/world/dogmud/rooms/ironwind_steppe/3109.yaml _datafiles/world/dogmud/rooms/labyrinth_of_low_tunnels/301.yaml _datafiles/world/dogmud/rooms/labyrinth_of_low_tunnels/310.yaml _datafiles/world/dogmud/rooms/labyrinth_of_low_tunnels/314.yaml _datafiles/world/dogmud/rooms/labyrinth_of_low_tunnels/317.yaml _datafiles/world/dogmud/rooms/new_plymouth_old_quarter/6032.yaml _datafiles/world/dogmud/rooms/new_plymouth_sewers/6403.yaml _datafiles/world/dogmud/rooms/new_plymouth_sewers/6404.yaml _datafiles/world/dogmud/rooms/new_plymouth_sewers/6405.yaml _datafiles/world/dogmud/rooms/new_plymouth_sewers/6407.yaml _datafiles/world/dogmud/rooms/new_plymouth_sewers/6411.yaml _datafiles/world/dogmud/rooms/pothole_coulee/5255.yaml _datafiles/world/dogmud/rooms/stillwater/4127.yaml _datafiles/world/dogmud/rooms/test_arena/204.yaml _datafiles/world/dogmud/rooms/the_confluence/6200.yaml _datafiles/world/dogmud/rooms/thornwall_city/488.yaml _datafiles/world/dogmud/rooms/thornwall_city/490.yaml _datafiles/world/dogmud/rooms/thornwall_city/493.yaml _datafiles/world/dogmud/rooms/thornwall_city/496.yaml _datafiles/world/dogmud/rooms/thornwall_city/497.yaml _datafiles/world/dogmud/rooms/thornwall_city/498.yaml _datafiles/world/dogmud/rooms/thornwall_city/503.yaml lighting_promised_light_rooms_test.go testdata/lighting_balance_spread.golden testdata/lighting_daycycle.golden testdata/lighting_parity.golden"
git add $P
git diff --cached eb8d826ff -- $P
git commit -m "feat(world): rooms that promise light get it, dense forest costs 1.4 (#372)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

The diff must print nothing.

---

### Task 8: docs and patch notes

Checkpoint `8d7fa2267`. Model: sonnet. Every symbol a block names exists by Task 7 (the `codegraph_search` or `Select-String` check in `CLAUDE.md`).

**Modify `docs/PATCH_NOTES.md`:**

````diff
@@ -1,5 +1,33 @@
 # DOGMud Patch Notes
 
+## 2026-10-07: Light, balanced
+
+- City street lamps are now lit when the light fails and put out once the
+  day is bright, like a lamplighter working by eye. They burn all night,
+  and on a dim winter morning or evening they stay lit, so faces can still
+  be read in the street. A storm does not light them. Every street lamp,
+  and the lantern on Stillwater's North Gate, changes at the same moment.
+  On a bright day a street is lit by the sky alone, so a main street at
+  noon no longer dazzles. At night the streets are as bright as before.
+- Seeing into the next room needs a little less light. A lamplit street at
+  night is still not enough, but a candle or anything brighter lets you
+  look and scan through an exit.
+- Heat sight now works through an exit. When it is too dark to see that
+  way, you still sense the warmth of whoever stands in the next room, as
+  figures, never by name.
+- No place is darker than a sealed cave unless magic makes it so. A cell
+  with a window slit at night is now barely lit, not blacker than stone.
+  Faint light at dusk, under thick trees or in a swamp reads a little
+  brighter than before.
+- Rooms whose descriptions speak of light now have it: the glowing fungi
+  and fires of the low tunnels, the oil lamps of the Thornwall
+  underground, the Champion's Pit, a drowned court's doorway lamps and
+  more. Cave mouths, mine entrances and sewer grates let in daylight.
+- Moving through dense forest now costs as much effort as wading through
+  snow.
+- Two weak lights, or two weak sources of heat sight, now add up to a
+  little less than before. Strong ones add up as they always did.
+
 ## 2026-10-06: Clearer in the dark
 
 - When the room is too dark for you, `look` now says so: "It is too dark
````

**Modify `docs/schemas/behavior.md`:**

````diff
@@ -98,7 +98,7 @@ node.
 
 | Condition | Params | Description |
 |-----------|--------|-------------|
-| `time_of_day` | `period` ("day", "night", or "after_dusk" with `hours` N), or `range` ("`<start>-<end>`") | Checks in-game time of day. `after_dusk` is true from the night boundary until N game hours later (lighting 5e). |
+| `time_of_day` | `period` ("day", "night", "lamplit", or "after_dusk" with `hours` N), or `range` ("`<start>-<end>`") | Checks in-game time of day. `after_dusk` is true from the night boundary until N game hours later (lighting 5e). `lamplit` is true while the city's street lamps burn: at night, and while the clear sky is too dim to read a face (lighting plan 6); a lantern on it changes with every street lamp. |
 | `round_mod` | `n` (int) | Succeeds when current round % n == 0. |
 | `random_chance` | `percent` (int) | Succeeds with N% probability. |
 | `players_in_room` | none | At least one player is in the mob's room. |
````

**Modify `internal/actions/context.md`:**

````diff
@@ -488,7 +488,15 @@ perceive can be named), then a sealed crate or a known room container
 (`lookNamesAnObject`, `:116`), then an exit (direction alias resolved, then
 through-sight, then lock). `LookKind` (`:12`) is `LookBlind`, `LookTooDark`,
 `LookRoom`, `LookCreature`, `LookExit`, `LookExitTooDark`, `LookExitLocked`,
-`LookOther`. Sight `SightNone` splits by cause (#364): `LookBlind` for a
+`LookExitShapes`, `LookOther`. `LookExitShapes` (lighting plan 6, owner
+ruling O6) is an exit the light here is too poor to see through
+(`messaging.SeesThroughExit` false) whose next room heat still reaches
+(`messaging.SensesHeatThroughExit`); a locked exit under heat is still
+`LookExitLocked`. The player wrapper prints one `messaging.UnseenFigure`
+per occupant from `FiguresSensedIn(viewer, room, selfUserId)` (`scan.go`),
+never a name or the room's description; the mob wrapper is silent on it,
+as on every refusal. `Scan` takes the same heat path per exit, listing
+figures where the light test fails. Sight `SightNone` splits by cause (#364): `LookBlind` for a
 looker whose `Perception` is `Blinded` (the same check `ParticipantSight`
 answers `SightNone` on first), `LookTooDark` for one the room is too dark
 for, so a caller can say that light would help.
````

**Modify `internal/behaviortree/context.md`:**

````diff
@@ -251,7 +251,7 @@ Condition nodes use `type: condition` with `check: <name>`.
 
 | Condition | Params | Description |
 |-----------|--------|-------------|
-| `time_of_day` | `period` ("day", "night", or "after_dusk" with `hours` N) OR `range` ("`<start>-<end>`", 24h format, e.g., `"9-17"`; wraps midnight when start > end). When both set, `range` takes precedence. | In-game time of day. Range uses `[start, end)` semantics (inclusive start, exclusive end). Empty range (`"5-5"`) always Failure; full-day range (`"0-24"`) always Success; both log a warning once. `after_dusk` (lighting 5e) is true from the unrounded night boundary until N game hours later (`gametime.GameDate.HoursAfterDusk`); a missing or non-positive `hours` logs once and fails. Malformed ranges log an error once and return Failure. |
+| `time_of_day` | `period` ("day", "night", "lamplit", or "after_dusk" with `hours` N) OR `range` ("`<start>-<end>`", 24h format, e.g., `"9-17"`; wraps midnight when start > end). When both set, `range` takes precedence. | In-game time of day. Range uses `[start, end)` semantics (inclusive start, exclusive end). Empty range (`"5-5"`) always Failure; full-day range (`"0-24"`) always Success; both log a warning once. `after_dusk` (lighting 5e) is true from the unrounded night boundary until N game hours later (`gametime.GameDate.HoursAfterDusk`); a missing or non-positive `hours` logs once and fails. `lamplit` (lighting plan 6) is `gametime.LampsLit()`, the test the biome street lamps (`rooms.BiomeInfo.StreetLamp`) read: night, or the clear-sky celestial light below `LightDimBelow`; the arch lantern's `dusk_to_dawn` tree uses it so the lantern changes in the same round as the street lamps. An unknown `period` is not validated at load; it simply fails. Malformed ranges log an error once and return Failure. |
 | `round_mod` | `n` (int) | `round % n == 0`. |
 | `random_chance` | `percent` (int) | N% probability. |
 | `players_in_room` | none | At least one player in the room, and the mob can make out the room (`mobCanSee`, below). |
````

**Modify `internal/configs/context.md`:**

````diff
@@ -748,20 +748,23 @@ See the live config and validation code for tuning values.
 
 ### Graded room lighting (plans 1, 2, 3a, 5a, 5b and 5c of the graded lighting arc)
 
-Thirteen knobs, validated in their own file (`config.balance.lighting.go`)
+Fourteen knobs, validated in their own file (`config.balance.lighting.go`)
 rather than folded into `validateMisc`, because the arc kept adding more
 here across plans: plan 1 shipped the three band thresholds, plan 2 added
 the vision-strength fallback, plan 3a added the eight knobs that turn the
-sky itself into a solar and lunar model, and plan 5b added the dazzle
-edge. Twelve of the thirteen are absent from `_datafiles/config.yaml`, so
-the shipped value is the Go default in every case; `LightDazzleAbove` is
-the exception, shipped in the file at its default of 75.
+sky itself into a solar and lunar model, plan 5b added the dazzle edge, and
+plan 6 added the real-light floor. Since lighting plan 6 every one of them
+ships in `_datafiles/config.yaml` (before it, twelve ran unseen at their Go
+defaults); the root guard `lighting_config_surface_test.go` fails if a key
+goes missing. Every shipped value is the Go default except
+`LightExitsAbove`, shipped at 55 (owner ruling O5).
 
 | Knob | Type | Default | Effect |
 |------|------|---------|--------|
 | `LightBlindBelow` | ConfigInt | 25 | Below this, a normal observer is blind. |
 | `LightDimBelow` | ConfigInt | 50 | Below this, a normal observer reads shapes only. |
-| `LightExitsAbove` | ConfigInt | 65 | At or above this, exits into adjacent rooms are visible. |
+| `LightExitsAbove` | ConfigInt | 65 | At or above this, exits into adjacent rooms are visible. Ships at 55. |
+| `LightRealMinimum` | ConfigInt | 3 | The least a room with any real light reads before darkness is subtracted (`rooms.composeWithFixtures`). Zero is unset and takes 3; clamped below `LightBlindBelow`. Plan 6. |
 | `LightDazzleAbove` | ConfigInt | 75 | Where the comfortable band ends and too-bright begins for a normal observer; a vision ability moves it down by its strength. Plan 5b. |
 | `LightDefaultVisionStrength` | ConfigInt | 12 | Window shift (`internal/messaging.SightThroughWindow`'s `strength`) for a vision flag that declares no strength of its own. Plan 2. |
 | `LightDoublingStep` | ConfigFloat | 8 | Scale points per doubling of physical light; the one constant `internal/lightscale.Combine`/`Attenuate` take as `step`. Plan 3a. |
````

**Modify `internal/gametime/context.md`:**

````diff
@@ -132,19 +132,40 @@ and 15h37m at midwinter.
 
 ```go
 func NightHoursAt(latitudeDegrees float64, dayOfYear int) float64
+func NightAt(latitudeDegrees float64, dayOfYear int, hour float64) bool
 func SunLight(cfg configs.Lighting, dayOfYear int, hour float64) float64
 func MoonLight(cfg configs.Lighting, swiftmoon, wanderer, eye float64) float64
 func CelestialLight() float64
+func LampsLitAt(night bool, celestial float64, dimBelow int) bool
+func LampsLit() bool
 ```
 
 - `NightHoursAt` is how `GameDate.ReCalculate` places the day/night boundary,
   replacing the old flat `Timing.NightHours` cutoff. It clamps to 12 (polar
   day) or 0 (polar night) rather than returning NaN beyond the polar circles.
+- `NightAt` (lighting plan 6) is that boundary as a predicate: night is
+  centred on midnight and `NightHoursAt` long. `GameDate.Night` (and so
+  `IsNight`) is computed through it, so a caller holding a day and an hour
+  (the light goldens, the street lamp's tests) asks the same question
+  without moving the round counter.
+- `LampsLitAt` (lighting plan 6, owner ruling O4 as amended) is the street
+  lamps' rule, pure: lit while `night` OR the clear-sky `celestial` reads
+  below `dimBelow` (the faces edge, `LightDimBelow`), a lamplighter working
+  by eye, so a midwinter 08:00 (day, sky about 40) keeps its lamps. Absent
+  celestial is below any edge; NaN leaves it to `night`. `LampsLit` is it
+  on the current round: `IsNight()`, `CelestialLight()` and
+  `configs.GetLightingConfig().DimBelow`. 🔑 Its input is the clear sky,
+  before any sky fraction or weather, so a storm lights no lamp and every
+  street lamp in the world changes in the same round. Two readers share it:
+  the biome street lamp (`rooms.BiomeInfo.StreetLamp` through
+  `Room.LightLevel`) and the behaviour condition `time_of_day period:
+  lamplit` (the North Gate arch lantern's `dusk_to_dawn` tree).
 - `SunLight` is the sun's own contribution to the sky, calibrated so an
   equinox noon reads exactly `cfg.EquinoxNoon`. 🔑 **The sun is Absent below
   the horizon, not zero.** `sin(altitude) <= 0` returns `lightscale.Absent()`
-  directly, so night needs no separate branch: Absent already composes
-  correctly through `lightscale.Combine`.
+  directly, so night needs no separate branch: since lighting plan 6
+  `lightscale.Combine` reads Absent as a term of 0, which adds nothing.
+  The night-trade guard still reads `SunLight` Absent as "the sun is down".
 - `MoonLight` is the three moons' combined contribution (each moon's phase
   from `PhasesAtRound`/`GetAllPhases`, below), interpolated between a
   starlight anchor and a full-moon anchor on a logarithmic intensity axis.
@@ -158,9 +179,10 @@ func CelestialLight() float64
 
 - **A bare `Balance{}` has `WorldLatitude` zero, and validation COERCES that
   to 46.5** (`internal/configs/config.balance.lighting.go`) rather than
-  honouring it. Right for production, where a bare struct never ships and
-  none of the lighting knobs appear in `_datafiles/config.yaml` today so
-  production genuinely runs one. A test that wants a specific latitude must
+  honouring it. Right for production: until lighting plan 6 surfaced the
+  lighting knobs in `_datafiles/config.yaml`, production genuinely ran a bare
+  latitude, and any config file that omits the key still does. A test that
+  wants a specific latitude must
   set `Balance.WorldLatitude` and call `Validate()` explicitly.
 - **A test binary's `Timing` defaults are not the shipped ones.** Any test
   that touches night must pin BOTH `Timing` (`RoundsPerDay`, `NightHours`,
````

**Modify `internal/lightnotice/context.md`:**

````diff
@@ -59,6 +59,11 @@ either state; see "The seams" below.
    5d: `Darkened` flipped or `Dark` moved, a carried darkness arriving,
    lapsing or changing strength), then **carried**, then the **room's own
    lamp**, then **weather** filtering the sky, then the **sky** itself.
+   The lamp is named only when it moved the same way as the room
+   (`lampAgrees`, lighting plan 6): street lamps now light when the clear
+   sky grows too dim to read a face and go out once it is bright again
+   (`gametime.LampsLit`), so across a long gap (noon to midnight) the lamp
+   can come on while the room darkens, and that change belongs to the sky.
    Darkness comes first because without its own cause a darkness moves no
    light term and fell to `CauseEyes`. Its lines live in
    `narration/light-notices/darkness.yaml`, all six transitions, like every
@@ -218,7 +223,7 @@ next check after reconnect is a fresh `TriggerQuiet` login record.
 lantern its schedule dims, a sunstone fading, a second light joining one
 already here; all of these used to fall to `eyes`), and `lamp` when the
 room's lamp OR its light fixtures move (`LightTerms.Fixture`: the North
-Gate's arch lantern at dusk and dawn). A darkness fixture moves `Dark` and
+Gate's arch lantern as it is lit and snuffed with the street lamps). A darkness fixture moves `Dark` and
 reads `darkness`, checked first. Cadence is unchanged: an idle player learns
 of dusk on their next command, move or combat round. `lamp.yaml`'s header
 names fixtures; `carried.yaml`'s two darker "is gone" lines read "fades"
````

**Modify `internal/lightscale/context.md`:**

````diff
@@ -4,45 +4,68 @@ The arithmetic of the graded light scale. Pure: no config, no globals, no locks.
 
 ## What it is for
 
-The scale runs -100 to 100 and is perceptual, not linear. Zero is the darkest
-naturally occurring light (an unlit cave). Negative is magical darkness.
+The scale runs -100 to 100 and is perceptual. Zero is the darkest a place can
+be without active magical darkness (an unlit cave). Negative is magical
+darkness and nothing else (lighting plan 6, owner ruling O1).
 
 One constant relates the scale to physical light: the **doubling step**, how
 many points twice as much light is worth. It is `LightDoublingStep` in config,
 shipped at 8, and it is passed in rather than read here.
 
+## The arithmetic (lighting plan 6)
+
+Every operation works on **linear brightness**, not on points:
+
+- `B(p) = 2^(p/step) - 1` for a light term, so `B(0) = 0`; a term at or below
+  0, NaN, +Inf or `Absent()` reads `B = 0`.
+- `p(B) = step * log2(1 + B)`, the inverse.
+- `Combine` is `p(sum of B)`; `Attenuate` is `p(fraction * B)`.
+
+So no sum or fraction of real light can read below 0, a single term reads
+itself, and two equal terms read a little under one step brighter (52 and 52
+read 59.9). Well above the dim end the result matches the old log-domain
+operators to within half a point; the gap shrinks as `2^(-p/step)` and is
+under 0.5 for readings at or above about 37 at step 8 (not 25: at 25 two equal
+terms read about 1.4 lower than before).
+
 ## Surface
 
 | Symbol | Purpose |
 |---|---|
-| `Absent() float64` | A term that is not present at all, distinct from a dark term |
-| `Combine(step float64, terms ...float64) float64` | Every present term together |
-| `Attenuate(step, light, fraction float64) float64` | A transmission fraction applied to one term |
-| `Trim(step, others, max, target float64) float64` | The one solve: the output an adjustable source runs at so the combine of `others` and itself lands on `target` (`trim.go`) |
+| `Absent() float64` | A marker (`-Inf`) for a term that is not there: a sun below the horizon, a trimmed source switched off, an unlit fixture. Reads as `B = 0`; `Combine` and `Attenuate` never return it |
+| `Combine(step float64, terms ...float64) float64` | Every term together, on the linear sum; 0 for none |
+| `Attenuate(step, light, fraction float64) float64` | A transmission fraction applied to one term; never below 0; 0 for a fraction at or below 0 |
+| `Trim(step, others, max, target float64) float64` | The one solve: the output an adjustable source runs at so `B(target) = B(others) + B(out)`, capped at `max` (`trim.go`) |
 | `TrimDarkness(step, light, otherDark, max, floor float64) float64` | A darkness's trim (lighting plan 5d, ruling D2): `Trim(step, otherDark, max, light - floor)`, Absent light read as 0, Absent when the budget is 0 or less (`trim.go`) |
 
 ## Traps
 
-- **Absent is not zero.** A cave has no sky; a sky contributing zero would make
-  the cave brighter, because two terms at zero combine to one step above zero.
-  `Attenuate` with fraction 0 returns `Absent()` for this reason.
-- **A multiplier is a subtraction here.** Half the light is minus one step.
-- `Combine` skips NaN as well as -Inf, so one bad caller cannot poison a room.
-- Both functions coerce a non-positive step to 1 rather than dividing by zero.
-  `Trim` does the same.
-- **`Trim` solves the combine exactly; it is not `target - others`.** It
-  returns `Absent()` when the combine already reaches `target` without the
-  source, or when the needed term would fall below 0. A NaN `target` or `max`
-  returns `Absent()`.
-- **Darkness uses the same solve (lighting plan 5d).** Darknesses combine among
-  themselves by the halving rule and the result is subtracted from the light,
-  so keeping the room at or above `floor` is `Combine(otherDark, d) <= light -
-  floor`: `Trim` with that budget as its target. The old linear `Darkens`
-  branch and the `Polarity` type are DELETED; one solve serves both.
+- **No sky and no light are the same statement now.** The old `-Inf`
+  sentinel existed because two log-domain zeros combined one step brighter
+  than one; `B(0) + B(0)` is still 0. `Absent()` survives only as the "off" /
+  "not there" marker that `conditions.SetLightOutput` and `internal/itemlight`
+  read; arithmetic treats it as 0.
+- **A multiplier is not a fixed subtraction any more.** Half the light is very
+  nearly one step down by day and less near 0, where there is little light
+  left to take (a night storm takes 3 to 8 points, not 8).
+- `Combine` skips NaN and reads negative light terms as 0, so one bad caller
+  cannot poison a room and a light term can never darken one.
+- Every function coerces a non-positive step to 1 rather than dividing by zero.
+- **`Trim` solves the linear sum exactly; it is not `target - others`.** It
+  returns `Absent()` when `others` already reaches `target`, or when the
+  needed term has no brightness. With no other light it returns
+  `min(target, max)`. A NaN `target` or `max` returns `Absent()`.
+- **Darkness uses the same solve (lighting plan 5d).** Darknesses combine
+  among themselves on the same sum and the room subtracts the result in
+  points, so keeping the room at or above `floor` is
+  `Combine(otherDark, d) <= light - floor`: `Trim` with that budget as its
+  target.
+- **The floor is not here.** `LightRealMinimum` (any real light reads at
+  least 3) is the room's rule, applied in `rooms.composeWithFixtures`.
 
 ## Who uses it
 
-`internal/gametime` (sun plus moons), `internal/rooms` (ambient plus lamp plus
-carried light, minus carried darkness; `Trim` and `TrimDarkness` from
-`light_trim.go`). Plan 4 added weather occlusion; plan 5d added darkness on
-the same functions.
+`internal/gametime` (sun plus moons), `internal/rooms` (sky, lamp, fixtures
+and carried light, minus darkness; `Trim` and `TrimDarkness` from
+`light_trim.go`), `internal/characters` (infra reach sources combine through
+`Combine`), `internal/behaviortree` (`Absent` as an item light's "off").
````

**Modify `internal/messaging/context.md`:**

````diff
@@ -155,11 +155,23 @@ Functions:
   lighting plan 5c): whether an observer sees THROUGH an exit.
   `exitsAbove` (`LightExitsAbove`) is the normal-eyes edge; night-vision
   strength moves it down exactly as it moves the blind and dim edges. Infra
-  reach plays no part.
+  reach plays no part in this light test; heat has its own path,
+  `SensesHeatThroughExit`. Shipped edge: 55 (lighting plan 6, owner ruling
+  O5; the Go default stays 65).
 - `SeesThroughExit(observer, room) bool` (`predicates.go`, lighting plan
   5c): `ParticipantSight` is not `SightNone` AND `ExitThroughWindow` at
-  the room's light and the observer's strength. `look <direction>` gates on
-  it; it replaced a nightvision-FLAG waiver.
+  the room's light and the observer's strength. `look <direction>` and
+  `scan` gate on it; it replaced a nightvision-FLAG waiver.
+- `SensesHeatThroughExit(observer, here, next RoomVisibility) bool`
+  (`predicates.go`, lighting plan 6, owner ruling O6): infravision through an
+  exit. True when the observer has infra reach, some sight `here` (not
+  `SightNone`, so a blinded observer senses nothing) and the `next` room's
+  light is at or above minus the reach. A caller asks it only after
+  `SeesThroughExit` refused, so it never upgrades a view the light grants,
+  and renders shapes only: the roster's `UnseenFigure(SightShapes)` per
+  occupant (`actions.FiguresSensedIn`), never a name, the room's title,
+  description or items. Every occupant counts as warm: the codebase has no
+  cold-body concept, and own-room infravision shows everyone too.
 - `FixedLight` (`predicates.go`, lighting plan 5c): an `int` that
   satisfies `RoomVisibility`. A caller judging many observers in one room
   reads `room.LightLevel()` once and passes `FixedLight`, as
@@ -501,7 +513,7 @@ The package is the pipeline, one stage per file, plus the fan-out (`trio.go`):
 | `hidenames.go` | `HideNames`, `NameHider`, `HideSpeakerNames` (sight gates slice 5b): replacing specific names in bare prose, longest-first, whole-word |
 | `hidenames_tagged.go` | Identity-tag-aware name replacement `HideNames` and `Anonymize` share, including the trailing adjective span |
 | `wrap.go` | `WrapAnsi`, ANSI-aware folding at a caller-supplied width measured in visible runes; called by the pipeline for the categories `shouldWrap` admits, and directly by `motd.go` for its box-bordered banner |
-| `predicates.go` | `ParticipantSight` (the optics primitive) plus `CanSeeClearly`/`CanSeeShapes`/`CanSeeSightImpairedOnly`, the one-line attention policies built on it; `SeesThroughExit` and `FixedLight` (lighting plan 5c) |
+| `predicates.go` | `ParticipantSight` (the optics primitive) plus `CanSeeClearly`/`CanSeeShapes`/`CanSeeSightImpairedOnly`, the one-line attention policies built on it; `SeesThroughExit` and `FixedLight` (lighting plan 5c); `SensesHeatThroughExit` (lighting plan 6) |
 | `window.go` | `SightThroughWindow`, the pure window-model function `ParticipantSight` calls, `clampShift`, `ExitThroughWindow` (lighting plan 5c), `LightTrimTarget` (lighting plan 5a, reparameterized in 5b to take `dazzleAbove` instead of reading the now-retired `windowDazzleEdge` constant), `DarknessTrimTarget` (lighting plan 5d), plus its two remaining unexported constants (`windowShiftCap`, `windowFloor`) |
 | `band.go` | `Band`, `BandThroughWindow`, `LightBand` (lighting plan 3d): the band-grained twin of `SightDecision`/`SightThroughWindow`/`ParticipantSight`, adding the dazzled tier for `internal/lightnotice` |
 | `comfort.go` | `ComfortDistance` (lighting plan 5b): how far a room's light sits outside the observer's comfortable band, as dark/bright fractions of the way to the cap; `infraDarkCap` (lighting plan 5c), the unexported cap it applies to the dark fraction for an observer with infra reach |
````

**Modify `internal/rooms/context.md`:**

````diff
@@ -104,34 +104,67 @@ The `internal/rooms` package is the core world management system for GoMud, hand
 - **Item requirements**: Biomes that require specific items to navigate safely
 - **Dynamic loading**: File-based biome definitions with validation
 
-### Room Lighting (`lighting.go`, `light_trim.go`, graded scale, plans 1 through 5a of the lighting arc)
+### Room Lighting (`lighting.go`, `light_trim.go`, graded scale, plans 1 through 6 of the lighting arc)
 
 `Room.LightLevel() int` is the light accessor every consumer reads. It
 reports light on a continuous -100 to 100 scale, composed from three kinds
-of term on one logarithmic operator (`internal/lightscale.Combine`):
+of term on one operator, the sum of linear brightness
+(`internal/lightscale.Combine`, rebuilt by lighting plan 6 so no sum or
+fraction of real light can read below 0):
 
 1. **The sky**: `internal/gametime.CelestialLight()` (sun plus moons, one
    value for the whole world per round), attenuated by this room's sky
    fraction and then by `mutatorSkyFilter()`, the product of every active
    mutator's `SkyLight` fraction (1 when clear; weather multiplies it down).
-2. **The room's own lamp**, if it has one.
+   A room with no sky gets a term of 0, which adds nothing.
+2. **The room's own lamp**, if it has one, as it burns at this hour
+   (`lampValue(lampsLit)`). A room's `lamp:` override burns at all hours. A
+   biome lamp burns at all hours unless the biome sets `streetlamp: true`
+   (`BiomeInfo.StreetLamp`, `BiomeInfo.LampAt(lampsLit)`; lighting plan 6,
+   owner ruling O4 as amended): then it joins only while
+   `gametime.LampsLit()`, which is night OR the clear-sky
+   `CelestialLight()` below `LightDimBelow`, a lamplighter working by eye,
+   so a midwinter 08:00 (day, sky about 40) keeps its lamps. Weather never
+   enters it, so every street lamp lights and goes out in the same round,
+   and the North Gate arch lantern's `dusk_to_dawn` tree reads the same
+   test (`time_of_day period: lamplit`). `city_thoroughfare` (52) and
+   `city_backstreet` (35) set it; `interior`, `ether` and `spiderweb` do
+   not. Once the clear sky shows faces a street reads its daylight alone.
 3. **Every light anyone present carries, one term each** (lighting plan 5a).
    `carriedTerms(exclude)` makes one pass over `r.mobs` and `r.players`,
    reading each bearer's `Conditions.LightAndDarknessSources()` through
    `Condition.LightNow`, so a hooded or trimmed-off source adds nothing and
-   two torches are one doubling step brighter than one. Before 5a any light
-   lifted the room to a flat `DimBelow`; the `FindHasLight` find flag that
-   test used is DELETED.
-4. **Every darkness anyone present carries** (lighting plan 5d), from the
-   same pass. Darknesses combine among themselves by the same halving rule
-   (two of 50 take 58) and the combined darkness is SUBTRACTED from the
-   combined light (Absent light reads 0), so a room can read below 0, down
-   to the -100 clamp. An unlit room with no darkness is still 0.
+   two torches are a little under one doubling step brighter than one.
+   Before 5a any light lifted the room to a flat `DimBelow`; the
+   `FindHasLight` find flag that test used is DELETED.
+
+**The floor (lighting plan 6, owner ruling O3).** The combined light of any
+real light (above 0) reads at least `LightRealMinimum` (3) before darkness
+is subtracted, so a sliver of starlight through a crack never rounds to the
+same 0 as a sealed cave. `LightTerms.Light` carries the floored value. A room
+with no light at all reads exactly 0; the old `-Inf` branch that turned "no
+light" into 0 is gone, because a combine of nothing already reads 0.
 
-The composition is layered so a trim can leave one source out: `composeLight`
-calls `composeLightExcluding(cfg, celestial, skyFilter, exclude)`, which calls
-`composeWith(cfg, celestial, skyFilter, carried, dark)` (the pure core a test
-can feed carried light and darkness terms without users or mobs).
+4. **Every darkness anyone present carries** (lighting plan 5d), from the
+   same pass. Darknesses combine among themselves on the same sum (two of 50
+   take 58) and the combined darkness is SUBTRACTED from the combined light
+   in points. That subtraction is the only way a room reads below 0 (owner
+   ruling O1), down to the -100 clamp. The root guard
+   `lighting_no_natural_negative_test.go` holds every shipped room at 0 or
+   above at every sampled hour, season, moon and weather with no darkness
+   present.
+
+The composition is layered so a trim can leave one source out, and both
+clock reads (the celestial light and `gametime.LampsLit()`) are injected so a
+test needs no global clock: `composeLight(cfg, celestial, lampsLit,
+skyFilter)` calls `composeLightExcluding(cfg, celestial, lampsLit, skyFilter,
+exclude)`, which calls `composeWithFixtures(cfg, celestial, lampsLit,
+skyFilter, carried, dark, fixtureLight, fixtureDark)`. `composeWith(cfg,
+celestial, skyFilter, carried, dark)` is the fixture-free test core and
+composes with every street lamp lit. `LightTermsAtForTest(celestial,
+lampsLit, skyFilter)` (`test_helpers.go`) exposes the composition to a
+cross-package golden, whose caller works out `lampsLit` with
+`gametime.LampsLitAt(night, celestial, cfg.DimBelow)` on its own sample.
 
 **Trimming (`light_trim.go`, plan 5a; darkness plan 5d).** `(*Room).TrimLightFor(c)`
 trims every adjustable, unhooded light AND darkness record `c` holds to `c`'s
@@ -169,15 +202,17 @@ onto itself.
 same light broken into the terms `LightLevel` combines, for a caller that
 needs to know WHY the light is what it is: `Level` (identical to
 `LightLevel()`, both come from the shared `composeLight`), `Sky` (the sky
-term after fraction and the weather filter, `lightscale.Absent()` when the
-room has no sky), `SkyFilter` (the product of the active mutators'
-`SkyLight` fractions, 1 when clear), `Lamp`/`HasLamp`, `Carried`, and (plan
-5a) `Raw`. Since lighting plan 5d `Raw` is the NET light `Level` rounds
-(`Light` read as 0 when Absent, minus `Dark`), and three fields carry the
-parts: `Light` (the combined light, `Absent` when nothing lights the room;
-the light trim solves against it), `Dark` (the combined darkness, `Absent`
-when nobody carries one) and `Darkened` (someone carries a darkness, as
-`Carried` means someone carries a light).
+term after fraction and the weather filter, 0 when the room has no sky),
+`SkyFilter` (the product of the active mutators' `SkyLight` fractions, 1
+when clear), `Lamp`/`HasLamp` (the lamp as it burns now: a street lamp is
+`HasLamp` false while `gametime.LampsLit()` is false, a day whose clear sky
+shows faces), `Carried`, and (plan 5a) `Raw`. Since lighting plan
+5d `Raw` is the NET light `Level` rounds (`Light` minus `Dark`), and three
+fields carry the parts: `Light` (the combined light after the plan 6 floor,
+0 when nothing lights the room; both trims solve against it), `Dark` (the
+combined darkness, 0 when nobody carries one) and `Darkened` (someone
+carries a darkness, as `Carried` means someone carries a light). Since plan
+6 no `LightTerms` field is ever `-Inf`.
 `internal/lightnotice` is the one consumer: it compares two `LightTerms`
 snapshots to name which term moved and so which cause to report for a band
 change. `LightTerms` and `LightLevel` share one computation
@@ -200,9 +235,10 @@ YAML keys are `skylight` and `lamp`:
   this override in most of the cases that once seemed to need it: a brick
   sewer vault, a wrecked ship's interior and a web-choked lair each got
   their own biome (`sewer`, `interior`, `spiderweb`) rather than a
-  room-level number. **The `lamp` override ships on twenty-seven rooms**
+  room-level number. **The `lamp` override ships on forty-one rooms**
   (grep `^lamp:` under `_datafiles/world/dogmud/rooms`): the eleven shop
-  rooms of the merchants slice described below, and sixteen older ones.
+  rooms of the merchants slice described below, the fourteen "rooms that
+  promise light" of lighting plan 6 (below), and sixteen older ones.
   The sixteen older rooms are the three-room Planar
   Oasis (`instance_planar_oasis/500{3,4,5}.yaml`), which sets `lamp: 38`
   against `ether`'s biome lamp of `60` because its room text reads "Shapes
@@ -231,8 +267,18 @@ YAML keys are `skylight` and `lamp`:
   all open to the sky by day; the lamp on 5478 and 5480 only matters after
   dark.
 
-  **The `skylight` override ships on thirteen rooms** (grep `^skylight:`
-  under `_datafiles/world/dogmud/rooms`). The first two, new in plan 3c-2,
+  **Rooms that promise light (lighting plan 6, owner ruling O7).** Rooms
+  that are structurally dark but whose text names a light got one, as the
+  owner's review page approved: fourteen room lamps at all hours (3109 35,
+  310 35, 503 45, 488 30, 317 50, 314 40, 6032 45, 204 60, 497 60, 498 64,
+  493 30, 6200 30, 301 28, 496 28) and nine sky fractions for openings that
+  let daylight in (3101 0.50, 5255 0.50, 6403 0.35, 6411 0.25, 490 0.25,
+  4127 0.25, 3102 0.20, 6407 0.15, 6404 0.10). The root test
+  `lighting_promised_light_rooms_test.go` pins every value.
+
+  **The `skylight` override ships on twenty-two rooms** (grep `^skylight:`
+  under `_datafiles/world/dogmud/rooms`): the nine of plan 6 above and the
+  thirteen before it. The first two, new in plan 3c-2,
   are the holding cells beneath Thornwall's guard barracks
   (`thornwall_city/5105.yaml`) and Stillwater's constabulary
   (`stillwater/5106.yaml`), both `dungeon` biome (`skylight: 0.0` by
@@ -281,8 +327,8 @@ carried light or the room adds).
 | `river` | `1.0` | none | Flowing water, fully open sky |
 | `ether` | `0.0` | `60` | Outside the world; time-invariant. Character creation, the shadow realm, the planar oasis |
 | `spiderweb` | `0.0` | `45` | A web-choked lair. Declared before plan 3b but held zero rooms until this plan gave it the Foldweave |
-| `city_thoroughfare` | `0.95` | `52` | A city's main streets, squares, markets and gates. Lamps hold it in the faces band at any hour, day or night. Added by plan 3c-1 |
-| `city_backstreet` | `0.95` | `35` | A city's lanes, alleys, courts and yards off the main ways. No lamp reaches them, so a normal eye reads shapes, not faces, after dark. Added by plan 3c-1 |
+| `city_thoroughfare` | `0.95` | `52` | A city's main streets, squares, markets and gates. Street lamps (`streetlamp: true`, lit at night and while the clear sky is too dim for faces) hold it in the faces band at any hour, day or night. Added by plan 3c-1 |
+| `city_backstreet` | `0.95` | `35` | A city's lanes, alleys, courts and yards off the main ways. No main-street lamp reaches them; their own dim street lamps (`streetlamp: true`, lit with the main streets') leave a normal eye reading shapes, not faces, after dark. Added by plan 3c-1 |
 | `ruins` | `0.75` | none | A roofless building: takes weather and sky like open ground, a little shaded by whatever walls still stand, dark at night. `movementcost: 1.0` for the rubble underfoot. Added by plan 3c-1 |
 
 **`city` is gone from the dogmud world.** Plan 3c-1 split New Plymouth's
````

- [ ] **Step 1: Checks and commit**

```bash
python tools/context_md_audit.py 2>&1 | grep -A2 "^internal/\(actions\|behaviortree\|configs\|gametime\|lightnotice\|lightscale\|messaging\|rooms\)\b"
git diff origin/master -- docs internal/*/context.md | grep "^+" | grep -cP "\x{2014}|\x{2013}"
P="docs/PATCH_NOTES.md docs/schemas/behavior.md internal/actions/context.md internal/behaviortree/context.md internal/configs/context.md internal/gametime/context.md internal/lightnotice/context.md internal/lightscale/context.md internal/messaging/context.md internal/rooms/context.md"
git add $P
git diff --cached 8d7fa2267 -- $P
git commit -m "docs: lighting plan 6 context.md and patch notes (#372)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected: the audit prints only `internal/configs (2 phantom of 19 documented)` with `func server_Config` and `func Get`, both there before this plan (the audit's 27 phantoms are the same before and after); then `0` (run the `grep -c` on its own line: it exits 1 on a zero count, so it must not lead an `&&` chain); then an empty diff. After this commit the tree equals the dry run's final tree outside the spec and plan: `git diff 8d7fa2267 HEAD -- . ":(exclude)docs/superpowers"` prints nothing, unless master moved some other file after `f3144f81a` (then only that file shows).

---

### Task 9: gates, the closing playtest, and the PR

No checkpoint. The controller runs this task.

- [ ] **Step 1: Full gate**

```bash
gofmt -l internal/ modules/
go vet ./...
go build ./...
go test ./... -count=1 2>&1 | grep -E "^(--- FAIL|FAIL|panic)"
golangci-lint run --new-from-merge-base=origin/master
python tools/context_md_audit.py 2>&1 | tail -3
```

Expected: nothing, nothing, nothing, nothing, `0 issues.` (the dry run also printed a stale-cache warning naming another worktree), and the audit's totals unchanged from master (27 phantoms in 14 packages in the dry run, before and after). Known flake: `internal/playtestrun` can hang under load; rerun it alone. Then the rest of the pre-push gate in `dogmud-shipping`, including its detached-worktree boot check on private ports. Never kill a server by name or port: the owner runs their own on this machine.

- [ ] **Step 2: Whole-branch review** (opus, read-only), findings fixed in their own commits (no checkpoint covers them) and gated as in Step 1.

- [ ] **Step 3: The closing playtest (spec section 9, the gate)**

Run by the controller with the `playtest-scenario` skill (ephemeral goals file, `--checkout` of this branch; `dogmud-playtesting` for the harness, its location and its traps). Because of #408 (`server set day` rewinds the clock to that day's sunrise), reach every target hour by advancing rounds, never with `server set day`. Six scenarios:

1. **Crime in the dark**, the rewritten M5 PR 3 Task 9 (`docs/superpowers/plans/completed/2026-09-22-messaging-m5-pr3-crime-in-the-dark.md`) under graded light, its five scenarios kept: clear sight identifies; no sight records an unknown perpetrator with no reputation change; shapes only, through a real source (Heat Sight or the Pitsense Tincture, not a hand-applied condition 85), stays unknown but seeds revenge; night vision identifies; a sleeping witness is no witness. Plus: an unlit backstreet at night is a low-risk scene, a lamplit main street is not.
2. **#332 merchant night checks:** with no carried light at night, trade with Siv and the other lantern keepers; keepers show by name; pinned keepers stay put.
3. **Street lamps:** a main street reads daylight only at noon and lamplit after dusk; the lamps change in the same round as the 4111 arch lantern; on a midwinter morning at 08:00 (day 356) the lamps are still lit and a stall keeper trades.
4. **Through exits:** on a bare lamplit street at night `look <exit>` is refused; with a lantern it sees through; an infravision character sees "a figure" in the next room, never a name.
5. **Relit rooms:** visit 3109, 497 and 6403 by day, and 5255; each reads as its text says.
6. **No natural negatives:** holding cell 5105 at night with no moons reads as barely lit (3), not darker than a sealed cave.

Findings go to GitHub issues (`gh issue create --repo pruuk/DOGMud ...`, searching first); reports are gitignored. A finding that is a defect in this branch is fixed here, in its own commit, before the PR merges.

- [ ] **Step 4: Issue upkeep.** `gh issue view 372 --repo pruuk/DOGMud`, then refresh its stale checklist (spec section 8) and correct the note that cave trims never fire (spec section 6: a glow spell cast at 75 or more trims to 74 on entry) with `gh issue edit 372 --repo pruuk/DOGMud --body-file "$TMP/372-body.md"`, built from the body just read. The same correction goes to the session handoff memory at EOD.

- [ ] **Step 5: PR.** Before pushing, `git ls-files -v _datafiles/config.yaml` once more: `S` means the committed blob came from Task 5's procedure and the disk copy may differ, which is fine; `H` in a worktree where it read `S` before means `--cacheinfo` cleared the bit, so restore it with `git update-index --skip-worktree _datafiles/config.yaml`. Then `git push -u origin feat/lighting-plan-6`; `gh pr create --repo pruuk/DOGMud --base master --head feat/lighting-plan-6 ...`. Every `gh` command carries `--repo pruuk/DOGMud`. The body uses a closing keyword only for an issue this PR closes: "Closes #372" once the closing playtest passed and the checklist is done, and "closes #332" once scenario 2 passed; otherwise "Part of #372". Never put "closes", "fixes" or "resolves" in front of any other issue number, even in prose about later work (#207, #408 and the out-of-scope list in the spec are mentioned bare). Read back the URL: it must say `pruuk/DOGMud`. Merge with `--merge --delete-branch` once checks are green. The owner deploys.

---

## Self-review

- Spec coverage: section 1 (arithmetic, floor, Trim, other `Combine` users) Task 2, with its world test in Task 3; section 2 (street lamps, arch lantern) Task 4; section 3 (`LightExitsAbove` 55) Task 5 and (infravision through exits) Task 6; section 4 (rooms) and section 5 (dense forest) Task 7; section 6 (cave trim notes) Task 9 Step 4; section 7 (config surfacing) Task 5; section 8 (docs, patch notes, #372 checklist) Task 8 and Task 9 Step 4; section 9 (closing playtest) Task 9 Step 3; Testing (golden before the rebuild, unit properties, world test, #207 guard green) Tasks 1 to 4.
- Every checkpoint passed `go build ./...`, `go vet ./...` and `go test ./... -count=1` in the dry run, so no task leaves a red test behind it.
