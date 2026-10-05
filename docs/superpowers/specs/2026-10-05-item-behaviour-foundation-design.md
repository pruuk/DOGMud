# Item behaviour foundation, scheduled light first (lighting 5e)

Date: 2026-10-05. Issue: #365 (lighting arc epic #372). Arc: graded room
lighting, plan 5, slice 5e, widened by the owner on 2026-10-05 into a small
item behaviour foundation. Parent: `docs/superpowers/specs/2026-09-26-lighting-plan5-light-as-play-design.md`
(arc table row 5e, `:59`). Format follows `2026-09-30-lighting-plan5d-darkness-design.md`.
Design approved by the owner on 2026-10-05; this spec verifies it against
source and records where the source disagrees with it.

One spec, three plans and three PRs:

| Slice | Delivers | Closes |
|---|---|---|
| 1 | Items as the third behaviour-tree subject, the item tick, light nodes, fixtures, scheduled carried light; lamp-posts, a pulsing rune stone, keeper lanterns, a sunstone | #365 slice 1 |
| 2 | `speak(pool)`, chattiness levels, a per-listener ambient cap; Aegis and Blackrazor rebuilt as trees | #222 |
| 3 | `proc(effect, params)`; the four Pinnacle proc items rebuilt as trees | #223 (owner decision O1) |

## Facts verified against source (2026-10-05, master `7e0748145`)

Every row was read at `7e0748145` in the worktree `C:/tmp/dogmud-itembeh`.
Negative rows name the search and a positive hit proving the same search could
match. Knob rows give the Go default AND the shipped value; "absent" means the
key is not in the `git show HEAD:_datafiles/config.yaml` blob, so the Go
default is live.

### The behaviour-tree engine

| # | Fact | Where |
|---|---|---|
| E1 | `Node` is one method, `Evaluate(ctx *EvalContext) Result`; results `Success`, `Failure`, `Running` | `internal/behaviortree/types.go:9-15,37-40` |
| E2 | `EvalContext{Event EventContext; MobState *BehaviorState; MobId; InstanceId; RoomId int; MobName string; Intercepted bool; SoftTarget state.ActorRef}`. There is no item field | `types.go:43-63` |
| E3 | `EventContext` carries `EventType`, `UserId`, `MobId`, `Text`, `ItemId`, `ItemUUID uuid.UUID`, `RoomId`, `Extra`, `Command`, `Rest`, `Direction` | `types.go:18-34` |
| E4 | `NodeDef{Type, Event, Children, Check, Do, Mod, Note, Child, Params (inline)}`; `TreeDef{Notes, Tree, GoalWeights, DefaultGoals}` | `types.go:76-86,100-105` |
| E5 | Decorators: `cooldown` (`rounds`), `repeat` (`times`), `invert`, `random` (`percent`), `delay` (`rounds`). Cooldown and delay state live in the subject's `BehaviorState` under the positional key `<path>_cooldown` / `<path>_delay` | `loader.go:154-198`; `decorators.go:7-91` |
| E6 | `CooldownDecorator` blocks while `round - lastRun < Rounds` and records the round only when the child returns `Success`; `RandomDecorator` always calls `util.Rand(100)` and fails when `>= Percent` | `decorators.go:14-25,65-70` |
| E7 | Per-mob and room trees compile under root label `root`, archetypes under `arch`, so a composed mob's two trees cannot share a positional key | `loader.go:54,67` |
| E8 | `BehaviorState` is a `map[string]any` with `Get/GetString/GetInt/Set/Delete` | `state.go:5-44` |
| E9 | Room tree state is a package map `roomStates map[int]*BehaviorState` behind an RWMutex, `EnsureRoomBTreeState`, `EvictRoomBTreeState`; process lifetime, not persisted | `room_state.go:5-46` |
| E10 | `TryRoomBehavior(roomId int, event EventContext) bool` resolves an ephemeral room to its template's tree, lazy-loads `behaviors/rooms/<zone>/<roomId>.yaml`, negative-caches a missing file, builds `EvalContext{Event, MobState, RoomId}` | `helpers.go:95-145`; path `:74-79` |
| E11 | Mob tree path `behaviors/<zone>/<mobId>-<name>.yaml`; archetype path `behaviors/archetypes/<name>.yaml`, named by `behavior_archetype` on the mob | `helpers.go:51-57,86-90`; `internal/mobs/mobs.go:159` |
| E12 | A missing per-mob tree is negative-cached silently; a missing archetype warns once. Neither fails boot | `helpers.go:152-169,176-193` |
| E13 | The boot-failing precedent for a declared reference is `voice_id` (panic on a dangling id) and `schedule_id` (panic) | `internal/itemvoices/itemvoices.go:146-153`; `internal/mobs/mobs.go:1458-1467` |
| E14 | `TryMobBehavior` evaluates with no recover; delayed actions run through `safeExecuteDelayed`, which recovers | `helpers.go:228-285`; `engine.go:320` |
| E15 | `room_idle` fires from `UserRoundTick` once per round per room **with players** (`rooms.GetRoomsWithPlayers()`) | `internal/hooks/NewRound_UserRoundTick.go:148-166` |
| E16 | `KnownBehaviorEvents` holds 15 events; `events_test.go` fails on an event fired as an `EventType: "..."` literal but not listed, and on a listed event nothing fires | `events.go:16-32`; `events_test.go:17-63` |
| E17 | 87 action and 66 condition registrations, `map[string]ActionFunc` / `map[string]ConditionFunc` by name only: no subject metadata | `actions.go:12`; `conditions.go:9`; grep `Registry["` |
| E18 | `mobs.GetInstance(ctx.InstanceId)` appears 84 times in 25 non-test files (45 in action files, 39 in condition files); 77 are followed within three lines by `return Failure`. `actSay`/`actEmote` return `Failure` on a nil mob | grep; `actions_dialogue.go:56-89` |
| E19 | `time_of_day` already does hour ranges (`range: "18-6"`, wraps midnight, `[start, end)`) and `period: day/night` from `gametime.IsNight()`; it reads no mob | `conditions_state.go:73-129`; registered `conditions.go:21` |
| E20 | Subject-free conditions exist: `round_mod`, `random_chance`, `state_equals`, `state_greater_than`; `mob_in_combat` reads the mob | `conditions_state.go:174,185`; `conditions_mob.go:9` |
| E21 | `ListTreeFiles` treats every `behaviors/` subdirectory except `archetypes` and `rooms` as a mob zone; `TestListTreeFiles_FindsAllThreeKinds` pins three kinds | `save.go:262`; `save_test.go:254` |
| E22 | `TestRoundTrip_MarshalFixedPointEveryLiveBehaviorFile` and `TestEventVocabulary_CoversAllLiveBehaviorYAML` walk every file under `behaviors/` | `roundtrip_test.go:74-79`; `events_test.go:67-83` |
| E23 | No item subject exists: grep `TryItemBehavior\|item_idle` over `*.go` finds nothing; the same grep shape for `TryRoomBehavior` finds `helpers.go:95` | grep |
| E24 | `behaviortree` imports `rooms` (`helpers.go:12`) and `actions` (e.g. `actions_combat.go`); grep for the `behaviortree` import in `internal/rooms`, `items`, `characters`, `conditions`, `lightnotice`, `messaging` finds nothing, the same grep in `internal/hooks` finds `CombatPhase_BtreeEvents.go` | grep |

### Items

| # | Fact | Where |
|---|---|---|
| I1 | `items.Item` fields: `ItemId`, `UUID` (`yaml:"-"`), `Blob`, `Uses`, `Loaded`, `DropChance`, `LastUsedRound`, `CraftedRound`, `CraftSkill`, `BottleMultiplier`, `MakerName`, `Spec` (`overrides`), `Affixed`, `Uncursed`, `Enchantments`, `Adjectives`, `EnchantTier/Uses/Type/Baseline`, `ReservePool`, `StashedBy`, `DetuneMigrated`, `Bauble*`, and `tempDataStore` ("Not saved to disk") | `internal/items/items.go:41-70` |
| I2 | `UUID` is `[16]byte` (comparable, a valid map key), never saved, and minted by `Validate()` whenever nil: every load (restart, login, mob instance restore) gives an item a new identity; `buy` also re-mints | `internal/uuid/uuid.go:34`; `items.go:43,224-227`; `internal/actions/buy.go:661` |
| I3 | `Item` is a value type: slots, backpacks and floors hold copies; `GetSpec()` returns `*i.Spec` whole when an override exists, else the template | `items.go:329-341` |
| I4 | ItemSpec behaviour fields: `WornConditionIds` (`wornconditionids`), `Procs` (`procs`), `MutationTickInterval/Chance/RarityFloor`, `VoiceId` (`voice_id`), `HungerRounds`, `HungerDrainPct`, `TauntPull` (`taunt_pull`) | `internal/items/itemspec.go:274-297` |
| I5 | `ItemProc{Trigger, Chance, CooldownRounds, Effect, Params map[string]float64}`; triggers `on_hit, on_kill, on_block, on_grapple, on_spell_hit`; effects `lifesteal, steal_pool, aoe_stun, apply_condition`; validated at load; `ProcsFor` | `itemspec.go:256-271,769-772,807` |
| I6 | Shipped readers of these fields in non-test `internal/hooks`: `VoiceId` 6, `HungerRounds` 6, `PreservesContents` 3, `HungerDrainPct` 2, `MutationTickInterval` 2, `AmbientPotions`, `MutationTickChance`, `MutationRarityFloor`, `ProcsFor`, `TauntPull` 1 each, all in `item_procs.go`, `MobDeath_ItemProcs.go`, `pinnacle_tick.go` | grep |
| I7 | Content: `procs:` on 40183 Blackrazor (on_hit lifesteal 100%), 40185 Aegis (on_block aoe_stun 10%, cd 20), 40186 Thornwall Harness (on_grapple bleed 50%, cd 5), 40189 Staff of the Hollow Choir (on_spell_hit steal_pool 100%, cd 3); `voice_id` on 40183, 40185; `hunger_rounds: 50` on 40183; `taunt_pull: true` on 40185; `mutation_tick_interval: 300` on 40187 | `items/materials-40000/` |
| I8 | The web builder's item editor reads and writes `Procs`, `VoiceId`, `TauntPull` | `modules/gmcp/gmcp.Item.go:102-107,201-206,260-264`; `_datafiles/html/public/static/js/items.js` |
| I9 | Unknown keys in item YAML fail the boot smoke test's strict probe, so a deleted Go field needs its YAML keys removed first | `internal/fileloader/fileloader.go:58-66,96` |

### Procs today

| # | Fact | Where |
|---|---|---|
| P1 | `dispatchItemProcs(trigger, owner, other, room, damage)`: per proc, `procGateOpen` (feature gate, cooldown, chance), switch on effect, `markProcCooldown` only when the effect executed | `internal/hooks/item_procs.go:107-140` |
| P2 | Seven call sites in four files: `MobDeath_ItemProcs.go:24` (on_kill), `NewRound_DoCombat_unified.go:166` (on_hit), `:180` (on_block), `Position_GrappleTick.go:408-409` (on_grapple, both sides), `spell_effects.go:244,357` (on_spell_hit) | grep |
| P3 | `procBearingItems`: on_hit/on_kill/on_spell_hit read the weapon slot, on_block the offhand, on_grapple the body | `item_procs.go:145-155` |
| P4 | The roll: `p.Chance < 100 && util.Rand(100) >= p.Chance` fails; at chance 100 no random number is drawn | `item_procs.go:72` |
| P5 | Cooldown key `pinnacle_proc_cd_<itemId>_<procIdx>` in the OWNER's MiscData (per template, persisted with the character); stores the round it may fire again | `item_procs.go:33-35,65-71,79-84` |
| P6 | `ItemProcsEnabled` gates procs: Go bool, shipped `true` | `internal/configs/config.gameplay.go:30`; blob `:381` |
| P7 | `util.Rand` is `rand.Intn` on the `math/rand` global; `go 1.25.0`; grep `rand.Seed(\|rand.New(` over non-test `internal` finds only `gametime/zodiac.go:240`. No test seam seeds `util.Rand` | `internal/util/util.go:204-209`; `go.mod:3`; grep |

### Voices and the Pinnacle tick today

| # | Fact | Where |
|---|---|---|
| V1 | `pinnacleUserTick(user, room)` runs once per **player** per round from `UserRoundTick`, gated by `PinnacleItemsEnabled` (shipped `true`); it walks `GetAllWornItems()` once for the sub-ticks; nothing like it runs for mobs | `pinnacle_tick.go:44-57`; `NewRound_UserRoundTick.go:396`; `config.gameplay.go:29`; blob `:377` |
| V2 | `itemvoices` loads `_datafiles/world/dogmud/itemvoices/*.yaml` (`aegis`, `blackrazor`); `VoiceSpec{VoiceId, Lines map[event][]string}`; allowed events: `on_equip, on_unequip, on_kill, on_idle, on_hunger_warning, on_hunger_feeding, on_taunt, on_grudge` | `itemvoices.go:23-41,134-153` |
| V3 | Fired: `on_idle`, `on_taunt`, `on_hunger_warning` (chosen per round by `pickVoiceEvent`: in combat, else past 3/4 of the hunger window, else idle), `on_kill` (`tryEmitVoice`, weapon slot only), `on_hunger_feeding` (`tickHunger`, with a guaranteed fallback line). Never fired: `on_equip`, `on_unequip`, `on_grudge` (grep of the quoted names over non-test Go finds only `itemvoices.go:24,25,31`) | `pinnacle_tick.go:184-189,397-409,449`; `MobDeath_ItemProcs.go:32`; #222 |
| V4 | `on_taunt` and `on_hunger_warning` have no dispatch site of their own: they are states read by `pickVoiceEvent` | `pinnacle_tick.go:397-409` |
| V5 | `tickVoices`: per-bearer cooldown in MiscData `pinnacle_voice_next_round`, one `util.Rand(100)` roll against `SentientChatterChancePct` per round once a line exists, first worn voiced item in slot order wins, one line per round | `pinnacle_tick.go:425-460` |
| V6 | `TauntPull` fires only when an `on_taunt` LINE was emitted, so it shares the chatter cooldown and chance | `pinnacle_tick.go:449-457,469-484` |
| V7 | `SentientChatterCooldownRounds` has a second reader: it paces the hunger feeding line (`pinnacle_hunger_msg_next_round`) | `pinnacle_tick.go:184-188` |
| V8 | `emitVoiceLine` sends the holder `<Item> says, "..."` by `user.SendText`, and the room `<Name>'s <Item> mutters, "..."` by `room.SendTextVisual` with the raw `user.Character.Name` | `pinnacle_tick.go:513-535` |
| V9 | NPC speech goes through `Room.SendTextHidingNames(cat, txt, names, messaging.HideSpeakerNames)`: heard by all, each name hidden at that listener's sight, never deafen-filtered (owner ruling 6, sight gates 5b) | `internal/rooms/rooms.go:290-300`, comment `:225-227` |
| V10 | No physical deafness exists: grep `Deaf` over `internal/conditions`, `messaging`, `users` finds only the moderation flag `Deafened` | `internal/users/userrecord.go:50` |
| V11 | `itemvoices.golden` snapshots both voices' pools through `narration.Picker` (`LineWith`) | `internal/narration/testdata/stores/itemvoices.golden`; `itemvoices.go:87-93`; `snapshot_test.go:55,101` |
| V12 | Equip and remove for players and mobs both go through `actions.equipItem` and `actions.removeWorn`, which queue `events.EquipmentChange{UserId, MobInstanceId, ItemsWorn, ItemsRemoved}`; `behaviortree` imports `actions` (E24), so `actions` cannot call into trees | `internal/actions/remove_equip.go:46,95-96,155,181`; `internal/events/eventtypes.go:253-260` |

### Where items are, and who walks them

| # | Fact | Where |
|---|---|---|
| W1 | Nothing walks room floor items or mob inventories per round for behaviour: `IdleMobs` walks every mob instance each round but reads no item (grep `Items\|Equipment\|GetSpec` in it finds 0); `Room.RoundTick` decays corpses and sweeps untaken baubles only | `NewRound_IdleMobs.go:27-44`; `rooms.go:2770-2821` |
| W2 | A mob scans its room's floor when it idles (`EquipBestFloorItem`) and removes the item it takes | `MobIdle_HandleIdleMobs.go:213`; `mob_equip_best_floor_item.go:29,65` |
| W3 | Room floor items are `Room.Items []items.Item` (`yaml:"items"`, NOT `instance:"skip"`, so instance saves keep them) | `rooms.go:113` |
| W4 | Room YAML places a floor item through `spawninfo: - itemid: N`; `Room.Prepare` appends it directly to `r.Items` when `FindOnFloor("!N")` misses; `Prepare` runs when a player enters an empty room and on respawn | `rooms.go:1184-1195`; `internal/rooms/spawninfo.go:14`; `roommanager.go:431-433`; `NewRound_HandleRespawns.go:22` |
| W5 | Floor adds: `Room.AddItem` and the `Prepare` append. Character adds: `Character.StoreItem`, plus direct appends at `actions/steal_pocket.go:214`, `characters/worn.go:626,666`, `hooks/NewRound_AutoHeal.go:153`, `hooks/PlayerSpawn_HandleJoin.go:160`, `mobs/crafter.go:603`, `mobs/mobs.go:688` | `rooms.go:1338-1348`; `characters/inventory.go:169-225`; grep |
| W6 | No item is untakeable today: grep `untakeable\|nopickup\|immovable\|fixed in place` over non-test `internal` finds only grapple immovability. `actions.TakeFloorItem` refuses darkness, `exploding`, and a household bauble (`ErrHouseholdBauble`); `get all` filters household baubles in `takeableOnFloor`. The only precedent for a fixed object is a room tree intercepting `get` (`context.md`, room 113); item 13 Mosaic Map is placed by no room | `internal/actions/get.go:14,42-62`; `usercommands/get.go:811-826` |
| W7 | Floor removal sites: `actions/get.go:60` (player and mob take), `actions/steal.go:917`, `hooks/mob_equip_best_floor_item.go:65`, `actions/search.go:287` (stash), and `usercommands/look.go:203,726,751` (invalid-item cleanup only) | grep |
| W8 | `look` lists every floor item under "On the Ground:" | `usercommands/look.go:722-770`; `templates/descriptions/ontheground.template` |
| W9 | The `light` equipment slot is the carried light (`Worn.Light`, `yaml:"light"`); worn-item conditions come only from worn items, so a light in the backpack has no record and gives no light | `characters/worn.go:34,67`; `characters/conditions.go:249-251` |

### Light

| # | Fact | Where |
|---|---|---|
| L1 | Record fields `LightTrim LightTrim` (`lighttrim`), `LightOutput float64` (`lightoutput`), `Hooded bool` (`hooded`), all saved | `internal/conditions/conditions.go:41-43` |
| L2 | `LightMax(spec)` reads `light_strength` or `darkness_strength`; `LightNow(spec)` is false when expired, hooded, off or strengthless, else `min(LightOutput, max)` when trimmed, `max` when full | `internal/conditions/light.go:24-69` |
| L3 | `ResetLight()` sets full, output 0, hood open; equipping a `type: light` item calls it on every record of its `WornConditionIds` | `light.go:87-89`; `characters/worn.go:592-600` |
| L4 | `LightAndDarknessSources()` is every unexpired light or darkness record in held order | `light.go:126-135` |
| L5 | `carriedTerms(exclude)` walks only `r.mobs` then `r.players`, appending each record's `LightNow` to `light` or `dark` | `internal/rooms/lighting.go:185-214` |
| L6 | `composeWith`: sky, the room lamp, every carried light in one `Combine` (`Light`); carried darkness in a second `Combine` (`Dark`); `Carried = len(carried) > 0`, `Darkened = len(dark) > 0`; `Raw = Light (0 if Absent) - Dark`; `Level` rounds and clamps to [-100, 100] | `lighting.go:118-179` |
| L7 | `LightTerms{Level, Raw, Light, Dark, Sky, SkyFilter, Lamp int, HasLamp, Carried, Darkened}`; the only readers of `Carried`, `HasLamp`, `Darkened` outside `rooms` are `lightnotice.attribute` | `lighting.go:62-90`; grep |
| L8 | `lampValue()`: the room's `Lamp *int` (`lamp`, `instance:"skip"`) if set, else the biome's `lamp`; an authored 0 is returned as `(0, true)` and joins the combine as a term | `lighting.go:250-258`; `rooms.go:102`; `biomes.go:97-102` |
| L9 | Biome lamps: `city_thoroughfare` 52, `interior` 50, `ether` 60, `spiderweb` 45, `city_backstreet` 35. Room overrides ship as `lamp: 38` (16 rooms) and `lamp: 50` (11); no room ships `lamp: 0` | `biomes/*.yaml`; grep |
| L10 | `TrimLightFor(c)` trims only the bearer's adjustable, unhooded records, against `composeLightExcluding`; its callers are `MoveToRoom` and `Room.AddMob` | `internal/rooms/light_trim.go:35-93`; `roommanager.go:474`; `rooms.go:1316` |
| L11 | `EmitsLight()` is `len(LightTerms()) > 0` from `LightNow`, so any change to `LightNow` reaches sneak, the `lit` adjective and waking sleepers | `characters/light.go:7-36` |
| L12 | Band edges for a normal observer: `LightBlindBelow` 25, `LightDimBelow` 50 (both absent, Go defaults), `LightDazzleAbove` 75 (shipped 75); `LightDoublingStep` 8 (absent) | `config.balance.go:1150-1158,1196`; blob `:946` |

### Notices and the band

| # | Fact | Where |
|---|---|---|
| N1 | Causes: `movement, carried, lamp, weather, sky, eyes, darkness`, one YAML per cause under `narration/light-notices/` | `internal/lightnotice/store.go:24-34,49` |
| N2 | `attribute`: movement, the eyes counterfactual, then `Darkened` or `Dark` moved, then `a.Carried != b.Carried` (presence only), then `HasLamp`/`Lamp` changed, weather, sky, else eyes. A second carried light arriving where one already is moves no checked term and falls to `eyes` | `tracker.go:149-176` |
| N3 | `Check(user, trigger)` runs on every command (`usercommands.go:331`), each combat round (`NewRound_DoCombat.go:164`), each move and login (`LightNotice_Triggers.go:22,29`), and after equip, remove, hood, unhood, cancel; it queues `events.SightBandChanged` when the band moves (the `Char.Sight` GMCP field and the Game-window border) | `tracker.go:200-237`; grep; `eventtypes.go:444` |
| N4 | `lamp.yaml` authors all six transitions and says "nothing reaches these lines until runtime lamps arrive (scheduled light sources)" | `_datafiles/world/dogmud/narration/light-notices/lamp.yaml:1-3` |
| N5 | `carried.yaml` darker lines read "The carried light is gone ..." | `light-notices/carried.yaml` |
| N6 | `Room.VisualSnapshot()` and `SendTextVisualToSnapshot` exist for judging a line against the room before a change (owner rule, memory `feedback-narration-judged-before-resolution.md`; #220 open) | `rooms.go:424,443` |

### Time and schedules

| # | Fact | Where |
|---|---|---|
| T1 | `GameDate{..., Hour24, MinuteFloat, Night, DayStart, NightStart}`; `Night` uses the fractional boundary derived from `WorldLatitude` and the day of year; `DayStart`/`NightStart` are rounded display values | `internal/gametime/gametime.go:95-108,260-305` |
| T2 | `IsNight()` is `GetDate().Night` | `gametime.go:191-194` |
| T3 | `WorldLatitude` absent, coerced to 46.5; `RoundsPerDay` shipped 900 (37.5 rounds per game hour) | `config.balance.lighting.go:135-136`; blob `:215` |
| T4 | `events.DayNightCycle{IsSunrise, Day, Month, Year, Time}` is queued by `CheckNewDay` when `Night` flips; its only listener plays the sunrise/sunset splash | `eventtypes.go:380-388`; `NewRound_CheckNewDay.go:21-29`; `hooks.go:97` |
| T5 | `ScheduleSegment{Start, End, TargetRoom, Activity ("" \| craft \| sleeping \| patrol), PatrolId, IdleCommands}`; `CurrentSegment(hour24)` | `internal/mobs/schedule.go:15-55` |
| T6 | The runner sends `sleep` when the segment is `sleeping` and the mob is at its target; a woken mob stays awake through `ScheduleWakeGraceRounds` (absent, coerced to 50) while still inside the sleeping segment (`SuppressSleepIdle`) | `NewRound_IdleMobs_schedule.go:105-121,197-200`; `config.balance.mobs.go:35-36` |
| T7 | `ShopClosedForSleep` and `TargetAsleep` read the `Sleeping` condition flag, not the schedule | `internal/actions/sleeping_target.go:24-26,78-98` |
| T8 | `server day` / `server night` set the clock for a playtest | `usercommands/admin.server.go:98-107` |

### Content anchors

| # | Fact | Where |
|---|---|---|
| C1 | 5803 "The Processional, North End", `city_thoroughfare` (lamp 52 at every hour), describes civic-lamps "lit each evening by a lamplighter"; noun `civic lamps`; idle line "The civic-lamps are unlit in the daytime ..." (fires at any hour) | `rooms/new_plymouth_merchant/5803.yaml` |
| C2 | 4111 "North Gate", `plains`, no lamp; nouns `arch` and `lantern` ("lit at dusk and snuffed at first light") | `rooms/stillwater/4111.yaml` |
| C3 | 22 mob files fill the `light` slot: 21 with 40038 Oil Lantern, 272 with 20098. Nine of the 21 have a schedule, and all nine have a `sleeping` segment: 9492 Mistress Odell the Victualler (23-5), 9531 Wren the Ostler (23-6), 9333 Master Halvard (21-5), 9303 Dunmar Wells (21-6), 9607 Jeweler Vurl (22-6), 348 Miller Bram (20-5), 9588 Enchanter Rane (22-6), 97 Blacksmith Kerra (22-6), 98 Apothecary Voss (21-6). The other twelve have no schedule and never sleep | grep `^    light:`; `schedules/*/` |
| C4 | 40038 is also stocked by shops and handed out by quest 14; `TestShippedLightItemsMatchTheLadder` pins it at 52 | `items/shipped_light_items_test.go:32`; merchant spec |
| C5 | A saved mob instance replaces the template's whole equipment block on spawn, so swapping the item in a keeper's slot does not reach a keeper with an instance file | merchant spec 2.6; `internal/mobs/mobs.go:641-643` |
| C6 | Merchant-lights ruling: "shopkeepers bring their own light while awake (behaviour), unless their schedule puts them to sleep"; the equipped lantern was the content stopgap, "the behaviour version ... stays a follow-up" | `specs/completed/2026-09-30-merchant-shops-and-lights-design.md:222-262` |
| C7 | `TestEveryShopkeeperCanTradeAtNightWhileAwake` spawns each real keeper and samples 72 hours; a keeper in a `sleeping` (or patrol) segment is skipped "since ShopClosedForSleep refuses before the sight gate"; it runs no item tick | `shop_night_trade_guard_test.go:34-74,207-230` |
| C8 | `lighting_daycycle.golden` loads biomes, rooms, conditions, mutators (no items, no `Prepare`, no tick) and records every room's `LightLevel()` at 12 moments; 5803 reads 54/54/66/54/53/53/72 ..., 4111 34/34/63/31/21/22/70 ... | `lighting_daycycle_golden_test.go:101-140`; `testdata/lighting_daycycle.golden` |
| C9 | 5000 "The Rift Chamber", `dungeon` (skylight 0, no biome lamp), room `lamp: 38`: "dark stone etched in geometric patterns that pulse with faint light", runes on the archway; spawns 315 Sable (no shop) | `rooms/thornwall_city/5000.yaml`; `biomes/dungeon.yaml` |
| C10 | 9588 Enchanter Rane (Stillwater) sells chrysalis reagents | `mobs/stillwater/9588-enchanter_rane.yaml:51-63` |
| C11 | "sunstone" is used once, as a bauble material (Polished Sunstone Falcon) | `bauble-corpus.yaml` |
| C13 | Shipped `vendor_categories` values: blacksmithing 117, alchemy 82, tailoring 75, jewelcrafting 50, cooking 49, enchanting 19 | grep over `items/` |
| C12 | Behaviour unification epic #374 lists "Foragers and shopkeepers carrying light (#207 gave lanterns to 21 keepers)" as remaining | `gh issue view 374` |

### Free ids (NEW)

| # | Id | Evidence |
|---|---|---|
| F1 | Item **20099** (sunstone) in `armor-20000/light/` | `id_inventory.py --type items`: `light 20096-20098 ... next-in-zone 20099`; grep `^itemid: 20099$` finds nothing, the same grep for 20098 finds the Umbral Lantern |
| F2 | Items **55, 56, 57** (fixtures) in `other-0/`, beside the Mosaic Map | `id_inventory.py`: `other-0 ... gaps 55-899`; grep `^itemid: (55\|56\|57)$` finds nothing, the same for 54 finds `54-sorens_iron_pin.yaml` |
| F3 | Condition **134** (sunstone glow) | `id_inventory.py --alloc conditions 1`: "reserve IDs 134-134 ... past current global max (133)"; grep `^conditionid: 13[45]$` finds nothing, the same for 133 finds `133-phantom_heat_sense.yaml` |

## The approved design (owner, 2026-10-05; rulings)

A1. **Shape A.** Items are a third subject of the behaviour-tree engine beside
mobs and rooms: item trees in YAML under `behaviors/items/`, an item field
naming its tree, `TryItemBehavior` mirroring `TryRoomBehavior`, and an
`EvalContext` item handle (instance plus where it is).

A2. One item tick per round over items that HAVE a tree (worn, carried,
mob-held, floor) via an index, never a whole-world walk; it fires `item_idle`.
Events `on_equip`, `on_unequip`, `on_hit`, `on_block`, `on_kill`,
`on_grapple`, `on_spell_hit`, `on_taunt`, `on_hunger_warning` fire from
today's sites.

A3. Per-instance tree state in a package map keyed by item instance, not
persisted; a restart resets cooldowns and schedules recompute from the hour.

A4. Nodes: `hour_between`, `is_night`, `holder_asleep`, `worn`, `in_combat`;
`set_light(n | off)`, `pulse_light(min, max, period_rounds)`, `speak(pool)`,
`proc(effect, params)`. Mob-only actions refuse an item subject, and a
validator rejects an item tree naming one.

A5. **Light.** Worn and carried lights keep the record model; the schedule
writes a new `Scheduled` value, `LightNow = min(full, Scheduled, trim)`,
`set_light(off)` is dark like a hood, an equip lights at full until the next
tick; two items sharing one light condition on one bearer share a record.
**Fixtures are items**: untakeable floor items whose output lives in tree
state, one term per lit fixture, a fixture may be a darkness, **fixtures do
not trim**. A pulse may not straddle a band edge. Fixture changes notify as
`lamp`, scheduled carried light as `carried`; cadence unchanged.

A6. **Content slice 1**: lamp-posts in 5803 and 4111, dusk to dawn; one
pulsing rune stone; the #207 keeper lanterns snuffed while their schedule
says they sleep; one sunstone.

A7. **Voices (slice 2)**: `speak(pool)` with pools authored with the tree,
replacing `itemvoices/`; shared narration senders; `on_equip`/`on_unequip`
fire (closes #222); `on_grudge` stays out. Each tree sets quiet, normal or
chatty (cooldown plus chance in `config.yaml`); a per-listener ambient cap;
event lines bypass the cap but keep the item's cooldown. Retire `tickVoices`,
the two `SentientChatter*` knobs and their MiscData keys; hunger and taunt
mechanics stay on the Pinnacle tick.

A8. **Procs (slice 3)**: `proc(effect, params)` wraps today's Go effects;
chance and cooldown move to `random`/`cooldown`; dispatched from today's call
sites for items in today's slots; a plain random roll; cooldown per instance
unless the owner rules otherwise on #223; retire `dispatchItemProcs`,
`procs:` and the cooldown keys.

A9. Each slice proves the old behaviour first (goldens under a seeded random)
before deleting the old path. Guards and validators as listed in the rules.
Failure modes: a missing tree fails boot; a gone holder or room skips the
round; a tree error is logged and the item does nothing.

## Where the facts push back

Each item names the facts, the smallest change, and whether it needs the
owner (then it is repeated under "Open owner decisions").

**X1. `hour_between` and `is_night` already exist as `time_of_day` (E19).**
`time_of_day range: "18-6"` and `period: night` are subject-free. House rule 1
says reuse. **Change:** item trees use `time_of_day`; no new hour or night
node. Owner decision O7.

**X2. "An hour after dusk" is not an hour range (T1).** Dusk moves with the
season (latitude-derived boundary), so the sunstone's faint hour cannot be a
fixed `range`. **Change:** `time_of_day` gains `period: after_dusk` with
`hours: N`, true from the fractional night boundary until N game hours later,
read from a new `gametime` helper over the same unrounded boundary `Night`
uses.

**X3. `holder_asleep` from the schedule disagrees with the shop gate
(T6, T7, C7).** A keeper roused inside a sleeping segment stays awake for at
least 50 rounds; a schedule reading would snuff an awake keeper who can
trade, and the shop gate keys on the `Sleeping` flag. **Change:**
`holder_asleep` reads `actions.TargetAsleep` (the flag), the same predicate as
`ShopClosedForSleep`; it then works for any holder. Owner decision O2.

**X4. A tree belongs to an item template, and 40038 is everyone's lantern
(C3 to C5).** A tree on 40038 snuffs every sleeping bearer: the nine
scheduled keepers, never the twelve unscheduled ones (they never sleep), and
any player who sleeps with a lantern in the light slot. A separate keeper
lantern item would reach no keeper that has a saved instance (C5).
**Recommendation:** the tree on 40038 itself, keyed on X3's flag. Owner
decision O3.

**X5. Notice attribution cannot see these changes (N2, L6, L7).** `carried`
keys on presence only, so a lantern dimming while still lit, or a second
light arriving, reads as `eyes`; `lamp` keys on the room's static `Lamp` int,
and a fixture term appended to the carried slice would set `Carried` and be
blamed on a carried light. **Change:** fixtures get their own term
(`LightTerms.Fixture`, the combined fixture light, Absent when none) that does
not set `Carried`; `LightTerms.CarriedLight` is the combine of carried light
alone; `attribute` checks `Carried` or `CarriedLight` moved for `carried`, and
`HasLamp`, `Lamp` or `Fixture` moved for `lamp`. A darkness fixture joins the
darkness combine and reports as `darkness` (checked first). This also fixes
the second-light case, a sibling path.

**X6. `rooms` cannot read `behaviortree` state (E24).** The composition lives
in `rooms`, which `behaviortree` imports. **Change:** fixture output lives in
a new leaf package `internal/itemlight` (per room, per item UUID, light or
darkness) that `behaviortree` writes and `rooms` reads in O(1) per room. The
tree's other state stays in the `behaviortree` map.

**X7. 5803 is already lamp-lit at every hour (C1, L8, L9).** Its biome lamp 52
burns by day too ("Lamps burn here all night"); a dusk-to-dawn fixture on top
double-counts at night and leaves the lamps lit at noon. **Change:** 5803
gets the room override `lamp: 0`, and `lampValue` reads an authored 0 as no
lamp term (no room ships `lamp: 0` today, L9). Night stays at today's level
(fixture 52 for lamp 52); the day loses a lamp the sky dwarfs. The other 97
thoroughfare rooms keep their static lamp. Owner decision O4.

**X8. The day-cycle golden cannot see fixtures (C8).** It loads no items and
runs no `Prepare` or tick, so 4111 does not move and 5803 moves only because
of X7. **Change:** slice 1 adds a fixture day-cycle test that loads items,
prepares the fixture rooms and runs the item tick at the golden's twelve
samples; the existing golden's 5803 rows are re-recorded with the move
explained.

**X9. A per-instance identity resets on every load, not only on restart
(I2).** Light and chatter do not care. Proc cooldowns do: today they persist
in the bearer's MiscData (P5); per-instance tree state would let a relog clear
an Aegis stun cooldown. Owner decision O1, with the options there.

**X10. No untakeable mechanism exists, and floors are removed from in five
places (W6, W7, W8).** **Change:** ItemSpec gains `fixture: light |
darkness`. A fixture is refused by `actions.TakeFloorItem` (new `ErrFixture`
beside `ErrHouseholdBauble`, covering player and mob `get`),
`takeableOnFloor` (`get all`), the floor branch of `steal`, and
`EquipBestFloorItem`; `look` leaves fixtures out of "On the Ground" (the room
text describes them) and `look <name>` still finds them. Owner decision O9
for the ground list.

**X11. Nothing marks an action as mob-only (E17, E18).** 77 of 84 mob lookups
fail cleanly with no mob, but the rest were not read and room actions act on
`ctx.RoomId`. **Change:** the validator is an allowlist of item-safe nodes,
not a blocklist (Rule 8).

**X12. A missing mob tree does not fail boot (E12, E13).** **Change:** an
item's `behavior:` resolves at boot after items load and panics on a missing
or uncompilable file, the `voice_id` precedent; that is the design's intent.

**X13. `ListTreeFiles` would read `behaviors/items/` as a mob zone (E21).**
**Change:** skip `items` there, list item trees as a fourth kind, update the
three-kinds test.

**X14. `on_taunt` and `on_hunger_warning` are states, not events (V4);
`on_hunger_feeding` is an event the design omits (V3).** **Change:** a tree
expresses taunt and hunger warning as `item_idle` branches gated by
`in_combat` and a new `hunger_overdue(fraction)` (reads the Blackrazor anchor
the way `pickVoiceEvent` does), ordered taunt, hunger, idle as today;
`tickHunger` fires `on_hunger_feeding` into the weapon's tree.

**X15. `TauntPull` is triggered by the taunt line (V6).** Moving voices to
trees moves its trigger. **Change:** an item action `taunt_pull` placed after
`speak(taunt)` in the Aegis tree keeps today's pacing exactly, and
`taunt_pull:` is retired. This moves one taunt mechanic off the Pinnacle tick,
against A7's wording. Owner decision O5.

**X16. `SentientChatterCooldownRounds` also paces the hunger feeding line
(V7).** **Change:** a new knob `HungerFeedingLineCooldownRounds` (shipped 20,
today's value) takes that job when the chatter knobs retire.

**X17. Today's voice line leaks the bearer's name (V8) and "deafness" has
nothing to filter (V9, V10).** **Change:** the room line goes through
`SendTextHidingNames` with `HideSpeakerNames` (heard by all, the bearer's name
hidden by each listener's sight); authored item lines are never
deafen-filtered (ruling 6).

**X18. Equip events cannot be fired from `actions` (V12).** **Change:** a
hooks listener on `events.EquipmentChange` fires `on_equip` for `ItemsWorn`
and `on_unequip` for `ItemsRemoved`, players and mobs alike.

**X19. There is no seeded random (P7), and the `random` decorator draws where
today's 100% procs draw nothing (E6, P4).** **Change:** slice 3 adds a test
seam that swaps `util.Rand`'s source; a proc tree omits `random` at 100% so
the sequence of draws, and so the golden, is unchanged. Slice 2's line goldens
use the existing `narration.Picker` seam (V11) plus the same seam for the
chance roll.

**X20. Only players' items speak today (V1).** The item tick also reaches
mob-held items, so a mob wielding the Blackrazor would chatter. Owner
decision O6.

**X21. The web builder edits the fields slices 2 and 3 retire (I8).**
**Change:** those slices remove `Procs`, `VoiceId`, `TauntPull` from
`gmcp.Item.go` and `items.js` (sibling path); editing trees stays #367.

## Open owner decisions

| # | Decision | Recommendation |
|---|---|---|
| O1 | #223: proc cooldown per instance or per template? Per instance resets on every relog (X9) | Per template in the bearer's MiscData, keyed by item id and tree branch: today's behaviour, relog-proof. Per instance would need the item UUID saved (`items.go:43`), a larger change |
| O2 | `holder_asleep`: the `Sleeping` flag or the schedule's segment (X3) | The flag |
| O3 | Tree on 40038 (players' lanterns also go dark while they sleep) or a keeper-only lantern (misses saved instances) (X4) | 40038 |
| O4 | 5803: `lamp: 0` plus "authored 0 is no lamp", or keep the static lamp and add the fixture (X7) | `lamp: 0` |
| O5 | `taunt_pull` as a tree action after the taunt line, or a separate Pinnacle-tick pull with its own knob (X15) | Tree action |
| O6 | Do mob-held sentient items speak (room line only)? (X20) | Yes; the listener cap bounds it |
| O7 | Reuse `time_of_day` instead of new `hour_between` / `is_night` (X1) | Reuse |
| O8 | Reword `carried.yaml`'s two darker lines so a fading light reads right ("fades" for "is gone") | Yes, slice 1 |
| O9 | Leave fixtures out of "On the Ground" (X10) | Yes |

## The rules

### Slice 1: the engine

**Rule 1, item trees.** Named trees `_datafiles/world/dogmud/behaviors/items/<name>.yaml`,
the archetype pattern (E11), so several items share one tree. ItemSpec gains
`Behavior string` (`yaml:"behavior,omitempty"`, the name #367 already uses).
Trees compile under root label `item` (E7). At boot, after items load, every
`behavior:` must resolve and compile or the boot panics (X12). `ListTreeFiles`
lists them as kind `item` (X13).

**Rule 2, the subject.** `EvalContext` gains `Item *ItemSubject{UUID,
ItemId, UserId, MobInstanceId, RoomId, Slot string, OnFloor bool}` (`Slot` is
the `AllSlots` key, empty in a backpack). For an item, `MobState` is the
item's state and `InstanceId`/`MobId` are 0.

**Rule 3, `TryItemBehavior(event EventContext, subject ItemSubject) bool`.**
Mirrors `TryRoomBehavior`: resolve the template's tree, resolve the holder
(gone: return false, the round is skipped), build the context, evaluate under
a recover that logs and returns false (E14). Returns true on `Success`.

**Rule 4, state.** `behaviortree/item_state.go`: `map[uuid.UUID]*BehaviorState`
behind an RWMutex, `EnsureItemBTreeState`, `EvictItemBTreeState`, the
`room_state.go` shape. Not persisted. Identity is `Item.UUID`, which is
re-minted on every load (I2), so state resets on restart, login and mob
instance restore.

**Rule 5, the tick.** `hooks.ItemRoundTick`, a `NewRound` listener, fires
`item_idle` for each treed item it visits, in this order: every online
player's worn slots then backpack (bounded by players, the Pinnacle tick's
precedent); indexed mobs; indexed rooms' floors. The index holds holders, not
items: a mob enters it at spawn and on `StoreItem` / `Wear` of a treed item
and at the direct appends in W5; a room enters it at `Room.AddItem`, the
`Prepare` append and instance load. A visited holder with no treed item left
drops out; an item's state is evicted once a full round passes with no visit
to it, so an item handed from a mob to a player keeps its state. A fixture is evaluated once when
its room is indexed, so a first visit is never dark for a round. Container
contents are not visited.

**Rule 6, events.** Each event is added to `KnownBehaviorEvents` in the slice
that fires it: `item_idle` (slice 1); `on_equip`, `on_unequip` (X18),
`on_kill`, `on_hunger_feeding` (slice 2); `on_hit`, `on_block`, `on_grapple`,
`on_spell_hit` (slice 3, today's `dispatchItemProcs` sites, P2, P3).
`on_taunt` and `on_hunger_warning` become conditions (X14).

**Rule 7, item nodes.** Conditions: `time_of_day` with the new
`period: after_dusk` + `hours` (X1, X2); `holder_asleep` (X3, O2); `worn` (the
item is in an equipment slot); `in_combat` (the holder `IsInCombat()`);
slice 2 adds `hunger_overdue(fraction)`. Actions: `set_light` and
`pulse_light` (slice 1), `speak` and `taunt_pull` (slice 2), `proc` (slice 3).

**Rule 8, the validator.** At compile time an item tree may name only an
allowlist: the item nodes of Rule 7, the subject-free `time_of_day`,
`round_mod`, `random_chance`, `state_equals`, `state_greater_than`,
`set_state`, `increment_state`, `decrement_state`, and every decorator. Any
other node refuses with its path (X11). A mob or room tree naming an item-only
node refuses too.

**Rule 9, scheduled carried light.** `conditions.Condition` gains
`Scheduled float64` and `ScheduledSet bool`, both `yaml:"-"`. `LightNow`
returns `min(full, Scheduled, trimmed)` when set; `Scheduled <= 0` returns
false, like a hood, without touching `Hooded`. `ResetLight` clears it, so an
equip is full until the next tick (A5). `set_light(n | full | off)` and
`pulse_light` on a worn item write every light or darkness record of its
`WornConditionIds`; on an item that is neither worn nor a fixture they return
`Failure` and change nothing. Two items sharing one condition on one bearer
share the record (known limit). The trim keeps solving against `full`
(L10); the schedule only caps.

**Rule 10, fixtures.** ItemSpec gains `Fixture string` (`yaml:"fixture"`,
`light` or `darkness`). A fixture's output (from `set_light`/`pulse_light`)
lives in `internal/itemlight` per room and UUID (X6). `composeWith` adds lit
light fixtures to the light combine and darkness fixtures to the darkness
combine; `LightTerms.Fixture` reports the light fixtures' combine (X5).
Fixtures never trim and are never reset by a bearer. A fixture is refused at
every floor removal (X10). An item moved off a floor (admin) drops its output.

**Rule 11, `pulse_light(min, max, period_rounds)`.** A triangle wave from the
round count, `min` to `max` and back over `period_rounds` (at least 2):
deterministic, no state. Content guard (repo root): for every fixture placed
by `spawninfo` whose tree pulses, the normal observer's band at `min` and at
`max`, through the real room at the shop guard's 72 samples (C7), must agree
at every sample. Content guidance: nightvision moves the edges, so keep a
pulse well inside a band.

**Rule 12, notices.** `attribute` as X5. `lamp.yaml`'s header comment changes
to name fixtures. Cadence unchanged (N3): an idle player learns of dusk on
their next command, move or combat round; `Char.Sight` and the border follow.

**Rule 13, narration order.** Slice 1 adds no room line for a light change
beyond the notices. Any later line that announces a fixture or scheduled
change is judged against a `VisualSnapshot` taken before the change (N6).

**Rule 14, failure modes.** Missing or broken tree file: boot panic (Rule 1).
Holder or room gone: skipped that round (Rule 3). Panic in a node: logged once
per tree and node (the `time_of_day` log-once precedent,
`conditions_state.go:80`), and the item does nothing that round (Rule 3).

**Rule 15, guards.** `item_behaviour_guard_test.go` at the repo root:
(a) the ItemSpec fields read in non-test `internal/hooks` are pinned to I6's
list, and each slice shrinks it; a new field fails ("new item behaviour goes
in a tree"); (b) every `behavior:` resolves; (c) an item whose tree uses
`set_light` or `pulse_light` is `type: light` or a fixture, and an item placed
by `spawninfo` with such a tree is a fixture (the takeable-fixture check);
(d) Rule 11's band check.

### Slice 1: content

| Piece | What | Values |
|---|---|---|
| Civic lamp, item **55** `civic_lamp` | `type: object`, `fixture: light`, `not_salable`, `behavior: dusk_to_dawn`; placed in 5803 by `spawninfo` | `set_light 52` at night, `off` by day; 5803 `lamp: 0` (X7, O4); the `civic lamps` noun moves into the item; the "unlit in the daytime" idle line is replaced by one true at any hour |
| Arch lantern, item **56** `arch_lantern` | Same shape, placed in 4111 | `set_light 52` at night (the oil lantern's rung), `off` by day; the `lantern` noun moves into the item. At the golden's midwinter midnight 4111 goes from 34 (moonlight, shapes) to 54 (faces) |
| Rift stones, item **57** `rift_stones` | `fixture: light`, `behavior: rift_pulse`, placed in 5000, whose text already says the stone "pulse[s] with faint light" (C9); a dungeon, no sky, so the pulse is the same at every hour | `pulse_light 20 36 12`: with the room's lamp 38 the room swings 40 to 45, inside shapes for normal eyes, so no notice fires |
| Keeper lanterns | `behavior: keeper_lantern` on 40038 (O3): `holder_asleep` then `set_light off`, else `set_light full` | Snuffs the nine scheduled keepers of C3 while asleep; the twelve unscheduled keepers never sleep and are unchanged |
| Sunstone, item **20099** | `type: light`, `subtype: wearable`, `wornconditionids: [134]`, `behavior: sunstone`; a chrysalis-grown stone that drinks the sun. Stocked by 9588 Enchanter Rane (C10) as a bare `- itemid:` entry; value 40 (between the hooded lantern's 20 and the Umbral Lantern's 60); `vendor_categories: [enchanting]` (19 shipped items use it) | Condition **134** "Sunstone Glow": secret, `light_strength: 46`, not adjustable, the torch's shape (`triggerrate: 5 real minutes`, `triggercount: 1`). Tree: `period: day` then `full`; `after_dusk hours: 1` then `set_light 30`; else `off` |

Fixtures follow house rule 7: a fixture is not loot, because it cannot be
taken (Rule 10). `TestShippedLightItemsMatchTheLadder` gains the sunstone row.

### Slice 2: voices

**Rule 16, `speak(pool)`.** `TreeDef` gains `speech: map[pool][]string` and
`chatter: quiet | normal | chatty` (default `normal`). `speak` picks a line
through `narration.Render` (the picker seam, V11), sends the holder
`<Item> says, "..."` and the room `<Name>'s <Item> mutters, "..."` through
`SendTextHidingNames` / `HideSpeakerNames` (X17). A mob holder gets the room
line only (O6). Gated by `PinnacleItemsEnabled`.

**Rule 17, pacing.** An `item_idle` line is ambient: it needs the item's
cooldown open (the level's rounds), the level's chance, and every listener in
the room past their own ambient cap; a listener who hears it starts their
cap. An event line (`on_equip`, `on_unequip`, `on_kill`, `on_hunger_feeding`)
bypasses the cap but needs the item's cooldown. Cooldown and cap state live in
memory (item state; a per-listener map in `behaviortree`).

**Rule 18, the two voices as trees.** `behaviors/items/blackrazor.yaml` and
`aegis.yaml` carry the pools of `itemvoices/*.yaml` byte for byte, a
selector of `in_combat` (taunt, then `taunt_pull` for the Aegis, O5),
`hunger_overdue 0.75` (warning), else idle, and the four event branches.
`tickHunger` fires `on_hunger_feeding`; when the tree does not handle it, the
fallback line goes out paced by `HungerFeedingLineCooldownRounds` (X16).

**Rule 19, retire.** `tickVoices`, `pickVoiceEvent`, `tryEmitVoice`,
`emitVoiceLine`, the `itemvoices` package and its data folder and golden, the
`voice_id` and `taunt_pull` fields and keys, the builder fields (X21), the
knobs `SentientChatterCooldownRounds` and `SentientChatterChancePct`, the
MiscData key `pinnacle_voice_next_round` (left inert in saves, the Task 4
precedent). Hunger drain, the mutation drip, reserves and the bandolier stay
on the Pinnacle tick. `on_grudge` is dropped.

### Slice 3: procs

**Rule 20, `proc(effect, params)`.** Wraps `procLifesteal`, `procStealPool`,
`procAoeStun`, `procApplyCondition` unchanged; returns `Success` only when the
effect executed, so a `cooldown` decorator records exactly when
`markProcCooldown` did (E6, P1). Gated by `ItemProcsEnabled`.

**Rule 21, dispatch.** The seven call sites (P2) call `TryItemBehavior` with
the event for the items `procBearingItems` names today (P3). The roll stays a
plain random, not a contest. A proc at chance 100 has no `random` decorator
(X19).

**Rule 22, cooldown store.** As ruled on O1. Under the recommendation, a proc
branch's cooldown is read from and written to the bearer's MiscData keyed by
item id and branch path, so it survives relog as today.

**Rule 23, retire.** `dispatchItemProcs`, `procGateOpen`, `markProcCooldown`,
`procBearingItems` (folded into dispatch), `ItemProc`, `procs:` keys, the
`pinnacle_proc_cd_*` keys (inert in saves), the builder fields (X21).

## Player-visible lines that change

| Where | Today | After | Slice |
|---|---|---|---|
| `lamp` notices (N4) | Unreachable | Fire when a fixture crosses a band, e.g. 4111 at dusk on a moonless night | 1 |
| `carried` notices (N5) | Arrival or departure only; a second light reads as `eyes` | Also a keeper's lantern going out, a sunstone fading; O8 rewords the two darker lines to "fades" | 1 |
| "On the Ground" | Lists every floor item | Leaves fixtures out (O9) | 1 |
| `get` / `steal` a fixture | No such item | New refusal: "The {item} is fixed in place." | 1 |
| 5803 idle line, nouns (C1) | "unlit in the daytime" at any hour | A line true at any hour; noun text moves into item 55 | 1 |
| 4111 noun `lantern` (C2) | Room noun | Item 56's description | 1 |
| Sunstone, condition 134, three fixtures | None | New descriptions | 1 |
| Item room line | `<Name>'s <Item> mutters, "..."`, sight only, raw name | Heard by all, name hidden by sight (X17) | 2 |
| `on_equip` / `on_unequip` | Never fire (#222) | The authored 10 and 11 lines fire | 2 |
| Mob-held voiced items | Silent | Room line (O6) | 2 |

Every new line: hard-wrapped at 80 columns, no raw numbers, no em or en
dashes, plain English (`dogmud-player-copy`). Lines an item speaks may use
character voice. No semicolons in anything spoken.

## Config knobs

| Knob | What | Proposed shipped | Slice |
|---|---|---|---|
| `ItemChatterQuietCooldownRounds` / `ChancePct` | Quiet tree pacing | 40 / 10 | 2 |
| `ItemChatterNormalCooldownRounds` / `ChancePct` | Normal tree pacing (today's values, so parity holds) | 20 / 15 | 2 |
| `ItemChatterChattyCooldownRounds` / `ChancePct` | Chatty tree pacing | 10 / 25 | 2 |
| `ItemChatterListenerCapRounds` | One ambient item line per listener per N rounds | 10 | 2 |
| `HungerFeedingLineCooldownRounds` | Paces the hunger feeding line (X16) | 20 | 2 |
| `SentientChatterCooldownRounds`, `SentientChatterChancePct` | Retired | (were 20, 15) | 2 |

Zero is legal: chance 0 silences a level, cooldown 0 allows every round, cap
0 removes the cap; negatives coerce to the default. Slice 1 adds no knob
(fixture and sunstone strengths are per-item content, house rule 2). Slice 3
adds and retires none; `ItemProcsEnabled` and `PinnacleItemsEnabled` stay.
Each knob is written into `config.yaml` from the `git show HEAD:` blob, never
from disk (`dogmud-balance-config`).

## Out of scope

The builder editing surface (#367, under #377). Turning the other 97
thoroughfare rooms' static lamps into fixtures. Snuffing keeper lanterns by
day (the merchant spec's dazzle cost). The mutation drip, hunger drain,
reserves and bandolier mechanics. `on_grudge`. Saving tree state. Trees on
items inside containers. Fixtures a player can light or snuff. The rest of
the behaviour unification arc (#374), which can tick its "shopkeepers
carrying light" line for the scheduled keepers.

## Testing and gates

**Slice 1.**
- Record: `LightNow` is `min(full, Scheduled, trim)`; `Scheduled <= 0` is off
  and leaves `Hooded` alone; `ResetLight` clears it; an equip is full until
  the next tick.
- Composition: a fixture table through `composeWith` (alone, with a lamp,
  with carried light, a darkness fixture); fixtures never set `Carried`.
- Notices: `attribute` names `lamp` for a fixture change, `carried` for a
  dimming lantern and for a second light arriving; each test shown to fail
  against today's `attribute`.
- Engine: `TryItemBehavior` on worn, backpack, mob-held and floor items;
  state per UUID; a panic in a node is logged and skipped; a missing
  `behavior:` panics the boot loader; the allowlist refuses `attack` in an
  item tree and `set_light` in a mob tree; `ListTreeFiles` finds four kinds;
  `events_test` passes with `item_idle`.
- Tick: holders enter and leave the index at every W5 path; a floor fixture
  is lit on its first visit; a gone holder is skipped.
- Fixtures: refused by player `get`, `get all`, mob `get`, `steal` and the
  mob idle floor equip; absent from "On the Ground"; found by `look`.
- `pulse_light` is deterministic; Rule 11's guard refuses a straddling pulse
  (shown by a fixture pulsing 20 to 60 in 5000).
- Shop guard: `TestEveryShopkeeperCanTradeAtNightWhileAwake` also runs each
  keeper's light-slot tree at every sample with the `Sleeping` flag set as the
  schedule would, and still passes; a keeper flagged asleep reads its lantern
  off.
- Goldens: the fixture day-cycle test (X8) is recorded new; the day-cycle
  golden moves only on 5803's rows, explained with
  `tools/lighting_golden_diff.py`; the parity golden does not move;
  `light_notices.golden` moves only for O8.
- Playtest (`playtest-scenario`, multi-agent, ending with the adversarial
  content gate): `server night` and `server day` at 4111 and 5803, reporting
  the band and notices; two agents in 5000 report a steady band; one agent
  waits in Blacksmith Kerra's sleeping room (5101) across 22:00, reports the
  lantern going out and `list` refused for sleep, wakes her with a shout and
  reports the light back; one carries a sunstone into a cave by day and past
  dusk; every agent tries to take, steal and sell a fixture.

**Slice 2.** Before deleting the old path: the tree pools are proved
byte-identical to `itemvoices.golden`; a line golden drives Blackrazor and
Aegis through 200 rounds (idle, combat, hungry, kill) under the picker seam and
X19's seam on both paths and they match. Then: the listener cap holds across
two voiced items and lets event lines through; the bearer's name is hidden in
the dark (shown to fail against today's sender); `on_equip`/`on_unequip` fire
for a player and a mob; `TauntPull` fires only with a taunt line; the feeding
fallback is paced by its knob (pinned with `SetConfigForTest`). Playtest: two
players wearing the Aegis and the Blackrazor and a third listening, in a lit
room and a dark one; equip, remove, fight, go hungry.

**Slice 3.** Before deleting: a proc outcome golden for the four items under
X19's seam (on_hit, on_block, on_grapple, on_spell_hit, on_kill; hits and
misses; cooldown windows) on both paths, identical. Then: a 100% proc draws
no random number; `ItemProcsEnabled` off stops every proc; O1's cooldown
store survives (or does not survive) a relog as ruled. Playtest: the four
items in real fights, an agent relogging mid-cooldown.

**Every slice.** `context.md` for each package touched (`behaviortree`,
`items`, `conditions`, `rooms`, `lightnotice`, `hooks`, the new `itemlight`;
`itemvoices` removed in slice 2), checked with `tools/context_md_audit.py`;
`docs/README.md` rows for new files; patch notes; the full local gate (test,
vet, lint), a `-race` run, and a boot check on private ports.
