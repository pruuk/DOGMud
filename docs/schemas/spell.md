# Spell Schema Reference

## 1. Filename & Location

**Path formula:**
```
_datafiles/world/dogmud/spells/{spellid}.yaml
_datafiles/world/dogmud/spells/{spellid}.js   (optional — only if spell has logic)
```

- `{spellid}` is used **directly** as the filename — no `ConvertForFilename` conversion.
- The `.js` file is **optional**. Flavor-only spells use YAML text fields
  instead (see Section 2b). Only create a `.js` when the spell needs
  custom logic (companion spawning, teleportation, validation, etc.).

**Worked example:**
- spellid: `fire-bolt`
- YAML: `_datafiles/world/dogmud/spells/fire-bolt.yaml`
- JS:   `_datafiles/world/dogmud/spells/fire-bolt.js` (only if logic needed)

**Existing spells** (for reference IDs):
`aidskill`, `blind`, `curepoison`, `fireball`, `fire-bolt`, `heal`, `healall`, `illum`, `minor-shield`, `mm`, `sparks`, `stun`, `tame`, `throw-stone`

---

## 2. Field Reference

### SpellData Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `spellid` | string | **yes** | Must match filename exactly. |
| `name` | string | **yes** | Display name shown to players. |
| `description` | string | **yes** | Flavor text describing the spell. |
| `attack_type` | string | **yes** | `"spell"` for a harmful cast, `"none"` for a cast that harms nobody. See the axes table below. |
| `damage_type` | string | **yes** | `"physical"`, `"mental"`, `"social"`, or `"non_harm"`. See the axes table below. |
| `targeting` | string | **yes** | `"self"`, `"single"`, `"multi"`, or `"area"`. See the axes table below. |
| `schools` | list | **yes** | One or more school tags. See valid schools below. |
| `cost` | int | no | Conviction (mana) cost. Default: 0. |
| `healthcost` | int | no | HP cost on cast (for blood magic / life-force spells). |
| `waitrounds` | int | no | Rounds of casting time before the spell fires. |
| `difficulty` | int | no | Adjusts final success chance by this percentage (negative = harder). |
| `primarystat` | string | no | Stat used for spell rolls and progression. Usually `willpower` or `perception`. |
| `base_folds` | int | no | Base fold complexity. 0 = defaults to 4. |
| `component_tag` | string | no | Required item component (e.g. `"stone"` requires throw-stone component). |
| `effect_type` | string | no | `"damage"`, `"heal"`, `"condition"`, `"tame"`, `"shield"`, `"charm"`. |
| `effect_magnitude` | int | no | Base damage or heal amount for simple effects. |
| `condition_ids` | list | no | Condition IDs applied to target on success (for `effect_type: condition`). |
| `summon_mob_id` | int | no | Mob ID to summon. Non-zero = this is a summon spell. |
| `summon_pet_multiplier` | float | **yes for summons** | The pet's tier dial. Scales the caster's own power into the companion's stat pool, and scales `CompanionReserveDefault` into the ongoing Conviction the companion reserves. See "Summon pet multipliers" below. |
| `summon_component_id` | int | no | Item ID consumed on cast. 0 = no component needed. |
| `summon_requires_corpse` | bool | no | If true, requires and consumes a room corpse. |
| `summon_min_corpse_pool` | int | no | Minimum corpse stat pool required for raise spells. |
| `cast_actor` | string | no | Text sent to caster on cast. Supports `{actor}`, `{actee}` tokens. |
| `cast_observer` | string | no | Text sent to room on cast. Supports `{actor}`, `{actee}` tokens. |
| `wait_actor` | string | no | Text sent to caster each wait round. |
| `wait_observer` | string | no | Text sent to room each wait round. For a mob cast held by `min_cast_rounds`, it replaces the generic "weaves magic" line on each held round. |
| `min_cast_rounds` | int | no | Mob casts only, and only on a `mob_only: true` spell (validated at load). Holds a mob's cast open for at least this many rounds, whatever its fold rate, so a boss telegraph can be interrupted across the whole windup. 0 or absent changes nothing. Shipped at 2 on `core-discharge`, `core-drain` and `warden-discharge` (#480). |
| `magic_actor` | string | no | Text sent to caster on resolution. |
| `magic_observer` | string | no | Text sent to room on resolution. |

The six narration keys name the PHASE and then the AUDIENCE. A spell's caster
is the `actor`, the opposite of a condition, where the holder it happens to is
the `actee`. Before the M4b-1 role-key rename these were `cast_user_text` /
`cast_room_text` and their wait and magic siblings, and the tokens were
`{source}` and `{target}`. An unknown token now fails the load with a boot
panic, so an old spelling will not quietly render raw.

### YAML Text Fields (Section 2b)

Flavor text can live in YAML instead of JS. The engine sends YAML text
automatically before calling any JS hooks. If a `.js` file also exists,
both run (YAML text first, then JS).

**Token substitution:**

| Token | Resolves to |
|-------|------------|
| `{actor}` | Caster's ANSI-tagged display name |
| `{actee}` | Target's ANSI-tagged display name |
| `{actor_plain}` | Caster's plain name (for possessives) |
| `{actee_plain}` | Target's plain name |

**Example — flavor-only spell (no JS needed):**
```yaml
spellid: conviction-surge
name: Conviction Surge
attack_type: none
damage_type: non_harm
targeting: single
schools:
  - enhancement
cost: 35
waitrounds: 1
effect_type: condition
condition_ids:
  - 26
cast_actor: You channel conviction into empowering energy.
cast_observer: "{actor} gathers conviction, a fierce glow building."
```

**Example — spell with logic (JS handles onMagic only):**
```yaml
spellid: raise-skeleton
name: Raise Skeleton
attack_type: none
damage_type: non_harm
targeting: self
# ... other fields ...
cast_actor: You reach toward the remains, dark energy gathering.
cast_observer: "{actor} reaches toward the remains, tendrils of shadow curling from outstretched fingers."
# JS file still exists for onMagic companion spawning logic
```

**Example — Raise Skeleton (corpse-consuming summon):**
```yaml
spellid: raise-skeleton
name: Raise Skeleton
attack_type: none
damage_type: non_harm
targeting: self
schools:
  - manifestation
cost: 30
waitrounds: 4
summon_mob_id: 300
summon_pet_multiplier: 0.50
summon_requires_corpse: true
summon_min_corpse_pool: 30
cast_actor: "You reach toward the remains, dark energy gathering."
cast_observer: "{actor} reaches toward the remains, tendrils of shadow curling."
```

**Example — Conjure Earth Elemental (no corpse, no component):**
```yaml
spellid: conjure-earth
name: Conjure Earth Elemental
attack_type: none
damage_type: non_harm
targeting: self
schools:
  - manifestation
cost: 45
waitrounds: 3
summon_mob_id: 311
summon_pet_multiplier: 1.05
cast_actor: "You slam your fist into the ground, willing stone to rise."
cast_observer: "{actor} slams a fist into the ground with a thunderous crack."
```

### Summon pet multipliers

`summon_pet_multiplier` is the **only** dial that separates one pet tier from
another. It is a float, and it does three jobs at once:

1. **Stat pool.** `characters.CalcCompanionPool` builds a power base from the
   caster (`charisma + manifestation x 5`), averages the corpse's pool into
   that base for corpse-consuming raises, and applies the multiplier **after**
   the average. Applying it after is what keeps a golem visibly stronger than
   a skeleton at every corpse size instead of only at small ones.
2. **Ongoing reservation.** `characters.CompanionReserveBase` returns
   `round(CompanionReserveDefault x summon_pet_multiplier)`. Reservation is
   **derived, never authored**: there is no per-spell reservation field, and
   adding one back would give the value two sources of truth.
3. **Validation.** A spell with `summon_mob_id` set and no positive
   `summon_pet_multiplier` logs a warning at load and will field a
   pool-of-one companion.

Cast `cost` is a separate, one-time toll and is authored per spell. It is
deliberately not derived from the multiplier: a companion persists across
logout and reboot, so the ongoing reservation is where tier differences
belong.

**Retired fields.** `summon_base_pool`, `summon_scaling_divisor` and
`summon_conviction_reserve` were removed in U7b (2026-08-15). The loader no
longer reads any of them, and leaving one in a spell file has no effect at
all. Do not copy them from an old file.

### The Three Axes (replaces `type` and `target_defense_type`)

Every spell authors `attack_type`, `damage_type` and `targeting` instead of
the old single `type` field. `attack_type: none` pairs ONLY with
`damage_type: non_harm` (an uncontested cast — a heal is not an attack);
every other pairing is `attack_type: spell` with a real `damage_type`.
`damage_type` also picks the defence set (`physical` -> dodge/block,
`mental` -> quell, `social` -> defy) and the mitigation channel a harmful
spell's damage is reduced through. `self` means NO target is resolved and
the argument text passes through (summons, identify); a non-harm cast that
should default to the caster is `single`. A harmful cast cannot be `self`
(the loader refuses it: `self` resolves no target, so a harmful `self` cast
could never be cast).

The table below maps each retired `SpellType` value to its axes, for anyone
updating an old spell file from memory:

| Old `type` value | `attack_type` | `damage_type` | `targeting` |
|-------------------|---------------|----------------|-------------|
| `neutral` | `none` | `non_harm` | `self` |
| `harmsingle` | `spell` | `physical`, `mental`, or `social` (was `target_defense_type`) | `single` |
| `helpsingle` | `none` | `non_harm` | `single` |
| `harmmulti` | `spell` | `physical`, `mental`, or `social` | `multi` |
| `helpmulti` | `none` | `non_harm` | `multi` |
| `harmarea` | `spell` | `physical`, `mental`, or `social` | `area` |
| `helparea` | `none` | `non_harm` | `area` |

### Valid School Values

| Value | Meaning |
|-------|---------|
| `elemental` | Fire, ice, lightning, earth spells |
| `enhancement` | Empowering conditions, shields, stat boosts |
| `mental` | Mind control, illusion, stunning |
| `vital` | Healing, life force, death |

A spell can belong to multiple schools:
```yaml
schools:
  - elemental
  - mental
```

---

## 3. JS Script Contract

A `.js` file is only needed for spells with custom logic (validation,
companion spawning, teleportation, etc.). Flavor-only spells should use
YAML text fields instead.

When a `.js` is needed, it can define up to three functions:

```javascript
// Called when casting begins (the cast command is issued)
// Return false to abort the cast (with a reason message already sent)
function onCast(sourceActor, targetActor) {
    // Validate target, send pre-cast messages
    // Return true to proceed, false to cancel
    return true;
}

// Called each wait round (if waitrounds > 0)
// Return false to cancel mid-cast
function onWait(sourceActor, targetActor) {
    // Send "still casting" messages
    // Return true to continue, false to cancel
    return true;
}

// Called when the spell successfully resolves
function onMagic(sourceActor, targetActor) {
    // Apply effects, send result messages
    // No return value needed
}
```

**Key JS API methods:**
```javascript
sourceActor.GetRoomId()           // Room the caster is in
sourceActor.UserId()              // User ID (0 for mobs)
sourceActor.GetCharacterName(true) // Display name
targetActor.GetHealth()           // Current HP (negative = incapacitated)
targetActor.AddHealth(amount)     // Heal/damage target

SendUserMessage(userId, text)     // Send to one player
SendRoomMessage(roomId, text, ...excludeIds)  // Send to room, excluding IDs
```

---

## 4. Annotated Example

```yaml
# _datafiles/world/dogmud/spells/aidskill.yaml
spellid: aidskill              # Filename: aidskill.yaml (no conversion)
name: Aid
description: Revives a fallen ally
attack_type: none              # A revive is not an attack
damage_type: non_harm
targeting: single              # Targets one ally
schools:
  - vital                      # Healing school
cost: 0                        # No conviction cost (tied to skill use)
waitrounds: 2                  # 2-round casting time
difficulty: 0                  # Standard difficulty
primarystat: willpower         # Willpower governs rolls and progression
```

**Corresponding JS** (abbreviated):
```javascript
// aidskill.js
function onCast(sourceActor, targetActor) {
    if (targetActor.GetHealth() > 0) {
        SendUserMessage(sourceActor.UserId(), targetActor.GetCharacterName(true) + ' is not in need of aid.');
        return false;  // Abort — target isn't down
    }
    // Send pre-cast messages to source, target, room
    return true;
}

function onWait(sourceActor, targetActor) {
    if (targetActor.GetHealth() > 0) {
        SendUserMessage(sourceActor.UserId(), 'They are no longer in need of aid.');
        return false;
    }
    // Send "still working" messages
    return true;
}

function onMagic(sourceActor, targetActor) {
    let hp = targetActor.GetHealth();
    if (hp > 0) { return; }
    targetActor.AddHealth((hp * -1) + 1);  // Revive to 1 HP
    // Send success messages — NO raw numbers to player
}
```

---

## 5. Gotchas

**spellid IS the filename — no ConvertForFilename.**
Unlike mobs/items/conditions, spell filenames use the `spellid` value directly.
`spellid: fire-bolt` → `fire-bolt.yaml`. Do not apply underscore conversion.

**JS is optional.** Flavor-only spells use YAML text fields. Only create
a `.js` file when the spell needs custom logic (companion spawning,
validation, teleportation, etc.). If a `.js` exists, it runs after YAML
text is sent.

**`waitrounds: 0` means instant.**
The `onWait` function is never called for instant spells.

**`effect_magnitude` for simple spells only.**
For spells with complex logic in JS, `effect_magnitude` is ignored. The JS `onMagic` function handles all effect application. Only use `effect_magnitude` for spells that rely on the engine's built-in effect system.

**Never display raw damage/heal numbers to players.**
The JS must use `combat.GetDamageDescription()` / `combat.GetHealDescription()` or equivalent descriptive language. See CLAUDE.md: "Player-Facing Messages — No Hard Numbers".

**School tags affect progression, not just flavor.**
Players progress different skill trees based on spell schools. An `elemental` spell advances the elemental magic skill; a `vital` spell advances the vital magic skill. Assign schools accurately.
