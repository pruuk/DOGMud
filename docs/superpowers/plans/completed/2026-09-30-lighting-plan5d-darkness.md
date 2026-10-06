# Lighting Plan 5d: Darkness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rooms can be darker than any cave: a new `darkness_strength` record kind, combined by the halving rule and subtracted from the room's light, trimmed to its bearer's usable range by the one log solve, carried by the Chrysalis Pall spell and the Umbral Lantern, with the Chrysalis Phantom holding its lair at -50, mobs acting on any shape they can make out, a darkness notice cause, and a `Char.Sight` band tinting the web client's Game window.

**Architecture:** `internal/lightscale` loses the linear `Darkens` branch and `Polarity`; `Trim(step, others, max, target)` is the one solve and `TrimDarkness` feeds it the darkness budget (ruling D2). `internal/conditions` gains `EffectDarknessStrength`, a light record with darkening polarity that `IsLightSource()` never matches (D1). `internal/rooms` composes light minus darkness and trims both kinds in held order; `LightTerms` gains `Light`, `Dark`, `Darkened`, and `Raw` becomes the net (D3). `behaviortree.mobCanSee` accepts `SightShapes` from `ParticipantSight` directly, leaving the combat predicate alone (D8). `lightnotice` gains `CauseDarkness` (D4) and queues `events.SightBandChanged`, which `modules/gmcp` answers with `Char.Sight` (D7). Content, help and the web client follow.

**Tech Stack:** Go 1.25, YAML world data, the `internal/events` queue, GMCP JSON, vanilla JS and CSS in `_datafiles/html/public`, Node for the static web-client checks.

**Spec (binding):** `docs/superpowers/specs/2026-09-30-lighting-plan5d-darkness-design.md`, owner decisions 1 to 9 and the owner rulings D1 to D9 (D8 ruled against its recommendation: ALL shapes count for `mobCanSee`, and the fix lives in `mobCanSee` only, never in `messaging.CanSeeSightImpairedOnly`).

**Branch:** implementation branch `feature/lighting-5d-darkness`, cut from master AFTER this docs branch (`docs/lighting-5d-spec`) merges, in the worktree `C:/tmp/dogmud-5d`:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feature/lighting-5d-darkness C:/tmp/dogmud-5d origin/master
```

All paths below are relative to that worktree. Run Go commands from its root. Name `C:/tmp/dogmud-5d` in the handoff memory while the branch is open (it must outlive the session); remove it with `git worktree remove C:/tmp/dogmud-5d` once the PR merges. Throwaway output goes in the session scratchpad (`$TMP`), never `C:/tmp`.

Every commit names its paths (never `git add -A` or `git add .`) and ends with a blank line and then exactly `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; the `-m "..." -m "Co-Authored-By: ..."` form below produces that.

---

## Facts verified against source (2026-09-30, worktree HEAD `81cb730a0`)

Every row was read or grepped at `81cb730a0` (the D1 to D9 rulings commit; its Go and data tree is byte-identical to master `47220bce3`, the spec's base). The spec's own facts table (L1 to L13, R1 to R17, S1 to S11, K1 to K11, P1 to P8, I1 to I9, M1 to M5, RM1 and RM2, N1 to N4, G1 to G3, W1 to W3, H1 and H2, F1 to F3) was re-read and holds; the rows below are the ones this plan leans on, and rows marked **NEW** are facts the spec does not state, or states wrongly, and that change the plan. Every code block in Tasks 1 to 12 was applied to this worktree and run through `gofmt -l internal/ modules/ .` (clean), `go vet ./...` (clean), `go build ./...`, `go test ./... -count=1` (129 packages `ok`, none failing), `golangci-lint run --new-from-merge-base=origin/master` (`0 issues.`), both Node web-client checks (`ALL CHECKS PASSED`), `python tools/context_md_audit.py` (output byte-identical before and after the docs, see F29) and a boot on private ports (`Server Ready`, no panic, `mapper.ValidateZoneConsi errors=0`), then reverted. Each failing-first test was seen to fail for the reason its step names, and each "proven able to fail" probe was run and seen red. The Docker race run was not part of the proof; Task 12 runs it.

| # | Fact | Where |
|---|---|---|
| F1 | `Trim(step, others, max, target float64, p Polarity) float64`; the `Darkens` branch is linear (`cut := level - target`); `Polarity`, `Brightens`, `Darkens` are declared only here | `internal/lightscale/trim.go:10-17,52-84` |
| F2 | The only non-test caller of `Trim` is `internal/rooms/light_trim.go:64` (`lightscale.Brightens`). **NEW:** `trim_test.go` reads `Darkens` in six tests (`TestTrimDarknessIsTheInverse` `:42`, NaN `:78,81`, step `:95`, NaN others `:105`, non-positive max `:110`, at target `:118`); Task 1 replaces the file whole | grep `Darkens\|Brightens\|Polarity` |
| F3 | `composeWith(cfg, celestial, skyFilter float64, carried []float64) LightTerms` sets `Raw` to the light combine (`-Inf` when nothing lights the room) before the Absent-to-0 step; `carriedLight(exclude)` walks `r.mobs` then `r.players` over `LightSources()` | `internal/rooms/lighting.go:59-76,97-146,151-174` |
| F4 | **NEW.** `composeWith` has exactly one production caller (`lighting.go:92`) and five test calls, all in `internal/rooms/carried_light_test.go` (`:17,18,25,31,43`). `:31` asserts an empty cave's `Raw` is `-Inf`; under ruling D3 `Raw` is the net and reads 0 there, so Task 5 moves that assertion onto the new `Light` field (the same `-Inf` claim, on the field that now carries it) and pins `Raw` 0 | grep `composeWith(` |
| F5 | `TrimLightFor` collects `LightSources()` records that are adjustable and unhooded, sets them all off, then solves each against `composeLightExcluding(...).Raw`; callers `MoveToRoom` and `AddMob` only | `internal/rooms/light_trim.go:26-72`; `roommanager.go:474`; `rooms.go:1253` |
| F6 | `listMobInRoom(target, id)` appends to `target.mobs` with no trim and no `RoomChange`; `Room.Prepare` spawns through `mobs.NewMobById`. Measured: after `Prepare`, room 508 reads -50 with the lantern at full | `internal/rooms/rooms.go:890-907,952-1060` |
| F7 | `EffectKind` set: `AllEffectKinds` `effects.go:45-49`, `ScaledKinds` `:73`, `validateEffects` light rules `:171-181`, `Effect` returns 0 for light `:201-204`. `TestEveryEffectKindIsClassifiedExactlyOnce` walks `AllEffectKinds` (`effects_test.go:299`); a darkness kind is none of multiplier, cap or max, so it passes unchanged | `internal/conditions/effects.go`; `effects_test.go:299-315` |
| F8 | `LightMax` reads only `light_strength` (`light.go:20-32`); `LightNow` rides it; `LightSources` `:87-98`; `IsLightSource` `conditionspec.go:248-251`; the `Adjustable` doc `:106-109`; `AddConditionMagnitude` resets only `IsLightSource()` records (`conditions.go:367-369`) | files named |
| F9 | `SpellScaledMagnitude(kind, stat, skill)` picks a trio by kind; `magnitudeSpellApplication` computes triggers inline from the shared `SpellDuration*` trio and imports `math` and `configs` only for that | `internal/conditions/scaled_magnitude.go:17-27`; `internal/hooks/light_spell.go:24-45` |
| F10 | **NEW.** `condition_apply_path_guard_test.go:263` keys `"internal/hooks/light_spell.go|63"` (the `target.AddConditionMagnitude` line). Task 3 deletes the two imports and the inline duration and adds two comment lines, so the call moves to line 58; measured: the guard reports the unallowlisted `|58` and the stale `|63` until re-keyed. Task 3 re-keys it and widens the reason to name darkness | `condition_apply_path_guard_test.go:263`; `TestPlayerConditionsTravelTheEventPath` |
| F11 | **NEW.** The same guard keys `Condition_ApplyConditions.go|103`, `|105`, `|107` (`:111-113`). Task 6 edits `:175-176` in place (two lines for two) and appends its helper after the file's last function, so no keyed line moves | `condition_apply_path_guard_test.go:111-113` |
| F12 | The start room line is sent at `Condition_ApplyConditions.go:175-176` through `r.SendTextVisualHidingNames(messaging.CategoryConditionApply, roles.Observer, []string{charPlainName}, excludeId)`; `conditionInfo` is the `*conditions.ConditionSpec` from `:34`. Its end twin is `sendConditionEndRoomText` | `internal/hooks/Condition_ApplyConditions.go:34,175-176`; `NewTurn_PruneConditions.go:127-145` |
| F13 | **NEW.** `messaging_surface_guard_test.go:1359` registers `usercommands/equip.go|You wear your <ansi fg="item">%s</ansi>.` as actor + observer. Its walk (`narrationCollectFromBlock`, `:1009-1062`) splits an `if/else` with no trailing narration call into separate events, but flattens an else-less, non-terminating `if` into the surrounding event. Measured: an `if/else` choosing `SendTextVisualAsLit` or `SendTextVisual` makes that entry STALE (`TestNarrationSitesMatchViewpointAudit`). Task 6 uses two else-less `if`s, which keeps the entry and its coverage | `messaging_surface_guard_test.go:1030-1062,1359` |
| F14 | The mob equip wearable line is `room.SendTextVisual(... puts on ...)` at `internal/mobcommands/equip.go:86-88`; no guard keys it (the surface walk is actor-anchored and a mob path has no actor line) | file named; full root run |
| F15 | `sight_gates_wrapper_guard_test.go:34-38` forbids `IsCursed|Spellcasting|CursedRefusal|ChooseWornSlot|\.Wear\(` and `GetHandPairs|HandsRequired|ItemPtr` in `usercommands/equip.go`; Task 6 adds none of them | file named |
| F16 | `mobCanSee` is `messaging.CanSeeSightImpairedOnly(&mob.Character, room)`; callers `conditions_player.go:118,196`, `actions_party.go:238`. **NEW:** measured, widening it to `SightFull || SightShapes` breaks no existing test in the repo (full run green); three test comments (`sight_test.go:66-69`, `conditions_sight_test.go:19-22`, `conditions_test.go:494-498`) still say it requires `SightFull`, and Task 7 corrects them | `internal/behaviortree/sight.go:29-34`; grep |
| F17 | `CanSeeSightImpairedOnly` non-test readers besides `mobCanSee`: `hooks/NewRound_DoCombat_resolution.go:98,111`, `hooks/NewRound_DoCombat_unified.go:565,568`. **NEW:** `internal/messaging/context.md` tells this predicate's combat history but names no current reader; Task 12 adds them | grep `CanSeeSightImpairedOnly(` |
| F18 | `sightScene(t, biome)` seeds `cave` (`SkyLight 0`) and `city` (`Lamp 90`), mob 8101 in room 8100, conditions 29 (NightVision flag) and 1 | `internal/behaviortree/sight_test.go:18-60` |
| F19 | `allCauses` `store.go:44`; `LoadFrom` refuses a cause with no file (`:174-178`); the narration snapshot iterates `lightnotice.Causes()` in order (`snapshot_test.go:1632`). **NEW:** appending `CauseDarkness` LAST makes `light_notices.golden` grow by exactly twelve trailing lines (measured); `store_test.go:73` drops `Causes()[1:]`, still the movement file | `internal/lightnotice/store.go`; `internal/narration/snapshot_test.go:1620-1650` |
| F20 | `attribute` order: movement, eyes counterfactual, then `Carried`, lamp, weather, `skyMoved(a.Sky, b.Sky)`; `skyMoved` has no other caller (grep, tests included) | `internal/lightnotice/tracker.go:146-177` |
| F21 | **NEW.** `decide` keeps the PREVIOUS record, band included, while the player sleeps or is blinded (`tracker.go:86-95`), so "the player's recorded band" (ruling D7) would not follow `LightBand` for a sleeper. Task 8 keeps the last band handed to GMCP in its own `sentBands` map under the same mutex | `internal/lightnotice/tracker.go:86-95,180-183` |
| F22 | **NEW.** `seedLampWorld` (`check_test.go:17-35`) seats user 1 in room 1 by `RoomId` only, never `AddPlayer`; a carried darkness counts only for someone the room lists (`carriedTerms` walks `r.players`), so Task 8's check test calls `r1.AddPlayer` | file named; measured |
| F23 | `CharacterVitalsChanged{UserId}` `eventtypes.go:434-438`; `DrainQueuedVitalsChangedForTest(userId)` `events.go:599-620` is the drain shape Task 8 copies | files named |
| F24 | `GMCPCharUpdate{UserId, Identifier}`; `buildAndSendGMCPPayload` title-cases each dotted part (`Char.Sight` stays `Char.Sight`) and calls `GetCharNode`; `GetCharNode` builds every block when `all` (`gmcpModule == "Char"`); `wantsGMCPPayload` is a prefix test that returns false for a longer request | `modules/gmcp/gmcp.Char.go:89-94,343-391,393-693,696-711` |
| F25 | **NEW.** `messaging.LightBand(observer, room RoomVisibility)` guards `room == nil` on the INTERFACE; a nil `*rooms.Room` passed through it is a non-nil interface and panics on `LightLevel`. Task 9's `sightBand` passes an untyped nil when `rooms.LoadRoom` misses | `internal/messaging/band.go:68-84` |
| F26 | The Game window is `#panel-feed` (`webclient-pure.html:318`); `handleGMCP` runs only the first handler matching the longest path and returns (`:2091-2100`), so a full `Char` push reaches only the `"Char"` handler, which must delegate (`tools/webclient-tests/char-handler-shadowing.js` fails a `Char.*` handler that is neither delegated nor listed event-only). **NEW:** the pop-out is `new WinBox({mount: panel})` (`static/js/dashboard.js:628-660`), which MOVES `#panel-feed` into the window, and no CSS rule keys `[data-popped]`; a class on `#panel-feed` draws its border docked and popped alike | files named |
| F27 | `.dash-panel` draws `border: 1px solid var(--antique-gold, #b89047)` (`dashboard.css:63-69`); `#panel-feed` rules at `:129-137`. `node --version` is `v24.13.0` on this box | files named |
| F28 | **NEW, and the spec's Rule 5 cannot ship as written.** `items.ValidateVendorCategories` (`internal/items/validation.go:18-61`) PANICS the boot for any item that is not `notsalable` and not a quest token and has no `vendor_categories`; measured: `item 20098 ("Umbral Lantern"): missing vendor_categories`. No unit test catches it. `vendor_categories` says which shops BUY the item (`internal/shops/buyrules.go:52-55`); stock is authored per shop, so the list does not stock it | files named; boot log |
| F29 | **NEW.** `python tools/context_md_audit.py` already reports 16 phantom lines on this tree, none in a package this plan touches except `internal/configs` (`func server_Config`, `func Get`, both pre-existing). Its output was byte-identical with and without Task 12's edits | measured, `diff` of both runs |
| F30 | **NEW.** `shipped_narration_data_guard_test.go`'s `observerIdentityGuardContentSafeViaCode` (`:1003-1070`) lists every condition file whose observer lines carry a bare `{actee_plain}`; measured: `131-chrysalis_pall.yaml:18` and `:20` fail `TestObserverIdentityTagsAreAnonymizable` until registered. Task 11 registers it beside 128 to 130, with its reason | file named; measured |
| F31 | **NEW.** Adding the content moves exactly three narration goldens, all by pure additions (measured): `conditions.golden` +4 (`condition|131|...`), `spells.golden` +3 (`spell|chrysalis-pall|...`), `light_notices.golden` +12 (Task 8). The lighting parity and daycycle goldens do not move (root package green) | `internal/narration/testdata/stores/`; `lighting_*_golden_test.go` |
| F32 | **NEW.** `users.SeedUsersForTest` REPLACES the whole registry, so two calls in one test leave only the second call's users; Task 5's fixture reseeds all of a test's users on each add | `internal/users/test_helpers.go:29`; measured |
| F33 | Test fixtures reused: hooks `seedAllRegistries` (users 1 "Aliceia" and 2 in room 1), `drainPlain`, `countContaining`; usercommands `seedAllRegistries`, `hoodTestText` (`hood_test.go:56`), `Equip(rest, user, room, flags)`; mobcommands `seedAllRegistries`, `getTestMobAndRoom` (mob 100 in room 1 with users 1 and 2); rooms `withShippedBiomesAndClock`, `requireBiome`, `modelCfg`, `LampPtr`; lightnotice `seedLampWorld`, `captureFor`, `containsAny`, `rec`, `obs`, `sight` | files named |
| F34 | `_datafiles/config.yaml`: the `git show HEAD:` blob's 5c block ends at line 981, `  LightInfraPenaltyFloor: 0.90`, followed by a blank line and `# ── COMBAT: DAMAGE`. In this fresh worktree `git ls-files -v` prints `H`; the skip-worktree bit lives in the MAIN checkout's index (CLAUDE.md), so Task 2 builds the file from the blob either way | `git show HEAD:_datafiles/config.yaml`; `git ls-files -v` |
| F35 | Ids and names: `python tools/id_inventory.py --alloc conditions 3` prints `reserve IDs 131-133`; `^itemid: 20098$` matches nothing (the same grep for `20097` finds the hooded lantern); `Umbral`, `chrysalis-pall`, `Phantom Heat Sense`, `Umbral Dark` are unused; filenames follow `ConvertForFilename(name)`: `131-chrysalis_pall.yaml`, `132-umbral_dark.yaml`, `133-phantom_heat_sense.yaml`, `20098-umbral_lantern.yaml`, spell `chrysalis-pall.yaml` (the spellid) | grep; `tools/id_inventory.py` |
| F36 | Help: `light` aliases `[..., dark, darkness]` (`keywords.yaml:303-304`); `general:` holds `light`, `moons`, `seasons` (`:165-167`); the See also lines are `light.template:56-58`, `seasons.template:17`, `moons.template:16` | files named |
| F37 | `docs/PATCH_NOTES.md`'s newest entry is `## 2026-10-01: Secondhand shelves` (`:3`); `docs/README.md:152` is this slice's spec row and `:73` the `tools/webclient-tests/` row | files named |

## Where the spec could not be implemented as written

1. **The Umbral Lantern ships `vendor_categories: [blacksmithing]`, not none (F28).** Rule 5 says "no `vendor_categories` (a boss drop, not stocked)". The boot refuses that. The list names who BUYS the item, not who stocks it, so `[blacksmithing]` (the hooded lantern's) keeps it a drop that is never stocked and still sells. `notsalable: true` was the other way out; it would make a boss drop unsellable, which the spec did not ask for. Task 11's shipped-item test pins the list.
2. **The lantern's `value: 60` is this plan's number, not the spec's.** The spec is silent; the hooded lantern is 20. Three times it, for a boss drop, is a guess the owner may retune; it is named in the PR.
3. **The GMCP band has its own record (F21).** D7 says `Check` compares "the player's recorded one". The notice record keeps its old band while its player sleeps or is blinded, so the plan keeps `sentBands` beside it: the band last handed to GMCP, cleared by `Forget` with the record.
4. **`Raw` reads 0, not `-Inf`, in an unlit room (F4).** D3 makes `Raw` the net with Absent light read as 0; one existing assertion said `-Inf` and moves onto `Light`. Nothing else reads `Raw` (spec L9).
5. **The equip line uses two else-less `if`s (F13).** D6's as-lit equip line, written as an `if/else`, blinds the viewpoint guard to that site's observer. The plan keeps the guard's coverage by shape rather than editing its registry.
6. **Three helpers the spec does not name.** Rule 3's trim walks "light AND darkness records in held order"; the spec names `DarknessSources` and `LightSources` but no walk over both in one order, so the plan adds `Conditions.LightAndDarknessSources()` (the composition pass uses it too). Rule 4's "the duration trio by kind" becomes `conditions.SpellScaledTriggers`, beside `SpellScaledMagnitude`. D6's "an item whose worn condition is a darkness source" becomes `conditions.AnyDarknessSource(ids)`, which both equip paths call.
7. **The mob equip path gets D6 as well.** D6 names "the equip room line"; `internal/mobcommands/equip.go` sends the same line for a mob, so the sibling path is finished rather than filed.

No design ruling changes. D8 is implemented exactly as ruled: `mobCanSee` reads `messaging.ParticipantSight` and accepts `SightFull` or `SightShapes`; `CanSeeSightImpairedOnly` and the combat darkness penalty are untouched (Task 7 pins both).

## Player-visible lines that change (everything else stays byte-identical)

| Line | Before | After |
|---|---|---|
| A darkness arriving, lapsing or changing strength in the player's room | Not possible | A notice from `narration/light-notices/darkness.yaml` (e.g. `A darkness swallows the light, and you can see nothing.`), never an `eyes` or `carried` line |
| `A pall of dark spores gathers around <name>.` (room) / `A pall of dark spores gathers around you.` (holder) | Not possible | Condition 131's start lines; the room line is judged as lit (D6) |
| `The pall around <name> thins away.` / `Your pall thins, and the light comes back.` | Not possible | Condition 131's end lines |
| `You coax the Chrysalis spores around you to drink the light.`, `<name> concentrates as the air around them dims and thickens.`, `You hold the image steady as the spores darken...` | Not possible | The spell's cast, observer and wait lines |
| `<name> puts on their Umbral Lantern.` (player) / `<mob> puts on Umbral Lantern.` (mob) | Judged by the room as it is | For an item whose worn condition is a darkness, judged as lit; every other item unchanged |
| `Your Umbral Lantern has no hood.` | Not possible | `hood` / `unhood` with the lantern in the light slot (the existing refusal line) |
| `help darkness`, `help umbral`, `help pall` | `darkness` opened `light` | The new `darkness` topic; `help dark` still opens `light`; `help chrysalis-pall` opens the spell page |
| `help light`, `help seasons`, `help moons` "See also" | | Gains `help darkness` |
| Mob behaviour in any dim (shapes-band) room with a player present | The mob saw nobody (`SightFull` only) | Ambushers, aggressive mobs and party aggro act on the shape (D8), everywhere |
| The web client's Game window border | Always antique gold | Tinted by band, with a tooltip: `Too dark to see.`, `Dim light: shapes, not faces.`, `Good light.`, `Too bright: the glare hurts.` |

## File map

| File | Change |
|---|---|
| `internal/lightscale/trim.go`, `trim_test.go` | `Polarity` and the linear branch deleted; `Trim(step, others, max, target)`; `TrimDarkness` (Task 1) |
| `internal/rooms/light_trim.go` | One-line caller update in Task 1; rewritten in Task 5 |
| `internal/configs/config.balance.go`, `config.balance.lighting.go`, `config.lighting_accessor.go` | Six `LightDarknessSpell*` knobs, defaults, accessor fields (Task 2) |
| `internal/configs/config_lighting_5d_test.go` | Create (Task 2) |
| `_datafiles/config.yaml` | The six keys, built from the `HEAD` blob (Task 2) |
| `internal/conditions/effects.go`, `conditionspec.go`, `conditions.go`, `light.go`, `scaled_magnitude.go` | `EffectDarknessStrength`, validation, `IsDarknessSource`, `AnyDarknessSource`, `DarknessSources`, `LightAndDarknessSources`, `LightMax` reads either kind, the reset, `SpellScaledTriggers` (Task 3) |
| `internal/conditions/darkness_test.go` | Create (Task 3) |
| `internal/hooks/light_spell.go`, `condition_apply_path_guard_test.go` | Duration through `SpellScaledTriggers`; guard re-keyed `|63` to `|58` (Task 3) |
| `internal/characters/light.go`, `internal/messaging/window.go` | `DarknessTerms`; `DarknessTrimTarget` (Task 4) |
| `internal/messaging/darkness_trim_target_test.go` | Create (Task 4) |
| `internal/rooms/lighting.go`, `light_trim.go`, `carried_light_test.go` | Light minus darkness; `LightTerms.Light`, `Dark`, `Darkened`; `carriedTerms`; both polarities trim (Task 5) |
| `internal/rooms/darkness_compose_test.go`, `darkness_trim_test.go` | Create (Task 5) |
| `internal/hooks/Condition_ApplyConditions.go`, `internal/usercommands/equip.go`, `internal/mobcommands/equip.go` | D6 as-lit start and equip lines (Task 6) |
| `internal/hooks/darkness_condition_test.go`, `internal/usercommands/darkness_test.go`, `internal/mobcommands/equip_darkness_test.go` | Create (Task 6) |
| `internal/behaviortree/sight.go`, `sight_test.go`, `conditions_sight_test.go`, `conditions_test.go` | D8; three stale comments (Task 7) |
| `internal/behaviortree/sight_shapes_test.go` | Create (Task 7) |
| `internal/events/eventtypes.go`, `events.go` | `SightBandChanged`, its drain helper (Task 8) |
| `internal/lightnotice/store.go`, `tracker.go` | `CauseDarkness`, `termMoved`, `sentBands` and the event (Task 8) |
| `_datafiles/world/dogmud/narration/light-notices/darkness.yaml` | Create (Task 8) |
| `internal/lightnotice/darkness_test.go` | Create (Task 8) |
| `internal/narration/testdata/stores/light_notices.golden` | Re-recorded, +12 lines (Task 8) |
| `modules/gmcp/gmcp.Char.go`, `modules/gmcp/gmcp.CharSight_test.go` | `Char.Sight` (Task 9) |
| `_datafiles/html/public/webclient-pure.html`, `_datafiles/html/public/static/css/dashboard.css`, `tools/webclient-tests/sight-border.js` | The border (Task 10) |
| `_datafiles/world/dogmud/spells/chrysalis-pall.yaml`, `conditions/131-chrysalis_pall.yaml`, `conditions/132-umbral_dark.yaml`, `conditions/133-phantom_heat_sense.yaml`, `items/armor-20000/light/20098-umbral_lantern.yaml`, `templates/help/darkness.template`, `templates/help/chrysalis-pall.template` | Create (Task 11) |
| `_datafiles/world/dogmud/mobs/thornwall_city/272-chrysalis_phantom.yaml`, `keywords.yaml`, `templates/help/light.template`, `seasons.template`, `moons.template` | Modify (Task 11) |
| `internal/items/shipped_light_items_test.go`, `internal/spells/chrysalis_pall_test.go`, `internal/behaviortree/phantom_lair_test.go` | Extend; create; create (Task 11) |
| `shipped_narration_data_guard_test.go`, `internal/narration/testdata/stores/conditions.golden`, `spells.golden` | Register 131; re-record +4 and +3 (Task 11) |
| `context.md` in `internal/lightscale`, `configs`, `conditions`, `characters`, `messaging`, `rooms`, `hooks`, `usercommands`, `mobcommands`, `behaviortree`, `lightnotice`, `events`, `modules/gmcp`; `docs/PATCH_NOTES.md`; `docs/README.md` | Task 12 |

### Repo-root guards and goldens this plan touches

| Guard or golden | Keyed by | What moves | Where handled |
|---|---|---|---|
| `condition_apply_path_guard_test.go` `conditionApplyPathAllowlist` | `file|line` | `internal/hooks/light_spell.go|63` becomes `|58` (F10) | Task 3, re-keyed |
| `condition_apply_path_guard_test.go` `Condition_ApplyConditions.go|103,105,107` | `file|line` | Nothing: the edit is line-neutral above the helper (F11) | Task 6, verified |
| `messaging_surface_guard_test.go` `usercommands/equip.go|You wear your ...` | `file|literal` plus event shape | Nothing, by the two-`if` shape (F13) | Task 6, verified |
| `shipped_narration_data_guard_test.go` `observerIdentityGuardContentSafeViaCode` | content file | `conditions/131-chrysalis_pall.yaml` registered, with its reason (F30) | Task 11 |
| `sight_gates_wrapper_guard_test.go` | forbidden words in `equip.go` | Nothing (F15) | Task 6, verified |
| `internal/narration/testdata/stores/light_notices.golden` | snapshot | +12 trailing `darkness|...` lines, nothing else (F19) | Task 8 |
| `internal/narration/testdata/stores/conditions.golden`, `spells.golden` | snapshot | +4 `condition|131|...`, +3 `spell|chrysalis-pall|...` (F31) | Task 11 |
| `lighting_parity_golden_test.go`, `lighting_daycycle_golden_test.go` | snapshot | Must NOT move (nothing sampled carries darkness); if either moves, explain it with `tools/lighting_golden_diff.py` before any re-record | Tasks 5 and 12 |
| `tools/webclient-tests/char-handler-shadowing.js` | handler keys | `Char.Sight` must be delegated from the `"Char"` handler | Task 10 |

No guard is weakened: one key is re-keyed to the line its call moved to, one content file is registered with the reason the guard asks for, and the goldens gain only the new entries.

---

### Task 1: One log solve for both polarities: `Trim` and `TrimDarkness` (ruling D2)

**Model:** haiku (mechanical, code given).

**Files:**
- Modify: `internal/lightscale/trim.go` (whole file)
- Modify: `internal/lightscale/trim_test.go` (whole file)
- Modify: `internal/rooms/light_trim.go:64` (the one caller)

- [ ] **Step 1: Write the failing tests**

Replace the whole of `internal/lightscale/trim_test.go` with the file below. The light tests keep their assertions with the `Brightens` argument dropped; the six `Darkens` tests go (the linear branch is deleted, F2); the Rule 3 table arrives as `TestTrimDarknessKeepsTheRoomOnTheFloor`, each row checked twice (output, and the room recomputed with `Combine`).

```go
package lightscale

import (
	"math"
	"testing"
)

func TestTrimLightLandsExactlyOnTarget(t *testing.T) {
	for _, others := range []float64{0, 20, 50, 60, 73} {
		out := Trim(8, others, 100, 74)
		if got := Combine(8, others, out); math.Abs(got-74) > 1e-9 {
			t.Errorf("others %v: Combine(others, Trim) = %v, want 74", others, got)
		}
	}
}

func TestTrimLightIsCappedAtFullStrength(t *testing.T) {
	if got := Trim(8, 0, 54, 74); got != 54 {
		t.Errorf("a weak lantern in a faint room runs at %v, want its full 54", got)
	}
}

func TestTrimLightInAnUnlitRoomIsTheTarget(t *testing.T) {
	if got := Trim(8, Absent(), 90, 74); got != 74 {
		t.Errorf("a strong glow in a cave trims to %v, want 74", got)
	}
	if got := Trim(8, Absent(), 54, 74); got != 54 {
		t.Errorf("a weak lantern in a cave runs at %v, want 54", got)
	}
}

func TestTrimLightGoesDarkWhenTheRoomIsAlreadyBright(t *testing.T) {
	for _, others := range []float64{74, 80} {
		if got := Trim(8, others, 90, 74); !math.IsInf(got, -1) {
			t.Errorf("others %v: Trim = %v, want Absent", others, got)
		}
	}
}

// A room within a hair of the target needs a term below zero, the darkest
// natural light, so the source is not needed at all rather than "lit" at a
// meaningless negative value.
func TestTrimLightJustBelowTargetIsAbsent(t *testing.T) {
	for _, d := range []float64{1e-3, 1e-9, 1e-14, 1e-15} {
		if got := Trim(8, 74-d, 90, 74); !math.IsInf(got, -1) {
			t.Errorf("others 74-%v: Trim = %v, want Absent", d, got)
		}
	}
	// Just far enough below that a real (non-negative) term is needed.
	if got := Trim(8, 70, 90, 74); !(got >= 0 && got < 74) {
		t.Errorf("others 70: Trim = %v, want a term in [0, 74)", got)
	}
}

func TestTrimNaNTargetOrMaxIsNeverNaN(t *testing.T) {
	if got := Trim(8, 20, 90, math.NaN()); !math.IsInf(got, -1) {
		t.Errorf("NaN target: Trim = %v, want Absent", got)
	}
	if got := Trim(8, 20, math.NaN(), 74); !math.IsInf(got, -1) {
		t.Errorf("NaN max: Trim = %v, want Absent", got)
	}
}

// Mirrors TestNonPositiveStepDoesNotPanicOrNaN in lightscale_test.go: a
// non-positive step must not panic or hand back NaN or +Inf. -Inf (Absent) is
// a legitimate result and is not checked against.
func TestTrimNonPositiveStepDoesNotPanicOrNaN(t *testing.T) {
	for _, step := range []float64{0, -4} {
		if got := Trim(step, 20, 90, 74); math.IsNaN(got) || math.IsInf(got, 1) {
			t.Errorf("step %v, light: Trim = %v", step, got)
		}
		if got := TrimDarkness(step, 50, 20, 40, 25); math.IsNaN(got) || math.IsInf(got, 1) {
			t.Errorf("step %v, darkness: TrimDarkness = %v", step, got)
		}
	}
}

func TestTrimNaNOthersBehavesAsAbsent(t *testing.T) {
	if got, want := Trim(8, math.NaN(), 90, 74), Trim(8, Absent(), 90, 74); got != want {
		t.Errorf("NaN others: Trim = %v, want %v (same as Absent others)", got, want)
	}
}

// The Rule 3 trim table of the 5d spec: an Umbral Lantern (full 50) entering
// alone, or beside other darkness. Every row is checked twice: the output,
// and the room it leaves, recomputed with Combine the way the room composes
// it (light, 0 when Absent, minus the combined darkness).
func TestTrimDarknessKeepsTheRoomOnTheFloor(t *testing.T) {
	absent := Absent()
	cases := []struct {
		name                    string
		light, otherDark, floor float64
		wantOut                 float64 // Absent() for off
		wantRoom                float64
	}{
		{"normal eyes, cave: off", absent, absent, 25, absent, 0},
		{"normal eyes, tavern 50", 50, absent, 25, 25, 25},
		{"normal eyes, noon 70", 70, absent, 25, 45, 25},
		{"normal eyes, light 90: full", 90, absent, 25, 50, 40},
		{"nightvision 24, light 50", 50, absent, 1, 49, 1},
		{"infravision 30, cave", absent, absent, -30, 30, -30},
		{"infravision 50, cave: full", absent, absent, -50, 50, -50},
		{"infravision 50, light 50: full", 50, absent, -50, 50, 0},
		{"normal eyes, light 50, other darkness 30: off", 50, 30, 25, absent, 20},
	}
	for _, c := range cases {
		out := TrimDarkness(8, c.light, c.otherDark, 50, c.floor)
		if math.IsInf(c.wantOut, -1) {
			if !math.IsInf(out, -1) {
				t.Errorf("%s: TrimDarkness = %v, want Absent (off)", c.name, out)
			}
		} else if math.Abs(out-c.wantOut) > 1e-9 {
			t.Errorf("%s: TrimDarkness = %v, want %v", c.name, out, c.wantOut)
		}
		light := c.light
		if math.IsInf(light, -1) {
			light = 0
		}
		dark := Combine(8, c.otherDark, out)
		if math.IsInf(dark, -1) {
			dark = 0
		}
		if room := light - dark; math.Abs(room-c.wantRoom) > 1e-9 {
			t.Errorf("%s: room after = %v, want %v", c.name, room, c.wantRoom)
		}
	}
}

// Other darkness below the budget: the source solves the halving rule, not a
// linear cut. Two darknesses of 20 and 12.9 combine to 25, not 32.9.
func TestTrimDarknessSolvesTheDarknessCombine(t *testing.T) {
	out := TrimDarkness(8, 50, 20, 50, 25)
	if !(out > 12.9 && out < 13.0) {
		t.Fatalf("TrimDarkness(light 50, other 20, floor 25) = %v, want about 12.9", out)
	}
	if got := 50 - Combine(8, 20, out); math.Abs(got-25) > 1e-9 {
		t.Errorf("room after = %v, want exactly the floor 25", got)
	}
}

func TestTrimDarknessIsCappedAtFullStrength(t *testing.T) {
	if got := TrimDarkness(8, 100, Absent(), 50, 25); got != 50 {
		t.Errorf("light 100, floor 25: TrimDarkness = %v, want its full 50", got)
	}
}

func TestTrimDarknessNaNFloorOrMaxIsAbsent(t *testing.T) {
	if got := TrimDarkness(8, 50, Absent(), 50, math.NaN()); !math.IsInf(got, -1) {
		t.Errorf("NaN floor: TrimDarkness = %v, want Absent", got)
	}
	if got := TrimDarkness(8, 50, Absent(), math.NaN(), 25); !math.IsInf(got, -1) {
		t.Errorf("NaN max: TrimDarkness = %v, want Absent", got)
	}
}

// A NaN light is read as Absent, which reads 0, exactly as an unlit room.
func TestTrimDarknessNaNLightReadsAsAnUnlitRoom(t *testing.T) {
	if got, want := TrimDarkness(8, math.NaN(), Absent(), 50, -30), TrimDarkness(8, Absent(), Absent(), 50, -30); got != want {
		t.Errorf("NaN light: TrimDarkness = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/lightscale/ -count=1`
Expected: FAIL to build. The old `Trim` takes five arguments, so the four-argument calls report `not enough arguments in call to Trim`, and the new tests report `undefined: TrimDarkness`.

- [ ] **Step 3: Replace `trim.go`**

Replace the whole of `internal/lightscale/trim.go` with:

```go
package lightscale

import "math"

// Trim returns the output an adjustable light source should run at: the
// smallest cut from its full strength that keeps the combined light at or
// below target.
//
// others is the combine of every other term the source joins, Absent when
// there is none. max and the result are light-scale terms fed to Combine.
//
// The result solves Combine(others, out) == target analytically; in floating
// point to rounding, capped at max. A non-positive max is passed through
// unchanged, because 0 (or below) is a legitimate light term and the caller,
// not Trim, is responsible for refusing a strengthless source. The result is
// Absent when others already reaches target without this source, or when the
// term the arithmetic needs would fall below 0, the darkest light that occurs
// naturally: such a source would have to be darker than an unlit cave to
// matter, so it is not needed at all rather than "lit" at a meaningless
// negative value. A linear "target - others" is wrong here: on a log scale
// adding a source does not add its value. With others Absent the result is
// min(target, max).
//
// A NaN target or max cannot produce a meaningful term; Trim returns Absent
// rather than propagate the NaN.
//
// It is the one solve for both polarities (lighting plan 5d, ruling D2): a
// darkness source solves the same equation on the darkness combine through
// TrimDarkness, which supplies the floor a low target needs.
func Trim(step, others, max, target float64) float64 {
	if math.IsNaN(target) || math.IsNaN(max) {
		return Absent()
	}
	if !(step > 0) {
		step = 1
	}
	if !present(others) {
		return math.Min(target, max)
	}
	if others >= target {
		return Absent()
	}
	need := target + step*math.Log2(1-math.Exp2((others-target)/step))
	if need < 0 {
		return Absent()
	}
	return math.Min(need, max)
}

// TrimDarkness returns the output an adjustable darkness source should run at
// so the room stays at or above floor: the least cut from its full strength
// that keeps light - Combine(otherDark, out) >= floor.
//
// light is the room's combined light (Absent reads 0, an unlit cave),
// otherDark the combine of every other darkness in the room (Absent when
// there is none), max the source's full strength and floor the bottom of its
// bearer's usable range (messaging.DarknessTrimTarget).
//
// Keeping the room at or above floor means Combine(otherDark, d) <= light -
// floor, which is Trim with others = otherDark and target = light - floor:
// darkness sources combine among themselves by the same halving rule lights
// do (lighting plan 5d, owner decision 1). A budget at or below 0 means the
// room already sits at or below floor without this source, so it is not
// needed and the result is Absent. That is the caller-side floor Trim's low
// targets need, applied once here.
func TrimDarkness(step, light, otherDark, max, floor float64) float64 {
	if math.IsNaN(floor) || math.IsNaN(max) {
		return Absent()
	}
	if !present(light) {
		light = 0
	}
	budget := light - floor
	if !(budget > 0) {
		return Absent()
	}
	return Trim(step, otherDark, max, budget)
}
```

- [ ] **Step 4: Update the one caller**

In `internal/rooms/light_trim.go`, replace

```go
		out := lightscale.Trim(cfg.DoublingStep, others, full, target, lightscale.Brightens)
```

with

```go
		out := lightscale.Trim(cfg.DoublingStep, others, full, target)
```

(Task 5 rewrites this file; this keeps the tree building in between.)

- [ ] **Step 5: Run them to see them pass**

Run: `go build ./... && go test ./internal/lightscale/ ./internal/rooms/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/lightscale` and `ok  	github.com/GoMudEngine/GoMud/internal/rooms`.

Run: `grep -rn "Darkens\|Brightens\|Polarity" --include=*.go .`
Expected: no output (exit 1 is correct here; run it standalone, not in an `&&` chain).

- [ ] **Step 6: Commit**

```bash
git add internal/lightscale/trim.go internal/lightscale/trim_test.go internal/rooms/light_trim.go
git commit -m "feat(lightscale): one log solve for light and darkness (5d D2)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Six `LightDarknessSpell*` knobs, defaulted and shipped (Rule 4, D9)

**Model:** haiku (mechanical, code given; the `config.yaml` step is exact).

**Files:**
- Modify: `internal/configs/config.balance.go` (end of the `Balance` struct, after `LightInfraPenaltyFloor` at `:1303`)
- Modify: `internal/configs/config.balance.lighting.go:193-196` (the spell-scaling default loop)
- Modify: `internal/configs/config.lighting_accessor.go:31-39,76-81`
- Create: `internal/configs/config_lighting_5d_test.go`
- Modify: `_datafiles/config.yaml` (after line 981), built from the `HEAD` blob

Load `dogmud-balance-config` before this task. Tests read the Go defaults, never `config.yaml`, except the one test here that reads the file on purpose to prove the keys ship.

- [ ] **Step 1: Write the failing tests**

Create `internal/configs/config_lighting_5d_test.go`:

```go
package configs

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

// A zero Balance is what a test binary sees; every 5d knob must default to
// glow's value (owner decision 3: shipped at glow's values).
func TestLighting5dKnobDefaults(t *testing.T) {
	var b Balance
	b.validateLighting()
	checks := []struct {
		name      string
		got, want float64
	}{
		{"LightDarknessSpellStrengthBase", float64(b.LightDarknessSpellStrengthBase), 40},
		{"LightDarknessSpellStrengthStatDivisor", float64(b.LightDarknessSpellStrengthStatDivisor), 10},
		{"LightDarknessSpellStrengthSkillDivisor", float64(b.LightDarknessSpellStrengthSkillDivisor), 2},
		{"LightDarknessSpellDurationBase", float64(b.LightDarknessSpellDurationBase), 2},
		{"LightDarknessSpellDurationStatDivisor", float64(b.LightDarknessSpellDurationStatDivisor), 50},
		{"LightDarknessSpellDurationSkillDivisor", float64(b.LightDarknessSpellDurationSkillDivisor), 20},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// A zero or negative divisor would divide by zero; it reverts.
func TestLighting5dKnobsRevertWhenNotPositive(t *testing.T) {
	b := Balance{LightDarknessSpellStrengthStatDivisor: -1, LightDarknessSpellDurationSkillDivisor: 0}
	b.validateLighting()
	if b.LightDarknessSpellStrengthStatDivisor != 10 || b.LightDarknessSpellDurationSkillDivisor != 20 {
		t.Errorf("divisors = %v / %v, want 10 / 20", b.LightDarknessSpellStrengthStatDivisor, b.LightDarknessSpellDurationSkillDivisor)
	}
}

func TestLightingAccessorCarries5dKnobs(t *testing.T) {
	cfg := GetConfig()
	cfg.Balance.LightDarknessSpellStrengthBase = 33
	cfg.Balance.LightDarknessSpellDurationStatDivisor = 44
	SetConfigForTest(t, cfg)
	l := GetLightingConfig()
	if l.DarknessSpellStrengthBase != 33 || l.DarknessSpellDurationStatDivisor != 44 ||
		l.DarknessSpellStrengthStatDivisor != 10 || l.DarknessSpellStrengthSkillDivisor != 2 ||
		l.DarknessSpellDurationBase != 2 || l.DarknessSpellDurationSkillDivisor != 20 {
		t.Errorf("accessor = %+v", l)
	}
}

// The six keys ship in config.yaml at glow's values, so the live value never
// silently rides the Go default (dogmud-balance-config).
func TestLighting5dKnobsShipInConfigYaml(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	// A map, not the Config struct: an ABSENT key must fail here, and a
	// struct field cannot tell absent from a shipped 0.
	var doc struct {
		Balance map[string]any `yaml:"Balance"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse config.yaml: %v", err)
	}
	want := map[string]float64{
		"LightDarknessSpellStrengthBase":         40,
		"LightDarknessSpellStrengthStatDivisor":  10,
		"LightDarknessSpellStrengthSkillDivisor": 2,
		"LightDarknessSpellDurationBase":         2,
		"LightDarknessSpellDurationStatDivisor":  50,
		"LightDarknessSpellDurationSkillDivisor": 20,
	}
	for k, v := range want {
		raw, ok := doc.Balance[k]
		if !ok {
			t.Errorf("config.yaml Balance has no %s", k)
			continue
		}
		var got float64
		switch n := raw.(type) {
		case int:
			got = float64(n)
		case float64:
			got = n
		default:
			t.Errorf("config.yaml Balance.%s = %v (%T), want a number", k, raw, raw)
			continue
		}
		if got != v {
			t.Errorf("config.yaml Balance.%s = %v, want %v", k, got, v)
		}
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/configs/ -run "Lighting5d|Carries5d" -count=1`
Expected: FAIL to build, `b.LightDarknessSpellStrengthBase undefined` (and the other five fields).

- [ ] **Step 3: Declare the knobs**

In `internal/configs/config.balance.go`, replace

```go
	LightInfraReachCap     ConfigInt   `yaml:"LightInfraReachCap"`     // default 50
	LightInfraPenaltyFloor ConfigFloat `yaml:"LightInfraPenaltyFloor"` // default 0.90
}
```

with

```go
	LightInfraReachCap     ConfigInt   `yaml:"LightInfraReachCap"`     // default 50
	LightInfraPenaltyFloor ConfigFloat `yaml:"LightInfraPenaltyFloor"` // default 0.90

	// Darkness-spell scaling (lighting plan 5d), the light trio's shape: a
	// spell whose condition declares darkness_strength: magnitude is cast at
	//   LightDarknessSpellStrengthBase + stat/LightDarknessSpellStrengthStatDivisor + spellcasting/LightDarknessSpellStrengthSkillDivisor
	// for
	//   LightDarknessSpellDurationBase + stat/LightDarknessSpellDurationStatDivisor + spellcasting/LightDarknessSpellDurationSkillDivisor
	// triggers (rounded, at least 1). Shipped at glow's values: a new caster
	// (100, 0) casts 50 for 4 triggers, a mid caster (130, 30) 68, an endgame
	// caster (175, 65) 90 for 9. Not capped: the scale clamps the room.
	LightDarknessSpellStrengthBase         ConfigFloat `yaml:"LightDarknessSpellStrengthBase"`         // default 40
	LightDarknessSpellStrengthStatDivisor  ConfigFloat `yaml:"LightDarknessSpellStrengthStatDivisor"`  // default 10
	LightDarknessSpellStrengthSkillDivisor ConfigFloat `yaml:"LightDarknessSpellStrengthSkillDivisor"` // default 2
	LightDarknessSpellDurationBase         ConfigFloat `yaml:"LightDarknessSpellDurationBase"`         // default 2
	LightDarknessSpellDurationStatDivisor  ConfigFloat `yaml:"LightDarknessSpellDurationStatDivisor"`  // default 50
	LightDarknessSpellDurationSkillDivisor ConfigFloat `yaml:"LightDarknessSpellDurationSkillDivisor"` // default 20
}
```

- [ ] **Step 4: Default them**

In `internal/configs/config.balance.lighting.go`, replace

```go
		{&b.LightInfraSpellBase, 5}, {&b.LightInfraSpellStatDivisor, 7}, {&b.LightInfraSpellSkillDivisor, 3},
	} {
```

with

```go
		{&b.LightInfraSpellBase, 5}, {&b.LightInfraSpellStatDivisor, 7}, {&b.LightInfraSpellSkillDivisor, 3},
		{&b.LightDarknessSpellStrengthBase, 40}, {&b.LightDarknessSpellStrengthStatDivisor, 10}, {&b.LightDarknessSpellStrengthSkillDivisor, 2},
		{&b.LightDarknessSpellDurationBase, 2}, {&b.LightDarknessSpellDurationStatDivisor, 50}, {&b.LightDarknessSpellDurationSkillDivisor, 20},
	} {
```

- [ ] **Step 5: Expose them on `configs.Lighting` (K10: the accessor drops the `Light` prefix)**

In `internal/configs/config.lighting_accessor.go`, replace

```go
	InfraSpellBase, InfraSpellStatDivisor, InfraSpellSkillDivisor                   float64

	InfraReachCap     int
```

with

```go
	InfraSpellBase, InfraSpellStatDivisor, InfraSpellSkillDivisor                   float64

	DarknessSpellStrengthBase, DarknessSpellStrengthStatDivisor, DarknessSpellStrengthSkillDivisor float64
	DarknessSpellDurationBase, DarknessSpellDurationStatDivisor, DarknessSpellDurationSkillDivisor float64

	InfraReachCap     int
```

and replace

```go
		InfraSpellSkillDivisor:       float64(b.LightInfraSpellSkillDivisor),
		InfraReachCap:                int(b.LightInfraReachCap),
		InfraPenaltyFloor:            float64(b.LightInfraPenaltyFloor),
		DarkCap:                      float64(b.DarknessCombatPenalty),
	}
```

with

```go
		InfraSpellSkillDivisor:       float64(b.LightInfraSpellSkillDivisor),
		InfraReachCap:                int(b.LightInfraReachCap),
		InfraPenaltyFloor:            float64(b.LightInfraPenaltyFloor),
		DarkCap:                      float64(b.DarknessCombatPenalty),

		DarknessSpellStrengthBase:         float64(b.LightDarknessSpellStrengthBase),
		DarknessSpellStrengthStatDivisor:  float64(b.LightDarknessSpellStrengthStatDivisor),
		DarknessSpellStrengthSkillDivisor: float64(b.LightDarknessSpellStrengthSkillDivisor),
		DarknessSpellDurationBase:         float64(b.LightDarknessSpellDurationBase),
		DarknessSpellDurationStatDivisor:  float64(b.LightDarknessSpellDurationStatDivisor),
		DarknessSpellDurationSkillDivisor: float64(b.LightDarknessSpellDurationSkillDivisor),
	}
```

- [ ] **Step 6: Run the Go-default tests**

Run: `go test ./internal/configs/ -run "Lighting5d|Carries5d" -count=1`
Expected: FAIL in `TestLighting5dKnobsShipInConfigYaml` only, six lines `config.yaml Balance has no LightDarknessSpell...`; the other three tests pass.

- [ ] **Step 7: Ship the keys in `config.yaml`, built from the blob**

Never edit `_datafiles/config.yaml` from disk and never with a Python read-modify-write (F34, CLAUDE.md tripwires). Write the block to the scratchpad, then build the file from the `HEAD` blob with the block inserted after the 5c block's last key:

```bash
cat > "$TMP/5d-config-block.yaml" <<'BLOCK'

  # ── LIGHT: DARKNESS SPELL (lighting plan 5d) ───────────────────────────────
  # Chrysalis Pall (condition darkness_strength: magnitude) scales like the
  # light spells above, and on its own knobs so darkness can be retuned apart
  # from light:
  #   base + stat/StatDivisor + spellcasting/SkillDivisor
  # for the same shape of duration. Shipped at glow's values: a new (100, 0),
  # mid (130, 30) and endgame (175, 65) caster darkens by 50 / 68 / 90, the
  # new caster for 20 minutes and the endgame caster for about 45. Not capped:
  # the scale clamps the room at -100. Zero or negative reverts to the default.
  LightDarknessSpellStrengthBase: 40
  LightDarknessSpellStrengthStatDivisor: 10
  LightDarknessSpellStrengthSkillDivisor: 2
  LightDarknessSpellDurationBase: 2
  LightDarknessSpellDurationStatDivisor: 50
  LightDarknessSpellDurationSkillDivisor: 20
BLOCK
git show HEAD:_datafiles/config.yaml | sed "/^  LightInfraPenaltyFloor: 0.90\$/r $TMP/5d-config-block.yaml" > _datafiles/config.yaml
git diff --stat _datafiles/config.yaml
```

Expected: ` _datafiles/config.yaml | 16 ++++++++++++++++` and `1 file changed, 16 insertions(+)`; `git diff _datafiles/config.yaml` shows only the block, between `  LightInfraPenaltyFloor: 0.90` and the blank line before `# ── COMBAT: DAMAGE`. If `git diff` shows any other line, stop: the blob and the sed anchor disagree.

- [ ] **Step 8: Run the package**

Run: `go test ./internal/configs/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/configs`.

- [ ] **Step 9: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.lighting.go internal/configs/config.lighting_accessor.go internal/configs/config_lighting_5d_test.go _datafiles/config.yaml
git commit -m "feat(configs): LightDarknessSpell knobs at glow's values (5d Rule 4)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Then confirm the commit carries exactly the block: `git show --stat HEAD -- _datafiles/config.yaml` reads `16 insertions(+)`. In this worktree `git ls-files -v _datafiles/config.yaml` prints `H`; the main checkout keeps its own `S` bit, so after the PR merges and the owner pulls, `git ls-files -v _datafiles/config.yaml` in the main checkout still prints `S` (the bit is per index and nothing here touches it).

---

### Task 3: Darkness is a record kind: `EffectDarknessStrength` (ruling D1) and its spell scaling

**Model:** sonnet (a new effect kind across five files, a shared helper moved under a spell hook, and one `file|line` guard re-key).

**Files:**
- Modify: `internal/conditions/effects.go` (`:36-49`, `:69-73`, `:122-130`, `:171-181`, `:201-204`)
- Modify: `internal/conditions/conditionspec.go` (`Adjustable` doc `:106-109`, `IsLightSource` `:248-251`)
- Modify: `internal/conditions/conditions.go:367` (the reset)
- Modify: `internal/conditions/light.go` (whole file)
- Modify: `internal/conditions/scaled_magnitude.go` (whole file)
- Modify: `internal/hooks/light_spell.go` (whole file)
- Modify: `condition_apply_path_guard_test.go:263` (re-key, F10)
- Create: `internal/conditions/darkness_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/conditions/darkness_test.go`. It reuses `testLanternId` from `light_test.go` (a literal light 54, adjustable); its own ids 9711 to 9718 are free in the test registry.

```go
package conditions

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

const (
	testDarkLanternId = 9711 // literal darkness 50, adjustable: the Umbral Dark's shape
	testPallId        = 9712 // magnitude darkness, adjustable, cancellable: the pall's shape
)

func seedDarknessSpecs(t *testing.T) {
	t.Helper()
	t.Cleanup(SeedConditionsForTest(map[int]*ConditionSpec{
		testLanternId: {ConditionId: testLanternId, Name: "Test Lantern", TriggerCount: 1, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectLightStrength: {Literal: 54}},
			Flags:   []Flag{Adjustable}},
		testDarkLanternId: {ConditionId: testDarkLanternId, Name: "Test Dark Lantern", TriggerCount: 1, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {Literal: 50}},
			Flags:   []Flag{Adjustable}},
		testPallId: {ConditionId: testPallId, Name: "Test Pall", TriggerCount: 4, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {UsesMagnitude: true}},
			Flags:   []Flag{Adjustable, Cancellable}},
	}))
}

// A darkness record is never a light: LightSources skips it, DarknessSources
// finds it, and the combined walk keeps held order across both kinds.
func TestDarknessIsItsOwnKindOfSource(t *testing.T) {
	seedDarknessSpecs(t)
	bs := New()
	bs.AddCondition(testDarkLanternId, true)
	bs.AddCondition(testLanternId, true)

	if got := bs.LightSources(); len(got) != 1 || got[0].ConditionId != testLanternId {
		t.Fatalf("LightSources = %v, want only the lantern", got)
	}
	if got := bs.DarknessSources(); len(got) != 1 || got[0].ConditionId != testDarkLanternId {
		t.Fatalf("DarknessSources = %v, want only the dark lantern", got)
	}
	both := bs.LightAndDarknessSources()
	if len(both) != 2 || both[0].ConditionId != testDarkLanternId || both[1].ConditionId != testLanternId {
		t.Fatalf("LightAndDarknessSources = %v, want dark lantern then lantern (held order)", both)
	}
	if !GetConditionSpec(testDarkLanternId).IsDarknessSource() || GetConditionSpec(testDarkLanternId).IsLightSource() {
		t.Error("the dark lantern must be a darkness source and not a light source")
	}
	if !AnyDarknessSource([]int{testLanternId, testDarkLanternId}) || AnyDarknessSource([]int{testLanternId, 424242}) {
		t.Error("AnyDarknessSource must find the dark lantern and skip lights and unknown ids")
	}
}

// LightMax and LightNow read darkness_strength for a darkness record, through
// the same trim states a light uses.
func TestDarknessRecordStates(t *testing.T) {
	seedDarknessSpecs(t)
	bs := New()
	bs.AddCondition(testDarkLanternId, true)
	rec := bs.DarknessSources()[0]
	spec := GetConditionSpec(testDarkLanternId)

	if got := rec.LightMax(spec); got != 50 {
		t.Fatalf("LightMax = %v, want 50", got)
	}
	if v, ok := rec.LightNow(spec); !ok || v != 50 {
		t.Fatalf("fresh dark lantern = (%v, %v), want (50, true)", v, ok)
	}
	rec.SetLightOutput(20)
	if v, ok := rec.LightNow(spec); !ok || v != 20 {
		t.Errorf("trimmed dark lantern = (%v, %v), want (20, true)", v, ok)
	}
	rec.SetLightOutput(math.Inf(-1))
	if _, ok := rec.LightNow(spec); ok {
		t.Error("a darkness trimmed to nothing still takes light away")
	}
	rec.ResetLight()
	if v, ok := rec.LightNow(spec); !ok || v != 50 {
		t.Errorf("reset dark lantern = (%v, %v), want (50, true)", v, ok)
	}
}

// A recast pall is a fresh source at full strength, as a recast glow is.
func TestFreshMagnitudeResetsADarkness(t *testing.T) {
	seedDarknessSpecs(t)
	bs := New()
	bs.AddConditionMagnitude(testPallId, 4, 68)
	rec := bs.DarknessSources()[0]
	rec.SetLightOutput(10)
	bs.AddConditionMagnitude(testPallId, 4, 68)
	if v, ok := rec.LightNow(GetConditionSpec(testPallId)); !ok || v != 68 {
		t.Errorf("recast pall = (%v, %v), want (68, true)", v, ok)
	}
}

func TestEffectNeverAggregatesDarkness(t *testing.T) {
	seedDarknessSpecs(t)
	bs := New()
	bs.AddCondition(testDarkLanternId, true)
	if got := bs.Effect(EffectDarknessStrength); got != 0 {
		t.Errorf("Effect(darkness_strength) = %v, want 0: darkness is per record", got)
	}
}

func TestDarknessSpecValidation(t *testing.T) {
	good := &ConditionSpec{ConditionId: 9713, Name: "Good", TriggerCount: 1, RoundInterval: 1,
		Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {Literal: 50}}, Flags: []Flag{Adjustable}}
	if err := good.Validate(); err != nil {
		t.Fatalf("control: an adjustable literal darkness should validate, got %v", err)
	}
	cases := map[string]*ConditionSpec{
		"a darkness_strength of 0": {ConditionId: 9714, Name: "Bad", TriggerCount: 1, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {Literal: 0}}},
		"both light and darkness": {ConditionId: 9715, Name: "Bad", TriggerCount: 1, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectLightStrength: {Literal: 40}, EffectDarknessStrength: {Literal: 40}}},
		"a stacking darkness": {ConditionId: 9716, Name: "Bad", TriggerRate: "1 round", TriggerCount: 4, RoundInterval: 1,
			Flags: []Flag{Stacking}, TickPool: "health", TickFromMagnitude: true,
			Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {Literal: 5}}},
		"adjustable with neither kind": {ConditionId: 9717, Name: "Bad", TriggerCount: 1, RoundInterval: 1, Flags: []Flag{Adjustable}},
		"two magnitude kinds": {ConditionId: 9718, Name: "Bad", TriggerCount: 1, RoundInterval: 1,
			Effects: map[EffectKind]EffectValue{EffectDarknessStrength: {UsesMagnitude: true}, EffectInfraReach: {UsesMagnitude: true}}},
	}
	for name, spec := range cases {
		if err := spec.Validate(); err == nil {
			t.Errorf("%s validated", name)
		}
	}
}

// The darkness spell reads its own knob trios, magnitude and duration, at the
// three reference casters (spec Rule 4).
func TestDarknessSpellScalesOnItsOwnKnobs(t *testing.T) {
	configs.SetConfigForTest(t, configs.GetConfig())
	cases := []struct {
		stat, skill  float64
		wantStrength float64
		wantTriggers int
	}{
		{100, 0, 50, 4},
		{130, 30, 68, 6},
		{175, 65, 90, 9},
	}
	for _, c := range cases {
		if got := SpellScaledMagnitude(EffectDarknessStrength, c.stat, c.skill); got != c.wantStrength {
			t.Errorf("(%v, %v): magnitude %v, want %v", c.stat, c.skill, got, c.wantStrength)
		}
		if got := SpellScaledTriggers(EffectDarknessStrength, c.stat, c.skill); got != c.wantTriggers {
			t.Errorf("(%v, %v): triggers %d, want %d", c.stat, c.skill, got, c.wantTriggers)
		}
	}

	// Move one knob of each darkness trio: darkness follows, light does not.
	cfg := configs.GetConfig()
	cfg.Balance.LightDarknessSpellStrengthBase = 10
	cfg.Balance.LightDarknessSpellDurationBase = 12
	configs.SetConfigForTest(t, cfg)
	if got := SpellScaledMagnitude(EffectDarknessStrength, 100, 0); got != 20 {
		t.Errorf("darkness base 10: magnitude %v, want 20", got)
	}
	if got := SpellScaledTriggers(EffectDarknessStrength, 100, 0); got != 14 {
		t.Errorf("darkness duration base 12: triggers %d, want 14", got)
	}
	if got := SpellScaledMagnitude(EffectLightStrength, 100, 0); got != 50 {
		t.Errorf("light moved with the darkness knob: %v, want 50", got)
	}
	if got := SpellScaledTriggers(EffectLightStrength, 100, 0); got != 4 {
		t.Errorf("light duration moved with the darkness knob: %d, want 4", got)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/conditions/ -run "Darkness" -count=1`
Expected: FAIL to build, `undefined: EffectDarknessStrength` (and `DarknessSources`, `LightAndDarknessSources`, `IsDarknessSource`, `AnyDarknessSource`, `SpellScaledTriggers`).

- [ ] **Step 3: The effect kind (`effects.go`)**

Replace

```go
	EffectLightStrength EffectKind = `light_strength`
)

// AllEffectKinds is the closed set, for validation and docs.
var AllEffectKinds = []EffectKind{
	EffectDamageMult, EffectDefenseMult, EffectDodgeMult, EffectRegenMult,
	EffectMitigationFlat, EffectPoolMaxPct, EffectAttacksCap,
	EffectNightVisionStrength, EffectInfraReach, EffectLightStrength,
}
```

with

```go
	EffectLightStrength EffectKind = `light_strength`
	// EffectDarknessStrength is a darkness source's full strength: light
	// taken away from the room, a literal for an item, "magnitude" for a
	// spell (lighting plan 5d, ruling D1). It is a light record with
	// darkening polarity: it shares the record's trim state (LightTrim,
	// LightOutput, ResetLight) and is read through LightMax and LightNow, but
	// IsLightSource stays false for it, so EmitsLight, hood, the as-lit end
	// line and every other light reader exclude it by construction. Read
	// through Conditions.DarknessSources, never Effect().
	EffectDarknessStrength EffectKind = `darkness_strength`
)

// AllEffectKinds is the closed set, for validation and docs.
var AllEffectKinds = []EffectKind{
	EffectDamageMult, EffectDefenseMult, EffectDodgeMult, EffectRegenMult,
	EffectMitigationFlat, EffectPoolMaxPct, EffectAttacksCap,
	EffectNightVisionStrength, EffectInfraReach, EffectLightStrength,
	EffectDarknessStrength,
}
```

Replace

```go
// ScaledKinds are the effect kinds a spell or potion scales from its source
// (lighting plan 5c): a light's strength, nightvision's strength, and infra's
// reach. A record carries one Magnitude, so a condition may declare at most
// one of them as "magnitude" (validateEffects refuses two).
var ScaledKinds = []EffectKind{EffectLightStrength, EffectNightVisionStrength, EffectInfraReach}
```

with

```go
// ScaledKinds are the effect kinds a spell or potion scales from its source
// (lighting plan 5c): a light's strength, nightvision's strength, infra's
// reach, and a darkness's strength (5d). A record carries one Magnitude, so a
// condition may declare at most one of them as "magnitude" (validateEffects
// refuses two).
var ScaledKinds = []EffectKind{EffectLightStrength, EffectNightVisionStrength, EffectInfraReach, EffectDarknessStrength}
```

Replace

```go
// literal light_strength of zero or less, an adjustable record that
// declares no light_strength, and a stacking record that is also a light
// source (a stack's summed magnitude is not a light strength, and
```

with

```go
// literal light_strength or darkness_strength of zero or less, a record
// declaring both (lighting plan 5d: one polarity per record), an adjustable
// record that declares neither, and a stacking record that is also a light
// or darkness source (a stack's summed magnitude is not a light strength, and
```

Replace

```go
	if slices.Contains(b.Flags, Adjustable) {
		if _, ok := b.Effects[EffectLightStrength]; !ok {
			return fmt.Errorf("conditionId %d (%s) is adjustable but declares no light_strength", b.ConditionId, b.Name)
		}
	}
	if b.IsStacking() && b.IsLightSource() {
		return fmt.Errorf("conditionId %d (%s) is a stacking record and a light source; a stack's summed magnitude is not a light strength", b.ConditionId, b.Name)
	}
```

with

```go
	if v, ok := b.Effects[EffectDarknessStrength]; ok && !v.UsesMagnitude && v.Literal <= 0 {
		return fmt.Errorf("conditionId %d (%s) declares darkness_strength %v; a darkness must take some light away", b.ConditionId, b.Name, v.Literal)
	}
	if b.IsLightSource() && b.IsDarknessSource() {
		return fmt.Errorf("conditionId %d (%s) declares both light_strength and darkness_strength; a record has one polarity", b.ConditionId, b.Name)
	}
	if slices.Contains(b.Flags, Adjustable) && !b.IsLightSource() && !b.IsDarknessSource() {
		return fmt.Errorf("conditionId %d (%s) is adjustable but declares no light_strength or darkness_strength", b.ConditionId, b.Name)
	}
	if b.IsStacking() && b.IsLightSource() {
		return fmt.Errorf("conditionId %d (%s) is a stacking record and a light source; a stack's summed magnitude is not a light strength", b.ConditionId, b.Name)
	}
	if b.IsStacking() && b.IsDarknessSource() {
		return fmt.Errorf("conditionId %d (%s) is a stacking record and a darkness source; a stack's summed magnitude is not a darkness strength", b.ConditionId, b.Name)
	}
```

Replace

```go
	if kind == EffectLightStrength {
		// Per-record, never aggregated: see LightSources.
		return 0
	}
```

with

```go
	if kind == EffectLightStrength || kind == EffectDarknessStrength {
		// Per-record, never aggregated: see LightSources and DarknessSources.
		return 0
	}
```

- [ ] **Step 4: `IsDarknessSource`, `AnyDarknessSource` and the flag doc (`conditionspec.go`)**

Replace

```go
	// each time the bearer enters a room (lighting plan 5a). It requires the
	// light_strength effect.
```

with

```go
	// each time the bearer enters a room (lighting plan 5a), or a darkness
	// source that trims itself to its bearer's usable range (5d). It requires
	// the light_strength or the darkness_strength effect.
```

Replace

```go
// IsLightSource reports whether a record of this spec sheds light.
func (b *ConditionSpec) IsLightSource() bool {
	_, ok := b.Effects[EffectLightStrength]
	return ok
}
```

with

```go
// IsLightSource reports whether a record of this spec sheds light. A darkness
// source is not a light (lighting plan 5d, ruling D1): this stays light-only,
// so every light reader excludes darkness by construction.
func (b *ConditionSpec) IsLightSource() bool {
	_, ok := b.Effects[EffectLightStrength]
	return ok
}

// IsDarknessSource reports whether a record of this spec takes light away
// from its room (lighting plan 5d).
func (b *ConditionSpec) IsDarknessSource() bool {
	_, ok := b.Effects[EffectDarknessStrength]
	return ok
}

// AnyDarknessSource reports whether any of the condition ids names a
// darkness source: an item whose worn conditions darken its room announces
// itself as lit (lighting plan 5d, ruling D6). Unknown ids are skipped.
func AnyDarknessSource(conditionIds []int) bool {
	for _, id := range conditionIds {
		if spec := GetConditionSpec(id); spec != nil && spec.IsDarknessSource() {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: A fresh magnitude resets a darkness too (`conditions.go`)**

Replace

```go
		if spec.IsLightSource() {
			// A fresh magnitude is a fresh cast: full strength, hood open.
```

with

```go
		if spec.IsLightSource() || spec.IsDarknessSource() {
			// A fresh magnitude is a fresh cast: full strength, hood open.
```

- [ ] **Step 6: `LightMax` reads either kind; the two walks (`light.go`)**

Replace the whole of `internal/conditions/light.go` with:

```go
package conditions

import "math"

// LightTrim is where an adjustable light record's output stands. It is an
// explicit state rather than a stored -Inf, so a save never has to encode an
// infinity.
type LightTrim string

const (
	LightFull    LightTrim = ""        // untrimmed: full strength
	LightTrimmed LightTrim = "trimmed" // running at LightOutput
	LightOff     LightTrim = "off"     // trimmed to nothing: the room was bright enough
)

// LightMax is the record's full strength: the applier's magnitude for a spell
// source, the authored number for an item. It reads whichever of
// light_strength and darkness_strength the spec declares (validateEffects
// refuses both), so one record shape and one trim state serve either
// polarity (lighting plan 5d, ruling D1); the caller knows which it holds
// from IsLightSource / IsDarknessSource. 0 when spec declares neither. A
// magnitude source added with no magnitude (an admin setcondition) is 0 and
// adds nothing; cast the spell instead.
func (b *Condition) LightMax(spec *ConditionSpec) float64 {
	if spec == nil {
		return 0
	}
	v, ok := spec.Effects[EffectLightStrength]
	if !ok {
		v, ok = spec.Effects[EffectDarknessStrength]
	}
	if !ok {
		return 0
	}
	if v.UsesMagnitude {
		return b.Magnitude
	}
	return v.Literal
}

// LightNow is the term this record adds to its room's light right now, and
// false when it adds none: expired, hooded, trimmed to nothing, or strengthless.
func (b *Condition) LightNow(spec *ConditionSpec) (float64, bool) {
	if b.Expired() || b.Hooded {
		return 0, false
	}
	max := b.LightMax(spec)
	if max <= 0 {
		return 0, false
	}
	switch b.LightTrim {
	case LightOff:
		return 0, false
	case LightTrimmed:
		// lightscale.Trim never produces a negative output or one above full
		// strength, but a hand-edited or future save could. Neither may
		// enter the combine as stored.
		if b.LightOutput < 0 {
			return 0, false
		}
		return math.Min(b.LightOutput, max), true
	case LightFull:
		return max, true
	default:
		// An unrecognised state from a save is treated as full strength:
		// fail open, deliberately, so a bad save never snuffs a light.
		return max, true
	}
}

// SetLightOutput records a trim result from lightscale.Trim. Any non-finite
// output means the room needs nothing from this source and lands on LightOff:
// -Inf (lightscale.Absent), and equally +Inf and NaN.
func (b *Condition) SetLightOutput(out float64) {
	if math.IsInf(out, 0) || math.IsNaN(out) {
		b.LightTrim, b.LightOutput = LightOff, 0
		return
	}
	b.LightTrim, b.LightOutput = LightTrimmed, out
}

// ResetLight returns the record to full strength with any hood open: a fresh
// cast, a fresh equip, or unhood. Only AddConditionMagnitude calls it
// automatically. An equip path must call it itself (Character.Wear in
// internal/characters/worn.go does), and a plain AddCondition that revives an
// expired-but-unpruned record keeps that record's old hood and trim.
func (b *Condition) ResetLight() {
	b.LightTrim, b.LightOutput, b.Hooded = LightFull, 0, false
}

// LightSources returns every held, unexpired record whose spec declares a
// light strength, in held order. The order is stable, so a bearer's several
// sources always trim in the same sequence.
func (bs *Conditions) LightSources() []*Condition {
	var out []*Condition
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetConditionSpec(b.ConditionId); spec != nil && spec.IsLightSource() {
			out = append(out, b)
		}
	}
	return out
}

// DarknessSources returns every held, unexpired record whose spec declares a
// darkness strength, in held order (lighting plan 5d). It is LightSources'
// twin: a darkness is never in LightSources.
func (bs *Conditions) DarknessSources() []*Condition {
	var out []*Condition
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetConditionSpec(b.ConditionId); spec != nil && spec.IsDarknessSource() {
			out = append(out, b)
		}
	}
	return out
}

// LightAndDarknessSources returns every held, unexpired light or darkness
// record in one held order, for the one trim pass that solves both
// polarities in turn (internal/rooms.(*Room).TrimLightFor, lighting plan 5d).
func (bs *Conditions) LightAndDarknessSources() []*Condition {
	var out []*Condition
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetConditionSpec(b.ConditionId); spec != nil && (spec.IsLightSource() || spec.IsDarknessSource()) {
			out = append(out, b)
		}
	}
	return out
}
```

- [ ] **Step 7: The darkness trios (`scaled_magnitude.go`)**

Replace the whole of `internal/conditions/scaled_magnitude.go` with:

```go
package conditions

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// SpellScaledMagnitude is the magnitude a spell applies a condition reading
// kind (one of ScaledKinds) at, for a caster with the given primary-stat value
// and spellcasting skill: base + stat/statDivisor + skill/skillDivisor, from
// that kind's knob trio in configs.GetLightingConfig() (LightSpellStrength* for
// light, LightNightVisionSpell* for nightvision, LightInfraSpell* for infra
// reach, LightDarknessSpellStrength* for darkness), capped by
// CapScaledMagnitude so the record holds the value it acts at.
//
// internal/hooks' spell path calls it with the caster's numbers; the admin
// setcondition command calls it at a new character's (stat 100, skill 0), so a
// magnitude-scaled condition applied by hand lands at a real strength rather
// than zero.
func SpellScaledMagnitude(kind EffectKind, stat, skill float64) float64 {
	cfg := configs.GetLightingConfig()
	base, statDiv, skillDiv := cfg.SpellStrengthBase, cfg.SpellStrengthStatDivisor, cfg.SpellStrengthSkillDivisor
	switch kind {
	case EffectNightVisionStrength:
		base, statDiv, skillDiv = cfg.NightVisionSpellBase, cfg.NightVisionSpellStatDivisor, cfg.NightVisionSpellSkillDivisor
	case EffectInfraReach:
		base, statDiv, skillDiv = cfg.InfraSpellBase, cfg.InfraSpellStatDivisor, cfg.InfraSpellSkillDivisor
	case EffectDarknessStrength:
		base, statDiv, skillDiv = cfg.DarknessSpellStrengthBase, cfg.DarknessSpellStrengthStatDivisor, cfg.DarknessSpellStrengthSkillDivisor
	}
	return CapScaledMagnitude(kind, base+stat/statDiv+skill/skillDiv)
}

// SpellScaledTriggers is the trigger count a spell applies a condition
// reading kind at: base + stat/statDivisor + skill/skillDivisor, rounded, at
// least 1. Darkness reads its own LightDarknessSpellDuration* trio (lighting
// plan 5d, ruling D9); light, nightvision and infra reach share the light
// trio LightSpellDuration*, as 5a and 5c shipped.
func SpellScaledTriggers(kind EffectKind, stat, skill float64) int {
	cfg := configs.GetLightingConfig()
	base, statDiv, skillDiv := cfg.SpellDurationBase, cfg.SpellDurationStatDivisor, cfg.SpellDurationSkillDivisor
	if kind == EffectDarknessStrength {
		base, statDiv, skillDiv = cfg.DarknessSpellDurationBase, cfg.DarknessSpellDurationStatDivisor, cfg.DarknessSpellDurationSkillDivisor
	}
	triggers := int(math.Round(base + stat/statDiv + skill/skillDiv))
	if triggers < 1 {
		triggers = 1
	}
	return triggers
}

// CapScaledMagnitude bounds a scaled kind's magnitude to the most it can act
// at: infra reach at LightInfraReachCap, nightvision strength at
// configs.LightWindowShiftCap (the window model clamps any strength there
// anyway, so a record above it would only misreport itself). Light is not
// capped here; the light scale clamps the room's composed light instead. The
// spell and potion paths both apply it.
func CapScaledMagnitude(kind EffectKind, magnitude float64) float64 {
	limit := -1.0
	switch kind {
	case EffectInfraReach:
		limit = float64(configs.GetLightingConfig().InfraReachCap)
	case EffectNightVisionStrength:
		limit = configs.LightWindowShiftCap
	}
	if limit >= 0 && magnitude > limit {
		return limit
	}
	return magnitude
}

// NewCharacterSpellStat and NewCharacterSpellSkill are the caster numbers
// SpellScaledMagnitude is evaluated at when no caster exists, such as the
// admin setcondition command: a new character's stat (stats centre on 100)
// and an untrained skill.
const (
	NewCharacterSpellStat  = 100.0
	NewCharacterSpellSkill = 0.0
)
```

- [ ] **Step 8: Run the conditions package**

Run: `go test ./internal/conditions/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/conditions` (the new tests and every existing light test, `TestEveryEffectKindIsClassifiedExactlyOnce` included, F7).

- [ ] **Step 9: The spell hook reads the duration by kind (`light_spell.go`)**

Replace the whole of `internal/hooks/light_spell.go` with the file below. The duration moves into `conditions.SpellScaledTriggers`, so `math` and `configs` leave the imports.

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// magnitudeSpellApplication reports how a spell's condition should be applied
// when it reads one of conditions.ScaledKinds from its magnitude: at a value
// and a duration scaled from the CASTER's primary stat and spellcasting skill
// (lighting plan 5a for light, 5c for nightvision and infra reach, 5d for
// darkness). Each kind has its own base + stat/D1 + skill/D2 trio
// (conditions.SpellScaledMagnitude, shared with the admin setcondition
// command); the duration comes from conditions.SpellScaledTriggers, where
// darkness reads its own trio and the other three share the light duration
// trio. Infra reach is capped at LightInfraReachCap and nightvision
// at the window shift cap there, so the record holds the value it acts at.
// ok is false for any other condition, which keeps its authored
// application. A light then trims to its HOLDER's eyes, who may not be the
// caster.
func magnitudeSpellApplication(spellData *spells.SpellData, caster *characters.Character, conditionId int) (magnitude float64, triggers int, ok bool) {
	if spellData == nil || caster == nil {
		return 0, 0, false
	}
	spec := conditions.GetConditionSpec(conditionId)
	if spec == nil {
		return 0, 0, false
	}
	kind, scaled := spec.ScaledKind()
	if !scaled {
		return 0, 0, false
	}
	stat := float64(spellData.CasterStatValue(caster.Stats))
	skill := float64(caster.GetSkillLevel(skills.Spellcasting))
	magnitude = conditions.SpellScaledMagnitude(kind, stat, skill)
	triggers = conditions.SpellScaledTriggers(kind, stat, skill)
	return magnitude, triggers, true
}

// spellConditionTarget is what a spell condition lands on: a player record or
// a mob, both of which queue the narrating events.Condition.
type spellConditionTarget interface {
	AddCondition(conditionId int, source string)
	AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string)
	AddConditionTickScaled(conditionId int, scale float64, source string)
}

// applySpellCondition applies one of a spell's conditions to its target: a
// magnitude-scaled light or sight at the caster's scaled value and duration,
// a heal- or damage-over-time at the caster's spellTickScale (the apply hook
// computes the amount where it lands), anything else at its authored values.
// Every door queues events.Condition, so the holder reads the start notice
// either way.
func applySpellCondition(target spellConditionTarget, spellData *spells.SpellData, caster *characters.Character, conditionId int) {
	if mag, trig, ok := magnitudeSpellApplication(spellData, caster, conditionId); ok {
		target.AddConditionMagnitude(conditionId, trig, mag, "spell")
		return
	}
	if spec := conditions.GetConditionSpec(conditionId); spec != nil && spec.TickPool != "" {
		target.AddConditionTickScaled(conditionId, spellTickScale(caster), "spell")
		return
	}
	target.AddCondition(conditionId, "spell")
}
```

- [ ] **Step 10: See the guard name the moved line, then re-key it**

Run: `go test . -run TestPlayerConditionsTravelTheEventPath -count=1`
Expected: FAIL with two problems: `internal/hooks/light_spell.go|58: target.AddConditionMagnitude(conditionId, trig, mag, "spell")` (unallowlisted) and `internal/hooks/light_spell.go|63: allowlisted but no direct condition add is on that line any more` (F10).

In `condition_apply_path_guard_test.go`, replace

```go
	"internal/hooks/light_spell.go|63": "light or sight spell at the caster's scaled magnitude and triggers: the EVENT door (users.UserRecord / mobs.Mob AddConditionMagnitude both queue events.Condition); listed only because arity cannot tell it from the silent character door",
```

with

```go
	"internal/hooks/light_spell.go|58": "light, sight or darkness spell at the caster's scaled magnitude and triggers: the EVENT door (users.UserRecord / mobs.Mob AddConditionMagnitude both queue events.Condition); listed only because arity cannot tell it from the silent character door",
```

(The key keeps its length, so `gofmt` realigns nothing.)

- [ ] **Step 11: Run the hook tests and the guard**

Run: `go build ./... && go test ./internal/hooks/ -run "Light|Spell" -count=1 && go test . -run TestPlayerConditionsTravelTheEventPath -count=1`
Expected: `ok` for `internal/hooks` (the existing `TestLightSpellScalesFromStatAndSkill` and `TestLightSpellReadsTheKnobs` still read 50 for 4 and 90 for 9) and `ok` for the root package.

- [ ] **Step 12: Commit**

```bash
git add internal/conditions/effects.go internal/conditions/conditionspec.go internal/conditions/conditions.go internal/conditions/light.go internal/conditions/scaled_magnitude.go internal/conditions/darkness_test.go internal/hooks/light_spell.go condition_apply_path_guard_test.go
git commit -m "feat(conditions): darkness_strength, a light record with darkening polarity (5d D1)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: The bottom of the usable range: `messaging.DarknessTrimTarget` (Rule 3)

**Model:** haiku (one pure function, code given).

**Files:**
- Modify: `internal/messaging/window.go` (after `LightTrimTarget`, `:95-97`)
- Create: `internal/messaging/darkness_trim_target_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/messaging/darkness_trim_target_test.go`:

```go
package messaging

import "testing"

// The bottom of an observer's usable range, the floor a darkness trims to
// (lighting plan 5d, Rule 3): the blind edge for normal eyes, the shifted
// blind edge floored at windowFloor for nightvision, minus the reach for
// infravision.
func TestDarknessTrimTarget(t *testing.T) {
	cases := []struct {
		name            string
		strength, reach int
		want            float64
	}{
		{"normal eyes", 0, 0, 25},
		{"nightvision 12", 12, 0, 13},
		{"nightvision 24: the window floor", 24, 0, 1},
		{"nightvision past the cap clamps", 40, 0, 1},
		{"infravision 30", 12, 30, -30},
		{"infravision 50, no nightvision", 0, 50, -50},
	}
	for _, c := range cases {
		if got := DarknessTrimTarget(c.strength, c.reach, 25); got != c.want {
			t.Errorf("%s: DarknessTrimTarget(%d, %d, 25) = %v, want %v", c.name, c.strength, c.reach, got, c.want)
		}
	}
}

// The floor is a usable edge: an observer standing exactly on it still reads
// shapes, and one point below reads nothing.
func TestDarknessTrimTargetIsTheUsableEdge(t *testing.T) {
	for _, c := range []struct{ strength, reach int }{{0, 0}, {24, 0}, {0, 50}} {
		floor := int(DarknessTrimTarget(c.strength, c.reach, 25))
		if got := SightThroughWindow(floor, c.strength, c.reach, 25, 50); got == SightNone {
			t.Errorf("strength %d reach %d: the floor %d reads nothing", c.strength, c.reach, floor)
		}
		if got := SightThroughWindow(floor-1, c.strength, c.reach, 25, 50); got != SightNone {
			t.Errorf("strength %d reach %d: one below the floor (%d) still reads %v", c.strength, c.reach, floor-1, got)
		}
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/messaging/ -run DarknessTrimTarget -count=1`
Expected: FAIL to build, `undefined: DarknessTrimTarget`.

- [ ] **Step 3: Add the function**

In `internal/messaging/window.go`, replace

```go
func LightTrimTarget(strength, dazzleAbove int) float64 {
	return float64(dazzleAbove - clampShift(strength) - 1)
}
```

with

```go
func LightTrimTarget(strength, dazzleAbove int) float64 {
	return float64(dazzleAbove - clampShift(strength) - 1)
}

// DarknessTrimTarget is the darkest room light an observer can still use: the
// floor an adjustable darkness trims to (lighting plan 5d). With infravision
// it is minus the reach, where heat still reads shapes; otherwise it is the
// shifted blind edge, but never below windowFloor, where natural shapes need
// light at least that high (SightThroughWindow). Exactly the edge, not a
// point inside it: the edge counts as usable, and a solved room within
// rounding of it rounds onto it. blindBelow is the caller's config knob
// (Lighting.BlindBelow).
func DarknessTrimTarget(strength, reach, blindBelow int) float64 {
	if reach > 0 {
		return float64(-reach)
	}
	floor := blindBelow - clampShift(strength)
	if floor < windowFloor {
		floor = windowFloor
	}
	return float64(floor)
}
```

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./internal/messaging/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/messaging`.

- [ ] **Step 5: Commit**

```bash
git add internal/messaging/window.go internal/messaging/darkness_trim_target_test.go
git commit -m "feat(messaging): DarknessTrimTarget, the bottom of an observer's usable range (5d Rule 3)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Light minus darkness, and both polarities trim (Rules 2 and 3, rulings D2 and D3)

**Model:** sonnet (the room composition and the trim, with a changed struct and a changed internal signature).

**Files:**
- Modify: `internal/rooms/lighting.go` (`LightLevel` doc `:23-24`, `LightTerms` `:59-76`, `composeLightExcluding` / `composeWith` `:89-146`, `carriedLight` `:148-174`)
- Modify: `internal/rooms/light_trim.go` (whole file)
- Modify: `internal/rooms/carried_light_test.go:17,18,25,31-33,43` (F4)
- Create: `internal/rooms/darkness_compose_test.go`, `internal/rooms/darkness_trim_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/rooms/darkness_compose_test.go` (the Rule 2 table through the pure `composeWith`, whose new fifth argument is the carried darkness terms):

```go
package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/lightscale"
)

// The Rule 2 table of the lighting plan 5d spec: lights combine, darknesses
// combine among themselves by the same halving rule, and the net is the
// light (0 when none) minus the darkness (0 when none), clamped at -100.
func TestDarknessComposition(t *testing.T) {
	cfg := modelCfg()
	zero, open := 0.0, 1.0
	cave := Room{SkyLight: &zero}
	tavern := Room{SkyLight: &zero, Lamp: LampPtr(50)}
	field := Room{SkyLight: &open}

	cases := []struct {
		name        string
		room        Room
		celestial   float64
		light, dark []float64
		want        int
	}{
		{"508 today, nobody carrying", cave, 60, nil, nil, 0},
		{"508 with the Phantom's lantern", cave, 60, nil, []float64{50}, -50},
		{"508, lantern, a torch", cave, 60, []float64{56}, []float64{50}, 6},
		{"508, lantern, endgame glow", cave, 60, []float64{90}, []float64{50}, 40},
		{"508, lantern, glow 90 and torch 56", cave, 60, []float64{90, 56}, []float64{50}, 41},
		{"tavern lamp 50, one darkness 50", tavern, 60, nil, []float64{50}, 0},
		{"open ground at noon, new caster's pall", field, 70, nil, []float64{50}, 20},
		{"open ground at noon, endgame pall", field, 70, nil, []float64{90}, -20},
		{"cave, two darkness 50", cave, 60, nil, []float64{50, 50}, -58},
		{"cave, darkness 50 and 40", cave, 60, nil, []float64{50, 40}, -54},
		{"cave, three endgame palls: clamped", cave, 60, nil, []float64{90, 90, 90}, -100},
	}
	for _, c := range cases {
		got := c.room.composeWith(cfg, c.celestial, 1, c.light, c.dark)
		if got.Level != c.want {
			t.Errorf("%s: Level = %d, want %d", c.name, got.Level, c.want)
		}
	}
}

// Light, Dark and Raw carry the two combines and the net; Carried and
// Darkened are independent.
func TestDarknessTermsAreReported(t *testing.T) {
	cfg := modelCfg()
	zero := 0.0
	cave := Room{SkyLight: &zero}

	dark := cave.composeWith(cfg, 60, 1, nil, []float64{50, 50})
	if !math.IsInf(dark.Light, -1) || dark.Carried {
		t.Errorf("darkness alone: Light %v Carried %v, want -Inf and false", dark.Light, dark.Carried)
	}
	if want := lightscale.Combine(cfg.DoublingStep, 50, 50); math.Abs(dark.Dark-want) > 1e-9 || !dark.Darkened {
		t.Errorf("darkness alone: Dark %v Darkened %v, want %v and true", dark.Dark, dark.Darkened, want)
	}
	if math.Abs(dark.Raw-(-dark.Dark)) > 1e-9 {
		t.Errorf("darkness alone: Raw %v, want %v (Absent light reads 0)", dark.Raw, -dark.Dark)
	}

	lit := cave.composeWith(cfg, 60, 1, []float64{56}, nil)
	if !lit.Carried || lit.Darkened || !math.IsInf(lit.Dark, -1) || lit.Light != 56 || lit.Raw != 56 {
		t.Errorf("a torch alone: %+v, want Carried, not Darkened, Dark -Inf, Light and Raw 56", lit)
	}

	both := cave.composeWith(cfg, 60, 1, []float64{56}, []float64{50})
	if !both.Carried || !both.Darkened || both.Light != 56 || both.Dark != 50 || both.Raw != 6 {
		t.Errorf("a torch and a darkness: %+v, want both flags, Light 56, Dark 50, Raw 6", both)
	}
}

// Darkness takes from sky, lamp and carried light alike: it subtracts from
// the combined light, not from any one term.
func TestDarknessSubtractsFromTheWholeCombine(t *testing.T) {
	cfg := modelCfg()
	open := 1.0
	room := Room{SkyLight: &open, Lamp: LampPtr(50)}
	light := lightscale.Combine(cfg.DoublingStep, 60, 50, 56)
	got := room.composeWith(cfg, 60, 1, []float64{56}, []float64{40})
	if want := int(math.Round(light - 40)); got.Level != want {
		t.Errorf("sky 60, lamp 50, torch 56, darkness 40: Level %d, want %d", got.Level, want)
	}
}
```

Create `internal/rooms/darkness_trim_test.go` (the Rule 3 table through a real room, the light row with darkness present, held order across both kinds, entry order across three bearers, and the spawn seam; `darkUsers` reseeds every user of a test together because `SeedUsersForTest` replaces the whole registry, F32):

```go
package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	darkUmbralId    = 9751 // literal darkness 50, adjustable: the Umbral Dark's shape
	darkFixedId     = 9752 // magnitude darkness, NOT adjustable: someone else's darkness
	darkSight24Id   = 9753 // nightvision 24
	darkInfra30Id   = 9754 // infra reach 30
	darkInfra50Id   = 9755 // infra reach 50
	darkHoodedId    = 9756 // literal light 54, adjustable: the hooded lantern's shape
	darkGlowId      = 9757 // magnitude light, adjustable: glow's shape
	darkFirstUser   = 7750
	darkFirstRoom   = 7770
	darkMobInst     = 7790
	darkMobId       = 7791
	darkTrimEpsilon = 1e-9
)

// seedDarknessTrim seeds the conditions every darkness trim test uses, on the
// shipped biomes and clock (the cave biome has no sky, so a cave room's light
// is its lamp alone).
func seedDarknessTrim(t *testing.T) *darkUsers {
	t.Helper()
	withShippedBiomesAndClock(t)
	requireBiome(t, "cave")
	lit := func(id int, name string, v conditions.EffectValue, kind conditions.EffectKind, adjustable bool) *conditions.ConditionSpec {
		s := &conditions.ConditionSpec{ConditionId: id, Name: name, TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{kind: v}}
		if adjustable {
			s.Flags = []conditions.Flag{conditions.Adjustable}
		}
		return s
	}
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		darkUmbralId:  lit(darkUmbralId, "Test Umbral", conditions.EffectValue{Literal: 50}, conditions.EffectDarknessStrength, true),
		darkFixedId:   lit(darkFixedId, "Test Fixed Dark", conditions.EffectValue{UsesMagnitude: true}, conditions.EffectDarknessStrength, false),
		darkSight24Id: lit(darkSight24Id, "Test Sight", conditions.EffectValue{Literal: 24}, conditions.EffectNightVisionStrength, false),
		darkInfra30Id: lit(darkInfra30Id, "Test Infra 30", conditions.EffectValue{Literal: 30}, conditions.EffectInfraReach, false),
		darkInfra50Id: lit(darkInfra50Id, "Test Infra 50", conditions.EffectValue{Literal: 50}, conditions.EffectInfraReach, false),
		darkHoodedId:  lit(darkHoodedId, "Test Hooded", conditions.EffectValue{Literal: 54}, conditions.EffectLightStrength, true),
		darkGlowId:    lit(darkGlowId, "Test Glow", conditions.EffectValue{UsesMagnitude: true}, conditions.EffectLightStrength, true),
	}))
	return &darkUsers{t: t, all: map[int]*users.UserRecord{}}
}

// darkUsers is one test's users. SeedUsersForTest replaces the whole
// registry, so every add reseeds all of the test's users together; the
// stacked cleanups restore the registry in reverse.
type darkUsers struct {
	t   *testing.T
	all map[int]*users.UserRecord
}

// add makes a test user holding the given literal conditions and seeds it
// beside every user this test already made.
func (d *darkUsers) add(id int, conds ...int) *users.UserRecord {
	d.t.Helper()
	u := users.NewTestUser(id, "darku", "Darku", uint64(90000+id))
	for _, c := range conds {
		u.Character.Conditions.AddCondition(c, false)
	}
	d.all[id] = u
	seed := make(map[int]*users.UserRecord, len(d.all))
	for k, v := range d.all {
		seed[k] = v
	}
	d.t.Cleanup(users.SeedUsersForTest(seed))
	return u
}

// sourceNow is the record's current output, and false when it is off.
func sourceNow(c *characters.Character, conditionId int) (float64, bool) {
	for _, rec := range c.Conditions.LightAndDarknessSources() {
		if rec.ConditionId == conditionId {
			return rec.LightNow(conditions.GetConditionSpec(rec.ConditionId))
		}
	}
	return 0, false
}

// The Rule 3 trim table of the spec, through a real room: an Umbral Lantern
// (full 50) entering alone, or beside someone else's darkness.
func TestUmbralLanternTrimsToItsBearersEyes(t *testing.T) {
	us := seedDarknessTrim(t)
	cases := []struct {
		name      string
		lamp      int // 0 means no lamp: an unlit cave
		eyes      []int
		otherDark float64 // 0 means none
		wantOff   bool
		wantOut   float64
		wantRoom  int
	}{
		{"normal eyes, cave: off", 0, nil, 0, true, 0, 0},
		{"normal eyes, tavern 50", 50, nil, 0, false, 25, 25},
		{"normal eyes, light 70", 70, nil, 0, false, 45, 25},
		{"normal eyes, light 90: full", 90, nil, 0, false, 50, 40},
		{"nightvision 24, light 50", 50, []int{darkSight24Id}, 0, false, 49, 1},
		{"infravision 30, cave", 0, []int{darkInfra30Id}, 0, false, 30, -30},
		{"infravision 50, cave: full", 0, []int{darkInfra50Id}, 0, false, 50, -50},
		{"infravision 50, light 50: full", 50, []int{darkInfra50Id}, 0, false, 50, 0},
		{"normal eyes, light 50, other darkness 20", 50, nil, 20, false, 12.93, 25},
		{"normal eyes, light 50, other darkness 30: off", 50, nil, 30, true, 0, 20},
	}
	for i, c := range cases {
		room := &Room{RoomId: darkFirstRoom + i, Biome: "cave"}
		if c.lamp > 0 {
			room.Lamp = LampPtr(c.lamp)
		}
		if c.otherDark > 0 {
			other := us.add(darkFirstUser + 100 + i)
			other.Character.Conditions.AddConditionMagnitude(darkFixedId, 4, c.otherDark)
			room.AddPlayer(other.UserId)
		}
		bearer := us.add(darkFirstUser+i, append(append([]int{}, c.eyes...), darkUmbralId)...)
		room.AddPlayer(bearer.UserId)
		room.TrimLightFor(bearer.Character)

		out, on := sourceNow(bearer.Character, darkUmbralId)
		switch {
		case c.wantOff && on:
			t.Errorf("%s: output %v, want off", c.name, out)
		case !c.wantOff && (!on || math.Abs(out-c.wantOut) > 0.01):
			t.Errorf("%s: output (%v, %v), want %v", c.name, out, on, c.wantOut)
		}
		if got := room.LightLevel(); got != c.wantRoom {
			t.Errorf("%s: room after = %d, want %d", c.name, got, c.wantRoom)
		}
	}
}

// A full-strength result is recorded as full, not as trimmed at 50.
func TestAnUncutDarknessStaysFull(t *testing.T) {
	us := seedDarknessTrim(t)
	room := &Room{RoomId: darkFirstRoom + 20, Biome: "cave"}
	bearer := us.add(darkFirstUser+20, darkInfra50Id, darkUmbralId)
	room.AddPlayer(bearer.UserId)
	room.TrimLightFor(bearer.Character)
	if rec := bearer.Character.Conditions.DarknessSources()[0]; rec.LightTrim != conditions.LightFull {
		t.Errorf("an uncut darkness trim state = %q, want full", rec.LightTrim)
	}
}

// The light row of Rule 3: a light in a darkened room may run brighter before
// it dazzles, because the light trim solves against its target plus the
// room's darkness (ruling D3).
func TestALightTrimsAgainstTheRoomsDarkness(t *testing.T) {
	us := seedDarknessTrim(t)
	cases := []struct {
		name     string
		lamp     int
		darkness bool
		wantFull bool
		wantRoom int
	}{
		{"light 70, darkness 50", 70, true, true, 23},
		{"light 74, darkness 50", 74, true, true, 26},
		{"light 74, no darkness: off", 74, false, false, 74},
	}
	for i, c := range cases {
		room := &Room{RoomId: darkFirstRoom + 30 + i, Biome: "cave", Lamp: LampPtr(c.lamp)}
		if c.darkness {
			other := us.add(darkFirstUser + 130 + i)
			other.Character.Conditions.AddConditionMagnitude(darkFixedId, 4, 50)
			room.AddPlayer(other.UserId)
		}
		bearer := us.add(darkFirstUser+30+i, darkHoodedId)
		room.AddPlayer(bearer.UserId)
		room.TrimLightFor(bearer.Character)

		rec := bearer.Character.Conditions.LightSources()[0]
		if c.wantFull && rec.LightTrim != conditions.LightFull {
			t.Errorf("%s: the lantern trimmed to %q, want full", c.name, rec.LightTrim)
		}
		if !c.wantFull && rec.LightTrim != conditions.LightOff {
			t.Errorf("%s: the lantern trim = %q, want off", c.name, rec.LightTrim)
		}
		if got := room.LightLevel(); got != c.wantRoom {
			t.Errorf("%s: room after = %d, want %d", c.name, got, c.wantRoom)
		}
	}
}

// A bearer's light and darkness trim in held order, each seeing the room as
// the earlier ones left it. Glow first: the glow lights the cave to 74, then
// the darkness cuts it to the bearer's floor. Darkness first: the cave is
// already below the floor, so the darkness goes off, then the glow runs to 74.
func TestALightAndADarknessTrimInHeldOrder(t *testing.T) {
	us := seedDarknessTrim(t)

	glowFirst := us.add(darkFirstUser + 40)
	glowFirst.Character.Conditions.AddConditionMagnitude(darkGlowId, 4, 90)
	glowFirst.Character.Conditions.AddCondition(darkUmbralId, false)
	room := &Room{RoomId: darkFirstRoom + 40, Biome: "cave"}
	room.AddPlayer(glowFirst.UserId)
	room.TrimLightFor(glowFirst.Character)
	if v, ok := sourceNow(glowFirst.Character, darkGlowId); !ok || math.Abs(v-74) > darkTrimEpsilon {
		t.Errorf("glow first: glow = (%v, %v), want 74", v, ok)
	}
	if v, ok := sourceNow(glowFirst.Character, darkUmbralId); !ok || math.Abs(v-49) > darkTrimEpsilon {
		t.Errorf("glow first: darkness = (%v, %v), want 49", v, ok)
	}
	if got := room.LightLevel(); got != 25 {
		t.Errorf("glow first: room = %d, want 25", got)
	}

	darkFirst := us.add(darkFirstUser+41, darkUmbralId)
	darkFirst.Character.Conditions.AddConditionMagnitude(darkGlowId, 4, 90)
	room2 := &Room{RoomId: darkFirstRoom + 41, Biome: "cave"}
	room2.AddPlayer(darkFirst.UserId)
	room2.TrimLightFor(darkFirst.Character)
	if _, ok := sourceNow(darkFirst.Character, darkUmbralId); ok {
		t.Error("darkness first: the darkness should go off in an unlit cave")
	}
	if got := room2.LightLevel(); got != 74 {
		t.Errorf("darkness first: room = %d, want 74", got)
	}
}

// Arrivals trim in entry order and nobody already here re-trims: darkness
// arriving is never countered by what is there, and a later arrival does
// not move an earlier one (arc ruling 3).
func TestDarknessEntryOrderAcrossThreeBearers(t *testing.T) {
	us := seedDarknessTrim(t)
	room := &Room{RoomId: darkFirstRoom + 50, Biome: "cave", Lamp: LampPtr(50)}

	a := us.add(darkFirstUser+50, darkUmbralId)
	room.AddPlayer(a.UserId)
	room.TrimLightFor(a.Character)
	if got := room.LightLevel(); got != 25 {
		t.Fatalf("after the normal-eyed bearer: room %d, want 25", got)
	}

	b := us.add(darkFirstUser+51, darkInfra50Id, darkUmbralId)
	room.AddPlayer(b.UserId)
	room.TrimLightFor(b.Character)
	if rec := b.Character.Conditions.DarknessSources()[0]; rec.LightTrim != conditions.LightFull {
		t.Errorf("the infravision bearer's darkness = %q, want full", rec.LightTrim)
	}
	if got := room.LightLevel(); got != -1 {
		t.Errorf("after the infravision bearer: room %d, want -1", got)
	}

	c := us.add(darkFirstUser + 52)
	c.Character.Conditions.AddConditionMagnitude(darkGlowId, 4, 90)
	room.AddPlayer(c.UserId)
	room.TrimLightFor(c.Character)
	if rec := c.Character.Conditions.LightSources()[0]; rec.LightTrim != conditions.LightFull {
		t.Errorf("the glow arriving in the darkened room = %q, want full", rec.LightTrim)
	}
	if got := room.LightLevel(); got != 39 {
		t.Errorf("after the glow: room %d, want 39", got)
	}

	if v, ok := sourceNow(a.Character, darkUmbralId); !ok || math.Abs(v-25) > darkTrimEpsilon {
		t.Errorf("the first bearer re-trimmed to (%v, %v) when others entered, want 25", v, ok)
	}
}

// A spawn lists its mob with listMobInRoom, not AddMob, so it does not trim:
// the Phantom's lantern is at full strength in its lair from the start.
func TestASpawnedMobsDarknessDoesNotTrim(t *testing.T) {
	seedDarknessTrim(t)
	m := &mobs.Mob{MobId: darkMobId, InstanceId: darkMobInst,
		Character: characters.Character{Name: "darkmob", RoomId: darkFirstRoom + 60, Conditions: conditions.New()}}
	m.Character.Conditions.AddCondition(darkUmbralId, false)
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{darkMobInst: m}))

	room := &Room{RoomId: darkFirstRoom + 60, Biome: "cave"}
	listMobInRoom(room, darkMobInst)
	if rec := m.Character.Conditions.DarknessSources()[0]; rec.LightTrim != conditions.LightFull {
		t.Errorf("a spawned mob's darkness = %q, want full", rec.LightTrim)
	}
	if got := room.LightLevel(); got != -50 {
		t.Errorf("the lair after a spawn = %d, want -50", got)
	}
}

// A mob walking in (AddMob) does trim: a normal-eyed mob's darkness goes off
// in an unlit cave.
func TestAMobWalkingInTrimsItsDarkness(t *testing.T) {
	seedDarknessTrim(t)
	m := &mobs.Mob{MobId: darkMobId, InstanceId: darkMobInst + 1,
		Character: characters.Character{Name: "darkmob", RoomId: darkFirstRoom + 61, Conditions: conditions.New()}}
	m.Character.Conditions.AddCondition(darkUmbralId, false)
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{darkMobInst + 1: m}))

	room := &Room{RoomId: darkFirstRoom + 62, Biome: "cave"}
	room.AddMob(darkMobInst + 1)
	if _, ok := sourceNow(&m.Character, darkUmbralId); ok {
		t.Error("a normal-eyed mob walking into an unlit cave kept its darkness on")
	}
}
```

In `internal/rooms/carried_light_test.go`, give each `composeWith` call its new `dark` argument and move the empty-cave `-Inf` assertion onto `Light` (F4). Replace

```go
	one := cave.composeWith(cfg, 60, 1, []float64{56})
	two := cave.composeWith(cfg, 60, 1, []float64{56, 56})
```

with

```go
	one := cave.composeWith(cfg, 60, 1, []float64{56}, nil)
	two := cave.composeWith(cfg, 60, 1, []float64{56, 56}, nil)
```

replace

```go
	if !one.Carried || cave.composeWith(cfg, 60, 1, nil).Carried {
```

with

```go
	if !one.Carried || cave.composeWith(cfg, 60, 1, nil, nil).Carried {
```

replace

```go
	if empty := cave.composeWith(cfg, 60, 1, nil); !math.IsInf(empty.Raw, -1) || empty.Level != 0 {
		t.Errorf("an empty cave: Raw %v Level %d, want -Inf and 0", empty.Raw, empty.Level)
	}
```

with

```go
	// Since lighting plan 5d, Raw is the net light (Absent light reads 0), and
	// the light combine alone is Light.
	if empty := cave.composeWith(cfg, 60, 1, nil, nil); !math.IsInf(empty.Light, -1) || empty.Raw != 0 || empty.Level != 0 {
		t.Errorf("an empty cave: Light %v Raw %v Level %d, want -Inf, 0 and 0", empty.Light, empty.Raw, empty.Level)
	}
```

and replace

```go
	got := cave.composeWith(cfg, 60, 1, []float64{0})
```

with

```go
	got := cave.composeWith(cfg, 60, 1, []float64{0}, nil)
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/rooms/ -count=1`
Expected: FAIL to build: `too many arguments in call to cave.composeWith`, `got.Light undefined`, `Dark undefined`, `Darkened undefined`.

- [ ] **Step 3: The composition (`lighting.go`)**

Replace

```go
//  3. Everything anyone in the room carries, one term per light.
//
```

with

```go
//  3. Everything anyone in the room carries, one term per light.
//
// Every carried darkness is then combined on the same operator and taken
// away from the result, so a room can read below 0 (lighting plan 5d).
//
```

Replace

```go
	// Level is exactly LightLevel(): both come from composeLight.
	Level int
	// Raw is the combined light before rounding and clamping; Absent when
	// nothing lights the room. A trim solves against it.
	Raw float64
```

with

```go
	// Level is exactly LightLevel(): both come from composeLight.
	Level int
	// Raw is the net light Level rounds and clamps: Light (read as 0 when
	// Absent) minus Dark (read as 0 when Absent). An unlit room with no
	// darkness is 0 (lighting plan 5d, ruling D3).
	Raw float64
	// Light is the combined light of the sky, the lamp and every carried
	// light; lightscale.Absent() when nothing lights the room. A light's trim
	// solves against it.
	Light float64
	// Dark is the combined darkness every carried darkness takes away, by the
	// same halving rule; lightscale.Absent() when nobody carries one
	// (lighting plan 5d).
	Dark float64
```

Replace

```go
	// Carried reports that someone in the room carries a light.
	Carried bool
}
```

with

```go
	// Carried reports that someone in the room carries a light.
	Carried bool
	// Darkened reports that someone in the room carries a darkness.
	Darkened bool
}
```

Replace

```go
// composeLightExcluding is composeLight with one carried record left out, which
// is the room a trimming source sees: everything except itself.
func (r *Room) composeLightExcluding(cfg configs.Lighting, celestial, skyFilter float64, exclude *conditions.Condition) LightTerms {
	return r.composeWith(cfg, celestial, skyFilter, r.carriedLight(exclude))
}

// composeWith is the composition with the carried terms supplied, so a test
// needs no users or mobs.
func (r *Room) composeWith(cfg configs.Lighting, celestial, skyFilter float64, carried []float64) LightTerms {
```

with

```go
// composeLightExcluding is composeLight with one carried record left out of
// whichever combine it belongs to, which is the room a trimming source sees:
// everything except itself.
func (r *Room) composeLightExcluding(cfg configs.Lighting, celestial, skyFilter float64, exclude *conditions.Condition) LightTerms {
	carried, dark := r.carriedTerms(exclude)
	return r.composeWith(cfg, celestial, skyFilter, carried, dark)
}

// composeWith is the composition with the carried light and darkness terms
// supplied, so a test needs no users or mobs.
//
// Lights combine as they always have; darknesses combine among themselves by
// the same halving rule; the net light is the combined light (0 when none)
// minus the combined darkness (0 when none), clamped to [-100, 100]
// (lighting plan 5d, owner decision 1).
func (r *Room) composeWith(cfg configs.Lighting, celestial, skyFilter float64, carried, dark []float64) LightTerms {
```

Replace

```go
	v := lightscale.Combine(step, terms...)
	out.Raw = v
	if math.IsInf(v, -1) {
		// No light of any kind. Zero is the darkest light that NATURALLY
		// occurs, which is what an unlit cave is. Magical darkness goes below
		// this and arrives in plan 5d.
		v = 0
	}
```

with

```go
	out.Light = lightscale.Combine(step, terms...)
	v := out.Light
	if math.IsInf(v, -1) {
		// No light of any kind. Zero is the darkest light that NATURALLY
		// occurs, which is what an unlit cave is. Only magical darkness goes
		// below it.
		v = 0
	}

	// 4. Every darkness anyone here carries (lighting plan 5d), combined
	// among themselves by the same halving rule and taken away from the
	// light. Two darknesses of 50 take 58, not 100.
	out.Dark = lightscale.Combine(step, dark...)
	if len(dark) > 0 {
		out.Darkened = true
	}
	if !math.IsInf(out.Dark, -1) {
		v -= out.Dark
	}
	out.Raw = v
```

Replace

```go
// carriedLight is every carried light term in the room, leaving out one record
// (the source being trimmed) when exclude is non-nil. One pass over the room's
// occupants: each bearer's records are read exactly once.
func (r *Room) carriedLight(exclude *conditions.Condition) []float64 {
	var terms []float64
	add := func(c *characters.Character) {
		for _, rec := range c.Conditions.LightSources() {
			if rec == exclude {
				continue
			}
			if v, ok := rec.LightNow(conditions.GetConditionSpec(rec.ConditionId)); ok {
				terms = append(terms, v)
			}
		}
	}
```

with

```go
// carriedTerms is every carried light term and every carried darkness term in
// the room, leaving out one record (the source being trimmed) when exclude is
// non-nil. One pass over the room's occupants: each bearer's records are read
// exactly once.
func (r *Room) carriedTerms(exclude *conditions.Condition) (light, dark []float64) {
	add := func(c *characters.Character) {
		for _, rec := range c.Conditions.LightAndDarknessSources() {
			if rec == exclude {
				continue
			}
			spec := conditions.GetConditionSpec(rec.ConditionId)
			v, ok := rec.LightNow(spec)
			if !ok {
				continue
			}
			if spec.IsDarknessSource() {
				dark = append(dark, v)
			} else {
				light = append(light, v)
			}
		}
	}
```

and, at the end of that function, replace

```go
			add(u.Character)
		}
	}
	return terms
}
```

with

```go
			add(u.Character)
		}
	}
	return light, dark
}
```

- [ ] **Step 4: The trim (`light_trim.go`)**

Replace the whole of `internal/rooms/light_trim.go` with:

```go
package rooms

import (
	"math"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// TrimLightFor trims every adjustable, unhooded light AND darkness record c
// holds to c's own eyes. A light takes the least cut from full strength that
// keeps this room from dazzling them; a darkness the least cut that keeps the
// room at or above the bottom of their usable range (lighting plan 5d). The
// records trim one after another in held order, each seeing the room as the
// previous trims left it. A pre-pass sets the bearer's eligible sources off
// first, which already keeps each source out of its own combine; the exclude
// argument to composeLightExcluding is belt and braces, kept because it
// states the intent.
//
// A light solves Trim on the light combine against its target plus the
// room's darkness: a light in a darkened room may run brighter before it
// dazzles, which is what the room does (ruling D3). A darkness solves
// TrimDarkness against the light and the other darkness (ruling D2).
//
// It is the only trim trigger. MoveToRoom and AddMob call it once the mover
// is in the room, so arrivals trim in entry order, and nobody already here
// re-trims when someone else walks in. Nothing calls it on a round tick: a
// room that changes around a standing bearer leaves their sources as they
// were.
func (r *Room) TrimLightFor(c *characters.Character) {
	if r == nil || c == nil {
		return
	}
	type trimmable struct {
		rec  *conditions.Condition
		spec *conditions.ConditionSpec
		dark bool
	}
	var todo []trimmable
	for _, rec := range c.Conditions.LightAndDarknessSources() {
		spec := conditions.GetConditionSpec(rec.ConditionId)
		if spec == nil || rec.Hooded || !slices.Contains(spec.Flags, conditions.Adjustable) {
			continue
		}
		todo = append(todo, trimmable{rec, spec, spec.IsDarknessSource()})
	}
	// A bearer with nothing adjustable (a plain torch, or no source at all)
	// pays nothing on a move.
	if len(todo) == 0 {
		return
	}

	cfg := configs.GetLightingConfig()
	celestial := gametime.CelestialLight()
	skyFilter := r.mutatorSkyFilter()
	strength := c.NightVisionStrength()
	lightTarget := messaging.LightTrimTarget(strength, cfg.DazzleAbove)
	darkFloor := messaging.DarknessTrimTarget(strength, c.InfraReach(), cfg.BlindBelow)

	// Every source about to trim leaves the room first. Otherwise a source
	// would see the later ones still at full strength, trim to nothing, and
	// hand the room to whichever source comes last: held order inverted. With
	// them out, each source sees the room with the earlier trims and nothing
	// of its own that has not trimmed yet.
	for _, t := range todo {
		t.rec.SetLightOutput(lightscale.Absent())
	}
	for _, t := range todo {
		terms := r.composeLightExcluding(cfg, celestial, skyFilter, t.rec)
		full := t.rec.LightMax(t.spec)
		var out float64
		if t.dark {
			out = lightscale.TrimDarkness(cfg.DoublingStep, terms.Light, terms.Dark, full, darkFloor)
		} else {
			dark := terms.Dark
			if math.IsInf(dark, -1) {
				dark = 0
			}
			out = lightscale.Trim(cfg.DoublingStep, terms.Light, full, lightTarget+dark)
		}
		if out >= full {
			// No cut at all: the source runs at full strength, so it is not "trimmed".
			t.rec.LightTrim, t.rec.LightOutput = conditions.LightFull, 0
			continue
		}
		t.rec.SetLightOutput(out)
	}
}
```

- [ ] **Step 5: Run them to see them pass**

Run: `go vet ./internal/rooms/ && go test ./internal/rooms/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/rooms`. The three-bearer entry test reads `25`, then `-1`, then `39`; the spawn test `-50`.

Run: `go test . -run "Lighting" -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud` (the parity and daycycle goldens do not move: nothing they sample carries darkness).

- [ ] **Step 6: Commit**

```bash
git add internal/rooms/lighting.go internal/rooms/light_trim.go internal/rooms/carried_light_test.go internal/rooms/darkness_compose_test.go internal/rooms/darkness_trim_test.go
git commit -m "feat(rooms): darkness subtracts from the room and trims to its bearer's floor (5d Rules 2, 3)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: A darkness is never a light, and its arrival is seen before it falls (D1 guard, D6)

**Model:** sonnet (two narration sites under a `file|literal` viewpoint guard and a `file|line` allowlist; the shapes are chosen to keep both).

**Files:**
- Modify: `internal/characters/light.go` (add `DarknessTerms`)
- Modify: `internal/hooks/Condition_ApplyConditions.go:175-176` (two lines for two) and append a helper after the last function
- Modify: `internal/usercommands/equip.go` (imports `:7-17`, the wearable branch `:152-159`)
- Modify: `internal/mobcommands/equip.go:86-88`
- Create: `internal/hooks/darkness_condition_test.go`, `internal/usercommands/darkness_test.go`, `internal/mobcommands/equip_darkness_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/darkness_condition_test.go` (the D6 start line in a room the pall darkens below anyone's sight, and the darkness spell's scale through the shared seam):

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPallConditionId is a magnitude darkness, the shape of the shipped
// condition 131 Chrysalis Pall.
const testPallConditionId = 9741

func seedPallCondition() func() {
	return conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		testPallConditionId: {ConditionId: testPallConditionId, Name: "Test Pall", RoundInterval: 5, TriggerCount: 4,
			StartRoomText: "A pall of dark spores gathers around {actee_plain}.",
			Effects:       map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {UsesMagnitude: true}},
			Flags:         []conditions.Flag{conditions.Adjustable, conditions.Cancellable}},
	})
}

// Ruling D6: a darkness's start line is judged as lit. The record is already
// held when the line goes out, so judged by the room as it now is, the
// observers the darkness has just blinded would miss it.
func TestDarknessStartLineIsSeenBeforeTheDarkFalls(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedPallCondition()
	defer restore()
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(50)
	drainPlain(2)

	require.Equal(t, events.Continue, ApplyConditions(events.Condition{
		UserId: 1, ConditionId: testPallConditionId, Magnitude: 90, Triggers: 4}))
	require.Less(t, room.LightLevel(), 0, "the pall must actually have darkened the room below anyone's sight")
	assert.Equal(t, 1, countContaining(drainPlain(2), "A pall of dark spores gathers around Aliceia."),
		"the observer the pall just blinded must still see it gather")
}

// The darkness spell scales from its own trios through the shared seam
// (spec Rule 4): a mid caster (130, 30) darkens by 68 for 6 triggers.
func TestDarknessSpellAppliesAtTheCastersScale(t *testing.T) {
	configs.SetConfigForTest(t, configs.GetConfig())
	t.Cleanup(seedPallCondition())
	spell := &spells.SpellData{SpellId: "test-pall", PrimaryStat: "willpower"}
	caster := characters.New()
	caster.Stats.Willpower.ValueAdj = 130
	caster.SetSkill("spellcasting", 30)
	mag, trig, ok := magnitudeSpellApplication(spell, caster, testPallConditionId)
	if !ok || mag != 68 || trig != 6 {
		t.Errorf("mid caster: (%v, %d, %v), want (68, 6, true)", mag, trig, ok)
	}
}
```

Create `internal/usercommands/darkness_test.go` (the spec's "darkness is never light" guard, and the D6 equip line; `hoodTestText` is `hood_test.go`'s drain):

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	darkTestUmbralCond = 9761 // literal darkness 50, adjustable, secret: condition 132's shape
	darkTestPallCond   = 9762 // magnitude darkness, adjustable, cancellable: condition 131's shape
	darkTestUmbralItem = 999970
)

// darknessFixture seeds the two darkness shapes, an Umbral Lantern-shaped
// light-slot item, a species for Wear to dereference, and returns user 1
// standing in room 2, lit by a lamp of 50, with user 2 watching.
func darknessFixture(t *testing.T) (*users.UserRecord, *users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "human", Size: species.Medium},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		darkTestUmbralCond: {ConditionId: darkTestUmbralCond, Name: "Test Umbral Dark", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 50}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
		darkTestPallCond: {ConditionId: darkTestPallCond, Name: "Test Pall", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {UsesMagnitude: true}},
			Flags:   []conditions.Flag{conditions.Adjustable, conditions.Cancellable}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		darkTestUmbralItem: {ItemId: darkTestUmbralItem, Name: "test umbral lantern", NameSimple: "lantern",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{darkTestUmbralCond}},
	}))

	user := users.GetByUserId(1)
	require.NotNil(t, user)
	user.Character.SpeciesId = 0
	user.Character.Stats.Strength.ValueAdj = 100

	room := rooms.LoadRoom(2)
	require.NotNil(t, room)
	room.Biome = "cave"
	room.Lamp = rooms.LampPtr(50)
	user.Character.RoomId = 2
	room.AddPlayer(user.UserId)

	observer := users.GetByUserId(2)
	require.NotNil(t, observer)
	observer.Character.RoomId = 2
	room.AddPlayer(observer.UserId)
	return user, observer, room
}

// The guard the spec names "darkness is never light": a character holding a
// pall and an Umbral Dark sheds no light, is no sneak beacon, and cannot hood
// the lantern. Proven able to fail by making IsLightSource accept
// darkness_strength.
func TestDarknessIsNeverLight(t *testing.T) {
	user, _, room := darknessFixture(t)
	_, ok, why := user.Character.Wear(items.New(darkTestUmbralItem))
	require.True(t, ok, why)
	require.True(t, user.Character.Conditions.AddConditionMagnitude(darkTestPallCond, 4, 50))
	require.Len(t, user.Character.DarknessTerms(), 2, "fixture: both darknesses must be held and on")

	require.Empty(t, user.Character.LightTerms(), "a darkness is not a light term")
	require.False(t, user.Character.EmitsLight(), "a darkness bearer must not shed light")

	// The sneak score takes the no-light branch: exactly what the same
	// character scores holding nothing at all.
	bare := users.NewTestUser(9763, "bare", "Bare", 99763)
	bare.Character.Stats = user.Character.Stats
	bare.Character.Skills = user.Character.Skills
	for _, lit := range []bool{false, true} {
		require.Equal(t, actions.CalcSneakScore(bare.Character, lit), actions.CalcSneakScore(user.Character, lit),
			"lit=%v: a darkness bearer's sneak score moved, so it counted as a light", lit)
	}

	events.DrainQueuedMessagesForTest(user.UserId)
	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	require.Contains(t, hoodTestText(user.UserId), "has no hood.", "hood must refuse a darkness in the light slot")
}

// Ruling D6: the equip room line of a darkness item is judged as lit. The
// lantern takes the room from 50 to 0 as it goes on, and the observer, now in
// the dark, still sees it put on.
func TestEquippingADarknessIsSeenBeforeTheDarkFalls(t *testing.T) {
	user, observer, room := darknessFixture(t)
	user.Character.StoreItem(items.Item{ItemId: darkTestUmbralItem})
	events.DrainQueuedMessagesForTest(observer.UserId)

	handled, err := Equip("test umbral lantern", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, darkTestUmbralItem, user.Character.Equipment.Light.ItemId, "fixture: the lantern must be worn")
	require.Less(t, room.LightLevel(), 25, "fixture: the lantern must have taken the room below normal sight")
	require.Contains(t, hoodTestText(observer.UserId), "puts on their",
		"the observer the lantern just blinded missed it being put on")
}
```

Create `internal/mobcommands/equip_darkness_test.go` (the mob path's sibling line):

```go
package mobcommands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Ruling D6 on the mob path, the sibling of the player's equip line: a mob
// putting on a darkness is announced judged as lit, so the players it has
// just blinded still see it happen.
func TestMobEquippingADarknessIsSeenBeforeTheDarkFalls(t *testing.T) {
	const umbralCond, umbralItem = 9771, 999971
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		umbralCond: {ConditionId: umbralCond, Name: "Test Umbral Dark", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 50}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		umbralItem: {ItemId: umbralItem, Name: "test umbral lantern", NameSimple: "lantern",
			Type: items.Light, Subtype: items.Wearable, WornConditionIds: []int{umbralCond}},
	}))
	mob, room := getTestMobAndRoom(t)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = rooms.LampPtr(50)
	require.True(t, mob.Character.StoreItem(items.Item{ItemId: umbralItem}))
	events.DrainQueuedMessagesForTest(1)

	_, err := Equip(fmt.Sprintf("!%d", umbralItem), mob, room)
	require.NoError(t, err)
	require.Equal(t, umbralItem, mob.Character.Equipment.Light.ItemId, "fixture: the lantern must be worn")
	require.Less(t, room.LightLevel(), 25, "fixture: the lantern must have taken the room below normal sight")
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(1), "\n"), "puts on",
		"the player the mob's lantern just blinded missed it being put on")
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/hooks/ ./internal/usercommands/ ./internal/mobcommands/ -run "Darkness|DarknessIsNever" -count=1`
Expected: `internal/usercommands` FAILS to build, `user.Character.DarknessTerms undefined`. `internal/hooks` FAILS `TestDarknessStartLineIsSeenBeforeTheDarkFalls` (`expected: 1, actual: 0`: the line is judged by the room the pall has already blinded); `TestDarknessSpellAppliesAtTheCastersScale` passes (Task 3 built it). `internal/mobcommands` FAILS `TestMobEquippingADarknessIsSeenBeforeTheDarkFalls` with `the player the mob's lantern just blinded missed it being put on`.

- [ ] **Step 3: `DarknessTerms` (`internal/characters/light.go`)**

Replace

```go
// EmitsLight reports whether this character sheds any light right now. A
```

with

```go
// DarknessTerms is every darkness term this character takes from its room
// right now, one per held darkness record (lighting plan 5d). LightTerms'
// twin: a darkness is never one of LightTerms, so it never makes a
// character EmitsLight.
func (c *Character) DarknessTerms() []float64 {
	var out []float64
	for _, rec := range c.Conditions.DarknessSources() {
		if v, ok := rec.LightNow(conditions.GetConditionSpec(rec.ConditionId)); ok {
			out = append(out, v)
		}
	}
	return out
}

// EmitsLight reports whether this character sheds any light right now. A
```

- [ ] **Step 4: The start line (`internal/hooks/Condition_ApplyConditions.go`)**

Replace (two lines for two, so the allowlisted `|103`, `|105`, `|107` above do not move, F11)

```go
					r.SendTextVisualHidingNames(messaging.CategoryConditionApply,
						roles.Observer, []string{charPlainName}, excludeId)
```

with

```go
					sendConditionStartRoomText(r, conditionInfo,
						roles.Observer, []string{charPlainName}, excludeId)
```

and append at the end of the file (after `ApplyConditions`' closing brace):

```go

// sendConditionStartRoomText sends a condition's start room line on the
// visual channel. A darkness source's line is judged as if the room were lit
// (lighting plan 5d, ruling D6): the record is already held when the line
// goes out, so the room is judged with the new darkness in it, and the
// observers it has just blinded would miss "a pall gathers around X". It is
// the mirror of sendConditionEndRoomText's light end line. Every other start
// line is judged by the room as it is.
//
// names is the holder's PLAIN name, handed to HideNames explicitly because a
// start line may author a bare {actee_plain}.
func sendConditionStartRoomText(r *rooms.Room, spec *conditions.ConditionSpec, msg string, names []string, skip ...int) {
	if spec.IsDarknessSource() {
		r.SendTextVisualAsLitHidingNames(messaging.CategoryConditionApply, msg, names, skip...)
		return
	}
	r.SendTextVisualHidingNames(messaging.CategoryConditionApply, msg, names, skip...)
}
```

- [ ] **Step 5: The player equip line (`internal/usercommands/equip.go`)**

Add `"github.com/GoMudEngine/GoMud/internal/conditions"` to the imports, directly below `"github.com/GoMudEngine/GoMud/internal/characters"`.

Replace

```go
				room.SendTextVisual(messaging.CategoryEquipment,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> puts on their <ansi fg="item">%s</ansi>.`, user.Character.Name, result.Item.DisplayName()),
					user.UserId,
				)
```

with

```go
				putsOn := fmt.Sprintf(`<ansi fg="username">%s</ansi> puts on their <ansi fg="item">%s</ansi>.`, user.Character.Name, result.Item.DisplayName())
				// A darkness is judged as lit (lighting plan 5d, ruling D6): it
				// is already worn, and would silence its own arrival for
				// everyone it has just blinded. Two else-less ifs rather than an
				// if/else: messaging_surface_guard_test.go's walk splits an
				// if/else into separate events and would lose this line's
				// observer, which it tracks.
				asLit := conditions.AnyDarknessSource(result.Item.GetSpec().WornConditionIds)
				if asLit {
					room.SendTextVisualAsLit(messaging.CategoryEquipment, putsOn, user.UserId)
				}
				if !asLit {
					room.SendTextVisual(messaging.CategoryEquipment, putsOn, user.UserId)
				}
```

Do NOT write it as `if asLit { ... } else { ... }`: measured, that makes `messaging_surface_guard_test.go`'s `usercommands/equip.go|You wear your ...` entry stale (F13).

- [ ] **Step 6: The mob equip line (`internal/mobcommands/equip.go`)**

Replace

```go
		if iSpec.Subtype == items.Wearable {
			room.SendTextVisual(messaging.CategoryEquipment,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> puts on <ansi fg="item">%s</ansi>.`, mob.Character.Name, result.Item.DisplayName()))
		} else {
```

with

```go
		if iSpec.Subtype == items.Wearable {
			putsOn := fmt.Sprintf(`<ansi fg="mobname">%s</ansi> puts on <ansi fg="item">%s</ansi>.`, mob.Character.Name, result.Item.DisplayName())
			if conditions.AnyDarknessSource(iSpec.WornConditionIds) {
				// Judged as lit (lighting plan 5d, ruling D6), as the player path.
				room.SendTextVisualAsLit(messaging.CategoryEquipment, putsOn)
			} else {
				room.SendTextVisual(messaging.CategoryEquipment, putsOn)
			}
		} else {
```

(`conditions` is already imported there.)

- [ ] **Step 7: Run them to see them pass**

Run: `go build ./... && go test ./internal/characters/ ./internal/hooks/ ./internal/usercommands/ ./internal/mobcommands/ -count=1`
Expected: `ok` for all four.

Run: `go test . -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud` (the viewpoint walk, the condition-path allowlist and the sight-gate wrapper guard all unchanged, F11, F13, F15).

- [ ] **Step 8: Prove the guard test can fail**

In `internal/conditions/conditionspec.go`, temporarily make `IsLightSource` also accept darkness: replace its `return ok` with `_, dark := b.Effects[EffectDarknessStrength]` then `return ok || dark`. Run `go test ./internal/usercommands/ -run TestDarknessIsNeverLight -count=1`. Expected: FAIL, `Should be empty, but was [50 50]` / `a darkness is not a light term`. Restore the file (`git diff internal/conditions/conditionspec.go` must print nothing) and re-run: `ok`.

- [ ] **Step 9: Commit**

```bash
git add internal/characters/light.go internal/hooks/Condition_ApplyConditions.go internal/usercommands/equip.go internal/mobcommands/equip.go internal/hooks/darkness_condition_test.go internal/usercommands/darkness_test.go internal/mobcommands/equip_darkness_test.go
git commit -m "feat(darkness): never a light, and its arrival is judged as lit (5d D1, D6)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: A mob acts on any shape it can make out (ruling D8)

**Model:** sonnet (a balance-visible behaviour change across every dim room; the fix must stay out of the combat predicate).

**Files:**
- Modify: `internal/behaviortree/sight.go` (whole file)
- Modify: `internal/behaviortree/sight_test.go:66-69`, `conditions_sight_test.go:19-22`, `conditions_test.go:494-498` (stale comments, F16)
- Create: `internal/behaviortree/sight_shapes_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/behaviortree/sight_shapes_test.go`:

```go
package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const sightHeatConditionId = 9781 // infra reach 50, no nightvision: the Phantom's heat sense shape

// Lighting plan 5d, ruling D8: a mob acts on a figure it can make out. In a
// DIM room (the shapes band) with a player present, a mob with no
// infravision passes mobCanSee and condPlayersInRoom, where before 5d it saw
// nobody. Proven able to fail by reverting mobCanSee to SightFull only.
func TestMobActsOnShapesInADimRoom(t *testing.T) {
	m, room := sightScene(t, "cave")
	room.Lamp = rooms.LampPtr(30) // shapes for normal eyes: 25 <= 30 < 50
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(&m.Character, room), "fixture: the room must read shapes")

	u := users.NewTestUser(8130, "dim", "Dim", 98130)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8130: u}))
	room.AddPlayer(8130)

	require.True(t, mobCanSee(m, room), "a mob that can make out a shape acts on it")
	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Success, condPlayersInRoom(nil, ctx), "a dim room's mob finds the player it can make out")
}

// Heat reads shapes below any light: a heat-sensing mob acts in darkness its
// reach covers, from no light at all down to minus its reach.
func TestMobActsOnHeatInTheDark(t *testing.T) {
	m, room := sightScene(t, "cave")
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		sightHeatConditionId: {ConditionId: sightHeatConditionId, Name: "Test Heat Sense",
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 50}},
			Flags:   []conditions.Flag{conditions.InfraredVision}},
	}))
	require.False(t, mobCanSee(m, room), "fixture: without heat the unlit cave blinds it")
	require.NoError(t, m.Character.AddCondition(sightHeatConditionId, true))
	require.True(t, mobCanSee(m, room), "heat shows the room's shapes at light 0")
}

// A genuinely dark room, below the shapes floor, with no infravision on
// either side, still blinds a mob: D8 widens shapes, not darkness.
func TestMobStillSeesNothingInTrueDarkness(t *testing.T) {
	m, room := sightScene(t, "cave")
	u := users.NewTestUser(8131, "dark", "Dark", 98131)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8131: u}))
	room.AddPlayer(8131)

	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(&m.Character, room), "fixture: the cave must read nothing")
	require.False(t, mobCanSee(m, room))
	ctx := &EvalContext{InstanceId: m.InstanceId, RoomId: room.RoomId}
	require.Equal(t, Failure, condPlayersInRoom(nil, ctx))
}

// The combat darkness penalty still keys on full sight: D8 widens only mob
// decisions, never messaging.CanSeeSightImpairedOnly.
func TestCombatSightStaysFullOnly(t *testing.T) {
	m, room := sightScene(t, "cave")
	room.Lamp = rooms.LampPtr(30)
	require.True(t, mobCanSee(m, room))
	require.False(t, messaging.CanSeeSightImpairedOnly(&m.Character, room),
		"the combat predicate must stay SightFull only; D8 widens mob decisions alone")
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/behaviortree/ -run "Shapes|Heat|TrueDarkness|CombatSight" -count=1`
Expected: FAIL `TestMobActsOnShapesInADimRoom` (`a mob that can make out a shape acts on it`), `TestMobActsOnHeatInTheDark` (`heat shows the room's shapes at light 0`) and `TestCombatSightStaysFullOnly` (its first `require.True(mobCanSee)`); `TestMobStillSeesNothingInTrueDarkness` passes.

- [ ] **Step 3: Replace `sight.go`**

Replace the whole of `internal/behaviortree/sight.go` with:

```go
package behaviortree

import (
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// mobCanSee reports whether a mob can make out anyone in the room it is
// standing in, for its DECISIONS: target acquisition (condPlayersInRoom), the
// enemy count (condMultipleEnemies) and party aggro
// (engageHostilePlayerInRoom).
//
// LIGHTING PLAN 5d, ruling D8 (owner, 2026-09-30): a figure the mob can make
// out is enough. It is true at SightFull and at SightShapes from ANY cause,
// natural dim light or infravision, so a mob in a dim room acts on the shape
// it can see, and a heat-sensing mob acts in darkness its reach reads. This
// overturns slice F's ruling that a mob's sight was SightFull only
// (docs/superpowers/specs/completed/2026-09-11-followup-slice-f-mobs-perceive-darkness-design.md).
//
// 🔑 It calls messaging.ParticipantSight directly and does NOT go through
// messaging.CanSeeSightImpairedOnly any more. That predicate stays SightFull
// only because combat reads it to gate Balance.DarknessCombatPenalty on each
// side of a fight, and widening it would hand every infrared character a
// silent balance change. Only mob decisions widen; the combat darkness
// penalty is untouched.
//
// Night vision still cannot help below its floor: a shifted window reads
// nothing under windowFloor (internal/messaging/window.go), so a nightvision
// mob with no infra reach is blind in an unlit cave. A light carried by ANY
// player or mob lifts the darkness for everyone, because Room.LightLevel
// composes a term for every carried light (internal/rooms/lighting.go).
//
// A nil mob or room returns true. These run on every behaviour tree tick and a
// missing instance must not silently blind the world.
func mobCanSee(mob *mobs.Mob, room *rooms.Room) bool {
	if mob == nil || room == nil {
		return true
	}
	d := messaging.ParticipantSight(&mob.Character, room)
	return d == messaging.SightFull || d == messaging.SightShapes
}
```

- [ ] **Step 4: Correct the three stale comments**

In `internal/behaviortree/sight_test.go`, replace

```go
	// GRADED LIGHTING PLAN 2: mobCanSee requires SightFull specifically
	// (messaging.CanSeeSightImpairedOnly), and a shifted window is still
	// blind below its floor at light 0 no matter how strong the shift, so
	// night vision alone no longer restores sight in a pitch dark room.
```

with

```go
	// GRADED LIGHTING PLAN 2: a shifted window is still blind below its
	// floor at light 0 no matter how strong the shift, so night vision alone
	// does not restore sight in a pitch dark room. Since lighting plan 5d
	// (ruling D8) mobCanSee also accepts shapes, but night vision gives none
	// below the window floor either.
```

In `internal/behaviortree/conditions_sight_test.go`, replace

```go
	// GRADED LIGHTING PLAN 2: this rides mobCanSee, which requires SightFull
	// specifically. A shifted window is still blind below its floor at
	// light 0, so night vision alone no longer restores sight in true
	// darkness; see sight.go's doc comment.
```

with

```go
	// GRADED LIGHTING PLAN 2: this rides mobCanSee. A shifted window is
	// still blind below its floor at light 0, so night vision alone does
	// not restore sight in true darkness, not even shapes (which mobCanSee
	// accepts since lighting plan 5d); see sight.go's doc comment.
```

In `internal/behaviortree/conditions_test.go`, replace

```go
	// Seed room. Lamp pins it fully lit regardless of the ambient test
	// round: condMultipleEnemies gates on mobCanSee, which requires
	// SightFull, and since graded lighting plan 3a Task 8, LightLevel()
	// reads the real celestial term at whatever round util.GetRoundCount()
	// holds, which a bare unpinned round reads as shapes tier, not full.
```

with

```go
	// Seed room. Lamp pins it fully lit regardless of the ambient test
	// round: condMultipleEnemies gates on mobCanSee, and since graded
	// lighting plan 3a Task 8, LightLevel() reads the real celestial term at
	// whatever round util.GetRoundCount() holds, which a bare unpinned round
	// can read as anything from dark to full.
```

- [ ] **Step 5: Run the package, then everything that reads mob sight**

Run: `go test ./internal/behaviortree/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/behaviortree` (every existing darkness test still reads a mob blind at light 0, F16).

Run: `go test ./internal/hooks/ ./internal/mobs/ ./internal/combat/ . -count=1`
Expected: `ok` for all four. Measured on this plan's dry run: widening `mobCanSee` broke no existing test anywhere (F16). A failure here is a behaviour the widening reached: read it, and report it rather than narrowing D8.

- [ ] **Step 6: Prove the tests can fail**

Temporarily change the last line of `mobCanSee` to `return d == messaging.SightFull`. Run `go test ./internal/behaviortree/ -run "Shapes|Heat|TrueDarkness|CombatSight" -count=1`. Expected: FAIL in `TestMobActsOnShapesInADimRoom`, `TestMobActsOnHeatInTheDark`, `TestCombatSightStaysFullOnly`. Restore it (`git diff internal/behaviortree/sight.go` shows only this task's change) and re-run: `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/behaviortree/sight.go internal/behaviortree/sight_test.go internal/behaviortree/conditions_sight_test.go internal/behaviortree/conditions_test.go internal/behaviortree/sight_shapes_test.go
git commit -m "feat(behaviortree): a mob acts on any shape it can make out (5d D8)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: The darkness notice cause and the band-changed event (rulings D4, D7)

**Model:** sonnet (a new notice cause with its store file and golden, and a new event on the 3d cadence).

**Files:**
- Modify: `internal/events/eventtypes.go` (after `CharacterVitalsChanged`, `:434-438`)
- Modify: `internal/events/events.go` (above `DrainQueuedSkillUsedForTest`, `:622`)
- Modify: `internal/lightnotice/store.go:29,44`
- Modify: `internal/lightnotice/tracker.go` (imports, `attribute` `:144-177`, the state `:179-183`, `Check` `:197-205`, `Forget`, `ResetForTest`)
- Create: `_datafiles/world/dogmud/narration/light-notices/darkness.yaml`
- Create: `internal/lightnotice/darkness_test.go`
- Modify: `internal/narration/testdata/stores/light_notices.golden` (re-recorded)

- [ ] **Step 1: Write the failing tests**

Create `internal/lightnotice/darkness_test.go` (attribution cases, the cause through `Check` on a cast and on expiry, and the event on a change only; `r1.AddPlayer` because a carried darkness counts only for someone the room lists, F22):

```go
package lightnotice

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

const noticePallId = 9791 // magnitude darkness: the pall's shape

// Ruling D4: a darkness arriving, lapsing or changing strength is its own
// cause, checked first among the terms, never the eyes and never carried.
func TestAttributionNamesTheDarkness(t *testing.T) {
	lit := rooms.LightTerms{Level: 60, Sky: 55, SkyFilter: 1, Lamp: 40, HasLamp: true, Dark: lightscale.Absent()}
	with := func(f func(*rooms.LightTerms)) rooms.LightTerms { t2 := lit; f(&t2); return t2 }
	cases := []struct {
		name string
		prev record
		now  observation
		want Cause
	}{
		{"a darkness arrives",
			rec(1, messaging.BandFaces, lit),
			obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Darkened = true; x.Dark = 20 })),
			CauseDarkness},
		{"a darkness lapses",
			rec(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Darkened = true; x.Dark = 20 })),
			obs(1, messaging.BandFaces, lit),
			CauseDarkness},
		{"a darkness changes strength",
			rec(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Darkened = true; x.Dark = 20 })),
			obs(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 55; x.Darkened = true; x.Dark = 5 })),
			CauseDarkness},
		{"precedence: darkness beats a carried light changing with it",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Carried = true })),
			obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Darkened = true; x.Dark = 20 })),
			CauseDarkness},
		{"control: a steady darkness does not claim a lamp change",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Darkened = true; x.Dark = 20 })),
			obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 40; x.Lamp = 20; x.Darkened = true; x.Dark = 20 })),
			CauseLamp},
	}
	for _, c := range cases {
		if got := c.now.bandAt(c.now.terms.Level); got != c.now.band {
			t.Fatalf("%s: fixture inconsistent: sight reads %v at %d, case claims %v", c.name, got, c.now.terms.Level, c.now.band)
		}
		if got := attribute(c.prev, c.now); got != c.want {
			t.Errorf("%s: attribute = %q, want %q", c.name, got, c.want)
		}
	}
}

// Through Check: a pall cast in a lit room is announced from the darkness
// pool, and its expiry too; neither blames the eyes.
func TestCheckNamesADarknessOnCastAndOnExpiry(t *testing.T) {
	u, r1, _ := seedLampWorld(t)
	r1.AddPlayer(u.UserId) // a carried darkness counts only for someone the room lists
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		noticePallId: {ConditionId: noticePallId, Name: "Test Pall", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {UsesMagnitude: true}},
			Flags:   []conditions.Flag{conditions.Adjustable, conditions.Cancellable}},
	}))
	drain := captureFor(t, 1)

	Check(u, TriggerQuiet) // lamp 60: faces
	if !u.Character.Conditions.AddConditionMagnitude(noticePallId, 4, 90) {
		t.Fatal("fixture: the pall did not land")
	}
	Check(u, TriggerCommand) // 60 - 90 = -30: dark
	got := drain()
	if len(got) != 1 || !containsAny(got[0], Pool(CauseDarkness, DarkerDark, false)) {
		t.Fatalf("want one darkness darker_dark line on the cast, got %q", got)
	}

	u.Character.Conditions.RemoveCondition(noticePallId) // expires it; LightNow reads nothing
	Check(u, TriggerCommand)
	got = drain()
	if len(got) != 1 || !containsAny(got[0], Pool(CauseDarkness, LighterFaces, false)) {
		t.Fatalf("want one darkness lighter_faces line on expiry, got %q", got)
	}
}

// Ruling D7: Check queues SightBandChanged the first time and on a change of
// band, and not on a repeat.
func TestCheckQueuesSightBandChangedOnlyOnAChange(t *testing.T) {
	u, r1, _ := seedLampWorld(t)
	events.DrainQueuedSightBandChangedForTest(u.UserId)

	Check(u, TriggerQuiet)
	if got := events.DrainQueuedSightBandChangedForTest(u.UserId); len(got) != 1 {
		t.Fatalf("the first check must report the band once, got %d", len(got))
	}
	Check(u, TriggerCommand)
	if got := events.DrainQueuedSightBandChangedForTest(u.UserId); len(got) != 0 {
		t.Fatalf("a repeat check in the same band must queue nothing, got %d", len(got))
	}
	r1.Lamp = rooms.LampPtr(30)
	Check(u, TriggerCommand)
	if got := events.DrainQueuedSightBandChangedForTest(u.UserId); len(got) != 1 {
		t.Fatalf("a change of band must queue one update, got %d", len(got))
	}
	Forget(u.UserId)
	Check(u, TriggerQuiet)
	if got := events.DrainQueuedSightBandChangedForTest(u.UserId); len(got) != 1 {
		t.Fatalf("after Forget (logout) the next check must report the band again, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/lightnotice/ -count=1`
Expected: FAIL to build, `undefined: CauseDarkness` and `undefined: events.DrainQueuedSightBandChangedForTest`.

- [ ] **Step 3: The event and its drain (`internal/events`)**

In `internal/events/eventtypes.go`, replace

```go
func (p CharacterVitalsChanged) Type() string { return `CharacterVitalsChanged` }
```

with

```go
func (p CharacterVitalsChanged) Type() string { return `CharacterVitalsChanged` }

// SightBandChanged says a player's light band (messaging.LightBand: dark,
// shapes, faces or dazzled) differs from the one last reported. Queued by
// lightnotice.Check; the GMCP module answers it with Char.Sight, the web
// client's Game-window border (lighting plan 5d, ruling D7).
type SightBandChanged struct {
	UserId int
}

func (p SightBandChanged) Type() string { return `SightBandChanged` }
```

In `internal/events/events.go`, insert immediately above the line `// DrainQueuedSkillUsedForTest removes all SkillUsed events from the global`:

```go
// DrainQueuedSightBandChangedForTest removes all SightBandChanged events from
// the global queue for the given userId and returns them. Pass 0 to drain
// every SightBandChanged event regardless of user.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedSightBandChangedForTest(userId int) []SightBandChanged {
	qLock.Lock()
	defer qLock.Unlock()
	var found []SightBandChanged
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		sc, ok := pe.event.(SightBandChanged)
		if !ok {
			remaining = append(remaining, pe)
			continue
		}
		if userId == 0 || sc.UserId == userId {
			found = append(found, sc)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

```

- [ ] **Step 4: The cause (`store.go`)**

Replace

```go
	CauseEyes     Cause = "eyes"     // no light term explains it; the observer's sight changed
)
```

with

```go
	CauseEyes     Cause = "eyes"     // no light term explains it; the observer's sight changed
	// CauseDarkness is a carried darkness arriving, lapsing or changing
	// strength (lighting plan 5d, ruling D4). Without it such a change moves
	// no light term and falls to CauseEyes, and carried's lines ("The
	// carried light is gone") are wrong for it.
	CauseDarkness Cause = "darkness"
)
```

and replace

```go
var allCauses = []Cause{CauseMovement, CauseCarried, CauseLamp, CauseWeather, CauseSky, CauseEyes}
```

with (appended LAST, so the snapshot grows only at its end, F19)

```go
var allCauses = []Cause{CauseMovement, CauseCarried, CauseLamp, CauseWeather, CauseSky, CauseEyes, CauseDarkness}
```

- [ ] **Step 5: Attribution, the sent band and the event (`tracker.go`)**

Add `"github.com/GoMudEngine/GoMud/internal/events"` to the imports, directly below `"github.com/GoMudEngine/GoMud/internal/configs"`.

Replace

```go
// Otherwise, the first term that moved, in the order carried light, the
// room's own light, weather, sky.
func attribute(prev record, now observation) Cause {
	if prev.roomId != now.roomId {
		return CauseMovement
	}
	a, b := prev.terms, now.terms
	if now.bandAt != nil && now.bandAt(prev.terms.Level) == now.band {
		return CauseEyes
	}
	switch {
	case a.Carried != b.Carried:
```

with

```go
// Otherwise, the first term that moved, in the order carried darkness,
// carried light, the room's own light, weather, sky. Darkness comes first
// (lighting plan 5d, ruling D4): a darkness arriving or lapsing is the
// deliberate act in the room, and a light that trims around it is not.
func attribute(prev record, now observation) Cause {
	if prev.roomId != now.roomId {
		return CauseMovement
	}
	a, b := prev.terms, now.terms
	if now.bandAt != nil && now.bandAt(prev.terms.Level) == now.band {
		return CauseEyes
	}
	switch {
	case a.Darkened != b.Darkened || termMoved(a.Dark, b.Dark):
		return CauseDarkness
	case a.Carried != b.Carried:
```

Replace

```go
	case skyMoved(a.Sky, b.Sky):
		return CauseSky
	}
	return CauseEyes
}

func skyMoved(a, b float64) bool {
```

with (the helper was sky-only in name; it now serves the darkness term too, and has no other caller, F20)

```go
	case termMoved(a.Sky, b.Sky):
		return CauseSky
	}
	return CauseEyes
}

// termMoved reports whether a light-scale term changed: appeared, went Absent,
// or moved by more than float noise. The sky and a carried darkness share it.
func termMoved(a, b float64) bool {
```

Replace

```go
// Per-player state, in memory only: cleared on logout, never saved.
var (
	mu      sync.Mutex
	records = map[int]record{}
)
```

with

```go
// Per-player state, in memory only: cleared on logout, never saved.
//
// sentBands is the band last handed to GMCP (Char.Sight), kept apart from
// records because a record deliberately keeps its old band while its player
// sleeps or is blinded (decide), and the Game-window border must still follow
// what LightBand reads (lighting plan 5d, ruling D7).
var (
	mu        sync.Mutex
	records   = map[int]record{}
	sentBands = map[int]messaging.Band{}
)
```

Replace (inside `Check`)

```go
	mu.Lock()
	prev, known := records[user.UserId]
	n, speak, next := decide(prev, known, now, trigger)
	records[user.UserId] = next
	mu.Unlock()

	if !speak {
```

with

```go
	mu.Lock()
	prev, known := records[user.UserId]
	n, speak, next := decide(prev, known, now, trigger)
	records[user.UserId] = next
	sent, hadSent := sentBands[user.UserId]
	bandMoved := !hadSent || sent != now.band
	if bandMoved {
		sentBands[user.UserId] = now.band
	}
	mu.Unlock()

	// The Game-window border (lighting plan 5d, ruling D7) rides this check,
	// which already computes the band on the 3d cadence: every command, every
	// combat round, every move, login, and the light commands. Only a change
	// is queued, so a repeat check costs GMCP nothing.
	if bandMoved {
		events.AddToQueue(events.SightBandChanged{UserId: user.UserId})
	}

	if !speak {
```

Replace

```go
	mu.Lock()
	delete(records, userId)
	mu.Unlock()
}

// ResetForTest unloads the store and clears every record.
func ResetForTest() {
	mu.Lock()
	records = map[int]record{}
	mu.Unlock()
```

with

```go
	mu.Lock()
	delete(records, userId)
	delete(sentBands, userId)
	mu.Unlock()
}

// ResetForTest unloads the store and clears every record.
func ResetForTest() {
	mu.Lock()
	records = map[int]record{}
	sentBands = map[int]messaging.Band{}
	mu.Unlock()
```

- [ ] **Step 6: The store file**

Create `_datafiles/world/dogmud/narration/light-notices/darkness.yaml` (every transition, two lines each, one line of 80 columns or fewer, no numbers, no dashes, no tokens; `validateLine` enforces all four at load):

```yaml
cause: darkness
transitions:
  darker_faces:
    any:
      - 'Something drinks the glare away, and your eyes ease.'
      - 'A darkness dims the glaring light; your eyes stop aching.'
  darker_shapes:
    any:
      - 'A darkness drinks the light, and faces blur into shapes.'
      - 'The light thins as something drinks it; you make out only shapes.'
  darker_dark:
    any:
      - 'A darkness swallows the light, and you can see nothing.'
      - 'The light is drunk away, and darkness closes over you.'
  lighter_shapes:
    any:
      - 'The darkness thins; shapes come back out of the black.'
      - 'The unnatural dark lifts a little; you can make out shapes.'
  lighter_faces:
    any:
      - 'The darkness lifts, and faces are clear again.'
      - 'The drinking dark falls away; you can make out faces again.'
  dazzled:
    any:
      - 'The darkness lifts all at once, and the glare stabs at your eyes.'
      - 'With the dark gone, the light floods back into a glare that hurts.'
```

- [ ] **Step 7: Run them to see them pass**

Run: `go build ./... && go test ./internal/events/ ./internal/lightnotice/ ./internal/hooks/ ./internal/usercommands/ -count=1`
Expected: `ok` for all four (`hooks/LightNotice_Triggers_test.go:38` and `usercommands/light_notice_wiring_test.go:21` walk `lightnotice.Causes()` and now find the darkness file too).

- [ ] **Step 8: Re-record the notice golden, additions only**

Run: `go test ./internal/narration/ -run TestSnapshotStores/light_notices -count=1`
Expected: FAIL, `golden mismatch for store "light_notices.golden"`, first differing line 88 (the first `darkness|` line).

Run: `go test ./internal/narration/ -run TestSnapshotStores/light_notices -update -count=1`, then `git diff --stat internal/narration/testdata/stores/`.
Expected: exactly ` internal/narration/testdata/stores/light_notices.golden | 12 ++++++++++++`, and `git diff internal/narration/testdata/stores/light_notices.golden | grep "^-[^-]"` prints nothing. Any other golden in the stat, or any removed line, is a finding: stop.

Run: `go test ./internal/narration/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/narration`.

- [ ] **Step 9: Commit**

```bash
git add internal/events/eventtypes.go internal/events/events.go internal/lightnotice/store.go internal/lightnotice/tracker.go internal/lightnotice/darkness_test.go _datafiles/world/dogmud/narration/light-notices/darkness.yaml internal/narration/testdata/stores/light_notices.golden
git commit -m "feat(lightnotice): a darkness cause, and SightBandChanged on a band change (5d D4, D7)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: `Char.Sight`, one GMCP field carrying the player's band (ruling D7)

**Model:** haiku (the module's own pattern, code given).

**Files:**
- Modify: `modules/gmcp/gmcp.Char.go` (`init` `:79-81`, `GetCharNode` after the `Char.Conditions` block `:678-685`, `GMCPCharModule_Payload` `:713-724`)
- Create: `modules/gmcp/gmcp.CharSight_test.go`

- [ ] **Step 1: Write the failing tests**

Create `modules/gmcp/gmcp.CharSight_test.go`:

```go
package gmcp

import (
	"encoding/json"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// sightViewer seeds one sky-less room lit by exactly its lamp (0 for an unlit
// cave) and one viewer standing in it.
func sightViewer(t *testing.T, lamp int) *users.UserRecord {
	t.Helper()
	zero := 0.0
	room := &rooms.Room{RoomId: 9710, Zone: "SightZone", SkyLight: &zero}
	if lamp > 0 {
		room.Lamp = rooms.LampPtr(lamp)
	}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9710: room},
		map[string]*rooms.ZoneConfig{"SightZone": {Name: "SightZone", RoomId: 9710, RoomIds: map[int]struct{}{9710: {}}}},
	))
	viewer := users.NewTestUser(9711, "sighted", "Sighted", 97711)
	viewer.Character.RoomId = 9710
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9711: viewer}))
	room.AddPlayer(9711)
	return viewer
}

// Char.Sight carries only the player's own band, as messaging.Band names it
// (lighting plan 5d, ruling D7).
func TestCharSightCarriesThePlayersBand(t *testing.T) {
	cases := []struct {
		lamp int
		want string
	}{
		{0, "dark"},
		{30, "shapes"},
		{60, "faces"},
		{90, "dazzled"},
	}
	g := &GMCPCharModule{}
	for _, c := range cases {
		viewer := sightViewer(t, c.lamp)
		data, moduleName := g.GetCharNode(viewer, `Char.Sight`)
		require.Equal(t, `Char.Sight`, moduleName)
		sight, ok := data.(*GMCPCharModule_Payload_Sight)
		require.True(t, ok, "Char.Sight payload must be *GMCPCharModule_Payload_Sight, got %T", data)
		require.Equal(t, c.want, sight.Band, "lamp %d", c.lamp)

		raw, err := json.Marshal(sight)
		require.NoError(t, err)
		require.JSONEq(t, `{"band":"`+c.want+`"}`, string(raw), "Char.Sight must be exactly one field")
	}
}

// A full Char push carries Sight too, so a client that logs in or refreshes
// draws the border without waiting for a band change.
func TestFullCharPayloadCarriesSight(t *testing.T) {
	viewer := sightViewer(t, 60)
	data, moduleName := (&GMCPCharModule{}).GetCharNode(viewer, `Char`)
	require.Equal(t, `Char`, moduleName)
	payload, ok := data.(GMCPCharModule_Payload)
	require.True(t, ok, "Char payload type %T", data)
	require.NotNil(t, payload.Sight)
	require.Equal(t, "faces", payload.Sight.Band)
}

// The SightBandChanged event becomes one Char.Sight update for that player.
func TestSightBandChangedQueuesCharSight(t *testing.T) {
	events.ProcessEvents() // drop anything queued before the capture
	var got []GMCPCharUpdate
	id := events.RegisterListener(GMCPCharUpdate{}, func(e events.Event) events.ListenerReturn {
		if u, ok := e.(GMCPCharUpdate); ok && u.UserId == 9712 {
			got = append(got, u)
		}
		return events.Continue
	})
	t.Cleanup(func() { events.UnregisterListener(GMCPCharUpdate{}, id) })

	(&GMCPCharModule{}).sightBandChangedHandler(events.SightBandChanged{UserId: 9712})
	events.ProcessEvents()
	require.Len(t, got, 1)
	require.Equal(t, `Char.Sight`, got[0].Identifier)
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./modules/gmcp/ -run "Sight" -count=1`
Expected: FAIL to build, `undefined: GMCPCharModule_Payload_Sight`, `payload.Sight undefined`, `sightBandChangedHandler undefined`.

- [ ] **Step 3: The listener**

In `modules/gmcp/gmcp.Char.go`, replace

```go
	events.RegisterListener(events.Quest{}, g.questProgressHandler)

}
```

with

```go
	events.RegisterListener(events.Quest{}, g.questProgressHandler)

	// Char.Sight: the player's light band for the web client's Game-window
	// border (lighting plan 5d, ruling D7). lightnotice.Check queues the
	// event only when the band changes.
	events.RegisterListener(events.SightBandChanged{}, g.sightBandChangedHandler)

}

// sightBandChangedHandler pushes Char.Sight when a player's light band
// changes. Not on Char.Vitals: that rides every pool change and would
// recompute the room's light on each.
func (g *GMCPCharModule) sightBandChangedHandler(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.SightBandChanged)
	if !typeOk {
		return events.Continue
	}

	if evt.UserId == 0 {
		return events.Continue
	}

	events.AddToQueue(GMCPCharUpdate{
		UserId:     evt.UserId,
		Identifier: `Char.Sight`,
	})

	return events.Continue
}
```

- [ ] **Step 4: The node**

Replace

```go
		payload.Conditions = buildConditionsPayload(user.Character)

		if !all {
			return payload.Conditions, `Char.Conditions`
		}
	}
```

with

```go
		payload.Conditions = buildConditionsPayload(user.Character)

		if !all {
			return payload.Conditions, `Char.Conditions`
		}
	}

	if all || g.wantsGMCPPayload(`Char.Sight`, gmcpModule) {

		payload.Sight = &GMCPCharModule_Payload_Sight{Band: sightBand(user.Character)}

		if !all {
			return payload.Sight, `Char.Sight`
		}
	}
```

- [ ] **Step 5: The payload type**

Replace

```go
	Conditions map[string]GMCPCondition          `json:"Conditions,omitempty"`
}
```

with

```go
	Conditions map[string]GMCPCondition          `json:"Conditions,omitempty"`
	Sight      *GMCPCharModule_Payload_Sight     `json:"Sight,omitempty"`
}

// /////////////////
// Char.Sight
// /////////////////
//
// The player's own light band, one field (lighting plan 5d, ruling D7):
// "dark", "shapes", "faces" or "dazzled" (messaging.Band.String()). The web
// client tints the Game window's border by it. It carries nothing about the
// room's contents: the text already tells the player how well they see.
type GMCPCharModule_Payload_Sight struct {
	Band string `json:"band"`
}

// sightBand is messaging.LightBand for a character in its current room. A
// room that is not loaded reads as faces, LightBand's own nil-room default;
// the nil is passed as an untyped nil, never a typed nil *rooms.Room, which
// LightBand would dereference.
func sightBand(c *characters.Character) string {
	if room := rooms.LoadRoom(c.RoomId); room != nil {
		return messaging.LightBand(c, room).String()
	}
	return messaging.LightBand(c, nil).String()
}
```

(`characters`, `messaging` and `rooms` are already imported in this file.)

- [ ] **Step 6: Run them to see them pass**

Run: `gofmt -l modules/gmcp/ && go test ./modules/gmcp/ -count=1`
Expected: `gofmt` prints nothing; `ok  	github.com/GoMudEngine/GoMud/modules/gmcp`.

- [ ] **Step 7: Commit**

```bash
git add modules/gmcp/gmcp.Char.go modules/gmcp/gmcp.CharSight_test.go
git commit -m "feat(gmcp): Char.Sight carries the player's light band (5d D7)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: The Game-window border (Rule 7)

**Model:** sonnet (browser code with no JS test harness; the handler-shadowing trap and the pop-out need judgment).

**Files:**
- Create: `tools/webclient-tests/sight-border.js`
- Modify: `_datafiles/html/public/webclient-pure.html` (the `"Char"` handler after its `Char.Quests` delegation `:708-711`; a `"Char.Sight"` handler after `"Char.Vitals"` `:731-733`)
- Modify: `_datafiles/html/public/static/css/dashboard.css` (after `#panel-feed .dash-panel-body` `:131`)

- [ ] **Step 1: Write the failing check**

Create `tools/webclient-tests/sight-border.js` (static analysis in the style of `char-handler-shadowing.js`, zero dependencies):

```javascript
#!/usr/bin/env node
/*
 * Guards the Game-window sight border (lighting plan 5d, Char.Sight).
 *
 *   node tools/webclient-tests/sight-border.js
 *
 * Zero dependencies. Static analysis of webclient-pure.html and
 * dashboard.css; it does not execute the UI, so it needs no DOM.
 *
 * The server pushes Char.Sight {"band": "dark"|"shapes"|"faces"|"dazzled"}
 * when a player's light band changes, and inside every full Char payload.
 * The client must:
 *   1. handle "Char.Sight" and set one sight-<band> class on #panel-feed,
 *      with the tooltip the spec names for each band;
 *   2. delegate to it from the generic "Char" handler, which is the only
 *      handler a full Char push reaches (see char-handler-shadowing.js);
 *   3. style all four classes in dashboard.css, on #panel-feed itself, the
 *      element a pop-out moves into its window.
 */
'use strict';

const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..', '..', '_datafiles', 'html', 'public');
const html = fs.readFileSync(path.join(root, 'webclient-pure.html'), 'utf8');
const css = fs.readFileSync(path.join(root, 'static', 'css', 'dashboard.css'), 'utf8');

const bands = {
  dark: 'Too dark to see.',
  shapes: 'Dim light: shapes, not faces.',
  faces: 'Good light.',
  dazzled: 'Too bright: the glare hurts.',
};

function handlerBody(src, key) {
  const start = src.indexOf('"' + key + '": function()');
  if (start < 0) return null;
  const open = src.indexOf('{', start);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  return null;
}

let fails = 0;
function check(ok, what) {
  console.log((ok ? '  OK        ' : '  FAIL      ') + what);
  if (!ok) fails++;
}

const sight = handlerBody(html, 'Char.Sight');
check(sight !== null, 'a "Char.Sight" handler exists');
if (sight) {
  check(sight.includes('panel-feed'), 'it targets #panel-feed');
  check(sight.includes('"sight-" + band') || sight.includes("'sight-' + band"), 'it sets a sight-<band> class');
  for (const [band, tip] of Object.entries(bands)) {
    check(sight.includes(tip), 'the ' + band + ' tooltip reads "' + tip + '"');
  }
}

const charStart = html.indexOf('"Char":function()');
const charBody = charStart < 0 ? '' : html.slice(charStart, html.indexOf('"Char.Conditions": function()', charStart));
check(charBody.includes("GMCPUpdateHandlers['Char.Sight']"), 'the generic "Char" handler delegates to Char.Sight');

for (const band of Object.keys(bands)) {
  check(new RegExp('#panel-feed\\.sight-' + band + '\\s*\\{[^}]*border-color').test(css),
    'dashboard.css colours #panel-feed.sight-' + band);
}

console.log(fails === 0 ? '\nALL CHECKS PASSED' : '\n' + fails + ' CHECK(S) FAILED');
process.exit(fails ? 1 : 0);
```

- [ ] **Step 2: Run it to see it fail**

Run: `node tools/webclient-tests/sight-border.js`
Expected: `FAIL      a "Char.Sight" handler exists`, the delegation FAIL, the four `dashboard.css colours #panel-feed.sight-...` FAILs, and `6 CHECK(S) FAILED` (exit 1).

- [ ] **Step 3: The handler and its delegation (`webclient-pure.html`)**

Replace

```javascript
                if ( obj.Quests ) {
                    GMCPUpdateHandlers['Char.Quests']();
                }
```

with

```javascript
                if ( obj.Quests ) {
                    GMCPUpdateHandlers['Char.Quests']();
                }

                // Char.Sight rides the full payload too, so the Game window's
                // border is right on login and refresh, not only after the
                // next change of light.
                if ( obj.Sight ) {
                    GMCPUpdateHandlers['Char.Sight']();
                }
```

Replace

```javascript
            "Char.Vitals": function() {
                updateVitalsWindow();
            },
```

with

```javascript
            "Char.Vitals": function() {
                updateVitalsWindow();
            },
            // How well the player can see, as the Game window's border
            // (lighting plan 5d). The band is the player's own; the class
            // goes on #panel-feed itself, which a pop-out MOVES into its
            // window (WinBox mount), so the border follows it there.
            "Char.Sight": function() {
                var sight = (GMCPStructs["Char"] || {}).Sight;
                var band = sight ? sight.band : "";
                var tips = {
                    "dark":    "Too dark to see.",
                    "shapes":  "Dim light: shapes, not faces.",
                    "faces":   "Good light.",
                    "dazzled": "Too bright: the glare hurts."
                };
                var panel = document.getElementById("panel-feed");
                if ( !panel ) {
                    return;
                }
                for ( var b in tips ) {
                    panel.classList.remove("sight-" + b);
                }
                var head = panel.querySelector(".dash-panel-head");
                if ( !tips.hasOwnProperty(band) ) {
                    if ( head ) head.removeAttribute("title");
                    return;
                }
                panel.classList.add("sight-" + band);
                if ( head ) head.title = tips[band];
            },
```

- [ ] **Step 4: The four tones (`dashboard.css`)**

Replace

```css
#panel-feed .dash-panel-body { overflow: hidden; }   /* xterm manages its own scroll */
```

with

```css
#panel-feed .dash-panel-body { overflow: hidden; }   /* xterm manages its own scroll */
/* Sight border (lighting plan 5d, Char.Sight): the Game window's border shows
   how well the player can see. The class sits on #panel-feed, which a pop-out
   moves into its WinBox, so the border follows it there. Faces is today's
   antique gold, so a client without Char.Sight looks as it always did. */
#panel-feed {
  --sight-dark:    #3a2d17;   /* near black, a darkened antique gold */
  --sight-shapes:  #7a6232;   /* dim, a muted gold */
  --sight-faces:   var(--antique-gold, #b89047);
  --sight-dazzled: #f6e7b0;   /* bright, a pale hot gold */
  transition: border-color 0.4s ease;
}
#panel-feed.sight-dark    { border-color: var(--sight-dark); }
#panel-feed.sight-shapes  { border-color: var(--sight-shapes); }
#panel-feed.sight-faces   { border-color: var(--sight-faces); }
#panel-feed.sight-dazzled { border-color: var(--sight-dazzled); box-shadow: 0 0 6px var(--sight-dazzled); }
```

- [ ] **Step 5: Run the checks**

Run: `node tools/webclient-tests/sight-border.js`
Expected: every line `OK`, then `ALL CHECKS PASSED`.

Run: `node tools/webclient-tests/char-handler-shadowing.js`
Expected: `OK        Char.Sight  (delegates)` among the lines, then `ALL CHECKS PASSED`.

Run the other four for regressions: `node tools/webclient-tests/quest-panel-refresh.js`, `node tools/webclient-tests/vitals-rows.js`, `node tools/webclient-tests/trigger-status-key-match.js`, `node tools/webclient-tests/zone-crossing-center.js`. Expected: each exits 0, as before.

- [ ] **Step 6: Commit**

```bash
git add tools/webclient-tests/sight-border.js _datafiles/html/public/webclient-pure.html _datafiles/html/public/static/css/dashboard.css
git commit -m "feat(webclient): the Game window's border shows how well you can see (5d Rule 7)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

The live look (four tones, the tooltip, the pop-out) is checked in Task 13's playtest.

---

### Task 11: Content: Chrysalis Pall, the Umbral Lantern, the Phantom's lair, and `help darkness` (Rules 4, 5, 6, 8; D5, D9)

**Model:** sonnet (world YAML under the boot-panic rules, three narration goldens, one guard registration and a shipped-world integration test).

Load `dogmud-authoring-content` and `dogmud-player-copy` before this task. Every file below was run through the boot (F28 is how the `vendor_categories` rule was found). Scan every new YAML for an unquoted `: ` inside text before committing (none below has one; the `description:` continuation lines carry no colon).

**Files:**
- Create: `_datafiles/world/dogmud/spells/chrysalis-pall.yaml`
- Create: `_datafiles/world/dogmud/conditions/131-chrysalis_pall.yaml`, `132-umbral_dark.yaml`, `133-phantom_heat_sense.yaml`
- Create: `_datafiles/world/dogmud/items/armor-20000/light/20098-umbral_lantern.yaml`
- Create: `_datafiles/world/dogmud/templates/help/darkness.template`, `chrysalis-pall.template`
- Modify: `_datafiles/world/dogmud/mobs/thornwall_city/272-chrysalis_phantom.yaml:3,76-78`
- Modify: `_datafiles/world/dogmud/keywords.yaml:165-167,303-304`
- Modify: `_datafiles/world/dogmud/templates/help/light.template:58`, `seasons.template:17`, `moons.template:16`
- Modify: `internal/items/shipped_light_items_test.go:63-66`
- Create: `internal/spells/chrysalis_pall_test.go`, `internal/behaviortree/phantom_lair_test.go`
- Modify: `shipped_narration_data_guard_test.go` (`observerIdentityGuardContentSafeViaCode`, after the `130-pitsense_tincture.yaml` entry)
- Modify: `internal/narration/testdata/stores/conditions.golden`, `spells.golden` (re-recorded)

- [ ] **Step 1: Confirm the ids are still free**

Run: `python tools/id_inventory.py --alloc conditions 3`
Expected: `conditions: reserve IDs 131-133 (3 IDs)`. If master has moved and this prints anything else, stop and re-plan the ids (F35).

Run (each standalone; an empty result exits 1): `grep -rln "^conditionid: 13[1-3]$" _datafiles/`, `grep -rln "^itemid: 20098$" _datafiles/`, `grep -rn "Umbral\|chrysalis-pall" _datafiles/world`.
Expected: no output from any of the three; and `grep -rln "^itemid: 20097$" _datafiles/` prints the hooded lantern's file, proving the search can match.

- [ ] **Step 2: Write the failing tests**

In `internal/items/shipped_light_items_test.go`, replace

```go
	if spec := items.GetItemSpec(20097); spec == nil || spec.Nouns["hood"] == "" {
		t.Error("the hooded lantern must carry a hood noun")
	}
}
```

with

```go
	if spec := items.GetItemSpec(20097); spec == nil || spec.Nouns["hood"] == "" {
		t.Error("the hooded lantern must carry a hood noun")
	}

	// Lighting plan 5d: the Umbral Lantern is the ladder's one darkness, a
	// light-slot item whose one secret condition takes 50 away, adjustable,
	// with no hood and never a light.
	umbral := items.GetItemSpec(20098)
	if umbral == nil {
		t.Fatal("item 20098 (Umbral Lantern) is not shipped")
	}
	if umbral.Type != items.Light || umbral.Subtype != items.Wearable || len(umbral.WornConditionIds) != 1 {
		t.Fatalf("Umbral Lantern is %s/%s with %d conditions, want light/wearable with 1", umbral.Type, umbral.Subtype, len(umbral.WornConditionIds))
	}
	dark := conditions.GetConditionSpec(umbral.WornConditionIds[0])
	if dark == nil || !dark.Secret || dark.IsLightSource() || !dark.IsDarknessSource() {
		t.Fatalf("the Umbral Lantern's condition must exist, be secret, and be a darkness and not a light: %+v", dark)
	}
	if v := dark.Effects[conditions.EffectDarknessStrength]; v.UsesMagnitude || v.Literal != 50 {
		t.Errorf("Umbral Lantern darkens at %+v, want 50", v)
	}
	adjustable := false
	for _, f := range dark.Flags {
		adjustable = adjustable || f == conditions.Adjustable
	}
	if !adjustable {
		t.Error("the Umbral Lantern's darkness must be adjustable")
	}
	if len(umbral.Nouns) != 0 {
		t.Errorf("the Umbral Lantern carries nouns %v, want none (it has no hood)", umbral.Nouns)
	}
	// Boot refuses an economy item with no vendor_categories
	// (items.ValidateVendorCategories); the list says who buys it, not who
	// stocks it, so the boss drop still sells.
	if len(umbral.VendorCategories) != 1 || umbral.VendorCategories[0] != "blacksmithing" {
		t.Errorf("Umbral Lantern vendor_categories = %v, want [blacksmithing], the hooded lantern's", umbral.VendorCategories)
	}
}
```

Create `internal/spells/chrysalis_pall_test.go`:

```go
package spells

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Chrysalis Pall (lighting plan 5d, Rule 4, ruling D9) as shipped: mental,
// single, willpower, cost 50, three wait rounds, condition 131, and found by
// discovery at spellcasting 25 (the shipped and Go-default
// SpellDiscoverySkillPerDifficulty is 1.0), never taught.
func TestShippedChrysalisPallIsDiscoveredAtSpellcasting25(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/spells/chrysalis-pall.yaml")
	require.NoError(t, err)
	var sp SpellData
	require.NoError(t, yaml.Unmarshal(data, &sp))
	require.Equal(t, "chrysalis-pall", sp.SpellId, "read the wrong file, so this test proves nothing")

	assert.Equal(t, "Chrysalis Pall", sp.Name)
	assert.Equal(t, []string{"pall"}, sp.Aliases)
	assert.True(t, sp.HasSchool(SchoolMental))
	assert.Equal(t, 50, sp.Cost)
	assert.Equal(t, 3, sp.WaitRounds)
	assert.Equal(t, 25, sp.Difficulty)
	assert.Equal(t, []int{131}, sp.ConditionIds)

	restore := allSpells
	t.Cleanup(func() { allSpells = restore })
	allSpells = map[string]*SpellData{sp.SpellId: &sp}
	assert.NotContains(t, GetEligibleSpells(map[string]int{}, 24, SchoolMental), "chrysalis-pall",
		"spellcasting 24 must not discover the pall")
	assert.Contains(t, GetEligibleSpells(map[string]int{}, 25, SchoolMental), "chrysalis-pall",
		"spellcasting 25 discovers the pall")
}
```

Create `internal/behaviortree/phantom_lair_test.go` (the spec's lair test on the shipped world, through the real spawn path `Room.Prepare`, F6):

```go
package behaviortree

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// The Chitin Throne as shipped (lighting plan 5d, Rule 6): room 508 spawns the
// Chrysalis Phantom (272) through the real spawn path (Room.Prepare), which
// lists the mob without trimming, so its Umbral Lantern is at full strength
// and the lair reads -50. Its heat sense (condition 133, reach 50) reads
// shapes at exactly -50, so under ruling D8 it can act; a normal player and a
// normal-eyed mob there read nothing.
func TestThePhantomsLairIsBlack(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	// fileloader and ReloadConfig resolve "_datafiles/..." against the CWD,
	// which a shared test binary does not set to this package's directory
	// (dogmud-writing-tests). Anchor on this file and chdir to the repo root.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	require.NoError(t, err)
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoRoot))
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// The real shipped config (config.yaml): a bare test binary would read
	// Network.LogoutRounds as 0 and conditions.LoadDataFiles would panic on
	// condition 0. SetConfigForTest restores the pre-test config afterwards.
	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())

	// Each loader replaces its registry with no restore of its own; seeding a
	// throwaway first captures the pre-test registry for a real restore.
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(species.SeedSpeciesForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(rooms.SeedBiomesForTest(nil))
	conditions.LoadDataFiles()
	species.LoadDataFiles()
	items.LoadDataFiles()
	rooms.LoadBiomeDataFiles()

	dataFiles := string(configs.GetFilePathsConfig().DataFiles)
	thornwall, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataFiles + `/mobs/thornwall_city`)
	require.NoError(t, err)
	phantomSpec, ok := thornwall[272]
	require.True(t, ok, "mob 272 (Chrysalis Phantom) is not shipped")
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{272: phantomSpec}, map[int]*mobs.Mob{}))

	raw, err := os.ReadFile(dataFiles + `/rooms/thornwall_city/508.yaml`)
	require.NoError(t, err)
	lair := &rooms.Room{}
	require.NoError(t, yaml.Unmarshal(raw, lair))
	require.Equal(t, 508, lair.RoomId, "read the wrong room file, so this test proves nothing")
	require.Equal(t, "cave", lair.Biome)
	require.Nil(t, lair.Lamp, "the lair must have no lamp of its own")
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{508: lair},
		map[string]*rooms.ZoneConfig{lair.Zone: {Name: lair.Zone, RoomId: 508, RoomIds: map[int]struct{}{508: {}}}}))

	require.Equal(t, 0, lair.LightLevel(), "before the spawn the lair is an unlit cave")
	lair.Prepare(false)
	mobIds := lair.GetMobs()
	require.Len(t, mobIds, 1, "room 508 must spawn exactly its Phantom")
	phantom := mobs.GetInstance(mobIds[0])
	require.NotNil(t, phantom)
	t.Cleanup(func() { mobs.SetInstanceForTest(mobIds[0], nil) })
	require.Equal(t, 272, int(phantom.MobId))

	require.Equal(t, 20098, phantom.Character.Equipment.Light.ItemId, "the Phantom carries the Umbral Lantern in its light slot")
	require.Equal(t, 25, phantom.Character.Equipment.Light.DropChance, "the lantern drops one kill in four")
	require.Equal(t, -50, lair.LightLevel(), "the lantern is at full strength after a spawn: 0 before, -50 after")
	require.Equal(t, 50, phantom.Character.InfraReach(), "the Phantom's heat sense reaches 50")
	require.Equal(t, messaging.BandShapes, messaging.LightBand(&phantom.Character, lair), "the Phantom reads shapes in its own dark")
	require.True(t, mobCanSee(phantom, lair), "ruling D8: the Phantom acts on what it makes out, so its ambush fires")

	player := users.NewTestUser(8140, "delver", "Delver", 98140)
	require.Equal(t, messaging.BandDark, messaging.LightBand(player.Character, lair), "a normal player sees nothing in the lair")

	normal := &mobs.Mob{MobId: 8141, InstanceId: 8142,
		Character: characters.Character{Name: "Rat", RoomId: 508, Conditions: conditions.New(), SpeciesId: 1}}
	require.False(t, mobCanSee(normal, lair), "a normal-eyed mob at -50 is below the window floor: D8 does not reach it")
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/items/ -run TestShippedLightItemsMatchTheLadder -count=1`
Expected: FAIL, `item 20098 (Umbral Lantern) is not shipped`.

Run: `go test ./internal/spells/ -run ChrysalisPall -count=1`
Expected: FAIL, `open ../../_datafiles/world/dogmud/spells/chrysalis-pall.yaml: The system cannot find the file specified.`

Run: `go test ./internal/behaviortree/ -run TestThePhantomsLairIsBlack -count=1`
Expected: FAIL at `the Phantom carries the Umbral Lantern in its light slot` (`expected: 20098, actual: 0`).

- [ ] **Step 4: The spell and its condition**

Create `_datafiles/world/dogmud/spells/chrysalis-pall.yaml` (the filename is the spellid):

```yaml
# Found by discovery, never taught (lighting plan 5d). Glow's twin: spores
# that drink light where glow's spores give it. Costs more and needs more
# spellcasting than Night Vision, less than Heat Sight (owner ruling D9).
spellid: chrysalis-pall
name: Chrysalis Pall
aliases: [pall]
description: Coaxes Chrysalis spores to drink the light around the target.
attack_type: none
damage_type: non_harm
targeting: single
schools:
  - mental
cost: 50
waitrounds: 3
difficulty: 25
primarystat: willpower
effect_type: condition
condition_ids:
  - 131
cast_actor: "You coax the Chrysalis spores around you to drink the light."
cast_observer: "{actor} concentrates as the air around them dims and thickens."
wait_actor: "You hold the image steady as the spores darken..."
```

Create `_datafiles/world/dogmud/conditions/131-chrysalis_pall.yaml`:

```yaml
conditionid: 131
name: Chrysalis Pall
description: Dark spores drink the light around you. The place grows darker
  for everyone, you included. Each time you move, the pall eases to what your
  own eyes can still use, and a fresh pall starts at full strength. The warmth
  of living things still shows through it to eyes that see heat.
secret: false
triggerrate: 5 real minutes
triggercount: 4
# darkness_strength: magnitude, scaled by the Chrysalis Pall spell
# (LightDarknessSpell*); not capped, the scale clamps the room at -100.
effects:
  darkness_strength: magnitude
flags:
  - adjustable
  - cancellable
start_actee: "A pall of dark spores gathers around you."
start_observer: "A pall of dark spores gathers around {actee_plain}."
end_actee: "Your pall thins, and the light comes back."
end_observer: "The pall around {actee_plain} thins away."
```

- [ ] **Step 5: The lantern and its condition**

Create `_datafiles/world/dogmud/conditions/132-umbral_dark.yaml`:

```yaml
conditionid: 132
name: Umbral Dark
description: Your Umbral Lantern drinks the light around you, trimmed to your
  eyes.
secret: true
triggerrate: 5 real minutes
triggercount: 1
effects:
  darkness_strength: 50
flags:
  - adjustable
```

Create `_datafiles/world/dogmud/items/armor-20000/light/20098-umbral_lantern.yaml` (`vendor_categories` is required by the boot, F28; `value: 60` is this plan's number, see "Where the spec could not be implemented as written" item 2):

```yaml
itemid: 20098
name: Umbral Lantern
namesimple: lantern
description: A lantern of black glass and tarnished iron. Inside it a knot of
  dark spores drinks the light instead of giving it. It trims its darkness to
  your eyes each time you step somewhere new.
type: light
subtype: wearable
weight: 0.3
value: 60
# A boss drop, never stocked: vendor_categories says who BUYS it (items
# ValidateVendorCategories refuses an empty list at boot), not who sells it.
vendor_categories:
- blacksmithing
wornconditionids:
  - 132
```

- [ ] **Step 6: The Phantom's heat sense and its lantern**

Create `_datafiles/world/dogmud/conditions/133-phantom_heat_sense.yaml`:

```yaml
conditionid: 133
name: Phantom Heat Sense
description: "You sense the warmth of living things through any darkness."
# The Chrysalis Phantom's heat sense (lighting plan 5d, ruling D9): reach 50,
# so it reads shapes at exactly the -50 its own Umbral Lantern makes of its
# lair. No nightvision_strength, so it is dazzled no sooner than anyone.
# A pure flag, the shape of condition 85: no trigger rate, so it never ticks,
# and triggercount 1 so it is not born expired.
triggercount: 1
effects:
  infra_reach: 50
flags:
  - infraredvision
secret: true
```

In `_datafiles/world/dogmud/mobs/thornwall_city/272-chrysalis_phantom.yaml`, replace

```yaml
conditionids: [9]
```

with

```yaml
conditionids: [9, 133]
```

and replace

```yaml
    offhand:
      itemid: 10024
  items:
```

with

```yaml
    offhand:
      itemid: 10024
    # Lighting plan 5d: the Umbral Lantern keeps the Chitin Throne black
    # while the Phantom lives (508 reads -50), and its heat sense (condition
    # 133, reach 50) reads shapes there. dropchance overrides the mob's
    # itemdropchance for this item only: one kill in four.
    light:
      itemid: 20098
      dropchance: 25
  items:
```

`maxwander: 0` stays (Rule 6).

- [ ] **Step 7: Help (D5)**

Create `_datafiles/world/dogmud/templates/help/darkness.template`:

```text
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">darkness</ansi>

Darkness is light taken away. A darkness can make a place darker than
the deepest cave, darker than any night, and it darkens it for everyone
there, whoever brought it.

Two darknesses together are darker than one, but not twice as dark,
just as a second light adds less than the first did.

<ansi fg="yellow">Where darkness comes from</ansi>

  The <ansi fg="command">Chrysalis Pall</ansi> spell gathers dark spores that drink the light
  around its target. The <ansi fg="command">Umbral Lantern</ansi>, a rare find, does the same
  from the light slot. Both are darkest when they are new: a fresh pall
  or a lantern just put on is at full strength. Each time you move, they
  ease to what your own eyes can still use there.

  Lights already in a place do not push back when a darkness arrives.
  A light carried in afterwards adds to the room as it always does.

<ansi fg="yellow">Seeing through it</ansi>

  Night vision helps only a little: it cannot read what is not there.
  Infravision is the answer. It sees the warmth of living things, not
  light, so it shows you shapes in darkness no eye can use, as deep as
  its reach.

  A darkness is not a light. It never gives you away when you sneak.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help light</ansi>, <ansi fg="command">help chrysalis-pall</ansi>, <ansi fg="command">help heat-sight</ansi>,
  <ansi fg="command">help night-vision</ansi>, <ansi fg="command">help seasons</ansi>, <ansi fg="command">help moons</ansi>
```

Create `_datafiles/world/dogmud/templates/help/chrysalis-pall.template` (the `heat-sight.template` shape, including its `Conv. Cost` detail row):

```text
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">chrysalis-pall</ansi> spell

The <ansi fg="command">Chrysalis Pall</ansi> spell coaxes Chrysalis spores to drink the light
around its target, the dark twin of Chrysalis Glow. It darkens the
place for everyone there, the target included. The stronger your
willpower and your spellcasting, the deeper the dark and the longer it
lasts.

A fresh pall is at full strength. Each time its holder moves, it eases
to what their own eyes can still use there. Use <ansi fg="command">cancel pall</ansi> to end
it early. It is found, never taught.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">cast chrysalis-pall</ansi> [target]

<ansi fg="yellow">Details: </ansi>

  <ansi fg="yellow">Type:        </ansi> Help Single
  <ansi fg="yellow">School:      </ansi> Mental
  <ansi fg="yellow">Conv. Cost:  </ansi> 50
  <ansi fg="yellow">Effect:      </ansi> Takes the light away around its target

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help darkness</ansi>, <ansi fg="command">help chrysalis-glow</ansi>, <ansi fg="command">help spells</ansi>
```

In `_datafiles/world/dogmud/keywords.yaml`, replace

```yaml
      - light
      - moons
      - seasons
```

with

```yaml
      - light
      - moons
      - seasons
      - darkness
```

and replace

```yaml
  light:            [lighting, lantern, lanterns, torch, torches, candle, candles,
                     dazzle, dazzled, dark, darkness]
```

with

```yaml
  light:            [lighting, lantern, lanterns, torch, torches, candle, candles,
                     dazzle, dazzled, dark]
  darkness:         [umbral, pall]
```

In `_datafiles/world/dogmud/templates/help/light.template`, replace the last line

```text
  <ansi fg="command">help biome</ansi>, <ansi fg="command">help equipment</ansi>
```

with

```text
  <ansi fg="command">help biome</ansi>, <ansi fg="command">help equipment</ansi>, <ansi fg="command">help darkness</ansi>
```

In `_datafiles/world/dogmud/templates/help/seasons.template`, replace the last line

```text
<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help light</ansi>, <ansi fg="command">help moons</ansi>, <ansi fg="command">help weather</ansi>, <ansi fg="command">help biome</ansi>
```

with

```text
<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help light</ansi>, <ansi fg="command">help moons</ansi>, <ansi fg="command">help weather</ansi>, <ansi fg="command">help biome</ansi>,
  <ansi fg="command">help darkness</ansi>
```

In `_datafiles/world/dogmud/templates/help/moons.template`, replace the last line

```text
<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help light</ansi>, <ansi fg="command">help seasons</ansi>, <ansi fg="command">help weather</ansi>
```

with

```text
<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help light</ansi>, <ansi fg="command">help seasons</ansi>, <ansi fg="command">help weather</ansi>,
  <ansi fg="command">help darkness</ansi>
```

Check the rendered width (tags stripped) of both new templates:

```bash
for f in _datafiles/world/dogmud/templates/help/darkness.template _datafiles/world/dogmud/templates/help/chrysalis-pall.template; do sed 's/<[^>]*>//g' "$f" | awk -v F="$f" 'length($0) > 80 { print F": "length($0) }'; done
```

Expected: no output.

- [ ] **Step 8: Run the content tests**

Run: `go test ./internal/items/ ./internal/spells/ ./internal/behaviortree/ -count=1`
Expected: `ok` for all three. The lair test reads 0 before the spawn and -50 after, `InfraReach()` 50, shapes for the Phantom, dark for a player, the lantern at drop chance 25, `mobCanSee` true for the Phantom and false for a normal-eyed mob.

Prove the lair test can fail: change `conditionids: [9, 133]` back to `[9]` in `272-chrysalis_phantom.yaml`, run `go test ./internal/behaviortree/ -run TestThePhantomsLairIsBlack -count=1`, expect FAIL at `the Phantom's heat sense reaches 50`, then restore `[9, 133]` and re-run: `ok`.

- [ ] **Step 9: Register 131 with the observer-name guard, and re-record two goldens**

Run: `go test . -run TestObserverIdentityTagsAreAnonymizable -count=1`
Expected: FAIL, two lines `_datafiles/world/dogmud/conditions/131-chrysalis_pall.yaml:18` and `:20`: `{actee_plain} is not anonymizable` (F30).

In `shipped_narration_data_guard_test.go`, replace

```go
	"conditions/130-pitsense_tincture.yaml": true,
```

with

```go
	"conditions/130-pitsense_tincture.yaml": true,
	// Lighting plan 5d: Chrysalis Pall. Its start line goes out through
	// sendConditionStartRoomText, which as a darkness routes it through
	// SendTextVisualAsLitHidingNames (no SightShapes tier, ruling D6); its
	// end line through sendConditionEndRoomText with the holder's plain name.
	"conditions/131-chrysalis_pall.yaml": true,
```

Run: `gofmt -l . && go test . -run TestObserverIdentityTagsAreAnonymizable -count=1`
Expected: `gofmt` prints nothing; `ok`.

Run: `go test ./internal/narration/ -run TestSnapshotStores -count=1`
Expected: FAIL in `TestSnapshotStores/conditions` (first differing line 289, `got: condition|131|start_actee => A pall of dark spores gathers around you.`) and `TestSnapshotStores/spells` (line 23, `got: spell|chrysalis-pall|cast_actor => ...`).

Run: `go test ./internal/narration/ -run "TestSnapshotStores/(conditions|spells)$" -update -count=1`, then `git diff --stat internal/narration/testdata/stores/`.
Expected: `conditions.golden | 4 ++++` and `spells.golden | 3 +++` (plus Task 8's already-committed light notices, not in this diff); `git diff internal/narration/testdata/stores/ | grep "^-[^-]"` prints nothing. The seven added lines are:

```text
condition|131|start_actee => A pall of dark spores gathers around you.
condition|131|start_observer => A pall of dark spores gathers around Aliceia.
condition|131|end_actee => Your pall thins, and the light comes back.
condition|131|end_observer => The pall around Aliceia thins away.
spell|chrysalis-pall|cast_actor => You coax the Chrysalis spores around you to drink the light.
spell|chrysalis-pall|cast_observer => <ansi fg="username">Aliceia</ansi> concentrates as the air around them dims and thickens.
spell|chrysalis-pall|wait_actor => You hold the image steady as the spores darken...
```

- [ ] **Step 10: Run the root and the stores**

Run: `go test . ./internal/narration/ ./internal/usercommands/ ./internal/devtools/ -count=1`
Expected: `ok` for all four (the help completeness checks included).

- [ ] **Step 11: Commit**

```bash
git add _datafiles/world/dogmud/spells/chrysalis-pall.yaml _datafiles/world/dogmud/conditions/131-chrysalis_pall.yaml _datafiles/world/dogmud/conditions/132-umbral_dark.yaml _datafiles/world/dogmud/conditions/133-phantom_heat_sense.yaml _datafiles/world/dogmud/items/armor-20000/light/20098-umbral_lantern.yaml _datafiles/world/dogmud/mobs/thornwall_city/272-chrysalis_phantom.yaml _datafiles/world/dogmud/templates/help/darkness.template _datafiles/world/dogmud/templates/help/chrysalis-pall.template _datafiles/world/dogmud/keywords.yaml _datafiles/world/dogmud/templates/help/light.template _datafiles/world/dogmud/templates/help/seasons.template _datafiles/world/dogmud/templates/help/moons.template internal/items/shipped_light_items_test.go internal/spells/chrysalis_pall_test.go internal/behaviortree/phantom_lair_test.go shipped_narration_data_guard_test.go internal/narration/testdata/stores/conditions.golden internal/narration/testdata/stores/spells.golden
git commit -m "feat(content): Chrysalis Pall, the Umbral Lantern and a black Chitin Throne (5d Rules 4 to 6, 8)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: Docs, full gate, race run, boot check

**Model:** sonnet.

**Files:**
- Modify: `context.md` in `internal/lightscale`, `internal/configs`, `internal/conditions`, `internal/characters`, `internal/messaging`, `internal/rooms`, `internal/hooks`, `internal/usercommands`, `internal/mobcommands`, `internal/behaviortree`, `internal/lightnotice`, `internal/events`, `modules/gmcp`
- Modify: `docs/PATCH_NOTES.md`, `docs/README.md`

- [ ] **Step 1: Verify every symbol before naming it**

Run (PowerShell): `Select-String -Path internal\lightscale\trim.go,internal\conditions\light.go,internal\conditions\conditionspec.go,internal\conditions\scaled_magnitude.go,internal\characters\light.go,internal\messaging\window.go,internal\rooms\lighting.go,internal\lightnotice\store.go,internal\events\eventtypes.go,internal\events\events.go,modules\gmcp\gmcp.Char.go -Pattern '^(func|type|const|var)\s'`
Expected to include: `func Trim(step, others, max, target float64) float64`, `func TrimDarkness(step, light, otherDark, max, floor float64) float64`, `func (bs *Conditions) DarknessSources() []*Condition`, `func (bs *Conditions) LightAndDarknessSources() []*Condition`, `func (b *ConditionSpec) IsDarknessSource() bool`, `func AnyDarknessSource(conditionIds []int) bool`, `func SpellScaledTriggers(kind EffectKind, stat, skill float64) int`, `func (c *Character) DarknessTerms() []float64`, `func DarknessTrimTarget(strength, reach, blindBelow int) float64`, `func (r *Room) carriedTerms(exclude *conditions.Condition) (light, dark []float64)`, `type SightBandChanged struct {`, `func DrainQueuedSightBandChangedForTest(userId int) []SightBandChanged`, `type GMCPCharModule_Payload_Sight struct {`, `func sightBand(c *characters.Character) string`. Name nothing that is not in that output. (`CauseDarkness` is a `const` inside a block, `sendConditionStartRoomText` is in `internal/hooks/Condition_ApplyConditions.go`, `mobCanSee` in `internal/behaviortree/sight.go`, `sentBands` and `termMoved` in `internal/lightnotice/tracker.go`: grep each.)

- [ ] **Step 2: `internal/lightscale/context.md`**

Replace the whole file with:

```markdown
# internal/lightscale

The arithmetic of the graded light scale. Pure: no config, no globals, no locks.

## What it is for

The scale runs -100 to 100 and is perceptual, not linear. Zero is the darkest
naturally occurring light (an unlit cave). Negative is magical darkness.

One constant relates the scale to physical light: the **doubling step**, how
many points twice as much light is worth. It is `LightDoublingStep` in config,
shipped at 8, and it is passed in rather than read here.

## Surface

| Symbol | Purpose |
|---|---|
| `Absent() float64` | A term that is not present at all, distinct from a dark term |
| `Combine(step float64, terms ...float64) float64` | Every present term together |
| `Attenuate(step, light, fraction float64) float64` | A transmission fraction applied to one term |
| `Trim(step, others, max, target float64) float64` | The one solve: the output an adjustable source runs at so the combine of `others` and itself lands on `target` (`trim.go`) |
| `TrimDarkness(step, light, otherDark, max, floor float64) float64` | A darkness's trim (lighting plan 5d, ruling D2): `Trim(step, otherDark, max, light - floor)`, Absent light read as 0, Absent when the budget is 0 or less (`trim.go`) |

## Traps

- **Absent is not zero.** A cave has no sky; a sky contributing zero would make
  the cave brighter, because two terms at zero combine to one step above zero.
  `Attenuate` with fraction 0 returns `Absent()` for this reason.
- **A multiplier is a subtraction here.** Half the light is minus one step.
- `Combine` skips NaN as well as -Inf, so one bad caller cannot poison a room.
- Both functions coerce a non-positive step to 1 rather than dividing by zero.
  `Trim` does the same.
- **`Trim` solves the combine exactly; it is not `target - others`.** It
  returns `Absent()` when the combine already reaches `target` without the
  source, or when the needed term would fall below 0. A NaN `target` or `max`
  returns `Absent()`.
- **Darkness uses the same solve (lighting plan 5d).** Darknesses combine among
  themselves by the halving rule and the result is subtracted from the light,
  so keeping the room at or above `floor` is `Combine(otherDark, d) <= light -
  floor`: `Trim` with that budget as its target. The old linear `Darkens`
  branch and the `Polarity` type are DELETED; one solve serves both.

## Who uses it

`internal/gametime` (sun plus moons), `internal/rooms` (ambient plus lamp plus
carried light, minus carried darkness; `Trim` and `TrimDarkness` from
`light_trim.go`). Plan 4 added weather occlusion; plan 5d added darkness on
the same functions.
```

- [ ] **Step 3: `internal/conditions/context.md`**

(a) Replace `  `validateEffects` refuses it without `light_strength`.` (the `adjustable` flag bullet) with:

```markdown
  `validateEffects` refuses it without `light_strength` or (lighting plan 5d)
  `darkness_strength`.
```

(b) Replace

```markdown
`validateEffects` refuses a literal `light_strength` of 0 or less, `adjustable`
without `light_strength`, and a `stacking` record that is also a light.

Per-record state lives on `Condition`:
```

with

```markdown
`validateEffects` refuses a literal `light_strength` of 0 or less, `adjustable`
without `light_strength`, and a `stacking` record that is also a light.

**Darkness sources (lighting plan 5d, ruling D1).** `effects:
{darkness_strength: N}` (`EffectDarknessStrength`, in `AllEffectKinds` and
`ScaledKinds`) makes a record a darkness: light taken away from its room, a
literal for an item (132 Umbral Dark), `magnitude` for a spell (131 Chrysalis
Pall). It is a light record with darkening polarity: it shares `LightTrim`,
`LightOutput` and `ResetLight`, and `LightMax` / `LightNow` read whichever of
the two kinds the spec declares. It is NOT a light: `IsLightSource()` stays
light-only, so `LightSources`, `EmitsLight`, `hood` and the as-lit end line
exclude it by construction. Ask `ConditionSpec.IsDarknessSource()`, walk
`(*Conditions).DarknessSources()`, or `(*Conditions).LightAndDarknessSources()`
for both kinds in one held order (the trim's walk). `AnyDarknessSource(ids)`
reports whether any condition id names a darkness (the equip line's as-lit
test). `Effect(EffectDarknessStrength)` returns 0. `validateEffects` also
refuses a literal `darkness_strength` of 0 or less, a spec declaring both
kinds, and a `stacking` darkness; `AddConditionMagnitude` resets either kind.

Per-record state lives on `Condition`:
```

(c) In the file table, replace the `scaled_magnitude.go` row's `; `NewCharacterSpellStat`` with `; `SpellScaledTriggers` (lighting plan 5d), the trigger count, where darkness reads its own `LightDarknessSpellDuration*` trio and the other kinds share the light trio; `NewCharacterSpellStat``, and replace the `light.go` row with:

```markdown
| `light.go` | Lighting plan 5a: `LightTrim`, `Condition.LightMax` / `LightNow` / `SetLightOutput` / `ResetLight`, `Conditions.LightSources`; plan 5d: `Conditions.DarknessSources`, `Conditions.LightAndDarknessSources` |
```

- [ ] **Step 4: `internal/rooms/context.md`**

(a) Replace the composition list item 3 and the two paragraphs after it (from `3. **Every light anyone present carries, one term each** (lighting plan 5a).` through `sources trim in held order after a pre-pass sets them all off. It is the ONLY`) with:

```markdown
3. **Every light anyone present carries, one term each** (lighting plan 5a).
   `carriedTerms(exclude)` makes one pass over `r.mobs` and `r.players`,
   reading each bearer's `Conditions.LightAndDarknessSources()` through
   `Condition.LightNow`, so a hooded or trimmed-off source adds nothing and
   two torches are one doubling step brighter than one. Before 5a any light
   lifted the room to a flat `DimBelow`; the `FindHasLight` find flag that
   test used is DELETED.
4. **Every darkness anyone present carries** (lighting plan 5d), from the
   same pass. Darknesses combine among themselves by the same halving rule
   (two of 50 take 58) and the combined darkness is SUBTRACTED from the
   combined light (Absent light reads 0), so a room can read below 0, down
   to the -100 clamp. An unlit room with no darkness is still 0.

The composition is layered so a trim can leave one source out: `composeLight`
calls `composeLightExcluding(cfg, celestial, skyFilter, exclude)`, which calls
`composeWith(cfg, celestial, skyFilter, carried, dark)` (the pure core a test
can feed carried light and darkness terms without users or mobs).

**Trimming (`light_trim.go`, plan 5a; darkness plan 5d).** `(*Room).TrimLightFor(c)`
trims every adjustable, unhooded light AND darkness record `c` holds to `c`'s
own eyes. A light takes the least cut from full strength that keeps the room
under `messaging.LightTrimTarget(c.NightVisionStrength(), cfg.DazzleAbove)`,
solved by `lightscale.Trim` against `LightTerms.Light` with the target raised
by `LightTerms.Dark` (a light in a darkened room may run brighter before it
dazzles, ruling D3). A darkness takes the least cut that keeps the room at or
above `messaging.DarknessTrimTarget(strength, InfraReach(), cfg.BlindBelow)`,
solved by `lightscale.TrimDarkness` against the light and the other darkness
(ruling D2). A fresh cast or equip is full strength until the next move.
Sources trim in held order, whatever their kind, after a pre-pass sets them
all off. It is the ONLY
```

(b) Replace

```markdown
5a) `Raw`, the combined light before rounding and clamping (`Absent` when
nothing lights the room), which a trim solves against.
```

with

```markdown
5a) `Raw`. Since lighting plan 5d `Raw` is the NET light `Level` rounds
(`Light` read as 0 when Absent, minus `Dark`), and three fields carry the
parts: `Light` (the combined light, `Absent` when nothing lights the room;
the light trim solves against it), `Dark` (the combined darkness, `Absent`
when nobody carries one) and `Darkened` (someone carries a darkness, as
`Carried` means someone carries a light).
```

(c) In the file table, replace the `lighting.go` row with

```markdown
| `lighting.go` | `Room.LightLevel()`, `Room.IsLit()`, `Room.LightTerms()` (plan 3d), and the sky/lamp/mutator/carried-light and carried-darkness composition (`composeLight`, `composeLightExcluding`, `composeWith`, `carriedTerms`; darkness plan 5d) |
```

and the `light_trim.go` row with

```markdown
| `light_trim.go` | `Room.TrimLightFor` (lighting plan 5a, darkness 5d): trims an arrival's adjustable lights and darknesses to their eyes; called from `MoveToRoom` and `AddMob` only |
```

- [ ] **Step 5: `internal/messaging/context.md`**

(a) After the `LightTrimTarget` bullet (ending `half, so a room at 74.5 cannot round up onto the edge.`) add:

```markdown
- `DarknessTrimTarget(strength, reach, blindBelow int) float64`
  (`window.go`), lighting plan 5d: the darkest room light an observer can
  still use, the floor an adjustable darkness trims to. `-reach` with
  infravision, else `blindBelow - clampShift(strength)` floored at
  `windowFloor`. Exactly the edge: the edge reads shapes, one point below
  reads nothing.
```

(b) In the `CanSeeSightImpairedOnly` policy list, directly above the bullet `  - `sleep_policy_test.go` pins the contract by absence: a sleeper reads`, add (F17):

```markdown
  - Its non-test readers are the round's combat sight gates in
    `internal/hooks` (`NewRound_DoCombat_resolution.go`,
    `NewRound_DoCombat_unified.go`), which gate
    `Balance.DarknessCombatPenalty`. `internal/behaviortree`'s `mobCanSee`
    stopped reading it in lighting plan 5d (ruling D8): mob decisions accept
    `SightShapes` from `ParticipantSight` directly, and this predicate stays
    `SightFull` only so the combat penalty does not move.
```

(c) In the file table's `window.go` row, replace `constant), plus` with `constant), `DarknessTrimTarget` (lighting plan 5d), plus`.

- [ ] **Step 6: `internal/characters/context.md`, `internal/behaviortree/context.md`**

(a) In `internal/characters/context.md`, after the `EmitsLight() bool` bullet (ending `It replaced the retired `lightsource` flag.`) add:

```markdown
- `DarknessTerms() []float64` (`light.go`, lighting plan 5d): `LightTerms`'
  twin, one term per held darkness record (`Conditions.DarknessSources`). A
  darkness is never a light term, so a darkness bearer does not `EmitsLight`:
  no sneak beacon, no `lit` adjective, no woken sleepers.
```

and in the file index row ending `light.go` (`LightTerms`, `EmitsLight`, plan 5a) |`, replace that ending with `light.go` (`LightTerms`, `EmitsLight`, plan 5a; `DarknessTerms`, plan 5d) |`.

(b) In `internal/behaviortree/context.md`, replace the `players_in_room` row with

```markdown
| `players_in_room` | none | At least one player in the room, and the mob can make out the room (`mobCanSee`, below). |
```

replace the `multiple_enemies` row with

```markdown
| `multiple_enemies` | none | More than one player + charmed mob in room, gated on `mobCanSee` like `players_in_room`. |
```

and insert between that table and `### Combat Assessment`:

```markdown
**The sight gate (`sight.go`).** `mobCanSee(mob, room)` gates target
acquisition (`condPlayersInRoom`), the enemy count (`condMultipleEnemies`) and
party aggro (`engageHostilePlayerInRoom` in `actions_party.go`). Since
lighting plan 5d (ruling D8, owner 2026-09-30) it is true at `SightFull` AND
at `SightShapes` from any cause, natural dim light or infravision, read from
`messaging.ParticipantSight` directly: a mob acts on a figure it can make
out, so every mob in a dim room now engages, and a heat-sensing mob (the
Chrysalis Phantom, the condition-85 mobs) acts in darkness its reach covers.
This overturns slice F's `SightFull`-only ruling. It deliberately does NOT
call `messaging.CanSeeSightImpairedOnly` any more: that stays `SightFull`
only because combat's darkness penalty reads it. A nil mob or room reads true.

```

- [ ] **Step 7: `internal/hooks/context.md`, `internal/usercommands/context.md`, `internal/mobcommands/context.md`**

(a) `internal/hooks/context.md`, section "Vision-scaled spells": replace `(`light_strength`, `nightvision_strength`, `infra_reach`) as `magnitude` (via` with

```markdown
(`light_strength`, `nightvision_strength`, `infra_reach`, and since lighting
plan 5d `darkness_strength`) as `magnitude` (via
```

replace

```markdown
`NightVisionSpell*` for nightvision, `InfraSpell*` for infra reach, all on
`configs.Lighting`), caps an infra-reach result at `Lighting.InfraReachCap`
and a nightvision result at `configs.LightWindowShiftCap`
(`conditions.CapScaledMagnitude`), and computes duration from
the shared `SpellDuration*` trio all three kinds use (triggers floored at 1).
```

with

```markdown
`NightVisionSpell*` for nightvision, `InfraSpell*` for infra reach,
`DarknessSpellStrength*` for darkness, all on `configs.Lighting`), caps an
infra-reach result at `Lighting.InfraReachCap` and a nightvision result at
`configs.LightWindowShiftCap` (`conditions.CapScaledMagnitude`). Its duration
comes from `conditions.SpellScaledTriggers(kind, stat, skill)` (lighting plan
5d): darkness reads its own `DarknessSpellDuration*` trio, the other three
kinds share `SpellDuration*` (triggers floored at 1).
```

and after the paragraph ending `flag was spec-level as well).` add:

```markdown

`sendConditionStartRoomText` (`Condition_ApplyConditions.go`, lighting plan
5d, ruling D6) is its mirror for start lines: a darkness source's start line
("A pall of dark spores gathers around X.") is judged as lit by
`spec.IsDarknessSource()`, because the record is already held when the line
goes out and the observers it has just blinded would otherwise miss it.
Every other start line is judged by the room as it is. A darkness's end
line needs nothing: the room is lighter by then.
```

(b) `internal/usercommands/context.md`, the `**`Equip`**` bullet: after its last line (`pack lands on the floor instead of being lost, matching the plain path.`) add:

```markdown
  The wearable room line ("X puts on their Y.") is judged as lit
  (`room.SendTextVisualAsLit`) when the item's worn conditions include a
  darkness source (`conditions.AnyDarknessSource`, lighting plan 5d, ruling
  D6), so the observers the Umbral Lantern has just blinded still see it go
  on. Two else-less `if`s, not an `if/else`, on purpose: the viewpoint walk in
  `messaging_surface_guard_test.go` splits an `if/else` into separate events
  and would lose the line's observer. `hood` refuses a darkness in the light
  slot with its existing "has no hood." line (`hoodedLight` reads light
  sources only).
```

(c) `internal/mobcommands/context.md`, the Equipment management bullet: replace `  (a cursed item simply stays on, with no line).` with

```markdown
  (a cursed item simply stays on, with no line). `equip` judges its
  wearable room line ("X puts on Y.") as lit when the item's worn conditions
  include a darkness source (`conditions.AnyDarknessSource`, lighting plan
  5d, ruling D6), the sibling of the player's `equip`.
```

- [ ] **Step 8: `internal/lightnotice/context.md`, `internal/events/context.md`, `modules/gmcp/context.md`**

(a) `internal/lightnotice/context.md`: in the surface table, replace the `Cause` row's `CauseEyes` |` ending with `CauseEyes`, `CauseDarkness` (lighting plan 5d) |`. Replace attribution step 3

```markdown
3. Otherwise, the first light term that moved: **carried**, then the
   **room's own lamp**, then **weather** filtering the sky, then the
   **sky** itself.
```

with

```markdown
3. Otherwise, the first light term that moved: **darkness** (lighting plan
   5d: `Darkened` flipped or `Dark` moved, a carried darkness arriving,
   lapsing or changing strength), then **carried**, then the **room's own
   lamp**, then **weather** filtering the sky, then the **sky** itself.
   Darkness comes first because without its own cause a darkness moves no
   light term and fell to `CauseEyes`. Its lines live in
   `narration/light-notices/darkness.yaml`, all six transitions, like every
   cause (`LoadFrom` refuses a missing file).
```

and after the paragraph under "## The seams" that ends `when `decide` says to speak.` add:

```markdown

It also feeds the web client's Game-window border (lighting plan 5d, ruling
D7): when the band it computed differs from the one last handed on (kept in
`sentBands`, apart from `records`, because a record keeps its old band while
its player sleeps), or none was, it queues `events.SightBandChanged`, which
`modules/gmcp` answers with `Char.Sight`. A repeat check in the same band
queues nothing; `Forget` clears it with the record.
```

(b) `internal/events/context.md`: in the "Core Game Events" code block, directly after the `CharacterVitalsChanged` struct, add

```go

// lighting plan 5d: a player's light band changed (lightnotice.Check);
// modules/gmcp answers with Char.Sight
type SightBandChanged struct {
    UserId int
}
```

and after the paragraph that ends `mobs only.` (the `DrainQueuedUserInputsForTest` note) add:

```markdown

`DrainQueuedSightBandChangedForTest(userId)` (lighting plan 5d) drains the
`SightBandChanged` events queued for a player (0 drains every one), the
`DrainQueuedVitalsChangedForTest` shape.
```

(c) `modules/gmcp/context.md`: insert directly above `## Module index`:

```markdown
## Char.Sight: the player's light band (lighting plan 5d, 2026-09-30)

`Char.Sight` is one field, `{"band": "dark" | "shapes" | "faces" |
"dazzled"}` (`GMCPCharModule_Payload_Sight`), built in `GetCharNode` from
`messaging.LightBand` for the player in their current room (`sightBand`; an
unloaded room reads faces, passed as an untyped nil) and carried in the full
`Char` payload. The web client tints the Game window's border by it
(`webclient-pure.html`, `"Char.Sight"` handler; `dashboard.css`
`#panel-feed.sight-*`; `tools/webclient-tests/sight-border.js`).

- **Push trigger.** `sightBandChangedHandler` answers
  `events.SightBandChanged`, which `internal/lightnotice.Check` queues only
  when the band changes: the 3d notice cadence (every command, combat round,
  move, login, and the light commands). Not on `Char.Vitals`, which rides
  every pool change and would recompute the room's light on each.
- **No room contents.** The band is the player's own, and the text already
  tells them how well they see.

```

- [ ] **Step 9: `internal/configs/context.md`**

After the paragraph that ends `narrow `Lighting` struct rather than the 400-field `Balance` copy.` (the 5c `DarkCap` note) add:

```markdown

**Darkness-spell scaling (lighting plan 5d).** Six more knobs in
`validateLighting`, the light trio's idiom on their own names so darkness can
be retuned apart from light (owner ruling D9): the yaml keep the `Light`
family prefix, and `configs.Lighting` drops it (`DarknessSpellStrengthBase`,
`DarknessSpellStrengthStatDivisor`, `DarknessSpellStrengthSkillDivisor`,
`DarknessSpellDurationBase`, `DarknessSpellDurationStatDivisor`,
`DarknessSpellDurationSkillDivisor`). `conditions.SpellScaledMagnitude` reads
the strength trio and `conditions.SpellScaledTriggers` the duration trio for
a `darkness_strength: magnitude` condition (Chrysalis Pall, 131). Zero or
negative reverts to the default. All six ship in `_datafiles/config.yaml` at
glow's values, in the "LIGHT: DARKNESS SPELL (lighting plan 5d)" block after
the 5c block.

| Knob | Type | Default and shipped |
|------|------|---------|
| `LightDarknessSpellStrengthBase` | ConfigFloat | 40 |
| `LightDarknessSpellStrengthStatDivisor` | ConfigFloat | 10 |
| `LightDarknessSpellStrengthSkillDivisor` | ConfigFloat | 2 |
| `LightDarknessSpellDurationBase` | ConfigFloat | 2 |
| `LightDarknessSpellDurationStatDivisor` | ConfigFloat | 50 |
| `LightDarknessSpellDurationSkillDivisor` | ConfigFloat | 20 |
```

- [ ] **Step 10: Audit the docs**

Run: `python tools/context_md_audit.py > "$TMP/5d-audit.txt"; grep -E "^(internal/(lightscale|conditions|rooms|messaging|lightnotice|characters|behaviortree|events|hooks|usercommands|mobcommands)|modules/gmcp) " "$TMP/5d-audit.txt"`
Expected: no output (the grep exits 1; run it standalone). `internal/configs` appears in the full report with `func server_Config` and `func Get`, both pre-existing (F29); compare against a run on `origin/master` if it lists anything else.

Search every line this task added for dashes: `git diff -U0 -- '*.md' | grep "^+" | grep -n "—\|–"`. Expected: no output.

- [ ] **Step 11: Patch notes**

Add at the top of `docs/PATCH_NOTES.md`, directly under `# DOGMud Patch Notes` and its blank line, above the newest entry (`## 2026-10-01: Secondhand shelves` at the time of writing, F37). Player-facing, no numbers, no dashes, 80 columns; the heading takes the date the PR merges (shown here as 2026-10-01):

```markdown
## 2026-10-01: Darkness

- A place can now be darker than any cave. A darkness takes light away,
  and it darkens a place for everyone there, whoever brought it.
- A new spell, Chrysalis Pall, gathers dark spores that drink the light.
  It is found, never taught, once your spellcasting is good enough. Use
  `cancel pall` to end it early.
- A pall, or the rare Umbral Lantern, is darkest when it is new. Each time
  you move, it eases to what your own eyes can still use.
- Lights already in a place do not push back when a darkness arrives. A
  light carried in afterwards still helps.
- Infravision sees through darkness to the warmth of living things.
- The Chitin Throne is black while its master lives, and its master can
  see you there. It sometimes drops the Umbral Lantern.
- Creatures now act on any figure they can make out. In dim light a
  creature no longer needs to see your face to come for you.
- A darkness never gives you away when you sneak.
- In the web client, the Game window's border now shows how well you can
  see. Hover over its title for a word on it.
- `help darkness` explains it all.
```

- [ ] **Step 12: `docs/README.md`**

The plan's row was added with the plan. Append to the `tools/webclient-tests/` row (`:73`), inside its last cell and before the closing ` |`: `, `sight-border.js` (the Game-window border: a `Char.Sight` handler setting a `sight-<band>` class and tooltip on `#panel-feed`, delegated from the generic `Char` handler, with all four tones in `dashboard.css`; lighting plan 5d)`.

- [ ] **Step 13: Full gate**

```bash
gofmt -l internal/ modules/ .
go vet ./...
go build ./...
go test ./... -count=1
golangci-lint run --new-from-merge-base=origin/master
for f in tools/webclient-tests/*.js; do node "$f" > "$TMP/wc.log" 2>&1 || echo "FAIL $f"; done
```

Expected: `gofmt` and `vet` print nothing; every package `ok` (129 on the dry run); lint `0 issues.` (a stale-cache warning naming another worktree's path is not an issue); no `FAIL` line from the Node loop. A root guard keyed by `file|literal` or by line that reports a moved site is re-keyed in this commit after reading it, never deleted; the plan measured only Task 3's and Task 11's. If `lighting_parity_golden_test.go` or `lighting_daycycle_golden_test.go` fails, run `python tools/lighting_golden_diff.py` and explain the move before anything is re-recorded. A failure unrelated to this slice is compared against a detached master worktree (`git worktree add --detach C:/tmp/dogmud-5d-base origin/master`), reported, and not fixed here; remove that worktree afterwards.

- [ ] **Step 14: Race run in Docker**

```bash
docker build --target test -f provisioning/Dockerfile -t dogmud-5d-test .
docker run --rm dogmud-5d-test > "$TMP/5d-race.log" 2>&1
grep -E '^(--- FAIL|FAIL|WARNING: DATA RACE)' "$TMP/5d-race.log"
```

Expected: no `WARNING: DATA RACE`; the only failures are the two tests that shell out to `git`, which has no repository inside the image (`TestNoStringOrDataSaysBuff` in the root package and `TestU10DoneWhen_DeadPathsStayDead` in `internal/combat`), with their `FAIL` package lines. Any other failure is a finding. `lightnotice.Check` writes `sentBands` under the same `mu` as `records`, so a race here is a real one. Remove the image afterwards (`docker rmi dogmud-5d-test`).

- [ ] **Step 15: Boot check on private ports (per `dogmud-shipping`)**

The owner's server uses 8090; this check binds only private ports and stops only the process it started (`timeout` ends its own child). The boot is what catches a content panic (F28 was found here).

```bash
git worktree add --detach C:/tmp/dogmud-5d-boot HEAD
cd C:/tmp/dogmud-5d-boot
printf 'Network.TelnetPort: [33337]\nNetwork.LocalPort: 9997\nNetwork.HttpPort: 8097\nNetwork.HttpsPort: 0\nNetwork.AIPort: 0\n' > boot-overrides.yaml
go build -o boot-check.exe .
CONFIG_PATH=C:/tmp/dogmud-5d-boot/boot-overrides.yaml LOG_NOCOLOR=1 timeout 180 ./boot-check.exe > boot.log 2>&1
echo "exit $?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error|PANIC" boot.log
grep -c "Server Ready" boot.log
grep -c "LightDarknessSpellStrengthBase value=40" boot.log
```

Expected: `exit 124`, then `0`, then `1`, then `1`. The detached worktree boots on this branch's own committed `config.yaml` (it has no skip-worktree bit), which carries the six new keys; the dry run's boot log printed `name=Balance.LightDarknessSpellStrengthBase value=40` and `mapper.ValidateZoneConsi errors=0 warnings=0 mode=panic`. Then `cd` back, `git worktree remove --force C:/tmp/dogmud-5d-boot` (if Windows holds the exe, PowerShell `Remove-Item -Recurse -Force C:\tmp\dogmud-5d-boot`, then `git worktree prune`). Never stop a server this session did not start.

- [ ] **Step 16: Commit**

```bash
git add internal/lightscale/context.md internal/configs/context.md internal/conditions/context.md internal/characters/context.md internal/messaging/context.md internal/rooms/context.md internal/hooks/context.md internal/usercommands/context.md internal/mobcommands/context.md internal/behaviortree/context.md internal/lightnotice/context.md internal/events/context.md modules/gmcp/context.md docs/PATCH_NOTES.md docs/README.md
git commit -m "docs(lighting-5d): context.md, patch notes and README for darkness" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(Add by name any guard file re-keyed in Step 13.)

---

### Task 13: Playtest and PR

**Model:** opus (judgment on live findings; the adversarial content gate).

- [ ] **Step 1: Playtest (the spec's procedure, ending with the adversarial content gate)**

Load `dogmud-playtesting` and `playtest-scenario` and follow them: an EPHEMERAL scenario file written in the scratchpad (not the repo), `--checkout C:/tmp/dogmud-5d`, private ports only, never touch the owner's server (8090) and never kill a process this session did not start; reports are gitignored, so findings go to memory. The harness bans the `admin` profile from `playtestrun scenario`: give the operator admin by flipping `role: admin` in a normal profile's save, and use a distinct profile per actor. `locate` needs the full mob name (`locate Chrysalis Phantom`). Every actor quotes lines verbatim, and every actor reports the Game-window border tone and its tooltip at each step (the web client, or the `Char.Sight` GMCP frames the harness logs).

Roster: an operator (admin), a caster given spellcasting 25 (`skillset`) with the Chrysalis Pall spell (`spell` admin grant, or a discovery roll at 25), a Heat Sight caster, and two plain players.

1. *The lair, no light.* The party walks from room 505 south into 508 (the Chitin Throne). Each reports the light notice, the border (`Too dark to see.`), what `look` shows of the Phantom, and whether the Phantom ambushes (D8: it should). The Heat Sight caster reports shapes there.
2. *The lair, lit.* Re-spawn (`respawn` or wait the 30 minutes), then enter carrying a torch and a Chrysalis Glow. Report the level the notices imply (a torch alone leaves it dark to normal eyes; glow and torch lift it to faces), and the Phantom's behaviour.
3. *The pall in a lit tavern.* The caster casts `chrysalis-pall` on themself in a lamplit room (a Thornwall tavern). The others report the start line (`A pall of dark spores gathers around <name>.`, seen although the room has just gone dark: D6), the darkness notice (never `eyes`), and the border change. The caster walks out and back in and reports the trimmed level (their own notices and border). `cancel pall` restores the room: the others report the end line and the darkness notice.
4. *The drop.* Kill the Phantom with the lantern forced to drop (admin: give the Phantom's lantern to a player with `item` spawn of 20098 if the 25% roll misses; say which way it came). The holder equips it (`equip lantern`) in a lit street: observers report `<name> puts on their Umbral Lantern.` and the notice; `hood` says `Your Umbral Lantern has no hood.`; the holder walks a lit street and then a cave and reports the trim each step.
5. *Help and sneak.* `help darkness`, `help pall`, `help umbral`, `help dark` (still `light`), `help chrysalis-pall`: quote the headings and See also lines. A lantern holder sneaks past a plain player in a dim room and reports nothing marks them as lit.
6. *Pop-out.* In the web client, pop the Game window out and back; the border tone must follow the window.

Then run the adversarial content gate (`dogmud-authoring-content`): a fresh character reads every new line (`help darkness`, `help chrysalis-pall`, the item's `look`, the condition's `conditions` entry, the cast and wait lines) as a confused player would and reports anything unclear, over 80 columns, carrying a number where the copy rules forbid one, or with a dash.

Extract every finding to memory: a `project-lighting-5d-playtest-findings-2026-09-30.md` topic file and one pointer line in `MEMORY.md`, added with the Edit tool (never a Python read-modify-write). Fix what the findings show before the PR; a fix is a normal TDD step with its own commit.

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feature/lighting-5d-darkness
gh pr create --repo pruuk/DOGMud --base master --head feature/lighting-5d-darkness --title "feat(lighting): plan 5d, darkness" --body-file "$TMP/5d-pr-body.md"
```

Body (`$TMP/5d-pr-body.md`): what ships (the rules, one line each); the D8 ruling in bold, with its reach (every mob in every dim room now acts on a figure it can make out; slice F's ruling overturned; the combat darkness penalty untouched); the seven items under "Where the spec could not be implemented as written", leading with F28 (the Umbral Lantern needs `vendor_categories`; it ships `[blacksmithing]`) and the invented `value: 60`, both for the owner; the "Player-visible lines that change" table; the guards and goldens (the `light_spell.go|63` to `|58` re-key, the 131 registration, the three additive goldens, the parity and daycycle goldens unmoved); gate results with counts; the race run; the boot check; the playtest outcome and where its findings live in memory; the spec and plan paths; and a line that CI minutes are exhausted for September, so the local gate, the race run and the boot check are the merge gate. End with:

```
🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

Read back the URL `gh` prints and confirm it says `pruuk/DOGMud`. The owner runs any deploy; do not deploy, and do not nag about deploying. After merge, `git worktree remove C:/tmp/dogmud-5d` and drop its name from the handoff memory.

---

## Self-review

- **Spec coverage.** Owner decision 1 and Rule 2, darkness combined by the halving rule and subtracted, the -100 clamp, an unlit room still 0, every row of the Rule 2 table (T5). Decision 2, Rule 3 and D2, `TrimDarkness` through the one `Trim`, the linear branch and `Polarity` deleted, every row of the Rule 3 table checked by output and by recomputed room (T1) and through a real room (T5); the light row with darkness present (D3, T5); held order across kinds, entry order across three bearers, nobody re-trims, a spawn does not trim (T5, and the real spawn path in T11). D1, the new kind in `AllEffectKinds` and `ScaledKinds`, shared trim state, `IsLightSource` light-only, `IsDarknessSource`, `DarknessSources`, `LightMax` / `LightNow` either kind, both kinds refused, the reset (T3); the "darkness is never light" guard (empty `LightTerms`, no `EmitsLight`, the no-light sneak branch, `hood` refuses), proven able to fail (T6). Decision 3 and Rule 4, the spell at difficulty 25, cost 50, three wait rounds, single, mental, willpower, condition 131, the six knobs with Go defaults and shipped values built from the blob, the three reference casters, full strength on recast, discovery at 24 no and 25 yes (T2, T3, T6, T11); `cancel pall` rides the existing `Cancellable` path (spec P7) and is exercised live in T13. Decision 4 and Rule 5, item 20098 and condition 132 (T11, with F28's deviation). Decision 5, Rule 6 and D9, condition 133 at reach 50 with no nightvision, the lantern at drop chance 25, 508 at -50, the lair test with shapes for the Phantom and dark for a player and a normal-eyed mob (T11). D8, `mobCanSee` on `ParticipantSight` accepting shapes from any cause, dim-room and heat cases, true darkness still blind, the combat predicate pinned `SightFull`, proven able to fail (T7). D4, `CauseDarkness` first among the terms, its six-transition file, the golden additive only, the cause on a cast and on expiry (T8). D5, the `darkness` topic with `umbral` and `pall`, `darkness` off `light`'s aliases, `dark` kept, the spell page, three See also lines (T11). D6, the start line and both equip lines as lit, end lines untouched (T6). D7, `Char.Sight` with one field in `GetCharNode` and the full payload, `SightBandChanged` from `Check` on a change only (T8, T9), the border with four tones and tooltips, docked and popped (T10). Decision 8, free consequences unchanged (the full gate), no sneak beacon (T6). Docs, gate, race run, boot on private ports (T12); playtest with the adversarial gate and PR (T13). Out of scope and untouched: other darkness mobs, potions, scheduled darkness, a `look` adjective, Mudlet, the GMCP room-contents leak, plan 6.
- **Beyond the spec, stated above.** `Conditions.LightAndDarknessSources`, `conditions.SpellScaledTriggers`, `conditions.AnyDarknessSource`, `events.DrainQueuedSightBandChangedForTest`, the `sentBands` map, the `termMoved` rename, `rooms.carriedTerms` in place of `carriedLight`, the mob equip sibling, `tools/webclient-tests/sight-border.js`, the lantern's `vendor_categories` and `value`.
- **Names across tasks.** `Trim(step, others, max, target)`, `TrimDarkness(step, light, otherDark, max, floor)` (T1); `LightDarknessSpellStrengthBase` / `StatDivisor` / `SkillDivisor`, `LightDarknessSpellDurationBase` / `StatDivisor` / `SkillDivisor`, and `Lighting.DarknessSpell*` (T2); `EffectDarknessStrength`, `IsDarknessSource`, `AnyDarknessSource`, `DarknessSources`, `LightAndDarknessSources`, `SpellScaledTriggers` (T3); `DarknessTrimTarget(strength, reach, blindBelow)` (T4); `LightTerms.Light`, `Dark`, `Darkened`, `composeWith(cfg, celestial, skyFilter, carried, dark)`, `carriedTerms` (T5); `DarknessTerms`, `sendConditionStartRoomText` (T6); `mobCanSee` (T7); `SightBandChanged`, `DrainQueuedSightBandChangedForTest`, `CauseDarkness`, `termMoved`, `sentBands` (T8); `GMCPCharModule_Payload_Sight`, `sightBand`, `sightBandChangedHandler` (T9). Test helpers: `seedDarknessSpecs`, `testDarkLanternId`, `testPallId` (conditions); `seedDarknessTrim`, `darkUsers.add`, `sourceNow` and the `dark*` constants (rooms); `seedPallCondition`, `testPallConditionId` (hooks); `darknessFixture`, `darkTest*` (usercommands); `sightHeatConditionId` (behaviortree); `noticePallId` (lightnotice); `sightViewer` (gmcp). None collides with an existing name in its package (each package built and ran).
- **Placeholder scan.** The only angle-bracketed fills are the playtest's live `<name>` / `<mob>` in quoted lines and the PR body's live results, which only the executor can know.
- **Order.** T1 before T5 (the rooms trim calls `TrimDarkness`); T3 before T5, T6, T8 and T11 (the kind and its helpers); T4 before T5 (`DarknessTrimTarget`); T5 before T6 (the start-line test needs a darkened room); T7 before T11 (the lair test asserts D8); T8 before T9 (the event); T9 before T10 (the field); T11 after T8 (its narration guard sees the new condition file only then). Each task's end state passed `go test ./... -count=1` on the dry run.
