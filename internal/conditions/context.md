# GoMud Conditions System Context

## Overview

The conditions system provides timed status effects for characters with support for stat modifications, behavioral flags, round-based triggers, and duration management. It has a dual-layer architecture with immutable condition specifications and mutable condition instances, supporting complex timing mechanics, permanent conditions, and flag-based behavior modification.

## Names

Since conditions unification slice 3 (2026-09-15) every file, save key, GMCP field, category string, colour alias and template function also says condition; `internal/conditionrename` holds the spelling map and migration 0.17.0 converted old saves.

## Not these conditions

The word is used by four unrelated things. None of them is this package:

- **Behaviour tree condition nodes**: the predicate leaves of a mob's
  behaviour tree, in `internal/behaviortree/conditions_*.go`
  (`conditions_combat.go`, `conditions_mob.go`, `conditions_room.go`, ...).
  The node `mob_has_condition` is one of them; it reads this package.
- **Quest trigger conditions**: `quests.Conditions`
  (`internal/quests/triggers.go`), the gate on when a trigger may fire. The
  quest ACTION that applies one of these records is
  `ApplyStatusCondition *StatusConditionDef` (YAML key `apply_condition:`), named
  that way to stay apart from the trigger's `Conditions`.
- **Web client trigger conditions**: the `condition` on a player-defined
  client trigger, evaluated by `evalTriggerCondition` in
  `_datafiles/html/public/webclient-pure.html`.
- **Bounty conditions**: `bounties.Condition` (`internal/bounties/types.go`),
  a string type naming what a contract asks for (`ConditionKill`).

## One collection of timed state (slice 1 of the conditions unification, 2026-09-12)

There is ONE collection of timed state on a character: `Character.Conditions`
(saved under the `conditions:` key). The former `characters.CombatCondition` enum
(ten combat conditions with their own tick, magnitude and display) is gone;
the ten became NINE records under `_datafiles/world/dogmud/conditions/` (79, 80,
117 to 123), because blinded had no producer and was deleted rather than
ported. **Do not add a second collection or a Go enum of timed effects**; the
root guard `timed_state_guard_test.go` fails the build on a struct outside
this package that pairs a duration field with a `Magnitude`, on the deleted
enum's own names, and on an unlisted wrapper method named like the primitive's
verbs.

| Was a combat condition | Is now the record | Id |
|---|---|---|
| Warcry | Warcry | 79 |
| Rally | Rally | 80 |
| Defense penalty (failed grapple) | Off Balance | 117 |
| Recovery penalty (standing up) | Recovering | 118 |
| Shield | Minor Shield | 119 |
| Regenerating | Regenerating | 120 |
| Poisoned (the spell dot) | Poisoned | 121 |
| Bleeding | Bleeding | 122 |
| Enchant withdrawal | Enchant Withdrawal | 123 |
| Blinded | nothing; it had no producer, and perception rides records 3 and 77 | |

`ids.go` holds those ids as constants (`ConditionIdEmpathicShroud`, then `ConditionIdWarcry` through
`ConditionIdEnchantWithdrawal`) so a producer or a reader never spells a bare
number. Warcry and rally used to apply a combat condition AND a display-only
mirror record side by side; there is one record now, and the
`condition-mirror` flag and both skip loops are deleted.

### The record's strength

Every instance carries `Magnitude float64` (`yaml:"magnitude,omitempty"`), the
strength the applier computed for that one holder. What the strength DOES is
declared by the spec, in the closed `effects:` map documented under "Effects
Vocabulary (`effects.go`)" below: ten keys (`AllEffectKinds`), each value either a literal
number or the word `magnitude`, meaning "read the holding instance".

Combat reads timed state through `Conditions.Effect(kind)` and
`Conditions.HasEffect(kind)` and through nothing else. The live readers:

| Kind | Read by |
|---|---|
| `attacks_cap` | `calcSwingCount`, `internal/combat/combat_helpers.go` |
| `damage_mult` | the damage mean and crit base, same file |
| `defense_mult` | the defense score, same file |
| `mitigation_flat` | `Character.GetPhysicalMitigation`, `internal/characters/combat.go`; `internal/combat/ai.go` and `internal/behaviortree/action_cast_best_in_category.go` ask `HasEffect` before casting a ward |
| `dodge_mult` | `Character.GetDefenseScoreFor`, `internal/characters/combat.go` (`GetDefenseScore` is a one-line wrapper over it; no shipped producer, and the seam is kept because `Effect` is a product with identity 1.0) |
| `regen_mult` | `internal/hooks/NewRound_AutoHeal.go`, four branches across the player and mob paths |
| `pool_max_pct` | the pool loop in `internal/characters/validate.go`; the pool NAME rides on the instance's `Source` |

A spec that sets `tick_from_magnitude` snapshots the magnitude into the
instance's `TickAmount`, so a dot deals exactly the signed integer its producer
passed, floored to plus or minus one. The writer door is
`AddConditionMagnitude`, documented under "Magnitude Records
(`AddConditionMagnitude`)" below:
`Conditions.AddConditionMagnitude(id, triggers, magnitude)`, wrapped by
`Character.AddConditionMagnitude(id, triggers, magnitude, source)`
(synchronous, sets `Source`, then `Validate`) and
`UserRecord.AddConditionMagnitude(...)` (queues `events.Condition` with
`Magnitude` and `Triggers`).

**Scaling stays in the appliers.** The record carries base values and a
vocabulary; the formula that produces a magnitude and a duration still lives in
the spell, potion, action or skill that applies it, unchanged by this slice.
Moving those formulas is the spell scaling arc's job, and it now has one seam
to plug into.

### Flags added

- `bleeding`: marks a bleed record. The death cause reads it the way it reads
  `poison`; see `tickCauseFor` in `internal/hooks/tick_cause.go`.
- `quiet`: the record is listed, but sends no start line and no end line. It
  exists for a record reapplied every round it persists (117 and 118), where a
  line would repeat every round. The notice guard exempts it at both ends; see
  "The player-side notice (slice C, `notice.go`)" below.
- `stacking` (slice 1b): every application is its own `Stack` with its own
  rounds, held inside the one record. `Trigger` lands the SUM of the live
  stacks as the record's `TickAmount` once a round, drops spent stacks, and
  sets `TriggersLeft` to the longest remaining one; the record ends with its
  last stack. Requires `tick_from_magnitude` and a one-round `triggerrate`
  (`Validate` refuses anything else). Only 122 Bleeding carries it. See
  "Stacking records (`stacks.go`)" below.
- `adjustable` (lighting plan 5a): a light source that trims itself to its
  bearer's eyes on each room entry (`internal/rooms.(*Room).TrimLightFor`).
  `validateEffects` refuses it without `light_strength` or (lighting plan 5d)
  `darkness_strength`.
- `cancellable` (lighting plan 5a): the holder may end the record early with
  `cancel <spell>` (`internal/usercommands/cancel.go`). Opt-in; the Cat's Eye
  Draught is ruled uncancellable.
- `lightsource` (`EmitsLight`) was DELETED in plan 5a. A light is now any
  record whose spec declares `light_strength`; ask `ConditionSpec.IsLightSource()`.

### Light sources (`light.go`, lighting plan 5a)

`effects: {light_strength: N}` makes a record a light: a literal for an item,
`magnitude` for a spell cast at a scaled strength. It is **per-record, never
aggregated**: `Conditions.Effect(EffectLightStrength)` returns 0, and each held
light is its own term in the room's combine, found through
`(*Conditions).LightSources() []*Condition` (held, unexpired, in held order).
`validateEffects` refuses a literal `light_strength` of 0 or less, `adjustable`
without `light_strength`, and a `stacking` record that is also a light.

**Darkness sources (lighting plan 5d, ruling D1).** `effects:
{darkness_strength: N}` (`EffectDarknessStrength`, in `AllEffectKinds` and
`ScaledKinds`) makes a record a darkness: light taken away from its room, a
literal for an item (132 Umbral Dark), `magnitude` for a spell (131 Chrysalis
Pall). It is a light record with darkening polarity: it shares `LightTrim`,
`LightOutput` and `ResetLight`, and `LightMax` / `LightNow` read whichever of
the two kinds the spec declares. It is NOT a light: `IsLightSource()` stays
light-only, so `LightSources`, `EmitsLight` and `hood` exclude it by
construction. Ask `ConditionSpec.IsDarknessSource()`, walk
`(*Conditions).DarknessSources()`, or `(*Conditions).LightAndDarknessSources()`
for both kinds in one held order (the trim's walk).
`Effect(EffectDarknessStrength)` returns 0. `validateEffects` also refuses a literal `darkness_strength` of 0 or less, a spec declaring both
kinds, and a `stacking` darkness; `AddConditionMagnitude` resets either kind.

Per-record state lives on `Condition`: `LightTrim` (`LightFull`,
`LightTrimmed`, `LightOff`; an explicit state so a save never encodes -Inf),
`LightOutput`, `Hooded`.
- `LightMax(spec) float64`: full strength (the magnitude or the literal).
- `LightNow(spec) (float64, bool)`: the term it adds right now; false when
  expired, hooded, off or strengthless. An unrecognised saved state fails open
  to full strength.
- `SetLightOutput(out)`: records a `lightscale.Trim` result; any non-finite
  value lands on `LightOff`.
- `ResetLight()`: full strength, hood open.

🪤 **Only `AddConditionMagnitude` resets these automatically** (a fresh cast).
The worn-item refresh re-adds a held record WITHOUT touching them, so a trim
survives an unrelated equipment change; an equip path that wants a fresh light
must call `ResetLight` itself (`Character.Wear` does).

🪤 **A magnitude light added without a magnitude sheds nothing.** `LightMax`
reads `Magnitude`, so a `light_strength: magnitude` record added through plain
`AddCondition` (an admin `setcondition`, for one) reads 0 and is dark; cast the
spell instead. A glow record saved before plan 5a was applied that way and
carries `Magnitude` 0, so on the first login after deploy it lists as
Illumination but sheds no light until it expires (at most 20 real minutes) or
is recast; no migration is needed.

### Cadence

121 Poisoned and 122 Bleeding tick every round (`triggerrate: 1 round`). Slice
1 shipped them at `3 rounds` to reproduce the AutoHeal hook they replaced,
which was gated on `RoundNumber%3`; the owner ruled on 2026-09-14 that both
tick every round. A producer passes a duration in rounds, which for a
one-round record IS the trigger count. Ticking three times as often would have
tripled the spell dot's total, so the owner halved the two spell dot
`effect_magnitude` values in the same slice (`blood-boil` 80 to 40,
`neural-toxin` 60 to 30), which lands about 1.5 times the old total.

### Facts worth knowing

- **These records persist.** The deleted combat condition slice was tagged
  `yaml:"-"`, so every combat condition vanished on logout or restart. A
  record is saved with the rest of `Character.Conditions`, so a poison or a
  ward now survives a short absence.
- **The prone recovery cap bites for players and mobs alike.** 118 Recovering
  lives exactly one tick, so both round ticks add it AFTER their condition
  tick (`MobRoundTick` always did; `UserRoundTick` since slice 1b), and it is
  live when `DoCombat` reads `attacks_cap`. `UserRoundTick` skips the stand
  attempt for a player at zero health or with a death queued, so a dying
  player does not scramble to their feet.
- **A killing tick still names its cause.** `Conditions.Trigger` decrements
  `TriggersLeft` before returning, so a record's LAST tick arrives already
  `Expired`, and `PruneConditions` can remove it before the queued death event
  is handled. Both round ticks therefore stamp `Character.LastTickCause` and
  `LastTickCauseRound` where the harm lands, and `deathCauseFor` reads the held
  record by id first and falls back to the stamp within one round.
- **A record's final trigger used to be dropped on the player side.**
  `UserRoundTick` gated the whole tick body on `!condition.Expired()`, so the
  one and only tick of a one-trigger record never landed. The mob tick never
  had the defect. Fixed in slice 1, which means every player-held tick record
  now lands one more tick than it did before. The owner ruled this extra
  final tick intended behaviour, not a side effect to correct (2026-09-14).
- **`Discard` drops a record with no end line.** `RemoveCondition` only marks
  a record expired, and the prune pass then narrates its end. `Discard`
  deletes it outright and rebuilds the lookups, so nothing is told. It exists
  for one hide handing over to another (#444): the loser's record (9 or 31)
  goes without telling the room of a reveal that did not happen.

## Architecture

The conditions system is built around several key components:

### Core Components

**Condition Specifications (ConditionSpec):**
- Immutable blueprint definitions for all condition types
- YAML-based storage with automatic loading and validation
- Time-based trigger rate calculations with game time integration
- Stat modification definitions and behavioral flags
- Config-driven tick behaviors with stat scaling

**Condition Instances (Condition):**
- Runtime instances with unique state tracking
- Round-based trigger counters and expiration management
- Source tracking for condition origin identification
- Permanent condition support for equipment and racial effects
- Start event queuing for delayed activation

**Conditions Collection (Conditions):**
- Collection management with flag indexing
- Fast lookup maps for condition IDs and flags
- Automatic validation and rebuilding of internal indexes
- Batch operations for triggering and pruning

## Key Features

### 1. **Comprehensive Flag System**
- Behavioral modification flags (combat, movement, fleeing restrictions)
- Death prevention and revival mechanics
- Equipment interaction flags (permanent gear, curse removal)
- Status effect flags (poison, accuracy, stealth, vision enhancement)
- Environmental interaction flags (water cancellation). Light emission is not a
  flag: since lighting plan 5a a light is a record declaring the
  `light_strength` effect (see "Light sources (`light.go`)" below)

### 2. **Advanced Timing System**
- Round-based trigger intervals with game time integration
- Flexible trigger rates using time string parsing
- Unlimited duration support for permanent effects
- Precise expiration tracking and automatic cleanup
- Trigger counting with configurable limits

### 3. **Stat Modification Integration**
- Dynamic stat bonuses and penalties
- Cumulative effects from multiple conditions
- Integration with character stat system
- Racial and equipment stat modifications
- Combat effectiveness modifiers

## Condition Structure

### Condition Specification Structure
```go
type ConditionSpec struct {
    ConditionId   int               `yaml:"conditionid"` // Unique identifier
    Name          string            // Display name
    Description   string            // Description text
    Secret        bool              // Hidden from player view
    TriggerNow    bool              // Immediate trigger on application
    TriggerRate   string            // Time-based trigger frequency
    RoundInterval int               // Calculated round interval
    TriggerCount  int               // Total trigger limit
    StatMods      statmods.StatMods // Stat modifications
    Flags         []Flag            // Behavioral flags
}
```

### Condition Instance Structure
```go
type Condition struct {
    ConditionId    int     `yaml:"conditionid"` // Reference to ConditionSpec
    Source         string  // Origin identifier (spell, item, area)
    OnStartWaiting bool    // Pending start event
    Permanent      bool    `yaml:"permanent,omitempty"` // Permanent condition flag
    RoundCounter   int     // Elapsed rounds
    TriggersLeft   int     // Remaining triggers
    TickAmount     int     // Signed per-trigger amount snapshot
    Magnitude      float64 // Per-instance strength the applier set
    Stacks         []Stack // A stacking record's applications; see stacks.go
    LightTrim      LightTrim // plan 5a: full, trimmed or off; see light.go
    LightOutput    float64   // plan 5a: the trimmed term when LightTrim is LightTrimmed
    Hooded         bool      // plan 5a: a hooded light sheds nothing
}
```

### Conditions Collection Structure
```go
type Conditions struct {
    List           []*Condition   // Active condition instances
    conditionFlags map[Flag][]int // Flag to condition index mapping
    conditionIds   map[int]int    // ConditionId to index mapping
}
```

## Flag System

### Behavioral Flags
```go
const (
    // Combat and Movement Restrictions
    NoCombat       Flag = "no-combat"        // Prevents combat initiation
    NoMovement     Flag = "no-go"            // Prevents movement
    NoFlee         Flag = "no-flee"          // Prevents fleeing combat
    
    // Cancellation Conditions
    CancelIfCombat Flag = "cancel-on-combat" // Removes condition when combat starts
    CancelOnAction Flag = "cancel-on-action" // Removes condition on any action
    CancelOnWater  Flag = "cancel-on-water"  // Removes condition in water
    
    // Death and Revival
    ReviveOnDeath  Flag = "revive-on-death"  // Prevents death once
    
    // Equipment Interaction
    PermaGear      Flag = "perma-gear"       // Equipment cannot be removed
    RemoveCurse    Flag = "remove-curse"     // Allows cursed item removal
    
    // Status Effects
    Poison         Flag = "poison"           // Harmful poison effect
    Drunk          Flag = "drunk"            // Intoxication effects
    Hidden         Flag = "hidden"           // Stealth/invisibility
    Accuracy       Flag = "accuracy"         // Enhanced hit chance
    Blink          Flag = "blink"            // Dodge enhancement
    
    // Sensory Enhancement
    SuperHearing   Flag = "superhearing"     // Enhanced hearing
    NightVision    Flag = "nightvision"      // See in darkness
    SeeHidden      Flag = "see-hidden"       // Detect hidden entities
    SeeNouns       Flag = "see-nouns"        // Enhanced object identification
    
    // Environmental Status
    Warmed         Flag = "warmed"           // Temperature regulation
    Hydrated       Flag = "hydrated"         // Hydration status
    Thirsty        Flag = "thirsty"          // Dehydration status
)
```

### Progression Flags and `progress_mult`

Two flags are more than booleans the engine merely tests. `skill-progress` and
`mutation-rate` quicken skill progression and mutation progress while the
condition is held, and how much they quicken it is authored per condition
through the optional `progress_mult:` YAML key behind
`ConditionSpec.ProgressMult`. Read it with `Conditions.ProgressMult(flag)`
rather than by testing the flag: it returns 1.0 when no held condition carries
the flag, so a caller can multiply unconditionally, and a held flagged
condition that declares no `progress_mult` is worth 2.0, the default the two
consuming call sites (`internal/characters/progression.go` and
`internal/hooks/NewRound_UserRoundTick.go`) used to hardcode. Held flagged
conditions do not stack; the strongest value wins. So Savant's Infusion
(condition 72) at 2.5 beats Essence of Growth (condition 71) at the default,
and Chrysalis Catalyst (condition 74) at 3.0 beats Mutagen Brew (condition 73).

### Flags are validated at load (slice E, 2026-09-12)

`AllFlags` lists every declared `Flag`. `ConditionSpec.ValidateFlags()` reports
the first flag a spec carries that is not in the list. `ValidateLoadedFlags()`
walks the loaded registry in id order and panics on the first offender, naming
the condition id, name and flag; `LoadDataFiles` calls it after the registry is
live. It is a named function precisely so a test can call it: no test loads
world YAML, so an inlined check had nothing red to prove it works. The root
guard `condition_flag_guard_test.go` walks the dogmud condition files and fails
the build on the same condition, and `TestAllFlagsNamesEveryDeclaredConstant`
parses the constants out of this package so the list cannot fall behind. Flags
are compared exactly, nothing is normalised: the Cat's Eye Draught shipped with
`night-vision` for `nightvision` and did nothing for weeks.
The call from `LoadDataFiles` itself is pinned by nothing (no test loads world
YAML, and a clean boot cannot detect a missing negative check); the function is
pinned by a direct panic test, and the root guard is the gate that blocks a
merge. The boot panic is defence in depth for a file edited by hand on prod.


`poison-immunity` (`PoisonImmunity`, Stone Stomach): while held,
`AddCondition`, `AddConditionScaled` and therefore `AddConditionMagnitude`
refuse a spec carrying `poison`, which since the conditions unification
includes the 121 Poisoned record. Refusal is silent. There used to be a SECOND
immunity check in the combat condition path; it is deleted along with the
enum, so there is one refusal in one place. The three toxins (39 Venom, 40
Spore Toxin, 78 Toxic Cloud) and 75 Nausea carry `poison` since the same slice;
before it NO record did, so `CancelConditionsWithFlag(Poison)` in Purge
Affliction and Cleansing Wave cancelled nothing and the `poisoned` adjective
never showed.

A refusal is a refusal all the way out: `Character.AddCondition` returns an
error, `ApplyConditions` (`internal/hooks/Condition_ApplyConditions.go`)
returns on it before the start notice, `start_remove_conditions`,
`TrackConditionStarted` or `ConditionsTriggered`, and
`Character.AddConditionMagnitude` returns an `error` the two spell dot sites
test before narrating. `HasFlag` guards a nil spec, since every add now asks it
and a save can hold a dead condition id.

`ApplyConditions` also refuses, the same way and before any add, an event whose
`LifeEpoch` no longer matches its holder's `Character.LifeEpoch`: the holder
died after it was queued, so the condition was aimed at a life that has ended.
See the Alive to Dead cascade in `internal/hooks/context.md`.

### Flag Usage Patterns

The sketch below is abridged and predates two fixes in the live body: `All`
matches a flagless condition, and the spec lookup is nil-guarded before
`.Flags`. Read `conditions.go` for the real thing.

```go
// Check for specific behavioral flags
func (bs *Conditions) HasFlag(action Flag, expire bool) bool {
    if action != All {
        if _, ok := bs.conditionFlags[action]; !ok {
            return false
        }
    }
    
    found := false
    for index, b := range bs.List {
        if b.Expired() {
            continue
        }
        
        bSpec := GetConditionSpec(b.ConditionId)
        for _, flag := range bSpec.Flags {
            if flag == action || action == All {
                found = true
                
                // Optionally expire the condition when checked
                if expire {
                    if b.ConditionId == 0 { // Special condition 0 handling
                        bs.List = append(bs.List[:index], bs.List[index+1:]...)
                    } else {
                        b.expire()
                        bs.List[index] = b
                    }
                    break
                }
                
                return found
            }
        }
    }
    
    return found
}

// Get all condition IDs with specific flag
func (bs *Conditions) GetConditionIdsWithFlag(action Flag) []int {
    conditionIds := []int{}
    for _, idx := range bs.conditionFlags[action] {
        conditionIds = append(conditionIds, bs.List[idx].ConditionId)
    }
    return conditionIds
}
```

## Timing and Trigger System

### Round-Based Triggers
```go
// Trigger conditions based on round intervals (abridged; the live body also
// ticks stacking records through tickStacks)
func (bs *Conditions) Trigger(conditionId ...int) (triggeredConditions []*Condition) {
    for idx, b := range bs.List {
        // Handle specific condition triggering if requested
        if len(conditionId) > 0 {
            found := false
            for _, id := range conditionId {
                if b.ConditionId == id {
                    found = true
                    break
                }
            }
            if !found {
                continue
            }
        }
        
        if conditionInfo := GetConditionSpec(b.ConditionId); conditionInfo != nil {
            if b.TriggersLeft > 0 {
                b.RoundCounter++
                
                // Check if it's time to trigger
                if b.RoundCounter%conditionInfo.RoundInterval == 0 {
                    triggeredConditions = append(triggeredConditions, b)
                    
                    // Decrement triggers unless unlimited
                    if b.TriggersLeft != TriggersLeftUnlimited {
                        b.TriggersLeft--
                    } else {
                        // Reset counter to prevent overflow
                        b.RoundCounter = 0
                    }
                }
                
                bs.List[idx] = b
            }
        }
    }
    
    return triggeredConditions
}
```

### Duration Calculations

`GetDurations(condition *Condition, spec *ConditionSpec) (roundsLeft int,
totalRounds int)` reads the INSTANCE for `roundsLeft` (its `TriggersLeft`
times the spec's `RoundInterval`, less how far into the current interval it
is), so a record added with an exact trigger count reports its own remaining
rounds; `totalRounds` keeps the spec figure so a bar has a stable scale. A pure
flag record (`RoundInterval < 1`) reports `0, 0`. Read the doc comment in
`conditions.go` for the edge cases.

```go
// Check if condition has expired
func (b *Condition) Expired() bool {
    return b.TriggersLeft <= TriggersLeftExpired
}
```

### Stacking records (`stacks.go`)

A spec with the `stacking` flag keeps `Condition.Stacks []Stack`, each
`Stack{RoundsLeft, Amount}`. `Conditions.AddConditionMagnitude` routes such a
spec to `addStack`: `triggers` becomes the new stack's rounds (0 means the
spec's `triggercount`), the magnitude becomes its signed amount through
`tickAmountFor` (a non-zero magnitude never snapshots to zero; a magnitude of
exactly 0 is refused, since a zero stack would lengthen the record and print a
bleed line for no harm), and `syncStacks` derives the record-level fields
every other reader uses: `TriggersLeft` is the longest stack, `TickAmount` and
`Magnitude` the sum. So `Expired`, `GetDurations`, the prune pass, poison
immunity, `HasCondition`, the death cause and both condition lists work
unchanged.

**Only `AddConditionMagnitude` may add a stacking record.** `AddCondition`,
`AddConditionScaled` and `RefreshCondition` all return `false` for a stacking
spec, because none of them can carry a stack's rounds and amount: they would
create a live record with no stacks, or top one up to the spec's single
`TriggerCount`. `addStack` creates the record through the unexported
`addConditionScaled`, which does not refuse. The admin `setcondition` command
refuses a stacking condition id with a message for the same
reason.

`Conditions.Trigger` calls `tickStacks` for such a record: `TickAmount` becomes
this round's landed sum, every stack loses a round, spent stacks drop, and
`TriggersLeft` becomes the longest remaining stack. Both round-tick paths read
`TickAmount` after `Trigger` returns, so one tick is one harm, one wake, one
`cancel-on-damage` pass, one death-cause stamp and one flavour line however
many stacks are live. A stacking record with no stacks is expired without
being returned, so a zero amount never reaches the `tick_percent` fallback.
Between ticks, read `Stacks` or `Magnitude` for the whole bleed, never
`TickAmount` (it holds the last round's landed sum).

**Every path that expires a held record goes through `Condition.expire()`**,
which sets `TriggersLeft` to `TriggersLeftExpired` and clears `Stacks` in one
step: `RemoveCondition`, the expire branch of `HasFlag`, and `tickStacks` when
a record has no stacks. `addStack` revives an expired, unpruned record through
`addConditionScaled`, so one that kept its stacks would come back with them
live; `expire()` is the primary guard, and `addStack` clearing the stacks of
an expired record before adding is the second, so a cancel followed by a new
hit starts fresh.

`Condition.Source` is the LAST applier's source:
`Character.AddConditionMagnitude` overwrites it on every call, so for a
stacking record it is not per stack. Nothing reads a bleed's `Source` today.

Every bleed producer runs inside combat, after that round's tick, so a new
stack first ticks on the next round. `conditions.DisplayName` names a held
record for the condition lists and appends the live count above one stack
("Bleeding (3)"). Bleed stack numbers are the fifteen `<Move>Bleed*` balance
knobs; see `internal/actions/bleed.go` and the Bleed stacks block in
`config.yaml`.

### Time String Processing

`ConditionSpec.Validate()` returns an error for an unknown text token (M4a:
the loader panics, so a typo cannot render raw to a player), converts `TriggerRate` to
`RoundInterval` through the game time calculator, forces condition 0's
`TriggerCount` to the configured logout rounds, validates effects and
narration (flags are checked separately by `ValidateLoadedFlags`), and returns an error for a spec with no usable trigger count or
interval. Read `conditionspec.go` for the body.

## Stat Modification System

### Individual Condition Stat Modifications
```go
// Get stat modification from single condition
func (b *Condition) StatMod(statName string) int {
    if b.Expired() {
        return 0
    }
    if conditionInfo := GetConditionSpec(b.ConditionId); conditionInfo != nil {
        return conditionInfo.StatMods.Get(statName)
    }
    return 0
}
```

### Cumulative Stat Modifications
```go
// Calculate total stat modification from all active conditions
func (bs *Conditions) StatMod(statName string) int {
    conditionAmt := 0
    for _, b := range bs.List {
        conditionAmt += b.StatMod(statName)
    }
    return conditionAmt
}
```

### Condition Value Calculation
```go
// Calculate relative power/value of a condition for balance
func (b *ConditionSpec) GetValue() int {
    val := 0
    
    // Sum absolute values of all stat modifications
    for _, v := range b.StatMods {
        val += int(math.Abs(float64(v)))
    }
    
    // Add value for frequency (more frequent = more valuable)
    freqVal := max(5-b.RoundInterval, 0)
    val += freqVal
    
    // Add value for flags (5 points per flag)
    val += len(b.Flags) * 5
    
    // Multiply by trigger count for total effect
    if b.TriggerCount > 0 {
        val *= b.TriggerCount
    }
    
    return val
}
```

## Effects Vocabulary (`effects.go`)

`EffectKind` is a closed set of mechanical effects a `ConditionSpec` may
declare under its `effects:` map, keyed by kind and pointing at an
`EffectValue` (either a literal number or the YAML word `"magnitude"`, meaning
"read the holding instance's own `Magnitude`"). The set is closed on purpose:
a new kind is a code change with a reader, never a data change.
`ConditionSpec.validateEffects` refuses an unknown key, and refuses
`TickFromMagnitude` set without a `TickPool`, or set alongside a non-zero
`TickPercent`.

```go
const (
    EffectDamageMult     EffectKind = "damage_mult"     // physical damage multiplier (warcry)
    EffectDefenseMult    EffectKind = "defense_mult"    // defense score multiplier (rally, grapple exposure)
    EffectDodgeMult      EffectKind = "dodge_mult"      // dodge score multiplier (no producer today; kept for parity with the reader)
    EffectRegenMult      EffectKind = "regen_mult"      // multiplier on base health regen (heal spells, corpse feeding)
    EffectMitigationFlat EffectKind = "mitigation_flat" // flat physical mitigation points (wards)
    EffectPoolMaxPct     EffectKind = "pool_max_pct"    // fraction taken off a pool maximum; the pool rides on Condition.Source
    EffectAttacksCap     EffectKind = "attacks_cap"     // upper bound on swings per round
    EffectNightVisionStrength EffectKind = "nightvision_strength" // graded lighting plan 2: how far DOWN the light scale the observer's usable band shifts
    EffectInfraReach          EffectKind = "infra_reach"          // graded lighting plan 2, reworked 5c: how far into the dark heat-sensing still reads shapes (shapes at any light down to minus this many points)
    EffectLightStrength       EffectKind = "light_strength"       // lighting plan 5a: a light's full strength; per-record, Effect() returns 0 for it
)
```

`Conditions.Effect(kind EffectKind) float64` is the ONE door combat (and,
since graded lighting plan 2, `internal/characters.Character.NightVisionStrength`)
reads timed state through. It folds every held, unexpired record's
contribution for that kind: a multiplier kind (`damage_mult`, `defense_mult`,
`dodge_mult`, `regen_mult`) multiplies across records with identity `1.0`;
`attacks_cap` takes the minimum non-zero value (`0` meaning no cap); a MAX
kind (`nightvision_strength`, `infra_reach`) takes the strongest held value
rather than summing, so two sources of the same vision effect cannot stack
into a window wider than the better one grants; everything else
(`mitigation_flat`, `pool_max_pct`) sums with identity `0`. It never calls
`HasFlag` with `expire=true`, so reading it has no side effect.
`Conditions.HasEffect(kind EffectKind) bool` reports whether any held,
unexpired record declares the kind at all, without computing a value.

**Since lighting plan 5c**, `EffectInfraReach` is `isMax` for `Effect()`'s own
callers, but `Character.InfraReach` does not call `Effect()` for this kind
(owner ruling: reach sources COMBINE, unlike nightvision strength, which
still does take the best). It instead calls `Conditions.EffectValues(kind
EffectKind) []float64` (new, `effects.go`): every held, unexpired record's
value for one kind, magnitude-aware exactly as `Effect` reads it, in list
order, so a reader can combine them some other way than `Effect`'s own rule.
See `internal/characters/context.md` for how `InfraReach` combines the
result through `lightscale.Combine` (a sum of linear brightness since
lighting plan 6).

Which predicate a kind matches is closed and mutually exclusive:
`EffectKind.isMultiplier()`, `.isCap()` and `.isMax()` are checked in that
order inside `Effect`'s switch, so a kind classified under more than one
would silently lose whichever mode its later predicate would have produced.
`TestEveryEffectKindIsClassifiedExactlyOnce` (`effects_test.go`) guards this
by counting matches rather than checking pairs by hand, so it also covers any
kind added after the test was written.

### ScaledKinds and ConditionSpec.ScaledKind (lighting plan 5c)

`ScaledKinds` (`effects.go`) is the closed list of effect kinds a spell or
potion may scale from its holding instance's own `Magnitude`:
`EffectLightStrength`, `EffectNightVisionStrength`, `EffectInfraReach`. A
condition record carries exactly one `Magnitude`, so `ConditionSpec.ScaledKind()
(EffectKind, bool)` reports at most one of them, whichever declares
`UsesMagnitude` in its `EffectValue`; `validateEffects` refuses a spec that
declares `UsesMagnitude` on more than one `ScaledKinds` entry at load time,
with `ok == false` when none does. Both `internal/hooks.magnitudeSpellApplication`
(the one spell-casting hook that replaced the light-only
`lightSpellApplication`) and `internal/items.PotionMagnitudeApplication`
call it to find which trio of scaling knobs applies to a given condition.

## Condition Management Operations

### Adding Conditions
```go
// Add new condition or refresh existing condition (abridged)
func (bs *Conditions) AddCondition(conditionId int, isPermanent bool) bool {
    if conditionInfo := GetConditionSpec(conditionId); conditionInfo != nil {
        if conditionInfo.IsStacking() {
            return false // only AddConditionMagnitude adds a stacking record
        }
        if slices.Contains(conditionInfo.Flags, Poison) && bs.HasFlag(PoisonImmunity, false) {
            return false // poison immunity, silent
        }

        newCondition := Condition{
            ConditionId:  conditionInfo.ConditionId,
            RoundCounter: 0,
            Permanent:    false,
            TriggersLeft: conditionInfo.TriggerCount,
        }
        
        // Handle permanent conditions (from equipment/race)
        if isPermanent {
            newCondition.TriggersLeft = TriggersLeftUnlimited
            newCondition.Permanent = true
        }
        
        // Refresh an existing record
        if idx, ok := bs.conditionIds[conditionId]; ok {
            bs.List[idx].TriggersLeft = newCondition.TriggersLeft
            bs.List[idx].RoundCounter = 0
            bs.List[idx].Permanent = newCondition.Permanent
            return true
        }
        
        // Add a new record and index its flags
        bs.List = append(bs.List, &newCondition)
        listIndex := len(bs.List) - 1
        bs.conditionIds[conditionId] = listIndex
        for _, flag := range conditionInfo.Flags {
            bs.conditionFlags[flag] = append(bs.conditionFlags[flag], listIndex)
        }
        
        return true
    }
    
    return false
}
```

### Magnitude Records (`AddConditionMagnitude`)

`Conditions.AddConditionMagnitude(conditionId int, triggers int, magnitude float64) bool`
is the writer door for every record that used to be a hand-rolled combat
condition (Minor Shield, Regenerating, Poisoned, Bleeding). It refreshes or
adds the condition via `addConditionScaled(conditionId, 1.0)`, then, if
`triggers > 0`, overwrites `TriggersLeft` with the exact count, **a trigger
count, not a duration in rounds**; every record that goes through this door
today (79, 80, 117 to 123) ticks once a round, so the two coincide. A
`stacking` spec appends a stack instead of refreshing; see "Stacking records"
above. `triggers` of `0` leaves the spec's own `TriggerCount` in place. It also
stamps `Magnitude`, and for a spec with `TickFromMagnitude` set, snapshots
`TickAmount` from the magnitude's sign (floored to plus or minus 1 rather than
0, since a zero tick would never recover). For a light source
(`IsLightSource`) it calls `ResetLight`, since a fresh magnitude is a fresh
cast. Returns `false` on refusal (e.g.
poison immunity via `AddConditionScaled`) or an unknown condition id, exactly
like `AddConditionScaled`.

Three doors wrap it, each documenting the same "triggers, not rounds"
contract:
- `Character.AddConditionMagnitude(conditionId, triggers, magnitude, source)`
  applies synchronously and sets `Source` on the held record, then
  revalidates. Every producer of a former combat condition (the ward and heal
  and dot spells, the shouts, the bleed actions, the grapple and stand paths,
  disenchant) calls this one, because those effects must land within the same
  round tick and narrate the moment themselves.
- `UserRecord.AddConditionMagnitude(conditionId, triggers, magnitude, source)`
  queues an `events.Condition{Triggers: triggers, Magnitude: magnitude}`
  instead, for a spell or item that wants the holder's start notice through
  the normal `ApplyConditions` door.
- `events.Condition.Triggers` (with `.Magnitude`) is the field `UserRecord`
  populates; `ApplyConditions` routes to `Character.AddConditionMagnitude`
  when either is non-zero, and to the `DurationMult` path otherwise.

### Removing Conditions
```go
// Remove specific condition by ID
func (bs *Conditions) RemoveCondition(conditionId int) bool {
    if index, ok := bs.conditionIds[conditionId]; ok {
        bs.List[index].expire()
        return true
    }
    return false
}

// Mark condition as started (no longer waiting for start event)
func (bs *Conditions) Started(conditionId int) {
    if idx, ok := bs.conditionIds[conditionId]; ok {
        bs.List[idx].OnStartWaiting = false
    }
}
```

### Pruning Expired Conditions
```go
// Remove all expired conditions and rebuild indexes (abridged)
func (bs *Conditions) Prune() (prunedConditions []*Condition) {
    if len(bs.List) == 0 {
        return prunedConditions
    }
    
    didPrune := false
    
    // Iterate backwards to safely remove items
    for i := len(bs.List) - 1; i >= 0; i-- {
        b := bs.List[i]
        if GetConditionSpec(b.ConditionId) == nil || b.Expired() {
            prunedConditions = append(prunedConditions, b)
            bs.List = append(bs.List[:i], bs.List[i+1:]...)
            didPrune = true
        }
    }
    
    // Rebuild lookup indexes if any conditions were pruned
    if didPrune {
        bs.Validate(true)
    }
    
    return prunedConditions
}
```

## Collection Validation and Indexing

### Index Management

`Conditions.Validate(forceRebuild ...bool)` lazily creates the two private
indexes (`conditionFlags map[Flag][]int`, `conditionIds map[int]int`) and
rebuilds both when the list length disagrees with the id index or a rebuild is
forced. A held record whose spec no longer exists is logged and skipped for
flag indexing. Read `conditions.go` for the body.

### Query Operations
```go
// Check if specific condition exists (map membership: true for an
// expired-but-unpruned record too; use TriggersLeft(id) > 0 for "live")
func (bs *Conditions) HasCondition(conditionId int) bool

// Get remaining triggers for condition
func (bs *Conditions) TriggersLeft(conditionId int) int

// Get all active conditions (optionally filtered by ID)
func (bs *Conditions) GetConditions(conditionId ...int) []*Condition
```

## Display and Visibility
```go
// Listed reports whether a held record appears in the player's condition
// lists: the in-game `conditions` command and the Char.Conditions GMCP
// payload. Both call this, so the two can never disagree.
func (b *ConditionSpec) Listed() bool {
	return !b.Secret && !slices.Contains(b.Flags, Hidden)
}

// DisplayName: the spec name, plus the live stack count above one ("Bleeding (3)").
func DisplayName(b *Condition, spec *ConditionSpec) string

// Get condition display name
func (bs *Condition) Name() string
```

### The player-side notice (slice C, `notice.go`)

- `StartUserNotice() string` / `EndUserNotice() string`: the ONE door for the
  line the holder reads when a condition lands or ends. Authored
  `start_actee` / `end_actee` first; otherwise the generic
  "<Name> takes effect." / "<Name> has expired."; an empty string for a
  `secret` condition or one with no name. `ApplyConditions` and the player
  prune pass in `NewTurn_PruneConditions.go` read these instead of the raw
  fields. Room text is untouched and stays authored-only.
- `SilentNoticeConditions() []string`: every loaded non-secret condition
  relying on the generic line, as `"<id> <name> (start, end)"`.
  `WarnSilentNotices()` logs one warning per entry at boot (wired in `main.go`
  after the species guard).
- Three flags declare a deliberate silence, and `StartUserNotice` /
  `EndUserNotice` are where each one returns the empty string:
  - `silent-start` silences the START only (the applying command narrates it;
    Warcry, Rally, Throttled, Sleeping and Bloom Detox are applied through
    `Character.AddCondition`, which never queues the condition event). Such a
    condition still owes the holder a line, just not from the hook:
    `actions.Sleep` sends condition 15's line through `AuthoredStartLine`,
    which is what the flag means by "the applier narrates".
  - `hidden` silences the END only (a hider must not learn when the cover
    lapsed; room text still goes out).
  - `quiet` silences BOTH, for a record reapplied every round it persists
    (117 and 118), where either line would repeat every round.

  A `secret` condition is silent at both ends too, but that is a spec field
  rather than a flag, and it also hides the record from the `conditions` list.
- **A player condition must be applied through
  `users.UserRecord.AddCondition` or `UserRecord.AddConditionScaled`, the
  event path, or it lands in silence: `Character.AddCondition` /
  `Character.AddConditionScaled` apply in place and queue nothing, so
  `ApplyConditions` never runs and no notice reaches the holder.** The
  multiplier rides on `events.Condition.DurationMult`, so a scaled application
  takes the same door. The root guard `condition_apply_path_guard_test.go`
  walks all of `internal/` and `modules/` except the primitive packages
  (`internal/conditions`, `internal/characters`, which define the primitive
  being called) and fails the build on any direct character-level add outside
  them that is not in its allowlist with a reason; the three prone-recovery
  `AddConditionMagnitude` producers inside `internal/characters/skills.go` are
  exempted by that package-level carve-out, not individually allowlisted.
- **Every non-secret condition in the dogmud world must carry authored
  `start_actee` (unless `silent-start` or `quiet`) AND `end_actee`
  (unless `hidden` or `quiet`), and a secret condition must carry no player
  text.** The root guard `condition_notice_guard_test.go` fails the build
  otherwise; the generic line is a runtime net, never the shipped experience.
  `secret: true` also hides the condition from `conditions`.
  `SilentNoticeConditions` exempts the same three flags, so the boot warning
  and the guard agree.

### The narration door (M3 item 5b, `narration.go`)

- `Phase` (`PhaseStart`, `PhaseTrigger`, `PhaseEnd`) selects the moment.
- `Narration(p Phase) narration.Variants`: the holder's line is the **Actee**
  (the condition happens to them; owner ruling 2026-09-12) and the room's line
  the Observer. Actor is empty and reserved for the caster, which M6 authors
  once `events.Condition` carries one. Start and End go through
  `StartUserNotice` / `EndUserNotice`, so the notice rules stay in their one
  door.
- `Narrate(p Phase, holderName, holderPlainName string) narration.Roles` renders
  it. This is what `ApplyConditions`, both round ticks and `PruneConditions`
  call; they deliver each role themselves on today's category and channel. It
  takes the HOLDER, not a token context: the holder is always the actee, so the
  store fills the slot and no call site can put the name in the wrong one
  (messaging M4a). The holder's name renders `{actee}` and `{actee_plain}`.
- `AuthoredStartLine(holderName, holderPlainName string) string` renders
  `start_actee` as written, taking the holder for the same reason, ignoring
  the notice rules: the door for a silent-start condition's applier (sleep 15,
  arrest 88, stun 84, broken limb 83). Throttled (89) is silent-start too but
  has no sender: its move narrates the choke itself.
- `Validate` runs `narration.ValidateVariants` over every authored phase, on the
  raw fields, so a whitespace-only line fails the load even where a notice
  would hide it.
- **No file outside this package reads the six text fields.** The root guard
  `store_text_fields_guard_test.go` fails the build on one.

The six authored keys are `start_actee` / `start_observer`, `trigger_actee` /
`trigger_observer` and `end_actee` / `end_observer`. M4b-1 renamed them from
`start_user_text` / `start_room_text` and their trigger and end siblings: the
phase half stays, the role half is now the canonical vocabulary every
narration store shares. The holder's half is `actee`, NOT `actor`, because a
condition is something that happens TO whoever holds it; that asymmetry is the
same one M4a's token flip recorded, and it is deliberate.

## Data Management and Search

### Condition Discovery

`SearchConditions(searchTerm string) []int` returns the ids of every loaded
spec whose name or description contains the term (case-insensitive).
`GetAllConditionIds() []int` returns every loaded id. `GetConditionSpec(id)`
returns the spec or nil, and `HasSpec(id)` is its boolean form, injected into
cross-package validators such as `species.ValidateSpeciesConditionIds`.

### File Management
```go
// Generate filename for condition specification
func (b *ConditionSpec) Filename() string {
    filename := util.ConvertForFilename(b.Name)
    return fmt.Sprintf("%d-%s.yaml", b.ConditionId, filename)
}

// Load all condition specifications from files (abridged). The data folder
// is `conditions/`.
func LoadDataFiles() {
    dataPath := string(configs.GetFilePathsConfig().DataFiles) + `/conditions`
    tmpConditions, err := fileloader.LoadAllFlatFiles[int, *ConditionSpec](dataPath)
    if err != nil {
        panic(errors.Wrap(err, `filepath: `+dataPath))
    }
    conditions = tmpConditions
    ValidateLoadedFlags()
}
```

## Integration Patterns

### Character System Integration
```go
// Conditions integrate with character stats and behavior
- character.Conditions.StatMod("strength")          // Stat modifications
- character.Conditions.HasFlag(conditions.NoCombat, false) // Behavioral restrictions
- character.Conditions.Trigger()                    // Round-based processing
- character.Conditions.Prune()                      // Cleanup expired conditions
```

`(*Condition).ExpiresOnNextTrigger(spec)` asks Trigger's own arithmetic one
round early: true when the next `Trigger` leaves the record expired (its last
trigger lands, or a stacking record's last stack drops). It must change
whenever `Trigger` does; `TestExpiresOnNextTriggerAgreesWithTrigger` pins the
two together. The hooks round ticks use it to snapshot a room just before a
light or darkness runs out, for its end line (#220).

### Combat System Integration
```go
// Combat checks condition flags for behavior modification
if sourceChar.HasConditionFlag(conditions.Accuracy) {
    critChance *= 2 // Double crit chance
}

if targetChar.HasConditionFlag(conditions.Blink) {
    critChance /= 2 // Half crit chance against blink
}
```

### Event System Integration
```go
// Conditions queue an event for start, effect, and end
events.AddToQueue(events.Condition{
    MobInstanceId: mobInstanceId,
    ConditionId:   conditionId,
    Source:        source,
})
```

## Usage Examples

### Basic Condition Management
```go
// Create new condition collection
held := conditions.New()

// Add temporary condition
held.AddCondition(poisonConditionId, false)

// Add permanent condition (from equipment)
held.AddCondition(strengthConditionId, true)

// Check for specific behavior
if held.HasFlag(conditions.NoCombat, false) {
    user.SendText("You cannot engage in combat right now.")
    return
}

// Process round-based triggers
for _, c := range held.Trigger() {
    processCondition(c)
}

// Clean up expired conditions
for _, c := range held.Prune() {
    notifyConditionExpired(c)
}
```

### Stat Modification Usage
```go
// Calculate total stat bonuses from all conditions
strengthBonus := character.Conditions.StatMod("strength")
dexBonus := character.Conditions.StatMod("dexterity")
```

### Flag-Based Behavior Control
```go
// Check movement restrictions
if character.Conditions.HasFlag(conditions.NoMovement, false) {
    user.SendText("You are unable to move.")
    return
}

// Check with expiration
if character.Conditions.HasFlag(conditions.CancelOnAction, true) {
    user.SendText("Your concentration is broken!")
    // Condition automatically expired by HasFlag call
}
```

## Dependencies

- `internal/statmods` - Stat modification system integration
- `internal/configs` - Configuration management for file paths and timing
- `internal/gametime` - Game time system for trigger rate calculations
- `internal/fileloader` - YAML file loading and validation system
- `internal/util` - Utility functions for file operations and validation
- `internal/mudlog` - Logging system for debugging and monitoring

---

## DOGMud chunk-4d conditions

Two conditions added in chunk 4d (T9 + T10). Neither is a regen potion or
combat potion; they are combat consequence conditions applied by the
submission outcome resolver (`internal/combat/submission_outcome.go`).

| ID | Name | Duration | Source | Effect |
|----|------|----------|--------|--------|
| 83 | Broken Limb | ~3600 rounds (~1 hr play) | Cripple submission outcome via `applyBrokenLimbCondition` | Reduces combat effectiveness for the afflicted limb's weapon role; persists across respawn; cannot be dispelled early by normal means |
| 84 | Submission Stunned | 1 round | Crit submission tier (mercy policy only) via `applyStunnedCondition` | Brief combat stagger; auto-clears at the end of the following round |

**Condition 83 (Broken Limb)** is the first persistent, non-dispellable
harmful combat condition players commonly encounter. Triggered by a
cripple-policy submission where the sub type targets a joint (armbar, kimura).
Choke-class subs (RNC, Triangle, Anaconda, Guillotine) do NOT trigger
condition 83 because they have no body-part target; the policy degrades to
subdue instead.

**Condition 84 (Submission Stunned)** is a 1-round stagger applied to the
recipient when a mercy-policy submission lands a crit roll
(`SubTierCrit`). Only fires on mercy policy because subdue/cripple/lethal
send the defender through the death cascade and the condition would be a
no-op.

See `internal/combat/context.md` "Submission System" for the full
context in which these conditions are applied.

---

## DOGMud chunk-3.3 conditions (Sleeping)

### New flags

| Flag | String value | Purpose |
|------|-------------|---------|
| `Sleeping` | `"sleeping"` | Bearer is asleep: gates regen boost, first-hit-crit, room rendering. Chunk 3.3. |
| `CancelOnDamage` | `"cancel-on-damage"` | Condition cancels when any damage is applied to bearer. Wired in damage pipeline. Chunk 3.3. |

### New condition

| ID | Name | Duration | Source | Effect |
|----|------|----------|--------|--------|
| 15 | Sleeping | Unlimited (until woken) | `actions.Sleep`, sleep user command, sleep mob command, schedule executor | Applies `Sleeping` + `CancelOnDamage` + `NoCombat` + `NoMovement` flags; triggers `SleepRegenMultiplier` (5x) regen; forces first-hit-crit on all attackers for the round the condition is active. Cancelled by damage, failed steal, shout-in-room, light source entering room, `stand`, or schedule segment end. |

### Usage pattern

```go
// Check if character is asleep
if c.HasConditionFlag(conditions.Sleeping) { ... }

// Wake a sleeper (central hook)
mobs.OnSleeperWoken(c)

// Cancel all sleeping conditions (schedule exit path)
c.CancelConditionsWithFlag(conditions.Sleeping)
```

---

## DOGMud chunk-5.1c conditions (Jailed)

### New condition

| ID | Name | Duration | Source | Effect |
|----|------|----------|--------|--------|
| 88 | Jailed | Scaled to sentence rounds via `AddConditionScaled` | `internal/justice.ExecuteArrest` | Carries two flags: `no-go` (`NoMovement`) prevents all movement, and `no-aggro-target` makes the bearer invisible to mob aggro targeting. `TriggersLeft` is set to the sentence length in rounds so the condition expires naturally at sentence end. Removed explicitly by `internal/justice.ResolveDetention` on timer expiry or fine payment. |

**`NoMovement` flag** (`no-go`, `NoMovement Flag = "no-go"`) is checked by
`go.go` (room-exit commands), `flee.go` (flee), and `spell_foldrecall.go`
(recall) to keep a jailed player locked in by every egress path.

**`NoAggroTarget` flag** (`no-aggro-target`), the same flag respawn-grace
uses, makes the jailed player un-targetable by all mob aggro paths
(LookForTrouble, retarget, etc.). The combat round
(`hooks/NewRound_DoCombat.go`) additionally drops a mob's *stale* aggro on
a `no-aggro-target` player, so a guard that was already fighting the player
before arrest stops pursuing them into the cell.

**`AddConditionScaled(conditionId int, durationMult float64)`** is the
mechanism: passing `float64(rounds)` as the multiplier sets
`TriggersLeft = TriggerCount * durationMult`. For condition 88
(TriggerCount=1, TriggerRate="1 round"), this yields exactly `rounds` triggers
remaining, one per round of the sentence.

The condition's `start_actee` and `end_actee` fire automatically via
the condition system at cell entry and at removal. Because `RemoveCondition`
fires `end_actee` ("The cell door swings open. You are free to go."), that
is the single release line for BOTH the timer-expiry and pay-fine paths;
`ResolveDetention` deliberately sends no release flavor of its own (avoids
the duplicate-message bug). `ExecuteArrest` sends an additional
arrest-context line at cell entry; `payfine` sends a payment line that does
not mention the door.

---

## DOGMud Shield/Ward Spell Scaling

Two live spells use `effect_type: shield`: `conviction-ward`
(`effect_magnitude: 75`) and `chrysalis-cocoon` (`effect_magnitude: 125`), both
defined under `_datafiles/world/dogmud/spells/`. `applySpellShield` in `internal/hooks/spell_help_effects.go` is the single
handler for every shield spell, whoever casts it and whoever it lands on
(parity slice 3b; a shield on a pet, or from a creature, applied nothing
before it).

```go
shieldBonus := (stat + weightedSkill) / 3
if c.magnitude > 0 {
    shieldBonus = int(math.Round(float64(shieldBonus) * float64(c.magnitude) / 100.0))
}
_ = c.targetChar().AddConditionMagnitude(conditions.ConditionIdMinorShield, duration, float64(shieldBonus), "spell")
```

`stat` and the skill come from `spellCasterStatAndSkill`: the spell's
primarystat through `CasterStatValue`, and the school's cast skill.
`weightedSkill` is that skill times `SkillWeight` (ships 5.0 against a Go
default of 2.0). `magnitude` is `spellData.EffectMagnitude`, and 100 is the
1.0x baseline: a spell carrying `effect_magnitude: 75` applies 0.75 of the
base roll, one carrying 125 applies 1.25x. A shield does not crit. The
player path used to carry a x1.5 crit bump no cast could reach: both
shipped shields are `attack_type: none`, so every resolver takes them
through `resolveHelpSpell` with `uncontestedSpellResult()` and no roll;
slice 3b deleted the bump (owner ruling, 2026-09-28). A future crit would
need a real roll: the static-difficulty seam
`contest.AgainstDifficulty(score, difficulty)` (`internal/contest/contest.go`)
already exists and is used by search, track, and forage checks; no spell path
calls it. Duration is computed
by the unexported `calcSpellDuration(baseFolds, spellcastingSkill, willpower)`
in the same file, not by anything in this package:
`duration = baseFolds * (10 + willpower/20 + spellcastingSkill/2)`.

**Where the magnitude actually lands.** The 119 Minor Shield record does not
touch `magical_mitigation` or `conviction_mitigation` at all. Its declared
effect is `mitigation_flat: magnitude`, and the only reader of that kind on a
character is `Character.GetPhysicalMitigation()`
(`internal/characters/combat.go`), which takes it through
`c.Conditions.Effect(conditions.EffectMitigationFlat)` and sums it with gear
`physical_mitigation`, mutation natural armor, and species natural armor, then
clamps the total at `PhysicalMitigationCap`. So both "magical" ward spells buy
physical mitigation through the Minor Shield record, not magical or conviction
mitigation.

A shield spell can separately carry `condition_ids`,
and those conditions use the ordinary statmod path described above instead:
`chrysalis-cocoon` grants `condition_ids: [52]` (Chrysalis Shell, in
`_datafiles/world/dogmud/conditions/52-chrysalis_shell.yaml`), whose
`statmods: {magical_mitigation: 15, conviction_mitigation: 15}` are summed by
`Conditions.StatMod()` and read by `Character.GetMagicalMitigation()` /
`GetConvictionMitigation()` through `c.StatMod("magical_mitigation")` /
`c.StatMod("conviction_mitigation")`. `conviction-ward` sets no `condition_ids`, so
it grants no magical or conviction mitigation at all despite its name.

### Mitigation caps

| Cap | Shipped (`config.yaml`) | Go default |
|-----|--------------------------|------------|
| `PhysicalMitigationCap` | 0.75 | 0.75 |
| `MagicalMitigationCap` | 0.75 | 0.75 |
| `ConvictionMitigationCap` | 0.75 | 0.75 |

All three are read by `combat.ApplyMitigation`
(`internal/combat/damage_pipeline.go`) and validated positive and at most 1.0
by `internal/configs/smoke_test.go`. Nothing in this package enforces them;
they live downstream, in the damage pipeline.

## Gotchas

- **Shield magnitude converges on the physical mitigation cap, not on a
  per-spell ceiling.** `conviction-ward` (magnitude 75) and `chrysalis-cocoon`
  (magnitude 125) both feed `GetPhysicalMitigation()`, and once a caster's
  shield roll alone clears `PhysicalMitigationCap` (0.75), every shield spell
  reads identically at the player: same condition, same message, same
  effective mitigation, regardless of magnitude. The full mechanism, the
  thresholds it saturates at, and the design options under discussion are in
  `docs/audits/2026-08-30-shield-spells-converge-at-the-cap.md`; read that
  before re-deriving or retuning this.
- The one durable difference between the two spells today is
  `chrysalis-cocoon`'s `condition_ids: [52]` grant (adds magical and conviction
  mitigation through the statmod path above, invisible unless the target is
  actually taking magical or conviction damage) and its longer `base_folds`
  (8 against 4), which changes duration but is never surfaced to the player.

## Files

| File | Purpose |
|------|---------|
| `conditionspec.go` | The authored `ConditionSpec` and its loader |
| `notice.go` | The player-side start/end notice resolver and `SilentNoticeConditions` |
| `narration.go` | The narration door: `Phase`, `Narration`, `Narrate`, `AuthoredStartLine`, `validateNarration` |
| `conditions.go` | Held condition instances (`Condition`, `Conditions`), flags, stat mods, `AddConditionMagnitude`, `GetDurations` |
| `tick.go` | `ComputeTickAmount`, the tick-pool amount formula |
| `stacks.go` | `Stack`, the stacking tick (`addStack`, `syncStacks`, `tickStacks`), `tickAmountFor`, `DisplayName` |
| `effects.go` | `EffectKind`, the closed effects vocabulary, `Conditions.Effect` / `Conditions.HasEffect` / `Conditions.EffectValues` (lighting plan 5c), `ScaledKinds` / `ConditionSpec.ScaledKind` (lighting plan 5c) |
| `scaled_magnitude.go` | `SpellScaledMagnitude` (lighting plan 5c): the base + stat/D1 + skill/D2 value a spell applies a scaled kind at, capped; `CapScaledMagnitude`, the cap both spell and potion apply (infra reach at `LightInfraReachCap`, nightvision at `configs.LightWindowShiftCap`); `SpellScaledTriggers` (lighting plan 5d), the trigger count, where darkness reads its own `LightDarknessSpellDuration*` trio and the other kinds share the light trio; `NewCharacterSpellStat` / `NewCharacterSpellSkill`, the numbers the admin `setcondition` command evaluates it at |
| `light.go` | Lighting plan 5a: `LightTrim`, `Condition.LightMax` / `LightNow` / `SetLightOutput` / `ResetLight`, `Conditions.LightSources`; plan 5d: `Conditions.DarknessSources`, `Conditions.LightAndDarknessSources` |
| `ids.go` | The record ids the engine names in code: `ConditionIdWarcry` (79) through `ConditionIdEnchantWithdrawal` (123) |
| `test_helpers.go` | Test fixtures: `SeedConditionsForTest` (replaces the registry) and `SeedConditionRecordsForTest` (adds 79, 80 and 117 to 123 on top of whatever is already seeded) |

Condition files are named `{conditionid}-{ConvertForFilename(name)}.yaml`: `name:
Stunned` must be `2-stunned.yaml`, or loading panics at startup.
