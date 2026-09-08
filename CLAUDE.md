# DOGMud - Claude Code Project Memory

## Subagent Model Preference
Pick the model that fits the task — don't reflexively pin everything to haiku.

- **haiku** — trivial mechanical work: a single-file grep/glob, a one-shot
  symbol lookup, a fixed-recipe edit. Cheap and fine when there's no judgment
  involved.
- **sonnet / opus** — exploration or implementation that benefits from
  reasoning: tracing how a subsystem fits together, multi-file searches where
  the answer requires synthesis, refactoring, architectural decisions,
  multi-step code writing, or executing a plan task with real logic.

We added the **codegraph MCP** specifically to cut token use on code
intelligence (sub-millisecond symbol/caller/callee queries instead of grep +
many Reads). That headroom means using a stronger exploration agent is usually
the right call when the task warrants deeper reasoning — instruct those agents
to prefer codegraph tools for symbol verification so the stronger model spends
its budget on thinking, not file-spelunking. When in doubt for a non-trivial
task, default up (sonnet), not down.

## Shop Persistence (Living Economy)
Shop economic state (stock levels, NPC gold, restock timers) persists in
`_datafiles/world/dogmud/shops/{zone}/{mobid}-room{roomid}.yaml`. This
directory is completely separate from `rooms.instances/` and
`mobs.instances/` and is NOT cleaned by the instance save cleanup SOP.
Deleting a shop file resets that merchant to template defaults (500g
starting gold, base stock levels).

Dynamic pricing ranges from 0.25x (overstocked) to 5.0x (out of stock),
driven by the `ShopAbundanceThreshold` and normalized per item by restock
quantity. Config knobs: `ShopBuyRatio`, `ShopPriceFloor`, `ShopPriceCeiling`,
`ShopAbundanceThreshold`, `ShopMaterialReserve`, `ShopGoldReserveRatio`,
`BarterMaxDiscount`, `BarterMaxBonus`.

## Project Context
- DOGMud (Delusions of Grandeur) is a MUD built on the GoMud engine
- World design document: `docs/world.md`
- Development roadmap: `docs/roadmaps/DEVELOPMENT_PLAN.md`
- Remote origin: https://github.com/pruuk/DOGMud
- Remote upstream: https://github.com/GoMudEngine/GoMud

## Regen System (Stage 29.5)
All HP/SP/CP regeneration is **percentage-of-max** — never flat values.
- Six config knobs in `Balance`: `PlayerHealthRegenPct`, `PlayerStaminaRegenPct`, `PlayerConvictionRegenPct`, `MobHealthRegenPct`, `MobStaminaRegenPct`, `MobConvictionRegenPct` (default 0.01 = 1% per tick)
- `HealthPerRound()` / `StaminaPerRound()` / `ConvictionPerRound()` compute `floor(poolMax * pct)`, min 1
- **Mutations** use multiplier effects (`health_regen_multiplier`, `health_regen_if_lit_multiplier`, `stamina_regen_multiplier`) — never flat `health_regen` effects
- **Heal spells** store a regen multiplier in `effect_magnitude` (e.g. 3 = 3x base regen); applied via `ConditionRegen`
- **Heal buffs** that heal should compute `floor(poolMax * fraction)` — never flat dice for healing
- NPCs regen health (out of combat), stamina (1/4 in combat), and conviction every tick

## Codegraph MCP — Code Intelligence

The `codegraph` MCP server indexes every Go symbol in the repo into a
local SQLite knowledge graph (~4.6k files, ~18k nodes, ~60k edges).
Sub-millisecond queries return signatures, sources, callers, callees,
and trails. Use it BEFORE writing code, not during.

**Use it for:**
- **Pre-dispatch verification.** Before sending a subagent off, run 2–3
  `codegraph_node` / `codegraph_search` calls to confirm the struct
  shapes, function signatures, and field names the plan references.
  Cheaper than letting a subagent waste turns rediscovering or, worse,
  shipping code against a stale plan. (Caught a real `Engine` field
  rename during 4.2 — plan said `mobTrees`/`noMobTree`, actual is
  `trees`/`noTree`.)
- **Symbol-trail navigation.** `codegraph_node Foo` with `includeCode:true`
  returns the source + callers/callees with file:line. Replaces
  grep + 5–10 Reads.
- **Disambiguation.** Code-base has many `Add` / `Remove` / `Clear` /
  `ClearCache` symbols across packages. `codegraph_node` lists all
  matches and shows the one you asked for, so you don't accidentally
  Read the wrong file.
- **Front-loading subagent prompts.** Paste the verified struct/signature
  into the prompt's "context I've already verified for you" block so
  the subagent skips exploration.

**Don't use it for:**
- File authoring that doesn't reference Go symbols — YAML data files,
  templates, prose docs.
- "Find me the test helper that looks similar to X" — codegraph models
  structure, not similarity. Glob + Read is right for that.
- Confirming code you JUST edited — the index lags ~1s; trust your edit
  + file state over the index for symbols you touched this turn.

**Tool selection by intent (lifted from the codegraph server docs):**
- "What's the deal with this task/feature/area?" → `codegraph_context`
  (composes search + node + callers + callees in one call).
- "What is/calls/triggers this symbol?" → `codegraph_node` (with
  `includeCode:true` for source).
- "Find a symbol by name" → `codegraph_search`.
- "Trace from X to Y" → `codegraph_trace`.

**Subagent guidance.** When dispatching a subagent that needs to touch
unfamiliar code, instruct it to prefer codegraph MCP tools over Read/Grep
for symbol verification — saves their context window and reduces
back-and-forth.

## Package `context.md` Convention

Every package under `internal/` and `modules/` carries a `context.md` — a
developer/agent-facing description of what the package is and how to use it
correctly. **Any work that creates a new package MUST ship one; any work that
reshapes an existing package's API, data model, or file list MUST update it.**
(This rule previously lived only in `docs/roadmaps/MOB_ALIVENESS_ROADMAP.md`,
which is why coverage drifted — 37 packages had none and several documented
functions that did not exist.)

**Verify before you document.** Every symbol you name must exist. Check it with
`codegraph_search` / `codegraph_node`, or extract the real surface with:

```powershell
Select-String -Path internal\<pkg>\*.go -Pattern '^(func|type|const|var)\s'
```

A `context.md` that describes an invented API is worse than no file at all — an
agent will code against it and the mistake surfaces at compile time or, worse,
at runtime.

**Structure** (adapt, don't pad):

- `## Purpose` — what it does and why it exists, 2–4 sentences. Say what it
  deliberately does *not* do.
- `## Files` — one line per file.
- Core types with real field names, in a `go` block.
- `## Public API` — verified signatures, grouped by job.
- `## Gotchas` — the things that bite. Nil-return contracts, panics,
  comparison hazards, ordering requirements, deliberate-looking-wrong code.
- `## Dependencies` and `## Consumers`.

**Do not write** "Future Enhancements," "Security Considerations,"
"Performance Characteristics," "Administrative Features," or "Scalability"
sections unless the package genuinely has something specific to say. The
upstream-generated files are full of that filler and it is being removed, not
copied.

Good exemplars (verified 2026-07-31): `internal/term/context.md` (small,
declarative), `internal/mutators/context.md` (medium, lifecycle-heavy),
`internal/mapper/context.md` (large, multi-subsystem).

## Equipment Slots
Default slots: Weapon, Offhand, Head, Neck, Shoulders, Body, Back, Belt,
Wrist (x2), Gloves, Ring (x2), Legs, Feet, Component Bag.

Mutation-gated slots (Extra Arms mutation, levels 1-4):
- Each level unlocks one ExtraArm + one ExtraWrist slot
- Level 1: Arm 3 + Wrist 3. Level 2: Arm 4 + Wrist 4.
  Level 3: Arm 5 + Wrist 5. Level 4: Arm 6 + Wrist 6.
- Escalating penalties: charisma -28/-42/-56/-70, aggro 1.0/1.5/2.0/2.5x
- Combat hit penalty: +20 per arm beyond offhand

Back slot: Cloaks (stats) or backpacks (weight reduction on backpack
contents). Component Bag slot: Holds crafting materials. `is_component:
true` items auto-route on pickup. `sort` command migrates existing
materials. `bag_capacity` limits items. Weight reduction on component bag
contents (typical 30%).

ItemSpec fields: `is_component` (bool), `weight_reduction` (float64 0-1),
`bag_capacity` (int). New ItemTypes: `wrist`, `back`, `shoulders`,
`componentbag`.

Tail mutation: adds Tail slot, disables Legs slot. `tail` ItemType. Trip
reskins to tailsweep with enhanced damage/knockdown when mutation active.

## Spell Duration System
All spell durations use `calcSpellDuration(baseFolds, skill, willpower)`:
`duration = baseFolds × (10 + wil/20 + skill/2)`. Effect-specific scaling:
shield = full, heal = ÷2, DoT = ÷3.

## Buff/Ward Spell System
- Shield spells scale by `effect_magnitude` (100 = 1.0x baseline).
  Conviction Ward = 75, Chrysalis Cocoon = 125.
- Shield duration: via `calcSpellDuration`. Crits +50% strength.
- Buff statmods `magical_mitigation` and `conviction_mitigation` flow
  through `GetMagicalMitigation()` / `GetConvictionMitigation()`.
- Kick command auto-selects variant: kick (standing), stomp (prone),
  knee (grapple+control). Config: `KickDamagePercent` (0.80),
  `StompDamagePercent` (1.20), `KneeDamagePercent` (1.00).
- Hidden mob detection on room entry: Perception+Search vs Dex+Skullduggery
  opposed roll in `go.go`. Mobs can spawn hidden via `buffids: [9]`.

## Inventory & Item Disambiguation
- **Disambiguation formats:** Players can use `N.item` (diku-style) or `item#N`
  (hash-style) to target a specific item when multiples exist. `all.item` targets
  all matching items (supported by `get` and `drop`).
- **Unified FindItem:** `look` and `identify` search backpack + equipped items as
  a single pool for disambiguation. `dagger#2` can reach a wielded dagger if the
  first match is in backpack. Source is reported ("in your backpack" / "wielded").
- **Inventory stacking:** Display-only. Items with same ItemId + EnchantType +
  EnchantTier + Uses are grouped with `(xN)` count. Storage is unchanged.
- **Carry capacity:** `Strength × Balance.CarryCapacityMultiplier` (default 0.65).
  Displayed as colored encumbrance tiers (light/moderate/heavy/overburdened/crushed),
  never raw numbers. `{enc}` prompt token available.
- **Encumbrance penalties:** Movement stamina 1-5x multiplier when over capacity
  (`go.go`). Combat swings reduced up to 50% when over capacity (`combat_helpers.go`).
- **Multi-buy:** `buy 5 iron ingot` purchases N copies, stops early on insufficient
  funds or carry capacity.
- **Enchanting targeting:** `craft <recipe> <item-name>` targets a specific item.
  Searches both backpack and equipped items. Shows numbered list when ambiguous.

## Caster Weapon Types
Three weapon subtypes designed for spellcasters: `wand`, `sceptre`, `staff`.
Each has a `spell_damage_multiplier` field on ItemSpec that multiplies spell
damage when the weapon is equipped. This is independent of `damage_multiplier`
(melee). Caster weapons use `weapon-combat` skill for melee (same as swords).

| Subtype  | Hands | Melee Mult | Spell Mult | Speed | Parry | Notes |
|----------|-------|-----------|------------|-------|-------|-------|
| wand     | 1     | 0.40      | 1.30       | 1.2   | 2     | Light, fast |
| sceptre  | 1     | 0.55      | 1.25       | 0.9   | 4     | Moderate |
| staff    | 2     | 0.80      | 1.60       | 0.7   | 12    | Defensive, high spell boost |

`spell_damage_multiplier` is applied in `calcSpellDamage()` and
`calcMobSpellDamage()` in `internal/hooks/spell_resolution.go`.

## Alchemy & Potions System
Potions use a witcher-style design with aging, toxicity, and craft-skill scaling.

### Potion Aging
- Five phases: Fresh (1.0x) → Fermented (1.15x) → Peak (1.30x) → Declining (1.30→0.5x) → Spoiled (harmful)
- Thresholds defined per-potion in `aging:` YAML field (ferment/peak/decay/spoil rounds)
- Aging speed = `bottleMultiplier × (1.0 - craftSkill/200)` — higher = faster aging
- `items.GetAgingPhase()` and `items.CalcEffectiveAgingSpeed()` in `internal/items/aging.go`

### Bottle Tiers
| Bottle | ItemID | Aging Multiplier | component_tag |
|--------|--------|-----------------|---------------|
| Clay Flask | 40043 | 3.0x (fastest) | bottle |
| Glass Vial | 40006 | 1.0x (baseline) | bottle |
| Sealed Phial | 40044 | 0.5x | bottle |
| Crystalline Decanter | 40045 | 0.25x (slowest) | bottle |

All share `component_tag: bottle`. Crafting consumes the first match. The bottle's `BottleAgingMultiplier` is stamped on the output item's `BottleMultiplier` field.

### Toxicity
- Each potion has a `toxicity` field (int) on ItemSpec
- `Character.Toxicity` accumulates; decays by `ToxicityDecayPerTick` per regen tick
- `GetToxicityMax() = ToxicityBaseMax + Vitality/ToxicityVitalityScale`
- Threshold penalties via `GetToxicityPenalties()`: regen/Per/Dex penalties at 50/75/90%
- Spoiled potions apply 3x toxicity + nausea debuff (buff 75)

### Craft Skill Scaling
- Duration: `baseDuration × (1.0 + craftSkill/100) × agingPotencyMultiplier`
- Aging speed reduction: skill 30 = 15% slower aging
- Applied in `drink.go` via `AddBuffScaled()`

### Potion Bandolier
- Belt-slot item with `is_bandolier: true` and `bandolier_capacity` field
- Auto-routes potions in `StoreItem()`, consumed first by `drink` (oldest first)
- Removal spills to backpack. Weight reduction applies to contents.
- `Character.PotionItems` slice, displayed in inventory "Potions:" section

### Buff IDs
- 54-60: Pool regen potions (healing salve through elixir of renewal)
- 61-70: Combat/utility potions (ironhide through purging draught)
- 71-74: Progression potions (essence of growth through chrysalis catalyst)
- 75: Spoiled potion nausea debuff
- 76: Purging draught weakness debuff

### Item IDs
- 30036-30056: New potion items
- 40043-40049: New alchemy materials (bottles + forage/drop ingredients)

## Salvage System
Players can break down crafted items (or items with `salvage_returns` on
their ItemSpec) to recover materials. New standalone skill: `salvage`,
primary stat: Perception, progression multiplier 2.0.

### How It Works
- `salvage <item>` starts a multi-round activity (1-5 rounds based on
  ingredient gold value).
- Each ingredient is rolled independently. Chance scales with skill:
  `chance = min + (max - min) * sqrt(skill / softCap)`.
- Config: `SalvageMinChance` (0.15), `SalvageMaxChance` (0.85),
  `SalvageSoftCap` (50).
- Item is always consumed, even if no materials recovered.

### Stations
- Salvage works anywhere; no tool required as of 2026-05-01.
- Skill rank gates yield rate (Perception-based, see formula above).

### ItemSpec Fields
- `salvage_returns`: list of `{item_tag, quantity}` for non-crafted items.
  Every `item_tag` must match a valid `component_tag` on an existing item.
