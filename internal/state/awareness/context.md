# awareness — Package Documentation

## Overview

The `internal/state/awareness` package is the second consumer of the
`internal/state` framework. It defines the **Awareness state machine**,
replacing the buff-#9 "Hidden flag" as the canonical source of truth for
"is this character hidden?" and handling state transitions for concealment,
detection, and revealing.

Awareness has four states:

| State | Meaning |
|-------|---------|
| `Visible` | Not hidden; detectable by normal observation. |
| `Concealing` | Sneak attempt in flight; transitioning to Hidden. |
| `Hidden` | Successfully concealed; undetectable by unprepared observers. |
| `Revealing` | Detection or combat entry occurred; transitioning to Visible. |

The machine is mob/player symmetric: both player and mob `Character` instances
carry a `*awareness.Machine` field. The btree event wiring does not fire for
Awareness transitions (there is no btree transition event); instead, the
framework uses internal state-change cascades and hook subscribers to
integrate with the world (buff #9 mirroring, light-source re-rolls, movement
stamina scaling).

---

## Key Components

### Core Files

- **awareness.go** — `Machine` wrapper, state-data structs, all public API
  (transition methods, predicate methods, registration methods, self tracking).
- **transitions.go** — `validTransitions` table constant and Trigger string
  constants.
- **rules.go** — `vetoChain` and veto registration (activity check, detection
  roll).
- **awareness_test.go** — Behavior Matrix tests AW-001 through AW-033.

---

## Key Functions

### Construction

```go
func NewMachine() *Machine
```

Returns a `Machine` in `Visible`. The machine is meant to be paired with
a `Character` via `SetSelf()`/`RegisterMachine()` so detection rolls and
cascading updates can reference the character's stats and inventory —
**but see Gotchas: no production code actually calls either one.**

### Entry into concealment

```go
func (m *Machine) TransitionToConcealing(d ConcealingData, r state.TransitionReason) error
```

Initiates a sneak attempt. Runs the activity veto before delegating to
the inner framework. On success stores `ConcealingData` (`d`) on the
machine. The caller (`actions.Sneak`, `internal/actions/sneak.go`) is
responsible for then rolling the opposed checks against every observer
in the room and calling `ResolveConcealment` with the outcome — the
machine itself does not roll or self-resolve.

### Detection resolution

```go
func (m *Machine) ResolveConcealment(success bool, r state.TransitionReason)
```

Finalizes an in-flight sneak attempt with an outcome already decided by
the caller. No return value. `success=true` transitions `Concealing →
Hidden`; `success=false` transitions `Concealing → Visible`. Idempotent:
no-op if the machine is not currently `Concealing`. The actual opposed
rolls (Perception + Search vs. Dexterity + Skullduggery, via
`actions.CalcSneakScoreVsObserver` / `combat.RunContest`) happen in
`internal/actions/sneak.go`, looping over every player and mob observer
in the room; this method only records the already-computed result.

### Revealing after detection or combat entry

```go
func (m *Machine) TransitionToRevealing(r state.TransitionReason) error
```

Initiates the reveal cascade, typically triggered by a detection roll or by
the Combat Phase `Idle → Engaging` cascade in
`internal/hooks/Awareness_Cascades.go`. Stores the `TransitionReason` for
subscribers to query
why the reveal is happening. Immediately cascades to `Visible` in a single call
(today); future multi-round revealing will defer this.

### Room change notification

```go
func (m *Machine) NotifyRoomChanged(detected bool, r state.TransitionReason)
```

Receives an already-computed detection outcome for a room move (the
caller has run the per-observer detection rolls beforehand). If
`detected` is true and the machine is currently `Hidden`, transitions
`Hidden → Revealing` via `TransitionToRevealing`. If `detected` is
false, or the machine isn't `Hidden`, it's a no-op. It does **not**
take room IDs and does **not** unconditionally reset to `Visible`.
In production, `internal/usercommands/go.go`'s room-entry detection
calls `TransitionToRevealing` directly rather than going through this
method — `NotifyRoomChanged` today is exercised only by
`awareness_test.go`.

### Force visible

```go
func (m *Machine) ForceVisible(r state.TransitionReason)
```

Transitions to `Visible` from any state, clearing all state-data. Used for
logout, character cleanup, and edge cases. Functionally equivalent to
`TransitionToRevealing` but doesn't queue a cascade.

### Activity check registration

```go
func (m *Machine) RegisterActivityCheck(check func() bool)
```

Wired at character-creation time by `Awareness_Vetoes.go` to ensure the
character is not engaged in crafting or casting before allowing the sneak
transition. The closure reads `c.IsActing()` (negated) to check whether
the character is free — querying the Activity machine internally.

### Self reference

```go
func (m *Machine) SetSelf(actorRef state.ActorRef)
func (m *Machine) Self() state.ActorRef
```

`SetSelf()` stores the character's `ActorRef` on the machine so detection
rolls and cascading updates can reference the character by identity without
creating a dependency cycle. Designed to be called at character-creation
time via the `RegisterMachine` flow — but per Gotchas below, nothing in
production actually calls `RegisterMachine`, so `self` is left zero-valued
on every real character today.

### Predicate methods

```go
func (m *Machine) IsHidden() bool       // State == Hidden
```

There is no `IsConcealing()` or `IsRevealing()` — check `State() ==
Concealing` / `State() == Revealing` directly if needed.

### Registry helpers

```go
func RegisterMachine(ref state.ActorRef, m *Machine)
func UnregisterMachine(ref state.ActorRef)
func lookupMachine(ref state.ActorRef) *Machine // unexported, package-internal
```

Framework-maintained registry of active machines, keyed by `ActorRef`.
Populated at character creation (`RegisterMachine` also stamps `m.self =
ref`, making a separate `SetSelf` call redundant when going through the
registry); cleared on logout / despawn. There is no exported `GetMachine`
— `lookupMachine` is unexported and used only inside this package.

---

## Global State

### machineRegistry

```go
var machineRegistry = map[state.ActorRef]*Machine{}
```

Guarded by `registryMu`. **Still never populated in production, and that is
now deliberate.** `lookupMachine` is defined in this package and called from
nowhere -- not even inside it -- so there is nothing for a registration to
serve.

U11 (2026-08-30) considered wiring all five state-machine registries and
concluded the opposite: `internal/state/combatphase` is the only package with a
real lookup consumer, and it replaced its map with an on-demand resolver
(`combatphase.SetMachineResolver`, wired in `internal/hooks`). Populating a map
here would have been pure retention cost with no reader. If awareness ever grows
a cross-character notification, copy the resolver pattern rather than reviving
`RegisterMachine`; see the Gotchas in `internal/state/combatphase/context.md`
for why the map form kept producing cache-coherence bugs.

The registry is designed as the bridge between `ActorRef` (the identity
type used in `TransitionReason`) and the live `Machine` pointer, so the
framework could call notifications on a character's awareness machine
without the caller knowing its memory address. Today nothing exercises
that path outside `awareness_test.go`.

---

## Data Structure Design

### State enum

```go
type State int
const (
    Visible State = iota
    Concealing
    Hidden
    Revealing
)
```

### Per-state data structs

```go
type VisibleData struct{}
    // Empty — default state has no metadata.

type ConcealingData struct {
    RoundsUntil int  // Reserved for multi-round sneak in future chunks.
}

type HiddenData struct{}
    // Empty — reserved for future light-source tracking or
    // per-observer awareness lists.

type RevealingData struct {
    Reason state.TransitionReason  // Why is the character being revealed?
}
```

Stored as nullable pointer fields on `Machine`. Only the pointer for the
current state is non-nil; transitions nil out the previous state's data.

### validTransitions

```go
var validTransitions = state.TransitionTable[State]{
    Visible:    {Concealing},
    Concealing: {Hidden, Visible},     // Visible on detection failure
    Hidden:     {Revealing, Visible},  // Revealing for the cascade; Visible for
                                       // force (logout / death)
    Revealing:  {Visible},             // Visible after cascade
}
```

### Trigger constants

Defined in `transitions.go`. Use these string constants instead of
inline literals to ensure stable identifiers across the codebase:

| Constant | Value |
|----------|-------|
| `TriggerSneakCommand` | `"sneak_command"` |
| `TriggerSneakSuccess` | `"sneak_success"` |
| `TriggerSneakFailed` | `"sneak_failed"` |
| `TriggerCombatEntered` | `"combat_entered"` |
| `TriggerMovementDetected` | `"movement_detected"` |
| `TriggerObserverSearch` | `"observer_search"` |
| `TriggerLightChange` | `"light_change"` |
| `TriggerSkullduggeryFailed` | `"skullduggery_failed"` |
| `TriggerNoisyAction` | `"noisy_action"` |
| `TriggerRangedSurpriseShot` | `"ranged_surprise_shot"` |
| `TriggerLogout` | `"logout_safety_valve"` |
| `TriggerDeath` | `"death_cascade"` |
| `TriggerForceVisible` | `"force_visible"` |

Verified against `transitions.go` 2026-08-25. Four names this table
previously carried -- `TriggerDetectionSuccess`, `TriggerDetectionFailure`,
`TriggerRoomChange`, `TriggerLightSourceChange` -- have never existed in the
package; the last is `TriggerLightChange`.

---

## Integration Notes

### Consumes

- `internal/state` — framework (`Machine[State]`, `TransitionTable`,
  `TransitionReason`, `ActorRef`).
- `internal/characters` — reads `Dexterity`, `Perception`, and
  `Character.Activity` (via `IsActing()`) for veto and detection checks.
- `internal/rooms` — room entry/exit for detecting observers.

### Consumed by

- **`internal/characters`** — `Character.Awareness *awareness.Machine`
  field. Initialized in `New()` and (lazily) in `Validate()`. Predicate
  method `IsHidden()` delegates to the machine.
- **`internal/hooks/Awareness_Vetoes.go`** — wires the activity and
  detection-check callbacks at character-creation time via
  `OnCharacterCreated`.
- **`internal/hooks/Awareness_Cascades.go`** — subscribes to `AfterTransition`
  on both this machine and Combat Phase. Mirrors `Hidden` state to buff #9
  (apply on Hidden entry, remove on Hidden exit), and reveals on Combat
  Phase `Idle → Engaging` — including surprise attacks, which get no
  stealth grace period.
- **`internal/hooks/Awareness_LightChange.go`** — subscribes to light-state-change
  events and re-rolls detection when the room's light state changes (scaffolding
  for future-chunk light-source mechanics; today a no-op pending full design).
- **`internal/hooks/Logout_AwarenessCleanup.go`** — calls `ForceVisible` when a
  character logs out, ensuring the awareness machine doesn't leak state.

### Replaces

No direct replacement, but removes the canonical use of the buff-#9 "Hidden" flag
for detection logic. Buff #9 is now a **side-effect carrier only** — the Awareness
machine is the source of truth; the Cascades hook ensures buff #9 stays in sync.

---

## Gotchas

**`RegisterMachine` IS called as of U11 (2026-08-30); `SetSelf` still is not,
but no longer needs to be.** `(*characters.Character).syncMachineRegistry`
registers this machine under the Character's `ActorRef()`, and
`awareness.RegisterMachine` assigns `m.self = ref` itself
(`awareness.go:127`), so `Self()` is populated as a side effect. The separate
`SetSelf` setter remains uncalled in production.

What is still true:

- `lookupMachine` is called from nowhere inside this package. The registry is
  populated for it, but awareness has no cross-character notification of its
  own yet, so nothing reads it here.
- `NotifyRoomChanged`, despite being fully implemented, is **still not wired
  into any production call site**. The actual room-move reveal path
  (`internal/usercommands/go.go`) calls `TransitionToRevealing` directly on the
  machine it already holds a pointer to, sidestepping the registry entirely.
  U11 wired registration, not this consumer; do not assume the two changed
  together.

This mirrors the same gap documented in
`internal/state/combatphase/context.md` for `combatphase.RegisterMachine`
— both were pre-wired for cross-character lookups that no caller ended
up needing, because every real call site already holds the target's
`*Machine` pointer directly (via the `Character` struct) rather than
going through an `ActorRef` lookup.

---

## Testing Notes

### awareness_test.go — Behavior Matrix

Tests follow the AW-NNN naming scheme from the chunk 1 spec. Each test
exercises one cell of the state × trigger × veto matrix.

| Range | Area |
|-------|------|
| AW-001 – AW-003 | Basic state transitions (happy path) |
| AW-004 – AW-011 | Detection rolls (observer presence, opposed checks) |
| AW-012 – AW-014 | Light-state interaction scaffold |
| AW-015, AW-016 | Combat Phase subscription (reveal on combat entry) |
| AW-019 – AW-020 | Hidden movement stamina cost |
| AW-021 – AW-022 | Logout cascade |
| AW-023 | Activity veto (crafting/casting block) |
| AW-024 – AW-027 | CalcSneakScore refactor tests (in actions/skill_helpers_test.go) |
| AW-028 | ForceVisible from any state |
| AW-029 – AW-030 | Registry helpers |
| AW-031 | IsHidden() predicate |
| AW-032 – AW-033 | Writer migration (noisy actions) |

All tests use local `Machine` instances with injected veto closures;
no server / database setup required. Light-state tests (AW-012 – AW-014)
exercise the scaffold scaffolding but do not yet test light-source re-roll logic
(pending full light-system design in later chunks).

---

## Veto Chain

`vetoChain` (`rules.go`) holds exactly one registered check today:

1. **Activity Veto** (`activitySelf`, registered via
   `RegisterActivityCheck`) — blocks `Visible → Concealing` if the
   character is crafting or casting.

There is no separate "Detection Veto." Detection is not resolved
internally by the machine at all — see Detection Roll Mechanics below.
`ResolveConcealment` takes the outcome as a `success bool` parameter
already decided by the caller; it does not roll anything itself.

---

## Detection Roll Mechanics

**The awareness package does not perform detection rolls.** The rolls
live in `internal/actions/sneak.go` (`actions.Sneak`), which loops over
every player and mob observer in the room, computes both sides of the
contest, and only then calls `ResolveConcealment(success, reason)` with
the aggregate outcome.

### Formula

```
sneaker_roll  = actions.CalcSneakScoreVsObserver(sneaker, observer, room)
              = Dexterity + Skullduggery_rank × SkillWeight (+ light modifier, + stealth mutation bonus)
observer_roll = actions.CalcDetectionScore(observer)
```

Both live in `internal/actions/skill_helpers.go`. The opposed check
itself runs through `combat.RunContest(sneakScore, []contest.Entry{{Score:
observerScore}})` — the shared opposed-contest entry point, and the only
place `Balance.ContestFloor` is applied (`contest.Run` itself is
unfloored) — not a
bespoke roll inside this package.

### Behavior

Any single observer beating the sneaker's score fails the whole attempt
(`ResolveConcealment(false, ...)`, `TriggerSneakFailed`). If no observer
wins, the actor transitions to `Hidden`
(`ResolveConcealment(true, ...)`, `TriggerSneakSuccess`).
