# DOGMud Spells System Context

## Overview

The DOGMud spells system provides a comprehensive magic framework with 45 spells across
multiple types and schools, a fold-based casting mechanic, use-based spell discovery, and
flexible targeting. Spells use Conviction (not mana) as their resource.

**DOGMud Differences from upstream GoMud:**
- Mana removed — spells use Conviction resource
- Optional Health costs for life-force magic (vital school spells may sacrifice health)
- Spell cost scaling via config multipliers (SpellConvictionCostMultiplier, SpellHealthCostMultiplier)
- Spellcasting skill (DOG) replaces legacy Cast skill for combat resolution
- Schools changed from single value to array (can have multiple schools)
- Four DOG schools: Elemental, Enhancement, Mental, Vital
- Use-based spell discovery (Phase 25) — no trainers, spells emerge through casting practice
- New effect types: dot, knockdown, purge (Phase 25)
- Starting spells reduced to 1 (Conviction Spike only)

---

## Architecture

### Core Components

**Spell Data Management:**
- Unique spell identification with string-based IDs
- YAML-based storage with automatic loading and validation
- Spell discovery by name or ID with fuzzy matching
- In-memory caching for fast spell lookups

**Spell Classification System:**
- Type-based targeting (single, multi, area, neutral)
- School-based categorization (elemental, enhancement, mental, vital)
- Harm/help classification for spell effects
- Difficulty scaling for success calculations

---

## SpellData Structure

```go
type SpellData struct {
    SpellId         string                 // Unique spell identifier (also filename base)
    Name            string                 // Display name
    Description     string                 // Spell description
    AttackType      combatvocab.AttackType // yaml: attack_type. HOW the attack is delivered ("spell", "none")
    DamageType      combatvocab.DamageType // yaml: damage_type. WHAT it does ("physical", "mental", "social", "non_harm")
    Targeting       combatvocab.Targeting  // yaml: targeting. HOW MANY it reaches ("self", "single", "multi", "area")
    Schools         []string               // Magic school classification (can have multiple)
    Cost            int                    // Conviction cost
    HealthCost      int                    // Optional Health cost for vital school
    WaitRounds      int                    // Casting delay in rounds
    Difficulty      int                    // Success modifier (0-100%)
    PrimaryStat     string                 // REQUIRED (U9). Caster-side stat. See below.
    BaseFolds       int                    // Number of folds required to cast
    EffectType      string                 // "damage", "heal", "shield", "dot", "knockdown", "purge", "none"
    EffectMagnitude int                    // Base power of the effect
    EffectDuration  int                    // For DoT: number of tick cycles
}

func (s *SpellData) CasterStatValue(stats stats.Statistics) int
func (s *SpellData) Validate() error // calls the unexported validatePrimaryStat, then validateAxes
```

**`PrimaryStat` is REQUIRED and validated at load (U9).** `Validate()` calls
the unexported `validatePrimaryStat`, which fails the boot if `primarystat:`
is empty or not one of the six stat names (`strength`, `dexterity`,
`perception`, `vitality`, `willpower`, `charisma`) -- a typo now fails at
startup instead of silently doing nothing, which is what the field used to
do (see the boot-test SOP in `CLAUDE.md`). `CasterStatValue(stats
stats.Statistics) int` reads the matching field off the caster's
`stats.Statistics` and is CASTER SIDE ONLY -- the defender's stat is owned
by the U6 defence set (`quell` stays on Willpower by design; routing it
through here would silently move quell off the stat U6 designed it around).

It drives:
- the caster's spell attack roll (`characters.CalcSpellAttack(spellData.CasterStatValue(...), skillLevel)`,
  `internal/hooks/spell_resolution.go`),
- spell duration and shield-bonus magnitude (`calcSpellDuration`, both via
  `spellData.CasterStatValue(...)`),
- and, in `internal/hooks/NewRound_DoCombat_helpers.go`, which stat the cast
  TRAINS: `OnSkillUseScaled` already rolls the casting skill's own default
  primary stat (spellcasting -> willpower, manifestation -> charisma), and an
  explicit `OnStatUse(spellData.PrimaryStat, userId)` fires ONLY when the
  spell's `primarystat` differs from that default -- so for every spell
  shipped at 2026-08-19 (which all declare willpower or charisma matching
  their school) this is a no-op in practice; it exists so a spell that
  declares something else actually trains it.

**It does NOT drive raw damage magnitude.** `calcSpellDamageForCharacter`
(`internal/hooks/combat_shared_helpers.go`) reads `caster.Stats.Willpower.ValueAdj`
directly through `combat.CalcRawDamage`, not `spellData.CasterStatValue(...)`.
A spell with a non-willpower `primarystat` still rolls its attack and trains
its progression off that stat, but its damage number is Willpower regardless.
This is not drift to "fix" without a design decision -- it is the current
shipped behaviour, and the Effect Types row below has been corrected to
match it rather than the aspiration.

### Spell Difficulty and Target Types

**Difficulty Field:** The `Difficulty` integer (range 0–75) affects skill
progression via difficulty-scaled bonus multiplier (applied in spell resolution).
Values: 0 (utility), 1–15 (weak combat), 15–30 (moderate), 30–50 (strong),
50–75 (apex combat spells).

**TargetTypeString:** a `targeting: self` spell prints "Self".

### The Three Axes (messaging M4b-2, `axes.go`)

The legacy `SpellType` enum (`neutral`/`harmsingle`/`harmmulti`/`helpsingle`/
`helpmulti`/`harmarea`/`helparea`) and `TargetDefenseType` are gone. Every
spell instead authors three fields, shared with the rest of combat through
`internal/combatvocab` (see that package's doc comment for the full axis
definitions):

- `attack_type` (`combatvocab.AttackType`) — `spell` for every harmful cast,
  `none` for a cast that harms nobody (a heal is not an attack). `none` pairs
  ONLY with `damage_type: non_harm` (`combatvocab.Attack.Valid`).
- `damage_type` (`combatvocab.DamageType`) — `physical`, `mental`, `social`,
  or `non_harm`. Answers "what is this?" for defence-set lookup
  (`combatvocab.EligibleDefences`) and mitigation-channel lookup
  (`combat.MitigationChannelFor`).
- `targeting` (`combatvocab.Targeting`) — `self` (no target resolved, the
  argument passes through: summons, identify), `single`, `multi`, or `area`.

`(*SpellData).Validate` calls the unexported `validateAxes`, which requires
all three keys and rejects a pairing `combatvocab`'s eligibility table does
not know — an authoring typo fails the boot rather than silently loading.

Display and routing methods derived from the axes (all on `*SpellData`,
`axes.go`):

```go
func (s *SpellData) Attack() combatvocab.Attack        // the value the contest seam resolves
func (s *SpellData) IsHarm() bool                      // DamageType.IsHarm(): harm vs. help in one place
func (s *SpellData) HelpOrHarmString() string           // "Harmful" / "Helpful" / "Neutral" (spells listing, help template)
func (s *SpellData) TargetTypeString(short ...bool) string // "Single Target" / "Group" / "Area" / "Self" (short form for the listing)
func (s *SpellData) DefenceNames() string               // "quell" / "defy" / "dodge or block" / "" — the help template's "Resisted by" line
```

### Magic Schools
```go
const (
    SchoolElemental   = "elemental"   // Fire, ice, lightning, earth — offensive elemental magic
    SchoolEnhancement = "enhancement" // Conditions, shields, enchantments — augmentation magic
    SchoolMental      = "mental"      // Illusions, charms, telepathy — mind-affecting magic
    SchoolVital       = "vital"       // Healing, curing, life/death manipulation
)
```

---

## Effect Types (spell_resolution.go)

| EffectType | Behavior |
|------------|----------|
| `damage` | Direct HP damage to target(s). New pipeline (DamageMultiplier > 0) scales off Willpower always, NOT PrimaryStat -- see the PrimaryStat note above. Legacy path (DamageMultiplier == 0) scales off EffectMagnitude. |
| `heal` | Direct HP restoration to target(s) |
| `shield` | Applies ConditionShield with magnitude = damage absorbed |
| `dot` | Applies the spell's one `condition_ids` record (Blood Boil: 143 Boiling Blood), or 121 Poisoned when it names none (Neural Toxin); duration from the caster (`applySpellDot`, `spellDotConditionId`) |
| `knockdown` | Deals damage + knocks the target Supine (face-up "slams to the ground") via `Position.TransitionToSupine(MinRecoveryRounds: 1, TriggerKnockdownSpell)`. The legacy `CombatPosition = PositionProne` parallel-write is removed (T21 sunset). Future work may add a direction config to distinguish blast (Supine default) from shockwave (Prone). |
| `purge` | Removes poison conditions and ConditionPoisoned from target(s) |
| `none` | No automatic effect — spell behavior handled in Go hooks (used by condition spells, summons, utility) |

---

## Spell Discovery System (Phase 25)

Spells are learned through casting, not trainers. After each successful cast:

1. Base discovery chance: ~5% per cast
2. Scaled down by known spell count: `chance = baseChance / (1 + knownCount * 0.1)`
3. `GetEligibleSpells()` returns unlearned spells whose `BaseFolds` ≤ skill-gated threshold
4. Random selection from eligible pool
5. Player receives: "A new pattern crystallizes in your mind: <spell name>"

### Fold Threshold Table (MaxFoldsForSkill)
| Skill Level | Max Discoverable Folds |
|-------------|----------------------|
| 1–4 | ≤ 4 |
| 5–9 | ≤ 6 |
| 10–19 | ≤ 8 |
| 20–29 | ≤ 10 |
| 30–39 | ≤ 12 |
| 40–49 | ≤ 16 |
| 50–59 | ≤ 20 |
| 60–69 | ≤ 24 |
| 70–79 | ≤ 28 |
| 80+ | ≤ 32 |

### Starting Spells
New characters begin with only `mm` (Conviction Spike). All other spells are
discovered through casting practice.

---

## Condition Integration (Phase 25.3)

Several condition flags affect spell and combat systems:

| Flag | Effect | Applied In |
|------|--------|-----------|
| `damage-bonus` | +15% physical damage | NewRound_DoCombat.go |
| `haste` | Speed effects | Various |
| `slow` | Movement penalty | Various |
| `skill-progress` | 2x skill progression chance | progression.go |
| `mutation-rate` | 2x mutation progress gain | UserRoundTick.go |

---

## Summon Fields (U7b, 2026-08-15)

`SpellData` carries the summon contract in five fields:

```go
SummonMobId          int     `yaml:"summon_mob_id,omitempty"`
SummonPetMultiplier  float64 `yaml:"summon_pet_multiplier,omitempty"`
SummonComponentId    int     `yaml:"summon_component_id,omitempty"`
SummonRequiresCorpse bool    `yaml:"summon_requires_corpse,omitempty"`
SummonMinCorpsePool  int     `yaml:"summon_min_corpse_pool,omitempty"`
```

`SummonPetMultiplier` is the **single dial for a pet's tier**. It scales the
caster's own power into the companion's stat pool via
`characters.CalcCompanionPool`, and it scales `CompanionReserveDefault` into the
ongoing Conviction the companion reserves via
`characters.CompanionReserveBase`.

**Reservation is derived, never authored.** There is no per-spell reservation
field. `SummonConvictionReserve` used to be one and was deleted precisely
because a second authorable source of truth beside the multiplier drifts on the
first retune.

**Three fields were removed and must not be re-added casually.**
`SummonBasePool` and `SummonScalingDivisor` went with the old formula, in which
the pet's base pool multiplied the caster's power and the corpse was averaged in
afterwards, so the corpse's share grew until it swamped the pet choice.
`SummonConvictionReserve` went for the reason above. `Validate` now warns when
`SummonMobId > 0` and `SummonPetMultiplier <= 0`; it used to warn on a zero
`summon_base_pool`.

## Summon Spells (Phase 25.4)

Two permanent summon spells use components, resolved in Go hooks:
- `chrysalis-construct` (20 folds) — requires Chrysalis Core (item 40010), spawns mob 110
- `summon-hive-swarm` (24 folds) — requires Hive Fragment (item 40011), spawns mob 111

Summons persist until killed, one per type per caster. Go hooks in
`spell_resolution.go` handle component checking/consumption, mob spawning,
and permanent charm via `CharmSet(userId, 99999)`.

---

## All Spells (45 total)

### Re-Themed Original Spells (14)
mm (Conviction Spike), fire-bolt (Pyretic Surge), heal (Mend Flesh),
fireball (Hemorrhagic Burst), minor-shield (Conviction Ward),
curepoison (Purge Affliction), tame (Empathic Bond), sparks (Conviction Sparks),
stun (Neural Stun), blind (Sensory Veil), throw-stone (Kinetic Hurl),
illum (Chrysalis Glow), healall (Mend All), aidskill (Chrysalis Aid).

### Damage/Heal/DoT/Shield Spells (12)
mind-spike, kinetic-shove, blood-boil, hemorrhagic-wave, synaptic-overload,
veil-rend, mend-wounds, communion-of-flesh, chrysalis-cocoon, neural-toxin,
conviction-barrage, cleansing-wave.

### Condition/Utility Spells (17)
conviction-surge, iron-will, chrysalis-haste, mind-fog, nerve-disruption,
empathic-shroud, vital-surge, chrysalis-regeneration, skill-attunement,
mutation-catalyst, psychic-anchor, sensory-overload, conviction-armor,
veil-sight, fold-anchor (set/recall toggle), mass-mend.

### Summon Spells (2)
chrysalis-construct, summon-hive-swarm.

---

## Casting Messages (`casting_messages.go`)

The atmospheric lines the casting system shows a caster, loaded from
`_datafiles/world/dogmud/casting-messages.yaml`. Four pools, keyed by category:
`already_casting`, `cast_started`, `cast_continuing`, `concentration_slipped`.

```go
type CastingMessages struct { /* four []string pools */ }

func (cm *CastingMessages) Validate() error
func GetCastMessage(category, spellName string, picker ...narration.Picker) string
```

`GetCastMessage` renders through `narration.Render` as the single-role
degenerate case: one role (the caster), no band, one token (`{spell}`).

### Gotchas

**`spellName` is the DISPLAY name (`spellInfo.Name`), never the spellid.**
Passing the id leaks an internal identifier into player output, which is what
the round loop used to do.

**A missing, unparseable or invalid file PANICS at startup**, matching defence
and combat-messages (`internal/items/itemspec.go:788`, `:795`). This changed on
2026-09-09. There used to be a `defaultCastingMessages()` fallback that silently
substituted hardcoded Go text, and it was deleted: a fallback that shadows
shipped data means a YAML typo changes what players read and nobody finds out,
the same hazard as reading a balance number from a Go default rather than
`config.yaml`.

**The validator's minimum is 3, not defence's 5.** The file ships pools of
3/3/3/4, so defence's minimum would fail boot on shipped data without improving
a single line of text. The minimum exists to catch a regression (a pool emptied,
a key renamed), not to set a content quality bar.

**One fallback deliberately survives.** An unrecognised *category* still returns
a sentence rather than `""`, because that is a caller bug the validator cannot
see, and a caller printing an empty string would show the player nothing.

⚠️ **Pool depth is a known content gap, filed to the messaging arc's M6.**
`cast_started` fires on EVERY cast and has 3 variants, against 10 to 14 per pool
for defence, so a caster sees a repeat every third spell.

### The narration door (M3 item 5b, `narration.go`)

`Phase` (`PhaseCast`, `PhaseWait`, `PhaseMagic`); `Narration(p)` puts
`*_actor` in Actor and `*_observer` in Observer (Actee is authored in M6);
`Narrate(p, ctx)` renders through `textutil.Narrate`. The cast command (player
and mob), the two wait-text sites in `NewRound_DoCombat_helpers.go`, the magic
text in `spell_resolution.go` and the mob `aid` command all render through it
and deliver on their own channel. `Validate` refuses a whitespace-only line,
and since M4a an unknown `{token}` too: the loader panics, so a typo cannot
render raw to a player.
No file outside this package reads the six text fields (root guard
`store_text_fields_guard_test.go`). The two shipped `wait_observer` lines still
go out on the audio channel; that is filed, not a property of the door.

The six authored keys are `cast_actor` / `cast_observer`, `wait_actor` /
`wait_observer` and `magic_actor` / `magic_observer`. M4b-1 renamed them from
`cast_user_text` / `cast_room_text` and their wait and magic siblings: the
phase half stays, the role half is now the canonical vocabulary every
narration store shares. A spell's caster is the `actor`, which is the opposite
of `internal/conditions`, where the holder a condition happens to is the
`actee`.

## Hook Integration Points

| Hook File | What It Does |
|-----------|-------------|
| `internal/hooks/spell_resolution.go` | Effect dispatch (damage, heal, shield, dot, knockdown, purge), HelpArea targeting. U9: the player- and mob-caster magical-crit branches build a `progression.Outcome{ToughenStat: characters.ToughenStatFor("magical"), Exceptional: progression.ExcAttackCrit}`, take `progression.BonusEvents`, and apply only the defender side via `target.Character.ApplyProgression(...)` -- see `internal/progression/context.md` and `internal/characters/context.md`'s "Contest Progression Seam" section. |
| `internal/hooks/NewRound_DoCombat_helpers.go` | Ordinary casting progression: `OnSkillUseScaled` on the casting skill (spellcasting or manifestation), then `OnStatUse(spellData.PrimaryStat, ...)` when it differs from that skill's default stat. |
| `internal/hooks/NewRound_DoCombat.go` | Spell discovery after cast, DamageBonus condition check |
| `internal/hooks/NewRound_AutoHeal.go` | Mob poison DoT ticking |
| `internal/hooks/NewRound_UserRoundTick.go` | MutationRate condition check |
| `internal/characters/progression.go` | SkillProgress condition check (2x casting skill gain) |

---

## Files in This Package

| File | Purpose |
|------|---------|
| `spells.go` | SpellData struct, registry, loader, GetEligibleSpells(), MaxFoldsForSkill() |
| `axes.go` | The three axes' display/routing methods and `validateAxes` (messaging M4b-2) |
| `narration.go` | The narration door: `Phase`, `Narration`, `Narrate`, `validateNarration` |
| `shipped_axes_test.go` | Boot guard: every shipped spell YAML carries the three axis keys and no legacy key |
| `context.md` | This file — package overview for Claude Code |

---

## Stage Roadmap

- **Phase 25.1** (complete) — Re-themed 14 spells, Go infrastructure (dot/knockdown/purge), spell discovery, HelpArea fix
- **Phase 25.2** (complete) — 12 new damage/heal/DoT/shield spells
- **Phase 25.3** (complete) — 13 new conditions, 17 new condition/utility spells, hook integration
- **Phase 25.4** (complete) — 2 summon spells with component items and permanent charm
