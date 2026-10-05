# Shadow Follow Parity (Slice 6) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every shadower, player or mob, follows its quarry, player or mob, through one `RoomChange` listener, and the spotted check and the sense roll run on the shadower's own arrival, through shared bodies in `internal/actions`.

**Architecture:** `internal/actions/shadow.go` gains `ShadowingConditionId`, `ShadowTargetOf`, `ClearShadow`, `EndShadow` and `ShadowSenseRoll` and becomes the only reader of the two misc-data keys and the only place condition 87 is named. A new `hooks.RoomChangeShadowFollow` (with the unexported exit helper `shadowExitTo`, ruling D3) replaces the follow loop in `usercommands/go.go` and `MobRoomChange_ShadowFollow.go`. `actions.RelocateMob` takes the mover's sneaking state and goes quiet for a sneaking mob (ruling D1). `shadow stop` and the death and logoff cleanups route through the shared bodies (ruling D2), and a repo-root guard stops the logic forking again.

**Tech Stack:** Go 1.25, the `internal/events` queue, the `internal/actions` Actor seam, `combat.RunContest`, repo-root `go/ast` guard tests.

**Spec (binding):** `docs/superpowers/specs/2026-09-30-shadow-follow-parity-design.md`, owner decisions 1 to 8 and owner rulings D1 to D4 (every recommendation taken).

**Branch:** implementation branch `feature/shadow-follow`, cut from master AFTER this docs branch (`docs/shadow-follow-spec`) merges, in the worktree `C:/tmp/dogmud-shadow`:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feature/shadow-follow C:/tmp/dogmud-shadow origin/master
```

All paths below are relative to that worktree. Run Go commands from its root. Name `C:/tmp/dogmud-shadow` in the handoff memory while the branch is open (it must outlive the session); remove it with `git worktree remove C:/tmp/dogmud-shadow` once the PR merges.

---

## Facts verified against source (2026-09-30, worktree HEAD `bd2897473`)

Every row was read or grepped at `bd2897473` (the spec commit; its Go tree is byte-identical to master `22e0c4178`, the spec's base). `origin/master` has since moved to `5015de014` (#199, baubles slice D spec and plan): docs only, touching `docs/README.md` two rows below this plan's anchor and no Go file, so no row below moves. Rows marked **NEW** are facts the spec does not state, or states wrongly, and that change the plan. Every code block in Tasks 1 to 8 was applied to this worktree, built, and run through the full gate (`go test ./... -count=1`, `go vet ./...`, `gofmt -l`, `golangci-lint run --new-from-merge-base=origin/master`: all clean), then reverted. The tasks were then replayed in order from the plan text itself (blocks extracted from this file, every "replace" anchor confirmed unique in HEAD): each task's end state passed `go test ./... -count=1`, and every guard failure a task predicts (Tasks 3, 6) and every "passes before the refactor" claim (Tasks 6, 7) was seen. The guard probes in Task 8 were each run and seen to fail, and the comment probe to pass.

| # | Fact | Where |
|---|---|---|
| F1 | `Shadow(actor Actor, opts ShadowOptions) ShadowResult` requires `IsHidden()`, refuses in combat and on no target, then `TryCooldown(skills.Skullduggery.String("shadow"), fmt.Sprintf("%d rounds", cfg.ShadowCooldown))` before dispatching to `shadowMob` / `shadowPlayer` | `internal/actions/shadow.go:38-79` |
| F2 | `shadowMob` writes `SetMiscData("shadow-target-user", nil)` / `("shadow-target-mob", m.InstanceId)` at `:90-91`, `actor.AddCondition(87, "skill")` at `:92`, sends the start line, then `actor.AwardResolved(true, ...CandidateFor(Skullduggery))` at `:103` with no contest, then the quest notify | `shadow.go:82-123` |
| F3 | `shadowPlayer` writes the keys at `:137-138`, `AddCondition(87, "skill")` at `:139`, start line, quest notify, then the contest at `:164-169` (`sight := combat.SightRoom(actor.GetRoom())`, `CalcSneakScoreVsObserver(char, target, sight) * messaging.SightMult(char, sight)` against `CalcDetectionScore(target, sight)` via `combat.RunContest`, target as attacker), `AwardResolved(!detected, ...)` at `:179`, and `targetUser.SendText(CategorySystem, "You sense someone following close behind you.")` at `:181` | `shadow.go:129-189` |
| F4 | `ShadowOptions{TargetMobInstanceId int; TargetUserId int}`, `ShadowResult{Succeeded, Detected, TargetName, OnCooldown, Reason}` | `shadow.go:18-30` |
| F5 | `usercommands.Shadow` handles `shadow stop` by reading both keys raw (`:42-43`) and calling `endShadow(user, "You stop shadowing your target.")`; `endShadow` (`:92-105`), `getShadowTargetUserId` (`:108-117`, no caller), `shadowIsTargetingUser` (`:121-128`), `shadowDetectionRoll` (`:135-146`, a `RunContest` site). Imports `combat`, `configs`, `contest` are used only by those helpers | `internal/usercommands/skill.skullduggery.shadow.go` |
| F6 | The player-mover follow loop is `go.go:394-435` (comment at `:394`, loop `:396-435`, blank `:436`); it calls `shadowIsTargetingUser` `:407`, `HasCondition(87)` `:414`, the keys `:415-416`, `shadowP.Command(rest)` `:420`, `endShadow` `:426`, `shadowDetectionRoll` `:431`. Deleting it leaves every `go.go` import in use (built) | `internal/usercommands/go.go` |
| F7 | A sneaking player sends only the self line; the room exit line, entry line and `destRoom.SendTextToExits` are in the `else` of `if isSneaking` (`go.go:296-356`). **D1 sounds, verified:** `room.PlaySound("room-exit", ...)` and `destRoom.PlaySound("room-enter", ...)` at `go.go:620-621` run for EVERY walk, sneaking or not. So a sneaking mob keeps its two sounds | `go.go:296-356,620-621` |
| F8 | `MobRoomChangeShadowFollow` (mob movers, player shadowers only, `FindExitTo`, no stale guard, dead spotted check) and `inlineShadowEnd`; the whole file is 103 lines; registered `hooks.go:39` | `internal/hooks/MobRoomChange_ShadowFollow.go`; `internal/hooks/hooks.go:39` |
| F9 | `const ( activeTrackingCondition = 86; shadowingCondition = 87 )` at `MobDeath_TrackingCleanup.go:9-12`; `shadowingCondition` used at `:54`, `PlayerDespawn_TrackingCleanup.go:48` and `MobRoomChange_ShadowFollow.go:52,93`; `activeTrackingCondition` at `MobDeath:46`, `PlayerDespawn:40`. Both cleanups take a `clearPointers(c interface{GetMiscData; SetMiscData; RemoveCondition})` closure called with `u.Character` and `&m.Character` (both `*characters.Character`), and clear ONLY the matching key plus condition 87, no cooldown, no line | `internal/hooks/MobDeath_TrackingCleanup.go:9-80`; `internal/hooks/PlayerDespawn_TrackingCleanup.go:20-71` |
| F10 | `RelocateMob(mob *mobs.Mob, from *rooms.Room, exitName string, dest *rooms.Room)`: exit line `:96-100`, entry line `:102-106`, `SendTextToExits` `:108`, sounds `:110-111`, no hidden check. Callers: `mobcommands/go.go:140` (its `sneaking` is computed at `:136-139` from `IsHidden()` or the `sneaking` misc flag), `hooks/NewRound_DoCombat_helpers.go:947` (flee), test `actions/relocate_mob_test.go:31` | `internal/actions/relocate_mob.go:84-114`; grep `RelocateMob(` |
| F11 | `UserRecord.Command(inputTxt string, waitSeconds ...float64)` and `Mob.Command(inputTxt string, waitSeconds ...float64)` only queue `events.Input`. **NEW:** the signatures are identical, so one `interface{ Command(string, ...float64) }` holds either, and the listener has ONE follow dispatch | `internal/users/userrecord.go:375-388`; `internal/mobs/mobs.go:942-972` |
| F12 | **NEW.** No test helper drains a USER's queued `Input`: `InspectQueuedInputForTest(instanceId, prefix)` (`:246`) and `DrainQueuedInputsForTest(instanceId)` (`:314`) both match `inp.MobInstanceId` only. Task 1 adds `DrainQueuedUserInputsForTest(userId)` | `internal/events/events.go:241-330` |
| F13 | `events.RoomChange{UserId, MobInstanceId, FromRoomId, ToRoomId, Unseen}`; the player move queues it in `rooms.MoveToRoom`, the mob move at the top of `Room.AddMob` (before `RoomId` is updated; the listener runs after, so it reads the updated `RoomId`) | `internal/events/eventtypes.go:192-198`; `internal/rooms/rooms.go:1222-1256` |
| F14 | `Room.FindExitTo` returns a temp exit's `Title` (`:2236-2240`); `AddTemporaryExit` defaults `Title` to the key only when empty (`:709-728`); active mutator exits are reached with `for mut := range r.ActiveMutators` and `mut.GetSpec().Exits` (`FindExitTo:2242-2249`, `ActiveMutators` is an iterator at `:3000`). `modules/follow` resolves the key from `Exits` then `ExitsTemp` and queues with a `.25` wait | `internal/rooms/rooms.go`; `modules/follow/follow.go:253-325` |
| F15 | **NEW.** `Character.RemoveCondition` only EXPIRES a held condition (`Conditions.RemoveCondition` sets `TriggersLeft = TriggersLeftExpired`, `conditions.go:131-137`); `HasCondition` stays true until `Conditions.Prune()` (`:550`) at the next turn (`hooks/NewTurn_PruneConditions.go:39`), which then sends a player the condition's `end_actee`, "Your focus on the quarry breaks." (`87-shadowing.yaml`). So tests prove condition 87 is gone by calling `Conditions.Prune()` first, and a spotted player reads the end line one turn after the spotted line, exactly as after `shadow stop` today | files named |
| F16 | **NEW, and the spec's S7 is wrong on it.** `ShadowCooldown` ships at `5` (`_datafiles/config.yaml:924`, same in the `git show HEAD:` blob), but the Go default is `0`: the struct field has no default and the validator is `if b.ShadowCooldown < 0 { = 5 }`, which cannot repair an absent 0 (`config.balance.combat.go:271-272`; the trap is named at `config.balance.misc.go:246-253`). Measured in the test binary: `ShadowCooldown=0`, and `Cooldowns.Try(tag, "0 rounds")` stores `1` (`gametime AddPeriod("0 rounds")` is one round, `cooldowns.go:31-56`). So tests assert a RUNNING cooldown (`> 0`), never `5` | measured with a throwaway test |
| F17 | **NEW.** `ContestFloor` is `0.125` in the Go defaults (`config.balance.misc.go:169-170`) and `contest.RunWithFloors` flips any outcome with that probability (`contest.go:150-173`), so no single sense roll can be forced by stats. Tests roll 200 times and require both outcomes (chance of missing one: below `0.875^200`, about 3e-12). The 2.3% attack fumble is a combat-attack rule and does not touch `RunContest` | files named |
| F18 | **NEW.** A real actor's award is observable: `Character.AwardResolved` ends in `ApplyProgression`, whose `OnSkillUseScaled` increments `SkillUseCount` for the winning candidate on a WON and a LOST award alike, read with `GetSkillUseCount("skullduggery")` (measured: exactly one per sense roll, both outcomes). Stub actors record awards in `awardRecorder.awards` (`{won, cands}`) | `internal/characters/progression_award_resolved.go:32-83`; `internal/actions/testsupport_test.go:58-95` |
| F19 | Actor seam: `Actor` interface `actor.go:14-63`; `NewUserActor`, `NewUserActorInRoom` `actor_user.go:23,30`; `NewMobActorInRoom` `actor_mob.go:30`; `MobActor.SendText` is a no-op (`actor_mob.go:43`), so a mob target "gets nothing visible" by construction | `internal/actions/` |
| F20 | Fixtures: `users.NewTestUser(id, username, name, connId)` (Perception, Dex 100; `Awareness` machine set), `users.SeedUsersForTest(map) func()`, `mobs.SeedMobsForTest(specs, instances) func()`, `mobs.SetInstanceForTest(id, m)`, `rooms.SeedRoomsForTest(rooms, zones) func()`, `conditions.SeedConditionsForTest(map) func()` (REPLACES the whole registry; restore with its returned func). Hiding: `Awareness.ForceVisible(r)`, `TransitionToConcealing(awareness.ConcealingData{}, r)`, `ResolveConcealment(true, r)` (`awareness.go:151,168,234`). actions test helpers: `newShadowPlayerActor(dex, rank, hidden)` `shadow_test.go:64`, `newShadowMobActor` `:85`, `resetHiddenCondition` `:106`, package `init` seeds only condition 9 (`:22-36`), `stubActorWithId` `steal_test.go:77`, `newStealTestRoom()` (`RoomId 99998`) `steal_test.go:38` | files named |
| F21 | **NEW.** No existing test calls `endShadow`, `shadowDetectionRoll`, `shadowIsTargetingUser`, `getShadowTargetUserId`, `inlineShadowEnd`, `MobRoomChangeShadowFollow`, `MobDeathTrackingCleanup` or `PlayerDespawnTrackingCleanup` (grep over `*_test.go`), so no test is deleted; `shadow_test.go`'s six tests keep passing unchanged (run) | grep |
| F22 | `contestSiteOwners` keys `"internal/actions/shadow.go:shadowPlayer"` (`:72`) and `"internal/usercommands/skill.skullduggery.shadow.go:shadowDetectionRoll"` (`:73`); `TestEveryContestSiteIsOwned` (`:232`) fails on an unowned site and on a stale row. **NEW:** the `shadowDetectionRoll` key is the LONGEST in its block, so deleting its row makes `gofmt` realign all 16 rows of the block (`:60-75`); Task 6 shows the whole block | `internal/combat/contest_site_guard_test.go:57-75,232-266` |
| F23 | `legacyLiteralFiles` (`:360-374`) lists `internal/actions/shadow.go`, `internal/usercommands/go.go`, `internal/usercommands/skill.skullduggery.shadow.go` (all survive) and `Position_GrappleTick.go` at `:365`; a listed file that does not exist is `t.Fatalf` | `contest_site_guard_test.go:360-374,396-413` |
| F24 | **NEW (spec silent).** `messaging_surface_guard_test.go` registers `"actions/shadow.go|You begin shadowing <ansi fg=\"username\">%s</ansi>,"` at `:1278` as actor+actee (the actee is `targetUser.SendText` in `shadowPlayer`). Once that send moves into `ShadowSenseRoll` (receiver `target`, no actor call) the event is actor-only, which the walk does not track, so `TestNarrationSitesMatchViewpointAudit` (`:1465`) reports the key STALE. Measured: it is the only narration key this slice moves; the entry says "Not part of the 2026-09-07 audit", so no audit doc changes with it | `messaging_surface_guard_test.go:1267,1278,1465-1523` |
| F25 | Guards read and found unaffected (measured by the full run): `lookup_viewer_guard_test.go:78` (`usercommands/.../shadow.go|Shadow` `{viewer: 1, plain: 1}`; the stop branch does no lookup), `move_wrapper_guard_test.go:16-27` (forbids `RunContest` etc. in both `go.go`; deletion only helps), `condition_apply_path_guard_test.go` (no key in a touched file), `timed_state_guard_test.go` (methods only; the new bodies are functions), `raw_events_message_guard_test.go` (no new `events.Message{`), `internal/progression/seam_guard_test.go` (actor `AwardResolved` only), `bauble_finder_view_guard_test.go`, `internal/actions/command_readiness_drift_test.go`. No guard keys a touched file by LINE NUMBER (grep `\|[0-9]` over `*_guard_test.go`) | files named |
| F26 | Model guard: `speech_wrapper_guard_test.go` (package `main`) strips comments with `parser.ParseFile(..., 0)` + `printer.Fprint` in `speechGuardCode` (`:59`) and walks with `speechGuardWalk`. The new guard's helpers are named `shadowGuardCode` / `shadowGuardWalk`; no root test declares those names | repo root |
| F27 | On this tree the guard's patterns match: the two keys in five files outside `internal/actions` (`MobDeath_TrackingCleanup.go`, `MobRoomChange_ShadowFollow.go`, `PlayerDespawn_TrackingCleanup.go`, `usercommands/go.go`, `skill.skullduggery.shadow.go`); `Condition\(87\b|=\s*87\b` at `go.go:414`, `skill.skullduggery.shadow.go:95`, `MobDeath_TrackingCleanup.go:11` (plus `shadow.go:92,139` via `(87,`, which the new code drops); all six deleted names. Non-Go data also names a key (`tools/playtest/profiles/veteran.yaml:1294`, `shadow-target-mob: 6`), outside the guard's `.go` scope and still a valid save shape | grep |
| F28 | `hooks/context.md` file table names `Death_MobTracking_Cleanup.go` (`:1211`, the file is `MobDeath_TrackingCleanup.go`) and `PlayerDespawn_TrackingCleanup.go` (`:1219`); the prefix census at `:2158-2168` is stale: measured today 139 non-test files, 100 prefixed, 39 helpers, `RoomChange_*` 4 and `MobRoomChange_*` 3. After this slice: 139, 100, 39, `RoomChange_*` 5, `MobRoomChange_*` 2 (joins the prefixes with one or two files, 33 of them) | `internal/hooks/context.md`; `ls internal/hooks/*.go` |
| F29 | `actions/context.md` is stale on shadow: `RelocateMob` signature `:303`; the `### Shadow` section `:1258-1296` (wrong fields `Success`/`Message`, "no cooldown", "no progression", follow via `modules/follow`); table row `:1540`; `ShadowOptions` block `:1581-1585` (`TargetUserId string; TargetMobId int`); `internal/modules/follow — Auto-follow (used by Shadow)` `:1720` (Shadow never used it). `usercommands/context.md:270-276` says a contest remains in `skill.skullduggery.shadow.go`; `mobcommands/context.md:276-281` describes `RelocateMob`; `events/context.md:586-609` lists the drain seams | files named |
| F30 | Admin tools for the playtest: `command <mob> <cmd>` with no viewer, so a hidden mob can be named (`admin.command.go:43-66`); `locate <name>` (`admin.locate.go:20-80`); `setcondition <mob> 9` does NOT hide a mob (condition 9 mirrors the awareness machine one way, spec T6); `actions.Sneak` rolls a mob against every player present, staff included (spec T7); `skillset` (`admin.skillset.go`) | files named |
| F31 | `docs/PATCH_NOTES.md` top entry is `## 2026-09-30: Voices in the dark` (`:3`); `docs/README.md` spec row for this slice is `:166`, the 5b plan row `:164`, the 5b guard row `:165` | files named |

## Where the spec could not be implemented as written

1. **S7 "the Go default is also 5" is false (F16).** The test binary reads `ShadowCooldown = 0`, which `Cooldowns.Try` turns into a one-round cooldown. Production reads `5` from `config.yaml`. No code changes for this: the plan's tests assert a running cooldown (`> 0`) rather than 5, and the validator's `< 0` repair is left alone, because an absent key reading 0 is a documented, legal shipped state (`dogmud-balance-config`) and retuning is out of this slice's scope. Reported to the owner in the PR.
2. **"condition 87 removed" cannot be observed as `!HasCondition(87)` (F15).** `RemoveCondition` expires the condition and the next turn's prune drops it; tests call `Conditions.Prune()` before asserting. Production behaviour is unchanged from `endShadow` today, including the end line "Your focus on the quarry breaks." one turn later.
3. **The guard's third assertion counts `.Command(` and `ShadowTargetOf(` once each (spec wording).** The listener satisfies it by design, not by dodging: one `shadowFollower` list holds players and mobs behind the shared `Command` signature (F11), and one `shadowOf` helper serves both passes. The spec did not say how; this is the shape.
4. **One messaging-surface key goes stale (F24).** The spec's guard list does not name `messaging_surface_guard_test.go`; Task 3 deletes the one stale entry with its reason, after reading it against source as the guard asks.
5. **The keys guard allows all of `internal/actions/`, as the spec says,** although only `shadow.go` names them. Tightening it to one file is not what the spec wrote; the condition-87 check is already file-scoped, as the spec wrote.

## Player-visible lines that change (everything else stays byte-identical)

| Line | Before | After |
|---|---|---|
| `You sense someone following close behind you.` (to a player quarry) | Rolled only when a PLAYER shadower followed a player's walk, before the shadower moved, in the destination's light, awarding nothing; never for a mob shadower, a flee or a scripted move | Rolled each time any shadower, player or mob, arrives in the player quarry's room, by any named exit (walk, flee, scripted move), in the shared room's light; the shadower trains Skullduggery on both outcomes. A mob quarry reads nothing |
| `You've been spotted -- your shadow ends.` (to the shadower) | Never sent (both spotted checks were dead) | Sent to a player shadower that arrives in its quarry's room no longer hidden; the shadow ends with the cooldown, and one turn later the condition's own end line, `Your focus on the quarry breaks.`, follows (as after `shadow stop` today) |
| A sneaking mob's `<name> leaves towards the <exit> exit.` / `You hear footsteps moving away.` (old room), `<name> enters from <exit>.` / `You hear footsteps approaching.` (new room), `You hear someone moving around.` (next rooms) | Sent for every mob, hidden or not | Not sent when the mob is hidden or carries the `sneaking` flag (walk), or is hidden (flee). The movement sound effects still play, as for a sneaking player |
| `shadow stop` lines, the start lines, `You need to wait %s before shadowing again.` | | Unchanged |

## File map

| File | Change |
|---|---|
| `internal/events/events.go` | Add `DrainQueuedUserInputsForTest` |
| `internal/events/user_input_drain_test.go` | Create |
| `internal/actions/shadow.go` | Keys as consts, `ShadowingConditionId`, `ShadowTargetOf`, `ClearShadow`, `EndShadow` (Task 2); `ShadowSenseRoll`, both start paths roll it (Task 3) |
| `internal/actions/shadow_state_test.go` | Create (Task 2) |
| `internal/actions/shadow_sense_test.go` | Create (Task 3) |
| `messaging_surface_guard_test.go` | Delete the stale `actions/shadow.go|You begin shadowing ...` entry (Task 3) |
| `internal/combat/contest_site_guard_test.go` | `shadowPlayer` row becomes `ShadowSenseRoll` (Task 3); `legacyLiteralFiles` gains the listener (Task 5); `shadowDetectionRoll` row deleted, block realigned (Task 6) |
| `internal/actions/relocate_mob.go`, `relocate_mob_test.go` | `sneaking bool` parameter; quiet sneaking mob (Task 4) |
| `internal/mobcommands/go.go`, `internal/hooks/NewRound_DoCombat_helpers.go` | Pass the sneaking state (Task 4) |
| `internal/hooks/RoomChange_ShadowFollow.go` | Create: `RoomChangeShadowFollow`, `shadowFollowPass`, `shadowArrivalPass`, `shadowOf`, `shadowNamesMover`, `shadowExitTo` (Task 5) |
| `internal/hooks/RoomChange_ShadowFollow_test.go` | Create: the mover x shadower table (Task 5) |
| `internal/hooks/hooks.go` | Register `RoomChangeShadowFollow` in place of `MobRoomChangeShadowFollow` (Task 5) |
| `internal/hooks/MobRoomChange_ShadowFollow.go` | Delete (Task 5) |
| `internal/usercommands/go.go` | Delete the follow loop (Task 5) |
| `internal/usercommands/skill.skullduggery.shadow.go` | `shadow stop` through the shared bodies; four helpers deleted (Task 6) |
| `internal/usercommands/shadow_stop_test.go` | Create (Task 6) |
| `internal/hooks/MobDeath_TrackingCleanup.go`, `PlayerDespawn_TrackingCleanup.go` | `ShadowTargetOf` + `ClearShadow`; `shadowingCondition` const deleted (Task 7) |
| `internal/hooks/TrackingCleanup_Shadow_test.go` | Create (Task 7) |
| `shadow_follow_guard_test.go` | Create, repo root (Task 8) |
| `context.md` in `internal/actions`, `internal/hooks`, `internal/usercommands`, `internal/mobcommands`, `internal/events` | Update (Task 9) |
| `docs/PATCH_NOTES.md`, `docs/README.md` | Entry; row for the new guard (Task 9) |

---

### Task 1: A queue helper that drains a user's queued commands

**Model:** haiku (mechanical, code given).

**Files:**
- Modify: `internal/events/events.go` (after `DrainQueuedInputsForTest`, which ends at `:330`)
- Create: `internal/events/user_input_drain_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/events/user_input_drain_test.go`:

```go
package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A user's queued Input is drained by user id; a mob's Input and another
// user's stay queued.
func TestDrainQueuedUserInputsForTest(t *testing.T) {
	DrainQueuedUserInputsForTest(7301)
	DrainQueuedUserInputsForTest(7302)
	DrainQueuedInputsForTest(7301)

	AddToQueue(Input{UserId: 7301, InputText: "north"})
	AddToQueue(Input{UserId: 7302, InputText: "south"})
	AddToQueue(Input{MobInstanceId: 7301, InputText: "east"})

	require.Equal(t, []string{"north"}, DrainQueuedUserInputsForTest(7301))
	require.Empty(t, DrainQueuedUserInputsForTest(7301), "a drained input is gone")
	require.Equal(t, []string{"south"}, DrainQueuedUserInputsForTest(7302))
	require.Equal(t, []string{"east"}, DrainQueuedInputsForTest(7301), "a mob input with the same id is not a user input")
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/events -run TestDrainQueuedUserInputsForTest -count=1`
Expected: FAIL to build, `undefined: DrainQueuedUserInputsForTest`.

- [ ] **Step 3: Add the helper**

In `internal/events/events.go`, insert immediately above the line `// DrainQueuedBroadcastsForTest removes all Broadcast events from the global`:

```go
// DrainQueuedUserInputsForTest removes all Input events from the global queue
// for the given user id (UserRecord.Command queues them with MobInstanceId 0)
// and returns their InputText values. The player twin of
// DrainQueuedInputsForTest.
//
// FOR TEST USE ONLY. Mutates the queue.
func DrainQueuedUserInputsForTest(userId int) []string {
	qLock.Lock()
	defer qLock.Unlock()
	var found []string
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		inp, ok := pe.event.(Input)
		if ok && inp.MobInstanceId == 0 && inp.UserId == userId {
			found = append(found, inp.InputText)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

```

- [ ] **Step 4: Run it to see it pass**

Run: `go test ./internal/events -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/events`.

- [ ] **Step 5: Commit**

```bash
git add internal/events/events.go internal/events/user_input_drain_test.go
git commit -m "test(events): drain a user's queued commands" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: The shadow's state behind shared bodies: `ShadowingConditionId`, `ShadowTargetOf`, `ClearShadow`, `EndShadow`

**Model:** haiku (mechanical, code given).

**Files:**
- Modify: `internal/actions/shadow.go` (imports `:3-14`, `shadowMob` `:90-92`, `shadowPlayer` `:137-139`, end of file)
- Create: `internal/actions/shadow_state_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/shadow_state_test.go`:

```go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// seedShadowingCondition makes condition 87 appliable for one test.
// SeedConditionsForTest replaces the whole registry, and its returned func
// restores the package's own seed (condition 9) afterwards.
func seedShadowingCondition(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		ShadowingConditionId: {ConditionId: ShadowingConditionId, Name: "Shadowing", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25},
	}))
}

// shadowingChar is a character shadowing player 7320 with condition 87 held.
func shadowingChar(t *testing.T) *characters.Character {
	t.Helper()
	c := characters.New()
	c.SetMiscData(shadowTargetUserKey, 7320)
	if err := c.AddCondition(ShadowingConditionId, false); err != nil {
		t.Fatalf("add condition 87: %v", err)
	}
	return c
}

// shadowConditionEnded reports whether condition 87 is gone once the turn's
// prune runs. RemoveCondition only expires a condition: HasCondition stays
// true until Conditions.Prune drops it (hooks/NewTurn_PruneConditions.go).
func shadowConditionEnded(c *characters.Character) bool {
	c.Conditions.Prune()
	return !c.HasCondition(ShadowingConditionId)
}

// ShadowTargetOf reads whichever key is set, and zeros for none.
func TestShadowTargetOf_ReadsEitherKey(t *testing.T) {
	c := characters.New()
	if u, m := ShadowTargetOf(c); u != 0 || m != 0 {
		t.Errorf("no shadow: got (%d, %d), want (0, 0)", u, m)
	}
	c.SetMiscData(shadowTargetUserKey, 7330)
	if u, m := ShadowTargetOf(c); u != 7330 || m != 0 {
		t.Errorf("player quarry: got (%d, %d), want (7330, 0)", u, m)
	}
	c.SetMiscData(shadowTargetUserKey, nil)
	c.SetMiscData(shadowTargetMobKey, 88330)
	if u, m := ShadowTargetOf(c); u != 0 || m != 88330 {
		t.Errorf("mob quarry: got (%d, %d), want (0, 88330)", u, m)
	}
	if u, m := ShadowTargetOf(nil); u != 0 || m != 0 {
		t.Errorf("nil character: got (%d, %d), want (0, 0)", u, m)
	}
}

// Shadow stores what ShadowTargetOf reads back.
func TestShadowTargetOf_ReadsWhatShadowStored(t *testing.T) {
	target := users.NewTestUser(7331, "stored", "Stored", 0)
	cleanup := users.SeedUsersForTest(map[int]*users.UserRecord{7331: target})
	defer cleanup()
	actor := newShadowPlayerActor(100, 5, true)

	if result := Shadow(actor, ShadowOptions{TargetUserId: 7331}); !result.Succeeded {
		t.Fatalf("shadow did not start: %+v", result)
	}
	if u, m := ShadowTargetOf(actor.char); u != 7331 || m != 0 {
		t.Errorf("ShadowTargetOf = (%d, %d), want (7331, 0)", u, m)
	}
}

// ClearShadow is the silent drop: the target and condition 87 go, and no
// cooldown starts (the stale guard and the death and logoff cleanups).
func TestClearShadow_DropsTargetAndConditionWithoutCooldown(t *testing.T) {
	seedShadowingCondition(t)
	c := shadowingChar(t)
	c.SetMiscData(shadowTargetMobKey, 88320)

	ClearShadow(c)

	if userId, mobInstanceId := ShadowTargetOf(c); userId != 0 || mobInstanceId != 0 {
		t.Errorf("ShadowTargetOf after ClearShadow = (%d, %d), want (0, 0)", userId, mobInstanceId)
	}
	if !shadowConditionEnded(c) {
		t.Error("condition 87 survived ClearShadow")
	}
	if got := c.GetCooldown(skills.Skullduggery.String("shadow")); got != 0 {
		t.Errorf("ClearShadow started a %d round cooldown; it must start none", got)
	}
}

// EndShadow is ClearShadow plus the cooldown and the reason line.
func TestEndShadow_ClearsStartsTheCooldownAndTellsTheActor(t *testing.T) {
	seedShadowingCondition(t)
	const userId = 7321
	u := users.NewTestUser(userId, "ender", "Ender", 0)
	u.Character = shadowingChar(t)
	events.DrainQueuedMessagesForTest(userId)

	EndShadow(NewUserActor(u), "You stop shadowing your target.")

	if tu, tm := ShadowTargetOf(u.Character); tu != 0 || tm != 0 {
		t.Errorf("ShadowTargetOf after EndShadow = (%d, %d), want (0, 0)", tu, tm)
	}
	if !shadowConditionEnded(u.Character) {
		t.Error("condition 87 survived EndShadow")
	}
	// The test binary reads the Go default ShadowCooldown (0), which
	// Cooldowns.Try rounds up to one round; production ships 5. Either way
	// a cooldown is running.
	if got := u.Character.GetCooldown(skills.Skullduggery.String("shadow")); got <= 0 {
		t.Errorf("EndShadow left cooldown %d, want a running cooldown", got)
	}
	msgs := events.DrainQueuedMessagesForTest(userId)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "You stop shadowing your target.") {
		t.Errorf("actor messages %q, want the one reason line", msgs)
	}
}

// An empty reason sends nothing.
func TestEndShadow_EmptyReasonIsSilent(t *testing.T) {
	seedShadowingCondition(t)
	const userId = 7322
	u := users.NewTestUser(userId, "quiet", "Quiet", 0)
	u.Character = shadowingChar(t)
	events.DrainQueuedMessagesForTest(userId)

	EndShadow(NewUserActor(u), "")

	if msgs := events.DrainQueuedMessagesForTest(userId); len(msgs) != 0 {
		t.Errorf("an empty reason sent %q", msgs)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/actions -run 'TestShadowTargetOf|TestClearShadow|TestEndShadow' -count=1`
Expected: FAIL to build, `undefined: ShadowingConditionId`, `undefined: shadowTargetUserKey`, `undefined: ShadowTargetOf` (and the rest).

- [ ] **Step 3: Implement**

In `internal/actions/shadow.go`:

(a) Add the `characters` import. Replace

```go
import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/combat"
```

with

```go
import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
```

(b) Insert immediately above `// ShadowOptions parameterizes a shadow attempt.`:

```go
// ShadowingConditionId is condition 87, "Shadowing": held while a shadow is
// live. It is named here and nowhere else (shadow_follow_guard_test.go).
const ShadowingConditionId = 87

// The two misc-data keys that hold a shadow's quarry. At most one is set.
// Only this file reads or writes them (shadow_follow_guard_test.go); every
// other package goes through ShadowTargetOf, ClearShadow and EndShadow.
const (
	shadowTargetUserKey = "shadow-target-user"
	shadowTargetMobKey  = "shadow-target-mob"
)

```

(c) In `shadowMob`, replace

```go
	char.SetMiscData("shadow-target-user", nil)
	char.SetMiscData("shadow-target-mob", m.InstanceId)
	actor.AddCondition(87, "skill")
```

with

```go
	char.SetMiscData(shadowTargetUserKey, nil)
	char.SetMiscData(shadowTargetMobKey, m.InstanceId)
	actor.AddCondition(ShadowingConditionId, "skill")
```

(d) In `shadowPlayer`, replace

```go
	char.SetMiscData("shadow-target-user", targetUser.UserId)
	char.SetMiscData("shadow-target-mob", nil)
	actor.AddCondition(87, "skill")
```

with

```go
	char.SetMiscData(shadowTargetUserKey, targetUser.UserId)
	char.SetMiscData(shadowTargetMobKey, nil)
	actor.AddCondition(ShadowingConditionId, "skill")
```

(e) Append at the end of the file:

```go

// ShadowTargetOf returns the quarry c is shadowing: a player's user id or a
// mob's instance id, the other zero, or both zero when c shadows no one. It
// reads the target only; whether the shadow is live is
// c.HasCondition(ShadowingConditionId).
func ShadowTargetOf(c *characters.Character) (userId, mobInstanceId int) {
	if c == nil {
		return 0, 0
	}
	userId, _ = c.GetMiscData(shadowTargetUserKey).(int)
	mobInstanceId, _ = c.GetMiscData(shadowTargetMobKey).(int)
	return userId, mobInstanceId
}

// ClearShadow drops c's shadow outright: both target keys and
// ShadowingConditionId. No cooldown and no message, which is what the
// stale-state guard and the target death and logoff cleanups want. A shadow
// that ENDS (spotted, or `shadow stop`) goes through EndShadow instead.
func ClearShadow(c *characters.Character) {
	if c == nil {
		return
	}
	c.SetMiscData(shadowTargetUserKey, nil)
	c.SetMiscData(shadowTargetMobKey, nil)
	c.RemoveCondition(ShadowingConditionId)
}

// EndShadow ends actor's shadow: ClearShadow, then the shadow cooldown
// (Balance.ShadowCooldown rounds), then reason to the actor when it is not
// empty (a mob reads nothing).
func EndShadow(actor Actor, reason string) {
	char := actor.GetCharacter()
	ClearShadow(char)

	cfg := configs.GetBalanceConfig()
	char.TryCooldown(skills.Skullduggery.String(`shadow`),
		fmt.Sprintf(`%d rounds`, cfg.ShadowCooldown))

	if reason != "" {
		actor.SendText(messaging.CategorySystem, reason)
	}
}
```

- [ ] **Step 4: Run the package**

Run: `gofmt -l internal/actions && go test ./internal/actions -count=1`
Expected: gofmt prints nothing; `ok  	github.com/GoMudEngine/GoMud/internal/actions`. The six existing `TestShadow_*` tests pass unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/actions/shadow.go internal/actions/shadow_state_test.go
git commit -m "feat(actions): shadow state behind ShadowTargetOf, ClearShadow and EndShadow" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: `ShadowSenseRoll`, and a mob quarry rolls when a shadow starts

**Model:** sonnet (moves a contest site and a narration site; two guards follow it).

**Files:**
- Modify: `internal/actions/shadow.go` (whole file)
- Create: `internal/actions/shadow_sense_test.go`
- Modify: `internal/combat/contest_site_guard_test.go:72` (the `shadowPlayer` row)
- Modify: `messaging_surface_guard_test.go:1278` (one stale entry)

Guards this task moves (F22, F24): `TestEveryContestSiteIsOwned` would report `internal/actions/shadow.go:ShadowSenseRoll` as a NEW site and `internal/actions/shadow.go:shadowPlayer` as STALE; `TestNarrationSitesMatchViewpointAudit` would report `actions/shadow.go|You begin shadowing <ansi fg="username">%s</ansi>,` as STALE, because the quarry's line now lives in `ShadowSenseRoll` and `shadowPlayer`'s event is actor-only. Both are re-keyed in this task. `shadowDetectionRoll` still exists until Task 6, so its row stays.

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/shadow_sense_test.go`:

```go
package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// shadowSenseTrials is how many rolls a sense test makes. The contest floor
// (Balance.ContestFloor, 0.125 in the Go defaults) gives each outcome at least
// a one in eight chance on every roll, so no single roll can be forced; over
// 200 rolls the chance of never seeing one outcome is below 0.875^200, about
// 3e-12.
const shadowSenseTrials = 200

// countSensedLines counts the sense line among userId's queued messages and
// drains them.
func countSensedLines(userId int) int {
	n := 0
	for _, msg := range events.DrainQueuedMessagesForTest(userId) {
		if strings.Contains(msg, shadowSensedLine) {
			n++
		}
	}
	return n
}

// A player target reads the sense line exactly when it senses the shadower,
// and the shadower trains Skullduggery on every roll, a win exactly when it
// went unsensed.
func TestShadowSenseRoll_PlayerTargetReadsTheLineAndBothOutcomesAward(t *testing.T) {
	const targetId = 7311
	target := users.NewTestUser(targetId, "senser", "Senser", 0)
	shadower := newShadowMobActor(100, 0, true)
	room := newStealTestRoom()
	targetActor := NewUserActorInRoom(target, room)
	events.DrainQueuedMessagesForTest(targetId)

	detected := 0
	for i := 0; i < shadowSenseTrials; i++ {
		if ShadowSenseRoll(shadower, targetActor, room) {
			detected++
		}
	}

	if detected == 0 || detected == shadowSenseTrials {
		t.Fatalf("detected %d of %d: both outcomes must occur for this test to prove anything", detected, shadowSenseTrials)
	}
	if got := countSensedLines(targetId); got != detected {
		t.Errorf("target read the sense line %d times, want %d (once per detection)", got, detected)
	}
	if len(shadower.awards) != shadowSenseTrials {
		t.Fatalf("shadower got %d awards over %d rolls, want one per roll on both outcomes", len(shadower.awards), shadowSenseTrials)
	}
	lost := 0
	for _, a := range shadower.awards {
		if !a.won {
			lost++
		}
		if len(a.cands) != 1 || a.cands[0].Skill != string(skills.Skullduggery) {
			t.Fatalf("award candidates %+v, want Skullduggery alone", a.cands)
		}
	}
	if lost != detected {
		t.Errorf("%d lost awards, want %d: a detected roll is the shadower's loss", lost, detected)
	}
}

// A mob target gets nothing visible, but the roll is real: it senses the
// shadower some of the time and the shadower trains either way.
func TestShadowSenseRoll_MobTargetShowsNothingAndBothOutcomesAward(t *testing.T) {
	const observerId = 7312
	observer := users.NewTestUser(observerId, "bystander", "Bystander", 0)
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{observerId: observer})
	defer cleanupUsers()

	m := &mobs.Mob{InstanceId: 88312, Character: *characters.New()}
	m.Character.Name = "Watcher"
	m.Character.Stats.Perception.ValueAdj = 100
	room := newStealTestRoom()
	room.AddPlayer(observerId)
	defer room.RemovePlayer(observerId)

	shadower := newShadowPlayerActor(100, 0, true)
	events.DrainQueuedMessagesForTest(observerId)

	detected := 0
	for i := 0; i < shadowSenseTrials; i++ {
		if ShadowSenseRoll(shadower, NewMobActorInRoom(m, room), room) {
			detected++
		}
	}

	if detected == 0 || detected == shadowSenseTrials {
		t.Fatalf("detected %d of %d: both outcomes must occur", detected, shadowSenseTrials)
	}
	if got := len(events.DrainQueuedMessagesForTest(observerId)); got != 0 {
		t.Errorf("a player in the room received %d messages; sensing is awareness only", got)
	}
	if len(shadower.awards) != shadowSenseTrials {
		t.Errorf("shadower got %d awards over %d rolls, want one per roll", len(shadower.awards), shadowSenseTrials)
	}
}

// Ruling 5: starting a shadow on a mob runs the same sense roll as on a
// player. Before, it ran no contest and awarded a win every time.
func TestShadow_MobTargetRollsTheSenseContest(t *testing.T) {
	const instId = 88313
	m := &mobs.Mob{InstanceId: instId, Character: *characters.New()}
	m.Character.Name = "Sentry"
	m.Character.Stats.Perception.ValueAdj = 100
	mobs.SetInstanceForTest(instId, m)
	defer mobs.SetInstanceForTest(instId, nil)

	actor := newShadowMobActor(100, 0, true)

	detected := 0
	for i := 0; i < shadowSenseTrials; i++ {
		delete(actor.char.Cooldowns, skills.Skullduggery.String("shadow"))
		resetHiddenCondition(actor)
		result := Shadow(actor, ShadowOptions{TargetMobInstanceId: instId})
		if !result.Succeeded {
			t.Fatalf("trial %d: shadow did not start: %+v", i, result)
		}
		if result.Detected {
			detected++
		}
		if last := actor.awards[len(actor.awards)-1]; last.won == result.Detected {
			t.Fatalf("trial %d: award won=%v with Detected=%v; the award must be the roll's outcome", i, last.won, result.Detected)
		}
	}

	if detected == 0 || detected == shadowSenseTrials {
		t.Fatalf("detected %d of %d: a mob target must really roll", detected, shadowSenseTrials)
	}
	if len(actor.awards) != shadowSenseTrials {
		t.Errorf("%d awards over %d starts, want one per start", len(actor.awards), shadowSenseTrials)
	}
	if userId, mobInstanceId := ShadowTargetOf(actor.char); userId != 0 || mobInstanceId != instId {
		t.Errorf("ShadowTargetOf = (%d, %d), want (0, %d)", userId, mobInstanceId, instId)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/actions -run 'TestShadowSenseRoll|TestShadow_MobTargetRollsTheSenseContest' -count=1`
Expected: FAIL to build, `undefined: ShadowSenseRoll` and `undefined: shadowSensedLine`.

- [ ] **Step 3: Replace `internal/actions/shadow.go` with the final version**

Replace the whole file with:

```go
package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ShadowingConditionId is condition 87, "Shadowing": held while a shadow is
// live. It is named here and nowhere else (shadow_follow_guard_test.go).
const ShadowingConditionId = 87

// The two misc-data keys that hold a shadow's quarry. At most one is set.
// Only this file reads or writes them (shadow_follow_guard_test.go); every
// other package goes through ShadowTargetOf, ClearShadow and EndShadow.
const (
	shadowTargetUserKey = "shadow-target-user"
	shadowTargetMobKey  = "shadow-target-mob"
)

// shadowSensedLine is what a player target reads when it senses a shadower.
const shadowSensedLine = "You sense someone following close behind you."

// ShadowOptions parameterizes a shadow attempt.
// Exactly one of TargetMobInstanceId / TargetUserId must be set.
type ShadowOptions struct {
	TargetMobInstanceId int
	TargetUserId        int
}

// ShadowResult is the structured outcome of a shadow attempt.
type ShadowResult struct {
	Succeeded  bool   // target id was stored and shadow tracking began
	Detected   bool   // target won the initial sense roll
	TargetName string // display name of the target
	OnCooldown bool   // attempt was blocked by shadow cooldown
	Reason     string // when Succeeded==false and !OnCooldown, why
}

// Shadow attempts to track a target while hidden. The actor must be hidden
// (Character.IsHidden). On success it stores the target's id (ShadowTargetOf
// reads it back) and applies ShadowingConditionId, so
// hooks.RoomChangeShadowFollow moves the actor after its quarry. The target
// then makes the initial sense roll (ShadowSenseRoll), player or mob alike:
// if it wins it senses pursuit (Detected=true), but the shadow begins either
// way.
func Shadow(actor Actor, opts ShadowOptions) ShadowResult {
	char := actor.GetCharacter()

	// Must be hidden to shadow.
	if !char.IsHidden() {
		actor.SendText(messaging.CategorySystem,
			"You must be hidden to shadow someone. "+
				`Try <ansi fg="command">sneak</ansi> first.`)
		return ShadowResult{Reason: "not hidden"}
	}

	// Combat gate.
	if char.IsInCombat() {
		actor.SendText(messaging.CategorySystem, "You can't do that while in combat!")
		return ShadowResult{Reason: "in combat"}
	}

	// Require a target.
	if opts.TargetMobInstanceId == 0 && opts.TargetUserId == 0 {
		actor.SendText(messaging.CategorySystem, "Shadow whom?")
		return ShadowResult{Reason: "no target"}
	}

	cfg := configs.GetBalanceConfig()
	cooldownKey := skills.Skullduggery.String(`shadow`)

	// Check cooldown before doing target resolution.
	if !char.TryCooldown(cooldownKey,
		fmt.Sprintf(`%d rounds`, cfg.ShadowCooldown)) {
		return ShadowResult{
			OnCooldown: true,
			Reason: fmt.Sprintf("%d rounds remaining",
				char.GetCooldown(cooldownKey)),
		}
	}

	if opts.TargetMobInstanceId > 0 {
		return shadowMob(actor, opts.TargetMobInstanceId, cfg)
	}

	return shadowPlayer(actor, opts.TargetUserId, cfg)
}

// shadowMob handles the mob-target shadow path.
func shadowMob(actor Actor, mobInstanceId int, cfg configs.Balance) ShadowResult {
	m := mobs.GetInstance(mobInstanceId)
	if m == nil {
		actor.SendText(messaging.CategorySystem, "They seem to have vanished.")
		return ShadowResult{Reason: "target not found"}
	}

	char := actor.GetCharacter()
	char.SetMiscData(shadowTargetUserKey, nil)
	char.SetMiscData(shadowTargetMobKey, m.InstanceId)
	actor.AddCondition(ShadowingConditionId, "skill")

	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`You begin shadowing <ansi fg="mobname">%s</ansi>, `+
			`moving silently in their wake.`,
		m.Character.Name))

	// Parity slice 6, ruling 5: a mob target makes the same sense roll a
	// player target does. Until then this path ran no contest and awarded a
	// win outright. The roll awards Skullduggery on both outcomes.
	room := actor.GetRoom()
	detected := ShadowSenseRoll(actor, NewMobActorInRoom(m, room), room)

	// Quest engine notification — player actors only.
	if actor.IsPlayer() {
		if u := users.GetByUserId(actor.GetUserId()); u != nil {
			bridge := questengine.NewGameBridge(u, room.RoomId)
			questengine.GetEngine().Notify("command", questengine.EventDetails{
				UserId:  actor.GetUserId(),
				RoomId:  room.RoomId,
				Command: "shadow",
			}, bridge, bridge)

		}
	}

	return ShadowResult{
		Succeeded:  true,
		Detected:   detected,
		TargetName: m.Character.Name,
	}
}

// shadowPlayer handles the player-target shadow path. The target makes the
// initial sense roll (ShadowSenseRoll); the shadow begins either way, and
// Detected reports the roll.
func shadowPlayer(actor Actor, targetUserId int, cfg configs.Balance) ShadowResult {
	targetUser := users.GetByUserId(targetUserId)
	if targetUser == nil {
		actor.SendText(messaging.CategorySystem, "They seem to have vanished.")
		return ShadowResult{Reason: "target not found"}
	}

	char := actor.GetCharacter()
	char.SetMiscData(shadowTargetUserKey, targetUser.UserId)
	char.SetMiscData(shadowTargetMobKey, nil)
	actor.AddCondition(ShadowingConditionId, "skill")

	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`You begin shadowing <ansi fg="username">%s</ansi>, `+
			`watching their every move.`,
		targetUser.Character.Name))

	// Quest engine notification — player actors only.
	if actor.IsPlayer() {
		if u := users.GetByUserId(actor.GetUserId()); u != nil {
			room := actor.GetRoom()
			bridge := questengine.NewGameBridge(u, room.RoomId)
			questengine.GetEngine().Notify("command", questengine.EventDetails{
				UserId:  actor.GetUserId(),
				RoomId:  room.RoomId,
				Command: "shadow",
			}, bridge, bridge)

		}
	}

	room := actor.GetRoom()
	detected := ShadowSenseRoll(actor, NewUserActorInRoom(targetUser, room), room)

	return ShadowResult{
		Succeeded:  true,
		Detected:   detected,
		TargetName: targetUser.Character.Name,
	}
}

// ShadowSenseRoll is the one contest a shadow runs: does the target sense
// the shadower? The TARGET is the attacker (it is the one trying to notice),
// its CalcDetectionScore against the shadower's CalcSneakScoreVsObserver,
// with the shadower paying its sight ramp in room's light. It is rolled when
// a shadow starts and each time the shadower arrives in its quarry's room
// (hooks.RoomChangeShadowFollow).
//
// It awards the shadower's Skullduggery on both outcomes, the resolved-contest
// convention (U10b-2): a win when the target did not sense it. A player
// target that senses it reads shadowSensedLine; a mob target learns nothing
// visible (Actor.SendText is a no-op for a mob). It reveals nothing: a reveal
// is entry detection's job. It returns whether the target sensed the shadower.
func ShadowSenseRoll(shadower, target Actor, room *rooms.Room) bool {
	sight := combat.SightRoom(room)
	shadowChar := shadower.GetCharacter()
	targetChar := target.GetCharacter()

	sneakScore := CalcSneakScoreVsObserver(shadowChar, targetChar, sight)
	// sight ramp (plan 5b): the shadower needs to see the quarry to keep on it.
	sneakScore *= messaging.SightMult(shadowChar, sight)
	senseScore := CalcDetectionScore(targetChar, sight)

	detected := combat.RunContest(senseScore, []contest.Entry{{Score: sneakScore}}).Success
	shadower.AwardResolved(!detected, shadowChar.CandidateFor(string(skills.Skullduggery)))
	if detected {
		target.SendText(messaging.CategorySystem, shadowSensedLine)
	}
	return detected
}

// ShadowTargetOf returns the quarry c is shadowing: a player's user id or a
// mob's instance id, the other zero, or both zero when c shadows no one. It
// reads the target only; whether the shadow is live is
// c.HasCondition(ShadowingConditionId).
func ShadowTargetOf(c *characters.Character) (userId, mobInstanceId int) {
	if c == nil {
		return 0, 0
	}
	userId, _ = c.GetMiscData(shadowTargetUserKey).(int)
	mobInstanceId, _ = c.GetMiscData(shadowTargetMobKey).(int)
	return userId, mobInstanceId
}

// ClearShadow drops c's shadow outright: both target keys and
// ShadowingConditionId. No cooldown and no message, which is what the
// stale-state guard and the target death and logoff cleanups want. A shadow
// that ENDS (spotted, or `shadow stop`) goes through EndShadow instead.
func ClearShadow(c *characters.Character) {
	if c == nil {
		return
	}
	c.SetMiscData(shadowTargetUserKey, nil)
	c.SetMiscData(shadowTargetMobKey, nil)
	c.RemoveCondition(ShadowingConditionId)
}

// EndShadow ends actor's shadow: ClearShadow, then the shadow cooldown
// (Balance.ShadowCooldown rounds), then reason to the actor when it is not
// empty (a mob reads nothing).
func EndShadow(actor Actor, reason string) {
	char := actor.GetCharacter()
	ClearShadow(char)

	cfg := configs.GetBalanceConfig()
	char.TryCooldown(skills.Skullduggery.String(`shadow`),
		fmt.Sprintf(`%d rounds`, cfg.ShadowCooldown))

	if reason != "" {
		actor.SendText(messaging.CategorySystem, reason)
	}
}
```

- [ ] **Step 4: Run the package**

Run: `gofmt -l internal/actions && go test ./internal/actions -count=1`
Expected: gofmt prints nothing; `ok  	github.com/GoMudEngine/GoMud/internal/actions` (the six `TestShadow_*`, Task 2's tests and the three new ones pass).

- [ ] **Step 5: See the two guards fail as predicted**

Run: `go test ./internal/combat -run TestEveryContestSiteIsOwned -count=1` and `go test . -run TestNarrationSitesMatchViewpointAudit -count=1`
Expected: the first FAILs naming `NEW contest site internal/actions/shadow.go:ShadowSenseRoll` and `stale allowlist entry internal/actions/shadow.go:shadowPlayer`; the second FAILs with one stale entry, `actions/shadow.go|You begin shadowing <ansi fg="username">%s</ansi>,`. Any other failure is a surprise: stop and read it.

- [ ] **Step 6: Re-key the contest site**

In `internal/combat/contest_site_guard_test.go`, replace the line

```go
	"internal/actions/shadow.go:shadowPlayer":                                "U6b task 16",
```

with

```go
	"internal/actions/shadow.go:ShadowSenseRoll":                             "U6b task 16 (parity slice 6 lifted the shadow sense contest out of shadowPlayer into this one body; it runs when a shadow starts and on each arrival)",
```

- [ ] **Step 7: Retire the stale narration key**

Read `internal/actions/shadow.go` first and confirm the reason: `shadowPlayer` now sends only the actor's start line, and the quarry's line is `target.SendText` inside `ShadowSenseRoll` (receiver `target`, no actor call), so neither event is one the walk tracks. The entry says "Not part of the 2026-09-07 audit", so no audit document changes with it. Then in `messaging_surface_guard_test.go` delete this one line (`:1278`):

```go
	"actions/shadow.go|You begin shadowing <ansi fg=\"username\">%s</ansi>,":                                                {verdictCorrect, true, true, false, "shadow is a covert-observation skill; the target is privately notified they are being shadowed via a separate SendText this walk groups elsewhere, but the room is deliberately not told, which would defeat the point of a stealth skill. Not part of the 2026-09-07 audit; read against source for this guard."},
```

- [ ] **Step 8: Run the guards and the formatters**

Run: `gofmt -l . internal/ modules/` then `go test ./internal/combat -count=1` then `go test . -count=1`
Expected: gofmt prints nothing (neither edit is the longest key in its block); both `ok`.

- [ ] **Step 9: Commit**

```bash
git add internal/actions/shadow.go internal/actions/shadow_sense_test.go internal/combat/contest_site_guard_test.go messaging_surface_guard_test.go
git commit -m "feat(actions): one shadow sense roll, and a mob quarry rolls when a shadow starts" -m "The shadow contest moves out of shadowPlayer into ShadowSenseRoll, which shadowMob now runs too (ruling 5). The contest-site row follows it and the one narration key it made stale is retired." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: A sneaking mob moves unannounced (ruling D1)

**Model:** haiku (one parameter, three call sites, code given).

**Files:**
- Modify: `internal/actions/relocate_mob.go:77-111`
- Modify: `internal/actions/relocate_mob_test.go` (call at `:31`, imports, new test)
- Modify: `internal/mobcommands/go.go:140`
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:947`

No guard keys these lines (F25); `RelocateMob`'s sends are on receivers named `from` and `dest`, which the narration walk does not recognise, so no registry key moves (measured).

- [ ] **Step 1: Write the failing test**

In `internal/actions/relocate_mob_test.go`, replace the import block

```go
import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)
```

with

```go
import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)
```

replace `	RelocateMob(m, fromRoom, "north", toRoom)` with `	RelocateMob(m, fromRoom, "north", toRoom, false)`, and append:

```go

// relocateWatchers seeds a player in each room of a RelocateMob move and
// returns the messages each one received, draining them.
func relocateWatchers(t *testing.T, sneaking bool) (fromMsgs, toMsgs []string) {
	t.Helper()
	const from, to, instId = 99421, 99422, 98421
	const fromWatcher, toWatcher = 99431, 99432
	cleanupRooms := rooms.SeedRoomsForTest(map[int]*rooms.Room{
		from: {RoomId: from, Zone: "test", Exits: map[string]exit.RoomExit{"north": {RoomId: to}}},
		to:   {RoomId: to, Zone: "test", Exits: map[string]exit.RoomExit{"south": {RoomId: from}}},
	}, map[string]*rooms.ZoneConfig{})
	defer cleanupRooms()

	fw := users.NewTestUser(fromWatcher, "fromwatch", "Fromwatch", 0)
	fw.Character.RoomId = from
	tw := users.NewTestUser(toWatcher, "towatch", "Towatch", 0)
	tw.Character.RoomId = to
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{fromWatcher: fw, toWatcher: tw})
	defer cleanupUsers()

	m := &mobs.Mob{InstanceId: instId, Character: *characters.New()}
	m.Character.Name = "Prowler"
	m.Character.RoomId = from
	mobs.SetInstanceForTest(instId, m)
	defer mobs.SetInstanceForTest(instId, nil)

	fromRoom, toRoom := rooms.LoadRoom(from), rooms.LoadRoom(to)
	fromRoom.AddPlayer(fromWatcher)
	toRoom.AddPlayer(toWatcher)
	fromRoom.AddMob(instId)
	events.DrainQueuedMessagesForTest(fromWatcher)
	events.DrainQueuedMessagesForTest(toWatcher)

	RelocateMob(m, fromRoom, "north", toRoom, sneaking)

	return events.DrainQueuedMessagesForTest(fromWatcher), events.DrainQueuedMessagesForTest(toWatcher)
}

// Owner ruling D1: a sneaking mob's step is not announced, as a sneaking
// player's never was. The walking control proves the watchers can hear one.
func TestRelocateMob_ASneakingMobMovesUnannounced(t *testing.T) {
	fromMsgs, toMsgs := relocateWatchers(t, false)
	if len(fromMsgs) == 0 || len(toMsgs) == 0 {
		t.Fatalf("control: a walking mob must be announced to both rooms, got from=%q to=%q", fromMsgs, toMsgs)
	}

	fromMsgs, toMsgs = relocateWatchers(t, true)
	if len(fromMsgs) != 0 || len(toMsgs) != 0 {
		t.Errorf("a sneaking mob was announced: from=%q to=%q", fromMsgs, toMsgs)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/actions -run TestRelocateMob -count=1`
Expected: FAIL to build, `too many arguments in call to RelocateMob`.

- [ ] **Step 3: Implement**

In `internal/actions/relocate_mob.go`, replace

```go
// (hooks.handleMobFlee) both end here. It drops aggro the old room held on the
// mob, narrates the exit and the entry (sight-gated, with a sound fallback),
// plays the movement sounds, and pulls an NPC party's idle members after
// their leader.
func RelocateMob(mob *mobs.Mob, from *rooms.Room, exitName string, dest *rooms.Room) {
```

with

```go
// (hooks.handleMobFlee) both end here. It drops aggro the old room held on the
// mob, narrates the exit and the entry (sight-gated, with a sound fallback),
// plays the movement sounds, and pulls an NPC party's idle members after
// their leader.
//
// sneaking is the mover's sneaking state, read by the caller before the move
// (as usercommands.Go reads a player's). A sneaking mob sends no exit line, no
// entry line and nothing to the neighbouring rooms, as a sneaking player
// never has (parity slice 6, owner ruling D1); the movement sounds play for
// both, as they do on the player path.
func RelocateMob(mob *mobs.Mob, from *rooms.Room, exitName string, dest *rooms.Room, sneaking bool) {
```

and replace

```go
	c := configs.GetTextFormatsConfig()

	from.SendTextVisualWithAudio(messaging.CategoryRoomExit,
		fmt.Sprintf(string(c.ExitRoomMessageWrapper),
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> leaves towards the <ansi fg="exit">%s</ansi> exit.`, mob.Character.Name, exitName),
		),
		`You hear footsteps moving away.`)

	dest.SendTextVisualWithAudio(messaging.CategoryRoomEntry,
		fmt.Sprintf(string(c.EnterRoomMessageWrapper),
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> enters from %s.`, mob.Character.Name, enterFrom),
		),
		`You hear footsteps approaching.`)

	dest.SendTextToExits(`You hear someone moving around.`, true, from.GetPlayers(rooms.FindAll)...)
```

with

```go
	c := configs.GetTextFormatsConfig()

	if !sneaking {
		from.SendTextVisualWithAudio(messaging.CategoryRoomExit,
			fmt.Sprintf(string(c.ExitRoomMessageWrapper),
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> leaves towards the <ansi fg="exit">%s</ansi> exit.`, mob.Character.Name, exitName),
			),
			`You hear footsteps moving away.`)

		dest.SendTextVisualWithAudio(messaging.CategoryRoomEntry,
			fmt.Sprintf(string(c.EnterRoomMessageWrapper),
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> enters from %s.`, mob.Character.Name, enterFrom),
			),
			`You hear footsteps approaching.`)

		dest.SendTextToExits(`You hear someone moving around.`, true, from.GetPlayers(rooms.FindAll)...)
	}
```

In `internal/mobcommands/go.go`, replace `		actions.RelocateMob(mob, room, exitName, destRoom)` with `		actions.RelocateMob(mob, room, exitName, destRoom, sneaking)` (the `sneaking` computed just above it, `IsHidden()` or the `sneaking` flag, as the player path does).

In `internal/hooks/NewRound_DoCombat_helpers.go`, replace `		actions.RelocateMob(mob, room, out.ExitName, dest)` with `		actions.RelocateMob(mob, room, out.ExitName, dest, mob.Character.IsHidden())`.

- [ ] **Step 4: Run**

Run: `gofmt -l internal/ && go build ./... && go test ./internal/actions ./internal/mobcommands ./internal/hooks -count=1`
Expected: gofmt prints nothing; build clean; three `ok` lines.

- [ ] **Step 5: Commit**

```bash
git add internal/actions/relocate_mob.go internal/actions/relocate_mob_test.go internal/mobcommands/go.go internal/hooks/NewRound_DoCombat_helpers.go
git commit -m "feat(actions): a sneaking mob moves without announcing itself" -m "RelocateMob takes the mover's sneaking state and skips the exit line, the entry line and the next-room line for a sneaking mob, as the player path does; the movement sounds still play for both (owner ruling D1)." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: One listener moves every shadower and checks it on arrival

**Model:** sonnet (the slice's main integration: a new listener, a registration swap, two deletions, the table test).

**Files:**
- Create: `internal/hooks/RoomChange_ShadowFollow.go`
- Create: `internal/hooks/RoomChange_ShadowFollow_test.go`
- Modify: `internal/hooks/hooks.go:39`
- Delete: `internal/hooks/MobRoomChange_ShadowFollow.go`
- Modify: `internal/usercommands/go.go:394-436` (delete the follow loop)
- Modify: `internal/combat/contest_site_guard_test.go:365` (`legacyLiteralFiles`)

Guards: `legacyLiteralFiles` gains the new listener (spec, "Existing guards"). Deleting the `go.go` loop removes no narration key (its lines were actor-only events) and only helps `move_wrapper_guard_test.go` (F25). The registration swap, the new listener, both deletions and the go.go loop land in ONE commit: with the listener registered and either old site still present, a shadower would be queued twice. After this task `endShadow`, `shadowIsTargetingUser`, `shadowDetectionRoll` and `getShadowTargetUserId` in `usercommands` have no caller; Go allows that, and Task 6 deletes them.

- [ ] **Step 1: Write the failing table test**

Create `internal/hooks/RoomChange_ShadowFollow_test.go`:

```go
package hooks

import (
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Room layout: the quarry leaves shadowRoomA for shadowRoomB by "north", or
// for shadowRoomTemp by the temporary exit "crack" (titled "a narrow crack",
// so the key and the title differ). shadowRoomFar has no exit from A: a move
// there is a teleport. Every character id is unique to this file.
const (
	shadowRoomA    = 71010
	shadowRoomB    = 71011
	shadowRoomFar  = 71012
	shadowRoomTemp = 71013

	shadowQuarryUser = 7101
	shadowSlyUser    = 7102
	shadowQuarryMob  = 71101
	shadowSlyMob     = 71102
	shadowNobody     = 71199 // a target id no character in the scene has
)

// shadowSensedText is the sense line actions.ShadowSenseRoll sends a player
// quarry (unexported there).
const shadowSensedText = "You sense someone following close behind you."

// shadowScene is one fresh world per case: four rooms, a quarry and a
// shadower of each kind, condition 87 seeded.
type shadowScene struct {
	t                   *testing.T
	quarryUser, slyUser *users.UserRecord
	quarryMob, slyMob   *mobs.Mob
	roomB, roomFar      *rooms.Room
}

func newShadowScene(t *testing.T) *shadowScene {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		actions.ShadowingConditionId: {ConditionId: actions.ShadowingConditionId, Name: "Shadowing", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25},
	}))
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		shadowRoomA: {RoomId: shadowRoomA, Zone: "test",
			Exits:     map[string]exit.RoomExit{"north": {RoomId: shadowRoomB}},
			ExitsTemp: map[string]exit.TemporaryRoomExit{"crack": {RoomId: shadowRoomTemp, Title: "a narrow crack"}}},
		shadowRoomB:    {RoomId: shadowRoomB, Zone: "test", Exits: map[string]exit.RoomExit{"south": {RoomId: shadowRoomA}}},
		shadowRoomFar:  {RoomId: shadowRoomFar, Zone: "test", Exits: map[string]exit.RoomExit{}},
		shadowRoomTemp: {RoomId: shadowRoomTemp, Zone: "test", Exits: map[string]exit.RoomExit{}},
	}, map[string]*rooms.ZoneConfig{}))

	s := &shadowScene{
		t:          t,
		quarryUser: users.NewTestUser(shadowQuarryUser, "quarry", "Quarry", 0),
		slyUser:    users.NewTestUser(shadowSlyUser, "sly", "Sly", 0),
		quarryMob:  &mobs.Mob{InstanceId: shadowQuarryMob, Character: *characters.New()},
		slyMob:     &mobs.Mob{InstanceId: shadowSlyMob, Character: *characters.New()},
		roomB:      rooms.LoadRoom(shadowRoomB),
		roomFar:    rooms.LoadRoom(shadowRoomFar),
	}
	s.quarryMob.Character.Name = "Stag"
	s.slyMob.Character.Name = "Lurker"
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		shadowQuarryUser: s.quarryUser, shadowSlyUser: s.slyUser,
	}))
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{}, map[int]*mobs.Mob{
		shadowQuarryMob: s.quarryMob, shadowSlyMob: s.slyMob,
	}))

	// Leftovers from an earlier test must not count.
	events.DrainQueuedUserInputsForTest(shadowSlyUser)
	events.DrainQueuedUserInputsForTest(shadowQuarryUser)
	events.DrainQueuedInputsForTest(shadowSlyMob)
	events.DrainQueuedInputsForTest(shadowQuarryMob)
	events.DrainQueuedMessagesForTest(shadowSlyUser)
	events.DrainQueuedMessagesForTest(shadowQuarryUser)
	return s
}

// shadowKind names a side of the table: "player" or "mob".
type shadowKind string

const (
	shadowPlayer shadowKind = "player"
	shadowMob    shadowKind = "mob"
)

var shadowKinds = []shadowKind{shadowPlayer, shadowMob}

// sly is the shadower of kind k.
func (s *shadowScene) sly(k shadowKind) *characters.Character {
	if k == shadowPlayer {
		return s.slyUser.Character
	}
	return &s.slyMob.Character
}

// place puts the quarry (quarry=true) or the shadower of kind k in room.
func (s *shadowScene) place(k shadowKind, quarry bool, room *rooms.Room) {
	switch {
	case k == shadowPlayer && quarry:
		room.AddPlayer(shadowQuarryUser)
		s.quarryUser.Character.RoomId = room.RoomId
	case k == shadowPlayer:
		room.AddPlayer(shadowSlyUser)
		s.slyUser.Character.RoomId = room.RoomId
	case quarry:
		room.AddMob(shadowQuarryMob)
	default:
		room.AddMob(shadowSlyMob)
	}
}

// shadowTarget sets c's shadow on the quarry of kind k (or on nobody), with
// condition 87 when live, and hides c when hidden.
func (s *shadowScene) shadowTarget(c *characters.Character, k shadowKind, nobody, live, hidden bool) {
	s.t.Helper()
	switch {
	case nobody:
		c.SetMiscData("shadow-target-mob", shadowNobody)
	case k == shadowPlayer:
		c.SetMiscData("shadow-target-user", shadowQuarryUser)
	default:
		c.SetMiscData("shadow-target-mob", shadowQuarryMob)
	}
	if live {
		if err := c.AddCondition(actions.ShadowingConditionId, false); err != nil {
			s.t.Fatalf("add condition 87: %v", err)
		}
	}
	if hidden {
		r := state.TransitionReason{Trigger: "test"}
		c.Awareness.ForceVisible(r)
		_ = c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r)
		c.Awareness.ResolveConcealment(true, r)
		if !c.IsHidden() {
			s.t.Fatal("fixture: the shadower did not become hidden")
		}
	}
}

// moveEvent is the RoomChange of the character of kind k (quarry or shadower).
func moveEvent(k shadowKind, quarry bool, from, to int) events.RoomChange {
	evt := events.RoomChange{FromRoomId: from, ToRoomId: to}
	switch {
	case k == shadowPlayer && quarry:
		evt.UserId = shadowQuarryUser
	case k == shadowPlayer:
		evt.UserId = shadowSlyUser
	case quarry:
		evt.MobInstanceId = shadowQuarryMob
	default:
		evt.MobInstanceId = shadowSlyMob
	}
	return evt
}

// queued drains what the shadower of kind k has queued.
func (s *shadowScene) queued(k shadowKind) []string {
	if k == shadowPlayer {
		return events.DrainQueuedUserInputsForTest(shadowSlyUser)
	}
	return events.DrainQueuedInputsForTest(shadowSlyMob)
}

// shadowEnded reports whether c's shadow is gone: no target, and condition 87
// gone once the turn's prune runs (RemoveCondition only expires it).
func shadowEnded(c *characters.Character) bool {
	c.Conditions.Prune()
	userId, mobInstanceId := actions.ShadowTargetOf(c)
	return userId == 0 && mobInstanceId == 0 && !c.HasCondition(actions.ShadowingConditionId)
}

func shadowCooldown(c *characters.Character) int {
	return c.GetCooldown(skills.Skullduggery.String("shadow"))
}

// The follow pass, for every mover and shadower kind.
func TestRoomChangeShadowFollow_FollowPass(t *testing.T) {
	cases := []struct {
		name                   string
		live, hidden, nobody   bool
		shadowerRoom, toRoomId int
		want                   []string
		cleared                bool
	}{
		{name: "hidden, live, on the mover, in the old room: follows", live: true, hidden: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomB, want: []string{"north"}},
		{name: "not hidden: stays", live: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomB},
		{name: "in another room: stays", live: true, hidden: true, shadowerRoom: shadowRoomFar, toRoomId: shadowRoomB},
		{name: "shadowing someone else: stays", live: true, hidden: true, nobody: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomB},
		{name: "stale (no condition 87): stays and is cleared", hidden: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomB, cleared: true},
		{name: "teleport: no exit, stays", live: true, hidden: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomFar},
		{name: "temp exit: the key is queued, not the title", live: true, hidden: true, shadowerRoom: shadowRoomA, toRoomId: shadowRoomTemp, want: []string{"crack"}},
	}
	for _, mover := range shadowKinds {
		for _, shadower := range shadowKinds {
			for _, tc := range cases {
				t.Run(string(mover)+" mover, "+string(shadower)+" shadower: "+tc.name, func(t *testing.T) {
					s := newShadowScene(t)
					s.place(mover, true, rooms.LoadRoom(tc.toRoomId))
					s.place(shadower, false, rooms.LoadRoom(tc.shadowerRoom))
					c := s.sly(shadower)
					s.shadowTarget(c, mover, tc.nobody, tc.live, tc.hidden)

					RoomChangeShadowFollow(moveEvent(mover, true, shadowRoomA, tc.toRoomId))

					got := s.queued(shadower)
					if strings.Join(got, ";") != strings.Join(tc.want, ";") {
						t.Fatalf("queued %q, want %q", got, tc.want)
					}
					userId, mobInstanceId := actions.ShadowTargetOf(c)
					stillSet := userId != 0 || mobInstanceId != 0
					if tc.cleared && stillSet {
						t.Errorf("stale state kept: ShadowTargetOf = (%d, %d)", userId, mobInstanceId)
					}
					if !tc.cleared && !stillSet {
						t.Error("the shadow target was cleared, but only stale state may be")
					}
					if got := shadowCooldown(c); got != 0 {
						t.Errorf("the follow pass started a %d round cooldown", got)
					}
				})
			}
		}
	}
}

// Arrival not hidden: the shadow ends with the cooldown, and a player
// shadower reads the spotted line.
func TestRoomChangeShadowFollow_ArrivalSpottedEndsTheShadow(t *testing.T) {
	for _, shadower := range shadowKinds {
		for _, quarry := range shadowKinds {
			t.Run(string(shadower)+" shadower, "+string(quarry)+" quarry", func(t *testing.T) {
				s := newShadowScene(t)
				s.place(quarry, true, s.roomB)
				s.place(shadower, false, s.roomB)
				c := s.sly(shadower)
				s.shadowTarget(c, quarry, false, true, false) // live, NOT hidden

				RoomChangeShadowFollow(moveEvent(shadower, false, shadowRoomA, shadowRoomB))

				if !shadowEnded(c) {
					t.Error("the spotted shadow did not end")
				}
				if shadowCooldown(c) <= 0 {
					t.Error("the spotted end started no cooldown")
				}
				spotted := 0
				for _, msg := range events.DrainQueuedMessagesForTest(shadowSlyUser) {
					if strings.Contains(msg, shadowSpottedLine) {
						spotted++
					}
				}
				want := 0
				if shadower == shadowPlayer {
					want = 1
				}
				if spotted != want {
					t.Errorf("spotted line sent %d times, want %d", spotted, want)
				}
			})
		}
	}
}

// Arrival still hidden: the quarry makes the sense roll on every arrival, the
// shadower trains Skullduggery on every roll, and only a player quarry reads
// a line. 200 arrivals, so the contest floor guarantees both outcomes (see
// the shadow sense tests in internal/actions).
func TestRoomChangeShadowFollow_ArrivalHiddenRollsTheSense(t *testing.T) {
	const arrivals = 200
	for _, shadower := range shadowKinds {
		for _, quarry := range shadowKinds {
			t.Run(string(shadower)+" shadower, "+string(quarry)+" quarry", func(t *testing.T) {
				s := newShadowScene(t)
				s.place(quarry, true, s.roomB)
				s.place(shadower, false, s.roomB)
				c := s.sly(shadower)
				s.shadowTarget(c, quarry, false, true, true)
				before := c.GetSkillUseCount(string(skills.Skullduggery))

				for i := 0; i < arrivals; i++ {
					RoomChangeShadowFollow(moveEvent(shadower, false, shadowRoomA, shadowRoomB))
				}

				if got := c.GetSkillUseCount(string(skills.Skullduggery)) - before; got != arrivals {
					t.Errorf("Skullduggery used %d times over %d arrivals, want one per roll on both outcomes", got, arrivals)
				}
				if shadowEnded(c) {
					t.Fatal("a hidden arrival ended the shadow")
				}
				sensed := 0
				for _, msg := range events.DrainQueuedMessagesForTest(shadowQuarryUser) {
					if strings.Contains(msg, shadowSensedText) {
						sensed++
					}
				}
				if quarry == shadowPlayer && (sensed == 0 || sensed == arrivals) {
					t.Errorf("player quarry sensed %d of %d arrivals: both outcomes must occur", sensed, arrivals)
				}
				if quarry == shadowMob && sensed != 0 {
					t.Errorf("a mob quarry's roll sent %d lines to a player", sensed)
				}
				if got := len(events.DrainQueuedMessagesForTest(shadowSlyUser)); got != 0 {
					t.Errorf("the shadower read %d lines on a hidden arrival", got)
				}
			})
		}
	}
}

// D4: a shadower that arrives in a room its quarry has already left gets no
// check, and a stale shadower (no condition 87) arriving gets none either.
func TestRoomChangeShadowFollow_ArrivalNeedsTheQuarryAndALiveShadow(t *testing.T) {
	for _, shadower := range shadowKinds {
		t.Run(string(shadower)+" shadower, quarry gone", func(t *testing.T) {
			s := newShadowScene(t)
			s.place(shadowPlayer, true, s.roomFar)
			s.place(shadower, false, s.roomB)
			c := s.sly(shadower)
			s.shadowTarget(c, shadowPlayer, false, true, false) // not hidden: would be spotted

			RoomChangeShadowFollow(moveEvent(shadower, false, shadowRoomA, shadowRoomB))

			if shadowEnded(c) || shadowCooldown(c) != 0 {
				t.Error("an arrival without the quarry ended the shadow")
			}
		})
		t.Run(string(shadower)+" shadower, stale", func(t *testing.T) {
			s := newShadowScene(t)
			s.place(shadowPlayer, true, s.roomB)
			s.place(shadower, false, s.roomB)
			c := s.sly(shadower)
			s.shadowTarget(c, shadowPlayer, false, false, false)

			RoomChangeShadowFollow(moveEvent(shadower, false, shadowRoomA, shadowRoomB))

			if shadowCooldown(c) != 0 {
				t.Error("a stale arrival started the spotted cooldown")
			}
			if userId, _ := actions.ShadowTargetOf(c); userId != shadowQuarryUser {
				t.Error("the arrival pass touched a stale shadow; only the follow pass clears one")
			}
		})
	}
}

// The listener is registered in place of the mob-only one.
func TestRoomChangeShadowFollowIsRegistered(t *testing.T) {
	src, err := os.ReadFile("hooks.go")
	if err != nil {
		t.Fatalf("reading hooks.go: %v", err)
	}
	if !strings.Contains(string(src), "events.RegisterListener(events.RoomChange{}, RoomChangeShadowFollow)") {
		t.Error("hooks.go no longer registers RoomChangeShadowFollow on RoomChange")
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/hooks -run 'TestRoomChangeShadowFollow' -count=1`
Expected: FAIL to build, `undefined: RoomChangeShadowFollow` and `undefined: shadowSpottedLine`.

- [ ] **Step 3: Write the listener**

Create `internal/hooks/RoomChange_ShadowFollow.go`:

```go
package hooks

// RoomChange_ShadowFollow.go: parity slice 6. The one place a shadower follows
// its quarry and the one place the shadow is checked on arrival, for players
// and mobs alike on both sides. It replaced a loop in usercommands/go.go
// (player movers, player shadowers only) and MobRoomChange_ShadowFollow.go
// (mob movers, player shadowers only); a mob shadower was never moved.

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// shadowSpottedLine is what a shadower reads when it arrives in its quarry's
// room no longer hidden.
const shadowSpottedLine = "You've been spotted -- your shadow ends."

// RoomChangeShadowFollow runs two passes on every RoomChange.
//
// Follow: when the mover left through a named exit (shadowExitTo), every
// player and mob still in the old room whose shadow names the mover queues
// the same exit, if it is still hidden. A shadower whose target is set but
// whose Shadowing condition is gone is stale: its state is cleared
// (actions.ClearShadow) and it stays put. A teleport has no named exit and
// moves no one.
//
// Arrival: when the mover is itself shadowing someone now in the room it
// entered, the shadow is checked. Not hidden ends it (actions.EndShadow with
// shadowSpottedLine and the cooldown); still hidden, the quarry makes the
// sense roll (actions.ShadowSenseRoll) in that room's light.
//
// Listeners run after the moving command returns (events.ProcessEvents), so
// by now the mover's entry detection has run and IsHidden reflects any
// reveal. RoomChange.Unseen was captured before detection, which is why it is
// never read here.
func RoomChangeShadowFollow(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.RoomChange)
	if !ok {
		return events.Continue
	}
	if evt.UserId == 0 && evt.MobInstanceId == 0 {
		return events.Continue
	}
	shadowFollowPass(evt)
	shadowArrivalPass(evt)
	return events.Continue
}

// shadowFollower is one character in the mover's old room that may be
// shadowing it, with the queue its follow command goes on. *users.UserRecord
// and *mobs.Mob share Command's signature.
type shadowFollower struct {
	char *characters.Character
	cmd  interface{ Command(string, ...float64) }
}

func shadowFollowPass(evt events.RoomChange) {
	fromRoom := rooms.LoadRoom(evt.FromRoomId)
	if fromRoom == nil {
		return
	}
	exitName := shadowExitTo(fromRoom, evt.ToRoomId)
	if exitName == "" {
		return
	}

	var followers []shadowFollower
	for _, userId := range fromRoom.GetPlayers(rooms.FindAll) {
		if userId == evt.UserId {
			continue
		}
		if u := users.GetByUserId(userId); u != nil {
			followers = append(followers, shadowFollower{char: u.Character, cmd: u})
		}
	}
	for _, instId := range fromRoom.GetMobs(rooms.FindAll) {
		if instId == evt.MobInstanceId {
			continue
		}
		if m := mobs.GetInstance(instId); m != nil {
			followers = append(followers, shadowFollower{char: &m.Character, cmd: m})
		}
	}

	for _, f := range followers {
		userId, mobInstanceId, live := shadowOf(f.char)
		if !shadowNamesMover(evt, userId, mobInstanceId) {
			continue
		}
		if !live {
			actions.ClearShadow(f.char)
			continue
		}
		if !f.char.IsHidden() {
			continue
		}
		f.cmd.Command(exitName)
	}
}

func shadowArrivalPass(evt events.RoomChange) {
	dest := rooms.LoadRoom(evt.ToRoomId)
	if dest == nil {
		return
	}

	var shadower actions.Actor
	if evt.UserId > 0 {
		u := users.GetByUserId(evt.UserId)
		if u == nil {
			return
		}
		shadower = actions.NewUserActorInRoom(u, dest)
	} else {
		m := mobs.GetInstance(evt.MobInstanceId)
		if m == nil {
			return
		}
		shadower = actions.NewMobActorInRoom(m, dest)
	}

	userId, mobInstanceId, live := shadowOf(shadower.GetCharacter())
	if !live {
		return
	}

	var target actions.Actor
	switch {
	case userId > 0:
		u := users.GetByUserId(userId)
		if u == nil || u.Character.RoomId != evt.ToRoomId {
			return
		}
		target = actions.NewUserActorInRoom(u, dest)
	case mobInstanceId > 0:
		m := mobs.GetInstance(mobInstanceId)
		if m == nil || m.Character.RoomId != evt.ToRoomId {
			return
		}
		target = actions.NewMobActorInRoom(m, dest)
	default:
		return
	}

	if !shadower.GetCharacter().IsHidden() {
		actions.EndShadow(shadower, shadowSpottedLine)
		return
	}
	actions.ShadowSenseRoll(shadower, target, dest)
}

// shadowOf is c's shadow target and whether the shadow is live (the
// Shadowing condition is held).
func shadowOf(c *characters.Character) (userId, mobInstanceId int, live bool) {
	userId, mobInstanceId = actions.ShadowTargetOf(c)
	return userId, mobInstanceId, c.HasCondition(actions.ShadowingConditionId)
}

// shadowNamesMover reports whether a shadow target is the RoomChange's mover.
func shadowNamesMover(evt events.RoomChange, userId, mobInstanceId int) bool {
	if evt.UserId > 0 {
		return userId == evt.UserId
	}
	return mobInstanceId > 0 && mobInstanceId == evt.MobInstanceId
}

// shadowExitTo is the command word that leads from room to toRoomId, or ""
// when no exit does (a teleport). It returns the exit's map KEY, which is what
// `go` matches, looking in the permanent exits, then the temporary exits,
// then the active mutators' exits. Room.FindExitTo is not used: for a
// temporary exit it returns the Title, which need not be the key.
func shadowExitTo(room *rooms.Room, toRoomId int) string {
	for exitName, exitInfo := range room.Exits {
		if exitInfo.RoomId == toRoomId {
			return exitName
		}
	}
	for exitName, exitInfo := range room.ExitsTemp {
		if exitInfo.RoomId == toRoomId {
			return exitName
		}
	}
	for mut := range room.ActiveMutators {
		for exitName, exitInfo := range mut.GetSpec().Exits {
			if exitInfo.RoomId == toRoomId {
				return exitName
			}
		}
	}
	return ""
}
```

- [ ] **Step 4: Swap the registration and delete the old hook**

In `internal/hooks/hooks.go`, replace `	events.RegisterListener(events.RoomChange{}, MobRoomChangeShadowFollow)` with `	events.RegisterListener(events.RoomChange{}, RoomChangeShadowFollow)`.

Run: `git rm internal/hooks/MobRoomChange_ShadowFollow.go`

- [ ] **Step 5: Delete the player-mover follow loop**

In `internal/usercommands/go.go`, delete this block and the blank line after it (`:394-436`), so the charmed-mob loop's closing `}` is followed by one blank line and then `// Hidden detection on room entry, both directions: the sneaking`:

```go
			// Shadow follow -- check if any hidden player in the OLD room was
			// shadowing the mover (user). Auto-move them to the destination.
			for _, pId := range room.GetPlayers(rooms.FindAll) {
				if pId == user.UserId {
					continue
				}
				shadowP := users.GetByUserId(pId)
				if shadowP == nil {
					continue
				}
				if !shadowP.Character.IsHidden() {
					continue
				}
				if !shadowIsTargetingUser(shadowP, user.UserId) {
					continue
				}
				// Condition-absent guard: misc data set but condition 87 gone means the
				// shadow expired or was cancelled out-of-band. Clear stale state
				// and skip the auto-follow so a dead/logged-off target can't drag
				// the player to an unexpected room.
				if !shadowP.Character.HasCondition(87) {
					shadowP.Character.SetMiscData("shadow-target-user", nil)
					shadowP.Character.SetMiscData("shadow-target-mob", nil)
					continue
				}
				// Shadower is in the old room and tracking the mover -- follow.
				shadowP.Command(rest)

				// After the move attempt, check if the shadower is still hidden.
				// The room-entry detection in go.go runs for the shadower's move,
				// so if they were spotted their hidden condition will already be gone.
				if !shadowP.Character.IsHidden() {
					endShadow(shadowP, "You've been spotted -- your shadow ends.")
					continue
				}

				// Target-specific detection roll: does the mover sense pursuit?
				if shadowDetectionRoll(shadowP, user, destRoom) {
					user.SendText(messaging.CategorySystem,
						"You sense someone following close behind you.")
				}
			}

```

- [ ] **Step 6: Add the listener to `legacyLiteralFiles`**

In `internal/combat/contest_site_guard_test.go`, replace

```go
	"internal/hooks/Position_GrappleTick.go",
	"internal/actions/steal.go",
```

with

```go
	"internal/hooks/Position_GrappleTick.go",
	"internal/hooks/RoomChange_ShadowFollow.go",
	"internal/actions/steal.go",
```

- [ ] **Step 7: Run**

Run: `gofmt -l internal/ modules/ . && go build ./... && go vet ./internal/hooks ./internal/usercommands && go test ./internal/hooks ./internal/usercommands ./internal/combat -count=1 && go test . -count=1`
Expected: gofmt and vet print nothing; four `ok` lines. `go test ./internal/hooks -run TestRoomChangeShadowFollow -v -count=1` shows 45 `--- PASS` lines (28 follow cases, 4 spotted, 4 sense, 4 arrival-needs, the registration test, and 4 parents).

- [ ] **Step 8: Commit**

```bash
git add internal/hooks/RoomChange_ShadowFollow.go internal/hooks/RoomChange_ShadowFollow_test.go internal/hooks/hooks.go internal/usercommands/go.go internal/combat/contest_site_guard_test.go
git commit -m "feat(hooks): one RoomChange listener moves every shadower and checks it on arrival" -m "Replaces the go.go follow loop (player movers, player shadowers) and MobRoomChange_ShadowFollow (mob movers, player shadowers). Any named-exit move now queues the same exit on every hidden, live shadower of the mover, player or mob; stale state is cleared; the spotted end and the sense roll run on the shadower's own arrival. The exit is the map key from Exits, ExitsTemp, then mutator exits (owner ruling D3)." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(`git rm` in Step 4 already staged the deletion; `git status` must show `deleted: internal/hooks/MobRoomChange_ShadowFollow.go` in the commit.)

---

### Task 6: `shadow stop` through the shared bodies; the forked helpers go

**Model:** haiku (compiler-guided deletion, code given).

**Files:**
- Modify: `internal/usercommands/skill.skullduggery.shadow.go` (whole file)
- Create: `internal/usercommands/shadow_stop_test.go`
- Modify: `internal/combat/contest_site_guard_test.go:60-75` (the U6b family block)

Guard this task moves (F22): deleting `shadowDetectionRoll` makes its `contestSiteOwners` row stale, and that key is the longest in its block, so `gofmt` realigns all its rows; Step 5 gives the whole block. `lookup_viewer_guard_test.go:78` (`Shadow` `{viewer: 1, plain: 1}`) is unchanged: the stop branch does no lookup (measured).

- [ ] **Step 1: Write the tests**

Create `internal/usercommands/shadow_stop_test.go`:

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// shadowStopUser is a player with Skullduggery 3 in a bare room, and the room.
func shadowStopUser(t *testing.T, userId int) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		actions.ShadowingConditionId: {ConditionId: actions.ShadowingConditionId, Name: "Shadowing", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 25},
	}))
	u := users.NewTestUser(userId, "stopper", "Stopper", 0)
	u.Character.Skills = map[string]int{string(skills.Skullduggery): 3}
	events.DrainQueuedMessagesForTest(userId)
	return u, &rooms.Room{RoomId: 99401}
}

// `shadow stop` ends a live shadow through actions.EndShadow: target and
// condition gone, cooldown running, the stop line sent.
func TestShadowStop_EndsTheShadow(t *testing.T) {
	const userId = 7401
	u, room := shadowStopUser(t, userId)
	u.Character.SetMiscData("shadow-target-mob", 88401)
	if err := u.Character.AddCondition(actions.ShadowingConditionId, false); err != nil {
		t.Fatalf("add condition 87: %v", err)
	}

	if handled, err := Shadow("stop", u, room, 0); !handled || err != nil {
		t.Fatalf("Shadow(stop) = %v, %v", handled, err)
	}

	if tu, tm := actions.ShadowTargetOf(u.Character); tu != 0 || tm != 0 {
		t.Errorf("ShadowTargetOf after stop = (%d, %d), want (0, 0)", tu, tm)
	}
	u.Character.Conditions.Prune()
	if u.Character.HasCondition(actions.ShadowingConditionId) {
		t.Error("condition 87 survived shadow stop")
	}
	if u.Character.GetCooldown(skills.Skullduggery.String("shadow")) <= 0 {
		t.Error("shadow stop started no cooldown")
	}
	msgs := events.DrainQueuedMessagesForTest(userId)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "You stop shadowing your target.") {
		t.Errorf("messages %q, want the stop line", msgs)
	}
}

// With no shadow, `shadow stop` says so and starts nothing.
func TestShadowStop_WithNoShadowSaysSo(t *testing.T) {
	const userId = 7402
	u, room := shadowStopUser(t, userId)

	if handled, err := Shadow("stop", u, room, 0); !handled || err != nil {
		t.Fatalf("Shadow(stop) = %v, %v", handled, err)
	}

	if u.Character.GetCooldown(skills.Skullduggery.String("shadow")) != 0 {
		t.Error("stopping no shadow started a cooldown")
	}
	msgs := events.DrainQueuedMessagesForTest(userId)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "You aren't shadowing anyone.") {
		t.Errorf("messages %q, want the not-shadowing line", msgs)
	}
}
```

- [ ] **Step 2: Run them (they pin behaviour before the refactor)**

Run: `go test ./internal/usercommands -run TestShadowStop -count=1`
Expected: PASS. `shadow stop` behaves the same before and after this task; these tests pin it so the refactor below cannot change it.

- [ ] **Step 3: Rewrite the command file**

Replace the whole of `internal/usercommands/skill.skullduggery.shadow.go` with:

```go
package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

/*
Skullduggery Skill
Level 3 - Shadow: follow a target between rooms while remaining hidden.
When the target moves through an exit, the shadower moves with them
(hooks.RoomChangeShadowFollow). On each arrival the target may sense the
shadower (actions.ShadowSenseRoll). The shadow ends if the shadower arrives
no longer hidden, or on "shadow stop" (actions.EndShadow).
*/
func Shadow(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	skillLevel := user.Character.GetSkillLevel(skills.Skullduggery)

	// Requires skullduggery rank 3
	if skillLevel < 1 {
		return false, nil
	}
	if skillLevel < 3 {
		user.SendText(messaging.CategorySystem, "You aren't advanced enough at skullduggery for that.")
		return true, nil
	}

	rest = strings.TrimSpace(rest)

	// "shadow stop" cancels an active shadow
	if strings.ToLower(rest) == "stop" {
		if targetUserId, targetMobId := actions.ShadowTargetOf(user.Character); targetUserId != 0 || targetMobId != 0 {
			actions.EndShadow(actions.NewUserActorInRoom(user, room), "You stop shadowing your target.")
		} else {
			user.SendText(messaging.CategorySystem, "You aren't shadowing anyone.")
		}
		return true, nil
	}

	if rest == "" {
		user.SendText(messaging.CategorySystem, "Shadow whom?")
		return true, nil
	}

	// Resolve target in the current room, excluding the player themselves.
	target, err := actions.ResolveTargetActor(room, strings.ToLower(rest), actions.ResolveTargetOptions{
		ExcludeUserId: user.UserId,
		Viewer:        user.Character,
	})
	if err != nil {
		// Check whether the name matched the player themselves.
		if pId, _ := room.FindByName(strings.ToLower(rest)); pId == user.UserId {
			user.SendText(messaging.CategorySystem, "You can't shadow yourself.")
			return true, nil
		}
		user.SendText(messaging.CategorySystem, "Shadow whom?")
		return true, nil
	}

	opts := actions.ShadowOptions{}
	if target.IsPlayer() {
		opts.TargetUserId = target.GetUserId()
	} else {
		opts.TargetMobInstanceId = target.GetMobInstanceId()
	}

	actor := &actions.UserActor{User: user, Room: room}
	result := actions.Shadow(actor, opts)

	if result.OnCooldown {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			"You need to wait %s before shadowing again.",
			result.Reason))
	}

	return true, nil
}
```

- [ ] **Step 4: See the contest guard fail as predicted**

Run: `go build ./... && go test ./internal/combat -run TestEveryContestSiteIsOwned -count=1`
Expected: build clean; FAIL with exactly `stale allowlist entry internal/usercommands/skill.skullduggery.shadow.go:shadowDetectionRoll`.

- [ ] **Step 5: Delete the row; the block realigns**

In `internal/combat/contest_site_guard_test.go`, replace the sixteen rows under `// strings come from the plan, not from the implementer).` (from the `flee.go:ResolveFleeBlockers` row through the `move.go:newcomerSpots` row) with these fifteen, which is exactly what `gofmt -w` produces once the longest key is gone:

```go
	"internal/combat/flee.go:ResolveFleeBlockers":               "U6b task 12 (both the mob-blocker and player-blocker loops)",
	"internal/combat/grapple.go:AttemptGrapple":                 "U6b task 13",
	"internal/combat/submission.go:RollSubmissionAttempt":       "U6b task 13",
	"internal/hooks/Position_GrappleTick.go:processGrapplePair": "U6b task 14 (grapple drift — RunContest passed to processGrapplePairWithContest)",
	"internal/actions/plant.go:plantOnMob":                      "U6b task 15",
	"internal/actions/plant.go:plantOnPlayer":                   "U6b task 15 (two sites: plant roll + observer pass)",
	"internal/actions/plant.go:plantInContainer":                "U6b task 15",
	"internal/actions/steal.go:stealFromMob":                    "U6b task 15",
	"internal/actions/steal.go:stealFromPlayer":                 "U6b task 15 (two sites: steal roll + observer pass)",
	"internal/actions/steal.go:stealObserverPass":               "U6b task 15 (the container-theft observer pass; household bauble theft shares it)",
	"internal/actions/stolen_bauble.go:stolenRecognitionRoll":   "baubles Phase 6c (an owner recognising its stolen bauble: stealVictimScore against the steal score)",
	"internal/actions/sneak.go:Sneak":                           "U6b task 16 (two sites)",
	"internal/actions/shadow.go:ShadowSenseRoll":                "U6b task 16 (parity slice 6 lifted the shadow sense contest out of shadowPlayer into this one body; it runs when a shadow starts and on each arrival)",
	"internal/actions/move.go:sneakerSpotted":                   "U6b task 16 (hidden detection on room entry: a sneaking mover against the room's players and mobs; moved from usercommands/go.go by movement parity 4b)",
	"internal/actions/move.go:newcomerSpots":                    "U6b task 16 (hidden detection on room entry: the newcomer against hidden players and mobs; moved from usercommands/go.go by movement parity 4b)",
```

(The two em dashes are the existing owner strings, unchanged.) Then run `gofmt -w internal/combat/contest_site_guard_test.go` and confirm `git diff --stat` shows only this block changed in that file.

- [ ] **Step 6: Run**

Run: `gofmt -l internal/ modules/ . && go vet ./internal/usercommands && go test ./internal/usercommands ./internal/combat -count=1 && go test . -count=1`
Expected: gofmt and vet print nothing; three `ok` lines (the two `TestShadowStop_*` still pass).

- [ ] **Step 7: Commit**

```bash
git add internal/usercommands/skill.skullduggery.shadow.go internal/usercommands/shadow_stop_test.go internal/combat/contest_site_guard_test.go
git commit -m "refactor(usercommands): shadow stop through actions.EndShadow; delete the forked shadow helpers" -m "endShadow, shadowIsTargetingUser, shadowDetectionRoll and the dead getShadowTargetUserId are gone; their contest-site row goes with them." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: The death and logoff cleanups use `ShadowTargetOf` and `ClearShadow` (ruling D2)

**Model:** haiku (two small files, code given).

**Files:**
- Modify: `internal/hooks/MobDeath_TrackingCleanup.go` (whole file)
- Modify: `internal/hooks/PlayerDespawn_TrackingCleanup.go` (whole file)
- Create: `internal/hooks/TrackingCleanup_Shadow_test.go`

No guard keys these files. The cleanup's player-visible behaviour is unchanged: no cooldown, no line of its own (the condition's end line at the next prune, as today).

- [ ] **Step 1: Write the test (it pins today's behaviour)**

Create `internal/hooks/TrackingCleanup_Shadow_test.go` (it reuses Task 5's `newShadowScene`, same package):

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
)

// A quarry's death or logoff drops every shadow on it through
// actions.ClearShadow: target and condition 87 gone, no cooldown, no line
// (owner ruling D2). A shadow on someone else is untouched.
func TestTrackingCleanup_DropsShadowsOnTheDepartedQuarry(t *testing.T) {
	for _, shadower := range shadowKinds {
		t.Run(string(shadower)+" shadower, mob quarry dies", func(t *testing.T) {
			s := newShadowScene(t)
			c := s.sly(shadower)
			s.shadowTarget(c, shadowMob, false, true, true)
			other := s.quarryUser.Character // shadows nobody in this scene
			s.shadowTarget(other, shadowMob, true, true, false)

			MobDeathTrackingCleanup(events.MobDeath{InstanceId: shadowQuarryMob, CharacterName: "Stag"})

			if !shadowEnded(c) {
				t.Error("the shadow on the dead mob survived")
			}
			if shadowCooldown(c) != 0 {
				t.Error("the death cleanup started a cooldown")
			}
			if _, mobInstanceId := actions.ShadowTargetOf(other); mobInstanceId != shadowNobody {
				t.Error("a shadow on another mob was cleared")
			}
			if got := len(events.DrainQueuedMessagesForTest(shadowSlyUser)); got != 0 {
				t.Errorf("the death cleanup sent the shadower %d lines", got)
			}
		})
		t.Run(string(shadower)+" shadower, player quarry logs off", func(t *testing.T) {
			s := newShadowScene(t)
			c := s.sly(shadower)
			s.shadowTarget(c, shadowPlayer, false, true, true)

			PlayerDespawnTrackingCleanup(events.PlayerDespawn{UserId: shadowQuarryUser, CharacterName: "Quarry"})

			if !shadowEnded(c) {
				t.Error("the shadow on the departed player survived")
			}
			if shadowCooldown(c) != 0 {
				t.Error("the logoff cleanup started a cooldown")
			}
			if got := len(events.DrainQueuedMessagesForTest(shadowSlyUser)); got != 0 {
				t.Errorf("the logoff cleanup sent the shadower %d lines", got)
			}
		})
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/hooks -run TestTrackingCleanup_DropsShadows -count=1`
Expected: PASS (4 subtests). This is a refactor: the test pins the behaviour the rewrite must keep.

- [ ] **Step 3: Rewrite both cleanups**

Replace the whole of `internal/hooks/MobDeath_TrackingCleanup.go` with:

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const activeTrackingCondition = 86

// MobDeathTrackingCleanup clears tracking/shadow state on any character
// (player or mob) that was tracking or shadowing the now-dead mob. Pairs
// with PlayerDespawn_TrackingCleanup for the symmetric logoff path.
//
// State cleared per pointing character:
//   - tracking-mob misc data (string match on CharacterName)
//   - tracking-display-count misc data (cleared alongside tracking-mob)
//   - condition 86 (Active Tracking) — only if tracking-mob pointed at this mob
//   - the shadow (actions.ClearShadow: target and Shadowing condition, no
//     cooldown, no message), only if actions.ShadowTargetOf names this mob
func MobDeathTrackingCleanup(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.MobDeath)
	if !ok {
		return events.Continue
	}

	// CharacterName is populated at death time on the event payload;
	// no need to fetch the instance or template.
	dyingName := evt.CharacterName
	dyingInstanceId := evt.InstanceId

	clearPointers := func(c *characters.Character) {
		// Tracking by name.
		if dyingName != "" {
			if v := c.GetMiscData("tracking-mob"); v != nil {
				if s, ok := v.(string); ok && s == dyingName {
					c.SetMiscData("tracking-mob", nil)
					c.SetMiscData("tracking-display-count", nil)
					c.RemoveCondition(activeTrackingCondition)
				}
			}
		}
		// Shadow by InstanceId.
		if _, mobInstanceId := actions.ShadowTargetOf(c); mobInstanceId != 0 && mobInstanceId == dyingInstanceId {
			actions.ClearShadow(c)
		}
	}

	// Walk all online users.
	for _, u := range users.GetAllActiveUsers() {
		if u == nil {
			continue
		}
		clearPointers(u.Character)
	}

	// Walk all active mob instances.
	for _, instId := range mobs.GetAllMobInstanceIds() {
		if instId == dyingInstanceId {
			continue
		}
		m := mobs.GetInstance(instId)
		if m == nil {
			continue
		}
		clearPointers(&m.Character)
	}

	return events.Continue
}
```

(The `condition 86` bullet keeps its existing em dash; the rewritten shadow bullet has none.)

Replace the whole of `internal/hooks/PlayerDespawn_TrackingCleanup.go` with:

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// PlayerDespawnTrackingCleanup clears tracking/shadow state on any
// character (player or mob) that was tracking or shadowing the now-
// despawning player. Pairs with MobDeath_TrackingCleanup for the
// symmetric mob-death path.
//
// State cleared per pointing character:
//   - tracking-user misc (string match on CharacterName)
//   - tracking-display-count misc (cleared alongside tracking-user)
//   - condition 86 (Active Tracking) — only if tracking-user state was on this char
//   - the shadow (actions.ClearShadow: target and Shadowing condition, no
//     cooldown, no message), only if actions.ShadowTargetOf names this player
func PlayerDespawnTrackingCleanup(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.PlayerDespawn)
	if !ok {
		return events.Continue
	}

	leavingName := evt.CharacterName
	leavingUserId := evt.UserId

	clearPointers := func(c *characters.Character) {
		// Tracking by name.
		if leavingName != "" {
			if v := c.GetMiscData("tracking-user"); v != nil {
				if s, ok := v.(string); ok && s == leavingName {
					c.SetMiscData("tracking-user", nil)
					c.SetMiscData("tracking-display-count", nil)
					c.RemoveCondition(activeTrackingCondition)
				}
			}
		}
		// Shadow by UserId.
		if userId, _ := actions.ShadowTargetOf(c); userId != 0 && userId == leavingUserId {
			actions.ClearShadow(c)
		}
	}

	// Walk all other online users.
	for _, u := range users.GetAllActiveUsers() {
		if u == nil || u.UserId == leavingUserId {
			continue
		}
		clearPointers(u.Character)
	}

	// Walk all active mob instances.
	for _, instId := range mobs.GetAllMobInstanceIds() {
		m := mobs.GetInstance(instId)
		if m == nil {
			continue
		}
		clearPointers(&m.Character)
	}

	return events.Continue
}
```

The `!= 0` checks keep an event that carries a zero id from matching every character that shadows no one.

- [ ] **Step 4: Run**

Run: `gofmt -l internal/ && go build ./... && go test ./internal/hooks -count=1`
Expected: gofmt prints nothing; build clean (the `shadowingCondition` const is gone and nothing names it); `ok  	github.com/GoMudEngine/GoMud/internal/hooks`.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/MobDeath_TrackingCleanup.go internal/hooks/PlayerDespawn_TrackingCleanup.go internal/hooks/TrackingCleanup_Shadow_test.go
git commit -m "refactor(hooks): death and logoff drop a shadow through actions.ClearShadow" -m "Owner ruling D2: the cleanups keep their silent drop (no cooldown, no line) and read the target through actions.ShadowTargetOf. The shadowingCondition const goes with its last users." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: The shadow follow guard

**Model:** sonnet (the probes need judgment: each must compile and fail for the named reason).

**Files:**
- Create: `shadow_follow_guard_test.go` (repo root, package `main`)

- [ ] **Step 1: Write the guard**

Create `shadow_follow_guard_test.go`:

```go
package main

import (
	"bytes"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Parity slice 6 made one listener, hooks.RoomChangeShadowFollow, the only
// place a shadower follows its quarry, and put the shadow's state behind
// shared bodies in internal/actions (ShadowTargetOf, ClearShadow, EndShadow,
// ShadowSenseRoll, ShadowingConditionId). Before that there were two follow
// sites, a loop in usercommands/go.go and a mob-only hook, each reading the
// misc-data keys and condition 87 itself, each with its own copy of the end
// logic, and neither moved a mob shadower. These tests fail if that forks
// again.

// shadowGuardCode is path's Go source with every comment removed (parsed
// without ParseComments and printed back), so a comment naming a key, a
// condition id or a deleted function neither fails nor satisfies a check.
func shadowGuardCode(path string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, f); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// shadowGuardWalk calls fn with the comment-free code of every production Go
// file under internal/ and modules/, and fails the test when it parsed fewer
// than 50 (a walk that sees nothing proves nothing).
func shadowGuardWalk(t *testing.T, fn func(rel, code string)) {
	t.Helper()
	parsed := 0
	for _, root := range []string{"internal", "modules"} {
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
			code, perr := shadowGuardCode(path)
			if perr != nil {
				// A syntax error is the compiler's to report, and another
				// root test may create and remove a scratch file mid-walk.
				return nil
			}
			parsed++
			fn(filepath.ToSlash(path), code)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if parsed < 50 {
		t.Fatalf("parsed only %d files; the walk is not seeing the tree, so a pass proves nothing", parsed)
	}
}

// The two misc-data keys are read and written only in internal/actions. On
// master before this slice five files outside it named them.
func TestShadowKeysLiveInActions(t *testing.T) {
	pattern := regexp.MustCompile(`shadow-target-(user|mob)`)
	seen := false
	shadowGuardWalk(t, func(rel, code string) {
		if !pattern.MatchString(code) {
			return
		}
		if strings.HasPrefix(rel, "internal/actions/") {
			seen = true
			return
		}
		t.Errorf("%s names a shadow misc-data key; use actions.ShadowTargetOf, ClearShadow or EndShadow", rel)
	})
	if !seen {
		t.Fatal("no shadow key found in internal/actions: the pattern cannot match, so this guard proves nothing")
	}
}

// Condition 87 is named by literal only in internal/actions/shadow.go, as
// actions.ShadowingConditionId. On master before this slice go.go, the
// skullduggery command and a hooks const each carried the literal.
func TestShadowConditionIsNamedOnce(t *testing.T) {
	pattern := regexp.MustCompile(`Condition\(87\b|=\s*87\b`)
	seen := false
	shadowGuardWalk(t, func(rel, code string) {
		if !pattern.MatchString(code) {
			return
		}
		if rel == "internal/actions/shadow.go" {
			seen = true
			return
		}
		t.Errorf("%s names condition 87 by literal; use actions.ShadowingConditionId", rel)
	})
	if !seen {
		t.Fatal("no condition 87 literal found in internal/actions/shadow.go: the pattern cannot match")
	}
}

// A shadow follow is ShadowTargetOf and a queued Command in one file. Only the
// listener does that, with one of each: one follow dispatch for every kind of
// shadower. On master before this slice go.go and the mob hook each followed.
func TestOneShadowFollowSite(t *testing.T) {
	const site = "internal/hooks/RoomChange_ShadowFollow.go"
	target := regexp.MustCompile(`ShadowTargetOf\(`)
	command := regexp.MustCompile(`\.Command\(`)
	seen := false
	shadowGuardWalk(t, func(rel, code string) {
		if !target.MatchString(code) || !command.MatchString(code) {
			return
		}
		if rel != site {
			t.Errorf("%s reads a shadow target and queues a command; shadow following lives in %s alone", rel, site)
			return
		}
		seen = true
		if n := len(target.FindAllString(code, -1)); n != 1 {
			t.Errorf("%s calls ShadowTargetOf %d times, want 1", site, n)
		}
		if n := len(command.FindAllString(code, -1)); n != 1 {
			t.Errorf("%s queues %d commands, want 1 follow dispatch for every kind of shadower", site, n)
		}
	})
	if !seen {
		t.Fatalf("%s does not read a shadow target and queue a command: the listener is gone or the patterns cannot match", site)
	}
}

// The forked follow and end logic this slice deleted stays deleted.
func TestDeletedShadowForksStayDeleted(t *testing.T) {
	pattern := regexp.MustCompile(`\bshadowIsTargetingUser\b|\bshadowDetectionRoll\b|\binlineShadowEnd\b|\bendShadow\(|\bMobRoomChangeShadowFollow\b|\bgetShadowTargetUserId\b`)
	shadowGuardWalk(t, func(rel, code string) {
		if loc := pattern.FindStringIndex(code); loc != nil {
			t.Errorf("%s names %q, deleted by parity slice 6; use the shared bodies in internal/actions", rel, code[loc[0]:loc[1]])
		}
	})
	// Proof the pattern can match: its own alternatives, as they were spelled
	// on master before this slice.
	for _, probe := range []string{"shadowIsTargetingUser(u, 1)", "shadowDetectionRoll(a, b, r)", "inlineShadowEnd(u, s)", "endShadow(u, s)", "MobRoomChangeShadowFollow)", "getShadowTargetUserId(u)"} {
		if !pattern.MatchString(probe) {
			t.Fatalf("the deleted-name pattern does not match %q: it cannot see what it guards", probe)
		}
	}
	if pattern.MatchString("actions.EndShadow(actor, reason)") {
		t.Fatal("the deleted-name pattern matches the shared EndShadow; it would fail every caller")
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test . -run 'TestShadowKeysLiveInActions|TestShadowConditionIsNamedOnce|TestOneShadowFollowSite|TestDeletedShadowForksStayDeleted' -count=1 -v`
Expected: four `--- PASS` lines.

- [ ] **Step 3: Prove each assertion can fail, with a violation that COMPILES**

A build error is not a proof. Do these one at a time: make the change, run `go build ./...` (must succeed), run the Step 2 command, see the named failure, then undo with `git checkout -- <file>` (every file below is committed by now) and confirm `git status --short` is empty before the next probe.

1. `internal/hooks/MobDeath_TrackingCleanup.go`: add the line `		_ = c.GetMiscData("shadow-target-mob")` directly above `		// Shadow by InstanceId.`. Expect `TestShadowKeysLiveInActions` to report `internal/hooks/MobDeath_TrackingCleanup.go names a shadow misc-data key`.
2. `internal/hooks/PlayerDespawn_TrackingCleanup.go`: add `			c.RemoveCondition(87)` on the line after `			actions.ClearShadow(c)`. Expect `TestShadowConditionIsNamedOnce` to report `internal/hooks/PlayerDespawn_TrackingCleanup.go names condition 87 by literal`.
3. `internal/hooks/MobDeath_TrackingCleanup.go`: add `		u.Command("look")` on the line after `		clearPointers(u.Character)`. Expect `TestOneShadowFollowSite` to report `internal/hooks/MobDeath_TrackingCleanup.go reads a shadow target and queues a command`.
4. `internal/hooks/RoomChange_ShadowFollow.go`: duplicate the line `		f.cmd.Command(exitName)`. Expect `internal/hooks/RoomChange_ShadowFollow.go queues 2 commands, want 1`.
5. `internal/hooks/RoomChange_ShadowFollow.go`: add `	_, _ = actions.ShadowTargetOf(nil)` as the first line of `shadowArrivalPass`. Expect `calls ShadowTargetOf 2 times, want 1`.
6. `internal/usercommands/skill.skullduggery.shadow.go`: append `func endShadow(u *users.UserRecord) { actions.EndShadow(actions.NewUserActor(u), "") }` at the end of the file (an unused function compiles). Expect `TestDeletedShadowForksStayDeleted` to report the file names `"endShadow("`.
7. Comments do not count: in `internal/hooks/MobDeath_TrackingCleanup.go` replace `// Shadow by InstanceId.` with `// Shadow by InstanceId (was shadow-target-mob, condition = 87, endShadow(), u.Command()).`. Expect all four tests to PASS.

Record the six failure messages and the comment-probe pass in the commit body.

- [ ] **Step 4: Commit**

```bash
git status --short
git add shadow_follow_guard_test.go
git commit -m "test(guard): one shadow follow site, the shadow state only in internal/actions" -m "<paste the six probe failure messages and the comment-probe pass here>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

`git status --short` before the add must show only `?? shadow_follow_guard_test.go`.

---

### Task 9: Docs, full gate, race run, boot check

**Model:** sonnet.

**Files:**
- Modify: `internal/actions/context.md`, `internal/hooks/context.md`, `internal/usercommands/context.md`, `internal/mobcommands/context.md`, `internal/events/context.md`, `docs/PATCH_NOTES.md`, `docs/README.md`

- [ ] **Step 1: Verify every symbol before naming it**

Run (PowerShell): `Select-String -Path internal\actions\shadow.go,internal\actions\relocate_mob.go,internal\hooks\RoomChange_ShadowFollow.go,internal\events\events.go -Pattern '^(func|type|const|var)\s'`
Expected to include: `const ShadowingConditionId = 87`, `func ShadowSenseRoll(shadower, target Actor, room *rooms.Room) bool`, `func ShadowTargetOf(c *characters.Character) (userId, mobInstanceId int)`, `func ClearShadow(c *characters.Character)`, `func EndShadow(actor Actor, reason string)`, `func RelocateMob(mob *mobs.Mob, from *rooms.Room, exitName string, dest *rooms.Room, sneaking bool)`, `func RoomChangeShadowFollow(e events.Event) events.ListenerReturn`, `func shadowExitTo(room *rooms.Room, toRoomId int) string`, `func DrainQueuedUserInputsForTest(userId int) []string`. Name nothing that is not in that output.

- [ ] **Step 2: `internal/actions/context.md`**

(a) Replace the `RelocateMob` bullet (`:303-309`, from `- **`RelocateMob(mob *mobs.Mob, from *rooms.Room, exitName string, dest *rooms.Room)`**` through `an NPC party's idle (not in-combat) members through the same exit.`) with:

```markdown
- **`RelocateMob(mob *mobs.Mob, from *rooms.Room, exitName string, dest *rooms.Room, sneaking bool)`**
  is the mob's move with no gate and no charge: walking
  (`mobcommands.Go`, after its own lock check, passing `IsHidden()` or the
  `sneaking` flag) and a successful flee (`hooks.handleMobFlee`, passing
  `IsHidden()`) both end here. It removes the mob from `from`, calls
  `ClearRoomAggroOnDeparture`, adds it to `dest`, narrates both sides
  (sight-gated, with a sound fallback) unless `sneaking`, plays the movement
  sounds either way, and pulls an NPC party's idle (not in-combat) members
  through the same exit. A sneaking mob sends no exit, entry or next-room
  line, as a sneaking player never has (parity slice 6, ruling D1).
```

(b) Replace the whole `### Shadow` section (`:1258` down to the line before `### Track`) with the block below (the outer four-backtick fence is not part of the text; the inner `go` fence is):

````markdown
### Shadow

**Function:** `Shadow(actor, opts) ShadowResult`

Follow a quarry while hidden. Player and mob actors, player and mob quarries,
all by the same rules (player/mob parity slice 6).

- **Gates:** the actor must be hidden (`Character.IsHidden`), not in combat,
  name a target, and be off the shadow cooldown (`Balance.ShadowCooldown`
  rounds; the Go default is 0, which a cooldown rounds to one round, and
  `config.yaml` ships 5).
- **Start:** stores the quarry (read back with `ShadowTargetOf`), applies
  `ShadowingConditionId` (condition 87, 25 rounds), sends the start line, and
  runs `ShadowSenseRoll`: `Detected` reports it; the shadow starts either way.
- **Following:** `hooks.RoomChangeShadowFollow` moves the shadower after its
  quarry on any named-exit move and runs the arrival check. This package owns
  no follow logic.
- **`ShadowSenseRoll(shadower, target Actor, room *rooms.Room) bool`** is the
  one shadow contest: the target's `CalcDetectionScore` (as attacker) against
  the shadower's `CalcSneakScoreVsObserver` times its `SightMult`, through
  `combat.RunContest`, in `room`'s light. It awards the shadower's
  Skullduggery on both outcomes (a win when unsensed), sends a player target
  "You sense someone following close behind you." when it senses the
  shadower, shows a mob target nothing, reveals no one, and returns whether
  the target sensed. Run at the start of a shadow and on each arrival.
- **`ShadowTargetOf(c) (userId, mobInstanceId int)`** reads the quarry. The
  two misc-data keys behind it are unexported constants, and nothing outside
  this package names them (`shadow_follow_guard_test.go`).
- **`ClearShadow(c)`** drops the quarry and condition 87 with no cooldown and
  no line: the stale-state guard and the death and logoff cleanups.
- **`EndShadow(actor, reason)`** is `ClearShadow` plus the cooldown plus
  `reason` to the actor: `shadow stop` and the spotted end.
- `RemoveCondition` only expires condition 87; it is pruned, and a player
  reads its end line, at the next turn.

**Result struct:**
```go
type ShadowResult struct {
	Succeeded  bool   // target id was stored and shadow tracking began
	Detected   bool   // target won the initial sense roll
	TargetName string // display name of the target
	OnCooldown bool   // attempt was blocked by shadow cooldown
	Reason     string // when Succeeded==false and !OnCooldown, why
}
```

````

(c) Replace the table row `| Shadow | actions | self→target | ShadowResult | varies | none |` with `| Shadow | actions | self→target | ShadowResult | varies | ShadowCooldown (start, stop, spotted) |`.

(d) Replace the `ShadowOptions` block (`:1581-1585`) with:

```go
type ShadowOptions struct {
	TargetMobInstanceId int // mob to shadow
	TargetUserId        int // player to shadow
	// Exactly one should be set; the mob id is checked first
}
```

(e) Replace the line (`:1720`)

```markdown
- `internal/modules/follow` — Auto-follow (used by Shadow)
```

with

```markdown
- `internal/hooks`: `RoomChangeShadowFollow` moves a shadower after its quarry (Shadow sets the state it reads)
```

- [ ] **Step 3: `internal/hooks/context.md`**

(a) Replace the row starting `| `Death_MobTracking_Cleanup.go` |` (`:1211`) with:

```markdown
| `MobDeath_TrackingCleanup.go` | Clears `tracking-mob` / `tracking-display-count` + condition 86, and drops any shadow on the dying mob through `actions.ClearShadow` (found with `actions.ShadowTargetOf`; no cooldown, no line), on every player and mob (chunk 2.8; parity slice 6) |
```

(b) Replace the row starting `| `PlayerDespawn_TrackingCleanup.go` |` (`:1219`) with:

```markdown
| `PlayerDespawn_TrackingCleanup.go` | Clears `tracking-user` / `tracking-display-count` + condition 86, and drops any shadow on the departing user through `actions.ClearShadow` (no cooldown, no line), on every other player and every mob (chunk 2.8; parity slice 6) |
```

(c) Insert immediately above `### Scheduler observer (in `validate.go` + `mobs.go`)`:

```markdown
### RoomChange_ShadowFollow.go

Registered as a `RoomChange` event listener (player/mob parity slice 6); it
replaced a loop in `usercommands/go.go` and `MobRoomChange_ShadowFollow.go`,
which moved only player shadowers. `RoomChangeShadowFollow` runs two passes:

- **Follow.** `shadowExitTo` resolves the exit's map key from `Exits`, then
  `ExitsTemp`, then the active mutators' exits (not `Room.FindExitTo`, which
  returns a temporary exit's title). No exit (a teleport): nothing. Otherwise
  every player and mob in the old room whose `actions.ShadowTargetOf` names
  the mover is checked: no condition 87 means stale, `actions.ClearShadow`;
  not hidden, it stays; else it queues the exit through one `Command` call
  (users and mobs share the signature).
- **Arrival.** When the mover has a live shadow whose quarry is in the room
  it entered: not hidden, `actions.EndShadow` with "You've been spotted --
  your shadow ends."; still hidden, `actions.ShadowSenseRoll` in that room.

It reads `IsHidden()` live, never `RoomChange.Unseen`: listeners run after the
moving command returns, so entry detection has already run.
`TestRoomChangeShadowFollowIsRegistered` reads `hooks.go`, and
`shadow_follow_guard_test.go` keeps it the only follow site.

```

(d) Replace the census paragraphs from `135 non-test files (recounted for baubles Phase 6c, which added` through `The remaining 35 files are shared helpers rather than handlers and carry no` with (first confirm the numbers with `ls internal/hooks/*.go | grep -v _test.go | wc -l` (139) and `ls internal/hooks/*.go | grep -v _test.go | grep '^internal/hooks/[A-Z]' | wc -l` (100)):

```markdown
139 non-test files (recounted for parity slice 6, which replaced
`MobRoomChange_ShadowFollow.go` with `RoomChange_ShadowFollow.go`). The
filename **is** the index. Each is named for the event it handles and the job
it does, so `NewRound_IdleMobs.go` is the idle-mob step of the new-round
event.

The prefix IS the event name, so there is no short list of them: 100 of the 139
files carry one and they spell 42 distinct events. The big ones are
`NewRound_*` (21 files), `Death_*` (11), `MobDeath_*` (7), `NewTurn_*`,
`Position_*` and `RoomChange_*` (5 each), `CombatPhase_*` (4), and
`Awareness_*` and `PlayerDespawn_*` (3 each); the other 33
prefixes carry one or two files apiece. Enumerate them with
`ls internal/hooks/*.go | sed 's/_.*//' | sort -u` rather than trusting a list
here. There is no `Input_*` or `Combat_*` prefix.

The remaining 39 files are shared helpers rather than handlers and carry no
```

- [ ] **Step 4: `internal/usercommands/context.md`, `internal/mobcommands/context.md`, `internal/events/context.md`**

(a) `usercommands/context.md:270-276`: replace

```markdown
  in `actions.EntryDetection`, shared with mobs. The one in
  `skill.skullduggery.shadow.go` and the one in `throw.go` are still here and
  all resolve through
```

with

```markdown
  in `actions.EntryDetection`, shared with mobs. The shadow contest moved to
  `actions.ShadowSenseRoll` in parity slice 6. The one in `throw.go` is
  still here and resolves through
```

and add, as a new bullet directly after that `internal/combat` bullet:

```markdown
- `shadow` (`skill.skullduggery.shadow.go`) only resolves the target and calls
  `actions.Shadow`; `shadow stop` reads `actions.ShadowTargetOf` and calls
  `actions.EndShadow`. Following, the spotted end and the sense roll live in
  `hooks.RoomChangeShadowFollow` (parity slice 6), not in `go.go`.
```

(b) `mobcommands/context.md:276-281`: after the sentence ending `so both callers` / `share it.`, append to that bullet:

```markdown
  `Go` passes its `sneaking` state (`IsHidden()` or the `sneaking` flag), so a
  sneaking mob's step is not announced (parity slice 6, ruling D1).
```

(c) `events/context.md`: after the line `` `Room.SendTextToExits`) rather than a per-user one.`` add:

```markdown
`DrainQueuedUserInputsForTest(userId)` (parity slice 6) drains the `Input`
events `UserRecord.Command` queued for a player (`MobInstanceId` 0, matched on
`UserId`); `DrainQueuedInputsForTest` and `InspectQueuedInputForTest` match
mobs only.
```

Then run `python tools/context_md_audit.py` and expect no phantom symbol reported for `internal/actions`, `internal/hooks`, `internal/usercommands`, `internal/mobcommands` or `internal/events`. Search the five files for `—` and `–` in any line this step added: there must be none.

- [ ] **Step 5: Patch notes**

Add at the top of `docs/PATCH_NOTES.md`, directly under `# DOGMud Patch Notes` and its blank line, above `## 2026-09-30: Voices in the dark` (player-facing, no numbers, no dashes; the heading takes the date the PR merges, shown here as 2026-09-30):

```markdown
## 2026-09-30: Shadows that follow

- Creatures can shadow you now. A hidden creature that sets out to shadow
  someone follows them from room to room, as a player can.
- A creature that is sneaking no longer announces itself as it comes and
  goes, just as a sneaking player never has.
- If you are shadowing someone and they spot you as you arrive, your shadow
  ends, and you must wait a little before you can shadow again.
- The feeling that someone is following close behind you now comes once
  they have arrived, and it depends on the light where you both stand.
- Every step you take while shadowing trains your skullduggery, whether or
  not your quarry senses you.
- A shadow now follows a quarry who flees, or who is moved to the next room
  by other means.
- Starting to shadow a creature is now a real test of your stealth against
  its eyes, as it is against a player's.
```

- [ ] **Step 6: `docs/README.md`**

The plan's row was added with the plan. Add a row for the new guard directly below it, matching the 5b guard row (`:165`):

```markdown
| [`../shadow_follow_guard_test.go`](../shadow_follow_guard_test.go) | Parity slice 6 re-fork guard: the shadow misc-data keys only in internal/actions, condition 87 named only in actions/shadow.go, one shadow follow site (RoomChange_ShadowFollow.go, one ShadowTargetOf and one Command), and the deleted forks stay deleted |
```

- [ ] **Step 7: Full gate**

```bash
gofmt -l internal/ modules/ .
go vet ./...
go build ./...
go test ./... -count=1
golangci-lint run --new-from-merge-base=origin/master
```

Expected: gofmt and vet print nothing; every package `ok`; lint `0 issues` (a stale-cache warning naming another worktree's path is not an issue). A root guard keyed by `file|literal` or by line that reports a moved site is re-keyed in this commit, never deleted without reading it; the plan measured none beyond Tasks 3, 5 and 6. A failure unrelated to this slice is compared against a detached master worktree (`git worktree add --detach C:/tmp/dogmud-shadow-base origin/master`), reported, and not fixed here; remove that worktree afterwards.

- [ ] **Step 8: Race run in Docker**

```bash
docker build --target test -f provisioning/Dockerfile -t dogmud-shadow-test .
docker run --rm dogmud-shadow-test > "$TMP/shadow-race.log" 2>&1
grep -E '^(--- FAIL|FAIL|WARNING: DATA RACE)' "$TMP/shadow-race.log"
```

Expected: no `WARNING: DATA RACE`; the only failures are the two tests that shell out to `git`, which has no repository inside the image (`TestNoStringOrDataSaysBuff` in the root package and `TestU10DoneWhen_DeadPathsStayDead` in `internal/combat`), with their `FAIL` package lines. Any other failure is a finding. Remove the image afterwards (`docker rmi dogmud-shadow-test`).

- [ ] **Step 9: Boot check on private ports (per `dogmud-shipping`)**

The owner's server uses 8090; this check binds only private ports and stops only the process it started (`timeout` ends its own child).

```bash
git worktree add --detach C:/tmp/dogmud-shadow-boot HEAD
cp "C:/Users/Calabe Davis/workspace/DOGMud/_datafiles/config.yaml" C:/tmp/dogmud-shadow-boot/_datafiles/config.yaml
cd C:/tmp/dogmud-shadow-boot
printf 'Network.TelnetPort: [33337]\nNetwork.LocalPort: 9997\nNetwork.HttpPort: 8097\nNetwork.HttpsPort: 0\nNetwork.AIPort: 0\n' > boot-overrides.yaml
go build -o boot-check.exe .
CONFIG_PATH=C:/tmp/dogmud-shadow-boot/boot-overrides.yaml LOG_NOCOLOR=1 timeout 180 ./boot-check.exe > boot.log 2>&1
echo "exit $?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log
grep -c "Server Ready" boot.log
```

Expected: `exit 124`, then `0`, then `1`. Then `cd` back, `git worktree remove --force C:/tmp/dogmud-shadow-boot` (if Windows holds the exe, PowerShell `Remove-Item -Recurse -Force C:\tmp\dogmud-shadow-boot`, then `git worktree prune`). Never stop a server this session did not start.

- [ ] **Step 10: Commit**

```bash
git add internal/actions/context.md internal/hooks/context.md internal/usercommands/context.md internal/mobcommands/context.md internal/events/context.md docs/PATCH_NOTES.md docs/README.md
git commit -m "docs(shadow-follow): context.md, patch notes and README for parity slice 6" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(Add by name any guard file re-keyed in Step 7.)

---

### Task 10: Playtest and PR

**Model:** opus (judgment on live findings).

- [ ] **Step 1: Playtest (the spec's procedure, exactly)**

Load `dogmud-playtesting` and follow it: an ephemeral scenario written in the scratchpad (not the repo), `--checkout` of `C:/tmp/dogmud-shadow`, private ports, never touch the owner's server, and reports are gitignored so findings go to memory. Model the scenario on `tools/playtest/scenarios/slice-a-hidden.yaml`. Roster: an `admin` profile (the operator), a target player with low Perception and no Search (a `fresh` profile with those stats), and a witness (`m2-witness`). Location: a lit room holding a non-wandering mob fixture with good Dexterity, on a route of three or more rooms joined by named exits (the operator finds one with `locate` and the zone files, or spawns the mob). Every actor quotes lines verbatim.

*Mob shadows player.*
1. Operator alone in the room with the mob (every player present is an observer the sneak must beat, the operator included). Run `command <mob> sneak` until the mob no longer appears in `look`. `setcondition <mob> 9` is NOT a fallback: it adds the condition without hiding the mob.
2. Target and witness enter. If entry detection reveals the mob (the target's or witness's newcomer roll), send them out and repeat step 1.
3. `command <mob> shadow <target>` (the operator can name a hidden mob).
4. Target walks the route by named exits. After each step the operator runs `locate <mob>` and confirms the mob is in the target's room. The target and witness report that no line names the mob leaving or arriving (D1), that the mob is absent from `look`, and whether `You sense someone following close behind you.` appeared.

*Player shadows mob.* A player given Skullduggery 3 (`skillset`) sneaks, types `shadow <mob>`, and the operator walks the mob with `command <mob> <exit>` through three rooms, following it (`command` reaches only the operator's own room). The player arrives each step, or reads `You've been spotted -- your shadow ends.` and stops.

*Covered by unit tests only:* spotted-on-arrival and teleport, since a live roll cannot be forced.

Extract every finding to memory (a `project-shadow-follow-playtest-findings-2026-09-30` file and a pointer line in `MEMORY.md`, added with the Edit tool).

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feature/shadow-follow
gh pr create --repo pruuk/DOGMud --base master --head feature/shadow-follow --title "feat(shadow): every shadower follows by one rule, checked on arrival (parity slice 6)" --body-file "$TMP/shadow-pr-body.md"
```

Body: the spec's parity table; the five items under "Where the spec could not be implemented as written" (lead with F16, the Go default of `ShadowCooldown` is 0, for the owner); the "Player-visible lines that change" table; the guards re-keyed (the `contestSiteOwners` swap and deletion with the block realignment, `legacyLiteralFiles`, the one retired narration key) and the new guard with its seven probes; gate results with counts; the race run; the boot check; the playtest outcome; the spec and plan paths; and a line that CI minutes are exhausted for September, so the local gate, race run and boot check are the merge gate. End with:

```
🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

Read back the URL `gh` prints and confirm it says `pruuk/DOGMud`. The owner runs any deploy; do not deploy, and do not nag about deploying.

---

## Self-review

- **Spec coverage.** Rule 1, one listener with both passes, every mover and shadower kind, stale state cleared for both kinds, teleport moves no one, no wait argument (T5); the go.go loop, `endShadow`, `shadowIsTargetingUser`, `shadowDetectionRoll`, `getShadowTargetUserId`, `MobRoomChange_ShadowFollow.go` and `inlineShadowEnd` deleted (T5, T6); `shadowingCondition` const deleted and 87 named once as `ShadowingConditionId` (T2, T7). Rules 2 and 3, `ShadowSenseRoll` with the target as attacker, the real room's light, award on both outcomes, a line to a player target only, no reveal (T3, proven in T3 and T5). Rule 4, `ShadowTargetOf`, `ClearShadow` (D2), `EndShadow` (T2), used by `shadow stop` (T6) and the cleanups (T7). Rule 5, `shadowMob` rolls and returns `Detected`, `shadowPlayer` calls the same body (T3). D1, `RelocateMob` quiet for a sneaking mob, sounds kept after checking `go.go:620-621` (T4). D3, `shadowExitTo` with the mutator fallback, `modules/follow` and `FindExitTo` untouched (T5). D4, the three edges accepted, the late-arrival edge tested (T5). Table test, every case the spec lists for mover x shadower, the temp-exit key case, spotted and sense arrival cases (T5). Action tests, mob start roll, award on both outcomes, `ClearShadow` vs `EndShadow` cooldown, `RelocateMob` quiet (T2, T3, T4). Guard, all four assertions plus the 50-file floor, each proven able to fail by a compiling violation and the comment probe (T8). Existing guards, `contestSiteOwners` and `legacyLiteralFiles` as the spec says, plus the stale narration key it did not foresee (T3, T5, T6). Docs, both G5 stale facts fixed, five `context.md` files, patch notes, README (T9). Gate, race run, boot on private ports (T9). Playtest and PR (T10). Out of scope items untouched: no `try_shadow` authoring, no "target lost" rule, entry detection unchanged.
- **Beyond the spec, stated above.** `DrainQueuedUserInputsForTest` (F12); the `shadowFollower` interface and `shadowOf` helper that give the guard's "once each" its meaning; `shadow_stop_test.go` and `TrackingCleanup_Shadow_test.go`, which pin unchanged behaviour through the refactors; the `ShadowCooldown` Go default and the `RemoveCondition` expiry, both recorded rather than changed.
- **Names across tasks.** `DrainQueuedUserInputsForTest(userId int) []string` (T1); `ShadowingConditionId`, `shadowTargetUserKey`, `shadowTargetMobKey`, `ShadowTargetOf(c) (userId, mobInstanceId int)`, `ClearShadow(c)`, `EndShadow(actor, reason)` (T2); `shadowSensedLine`, `ShadowSenseRoll(shadower, target Actor, room *rooms.Room) bool` (T3); `RelocateMob(..., sneaking bool)` (T4); `RoomChangeShadowFollow`, `shadowSpottedLine`, `shadowFollower`, `shadowFollowPass`, `shadowArrivalPass`, `shadowOf`, `shadowNamesMover`, `shadowExitTo` (T5). Test helpers: `seedShadowingCondition`, `shadowingChar`, `shadowConditionEnded` (actions, T2); `shadowSenseTrials`, `countSensedLines` (actions, T3); `relocateWatchers` (actions, T4); `newShadowScene`, `shadowScene`, `shadowKind`, `shadowKinds`, `sly`, `place`, `shadowTarget`, `moveEvent`, `queued`, `shadowEnded`, `shadowCooldown`, `shadowSensedText` (hooks, T5, reused in T7); `shadowStopUser` (usercommands, T6); `shadowGuardCode`, `shadowGuardWalk` (root, T8). None collides with an existing name in its package (each file built and ran).
- **Placeholder scan.** The only angle-bracketed fills are the playtest's live `<mob>` / `<target>` / `<exit>` and the commit body's probe messages, both of which only the executor can know.
