# internal/species — Package Context

## Purpose

Implements the species system for DOGMud. Species define character ancestry,
base stats, stat modifiers, and intrinsic mutations. Unlike the upstream GoMud
race system, species are a full custom system tailored to DOGMud's mutation and
progression mechanics.

All players in DOGMud are Human (a species choice at character creation).
Mobs can be any species defined in the world data.

---

## Key Concepts

### Species Definition (SpeciesInfo struct)

A `SpeciesInfo` struct loaded from YAML (`_datafiles/world/dogmud/species/`)
contains:

- `specieid` — unique identifier (e.g., "human", "wolf", "spirit")
- `name` — display name ("Human", "Gray Wolf", "Spirit Elemental")
- `description` — flavor text shown when examining a character or mob
- `base_stats` — per-stat baseline (Strength, Dexterity, Perception, Vitality,
  Willpower, Charisma). Applied additively to a random stat pool during
  character creation.
- `stat_modifiers` — optional equipment-independent stat adjustments
  (e.g., "wolves get +10 Perception")
- `body_parts` — canonical anatomy tags (chunk 2.5)
- `intrinsic_mutations` — natural traits (chunk 2.5)
- `natural_attack` — `ItemSubType` used for BASIC unarmed attack
  combat messages (`bite`, `claws`, `slam`, `gore`, `sting`, `siphon`,
  `trample`, `lash`, `blast`). Empty
  means the humanoid default (generic-punch messaging). Validated at
  load by `validateNaturalAttack`; panics on unknown values.
- `speech` — `*bool`; `speech: false` marks a species with no voice (beasts,
  slimes, elementals, constructs). Absent means it speaks. Read through
  `(*Species).CanSpeak()`; `characters.SpeechState` is the consumer.

### Registry

```go
species.LoadSpeciesFiles()      // called once from main.go at startup
species.GetSpecies(id)          // look up a spec by id
species.GetAll()                // the full map
```

---

## Body Plan & Intrinsic Mutations (chunk 2.5)

Species declare anatomy via `BodyParts []string` and natural traits via
`IntrinsicMutations map[string]int`. The seven canonical body-part tags
are `arms`, `hands`, `legs`, `eyes`, `mouth`, `skin`, `tail` — see
`CanonicalBodyParts` in `species.go`.

- `BodyParts: nil` (absent in YAML) → fail-open. Every mutation passes
  the body-parts gate. Use for un-migrated species; legacy behavior.
- `BodyParts: []` (explicit empty in YAML) → no body parts. Every
  body-part-requiring mutation is gated. Use for incorporeal species.
- `BodyParts: [arms, hands, ...]` → declared anatomy. Mutations whose
  `RequiresBodyParts` is a subset of this list pass the gate.

`IntrinsicMutations` is merged additively into `Character.Mutations` at
character init via `Character.ApplyIntrinsicMutations(species)`. Caps
respected (default MutationMaxRank = 4).

Boot-time validation (`ValidateBodyPartTags`) panics on unknown tags
or unknown mutation ids in intrinsic_mutations.

Helpers: `HasBodyPart(tag)`, `HasAllBodyParts(required)`,
`IsCanonicalBodyPart(tag)`.

Design: `docs/superpowers/specs/completed/2026-05-12-mob-aliveness-2.5-mutations-on-mobs-design.md`

---

## Natural-Attack Subtype (Phase 1 non-human attacks)

Non-human species declare their BASIC unarmed attack messaging style via
`NaturalAttack items.ItemSubType` (YAML: `natural_attack:`). This field
selects the combat-message file used when the mob has no equipped weapon.

Valid values: `bite`, `claws`, `slam`, `gore`, `sting`, `siphon`, `trample`,
`lash`, `blast` (the keys of `validNaturalAttacks` in `species.go`). An empty string
falls back to the humanoid default (generic-punch messaging). Unknown
values cause a startup panic via `validateNaturalAttack`. `siphon` also
requires `lifedrain: true` on the species (`validateSiphonHasLifeDrain`,
panics at load), because the `drain` special move is how it steals life;
`gore` likewise requires the `horns` body part (`validateGoreHasHorns`). The field has
no effect on armed mobs — an equipped weapon's own `Subtype` always takes
precedence.

**The principle (owner ruling, 2026-10-16).** A species' default attack
(`natural_attack`) is the attack it makes most of the time. Its special
moves are rarer, or finishing moves. Pick the default attack for what the
creature does in an ordinary fight, and give the flourish to a special move.
Spec: `docs/superpowers/specs/2026-10-16-combat-message-reachability-design.md`.

**A creature is the species it looks like.** A wolf is `canine`, a crow or
a heron `bird`, a hawk `raptor` (birds that hunt), an otter `mustelid`. The
species decides the default attack, body parts, damage and which special
moves a mob can use, so a wrong species gives a creature the wrong fight.
Slice 1 of the spec above moved 32 mobs that were on the wrong species
(ported from PR #208, owner decisions in
`docs/superpowers/plans/2026-10-16-combat-reachability-slice1-species.md`);
`internal/species/species_assignment_test.go` pins them.

Species 25 `bird` (claws, "beak and claws", small; does not hunt other
beasts) and 26 `horse` (then slam, now trample; "hooves", large) were added
then. Like every beast species they set `speech: false`; a species that omits
it can speak.

### The slam split (#541, owner ruling S1, 2026-10-16)

`slam` once covered 13 species, and one file of lines could not fit hooves,
pseudopods, stone fists and a gust of wind at once ("hurl your hooves",
"smash your molten slam into", feet on a slime). It was split by body, so
each file's lines say what that body does and stay within 80 columns:

| Type | Species (id, `unarmedname`) | What its lines do |
|---|---|---|
| `slam` | earth elemental 37 "stone fists", automaton 45 "servo-driven fists" | heavy bodies pound with fists and drive their weight |
| `trample` | horse 26 "hooves" | kicks, rears, stamps; may name hooves, legs, head, chest |
| `lash` | fungal colony 15 "spore tendrils", slime 16 "pseudopods" | whips, smothers, engulfs; names no feet, head, chest or shoulder |
| `blast` | water 36, air 38, fire 39, magma 40, sand 41, storm 42, ice 43 elementals | the element strikes through its own `unarmedname`; no body parts they lack |

The smoke elemental (44) left slam for `claws` ("smoke-wrought claws"): it
materialises claws and rakes. Its signature `smother` special move lands in
slice 2 (Appendix A of
`docs/superpowers/specs/2026-10-16-combat-message-reachability-design.md`).
`species_slam_split_test.go` (`TestSlamSplitAssignments`) pins every row.
Spec: `docs/superpowers/specs/2026-10-16-slam-split-design.md`.

### Choosing a natural attack for a new species

1. **Pick the default for what it does most of the time** (the principle
   above). A rare flourish or finisher is a special move, not the default.
2. **Pick by body.** Read the candidate type's combat-message file with the
   new species' `unarmedname` in every `{itemname}` slot; if a line gives it
   a body part it lacks or a verb that does not fit, choose another type.
   Natural pools put the attacker on the verb ("You rake X with your
   {itemname}"), so a singular or plural `unarmedname` both read right.
3. **A new type needs all of these, or it fails at load or in a guard:**
   the subtype constant and description in `internal/items/itemspec.go`; an
   entry in `validNaturalAttacks` (`species.go`, checked by
   `validateNaturalAttack`); a combat-message file of the same name in
   `_datafiles/world/dogmud/combat-messages/`; a hit band in
   `combat.CategoryForWeaponSubtype` (`internal/combat/attackresult.go`); the
   `naturalWeaponPools` list in `internal/items/natural_weapon_agreement_test.go`;
   and a reach case in `items.DefaultReachForSubtype` (`internal/items/reach.go`).
   `siphon` (#538) and the slam split (#541) are the worked examples.

---

## Files in This Package

| File | Purpose |
|------|---------|
| `species.go` | Structs, registry, loader, body-part validation |
| `species_test.go` | Unit tests for body-part logic |
| `species_slam_split_test.go` | Pins each former slam species to its type after the slam split (#541) |
| `test_helpers.go` | Test utilities |
| `testing_support.go` | `SetSpeciesForTest` / `LoadForTest` — scoped roster swaps |
| `context.md` | This file — package overview for Claude Code |

### Loading the roster inside a test

`LoadDataFiles()` is called once from `main.go`, so a test binary starts with an
empty roster and `GetSpecies` returns nil for everything. Filling it is not
inert: other packages build bare fixtures whose behaviour changes once a species
record exists to hydrate from (`internal/characters`' `Wear` tests are a live
example, and they fail if the roster is left loaded).

Use `LoadForTest(t)` — it loads the real files and restores the previous roster
via `t.Cleanup` — or `SetSpeciesForTest(t, map)` to install a fixture roster.
Both mirror `configs.SetConfigForTest`. `LoadForTest` reads
`configs.GetFilePathsConfig().DataFiles` relative to the working directory, so
the caller must already be at the repo root.

---

## Integration Points

Species are consulted at:
1. **Character creation** (`modules/newchar.go`) — seed base stats, apply body-plan
   gating during random mutation acquisition, apply intrinsic mutations
2. **Mob spawn** (`mobs.go`) — apply intrinsic mutations + curated spawn mutations
   to fresh mob instances
3. **Mutation acquisition** (`internal/mutations/mutations.go`,
   `GetWeightedPool()`) — filter candidates by body-part compatibility
4. **Basic unarmed attacks** (`internal/combat/combat_helpers.go`) —
   `buildWeaponSetup` reads `NaturalAttack` to select the attack-message
   subtype for weapon-less mobs

---

## Stage Roadmap

- **chunk 2.5** (in progress) — body-plan gating, intrinsic mutations
