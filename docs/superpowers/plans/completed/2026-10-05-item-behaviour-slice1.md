# Item Behaviour Slice 1: Engine and Scheduled Light (Lighting 5e) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Items become the third behaviour-tree subject beside mobs and rooms, with one item tick per round over the items that have a tree, and slice 1 ships scheduled light on it: the shared Oil Lantern dark while its holder sleeps, an arch lantern fixture at Stillwater's North Gate lit from dusk to dawn, a pulsing Rift Stone in the Rift Chamber, and a sunstone that drinks the sun.

**Architecture:** `internal/behaviortree` gains the item subject (`EvalContext.Item *ItemSubject`, `TryItemBehavior`, per-UUID state in `item_state.go`, named trees under `behaviors/items/` compiled under root label `item` and held to an item-safe allowlist, `ValidateItemBehaviors` failing the boot), five item nodes (`holder_asleep`, `worn`, `in_combat`, `set_light`, `pulse_light`) and `time_of_day period: after_dusk` over a new `gametime.GameDate.HoursAfterDusk`. A worn item's light is written through its condition record's existing trimmed-output state (`SetLightOutput`, one mechanism with the trim, owner ruling R3); a fixture's output lives in a new leaf package `internal/itemlight` that `internal/rooms` reads as one term each (`LightTerms.Fixture`, `LightTerms.CarriedLight`). `internal/items` gains `behavior:` and `fixture:` and a holder index of mobs and rooms; `hooks.ItemRoundTick` walks online players plus that index and fires `item_idle`. `lightnotice.attribute` names fixture changes `lamp` and a dimming or second carried light `carried`. Fixtures are refused at every floor-removal path and shown as part of the room's look.

**Tech Stack:** Go 1.25, YAML world data, `go/types` over the compiler's export data for one repo-root guard.

**Spec (binding):** `docs/superpowers/specs/completed/2026-10-05-item-behaviour-foundation-design.md` (merged with this plan from `docs/item-behaviour-spec`), its facts table, "Where the facts push back" X1 to X22 and the owner rulings R1 to R10. Two later owner answers (2026-10-05) also bind: a sleeping player's Oil Lantern relighting by itself on the first round after waking is fine, and the scheduled-off record state surviving a restart until the next round is fine. Slices 2 (voices) and 3 (procs) are out of this plan; this slice builds the engine they plug into.

**Branch:** implementation branch `feature/item-behaviour-slice1`, cut from master AFTER the docs branch `docs/item-behaviour-spec` (this plan and the spec) merges, in the worktree `C:/tmp/dogmud-itembeh1`:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feature/item-behaviour-slice1 C:/tmp/dogmud-itembeh1 origin/master
```

All paths below are relative to that worktree. Run Go commands from its root. Name `C:/tmp/dogmud-itembeh1` in the handoff memory while the branch is open (it must outlive the session); remove it with `git worktree remove C:/tmp/dogmud-itembeh1` once the PR merges. Throwaway output goes in the session scratchpad (`$TMP`), never `C:/tmp`. Edits use the Edit and Write tools only, never a Python read-modify-write.

Every commit names its paths (never `git add -A` or `git add .`) and ends with a blank line and then exactly `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; the `-m "..." -m "Co-Authored-By: ..."` form below produces that.

**Dry run (2026-10-05).** Every code and content block of Tasks 1 to 14 was applied, in order, to a scratch worktree `C:/tmp/dogmud-itembeh-dry` detached at `origin/master` `6ae8bf5f6`, and the worktree was removed afterwards (`git worktree remove`, `git worktree prune`); nothing was committed from it. Each failing-first step was run and failed for the reason its step names; each "proven able to fail" probe was run, seen red, and restored. Then: `gofmt -l internal/ modules/ .` clean; `go vet ./...` clean; `go build ./...`; `go test ./... -count=1` 130 packages `ok`, 0 failing (23 with no test files); `golangci-lint run --new-from-merge-base=origin/master` `0 issues.` (one stale-cache warning naming another worktree's path, not an issue); `python tools/context_md_audit.py` byte-identical in its package list before and after Task 14's docs (13 pre-existing phantom packages, none this plan touches); every added line free of em and en dashes; a boot on private ports (telnet 33357, local 9957, http 8057) reached `Server Ready` with no panic and `mapper.ValidateZoneConsi errors=0 warnings=0 mode=panic`, stopped by PID. The Docker race run and the playtest were not part of the dry run; Tasks 14 and 15 run them.

**Second dry run (2026-10-05, after the owner's guard (a) ruling).** The same blocks, with Task 13's reclassified guard (a), were applied to a fresh scratch worktree `C:/tmp/dogmud-itembeh-dry2` detached at `origin/master` `26dddeb33` (PR #406 merged), then removed and pruned. Task 13's failing-first step and every probe were re-run and seen red as written (including both new guard (a) probes); `gofmt -l` clean, `go vet .` clean, `go build ./...`, `go test -count=1 .` `ok`. The only conflict was `docs/PATCH_NOTES.md`, where #406's `## 2026-10-05: Veyra's secrets` entry now sits on top; Task 14 Step 5 places this slice's entry above it.

---

## Facts verified against source (2026-10-05, `origin/master` `6ae8bf5f6`)

The spec's facts were read at `7e0748145`. `origin/master` is now `6ae8bf5f6` (#379, #380): `git diff --stat 7e0748145 6ae8bf5f6` touches only `docs/PATH_TO_1.0.md` and `internal/gamelock/` (4 files), so every spec row this plan leans on still holds; each was re-read at `6ae8bf5f6` and the line numbers below are master's. **NEW:** PR #406 (Veyra dialogue gold gates) has since merged as `26dddeb33`. `git diff --name-only 6ae8bf5f6 26dddeb33` (81 files) touches no file this plan edits except `docs/PATCH_NOTES.md` (a new top entry, F31) and `docs/README.md` (two new rows elsewhere, no conflict); every line number below holds at `26dddeb33`. Rows marked **NEW** are facts the spec does not state, or states wrongly, and that change the plan.

| # | Fact | Where (master `6ae8bf5f6`) |
|---|---|---|
| F1 | `EvalContext{Event, MobState, MobId, InstanceId, RoomId, MobName, Intercepted, SoftTarget}` ends at `SoftTarget state.ActorRef` | `internal/behaviortree/types.go:43-63` |
| F2 | **NEW.** `ActionNode.Evaluate` copies the context field by field into a closure for a static `delay:` param and for perception-delayed actions (`Intercepted: ctx.Intercepted,` at `:177`, `:198`); an `Item` field not copied there would reach a delayed item action as nil. Task 4 copies it (sibling path) | `internal/behaviortree/actions.go:160-207` |
| F3 | `compileNode(def, path)` threads a path string from the root label; `LoadTreeFromBytes` compiles under `root`, archetypes under `arch`; `compileCondition` / `compileAction` look the name up at `:128` / `:143` | `internal/behaviortree/loader.go:54,67,71,128,143` |
| F4 | Registries: `conditionRegistry` ends with `mob_at_target_room` (`conditions.go:56`), `actionRegistry` with `try_mutation_active_at_target` (`actions.go:124`); `ConditionNode.Evaluate` is `conditions.go:72-74` | files named |
| F5 | `ListTreeFiles` skips only `archetypes` and `rooms` as zones (`save.go:262`); `TreeFileRow.Kind` comment `archetype \| mob \| room` (`:231`). **NEW:** the GMCP editor list (`modules/gmcp/gmcp.Behavior.go:150-163`) switches on kind and ignores an unknown one, so a fourth kind `item` needs no editor change (item tree editing stays #367) | files named |
| F6 | `KnownBehaviorEvents` begins `"heard_callforhelp"` (`events.go:17`); `events_test.go` fails on a fired `EventType: "..."` literal missing from it | `internal/behaviortree/events.go:16-32`; `events_test.go:17-63` |
| F7 | `condTimeOfDay`'s period branch begins at `// Existing binary form` (`conditions_state.go:115`); `getFloatParam(params, key, default)` exists (`params.go:40`); `loggedTimeOfDayMisconfigs` is the log-once map (`:71`) | files named |
| F8 | `GameDate` ends `DayStart int; NightStart int` (`gametime.go:106-107`); `ReCalculate` sets `night` from the unrounded `hourOfDay` and `nightStartHour` and rounds only `NightStart`/`DayStart` (`:305`); test helpers `pinTiming(t, lat)` and `roundFor(doy, hour)` exist (`nightlength_test.go:10,33`) | `internal/gametime/gametime.go:95-108,260-306` |
| F9 | `ItemSpec` ends its data block at `Restricted` (`itemspec.go:380`); `Validate` ends with the `mutation_rarity_floor` check (`:800`); `GetAllItemSpecsMap` returns the cached pointers (`:536`); `Item.GetSpec` returns the override spec whole when one exists (`items.go:329-341`) | `internal/items/itemspec.go`, `items.go` |
| F10 | `Character.MobInstanceId` is set at the top of a mob spawn (`mobs.go:469`), before any item is stored or worn, and the instance is registered at `:784`. `StoreItem` begins at `inventory.go:169` (`i.ClearBaublePlacement()` `:178`) and its capacity refusal comes after; `Wear`'s light reset is `worn.go:592-600`; the two spills are `worn.go:626` and `:666` | `internal/mobs/mobs.go`, `internal/characters/` |
| F11 | **NEW.** The W5 "direct appends" are fewer than the spec's list: `mobs/mobs.go:688` copies the template's slice (no new item), `crafter.go` stores crafted output through `StoreItem` (`:394`), `NewRound_AutoHeal.go:153` and `PlayerSpawn_HandleJoin.go:160` are a player's backpack (players are walked, never indexed) except the companion gear restore (`PlayerSpawn_HandleJoin.go:159-168`), and `steal_pocket.go:214`'s fallback append runs only after `StoreItem` refused, which has already indexed (Task 6 indexes before the capacity check). So the index is entered at `StoreItem`, `Wear`, the two spills, spawn and the companion restore | files named |
| F12 | **NEW.** Rooms load lazily and unload: `LoadRoom` (`save_and_load.go:79`) stores through `addRoomToMemory` (`roommanager.go:659-699`), and `removeRoomFromMemory` (`:588-652`) deletes. `IsRoomLoaded` (`:723`) reads memory without loading. Rooms loaded before item specs exist: `factions.ValidateHoldingCells` (`main.go:1646`) runs before `items.LoadDataFiles()` (`:1651`). So the room index follows load and unload, and boot indexes already-loaded rooms once after items load | files named |
| F13 | `Room.AddItem` (`rooms.go:1338-1348`) and `Prepare`'s spawn append (`:1190-1194`, "just append to avoid a mutex double lock") are the two floor adds; `RemoveItem` is `:1497-1515`. No room mutex guards `Items` | `internal/rooms/rooms.go` |
| F14 | `composeWith(cfg, celestial, skyFilter, carried, dark)` (`lighting.go:118`) has one production caller, `composeLightExcluding` (`:106-109`); `LightTerms` ends `Darkened bool` (`:89`); the darkness combine is `:162` | `internal/rooms/lighting.go` |
| F15 | `TrimLightFor` skips a record whose spec lacks `Adjustable` (`light_trim.go:47`); `SetLightOutput` lands any non-finite output on `LightOff` (`light.go:74-80`); `LightNow` returns false for `LightOff` and caps `LightTrimmed` at full (`:43-69`); condition 125 (Lantern Light, 52) has no flags | `internal/rooms/light_trim.go`; `internal/conditions/light.go`; `conditions/125-lantern_light.yaml` |
| F16 | `attribute` checks `a.Carried != b.Carried` (`tracker.go:160`) then the lamp (`:162`); `termMoved` exists (`:178-185`) | `internal/lightnotice/tracker.go` |
| F17 | `carried.yaml`'s two "is gone" lines are `darker_shapes[0]` and `darker_dark[0]`; `light_notices.golden` lines 18 and 20 carry them; the narration golden updates with `go test ./internal/narration/ -run 'TestSnapshotStores/light_notices' -update` | `light-notices/carried.yaml:9,13`; `internal/narration/snapshot_test.go:116,863` |
| F18 | `TargetAsleep(c)` is `c.HasConditionFlag(conditions.Sleeping)`; condition 15 carries the `sleeping` flag and `actions.Sleep` applies it (`sleep.go:60`) | `internal/actions/sleeping_target.go:24-26`; `conditions/15-sleeping.yaml` |
| F19 | `TakeFloorItem` gates `ErrTooDark`, `ErrExploding`, `ErrHouseholdBauble` (`get.go:42-63`). **NEW:** the player `get` maps any other error to "you're already overloaded!" (`usercommands/get.go:643-647`), so a new refusal needs its own case; bare `get all` calls `Get(item.Name())` per floor item (`:216-231`); `getAllMatchingFromFloor` says `You don't see any "X" to pick up.` when nothing matched (`:66-70`); the mob's `get all` does the same loop (`mobcommands/get.go:25-38`) | files named |
| F20 | **NEW.** `steal <floor item>` reaches the floor only for a household bauble (`householdBaubleNamed`); any other floor name answers "Steal from whom?" (`skill.skullduggery.steal.go:111`); `actions.stealHouseholdBauble` is the only floor removal in `steal.go` (`:917`) | files named |
| F21 | `EquipBestFloorItem` scores every floor item (`mob_equip_best_floor_item.go:45`). **NEW:** the hooks fixture mob 100 (`seedAllRegistries`) has no `Awareness` machine, and the equip path calls `Awareness.TransitionToRevealing` (`remove_equip.go:77`), so a test that lets the mob equip must give it one or it panics | files named |
| F22 | **NEW.** `look` resolves room nouns (`look.go:340`) before floor items (`:488`), so 4111's `lantern` noun would shadow item 55 and must move into the item. The room description renders at `look.go:638`; the ground stacks at `:723-743`; `where := \`on the ground\`` at `:491` | `internal/usercommands/look.go` |
| F23 | **NEW.** Two more "ground lists": GMCP `Room.Info.Contents.Items` lists every floor item (`modules/gmcp/gmcp.Room.go:254`), and tab completion for `get` offers every floor item (`world.go:450`). Ruling R9's sibling paths | files named |
| F24 | **NEW.** usercommands tests read no templates unless a test calls `useDogmudTemplates(t)` (`template_freeze_test.go:40-49`), which points `templates.Process` at the shipped world | file named |
| F25 | **NEW.** YAML 1.1 (`gopkg.in/yaml.v2`) reads an unquoted `off` as the boolean `false`; tree params arrive as `map[string]any`, so `set_light` accepts `false` as off and the shipped trees quote `"off"` | measured |
| F26 | Content: 40038 Oil Lantern (`items/materials-40000/40038-oil_lantern.yaml`, `wornconditionids: [125]`); 4111 has no `spawninfo`, its `nouns:` start at `:30` and the `lantern` noun is `:49-53`; 5000's `spawninfo:` is `:21-23` (Sable, 315); Rane's shop ends with 40053 at `mobs/stillwater/9588-enchanter_rane.yaml:83-86`; the sunstone, condition and fixture ids are free (`python tools/id_inventory.py --type items`: `light 20096-20098 ... 20099`, `other-0 ... gaps 3-9, 42-49, 55-899`; `--alloc conditions 1`: `134-134`; grep `^itemid: (55\|56\|20099)$` and `^conditionid: 134$` find nothing, the same greps for 54 and 133 find `54-sorens_iron_pin.yaml` and `133-phantom_heat_sense.yaml`) | files named |
| F27 | **NEW.** `TestShippedLightItemsMatchTheLadder`'s table ends at `{20097, 54, true}` (`shipped_light_items_test.go:34`); `TestEveryShopkeeperCanTradeAtNightWhileAwake` declares `totalSamples, totalNight` at `:168`, computes `night` at `:210`, and checks `totalNight` at `:246`. Measured with this plan applied: 21 keepers carry 40038, so the lantern tree runs at 1512 samples, 216 of them asleep | files named; measured |
| F28 | **NEW.** Measured: the lighting day-cycle and parity goldens do not move (they load no items and run no `Prepare` or tick), `light_notices.golden` moves by exactly the two R8 lines, and no other narration golden moves (condition 134 carries no start or end text). A root `Prepare(false)` of 4111 and 5000 with no mob templates loaded spawns the two fixtures and no mob | measured |
| F29 | **NEW.** `items.ItemSpec` has 73 exported fields (with this slice's `Behavior` and `Fixture`). Measured with `go/types` over `go list -export` data (Task 13), non-test `internal/hooks` reads the I6 behaviour fields at twelve file sites: `pinnacle_tick.go` (`AmbientPotions`, `HungerDrainPct`, `HungerRounds`, `MutationRarityFloor`, `MutationTickChance`, `MutationTickInterval`, `PreservesContents`, `TauntPull`, `VoiceId`), `MobDeath_ItemProcs.go` (`VoiceId`), `PlayerSpawn_HandleJoin.go` (`PreservesContents`), `item_procs.go` (`Procs`, through `ProcsFor`); it also reads 15 plain data fields. A name-uniqueness scan cannot replace the type check: `VoiceId` and `HungerRounds` are also field names on other types. The precedent for classifying every member exactly once is `TestEveryEffectKindIsClassifiedExactlyOnce` (`internal/conditions/effects_test.go:299`) | measured |
| F30 | `NewRound` listeners register in `hooks.go`; `IdleMobs` is `:66`; `main.go` loads item voices at `:1672` and validates auto-aggro gates at `:1733` | files named |
| F31 | **NEW (after #406).** `docs/PATCH_NOTES.md`'s newest entry is `## 2026-10-05: Veyra's secrets` (`:3`), above `## 2026-10-05: Darkness`; the docs branch's README carries this slice's spec and plan rows together | files named |

## Where the spec could not be implemented as written

1. **The rift stones are one item, the Rift Stone (56), and the fixture line reads "The {Name} is lit."** Rule 10 says "{Name} is lit." and the content row says "Rift stones, item 56 `rift_stones`". With a title-case name and no article the line reads "Rift Stones is lit.", ungrammatical, and "Arch Lantern is lit." reads oddly. The template adds "The" and the item is named singular (`56-rift_stone.yaml`, namesimple `stone`); its description says the stone is set into the wall. Named for the owner in the PR.
2. **Guard (a) classifies every ItemSpec field once (owner ruling, 2026-10-05; F29).** "The ItemSpec fields read in non-test `internal/hooks` are pinned to I6's list" cannot hold literally: hooks also read plain data. Following `TestEveryEffectKindIsClassifiedExactlyOnce`, the guard classifies all 73 exported fields exactly once, as BEHAVIOUR (19, each with a reason: the Pinnacle block, `Behavior`, `Fixture`, the four `OnUse*` effects) or DATA (54); a new unclassified field fails. Hooks read data freely; a hook reading a behaviour field (directly or through `ProcsFor`) outside twelve allowlisted `file|field` sites fails, found by type-checking `internal/hooks` (`go/types` over `go list -export`). There are no read counts: the site allowlist does the job, and an allowlisted site nothing reads any more fails, so slices 2 and 3 can only shrink it.
3. **The holder index is entered at fewer sites than W5 lists (F11).** `StoreItem` indexes before its capacity check, which covers `intoPocket`'s fallback; crafted output goes through `StoreItem`; the player appends need no index. Rooms are indexed at `AddItem`, `Prepare`'s append, `addRoomToMemory` (instance load), and once at boot by `rooms.IndexTreedFloors()` for rooms loaded before item specs (F12); they leave at `removeRoomFromMemory`.
4. **"A fixture is evaluated once when its room is indexed" is a callback.** `rooms` cannot reach `behaviortree`, so `items.OnRoomHolderIndexed` (set by `hooks.RegisterListeners` to `hooks.EvaluateRoomFixtures`) runs the room's floor trees the moment the room enters the index.
5. **Eviction reads "a full round with no visit" as "not visited in this round's tick".** `EnsureItemBTreeState` stamps the visit round; `EvictUnseenItemBTreeStates(round)` at the end of the tick drops entries stamped earlier. An item handed between holders between ticks is visited in the next tick and keeps its state.
6. **`set_light full` fails on a fixture.** A fixture has no condition and so no full strength; its tree names a number. Rule 9 defines `full` for records only.
7. **`set_light` also skips adjustable records at run time.** Rule 8's boot check refuses such a tree; the run-time skip keeps one writer per record even for a tree loaded by a test.
8. **The steal refusal lives in the player command (F20).** `steal`'s floor branch only ever takes a household bauble, and no mob steals off a floor, so `parseStealArgs` answers "The {item} is fixed in place." for a fixture before it would say "Steal from whom?".
9. **Siblings of R9 (F23).** GMCP `Room.Info.Contents.Items` and `get` tab completion leave fixtures out; `look <fixture>` says "here" instead of "on the ground"; `get all <name>` that only matches a fixture says it is fixed in place instead of "You don't see any".
10. **Guard (e) is folded into (b).** `TestEveryShippedItemBehaviorResolves` runs `ValidateItemBehaviors` (which includes the one-writer check) over the shipped world and repeats the adjustable check explicitly.
11. **`internal/conditions` gets no `context.md` change.** The spec lists it among the packages a slice touches; this slice reads `SetLightOutput`, `LightOff` and `LightFull` as they are and changes no conditions code (owner ruling R3: no new field, no new state).
12. **No help text.** The spec names none for slice 1; the patch notes carry the player-facing summary.

No ruling changes. R2 (`holder_asleep` reads the Sleeping flag through `actions.TargetAsleep`), R3 (the tree on the shared 40038, fully dark asleep, one mechanism: `SetLightOutput` on the existing record, fixtures in `internal/itemlight`), R4 (4111 only, 5803 untouched), R7 (`time_of_day` with `period: after_dusk` + `hours`), R8 ("fades") and R9 (fixtures out of "On the Ground", shown as part of the room) are implemented as ruled. R10's numbers ship as ruled: stones 20 to 36 (room 40 to 45), fixture ids 55 and 56, sunstone 20099 with condition 134 at 46, faint 30 for an hour after dusk, value 40, sold by Enchanter Rane.

## Player-visible lines that change (everything else stays byte-identical)

| Line | Before | After |
|---|---|---|
| A `lamp` notice in 4111 at dusk or dawn on a moonless night | Unreachable | e.g. `A lamp flickers to life; shapes come out of the dark.` / `The lamps burn brighter; faces are clear again.` from `lamp.yaml` |
| `carried` darker lines (R8) | `The carried light is gone, and faces blur into shapes.` / `The carried light is gone, and darkness closes in.` | `The carried light fades, and faces blur into shapes.` / `The carried light fades, and darkness closes in.` |
| A keeper's or a player's Oil Lantern as its holder falls asleep, a sunstone fading | No light change (or `eyes`) | The lantern goes dark (a `carried` notice to anyone whose band moves); a dimming or second carried light now reads `carried`, not `eyes` |
| Room look of 4111 and 5000 | No fixture line | `The Arch Lantern is lit.` / `The Arch Lantern is unlit.` / `The Rift Stone is lit.` right after the description, coloured like it |
| "On the Ground", GMCP room items, `get` tab completion | Every floor item | Fixtures left out |
| `look lantern` in 4111 | The `lantern` noun | `You look at the Arch Lantern here:` and item 55's description |
| `get lantern`, `steal lantern`, `get all lantern` in 4111 | (no such item) | `The Arch Lantern is fixed in place.`; bare `get all` passes it without a word |
| `list` at Enchanter Rane (Stillwater) | No sunstone | The Sunstone for sale; its description, condition 134 "Sunstone Glow" (secret) |
| `docs/PATCH_NOTES.md` | | `## 2026-10-05: Lights that keep time` (Task 14) |

Every new line is under 80 columns, carries no raw number and no em or en dash.

## File map

| File | Change |
|---|---|
| `internal/gametime/gametime.go`, `after_dusk_test.go` | `GameDate.HoursAfterDusk` (Task 1) |
| `internal/behaviortree/conditions_state.go`, `conditions_after_dusk_test.go` | `time_of_day period: after_dusk` (Task 1) |
| `internal/itemlight/itemlight.go`, `itemlight_test.go`, `context.md` | Create: fixture outputs per room (Task 2) |
| `internal/items/itemspec.go`, `item_behavior.go`, `item_behavior_test.go` | `Behavior`, `Fixture`, `HasBehavior`, `IsFixture`, the holder index (Task 3) |
| `internal/behaviortree/types.go`, `conditions.go`, `actions.go`, `engine.go`, `loader.go`, `save.go`, `test_export.go`, `item_engine.go`, `item_state.go`, `item_engine_test.go`; `main.go` | The item subject, state, loader, allowlist, `TryItemBehavior`, `ValidateItemBehaviors`, `ListTreeFiles` kind `item` (Task 4) |
| `internal/behaviortree/conditions_item.go`, `actions_item_light.go`, `item_light_test.go`; `conditions.go`, `actions.go`, `loader.go`, `item_engine.go` | Item nodes, item-only refusal, the one-writer check (Task 5) |
| `internal/characters/item_behaviour.go`, `inventory.go`, `worn.go`, `item_behaviour_test.go`; `internal/mobs/mobs.go`, `item_behaviour_index_test.go`; `internal/hooks/PlayerSpawn_HandleJoin.go`; `internal/rooms/item_behaviour.go`, `rooms.go`, `roommanager.go`, `item_behaviour_test.go`; `main.go` | The holder index wired (Task 6) |
| `internal/hooks/NewRound_ItemRoundTick.go`, `NewRound_ItemRoundTick_test.go`, `hooks.go`; `internal/behaviortree/events.go` | The item tick, `item_idle` (Task 7) |
| `internal/rooms/lighting.go`, `fixture_compose_test.go` | Fixture terms, `LightTerms.Fixture`, `CarriedLight` (Task 8) |
| `internal/lightnotice/tracker.go`, `fixture_attribution_test.go`; `light-notices/carried.yaml`, `lamp.yaml`; `internal/narration/testdata/stores/light_notices.golden` | Attribution, R8 (Task 9) |
| `internal/actions/get.go`, `get_fixture_test.go`; `internal/usercommands/get.go`, `skill.skullduggery.steal.go`, `get_fixture_test.go`; `internal/hooks/mob_equip_best_floor_item.go`, `mob_equip_floor_fixture_test.go`; `internal/mobcommands/get.go` | Fixtures untakeable (Task 10) |
| `internal/usercommands/look.go`, `look_fixture_test.go`; `templates/descriptions/fixtures.template`; `modules/gmcp/gmcp.Room.go`, `gmcp.Room_fixture_test.go`; `world.go` | Fixtures in the look (Task 11) |
| `behaviors/items/dusk_to_dawn.yaml`, `rift_pulse.yaml`, `keeper_lantern.yaml`, `sunstone.yaml`; `items/other-0/55-arch_lantern.yaml`, `56-rift_stone.yaml`; `items/armor-20000/light/20099-sunstone.yaml`; `conditions/134-sunstone_glow.yaml`; `40038-oil_lantern.yaml`, `rooms/stillwater/4111.yaml`, `rooms/thornwall_city/5000.yaml`, `mobs/stillwater/9588-enchanter_rane.yaml`; `internal/items/shipped_light_items_test.go`; `internal/behaviortree/shipped_item_trees_test.go` | Content (Task 12) |
| `item_behaviour_guard_test.go`, `lighting_fixture_daycycle_golden_test.go`, `testdata/lighting_fixture_daycycle.golden`, `shop_night_trade_guard_test.go` (repo root) | Guards and the fixture day-cycle record (Task 13) |
| `context.md` in `internal/behaviortree`, `items`, `rooms`, `lightnotice`, `gametime`, `hooks`, `characters`, `actions`, `usercommands`, `mobcommands`, `mobs`, `modules/gmcp`; `docs/schemas/behavior.md`; `docs/PATCH_NOTES.md` | Docs (Task 14) |

### Repo-root guards and goldens this plan touches

| Guard or golden | Keyed by | What moves | Where handled |
|---|---|---|---|
| `internal/behaviortree/events_test.go` both-ways vocabulary | `EventType: "..."` literals | `item_idle` added to `KnownBehaviorEvents` with its dispatch site | Task 7 |
| `internal/behaviortree/roundtrip_test.go`, `events_test.go` live-YAML walks | every file under `behaviors/` | Four new item trees; they round-trip and use only `item_idle` | Task 12, verified |
| `internal/narration/testdata/stores/light_notices.golden` | snapshot | Exactly the two R8 lines (F17) | Task 9 |
| `lighting_daycycle_golden_test.go`, `lighting_parity_golden_test.go` | snapshot | Must NOT move (F28); if either moves, explain it with `tools/lighting_golden_diff.py` before any re-record | Tasks 8, 12, 14 |
| `testdata/lighting_fixture_daycycle.golden` | snapshot | New: 4111 and 5000 at the twelve samples (X8) | Task 13 |
| `shop_night_trade_guard_test.go` | content walk | Runs each keeper's light-slot tree at all 72 samples with the Sleeping flag set as the schedule would; asleep lanterns must be dark | Task 13 |
| `item_behaviour_guard_test.go` (new) | every exported `ItemSpec` field classified once (behaviour or data); type-checked behaviour reads in `internal/hooks` against a `file\|field` allowlist; shipped trees, items and spawninfo | Rule 15 (a) to (e) | Task 13 |

No guard is weakened and no existing key is re-keyed.

---

### Task 1: `time_of_day period: after_dusk` over `GameDate.HoursAfterDusk` (X2, R7)

**Model:** haiku (mechanical, code given).

**Files:**
- Create: `internal/gametime/after_dusk_test.go`
- Modify: `internal/gametime/gametime.go:106-108` (struct tail), `:305` (`ReCalculate`)
- Create: `internal/behaviortree/conditions_after_dusk_test.go`
- Modify: `internal/behaviortree/conditions_state.go:115-117` (the period branch)

- [ ] **Step 1: Write the failing gametime test**

Create `internal/gametime/after_dusk_test.go`:

```go
package gametime

import (
	"math"
	"testing"
)

// duskRound scans one day for the first round at or after noon that reads
// Night: the round dusk falls in.
func duskRound(t *testing.T, dayOfYear int) uint64 {
	t.Helper()
	for r := roundFor(dayOfYear, 12); r < roundFor(dayOfYear+1, 0); r++ {
		if GetDate(r).Night {
			return r
		}
	}
	t.Fatalf("day %d has no dusk after noon", dayOfYear)
	return 0
}

// HoursAfterDusk reads the same unrounded boundary Night does, so it is
// near 0 in the round dusk falls in and grows one game hour per 37.5 rounds.
func TestHoursAfterDuskStartsAtTheNightBoundary(t *testing.T) {
	pinTiming(t, 46.5)
	for _, doy := range []int{356, 81, 172} {
		dusk := duskRound(t, doy)
		if h := GetDate(dusk).HoursAfterDusk(); h < 0 || h >= 1.0/37.5 {
			t.Errorf("day %d: at the dusk round HoursAfterDusk = %v, want within one round of 0", doy, h)
		}
		if h := GetDate(dusk + 75).HoursAfterDusk(); math.Abs(h-2) > 1.0/37.5 {
			t.Errorf("day %d: 75 rounds after dusk HoursAfterDusk = %v, want about 2", doy, h)
		}
		// The round before dusk is day, and reads nearly a whole day since
		// the boundary: never "just after dusk".
		if gd := GetDate(dusk - 1); gd.Night || gd.HoursAfterDusk() < 23 {
			t.Errorf("day %d: the round before dusk reads Night=%v HoursAfterDusk=%v, want day and over 23",
				doy, gd.Night, gd.HoursAfterDusk())
		}
	}
}

// Past midnight it keeps counting from the evening's dusk.
func TestHoursAfterDuskWrapsPastMidnight(t *testing.T) {
	pinTiming(t, 46.5)
	dusk := duskRound(t, 356)
	want := 24 - (float64(dusk%900) / 37.5) // hours from dusk to midnight
	if h := GetDate(roundFor(357, 0)).HoursAfterDusk(); math.Abs(h-want) > 1.0/37.5 {
		t.Errorf("midnight after midwinter dusk: HoursAfterDusk = %v, want about %v", h, want)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/gametime/ -run HoursAfterDusk -count=1`
Expected: FAIL to build: `gd.HoursAfterDusk undefined (type GameDate has no field or method HoursAfterDusk)`.

- [ ] **Step 3: Implement `HoursAfterDusk`**

In `internal/gametime/gametime.go`, replace

```go
	DayStart   int
	NightStart int
}
```

with

```go
	DayStart   int
	NightStart int

	// hourOfDay and duskHour are the unrounded clock and night boundary
	// Night is computed from, kept for HoursAfterDusk (lighting 5e). Not
	// exported, so never serialised.
	hourOfDay float64
	duskHour  float64
}

// HoursAfterDusk is how many game hours have passed since dusk, read from
// the same unrounded boundary Night uses (the world's latitude and the day of
// the year), in [0, 24). Past midnight it keeps counting from the evening's
// dusk; before dusk it reads the hours since dusk a day earlier, which is
// more than a day's daylight, so it never reads "just after dusk" in the
// afternoon. The behaviour-tree condition time_of_day reads it for
// `period: after_dusk`.
func (g GameDate) HoursAfterDusk() float64 {
	h := g.hourOfDay - g.duskHour
	if h < 0 {
		h += 24
	}
	return h
}
```

and in `ReCalculate`, replace

```go
	g.DayStart = int(math.Round(nightEndHour))
```

with

```go
	g.DayStart = int(math.Round(nightEndHour))
	g.hourOfDay, g.duskHour = hourOfDay, nightStartHour
```

- [ ] **Step 4: Run it to see it pass**

Run: `go test ./internal/gametime/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/gametime`.

- [ ] **Step 5: Write the failing condition test**

Create `internal/behaviortree/conditions_after_dusk_test.go`:

```go
package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// pinAfterDuskClock pins the shipped day (900 rounds, latitude 46.5) and
// returns the round midwinter's dusk falls in (lighting 5e, X2).
func pinAfterDuskClock(t *testing.T) uint64 {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.NightHours = 8
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	c.Balance.WorldLatitude = 46.5
	c.Balance.Validate()
	configs.SetConfigForTest(t, c)
	gametime.ClearDateCacheForTest()
	prev := util.GetRoundCount()
	t.Cleanup(func() {
		util.SetRoundCountForTest(prev)
		gametime.ClearDateCacheForTest()
	})
	for r := uint64(355*900 + 450); r < 356*900; r++ {
		if gametime.GetDate(r).Night {
			return r
		}
	}
	t.Fatal("midwinter has no dusk after noon")
	return 0
}

// period: after_dusk with hours: N is true from the night boundary until N
// game hours later, then false; it is false all afternoon.
func TestCondTimeOfDay_AfterDusk(t *testing.T) {
	dusk := pinAfterDuskClock(t)
	params := map[string]any{"period": "after_dusk", "hours": 1}
	for _, c := range []struct {
		name  string
		round uint64
		want  Result
	}{
		{"the dusk round", dusk, Success},
		{"half an hour after dusk", dusk + 19, Success},
		{"just over an hour after dusk", dusk + 38, Failure},
		{"the round before dusk", dusk - 1, Failure},
		{"noon", 355*900 + 450, Failure},
		{"midnight", 356 * 900, Failure},
	} {
		util.SetRoundCountForTest(c.round)
		if got := condTimeOfDay(params, nil); got != c.want {
			t.Errorf("%s (round %d): got %v, want %v", c.name, c.round, got, c.want)
		}
	}
}

// A missing or non-positive hours never matches.
func TestCondTimeOfDay_AfterDuskNeedsHours(t *testing.T) {
	dusk := pinAfterDuskClock(t)
	util.SetRoundCountForTest(dusk)
	for _, p := range []map[string]any{
		{"period": "after_dusk"},
		{"period": "after_dusk", "hours": 0},
		{"period": "after_dusk", "hours": -2},
	} {
		if got := condTimeOfDay(p, nil); got != Failure {
			t.Errorf("%v at dusk: got %v, want Failure", p, got)
		}
	}
}
```

- [ ] **Step 6: Run it to see it fail**

Run: `go test ./internal/behaviortree/ -run AfterDusk -count=1`
Expected: FAIL: `the dusk round (round 320108): got 1, want 0` and `half an hour after dusk (round 320127): got 1, want 0` (`Failure` is 1: no `after_dusk` branch yet).

- [ ] **Step 7: Implement `after_dusk`**

In `internal/behaviortree/conditions_state.go`, replace

```go
	// Existing binary form (period: day / period: night) — unchanged.
	period := getStringParam(params, "period")
	isNight := gametime.IsNight()
	switch strings.ToLower(period) {
	case "night":
```

with

```go
	// Existing binary form (period: day / period: night) — unchanged.
	period := getStringParam(params, "period")

	// period: after_dusk with hours: N (lighting 5e, X2): true from the
	// night boundary until N game hours later. Dusk moves with the season,
	// so this cannot be a fixed `range`; it reads the same unrounded
	// boundary IsNight does.
	if strings.ToLower(period) == "after_dusk" {
		hours := getFloatParam(params, "hours", 0)
		if !(hours > 0) {
			if _, already := loggedTimeOfDayMisconfigs.LoadOrStore("after_dusk:hours", true); !already {
				mudlog.Error("time_of_day",
					"error", "period: after_dusk needs a positive `hours`",
					"value", params["hours"])
			}
			return Failure
		}
		if gametime.GetDate().HoursAfterDusk() < hours {
			return Success
		}
		return Failure
	}

	isNight := gametime.IsNight()
	switch strings.ToLower(period) {
	case "night":
```

- [ ] **Step 8: Run it to see it pass**

Run: `go test ./internal/behaviortree/ -run 'TimeOfDay' -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/behaviortree`.

- [ ] **Step 9: Commit**

```bash
git add internal/gametime/gametime.go internal/gametime/after_dusk_test.go internal/behaviortree/conditions_state.go internal/behaviortree/conditions_after_dusk_test.go
git commit -m "feat(behaviortree): time_of_day after_dusk over GameDate.HoursAfterDusk (5e X2)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: `internal/itemlight`, fixture outputs per room (X6)

**Model:** haiku (mechanical, code given).

**Files:**
- Create: `internal/itemlight/itemlight_test.go`, `internal/itemlight/itemlight.go`, `internal/itemlight/context.md`

- [ ] **Step 1: Write the failing test**

Create `internal/itemlight/itemlight_test.go`:

```go
package itemlight

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/uuid"
)

func id(b byte) uuid.UUID { return uuid.UUID{b} }

func TestSetRecordsAndReportsChange(t *testing.T) {
	t.Cleanup(ResetForTest())
	if !Set(7, id(1), Light, 52) {
		t.Fatal("first Set reported no change")
	}
	if Set(7, id(1), Light, 52) {
		t.Error("the same output again reported a change: the tick would rewrite every round")
	}
	if !Set(7, id(1), Light, 30) {
		t.Error("a new value reported no change")
	}
	if v, ok := Get(7, id(1)); !ok || v != 30 {
		t.Errorf("Get = %v, %v; want 30, true", v, ok)
	}
}

func TestUnlitIsKnownButAddsNoTerm(t *testing.T) {
	t.Cleanup(ResetForTest())
	Set(7, id(1), Light, math.Inf(-1))
	Set(7, id(2), Light, -3) // negative reads as unlit
	if v, ok := Get(7, id(1)); !ok || !math.IsInf(v, -1) {
		t.Errorf("unlit Get = %v, %v; want -Inf, true", v, ok)
	}
	if Lit(7, id(1)) || Lit(7, id(2)) {
		t.Error("an unlit fixture reads lit")
	}
	if light, dark := Terms(7); len(light) != 0 || len(dark) != 0 {
		t.Errorf("Terms = %v, %v; want none", light, dark)
	}
}

func TestTermsSplitByKindInUUIDOrder(t *testing.T) {
	t.Cleanup(ResetForTest())
	Set(7, id(3), Light, 20)
	Set(7, id(1), Light, 52)
	Set(7, id(2), Darkness, 40)
	Set(8, id(4), Light, 99) // another room
	light, dark := Terms(7)
	if len(light) != 2 || light[0] != 52 || light[1] != 20 {
		t.Errorf("light = %v, want [52 20] (UUID order)", light)
	}
	if len(dark) != 1 || dark[0] != 40 {
		t.Errorf("dark = %v, want [40]", dark)
	}
}

func TestClearRetainAndClearRoom(t *testing.T) {
	t.Cleanup(ResetForTest())
	Set(7, id(1), Light, 52)
	Set(7, id(2), Light, 30)
	Set(7, id(3), Light, 20)
	Clear(7, id(1))
	if _, ok := Get(7, id(1)); ok {
		t.Error("Clear left the output")
	}
	Retain(7, map[uuid.UUID]bool{id(2): true})
	if _, ok := Get(7, id(3)); ok {
		t.Error("Retain kept an output not in keep")
	}
	if _, ok := Get(7, id(2)); !ok {
		t.Error("Retain dropped an output in keep")
	}
	ClearRoom(7)
	if light, _ := Terms(7); len(light) != 0 {
		t.Errorf("ClearRoom left %v", light)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/itemlight/ -count=1`
Expected: FAIL to build: `undefined: Set`.

- [ ] **Step 3: Implement the package**

Create `internal/itemlight/itemlight.go`:

```go
// Package itemlight holds the light and darkness that fixtures give their
// rooms (lighting 5e, item behaviour slice 1, spec X6).
//
// A fixture is an item fixed to a room's floor (ItemSpec `fixture: light` or
// `fixture: darkness`). Its behaviour tree writes its output here through
// set_light and pulse_light, and internal/rooms reads every lit fixture of a
// room as one term each when it composes the room's light. The package is a
// leaf so that both can reach it: behaviortree imports rooms, so rooms cannot
// read tree state, and this sits below both.
//
// Outputs are in memory only, keyed by room and item UUID. An item's UUID is
// minted afresh on every load, so a restart or a room reload starts empty and
// the item tick writes the outputs again (the room's first visit evaluates
// its fixtures at once, internal/hooks).
package itemlight

import (
	"bytes"
	"math"
	"sort"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// Kind is which combine a fixture's output joins.
type Kind uint8

const (
	// Light joins the room's light combine.
	Light Kind = iota
	// Darkness joins the room's darkness combine.
	Darkness
)

type output struct {
	kind  Kind
	value float64 // math.Inf(-1) when unlit
}

var (
	mu    sync.RWMutex
	rooms = map[int]map[uuid.UUID]output{}
)

// Set records a fixture's output in its room and reports whether anything
// changed. A non-finite or negative value records the fixture as unlit
// (it stays known, so a look can say "unlit").
func Set(roomId int, id uuid.UUID, kind Kind, value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		value = math.Inf(-1)
	}
	mu.Lock()
	defer mu.Unlock()
	byId, ok := rooms[roomId]
	if !ok {
		byId = map[uuid.UUID]output{}
		rooms[roomId] = byId
	}
	next := output{kind: kind, value: value}
	if prev, ok := byId[id]; ok && prev == next {
		return false
	}
	byId[id] = next
	return true
}

// Get returns a fixture's recorded output and whether one is recorded. An
// unlit fixture returns math.Inf(-1) and true.
func Get(roomId int, id uuid.UUID) (float64, bool) {
	mu.RLock()
	defer mu.RUnlock()
	o, ok := rooms[roomId][id]
	return o.value, ok
}

// Lit reports whether a fixture is recorded and gives light (or darkness).
func Lit(roomId int, id uuid.UUID) bool {
	v, ok := Get(roomId, id)
	return ok && !math.IsInf(v, -1)
}

// Clear drops one fixture's output: it left the floor.
func Clear(roomId int, id uuid.UUID) {
	mu.Lock()
	defer mu.Unlock()
	if byId, ok := rooms[roomId]; ok {
		delete(byId, id)
		if len(byId) == 0 {
			delete(rooms, roomId)
		}
	}
}

// ClearRoom drops every output in a room: it left memory.
func ClearRoom(roomId int) {
	mu.Lock()
	defer mu.Unlock()
	delete(rooms, roomId)
}

// Retain drops every output in a room whose UUID is not in keep, so an item
// that left the floor by a path that never called Clear stops lighting it.
func Retain(roomId int, keep map[uuid.UUID]bool) {
	mu.Lock()
	defer mu.Unlock()
	byId, ok := rooms[roomId]
	if !ok {
		return
	}
	for id := range byId {
		if !keep[id] {
			delete(byId, id)
		}
	}
	if len(byId) == 0 {
		delete(rooms, roomId)
	}
}

// Terms returns a room's lit light outputs and lit darkness outputs, each in
// a stable order (by UUID), ready for the room's two combines. Unlit
// fixtures add no term. O(fixtures in the room).
func Terms(roomId int) (light, dark []float64) {
	mu.RLock()
	defer mu.RUnlock()
	byId := rooms[roomId]
	if len(byId) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(byId))
	for id := range byId {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	for _, id := range ids {
		o := byId[id]
		if math.IsInf(o.value, -1) {
			continue
		}
		if o.kind == Darkness {
			dark = append(dark, o.value)
		} else {
			light = append(light, o.value)
		}
	}
	return light, dark
}

// ResetForTest empties every room and returns a restore func.
func ResetForTest() func() {
	mu.Lock()
	orig := rooms
	rooms = map[int]map[uuid.UUID]output{}
	mu.Unlock()
	return func() {
		mu.Lock()
		rooms = orig
		mu.Unlock()
	}
}
```

- [ ] **Step 4: Run it to see it pass**

Run: `go test ./internal/itemlight/ -count=1 && go vet ./internal/itemlight/`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/itemlight`, vet silent.

- [ ] **Step 5: The package's `context.md` (house convention: a new package ships one)**

Create `internal/itemlight/context.md`:

```markdown
# internal/itemlight

The light and darkness that **fixtures** give their rooms (lighting 5e, item
behaviour slice 1, spec X6 and Rule 10). A leaf: it imports only `uuid`.

## What it is for

A fixture is an item fixed to a room's floor (`ItemSpec.Fixture`, `light` or
`darkness`): the North Gate's arch lantern (item 55, room 4111), the Rift
Stone (item 56, room 5000). Its behaviour tree writes its output here through
`set_light` and `pulse_light` (`internal/behaviortree/actions_item_light.go`),
and `internal/rooms` reads every lit fixture of a room as one term each when it
composes the room's light (`composeLightExcluding` in `lighting.go`).

It exists because `behaviortree` imports `rooms`, so `rooms` cannot read tree
state. This package sits below both. A worn light never comes here: its light
lives on its holder's condition record.

## Surface

| Symbol | Purpose |
|---|---|
| `type Kind`, `Light`, `Darkness` | Which combine an output joins |
| `Set(roomId int, id uuid.UUID, kind Kind, value float64) bool` | Record a fixture's output; reports a change. Non-finite or negative records it unlit |
| `Get(roomId int, id uuid.UUID) (float64, bool)` | A fixture's output (`-Inf` when unlit) and whether one is recorded |
| `Lit(roomId int, id uuid.UUID) bool` | Recorded and giving light (the room look's "is lit") |
| `Clear(roomId int, id uuid.UUID)` | Drop one output: the item left the floor (`Room.RemoveItem`) |
| `ClearRoom(roomId int)` | Drop a room's outputs: it left memory (`removeRoomFromMemory`, the item tick) |
| `Retain(roomId int, keep map[uuid.UUID]bool)` | Drop outputs whose item is no longer on the floor (the item tick, every round) |
| `Terms(roomId int) (light, dark []float64)` | A room's lit outputs, UUID order, for the two combines |
| `ResetForTest() func()` | Empty every room; returns a restore |

## Traps

- **In memory only, keyed by item UUID.** A UUID is minted on every load, so a
  restart or a room reload starts empty. The room's first visit evaluates its
  fixtures at once (`items.OnRoomHolderIndexed`, set by `internal/hooks`), so
  nobody reads a fixture room dark for a round.
- **Unlit is recorded, not absent.** `Set` with `-Inf` keeps the fixture known
  so the look can say "unlit"; `Terms` skips it.
- **Fixtures never trim** and are never reset by a bearer (owner ruling R3).
  A carried adjustable light trims against them like any other term.

## Who uses it

`internal/behaviortree` writes; `internal/rooms` reads and clears;
`internal/hooks` (the item tick) retains and clears; `internal/usercommands`
(`look`) reads `Lit`.
```

(Later tasks create every caller this file names; it describes the finished slice.)

- [ ] **Step 6: Commit**

```bash
git add internal/itemlight/itemlight.go internal/itemlight/itemlight_test.go internal/itemlight/context.md
git commit -m "feat(itemlight): fixture light outputs per room, a leaf package (5e X6)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: `behavior:`, `fixture:` and the holder index in `internal/items` (Rules 1, 5, 10)

**Model:** haiku (mechanical, code given).

**Files:**
- Create: `internal/items/item_behavior_test.go`, `internal/items/item_behavior.go`
- Modify: `internal/items/itemspec.go:380` (after `Restricted`), `:799-803` (end of `Validate`)

- [ ] **Step 1: Write the failing test**

Create `internal/items/item_behavior_test.go`:

```go
package items

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// The two slice 1 keys load from item YAML (lighting 5e, Rules 1 and 10).
func TestItemSpecLoadsBehaviorAndFixture(t *testing.T) {
	var spec ItemSpec
	if err := yaml.Unmarshal([]byte("itemid: 55\nname: Arch Lantern\nbehavior: dusk_to_dawn\nfixture: light\n"), &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Behavior != "dusk_to_dawn" || spec.Fixture != FixtureLight {
		t.Errorf("Behavior=%q Fixture=%q, want dusk_to_dawn, light", spec.Behavior, spec.Fixture)
	}
}

func TestValidateRefusesAnUnknownFixtureKind(t *testing.T) {
	for _, ok := range []string{"", FixtureLight, FixtureDarkness} {
		spec := ItemSpec{ItemId: 1, Name: "x", Value: 1, Fixture: ok}
		if err := spec.Validate(); err != nil {
			t.Errorf("fixture %q refused: %v", ok, err)
		}
	}
	spec := ItemSpec{ItemId: 1, Name: "x", Value: 1, Fixture: "lamp"}
	if err := spec.Validate(); err == nil || !strings.Contains(err.Error(), "fixture") {
		t.Errorf("fixture lamp: err = %v, want a fixture refusal", err)
	}
}

// IsFixture and HasBehavior read the template, so an item instance carrying
// an override spec (an enchanted or renamed copy) cannot shed either.
func TestIsFixtureAndHasBehaviorReadTheTemplate(t *testing.T) {
	t.Cleanup(SeedItemsForTest(map[int]*ItemSpec{
		55: {ItemId: 55, Name: "Arch Lantern", Fixture: FixtureLight, Behavior: "dusk_to_dawn"},
		56: {ItemId: 56, Name: "Plain Stone"},
	}))
	fixture := Item{ItemId: 55, Spec: &ItemSpec{ItemId: 55, Name: "Renamed"}}
	if !fixture.IsFixture() || !fixture.HasBehavior() {
		t.Errorf("an overridden fixture reads IsFixture=%v HasBehavior=%v, want both true", fixture.IsFixture(), fixture.HasBehavior())
	}
	plain := Item{ItemId: 56}
	if plain.IsFixture() || plain.HasBehavior() {
		t.Error("a plain item reads as a fixture or as treed")
	}
	if (Item{}).IsFixture() || (Item{}).HasBehavior() {
		t.Error("an empty item reads as a fixture or as treed")
	}
}

// The holder index (Rule 5) holds mobs and rooms, not items: each enters
// once, leaves when dropped, and lists in id order.
func TestHolderIndex(t *testing.T) {
	t.Cleanup(ResetHolderIndexForTest())
	IndexMobHolder(9)
	IndexMobHolder(3)
	IndexMobHolder(9)
	IndexMobHolder(0) // never a holder
	if got := MobHolders(); len(got) != 2 || got[0] != 3 || got[1] != 9 {
		t.Errorf("MobHolders = %v, want [3 9]", got)
	}
	DropMobHolder(3)
	if got := MobHolders(); len(got) != 1 || got[0] != 9 {
		t.Errorf("after DropMobHolder(3), MobHolders = %v, want [9]", got)
	}

	var indexed []int
	OnRoomHolderIndexed = func(roomId int) { indexed = append(indexed, roomId) }
	t.Cleanup(func() { OnRoomHolderIndexed = nil })
	IndexRoomHolder(4111)
	IndexRoomHolder(4111)
	IndexRoomHolder(5000)
	if len(indexed) != 2 || indexed[0] != 4111 || indexed[1] != 5000 {
		t.Errorf("OnRoomHolderIndexed saw %v, want [4111 5000]: once per room entering the index", indexed)
	}
	if got := RoomHolders(); len(got) != 2 || got[0] != 4111 || got[1] != 5000 {
		t.Errorf("RoomHolders = %v, want [4111 5000]", got)
	}
	DropRoomHolder(4111)
	IndexRoomHolder(4111)
	if len(indexed) != 3 {
		t.Errorf("a room re-entering the index was not reported again: %v", indexed)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/items/ -run 'ItemSpecLoadsBehavior|UnknownFixture|IsFixtureAndHas|HolderIndex' -count=1`
Expected: FAIL to build: `unknown field Fixture in struct literal of type ItemSpec`, `undefined: FixtureLight`.

- [ ] **Step 3: Add the two fields and their validation**

In `internal/items/itemspec.go`, replace

```go
	Restricted       bool     `yaml:"restricted,omitempty"`        // Contraband: bid on by the auction Official (The Crown Assessor, econ #2.5). Interest tag only — no other mechanics.
```

with

```go
	Restricted       bool     `yaml:"restricted,omitempty"`        // Contraband: bid on by the auction Official (The Crown Assessor, econ #2.5). Interest tag only — no other mechanics.

	// Behavior names the item's behaviour tree,
	// behaviors/items/<name>.yaml (lighting 5e, item behaviour slice 1,
	// Rule 1). Several items may share one tree. The tree belongs to the
	// template: read it through GetItemSpec or Item.HasBehavior, never an
	// instance's override spec. The boot panics on a name that does not
	// resolve (behaviortree.ValidateItemBehaviors).
	Behavior string `yaml:"behavior,omitempty"`
	// Fixture marks an item fixed to a room's floor: FixtureLight or
	// FixtureDarkness (Rule 10). No path takes it off the floor, "On the
	// Ground" leaves it out, and its tree's set_light / pulse_light output
	// lights (or darkens) the room through internal/itemlight.
	Fixture string `yaml:"fixture,omitempty"`
```

(The `Restricted` line is existing text and keeps its dash; the added lines carry none.) Then, at the end of `Validate`, replace

```go
	if i.MutationRarityFloor < 0 || i.MutationRarityFloor > 10 {
		return fmt.Errorf("item %d: mutation_rarity_floor must be 0-10, got %d", i.ItemId, i.MutationRarityFloor)
	}

	return nil
}
```

with

```go
	if i.MutationRarityFloor < 0 || i.MutationRarityFloor > 10 {
		return fmt.Errorf("item %d: mutation_rarity_floor must be 0-10, got %d", i.ItemId, i.MutationRarityFloor)
	}
	switch i.Fixture {
	case ``, FixtureLight, FixtureDarkness:
	default:
		return fmt.Errorf("item %d: fixture must be %q or %q, got %q", i.ItemId, FixtureLight, FixtureDarkness, i.Fixture)
	}

	return nil
}
```

- [ ] **Step 4: Create the helpers and the holder index**

Create `internal/items/item_behavior.go`:

```go
package items

import (
	"sort"
	"sync"
)

// Fixture kinds (ItemSpec.Fixture, lighting 5e Rule 10).
const (
	FixtureLight    = `light`
	FixtureDarkness = `darkness`
)

// HasBehavior reports whether the item's TEMPLATE names a behaviour tree. A
// tree belongs to the template (spec X4), so an instance with an override
// spec still runs its template's tree.
func (i Item) HasBehavior() bool {
	spec := GetItemSpec(i.ItemId)
	return spec != nil && spec.Behavior != ``
}

// IsFixture reports whether the item's template fixes it to a room's floor.
// Every floor-removal path refuses a fixture (actions.ErrFixture).
func (i Item) IsFixture() bool {
	spec := GetItemSpec(i.ItemId)
	return spec != nil && spec.Fixture != ``
}

// The item tick's holder index (lighting 5e Rule 5). It holds HOLDERS, not
// items: a mob instance or a room that may hold an item with a behaviour
// tree. The tick (internal/hooks.ItemRoundTick) visits only these, plus every
// online player, so no round walks the world; a visited holder with no treed
// item left is dropped.
//
// A mob enters at spawn and whenever it stores or wears a treed item
// (internal/characters.(*Character).IndexTreedItem). A room enters when a
// treed item lands on its floor (Room.AddItem, Room.Prepare's spawn append)
// and when it loads into memory holding one.
var (
	holderMu    sync.Mutex
	mobHolders  = map[int]struct{}{}
	roomHolders = map[int]struct{}{}

	// OnRoomHolderIndexed, when set, is called each time a room ENTERS the
	// index, outside the lock. internal/hooks sets it to evaluate the room's
	// treed floor items at once, so a fixture is never dark for a round on
	// a room's first visit. Nil in a unit test that does not set it.
	OnRoomHolderIndexed func(roomId int)
)

// IndexMobHolder enters a mob instance in the index. Ids below 1 are ignored.
func IndexMobHolder(instanceId int) {
	if instanceId < 1 {
		return
	}
	holderMu.Lock()
	mobHolders[instanceId] = struct{}{}
	holderMu.Unlock()
}

// IndexRoomHolder enters a room in the index, and reports a room that was
// not already in it to OnRoomHolderIndexed.
func IndexRoomHolder(roomId int) {
	holderMu.Lock()
	_, already := roomHolders[roomId]
	roomHolders[roomId] = struct{}{}
	holderMu.Unlock()
	if !already && OnRoomHolderIndexed != nil {
		OnRoomHolderIndexed(roomId)
	}
}

// DropMobHolder removes a mob instance from the index.
func DropMobHolder(instanceId int) {
	holderMu.Lock()
	delete(mobHolders, instanceId)
	holderMu.Unlock()
}

// DropRoomHolder removes a room from the index.
func DropRoomHolder(roomId int) {
	holderMu.Lock()
	delete(roomHolders, roomId)
	holderMu.Unlock()
}

// MobHolders returns the indexed mob instance ids in ascending order.
func MobHolders() []int {
	holderMu.Lock()
	defer holderMu.Unlock()
	return sortedKeys(mobHolders)
}

// RoomHolders returns the indexed room ids in ascending order.
func RoomHolders() []int {
	holderMu.Lock()
	defer holderMu.Unlock()
	return sortedKeys(roomHolders)
}

func sortedKeys(m map[int]struct{}) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// ResetHolderIndexForTest empties the index and returns a restore func.
func ResetHolderIndexForTest() func() {
	holderMu.Lock()
	origMobs, origRooms := mobHolders, roomHolders
	mobHolders, roomHolders = map[int]struct{}{}, map[int]struct{}{}
	holderMu.Unlock()
	return func() {
		holderMu.Lock()
		mobHolders, roomHolders = origMobs, origRooms
		holderMu.Unlock()
	}
}
```

- [ ] **Step 5: Run it to see it pass**

Run: `go test ./internal/items/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/items`.

- [ ] **Step 6: Commit**

```bash
git add internal/items/itemspec.go internal/items/item_behavior.go internal/items/item_behavior_test.go
git commit -m "feat(items): behavior and fixture keys, the item tick's holder index (5e Rules 1, 5, 10)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: The item subject in the engine: `TryItemBehavior`, state, loader, allowlist, boot check (Rules 1 to 4, 8, 14; X11 to X13)

**Model:** sonnet (several files, the compile path and the boot).

**Files:**
- Create: `internal/behaviortree/item_engine_test.go`, `internal/behaviortree/item_engine.go`, `internal/behaviortree/item_state.go`
- Modify: `internal/behaviortree/types.go:62` (end of `EvalContext`), `conditions.go:72-74`, `actions.go:170-206`, `engine.go:22,42,208`, `loader.go:3-8,71,128-131,143-146`, `save.go:231-232,239-240,257-263`, `test_export.go` (append), `main.go:1672`

- [ ] **Step 1: Write the failing tests**

Create `internal/behaviortree/item_engine_test.go`:

```go
package behaviortree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

const (
	engineProbeItemId = 9901
	engineProbeRoomId = 9900
	engineProbeMobId  = 9902
	engineProbeInstId = 9903
)

// countingTree counts its item_idle visits in the item's own state.
const countingTree = `
tree:
  type: action
  event: item_idle
  do: increment_state
  key: visits
`

// seedItemEngineWorld seeds one treed item template, one player (user 1)
// and one mob instance, all in one loaded room.
func seedItemEngineWorld(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		engineProbeItemId: {ItemId: engineProbeItemId, Name: "Probe Lamp", Behavior: "engine_probe"},
	}))
	LoadItemTreeForTest(t, "engine_probe", countingTree)
	room := rooms.NewRoom("probe")
	room.RoomId = engineProbeRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{engineProbeRoomId: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(1, "probe", "Probe", 0)
	u.Character.RoomId = engineProbeRoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u}))
	t.Cleanup(seedTestMob(t, engineProbeMobId, engineProbeInstId, engineProbeRoomId, "Probe Keeper"))
	return u
}

func idleEvent() EventContext { return EventContext{EventType: "item_idle"} }

// Rule 3: the tree runs for an item wherever it is, its state is per item
// instance (UUID), and the subject's room is the holder's room.
func TestTryItemBehaviorRunsWornCarriedMobHeldAndFloor(t *testing.T) {
	seedItemEngineWorld(t)
	t.Cleanup(ResetItemBTreeStatesForTest())

	subjects := map[string]ItemSubject{
		"worn":     {UUID: uuid.UUID{1}, ItemId: engineProbeItemId, UserId: 1, Slot: "light"},
		"backpack": {UUID: uuid.UUID{2}, ItemId: engineProbeItemId, UserId: 1},
		"mob-held": {UUID: uuid.UUID{3}, ItemId: engineProbeItemId, MobInstanceId: engineProbeInstId, Slot: "light"},
		"floor":    {UUID: uuid.UUID{4}, ItemId: engineProbeItemId, RoomId: engineProbeRoomId, OnFloor: true},
	}
	for name, s := range subjects {
		if !TryItemBehavior(idleEvent(), s) {
			t.Errorf("%s: TryItemBehavior = false, want true (the tree succeeds)", name)
		}
	}
	TryItemBehavior(idleEvent(), subjects["worn"])
	if got := ItemBTreeStateForTest(uuid.UUID{1}).GetInt("visits"); got != 2 {
		t.Errorf("worn item visits = %d, want 2", got)
	}
	for _, id := range []uuid.UUID{{2}, {3}, {4}} {
		if got := ItemBTreeStateForTest(id).GetInt("visits"); got != 1 {
			t.Errorf("item %v visits = %d, want 1: state is per instance", id, got)
		}
	}
}

// Rule 3 and Rule 14: a holder or room that is gone skips the round.
func TestTryItemBehaviorSkipsAGoneHolder(t *testing.T) {
	seedItemEngineWorld(t)
	t.Cleanup(ResetItemBTreeStatesForTest())
	for name, s := range map[string]ItemSubject{
		"logged-out player": {UUID: uuid.UUID{5}, ItemId: engineProbeItemId, UserId: 77},
		"despawned mob":     {UUID: uuid.UUID{6}, ItemId: engineProbeItemId, MobInstanceId: 7777},
		"unloaded room":     {UUID: uuid.UUID{7}, ItemId: engineProbeItemId, RoomId: 123456, OnFloor: true},
		"no holder at all":  {UUID: uuid.UUID{8}, ItemId: engineProbeItemId},
		"untreed item":      {UUID: uuid.UUID{9}, ItemId: 1, UserId: 1},
	} {
		if TryItemBehavior(idleEvent(), s) {
			t.Errorf("%s: TryItemBehavior = true, want false", name)
		}
	}
}

// Rule 14: a panic inside a node is recovered, logged, and the item does
// nothing that round; the next item still runs.
func TestTryItemBehaviorRecoversAPanickingNode(t *testing.T) {
	seedItemEngineWorld(t)
	t.Cleanup(ResetItemBTreeStatesForTest())
	actionRegistry["probe_panic"] = func(map[string]any, *EvalContext) Result { panic("probe") }
	itemSafeActions["probe_panic"] = true
	t.Cleanup(func() {
		delete(actionRegistry, "probe_panic")
		delete(itemSafeActions, "probe_panic")
	})
	LoadItemTreeForTest(t, "engine_probe", "tree:\n  type: action\n  do: probe_panic\n")
	if TryItemBehavior(idleEvent(), ItemSubject{UUID: uuid.UUID{1}, ItemId: engineProbeItemId, UserId: 1}) {
		t.Error("a panicking tree reported Success")
	}
}

// Rule 8: an item tree may name only the item-safe allowlist; anything else
// refuses at compile time with its path.
func TestItemTreeAllowlistRefusesMobNodes(t *testing.T) {
	for _, bad := range []string{
		"tree:\n  type: action\n  do: attack\n",
		"tree:\n  type: selector\n  children:\n    - type: condition\n      check: mob_in_combat\n",
	} {
		_, err := LoadItemTreeFromBytes([]byte(bad))
		if err == nil || !strings.Contains(err.Error(), "item tree") || !strings.Contains(err.Error(), "item") {
			t.Errorf("compiling %q: err = %v, want an item-tree refusal naming the path", bad, err)
		}
	}
	ok := "tree:\n  type: decorator\n  mod: cooldown\n  rounds: 3\n  child:\n    type: sequence\n    children:\n      - type: condition\n        check: time_of_day\n        period: night\n      - type: action\n        do: set_state\n        key: k\n        value: v\n"
	if _, err := LoadItemTreeFromBytes([]byte(ok)); err != nil {
		t.Errorf("an all-safe item tree refused: %v", err)
	}
}

// X12: a behavior: naming no file, or a file that does not compile, is a
// boot failure that names the item.
func TestValidateItemBehaviorsRefusesAMissingOrBrokenTree(t *testing.T) {
	dir := overrideBTDataFilesDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "behaviors", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "behaviors", "items", "broken_probe.yaml"),
		[]byte("tree:\n  type: action\n  do: attack\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		9911: {ItemId: 9911, Name: "Lost Lamp", Behavior: "no_such_tree"},
		9912: {ItemId: 9912, Name: "Bad Lamp", Behavior: "broken_probe"},
		9913: {ItemId: 9913, Name: "Plain Rock"},
	}))
	t.Cleanup(func() {
		e := GetEngine()
		e.EvictItemTree("no_such_tree")
		e.EvictItemTree("broken_probe")
	})
	err := ValidateItemBehaviors()
	if err == nil {
		t.Fatal("ValidateItemBehaviors passed a missing tree and a broken one")
	}
	for _, want := range []string{"item 9911", "no_such_tree", "item 9912", "broken_probe"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "9913") {
		t.Errorf("error names an item with no behavior: %q", err)
	}
}

// X13: item trees are a fourth kind, and behaviors/items is not a mob zone.
func TestListTreeFiles_ListsItemTreesAsTheirOwnKind(t *testing.T) {
	dir := overrideBTDataFilesDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "behaviors", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lister_probe.yaml", "424245-numbered_probe.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, "behaviors", "items", name), []byte(countingTree), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	found := false
	for _, r := range ListTreeFiles() {
		if r.Zone == "items" {
			t.Errorf("behaviors/items was listed as a mob zone: %+v", r)
		}
		if r.Kind == "item" && r.Name == "lister_probe" {
			found = true
		}
	}
	if !found {
		t.Error("the item tree lister_probe was not listed as kind item")
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/behaviortree/ -run 'TryItemBehavior|ItemTreeAllowlist|ValidateItemBehaviors|ListsItemTrees' -count=1`
Expected: FAIL to build: `undefined: ItemBTreeStateForTest`, `undefined: ResetItemBTreeStatesForTest`, `undefined: ItemSubject`, `undefined: TryItemBehavior`.

- [ ] **Step 3: The subject handle on `EvalContext`, and the evaluating node's name**

In `internal/behaviortree/types.go`, replace

```go
	// that this slot exists to prevent.
	SoftTarget state.ActorRef
}
```

with

```go
	// that this slot exists to prevent.
	SoftTarget state.ActorRef

	// Item is the subject when an ITEM's tree runs (lighting 5e, Rule 2):
	// the instance and where it is. Nil for mob and room trees. For an item,
	// MobState is the item's own state and MobId / InstanceId are 0.
	Item *ItemSubject

	// node is the name of the condition or action evaluating now, so a
	// recovered panic can name it (TryItemBehavior, Rule 14).
	node string
}
```

In `internal/behaviortree/conditions.go`, replace

```go
func (n *ConditionNode) Evaluate(ctx *EvalContext) Result {
	return n.Fn(n.Params, ctx)
}
```

with

```go
func (n *ConditionNode) Evaluate(ctx *EvalContext) Result {
	if ctx != nil {
		ctx.node = n.Name
	}
	return n.Fn(n.Params, ctx)
}
```

In `internal/behaviortree/actions.go`, the two closures that copy the context field by field (`:170-178` for a static `delay:`, `:191-199` for a perception delay) must carry the item too (F2). Replace BOTH occurrences of

```go
				MobName:     ctx.MobName,
				Intercepted: ctx.Intercepted,
			}
```

with (Edit tool, `replace_all: true`)

```go
				MobName:     ctx.MobName,
				Intercepted: ctx.Intercepted,
				Item:        ctx.Item,
			}
```

and replace the last two lines of `ActionNode.Evaluate`

```go
			return Success
		}
	}
	return n.Fn(n.Params, ctx)
}
```

with

```go
			return Success
		}
	}
	if ctx != nil {
		ctx.node = n.Name
	}
	return n.Fn(n.Params, ctx)
}
```

- [ ] **Step 4: Item tree state**

Create `internal/behaviortree/item_state.go`:

```go
package behaviortree

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// Item tree state (lighting 5e, Rule 4): one BehaviorState per item
// INSTANCE, keyed by Item.UUID, in memory only. A UUID is minted afresh on
// every load (restart, login, mob instance restore, buy), so this state
// resets with it: cooldowns start over and a schedule recomputes from the
// hour. It is the room_state.go shape plus the round each entry was last
// visited, so the item tick can evict what it no longer reaches.
type itemStateEntry struct {
	state    *BehaviorState
	lastSeen uint64
}

var (
	itemStateMu sync.RWMutex
	itemStates  = make(map[uuid.UUID]*itemStateEntry)
)

// EnsureItemBTreeState returns the item instance's state, creating it on
// first use, and marks it visited this round.
func EnsureItemBTreeState(id uuid.UUID) *BehaviorState {
	round := util.GetRoundCount()
	itemStateMu.Lock()
	defer itemStateMu.Unlock()
	e, ok := itemStates[id]
	if !ok {
		e = &itemStateEntry{state: NewBehaviorState()}
		itemStates[id] = e
	}
	e.lastSeen = round
	return e.state
}

// EvictItemBTreeState drops one item instance's state.
func EvictItemBTreeState(id uuid.UUID) {
	itemStateMu.Lock()
	defer itemStateMu.Unlock()
	delete(itemStates, id)
}

// EvictUnseenItemBTreeStates drops the state of every item the tick did not
// visit this round, so state follows items the tick still reaches. An item
// handed from a mob to a player is visited in the same or the next tick and
// keeps its state. Returns how many were dropped.
func EvictUnseenItemBTreeStates(round uint64) int {
	itemStateMu.Lock()
	defer itemStateMu.Unlock()
	n := 0
	for id, e := range itemStates {
		if e.lastSeen < round {
			delete(itemStates, id)
			n++
		}
	}
	return n
}

// ItemBTreeStateForTest returns an item's state without marking it visited,
// or nil when it has none.
func ItemBTreeStateForTest(id uuid.UUID) *BehaviorState {
	itemStateMu.RLock()
	defer itemStateMu.RUnlock()
	if e, ok := itemStates[id]; ok {
		return e.state
	}
	return nil
}

// ResetItemBTreeStatesForTest empties the item state map and returns a
// restore func.
func ResetItemBTreeStatesForTest() func() {
	itemStateMu.Lock()
	orig := itemStates
	itemStates = make(map[uuid.UUID]*itemStateEntry)
	itemStateMu.Unlock()
	return func() {
		itemStateMu.Lock()
		itemStates = orig
		itemStateMu.Unlock()
	}
}
```

- [ ] **Step 5: The engine's item tree cache**

In `internal/behaviortree/engine.go`, replace

```go
	archetypeDefaultGoals map[string][]GoalDefault      // chunk 4.3 — per-archetype default goals
	queue                 []DelayedAction
}
```

with

```go
	archetypeDefaultGoals map[string][]GoalDefault      // chunk 4.3 — per-archetype default goals
	itemTrees             map[string]Node               // item tree name → compiled root node (lighting 5e)
	noItemTree            map[string]bool               // item tree name → its file failed to load
	queue                 []DelayedAction
}
```

(the first line is existing text with its dash), replace

```go
		archetypeDefaultGoals: make(map[string][]GoalDefault),
	}
```

with

```go
		archetypeDefaultGoals: make(map[string][]GoalDefault),
		itemTrees:             make(map[string]Node),
		noItemTree:            make(map[string]bool),
	}
```

and insert directly above `// GetArchetypeGoalWeights returns the cached goal_weights map for the` (`:204`):

```go
// LoadItemTree loads and caches a named item tree (lighting 5e), compiled
// under the root label "item" and the item-safe allowlist. Clears any
// negative entry.
func (e *Engine) LoadItemTree(name string, path string) error {
	node, err := LoadItemTreeFromFile(path)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.itemTrees[name] = node
	delete(e.noItemTree, name)
	e.mu.Unlock()
	return nil
}

// GetItemTree returns the cached item tree by name, or nil.
func (e *Engine) GetItemTree(name string) Node {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.itemTrees[name]
}

// HasNoItemTree reports whether the named item tree failed to load.
func (e *Engine) HasNoItemTree(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.noItemTree[name]
}

// SetNoItemTree records that the named item tree failed to load, so the
// tick does not re-read the file every round.
func (e *Engine) SetNoItemTree(name string) {
	e.mu.Lock()
	e.noItemTree[name] = true
	e.mu.Unlock()
}

// EvictItemTree removes an item tree and its negative entry, so the next
// request reads the file again.
func (e *Engine) EvictItemTree(name string) {
	e.mu.Lock()
	delete(e.itemTrees, name)
	delete(e.noItemTree, name)
	e.mu.Unlock()
}

```

- [ ] **Step 6: The item loader and the allowlist**

In `internal/behaviortree/loader.go`, add `"strings"` to the imports (between `"os"` and the blank line before `"gopkg.in/yaml.v2"`), insert directly above `// compileNode recursively converts a NodeDef into a concrete Node.` (`:70`):

```go
// itemRootLabel is the root label item trees compile under (lighting 5e,
// Rule 1), so their decorator state keys can never meet a mob's ("root")
// or an archetype's ("arch"), and so the compiler knows which subject a
// node is being compiled for.
const itemRootLabel = "item"

// LoadItemTreeFromFile reads an item tree YAML file and compiles it.
func LoadItemTreeFromFile(path string) (Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadItemTreeFromBytes(data)
}

// LoadItemTreeFromBytes parses an item tree and compiles it under the item
// root label, which holds every node to the item-safe allowlist (Rule 8).
func LoadItemTreeFromBytes(data []byte) (Node, error) {
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return compileNode(def.Tree, itemRootLabel)
}

// isItemTreePath reports whether a compile path belongs to an item tree.
func isItemTreePath(path string) bool {
	return path == itemRootLabel || strings.HasPrefix(path, itemRootLabel+".")
}

// Rule 8: the nodes an item tree may name. An allowlist, not a blocklist
// (X11): the 87 mob and room actions and 66 conditions carry no subject
// metadata, and a node not read for an item subject is refused rather than
// trusted. Decorators and composites are always allowed.
var (
	itemSafeConditions = map[string]bool{
		"time_of_day":        true,
		"round_mod":          true,
		"random_chance":      true,
		"state_equals":       true,
		"state_greater_than": true,
	}
	itemSafeActions = map[string]bool{
		"set_state":       true,
		"increment_state": true,
		"decrement_state": true,
	}
)

// checkNodeSubject refuses a node the tree's subject may not name.
func checkNodeSubject(path, kind, name string) error {
	if isItemTreePath(path) {
		safe := itemSafeConditions
		if kind == "action" {
			safe = itemSafeActions
		}
		if !safe[name] {
			return fmt.Errorf("%s: %s %q is not allowed in an item tree (item-safe nodes only)", path, kind, name)
		}
	}
	return nil
}

```

then replace

```go
	fn := LookupCondition(def.Check)
	if fn == nil {
		return nil, fmt.Errorf("%s: unknown condition %q", path, def.Check)
	}
```

with

```go
	fn := LookupCondition(def.Check)
	if fn == nil {
		return nil, fmt.Errorf("%s: unknown condition %q", path, def.Check)
	}
	if err := checkNodeSubject(path, "condition", def.Check); err != nil {
		return nil, err
	}
```

and replace

```go
	fn := LookupAction(def.Do)
	if fn == nil {
		return nil, fmt.Errorf("%s: unknown action %q", path, def.Do)
	}
```

with

```go
	fn := LookupAction(def.Do)
	if fn == nil {
		return nil, fmt.Errorf("%s: unknown action %q", path, def.Do)
	}
	if err := checkNodeSubject(path, "action", def.Do); err != nil {
		return nil, err
	}
```

- [ ] **Step 7: `TryItemBehavior` and `ValidateItemBehaviors`**

Create `internal/behaviortree/item_engine.go` (Task 5 extends `ValidateItemBehaviors` with the one-writer rule):

```go
package behaviortree

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// Items are the third subject of the behaviour-tree engine, beside mobs and
// rooms (lighting 5e, item behaviour slice 1, spec Shape A).

// ItemSubject is the item a tree runs for, and where it is (Rule 2).
type ItemSubject struct {
	UUID          uuid.UUID // the instance; keys its tree state
	ItemId        int       // the template; names its tree
	UserId        int       // the player holding it, 0 if none
	MobInstanceId int       // the mob holding it, 0 if none
	RoomId        int       // its room: the holder's, or the floor's
	Slot          string    // the characters.Worn AllSlots key it is worn in; "" in a backpack or on a floor
	OnFloor       bool      // lying on RoomId's floor
}

// GetItemTreePath is the path of a named item tree:
// {dataFiles}/behaviors/items/{name}.yaml. Several items may share one tree
// (the archetype pattern, Rule 1); the name is authored, so it is not
// sanitized.
func GetItemTreePath(name string) string {
	dataFiles := configs.GetFilePathsConfig().DataFiles.String()
	return util.FilePath(dataFiles, `/`, `behaviors`, `/`, `items`, `/`, name+`.yaml`)
}

// resolveItemTree returns a named item tree, loading it on first request. At
// boot ValidateItemBehaviors has already loaded every tree a template names,
// so this load path serves tests and is a safety net; a failure is logged
// once and negative-cached.
func resolveItemTree(name string) Node {
	e := GetEngine()
	if tree := e.GetItemTree(name); tree != nil {
		return tree
	}
	if e.HasNoItemTree(name) {
		return nil
	}
	if err := e.LoadItemTree(name, GetItemTreePath(name)); err != nil {
		mudlog.Error("TryItemBehavior", "tree", name, "error", err.Error())
		e.SetNoItemTree(name)
		return nil
	}
	return e.GetItemTree(name)
}

// loggedItemTreePanics keeps a node's panic to one log line per tree and
// node (the time_of_day log-once precedent).
var loggedItemTreePanics sync.Map

// TryItemBehavior runs an item's tree for an event (Rule 3), mirroring
// TryRoomBehavior: resolve the template's tree, resolve the holder (gone:
// return false, the round is skipped), build the context, evaluate. A panic
// in a node is recovered, logged once per tree and node, and returns false:
// the item does nothing that round (Rule 14). Returns true on Success.
func TryItemBehavior(event EventContext, subject ItemSubject) (handled bool) {
	spec := items.GetItemSpec(subject.ItemId)
	if spec == nil || spec.Behavior == `` {
		return false
	}
	tree := resolveItemTree(spec.Behavior)
	if tree == nil {
		return false
	}

	switch {
	case subject.UserId > 0:
		u := users.GetByUserId(subject.UserId)
		if u == nil || u.Character == nil {
			return false
		}
		subject.RoomId = u.Character.RoomId
	case subject.MobInstanceId > 0:
		m := mobs.GetInstance(subject.MobInstanceId)
		if m == nil {
			return false
		}
		subject.RoomId = m.Character.RoomId
	case subject.OnFloor:
		if !rooms.IsRoomLoaded(subject.RoomId) {
			return false
		}
	default:
		return false
	}

	event.RoomId = subject.RoomId
	ctx := &EvalContext{
		Event:    event,
		MobState: EnsureItemBTreeState(subject.UUID),
		RoomId:   subject.RoomId,
		Item:     &subject,
	}
	defer func() {
		if r := recover(); r != nil {
			key := spec.Behavior + `/` + ctx.node
			if _, already := loggedItemTreePanics.LoadOrStore(key, true); !already {
				mudlog.Error("TryItemBehavior", "tree", spec.Behavior, "node", ctx.node,
					"itemId", subject.ItemId, "error", fmt.Sprint(r))
			}
			handled = false
		}
	}()
	return tree.Evaluate(ctx) == Success
}

// ValidateItemBehaviors loads and compiles every item tree a template names
// and returns one error listing every problem, or nil. main.go panics on it
// after items load (X12, the voice_id precedent): a missing or broken tree
// fails the boot instead of leaving an item silently inert.
func ValidateItemBehaviors() error {
	specs := items.GetAllItemSpecsMap()
	ids := make([]int, 0, len(specs))
	for id, spec := range specs {
		if spec != nil && spec.Behavior != `` {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)

	e := GetEngine()
	loaded := map[string]error{}
	var problems []string
	for _, id := range ids {
		spec := specs[id]
		err, seen := loaded[spec.Behavior]
		if !seen {
			err = e.LoadItemTree(spec.Behavior, GetItemTreePath(spec.Behavior))
			loaded[spec.Behavior] = err
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("item %d (%s): behavior %q: %v", id, spec.Name, spec.Behavior, err))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
```

- [ ] **Step 8: `ListTreeFiles` lists item trees as a fourth kind (X13)**

In `internal/behaviortree/save.go`, replace

```go
	Kind   string // archetype | mob | room
	Name   string // archetype name (archetype kind)
```

with

```go
	Kind   string // archetype | mob | room | item
	Name   string // archetype or item tree name (archetype and item kinds)
```

replace

```go
// ListTreeFiles walks the behaviors tree and returns every archetype,
// per-mob, and room tree file.
```

with

```go
// ListTreeFiles walks the behaviors tree and returns every archetype,
// item, per-mob, and room tree file. behaviors/items is the item kind, never
// a mob zone (lighting 5e, X13).
```

replace

```go
	zoneDirs, err := os.ReadDir(root)
	if err != nil {
		return rows
	}
	for _, zd := range zoneDirs {
		if !zd.IsDir() || zd.Name() == "archetypes" || zd.Name() == "rooms" {
			continue
		}
```

with

```go
	// items/<name>.yaml (lighting 5e): named item trees, the fourth kind.
	if entries, err := os.ReadDir(filepath.Join(root, "items")); err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".yaml")
			rows = append(rows, TreeFileRow{Kind: "item", Name: name,
				Path: filepath.Join(root, "items", e.Name())})
		}
	}

	zoneDirs, err := os.ReadDir(root)
	if err != nil {
		return rows
	}
	for _, zd := range zoneDirs {
		if !zd.IsDir() || zd.Name() == "archetypes" || zd.Name() == "rooms" || zd.Name() == "items" {
			continue
		}
```

- [ ] **Step 9: The test hook other packages use**

Append to `internal/behaviortree/test_export.go`:

```go

// LoadItemTreeForTest compiles yamlText as an item tree (the item root label
// and the item-safe allowlist, exactly as a file would load) and installs it
// under name for the rest of the test. hooks and the repo-root guards use it
// to drive TryItemBehavior without a behaviors/items file.
func LoadItemTreeForTest(t *testing.T, name string, yamlText string) {
	t.Helper()
	node, err := LoadItemTreeFromBytes([]byte(yamlText))
	if err != nil {
		t.Fatalf("LoadItemTreeForTest(%s): %v", name, err)
	}
	e := GetEngine()
	e.mu.Lock()
	prev, hadPrev := e.itemTrees[name]
	e.itemTrees[name] = node
	delete(e.noItemTree, name)
	e.mu.Unlock()
	t.Cleanup(func() {
		e.mu.Lock()
		if hadPrev {
			e.itemTrees[name] = prev
		} else {
			delete(e.itemTrees, name)
		}
		e.mu.Unlock()
	})
}
```

- [ ] **Step 10: The boot check (X12)**

In `main.go`, replace

```go
	itemvoices.LoadDataFiles()
	// Messaging M3 item 7
```

with

```go
	itemvoices.LoadDataFiles()
	// Lighting 5e (item behaviour slice 1): every item's behavior: must name
	// a tree under behaviors/items/ that loads and compiles, or the boot fails
	// here rather than leave the item silently inert (spec X12).
	if err := behaviortree.ValidateItemBehaviors(); err != nil {
		panic(err)
	}
	// Messaging M3 item 7
```

- [ ] **Step 11: Run them to see them pass**

Run: `go build ./... && go vet ./internal/behaviortree/ . && go test ./internal/behaviortree/ -count=1`
Expected: build and vet silent; `ok  	github.com/GoMudEngine/GoMud/internal/behaviortree`.

- [ ] **Step 12: Commit**

```bash
git add internal/behaviortree/types.go internal/behaviortree/conditions.go internal/behaviortree/actions.go internal/behaviortree/engine.go internal/behaviortree/loader.go internal/behaviortree/save.go internal/behaviortree/test_export.go internal/behaviortree/item_engine.go internal/behaviortree/item_state.go internal/behaviortree/item_engine_test.go main.go
git commit -m "feat(behaviortree): items as the third tree subject (5e Rules 1 to 4, 8, 14)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Item nodes: `holder_asleep`, `worn`, `in_combat`, `set_light`, `pulse_light`; one writer per record (Rules 7 to 9, 11; R2, R3; X22)

**Model:** sonnet (the record-writing contract with the trim).

**Files:**
- Create: `internal/behaviortree/item_light_test.go`, `internal/behaviortree/conditions_item.go`, `internal/behaviortree/actions_item_light.go`
- Modify: `internal/behaviortree/conditions.go:56` (registry), `actions.go:124` (registry), `loader.go` (the Task 4 allowlist), `item_engine.go` (imports, `ValidateItemBehaviors`, `TreeWritesLight`)

- [ ] **Step 1: Write the failing tests**

Create `internal/behaviortree/item_light_test.go`:

```go
package behaviortree

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

const (
	lightProbeLanternId   = 9921 // a lantern: type light, worn condition 9925
	lightProbeHoodedId    = 9922 // a hooded lantern: worn condition 9927 (adjustable)
	lightProbeFixtureId   = 9923 // a light fixture
	lightProbeDarkFixture = 9924 // a darkness fixture
	lightProbeLanternCond = 9925
	lightProbeSleepCond   = 9926
	lightProbeHoodedCond  = 9927
)

var lightProbeLanternUUID = uuid.UUID{0x51}

// seedItemLightWorld seeds the lantern, a hooded lantern, two fixtures, their
// conditions, and user 1 wearing the lantern in the light slot with its
// record held at full strength, in a loaded room.
func seedItemLightWorld(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		lightProbeLanternCond: {ConditionId: lightProbeLanternCond, Name: "Probe Lantern Light", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 52}}},
		lightProbeHoodedCond: {ConditionId: lightProbeHoodedCond, Name: "Probe Hooded Light", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 54}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
		lightProbeSleepCond: {ConditionId: lightProbeSleepCond, Name: "Probe Sleeping", TriggerCount: 4, RoundInterval: 1,
			Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		lightProbeLanternId: {ItemId: lightProbeLanternId, Name: "Probe Lantern", Type: items.Light, Subtype: items.Wearable,
			WornConditionIds: []int{lightProbeLanternCond}, Behavior: "light_probe"},
		lightProbeHoodedId: {ItemId: lightProbeHoodedId, Name: "Probe Hooded Lantern", Type: items.Light, Subtype: items.Wearable,
			WornConditionIds: []int{lightProbeHoodedCond}, Behavior: "light_probe"},
		lightProbeFixtureId: {ItemId: lightProbeFixtureId, Name: "Probe Lamp Post", Type: items.Object,
			Fixture: items.FixtureLight, Behavior: "light_probe"},
		lightProbeDarkFixture: {ItemId: lightProbeDarkFixture, Name: "Probe Shadow Stone", Type: items.Object,
			Fixture: items.FixtureDarkness, Behavior: "light_probe"},
	}))
	room := rooms.NewRoom("probe")
	room.RoomId = engineProbeRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{engineProbeRoomId: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(1, "probe", "Probe", 0)
	u.Character.RoomId = engineProbeRoomId
	u.Character.Equipment.Light = items.Item{ItemId: lightProbeLanternId, UUID: lightProbeLanternUUID}
	u.Character.Conditions.AddCondition(lightProbeLanternCond, true)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u}))
	t.Cleanup(ResetItemBTreeStatesForTest())
	t.Cleanup(itemlight.ResetForTest())
	return u
}

func lanternRecord(t *testing.T, c *characters.Character) *conditions.Condition {
	t.Helper()
	recs := c.Conditions.GetConditions(lightProbeLanternCond)
	if len(recs) != 1 {
		t.Fatalf("holder carries %d lantern records, want 1", len(recs))
	}
	return recs[0]
}

func runLightTree(t *testing.T, tree string, s ItemSubject) bool {
	t.Helper()
	LoadItemTreeForTest(t, "light_probe", tree)
	return TryItemBehavior(EventContext{EventType: "item_idle"}, s)
}

var wornLantern = ItemSubject{UUID: lightProbeLanternUUID, ItemId: lightProbeLanternId, UserId: 1, Slot: "light"}

// Rule 9: set_light writes the record's existing trimmed-output state, one
// mechanism with the trim: off is LightOff (fully dark, EmitsLight false), a
// number is LightTrimmed at it, full is LightFull.
func TestSetLightWritesTheWornRecord(t *testing.T) {
	u := seedItemLightWorld(t)
	rec := lanternRecord(t, u.Character)

	if !runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n", wornLantern) {
		t.Fatal("set_light off on a worn lantern did not succeed")
	}
	if rec.LightTrim != conditions.LightOff || u.Character.EmitsLight() {
		t.Errorf("after off: LightTrim=%q EmitsLight=%v, want off and false", rec.LightTrim, u.Character.EmitsLight())
	}

	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: 30\n", wornLantern)
	if v, ok := rec.LightNow(conditions.GetConditionSpec(lightProbeLanternCond)); rec.LightTrim != conditions.LightTrimmed || !ok || v != 30 {
		t.Errorf("after 30: LightTrim=%q LightNow=%v,%v, want trimmed at 30", rec.LightTrim, v, ok)
	}

	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: full\n", wornLantern)
	if rec.LightTrim != conditions.LightFull || rec.LightOutput != 0 {
		t.Errorf("after full: LightTrim=%q LightOutput=%v, want full and 0", rec.LightTrim, rec.LightOutput)
	}
}

// YAML 1.1 reads an unquoted off as false; it means off all the same.
func TestSetLightReadsAnUnquotedOff(t *testing.T) {
	u := seedItemLightWorld(t)
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: off\n", wornLantern)
	if rec := lanternRecord(t, u.Character); rec.LightTrim != conditions.LightOff {
		t.Errorf("unquoted off: LightTrim=%q, want off", rec.LightTrim)
	}
}

// Rule 9: TrimLightFor owns adjustable records only, so a move into a new
// room cannot relight a lantern the scheduler put out.
func TestATrimOnEntryDoesNotRelightAScheduledOffLantern(t *testing.T) {
	u := seedItemLightWorld(t)
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n", wornLantern)
	room := rooms.LoadRoom(engineProbeRoomId)
	room.AddPlayer(u.UserId)
	room.TrimLightFor(u.Character)
	if rec := lanternRecord(t, u.Character); rec.LightTrim != conditions.LightOff {
		t.Errorf("after an entry trim: LightTrim=%q, want still off", rec.LightTrim)
	}
}

// A5: an equip is a fresh start, full strength until the next tick.
func TestAnEquipIsFullUntilTheNextTick(t *testing.T) {
	u := seedItemLightWorld(t)
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n", wornLantern)
	lantern := u.Character.Equipment.Light
	u.Character.Equipment.Light = items.Item{}
	if _, worn, reason := u.Character.Wear(lantern); !worn {
		t.Fatalf("re-equipping the lantern refused: %s", reason)
	}
	if rec := lanternRecord(t, u.Character); rec.LightTrim != conditions.LightFull {
		t.Errorf("after an equip: LightTrim=%q, want full", rec.LightTrim)
	}
}

// Rule 9: on an item that is neither worn nor a fixture both actions fail
// and change nothing.
func TestLightActionsFailForABackpackItem(t *testing.T) {
	u := seedItemLightWorld(t)
	pack := ItemSubject{UUID: uuid.UUID{0x52}, ItemId: lightProbeLanternId, UserId: 1}
	if runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n", pack) {
		t.Error("set_light on a backpack item succeeded")
	}
	if runLightTree(t, "tree:\n  type: action\n  do: pulse_light\n  min: 10\n  max: 20\n  period_rounds: 4\n", pack) {
		t.Error("pulse_light on a backpack item succeeded")
	}
	if rec := lanternRecord(t, u.Character); rec.LightTrim != conditions.LightFull {
		t.Errorf("the worn record moved: LightTrim=%q", rec.LightTrim)
	}
}

// Rule 10: a fixture's output lives in internal/itemlight, light or
// darkness by its kind; off records it unlit.
func TestSetLightWritesAFixtureToItemlight(t *testing.T) {
	seedItemLightWorld(t)
	post := ItemSubject{UUID: uuid.UUID{0x53}, ItemId: lightProbeFixtureId, RoomId: engineProbeRoomId, OnFloor: true}
	shadow := ItemSubject{UUID: uuid.UUID{0x54}, ItemId: lightProbeDarkFixture, RoomId: engineProbeRoomId, OnFloor: true}
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: 52\n", post)
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: 40\n", shadow)
	light, dark := itemlight.Terms(engineProbeRoomId)
	if len(light) != 1 || light[0] != 52 || len(dark) != 1 || dark[0] != 40 {
		t.Errorf("Terms = %v, %v; want [52], [40]", light, dark)
	}
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n", post)
	if v, ok := itemlight.Get(engineProbeRoomId, post.UUID); !ok || !math.IsInf(v, -1) {
		t.Errorf("after off the fixture reads %v, %v; want recorded unlit", v, ok)
	}
	if runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: full\n", post) {
		t.Error("set_light full on a fixture succeeded: a fixture has no full strength of its own")
	}
}

// Rule 11: pulse_light is a triangle wave over period_rounds, from the round
// count alone.
func TestPulseLightIsADeterministicTriangleWave(t *testing.T) {
	for _, c := range []struct {
		round uint64
		want  float64
	}{{0, 20}, {3, 28}, {6, 36}, {9, 28}, {12, 20}, {15, 28}} {
		if got := PulseLightValue(20, 36, 12, c.round); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("round %d: PulseLightValue(20, 36, 12) = %v, want %v", c.round, got, c.want)
		}
	}
	seedItemLightWorld(t)
	stone := ItemSubject{UUID: uuid.UUID{0x55}, ItemId: lightProbeFixtureId, RoomId: engineProbeRoomId, OnFloor: true}
	prev := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCountForTest(prev) })
	util.SetRoundCountForTest(6006) // 6006 % 12 == 6: the crest
	runLightTree(t, "tree:\n  type: action\n  do: pulse_light\n  min: 20\n  max: 36\n  period_rounds: 12\n", stone)
	if v, _ := itemlight.Get(engineProbeRoomId, stone.UUID); v != 36 {
		t.Errorf("at the crest the stone reads %v, want 36", v)
	}
}

// R2: holder_asleep reads the Sleeping flag; worn is an equipment slot;
// in_combat is the holder's combat state. A floor item has no holder.
func TestItemConditionsReadTheHolder(t *testing.T) {
	u := seedItemLightWorld(t)
	t.Cleanup(seedTestMob(t, engineProbeMobId, engineProbeInstId, engineProbeRoomId, "Probe Keeper"))
	cases := []struct {
		name string
		tree string
		s    ItemSubject
		want bool
	}{
		{"awake holder", "holder_asleep", wornLantern, false},
		{"worn", "worn", wornLantern, true},
		{"backpack", "worn", ItemSubject{UUID: uuid.UUID{0x56}, ItemId: lightProbeLanternId, UserId: 1}, false},
		{"holder at peace", "in_combat", wornLantern, false},
		{"floor has no holder", "holder_asleep", ItemSubject{UUID: uuid.UUID{0x57}, ItemId: lightProbeFixtureId, RoomId: engineProbeRoomId, OnFloor: true}, false},
	}
	for _, c := range cases {
		if got := runLightTree(t, "tree:\n  type: condition\n  check: "+c.tree+"\n", c.s); got != c.want {
			t.Errorf("%s: %s = %v, want %v", c.name, c.tree, got, c.want)
		}
	}
	u.Character.Conditions.AddCondition(lightProbeSleepCond, false)
	if !runLightTree(t, "tree:\n  type: condition\n  check: holder_asleep\n", wornLantern) {
		t.Error("holder_asleep with the Sleeping flag held: false, want true")
	}
	u.Character.SetAggro(0, engineProbeInstId, characters.DefaultAttack)
	if !runLightTree(t, "tree:\n  type: condition\n  check: in_combat\n", wornLantern) {
		t.Error("in_combat with the holder fighting: false, want true")
	}
}

// Rule 8: item-only nodes refuse in a mob or room tree.
func TestItemOnlyNodesRefuseInMobAndRoomTrees(t *testing.T) {
	for _, bad := range []string{
		"tree:\n  type: action\n  do: set_light\n  level: 52\n",
		"tree:\n  type: condition\n  check: holder_asleep\n",
	} {
		if _, err := LoadTreeFromBytes([]byte(bad)); err == nil || !strings.Contains(err.Error(), "item") {
			t.Errorf("a mob tree compiled %q: err = %v, want an item-only refusal", bad, err)
		}
	}
}

// Rule 8 and X22: one writer per record. A light-writing tree on an item
// whose worn light is adjustable (the trim owns it) refuses at boot.
func TestValidateItemBehaviorsRefusesALightTreeOnAnAdjustableLight(t *testing.T) {
	dir := overrideBTDataFilesDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "behaviors", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "behaviors", "items", "light_probe.yaml"),
		[]byte("tree:\n  type: action\n  do: set_light\n  level: full\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedItemLightWorld(t)
	t.Cleanup(func() { GetEngine().EvictItemTree("light_probe") })
	err := ValidateItemBehaviors()
	if err == nil || !strings.Contains(err.Error(), "item 9922") || !strings.Contains(err.Error(), "adjustable") {
		t.Fatalf("err = %v, want item 9922 refused for an adjustable light", err)
	}
	if strings.Contains(err.Error(), "item 9921") {
		t.Errorf("the plain lantern was refused too: %v", err)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/behaviortree/ -run 'SetLight|TrimOnEntry|EquipIsFull|LightActionsFail|PulseLight|ItemConditions|ItemOnlyNodes|AdjustableLight' -count=1`
Expected: FAIL to build: `undefined: PulseLightValue`.

- [ ] **Step 3: The three item conditions**

Create `internal/behaviortree/conditions_item.go`:

```go
package behaviortree

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Item-subject conditions (lighting 5e, Rule 7). Each needs an item
// subject: the compiler refuses them in a mob or room tree (Rule 8).

// itemHolder is the character holding the subject item: its player or its
// mob. Nil for a floor item, a mob or room tree, or a holder that is gone.
func itemHolder(ctx *EvalContext) *characters.Character {
	if ctx == nil || ctx.Item == nil {
		return nil
	}
	if ctx.Item.UserId > 0 {
		if u := users.GetByUserId(ctx.Item.UserId); u != nil {
			return u.Character
		}
		return nil
	}
	if ctx.Item.MobInstanceId > 0 {
		if m := mobs.GetInstance(ctx.Item.MobInstanceId); m != nil {
			return &m.Character
		}
	}
	return nil
}

// condHolderAsleep: the holder holds the Sleeping flag. It reads
// actions.TargetAsleep, the predicate ShopClosedForSleep reads, not the
// schedule, so a keeper roused inside a sleeping segment counts as awake
// (owner ruling R2, spec X3).
func condHolderAsleep(_ map[string]any, ctx *EvalContext) Result {
	if c := itemHolder(ctx); c != nil && actions.TargetAsleep(c) {
		return Success
	}
	return Failure
}

// condWorn: the item is in an equipment slot.
func condWorn(_ map[string]any, ctx *EvalContext) Result {
	if ctx != nil && ctx.Item != nil && ctx.Item.Slot != `` {
		return Success
	}
	return Failure
}

// condInCombat: the holder is in combat.
func condInCombat(_ map[string]any, ctx *EvalContext) Result {
	if c := itemHolder(ctx); c != nil && c.IsInCombat() {
		return Success
	}
	return Failure
}
```

- [ ] **Step 4: The two light actions**

Create `internal/behaviortree/actions_item_light.go`:

```go
package behaviortree

import (
	"slices"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// The light actions (lighting 5e, Rules 9 to 11). Both need an item
// subject (Rule 8).
//
// A WORN item's light lives on its holder's condition records, and these
// write the record's existing trimmed-output state, the same state
// rooms.(*Room).TrimLightFor writes on entry (Rule 9, owner ruling R3): one
// mechanism, no second value. The two writers never share a record: the
// trim owns adjustable records and these refuse them, and the boot refuses a
// light-writing tree on an item whose worn light is adjustable
// (ValidateItemBehaviors). A FIXTURE's output lives in internal/itemlight
// (Rule 10). On any other item both actions fail and change nothing.

// itemLightLevel is a light level an item tree asks for.
type itemLightLevel struct {
	full  bool    // the record's full strength (worn lights only)
	off   bool    // no light at all
	value float64 // a strength on the light scale, when neither full nor off
}

// lightLevelParam reads set_light's level: full, off, or a number at or
// above 0. YAML 1.1 reads an unquoted off as the boolean false, so false
// means off too.
func lightLevelParam(v any) (itemLightLevel, bool) {
	switch x := v.(type) {
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case `full`:
			return itemLightLevel{full: true}, true
		case `off`:
			return itemLightLevel{off: true}, true
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil && f >= 0 {
			return itemLightLevel{value: f}, true
		}
	case bool:
		if !x {
			return itemLightLevel{off: true}, true
		}
	case int:
		if x >= 0 {
			return itemLightLevel{value: float64(x)}, true
		}
	case float64:
		if x >= 0 {
			return itemLightLevel{value: x}, true
		}
	}
	return itemLightLevel{}, false
}

// actSetLight: `set_light` with `level: full | off | <n>`.
func actSetLight(params map[string]any, ctx *EvalContext) Result {
	lv, ok := lightLevelParam(params["level"])
	if !ok {
		return Failure
	}
	return writeItemLight(ctx, lv)
}

// actPulseLight: `pulse_light` with `min`, `max`, `period_rounds` (at
// least 2): the triangle wave of PulseLightValue at this round.
func actPulseLight(params map[string]any, ctx *EvalContext) Result {
	min := getFloatParam(params, "min", -1)
	max := getFloatParam(params, "max", -1)
	period := getIntParam(params, "period_rounds")
	if min < 0 || max < min || period < 2 {
		return Failure
	}
	return writeItemLight(ctx, itemLightLevel{value: PulseLightValue(min, max, period, util.GetRoundCount())})
}

// PulseLightValue is pulse_light's output at a round: a triangle wave from
// min at round 0 up to max at half the period and back to min at the full
// period. Deterministic, no state (Rule 11).
func PulseLightValue(min, max float64, periodRounds int, round uint64) float64 {
	if periodRounds < 2 {
		periodRounds = 2
	}
	phase := float64(round % uint64(periodRounds))
	half := float64(periodRounds) / 2
	frac := phase / half
	if phase > half {
		frac = (float64(periodRounds) - phase) / half
	}
	return min + (max-min)*frac
}

// writeItemLight applies a level to the subject: a fixture on its floor, or
// a worn item's light and darkness records.
func writeItemLight(ctx *EvalContext, lv itemLightLevel) Result {
	if ctx == nil || ctx.Item == nil {
		return Failure
	}
	tmpl := items.GetItemSpec(ctx.Item.ItemId)
	if tmpl == nil {
		return Failure
	}

	if ctx.Item.OnFloor && tmpl.Fixture != `` {
		if lv.full {
			// A fixture has no condition, so no full strength of its own:
			// its tree names a number.
			return Failure
		}
		kind := itemlight.Light
		if tmpl.Fixture == items.FixtureDarkness {
			kind = itemlight.Darkness
		}
		v := lv.value
		if lv.off {
			v = lightscale.Absent()
		}
		itemlight.Set(ctx.Item.RoomId, ctx.Item.UUID, kind, v)
		return Success
	}

	if ctx.Item.Slot == `` {
		return Failure
	}
	holder := itemHolder(ctx)
	if holder == nil {
		return Failure
	}
	worn, ok := wornItemIn(holder, ctx.Item.Slot, ctx.Item.UUID)
	if !ok {
		return Failure
	}
	wrote := false
	for _, id := range worn.GetSpec().WornConditionIds {
		cspec := conditions.GetConditionSpec(id)
		if cspec == nil || !(cspec.IsLightSource() || cspec.IsDarknessSource()) {
			continue
		}
		// The trim owns an adjustable record; this never writes one (X22).
		if slices.Contains(cspec.Flags, conditions.Adjustable) {
			continue
		}
		for _, rec := range holder.Conditions.GetConditions(id) {
			writeRecordLight(rec, lv)
			wrote = true
		}
	}
	if !wrote {
		return Failure
	}
	return Success
}

// wornItemIn returns the item in the holder's slot when it is still the
// subject instance.
func wornItemIn(c *characters.Character, slot string, id uuid.UUID) (items.Item, bool) {
	for _, s := range c.Equipment.AllSlots() {
		if s.Key == slot && s.Item.ItemId > 0 && s.Item.UUID == id {
			return *s.Item, true
		}
	}
	return items.Item{}, false
}

// writeRecordLight writes a level to one record, only when its state
// changes: full is the trim's own full branch (LightFull, output 0), off is
// SetLightOutput(Absent) (LightOff), a number is SetLightOutput(n)
// (LightTrimmed; LightNow caps it at full strength).
func writeRecordLight(rec *conditions.Condition, lv itemLightLevel) {
	switch {
	case lv.full:
		if rec.LightTrim != conditions.LightFull || rec.LightOutput != 0 {
			rec.LightTrim, rec.LightOutput = conditions.LightFull, 0
		}
	case lv.off:
		if rec.LightTrim != conditions.LightOff {
			rec.SetLightOutput(lightscale.Absent())
		}
	default:
		if rec.LightTrim != conditions.LightTrimmed || rec.LightOutput != lv.value {
			rec.SetLightOutput(lv.value)
		}
	}
}
```

- [ ] **Step 5: Register them, extend the allowlist, refuse them outside item trees**

In `internal/behaviortree/conditions.go`, replace

```go
	// Schedule suite (3.2)
	conditionRegistry["mob_at_target_room"] = condMobAtTargetRoom
}
```

with

```go
	// Schedule suite (3.2)
	conditionRegistry["mob_at_target_room"] = condMobAtTargetRoom

	// Item subject (lighting 5e): item trees only (Rule 8)
	conditionRegistry["holder_asleep"] = condHolderAsleep
	conditionRegistry["worn"] = condWorn
	conditionRegistry["in_combat"] = condInCombat
}
```

In `internal/behaviortree/actions.go`, replace

```go
	// Single-target mutation dispatch with engaged-target resolution
	actionRegistry["try_mutation_active_at_target"] = actTryMutationActiveAtTarget
}
```

with

```go
	// Single-target mutation dispatch with engaged-target resolution
	actionRegistry["try_mutation_active_at_target"] = actTryMutationActiveAtTarget

	// Item subject (lighting 5e): item trees only (Rule 8)
	actionRegistry["set_light"] = actSetLight
	actionRegistry["pulse_light"] = actPulseLight
}
```

In `internal/behaviortree/loader.go`, replace the Task 4 allowlist and check

```go
		"state_equals":       true,
		"state_greater_than": true,
	}
	itemSafeActions = map[string]bool{
		"set_state":       true,
		"increment_state": true,
		"decrement_state": true,
	}
)

// checkNodeSubject refuses a node the tree's subject may not name.
func checkNodeSubject(path, kind, name string) error {
	if isItemTreePath(path) {
		safe := itemSafeConditions
		if kind == "action" {
			safe = itemSafeActions
		}
		if !safe[name] {
			return fmt.Errorf("%s: %s %q is not allowed in an item tree (item-safe nodes only)", path, kind, name)
		}
	}
	return nil
}
```

with

```go
		"state_equals":       true,
		"state_greater_than": true,
		"holder_asleep":      true,
		"worn":               true,
		"in_combat":          true,
	}
	itemSafeActions = map[string]bool{
		"set_state":       true,
		"increment_state": true,
		"decrement_state": true,
		"set_light":       true,
		"pulse_light":     true,
	}
	// itemOnlyNodes need an item subject, so a mob or room tree may not
	// name them.
	itemOnlyNodes = map[string]bool{
		"holder_asleep": true,
		"worn":          true,
		"in_combat":     true,
		"set_light":     true,
		"pulse_light":   true,
	}
)

// checkNodeSubject refuses a node the tree's subject may not name.
func checkNodeSubject(path, kind, name string) error {
	if isItemTreePath(path) {
		safe := itemSafeConditions
		if kind == "action" {
			safe = itemSafeActions
		}
		if !safe[name] {
			return fmt.Errorf("%s: %s %q is not allowed in an item tree (item-safe nodes only)", path, kind, name)
		}
		return nil
	}
	if itemOnlyNodes[name] {
		return fmt.Errorf("%s: %s %q needs an item subject and is allowed only in an item tree", path, kind, name)
	}
	return nil
}
```

- [ ] **Step 6: The one-writer boot check (Rule 8, X22)**

In `internal/behaviortree/item_engine.go`, replace the imports

```go
import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/configs"
```

with

```go
import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
```

replace the loop of `ValidateItemBehaviors`

```go
	e := GetEngine()
	loaded := map[string]error{}
	var problems []string
	for _, id := range ids {
		spec := specs[id]
		err, seen := loaded[spec.Behavior]
		if !seen {
			err = e.LoadItemTree(spec.Behavior, GetItemTreePath(spec.Behavior))
			loaded[spec.Behavior] = err
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("item %d (%s): behavior %q: %v", id, spec.Name, spec.Behavior, err))
		}
	}
```

with

```go
	e := GetEngine()
	loaded := map[string]error{}
	writesLight := map[string]bool{}
	var problems []string
	for _, id := range ids {
		spec := specs[id]
		err, seen := loaded[spec.Behavior]
		if !seen {
			path := GetItemTreePath(spec.Behavior)
			err = e.LoadItemTree(spec.Behavior, path)
			loaded[spec.Behavior] = err
			if err == nil {
				if def, derr := LoadTreeDef(path); derr == nil {
					writesLight[spec.Behavior] = TreeWritesLight(def.Tree)
				}
			}
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("item %d (%s): behavior %q: %v", id, spec.Name, spec.Behavior, err))
			continue
		}
		// One writer per record (Rule 8, X22): the trim owns an adjustable
		// light, so no tree may schedule one.
		if writesLight[spec.Behavior] {
			for _, cid := range spec.WornConditionIds {
				cspec := conditions.GetConditionSpec(cid)
				if cspec != nil && (cspec.IsLightSource() || cspec.IsDarknessSource()) &&
					slices.Contains(cspec.Flags, conditions.Adjustable) {
					problems = append(problems, fmt.Sprintf(
						"item %d (%s): behavior %q writes light, but worn condition %d is adjustable: the trim owns it, so no tree may schedule it",
						id, spec.Name, spec.Behavior, cid))
				}
			}
		}
	}
```

and append at the end of the file:

```go

// TreeWritesLight reports whether a tree names set_light or pulse_light
// anywhere. The repo-root content guards read it too.
func TreeWritesLight(def NodeDef) bool {
	if def.Do == `set_light` || def.Do == `pulse_light` {
		return true
	}
	for _, ch := range def.Children {
		if TreeWritesLight(ch) {
			return true
		}
	}
	return def.Child != nil && TreeWritesLight(*def.Child)
}
```

- [ ] **Step 7: Run them to see them pass**

Run: `go test ./internal/behaviortree/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/behaviortree`.

- [ ] **Step 8: Prove the trim test able to fail**

In `seedItemLightWorld`, add `, Flags: []conditions.Flag{conditions.Adjustable}` to the `lightProbeLanternCond` spec (after its `Effects` map) and run `go test ./internal/behaviortree/ -run TrimOnEntry -count=1`.
Expected: FAIL `after an entry trim: LightTrim="", want still off` (an adjustable lantern is the trim's to write, so `set_light` leaves it and the entry trim runs it at full). Restore the line and re-run: `ok`.

- [ ] **Step 9: Commit**

```bash
git add internal/behaviortree/conditions_item.go internal/behaviortree/actions_item_light.go internal/behaviortree/item_light_test.go internal/behaviortree/conditions.go internal/behaviortree/actions.go internal/behaviortree/loader.go internal/behaviortree/item_engine.go
git commit -m "feat(behaviortree): item nodes and scheduled light through the record's trim state (5e Rules 7 to 9, 11)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: The holder index wired: mobs and rooms enter and leave it (Rule 5, F11, F12)

**Model:** sonnet (five packages, load and unload paths).

**Files:**
- Create: `internal/characters/item_behaviour_test.go`, `internal/characters/item_behaviour.go`
- Modify: `internal/characters/inventory.go:178`, `internal/characters/worn.go:598-603,626,666`
- Create: `internal/mobs/item_behaviour_index_test.go`; Modify: `internal/mobs/mobs.go:782-786`
- Modify: `internal/hooks/PlayerSpawn_HandleJoin.go:166-168`
- Create: `internal/rooms/item_behaviour_test.go`, `internal/rooms/item_behaviour.go`
- Modify: `internal/rooms/rooms.go:10-30` (imports), `:1192`, `:1343-1347`, `:1507-1514`; `internal/rooms/roommanager.go:14-23` (imports), `:649-651`, `:696-698`
- Modify: `main.go` (after Task 4's boot check)

- [ ] **Step 1: Write the failing tests**

Create `internal/characters/item_behaviour_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

const (
	treedLanternItem = 999950
	plainRockItem    = 999951
)

func seedTreedItems(t *testing.T) {
	t.Helper()
	t.Cleanup(seedMediumSpecies())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		treedLanternItem: {ItemId: treedLanternItem, Name: "test keeper lantern", Type: items.Light, Subtype: items.Wearable,
			Behavior: "keeper_lantern"},
		plainRockItem: {ItemId: plainRockItem, Name: "test rock", Type: items.Object},
	}))
	t.Cleanup(items.ResetHolderIndexForTest())
}

func indexedMobs() map[int]bool {
	out := map[int]bool{}
	for _, id := range items.MobHolders() {
		out[id] = true
	}
	return out
}

// Rule 5: a mob joins the item tick's index when it stores or wears a
// treed item, and through IndexTreedItems at spawn. A player never does
// (the tick walks every online player), and an untreed item indexes nobody.
func TestAMobHandedATreedItemJoinsTheIndex(t *testing.T) {
	seedTreedItems(t)

	stored := New()
	stored.MobInstanceId = 41
	stored.Stats.Strength.ValueAdj = 100
	stored.StoreItem(items.New(treedLanternItem))

	worn := New()
	worn.MobInstanceId = 42
	worn.Stats.Strength.ValueAdj = 100
	if _, ok, why := worn.Wear(items.New(treedLanternItem)); !ok {
		t.Fatalf("could not wear the lantern: %s", why)
	}

	spawned := New()
	spawned.MobInstanceId = 43
	spawned.Equipment.Light = items.New(treedLanternItem)
	spawned.IndexTreedItems()

	rock := New()
	rock.MobInstanceId = 44
	rock.Stats.Strength.ValueAdj = 100
	rock.StoreItem(items.New(plainRockItem))
	rock.IndexTreedItems()

	player := New()
	player.Stats.Strength.ValueAdj = 100
	player.StoreItem(items.New(treedLanternItem))

	got := indexedMobs()
	for _, id := range []int{41, 42, 43} {
		if !got[id] {
			t.Errorf("mob %d holds a treed item but is not indexed (index %v)", id, got)
		}
	}
	if got[44] || len(got) != 3 {
		t.Errorf("index %v, want exactly mobs 41, 42 and 43", got)
	}
}
```

Create `internal/mobs/item_behaviour_index_test.go`:

```go
package mobs

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Rule 5: a mob spawned holding a treed item (a keeper's lantern in the
// light slot) joins the item tick's holder index; one with none does not.
func TestSpawnIndexesAMobHoldingATreedItem(t *testing.T) {
	t.Cleanup(seedRegistry())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		999970: {ItemId: 999970, Name: "test keeper lantern", Type: items.Light, Subtype: items.Wearable,
			Behavior: "keeper_lantern"},
	}))
	t.Cleanup(items.ResetHolderIndexForTest())

	mobs[1].Character.Equipment.Light = items.Item{ItemId: 999970}
	t.Cleanup(func() { mobs[1].Character.Equipment.Light = items.Item{} })

	keeper := NewMobByIdFresh(MobId(1), 4242)
	if keeper == nil {
		t.Fatal("spawn returned nil")
	}
	defer DestroyInstance(keeper.InstanceId)
	plain := NewMobByIdFresh(MobId(2), 4242)
	if plain == nil {
		t.Fatal("spawn returned nil")
	}
	defer DestroyInstance(plain.InstanceId)

	got := items.MobHolders()
	if len(got) != 1 || got[0] != keeper.InstanceId {
		t.Errorf("MobHolders = %v, want only the keeper %d", got, keeper.InstanceId)
	}
}
```

(`t.Cleanup(seedRegistry())` is registered first so it runs last: the slot reset above it still finds the seeded template.)

Create `internal/rooms/item_behaviour_test.go`:

```go
package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
)

const (
	floorTreedItem = 999960
	floorPlainItem = 999961
)

// seedFloorIndex seeds one treed and one plain item and an empty index, and
// records every room the index reports entering.
func seedFloorIndex(t *testing.T) *[]int {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		floorTreedItem: {ItemId: floorTreedItem, Name: "test arch lantern", Type: items.Object,
			Fixture: items.FixtureLight, Behavior: "dusk_to_dawn"},
		floorPlainItem: {ItemId: floorPlainItem, Name: "test pebble", Type: items.Object},
	}))
	t.Cleanup(items.ResetHolderIndexForTest())
	t.Cleanup(itemlight.ResetForTest())
	var entered []int
	prev := items.OnRoomHolderIndexed
	items.OnRoomHolderIndexed = func(roomId int) { entered = append(entered, roomId) }
	t.Cleanup(func() { items.OnRoomHolderIndexed = prev })
	return &entered
}

// Rule 5: a room joins the index when a treed item lands on its floor, by
// AddItem or by Prepare's spawn append; a plain item or a stash does not
// index it.
func TestATreedFloorItemIndexesItsRoom(t *testing.T) {
	entered := seedFloorIndex(t)

	plain := &Room{RoomId: 7801}
	plain.AddItem(items.New(floorPlainItem), false)
	stashed := &Room{RoomId: 7802}
	stashed.AddItem(items.New(floorTreedItem), true)
	added := &Room{RoomId: 7803}
	added.AddItem(items.New(floorTreedItem), false)
	spawned := &Room{RoomId: 7804, SpawnInfo: []SpawnInfo{{ItemId: floorTreedItem}}}
	spawned.Prepare(false)

	if got := items.RoomHolders(); len(got) != 2 || got[0] != 7803 || got[1] != 7804 {
		t.Errorf("RoomHolders = %v, want [7803 7804]", got)
	}
	if len(*entered) != 2 {
		t.Errorf("OnRoomHolderIndexed saw %v, want the two rooms once each", *entered)
	}
}

// A room loading into memory with a treed item already on its floor (kept
// by its instance file) joins the index, and IndexTreedFloors catches a room
// loaded before item specs existed.
func TestALoadedRoomHoldingATreedItemIsIndexed(t *testing.T) {
	seedFloorIndex(t)
	t.Cleanup(SeedRoomsForTest(map[int]*Room{}, map[string]*ZoneConfig{}))

	loaded := &Room{RoomId: 7811, Zone: "probe", Items: []items.Item{items.New(floorTreedItem)}}
	if err := addRoomToMemory(loaded); err != nil {
		t.Fatal(err)
	}
	if got := items.RoomHolders(); len(got) != 1 || got[0] != 7811 {
		t.Fatalf("after addRoomToMemory RoomHolders = %v, want [7811]", got)
	}

	early := &Room{RoomId: 7812, Zone: "probe", Items: []items.Item{items.New(floorTreedItem)}}
	roomManager.rooms[early.RoomId] = early // in memory, never indexed
	IndexTreedFloors()
	if got := items.RoomHolders(); len(got) != 2 || got[1] != 7812 {
		t.Errorf("after IndexTreedFloors RoomHolders = %v, want [7811 7812]", got)
	}
}

// Rule 10: an item taken off the floor stops lighting the room.
func TestRemoveItemDropsAFixturesOutput(t *testing.T) {
	seedFloorIndex(t)
	r := &Room{RoomId: 7821}
	lamp := items.New(floorTreedItem)
	r.AddItem(lamp, false)
	itemlight.Set(r.RoomId, lamp.UUID, itemlight.Light, 52)
	r.RemoveItem(lamp, false)
	if _, ok := itemlight.Get(r.RoomId, lamp.UUID); ok {
		t.Error("the removed fixture still has an output")
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/characters/ ./internal/mobs/ ./internal/rooms/ -run 'TreedItem|TreedFloorItem|LoadedRoomHolding|RemoveItemDrops|SpawnIndexes' -count=1`
Expected: `characters` and `rooms` FAIL to build (`IndexTreedItems undefined`, `undefined: IndexTreedFloors`); `mobs` builds and FAILS: `MobHolders = [], want only the keeper` (nothing indexes a spawn yet).

- [ ] **Step 3: The character side**

Create `internal/characters/item_behaviour.go`:

```go
package characters

import "github.com/GoMudEngine/GoMud/internal/items"

// IndexTreedItem enters this character's mob in the item tick's holder
// index when the item names a behaviour tree (lighting 5e, Rule 5). A player
// is never indexed: the tick walks every online player anyway. Called from
// every path that hands a mob an item: StoreItem, Wear, the spills in
// RemoveFromBody, and the direct appends outside this package.
func (c *Character) IndexTreedItem(i items.Item) {
	if c.MobInstanceId > 0 && i.HasBehavior() {
		items.IndexMobHolder(c.MobInstanceId)
	}
}

// IndexTreedItems enters this character's mob in the holder index when any
// worn or backpack item names a behaviour tree. Mob spawn and the companion
// gear restore call it once the mob's items are in place.
func (c *Character) IndexTreedItems() {
	if c.MobInstanceId <= 0 {
		return
	}
	for _, s := range c.Equipment.AllSlots() {
		if s.Item.ItemId > 0 && s.Item.HasBehavior() {
			items.IndexMobHolder(c.MobInstanceId)
			return
		}
	}
	for _, it := range c.Items {
		if it.HasBehavior() {
			items.IndexMobHolder(c.MobInstanceId)
			return
		}
	}
}
```

In `internal/characters/inventory.go` (`StoreItem`), replace

```go
	// A found bauble someone carries is no longer lying anywhere: it shows no
	// spot, belongs to no household and never vanishes (items/bauble_placement.go).
	i.ClearBaublePlacement()
```

with

```go
	// A found bauble someone carries is no longer lying anywhere: it shows no
	// spot, belongs to no household and never vanishes (items/bauble_placement.go).
	i.ClearBaublePlacement()

	// A mob given a treed item joins the item tick (lighting 5e). Harmless
	// when the store below refuses: the tick drops a holder with nothing.
	c.IndexTreedItem(i)
```

In `internal/characters/worn.go`, replace (end of `wear`)

```go
				rec.ResetLight()
			}
		}
	}
	return returnItems, newItemWorn, failureReason
}
```

with

```go
				rec.ResetLight()
			}
		}
	}
	c.IndexTreedItem(i)
	return returnItems, newItemWorn, failureReason
}
```

replace (the bandolier spill in `RemoveFromBody`)

```go
			for _, pi := range c.PotionItems {
				c.Items = append(c.Items, pi)
			}
```

with

```go
			for _, pi := range c.PotionItems {
				c.Items = append(c.Items, pi)
				c.IndexTreedItem(pi)
			}
```

and replace (the component-bag spill)

```go
		for _, ci := range c.ComponentItems {
			c.Items = append(c.Items, ci)
		}
```

with

```go
		for _, ci := range c.ComponentItems {
			c.Items = append(c.Items, ci)
			c.IndexTreedItem(ci)
		}
```

- [ ] **Step 4: Spawn and the companion restore**

In `internal/mobs/mobs.go` (end of `newMobByIdInternal`), replace

```go
		// Save the mob instance
		mobInstancesMu.Lock()
		mobInstances[mob.InstanceId] = &mob
		mobInstancesMu.Unlock()

		return &mob
```

with

```go
		// Save the mob instance
		mobInstancesMu.Lock()
		mobInstances[mob.InstanceId] = &mob
		mobInstancesMu.Unlock()

		// A mob spawned holding a treed item (a keeper's lantern) joins the
		// item tick (lighting 5e), whichever of template or saved instance
		// supplied the item.
		mob.Character.IndexTreedItems()

		return &mob
```

In `internal/hooks/PlayerSpawn_HandleJoin.go`, replace

```go
	if compHasEquipment(comp) {
		mob.Character.Equipment = comp.Equipment
	}
```

with

```go
	if compHasEquipment(comp) {
		mob.Character.Equipment = comp.Equipment
	}
	// The restored gear may hold a treed item (lighting 5e).
	mob.Character.IndexTreedItems()
```

- [ ] **Step 5: The room side**

Create `internal/rooms/item_behaviour.go`:

```go
package rooms

import "github.com/GoMudEngine/GoMud/internal/items"

// A room's floor joins the item tick's holder index (lighting 5e, Rule 5)
// when a treed item lands on it (AddItem, Prepare's spawn append) and when
// the room loads into memory holding one (addRoomToMemory). It leaves when
// it unloads (removeRoomFromMemory) or when the tick finds nothing treed
// left on it. Stashed items are never visited, so a stash does not index.

// noteTreedFloorItem enters this room in the index when the item names a
// behaviour tree.
func (r *Room) noteTreedFloorItem(i items.Item) {
	if i.HasBehavior() {
		items.IndexRoomHolder(r.RoomId)
	}
}

// indexTreedFloor enters this room in the index when any floor item names a
// behaviour tree.
func (r *Room) indexTreedFloor() {
	for _, it := range r.Items {
		if it.HasBehavior() {
			items.IndexRoomHolder(r.RoomId)
			return
		}
	}
}

// IndexTreedFloors enters every room already in memory that holds a treed
// floor item. main.go calls it once, after item specs load: a room loaded
// before then (a faction holding cell) could not tell which items are
// treed.
func IndexTreedFloors() {
	for _, r := range roomManager.rooms {
		r.indexTreedFloor()
	}
}
```

In `internal/rooms/rooms.go`, add `"github.com/GoMudEngine/GoMud/internal/itemlight"` to the imports (directly above `"github.com/GoMudEngine/GoMud/internal/items"`). Replace (`Prepare`'s spawn append)

```go
					if item := items.New(spawnInfo.ItemId); item.ItemId != 0 {
						r.Items = append(r.Items, item) // just append to avoid a mutex double lock
					}
```

with

```go
					if item := items.New(spawnInfo.ItemId); item.ItemId != 0 {
						r.Items = append(r.Items, item) // just append to avoid a mutex double lock
						r.noteTreedFloorItem(item)
					}
```

replace (`AddItem`)

```go
	if stash {
		r.Stash = append(r.Stash, item)
	} else {
		r.Items = append(r.Items, item)
	}

}
```

with

```go
	if stash {
		r.Stash = append(r.Stash, item)
	} else {
		r.Items = append(r.Items, item)
		r.noteTreedFloorItem(item)
	}

}
```

and replace (the floor branch of `RemoveItem`)

```go
		for j := len(r.Items) - 1; j >= 0; j-- {
			if r.Items[j].Equals(i) {
				r.Items = append(r.Items[:j], r.Items[j+1:]...)
				break
			}
		}
	}

}
```

with

```go
		for j := len(r.Items) - 1; j >= 0; j-- {
			if r.Items[j].Equals(i) {
				r.Items = append(r.Items[:j], r.Items[j+1:]...)
				break
			}
		}
		// An item off the floor stops lighting the room (lighting 5e,
		// Rule 10): a no-op for anything that is not a lit fixture.
		itemlight.Clear(r.RoomId, i.UUID)
	}

}
```

In `internal/rooms/roommanager.go`, add `"github.com/GoMudEngine/GoMud/internal/itemlight"` and `"github.com/GoMudEngine/GoMud/internal/items"` to the imports (directly after `"github.com/GoMudEngine/GoMud/internal/fileloader"`). Replace (end of `removeRoomFromMemory`)

```go
	SaveRoomInstance(*room)

	delete(roomManager.rooms, r.RoomId)
}
```

with

```go
	SaveRoomInstance(*room)

	// Its fixtures stop lighting it and the item tick stops visiting it
	// (lighting 5e); a reload mints fresh item UUIDs and re-indexes.
	itemlight.ClearRoom(r.RoomId)
	items.DropRoomHolder(r.RoomId)

	delete(roomManager.rooms, r.RoomId)
}
```

and replace (end of `addRoomToMemory`)

```go
	// Populate the room present lookup in the zone info
	zoneInfo.RoomIds[room.RoomId] = struct{}{}

	roomManager.zones[room.Zone] = zoneInfo

	return nil
}
```

with

```go
	// Populate the room present lookup in the zone info
	zoneInfo.RoomIds[room.RoomId] = struct{}{}

	roomManager.zones[room.Zone] = zoneInfo

	// A room loading with a treed item on its floor (a fixture kept by its
	// instance file) joins the item tick, and its fixtures are evaluated at
	// once (lighting 5e). Last, so the room is already findable.
	room.indexTreedFloor()

	return nil
}
```

- [ ] **Step 6: Rooms loaded before item specs (F12)**

In `main.go`, replace (Task 4's boot check)

```go
	if err := behaviortree.ValidateItemBehaviors(); err != nil {
		panic(err)
	}
```

with

```go
	if err := behaviortree.ValidateItemBehaviors(); err != nil {
		panic(err)
	}
	// Rooms loaded before item specs (faction holding cells, above) could
	// not tell a treed floor item; index them now.
	rooms.IndexTreedFloors()
```

- [ ] **Step 7: Run them to see them pass**

Run: `go build ./... && go test ./internal/characters/ ./internal/mobs/ ./internal/rooms/ ./internal/hooks/ -count=1`
Expected: build silent; four `ok` lines.

- [ ] **Step 8: Prove the room test able to fail**

Delete the `r.noteTreedFloorItem(item)` line from `AddItem` (not the one in `Prepare`) and run `go test ./internal/rooms/ -run TreedFloorItem -count=1`.
Expected: FAIL `OnRoomHolderIndexed saw [7804], want the two rooms once each`. Restore the line; re-run: `ok`.

- [ ] **Step 9: Commit**

```bash
git add internal/characters/item_behaviour.go internal/characters/item_behaviour_test.go internal/characters/inventory.go internal/characters/worn.go internal/mobs/mobs.go internal/mobs/item_behaviour_index_test.go internal/hooks/PlayerSpawn_HandleJoin.go internal/rooms/item_behaviour.go internal/rooms/item_behaviour_test.go internal/rooms/rooms.go internal/rooms/roommanager.go main.go
git commit -m "feat(items): mobs and rooms enter and leave the item tick's holder index (5e Rule 5)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: The item tick and `item_idle` (Rules 4 to 6)

**Model:** sonnet (the tick's order, eviction and the first-visit callback).

**Files:**
- Create: `internal/hooks/NewRound_ItemRoundTick_test.go`, `internal/hooks/NewRound_ItemRoundTick.go`
- Modify: `internal/hooks/hooks.go:3-10` (imports), `:66`; `internal/behaviortree/events.go:17`

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/NewRound_ItemRoundTick_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

const (
	tickTreedItem   = 999980
	tickFixtureItem = 999981
	tickPlainItem   = 999982
	tickRoomA       = 9980
	tickRoomB       = 9981
	tickMobInst     = 9982
)

// tickCountingTree counts item_idle visits in the item's own state.
const tickCountingTree = "tree:\n  type: action\n  event: item_idle\n  do: increment_state\n  key: visits\n"

// tickFixtureTree lights a fixture at 52 on every visit.
const tickFixtureTree = "tree:\n  type: action\n  event: item_idle\n  do: set_light\n  level: 52\n"

type tickWorld struct {
	user    *users.UserRecord
	mob     *mobs.Mob
	roomA   *rooms.Room
	roomB   *rooms.Room
	worn    items.Item
	pack    items.Item
	mobHeld items.Item
	floor   items.Item
	fixture items.Item
}

// seedTickWorld: user 1 wears a treed item and carries one; mob 9982
// carries one; room A has a treed item on the floor and room B a fixture;
// both rooms are loaded and every holder is indexed.
func seedTickWorld(t *testing.T) *tickWorld {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		tickTreedItem:   {ItemId: tickTreedItem, Name: "test humming ring", Type: items.Ring, Subtype: items.Wearable, Behavior: "tick_count"},
		tickFixtureItem: {ItemId: tickFixtureItem, Name: "test lamp post", Type: items.Object, Fixture: items.FixtureLight, Behavior: "tick_fixture"},
		tickPlainItem:   {ItemId: tickPlainItem, Name: "test pebble", Type: items.Object},
	}))
	behaviortree.LoadItemTreeForTest(t, "tick_count", tickCountingTree)
	behaviortree.LoadItemTreeForTest(t, "tick_fixture", tickFixtureTree)
	t.Cleanup(items.ResetHolderIndexForTest())
	t.Cleanup(itemlight.ResetForTest())
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	prevHook := items.OnRoomHolderIndexed
	items.OnRoomHolderIndexed = nil
	t.Cleanup(func() { items.OnRoomHolderIndexed = prevHook })

	w := &tickWorld{
		worn: items.New(tickTreedItem), pack: items.New(tickTreedItem), mobHeld: items.New(tickTreedItem),
		floor: items.New(tickTreedItem), fixture: items.New(tickFixtureItem),
	}
	w.roomA = rooms.NewRoom("probe")
	w.roomA.RoomId = tickRoomA
	w.roomB = rooms.NewRoom("probe")
	w.roomB.RoomId = tickRoomB
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{tickRoomA: w.roomA, tickRoomB: w.roomB}, map[string]*rooms.ZoneConfig{}))

	w.user = users.NewTestUser(1, "ticker", "Ticker", 0)
	w.user.Character.RoomId = tickRoomA
	w.user.Character.Equipment.Ring = w.worn
	w.user.Character.Items = []items.Item{w.pack, items.New(tickPlainItem)}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: w.user}))

	w.mob = &mobs.Mob{MobId: 1, InstanceId: tickMobInst, HomeRoomId: tickRoomA,
		Character: characters.Character{Name: "Tick Keeper", RoomId: tickRoomA, MobInstanceId: tickMobInst,
			Conditions: conditions.New(), Items: []items.Item{w.mobHeld}}}
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{1: w.mob}, map[int]*mobs.Mob{tickMobInst: w.mob}))

	w.roomA.AddItem(w.floor, false)
	w.roomB.AddItem(w.fixture, false)
	w.mob.Character.IndexTreedItems()

	prevRound := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCountForTest(prevRound) })
	util.SetRoundCountForTest(5000)
	return w
}

func visits(t *testing.T, it items.Item) int {
	t.Helper()
	s := behaviortree.ItemBTreeStateForTest(it.UUID)
	if s == nil {
		return 0
	}
	return s.GetInt("visits")
}

// Rule 5: one tick fires item_idle once for every treed item it reaches:
// a player's worn and backpack items, an indexed mob's, an indexed room's
// floor. A fixture's tree lights it.
func TestItemRoundTickVisitsEveryTreedItemOnce(t *testing.T) {
	w := seedTickWorld(t)
	ItemRoundTick(events.NewRound{})
	for name, it := range map[string]items.Item{"worn": w.worn, "backpack": w.pack, "mob-held": w.mobHeld, "floor": w.floor} {
		if got := visits(t, it); got != 1 {
			t.Errorf("%s item visited %d times, want 1", name, got)
		}
	}
	if v, _ := itemlight.Get(tickRoomB, w.fixture.UUID); v != 52 {
		t.Errorf("the fixture reads %v after a tick, want 52", v)
	}
}

// Rule 5: a holder with nothing treed left, a gone mob and an unloaded room
// drop out of the index; a room that unloads loses its fixtures' output.
func TestItemRoundTickDropsHoldersWithNothingLeft(t *testing.T) {
	w := seedTickWorld(t)
	items.IndexMobHolder(424242) // a mob that no longer exists
	w.roomA.RemoveItem(w.floor, false)
	ItemRoundTick(events.NewRound{})
	if got := items.MobHolders(); len(got) != 1 || got[0] != tickMobInst {
		t.Errorf("MobHolders = %v, want only %d", got, tickMobInst)
	}
	if got := items.RoomHolders(); len(got) != 1 || got[0] != tickRoomB {
		t.Errorf("RoomHolders = %v, want only %d", got, tickRoomB)
	}

	w.mob.Character.Items = nil
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{}, map[string]*rooms.ZoneConfig{})) // room B unloads
	ItemRoundTick(events.NewRound{})
	if got := items.MobHolders(); len(got) != 0 {
		t.Errorf("a mob holding nothing treed is still indexed: %v", got)
	}
	if got := items.RoomHolders(); len(got) != 0 {
		t.Errorf("an unloaded room is still indexed: %v", got)
	}
	if light, _ := itemlight.Terms(tickRoomB); len(light) != 0 {
		t.Errorf("an unloaded room's fixture still lights it: %v", light)
	}
}

// Rule 4: an item's state is evicted once a round passes without a visit,
// and an item handed from a mob to a player keeps its state.
func TestItemStateFollowsTheItem(t *testing.T) {
	w := seedTickWorld(t)
	ItemRoundTick(events.NewRound{})

	// The mob hands its item to the player between rounds.
	w.user.Character.Items = append(w.user.Character.Items, w.mobHeld)
	w.mob.Character.Items = nil
	// The floor item leaves for somewhere the tick does not reach.
	w.roomA.RemoveItem(w.floor, false)

	util.SetRoundCountForTest(5001)
	ItemRoundTick(events.NewRound{})
	if got := visits(t, w.mobHeld); got != 2 {
		t.Errorf("the handed item's visits = %d, want 2: its state followed it", got)
	}
	if s := behaviortree.ItemBTreeStateForTest(w.floor.UUID); s != nil {
		t.Error("an item the tick no longer reaches kept its state")
	}
}

// Rule 5: a fixture is evaluated the moment its room joins the index, so a
// first visit is never dark for a round.
func TestAFixtureIsLitTheMomentItsRoomIsIndexed(t *testing.T) {
	w := seedTickWorld(t)
	items.OnRoomHolderIndexed = EvaluateRoomFixtures
	r := rooms.NewRoom("probe")
	r.RoomId = 9983
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{tickRoomA: w.roomA, tickRoomB: w.roomB, 9983: r}, map[string]*rooms.ZoneConfig{}))
	post := items.New(tickFixtureItem)
	r.AddItem(post, false)
	if v, _ := itemlight.Get(9983, post.UUID); v != 52 {
		t.Errorf("before any tick the new fixture reads %v, want 52", v)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/hooks/ -run 'ItemRoundTick|ItemStateFollows|FixtureIsLit' -count=1`
Expected: FAIL to build: `undefined: ItemRoundTick`, `undefined: EvaluateRoomFixtures`.

- [ ] **Step 3: The tick**

Create `internal/hooks/NewRound_ItemRoundTick.go`:

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// ItemRoundTick fires item_idle once a round for every item with a behaviour
// tree that it reaches (lighting 5e, item behaviour slice 1, Rule 5), in this
// order: every online player's worn slots then backpack; the indexed mobs'
// worn slots then backpack; the indexed rooms' floors. Never a world walk:
// players are bounded by who is online (the Pinnacle tick's precedent), and
// mobs and rooms come from the holder index (internal/items). A visited
// holder with nothing treed left drops out of the index. Container contents
// are not visited. Last, the state of every item not visited this round is
// evicted (Rule 4).
func ItemRoundTick(e events.Event) events.ListenerReturn {
	round := util.GetRoundCount()

	for _, user := range users.GetAllActiveUsers() {
		if user.Character != nil {
			tickHeldItems(user.Character, user.UserId, 0)
		}
	}
	for _, id := range items.MobHolders() {
		m := mobs.GetInstance(id)
		if m == nil || !tickHeldItems(&m.Character, 0, id) {
			items.DropMobHolder(id)
		}
	}
	for _, roomId := range items.RoomHolders() {
		if !tickFloorItems(roomId) {
			items.DropRoomHolder(roomId)
		}
	}

	behaviortree.EvictUnseenItemBTreeStates(round)
	return events.Continue
}

// EvaluateRoomFixtures runs the trees of every treed item on a room's floor
// now. RegisterListeners sets it as items.OnRoomHolderIndexed, so a room
// that joins the index (a fixture spawned, or a room loaded holding one) is
// lit before anyone reads it.
func EvaluateRoomFixtures(roomId int) {
	tickFloorItems(roomId)
}

func itemIdleEvent() behaviortree.EventContext {
	return behaviortree.EventContext{EventType: "item_idle"}
}

// tickHeldItems runs item_idle for a character's treed worn and backpack
// items, and reports whether it found any.
func tickHeldItems(c *characters.Character, userId, mobInstanceId int) bool {
	found := false
	for _, s := range c.Equipment.AllSlots() {
		if s.Item.ItemId <= 0 || !s.Item.HasBehavior() {
			continue
		}
		found = true
		behaviortree.TryItemBehavior(itemIdleEvent(), behaviortree.ItemSubject{
			UUID: s.Item.UUID, ItemId: s.Item.ItemId, UserId: userId, MobInstanceId: mobInstanceId,
			RoomId: c.RoomId, Slot: s.Key,
		})
	}
	for _, it := range c.GetAllBackpackItems() {
		if !it.HasBehavior() {
			continue
		}
		found = true
		behaviortree.TryItemBehavior(itemIdleEvent(), behaviortree.ItemSubject{
			UUID: it.UUID, ItemId: it.ItemId, UserId: userId, MobInstanceId: mobInstanceId,
			RoomId: c.RoomId,
		})
	}
	return found
}

// tickFloorItems runs item_idle for a loaded room's treed floor items, drops
// any fixture output whose item is no longer on the floor, and reports
// whether it found any treed item. An unloaded room loses its outputs and
// reports none.
func tickFloorItems(roomId int) bool {
	if !rooms.IsRoomLoaded(roomId) {
		itemlight.ClearRoom(roomId)
		return false
	}
	r := rooms.LoadRoom(roomId)
	if r == nil {
		return false
	}
	keep := map[uuid.UUID]bool{}
	for _, it := range append([]items.Item(nil), r.Items...) {
		if !it.HasBehavior() {
			continue
		}
		keep[it.UUID] = true
		behaviortree.TryItemBehavior(itemIdleEvent(), behaviortree.ItemSubject{
			UUID: it.UUID, ItemId: it.ItemId, RoomId: roomId, OnFloor: true,
		})
	}
	itemlight.Retain(roomId, keep)
	return len(keep) > 0
}
```

- [ ] **Step 4: Run them to see them pass, and the vocabulary gate fail**

Run: `go test ./internal/hooks/ -run 'ItemRoundTick|ItemStateFollows|FixtureIsLit' -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/hooks`.

Run: `go test ./internal/behaviortree/ -run TestEventVocabulary -count=1`
Expected: FAIL `event "item_idle" is fired by the engine but missing from KnownBehaviorEvents — add it (the editor would refuse it)` (the guard's own text): the tick is now a dispatch site.

- [ ] **Step 5: Register the tick and the event**

In `internal/behaviortree/events.go`, replace

```go
	"heard_callforhelp":      true, // a routine-matched packmate in an adjacent room called for help
```

with

```go
	"heard_callforhelp":      true, // a routine-matched packmate in an adjacent room called for help
	"item_idle":              true, // item trees: once a round, from the item tick (lighting 5e)
```

In `internal/hooks/hooks.go`, add `"github.com/GoMudEngine/GoMud/internal/items"` to the imports (after `forager`), and replace

```go
	events.RegisterListener(events.NewRound{}, IdleMobs)
```

with

```go
	events.RegisterListener(events.NewRound{}, IdleMobs)
	// Lighting 5e: item behaviour trees (worn, carried, mob-held, floor), and
	// a room's fixtures lit the moment the room joins the item index.
	events.RegisterListener(events.NewRound{}, ItemRoundTick)
	items.OnRoomHolderIndexed = EvaluateRoomFixtures
```

- [ ] **Step 6: Run the packages**

Run: `gofmt -l internal/ && go build ./... && go test ./internal/behaviortree/ ./internal/hooks/ -count=1`
Expected: gofmt silent; `ok` for both.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/NewRound_ItemRoundTick.go internal/hooks/NewRound_ItemRoundTick_test.go internal/hooks/hooks.go internal/behaviortree/events.go
git commit -m "feat(hooks): the item tick fires item_idle over players and the holder index (5e Rules 4 to 6)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Fixtures in the room's light: `LightTerms.Fixture` and `CarriedLight` (Rule 10, X5)

**Model:** haiku (mechanical, code given).

**Files:**
- Create: `internal/rooms/fixture_compose_test.go`
- Modify: `internal/rooms/lighting.go:3-13` (imports), `:86-90` (`LightTerms` tail), `:106-126` (`composeLightExcluding`, `composeWith`), `:142-148`, `:159-165`

- [ ] **Step 1: Write the failing tests**

Create `internal/rooms/fixture_compose_test.go`:

```go
package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// Rule 10 and X5: a lit fixture is one term in the light combine, a darkness
// fixture one in the darkness combine; LightTerms.Fixture reports the light
// fixtures' combine and never sets Carried; CarriedLight is the carried
// light alone.
func TestFixtureComposition(t *testing.T) {
	cfg := modelCfg()
	zero := 0.0
	cave := Room{SkyLight: &zero}
	tavern := Room{SkyLight: &zero, Lamp: LampPtr(38)}

	cases := []struct {
		name             string
		room             Room
		carried, dark    []float64
		fxLight, fxDark  []float64
		want             int
		wantFixture      float64 // Absent when none
		wantCarriedLight float64 // Absent when none
	}{
		{"a fixture alone", cave, nil, nil, []float64{52}, nil, 52, 52, lightscale.Absent()},
		{"5000: lamp 38, stones at their trough", tavern, nil, nil, []float64{20}, nil, 40, 20, lightscale.Absent()},
		{"5000: lamp 38, stones at their crest", tavern, nil, nil, []float64{36}, nil, 45, 36, lightscale.Absent()},
		{"a fixture and a carried torch", cave, []float64{56}, nil, []float64{52}, nil, 62, 52, 56},
		{"two fixtures", cave, nil, nil, []float64{52, 52}, nil, 60, 60, lightscale.Absent()},
		{"a darkness fixture under a lamp", tavern, nil, nil, nil, []float64{30}, 8, lightscale.Absent(), lightscale.Absent()},
		{"a darkness fixture and a carried darkness", cave, nil, []float64{50}, nil, []float64{50}, -58, lightscale.Absent(), lightscale.Absent()},
	}
	for _, c := range cases {
		got := c.room.composeWithFixtures(cfg, 60, 1, c.carried, c.dark, c.fxLight, c.fxDark)
		if got.Level != c.want {
			t.Errorf("%s: Level = %d, want %d", c.name, got.Level, c.want)
		}
		if !sameTerm(got.Fixture, c.wantFixture) {
			t.Errorf("%s: Fixture = %v, want %v", c.name, got.Fixture, c.wantFixture)
		}
		if !sameTerm(got.CarriedLight, c.wantCarriedLight) {
			t.Errorf("%s: CarriedLight = %v, want %v", c.name, got.CarriedLight, c.wantCarriedLight)
		}
		if got.Carried != (len(c.carried) > 0) {
			t.Errorf("%s: Carried = %v, want %v: a fixture is never a carried light", c.name, got.Carried, len(c.carried) > 0)
		}
	}
}

// composeWith is composeWithFixtures with no fixtures: every existing
// caller and test reads the same terms as before.
func TestComposeWithIsFixtureFree(t *testing.T) {
	cfg := modelCfg()
	zero := 0.0
	cave := Room{SkyLight: &zero, Lamp: LampPtr(50)}
	a := cave.composeWith(cfg, 60, 1, []float64{56}, []float64{20})
	b := cave.composeWithFixtures(cfg, 60, 1, []float64{56}, []float64{20}, nil, nil)
	if a != b {
		t.Errorf("composeWith %+v != composeWithFixtures with none %+v", a, b)
	}
}

// LightLevel reads a room's fixtures from internal/itemlight by its id.
func TestLightLevelReadsTheRoomsFixtures(t *testing.T) {
	t.Cleanup(itemlight.ResetForTest())
	zero := 0.0
	cave := &Room{RoomId: 7831, SkyLight: &zero}
	before := cave.LightTerms()
	itemlight.Set(7831, uuid.UUID{1}, itemlight.Light, 52)
	after := cave.LightTerms()
	if before.Level != 0 || after.Level != 52 || after.Carried {
		t.Errorf("before %d, after %d (Carried %v); want 0, 52, false", before.Level, after.Level, after.Carried)
	}
}

func sameTerm(a, b float64) bool {
	if math.IsInf(a, -1) || math.IsInf(b, -1) {
		return math.IsInf(a, -1) && math.IsInf(b, -1)
	}
	return math.Abs(a-b) < 0.5
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/rooms/ -run 'FixtureComposition|FixtureFree|ReadsTheRoomsFixtures' -count=1`
Expected: FAIL to build: `c.room.composeWithFixtures undefined (type Room has no field or method composeWithFixtures)`.

- [ ] **Step 3: Compose fixtures**

In `internal/rooms/lighting.go`, add `"github.com/GoMudEngine/GoMud/internal/itemlight"` to the imports (between `gametime` and `lightscale`). Replace

```go
	// Carried reports that someone in the room carries a light.
	Carried bool
	// Darkened reports that someone in the room carries a darkness.
	Darkened bool
}
```

with

```go
	// Carried reports that someone in the room carries a light.
	Carried bool
	// Darkened reports that someone in the room carries a darkness.
	Darkened bool
	// Fixture is the combined light of the room's lit light fixtures (items
	// with `fixture: light`, internal/itemlight); lightscale.Absent() when
	// none is lit. A fixture is part of the room, never a carried light, so
	// it never sets Carried (lighting 5e, X5).
	Fixture float64
	// CarriedLight is the combine of carried light alone;
	// lightscale.Absent() when nobody here carries a lit light. A lantern
	// dimming while still lit, or a second light arriving, moves it where
	// Carried does not (lighting 5e, X5).
	CarriedLight float64
}
```

Replace

```go
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
	step := cfg.DoublingStep
	if !(step > 0) {
		step = 1
	}

	out := LightTerms{SkyFilter: skyFilter}
	terms := make([]float64, 0, 2+len(carried))
```

with

```go
func (r *Room) composeLightExcluding(cfg configs.Lighting, celestial, skyFilter float64, exclude *conditions.Condition) LightTerms {
	carried, dark := r.carriedTerms(exclude)
	fxLight, fxDark := itemlight.Terms(r.RoomId)
	return r.composeWithFixtures(cfg, celestial, skyFilter, carried, dark, fxLight, fxDark)
}

// composeWith is the composition with the carried light and darkness terms
// supplied and no fixtures, so a test needs no users, mobs or items.
func (r *Room) composeWith(cfg configs.Lighting, celestial, skyFilter float64, carried, dark []float64) LightTerms {
	return r.composeWithFixtures(cfg, celestial, skyFilter, carried, dark, nil, nil)
}

// composeWithFixtures is the composition itself, with every term supplied.
//
// Lights combine as they always have; darknesses combine among themselves by
// the same halving rule; the net light is the combined light (0 when none)
// minus the combined darkness (0 when none), clamped to [-100, 100]
// (lighting plan 5d, owner decision 1). A lit light fixture is one term in
// the light combine and a darkness fixture one in the darkness combine,
// beside what people carry (lighting 5e, Rule 10). Fixtures never trim.
func (r *Room) composeWithFixtures(cfg configs.Lighting, celestial, skyFilter float64, carried, dark, fixtureLight, fixtureDark []float64) LightTerms {
	step := cfg.DoublingStep
	if !(step > 0) {
		step = 1
	}

	out := LightTerms{SkyFilter: skyFilter}
	terms := make([]float64, 0, 2+len(fixtureLight)+len(carried))
```

Replace

```go
	// 3. Every light anyone here carries, each its own term (lighting plan 5a):
	// a candle and a torch are different sources, and two torches are one
	// doubling step brighter than one.
	if len(carried) > 0 {
		out.Carried = true
		terms = append(terms, carried...)
	}
```

with

```go
	// 3. Every lit light fixture, each its own term (lighting 5e): part of
	// the room, like its lamp, never a carried light.
	out.Fixture = lightscale.Combine(step, fixtureLight...)
	terms = append(terms, fixtureLight...)

	// 4. Every light anyone here carries, each its own term (lighting plan 5a):
	// a candle and a torch are different sources, and two torches are one
	// doubling step brighter than one.
	out.CarriedLight = lightscale.Combine(step, carried...)
	if len(carried) > 0 {
		out.Carried = true
		terms = append(terms, carried...)
	}
```

and replace

```go
	// 4. Every darkness anyone here carries (lighting plan 5d), combined
	// among themselves by the same halving rule and taken away from the
	// light. Two darknesses of 50 take 58, not 100.
	out.Dark = lightscale.Combine(step, dark...)
	if len(dark) > 0 {
		out.Darkened = true
	}
```

with

```go
	// 5. Every darkness anyone here carries (lighting plan 5d), and every
	// darkness fixture (lighting 5e), combined among themselves by the same
	// halving rule and taken away from the light. Two darknesses of 50 take
	// 58, not 100. Darkened still means a CARRIED darkness.
	darkTerms := make([]float64, 0, len(dark)+len(fixtureDark))
	darkTerms = append(darkTerms, dark...)
	darkTerms = append(darkTerms, fixtureDark...)
	out.Dark = lightscale.Combine(step, darkTerms...)
	if len(dark) > 0 {
		out.Darkened = true
	}
```

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./internal/rooms/ ./internal/lightnotice/ -count=1 && go test . -run 'TestLightingDayCycleAcrossSampleRounds|Parity' -count=1`
Expected: `ok` for `rooms` and `lightnotice`, and the root `ok`: the day-cycle and parity goldens do not move (no fixture is lit in either, F28). If either moves, run `python tools/lighting_golden_diff.py` and explain the move before anything is re-recorded.

- [ ] **Step 5: Commit**

```bash
git add internal/rooms/lighting.go internal/rooms/fixture_compose_test.go
git commit -m "feat(rooms): fixtures join the room's light, LightTerms.Fixture and CarriedLight (5e Rule 10, X5)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Notices name a fixture `lamp` and a dimming or second carried light `carried`; R8 (Rule 12, X5)

**Model:** haiku (mechanical, code given).

**Files:**
- Create: `internal/lightnotice/fixture_attribution_test.go`
- Modify: `internal/lightnotice/tracker.go:145-148` (comment), `:160-163`, `:176-177` (comment)
- Modify: `_datafiles/world/dogmud/narration/light-notices/carried.yaml:9,13`, `lamp.yaml:1-3`
- Re-record: `internal/narration/testdata/stores/light_notices.golden` (two lines)

- [ ] **Step 1: Write the failing test**

Create `internal/lightnotice/fixture_attribution_test.go`:

```go
package lightnotice

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// X5 and Rule 12: a fixture changing notifies as lamp; a carried light that
// dims while still lit, or a second carried light arriving, notifies as
// carried. Each case reads eyes against the pre-5e attribute, which keyed
// carried on presence and lamp on the room's static Lamp int only.
func TestAttributionNamesFixturesAndScheduledCarriedLight(t *testing.T) {
	// 4111 at midwinter midnight: moonlight 34, no lamp of its own.
	night := rooms.LightTerms{Level: 34, Sky: 34, SkyFilter: 1, Dark: lightscale.Absent(),
		Fixture: lightscale.Absent(), CarriedLight: lightscale.Absent()}
	with := func(f func(*rooms.LightTerms)) rooms.LightTerms { t2 := night; f(&t2); return t2 }

	cases := []struct {
		name string
		prev record
		now  observation
		want Cause
	}{
		{"the arch lantern is lit at dusk",
			rec(1, messaging.BandShapes, night),
			obs(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 54; x.Fixture = 52 })),
			CauseLamp},
		{"the arch lantern is snuffed at dawn",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 54; x.Fixture = 52 })),
			obs(1, messaging.BandShapes, night),
			CauseLamp},
		{"a fixture's pulse crosses a band",
			rec(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 45; x.Fixture = 36 })),
			obs(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 52; x.Fixture = 50 })),
			CauseLamp},
		{"a sunstone fades while still lit",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 52; x.Carried = true; x.CarriedLight = 46 })),
			obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 41; x.Carried = true; x.CarriedLight = 30 })),
			CauseCarried},
		{"a second light arrives where one already is",
			rec(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 45; x.Carried = true; x.CarriedLight = 40 })),
			obs(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 52; x.Carried = true; x.CarriedLight = 50 })),
			CauseCarried},
		{"control: a steady fixture and lantern leave a sky change to the sky",
			rec(1, messaging.BandFaces, with(func(x *rooms.LightTerms) { x.Level = 54; x.Sky = 50; x.Fixture = 52 })),
			obs(1, messaging.BandShapes, with(func(x *rooms.LightTerms) { x.Level = 49; x.Sky = 20; x.Fixture = 52 })),
			CauseSky},
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
```

- [ ] **Step 2: Run it to see it fail against today's `attribute`**

Run: `go test ./internal/lightnotice/ -run NamesFixtures -count=1`
Expected: FAIL, five lines: `the arch lantern is lit at dusk: attribute = "eyes", want "lamp"`, `... snuffed at dawn: attribute = "eyes", want "lamp"`, `a fixture's pulse crosses a band: attribute = "eyes", want "lamp"`, `a sunstone fades while still lit: attribute = "eyes", want "carried"`, `a second light arrives where one already is: attribute = "eyes", want "carried"`. The control passes.

- [ ] **Step 3: Implement**

In `internal/lightnotice/tracker.go`, replace

```go
	case a.Carried != b.Carried:
		return CauseCarried
	case a.HasLamp != b.HasLamp || a.Lamp != b.Lamp:
		return CauseLamp
```

with

```go
	// A carried light arriving or leaving, and one that changes while lit (a
	// lantern its schedule dims, a second light joining one already here)
	// (lighting 5e, X5).
	case a.Carried != b.Carried || termMoved(a.CarriedLight, b.CarriedLight):
		return CauseCarried
	// The room's own light: its lamp, and its fixtures (lighting 5e).
	case a.HasLamp != b.HasLamp || a.Lamp != b.Lamp || termMoved(a.Fixture, b.Fixture):
		return CauseLamp
```

replace the comment above `attribute`

```go
// Otherwise, the first term that moved, in the order carried darkness,
// carried light, the room's own light, weather, sky. Darkness comes first
```

with

```go
// Otherwise, the first term that moved, in the order darkness (carried or a
// fixture's), carried light (present, or its strength), the room's own light
// (its lamp and its light fixtures), weather, sky. Darkness comes first
```

and replace

```go
// or moved by more than float noise. The sky and a carried darkness share it.
```

with

```go
// or moved by more than float noise. The sky, the darkness, the carried light
// and the fixtures share it.
```

- [ ] **Step 4: Run it to see it pass**

Run: `go test ./internal/lightnotice/ -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/lightnotice`.

- [ ] **Step 5: R8, the two darker lines say "fades"**

In `_datafiles/world/dogmud/narration/light-notices/carried.yaml`, replace

```yaml
      - 'The carried light is gone, and faces blur into shapes.'
```

with

```yaml
      - 'The carried light fades, and faces blur into shapes.'
```

and replace

```yaml
      - 'The carried light is gone, and darkness closes in.'
```

with

```yaml
      - 'The carried light fades, and darkness closes in.'
```

In `_datafiles/world/dogmud/narration/light-notices/lamp.yaml`, replace

```yaml
# The room's own lamp. Lamps do not change at runtime yet and no mutator
# adds light any more, so nothing reaches these lines until runtime lamps
# arrive (scheduled light sources).
```

with

```yaml
# The room's own light: its lamp, and its fixtures (lighting 5e). A room's
# lamp does not change at runtime, but a fixture does: the North Gate's arch
# lantern lit at dusk and snuffed at dawn, a pulsing stone. These lines fire
# when one moves the observer's band.
```

- [ ] **Step 6: See the narration golden move by exactly the two lines, then re-record**

Run: `go test ./internal/narration/ -run 'TestSnapshotStores/light_notices' -count=1`
Expected: FAIL `golden mismatch for store "light_notices.golden"`, first differing line 18, want `The carried light is gone, and faces blur into shapes.`, got `The carried light fades, and faces blur into shapes.`.

Run: `go test ./internal/narration/ -run 'TestSnapshotStores/light_notices' -count=1 -update` then `git diff internal/narration/`.
Expected: exactly two changed lines, `carried|darker_shapes|any|0` and `carried|darker_dark|any|0`, "is gone" becoming "fades". Then `go test ./internal/narration/ -count=1`: `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/lightnotice/tracker.go internal/lightnotice/fixture_attribution_test.go _datafiles/world/dogmud/narration/light-notices/carried.yaml _datafiles/world/dogmud/narration/light-notices/lamp.yaml internal/narration/testdata/stores/light_notices.golden
git commit -m "feat(lightnotice): fixtures notify as lamp, a dimming or second carried light as carried; carried lines fade (5e X5, R8)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Fixtures are untakeable at every floor-removal path (X10)

**Model:** sonnet (four packages, player and mob paths).

**Files:**
- Create: `internal/actions/get_fixture_test.go`; Modify: `internal/actions/get.go:21-22,38-45`
- Create: `internal/usercommands/get_fixture_test.go`; Modify: `internal/usercommands/get.go:31-43,66-70,224-229,631-633,809-817,828`, `internal/usercommands/skill.skullduggery.steal.go:106-112`
- Create: `internal/hooks/mob_equip_floor_fixture_test.go`; Modify: `internal/hooks/mob_equip_best_floor_item.go:45-46`, `internal/mobcommands/get.go:31-33`

The five paths: a player's `get` and a mob's `get` (both through `actions.TakeFloorItem`), `get all` (player `takeableOnFloor` and both bare sweeps), `steal` (the player command's floor resolution, F20), and the mob idle floor equip (`EquipBestFloorItem`).

- [ ] **Step 1: Write the failing action test**

Create `internal/actions/get_fixture_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const getFixtureItemId = 999990

// X10 and Rule 10: a fixture is refused to every taker that picks up
// through TakeFloorItem (a player's get, a mob's, a companion's). Nothing
// moves.
func TestGetItemFromFloor_RefusesAFixtureToEveryTaker(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		getFixtureItemId: {ItemId: getFixtureItemId, Name: "Arch Lantern", NameSimple: "lantern", Type: items.Object,
			Fixture: items.FixtureLight},
	}))
	char := newTestChar()
	room := newTestRoom()
	room.RoomId = 9702
	room.Items = append(room.Items, items.New(getFixtureItemId))
	actor := newStubActor(char, room)

	result := GetItemFromFloor(actor, "lantern", false)
	require.True(t, result.Found, "the lantern is found")
	require.ErrorIs(t, result.Err, ErrFixture)
	assert.Equal(t, 0, countCharItems(char), "nothing taken")
	assert.Equal(t, 1, countFloorItems(room), "the lantern stays fixed")
}
```

Run: `go test ./internal/actions/ -run RefusesAFixture -count=1`
Expected: FAIL to build: `undefined: ErrFixture`.

- [ ] **Step 2: `ErrFixture` in the shared pickup**

In `internal/actions/get.go`, replace

```go
// ErrExploding refuses an item that is about to explode; a sweep stops on it.
var ErrExploding = errors.New(`it is about to explode`)
```

with

```go
// ErrExploding refuses an item that is about to explode; a sweep stops on it.
var ErrExploding = errors.New(`it is about to explode`)

// ErrFixture refuses a fixture: an item fixed to the room's floor (ItemSpec
// fixture, lighting 5e Rule 10). Every taker is held to it, a player's
// `get`, a mob's, a companion's or a scavenger's; `get all`, `steal` and a
// mob's floor equip never reach for one at all.
var ErrFixture = errors.New(`it is fixed in place`)
```

and replace

```go
// TakeFloorItem moves an item already found on the floor (or in the stash)
// into the actor's backpack, through every pickup gate in the player's order:
// ErrTooDark, ErrExploding, ErrHouseholdBauble, then the transfer (which
// fires ItemOwnership, or rolls back on a full pack).
func TakeFloorItem(actor Actor, item items.Item, stash bool) error {
	if TooDarkToGet(actor) {
		return ErrTooDark
	}
```

with

```go
// TakeFloorItem moves an item already found on the floor (or in the stash)
// into the actor's backpack, through every pickup gate in the player's order:
// ErrTooDark, ErrFixture, ErrExploding, ErrHouseholdBauble, then the transfer
// (which fires ItemOwnership, or rolls back on a full pack).
func TakeFloorItem(actor Actor, item items.Item, stash bool) error {
	if TooDarkToGet(actor) {
		return ErrTooDark
	}
	if item.IsFixture() {
		return ErrFixture
	}
```

Run: `go test ./internal/actions/ -run RefusesAFixture -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/actions`.

- [ ] **Step 3: Write the failing player tests**

Create `internal/usercommands/get_fixture_test.go`:

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fixtureCmdItemId = 999991
	pebbleCmdItemId  = 999992
)

// seedFixtureRoom puts a fixture (Arch Lantern) and a pebble on user 1's
// floor at midsummer noon, so nothing is refused as blind.
func seedFixtureRoom(t *testing.T) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 20
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)
	util.SetRoundCountForTest(uint64(3430))
	t.Cleanup(util.ResetRoundCountForTest)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		fixtureCmdItemId: {ItemId: fixtureCmdItemId, Name: "Arch Lantern", NameSimple: "lantern", Type: items.Object,
			Fixture: items.FixtureLight, Value: 1, NotSalable: true},
		pebbleCmdItemId: {ItemId: pebbleCmdItemId, Name: "Grey Pebble", NameSimple: "pebble", Type: items.Object,
			Weight: 0.1, Value: 1},
	}))
	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Strength.ValueAdj = 50
	origItems := user.Character.Items
	user.Character.Items = nil
	t.Cleanup(func() { user.Character.Items = origItems })
	lantern, pebble := items.New(fixtureCmdItemId), items.New(pebbleCmdItemId)
	room.AddItem(lantern, false)
	room.AddItem(pebble, false)
	t.Cleanup(func() {
		room.RemoveItem(lantern, false)
		room.RemoveItem(pebble, false)
	})
	events.DrainQueuedMessagesForTest(user.UserId)
	return user, room
}

func sentTo(user *users.UserRecord) string {
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
}

// X10: `get <fixture>` says it is fixed in place and takes nothing.
func TestGetAFixtureIsRefused(t *testing.T) {
	user, room := seedFixtureRoom(t)
	_, err := Get("lantern", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "The <ansi fg=\"itemname\">Arch Lantern</ansi> is fixed in place.")
	assert.Empty(t, user.Character.Items, "nothing taken")
}

// X10: `get all` sweeps past a fixture without a word about it, and
// `get all lantern` names it as fixed rather than "you don't see any".
func TestGetAllLeavesAFixture(t *testing.T) {
	user, room := seedFixtureRoom(t)
	Get("all", user, room, 0)
	out := sentTo(user)
	require.Len(t, user.Character.Items, 1, "the pebble is taken")
	assert.Equal(t, pebbleCmdItemId, user.Character.Items[0].ItemId)
	assert.NotContains(t, out, "fixed in place", "a sweep does not name what it leaves fixed")
	_, still := room.FindOnFloor("lantern", false)
	assert.True(t, still, "the lantern stays")

	Get("all lantern", user, room, 0)
	assert.Contains(t, sentTo(user), "is fixed in place.")
}

// X10: `steal <fixture>` names it as fixed in place.
func TestStealAFixtureIsRefused(t *testing.T) {
	user, room := seedFixtureRoom(t)
	user.Character.SetSkill(string(skills.Skullduggery), 2)
	Steal("lantern", user, room, 0)
	assert.Contains(t, sentTo(user), "is fixed in place.")
	assert.Empty(t, user.Character.Items, "nothing taken")
}
```

Run: `go test ./internal/usercommands/ -run 'AFixture|LeavesAFixture' -count=1`
Expected: FAIL, three tests: `get lantern` prints `You can't carry the Arch Lantern - you're already overloaded!` (the default error case, F19); `get all lantern` prints that same line and then `You don't see any "lantern" to pick up.`; `steal lantern` prints `Steal from whom?`.

- [ ] **Step 4: The player paths**

In `internal/usercommands/get.go`, replace

```go
				case errors.Is(result.Err, actions.ErrExploding):
					user.SendText(messaging.CategorySystem, `You can't pick that up, it's about to explode!`)
					return true, nil
				case errors.Is(result.Err, actions.ErrHouseholdBauble):
```

with

```go
				case errors.Is(result.Err, actions.ErrExploding):
					user.SendText(messaging.CategorySystem, `You can't pick that up, it's about to explode!`)
					return true, nil
				case errors.Is(result.Err, actions.ErrFixture):
					fixedInPlace(user, matchItem)
					return true, nil
				case errors.Is(result.Err, actions.ErrHouseholdBauble):
```

replace

```go
// takeableOnFloor is room.FindOnFloor over what may be swept up: the floor
// without this room's household baubles.
func takeableOnFloor(room *rooms.Room, itemName string) (items.Item, bool) {
	takeable := make([]items.Item, 0, len(room.Items))
	for _, it := range room.Items {
		if !it.BaubleBelongsTo(room.RoomId) {
			takeable = append(takeable, it)
		}
	}
```

with

```go
// takeableOnFloor is room.FindOnFloor over what may be swept up: the floor
// without this room's household baubles and without its fixtures (lighting
// 5e).
func takeableOnFloor(room *rooms.Room, itemName string) (items.Item, bool) {
	takeable := make([]items.Item, 0, len(room.Items))
	for _, it := range room.Items {
		if !it.BaubleBelongsTo(room.RoomId) && !it.IsFixture() {
			takeable = append(takeable, it)
		}
	}
```

replace

```go
// leaveHouseholdBauble tells a player sweeping the floor with `get all` that
// a household's bauble was left alone.
```

with

```go
// fixedInPlace tells a player a fixture cannot be taken (lighting 5e).
func fixedInPlace(user *users.UserRecord, itm items.Item) {
	user.SendText(messaging.CategorySystem, fmt.Sprintf(
		`The <ansi fg="itemname">%s</ansi> is fixed in place.`, itm.DisplayName()))
}

// leaveHouseholdBauble tells a player sweeping the floor with `get all` that
// a household's bauble was left alone.
```

in `getAllMatchingFromFloor`, replace

```go
	left := 0
	for _, it := range room.Items {
		if !it.BaubleBelongsTo(room.RoomId) {
			continue
		}
		if part, full := it.NameMatch(itemName, true); part || full {
			leaveHouseholdBauble(user, it)
			left++
		}
	}
```

with

```go
	left := 0
	var fixed []items.Item // fixtures the name matches: never swept (lighting 5e)
	for _, it := range room.Items {
		part, full := it.NameMatch(itemName, true)
		if !part && !full {
			continue
		}
		if it.IsFixture() {
			fixed = append(fixed, it)
			continue
		}
		if !it.BaubleBelongsTo(room.RoomId) {
			continue
		}
		leaveHouseholdBauble(user, it)
		left++
	}
```

and

```go
	if picked == 0 {
		if left == 0 {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`You don't see any "%s" to pick up.`, itemName))
		}
		return
	}
```

with

```go
	if picked == 0 {
		switch {
		case left > 0:
		case len(fixed) > 0:
			// The name only matched what is fixed in place: say so rather
			// than deny there is anything there.
			fixedInPlace(user, fixed[0])
		default:
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`You don't see any "%s" to pick up.`, itemName))
		}
		return
	}
```

and in the bare `get all` sweep, replace

```go
			for _, item := range iCopies {
				// Never by accident: see getAllMatchingFromFloor.
				if item.BaubleBelongsTo(room.RoomId) {
					leaveHouseholdBauble(user, item)
					continue
				}
```

with

```go
			for _, item := range iCopies {
				// A fixture is part of the room: a sweep passes it without a
				// word (lighting 5e).
				if item.IsFixture() {
					continue
				}
				// Never by accident: see getAllMatchingFromFloor.
				if item.BaubleBelongsTo(room.RoomId) {
					leaveHouseholdBauble(user, item)
					continue
				}
```

In `internal/usercommands/skill.skullduggery.steal.go`, replace

```go
	if itm, ok := householdBaubleNamed(room, strings.Join(args, " ")); ok {
		return &actions.StealOptions{HouseholdItem: itm}
	}

	user.SendText(messaging.CategorySystem, "Steal from whom?")
	return nil
}
```

with

```go
	if itm, ok := householdBaubleNamed(room, strings.Join(args, " ")); ok {
		return &actions.StealOptions{HouseholdItem: itm}
	}

	// A fixture is part of the room: nothing to steal (lighting 5e).
	if itm, ok := room.FindOnFloor(strings.Join(args, " "), false); ok && itm.IsFixture() {
		fixedInPlace(user, itm)
		return nil
	}

	user.SendText(messaging.CategorySystem, "Steal from whom?")
	return nil
}
```

Run: `go test ./internal/usercommands/ -run 'AFixture|LeavesAFixture' -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/usercommands`.

- [ ] **Step 5: Write the failing mob test**

Create `internal/hooks/mob_equip_floor_fixture_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/stretchr/testify/require"
)

// X10: a mob idling over a floor fixture never takes it, by the idle floor
// equip (EquipBestFloorItem) or by `get all`, even when it would score as an
// upgrade.
func TestAMobNeverTakesAFloorFixture(t *testing.T) {
	const fixtureItem = 999972
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		fixtureItem: {ItemId: fixtureItem, Name: "test bolted lamp", NameSimple: "lamp",
			Type: items.Light, Subtype: items.Wearable, Fixture: items.FixtureLight,
			PhysicalMitigation: 20}, // would score as an upgrade
	}))
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.BehaviorArchetype = "generic_fighter"
	mob.Character.Awareness = awareness.NewMachine() // the equip path reveals the wearer
	room := rooms.LoadRoom(mob.Character.RoomId)
	require.NotNil(t, room)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = rooms.LampPtr(60)
	room.Items = []items.Item{items.New(fixtureItem)}
	t.Cleanup(func() { room.Items = nil })

	require.False(t, EquipBestFloorItem(mob, room), "the mob took a fixture to wear")
	mobcommands.Get("all", mob, room)
	require.Len(t, room.Items, 1, "the fixture left the floor")
	require.Zero(t, mob.Character.Equipment.Light.ItemId)
	require.Empty(t, mob.Character.Items)
}
```

(The `Awareness` machine is F21: without it the equip this test forbids panics instead of failing cleanly.)

Run: `go test ./internal/hooks/ -run NeverTakesAFloorFixture -count=1 -v`
Expected: FAIL `Should be false ... the mob took a fixture to wear`.

- [ ] **Step 6: The mob paths**

In `internal/hooks/mob_equip_best_floor_item.go`, replace

```go
	for _, floorItem := range room.Items {
		delta := itemvalue.ItemValueDelta(&mob.Character, profile, floorItem)
```

with

```go
	for _, floorItem := range room.Items {
		// A fixture is part of the room, never loot (lighting 5e, X10).
		if floorItem.IsFixture() {
			continue
		}
		delta := itemvalue.ItemValueDelta(&mob.Character, profile, floorItem)
```

In `internal/mobcommands/get.go`, replace

```go
			iCopies := []items.Item{}
			for _, item := range room.Items {
				iCopies = append(iCopies, item)
			}
```

with

```go
			iCopies := []items.Item{}
			for _, item := range room.Items {
				// A fixture is part of the room (lighting 5e): not swept.
				if item.IsFixture() {
					continue
				}
				iCopies = append(iCopies, item)
			}
```

- [ ] **Step 7: Run the packages**

Run: `go build ./... && go test ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ ./internal/hooks/ -count=1 && go test . -count=1`
Expected: build silent; four `ok` lines; the repo root `ok` (the messaging and narration guards accept the new `fixedInPlace` line; measured).

- [ ] **Step 8: Commit**

```bash
git add internal/actions/get.go internal/actions/get_fixture_test.go internal/usercommands/get.go internal/usercommands/skill.skullduggery.steal.go internal/usercommands/get_fixture_test.go internal/hooks/mob_equip_best_floor_item.go internal/hooks/mob_equip_floor_fixture_test.go internal/mobcommands/get.go
git commit -m "feat(actions): fixtures are fixed in place at every floor-removal path (5e X10)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: A fixture is part of the room's look (Rule 10, R9)

**Model:** sonnet (template, look and two sibling lists).

**Files:**
- Create: `internal/usercommands/look_fixture_test.go`, `_datafiles/world/dogmud/templates/descriptions/fixtures.template`
- Modify: `internal/usercommands/look.go:3-24` (imports), `:491-494`, `:638-639`, `:725-729`, end of file
- Create: `modules/gmcp/gmcp.Room_fixture_test.go`; Modify: `modules/gmcp/gmcp.Room.go:254`
- Modify: `world.go:448-451`

- [ ] **Step 1: Write the failing tests**

Create `internal/usercommands/look_fixture_test.go` (it reuses Task 10's `seedFixtureRoom` and `sentTo`, and `useDogmudTemplates` (F24)):

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R9 and Rule 10: a fixture is part of the room. The room look gives it a
// line of its own after the description, lit or unlit, and "On the Ground"
// leaves it out; `look <name>` still finds it.
func TestLookShowsAFixtureAsPartOfTheRoom(t *testing.T) {
	user, room := seedFixtureRoom(t)
	useDogmudTemplates(t)
	t.Cleanup(itemlight.ResetForTest())

	_, err := Look("", user, room, 0)
	require.NoError(t, err)
	out := sentTo(user)
	assert.Contains(t, out, `The <ansi fg="item">Arch Lantern</ansi> is unlit.`)
	ground := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "On the Ground") {
			ground = line
		}
	}
	assert.Contains(t, ground, "Grey Pebble", "the pebble is still on the ground")
	assert.NotContains(t, ground, "Arch Lantern", "the lantern is listed on the ground")

	lantern, ok := room.FindOnFloor("lantern", false)
	require.True(t, ok)
	itemlight.Set(room.RoomId, lantern.UUID, itemlight.Light, 52)
	Look("", user, room, 0)
	assert.Contains(t, sentTo(user), `The <ansi fg="item">Arch Lantern</ansi> is lit.`)

	Look("lantern", user, room, 0)
	assert.Contains(t, sentTo(user), `You look at the <ansi fg="item">Arch Lantern</ansi> here:`)
}
```

Create `modules/gmcp/gmcp.Room_fixture_test.go`:

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// R9, sibling path: the web client's room contents list is the GMCP twin of
// "On the Ground", so a fixture (part of the room) is left out of it too.
func TestRoomContentsItemsLeaveOutFixtures(t *testing.T) {
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		999993: {ItemId: 999993, Name: "Arch Lantern", Type: items.Object, Fixture: items.FixtureLight},
		999994: {ItemId: 999994, Name: "Grey Pebble", Type: items.Object},
	}))
	room := &rooms.Room{RoomId: 9993, Zone: "probe",
		Items: []items.Item{items.New(999993), items.New(999994)}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9993: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(1, "viewer", "Viewer", 0)
	u.Character.RoomId = 9993

	g := &GMCPRoomModule{}
	data, _ := g.GetRoomNode(u, `Room.Info.Contents.Items`)
	got, ok := data.([]GMCPRoomModule_Payload_Contents_Item)
	if !ok {
		t.Fatalf("payload is %T", data)
	}
	if len(got) != 1 || got[0].Name != "Grey Pebble" {
		t.Errorf("contents items = %+v, want only the pebble", got)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/usercommands/ -run LookShowsAFixture -count=1 && go test ./modules/gmcp/ -run LeaveOutFixtures -count=1`
Expected: FAIL. The look output has no `is unlit.` or `is lit.` line, its `On the Ground:` line lists `Arch Lantern and Grey Pebble`, and `look lantern` says `You look at the Arch Lantern on the ground:`; GMCP lists both items (`contents items = [{... Name:Arch Lantern ...} {... Name:Grey Pebble ...}], want only the pebble`).

- [ ] **Step 3: The template**

Create `_datafiles/world/dogmud/templates/descriptions/fixtures.template` (two lines, no trailing blank line):

```
{{- range $index, $fixture := .Fixtures }}<ansi fg="room-description{{ if or $.IsNight $.IsDark }}-dark{{ end }}">The <ansi fg="item">{{ $fixture.Name }}</ansi> is {{ if $fixture.Lit }}lit{{ else }}unlit{{ end }}.</ansi>
{{ end -}}
```

- [ ] **Step 4: `look`**

In `internal/usercommands/look.go`, add `"github.com/GoMudEngine/GoMud/internal/itemlight"` to the imports (between `gametime` and `items`). Replace

```go
	textOut, _ = templates.Process("descriptions/room", details, user.UserId)
	user.SendText(messaging.CategoryRoomDescription, textOut)
```

with

```go
	textOut, _ = templates.Process("descriptions/room", details, user.UserId)
	user.SendText(messaging.CategoryRoomDescription, textOut)

	// Fixtures are part of the room, not things lying on its floor (lighting
	// 5e, ruling R9): a line each, lit or unlit, right after the description
	// and under its sight rule.
	if lines := fixtureLines(room); len(lines) > 0 {
		textOut, _ = templates.Process("descriptions/fixtures", map[string]any{
			`Fixtures`: lines,
			`IsDark`:   details.IsDark,
			`IsNight`:  details.IsNight,
		}, user.UserId)
		user.SendText(messaging.CategoryRoomDescription, textOut)
	}
```

replace (the ground stacks)

```go
	for _, item := range room.Items {
		if !item.IsValid() {
			room.RemoveItem(item, false)
			continue
		}
		// Baubles share one ItemId; each is its own object with its own name.
```

with

```go
	for _, item := range room.Items {
		if !item.IsValid() {
			room.RemoveItem(item, false)
			continue
		}
		// A fixture shows as part of the room, above (R9).
		if item.IsFixture() {
			continue
		}
		// Baubles share one ItemId; each is its own object with its own name.
```

replace (`look <floor item>`)

```go
		// A found bauble lies somewhere in particular ("on the bookshelf").
		where := `on the ground`
		if floorItem.IsBauble() && floorItem.BaubleSpot != `` {
			where = floorItem.BaubleSpot
		}
```

with

```go
		// A found bauble lies somewhere in particular ("on the bookshelf").
		where := `on the ground`
		if floorItem.IsBauble() && floorItem.BaubleSpot != `` {
			where = floorItem.BaubleSpot
		}
		// A fixture is part of the room, not lying on its floor (R9).
		if floorItem.IsFixture() {
			where = `here`
		}
```

and append at the end of the file:

```go

// fixtureLine is one fixture in the room look's descriptions/fixtures
// template (lighting 5e, R9).
type fixtureLine struct {
	Name string
	Lit  bool
}

// fixtureLines lists the room's fixtures in floor order, each lit or unlit
// from its current output (internal/itemlight).
func fixtureLines(room *rooms.Room) []fixtureLine {
	var out []fixtureLine
	for _, it := range room.Items {
		if it.IsFixture() {
			out = append(out, fixtureLine{Name: it.DisplayName(), Lit: itemlight.Lit(room.RoomId, it.UUID)})
		}
	}
	return out
}
```

- [ ] **Step 5: GMCP and tab completion (sibling ground lists, F23)**

In `modules/gmcp/gmcp.Room.go`, replace

```go
		for _, itm := range room.Items {
			payload.Contents.Items = append(payload.Contents.Items, GMCPRoomModule_Payload_Contents_Item{
```

with

```go
		for _, itm := range room.Items {
			// A fixture is part of the room, not on its floor (lighting 5e, R9).
			if itm.IsFixture() {
				continue
			}
			payload.Contents.Items = append(payload.Contents.Items, GMCPRoomModule_Payload_Contents_Item{
```

In `world.go` (tab completion for `get`), replace

```go
			// all items on the floor
			if room := rooms.LoadRoom(user.Character.RoomId); room != nil {
				itemList = room.GetAllFloorItems(false)
			}
```

with

```go
			// all items on the floor, but a fixture, which cannot be taken
			// (lighting 5e)
			if room := rooms.LoadRoom(user.Character.RoomId); room != nil {
				for _, itm := range room.GetAllFloorItems(false) {
					if !itm.IsFixture() {
						itemList = append(itemList, itm)
					}
				}
			}
```

(No test seam reaches the main package's tab completion; the edit mirrors `takeableOnFloor`.)

- [ ] **Step 6: Run them to see them pass**

Run: `go build ./... && go vet . ./internal/usercommands/ ./modules/gmcp/ && go test ./internal/usercommands/ ./modules/gmcp/ -count=1`
Expected: build and vet silent; two `ok` lines.

- [ ] **Step 7: Commit**

```bash
git add internal/usercommands/look.go internal/usercommands/look_fixture_test.go _datafiles/world/dogmud/templates/descriptions/fixtures.template modules/gmcp/gmcp.Room.go modules/gmcp/gmcp.Room_fixture_test.go world.go
git commit -m "feat(look): fixtures show as part of the room, not on the ground (5e R9)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: Content: the four trees, the arch lantern, the Rift Stone, the keeper lantern, the sunstone (slice 1 content, R3, R4, R10)

**Model:** sonnet (world YAML; load `dogmud-authoring-content` and `dogmud-player-copy` first).

**Files:**
- Create: `internal/behaviortree/shipped_item_trees_test.go`
- Modify: `internal/items/shipped_light_items_test.go:34` and append
- Create: `_datafiles/world/dogmud/behaviors/items/dusk_to_dawn.yaml`, `rift_pulse.yaml`, `keeper_lantern.yaml`, `sunstone.yaml`
- Create: `_datafiles/world/dogmud/items/other-0/55-arch_lantern.yaml`, `_datafiles/world/dogmud/items/other-0/56-rift_stone.yaml`, `_datafiles/world/dogmud/items/armor-20000/light/20099-sunstone.yaml`, `_datafiles/world/dogmud/conditions/134-sunstone_glow.yaml`
- Modify: `_datafiles/world/dogmud/items/materials-40000/40038-oil_lantern.yaml`, `rooms/stillwater/4111.yaml:27-30,49-53`, `rooms/thornwall_city/5000.yaml:21-23`, `mobs/stillwater/9588-enchanter_rane.yaml:83-86`

Filenames follow `ConvertForFilename(name)` (`55-arch_lantern.yaml`, `56-rift_stone.yaml`, `20099-sunstone.yaml`, `134-sunstone_glow.yaml`); the ids are free (F26).

- [ ] **Step 1: Write the failing content tests**

Create `internal/behaviortree/shipped_item_trees_test.go`:

```go
package behaviortree

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// loadShippedItemWorld points the engine at the shipped world, loads its
// conditions and items, validates every item tree, and pins the shipped day.
// Returns the round midwinter's dusk falls in.
func loadShippedItemWorld(t *testing.T) uint64 {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(`../../_datafiles/world/dogmud`)
	cfg.Network.LogoutRounds = 3 // condition 0 refuses a 0 trigger count
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	for _, name := range []string{"keeper_lantern", "sunstone", "dusk_to_dawn", "rift_pulse"} {
		GetEngine().EvictItemTree(name)
		t.Cleanup(func() { GetEngine().EvictItemTree(name) })
	}
	if err := ValidateItemBehaviors(); err != nil {
		t.Fatalf("the shipped item trees do not validate: %v", err)
	}
	t.Cleanup(ResetItemBTreeStatesForTest())
	t.Cleanup(itemlight.ResetForTest())
	room := rooms.NewRoom("probe")
	room.RoomId = engineProbeRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{engineProbeRoomId: room}, map[string]*rooms.ZoneConfig{}))
	return pinAfterDuskClock(t)
}

// wearShipped puts a shipped light item in user 1's light slot with its
// worn condition applied, as an equip does.
func wearShipped(t *testing.T, itemId int) (*users.UserRecord, ItemSubject) {
	t.Helper()
	u := users.NewTestUser(1, "probe", "Probe", 0)
	u.Character.RoomId = engineProbeRoomId
	it := items.New(itemId)
	u.Character.Equipment.Light = it
	for _, id := range it.GetSpec().WornConditionIds {
		u.Character.Conditions.AddCondition(id, true)
	}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u}))
	return u, ItemSubject{UUID: it.UUID, ItemId: itemId, UserId: 1, Slot: "light"}
}

func shippedLightNow(t *testing.T, u *users.UserRecord, conditionId int) (float64, bool) {
	t.Helper()
	recs := u.Character.Conditions.GetConditions(conditionId)
	if len(recs) != 1 {
		t.Fatalf("holder carries %d records of condition %d, want 1", len(recs), conditionId)
	}
	return recs[0].LightNow(conditions.GetConditionSpec(conditionId))
}

// R2, R3: the shared Oil Lantern is fully dark while its holder sleeps, and
// back at full on the first round after waking.
func TestShippedKeeperLanternIsDarkWhileItsHolderSleeps(t *testing.T) {
	loadShippedItemWorld(t)
	u, lantern := wearShipped(t, 40038)
	idle := EventContext{EventType: "item_idle"}

	TryItemBehavior(idle, lantern)
	if v, ok := shippedLightNow(t, u, 125); !ok || v != 52 {
		t.Errorf("awake: the lantern gives %v, %v; want 52", v, ok)
	}
	u.Character.Conditions.AddCondition(15, false) // the shipped Sleeping
	TryItemBehavior(idle, lantern)
	if _, ok := shippedLightNow(t, u, 125); ok || u.Character.EmitsLight() {
		t.Error("asleep: the lantern still gives light")
	}
	u.Character.Conditions.RemoveCondition(15)
	TryItemBehavior(idle, lantern)
	if v, ok := shippedLightNow(t, u, 125); !ok || v != 52 {
		t.Errorf("woken: the lantern gives %v, %v; want 52 again", v, ok)
	}
}

// The sunstone: full by day, a faint 30 for an hour after dusk, dark
// through the night.
func TestShippedSunstoneFollowsTheSun(t *testing.T) {
	dusk := loadShippedItemWorld(t)
	u, stone := wearShipped(t, 20099)
	idle := EventContext{EventType: "item_idle"}
	for _, c := range []struct {
		name  string
		round uint64
		want  float64 // -1: dark
	}{
		{"noon", 355*900 + 450, 46},
		{"the dusk round", dusk, 30},
		{"half an hour after dusk", dusk + 19, 30},
		{"an hour and more after dusk", dusk + 40, -1},
		{"midnight", 356 * 900, -1},
	} {
		util.SetRoundCountForTest(c.round)
		TryItemBehavior(idle, stone)
		v, ok := shippedLightNow(t, u, 134)
		switch {
		case c.want < 0 && ok:
			t.Errorf("%s: the sunstone gives %v, want dark", c.name, v)
		case c.want >= 0 && (!ok || v != c.want):
			t.Errorf("%s: the sunstone gives %v, %v; want %v", c.name, v, ok, c.want)
		}
	}
}

// The arch lantern (55) is lit at 52 from dusk to dawn; the Rift Stone (56)
// pulses within 20 to 36.
func TestShippedFixtureTrees(t *testing.T) {
	dusk := loadShippedItemWorld(t)
	idle := EventContext{EventType: "item_idle"}
	arch := ItemSubject{UUID: uuid.UUID{0x61}, ItemId: 55, RoomId: engineProbeRoomId, OnFloor: true}
	stone := ItemSubject{UUID: uuid.UUID{0x62}, ItemId: 56, RoomId: engineProbeRoomId, OnFloor: true}

	util.SetRoundCountForTest(355*900 + 450) // noon
	TryItemBehavior(idle, arch)
	if itemlight.Lit(engineProbeRoomId, arch.UUID) {
		t.Error("noon: the arch lantern is lit")
	}
	util.SetRoundCountForTest(dusk)
	TryItemBehavior(idle, arch)
	if v, _ := itemlight.Get(engineProbeRoomId, arch.UUID); v != 52 {
		t.Errorf("dusk: the arch lantern reads %v, want 52", v)
	}
	for r := uint64(0); r < 24; r++ {
		util.SetRoundCountForTest(dusk + r)
		TryItemBehavior(idle, stone)
		v, _ := itemlight.Get(engineProbeRoomId, stone.UUID)
		if v < 20 || v > 36 || math.IsInf(v, -1) {
			t.Errorf("round %d: the Rift Stone reads %v, want within 20 to 36", dusk+r, v)
		}
	}
}
```

In `internal/items/shipped_light_items_test.go`, replace

```go
		{20097, 54, true},  // Hooded Lantern
	}
```

with

```go
		{20097, 54, true},  // Hooded Lantern
		{20099, 46, false}, // Sunstone (lighting 5e)
	}
```

and append to the file:

```go

// Lighting 5e: the trees and fixtures slice 1 ships. The Oil Lantern carries
// the keeper lantern tree (owner ruling R3); the sunstone is worth 40 (between
// the hooded lantern's 20 and the Umbral Lantern's 60) and sells to an
// enchanter; the two fixtures are fixed lights nobody can sell.
func TestShippedItemBehaviours(t *testing.T) {
	mudlog.SetupLogger(nil, "", "", false)
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(`../../_datafiles/world/dogmud`)
	cfg.Network.LogoutRounds = 3
	configs.SetConfigForTest(t, cfg)
	conditions.LoadDataFiles()
	items.LoadDataFiles()

	for _, c := range []struct {
		itemId   int
		behavior string
		fixture  string
	}{
		{40038, "keeper_lantern", ""},
		{20099, "sunstone", ""},
		{55, "dusk_to_dawn", items.FixtureLight},
		{56, "rift_pulse", items.FixtureLight},
	} {
		spec := items.GetItemSpec(c.itemId)
		if spec == nil {
			t.Fatalf("item %d is not shipped", c.itemId)
		}
		if spec.Behavior != c.behavior || spec.Fixture != c.fixture {
			t.Errorf("item %d: behavior %q fixture %q, want %q and %q", c.itemId, spec.Behavior, spec.Fixture, c.behavior, c.fixture)
		}
		if c.fixture != "" && !spec.NotSalable {
			t.Errorf("fixture %d must be not_salable: it is never loot", c.itemId)
		}
	}
	sun := items.GetItemSpec(20099)
	if sun.Value != 40 || len(sun.VendorCategories) != 1 || sun.VendorCategories[0] != "enchanting" {
		t.Errorf("sunstone value %d, vendor_categories %v; want 40 and [enchanting]", sun.Value, sun.VendorCategories)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/behaviortree/ -run Shipped -count=1; go test ./internal/items/ -run Shipped -count=1`
Expected: `behaviortree` FAIL: `asleep: the lantern still gives light` (40038 has no tree yet), `holder carries 0 records of condition 134, want 1`, `dusk: the arch lantern reads 0, want 52` and `round 320108: the Rift Stone reads 0, want within 20 to 36` and on. `items` FAIL: `item 20099 is not shipped`, and `item 40038: behavior "" fixture "", want "keeper_lantern" and ""`.

- [ ] **Step 3: The four trees**

Create `_datafiles/world/dogmud/behaviors/items/dusk_to_dawn.yaml`:

```yaml
# dusk_to_dawn: a fixed lamp lit from dusk to dawn (lighting 5e, item
# behaviour slice 1). Item 55, the North Gate's arch lantern (room 4111).
# 52 is the Oil Lantern's rung. A fixture's light lives in
# internal/itemlight and never trims.
notes: Lit at the oil lantern's 52 from dusk to dawn, dark by day.
tree:
  type: selector
  event: item_idle
  children:
    - type: sequence
      children:
        - type: condition
          check: time_of_day
          period: night
        - type: action
          do: set_light
          level: 52
    - type: action
      do: set_light
      level: "off"
```

Create `_datafiles/world/dogmud/behaviors/items/rift_pulse.yaml`:

```yaml
# rift_pulse: a slow pulse of faint light (lighting 5e, item behaviour
# slice 1). Item 56, the Rift Stone in room 5000, whose text already says
# the stone pulses with faint light. With the room's lamp 38 the room swings
# 40 to 45, inside shapes for normal eyes, so no notice fires. A pulse may
# not straddle a band edge: item_behaviour_guard_test.go checks it.
notes: Pulses 20 to 36 and back every 12 rounds.
tree:
  type: action
  event: item_idle
  do: pulse_light
  min: 20
  max: 36
  period_rounds: 12
```

Create `_datafiles/world/dogmud/behaviors/items/keeper_lantern.yaml`:

```yaml
# keeper_lantern: the Oil Lantern (40038), shared by shopkeepers and
# players (lighting 5e, owner rulings R2 and R3). Fully dark while its
# holder sleeps (the Sleeping flag, the predicate the shop's sleep gate
# reads), full strength otherwise. Condition 125 is not adjustable, so this
# tree is its record's only writer and an entry trim cannot relight it.
notes: Dark while the holder sleeps, full otherwise.
tree:
  type: selector
  event: item_idle
  children:
    - type: sequence
      children:
        - type: condition
          check: holder_asleep
        - type: action
          do: set_light
          level: "off"
    - type: action
      do: set_light
      level: full
```

Create `_datafiles/world/dogmud/behaviors/items/sunstone.yaml`:

```yaml
# sunstone: a stone that drinks the sun (lighting 5e, item behaviour slice
# 1). Item 20099, condition 134 (strength 46). Full by day, a faint 30 for
# an hour after dusk (dusk moves with the season: time_of_day after_dusk),
# dark the rest of the night.
notes: Full by day, faint for an hour after dusk, dark through the night.
tree:
  type: selector
  event: item_idle
  children:
    - type: sequence
      children:
        - type: condition
          check: time_of_day
          period: day
        - type: action
          do: set_light
          level: full
    - type: sequence
      children:
        - type: condition
          check: time_of_day
          period: after_dusk
          hours: 1
        - type: action
          do: set_light
          level: 30
    - type: action
      do: set_light
      level: "off"
```

- [ ] **Step 4: The items and the condition**

Create `_datafiles/world/dogmud/items/other-0/55-arch_lantern.yaml`:

```yaml
itemid: 55
name: Arch Lantern
namesimple: lantern
# A fixture (lighting 5e): hangs from the North Gate's arch (room 4111),
# placed there by spawninfo, and cannot be taken. Its tree lights it from
# dusk to dawn at the oil lantern's rung.
description: A small iron lantern hung from the arch's crossbeam, the glass
  smoke-blackened along its upper rim. It is lit at dusk and snuffed at
  first light. Even unlit, the smell of old oil lingers faintly around it.
type: object
subtype: mundane
weight: 0.1
value: 1
not_salable: true
fixture: light
behavior: dusk_to_dawn
```

Create `_datafiles/world/dogmud/items/other-0/56-rift_stone.yaml`:

```yaml
itemid: 56
name: Rift Stone
namesimple: stone
# A fixture (lighting 5e): set into the wall of the Rift Chamber (room
# 5000), placed there by spawninfo, and cannot be taken. Its tree pulses it
# between 20 and 36, inside one band for normal eyes.
description: A slab of dark stone set into the chamber wall and etched in
  geometric patterns. A faint light moves through the grooves, swelling and
  ebbing like slow breath, and never quite goes out.
type: object
subtype: mundane
weight: 0.1
value: 1
not_salable: true
fixture: light
behavior: rift_pulse
```

Create `_datafiles/world/dogmud/items/armor-20000/light/20099-sunstone.yaml`:

```yaml
itemid: 20099
name: Sunstone
namesimple: sunstone
# Lighting 5e: a light whose tree follows the sun (behaviors/items/
# sunstone.yaml). Condition 134 is not adjustable, so the tree is its only
# writer. Sold by Enchanter Rane in Stillwater.
description: A pale gold stone grown in a chrysalis vat until it learned to
  drink the sun. Carried as a light, it glows by day, keeps a faint glow for
  a short while after dusk, and is dark through the night.
type: light
subtype: wearable
weight: 0.1
value: 40
vendor_categories:
- enchanting
wornconditionids:
  - 134
behavior: sunstone
```

Create `_datafiles/world/dogmud/conditions/134-sunstone_glow.yaml`:

```yaml
conditionid: 134
name: Sunstone Glow
description: Your sunstone gives back the daylight it drank.
secret: true
triggerrate: 5 real minutes
triggercount: 1
# Lighting 5e: the sunstone's light, the torch's shape. Not adjustable: the
# sunstone's tree (behaviors/items/sunstone.yaml) is its only writer.
effects:
  light_strength: 46
```

In `_datafiles/world/dogmud/items/materials-40000/40038-oil_lantern.yaml`, replace

```yaml
wornconditionids:
  - 125
vendor_categories:
- blacksmithing
```

with

```yaml
wornconditionids:
  - 125
vendor_categories:
- blacksmithing
# Lighting 5e (owner rulings R2, R3): dark while its holder sleeps.
behavior: keeper_lantern
```

- [ ] **Step 5: Place the fixtures, move the 4111 noun into the item, stock the sunstone**

In `_datafiles/world/dogmud/rooms/stillwater/4111.yaml`, replace

```yaml
    roomid: 5372
    zone: North Road North
nouns:
```

with

```yaml
    roomid: 5372
    zone: North Road North
# Lighting 5e: the arch lantern is a fixture (item 55), lit from dusk to
# dawn by its tree; it replaced the old `lantern` noun.
spawninfo:
- itemid: 55
nouns:
```

and delete the `lantern` noun (F22: a noun is resolved before a floor item, so it would shadow item 55), replacing

```yaml
  lantern: A small iron lantern hung from the arch's
    crossbeam, the glass smoke-blackened along its
    upper rim. It is lit at dusk and snuffed at first
    light. Even unlit, the soot-smell of old oil
    lingers faintly around it.
  view:
```

with

```yaml
  view:
```

(The `arch` noun and the idle messages that mention the lantern stay.)

In `_datafiles/world/dogmud/rooms/thornwall_city/5000.yaml`, replace

```yaml
spawninfo:
  - mobid: 315
    respawnrate: "1s"
```

with

```yaml
spawninfo:
  - mobid: 315
    respawnrate: "1s"
  # Lighting 5e: the pulsing Rift Stone, a fixture (item 56).
  - itemid: 56
```

In `_datafiles/world/dogmud/mobs/stillwater/9588-enchanter_rane.yaml`, replace

```yaml
    - itemid: 40053    # Stillwater black pearl
      quantity: 0
      quantitymax: 0
      price: 600
```

with

```yaml
    - itemid: 40053    # Stillwater black pearl
      quantity: 0
      quantitymax: 0
      price: 600
    # Lighting 5e: a chrysalis-grown light that drinks the sun
    - itemid: 20099    # Sunstone
```

(A bare `- itemid:` entry is the shipped shape for stocked lights, e.g. the hooded lantern at `mobs/hartcharn/9182-severin_pell.yaml:57`.)

- [ ] **Step 6: Run them to see them pass, and the content-wide guards**

Run: `go build ./... && go test ./internal/behaviortree/ ./internal/items/ ./internal/narration/ ./internal/rooms/ ./internal/mobs/ ./internal/shops/ -count=1 && go test . -count=1`
Expected: build silent; every package `ok`, including `behaviortree`'s `TestRoundTrip_MarshalFixedPointEveryLiveBehaviorFile` and `TestEventVocabulary_CoversAllLiveBehaviorYAML` over the four new trees, and the repo root (the shop night-trade guard still green, F27; the day-cycle and parity goldens unmoved, F28).

Search the new content for dashes and width: `git diff -U0 -- _datafiles | grep "^+" | grep -c "—\|–"` (run standalone; expect `0` and exit 1) and `git diff -U0 -- _datafiles | grep "^+" | awk 'length > 81'` (expect nothing).

- [ ] **Step 7: Commit**

```bash
git add _datafiles/world/dogmud/behaviors/items/dusk_to_dawn.yaml _datafiles/world/dogmud/behaviors/items/rift_pulse.yaml _datafiles/world/dogmud/behaviors/items/keeper_lantern.yaml _datafiles/world/dogmud/behaviors/items/sunstone.yaml _datafiles/world/dogmud/items/other-0/55-arch_lantern.yaml _datafiles/world/dogmud/items/other-0/56-rift_stone.yaml _datafiles/world/dogmud/items/armor-20000/light/20099-sunstone.yaml _datafiles/world/dogmud/conditions/134-sunstone_glow.yaml _datafiles/world/dogmud/items/materials-40000/40038-oil_lantern.yaml _datafiles/world/dogmud/rooms/stillwater/4111.yaml _datafiles/world/dogmud/rooms/thornwall_city/5000.yaml _datafiles/world/dogmud/mobs/stillwater/9588-enchanter_rane.yaml internal/behaviortree/shipped_item_trees_test.go internal/items/shipped_light_items_test.go
git commit -m "content(lighting-5e): arch lantern and Rift Stone fixtures, keeper lantern tree, the sunstone" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Repo-root guards: Rule 15, the fixture day-cycle record (X8), the shop guard runs the lantern tree

**Model:** sonnet (go/types over export data; whole-world walks).

**Files:**
- Create: `item_behaviour_guard_test.go`, `lighting_fixture_daycycle_golden_test.go`, `testdata/lighting_fixture_daycycle.golden`
- Modify: `shop_night_trade_guard_test.go:10-11` (imports), `:168`, `:210-212`, `:246-250`

- [ ] **Step 1: Rule 15's guards**

Create `item_behaviour_guard_test.go`:

```go
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// The item behaviour foundation's content guards (lighting 5e, item
// behaviour slice 1, spec Rule 15). New item behaviour goes in a tree under
// _datafiles/world/dogmud/behaviors/items, not in another ItemSpec field that
// a hook reads.

// itemSpecBehaviourFields are the ItemSpec fields that make an item DO
// something: per-item reactions a behaviour tree now owns, or the tree
// machinery itself. Each carries the reason it is behaviour. A hook may read
// one only at a site in itemSpecBehaviourReadSites. Every other exported
// field is plain data (itemSpecDataFields), which hooks read freely. The
// pattern is conditions' TestEveryEffectKindIsClassifiedExactlyOnce: every
// field is classified exactly once, so a new field fails until someone says
// which it is.
var itemSpecBehaviourFields = map[string]string{
	"Procs":                "combat procs; slice 3 moves them into proc nodes",
	"ReserveHealthPct":     "Pinnacle reserve held while worn",
	"ReserveStaminaPct":    "Pinnacle reserve held while worn",
	"ReserveConvictionPct": "Pinnacle reserve held while worn",
	"PreservesContents":    "bandolier: contents never age",
	"AmbientPotions":       "bandolier: slotted potions tick while worn",
	"MutationTickInterval": "Pinnacle mutation drip",
	"MutationTickChance":   "Pinnacle mutation drip",
	"MutationRarityFloor":  "Pinnacle mutation drip",
	"VoiceId":              "sentient voice; slice 2 moves it into speak",
	"HungerRounds":         "the Blackrazor's hunger",
	"HungerDrainPct":       "the Blackrazor's hunger",
	"TauntPull":            "the Aegis's taunt pull; slice 2 makes it a tree action",
	"Behavior":             "the item's tree: hooks reach it through behaviortree.TryItemBehavior",
	"Fixture":              "fixed to a floor: read through items.Item.IsFixture",
	"OnUseTrainSkill":      "use effect, the YAML replacement for JS onUse",
	"OnUseTrainAmount":     "use effect, the YAML replacement for JS onUse",
	"OnUseUserText":        "use effect, the YAML replacement for JS onUse",
	"OnUseRoomText":        "use effect, the YAML replacement for JS onUse",
}

// itemSpecDataFields are the ItemSpec fields that describe an item: what it
// is, weighs, costs, protects, and how it is named and stored.
var itemSpecDataFields = []string{
	"ItemId", "Value", "Uses", "ConditionIds", "WornConditionIds", "Nouns",
	"PhysicalMitigation", "MagicalMitigation", "ConvictionMitigation",
	"DamageMultiplier", "SpellDamageMultiplier", "ParryRating", "BlockRating",
	"AmmoTag", "MinStrength", "WaitRounds", "StaminaCost", "SpeedMultiplier",
	"Weight", "GrappleModifier", "EscapeModifier", "Reach", "Hands", "Name",
	"DisplayName", "NameSimple", "Description", "QuestToken", "Type",
	"Subtype", "Damage", "Element", "StatMods", "BreakChance", "Cursed",
	"KeyLockId", "ComponentTag", "IsComponent", "WeightReduction",
	"BagCapacity", "Aging", "BottleAgingMultiplier", "Toxicity", "Magnitude",
	"IsBandolier", "BandolierCapacity", "SalvageReturns", "RarityTier",
	"MaterialTier", "VendorCategories", "NotSalable", "NeverDrops",
	"Restricted",
}

// itemSpecBehaviourMethods maps an ItemSpec method to the behaviour field it
// reads, so calling it counts as reading that field.
var itemSpecBehaviourMethods = map[string]string{
	"ProcsFor": "Procs",
}

// itemSpecBehaviourReadSites are the only places non-test internal/hooks
// reads a behaviour field: "file|field". These are today's Pinnacle
// mechanics. Slice 2 retires the VoiceId and TauntPull sites, slice 3 the
// Procs ones; an entry nothing reads any more fails, so the list only
// shrinks. A new site fails: put the behaviour in a tree.
var itemSpecBehaviourReadSites = map[string]bool{
	"MobDeath_ItemProcs.go|VoiceId":               true,
	"PlayerSpawn_HandleJoin.go|PreservesContents": true,
	"item_procs.go|Procs":                         true,
	"pinnacle_tick.go|AmbientPotions":             true,
	"pinnacle_tick.go|HungerDrainPct":             true,
	"pinnacle_tick.go|HungerRounds":               true,
	"pinnacle_tick.go|MutationRarityFloor":        true,
	"pinnacle_tick.go|MutationTickChance":         true,
	"pinnacle_tick.go|MutationTickInterval":       true,
	"pinnacle_tick.go|PreservesContents":          true,
	"pinnacle_tick.go|TauntPull":                  true,
	"pinnacle_tick.go|VoiceId":                    true,
}

// (a) Rule 15, part one: every exported ItemSpec field is classified
// exactly once, behaviour or data, and no classification names a field that
// does not exist.
func TestEveryItemSpecFieldIsClassifiedExactlyOnce(t *testing.T) {
	data := map[string]bool{}
	for _, f := range itemSpecDataFields {
		if data[f] {
			t.Errorf("ItemSpec field %s is listed twice as data", f)
		}
		data[f] = true
	}
	fields := map[string]bool{}
	st := reflect.TypeOf(items.ItemSpec{})
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if !f.IsExported() {
			continue
		}
		fields[f.Name] = true
		_, behaviour := itemSpecBehaviourFields[f.Name]
		switch {
		case behaviour && data[f.Name]:
			t.Errorf("ItemSpec field %s is classified as both behaviour and data", f.Name)
		case !behaviour && !data[f.Name]:
			t.Errorf("ItemSpec field %s is not classified. If it makes an item DO something, it belongs "+
				"in a behaviour tree (behaviors/items); if it only describes the item, list it in "+
				"itemSpecDataFields", f.Name)
		}
	}
	for name, reason := range itemSpecBehaviourFields {
		if !fields[name] {
			t.Errorf("itemSpecBehaviourFields names %s, which ItemSpec does not have", name)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("behaviour field %s carries no reason", name)
		}
	}
	for name := range data {
		if !fields[name] {
			t.Errorf("itemSpecDataFields names %s, which ItemSpec does not have", name)
		}
	}
	for method, field := range itemSpecBehaviourMethods {
		if _, ok := itemSpecBehaviourFields[field]; !ok {
			t.Errorf("method %s maps to %s, which is not a behaviour field", method, field)
		}
	}
}

// (a) Rule 15, part two: internal/hooks reads a behaviour field only at an
// allowlisted site. Data fields are free.
func TestHooksReadItemBehaviourFieldsOnlyAtAllowlistedSites(t *testing.T) {
	reads := itemSpecReadsIn(t, "./internal/hooks")
	if len(reads["VoiceId"]) == 0 {
		t.Fatalf("the scan found no VoiceId read: it is not seeing internal/hooks (got %v)", reads)
	}
	seen := map[string]bool{}
	var problems []string
	for member, files := range reads {
		field := member
		if f, ok := itemSpecBehaviourMethods[member]; ok {
			field = f
		}
		if _, behaviour := itemSpecBehaviourFields[field]; !behaviour {
			continue
		}
		for file := range files {
			site := file + "|" + field
			seen[site] = true
			if !itemSpecBehaviourReadSites[site] {
				problems = append(problems, fmt.Sprintf("%s reads behaviour field %s (via %s), not an allowlisted site", file, field, member))
			}
		}
	}
	for site := range itemSpecBehaviourReadSites {
		if !seen[site] {
			problems = append(problems, fmt.Sprintf("allowlisted site %s is no longer read: drop it", site))
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Errorf("internal/hooks reads ItemSpec behaviour fields off the allowlist. New item behaviour goes "+
			"in a tree (behaviors/items), not in a field a hook reads; a retired read drops its site.\n%s",
			strings.Join(problems, "\n"))
	}
}

// itemSpecReadsIn type-checks one package's non-test files and returns, for
// every ItemSpec member a selector reads (field or method, through a value
// or a pointer), the files that read it. Dependencies come from the build
// cache's export data (go list -export), so nothing is type-checked twice.
func itemSpecReadsIn(t *testing.T, pkg string) map[string]map[string]bool {
	t.Helper()
	out, err := exec.Command("go", "list", "-export", "-deps", "-f", "{{.ImportPath}}={{.Export}}", pkg).Output()
	if err != nil {
		t.Fatalf("go list -export %s: %v", pkg, err)
	}
	exports := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if path, file, ok := strings.Cut(sc.Text(), "="); ok && file != "" {
			exports[path] = file
		}
	}
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(file)
	})

	dir := filepath.FromSlash(strings.TrimPrefix(pkg, "./"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatal(perr)
		}
		files = append(files, f)
	}
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: imp}
	if _, err := conf.Check(pkg, fset, files, info); err != nil {
		t.Fatalf("type-checking %s: %v", pkg, err)
	}

	got := map[string]map[string]bool{}
	for expr, sel := range info.Selections {
		recv := sel.Recv()
		if p, ok := recv.(*types.Pointer); ok {
			recv = p.Elem()
		}
		named, ok := recv.(*types.Named)
		if !ok || named.Obj().Name() != "ItemSpec" || named.Obj().Pkg() == nil ||
			named.Obj().Pkg().Path() != "github.com/GoMudEngine/GoMud/internal/items" {
			continue
		}
		member := sel.Obj().Name()
		if got[member] == nil {
			got[member] = map[string]bool{}
		}
		got[member][filepath.Base(fset.Position(expr.Pos()).Filename)] = true
	}
	return got
}

// loadItemBehaviourWorld loads the shipped world the way the lighting
// goldens do, with the shipped day pinned.
func loadItemBehaviourWorld(t *testing.T) {
	t.Helper()
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 900
	cfg.Timing.NightHours = 8
	cfg.Timing.RoundSeconds = 4
	cfg.Timing.Validate()
	configs.SetConfigForTest(t, cfg)
	gametime.ClearDateCacheForTest()
	gametime.ClearCelestialMemoForTest()
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()
	conditions.LoadDataFiles()
	items.LoadDataFiles()
	mutators.LoadDataFiles()
	originalRound := util.GetRoundCount()
	t.Cleanup(func() {
		util.SetRoundCount(originalRound)
		gametime.ClearDateCacheForTest()
		gametime.ClearCelestialMemoForTest()
	})
}

// treedItems returns every shipped item that names a tree, with the tree's
// definition, in item id order.
func treedItems(t *testing.T) ([]*items.ItemSpec, map[string]behaviortree.TreeDef) {
	t.Helper()
	var out []*items.ItemSpec
	defs := map[string]behaviortree.TreeDef{}
	for _, spec := range items.GetAllItemSpecsMap() {
		if spec.Behavior == "" {
			continue
		}
		out = append(out, spec)
		if _, ok := defs[spec.Behavior]; !ok {
			def, err := behaviortree.LoadTreeDef(behaviortree.GetItemTreePath(spec.Behavior))
			if err != nil {
				t.Fatalf("item %d: behavior %q: %v", spec.ItemId, spec.Behavior, err)
			}
			defs[spec.Behavior] = def
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemId < out[j].ItemId })
	if len(out) < 4 {
		t.Fatalf("found only %d treed items: the scan is not seeing the world", len(out))
	}
	return out, defs
}

// (b) and (e) Rule 15: every behavior: resolves and compiles, and no tree
// schedules an adjustable light (the boot check, run over the shipped world).
func TestEveryShippedItemBehaviorResolves(t *testing.T) {
	loadItemBehaviourWorld(t)
	if err := behaviortree.ValidateItemBehaviors(); err != nil {
		t.Fatal(err)
	}
	treed, defs := treedItems(t)
	for _, spec := range treed {
		if !behaviortree.TreeWritesLight(defs[spec.Behavior].Tree) {
			continue
		}
		for _, cid := range spec.WornConditionIds {
			c := conditions.GetConditionSpec(cid)
			for _, f := range c.Flags {
				if f == conditions.Adjustable {
					t.Errorf("item %d (%s): tree %q writes light, but worn condition %d is adjustable", spec.ItemId, spec.Name, spec.Behavior, cid)
				}
			}
		}
	}
}

// spawnedFloorItems returns, per room, the ids of the items its spawninfo
// places on the floor.
func spawnedFloorItems(t *testing.T) map[int][]int {
	t.Helper()
	ids := rooms.GetAllRoomIds()
	if len(ids) < 1000 {
		t.Fatalf("loaded only %d rooms: the walk is not seeing the world", len(ids))
	}
	sort.Ints(ids)
	out := map[int][]int{}
	for _, id := range ids {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load", id)
		}
		for _, si := range r.SpawnInfo {
			if si.ItemId > 0 && si.Container == "" {
				out[id] = append(out[id], si.ItemId)
			}
		}
	}
	return out
}

// (c) Rule 15: a light-writing tree belongs to a light or a fixture, and an
// item spawninfo leaves on a floor with such a tree is a fixture (else
// anyone could pick the lamp-post up).
func TestLightWritingTreesBelongToLightsAndFixtures(t *testing.T) {
	loadItemBehaviourWorld(t)
	treed, defs := treedItems(t)
	writes := map[int]bool{}
	for _, spec := range treed {
		if !behaviortree.TreeWritesLight(defs[spec.Behavior].Tree) {
			continue
		}
		writes[spec.ItemId] = true
		if spec.Type != items.Light && spec.Fixture == "" {
			t.Errorf("item %d (%s): tree %q writes light but the item is neither type light nor a fixture", spec.ItemId, spec.Name, spec.Behavior)
		}
	}
	placed := 0
	for roomId, itemIds := range spawnedFloorItems(t) {
		for _, id := range itemIds {
			if !writes[id] {
				continue
			}
			placed++
			if spec := items.GetItemSpec(id); spec.Fixture == "" {
				t.Errorf("room %d places item %d (%s), whose tree writes light, on its floor, but it is not a fixture: anyone could take it", roomId, id, spec.Name)
			}
		}
	}
	if placed < 2 {
		t.Errorf("found %d light-writing floor items placed by spawninfo, want the arch lantern and the Rift Stone at least: the walk is broken", placed)
	}
}

// pulseNodes collects every pulse_light node's min and max.
func pulseNodes(def behaviortree.NodeDef, out *[][2]float64) {
	if def.Do == "pulse_light" {
		*out = append(*out, [2]float64{paramFloat(def.Params["min"]), paramFloat(def.Params["max"])})
	}
	for _, ch := range def.Children {
		pulseNodes(ch, out)
	}
	if def.Child != nil {
		pulseNodes(*def.Child, out)
	}
}

func paramFloat(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case float64:
		return x
	}
	return math.NaN()
}

// (d) Rule 11: a pulse may not straddle a band edge. For every fixture
// spawninfo places whose tree pulses, a normal observer's band with the
// fixture at its min and at its max, through the real room at the shop
// guard's 72 samples, must agree at every sample, so a pulse never fires a
// notice. Nightvision moves the edges, so keep a pulse well inside a band.
func TestPulsingFixturesStayInsideOneBand(t *testing.T) {
	loadItemBehaviourWorld(t)
	t.Cleanup(itemlight.ResetForTest())
	treed, defs := treedItems(t)
	pulses := map[int][][2]float64{}
	for _, spec := range treed {
		if spec.Fixture == "" {
			continue
		}
		var nodes [][2]float64
		pulseNodes(defs[spec.Behavior].Tree, &nodes)
		if len(nodes) > 0 {
			pulses[spec.ItemId] = nodes
		}
	}
	observer := &characters.Character{}
	probe := uuid.UUID{0x5e}
	checked := 0
	for roomId, itemIds := range spawnedFloorItems(t) {
		r := rooms.LoadRoom(roomId)
		for _, id := range itemIds {
			kind := itemlight.Light
			if items.GetItemSpec(id).Fixture == items.FixtureDarkness {
				kind = itemlight.Darkness
			}
			for _, mm := range pulses[id] {
				for _, d := range nightTradeSampleDays {
					for hour := 0; hour < 24; hour++ {
						util.SetRoundCount(uint64(d.Doy-1)*900 + uint64(math.Ceil(float64(hour)*37.5)))
						itemlight.Set(roomId, probe, kind, mm[0])
						atMin := messaging.LightBand(observer, r)
						itemlight.Set(roomId, probe, kind, mm[1])
						atMax := messaging.LightBand(observer, r)
						itemlight.Clear(roomId, probe)
						checked++
						if atMin != atMax {
							t.Errorf("room %d item %d pulses %v to %v, which straddles a band edge at %s %02d:00 (%v at min, %v at max)",
								roomId, id, mm[0], mm[1], d.Name, hour, atMin, atMax)
						}
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no pulsing fixture was checked: the Rift Stone in room 5000 should be")
	}
}
```

Run: `go test . -run 'ClassifiedExactlyOnce|AllowlistedSites|EveryShippedItemBehavior|LightWritingTrees|PulsingFixtures' -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud`.

- [ ] **Step 2: Prove each guard able to fail**

(a), the site check. Create `internal/hooks/zz_probe.go` containing `package hooks`, `import "github.com/GoMudEngine/GoMud/internal/items"`, `var probeWeight = items.ItemSpec{}.Weight` and `var probeVoice = items.ItemSpec{}.VoiceId`; run `go test . -run AllowlistedSites -count=1`. Expected: FAIL with exactly one problem, `zz_probe.go reads behaviour field VoiceId (via VoiceId), not an allowlisted site` (the `Weight` read is data and passes). Delete the file.

(a), the classification. In `internal/items/itemspec.go` add a line `ProbeKnob int \`yaml:"probe_knob,omitempty"\`` directly after the `Fixture` field; run `go test . -run ClassifiedExactlyOnce -count=1`. Expected: FAIL `ItemSpec field ProbeKnob is not classified. If it makes an item DO something, it belongs in a behaviour tree (behaviors/items); if it only describes the item, list it in itemSpecDataFields`. Remove the line.

(c) Delete `fixture: light` from `56-rift_stone.yaml`; run `go test . -run LightWritingTrees -count=1`. Expected: FAIL `item 56 (Rift Stone): tree "rift_pulse" writes light but the item is neither type light nor a fixture` and `room 5000 places item 56 (Rift Stone), whose tree writes light, on its floor, but it is not a fixture: anyone could take it`. Restore the line.

(d) The spec's probe: set `max: 60` in `rift_pulse.yaml`; run `go test . -run PulsingFixtures -count=1`. Expected: FAIL `room 5000 item 56 pulses 20 to 60, which straddles a band edge at midwinter 00:00 (shapes at min, faces at max)`. Restore `max: 36`.

Run `git status --short` afterwards: only the files this task creates are new.

- [ ] **Step 3: The fixture day-cycle record (X8)**

Create `lighting_fixture_daycycle_golden_test.go`:

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/hooks"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

var updateFixtureDaycycle = flag.Bool("update-lighting-fixture-daycycle", false,
	"re-record testdata/lighting_fixture_daycycle.golden")

// fixtureDaycycleRooms are the rooms whose fixtures slice 1 ships: 4111 (the
// North Gate's arch lantern, item 55) and 5000 (the Rift Chamber's Rift
// Stone, item 56).
var fixtureDaycycleRooms = []int{4111, 5000}

// TestLightingFixtureDayCycle is the fixture day-cycle record of lighting 5e
// (spec X8). The day-cycle golden loads no items and runs no Prepare and no
// tick, so it cannot see a fixture. This test loads items, prepares the two
// fixture rooms (spawninfo places each fixture), and runs the real item tick
// (hooks.ItemRoundTick) at the day-cycle golden's twelve samples, recording
// each room's light and its fixture's state.
//
// Expected shape: 4111 reads its sky by day and the sky combined with the
// lantern's 52 at night (midwinter midnight 34 becomes 54: shapes to
// faces); 5000 is a dungeon with no sky, so it reads 40 to 45 at every hour,
// whatever round the sample lands on in the stone's twelve-round pulse.
func TestLightingFixtureDayCycle(t *testing.T) {
	loadItemBehaviourWorld(t)
	t.Cleanup(items.ResetHolderIndexForTest())
	t.Cleanup(itemlight.ResetForTest())
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	prevHook := items.OnRoomHolderIndexed
	items.OnRoomHolderIndexed = hooks.EvaluateRoomFixtures
	t.Cleanup(func() { items.OnRoomHolderIndexed = prevHook })

	prepared := map[int]*rooms.Room{}
	for _, id := range fixtureDaycycleRooms {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load", id)
		}
		r.Prepare(false)
		prepared[id] = r
	}
	// Leave the shared rooms as the other root tests expect them: no
	// fixture on the floor, nothing lit.
	t.Cleanup(func() {
		for _, r := range prepared {
			for _, it := range append([]items.Item(nil), r.Items...) {
				if it.IsFixture() {
					r.RemoveItem(it, false)
				}
			}
		}
	})

	var b strings.Builder
	for _, s := range sampleRounds() {
		util.SetRoundCount(s.Round)
		hooks.ItemRoundTick(events.NewRound{})
		fmt.Fprintf(&b, "== %s (round %d)\n", s.Label, s.Round)
		for _, id := range fixtureDaycycleRooms {
			r := prepared[id]
			fixtures := []string{}
			for _, it := range r.Items {
				if !it.IsFixture() {
					continue
				}
				state := "unlit"
				if v, ok := itemlight.Get(id, it.UUID); ok && itemlight.Lit(id, it.UUID) {
					state = fmt.Sprintf("lit %.1f", v)
				}
				fixtures = append(fixtures, fmt.Sprintf("%d %s", it.ItemId, state))
			}
			if len(fixtures) == 0 {
				t.Fatalf("room %d has no fixture on its floor after Prepare: spawninfo did not place it", id)
			}
			fmt.Fprintf(&b, "room %d light=%d fixtures=[%s]\n", id, r.LightLevel(), strings.Join(fixtures, ", "))
		}
	}
	got := b.String()

	goldenPath := filepath.Join("testdata", "lighting_fixture_daycycle.golden")
	if *updateFixtureDaycycle {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("recorded %s", goldenPath)
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (record it with -update-lighting-fixture-daycycle)", err)
	}
	if got != string(want) {
		t.Errorf("fixture day-cycle golden moved. Explain every changed line (which fixture, "+
			"which sample, why) before re-recording with\n"+
			"  go test . -run TestLightingFixtureDayCycle -update-lighting-fixture-daycycle -v\n\ngot:\n%s", got)
	}
}
```

Run: `go test . -run TestLightingFixtureDayCycle -count=1`
Expected: FAIL `read golden: open testdata\lighting_fixture_daycycle.golden: The system cannot find the file specified. (record it with -update-lighting-fixture-daycycle)`.

Run: `go test . -run TestLightingFixtureDayCycle -count=1 -update-lighting-fixture-daycycle`, then compare `testdata/lighting_fixture_daycycle.golden` with this, which the dry run recorded (it must match line for line; a difference is a finding to explain, not to re-record over):

```
== midwinter-midnight (round 319500)
room 4111 light=54 fixtures=[55 lit 52.0]
room 5000 light=40 fixtures=[56 lit 20.0]
== midwinter-dawn (round 319725)
room 4111 light=54 fixtures=[55 lit 52.0]
room 5000 light=42 fixtures=[56 lit 28.0]
== midwinter-noon (round 319950)
room 4111 light=63 fixtures=[55 unlit]
room 5000 light=45 fixtures=[56 lit 36.0]
== midwinter-dusk (round 320175)
room 4111 light=54 fixtures=[55 lit 52.0]
room 5000 light=42 fixtures=[56 lit 28.0]
== equinox-midnight (round 72000)
room 4111 light=53 fixtures=[55 lit 52.0]
room 5000 light=40 fixtures=[56 lit 20.0]
== equinox-dawn (round 72225)
room 4111 light=53 fixtures=[55 lit 52.0]
room 5000 light=42 fixtures=[56 lit 28.0]
== equinox-noon (round 72450)
room 4111 light=70 fixtures=[55 unlit]
room 5000 light=45 fixtures=[56 lit 36.0]
== equinox-dusk (round 72675)
room 4111 light=53 fixtures=[55 lit 52.0]
room 5000 light=42 fixtures=[56 lit 28.0]
== midsummer-midnight (round 153900)
room 4111 light=54 fixtures=[55 lit 52.0]
room 5000 light=40 fixtures=[56 lit 20.0]
== midsummer-dawn (round 154125)
room 4111 light=61 fixtures=[55 unlit]
room 5000 light=42 fixtures=[56 lit 28.0]
== midsummer-noon (round 154350)
room 4111 light=74 fixtures=[55 unlit]
room 5000 light=45 fixtures=[56 lit 36.0]
== midsummer-dusk (round 154575)
room 4111 light=61 fixtures=[55 unlit]
room 5000 light=42 fixtures=[56 lit 28.0]
```

The 4111 day readings are the day-cycle golden's own (`63`, `70`, `74`, `61`); every night reading is the sky combined with the lantern's 52; 5000 stays inside 40 to 45.

- [ ] **Step 4: The shop guard runs each keeper's light-slot tree (spec "Shop guard")**

In `shop_night_trade_guard_test.go`, add `"github.com/GoMudEngine/GoMud/internal/behaviortree"` to the imports (after `actions`). Replace

```go
	totalSamples, totalNight := 0, 0
```

with

```go
	totalSamples, totalNight := 0, 0
	treedSamples, asleepSamples := 0, 0
	var litAsleep []string // keepers asleep with their lantern still lit
```

replace

```go
				sun := gametime.SunLight(lighting, gd.Day, float64(gd.Hour24)+gd.MinuteFloat/60)
				night := math.IsInf(sun, -1)

				here := roamRooms
```

with

```go
				sun := gametime.SunLight(lighting, gd.Day, float64(gd.Hour24)+gd.MinuteFloat/60)
				night := math.IsInf(sun, -1)

				// Lighting 5e (owner rulings R2, R3): the keeper's light-slot
				// tree runs at every sample, with the Sleeping flag set as
				// the schedule would set it. Asleep, its lantern must be
				// dark; awake, the samples below must still pass.
				asleep := false
				if sched != nil {
					if seg := sched.CurrentSegment(hour); seg != nil && seg.Activity == "sleeping" {
						asleep = true
					}
				}
				if asleep {
					inst.Character.Conditions.AddCondition(15, false) // Sleeping
				} else {
					inst.Character.Conditions.RemoveCondition(15)
				}
				if lamp := inst.Character.Equipment.Light; lamp.ItemId > 0 && lamp.HasBehavior() {
					behaviortree.TryItemBehavior(behaviortree.EventContext{EventType: "item_idle"}, behaviortree.ItemSubject{
						UUID: lamp.UUID, ItemId: lamp.ItemId, MobInstanceId: inst.InstanceId, Slot: "light"})
					treedSamples++
					if asleep {
						asleepSamples++
						if inst.Character.EmitsLight() {
							litAsleep = append(litAsleep, fmt.Sprintf("mob %d %s at %s %02d:00",
								p.mob.MobId, p.mob.Character.Name, d.Name, hour))
						}
					}
				}

				here := roamRooms
```

and replace

```go
	t.Logf("checked %d shop placements across %d awake samples, %d of them at night",
		len(placements), totalSamples, totalNight)
```

with

```go
	// Nine scheduled keepers carry the Oil Lantern and sleep (spec C3).
	if treedSamples == 0 || asleepSamples == 0 {
		t.Fatalf("the keepers' lantern tree ran at %d samples, %d of them asleep: the 5e check saw nothing", treedSamples, asleepSamples)
	}
	t.Logf("checked %d shop placements across %d awake samples, %d of them at night; the lantern tree ran at %d samples, %d asleep",
		len(placements), totalSamples, totalNight, treedSamples, asleepSamples)
	if len(litAsleep) > 0 {
		t.Errorf("%d samples found a keeper asleep with its lantern still lit. The Oil Lantern's tree\n"+
			"(behaviors/items/keeper_lantern.yaml) must put it out while its holder sleeps (owner\n"+
			"ruling R3).\n\n%s", len(litAsleep), strings.Join(litAsleep, "\n"))
	}
```

Run: `go test . -run TestEveryShopkeeperCanTradeAtNightWhileAwake -count=1 -v`
Expected: `ok`, logging `... the lantern tree ran at 1512 samples, 216 asleep` (21 keepers carry 40038, nine of them sleep; F27). The awake samples still pass: ruling R2's flag reading keeps an awake keeper's lantern lit.

Prove it able to fail: in `keeper_lantern.yaml` change the first `level: "off"` to `level: full` and re-run. Expected: FAIL `216 samples found a keeper asleep with its lantern still lit.` beginning `mob 97 Blacksmith Kerra at midwinter 00:00`. Restore `level: "off"`.

- [ ] **Step 5: The whole root package**

Run: `go test . -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud` (the fixture day-cycle test removes its fixtures on cleanup, so the day-cycle golden that may run after it is unmoved).

- [ ] **Step 6: Commit**

```bash
git add item_behaviour_guard_test.go lighting_fixture_daycycle_golden_test.go testdata/lighting_fixture_daycycle.golden shop_night_trade_guard_test.go
git commit -m "test(lighting-5e): item behaviour guards, the fixture day-cycle record, keepers' lanterns in the shop guard (Rule 15, X8)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 14: Docs, full gate, race run, boot check

**Model:** sonnet.

**Files:**
- Modify: `context.md` in `internal/behaviortree`, `internal/items`, `internal/rooms`, `internal/lightnotice`, `internal/gametime`, `internal/hooks`, `internal/characters`, `internal/actions`, `internal/usercommands`, `internal/mobcommands`, `internal/mobs`, `modules/gmcp` (`internal/itemlight/context.md` shipped with Task 2; `internal/conditions` is unchanged, "Where the spec could not be implemented as written" item 11)
- Modify: `docs/schemas/behavior.md:101` and append; `docs/PATCH_NOTES.md:1-3`

- [ ] **Step 1: Verify every symbol before naming it**

Run (PowerShell): `Select-String -Path internal\behaviortree\item_engine.go,internal\behaviortree\item_state.go,internal\behaviortree\conditions_item.go,internal\behaviortree\actions_item_light.go,internal\behaviortree\loader.go,internal\itemlight\itemlight.go,internal\items\item_behavior.go,internal\characters\item_behaviour.go,internal\rooms\item_behaviour.go,internal\rooms\lighting.go,internal\hooks\NewRound_ItemRoundTick.go,internal\gametime\gametime.go,internal\actions\get.go,internal\usercommands\get.go,internal\usercommands\look.go -Pattern '^(func|type|const|var)\s'`
Expected to include: `func TryItemBehavior(event EventContext, subject ItemSubject) (handled bool)`, `type ItemSubject struct {`, `func GetItemTreePath(name string) string`, `func ValidateItemBehaviors() error`, `func TreeWritesLight(def NodeDef) bool`, `func EnsureItemBTreeState(id uuid.UUID) *BehaviorState`, `func EvictItemBTreeState(id uuid.UUID)`, `func EvictUnseenItemBTreeStates(round uint64) int`, `func PulseLightValue(min, max float64, periodRounds int, round uint64) float64`, `func LoadItemTreeFromFile(path string) (Node, error)`, `func LoadItemTreeFromBytes(data []byte) (Node, error)`, `func (i Item) HasBehavior() bool`, `func (i Item) IsFixture() bool`, `func IndexMobHolder(instanceId int)`, `func IndexRoomHolder(roomId int)`, `func MobHolders() []int`, `func RoomHolders() []int`, `func (c *Character) IndexTreedItem(i items.Item)`, `func (c *Character) IndexTreedItems()`, `func IndexTreedFloors()`, `func ItemRoundTick(e events.Event) events.ListenerReturn`, `func EvaluateRoomFixtures(roomId int)`, `func (g GameDate) HoursAfterDusk() float64`, `var ErrFixture`, `func fixedInPlace(`, `func fixtureLines(`. Name nothing that is not in that output. (`itemSafeConditions`, `itemSafeActions`, `itemOnlyNodes` are in a `var (` block in `loader.go`; `composeWithFixtures` and the `LightTerms` fields are in `lighting.go`; `EvalContext.node` in `types.go`: grep each.)

- [ ] **Step 2: `internal/behaviortree/context.md`**

Four edits and one appended section. In the Event Types table, replace

```markdown
| `player_enter` | A player enters the mob's room | `UserId` = player |
```

with

```markdown
| `player_enter` | A player enters the mob's room | `UserId` = player |
| `item_idle` | Item trees only: once a round from the item tick (`hooks.ItemRoundTick`, lighting 5e) | `RoomId` = the item's room; the subject is `ctx.Item` |
```

In the Environment table, replace the start of the `time_of_day` row

```markdown
| `time_of_day` | `period` ("day" or "night") OR `range` ("`<start>-<end>`", 24h format, e.g., `"9-17"`; wraps midnight when start > end). When both set, `range` takes precedence. |
```

with

```markdown
| `time_of_day` | `period` ("day", "night", or "after_dusk" with `hours` N) OR `range` ("`<start>-<end>`", 24h format, e.g., `"9-17"`; wraps midnight when start > end). When both set, `range` takes precedence. `after_dusk` (lighting 5e) is true from the unrounded night boundary until N game hours later (`gametime.GameDate.HoursAfterDusk`); a missing or non-positive `hours` logs once and fails. |
```

and in that same row's last cell replace `always Success — both log a warning once.` with `always Success; both log a warning once.` (the row is rewritten, so it loses its dash). Under Entry Points, replace

```markdown
- `TryRoomBehavior(roomId int, event EventContext) bool` — room entry point.
```

with

```markdown
- `TryItemBehavior(event EventContext, subject ItemSubject) bool`: the item
  entry point (lighting 5e); see "Item behaviour trees" below.
- `TryRoomBehavior(roomId int, event EventContext) bool` — room entry point.
```

In the Files table, replace

```markdown
| Misc | `archetype_shift.go`, `room_state.go` |
```

with

```markdown
| Misc | `archetype_shift.go`, `room_state.go` |
| Items (lighting 5e) | `item_engine.go`, `item_state.go`, `conditions_item.go`, `actions_item_light.go` |
```

Append at the end of the file:

```markdown

## Item behaviour trees (lighting 5e, item behaviour slice 1)

Items are the third subject of the engine, beside mobs and rooms (spec
`docs/superpowers/specs/completed/2026-10-05-item-behaviour-foundation-design.md`).

- **Files.** Named trees `behaviors/items/<name>.yaml` (`GetItemTreePath`),
  the archetype pattern: several items share one tree. An item names its tree
  with `behavior:` (`items.ItemSpec.Behavior`); the tree belongs to the
  TEMPLATE, never an instance's override spec. `ListTreeFiles` lists them as
  kind `item` and never reads `behaviors/items` as a mob zone.
- **Boot.** `ValidateItemBehaviors()` (main.go, after items load) loads and
  compiles every named tree and returns one error naming every item whose
  tree is missing, does not compile, or writes light on an adjustable worn
  condition; main.go panics on it (X12, the `voice_id` precedent).
- **Compile.** Item trees compile under root label `item`
  (`LoadItemTreeFromFile` / `LoadItemTreeFromBytes`), so their decorator keys
  never meet a mob's (`root`) or archetype's (`arch`). An item tree may name
  only the item-safe allowlist (`itemSafeConditions`, `itemSafeActions` in
  `loader.go`): `time_of_day`, `round_mod`, `random_chance`, `state_equals`,
  `state_greater_than`, `holder_asleep`, `worn`, `in_combat`; `set_state`,
  `increment_state`, `decrement_state`, `set_light`, `pulse_light`; every
  decorator and composite. Anything else refuses with its path. The item-only
  nodes (`itemOnlyNodes`) refuse in a mob or room tree.
- **Subject.** `EvalContext.Item *ItemSubject{UUID, ItemId, UserId,
  MobInstanceId, RoomId, Slot, OnFloor}`; `Slot` is the `characters.Worn`
  `AllSlots` key, empty in a backpack or on a floor. For an item, `MobState`
  is the item's own state and `MobId` / `InstanceId` are 0.
- **`TryItemBehavior(event, subject)`** resolves the tree, then the holder
  (player, mob, or a loaded room for a floor item; gone: false, the round is
  skipped), evaluates, and returns true on Success. A panic in a node is
  recovered and logged once per tree and node (`EvalContext.node`); the item
  does nothing that round.
- **State** (`item_state.go`): `map[uuid.UUID]` behind an RWMutex,
  `EnsureItemBTreeState` (marks the visit round), `EvictItemBTreeState`,
  `EvictUnseenItemBTreeStates(round)` (the tick evicts what it did not reach
  this round). Not persisted; a UUID is re-minted on every load, so state
  resets on restart, login and mob instance restore.
- **Nodes.** `holder_asleep` reads the Sleeping flag through
  `actions.TargetAsleep`, the shop sleep gate's predicate (owner ruling R2).
  `set_light` takes `level: full | off | <n>` (an unquoted `off` reads as
  YAML false and still means off). `pulse_light` takes `min`, `max`,
  `period_rounds` (at least 2): `PulseLightValue`, a triangle wave from the
  round count. Both write a WORN item's light and darkness records through
  their existing trimmed-output state (`SetLightOutput`, `LightOff`,
  `LightFull`; one mechanism with the trim, owner ruling R3), skip adjustable
  records (the trim owns those), and write only on a change; a FIXTURE's
  output goes to `internal/itemlight` (`full` fails there: a fixture names a
  number); anything else fails and changes nothing. `TreeWritesLight` reports
  a tree that names either.
- **Tests.** `LoadItemTreeForTest`, `ItemBTreeStateForTest` and
  `ResetItemBTreeStatesForTest` let other packages drive the engine.
```

- [ ] **Step 3: The other packages' `context.md` (one appended section each)**

Append to `internal/items/context.md`:

```markdown

## Item behaviour: `behavior:`, fixtures and the holder index (lighting 5e)

- **`ItemSpec.Behavior`** (`behavior:`) names a tree under
  `behaviors/items/` (see `internal/behaviortree/context.md`). It belongs to
  the template: read it through `Item.HasBehavior()` or `GetItemSpec`, never
  an instance's override spec. The boot refuses a name that does not resolve.
- **`ItemSpec.Fixture`** (`fixture: light | darkness`, constants
  `FixtureLight` and `FixtureDarkness`, checked by `Validate`) fixes an item
  to a room's floor. `Item.IsFixture()` reads the template. Every
  floor-removal path refuses a fixture (`actions.ErrFixture`, the `get all`
  and mob sweeps, `steal`, the mob idle floor equip), "On the Ground" and the
  GMCP room contents leave it out, and its tree's output lights the room
  through `internal/itemlight`. Ship a fixture `not_salable`: it is never
  loot.
- **The holder index** (`item_behavior.go`) holds HOLDERS, not items: mob
  instances (`IndexMobHolder`, `DropMobHolder`, `MobHolders`) and rooms
  (`IndexRoomHolder`, `DropRoomHolder`, `RoomHolders`) that may hold a treed
  item. The item tick (`hooks.ItemRoundTick`) visits only these plus every
  online player. `OnRoomHolderIndexed`, set by `internal/hooks`, is called
  once each time a room enters the index, outside the lock, so its fixtures
  are lit before anyone reads the room. `ResetHolderIndexForTest` swaps in an
  empty index.
```

Append to `internal/rooms/context.md`:

```markdown

## Fixtures and the item index (lighting 5e)

- **Composition.** `composeLightExcluding` reads the room's fixture outputs
  from `internal/itemlight` (`Terms(r.RoomId)`) and composes through
  `composeWithFixtures(cfg, celestial, skyFilter, carried, dark,
  fixtureLight, fixtureDark)`; `composeWith` is the same with no fixtures,
  so every older test reads the same terms. A lit light fixture is one term
  in the light combine, a darkness fixture one in the darkness combine.
  Fixtures never trim; a carried adjustable light trims against them.
- **`LightTerms.Fixture`** is the combine of the lit light fixtures (Absent
  when none) and never sets `Carried`; **`LightTerms.CarriedLight`** is the
  combine of carried light alone (Absent when none). `Darkened` still means a
  CARRIED darkness; a darkness fixture moves `Dark`. `internal/lightnotice`
  reads all three to name a cause.
- **The item index.** A room joins `items`' holder index when a treed item
  lands on its floor (`AddItem`, Prepare's spawn append), when it loads into
  memory holding one (`addRoomToMemory`), and at boot through
  `IndexTreedFloors()` for rooms loaded before item specs. It leaves on
  unload (`removeRoomFromMemory`, which also clears its fixture outputs).
  `RemoveItem` clears the removed item's fixture output. Stashed items are
  never visited and never index a room.
```

Append to `internal/lightnotice/context.md`:

```markdown

## Fixtures and scheduled carried light (lighting 5e, spec X5)

`attribute` names `carried` when a carried light arrives or leaves
(`Carried`) OR changes strength while lit (`LightTerms.CarriedLight`: a
lantern its schedule dims, a sunstone fading, a second light joining one
already here; all of these used to fall to `eyes`), and `lamp` when the
room's lamp OR its light fixtures move (`LightTerms.Fixture`: the North
Gate's arch lantern at dusk and dawn). A darkness fixture moves `Dark` and
reads `darkness`, checked first. Cadence is unchanged: an idle player learns
of dusk on their next command, move or combat round. `lamp.yaml`'s header
names fixtures; `carried.yaml`'s two darker "is gone" lines read "fades"
(owner ruling R8).
```

Append to `internal/gametime/context.md`:

```markdown

## `GameDate.HoursAfterDusk` (lighting 5e)

`HoursAfterDusk()` is how many game hours have passed since dusk, in
[0, 24), read from the same unrounded night boundary `Night` uses (the
world's latitude and the day of the year; kept in two unexported fields so
the struct's serialised shape does not change). Before dusk it reads more
than the day's daylight, so it never reads "just after dusk" in the
afternoon. The behaviour-tree condition `time_of_day` reads it for
`period: after_dusk` with `hours: N` (the sunstone's faint hour).
```

Append to `internal/hooks/context.md`:

```markdown

## The item tick (`NewRound_ItemRoundTick.go`, lighting 5e)

`ItemRoundTick`, a `NewRound` listener, fires `item_idle` through
`behaviortree.TryItemBehavior` once a round for every item with a behaviour
tree that it reaches: every online player's worn slots then backpack; the
indexed mobs' worn slots then backpack (`items.MobHolders`); the indexed
rooms' floors (`items.RoomHolders`). Never a world walk. A visited holder
with nothing treed left drops out of the index; a room no longer loaded loses
its fixture outputs. Container contents are not visited. Last,
`behaviortree.EvictUnseenItemBTreeStates` drops the state of every item not
visited this round. `RegisterListeners` also sets `items.OnRoomHolderIndexed
= EvaluateRoomFixtures`, so a room's fixtures are lit the moment the room
joins the index (a fixture spawned, or a room loaded holding one).
`HandleJoin`'s companion gear restore re-indexes the companion
(`IndexTreedItems`). `EquipBestFloorItem` skips fixtures.
```

Append to `internal/characters/context.md`:

```markdown

## The item tick's holder index (lighting 5e)

`IndexTreedItem(i)` enters this character's MOB in `items`' holder index
when the item names a behaviour tree; `IndexTreedItems()` does it when any
worn or backpack item does. `StoreItem` (before its capacity check, so a
refused or fallback store is covered too), `Wear` and the two spills in
`RemoveFromBody` call the first; mob spawn and the companion gear restore
call the second. A player is never indexed: the tick walks every online
player. Equipping a light still resets its records to full (`ResetLight`), so
a scheduled light is full until the next item tick.
```

Append to `internal/actions/context.md`:

```markdown

## Fixtures (lighting 5e)

`TakeFloorItem` refuses a fixture (`items.Item.IsFixture`) with `ErrFixture`
right after `ErrTooDark`, for every taker: a player's `get`, a mob's, a
companion's, a scavenger's. `steal`'s floor branch only ever takes a
household bauble, and the player command names a fixture as fixed in place
before it gets there. `TargetAsleep` is also the predicate the item
condition `holder_asleep` reads (owner ruling R2).
```

Append to `internal/usercommands/context.md`:

```markdown

## Fixtures (lighting 5e, ruling R9)

A fixture is part of the room. `look` leaves it out of "On the Ground" and
renders `descriptions/fixtures` right after the room description: one line
each, "The <Name> is lit." or "is unlit." from `itemlight.Lit`, coloured by
the description's own day/dark rule (`fixtureLines`). `look <name>` still
finds it ("You look at the <Name> here:"). `get <fixture>` and `steal
<fixture>` answer "The <Name> is fixed in place." (`fixedInPlace`); `get all`
passes fixtures without a word, and `get all <name>` that only matches a
fixture says it is fixed in place.
```

Append to `internal/mobcommands/context.md`:

```markdown

## Fixtures (lighting 5e)

A mob's `get all` skips fixtures; a single `get` of one is refused by
`actions.TakeFloorItem` (`ErrFixture`).
```

Append to `internal/mobs/context.md`:

```markdown

## The item tick (lighting 5e)

A spawned mob holding an item with a behaviour tree (a keeper's Oil Lantern,
from the template or the saved instance) joins `items`' holder index at the
end of `newMobByIdInternal` (`Character.IndexTreedItems`), so the item tick
reaches it.
```

Append to `modules/gmcp/context.md`:

```markdown

## Room.Info.Contents.Items leaves out fixtures (lighting 5e)

The GMCP twin of "On the Ground" skips fixtures (`items.Item.IsFixture`,
owner ruling R9): a fixture is part of the room, shown in its look.
```

- [ ] **Step 4: `docs/schemas/behavior.md`**

Replace

```markdown
| `time_of_day` | `period` ("day" or "night") | Checks in-game time of day. |
```

with

```markdown
| `time_of_day` | `period` ("day", "night", or "after_dusk" with `hours` N), or `range` ("`<start>-<end>`") | Checks in-game time of day. `after_dusk` is true from the night boundary until N game hours later (lighting 5e). |
```

and append at the end of the file:

```markdown

## Item Behavior Trees (lighting 5e)

Items are the third tree subject. A tree lives at
`behaviors/items/<name>.yaml` and an item names it with `behavior: <name>`
in its YAML; several items may share one. The boot fails on a name with no
file or a tree that does not compile. Item trees fire `item_idle` once a
round for every treed item a player wears or carries, a mob wears or
carries, or a room has on its floor.

An item tree may name only these nodes (anything else refuses at load):

| Kind | Nodes |
|---|---|
| Conditions | `time_of_day`, `round_mod`, `random_chance`, `state_equals`, `state_greater_than`, `holder_asleep`, `worn`, `in_combat` |
| Actions | `set_state`, `increment_state`, `decrement_state`, `set_light`, `pulse_light` |
| Decorators | all |

| Node | Params | Description |
|---|---|---|
| `holder_asleep` | none | The player or mob holding the item has the Sleeping flag. |
| `worn` | none | The item is in an equipment slot. |
| `in_combat` | none | The holder is in combat. |
| `set_light` | `level`: `full`, `"off"`, or a number | A worn light's record at full strength, off, or that strength; a fixture's output (a number or off). Fails on anything else. |
| `pulse_light` | `min`, `max`, `period_rounds` (at least 2) | A triangle wave from `min` to `max` and back over the period, from the round count. |

The item-only nodes (`holder_asleep`, `worn`, `in_combat`, `set_light`,
`pulse_light`) refuse in a mob or room tree. A tree that writes light may
not sit on an item whose worn light is `adjustable` (the trim owns it). A
fixture (`fixture: light` or `darkness` on the item) cannot be taken off the
floor; a pulsing fixture must stay inside one light band
(`item_behaviour_guard_test.go`).
```

- [ ] **Step 5: Patch notes**

Add at the top of `docs/PATCH_NOTES.md`, directly under `# DOGMud Patch Notes` and its blank line, above the newest entry (`## 2026-10-05: Veyra's secrets` since #406 merged, F31). Player-facing, no numbers, no dashes, 80 columns; the heading takes the date the PR merges:

```markdown
## 2026-10-05: Lights that keep time

- Some lights now keep their own hours. The lantern on the arch at
  Stillwater's North Gate is lit at dusk and snuffed at first light.
- A shopkeeper's oil lantern goes dark while they sleep and is lit again
  when they wake. So is yours, if you sleep with one in hand.
- The stone in the Rift Chamber's wall pulses with a slow, faint light.
- A light that is part of a place is fixed there: nobody can take it or
  steal it. Look around and you will see whether it is lit.
- Enchanter Rane in Stillwater sells the sunstone, a light that drinks the
  sun. It glows by day, keeps a faint glow for a while after dusk, and is
  dark through the night.
- When a light you carry dims, or a second light joins it, the message now
  says so instead of blaming your eyes.

```

- [ ] **Step 6: Audit the docs**

Run: `python tools/context_md_audit.py > "$TMP/5e-audit.txt"; grep -E "^(internal/(itemlight|behaviortree|items|rooms|lightnotice|hooks|gametime|characters|actions|usercommands|mobcommands|mobs)|modules/gmcp) " "$TMP/5e-audit.txt"`
Expected: no output (the grep exits 1; run it standalone). The report's package list is the same 13 pre-existing phantom packages as on `origin/master` (compare a run there if it lists anything else).

Search every added line for dashes: `git diff -U0 origin/master -- . | grep "^+" | grep -c "—\|–"` (standalone). Expected: `0`.

- [ ] **Step 7: Full gate**

```bash
gofmt -l internal/ modules/ .
go vet ./...
go build ./...
go test ./... -count=1
golangci-lint run --new-from-merge-base=origin/master
```

Expected: `gofmt` and `vet` print nothing; every package `ok` (130 on the dry run, 23 with no test files); lint `0 issues.` (a stale-cache warning naming another worktree's path is not an issue). Known flake: `internal/playtestrun` can hang under load; re-run it alone (`go test ./internal/playtestrun/ -count=1`) before calling it a failure. If `lighting_parity_golden_test.go` or `lighting_daycycle_golden_test.go` fails, run `python tools/lighting_golden_diff.py` and explain the move before anything is re-recorded. A failure unrelated to this slice is compared against a detached master worktree (`git worktree add --detach C:/tmp/dogmud-itembeh1-base origin/master`), reported, and not fixed here; remove that worktree afterwards.

- [ ] **Step 8: Race run in Docker**

```bash
docker build --target test -f provisioning/Dockerfile -t dogmud-5e-test .
docker run --rm dogmud-5e-test > "$TMP/5e-race.log" 2>&1
grep -E '^(--- FAIL|FAIL|WARNING: DATA RACE)' "$TMP/5e-race.log"
```

Expected: no `WARNING: DATA RACE`; the only failures are the two tests that shell out to `git`, which has no repository inside the image (`TestNoStringOrDataSaysBuff` in the root package and `TestU10DoneWhen_DeadPathsStayDead` in `internal/combat`), with their `FAIL` package lines. Any other failure is a finding (Task 13's guard (a) shells out to `go`, which the test image has). The item index, `itemlight` and item state are each behind their own mutex; a race here is a real one. Remove the image afterwards (`docker rmi dogmud-5e-test`).

- [ ] **Step 9: Boot check on private ports**

The owner runs their own server on this machine: bind only private ports, start the process hidden, and stop only the PID this step started (never by name or port). The boot is what catches a content panic (`ValidateItemBehaviors`, the strict item YAML probe).

PowerShell:

```powershell
$S = $env:TMP
"Network.TelnetPort: [33357]`nNetwork.LocalPort: 9957`nNetwork.HttpPort: 8057`nNetwork.HttpsPort: 0`nNetwork.AIPort: 0`n" | Set-Content -NoNewline "$S\5e-boot-overrides.yaml"
go build -o "$S\5e-boot-check.exe" .
$env:CONFIG_PATH = "$S\5e-boot-overrides.yaml"; $env:LOG_NOCOLOR = "1"
$p = Start-Process -FilePath "$S\5e-boot-check.exe" -WorkingDirectory (Get-Location) -WindowStyle Hidden -RedirectStandardOutput "$S\5e-boot.log" -RedirectStandardError "$S\5e-boot.err" -PassThru
$deadline = (Get-Date).AddSeconds(150)
while ((Get-Date) -lt $deadline) { $t = (Get-Content "$S\5e-boot.log","$S\5e-boot.err" -Raw -ErrorAction SilentlyContinue) -join ""; if ($t -match "Server Ready|panic:") { break }; Start-Sleep -Seconds 3 }
$all = Get-Content "$S\5e-boot.log","$S\5e-boot.err"
"ready=" + ($all | Select-String "Server Ready").Count
"panic=" + ($all | Select-String "^panic:|goroutine \d+ \[running\]|runtime error").Count
$all | Select-String "ValidateZoneConsi"
if ((Get-Process -Id $p.Id -ErrorAction SilentlyContinue).Path -like "*5e-boot-check.exe") { Stop-Process -Id $p.Id -Confirm:$false }
Remove-Item Env:CONFIG_PATH, Env:LOG_NOCOLOR
```

Expected: `ready=1`, `panic=0`, `mapper.ValidateZoneConsi errors=0 warnings=0 mode=panic`. The boot writes living state under `_datafiles/world/dogmud` (gitignored); `git status --short` must show nothing new outside this slice's files.

- [ ] **Step 10: Commit**

```bash
git add internal/behaviortree/context.md internal/items/context.md internal/rooms/context.md internal/lightnotice/context.md internal/gametime/context.md internal/hooks/context.md internal/characters/context.md internal/actions/context.md internal/usercommands/context.md internal/mobcommands/context.md internal/mobs/context.md modules/gmcp/context.md docs/schemas/behavior.md docs/PATCH_NOTES.md
git commit -m "docs(lighting-5e): context.md, behaviour schema and patch notes for item behaviour slice 1" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 15: Adversarial playtest and PR

**Model:** opus (judgment on live findings; the adversarial content gate).

- [ ] **Step 1: Playtest (the spec's procedure, ending with the adversarial content gate)**

Load `dogmud-playtesting` and `playtest-scenario` and follow them: an EPHEMERAL scenario file in the scratchpad (not the repo), `--checkout C:/tmp/dogmud-itembeh1`, private ports only, never touch the owner's server and never stop a process this session did not start; reports are gitignored, so findings go to memory. The harness bans the `admin` profile from `playtestrun scenario`: give the operator admin by flipping `role: admin` in a normal profile's save, and use a distinct profile per actor. `locate` needs the full mob name (`locate Blacksmith Kerra`). Every actor quotes lines verbatim.

Roster: an operator (admin; `server night` and `server day` set the clock to the round before the next dusk or dawn, spec fact T8, `usercommands/admin.server.go:97-107`; a game hour is 37.5 rounds, about two and a half real minutes), four plain players.

1. *North Gate (4111).* `server day`, then two agents walk in from 4109 and report the light notice, the band, the room look (expect `The Arch Lantern is unlit.` after the description, no lantern under "On the Ground"), and `look lantern` (`You look at the Arch Lantern here:` and its description). `server night`; each types a command and reports the `lamp` notice (e.g. `A lamp flickers to life; shapes come out of the dark.` on a moonless night) and `The Arch Lantern is lit.`
2. *Rift Chamber (5000).* Two agents stand there for twenty rounds, typing `look` every few rounds: the band stays steady and no notice fires; `The Rift Stone is lit.` every time.
3. *Kerra's lantern.* One agent waits in Blacksmith Kerra's sleeping room (5101) across 22:00 (`server night`, then wait out the hours): report the lantern going out (the room's band and any `carried` notice), `list` refused for sleep (`... is fast asleep. You could make some noise -- try shout wake up.`), then `shout wake up` and report the light back on the next round.
4. *The sunstone.* One agent buys a sunstone from Enchanter Rane (`list`, `buy sunstone`; report the price and description), equips it in a cave by day (bright), and stays past dusk: report the faint glow, then dark an hour later, with each notice verbatim (a `carried` line, never `eyes`).
5. *Sleeping with a lantern.* One agent holds an Oil Lantern, sleeps in a dark room, then wakes: report the lantern dark on the next look before waking and lit again the round after (owner: fine).
6. *Fixed in place.* Every agent tries `get lantern`, `get all`, `get all lantern`, `steal lantern` (an agent with skullduggery) in 4111 and the same with `stone` in 5000; report every line verbatim (expect `The Arch Lantern is fixed in place.`; bare `get all` silent about it) and that nothing left the floor. Also `sell lantern` (they cannot hold it).

Then run the adversarial content gate (`dogmud-authoring-content`): a fresh character reads every new line (the two fixture descriptions, the sunstone's description and `look sunstone`, the room-look fixture lines, `fixed in place`, the two reworded `carried` notices if they can be triggered) as a confused player would and reports anything unclear, over 80 columns, carrying a number where the copy rules forbid one, or with a dash.

Extract every finding to memory: a `project-lighting-5e-playtest-findings-2026-10-05.md` topic file and one pointer line in `MEMORY.md`, added with the Edit tool (never a Python read-modify-write). Fix what the findings show before the PR; a fix is a normal TDD step with its own commit.

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feature/item-behaviour-slice1
gh pr create --repo pruuk/DOGMud --base master --head feature/item-behaviour-slice1 --title "feat(lighting): 5e slice 1, item behaviour engine and scheduled light" --body-file "$TMP/5e-pr-body.md"
```

Body (`$TMP/5e-pr-body.md`): what ships (items as the third tree subject, the item tick and index, the five nodes and `after_dusk`, scheduled light through the record's existing trim state, fixtures, the content), one line each; the owner rulings it implements (R2, R3, R4, R7, R8, R9, R10) and the two later answers (the lantern relights on the first round after waking; the scheduled-off state survives a restart until the next round); the twelve items under "Where the spec could not be implemented as written", leading with the Rift Stone's singular name and the "The {Name} is lit." line, and guard (a)'s field classification (owner ruling 2026-10-05), both as ruled; the "Player-visible lines that change" table; the guards and goldens (the R8 two-line move, the new fixture day-cycle golden, the parity and day-cycle goldens unmoved, the shop guard now running 1512 lantern samples); gate results with counts; the race run; the boot check; the playtest outcome and where its findings live in memory; the spec and plan paths; references to #365 (this slice) and #372 (the lighting arc), closing nothing (slices 2 and 3 remain under #365). End with:

```
🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

Read back the URL `gh` prints and confirm it says `pruuk/DOGMud`. The owner runs any deploy; do not deploy, and do not nag about deploying. After merge, `git worktree remove C:/tmp/dogmud-itembeh1` and drop its name from the handoff memory.

---

## Self-review

- **Spec coverage.** Rule 1 (named trees, `behavior:`, root label `item`, boot failure, `ListTreeFiles` kind `item`): T3, T4, T6 boot index, T12. Rule 2 (`ItemSubject`, `MobState` is the item's state): T4. Rule 3 (`TryItemBehavior`, gone holder skips, recover): T4. Rule 4 (per-UUID state, `EnsureItemBTreeState`, `EvictItemBTreeState`, eviction after an unvisited round, not persisted): T4, T7. Rule 5 (the tick's order, the index of holders, entry at spawn, `StoreItem`, `Wear`, spills, companion restore, `AddItem`, the `Prepare` append, instance load and boot; drop when empty; first-visit evaluation; containers not visited): T3, T6, T7. Rule 6 (`item_idle` added with its dispatch site; the other events stay with slices 2 and 3): T7. Rule 7 (`time_of_day after_dusk` R7/X2, `holder_asleep` R2, `worn`, `in_combat`, `set_light`, `pulse_light`): T1, T5. Rule 8 (allowlist with paths, item-only nodes refused elsewhere, one writer per record): T4, T5, T13(b,e). Rule 9 (full / n / off through `LightFull` / `SetLightOutput`, write on change, adjustable skipped, entry trim cannot relight, `ResetLight` on equip is full until the next tick, backpack fails): T5, T12. Rule 10 (`fixture:`, `internal/itemlight`, fixture terms, `LightTerms.Fixture`, never trim, refused at every floor removal, off-floor drops output, the look line after the description under its colour rule, `look <name>`): T2, T3, T6, T8, T10, T11. Rule 11 (deterministic triangle wave; band-straddle guard at 72 samples with the 20-to-60 probe): T5, T13(d). Rule 12 (attribution, `lamp.yaml` header, cadence unchanged): T9. Rule 13 (no new room line): nothing added. Rule 14 (boot panic, skipped round, log once per tree and node): T4. Rule 15 (a) to (e): T13, (a) as the owner ruled it (every exported `ItemSpec` field classified once as behaviour or data; behaviour reads in hooks only at twelve allowlisted sites, no read counts). Content table (arch lantern 55 in 4111 with the noun moved, the Rift Stone 56 in 5000 pulsing 20 to 36, the keeper lantern tree on 40038, the sunstone 20099 with condition 134 at 46, faint 30 for an hour after dusk, value 40, `vendor_categories: [enchanting]`, sold by Rane, the ladder row): T12. X8 fixture day-cycle: T13. Shop guard: T13. R8: T9. R9 siblings: T11. Testing and gates: T1 to T14. Playtest and PR: T15. Out of scope and untouched: slices 2 and 3, 5803 and street lighting (R4), sleeping schedules (#404), saving tree state, container trees, player-lit fixtures, the builder editing surface (#367).
- **Beyond the spec, stated above.** `items.OnRoomHolderIndexed` and `hooks.EvaluateRoomFixtures`; `rooms.IndexTreedFloors`; `behaviortree.TreeWritesLight`, `PulseLightValue`, `LoadItemTreeForTest`, `ItemBTreeStateForTest`, `ResetItemBTreeStatesForTest`, `EvalContext.node`; `itemlight.ResetForTest`, `items.ResetHolderIndexForTest`; `composeWithFixtures`; the GMCP and tab-completion siblings; `look <fixture>` "here"; `get all <name>` naming a fixture; the separated `litAsleep` report in the shop guard.
- **Names across tasks.** `ItemSubject{UUID, ItemId, UserId, MobInstanceId, RoomId, Slot, OnFloor}`, `TryItemBehavior`, `GetItemTreePath`, `ValidateItemBehaviors`, `LoadItemTreeFromFile`, `LoadItemTreeFromBytes`, `itemRootLabel`, `itemSafeConditions`, `itemSafeActions`, `itemOnlyNodes`, `checkNodeSubject`, `EnsureItemBTreeState`, `EvictUnseenItemBTreeStates` (T4); `condHolderAsleep`, `condWorn`, `condInCombat`, `actSetLight`, `actPulseLight`, `PulseLightValue`, `TreeWritesLight` (T5); `items.FixtureLight`, `FixtureDarkness`, `HasBehavior`, `IsFixture`, `IndexMobHolder`, `IndexRoomHolder`, `DropMobHolder`, `DropRoomHolder`, `MobHolders`, `RoomHolders`, `OnRoomHolderIndexed` (T3); `IndexTreedItem`, `IndexTreedItems`, `IndexTreedFloors` (T6); `ItemRoundTick`, `EvaluateRoomFixtures` (T7); `LightTerms.Fixture`, `CarriedLight`, `composeWithFixtures` (T8); `ErrFixture`, `fixedInPlace` (T10); `fixtureLines`, `fixtureLine`, `descriptions/fixtures` (T11); trees `dusk_to_dawn`, `rift_pulse`, `keeper_lantern`, `sunstone` (T12); `itemSpecBehaviourFields`, `itemSpecDataFields`, `itemSpecBehaviourMethods`, `itemSpecBehaviourReadSites`, `itemSpecReadsIn`, `TestEveryItemSpecFieldIsClassifiedExactlyOnce`, `TestHooksReadItemBehaviourFieldsOnlyAtAllowlistedSites` (T13). Test helpers and ids (`engineProbe*` 9900 to 9903, `lightProbe*` 9921 to 9927, `seedFixtureRoom`, `sentTo`, `pinAfterDuskClock`, `loadItemBehaviourWorld`, item ids 999950 to 999994) collide with nothing in their packages (each package built and ran on the dry run).
- **Placeholder scan.** No TBD, no "similar to". The only fills are the playtest's live quotes and the PR body's live results, which only the executor can know.
- **Order.** T1 before T5 and T12 (`after_dusk`, `pinAfterDuskClock`); T2 before T5, T6, T8 (`itemlight`); T3 before T4 (`Behavior`, `Fixture`) and T6 (the index); T4 before T5 (allowlist, engine); T5 before T7 (`set_light` in the tick test) and T12; T6 before T7 (indexing on `AddItem`); T8 before T9 (`Fixture`, `CarriedLight`); T10 before T11 (`seedFixtureRoom`); T12 before T13 (content the guards walk). Each task's end state passed its packages on the dry run, and the whole suite passed after T13.

