# Shadow follow parity: every shadower follows by one rule, and the checks run on arrival

Date: 2026-09-30. Player/mob parity slice 6, the last row of the
owner-ordered parity audit of 2026-09-28 (row 11, "mob shadower never
follows", filed as "not re-read"; the audit lives in auto-memory as
`project-player-mob-parity-audit-2026-09-28`, not in `docs/`). Design
approved by the owner on 2026-09-30; this spec verifies it against source and
records where the source disagrees with it.

## Facts verified against source (2026-09-30, master `22e0c4178`)

Every row was read at `22e0c4178` in a fresh worktree. Negative rows name the
search and a positive hit proving the same search could match.

### The two follow sites

| # | Fact | Where |
|---|---|---|
| F1 | Player-mover follow: inside `usercommands.Go`, after the charmed-mob loop, a loop over `room.GetPlayers(rooms.FindAll)` of the OLD room skips the mover, skips a shadower that is not `IsHidden()`, skips one where `shadowIsTargetingUser(shadowP, user.UserId)` is false, then, if `!HasCondition(87)`, clears both misc keys and skips (the stale-state guard). Otherwise it calls `shadowP.Command(rest)` | `internal/usercommands/go.go:394-420` |
| F2 | The same loop then checks `!shadowP.Character.IsHidden()` and calls `endShadow(shadowP, "You've been spotted -- your shadow ends.")`, else calls `shadowDetectionRoll(shadowP, user, destRoom)` and on true sends the mover "You sense someone following close behind you." | `go.go:425-434` |
| F3 | Only player shadowers are considered by F1: it iterates `GetPlayers`, never mobs | `go.go:396` |
| F4 | Mob-mover follow: `MobRoomChangeShadowFollow` returns early when `evt.MobInstanceId == 0`, resolves the exit with `fromRoom.FindExitTo(evt.ToRoomId)` and returns on `""` (teleport), then loops `users.GetAllActiveUsers()` requiring condition 87, `IsHidden()`, `shadow-target-mob == evt.MobInstanceId` and `RoomId == evt.FromRoomId`, and calls `u.Command(exitName)` | `internal/hooks/MobRoomChange_ShadowFollow.go:27-75` |
| F5 | F4 then checks `!u.Character.IsHidden()` and calls `inlineShadowEnd`. It has no stale-state guard (a shadower missing condition 87 is skipped, its keys kept) and no sense roll | `MobRoomChange_ShadowFollow.go:52,77-81` |
| F6 | F4 considers only player shadowers (`GetAllActiveUsers`); a mob shadower is never moved by either site | `MobRoomChange_ShadowFollow.go:47` |
| F7 | F4 is registered as a `RoomChange` listener | `internal/hooks/hooks.go:39` |

### Queued commands and event ordering

| # | Fact | Where |
|---|---|---|
| Q1 | `UserRecord.Command` only calls `events.AddToQueue(events.Input{...})` with `ReadyTurn` = current turn (plus any wait) | `internal/users/userrecord.go:375-388` |
| Q2 | `Mob.Command` only queues `events.Input` per `;`-split command, scheduled from `m.lastCommandTurn` so it lands after any command the mob already has queued | `internal/mobs/mobs.go:942-972` |
| Q3 | `events.ProcessEvents` pops one event at a time and runs its listeners; an event queued by a listener is processed after that listener returns. `Input` is handled by `World.HandleInputEvents` | `internal/events/events.go:90-121,176-235`; `world.go:77,82` |
| Q4 | A user command runs inside the `Input` listener; a user Input is processed at most once per turn per user (`userInputEventTracker`, else `CancelAndRequeue`), and a mob's pending Input blocks its later ones (`mobInputEventTracker`) | `world.go:100-124,145-186` |
| Q5 | Consequence of Q1 to Q4: in F2 and F5 the shadower has not moved when `!IsHidden()` is checked, so the spotted branch can only fire if the shadower was already visible, which F1 and F4 filter out. Both spotted branches are dead. F2's sense roll runs with the shadower still in the old room, before its move | derived from Q1-Q4, F1-F5 |
| Q6 | The player move emits `RoomChange{UserId, FromRoomId, ToRoomId, Unseen: IsHidden()}` via `events.AddToQueue` inside `rooms.MoveToRoom` | `internal/rooms/roommanager.go:361,476-481` |
| Q7 | The mob move emits `RoomChange{MobInstanceId, FromRoomId: mob.Character.RoomId, ToRoomId, Unseen}` via `events.AddToQueue` at the top of `Room.AddMob`, before the mob's `RoomId` is updated | `internal/rooms/rooms.go:1223-1240` |
| Q8 | Player walking: `MoveToRoom` at `go.go:264`, then `actions.EntryDetection(...)` at `go.go:440`, both synchronous inside the same `Input` listener | `go.go:264,440` |
| Q9 | Mob walking: `actions.RelocateMob` (which calls `Room.AddMob`) at `mobcommands/go.go:140`, then `actions.EntryDetection(arrived, destRoom, sneaking)` at `:146`, both synchronous inside the mob's `Input` | `internal/mobcommands/go.go:136-146`; `internal/actions/relocate_mob.go:84-92` |
| Q10 | **The ordering the design relies on holds.** From Q3, Q6 to Q9: a `RoomChange` is only queued during the move, and its listeners run after the moving command returns, so by the time a listener sees the shadower's own `RoomChange`, `EntryDetection` for that move has already run and `IsHidden()` reflects any reveal. `RoomChange.Unseen` does NOT: it is captured before detection (Q6, Q7), so the listener must read `IsHidden()` live | derived |
| Q11 | `IsHidden()` is `Awareness.IsHidden()`, true only in the awareness machine's `Hidden` state; a reveal is `TransitionToRevealing`, which leaves `Hidden` synchronously | `internal/characters/character.go:874-879`; `internal/state/awareness/awareness.go:85` |

### Entry detection

| # | Fact | Where |
|---|---|---|
| E1 | `EntryDetection(mover Actor, dest *rooms.Room, sneaking bool) EntryDetectionResult`: a sneaking mover is rolled against every non-allied player then mob in `dest` (`sneakerSpotted`); spotted drives `TransitionToRevealing` and clears `sneaking`. If the mover is not (or no longer) sneaking, `newcomerSpots` rolls it against every hidden player and mob in `dest` and awards Search on both outcomes | `internal/actions/move.go:200-223,293,337` |
| E2 | Player walkers pass `isSneaking := IsHidden()` (or the `sneaking` misc flag); mob walkers pass `IsHidden()` (or the flag) | `go.go:120-123`; `mobcommands/go.go:136-139` |
| E3 | So an arriving shadower (hidden) is rolled against the target and every other observer in the target's room. That is the reveal the design leaves to entry detection | E1, E2 |

### Movement narration of a hidden mover

| # | Fact | Where |
|---|---|---|
| N1 | A sneaking PLAYER sends only "You sneak towards the X exit." to itself; the room exit line, the destination entry line and `SendTextToExits` are all in the `else` branch | `go.go:296-356` |
| N2 | `RelocateMob` sends the exit line (`from.SendTextVisualWithAudio`, "<name> leaves towards the X exit."), the entry line (`dest.SendTextVisualWithAudio`, "<name> enters from X."), `SendTextToExits` and both sounds for EVERY mob, with no hidden or sneaking check | `internal/actions/relocate_mob.go:96-111` |
| N3 | `Room.SendTextVisualWithAudio` gates only on each listener's sight; it takes no subject and knows nothing of the mover being hidden | `internal/rooms/rooms.go:451-476` |

### Exit resolution

| # | Fact | Where |
|---|---|---|
| X1 | `modules/follow` `roomChangeHandler` handles both mover kinds and both follower kinds, resolves the exit as the map KEY from `fromRoom.Exits`, then `fromRoom.ExitsTemp`, and queues `mob.Command(exit, .25)` / `user.Command(exit, .25)`. It does not look at mutator exits | `modules/follow/follow.go:253-325` |
| X2 | `Room.FindExitTo(roomId)` checks `Exits` (returns the key), `ExitsTemp` (returns `exit.Title`, not the key) and active mutator exits (returns the key) | `internal/rooms/rooms.go:2228-2251` |
| X3 | `AddTemporaryExit` sets `Title` to the key only when `Title` is empty, so a titled temp exit's `Title` can differ from the key that `FindExitByName` matches | `rooms.go:709-728,2444` |

### Starting and ending a shadow

| # | Fact | Where |
|---|---|---|
| S1 | `actions.Shadow(actor Actor, opts ShadowOptions) ShadowResult` requires `IsHidden()`, refuses in combat, then `TryCooldown(skills.Skullduggery.String("shadow"), "<ShadowCooldown> rounds")` before resolving the target | `internal/actions/shadow.go:38-79` |
| S2 | `shadowMob` sets `shadow-target-mob`, clears `shadow-target-user`, `actor.AddCondition(87, "skill")`, and calls `AwardResolved(true, ...)` with no contest | `shadow.go:82-103` |
| S3 | `shadowPlayer` sets `shadow-target-user`, clears `shadow-target-mob`, adds 87, rolls `CalcSneakScoreVsObserver(char, target, sight) * messaging.SightMult(char, sight)` against `CalcDetectionScore(target, sight)` via `combat.RunContest` (the target is the attacker), `AwardResolved(!detected, ...)`, and on detected sends "You sense someone following close behind you." | `shadow.go:129-189` |
| S4 | F2's `shadowDetectionRoll(shadower, target *users.UserRecord, room)` is the same formula as S3 but awards nothing | `internal/usercommands/skill.skullduggery.shadow.go:135-146` |
| S5 | `ShadowOptions{TargetMobInstanceId int; TargetUserId int}`; `ShadowResult{Succeeded, Detected, TargetName, OnCooldown, Reason}` | `shadow.go:18-30` |
| S6 | Condition 87 "Shadowing": `triggerrate: 1 round`, `triggercount: 25`, no flags, `end_actee: Your focus on the quarry breaks.` | `_datafiles/world/dogmud/conditions/87-shadowing.yaml` |
| S7 | `ShadowCooldown` ships at `5` in `_datafiles/config.yaml:924` (same in the `git show HEAD:` blob); the Go default is also 5 | `config.yaml:924`; `internal/configs/config.balance.combat.go:271-272` |
| S8 | `endShadow(user *users.UserRecord, reason string)`: clears both keys, `RemoveCondition(87)`, starts the cooldown, sends `reason` if non-empty. `inlineShadowEnd` is a line-for-line copy | `skill.skullduggery.shadow.go:92-105`; `MobRoomChange_ShadowFollow.go:90-103` |
| S9 | `shadow stop` reads both keys directly and calls `endShadow(user, "You stop shadowing your target.")` | `skill.skullduggery.shadow.go:41-49` |
| S10 | `getShadowTargetUserId` has no production caller (grep of `getShadowTargetUserId(` in non-test Go finds only its definition; the same grep finds `shadowIsTargetingUser(` at its call site `go.go:407`) | `skill.skullduggery.shadow.go:108-117` |
| S11 | `MobDeathTrackingCleanup` and `PlayerDespawnTrackingCleanup` clear the matching key and `RemoveCondition(shadowingCondition)` on every user and mob pointing at the departed target, with NO cooldown and NO message | `internal/hooks/MobDeath_TrackingCleanup.go:9-12,50-56`; `internal/hooks/PlayerDespawn_TrackingCleanup.go:44-50`; registered `hooks.go:90,139` |
| S12 | `internal/characters/die.go` has no shadow handling (grep `hadow` hits only the "Shadow Realm" comment at `die.go:26`) | `die.go:26` |
| S13 | Every production reader or writer of the two misc keys: `actions/shadow.go:90,91,137,138`; `hooks/MobDeath_TrackingCleanup.go:51,53`; `hooks/MobRoomChange_ShadowFollow.go:60,91,92`; `hooks/PlayerDespawn_TrackingCleanup.go:45,47`; `usercommands/go.go:415,416`; `usercommands/skill.skullduggery.shadow.go:42,43,93,94,109,122` | grep `shadow-target-user\|shadow-target-mob`, non-test `.go` |
| S14 | Every production use of condition 87: `actions/shadow.go:92,139` (add); `hooks/MobDeath_TrackingCleanup.go:11` (const `shadowingCondition = 87`), `:54`; `hooks/MobRoomChange_ShadowFollow.go:52,93`; `hooks/PlayerDespawn_TrackingCleanup.go:48`; `usercommands/go.go:414`; `usercommands/skill.skullduggery.shadow.go:95` | grep `Condition\((87\|shadowingCondition)\b\|= 87\b\|\(87,`, non-test `.go` |

### Mob entry points and tooling

| # | Fact | Where |
|---|---|---|
| T1 | Mob `shadow <name>` resolves the target with no viewer and calls `actions.Shadow`; registered as mob command `shadow`; mob `sneak` is registered too | `internal/mobcommands/shadow.go:15-37`; `internal/mobcommands/mobcommands.go:81,91` |
| T2 | Btree `try_shadow` is `actTryShadow`, which calls `actions.Shadow` | `internal/behaviortree/actions.go:99`; `internal/behaviortree/actions_skullduggery.go:92-110` |
| T3 | No authored data uses `try_shadow`: grep over `*.yaml`, `*.yml`, `*.json`, `*.js` finds nothing, while the same grep for `try_sneak\|try_steal` finds `_datafiles/world/dogmud/behaviors/archetypes/thief.yaml` | grep |
| T4 | Admin `command <name> <cmd>` resolves the target in the operator's room with `ResolveTargetActor(room, searchName)` and no `Viewer`, so a hidden mob can be named (`findMobByName` skips the perceive check when `viewer == nil`) | `internal/usercommands/admin.command.go:43-66`; `internal/actions/target_resolution.go:23-27`; `internal/rooms/rooms.go:2106-2110` |
| T5 | Admin `locate <name>` finds a player by character name, else lists matching mobs with their room (`locate *` lists all) | `internal/usercommands/admin.locate.go:20-80,123` |
| T6 | `setcondition <target> <id>` can target a mob. It CANNOT make a mob hidden: condition 9 is a side-effect carrier mirrored one way from the awareness machine (`Hidden` state adds 9, leaving `Hidden` cancels it); adding 9 does not move the machine, so `IsHidden()` stays false | `internal/usercommands/admin.setcondition.go:168-186`; `internal/hooks/Awareness_Cascades.go:43-60`; `internal/state/awareness/awareness.go:1-8,85` |
| T7 | `actions.Sneak` rolls a mob actor against every player in the room with no staff exemption (allies are computed only for player actors), then against mobs | `internal/actions/sneak.go:104-137` |

### Other movers that reach RoomChange

| # | Fact | Where |
|---|---|---|
| M1 | A fleeing player moves through `rooms.MoveToRoom(user.UserId, out.ExitRoomId)` with a named exit, not through `Go`, so F1 never runs and a player target's shadower does not follow a flee today | `internal/hooks/NewRound_DoCombat_helpers.go:879` |
| M2 | A fleeing mob moves through `actions.RelocateMob`, so F4 does follow it today | `NewRound_DoCombat_helpers.go:947` |
| M3 | `rooms.MoveToRoom` has 18 production call sites in 14 files (quest, recall, respawn, ferry, teleport and others); any of them into an adjacent room resolves to a named exit | grep `rooms.MoveToRoom(` |

### Guards and tests that key on this code

| # | Fact | Where |
|---|---|---|
| G1 | `contestSiteOwners` lists `internal/actions/shadow.go:shadowPlayer` and `internal/usercommands/skill.skullduggery.shadow.go:shadowDetectionRoll`; `TestEveryContestSiteIsOwned` fails on an unowned site AND on a stale entry | `internal/combat/contest_site_guard_test.go:72-73,232-265` |
| G2 | `legacyLiteralFiles` names `internal/actions/shadow.go`, `internal/usercommands/go.go` and `internal/usercommands/skill.skullduggery.shadow.go` | `contest_site_guard_test.go:360-374` |
| G3 | `internal/actions/shadow_test.go` holds `TestShadow_RequiresHidden`, `_NoTarget`, `_Success`, `_Cooldown`, `_SkillProgressionFires`, `_DetectionWin`; `actions_skullduggery_test.go` holds `TestActTryShadow_*` | `shadow_test.go:117-262`; `internal/behaviortree/actions_skullduggery_test.go:288-320` |
| G4 | The model guard: `speech_wrapper_guard_test.go` (package `main`, repo root) strips comments by parse-and-print (`speechGuardCode`), counts required calls per file, and walks the tree (`speechGuardWalk`) | `speech_wrapper_guard_test.go:59,72,92,123` |
| G5 | Stale docs: `internal/actions/context.md:1581-1585` documents `ShadowOptions` as `TargetUserId string; TargetMobId int` (the struct is S5); `internal/hooks/context.md:1211` names `Death_MobTracking_Cleanup.go`, which does not exist (the file is `MobDeath_TrackingCleanup.go`) | as cited |

## Owner decisions (binding, 2026-09-30)

1. **One listener moves every shadower.** `internal/hooks/RoomChange_ShadowFollow.go`
   (NEW) replaces `MobRoomChange_ShadowFollow.go`; the loop at `go.go:394-435`
   is deleted. On any `RoomChange` through a named exit, every character in the
   old room, player or mob, still hidden, carrying condition 87 and targeting
   the mover, queues the same exit. Stale state (target set, condition 87 gone)
   is cleared for both kinds. A teleport (no named exit) moves no one.
2. **Checks run on arrival.** On the shadower's own `RoomChange` into its
   target's current room: not hidden means the shadow ends properly
   (condition removed, target cleared, cooldown, "You've been spotted -- your
   shadow ends."); still hidden means the target makes the sense roll with the
   real room's light.
3. **Sensing is awareness only on both sides.** A player target reads "You
   sense someone following close behind you."; a mob target gets nothing
   visible. The roll awards the shadower's Skullduggery on both outcomes
   (resolved-contest convention, U10b-2). Reveal stays entry detection's job.
4. **Shared bodies in `internal/actions`:** `ShadowSenseRoll`, `EndShadow`,
   `ShadowTargetOf`; `shadow stop` and the death and despawn cleanup hooks use
   them.
5. **Starting a shadow follows the same rule:** `shadowMob` runs the same
   initial sense roll as `shadowPlayer`.
6. **Out of scope:** authoring mob behaviour that uses `try_shadow`.
7. **Guard and tests** as in "Testing and gates".
8. **Playtest procedure** as in "Testing and gates".

## Owner rulings on the source findings (binding, 2026-09-30)

The ordering the design depends on holds (Q10). Four facts bore on the design;
the owner ruled on each on 2026-09-30 and took every recommendation: D1 silence
a sneaking mob's movement lines in this slice, D2 `ClearShadow` under
`EndShadow`, D3 one exit helper for the shadow listener only (`modules/follow`
unchanged), D4 accept the three edges. The text below keeps each finding and
its recommendation, now ruled.

**D1. A hidden mob's steps are announced by name (N1 to N3). Blocking.** A mob
shadower that follows a player will be seen doing it: `RelocateMob` sends
"<name> leaves towards the north exit." to the old room and "<name> enters
from the south." to the destination, which is the target's room, for a hidden
mob as for any other. A sneaking player sends neither. Without a change, every
mob-shadows-player step tells the target who is following, the playtest step
"target does not see the mob" fails, and the awareness-only sense roll
(ruling 3) is moot. Slice 4b made hidden detection symmetric but left this
narration asymmetric. **Ruled (owner, 2026-09-30):** in this slice, `RelocateMob` takes
the mover's sneaking state and, when sneaking, skips the exit line, the entry
line and `SendTextToExits`, as `go.go:296-356` does for a player. This changes
every hidden mob's movement, not only shadowing (a thief archetype mob that
sneaks and walks goes quiet too), which is the parity the audit asked for.
Sounds (`relocate_mob.go:110-111`) follow whatever the player path does at
`go.go:620-621`, verified during planning.

**D2. `EndShadow` has three callers with different needs (S8, S11).** The
spotted end and `shadow stop` start the cooldown and send a line. The death and
despawn cleanup hooks today clear state with no cooldown and no line, and so
does the stale-state guard (F1). Routing the hooks through `EndShadow` as
ruling 4 reads would start a 5-round cooldown and send text when the target
dies or logs off, a behaviour change nobody asked for. **Ruled (owner, 2026-09-30):** one
NEW `ClearShadow(char *characters.Character)` in `internal/actions` (keys and
condition 87 only) that `EndShadow` calls and then adds the cooldown and line;
the stale guard and both cleanup hooks call `ClearShadow`. Still one body, no
copies.

**D3. Exit resolution (X1 to X3).** `modules/follow` returns the command word
(map key) but ignores mutator exits; `FindExitTo`, which the mob hook uses
today, covers mutator exits but returns a temp exit's `Title`, which is not
guaranteed to be the key `go` matches. **Ruled (owner, 2026-09-30):** the listener
resolves the key from `Exits`, then `ExitsTemp`, then active mutator exits
(the follow order plus the mutator fallback), in one unexported helper next to
the listener. `FindExitTo` is left alone because `RelocateMob` uses it for
display text.

**D4. What "arrival" and "named exit" now cover (M1 to M3).** Three
consequences of ruling 1 and 2, all accepted by the owner, 2026-09-30:
- The arrival check fires on any `RoomChange` of a shadower into its target's
  room, including one it walked by itself; the listener cannot tell a
  shadow-follow move from any other. A shadower that walks in on its quarry
  gets the spotted check and gives the target a sense roll. Accept.
- A shadower's queued move can land a turn late (Q2, Q4). If the target has
  moved on, the shadower arrives in a room its target has left: no arrival
  check, and the next target move is not followed because the shadower is not
  in its old room. The shadow then lingers until condition 87 lapses (25
  rounds), as it does today. Accept; ending it here would need a "target lost"
  rule the owner has not asked for.
- A player target is now followed on every named-exit move, including a flee
  (M1) and a scripted move into an adjacent room (M3), as a mob target already
  is (M2). Accept.

## The rules

### Rule 1: one listener

`internal/hooks/RoomChange_ShadowFollow.go` (NEW) registers
`RoomChangeShadowFollow` (NEW) in `hooks.go` in place of
`MobRoomChangeShadowFollow` (`hooks.go:39`). For each `RoomChange`:

1. **Follow pass.** Resolve the exit (D3). If there is none, skip the pass. For
   every player (`fromRoom.GetPlayers(rooms.FindAll)`) and mob
   (`fromRoom.GetMobs(rooms.FindAll)`) in the old room other than the mover:
   look up `actions.ShadowTargetOf`; skip unless it names the mover. If
   condition 87 is gone, `ClearShadow` (D2) and skip. Skip if not
   `IsHidden()`. Otherwise `Command(exit)` on the user or mob. No wait
   argument: the follow module's `.25` delay exists to let a visible follower
   trail behind; a shadower matching its quarry's steps should not lag more
   than the queue already makes it (Q4).
2. **Arrival pass.** If the mover itself has a live shadow target
   (`ShadowTargetOf`) and condition 87, and that target's current room is
   `evt.ToRoomId`: if the mover is not `IsHidden()` (read live, never
   `evt.Unseen`, Q10), `EndShadow(mover, "You've been spotted -- your shadow
   ends.")`; otherwise `ShadowSenseRoll(mover, target, room)`.

The go.go loop (`go.go:394-435`), `endShadow`, `shadowIsTargetingUser`,
`shadowDetectionRoll` and the dead `getShadowTargetUserId` (S10) are deleted,
as is `MobRoomChange_ShadowFollow.go` with `inlineShadowEnd`. The
`shadowingCondition` const in `MobDeath_TrackingCleanup.go:11` goes with its
last users; condition 87 is named once, as `actions.ShadowingConditionId`
(NEW).

### Rule 2 and 3: sensing

`ShadowSenseRoll(shadower, target Actor, room *rooms.Room) bool` (NEW) is S3's
contest lifted out of `shadowPlayer`: `sight := combat.SightRoom(room)`, the
shadower's `CalcSneakScoreVsObserver(...) * messaging.SightMult(...)` against
the target's `CalcDetectionScore(target, sight)` through `combat.RunContest`,
target as attacker. It calls `shadower.AwardResolved(!detected,
CandidateFor(Skullduggery))` on both outcomes and, when detected, sends
"You sense someone following close behind you." through `target.SendText`,
which is already a no-op for a mob actor (`internal/actions/actor.go:23`,
the `SendText` doc). It reveals nothing and returns `detected`.

### Rule 4: shared bodies

All NEW, in `internal/actions/shadow.go`:

- `ShadowingConditionId = 87`.
- `ShadowTargetOf(c *characters.Character) (userId, mobInstanceId int)`: the
  only reader of the two misc keys outside `Shadow` itself.
- `ClearShadow(c *characters.Character)` (D2).
- `EndShadow(actor Actor, reason string)`: `ClearShadow`, cooldown from
  `ShadowCooldown`, `reason` to the actor if non-empty.
- `ShadowSenseRoll` (above).

`shadow stop` uses `ShadowTargetOf` and `EndShadow`. `MobDeathTrackingCleanup`
and `PlayerDespawnTrackingCleanup` use `ShadowTargetOf` and `ClearShadow`
(D2), keeping their tracking-key logic as is.

### Rule 5: starting a shadow

`shadowMob` calls `ShadowSenseRoll(actor, NewMobActorInRoom(m, room), room)`
in place of `AwardResolved(true, ...)` and returns `Detected` like
`shadowPlayer`. `shadowPlayer` calls the same function in place of its inline
contest (`shadow.go:164-182`). Both keep their start text and quest notify.

## Parity table

| Rule | Player today | Mob today | Both after |
|---|---|---|---|
| Shadower follows a player mover | yes (walking only, F1) | no | yes, any named exit |
| Shadower follows a mob mover | yes (F4) | no | yes |
| Stale state cleared on a mover's step | player-mover only | no | yes |
| Spotted on arrival ends the shadow with cooldown | never fires (Q5) | never fires | yes |
| Sense roll after each step | before the move, player targets only, no award | none | on arrival, both targets, award both outcomes |
| Sense roll when starting | player target, award both outcomes | mob target: no roll, award as a win | both targets, award both outcomes |
| Sense result shown | player target: line | n/a | player target: line; mob target: nothing |
| Hidden mover's steps narrated to the room | no (N1) | yes, by name (N2) | no (D1) |
| Target death or logoff clears the shadow | yes, no cooldown | yes, no cooldown | unchanged (D2) |

## What changes in play

- **Mob shadowers follow.** A hidden mob told to `shadow` a player or mob now
  walks after it, and does so without its steps being announced (D1).
- **A spotted shadower's shadow ends.** Today a shadower revealed on arrival
  keeps condition 87 and its target, and follows again as soon as it re-hides.
  Now the shadow ends with "You've been spotted -- your shadow ends." and the
  5-round cooldown (`ShadowCooldown`, S7).
- **The sense line comes after the step.** A player target reads "You sense
  someone following close behind you." once the shadower has arrived, not
  while it is still behind them, and the room's light is the room they share.
- **Shadowing trains on every step.** Each arrival is a resolved contest and
  trains Skullduggery on both outcomes; the per-step roll trained nothing
  before (S4).
- **Shadowing a mob is a real contest.** The start roll now runs against the
  mob's detection score, so a sharp-eyed mob trains the shadower less than an
  oblivious one.
- **Fleeing players are followed** (D4), as fleeing mobs already were.
- **Every sneaking creature moves silently** (D1), not only a shadower: a
  hidden mob no longer announces "<name> leaves towards ..." or "<name> enters
  from ...", as a sneaking player never has. A thief that sneaks and walks is
  no longer given away by its own movement lines.

## Testing and gates

**Table test (NEW, `internal/hooks/RoomChange_ShadowFollow_test.go`).** Drives
`RoomChangeShadowFollow` with seeded users and mobs and inspects the queue
with `events.InspectQueuedInputForTest` (mobs) and the user-side equivalent the
plan finds or adds. Cases, each for mover {player, mob} x shadower
{player, mob}:

- hidden, condition 87, targeting the mover, in the old room: the exit is
  queued;
- not hidden / in another room / targeting someone else: nothing queued;
- stale (target set, no condition 87): nothing queued, keys cleared, no
  cooldown;
- teleport (`ToRoomId` not reachable by an exit): nothing queued;
- temp exit with a `Title` differing from its key: the key is queued (D3);
- arrival, shadower not hidden: condition 87 removed, keys cleared, cooldown
  set, spotted line to a player shadower;
- arrival, still hidden: the sense roll runs; the line reaches a player target
  only (forced outcomes via extreme stats, as `TestShadow_DetectionWin` does);
  Skullduggery awarded on both outcomes.

**Action tests.** `shadow_test.go` gains the mob-target start roll and the
award-on-both-outcomes checks for `ShadowSenseRoll`; `ClearShadow` versus
`EndShadow` cooldown behaviour. `RelocateMob` gains a test that a sneaking mob
sends no exit or entry line (D1).

**Guard (NEW, repo root `shadow_follow_guard_test.go`, package `main`).**
Modelled on `speech_wrapper_guard_test.go` (G4): comment-stripped source,
walked over non-test `.go` under `internal/` and `modules/`.
- The strings `shadow-target-user` and `shadow-target-mob` appear only in
  `internal/actions/`. Proof it can match: on master it finds five files
  outside `internal/actions` (S13).
- Condition 87 by literal (`Condition(87`, `= 87`) appears only in
  `internal/actions/shadow.go`. Proof: on master it finds `go.go:414`,
  `skill.skullduggery.shadow.go:95` and the hooks const (S14).
- `ShadowTargetOf(` together with `.Command(` in one file occurs only in
  `internal/hooks/RoomChange_ShadowFollow.go`, exactly once each.
- The deleted names `shadowIsTargetingUser`, `shadowDetectionRoll`,
  `inlineShadowEnd`, `endShadow(`, `MobRoomChangeShadowFollow` appear
  nowhere. Proof: all five are found on master.
- Vacuity floor: the walk parses at least 50 files.
Each assertion is proven able to fail by a temporary violation during the
plan, as the 5b guard was.

**Existing guards.** `contestSiteOwners` (G1): drop
`skill.skullduggery.shadow.go:shadowDetectionRoll` and
`shadow.go:shadowPlayer`, add `internal/actions/shadow.go:ShadowSenseRoll`
owned by this slice. `legacyLiteralFiles` (G2): drop nothing the tree still
holds; add `internal/hooks/RoomChange_ShadowFollow.go`.

**Docs.** `internal/actions/context.md` documents the five new symbols and
fixes the `ShadowOptions` block (G5); `internal/hooks/context.md` lists
`RoomChange_ShadowFollow.go`, drops the mob-only hook, and corrects the
`Death_MobTracking_Cleanup.go` name (G5). Patch notes and the README row
follow the slice 5 pattern.

**Gates.** Local gate plus boot check (CI minutes are exhausted for September).

**Playtest.** Three characters: an admin operator, a target player with low
Perception and no Search, and a witness player. A lit room holding a
non-wandering mob fixture with good Dexterity and a named exit route of three
or more rooms.

*Mob shadows player.*
1. Operator alone in the room with the mob (T7: every player present is an
   observer the sneak must beat, the operator included). Run
   `command <mob> sneak` until the mob no longer appears in `look`.
   `setcondition <mob> 9` is NOT a fallback: it adds the condition without
   hiding the mob (T6).
2. Target and witness enter. If entry detection reveals the mob (the target's
   or witness's newcomer roll, E1), send them out and repeat step 1.
3. `command <mob> shadow <target>` (T4: the operator can name a hidden mob).
4. Target walks the route by named exits. After each step the operator runs
   `locate <mob>` and confirms the mob is in the target's room. The target and
   witness report that no line names the mob leaving or arriving (D1), that
   the mob is absent from `look`, and whether the sense line appeared.

*Player shadows mob.* A player with Skullduggery 3 (`skillset`) sneaks,
`shadow <mob>`, and the operator walks the mob with `command <mob> <exit>`
through three rooms (the operator follows it, since `command` reaches only
its own room). The player arrives each step, or reads the spotted line and
stops.

*Covered by unit tests only:* spotted-on-arrival and teleport, since a live
roll cannot be forced.

## Out of scope

- Authoring mob behaviour that uses `try_shadow` (the behaviour unification
  arc; T3).
- A "target lost" rule for a shadower left behind (D4).
- Any change to how entry detection reveals a shadower (ruling 3).
- Hidden-mover sound effects beyond matching the player path (D1).
