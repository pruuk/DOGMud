# Moving by touch, plan 2a: the server (implementation plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One server rule decides which exits a player knows (secret exits only once found), every exit lister and every exit-naming path reads it, NPCs follow ruling 13, and `Room.Info` carries the first slice of the button protocol (movement ids, press handling, stale refusal, dark marks) and is kept current by a per-round watcher.

**Architecture:** `characters.Character.KnowsSecretExit` is the one secret-exit predicate. `rooms.KnownExits(room, char)` builds the per-room player view (name, kind, locked now, destination, dark) on top of it, and `Room.ExitKnownTo`, `Room.FindKnownExitByName`, `Room.FindKnownExitTo`, `Room.GetRandomKnownExit` and `mapper.SecretExitsKnownTo` are thin filters over the same predicate. A new leaf package `internal/buttons` issues movement button ids and holds dark marks; `modules/gmcp` puts both into `Room.Info`, resolves presses (`Char.Button.Press`) into typed input, and re-sends `Room.Info` from a once-a-round watcher modelled on `lastSentVitals`.

**Tech Stack:** Go 1.25 (module `github.com/GoMudEngine/GoMud`), testify, Go AST guards in `internal/guards`, plain Node 24 jstests for the one client change.

**Spec:** `docs/superpowers/specs/2026-11-22-moving-by-touch-design.md` (issue #781, epic #783). This plan covers "2a: the server", the server half of "The button protocol slice", the server and protocol parts of "Tests and checks", owner rulings 1, 2, 3 (server state), 4 (dark marks, server side), 6, 7, 8 and 13, and the measurement. Client rendering (2b) and the Map tab and sideways phone (2c) are not here. The only client change is the `Room.Info` handler not re-centring the map when the room is unchanged (spec "Kept current").

**Branch.** Execute on `feature/781-moving-by-touch-2a`, branched from `docs/spec-moving-by-touch` (or from `master` once the spec PR has merged). Every commit below builds and passes its tests.

**Rules every task follows (brief every subagent on these):**
- Edit files with the Edit and Write tools only. No `sed -i`, no Python read-modify-write, no `>` into tracked files (`guard.py` H3).
- `git add` named paths only; never `-A`, `.`, `--all`, never `git commit -a` (H1). Never stash (H6). Never `--no-verify` (H2).
- No em or en dashes in code comments, docs or player text. Player text follows `dogmud-player-copy` (80 columns, no raw numbers).
- Every task's test step also runs `go test . ./internal/guards/ -count=1` (the line-number allowlist guard lives there).
- A new test is trusted only after it was seen red: every task has a sabotage step. Restore the exact line afterwards.
- Every package touched keeps its `context.md` accurate (the step says what to add). `tools/context_md_audit.py` runs in the final gate.
- Commit messages through a quoted heredoc, ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

---

## Facts verified against source (2026-11-22, read on `docs/spec-moving-by-touch` at `9c6661805`)

| # | Fact | Where |
|---|---|---|
| F1 | `Room.Exits map[string]exit.RoomExit` (yaml `exits`, `instance:"skip"`); `Room.ExitsTemp map[string]exit.TemporaryRoomExit` (yaml `-`) | `internal/rooms/rooms.go:117`, `:119` |
| F2 | `exit.RoomExit{RoomId, Secret, MapDirection, ExitMessage, Lock gamelock.Lock, OneWay}`; `HasLock()` is `Lock.Difficulty > 0`. `exit.TemporaryRoomExit{RoomId, Title, UserId, SpawnedRound, Expires}`; `AddTemporaryExit` sets `Title = exitName` when empty | `internal/exit/exit.go:7-26`; `rooms.go:816-836` |
| F3 | `gamelock.Lock.IsLocked()`: false at difficulty 0; true when `UnlockedRound == 0`; otherwise true once the round count passes `gametime.GetDate(UnlockedRound).AddPeriod(RelockInterval or "1 hour")`. `SetUnlocked` writes `UnlockedRound = util.GetRoundCount()` (the round count starts at `RoundCountMinimum` = 1,314,000, never 0; tests pin it with `util.SetRoundCountForTest` so a relock round can be computed). There is no turn-count setter: `util.GetTurnCount` / `IncrementTurnCount` only | `internal/gamelock/gamelock.go:24-55`; `internal/util/util.go:90`, `:125-157` |
| F4 | Mutator exits: `MutatorSpec.Exits map[string]exit.RoomExit`; `Room.ActiveMutators` iterates room then zone mutators (outdoor-only skipped indoors). `GetExitInfo` gives the first active mutator with that name priority over `Room.Exits`; `GetDetails` instead copied every mutator's exits last (last wins). No shipped mutator defines an exit (0 of 23 files under `_datafiles/world/dogmud/mutators/` contain `roomid`) | `internal/mutators/mutators.go:78`; `rooms.go:3251`, `:1518-1533`; `roomdetails.go:434-438` |
| F5 | `GetDetails` builds `VisibleExits` (a secret exit only when the far room's visitor trail has the viewer, `HasVisited`) and `TemporaryExits` (`maps.Copy` of `ExitsTemp`), then copies mutator exits over `VisibleExits`. It ignores sight: the `exits` command lists exits in the dark today | `internal/rooms/roomdetails.go:363-379`, `:434-438`; `internal/usercommands/exits.go:11-19` |
| F6 | The exits line template reads `.VisibleExits` (`$exitInfo.Secret`, `.HasLock`, `.Lock.IsLocked`) and `.TemporaryExits` (`.Title`) | `_datafiles/world/dogmud/templates/descriptions/exits.template:6-30` |
| F7 | `visitorTrackingTimeout = 180` (seconds, scaled by `TurnsPerSecond`); `HasVisited(id, vType)` reads the trail | `rooms.go:31`, `:2041-2050` |
| F8 | Sight: `messaging.ParticipantSight(observer, room) SightDecision`, `SightFull`/`SightShapes`/`SightNone`; nil observer is `SightFull`. `actions.ResolveLook(actor, lookAt)` gives `LookTooDark`/`LookBlind` at no sight; `usercommands.noSightRefusal(kind)` words it | `internal/messaging/predicates.go:56`; `internal/actions/look.go:64-87`; `internal/usercommands/look.go:617-625` |
| F9 | Discoveries: `Character.Discoveries map[int][]string` (yaml `discoveries`, saved with the user file); `SecretExitDiscoveryKey(name) = "exit:"+name`; `HasDiscovery`, `AddDiscovery` (allocates the map when nil) | `internal/characters/character.go:331`, `:673-692` |
| F10 | `search` tier 1 walks `room.Exits` only (not mutator exits), records `AddDiscovery(room.RoomId, exit key)` on a find at target 125, and re-reports a found one without rolling | `internal/actions/search.go:419-454` |
| F11 | NPC searches record on the mob's own character: callers `internal/mobcommands/search.go:11`, `internal/behaviortree/actions_scout.go:133` | as stated |
| F12 | Mob discoveries are not persisted: `MobInstanceData` has no discoveries field. Spawn copies the template with `mob := *m` and deep-copies Skills, SpellBook, Mutations, Shop, Groups, Adjectives, not Discoveries | `internal/mobs/instance_save.go:22-52`; `internal/mobs/mobs.go:486-596` |
| F13 | `FindExitByName(search)`: a direction alias (`keywords.TryDirectionAlias`) is tried exactly against `Exits`, `ExitsTemp`, mutator exits; otherwise fuzzy `util.FindMatchIn` over all three, with a `portal` prefix rule. It never looks at `Secret` | `rooms.go:2677-2757` |
| F14 | Non-test callers of `FindExitByName` (14): `actions/combat_fire.go:116` (`ShotSight`), `:228` (`ExecuteFire`); `actions/defuse.go:218` (`resolveDefuseLockTarget`); `actions/go.go:31` (`FindExit`); `actions/look.go:138`, `:141` (`ResolveLook`); `usercommands/keyring.go:95`, `:130` (`KeyRing`); `usercommands/lock.go:31`; `usercommands/picklock.go:63`; `usercommands/shoot.go:728` (`resolveShootTarget`); `usercommands/unlock.go:26`; `usercommands/usercommands.go:416` (`TryCommand` exit rewrite); `modules/aicompanion/loot.go:160` (`mobCompanionUnlock`) | grep `FindExitByName(` |
| F15 | Non-test callers of `actions.FindExit` (5): `actions/move.go:110` (`QuoteMobStep`); `mobcommands/go.go:50` (`isMoveAttempt`), `:132` (`Go`); `usercommands/go.go:57` (`Go`); `usercommands/usercommands.go:656` (`fallbackRoute`) | grep `FindExit(` |
| F16 | Player `Go` refusals in order: combat `refusal.WhileFighting` :77; quest sequence lock :82; `NoMovement` condition :96; locked door line :185; quest `room_gate` :206; exhaustion :227; instance not opened :262-268 (MoveToRoom narrates); no such exit "You're bumping into walls." :615 (only for the ten full compass words in `rest`). A non-compass word with no exit returns `(false, nil)`, which the dispatcher answers "not recognized" | `internal/usercommands/go.go` |
| F17 | Party follow: the leader's `Go` runs `partyUser.CommandFlagged(rest, events.CmdFollowStep)` after a `DepartureSight` check; charmed mobs run `mob.Command(rest)` | `usercommands/go.go:353-391` |
| F18 | Follow module: `roomChangeHandler` finds the exit to `ToRoomId` in `fromRoom.Exits` then `ExitsTemp`; a mob follower runs `mob.Command(followExitName, .25)`; a user follower is sight-checked, then `user.Command(followExitName, .25)` | `modules/follow/follow.go:346-441` |
| F19 | NPC exit choice points that include secret exits today: `mobcommands/wander.go:79-103` (`wander loot`, `wander players` candidate loops over `room.Exits`); `behaviortree/actions_mob.go:295-321` (`actGoToCallerRoom`); `behaviortree/actions_archer.go:315-330` (`pickRetreatExit`, used by `actFlee`'s walk-away); `hooks/NewRound_DoCombat_unified.go:931-941` (a mob walks to a player who attacked from the next room); `mobcommands/pathto.go:51` (`mapper.GetPath`, every secret exit); `modules/follow` mob followers; `mobs.MovePackFollowers` (packmates follow the alpha's exit) | as stated |
| F20 | NPC paths that already skip every secret exit (stricter than ruling 13, left alone): `Room.GetRandomExitWhere` (wander's fallback and flee); `actions.fleeExit` (`flee.go:215-235`); scout tracking `findBestExitToward` (`actions_scout.go:282-289`); `actions.Scan` (`scan.go:62-65`); `actions.findExitedTrack` (`track.go:653-655`); `Room.findMobExit`/`findUserExit` (`rooms.go:2867-2872`, `:2911-2916`); `modules/aicompanion` (`worldmap.go:103`, `autonomy.go:713`, `tools.go:191`, `perception.go:100`) | as stated |
| F21 | `pathto` callers: planners (18 `PlanResult{Command: "pathto ..."}` sites in `internal/planners/`), all executed by one call, `mob.Command(result.Command)` in `behaviortree.RunGoalPlanner` (`actions_goal.go:71-72`). Scripted callers: schedules (`hooks/NewRound_IdleMobs_schedule.go:204`, `:212`), patrols (`NewRound_IdleMobs_patrol.go:117`, `:177`), idle commands (`NewRound_IdleMobs.go:208`), behaviour trees (`actions_forager.go` 4, `actions_forager_storage.go:64`, `actions_party.go:385`), ferry factor (`ferry/factor.go:321`), home returns (`wander.go:65`, `mobcommands/go.go:128`, `MobIdle_HandleIdleMobs.go:120`, `:317`, `NewRound_IdleMobs.go:216`) | grep `pathto` |
| F22 | `mapper.GetPath(start, end...)` (A*, LRU `pathCache` of 128) takes every secret exit; `mapper.SecretExitKnown func(roomId int, exitName string, toRoomId int) bool`, nil lets none through; `anySecretExit` (unexported) lets all through; `NearestOf(..., known)` is quest guidance | `internal/mapper/mapper.path.go:125-290`; `mapper.nearest.go:79-90` |
| F23 | Secret-exit listers besides `GetDetails` and `Room.Info`: `Zone.Map` snapshot hides a secret exit until the far room is in the player's visited set (`mapper.snapshot.go:250-263`, loop `for _, e := range n.Exits`); the ASCII map `GetLimitedMap` uses the 180-second trail (`mapper.go:977-987`, loop `for _, exitInfo := range node.Exits`); quest guidance `secretExitsKnownTo` accepts a discovery OR the far room on the map OR the trail (`questengine/destination.go:383-395`, pinned by `destination_secret_test.go:16-47`) | as stated |
| F24 | `Snapshot` callers: prod `modules/gmcp/gmcp.Zone.go:200` (`view.Snapshot(visited)`, user in scope) and `modules/gmcp/gmcp.Build.go:1168` (builder); tests `mapper.snapshot_cold_test.go:134`, `:174`, `:194`, `:280`; `mapper.snapshot_test.go:25`, `:52`, `:70`, `:99`, `:151`, `:203`, `:248`; `mapper.zoneborder_test.go:598` | grep `.Snapshot(` |
| F25 | `Room.Info` exits: built in `GMCPRoomModule.GetRoomNode` (`gmcp.Room.go:478-542`): `exits` (name to room id) and `exitsv2` (`num`, `dx`, `dy`, `dz`, `details`). A secret exit is listed only on the visitor trail (:489-495). `locked` is a string in `details`, added from `HasLock()` (:524-538), with `player_has_key` / `player_has_pick_combo`. Temporary and mutator exits are not listed. In the dark both maps are present and empty (:479-487) | `modules/gmcp/gmcp.Room.go` |
| F26 | Payload structs: `GMCPRoomModule_Payload` (`num`, `name`, `description`, `area`, `environment`, `symbol`, `coords`, `exits`, `exitsv2`, `details`, `Contents`); `GMCPRoomModule_Payload_Contents_ExitInfo` (`num`, `dx`, `dy`, `dz`, `details`) | `gmcp.Room.go:601-621` |
| F27 | `Room.Info` is sent on `RoomChange` (moved player) and `SightBandChanged`; `buildAndSendGMCPPayload` handles `GMCPRoomUpdate{UserId, Identifier}` and checks `isGMCPEnabled` | `gmcp.Room.go:38-42`, `:159-264` |
| F28 | Watcher pattern: `lastSentVitals map[int]...` + mutex, `buildVitalsPayload`, `recordSentVitals`, `forgetSentVitals`, `vitalsDue` (never sent is due), `vitalsWatchHandler` on `events.NewRound` over `users.GetAllActiveUsers()` gated on `isGMCPEnabled`; despawn forgets | `modules/gmcp/gmcp.CharVitalsWatch.go:28-103`; registered `gmcp.Char.go:128-129` |
| F29 | Inbound GMCP: `HandleIAC` copies the payload and queues `GMCPCharOp{ConnectionId, Command, Payload}` for `Char.Automation.*`, `Char.Action.Try`, `Char.Quests.*`; `handleCharOp` runs on MainWorker; `userIdForConnection(connId)` scans active users | `modules/gmcp/gmcp.go:400-415`, `:500-507`; `gmcp.CharOp.go:13-160` |
| F30 | Typed input: the input worker queues `events.Input{UserId, InputText, ReadyTurn: GetTurnCount(), Flags: CmdTyped, Seq}`. `HandleInputEvents` runs at most one ready input per user per turn (`userInputEventTracker`, cleared each `EventLoop`); `ReadyTurn <= 0` bypasses the limit. `UserRecord.CommandFlagged(text, flags)` queues at the current turn. `CmdTyped` lets a line answer a menu, a prompt or the pager; macro lines keep it with `Seq` 0, and `Char.Input.Done` ignores `Seq` 0 | `world.go:86-190`, `:983-990`, `:1020-1110`; `internal/users/userrecord.go:442-460`; `modules/gmcp/gmcp.InputDone.go:31-36` |
| F31 | `UserRecord.GetPrompt()` is the open prompt (nil when none); `MenuOrPromptOpen()`, `HasOpenMenu()`; tests open one with `u.StartPrompt("mail", "Bob")` | `internal/users/userrecord.go:837`, `:878`; `userrecord.menu.go:42-53`; `internal/users/prompt_interrupt_test.go:43` |
| F32 | `Commands.State` (mode) is pushed every round to GMCP users; nothing server-side is needed for combat dimming in 2a | `modules/gmcp/gmcp.Commands.go:96-115` |
| F33 | Web client `Room` handler: per `Room.Info` it pushes the room into `allRooms.roomZones[zoneKey]`, adds it to the grid unless known from a snapshot, and always calls `gr.centerOnRoom(r.RoomId)`; it draws an exit edge for every `exitsv2` entry with `dz == 0`. `allRooms` is declared at :820 | `_datafiles/html/public/webclient-pure.html:820-823`, `:881-976` |
| F34 | `static/js/mapwhere.js` is a UMD module (`MapWhere.text`) tested by `tools/jstest/mapwhere.test.js`, which already reads `webclient-pure.html` for source checks | as stated |
| F35 | Refusal lines live in `internal/refusal` (one helper and a `...Line` constant per situation, `send(to, line)` on `CategoryRefusal`) | `internal/refusal/refusal.go`, `nothere.go` |
| F36 | `NpcMarkersChanged{UserId}` is the shape for a unique per-user refresh event (`UniqueID`, a typed drain for tests) | `internal/events/npcmarkers.go` |
| F37 | Creature-lookup AST guard to mirror: `internal/guards/lookup_viewer_guard_test.go` (`lookupRegistry`, `lookupFuncName`, walk of `internal` and `modules`) | as stated |
| F38 | Fixtures: `rooms` `seedRegistry()` room 1 has `north`, `south`, `east` (lock 10) and `west` (secret) (`rooms_test.go:16-62`); `usercommands` `seedAllRegistries()` room 1 (Lamp 60) north to room 2, users 1 and 2 in room 1, mob 100, biome `cave` with SkyLight 0 (`usercommands_test.go:115-366`); gmcp `roomSightFixture(t, lamp)` rooms 9720 (north to 9721) and 9721, viewer 9722 (conn 97722), Kesh 9723 (`gmcp.RoomSight_test.go:19-60`); mapper `secretRingWorld` (`mapper.secret_guidance_test.go:47-62`); follow `followWalkScene` (`follow_light_test.go:28-77`); actions `scanLockScene` (`scan_lock_test.go:30-69`); mobs `seedTemplateIsolationFixture` (`spawn_template_isolation_test.go:28-82`) | as stated |
| F39 | Shipped secret exits: 5 (`6323 west`, `5441 east`, `6418 north`, `4161 down`, `487 down` locked 15). Rooms 6418 (sewer biome, `skylight: 0.0`, no lamp) and 487 (cave) are dark | `_datafiles/world/dogmud/rooms/`, `biomes/sewer.yaml` |
| F40 | Non-T exits in 1,355 room files: 22 diagonal, 10 named (`out` 2, `watchtower` 2, `shrine`, `overlook`, `drift`, `in`, `portal`, `crack`); the most other exits in one room is 2 | room YAML count |
| F41 | Node is v24 locally; CI's JS job uses Node 20 and runs `node --check` on every non-vendored `.js`, every `tools/jstest/*.test.js`, every `tools/webclient-tests/*.js` | `.github/workflows/validate.yml:70-130` |

### Corrections to the spec's facts table

The spec was not edited; these are recorded here for the owner.

- **Row "NPC wandering picks from every exit in `room.Exits`, secret ones included" (`wander.go:82-105`)** is only half true. `wander loot` and `wander players` build candidates from every exit, secret included; plain wandering falls back to `GetRandomExit`, which already skips every secret and every locked exit (F19, F20).
- **Row "Fleeing (player or NPC) already skips secret ... exits" (`flee.go:215-221`)** is true of `actions.fleeExit`, but the behaviour tree's out-of-combat walk-away (`actFlee` through `pickRetreatExit`, `actions_archer.go:315`) takes secret exits (F19).
- **Row "`go` and a bare exit name reach exits through `FindExitByName`"** is true, and seven more player paths resolve exits the same way: `look`, `lock`, `unlock`, `picklock`, `shoot`, `fire`, `defuse` (F14). Each would confirm an unfound secret exit.
- **Rows on `Zone.Map` and `search`**: two more listers use the old rules, the ASCII map (`GetLimitedMap`, the 180-second trail) and quest guidance (`secretExitsKnownTo`: discovery, map or trail) (F23). "Quest destinations already read it, nothing else does" is true, but destinations also accept the map and the trail.
- **Row "`exitsv2` `locked` is set from `HasLock()`"**: `locked` is a string inside `details`, not a field (F25).
- **Row "Any actor can search ... `search.go:39-45`"**: the record is at `search.go:443-449`; lines 39-45 are `spotsHider`.
- **Row "Movement refusals in order ... `go.go:77, 96, 185, 206, 227, 615`"**: eight refusals, nine anchors: combat 77, quest sequence lock 82, no-go 96, locked 185, room_gate 206, exhaustion 227, instance 262-268, no exit 615 (F16).
- **Row "no room has more than three"**: true; the most is two (F40).
- **Row "`Room.Info` exits ... `gmcp.Room.go:480-540`"**: the block is 478-542.

---

## Decisions taken without asking

| # | Decision | Why | If wrong |
|---|---|---|---|
| D1 | The single secret-exit rule is a method, `Character.KnowsSecretExit(roomId, exitName)`, beside `SecretExitDiscoveryKey`; `rooms.KnownExits` and every filter call it | Every lister (rooms, mapper, questengine, mobcommands, behaviortree, hooks, follow) already reaches `characters`; a rooms-only function could not be called from `mapper` snapshots, which must not load rooms | Move it; the callers are listed in this plan |
| D2 | `rooms.KnownExits(r *Room, c *characters.Character) KnownExitList` computes sight itself (`ParticipantSight`); a nil character sees fully and knows no secret exit | The spec's `KnownExits(room, user)`; `rooms` cannot import `users` types for a user parameter without needing only the character | Take a sight argument instead |
| D3 | Widened scope: every player verb that names an exit (`look`, `lock`, `unlock`, `picklock`, `shoot`, `fire`, `defuse`, the `go` rewrite and `fallbackRoute`) resolves through `FindKnownExitByName`, and a new AST guard keeps unfiltered lookups registered | Ruling 1 plus "a guess reveals nothing": `look west` or `pick west` at an unfound secret door would confirm it. CLAUDE.md: a flaw found mid-task widens scope | Revert those call sites to `FindExitByName` and register them in the guard |
| D4 | The ASCII map and quest guidance move to the discovery rule too | Ruling 1 is one rule everywhere (spec decision on `Zone.Map`); the spec listed neither | Restore their old rules in `GetLimitedMap` and `secretExitsKnownTo` |
| D5 | The `exits` command in the dark answers with look's dark line ("It is too dark to see. You need light, or eyes that do not need it.") instead of "Exits: None" | `KnownExits` lists nothing at no sight; "None" would be untrue | Print "Exits: None" |
| D6 | Mutator exits follow `GetExitInfo`'s precedence (first active mutator over the room exit) in `KnownExits`; `GetDetails` used last-wins | One rule; no shipped mutator has exits (F4) | Make `KnownExits` last-wins |
| D7 | Temporary exits appear in both `exits` and `exitsv2`, `kind: "temporary"`, `num` their destination; the web map skips them as edges | The spec makes `Room.Info` the one exit list; a gangplank drawn as an edge to its own square would be wrong | List them in `exitsv2` only |
| D8 | `Room.Info` gains top-level `dark` (bool), `moves` (the six T directions to their fixed ids, sent in every room, dark or lit) and `darkmarks` (direction to `"wall"` or `"locked"`), and per exit `kind` and `id`. `details` keeps `secret`, `player_has_key`, `player_has_pick_combo`; `locked` now means locked now | In the dark `exitsv2` is empty, so the fixed ids need their own field; they reveal nothing (same everywhere) | Rename the fields before 2b builds on them |
| D9 | Fixed ids are `m1` to `m6` (north, south, east, west, up, down); per-room ids are `x` plus 8 random hex digits from `crypto/rand`, kept while the player stays in the room and the exit stays known | Opaque, unguessable, stable so a resend does not make a tap stale | Any other id shape; only `internal/buttons` changes |
| D10 | A fixed id is accepted in any room at any time; a per-room id only for the room it was issued for | The server does not know whether the deck is on; ruling 8 makes fixed ids "north from wherever you are" | Track deck state from the client |
| D11 | The stale line is `That button was out of date. Try again.` (in `internal/refusal`), and a stale press re-sends `Room.Info` | Ruling 7 asks for a plain line; the re-send makes "try again" true | Reword |
| D12 | The press message is `Char.Button.Press` with `{"id": "..."}`, deferred to MainWorker through `GMCPCharOp`, queued with `CommandFlagged(line, events.CmdTyped)` at the current turn | "Pressing north is typing north": the same parser, the one-per-turn limit, and a menu or pager stop treats it as typed; `Seq` 0 sends no `Char.Input.Done` (F30) | Queue without `CmdTyped` (a press would then not close a menu) |
| D13 | A press while a prompt is open is dropped silently | A prompt takes the next typed line as its answer (a password, a confirm); a button is never an answer | Refuse with a line, or let it answer |
| D14 | Dark marks are kept for the six T directions only, in memory in `internal/buttons` (lost on restart and logout), cleared on the player's `RoomChange` and lazily once the room is not dark to them; a new mark queues `events.RoomExitsChanged` so `Room.Info` goes out at once | Only the T is offered in the dark; marks are session state, not living state (`dogmud-persistence`); a 4-second round lag after a bump would feel broken | Persist them; mark diagonals too |
| D15 | A follower learns a secret exit when its follow step is issued (after the existing sight check), before the step runs | The step runs through the follower's own known-exit rule; learning after it would refuse the step | Learn on arrival and let the follow step bypass the rule |
| D16 | A follow-module NPC follower that is not charmed by the mover stops following (`follow stop`) at a secret exit it has not found; a charmed one learns it and follows | Ruling 13: following a player is unscripted; a companion moving with its leader learns | Let any follower learn |
| D17 | Packmates moving behind their alpha (`MovePackFollowers`) learn the alpha's secret exit | Ruling 2's NPC analogue: they walked through behind their leader | Packmates stop at it instead |
| D18 | Classification of NPC moves. Unscripted, filtered: `wander loot`/`players` candidates and the wander fallback, `actGoToCallerRoom`, `pickRetreatExit`, the mob aggro walk in `NewRound_DoCombat_unified.go`, planner `pathto` (rewritten to `pathto found ...` in `RunGoalPlanner`), follow-module followers. Scripted, unchanged: schedules, patrols, idle commands, behaviour tree `pathto`/`move`, ferry factors, `pathto home` returns, admin `command`. Unchanged because already stricter: everything in F20. `callforhelp`'s `go <roomId>` is unchanged (it moves the mob with or without an exit) | Ruling 13 and the spec decision on schedules; a mob's return home is its regular way | Move any row; each is one call site |
| D19 | `GetRandomKnownExit` lets a wanderer take a secret exit it found; locked exits stay out | The spec test "takes one it has" for wandering | Keep the fallback secret-free |
| D20 | Mob templates' `Discoveries` are deep-copied at spawn | Mob discoveries are written per instance now; a shared map would teach every spawn (`dogmud-refactoring`, shallow copies) | Drop the copy (no template authors `discoveries:` today) |
| D21 | `mapper.GetPathKnown` reuses the shared path cache when the cached route takes no refused secret exit, and otherwise searches again uncached | Only five secret exits exist; recomputing every planner `pathto` would waste the cache | Key the cache on the filter |
| D22 | New leaf package `internal/buttons` holds movement ids and dark marks | Both are read by `modules/gmcp` and written or tested from `internal/usercommands`; `internal` cannot import `modules` | Split marks into `actions` |
| D23 | The client change goes into `mapwhere.js` (`MapWhere.recentre`) rather than a new file | It is the map's "where am I"; no new file to list | A new module |
| D24 | The measurement counts as "noticeable" when the median lag from the move's "You head" line to `Room.Info` is over 250 ms, or any single one is over 500 ms | The spec gives no threshold | Owner sets one |
| D25 | The adversarial seat starts in room 6418 (dark sewer, secret north) as `early` with a Hooded Lantern (20097) and Search 3 | A dark room with a secret exit and a real wall beside it; the lantern lets the seat compare dark and lit; Search 3 makes a find reachable in a session | Another room or profile |
| D29 | (Task 7, 2026-11-25) `actGoToCallerRoom` and the mob aggro walk in `NewRound_DoCombat_unified.go` use `Room.FindKnownExitTo`, which widens them: they now also take a temporary exit (by its `Title`, the name `FindKnownExitTo` returns for one) and follow mutator precedence; before, both read `room.Exits` only. Accepted | One known-exit lookup for every NPC heading to a room; no shipped mutator has exits (F4) | Restore a `room.Exits` loop with the known-exit check at both sites |
| D30 | (Task 7) Correction to F20: `actions.fleeExit` honours a named (preferred) exit without checking `Secret`; only its random fallback skips secret exits. Unscripted mob callers that name a flee exit filter at the caller (`pickRetreatExit`, the survival planner's `pickRandomExit`); a scripted `flee <exit>` keeps every exit under ruling 13. A player's flee names no exit (`usercommands/flee.go` passes `""`) | Ruling 13 splits on who chose the exit, which only the caller knows | Check `Secret` inside `fleeExit`, which would also refuse a scripted `flee <secret exit>` |
| D31 | (Task 7) The survival planner's flee exit (`planners.pickRandomExit`) takes a secret exit only once found | D18 classes planner moves as unscripted; the planner names the exit to `flee`, which honours it (D30) | Restore the unfiltered pick |
| D32 | (Task 7) The forager's territory roam (`npcWanderTerritoryNeighbor`, behaviour tree `wander_territory`) is unscripted: the territory is authored, the pick among its neighbours is random, so a secret exit only once found | Ruling 13: a random roam is the NPC's own choice | Move it to the scripted rows of D18 |
| D33 | (Task 7) A planner's path remembers it is found-only: `PathQueue.SetPath(path, foundOnly)` stores the flag `pathto found` set, and `hooks.advanceMobPath` re-paths a displaced mob as `pathto found <waypoints>`; a scripted path re-paths plain | A displaced errand would otherwise come back as a scripted route and take every secret exit | Drop the flag (a displaced errand then takes every secret exit) |
| D34 | (Task 7) NPC loops over a room's exits (`wanderCandidates`, `pickRetreatExit`, `pickRandomExit`, the forager roam) range over `Room.PermanentExitsKnownTo(char)`, an iterator over `permanentExits` filtered by `exitKnown`: `ExitKnownTo`'s rule, mutators resolved once per call, not once per exit. `pickWanderExit` resolves a picked candidate with `GetExitInfo` | One rule (D6) without a per-exit `ActiveMutators` call on the wander hot path | Inline `ex.Secret && !KnowsSecretExit` over `room.Exits` (cheaper, but disagrees with `ExitKnownTo` when a mutator exit shadows a room exit) |
| D35 | (Task 5c) The NPC archer (`actTryFire`) picks its line of fire with `Room.FindKnownExitTo`: a secret exit only once found | Ruling 13: the archer chooses the shot; and `actions.ExecuteFire` already resolves a mob's exit word with `FindKnownExitByName`, so an unfound secret exit was a shot that misfired | Restore `FindExitTo` |
| D36 | (Task 5c) `FindKnownExitTo` answers a temporary exit by its map key, not its `Title` (amends D29) | Every caller puts the answer in a command (`go`, `shoot`), and those match keys: a rift titled "x-7 rift" or a coloured portal title matched nothing, so D29's walkers and the archer could not use such an exit | Return `Title` again (the walkers and the archer then fail at titled temporary exits) |
| D37 | (Task 5c) Ruling 15 also covers lines the review did not list: disarming a secret exit's trap (`defuse`), "tries to pick the <exit> lock", and the far room's "is looking into the room from the <exit>" for a peer and the `arrival_known_*` line for a shot (the far side teaches the exit there leading back, as ruling 14's arrival side does). A shot's target, left out of the arrival line, does not learn | Each is a seen line naming the exit because someone used it | Drop the hook at that line |
| D38 | (Task 5c) Ruling 16 also covers a shout next door: "Someone shouts from nearby, ..." for a listener who has not found the secret exit, and the GMCP Chat sender "Someone (nearby)" | The same heard sound through the same exit; leaving the name there would undo the rule | Send the exit name in the shout lines |
| D39 | (Task 5c) A mob's "tries to go the <exit> exit, but it's locked." is sent at once through `actions.SendSeen` instead of a queued `emote` command | The line and the learning must share one sight decision; a queued emote is judged a turn later | Queue the emote again and teach when it runs (needs a hook in the emote path) |
| D40 | (Task 5c fix round) `Room.FindExitTo` itself becomes ranked for every caller, on one ranking `Room.ExitsTo` (plain room or mutator exit, then temporary, then secret, A to Z by key; a temporary exit prints its Title, an empty Title its key). `FindKnownExitTo` and `actions.FollowExitTo` pick from the same list. No caller depended on the random pick: the line sites want the plain exit; go.go and mobcommands go.go read the far side's lock (a plain exit back is the door walked through); `hooks` asks only `!= ""`; the two aicompanion callers refuse a secret exit or `go` through it | One ranking; a random map walk could name, and teach, a secret exit while a plain one led back | Keep `FindExitTo` random and call a ranked twin at the line sites only |
| D41 | (Task 5c fix round) `FindExitTo` resolves room exits through `permanentExits`, so an active mutator exit takes the place of the room exit of its name, as `GetExitInfo` does; before, it returned the shadowed room exit's name | One mutator rule everywhere (D34) | Return to the raw `r.Exits` walk |
| D42 | (Task 5c fix round) `SendTextToExitsExcluding` walks `ForEachAdjacentRoom`: one line per neighbouring room naming `FindExitTo`'s exit back, and neighbours reached by a mutator exit, or answering through a temporary or mutator exit back, now hear too (before, only room exits back were read) | One adjacency walk for every heard line, shout's included | Restore the per-exit loop over `tgtRoom.Exits`, deduplicated |
| D43 | (Task 5c fix round) The heard-line race is NOT fixed yet: `events.Message` has no recipient allow-list (fields `UserId`, `ExcludeUserIds`, `RoomId`, `Text`, `IsQuiet`, `IsCommunication`, `Reply`). Proposed smallest safe addition, awaiting a yes: an `OnlyUserIds []int` field honoured in `hooks.Message_SendMessage`'s RoomId branch (skip any player not in it when it is non-empty), set on the named line only | Per-user `UserId` messages would skip the IsQuiet hearing filter and the "still in the room" check; an engine-wide message field is a change the owner should see first | n/a |
| D44 | (Task 5c fix round) `mobcommands.Fire`'s narration moves into `sendMobShootMessages(mob, room, result)` so its ruling 15 hooks can be tested with a fixed `FireResult`; `Fire` keeps the reveal, retaliation and counter in the same order | A shot cannot be driven deterministically through `ExecuteFire` | Inline it again and test through `Fire` with a seeded weapon |

---

**Added during the build (2026-11-22):** D26: shadowing is following:
a player shadower passes a secret exit and learns it (ruling 2); a mob shadower
learns it if charmed by the target, otherwise the shadow ends quietly
(ruling 13). D27: a mob following a mob leader learns the exit, like packmates
(D17). D28: ruling 14 applies only to onlookers who see the mover, on both the
leave and the arrive side (Task 5b).

## File map

**New files**

| Path | Responsibility |
|---|---|
| `internal/rooms/known_exits.go` | `ExitKind`, `KnownExit`, `KnownExitList`, `KnownExits`, `ExitKindOf`, `Room.ExitKnownTo`, `Room.FindKnownExitTo`, `Room.permanentExits` |
| `internal/rooms/known_exits_test.go` | Tests for the above, `FindKnownExitByName`, `GetRandomKnownExit`, `GetDetails` exits |
| `internal/characters/secret_exit_test.go` | `KnowsSecretExit`, `LearnSecretExit` |
| `internal/actions/follow_secret.go` | `LearnSecretExitFollowing` |
| `internal/actions/follow_secret_test.go` | its test |
| `internal/actions/look_secret_exit_test.go` | `ResolveLook` at an unfound secret exit |
| `internal/usercommands/go_secret_exit_test.go` | player path, followers, dark marks, `exits` in the dark |
| `internal/usercommands/button_roundtrip_test.go` | guard 1 (round trip) for movement ids |
| `internal/guards/exit_lookup_guard_test.go` | every unfiltered exit lookup is registered with a reason |
| `internal/buttons/context.md`, `buttons.go`, `darkmarks.go`, `buttons_test.go`, `darkmarks_test.go` | movement ids, press resolution, dark marks |
| `internal/events/roomexits.go` | `RoomExitsChanged` |
| `internal/refusal/button.go`, `internal/refusal/button_test.go` | `StaleButtonLine`, `StaleButton` |
| `internal/mobs/pathto.go` | `PathtoFoundOnly` |
| `internal/mobcommands/wander_secret_test.go`, `internal/mobcommands/pathto_found_test.go` | NPC wandering and errand routes |
| `internal/behaviortree/secret_exit_route_test.go` | `unscriptedRoute`, `pickRetreatExit` |
| `modules/follow/follow_secret_test.go` | followers learn or stop |
| `modules/gmcp/gmcp.RoomExits_test.go` | `Room.Info` exits, ids, dark, marks |
| `modules/gmcp/gmcp.RoomExitsWatch.go`, `modules/gmcp/gmcp.RoomExitsWatch_test.go` | the watcher |
| `modules/gmcp/gmcp.ButtonPress.go`, `modules/gmcp/gmcp.ButtonPress_test.go` | the press handler |
| `tools/playtest/scenarios/781-secret-exits.yaml`, `tools/playtest/goals/scenarios/781-secret-exits/prober.yaml` | adversarial seat |

**Modified files** (task in brackets): `internal/characters/character.go` [1]; `internal/mobs/mobs.go`, `internal/mobs/spawn_template_isolation_test.go` [1]; `internal/actions/search.go` [1, 3]; `internal/rooms/rooms.go` [2]; `internal/rooms/roomdetails.go` [3]; `internal/usercommands/exits.go` [3]; `internal/actions/go.go`, `internal/actions/look.go`, `internal/actions/combat_fire.go`, `internal/actions/defuse.go`, `internal/usercommands/usercommands.go`, `internal/usercommands/go.go`, `lock.go`, `unlock.go`, `picklock.go`, `shoot.go` [4]; `internal/usercommands/go.go`, `modules/follow/follow.go` [5]; `internal/mapper/mapper.nearest.go`, `mapper.crosszone.go`, `mapper.path.go`, `mapper.snapshot.go`, `mapper.go`, the mapper tests in F24, `mapper.secret_guidance_test.go`, `modules/gmcp/gmcp.Zone.go`, `modules/gmcp/gmcp.Build.go`, `internal/questengine/destination.go`, `internal/questengine/destination_secret_test.go` [6]; `internal/mobcommands/wander.go`, `wander_filter_test.go`, `pathto.go`, `internal/behaviortree/actions_goal.go`, `actions_mob.go`, `actions_archer.go`, `internal/hooks/NewRound_DoCombat_unified.go` [7]; `internal/usercommands/go.go` [9]; `modules/gmcp/gmcp.Room.go`, `gmcp_wire_freeze_test.go` [10]; `modules/gmcp/gmcp.Room.go` [11]; `modules/gmcp/gmcp.go`, `gmcp.CharOp.go` [12]; `_datafiles/html/public/static/js/mapwhere.js`, `_datafiles/html/public/webclient-pure.html`, `tools/jstest/mapwhere.test.js` [13]; `docs/README.md`, `docs/PATCH_NOTES.md` [14]; every touched package's `context.md` [its task].

---

## Task list

1. The secret-exit predicate (`KnowsSecretExit`, `LearnSecretExit`) and per-instance mob discoveries
2. `rooms.KnownExits` and the known-exit filters
3. The exits line reads `KnownExits`; `exits` in the dark
4. Every player path that names an exit uses the known lookup, plus the exit lookup guard
5. Followers learn the secret exit their leader takes
6. The maps and quest guidance use the discovery rule; `GetPathKnown`
7. NPCs: unscripted moves take only found secret exits
8. `internal/buttons`, `events.RoomExitsChanged`, `refusal.StaleButton`
9. Dark marks recorded from typed and tapped tries
10. `Room.Info` built from `KnownExits`, with kinds, ids, dark and marks
11. The `Room.Info` watcher
12. The press handler (`Char.Button.Press`) and guard 1
13. Client: the map does not re-centre when the room is unchanged
14. Docs: README, PATCH_NOTES, context.md sweep
15. Full gate and boot checks
16. MEASUREMENT: `Room.Info` lag (and the Conditions lag report)
17. ADVERSARIAL PLAYTEST: one seat against secret exits and dark bumps
18. PR

---

### Task 1: The secret-exit predicate and per-instance mob discoveries

**Files:**
- Modify: `internal/characters/character.go` (after `SecretExitDiscoveryKey`, line 675)
- Create: `internal/characters/secret_exit_test.go`
- Modify: `internal/mobs/mobs.go` (spawn copy block, after the `Adjectives` copy, about line 596)
- Modify: `internal/mobs/spawn_template_isolation_test.go` (append a test)
- Modify: `internal/characters/context.md`, `internal/mobs/context.md`

- [ ] **Step 1: Write the failing tests**

Create `internal/characters/secret_exit_test.go`:

```go
package characters

import "testing"

// #781, owner rulings 1, 2 and 13: a secret exit is known once found, and
// KnowsSecretExit is the one rule every exit list and every mover asks. It
// reads the same discovery record `search` writes.
func TestKnowsSecretExit_OnlyTheExitFoundInTheRoomItWasFound(t *testing.T) {
	c := &Character{}
	if c.KnowsSecretExit(10, "west") {
		t.Fatal("a fresh character knows no secret exit")
	}
	c.LearnSecretExit(10, "west")
	if !c.KnowsSecretExit(10, "west") {
		t.Fatal("a learned secret exit must be known")
	}
	if !c.HasDiscovery(10, SecretExitDiscoveryKey("west")) {
		t.Error("LearnSecretExit must write the record search writes")
	}
	if c.KnowsSecretExit(10, "north") {
		t.Error("learning west must not reveal north")
	}
	if c.KnowsSecretExit(11, "west") {
		t.Error("learning west in room 10 must not reveal west in room 11")
	}
	c.LearnSecretExit(10, "west")
	if got := len(c.Discoveries[10]); got != 1 {
		t.Errorf("learning twice stored %d records, want 1", got)
	}
}

func TestKnowsSecretExit_NilCharacterKnowsNone(t *testing.T) {
	var c *Character
	if c.KnowsSecretExit(10, "west") {
		t.Error("a nil character knows no secret exit")
	}
}
```

Append to `internal/mobs/spawn_template_isolation_test.go`:

```go
// #781: a mob instance learns secret exits (a search find, a leader followed).
// Spawning copies the template with `mob := *m`, so an authored discoveries
// map would be shared by every spawn and one mob's find would teach them all.
func TestSpawn_SecretExitDiscoveriesArePerInstance(t *testing.T) {
	cleanup := seedTemplateIsolationFixture(t)
	defer cleanup()

	template := mobs[stiTemplateId]
	template.Character.Discoveries = map[int][]string{5: {characters.SecretExitDiscoveryKey("west")}}

	first := NewMobByIdFresh(stiTemplateId, 1)
	if first == nil {
		t.Fatal("NewMobByIdFresh returned nil for the first spawn")
	}
	defer DestroyInstance(first.InstanceId)
	second := NewMobByIdFresh(stiTemplateId, 1)
	if second == nil {
		t.Fatal("NewMobByIdFresh returned nil for the second spawn")
	}
	defer DestroyInstance(second.InstanceId)

	if !second.Character.KnowsSecretExit(5, "west") {
		t.Fatal("precondition: an authored discovery reaches every spawn")
	}
	first.Character.LearnSecretExit(5, "north")
	if second.Character.KnowsSecretExit(5, "north") {
		t.Error("the second spawn learned what the first found: the discoveries map is shared")
	}
	if got := len(template.Character.Discoveries[5]); got != 1 {
		t.Errorf("template discoveries for room 5 = %d, want 1 (polluted by a spawn)", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/characters/ ./internal/mobs/ -run 'TestKnowsSecretExit|TestSpawn_SecretExitDiscoveriesArePerInstance' -count=1`
Expected: FAIL to compile, `c.KnowsSecretExit undefined (type *Character has no field or method KnowsSecretExit)`.

- [ ] **Step 3: Write the implementation**

In `internal/characters/character.go`, directly after `func SecretExitDiscoveryKey` (line 673-675), add:

```go
// KnowsSecretExit reports whether c has found the secret exit exitName of
// roomId: `search` found it (actions/search.go), or c moved through it behind
// a leader it follows (actions.LearnSecretExitFollowing). It is the one
// secret-exit rule (#781, owner rulings 1, 2 and 13): rooms.KnownExits, every
// player path that names an exit, the maps, quest guidance and an NPC's own
// unscripted moves all ask it. A nil character knows none.
func (c *Character) KnowsSecretExit(roomId int, exitName string) bool {
	if c == nil {
		return false
	}
	return c.HasDiscovery(roomId, SecretExitDiscoveryKey(exitName))
}

// LearnSecretExit records that c knows the secret exit exitName of roomId, in
// the record `search` writes. Recording it twice stores it once.
func (c *Character) LearnSecretExit(roomId int, exitName string) {
	c.AddDiscovery(roomId, SecretExitDiscoveryKey(exitName))
}
```

In `internal/mobs/mobs.go`, directly after the `Adjectives` copy block (the one ending `mob.Character.Adjectives = adjCopy` and `}`), add:

```go
		// Discoveries is written per instance: a search find, and since #781 a
		// secret exit learned behind a leader. An authored discoveries: map
		// would otherwise be shared by every spawn of the template, so one
		// mob's find would teach them all.
		if mob.Character.Discoveries != nil {
			discCopy := make(map[int][]string, len(mob.Character.Discoveries))
			for roomId, keys := range mob.Character.Discoveries {
				discCopy[roomId] = append([]string(nil), keys...)
			}
			mob.Character.Discoveries = discCopy
		}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/characters/ ./internal/mobs/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok` for each package.

- [ ] **Step 5: Prove the mob test can fail**

Comment out the new `if mob.Character.Discoveries != nil { ... }` block in `mobs.go`, run `go test ./internal/mobs/ -run TestSpawn_SecretExitDiscoveriesArePerInstance -count=1`, and confirm it FAILS with "the second spawn learned what the first found". Restore the block; rerun; PASS.

- [ ] **Step 6: Update context.md**

`internal/characters/context.md`: in the section that documents `Discoveries` / `SecretExitDiscoveryKey` (search the file for `SecretExitDiscoveryKey`; if absent, add a `## Secret exits (#781)` section at the end), add:

```markdown
`KnowsSecretExit(roomId, exitName)` is the one secret-exit rule (#781,
owner rulings 1, 2 and 13): true once the character holds the discovery
`SecretExitDiscoveryKey(exitName)` for that room. `search` writes it, and
`LearnSecretExit` writes it for a follower moving behind its leader. A nil
character knows none. `rooms.KnownExits` and every exit filter call it.
```

`internal/mobs/context.md`: in the spawn / shallow-copy section (search for `Mutations` or `mob := *m`), add one bullet: `Character.Discoveries` is deep-copied at spawn (#781), so a mob's search find or a secret exit learned behind its leader stays with that instance. Discoveries are not saved in `MobInstanceData`; a fresh spawn has found nothing.

- [ ] **Step 7: Commit**

```bash
git add internal/characters/character.go internal/characters/secret_exit_test.go internal/characters/context.md internal/mobs/mobs.go internal/mobs/spawn_template_isolation_test.go internal/mobs/context.md
git commit -F - <<'EOF'
feat(characters): one secret-exit rule, KnowsSecretExit (#781)

KnowsSecretExit and LearnSecretExit read and write the discovery search
records. Mob templates' discoveries are deep-copied at spawn so one
instance's find stays its own.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: `rooms.KnownExits` and the known-exit filters

**Files:**
- Create: `internal/rooms/known_exits.go`
- Create: `internal/rooms/known_exits_test.go`
- Modify: `internal/rooms/rooms.go` (`FindExitByName` 2677-2757, `GetRandomExitWhere` 1539-1588)
- Modify: `internal/rooms/context.md`

- [ ] **Step 1: Write the failing tests**

Create `internal/rooms/known_exits_test.go`:

```go
package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// knownExitsRoom is seedRegistry's room 1 (north, south, east locked at 10,
// west secret) made a sky-less cave lit by a lamp, with a viewer who has found
// nothing. Pass lit=false for a pitch-dark room.
func knownExitsRoom(t *testing.T, lit bool) (*Room, *characters.Character) {
	t.Helper()
	t.Cleanup(seedRegistry())
	t.Cleanup(SeedBiomesForTest(map[string]*BiomeInfo{
		"cave":    {BiomeId: "cave", SkyLight: SkyLightPtr(0.0)},
		"city":    {BiomeId: "city"},
		"default": {BiomeId: "default"},
	}))
	r := roomManager.rooms[1]
	r.Biome = "cave"
	if lit {
		r.Lamp = LampPtr(60)
	}
	viewer := users.NewTestUser(7801, "knower", "Knower", 97801)
	want := messaging.SightFull
	if !lit {
		want = messaging.SightNone
	}
	if got := messaging.ParticipantSight(viewer.Character, r); got != want {
		t.Fatalf("fixture premise: sight %v, want %v", got, want)
	}
	return r, viewer.Character
}

func knownNames(l KnownExitList) []string {
	out := []string{}
	for _, ke := range l.Exits {
		out = append(out, ke.Name)
	}
	return out
}

func knownExit(t *testing.T, l KnownExitList, name string) KnownExit {
	t.Helper()
	for _, ke := range l.Exits {
		if ke.Name == name {
			return ke
		}
	}
	t.Fatalf("exit %q not in %v", name, knownNames(l))
	return KnownExit{}
}

// Ruling 1: a secret exit is known only once found.
func TestKnownExits_SecretExitUnknownUntilFound(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	got := KnownExits(r, c)
	if got.Dark {
		t.Fatal("a lit room must not read dark")
	}
	if want := []string{"north", "east", "south"}; !equalStrings(knownNames(got), want) {
		t.Fatalf("known exits %v, want %v (west is an unfound secret)", knownNames(got), want)
	}
	c.LearnSecretExit(1, "west")
	if want := []string{"north", "east", "south", "west"}; !equalStrings(knownNames(KnownExits(r, c)), want) {
		t.Fatalf("after the find: %v, want %v", knownNames(KnownExits(r, c)), want)
	}
}

// Ruling 3: locked means locked now, not "has a lock".
func TestKnownExits_LockedNowVersusHasALock(t *testing.T) {
	util.SetRoundCountForTest(1000)
	t.Cleanup(util.ResetRoundCountForTest)
	r, c := knownExitsRoom(t, true)
	east := knownExit(t, KnownExits(r, c), "east")
	if !east.LockedNow || !east.Exit.HasLock() {
		t.Fatalf("east: lockedNow %v hasLock %v, want both true", east.LockedNow, east.Exit.HasLock())
	}
	r.SetExitLock("east", false)
	east = knownExit(t, KnownExits(r, c), "east")
	if east.LockedNow {
		t.Error("an unlocked door must not read locked now")
	}
	if !east.Exit.HasLock() {
		t.Error("an unlocked door still has its lock")
	}
	if knownExit(t, KnownExits(r, c), "north").LockedNow {
		t.Error("a door with no lock is never locked")
	}
}

// Temporary and mutator exits are listed, each with its kind.
func TestKnownExits_TemporaryAndMutatorExitsAreListed(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
		MutatorId: "test-known-bridge",
		Exits:     map[string]exit.RoomExit{"bridge": {RoomId: 2}, "northeast": {RoomId: 3}},
	}))
	if !GetZoneConfig("TestZone").Mutators.Add("test-known-bridge") {
		t.Fatal("fixture premise: the mutator must attach")
	}
	if !r.AddTemporaryExit("gangplank", exit.TemporaryRoomExit{RoomId: 4}) {
		t.Fatal("fixture premise: the temporary exit must attach")
	}
	got := KnownExits(r, c)
	want := []string{"north", "east", "south", "northeast", "bridge", "gangplank"}
	if !equalStrings(knownNames(got), want) {
		t.Fatalf("known exits %v, want %v", knownNames(got), want)
	}
	for name, kind := range map[string]ExitKind{
		"north": ExitCompass, "northeast": ExitDiagonal, "bridge": ExitNamed, "gangplank": ExitTemporary,
	} {
		if got := knownExit(t, got, name).Kind; got != kind {
			t.Errorf("%s kind %q, want %q", name, got, kind)
		}
	}
	if gp := knownExit(t, got, "gangplank"); gp.RoomId != 4 || gp.Temp.Title != "gangplank" {
		t.Errorf("gangplank: room %d title %q, want 4 and its title", gp.RoomId, gp.Temp.Title)
	}
}

// At no sight nothing is listed and the list says dark.
func TestKnownExits_DarkListsNothing(t *testing.T) {
	r, c := knownExitsRoom(t, false)
	c.LearnSecretExit(1, "west")
	got := KnownExits(r, c)
	if !got.Dark {
		t.Fatal("a pitch-dark room must read dark")
	}
	if len(got.Exits) != 0 {
		t.Fatalf("the dark listed %v", knownNames(got))
	}
}

func TestExitKindOf(t *testing.T) {
	for name, want := range map[string]ExitKind{
		"north": ExitCompass, "west": ExitCompass, "up": ExitVertical, "down": ExitVertical,
		"southwest": ExitDiagonal, "out": ExitNamed, "crack": ExitNamed,
	} {
		if got := ExitKindOf(name, false); got != want {
			t.Errorf("ExitKindOf(%q) = %q, want %q", name, got, want)
		}
	}
	if got := ExitKindOf("north", true); got != ExitTemporary {
		t.Errorf("a temporary exit is temporary whatever its name, got %q", got)
	}
}

// A guess at an unfound secret exit matches nothing: by its name, by its alias
// and by a prefix. The unfiltered lookup still finds it (mob moves use it).
func TestFindKnownExitByName_AnUnfoundSecretExitMatchesNothing(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	for _, guess := range []string{"west", "w", "wes"} {
		if name, id := r.FindKnownExitByName(c, guess); name != "" || id != 0 {
			t.Errorf("guess %q found %q (%d) before the find", guess, name, id)
		}
	}
	if name, _ := r.FindExitByName("west"); name != "west" {
		t.Fatalf("the unfiltered lookup must still find west, got %q", name)
	}
	if name, _ := r.FindKnownExitByName(c, "north"); name != "north" {
		t.Errorf("an ordinary exit must be found, got %q", name)
	}
	c.LearnSecretExit(1, "west")
	if name, id := r.FindKnownExitByName(c, "west"); name != "west" || id != 5 {
		t.Errorf("after the find: %q (%d), want west (5)", name, id)
	}
	if name, _ := r.FindKnownExitByName(nil, "west"); name != "" {
		t.Errorf("a nil character knows no secret exit, found %q", name)
	}
}

func TestExitKnownTo(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	if !r.ExitKnownTo(c, "north") || r.ExitKnownTo(c, "west") || r.ExitKnownTo(c, "nowhere") {
		t.Fatal("north known, west (unfound secret) and nowhere (no exit) not known")
	}
	c.LearnSecretExit(1, "west")
	if !r.ExitKnownTo(c, "west") {
		t.Error("a found secret exit is known")
	}
}

// FindKnownExitTo: the way to a room, among the exits the mover knows.
func TestFindKnownExitTo(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	if got := r.FindKnownExitTo(c, 5); got != "" {
		t.Fatalf("the only way to room 5 is an unfound secret exit, got %q", got)
	}
	if got := r.FindExitTo(5); got != "west" {
		t.Fatalf("the unfiltered FindExitTo must still answer west, got %q", got)
	}
	c.LearnSecretExit(1, "west")
	if got := r.FindKnownExitTo(c, 5); got != "west" {
		t.Errorf("after the find: %q, want west", got)
	}
}

// Ruling 13 (an NPC's own wandering): a random exit may be a secret exit the
// mover found, never one it has not. Locked exits stay out, as before.
func TestGetRandomKnownExit(t *testing.T) {
	r := &Room{RoomId: 77, Exits: map[string]exit.RoomExit{
		"west": {RoomId: 5, Secret: true},
		"east": {RoomId: 6, Lock: gamelock.Lock{Difficulty: 10}},
	}}
	c := &characters.Character{}
	for i := 0; i < 50; i++ {
		if name, _ := r.GetRandomKnownExit(c); name != "" {
			t.Fatalf("picked %q: west is unfound and east is locked", name)
		}
	}
	c.LearnSecretExit(77, "west")
	for i := 0; i < 50; i++ {
		if name, id := r.GetRandomKnownExit(c); name != "west" || id != 5 {
			t.Fatalf("picked %q (%d), want the found west (5)", name, id)
		}
	}
	if name, _ := r.GetRandomExit(); name != "" {
		t.Errorf("GetRandomExit still skips every secret exit, picked %q", name)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/rooms/ -run 'TestKnownExits|TestExitKindOf|TestFindKnownExitByName|TestExitKnownTo|TestFindKnownExitTo|TestGetRandomKnownExit' -count=1`
Expected: FAIL to compile, `undefined: KnownExits` (and the other new names).

- [ ] **Step 3: Write `internal/rooms/known_exits.go`**

```go
package rooms

import (
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// ExitKind sorts an exit for the touch controls (#781): the six directions of
// the inverted T are compass and vertical; diagonals, named and temporary
// exits sit behind the other-exits button.
type ExitKind string

const (
	ExitCompass   ExitKind = "compass"   // north, south, east, west
	ExitVertical  ExitKind = "vertical"  // up, down
	ExitDiagonal  ExitKind = "diagonal"  // northeast, southeast, southwest, northwest
	ExitNamed     ExitKind = "named"     // any other permanent exit: out, shrine, crack
	ExitTemporary ExitKind = "temporary" // a gangplank, a return portal, a tree or mob portal
)

// exitOrder is the order KnownExits lists exits in: the six T directions,
// then the diagonals clockwise from northeast (spec 2b, other exits). Every
// other exit follows, A to Z.
var exitOrder = map[string]int{
	"north": 0, "east": 1, "south": 2, "west": 3, "up": 4, "down": 5,
	"northeast": 6, "southeast": 7, "southwest": 8, "northwest": 9,
}

// ExitKindOf is the kind of the exit named exitName; temporary marks an
// ExitsTemp entry, which is temporary whatever its name.
func ExitKindOf(exitName string, temporary bool) ExitKind {
	if temporary {
		return ExitTemporary
	}
	switch exitName {
	case "north", "south", "east", "west":
		return ExitCompass
	case "up", "down":
		return ExitVertical
	case "northeast", "southeast", "southwest", "northwest":
		return ExitDiagonal
	}
	return ExitNamed
}

// KnownExit is one exit a character knows in a room.
type KnownExit struct {
	Name      string        // what the player types: the Exits, mutator or ExitsTemp key
	Kind      ExitKind      // see ExitKind
	LockedNow bool          // Lock.IsLocked() now, not merely HasLock (owner ruling 3)
	RoomId    int           // the destination, for the server's own use
	Exit      exit.RoomExit // the room or mutator exit; zero for a temporary exit
	Temp      exit.TemporaryRoomExit
}

// KnownExitList is what a character knows of a room's exits. Dark is true
// when the character sees nothing there; then Exits is empty.
type KnownExitList struct {
	Dark  bool
	Exits []KnownExit
}

// KnownExits is the one answer to "which exits of r does c know" (#781,
// owner ruling 6). Every lister reads it: the exits line (GetDetails),
// Room.Info and the touch buttons. It covers r.Exits, active mutator exits (a
// mutator's exit takes the place of a room exit of the same name, as in
// GetExitInfo) and ExitsTemp. A secret exit is listed once c found it
// (Character.KnowsSecretExit, ruling 1). At no sight (messaging.
// ParticipantSight) the list is dark and empty. A nil c sees fully and knows
// no secret exit. Exits come in exitOrder, then A to Z.
func KnownExits(r *Room, c *characters.Character) KnownExitList {
	list := KnownExitList{Exits: []KnownExit{}}
	if messaging.ParticipantSight(c, r) == messaging.SightNone {
		list.Dark = true
		return list
	}
	for name, ex := range r.permanentExits() {
		if ex.Secret && !c.KnowsSecretExit(r.RoomId, name) {
			continue
		}
		list.Exits = append(list.Exits, KnownExit{
			Name:      name,
			Kind:      ExitKindOf(name, false),
			LockedNow: ex.Lock.IsLocked(),
			RoomId:    ex.RoomId,
			Exit:      ex,
		})
	}
	for name, t := range r.ExitsTemp {
		list.Exits = append(list.Exits, KnownExit{Name: name, Kind: ExitTemporary, RoomId: t.RoomId, Temp: t})
	}
	sort.SliceStable(list.Exits, func(i, j int) bool { return knownExitLess(list.Exits[i], list.Exits[j]) })
	return list
}

func knownExitLess(a, b KnownExit) bool {
	ra, rb := knownExitRank(a), knownExitRank(b)
	if ra != rb {
		return ra < rb
	}
	la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name)
	if la != lb {
		return la < lb
	}
	return a.Kind != ExitTemporary && b.Kind == ExitTemporary
}

func knownExitRank(k KnownExit) int {
	if k.Kind != ExitTemporary {
		if rank, ok := exitOrder[k.Name]; ok {
			return rank
		}
	}
	return len(exitOrder)
}

// permanentExits is every room and mutator exit of r by name. The first
// active mutator holding a name takes the place of the room exit and of later
// mutators, GetExitInfo's rule.
func (r *Room) permanentExits() map[string]exit.RoomExit {
	all := make(map[string]exit.RoomExit, len(r.Exits))
	for name, ex := range r.Exits {
		all[name] = ex
	}
	fromMutator := map[string]bool{}
	for mut := range r.ActiveMutators {
		spec := mut.GetSpec()
		if spec == nil {
			continue
		}
		for name, ex := range spec.Exits {
			if fromMutator[name] {
				continue
			}
			fromMutator[name] = true
			all[name] = ex
		}
	}
	return all
}

// ExitKnownTo reports whether char knows the exit exitName of r: any
// temporary exit, any room or mutator exit that is not secret, and a secret
// one once found (Character.KnowsSecretExit). No such exit: false. NPC choice
// points ask it (owner ruling 13).
func (r *Room) ExitKnownTo(char *characters.Character, exitName string) bool {
	if ex, ok := r.GetExitInfo(exitName); ok {
		return !ex.Secret || char.KnowsSecretExit(r.RoomId, exitName)
	}
	_, ok := r.ExitsTemp[exitName]
	return ok
}

// FindKnownExitTo is FindExitTo among the exits char knows (ExitKnownTo): the
// exit of r that leads to roomId, or "" when the only way there is a secret
// exit char has not found. An NPC heading for a room on its own uses it
// (owner ruling 13).
func (r *Room) FindKnownExitTo(char *characters.Character, roomId int) string {
	for exitName, ex := range r.Exits {
		if ex.RoomId == roomId && r.ExitKnownTo(char, exitName) {
			return exitName
		}
	}
	for _, ex := range r.ExitsTemp {
		if ex.RoomId == roomId {
			return ex.Title
		}
	}
	for mut := range r.ActiveMutators {
		spec := mut.GetSpec()
		if spec == nil {
			continue
		}
		for exitName, ex := range spec.Exits {
			if ex.RoomId == roomId && r.ExitKnownTo(char, exitName) {
				return exitName
			}
		}
	}
	return ""
}
```

- [ ] **Step 4: Refactor `FindExitByName` and add `FindKnownExitByName` in `internal/rooms/rooms.go`**

Replace the whole of `func (r *Room) FindExitByName(...)` (lines 2677-2757) with:

```go
func (r *Room) FindExitByName(exitNameSearch string) (exitName string, exitRoomId int) {
	return r.findExitByName(exitNameSearch, func(string, exit.RoomExit) bool { return true })
}

// FindKnownExitByName is FindExitByName limited to the exits char knows
// (ExitKnownTo): a secret exit char has not found matches no word, by name,
// alias or prefix, as if it were not there, so a guess at it answers exactly
// as no exit does (#781, owner ruling 1). Every player path that names an
// exit uses it (internal/guards/exit_lookup_guard_test.go). A nil char knows
// no secret exit.
func (r *Room) FindKnownExitByName(char *characters.Character, exitNameSearch string) (exitName string, exitRoomId int) {
	return r.findExitByName(exitNameSearch, func(name string, ex exit.RoomExit) bool {
		return !ex.Secret || char.KnowsSecretExit(r.RoomId, name)
	})
}

// findExitByName is FindExitByName over the room and mutator exits usable
// accepts. Temporary exits are never secret and are always searched.
func (r *Room) findExitByName(exitNameSearch string, usable func(name string, ex exit.RoomExit) bool) (exitName string, exitRoomId int) {

	// Check for direction aliases from keywords.yaml first
	fullDirection := keywords.TryDirectionAlias(exitNameSearch)
	if fullDirection != exitNameSearch {
		// A direction alias was found, check if this exact direction exists
		if exitInfo, ok := r.Exits[fullDirection]; ok && usable(fullDirection, exitInfo) {
			return fullDirection, exitInfo.RoomId
		}
		// Check temporary exits
		if tempExit, ok := r.ExitsTemp[fullDirection]; ok {
			return fullDirection, tempExit.RoomId
		}
		// Check mutator exits
		for mut := range r.ActiveMutators {
			spec := mut.GetSpec()
			if exitInfo, ok := spec.Exits[fullDirection]; ok && usable(fullDirection, exitInfo) {
				return fullDirection, exitInfo.RoomId
			}
		}
		// Direction alias used but exit doesn't exist
		return ``, 0
	}

	// Build list of all exits for fuzzy matching
	exitNames := []string{}
	for exitName, exitInfo := range r.Exits {
		if usable(exitName, exitInfo) {
			exitNames = append(exitNames, exitName)
		}
	}

	for exitName := range r.ExitsTemp {
		exitNames = append(exitNames, exitName)
	}

	mutatorExits := map[string]exit.RoomExit{}
	for mut := range r.ActiveMutators {
		spec := mut.GetSpec()
		for exitName, exitInfo := range spec.Exits {
			if !usable(exitName, exitInfo) {
				continue
			}
			mutatorExits[exitName] = exitInfo
			exitNames = append(exitNames, exitName)
		}
	}

	// Use fuzzy matching for all exits
	exactMatch, closeMatch := util.FindMatchIn(exitNameSearch, exitNames...)

	if len(exactMatch) == 0 {
		portalStr := `portal`
		if strings.HasPrefix(closeMatch, exitNameSearch) {
			exactMatch = closeMatch
		} else if strings.Contains(closeMatch, portalStr) { // If has portal in the word, lets consider a partial match on "portal"
			if exitNameSearch == portalStr {
				exactMatch = closeMatch
			} else { // partial starting match on "portal"?
				searchLen := len(exitNameSearch)
				if searchLen <= len(portalStr) {
					if portalStr[:searchLen] == exitNameSearch {
						exactMatch = closeMatch
					}
				}
			}
		}
	}
	if len(closeMatch) == 0 {
		return "", 0
	}

	if exitInfo, ok := r.Exits[exactMatch]; ok && usable(exactMatch, exitInfo) {
		return exactMatch, exitInfo.RoomId
	}

	if exitInfo, ok := r.ExitsTemp[exactMatch]; ok {
		return exitInfo.Title, exitInfo.RoomId
	}

	if exitInfo, ok := mutatorExits[exactMatch]; ok {
		return exactMatch, exitInfo.RoomId
	}

	return "", 0
}
```

`rooms.go` already imports `characters`, `exit`, `keywords`, `strings`, `util`; confirm with `go build ./internal/rooms/`.

- [ ] **Step 5: Refactor `GetRandomExitWhere` and add `GetRandomKnownExit` in `internal/rooms/rooms.go`**

Replace `func (r *Room) GetRandomExitWhere(...)` (from its doc comment at line 1539 through the closing brace that ends the function, after its final empty-string return) with:

```go
// GetRandomExitWhere is GetRandomExit restricted to the exits keep accepts
// (nil keeps every one). Secret and locked exits are excluded before keep is
// asked. Flee passes the fleeing player's quest gate check (#588).
func (r *Room) GetRandomExitWhere(keep func(exitName string, roomId int) bool) (exitName string, roomId int) {
	return r.randomExit(func(_ string, ex exit.RoomExit) bool { return !ex.Secret }, keep)
}

// GetRandomKnownExit is GetRandomExit for a mover who may take the secret
// exits it has found (#781, owner ruling 13: an NPC's own wandering). Locked
// exits stay out, as in GetRandomExit.
func (r *Room) GetRandomKnownExit(char *characters.Character) (exitName string, roomId int) {
	return r.randomExit(func(name string, ex exit.RoomExit) bool {
		return !ex.Secret || char.KnowsSecretExit(r.RoomId, name)
	}, nil)
}

// randomExit picks uniformly among the unlocked room and mutator exits allowed
// accepts and keep (nil keeps every one) accepts.
func (r *Room) randomExit(allowed func(exitName string, ex exit.RoomExit) bool, keep func(exitName string, roomId int) bool) (exitName string, roomId int) {

	allExits := map[string]int{}

	for exitName, ex := range r.Exits {
		if !allowed(exitName, ex) {
			continue
		}
		if ex.Lock.IsLocked() {
			continue
		}

		allExits[exitName] = ex.RoomId
	}

	for mut := range r.ActiveMutators {
		spec := mut.GetSpec()
		for exitName, ex := range spec.Exits {
			if !allowed(exitName, ex) {
				continue
			}
			if ex.Lock.IsLocked() {
				continue
			}
			allExits[exitName] = ex.RoomId
		}
	}

	if keep != nil {
		for exitName, roomId := range allExits {
			if !keep(exitName, roomId) {
				delete(allExits, exitName)
			}
		}
	}

	roomSelection := util.Rand(len(allExits))

	for exitName, roomId := range allExits {
		if roomSelection == 0 {
			return exitName, roomId
		}
		roomSelection--
	}

	return ``, 0
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/rooms/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok`. (`TestRoom_GetRandomExit_SkipsSecret` and the `FindExitByName` tests in `rooms_test.go` must stay green: the unfiltered paths keep their old answers.)

- [ ] **Step 7: Prove the tests can fail**

In `FindKnownExitByName`, change `return !ex.Secret || char.KnowsSecretExit(r.RoomId, name)` to `return true`. Run `go test ./internal/rooms/ -run TestFindKnownExitByName -count=1`: FAIL naming `guess "west" found "west"`. Restore. Then in `KnownExits` delete the `continue` under `if ex.Secret && ...`; run `-run TestKnownExits_SecretExitUnknownUntilFound`: FAIL. Restore. Rerun the package: PASS.

- [ ] **Step 8: Update `internal/rooms/context.md`**

Add a section:

```markdown
## Known exits (#781)

`KnownExits(room, char) KnownExitList` is the one answer to which exits a
character knows in a room (owner ruling 6). It covers `Exits`, active
mutator exits (the first active mutator with a name takes the place of the
room exit, `GetExitInfo`'s rule) and `ExitsTemp`; a secret exit is listed
once `char.KnowsSecretExit` (ruling 1). Each `KnownExit` carries `Name`,
`Kind` (`ExitCompass`, `ExitVertical`, `ExitDiagonal`, `ExitNamed`,
`ExitTemporary`, see `ExitKindOf`), `LockedNow` (`Lock.IsLocked()`, ruling
3), `RoomId` and the `Exit` or `Temp` record. At no sight it is `Dark` and
empty. Order: north, east, south, west, up, down, the diagonals clockwise
from northeast, then A to Z.

The filters on the same rule: `ExitKnownTo(char, name)`,
`FindKnownExitByName(char, search)` (every player path that names an exit;
`internal/guards/exit_lookup_guard_test.go` registers the unfiltered
`FindExitByName` callers), `FindKnownExitTo(char, roomId)` and
`GetRandomKnownExit(char)` (NPC choice points, ruling 13). The unfiltered
`FindExitByName`, `FindExitTo` and `GetRandomExit` keep their old answers for
mob moves, scripted routes and flee.
```

- [ ] **Step 9: Commit**

```bash
git add internal/rooms/known_exits.go internal/rooms/known_exits_test.go internal/rooms/rooms.go internal/rooms/context.md
git commit -F - <<'EOF'
feat(rooms): KnownExits, the one rule for the exits a player knows (#781)

KnownExits lists room, mutator and temporary exits with kind, locked now
and destination; a secret exit only once found; nothing in the dark.
ExitKnownTo, FindKnownExitByName, FindKnownExitTo and GetRandomKnownExit
filter on the same rule.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: The exits line reads `KnownExits`; `exits` in the dark

**Files:**
- Modify: `internal/rooms/roomdetails.go` (lines 363-379 and 434-438; the `maps` import)
- Modify: `internal/rooms/known_exits_test.go` (append)
- Modify: `internal/usercommands/exits.go`
- Create: `internal/usercommands/go_secret_exit_test.go` (first test; later tasks append)
- Modify: `internal/actions/search.go` (comment at lines 434-440 only)
- Modify: `internal/rooms/context.md`, `internal/usercommands/context.md`

- [ ] **Step 1: Write the failing tests**

Append to `internal/rooms/known_exits_test.go`:

```go
// The exits line lists exactly what KnownExits lists: the found secret exit
// joins it, a temporary exit sits in TemporaryExits, and the dark lists none.
func TestGetDetails_ExitsLineReadsKnownExits(t *testing.T) {
	r, c := knownExitsRoom(t, true)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{}))
	viewer := users.NewTestUser(7802, "liner", "Liner", 97802)
	viewer.Character = c
	if !r.AddTemporaryExit("gangplank", exit.TemporaryRoomExit{RoomId: 4}) {
		t.Fatal("fixture premise: the temporary exit must attach")
	}
	keys := func(d RoomTemplateDetails) map[string]bool {
		out := map[string]bool{}
		for k := range d.VisibleExits {
			out[k] = true
		}
		return out
	}
	d := GetDetails(r, viewer)
	if got := keys(d); len(got) != 3 || got["west"] {
		t.Fatalf("visible exits %v, want north, south, east (west unfound)", got)
	}
	if _, ok := d.TemporaryExits["gangplank"]; !ok {
		t.Errorf("the gangplank must be in TemporaryExits, got %v", d.TemporaryExits)
	}
	c.LearnSecretExit(1, "west")
	if got := keys(GetDetails(r, viewer)); !got["west"] {
		t.Errorf("the found west must be listed, got %v", got)
	}
	r.Lamp = nil
	dark := GetDetails(r, viewer)
	if len(dark.VisibleExits) != 0 || len(dark.TemporaryExits) != 0 {
		t.Errorf("the dark listed %v and %v", dark.VisibleExits, dark.TemporaryExits)
	}
}
```

Create `internal/usercommands/go_secret_exit_test.go`:

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// darkenRoom makes the fixture room pitch dark (biome cave, SkyLight 0, no
// lamp) until the test ends.
func darkenRoom(t *testing.T, room *rooms.Room) {
	t.Helper()
	biome, lamp := room.Biome, room.Lamp
	room.Biome, room.Lamp = "cave", nil
	t.Cleanup(func() { room.Biome, room.Lamp = biome, lamp })
}

func drainedText(u *users.UserRecord) string {
	return strings.Join(events.DrainQueuedMessagesForTest(u.UserId), "\n")
}

// #781 decision D5: in the dark the exits line would read "None", which is not
// true. `exits` answers as look does.
func TestExits_InTheDarkSaysItIsTooDark(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	darkenRoom(t, room)
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Exits("", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	got := drainedText(user)
	require.Contains(t, got, "It is too dark to see.")
	require.NotContains(t, got, "None")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/rooms/ -run TestGetDetails_ExitsLineReadsKnownExits -count=1; go test ./internal/usercommands/ -run TestExits_InTheDarkSaysItIsTooDark -count=1`
Expected: the rooms test FAILS (west listed or exits listed in the dark: today's code lists every exit in the dark); the usercommands test FAILS (no dark line).

- [ ] **Step 3: Implement**

In `internal/rooms/roomdetails.go`, replace lines 363-379:

```go
	maps.Copy(details.TemporaryExits, r.ExitsTemp)

	// Do this twice to ensure secrets are last

	for exitStr, exitInfo := range r.Exits {

		// If it's a secret room we need to make sure the player has recently been there before including it in the exits
		if exitInfo.Secret {
			if targetRm := LoadRoom(exitInfo.RoomId); targetRm != nil {
				if targetRm.HasVisited(user.UserId, VisitorUser) {
					details.VisibleExits[exitStr] = exitInfo
				}
			}
		} else {
			details.VisibleExits[exitStr] = exitInfo
		}
	}
```

with:

```go
	// The exits line lists what the viewer knows of this room's exits, the
	// one rule Room.Info, the maps and the touch buttons read too (KnownExits,
	// #781): a secret exit once found, mutator and temporary exits, and
	// nothing at all in the dark.
	for _, ke := range KnownExits(r, user.Character).Exits {
		if ke.Kind == ExitTemporary {
			details.TemporaryExits[ke.Name] = ke.Temp
			continue
		}
		details.VisibleExits[ke.Name] = ke.Exit
	}
```

and delete lines 434-438:

```go
	// assign mutator exits last so that they can overwrite normal exits
	for mut := range r.ActiveMutators {
		spec := mut.GetSpec()
		maps.Copy(details.VisibleExits, spec.Exits)
	}
```

Remove `"maps"` from the import block (no other use in the file; the compiler confirms).

Replace `internal/usercommands/exits.go`'s function body:

```go
func Exits(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// The exits line lists nothing at no sight (rooms.KnownExits, #781), and
	// "Exits: None" would not be true: answer as look does.
	if line, refused := noSightRefusal(actions.ResolveLook(actions.NewUserActorInRoom(user, room), ``).Kind); refused {
		user.SendText(messaging.CategoryRefusal, line)
		return true, nil
	}

	details := rooms.GetDetails(room, user)

	exitTxt, _ := templates.Process("descriptions/exits", details, user.UserId)
	user.SendText(messaging.CategoryRoomDescription, exitTxt)

	return true, nil
}
```

and add `"github.com/GoMudEngine/GoMud/internal/actions"` to its imports.

In `internal/actions/search.go`, replace only these comment lines (434-440):

```go
		// ⚠️ It is still REPORTED, which is where this deliberately differs from
		// the container and noun tiers that `continue` outright. A found
		// container is reachable afterwards through `get`, but a secret exit
		// stays out of the room's exit list until the player VISITS the room
		// beyond it (roomdetails.go gates on HasVisited, not on a discovery). So
		// skipping it silently would leave someone who found it and did not walk
		// through with no way to be reminded of the name. Reporting costs
```

with:

```go
		// ⚠️ It is still REPORTED, which is where this deliberately differs from
		// the container and noun tiers that `continue` outright. Since #781 the
		// discovery itself puts the exit in the room's exit list
		// (rooms.KnownExits), and the report reminds a player who searches
		// again of the name they found. Reporting costs
```

(The next line, `// nothing they have not already earned; the roll and the award are what`, stays as it is.)

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/rooms/ ./internal/usercommands/ ./internal/actions/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok`. `internal/guards/looking_screens_test.go` and `template_width_guard_test.go` render the exits template from their own fixtures and must stay green.

- [ ] **Step 5: Prove the tests can fail**

In `exits.go` change `if line, refused := ...; refused {` to `if line, refused := ...; refused && false {` (keep `line` used), run the usercommands test: FAIL. Restore. In `roomdetails.go` remove the `continue` after `details.TemporaryExits[...] = ke.Temp`: the rooms test FAILS (gangplank lands in `VisibleExits`, four keys). Restore; PASS.

- [ ] **Step 6: Update context.md**

`internal/rooms/context.md`, in the new "Known exits" section, add: `GetDetails` fills `VisibleExits` and `TemporaryExits` from `KnownExits`, so the exits line follows the same rule; at no sight both are empty.

`internal/usercommands/context.md`: where `exits` is listed (search `Exits`), add: in the dark `exits` answers with look's no-sight line (`noSightRefusal`), because the exits line lists nothing at no sight (#781).

- [ ] **Step 7: Commit**

```bash
git add internal/rooms/roomdetails.go internal/rooms/known_exits_test.go internal/rooms/context.md internal/usercommands/exits.go internal/usercommands/go_secret_exit_test.go internal/usercommands/context.md internal/actions/search.go
git commit -F - <<'EOF'
feat(rooms): the exits line reads KnownExits (#781)

GetDetails lists the viewer's known exits: a secret exit once found, not
after a visit; mutator and temporary exits; none in the dark. The exits
command answers the dark as look does instead of "Exits: None".

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Every player path that names an exit uses the known lookup, plus the exit lookup guard

**Files:**
- Modify: `internal/actions/go.go` (add `FindKnownExit`)
- Modify: `internal/usercommands/usercommands.go:416`, `:656`
- Modify: `internal/usercommands/go.go:57`
- Modify: `internal/actions/look.go:138`, `:141`
- Modify: `internal/usercommands/lock.go:31`, `unlock.go:26`, `picklock.go:63`, `shoot.go:728`
- Modify: `internal/actions/combat_fire.go:116`, `:228`; `internal/actions/defuse.go:218`
- Create: `internal/actions/look_secret_exit_test.go`
- Modify: `internal/usercommands/go_secret_exit_test.go` (append)
- Create: `internal/guards/exit_lookup_guard_test.go`
- Modify: `internal/actions/context.md`, `internal/usercommands/context.md`, `internal/guards/context.md`

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/look_secret_exit_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 1: `look north` at a secret exit the looker has not found
// reads as if no exit were there; once found it looks through it.
func TestResolveLook_AnUnfoundSecretExitIsNotAnExit(t *testing.T) {
	actor := scanLockScene(t, 90, false, false, true)
	here := rooms.LoadRoom(scanLockHereId)
	north := here.Exits["north"]
	north.Secret = true
	here.Exits["north"] = north

	res := ResolveLook(actor, "north")
	require.NotEqual(t, LookExit, res.Kind, "an unfound secret exit must not be looked through")
	require.Equal(t, "", res.ExitName)

	actor.char.LearnSecretExit(scanLockHereId, "north")
	res = ResolveLook(actor, "north")
	require.Equal(t, LookExit, res.Kind)
	require.Equal(t, "north", res.ExitName)
}
```

Append to `internal/usercommands/go_secret_exit_test.go` (add `"github.com/GoMudEngine/GoMud/internal/exit"` to its imports):

```go
// secretExit makes room's exit name secret (adding it, to room 2, when the
// room has none) until the test ends.
func secretExit(t *testing.T, room *rooms.Room, name string) {
	t.Helper()
	orig, had := room.Exits[name]
	ex := orig
	if !had {
		ex = exit.RoomExit{RoomId: 2}
	}
	ex.Secret = true
	room.Exits[name] = ex
	t.Cleanup(func() {
		if had {
			room.Exits[name] = orig
		} else {
			delete(room.Exits, name)
		}
	})
}

// Ruling 1 and the spec decision: an unfound secret exit answers a compass
// word exactly as no exit does.
func TestGo_AnUnfoundSecretExitBumpsLikeAWall(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	secretExit(t, room, "north")
	events.DrainQueuedMessagesForTest(user.UserId)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, user.Character.RoomId, "the player must not pass an unfound secret exit")
	require.Contains(t, drainedText(user), "You're bumping into walls.")
}

func TestGo_AFoundSecretExitWalks(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	secretExit(t, room, "north")
	user.Character.LearnSecretExit(1, "north")

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 2, user.Character.RoomId, "a found secret exit walks")
}

// A named secret exit answers "not recognized", as a word naming no exit does,
// and so does a prefix guess at it. Once found it is a move.
func TestTyped_AnUnfoundSecretNameIsNotRecognized(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)

	require.Equal(t, routeUnknown, fallbackRoute(user, room, "crack"), "premise: no crack exit at all")
	secretExit(t, room, "crack")
	for _, guess := range []string{"crack", "cra"} {
		require.Equal(t, routeUnknown, fallbackRoute(user, room, guess), "guess %q", guess)
		require.False(t, isKnownCommandLine(user, room, guess), "guess %q", guess)
	}
	handled, err := Go("crack", user, room, 0)
	require.False(t, handled, "go at an unfound named secret exit falls to not recognized")
	require.NoError(t, err)

	user.Character.LearnSecretExit(1, "crack")
	require.Equal(t, routeGo, fallbackRoute(user, room, "crack"))
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/actions/ -run TestResolveLook_AnUnfoundSecretExitIsNotAnExit -count=1; go test ./internal/usercommands/ -run 'TestGo_AnUnfoundSecretExitBumpsLikeAWall|TestTyped_AnUnfoundSecretNameIsNotRecognized' -count=1`
Expected: FAIL (the look resolves `LookExit`; the player walks north; `crack` routes to go).

- [ ] **Step 3: Add `actions.FindKnownExit`**

In `internal/actions/go.go`, add `"github.com/GoMudEngine/GoMud/internal/characters"` to the imports and append:

```go
// FindKnownExit is FindExit for a mover who may use only the exits they know
// (Room.FindKnownExitByName): a secret exit they have not found is not found
// (#781, owner ruling 1). Every player path that names an exit uses it; mob
// moves keep FindExit, since a scripted route takes its secret exits.
func FindKnownExit(room *rooms.Room, char *characters.Character, exitName string) FindExitResult {
	name, roomId := room.FindKnownExitByName(char, exitName)
	if name == `` {
		return FindExitResult{Found: false}
	}
	return FindExitResult{
		ExitName: name,
		RoomId:   roomId,
		Found:    true,
	}
}
```

- [ ] **Step 4: Switch every player call site (one line each)**

| File:line | Before | After |
|---|---|---|
| `internal/usercommands/usercommands.go:416` | `exitName, _ := room.FindExitByName(cmd)` | `exitName, _ := room.FindKnownExitByName(user.Character, cmd)` |
| `internal/usercommands/usercommands.go:656` | `if (room != nil && actions.FindExit(room, cmd).Found) \|\| actions.IsCompassDirection(cmd) {` | `if (room != nil && actions.FindKnownExit(room, user.Character, cmd).Found) \|\| actions.IsCompassDirection(cmd) {` |
| `internal/usercommands/go.go:57` | `exitResult := actions.FindExit(room, rest)` | `exitResult := actions.FindKnownExit(room, user.Character, rest)` |
| `internal/actions/look.go:138` | `exitName, exitRoomId := room.FindExitByName(lookAt)` | `exitName, exitRoomId := room.FindKnownExitByName(char, lookAt)` |
| `internal/actions/look.go:141` | the alias lookup `room.FindExitByName(alias)` | `room.FindKnownExitByName(char, alias)` (rest of the line unchanged) |
| `internal/usercommands/lock.go:31` | `exitName, _ := room.FindExitByName(args[0])` | `exitName, _ := room.FindKnownExitByName(characterOf(user), args[0])` |
| `internal/usercommands/unlock.go:26` | `exitName, _ := room.FindExitByName(args[0])` | `exitName, _ := room.FindKnownExitByName(characterOf(user), args[0])` |
| `internal/usercommands/picklock.go:63` | `exitName, _ := room.FindExitByName(args[0])` | `exitName, _ := room.FindKnownExitByName(characterOf(user), args[0])` |
| `internal/usercommands/shoot.go:728` | `if name, roomId := room.FindExitByName(args[len(args)-1]); name != "" {` | `if name, roomId := room.FindKnownExitByName(viewer, args[len(args)-1]); name != "" {` |
| `internal/actions/combat_fire.go:116` | `if name, roomId := room.FindExitByName(args[len(args)-1]); name != "" {` | `if name, roomId := room.FindKnownExitByName(viewer, args[len(args)-1]); name != "" {` |
| `internal/actions/combat_fire.go:228` | `if name, roomId := room.FindExitByName(args[len(args)-1]); name != "" {` | `if name, roomId := room.FindKnownExitByName(actor.GetCharacter(), args[len(args)-1]); name != "" {` |
| `internal/actions/defuse.go:218` | `exitName, _ := room.FindExitByName(noun)` | `exitName, _ := room.FindKnownExitByName(actor.GetCharacter(), noun)` |

(`char` in `ResolveLook` is `actor.GetCharacter()`, line 65; `viewer` is the `*characters.Character` parameter of `ShotSight` and `resolveShootTarget`; `actor` is `ExecuteFire`'s and `resolveDefuseLockTarget`'s `Actor`.) Each is a same-line edit, so no allowlisted line number moves.

- [ ] **Step 5: Write the guard `internal/guards/exit_lookup_guard_test.go`**

```go
package guards

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// exitLookupEntry is how many unfiltered exit lookups one function makes and
// why the unfiltered form is right there.
type exitLookupEntry struct {
	plain int
	why   string
}

const whyExitMob = "a mob's own move: a scripted route keeps its secret exits (#781 ruling 13); an NPC's unscripted choice is filtered where it is made"

// exitLookupRegistry is every non-test call in internal/ and modules/ that
// resolves an exit by name with no knower (Room.FindExitByName,
// actions.FindExit), keyed "path|function". #781, owner ruling 1: a player
// path that names an exit uses Room.FindKnownExitByName or
// actions.FindKnownExit, or a guess at a secret exit gives it away.
var exitLookupRegistry = map[string]exitLookupEntry{
	"internal/actions/go.go|FindExit":                 {plain: 1, why: "the unfiltered form itself, for mob moves"},
	"internal/actions/move.go|QuoteMobStep":           {plain: 1, why: whyExitMob},
	"internal/mobcommands/go.go|isMoveAttempt":        {plain: 1, why: whyExitMob},
	"internal/mobcommands/go.go|Go":                   {plain: 1, why: whyExitMob},
	"internal/usercommands/keyring.go|KeyRing":        {plain: 2, why: "names the door a key the player already holds opens"},
	"modules/aicompanion/loot.go|mobCompanionUnlock":  {plain: 1, why: "a companion's own unlock; its routes never take a secret exit (worldmap.go)"},
}

// exitLookupCalls adds to found, per "rel|function" of one parsed file, the
// calls that resolve an exit by name with no knower.
func exitLookupCalls(rel string, file *ast.File, found map[string]int) {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		key := rel + "|" + lookupFuncName(fd)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := ""
			switch f := call.Fun.(type) {
			case *ast.SelectorExpr:
				callee = f.Sel.Name
			case *ast.Ident:
				callee = f.Name
			}
			if callee == "FindExitByName" || callee == "FindExit" {
				found[key]++
			}
			return true
		})
	}
}

// exitLookupProblems compares found against exitLookupRegistry.
func exitLookupProblems(found map[string]int) []string {
	var problems []string
	for key, got := range found {
		want, ok := exitLookupRegistry[key]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: not in exitLookupRegistry (%d unfiltered exit lookups)", key, got))
		case got != want.plain:
			problems = append(problems, fmt.Sprintf("%s: %d unfiltered exit lookups, registry says %d", key, got, want.plain))
		}
	}
	for key, want := range exitLookupRegistry {
		if _, ok := found[key]; !ok {
			problems = append(problems, key+": in exitLookupRegistry but no unfiltered exit lookup found there (stale)")
		}
		if want.why == "" {
			problems = append(problems, key+": an unfiltered exit lookup needs a reason")
		}
	}
	sort.Strings(problems)
	return problems
}

// TestEveryPlainExitLookupIsRegistered fails when a function resolves an exit
// by name without a knower and is not registered. If you are here for a new
// player command, call Room.FindKnownExitByName(char, ...) or
// actions.FindKnownExit. A mob path or a tool that must see every exit is
// registered with the reason.
func TestEveryPlainExitLookupIsRegistered(t *testing.T) {
	found := map[string]int{}
	for _, root := range []string{"internal", "modules"} {
		fset := token.NewFileSet()
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}
			exitLookupCalls(filepath.ToSlash(path), file, found)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s (test must run from the repo root): %v", root, err)
		}
	}
	if len(found) == 0 {
		t.Fatal("no unfiltered exit lookups found at all: the walk is broken, not the code")
	}
	if problems := exitLookupProblems(found); len(problems) > 0 {
		t.Errorf("%d exit lookup problem(s):\n  %s\n\n"+
			"A player path that names an exit must use Room.FindKnownExitByName or "+
			"actions.FindKnownExit, or a guess confirms a secret exit (#781).",
			len(problems), strings.Join(problems, "\n  "))
	}
}

// The scanner sees an unfiltered lookup in an unregistered function and
// ignores the filtered one.
func TestExitLookupGuardCanFail(t *testing.T) {
	const src = `package probe
func Bad(r *rooms.Room) { r.FindExitByName("west") }
func AlsoBad(r *rooms.Room) { actions.FindExit(r, "west") }
func Good(r *rooms.Room, c *characters.Character) { r.FindKnownExitByName(c, "west"); actions.FindKnownExit(r, c, "west") }
`
	file, err := parser.ParseFile(token.NewFileSet(), "probe.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]int{}
	exitLookupCalls("internal/probe/probe.go", file, found)
	if found["internal/probe/probe.go|Bad"] != 1 || found["internal/probe/probe.go|AlsoBad"] != 1 {
		t.Fatalf("the scanner missed an unfiltered lookup: %v", found)
	}
	if _, ok := found["internal/probe/probe.go|Good"]; ok {
		t.Fatalf("the scanner counted a filtered lookup: %v", found)
	}
	joined := strings.Join(exitLookupProblems(found), "\n")
	if !strings.Contains(joined, "internal/probe/probe.go|Bad: not in exitLookupRegistry") {
		t.Errorf("an unregistered unfiltered lookup must be reported, got:\n%s", joined)
	}
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/actions/ ./internal/usercommands/ ./internal/rooms/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok` everywhere, including `TestEveryPlainExitLookupIsRegistered`.

- [ ] **Step 7: Prove the guard and the tests can fail**

Revert one call site, `usercommands/lock.go:31`, to `room.FindExitByName(args[0])`. Run `go test ./internal/guards/ -run TestEveryPlainExitLookupIsRegistered -count=1`: FAIL naming `internal/usercommands/lock.go|Lock: not in exitLookupRegistry`. Restore. Revert `usercommands/go.go:57` to `actions.FindExit(room, rest)`: `TestGo_AnUnfoundSecretExitBumpsLikeAWall` FAILS (the player reaches room 2) and the guard names `usercommands/go.go|Go`. Restore. Rerun: PASS.

- [ ] **Step 8: Update context.md**

`internal/actions/context.md`: under the movement or look section, add: `FindKnownExit(room, char, name)` is `FindExit` for a mover who knows only some exits (#781); every player path uses it, mob moves keep `FindExit`. `ResolveLook` resolves exits with `FindKnownExitByName`, so an unfound secret exit is not looked through.

`internal/usercommands/context.md`: add: every verb that names an exit (`go` and the bare exit word, `look`, `lock`, `unlock`, `picklock`, `shoot`) resolves it with `Room.FindKnownExitByName(characterOf(user), ...)`; an unfound secret exit answers "You're bumping into walls." for a compass word and "not recognized" otherwise, exactly as no exit (#781, owner ruling 1).

`internal/guards/context.md`: add a row for `exit_lookup_guard_test.go`: every unfiltered exit lookup (`FindExitByName`, `actions.FindExit`) in `internal/` and `modules/` is registered with a reason; a player path must use `FindKnownExitByName` / `FindKnownExit` (#781). `TestExitLookupGuardCanFail` proves the scanner fires.

- [ ] **Step 9: Commit**

```bash
git add internal/actions/go.go internal/actions/look.go internal/actions/combat_fire.go internal/actions/defuse.go internal/actions/look_secret_exit_test.go internal/actions/context.md internal/usercommands/usercommands.go internal/usercommands/go.go internal/usercommands/lock.go internal/usercommands/unlock.go internal/usercommands/picklock.go internal/usercommands/shoot.go internal/usercommands/go_secret_exit_test.go internal/usercommands/context.md internal/guards/exit_lookup_guard_test.go internal/guards/context.md
git commit -F - <<'EOF'
feat(usercommands): a guess at an unfound secret exit reveals nothing (#781)

go, the bare exit word, look, lock, unlock, picklock, shoot, fire and
defuse resolve exits through FindKnownExitByName: an unfound secret exit
answers exactly as no exit. A guard registers every unfiltered lookup.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: Followers learn the secret exit their leader takes

**Files:**
- Create: `internal/actions/follow_secret.go`, `internal/actions/follow_secret_test.go`
- Modify: `internal/usercommands/go.go` (party follow loop about line 376; charmed loop about line 389)
- Modify: `internal/usercommands/go_secret_exit_test.go` (append)
- Modify: `modules/follow/follow.go` (`roomChangeHandler`, lines 388-430)
- Create: `modules/follow/follow_secret_test.go`
- Modify: `internal/actions/context.md`, `modules/follow/context.md`

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/follow_secret_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 2 (and 13 for NPCs): moving through a secret exit behind
// the leader teaches it; an ordinary exit records nothing.
func TestLearnSecretExitFollowing(t *testing.T) {
	room := &rooms.Room{RoomId: 61, Exits: map[string]exit.RoomExit{
		"west":  {RoomId: 62, Secret: true},
		"north": {RoomId: 63},
	}}
	c := &characters.Character{}
	LearnSecretExitFollowing(c, room, "north")
	LearnSecretExitFollowing(c, room, "nowhere")
	require.Empty(t, c.Discoveries, "an ordinary or missing exit records nothing")
	LearnSecretExitFollowing(c, room, "west")
	require.True(t, c.KnowsSecretExit(61, "west"))
	LearnSecretExitFollowing(nil, room, "west") // must not panic
}
```

Append to `internal/usercommands/go_secret_exit_test.go` (add imports `"github.com/GoMudEngine/GoMud/internal/characters"`, `"github.com/GoMudEngine/GoMud/internal/mobs"`, `"github.com/GoMudEngine/GoMud/internal/parties"`):

```go
// Ruling 2: a party member who moves through a secret exit behind the leader
// learns it. Ruling 13: a companion moving with its owner learns it too.
func TestGo_FollowersLearnTheSecretExitTheLeaderTakes(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	secretExit(t, room, "north")
	user.Character.LearnSecretExit(1, "north")

	member := users.GetByUserId(2)
	p := parties.New(1)
	require.NotNil(t, p)
	require.True(t, p.InvitePlayer(2))
	require.True(t, p.AcceptInvite(2))
	companion := mobs.GetInstance(100)
	require.NotNil(t, companion)
	companion.Character.Charmed = characters.NewCharm(1, -1, "")
	require.False(t, member.Character.KnowsSecretExit(1, "north"), "premise")
	require.False(t, companion.Character.KnowsSecretExit(1, "north"), "premise")
	events.DrainQueuedUserInputsForTest(2)

	handled, err := Go("north", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 2, user.Character.RoomId)
	require.True(t, member.Character.KnowsSecretExit(1, "north"), "the party member walked behind the leader")
	require.Contains(t, events.DrainQueuedUserInputsForTest(2), "north", "the member's follow step is queued")
	require.True(t, companion.Character.KnowsSecretExit(1, "north"), "the companion walked behind its owner")
}
```

Create `modules/follow/follow_secret_test.go`:

```go
package follow

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// secretNorth makes the square's north exit secret and teaches it to the
// leader (Bobrick, user 2).
func secretNorth(t *testing.T, square *rooms.Room, leader *characters.Character) {
	t.Helper()
	north := square.Exits["north"]
	north.Secret = true
	square.Exits["north"] = north
	leader.LearnSecretExit(square.RoomId, "north")
}

// Ruling 2: a player following someone through a secret exit learns it.
func TestFollowSecret_APlayerFollowerLearnsTheExit(t *testing.T) {
	f, u1, u2, square, cave := followWalkScene(t, true)
	secretNorth(t, square, u2.Character)
	f.startFollow(followId{userId: 2}, followId{userId: 1}, 0)

	walkNorth(f, u2, square, cave)

	assert.Equal(t, []string{"north"}, events.DrainQueuedUserInputsForTest(1), "the follower steps after the leader")
	assert.True(t, u1.Character.KnowsSecretExit(square.RoomId, "north"), "the follower learned the exit")
}

// Ruling 13: a companion moving with its leader learns the exit; any other
// NPC following a player stops at a secret exit it has not found.
func TestFollowSecret_ACompanionLearnsAndAStrangerStops(t *testing.T) {
	f, _, u2, square, cave := followWalkScene(t, true)
	secretNorth(t, square, u2.Character)

	pet := &mobs.Mob{InstanceId: 9781}
	pet.Character.Name = "Loyal Hound"
	pet.Character.RoomId = square.RoomId
	pet.Character.Charmed = characters.NewCharm(2, -1, "")
	stray := &mobs.Mob{InstanceId: 9782}
	stray.Character.Name = "Curious Cat"
	stray.Character.RoomId = square.RoomId
	mobs.SetInstanceForTest(9781, pet)
	mobs.SetInstanceForTest(9782, stray)
	t.Cleanup(func() {
		mobs.SetInstanceForTest(9781, nil)
		mobs.SetInstanceForTest(9782, nil)
	})
	square.AddMob(9781)
	square.AddMob(9782)
	f.startFollow(followId{userId: 2}, followId{mobInstanceId: 9781}, 0)
	f.startFollow(followId{userId: 2}, followId{mobInstanceId: 9782}, 0)
	events.DrainQueuedInputsForTest(9781)
	events.DrainQueuedInputsForTest(9782)

	walkNorth(f, u2, square, cave)

	require.Equal(t, []string{"north"}, events.DrainQueuedInputsForTest(9781), "the companion follows")
	assert.True(t, pet.Character.KnowsSecretExit(square.RoomId, "north"), "the companion learned the exit")
	assert.Equal(t, []string{"follow stop"}, events.DrainQueuedInputsForTest(9782), "the stray stops")
	assert.False(t, stray.Character.KnowsSecretExit(square.RoomId, "north"))
	assert.False(t, f.isFollowing(followId{mobInstanceId: 9782}))
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/actions/ -run TestLearnSecretExitFollowing -count=1; go test ./internal/usercommands/ -run TestGo_FollowersLearnTheSecretExitTheLeaderTakes -count=1; go test ./modules/follow/ -run TestFollowSecret -count=1`
Expected: the actions test fails to compile (`undefined: LearnSecretExitFollowing`); the others FAIL on "learned the exit" (and the stray follows north).

- [ ] **Step 3: Write `internal/actions/follow_secret.go`**

```go
package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// LearnSecretExitFollowing records that c, moving through exitName of room
// behind the one it follows, now knows that exit when it is secret (#781,
// owner ruling 2: a player who moves through a secret exit with the leader
// they follow learns it; ruling 13 applies it to a companion or charmed NPC
// moving with its leader, and to a packmate behind its alpha). It is called
// when the follow step is issued, after the follower's sight check and before
// the step runs, because the step answers to the follower's own known-exit
// rule. An exit that is not secret, or no such exit: nothing.
func LearnSecretExitFollowing(c *characters.Character, room *rooms.Room, exitName string) {
	if c == nil || room == nil {
		return
	}
	if ex, ok := room.GetExitInfo(exitName); ok && ex.Secret {
		c.LearnSecretExit(room.RoomId, exitName)
	}
}
```

- [ ] **Step 4: Wire it into `internal/usercommands/go.go`**

In the party loop, replace:

```go
								partyUser.SendText(messaging.CategoryRoomExit, `You follow the party leader.`)
								// CmdFollowStep: the room they leave has just
								// lost the leader's light, which they walk on
								// with, so it must not be announced (#620).
								partyUser.CommandFlagged(rest, events.CmdFollowStep)
```

with:

```go
								partyUser.SendText(messaging.CategoryRoomExit, `You follow the party leader.`)
								// A member walking behind the leader through a
								// secret exit learns it (#781, owner ruling 2),
								// so their own step passes the known-exit rule.
								actions.LearnSecretExitFollowing(partyUser.Character, room, exitName)
								// CmdFollowStep: the room they leave has just
								// lost the leader's light, which they walk on
								// with, so it must not be announced (#620).
								partyUser.CommandFlagged(rest, events.CmdFollowStep)
```

In the charmed loop, directly above the line `mob.Command(rest)` inside `if mob.Character.IsCharmed(user.UserId) {`, insert:

```go
					// A companion moving with its owner through a secret
					// exit learns it (#781, owner ruling 13).
					actions.LearnSecretExitFollowing(&mob.Character, room, exitName)
```

- [ ] **Step 5: Wire it into `modules/follow/follow.go` `roomChangeHandler`**

Replace the mob branch body:

```go
				if mob := mobs.GetInstance(fId.mobInstanceId); mob != nil {
					if fromRoom.RoomId == mob.Character.RoomId {
						mob.Command(followExitName, .25)
```

with:

```go
				if mob := mobs.GetInstance(fId.mobInstanceId); mob != nil {
					if fromRoom.RoomId == mob.Character.RoomId {
						// #781, owner ruling 13: an NPC following a player
						// takes a secret exit only if it found it; a
						// companion or charmed NPC moving with its leader
						// learns it. Any other follower stops here.
						if !fromRoom.ExitKnownTo(&mob.Character, followExitName) {
							if evt.UserId == 0 || !mob.Character.IsCharmed(evt.UserId) {
								mob.Command(`follow stop`)
								f.stopFollowing(fId)
								continue
							}
							actions.LearnSecretExitFollowing(&mob.Character, fromRoom, followExitName)
						}
						mob.Command(followExitName, .25)
```

(The rest of the branch, `mob.WanderCount++` and `continue`, stays.) In the user branch, replace:

```go
						actions.BreakOffMenuToFollow(user, moverCharacter(evt), evt.UserId, fromRoom)
						user.Command(followExitName, .25)
```

with:

```go
						actions.BreakOffMenuToFollow(user, moverCharacter(evt), evt.UserId, fromRoom)
						// A follower who saw the one they follow go through
						// a secret exit learns it (#781, owner ruling 2).
						actions.LearnSecretExitFollowing(user.Character, fromRoom, followExitName)
						user.Command(followExitName, .25)
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/actions/ ./internal/usercommands/ ./modules/follow/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok`, including the existing `follow_light_test.go` and `follow_sight_test.go`.

- [ ] **Step 7: Prove the tests can fail**

Delete the party loop's `actions.LearnSecretExitFollowing(...)` line: `TestGo_FollowersLearnTheSecretExitTheLeaderTakes` FAILS at "the party member walked behind the leader". Restore. In `follow.go` change `if evt.UserId == 0 || !mob.Character.IsCharmed(evt.UserId) {` to `if true {`: the companion half FAILS. Restore; PASS.

- [ ] **Step 8: Update context.md**

`internal/actions/context.md`: add `LearnSecretExitFollowing(c, room, exitName)`: a follower moving behind its leader through a secret exit learns it (#781 rulings 2 and 13); called by the party follow and companion loops in `usercommands.Go`, by `modules/follow`, and by pack followers in `mobcommands.Wander`.

`modules/follow/context.md`: add: a player follower learns a secret exit the one they follow takes (#781 ruling 2). An NPC follower takes a secret exit only if it found it; one charmed by the mover learns it and follows, any other stops following (`follow stop`, ruling 13).

- [ ] **Step 9: Commit**

```bash
git add internal/actions/follow_secret.go internal/actions/follow_secret_test.go internal/actions/context.md internal/usercommands/go.go internal/usercommands/go_secret_exit_test.go modules/follow/follow.go modules/follow/follow_secret_test.go modules/follow/context.md
git commit -F - <<'EOF'
feat(follow): followers learn the secret exit their leader takes (#781)

A party member, a player follower or a companion moving behind its leader
through a secret exit learns it. Any other NPC following a player stops at
a secret exit it has not found.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5b (added 2026-11-22, owner ruling 14): onlookers learn the secret exit they see used

Added during the build after the batch 2 review found that the leave and
arrive lines name a secret exit to onlookers who had not found it. Owner
ruling 14: an onlooker who sees the mover learns the exit.

**Files:** `internal/usercommands/go.go` (the leave line, about line 300, and
the arrive line, about line 238, which uses `destRoom.FindExitTo`), and any
other place a leave or arrive line names an exit for onlookers (grep the
movement paths for the room-wide leave/arrive messages: `go.go`, mob moves in
`internal/mobcommands/go.go`, `RoomChange_ShadowFollow.go`, follow moves).

- [ ] **Step 1:** Find every place that sends a leave or arrive line naming an
  exit to the other characters in the room, for player and mob movers.
- [ ] **Step 2:** Write failing tests: (a) a sighted player in the origin room
  sees "X leaves <exit>" through a secret exit and afterwards
  `KnowsSecretExit(origin, exit)` is true; (b) a sighted player in the
  destination room sees X arrive and learns the destination room's secret
  exit leading back to the origin; (c) an onlooker who cannot see the mover
  (dark room, or the mover unseen) learns nothing; (d) the mover's own
  knowledge is unchanged by this rule; (e) an ordinary (not secret) exit
  records nothing.
- [ ] **Step 3:** Implement one helper beside `LearnSecretExitFollowing` in
  `internal/actions/follow_secret.go`, e.g.
  `LearnSecretExitWatching(room *rooms.Room, exitName string, mover any, onlookers ...)`,
  using the same sight decision that decides whether the line is shown to that
  onlooker (reuse the existing per-viewer visibility check of the leave and
  arrive lines; do not add a second sight rule). Call it where each line is
  sent, for player and NPC onlookers alike (NPC onlookers record on their
  instance, as search does).
- [ ] **Step 4:** Run the tests and null probes, `go test ./internal/usercommands/ ./internal/actions/ ./internal/mobcommands/ -count=1`.
- [ ] **Step 5:** Commit `feat(go): onlookers learn the secret exit they see used (#781)`.

### Task 5c (added 2026-11-25, owner rulings 15 and 16): seen use teaches, heard sound hides the name

Added after the Task 5b spec review found more lines that name a secret exit
to onlookers or listeners who had not found it.

**Sites from the review** (line numbers at `0c6149047`; re-grep, they move):
- Seen use, ruling 15 (teach with `actions.LearnSecretExitWatching` /
  `LearnSecretExitWatchingNow`, judged by the sight decision of the line):
  `usercommands/go.go` `unlockExit` (~45, three callers ~153 ~159 ~181);
  `usercommands/unlock.go` (~112 ~136), `lock.go` (~114 ~137),
  `picklock.go` (~165 ~283 ~296); `mobcommands/go.go` (~139, "tries to go
  the ... exit, but it's locked"); the "peers <exit>" lines
  (`usercommands/look.go` ~220 ~235, `mobcommands/look.go` ~60); shoot
  narration that names the exit the shot crossed (`{exitname}`).
- Heard, ruling 16: `rooms.SendTextToExitsExcluding` (`rooms.go` ~760), the
  `(From <exit>)` prefix. For a secret exit in the listener's room, a player
  who knows it reads the name, any other reads `(From nearby)`.
- NPC archer line of fire (`behaviortree/actions_archer.go` ~237,
  `actShootTarget` uses `room.FindExitTo`): an NPC archer chooses on its own
  to shoot through an exit, so under ruling 13 it may shoot only through a
  secret exit it has found (`FindKnownExitTo`). Decision taken without
  asking (D35), from the Task 7 quality review.
- Out of scope: `modules/aicompanion` (owner: never enabled).

- [ ] **Step 1:** Re-grep every non-test line in `internal/` and `modules/`
  that sends a room or neighbour line naming an exit (`fg="exit"`,
  `{exitname}`, the `exit.*Phrase` helpers) and classify each: movement
  (done in 5b), seen use (ruling 15), heard (ruling 16), mover-only (no
  change). Report the list.
- [ ] **Step 2:** Failing tests: (a) a sighted onlooker sees a lock picked or
  unlocked on a secret exit and learns the exit, and does NOT learn the
  lock's sequence; (b) the same in the dark learns nothing; (c) a peer and a
  shot through a secret exit teach a sighted onlooker; (d) a mob trying a
  locked secret exit teaches a sighted onlooker; (e) a neighbour who has not
  found the secret exit reads `(From nearby)`, one who has reads the name;
  an ordinary exit still reads its name for everyone; (f) the actor's own
  knowledge is unchanged by the rule.
- [ ] **Step 3:** Hook the existing helpers at each seen-use line; take
  snapshots only when the exit is secret (Task 5b's cost rule). For the
  heard prefix, when the listener-room exit is secret, send the named line to
  the players there who know it and the `(From nearby)` line to the rest,
  through the existing message exclusion, with no new sight rule. Learning
  stays off for heard lines.
- [ ] **Step 4:** Run `go test ./internal/usercommands/ ./internal/actions/ ./internal/mobcommands/ ./internal/rooms/ ./internal/hooks/ . ./internal/guards/ -count=1`, sabotage each new test, update `context.md` files.
- [ ] **Step 5:** Commit `feat(rooms): seen use teaches a secret exit, heard sound hides it (#781)`.

### Task 6: The maps and quest guidance use the discovery rule; `GetPathKnown`

**Files:**
- Modify: `internal/mapper/mapper.nearest.go` (lines 79-90), `mapper.crosszone.go:209`, `:278`
- Modify: `internal/mapper/mapper.path.go` (`findPath`, `GetPath`; add `GetPathKnown`, `takesRefusedSecret`)
- Modify: `internal/mapper/mapper.snapshot.go` (`Snapshot`, `WebMap.Snapshot`, `snapshotInto`, the gate at 245-263)
- Modify: `internal/mapper/mapper.go` (`GetLimitedMap`, the exit loop at 950 and gate at 977-987)
- Modify: the 12 test call sites in F24, and `internal/mapper/mapper.secret_guidance_test.go` (append)
- Modify: `modules/gmcp/gmcp.Zone.go:200`, `modules/gmcp/gmcp.Build.go:1168`
- Modify: `internal/questengine/destination.go:374-395`, `internal/questengine/destination_secret_test.go:13-47`
- Modify: `internal/mapper/context.md`, `internal/questengine/context.md`

- [ ] **Step 1: Write the failing tests**

Append to `internal/mapper/mapper.secret_guidance_test.go` (add imports `"strings"`, `"github.com/GoMudEngine/GoMud/internal/users"`):

```go
// #781 ruling 13: an NPC's unscripted route takes a secret exit only once the
// NPC found it; a scripted route (GetPath, AnySecretExit) takes it as before.
func TestGetPathKnown_TakesOnlyAllowedSecretExits(t *testing.T) {
	secretRingWorld(t)

	p, err := GetPathKnown(48001, nil, 48002)
	if err != nil || len(p) != 3 || p[0].exitName != "south" {
		t.Fatalf("unknown secret exit: %v, %v; want the 3-step open way starting south", p, err)
	}
	p, err = GetPathKnown(48001, knowsOnly(secretExit{48001, "east"}), 48002)
	if err != nil || len(p) != 1 || p[0].exitName != "east" {
		t.Fatalf("known secret exit: %v, %v; want the one step east", p, err)
	}
	p, err = GetPathKnown(48001, AnySecretExit, 48002)
	if err != nil || len(p) != 1 || p[0].exitName != "east" {
		t.Fatalf("a scripted route: %v, %v; want the one secret step east", p, err)
	}
	// The cached every-exit route must not leak into a filtered search.
	if p, err = GetPathKnown(48001, nil, 48002); err != nil || len(p) != 3 {
		t.Fatalf("after a cached secret route: %v, %v; want 3 steps", p, err)
	}
}

// Zone.Map draws a secret exit once the player found it (ruling 1), not once
// they visited the room beyond.
func TestSnapshot_SecretExitDrawnOnlyOnceFound(t *testing.T) {
	secretRingWorld(t)
	visited := map[int]struct{}{48001: {}, 48002: {}, 48003: {}, 48004: {}}
	hasEdge := func(snap []SnapshotRoom, from, to int) bool {
		for _, r := range snap {
			if r.RoomId != from {
				continue
			}
			for _, e := range r.Exits {
				if e.ToRoomId == to {
					return true
				}
			}
		}
		return false
	}
	m := GetMapper(48001)
	if hasEdge(m.Snapshot(visited, nil), 48001, 48002) {
		t.Fatal("an unfound secret exit was drawn although the room beyond is visited")
	}
	if !hasEdge(m.Snapshot(visited, knowsOnly(secretExit{48001, "east"})), 48001, 48002) {
		t.Fatal("a found secret exit must be drawn")
	}
}

// The ASCII map draws the secret glyph only for a secret exit the player found.
func TestGetLimitedMap_SecretExitDrawnOnlyOnceFound(t *testing.T) {
	secretRingWorld(t)
	u := users.NewTestUser(48090, "mapper", "Mapper", 948090)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{48090: u}))
	m := GetMapper(48001)
	cfg := Config{ZoomLevel: 1, Width: 21, Height: 11, UserId: 48090}
	glyphs := func() int { return strings.Count(renderText(m.GetLimitedMap(48001, cfg)), string(SecretSymbol)) }
	before := glyphs()
	u.Character.LearnSecretExit(48001, "east")
	if after := glyphs(); after <= before {
		t.Fatalf("secret glyphs %d before the find and %d after: a found secret exit must be drawn, an unfound one not", before, after)
	}
}
```

In `internal/questengine/destination_secret_test.go`, replace lines 13-47 (the comment and `TestSecretExitsKnownTo`) with:

```go
// Guidance may send a player through a secret exit only once they found it
// (#781, owner ruling 1: one rule everywhere). Being on the player's map or
// having been there recently no longer counts.
func TestSecretExitsKnownTo(t *testing.T) {
	const here, beyond = 61001, 61002
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		here:   {RoomId: here, Zone: "zk"},
		beyond: {RoomId: beyond, Zone: "zk"},
	}, map[string]*rooms.ZoneConfig{}))

	u := destUser(here)
	known := secretExitsKnownTo(u)
	if known(here, "west", beyond) {
		t.Fatalf("a secret exit the player has not found must not be used")
	}

	u.Character.AddDiscovery(here, characters.SecretExitDiscoveryKey("west"))
	if !known(here, "west", beyond) {
		t.Errorf("a secret exit `search` found must be usable")
	}
	if known(here, "north", beyond) {
		t.Errorf("finding the west exit must not reveal the north one")
	}

	v := destUser(here)
	v.Character.MarkRoomVisited("zk", beyond)
	if secretExitsKnownTo(v)(here, "north", beyond) {
		t.Errorf("the room beyond on the player's map must no longer open an unfound secret exit")
	}

	w := destUser(here)
	rooms.LoadRoom(beyond).MarkVisited(w.UserId, rooms.VisitorUser)
	if secretExitsKnownTo(w)(here, "north", beyond) {
		t.Errorf("a recent visit to the room beyond must no longer open an unfound secret exit")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/mapper/ -run 'TestGetPathKnown|TestSnapshot_SecretExitDrawnOnlyOnceFound|TestGetLimitedMap_SecretExitDrawnOnlyOnceFound' -count=1; go test ./internal/questengine/ -run TestSecretExitsKnownTo -count=1`
Expected: mapper fails to compile (`undefined: GetPathKnown`, `AnySecretExit`, too many arguments to `Snapshot`); questengine FAILS on the map and recent-visit cases.

- [ ] **Step 3: `mapper.nearest.go` and `mapper.crosszone.go`**

Replace lines 79-90 of `mapper.nearest.go`:

```go
// SecretExitKnown reports whether quest guidance may send its traveller
// through the secret exit exitName of roomId, which leads to toRoomId. Only
// guidance asks: a mob's path (GetPath, `pathto`) takes every secret exit.
// A nil SecretExitKnown lets none through.
type SecretExitKnown func(roomId int, exitName string, toRoomId int) bool

func (k SecretExitKnown) allows(roomId int, exitName string, toRoomId int) bool {
	return k != nil && k(roomId, exitName, toRoomId)
}

// anySecretExit lets every secret exit through, as GetPath does.
func anySecretExit(int, string, int) bool { return true }
```

with:

```go
// SecretExitKnown reports whether a traveller may go through the secret exit
// exitName of roomId, which leads to toRoomId. Quest guidance, the player's
// maps (Snapshot, GetLimitedMap) and an NPC's unscripted route (GetPathKnown)
// ask it, built from SecretExitsKnownTo (#781). A scripted route (GetPath)
// takes every secret exit. A nil SecretExitKnown lets none through.
type SecretExitKnown func(roomId int, exitName string, toRoomId int) bool

func (k SecretExitKnown) allows(roomId int, exitName string, toRoomId int) bool {
	return k != nil && k(roomId, exitName, toRoomId)
}

// AnySecretExit lets every secret exit through: GetPath (a scripted route)
// and the room builder's map.
func AnySecretExit(int, string, int) bool { return true }

// SecretExitsKnownTo lets through the secret exits c has found
// (Character.KnowsSecretExit), the one secret-exit rule (#781, owner rulings 1
// and 13).
func SecretExitsKnownTo(c *characters.Character) SecretExitKnown {
	return func(roomId int, exitName string, _ int) bool {
		return c.KnowsSecretExit(roomId, exitName)
	}
}
```

Add `"github.com/GoMudEngine/GoMud/internal/characters"` to `mapper.nearest.go`'s imports. In `mapper.crosszone.go` lines 209 and 278 replace `anySecretExit` with `AnySecretExit`.

- [ ] **Step 4: `mapper.path.go`**

Rename `func (r *mapper) findPath(startRoom, goalRoom int) ([]pathStep, error) {` to `func (r *mapper) findPathWhere(startRoom, goalRoom int, known SecretExitKnown) ([]pathStep, error) {` and above it add:

```go
// findPath is findPathWhere taking every secret exit.
func (r *mapper) findPath(startRoom, goalRoom int) ([]pathStep, error) {
	return r.findPathWhere(startRoom, goalRoom, AnySecretExit)
}
```

Update the doc comment's first line to `// findPathWhere returns the sequence of ExitName/RoomID steps from startRoom to goalRoom, taking a secret exit only when known lets it through.` Inside the neighbour loop replace:

```go
		for exitName, exitInfo := range node.Exits {
			neighbor := exitInfo.RoomId
			tentativeG := gScore[current.roomId] + 1 // uniform cost
```

with:

```go
		for exitName, exitInfo := range node.Exits {
			neighbor := exitInfo.RoomId
			// A secret exit known does not let through is not taken (#781).
			if exitInfo.Secret && !known.allows(current.roomId, exitName, neighbor) {
				continue
			}
			tentativeG := gScore[current.roomId] + 1 // uniform cost
```

After `GetPath`'s closing brace add:

```go
// GetPathKnown is GetPath for a traveller who takes a secret exit only when
// known lets it through: an NPC's unscripted route, such as a planner's
// errand (#781, owner ruling 13). The shared path cache holds GetPath's
// every-exit routes, so a cached route is used when it takes no secret exit
// known refuses; otherwise the route is searched again, uncached, without
// those exits.
func GetPathKnown(startRoomId int, known SecretExitKnown, endRoomId ...int) ([]pathStep, error) {
	if path, err := GetPath(startRoomId, endRoomId...); err == nil && !takesRefusedSecret(startRoomId, path, known) {
		return path, nil
	}

	start := time.Now()
	defer func() {
		util.TrackTime(`mapper.GetPathKnown()`, time.Since(start).Seconds())
	}()

	if len(endRoomId) == 0 {
		return []pathStep{}, fmt.Errorf("%d => %d (endRoom not found): %w", startRoomId, endRoomId, ErrPathNotFound)
	}
	startRoom := rooms.LoadRoom(startRoomId)
	if startRoom == nil {
		return []pathStep{}, fmt.Errorf("%d => %d (startRoom not found): %w", startRoomId, endRoomId, ErrPathNotFound)
	}
	m := GetMapper(startRoom.RoomId)
	if m == nil {
		return []pathStep{}, fmt.Errorf("%d => %d (mapper not found): %w", startRoomId, endRoomId, ErrPathNotFound)
	}

	rNow := startRoomId
	finalPath := []pathStep{}
	for _, roomId := range endRoomId {
		if rNow == roomId {
			continue
		}
		p, err := m.findPathWhere(rNow, roomId, known)
		if err != nil {
			return []pathStep{}, fmt.Errorf("%d => %d: %w", rNow, roomId, ErrPathNotFound)
		}
		finalPath = append(finalPath, p...)
		rNow = roomId
	}
	if len(finalPath) == 0 {
		return finalPath, ErrPathDestMatch
	}
	return finalPath, nil
}

// takesRefusedSecret reports whether path, walked from startRoomId, goes
// through a secret exit known does not let through.
func takesRefusedSecret(startRoomId int, path []pathStep, known SecretExitKnown) bool {
	m := GetMapper(startRoomId)
	if m == nil {
		return true
	}
	at := startRoomId
	for _, step := range path {
		node := m.nodeFor(at)
		if node == nil {
			return true
		}
		if ex, ok := node.Exits[step.exitName]; ok && ex.Secret && !known.allows(at, step.exitName, step.roomId) {
			return true
		}
		at = step.roomId
	}
	return false
}
```

- [ ] **Step 5: `mapper.snapshot.go`**

- `func (r *mapper) Snapshot(visited map[int]struct{}) []SnapshotRoom {` becomes `func (r *mapper) Snapshot(visited map[int]struct{}, known SecretExitKnown) []SnapshotRoom {` and its body `return r.snapshotInto(make([]SnapshotRoom, 0, len(visited)), visited, nil, nil, known)`.
- In its doc comment replace `// Undiscovered secret exits are omitted entirely, matching GetLimitedMap's` / `// behaviour for the ASCII map. See the comment at the gate below.` with `// A secret exit is drawn only when known lets it through (the player's` / `// SecretExitsKnownTo, #781), as GetLimitedMap draws the ASCII map.`
- `func (w *WebMap) Snapshot(visited map[int]struct{}) []SnapshotRoom {` becomes `func (w *WebMap) Snapshot(visited map[int]struct{}, known SecretExitKnown) []SnapshotRoom {`, and its call `out = m.snapshotInto(out, visited, far, func(id int) bool { return drawnFrom[id] == m })` becomes `out = m.snapshotInto(out, visited, far, func(id int) bool { return drawnFrom[id] == m }, known)`.
- `func (r *mapper) snapshotInto(out []SnapshotRoom, visited map[int]struct{}, far func(int) *mapNode, own func(int) bool) []SnapshotRoom {` gains `, known SecretExitKnown` as its last parameter; add `// known (nil: none) lets a secret exit be drawn.` to its comment.
- Replace the exit loop head and the secret gate:

```go
		for _, e := range n.Exits {
			// Don't render an exit stub toward an ephemeral/instance room.
			if rooms.IsEphemeralRoomId(e.RoomId) {
				continue
			}
			// A secret exit is not disclosed until the player has been through
			// it. GetLimitedMap does exactly this for the ASCII map (see the
			// exitInfo.Secret block there): it skips the exit entirely unless
			// the room on the far side has been visited. Without the same gate
			// the web map revealed secret exits the ASCII map hides, which is a
			// real advantage rather than a cosmetic difference. Skipping the
			// whole exit (rather than just clearing the Secret flag) is
			// deliberate: emitting an unflagged stub would still tell the
			// player a passage is there.
			if e.Secret {
				if _, seen := visited[e.RoomId]; !seen {
					continue
				}
			}
```

with:

```go
		for exitName, e := range n.Exits {
			// Don't render an exit stub toward an ephemeral/instance room.
			if rooms.IsEphemeralRoomId(e.RoomId) {
				continue
			}
			// A secret exit is drawn once the player has found it, the one
			// rule every exit list reads (#781, owner ruling 1; known is the
			// player's SecretExitsKnownTo). Skipping the whole exit, rather
			// than clearing its Secret flag, is deliberate: an unflagged stub
			// would still tell the player a passage is there.
			if e.Secret && !known.allows(id, exitName, e.RoomId) {
				continue
			}
```

- [ ] **Step 6: `mapper.go` `GetLimitedMap`**

Directly after `out := newMapRender(c.Width, c.Height)` add:

```go
	// The secret exits this map may draw: the player's found ones (#781), all
	// of them for a debug render (UserId below zero), none when there is no
	// such player.
	var secretKnown SecretExitKnown
	if c.UserId < 0 {
		secretKnown = AnySecretExit
	} else if u := users.GetByUserId(c.UserId); u != nil {
		secretKnown = SecretExitsKnownTo(u.Character)
	}
```

Change the crawl loop head `for _, exitInfo := range node.Exits {` (the one at about line 950, inside the `for len(r.crawlQueue) > 0` loop, followed by `if _, ok := tmpCrawledTracker[exitInfo.RoomId]; ok {`) to `for exitName, exitInfo := range node.Exits {`, and replace the gate:

```go
			if exitInfo.Secret {
				targetRoom := rooms.LoadRoom(exitInfo.RoomId)
				if targetRoom == nil {
					continue
				}

				if c.UserId >= 0 {
					if !targetRoom.HasVisited(c.UserId, rooms.VisitorUser) {
						continue
					}
				}
			}
```

with:

```go
			if exitInfo.Secret && !secretKnown.allows(node.RoomId, exitName, exitInfo.RoomId) {
				continue
			}
```

Leave `GetFullMap`'s loop (about line 1149) alone: it is not a player list.

- [ ] **Step 7: Callers**

- `modules/gmcp/gmcp.Zone.go:200`: `snap := view.Snapshot(visited)` becomes `snap := view.Snapshot(visited, mapper.SecretExitsKnownTo(user.Character))`.
- `modules/gmcp/gmcp.Build.go:1168`: `snap := view.Snapshot(visited)` becomes `snap := view.Snapshot(visited, mapper.AnySecretExit)` (the builder sees every exit).
- Tests (F24), add `, nil` as the last argument: `mapper.snapshot_cold_test.go:134` (`m.WebMap().Snapshot(visited)`), `:174`, `:194`, `:280`; `mapper.snapshot_test.go:25`, `:52`, `:70`, `:99`, `:151`, `:203`, `:248`; `mapper.zoneborder_test.go:598`. The golden at `snapshot_cold_test.go` omits the unvisited secret 9004 and stays byte-identical with nil.
- `internal/questengine/destination.go:374-395`: replace the comment and the variable with:

```go
// secretExitsKnownTo is the secret exits guidance may send user through: the
// ones they found, the one rule every exit list reads (#781, owner ruling 1;
// mapper.SecretExitsKnownTo). Any other secret exit is left out of the route,
// in the zone and across a border, so "Head west." never names an exit the
// player has not found. Mob paths take them all.
var secretExitsKnownTo = func(user *users.UserRecord) mapper.SecretExitKnown {
	return mapper.SecretExitsKnownTo(user.Character)
}
```

Run `go build ./...`; the compiler lists any Snapshot call this table missed. Remove imports it reports unused (none expected: `destination.go` still uses `characters` and `rooms`).

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./internal/mapper/ ./internal/questengine/ ./modules/gmcp/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok`, including `TestSnapshot_WarmPayloadMatchesGolden`, `TestGuidanceSkipsAnUnknownSecretExit` and `TestGuidanceSkipsAnUnknownSecretBorder`.

- [ ] **Step 9: Prove the tests can fail**

In `findPathWhere`, delete the new `if exitInfo.Secret && !known.allows(...) { continue }`: `TestGetPathKnown_TakesOnlyAllowedSecretExits` FAILS (one step east for nil). Restore. In `GetLimitedMap` set `secretKnown = AnySecretExit` unconditionally: the ASCII test FAILS. Restore; PASS.

- [ ] **Step 10: Update context.md**

`internal/mapper/context.md`: add: `SecretExitsKnownTo(c)` is the secret-exit filter built on `Character.KnowsSecretExit` (#781); `AnySecretExit` lets all through. `Snapshot(visited, known)` and `WebMap.Snapshot(visited, known)` draw a secret exit only when `known` allows (Zone.Map passes the player's, the builder `AnySecretExit`); `GetLimitedMap` uses the player's found exits. `GetPath` (scripted routes) takes every secret exit; `GetPathKnown(start, known, end...)` is an NPC's unscripted route. Replace any sentence saying a secret exit shows once the far room is visited.

`internal/questengine/context.md`: where secret exits in guidance are described, replace the three-way rule with: guidance sends a player only through secret exits they found (`mapper.SecretExitsKnownTo`, #781).

- [ ] **Step 11: Commit**

```bash
git add internal/mapper/ modules/gmcp/gmcp.Zone.go modules/gmcp/gmcp.Build.go internal/questengine/destination.go internal/questengine/destination_secret_test.go internal/questengine/context.md
git commit -F - <<'EOF'
feat(mapper): maps and guidance show a secret exit only once found (#781)

Zone.Map, the ASCII map and quest guidance read SecretExitsKnownTo, the
discovery rule, instead of the visited set and the visitor trail.
GetPathKnown is GetPath for an NPC's unscripted route.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

(`git add internal/mapper/` names one directory whose every change belongs to this task; check `git status` first.)

---

### Task 7: NPCs: unscripted moves take only found secret exits

**Files:**
- Modify: `internal/mobcommands/wander.go`, `internal/mobcommands/wander_filter_test.go` (5 calls)
- Create: `internal/mobcommands/wander_secret_test.go`
- Create: `internal/mobs/pathto.go`
- Modify: `internal/mobcommands/pathto.go`; Create: `internal/mobcommands/pathto_found_test.go`
- Modify: `internal/behaviortree/actions_goal.go:71-72`, `actions_mob.go:295-321`, `actions_archer.go:315-330`
- Create: `internal/behaviortree/secret_exit_route_test.go`
- Modify: `internal/hooks/NewRound_DoCombat_unified.go:931-941`
- Modify: `internal/mobcommands/context.md`, `internal/behaviortree/context.md`, `internal/hooks/context.md`, `internal/mobs/context.md`

- [ ] **Step 1: Write the failing tests**

Create `internal/mobcommands/wander_secret_test.go`:

```go
package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// wanderSecretRoom is room 1 with a secret north to 100 and an open south to
// 200; both far rooms hold gold, so `wander loot` wants both.
func wanderSecretRoom(t *testing.T) *rooms.Room {
	t.Helper()
	here := &rooms.Room{RoomId: 1, Exits: map[string]exit.RoomExit{
		"north": {RoomId: 100, Secret: true},
		"south": {RoomId: 200},
	}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		1:   here,
		100: {RoomId: 100, Gold: 5},
		200: {RoomId: 200, Gold: 5},
	}, map[string]*rooms.ZoneConfig{}))
	return here
}

// #781, owner ruling 13: a wandering NPC takes a secret exit only once it
// found it.
func TestWanderCandidates_AnUnfoundSecretExitIsNoCandidate(t *testing.T) {
	room := wanderSecretRoom(t)
	mover := &characters.Character{}
	wantsAll := func(*rooms.Room) bool { return true }

	got := wanderCandidates(room, mover, wantsAll)
	if len(got) != 1 || got[0] != "south" {
		t.Fatalf("candidates %v, want only south", got)
	}
	mover.LearnSecretExit(1, "north")
	if got := wanderCandidates(room, mover, wantsAll); len(got) != 2 {
		t.Fatalf("after the find: candidates %v, want north and south", got)
	}
}

func TestPickWanderExit_FallbackTakesOnlyAFoundSecretExit(t *testing.T) {
	room := &rooms.Room{RoomId: 1, Exits: map[string]exit.RoomExit{"north": {RoomId: 100, Secret: true}}}
	mover := &characters.Character{}
	for i := 0; i < 50; i++ {
		if name, _ := pickWanderExit(nil, room, mover); name != "" {
			t.Fatalf("picked %q: the only exit is an unfound secret", name)
		}
	}
	mover.LearnSecretExit(1, "north")
	if name, _ := pickWanderExit(nil, room, mover); name != "north" {
		t.Fatalf("picked %q, want the found north", name)
	}
}
```

Create `internal/mobcommands/pathto_found_test.go`:

```go
package mobcommands

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// `pathto found <ids>` is a planner's errand (#781, ruling 13): the route
// takes only the secret exits the mob found. Without the word it is a
// scripted route and takes them all.
func TestPathtoTargets(t *testing.T) {
	ids, foundOnly := pathtoTargets(mobs.PathtoFoundOnly+" 12 13", 7)
	if !foundOnly || !reflect.DeepEqual(ids, []int{12, 13}) {
		t.Fatalf("found route: %v %v, want [12 13] true", ids, foundOnly)
	}
	ids, foundOnly = pathtoTargets("home 4", 7)
	if foundOnly || !reflect.DeepEqual(ids, []int{7, 4}) {
		t.Fatalf("scripted route: %v %v, want [7 4] false", ids, foundOnly)
	}
}
```

Create `internal/behaviortree/secret_exit_route_test.go`:

```go
package behaviortree

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// #781, owner ruling 13: a planner's pathto is the mob's own errand, so it is
// sent as `pathto found ...`; any other command passes as written.
func TestUnscriptedRoute(t *testing.T) {
	for in, want := range map[string]string{
		"pathto 12":       "pathto " + mobs.PathtoFoundOnly + " 12",
		"pathto 12 13":    "pathto " + mobs.PathtoFoundOnly + " 12 13",
		"wander":          "wander",
		"pathto":          "pathto",
		"pathtoo 12":      "pathtoo 12",
		"go north":        "go north",
	} {
		if got := unscriptedRoute(in); got != want {
			t.Errorf("unscriptedRoute(%q) = %q, want %q", in, got, want)
		}
	}
}

// A walk-away (actFlee out of combat) heads home only through exits the mob
// knows.
func TestPickRetreatExit_SkipsAnUnfoundSecretExit(t *testing.T) {
	room := &rooms.Room{RoomId: 31, Exits: map[string]exit.RoomExit{
		"west": {RoomId: 40, Secret: true},
		"east": {RoomId: 41},
	}}
	mob := &mobs.Mob{HomeRoomId: 40}
	if got := pickRetreatExit(mob, room); got != "east" {
		t.Fatalf("retreat %q, want east: west is an unfound secret exit", got)
	}
	mob.Character.LearnSecretExit(31, "west")
	if got := pickRetreatExit(mob, room); got != "west" {
		t.Fatalf("retreat %q, want west (home, found)", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/mobcommands/ -run 'TestWanderCandidates|TestPickWanderExit_FallbackTakesOnlyAFoundSecretExit|TestPathtoTargets' -count=1; go test ./internal/behaviortree/ -run 'TestUnscriptedRoute|TestPickRetreatExit_SkipsAnUnfoundSecretExit' -count=1`
Expected: compile failures (`undefined: wanderCandidates`, `pathtoTargets`, `mobs.PathtoFoundOnly`, `unscriptedRoute`; `pickWanderExit` argument count).

- [ ] **Step 3: `internal/mobs/pathto.go`**

```go
package mobs

// PathtoFoundOnly is the word `pathto` reads as "take only the secret exits
// this mob has found" (#781, owner ruling 13). A planner's errand route
// carries it (behaviortree.RunGoalPlanner). A schedule's, a patrol's, a
// behaviour tree's, an idle command's or an admin's pathto does not: that
// route is the mob's regular way, and it knows it.
const PathtoFoundOnly = `found`
```

- [ ] **Step 4: `internal/mobcommands/wander.go`**

Add `"github.com/GoMudEngine/GoMud/internal/characters"` to the imports. Replace `pickWanderExit`:

```go
func pickWanderExit(exitOptions []string, room *rooms.Room) (string, int) {
	if len(exitOptions) > 0 {
		picked := exitOptions[util.Rand(len(exitOptions))]
		if ex, ok := room.Exits[picked]; ok {
			return picked, ex.RoomId
		}
	}
	return room.GetRandomExit()
}
```

with:

```go
//
// The fallback takes a secret exit only once the mover found it (#781, owner
// ruling 13: wandering is the NPC's own choice).
func pickWanderExit(exitOptions []string, room *rooms.Room, mover *characters.Character) (string, int) {
	if len(exitOptions) > 0 {
		picked := exitOptions[util.Rand(len(exitOptions))]
		if ex, ok := room.Exits[picked]; ok {
			return picked, ex.RoomId
		}
	}
	return room.GetRandomKnownExit(mover)
}

// wanderCandidates is the exits of room whose far room wants accepts, among
// the exits mover knows: a secret exit only once found (#781, ruling 13).
func wanderCandidates(room *rooms.Room, mover *characters.Character, wants func(dest *rooms.Room) bool) []string {
	out := []string{}
	for exitName, ex := range room.Exits {
		if !room.ExitKnownTo(mover, exitName) {
			continue
		}
		dest := rooms.LoadRoom(ex.RoomId)
		if dest == nil || !wants(dest) {
			continue
		}
		out = append(out, exitName)
	}
	return out
}
```

(The `//` line continues the existing `pickWanderExit` doc comment.) In `Wander`, replace from `exitOptions := make([]string, 0)` through the closing brace of the second candidate block (the one for `players`, just before the `pickWanderExit` call) with:

```go
	exitOptions := make([]string, 0)
	restrictZone := true

	switch rest {
	case `loot`: // first only consider adjacent rooms with loot in them
		exitOptions = wanderCandidates(room, &mob.Character, func(dest *rooms.Room) bool {
			return hasLootItems(dest.Items) || dest.Gold > 0
		})
	case `players`: // first only consider adjacent rooms with players in them
		exitOptions = wanderCandidates(room, &mob.Character, func(dest *rooms.Room) bool {
			return dest.PlayerCt() > 0
		})
	}
```

In the line that calls `pickWanderExit(exitOptions, room)`, change that call to `pickWanderExit(exitOptions, room, &mob.Character)` (the rest of the line unchanged). In the `MovePackFollowers` callback, between `if !actions.QuoteMobStep(f, exitName).OK() { return false }` and `ambient.CountMove(room.RoomId, ambient.MoveIdle)`, insert:

```go
						// A packmate walking behind its alpha learns a secret
						// exit the alpha took (#781, ruling 2's NPC analogue).
						actions.LearnSecretExitFollowing(&f.Character, room, exitName)
```

In `wander_filter_test.go`, every `pickWanderExit(x, room)` (lines 34, 50, 66, 82, 98) becomes `pickWanderExit(x, room, nil)`.

- [ ] **Step 5: `internal/mobcommands/pathto.go`**

Replace from `toRoomIds := []int{}` through the end of the `for _, roomIdStr := range ...` loop with:

```go
	toRoomIds, foundOnly := pathtoTargets(rest, mob.HomeRoomId)
```

Replace `path, err := mapper.GetPath(mob.Character.RoomId, toRoomIds...)` with:

```go
	// A planner's errand (`pathto found ...`) takes only the secret exits
	// this mob found; any other pathto is a scripted route and takes them all
	// (#781, owner ruling 13).
	var known mapper.SecretExitKnown = mapper.AnySecretExit
	if foundOnly {
		known = mapper.SecretExitsKnownTo(&mob.Character)
	}
	path, err := mapper.GetPathKnown(mob.Character.RoomId, known, toRoomIds...)
```

Append:

```go
// pathtoTargets reads pathto's words: room ids, `home` (homeRoomId), and
// mobs.PathtoFoundOnly, which marks an NPC's own errand.
func pathtoTargets(rest string, homeRoomId int) (toRoomIds []int, foundOnly bool) {
	toRoomIds = []int{}
	for _, roomIdStr := range util.SplitButRespectQuotes(strings.ToLower(rest)) {
		if roomIdStr == mobs.PathtoFoundOnly {
			foundOnly = true
			continue
		}
		if roomIdStr == `home` {
			toRoomIds = append(toRoomIds, homeRoomId)
			continue
		}
		if roomIdInt, err := strconv.Atoi(roomIdStr); err == nil {
			toRoomIds = append(toRoomIds, roomIdInt)
		}
	}
	return toRoomIds, foundOnly
}
```

- [ ] **Step 6: `internal/behaviortree`**

`actions_goal.go`: change `mob.Command(result.Command)` (line 72) to `mob.Command(unscriptedRoute(result.Command))` and append to the file:

```go
// unscriptedRoute marks a planner's pathto as the mob's own errand, so its
// route takes a secret exit only if the mob found it (#781, owner ruling 13).
// Every planner command runs through RunGoalPlanner, so this is the one
// place. A schedule's, a patrol's or a behaviour tree's pathto never comes
// here and stays scripted.
func unscriptedRoute(cmd string) string {
	verb, rest, ok := strings.Cut(cmd, " ")
	if !ok || verb != "pathto" {
		return cmd
	}
	return "pathto " + mobs.PathtoFoundOnly + " " + rest
}
```

(add `"strings"` to its imports if absent).

`actions_mob.go`, in `actGoToCallerRoom`, replace:

```go
	for exitName, info := range myRoom.Exits {
		if info.RoomId == caller.Character.RoomId {
			// Movement parity 4b: a tired mob fails the node so a selector
			// falls through, rather than issuing a step it cannot pay.
			if !actions.QuoteMobStep(self, exitName).OK() {
				return Failure
			}
			self.Command(fmt.Sprintf("go %s", exitName))
			return Success
		}
	}
	return Failure
```

with:

```go
	// Heading to a packmate is the mob's own choice: a secret exit only if it
	// found it (#781, owner ruling 13).
	exitName := myRoom.FindKnownExitTo(&self.Character, caller.Character.RoomId)
	if exitName == `` {
		return Failure
	}
	// Movement parity 4b: a tired mob fails the node so a selector
	// falls through, rather than issuing a step it cannot pay.
	if !actions.QuoteMobStep(self, exitName).OK() {
		return Failure
	}
	self.Command(fmt.Sprintf("go %s", exitName))
	return Success
```

`actions_archer.go`, in `pickRetreatExit`, replace:

```go
	for dir, info := range room.Exits {
		if info.Lock.IsLocked() {
			continue
		}
```

with:

```go
	for dir, info := range room.Exits {
		if info.Lock.IsLocked() {
			continue
		}
		// A walk-away is the mob's own choice (#781, owner ruling 13).
		if !room.ExitKnownTo(&mob.Character, dir) {
			continue
		}
```

- [ ] **Step 7: `internal/hooks/NewRound_DoCombat_unified.go`**

Replace (about lines 931-941):

```go
				if mobRoom := rooms.LoadRoom(defChar.RoomId); mobRoom != nil {
					for exitName, exitInfo := range mobRoom.Exits {
						if exitInfo.RoomId == atkChar.RoomId {
							defMob.Command(fmt.Sprintf(`go %s`, exitName))
							if actionStr := defMob.GetAngryCommand(); actionStr != `` {
								defMob.Command(actionStr)
							}
							break
						}
					}
				}
```

with:

```go
				if mobRoom := rooms.LoadRoom(defChar.RoomId); mobRoom != nil {
					// Going after the attacker is the mob's own choice: a
					// secret exit only if it found it (#781, owner ruling 13).
					if exitName := mobRoom.FindKnownExitTo(defChar, atkChar.RoomId); exitName != `` {
						defMob.Command(fmt.Sprintf(`go %s`, exitName))
						if actionStr := defMob.GetAngryCommand(); actionStr != `` {
							defMob.Command(actionStr)
						}
					}
				}
```

(`defChar` is `def.GetCharacter()`, a `*characters.Character`.) `FindKnownExitTo`'s own tests (task 2) cover the rule; this site and `actGoToCallerRoom` only call it.

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/mobcommands/ ./internal/behaviortree/ ./internal/hooks/ ./internal/mobs/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok`, including `TestEveryPlannerCommandIsRegistered` (the verb is still `pathto`) and the existing wander tests.

- [ ] **Step 9: Prove the tests can fail**

In `wanderCandidates` delete `if !room.ExitKnownTo(mover, exitName) { continue }`: the wander test FAILS. Restore. In `unscriptedRoute` return `cmd` first thing: `TestUnscriptedRoute` FAILS. Restore. In `pickRetreatExit` delete the new check: the retreat test FAILS (west picked as home). Restore; PASS.

- [ ] **Step 10: Update context.md**

`internal/mobcommands/context.md`: wandering (`wander`, `wander loot`, `wander players`, the random fallback) takes a secret exit only once the mob found it, and packmates following the alpha learn it (#781 ruling 13). `pathto found <ids>` (`mobs.PathtoFoundOnly`) routes through `mapper.GetPathKnown` with the mob's found exits; plain `pathto` (schedules, patrols, trees, idle commands, admin) takes every secret exit.

`internal/behaviortree/context.md`: `RunGoalPlanner` sends a planner's `pathto` as `pathto found ...` (`unscriptedRoute`); `actGoToCallerRoom` and `pickRetreatExit` use only exits the mob knows (#781).

`internal/hooks/context.md`: in the combat section, a mob walking to a player who attacked from the next room uses `FindKnownExitTo` (#781).

`internal/mobs/context.md`: `PathtoFoundOnly` (`pathto.go`).

- [ ] **Step 11: Commit**

```bash
git add internal/mobs/pathto.go internal/mobs/context.md internal/mobcommands/wander.go internal/mobcommands/wander_filter_test.go internal/mobcommands/wander_secret_test.go internal/mobcommands/pathto.go internal/mobcommands/pathto_found_test.go internal/mobcommands/context.md internal/behaviortree/actions_goal.go internal/behaviortree/actions_mob.go internal/behaviortree/actions_archer.go internal/behaviortree/secret_exit_route_test.go internal/behaviortree/context.md internal/hooks/NewRound_DoCombat_unified.go internal/hooks/context.md
git commit -F - <<'EOF'
feat(mobs): an NPC's own moves take only secret exits it found (#781)

Wandering, a planner's errand route, heading to a packmate, a walk-away
and going after an attacker in the next room use only exits the mob
knows. Scripted routes (schedules, patrols, trees, admin) are unchanged.
Packmates learn the secret exit their alpha takes.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: `internal/buttons`, `events.RoomExitsChanged`, `refusal.StaleButton`

**Files:**
- Create: `internal/buttons/buttons.go`, `internal/buttons/darkmarks.go`, `internal/buttons/buttons_test.go`, `internal/buttons/darkmarks_test.go`, `internal/buttons/context.md`
- Create: `internal/events/roomexits.go`; Modify: `internal/events/context.md`
- Create: `internal/refusal/button.go`, `internal/refusal/button_test.go`; Modify: `internal/refusal/context.md`

- [ ] **Step 1: Write the failing tests**

Create `internal/buttons/buttons_test.go`:

```go
package buttons

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
)

func list(names ...string) rooms.KnownExitList {
	l := rooms.KnownExitList{}
	for _, n := range names {
		kind := rooms.ExitKindOf(n, n == "portal")
		l.Exits = append(l.Exits, rooms.KnownExit{Name: n, Kind: kind})
	}
	return l
}

var perRoomId = regexp.MustCompile(`^x[0-9a-f]{8}$`)

// Ruling 8: the six T directions have fixed ids, the same in every room and
// for every player; a fast double tap works like typing.
func TestMovementIds_TheTDirectionsHaveFixedIds(t *testing.T) {
	t.Cleanup(func() { Forget(1); Forget(2) })
	a := MovementIds(1, &rooms.Room{RoomId: 5}, list("north", "up"))
	b := MovementIds(2, &rooms.Room{RoomId: 6}, list("north", "up"))
	if a["north"] != "m1" || a["up"] != "m5" || b["north"] != "m1" || b["up"] != "m5" {
		t.Fatalf("fixed ids differ: %v %v", a, b)
	}
	for dir, id := range FixedIds() {
		line, ok := Resolve(1, 999, id)
		if !ok || line != dir {
			t.Errorf("fixed id %q resolves to %q %v in any room, want %q", id, line, ok, dir)
		}
	}
}

// Other exits get per-room ids: opaque, unguessable, stable while the player
// stays in the room, stale once they leave.
func TestMovementIds_PerRoomIdsAreStableThenStale(t *testing.T) {
	t.Cleanup(func() { Forget(1) })
	room5 := &rooms.Room{RoomId: 5}
	first := MovementIds(1, room5, list("north", "northeast", "out", "portal"))
	for _, name := range []string{"northeast", "out", "portal"} {
		if !perRoomId.MatchString(first[name]) {
			t.Fatalf("%s id %q is not an opaque per-room id", name, first[name])
		}
		if strings.Contains(first[name], name) {
			t.Fatalf("%s id %q names its exit", name, first[name])
		}
	}
	again := MovementIds(1, room5, list("north", "northeast", "out"))
	if again["northeast"] != first["northeast"] || again["out"] != first["out"] {
		t.Fatalf("a resend in the same room changed the ids: %v then %v", first, again)
	}
	if line, ok := Resolve(1, 5, first["northeast"]); !ok || line != "go northeast" {
		t.Fatalf("resolve %q = %q %v, want go northeast", first["northeast"], line, ok)
	}
	if _, ok := Resolve(1, 5, first["portal"]); ok {
		t.Error("the portal is gone, its id must be stale")
	}
	MovementIds(1, &rooms.Room{RoomId: 6}, list("south"))
	if _, ok := Resolve(1, 5, first["northeast"]); ok {
		t.Error("after moving, an old per-room id must be stale")
	}
	if _, ok := Resolve(1, 6, first["northeast"]); ok {
		t.Error("an old per-room id must not resolve in the new room")
	}
}

// Abuse rules: an id never issued, or issued to someone else, is refused.
func TestResolve_RefusesIdsNeverIssuedToThisPlayer(t *testing.T) {
	t.Cleanup(func() { Forget(1); Forget(2) })
	theirs := MovementIds(2, &rooms.Room{RoomId: 5}, list("northeast"))
	MovementIds(1, &rooms.Room{RoomId: 5}, list("northeast"))
	if _, ok := Resolve(1, 5, "xdeadbeef"); ok {
		t.Error("an id never issued must be refused")
	}
	if _, ok := Resolve(1, 5, theirs["northeast"]); ok {
		t.Error("another player's id must be refused")
	}
	if _, ok := Resolve(1, 5, "north"); ok {
		t.Error("an exit name is not an id")
	}
}

// Ids reveal nothing: two players in the same room get different per-room
// ids, so an id says nothing about the room or the exit.
func TestMovementIds_TwoPlayersGetDifferentIds(t *testing.T) {
	t.Cleanup(func() { Forget(1); Forget(2) })
	a := MovementIds(1, &rooms.Room{RoomId: 5}, list("northeast"))
	b := MovementIds(2, &rooms.Room{RoomId: 5}, list("northeast"))
	if a["northeast"] == b["northeast"] {
		t.Fatalf("two players got the same per-room id %q", a["northeast"])
	}
}

// In the dark (no known exits) no per-room id survives.
func TestMovementIds_TheDarkIssuesNoPerRoomIds(t *testing.T) {
	t.Cleanup(func() { Forget(1) })
	lit := MovementIds(1, &rooms.Room{RoomId: 5}, list("northeast"))
	dark := MovementIds(1, &rooms.Room{RoomId: 5}, rooms.KnownExitList{Dark: true})
	if len(dark) != 0 {
		t.Fatalf("the dark issued %v", dark)
	}
	if _, ok := Resolve(1, 5, lit["northeast"]); ok {
		t.Error("a lit room's per-room id must be stale once the room is dark")
	}
}
```

Create `internal/buttons/darkmarks_test.go`:

```go
package buttons

import (
	"reflect"
	"testing"
)

// Ruling 4: a direction tried in the dark keeps its mark until the player
// leaves the room or the room is lit for them.
func TestDarkMarks(t *testing.T) {
	t.Cleanup(func() { Forget(1) })
	if !RecordDarkMark(1, 5, "north", MarkWall) {
		t.Fatal("a new mark must report a change")
	}
	if RecordDarkMark(1, 5, "north", MarkWall) {
		t.Error("the same mark again is no change")
	}
	if !RecordDarkMark(1, 5, "east", MarkLocked) {
		t.Fatal("a second mark must report a change")
	}
	if RecordDarkMark(1, 5, "northeast", MarkWall) {
		t.Error("only the six T directions are marked")
	}
	want := map[string]string{"north": MarkWall, "east": MarkLocked}
	if got := DarkMarks(1, 5, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("marks %v, want %v", got, want)
	}
	if got := DarkMarks(1, 6, true); len(got) != 0 {
		t.Fatalf("another room shows %v", got)
	}
	if got := DarkMarks(1, 5, true); len(got) != 0 {
		t.Fatalf("reading another room cleared nothing: %v", got)
	}

	RecordDarkMark(1, 5, "north", MarkWall)
	if got := DarkMarks(1, 5, false); len(got) != 0 {
		t.Fatalf("a lit room shows %v", got)
	}
	if got := DarkMarks(1, 5, true); len(got) != 0 {
		t.Fatalf("light must clear the marks, still %v", got)
	}

	RecordDarkMark(1, 5, "north", MarkWall)
	ClearDarkMarks(1)
	if got := DarkMarks(1, 5, true); len(got) != 0 {
		t.Fatalf("ClearDarkMarks left %v", got)
	}
}
```

Create `internal/refusal/button_test.go`:

```go
package refusal

import "testing"

func TestStaleButtonLine(t *testing.T) {
	if StaleButtonLine != `That button was out of date. Try again.` {
		t.Fatalf("StaleButtonLine = %q", StaleButtonLine)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/buttons/ ./internal/refusal/ -count=1`
Expected: FAIL, `no Go files in ...internal/buttons` / `undefined: StaleButtonLine`.

- [ ] **Step 3: Write `internal/buttons/buttons.go`**

```go
// Package buttons is the server side of the touch controls' buttons (#781,
// touch arc spec 2, the first slice of the button protocol). The client never
// builds a command: the server gives every button an opaque id, the client
// sends back only the id, and the server turns it into the line the player
// would have typed. Movement is the first slice: the six directions of the
// inverted T have fixed ids, the same in every room; every other exit gets an
// id issued to one player for one room. Dark marks, the directions a player
// tried in the dark, live here too (darkmarks.go). All state is in memory and
// per session: nothing here is living state.
package buttons

import (
	"crypto/rand"
	"encoding/hex"
	"maps"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// fixedIds are the T directions' ids (owner ruling 8). They are the same in
// every room and for every player, so they say nothing about where an exit is,
// and they are the only ids the dark offers.
var fixedIds = map[string]string{
	"north": "m1", "south": "m2", "east": "m3", "west": "m4", "up": "m5", "down": "m6",
}

var fixedDirections = func() map[string]string {
	out := make(map[string]string, len(fixedIds))
	for dir, id := range fixedIds {
		out[id] = dir
	}
	return out
}()

// FixedIds returns the T directions' ids, direction to id. Room.Info sends
// it in every room, dark or lit.
func FixedIds() map[string]string {
	return maps.Clone(fixedIds)
}

// issued is the per-room ids one player holds: the room they were issued
// for, and the exit name to id map both ways.
type issued struct {
	roomId int
	byName map[string]string
	byId   map[string]string
}

var (
	mu       sync.Mutex
	issuedTo = map[int]*issued{}
)

// MovementIds returns the id of every movement button for userId standing in
// room, keyed by exit name: a T direction's fixed id, any other known exit's
// per-room id. Per-room ids are issued here, replacing the player's last set:
// an exit that is still known in the same room keeps its id, so a resend does
// not make a tap stale. In the dark known lists no exits, so no per-room id
// survives (the fixed ids come from FixedIds).
func MovementIds(userId int, room *rooms.Room, known rooms.KnownExitList) map[string]string {
	ids := map[string]string{}
	others := []string{}
	for _, ke := range known.Exits {
		if _, dup := ids[ke.Name]; dup {
			continue
		}
		if id, fixed := fixedIds[ke.Name]; fixed {
			ids[ke.Name] = id
			continue
		}
		ids[ke.Name] = ``
		others = append(others, ke.Name)
	}
	maps.Copy(ids, issue(userId, room.RoomId, others))
	return ids
}

// issue gives each of names a per-room id for userId in roomId.
func issue(userId, roomId int, names []string) map[string]string {
	mu.Lock()
	defer mu.Unlock()
	prev := issuedTo[userId]
	next := &issued{roomId: roomId, byName: map[string]string{}, byId: map[string]string{}}
	for _, name := range names {
		id := ``
		if prev != nil && prev.roomId == roomId {
			id = prev.byName[name]
		}
		for id == `` || next.byId[id] != `` {
			id = newId()
		}
		next.byName[name] = id
		next.byId[id] = name
	}
	issuedTo[userId] = next
	return maps.Clone(next.byName)
}

// newId is an opaque per-room id: "x" and eight random hex digits. It encodes
// no room, destination or exit.
func newId() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b) // crypto/rand.Read never fails (Go 1.24+)
	return `x` + hex.EncodeToString(b)
}

// Resolve turns a pressed id into the line the player would have typed: a
// fixed id is its bare direction ("north"), in any room; a per-room id is
// "go <exit>", but only while the player stands in the room it was issued
// for. ok is false for an id never issued to this player, or issued for
// another room: the press is stale (owner ruling 7).
func Resolve(userId, roomId int, id string) (line string, ok bool) {
	if dir, fixed := fixedDirections[id]; fixed {
		return dir, true
	}
	mu.Lock()
	defer mu.Unlock()
	is := issuedTo[userId]
	if is == nil || is.roomId != roomId {
		return ``, false
	}
	name, ok := is.byId[id]
	if !ok {
		return ``, false
	}
	return `go ` + name, true
}

// Forget drops every id and dark mark held for userId (they left the game).
func Forget(userId int) {
	mu.Lock()
	defer mu.Unlock()
	delete(issuedTo, userId)
	delete(marksOf, userId)
}
```

- [ ] **Step 4: Write `internal/buttons/darkmarks.go`**

```go
package buttons

import "maps"

// The two dark marks (owner ruling 4): a direction tried in the dark that
// answered "You're bumping into walls." (MarkWall, drawn as a cross) or the
// locked line (MarkLocked, drawn as a padlock).
const (
	MarkWall   = `wall`
	MarkLocked = `locked`
)

type darkMarks struct {
	roomId int
	marks  map[string]string
}

// marksOf is guarded by mu (buttons.go).
var marksOf = map[int]*darkMarks{}

// RecordDarkMark notes that userId, seeing nothing in roomId, tried direction
// and was answered mark. Only the six T directions are marked: they are the
// only buttons the dark offers. A mark in a new room starts that room's set.
// It reports whether anything changed.
func RecordDarkMark(userId, roomId int, direction, mark string) bool {
	if _, ok := fixedIds[direction]; !ok {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	dm := marksOf[userId]
	if dm == nil || dm.roomId != roomId {
		dm = &darkMarks{roomId: roomId, marks: map[string]string{}}
		marksOf[userId] = dm
	}
	if dm.marks[direction] == mark {
		return false
	}
	dm.marks[direction] = mark
	return true
}

// DarkMarks returns userId's marks in roomId, direction to mark, while dark
// is true. When they stand in another room, or the room is not dark to them,
// the marks are cleared and none are returned: leaving the room or the room
// becoming lit clears them (ruling 4).
func DarkMarks(userId, roomId int, dark bool) map[string]string {
	mu.Lock()
	defer mu.Unlock()
	dm := marksOf[userId]
	if dm == nil {
		return map[string]string{}
	}
	if !dark || dm.roomId != roomId {
		delete(marksOf, userId)
		return map[string]string{}
	}
	return maps.Clone(dm.marks)
}

// ClearDarkMarks drops userId's marks: they changed rooms.
func ClearDarkMarks(userId int) {
	mu.Lock()
	defer mu.Unlock()
	delete(marksOf, userId)
}
```

- [ ] **Step 5: Write `internal/events/roomexits.go`**

```go
package events

import (
	"container/heap"
	"strconv"
)

// RoomExitsChanged says what a player's movement buttons show may have
// changed outside the once-a-round check (#781): a dark mark was recorded.
// modules/gmcp answers with Room.Info.
type RoomExitsChanged struct {
	UserId int
}

func (p RoomExitsChanged) Type() string { return `RoomExitsChanged` }

// UniqueID merges the refreshes queued for one player before the queue runs.
func (p RoomExitsChanged) UniqueID() string {
	return `RoomExitsChanged-` + strconv.Itoa(p.UserId)
}

// DrainQueuedRoomExitsChangedForTest removes the RoomExitsChanged events for
// userId from the global queue and returns them. Pass 0 to drain every one.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedRoomExitsChangedForTest(userId int) []RoomExitsChanged {
	qLock.Lock()
	defer qLock.Unlock()
	var found []RoomExitsChanged
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		m, ok := pe.event.(RoomExitsChanged)
		if ok && (userId == 0 || m.UserId == userId) {
			found = append(found, m)
			delete(uniqueMap, m.UniqueID())
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}
```

- [ ] **Step 6: Write `internal/refusal/button.go`**

```go
package refusal

// StaleButtonLine answers a touch button whose id the server did not issue
// to this player for the room they stand in (#781, owner ruling 7): their
// screen was a step behind. modules/gmcp re-sends Room.Info with it, so the
// next tap is current.
const StaleButtonLine = `That button was out of date. Try again.`

func StaleButton(to Recipient) { send(to, StaleButtonLine) }
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/buttons/ ./internal/refusal/ ./internal/events/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok`.

- [ ] **Step 8: Prove the tests can fail**

In `issue`, delete the `if prev != nil && prev.roomId == roomId { id = prev.byName[name] }` lines: `TestMovementIds_PerRoomIdsAreStableThenStale` FAILS ("a resend in the same room changed the ids"). Restore. In `Resolve` delete `|| is.roomId != roomId`: the same test FAILS on "after moving". Restore. In `DarkMarks` change `if !dark || dm.roomId != roomId {` to `if dm.roomId != roomId {`: `TestDarkMarks` FAILS ("a lit room shows"). Restore; PASS.

- [ ] **Step 9: Write `internal/buttons/context.md` and update the others**

`internal/buttons/context.md`:

```markdown
# buttons: Package Documentation

## Overview

The server side of the touch controls' buttons (#781, touch arc spec 2; the
first slice of the button protocol in `docs/roadmaps/TOUCH_ARC_ROADMAP.md`).
The client never builds a command: the server gives each button an opaque
id, the client sends back only the id, and the server turns it into the line
the player would have typed. Plan 3a widens this to every button.

## Movement ids

- `FixedIds()`: the six T directions' ids (`m1` north, `m2` south, `m3`
  east, `m4` west, `m5` up, `m6` down). The same in every room and for every
  player (owner ruling 8); the only ids the dark offers. `Room.Info` sends
  them as `moves`.
- `MovementIds(userId, room, rooms.KnownExits(...))`: each known exit's id
  by name; a T direction gets its fixed id, any other exit a per-room id
  (`x` and 8 random hex digits) issued to that player for that room. A
  resend in the same room keeps the ids of exits still known.
- `Resolve(userId, roomId, id)`: a fixed id is its bare direction
  (`north`) in any room; a per-room id is `go <exit>` only in the room it
  was issued for. `ok` false is a stale press (ruling 7); the caller sends
  `refusal.StaleButton` and re-sends `Room.Info`.
- `Forget(userId)`: drop everything (despawn).

## Dark marks

`RecordDarkMark(userId, roomId, direction, MarkWall|MarkLocked)` notes a T
direction tried in the dark (ruling 4); `usercommands.Go` calls it, so typed
and tapped tries both mark. `DarkMarks(userId, roomId, dark)` returns them
and clears them once the player is elsewhere or the room is not dark to
them; `ClearDarkMarks(userId)` on a room change. `Room.Info` sends them as
`darkmarks`.

## State

In memory, guarded by one mutex, per session: not living state
(`dogmud-persistence`). A restart or a logout forgets ids and marks; the
next `Room.Info` issues fresh ones.

## Callers

`modules/gmcp` (`gmcp.Room.go` builds the ids and marks into `Room.Info`,
`gmcp.ButtonPress.go` resolves presses, `gmcp.RoomExitsWatch.go` forgets on
despawn and clears marks on a room change); `internal/usercommands/go.go`
(dark marks).
```

`internal/events/context.md`: beside `npcmarkers.go`, add: `roomexits.go` holds `RoomExitsChanged{UserId}` (#781): a dark mark was recorded; `modules/gmcp` re-sends `Room.Info`. `UniqueID` merges refreshes for one player; `DrainQueuedRoomExitsChangedForTest`.

`internal/refusal/context.md`: add a helper row: `StaleButton` / `StaleButtonLine` ("That button was out of date. Try again."): a touch button id not issued to this player for this room (#781).

- [ ] **Step 10: Commit**

```bash
git add internal/buttons/ internal/events/roomexits.go internal/events/context.md internal/refusal/button.go internal/refusal/button_test.go internal/refusal/context.md
git commit -F - <<'EOF'
feat(buttons): movement button ids and dark marks (#781)

Fixed ids for the six T directions, per-room ids for other exits issued to
one player for one room, Resolve with stale refusal, and dark marks kept
per player and room. RoomExitsChanged and refusal.StaleButton.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 9: Dark marks recorded from typed and tapped tries

**Files:**
- Modify: `internal/usercommands/go.go` (after the locked line at 185; after the bump line at 615; a helper)
- Modify: `internal/usercommands/go_secret_exit_test.go` (append)
- Modify: `internal/usercommands/context.md`

- [ ] **Step 1: Write the failing test**

Append to `internal/usercommands/go_secret_exit_test.go` (add imports `"github.com/GoMudEngine/GoMud/internal/buttons"`, `"github.com/GoMudEngine/GoMud/internal/gamelock"`, `"github.com/GoMudEngine/GoMud/internal/messaging"`, `"github.com/GoMudEngine/GoMud/internal/util"`):

```go
// Ruling 4: a T direction tried in the dark is marked, typed or tapped (both
// run Go): a cross after the bump line, a padlock after the locked line, and
// Room.Info goes out at once. A lit room marks nothing.
func TestGo_DarkTriesLeaveMarks(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	util.SetRoundCountForTest(1000)
	t.Cleanup(util.ResetRoundCountForTest)
	user, room := getTestUserAndRoom(t)
	t.Cleanup(func() { buttons.Forget(user.UserId) })
	orig := room.Exits["north"]
	locked := orig
	locked.Lock = gamelock.Lock{Difficulty: 10}
	room.Exits["north"] = locked
	t.Cleanup(func() { room.Exits["north"] = orig })

	_, _ = Go("west", user, room, 0)
	require.Empty(t, buttons.DarkMarks(user.UserId, room.RoomId, true), "a lit bump marks nothing")

	darkenRoom(t, room)
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(user.Character, room), "premise: the room is dark")
	events.DrainQueuedRoomExitsChangedForTest(user.UserId)

	_, _ = Go("west", user, room, 0)
	_, _ = Go("north", user, room, 0)
	require.Equal(t, 1, user.Character.RoomId, "premise: the locked door held")
	require.Equal(t,
		map[string]string{"west": buttons.MarkWall, "north": buttons.MarkLocked},
		buttons.DarkMarks(user.UserId, room.RoomId, true))
	require.NotEmpty(t, events.DrainQueuedRoomExitsChangedForTest(user.UserId), "Room.Info must go out at once")
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/usercommands/ -run TestGo_DarkTriesLeaveMarks -count=1`
Expected: FAIL, the marks map is empty.

- [ ] **Step 3: Implement in `internal/usercommands/go.go`**

Add `"github.com/GoMudEngine/GoMud/internal/buttons"` to the imports. After `const PartyFollowLostLine = ...` add:

```go
// noteDarkTry records a dark mark when the player, seeing nothing here, tried
// direction and was answered mark (#781, owner ruling 4): a cross after
// "You're bumping into walls.", a padlock after the locked line. Typed and
// tapped tries both come through Go, so both mark. Room.Info carries the
// marks; RoomExitsChanged sends it now rather than at the next round.
func noteDarkTry(user *users.UserRecord, room *rooms.Room, direction, mark string) {
	if messaging.ParticipantSight(user.Character, room) != messaging.SightNone {
		return
	}
	if buttons.RecordDarkMark(user.UserId, room.RoomId, direction, mark) {
		events.AddToQueue(events.RoomExitsChanged{UserId: user.UserId})
	}
}
```

Directly after the locked line (line 185, `user.SendText(messaging.CategoryRefusal, `That way is locked. ...`)`), add:

```go
					noteDarkTry(user, room, exitName, buttons.MarkLocked)
```

Directly after the bump line (line 615, `user.SendText(messaging.CategoryRefusal, "You're bumping into walls.")`), add:

```go
			noteDarkTry(user, room, rest, buttons.MarkWall)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/usercommands/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok`.

- [ ] **Step 5: Prove the test can fail**

In `noteDarkTry` change `!= messaging.SightNone` to `== messaging.SightNone`: FAIL (the lit bump marks, the dark ones do not). Restore; PASS.

- [ ] **Step 6: Update `internal/usercommands/context.md`**

Add to the movement notes: in the dark, a T direction that answers "You're bumping into walls." or the locked line is recorded as a dark mark (`noteDarkTry`, `buttons.RecordDarkMark`, #781 ruling 4), typed or tapped, and `events.RoomExitsChanged` re-sends `Room.Info`.

- [ ] **Step 7: Commit**

```bash
git add internal/usercommands/go.go internal/usercommands/go_secret_exit_test.go internal/usercommands/context.md
git commit -F - <<'EOF'
feat(usercommands): dark tries leave marks (#781)

A T direction tried in the dark records a wall or a padlock mark, typed or
tapped, and Room.Info goes out at once.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: `Room.Info` built from `KnownExits`, with kinds, ids, dark and marks

**Files:**
- Modify: `modules/gmcp/gmcp.Room.go` (exits block 478-542; structs 601-621; a builder)
- Create: `modules/gmcp/gmcp.RoomExits_test.go`
- Modify: `modules/gmcp/gmcp_wire_freeze_test.go`
- Modify: `modules/gmcp/context.md`

- [ ] **Step 1: Write the failing tests**

Create `modules/gmcp/gmcp.RoomExits_test.go`:

```go
package gmcp

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

var perRoomIdInJSON = regexp.MustCompile(`"x[0-9a-f]{8}"`)

// exitsFixture is roomSightFixture's cellar (9720, north to 9721) at lamp,
// given one more exit of every kind: up, northeast, out, a secret west and a
// temporary portal.
func exitsFixture(t *testing.T, lamp int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	viewer := roomSightFixture(t, lamp)
	t.Cleanup(func() { buttons.Forget(viewer.UserId) })
	room := rooms.LoadRoom(9720)
	room.Exits["up"] = exit.RoomExit{RoomId: 9721}
	room.Exits["northeast"] = exit.RoomExit{RoomId: 9721}
	room.Exits["out"] = exit.RoomExit{RoomId: 9721}
	room.Exits["west"] = exit.RoomExit{RoomId: 9721, Secret: true}
	require.True(t, room.AddTemporaryExit("portal", exit.TemporaryRoomExit{RoomId: 9721}))
	return viewer, room
}

func payloadJSON(t *testing.T, p GMCPRoomModule_Payload) string {
	t.Helper()
	b, err := json.Marshal(p)
	require.NoError(t, err)
	return string(b)
}

// Guard 5 (sight) and "nothing leaks in the dark": dark is set, both exit
// maps are present and empty, and the only ids are the fixed T ids, which are
// the same everywhere.
func TestRoomInfo_TheDarkLeaksNoExitAndOffersOnlyTheFixedIds(t *testing.T) {
	viewer, _ := exitsFixture(t, 0)
	viewer.Character.LearnSecretExit(9720, "west")
	p := roomInfoFor(t, viewer)
	require.True(t, p.Dark)
	require.NotNil(t, p.Exits)
	require.Empty(t, p.Exits)
	require.NotNil(t, p.ExitsV2)
	require.Empty(t, p.ExitsV2)
	require.Equal(t, buttons.FixedIds(), p.Moves)
	js := payloadJSON(t, p)
	require.False(t, perRoomIdInJSON.MatchString(js), "the dark must carry no per-room id: %s", js)
	for _, leak := range []string{"northeast", "portal", `"out"`} {
		require.NotContains(t, js, leak)
	}
}

// Ruling 1 and the kinds and ids of every exit: an unfound secret exit is
// never sent; T directions carry fixed ids, others per-room ids that resolve
// to their exit.
func TestRoomInfo_KindsIdsAndOnlyKnownExits(t *testing.T) {
	viewer, _ := exitsFixture(t, 60)
	p := roomInfoFor(t, viewer)
	require.False(t, p.Dark)
	require.NotContains(t, p.Exits, "west")
	require.NotContains(t, p.ExitsV2, "west")
	for name, kind := range map[string]string{
		"north": "compass", "up": "vertical", "northeast": "diagonal", "out": "named", "portal": "temporary",
	} {
		require.Equal(t, kind, p.ExitsV2[name].Kind, name)
	}
	require.Equal(t, "m1", p.ExitsV2["north"].Id)
	require.Equal(t, "m5", p.ExitsV2["up"].Id)
	for _, name := range []string{"northeast", "out", "portal"} {
		id := p.ExitsV2[name].Id
		require.Regexp(t, `^x[0-9a-f]{8}$`, id, name)
		line, ok := buttons.Resolve(viewer.UserId, 9720, id)
		require.True(t, ok, name)
		require.Equal(t, "go "+name, line)
	}
	require.Equal(t, 9721, p.Exits["portal"])

	viewer.Character.LearnSecretExit(9720, "west")
	p = roomInfoFor(t, viewer)
	require.Contains(t, p.ExitsV2["west"].Details, "secret")
	require.Equal(t, "m4", p.ExitsV2["west"].Id)
}

// Ruling 3: `locked` follows the door's real state: lock, unlock, and a
// relock on its timer.
func TestRoomInfo_LockedMeansLockedNow(t *testing.T) {
	util.SetRoundCountForTest(1000)
	t.Cleanup(util.ResetRoundCountForTest)
	viewer, room := exitsFixture(t, 60)
	north := room.Exits["north"]
	north.Lock = gamelock.Lock{Difficulty: 5}
	room.Exits["north"] = north

	require.Contains(t, roomInfoFor(t, viewer).ExitsV2["north"].Details, "locked")
	room.SetExitLock("north", false)
	require.NotContains(t, roomInfoFor(t, viewer).ExitsV2["north"].Details, "locked", "unlocked now")
	util.SetRoundCountForTest(gametime.GetDate(1000).AddPeriod(gamelock.DefaultRelockTime))
	require.Contains(t, roomInfoFor(t, viewer).ExitsV2["north"].Details, "locked", "relocked on its timer")
}

// Ruling 4: the dark marks ride in Room.Info while the room is dark.
func TestRoomInfo_DarkMarksRideOnlyInTheDark(t *testing.T) {
	viewer, room := exitsFixture(t, 0)
	buttons.RecordDarkMark(viewer.UserId, 9720, "north", buttons.MarkWall)
	require.Equal(t, map[string]string{"north": buttons.MarkWall}, roomInfoFor(t, viewer).DarkMarks)
	room.Lamp = rooms.LampPtr(60)
	p := roomInfoFor(t, viewer)
	require.NotNil(t, p.DarkMarks)
	require.Empty(t, p.DarkMarks)
}
```

Append to `TestWireFreeze_GMCPJSONFieldNames` in `modules/gmcp/gmcp_wire_freeze_test.go`:

```go
	// Room.Info (#781): the touch controls read these.
	for _, k := range []string{"num", "exits", "exitsv2", "dark", "moves", "darkmarks"} {
		assert.Contains(t, keysOf(GMCPRoomModule_Payload{}), k)
	}
	for _, k := range []string{"num", "dx", "dy", "dz", "details", "kind", "id"} {
		assert.Contains(t, keysOf(GMCPRoomModule_Payload_Contents_ExitInfo{}), k)
	}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./modules/gmcp/ -run 'TestRoomInfo_|TestWireFreeze' -count=1`
Expected: FAIL to compile (`p.Dark undefined`, `p.Moves undefined`, `.Kind undefined`).

- [ ] **Step 3: Extend the structs in `gmcp.Room.go`**

`GMCPRoomModule_Payload` gains, after `Details`:

```go
	Dark        bool                                                `json:"dark"`      // the viewer sees nothing here (#781); exits and exitsv2 are then empty
	Moves       map[string]string                                   `json:"moves"`     // the six T directions to their fixed button ids, in every room (#781)
	DarkMarks   map[string]string                                   `json:"darkmarks"` // T direction to "wall" or "locked", tried in the dark (#781, ruling 4)
```

`GMCPRoomModule_Payload_Contents_ExitInfo` gains, after `Details`:

```go
	Kind    string   `json:"kind"` // compass, vertical, diagonal, named or temporary (rooms.ExitKind)
	Id      string   `json:"id"`   // the movement button's id (internal/buttons)
```

- [ ] **Step 4: Add the builder and use it**

Add `"github.com/GoMudEngine/GoMud/internal/buttons"` to the imports. Above `func (g *GMCPRoomModule) GetRoomNode(`, add:

```go
// roomExitsInfo is Room.Info's exit half for one viewer (#781): the exits
// they know here (rooms.KnownExits, the one rule the exits line and the maps
// read too), each with its kind, whether it is locked now and its button id;
// the dark flag; the T directions' fixed ids; and the dark marks. GetRoomNode
// sends it and the round watcher compares it, so the two cannot disagree
// about what "changed" means.
type roomExitsInfo struct {
	Exits     map[string]int
	ExitsV2   map[string]GMCPRoomModule_Payload_Contents_ExitInfo
	Dark      bool
	Moves     map[string]string
	DarkMarks map[string]string
}

func buildRoomExitsInfo(user *users.UserRecord, room *rooms.Room) roomExitsInfo {
	known := rooms.KnownExits(room, user.Character)
	info := roomExitsInfo{
		Exits:     map[string]int{},
		ExitsV2:   map[string]GMCPRoomModule_Payload_Contents_ExitInfo{},
		Dark:      known.Dark,
		Moves:     buttons.FixedIds(),
		DarkMarks: buttons.DarkMarks(user.UserId, room.RoomId, known.Dark),
	}
	ids := buttons.MovementIds(user.UserId, room, known)
	for _, ke := range known.Exits {
		// A temporary exit named like a permanent one is reached by the
		// permanent one's name (Room.FindExitByName), so it is listed once.
		if _, dup := info.ExitsV2[ke.Name]; dup {
			continue
		}
		info.Exits[ke.Name] = ke.RoomId

		direction := ke.Name
		if ke.Exit.MapDirection != `` {
			direction = ke.Exit.MapDirection
		}
		deltaX, deltaY, deltaZ := mapper.GetDelta(direction)
		v2 := GMCPRoomModule_Payload_Contents_ExitInfo{
			RoomId:  ke.RoomId,
			DeltaX:  deltaX,
			DeltaY:  deltaY,
			DeltaZ:  deltaZ,
			Details: []string{},
			Kind:    string(ke.Kind),
			Id:      ids[ke.Name],
		}
		if ke.Exit.Secret {
			v2.Details = append(v2.Details, `secret`)
		}
		// Locked now, not "has a lock" (owner ruling 3).
		if ke.LockedNow {
			v2.Details = append(v2.Details, `locked`)
		}
		if ke.Exit.HasLock() {
			lockId := fmt.Sprintf(`%d-%s`, room.RoomId, ke.Name)
			haskey, hascombo := user.Character.HasKey(lockId, int(ke.Exit.Lock.Difficulty))
			if haskey {
				v2.Details = append(v2.Details, `player_has_key`)
			}
			if hascombo {
				v2.Details = append(v2.Details, `player_has_pick_combo`)
			}
		}
		info.ExitsV2[ke.Name] = v2
	}
	return info
}
```

In `GetRoomNode`, replace from `// set exits` (line 478) through `// end exits` (line 542) with:

```go
		// Exits: what the viewer knows here and the touch buttons' ids
		// (#781). In the dark both maps are present and empty, so a client
		// merging payloads drops the last sighted exits (the Char.Enemies
		// rule).
		exitsInfo := buildRoomExitsInfo(user, room)
		payload.Exits = exitsInfo.Exits
		payload.ExitsV2 = exitsInfo.ExitsV2
		payload.Dark = exitsInfo.Dark
		payload.Moves = exitsInfo.Moves
		payload.DarkMarks = exitsInfo.DarkMarks
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./modules/gmcp/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok`, including the existing `gmcp.RoomSight_test.go` (the dark test still sees empty, non-nil maps).

- [ ] **Step 6: Prove the tests can fail**

In `buildRoomExitsInfo` replace `if ke.LockedNow {` with `if ke.Exit.HasLock() {` (the old rule): `TestRoomInfo_LockedMeansLockedNow` FAILS on "unlocked now". Restore. Replace `Id: ids[ke.Name],` with `Id: ke.Name,`: `TestRoomInfo_KindsIdsAndOnlyKnownExits` FAILS. Restore; PASS.

- [ ] **Step 7: Update `modules/gmcp/context.md`**

In the `Room.Info` section, add:

```markdown
`Room.Info` exits come from `rooms.KnownExits` through `buildRoomExitsInfo`
(#781): only exits the viewer knows (a secret exit once found), room,
mutator and temporary exits. Each `exitsv2` entry has `kind` (`compass`,
`vertical`, `diagonal`, `named`, `temporary`), `id` (the movement button id,
`internal/buttons`) and `details` (`secret`; `locked` means locked now;
`player_has_key`, `player_has_pick_combo` on any lock). Top level: `dark`
(the viewer sees nothing; the exit maps are then empty), `moves` (the six T
directions to their fixed ids, always sent) and `darkmarks` (T direction to
`wall` or `locked`).
```

- [ ] **Step 8: Commit**

```bash
git add modules/gmcp/gmcp.Room.go modules/gmcp/gmcp.RoomExits_test.go modules/gmcp/gmcp_wire_freeze_test.go modules/gmcp/context.md
git commit -F - <<'EOF'
feat(gmcp): Room.Info lists known exits with kinds, ids and dark marks (#781)

Room.Info's exits come from rooms.KnownExits: secret exits only once
found, temporary and mutator exits, locked meaning locked now. Each exit
carries its kind and button id; the payload carries dark, the fixed T ids
and the dark marks.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 11: The `Room.Info` watcher

**Files:**
- Create: `modules/gmcp/gmcp.RoomExitsWatch.go`, `modules/gmcp/gmcp.RoomExitsWatch_test.go`
- Modify: `modules/gmcp/gmcp.Room.go` (`init`, `despawnHandler`, `roomChangeHandler`, `buildAndSendGMCPPayload`)
- Modify: `modules/gmcp/context.md`

- [ ] **Step 1: Write the failing tests**

Create `modules/gmcp/gmcp.RoomExitsWatch_test.go`:

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// watchedViewer is exitsFixture with the viewer on a GMCP connection and no
// record of a Room.Info sent to them.
func watchedViewer(t *testing.T, lamp int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	viewer, room := exitsFixture(t, lamp)
	gmcpModule.cache.Add(viewer.ConnectionId(), GMCPSettings{GMCPAccepted: true, HelloReceived: true})
	t.Cleanup(func() { gmcpModule.cache.Remove(viewer.ConnectionId()) })
	forgetSentRoomExits(viewer.UserId)
	t.Cleanup(func() { forgetSentRoomExits(viewer.UserId) })
	events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`)
	events.DrainQueuedByTypeForTest(`GMCPOut`)
	t.Cleanup(func() {
		events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`)
		events.DrainQueuedByTypeForTest(`GMCPOut`)
	})
	return viewer, room
}

// sendRoomInfo sends viewer a Room.Info as the queue would.
func sendRoomInfo(viewer *users.UserRecord) {
	(&GMCPRoomModule{}).buildAndSendGMCPPayload(GMCPRoomUpdate{UserId: viewer.UserId, Identifier: `Room.Info`})
	events.DrainQueuedByTypeForTest(`GMCPOut`)
}

// roomInfoUpdates drains the queued Room.Info requests and counts userId's.
func roomInfoUpdates(userId int) int {
	n := 0
	for _, e := range events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`) {
		if u, ok := e.(GMCPRoomUpdate); ok && u.UserId == userId && u.Identifier == `Room.Info` {
			n++
		}
	}
	return n
}

// "Kept current": the watcher re-sends Room.Info on a real change (a door
// unlocked, relocking on its timer, a gangplank coming and going, a secret
// exit found, a light going out) and only then.
func TestRoomExitsWatch_ResendsOnlyOnARealChange(t *testing.T) {
	util.SetRoundCountForTest(1000)
	t.Cleanup(util.ResetRoundCountForTest)
	viewer, room := watchedViewer(t, 60)
	north := room.Exits["north"]
	north.Lock = gamelock.Lock{Difficulty: 5}
	room.Exits["north"] = north
	g := &GMCPRoomModule{}

	sendRoomInfo(viewer)
	g.roomExitsWatchHandler(events.NewRound{})
	require.Zero(t, roomInfoUpdates(viewer.UserId), "an unchanged round sends nothing")

	steps := []struct {
		name   string
		change func()
	}{
		{"a door unlocked", func() { room.SetExitLock("north", false) }},
		{"the door relocked on its timer", func() {
			util.SetRoundCountForTest(gametime.GetDate(1000).AddPeriod(gamelock.DefaultRelockTime))
		}},
		{"a gangplank came", func() { room.AddTemporaryExit("gangplank", exit.TemporaryRoomExit{RoomId: 9721}) }},
		{"the gangplank went", func() { room.RemoveTemporaryExit(room.ExitsTemp["gangplank"]) }},
		{"a secret exit found", func() { viewer.Character.LearnSecretExit(9720, "west") }},
		{"the light went out", func() { room.Lamp = nil }},
	}
	for _, s := range steps {
		s.change()
		g.roomExitsWatchHandler(events.NewRound{})
		require.Equal(t, 1, roomInfoUpdates(viewer.UserId), s.name)
		sendRoomInfo(viewer)
		g.roomExitsWatchHandler(events.NewRound{})
		require.Zero(t, roomInfoUpdates(viewer.UserId), "%s: once sent, not repeated", s.name)
	}
	require.Zero(t, roomInfoUpdates(9723), "Kesh has no GMCP connection and is never sent one")
}

// A dark mark sends Room.Info at once, not at the next round.
func TestRoomExitsChanged_SendsRoomInfoAtOnce(t *testing.T) {
	viewer, _ := watchedViewer(t, 0)
	(&GMCPRoomModule{}).roomExitsChangedHandler(events.RoomExitsChanged{UserId: viewer.UserId})
	require.Equal(t, 1, roomInfoUpdates(viewer.UserId))
}

// Ruling 4: leaving the room clears the marks, so coming back shows none.
func TestRoomChange_ClearsDarkMarks(t *testing.T) {
	viewer, _ := watchedViewer(t, 0)
	buttons.RecordDarkMark(viewer.UserId, 9720, "north", buttons.MarkWall)
	(&GMCPRoomModule{}).roomChangeHandler(events.RoomChange{UserId: viewer.UserId, FromRoomId: 9720, ToRoomId: 9721})
	require.Empty(t, buttons.DarkMarks(viewer.UserId, 9720, true))
}

// A despawn forgets the player's ids and the record of what was sent.
func TestRoomExitsWatch_DespawnForgets(t *testing.T) {
	viewer, _ := watchedViewer(t, 60)
	p := roomInfoFor(t, viewer)
	id := p.ExitsV2["northeast"].Id
	sendRoomInfo(viewer)
	(&GMCPRoomModule{}).despawnHandler(events.PlayerDespawn{UserId: viewer.UserId, RoomId: 9720, CharacterName: "Viewer"})
	_, ok := buttons.Resolve(viewer.UserId, 9720, id)
	require.False(t, ok, "a despawned player's per-room ids are gone")
	require.True(t, roomExitsDue(viewer), "nothing is recorded as sent any more")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./modules/gmcp/ -run 'TestRoomExitsWatch|TestRoomExitsChanged|TestRoomChange_ClearsDarkMarks' -count=1`
Expected: FAIL to compile (`undefined: forgetSentRoomExits`, `roomExitsWatchHandler`, ...).

- [ ] **Step 3: Write `modules/gmcp/gmcp.RoomExitsWatch.go`**

```go
package gmcp

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Room.Info kept current (#781, spec "Kept current").
//
// Room.Info goes out on a move and on a light change. A door unlocked or
// relocking on its timer (gamelock relocks lazily, by a time compare, and a
// dozen call sites change locks with no event), a gangplank or portal coming
// and going, a secret exit found, and a light going out all change what the
// touch buttons must show without either. Rather than touch every one of
// those writers, the module checks once a round, as the Char.Vitals watcher
// does (gmcp.CharVitalsWatch.go): it builds the exit half each GMCP client
// would be sent (buildRoomExitsInfo) and, when its signature differs from the
// last one actually sent to that user, re-sends Room.Info. A round with no
// change sends nothing. A dark mark does not wait for the round:
// events.RoomExitsChanged sends it at once.

var (
	lastSentRoomExits   = map[int]string{}
	lastSentRoomExitsMu sync.Mutex
)

// roomExitsSignature is everything the touch buttons show, as one comparable
// string: the room, the dark flag, each exit's name, kind, id, destination and
// details, and the dark marks. Map order is sorted away.
func roomExitsSignature(roomId int, dark bool, exitsV2 map[string]GMCPRoomModule_Payload_Contents_ExitInfo, marks map[string]string) string {
	parts := []string{strconv.Itoa(roomId), strconv.FormatBool(dark)}
	for _, name := range slices.Sorted(maps.Keys(exitsV2)) {
		v := exitsV2[name]
		parts = append(parts, name+`|`+v.Kind+`|`+v.Id+`|`+strconv.Itoa(v.RoomId)+`|`+strings.Join(v.Details, `,`))
	}
	for _, dir := range slices.Sorted(maps.Keys(marks)) {
		parts = append(parts, `mark|`+dir+`|`+marks[dir])
	}
	return strings.Join(parts, "\n")
}

func recordSentRoomExits(userId int, signature string) {
	lastSentRoomExitsMu.Lock()
	lastSentRoomExits[userId] = signature
	lastSentRoomExitsMu.Unlock()
}

func forgetSentRoomExits(userId int) {
	lastSentRoomExitsMu.Lock()
	delete(lastSentRoomExits, userId)
	lastSentRoomExitsMu.Unlock()
}

// roomExitsDue reports whether what u's buttons should show differs from the
// last Room.Info sent to them. Never sent is due.
func roomExitsDue(u *users.UserRecord) bool {
	room := rooms.LoadRoom(u.Character.RoomId)
	if room == nil {
		return false
	}
	now := buildRoomExitsInfo(u, room)
	sig := roomExitsSignature(room.RoomId, now.Dark, now.ExitsV2, now.DarkMarks)
	lastSentRoomExitsMu.Lock()
	last, ok := lastSentRoomExits[u.UserId]
	lastSentRoomExitsMu.Unlock()
	return !ok || last != sig
}

// roomExitsWatchHandler runs each round and re-sends Room.Info to every GMCP
// client whose exits, locks, dark flag or dark marks moved since the last
// one. Telnet users without GMCP cost one map lookup.
func (g *GMCPRoomModule) roomExitsWatchHandler(e events.Event) events.ListenerReturn {
	if _, ok := e.(events.NewRound); !ok {
		return events.Continue
	}
	for _, u := range users.GetAllActiveUsers() {
		if u.UserId == 0 || u.Character == nil || !isGMCPEnabled(u.ConnectionId()) {
			continue
		}
		if roomExitsDue(u) {
			events.AddToQueue(GMCPRoomUpdate{UserId: u.UserId, Identifier: `Room.Info`})
		}
	}
	return events.Continue
}

// roomExitsChangedHandler sends Room.Info now: a dark mark was recorded.
func (g *GMCPRoomModule) roomExitsChangedHandler(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.RoomExitsChanged)
	if !ok || evt.UserId < 1 {
		return events.Continue
	}
	user := users.GetByUserId(evt.UserId)
	if user == nil || !isGMCPEnabled(user.ConnectionId()) {
		return events.Continue
	}
	events.AddToQueue(GMCPRoomUpdate{UserId: evt.UserId, Identifier: `Room.Info`})
	return events.Continue
}
```

- [ ] **Step 4: Wire it in `gmcp.Room.go`**

In `init()`, after `events.RegisterListener(events.NpcMarkersChanged{}, g.npcMarkersChangedHandler)`, add:

```go
	// Room.Info kept current for the touch buttons (#781): see
	// gmcp.RoomExitsWatch.go.
	events.RegisterListener(events.NewRound{}, g.roomExitsWatchHandler)
	events.RegisterListener(events.RoomExitsChanged{}, g.roomExitsChangedHandler)
```

Add `"github.com/GoMudEngine/GoMud/internal/buttons"` if not already imported (task 10 added it). In `despawnHandler`, directly after

```go
	if evt.UserId == 0 {
		return events.Continue
	}
```

add:

```go
	// The player left the game: their button ids, dark marks and the record
	// of the last Room.Info sent go with them (#781).
	buttons.Forget(evt.UserId)
	forgetSentRoomExits(evt.UserId)
```

In `roomChangeHandler`, directly after

```go
	if evt.UserId == 0 {
		return events.Continue
	}
```

add:

```go
	// Leaving a room clears its dark marks (#781, owner ruling 4).
	buttons.ClearDarkMarks(evt.UserId)
```

In `buildAndSendGMCPPayload`, replace:

```go
			payload, moduleName := g.GetRoomNode(user, requestedId)
```

with:

```go
			payload, moduleName := g.GetRoomNode(user, requestedId)
			// What the touch buttons were just sent, for the round watcher
			// (gmcp.RoomExitsWatch.go).
			if p, ok := payload.(GMCPRoomModule_Payload); ok && moduleName == `Room.Info` {
				recordSentRoomExits(evt.UserId, roomExitsSignature(p.Id, p.Dark, p.ExitsV2, p.DarkMarks))
			}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./modules/gmcp/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok`, including `gmcp.CharVitalsWatch_test.go`.

- [ ] **Step 6: Prove the tests can fail**

Remove the `recordSentRoomExits(...)` lines from `buildAndSendGMCPPayload`: `TestRoomExitsWatch_ResendsOnlyOnARealChange` FAILS at "an unchanged round sends nothing". Restore. Remove `buttons.ClearDarkMarks(evt.UserId)`: `TestRoomChange_ClearsDarkMarks` FAILS. Restore; PASS.

- [ ] **Step 7: Update `modules/gmcp/context.md`**

Add to the `Room.Info` section: kept current by `gmcp.RoomExitsWatch.go`: once a round each GMCP user's exit half (`buildRoomExitsInfo`) is compared by `roomExitsSignature` with the last one sent (`lastSentRoomExits`, recorded in `buildAndSendGMCPPayload`), and `Room.Info` is re-sent only on a change (doors, timed relocks, temporary exits, found secret exits, light). `events.RoomExitsChanged` (a dark mark) sends it at once. A room change clears dark marks; a despawn forgets ids, marks and the record.

- [ ] **Step 8: Commit**

```bash
git add modules/gmcp/gmcp.RoomExitsWatch.go modules/gmcp/gmcp.RoomExitsWatch_test.go modules/gmcp/gmcp.Room.go modules/gmcp/context.md
git commit -F - <<'EOF'
feat(gmcp): keep Room.Info current for the touch buttons (#781)

A once-a-round watcher re-sends Room.Info when a player's known exits,
locks, dark flag or dark marks change, and only then. A dark mark sends it
at once; a room change clears the marks.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 12: The press handler (`Char.Button.Press`) and guard 1

**Files:**
- Modify: `modules/gmcp/gmcp.go` (the `GMCPCharOp` case list, about line 409)
- Modify: `modules/gmcp/gmcp.CharOp.go` (a case in `handleCharOp`)
- Create: `modules/gmcp/gmcp.ButtonPress.go`, `modules/gmcp/gmcp.ButtonPress_test.go`
- Create: `internal/usercommands/button_roundtrip_test.go`
- Modify: `modules/gmcp/context.md`

- [ ] **Step 1: Write the failing tests**

Create `modules/gmcp/gmcp.ButtonPress_test.go`:

```go
package gmcp

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// queuedInputs drains userId's queued input events.
func queuedInputs(userId int) []events.Input {
	var out []events.Input
	for _, e := range events.DrainQueuedByTypeForTest(`Input`) {
		if in, ok := e.(events.Input); ok && in.UserId == userId && in.MobInstanceId == 0 {
			out = append(out, in)
		}
	}
	return out
}

func pressFixture(t *testing.T) (*users.UserRecord, map[string]string) {
	t.Helper()
	viewer, room := exitsFixture(t, 60)
	ids := buttons.MovementIds(viewer.UserId, room, rooms.KnownExits(room, viewer.Character))
	queuedInputs(viewer.UserId)
	events.DrainQueuedMessagesForTest(viewer.UserId)
	events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`)
	return viewer, ids
}

// Ruling 7: a press goes through the normal input queue as the typed line,
// at the current turn (the one-command-per-turn limit applies; a ReadyTurn at
// or below 0 would bypass it, so the turn count is moved off 0 first; there
// is no setter, and one more turn harms no other test), flagged typed so a
// menu or the pager treats it as typing.
func TestPressButton_QueuesTheTypedLine(t *testing.T) {
	if util.GetTurnCount() == 0 {
		util.IncrementTurnCount()
	}
	viewer, ids := pressFixture(t)

	pressButton(viewer, "m1")
	pressButton(viewer, ids["northeast"])
	got := queuedInputs(viewer.UserId)
	require.Len(t, got, 2)
	require.Equal(t, "north", got[0].InputText)
	require.Equal(t, "go northeast", got[1].InputText)
	for _, in := range got {
		require.True(t, in.Flags.Has(events.CmdTyped), "a press is typed input")
		require.Equal(t, util.GetTurnCount(), in.ReadyTurn, "a press waits its turn like typing")
		require.NotZero(t, in.ReadyTurn, "ReadyTurn 0 would bypass the per-turn limit")
	}
}

// Abuse rules: an id never issued, another player's id, and an old per-room
// id after moving are all refused as stale, and Room.Info is sent again.
func TestPressButton_RefusesStaleIds(t *testing.T) {
	viewer, ids := pressFixture(t)
	kesh := users.GetByUserId(9723)
	keshIds := buttons.MovementIds(kesh.UserId, rooms.LoadRoom(9720), rooms.KnownExits(rooms.LoadRoom(9720), kesh.Character))
	t.Cleanup(func() { buttons.Forget(kesh.UserId) })

	for name, id := range map[string]string{
		"never issued":        "xdeadbeef",
		"another player's id": keshIds["northeast"],
		"an exit name":        "northeast",
	} {
		pressButton(viewer, id)
		require.Empty(t, queuedInputs(viewer.UserId), name)
		require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(viewer.UserId), "\n"), refusal.StaleButtonLine, name)
		require.Len(t, events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`), 1, "%s: Room.Info is sent again", name)
	}

	viewer.Character.RoomId = 9721 // moved, no Room.Info built for the new room yet
	pressButton(viewer, ids["northeast"])
	require.Empty(t, queuedInputs(viewer.UserId), "an old per-room id after moving")
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(viewer.UserId), "\n"), refusal.StaleButtonLine)
}

// A press never answers a prompt.
func TestPressButton_DoesNothingWhileAPromptIsOpen(t *testing.T) {
	viewer, _ := pressFixture(t)
	_, ok := viewer.StartPrompt("mail", "Bob")
	require.True(t, ok)
	t.Cleanup(viewer.ClearPrompt)
	pressButton(viewer, "m1")
	require.Empty(t, queuedInputs(viewer.UserId))
	require.Empty(t, events.DrainQueuedMessagesForTest(viewer.UserId))
}

// The wire: Char.Button.Press reaches pressButton on MainWorker.
func TestHandleCharOp_ButtonPress(t *testing.T) {
	viewer, _ := pressFixture(t)
	gmcpModule.handleCharOp(GMCPCharOp{
		ConnectionId: viewer.ConnectionId(),
		Command:      `Char.Button.Press`,
		Payload:      []byte(`{"id":"m2"}`),
	})
	got := queuedInputs(viewer.UserId)
	require.Len(t, got, 1)
	require.Equal(t, "south", got[0].InputText)
}
```

Create `internal/usercommands/button_roundtrip_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// Guard 1 of the touch arc (round trip), movement slice (#781): every
// movement id the server issues, in a room holding a compass, a vertical, a
// diagonal, a named, a temporary, a locked and a found secret exit, and the
// six fixed ids the dark offers, becomes a line the real parser accepts and
// that reaches the exit it was issued for.
func TestButtons_EveryIssuedMovementIdRoundTrips(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	t.Cleanup(func() { buttons.Forget(user.UserId) })
	origExits, origTemp := room.Exits, room.ExitsTemp
	room.Exits = map[string]exit.RoomExit{
		"north":     {RoomId: 2},
		"up":        {RoomId: 2},
		"northeast": {RoomId: 2},
		"out":       {RoomId: 2},
		"east":      {RoomId: 2, Lock: gamelock.Lock{Difficulty: 10}},
		"west":      {RoomId: 2, Secret: true},
	}
	room.ExitsTemp = nil
	t.Cleanup(func() { room.Exits, room.ExitsTemp = origExits, origTemp })
	user.Character.LearnSecretExit(1, "west")
	require.True(t, room.AddTemporaryExit("portal", exit.TemporaryRoomExit{RoomId: 2}))

	known := rooms.KnownExits(room, user.Character)
	require.False(t, known.Dark, "premise: room 1 is lit")
	ids := buttons.MovementIds(user.UserId, room, known)
	require.Len(t, ids, 7)
	for name, id := range ids {
		line, ok := buttons.Resolve(user.UserId, room.RoomId, id)
		require.True(t, ok, "%s: id %q must resolve", name, id)
		require.True(t, isKnownCommandLine(user, room, line), "%s: %q must be a line the parser accepts", name, line)
		verb, rest := SplitInput(line)
		word := verb
		if verb == "go" {
			word = rest
		}
		got, _ := room.FindKnownExitByName(user.Character, word)
		require.Equal(t, name, got, "%s: %q must reach the exit it was issued for", name, line)
	}
	for dir, id := range buttons.FixedIds() {
		line, ok := buttons.Resolve(user.UserId, room.RoomId, id)
		require.True(t, ok)
		require.Equal(t, dir, line)
		require.True(t, isKnownCommandLine(user, room, line), "the dark's %s must parse", dir)
	}
}

// The round trip can fail: a line the parser does not take, and a word that
// reaches no exit, are caught by the same checks.
func TestButtons_RoundTripGuardCanFail(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	require.False(t, isKnownCommandLine(user, room, "gox north"))
	got, _ := room.FindKnownExitByName(user.Character, "southwest")
	require.NotEqual(t, "southwest", got)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./modules/gmcp/ -run 'TestPressButton|TestHandleCharOp_ButtonPress' -count=1; go test ./internal/usercommands/ -run TestButtons_ -count=1`
Expected: gmcp fails to compile (`undefined: pressButton`); the usercommands round trip PASSES already (it exercises tasks 2, 4 and 8) and that is fine: its can-fail partner proves the checks bite.

- [ ] **Step 3: Write `modules/gmcp/gmcp.ButtonPress.go`**

```go
package gmcp

import (
	"github.com/GoMudEngine/GoMud/internal/buttons"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// pressButton runs a touch button the player pressed (#781, the movement
// slice of the button protocol, owner rulings 7 and 8). The client sends only
// the id (Char.Button.Press {"id": ...}); buttons.Resolve turns it back into
// the line the player would have typed, and it is queued as typed input at
// the current turn: the same parser, the same one-command-per-turn queue, and
// a menu or a page stop treats it as typing. An id the server did not issue
// to this player for this room is refused as stale, and Room.Info is sent
// again so the next press is current. While a prompt is open a press does
// nothing: a prompt takes the next typed line as its answer, and a button is
// never an answer.
func pressButton(u *users.UserRecord, id string) {
	if u.GetPrompt() != nil {
		return
	}
	line, ok := buttons.Resolve(u.UserId, u.Character.RoomId, id)
	if !ok {
		refusal.StaleButton(u)
		events.AddToQueue(GMCPRoomUpdate{UserId: u.UserId, Identifier: `Room.Info`})
		return
	}
	u.CommandFlagged(line, events.CmdTyped)
}
```

- [ ] **Step 4: Wire the message**

`modules/gmcp/gmcp.go`, replace:

```go
		case `Char.Automation.Set`, `Char.Automation.Remove`,
			`Char.Action.Try`, `Char.Quests.Focus`, `Char.Quests.Abandon`:
```

with:

```go
		case `Char.Automation.Set`, `Char.Automation.Remove`,
			`Char.Action.Try`, `Char.Quests.Focus`, `Char.Quests.Abandon`,
			`Char.Button.Press`:
```

`modules/gmcp/gmcp.CharOp.go`, add a case to the `switch evt.Command` in `handleCharOp` (beside `Char.Action.Try`):

```go
	case `Char.Button.Press`:
		// A touch button (#781): the id only, never a command.
		var req struct {
			Id string `json:"id"`
		}
		if err := json.Unmarshal(evt.Payload, &req); err != nil {
			break
		}
		if uid := userIdForConnection(evt.ConnectionId); uid > 0 {
			if u := users.GetByUserId(uid); u != nil {
				pressButton(u, req.Id)
			}
		}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./modules/gmcp/ ./internal/usercommands/ -count=1 && go test . ./internal/guards/ -count=1`
Expected: `ok`.

- [ ] **Step 6: Prove the tests can fail**

In `pressButton` replace `u.CommandFlagged(line, events.CmdTyped)` with `u.Command(line)`: `TestPressButton_QueuesTheTypedLine` FAILS ("a press is typed input"). Restore. Delete the `if u.GetPrompt() != nil { return }` lines: the prompt test FAILS. Restore. In `TestButtons_EveryIssuedMovementIdRoundTrips`, temporarily make `buttons.Resolve` return `"gox " + name` for per-room ids: the round trip FAILS naming the exit. Restore; PASS.

- [ ] **Step 7: Update `modules/gmcp/context.md`**

Add to the inbound list: `Char.Button.Press {"id": "..."}` (#781) is deferred to MainWorker through `GMCPCharOp`; `pressButton` (`gmcp.ButtonPress.go`) resolves the id with `buttons.Resolve` and queues the line as typed input (`CmdTyped`, current turn); a stale id answers `refusal.StaleButton` and re-sends `Room.Info`; nothing happens while a prompt is open. Guard 1 (round trip) for movement ids is `internal/usercommands/button_roundtrip_test.go`.

- [ ] **Step 8: Commit**

```bash
git add modules/gmcp/gmcp.go modules/gmcp/gmcp.CharOp.go modules/gmcp/gmcp.ButtonPress.go modules/gmcp/gmcp.ButtonPress_test.go modules/gmcp/context.md internal/usercommands/button_roundtrip_test.go
git commit -F - <<'EOF'
feat(gmcp): Char.Button.Press runs a movement button as typed input (#781)

The client sends a button id; the server resolves it to the line the
player would type and queues it as typed input, one command per turn. An
id not issued to this player for this room is refused as stale. Guard 1
(round trip) covers every movement id.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 13: Client: the map does not re-centre when the room is unchanged

**Files:**
- Modify: `_datafiles/html/public/static/js/mapwhere.js`
- Modify: `_datafiles/html/public/webclient-pure.html` (`allRooms` at 820; the `Room` handler 881-976; `socket.onopen` about 3180)
- Modify: `tools/jstest/mapwhere.test.js`

- [ ] **Step 1: Write the failing tests**

In `tools/jstest/mapwhere.test.js`, directly before the final `console.log(failures ? ...)` line, add:

```js
// #781: Room.Info comes again in the same room when its exits change; only a
// new room moves the map.
check('recentre: the first Room.Info centres', MW.recentre(null, 5), true);
check('recentre: a new room centres', MW.recentre(5, 6), true);
check('recentre: the same room again does not', MW.recentre(5, 5), false);
check('the Room handler asks recentre before it moves the map',
    /MapWhere\.recentre\(allRooms\.lastRoomId, obj\.Info\.num\)/.test(page), true);
check('a zone or floor change forgets the last room', /gr\.reset\(\);\s*allRooms\.lastRoomId = null;/.test(page), true);
check('a new connection forgets the last room', /socket\.onopen = function\(\) \{\s*allRooms\.lastRoomId = null;/.test(page), true);
check('a temporary exit is not drawn as a map edge', /exitInfo\.kind !== "temporary"/.test(page), true);
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `node tools/jstest/mapwhere.test.js`
Expected: exits 1; `MW.recentre is not a function` (TypeError), or FAIL lines for the page checks.

- [ ] **Step 3: Implement**

`mapwhere.js`, replace `    return { text: text };` with:

```js
    // recentre: Room.Info comes again in the same room when its exits change
    // (#781: a door unlocks, a portal opens, a light goes out). The map moves
    // to the player only in a new room, so a map the player panned stays
    // where they put it. lastRoomId: the room last centred on, or null.
    function recentre(lastRoomId, roomId) {
        return lastRoomId === null || lastRoomId === undefined || lastRoomId !== roomId;
    }
    return { text: text, recentre: recentre };
```

`webclient-pure.html`:

- `let allRooms = {` block: add `lastRoomId: null,` after `roomZones: {},`.
- In the `Room` handler replace:

```js
                    if ( allRooms.currentZoneKey != zoneKey ) {
                        gr.reset();
                        allRooms.currentZoneKey = zoneKey;
```

with:

```js
                    if ( allRooms.currentZoneKey != zoneKey ) {
                        gr.reset();
                        allRooms.lastRoomId = null;
                        allRooms.currentZoneKey = zoneKey;
```

- Directly before `var r = {` add:

```js
                    // Room.Info comes again in the same room when its exits
                    // change (#781). Only a new room joins the map and moves
                    // it; the same room leaves a panned map where it is.
                    if ( !MapWhere.recentre(allRooms.lastRoomId, obj.Info.num) ) {
                        updateMapWhere();
                        return;
                    }
                    allRooms.lastRoomId = obj.Info.num;

```

- In the `exitsv2` loop replace `if ( exitInfo.dz == 0 ) {` with `if ( exitInfo.dz == 0 && exitInfo.kind !== "temporary" ) {` and add above it `// A temporary exit (a gangplank, a portal) is no map edge (#781).`
- Replace `socket.onopen = function() {` with:

```js
            socket.onopen = function() {
                allRooms.lastRoomId = null;
```

(the old first line `linesSent = 0;` follows unchanged).

- [ ] **Step 4: Run the JS gate**

Run (Git Bash):

```bash
failed=0
while IFS= read -r f; do case "$f" in *.min.js) continue ;; esac; if [ -f .jshintignore ] && grep -Fxq "$f" .jshintignore; then continue; fi; node --check "$f" || failed=1; done < <(git ls-files '*.js')
for t in tools/jstest/*.test.js; do node "$t" >/dev/null || { echo "FAIL $t"; failed=1; }; done
for t in tools/webclient-tests/*.js; do node "$t" >/dev/null || { echo "FAIL $t"; failed=1; }; done
echo "failed=$failed"
```

Expected: `failed=0`.

- [ ] **Step 5: Prove the test can fail**

Change `recentre` to `return true;`: `mapwhere.test.js` exits 1 at "the same room again does not". Restore; exits 0.

- [ ] **Step 6: Commit**

```bash
git add _datafiles/html/public/static/js/mapwhere.js _datafiles/html/public/webclient-pure.html tools/jstest/mapwhere.test.js
git commit -F - <<'EOF'
fix(webclient): the map stays put when Room.Info repeats in the same room (#781)

Room.Info now comes again in the same room when its exits change. Only a
new room moves the map; a temporary exit is not drawn as a map edge.

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 14: Docs: README, PATCH_NOTES, context.md sweep

**Files:**
- Modify: `docs/README.md`, `docs/PATCH_NOTES.md`
- Check: every `context.md` touched by tasks 1-12

- [ ] **Step 1: `docs/README.md`**

Add rows (in the nearest matching table; the plan beside the spec row at line 332):

```markdown
| [`superpowers/plans/2026-11-22-moving-by-touch-2a-server.md`](superpowers/plans/2026-11-22-moving-by-touch-2a-server.md) | Touch arc plan 2a (#781): `Character.KnowsSecretExit` and `rooms.KnownExits`, the one rule for the exits a player knows (secret exits once searched); every lister (exits line, Room.Info, Zone.Map, the ASCII map, quest guidance) and every player exit verb reads it, with an exit lookup guard; followers learn; NPC unscripted moves take only found secret exits; `internal/buttons` (movement ids, dark marks); Room.Info kinds, ids, dark, marks and its round watcher; `Char.Button.Press`; guard 1; the map not re-centring; measurement and an adversarial seat |
| [`../internal/buttons/context.md`](../internal/buttons/context.md) | The touch buttons' server side (#781): fixed T ids, per-room ids, `Resolve` and stale presses, dark marks; in memory, per session |
| [`../internal/guards/exit_lookup_guard_test.go`](../internal/guards/exit_lookup_guard_test.go) | Every unfiltered exit lookup (`FindExitByName`, `actions.FindExit`) in `internal/` and `modules/` is registered with a reason; a player path must use `FindKnownExitByName` (#781); a fixture proves it can fail |
| [`../tools/playtest/scenarios/781-secret-exits.yaml`](../tools/playtest/scenarios/781-secret-exits.yaml) | Plan 2a's adversarial seat: find, guess or walk through a secret exit, and learn anything from dark bumps (goals in `goals/scenarios/781-secret-exits/`) |
```

and extend the `tools/jstest/` row's sentence list with: `Added 2026-11-22 (touch arc plan 2a): mapwhere.test.js also covers MapWhere.recentre, the Room handler not moving the map when Room.Info repeats in the same room, and temporary exits not drawn as edges.`

- [ ] **Step 2: `docs/PATCH_NOTES.md`**

Add at the top, under `# DOGMud Patch Notes`:

```markdown
## 2026-11-22: Secret exits stay secret until you find them

- A secret exit now joins a room's exits, your map and quest directions once you find it with `search`, and stays there. Walking through it no longer reveals it for a few minutes; finding it does.
- Until you find it, a secret exit answers like a wall: going, looking, unlocking, picking or shooting that way tells you nothing.
- Following someone through a secret exit, in a party or with `follow`, teaches it to you. Companions learn it too.
- Creatures wander, chase and run errands only through secret exits they have found. Patrols and daily routes keep their usual ways.
- `exits` in the dark now says it is too dark to see, as `look` does.
- On the web client the map no longer jumps back to you while you stay in one room.
```

- [ ] **Step 3: context.md sweep**

Run: `python tools/context_md_audit.py`
Expected: no line naming `internal/rooms`, `internal/characters`, `internal/actions`, `internal/usercommands`, `internal/mobcommands`, `internal/behaviortree`, `internal/hooks`, `internal/mapper`, `internal/questengine`, `internal/events`, `internal/refusal`, `internal/mobs`, `internal/buttons`, `internal/guards`, `modules/gmcp` or `modules/follow` (other packages' known false positives may print; leave them). Grep each of those `context.md` files for `HasVisited` and `visited set` and correct any sentence still describing the old secret-exit rule.

- [ ] **Step 4: Commit**

```bash
git add docs/README.md docs/PATCH_NOTES.md
git commit -F - <<'EOF'
docs: plan 2a in the README, patch notes for secret exits (#781)

Part of #781

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

(Add any `context.md` corrected in step 3 to the same `git add`.)

---

### Task 15: Full gate and boot checks

- [ ] **Step 1: gofmt**

Run: `gofmt -l internal/ modules/ *.go`
Expected: no output. For any file listed, run `gofmt -d <file>` and apply the shown change with the Edit tool.

- [ ] **Step 2: Build and the whole suite**

Run: `go build ./... && go test ./... -count=1 2>&1 | tail -40`
Expected: every package `ok` (or `no test files`). Then `go test . ./internal/guards/ -count=1`: `ok`.

- [ ] **Step 3: Lint at CI's pin**

Run: `bash tools/lint.sh`
Expected: `0 issues.`

- [ ] **Step 4: JS gate**

Run the three loops from task 13 step 4. Expected: `failed=0`.

- [ ] **Step 5: Boot smoke**

Run: `DOGMUD_BOOT_SMOKE=1 go test . -run TestSmoke -count=1`
Expected: `ok`. (The known Windows failures `TestDeleteZone_RemovesEveryTree` and `TestRenameZone_MovesRewritesAndRekeys` live in `internal/rooms`, not here; if any `ziggurat_test_zone/` or `rename_probe_zone/` folder appears under `_datafiles/world/dogmud/rooms/`, remove it.)

- [ ] **Step 6: Isolated boot check (dogmud-shipping, pre-push step 6)**

```bash
S="<session scratchpad directory, full path>"
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
printf 'Network:\n  TelnetPort: [43333]\n  LocalPort: 19999\n  HttpPort: 18090\n  AIPort: 45555\n' > "$S/boot-override.yaml"
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
CONFIG_PATH="$S/boot-override.yaml" timeout 180 ./boot-check.exe > "$S/boot.log" 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$S/boot.log"
grep -c "Server Ready" "$S/boot.log"
```

Expected: exit 124 from `timeout`; `0` then `1`. Remove the worktree after (`git worktree remove --force C:/tmp/dogmud-boot-check`; PowerShell `Remove-Item -Recurse -Force` if Windows holds the exe).

---

### Task 16: MEASUREMENT: `Room.Info` lag (and the Conditions lag report)

**What:** on a local server, the time between a move's room text ("You head ...") and the new `Room.Info` reaching the web client; and the owner's report that the Conditions panel trails the game text by about half a second (Rally). Noticeable (decision D24): a median over 250 ms or any single move over 500 ms. Noticeable Room.Info lag is fixed or filed before 2b ships; the Conditions lag is filed as its own issue either way.

- [ ] **Step 1: Boot a measurement server**

Reuse task 15's worktree and override, adding a profiles manifest. Write `"$S/measure-manifest.yaml"` with the Write tool:

```yaml
entries:
  - profile: mid
    start_room: 5227
    actor_id: measurer
```

Append `Playtest:\n  ProfilesManifest: "<S>/measure-manifest.yaml"\n` to the override (Write tool), start `boot-check.exe` in the background with `CONFIG_PATH` as before, record its PID, and wait for `Server Ready` in the log. The server writes `creds.json` beside the manifest.

- [ ] **Step 2: Drive the real web client and timestamp frames**

Write `"$S/measure-shots.json"` (username and password from `creds.json`):

```json
{ "url": "http://127.0.0.1:18090/webclient-pure.html",
  "edge": "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe",
  "profileDir": "<S>/edge-profile", "width": 1600, "height": 1000,
  "sessions": [ { "username": "<user>", "password": "<password>",
    "steps": [
      {"wait": 3000},
      {"eval": "(() => { const s = socket, orig = s.onmessage; window.__lag = []; s.onmessage = function (ev) { const t = performance.now(); if (typeof ev.data === 'string') { if (ev.data.indexOf('!!GMCP(Room.Info ') === 0) window.__lag.push(['info', t]); else if (ev.data.indexOf('!!GMCP(Char.Conditions') === 0) window.__lag.push(['cond', t]); else if (ev.data.indexOf('You head') !== -1) window.__lag.push(['text', t]); else if (/rally/i.test(ev.data)) window.__lag.push(['rallytext', t]); } return orig.call(this, ev); }; return true; })()"},
      {"send": "west", "after": 2500}, {"send": "east", "after": 2500},
      {"send": "west", "after": 2500}, {"send": "east", "after": 2500},
      {"send": "west", "after": 2500}, {"send": "east", "after": 2500},
      {"send": "west", "after": 2500}, {"send": "east", "after": 2500},
      {"send": "west", "after": 2500}, {"send": "east", "after": 2500},
      {"send": "rally", "after": 3000},
      {"eval": "JSON.stringify(window.__lag)"}
    ] } ] }
```

Run: `node tools/webshot/live-shot.js "$S/measure-shots.json" > "$S/measure.out"`

- [ ] **Step 3: Compute**

For each `text` entry, the lag is the next `info` time minus its time; report n, median and max in ms. For `rallytext`, the next `cond` time minus its time. If `rally` was refused (the profile lacks it), record "Conditions lag not measured" and still file the issue with the owner's report.

- [ ] **Step 4: Act**

Room.Info median and max within D24: record the numbers in the PR body. Over: search open issues (`gh issue list --repo pruuk/DOGMud --search "Room.Info lag"`), file one if none (`gh issue create --repo pruuk/DOGMud --label bug,area:web-client`, body with the numbers and the method), and link it from #781 as a 2b blocker. Conditions lag: file `Conditions panel trails the game text (Rally)` with the measured number or "not measured", labels `bug`, `area:web-client`, `needs-design`.

- [ ] **Step 5: Clean up**

Stop the server by its recorded PID only (`Stop-Process -Id <pid>`); never by name or port. Leave the worktree for task 17 or remove it.

---

### Task 17: ADVERSARIAL PLAYTEST: one seat against secret exits and dark bumps

**Files:**
- Create: `tools/playtest/scenarios/781-secret-exits.yaml`
- Create: `tools/playtest/goals/scenarios/781-secret-exits/prober.yaml`

- [ ] **Step 1: Write the scenario**

`tools/playtest/scenarios/781-secret-exits.yaml`:

```yaml
# #781 plan 2a (owner rulings 1, 2, 4 and 13): a secret exit is known only
# once found with `search`; a guess at it, in any form, reveals nothing; a
# bump in the dark teaches nothing a real wall does not. One seat starts in
# the Old Watercourse (6418, a dark sewer with a secret north door) with a
# hooded lantern to compare dark and lit.
name: 781-secret-exits
mode: party
summary: >-
  An unfound secret exit answers exactly like a wall to every verb, in the
  dark and in the light; once searched out it lists, maps and walks; a dark
  bump says nothing about what is behind it.
on_actor_stop: continue
budgets:
  wall_clock: 40m
requires:
  max_connections: 5
roster:
  - id: prober
    personality: bug-finder
    goals: goals/scenarios/781-secret-exits/prober.yaml

group_goals:
  - id: secret-stays-secret
    do: Before finding it, the seat tries every way to learn of, name or walk through the north door.
    verify: Every answer for north matches the answer for south, a real wall, word for word, dark or lit; `exits` never lists north; no other verb confirms it.
  - id: found-exit-works
    do: The seat searches until it finds the door, then uses it.
    verify: After "You found a secret exit" the exits line shows (north), `north` walks to the next room and back, and it stays listed after leaving and returning.
  - id: dark-bumps-teach-nothing
    do: In the dark, the seat bumps every direction and compares.
    verify: The bump line is the same for the hidden door and the real walls; nothing in the text differs.
```

- [ ] **Step 2: Write the seat's goals**

`tools/playtest/goals/scenarios/781-secret-exits/prober.yaml`:

```yaml
# PROBER seat: you stand in the Old Watercourse, a dark sewer channel. There
# is a hidden door here that you have not found. Your job is to learn where
# it is, or get through it, WITHOUT finding it with `search` first, and then
# to check that finding it works. You carry a hooded lantern; do not equip
# it until a goal says so. Quote every game line VERBATIM, and for every
# comparison give both answers side by side.
ephemeral:
  profile: early
  start_room: 6418
  overlays:
    grant_items: [20097]
    grant_skills:
      search: 3
  budgets:
    wall_clock: 35m

goals:
  - >-
    In the dark, type each of these and quote every answer: `north`, `n`,
    `go north`, `go n`, `nort`, `south`, `s`, `go south`, `east`, `west`,
    `up`, `down`, `exits`, `look`, `look north`, `look south`. Say whether
    ANY answer for north differs from the same answer for south (south has
    no exit at all).
  - >-
    Still in the dark, try to get through or learn of the north wall with
    other verbs: `unlock north`, `lock north`, `pick north`, `picklock north`,
    `search north`, `look wall`, `defuse north`. Compare each with the same
    verb toward south. Quote both.
  - >-
    Type `equip lantern` so you can see. Repeat the first goal's list in the
    light and compare north with south again. Read the room description and
    the exits line closely and say what, if anything, hints at a door.
  - >-
    Type `search` and `search wall`, waiting for the cooldown between tries,
    until you get "You found a secret exit" (stop after 12 tries and say
    so). Quote the find line, then `exits` and `look`.
  - >-
    Walk `north`, quote the room you reach, then `south` back. Type `exits`
    again and quote it. Walk away east and come back west, and say whether
    the north exit is still listed.
  - >-
    Type `unequip lantern` (or remove it) to be in the dark again. Type
    `north` and quote it. Then try anything else you can think of that might
    reveal a hidden way you have not found, or tell a hidden door from a
    wall in the dark. Report exactly what you typed and what printed.
```

- [ ] **Step 3: Run it**

Check the harness first (`ls ../gomud-playtest-harness`; restore it per `dogmud-playtesting` if missing). Then:

```text
/playtest-scenario --checkout <absolute path of the plan 2a worktree> tools/playtest/scenarios/781-secret-exits.yaml
```

Kill only processes this run started, by PID; after the run, stop this run's leftover `tail.exe` processes by PID (`Get-CimInstance Win32_Process -Filter "Name='tail.exe'"`, matching this run's folder).

- [ ] **Step 4: Check the seat against the raw stream**

Read `tools/playtest/.run/<run_id>/actors/prober/bridge/events.jsonl` (Python with `-I` and `sys.stdout.reconfigure(encoding='utf-8')`). For each seat claim, find the verbatim line. Also check the `gmcp` events: every `Room.Info` before the find has no `north` key in `exitsv2` and `exits`; in the dark `dark` is true and `exitsv2` is `{}`; after the dark bumps `darkmarks` holds `north` and `south` as `wall`; after `equip lantern` `darkmarks` is `{}`; after the find `exitsv2.north` carries `secret` and id `m1`.

- [ ] **Step 5: File findings**

Each defect: check on the branch, search open issues, then `gh issue create --repo pruuk/DOGMud` with labels `bug`, `from-playtest` and an `area:*` label. Anything that lets a player pass or confirm an unfound secret exit goes to the owner directly, not a public issue (`dogmud-playtesting`, exploit rule). A finding inside plan 2a's scope is fixed on this branch before the PR (a flaw found mid-task widens scope).

---

### Task 18: PR

- [ ] **Step 1: Re-run the gate if anything changed since task 15** (steps 1 to 5 of task 15).

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feature/781-moving-by-touch-2a
gh pr create --repo pruuk/DOGMud --base master --head feature/781-moving-by-touch-2a --title "Touch arc 2a: known exits, movement button protocol, dark marks (#781)" --body-file "$S/pr-body.md"
```

`$S/pr-body.md` (written with the Write tool): a summary of tasks 1-13, the measurement numbers (task 16), the playtest result and issues filed (task 17), the line `Part of #781`, never a closing keyword, and the attribution line `🤖 Generated with [Claude Code](https://claude.com/claude-code)`. Confirm the URL `gh` prints says `pruuk/DOGMud`.

- [ ] **Step 3: Watch the checks**

Run: `gh pr checks <n> --repo pruuk/DOGMud --watch`, then `gh run list --repo pruuk/DOGMud --branch feature/781-moving-by-touch-2a` to confirm the JS job ran. On a red run, `gh run view <id> --repo pruuk/DOGMud --log-failed`.

- [ ] **Step 4: After the owner merges**

Comment on #781 with what landed and what is left (2b, 2c); do not close it. The owner deploys.

---

## Self-review

**Spec coverage.**

| Spec item | Task |
|---|---|
| `KnownExits`: name, kind, locked now, destination, dark; `Exits`, `ExitsTemp`, mutator exits | 2 |
| Every lister reads it: exits line, `Room.Info`, `Zone.Map` (plus the ASCII map and guidance, D4) | 3, 6, 10 |
| Secret exits known only once searched (ruling 1); the 180-second trail and the visited set replaced | 1, 2, 3, 6, 10 |
| Followers learn it (ruling 2) | 5 |
| Player movement path refuses an unfound secret exit as no exit ("bumping into walls" / "not recognized") | 4 |
| NPCs: wandering, chasing, errands, following use only found secret exits; scripted routes keep them; companions learn; flee unchanged (ruling 13) | 5, 7 |
| `Room.Info`: known exits only, locked now, `kind`, `dark`, empty maps at no sight, ids | 10 |
| Kept current: per-round watcher with a signature (exits, locks, dark, marks) | 11 |
| Button ids: fixed T ids, per-room ids, nothing encoded (rulings 7, 8) | 8, 10 |
| Press: GMCP message, issued-only, stale refusal with a plain line, normal input queue | 12 |
| Dark: only the fixed ids offered | 8, 10 |
| Dark marks: typed and tapped, cleared on leaving or light, ride in `Room.Info` (ruling 4) | 8, 9, 10, 11 |
| Tests: `KnownExits` cases; `go` and bare names; NPC cases; `Room.Info` cases; one shared fixture where the exits line and `Room.Info` agree | 2, 3, 4, 5, 7, 10 (the gmcp `exitsFixture` and the rooms `knownExitsRoom` feed both listers; `Zone.Map` is checked on the mapper's ring fixture) |
| Guard 1 (round trip), guard 5 (sight), abuse rules with can-fail cases | 12, 10, 8, 12 |
| The client `Room.Info` handler not re-centring | 13 |
| Measurement | 16 |
| Adversarial playtest seat | 17 |

**Gap noted, not closed:** the spec asks for "one shared fixture" checking that the exits line, `Zone.Map` and `Room.Info` agree. `Zone.Map` is built from the mapper's crawl, which needs rooms on disk (`writeFixtureWorld`), while the other two run on seeded rooms; this plan proves each against the same rule (`KnowsSecretExit`) instead of one fixture. Reported to the owner.

**Placeholder scan:** none. Every code step shows the code; every command shows its expected result.

**Type consistency:** `KnownExits(*Room, *characters.Character) KnownExitList`; `KnownExit{Name, Kind, LockedNow, RoomId, Exit, Temp}`; `ExitKind` constants; `Room.ExitKnownTo`, `FindKnownExitByName(char, search)`, `FindKnownExitTo(char, roomId)`, `GetRandomKnownExit(char)`; `actions.FindKnownExit(room, char, name)`, `actions.LearnSecretExitFollowing(c, room, exitName)`; `mapper.SecretExitsKnownTo(c)`, `mapper.AnySecretExit`, `mapper.GetPathKnown(start, known, end...)`, `Snapshot(visited, known)`; `mobs.PathtoFoundOnly`; `buttons.FixedIds`, `MovementIds(userId, room, known)`, `Resolve(userId, roomId, id)`, `Forget`, `RecordDarkMark`, `DarkMarks(userId, roomId, dark)`, `ClearDarkMarks`, `MarkWall`, `MarkLocked`; `events.RoomExitsChanged`; `refusal.StaleButton`, `StaleButtonLine`; gmcp `buildRoomExitsInfo`, `roomExitsSignature`, `recordSentRoomExits`, `forgetSentRoomExits`, `roomExitsDue`, `roomExitsWatchHandler`, `roomExitsChangedHandler`, `pressButton`. Used with these signatures in every later task.



