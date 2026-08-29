# combatphase — Package Documentation

## Overview

The `internal/state/combatphase` package is the first consumer of the
`internal/state` framework. It defines the **Combat Phase state machine**,
replacing `Character.Aggro` as the source of truth for "who am I attacking?"
and "am I in combat?"

Combat Phase has four states:

| State | Meaning |
|-------|---------|
| `Idle` | Not engaged in combat. |
| `Engaging` | Attack initiated; weapon wait-rounds countdown in progress. |
| `Engaged` | Actively fighting; round driver processes swings. |
| `Disengaging` | Flee in progress; waiting for flee resolution. |

The machine is mob/player symmetric: both player and mob `Character` instances
carry a `*combatphase.Machine` field. The btree event wiring fires only for
mob instances (players have no behavior tree); combat round dispatch reads the
same predicates for both.

---

## Key Components

### Core Files

- **combatphase.go** — `Machine` wrapper, state-data structs, all public API
  (transition methods, predicate methods, registration methods, Attackers
  tracking, tick-event dispatch).
- **transitions.go** — `validTransitions` table constant and Trigger string
  constants.
- **combatphase_test.go** — Behavior Matrix tests CP-001 through CP-036.

---

## Key Functions

### Construction

```go
func NewMachine() *Machine
```

Returns a `Machine` in `Idle`. `RegisterMachine` is meant to be called
immediately after creating a character so inbound-attacker tracking and
cross-character target-death notifications work correctly — **but see
the Gotchas section below: in production today, nothing calls it.**

### Entry into combat

```go
func (m *Machine) TransitionToEngaging(d EngagingData, r TransitionReason) error
```

Primary entry point into combat. Runs the seven veto chain (Combatant,
Activity, Life, Position, TargetCombatant, TargetLife, TargetPresence)
before delegating to the inner framework transition. On success:
- Stores `EngagingData` on the machine.
- Calls `target.RecordInboundAttacker(r.Actor)` to register self on the
  target's inbound list (enabling companion auto-assist).

### Disengaging

```go
func (m *Machine) TransitionToDisengaging(r TransitionReason) error
```

Starts a flee attempt. Vetoes via `positionSelf` when trigger is
`TriggerFleeCommand` and the character is grappled/clinched/grounded.

### Round tick

```go
func (m *Machine) OnRoundTick()
```

Called once per round per character by the round driver. In `Engaging`
state: decrements `RoundsUntil` and advances to `Engaged` when it hits
zero. In `Engaged` and `Disengaging`: no-op (those states are driven
externally by the combat resolution and flee logic).

### Flee resolution

```go
func (m *Machine) ResolveFlee(success bool)
```

Finalizes a `Disengaging` state. Success → `ForceIdle`
(`TriggerFleeSuccess`). Failure → back to `Engaged` with the last known
target restored in `EngagedData`.

### Force-idle

```go
func (m *Machine) ForceIdle(r TransitionReason)
```

Transitions to `Idle` from any state, clearing all state-data and
removing self from the target's inbound list. Used for death cascade,
`Combatant`-toggle, target-died, and `EndAggro`.

### Target-died notification

```go
func (m *Machine) NotifyTargetDied(target ActorRef)
```

Designed to be called by the dying target's `NotifySelfDied`. If my
current target matches, transitions to `Idle`. **Not called from
production** — see Gotchas. Death cleanup goes through `ForceIdle`
directly instead (`internal/hooks/Life_Cascades.go`).

### Self-died notification

```go
func (m *Machine) NotifySelfDied()
```

Designed to be called when this character dies: clears own outbound
state, clears all inbound attackers, and notifies each inbound attacker
that their target is gone (causing them to force-idle too). **Not called
from production** — see Gotchas. Only exercised by
`combatphase_test.go`. Actual death handling in
`internal/hooks/Life_Cascades.go` calls `ForceIdle` on the dying
character's own machine only; it does not cascade to attackers via this
method (unsurprising, since `Attackers()` is always empty in production
regardless).

### Tick-event dispatch

```go
func (m *Machine) DispatchTickEvent()
```

Fires the per-state tick event to all registered listeners:
- `Engaged` → fires `"mob_combat_round"`
- `Idle` → fires `"mob_idle"`
- `Engaging` / `Disengaging` → silent (no event)

Called once per round per character by the round driver before combat
resolution. Drives btree idle and combat-round events without polling.

### Predicate methods

```go
func (m *Machine) IsEngaged() bool       // State == Engaged
func (m *Machine) IsInCombat() bool      // State != Idle
func (m *Machine) CurrentTarget() ActorRef  // target across all non-Idle states
```

### Attackers

```go
func (m *Machine) Attackers() []state.ActorRef
func (m *Machine) RecordInboundAttacker(a state.ActorRef)
func (m *Machine) RemoveInboundAttacker(a state.ActorRef)
func (m *Machine) SubscribeAttackersChange(fn func([]state.ActorRef))
```

Framework-maintained inbound attacker list. `TransitionToEngaging` calls
`RecordInboundAttacker` on the target; `ForceIdle` calls
`RemoveInboundAttacker`. `SubscribeAttackersChange` is used by
`CombatPhase_CompanionAssist.go` to trigger reactive companion auto-assist
the moment a new attacker is recorded. **See Gotchas: none of this fires
in production** — `TransitionToEngaging` looks the target machine up via
the (never-populated) registry, so `RecordInboundAttacker` is never
actually called on a real target and `Attackers()` always reads empty.

### Veto registration

```go
func (m *Machine) RegisterCombatantVeto(check func() bool)
func (m *Machine) RegisterActivityCheck(check func() bool)
func (m *Machine) RegisterLifeCheck(check func() bool)
func (m *Machine) RegisterPositionCheck(check func() bool)
func (m *Machine) RegisterTargetCombatantCheck(c func(ActorRef) bool)
func (m *Machine) RegisterTargetLifeCheck(c func(ActorRef) bool)
func (m *Machine) RegisterTargetPresenceCheck(c func(ActorRef) bool)
```

Wired at character-creation time by `CombatPhase_Vetoes.go` via the
`OnCharacterCreated` callback. Each registration accepts a closure over
the character's own fields so the checks always read current values.

#### Presence veto on `Idle→Engaging` (chunk 5)

`RegisterTargetPresenceCheck` is populated by `hooks.wireCombatPhaseVetoes`
(chunk 5 T6). The closure reads the TARGET's `Presence.State()` and
returns false (block) for two terminal states:

- `presence.Disconnected` — TCP is gone; the character is in the graveyard.
- `presence.Despawning` — mob is on its last tick before removal.

Idle, AFK, and Dormant targets are NOT blocked — a player who typed `afk`
in a dangerous room still takes hits by design (see §2 of the chunk-5
spec). The NoAggroTarget grace buff (#81) also blocks `Idle→Engaging` via
the same `RegisterTargetPresenceCheck` hook for newly-respawned players.
This is orthogonal to Presence state: NoAggroTarget blocks regardless of
Presence, and Presence's Disconnected/Despawning block regardless of
NoAggroTarget. Neither subsumes the other.

### Surprise round

There is no surprise round. `TriggerSurpriseAttack` still distinguishes an
ambush from an ordinary attack in the `TransitionReason`, but the machine
gives it no special treatment: a hidden attacker takes the same
`Idle → Engaging` cascade as anyone else and is revealed on engaging. The
ambusher's opening strike is keyed off `Character.Aggro.Type`, not off
anything on this machine.

---

## Global State

### machineRegistry

```go
var machineRegistry = map[state.ActorRef]*Machine{}
```

Guarded by `registryMu`. Populated by `RegisterMachine` at character
creation; cleared by `UnregisterMachine` on logout / despawn.

The registry is the bridge between `ActorRef` (the identity type used
in `TransitionReason.Target`) and the live `Machine` pointer. It allows
the framework to call `NotifyTargetDied` and `RecordInboundAttacker` on
a target's machine without the caller knowing its memory address.

---

## Data Structure Design

### State enum

```go
type State int
const (
    Idle State = iota
    Engaging
    Engaged
    Disengaging
)
```

### Per-state data structs

```go
type EngagingData struct {
    Target      state.ActorRef
    Reason      state.TransitionReason  // caller-filled only; see gotcha below
    RoundsUntil int  // weapon WaitRounds before first swing
}
```

> `EngagingData.Reason` is read by nothing today, and
> `TransitionToEngaging(d, r)` does **not** copy its `r` argument into
> `d.Reason` — it stores `d` verbatim. The one production caller
> (`Character.SetAggro`) leaves `Reason` zero-valued. If you need the
> transition reason, take it from the `AfterTransition` callback's `r`
> argument, which is correct.

```go
type EngagedData struct {
    Target      state.ActorRef
    NextSwingAt int  // round number for next swing
}

type DisengagingData struct {
    LastTarget state.ActorRef  // target at time of flee initiation
    FleeRound  int             // round flee was initiated
}
```

Stored as nullable pointer fields on `Machine`. Only the pointer for the
current state is non-nil; transitions nil out the previous state's data.

### validTransitions

```go
var validTransitions = state.TransitionTable[State]{
    Idle:        {Engaging},
    Engaging:    {Engaged, Idle, Engaging},     // Idle on cancel / target-died; Engaging on RETARGET
    Engaged:     {Disengaging, Idle, Engaging}, // Idle direct on death / despawn; Engaging on RETARGET
    Disengaging: {Idle, Engaged},               // Engaged on flee failure
}
```

**Retarget is a fresh engagement.** Both `Engaging → Engaging` and
`Engaged → Engaging` exist so that switching targets mid-fight lands: new
target, fresh `RoundsUntil` wind-up, then back to `Engaged`. Before U12c-0 the
transition was refused and `SetAggro` discarded the error, so `CurrentTarget()`
kept returning the PREVIOUS enemy — which the `{target}` and `{targethealth}`
prompt tokens render.

**`Disengaging → Engaging` is deliberately absent.** Fleeing is a commitment;
the only way back into combat is `Disengaging → Engaged` on flee failure. Do
not add it without reading `handlePlayerFlee`, which reads `IsDisengaging()`
first and authoritatively.

⚠️ **A refused transition now refuses the whole commit.** Since U12c-0b,
`characters.SetAggro` attempts this transition FIRST and writes nothing if it
fails, so the six registered vetoes (`hooks/CombatPhase_Vetoes.go`) are
load-bearing rather than advisory. Any caller that needs to know whether an
engagement actually started must re-check state afterwards — `targeting.Commit`
is void. `hooks.RetargetOrEnd` is the worked example: it returns
`char.Aggro != nil`, not a bare `true`, because its callers dereference
`char.Aggro` on the strength of that return.

### Trigger constants

Defined in `transitions.go`. Use these string constants instead of
inline literals to ensure stable identifiers across the codebase:

| Constant | Value |
|----------|-------|
| `TriggerAttackCommand` | `"attack_command"` |
| `TriggerSurpriseAttack` | `"surprise_attack"` |
| `TriggerEngagementReady` | `"engagement_ready"` |
| `TriggerFleeCommand` | `"flee_command"` |
| `TriggerFleeSuccess` | `"flee_success"` |
| `TriggerFleeFailure` | `"flee_failure"` |
| `TriggerTargetDied` | `"target_died"` |
| `TriggerSelfDied` | `"self_died"` |
| `TriggerTargetOutOfRoom` | `"target_out_of_room"` |
| `TriggerCharm` | `"charm_acquired"` |
| `TriggerCombatantToggle` | `"combatant_toggle"` |
| `TriggerForceIdle` | `"force_idle"` |

---

## Integration Notes

### Consumes

- `internal/state` — framework (`Machine[State]`, `TransitionTable`,
  `TransitionReason`, `ActorRef`).

### Consumed by

- **`internal/characters`** — `Character.CombatPhase *combatphase.Machine`
  field. Initialized in `New()` and (lazily) in `Validate()`. Predicate
  methods `IsEngaged()`, `IsInCombat()`, `IsDisengaging()`,
  `EngagedTarget()`, `CurrentCombatTarget()`, `Attackers()` delegate to
  the machine.
- **`internal/hooks/CombatPhase_Vetoes.go`** — wires the seven veto
  callbacks at character-creation time via `OnCharacterCreated`.
- **`internal/hooks/CombatPhase_BtreeEvents.go`** — registers an
  `AfterTransition` cascade that fires `mob_engaging`, `mob_engaged`,
  `mob_disengaging`, `mob_combat_ended` btree events on state changes.
- **`internal/hooks/CombatPhase_CompanionAssist.go`** — registers
  `SubscribeAttackersChange` to reactively trigger companion auto-assist
  when a new attacker is recorded on the companion's inbound list.

### Replaces

`Character.Aggro` field (compat wrappers in
`internal/characters/combat_state_compat.go`). The `Aggro` struct and
`SetAggro` / `EndAggro` methods are preserved as a compat surface for
the ~200 direct field reads across the codebase that were not migrated
in chunk 0. All writes dual-write to `CombatPhase`. Full field removal
is scheduled for a cleanup chunk after chunks 1-5 land.

---

## Gotchas

**`RegisterMachine` has no production caller, for either this package or
`internal/state/awareness`.** `internal/characters/validate.go` constructs
`c.CombatPhase = combatphase.NewMachine()` directly and never registers
it in `machineRegistry`. Confirmed by grep (`combatphase.RegisterMachine`
and `awareness.RegisterMachine` appear only in this package's/awareness's
own tests, and in an explanatory comment in
`internal/actions/combat_fire.go:421`). Consequences, verified against
`TransitionToEngaging` (`lookupMachine(d.Target)` — always nil for a real
character):

- `RecordInboundAttacker` is never invoked on a real target's machine, so
  **`Attackers()` always reads empty in production.** The Attackers
  section below describes the intended mechanism, not what happens today.
- `internal/hooks/CombatPhase_CompanionAssist.go`'s reactive
  `SubscribeAttackersChange` handler therefore never fires in production —
  companion auto-assist runs only through the polling fallback
  (`CompanionAutoTarget` in `combat_retarget.go`, called from
  `NewRound_DoCombat`), not reactively.
- `internal/actions/combat_fire.go`'s `shooterIsUnengaged` deliberately
  does NOT use `Character.Attackers()` for this reason — it does a live
  room scan instead, with a comment explaining why.
- `NotifyTargetDied` / `NotifySelfDied` are likewise never called from
  production (only from `combatphase_test.go`). Real death cleanup
  (`internal/hooks/Life_Cascades.go`, `Alive → Dead`) calls `ForceIdle`
  directly on the dying character's own machine and does not cascade to
  attackers via `NotifySelfDied` — which would have nothing to iterate
  over anyway, since `Attackers()` is always empty.

**A second, independent break sits behind the first.** `SetAggro` builds the
attacker ref as `Actor: state.ActorRef{UserId: c.userId}`
(`internal/characters/combat_state_compat.go:146`), and nothing ever calls
`SetUserId` on a mob, so a mob's `userId` stays 0 and the ref is
`{UserId:0, MobInstanceId:0}` — which `ActorRef.IsZero()`
(`internal/state/transition.go:12-14`) reports true, and
`RecordInboundAttacker` early-returns on. **Even with a working registry, no
mob attacking a player would ever be recorded.** Anyone repairing this must fix
both halves; fixing only the registry silently leaves mobs invisible.

**An existing consumer is already inert because of this:** `recoveryContest`
(`internal/hooks/recovery_contest.go:23`) iterates `ch.Attackers()`, finds
nothing, and returns `nil` — which that function documents as *a free stand*.
Prone recovery has therefore never actually been contested, whatever U10
intended. See `internal/hooks/context.md`.

Handed to **U11** as a wire-it-up-or-delete-it decision; see the "U11 inbox
from U10d" section of `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`.

**`TransitionToEngaging` silently drops its `TransitionReason`.** The signature
is `TransitionToEngaging(d EngagingData, r state.TransitionReason) error`
(`combatphase.go:208`), and at `:232` it does `m.engaging = &d` — storing the
caller's struct verbatim and **never copying `r` into `d.Reason`**. So
`EngagingData.Reason` (`combatphase.go:41`) is the zero value on every real
character. This is what made `EngagedData.SurpriseLeft` false in production for
its entire life: `advanceToEngaged` computed it from `prevEngaging.Reason.Trigger`,
which was never set, while `Awareness_Cascades.go` read the correctly-passed `r`
and so appeared to work. U10d deleted `SurpriseLeft`, which removed the only
consumer, so the trap is now **latent rather than live** — `EngagingData.Reason`
is read by nothing today. It was deliberately not repaired: fixing a producer
whose sole consumer was being deleted would add a live code path nothing uses.
**Any future consumer of `EngagingData.Reason` must fix the producer first.**

This is a pre-existing gap (not introduced by U10d) but it means any
claim elsewhere in this doc about "the round driver wires this at
character-creation time" describes the designed data flow, not a
currently-exercised one, for the registry-dependent pieces specifically
(`Attackers`/`RecordInboundAttacker`/`RemoveInboundAttacker`/
`SubscribeAttackersChange`/`NotifyTargetDied` cross-character lookups).
Machine-local behavior (state transitions, vetoes, tick dispatch) is
unaffected and works normally without registration.

---

## Testing Notes

### combatphase_test.go — Behavior Matrix

Tests follow the CP-NNN naming scheme from the chunk 0 spec. Each test
exercises one cell of the state × trigger × veto matrix.

| Range | Area |
|-------|------|
| CP-001 – CP-008 | Basic state transitions (happy path) |
| CP-010 – CP-016 | Veto rules (non-combatant, dead, busy, grappled) |
| CP-017 – CP-022 | ForceIdle + NotifySelfDied cascade |
| CP-024 | Engaged carries the Engaging target |
| CP-026 – CP-036 | Tick events, attacker list, flee, retarget, edge cases |

All tests use local `Machine` instances with injected veto closures;
no server / database setup required.
