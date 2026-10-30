# #317 Ambient Retune Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every unprompted ("ambient") line source its own config knob with 0 as off, roll mob idle chatter and idle movement apart (weighted so every scale at 1.0 keeps today's rates), show a one-line room's line once per visit, cap what each player hears with a per-listener ambient budget, add zone and mob multipliers, and measure the result with switchable telemetry, a report script and two playtests.

**Architecture:** A new leaf package `internal/ambient` is the one gate every counted send site calls (`ambient.Plan`): it charges each listener's budget and returns the players to leave out, which the caller passes to the room sender it already uses, so sight gating, name hiding and categories stay where they are. Mob idle emotes and says reach it through a new mob-only command `ambient <source> <emote|say> <text>` that only the idle path and the gossip tick issue. The split idle roll is a pure function on `mobs.Mob`; zone multipliers are pointer fields on `rooms.ZoneConfig`. Telemetry lives in the same package behind `Balance.AmbientTelemetry`.

**Tech Stack:** Go 1.25, YAML config in `_datafiles/config.yaml`, YAML content under `_datafiles/world/dogmud/`, Python 3 for `tools/ambient_report.py`, the Docker playtest harness (`cmd/playtestrun`, `internal/playtestenv`, external `mudagent`).

**Spec:** `docs/superpowers/specs/2026-10-30-ambient-retune-design.md` (issue #317, last open item under #381)

**Branch / worktree:** branch `feat/317-ambient-retune`, worktree `C:/tmp/dogmud-317-ambient` off `origin/master`. Two more `C:/tmp` folders this plan creates and that must outlive a session until the PR merges: the playtest checkout `C:/tmp/dogmud-317-playtest` (a detached worktree whose `config.yaml` is edited for each run and never committed) and the evidence folder `C:/tmp/dogmud-317-evidence` (server logs and report tables from both runs).

---

## House rules for every executor

- Edit repo files only with the Edit and Write tools. Never write a repo file from the shell (no `>`, `>>`, `sed -i`, Python `open(..., 'w')`): `~/.claude/hooks/guard.py` blocks it. Scratch output goes in your session scratchpad (full path from your system prompt), never `$TMP` or `/tmp`.
- `git add` named paths only. Never `git add -A` / `.`, never `git commit -a`, never `--no-verify`.
- Every `gh` command carries `--repo pruuk/DOGMud`.
- No em or en dashes and no " -- " in prose, comments, player text or commit messages.
- Kill only processes you started, by PID. The owner's own server may be running.
- `-race` cannot run on this machine (no C compiler); CI runs it.
- Test binaries load Go defaults, never `config.yaml` (`dogmud-writing-tests`). Every test whose subject a knob can move pins that knob with `configs.SetConfigForTest`.
- Every task's test gate includes `go test . ./internal/guards/ -count=1` from the worktree root.
- Prove each new test can fail: the "see it fail" step is mandatory, and where a step says "sabotage", break the named line, watch the named test go red for that reason, restore, watch it go green.

---

## Spec deviations found

1. **The Sand Elemental's pool is four entries, not eight.** The spec's facts table says "one `wander` in an 8-entry pool" for mobs 318 and 319. The shipped `idlecommands` of both is four entries: two emotes, one `wander`, one blank (`mobs/instance_planar_oasis/318-sand_elemental.yaml:11-15`; the next four lines are `combatcommands`). Today's idle move chance per round is 25/100 x 0.99 x 1/4, about 6.2%. The parity tests below use the real four-entry pool.
2. **Dropping the 1% forced blank raises rates by exactly 1/0.99 (about 1.01%), not "under 1%".** Today's rate is 0.99 of the new one for every class. The formula test pins that exact ratio; the seeded run allows 3%.
3. **Telemetry also counts tracking steps and path steps.** The spec counts "mob moves" from "the idle-movement path", but its own Oasis gate asks whether "the royals' tracking or goal-planner moves are what read as too fast". The four royals have no `idlecommands` at all (`320-elemental_king.yaml`, `321`, `322`, `377`), so idle-only counts could never answer that. Telemetry counts three kinds: `move_idle`, `move_track` (`move_toward_tracked`) and `move_path` (each step along any path, which is how a goal planner's `pathto` walks).
4. **Thirty shipped idle emotes and says contain a `;`, and today `Mob.Command` cuts them there** (`internal/mobs/mobs.go:950` splits on `;`), so the text after the `;` is lost and the tail runs as an unknown command (e.g. `mobs/hartcharn/9183-drostan.yaml`, `schedules/the_confluence/cf_gate_captain.yaml`). The new `ambient` path queues the whole line as one command (`Mob.CommandNoSplit`), so players will start seeing those lines whole. This is a visible change at multiplier 1.0, and a fix. The loot goblin's real multi-command entries (`portal home;go portal;wander...`) are movement and keep `Mob.Command`'s split.
5. **One shipped idle entry has no valid verb.** `schedules/new_plymouth_noble/np_noble_wenna.yaml:33` starts `emote's skin brightens...`; `emote's` is no command, so it has never run. The verb guard would fail on it, so Task 5 rewrites that entry.
6. **The zone the spec calls "the Elemental Oasis" is named `Instance Planar Oasis`** (`rooms/instance_planar_oasis/zone-config.yaml:1`). Instance clones keep the template room's `Zone` (`internal/rooms/ephemeral.go:105` reads `room.Zone` of a clone), so `rooms.GetZoneConfig(room.Zone)` resolves inside a paid instance.
7. **`ambientmultiplier: 0` with the budget off.** The spec divides the budget window by the multiplier. This plan reads 0 literally: with the budget on, the window is endless, so no budgeted line reaches anyone in that zone; with the budget off (knob 0), the zone's mob chatter and room idle are still 0 (they are scaled), and weather, gossip and item lines are untouched. No shipped zone sets 0.
8. **Go defaults reproduce today; `config.yaml` carries the retune.** Every new knob's Go default is today's behaviour (scales 1.0, room idle 5, tips 75, budget 0, weather cadence 24, seasonal 3), so test binaries and a config without the keys behave as master does. The shipped values (0.5, 3, 5, 48, 4) live only in `config.yaml`, per `dogmud-balance-config`.

9. **The echo of an ambient say is budgeted (owner ruling 2026-10-30, added to spec section 2).** `actions.Say` echoes "You hear someone talking." into every neighbouring room with an exit back (`internal/actions/say.go:40`, `Room.SendTextToExits(..., true)`, `internal/rooms/rooms.go:681-712`). The echo is a quiet RoomId message, and the delivery hook gives a quiet message only to a player with the `superhearing` condition flag (`internal/hooks/Message_SendMessages.go:50-52,89-93`); its own comment notes no shipped condition grants that flag. So the budgeting (Task 8A) charges and counts only players with `superhearing`, and today it changes nothing a player sees; the `say_echo` telemetry rows will read 0 in both playtests. It becomes live the day a condition grants the flag. Non-ambient says echo exactly as today.

Observations, not changed (each would widen #317 past its spec; listed for the owner in the final summary):
- Behaviour-tree idle lines (`mob_idle` branches such as Cleric Hadwen's 4% hint, `behaviors/pothole_coulee/9100-cleric_hadwen.yaml:301-306`) and room behaviour trees (`room_idle`) are not touched by the knobs or the budget.
- `dogmud-balance-config` keeps dated knob counts, not a knob list, so it needs no edit.

---

## Facts verified against source (master `1b3eaca3a`, 2026-10-30)

| Fact | Source |
|---|---|
| Idle roll today: `idleCmd := lookfortrouble; if util.Rand(100) < mob.ActivityLevel { idleCmd = mob.GetIdleCommand(); if "" then lookfortrouble }; mob.Command(idleCmd)` | `internal/hooks/MobIdle_HandleIdleMobs.go:310-320` |
| `GetIdleCommand`: `util.Rand(100) == 0` forces a blank, else uniform pick over `IdleCommands`; only caller is the hook; one test `TestGetIdleCommand` | `internal/mobs/mobs.go:1200-1216`; `internal/mobs/mobs_test.go:353-377` |
| Gossip tick: `mob.Command("say " + line)` every `GossipIntervalRounds` + stagger, before the behaviour tree | `internal/hooks/MobIdle_HandleIdleMobs.go:236-256` |
| `Mob.Command` splits input on `;` into separate `events.Input`s | `internal/mobs/mobs.go:932-963` |
| `Mob` fields `ActivityLevel` (yaml `activitylevel`), `IdleCommands` (yaml `idlecommands`); `Validate` clamps AL to 1..100 at its top and returns `error` | `internal/mobs/mobs.go:87,112,1267-1273,1346` |
| Mob instance saves persist only `MobInstanceData` (training, skills, gold, gear, plan state), never template fields, so a new mob YAML field is never shadowed | `internal/mobs/instance_save.go:22-52` |
| Schedule executor swaps `mob.IdleCommands = plan.NewIdleCommands` on a segment change | `internal/hooks/NewRound_IdleMobs_schedule.go:93,165-170` |
| Room spawns can override a mob's pool (`SpawnInfo.IdleCommands`) | `internal/rooms/spawninfo.go:27`; `internal/rooms/rooms.go:1136-1137` |
| Shipped idle verbs (scan of 642 mob files, 138 schedules, 1354 rooms; 1028 non-empty pools): `emote` 2610, `say` 411, `wander` 37, `sneak` 9, `portal` 6, `cast` 5, `wear` 4, `consume` 4, `alchemy` 1, `emote's` 1; 1735 blank entries; 30 entries contain `;` | scratchpad scan `verbscan.py`, 2026-10-30 |
| Marek's 10-22 segment: three emotes and one say, no blanks; Marek AL 15 | `schedules/thornwall_city/thornwall_tavern_keeper.yaml:13-20`; `mobs/thornwall_city/96-tavern_keeper_marek.yaml:31` |
| Cube generator wanderer pool `{"wander", "", "wander", ""}` | `internal/rooms/cubegen.go:134` |
| Sand Elemental 318: AL 25, `behavior_archetype: generic_fighter`, pool of 4 (2 emote, 1 wander, 1 blank); Storm 319 same shape; royals 320-322, 377 have no `idlecommands` | `mobs/instance_planar_oasis/*.yaml` (python read 2026-10-30) |
| A mob whose tree returns Success on `mob_idle` skips the legacy idle block; `generic_fighter` and `leader` end in `try_goal_planner` on `mob_idle` | `internal/behaviortree/helpers.go:228-285`; `behaviors/archetypes/generic_fighter.yaml`; `behaviors/archetypes/leader.yaml:51-75` |
| `move_toward_tracked` issues `mob.Command("go " + dir)` after the step quote; target from misc `tracking-user` (a player) or `tracking-mob` | `internal/behaviortree/actions_scout.go:169-218` |
| Goal planner issues `mob.Command(result.Command)`; movement planners return `"pathto <room>"` | `internal/behaviortree/actions_goal.go:73-85`; `internal/planners/*.go` (e.g. `craft_item.go:83`) |
| Path walker issues each step at `mob.Command(nextStep.ExitName())` | `internal/hooks/NewRound_IdleMobs.go:222-226` |
| Conversation start roll: `util.Rand(10000) < int(float64(cfg.ConversationBaseChancePct)*100)` then `conversations.TryStart` | `internal/hooks/NewRound_IdleMobs.go:157-168` |
| Arrival boost already honours 0: `if boostPct > 0 && util.Rand(100) < boostPct` | `internal/usercommands/go.go:568-569` |
| Conversation cooldown: `until := roundNow + uint64(int(ConversationCooldownRounds))` | `internal/conversations/state.go:243-246` |
| Room idle: hardcoded `chanceIn100 := 5` (20 for room -1), `util.Rand(100) < chanceIn100`, single-line exemption `idleMsgCt == 1`, `sendVisualRoomText(room, CategoryRoomDescription, wrappedMsg)` | `internal/hooks/NewRound_UserRoundTick.go:168-221` |
| `sendVisualRoomText` passes `excludeUserIds...` to `Room.SendTextVisual` | `internal/hooks/NewRound_DoCombat_helpers.go:433-438` |
| Tips: `tipIntervalRounds = 75` const, `evt.RoundNumber%tipIntervalRounds != 0`; test uses the const | `internal/hooks/NewRound_BroadcastTips.go:14-24`; `internal/hooks/broadcast_tips_test.go:25-42` |
| Every room sender fans out per player and takes `excludeUserIds ...int` (`SendText`, `SendTextVisualHidingNames`, `SendTextHidingNames`) | `internal/rooms/rooms.go:258-357` |
| `actions.SendSeen` mob branch: `room.SendTextVisualHidingNames(cat, text, names, actor.GetUserId())`; `sendSpoken` mob branch: `room.SendTextHidingNames(cat, line, names, messaging.HideSpeakerNames)` with no excludes | `internal/actions/room_lines.go:36-50,58-71` |
| `MobActor` is `{Mob, Room}`; every literal in the tree is keyed | `internal/actions/actor_mob.go:12-15`; grep `MobActor{` |
| Speech guard: `mobcommands/emote.go` must call `actions.SendSeen(` exactly 2 times, `mobcommands/say.go` `actions.Say(` once, and neither may name `HidingNames`, `room.SendText(` etc. | `internal/guards/speech_wrapper_guard_test.go:25-55` |
| Mob command table and `TryCommand`; mob-only commands are listed in `mobOnlyCommands` or the boot parity audit warns | `internal/mobcommands/mobcommands.go:25-105,118-203`; `internal/actions/divergences.go:184-205,224-262` |
| Mob input is split at the first space only (`processMobInput`), so one queued input keeps its `;` | `world.go:1079-1111` |
| Say echo: `room.SendTextToExits("You hear someone talking.", true)` before the room line | `internal/actions/say.go:40` |
| `SendTextToExits(txt, isQuiet, excludeUserIds...)` queues one RoomId `events.Message` per exit in each neighbour that leads back, `(From <exit>) ` prefixed, all sharing one exclude list; 6 production callers | `internal/rooms/rooms.go:681-712`; grep `SendTextToExits(` |
| RoomId messages honour `ExcludeUserIds`; a quiet one reaches only a player with the `superhearing` flag (`conditions.SuperHearing`), which no shipped condition grants | `internal/hooks/Message_SendMessages.go:50-52,66-93`; `internal/conditions/conditionspec.go:66` |
| `events.DrainQueuedRoomMessagesForTest(roomId)` returns queued RoomId messages | `internal/events/events.go:486-502` |
| Item chatter: holder line `holderUser.SendText`, room line `room.SendTextHidingNames(..., exclude...)`, cap `listenerCapNext` set after an `item_idle` line | `internal/behaviortree/actions_item_voice.go:45-50,226-260` |
| Weather: `seasonalEmoteOneIn = 3` const; weather line and seasonal line both `room.SendText(messaging.CategoryWeather, line)`; signature ends `mildChancePct, strongChancePct int, roll func(int) int`; one test caller | `modules/weather/engine/emotes.go:12-15,40-43,83-97`; `modules/weather/engine/emotes_config_test.go:63` |
| Weather pass gate `m.cfg.EmoteMode == EmoteModeModule && evt.RoundNumber >= m.nextEmote` | `modules/weather/weather.go:88-91` |
| Weather config `EmoteEveryRounds: intOr(get("EmoteEveryRounds"), 24)`, clamped `< 5 -> 5`; plugin keys are flat leaves under `Modules.weather` | `modules/weather/weather_config.go:31,108,121-123` |
| Shipped weather block `EmoteEveryRounds: 24`, `EmoteMildChancePct: 30`, `EmoteStrongChancePct: 100` | `_datafiles/config.yaml:2828-2846` (file is 2887 lines) |
| Conversation fields `ConversationBaseChancePct` (Float), `ConversationPlayerArrivalBoostPct`, `ConversationCooldownRounds` (Int); absent from `config.yaml`; validators `<= 0`, `<= 0`, `< 1` reset | `internal/configs/config.balance.go:753-768`; `internal/configs/config.balance.mobs.go:42-51`; grep `config.yaml` = 0 |
| `GossipIntervalRounds` field, shipped `75`, validator `< 20 -> 75` | `internal/configs/config.balance.go:861`; `_datafiles/config.yaml:2078-2079`; `internal/configs/config.balance.mobs.go:148-151` |
| 0-as-off idiom: seed `-1` in `newUnloadedConfig`, validate `< 0 -> default` (`ProneAutoStandRounds`), tests through `loadConfig` | `internal/configs/configs.go:503-522`; `internal/configs/config.balance.misc.go:190-195`; `internal/configs/config_proneautostand_test.go` |
| `Balance.Validate` calls `validateMobs`; struct closes at line 1403 | `internal/configs/config.balance.go:1403-1416` |
| `ZoneConfig` fields end with `DefaultPlane`, `RoomIds`; `Validate() error`; zone configs load through `fileloader.LoadAllFlatFiles`, which fails boot on a `Validate` error | `internal/rooms/zoneconfig.go:8-45`; `internal/rooms/save_and_load.go:533`; `internal/fileloader/fileloader.go:107,219` |
| Oasis zone config: `name: Instance Planar Oasis`, 8 lines, no multiplier | `_datafiles/world/dogmud/rooms/instance_planar_oasis/zone-config.yaml` |
| Test seams: `util.SetRandForTest(seed) func()`, `util.SetRoundCountForTest`, `util.ResetRoundCountForTest`, `events.DrainQueuedInputsForTest`, `events.DrainQueuedMessagesForTest`, `rooms.SeedRoomsForTest`, `rooms.MarkRoomOccupancy`, `rooms.SkyLightPtr`, `rooms.LampPtr`, `users.NewTestUser`, `users.SeedUsersForTest`, `configs.SetConfigForTest` | `internal/util/util.go:150-157,221`; `internal/events/events.go:371,442`; `internal/rooms/test_helpers.go:8,55,82-83`; `internal/users/test_helpers.go:29,56`; `internal/configs/testing_support.go:33` |
| hooks fixture `seedAllRegistries`: room 1 (`TestZone`, biome `city`) with users 1 and 2 and mob instance 100; a lit room is pinned with `rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)` | `internal/hooks/hooks_test.go:57-226`; `internal/hooks/hidden_mob_room_lines_test.go:47` |
| Import graph: `rooms` and `users` already depend on `mobs` and `conversations`; none of `rooms`, `users`, `messaging`, `configs`, `util`, `mudlog` depends on `hooks`, `behaviortree`, `mobcommands`, `usercommands` or `actions` | `go list -deps` 2026-10-30 |
| `mudlog` lines: colour handler prints `msg` then `key=value` pairs with ANSI codes and quoted strings; message cut at 24 characters; no-colour handler is slog text (`msg=... key=value`) | `internal/mudlog/loghandler.go:21-122` |
| Shipped `Logging.LogLevel: "debug"`, so Info lines are written | `_datafiles/config.yaml:2557` |
| Playtest image builds from the checkout directory with `COPY . .` (a dirty `config.yaml` goes in); run overrides only set version, AI port, LogToFile and profiles; server log lands at `tools/playtest/.run/<run_id>/server.log`, refreshed by `playtestenv logs` | `internal/playtestenv/compose.playtest.yml:16-18`; `provisioning/Dockerfile:24,36`; `internal/playtestenv/compose.go:121-165`; `internal/playtestenv/lifecycle.go:28,172`; `cmd/playtestenv/main.go:196-206` |
| Sable (mob 315, room 5000) opens the Oasis through a talk menu topic `open_oasis` ("Open a rift to the Planar Oasis."), asks for a stake (min 100), then says "Type oasis-<userid> to enter." | `dialogue/thornwall_city/315.yaml:29-33`; `behaviors/thornwall_city/315-sable.yaml:55-75`; `internal/behaviortree/actions_mob.go:557-578` |
| Route rooms: 472 The Drowning Post Tavern (north 469), 469 north 462 Main Street Central, 462 east 463, 463 east 464, 464 east 465 Market Square Center; 5203 Hub Square; 3116 Badlands Crossing; 5003 Oasis Threshold | room YAML read 2026-10-30 |
| `RoundsPerDay: 900` (a game day is an hour of real time), `NightHours: 8` | `_datafiles/config.yaml:215-218` |
| `tools/*.py` have no test harness and CI runs none | grep `tools/*.py`, `.github/workflows` 2026-10-30 |

---

## File map

| Path | Create / Modify | Responsibility |
|---|---|---|
| `internal/configs/config.balance.go` | Modify | Seven new knob fields; conversation and gossip comments |
| `internal/configs/config.balance.mobs.go` | Modify | Defaults and floors; 0 legal for the off switches |
| `internal/configs/configs.go` | Modify | `-1` sentinels for knobs whose 0 is off and whose default is not 0 |
| `internal/configs/config_ambient_test.go` | Create | Absent, zero, floor and shipped-value tests |
| `_datafiles/config.yaml` | Modify | Shipped values with comments; weather cadence and seasonal |
| `internal/hooks/NewRound_BroadcastTips.go` | Modify | `TipIntervalRounds`, 0 off |
| `internal/hooks/broadcast_tips_test.go` | Modify | Read the knob; 0 sends nothing |
| `internal/hooks/MobIdle_HandleIdleMobs.go` | Modify | Gossip 0 off, gossip via `ambient`, split roll wiring |
| `internal/hooks/NewRound_IdleMobs.go` | Modify | `conversationStartRoll`, budget gate on starts, path-step telemetry |
| `internal/hooks/conversation_roll_test.go` | Create | Base chance 0 never starts |
| `internal/conversations/state.go` | Modify | `cooldownUntil` helper |
| `internal/conversations/cooldown_test.go` | Create | Cooldown 0 is no cooldown |
| `internal/mobs/idle_roll.go` | Create | Verb table, pool split, chances, `RollIdle`, `CommandNoSplit`, `WanderScale` |
| `internal/mobs/idle_roll_test.go` | Create | Parity by formula and seeded run, class-only pools, multipliers |
| `internal/mobs/mobs.go` | Modify | `WanderMultiplier` field and check; delete `GetIdleCommand` (Task 8) |
| `internal/mobs/mobs_test.go` | Modify | Delete `TestGetIdleCommand` (Task 8) |
| `internal/guards/idle_verb_class_guard_test.go` | Create | Every shipped idle verb is classified; planted-verb probe |
| `_datafiles/world/dogmud/schedules/new_plymouth_noble/np_noble_wenna.yaml` | Modify | Rewrite the `emote's` entry |
| `internal/rooms/zoneconfig.go` | Modify | `AmbientMultiplier`, `WanderMultiplier`, validation, lookups |
| `internal/rooms/test_helpers.go` | Modify | `MultiplierPtr` |
| `internal/rooms/zoneconfig_multiplier_test.go` | Create | Unset, zero, negative, YAML round trip |
| `internal/ambient/ambient.go` | Create | `Plan`, `AllHaveBudget`, single-line visits, `RoundTick`; `SourceSayEcho` and `HearsQuiet` (Task 8A) |
| `internal/ambient/telemetry.go` | Create (Task 12) | Counters, `CountMove`, summary lines |
| `internal/ambient/testing.go` | Create | `ResetForTest`, `ChargeForTest` |
| `internal/ambient/ambient_test.go` | Create | Budget, perception, exclusion, zone window, visits |
| `internal/ambient/telemetry_test.go` | Create (Task 12) | Off keeps nothing; on counts and writes one line per room |
| `internal/ambient/main_test.go` | Create | Logger setup |
| `internal/ambient/context.md` | Create | Package doc |
| `internal/actions/actor_mob.go` | Modify | `MobActor.Skip`; `MobActor.EchoExclude` (Task 8A) |
| `internal/actions/room_lines.go` | Modify | `roomLineExcludes` in the mob branches |
| `internal/actions/room_line_excludes_test.go` | Create | Skip list reaches the excludes |
| `internal/actions/divergences.go` | Modify | `ambient` in `mobOnlyCommands` |
| `internal/mobcommands/emote.go`, `say.go` | Modify | `emoteAs`, `sayAs` bodies |
| `internal/mobcommands/ambient.go` | Create | The `ambient` mob command |
| `internal/mobcommands/mobcommands.go` | Modify | Register `ambient` |
| `internal/mobcommands/ambient_test.go` | Create | Budgeted say and emote; plain say untouched |
| `internal/rooms/rooms.go` | Modify (Task 8A) | `SendTextToExitsExcluding`; `SendTextToExits` delegates to it |
| `internal/rooms/send_to_exits_test.go` | Create (Task 8A) | Per-neighbour excludes |
| `internal/actions/say.go` | Modify (Task 8A) | `sendSayEcho` honours `MobActor.EchoExclude` |
| `internal/actions/say_echo_test.go` | Create (Task 8A) | A player's say still echoes unbudgeted |
| `internal/mobcommands/ambient_echo_test.go` | Create (Task 8A) | Over-budget neighbour left out, in-budget one charged |
| `internal/hooks/MobIdle_idle_roll.go` | Create | `idleRollInput`, `issueIdleChatter` |
| `internal/hooks/idle_roll_wiring_test.go` | Create | Wrapping, zeros, `;`, multipliers, gossip |
| `internal/hooks/NewRound_RoomIdle.go` | Create | `roomIdleChancePer10k`, `sendRoomIdleLine` |
| `internal/hooks/NewRound_UserRoundTick.go` | Modify | Room idle knob, budget, single-line rule, `ambient.RoundTick` |
| `internal/hooks/room_idle_ambient_test.go` | Create | Once per visit, 0 off, budget, zone scaling |
| `modules/weather/engine/emotes.go` | Modify | Seasonal knob, budgeted sends |
| `modules/weather/engine/emotes_config_test.go` | Modify | New signature |
| `modules/weather/engine/emotes_ambient_test.go` | Create | Seasonal 0, budget |
| `modules/weather/weather_config.go`, `weather.go` | Modify | `SeasonalEmoteOneIn`, cadence 0 off |
| `modules/weather/weather_config_test.go` | Modify | New knob tests |
| `modules/weather/weather_shipped_test.go` | Create | Shipped 48 and 4 |
| `internal/behaviortree/actions_item_voice.go` | Modify | Item idle line through `ambient.Plan` |
| `internal/behaviortree/item_voice_ambient_test.go` | Create | Over-budget listener and holder skip |
| `internal/usercommands/go.go` | Modify | Arrival conversation needs budget |
| `internal/behaviortree/actions_scout.go` | Modify | Track telemetry (Task 12); conditional wander gate (Task 16) |
| `internal/behaviortree/actions_goal.go` | Modify (Task 16 only) | Conditional wander gate on goal `pathto` |
| `internal/behaviortree/zone_wander_gate_test.go` | Create (Task 16 only) | Gate tests |
| `tools/ambient_report.py` | Create | Log to tables, `--self-test` |
| `tools/playtest/scenarios/317-ambient.yaml` | Create | Four-seat scenario |
| `tools/playtest/goals/scenarios/317-ambient/{town,coulee,steppe,oasis}.yaml` | Create | Seat goals |
| `internal/playtestrun/scenario_317_test.go` | Create | The scenario parses |
| `_datafiles/world/dogmud/rooms/instance_planar_oasis/zone-config.yaml` | Modify (Task 17) | Oasis `wandermultiplier` |
| `docs/schemas/room.md`, `docs/schemas/mob.md` | Modify | New fields |
| `docs/PATCH_NOTES.md`, `docs/README.md` | Modify | Patch notes; plan row |
| `internal/{hooks,mobs,mobcommands,actions,rooms,configs,behaviortree,usercommands}/context.md`, `modules/weather/context.md`, `modules/weather/engine/context.md` | Modify | Package docs |

---

## Task 1: Worktree and plan row (Sonnet)

**Files:** `docs/README.md`

- [ ] **Step 1: Create the worktree**

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feat/317-ambient-retune C:/tmp/dogmud-317-ambient origin/master
cd C:/tmp/dogmud-317-ambient && git log --oneline -1
```

Expected: the new worktree on `feat/317-ambient-retune` at the current `origin/master`. Every later path is relative to `C:/tmp/dogmud-317-ambient`.

- [ ] **Step 2: Copy this plan into the worktree**

If `docs/superpowers/plans/2026-10-30-317-ambient-retune.md` is not yet on master, copy it from the main checkout with `cp` (a copy, not a rewrite) into the same path in the worktree.

- [ ] **Step 3: Add the README row**

In `C:/tmp/dogmud-317-ambient/docs/README.md`, insert this row on the line after the row that begins ``| [`superpowers/plans/2026-10-30-p7-coulee-5-close.md`]``:

```markdown
| [`superpowers/plans/2026-10-30-317-ambient-retune.md`](superpowers/plans/2026-10-30-317-ambient-retune.md) | Plan for the #317 spec above: knobs per ambient source with 0 as off, the split and weighted idle roll with a verb-class guard, one-line rooms once per visit, the `internal/ambient` listener budget and switchable telemetry, zone and mob multipliers, `tools/ambient_report.py`, a baseline and a shipped-values playtest, the Oasis wander value, docs and the gate |
```

- [ ] **Step 4: Commit**

```bash
cd C:/tmp/dogmud-317-ambient
git add docs/README.md docs/superpowers/plans/2026-10-30-317-ambient-retune.md
git commit -F - <<'EOF'
docs: #317 ambient retune plan and README row

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 2: The config surface (Opus)

**Files:** `internal/configs/config.balance.go`, `internal/configs/config.balance.mobs.go`, `internal/configs/configs.go`, `internal/configs/config_ambient_test.go`, `_datafiles/config.yaml`

- [ ] **Step 1: Write the failing test**

Create `internal/configs/config_ambient_test.go`:

```go
package configs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// #317 ambient knobs. Each source's 0 is its off switch, so an absent key is
// told apart by a -1 sentinel seeded in newUnloadedConfig (the
// ProneAutoStandRounds idiom). The Go defaults are today's behaviour; the
// retune lives in config.yaml. These run the real load path.

func loadAndValidate(t *testing.T, doc string) Balance {
	t.Helper()
	cfg, err := loadConfig([]byte(doc))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	return cfg.Balance
}

func TestAmbientKnobs_AbsentKeysKeepTodaysBehaviour(t *testing.T) {
	b := loadAndValidate(t, "Balance:\n  StandMinStamina: 0.15\n")
	if b.MobIdleChatterScale != 1.0 || b.MobIdleMovementScale != 1.0 {
		t.Errorf("idle scales absent must load 1.0/1.0, got %v/%v", b.MobIdleChatterScale, b.MobIdleMovementScale)
	}
	if b.RoomIdleChancePct != 5 {
		t.Errorf("RoomIdleChancePct absent must load today's 5, got %v", b.RoomIdleChancePct)
	}
	if b.TipIntervalRounds != 75 {
		t.Errorf("TipIntervalRounds absent must load today's 75, got %v", b.TipIntervalRounds)
	}
	if b.AmbientListenerBudgetRounds != 0 {
		t.Errorf("AmbientListenerBudgetRounds absent must load 0 (no budget, as today), got %v", b.AmbientListenerBudgetRounds)
	}
	if bool(b.AmbientTelemetry) || b.AmbientTelemetryReportMinutes != 5 {
		t.Errorf("telemetry absent must load off with a 5 minute window, got %v/%v", b.AmbientTelemetry, b.AmbientTelemetryReportMinutes)
	}
	if b.ConversationBaseChancePct != 1.0 || b.ConversationPlayerArrivalBoostPct != 25 || b.ConversationCooldownRounds != 50 {
		t.Errorf("conversation knobs absent must load 1.0/25/50, got %v/%v/%v",
			b.ConversationBaseChancePct, b.ConversationPlayerArrivalBoostPct, b.ConversationCooldownRounds)
	}
	if b.GossipIntervalRounds != 75 {
		t.Errorf("GossipIntervalRounds absent must load 75, got %v", b.GossipIntervalRounds)
	}
}

func TestAmbientKnobs_ExplicitZeroIsOff(t *testing.T) {
	b := loadAndValidate(t, "Balance:\n"+
		"  MobIdleChatterScale: 0\n  MobIdleMovementScale: 0\n  RoomIdleChancePct: 0\n"+
		"  TipIntervalRounds: 0\n  AmbientListenerBudgetRounds: 0\n"+
		"  ConversationBaseChancePct: 0\n  ConversationPlayerArrivalBoostPct: 0\n  ConversationCooldownRounds: 0\n"+
		"  GossipIntervalRounds: 0\n  AmbientTelemetryReportMinutes: 0\n")
	zeros := map[string]float64{
		"MobIdleChatterScale":               float64(b.MobIdleChatterScale),
		"MobIdleMovementScale":              float64(b.MobIdleMovementScale),
		"RoomIdleChancePct":                 float64(b.RoomIdleChancePct),
		"TipIntervalRounds":                 float64(b.TipIntervalRounds),
		"AmbientListenerBudgetRounds":       float64(b.AmbientListenerBudgetRounds),
		"ConversationBaseChancePct":         float64(b.ConversationBaseChancePct),
		"ConversationPlayerArrivalBoostPct": float64(b.ConversationPlayerArrivalBoostPct),
		"ConversationCooldownRounds":        float64(b.ConversationCooldownRounds),
		"GossipIntervalRounds":              float64(b.GossipIntervalRounds),
	}
	for name, got := range zeros {
		if got != 0 {
			t.Errorf("%s: an explicit 0 is the off switch and must survive validation, got %v", name, got)
		}
	}
	if b.AmbientTelemetryReportMinutes != 5 {
		t.Errorf("AmbientTelemetryReportMinutes 0 has no off meaning and must take 5, got %v", b.AmbientTelemetryReportMinutes)
	}
}

func TestAmbientKnobs_Floors(t *testing.T) {
	b := loadAndValidate(t, "Balance:\n  GossipIntervalRounds: 10\n  RoomIdleChancePct: 140\n")
	if b.GossipIntervalRounds != 75 {
		t.Errorf("GossipIntervalRounds 1 to 19 keeps the old floor and reads as 75, got %v", b.GossipIntervalRounds)
	}
	if b.RoomIdleChancePct != 100 {
		t.Errorf("RoomIdleChancePct is a percent and caps at 100, got %v", b.RoomIdleChancePct)
	}
}

func TestAmbientKnobs_ShippedValues(t *testing.T) {
	_, self, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(self), "..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	cfg, err := loadConfig(raw)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	cfg.Validate()
	b := cfg.Balance
	if b.MobIdleChatterScale != 0.5 || b.MobIdleMovementScale != 1.0 || b.RoomIdleChancePct != 3 ||
		b.TipIntervalRounds != 75 || b.AmbientListenerBudgetRounds != 5 || bool(b.AmbientTelemetry) ||
		b.AmbientTelemetryReportMinutes != 5 {
		t.Errorf("shipped ambient knobs: chatter %v movement %v room %v tips %v budget %v telemetry %v/%v; want 0.5 1.0 3 75 5 false/5",
			b.MobIdleChatterScale, b.MobIdleMovementScale, b.RoomIdleChancePct, b.TipIntervalRounds,
			b.AmbientListenerBudgetRounds, b.AmbientTelemetry, b.AmbientTelemetryReportMinutes)
	}
	if b.ConversationBaseChancePct != 1.0 || b.ConversationPlayerArrivalBoostPct != 25 || b.ConversationCooldownRounds != 50 || b.GossipIntervalRounds != 75 {
		t.Errorf("shipped conversation/gossip knobs %v/%v/%v/%v, want today's 1.0/25/50/75",
			b.ConversationBaseChancePct, b.ConversationPlayerArrivalBoostPct, b.ConversationCooldownRounds, b.GossipIntervalRounds)
	}
}

func TestNewUnloadedConfig_SeedsAmbientSentinels(t *testing.T) {
	b := newUnloadedConfig().Balance
	if b.MobIdleChatterScale >= 0 || b.MobIdleMovementScale >= 0 || b.RoomIdleChancePct >= 0 || b.TipIntervalRounds >= 0 ||
		b.ConversationBaseChancePct >= 0 || b.ConversationPlayerArrivalBoostPct >= 0 || b.ConversationCooldownRounds >= 0 ||
		b.GossipIntervalRounds >= 0 {
		t.Fatalf("every ambient knob whose 0 is off and whose default is not 0 needs a negative sentinel: %+v", b)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
cd C:/tmp/dogmud-317-ambient && go test ./internal/configs/ -run 'TestAmbientKnobs|TestNewUnloadedConfig_SeedsAmbient' -count=1
```

Expected: build failure, `b.MobIdleChatterScale undefined (type Balance has no field or method MobIdleChatterScale)`.

- [ ] **Step 3: Add the fields**

In `internal/configs/config.balance.go`, replace the conversation block (lines 753-768):

```go
	// ── NPC-NPC IDLE CONVERSATIONS (chunk 3.6) ───────────────────────────────────
	// ConversationBaseChancePct is the per-tick percentage chance that a
	// fully-idle NPC will attempt to start an idle conversation with an
	// in-room partner that has a relationship edge. Default 1.0 → ~once
	// per 100 ticks per NPC. Chunk 3.6.
	ConversationBaseChancePct ConfigFloat `yaml:"ConversationBaseChancePct"`

	// ConversationPlayerArrivalBoostPct is the percentage chance that
	// a conversation will start when a player arrives in a room with 2+
	// relateable, idle NPCs. Default 25. Chunk 3.6.
	ConversationPlayerArrivalBoostPct ConfigInt `yaml:"ConversationPlayerArrivalBoostPct"`

	// ConversationCooldownRounds is the cooldown applied to both NPCs
	// after a conversation completes, before either can initiate another.
	// Default 50 (~200 sec real-time). Chunk 3.6.
	ConversationCooldownRounds ConfigInt `yaml:"ConversationCooldownRounds"`
```

with:

```go
	// ── NPC-NPC IDLE CONVERSATIONS (chunk 3.6) ───────────────────────────────────
	// ConversationBaseChancePct is the per-tick percentage chance that a
	// fully-idle NPC will attempt to start an idle conversation with an
	// in-room partner that has a relationship edge. Default 1.0, about once
	// per 100 ticks per NPC; 0 turns idle conversations off (#317). Chunk 3.6.
	ConversationBaseChancePct ConfigFloat `yaml:"ConversationBaseChancePct"`

	// ConversationPlayerArrivalBoostPct is the percentage chance that
	// a conversation will start when a player arrives in a room with 2+
	// relateable, idle NPCs. Default 25; 0 turns the boost off (#317).
	ConversationPlayerArrivalBoostPct ConfigInt `yaml:"ConversationPlayerArrivalBoostPct"`

	// ConversationCooldownRounds is the cooldown applied to both NPCs
	// after a conversation completes, before either can initiate another.
	// Default 50 (about 200 seconds); 0 means no cooldown (#317).
	ConversationCooldownRounds ConfigInt `yaml:"ConversationCooldownRounds"`
```

Then replace the gossip block (lines 860-861):

```go
	// ── GOSSIP SYSTEM ────────────────────────────────────────────────────────
	GossipIntervalRounds ConfigInt `yaml:"GossipIntervalRounds"` // Rounds between gossip broadcasts for "gossiper" group mobs (default 75)
```

with:

```go
	// ── GOSSIP SYSTEM ────────────────────────────────────────────────────────
	GossipIntervalRounds ConfigInt `yaml:"GossipIntervalRounds"` // Rounds between gossip broadcasts for "gossiper" group mobs; 0 turns gossip off, 1 to 19 read as 75 (default 75)

	// ── AMBIENT (#317) ───────────────────────────────────────────────────────
	// One knob per unprompted-line source. 0 turns a source off; an absent
	// key keeps today's behaviour through the -1 sentinel seeded in
	// newUnloadedConfig (the ProneAutoStandRounds idiom). The retune itself
	// lives in config.yaml, never in these defaults.
	MobIdleChatterScale           ConfigFloat `yaml:"MobIdleChatterScale"`           // Scales every idle mob's chatter roll (emote, say, consume, cast...); 1.0 = the authored rate, 0 = off (default 1.0)
	MobIdleMovementScale          ConfigFloat `yaml:"MobIdleMovementScale"`          // Scales every idle mob's movement roll (wander, sneak, portal); 1.0 = the authored rate, 0 = off (default 1.0)
	RoomIdleChancePct             ConfigInt   `yaml:"RoomIdleChancePct"`             // Percent chance per round that a room with players shows an idle line; room -1 gets 4x; 0 = off (default 5)
	TipIntervalRounds             ConfigInt   `yaml:"TipIntervalRounds"`             // Rounds between gameplay tips; 0 = off (default 75)
	AmbientListenerBudgetRounds   ConfigInt   `yaml:"AmbientListenerBudgetRounds"`   // A player hears at most one ambient line per this many rounds; 0 = no budget (default 0)
	AmbientTelemetry              ConfigBool  `yaml:"AmbientTelemetry"`              // Count ambient lines and mob moves per room and log a summary (default false)
	AmbientTelemetryReportMinutes ConfigInt   `yaml:"AmbientTelemetryReportMinutes"` // Real minutes between telemetry summary lines (default 5)
```

- [ ] **Step 4: Seed the sentinels**

In `internal/configs/configs.go`, replace:

```go
func newUnloadedConfig() Config {
	c := Config{}
	c.Balance.ProgressionFailureFraction = -1
	c.Balance.ProneAutoStandRounds = -1
	return c
}
```

with:

```go
func newUnloadedConfig() Config {
	c := Config{}
	c.Balance.ProgressionFailureFraction = -1
	c.Balance.ProneAutoStandRounds = -1
	// #317: each of these has 0 as its off switch and a non-zero default.
	c.Balance.MobIdleChatterScale = -1
	c.Balance.MobIdleMovementScale = -1
	c.Balance.RoomIdleChancePct = -1
	c.Balance.TipIntervalRounds = -1
	c.Balance.ConversationBaseChancePct = -1
	c.Balance.ConversationPlayerArrivalBoostPct = -1
	c.Balance.ConversationCooldownRounds = -1
	c.Balance.GossipIntervalRounds = -1
	return c
}
```

- [ ] **Step 5: Defaults and floors**

In `internal/configs/config.balance.mobs.go`, replace:

```go
	// ── NPC-NPC IDLE CONVERSATIONS ───────────────────────────────────────────
	if b.ConversationBaseChancePct <= 0 {
		b.ConversationBaseChancePct = 1.0
	}
	if b.ConversationPlayerArrivalBoostPct <= 0 {
		b.ConversationPlayerArrivalBoostPct = 25
	}
	if b.ConversationCooldownRounds < 1 {
		b.ConversationCooldownRounds = 50
	}
```

with:

```go
	// ── NPC-NPC IDLE CONVERSATIONS ───────────────────────────────────────────
	// 0 is the off switch for each (#317), so not `<= 0`; an absent key
	// still reaches the default through the -1 sentinel seeded in
	// newUnloadedConfig.
	if b.ConversationBaseChancePct < 0 {
		b.ConversationBaseChancePct = 1.0
	}
	if b.ConversationPlayerArrivalBoostPct < 0 {
		b.ConversationPlayerArrivalBoostPct = 25
	}
	if b.ConversationCooldownRounds < 0 {
		b.ConversationCooldownRounds = 50
	}
```

and replace:

```go
	// ── GOSSIP ───────────────────────────────────────────────────────────────
	if b.GossipIntervalRounds < 20 {
		b.GossipIntervalRounds = 75
	}
```

with:

```go
	// ── GOSSIP ───────────────────────────────────────────────────────────────
	// 0 turns gossip off (#317); an absent key reaches 75 through the -1
	// sentinel. 1 to 19 keep the old floor and read as 75.
	if b.GossipIntervalRounds < 0 || (b.GossipIntervalRounds > 0 && b.GossipIntervalRounds < 20) {
		b.GossipIntervalRounds = 75
	}

	// ── AMBIENT (#317) ───────────────────────────────────────────────────────
	// Defaults are today's behaviour. 0 is legal and means off for every
	// knob but the report window, so the guards are `< 0`, reached by an
	// absent key through the -1 sentinels in newUnloadedConfig.
	if b.MobIdleChatterScale < 0 {
		b.MobIdleChatterScale = 1.0
	}
	if b.MobIdleMovementScale < 0 {
		b.MobIdleMovementScale = 1.0
	}
	if b.RoomIdleChancePct < 0 {
		b.RoomIdleChancePct = 5
	} else if b.RoomIdleChancePct > 100 {
		b.RoomIdleChancePct = 100
	}
	if b.TipIntervalRounds < 0 {
		b.TipIntervalRounds = 75
	}
	if b.AmbientListenerBudgetRounds < 0 {
		b.AmbientListenerBudgetRounds = 0
	}
	if b.AmbientTelemetryReportMinutes <= 0 {
		b.AmbientTelemetryReportMinutes = 5
	}
```

- [ ] **Step 6: Ship the values**

In `_datafiles/config.yaml`, replace:

```yaml
  # ── GOSSIP SYSTEM ───────────────────────────────────────────────────────────
  GossipIntervalRounds: 75  # Rounds between gossip broadcasts for "gossiper" mobs (min 20)
```

with:

```yaml
  # ── GOSSIP SYSTEM ───────────────────────────────────────────────────────────
  GossipIntervalRounds: 75  # Rounds between gossip broadcasts for "gossiper" mobs; 0 turns gossip off, 1 to 19 read as 75

  # ── AMBIENT (#317) ──────────────────────────────────────────────────────────
  # Unprompted lines, one knob per source. 0 turns a source off. A round is
  # 4 seconds (Timing.RoundSeconds), so 15 rounds is a real minute.
  # MobIdleChatterScale: scales every idle mob's chance to emote, say, eat or
  #   cast on its own. 1.0 is the rate its activitylevel and idle pool give.
  # MobIdleMovementScale: the same for idle wandering (wander, sneak,
  #   portal), rolled apart from chatter, so a quieter town keeps its feet.
  # RoomIdleChancePct: percent chance each round that a room with players
  #   shows one of its idle lines (room -1 gets four times this).
  # TipIntervalRounds: rounds between gameplay tips (75 is 5 minutes).
  # AmbientListenerBudgetRounds: a player hears at most one ambient line
  #   (room idle, mob idle chatter, weather, gossip, item chatter) per this
  #   many rounds; the rest of the room still hears a line one player skips.
  #   An NPC conversation starts only when everyone present has budget left.
  # Conversation*: NPC conversations. BaseChancePct is a per-round percent
  #   (1.0 = 1%), PlayerArrivalBoostPct the chance one starts as a player
  #   walks in, CooldownRounds the rest between two; 0 turns each off.
  # AmbientTelemetry: count ambient lines and mob moves per room and log one
  #   "ambient.telemetry" line per active room every
  #   AmbientTelemetryReportMinutes; read it with tools/ambient_report.py.
  MobIdleChatterScale: 0.5
  MobIdleMovementScale: 1.0
  RoomIdleChancePct: 3
  TipIntervalRounds: 75
  AmbientListenerBudgetRounds: 5
  ConversationBaseChancePct: 1.0
  ConversationPlayerArrivalBoostPct: 25
  ConversationCooldownRounds: 50
  AmbientTelemetry: false
  AmbientTelemetryReportMinutes: 5
```

- [ ] **Step 7: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/configs/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt prints nothing (if it prints `config.balance.go`, run `gofmt -w internal/configs/config.balance.go` and re-check); all `ok`.

- [ ] **Step 8: Prove the sentinel test can fail**

Sabotage: delete the line `c.Balance.TipIntervalRounds = -1` from `newUnloadedConfig`. Run `go test ./internal/configs/ -run 'TestAmbientKnobs_AbsentKeys|TestNewUnloadedConfig_SeedsAmbient' -count=1`. Expected: both fail, the first naming `TipIntervalRounds absent must load today's 75, got 0`. Restore the line; run again; green.

- [ ] **Step 9: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.mobs.go internal/configs/configs.go internal/configs/config_ambient_test.go _datafiles/config.yaml
git commit -F - <<'EOF'
feat(config): #317 ambient knobs with 0 as off

MobIdleChatterScale, MobIdleMovementScale, RoomIdleChancePct,
TipIntervalRounds, AmbientListenerBudgetRounds, AmbientTelemetry and
AmbientTelemetryReportMinutes. The three conversation knobs are written
into config.yaml at today's live values and their 0 now means off;
GossipIntervalRounds 0 is off. Absent keys keep today's behaviour through
-1 sentinels in newUnloadedConfig. Readers land in later commits.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 3: Tips, gossip and conversation readers honour 0 (Sonnet)

**Files:** `internal/hooks/NewRound_BroadcastTips.go`, `internal/hooks/broadcast_tips_test.go`, `internal/hooks/MobIdle_HandleIdleMobs.go`, `internal/hooks/NewRound_IdleMobs.go`, `internal/hooks/conversation_roll_test.go`, `internal/conversations/state.go`, `internal/conversations/cooldown_test.go`

- [ ] **Step 1: Write the failing tests**

In `internal/hooks/broadcast_tips_test.go`, replace the three `tipIntervalRounds` uses with a local read of the knob, and add a zero test. Replace:

```go
	BroadcastTips(events.NewRound{RoundNumber: tipIntervalRounds - 1})
```

with:

```go
	tipIntervalRounds := uint64(configs.GetBalanceConfig().TipIntervalRounds)
	BroadcastTips(events.NewRound{RoundNumber: tipIntervalRounds - 1})
```

add `"github.com/GoMudEngine/GoMud/internal/configs"` to its imports, and append:

```go
// TipIntervalRounds 0 turns tips off (#317): no round sends one.
func TestBroadcastTips_ZeroIntervalIsOff(t *testing.T) {
	drainEventQueueOnCleanup(t)
	on := users.NewTestUser(9311, "tipzero", "Tipzero", 19311)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{9311: on})()
	defer tips.SeedForTest([]string{"A tip."})()
	cfg := configs.GetConfig()
	cfg.Balance.TipIntervalRounds = 0
	configs.SetConfigForTest(t, cfg)
	events.DrainQueuedMessagesForTest(9311)

	for _, round := range []uint64{0, 1, 75, 150} {
		BroadcastTips(events.NewRound{RoundNumber: round})
	}
	if got := events.DrainQueuedMessagesForTest(9311); len(got) != 0 {
		t.Fatalf("TipIntervalRounds 0 must send no tip, got %q", got)
	}
}
```

Create `internal/hooks/conversation_roll_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func pinConversationChance(t *testing.T, pct float64) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.ConversationBaseChancePct = configs.ConfigFloat(pct)
	configs.SetConfigForTest(t, cfg)
}

// ConversationBaseChancePct 0 never opens a conversation (#317), even on
// the luckiest draw; 1.0 does on that draw.
func TestConversationStartRoll_ZeroNeverPasses(t *testing.T) {
	luckiest := func(int) int { return 0 }
	pinConversationChance(t, 0)
	if conversationStartRoll(luckiest) {
		t.Fatal("ConversationBaseChancePct 0 must never start a conversation")
	}
	pinConversationChance(t, 1.0)
	if !conversationStartRoll(luckiest) {
		t.Fatal("ConversationBaseChancePct 1.0 must pass on a draw of 0")
	}
}
```

Create `internal/conversations/cooldown_test.go`:

```go
package conversations

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// ConversationCooldownRounds 0 means no cooldown (#317): a pair that just
// finished may start again the same round.
func TestCooldownUntil_ZeroIsNoCooldown(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.ConversationCooldownRounds = 0
	configs.SetConfigForTest(t, cfg)
	if got := cooldownUntil(4000); got != 4000 {
		t.Fatalf("cooldown 0: until = %d, want 4000", got)
	}
	cfg.Balance.ConversationCooldownRounds = 50
	configs.SetConfigForTest(t, cfg)
	if got := cooldownUntil(4000); got != 4050 {
		t.Fatalf("cooldown 50: until = %d, want 4050", got)
	}
}
```

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./internal/hooks/ -run 'TestBroadcastTips|TestConversationStartRoll' -count=1
go test ./internal/conversations/ -run TestCooldownUntil -count=1
```

Expected: build failures, `undefined: conversationStartRoll` and `undefined: cooldownUntil`. (The tips test compiles only after the helpers exist; the zero test then fails with `TipIntervalRounds 0 must send no tip` until Step 3: it sends at rounds 0, 75 and 150.)

- [ ] **Step 3: Implement**

`internal/hooks/NewRound_BroadcastTips.go`, replace:

```go
// BroadcastTips sends the next gameplay tip (internal/tips) to every active
// player who is not deafened and has not turned tips off with `set tips`,
// every tipIntervalRounds rounds (~5 minutes at 4-second rounds).

const (
	tipIntervalRounds = 75
	tipsConfigKey     = `tips`
)

func BroadcastTips(e events.Event) events.ListenerReturn {
	evt := e.(events.NewRound)

	if evt.RoundNumber%tipIntervalRounds != 0 {
		return events.Continue
	}
```

with:

```go
// BroadcastTips sends the next gameplay tip (internal/tips) to every active
// player who is not deafened and has not turned tips off with `set tips`,
// every Balance.TipIntervalRounds rounds (75 is about 5 minutes at 4-second
// rounds; 0 turns tips off, #317).

const tipsConfigKey = `tips`

func BroadcastTips(e events.Event) events.ListenerReturn {
	evt := e.(events.NewRound)

	interval := uint64(configs.GetBalanceConfig().TipIntervalRounds)
	if interval == 0 || evt.RoundNumber%interval != 0 {
		return events.Continue
	}
```

and add `"github.com/GoMudEngine/GoMud/internal/configs"` to its imports.

`internal/hooks/MobIdle_HandleIdleMobs.go`, replace:

```go
	// Stage 42.5: Gossiper mob tick — broadcast world event gossip
	if mobHasGroup(mob, "gossiper") {
		gossipIntervalRounds := uint64(configs.GetBalanceConfig().GossipIntervalRounds)
```

with:

```go
	// Stage 42.5: Gossiper mob tick, broadcasting world event gossip.
	// GossipIntervalRounds 0 turns gossip off (#317).
	if gossipIntervalRounds := uint64(configs.GetBalanceConfig().GossipIntervalRounds); gossipIntervalRounds > 0 && mobHasGroup(mob, "gossiper") {
```

`internal/hooks/NewRound_IdleMobs.go`, replace:

```go
		// Phase 2: if fully idle and not on cooldown, roll for a new
		// conversation. ConversationBaseChancePct is a float percent
		// (1.0 = 1 %), so multiply by 100 and roll out of 10 000 to
		// preserve fractional precision.
		if conversationsTriggerEligible(mob) {
			cfg := configs.GetBalanceConfig()
			if util.Rand(10000) < int(float64(cfg.ConversationBaseChancePct)*100) {
				if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
					conversations.TryStart(conversationadapter.AdaptMob(mob), room.GetMobs())
				}
			}
		}
```

with:

```go
		// Phase 2: if fully idle and not on cooldown, roll for a new
		// conversation (conversationStartRoll).
		if conversationsTriggerEligible(mob) && conversationStartRoll(util.Rand) {
			if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
				conversations.TryStart(conversationadapter.AdaptMob(mob), room.GetMobs())
			}
		}
```

and append to the same file:

```go
// conversationStartRoll is the per-tick roll for a fully idle mob to open a
// conversation. ConversationBaseChancePct is a float percent (1.0 = 1%),
// rolled out of 10000 to keep the fraction; 0 never passes (#317).
func conversationStartRoll(randN func(int) int) bool {
	pct := float64(configs.GetBalanceConfig().ConversationBaseChancePct)
	return pct > 0 && randN(10000) < int(pct*100)
}
```

`internal/conversations/state.go`, replace:

```go
	roundNow := util.GetRoundCount()
	cooldown := uint64(int(configs.GetBalanceConfig().ConversationCooldownRounds))
	until := roundNow + cooldown
```

with:

```go
	roundNow := util.GetRoundCount()
	until := cooldownUntil(roundNow)
```

and append:

```go
// cooldownUntil is the round a pair that just finished talking may start
// again. ConversationCooldownRounds 0 means no cooldown (#317).
func cooldownUntil(roundNow uint64) uint64 {
	return roundNow + uint64(int(configs.GetBalanceConfig().ConversationCooldownRounds))
}
```

The arrival boost needs no change: `internal/usercommands/go.go:569` already reads `if boostPct > 0 && ...`.

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/hooks/ ./internal/conversations/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty; all `ok`.

- [ ] **Step 5: Prove the tip test can fail**

Sabotage: change `if interval == 0 || evt.RoundNumber%interval != 0 {` to `if evt.RoundNumber%(interval+1) != 0 {`. Run `go test ./internal/hooks/ -run TestBroadcastTips_ZeroIntervalIsOff -count=1`; expected red with `TipIntervalRounds 0 must send no tip`. Restore; green.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/NewRound_BroadcastTips.go internal/hooks/broadcast_tips_test.go internal/hooks/MobIdle_HandleIdleMobs.go internal/hooks/NewRound_IdleMobs.go internal/hooks/conversation_roll_test.go internal/conversations/state.go internal/conversations/cooldown_test.go
git commit -F - <<'EOF'
feat(hooks): #317 tips, gossip and conversations honour 0 as off

TipIntervalRounds replaces the hardcoded 75; 0 sends no tips. Gossip 0
skips the gossip tick. conversationStartRoll and cooldownUntil make the
conversation base chance and cooldown testable at 0.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 4: The split, weighted idle roll in `mobs` (Opus)

**Files:** `internal/mobs/idle_roll.go`, `internal/mobs/idle_roll_test.go`, `internal/mobs/mobs.go`

- [ ] **Step 1: Write the failing test**

Create `internal/mobs/idle_roll_test.go`:

```go
package mobs

import (
	"math"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// todayIdlePick is the single roll #317 replaced, pinned from master
// 1b3eaca3a (hooks/MobIdle_HandleIdleMobs.go:313-319 and mobs.go:1200-1216):
// one ActivityLevel roll, the 1% forced blank, then a uniform pick over the
// whole pool. "" means the mob looked for trouble instead.
func todayIdlePick(al int, pool []string, randN func(int) int) string {
	if randN(100) >= al {
		return ""
	}
	if randN(100) == 0 {
		return ""
	}
	if len(pool) == 0 {
		return ""
	}
	return pool[randN(len(pool))]
}

// todayRates is today's expected per-round chatter and movement rate.
func todayRates(al int, entries []string) (chatter, movement float64) {
	p := SplitIdlePool(entries)
	if p.Size == 0 {
		return 0, 0
	}
	base := float64(al) / 100 * 0.99
	return base * float64(len(p.Chatter)) / float64(p.Size), base * float64(len(p.Movement)) / float64(p.Size)
}

var parityPools = []struct {
	name    string
	al      int
	entries []string
}{
	{"padded: Sand Elemental 318", 25, []string{
		"emote shifts and reforms, sand cascading in slow spirals", "wander",
		"emote rises from the ground in a column of whirling grit", ""}},
	{"schedule pool, no blanks: Marek 10-22", 15, []string{
		"emote wipes down the bar with a worn rag.", "emote pulls a fresh tap.",
		"say What'll it be, friend?", "emote shouts an order back to the kitchen."}},
	{"movement only: cubegen wanderer", 10, []string{"wander", "", "wander", ""}},
}

var unitScales = IdleRollInput{ChatterScale: 1, MovementScale: 1, ZoneAmbient: 1, ZoneWander: 1, MobWander: 1}

func assertNear(t *testing.T, what string, got, want, rel float64) {
	t.Helper()
	if want == 0 {
		if got != 0 {
			t.Errorf("%s = %v, want 0", what, got)
		}
		return
	}
	if math.Abs(got-want) > rel*want {
		t.Errorf("%s = %.5f, want %.5f within %.0f%%", what, got, want, rel*100)
	}
}

func TestIdleVerb(t *testing.T) {
	for entry, want := range map[string]string{
		"emote waves.":          "emote",
		"  Say hello":           "say",
		"wander;wander;suicide": "wander",
		"portal home;go portal": "portal",
		"":                      "",
		"   ":                   "",
	} {
		if got := IdleVerb(entry); got != want {
			t.Errorf("IdleVerb(%q) = %q, want %q", entry, got, want)
		}
	}
}

func TestClassifyIdleVerb(t *testing.T) {
	if c, ok := ClassifyIdleVerb("sneak"); !ok || c != IdleMovement {
		t.Errorf("sneak = %v,%v, want movement,true", c, ok)
	}
	if c, ok := ClassifyIdleVerb("alchemy"); !ok || c != IdleChatter {
		t.Errorf("alchemy = %v,%v, want chatter,true", c, ok)
	}
	if c, ok := ClassifyIdleVerb("dance"); ok || c != IdleChatter {
		t.Errorf("an unknown verb rolls as chatter and reports unknown, got %v,%v", c, ok)
	}
}

func TestSplitIdlePool_CountsBlanksInTheSize(t *testing.T) {
	p := SplitIdlePool(parityPools[0].entries)
	if len(p.Chatter) != 2 || len(p.Movement) != 1 || p.Size != 4 {
		t.Fatalf("Sand Elemental pool split = %d chatter, %d movement, size %d; want 2, 1, 4", len(p.Chatter), len(p.Movement), p.Size)
	}
}

// At every scale 1.0 each class's chance is today's divided by 0.99: the
// only change is the dropped 1% forced blank.
func TestIdleChances_UnitScalesEqualTodayByFormula(t *testing.T) {
	for _, pc := range parityPools {
		gotC, gotM := IdleChances(pc.al, SplitIdlePool(pc.entries), unitScales)
		wantC, wantM := todayRates(pc.al, pc.entries)
		if math.Abs(gotC*0.99-wantC) > 1e-12 || math.Abs(gotM*0.99-wantM) > 1e-12 {
			t.Errorf("%s: chances %v/%v, today %v/%v (want today = 0.99 x new)", pc.name, gotC, gotM, wantC, wantM)
		}
	}
}

func TestRollIdle_UnitScalesMatchTodayOverManyRounds(t *testing.T) {
	const rounds = 1_000_000
	for _, pc := range parityPools {
		t.Run(pc.name, func(t *testing.T) {
			restore := util.SetRandForTest(317)
			var todayC, todayM int
			for i := 0; i < rounds; i++ {
				cmd := todayIdlePick(pc.al, pc.entries, util.Rand)
				if cmd == "" {
					continue
				}
				if c, _ := ClassifyIdleVerb(IdleVerb(cmd)); c == IdleMovement {
					todayM++
				} else {
					todayC++
				}
			}
			restore()

			m := &Mob{ActivityLevel: pc.al, IdleCommands: pc.entries}
			restore = util.SetRandForTest(318)
			var newC, newM int
			for i := 0; i < rounds; i++ {
				c, mv := m.RollIdle(unitScales, util.Rand)
				if c != "" {
					newC++
				}
				if mv != "" {
					newM++
				}
			}
			restore()

			wantC, wantM := todayRates(pc.al, pc.entries)
			assertNear(t, "today chatter", float64(todayC)/rounds, wantC, 0.03)
			assertNear(t, "today movement", float64(todayM)/rounds, wantM, 0.03)
			assertNear(t, "new chatter", float64(newC)/rounds, wantC/0.99, 0.03)
			assertNear(t, "new movement", float64(newM)/rounds, wantM/0.99, 0.03)
			assertNear(t, "new vs today chatter", float64(newC)/rounds, float64(todayC)/rounds, 0.04)
			assertNear(t, "new vs today movement", float64(newM)/rounds, float64(todayM)/rounds, 0.04)
		})
	}
}

func TestRollIdle_ClassOnlyPools(t *testing.T) {
	big := IdleRollInput{ChatterScale: 4, MovementScale: 4, ZoneAmbient: 1, ZoneWander: 1, MobWander: 1}
	talker := &Mob{ActivityLevel: 100, IdleCommands: []string{"emote hums.", "say Hello."}}
	walker := &Mob{ActivityLevel: 100, IdleCommands: []string{"wander", "sneak"}}
	for i := 0; i < 10000; i++ {
		if _, mv := talker.RollIdle(big, util.Rand); mv != "" {
			t.Fatalf("a mob with only chatter entries moved: %q", mv)
		}
		if c, _ := walker.RollIdle(big, util.Rand); c != "" {
			t.Fatalf("a mob with only movement entries spoke: %q", c)
		}
	}
}

func TestIdleChances_Multipliers(t *testing.T) {
	pool := SplitIdlePool([]string{"wander", "emote hums."})
	scaled := func(mod func(*IdleRollInput)) (float64, float64) {
		in := unitScales
		mod(&in)
		return IdleChances(50, pool, in)
	}
	baseC, baseM := scaled(func(*IdleRollInput) {})
	if baseC != 0.25 || baseM != 0.25 {
		t.Fatalf("AL 50, one of two each: chances %v/%v, want 0.25/0.25", baseC, baseM)
	}
	for _, c := range []struct {
		name         string
		mod          func(*IdleRollInput)
		wantC, wantM float64
	}{
		{"mob wander 0.5", func(in *IdleRollInput) { in.MobWander = 0.5 }, 0.25, 0.125},
		{"zone wander 0.5", func(in *IdleRollInput) { in.ZoneWander = 0.5 }, 0.25, 0.125},
		{"both wanders 0.5", func(in *IdleRollInput) { in.MobWander, in.ZoneWander = 0.5, 0.5 }, 0.25, 0.0625},
		{"zone wander 0", func(in *IdleRollInput) { in.ZoneWander = 0 }, 0.25, 0},
		{"zone ambient 2.0", func(in *IdleRollInput) { in.ZoneAmbient = 2 }, 0.5, 0.25},
		{"zone ambient 0", func(in *IdleRollInput) { in.ZoneAmbient = 0 }, 0, 0.25},
		{"chatter scale 0", func(in *IdleRollInput) { in.ChatterScale = 0 }, 0, 0.25},
		{"movement scale 0", func(in *IdleRollInput) { in.MovementScale = 0 }, 0.25, 0},
	} {
		gotC, gotM := scaled(c.mod)
		if gotC != c.wantC || gotM != c.wantM {
			t.Errorf("%s: chances %v/%v, want %v/%v", c.name, gotC, gotM, c.wantC, c.wantM)
		}
	}
}

func TestRollIdle_LeavesActivityLevelAlone(t *testing.T) {
	m := &Mob{ActivityLevel: 37, IdleCommands: []string{"emote hums.", "wander", ""}}
	for i := 0; i < 1000; i++ {
		m.RollIdle(unitScales, util.Rand)
	}
	if m.ActivityLevel != 37 {
		t.Fatalf("RollIdle changed ActivityLevel to %d; combat AI reads it", m.ActivityLevel)
	}
}

func TestMobWanderMultiplier_YAMLAndValidate(t *testing.T) {
	var zero Mob
	if err := yaml.Unmarshal([]byte("mobid: 1\nwandermultiplier: 0\n"), &zero); err != nil {
		t.Fatal(err)
	}
	if zero.WanderMultiplier == nil || *zero.WanderMultiplier != 0 || zero.WanderScale() != 0 {
		t.Fatalf("wandermultiplier: 0 must load as a set 0 (never), got %v", zero.WanderMultiplier)
	}
	var unset Mob
	if err := yaml.Unmarshal([]byte("mobid: 2\n"), &unset); err != nil {
		t.Fatal(err)
	}
	if unset.WanderMultiplier != nil || unset.WanderScale() != 1 {
		t.Fatalf("an absent wandermultiplier is unset and scales by 1.0, got %v", unset.WanderMultiplier)
	}
	neg := -0.5
	bad := &Mob{MobId: 3, WanderMultiplier: &neg}
	if err := bad.Validate(); err == nil {
		t.Fatal("a negative wandermultiplier must fail Validate (and so refuse boot)")
	}
}

func TestCommandNoSplit_KeepsSemicolons(t *testing.T) {
	m := &Mob{InstanceId: 31701}
	t.Cleanup(func() { events.DrainQueuedInputsForTest(31701) })
	m.CommandNoSplit("ambient mob_chatter emote plunges the iron in; steam rises.")
	if got := events.DrainQueuedInputsForTest(31701); len(got) != 1 || got[0] != "ambient mob_chatter emote plunges the iron in; steam rises." {
		t.Fatalf("CommandNoSplit queued %q, want the whole line as one input", got)
	}
	m.Command("emote plunges the iron in; steam rises.")
	if got := events.DrainQueuedInputsForTest(31701); len(got) != 2 {
		t.Fatalf("Command still splits on ';' (the loot goblin relies on it), got %q", got)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/mobs/ -run 'TestIdleVerb|TestClassifyIdleVerb|TestSplitIdlePool|TestIdleChances|TestRollIdle|TestMobWanderMultiplier|TestCommandNoSplit' -count=1
```

Expected: build failure, `undefined: SplitIdlePool` (and the other new names).

- [ ] **Step 3: Implement**

Create `internal/mobs/idle_roll.go`:

```go
package mobs

import (
	"math"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// IdleClass is which of the two idle rolls an idle command belongs to (#317).
type IdleClass int

const (
	// IdleChatter is anything seen or heard in place: emote, say, consume,
	// cast and the rest.
	IdleChatter IdleClass = iota
	// IdleMovement leaves the room: wander, sneak, portal.
	IdleMovement
)

// idleVerbClass classifies every idle verb shipped content uses. A verb not
// listed rolls as chatter; internal/guards/idle_verb_class_guard_test.go
// fails on one in shipped content, so a new verb is added here first.
var idleVerbClass = map[string]IdleClass{
	"emote":   IdleChatter,
	"say":     IdleChatter,
	"consume": IdleChatter,
	"cast":    IdleChatter,
	"wear":    IdleChatter,
	"alchemy": IdleChatter,
	"wander":  IdleMovement,
	"sneak":   IdleMovement,
	"portal":  IdleMovement,
}

// IdleVerb is an idle command's verb: its first word, lower case, cut at a
// ';' so "wander;wander" reads as "wander". A blank entry has none.
func IdleVerb(entry string) string {
	fields := strings.Fields(entry)
	if len(fields) == 0 {
		return ""
	}
	verb := strings.ToLower(fields[0])
	if i := strings.Index(verb, ";"); i >= 0 {
		verb = verb[:i]
	}
	return verb
}

// ClassifyIdleVerb returns verb's class and whether the table knows it. An
// unknown verb rolls as chatter.
func ClassifyIdleVerb(verb string) (IdleClass, bool) {
	c, ok := idleVerbClass[verb]
	if !ok {
		return IdleChatter, false
	}
	return c, true
}

// IdlePool is an idle pool split by class. Size counts every entry, blanks
// included: blank entries are the authors' rate dial.
type IdlePool struct {
	Chatter  []string
	Movement []string
	Size     int
}

// SplitIdlePool splits entries into chatter and movement.
func SplitIdlePool(entries []string) IdlePool {
	p := IdlePool{Size: len(entries)}
	for _, e := range entries {
		verb := IdleVerb(e)
		if verb == "" {
			continue
		}
		if c, _ := ClassifyIdleVerb(verb); c == IdleMovement {
			p.Movement = append(p.Movement, e)
		} else {
			p.Chatter = append(p.Chatter, e)
		}
	}
	return p
}

// IdleRollInput carries the multipliers of one idle round: the two global
// scales (Balance.MobIdleChatterScale, MobIdleMovementScale), the mob's room
// zone's ambientmultiplier and wandermultiplier, and the mob's own
// wandermultiplier. 1.0 everywhere is the authored rate.
type IdleRollInput struct {
	ChatterScale  float64
	MovementScale float64
	ZoneAmbient   float64
	ZoneWander    float64
	MobWander     float64
}

// IdleChances is each class's chance per round (0 to 1), weighted by its
// share of the pool so that at every multiplier 1.0 a mob speaks and moves
// as often as the old single roll let it (#317):
//
//	chatter  = AL x ChatterScale x ZoneAmbient x chatter entries / pool size
//	movement = AL x MovementScale x MobWander x ZoneWander x movement entries / pool size
func IdleChances(activityLevel int, p IdlePool, in IdleRollInput) (chatter, movement float64) {
	if p.Size == 0 || activityLevel <= 0 {
		return 0, 0
	}
	al := float64(activityLevel) / 100
	size := float64(p.Size)
	chatter = al * in.ChatterScale * in.ZoneAmbient * float64(len(p.Chatter)) / size
	movement = al * in.MovementScale * in.MobWander * in.ZoneWander * float64(len(p.Movement)) / size
	return clampChance(chatter), clampChance(movement)
}

func clampChance(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// idleRollScale is the resolution of one idle roll: a chance is drawn out of
// a million so a fraction such as 0.03125 is kept.
const idleRollScale = 1_000_000

func rollChance(p float64, randN func(int) int) bool {
	if p <= 0 {
		return false
	}
	if p >= 1 {
		return true
	}
	return randN(idleRollScale) < int(math.Round(p*idleRollScale))
}

// RollIdle makes this round's two independent idle rolls and returns the
// chatter command and the movement command each picked ("" when that roll
// failed or its class is empty). A mob may now speak and move in one round;
// the caller looks for trouble only when both are "". ActivityLevel is read,
// never written: combat AI reads it too. randN is util.Rand in production.
func (m *Mob) RollIdle(in IdleRollInput, randN func(int) int) (chatter, movement string) {
	pool := SplitIdlePool(m.IdleCommands)
	pc, pm := IdleChances(m.ActivityLevel, pool, in)
	if rollChance(pc, randN) {
		chatter = pool.Chatter[randN(len(pool.Chatter))]
	}
	if rollChance(pm, randN) {
		movement = pool.Movement[randN(len(pool.Movement))]
	}
	return chatter, movement
}

// WanderScale is the mob's wandermultiplier: 1.0 when unset.
func (m *Mob) WanderScale() float64 {
	if m.WanderMultiplier == nil {
		return 1.0
	}
	return *m.WanderMultiplier
}

// CommandNoSplit queues inputTxt as ONE command, never split on ';'. The
// idle handler sends an ambient emote or say this way (#317): authored text
// may hold a ';' ("...into the quench trough; steam rises"), and Command
// would cut it into a second, unknown command and lose the rest.
func (m *Mob) CommandNoSplit(inputTxt string) {
	readyTurn := util.GetTurnCount()
	if readyTurn > m.lastCommandTurn {
		m.lastCommandTurn = readyTurn
	}
	events.AddToQueue(events.Input{
		MobInstanceId: m.InstanceId,
		InputText:     inputTxt,
		ReadyTurn:     m.lastCommandTurn,
	})
}
```

In `internal/mobs/mobs.go`, add the field after `IdleCommands` (line 112):

```go
	IdleCommands       []string       `yaml:"idlecommands,omitempty"` // Commands they may do while idle (not in combat)
	// WanderMultiplier scales this mob's idle movement roll (#317). nil
	// (unset) is 1.0 and 0 is never; a pointer so the two differ. Spawned
	// instances share the template's pointer (mob := *m), which is safe only
	// because nothing writes through it.
	WanderMultiplier *float64 `yaml:"wandermultiplier,omitempty"`
```

(replace the single existing `IdleCommands` line with those six lines; gofmt realigns). Then in `Validate`, replace:

```go
	if r.ActivityLevel < 1 {
		r.ActivityLevel = 10
	} else if r.ActivityLevel > 100 {
		r.ActivityLevel = 100
	}
```

with:

```go
	if r.ActivityLevel < 1 {
		r.ActivityLevel = 10
	} else if r.ActivityLevel > 100 {
		r.ActivityLevel = 100
	}

	// A negative wandermultiplier is an authoring error; refuse it (#317).
	if r.WanderMultiplier != nil && *r.WanderMultiplier < 0 {
		return fmt.Errorf("mob %d: wandermultiplier must be 0 or more, got %v", r.MobId, *r.WanderMultiplier)
	}
```

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/mobs/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty; all `ok`. The many-round test takes a few seconds.

- [ ] **Step 5: Prove the parity test can fail**

Sabotage `IdleChances`: change `float64(len(p.Chatter)) / size` to `float64(len(p.Chatter)) / float64(len(p.Chatter)+len(p.Movement))` (blanks dropped from the size). Run `go test ./internal/mobs/ -run 'TestIdleChances_UnitScales|TestRollIdle_UnitScales' -count=1`. Expected: the Sand Elemental case goes red (`chances 0.1666...`, new chatter far from 0.125); Marek's no-blank case stays green, which shows why the padded pool is in the set. Restore; green.

- [ ] **Step 6: Commit**

```bash
git add internal/mobs/idle_roll.go internal/mobs/idle_roll_test.go internal/mobs/mobs.go
git commit -F - <<'EOF'
feat(mobs): #317 split and weighted idle roll, mob wandermultiplier

RollIdle makes two independent rolls, chatter and movement, each weighted
by its share of the pool with blanks counted, so at every multiplier 1.0
the expected rates equal today's single roll (less the dropped 1% forced
blank). A fixed verb table classifies entries. CommandNoSplit queues one
input whole so a ';' inside authored text survives. Mob wandermultiplier
is a pointer: unset is 1.0, 0 is never, negative refuses boot.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 5: The verb-class guard and the one bad entry (Sonnet)

**Files:** `internal/guards/idle_verb_class_guard_test.go`, `_datafiles/world/dogmud/schedules/new_plymouth_noble/np_noble_wenna.yaml`

- [ ] **Step 1: Write the guard**

Create `internal/guards/idle_verb_class_guard_test.go`:

```go
package guards

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// Every idle command in shipped content starts with a verb the idle roll's
// class table knows (#317). The roll splits a pool into chatter and movement
// by that table, and an unknown verb quietly rolls as chatter, which would
// move a new movement verb onto the chatter dial. Mob templates, schedule
// segments and room spawn overrides all carry pools.

func idleEntries(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if s, ok := e.(string); ok {
			out = append(out, s)
		} else {
			out = append(out, "")
		}
	}
	return out
}

func addIdlePool(pools map[string][]string, key string, v any) {
	if entries := idleEntries(v); len(entries) > 0 {
		pools[key] = entries
	}
}

// shippedIdlePools reads every idle pool in shipped content, keyed
// "<file>#<where>".
func shippedIdlePools(t *testing.T) map[string][]string {
	t.Helper()
	pools := map[string][]string{}
	root := filepath.Join("_datafiles", "world", "dogmud")
	for _, sub := range []string{"mobs", "schedules", "rooms"} {
		dir := filepath.Join(root, sub)
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var doc map[string]any
			if yaml.Unmarshal(raw, &doc) != nil {
				return nil // the loaders own parse errors
			}
			rel := filepath.ToSlash(path)
			switch sub {
			case "mobs":
				addIdlePool(pools, rel+"#mob", doc["idlecommands"])
			case "schedules":
				segs, _ := doc["segments"].([]any)
				for i, seg := range segs {
					if m, ok := seg.(map[any]any); ok {
						addIdlePool(pools, fmt.Sprintf("%s#segment%d", rel, i), m["idlecommands"])
					}
				}
			case "rooms":
				spawns, _ := doc["spawninfo"].([]any)
				for i, sp := range spawns {
					if m, ok := sp.(map[any]any); ok {
						addIdlePool(pools, fmt.Sprintf("%s#spawn%d", rel, i), m["idlecommands"])
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	return pools
}

// unclassifiedIdleVerbs lists every entry whose verb the table does not know.
func unclassifiedIdleVerbs(pools map[string][]string) []string {
	keys := make([]string, 0, len(pools))
	for k := range pools {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bad []string
	for _, k := range keys {
		for _, e := range pools[k] {
			verb := mobs.IdleVerb(e)
			if verb == "" {
				continue
			}
			if _, ok := mobs.ClassifyIdleVerb(verb); !ok {
				bad = append(bad, fmt.Sprintf("%s: %q", k, e))
			}
		}
	}
	return bad
}

func TestShippedIdleVerbsAreClassified(t *testing.T) {
	pools := shippedIdlePools(t)
	// The walk must be able to find something: a known pool with a wander.
	if _, ok := pools["_datafiles/world/dogmud/mobs/instance_planar_oasis/318-sand_elemental.yaml#mob"]; !ok {
		t.Fatal("the Sand Elemental's pool was not found: the walk is not reading shipped mob files")
	}
	if len(pools) < 500 {
		t.Fatalf("only %d idle pools found (1028 on 2026-10-30): the walk is not reading shipped content", len(pools))
	}
	for _, bad := range unclassifiedIdleVerbs(pools) {
		t.Errorf("idle command whose verb mobs.idleVerbClass does not classify: %s (add the verb to the table as chatter or movement)", bad)
	}
}

func TestUnclassifiedIdleVerbs_CatchesAPlantedVerb(t *testing.T) {
	got := unclassifiedIdleVerbs(map[string][]string{"planted#mob": {"emote waves.", "", "dance wildly"}})
	if len(got) != 1 || !strings.Contains(got[0], "dance wildly") {
		t.Fatalf("a planted unknown verb must be reported once, got %v", got)
	}
}
```

- [ ] **Step 2: Run it and see it fail on the real bad entry**

```bash
go test ./internal/guards/ -run 'TestShippedIdleVerbsAreClassified|TestUnclassifiedIdleVerbs' -count=1
```

Expected: `TestUnclassifiedIdleVerbs_CatchesAPlantedVerb` passes; `TestShippedIdleVerbsAreClassified` fails with exactly one error naming `_datafiles/world/dogmud/schedules/new_plymouth_noble/np_noble_wenna.yaml#segment...: "emote's skin brightens..."`. This is the guard proving it can fail on real content.

- [ ] **Step 3: Rewrite the entry**

In `_datafiles/world/dogmud/schedules/new_plymouth_noble/np_noble_wenna.yaml`, replace:

```yaml
      - emote's skin brightens faintly at the corners of her eyes — a
        brief involuntary glow — and fades as she stills herself with
        visible effort.
```

with:

```yaml
      - emote glows faintly at the corners of her eyes, a brief involuntary
        light that fades as she stills herself with visible effort.
```

- [ ] **Step 4: Run and pass**

```bash
go test ./internal/guards/ -count=1
go test . -count=1
```

Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add internal/guards/idle_verb_class_guard_test.go _datafiles/world/dogmud/schedules/new_plymouth_noble/np_noble_wenna.yaml
git commit -F - <<'EOF'
test(guards): #317 every shipped idle verb is classified

Walks mob templates, schedule segments and room spawn overrides (1028
pools) and fails on a verb mobs.idleVerbClass does not know. It found one:
Wenna's "emote's skin brightens..." had no valid verb and never ran; it is
rewritten as an emote.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 6: Zone multipliers (Sonnet)

**Files:** `internal/rooms/zoneconfig.go`, `internal/rooms/test_helpers.go`, `internal/rooms/zoneconfig_multiplier_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/rooms/zoneconfig_multiplier_test.go`:

```go
package rooms

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestZoneMultipliers_UnsetZeroAndSet(t *testing.T) {
	defer SeedRoomsForTest(map[int]*Room{}, map[string]*ZoneConfig{
		"Plain":  {Name: "Plain"},
		"Hushed": {Name: "Hushed", AmbientMultiplier: MultiplierPtr(0), WanderMultiplier: MultiplierPtr(0)},
		"Lively": {Name: "Lively", AmbientMultiplier: MultiplierPtr(2), WanderMultiplier: MultiplierPtr(0.5)},
	})()
	for _, c := range []struct {
		zone          string
		ambient, walk float64
	}{
		{"Plain", 1, 1},
		{"NoSuchZone", 1, 1},
		{"Hushed", 0, 0},
		{"Lively", 2, 0.5},
	} {
		if got := ZoneAmbientMultiplier(c.zone); got != c.ambient {
			t.Errorf("ZoneAmbientMultiplier(%q) = %v, want %v", c.zone, got, c.ambient)
		}
		if got := ZoneWanderMultiplier(c.zone); got != c.walk {
			t.Errorf("ZoneWanderMultiplier(%q) = %v, want %v", c.zone, got, c.walk)
		}
	}
}

func TestZoneMultipliers_YAMLTellsZeroFromUnset(t *testing.T) {
	var z ZoneConfig
	if err := yaml.Unmarshal([]byte("name: Z\nambientmultiplier: 0\n"), &z); err != nil {
		t.Fatal(err)
	}
	if z.AmbientMultiplier == nil || *z.AmbientMultiplier != 0 {
		t.Fatalf("ambientmultiplier: 0 must load as a set 0, got %v", z.AmbientMultiplier)
	}
	if z.WanderMultiplier != nil {
		t.Fatalf("an absent wandermultiplier must stay unset, got %v", *z.WanderMultiplier)
	}
}

func TestZoneMultipliers_NegativeRefusesBoot(t *testing.T) {
	z := &ZoneConfig{Name: "Bad", WanderMultiplier: MultiplierPtr(-1)}
	if err := z.Validate(); err == nil {
		t.Fatal("a negative wandermultiplier must fail Validate")
	}
	z = &ZoneConfig{Name: "Bad", AmbientMultiplier: MultiplierPtr(-0.1)}
	if err := z.Validate(); err == nil {
		t.Fatal("a negative ambientmultiplier must fail Validate")
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/rooms/ -run TestZoneMultipliers -count=1
```

Expected: build failure, `unknown field AmbientMultiplier in struct literal of type ZoneConfig`.

- [ ] **Step 3: Implement**

In `internal/rooms/zoneconfig.go`, replace the import block and the struct's last two fields:

```go
import (
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/util"
)
```

with:

```go
import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/util"
)
```

and:

```go
	DefaultPlane   int                  `yaml:"default_plane,omitempty"`   // builder convenience: default plane for new rooms in this zone (rooms' own plane is authoritative)
	RoomIds        map[int]struct{}     `yaml:"-"`                         // Does not get written. Built dyanmically when rooms are loaded.
}
```

with:

```go
	DefaultPlane   int                  `yaml:"default_plane,omitempty"`   // builder convenience: default plane for new rooms in this zone (rooms' own plane is authoritative)
	// AmbientMultiplier scales this zone's idle mob chatter roll and room
	// idle chance and divides the ambient budget window (#317): 1.5 is
	// livelier, 0.5 hushed. WanderMultiplier scales idle mob movement here.
	// Pointers: unset (nil) is 1.0 and 0 is never.
	AmbientMultiplier *float64         `yaml:"ambientmultiplier,omitempty"`
	WanderMultiplier  *float64         `yaml:"wandermultiplier,omitempty"`
	RoomIds           map[int]struct{} `yaml:"-"` // Does not get written. Built dyanmically when rooms are loaded.
}
```

In `Validate`, replace:

```go
	if z.Instanced && z.PortalDuration == "" {
		z.PortalDuration = "30 real minutes"
	}

	return nil
}
```

with:

```go
	if z.Instanced && z.PortalDuration == "" {
		z.PortalDuration = "30 real minutes"
	}

	if z.AmbientMultiplier != nil && *z.AmbientMultiplier < 0 {
		return fmt.Errorf("zone %q: ambientmultiplier must be 0 or more, got %v", z.Name, *z.AmbientMultiplier)
	}
	if z.WanderMultiplier != nil && *z.WanderMultiplier < 0 {
		return fmt.Errorf("zone %q: wandermultiplier must be 0 or more, got %v", z.Name, *z.WanderMultiplier)
	}

	return nil
}

// ZoneAmbientMultiplier is zone's ambientmultiplier, 1.0 when the zone or
// the field is unset (#317). An instance clone keeps its template room's
// Zone, so a paid instance reads its template zone's value.
func ZoneAmbientMultiplier(zone string) float64 {
	if z := GetZoneConfig(zone); z != nil && z.AmbientMultiplier != nil {
		return *z.AmbientMultiplier
	}
	return 1.0
}

// ZoneWanderMultiplier is zone's wandermultiplier, 1.0 when unset (#317).
func ZoneWanderMultiplier(zone string) float64 {
	if z := GetZoneConfig(zone); z != nil && z.WanderMultiplier != nil {
		return *z.WanderMultiplier
	}
	return 1.0
}
```

In `internal/rooms/test_helpers.go`, after the line `func LampPtr(n int) *int             { return &n }`, add:

```go
// MultiplierPtr returns a pointer to v for a zone or mob multiplier in a
// test fixture (#317).
func MultiplierPtr(v float64) *float64 { return &v }
```

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/rooms/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty; all `ok`. (`internal/rooms` zone lifecycle tests skip without `DOGMUD_BOOT_SMOKE`; see `dogmud-writing-tests` for the two Windows-only failures under it.)

- [ ] **Step 5: Commit**

```bash
git add internal/rooms/zoneconfig.go internal/rooms/test_helpers.go internal/rooms/zoneconfig_multiplier_test.go
git commit -F - <<'EOF'
feat(rooms): #317 zone ambientmultiplier and wandermultiplier

Pointer fields so unset (1.0) and 0 (never) differ; negatives refuse boot.
ZoneAmbientMultiplier and ZoneWanderMultiplier read them; an instance
clone keeps its template room's zone, so paid instances inherit.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 7: The `internal/ambient` budget core (Opus)

**Files:** `internal/ambient/ambient.go`, `internal/ambient/testing.go`, `internal/ambient/main_test.go`, `internal/ambient/ambient_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/ambient/main_test.go`:

```go
package ambient

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}
```

Create `internal/ambient/ambient_test.go`:

```go
package ambient

import (
	"fmt"
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

const (
	ambRoom  = 3170
	ambZone  = "AmbientZone"
	ambNow   = uint64(10000)
	listenA  = 3171
	listenB  = 3172
)

// seedAmbientRoom: one room in ambZone (zone multiplier zoneMult, nil =
// unset) holding the given players, the budget pinned to budget rounds and
// the round count at ambNow.
func seedAmbientRoom(t *testing.T, budget int, zoneMult *float64, userIds ...int) *rooms.Room {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.AmbientListenerBudgetRounds = configs.ConfigInt(budget)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ResetForTest())

	room := &rooms.Room{RoomId: ambRoom, Zone: ambZone}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{ambRoom: room},
		map[string]*rooms.ZoneConfig{ambZone: {Name: ambZone, AmbientMultiplier: zoneMult}}))
	seed := map[int]*users.UserRecord{}
	for _, id := range userIds {
		u := users.NewTestUser(id, fmt.Sprintf("amb%d", id), fmt.Sprintf("Amb%d", id), 0)
		u.Character.RoomId = ambRoom
		seed[id] = u
		room.AddPlayer(id)
	}
	t.Cleanup(users.SeedUsersForTest(seed))
	rooms.MarkRoomOccupancy(ambRoom, len(userIds), 0)
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(ambNow)
	return room
}

func sorted(ids []int) []int {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func TestPlan_BudgetOffDeliversToEveryone(t *testing.T) {
	room := seedAmbientRoom(t, 0, nil, listenA, listenB)
	for i := 0; i < 3; i++ {
		r := Plan(Line{Room: room, Source: SourceRoomIdle})
		if len(r.Exclude) != 0 || !slices.Equal(sorted(r.Delivered), []int{listenA, listenB}) {
			t.Fatalf("budget 0, line %d: exclude %v delivered %v, want everyone", i, r.Exclude, r.Delivered)
		}
	}
}

func TestPlan_TwoLinesInAWindowReachAListenerOnce(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA)
	if r := Plan(Line{Room: room, Source: SourceMobChatter}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("first line: delivered %v, want the listener", r.Delivered)
	}
	if r := Plan(Line{Room: room, Source: SourceWeather}); len(r.Delivered) != 0 || !slices.Equal(r.Exclude, []int{listenA}) {
		t.Fatalf("second line in the window: delivered %v exclude %v, want skipped", r.Delivered, r.Exclude)
	}
	util.SetRoundCountForTest(ambNow + 4)
	if r := Plan(Line{Room: room, Source: SourceGossip}); len(r.Delivered) != 0 {
		t.Fatalf("round +4 is still inside a 5-round window, delivered %v", r.Delivered)
	}
	util.SetRoundCountForTest(ambNow + 5)
	if r := Plan(Line{Room: room, Source: SourceItemChatter}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("round +5 opens the window again, delivered %v", r.Delivered)
	}
}

func TestPlan_ASecondListenerWithBudgetStillHears(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA, listenB)
	ChargeForTest(listenA, ambNow+5)
	r := Plan(Line{Room: room, Source: SourceRoomIdle})
	if !slices.Equal(r.Exclude, []int{listenA}) || !slices.Equal(r.Delivered, []int{listenB}) {
		t.Fatalf("exclude %v delivered %v, want A skipped and B hearing", r.Exclude, r.Delivered)
	}
}

func TestPlan_CallerExcludeIsNeverCharged(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA)
	r := Plan(Line{Room: room, Source: SourceRoomIdle, Exclude: []int{listenA}})
	if !slices.Equal(r.Exclude, []int{listenA}) || len(r.Delivered) != 0 {
		t.Fatalf("caller exclude: exclude %v delivered %v", r.Exclude, r.Delivered)
	}
	if r := Plan(Line{Room: room, Source: SourceRoomIdle}); !slices.Equal(r.Delivered, []int{listenA}) {
		t.Fatalf("a player the caller left out was charged anyway: delivered %v", r.Delivered)
	}
}

func TestPlan_APlayerWhoCannotPerceiveIsNotCharged(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA, listenB)
	blindB := func(u *users.UserRecord) bool { return u.UserId != listenB }
	r := Plan(Line{Room: room, Source: SourceRoomIdle, Perceives: blindB})
	if !slices.Equal(r.Delivered, []int{listenA}) || slices.Contains(r.Exclude, listenB) {
		t.Fatalf("delivered %v exclude %v: B neither delivered nor excluded", r.Delivered, r.Exclude)
	}
	if r := Plan(Line{Room: room, Source: SourceWeather}); !slices.Equal(r.Delivered, []int{listenB}) {
		t.Fatalf("B, who did not see the first line, must still have budget; delivered %v", r.Delivered)
	}
}

func TestBudgetWindow_ZoneMultiplier(t *testing.T) {
	for _, c := range []struct {
		name       string
		budget     int
		mult       *float64
		wantWindow uint64
		wantHushed bool
	}{
		{"unset", 4, nil, 4, false},
		{"2.0 halves it", 4, rooms.MultiplierPtr(2), 2, false},
		{"0.5 doubles it", 4, rooms.MultiplierPtr(0.5), 8, false},
		{"0 with the budget on is hushed", 4, rooms.MultiplierPtr(0), 0, true},
		{"0 with the budget off is nothing", 0, rooms.MultiplierPtr(0), 0, false},
		{"never below one round", 1, rooms.MultiplierPtr(4), 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			seedAmbientRoom(t, c.budget, c.mult)
			w, h := budgetWindow(ambZone)
			if w != c.wantWindow || h != c.wantHushed {
				t.Fatalf("window %d hushed %v, want %d %v", w, h, c.wantWindow, c.wantHushed)
			}
		})
	}
}

func TestPlan_HushedZoneSkipsEveryone(t *testing.T) {
	room := seedAmbientRoom(t, 5, rooms.MultiplierPtr(0), listenA, listenB)
	r := Plan(Line{Room: room, Source: SourceWeather})
	if len(r.Delivered) != 0 || !slices.Equal(sorted(r.Exclude), []int{listenA, listenB}) {
		t.Fatalf("hushed zone: delivered %v exclude %v", r.Delivered, r.Exclude)
	}
}

func TestAllHaveBudget(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA, listenB)
	if !AllHaveBudget(room) {
		t.Fatal("fresh players all have budget")
	}
	ChargeForTest(listenB, ambNow+3)
	if AllHaveBudget(room) {
		t.Fatal("a conversation must not start while a present player is over budget")
	}
	empty := seedAmbientRoom(t, 5, nil)
	if !AllHaveBudget(empty) {
		t.Fatal("a room with no players has nobody to cut off")
	}
	off := seedAmbientRoom(t, 0, nil, listenA)
	ChargeForTest(listenA, ambNow+3)
	if !AllHaveBudget(off) {
		t.Fatal("with the budget off every conversation may start")
	}
}

func TestSingleLineHeard_OncePerVisit(t *testing.T) {
	seedAmbientRoom(t, 0, nil, listenA, listenB)
	MarkSingleLineHeard(ambRoom, []int{listenA})
	if got := SingleLineHeard(ambRoom, []int{listenA, listenB}); !slices.Equal(got, []int{listenA}) {
		t.Fatalf("heard = %v, want only A", got)
	}
	users.GetByUserId(listenA).Character.RoomId = 1 // A walks out
	RoundTick(ambNow)
	users.GetByUserId(listenA).Character.RoomId = ambRoom // and back in
	if got := SingleLineHeard(ambRoom, []int{listenA}); len(got) != 0 {
		t.Fatalf("a new visit must show the line again, heard = %v", got)
	}
}

func TestRoundTick_PrunesSpentBudget(t *testing.T) {
	seedAmbientRoom(t, 5, nil, listenA)
	ChargeForTest(listenA, ambNow+2)
	util.SetRoundCountForTest(ambNow + 3)
	RoundTick(ambNow + 3)
	budgetMu.Lock()
	n := len(budgetNext)
	budgetMu.Unlock()
	if n != 0 {
		t.Fatalf("a spent budget entry must be pruned, %d left", n)
	}
}

func TestParseSource(t *testing.T) {
	for _, s := range Sources {
		if got, ok := ParseSource(string(s)); !ok || got != s {
			t.Errorf("ParseSource(%q) = %q,%v", s, got, ok)
		}
	}
	if _, ok := ParseSource("tips"); ok {
		t.Error("tips are exempt and are not a source")
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/ambient/ -count=1
```

Expected: `no non-test Go files in .../internal/ambient` or `undefined: Plan`.

- [ ] **Step 3: Implement**

Create `internal/ambient/ambient.go`:

```go
// Package ambient is the one gate every unprompted ("ambient") line passes
// on its way to players (#317). It owns the per-listener budget
// (Balance.AmbientListenerBudgetRounds), the once-per-visit rule for a room
// with a single idle line, and the telemetry that measures how much each
// room hears.
//
// A counted send site asks Plan who the line may reach and passes the
// returned Exclude to the room sender it already used, so sight gating,
// name hiding and categories stay exactly where they were. Sends that are
// not ambient never come here.
package ambient

import (
	"math"
	"slices"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Source names one counted ambient source. The strings are telemetry keys
// tools/ambient_report.py reads, so they never change.
type Source string

const (
	SourceRoomIdle    Source = "room_idle"
	SourceMobChatter  Source = "mob_chatter"
	SourceWeather     Source = "weather"
	SourceGossip      Source = "gossip"
	SourceItemChatter Source = "item_chatter"
)

// Sources lists every counted source, in report order.
var Sources = []Source{SourceRoomIdle, SourceMobChatter, SourceWeather, SourceGossip, SourceItemChatter}

// ParseSource maps a source word, as the `ambient` mob command carries it,
// back to its Source.
func ParseSource(word string) (Source, bool) {
	for _, s := range Sources {
		if string(s) == word {
			return s, true
		}
	}
	return "", false
}

// Line is one ambient line about to be sent into a room.
type Line struct {
	Room   *rooms.Room
	Source Source
	// Perceives reports whether a player takes the line in at all (a visual
	// line reaches nobody who cannot see). nil means everyone present does,
	// as for a heard line. A player who does not perceive it is neither
	// charged nor counted.
	Perceives func(u *users.UserRecord) bool
	// Exclude lists players the caller already leaves out. They are never
	// charged or counted, and they come back in Routing.Exclude.
	Exclude []int
}

// Routing is Plan's answer. Exclude goes to the room sender as its
// excludeUserIds; Delivered is every player who takes the line in and was
// charged for it.
type Routing struct {
	Exclude   []int
	Delivered []int
}

var (
	budgetMu sync.Mutex
	// budgetNext is the round each player may next hear an ambient line.
	// In memory only, on the shape of the item chatter listener cap
	// (behaviortree's listenerCapNext).
	budgetNext = map[int]uint64{}

	visitMu sync.Mutex
	// singleHeard is the room whose only idle line each player has seen
	// since walking in. RoundTick drops an entry once the player is anywhere
	// else, which ends the visit.
	singleHeard = map[int]int{}
)

// Sees is the Perceives for a visual line in room: a reader who makes out at
// least shapes takes it in, the judgement the visual senders make
// (messaging.ReaderSight).
func Sees(room *rooms.Room) func(*users.UserRecord) bool {
	return func(u *users.UserRecord) bool {
		return messaging.ReaderSight(u.Character, room) != messaging.SightNone
	}
}

// budgetWindow is the budget window in rounds for a room in zone: the knob
// divided by the zone's ambientmultiplier, at least 1. A window of 0 means
// the budget is off. hushed means the budget is on and the zone's
// multiplier is 0: the window is endless, so no budgeted line gets through.
func budgetWindow(zone string) (window uint64, hushed bool) {
	base := float64(configs.GetBalanceConfig().AmbientListenerBudgetRounds)
	if base <= 0 {
		return 0, false
	}
	mult := rooms.ZoneAmbientMultiplier(zone)
	if mult <= 0 {
		return 0, true
	}
	w := math.Round(base / mult)
	if w < 1 {
		w = 1
	}
	return uint64(w), false
}

// Plan decides who an ambient line reaches and charges each of them. A
// player over budget is added to Exclude: that player skips the line, and
// everyone else in the room still gets it.
func Plan(l Line) Routing {
	r := Routing{Exclude: slices.Clone(l.Exclude)}
	if l.Room == nil {
		return r
	}
	now := util.GetRoundCount()
	window, hushed := budgetWindow(l.Room.Zone)

	budgetMu.Lock()
	for _, uid := range l.Room.GetPlayers() {
		if slices.Contains(l.Exclude, uid) {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}
		if l.Perceives != nil && !l.Perceives(u) {
			continue
		}
		if hushed || (window > 0 && now < budgetNext[uid]) {
			r.Exclude = append(r.Exclude, uid)
			continue
		}
		if window > 0 {
			budgetNext[uid] = now + window
		}
		r.Delivered = append(r.Delivered, uid)
	}
	budgetMu.Unlock()

	return r
}

// AllHaveBudget reports whether every player in room could hear an ambient
// line now, without charging anyone. An NPC conversation starts only when
// this holds, so none is cut off halfway.
func AllHaveBudget(room *rooms.Room) bool {
	if room == nil {
		return true
	}
	window, hushed := budgetWindow(room.Zone)
	if window == 0 && !hushed {
		return true
	}
	players := room.GetPlayers()
	if len(players) == 0 {
		return true
	}
	if hushed {
		return false
	}
	now := util.GetRoundCount()
	budgetMu.Lock()
	defer budgetMu.Unlock()
	for _, uid := range players {
		if now < budgetNext[uid] {
			return false
		}
	}
	return true
}

// SingleLineHeard returns the players among players who have already seen
// roomId's only idle line during their current visit.
func SingleLineHeard(roomId int, players []int) []int {
	visitMu.Lock()
	defer visitMu.Unlock()
	var heard []int
	for _, uid := range players {
		if singleHeard[uid] == roomId {
			heard = append(heard, uid)
		}
	}
	return heard
}

// MarkSingleLineHeard records that userIds saw roomId's only idle line.
func MarkSingleLineHeard(roomId int, userIds []int) {
	visitMu.Lock()
	defer visitMu.Unlock()
	for _, uid := range userIds {
		singleHeard[uid] = roomId
	}
}

// RoundTick runs once a round from hooks.UserRoundTick. It ends the visits
// of players who left or logged out, and drops spent budget entries, so
// neither map grows past the players online.
func RoundTick(round uint64) {
	visitMu.Lock()
	for uid, roomId := range singleHeard {
		if u := users.GetByUserId(uid); u == nil || u.Character.RoomId != roomId {
			delete(singleHeard, uid)
		}
	}
	visitMu.Unlock()

	now := util.GetRoundCount()
	budgetMu.Lock()
	for uid, next := range budgetNext {
		if now >= next {
			delete(budgetNext, uid)
		}
	}
	budgetMu.Unlock()
}
```

Create `internal/ambient/testing.go`:

```go
package ambient

// ResetForTest empties the budget and the visits and returns a restore
// func. For tests in any package.
func ResetForTest() func() {
	budgetMu.Lock()
	origBudget := budgetNext
	budgetNext = map[int]uint64{}
	budgetMu.Unlock()

	visitMu.Lock()
	origVisits := singleHeard
	singleHeard = map[int]int{}
	visitMu.Unlock()

	return func() {
		budgetMu.Lock()
		budgetNext = origBudget
		budgetMu.Unlock()
		visitMu.Lock()
		singleHeard = origVisits
		visitMu.Unlock()
	}
}

// ChargeForTest sets the round userId may next hear an ambient line.
func ChargeForTest(userId int, nextRound uint64) {
	budgetMu.Lock()
	budgetNext[userId] = nextRound
	budgetMu.Unlock()
}
```

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go vet ./internal/ambient/
go test ./internal/ambient/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty (fix the test file's const alignment with `gofmt -w` if listed); all `ok`.

- [ ] **Step 5: Prove the budget test can fail**

Sabotage `Plan`: change `if hushed || (window > 0 && now < budgetNext[uid]) {` to `if hushed {`. Run `go test ./internal/ambient/ -run 'TestPlan_TwoLines|TestPlan_ASecond' -count=1`; expected red with `second line in the window: delivered [3171]`. Restore; green.

- [ ] **Step 6: Commit**

```bash
git add internal/ambient/ambient.go internal/ambient/testing.go internal/ambient/main_test.go internal/ambient/ambient_test.go
git commit -F - <<'EOF'
feat(ambient): #317 per-listener ambient budget and single-line visits

Plan charges each listener who takes a line in and returns the players
over budget for the caller's room sender to leave out, so sight gating and
names stay in the existing senders. The window is divided by the zone's
ambientmultiplier. AllHaveBudget gates conversation starts without
charging. SingleLineHeard and MarkSingleLineHeard hold the once-per-visit
rule; RoundTick ends visits and prunes spent entries.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 8: Mark mob idle lines ambient: `MobActor.Skip` and the `ambient` mob command (Opus)

**Files:** `internal/actions/actor_mob.go`, `internal/actions/room_lines.go`, `internal/actions/room_line_excludes_test.go`, `internal/actions/divergences.go`, `internal/mobcommands/emote.go`, `internal/mobcommands/say.go`, `internal/mobcommands/ambient.go`, `internal/mobcommands/mobcommands.go`, `internal/mobcommands/ambient_test.go`

Design: a player's emote or say, a mob's reactive or quest line and a conversation line all keep going through `mobcommands.Emote` / `Say` untouched. Only the idle path and the gossip tick queue `ambient <source> <emote|say> <text>`, a mob-only command (not authored in content) that asks `ambient.Plan` and sends through the same shared bodies with the over-budget players added to the room line's excludes.

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/room_line_excludes_test.go`:

```go
package actions

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
)

// A MobActor's Skip list (the listeners an ambient line is over budget for,
// #317) joins the room line's excludes; a player actor still excludes only
// itself.
func TestRoomLineExcludes(t *testing.T) {
	if got := roomLineExcludes(&MobActor{Skip: []int{4, 5}}); !slices.Equal(got, []int{0, 4, 5}) {
		t.Errorf("mob with Skip: %v, want [0 4 5]", got)
	}
	if got := roomLineExcludes(&MobActor{}); !slices.Equal(got, []int{0}) {
		t.Errorf("mob without Skip: %v, want [0]", got)
	}
	u := users.NewTestUser(7, "excl", "Excl", 0)
	if got := roomLineExcludes(NewUserActor(u)); !slices.Equal(got, []int{7}) {
		t.Errorf("player: %v, want [7]", got)
	}
}
```

Create `internal/mobcommands/ambient_test.go`:

```go
package mobcommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// seedAmbientCommand: the package fixture (users 1 and 2 with mob 100 in
// room 1), the budget pinned, room 1 lit, the round at 6000.
func seedAmbientCommand(t *testing.T, budget int) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.AmbientListenerBudgetRounds = configs.ConfigInt(budget)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(6000)
	room := rooms.LoadRoom(1)
	room.SkyLight = rooms.SkyLightPtr(0)
	room.Lamp = rooms.LampPtr(60)
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
}

func heard(userId int, needle string) int {
	n := 0
	for _, m := range events.DrainQueuedMessagesForTest(userId) {
		if strings.Contains(m, needle) {
			n++
		}
	}
	return n
}

func TestAmbientSay_BudgetSkipsOnlyTheListenerOverIt(t *testing.T) {
	seedAmbientCommand(t, 5)
	ambient.ChargeForTest(1, 6005)
	if handled, err := TryCommand("ambient", "mob_chatter say Fresh bread, warm from the oven!", 100); !handled || err != nil {
		t.Fatalf("ambient say: handled %v err %v", handled, err)
	}
	if n := heard(1, "Fresh bread"); n != 0 {
		t.Errorf("user 1 is over budget and must skip the line, heard it %d times", n)
	}
	if n := heard(2, "Fresh bread"); n != 1 {
		t.Errorf("user 2 has budget and must hear the line once, heard it %d times", n)
	}
}

func TestAmbientSay_TwoLinesInAWindowReachEachListenerOnce(t *testing.T) {
	seedAmbientCommand(t, 5)
	TryCommand("ambient", "mob_chatter say First line.", 100)
	TryCommand("ambient", "gossip say Second line.", 100)
	for _, uid := range []int{1, 2} {
		msgs := strings.Join(events.DrainQueuedMessagesForTest(uid), "")
		if !strings.Contains(msgs, "First line") || strings.Contains(msgs, "Second line") {
			t.Errorf("user %d read %q, want the first line only", uid, msgs)
		}
	}
}

func TestPlainSay_IsNeverBudgeted(t *testing.T) {
	seedAmbientCommand(t, 5)
	ambient.ChargeForTest(1, 6005)
	ambient.ChargeForTest(2, 6005)
	TryCommand("say", "This is a quest line.", 100)
	for _, uid := range []int{1, 2} {
		if n := heard(uid, "quest line"); n != 1 {
			t.Errorf("a plain say is not ambient; user %d heard it %d times, want 1", uid, n)
		}
	}
}

func TestAmbientEmote_BudgetSkipsTheListenerOverIt(t *testing.T) {
	seedAmbientCommand(t, 5)
	ambient.ChargeForTest(2, 6005)
	TryCommand("ambient", "mob_chatter emote rattles its bones; dust falls.", 100)
	if n := heard(1, "rattles its bones; dust falls"); n != 1 {
		t.Errorf("user 1 must see the whole emote once, saw it %d times", n)
	}
	if n := heard(2, "rattles its bones"); n != 0 {
		t.Errorf("user 2 is over budget, saw the emote %d times", n)
	}
}

func TestAmbient_RefusesAnUnknownSourceOrVerb(t *testing.T) {
	seedAmbientCommand(t, 0)
	if _, err := TryCommand("ambient", "tips say hello", 100); err == nil {
		t.Error("an unknown source must be an error, so a typo shows in the log")
	}
	if _, err := TryCommand("ambient", "mob_chatter wander", 100); err == nil {
		t.Error("only emote and say are ambient lines")
	}
}
```

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./internal/actions/ -run TestRoomLineExcludes -count=1
go test ./internal/mobcommands/ -run 'TestAmbient|TestPlainSay' -count=1
```

Expected: build failures, `unknown field Skip in struct literal of type MobActor` and `undefined: roomLineExcludes`; the mobcommands tests fail with `ambient say: handled false` (no such command).

- [ ] **Step 3: Implement the actions side**

`internal/actions/actor_mob.go`, replace:

```go
// MobActor adapts a *mobs.Mob so it satisfies the Actor interface.
type MobActor struct {
	Mob  *mobs.Mob
	Room *rooms.Room
}
```

with:

```go
// MobActor adapts a *mobs.Mob so it satisfies the Actor interface.
type MobActor struct {
	Mob  *mobs.Mob
	Room *rooms.Room
	// Skip lists players this actor's room line leaves out: the listeners an
	// ambient line is over budget for (#317, mobcommands.Ambient). Empty
	// for every other line.
	Skip []int
}
```

`internal/actions/room_lines.go`: in `SendSeen`, replace

```go
	room.SendTextVisualHidingNames(cat, text, names, actor.GetUserId())
}
```

with

```go
	room.SendTextVisualHidingNames(cat, text, names, roomLineExcludes(actor)...)
}
```

in `sendSpoken`, replace

```go
	room.SendTextHidingNames(cat, line, names, messaging.HideSpeakerNames)
}
```

with

```go
	room.SendTextHidingNames(cat, line, names, messaging.HideSpeakerNames, roomLineExcludes(actor)...)
}
```

and append to the file:

```go
// roomLineExcludes is who an actor's room line leaves out: the actor itself
// (a player does not read its own line; a mob's id 0 matches nobody), and,
// for a MobActor, every listener in Skip (an ambient line over budget,
// #317).
func roomLineExcludes(actor Actor) []int {
	ex := []int{actor.GetUserId()}
	if m, ok := actor.(*MobActor); ok {
		ex = append(ex, m.Skip...)
	}
	return ex
}
```

`internal/actions/divergences.go`, in `mobOnlyCommands`, after the line `"aid":            "mob-ai",` add:

```go
	"ambient":        "mob-ai: an idle emote or say counted against each listener's ambient budget (#317)",
```

- [ ] **Step 4: Implement the mob commands**

Replace the whole of `internal/mobcommands/emote.go` with:

```go
package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Emote sends a mob's emote through actions.SendSeen: seen by sight, a bare
// mention of its own name hidden at shapes too, and never deafen-filtered
// (NPC lines are authored content, owner ruling 6).
func Emote(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Don't bother if no players are present
	if room.PlayerCt() < 1 {
		return true, nil
	}

	return emoteAs(&actions.MobActor{Mob: mob, Room: room}, rest)
}

// emoteAs is Emote's body for a given actor, so an ambient emote (#317,
// ambient.go) sends the same line with its Skip list.
func emoteAs(actor *actions.MobActor, rest string) (bool, error) {
	if len(rest) == 0 {
		actions.SendSeen(actor, messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> emotes.`, actor.Mob.Character.Name), false)
		return true, nil
	}

	result := actions.Emote(rest)
	emoteText := rest
	if result.IsAlias {
		emoteText = result.AliasText
	} else if rest[0] == '@' && len(rest) > 1 {
		// The @ form, stripped as the player command strips it (#273).
		emoteText = rest[1:]
	}

	actions.SendSeen(actor, messaging.CategoryMobEmote,
		actions.FormatMobEmoteText(actor.Mob.Character.Name, emoteText), false)

	return true, nil
}
```

Replace the whole of `internal/mobcommands/say.go` with:

```go
package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Say speaks for a mob through actions.Say, which sends the room line with
// the mob's name hidden by each listener's sight and no deafen filter (NPC
// lines are authored content, owner ruling 6).
func Say(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Don't bother if no players are present
	if room.PlayerCt() < 1 {
		return true, nil
	}

	return sayAs(&actions.MobActor{Mob: mob, Room: room}, rest)
}

// sayAs is Say's body for a given actor, so an ambient say (#317,
// ambient.go) sends the same line with its Skip list.
func sayAs(actor *actions.MobActor, rest string) (bool, error) {
	actions.Say(actor, rest)
	return true, nil
}
```

Create `internal/mobcommands/ambient.go`:

```go
package mobcommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Ambient sends a mob's unprompted emote or say (#317), issued only by the
// idle handler and the gossip tick as `ambient <source> <emote|say> <text>`.
// Every listener over their ambient budget is left out of the room line;
// everyone else gets it exactly as a plain emote or say. A player's own
// command, a reaction, a quest line or a conversation line never comes this
// way, so none of them is ever budgeted. Content never authors it.
func Ambient(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	srcWord, after, _ := strings.Cut(strings.TrimSpace(rest), ` `)
	verb, text, _ := strings.Cut(strings.TrimSpace(after), ` `)

	src, ok := ambient.ParseSource(srcWord)
	if !ok {
		return false, fmt.Errorf(`ambient: unknown source %q`, srcWord)
	}
	verb = strings.ToLower(verb)
	if verb != `emote` && verb != `say` {
		return false, fmt.Errorf(`ambient: %q is not an ambient verb`, verb)
	}

	// Don't bother if no players are present
	if room.PlayerCt() < 1 {
		return true, nil
	}

	if verb == `emote` {
		// A hidden actor's emote reaches no one (actions.SendSeen), so it
		// must charge no one either.
		if mob.Character.IsHidden() {
			return true, nil
		}
		plan := ambient.Plan(ambient.Line{Room: room, Source: src, Perceives: ambient.Sees(room)})
		if len(plan.Delivered) == 0 {
			return true, nil
		}
		return emoteAs(&actions.MobActor{Mob: mob, Room: room, Skip: plan.Exclude}, text)
	}

	plan := ambient.Plan(ambient.Line{Room: room, Source: src})
	if len(plan.Delivered) == 0 {
		// Nobody would hear it: no reveal, no echo next door, no
		// Communication event.
		return true, nil
	}
	return sayAs(&actions.MobActor{Mob: mob, Room: room, Skip: plan.Exclude}, text)
}
```

`internal/mobcommands/mobcommands.go`, after the line `"aid":            {Aid, false},` add:

```go
		"ambient":        {Ambient, true},
```

- [ ] **Step 5: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/actions/ ./internal/mobcommands/ ./internal/ambient/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty; all `ok`. `TestSpeechWrappersDoNotReFork` stays green: `emote.go` still calls `actions.SendSeen(` twice and `say.go` `actions.Say(` once.

If `TestAmbientEmote_BudgetSkipsTheListenerOverIt` reports user 1 saw nothing, the fixture room is not lit for a mob emote: check `rooms.LoadRoom(1).Biome` in `seedAllRegistries` (`internal/mobcommands/mobcommands_test.go`) and whether biome `city` is seeded there; seed it the way `internal/hooks/hooks_test.go:174-180` does (`rooms.SeedBiomesForTest` with a `city` biome) inside `seedAmbientCommand`. Do not weaken the assertion.

- [ ] **Step 6: Prove the skip reaches the room line**

Sabotage `sendSpoken`: restore its old last line (drop `roomLineExcludes(actor)...`). Run `go test ./internal/mobcommands/ -run TestAmbientSay_BudgetSkips -count=1`; expected red with `user 1 is over budget and must skip the line, heard it 1 times`. Restore; green.

- [ ] **Step 7: Commit**

```bash
git add internal/actions/actor_mob.go internal/actions/room_lines.go internal/actions/room_line_excludes_test.go internal/actions/divergences.go internal/mobcommands/emote.go internal/mobcommands/say.go internal/mobcommands/ambient.go internal/mobcommands/mobcommands.go internal/mobcommands/ambient_test.go
git commit -F - <<'EOF'
feat(mobcommands): #317 ambient mob command for idle emotes and says

`ambient <source> <emote|say> <text>` asks ambient.Plan and sends through
the shared SendSeen and Say bodies with the over-budget listeners in the
new MobActor.Skip list. Plain emote and say, reactions, quest and
conversation lines are untouched. Registered mob-only.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 8A: Budget the echo of an ambient say (Opus)

Owner ruling 2026-10-30. When an ambient say (the `ambient` mob command: idle chatter and gossip) echoes "You hear someone talking." into neighbouring rooms, each listener there is charged against the same per-listener budget and left out if over it; telemetry counts it under source `say_echo` in the receiving room. Every other say (players, reactions, quests, conversations) echoes exactly as today. See spec deviation 9: the echo is quiet, so only a player with the `superhearing` flag takes it in, and only such a player is charged.

Design: `rooms` cannot import `ambient` (ambient imports rooms), so `Room.SendTextToExitsExcluding` takes a callback that names the excludes for each neighbouring room; `SendTextToExits` delegates to it with a fixed list. `MobActor` gains `EchoExclude func(*rooms.Room) []int`, which only `mobcommands.Ambient` sets; `actions.Say` uses it when present.

**Files:** `internal/rooms/rooms.go`, `internal/rooms/send_to_exits_test.go`, `internal/ambient/ambient.go`, `internal/actions/actor_mob.go`, `internal/actions/say.go`, `internal/actions/say_echo_test.go`, `internal/mobcommands/ambient.go`, `internal/mobcommands/ambient_echo_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/rooms/send_to_exits_test.go`:

```go
package rooms

import (
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
)

// SendTextToExitsExcluding asks for the excludes once per neighbouring room
// with an exit back, and puts them on that room's message only (#317).
func TestSendTextToExitsExcluding_PerNeighbourExcludes(t *testing.T) {
	a := &Room{RoomId: 3190, Exits: map[string]exit.RoomExit{"north": {RoomId: 3191}, "east": {RoomId: 3192}}}
	b := &Room{RoomId: 3191, Exits: map[string]exit.RoomExit{"south": {RoomId: 3190}}}
	c := &Room{RoomId: 3192, Exits: map[string]exit.RoomExit{"west": {RoomId: 3190}}}
	defer SeedRoomsForTest(map[int]*Room{3190: a, 3191: b, 3192: c}, map[string]*ZoneConfig{})()
	defer events.DrainAllQueuedEventsForTest()

	asked := map[int]int{}
	a.SendTextToExitsExcluding("You hear someone talking.", true, func(tgt *Room) []int {
		asked[tgt.RoomId]++
		return []int{tgt.RoomId * 10}
	})
	for _, tc := range []struct {
		room    int
		exit    string
		exclude int
	}{{3191, "south", 31910}, {3192, "west", 31920}} {
		msgs := events.DrainQueuedRoomMessagesForTest(tc.room)
		if len(msgs) != 1 {
			t.Fatalf("room %d: %d messages, want 1", tc.room, len(msgs))
		}
		m := msgs[0]
		if !strings.Contains(m.Text, tc.exit) || !strings.Contains(m.Text, "You hear someone talking.") || !m.IsQuiet {
			t.Errorf("room %d message %+v", tc.room, m)
		}
		if !slices.Equal(m.ExcludeUserIds, []int{tc.exclude}) {
			t.Errorf("room %d excludes %v, want only its own [%d]", tc.room, m.ExcludeUserIds, tc.exclude)
		}
		if asked[tc.room] != 1 {
			t.Errorf("room %d: excludes asked %d times, want once", tc.room, asked[tc.room])
		}
	}

	// The plain sender still puts one list on every neighbour.
	a.SendTextToExits("A bell rings.", false, 7)
	for _, room := range []int{3191, 3192} {
		msgs := events.DrainQueuedRoomMessagesForTest(room)
		if len(msgs) != 1 || !slices.Equal(msgs[0].ExcludeUserIds, []int{7}) {
			t.Errorf("SendTextToExits room %d: %+v", room, msgs)
		}
	}
}
```

Create `internal/mobcommands/ambient_echo_test.go`:

```go
package mobcommands

import (
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const keenEarsConditionId = 3170

// seedEchoNeighbour: the ambient fixture with user 2 moved next door into
// room 2 (room 1's north neighbour, which leads back south) and given the
// superhearing flag, the only thing that lets a quiet echo reach a player.
func seedEchoNeighbour(t *testing.T, budget int) {
	t.Helper()
	seedAmbientCommand(t, budget)
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		keenEarsConditionId: {ConditionId: keenEarsConditionId, Name: "Keen Ears", RoundInterval: 1, TriggerCount: 1000,
			Flags: []conditions.Flag{conditions.SuperHearing}},
	}))
	u2 := users.GetByUserId(2)
	rooms.LoadRoom(1).RemovePlayer(2)
	u2.Character.RoomId = 2
	rooms.LoadRoom(2).AddPlayer(2)
	u2.Character.Conditions.AddCondition(keenEarsConditionId, false)
	if !u2.Character.HasConditionFlag(conditions.SuperHearing) {
		t.Fatal("fixture: user 2 must hold the superhearing flag")
	}
	events.DrainQueuedRoomMessagesForTest(2)
}

func echoTo(t *testing.T, roomId int) events.Message {
	t.Helper()
	for _, m := range events.DrainQueuedRoomMessagesForTest(roomId) {
		if strings.Contains(m.Text, "You hear someone talking.") {
			return m
		}
	}
	t.Fatalf("no echo queued for room %d", roomId)
	return events.Message{}
}

func TestAmbientSayEcho_OverBudgetNeighbourIsLeftOut(t *testing.T) {
	seedEchoNeighbour(t, 5)
	ambient.ChargeForTest(2, 6005)
	TryCommand("ambient", "mob_chatter say Fresh bread!", 100)
	if m := echoTo(t, 2); !slices.Contains(m.ExcludeUserIds, 2) {
		t.Fatalf("user 2 is over budget and must be left out of the echo, excludes %v", m.ExcludeUserIds)
	}
}

func TestAmbientSayEcho_InBudgetNeighbourHearsAndIsCharged(t *testing.T) {
	seedEchoNeighbour(t, 5)
	TryCommand("ambient", "gossip say Heard about the bridge?", 100)
	if m := echoTo(t, 2); slices.Contains(m.ExcludeUserIds, 2) {
		t.Fatalf("user 2 has budget and must get the echo, excludes %v", m.ExcludeUserIds)
	}
	// The echo charged user 2. (A second ambient say would not show it: user
	// 1 in the speaker's room is now over budget too, and a say nobody in its
	// own room hears is dropped whole, echo included.)
	if ambient.AllHaveBudget(rooms.LoadRoom(2)) {
		t.Fatal("the echo must have charged user 2 against the ambient budget")
	}
}

func TestPlainMobSayEcho_IsNeverBudgeted(t *testing.T) {
	seedEchoNeighbour(t, 5)
	ambient.ChargeForTest(2, 6005)
	TryCommand("say", "Halt, who goes there?", 100)
	if m := echoTo(t, 2); slices.Contains(m.ExcludeUserIds, 2) {
		t.Fatalf("a plain say is not ambient; its echo must not leave anyone out, excludes %v", m.ExcludeUserIds)
	}
}
```

Create `internal/actions/say_echo_test.go`:

```go
package actions

import (
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// A player's say still echoes next door unbudgeted (#317): even with the
// neighbour over budget, nobody is left out of the echo.
func TestPlayerSayEcho_IsNeverBudgeted(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.AmbientListenerBudgetRounds = 5
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(6000)

	here := &rooms.Room{RoomId: 3195, Exits: map[string]exit.RoomExit{"north": {RoomId: 3196}}}
	next := &rooms.Room{RoomId: 3196, Exits: map[string]exit.RoomExit{"south": {RoomId: 3195}}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{3195: here, 3196: next}, map[string]*rooms.ZoneConfig{}))
	speaker := users.NewTestUser(31951, "echospk", "Echospk", 0)
	speaker.Character.RoomId = 3195
	neighbour := users.NewTestUser(31961, "echonb", "Echonb", 0)
	neighbour.Character.RoomId = 3196
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{31951: speaker, 31961: neighbour}))
	here.AddPlayer(31951)
	next.AddPlayer(31961)
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	ambient.ChargeForTest(31961, 6005)

	Say(NewUserActorInRoom(speaker, here), "Hello?")
	var echo *events.Message
	for _, m := range events.DrainQueuedRoomMessagesForTest(3196) {
		if strings.Contains(m.Text, "You hear someone talking.") {
			echo = &m
			break
		}
	}
	if echo == nil {
		t.Fatal("a player's say must still echo into the neighbouring room")
	}
	if slices.Contains(echo.ExcludeUserIds, 31961) {
		t.Fatalf("a player's say is never budgeted, excludes %v", echo.ExcludeUserIds)
	}
}
```

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./internal/rooms/ -run TestSendTextToExitsExcluding -count=1
go test ./internal/mobcommands/ -run 'TestAmbientSayEcho|TestPlainMobSayEcho' -count=1
go test ./internal/actions/ -run TestPlayerSayEcho -count=1
```

Expected: `a.SendTextToExitsExcluding undefined`; `TestAmbientSayEcho_OverBudgetNeighbourIsLeftOut` fails with `user 2 is over budget and must be left out of the echo, excludes []`; the plain-say and player-say tests already pass (they pin today's behaviour, and must stay green after Step 3).

- [ ] **Step 3: Implement**

`internal/rooms/rooms.go`, replace the whole `SendTextToExits` function:

```go
func (r *Room) SendTextToExits(txt string, isQuiet bool, excludeUserIds ...int) {

	testExitIds := []int{}
	for _, rExit := range r.Exits {
		testExitIds = append(testExitIds, rExit.RoomId)
	}
	for _, tExit := range r.ExitsTemp {
		testExitIds = append(testExitIds, tExit.RoomId)
	}

	for _, roomId := range testExitIds {

		tgtRoom := LoadRoom(roomId)
		if tgtRoom == nil {
			continue
		}

		for exitName, tExit := range tgtRoom.Exits {
			if tExit.RoomId != r.RoomId {
				continue
			}

			events.AddToQueue(events.Message{
				RoomId:         tgtRoom.RoomId,
				Text:           fmt.Sprintf(`(From <ansi fg="exit">%s</ansi>) `, exitName) + txt + "\n",
				IsQuiet:        isQuiet,
				ExcludeUserIds: excludeUserIds,
			})
		}

	}

}
```

with:

```go
func (r *Room) SendTextToExits(txt string, isQuiet bool, excludeUserIds ...int) {
	r.SendTextToExitsExcluding(txt, isQuiet, func(*Room) []int { return excludeUserIds })
}

// SendTextToExitsExcluding is SendTextToExits with the excludes named per
// neighbouring room: exclude is asked once for each neighbour that has an
// exit back, the first time one is found, and its answer goes on that
// room's messages only. An ambient say's echo uses it to leave out the
// listeners over their ambient budget in each room (#317).
func (r *Room) SendTextToExitsExcluding(txt string, isQuiet bool, exclude func(tgt *Room) []int) {

	testExitIds := []int{}
	for _, rExit := range r.Exits {
		testExitIds = append(testExitIds, rExit.RoomId)
	}
	for _, tExit := range r.ExitsTemp {
		testExitIds = append(testExitIds, tExit.RoomId)
	}

	excludes := map[int][]int{}
	for _, roomId := range testExitIds {

		tgtRoom := LoadRoom(roomId)
		if tgtRoom == nil {
			continue
		}

		for exitName, tExit := range tgtRoom.Exits {
			if tExit.RoomId != r.RoomId {
				continue
			}

			ex, asked := excludes[tgtRoom.RoomId]
			if !asked {
				ex = exclude(tgtRoom)
				excludes[tgtRoom.RoomId] = ex
			}

			events.AddToQueue(events.Message{
				RoomId:         tgtRoom.RoomId,
				Text:           fmt.Sprintf(`(From <ansi fg="exit">%s</ansi>) `, exitName) + txt + "\n",
				IsQuiet:        isQuiet,
				ExcludeUserIds: ex,
			})
		}

	}

}
```

`internal/ambient/ambient.go`: replace the `Source` constants and `Sources`:

```go
const (
	SourceRoomIdle    Source = "room_idle"
	SourceMobChatter  Source = "mob_chatter"
	SourceWeather     Source = "weather"
	SourceGossip      Source = "gossip"
	SourceItemChatter Source = "item_chatter"
)

// Sources lists every counted source, in report order.
var Sources = []Source{SourceRoomIdle, SourceMobChatter, SourceWeather, SourceGossip, SourceItemChatter}
```

with:

```go
const (
	SourceRoomIdle    Source = "room_idle"
	SourceMobChatter  Source = "mob_chatter"
	SourceWeather     Source = "weather"
	SourceGossip      Source = "gossip"
	SourceItemChatter Source = "item_chatter"
	// SourceSayEcho is "You hear someone talking." in a room next to an
	// ambient say, counted in the receiving room (owner 2026-10-30).
	SourceSayEcho Source = "say_echo"
)

// Sources lists every counted source, in report order.
var Sources = []Source{SourceRoomIdle, SourceMobChatter, SourceWeather, SourceGossip, SourceItemChatter, SourceSayEcho}
```

and append to the same file (adding `"github.com/GoMudEngine/GoMud/internal/conditions"` to its imports):

```go
// HearsQuiet is the Perceives for a quiet line (a say's echo next door):
// the delivery hook gives a quiet message only to a player with the
// superhearing flag (hooks/Message_SendMessages.go), so only such a player
// takes it in and is charged.
func HearsQuiet(u *users.UserRecord) bool {
	return u.Character.HasConditionFlag(conditions.SuperHearing)
}
```

`internal/actions/actor_mob.go`, replace the Task 8 struct:

```go
	// Skip lists players this actor's room line leaves out: the listeners an
	// ambient line is over budget for (#317, mobcommands.Ambient). Empty
	// for every other line.
	Skip []int
}
```

with:

```go
	// Skip lists players this actor's room line leaves out: the listeners an
	// ambient line is over budget for (#317, mobcommands.Ambient). Empty
	// for every other line.
	Skip []int
	// EchoExclude, when set, names the players to leave out of a say's
	// "You hear someone talking." echo in each neighbouring room. Only an
	// ambient say sets it (#317); every other say echoes to everyone.
	EchoExclude func(*rooms.Room) []int
}
```

`internal/actions/say.go`, replace:

```go
	room := actor.GetRoom()
	room.SendTextToExits(`You hear someone talking.`, true)
```

with:

```go
	room := actor.GetRoom()
	sendSayEcho(actor, room)
```

and append:

```go
// sayEchoText is what a neighbouring room hears of a say.
const sayEchoText = `You hear someone talking.`

// sendSayEcho echoes a say into the neighbouring rooms. An ambient say
// (a MobActor with EchoExclude, #317) leaves out each neighbour's
// over-budget listeners; every other say echoes exactly as before.
func sendSayEcho(actor Actor, room *rooms.Room) {
	if m, ok := actor.(*MobActor); ok && m.EchoExclude != nil {
		room.SendTextToExitsExcluding(sayEchoText, true, m.EchoExclude)
		return
	}
	room.SendTextToExits(sayEchoText, true)
}
```

adding `"github.com/GoMudEngine/GoMud/internal/rooms"` to `say.go`'s imports.

`internal/mobcommands/ambient.go`, replace the say branch's last line:

```go
	return sayAs(&actions.MobActor{Mob: mob, Room: room, Skip: plan.Exclude}, text)
}
```

with:

```go
	// The echo next door is ambient too: each neighbour's listeners are
	// charged and an over-budget one is left out (owner 2026-10-30).
	echo := func(tgt *rooms.Room) []int {
		return ambient.Plan(ambient.Line{Room: tgt, Source: ambient.SourceSayEcho, Perceives: ambient.HearsQuiet}).Exclude
	}
	return sayAs(&actions.MobActor{Mob: mob, Room: room, Skip: plan.Exclude, EchoExclude: echo}, text)
}
```

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/rooms/ ./internal/actions/ ./internal/mobcommands/ ./internal/ambient/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty; all `ok`. `TestSpeechWrappersDoNotReFork` stays green (`actions/say.go` is not a wrapper file, and the wrappers are unchanged). If `echoTo` finds no echo for room 2, check the mobcommands fixture's room 2 has an exit back to room 1 (`internal/mobcommands/mobcommands_test.go`, `room2.Exits`); if it has none, add `"south": {RoomId: 1}` to room 2's exits inside `seedEchoNeighbour` before moving user 2, rather than changing the shared fixture.

- [ ] **Step 5: Prove the echo test can fail**

Sabotage `sendSayEcho`: delete its `if m, ok := actor.(*MobActor); ok && m.EchoExclude != nil { ... }` block. Run `go test ./internal/mobcommands/ -run 'TestAmbientSayEcho' -count=1`; expected red with `user 2 is over budget and must be left out of the echo, excludes []`. Restore; green.

- [ ] **Step 6: Commit**

```bash
git add internal/rooms/rooms.go internal/rooms/send_to_exits_test.go internal/ambient/ambient.go internal/actions/actor_mob.go internal/actions/say.go internal/actions/say_echo_test.go internal/mobcommands/ambient.go internal/mobcommands/ambient_echo_test.go
git commit -F - <<'EOF'
feat: #317 budget the echo of an ambient say next door

Owner ruling 2026-10-30. An ambient say's "You hear someone talking."
echo charges each neighbouring room's listeners against the ambient budget
and leaves out the over-budget ones; telemetry source say_echo, counted in
the receiving room. Room.SendTextToExitsExcluding names excludes per
neighbour; MobActor.EchoExclude is set only by the ambient command, so
every other say echoes as before. The echo is quiet, so only a player
with the superhearing flag takes it in or is charged.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 9: Wire the split roll and gossip into `HandleIdleMobs` (Opus)

**Files:** `internal/hooks/MobIdle_idle_roll.go`, `internal/hooks/MobIdle_HandleIdleMobs.go`, `internal/hooks/idle_roll_wiring_test.go`, `internal/mobs/mobs.go`, `internal/mobs/mobs_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/hooks/idle_roll_wiring_test.go`:

```go
package hooks

import (
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gossip"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// idleMobFixture: the hooks fixture's Skeleton (instance 100, room 1,
// TestZone) with the given activity level and pool and the two idle scales
// pinned.
func idleMobFixture(t *testing.T, al int, pool []string, chatter, movement float64) *mobs.Mob {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.MobIdleChatterScale = configs.ConfigFloat(chatter)
	cfg.Balance.MobIdleMovementScale = configs.ConfigFloat(movement)
	configs.SetConfigForTest(t, cfg)
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.ActivityLevel = al
	m.IdleCommands = pool
	events.DrainQueuedInputsForTest(100)
	return m
}

func idleInputs(rounds int) []string {
	var all []string
	for i := 0; i < rounds; i++ {
		HandleIdleMobs(events.MobIdle{MobInstanceId: 100})
		all = append(all, events.DrainQueuedInputsForTest(100)...)
	}
	return all
}

func TestHandleIdleMobs_IdleEmoteGoesOutAsAmbient(t *testing.T) {
	idleMobFixture(t, 100, []string{"emote rattles its bones."}, 1, 1)
	got := idleInputs(1)
	require.Contains(t, got, "ambient mob_chatter emote rattles its bones.")
	require.NotContains(t, got, "lookfortrouble", "a round that picked a command does not also look for trouble")
}

func TestHandleIdleMobs_SemicolonStaysInOneLine(t *testing.T) {
	idleMobFixture(t, 100, []string{"emote plunges the iron in the trough; steam rises."}, 1, 1)
	got := idleInputs(1)
	require.Contains(t, got, "ambient mob_chatter emote plunges the iron in the trough; steam rises.")
	for _, in := range got {
		require.False(t, strings.HasPrefix(strings.TrimSpace(in), "steam"), "the text after ';' ran as its own command: %v", got)
	}
}

func TestHandleIdleMobs_OtherChatterVerbsAreNotWrapped(t *testing.T) {
	idleMobFixture(t, 100, []string{"consume corpse"}, 1, 1)
	require.Contains(t, idleInputs(1), "consume corpse")
}

func TestHandleIdleMobs_ChatterScaleZeroSilences(t *testing.T) {
	idleMobFixture(t, 100, []string{"emote rattles its bones.", "say Boo."}, 0, 1)
	got := idleInputs(200)
	for _, in := range got {
		require.False(t, strings.HasPrefix(in, "ambient "), "MobIdleChatterScale 0 must silence idle chatter, got %q", in)
	}
	require.Equal(t, 200, countExact(got, "lookfortrouble"), "every silent round looks for trouble")
}

func TestHandleIdleMobs_MovementRollsApartFromChatter(t *testing.T) {
	idleMobFixture(t, 100, []string{"wander"}, 0, 1)
	require.Contains(t, idleInputs(1), "wander", "chatter at 0 must not stop movement")

	idleMobFixture(t, 100, []string{"wander"}, 1, 0)
	require.NotContains(t, idleInputs(200), "wander", "MobIdleMovementScale 0 must stop idle movement")
}

func TestHandleIdleMobs_WanderMultipliers(t *testing.T) {
	m := idleMobFixture(t, 100, []string{"wander"}, 1, 1)
	m.WanderMultiplier = rooms.MultiplierPtr(0)
	require.NotContains(t, idleInputs(200), "wander", "mob wandermultiplier 0 must stop its idle movement")

	idleMobFixture(t, 100, []string{"wander"}, 1, 1)
	rooms.GetZoneConfig("TestZone").WanderMultiplier = rooms.MultiplierPtr(0)
	require.NotContains(t, idleInputs(200), "wander", "zone wandermultiplier 0 must stop idle movement there")

	idleMobFixture(t, 100, []string{"emote hums."}, 1, 1)
	rooms.GetZoneConfig("TestZone").AmbientMultiplier = rooms.MultiplierPtr(0)
	for _, in := range idleInputs(200) {
		require.False(t, strings.HasPrefix(in, "ambient "), "zone ambientmultiplier 0 must silence idle chatter, got %q", in)
	}
}

func countExact(list []string, want string) int {
	n := 0
	for _, s := range list {
		if s == want {
			n++
		}
	}
	return n
}

func gossiperFixture(t *testing.T, interval int) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.GossipIntervalRounds = configs.ConfigInt(interval)
	configs.SetConfigForTest(t, cfg)
	g := &mobs.Mob{
		MobId: 114, InstanceId: 200, HomeRoomId: 1, ActivityLevel: 7,
		Groups: []string{"humanoid", "gossiper"},
		Character: characters.Character{
			Name: "old Fen", Zone: "TestZone", RoomId: 1, Health: 50,
			Conditions: conditions.New(), Cooldowns: map[string]int{},
		},
	}
	g.Character.HealthMax.Value = 100
	t.Cleanup(mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{200: g}))
	t.Cleanup(gossip.SeedForTest(map[string][]string{"fallback": {"Quiet day."}}))
	events.DrainQueuedInputsForTest(200)
}

func TestHandleIdleMobs_GossipGoesOutAsAmbient(t *testing.T) {
	gossiperFixture(t, 75)
	HandleIdleMobs(events.MobIdle{MobInstanceId: 200})
	got := events.DrainQueuedInputsForTest(200)
	require.True(t, slices.Contains(got, "ambient gossip say Quiet day."), "gossip must go through the ambient command, got %v", got)
}

func TestHandleIdleMobs_GossipZeroIsOff(t *testing.T) {
	gossiperFixture(t, 0)
	HandleIdleMobs(events.MobIdle{MobInstanceId: 200})
	for _, in := range events.DrainQueuedInputsForTest(200) {
		require.NotContains(t, in, "Quiet day", "GossipIntervalRounds 0 must stop gossip")
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/hooks/ -run 'TestHandleIdleMobs_' -count=1
```

Expected: `TestHandleIdleMobs_IdleEmoteGoesOutAsAmbient` fails (`"emote rattles its bones."` arrives unwrapped); `TestHandleIdleMobs_GossipGoesOutAsAmbient` fails (`say Quiet day.`); the zero-scale tests fail (the scales are not read yet).

- [ ] **Step 3: Implement**

Create `internal/hooks/MobIdle_idle_roll.go`:

```go
package hooks

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// idleRollInput gathers the multipliers for one mob's idle round (#317): the
// two global scales, the zone of the room the mob stands in (an instance
// clone keeps its template zone), and the mob's own wandermultiplier.
func idleRollInput(mob *mobs.Mob, room *rooms.Room) mobs.IdleRollInput {
	b := configs.GetBalanceConfig()
	in := mobs.IdleRollInput{
		ChatterScale:  float64(b.MobIdleChatterScale),
		MovementScale: float64(b.MobIdleMovementScale),
		ZoneAmbient:   1,
		ZoneWander:    1,
		MobWander:     mob.WanderScale(),
	}
	if room != nil {
		in.ZoneAmbient = rooms.ZoneAmbientMultiplier(room.Zone)
		in.ZoneWander = rooms.ZoneWanderMultiplier(room.Zone)
	}
	return in
}

// issueIdleChatter queues an idle chatter command. An emote or say is an
// ambient line: it goes out whole (a ';' inside the text is kept) through
// the `ambient` mob command, so the listener budget applies. Any other
// chatter verb (consume, cast...) is queued as before.
func issueIdleChatter(mob *mobs.Mob, cmd string) {
	switch mobs.IdleVerb(cmd) {
	case `emote`, `say`:
		mob.CommandNoSplit(`ambient ` + string(ambient.SourceMobChatter) + ` ` + strings.TrimSpace(cmd))
	default:
		mob.Command(cmd)
	}
}
```

In `internal/hooks/MobIdle_HandleIdleMobs.go`, replace the gossip send:

```go
			if line != "" {
				mob.Command("say " + line)
				mob.SetTempData("lastGossipRound", roundNow)
			}
```

with:

```go
			if line != "" {
				// Gossip is ambient (#317): the listener budget applies.
				mob.CommandNoSplit("ambient " + string(ambient.SourceGossip) + " say " + line)
				mob.SetTempData("lastGossipRound", roundNow)
			}
```

and replace the idle roll:

```go
		//
		// Look for trouble
		//
		idleCmd := `lookfortrouble`
		if util.Rand(100) < mob.ActivityLevel {
			idleCmd = mob.GetIdleCommand()
			if idleCmd == `` {
				idleCmd = `lookfortrouble`
			}
		}
		mob.Command(idleCmd)
```

with:

```go
		//
		// Idle roll (#317): chatter and movement roll apart, each weighted
		// by its share of the pool, so at every multiplier 1.0 a mob speaks
		// and moves as often as the old single roll let it. A round where
		// neither roll picks a command looks for trouble.
		//
		idleRoom := rooms.LoadRoom(mob.Character.RoomId)
		chatterCmd, moveCmd := mob.RollIdle(idleRollInput(mob, idleRoom), util.Rand)
		if chatterCmd == `` && moveCmd == `` {
			mob.Command(`lookfortrouble`)
			return events.Continue
		}
		if chatterCmd != `` {
			issueIdleChatter(mob, chatterCmd)
		}
		if moveCmd != `` {
			mob.Command(moveCmd)
		}
```

and add `"github.com/GoMudEngine/GoMud/internal/ambient"` to that file's imports.

Delete the now unused `GetIdleCommand` from `internal/mobs/mobs.go` (the whole function at lines 1200-1216, from `func (m *Mob) GetIdleCommand() string {` to its closing `}`), and delete `func TestGetIdleCommand` (its whole body, lines 353-377) from `internal/mobs/mobs_test.go`, keeping the `// ─── Idle/Angry Commands ───` header for `TestGetAngryCommand`. Then let the compiler confirm no caller remains:

```bash
go build ./... && go vet ./internal/mobs/ ./internal/hooks/
```

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ modules/
go test ./internal/hooks/ ./internal/mobs/ ./internal/mobcommands/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty; all `ok`, including the existing `TestHandleIdleMobs_ValidMob` and `TestHandleIdleMobs_GossiperMob`.

- [ ] **Step 5: Prove the wiring test can fail**

Sabotage `issueIdleChatter`: make the `emote`, `say` case call `mob.Command(cmd)`. Run `go test ./internal/hooks/ -run 'TestHandleIdleMobs_IdleEmote|TestHandleIdleMobs_Semicolon' -count=1`; expected red, the semicolon test naming the stray `steam rises.` command. Restore; green.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/MobIdle_idle_roll.go internal/hooks/MobIdle_HandleIdleMobs.go internal/hooks/idle_roll_wiring_test.go internal/mobs/mobs.go internal/mobs/mobs_test.go
git commit -F - <<'EOF'
feat(hooks): #317 split idle roll and ambient idle chatter and gossip

HandleIdleMobs uses Mob.RollIdle with the global scales, the room's zone
multipliers and the mob's wandermultiplier; an idle emote or say and every
gossip line go out whole through the ambient mob command. GetIdleCommand
and its 1% forced blank are gone.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 10: Room idle lines: knob, zone scale, budget, once per visit (Sonnet)

**Files:** `internal/hooks/NewRound_RoomIdle.go`, `internal/hooks/NewRound_UserRoundTick.go`, `internal/hooks/room_idle_ambient_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/hooks/room_idle_ambient_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// roomIdleFixture: the hooks fixture with room 1 lit and given idle lines,
// RoomIdleChancePct and the budget pinned, a fresh budget, a seeded die.
func roomIdleFixture(t *testing.T, lines []string, pct, budget int) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.RoomIdleChancePct = configs.ConfigInt(pct)
	cfg.Balance.AmbientListenerBudgetRounds = configs.ConfigInt(budget)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())
	t.Cleanup(util.SetRandForTest(317))
	t.Cleanup(util.ResetRoundCountForTest)
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(90)
	room.IdleMessages = lines
	for _, id := range []int{1, 2} {
		require.NoError(t, users.GetByUserId(id).Character.Validate())
	}
	drainPlain(1)
	drainPlain(2)
}

func tickRounds(from, to uint64) {
	for r := from; r <= to; r++ {
		util.SetRoundCountForTest(r)
		UserRoundTick(events.NewRound{RoundNumber: r})
	}
}

func TestRoomIdle_SingleLineOncePerVisit(t *testing.T) {
	roomIdleFixture(t, []string{"A cart rattles past."}, 100, 0)
	tickRounds(1000, 1005)
	require.Equal(t, 1, countContaining(drainPlain(2), "cart rattles past"), "a one-line room shows its line once per visit")

	// User 2 walks out and back in: a new visit.
	u2 := users.GetByUserId(2)
	rooms.LoadRoom(1).RemovePlayer(2)
	u2.Character.RoomId = 2
	rooms.LoadRoom(2).AddPlayer(2)
	tickRounds(1006, 1006)
	rooms.LoadRoom(2).RemovePlayer(2)
	u2.Character.RoomId = 1
	rooms.LoadRoom(1).AddPlayer(2)
	tickRounds(1007, 1010)
	require.Equal(t, 1, countContaining(drainPlain(2), "cart rattles past"), "back in, user 2 sees the line once more")
	require.Equal(t, 1, countContaining(drainPlain(1), "cart rattles past"), "user 1 never left and saw it once in all")
}

func TestRoomIdle_ZeroChanceIsOff(t *testing.T) {
	roomIdleFixture(t, []string{"A cart rattles past.", "A dog barks.", "Bells ring."}, 0, 0)
	tickRounds(1000, 1050)
	got := drainPlain(2)
	require.Zero(t, countContaining(got, "cart rattles")+countContaining(got, "dog barks")+countContaining(got, "bells ring"),
		"RoomIdleChancePct 0 must show no idle line: %v", got)
}

func TestRoomIdle_BudgetCapsALongStay(t *testing.T) {
	lines := []string{"A cart rattles past.", "A dog barks.", "Bells ring."}
	count := func(budget int) int {
		roomIdleFixture(t, lines, 100, budget)
		tickRounds(1000, 1009)
		got := drainPlain(2)
		return countContaining(got, "cart rattles") + countContaining(got, "dog barks") + countContaining(got, "bells ring")
	}
	free := count(0)
	capped := count(5)
	require.GreaterOrEqual(t, free, 3, "without a budget ten rounds at 100%% show several lines")
	require.LessOrEqual(t, capped, 2, "a 5-round budget allows at most two lines in ten rounds")
	require.GreaterOrEqual(t, capped, 1)
}

func TestRoomIdleChancePer10k(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	cfg := configs.GetConfig()
	cfg.Balance.RoomIdleChancePct = 3
	configs.SetConfigForTest(t, cfg)
	room := rooms.LoadRoom(1)
	require.Equal(t, 300, roomIdleChancePer10k(room))
	rooms.GetZoneConfig("TestZone").AmbientMultiplier = rooms.MultiplierPtr(2)
	require.Equal(t, 600, roomIdleChancePer10k(room), "zone ambientmultiplier 2.0 doubles the chance")
	rooms.GetZoneConfig("TestZone").AmbientMultiplier = rooms.MultiplierPtr(0)
	require.Equal(t, 0, roomIdleChancePer10k(room), "zone ambientmultiplier 0 hushes room idle lines")
	rooms.GetZoneConfig("TestZone").AmbientMultiplier = nil
	limbo := &rooms.Room{RoomId: -1, Zone: "TestZone"}
	require.Equal(t, 1200, roomIdleChancePer10k(limbo), "room -1 keeps four times the chance")
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/hooks/ -run 'TestRoomIdle' -count=1
```

Expected: build failure, `undefined: roomIdleChancePer10k`.

- [ ] **Step 3: Implement**

Create `internal/hooks/NewRound_RoomIdle.go`:

```go
package hooks

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// roomIdleChancePer10k is a room's chance per round, out of 10000, of an
// idle line (#317): Balance.RoomIdleChancePct, four times that for room -1,
// scaled by the zone's ambientmultiplier.
func roomIdleChancePer10k(room *rooms.Room) int {
	chance := float64(configs.GetBalanceConfig().RoomIdleChancePct)
	if room.RoomId == -1 {
		chance *= 4
	}
	chance *= rooms.ZoneAmbientMultiplier(room.Zone)
	return int(math.Round(chance * 100))
}

// sendRoomIdleLine sends one room idle line through the ambient budget
// (#317). A room with a single idle line shows it at most once per visit: a
// player who already saw it since walking in is left out and not charged.
func sendRoomIdleLine(room *rooms.Room, msg string, singleLine bool) {
	var already []int
	if singleLine {
		already = ambient.SingleLineHeard(room.RoomId, room.GetPlayers())
	}
	plan := ambient.Plan(ambient.Line{
		Room:      room,
		Source:    ambient.SourceRoomIdle,
		Perceives: ambient.Sees(room),
		Exclude:   already,
	})
	sendVisualRoomText(room, messaging.CategoryRoomDescription, msg, plan.Exclude...)
	if singleLine {
		ambient.MarkSingleLineHeard(room.RoomId, plan.Delivered)
	}
}
```

In `internal/hooks/NewRound_UserRoundTick.go`, replace:

```go
func UserRoundTick(e events.Event) events.ListenerReturn {

	roomsWithPlayers := rooms.GetRoomsWithPlayers()
```

with:

```go
func UserRoundTick(e events.Event) events.ListenerReturn {

	if evt, ok := e.(events.NewRound); ok {
		ambient.RoundTick(evt.RoundNumber)
	}

	roomsWithPlayers := rooms.GetRoomsWithPlayers()
```

replace:

```go
				if allowIdleMessages {

					chanceIn100 := 5
					if room.RoomId == -1 {
						chanceIn100 = 20
					}

					var idleMsgs []string
```

with:

```go
				if allowIdleMessages {

					var idleMsgs []string
```

replace:

```go
					if idleMsgCt > 0 && util.Rand(100) < chanceIn100 {
```

with:

```go
					if idleMsgCt > 0 && util.Rand(10000) < roomIdleChancePer10k(room) {
```

and replace:

```go
								msg := idleMsgs[idleMsgIndex]
								if msg != `` {
									wrappedMsg := util.SplitStringNL(msg, 80)
									// Idle flavor text is visual; the visual
									// pipeline judges each reader's sight, dark room
									// or lit. The dark-room branch that tested the
									// nightvision FLAG is gone (lighting plan 5c).
									sendVisualRoomText(room, messaging.CategoryRoomDescription, wrappedMsg)
								}
```

with:

```go
								msg := idleMsgs[idleMsgIndex]
								if msg != `` {
									// Idle flavor text is visual; the visual
									// pipeline judges each reader's sight, dark room
									// or lit. It is ambient: the listener budget and
									// the once-per-visit rule for a one-line room
									// apply (#317).
									sendRoomIdleLine(room, util.SplitStringNL(msg, 80), idleMsgCt == 1)
								}
```

Add `"github.com/GoMudEngine/GoMud/internal/ambient"` to that file's imports.

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/hooks/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty; all `ok`. If `TestRoomIdle_BudgetCapsALongStay` reports `free` below 3 for seed 317, that is the no-repeat rule skipping draws; print the per-round picks, and if the seeded sequence really skips that often, change both `util.SetRandForTest(317)` seeds in this file to `util.SetRandForTest(1317)` and record the reason in the test comment. Do not relax the `capped <= 2` assertion.

- [ ] **Step 5: Prove the once-per-visit test can fail**

Sabotage `sendRoomIdleLine`: change `if singleLine {` (the first one) to `if false {`. Run `go test ./internal/hooks/ -run TestRoomIdle_SingleLineOncePerVisit -count=1`; expected red with `a one-line room shows its line once per visit` (6 lines). Restore; green.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/NewRound_RoomIdle.go internal/hooks/NewRound_UserRoundTick.go internal/hooks/room_idle_ambient_test.go
git commit -F - <<'EOF'
feat(hooks): #317 room idle lines through the knob and the budget

RoomIdleChancePct replaces the hardcoded 5 (room -1 keeps 4x), scaled by
the zone's ambientmultiplier. Lines go through ambient.Plan; a room with
one idle line shows it once per visit. UserRoundTick runs ambient's round
upkeep.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 11: Weather, item chatter and conversation starts (Sonnet)

**Files:** `modules/weather/engine/emotes.go`, `modules/weather/engine/emotes_config_test.go`, `modules/weather/engine/emotes_ambient_test.go`, `modules/weather/weather_config.go`, `modules/weather/weather.go`, `modules/weather/weather_config_test.go`, `modules/weather/weather_shipped_test.go`, `_datafiles/config.yaml`, `internal/behaviortree/actions_item_voice.go`, `internal/behaviortree/item_voice_ambient_test.go`, `internal/hooks/NewRound_IdleMobs.go`, `internal/usercommands/go.go`

- [ ] **Step 1: Write the failing tests**

Append to `modules/weather/weather_config_test.go`:

```go
// #317: the seasonal layer's 1-in-N is a knob (0 = never), and a cadence of
// 0 turns the ambient pass off instead of clamping to 5.
func TestBuildConfig_SeasonalAndZeroCadence(t *testing.T) {
	c := buildConfig(func(string) any { return nil })
	if c.SeasonalEmoteOneIn != 3 {
		t.Errorf("SeasonalEmoteOneIn default = %d, want today's 3", c.SeasonalEmoteOneIn)
	}
	vals := map[string]any{"EmoteEveryRounds": 0, "SeasonalEmoteOneIn": 0}
	c = buildConfig(func(k string) any { return vals[k] })
	if c.EmoteEveryRounds != 0 || c.SeasonalEmoteOneIn != 0 {
		t.Errorf("0 must stay 0 (off): cadence %d seasonal %d", c.EmoteEveryRounds, c.SeasonalEmoteOneIn)
	}
	vals = map[string]any{"SeasonalEmoteOneIn": -2}
	c = buildConfig(func(k string) any { return vals[k] })
	if c.SeasonalEmoteOneIn != 0 {
		t.Errorf("a negative SeasonalEmoteOneIn clamps to 0, got %d", c.SeasonalEmoteOneIn)
	}
}
```

Create `modules/weather/weather_shipped_test.go`:

```go
package weather

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v2"
)

// The shipped Modules.weather block reads as the #317 retune: a pass every
// 48 rounds and a seasonal line on 1 pass in 4.
func TestShippedWeatherAmbientKnobs(t *testing.T) {
	_, self, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(self), "..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	modules, _ := doc["Modules"].(map[any]any)
	block, _ := modules["weather"].(map[any]any)
	if block == nil {
		t.Fatal("no Modules.weather block in config.yaml")
	}
	c := buildConfig(func(k string) any { return block[k] })
	if c.EmoteEveryRounds != 48 || c.SeasonalEmoteOneIn != 4 {
		t.Fatalf("shipped weather cadence %d seasonal 1 in %d, want 48 and 4", c.EmoteEveryRounds, c.SeasonalEmoteOneIn)
	}
}
```

In `modules/weather/engine/emotes_config_test.go`, replace the call

```go
		return EmitAmbient(g, fronts, simCfg, weather, nil, tables, nil, 100, 100, roll)
```

with

```go
		return EmitAmbient(g, fronts, simCfg, weather, nil, tables, nil, 100, 100, 3, roll)
```

Create `modules/weather/engine/emotes_ambient_test.go`:

```go
package engine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/modules/weather/content"
	"github.com/GoMudEngine/GoMud/modules/weather/seasons"
	"github.com/GoMudEngine/GoMud/modules/weather/sim"
)

// seedWeatherRoom: room 1 in zoneA with players 9001 and 9002, the budget
// pinned, the round at 7000.
func seedWeatherRoom(t *testing.T, budget int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.AmbientListenerBudgetRounds = configs.ConfigInt(budget)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())
	// An outdoor biome, so the seasonal table's outdoor "default" pool is
	// the one picked (an unknown biome falls back to whatever the registry
	// holds).
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"plains": {BiomeId: "plains", Name: "Plains"},
	}))
	room := &rooms.Room{RoomId: 1, Zone: "zoneA", Biome: "plains"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{1: room}, map[string]*rooms.ZoneConfig{}))
	seed := map[int]*users.UserRecord{}
	for _, id := range []int{9001, 9002} {
		u := users.NewTestUser(id, "w"+string(rune('a'+id-9001)), "W"+string(rune('a'+id-9001)), 0)
		u.Character.RoomId = 1
		seed[id] = u
		room.AddPlayer(id)
	}
	t.Cleanup(users.SeedUsersForTest(seed))
	rooms.MarkRoomOccupancy(1, 2, 0)
	t.Cleanup(util.ResetRoundCountForTest)
	util.SetRoundCountForTest(7000)
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	events.DrainQueuedMessagesForTest(9001)
	events.DrainQueuedMessagesForTest(9002)
}

var calmSeason = struct {
	zoneSeasons map[sim.ZoneId]seasons.ZoneSeason
	tables      content.SeasonalTables
}{
	zoneSeasons: map[sim.ZoneId]seasons.ZoneSeason{"zoneA": {Track: "temperate", Season: "winter"}},
	tables: content.SeasonalTables{
		{Track: "temperate", Season: "winter"}: {Outdoor: map[string][]string{"default": {"SEASON LINE"}}},
	},
}

func emitCalm(oneIn int, roll func(int) int) int {
	return EmitAmbient(nil, nil, sim.Config{}, map[sim.ZoneId]sim.WeatherType{},
		calmSeason.zoneSeasons, content.Tables{}, calmSeason.tables, 30, 100, oneIn, roll)
}

func TestEmitAmbient_SeasonalOneIn(t *testing.T) {
	seedWeatherRoom(t, 0)
	zero := func(int) int { return 0 }
	if sent := emitCalm(0, zero); sent != 0 {
		t.Fatalf("SeasonalEmoteOneIn 0 must never send a seasonal line, sent %d", sent)
	}
	if sent := emitCalm(1, zero); sent != 1 {
		t.Fatalf("SeasonalEmoteOneIn 1 sends on every pass, sent %d", sent)
	}
	one := func(int) int { return 1 }
	if sent := emitCalm(4, one); sent != 0 {
		t.Fatalf("a draw of 1 out of 4 is not the 1 in 4, sent %d", sent)
	}
}

func TestEmitAmbient_SeasonalLineHonoursTheBudget(t *testing.T) {
	seedWeatherRoom(t, 5)
	ambient.ChargeForTest(9001, 7005)
	emitCalm(1, func(int) int { return 0 })
	if got := strings.Join(events.DrainQueuedMessagesForTest(9001), ""); strings.Contains(got, "SEASON LINE") {
		t.Errorf("9001 is over budget and must skip the line, got %q", got)
	}
	if got := strings.Join(events.DrainQueuedMessagesForTest(9002), ""); !strings.Contains(got, "SEASON LINE") {
		t.Errorf("9002 has budget and must get the line, got %q", got)
	}
}
```

Create `internal/behaviortree/item_voice_ambient_test.go`:

```go
package behaviortree

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/configs"
)

func pinAmbientBudget(t *testing.T, rounds int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.AmbientListenerBudgetRounds = configs.ConfigInt(rounds)
	configs.SetConfigForTest(t, cfg)
	t.Cleanup(ambient.ResetForTest())
}

// An item's ambient (item_idle) line counts toward the listener budget
// (#317): a listener over budget skips the room line, and so does a holder
// over budget skip their own line.
func TestItemIdleLine_SkipsAListenerOverBudget(t *testing.T) {
	seedVoiceWorld(t)
	pinAmbientBudget(t, 5)
	ambient.ChargeForTest(voiceListenerId, 5005)
	if !TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1)) {
		t.Fatal("the idle line must still go out to the holder")
	}
	if got := drained(voiceListenerId); strings.Contains(got, "mutters") {
		t.Errorf("the listener is over budget and must skip the room line, got %q", got)
	}
	if got := drained(voiceBearerId); !strings.Contains(got, "says") {
		t.Errorf("the holder has budget and must read the item's line, got %q", got)
	}
}

func TestItemIdleLine_SkipsAHolderOverBudget(t *testing.T) {
	seedVoiceWorld(t)
	pinAmbientBudget(t, 5)
	ambient.ChargeForTest(voiceBearerId, 5005)
	TryItemBehavior(voiceEvent("item_idle"), wornProbe(voiceProbeItemId, 1))
	if got := drained(voiceBearerId); strings.Contains(got, "says") {
		t.Errorf("the holder is over budget and must skip the item's line, got %q", got)
	}
	if got := drained(voiceListenerId); !strings.Contains(got, "mutters") {
		t.Errorf("the listener has budget and must hear the room line, got %q", got)
	}
}

// An event line (on_equip) is not ambient and is never budgeted.
func TestItemEventLine_IsNeverBudgeted(t *testing.T) {
	seedVoiceWorld(t)
	pinAmbientBudget(t, 5)
	ambient.ChargeForTest(voiceBearerId, 5005)
	ambient.ChargeForTest(voiceListenerId, 5005)
	TryItemBehavior(voiceEvent("on_equip"), wornProbe(voiceProbeItemId, 1))
	if got := drained(voiceListenerId); !strings.Contains(got, "Up we go") {
		t.Errorf("an event line is exempt; the listener must hear it, got %q", got)
	}
}
```

Note: `seedVoiceWorld` itself calls `configs.SetConfigForTest`; `pinAmbientBudget` re-reads `configs.GetConfig()` after it, so the Pinnacle switch and chatter knobs survive.

- [ ] **Step 2: Run them and see them fail**

```bash
go test ./modules/weather/... -count=1
go test ./internal/behaviortree/ -run 'TestItemIdleLine|TestItemEventLine' -count=1
```

Expected: weather build failure (`too many arguments in call to EmitAmbient`, `c.SeasonalEmoteOneIn undefined`); `TestShippedWeatherAmbientKnobs` would report `24 and 0`; the item tests fail with the over-budget player still reading the line.

- [ ] **Step 3: Implement weather**

`modules/weather/engine/emotes.go`: delete

```go
// seasonalEmoteOneIn throttles the seasonal-ambience layer: it fires on
// roughly 1 in N emote passes, only in calm zones — strictly quieter than
// weather (spec S-R1). Promote to config only if play feel demands.
const seasonalEmoteOneIn = 3

```

replace the signature lines

```go
func EmitAmbient(g *sim.Graph, fronts []sim.Front, simCfg sim.Config,
	weather map[sim.ZoneId]sim.WeatherType, zoneSeasons map[sim.ZoneId]seasons.ZoneSeason,
	tables content.Tables, seasonal content.SeasonalTables,
	mildChancePct, strongChancePct int, roll func(int) int) int {
```

with

```go
//
// seasonalOneIn is Modules.weather.SeasonalEmoteOneIn (#317): a calm zone
// gets its seasonal line on roughly 1 pass in N, and 0 turns the seasonal
// layer off. Every line goes through the ambient listener budget.
func EmitAmbient(g *sim.Graph, fronts []sim.Front, simCfg sim.Config,
	weather map[sim.ZoneId]sim.WeatherType, zoneSeasons map[sim.ZoneId]seasons.ZoneSeason,
	tables content.Tables, seasonal content.SeasonalTables,
	mildChancePct, strongChancePct, seasonalOneIn int, roll func(int) int) int {
```

replace the weather send

```go
			if line := tables.Pick(w, biomeId, indoor, f, strongFeltThreshold, season, roll); line != "" {
				room.SendText(messaging.CategoryWeather, line)
				sent++
			}
```

with

```go
			if line := tables.Pick(w, biomeId, indoor, f, strongFeltThreshold, season, roll); line != "" {
				sendAmbientWeather(room, line)
				sent++
			}
```

replace the seasonal block

```go
		if hasSeason && seasonal != nil && roll(seasonalEmoteOneIn) == 0 {
			if line := seasonal.Pick(zs.Track, zs.Season, biomeId, indoor, 1.0, strongFeltThreshold, roll); line != "" {
				room.SendText(messaging.CategoryWeather, line)
				sent++
			}
		}
```

with

```go
		if hasSeason && seasonal != nil && seasonalOneIn > 0 && roll(seasonalOneIn) == 0 {
			if line := seasonal.Pick(zs.Track, zs.Season, biomeId, indoor, 1.0, strongFeltThreshold, roll); line != "" {
				sendAmbientWeather(room, line)
				sent++
			}
		}
```

and append:

```go
// sendAmbientWeather sends a weather or season line into room through the
// ambient listener budget (#317): a player over budget skips it, the rest
// of the room hears it. Severe-weather onset and season splashes are event
// lines and do not come here.
func sendAmbientWeather(room *rooms.Room, line string) {
	plan := ambient.Plan(ambient.Line{Room: room, Source: ambient.SourceWeather})
	room.SendText(messaging.CategoryWeather, line, plan.Exclude...)
}
```

with `"github.com/GoMudEngine/GoMud/internal/ambient"` added to its imports.

`modules/weather/weather_config.go`: in `Config`, replace

```go
	EmoteEveryRounds     int     // ambient emote cadence in rounds (jittered ±25%, >= 5)
```

with

```go
	EmoteEveryRounds     int     // ambient emote cadence in rounds (jittered ±25%, >= 5); 0 = off (#317)
	SeasonalEmoteOneIn   int     // calm zones get a seasonal line on 1 pass in N; 0 = never (#317)
```

in `buildConfig`, after `EmoteStrongChancePct: intOr(get("EmoteStrongChancePct"), 100),` add

```go
		SeasonalEmoteOneIn:   intOr(get("SeasonalEmoteOneIn"), 3),
```

and replace

```go
	if c.EmoteEveryRounds < 5 {
		c.EmoteEveryRounds = 5
	}
```

with

```go
	// 0 turns the ambient pass off (#317); 1 to 4 still clamp to 5.
	if c.EmoteEveryRounds < 0 {
		c.EmoteEveryRounds = 0
	} else if c.EmoteEveryRounds > 0 && c.EmoteEveryRounds < 5 {
		c.EmoteEveryRounds = 5
	}
	if c.SeasonalEmoteOneIn < 0 {
		c.SeasonalEmoteOneIn = 0
	}
```

`modules/weather/weather.go`, replace

```go
	if m.cfg.EmoteMode == EmoteModeModule && evt.RoundNumber >= m.nextEmote {
		engine.EmitAmbient(m.graph, m.state.Fronts, m.simCfg, m.state.Weather, m.zoneSeasons, m.tables, m.seasonalTables, m.cfg.EmoteMildChancePct, m.cfg.EmoteStrongChancePct, util.Rand)
```

with

```go
	if m.cfg.EmoteMode == EmoteModeModule && m.cfg.EmoteEveryRounds > 0 && evt.RoundNumber >= m.nextEmote {
		engine.EmitAmbient(m.graph, m.state.Fronts, m.simCfg, m.state.Weather, m.zoneSeasons, m.tables, m.seasonalTables, m.cfg.EmoteMildChancePct, m.cfg.EmoteStrongChancePct, m.cfg.SeasonalEmoteOneIn, util.Rand)
```

`_datafiles/config.yaml`, in the `weather:` module block, replace

```yaml
    EmoteMode: module
    EmoteEveryRounds: 24
    EmoteMildChancePct: 30
    EmoteStrongChancePct: 100
```

with

```yaml
    EmoteMode: module
    # A weather or season line pass runs every EmoteEveryRounds rounds, give
    # or take a quarter (48 is about 3 minutes); 0 turns them off. In calm
    # weather a seasonal line plays on 1 pass in SeasonalEmoteOneIn (0 =
    # never). Each line counts toward Balance.AmbientListenerBudgetRounds.
    EmoteEveryRounds: 48
    EmoteMildChancePct: 30
    EmoteStrongChancePct: 100
    SeasonalEmoteOneIn: 4
```

- [ ] **Step 4: Implement item chatter**

`internal/behaviortree/actions_item_voice.go`, replace:

```go
	if holderUser != nil {
		holderUser.SendText(messaging.CategoryNPCDialogue, fmt.Sprintf(
			`<ansi fg="item">%s</ansi> says, "<ansi fg="yellow">%s</ansi>"`, messaging.CapitalizeStart(spec.Name), line))
	}
	room := rooms.LoadRoom(ctx.RoomId)
	roomHeard := toRoom && room != nil
	if roomHeard {
		exclude := []int{}
		if holderUser != nil {
			exclude = append(exclude, holderUser.UserId)
		}
```

with:

```go
	room := rooms.LoadRoom(ctx.RoomId)
	// An ambient (item_idle) line counts toward the listener budget (#317):
	// a player over budget, the holder included, skips it. Event lines
	// (on_equip, on_kill...) are never budgeted.
	var overBudget []int
	if ctx.Event.EventType == `item_idle` && room != nil {
		overBudget = ambient.Plan(ambient.Line{Room: room, Source: ambient.SourceItemChatter}).Exclude
	}
	if holderUser != nil && !slices.Contains(overBudget, holderUser.UserId) {
		holderUser.SendText(messaging.CategoryNPCDialogue, fmt.Sprintf(
			`<ansi fg="item">%s</ansi> says, "<ansi fg="yellow">%s</ansi>"`, messaging.CapitalizeStart(spec.Name), line))
	}
	roomHeard := toRoom && room != nil
	if roomHeard {
		exclude := slices.Clone(overBudget)
		if holderUser != nil {
			exclude = append(exclude, holderUser.UserId)
		}
```

and add `"slices"` and `"github.com/GoMudEngine/GoMud/internal/ambient"` to its imports.

- [ ] **Step 5: Gate conversation starts on everyone's budget**

`internal/hooks/NewRound_IdleMobs.go`, replace (the Task 3 version):

```go
		if conversationsTriggerEligible(mob) && conversationStartRoll(util.Rand) {
			if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
				conversations.TryStart(conversationadapter.AdaptMob(mob), room.GetMobs())
			}
		}
```

with:

```go
		if conversationsTriggerEligible(mob) && conversationStartRoll(util.Rand) {
			// A conversation starts only if every player here can hear it
			// out without going over their ambient budget (#317).
			if room := rooms.LoadRoom(mob.Character.RoomId); room != nil && ambient.AllHaveBudget(room) {
				conversations.TryStart(conversationadapter.AdaptMob(mob), room.GetMobs())
			}
		}
```

and add `"github.com/GoMudEngine/GoMud/internal/ambient"` to its imports.

`internal/usercommands/go.go`, replace:

```go
				boostPct := int(cfg.ConversationPlayerArrivalBoostPct)
				if boostPct > 0 && util.Rand(100) < boostPct {
```

with:

```go
				boostPct := int(cfg.ConversationPlayerArrivalBoostPct)
				// Only if everyone here can hear it out without going over
				// their ambient budget (#317).
				if boostPct > 0 && util.Rand(100) < boostPct && ambient.AllHaveBudget(destRoom) {
```

and add `"github.com/GoMudEngine/GoMud/internal/ambient"` to its imports. (`ambient.AllHaveBudget` itself is tested in Task 7.)

- [ ] **Step 6: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./modules/weather/... ./internal/behaviortree/ ./internal/hooks/ ./internal/usercommands/ ./internal/ambient/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty; all `ok`, including the existing `TestEmitAmbient_ReadsConfiguredStrongFeltThreshold` (its room's player 9001 has no user record; `Plan` skips an unknown user and `SendText` already did, and `sent` still counts the line).

- [ ] **Step 7: Prove the weather budget test can fail**

Sabotage `sendAmbientWeather`: drop `plan.Exclude...` from the `SendText` call. Run `go test ./modules/weather/engine/ -run TestEmitAmbient_SeasonalLineHonoursTheBudget -count=1`; expected red with `9001 is over budget and must skip the line`. Restore; green.

- [ ] **Step 8: Commit**

```bash
git add modules/weather/engine/emotes.go modules/weather/engine/emotes_config_test.go modules/weather/engine/emotes_ambient_test.go modules/weather/weather_config.go modules/weather/weather.go modules/weather/weather_config_test.go modules/weather/weather_shipped_test.go _datafiles/config.yaml internal/behaviortree/actions_item_voice.go internal/behaviortree/item_voice_ambient_test.go internal/hooks/NewRound_IdleMobs.go internal/usercommands/go.go
git commit -F - <<'EOF'
feat: #317 weather, item chatter and conversations meet the budget

Modules.weather.SeasonalEmoteOneIn replaces the hardcoded 3 (ships 4,
0 never); EmoteEveryRounds ships 48 and 0 turns the pass off. Weather and
season lines and item_idle lines go through ambient.Plan. An NPC
conversation, idle or on arrival, starts only if every player present has
budget.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 12: Ambient telemetry behind a switch (Opus)

**Files:** `internal/ambient/telemetry.go`, `internal/ambient/telemetry_test.go`, `internal/ambient/ambient.go`, `internal/ambient/testing.go`, `internal/hooks/MobIdle_HandleIdleMobs.go`, `internal/hooks/NewRound_IdleMobs.go`, `internal/behaviortree/actions_scout.go`

Line format (stable; `tools/ambient_report.py` reads it): message `ambient.telemetry`, then in this order `window` (report number), `room`, `zone`, `round_seconds`, `window_rounds`, `listener_rounds` (players present summed over the window's rounds), then for each source in `room_idle, mob_chatter, weather, gossip, item_chatter, say_echo` (Task 8A added `say_echo`, counted in the room that receives the echo) the three keys `<source>_offered`, `<source>_delivered`, `<source>_suppressed` (per listener: offered = delivered + suppressed), then `move_idle`, `move_track`, `move_path`. One line per room with any listener rounds, line or move in the window.

- [ ] **Step 1: Write the failing test**

Create `internal/ambient/telemetry_test.go`:

```go
package ambient

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func pinTelemetry(t *testing.T, on bool, minutes int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.AmbientTelemetry = configs.ConfigBool(on)
	cfg.Balance.AmbientTelemetryReportMinutes = configs.ConfigInt(minutes)
	configs.SetConfigForTest(t, cfg)
}

func captureEmit(t *testing.T) *[]map[string]any {
	t.Helper()
	var got []map[string]any
	orig := emit
	emit = func(args ...any) {
		m := map[string]any{}
		for i := 0; i+1 < len(args); i += 2 {
			m[args[i].(string)] = args[i+1]
		}
		got = append(got, m)
	}
	t.Cleanup(func() { emit = orig })
	return &got
}

func TestTelemetry_OffKeepsNothing(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA, listenB)
	pinTelemetry(t, false, 1)
	lines := captureEmit(t)
	Plan(Line{Room: room, Source: SourceMobChatter})
	CountMove(ambRoom, MoveIdle)
	telMu.Lock()
	n := len(telRooms)
	telMu.Unlock()
	if n != 0 {
		t.Fatalf("telemetry off must count nothing, %d rooms counted", n)
	}
	for r := ambNow + 1; r <= ambNow+40; r++ {
		RoundTick(r)
	}
	if len(*lines) != 0 {
		t.Fatalf("telemetry off must write nothing, wrote %d lines", len(*lines))
	}
}

func TestTelemetry_OnCountsAndWritesOneLinePerRoom(t *testing.T) {
	room := seedAmbientRoom(t, 5, nil, listenA, listenB)
	pinTelemetry(t, true, 1) // 1 minute = 15 rounds at 4 seconds
	lines := captureEmit(t)

	ChargeForTest(listenA, ambNow+5)
	Plan(Line{Room: room, Source: SourceMobChatter}) // A suppressed, B delivered
	for i := 0; i < 3; i++ {
		CountMove(ambRoom, MoveIdle)
	}
	CountMove(ambRoom, MovePath)

	for r := ambNow + 1; r <= ambNow+16; r++ {
		RoundTick(r)
	}
	if len(*lines) != 1 {
		t.Fatalf("one active room, one window: want 1 line, got %d: %v", len(*lines), *lines)
	}
	l := (*lines)[0]
	want := map[string]any{
		"window": 1, "room": ambRoom, "zone": ambZone, "round_seconds": 4, "window_rounds": 15,
		"listener_rounds":       32, // two players for 16 ticks
		"mob_chatter_offered":   2,
		"mob_chatter_delivered": 1, "mob_chatter_suppressed": 1,
		"room_idle_offered": 0, "move_idle": 3, "move_track": 0, "move_path": 1,
	}
	for k, v := range want {
		if l[k] != v {
			t.Errorf("%s = %v, want %v (line %v)", k, l[k], v, l)
		}
	}

	// The window reset its counters: the next line has listeners only.
	for r := ambNow + 17; r <= ambNow+31; r++ {
		RoundTick(r)
	}
	if len(*lines) != 2 || (*lines)[1]["mob_chatter_offered"] != 0 || (*lines)[1]["move_idle"] != 0 {
		t.Fatalf("second window must start from zero: %v", *lines)
	}
}

// An ambient say's echo (Task 8A) is planned against the neighbouring room,
// so it is counted there under say_echo, apart from the speaker's room.
func TestTelemetry_SayEchoCountsInTheReceivingRoom(t *testing.T) {
	neighbour := seedAmbientRoom(t, 0, nil, listenA, listenB)
	pinTelemetry(t, true, 1)
	lines := captureEmit(t)
	Plan(Line{Room: neighbour, Source: SourceSayEcho})
	for r := ambNow + 1; r <= ambNow+16; r++ {
		RoundTick(r)
	}
	if len(*lines) != 1 || (*lines)[0]["room"] != ambRoom || (*lines)[0]["say_echo_delivered"] != 2 || (*lines)[0]["mob_chatter_offered"] != 0 {
		t.Fatalf("the echo must be counted as say_echo in the receiving room: %v", *lines)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/ambient/ -run TestTelemetry -count=1
```

Expected: build failure, `undefined: emit`, `undefined: CountMove`.

- [ ] **Step 3: Implement**

Create `internal/ambient/telemetry.go`:

```go
package ambient

import (
	"sort"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// MoveKind names one counted kind of mob move. The strings are telemetry
// keys tools/ambient_report.py reads.
type MoveKind string

const (
	MoveIdle  MoveKind = "move_idle"  // a wander, sneak or portal from the idle roll
	MoveTrack MoveKind = "move_track" // a step toward a tracked target (move_toward_tracked)
	MovePath  MoveKind = "move_path"  // one step along a path: schedule, patrol, goal planner, home
)

// MoveKinds lists every move kind, in report order.
var MoveKinds = []MoveKind{MoveIdle, MoveTrack, MovePath}

// TelemetryMessage is the log message every summary line carries.
const TelemetryMessage = "ambient.telemetry"

type lineCounts struct{ offered, delivered, suppressed int }

type roomCounts struct {
	zone           string
	listenerRounds int
	lines          map[Source]*lineCounts
	moves          map[MoveKind]int
}

var (
	telMu         sync.Mutex
	telRooms      = map[int]*roomCounts{}
	telNextReport uint64
	telWindow     int

	// emit writes one summary line. Tests replace it to read the lines.
	emit = func(args ...any) { mudlog.Info(TelemetryMessage, args...) }
)

// TelemetryEnabled reports Balance.AmbientTelemetry. Off, nothing is
// counted and nothing is kept.
func TelemetryEnabled() bool {
	return bool(configs.GetBalanceConfig().AmbientTelemetry)
}

func roomEntryLocked(roomId int, zone string) *roomCounts {
	rc := telRooms[roomId]
	if rc == nil {
		rc = &roomCounts{zone: zone, lines: map[Source]*lineCounts{}, moves: map[MoveKind]int{}}
		telRooms[roomId] = rc
	}
	return rc
}

// countLine records one Plan: per listener, delivered and suppressed.
func countLine(room *rooms.Room, src Source, delivered, suppressed int) {
	if room == nil || delivered+suppressed == 0 || !TelemetryEnabled() {
		return
	}
	telMu.Lock()
	defer telMu.Unlock()
	rc := roomEntryLocked(room.RoomId, room.Zone)
	lc := rc.lines[src]
	if lc == nil {
		lc = &lineCounts{}
		rc.lines[src] = lc
	}
	lc.offered += delivered + suppressed
	lc.delivered += delivered
	lc.suppressed += suppressed
}

// CountMove counts one mob move that starts in roomId.
func CountMove(roomId int, kind MoveKind) {
	if !TelemetryEnabled() {
		return
	}
	room := rooms.LoadRoom(roomId)
	if room == nil {
		return
	}
	telMu.Lock()
	defer telMu.Unlock()
	roomEntryLocked(roomId, room.Zone).moves[kind]++
}

func roundSeconds() int {
	secs := int(configs.GetTimingConfig().RoundSeconds)
	if secs < 1 {
		secs = 4
	}
	return secs
}

func reportWindowRounds() uint64 {
	w := int(configs.GetBalanceConfig().AmbientTelemetryReportMinutes) * 60 / roundSeconds()
	if w < 1 {
		w = 1
	}
	return uint64(w)
}

// telemetryTick adds listener rounds for every room with players and, when
// the window is up, writes one line per active room and resets.
func telemetryTick(round uint64) {
	if !TelemetryEnabled() {
		telMu.Lock()
		telRooms = map[int]*roomCounts{}
		telNextReport = 0
		telMu.Unlock()
		return
	}
	window := reportWindowRounds()

	type occupied struct {
		id, players int
		zone        string
	}
	var occ []occupied
	for _, roomId := range rooms.GetRoomsWithPlayers() {
		if room := rooms.LoadRoom(roomId); room != nil {
			if n := len(room.GetPlayers()); n > 0 {
				occ = append(occ, occupied{id: roomId, players: n, zone: room.Zone})
			}
		}
	}

	telMu.Lock()
	defer telMu.Unlock()
	if telNextReport == 0 {
		telNextReport = round + window
	}
	for _, o := range occ {
		roomEntryLocked(o.id, o.zone).listenerRounds += o.players
	}
	if round < telNextReport {
		return
	}
	telWindow++
	flushLocked(window)
	telNextReport = round + window
}

func flushLocked(window uint64) {
	secs := roundSeconds()
	ids := make([]int, 0, len(telRooms))
	for id := range telRooms {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		rc := telRooms[id]
		args := []any{"window", telWindow, "room", id, "zone", rc.zone,
			"round_seconds", secs, "window_rounds", int(window), "listener_rounds", rc.listenerRounds}
		for _, src := range Sources {
			lc := rc.lines[src]
			if lc == nil {
				lc = &lineCounts{}
			}
			args = append(args,
				string(src)+"_offered", lc.offered,
				string(src)+"_delivered", lc.delivered,
				string(src)+"_suppressed", lc.suppressed)
		}
		for _, mk := range MoveKinds {
			args = append(args, string(mk), rc.moves[mk])
		}
		emit(args...)
	}
	telRooms = map[int]*roomCounts{}
}
```

In `internal/ambient/ambient.go`, replace the body of `Plan` from `now := util.GetRoundCount()` to `return r` with:

```go
	now := util.GetRoundCount()
	window, hushed := budgetWindow(l.Room.Zone)
	suppressed := 0

	budgetMu.Lock()
	for _, uid := range l.Room.GetPlayers() {
		if slices.Contains(l.Exclude, uid) {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil {
			continue
		}
		if l.Perceives != nil && !l.Perceives(u) {
			continue
		}
		if hushed || (window > 0 && now < budgetNext[uid]) {
			r.Exclude = append(r.Exclude, uid)
			suppressed++
			continue
		}
		if window > 0 {
			budgetNext[uid] = now + window
		}
		r.Delivered = append(r.Delivered, uid)
	}
	budgetMu.Unlock()

	countLine(l.Room, l.Source, len(r.Delivered), suppressed)
	return r
```

and at the end of `RoundTick`, after `budgetMu.Unlock()`, add:

```go

	telemetryTick(round)
```

In `internal/ambient/testing.go`, replace `ResetForTest` with:

```go
// ResetForTest empties the budget, the visits and the telemetry and returns
// a restore func. For tests in any package.
func ResetForTest() func() {
	budgetMu.Lock()
	origBudget := budgetNext
	budgetNext = map[int]uint64{}
	budgetMu.Unlock()

	visitMu.Lock()
	origVisits := singleHeard
	singleHeard = map[int]int{}
	visitMu.Unlock()

	telMu.Lock()
	origRooms, origNext, origWindow := telRooms, telNextReport, telWindow
	telRooms, telNextReport, telWindow = map[int]*roomCounts{}, 0, 0
	telMu.Unlock()

	return func() {
		budgetMu.Lock()
		budgetNext = origBudget
		budgetMu.Unlock()
		visitMu.Lock()
		singleHeard = origVisits
		visitMu.Unlock()
		telMu.Lock()
		telRooms, telNextReport, telWindow = origRooms, origNext, origWindow
		telMu.Unlock()
	}
}
```

Count the moves. `internal/hooks/MobIdle_HandleIdleMobs.go`, replace (the Task 9 block):

```go
		if moveCmd != `` {
			mob.Command(moveCmd)
		}
```

with:

```go
		if moveCmd != `` {
			ambient.CountMove(mob.Character.RoomId, ambient.MoveIdle)
			mob.Command(moveCmd)
		}
```

`internal/hooks/NewRound_IdleMobs.go`, replace:

```go
		if quote.OK() {
			mob.Path.Next()
			mob.Command(nextStep.ExitName())
```

with:

```go
		if quote.OK() {
			ambient.CountMove(mob.Character.RoomId, ambient.MovePath)
			mob.Path.Next()
			mob.Command(nextStep.ExitName())
```

`internal/behaviortree/actions_scout.go`, replace:

```go
	// Dispatch `go <direction>` via the engine command pipeline.
	mob.Command("go " + dir)
	return Success
```

with:

```go
	// Dispatch `go <direction>` via the engine command pipeline.
	ambient.CountMove(room.RoomId, ambient.MoveTrack)
	mob.Command("go " + dir)
	return Success
```

and add `"github.com/GoMudEngine/GoMud/internal/ambient"` to that file's imports.

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/ambient/ ./internal/hooks/ ./internal/behaviortree/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty; all `ok` (`mob_path_wait_test.go:113` still sees exactly `[]string{"north"}`: `CountMove` queues nothing).

- [ ] **Step 5: Prove the off test can fail**

Sabotage `countLine`: drop `|| !TelemetryEnabled()`. Run `go test ./internal/ambient/ -run TestTelemetry_OffKeepsNothing -count=1`; expected red with `telemetry off must count nothing, 1 rooms counted`. Restore; green.

- [ ] **Step 6: Commit**

```bash
git add internal/ambient/telemetry.go internal/ambient/telemetry_test.go internal/ambient/ambient.go internal/ambient/testing.go internal/hooks/MobIdle_HandleIdleMobs.go internal/hooks/NewRound_IdleMobs.go internal/behaviortree/actions_scout.go
git commit -F - <<'EOF'
feat(ambient): #317 ambient telemetry behind Balance.AmbientTelemetry

Per room and source: lines offered, delivered and suppressed per listener;
listener rounds; mob moves split into idle, tracking and path steps. One
"ambient.telemetry" line per active room every
AmbientTelemetryReportMinutes, then reset. Off keeps no counters.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 13: `tools/ambient_report.py` (Sonnet)

**Files:** `tools/ambient_report.py`

- [ ] **Step 1: Write the script with its self-test**

Create `tools/ambient_report.py`:

```python
#!/usr/bin/env python3
"""Ambient telemetry report (#317).

Reads a DOGMud server log with Balance.AmbientTelemetry on and prints, as
Markdown:
  1. lines per listener-minute for each room and source, worst first, with
     offered, delivered and suppressed counts (per listener);
  2. each room's total across sources, worst first;
  3. mob moves per minute per zone (idle, tracking, path).

The log can be the playtest's tools/playtest/.run/<run_id>/server.log, a
`docker logs <container>` dump, or a live server's log file. Colour codes
are stripped, so either mudlog handler's output parses.

Usage:
  python -I tools/ambient_report.py <server.log>
  python -I tools/ambient_report.py --self-test
"""
import argparse
import re
import sys

ANSI = re.compile(r"\x1b\[[0-9;]*m")
PAIR = re.compile(r'(\w+)=("(?:[^"\\]|\\.)*"|\S+)')
MARK = "ambient.telemetry"
SOURCES = ["room_idle", "mob_chatter", "weather", "gossip", "item_chatter", "say_echo"]
MOVES = ["move_idle", "move_track", "move_path"]


def parse_line(line):
    line = ANSI.sub("", line)
    if MARK not in line:
        return None
    rec = {}
    for key, val in PAIR.findall(line):
        if len(val) >= 2 and val[0] == '"' and val[-1] == '"':
            val = val[1:-1]
        rec[key] = val
    if "room" not in rec or "window" not in rec:
        return None
    return rec


def num(rec, key):
    try:
        return int(rec.get(key, "0"))
    except ValueError:
        return 0


def build(records):
    rooms = {}
    zones = {}
    windows = set()
    window_seconds = 0
    for rec in records:
        secs = num(rec, "round_seconds") or 4
        windows.add(rec["window"])
        window_seconds = max(window_seconds, num(rec, "window_rounds") * secs)
        room = rooms.setdefault(rec["room"], {
            "zone": rec.get("zone", ""),
            "listener_seconds": 0,
            "src": {s: [0, 0, 0] for s in SOURCES},
        })
        room["listener_seconds"] += num(rec, "listener_rounds") * secs
        for s in SOURCES:
            room["src"][s][0] += num(rec, s + "_offered")
            room["src"][s][1] += num(rec, s + "_delivered")
            room["src"][s][2] += num(rec, s + "_suppressed")
        zone = zones.setdefault(rec.get("zone", ""), {m: 0 for m in MOVES})
        for m in MOVES:
            zone[m] += num(rec, m)
    minutes = len(windows) * window_seconds / 60.0
    return rooms, zones, minutes


def render(rooms, zones, minutes):
    out = []
    rows = []
    totals = []
    for room_id, room in rooms.items():
        lm = room["listener_seconds"] / 60.0
        if lm <= 0:
            continue
        all_off = all_del = all_sup = 0
        for s in SOURCES:
            off, dl, sup = room["src"][s]
            all_off, all_del, all_sup = all_off + off, all_del + dl, all_sup + sup
            if off == 0:
                continue
            rows.append((dl / lm, room_id, room["zone"], s, lm, off, dl, sup))
        totals.append((all_del / lm, room_id, room["zone"], lm, all_off, all_del, all_sup))
    rows.sort(key=lambda r: (-r[0], r[1], r[3]))
    totals.sort(key=lambda r: (-r[0], r[1]))

    out.append("## Ambient lines per listener-minute (worst first)")
    out.append("")
    out.append("| Room | Zone | Source | Listener-min | Offered | Delivered | Suppressed | Delivered per listener-min |")
    out.append("|---|---|---|---|---|---|---|---|")
    for rate, room_id, zone, s, lm, off, dl, sup in rows:
        out.append(f"| {room_id} | {zone} | {s} | {lm:.1f} | {off} | {dl} | {sup} | {rate:.2f} |")
    out.append("")
    out.append("## Rooms, all sources (worst first)")
    out.append("")
    out.append("| Room | Zone | Listener-min | Offered | Delivered | Suppressed | Delivered per listener-min |")
    out.append("|---|---|---|---|---|---|---|")
    for rate, room_id, zone, lm, off, dl, sup in totals:
        out.append(f"| {room_id} | {zone} | {lm:.1f} | {off} | {dl} | {sup} | {rate:.2f} |")
    out.append("")
    out.append(f"## Mob moves per minute by zone ({minutes:.0f} minutes measured)")
    out.append("")
    out.append("| Zone | Idle | Tracking | Path | Total |")
    out.append("|---|---|---|---|---|")
    zrows = sorted(zones.items(), key=lambda kv: (-sum(kv[1].values()), kv[0]))
    for zone, z in zrows:
        total = sum(z.values())
        if minutes <= 0 or total == 0:
            continue
        idle, track, path = (z[m] / minutes for m in MOVES)
        out.append(f"| {zone} | {idle:.2f} | {track:.2f} | {path:.2f} | {total / minutes:.2f} |")
    return "\n".join(out) + "\n"


def report(lines):
    records = [r for r in (parse_line(l) for l in lines) if r]
    return render(*build(records))


def _fixture_line(colour, **kv):
    parts = []
    for k, v in kv.items():
        val = f'"{v}"' if isinstance(v, str) else str(v)
        parts.append(f"{k}=\x1b[31;49m{val}\x1b[0m " if colour else f"{k}={val} ")
    if colour:
        return "\x1b[90m[12:00:00]\x1b[0m \x1b[32mINFO: \x1b[0m \x1b[36mambient.telemetry\x1b[39;49m " + "".join(parts)
    return "time=12:00:00 level=INFO msg=ambient.telemetry " + "".join(parts)


def self_test():
    base = {s + k: 0 for s in SOURCES for k in ("_offered", "_delivered", "_suppressed")}
    base.update({m: 0 for m in MOVES})

    def rec(**kv):
        d = dict(base)
        d.update(kv)
        return d

    fixture = [
        "unrelated line=1",
        _fixture_line(True, **rec(window=1, room=472, zone="Thornwall City", round_seconds=4, window_rounds=75,
                                  listener_rounds=75, mob_chatter_offered=12, mob_chatter_delivered=10,
                                  mob_chatter_suppressed=2, room_idle_offered=2, room_idle_delivered=2)),
        _fixture_line(False, **rec(window=1, room=5003, zone="Instance Planar Oasis", round_seconds=4,
                                   window_rounds=75, listener_rounds=0, move_idle=6, move_path=4)),
        _fixture_line(False, **rec(window=2, room=472, zone="Thornwall City", round_seconds=4, window_rounds=75,
                                   listener_rounds=75, mob_chatter_offered=8, mob_chatter_delivered=8)),
        _fixture_line(True, **rec(window=2, room=5003, zone="Instance Planar Oasis", round_seconds=4,
                                  window_rounds=75, listener_rounds=0, move_idle=4)),
    ]
    out = report(fixture)
    want = [
        "| 472 | Thornwall City | mob_chatter | 10.0 | 20 | 18 | 2 | 1.80 |",
        "| 472 | Thornwall City | room_idle | 10.0 | 2 | 2 | 0 | 0.20 |",
        "| 472 | Thornwall City | 10.0 | 22 | 20 | 2 | 2.00 |",
        "## Mob moves per minute by zone (10 minutes measured)",
        "| Instance Planar Oasis | 1.00 | 0.00 | 0.40 | 1.40 |",
    ]
    missing = [w for w in want if w not in out]
    order_ok = out.find(want[0]) < out.find(want[1])
    if missing or not order_ok:
        print("self-test FAILED")
        for w in missing:
            print("  missing: " + w)
        if not order_ok:
            print("  rows are not worst first")
        print(out)
        return 1
    print("self-test ok")
    return 0


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("log", nargs="?", help="server log to read")
    ap.add_argument("--self-test", action="store_true", help="run the built-in fixture check")
    args = ap.parse_args()
    if args.self_test:
        return self_test()
    if not args.log:
        ap.error("give a log file or --self-test")
    with open(args.log, encoding="utf-8", errors="replace") as f:
        sys.stdout.write(report(f))
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 2: Run the self-test**

```bash
python -I tools/ambient_report.py --self-test
```

Expected: `self-test ok`.

- [ ] **Step 3: Prove the self-test can fail**

Sabotage: change `ANSI = re.compile(r"\x1b\[[0-9;]*m")` to `ANSI = re.compile(r"NEVERMATCHES")`. Run the self-test; expected `self-test FAILED` naming the missing `| 472 | ... 1.80 |` row (the coloured lines no longer parse). Restore; `self-test ok`.

- [ ] **Step 4: Commit**

```bash
git add tools/ambient_report.py
git commit -F - <<'EOF'
feat(tools): #317 ambient_report.py reads ambient telemetry from a log

Per room and source lines per listener-minute worst first, room totals,
and mob moves per minute per zone. Strips colour codes so either log
handler parses. --self-test runs a built-in fixture.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 14: The playtest scenario (Sonnet)

**Files:** `tools/playtest/scenarios/317-ambient.yaml`, `tools/playtest/goals/scenarios/317-ambient/town.yaml`, `coulee.yaml`, `steppe.yaml`, `oasis.yaml`, `internal/playtestrun/scenario_317_test.go`

- [ ] **Step 1: Write the scenario**

Create `tools/playtest/scenarios/317-ambient.yaml`:

```yaml
# #317 ambient retune: four feel-tester seats sit or walk fixed routes and
# tally every unprompted line while the server's ambient telemetry counts the
# same rooms. Telemetry is switched on in the playtest checkout's
# config.yaml, never in a commit. Run once at the baseline values and once at
# the shipped values; docs/superpowers/plans/2026-10-30-317-ambient-retune.md
# has the steps. Both runs start a fresh container at the same game time, so
# NPC schedules line up between them.
name: 317-ambient
mode: parallel
summary: >-
  Feel testers report how busy Thornwall's tavern, Main Street and market,
  the Pothole Coulee hub, the Ironwind Steppe at night and the Planar Oasis
  feel, with a five-minute tally of every unprompted line by source.
on_actor_stop: continue
budgets:
  wall_clock: 50m
requires:
  max_connections: 20
roster:
  - id: town
    personality: feel-tester
    goals: goals/scenarios/317-ambient/town.yaml
  - id: coulee
    personality: feel-tester
    goals: goals/scenarios/317-ambient/coulee.yaml
  - id: steppe
    personality: feel-tester
    goals: goals/scenarios/317-ambient/steppe.yaml
  - id: oasis
    personality: feel-tester
    goals: goals/scenarios/317-ambient/oasis.yaml

group_goals:
  - id: tallies
    do: Every seat keeps a five-minute tally of unprompted lines by source and a busy score.
    verify: Each seat's report has a tally row per five minutes and a feel note per room.
```

Create `tools/playtest/goals/scenarios/317-ambient/town.yaml`:

```yaml
# #317 ambient: the town seat sits in three Thornwall rooms for twelve
# minutes each and tallies every line nobody asked for.
name: 317-ambient-town
summary: Sit in the tavern, Main Street and the market; tally unprompted lines.
goals:
  - id: tally-town
    do: >-
      Stay quiet and watch. Do not talk to anyone, attack, shop or start
      anything; looking around and typing time are fine. Spend twelve
      minutes in The Drowning Post Tavern, where you start. Then go north
      twice to Main Street, Central and spend twelve minutes there. Then go
      east three times to Market Square, Center and spend twelve minutes
      there. Every five minutes, type time and write one tally row: the
      room, the game time, and how many lines you saw that you did not
      cause, split into NPC speech, NPC actions (emotes), NPCs walking in or
      out, room flavour lines, weather or season lines, gossip, tips, and
      conversations between NPCs. Add a busy score from 1 (dead) to 5 (too
      much to read) and one sentence on how the room feels.
    verify: >-
      Three rooms visited for about twelve minutes each; at least six tally
      rows; a feel note per room naming its noisiest source.

ephemeral:
  profile: mid
  start_room: 472
  budgets:
    wall_clock: 50m
```

Create `tools/playtest/goals/scenarios/317-ambient/coulee.yaml`:

```yaml
# #317 ambient: the coulee seat sits at the Pothole Coulee hub and tallies
# every line nobody asked for.
name: 317-ambient-coulee
summary: Sit at Hub Square in Pothole Coulee; tally unprompted lines.
goals:
  - id: tally-coulee
    do: >-
      Stay quiet and watch at Hub Square, where you start, for forty
      minutes. Do not talk to anyone, take quests, attack or shop; looking
      around and typing time are fine. Every five minutes, type time and
      write one tally row: the game time, and how many lines you saw that
      you did not cause, split into NPC speech, NPC actions (emotes), NPCs
      walking in or out, room flavour lines, weather or season lines,
      gossip, tips, and conversations between NPCs. Add a busy score from 1
      (dead) to 5 (too much to read) and one sentence on how it feels for a
      new player standing here.
    verify: >-
      At least seven tally rows from Hub Square and a closing feel note.

ephemeral:
  profile: mid
  start_room: 5203
  budgets:
    wall_clock: 50m
```

Create `tools/playtest/goals/scenarios/317-ambient/steppe.yaml`:

```yaml
# #317 ambient: the steppe seat stays at Badlands Crossing through dusk into
# night and tallies every line nobody asked for.
name: 317-ambient-steppe
summary: Stay at Badlands Crossing on the Ironwind Steppe into the night; tally lines.
goals:
  - id: tally-steppe
    do: >-
      Stay at Badlands Crossing, where you start, for the whole session. A
      game day here lasts about an hour of real time and night falls for a
      third of it, so night will come while you wait. Do not explore. If a
      creature attacks you, fight back or step one room away and come
      straight back. Every five minutes, type time and write one tally row:
      the game time, whether it is day or night and whether you can see,
      and how many lines you saw or heard that you did not cause, split
      into NPC or creature lines, creatures walking in or out, room flavour
      lines, weather or season lines, gossip and tips. In the dark you may
      see nothing; write that down too. Add a busy score from 1 (dead) to 5
      (too much to read) and one sentence on how it feels.
    verify: >-
      Tally rows across the session, at least two of them at night, and a
      feel note comparing day and night.

ephemeral:
  profile: mid
  start_room: 3116
  budgets:
    wall_clock: 50m
```

Create `tools/playtest/goals/scenarios/317-ambient/oasis.yaml`:

```yaml
# #317 ambient: the oasis seat pays Sable for a Planar Oasis rift and
# watches how restless its creatures are.
name: 317-ambient-oasis
summary: Open a rift to the Planar Oasis and watch how often its creatures move.
goals:
  - id: watch-oasis
    do: >-
      You start in the Rift Chamber. Type talk sable and choose the topic
      that opens a rift to the Planar Oasis. When Sable asks how much gold
      you will stake, answer 200. Sable then tells you what to type to step
      through; type it. Inside, stay at the first room (Oasis Threshold) or
      the nearest room where you can watch creatures, for thirty-five
      minutes. If a creature attacks, fight it; you are revived if you fall,
      and you can go back in. Every five minutes, write one tally row: how
      many creatures walked into or out of your room, which kinds (sand or
      storm elementals, or the elemental king, queen, prince or princess),
      how many other lines you saw, and a restlessness score from 1 (they
      stay put) to 5 (always coming and going). End with a sentence on
      whether the movement feels too fast and which creatures cause it.
    verify: >-
      Entered the Planar Oasis; at least six tally rows; a closing note on
      restlessness naming the creatures that move most.

ephemeral:
  profile: veteran
  start_room: 5000
  overlays:
    set_gold: 400
  budgets:
    wall_clock: 50m
```

- [ ] **Step 2: Add a parse test and run it**

`TestParseScenario_RepoMigratedScenarios` (`internal/playtestrun/scenario_repo_test.go:20`) parses a fixed list only, so add `internal/playtestrun/scenario_317_test.go`:

```go
package playtestrun

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The #317 ambient scenario parses: four seats, a 50 minute wall clock,
// every seat's goals file and ephemeral profile valid.
func TestParseScenario_317Ambient(t *testing.T) {
	checkout, playtestRoot := repoPlaytestRoot(t)
	got, err := ParseScenario(filepath.Join(playtestRoot, "scenarios", "317-ambient.yaml"), playtestRoot,
		ScenarioParseOpts{Checkout: checkout})
	require.NoError(t, err)
	require.Len(t, got.Roster, 4)
	require.Equal(t, 50*time.Minute, got.WallClock)
}
```

```bash
go test ./internal/playtestrun/ -run TestParseScenario -count=1
```

Expected: `ok`. If the new test fails, read its error (unknown key, missing goal field, profile rule) and fix the YAML to match the existing scenarios' shape; do not loosen the test.

- [ ] **Step 3: Commit**

```bash
git add internal/playtestrun/scenario_317_test.go tools/playtest/scenarios/317-ambient.yaml tools/playtest/goals/scenarios/317-ambient/town.yaml tools/playtest/goals/scenarios/317-ambient/coulee.yaml tools/playtest/goals/scenarios/317-ambient/steppe.yaml tools/playtest/goals/scenarios/317-ambient/oasis.yaml
git commit -F - <<'EOF'
test(playtest): #317 ambient scenario, four feel-tester seats

Town (tavern, Main Street, market), Pothole Coulee hub, Ironwind Steppe
into the night, and the Planar Oasis through Sable's rift. Each seat keeps
a five-minute tally of unprompted lines by source.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 15: Playtest run 1, the baseline (Opus)

**Files:** none committed. Uses `C:/tmp/dogmud-317-playtest` and `C:/tmp/dogmud-317-evidence`.

Read `dogmud-playtesting` first. Traps: `playtestrun stop` exits 0 and can leave the container up (remove it with `docker rm -f <its container id>` from the run's `manifest.json`, only that id); kill only PIDs you started; the harness `mudagent` is external at `C:/Users/Calabe Davis/workspace/gomud-playtest-harness` (check it is present; restore steps are in the skill).

- [ ] **Step 1: Make the playtest checkout**

```bash
cd C:/tmp/dogmud-317-ambient
git worktree add --detach C:/tmp/dogmud-317-playtest feat/317-ambient-retune
mkdir -p C:/tmp/dogmud-317-evidence
```

- [ ] **Step 2: Set the baseline values in the playtest checkout only**

With the Edit tool on `C:/tmp/dogmud-317-playtest/_datafiles/config.yaml` (never commit this worktree):

| Key | Shipped | Baseline (today) |
|---|---|---|
| `MobIdleChatterScale` | `0.5` | `1.0` |
| `MobIdleMovementScale` | `1.0` | `1.0` |
| `RoomIdleChancePct` | `3` | `5` |
| `AmbientListenerBudgetRounds` | `5` | `0` |
| `AmbientTelemetry` | `false` | `true` |
| `AmbientTelemetryReportMinutes` | `5` | `5` |
| `TipIntervalRounds`, `GossipIntervalRounds`, conversation knobs | today's | unchanged |
| `Modules.weather.EmoteEveryRounds` | `48` | `24` |
| `Modules.weather.SeasonalEmoteOneIn` | `4` | `3` |

Then confirm with `git -C C:/tmp/dogmud-317-playtest diff --stat` that only `_datafiles/config.yaml` changed.

- [ ] **Step 3: Run the scenario**

Invoke the `playtest-scenario` skill with `--checkout C:/tmp/dogmud-317-playtest tools/playtest/scenarios/317-ambient.yaml`, with `GOMUD_HARNESS_DIR=C:/Users/Calabe Davis/workspace/gomud-playtest-harness` set (the default `../gomud-playtest-harness` is relative and the checkout lives under `C:/tmp`). Note the `run_id` from the ready JSON. One subagent drives each seat, per the skill.

- [ ] **Step 4: Save the server log before stopping**

From `C:/tmp/dogmud-317-playtest`, build the CLI to a fixed path once and refresh the log:

```bash
cd C:/tmp/dogmud-317-playtest
go build -o playtestenv.exe ./cmd/playtestenv
./playtestenv.exe logs --checkout C:/tmp/dogmud-317-playtest --run <run_id>
cp tools/playtest/.run/<run_id>/server.log C:/tmp/dogmud-317-evidence/run1-baseline-server.log
grep -c "ambient.telemetry" C:/tmp/dogmud-317-evidence/run1-baseline-server.log
```

Expected: a count of at least 30 (four rooms or more, nine or more windows). If 0, the switch did not take: check the image was built from the edited checkout (`grep AmbientTelemetry` in the run's build log) before anything else. If `logs` fails, fall back to `docker logs <container_id from tools/playtest/.run/<run_id>/manifest.json> > ...` written into your scratchpad, then `cp` it to the evidence folder.

- [ ] **Step 5: Stop the run and verify the container is gone**

Stop through the skill (`playtestrun stop --checkout C:/tmp/dogmud-317-playtest --run <run_id>`), then `docker ps --filter "id=<container_id>"`. If it is still listed, `docker rm -f <container_id>` (that id only).

- [ ] **Step 6: Report**

```bash
cd C:/tmp/dogmud-317-ambient
python -I tools/ambient_report.py C:/tmp/dogmud-317-evidence/run1-baseline-server.log > C:/tmp/dogmud-317-evidence/run1-baseline-table.md
```

Copy each seat's feel report (from the skill's output under `tools/playtest/reports/` in the playtest checkout; reports are gitignored) into `C:/tmp/dogmud-317-evidence/run1-feel-<seat>.md` with `cp`.

- [ ] **Step 7: Post on #317**

Write a comment body with Write into your scratchpad: a two-line header (run 1, baseline values = today, commit `git -C C:/tmp/dogmud-317-playtest rev-parse --short HEAD`, run id), the full table from `run1-baseline-table.md`, a five-line summary of each seat's feel report, and the Oasis decision data (Step 8). Then:

```bash
gh issue comment 317 --repo pruuk/DOGMud --body-file "<scratchpad>/317-run1.md"
```

Read back the URL `gh` prints and confirm it says `pruuk/DOGMud`.

- [ ] **Step 8: Bring the Oasis decision to the owner**

From the moves table, the `Instance Planar Oasis` row gives idle, tracking and path moves per minute. Apply the spec's rule:
- If tracking plus path moves are at least the idle moves, or the oasis seat names the king, queen, prince or princess as the restless ones, Task 16 (scaling tracking and goal moves) is needed.
- Otherwise Task 16 is skipped and recorded as not needed.

Recommend a `wandermultiplier` for the Oasis zone config: `0.5` halves its idle moves, the spec's "Oasis wander slowed"; if the seat rated restlessness 2 or lower at baseline, recommend leaving it unset and say so. Put the numbers, the rule's outcome and the recommendation at the end of the #317 comment, and ask the controller (who asks the owner) for the value. **Stop here and wait for the answer before Task 16 or 17.**

---

## Task 16 (conditional on Task 15 Step 8): Scale tracking and goal moves by the zone wander multiplier (Opus)

Run only if Task 15's rule said so. Otherwise mark this task "not needed: <the numbers>" and go to Task 17.

**Files:** `internal/behaviortree/actions_scout.go`, `internal/behaviortree/actions_goal.go`, `internal/behaviortree/zone_wander_gate_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/behaviortree/zone_wander_gate_test.go`:

```go
package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func wanderGateRoom(t *testing.T, mult *float64) *rooms.Room {
	t.Helper()
	room := &rooms.Room{RoomId: 3180, Zone: "GateZone"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{3180: room},
		map[string]*rooms.ZoneConfig{"GateZone": {Name: "GateZone", WanderMultiplier: mult}}))
	return room
}

func TestZoneWanderAllows(t *testing.T) {
	m := &mobs.Mob{}
	if !zoneWanderAllows(m, wanderGateRoom(t, nil)) {
		t.Error("an unset multiplier always allows the move")
	}
	if zoneWanderAllows(m, wanderGateRoom(t, rooms.MultiplierPtr(0))) {
		t.Error("multiplier 0 never allows a move of the mob's own")
	}
	m.Character.SetMiscData("tracking-user", "Kesh")
	if !zoneWanderAllows(m, wanderGateRoom(t, rooms.MultiplierPtr(0))) {
		t.Error("a move toward a player the mob is tracking is never slowed")
	}
}

func TestZoneWanderAllows_HalfLetsAboutHalfThrough(t *testing.T) {
	room := wanderGateRoom(t, rooms.MultiplierPtr(0.5))
	m := &mobs.Mob{}
	t.Cleanup(util.SetRandForTest(316))
	allowed := 0
	for i := 0; i < 100000; i++ {
		if zoneWanderAllows(m, room) {
			allowed++
		}
	}
	if allowed < 48000 || allowed > 52000 {
		t.Fatalf("multiplier 0.5 allowed %d of 100000, want about half", allowed)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```bash
go test ./internal/behaviortree/ -run TestZoneWanderAllows -count=1
```

Expected: build failure, `undefined: zoneWanderAllows`.

- [ ] **Step 3: Implement**

Append to `internal/behaviortree/actions_scout.go`:

```go
// zoneWanderAllows applies the room's zone wandermultiplier to a move the
// mob makes on its own (#317): below 1 it lets that share of such moves
// through, at 0 none; 1 or more (or unset) always. A move toward a player
// the mob is tracking is never slowed, so a hunt is never weakened.
func zoneWanderAllows(mob *mobs.Mob, room *rooms.Room) bool {
	if mob == nil || room == nil {
		return true
	}
	if v, ok := mob.Character.GetMiscData("tracking-user").(string); ok && v != "" {
		return true
	}
	mult := rooms.ZoneWanderMultiplier(room.Zone)
	if mult >= 1 {
		return true
	}
	if mult <= 0 {
		return false
	}
	return util.Rand(1_000_000) < int(mult*1_000_000)
}
```

and add `"github.com/GoMudEngine/GoMud/internal/util"` to that file's imports. In `actMoveTowardTracked`, replace:

```go
	// Movement parity 4b: fail when the mob cannot pay for the step.
	if !actions.QuoteMobStep(mob, dir).OK() {
		return Failure
	}
```

with:

```go
	// The zone's wandermultiplier slows a step toward a tracked mob (#317).
	if !zoneWanderAllows(mob, room) {
		return Failure
	}

	// Movement parity 4b: fail when the mob cannot pay for the step.
	if !actions.QuoteMobStep(mob, dir).OK() {
		return Failure
	}
```

In `internal/behaviortree/actions_goal.go`, replace:

```go
	result := invokePlannerSafely(fn, mob, currentGoal)
	if result.Command != "" {
```

with:

```go
	result := invokePlannerSafely(fn, mob, currentGoal)
	// The zone's wandermultiplier slows a planner's walk (#317). A dropped
	// pathto is not stamped as acted, so the idle roll still runs this round.
	if strings.HasPrefix(result.Command, "pathto ") && !zoneWanderAllows(mob, rooms.LoadRoom(mob.Character.RoomId)) {
		result.Command = ""
	}
	if result.Command != "" {
```

and add `"strings"` and `"github.com/GoMudEngine/GoMud/internal/rooms"` to its imports.

- [ ] **Step 4: Run and pass**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/behaviortree/ ./internal/hooks/ -count=1
go test . ./internal/guards/ -count=1
```

Expected: gofmt empty; all `ok`.

- [ ] **Step 5: Prove the gate test can fail**

Sabotage: delete the `tracking-user` early return. Run `go test ./internal/behaviortree/ -run TestZoneWanderAllows -count=1`; expected red with `a move toward a player the mob is tracking is never slowed`. Restore; green.

- [ ] **Step 6: Commit**

```bash
git add internal/behaviortree/actions_scout.go internal/behaviortree/actions_goal.go internal/behaviortree/zone_wander_gate_test.go
git commit -F - <<'EOF'
feat(behaviortree): #317 zone wandermultiplier slows tracking and goal walks

Run 1's telemetry showed the Planar Oasis royals' tracking and path moves
at or above their idle moves, so the zone wander multiplier also gates
move_toward_tracked steps and goal planner pathto, never while the mob is
tracking a player.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 17: Set the Oasis wander multiplier (Sonnet)

Only with the owner's value from Task 15 Step 8. If the owner chose to leave it unset, record that in the PR body and skip to Task 18.

**Files:** `_datafiles/world/dogmud/rooms/instance_planar_oasis/zone-config.yaml`

- [ ] **Step 1: Set the value**

Replace the file's last line:

```yaml
instance_gold: true
```

with (V is the owner's value, e.g. `0.5`):

```yaml
instance_gold: true
# Idle wandering here runs at this share of the authored rate (#317,
# chosen from the 2026-10-30 baseline playtest's telemetry).
wandermultiplier: V
```

- [ ] **Step 2: Boot-load check**

```bash
go test ./internal/rooms/ -count=1
DOGMUD_BOOT_SMOKE=1 go test -v -run 'TestSmoke_' . 2>&1 | tail -20
```

Expected: `ok`; the smoke test loads every zone config (a typo or negative value fails it).

- [ ] **Step 3: Commit**

```bash
git add _datafiles/world/dogmud/rooms/instance_planar_oasis/zone-config.yaml
git commit -F - <<'EOF'
content(oasis): #317 slow the Planar Oasis's wandering

wandermultiplier set from the baseline playtest's movement numbers, as
the owner chose on #317.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

## Task 18: Playtest run 2, the shipped values (Opus)

**Files:** none committed.

- [ ] **Step 1: Reset the playtest checkout to the branch head with telemetry on**

```bash
git -C C:/tmp/dogmud-317-playtest checkout -- _datafiles/config.yaml
git -C C:/tmp/dogmud-317-playtest checkout --detach feat/317-ambient-retune
git -C C:/tmp/dogmud-317-playtest log --oneline -1
```

Then with the Edit tool set only `AmbientTelemetry: false` to `AmbientTelemetry: true` in `C:/tmp/dogmud-317-playtest/_datafiles/config.yaml`. `git -C C:/tmp/dogmud-317-playtest diff` must show that one line only.

- [ ] **Step 2: Run, save, stop, report**

Repeat Task 15 Steps 3 to 6 with `run2-shipped` in every evidence file name.

- [ ] **Step 3: Post on #317**

Comment with: header (run 2, shipped values, commit, run id), the run 2 table, the seats' feel summaries, and a short side-by-side of each route room's all-sources rate in run 1 and run 2 plus the Oasis moves per minute in both runs. `gh issue comment 317 --repo pruuk/DOGMud --body-file ...`; check the URL. Any defect a seat found that is not this change's goes to a GitHub issue per `dogmud-playtesting` ("After a run"), searched first.

- [ ] **Step 4: Remove the playtest checkout**

After both runs' evidence is in `C:/tmp/dogmud-317-evidence` (re-check the four `run*-server.log` and `run*-table.md` files exist and are non-empty):

```bash
cd C:/tmp/dogmud-317-ambient && git worktree remove --force C:/tmp/dogmud-317-playtest && git worktree prune
```

Keep `C:/tmp/dogmud-317-evidence` until the PR merges; then delete it as part of that day's `dogmud-end-of-day`.

---

## Task 19: Docs (Sonnet)

**Files:** `internal/ambient/context.md`, `internal/hooks/context.md`, `internal/mobs/context.md`, `internal/mobcommands/context.md`, `internal/actions/context.md`, `internal/rooms/context.md`, `internal/configs/context.md`, `internal/behaviortree/context.md`, `internal/usercommands/context.md`, `modules/weather/context.md`, `modules/weather/engine/context.md`, `docs/schemas/room.md`, `docs/schemas/mob.md`, `docs/PATCH_NOTES.md`

Verify every symbol you name exists (`codegraph_search`, or `Select-String -Path internal\ambient\*.go -Pattern '^(func|type|const|var)\s'`), then run `python -I tools/context_md_audit.py` at the end.

- [ ] **Step 1: Create `internal/ambient/context.md`**

```markdown
# internal/ambient

## Purpose

The one gate every unprompted ("ambient") line passes on its way to players
(#317): the per-listener budget, the once-per-visit rule for a room with a
single idle line, and switchable telemetry. Leaf package: it imports
`configs`, `messaging`, `mudlog`, `rooms`, `users` and `util`, and nothing
that imports it (hooks, behaviortree, mobcommands, usercommands, the
weather engine) may be imported back.

## Files

- **ambient.go**: `Source` and `Sources` (`room_idle`, `mob_chatter`,
  `weather`, `gossip`, `item_chatter`, `say_echo`), `ParseSource`, `Line`,
  `Routing`, `Plan`, `Sees`, `HearsQuiet`, `AllHaveBudget`,
  `SingleLineHeard`, `MarkSingleLineHeard`, `RoundTick`.
- **telemetry.go**: `MoveKind` and `MoveKinds` (`move_idle`, `move_track`,
  `move_path`), `TelemetryMessage`, `TelemetryEnabled`, `CountMove`, and
  the per-window summary.
- **testing.go**: `ResetForTest`, `ChargeForTest` for tests in any package.

## How a send site uses it

    plan := ambient.Plan(ambient.Line{Room: room, Source: ambient.SourceWeather})
    room.SendText(messaging.CategoryWeather, line, plan.Exclude...)

`Plan` charges every player who takes the line in and returns, in
`Exclude`, the caller's own exclusions plus every player over budget. The
caller keeps its own sender, so sight gating, name hiding and categories
stay where they were. Pass `Perceives: ambient.Sees(room)` for a visual
line, so a player who cannot see it is neither charged nor counted.

Counted sources and their sites: room idle (`hooks.sendRoomIdleLine`), mob
idle emote and say and gossip (the `ambient` mob command,
`mobcommands.Ambient`), weather and season lines
(`weather/engine.sendAmbientWeather`), item `item_idle` lines
(`behaviortree.actSpeak`), and the "You hear someone talking." echo of an
ambient say in each neighbouring room (`say_echo`, through
`MobActor.EchoExclude` and `rooms.Room.SendTextToExitsExcluding`, with
`Perceives: ambient.HearsQuiet` since the echo is quiet and only a
`superhearing` player hears it). Exempt: NPC conversations (only their start is
gated, by `AllHaveBudget`), anything a player's own action triggers, tips,
and event lines.

## Rules

- The budget window is `Balance.AmbientListenerBudgetRounds` divided by the
  room's zone `ambientmultiplier` (at least one round). 0 turns the budget
  off. A zone multiplier of 0 with the budget on lets no budgeted line
  through.
- State is in memory only (`budgetNext`, `singleHeard`, the telemetry
  counters), the shape of the item chatter listener cap. A restart clears
  it, which is harmless.
- Telemetry line (stable, read by `tools/ambient_report.py`): message
  `ambient.telemetry`, then `window`, `room`, `zone`, `round_seconds`,
  `window_rounds`, `listener_rounds`, `<source>_offered`,
  `<source>_delivered`, `<source>_suppressed` for each source, and
  `move_idle`, `move_track`, `move_path`. Counts are per listener. Off
  (`Balance.AmbientTelemetry: false`) keeps no counters.
```

- [ ] **Step 2: Fix the stale hooks snippets and add a section**

In `internal/hooks/context.md`, replace:

```
        // Check activity level for idle behavior
        if util.Rand(100) < mob.ActivityLevel {
            events.AddToQueue(events.MobIdle{
                MobInstanceId: mobInstanceId,
            })
        }
```

with:

```
        // Every idle mob gets one MobIdle a round; the ActivityLevel
        // rolls live in HandleIdleMobs (mobs.Mob.RollIdle, #317).
        events.AddToQueue(events.MobIdle{
            MobInstanceId: mobInstanceId,
        })
```

and replace:

```
    // Execute idle command (runs alongside crafting)
    idleCommand := mob.GetIdleCommand()
    if idleCommand != "" {
        mob.Command(idleCommand)
    }
```

with:

```
    // Idle roll (#317): chatter and movement roll apart, each weighted by
    // its share of the pool (mobs.Mob.RollIdle). An emote or say goes out
    // whole as `ambient mob_chatter <cmd>` (issueIdleChatter); a round with
    // neither looks for trouble.
    chatterCmd, moveCmd := mob.RollIdle(idleRollInput(mob, idleRoom), util.Rand)
```

Then append to the end of the file:

```markdown
### Ambient lines (#317)

- `MobIdle_idle_roll.go`: `idleRollInput` (global scales, the room's zone
  multipliers, the mob's `wandermultiplier`) and `issueIdleChatter`.
  Gossip goes out as `ambient gossip say <line>`; `GossipIntervalRounds` 0
  turns it off.
- `NewRound_RoomIdle.go`: `roomIdleChancePer10k` (`RoomIdleChancePct`,
  room -1 four times, zone `ambientmultiplier`) and `sendRoomIdleLine`
  (budget, and a one-line room shows its line once per visit).
- `UserRoundTick` calls `ambient.RoundTick` first each round.
- `BroadcastTips` reads `TipIntervalRounds` (0 off).
- `conversationStartRoll` (`ConversationBaseChancePct`, 0 never); a
  conversation starts only if `ambient.AllHaveBudget(room)`.
- Telemetry counts idle moves in `HandleIdleMobs` and path steps in
  `advanceMobPath`.
```

- [ ] **Step 3: Fix the stale mobs snippets and add a section**

In `internal/mobs/context.md`, replace:

```
// Get idle command based on mob or race defaults
func (m *Mob) GetIdleCommand() string {
    // 1% chance to do nothing (prevents required empty commands)
    if util.Rand(100) == 0 {
        return ""
    }
    
    // Check mob-specific idle commands
    if len(m.IdleCommands) > 0 {
        return m.IdleCommands[util.Rand(len(m.IdleCommands))]
    }
    
    return ""
}
```

with:

```
// Idle roll (#317, idle_roll.go): two independent rolls, each weighted by
// its class's share of the pool (blanks count in the size).
func (m *Mob) RollIdle(in IdleRollInput, randN func(int) int) (chatter, movement string)
```

and replace:

```
// Idle behavior processing
if mob.ActivityLevel > util.Rand(100) {
    idleCmd := mob.GetIdleCommand()
    if idleCmd != "" {
        mob.Command(idleCmd)
    }
}
```

with:

```
// Idle behavior processing (hooks.HandleIdleMobs)
chatterCmd, moveCmd := mob.RollIdle(in, util.Rand)
```

Then append:

```markdown
## Idle roll (#317)

`idle_roll.go`: `IdleClass` (`IdleChatter`, `IdleMovement`), the verb
table `idleVerbClass` (chatter: emote, say, consume, cast, wear, alchemy;
movement: wander, sneak, portal; an unknown verb rolls as chatter and
`internal/guards/idle_verb_class_guard_test.go` fails on one in shipped
content), `IdleVerb`, `ClassifyIdleVerb`, `IdlePool`, `SplitIdlePool`,
`IdleRollInput`, `IdleChances`, `RollIdle`, `WanderScale`,
`CommandNoSplit`.

- chatter chance = AL x `MobIdleChatterScale` x zone `ambientmultiplier`
  x chatter entries / pool size; movement chance = AL x
  `MobIdleMovementScale` x mob `wandermultiplier` x zone
  `wandermultiplier` x movement entries / pool size. At every multiplier
  1.0 the rates equal the old single roll's divided by 0.99 (the 1% forced
  blank is gone).
- `ActivityLevel` is never written by the roll; combat AI reads it.
- `Mob.WanderMultiplier` (yaml `wandermultiplier`): a pointer, nil = 1.0,
  0 = never, negative refuses boot. Instance saves never persist it.
- `CommandNoSplit` queues one input whole; `Command` still splits on `;`
  (the loot goblin's multi-command entries rely on it).
```

- [ ] **Step 4: mobcommands, actions, rooms, configs, behaviortree, usercommands**

Append to `internal/mobcommands/context.md`:

```markdown
## Ambient lines (#317)

`ambient.go`: `Ambient`, the mob-only command `ambient <source> <emote|say>
<text>` that the idle handler and the gossip tick issue (content never
authors it). It asks `ambient.Plan` and sends through `emoteAs` /
`sayAs` (the bodies of `Emote` and `Say`, in `emote.go` and `say.go`) with
the over-budget listeners in `actions.MobActor.Skip`. Plain `emote` and
`say` are never budgeted. Listed in `actions.mobOnlyCommands`.
```

Append to `internal/actions/context.md`:

```markdown
### MobActor.Skip (#317)

`MobActor.Skip` lists players a mob's room line leaves out: the listeners
an ambient emote or say is over budget for. `SendSeen` and `sendSpoken`
pass `roomLineExcludes(actor)` (the actor's own id plus `Skip`) to the
room senders. Empty for every other line. `MobActor.EchoExclude`, set only
by an ambient say, names the over-budget listeners to leave out of the
"You hear someone talking." echo in each neighbouring room
(`sendSayEcho` in `say.go`); every other say echoes as before.
```

Append to `internal/rooms/context.md`:

```markdown
### Zone ambient and wander multipliers (#317)

`ZoneConfig.AmbientMultiplier` (yaml `ambientmultiplier`) scales the
zone's idle mob chatter roll and room idle chance and divides the ambient
budget window; `ZoneConfig.WanderMultiplier` (yaml `wandermultiplier`)
scales idle mob movement. Pointers: unset is 1.0, 0 is never, negative
fails `Validate` (and boot). Read with `ZoneAmbientMultiplier(zone)` and
`ZoneWanderMultiplier(zone)`. An instance clone keeps its template room's
`Zone`, so a paid instance uses its template zone's values. Test helper
`MultiplierPtr`.

`SendTextToExitsExcluding(txt, isQuiet, exclude)` is `SendTextToExits` with
the excludes named per neighbouring room (asked once per neighbour with an
exit back); `SendTextToExits` delegates to it. The ambient say echo uses
it (#317).
```

In `internal/configs/context.md`, replace:

```
Seeded knobs today: `ProgressionFailureFraction` and `ProneAutoStandRounds`
(#398; `< 0` restores the default 5 in `config.balance.misc.go`, 0 turns the
auto-stand guard off, `config_proneautostand_test.go` covers the load path).
```

with:

```
Seeded knobs today: `ProgressionFailureFraction` and `ProneAutoStandRounds`
(#398; `< 0` restores the default 5 in `config.balance.misc.go`, 0 turns the
auto-stand guard off, `config_proneautostand_test.go` covers the load path),
and the #317 ambient knobs `MobIdleChatterScale`, `MobIdleMovementScale`,
`RoomIdleChancePct`, `TipIntervalRounds`, `ConversationBaseChancePct`,
`ConversationPlayerArrivalBoostPct`, `ConversationCooldownRounds` and
`GossipIntervalRounds` (defaults in `config.balance.mobs.go`, load path in
`config_ambient_test.go`). Their Go defaults are today's behaviour; the
retune ships in `config.yaml`.
```

Append to `internal/behaviortree/context.md`:

```markdown
### Ambient budget (#317)

An `item_idle` line from `speak` goes through `ambient.Plan` (source
`item_chatter`): a player over the ambient listener budget, the holder
included, skips it. Event lines are never budgeted. The item listener cap
(`listenerCapNext`) is unchanged and still gates `chatter_ready`.
`move_toward_tracked` counts a `move_track` for ambient telemetry.
```

Append to `internal/usercommands/context.md`:

```markdown
### Arrival conversations and the ambient budget (#317)

`go.go`'s player-arrival conversation boost starts a conversation only if
`ambient.AllHaveBudget(destRoom)`: every player present can hear it out.
```

If Task 16 ran, also append to `internal/behaviortree/context.md`:

```markdown
`zoneWanderAllows` (`actions_scout.go`) applies the zone's
`wandermultiplier` to a `move_toward_tracked` step and to a goal
planner's `pathto` (`RunGoalPlanner`), never while the mob tracks a
player.
```

- [ ] **Step 5: Weather docs**

In `modules/weather/context.md`, replace:

```
  EmoteMode, EmoteEveryRounds, ConditionsEnabled, Persist). Keys are flat because
```

with:

```
  EmoteMode, EmoteEveryRounds (0 = off), EmoteMildChancePct,
  EmoteStrongChancePct, SeasonalEmoteOneIn (0 = never, #317),
  ConditionsEnabled, Persist, SeasonsEnabled). Keys are flat because
```

In `modules/weather/engine/context.md`, replace:

```
- **emotes.go**: `EmitAmbient(graph, fronts, simCfg, weather, tables, roll)` —
```

with:

```
- **emotes.go**: `EmitAmbient(graph, fronts, simCfg, weather, zoneSeasons,
  tables, seasonal, mildPct, strongPct, seasonalOneIn, roll)`: every line
  goes through the ambient listener budget (`sendAmbientWeather`, #317);
```

- [ ] **Step 6: Schemas**

In `docs/schemas/room.md`, after the `non_cartesian` row of the Zone Config Fields table, add:

```markdown
| `ambientmultiplier` | float | Scales the zone's idle mob chatter roll and room idle chance, and divides the ambient listener budget window (1.5 livelier, 0.5 hushed). Unset is 1.0; 0 is never. #317 |
| `wandermultiplier` | float | Scales idle mob movement (wander, sneak, portal) in the zone. Unset is 1.0; 0 is never. #317 |
```

In `docs/schemas/mob.md`, after the `activitylevel` row, add:

```markdown
| `wandermultiplier` | float | no | Scales this mob's idle movement roll (wander, sneak, portal). Unset is 1.0; 0 is never. Chatter and movement roll apart, each weighted by its share of `idlecommands` (blank entries count in the size). #317 |
```

and in the `activitylevel` row, replace `Higher = more active. |` with `Higher = more active. Chatter and movement each roll at this level times their share of the pool. |` (leave the rest of the row as it is).

- [ ] **Step 7: Patch notes**

In `docs/PATCH_NOTES.md`, insert after the `# DOGMud Patch Notes` line and its blank line (use today's date from `Get-Date`):

```markdown
## 2026-10-30: A quieter world

- The world is a little quieter. Townsfolk, shopkeepers and guards
  chatter less often, and the weather speaks up less.
- Busy places stay busy: a crowded tavern still feels crowded, but you
  are no longer buried in lines while you stand still.
- When many things want your attention at once, you hear them one at a
  time, a few seconds apart. Conversations between people nearby are
  never cut off halfway.
- A room with only one thing to show you shows it once each time you
  walk in, instead of repeating it.
- Some townsfolk lines that used to stop halfway now show in full.
```

If Task 17 set an Oasis value below 1, add the bullet `- Creatures in the Planar Oasis drift between rooms less restlessly.`

- [ ] **Step 8: Check and commit**

```bash
python -I tools/context_md_audit.py
go test ./internal/guards/ -count=1
git add internal/ambient/context.md internal/hooks/context.md internal/mobs/context.md internal/mobcommands/context.md internal/actions/context.md internal/rooms/context.md internal/configs/context.md internal/behaviortree/context.md internal/usercommands/context.md modules/weather/context.md modules/weather/engine/context.md docs/schemas/room.md docs/schemas/mob.md docs/PATCH_NOTES.md
git commit -F - <<'EOF'
docs: #317 ambient retune context, schemas and patch notes

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

Expected: the audit reports no phantom symbol in any touched `context.md`; guards `ok`.

---

## Task 20: Gate and PR (Opus)

- [ ] **Step 1: Local gates, in order**

```bash
cd C:/tmp/dogmud-317-ambient
gofmt -l internal/ modules/
go build ./...
go test ./internal/ambient/ ./internal/configs/ ./internal/mobs/ ./internal/rooms/ ./internal/actions/ ./internal/mobcommands/ ./internal/hooks/ ./internal/behaviortree/ ./internal/conversations/ ./internal/usercommands/ ./internal/playtestrun/ ./modules/weather/... -count=1
go test . ./internal/guards/ -count=1
DOGMUD_BOOT_SMOKE=1 go test -v -run 'TestSmoke_|TestQuestDoor|TestDestinationDoor|TestQuestLoop' .
python -I tools/ambient_report.py --self-test
golangci-lint run --new-from-merge-base=origin/master
```

Expected: gofmt prints nothing; every `go test` is `ok` (the smoke run prints its tests as PASS, not SKIP); `self-test ok`; lint `0 issues`.

- [ ] **Step 2: Isolated boot check (from `dogmud-shipping`)**

```bash
S="<your session scratchpad, full path>"
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
printf 'Network:\n  TelnetPort: [43333]\n  LocalPort: 19999\n  HttpPort: 18090\n  AIPort: 45555\n' > "$S/boot-override.yaml"
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
CONFIG_PATH="$S/boot-override.yaml" timeout 180 ./boot-check.exe > "$S/boot.log" 2>&1; echo "exit $?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$S/boot.log"
grep -c "Server Ready" "$S/boot.log"
grep -c "CommandParity" "$S/boot.log"
```

Expected: `exit 124` (the server stayed up), `0` panics, `1` Server Ready, and no `CommandParity` warning naming `ambient`. Clean up with `cd C:/tmp/dogmud-317-ambient && git worktree remove --force C:/tmp/dogmud-boot-check` (PowerShell `Remove-Item -Recurse -Force` if Windows holds the exe).

- [ ] **Step 3: Push and open the PR**

Write the PR body with Write into your scratchpad (`pr-body.md`):

```markdown
## Summary

#317 ambient retune, per the spec `docs/superpowers/specs/2026-10-30-ambient-retune-design.md`.

- Per-source knobs, 0 = off: `MobIdleChatterScale` (ships 0.5), `MobIdleMovementScale` (1.0), `RoomIdleChancePct` (3), weather `EmoteEveryRounds` (48) and new `SeasonalEmoteOneIn` (4), the conversation knobs written into config.yaml, `TipIntervalRounds` (75), `GossipIntervalRounds` (75). Go defaults are today's behaviour.
- Mob idle chatter and movement roll apart, each weighted by its share of the pool, so every scale at 1.0 keeps today's rates (tested by formula and a seeded million-round run). A guard holds the verb table complete.
- A one-line room shows its line once per visit.
- `internal/ambient`: a per-listener budget (`AmbientListenerBudgetRounds`, ships 5) for room idle, mob idle chatter, weather, gossip and item chatter; conversations start only when everyone present has budget.
- The "You hear someone talking." echo of an ambient say charges each neighbouring room's listeners too (owner ruling 2026-10-30, telemetry `say_echo`). The echo is quiet, so today it reaches only players with the superhearing flag; other says echo as before.
- Zone `ambientmultiplier` / `wandermultiplier` and mob `wandermultiplier` (unset 1.0, 0 never). Planar Oasis `wandermultiplier`: <value, or "left unset">.
- Telemetry behind `AmbientTelemetry` (off) and `tools/ambient_report.py`.
- <If Task 16 ran: "The zone wander multiplier also slows tracking and goal walks (run 1 showed the royals' moves dominate)." Else: "Tracking and goal walks are not scaled; run 1 showed idle moves dominate (numbers on #317).">

Fixes found on the way: 30 idle lines with a `;` were cut short and now show whole; Wenna's `emote's` entry never ran and is rewritten.

## Playtests

Run 1 (baseline) and run 2 (shipped) tables and feel reports are on #317: <links to the two comments>.

## Test plan

- [x] `gofmt`, `go build ./...`, touched packages, root and guards, boot smoke loops
- [x] `golangci-lint --new-from-merge-base=origin/master`: 0 issues
- [x] Isolated boot check: exit 124, Server Ready, no CommandParity warning
- [x] `tools/ambient_report.py --self-test`

Closes #317. This was the last open item under the messaging epic #381.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

Fill the placeholders from Tasks 15 to 18 before pushing. "Closes" must name only #317 (a closing keyword before #381 would close the epic).

```bash
cd C:/tmp/dogmud-317-ambient
git push -u origin feat/317-ambient-retune
gh pr create --repo pruuk/DOGMud --base master --head feat/317-ambient-retune --title "feat: #317 ambient retune, per-source knobs and a listener budget" --body-file "<scratchpad>/pr-body.md"
```

Read the printed URL: it must be `https://github.com/pruuk/DOGMud/pull/...`. Then `gh pr checks <n> --repo pruuk/DOGMud --watch` and confirm with `gh run list --repo pruuk/DOGMud --branch feat/317-ambient-retune` that the expected workflows ran. Do not merge without the controller's go; the owner deploys.

---

## Self-review against the spec

| Spec item | Task |
|---|---|
| Section 1 knobs, 0 = off, shipped values | 2, 3, 9, 10, 11 |
| Split roll weighted by share, blanks counted, 1.0 = today, both may fire, lookfortrouble when neither, ActivityLevel untouched, 1% blank dropped | 4, 9 |
| Verb table and guard that can fail | 4, 5 |
| Single-line rooms once per visit | 7, 10 |
| Section 2 budget, counted and exempt sources, one helper, conversations gated | 7, 8, 10, 11 |
| Section 2 echo of an ambient say budgeted, `say_echo` telemetry (owner 2026-10-30) | 8A, 12 |
| Section 3 zone and mob multipliers, pointers, Oasis value after baseline, conditional tracking and goal scaling | 4, 6, 9, 15, 16, 17 |
| Section 4 telemetry, report script, two playtests on #317 | 12, 13, 14, 15, 18 |
| Testing list (knob config tests and 0 silences; parity; budget; zone multiplier; single line; guard; telemetry; script) | 2 to 13 |
| Docs | 19 |
| Gate | 20 |
