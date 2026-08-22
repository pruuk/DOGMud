# DOGMud Stats System Context

## Overview

The DOGMud stats system provides a character attribute framework with six core statistics that govern all character capabilities. Stats improve organically through use (skill-based progression) rather than through level-ups. The system features training point allocation and equipment modifications. There is no diminishing-returns step on stat values — see "Stat Calculation" below.

**DOGMud Differences from upstream GoMud:**
- Stats renamed: Speed → Dexterity, Smarts → Perception, Mysticism → Willpower, old Perception → Charisma
- Level-based progression removed — stats improve through use and training
- Mana removed — spells use Conviction instead
- Three resource pools: Health, Stamina, Conviction (no Mana)

## Architecture

The stats system is built around several key components:

### Core Components

**Six Primary Statistics:**
- **Strength**: Physical power affecting damage output and carrying capacity
- **Dexterity**: Agility and reflexes affecting combat speed and dodging
- **Perception**: Awareness affecting detection and observation abilities
- **Vitality**: Health and stamina affecting hit points and endurance
- **Willpower**: Mental fortitude affecting conviction capacity and spell effectiveness
- **Charisma**: Social influence affecting NPC interactions and charm abilities

**Multi-Layer Calculation System:**
- **Base Values**: Species starting statistics
- **Training**: What use-based progression has added since the character existed. Never authored in data; see "Base vs Training" below.
- **Equipment Modifiers**: Temporary bonuses from gear and effects
- **No Diminishing Returns**: `ValueAdj` is a straight copy of `Value` — stats are used raw, uncapped

## Key Features

### 1. **Use-Based Progression**
- Stats improve through gameplay — using Strength-related actions improves Strength
- Exponential decay progression curve (easy early gains, harder at high values)
- Progression slows sharply past virtual rank `StatProgressionSoftCap` (default
  150) — this throttles *how fast* a stat gains, not the stat value itself;
  there is no ceiling on the value
- No level-up stat grants

### 2. **Species Differentiation**
- All players are Human with balanced base stats
- NPC species have varied base stats
- Base stats influence starting capabilities

### 3. **Flexible Modification System**
- **Equipment Bonuses**: Gear provides temporary stat improvements
- **Spell Effects**: Magic can enhance or reduce statistics
- **Buff Integration**: Status effects modify character capabilities
- **Dynamic Recalculation**: Stats update automatically when modifiers change

## Stat Structure

### Statistics Collection
```go
type Statistics struct {
    Strength   StatInfo // Physical power and damage
    Dexterity  StatInfo // Agility, reflexes, and combat speed
    Perception StatInfo // Awareness, detection, and observation
    Vitality   StatInfo // Health, stamina, and endurance
    Willpower  StatInfo // Mental fortitude and conviction capacity
    Charisma   StatInfo // Social influence and charm
}
```

### Individual Stat Information
```go
type StatInfo struct {
    Training int  // Points progression has added since the character existed (persistent)
    Value    int  // Final calculated value (runtime)
    ValueAdj int  // Always equals Value now; kept only so existing call sites compile
    Racial   int  // Copy of Base, set by Recalculate (runtime)
    Base     int  // Starting value: species baseline, character roll, or authored (persistent)
    Mods     int  // Equipment/effect modifiers (runtime)
    BaseAuthored bool // The YAML carried a `base:` key (runtime, see below)
}
```

### `Base` vs `Training`, and `BaseAuthored`

`Base` is what the character or mob started with. `Training` is strictly what
progression added on top — `IncreaseStat` is the only thing that raises it. That
split matters because U10b-0 makes the progression curve read `Training` as its
difficulty rank: anything else parked there would make a stat harder to raise
for reasons that have nothing to do with how much it has already been raised.

Mob templates used to author their stat values under `training:`; 599 of them
did. `tools/fold_mob_training_to_base.py` moved those into `base:` on
2026-08-22, and `internal/mobs`'s `TestNoMobTemplateCarriesAuthoredTraining`
keeps them there.

`BaseAuthored` exists because that fold produces two stats whose value is
exactly zero, and `characters.Validate` fills an *unset* `Base` from the species
record. `Base == 0` alone cannot tell "nobody said" from "deliberately zero", so
`UnmarshalYAML` records whether a `base:` key was actually present and the
hydration pass consults it.

Three things to know about it:

- It implements the **gopkg.in/yaml.v2** `Unmarshaler` interface, because that
  is what `internal/fileloader` decodes with. A v3-shaped method would compile
  and never be called on the engine's real load path.
- It is runtime-only (`yaml:"-"`), and `base:` keeps `omitempty`, so a `Base` of
  0 written back out is indistinguishable from an absent one on the next load.
  That is fine for the paths that exist today — mob templates are re-read from
  their authored YAML, instance saves carry `Training` rather than `Base`, and a
  rolled player `Base` is never zero — but it is the sharp edge.
- It is false for every `StatInfo` built in Go. Hydration still also requires
  `Base == 0`, which is what stops a rolled character's stats being overwritten
  with the species baseline.

## Stat Calculation System

### Stat Calculation

    Value = Base (Racial) + Training + Mods
    ValueAdj = Value

There is no compression. `ValueAdj` is retained only so existing call sites
compile; it is always equal to `Value`. Do not reintroduce a soft cap here —
`HealthMax`, `StaminaMax`, `ConvictionMax` and `ActionPointsMax` are also
`StatInfo` and call the same `Recalculate()`, so anything added here silently
applies to every resource pool as well. That was the 2026-08-02 bug: a median
character's true 530 HP was being played as 322. Pool sizing belongs in the
`HealthPer*` / `StaminaPer*` / `ConvictionPer*` coefficients in the balance
config, where it is visible and tunable.

## Stat Applications

### Combat Integration
```go
// Strength affects damage output
baseDamage := weaponDamage + (character.Stats.Strength.ValueAdj / 10)

// Dexterity affects hit chance and attack frequency
hitBonus := character.Stats.Dexterity.ValueAdj - target.Stats.Dexterity.ValueAdj
```
(Health capacity is computed by the Resource Pools formulas below, not by a
generic stat multiplier — shown separately to avoid duplicating/contradicting
the real coefficients.)

### Resource Pools
```
HealthMax     = HealthBase + Vitality × HealthPerVitality (primary, ×3)
                            + Strength × HealthPerStrength (secondary, ×1)
StaminaMax    = StaminaBase + Vitality × StaminaPerVitality (primary, ×3)
                             + Willpower × StaminaPerWillpower (secondary, ×1)
ConvictionMax = ConvictionBase + Charisma × ConvictionPerCharisma (primary, ×3)
                                + Willpower × ConvictionPerWillpower (secondary, ×1)
```
Three resource pools (no Mana). Each pool has one primary stat (×3) and one
secondary stat (×1); the `*Base` and `*Per*` coefficients live in the balance
config (`_datafiles/config.yaml`) and are computed in
`internal/characters/validate.go` (`RecalculateStats`). Note Strength does
**not** contribute to Stamina in the current config (`StaminaPerStrength: 0`)
even though the field exists — a config knob absent from `config.yaml` falls
back to its Go zero-value default, and `0` here is a deliberate shipped value,
not a bug.

### Skill System Integration
```go
// Perception affects detection and awareness
detectionRange := baseRange + (character.Stats.Perception.ValueAdj / 20)

// Charisma affects social interactions
barteringBonus := character.Stats.Charisma.ValueAdj / 10
```

## Dependencies

- `gopkg.in/yaml.v2`, for `StatInfo.UnmarshalYAML` only. That is the library
  `internal/fileloader` decodes with, so the Unmarshaler interface has to match
  it. Nothing else here needs a dependency: `Recalculate()` stopped importing
  `math` when the compression curve was removed.
