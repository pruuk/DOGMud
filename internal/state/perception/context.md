# perception — Package Documentation

## Overview

The `internal/state/perception` package is the seventh consumer of the
`internal/state` framework — completing the combat-state-machines arc
(chunks 0-6). It defines a two-state FSM (`Sighted | Blinded`) that
gates "does this character's eyes work?" semantics.

**Status (shipped 2026-05-19, consumer landed 2026-05-20):**

The machine transitions correctly via the condition lifecycle hooks for its two
sources, Condition 3 Blinded and Condition 77 Flashbang Blindness. (A third source, a
blinded combat condition, was listed here and never had a producer; the
conditions unification deleted it on 2026-09-12.)
Originally shipped DORMANT in chunk 6 (2026-05-19) with no consumer.
Now consumed by the centralized messaging framework chunk (2026-05-20,
T3 of that chunk) via `internal/messaging/predicates.go:CanSeeClearly`
/ `CanSeeShapes`. Sight-gated visual broadcasts route through the
messaging pipeline; infrared "red shapes" rendering, color coding by
event category, and centralized line wrapping all sit on top of these
predicates.

The dormant-then-consumed lifecycle follows the chunk-4a precedent
(Position FSM shipped DORMANT before chunk 4b wired writers + readers).

---

## State enum

| State | Semantics |
|---|---|
| Sighted | Default — eyes work |
| Blinded | Either active blind source (Condition 3 or Condition 77) |

Two states. No transient states. No state-data structs.

---

## Transition table

```
Sighted → {Blinded}
Blinded → {Sighted}
```

Re-entry (Sighted→Sighted, Blinded→Blinded) is NOT in the table.
Callers must check current state before firing transitions. The one
production caller, `Character.reconcilePerception` (`internal/characters/sight.go`),
does: it fires only when the held blind sources and the state disagree.

---

## Trigger sources

| Source | File | Hook |
|---|---|---|
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

Two sources, both conditions. Detection is by condition ID (not by flag) because the
existing condition YAMLs don't carry a blindness-specific flag, and adding one
would require data file edits. Condition IDs `ConditionIdBlinded = 3` and
`ConditionIdFlashbangBlindness = 77` are constants in `transitions.go`, alongside
the two trigger reasons `TriggerConditionApplied` and `TriggerConditionExpired`. The
`TriggerConditionAdded` / `TriggerConditionRemoved` pair was deleted with the
combat condition enum on 2026-09-12.

---

## Helper: `HasAnyBlindSource`

`Character.HasAnyBlindSource()` (in `internal/characters/sight.go`)
returns true if either source is currently active. Used
by `reconcilePerception` to decide which way the state should point, so one of
two overlapping sources clearing leaves the holder Blinded.

Important implementation detail: the condition checks use
`Conditions.TriggersLeft(id) > 0` rather than `Conditions.HasCondition(id)`. The
condition system marks a condition expired (TriggersLeft=0) on RemoveCondition but
defers map-entry pruning to the next game-tick. HasCondition checks map
membership only and returns true for expired-but-not-yet-pruned conditions,
which would break the overlap guard. TriggersLeft > 0 returns false
immediately after RemoveCondition — correct semantic.

---

## Construction

`NewMachine()` returns a Machine in Sighted state. Same constructor
for both player and mob — no per-actor polymorphism (unlike chunk 5
Presence).

`Character.Perception` field initialized at four sites:

1. `characters.New()` — player default.
2. `Character.Validate()` — nil-guard for YAML-loaded characters.
3. `mobs.Mob.Validate()` — unconditional overwrite.
4. `Character.ResetForMobInstance()` — reset to nil so fresh mob
   instances get their own machine.

---

## Integration points

| When | Where |
|---|---|
| **Chunk 6 (2026-05-19)** | Transitions fire correctly; no consumer yet. |
| **Messaging framework chunk (2026-05-20)** | `messaging.CanSeeClearly` / `CanSeeShapes` in `internal/messaging/predicates.go` read `Perception.State()` to gate visual broadcasts and route infrared observers through the anonymizer. Every `Room.SendTextVisual` call now consults the FSM. |

---

## Testing

- `perception_test.go`: Behavior Matrix unit tests, pure-FSM coverage:
  PE-001, PE-002, PE-003, PE-005, PE-006, PE-008, PE-009. PE-004 and PE-007
  were the two condition-trigger cases and were deleted on 2026-09-12; both
  edges stay covered by their condition-trigger siblings.
- `integration_test.go`: real-Character integration via `AddCondition` /
  `RemoveCondition`: PE-INT-001 through PE-INT-005 and PE-INT-007 (overlap and
  single-source paths). PE-INT-006 was the condition-source case and was
  deleted with the enum. PE-INT-008 (natural expiry through `Prune` then
  `Validate`), PE-INT-009 (a reload keeps live blindness) and PE-INT-010 (the
  magnitude door blinds) pin the reconcile.
- `internal/hooks/blind_expiry_test.go`: the real round prune
  (`PruneConditions`) restores sight for conditions 3 and 77.

No smoke pass at chunk-6 ship — dormant. The messaging framework
chunk (2026-05-20) authored its own AI feature-tester goal at
`tools/testing/goals/messaging-framework-smoke.yaml` covering
sight-gating + infrared rendering end-to-end.
