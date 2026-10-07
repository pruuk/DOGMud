# Lighting Small Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Four small lighting follow-ups before plan 6: `look` and `who` say when darkness is why you cannot see (#364), a dazzled fighter is told once per fight that the glare costs them (#319), hood and light or darkness end lines are judged against the moment before the change (#220), and the unused `DarknessTerms` goes (#221).

**Architecture:** `actions.ResolveLook` tells a blinded viewer (`LookBlind`) from one in a room too dark (`LookTooDark`), and `look`, `who` and the mob look share it. The glare notice is the blind notice's twin in `hooks/combat_verbosity.go`, with a per-user "told this fight" mark cleared when the player leaves combat. For #220, `hood` takes `Room.VisualSnapshot()` before hooding; the user and mob round ticks take one just before `Conditions.Trigger` for any light or darkness record that will expire on that trigger (`Condition.ExpiresOnNextTrigger`), keep it in `hooks/condition_end_snapshot.go` keyed by the record, and the next prune sends the end line against it. `SendTextVisualAsLit`, its `HidingNames` twin and `litRoom` lose their last callers and are deleted.

**Tech Stack:** Go 1.25.

**Spec (binding):** `docs/superpowers/specs/completed/2026-10-06-lighting-small-fixes-design.md` (owner-approved 2026-10-06), with the corrections under "Where the spec could not be implemented as written" below.

**Branch:** implementation branch `fix/lighting-small-fixes`, cut from master AFTER the docs branch `docs/lighting-small-fixes` (the spec and this plan) merges, in the worktree `C:/tmp/dogmud-lightfix`:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b fix/lighting-small-fixes C:/tmp/dogmud-lightfix origin/master
```

(The docs branch uses the same path; remove that worktree once the docs PR merges, then cut this one.) All paths are relative to the worktree. Throwaway output goes in `$TMP`, never `C:/tmp`. Edits use the Edit and Write tools only, never a Python read-modify-write. **An Edit whose old or new text ends in a space loses that space**: anchor on whole lines and run `gofmt -l`. Every commit names its paths (never `git add -A` or `git add .`) and ends with a blank line and then exactly `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; the `-m "..." -m "Co-Authored-By: ..."` form below produces that. Subagent model: sonnet for Tasks 1, 2, 4, 5; opus for Task 3.

**How the code blocks read.** "Create" blocks are the whole file. "Modify" blocks are unified diffs against the file as the previous task left it: apply each hunk with the Edit tool (`-` lines old, `+` lines new, context lines locate it). The spec check for each task is `git diff <checkpoint> -- <the task's paths>` from the implementation worktree, run after staging with `--cached` as well (an untracked file shows as a deletion before it is staged); it must print nothing. The checkpoints live in the dry-run worktree `C:/tmp/dogmud-lightfix-dry`, which shares the repository's object store, so they are reachable by SHA with no fetch (`git cat-file -t bee8ae26e` prints `commit`). Keep that worktree registered until this branch merges.

**Dry run (2026-10-06).** Every block below was applied in this order to `C:/tmp/dogmud-lightfix-dry`, detached at `origin/master` `59851f0a4`, one checkpoint per task: Task 1 `eb4ea1e6a`, Task 2 `d423ea6fd`, Task 3 `b86442e68`, Task 4 `80326c021`, Task 5 `bee8ae26e`. Each failing-first step was run and failed as the step says. No line-keyed guard moved. At the end: `gofmt -l internal/ modules/` clean; `go build ./...`; `go vet ./...`; `go test ./... -count=1`, 129 packages `ok`, no `FAIL`; `golangci-lint run --new-from-merge-base=origin/master` `0 issues.` (one stale-cache warning naming another worktree); `python tools/context_md_audit.py` 27 phantoms before and after, none in a touched package; a Docker race run gave 0 races, 128 packages `ok`, and only the two tests that shell out to `git` failing (no repository in the image: `TestNoStringOrDataSaysBuff` in the root package, `TestU10DoneWhen_DeadPathsStayDead` in `internal/combat`); a boot on private ports (telnet 33334, local 9998, http 8091, AI port off, through a scratch `CONFIG_PATH` overrides file) stayed up to the 150 s timeout (exit 124) with 0 panics and one `Server Ready`.

---

## Facts verified against source (master `59851f0a4`)

The spec's facts table (F1 to F11) holds, with one correction (D7 below). Read in the dry run as well:

| # | Fact | Where |
|---|---|---|
| G1 | `ParticipantSight` decides blindness first, inline: `observer.Perception != nil && observer.Perception.State() == perception.Blinded`; that is the test `ResolveLook` reuses. `Character.HasAnyBlindSource` asks a different question (whether a blinding condition remains) and is not the sight verdict | `internal/messaging/predicates.go:56-60`; `internal/characters/sight.go:24` |
| G2 | `LookDark` is used in `actions/look.go`, `usercommands/look.go`, `mobcommands/look.go` and `actions/sight_gates_parity_test.go` (`usercommands/look_fixture_kind_test.go` only has it inside a test name) | grep |
| G3 | `Character.IsInCombat()` reports whether a character is fighting; the blind notice is flushed once per round from `NewRound_DoCombat.go` | `internal/characters/character.go:784`; `internal/hooks/NewRound_DoCombat.go:107` |
| G4 | `Conditions.Trigger` decrements `TriggersLeft` on the round its `RoundCounter` reaches a multiple of `RoundInterval`; a stacking record derives `TriggersLeft` from `tickStacks` | `internal/conditions/conditions.go:470-516` |
| G5 | `sendConditionStartRoomText(r, snap rooms.VisualSnapshot, msg, names, skip...)` already takes a snapshot; the end sender takes the spec | `internal/hooks/Condition_ApplyConditions.go:244`; `NewTurn_PruneConditions.go:140` |

## Where the spec could not be implemented as written

1. **`LookDark` is renamed `LookBlind`**, beside a new `LookTooDark`, so no caller can keep the old meaning by accident.
2. **`who` calls `actions.ResolveLook(actor, "")`** instead of `ParticipantSight`, so both commands share one refusal (`noSightRefusal(kind)` in `usercommands/look.go`).
3. **Three tests that used a dark room but expected the blind wording now expect the dark line**; `TestWho_BlindViewerIsRefused` is renamed `TestWho_ViewerInTheDarkIsRefused`.
4. **The glare notice has its own flush**, `flushGlareCombatNotices`, called right after the blind flush. "A fight ended" is `!Character.IsInCombat()` at that flush. A notice suppressed at Light verbosity does not count as told. Leaving and re-entering combat between two flushes reads as one fight, so the notice is withheld once at most and never repeats (documented in a comment).
5. **The expiry test is `(*Condition).ExpiresOnNextTrigger(spec)`** beside `Trigger` in `conditions.go`, stacking records included (mirrors `tickStacks`). The snapshot store is `hooks/condition_end_snapshot.go` (`endLineSnapshots`, keyed by record pointer, `keepEndLineSnapshots`, `takeEndLineSnapshot`); `PruneConditions` clears it at the end, so nothing outlives its prune. A holder who changed rooms between the tick and the prune has the end line judged by the new room as it is. `sendConditionEndRoomText` takes a snapshot, mirroring the start sender; `litRoom` goes, and `sendTextVisualJudgedBy` loses its lighting parameter (always the room now). The three existing light end-line tests run the real round tick instead of `expire()`; `TestConditionEndRoomText_LightPathHasNoShapesTier` becomes `..._LightEndNamesTheHolderToWhoSawThemByIt`. New narration test conditions 7010 (gloom) and 7011 (wick).
6. **Two stale comments fixed on the way:** `AnyDarknessSource` and the conditions `context.md` still said "as-lit"; the hooks `context.md` said the blind category sits in neither suppression table (it is in the Light table).
7. **Spec F10 is wrong about `internal/rooms/darkness_compose_test.go:49`:** only that test's NAME mentions `DarknessTerms`; it never calls it, so it is unchanged. The one real caller is `usercommands/darkness_test.go:72`.

## Player-visible lines

| Where | Before | After |
|---|---|---|
| `look`, `who` in a room too dark | You can't see anything! | It is too dark to see. You need light, or eyes that do not need it. |
| Fighting in glare | Nothing | Once per fight: The glare is too bright, so your attacks and defense are weaker. |
| Hooding, a light or darkness ending | Judged as lit, or by the room after the change | Judged by what each watcher could see just before |

## File map

| Task | Files |
|---|---|
| 1 | `internal/actions/look.go`, `context.md`, `sight_gates_parity_test.go`; `internal/mobcommands/look.go`; `internal/usercommands/look.go`, `who.go`, `context.md`, `look_who_darkness_test.go` (new), `darkness_gates_sight_test.go`, `shapes_roster_test.go` |
| 2 | `internal/hooks/combat_verbosity.go`, `NewRound_DoCombat.go`, `NewRound_DoCombat_unified.go`, `combat_glare_notice_test.go` (new) |
| 3 | `internal/conditions/conditions.go`, `conditionspec.go`, `context.md`, `expires_next_test.go` (new); `internal/hooks/condition_end_snapshot.go` (new), `condition_end_snapshot_test.go` (new), `NewRound_UserRoundTick.go`, `NewRound_MobRoundTick.go`, `NewTurn_PruneConditions.go`, `condition_room_text_test.go`, `narration_testhelpers_test.go`, `context.md`; `internal/rooms/rooms.go`, `hiding_senders_test.go`, `context.md`; `internal/usercommands/hood.go`, `hood_test.go`; root guards `messaging_surface_guard_test.go`, `bauble_finder_view_guard_test.go`, `shipped_narration_data_guard_test.go` |
| 4 | `internal/characters/light.go`, `context.md`; `internal/usercommands/darkness_test.go` |
| 5 | `docs/PATCH_NOTES.md`; `internal/hooks/context.md`, `internal/messaging/context.md`, `internal/usercommands/context.md` |

---

### Task 1: `look` and `who` name darkness (#364)

Checkpoint `eb4ea1e6a`. Model: sonnet.

- [ ] **Step 1: Apply the test first, then the code**

Create `internal/usercommands/look_who_darkness_test.go` (block below) and run `go test ./internal/usercommands -run TestLookAndWho -count=1`: it fails to build until the `LookTooDark` kind exists, or, with the kinds in place and the wording unchanged, fails with `"You can't see anything!" does not contain "It is too dark to see. You need light, or eyes that do not need it."`. `TestLookAndWho_BlindedKeepsThePlainRefusal` passes before and after: it guards the blind wording. Then apply the rest.

**Modify `internal/actions/context.md`:**

````diff
@@ -481,18 +481,22 @@ see. `GetGoldFromFloor(actor, amount) error` (`:83`) refuses the same way
 before `FloorPickupGold`.
 
 **`look.go`** (new file, slice 5a): `ResolveLook(actor Actor, lookAt string)
-LookResolution` (`:47`) is the shared look body, in the player's exact order:
+LookResolution` (`:49`) is the shared look body, in the player's exact order:
 sight first, then no-target (the room), then a creature (resolved with the
 looker itself as `ResolveTargetOptions.Viewer`, so nothing it does not
 perceive can be named), then a sealed crate or a known room container
-(`lookNamesAnObject`, `:108`), then an exit (direction alias resolved, then
-through-sight, then lock). `LookKind` (`:11`) is `LookDark`, `LookRoom`,
-`LookCreature`, `LookExit`, `LookExitTooDark`, `LookExitLocked`, `LookOther`.
+(`lookNamesAnObject`, `:116`), then an exit (direction alias resolved, then
+through-sight, then lock). `LookKind` (`:12`) is `LookBlind`, `LookTooDark`,
+`LookRoom`, `LookCreature`, `LookExit`, `LookExitTooDark`, `LookExitLocked`,
+`LookOther`. Sight `SightNone` splits by cause (#364): `LookBlind` for a
+looker whose `Perception` is `Blinded` (the same check `ParticipantSight`
+answers `SightNone` on first), `LookTooDark` for one the room is too dark
+for, so a caller can say that light would help.
 `LookOther` is deliberately one bucket for "anything else": the crate and
 container case defers to it rather than getting its own kind, so each
 wrapper's own noun and item resolution (which differs between the player and
 a mob) runs in its own order afterward, exactly as it does today.
-`LookResolution.PetUserId` (`:38`) is set only at `NamesCreatures` (clear
+`LookResolution.PetUserId` (`:40`) is set only at `NamesCreatures` (clear
 sight) and is NOT a `LookKind`: the player resolves a pet AFTER carried items
 and room nouns, so turning it into a kind would move the pet check ahead of
 those and change look order. Each wrapper reads `PetUserId` at its own
````

**Modify `internal/actions/look.go`:**

````diff
@@ -5,13 +5,15 @@ import (
 
 	"github.com/GoMudEngine/GoMud/internal/keywords"
 	"github.com/GoMudEngine/GoMud/internal/messaging"
+	"github.com/GoMudEngine/GoMud/internal/state/perception"
 )
 
 // LookKind is what a look resolved to.
 type LookKind int
 
 const (
-	LookDark        LookKind = iota // the looker sees nothing at all here
+	LookBlind       LookKind = iota // the looker is blinded and sees nothing anywhere
+	LookTooDark                     // the room is too dark for the looker's eyes
 	LookRoom                        // no target: the room itself
 	LookCreature                    // a creature the looker perceives, at clear sight
 	LookExit                        // an exit the looker can see through
@@ -51,7 +53,13 @@ func ResolveLook(actor Actor, lookAt string) LookResolution {
 	res.NamesCreatures = res.Sight == messaging.SightFull
 
 	if res.Sight == messaging.SightNone {
-		res.Kind = LookDark
+		// The cause, told apart by the same Perception check that
+		// ParticipantSight answers SightNone on first: light helps a looker
+		// in the dark and does nothing for a blinded one (#364).
+		res.Kind = LookTooDark
+		if char.Perception != nil && char.Perception.State() == perception.Blinded {
+			res.Kind = LookBlind
+		}
 		return res
 	}
 	if lookAt == `` {
````

**Modify `internal/actions/sight_gates_parity_test.go`:**

````diff
@@ -207,9 +207,12 @@ func TestGateParity_ResolveLook(t *testing.T) {
 				assert.Equal(t, LookRoom, room.Kind, who)
 				assert.Equal(t, LookOther, creature.Kind, "%s: at shapes a creature is not named", who)
 				assert.False(t, creature.NamesCreatures, who)
-			default:
-				assert.Equal(t, LookDark, room.Kind, "%s at %s", who, light)
-				assert.Equal(t, LookDark, creature.Kind, "%s at %s", who, light)
+			case gateDark:
+				assert.Equal(t, LookTooDark, room.Kind, "%s at %s", who, light)
+				assert.Equal(t, LookTooDark, creature.Kind, "%s at %s", who, light)
+			case gateBlinded:
+				assert.Equal(t, LookBlind, room.Kind, "%s at %s", who, light)
+				assert.Equal(t, LookBlind, creature.Kind, "%s at %s", who, light)
 			}
 		}
 	}
````

**Modify `internal/mobcommands/look.go`:**

````diff
@@ -40,7 +40,7 @@ func Look(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
 	// observer's sight, as the player's do.
 	res := actions.ResolveLook(actions.NewMobActorInRoom(mob, room), rest)
 	switch res.Kind {
-	case actions.LookDark, actions.LookExitTooDark, actions.LookExitLocked:
+	case actions.LookBlind, actions.LookTooDark, actions.LookExitTooDark, actions.LookExitLocked:
 		return true, nil
 
 	case actions.LookRoom:
````

**Modify `internal/usercommands/context.md`:**

````diff
@@ -538,8 +538,11 @@ a gate.
 - **`Look`** (`look.go`): every sight rule (no-sight refusal, a creature named
   only at clear sight and only if perceived, an exit's through-sight and
   lock, the pet at clear sight) lives in `actions.ResolveLook`, shared with
-  the mob look; this function switches on `res.Kind` (`actions.LookDark`,
-  `LookRoom`, `LookCreature`, ...) and only words the answer.
+  the mob look; this function switches on `res.Kind` (`actions.LookBlind`,
+  `LookTooDark`, `LookRoom`, `LookCreature`, ...) and only words the answer.
+  `noSightRefusal` words the two no-sight kinds for `look` and `Who`: a
+  blinded looker reads "You can't see anything!", one in a room too dark
+  is told light or other eyes would help (#364).
 - **`Remove`** (`remove.go`): the `all` branch calls
   `actions.RemoveAllEquipment(actor)`, which owns the busy gate, the curse
   gate per item and one `EquipmentChange` event per removal; the wrapper
````

**Modify `internal/usercommands/darkness_gates_sight_test.go`:**

````diff
@@ -84,7 +84,7 @@ func gateOutputs(t *testing.T, user *users.UserRecord, room *rooms.Room) map[str
 }
 
 var gateRefusals = map[string]string{
-	"look": "You can't see anything!",
+	"look": tooDarkToSeeLine,
 	"get":  "You can't see anything to pick up!",
 	"loot": "You can't see anything to loot!",
 }
@@ -146,11 +146,11 @@ func TestLookDirection_ExitThresholdShiftsWithNightVisionStrength(t *testing.T)
 			}
 			out := runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
 			if c.refused {
-				require.True(t, strings.Contains(out, tooDark) || strings.Contains(out, "You can't see anything!"),
+				require.True(t, strings.Contains(out, tooDark) || strings.Contains(out, tooDarkToSeeLine),
 					"must refuse; got:\n%s", out)
 			} else {
 				require.NotContains(t, out, tooDark)
-				require.NotContains(t, out, "You can't see anything!")
+				require.NotContains(t, out, tooDarkToSeeLine)
 				require.Contains(t, out, "You peer toward the south")
 			}
 		})
````

**Modify `internal/usercommands/look.go`:**

````diff
@@ -47,8 +47,8 @@ func Look(rest string, user *users.UserRecord, room *rooms.Room, flags events.Ev
 	// clear sight and only if perceived, the exit's through-sight and lock,
 	// the pet at clear sight. This function only words the answer.
 	res := actions.ResolveLook(&actions.UserActor{User: user, Room: room}, lookAt)
-	if res.Kind == actions.LookDark {
-		user.SendText(messaging.CategorySystem, `You can't see anything!`)
+	if line, refused := noSightRefusal(res.Kind); refused {
+		user.SendText(messaging.CategorySystem, line)
 		return true, nil
 	}
 	sight := res.Sight
@@ -512,6 +512,20 @@ func Look(rest string, user *users.UserRecord, room *rooms.Room, flags events.Ev
 
 }
 
+// noSightRefusal words the refusal of a looker who sees nothing here, shared
+// by look and who. A looker in the dark is told that light, or other eyes,
+// would help; a blinded one is not, since light would not (#364). refused is
+// false for every kind that sees something.
+func noSightRefusal(kind actions.LookKind) (line string, refused bool) {
+	switch kind {
+	case actions.LookBlind:
+		return `You can't see anything!`, true
+	case actions.LookTooDark:
+		return `It is too dark to see. You need light, or eyes that do not need it.`, true
+	}
+	return ``, false
+}
+
 // floorItemNamedInFull finds a floor item the words name in full, as more
 // than one word ("arch lantern"). A multi-word full name is specific: it
 // outranks a room noun or a carried item that only one of its words matches.
````

**Create `internal/usercommands/look_who_darkness_test.go`:**

````go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/stretchr/testify/require"
)

// File: look_who_darkness_test.go
//
// #364. A player who sees nothing because the room is too dark is told that
// darkness is the reason, and what would help; a blinded player keeps the
// plain refusal, since light would not help them.

const (
	tooDarkToSeeLine = "It is too dark to see. You need light, or eyes that do not need it."
	blindLookLine    = "You can't see anything!"
)

func TestLookAndWho_TooDarkNamesTheDarkness(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 0)
	for verb, cmd := range map[string]func() (bool, error){
		"look": func() (bool, error) { return Look("", user, room, 0) },
		"who":  func() (bool, error) { return Who("", user, room, 0) },
	} {
		out := runGate(t, user, cmd)
		require.Contains(t, out, tooDarkToSeeLine, "%s in a pitch-dark room", verb)
		require.NotContains(t, out, blindLookLine, "%s in a pitch-dark room", verb)
	}
}

func TestLookAndWho_BlindedKeepsThePlainRefusal(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 90)
	orig := user.Character.Perception
	t.Cleanup(func() { user.Character.Perception = orig })
	user.Character.Perception = characters.New().Perception
	require.NoError(t, user.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))

	for verb, cmd := range map[string]func() (bool, error){
		"look": func() (bool, error) { return Look("", user, room, 0) },
		"who":  func() (bool, error) { return Who("", user, room, 0) },
	} {
		out := runGate(t, user, cmd)
		require.Contains(t, out, blindLookLine, "%s while blinded in a lit room", verb)
		require.NotContains(t, out, tooDarkToSeeLine, "%s while blinded in a lit room", verb)
	}
}
````

**Modify `internal/usercommands/shapes_roster_test.go`:**

````diff
@@ -63,11 +63,11 @@ func TestLookAtCreature_ClearViewerStillLooks(t *testing.T) {
 	require.NotContains(t, out, "Look at what???")
 }
 
-func TestWho_BlindViewerIsRefused(t *testing.T) {
+func TestWho_ViewerInTheDarkIsRefused(t *testing.T) {
 	viewer, room := seedShapesRosterRoom(t, 10, 0)
 
 	out := runGate(t, viewer, func() (bool, error) { return Who("", viewer, room, 0) })
-	require.Contains(t, out, "You can't see anything!")
+	require.Contains(t, out, tooDarkToSeeLine)
 	require.NotContains(t, out, "Bobrick")
 }
 
@@ -79,7 +79,8 @@ func TestWho_SightedViewerIsNotRefused(t *testing.T) {
 		t.Run(name, func(t *testing.T) {
 			viewer, room := seedShapesRosterRoom(t, c.lamp, c.condition)
 			out := runGate(t, viewer, func() (bool, error) { return Who("", viewer, room, 0) })
-			require.NotContains(t, out, "You can't see anything!")
+			require.NotContains(t, out, tooDarkToSeeLine)
+			require.NotContains(t, out, blindLookLine)
 		})
 	}
 }
````

**Modify `internal/usercommands/who.go`:**

````diff
@@ -1,6 +1,7 @@
 package usercommands
 
 import (
+	"github.com/GoMudEngine/GoMud/internal/actions"
 	"github.com/GoMudEngine/GoMud/internal/events"
 	"github.com/GoMudEngine/GoMud/internal/messaging"
 	"github.com/GoMudEngine/GoMud/internal/rooms"
@@ -10,10 +11,13 @@ import (
 
 func Who(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
 
-	// Refused exactly where look is: a viewer who makes out nothing here has
-	// no roster to read. At shapes GetDetails lists anonymous figures.
-	if messaging.ParticipantSight(user.Character, room) == messaging.SightNone {
-		user.SendText(messaging.CategorySystem, `You can't see anything!`)
+	// Refused exactly where look is, with look's words: a viewer who makes
+	// out nothing here has no roster to read. The untargeted look resolution
+	// decides it, so blinded and too dark are told apart as look tells them.
+	// At shapes GetDetails lists anonymous figures.
+	res := actions.ResolveLook(&actions.UserActor{User: user, Room: room}, ``)
+	if line, refused := noSightRefusal(res.Kind); refused {
+		user.SendText(messaging.CategorySystem, line)
 		return true, nil
 	}
 
````

- [ ] **Step 2: Packages and root guards**

```bash
gofmt -l internal/
go build ./...
go test ./internal/actions ./internal/usercommands ./internal/mobcommands -count=1
go test . -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: all `ok`.

- [ ] **Step 3: Spec check and commit**

```bash
P="internal/actions/context.md internal/actions/look.go internal/actions/sight_gates_parity_test.go internal/mobcommands/look.go internal/usercommands/context.md internal/usercommands/darkness_gates_sight_test.go internal/usercommands/look.go internal/usercommands/look_who_darkness_test.go internal/usercommands/shapes_roster_test.go internal/usercommands/who.go"
git add $P
git diff --cached eb4ea1e6a -- $P
git commit -m "fix(look): look and who say when darkness is why you cannot see (#364)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

The diff must print nothing.

---

### Task 2: the once-per-fight glare notice (#319)

Checkpoint `d423ea6fd`. Model: sonnet.

- [ ] **Step 1: The test first**

Create `internal/hooks/combat_glare_notice_test.go` (block below). To make it compile before the behaviour exists, add empty declarations of `glareCombatNoticeText`, `markGlareCombatant` and `flushGlareCombatNotices` in `combat_verbosity.go`, then run `go test ./internal/hooks -run TestGlareCombatNotice -count=1`: three subtests fail with `Not equal` ("the first round of the fight tells them", "a later fight tells them again", "medium keeps the notice"); the four fixture preconditions pass (lamp 95 is sight full, the bright fraction is above 0, the sight cost is below 1.0). Then apply the rest.

**Modify `internal/hooks/NewRound_DoCombat.go`:**

````diff
@@ -106,6 +106,9 @@ func DoCombat(e events.Event) events.ListenerReturn {
 	// (M4d PR 2, Task 4).
 	flushBlindCombatNotices()
 
+	// Once-per-fight glare notice for dazzled combatants (#319).
+	flushGlareCombatNotices()
+
 	return events.Continue
 }
 
````

**Modify `internal/hooks/NewRound_DoCombat_unified.go`:**

````diff
@@ -576,6 +576,11 @@ func dispatchCritAndMessaging(atk, def actions.Actor, res *combat.AttackResult)
 	// sent until flushBlindCombatNotices runs at end of round.
 	markBlindCombatant(atk, srcCanSee)
 	markBlindCombatant(def, tgtCanSee)
+	// #319: the once-per-fight glare notice, for a player who sees clearly
+	// while too much light costs them. Same verdicts, same record-only seam;
+	// flushGlareCombatNotices sends.
+	markGlareCombatant(atk, srcCanSee)
+	markGlareCombatant(def, tgtCanSee)
 
 	// Crit effects (riposte / sweep / bash) compute side-specific text.
 	critResult := applyCritEffects(atkChar, defChar, *res, atkRoom)
````

**Create `internal/hooks/combat_glare_notice_test.go`:**

````go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// File: combat_glare_notice_test.go
//
// #319. A dazzled fighter sees every face (SightFull), so the blind notice
// never fires for them, yet glare lowers their SightMult all the same. They
// are told once per fight, through the same production seam as the blind
// notice (dispatchCritAndMessaging marks, flushGlareCombatNotices sends).

// glareNoticeNeedle is a substring unique to glareCombatNoticeText.
const glareNoticeNeedle = "glare is too bright"

type glareScene struct {
	u1       *users.UserRecord
	atk, def actions.Actor
	captured *[]events.Message
}

// newGlareScene puts user 1 in combat with mob 100 in skyless room 2 at a
// pinned lamp, with both notice sets reset.
func newGlareScene(t *testing.T, lamp int) glareScene {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedNarrationConditions())
	roundBlindCombatants = map[int]bool{}
	roundGlareCombatants = map[int]bool{}
	glareToldThisFight = map[int]bool{}

	room2 := rooms.LoadRoom(2)
	require.NotNil(t, room2)
	room2.Biome = "cave"
	room2.Lamp = rooms.LampPtr(lamp)
	require.Equal(t, lamp, room2.LightLevel(), "room 2 must sit at the pinned lamp")

	u1 := users.GetByUserId(1)
	require.NotNil(t, u1)
	u1.Character.RoomId = 2
	rooms.LoadRoom(1).RemovePlayer(1)
	room2.AddPlayer(1)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	u1.Character.SetAggro(0, mob.InstanceId, characters.DefaultAttack)
	require.True(t, u1.Character.IsInCombat(), "fixture: user 1 must be in a fight")
	t.Cleanup(u1.Character.EndAggro)

	captured, capCleanup := captureMessages(t)
	t.Cleanup(capCleanup)
	return glareScene{
		u1:       u1,
		atk:      actions.NewUserActorInRoom(u1, room2),
		def:      actions.NewMobActorInRoom(mob, room2),
		captured: captured,
	}
}

// round runs one combat round's seam and returns the glare notices user 1
// read in it.
func (s glareScene) round() int {
	before := countContaining(textsForUser(*s.captured, 1), glareNoticeNeedle)
	dispatchCritAndMessaging(s.atk, s.def, vbLandingResult())
	flushBlindCombatNotices()
	flushGlareCombatNotices()
	events.ProcessEvents()
	return countContaining(textsForUser(*s.captured, 1), glareNoticeNeedle) - before
}

func TestGlareCombatNotice(t *testing.T) {
	t.Run("a dazzled fighter is told once, on the first round", func(t *testing.T) {
		s := newGlareScene(t, 95)
		room := s.atk.GetRoom()
		require.Equal(t, messaging.SightFull, messaging.ParticipantSight(s.u1.Character, room),
			"precondition: a dazzled fighter sees faces")
		_, bright := messaging.ComfortDistance(s.u1.Character, room)
		require.Greater(t, bright, 0.0, "precondition: the room is past the dazzle ramp's start")
		require.Less(t, messaging.SightMult(s.u1.Character, room), 1.0, "precondition: the glare costs")

		assert.Equal(t, 1, s.round(), "the first round of the fight tells them")
		assert.Equal(t, 0, s.round(), "the second round of the same fight does not")
		assert.Equal(t, 0, countContaining(textsForUser(*s.captured, 1), blindNoticeNeedle),
			"the blind notice stays silent for a fighter who sees clearly")
	})

	t.Run("a later fight tells them again", func(t *testing.T) {
		s := newGlareScene(t, 95)
		require.Equal(t, 1, s.round())

		// The fight ends: the round closes with the player out of combat.
		s.u1.Character.EndAggro()
		require.False(t, s.u1.Character.IsInCombat())
		flushGlareCombatNotices()

		s.u1.Character.SetAggro(0, 100, characters.DefaultAttack)
		assert.Equal(t, 1, s.round(), "a new fight is told afresh")
	})

	t.Run("a fighter at full sight with no glare is never told", func(t *testing.T) {
		s := newGlareScene(t, 60)
		require.Equal(t, messaging.SightFull, messaging.ParticipantSight(s.u1.Character, s.atk.GetRoom()))
		require.Equal(t, 1.0, messaging.SightMult(s.u1.Character, s.atk.GetRoom()),
			"precondition: a comfortable room costs nothing")
		assert.Equal(t, 0, s.round())
		assert.Equal(t, 0, s.round())
	})

	t.Run("light verbosity suppresses it like the blind notice", func(t *testing.T) {
		s := newGlareScene(t, 95)
		orig := s.u1.CombatVerbosity
		t.Cleanup(func() { s.u1.CombatVerbosity = orig })
		s.u1.CombatVerbosity = "light"
		assert.Equal(t, 0, s.round())
		s.u1.CombatVerbosity = "medium"
		assert.Equal(t, 1, s.round(), "medium keeps the notice, and a suppressed one did not spend it")
	})
}
````

**Modify `internal/hooks/combat_verbosity.go`:**

````diff
@@ -506,3 +506,96 @@ func flushBlindCombatNotices() {
 		u.SendText(messaging.CategoryCombatBlindWarning, blindCombatNoticeText)
 	}
 }
+
+// ── The once-per-fight glare notice (#319) ─────────────────────────────
+
+// glareCombatNoticeText is the once-per-fight reminder sent to a player who
+// fights in light too bright for their eyes. A dazzled fighter sees every
+// face (SightFull), so the blind notice above never speaks for them, yet
+// the glare lowers their messaging.SightMult exactly as darkness does. It
+// names the cause and the cost without a number, as the blind notice does.
+const glareCombatNoticeText = "The glare is too bright, so your attacks and defense are weaker."
+
+// roundGlareCombatants is the per-round set of player userIds who took part
+// in combat this round at SightFull while glare cost them. Membership
+// answers "fought this round" exactly as roundBlindCombatants does, and for
+// the same reason it is a bool set. Game-loop goroutine only.
+var roundGlareCombatants = map[int]bool{}
+
+// glareToldThisFight is the set of players already told in the fight they
+// are in now. Unlike the blind notice, which speaks every round, the glare
+// notice speaks once per fight: a fighter who stays in the same light
+// learns nothing new from it the second time. flushGlareCombatNotices
+// drops a player from the set once they are out of combat, which is where
+// a fight ends, so a later fight tells them again. Game-loop goroutine only.
+var glareToldThisFight = map[int]bool{}
+
+// markGlareCombatant records a player-controlled combatant who sees
+// clearly (canSeeClearly, the caller's CanSeeSightImpairedOnly verdict, as
+// markBlindCombatant takes it) while glare costs them: the bright fraction
+// of messaging.ComfortDistance is above 0 and their SightMult in the room
+// is below 1.0. Both are asked because the line makes two claims: the
+// bright fraction says the light is too bright (SightMult alone would also
+// fall for a dark ramp), and SightMult says it costs (a Balance.DazzleCap
+// of 1.0 makes glare free, and then the line would be false).
+//
+// The blind and glare notices never mark the same player in one swing:
+// this one needs canSeeClearly and markBlindCombatant needs its negation.
+func markGlareCombatant(actor actions.Actor, canSeeClearly bool) {
+	if !actor.IsPlayer() || !canSeeClearly {
+		return
+	}
+	// The same typed-nil guard as markBlindCombatant: with no room there is
+	// no light to be dazzled by.
+	room := actor.GetRoom()
+	if room == nil {
+		return
+	}
+	char := actor.GetCharacter()
+	if _, bright := messaging.ComfortDistance(char, room); bright <= 0 {
+		return
+	}
+	if messaging.SightMult(char, room) >= 1.0 {
+		return
+	}
+	roundGlareCombatants[actor.GetUserId()] = true
+}
+
+// flushGlareCombatNotices sends the glare notice to every player marked
+// this round who has not yet been told in this fight, then clears the
+// round set and forgets every player no longer in combat. Called once at
+// the end of DoCombat each round, beside flushBlindCombatNotices.
+//
+// Same category and verbosity gate as the blind notice
+// (CategoryCombatBlindWarning, suppressible at Light only; see
+// flushBlindCombatNotices for the ruling). A suppressed notice is not
+// counted as told, so a player who turns verbosity up mid-fight reads it.
+//
+// "A fight ends" is read from Character.IsInCombat at the end of the round
+// (the combat phase is back to Idle). A player who leaves combat and enters
+// a new one between two flushes is still in combat at the second flush, so
+// that reads as the same fight and is not told again; the cost of that
+// window is one notice withheld, never one repeated.
+func flushGlareCombatNotices() {
+	for userId := range roundGlareCombatants {
+		delete(roundGlareCombatants, userId)
+		if glareToldThisFight[userId] {
+			continue
+		}
+		u := users.GetByUserId(userId)
+		if u == nil {
+			// Logged off mid-round; nothing to deliver.
+			continue
+		}
+		if u.GetCombatVerbosity().Suppresses(messaging.CategoryCombatBlindWarning) {
+			continue
+		}
+		u.SendText(messaging.CategoryCombatBlindWarning, glareCombatNoticeText)
+		glareToldThisFight[userId] = true
+	}
+	for userId := range glareToldThisFight {
+		if u := users.GetByUserId(userId); u == nil || !u.Character.IsInCombat() {
+			delete(glareToldThisFight, userId)
+		}
+	}
+}
````

- [ ] **Step 2: Packages and root guards**

```bash
gofmt -l internal/
go test ./internal/hooks -count=1
go test . -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: `ok`, including the existing blind-notice tests.

- [ ] **Step 3: Spec check and commit**

```bash
P="internal/hooks/NewRound_DoCombat.go internal/hooks/NewRound_DoCombat_unified.go internal/hooks/combat_glare_notice_test.go internal/hooks/combat_verbosity.go"
git add $P
git diff --cached d423ea6fd -- $P
git commit -m "feat(combat): a dazzled fighter is told once per fight that the glare costs them (#319)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: announcements judged against the moment before the change (#220)

Checkpoint `b86442e68`. Model: opus (the most files).

- [ ] **Step 1: The tests first**

Apply the new test files and test edits first: `internal/conditions/expires_next_test.go`, `internal/hooks/condition_end_snapshot_test.go`, the `condition_room_text_test.go` and `narration_testhelpers_test.go` edits, and `internal/usercommands/hood_test.go`. Against today's senders they fail: `TestLightEndLine_WatcherWhoCouldNotSeeByItIsNotTold` "Should be zero, but was 1"; `TestLightEndLine_ShapesWatcherDoesNotReadABareHolderName` reads `The wick held by Aliceia gutters out.`; `TestDarknessEndLine_WatcherBlindInTheDarknessIsNotTold` 1 line, want 0; `TestDarknessEndLine_WatcherWhoSawShapesInTheDarknessIsTold` reads `The gloom around Aliceia lifts.`; `TestDarknessEndLine_MobHolderIsJudgedBeforeItLifts` 1 line, want 0; `TestHood_ObserverWhoCouldNotSeeBeforeIsNotTold` the blind observer gets the hood line; `TestEndLineSnapshots_DoNotOutliveThePrune` and `TestExpiresOnNextTriggerAgreesWithTrigger` do not build (the store and predicate do not exist yet). `TestLightEndLine_HolderWhoMovedIsJudgedByTheNewRoom` passes before and after: it guards the moved-room fallback. Then apply the rest.

**Modify `bauble_finder_view_guard_test.go`:**

````diff
@@ -96,7 +96,7 @@ var finderViewSelectors = map[string]bool{
 // method (combat.AttackResult SendToSourceRoom, SendToTargetRoom).
 var beyondReaderCalls = map[string]bool{
 	"SendTextCommunication": true, "SendTextVisual": true, "SendTextVisualHidingNames": true,
-	"SendTextVisualAsLit": true, "SendTextVisualAsLitHidingNames": true, "SendTextVisualWithAudio": true, "SendTextVisualToSnapshot": true,
+	"SendTextVisualWithAudio": true, "SendTextVisualToSnapshot": true,
 	"SendTextHidingNames": true, "SendCommunicationHidingNames": true, "SendVisualCommunicationHidingNames": true,
 	"SendTextToExits": true, "SendTrio": true, "SendCounterTrio": true,
 	"SendMessage": true, "Command": true, "merchantSay": true,
````

**Modify `internal/conditions/conditions.go`:**

````diff
@@ -526,6 +526,32 @@ func (bs *Conditions) Trigger(conditionId ...int) (triggeredConditions []*Condit
 	return triggeredConditions
 }
 
+// ExpiresOnNextTrigger reports whether the next Trigger will leave this record
+// expired. It is Trigger's own arithmetic asked one round early, and must
+// change whenever Trigger does: a record with no spec or no interval is never
+// ticked, the round counter has to land on the interval, a non-stacking
+// record runs out on its last trigger, and a stacking one when tickStacks
+// drops its last stack. The round ticks use it to snapshot the room just
+// before a light or darkness runs out (#220). An unlimited record never
+// expires here.
+func (b *Condition) ExpiresOnNextTrigger(spec *ConditionSpec) bool {
+	if spec == nil || spec.RoundInterval < 1 || b.TriggersLeft <= 0 {
+		return false
+	}
+	if (b.RoundCounter+1)%spec.RoundInterval != 0 {
+		return false
+	}
+	if spec.IsStacking() {
+		for _, s := range b.Stacks {
+			if s.RoundsLeft > 1 {
+				return false
+			}
+		}
+		return true
+	}
+	return b.TriggersLeft == 1
+}
+
 func (bs *Conditions) GetConditions(conditionId ...int) []*Condition {
 	retConditions := []*Condition{}
 	for _, b := range bs.List {
````

**Modify `internal/conditions/conditionspec.go`:**

````diff
@@ -261,8 +261,9 @@ func (b *ConditionSpec) IsDarknessSource() bool {
 }
 
 // AnyDarknessSource reports whether any of the condition ids names a
-// darkness source: an item whose worn conditions darken its room announces
-// itself as lit (lighting plan 5d, ruling D6). Unknown ids are skipped.
+// darkness source: an item whose worn conditions darken its room has its
+// equip line judged against the room before it went on (lighting plan 5d,
+// ruling D6 as amended 2026-10-05). Unknown ids are skipped.
 func AnyDarknessSource(conditionIds []int) bool {
 	for _, id := range conditionIds {
 		if spec := GetConditionSpec(id); spec != nil && spec.IsDarknessSource() {
````

**Modify `internal/conditions/context.md`:**

````diff
@@ -139,12 +139,12 @@ literal for an item (132 Umbral Dark), `magnitude` for a spell (131 Chrysalis
 Pall). It is a light record with darkening polarity: it shares `LightTrim`,
 `LightOutput` and `ResetLight`, and `LightMax` / `LightNow` read whichever of
 the two kinds the spec declares. It is NOT a light: `IsLightSource()` stays
-light-only, so `LightSources`, `EmitsLight`, `hood` and the as-lit end line
-exclude it by construction. Ask `ConditionSpec.IsDarknessSource()`, walk
+light-only, so `LightSources`, `EmitsLight` and `hood` exclude it by
+construction. Ask `ConditionSpec.IsDarknessSource()`, walk
 `(*Conditions).DarknessSources()`, or `(*Conditions).LightAndDarknessSources()`
 for both kinds in one held order (the trim's walk). `AnyDarknessSource(ids)`
-reports whether any condition id names a darkness (the equip line's as-lit
-test). `Effect(EffectDarknessStrength)` returns 0. `validateEffects` also
+reports whether any condition id names a darkness (the equip line's test for
+taking a `rooms.VisualSnapshot` first). `Effect(EffectDarknessStrength)` returns 0. `validateEffects` also
 refuses a literal `darkness_strength` of 0 or less, a spec declaring both
 kinds, and a `stacking` darkness; `AddConditionMagnitude` resets either kind.
 
@@ -1012,6 +1012,13 @@ func LoadDataFiles() {
 - character.Conditions.Prune()                      // Cleanup expired conditions
 ```
 
+`(*Condition).ExpiresOnNextTrigger(spec)` asks Trigger's own arithmetic one
+round early: true when the next `Trigger` leaves the record expired (its last
+trigger lands, or a stacking record's last stack drops). It must change
+whenever `Trigger` does; `TestExpiresOnNextTriggerAgreesWithTrigger` pins the
+two together. The hooks round ticks use it to snapshot a room just before a
+light or darkness runs out, for its end line (#220).
+
 ### Combat System Integration
 ```go
 // Combat checks condition flags for behavior modification
````

**Create `internal/conditions/expires_next_test.go`:**

````go
package conditions

import "testing"

// ExpiresOnNextTrigger must agree with Trigger itself on every round of a
// record's life: the round ticks use it to snapshot a room just before a
// light or darkness runs out (#220), so a disagreement either misses the
// snapshot or takes one for a record that lives on.
func TestExpiresOnNextTriggerAgreesWithTrigger(t *testing.T) {
	timed := func(id, interval, count int) *ConditionSpec {
		return &ConditionSpec{ConditionId: id, Name: "Timed", RoundInterval: interval, TriggerCount: count}
	}
	withSpecs(t,
		timed(931, 1, 3),
		timed(932, 2, 2),
		timed(933, 3, 1),
		&ConditionSpec{ConditionId: 934, Name: "Flag only"},
		stackingSpec(),
	)

	cases := []struct {
		name string
		add  func(bs *Conditions)
		id   int
	}{
		{"every round, three triggers", func(bs *Conditions) { bs.AddCondition(931, false) }, 931},
		{"every second round, two triggers", func(bs *Conditions) { bs.AddCondition(932, false) }, 932},
		{"every third round, one trigger", func(bs *Conditions) { bs.AddCondition(933, false) }, 933},
		{"unlimited never expires", func(bs *Conditions) { bs.AddCondition(931, true) }, 931},
		{"a flag record never ticks", func(bs *Conditions) { bs.AddCondition(934, false) }, 934},
		{"stacking, outlived by its longest stack", func(bs *Conditions) {
			bs.AddConditionMagnitude(930, 2, -2)
			bs.AddConditionMagnitude(930, 4, -3)
		}, 930},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bs := New()
			c.add(&bs)
			rec := bs.List[0]
			spec := GetConditionSpec(c.id)
			sawExpiry := false
			for round := 1; round <= 8; round++ {
				predicted := rec.ExpiresOnNextTrigger(spec)
				wasLive := !rec.Expired()
				bs.Trigger()
				expiredNow := wasLive && rec.Expired()
				if predicted != expiredNow {
					t.Fatalf("round %d: ExpiresOnNextTrigger said %v, Trigger expired it: %v", round, predicted, expiredNow)
				}
				sawExpiry = sawExpiry || expiredNow
			}
			wantExpiry := c.id != 934 && c.name != "unlimited never expires"
			if sawExpiry != wantExpiry {
				t.Fatalf("expired within eight rounds = %v, want %v: the case does not test what it says", sawExpiry, wantExpiry)
			}
		})
	}

	t.Run("nil spec", func(t *testing.T) {
		if (&Condition{TriggersLeft: 1}).ExpiresOnNextTrigger(nil) {
			t.Fatal("a record with no spec is never ticked by Trigger")
		}
	})
}
````

**Modify `internal/hooks/NewRound_MobRoundTick.go`:**

````diff
@@ -216,6 +216,10 @@ func tickMobCharmDuration(mob *mobs.Mob) {
 
 // tickMobConditions — current inline block at lines 124–160.
 func tickMobConditions(mob *mobs.Mob, mobInstanceId int) {
+	// #220: a light or darkness about to run out on this Trigger has its
+	// room snapshotted first, for its end line at the prune.
+	keepEndLineSnapshots(&mob.Character, rooms.LoadRoom(mob.Character.RoomId))
+
 	if triggeredConditions := mob.Character.Conditions.Trigger(); len(triggeredConditions) > 0 {
 		triggeredConditionIds := []int{}
 		for _, condition := range triggeredConditions {
````

**Modify `internal/hooks/NewRound_UserRoundTick.go`:**

````diff
@@ -239,6 +239,10 @@ func UserRoundTick(e events.Event) events.ListenerReturn {
 					user.Character.Charmed.RoundsRemaining--
 				}
 
+				// #220: a light or darkness about to run out on this Trigger
+				// has its room snapshotted first, for its end line at the prune.
+				keepEndLineSnapshots(user.Character, room)
+
 				if triggeredConditions := user.Character.Conditions.Trigger(); len(triggeredConditions) > 0 {
 
 					//
````

**Modify `internal/hooks/NewTurn_PruneConditions.go`:**

````diff
@@ -50,7 +50,7 @@ func PruneConditions(e events.Event) events.ListenerReturn {
 							}
 							if roles.Observer != "" {
 								if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
-									sendConditionEndRoomText(r, endConditionSpec, roles.Observer,
+									sendConditionEndRoomText(r, takeEndLineSnapshot(conditionInfo, r.RoomId), roles.Observer,
 										[]string{user.Character.GetCharacterName(false)}, user.UserId)
 								}
 							}
@@ -109,7 +109,7 @@ func PruneConditions(e events.Event) events.ListenerReturn {
 						holderName, mob.Character.GetCharacterName(false))
 					if roles.Observer != "" {
 						if r := rooms.LoadRoom(mob.Character.RoomId); r != nil {
-							sendConditionEndRoomText(r, endConditionSpec, roles.Observer,
+							sendConditionEndRoomText(r, takeEndLineSnapshot(conditionInfo, r.RoomId), roles.Observer,
 								[]string{mob.Character.GetCharacterName(false)})
 						}
 					}
@@ -121,25 +121,30 @@ func PruneConditions(e events.Event) events.ListenerReturn {
 
 	}
 
+	// Every snapshot this prune did not narrate is stale: its record was
+	// revived, pruned by some other path, or silent. See endLineSnapshots.
+	clear(endLineSnapshots)
+
 	return events.Continue
 
 }
 
-// sendConditionEndRoomText sends a condition's end room line on the visual channel. A
-// light condition's line is judged as if the room were still lit, because its light
-// went out when the condition expired, a round before this prune: see
-// Room.SendTextVisualAsLit. The judgement is per spec, so a hooded or
-// trimmed-off light's end line is judged as lit too; accepted for now, as the
-// retired lightsource flag was spec-level as well. Every other end line is
-// judged by the room as it is.
+// sendConditionEndRoomText sends a condition's end room line on the visual
+// channel. With a snapshot (a light or darkness record that ran out on the
+// round tick, from takeEndLineSnapshot) the line is judged against the room
+// as it was just before the record ran out: the owner rule of 2026-10-05, a
+// line announcing a change is judged by the state before it. So a light's end
+// line reaches the watchers who saw by it and not one who never could, and a
+// darkness's does not reach a watcher who was blind in it. Every other end
+// line is judged by the room as it is.
 //
 // names is the holder's PLAIN name. An end line may author a bare
 // {actee_plain} (shipped conditions 1 and 9 both do), which tag-based
 // Anonymize cannot see, so the name must be handed to HideNames explicitly.
-// This is the End-phase twin of the start and trigger senders.
-func sendConditionEndRoomText(r *rooms.Room, spec *conditions.ConditionSpec, msg string, names []string, skip ...int) {
-	if spec.IsLightSource() {
-		r.SendTextVisualAsLitHidingNames(messaging.CategoryConditionExpire, msg, names, skip...)
+// This is the End-phase twin of sendConditionStartRoomText.
+func sendConditionEndRoomText(r *rooms.Room, snap rooms.VisualSnapshot, msg string, names []string, skip ...int) {
+	if snap != nil {
+		r.SendTextVisualToSnapshot(snap, messaging.CategoryConditionExpire, msg, names, skip...)
 		return
 	}
 	r.SendTextVisualHidingNames(messaging.CategoryConditionExpire, msg, names, skip...)
````

**Create `internal/hooks/condition_end_snapshot.go`:**

````go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// endLineSnapshot is the holder's room as everyone in it could see it just
// before a light or darkness record ran out, kept for that record's end line
// (#220).
type endLineSnapshot struct {
	roomId int
	snap   rooms.VisualSnapshot
}

// endLineSnapshots holds, for each light or darkness record that ran out on
// this round's tick, the room it ran out in as it looked a moment before.
//
// A record expires inside Conditions.Trigger on the round tick, and LightNow
// stops counting it at once; its end line goes out at the next turn's prune.
// Owner rule (2026-10-05): a line announcing a change to the room's light is
// judged against the state BEFORE the change. Judged at the prune, a light's
// end line met a room already dark and a darkness's a room already lit.
//
// Keyed by the record's pointer, which Prune hands back unchanged. Every
// entry lives at most until the next PruneConditions: the prune takes the
// ones it narrates and clears the rest (a record revived before it could be
// pruned, or one a death cascade pruned first), so nothing piles up. Written
// by the round ticks and read by the prune, both on the game-loop goroutine.
var endLineSnapshots = map[*conditions.Condition]endLineSnapshot{}

// keepEndLineSnapshots snapshots room for every light or darkness record c
// holds that will run out on the Trigger about to run. Call it just before
// c.Conditions.Trigger(), with the room c is in. The room is walked once,
// and only when such a record exists.
func keepEndLineSnapshots(c *characters.Character, room *rooms.Room) {
	if room == nil {
		return
	}
	var snap rooms.VisualSnapshot
	for _, rec := range c.Conditions.LightAndDarknessSources() {
		if !rec.ExpiresOnNextTrigger(conditions.GetConditionSpec(rec.ConditionId)) {
			continue
		}
		if snap == nil {
			snap = room.VisualSnapshot()
		}
		endLineSnapshots[rec] = endLineSnapshot{roomId: room.RoomId, snap: snap}
	}
}

// takeEndLineSnapshot returns and forgets the snapshot kept for rec, for an
// end line sent to the room endRoomId. It is nil when none was kept (a record
// removed some other way than running out) and when the holder has moved
// since: the snapshot belongs to the room the record ran out in, and the
// prune sends to the room the holder is in now, so a line sent against a
// snapshot of another room would reach nobody there. Both fall back to the
// room as it is, which is the right judgement when no light changed in it.
func takeEndLineSnapshot(rec *conditions.Condition, endRoomId int) rooms.VisualSnapshot {
	kept, ok := endLineSnapshots[rec]
	if !ok {
		return nil
	}
	delete(endLineSnapshots, rec)
	if kept.roomId != endRoomId {
		return nil
	}
	return kept.snap
}
````

**Create `internal/hooks/condition_end_snapshot_test.go`:**

````go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// File: condition_end_snapshot_test.go
//
// #220. A light or darkness record runs out inside Conditions.Trigger on the
// round tick, and its end line goes out at the next turn's prune. The owner
// rule of 2026-10-05: a line announcing a change is judged against the state
// BEFORE it resolves. So the round tick snapshots the room just before the
// record expires and the prune sends against that snapshot. Before this, a
// light's end line was judged as if the room were lit (which told a watcher
// who never could see by it) and a darkness's by the room after it lifted
// (which told a watcher who had been blind in it).

// expireOnNextTick sets a held record one trigger from the end, at the last
// round of its interval, so the next round tick expires it through Trigger
// itself, the path that takes the snapshot.
func expireOnNextTick(t *testing.T, list []*conditions.Condition, conditionId int) {
	t.Helper()
	spec := conditions.GetConditionSpec(conditionId)
	require.NotNil(t, spec)
	for _, b := range list {
		if b.ConditionId == conditionId {
			b.TriggersLeft = 1
			b.RoundCounter = spec.RoundInterval - 1
			require.True(t, b.ExpiresOnNextTrigger(spec), "fixture: the record must run out on the next tick")
			return
		}
	}
	t.Fatalf("condition %d not found to expire", conditionId)
}

// A faint light no normal eye can see by: the watcher was blind before it
// went out, so its end line tells them nothing. Judged as lit, it did.
func TestLightEndLine_WatcherWhoCouldNotSeeByItIsNotTold(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(wickConditionId, false))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(users.GetByUserId(2).Character, room),
		"fixture: the wick must be too faint for the watcher to see by")
	expireOnNextTick(t, holder.Character.Conditions.List, wickConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Zero(t, countContaining(drainPlain(2), "gutters out"),
		"a watcher blind before the wick went out was told it went out")
}

// The light end line has a shapes tier now: a heat-eyed watcher who made out
// the holder only as a shape by a faint light reads the line with the bare
// name hidden. Judged as lit, every reader was at faces and read the name.
// This replaces TestConditionEndRoomText_LightPathHasNoShapesTier, which
// pinned the old as-lit judgement.
func TestLightEndLine_ShapesWatcherDoesNotReadABareHolderName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	holder := users.GetByUserId(1)
	watcher := users.GetByUserId(2)
	require.True(t, holder.Character.Conditions.AddCondition(wickConditionId, false))
	require.True(t, watcher.Character.Conditions.AddCondition(heatEyesConditionId, true))
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(watcher.Character, room),
		"fixture: the watcher must make out shapes only by the wick")
	expireOnNextTick(t, holder.Character.Conditions.List, wickConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	PruneConditions(events.NewTurn{TurnNumber: 1})

	lines := drainPlain(2)
	require.Equal(t, 1, countContaining(lines, "gutters out"),
		"the shapes watcher must still receive the line, or this test proves nothing: %v", lines)
	assert.Zero(t, countContaining(lines, "Aliceia"),
		"a shapes-only watcher read the holder's bare name: %v", lines)
}

// A darkness's end line is judged by the darker room it ended in: a watcher
// blind in that darkness is not told it lifted, though the room is lit once
// it has. Judged by the room after, they were.
func TestDarknessEndLine_WatcherBlindInTheDarknessIsNotTold(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(50)
	holder := users.GetByUserId(1)
	watcher := users.GetByUserId(2)
	require.True(t, holder.Character.Conditions.AddCondition(gloomConditionId, false))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(watcher.Character, room),
		"fixture: the gloom must blind the watcher")
	expireOnNextTick(t, holder.Character.Conditions.List, gloomConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	require.NotEqual(t, messaging.SightNone, messaging.ParticipantSight(watcher.Character, room),
		"fixture: the room must be lit again once the gloom has run out")
	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Zero(t, countContaining(drainPlain(2), "gloom around"),
		"a watcher blind in the gloom was told it lifted")
}

// The other side of the same judgement: a watcher whose heat sight read
// shapes inside the gloom is told it lifted, with the holder unnamed.
func TestDarknessEndLine_WatcherWhoSawShapesInTheDarknessIsTold(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(80)
	holder := users.GetByUserId(1)
	watcher := users.GetByUserId(2)
	require.True(t, holder.Character.Conditions.AddCondition(gloomConditionId, false))
	require.True(t, watcher.Character.Conditions.AddCondition(heatEyesConditionId, true))
	require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(watcher.Character, room),
		"fixture: heat sight must read shapes inside the gloom")
	expireOnNextTick(t, holder.Character.Conditions.List, gloomConditionId)
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	PruneConditions(events.NewTurn{TurnNumber: 1})
	lines := drainPlain(2)
	assert.Equal(t, 1, countContaining(lines, "gloom around"), "%v", lines)
	assert.Zero(t, countContaining(lines, "Aliceia"), "judged at shapes, the holder is not named: %v", lines)
}

// A mob's darkness is snapshotted on the mob round tick the same way.
func TestDarknessEndLine_MobHolderIsJudgedBeforeItLifts(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(50)
	mob := mobs.GetInstance(100)
	require.Equal(t, 1, mob.Character.RoomId, "fixture: the mob must share the watcher's room")
	require.True(t, mob.Character.Conditions.AddCondition(gloomConditionId, false))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(users.GetByUserId(2).Character, room))
	expireOnNextTick(t, mob.Character.Conditions.List, gloomConditionId)
	drainPlain(2)

	tickMobConditions(mob, 100)
	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Zero(t, countContaining(drainPlain(2), "gloom around"),
		"a watcher blind in the mob's gloom was told it lifted")
}

// The snapshot belongs to the room the light went out in. A holder who walks
// into another room before the prune has their end line judged by that room
// as it is: nobody there was in the snapshot, and sending against it would
// silence the line for everyone.
func TestLightEndLine_HolderWhoMovedIsJudgedByTheNewRoom(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room1, room2 := rooms.LoadRoom(1), rooms.LoadRoom(2)
	room2.Lamp = rooms.LampPtr(60)
	holder, watcher := users.GetByUserId(1), users.GetByUserId(2)
	room1.RemovePlayer(2)
	watcher.Character.RoomId = 2
	room2.AddPlayer(2)
	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false))
	expireOnNextTick(t, holder.Character.Conditions.List, lanternConditionId)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	room1.RemovePlayer(1)
	holder.Character.RoomId = 2
	rooms.MarkRoomOccupancy(2, room2.AddPlayer(1), 0)
	drainPlain(2)

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia's light gutters out."),
		"a watcher in the holder's new, lit room must read the line")
}

// No snapshot outlives the prune that follows its round: one the prune does
// not consume (here, a record revived before it could be pruned) is dropped,
// so the map cannot grow.
func TestEndLineSnapshots_DoNotOutliveThePrune(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false))
	expireOnNextTick(t, holder.Character.Conditions.List, lanternConditionId)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	require.Len(t, endLineSnapshots, 1, "the tick must keep a snapshot for the expiring lantern")
	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false), "revive it before the prune")

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Empty(t, endLineSnapshots)
}
````

**Modify `internal/hooks/condition_room_text_test.go`:**

````diff
@@ -246,7 +246,9 @@ func TestMobConditionTriggerRoomText(t *testing.T) {
 // sent later, at the turn's prune. So a plain visual send judged sight in a
 // room that was already dark, and silenced the line for exactly the people
 // who had been seeing by that light. Found by the Task 2 review against
-// shipped condition 1, Illumination.
+// shipped condition 1, Illumination. Since #220 the round tick snapshots the
+// room just before the light runs out and the prune sends against that, so
+// these tests run the tick rather than expiring the record by hand.
 
 func TestConditionEndRoomText_LightConditionEndIsSeenByItsOwnLight_Player(t *testing.T) {
 	cleanup := seedAllRegistries()
@@ -258,10 +260,11 @@ func TestConditionEndRoomText_LightConditionEndIsSeenByItsOwnLight_Player(t *tes
 	holder := users.GetByUserId(1)
 	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false))
 	require.Greater(t, room.LightLevel(), 0, "the lantern must light the cave, or this test proves nothing")
-	expire(t, holder.Character.Conditions.List, lanternConditionId)
-	require.Equal(t, 0, room.LightLevel(), "the light is already out once the condition expires, before any prune")
+	expireOnNextTick(t, holder.Character.Conditions.List, lanternConditionId)
 	drainPlain(2)
 
+	UserRoundTick(events.NewRound{RoundNumber: 1})
+	require.Equal(t, 0, room.LightLevel(), "the light is already out once the condition expires, before any prune")
 	PruneConditions(events.NewTurn{TurnNumber: 1})
 	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia's light gutters out."))
 }
@@ -278,9 +281,10 @@ func TestConditionEndRoomText_LightConditionEnd_SleeperStillGetsNothing(t *testi
 	holder := users.GetByUserId(1)
 	require.True(t, holder.Character.Conditions.AddCondition(lanternConditionId, false))
 	require.True(t, users.GetByUserId(2).Character.Conditions.AddCondition(dozeConditionId, true))
-	expire(t, holder.Character.Conditions.List, lanternConditionId)
+	expireOnNextTick(t, holder.Character.Conditions.List, lanternConditionId)
 	drainPlain(2)
 
+	UserRoundTick(events.NewRound{RoundNumber: 1})
 	PruneConditions(events.NewTurn{TurnNumber: 1})
 	assert.Equal(t, 0, countContaining(drainPlain(2), "light gutters out"))
 }
@@ -295,9 +299,10 @@ func TestConditionEndRoomText_LightConditionEndIsSeenByItsOwnLight_Mob(t *testin
 	mob := mobs.GetInstance(100)
 	require.True(t, mob.Character.Conditions.AddCondition(lanternConditionId, false))
 	require.Greater(t, room.LightLevel(), 0, "the lantern must light the cave, or this test proves nothing")
-	expire(t, mob.Character.Conditions.List, lanternConditionId)
+	expireOnNextTick(t, mob.Character.Conditions.List, lanternConditionId)
 	drainPlain(2)
 
+	tickMobConditions(mob, 100)
 	PruneConditions(events.NewTurn{TurnNumber: 1})
 	assert.Equal(t, 1, countContaining(drainPlain(2), "Skeleton's light gutters out."))
 }
@@ -328,17 +333,14 @@ func TestConditionEndRoomText_InfraredObserverDoesNotReadABareHolderName(t *test
 		"an infrared-only observer read the holder's bare name: %v", lines)
 }
 
-// TestConditionEndRoomText_LightPathHasNoShapesTier pins the structural reason
-// shipped condition 1 ("The glow surrounding {actee_plain} fades away.") does
-// not leak despite authoring a bare name: SendTextVisualAsLit judges sight
-// against litRoom{}, and ParticipantSight returns SightShapes only for an
-// UNLIT room, so no reader of that path is ever at the one tier HideNames acts
-// on. An infrared observer therefore reads the real name here, correctly, and
-// a blind or sleeping one reads nothing.
-//
-// If this test ever fails, SendTextVisualAsLit has grown a shapes tier and
-// every light condition authoring a bare _plain token became a live leak.
-func TestConditionEndRoomText_LightPathHasNoShapesTier(t *testing.T) {
+// TestConditionEndRoomText_LightEndNamesTheHolderToWhoSawThemByIt is the
+// faces side of shipped condition 1 ("The glow surrounding {actee_plain}
+// fades away."): a heat-eyed observer who saw the holder clearly by the ember
+// itself reads the name, because the line is judged against the room just
+// before the ember went out. It replaces LightPathHasNoShapesTier, which
+// pinned the old as-lit judgement; the shapes side, where the bare name is
+// hidden, is TestLightEndLine_ShapesWatcherDoesNotReadABareHolderName.
+func TestConditionEndRoomText_LightEndNamesTheHolderToWhoSawThemByIt(t *testing.T) {
 	cleanup := seedAllRegistries()
 	defer cleanup()
 	restore := seedNarrationConditions()
@@ -347,14 +349,15 @@ func TestConditionEndRoomText_LightPathHasNoShapesTier(t *testing.T) {
 	holder := users.GetByUserId(1)
 	require.True(t, holder.Character.Conditions.AddCondition(emberConditionId, false))
 	require.True(t, users.GetByUserId(2).Character.Conditions.AddCondition(heatEyesConditionId, true))
-	expire(t, holder.Character.Conditions.List, emberConditionId)
+	expireOnNextTick(t, holder.Character.Conditions.List, emberConditionId)
 	drainPlain(2)
 
+	UserRoundTick(events.NewRound{RoundNumber: 1})
 	PruneConditions(events.NewTurn{TurnNumber: 1})
 
 	lines := drainPlain(2)
 	require.Equal(t, 1, countContaining(lines, "fades away"),
 		"the light line must reach the observer, or this test proves nothing: %v", lines)
 	assert.Equal(t, 1, countContaining(lines, "Aliceia"),
-		"the lit path has no shapes tier, so the name is expected here: %v", lines)
+		"the observer saw the holder clearly by the ember, so the name is expected: %v", lines)
 }
````

**Modify `internal/hooks/context.md`:**

````diff
@@ -1086,10 +1086,23 @@ is computed once `TickScale` reaches `hooks.setTickAmountAtApply` in
 `Condition_ApplyConditions`, where the record is guaranteed to exist; see
 "The damaging condition tick" below.
 
-`sendConditionEndRoomText` (`NewTurn_PruneConditions.go`) judges a light's end
-line as lit by `spec.IsLightSource()`; the judgement is per spec, so a hooded
-or trimmed-off light's end line is judged as lit too (accepted, as the retired
-flag was spec-level as well).
+`sendConditionEndRoomText(r, snap, msg, names, skip...)`
+(`NewTurn_PruneConditions.go`, #220) judges a light's or a darkness's end
+line against the room as it was just before the record ran out. The record
+expires inside `Conditions.Trigger` on the round tick, and its light stops
+counting at once, but the line goes out at the next turn's prune; judged by
+the room then, a light's line met a room already dark and a darkness's a
+room already lit. So both round ticks call `keepEndLineSnapshots(c, room)`
+just before `Trigger`: for every held light or darkness record whose
+`ExpiresOnNextTrigger(spec)` is true it stores `room.VisualSnapshot()` in
+`endLineSnapshots`, keyed by the record's pointer
+(`condition_end_snapshot.go`). The prune takes it with
+`takeEndLineSnapshot(rec, roomId)` and sends with
+`Room.SendTextVisualToSnapshot`; with none kept (a record removed some other
+way), or when the holder has moved rooms between the tick and the prune (the
+snapshot belongs to the room the record ran out in), the line is judged by
+the room as it is. The prune clears whatever it did not take, so no entry
+outlives the turn after its round.
 
 `sendConditionStartRoomText` (`Condition_ApplyConditions.go`, lighting plan
 5d, ruling D6 as amended by the owner on 2026-10-05) is its counterpart for
````

**Modify `internal/hooks/narration_testhelpers_test.go`:**

````diff
@@ -45,6 +45,8 @@ const (
 	dozeConditionId      = 7007 // puts the bearer to sleep; RoundInterval 0, so it never ticks
 	shadeConditionId     = 7008 // end_observer with a BARE {actee_plain}, mirrors shipped condition 9
 	emberConditionId     = 7009 // a light source whose end_observer has a BARE {actee_plain}, mirrors shipped condition 1
+	gloomConditionId     = 7010 // a darkness source with end_observer
+	wickConditionId      = 7011 // a light too faint for normal eyes, end_observer with a BARE {actee_plain}
 )
 
 // seedNarrationConditions installs the narration test conditions and returns the restore
@@ -77,6 +79,12 @@ func seedNarrationConditions() func() {
 		emberConditionId: {ConditionId: emberConditionId, Name: "Test Ember", RoundInterval: 5, TriggerCount: 3,
 			EndRoomText: "The glow surrounding {actee_plain} fades away.",
 			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 50}}},
+		gloomConditionId: {ConditionId: gloomConditionId, Name: "Test Gloom", RoundInterval: 5, TriggerCount: 3,
+			EndRoomText: "The gloom around {actee} lifts.",
+			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 90}}},
+		wickConditionId: {ConditionId: wickConditionId, Name: "Test Wick", RoundInterval: 5, TriggerCount: 3,
+			EndRoomText: "The wick held by {actee_plain} gutters out.",
+			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 10}}},
 	})
 }
 
````

**Modify `internal/rooms/context.md`:**

````diff
@@ -15,11 +15,6 @@ The `internal/rooms` package is the core world management system for GoMud, hand
   sight-gated. `SendTextVisual` gates each recipient by
   `messaging.CanSeeClearly` / `CanSeeShapes` and anonymizes for infrared-only
   observers. `SendTextVisualWithAudio` gives the unsighted an audio variant.
-  `SendTextVisualAsLit` judges sight as if the room were lit, and exists for one
-  case: an event that is itself a light whose light is already gone when the
-  line is sent, such as a light condition's end text (the light stops counting when
-  the condition expires, a round before the prune sends the line). Blinded and
-  sleeping observers still get nothing from it.
   `VisualSnapshot()` returns a `VisualSnapshot` (user id to
   `messaging.SightDecision`) of what every player in the room can see now;
   `SendTextVisualToSnapshot(snap, cat, txt, names, excludeUserIds...)`
@@ -28,8 +23,12 @@ The `internal/rooms` package is the core world management system for GoMud, hand
   rule (2026-10-05): a line announcing a change to the room's light lands
   with the state everyone was in BEFORE the change, so take the snapshot
   first, make the change, then send. Lighting plan 5d uses it for a
-  darkness's start line and its equip lines. All the visual senders share
-  one per-recipient body, `deliverVisual`.
+  darkness's start line and its equip lines; #220 added `hood` and the end
+  line of a light or darkness that runs out (the hooks round ticks snapshot
+  the room just before the record expires, and the prune sends against it).
+  The two as-lit senders this replaced, which judged every reader against a
+  stand-in lit room, are deleted. All the
+  visual senders share one per-recipient body, `deliverVisual`.
   `SendTextVisualHidingNames` is `SendTextVisual` for a line that names an
   event's parties: a shapes-only observer reads each name as "a figure". It is
   the observer half of `messaging.SendTrio`; `ParticipantSight(userId)` is the
````

**Modify `internal/rooms/hiding_senders_test.go`:**

````diff
@@ -113,10 +113,6 @@ func TestVisualSendersStayOffTheDeafenFilter(t *testing.T) {
 		"SendTextVisualHidingNames": func() {
 			r.SendTextVisualHidingNames(messaging.CategoryEmote, "Aliceia waves.", []string{"Aliceia"}, 7411)
 		},
-		"SendTextVisualAsLit": func() { r.SendTextVisualAsLit(messaging.CategoryEmote, "Aliceia waves.", 7411) },
-		"SendTextVisualAsLitHidingNames": func() {
-			r.SendTextVisualAsLitHidingNames(messaging.CategoryEmote, "Aliceia waves.", []string{"Aliceia"}, 7411)
-		},
 		"SendTextVisualToSnapshot": func() {
 			r.SendTextVisualToSnapshot(r.VisualSnapshot(), messaging.CategoryEmote, "Aliceia waves.", []string{"Aliceia"}, 7411)
 		},
````

**Modify `internal/rooms/rooms.go`:**

````diff
@@ -270,14 +270,14 @@ func (r *Room) SendText(cat messaging.Category, txt string, excludeUserIds ...in
 // is computed via messaging.CanSeeClearly / CanSeeShapes; infrared
 // observers get an anonymized render.
 func (r *Room) SendTextVisual(cat messaging.Category, txt string, excludeUserIds ...int) {
-	r.sendTextVisualJudgedBy(r, cat, txt, nil, false, excludeUserIds...)
+	r.sendTextVisualJudgedBy(cat, txt, nil, false, excludeUserIds...)
 }
 
 // SendTextVisualHidingNames is SendTextVisual for a line that names the
 // parties to an event: an observer who makes out shapes only reads each of
 // names as "a figure". It is the observer half of messaging.SendTrio.
 func (r *Room) SendTextVisualHidingNames(cat messaging.Category, txt string, names []string, excludeUserIds ...int) {
-	r.sendTextVisualJudgedBy(r, cat, txt, names, false, excludeUserIds...)
+	r.sendTextVisualJudgedBy(cat, txt, names, false, excludeUserIds...)
 }
 
 // SendVisualCommunicationHidingNames is SendTextVisualHidingNames for a
@@ -286,7 +286,7 @@ func (r *Room) SendTextVisualHidingNames(cat messaging.Category, txt string, nam
 // so the Deafened moderation filter spares a deafened player. Only the shared
 // bodies in internal/actions call it (speech_wrapper_guard_test.go).
 func (r *Room) SendVisualCommunicationHidingNames(cat messaging.Category, txt string, names []string, excludeUserIds ...int) {
-	r.sendTextVisualJudgedBy(r, cat, txt, names, true, excludeUserIds...)
+	r.sendTextVisualJudgedBy(cat, txt, names, true, excludeUserIds...)
 }
 
 // SendTextHidingNames is SendText for an authored line that names who made
@@ -352,49 +352,15 @@ func (r *Room) ParticipantSight(userId int) messaging.SightDecision {
 	return messaging.ParticipantSight(u.Character, r)
 }
 
-// SendTextVisualAsLit delivers a sight-gated message judged as if the room
-// were lit. Use it ONLY for an event that is itself a light and whose light is
-// already gone when the line is sent: a light condition's end text ("The glow
-// surrounding X fades away"). Conditions.HasFlag stops counting a light the moment
-// its condition expires, on the round tick, but the end text goes out at the turn's
-// prune, so SendTextVisual judged a room that was already dark and silenced
-// the line for everyone who had been seeing by that light.
-//
-// It is still a sight line: blinded and sleeping observers get nothing.
-func (r *Room) SendTextVisualAsLit(cat messaging.Category, txt string, excludeUserIds ...int) {
-	r.sendTextVisualJudgedBy(litRoom{}, cat, txt, nil, false, excludeUserIds...)
-}
-
-// SendTextVisualAsLitHidingNames is SendTextVisualAsLit for a line that names
-// the parties to an event.
-//
-// ⚠️ NO READER OF THIS PATH IS EVER AT SightShapes, so names is never
-// consulted today: litRoom{} reports the room lit, and ParticipantSight only
-// returns SightShapes for an unblinded observer in an UNLIT room. It exists so
-// that the light and non-light end-text paths are threaded identically, and so
-// that if SendTextVisualAsLit ever grows a shapes tier, the names are already
-// there rather than newly missing. TestConditionEndRoomText_LightPathHasNoShapesTier
-// pins the reason.
-func (r *Room) SendTextVisualAsLitHidingNames(cat messaging.Category, txt string, names []string, excludeUserIds ...int) {
-	r.sendTextVisualJudgedBy(litRoom{}, cat, txt, names, false, excludeUserIds...)
-}
-
-// litRoom is a messaging.RoomVisibility test stand-in that always reports a
-// lit room. 60 is not a scale constant (LightLevel now computes a continuous
-// value; there is no fixed point to name), it is simply a value that sits at
-// or above LightDimBelow (default 50) and below LightExitsAbove (default 65),
-// so ParticipantSight reads it as SightFull for the room itself.
-type litRoom struct{}
-
-func (litRoom) LightLevel() int { return 60 }
-
-// sendTextVisualJudgedBy is SendTextVisual with the lighting it judges sight
-// against passed in, so SendTextVisualAsLit shares one delivery path. names,
-// when given, are hidden from an observer who makes out shapes only, including
-// bare names Anonymize cannot see. communication marks every message as player
-// chatter for the Deafened filter; only SendVisualCommunicationHidingNames
-// sets it.
-func (r *Room) sendTextVisualJudgedBy(lighting messaging.RoomVisibility, cat messaging.Category, txt string, names []string, communication bool, excludeUserIds ...int) {
+// sendTextVisualJudgedBy is the one delivery body of the visual senders that
+// judge each reader by the room as it is now. A line announcing a change to
+// the room's light is judged by the moment before instead, through
+// VisualSnapshot and SendTextVisualToSnapshot (#220 retired the as-lit
+// senders that used to share this body). names, when given, are hidden from
+// an observer who makes out shapes only, including bare names Anonymize
+// cannot see. communication marks every message as player chatter for the
+// Deafened filter; only SendVisualCommunicationHidingNames sets it.
+func (r *Room) sendTextVisualJudgedBy(cat messaging.Category, txt string, names []string, communication bool, excludeUserIds ...int) {
 	for _, uid := range r.GetPlayers() {
 		if excluded(uid, excludeUserIds) {
 			continue
@@ -403,7 +369,7 @@ func (r *Room) sendTextVisualJudgedBy(lighting messaging.RoomVisibility, cat mes
 		if u == nil {
 			continue
 		}
-		deliverVisual(u, visualDecision(u.Character, lighting), cat, txt, names, communication)
+		deliverVisual(u, visualDecision(u.Character, r), cat, txt, names, communication)
 	}
 }
 
````

**Modify `internal/usercommands/hood.go`:**

````diff
@@ -48,17 +48,23 @@ func Hood(rest string, user *users.UserRecord, room *rooms.Room, flags events.Ev
 		user.SendText(messaging.CategorySystem, `Your lantern is already hooded.`)
 		return true, nil
 	}
+	// Judged against the room just before the hood goes down (owner rule,
+	// 2026-10-05; #220): the hood is the end of a light, and judged by the
+	// room after it, the line would be silenced for everyone who was seeing
+	// by the lantern, while a watcher who could not see even by it learns
+	// nothing.
+	var beforeHood rooms.VisualSnapshot
+	if room != nil {
+		beforeHood = room.VisualSnapshot()
+	}
 	for _, rec := range recs {
 		rec.Hooded = true
 	}
 	user.SendText(messaging.CategorySystem, `You lower the hood over your lantern, and its light narrows to nothing.`)
 	if room != nil {
-		// Judged as if lit: the hood is the end of a light, and the room may
-		// already be dark by the time this line goes out, which would silence
-		// it for everyone who was seeing by the lantern.
-		room.SendTextVisualAsLit(messaging.CategoryMobEmote,
+		room.SendTextVisualToSnapshot(beforeHood, messaging.CategoryMobEmote,
 			fmt.Sprintf(`<ansi fg="username">%s</ansi> lowers the hood of their lantern, and its glow goes dark.`, user.Character.Name),
-			user.UserId)
+			[]string{user.Character.Name}, user.UserId)
 	}
 	// The band notice rides this command's own output. Left to the next
 	// command's pre-check, "darkness closes in" arrives after whatever the
````

**Modify `internal/usercommands/hood_test.go`:**

````diff
@@ -8,6 +8,7 @@ import (
 	"github.com/GoMudEngine/GoMud/internal/events"
 	"github.com/GoMudEngine/GoMud/internal/items"
 	"github.com/GoMudEngine/GoMud/internal/lightnotice"
+	"github.com/GoMudEngine/GoMud/internal/messaging"
 	"github.com/GoMudEngine/GoMud/internal/rooms"
 	"github.com/GoMudEngine/GoMud/internal/species"
 	"github.com/GoMudEngine/GoMud/internal/users"
@@ -131,9 +132,10 @@ func TestHood_NonAdjustableLightRefuses(t *testing.T) {
 	require.Equal(t, conditions.LightFull, rec.LightTrim)
 }
 
-// The room line for hood is sent judged as if lit: in a dark room the lantern
-// is the only light, so by the time the line goes out the room is dark and a
-// plain SendTextVisual would hide it from the very people who saw by it.
+// The room line for hood is judged against the room just before the hood went
+// down (#220): in a dark room the lantern is the only light, so by the time
+// the line goes out the room is dark and a plain SendTextVisual would hide it
+// from the very people who saw by it.
 func TestHood_ObserverSeesBothLinesInADarkRoom(t *testing.T) {
 	user, room := hoodFixture(t)
 	room.Biome = "cave"
@@ -159,6 +161,40 @@ func TestHood_ObserverSeesBothLinesInADarkRoom(t *testing.T) {
 		"an observer missed the unhood line")
 }
 
+// The other side of the same judgement (#220): a watcher who could not see
+// even with the lantern lit, here because they hold a darkness deeper than
+// the lantern's light, is not told the hood went down. Judged as if lit,
+// they were, and read the hooder's name.
+func TestHood_ObserverWhoCouldNotSeeBeforeIsNotTold(t *testing.T) {
+	const hoodTestGloomCond = 9753
+	user, room := hoodFixture(t)
+	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
+		hoodTestAdjustableCond: conditions.GetConditionSpec(hoodTestAdjustableCond),
+		hoodTestGloomCond: {ConditionId: hoodTestGloomCond, Name: "Test Hood Gloom", Secret: true,
+			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {Literal: 90}}},
+	}))
+	room.Biome = "cave"
+
+	observer := users.GetByUserId(2)
+	require.NotNil(t, observer)
+	observer.Character.RoomId = room.RoomId
+	room.AddPlayer(observer.UserId)
+	require.True(t, observer.Character.Conditions.AddCondition(hoodTestGloomCond, true))
+
+	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
+	require.True(t, ok, why)
+	require.True(t, user.Character.EmitsLight(), "fixture: the lantern must be lit")
+	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(observer.Character, room),
+		"fixture: the gloom must leave the observer blind even by the lantern")
+	events.DrainQueuedMessagesForTest(observer.UserId)
+
+	_, err := Hood("", user, room, 0)
+	require.NoError(t, err)
+	require.False(t, user.Character.EmitsLight(), "fixture: the hood must have gone dark")
+	require.NotContains(t, hoodTestText(observer.UserId), "lowers the hood of their lantern",
+		"an observer blind before the hood went down was told of it")
+}
+
 func TestHood_TwiceSaysAlreadyHooded(t *testing.T) {
 	user, room := hoodFixture(t)
 	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
````

**Modify `messaging_surface_guard_test.go`:**

````diff
@@ -890,9 +890,10 @@ func narrationRecognizeCall(call *ast.CallExpr) (narrationCallViewpoint, bool) {
 				return viewpointObserver, true
 			}
 		case "SendTextVisual", "SendTextVisualHidingNames",
-			"SendTextVisualAsLit", "SendTextVisualWithAudio",
+			"SendTextVisualWithAudio",
 			// Lighting plan 5d (owner rule, 2026-10-05): a line judged
-			// against a snapshot of the room before a darkness landed.
+			// against a snapshot of the room before its light changed. #220
+			// moved the last as-lit senders onto it and deleted them.
 			"SendTextVisualToSnapshot",
 			// Sight gates slice 5b: the name-hiding room senders, audio and
 			// visual, are room broadcasts too.
@@ -1367,7 +1368,7 @@ var narrationViewpointRegistry = map[string]narrationEntry{
 	"usercommands/go.go|messaging.CategorySystem, playerMsg":                                                       {verdictCorrect, true, false, true, "audit: player unlocks a door -- acts on an exit, not a character (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/go.go:98)"},
 	"usercommands/guild.go|<ansi fg=\"username\">%s</ansi> invites you to join <ansi fg=\"yellow-bold\">%s</ans":   {verdictCorrect, true, true, false, "a guild invite is sent privately to the invitee (actee); guild management has no room-facing component anywhere in this file."},
 	"usercommands/guild.go|You have been removed from <ansi fg=\"yellow-bold\">%s</ansi>.":                         {verdictCorrect, true, true, false, "a guild kick notifies the removed member privately (actee); same no-room-component reasoning as the invite."},
-	"usercommands/hood.go|You lower the hood over your lantern, and its light narrows to nothing.":                 {verdictCorrect, true, false, true, "hood closes the lantern in the actor's own light slot -- self-targeted, no actee; the room gets the third-person line from the room.SendTextVisualAsLit just below. Lighting plan 5a, read against source for this guard."},
+	"usercommands/hood.go|You lower the hood over your lantern, and its light narrows to nothing.":                 {verdictCorrect, true, false, true, "hood closes the lantern in the actor's own light slot -- self-targeted, no actee; the room gets the third-person line from the room.SendTextVisualToSnapshot just below, judged by the room before the hood went down (#220). Lighting plan 5a, read against source for this guard."},
 	"usercommands/hood.go|You throw back the hood of your lantern, and light floods out around you.":               {verdictCorrect, true, false, true, "unhood opens the lantern in the actor's own light slot -- self-targeted, no actee; the room gets the third-person line from the room.SendTextVisual just below. Lighting plan 5a, read against source for this guard."},
 	"usercommands/inventory.go|<ansi fg=\"yellow\">%d grenade(s) have destabilized and dissolved into putrid resi": {verdictCorrect, true, false, true, "audit: own grenades destabilize in the backpack -- self-directed accident (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/inventory.go:94)"},
 	"usercommands/lock.go|You use a key to relock the <ansi fg=\"container\">%s</ansi>.":                           {verdictCorrect, true, false, true, "audit: relocks a container with a key -- target is an object (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md, usercommands/lock.go:63)"},
````

**Modify `shipped_narration_data_guard_test.go`:**

````diff
@@ -962,11 +962,12 @@ var observerIdentityGuardRoots = []string{
 //	                                    trigger NewRound_UserRoundTick.go:302
 //	                                    and NewRound_MobRoundTick.go:287, end
 //	                                    sendConditionEndRoomText
-//	                                    (NewTurn_PruneConditions.go:137).
-//	                                    1-illumination.yaml is safe for a
-//	                                    second reason as well: its light flag
-//	                                    routes it through SendTextVisualAsLit,
-//	                                    which has no SightShapes tier at all.
+//	                                    (NewTurn_PruneConditions.go:145), which
+//	                                    since #220 judges a light's or a
+//	                                    darkness's end line against a snapshot
+//	                                    of the room before it ran out, shapes
+//	                                    tier included, so 1-illumination.yaml
+//	                                    relies on HideNames like the rest.
 //	                                    inObserverRole was widened from an
 //	                                    exact "observer"/"remote_observer"
 //	                                    match to a suffix match, because
@@ -1019,7 +1020,7 @@ var observerIdentityGuardContentSafeViaCode = map[string]bool{
 	// name-referencing observer line, not only the ones that author a
 	// `_plain` token; see the doc comment above.
 	"conditions/0-meditating.yaml":         true,
-	"conditions/1-illumination.yaml":       true, // also routed through SendTextVisualAsLit, which has no SightShapes tier
+	"conditions/1-illumination.yaml":       true, // a light: its end line is judged against the room before it ran out (#220)
 	"conditions/2-stunned.yaml":            true,
 	"conditions/3-blinded.yaml":            true,
 	"conditions/9-hidden.yaml":             true,
````

- [ ] **Step 2: Nothing names the retired senders**

```bash
grep -rn "SendTextVisualAsLit\|litRoom{}" --include=*.go internal modules *.go
```

Expected: one line only, the dated "measured at that moment" comment in `messaging_surface_guard_test.go` that names `SendTextVisualAsLit` on purpose.

- [ ] **Step 3: Tests, and the expiry predicate can fail**

```bash
gofmt -l internal/ modules/
go build ./...
go vet ./internal/conditions ./internal/hooks ./internal/rooms ./internal/usercommands
go test ./internal/conditions ./internal/hooks ./internal/rooms ./internal/usercommands -count=1
go test . -count=1 2>&1 | grep -E "^(--- FAIL|ok|FAIL)"
```

Expected: clean, all `ok`. Null probe: in `ExpiresOnNextTrigger` change `== 1` to `<= 2`; `go test ./internal/conditions -run TestExpiresOnNextTrigger -count=1` fails at round 2. Restore.

- [ ] **Step 4: Spec check and commit**

```bash
P="bauble_finder_view_guard_test.go internal/conditions/conditions.go internal/conditions/conditionspec.go internal/conditions/context.md internal/conditions/expires_next_test.go internal/hooks/NewRound_MobRoundTick.go internal/hooks/NewRound_UserRoundTick.go internal/hooks/NewTurn_PruneConditions.go internal/hooks/condition_end_snapshot.go internal/hooks/condition_end_snapshot_test.go internal/hooks/condition_room_text_test.go internal/hooks/context.md internal/hooks/narration_testhelpers_test.go internal/rooms/context.md internal/rooms/hiding_senders_test.go internal/rooms/rooms.go internal/usercommands/hood.go internal/usercommands/hood_test.go messaging_surface_guard_test.go shipped_narration_data_guard_test.go"
git add $P
git diff --cached b86442e68 -- $P
git commit -m "fix(lighting): hood and light or darkness end lines are judged against the moment before the change (#220)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: delete `DarknessTerms` (#221)

Checkpoint `80326c021`. Model: sonnet. Delete the method first: `go build ./... && go vet ./internal/usercommands` fails at `darkness_test.go:72: ... DarknessTerms undefined`, the only caller. Then apply the test edit.

**Modify `internal/characters/context.md`:**

````diff
@@ -343,10 +343,13 @@ Hand candidates (`handCandidates`): a two-hander only offers whole pairs, ordere
   `internal/rooms` feeds into the room's combine.
 - `EmitsLight() bool` (`light.go`): any term at all. A shut hood or a source
   trimmed to nothing does not count. It replaced the retired `lightsource` flag.
-- `DarknessTerms() []float64` (`light.go`, lighting plan 5d): `LightTerms`'
-  twin, one term per held darkness record (`Conditions.DarknessSources`). A
-  darkness is never a light term, so a darkness bearer does not `EmitsLight`:
-  no sneak beacon, no `lit` adjective, no woken sleepers.
+- A darkness record (lighting plan 5d) is never a light term, so a darkness
+  bearer does not `EmitsLight`: no sneak beacon, no `lit` adjective, no woken
+  sleepers. There is no darkness twin of `LightTerms` on `Character` (#221
+  deleted the unused one); read the records with
+  `Conditions.DarknessSources()` and `Condition.LightNow`. `internal/rooms`
+  walks `Conditions.LightAndDarknessSources()` the same way when it composes
+  a room's light.
 - `FindItemNoun(word) (noun, desc string, ok bool)` (`itemnouns.go`): an EXACT
   match on `ItemSpec.Nouns` across worn items first, then the backpack and
   bandolier. Exact on purpose: `look` runs it before item matching, and a prefix
@@ -2021,7 +2024,7 @@ disk. Grouped by what they own:
 |-------|-------|
 | Core | `character.go`, `validate.go`, `migrations.go`, `overrides.go`, `description.go`, `formattedname.go`, `actor_identity.go` |
 | Stats & progression | `progression.go`, `progression_award_resolved.go` (`AwardResolved`, the U10b-1 firing rule), `progression_notify.go` (`SetProgressionNotifier`, the injected notify-text callback), `skills.go`, `effective_stats.go`, `mobmastery.go`, `kdstats.go` |
-| Resources & timed state | `pools.go`, `reservation.go`, `resources.go`, `cooldowns.go`, `conditions.go` (holds `Character.AddCondition`, `AddConditionScaled` and the `AddConditionMagnitude` writer door), `sight.go`, `vision.go` (`NightVisionStrength`, `InfraReach`, the window model's two observer numbers), `light.go` (`LightTerms`, `EmitsLight`, plan 5a; `DarknessTerms`, plan 5d) |
+| Resources & timed state | `pools.go`, `reservation.go`, `resources.go`, `cooldowns.go`, `conditions.go` (holds `Character.AddCondition`, `AddConditionScaled` and the `AddConditionMagnitude` writer door), `sight.go`, `vision.go` (`NightVisionStrength`, `InfraReach`, the window model's two observer numbers), `light.go` (`LightTerms`, `EmitsLight`, plan 5a) |
 | Inventory & gear | `inventory.go`, `inventory_handle.go`, `itemnouns.go` (`FindItemNoun`, plan 5a), `worn.go`, `hand_slots.go`, `anatomy.go`, `masterwork.go`, `migrate_enchantments.go`, `migrate_detuned_bows.go` |
 | Combat | `combat.go`, `combat_tokens.go`, `position_predicates.go`, `taunt_hold.go`, `submission_policy.go`, `die.go`, `respawn_home.go`, `engagement_storage.go` (was `combat_state_compat.go`; renamed by U12c-2 when the struct it kept compatible was deleted), `flee_admission.go` (`FleeAdmission`, `PublishFleeAdmission`, `TakeFleeAdmission`, `CancelFleeAdmission`, slice 4a) |
 | Casting | `cast_helpers.go`, `spells.go` |
````

**Modify `internal/characters/light.go`:**

````diff
@@ -14,20 +14,6 @@ func (c *Character) LightTerms() []float64 {
 	return out
 }
 
-// DarknessTerms is every darkness term this character takes from its room
-// right now, one per held darkness record (lighting plan 5d). LightTerms'
-// twin: a darkness is never one of LightTerms, so it never makes a
-// character EmitsLight.
-func (c *Character) DarknessTerms() []float64 {
-	var out []float64
-	for _, rec := range c.Conditions.DarknessSources() {
-		if v, ok := rec.LightNow(conditions.GetConditionSpec(rec.ConditionId)); ok {
-			out = append(out, v)
-		}
-	}
-	return out
-}
-
 // EmitsLight reports whether this character sheds any light right now. A
 // shut hood or a source trimmed to nothing does not count. It replaced the
 // retired lightsource flag.
````

**Modify `internal/usercommands/darkness_test.go`:**

````diff
@@ -69,7 +69,12 @@ func TestDarknessIsNeverLight(t *testing.T) {
 	_, ok, why := user.Character.Wear(items.New(darkTestUmbralItem))
 	require.True(t, ok, why)
 	require.True(t, user.Character.Conditions.AddConditionMagnitude(darkTestPallCond, 4, 50))
-	require.Len(t, user.Character.DarknessTerms(), 2, "fixture: both darknesses must be held and on")
+	held := user.Character.Conditions.DarknessSources()
+	require.Len(t, held, 2, "fixture: both darknesses must be held")
+	for _, rec := range held {
+		_, on := rec.LightNow(conditions.GetConditionSpec(rec.ConditionId))
+		require.True(t, on, "fixture: darkness %d must be on", rec.ConditionId)
+	}
 
 	require.Empty(t, user.Character.LightTerms(), "a darkness is not a light term")
 	require.False(t, user.Character.EmitsLight(), "a darkness bearer must not shed light")
````

- [ ] **Step 1: Tests and commit**

```bash
gofmt -l internal/
go test ./internal/characters ./internal/usercommands ./internal/rooms -count=1
P="internal/characters/context.md internal/characters/light.go internal/usercommands/darkness_test.go"
git add $P
git diff --cached 80326c021 -- $P
git commit -m "chore(lighting): delete the unused DarknessTerms (#221)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Then comment on #221 (search first that nobody has): the `carriedTerms` nil guard needs no code, because `LightNow` with a nil spec returns false (`LightMax(nil)` is 0) before `IsDarknessSource` is reached (`internal/conditions/light.go:24-27,43-52`; `internal/rooms/lighting.go:214-230`).

---

### Task 5: docs and patch notes

Checkpoint `bee8ae26e`. Model: sonnet.

**Modify `docs/PATCH_NOTES.md`:**

````diff
@@ -1,5 +1,19 @@
 # DOGMud Patch Notes
 
+## 2026-10-06: Clearer in the dark
+
+- When the room is too dark for you, `look` and `who` now say so: "It is
+  too dark to see. You need light, or eyes that do not need it." If you are
+  blinded, they still say "You can't see anything!", since light will not
+  help.
+- Fighting in light too bright for your eyes now tells you, once per
+  fight, that the glare is weakening your attacks and defense. With combat
+  messages set to light, you will not see it.
+- When a lantern is hooded, or a light or a darkness runs out, the line
+  about it now reaches exactly the people who could see just before it
+  changed. You no longer read that a light went out if you could not see
+  by it, or that a darkness lifted while it still blinded you.
+
 ## 2026-10-06: Gear that strikes back
 
 - The Blackrazor's life drain, the Aegis of Mockery's shockwave, the
````

**Modify `internal/hooks/context.md`:**

````diff
@@ -932,9 +932,10 @@ medium / light). Touch-points live in `dispatchCritAndMessaging`
   booleans (`srcCanSee`/`tgtCanSee`) that section already computes.
   Shapes-only viewers are included, not just fully blind ones: the
   notice keys on the sight VERDICT, while the score rides the sight ramp
-  (lighting plan 5b, `internal/combat/context.md`). Dazzle has its own
-  plan 3d notices, so a dazzled combatant with full sight gets no blind
-  notice, by design. Membership in this set is
+  (lighting plan 5b, `internal/combat/context.md`), and only when the
+  player's `messaging.SightMult` in the room is below 1.0 (lighting plan
+  5c). A dazzled combatant with full sight gets no blind notice, by
+  design: the glare notice below is theirs. Membership in this set is
   the "fought this round" signal; `roundTallies` cannot serve that role
   because it only contains Light-verbosity viewers who could ALSO see
   clearly (recording is skipped for a blind participant precisely to
@@ -943,11 +944,23 @@ medium / light). Touch-points live in `dispatchCritAndMessaging`
   `DoCombat` beside `flushCombatTallies`, sends
   `messaging.CategoryCombatBlindWarning` once per blind combatant and
   clears the set. Not floor-protected: it goes through the viewer's
-  ordinary `Verbosity.Suppresses` gate, but the category is
-  deliberately absent from both suppression tables (see
-  `internal/messaging/verbosity.go`), so it currently passes at every
-  verbosity level; it is the only combat text a blind Light-verbosity
-  combatant receives at all.
+  ordinary `Verbosity.Suppresses` gate, and the category is in
+  `suppressibleAtLight` only (`internal/messaging/verbosity.go`, owner
+  ruling): it passes at Full and Medium and is suppressed at Light.
+- **`markGlareCombatant` / `flushGlareCombatNotices`** (#319): the blind
+  notice's twin for glare. A dazzled fighter is `SightFull`, so the blind
+  notice never speaks for them, yet glare lowers their `SightMult` all the
+  same. `markGlareCombatant`, called beside `markBlindCombatant` in
+  `dispatchCritAndMessaging` with the same verdicts, records a player who
+  sees clearly while `messaging.ComfortDistance`'s bright fraction is above
+  0 and `SightMult` is below 1.0 (`roundGlareCombatants`).
+  `flushGlareCombatNotices`, called right after `flushBlindCombatNotices` at
+  the end of `DoCombat`, sends `glareCombatNoticeText` ("The glare is too
+  bright, so your attacks and defense are weaker.") on the same category and
+  verbosity gate, but ONCE PER FIGHT: `glareToldThisFight` remembers who was
+  told, and the flush forgets every player no longer
+  `Character.IsInCombat()`, so a later fight tells them again. A notice
+  suppressed at Light is not counted as told.
 
 ### Attacker progression firing (U10b-1 Task 10)
 
````

**Modify `internal/messaging/context.md`:**

````diff
@@ -60,7 +60,8 @@ Types and constants:
   the per-round compact tally emitted by the light-verbosity path, and
   `CategoryCombatBlindWarning` for the per-round "you can't see clearly"
   notice, M4d PR 2 Task 4, sent by `internal/hooks`'
-  `flushBlindCombatNotices`). `CategoryCombatBlindWarning` is in
+  `flushBlindCombatNotices`, and for the once-per-fight glare notice, #319,
+  sent by `flushGlareCombatNotices`). `CategoryCombatBlindWarning` is in
   `verbosity.go`'s `suppressibleAtLight` only (owner ruling, M4d PR 2
   followup): suppressible at Light, not at Medium, since at Medium the
   player still reads the swing prose the notice explains, while Light is
````

**Modify `internal/usercommands/context.md`:**

````diff
@@ -31,7 +31,10 @@ The `internal/usercommands` package implements the complete command system for p
     shut or open the hood of the `adjustable` light in the `Light` slot. A
     hooded record stays held and lit but sheds nothing (`Condition.Hooded`).
     `hoodedLight` sends its own refusal and tells an empty slot apart from a
-    light with no hood.
+    light with no hood. `Hood` takes `room.VisualSnapshot()` before setting
+    `Hooded` and sends the room line with `SendTextVisualToSnapshot`, so it
+    is judged by what each watcher could see just before the light went
+    (#220): it reaches those who saw by the lantern and nobody who could not.
   - `cancel <spell>` (`cancel.go`: `Cancel`, `cancelCondition`,
     `cancelNameMatches`): an activity in progress ALWAYS wins, whatever the
     argument; only a free user reaches `cancelCondition`, which ends the first
````

- [ ] **Step 1: Checks and commit**

```bash
python tools/context_md_audit.py 2>&1 | grep -A6 "^internal/\(actions\|usercommands\|mobcommands\|hooks\|rooms\|conditions\|characters\|messaging\)\b"
git diff origin/master -- docs internal/*/context.md | grep "^+" | grep -c "—\|–"
P="docs/PATCH_NOTES.md internal/hooks/context.md internal/messaging/context.md internal/usercommands/context.md"
git add $P
git diff --cached bee8ae26e -- $P
git commit -m "docs: lighting small fixes" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected: no audit output, then `0` (run the `grep -c` on its own line: it exits 1 on a zero count), then an empty diff.

---

### Task 6: Gates and PR

- [ ] **Step 1: Full gate**

```bash
gofmt -l internal/ modules/
go vet ./...
go build ./...
go test ./... -count=1 2>&1 | grep -E "^(--- FAIL|FAIL|panic)"
golangci-lint run --new-from-merge-base=origin/master
```

Expected: nothing, nothing, nothing, nothing, `0 issues.` Known flake: `internal/playtestrun` can hang under load; rerun it alone.

- [ ] **Step 2: Docker race run and private-port boot**

```bash
docker build --target test -f provisioning/Dockerfile -t dogmud-lf-test . > "$TMP/lf-build.log" 2>&1
docker run --rm dogmud-lf-test > "$TMP/lf-race.log" 2>&1
grep -c "WARNING: DATA RACE" "$TMP/lf-race.log"
grep -E '^(--- FAIL|FAIL)' "$TMP/lf-race.log"
docker rmi dogmud-lf-test
O="$TMP/boot-overrides.yaml"
printf 'Network.TelnetPort: [33334]\nNetwork.LocalPort: 9998\nNetwork.HttpPort: 8091\nNetwork.HttpsPort: 0\nNetwork.AIPort: 0\n' > "$O"
go build -o boot-check.exe .
CONFIG_PATH="$O" LOG_NOCOLOR=1 timeout 150 ./boot-check.exe > "$TMP/boot.log" 2>&1; echo "exit=$?"
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$TMP/boot.log"
grep -c "Server Ready" "$TMP/boot.log"
rm boot-check.exe
```

Expected: `0` races; the only failures are `TestNoStringOrDataSaysBuff` (root) and `TestU10DoneWhen_DeadPathsStayDead` (`internal/combat`), which shell out to `git` and have no repository in the image, with their `FAIL` package lines (the dry run gave exactly that, 128 packages `ok`); then `exit=124`, `0`, `1`. Never kill a server by name or port.

- [ ] **Step 3: Whole-branch review** (opus, read-only), findings fixed in their own commits. No playtest in this PR: plan 6's closing playtest covers lighting in game, with #332 (owner, 2026-10-06).

- [ ] **Step 4: PR.** `git push -u origin fix/lighting-small-fixes`; `gh pr create --repo pruuk/DOGMud --base master --head fix/lighting-small-fixes ...` with a body that says "Closes #220, closes #221, closes #319, closes #364" (each issue closes at merge) and "Part of #372"; never write a closing keyword before #332 or #372. Read back the URL: it must say `pruuk/DOGMud`. Merge with `--merge --delete-branch` once checks are green. The owner deploys.

---

## Self-review

- Spec coverage: #364 Task 1; #319 Task 2 (once per fight, owner change); #220 Task 3 (hood, light end, darkness end, retire the as-lit senders); #221 Task 4 (delete, nil guard shown moot); docs and patch notes Task 5; gates Task 6. #332 is not here (plan 6).
- Every symbol named in the prose exists at its checkpoint; G1 to G5 were read at `59851f0a4`.
