# Sight Gates Close-out Playtest Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix what the sight-gates close-out playtest (run `6b3b017287df1ded`) found, so a re-run finds no leak and #382 closes.

**Architecture:** Each fix reuses an existing seam: `Character.Validate` reconciles sight (F1); a tag-aware `messaging.HideWeapons` pass in the style of `Anonymize`, on combat paths only (F2); exported disruption sound lines through the visual-with-audio paths (F3); `SpottedLine`'s shared body for the arrival notice (F4); `AddCondition` entering Awareness Hidden for the stealth record (F5); copy and the GMCP map gate (F6, F7).

**Tech Stack:** Go, YAML content under `_datafiles/world/dogmud`, GMCP module, repo-root guard tests.

**Spec:** `docs/superpowers/specs/2026-10-08-sight-gates-playtest-fixes-design.md` (owner approved 2026-10-08).

---

## How to run this plan

- **One PR**, branch `fix/sight-gates-playtest` (worktree `C:/tmp/dogmud-p2-playtest`, spec commit `cc9584952` on `f508db20b`). Task order: T0, F1, F4, F5, F2a, F2b, F2c, F3a to F3d, F6a to F6d, F7, then P1 (gate) and P2 (re-run playtest).
- **Dry run.** Every section was dry-run on `f508db20b` on its own, NOT stacked. 🪤 Locate every edit by its quoted `old_string`, never by line number. Files edited by more than one task: `internal/characters/conditions.go` (F1 edits the Perception blocks, F5 the function tails), `internal/messaging/context.md` (F2a, F3a), `internal/hooks/NewRound_DoCombat_helpers.go` (F3a, F3b). A quoted "expected FAIL" output that differs only in a line number is the stacking, not a defect.
- **Gate for every task:** the package tests AND the repo root (`go test . -count=1`); `gofmt -l` on touched files before each commit.
- **Edits go through the Edit or Write tool only.** No sed, no Python, no scripted find-and-replace, even for the 15 YAML lines in F6a.
- **Commits:** named paths only, never `git add -A` or `git add .`. Every message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. No git checkout, switch, reset or stash.
- **PR body** uses `Refs #N`, never a closing keyword. Issues close by hand after P2.
- **No em or en dashes** in prose or player text; player text within 80 columns. Em dashes that sit inside existing code comments or context.md text an `old_string` must match (F3a, F7) stay as they are.
- **Never kill a server by name or port.** Kill only a PID you started.

## Task index

| Task | Fix |
|---|---|
| T0 | `TestSneak_BlindObserverRollsByEar` drains its queue first (a PR 1 test that fails under a filtered run) |
| F1 | Blindness ends: `Validate` reconciles Perception with the blind sources |
| F5 | A mob spawned hidden is hidden |
| F4 | A sneaker spotted on arrival is told |
| F2a to F2c | Weapon names generic below full sight (R8) |
| F3a to F3d | Every disruption is heard (R4) |
| F6a to F6d | Copy: article before mobname, aggro full stop, "On the Ground" wraps, `time` dusk and dawn |
| F7 | The GMCP zone map stays dark |
| P1, P2 | Gate and PR; re-run playtest |

## Known limits (accepted; note them in the PR body)

- F2: when both fighters hold weapons with the same display name, the other side's weapon stays named in a participant line, because it matches the reader's own.
- F3: a hidden mob that flees mid-cast is named in its break line; combat reveals a hidden mob first, so this is not reachable in play today.
- F5 keys on record 9. Condition 31 (Empathic Shroud) carries the `hidden` flag but has never hidden anyone; that is filed as a separate issue.

---

### Task T0: the blind-observer sneak test starts from an empty queue

**Files:**
- Modify: `internal/actions/sneak_hearing_test.go` (`TestSneak_BlindObserverRollsByEar`)

`TestSneak_BlindObserverRollsByEar` asserts user 9811's queue is empty without draining it first, so lines left by earlier move tests fail it under a `-run` filter. Its sibling `TestSneak_SuperhearingObserverStillHears` already drains.

- [ ] **Step 1: See it fail under the filter**

Run: `go test ./internal/actions -run 'TestEntryDetection|TestSpottedLine|TestSneak' -count=1`
Expected: FAIL in `TestSneak_BlindObserverRollsByEar` at `require.Empty(t, hearLines(9811))`.

- [ ] **Step 2: Drain before acting**

Edit `internal/actions/sneak_hearing_test.go`, old_string:

```go
	actor, mc := w.place(t, "player", 9810, "Sneak")
	mc.Stats.Dexterity.ValueAdj = 100

	got := Sneak(actor)
```

new_string:

```go
	actor, mc := w.place(t, "player", 9810, "Sneak")
	mc.Stats.Dexterity.ValueAdj = 100
	events.DrainQueuedMessageEventsForTest(9811)

	got := Sneak(actor)
```

- [ ] **Step 3: Run it again, then the package**

Run: `go test ./internal/actions -run 'TestEntryDetection|TestSpottedLine|TestSneak' -count=1`, then `go test ./internal/actions -count=1`
Expected: both PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/actions/sneak_hearing_test.go
git commit -m "test(sight): blind-observer sneak test drains its queue first (#333)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task F1: blindness ends (Validate reconciles Perception with the blind sources)

**Why:** only `Character.RemoveCondition` flipped Perception from Blinded back to Sighted. The round's prune (`hooks.PruneConditions` calls `Conditions.Prune()` and then `Validate`) never calls it, so a player whose Blinded (3) or Flashbang Blindness (77) ran out stayed blind until they relogged. `Perception` is `yaml:"-"`, so a load starts Sighted, which means a relog also cured blindness that was still live. `AddConditionMagnitude` had no flip at all. Fix: one reconcile in `Character.Validate`, after the condition lookups are rebuilt. The three per-door flips go away, since each of those doors already calls `Validate`.

**Files:**
- Modify: `internal/characters/sight.go` (add `reconcilePerception`)
- Modify: `internal/characters/validate.go` (call it after `c.Conditions.Validate()`)
- Modify: `internal/characters/conditions.go` (drop the three inline flips and the now-unused `perception` import)
- Modify: `internal/state/perception/context.md`, `internal/characters/context.md`
- Test: `internal/state/perception/integration_test.go` (PE-INT-008, 009, 010)
- Create: `internal/hooks/blind_expiry_test.go`

Facts (master `f508db20b`): flips at `internal/characters/conditions.go:144-148` (AddCondition), `:160-164` (AddConditionScaled), `:204-208` (RemoveCondition). `AddConditionMagnitude` (`:184`) has none. `HasAnyBlindSource` is at `sight.go:24`. `c.Conditions.Validate()` is at `validate.go:694`; the Perception nil-guard is at `:634`, earlier, so the reconcile must sit after `:694` or it reads unbuilt lookups. `Conditions.Prune()` drops the record and rebuilds the lookups (`internal/conditions/conditions.go:576-615`). `mobs.Mob.Validate` overwrites Perception unconditionally after `Character.Validate` (`internal/mobs/mobs.go:1306`, `:1321`); that only runs at spawn and load, and the next `Character.Validate` (any condition change) reconciles. No production code registers a Perception observer or veto, and nothing reads the transition `Metadata` (grep `Perception.RegisterObserver|Perception.RegisterVeto` over `internal` and `modules` finds nothing outside tests).

- [ ] **Step 1: Write the failing tests**

In `internal/state/perception/integration_test.go`, append after `TestIntegration_MixedSourceOrder` (anchor: the end of that function):

old_string:
```go
	c.RemoveCondition(perception.ConditionIdBlinded)
	if c.Perception.State() != perception.Sighted {
		t.Errorf("after RemoveCondition (no sources left), state = %v, want Sighted", c.Perception.State())
	}
}
```
new_string:
```go
	c.RemoveCondition(perception.ConditionIdBlinded)
	if c.Perception.State() != perception.Sighted {
		t.Errorf("after RemoveCondition (no sources left), state = %v, want Sighted", c.Perception.State())
	}
}

// expireCondition marks a held record spent, as the round tick does when its
// last trigger fires, without going through RemoveCondition.
func expireCondition(t *testing.T, c *characters.Character, conditionId int) {
	t.Helper()
	for _, b := range c.Conditions.List {
		if b.ConditionId == conditionId {
			b.TriggersLeft = conditions.TriggersLeftExpired
			return
		}
	}
	t.Fatalf("condition %d not held", conditionId)
}

// PE-INT-008: natural expiry. The prune pass drops a spent record and then
// calls Validate; it never calls RemoveCondition. Sight must still come back.
func TestIntegration_NaturalExpiryReturnsSighted(t *testing.T) {
	defer seedBlindConditions(t)()

	for _, id := range []int{perception.ConditionIdBlinded, perception.ConditionIdFlashbangBlindness} {
		c := characters.New()
		if err := c.AddCondition(id, false); err != nil {
			t.Fatalf("AddCondition(%d): %v", id, err)
		}
		expireCondition(t, c, id)
		c.Conditions.Prune()
		_ = c.Validate()
		if c.Perception.State() != perception.Sighted {
			t.Errorf("condition %d expired and pruned, state = %v, want Sighted", id, c.Perception.State())
		}
	}
}

// PE-INT-009: a fresh load. Perception is runtime only (yaml:"-"), so a load
// builds a Sighted machine; Validate must blind a holder whose blind
// condition is still live, or a relog cures blindness.
func TestIntegration_ReloadKeepsLiveBlindness(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	if err := c.AddCondition(perception.ConditionIdBlinded, false); err != nil {
		t.Fatalf("AddCondition(3): %v", err)
	}
	c.Perception = nil // what a load from YAML hands Validate
	_ = c.Validate()
	if c.Perception.State() != perception.Blinded {
		t.Errorf("reloaded with condition 3 live, state = %v, want Blinded", c.Perception.State())
	}
}

// PE-INT-010: every door that adds a record blinds, including the
// magnitude door, which had no Perception flip of its own.
func TestIntegration_MagnitudeDoorBlinds(t *testing.T) {
	defer seedBlindConditions(t)()

	c := characters.New()
	if err := c.AddConditionMagnitude(perception.ConditionIdFlashbangBlindness, 0, 1, "test"); err != nil {
		t.Fatalf("AddConditionMagnitude(77): %v", err)
	}
	if c.Perception.State() != perception.Blinded {
		t.Errorf("after AddConditionMagnitude(77), state = %v, want Blinded", c.Perception.State())
	}
}
```

(The file already imports `conditions`, `characters` and `perception`.)

Create `internal/hooks/blind_expiry_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Blindness that runs out on its own must give the player their sight back.
// The round's prune pass drops the spent record and calls Validate; it never
// goes through RemoveCondition, which used to be the only path that restored
// sight (playtest 2026-10-08: "Your vision slowly returns to normal." and then
// "You can't see anything!" until a relog).
func TestPruneConditions_ExpiredBlindnessRestoresSight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		perception.ConditionIdBlinded:            {ConditionId: perception.ConditionIdBlinded, Name: "Blinded", RoundInterval: 1, TriggerCount: 3, EndUserText: "Your vision slowly returns to normal."},
		perception.ConditionIdFlashbangBlindness: {ConditionId: perception.ConditionIdFlashbangBlindness, Name: "Flashbang Blindness", RoundInterval: 1, TriggerCount: 2},
	})
	defer restore()

	for _, id := range []int{perception.ConditionIdBlinded, perception.ConditionIdFlashbangBlindness} {
		holder := users.GetByUserId(1)
		require.NoError(t, holder.Character.Validate()) // the test user starts with no runtime machines
		require.NoError(t, holder.Character.AddCondition(id, false))
		require.Equal(t, perception.Blinded, holder.Character.Perception.State(), "condition %d blinds", id)

		expire(t, holder.Character.Conditions.List, id)
		PruneConditions(events.NewTurn{TurnNumber: 1})

		assert.Equal(t, perception.Sighted, holder.Character.Perception.State(), "condition %d ran out, so sight is back", id)
		drainPlain(1)
	}
}
```

`expire` (`internal/hooks/condition_room_text_test.go:24`), `seedAllRegistries` (`hooks_test.go:57`) and `drainPlain` are existing package test helpers. The `Validate` call matters: `users.NewTestUser` builds a character with a nil Perception, and without it `AddCondition`'s own flip is skipped on master, so the test would fail at setup instead of at the assertion under test.

- [ ] **Step 2: Run the tests and see them fail**

```
go test ./internal/state/perception/ -run 'TestIntegration_(NaturalExpiry|ReloadKeeps|MagnitudeDoor)' -count=1
go test ./internal/hooks/ -run TestPruneConditions_ExpiredBlindnessRestoresSight -count=1
```
Expected FAIL (dry run on `f508db20b`):
```
--- FAIL: TestIntegration_NaturalExpiryReturnsSighted (0.00s)
    integration_test.go:196: condition 3 expired and pruned, state = Blinded, want Sighted
    integration_test.go:196: condition 77 expired and pruned, state = Blinded, want Sighted
--- FAIL: TestIntegration_ReloadKeepsLiveBlindness (0.00s)
    integration_test.go:214: reloaded with condition 3 live, state = Sighted, want Blinded
--- FAIL: TestIntegration_MagnitudeDoorBlinds (0.00s)
    integration_test.go:228: after AddConditionMagnitude(77), state = Sighted, want Blinded
FAIL	github.com/GoMudEngine/GoMud/internal/state/perception
```
```
--- FAIL: TestPruneConditions_ExpiredBlindnessRestoresSight (0.00s)
    blind_expiry_test.go:37: ... expected: 0 actual  : 1 ... Messages: condition 3 ran out, so sight is back
    blind_expiry_test.go:37: ... expected: 0 actual  : 1 ... Messages: condition 77 ran out, so sight is back
FAIL	github.com/GoMudEngine/GoMud/internal/hooks
```
(Perception states: 0 is Sighted, 1 is Blinded.)

- [ ] **Step 3: Add the reconcile**

`internal/characters/sight.go`, old_string:
```go
import (
	"github.com/GoMudEngine/GoMud/internal/state/perception"
)
```
new_string:
```go
import (
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
)

// reconcilePerception brings the Perception machine in line with the blind
// sources the character holds. Validate calls it after the condition lookups
// are rebuilt, so every path that changes conditions agrees: an add on any
// door, RemoveCondition, the round's prune of a spent record, a death or
// purge that expires records, and a load (Perception is runtime only, so a
// load starts Sighted and a still-live blind condition must blind again).
func (c *Character) reconcilePerception() {
	if c.Perception == nil {
		return
	}
	blind := c.HasAnyBlindSource()
	switch {
	case blind && c.Perception.State() == perception.Sighted:
		_ = c.Perception.TransitionTo(perception.Blinded,
			state.TransitionReason{Trigger: perception.TriggerConditionApplied})
	case !blind && c.Perception.State() == perception.Blinded:
		_ = c.Perception.TransitionTo(perception.Sighted,
			state.TransitionReason{Trigger: perception.TriggerConditionExpired})
	}
}
```

`internal/characters/validate.go`, old_string:
```go
	c.Conditions.Validate()

	// Ensure all known skills exist at rank 1 minimum.
```
new_string:
```go
	c.Conditions.Validate()
	c.reconcilePerception()

	// Ensure all known skills exist at rank 1 minimum.
```

- [ ] **Step 4: Drop the three per-door flips** (each door already calls `Validate`)

`internal/characters/conditions.go`, AddCondition, old_string:
```go
	if !c.Conditions.AddCondition(conditionId, isPermanent) {
		return fmt.Errorf(`failed to add condition. target: "%s" conditionId: %d`, c.Name, conditionId)
	}
	// Chunk 6 (Perception): blind-source conditions trigger Sighted → Blinded.
	// Guard against re-entry: only fire if state is currently Sighted.
	if (conditionId == perception.ConditionIdBlinded || conditionId == perception.ConditionIdFlashbangBlindness) &&
		c.Perception != nil && c.Perception.State() == perception.Sighted {
		_ = c.Perception.TransitionTo(perception.Blinded,
			state.TransitionReason{Trigger: perception.TriggerConditionApplied, Metadata: map[string]any{"conditionId": conditionId}})
	}
	_ = c.Validate()
```
new_string:
```go
	if !c.Conditions.AddCondition(conditionId, isPermanent) {
		return fmt.Errorf(`failed to add condition. target: "%s" conditionId: %d`, c.Name, conditionId)
	}
	// Validate brings Perception in line with the blind sources.
	_ = c.Validate()
```

AddConditionScaled, old_string:
```go
	if !c.Conditions.AddConditionScaled(conditionId, durationMult) {
		return fmt.Errorf(`failed to add condition. target: "%s" conditionId: %d`, c.Name, conditionId)
	}
	// Chunk 6 (Perception): see AddCondition above.
	if (conditionId == perception.ConditionIdBlinded || conditionId == perception.ConditionIdFlashbangBlindness) &&
		c.Perception != nil && c.Perception.State() == perception.Sighted {
		_ = c.Perception.TransitionTo(perception.Blinded,
			state.TransitionReason{Trigger: perception.TriggerConditionApplied, Metadata: map[string]any{"conditionId": conditionId}})
	}
	_ = c.Validate()
```
new_string:
```go
	if !c.Conditions.AddConditionScaled(conditionId, durationMult) {
		return fmt.Errorf(`failed to add condition. target: "%s" conditionId: %d`, c.Name, conditionId)
	}
	// Validate brings Perception in line with the blind sources.
	_ = c.Validate()
```

RemoveCondition, old_string:
```go
	c.Conditions.RemoveCondition(conditionId)
	// Chunk 6 (Perception): clearing a blind-source condition may flip
	// Blinded → Sighted, but only if no other blind source remains.
	if (conditionId == perception.ConditionIdBlinded || conditionId == perception.ConditionIdFlashbangBlindness) &&
		c.Perception != nil && c.Perception.State() == perception.Blinded && !c.HasAnyBlindSource() {
		_ = c.Perception.TransitionTo(perception.Sighted,
			state.TransitionReason{Trigger: perception.TriggerConditionExpired, Metadata: map[string]any{"conditionId": conditionId}})
	}
	_ = c.Validate()
```
new_string:
```go
	c.Conditions.RemoveCondition(conditionId)
	// Validate brings Perception in line with the blind sources that remain.
	_ = c.Validate()
```

Imports, old_string:
```go
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
)
```
new_string:
```go
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)
```
(`state` stays imported; the file still uses it at `:63` and `:120`.)

- [ ] **Step 5: Run the tests and see them pass**

```
go build ./...
go test ./internal/state/perception/ ./internal/characters/ -count=1
go test ./internal/hooks/ -run TestPruneConditions_ExpiredBlindnessRestoresSight -count=1
```
Expected: `ok` for all three (dry run: perception 0.022s, characters 4.818s, hooks 0.018s).

- [ ] **Step 6: Update the context.md files**

`internal/state/perception/context.md`, old_string:
```
Callers must check current state before firing transitions — the
inline guards in `Character.AddCondition` / `RemoveCondition` handle this.
```
new_string:
```
Callers must check current state before firing transitions. The one
production caller, `Character.reconcilePerception` (`internal/characters/sight.go`),
does: it fires only when the held blind sources and the state disagree.
```

old_string:
```
| Condition 3 (Blinded) | `_datafiles/world/dogmud/conditions/3-blinded.yaml` | `Character.AddCondition` / `RemoveCondition` |
| Condition 77 (Flashbang Blindness) | `_datafiles/world/dogmud/conditions/77-flashbang_blindness.yaml` | `Character.AddCondition` / `RemoveCondition` |
```
new_string:
```
| Condition 3 (Blinded) | `_datafiles/world/dogmud/conditions/3-blinded.yaml` | `Character.Validate` → `reconcilePerception` |
| Condition 77 (Flashbang Blindness) | `_datafiles/world/dogmud/conditions/77-flashbang_blindness.yaml` | `Character.Validate` → `reconcilePerception` |

`Validate` reconciles after it rebuilds the condition lookups, so every path
agrees: any add door (`AddCondition`, `AddConditionScaled`,
`AddConditionMagnitude`), `RemoveCondition`, the round's prune of a spent
record (`hooks.PruneConditions` calls `Prune` then `Validate`, never
`RemoveCondition`), a death or purge that expires records, and a load. The
machine is `yaml:"-"`, so a load starts Sighted; without the reconcile a relog
cured live blindness, and before 2026-10-08 natural expiry never restored
sight at all.
```

old_string:
```
returns true if either source is currently active. Used
by the `RemoveCondition` expire-path to decide whether to fire Blinded→Sighted
when one of two overlapping sources clears.
```
new_string:
```
returns true if either source is currently active. Used
by `reconcilePerception` to decide which way the state should point, so one of
two overlapping sources clearing leaves the holder Blinded.
```

old_string:
```
  single-source paths). PE-INT-006 was the condition-source case and was
  deleted with the enum.
```
new_string:
```
  single-source paths). PE-INT-006 was the condition-source case and was
  deleted with the enum. PE-INT-008 (natural expiry through `Prune` then
  `Validate`), PE-INT-009 (a reload keeps live blindness) and PE-INT-010 (the
  magnitude door blinds) pin the reconcile.
- `internal/hooks/blind_expiry_test.go`: the real round prune
  (`PruneConditions`) restores sight for conditions 3 and 77.
```

`internal/characters/context.md`, old_string:
```
currently active. Used
by the expire-path in `RemoveCondition` to determine
whether to fire `Blinded` back to `Sighted` when one of two overlapping
sources clears. Uses
```
new_string:
```
currently active. Used by `reconcilePerception` (same file), which
`Validate` calls right after `Conditions.Validate()`: it moves Perception to
`Blinded` when a source is held and back to `Sighted` when none is, so an
add on any door, `RemoveCondition`, the round's prune, a death or purge, and
a load all agree. Uses
```

- [ ] **Step 7: Gate**

```
gofmt -l internal/characters internal/state/perception internal/hooks/blind_expiry_test.go
go vet ./internal/characters/ ./internal/hooks/ ./internal/state/perception/
go test ./... -count=1 2>&1 | grep -v "^time=" | grep -E "^(--- FAIL|FAIL|panic)"
python tools/context_md_audit.py
```
Expected: `gofmt` prints nothing, `vet` is clean, the suite grep prints nothing (dry run: the full suite, the repo root included, passed; root `ok github.com/GoMudEngine/GoMud 53.005s`), and the audit reports no phantom under `internal/characters` or `internal/state/perception`. No guard test keys on `internal/characters/conditions.go` line numbers (`condition_apply_path_guard_test.go` walks callers outside that file; its only `internal/characters` note, line 104, names `skills.go`).

Many existing tests hand-flip `Perception.TransitionTo(perception.Blinded, ...)` with no condition (for example `internal/actions/speech_sight_test.go:82`, `internal/rooms/hiding_senders_test.go:28`). A later `Validate` would now flip them back. None did in the dry run. If one fails after a rebase, give that fixture the blind condition (`AddCondition(perception.ConditionIdBlinded, false)` with a seeded spec) instead of the bare transition. Production has no other `Perception.TransitionTo` caller.

- [ ] **Step 8: Commit**

```
git add internal/characters/sight.go internal/characters/validate.go internal/characters/conditions.go internal/characters/context.md internal/state/perception/integration_test.go internal/state/perception/context.md internal/hooks/blind_expiry_test.go
git commit -m "fix(sight): blindness ends when it runs out; Validate reconciles Perception (#382)

Only RemoveCondition restored sight, so a Blinded or Flashbang record that
expired in the round prune left the player blind until a relog, and a relog
cured blindness that was still live. Validate now reconciles Perception with
the held blind sources on every path; the three per-door flips are gone.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


### Task F4: a sneaker spotted on arrival is told

**Spec:** F4. A player who sneaks into a room and is caught reads "You slip into the room but something notices you.", the spotter named only as far as the MOVER's sight allows, by the same rule the `sneak` command's "You try to blend into the shadows but ..." uses (`SpottedLine`, #215). A mob mover gets nothing (`MobActor.SendText` is a no-op, `internal/actions/actor.go:23-25`).

**Files:**
- Modify: `internal/actions/sneak.go` (`SpottedLine` gains a shared body with an opening)
- Modify: `internal/actions/move.go` (`sneakerSpotted` returns the spotter; `EntryDetection` tells the mover)
- Test: `internal/actions/sneak_spotted_line_test.go` (append)

Facts (master `f508db20b`): `sneakerSpotted` returns `bool` (`move.go:293`); its only caller is `EntryDetection` (`move.go:204`); the comment at `move.go:209-211` relies on condition 9's end text, and `conditions/9-hidden.yaml` has no `end_actee`. `SpottedLine(sneaker, room, spotter)` is at `sneak.go:66`, callers `usercommands/skill.skullduggery.sneak.go:71` and three tests.

- [ ] **Step 1: Write the failing test.** Append to `internal/actions/sneak_spotted_line_test.go` (after the last `}` of `TestSpottedLine_MobSpotterKeepsTheMobTag`):

```go
// Sight gates playtest fixes, F4: a sneaker caught on ARRIVAL is told, as a
// sneaker caught by the sneak command is, the spotter named only as far as
// the mover's own sight allows. Before, the mover read nothing: its hide
// simply ended, and condition 9 carries no end line for its holder.
func TestEntryDetection_SpottedMoverIsTold(t *testing.T) {
	cases := []struct {
		name    string
		lamp    int
		infra   bool
		spotter string
		want    string
	}{
		{"lit room names a player spotter", 80, false, "player",
			"You slip into the room but Watcher notices you."},
		{"lit room names a mob spotter", 80, false, "mob",
			"You slip into the room but Watcher notices you."},
		{"heat sight in the dark sees a figure", 0, true, "player",
			"You slip into the room but a figure notices you."},
		{"pitch dark sees nothing", 0, false, "player",
			"You slip into the room but something notices you."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDarkDetectWorld(t, 0.75)
			w.dest.Lamp = rooms.LampPtr(c.lamp)
			id := 9811
			if c.spotter == "mob" {
				id = 9851
			}
			_, obs := w.place(t, c.spotter, id, "Watcher")
			obs.Stats.Perception.ValueAdj = 1000
			require.True(t, obs.Conditions.AddCondition(hearSuperCond, true), "hears even where it cannot see")
			mover, mc := w.place(t, "player", 9810, "Sneak")
			mc.Stats.Dexterity.ValueAdj = 0
			if c.infra {
				require.True(t, mc.Conditions.AddCondition(hearInfraCond, true))
			}
			hideForMove(t, mc)
			mc.SetMiscData(`sneaking`, true)
			hearLines(9810)

			got := EntryDetection(mover, w.dest, true)

			require.False(t, got.StillSneaking)
			require.Equal(t, []string{c.want}, hearLines(9810))
			hearLines(9811) // the observer's notice; drained so later tests start clean
		})
	}
}
```

The helpers (`newDarkDetectWorld`, `hearSuperCond`, `hearInfraCond`, `hearLines`) are in `sneak_hearing_test.go`; `hideForMove` and `place` in `move_detection_test.go`. No import changes.

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/actions/ -run TestEntryDetection_SpottedMoverIsTold -count=1`

Expected (dry run on `f508db20b`), all four subtests fail the same way:
```
--- FAIL: TestEntryDetection_SpottedMoverIsTold/lit_room_names_a_player_spotter (0.00s)
    sneak_spotted_line_test.go:147:
        Error:      	Not equal:
        	            	expected: []string{"You slip into the room but Watcher notices you."}
        	            	actual  : []string(nil)
```

- [ ] **Step 3: Give `SpottedLine` a shared body with an opening.** In `internal/actions/sneak.go`, Edit:

old_string:
```go
func SpottedLine(sneaker *characters.Character, room messaging.RoomVisibility, spotter *characters.Character) string {
	tag := `mobname`
	if spotter.GetUserId() > 0 {
		tag = `username`
	}
	line := `You try to blend into the shadows but <ansi fg="` + tag + `">` +
		spotter.Name + `</ansi> notices you.`
```
new_string:
```go
func SpottedLine(sneaker *characters.Character, room messaging.RoomVisibility, spotter *characters.Character) string {
	return spottedLineFrom(spottedOpeningHide, sneaker, room, spotter)
}

// The openings of the two spotted lines: the sneak command's attempt, and a
// sneaking arrival caught at the door (EntryDetection).
const (
	spottedOpeningHide   = `You try to blend into the shadows`
	spottedOpeningArrive = `You slip into the room`
)

// spottedLineFrom is SpottedLine with its opening, so the sneak command and a
// sneaking arrival name the spotter by one rule.
func spottedLineFrom(opening string, sneaker *characters.Character, room messaging.RoomVisibility, spotter *characters.Character) string {
	tag := `mobname`
	if spotter.GetUserId() > 0 {
		tag = `username`
	}
	line := opening + ` but <ansi fg="` + tag + `">` +
		spotter.Name + `</ansi> notices you.`
```

The rest of the old `SpottedLine` body (the sight and `Perceives` check, the `HideNames` return) stays as is and is now the tail of `spottedLineFrom`.

- [ ] **Step 4: `sneakerSpotted` returns the spotter.** In `internal/actions/move.go`, three Edits.

old_string:
```go
// light modifier.
func sneakerSpotted(mover Actor, dest *rooms.Room, light messaging.RoomVisibility) bool {
```
new_string:
```go
// light modifier. It returns the observer who caught the mover, or nil.
func sneakerSpotted(mover Actor, dest *rooms.Room, light messaging.RoomVisibility) *characters.Character {
```

old_string:
```go
				moverName(mover)+` slips into the room but you notice them.`, mover.GetName(), sight))
			return true
		}
```
new_string:
```go
				moverName(mover)+` slips into the room but you notice them.`, mover.GetName(), sight))
			return p.Character
		}
```

old_string:
```go
		observerScore, _ := sneakObserverScore(&m.Character, dest)
		if !combat.RunContest(sneakScore, []contest.Entry{{Score: observerScore}}).Success {
			return true
		}
	}

	return false
}
```
new_string:
```go
		observerScore, _ := sneakObserverScore(&m.Character, dest)
		if !combat.RunContest(sneakScore, []contest.Entry{{Score: observerScore}}).Success {
			return &m.Character
		}
	}

	return nil
}
```

- [ ] **Step 5: `EntryDetection` tells the mover.** In `internal/actions/move.go`, Edit:

old_string:
```go
	if sneaking && sneakerSpotted(mover, dest, light) {
		mc := mover.GetCharacter()
		// Drive the Awareness FSM out of Hidden; the mirror cascade in
		// Awareness_Cascades.go clears the Hidden condition. Calling
		// CancelConditionsWithFlag directly would expire the condition but
		// leave the FSM in Hidden. Silent to the mover: if the observer is
		// itself hidden, naming it leaks what the mover cannot see; the
		// Hidden condition's own end text is the signal.
		_ = mc.Awareness.TransitionToRevealing(
			state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
		mc.SetMiscData(`sneaking`, nil)
		sneaking = false
	}
```
new_string:
```go
	var spotter *characters.Character
	if sneaking {
		spotter = sneakerSpotted(mover, dest, light)
	}
	if spotter != nil {
		mc := mover.GetCharacter()
		// Drive the Awareness FSM out of Hidden; the mirror cascade in
		// Awareness_Cascades.go clears the Hidden condition. Calling
		// CancelConditionsWithFlag directly would expire the condition but
		// leave the FSM in Hidden.
		_ = mc.Awareness.TransitionToRevealing(
			state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
		mc.SetMiscData(`sneaking`, nil)
		sneaking = false
		// The mover is told, the spotter named only as far as the mover's
		// own sight allows, as the sneak command does (SpottedLine): a
		// hidden spotter reads "something". A mob mover's SendText is a
		// no-op. Sight gates playtest fixes, F4.
		mover.SendText(messaging.CategorySystem,
			spottedLineFrom(spottedOpeningArrive, mc, dest, spotter))
	}
```

`move.go` already imports `characters` and `messaging`.

- [ ] **Step 6: Run it and see it pass, then the package.**

Run: `go test ./internal/actions/ -run TestEntryDetection_SpottedMoverIsTold -count=1 -v`
Expected: `--- PASS` for all four subtests.

Run: `go test ./internal/actions/ ./internal/usercommands/ ./internal/mobcommands/ -count=1`
Expected: all `ok` (dry run: `internal/actions` 65s).

🪤 Do NOT gate on a filtered run such as `-run 'TestEntryDetection|TestSpottedLine|TestSneak'`: `TestSneak_BlindObserverRollsByEar` (`sneak_hearing_test.go:103`) fails under that filter ON MASTER too, because it asserts user 9811's queue is empty without draining it first and earlier move-detection tests leave lines there. The whole package passes.

- [ ] **Step 7: Commit.**

```bash
gofmt -l internal/actions
git add internal/actions/sneak.go internal/actions/move.go internal/actions/sneak_spotted_line_test.go
git commit -m "fix(sight): a sneaker caught on arrival is told who noticed, at its own sight (#382, #215)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task F5: a mob spawned hidden is hidden

**Spec:** F5. Record 9 added to a Visible character drives Awareness into Hidden (`TransitionToConcealing`, then `ResolveConcealment(true)`), guarded against the cascade's own re-add. Once anything reveals it, the spawn-time hide is spent.

**Files:**
- Modify: `internal/characters/conditions.go` (`AddCondition`, `AddConditionScaled`, new `hideForStealthRecord`)
- Modify: `internal/state/awareness/transitions.go` (new trigger constant)
- Modify: `internal/hooks/Awareness_Cascades.go` (strip the permanent id on leaving Hidden)
- Test: `internal/hooks/hidden_spawn_test.go` (new)

Facts (master `f508db20b`):
- `IsHidden()` reads only `c.Awareness` (`internal/characters/character.go:889-895`).
- Spawn: `mobs.go:659` `Validate()` builds the machines and wires the cascades, `:678` `SetPermanentConditions(mob.ConditionIds)`, `:773` `Validate(true)` runs `reapplyPermanentConditions` (`validate.go:718-720`), which calls `AddCondition(id, true)` (`characters/conditions.go`, `reapplyPermanentConditions`). `AddCondition` touches only Perception.
- The admin `setcondition` and every event apply land in `Condition_ApplyConditions.go:111` `targetChar.AddCondition(evt.ConditionId, false)`, so they get the same fix.
- The cascade (`hooks/Awareness_Cascades.go:44-63`) runs Awareness to record only: entering Hidden calls `AddCondition(9, true)` (line 57; the state machine sets the new state before after-hooks run, `internal/state/machine.go:115-116`), leaving Hidden calls `CancelConditionsWithFlag(conditions.Hidden)`.
- `CancelCombatConditions` already strips CancelIfCombat ids from `permanentConditionIds` (`characters/conditions.go:80-99`). `RemovePermanentCondition` exists (`:220`) and has NO callers, so a mob revealed any other way (`search.go:496`, `move.go:395`) keeps 9 in its permanent ids.
- 9 DOGMud mobs carry `conditionids` with 9 (grep `conditionids:.*\b9\b` under `_datafiles/world/dogmud/mobs`).
- 🪤 Condition 31 (Empathic Shroud, spell `empathic-shroud`) ALSO carries the `hidden` flag, with a 16-round duration. The fix is keyed on record 9, not on the flag: driving Awareness to Hidden for 31 would make the cascade pin a permanent 9 that outlives the shroud. Shroud stays inert to `IsHidden` exactly as on master (see the report).
- `condition_apply_path_guard_test.go` keys `internal/hooks/Awareness_Cascades.go|57`. The edit below adds lines AFTER line 61 only, so line 57 does not move.

- [ ] **Step 1: Write the failing test.** Create `internal/hooks/hidden_spawn_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Sight gates playtest fixes, F5: a mob spawned with condition 9 in its
// conditionids (an ambusher) is hidden. IsHidden reads only the Awareness
// machine, and the spawn path added record 9 without ever entering Hidden,
// so the playtest's Pale Lurker was listed in the room and its emotes
// showed. Adding record 9 to a Visible character now drives Awareness into
// Hidden; once anything reveals it, the spawn-time hide is spent.

// seedHiddenSpawnCondition seeds condition 9 with its shipped flags and end
// line (_datafiles/world/dogmud/conditions/9-hidden.yaml).
func seedHiddenSpawnCondition() func() {
	return conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9: {ConditionId: 9, Name: "Hidden",
			Flags:       []conditions.Flag{conditions.Hidden, conditions.CancelIfCombat},
			EndRoomText: "{actee_plain} emerges from the shadows."},
	})
}

// spawnHiddenSkeleton gives the Skeleton (instance 100, room 1) condition 9
// the way a spawn does: permanent ids, then Validate(true). User 2 reads
// faces in room 1.
func spawnHiddenSkeleton(t *testing.T) *mobs.Mob {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedHiddenSpawnCondition())
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Validate()
	m.Character.SetPermanentConditions([]int{9})
	m.Character.Validate(true)
	events.DrainQueuedMessagesForTest(2)
	return m
}

func TestHiddenSpawn_PermanentHiddenHidesTheMob(t *testing.T) {
	m := spawnHiddenSkeleton(t)

	require.True(t, m.Character.HasCondition(9))
	require.True(t, m.Character.IsHidden(), "condition 9 from conditionids must hide the mob")
	u := users.GetByUserId(2)
	require.NotNil(t, u)
	require.False(t, u.Character.Perceives(&m.Character), "a faces reader without see-hidden does not perceive it")
}

// Entering Hidden once: the sneak path (Concealing, then Hidden) re-adds
// record 9 through the mirror cascade, and that re-add must not try to hide
// an already Hidden character again.
func TestHiddenSpawn_EntersHiddenOnce(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedHiddenSpawnCondition())
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Validate()
	entries := 0
	m.Character.Awareness.Inner().AfterTransition("hidden_spawn_test_count",
		func(from, to awareness.State, r state.TransitionReason) {
			if to == awareness.Hidden {
				entries++
			}
		})

	hideMob(t, m)
	require.True(t, m.Character.HasCondition(9))
	require.Equal(t, 1, entries)

	m.Character.Validate(true)
	require.Equal(t, 1, entries, "a Validate on a hidden mob does not re-enter Hidden")
}

// Combat reveals a spawned-hidden mob for good: CancelCombatConditions
// strips 9 from the permanent ids, so a later Validate(true) does not hide it
// again. The end line then reads at the reader's sight of the now visible mob.
func TestHiddenSpawn_CombatRevealSticks(t *testing.T) {
	m := spawnHiddenSkeleton(t)
	require.True(t, m.Character.IsHidden())

	m.Character.RevealForCombat()
	require.False(t, m.Character.IsHidden())
	m.Character.Validate(true)
	require.False(t, m.Character.IsHidden(), "the spawn-time hide is spent once combat reveals it")

	PruneConditions(events.NewTurn{TurnNumber: 1})
	got := drainPlain(2)
	require.Equal(t, []string{"Skeleton emerges from the shadows."}, got,
		"once, naming the mob combat has already shown the room")
}

// Any other reveal (a search, a newcomer spotting it) spends it too: the
// permanent id would otherwise re-hide the mob at its next Validate(true).
func TestHiddenSpawn_SpottedStaysSpotted(t *testing.T) {
	m := spawnHiddenSkeleton(t)
	require.True(t, m.Character.IsHidden())

	require.NoError(t, m.Character.Awareness.TransitionToRevealing(
		state.TransitionReason{Trigger: awareness.TriggerObserverSearch}))
	require.False(t, m.Character.IsHidden())
	m.Character.Validate(true)
	require.False(t, m.Character.IsHidden(), "a spotted ambusher does not slip back into hiding")
}
```

`seedAllRegistries` (`hooks_test.go:57`), `hideMob` (`hidden_mob_room_lines_test.go`), `drainPlain` (`narration_testhelpers_test.go:28`) already exist in package `hooks`.

- [ ] **Step 2: Run it and see it fail.**

Run: `go test ./internal/hooks/ -run TestHiddenSpawn -count=1`

Expected (dry run on `f508db20b`):
```
--- FAIL: TestHiddenSpawn_PermanentHiddenHidesTheMob (0.00s)
    hidden_spawn_test.go:54:
        Error:      	Should be true
        Messages:   	condition 9 from conditionids must hide the mob
--- FAIL: TestHiddenSpawn_CombatRevealSticks (0.00s)
    hidden_spawn_test.go:90:
        Error:      	Should be true
--- FAIL: TestHiddenSpawn_SpottedStaysSpotted (0.00s)
    hidden_spawn_test.go:106:
        Error:      	Should be true
FAIL
```
`TestHiddenSpawn_EntersHiddenOnce` passes on master: it is the guard against a double transition once the fix lands.

- [ ] **Step 3: Add the trigger.** In `internal/state/awareness/transitions.go`, Edit:

old_string:
```go
	TriggerForceVisible       = "force_visible"
)
```
new_string:
```go
	TriggerForceVisible       = "force_visible"
	// TriggerConditionApplied is the stealth record (condition 9) added to a
	// Visible character by anything but the sneak command: a mob's
	// spawn-time conditionids, an admin setcondition.
	TriggerConditionApplied = "condition_applied"
)
```

- [ ] **Step 4: Record 9 hides.** In `internal/characters/conditions.go`, two Edits. The anchors are the function tails, not the Perception blocks, because Task F1 edits those blocks in the same functions.

In `AddCondition`, old_string:
```go
	_ = c.Validate()
	return nil
}

// AddConditionScaled adds a condition with its duration scaled by durationMult.
```
new_string:
```go
	c.hideForStealthRecord(conditionId)
	_ = c.Validate()
	return nil
}

// conditionIdHidden is the stealth record the Awareness machine mirrors
// (Awareness_Cascades.go): entering Hidden adds it, leaving Hidden cancels it.
const conditionIdHidden = 9

// hideForStealthRecord is the other half of that mirror. Record 9 added to a
// Visible character (a mob's spawn-time conditionids, an admin setcondition)
// drives Awareness into Hidden, so IsHidden agrees with the record. Before,
// only the sneak command entered Hidden, and an ambusher spawned with 9 was
// listed in the room and emoted (sight gates playtest fixes, F5). The
// cascade's own re-add arrives while the machine is already Hidden, and a
// sneak in flight is Concealing, so neither is driven twice.
//
// Only record 9: another hidden-flag record (Empathic Shroud, 31) is timed,
// and the cascade would pin a permanent 9 that outlives it.
func (c *Character) hideForStealthRecord(conditionId int) {
	if conditionId != conditionIdHidden || c.Awareness == nil || c.Awareness.State() != awareness.Visible {
		return
	}
	reason := state.TransitionReason{Trigger: awareness.TriggerConditionApplied}
	if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		return
	}
	c.Awareness.ResolveConcealment(true, reason)
}

// AddConditionScaled adds a condition with its duration scaled by durationMult.
```

In `AddConditionScaled`, old_string:
```go
	_ = c.Validate()
	return nil
}

// AddConditionMagnitude
```
new_string:
```go
	c.hideForStealthRecord(conditionId)
	_ = c.Validate()
	return nil
}

// AddConditionMagnitude
```

`conditions.go` already imports `state` and `awareness`.

- [ ] **Step 5: A reveal spends the spawn-time hide.** In `internal/hooks/Awareness_Cascades.go`, Edit:

old_string:
```go
				// Remove condition #9 via cancel-on-flag mechanism.
				c.CancelConditionsWithFlag(conditions.Hidden)
```
new_string:
```go
				// Remove condition #9 via cancel-on-flag mechanism.
				c.CancelConditionsWithFlag(conditions.Hidden)
				// A spawn-time hide (conditionids: [9]) is spent once
				// anything reveals the mob, or the next Validate(true)
				// would re-add 9 and, through AddCondition, hide it again
				// (sight gates playtest fixes, F5). CancelCombatConditions
				// already strips it on the combat path.
				c.RemovePermanentCondition(9)
```

Dry-run probe: with this line commented out, `TestHiddenSpawn_SpottedStaysSpotted` fails ("a spotted ambusher does not slip back into hiding"), so the test proves the line.

- [ ] **Step 6: Run it and see it pass, then the packages and the guards.**

Run: `go test ./internal/hooks/ -run TestHiddenSpawn -count=1 -v`
Expected: four `--- PASS`.

Run: `go test ./internal/hooks/ ./internal/characters/ ./internal/state/... ./internal/mobs/ ./internal/mobcommands/ ./internal/usercommands/ ./internal/actions/ ./internal/behaviortree/ ./internal/rooms/ ./modules/... -count=1`
Expected: all `ok`.

Run: `go test . -count=1`
Expected: `ok` (the condition-apply-path guard keys `Awareness_Cascades.go|57`, unmoved).

Dry run: F4 and F5 stacked on `f508db20b`, `go test ./... -count=1` printed no `FAIL`.

What this changes in play: the 9 DOGMud ambushers now start hidden, so they are off the roster, `look` and `SendSeen` treat them as unseen, and their emotes are silent (R3, the hidden check in `mobcommands/emote.go`). A mob with `conditionids: [9]` also moves sneaking (`actions.MobIsSneaking` reads `IsHidden`), so it now rolls the arrival contest. Combat reveals it and prints "Skeleton emerges from the shadows." once at the next prune, naming the mob the fight has already shown.

- [ ] **Step 7: Commit.**

```bash
gofmt -l internal/characters internal/hooks internal/state
git add internal/characters/conditions.go internal/state/awareness/transitions.go internal/hooks/Awareness_Cascades.go internal/hooks/hidden_spawn_test.go
git commit -m "fix(sight): a mob spawned with condition 9 is hidden, and a reveal spends it (#382)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

## F2: weapons are generic below full sight (ruling R8)

Owner ruling R8: a combat line names a weapon only at full sight; at shapes and at no sight it reads "weapon" ("their weapon", "the fumbled weapon"). Three tasks: the messaging pass and its pipeline stage (F2a), the participant lines (F2b), the one untagged weapon template (F2c).

**Design decisions, verified on master `f508db20b`:**

- **Two audiences, one function.** Spectators read combat lines on the visual channel: `drainSpectatorLines` (`internal/hooks/combat_verbosity.go:332`) calls `room.SendTextVisualToUser`, and every Trio observer seat (`messaging/trio.go:116`, `:120`) calls `SendTextVisualHidingNames` then `deliverVisual` (`rooms/rooms.go:451`). Both end in `messaging.RenderForRecipient`, whose `SightShapes` case (`pipeline.go`, Stage 4) is the one place every spectator combat line passes. `SightNone` visual lines are already dropped there. Participants read on the audio channel through `u.SendText` (`users/userrecord.go:500`, no sight decision), so their lines are hidden at composition in `hideIdentitiesInPersonalLines` (`combat/combat.go:743`), which already holds each reader's sight.
- **Combat categories only.** The pipeline stage runs for `isCombatNarration(cat)`: the six `CategoryHit*`, Dodge, Parry, Block, GrappleFlow, GrappleHigh, Submission, SurpriseAttack, Kick, Trip, Bash. A non-combat line keeps its item ("a figure picks up a Torch"), which a test pins.
- **Tags matched:** `fg="item"` (combat and defence templates: 1403 of 1454 `{itemname}` are inside it) and `fg="itemname"` (`usercommands/shoot.go:516` wraps the shoot weapon in it; `throw.yaml:15` wraps the thrown item in it). A tag body may carry nested tags: `items.displayNameFrom` (`items/items.go:502`) adds a quest-star prefix and an adjective-span suffix, so the closing tag is found by depth counting, not a regex.
- **Unarmed names stay.** `GetWaitMessages` (`combat.go:270`) and `sendDefenseMessages` (`combat_helpers.go:1355`) put the species `UnarmedName` (or "fists") into the same tagged token. `HideWeapons` skips "fists" and every `species.GetAllSpecies()` `UnarmedName` (40 values in DOGMud species YAML). `messaging` already depends on `species` transitively (`go list -deps ./internal/messaging`), and `species` does not import `messaging`, so the new direct import adds no cycle.
- **A participant keeps their own gear.** `HideWeapons` takes `keep`; the participant pass keeps `heldWeaponNames(reader)`: every arm from `Character.GetHandPairs()` (`characters/hand_slots.go:25`), as `DisplayName()` and `GetSpec().Name` (the `{weapon}`/`{attack}` value, `combat_helpers.go:1361`). Known limit: if both fighters hold the same kind of weapon, the other's stays named.
- **Articles.** The pipeline normalizes before the sight stage, and the normalizer only turns "a" into "an" (`normalize.go`, `aBeforeVowel`), so a spectator line arrives as "with an Iron Longsword". `HideWeapons` turns a preceding "an"/"An" back into "a"/"A". The dry run shows the real failure: the shapes spectator got `with an <ansi fg="item">Iron Longsword</ansi>`.
- **Not a weapon, not touched:** the `{attack}` in `defense-messages/defy.yaml` (37) and `quell.yaml` (42) is untagged and is a taunt or spell name (callers pass `"aimed shot"`, `"firebomb"`, spell and taunt names to `RenderChannelDefenceMessages`), so it stays untagged. `slam.yaml`'s 31 untagged `{itemname}` are natural attacks (no item has `subtype: slam`). The `{itemname}` header comments in `combat-messages/*.yaml` and the `{weapon}` comments at line 5 of `block.yaml`, `dodge.yaml` and `parry.yaml` are comments only.
- **Already safe below full sight:** wait-round personal lines are replaced with fixed dark lines (`hooks/NewRound_DoCombat_resolution.go:98-121`); shoot's personal lines carry no `{weapon}` (only `shoot.yaml:14,17,75,78`, all observer lines, which the code already wraps in `fg="itemname"`); the disarm personal lines name the actee's weapon to the disarmer (who has their hand on it) and to its owner, both known by touch, and stay named.
- **Thrown items:** a thrown item in a combat-category observer line reads "weapon" at shapes. That is deliberate: in a fight it is used as one.

### Task F2a: `messaging.HideWeapons` and the shapes spectator stage

**Files:**
- Create: `internal/messaging/hideweapons.go`
- Create: `internal/messaging/hideweapons_test.go`
- Modify: `internal/messaging/pipeline.go` (Stage 4, `SightShapes` case)
- Modify: `internal/messaging/context.md`

- [ ] **Step 1: Write the failing tests**

Create `internal/messaging/hideweapons_test.go`:

```go
package messaging

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/species"
)

// Spec F2 (ruling R8): a combat line names a weapon only at full sight.

const sword = `<ansi fg="item">Iron Longsword</ansi>`

func TestHideWeapons_FullSightUnchanged(t *testing.T) {
	in := `Kesh slashes you with their ` + sword + `!`
	if got := HideWeapons(in, SightFull, nil); got != in {
		t.Fatalf("full sight changed the line: %q", got)
	}
}

func TestHideWeapons_BelowFullSightReadsWeapon(t *testing.T) {
	for _, d := range []SightDecision{SightShapes, SightNone} {
		in := `something slashes you with their ` + sword + `!`
		got := HideWeapons(in, d, nil)
		want := `something slashes you with their <ansi fg="combat-anon">weapon</ansi>!`
		if got != want {
			t.Fatalf("sight %d:\n got %q\nwant %q", d, got, want)
		}
	}
}

func TestHideWeapons_ParryAttackToken(t *testing.T) {
	in := `You smoothly sweep aside the fumbled ` + sword + `!`
	got := HideWeapons(in, SightNone, nil)
	if strings.Contains(got, "Longsword") || !strings.Contains(got, "the fumbled <ansi fg=\"combat-anon\">weapon</ansi>!") {
		t.Fatalf("parry line kept the weapon: %q", got)
	}
}

func TestHideWeapons_ArticleAgreesWithWeapon(t *testing.T) {
	in := `A figure draws an <ansi fg="item">Ivory Dagger</ansi>.`
	got := HideWeapons(in, SightShapes, nil)
	want := `A figure draws a <ansi fg="combat-anon">weapon</ansi>.`
	if got != want {
		t.Fatalf("\n got %q\nwant %q", got, want)
	}
}

func TestHideWeapons_NestedDisplayNameTags(t *testing.T) {
	// DisplayName can carry a quest star before the name and an adjective
	// span after it, both inside the item tag.
	in := `something hits you with their <ansi fg="item"><ansi fg="questflag">★</ansi>Iron Longsword <ansi fg="black-bold">(cursed)</ansi></ansi> hard.`
	got := HideWeapons(in, SightNone, nil)
	want := `something hits you with their <ansi fg="combat-anon">weapon</ansi> hard.`
	if got != want {
		t.Fatalf("\n got %q\nwant %q", got, want)
	}
}

func TestHideWeapons_ShootItemnameTag(t *testing.T) {
	in := `A figure fires their <ansi fg="itemname">Longbow</ansi> at a figure!`
	got := HideWeapons(in, SightShapes, nil)
	if strings.Contains(got, "Longbow") {
		t.Fatalf("fg=itemname weapon survived: %q", got)
	}
}

func TestHideWeapons_UnarmedNamesStay(t *testing.T) {
	restore := species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "wolf", UnarmedName: "fangs"},
	})
	defer restore()
	for _, natural := range []string{"fists", "fangs", "Fangs"} {
		in := `something bites you with their <ansi fg="item">` + natural + `</ansi>!`
		if got := HideWeapons(in, SightNone, nil); got != in {
			t.Fatalf("natural weapon %q was hidden: %q", natural, got)
		}
	}
}

func TestHideWeapons_KeepsTheReadersOwnWeapon(t *testing.T) {
	in := `You slash something with your ` + sword + `, and it parries with their <ansi fg="item">Buckler Blade</ansi>.`
	got := HideWeapons(in, SightNone, []string{`<ansi fg="item">Iron Longsword</ansi>`})
	if !strings.Contains(got, "Iron Longsword") {
		t.Fatalf("reader's own weapon was hidden: %q", got)
	}
	if strings.Contains(got, "Buckler Blade") {
		t.Fatalf("the other party's weapon survived: %q", got)
	}
}

func TestHideWeapons_SentenceStartCapitalised(t *testing.T) {
	in := `The blow lands. ` + sword + ` bites deep.`
	got := HideWeapons(in, SightShapes, nil)
	if !strings.Contains(got, `<ansi fg="combat-anon">Weapon</ansi> bites deep.`) {
		t.Fatalf("sentence-start word not capitalised: %q", got)
	}
}

// The pipeline hides weapons from a shapes-only spectator of a combat line,
// after its a/an stage has already agreed the article with the real name.
func TestRenderForRecipient_ShapesSpectatorReadsWeapon(t *testing.T) {
	got := RenderForRecipient(RenderInput{
		Category:      CategoryHitMelee,
		Text:          `*** <ansi fg="mobname">Kesh</ansi> DEVASTATES <ansi fg="username">Sil</ansi> with a <ansi fg="item">Iron Longsword</ansi>! ***`,
		Channel:       ChannelVisual,
		SightDecision: SightShapes,
		LineWidth:     200,
	})
	if strings.Contains(got, "Longsword") {
		t.Fatalf("shapes spectator read the weapon: %q", got)
	}
	if !strings.Contains(got, "with a <ansi fg=\"combat-anon\">weapon</ansi>") {
		t.Fatalf("want \"with a weapon\" (never \"an weapon\"): %q", got)
	}
}

// A non-combat line keeps its item at shapes: R8 covers combat lines only.
func TestRenderForRecipient_NonCombatItemUntouched(t *testing.T) {
	got := RenderForRecipient(RenderInput{
		Category:      CategoryDefault,
		Text:          `<ansi fg="mobname">Kesh</ansi> picks up a <ansi fg="item">Torch</ansi>.`,
		Channel:       ChannelVisual,
		SightDecision: SightShapes,
		LineWidth:     200,
	})
	if !strings.Contains(got, "Torch") {
		t.Fatalf("non-combat item hidden: %q", got)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```
go test ./internal/messaging/ -run 'HideWeapons|ShapesSpectatorReadsWeapon|NonCombatItemUntouched' -count=1
```
Expected (dry run on `f508db20b`): build failure, `internal\messaging\hideweapons_test.go:16:12: undefined: HideWeapons`, repeated for each call.

- [ ] **Step 3: Write `internal/messaging/hideweapons.go`**

```go
package messaging

import (
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/species"
)

// WeaponWord is what a reader below full sight reads in place of a weapon's
// name in a combat line (owner ruling R8, 2026-10-08): "something slashes you
// with their weapon", "the fumbled weapon".
const WeaponWord = "weapon"

// itemTagOpen matches the opening tag of an item name in narration. Combat
// templates wrap {itemname}, {weapon} and {attack} in fg="item"; the shoot
// verb wraps its weapon in fg="itemname" (usercommands/shoot.go).
var itemTagOpen = regexp.MustCompile(`<ansi fg="item(?:name)?">`)

// anyAnsiTag matches any opening or closing ansi tag, for reading a tag body
// as plain text.
var anyAnsiTag = regexp.MustCompile(`<ansi[^>]*>|</ansi>`)

// HideWeapons hides the weapons in a combat line from a reader at d. Below
// SightFull every item-tagged name becomes WeaponWord, except:
//
//   - a natural weapon: "fists" or any species' UnarmedName. Templates tag
//     {itemname} whether it holds an item or a body part, and a body part is
//     not a weapon a reader learns anything from (spec F2: unarmed names stay);
//   - a name in keep, which a participant passes for their own gear: the
//     reader knows what is in their own hand, as `look` by touch does (#218).
//
// A preceding "an" becomes "a", because the pipeline's a/an stage runs
// before the sight stage and has already agreed the article with the real
// name ("an Iron Longsword").
//
// Clear sight returns text unchanged. Untagged item names are not touched;
// spec F2 tags the combat templates that left a weapon token bare.
func HideWeapons(text string, d SightDecision, keep []string) string {
	if d == SightFull || text == "" || !strings.Contains(text, `<ansi fg="item`) {
		return text
	}
	var b strings.Builder
	last := 0
	for _, loc := range itemTagOpen.FindAllStringIndex(text, -1) {
		if loc[0] < last {
			continue // nested inside a tag already replaced
		}
		end := closingAnsiEnd(text, loc[1])
		if end < 0 {
			continue
		}
		plain := strings.TrimSpace(anyAnsiTag.ReplaceAllString(text[loc[1]:end], ""))
		if plain == "" || isNaturalWeapon(plain) || namedIn(plain, keep) {
			continue
		}
		word := WeaponWord
		if atSentenceStart(text, loc[0]) {
			word = strings.ToUpper(word[:1]) + word[1:]
		}
		b.WriteString(articleBeforeConsonant(text[last:loc[0]]))
		b.WriteString(`<ansi fg="combat-anon">` + word + `</ansi>`)
		last = end
	}
	if last == 0 {
		return text
	}
	b.WriteString(text[last:])
	return b.String()
}

// closingAnsiEnd returns the index just past the </ansi> that closes a tag
// whose opening ends at from, counting nested ansi tags (a display name can
// carry a quest star or an adjective span inside its item tag). It returns -1
// for an unclosed tag.
func closingAnsiEnd(text string, from int) int {
	depth := 1
	for i := from; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "</ansi>"):
			depth--
			i += len("</ansi>")
			if depth == 0 {
				return i
			}
		case strings.HasPrefix(text[i:], "<ansi"):
			depth++
			i += len("<ansi")
		default:
			i++
		}
	}
	return -1
}

// articleBeforeConsonant rewrites a trailing article "an " or "An " to "a "
// or "A ", for the consonant of WeaponWord.
func articleBeforeConsonant(prefix string) string {
	for _, art := range []string{"an ", "An "} {
		if !strings.HasSuffix(prefix, art) {
			continue
		}
		start := len(prefix) - len(art)
		if start > 0 {
			c := prefix[start-1]
			if c != ' ' && c != '>' && c != '\n' {
				return prefix // "Elan " is not an article
			}
		}
		return prefix[:start] + art[:1] + " "
	}
	return prefix
}

// isNaturalWeapon reports whether plain is a body part rather than an item:
// the "fists" default combat falls back to, or a species' UnarmedName.
func isNaturalWeapon(plain string) bool {
	if strings.EqualFold(plain, "fists") {
		return true
	}
	for _, s := range species.GetAllSpecies() {
		if s.UnarmedName != "" && strings.EqualFold(plain, s.UnarmedName) {
			return true
		}
	}
	return false
}

// namedIn reports whether plain matches one of names, ignoring case and tags.
func namedIn(plain string, names []string) bool {
	for _, n := range names {
		if n != "" && strings.EqualFold(plain, strings.TrimSpace(anyAnsiTag.ReplaceAllString(n, ""))) {
			return true
		}
	}
	return false
}

// isCombatNarration reports whether cat narrates a fight: swings, defences,
// grapples and the special-move verbs. The pipeline hides weapons from a
// shapes-only reader of these categories only (spec F2: combat paths).
func isCombatNarration(cat Category) bool {
	switch cat {
	case CategoryHitMelee, CategoryHitBlunt, CategoryHitNaturalSharp,
		CategoryHitRanged, CategoryHitCaster, CategoryHitUnarmed,
		CategoryDodge, CategoryParry, CategoryBlock,
		CategoryGrappleFlow, CategoryGrappleHigh, CategorySubmission,
		CategorySurpriseAttack, CategoryKick, CategoryTrip, CategoryBash:
		return true
	}
	return false
}
```

- [ ] **Step 4: Run again; only the pipeline test fails**

Same command. Expected: every `TestHideWeapons_*` and `TestRenderForRecipient_NonCombatItemUntouched` PASS; `TestRenderForRecipient_ShapesSpectatorReadsWeapon` FAILs (dry run):

```
hideweapons_test.go:111: shapes spectator read the weapon: "<ansi fg=\"hit-melee\">*** <ansi fg=\"combat-anon\">A figure</ansi> DEVASTATES <ansi fg=\"combat-anon\">a figure</ansi> with an <ansi fg=\"item\">Iron Longsword</ansi>! ***</ansi>"
```

Note the "an": the normalizer already agreed the article with "Iron", which is why `HideWeapons` fixes it.

- [ ] **Step 5: Wire the pipeline stage**

Edit `internal/messaging/pipeline.go`. old_string:

```go
			// Stage 4: anonymize (stubbed; T6 lands the implementation).
			text = anonymize(text)
```

new_string:

```go
			// Stage 4: anonymize (stubbed; T6 lands the implementation).
			text = anonymize(text)
			// A shapes-only spectator of a fight sees no weapon's name
			// (ruling R8). Every spectator combat line reaches here: the
			// spectator drain and the Trio observer seats both render
			// through this stage.
			if isCombatNarration(in.Category) {
				text = HideWeapons(text, SightShapes, nil)
			}
```

- [ ] **Step 6: Run the package**

```
go test ./internal/messaging/ -count=1
```
Expected: `ok` (dry run: `ok github.com/GoMudEngine/GoMud/internal/messaging 0.025s`).

- [ ] **Step 7: context.md**

Edit `internal/messaging/context.md`. old_string:

```
  survive as "a figure (dead)".
- `WrapAnsi(text string, maxWidth int) string`
```

new_string:

```
  survive as "a figure (dead)".
- `HideWeapons(text string, d SightDecision, keep []string) string`
  (`hideweapons.go`, owner ruling R8, 2026-10-08): below `SightFull` every
  `fg="item"` or `fg="itemname"` tag in a combat line becomes `WeaponWord`
  ("weapon"), nested display-name tags included, with "an" before it turned
  to "a". It keeps a natural weapon ("fists", any species' `UnarmedName`)
  and any name in `keep` (a participant's own held gear). The pipeline runs
  it at `SightShapes` for `isCombatNarration` categories only, so every
  spectator combat line is covered and a non-combat item line is not;
  `combat.hideIdentitiesInPersonalLines` runs it on the personal lines.
  Untagged item names pass through, which is why the combat templates tag
  every weapon token.
- `WrapAnsi(text string, maxWidth int) string`
```

- [ ] **Step 8: gofmt and commit**

```
gofmt -l internal/messaging
git add internal/messaging/hideweapons.go internal/messaging/hideweapons_test.go internal/messaging/pipeline.go internal/messaging/context.md
git commit -m "fix(sight): a shapes-only spectator reads \"weapon\", not the weapon's name (#382)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
`gofmt -l` prints nothing.

### Task F2b: participant lines name only the reader's own weapon

**Files:**
- Create: `internal/combat/darkness_weapon_hiding_test.go`
- Modify: `internal/combat/combat.go` (`hideIdentitiesInPersonalLines`, plus new `heldWeaponNames`)
- Modify: `internal/combat/context.md`

Depends on F2a (`messaging.HideWeapons`).

- [ ] **Step 1: Write the failing test**

Create `internal/combat/darkness_weapon_hiding_test.go`:

```go
package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// Spec F2 (ruling R8), the participant half: a reader below full sight does
// not learn the OTHER party's weapon, and keeps their own. Lines below are
// the shapes the playtest quoted (run 6b3b017287df1ded): the defender's hit
// line, and the parry line whose {attack} is the attacker's weapon.
func TestHideIdentitiesInPersonalLines_HidesTheOtherPartysWeapon(t *testing.T) {
	atk := characters.New()
	atk.Name = "Ordel"
	atk.Equipment.Weapon = items.Item{ItemId: 999901, Spec: &items.ItemSpec{Name: "iron longsword"}}
	def := characters.New()
	def.Name = "Fold"
	def.Equipment.Weapon = items.Item{ItemId: 999902, Spec: &items.ItemSpec{Name: "oak staff"}}

	build := func() *AttackResult {
		return &AttackResult{
			MessagesToSource: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: `You slash <ansi fg="username">Fold</ansi> with your <ansi fg="item">Iron Longsword</ansi>!`},
				{Category: messaging.CategoryParry, Text: `<ansi fg="username">Fold</ansi> turns your blow aside with their <ansi fg="item">Oak Staff</ansi>.`},
			},
			MessagesToTarget: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: `In a sweeping motion, <ansi fg="username">Ordel</ansi> slashes you with their <ansi fg="item">Iron Longsword</ansi>!`},
				{Category: messaging.CategoryParry, Text: `You smoothly sweep aside the fumbled <ansi fg="item">Iron Longsword</ansi>!`},
			},
		}
	}

	t.Run("full sight: every weapon named", func(t *testing.T) {
		res := build()
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{sourceSight: messaging.SightFull, targetSight: messaging.SightFull})
		if !strings.Contains(res.MessagesToTarget[0].Text, "Iron Longsword") || !strings.Contains(res.MessagesToSource[1].Text, "Oak Staff") {
			t.Fatalf("full sight lost a weapon name: %q / %q", res.MessagesToTarget[0].Text, res.MessagesToSource[1].Text)
		}
	})

	for _, d := range []messaging.SightDecision{messaging.SightShapes, messaging.SightNone} {
		res := build()
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{sourceSight: d, targetSight: d})

		for _, m := range res.MessagesToTarget {
			if strings.Contains(m.Text, "Longsword") {
				t.Fatalf("sight %d: defender learned the attacker's weapon: %q", d, m.Text)
			}
			if !strings.Contains(m.Text, "weapon") {
				t.Fatalf("sight %d: defender line lacks the generic word: %q", d, m.Text)
			}
		}
		if !strings.Contains(res.MessagesToSource[0].Text, "your <ansi fg=\"item\">Iron Longsword</ansi>") {
			t.Fatalf("sight %d: attacker lost the name of their own weapon: %q", d, res.MessagesToSource[0].Text)
		}
		if strings.Contains(res.MessagesToSource[1].Text, "Oak Staff") {
			t.Fatalf("sight %d: attacker learned the defender's weapon: %q", d, res.MessagesToSource[1].Text)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```
go test ./internal/combat/ -run TestHideIdentitiesInPersonalLines_HidesTheOtherPartysWeapon -count=1
```
Expected (dry run, F2a in place):

```
--- FAIL: TestHideIdentitiesInPersonalLines_HidesTheOtherPartysWeapon (0.00s)
    darkness_weapon_hiding_test.go:51: sight 1: defender learned the attacker's weapon: "In a sweeping motion, <ansi fg=\"combat-anon\">a figure</ansi> slashes you with their <ansi fg=\"item\">Iron Longsword</ansi>!"
```

- [ ] **Step 3: Implement**

Edit `internal/combat/combat.go`. old_string:

```go
func hideIdentitiesInPersonalLines(result *AttackResult, sourceChar, targetChar *characters.Character, ctx combatContext) {
	for i := range result.MessagesToSource {
		result.MessagesToSource[i].Text = messaging.HideNames(result.MessagesToSource[i].Text, []string{targetChar.Name}, ctx.sourceSight)
	}
	targetHides := []string{sourceChar.Name}
	if sourceChar.Pet.Exists() {
		targetHides = append(targetHides, sourceChar.Pet.PlainName())
	}
	for i := range result.MessagesToTarget {
		result.MessagesToTarget[i].Text = messaging.HideNames(result.MessagesToTarget[i].Text, targetHides, ctx.targetSight)
	}
}
```

new_string (the two leading `//` lines continue the function's existing doc comment, which ends just above with "...player has actually bonded a pet."):

```go
//
// Weapons follow the same rule (spec F2, ruling R8): below full sight a
// reader's line names no weapon but their own, which they hold.
func hideIdentitiesInPersonalLines(result *AttackResult, sourceChar, targetChar *characters.Character, ctx combatContext) {
	sourceKeeps := heldWeaponNames(sourceChar)
	for i := range result.MessagesToSource {
		text := messaging.HideNames(result.MessagesToSource[i].Text, []string{targetChar.Name}, ctx.sourceSight)
		result.MessagesToSource[i].Text = messaging.HideWeapons(text, ctx.sourceSight, sourceKeeps)
	}
	targetHides := []string{sourceChar.Name}
	if sourceChar.Pet.Exists() {
		targetHides = append(targetHides, sourceChar.Pet.PlainName())
	}
	targetKeeps := heldWeaponNames(targetChar)
	for i := range result.MessagesToTarget {
		text := messaging.HideNames(result.MessagesToTarget[i].Text, targetHides, ctx.targetSight)
		result.MessagesToTarget[i].Text = messaging.HideWeapons(text, ctx.targetSight, targetKeeps)
	}
}

// heldWeaponNames lists every name a combat line can print for what c holds
// in any arm: DisplayName for {itemname}, the spec Name for {weapon} and
// {attack}. messaging.HideWeapons keeps these on c's own lines.
func heldWeaponNames(c *characters.Character) []string {
	var names []string
	for _, pair := range c.GetHandPairs() {
		for _, slot := range []characters.HandSlot{pair.First, pair.Second} {
			if slot.ItemPtr == nil || slot.ItemPtr.ItemId < 1 {
				continue
			}
			names = append(names, slot.ItemPtr.DisplayName(), slot.ItemPtr.GetSpec().Name)
		}
	}
	return names
}
```

- [ ] **Step 4: Run the seam tests**

```
go test ./internal/combat/ -run 'TestHideIdentitiesInPersonalLines' -count=1 -v
```
Expected: `TestHideIdentitiesInPersonalLines_UnitSeam`, `_HidesAttackersPetFromBlindDefender` and `_HidesTheOtherPartysWeapon` all PASS.

- [ ] **Step 5: context.md**

Edit `internal/combat/context.md`. old_string:

```
### Personal-line identity hiding (M4d PR 2)
```

new_string:

```
### Personal-line identity hiding (M4d PR 2)

Weapons too, since the sight-gates playtest fixes (spec F2, ruling R8):
after hiding names, each side's lines go through `messaging.HideWeapons`
at that reader's sight, keeping `heldWeaponNames(reader)` (every arm's
`DisplayName` and spec `Name`), so a reader below full sight learns no
weapon but their own.
```

- [ ] **Step 6: gofmt and commit**

```
gofmt -l internal/combat
git add internal/combat/combat.go internal/combat/darkness_weapon_hiding_test.go internal/combat/context.md
git commit -m "fix(sight): a fighter below full sight learns no weapon but their own (#382)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task F2c: the disarm lines tag their weapon

**Files:**
- Modify: `_datafiles/world/dogmud/narration/special-moves/grapple.yaml` (the `disarm` event, three lines)
- Modify: `internal/combat/grapple_narration_pin_test.go` (the `disarm` subtest)

The disarm observer line goes out as `CategoryGrappleFlow` through a Trio observer seat (`usercommands/grapple.go:148`, `mobcommands/grapple.go:81`), so F2a's pipeline stage hides it once the token is tagged. It is the only untagged `{weapon}` in a combat or move template (grep `\{weapon\}` over `_datafiles/world/dogmud`, minus tagged and comment lines; shoot's is tagged by the code). The two personal lines are tagged too so the event reads alike; they stay named (touch).

- [ ] **Step 1: Update the pin to the tagged lines (failing test)**

Edit `internal/combat/grapple_narration_pin_test.go`. old_string:

```go
		wantMessage := `<ansi fg="yellow-bold">You disarm ` + targetName + `, loosening their grip on their ` + weaponName + `!</ansi>`
		wantTargetMsg := `<ansi fg="red-bold">` + sourceName + ` disarms you! Your ` + weaponName + ` slips from your grasp!</ansi>`
		wantRoomMessage := `<ansi fg="combat">` + sourceName + ` disarms ` + targetName + `, knocking their ` + weaponName + ` loose!</ansi>`
```

new_string:

```go
		// {weapon} sits in an item tag (spec F2) so the pipeline can hide it
		// from a spectator who sees only shapes.
		taggedWeapon := `<ansi fg="item">` + weaponName + `</ansi>`
		wantMessage := `<ansi fg="yellow-bold">You disarm ` + targetName + `, loosening their grip on their ` + taggedWeapon + `!</ansi>`
		wantTargetMsg := `<ansi fg="red-bold">` + sourceName + ` disarms you! Your ` + taggedWeapon + ` slips from your grasp!</ansi>`
		wantRoomMessage := `<ansi fg="combat">` + sourceName + ` disarms ` + targetName + `, knocking their ` + taggedWeapon + ` loose!</ansi>`
```

- [ ] **Step 2: Run to verify it fails**

```
go test ./internal/combat/ -run TestGrappleNarrationPinnedToOriginalLiterals -count=1
```
Expected (dry run):

```
--- FAIL: TestGrappleNarrationPinnedToOriginalLiterals/disarm (0.00s)
    expected: "<ansi fg=\"yellow-bold\">You disarm Victim, loosening their grip on their <ansi fg=\"item\">rusty dagger</ansi>!</ansi>"
    actual  : "<ansi fg=\"yellow-bold\">You disarm Victim, loosening their grip on their rusty dagger!</ansi>"
```

- [ ] **Step 3: Tag the three lines (Edit tool)**

Edit `_datafiles/world/dogmud/narration/special-moves/grapple.yaml`. old_string:

```yaml
      - '<ansi fg="yellow-bold">You disarm {actee_plain}, loosening their grip on their {weapon}!</ansi>'
    actee:
      - '<ansi fg="red-bold">{actor_plain} disarms you! Your {weapon} slips from your grasp!</ansi>'
    observer:
      - '<ansi fg="combat">{actor_plain} disarms {actee_plain}, knocking their {weapon} loose!</ansi>'
```

new_string:

```yaml
      - '<ansi fg="yellow-bold">You disarm {actee_plain}, loosening their grip on their <ansi fg="item">{weapon}</ansi>!</ansi>'
    actee:
      - '<ansi fg="red-bold">{actor_plain} disarms you! Your <ansi fg="item">{weapon}</ansi> slips from your grasp!</ansi>'
    observer:
      - '<ansi fg="combat">{actor_plain} disarms {actee_plain}, knocking their <ansi fg="item">{weapon}</ansi> loose!</ansi>'
```

- [ ] **Step 4: Run it and the F2 gate**

```
go test ./internal/combat/ -run TestGrappleNarrationPinnedToOriginalLiterals -count=1
go build ./... && go vet ./internal/messaging ./internal/combat
go test ./internal/messaging/... ./internal/combat/... ./internal/hooks/... ./internal/rooms/... ./internal/mobcommands/... ./internal/usercommands/... ./internal/movenarration/... . -count=1 2>&1 | grep -v "^time=" | grep -E "^(--- FAIL|FAIL|panic|ok)"
```
Expected: the pin test `ok`, build and vet clean, every package `ok`. Dry run with F2a, F2b and F2c stacked on `f508db20b`:

```
ok  	github.com/GoMudEngine/GoMud/internal/messaging	0.031s
ok  	github.com/GoMudEngine/GoMud/internal/combat	20.078s
ok  	github.com/GoMudEngine/GoMud/internal/hooks	3.882s
ok  	github.com/GoMudEngine/GoMud/internal/rooms	6.801s
ok  	github.com/GoMudEngine/GoMud/internal/mobcommands	0.062s
ok  	github.com/GoMudEngine/GoMud/internal/usercommands	1.489s
ok  	github.com/GoMudEngine/GoMud/internal/movenarration	0.016s
ok  	github.com/GoMudEngine/GoMud	38.945s
```
No goldens needed re-recording. `python tools/context_md_audit.py` reports nothing for `messaging` or `combat`.

- [ ] **Step 5: Commit**

```
git add _datafiles/world/dogmud/narration/special-moves/grapple.yaml internal/combat/grapple_narration_pin_test.go
git commit -m "fix(sight): the disarm lines tag their weapon so a shapes spectator reads \"weapon\" (#382)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

## F3. Every disruption is heard (R4)

Owner ruling R4 (#242): a spell disruption (a break, an interrupt, a fizzle, a falter) has a sound line for a reader who sees nothing; the quiet weave and the focus shift stay sight-only. The close-out wired that for the mob spell channel and the player's prone, grapple and hit breaks. The playtest found the rest: flee breaks, the boss-interrupt spell, throw and throttle interrupts were visual only, and a player's fizzle, falter and bleed-out break told the room nothing at all.

Shape of the fix, with no new name machinery:

- The two sound lines move from unexported `hooks` constants to `messaging.SoundChantBreaksOff` and `messaging.SoundSpellSputtersOut`, so `usercommands` and `mobcommands` can share them.
- `rooms.Room.SendTextUnsighted(cat, txt, excludeUserIds...)` is the sound half on its own: the audio channel, to every player at `SightNone`. `hooks.sendVisualElseAudible` delegates to it instead of looping by hand.
- `messaging.Trio` gains `ObserverSound Line`, delivered by `SendTrio` through a new `Broadcaster.SendTextUnsighted` with the trio's own exclusions. `usercommands.moveCategories` gains `ObserverSound string`, which `sendMoveEvent` passes through. The only `Broadcaster` implementers are `*rooms.Room` (`internal/rooms/rooms.go:356`) and the test fake in `internal/messaging/trio_test.go:32`; grep `) ParticipantSight(` finds exactly those two.
- Flee uses the existing `Room.SendTextVisualWithAudio` (`internal/rooms/rooms.go:490`) and the shared wording "X's concentration breaks." instead of "X breaks their concentration.", in `CategorySpellDisruption` instead of `CategoryMobEmote`.

Dry-run facts (master `f508db20b`):

- `sendMoveEvent` must keep its name at every call site: `move_narration_migration_guard_test.go:408` collects referenced `(verb, event)` pairs from calls named `sendMoveEvent`/`renderMoveEvent`. That is why the throw/throttle sound rides on `moveCategories` rather than on a new helper.
- No root guard keys on the changed literals. `messaging_surface_guard_test.go` registers only the flee's self line ("You lose your concentration as you flee!", line 1411), which does not change. `go test . -count=1` passed with every F3 task applied.
- `internal/mobcommands/predator_test.go:261` asserts the old flee wording; F3c updates it.
- The playtest's "throw interrupt" sends through `sendMoveEvent` with `sameMoveCategory(messaging.CategorySpellDisruption)` inline (`internal/usercommands/throw.go:384-387`); F3d names that value `throwCastInterruptCategories` so the test sends exactly what production sends.

### Task F3a: shared sound lines, `Room.SendTextUnsighted`, `Trio.ObserverSound`

**Files:**
- Create: `internal/messaging/disruption_sounds.go`
- Modify: `internal/messaging/trio.go`, `internal/rooms/rooms.go`, `internal/hooks/NewRound_DoCombat_helpers.go`, `internal/hooks/spell_channel_sight_test.go`, `internal/messaging/context.md`, `internal/rooms/context.md`
- Test: `internal/messaging/trio_test.go`, create `internal/rooms/send_unsighted_test.go`

- [ ] **Step 1: Write the failing tests**

In `internal/messaging/trio_test.go`, give the fake broadcaster the new method. Edit:

old_string:
```go
	excl  []int
	sight map[int]SightDecision
}

func (f *fakeBroadcaster) SendTextVisualHidingNames(cat Category, txt string, names []string, excludeUserIds ...int) {
	f.calls++
	f.cat = cat
	f.text = txt
	f.names = append([]string(nil), names...)
	f.excl = append([]int(nil), excludeUserIds...)
}
```
new_string:
```go
	excl  []int
	sight map[int]SightDecision

	soundCalls int
	soundCat   Category
	soundText  string
	soundExcl  []int
}

func (f *fakeBroadcaster) SendTextVisualHidingNames(cat Category, txt string, names []string, excludeUserIds ...int) {
	f.calls++
	f.cat = cat
	f.text = txt
	f.names = append([]string(nil), names...)
	f.excl = append([]int(nil), excludeUserIds...)
}

func (f *fakeBroadcaster) SendTextUnsighted(cat Category, txt string, excludeUserIds ...int) {
	f.soundCalls++
	f.soundCat = cat
	f.soundText = txt
	f.soundExcl = append([]int(nil), excludeUserIds...)
}
```

Then add two tests. Edit:

old_string:
```go
func TestSendTrioNilRoomIsSafe(t *testing.T) {
```
new_string:
```go
// #242, owner ruling R4: a disruption is heard as well as seen. A trio's
// ObserverSound goes to the room's readers who see nothing, with the same
// exclusions as the observer line, so the actor and actee never read it.
func TestSendTrioDeliversTheObserverSoundToTheUnsighted(t *testing.T) {
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:         Say(CategorySystem, "a"),
		Observer:      Say(CategorySpellDisruption, "b's spell collapses!"),
		ObserverSound: Say(CategorySpellDisruption, SoundChantBreaksOff),
	}, Audience{Actor: &fakeRecipient{}, ActorId: 7, ActeeId: 9, Room: room})

	if room.soundCalls != 1 || room.soundText != SoundChantBreaksOff || room.soundCat != CategorySpellDisruption {
		t.Fatalf("sound: %d calls, %q, %v", room.soundCalls, room.soundText, room.soundCat)
	}
	if len(room.soundExcl) != 2 || room.soundExcl[0] != 7 || room.soundExcl[1] != 9 {
		t.Fatalf("sound exclusions = %v, want [7 9]", room.soundExcl)
	}
}

func TestSendTrioWithoutObserverSoundIsSilentToTheUnsighted(t *testing.T) {
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "a"),
		Observer: Say(CategoryBash, "c"),
	}, Audience{Actor: &fakeRecipient{}, ActorId: 7, Room: room})

	if room.soundCalls != 0 {
		t.Fatalf("sound calls = %d, want 0: an event that makes no sound stays silent", room.soundCalls)
	}
}

func TestSendTrioNilRoomIsSafe(t *testing.T) {
```

Create `internal/rooms/send_unsighted_test.go` (it reuses `sightTestRoom`, `sightTestPlain` and `sightTestInfraredConditionId` from `participant_sight_test.go`; in a "cave" room all three players see nothing, infrared lifts one to shapes, and "city" is lit):

```go
package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// SendTextUnsighted is the sound half of an event that is both seen and
// heard (#242, owner ruling R4): it reaches only a player who cannot make out
// even shapes. A shapes reader gets the visual line instead, and an excluded
// player (the actor) reads its own line.
func TestSendTextUnsighted_ReachesOnlyThoseWhoSeeNothing(t *testing.T) {
	r := sightTestRoom(t, "cave")
	if !users.GetByUserId(7412).Character.Conditions.AddCondition(sightTestInfraredConditionId, true) {
		t.Fatal("precondition: Bobrick should now carry infrared")
	}

	r.SendTextUnsighted(messaging.CategorySpellDisruption, messaging.SoundChantBreaksOff, 7411)

	if got := events.DrainQueuedMessagesForTest(7411); len(got) != 0 {
		t.Fatalf("an excluded player got %q", got)
	}
	if got := events.DrainQueuedMessagesForTest(7412); len(got) != 0 {
		t.Fatalf("a shapes reader gets the visual line, not the sound: got %q", got)
	}
	got := sightTestPlain(events.DrainQueuedMessagesForTest(7413))
	if len(got) != 1 || got[0] != messaging.SoundChantBreaksOff {
		t.Fatalf("a reader who sees nothing read %q, want %q", got, messaging.SoundChantBreaksOff)
	}
}

func TestSendTextUnsighted_LitRoomIsSilent(t *testing.T) {
	r := sightTestRoom(t, "city")
	r.SendTextUnsighted(messaging.CategorySpellDisruption, messaging.SoundChantBreaksOff)
	for _, id := range []int{7411, 7412, 7413} {
		if got := events.DrainQueuedMessagesForTest(id); len(got) != 0 {
			t.Fatalf("user %d sees the room and still got the sound: %q", id, got)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/messaging/ -run TestSendTrio -count=1` and `go test ./internal/rooms/ -run TestSendTextUnsighted -count=1`
Expected: both FAIL to build. Dry run on master:
```
internal\messaging\trio_test.go:156:3: unknown field ObserverSound in struct literal of type Trio
internal\messaging\trio_test.go:156:47: undefined: SoundChantBreaksOff
FAIL	github.com/GoMudEngine/GoMud/internal/messaging [build failed]
internal\rooms\send_unsighted_test.go:30:42: undefined: messaging.SoundChantBreaksOff
internal\rooms\send_unsighted_test.go:37:4: r.SendTextUnsighted undefined (type *Room has no field or method SendTextUnsighted)
FAIL	github.com/GoMudEngine/GoMud/internal/rooms [build failed]
```

- [ ] **Step 3: Implement**

Create `internal/messaging/disruption_sounds.go`:

```go
package messaging

// The lines a reader who sees nothing hears when a spell being woven is
// disrupted (#242, owner ruling R4: disruptions are heard; the quiet weave and
// the focus shift are sight-only). Every disruption path shares them, mob and
// player caster alike, so a blind reader hears one kind of event one way.
const (
	// SoundChantBreaksOff is a broken concentration or an interrupted cast.
	SoundChantBreaksOff = `Someone's chant breaks off.`
	// SoundSpellSputtersOut is a spell that fizzles or falters.
	SoundSpellSputtersOut = `A half-formed spell sputters out.`
)
```

In `internal/messaging/trio.go`, three edits.

old_string:
```go
// audience had no seat here at all, so it travelled outside the pipeline.
type Trio struct{ Actor, Actee, Observer, RemoteObserver Line }
```
new_string:
```go
// audience had no seat here at all, so it travelled outside the pipeline.
//
// ObserverSound is what an observer who sees nothing hears of the event, for
// one that is heard as well as seen (a spell disruption, #242 owner ruling
// R4). It must name nobody. NoLine, the usual case, keeps the event silent to
// a reader who cannot see it.
type Trio struct{ Actor, Actee, Observer, RemoteObserver, ObserverSound Line }
```

old_string:
```go
	SendTextVisualHidingNames(cat Category, txt string, names []string, excludeUserIds ...int)
	// ParticipantSight
```
new_string:
```go
	SendTextVisualHidingNames(cat Category, txt string, names []string, excludeUserIds ...int)
	// SendTextUnsighted sends a line that names nobody to every player who
	// cannot make out even shapes, excluding some user ids: the sound half of
	// an event the observer line shows to those who can see.
	SendTextUnsighted(cat Category, txt string, excludeUserIds ...int)
	// ParticipantSight
```

old_string:
```go
			[]string{aud.ActorName, aud.ActeeName}, trioExclusions(aud)...)
	}
	if aud.RemoteRoom != nil
```
new_string:
```go
			[]string{aud.ActorName, aud.ActeeName}, trioExclusions(aud)...)
	}
	if aud.Room != nil && t.ObserverSound.Text != "" {
		aud.Room.SendTextUnsighted(t.ObserverSound.Cat, t.ObserverSound.Text, trioExclusions(aud)...)
	}
	if aud.RemoteRoom != nil
```

In `internal/rooms/rooms.go`, add the method just before `SendTextVisualToUser`.

old_string:
```go
// SendTextVisualToUser delivers a sight-gated message to a single
```
new_string:
```go
// SendTextUnsighted delivers txt on the audio channel to every player in the
// room who cannot make out even shapes: the sound half of an event that is
// both seen and heard, for a caller whose visual half travels on its own
// (messaging.SendTrio's ObserverSound, the spell-disruption senders in
// internal/hooks). txt must name nobody: the audio channel skips the sight
// gate and the anonymizer, as SendTextVisualWithAudio's audio half does.
func (r *Room) SendTextUnsighted(cat messaging.Category, txt string, excludeUserIds ...int) {
	if txt == "" {
		return
	}
	for _, uid := range r.GetPlayers() {
		if excluded(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
		if u == nil || visualDecision(u.Character, r) != messaging.SightNone {
			continue
		}
		rendered := messaging.RenderForRecipient(messaging.RenderInput{
			Category:  cat,
			Text:      txt,
			Channel:   messaging.ChannelAudio,
			LineWidth: u.GetLineWidth(),
		})
		if rendered == "" {
			continue
		}
		events.AddToQueue(events.Message{
			UserId: u.UserId,
			Text:   rendered + "\n",
		})
	}
}

// SendTextVisualToUser delivers a sight-gated message to a single
```

In `internal/hooks/NewRound_DoCombat_helpers.go`, `sendVisualElseAudible` delegates to the new method and the private constants go. Edit:

old_string:
```go
// sendVisualElseAudible sends visualMsg through the visual pipeline, which
// delivers it to every reader who makes out at least shapes (anonymising
// names for a shapes reader), and soundMsg to every player the pipeline
// skipped. Every player in the room reads exactly one of the two, except
// excludeUserIds, who read neither (a caster reads its own line).
func sendVisualElseAudible(room *rooms.Room, cat messaging.Category, visualMsg, soundMsg string, excludeUserIds ...int) {
	if room == nil {
		return
	}
	room.SendTextVisual(cat, visualMsg, excludeUserIds...)
	for _, uid := range room.GetPlayers() {
		if isExcludedUser(uid, excludeUserIds) {
			continue
		}
		u := users.GetByUserId(uid)
		if u != nil && !messaging.CanSeeShapes(u.Character, room) {
			u.SendText(cat, soundMsg)
		}
	}
}

// The sound lines a reader who sees nothing gets for a spell-channel
// disruption (#242, owner ruling R4). Mob and player casters share them.
const (
	spellChantBreaksOffSound = `Someone's chant breaks off.`
	spellSputtersOutSound    = `A half-formed spell sputters out.`
)
```
new_string:
```go
// sendVisualElseAudible sends visualMsg through the visual pipeline, which
// delivers it to every reader who makes out at least shapes (anonymising
// names for a shapes reader), and soundMsg to every player the pipeline
// skipped. Every player in the room reads exactly one of the two, except
// excludeUserIds, who read neither (a caster reads its own line). The sound
// lines for a spell disruption are messaging.SoundChantBreaksOff and
// messaging.SoundSpellSputtersOut (#242, owner ruling R4), shared with the
// flee, throw, throttle and boss-interrupt paths.
func sendVisualElseAudible(room *rooms.Room, cat messaging.Category, visualMsg, soundMsg string, excludeUserIds ...int) {
	if room == nil {
		return
	}
	room.SendTextVisual(cat, visualMsg, excludeUserIds...)
	room.SendTextUnsighted(cat, soundMsg, excludeUserIds...)
}
```

Then replace the three uses of the old constants in the same file:

old_string:
```go
		`%s's concentration breaks.`, mobSubjectName(mob, room)),
		spellChantBreaksOffSound)
}
```
new_string:
```go
		`%s's concentration breaks.`, mobSubjectName(mob, room)),
		messaging.SoundChantBreaksOff)
}
```

old_string:
```go
		`%s's spell %s.`, mobSubjectName(mob, room), verb),
		spellSputtersOutSound)
}
```
new_string:
```go
		`%s's spell %s.`, mobSubjectName(mob, room), verb),
		messaging.SoundSpellSputtersOut)
}
```

old_string:
```go
		`<ansi fg="username">%s</ansi>'s concentration breaks.`, caster.Character.Name),
		spellChantBreaksOffSound, caster.UserId)
}
```
new_string:
```go
		`<ansi fg="username">%s</ansi>'s concentration breaks.`, caster.Character.Name),
		messaging.SoundChantBreaksOff, caster.UserId)
}
```

`isExcludedUser` stays: `sendDarkRoomCombatFallback` still uses it.

In `internal/hooks/spell_channel_sight_test.go`, point the existing tests at the exported constants.

old_string:
```go
		{"concentration breaks", sendMobConcentrationBroke, "concentration breaks", spellChantBreaksOffSound},
		{"spell fizzles", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "fizzles") }, "spell fizzles", spellSputtersOutSound},
		{"spell falters", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "falters") }, "spell falters", spellSputtersOutSound},
```
new_string:
```go
		{"concentration breaks", sendMobConcentrationBroke, "concentration breaks", messaging.SoundChantBreaksOff},
		{"spell fizzles", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "fizzles") }, "spell fizzles", messaging.SoundSpellSputtersOut},
		{"spell falters", func(m *mobs.Mob, r *rooms.Room) { sendMobSpellFailed(m, r, "falters") }, "spell falters", messaging.SoundSpellSputtersOut},
```

old_string:
```go
		require.Equal(t, 1, countContaining(got, spellChantBreaksOffSound), "%v", got)
```
new_string:
```go
		require.Equal(t, 1, countContaining(got, messaging.SoundChantBreaksOff), "%v", got)
```

Update `internal/messaging/context.md` (four edits).

old_string:
```
  probe in `internal/combat/combat.go`; see the M4d Task 5 report.
- `Recipient` — minimal
```
new_string:
```
  probe in `internal/combat/combat.go`; see the M4d Task 5 report.
  A fifth field, `ObserverSound`, is what an observer who sees nothing
  hears of an event heard as well as seen (a spell disruption, #242 owner
  ruling R4). It names nobody; `NoLine`, the usual case, keeps the event
  silent to such a reader.
- `SoundChantBreaksOff`, `SoundSpellSputtersOut` (`disruption_sounds.go`):
  the two sound lines every spell-disruption path shares, mob and player
  caster alike (broken concentration or interrupt; fizzle or falter).
- `Recipient` — minimal
```

old_string:
```
  `SendTextVisualHidingNames(cat, txt, names, excludeUserIds ...int)` and
  `ParticipantSight(userId int) SightDecision`.
```
new_string:
```
  `SendTextVisualHidingNames(cat, txt, names, excludeUserIds ...int)`,
  `SendTextUnsighted(cat, txt, excludeUserIds ...int)` and
  `ParticipantSight(userId int) SightDecision`.
```

old_string:
```
  reader's `ParticipantSight`; the observer and remote-observer lines hide
  both, judged per-observer by their own room's `ParticipantSight`.
```
new_string:
```
  reader's `ParticipantSight`; the observer and remote-observer lines hide
  both, judged per-observer by their own room's `ParticipantSight`. An
  `ObserverSound` goes to `aud.Room.SendTextUnsighted` with the same
  exclusions, reaching only the observers who see nothing.
```

old_string:
```
| `trio.go` | `Line`/`Trio`/`Audience`/`SendTrio` — fan-out of one narrated event to its four audiences |
```
new_string:
```
| `trio.go` | `Line`/`Trio`/`Audience`/`SendTrio` — fan-out of one narrated event to its four audiences |
| `disruption_sounds.go` | `SoundChantBreaksOff`, `SoundSpellSputtersOut`: the shared sound lines of a spell disruption (#242, owner ruling R4) |
```

Update `internal/rooms/context.md`.

old_string:
```
  observers. `SendTextVisualWithAudio` gives the unsighted an audio variant.
```
new_string:
```
  observers. `SendTextVisualWithAudio` gives the unsighted an audio variant.
  `SendTextUnsighted(cat, txt, excludeUserIds...)` is that audio half on its
  own, for a caller whose visual half travels separately
  (`messaging.SendTrio`'s `ObserverSound`, the spell-disruption senders in
  `internal/hooks`); `txt` must name nobody.
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```
go build ./...
go test ./internal/messaging/ -run TestSendTrio -count=1 -v
go test ./internal/rooms/ -run TestSendTextUnsighted -count=1 -v
go test ./internal/hooks/ -run 'TestMobSpell|TestPlayerConcentrationBroke' -count=1 -v
```
Expected: all PASS, including the two new trio tests, the two new rooms tests, and the existing `TestMobSpellDisruption_ShapesReadAFigure_BlindHearTheSound`, `TestPlayerConcentrationBroke_FollowsTheObserversSight` and `TestMobSpellChannel_HiddenCasterIsNeverNamed` (now on `SendTextUnsighted`). Then `go test ./internal/messaging/ ./internal/rooms/ ./internal/hooks/ -count=1`: all `ok`. `gofmt -l` on the touched files prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/messaging/disruption_sounds.go internal/messaging/trio.go internal/messaging/trio_test.go internal/messaging/context.md internal/rooms/rooms.go internal/rooms/send_unsighted_test.go internal/rooms/context.md internal/hooks/NewRound_DoCombat_helpers.go internal/hooks/spell_channel_sight_test.go
git commit -m "feat(sight): shared disruption sound lines, Room.SendTextUnsighted, Trio.ObserverSound (#242)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task F3b: a player's fizzle, falter and bleed-out break reach the room

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go`
- Test: create `internal/hooks/player_spell_disruption_sight_test.go`

`handlePlayerFoldCasting` sends the bleed-out break (`IsDisabled`), `TargetGone` and `InsufficientConviction` to the caster only; the mob arms of `handleMobFoldCasting` call `sendMobSpellFailed` for the last two. A player and a mob caster must read alike.

- [ ] **Step 1: Write the failing test**

Create `internal/hooks/player_spell_disruption_sight_test.go`. `seedFallbackRoom` (`dark_room_fallback_sight_test.go:22`) puts users 1 and 2 in cave room 2 at a pinned lamp and gives user 1 the named eyes; `drainPlain` and `countContaining` are the package's existing helpers.

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4, closing playtest 2026-10-08: a player caster's
// fizzle, falter and bleed-out break told the room nothing, while the mob
// versions are seen by sight and heard by a reader who sees nothing. Player
// and mob casters read alike now. seedFallbackRoom: at lamp 10 user 1 (heat
// eyes) reads shapes; at lamp 0 user 1 (night eyes) reads nothing. User 2 is
// the caster.

// startPlayerTestCast puts u mid fold-cast against targetMobIds.
func startPlayerTestCast(t *testing.T, u *users.UserRecord, targetMobIds ...int) {
	t.Helper()
	u.Character.Activity = activity.NewMachine()
	require.NoError(t, u.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "sparks", FoldsNeeded: 4, TargetMobInstanceIds: targetMobIds},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	))
}

// bleedOut drops u to 0 health (Character.IsDisabled) until the test ends.
func bleedOut(t *testing.T, u *users.UserRecord) {
	t.Helper()
	was := u.Character.Health
	u.Character.Health = 0
	t.Cleanup(func() { u.Character.Health = was })
}

func TestPlayerSpellFizzle_ShapesSeeAFigure_BlindHearTheSpell(t *testing.T) {
	t.Run("shapes", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		startPlayerTestCast(t, caster, 999999) // no such mob: the target is gone

		require.True(t, handlePlayerFoldCasting(caster, caster.UserId))

		got := drainPlain(1)
		require.Equal(t, messaging.SightShapes, messaging.ParticipantSight(users.GetByUserId(1).Character, room))
		require.Equal(t, 1, countContaining(got, "spell fizzles"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})

	t.Run("sees nothing", func(t *testing.T) {
		seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		startPlayerTestCast(t, caster, 999999)

		require.True(t, handlePlayerFoldCasting(caster, caster.UserId))

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})
}

func TestPlayerBleedOutBreak_IsSeenAndHeard(t *testing.T) {
	t.Run("shapes", func(t *testing.T) {
		seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		startPlayerTestCast(t, caster)
		bleedOut(t, caster)

		require.True(t, handlePlayerFoldCasting(caster, caster.UserId))

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, "concentration breaks"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
	})

	t.Run("sees nothing", func(t *testing.T) {
		seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		startPlayerTestCast(t, caster)
		bleedOut(t, caster)

		require.True(t, handlePlayerFoldCasting(caster, caster.UserId))

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundChantBreaksOff), "%v", got)
	})
}

// The falter (not enough conviction to hold the fold) shares the fizzle's
// sender, as the mob's sendMobSpellFailed does for both.
func TestPlayerSpellFalter_ShapesSeeAFigure_BlindHearTheSpell(t *testing.T) {
	t.Run("shapes", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerSpellFailed(caster, room, "falters")

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, "spell falters"), "%v", got)
		require.Zero(t, countContaining(got, caster.Character.Name), "%v", got)
		require.Empty(t, drainPlain(2), "the caster reads its own line, not the room's")
	})

	t.Run("sees nothing", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerSpellFailed(caster, room, "falters")

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, messaging.SoundSpellSputtersOut), "%v", got)
	})

	t.Run("clear sight names the caster", func(t *testing.T) {
		room := seedFallbackRoom(t, 60, heatEyesConditionId)
		caster := users.GetByUserId(2)
		sendPlayerSpellFailed(caster, room, "falters")

		got := drainPlain(1)
		require.Equal(t, 1, countContaining(got, caster.Character.Name+"'s spell falters."), "%v", got)
	})
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/hooks/ -run 'TestPlayerSpellFizzle|TestPlayerBleedOut|TestPlayerSpellFalter' -count=1`
Expected: FAIL to build (`undefined: sendPlayerSpellFailed`). Dry run on master with F3a applied:
```
internal\hooks\player_spell_disruption_sight_test.go:92:3: undefined: sendPlayerSpellFailed
FAIL	github.com/GoMudEngine/GoMud/internal/hooks [build failed]
```
With only the helper of Step 3 added and the three call sites not yet wired, the falter test passes and the end-to-end fizzle and bleed-out tests fail at runtime, which proves they exercise `handlePlayerFoldCasting`:
```
--- FAIL: TestPlayerSpellFizzle_ShapesSeeAFigure_BlindHearTheSpell/sees_nothing (0.00s)
        	Error:      	Not equal:
        	            	expected: 1
        	            	actual  : 0
--- FAIL: TestPlayerBleedOutBreak_IsSeenAndHeard/shapes (0.00s)
--- FAIL: TestPlayerBleedOutBreak_IsSeenAndHeard/sees_nothing (0.00s)
```

- [ ] **Step 3: Implement**

In `internal/hooks/NewRound_DoCombat_helpers.go`, add the player twin of `sendMobSpellFailed` after `sendPlayerConcentrationBroke`.

old_string:
```go
		`<ansi fg="username">%s</ansi>'s concentration breaks.`, caster.Character.Name),
		messaging.SoundChantBreaksOff, caster.UserId)
}
```
new_string:
```go
		`<ansi fg="username">%s</ansi>'s concentration breaks.`, caster.Character.Name),
		messaging.SoundChantBreaksOff, caster.UserId)
}

// sendPlayerSpellFailed narrates a player's spell that fizzles (target gone)
// or falters (not enough conviction) to the rest of the room, as
// sendMobSpellFailed does for a mob; verb is "fizzles" or "falters". The
// caster reads its own line. Before the closing playtest of #382 the player
// versions told the room nothing.
func sendPlayerSpellFailed(caster *users.UserRecord, room *rooms.Room, verb string) {
	sendVisualElseAudible(room, messaging.CategorySpellDisruption, fmt.Sprintf(
		`<ansi fg="username">%s</ansi>'s spell %s.`, caster.Character.Name, verb),
		messaging.SoundSpellSputtersOut, caster.UserId)
}
```

Wire the three player arms of `handlePlayerFoldCasting`.

old_string:
```go
	// Bleeding out = automatic concentration break (player-only check).
	if user.Character.IsDisabled() {
		recordConcentrationFailure(combat.User, combat.Mob, user.Character, castingTargetChar(csBeforeProcess))
		clearCastingActivity(user.Character, activity.TriggerConcentrationBreak)
		events.AddToQueue(events.CastInterrupted{UserId: user.UserId, SpellId: csBeforeProcess.SpellId})
		return true
	}
```
new_string:
```go
	// Bleeding out = automatic concentration break (player-only check).
	if user.Character.IsDisabled() {
		recordConcentrationFailure(combat.User, combat.Mob, user.Character, castingTargetChar(csBeforeProcess))
		clearCastingActivity(user.Character, activity.TriggerConcentrationBreak)
		events.AddToQueue(events.CastInterrupted{UserId: user.UserId, SpellId: csBeforeProcess.SpellId})
		sendPlayerConcentrationBroke(user, rooms.LoadRoom(user.Character.RoomId))
		return true
	}
```

old_string:
```go
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your spell fizzles. The target is gone.</ansi>`)
```
new_string:
```go
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your spell fizzles. The target is gone.</ansi>`)
		sendPlayerSpellFailed(user, rooms.LoadRoom(user.Character.RoomId), "fizzles")
```

old_string:
```go
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your conviction wavers, and the fold collapses.</ansi>`)
```
new_string:
```go
		user.SendText(messaging.CategorySpellDisruption, `<ansi fg="red">Your conviction wavers, and the fold collapses.</ansi>`)
		sendPlayerSpellFailed(user, rooms.LoadRoom(user.Character.RoomId), "falters")
```

`SpellDataMissing` stays caster-only, as the mob arm stays silent: it is a data fault, not something the room witnesses.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/hooks/ -run 'TestPlayerSpellFizzle|TestPlayerBleedOut|TestPlayerSpellFalter|TestMobSpell|TestPlayerConcentrationBroke' -count=1 -v`
Expected: all PASS (dry run: 9 top-level tests PASS). Then `go test ./internal/hooks/ -count=1`: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/NewRound_DoCombat_helpers.go internal/hooks/player_spell_disruption_sight_test.go
git commit -m "fix(sight): a player's fizzle, falter and bleed-out break reach the room, seen or heard (#242)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task F3c: a flee that breaks a cast is heard, in the shared wording

**Files:**
- Modify: `internal/mobcommands/flee.go`, `internal/usercommands/usercommands.go`, `internal/mobcommands/predator_test.go`
- Test: create `internal/mobcommands/flee_concentration_sight_test.go`, create `internal/usercommands/flee_concentration_sight_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/mobcommands/flee_concentration_sight_test.go`. `mobSpeechRoom` (`speech_sight_test.go:41`) lights room 1 at lamp 60 and blinds Bobrick (user 2); `mobSpeechHeard` strips tags; `startTestCast` is in `predator_test.go`.

```go
package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4, closing playtest 2026-10-08: a mob that flees
// mid-cast broke its concentration on the visual channel only, so a reader
// who saw nothing got no line at all. A break is heard. mobSpeechRoom lights
// room 1 exactly: Aliceia (user 1) sees clearly, Bobrick (user 2) is blinded.
// The line is the shared spell-disruption wording ("X's concentration
// breaks."), not the flee's own "breaks their concentration."
func TestFlee_CastingMobBreakIsSeenAndHeard(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := mobSpeechRoom(t)
	mob.Character.SetAggro(1, 0, characters.DefaultAttack)
	startTestCast(t, mob)

	_, err := Flee("", mob, room)
	require.NoError(t, err)

	sighted, blind := mobSpeechHeard(1), mobSpeechHeard(2)
	require.Contains(t, sighted, "Skeleton's concentration breaks.")
	require.Contains(t, blind, messaging.SoundChantBreaksOff)
	for _, line := range blind {
		require.NotContains(t, line, "Skeleton", "a reader who sees nothing must not learn who broke off")
	}
}
```

Create `internal/usercommands/flee_concentration_sight_test.go`. `speechWrapperScene` (`speech_sight_wrapper_test.go`) lights room 1 at lamp 60 with Aliceia (1) and Bobrick (2) both at full sight; `blindForSpeechTest` blinds one; `speechWrapperHeard` strips tags. Do NOT add a `t.Cleanup` that touches `rooms.LoadRoom(1)`: `t.Cleanup` runs after the deferred `seedAllRegistries` cleanup, when room 1 is gone, and the dry run panicked there.

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4, closing playtest 2026-10-08: a player who flees
// mid-cast broke concentration on the visual channel only ("A figure breaks
// their concentration."), so a reader who saw nothing heard nothing. A
// break is heard, in the shared spell-disruption wording.

// fleeMidCast starts Aliceia casting in a fight with mob 100 and has her
// flee. The caller's seedAllRegistries cleanup restores the users and rooms.
func fleeMidCast(t *testing.T, alice *users.UserRecord) {
	t.Helper()
	alice.Character.SetAggro(0, 100, characters.DefaultAttack)
	alice.Character.Activity = activity.NewMachine()
	require.NoError(t, alice.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "sparks", ConvictionSpent: 3, FoldsNeeded: 4},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	))

	handled, err := TryCommand("flee", "", alice.UserId, events.CmdSkipScripts)
	require.True(t, handled)
	require.NoError(t, err)
	require.True(t, alice.Character.Activity.IsFree(), "the flee must drop the cast")
}

func TestFlee_CastingPlayerBreakIsHeardByTheBlind(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, bob, _ := speechWrapperScene(t)
	blindForSpeechTest(t, bob)

	fleeMidCast(t, alice)

	heard := speechWrapperHeard(2)
	require.Contains(t, heard, messaging.SoundChantBreaksOff)
	for _, line := range heard {
		require.NotContains(t, line, "Aliceia", "a reader who sees nothing must not learn who broke off")
	}
}

func TestFlee_CastingPlayerBreakNamesTheCasterAtClearSight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	alice, _, _ := speechWrapperScene(t)

	fleeMidCast(t, alice)

	require.Contains(t, speechWrapperHeard(2), "Aliceia's concentration breaks.")
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/mobcommands/ -run TestFlee_CastingMob -count=1` and `go test ./internal/usercommands/ -run TestFlee_CastingPlayer -count=1 -v`
Expected: FAIL. Dry run on master with F3a applied:
```
--- FAIL: TestFlee_CastingMobBreakIsSeenAndHeard (0.00s)
        	Error:      	[]string{"Skeleton breaks their concentration."} does not contain "Skeleton's concentration breaks."
--- FAIL: TestFlee_CastingPlayerBreakIsHeardByTheBlind (0.00s)
        	Error:      	[]string(nil) does not contain "Someone's chant breaks off."
--- FAIL: TestFlee_CastingPlayerBreakNamesTheCasterAtClearSight (0.00s)
        	Error:      	[]string{"Aliceia breaks their concentration."} does not contain "Aliceia's concentration breaks."
```

- [ ] **Step 3: Implement**

In `internal/mobcommands/flee.go`:

old_string:
```go
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi> breaks their concentration.`,
			mob.Character.Name))
```
new_string:
```go
		// Seen by sight, heard by a reader who sees nothing, in the shared
		// spell-disruption wording (#242, owner ruling R4).
		room.SendTextVisualWithAudio(messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="mobname">%s</ansi>'s concentration breaks.`,
			mob.Character.Name), messaging.SoundChantBreaksOff)
```

In `internal/usercommands/usercommands.go` (the fold-casting intercept):

old_string:
```go
			room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
				`<ansi fg="username">%s</ansi> breaks their concentration.`,
				user.Character.Name), user.UserId)
```
new_string:
```go
			// Seen by sight, heard by a reader who sees nothing, in the
			// shared spell-disruption wording (#242, owner ruling R4).
			room.SendTextVisualWithAudio(messaging.CategorySpellDisruption, fmt.Sprintf(
				`<ansi fg="username">%s</ansi>'s concentration breaks.`,
				user.Character.Name), messaging.SoundChantBreaksOff, user.UserId)
```

The existing mob flee test asserts the old wording. In `internal/mobcommands/predator_test.go`:

old_string:
```go
	assert.Equal(t, 1, countPerRecipient(t, captured, mu, "breaks their concentration."),
```
new_string:
```go
	assert.Equal(t, 1, countPerRecipient(t, captured, mu, "concentration breaks."),
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/mobcommands/ -run TestFlee -count=1 -v` and `go test ./internal/usercommands/ -run 'TestFlee_CastingPlayer|TestTryCommand' -count=1 -v`
Expected: PASS (dry run: 6 mob flee tests, both player tests, and `TestTryCommand` with its `casting_flee_*` subtests PASS). Then `go test ./internal/mobcommands/ ./internal/usercommands/ -count=1`: both `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/mobcommands/flee.go internal/mobcommands/predator_test.go internal/mobcommands/flee_concentration_sight_test.go internal/usercommands/usercommands.go internal/usercommands/flee_concentration_sight_test.go
git commit -m "fix(sight): a flee that breaks a cast is heard, in the shared disruption wording (#242)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task F3d: boss, throw and throttle interrupts are heard

**Files:**
- Modify: `internal/hooks/spell_effects.go`, `internal/usercommands/move_narration.go`, `internal/usercommands/throttle.go`, `internal/usercommands/throw.go`
- Test: create `internal/hooks/spell_interrupt_sight_test.go`, create `internal/usercommands/cast_interrupt_sound_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/hooks/spell_interrupt_sight_test.go`. It reuses `setMobCastingForSpellTest` (`spell_interrupt_test.go:18`), `newSpellEffectCtx` and `spellContestAttackWin`, and the "neural-stun" on "core-discharge" pairing that `TestHiddenMob_SpellInterruptRoomLineDoesNotNameIt` already proves interrupts in tests.

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4, closing playtest 2026-10-08: a boss-interrupt spell
// that collapses a mob's cast is a disruption, so a reader who sees nothing
// hears it, as every other disruption is heard. The room line ("X's spell
// collapses!") travelled on the trio's visual observer line only.
func TestSpellInterrupt_IsHeardByAReaderWhoSeesNothing(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedNarrationConditions())
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(90)
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Validate()
	saved := m.Character.Activity
	t.Cleanup(func() { m.Character.Activity = saved })
	setMobCastingForSpellTest(m, "core-discharge")

	bob := users.GetByUserId(2)
	bob.Character.Perception = perception.NewMachine()
	require.NoError(t, bob.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(bob.Character, room))
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)

	caster := users.GetByUserId(1)
	spell := &spells.SpellData{SpellId: "neural-stun", Name: "Neural Stun", EffectType: "damage"}
	interruptSpellTarget(newSpellEffectCtx(caster.Character, actions.NewUserActorInRoom(caster, room),
		actions.NewMobActorInRoom(m, room), room, spell, 10, spellContestAttackWin()))

	require.NotZero(t, countContaining(drainPlain(1), "scrambles"), "the caster reads its own line")
	got := drainPlain(2)
	require.Equal(t, 1, countContaining(got, messaging.SoundChantBreaksOff), "%v", got)
	require.Zero(t, countContaining(got, "Skeleton"), "%v", got)
	require.Zero(t, countContaining(got, "collapses"), "the seen line is for readers who see: %v", got)
}
```

Create `internal/usercommands/cast_interrupt_sound_test.go`. It sends `player_cast_interrupt` from the shipped special-move store (loaded by `TestMain`, `usercommands_test.go:96`) with the categories production passes.

```go
package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4, closing playtest 2026-10-08: throw and throttle
// interrupting a mob's cast is a disruption, so a reader who sees nothing
// hears it. The room line travelled on the trio's visual observer line only.
// Each case sends the event with the categories production passes.
func TestCastInterruptMoveEvents_AreHeardAndSeen(t *testing.T) {
	cases := []struct {
		verb string
		cats moveCategories
	}{
		{"throw", throwCastInterruptCategories},
		{"throttle", throttleCastInterruptCategories},
	}
	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			// send lights the scene, blinds Bobrick (user 2) if asked, and
			// has Aliceia (user 1) interrupt the Skeleton's cast.
			send := func(blindBob bool) {
				alice, bob, room := speechWrapperScene(t)
				if blindBob {
					blindForSpeechTest(t, bob)
				}
				ids := moveIdentities{
					Actor:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, alice.Character.Name),
					ActorPlain: alice.Character.Name,
					Actee:      `<ansi fg="mobname">Skeleton</ansi>`,
					ActeePlain: "Skeleton",
				}
				aud := messaging.Audience{Actor: alice, ActorId: alice.UserId, ActorName: alice.Character.Name,
					ActeeName: "Skeleton", Room: room}
				sendMoveEvent(tc.verb, "player_cast_interrupt", ids, aud, tc.cats, nil)
			}

			t.Run("sees nothing", func(t *testing.T) {
				cleanup := seedAllRegistries()
				defer cleanup()
				send(true)
				require.Equal(t, []string{messaging.SoundChantBreaksOff}, speechWrapperHeard(2))
			})

			t.Run("sees clearly", func(t *testing.T) {
				cleanup := seedAllRegistries()
				defer cleanup()
				send(false)
				heard := speechWrapperHeard(2)
				require.Len(t, heard, 1, "%v", heard)
				require.Contains(t, heard[0], "Skeleton")
				require.NotContains(t, heard[0], messaging.SoundChantBreaksOff)
			})
		})
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/hooks/ -run TestSpellInterrupt_IsHeard -count=1` and `go test ./internal/usercommands/ -run TestCastInterruptMoveEvents -count=1 -v`
Expected: FAIL. Dry run on master with F3a to F3c applied:
```
--- FAIL: TestSpellInterrupt_IsHeardByAReaderWhoSeesNothing (0.00s)
        	Error:      	Not equal:
        	            	expected: 1
        	            	actual  : 0
internal\usercommands\cast_interrupt_sound_test.go:20:13: undefined: throwCastInterruptCategories
FAIL	github.com/GoMudEngine/GoMud/internal/usercommands [build failed]
```
With only the `throwCastInterruptCategories` refactor of Step 3 in place (same value as before, no sound), both "sees nothing" subtests fail at runtime and both "sees clearly" subtests pass:
```
--- FAIL: TestCastInterruptMoveEvents_AreHeardAndSeen/throw/sees_nothing (0.00s)
        	            	expected: []string{"Someone's chant breaks off."}
        	            	actual  : []string(nil)
--- PASS: TestCastInterruptMoveEvents_AreHeardAndSeen/throw/sees_clearly (0.00s)
--- FAIL: TestCastInterruptMoveEvents_AreHeardAndSeen/throttle/sees_nothing (0.00s)
--- PASS: TestCastInterruptMoveEvents_AreHeardAndSeen/throttle/sees_clearly (0.00s)
```

- [ ] **Step 3: Implement**

In `internal/hooks/spell_effects.go` (`interruptSpellTarget`):

old_string:
```go
		Observer: messaging.Say(messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="cyan">%s's spell collapses!</ansi>`, roomName(actorHiddenFromRoom(c.target), c.targetName()))),
	}, c.audience())
```
new_string:
```go
		Observer: messaging.Say(messaging.CategorySpellDisruption, fmt.Sprintf(
			`<ansi fg="cyan">%s's spell collapses!</ansi>`, roomName(actorHiddenFromRoom(c.target), c.targetName()))),
		// A disruption is heard as well as seen (#242, owner ruling R4).
		ObserverSound: messaging.Say(messaging.CategorySpellDisruption, messaging.SoundChantBreaksOff),
	}, c.audience())
```

In `internal/usercommands/move_narration.go`, give `moveCategories` the sound and pass it through `sendMoveEvent` (the name `sendMoveEvent` must stay at every call site; see the dry-run facts above).

old_string:
```go
type moveCategories struct {
	Actor          messaging.Category
	Actee          messaging.Category
	Observer       messaging.Category
	RemoteObserver messaging.Category
}
```
new_string:
```go
type moveCategories struct {
	Actor          messaging.Category
	Actee          messaging.Category
	Observer       messaging.Category
	RemoteObserver messaging.Category
	// ObserverSound is what an observer who sees nothing hears of the
	// event, sent under the Observer category: a line that names nobody,
	// for an event heard as well as seen (a cast interrupt, #242 owner
	// ruling R4). Empty, the usual case, keeps it silent to them.
	ObserverSound string
}
```

old_string:
```go
		RemoteObserver: lineOrNone(cats.RemoteObserver, roles.ActeeObserver),
	}, aud)
}
```
new_string:
```go
		RemoteObserver: lineOrNone(cats.RemoteObserver, roles.ActeeObserver),
		ObserverSound:  lineOrNone(cats.Observer, cats.ObserverSound),
	}, aud)
}
```

In `internal/usercommands/throttle.go`:

old_string:
```go
var throttleCastInterruptCategories = moveCategories{Actor: messaging.CategorySystem, Actee: messaging.CategorySystem, Observer: messaging.CategorySpellDisruption}
```
new_string:
```go
//
// A disruption is heard as well as seen (#242, owner ruling R4), so a reader
// who sees nothing hears the chant break off.
var throttleCastInterruptCategories = moveCategories{Actor: messaging.CategorySystem, Actee: messaging.CategorySystem, Observer: messaging.CategorySpellDisruption,
	ObserverSound: messaging.SoundChantBreaksOff}
```

In `internal/usercommands/throw.go`, name the interrupt's categories (the same four roles `sameMoveCategory(messaging.CategorySpellDisruption)` gave) and add the sound.

old_string:
```go
// throwInterruptAudience returns a copy of throw's actee-less Audience that
```
new_string:
```go
// throwCastInterruptCategories: player_cast_interrupt rides
// CategorySpellDisruption on every role, as throttle's does: a bystander
// watching a spell die is reading about the disruption. A disruption is heard
// as well as seen (#242, owner ruling R4), so a reader who sees nothing hears
// the chant break off.
var throwCastInterruptCategories = moveCategories{
	Actor:          messaging.CategorySpellDisruption,
	Actee:          messaging.CategorySpellDisruption,
	Observer:       messaging.CategorySpellDisruption,
	RemoteObserver: messaging.CategorySpellDisruption,
	ObserverSound:  messaging.SoundChantBreaksOff,
}

// throwInterruptAudience returns a copy of throw's actee-less Audience that
```

old_string:
```go
				throwInterruptAudience(aud, mob.Character.Name),
				sameMoveCategory(messaging.CategorySpellDisruption), nil)
```
new_string:
```go
				throwInterruptAudience(aud, mob.Character.Name),
				throwCastInterruptCategories, nil)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```
go test ./internal/hooks/ -run 'TestSpellInterrupt_IsHeard|TestHiddenMob_SpellInterrupt' -count=1 -v
go test ./internal/usercommands/ -run 'TestCastInterruptMoveEvents|TestThrow|TestThrottle' -count=1 -v
```
Expected: all PASS (dry run: both hooks tests, the four interrupt subtests and every throw test PASS).

- [ ] **Step 5: Gate for F3**

```
gofmt -l internal/
go build ./...
go vet ./...
go test ./internal/messaging/ ./internal/rooms/ ./internal/hooks/ ./internal/usercommands/ ./internal/mobcommands/ -count=1
go test . -count=1
python -I tools/context_md_audit.py
```
Expected: `gofmt -l` prints nothing; build and vet are clean; all five packages `ok`; root `ok` (dry run: `ok github.com/GoMudEngine/GoMud 30.646s`); the audit lists no phantom symbol in `internal/messaging`, `internal/rooms`, `internal/hooks`, `internal/usercommands` or `internal/mobcommands`.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/spell_effects.go internal/hooks/spell_interrupt_sight_test.go internal/usercommands/move_narration.go internal/usercommands/throttle.go internal/usercommands/throw.go internal/usercommands/cast_interrupt_sound_test.go
git commit -m "fix(sight): boss, throw and throttle interrupts are heard by a reader who sees nothing (#242)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


### Task F6a: no article before a mobname tag in flavour lines

**Why:** Below clear sight `Anonymize` swaps a whole `mobname` tag body for "a figure", so room 4121's idle line `A <ansi fg="mobname">water-rat</ansi> emerges...` reads "A a figure emerges...". Fifteen idle lines tag a creature that is not a mob in the room at all (verified: rooms 4100, 4104, 4108, 4120, 4121, 4140, 4144, 4145, 4147, 4153, 4156 spawn no mobs; 4115 spawns 346 Young Fisherman Luc, 4116 spawns 342 Dock Master Arn, 4118 spawns 9571/9577/9578 ferry agent and factors, 4119 spawns 350 Child Pip; none is a gull, heron or bird). Flavour prose takes no identity tag.

**Files:**
- Create: `mobname_article_guard_test.go` (repo root)
- Modify: the 15 room YAML files listed in Step 3

- [ ] **Step 1: Write the failing guard**

Create `mobname_article_guard_test.go`:

```go
package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// mobnameArticlePattern matches an article written directly before a mobname
// tag. Below clear sight Anonymize swaps the whole tag body for "a figure", so
// "A <mobname>water-rat</mobname> emerges" reads "A a figure emerges" (#382
// playtest, room 4121). A creature named in flavour prose that is not a real
// mob in the room takes no identity tag at all.
var mobnameArticlePattern = regexp.MustCompile(`(?i)\b(a|an) <ansi fg="mobname`)

func TestDogmudContentHasNoArticleBeforeMobnameTag(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(here), "_datafiles", "world", "dogmud")
	scanned := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		for i, line := range strings.Split(string(data), "\n") {
			if mobnameArticlePattern.MatchString(line) {
				rel, _ := filepath.Rel(filepath.Dir(here), path)
				t.Errorf("%s:%d: article before a mobname tag: %s", filepath.ToSlash(rel), i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Fatal("scanned no YAML files; the walk is not reaching the world data")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test . -run TestDogmudContentHasNoArticleBeforeMobnameTag -count=1`

Expected (dry run on `f508db20b`): FAIL with 15 errors, first and last:
```
--- FAIL: TestDogmudContentHasNoArticleBeforeMobnameTag (17.68s)
    mobname_article_guard_test.go:42: _datafiles/world/dogmud/rooms/stillwater/4100.yaml:59: article before a mobname tag: - A <ansi fg="mobname">gull</ansi> wheels overhead, head cocked at the road, before sliding east toward the water.
    ...
    mobname_article_guard_test.go:42: _datafiles/world/dogmud/rooms/the_fernway/4156.yaml:57: article before a mobname tag: - A <ansi fg="mobname">sparrow</ansi> takes a quick drink from a small puddle at the embankment's foot and is gone.
FAIL
```

- [ ] **Step 3: Drop the tag in each line (Edit tool, one edit per file)**

Each file has exactly one match. Replace `old_string` with `new_string`:

| File | old_string | new_string |
|---|---|---|
| `_datafiles/world/dogmud/rooms/stillwater/4100.yaml` | `- A <ansi fg="mobname">gull</ansi> wheels` | `- A gull wheels` |
| `_datafiles/world/dogmud/rooms/stillwater/4104.yaml` | `- A <ansi fg="mobname">sparrow</ansi> alights` | `- A sparrow alights` |
| `_datafiles/world/dogmud/rooms/stillwater/4108.yaml` | `- A <ansi fg="mobname">gull</ansi> drops onto the step's` | `- A gull drops onto the step's` |
| `_datafiles/world/dogmud/rooms/stillwater/4115.yaml` | `- A <ansi fg="mobname">gull</ansi> drops onto the driftwood` | `- A gull drops onto the driftwood` |
| `_datafiles/world/dogmud/rooms/stillwater/4116.yaml` | `- A <ansi fg="mobname">gull</ansi> drops to the table` | `- A gull drops to the table` |
| `_datafiles/world/dogmud/rooms/stillwater/4118.yaml` | `- A <ansi fg="mobname">heron</ansi> stands` | `- A heron stands` |
| `_datafiles/world/dogmud/rooms/stillwater/4119.yaml` | `- A <ansi fg="mobname">gull</ansi> turns` | `- A gull turns` |
| `_datafiles/world/dogmud/rooms/stillwater/4120.yaml` | `- A <ansi fg="mobname">water-rat</ansi> darts` | `- A water-rat darts` |
| `_datafiles/world/dogmud/rooms/stillwater/4121.yaml` | `- A <ansi fg="mobname">water-rat</ansi> emerges` | `- A water-rat emerges` |
| `_datafiles/world/dogmud/rooms/stillwater/4140.yaml` | `- A <ansi fg="mobname">crow</ansi> calls once` | `- A crow calls once` |
| `_datafiles/world/dogmud/rooms/stillwater/4144.yaml` | `- A <ansi fg="mobname">raven</ansi> drops` | `- A raven drops` |
| `_datafiles/world/dogmud/rooms/stillwater/4145.yaml` | `- A <ansi fg="mobname">crow</ansi> drops` | `- A crow drops` |
| `_datafiles/world/dogmud/rooms/the_fernway/4147.yaml` | `- A <ansi fg="mobname">wood pigeon</ansi> calls` | `- A wood pigeon calls` |
| `_datafiles/world/dogmud/rooms/the_fernway/4153.yaml` | `- A <ansi fg="mobname">sparrow</ansi> drops` | `- A sparrow drops` |
| `_datafiles/world/dogmud/rooms/the_fernway/4156.yaml` | `- A <ansi fg="mobname">sparrow</ansi> takes` | `- A sparrow takes` |

- [ ] **Step 4: Run it to verify it passes**

Run: `go test . -run TestDogmudContentHasNoArticleBeforeMobnameTag -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud`

- [ ] **Step 5: Commit**

```bash
git add mobname_article_guard_test.go _datafiles/world/dogmud/rooms/stillwater/4100.yaml _datafiles/world/dogmud/rooms/stillwater/4104.yaml _datafiles/world/dogmud/rooms/stillwater/4108.yaml _datafiles/world/dogmud/rooms/stillwater/4115.yaml _datafiles/world/dogmud/rooms/stillwater/4116.yaml _datafiles/world/dogmud/rooms/stillwater/4118.yaml _datafiles/world/dogmud/rooms/stillwater/4119.yaml _datafiles/world/dogmud/rooms/stillwater/4120.yaml _datafiles/world/dogmud/rooms/stillwater/4121.yaml _datafiles/world/dogmud/rooms/stillwater/4140.yaml _datafiles/world/dogmud/rooms/stillwater/4144.yaml _datafiles/world/dogmud/rooms/stillwater/4145.yaml _datafiles/world/dogmud/rooms/the_fernway/4147.yaml _datafiles/world/dogmud/rooms/the_fernway/4153.yaml _datafiles/world/dogmud/rooms/the_fernway/4156.yaml
git commit -m "fix(content): flavour creatures take no mobname tag, no more \"A a figure\" (#382)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

**Note, not in scope:** two room descriptions put "the" before a mobname tag (`new_plymouth_docks/5502.yaml:23` "the <ansi fg=\"mobname\">quay agent", `the_confluence/6109.yaml:15` "the <ansi fg=\"mobname\">passage..."). They would read "the a figure" only if room descriptions were anonymized; the guard covers "a"/"an" only, as the spec names.

### Task F6b: the mob aggro room line ends with a full stop

**Files:**
- Create: `internal/mobcommands/attack_fullstop_test.go`
- Modify: `internal/mobcommands/attack.go` (two `fmt.Sprintf` literals, master lines 103 and 153)

- [ ] **Step 1: Write the failing test**

Create `internal/mobcommands/attack_fullstop_test.go`:

```go
package mobcommands

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

var fullStopTagPattern = regexp.MustCompile(`<[^>]*>`)

// requireRoomAggroLinesEndWithFullStop checks every captured "prepares to
// fight" ROOM line (not the victim's own "prepares to fight you!") ends with a
// full stop once tags are stripped. The #382 playtest read "A figure prepares
// to fight a figure" with none, unlike the player command's lines.
func requireRoomAggroLinesEndWithFullStop(t *testing.T, captured *[]events.Message) {
	t.Helper()
	events.ProcessEvents()
	seen := 0
	for _, m := range *captured {
		plain := strings.TrimSpace(fullStopTagPattern.ReplaceAllString(m.Text, ""))
		if !strings.Contains(plain, "prepares to fight") || strings.Contains(plain, "prepares to fight you") {
			continue
		}
		seen++
		require.True(t, strings.HasSuffix(plain, "."), "room aggro line lacks a full stop: %q", plain)
	}
	require.NotZero(t, seen, "no room aggro line was captured; the probe cannot fail")
}

func TestMobAttackMob_RoomLineEndsWithFullStop(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	attacker, room := getTestMobAndRoom(t)
	var targetInstanceId int
	for _, id := range room.GetMobs() {
		if id != attacker.InstanceId {
			targetInstanceId = id
			break
		}
	}
	require.NotZero(t, targetInstanceId, "need a second mob in the room to attack")

	captured, _, done := captureAnnounces(t)
	defer done()

	_, err := Attack(fmt.Sprintf("#%d", targetInstanceId), attacker, room)
	require.NoError(t, err)
	requireRoomAggroLinesEndWithFullStop(t, captured)
}

func TestMobAttackPlayer_RoomLineEndsWithFullStop(t *testing.T) {
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"city": {BiomeId: "city"},
	}))

	room := &rooms.Room{RoomId: 8300, Biome: "city"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{8300: room}, map[string]*rooms.ZoneConfig{}))

	m := &mobs.Mob{
		MobId: 8300, InstanceId: 8301, HomeRoomId: 8300,
		Character: characters.Character{
			Name: "Lurker", RoomId: 8300, Health: 100,
			Conditions: conditions.New(), Cooldowns: map[string]int{}, SpeciesId: 1,
		},
	}
	m.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(8301, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(8301, nil) })
	room.AddMob(8301)

	victim := users.NewTestUser(8310, "kesh", "Kesh", 98310)
	onlooker := users.NewTestUser(8311, "oryn", "Oryn", 98311)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8310: victim, 8311: onlooker}))
	room.AddPlayer(8310)
	room.AddPlayer(8311)

	captured, _, done := captureAnnounces(t)
	defer done()

	_, err := Attack("kesh", m, room)
	require.NoError(t, err)
	requireRoomAggroLinesEndWithFullStop(t, captured)
}
```

`captureAnnounces`, `seedAllRegistries` and `getTestMobAndRoom` already exist in the package's tests (`attack_announce_test.go:16`, `mobcommands_test.go:72`, `:280`).

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/mobcommands -run RoomLineEndsWithFullStop -count=1`

Expected (dry run):
```
        	Test:       	TestMobAttackMob_RoomLineEndsWithFullStop
        	Messages:   	room aggro line lacks a full stop: "A figure prepares to fight a figure"
--- FAIL: TestMobAttackPlayer_RoomLineEndsWithFullStop (0.00s)
        	Test:       	TestMobAttackPlayer_RoomLineEndsWithFullStop
        	Messages:   	room aggro line lacks a full stop: "A figure prepares to fight a figure"
FAIL	github.com/GoMudEngine/GoMud/internal/mobcommands
```

- [ ] **Step 3: Add the full stops**

In `internal/mobcommands/attack.go`, Edit:

old_string:
```go
prepares to fight <ansi fg="username">%s</ansi>`, mob.Character.Name, u.Character.Name),
```
new_string:
```go
prepares to fight <ansi fg="username">%s</ansi>.`, mob.Character.Name, u.Character.Name),
```

Then Edit:

old_string:
```go
prepares to fight <ansi fg="mobname">%s</ansi>`, mob.Character.Name, m.Character.Name))
```
new_string:
```go
prepares to fight <ansi fg="mobname">%s</ansi>.`, mob.Character.Name, m.Character.Name))
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/mobcommands -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/internal/mobcommands`

- [ ] **Step 5: Commit**

```bash
git add internal/mobcommands/attack.go internal/mobcommands/attack_fullstop_test.go
git commit -m "fix(copy): the mob aggro room line ends with a full stop (#382)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task F6c: "On the Ground" wraps to 80

**Why:** the list goes out as `CategoryRoomDescription` (look) or `CategorySystem` (search), which the pipeline does not wrap (`internal/messaging/pipeline.go:103-104`), so a room with a few corpses printed 116 columns. `RenderRoster` (`internal/actions/roster.go:19`) solved the same problem for "Also here:" (#430); `RenderGround` sits beside it.

**Files:**
- Create: `internal/actions/ground_test.go`
- Modify: `internal/actions/roster.go`, `internal/actions/search.go`, `internal/usercommands/look.go`, `internal/actions/context.md`

- [ ] **Step 1: Write the failing tests**

Create `internal/actions/ground_test.go`:

```go
package actions

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/require"
)

// #382 playtest: "On the Ground: corpse of a figure, ..." printed one line of
// 116 columns. Like the roster (#430) it goes out as room-description or
// system text, which the pipeline does not wrap. RenderGround wraps it to the
// reader's width.
func TestRenderGround_WrapsToTheLineWidth(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud")
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(root)
	configs.SetConfigForTest(t, cfg)
	templates.SetFSForTest(t, os.DirFS(root).(fs.ReadFileFS))

	out := RenderGround([]string{
		"corpse of a figure",
		"corpse of a figure",
		"Iron Longsword",
		"Torch",
		"Cotton Shirt",
		"Worn Boots",
	}, true, false, 0)
	require.Contains(t, out, "On the Ground:")
	require.Contains(t, out, "Worn Boots")

	tags := regexp.MustCompile(`<[^>]*>`)
	plain := tags.ReplaceAllString(out, "")
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	require.Greater(t, len(lines), 1, "a ground list this long must wrap: %q", plain)
	for _, line := range lines {
		require.LessOrEqual(t, len([]rune(line)), 80, "ground line too wide: %q", line)
	}

	require.Empty(t, RenderGround(nil, false, false, 0), "an empty floor prints nothing")
}

// Every ground send goes through RenderGround, so no caller can print the
// list unwrapped again.
func TestOnTheGroundTemplateRenderedOnlyByRenderGround(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	internalDir := filepath.Join(filepath.Dir(here), "..")
	scanned := 0
	err := filepath.WalkDir(internalDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		if strings.Contains(string(data), `"descriptions/ontheground"`) && filepath.Base(path) != "roster.go" {
			t.Errorf("%s renders descriptions/ontheground directly; use actions.RenderGround", path)
		}
		return nil
	})
	require.NoError(t, err)
	require.NotZero(t, scanned)
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/actions -run 'RenderGround|OnTheGroundTemplate' -count=1`

Expected (dry run):
```
internal\actions\ground_test.go:30:9: undefined: RenderGround
internal\actions\ground_test.go:49:19: undefined: RenderGround
FAIL	github.com/GoMudEngine/GoMud/internal/actions [build failed]
```

- [ ] **Step 3: Add `RenderGround` and route both sites through it**

In `internal/actions/roster.go`, Edit (append after `RenderRoster`):

old_string:
```go
	return messaging.WrapAnsi(text, users.GetByUserId(userId).GetLineWidth())
}
```
new_string:
```go
	return messaging.WrapAnsi(text, users.GetByUserId(userId).GetLineWidth())
}

// RenderGround renders the "On the Ground: ..." list (the
// descriptions/ontheground template) for userId and wraps it like
// RenderRoster: it goes out on the same two unwrapped categories, and a room
// with a few corpses printed one 116-column line (#382 playtest). Every
// ground send goes through here. An empty list renders nothing.
func RenderGround(groundStuff []string, isDark bool, isNight bool, userId int) string {
	details := map[string]any{
		`GroundStuff`: groundStuff,
		`IsDark`:      isDark,
		`IsNight`:     isNight,
	}
	text, _ := templates.Process("descriptions/ontheground", details, userId)
	if text == "" {
		return ""
	}
	return messaging.WrapAnsi(text, users.GetByUserId(userId).GetLineWidth())
}
```

In `internal/usercommands/look.go`, Edit:

old_string:
```go
	groundDetails := map[string]any{
		`GroundStuff`: groundStuff,
		`IsDark`:      !room.IsLit(),
		`IsNight`:     gametime.IsNight(),
	}
	textOut, _ = templates.Process("descriptions/ontheground", groundDetails, user.UserId)
	if len(textOut) > 0 {
```
new_string:
```go
	textOut = actions.RenderGround(groundStuff, !room.IsLit(), gametime.IsNight(), user.UserId)
	if len(textOut) > 0 {
```

(`look.go` keeps its `templates` import; it renders other templates.)

In `internal/actions/search.go`, Edit:

old_string:
```go
		details := map[string]any{
			"GroundStuff": stashedNames,
			"IsDark":      !room.IsLit(),
			"IsNight":     gametime.IsNight(),
		}
		text, _ := templates.Process("descriptions/ontheground", details, actor.GetUserId())
		actor.SendText(messaging.CategorySystem, text)
```
new_string:
```go
		actor.SendText(messaging.CategorySystem,
			RenderGround(stashedNames, !room.IsLit(), gametime.IsNight(), actor.GetUserId()))
```

Then drop the now unused import in `search.go`. Edit:

old_string:
```go
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/templates"
```
new_string:
```go
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
```

In `internal/actions/context.md`, Edit:

old_string:
```
`look` and a search's find both send it) |
```
new_string:
```
`look` and a search's find both send it; `RenderGround` does the same for the "On the Ground:" list from `descriptions/ontheground`, and a guard test keeps every ground send on it) |
```

- [ ] **Step 4: Run them to verify they pass**

Run: `go build ./... && go test ./internal/actions -run 'RenderGround|OnTheGroundTemplate' -count=1 -v`
Expected:
```
--- PASS: TestRenderGround_WrapsToTheLineWidth (0.00s)
--- PASS: TestOnTheGroundTemplateRenderedOnlyByRenderGround (0.10s)
ok  	github.com/GoMudEngine/GoMud/internal/actions
```
Then `go test ./internal/actions ./internal/usercommands -count=1`: both `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/actions/roster.go internal/actions/ground_test.go internal/actions/search.go internal/usercommands/look.go internal/actions/context.md
git commit -m "fix(copy): On the Ground wraps to the reader's width like the roster (#382)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task F6d: `time` says dusk or dawn while the lamps are lit

**Why:** `time` printed `gd.Night` (`modules/time/time.go:64-69`), the geometric sunset, so at 4:12PM it said "daytime" while the low sun already dimmed the streets and lit the lamps. `LampsLit()` (`internal/gametime/celestial.go:128`) is true at night and also while the clear sky through the dimmest street reads below faces (`LampsLitAt`, `:116-125`), which is exactly the twilight band at both ends of the day. The module's `context.md` says calendar logic belongs in `internal/gametime`, so the period rule goes there and the module only formats. The line also gains its final full stop.

**Files:**
- Create: `internal/gametime/day_period_test.go`, `modules/time/time_line_test.go`
- Modify: `internal/gametime/celestial.go`, `internal/gametime/context.md`, `modules/time/time.go`, `modules/time/context.md`

- [ ] **Step 1: Write the failing tests**

Create `internal/gametime/day_period_test.go`:

```go
package gametime

import "testing"

// #382 playtest: `time` said "It is daytime" at 4:12PM after the dusk notice
// had already dimmed the street. While the lamps are lit and it is not yet
// night, the period is dusk (afternoon) or dawn (morning).
func TestDayPeriod(t *testing.T) {
	cases := []struct {
		name     string
		night    bool
		lampsLit bool
		hour24   int
		want     string
	}{
		{"midday", false, false, 12, "day"},
		{"lamps lit in the afternoon", false, true, 16, "dusk"},
		{"lamps lit in the morning", false, true, 7, "dawn"},
		{"night", true, true, 22, "night"},
		{"night before dawn", true, true, 4, "night"},
		{"noon boundary counts as afternoon", false, true, 12, "dusk"},
	}
	for _, c := range cases {
		if got := DayPeriod(c.night, c.lampsLit, c.hour24); got != c.want {
			t.Errorf("%s: DayPeriod(%v, %v, %d) = %q, want %q", c.name, c.night, c.lampsLit, c.hour24, got, c.want)
		}
	}
}
```

Create `modules/time/time_line_test.go`:

```go
package time

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gametime"
)

var timeLineTags = regexp.MustCompile(`<[^>]*>`)

// #382 playtest: the time line read "It is daytime" at dusk and had no final
// full stop.
func TestTimeLine_NamesThePeriodAndEndsWithAFullStop(t *testing.T) {
	gd := gametime.GameDate{Year: 3, Month: 2, Day: 40, Hour: 4, Minute: 12, AmPm: "PM"}
	cases := map[string]string{
		"day":   "It is daytime on day 40",
		"night": "It is nighttime on day 40",
		"dusk":  "It is dusk on day 40",
		"dawn":  "It is dawn on day 40",
	}
	for period, want := range cases {
		plain := timeLineTags.ReplaceAllString(timeLine(gd, period), "")
		if !strings.Contains(plain, want) {
			t.Errorf("period %q: %q does not contain %q", period, plain, want)
		}
		if !strings.HasSuffix(plain, ".") {
			t.Errorf("period %q: %q lacks a final full stop", period, plain)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/gametime -run TestDayPeriod -count=1` and `go test ./modules/time -count=1`

Expected (dry run):
```
internal\gametime\day_period_test.go:24:13: undefined: DayPeriod
FAIL	github.com/GoMudEngine/GoMud/internal/gametime [build failed]
modules\time\time_line_test.go:24:42: undefined: timeLine
FAIL	github.com/GoMudEngine/GoMud/modules/time [build failed]
```

- [ ] **Step 3: Add `DayPeriod` and `timeLine`**

In `internal/gametime/celestial.go`, Edit:

old_string:
```go
func LampsLit() bool {
	return LampsLitAt(IsNight(), CelestialLight(), StreetLampSkyFraction(), configs.GetLightingConfig())
}
```
new_string:
```go
func LampsLit() bool {
	return LampsLitAt(IsNight(), CelestialLight(), StreetLampSkyFraction(), configs.GetLightingConfig())
}

// DayPeriod names the part of the day the `time` command reports: "night"
// while it is night, "dusk" (from noon) or "dawn" (before noon) while the
// lamps are lit but it is not yet night, else "day". Night is the geometric
// sunset; the lamps light earlier, when the low sun no longer shows faces, so
// a 4PM winter street already reads dim (#382 playtest).
func DayPeriod(night bool, lampsLit bool, hour24 int) string {
	switch {
	case night:
		return "night"
	case lampsLit && hour24 >= 12:
		return "dusk"
	case lampsLit:
		return "dawn"
	default:
		return "day"
	}
}
```

In `modules/time/time.go`, Edit:

old_string:
```go
	dayNight := `day`
	if gd.Night {
		dayNight = `night`
	}

	user.SendText(messaging.CategoryTimeOfDay, fmt.Sprintf(`It is now %s. It is <ansi fg="%s">%stime</ansi> on <ansi fg="230">day %d</ansi> of <ansi fg="230">year %d</ansi>. The month is <ansi fg="230">%s</ansi>, and it is the year of the <ansi fg="230">%s</ansi>`,
		gd.String(),
		dayNight,
		dayNight,
		gd.Day,
		gd.Year,
		gametime.MonthName(gd.Month),
		gametime.GetZodiac(gd.Year),
	))

	return true, nil
}
```
new_string:
```go
	// The lamps are read for the current round only; the testing argument
	// moves the date, not the sky, so it falls back to night alone.
	lampsLit := gd.Night
	if rest == `` {
		lampsLit = gametime.LampsLit()
	}

	user.SendText(messaging.CategoryTimeOfDay, timeLine(gd, gametime.DayPeriod(gd.Night, lampsLit, gd.Hour24)))

	return true, nil
}

// timeLine formats the `time` reply for a period from gametime.DayPeriod.
// Day and night keep their "-time" words and colours; dusk and dawn take the
// night colour, since the lamps are lit.
func timeLine(gd gametime.GameDate, period string) string {
	word, color := `daytime`, `day`
	switch period {
	case `night`:
		word, color = `nighttime`, `night`
	case `dusk`, `dawn`:
		word, color = period, `night`
	}
	return fmt.Sprintf(`It is now %s. It is <ansi fg="%s">%s</ansi> on <ansi fg="230">day %d</ansi> of <ansi fg="230">year %d</ansi>. The month is <ansi fg="230">%s</ansi>, and it is the year of the <ansi fg="230">%s</ansi>.`,
		gd.String(),
		color,
		word,
		gd.Day,
		gd.Year,
		gametime.MonthName(gd.Month),
		gametime.GetZodiac(gd.Year),
	)
}
```

(`day` and `night` are the existing colour aliases, `_datafiles/world/dogmud/ansi-aliases.yaml:139-140`; there is no dusk or dawn alias.)

In `internal/gametime/context.md`, Edit:

old_string:
```
func LampsLit() bool
func SetStreetLampSkyFraction(f float64) float64
```
new_string:
```
func LampsLit() bool
func DayPeriod(night bool, lampsLit bool, hour24 int) string
func SetStreetLampSkyFraction(f float64) float64
```

Then Edit:

old_string:
```
  same round. Two readers share it:
  the biome street lamp (`rooms.BiomeInfo.StreetLamp` through
  `Room.LightLevel`) and the behaviour condition `time_of_day period:
  lamplit` (the North Gate arch lantern's `dusk_to_dawn` tree).
```
new_string:
```
  same round. Three readers share it:
  the biome street lamp (`rooms.BiomeInfo.StreetLamp` through
  `Room.LightLevel`), the behaviour condition `time_of_day period:
  lamplit` (the North Gate arch lantern's `dusk_to_dawn` tree), and the
  `time` command through `DayPeriod`.
- `DayPeriod` names the part of the day for the `time` command: "night"
  while it is night, "dusk" (from noon) or "dawn" (before noon) while the
  lamps are lit but it is not yet night, else "day". Before it, `time`
  said "daytime" at a 4PM winter dusk the street already read as dim
  (#382 playtest).
```

In `modules/time/context.md`, Edit:

old_string:
```
## Gotchas
```
new_string:
```
`timeLine(gd, period)` formats the reply for a period from
`gametime.DayPeriod` ("day", "night", "dusk", "dawn"); `TimeCommand` reads
`gametime.LampsLit()` for the current round only, since the testing
argument moves the date, not the sky.

## Gotchas
```

- [ ] **Step 4: Run them to verify they pass**

Run: `go build ./... && go test ./internal/gametime ./modules/time -count=1`
Expected:
```
ok  	github.com/GoMudEngine/GoMud/internal/gametime
ok  	github.com/GoMudEngine/GoMud/modules/time
```

- [ ] **Step 5: Commit**

```bash
git add internal/gametime/celestial.go internal/gametime/day_period_test.go internal/gametime/context.md modules/time/time.go modules/time/time_line_test.go modules/time/context.md
git commit -m "fix(copy): time says dusk or dawn while the lamps are lit (#382)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task F7: the map stays dark

**Why:** `buildAndSend` always added the current room to the `Zone.Map` snapshot (`modules/gmcp/gmcp.Zone.go:93-94`), whatever the player could see, and the snapshot carries the room's title. The D2 gate (`actions.MarkRoomMappedIfSeen`) governs only the saved fog set, so a login in the dark named "Cave Mouth". `buildAndSend` cannot be driven in a test without a negotiated connection (`isGMCPEnabled`, `gmcp.go:79`), so the room-set logic moves into a seam, `zoneMapVisited`, and the test drives that with the existing `roomSightFixture` (`gmcp.RoomSight_test.go:22`; lamp 0 is pitch dark, 30 shapes).

**Files:**
- Create: `modules/gmcp/gmcp.ZoneSight_test.go`
- Modify: `modules/gmcp/gmcp.Zone.go`, `modules/gmcp/context.md`

- [ ] **Step 1: Write the failing tests**

Create `modules/gmcp/gmcp.ZoneSight_test.go`:

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #382 playtest: Zone.Map always added the player's current room, so a
// character who logged in already in the dark read "Cave Mouth" off the map
// payload. The saved fog map (D2) already refused a room seen in the dark;
// the snapshot must agree.
func TestZoneMapVisited_DarkCurrentRoomIsNotAdded(t *testing.T) {
	viewer := roomSightFixture(t, 0)
	room := rooms.LoadRoom(viewer.Character.RoomId)
	require.NotNil(t, room)
	visited := zoneMapVisited(viewer, room, func(int) bool { return true })
	require.NotContains(t, visited, 9720, "a room the player sees nothing in must not reach the map")
}

func TestZoneMapVisited_ShapesCurrentRoomIsAdded(t *testing.T) {
	viewer := roomSightFixture(t, 30)
	room := rooms.LoadRoom(viewer.Character.RoomId)
	require.NotNil(t, room)
	visited := zoneMapVisited(viewer, room, func(int) bool { return true })
	require.Contains(t, visited, 9720, "a room the player can make out belongs on the map")
}

func TestZoneMapVisited_KeepsRoomsAlreadyMapped(t *testing.T) {
	viewer := roomSightFixture(t, 0)
	viewer.Character.MarkRoomVisited("SightZone", 9721)
	room := rooms.LoadRoom(viewer.Character.RoomId)
	require.NotNil(t, room)
	visited := zoneMapVisited(viewer, room, func(int) bool { return true })
	require.Contains(t, visited, 9721, "the dark keeps rooms already seen (R5)")
	require.NotContains(t, visited, 9720)
}
```

- [ ] **Step 2: Extract the seam with master's behaviour, and see the tests fail**

In `modules/gmcp/gmcp.Zone.go`, Edit:

old_string:
```go
	// Every visited room, not just this zone's — a zone boundary is an engine
	// concept the player cannot see, and blanking the map on crossing one reads
	// as amnesia. HasRoom bounds this to rooms THIS zone's crawl reached, which
	// is the zone itself plus the ring of neighbours just over its edges; the
	// crawl already spans boundaries (it is how the builder finds foreign rooms
	// to dim), and those neighbours carry positions in this zone's frame, so
	// they place correctly with no coordinate translation.
	visited := map[int]struct{}{}
	for _, id := range user.Character.GetAllVisitedRooms() {
		if m.HasRoom(id) {
			visited[id] = struct{}{}
		}
	}
	// Always include the current room even before the move-handler marks it.
	visited[room.RoomId] = struct{}{}
```
new_string:
```go
	visited := zoneMapVisited(user, room, m.HasRoom)
```

Then Edit:

old_string:
```go
// ---- Build.Zone.* (admin web-building 4) --------------------------------
```
new_string:
```go
// zoneMapVisited is the room set a Zone.Map snapshot draws.
//
// Every visited room, not just this zone's — a zone boundary is an engine
// concept the player cannot see, and blanking the map on crossing one reads
// as amnesia. inCrawl (the zone mapper's HasRoom) bounds this to rooms THIS
// zone's crawl reached, which is the zone itself plus the ring of neighbours
// just over its edges; the crawl already spans boundaries (it is how the
// builder finds foreign rooms to dim), and those neighbours carry positions in
// this zone's frame, so they place correctly with no coordinate translation.
func zoneMapVisited(user *users.UserRecord, room *rooms.Room, inCrawl func(int) bool) map[int]struct{} {
	visited := map[int]struct{}{}
	for _, id := range user.Character.GetAllVisitedRooms() {
		if inCrawl(id) {
			visited[id] = struct{}{}
		}
	}
	// Always include the current room even before the move-handler marks it.
	visited[room.RoomId] = struct{}{}
	return visited
}

// ---- Build.Zone.* (admin web-building 4) --------------------------------
```

(The moved comment keeps its original em dash; it is a comment, not player copy.)

Run: `go test ./modules/gmcp -run ZoneMapVisited -count=1`

Expected (dry run):
```
--- FAIL: TestZoneMapVisited_DarkCurrentRoomIsNotAdded (0.00s)
        	Error:      	map[int]struct {}{9720:struct {}{}} should not contain 9720
        	Messages:   	a room the player sees nothing in must not reach the map
--- FAIL: TestZoneMapVisited_KeepsRoomsAlreadyMapped (0.00s)
        	Error:      	map[int]struct {}{9720:struct {}{}, 9721:struct {}{}} should not contain 9720
FAIL	github.com/GoMudEngine/GoMud/modules/gmcp
```
(`TestZoneMapVisited_ShapesCurrentRoomIsAdded` passes on master; it guards the gate from overreaching.)

- [ ] **Step 3: Gate the current room on sight**

In `modules/gmcp/gmcp.Zone.go`, Edit:

old_string:
```go
	// Always include the current room even before the move-handler marks it.
	visited[room.RoomId] = struct{}{}
	return visited
```
new_string:
```go
	// Include the current room even before the move-handler marks it, but
	// only when the player can see it: the fog map adds no room while the
	// player sees nothing (#252, R5), and a login in the dark used to name the
	// room here (#382 playtest).
	if messaging.ParticipantSight(user.Character, room) != messaging.SightNone {
		visited[room.RoomId] = struct{}{}
	}
	return visited
```

And the import. Edit:

old_string:
```go
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
```
new_string:
```go
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
```

In `modules/gmcp/context.md`, Edit:

old_string:
```
  map, and re-sends `Room.Info`, so lighting a torch in place refreshes both.
```
new_string:
```
  map, and re-sends `Room.Info`, so lighting a torch in place refreshes both.
  `zoneMapVisited` (`gmcp.Zone.go`) builds the snapshot's room set and adds
  the current room only when the player's `ParticipantSight` is not
  `SightNone`, so a login in the dark does not name the room (#382 playtest).
```

- [ ] **Step 4: Run them to verify they pass**

Run: `go build ./... && go test ./modules/gmcp -count=1`
Expected: `ok  	github.com/GoMudEngine/GoMud/modules/gmcp`

- [ ] **Step 5: Gate the section**

```
gofmt -l internal modules *.go
go vet ./internal/actions ./internal/usercommands ./internal/mobcommands ./internal/gametime ./modules/time ./modules/gmcp
go test ./internal/actions ./internal/usercommands ./internal/mobcommands ./internal/gametime ./modules/time ./modules/gmcp -count=1
go test . -count=1
```
Expected (dry run, all of F6 and F7 applied on `f508db20b`): gofmt and vet print nothing; every package `ok` (actions about 57s); root `ok` (about 29s).

- [ ] **Step 6: Commit**

```bash
git add modules/gmcp/gmcp.Zone.go modules/gmcp/gmcp.ZoneSight_test.go modules/gmcp/context.md
git commit -m "fix(gmcp): Zone.Map adds the current room only when the player can see it (#382)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## Gate and re-run

### Task P1: gate and PR

- [ ] **Step 1:** `go build ./...`, `go vet ./...`, and `go test ./... -count=1 2>&1 | grep -v "^time=" | grep -E "^(--- FAIL|FAIL|panic)"`. Expected: the last prints nothing.
- [ ] **Step 2:** `python tools/context_md_audit.py`: no phantom symbols in the touched packages.
- [ ] **Step 3:** Boot check per `dogmud-shipping` (detached worktree). Kill only the PID you started.
- [ ] **Step 4:** Code review of the branch diff against `origin/master` (`superpowers:requesting-code-review`). Fix every Critical and Important finding.
- [ ] **Step 5:** `git push -u origin fix/sight-gates-playtest`, then `gh pr create --repo pruuk/DOGMud --base master --head fix/sight-gates-playtest --title "fix(sight): close-out playtest fixes, blindness, weapons, disruptions (#382)"`. Body: one line per task with `Refs #382`, rulings R7 and R8, the known limits. End with the Claude Code line.

### Task P2: re-run playtest (#382)

After the PR merges. Load `dogmud-playtesting`. On merged master, re-run the cases that failed or never ran:

1. Blindness: `setcondition 3` on an actor, wait for "Your vision slowly returns to normal.", then `look` in a lit room (expect the room). Relog while still blind (expect still blind).
2. Weapons: a fight between armed players at pitch dark and at shapes, spectators at shapes and in the dark. Expect "weapon" and no item names below full sight; the reader's own weapon stays named.
3. Disruptions: a flee mid-cast, a fizzle, a break on hit, for a mob and a player; listeners at no sight hear the sound lines.
4. A sneaker arriving and spotted reads "You slip into the room but something notices you." (dark) or "a figure" (shapes).
5. A mob spawned hidden (for example Pale Lurker, mob 225): not on the roster, no emote, found by `search`.
6. `loot pass` in a party at shapes.
7. Copy: room 4121's idle line, the aggro full stop, "On the Ground" wrapping, `time` near dusk.
8. GMCP: log in already in a dark room; Zone.Map must not name it.

No leak: close #214, #242, #246, #216, #254, #333, #215, #274, #251, #276, #435, #252, #272, #218, #298, #260, #219, #409 with a comment naming the merge commits, tick the #382 checklist, close #382. A leak: file it, fix it, re-run.
