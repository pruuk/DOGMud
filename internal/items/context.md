# GoMud Game Items System Context

## Overview

The GoMud items system provides a comprehensive item management framework with support for equipment, consumables, weapons, and special objects. It features a dual-layer architecture with immutable item specifications and mutable item instances, supporting enchantments, durability, and complex item behaviors through type-based categorization and attribute systems.

## Architecture

The items system is built around two main components:

### Core Components

**Item Specifications (`ItemSpec`):**
- Immutable blueprint definitions for all item types
- YAML-based storage with automatic loading and validation
- Hierarchical organization by item type and subtype
- Automatic value calculation based on item properties

**Item Instances (`Item`):**
- Mutable runtime instances based on specifications
- UUID-based unique identification for each instance
- Support for enchantments, durability, and temporary modifications
- Blob storage for custom data and scripting integration

**Type System:**
- Primary types (weapon, armor, consumables, etc.) with ID ranges
- Subtypes for specialized behaviors (wearable, usable, throwable, etc.)
- Element types for magical damage and effects
- Weapon classification for combat message selection

**Attack Message System:**
- Dynamic combat message generation based on weapon subtypes
- Intensity-based message selection (miss, weak, normal, heavy, critical)
- Token replacement system for personalized combat text
- Separate messages for attacker, defender, and room observers

## Key Features

### 1. **Hierarchical Item Classification**
- Type-based organization with reserved ID ranges for different categories
- Subtype system for specialized behaviors and interactions
- Element system for magical properties and damage types
- Automatic categorization and validation

### 2. **Instance Management**
- UUID-based unique identification for every item instance
- Temporary data storage for runtime modifications
- Enchantment system with stat bonuses and curse mechanics
- Durability and usage tracking with break chance mechanics

### 3. **Dynamic Item Modification**
- Runtime enchantment system with stat modifications
- Temporary adjective system for visual effects
- Blob storage for custom content and runtime data
- Override specifications for personalized item properties

### 4. **Combat Integration**
- Weapon damage calculation with dice roll systems
- Attack message generation based on weapon type and damage intensity
- Critical hit mechanics with condition application
- Backstab compatibility based on weapon subtype

## Item Types and Categories

### Equipment Types (ID Ranges)
```go
// Weapons: 10000-19999
Weapon ItemType = "weapon"

// Armor: 20000-29999
Head    ItemType = "head"
Neck    ItemType = "neck"
Body    ItemType = "body"
Belt    ItemType = "belt"
Gloves  ItemType = "gloves"
Ring    ItemType = "ring"
Legs    ItemType = "legs"
Feet    ItemType = "feet"
Offhand ItemType = "offhand"
Light   ItemType = "light"   // lighting plan 5a: a carried light; new ones in armor-20000/light/ (torch 20096, hooded lantern 20097), while the older tallow candle 40077 and oil lantern 40038 keep their materials-40000 ids

// Consumables: 30000-39999
Potion     ItemType = "potion"
Food       ItemType = "food"
Drink      ItemType = "drink"
Botanical  ItemType = "botanical"

// Other: 0-9999
Scroll     ItemType = "scroll"
Readable   ItemType = "readable"
Key        ItemType = "key"
Object     ItemType = "object"
Gemstone   ItemType = "gemstone"
Lockpicks  ItemType = "lockpicks"
Grenade    ItemType = "grenade"
Junk       ItemType = "junk"
```

### Item Subtypes
```go
// Behavior Subtypes
Wearable  ItemSubType = "wearable"
Drinkable ItemSubType = "drinkable"
Edible    ItemSubType = "edible"
Usable    ItemSubType = "usable"
Throwable ItemSubType = "throwable"
Mundane   ItemSubType = "mundane"

// Weapon Subtypes (for combat messages)
Generic     ItemSubType = "generic"
Bludgeoning ItemSubType = "bludgeoning"
Cleaving    ItemSubType = "cleaving"
Stabbing    ItemSubType = "stabbing"
Slashing    ItemSubType = "slashing"
Shooting    ItemSubType = "shooting"
Claws       ItemSubType = "claws"
Whipping    ItemSubType = "whipping"
```

### Equipment Slots (Worn)

Equipment slots are a `internal/characters` concept (`characters.Worn`, worn
by `Character.Equipment`); this package defines the `ItemType`s that route
into those slots and the `ItemSpec` fields some slots interpret specially.
`Worn.AllSlots()` (`internal/characters/worn.go`) is the single source of
truth for the slot list + display labels; a reflection guard test fails if a
new `items.Item` field on `Worn` isn't registered there.

**Default slots** (always present): Weapon, Offhand, Head, Neck, Shoulders,
Body, Back, Belt, Wrist ×2, Gloves, Ring ×2, Legs, Feet, Component Bag.

**Mutation-gated slots (Extra Arms).** Each mutation level 1-4 grants one
`ExtraArm` + one `ExtraWrist` slot (level 1 = Arm 3 + Wrist 3, level 2 = Arm 4
+ Wrist 4, level 3 = Arm 5 + Wrist 5, level 4 = Arm 6 + Wrist 6). An
unavailable level moves any equipped item in that slot back to the backpack
rather than deleting it (`Character.validateMutationSlots`).

Cost scales per level as `baseValue × mutations.LevelMultiplier(level)`
(charisma flat penalty base -28, aggro-magnet multiplier base 1.0, from
`_datafiles/world/dogmud/mutations/extra-arms.yaml`). With the **shipped**
`MutationLevel2/3/4Multiplier` (1.6 / 2.5 / 4.0 in `config.yaml`, not the Go
defaults of 1.5 / 2.0 / 2.5), the real per-level numbers are:

| Level | Charisma | Aggro multiplier |
|-------|----------|-------------------|
| 1     | -28      | 1.0x |
| 2     | -44      | 1.6x |
| 3     | -70      | 2.5x |
| 4     | -112     | 4.0x |

Combat hit penalty is a separate, config-independent flat formula in
`internal/combat/combat_helpers.go`: `+20` per weapon index beyond the
offhand (`penalty += (weapIdx - 1) * 20`).

**Two things worth knowing before touching this table:**

- **`extra-arms` itself ships at `max_rank: 1`**, explicitly commented
  `apex-class, binary (no deepen)`
  (`_datafiles/world/dogmud/mutations/extra-arms.yaml`). A mutation without an
  explicit `max_rank` falls back to the global `MutationMaxLevel` (shipped
  `4`), but `extra-arms` is one of 22 mutations in the dataset that opt out of
  deepening entirely. So only the level-1 row above (Arm 3 / Wrist 3) is
  reachable today; the level 2-4 numbers are real code paths
  (`mutations.LevelMultiplier`) exercised by tests, just not currently
  reachable through this mutation.
- **The level-1 aggro multiplier of 1.0x is a no-op.** `GetAggroMagnet` is
  consumed as a direct multiplier (`entries * aggroMagnet`,
  `internal/mobcommands/lookfortrouble.go`), so at level 1 it multiplies by
  1.0 and changes nothing. The "draws hostile attention" flavor text on the
  mutation only becomes mechanically true at level 2+, which is currently
  unreachable (see above).

**Back slot** (`ItemType Back = "back"`): cloaks (stat mods) or backpacks
(`weight_reduction` reduces the effective weight of backpack contents).
**Component Bag**: `is_component: true` items auto-route to it on pickup;
`bag_capacity` caps how many it holds; the `sort` command
(`internal/usercommands/sort.go` → `Character.SortComponentItems()`)
migrates matching items already sitting in the backpack. **Tail mutation**:
adds the `Tail` slot (`ItemType Tail = "tail"`) and disables `Legs` via the
mutation's `disable-legs` flag (`Character.validateMutationSlots`); `trip`
swaps to `tailsweep` (enhanced damage/knockdown) when the mutation is present
(`internal/usercommands/trip.go`).

Relevant `ItemSpec` fields: `is_component` (bool), `weight_reduction`
(float64, 0.0-1.0), `bag_capacity` (int), `is_bandolier` (bool),
`bandolier_capacity` (int, see Alchemy & Potions below). `ItemType`s that
back these slots: `wrist`, `back`, `shoulders`, `componentbag`, `tail`, and
`light` (lighting plan 5a: the carried-light slot; its items shed light through
a `light_strength` worn condition, not a flag).

## Item Specification Structure

### Basic Item Properties
```go
type ItemSpec struct {
    ItemId          int
    Name            string
    DisplayName     string        // Formatted display name with colors
    NameSimple      string        // Simple name for matching
    Description     string
    Value           int           // Gold value (auto-calculated if 0)
    Type            ItemType
    Subtype         ItemSubType

    // Usage Properties
    Uses            int           // Number of uses before consumption
    ConditionIds         []int         // Conditions applied when used
    WornConditionIds     []int         // Conditions applied while worn
    Magnitude            float64       // lighting plan 5c: base strength for a ConditionIds record that reads its magnitude (conditions.ScaledKinds); drink.go scales it by potency, same as duration
    Nouns                map[string]string // lighting plan 5a: lookable details (the hooded lantern's hood); see characters.FindItemNoun
    QuestToken      string        // Quest progress granted when obtained

    // Combat Properties
    Damage              Damage        // Weapon damage specification
    DamageMultiplier    float64       // Weapon/spell damage scaling (0.15–2.5)
    PhysicalMitigation  int           // Physical damage reduction % (armor)
    MagicalMitigation   int           // Magical damage reduction % (enchanted gear)
    ConvictionMitigation int          // Conviction damage reduction % (willpower items)
    WaitRounds          int           // Extra combat rounds required
    Hands               WeaponHands   // 1 or 2 handed weapon
    Element             Element       // Magical element type

    // Ranged Weapon Properties (subtype: shooting)
    AmmoTag     string  // Ammo bundle tag required to reload ("arrows","bolts","shot")
    MinStrength int     // Minimum Strength to wield without penalty (0 = no gate)

    // Enhancement Properties
    StatMods        statmods.StatMods  // Stat modifications when worn
    BreakChance     uint8              // Chance to break on use (0-100)
    Cursed          bool               // Cannot be removed when equipped
    KeyLockId       string             // Lock ID this key opens
}
```

Note: `Loaded bool` lives on the **`Item` instance struct** (not ItemSpec).
`Item.Loaded = true` when a round is chambered/nocked; persists in instance
saves and is cleared on fire.

### Damage System
```go
type Damage struct {
    Attacks     int      // Number of attacks per round
    DiceCount   int      // Number of dice to roll
    SideCount   int      // Sides per die
    BonusDamage int      // Flat damage bonus
    DiceRoll    string   // Formatted dice roll (e.g., "2d6+3")
    CritConditionIds []int    // Conditions applied on critical hits
}
```

### Unified Damage & Mitigation Pipeline (Phase 34)

All damage in DOGMud flows through three channels. Each channel uses:
`raw = stat × SkillMultiplier(rank) × item_multiplier`, then
`final = raw × (1 - min(mitigation%, cap))`, then `dice.RollStat(final)`.

**Weapon `damage_multiplier` (float64):**
- Fists/unarmed: 0.30 (config `UnarmedDamageMultiplier`)
- Crude/improvised: 0.40–0.60
- Basic iron: 0.80–1.00
- Quality steel: 1.10–1.30
- Enchanted/rare: 1.40–1.80
- Legendary: 2.00–2.50

**Armor mitigation fields (integer percentages):**
- `physical_mitigation` — reduces melee/ranged/physical spell damage
- `magical_mitigation` — reduces mind-targeting spell damage
- `conviction_mitigation` — reduces taunt/rhetoric damage

Typical values by armor tier:
| Tier | Physical | Magical | Conviction |
|------|----------|---------|------------|
| Cloth/robes | 1–3% | 5–12% | 2–5% |
| Leather | 4–8% | 2–4% | 0–2% |
| Chain/scale | 8–12% | 1–3% | 0% |
| Plate | 12–18% | 0–2% | 0% |
| Shield (offhand) | 5–10% | 0–2% | 0% |
| Amulet/ring | 0–2% | 3–8% | 3–8% |

All three channels cap at 75% by default (configurable).

**Enchantment effects for the new pipeline:**
- `physical_mitigation_bonus` — adds to physical_mitigation (int)
- `magical_mitigation_bonus` — adds to magical_mitigation (int)
- `conviction_mitigation_bonus` — adds to conviction_mitigation (int)
- `damage_multiplier_bonus` — adds to damage_multiplier (int hundredths: 10 = +0.10)

## Item Instance Management

### Item Creation and Validation
```go
// Create new item instance
func New(itemId int) Item {
    itemSpec := GetItemSpec(itemId)
    newItm := Item{
        UUID:   uuid.New(UUIDItem),
        ItemId: itemId,
        Uses:   itemSpec.Uses,
    }
    newItm.Validate()
    return newItm
}

// Item validation ensures consistency
func (i *Item) Validate() {
    if i.UUID.IsNil() {
        i.UUID = uuid.New(UUIDItem)
    }
    
    iSpec := i.GetSpec()
    if i.Uses == 0 && iSpec.Uses > 0 {
        i.Uses = iSpec.Uses
    }
}
```

### Item Identification and Matching
```go
// Multiple identification methods
func (i *Item) ShorthandId() string {
    return fmt.Sprintf("!%d:%s", i.ItemId, i.UUID.String())
}

// Name matching with partial and full match support
func (i *Item) NameMatch(input string, allowContains bool) (partialMatch bool, fullMatch bool) {
    input = strings.ToLower(input)
    simpleName := strings.ToLower(i.Name())
    
    if allowContains && strings.Contains(simpleName, input) {
        return true, simpleName == input
    }
    
    if strings.HasPrefix(simpleName, input) {
        return true, simpleName == input
    }
    
    return false, false
}
```

### Baubles: one carrier, catalog-backed identity

A bauble (`docs/baubles/implementation-plan.md`) is item 900 plus
`Item.Bauble`, a catalog record id. It carries no `Spec`; instead
`GetSpec()` overlays Name, NameSimple, DisplayName, Description, Value and
Weight from the resolver that `internal/baubles` installs at boot
(`SetBaubleResolver`). This package never imports `internal/baubles`, which
imports it. With no resolver, or an unknown id, the bauble shows as the
plain carrier ("Curious Trinket").

- `SameStack` compares `Bauble`, so baubles never collapse into one row
  (they share an ItemId and have no Spec, which would otherwise stack them).
- `NameMatch` also accepts `bauble` and `trinket` for any bauble.
- **Word matching** (`baubleWordMatch`, baubles only). Model-made names are
  unpredictable, so a bauble answers to any word of its name, or several
  words in order, as a PARTIAL match: "Small Child's Doll" answers to `doll`,
  `childs doll`, `child's doll`, `child doll` and `small doll`; `a`/`the`
  in front are dropped; hyphens split words; `dol` or `sm doll` match too;
  `doll small` (wrong order) is none. A bauble is a FULL match only for its
  exact name or the generic keywords, like any item, so it never outranks a
  real item named in full. `FindMatchIn`, with no `N.` given and a bauble in
  the list, hands off to `findMatchWithBaubles`: the real items are chosen
  among themselves by the list-order rule (`findMatchInOrder`, the rule a
  list with no bauble uses), exactly as if no bauble were there, so a
  bauble never re-orders real items ("Ironwood Staff" before "Iron Dagger":
  `iron` takes the staff with or without an "Ironbound Locket" about). Only
  that real choice is weighed against the best bauble (`strongestBauble`,
  by `matchStrength`: 4 exact, 3 whole words, 2 the start of words, 1
  merely contained). A real item named in full wins outright (a real "Brass
  Lantern" beats a bauble of the same name); a bauble named in full beats a
  real item matched in part; otherwise the real item wins on an equal or
  stronger match: with a "Round Shield" and a bauble "Shield-Maiden's
  Brooch", `get shield` takes the shield (a whole word in both), but with a
  "Buttoned Leather Vest" and a bauble "Tarnished Copper Button", `sell
  button` sells the button (a whole word beats the start of one). A
  household's bauble (`BaubleHousehold` set: taking it is theft) never beats
  a real item on a partial match, so `get candle` takes a "Candlestick" over
  a household "Stub of Candle"; `steal` names household baubles on its own.
  `N.name` above 1 keeps plain list order over every item. `AuthoredKeyword(word)` (in
  itemspec.go) is whether a loaded item answers to a word, as its keyword or
  any word of its name: `internal/baubles` keeps bauble keywords off those.
  `AuthoredName(name)` is whether a loaded item's whole name matches after
  NFKC, lower case and collapsed spaces (`normalizeItemName`);
  `baubles.CleanReply` refuses such a name. Both read one snapshot
  (`authored`, an atomic pointer to words and names together) that every
  writer of the items map rebuilds (`rebuildAuthoredKeywords`: the load, `SaveItemSpec`,
  `DeleteItemSpec`, `CreateNewItemFile`, the test seeders), never the live
  map: it runs on the bauble goroutine, off the mud lock, and a range over
  the map racing a write is a fatal error. Everything that finds items by name
  goes through `NameMatch` via `FindMatchIn`, so `get`, `drop`, `look`,
  `appraise`, `sell`, `give` and `N.name` all behave the same.
- **Finder-only baubles** (`BaubleView.Finder` set: `internal/baubles`
  `Record.KeptToFinder`, a player key's text the server could not
  moderate). Every viewer-agnostic accessor (`GetSpec`, `Name`,
  `DisplayName`, `GetLongDescription`, ...) shows the generic trinket; only
  the viewer-aware ones in `bauble_viewer.go` (`GetSpecFor`,
  `DisplayNameFor`, `NameFor`, `LongDescriptionFor`) show the finder's own
  text, and only to `BaubleView.FinderUserId`; each is the other
  viewer-agnostic accessor's twin, call them only where the output reaches
  that one viewer (the repo-root guard `bauble_finder_view_guard_test.go`
  pins every caller and its call count). `displayNameFrom` and
  `longDescriptionFrom` are the spec-taking bodies of `DisplayName` and
  `GetLongDescription`. Matching is viewer-agnostic,
  so such a bauble matches ONLY by its generic words (`trinket`, `bauble`,
  the generic name), for everyone INCLUDING its finder: a hidden word that
  matched would confirm it to anyone who typed it (`look horse`), and as a
  whole-word match (strength 3) it would beat a real "Horseshoe" the word
  only starts (strength 2), so `get horse` would take the trinket. The
  trade-off: the finder reads "Painted Wooden Horse" but refers to it as a
  trinket. Moderated player-key and server-key baubles have no `Finder` and
  match by their real words as above.
- **Model-safe accessors** (`bauble_model.go`): `ModelName` and
  `ModelDescription` show any bauble whose text a player's own key wrote
  (`BaubleView.PlayerText`), moderated or not, as its carrier's own spec
  ("Curious Trinket"). Anything a language model is told uses these (the AI
  companion's perception, scene and actions).
- A bauble is never enchanted (`enchantments.ApplyTier` returns at once):
  an item with a `Spec` never consults the catalog again, so a baked
  bauble would outlive a retire and the finder-only view.
- `IsSpecial()` is false for a bauble. Any code that rebuilds an item from
  its ItemId alone (`items.New(id)`) drops the link; the sell path has its
  own bauble branch for this reason. Display code that groups items by
  ItemId must also key on `Bauble` (the floor listing in `look.go` does).
- **Placement** (`bauble_placement.go`). A found bauble LEFT LYING (in a
  household, or its finder could not carry it or was gone) carries
  `BaubleSpot` ("on the bookshelf"), `BaubleHousehold` (the owning room's id,
  or 0) and `BaubleLeftAt` (unix seconds). `LeaveBaubleAt` sets them,
  `ClearBaublePlacement` clears them, and `Character.StoreItem` clears them
  on every pickup, so a carried bauble is an ordinary possession: no spot, no
  owner, never vanishes, and dropping it sets nothing. `BaubleBelongsTo(roomId)`
  (taking it there is theft), `BaubleUntakenFor(now)` (rooms' 24-hour sweep)
  and `BaubleSpotSuffix()` (the " (on the bookshelf)" after its name on the
  ground).

## Enchantment and Modification System

The legacy upstream `Item.Enchant` (flat damage/defense/stat bonuses) was
removed 2026-08-03 with the `DamageReduction` field it wrote — the Chrysalis
system in `internal/enchantments` is the only live enchant path. `UnEnchant`
remains (clears `Spec` and `Enchantments`).

```go
// Curse management
func (i *Item) IsCursed() bool {
    return i.GetSpec().Cursed && !i.Uncursed
}

func (i *Item) Uncurse() {
    i.Uncursed = true
}
```

### Adjective System
```go
// Visual effects through adjectives
func (i *Item) SetAdjective(adj string, addToList bool) {
    if i.Adjectives == nil {
        i.Adjectives = []string{}
    }
    
    for idx, a := range i.Adjectives {
        if a == adj {
            if !addToList {
                i.Adjectives = append(i.Adjectives[:idx], i.Adjectives[idx+1:]...)
            }
            return
        }
    }
    
    if addToList {
        i.Adjectives = append(i.Adjectives, adj)
    }
}

// Display name with adjectives
func (i *Item) DisplayName() string {
    name := i.GetSpec().Name
    
    if len(i.Adjectives) > 0 {
        suffix := " <ansi fg=\"black-bold\">(" + strings.Join(i.Adjectives, "|") + ")</ansi>"
        name += suffix
    }
    
    return name
}
```

## Combat Message System

### Attack Message Structure
```go
type WeaponAttackMessageGroup struct {
    OptionId ItemSubType
    Options  AttackTypes
}

type AttackTypes map[Intensity]AttackOptions

type AttackOptions struct {
    Together TogetherMessages  // Same room messages
    Separate SeparateMessages  // Different room messages
}

type TogetherMessages struct {
    ToAttacker MessageOptions  // Messages to attacker
    ToDefender MessageOptions  // Messages to defender
    ToRoom     MessageOptions  // Messages to room observers
}
```

### Defence Message Structure

Defence pools use the same data-loader architecture under
`defense-messages/`, keyed by `DefencePool` (M4b-2). `DefencePool` is a store
key only, not a defence type: that vocabulary now lives in
`internal/combatvocab`, and `DefencePoolFor(combatvocab.Defence)` converts a
defence into the pool that narrates it, one per defence, named after it —
`DefencePoolFor(combatvocab.DefenceDodge)` is the `dodge` pool, and so on for
`DefenceParry`, `DefenceBlock`, `DefenceQuell`, and `DefenceDefy` — so the five
defence files do not move. Five counter-narration pools ride the same loader,
shape, and validator (U6b Task 11, re-keyed by the counters slice) and keep
their own constants because they are not defences: `CounterPoolDodge`,
`CounterPoolParry`, `CounterPoolBlock`, `CounterPoolQuell`, and
`CounterPoolDefy`. `CounterPoolFor(combatvocab.Defence)` names the pool for
the DEFENCE that won the counter, not the attack it answered, so a parry crit
always reads as a parry answered and a block crit as a block answered, with
reinterpreted bands (weak = the counter is turned aside, normal = it lands,
heavy = it crits; `internal/combat` maps outcomes to `(crit, margin)` inputs
accordingly). Every file must provide `weak`, `normal`, and `heavy`; each band
must have equal defender, attacker, and room lists containing at least five
non-empty variants. The dodge and block pools are written attack-agnostic
because either may answer a melee move, a same-room shot or a physical spell;
parry answers melee only, whatever swung, so its lines name the defender's
weapon and never the attacker's; quell answers workings only; defy reads for
a charm as well as a taunt, so no line in it names the attempt.

`RenderDefenseMessage` is the single defence band function for every attack
channel, melee included since M4c: `internal/combat`'s `sendDefenseMessages`
now calls it too, in place of the deleted `items.GetDefenseMessage`, which
banded melee on the defender's own self-relative roll z-score rather than on
how decisive the contest actually was. `RenderDefenseMessage` chooses one
index and applies it to all three audiences before token replacement.
Ordinary defended channel outcomes use Weak below a normalized margin of
`Balance.DefenceBandNormalThreshold` (shipped 0.5) and Normal at or above it.
They never use Heavy, because ordinary defence still allows a partial effect
through. Defensive crits alone use Heavy and may truthfully describe full
negation.

These coordinated pools are authoritative for the actual quell or defy
outcome. Callers pass the resolved defence result and display-ready, actor-aware
identities; they do not add a competing hardcoded outcome line. Private
shortage text is separate and never sent to observers or NPC actors.

### Message Selection and Token Replacement

`GetAttackMessage` bands `pctDamage`, damage dealt as a percentage of that
swing's expected damage against that target (not a share of the target's
health), into one of five authored intensities:

```go
// Get attack message based on damage percentage
func GetAttackMessage(subType ItemSubType, pctDamage int) AttackOptions {
    balance := configs.GetBalanceConfig()
    var intensity Intensity
    if pctDamage >= 101 {
        intensity = Critical
    } else if pctDamage >= int(balance.AttackBandHeavyThresholdPct) {
        intensity = Heavy
    } else if pctDamage >= int(balance.AttackBandNormalThresholdPct) {
        intensity = Normal
    } else if pctDamage >= 1 {
        intensity = Weak
    } else {
        intensity = Miss
    }

    // Get messages for weapon subtype and intensity
    if attackMsgOptions, ok := attackMessages[subType]; ok {
        if messages, ok := attackMsgOptions.Options[intensity]; ok {
            return messages
        }
    }

    // Fall back to generic, but never recurse into itself once already on
    // Generic; the zero value degrades to no message instead of overflowing
    // the stack.
    if subType == Generic {
        return AttackOptions{}
    }
    return GetAttackMessage(Generic, pctDamage)
}
```

The Normal and Heavy cutoffs (shipped at 30 and 75) are `Balance.AttackBandNormalThresholdPct`
and `Balance.AttackBandHeavyThresholdPct`, balance knobs validated as a pair in
`internal/configs`: an inverted or out-of-range pair reverts both to 30 and 75,
and zero is rejected on either. The Critical (101) and Miss (0) boundaries stay
hardcoded in Go on purpose, not as an oversight: `combat.attackMessagePct`
forces a crit to 101 and caps a non-crit at 100, which is what pairs the
crit-worded pool with the `***` banner, so moving either would decouple the
banner from the pool it announces.

```go
// Token replacement is NOT done here. There is one engine, in the core:
// narration.Substitute. Render/RenderTriad substitute as they render, and a
// caller holding a raw line converts its typed token map with TokenStrings
// and calls the core directly.
func TokenStrings(tokens map[TokenName]string) map[string]string
```

`ItemMessage.SetTokenValue` (a per-token `strings.Replace`) was the first of
the messaging arc's three leftover token engines and was deleted in M4a. The
other two, `grapplemessaging.RenderTemplate` and the local `substitute` in
`internal/hooks/Position_Messaging.go`, went the same way in the same slice.
Nothing in the package substitutes tokens by hand any more, and
`token_engine_guard_test.go` at the repo root fails the build if anything
starts again.

## Durability and Usage System

### Break Mechanics
```go
// Break chance testing
func (i *Item) BreakTest(increaseChance ...int) bool {
    bc := i.GetSpec().BreakChance
    if bc < 1 {
        return false
    }
    
    randNum := uint8(util.Rand(100))
    if len(increaseChance) > 0 {
        if uint8(increaseChance[0]) >= randNum {
            randNum = 0
        } else {
            randNum -= uint8(increaseChance[0])
        }
    }
    
    return bc > randNum
}

```

Usage-count decrementing is **not** a method on `Item`; this package only
carries the `Uses` field itself. The consuming logic lives on
`characters.Character` (`internal/characters/inventory.go`):
`Character.UseItem(i items.Item) int` decrements `Uses` on the matching
backpack item, removing it entirely once uses reach zero, and returns the
uses remaining; `Character.UseItemFromPotions` is the bandolier equivalent.

## Alchemy & Potions System

Potions use a witcher-style design: aging replaces flat durability as the
"is this still good" question, and overuse accumulates toxicity instead of
(or alongside) ordinary consumable consequences.

### Aging
Five phases, `AgingPhase` (`internal/items/aging.go`): `PhaseFresh` (1.0x
potency) → `PhaseFermented` (1.15x) → `PhasePeak` (1.30x) → `PhaseDeclining`
(linear 1.30x → 0.5x) → `PhaseSpoiled` (0x, harmful). Thresholds are
per-potion (`AgingThresholds`: `ferment_rounds` / `peak_rounds` /
`decay_rounds` / `spoil_rounds`, authored under the `aging:` YAML field).

```go
func GetAgingPhase(elapsedRounds uint64, thresholds AgingThresholds, effectiveSpeed float64) (AgingPhase, float64)
func CalcEffectiveAgingSpeed(bottleMultiplier float64, craftSkill int) float64
```

`CalcEffectiveAgingSpeed` = `bottleMultiplier × max(0.5, 1.0 - craftSkill/200)`,
so a higher effective speed ages faster (shorter phases); craft skill floors
out at a 50% speed reduction (skill approximately 100+).

### Bottle Tiers
All four bottles share `component_tag: bottle`; crafting consumes whichever
matches first. The chosen bottle's `BottleAgingMultiplier` (`ItemSpec`) is
stamped onto the crafted potion's `BottleMultiplier` (`Item` instance field),
which also feeds inventory stacking (see below: two potions aged in
different bottle types never stack).

| Bottle | ItemID | `bottle_aging_multiplier` |
|--------|--------|---------------------------|
| Clay Flask | 40043 | 3.0 (fastest aging) |
| Glass Vial | 40006 | 1.0 (baseline) |
| Sealed Phial | 40044 | 0.5 |
| Crystalline Decanter | 40045 | 0.25 (slowest aging) |

### Toxicity
Toxicity itself lives on `characters.Character`, not this package:
`internal/items` only carries the `toxicity` (int) field on `ItemSpec` that a
potion drinks into it.

- `Character.Toxicity` (float64) accumulates via `AddToxicity()`, clamped to
  `[0, GetToxicityMax()]`.
- `GetToxicityMax() = ToxicityBaseMax + AlchemySkill/ToxicityAlchemyScale +
  Vitality.ValueAdj/ToxicityVitalityScale`. Tolerance is earned by
  **brewing** (the Alchemy-skill term), not by raw Vitality alone.
  `ToxicityBaseMax` ships at `0` (a real, deliberate value, not "unset").
- Decays by `ToxicityDecayPerTick` per regen tick
  (`internal/hooks/NewRound_AutoHeal.go`).
- `GetToxicityPenalties()` returns `(regenMult, perceptionMult,
  dexterityMult)` at three coarse thresholds: 50% (-10% regen/Per), 75%
  (-20% regen, -10% Per/Dex), 90% (-40% regen, -20% Per, -10% Dex).
  `ToxicityBand()` exposes six finer feedback-only bands that deliberately do
  NOT mirror those thresholds; the two are intentionally desynced so a
  player gets early warning before any penalty bites.
- Drinking a `PhaseSpoiled` potion (`drink.go`) applies `toxicity × 3.0` and
  the nausea condition (condition 75).

### Craft Skill Scaling
`durationMult = potencyMult(agingPhase) × (1.0 + Item.CraftSkill/100.0)`. For
an ordinary potion condition this scales duration only, via
`UserRecord.AddConditionScaled(conditionId, durationMult, source)` in
`drink.go`. **Since lighting plan 5c**, a condition whose spec reads one of
`conditions.ScaledKinds` from its magnitude (`ConditionSpec.ScaledKind`)
instead goes through `PotionMagnitudeApplication` (`potion_conditions.go`,
shared by the player's drink in `internal/usercommands` and a mob's in
`internal/mobcommands`, which passes `durationMult` 1, so an AI companion's
potion lands at its real strength): it reads the new `ItemSpec.Magnitude` field (the
potion's base strength for that scaled kind) and scales both it and the
condition's `TriggerCount` by `durationMult`, capping an infra-reach result at
`configs.Lighting.InfraReachCap`, then queues through
`AddConditionMagnitude` instead. A potion carrying a magnitude-scaled
condition with no `Magnitude` authored is caught by a repo-root guard test
over the shipped world (the build, not the boot — a load-time check would
break every test binary that loads items without conditions), not by
`ItemSpec` validation itself.

### Potion Bandolier
Belt-slot item (`is_bandolier: true`, `bandolier_capacity` int). Auto-routes
potions into `Character.PotionItems` on pickup (`StoreItem`); `drink`
consumes the oldest match first (`FindInPotions`, sorted by `CraftedRound`
ascending). Unequipping the belt spills its contents back to the backpack
(`internal/characters/worn.go`). The bandolier's `weight_reduction` applies
to its contents, the same mechanic the Component Bag uses.

### Condition IDs
54-60 pool-regen potions (healing salve through elixir of renewal), 61-70
combat/utility potions (ironhide through purging draught), 71-74 progression
potions (essence of growth through chrysalis catalyst), 75 spoiled-potion
nausea, 76 purging-draught weakness. Condition 130, Pitsense Tincture
(lighting plan 5c), is a potion condition outside this block: literal
`nightvision_strength: 12`, `infra_reach: magnitude`. See "The purge strips a
derived set" below for how the Purging Draught reaches conditions outside
54-75.

### Item IDs
30036-30056 potion items (`items/consumables-30000/`), 40043-40049 alchemy
materials, bottles (40043-40045) plus forage/drop ingredients
(40046-40049), all under `items/materials-40000/`. Item 30068, Pitsense
Tincture, and material 40233, Heat-Pit Organ (lighting plan 5c), sit outside
both ranges.

### The purge strips a derived set (`potion_conditions.go`, lighting plan 5c)

`PotionEffectConditionIds() map[int]bool` returns every condition id a
`type: potion` `ItemSpec` names in its `ConditionIds`, minus any id also
named by a non-potion item's `ConditionIds` or `WornConditionIds` (a
condition also granted by food or worn gear is not potion-only, so stripping
it on purge would undo the other grant too). It is computed fresh on each
call from `GetAllItemSpecs()`, so an admin item reload cannot leave it stale.
`internal/usercommands.purgeableConditionIds` unions this set with the
original hardcoded 54-75 block (a floor kept so a test binary with no items
loaded still behaves as before) and then removes both detox items' own
conditions and the purge's applied weakness, and the Purging Draught strips
whatever that union contains — closing a leak where several shipped potions
(first conditions 7, 44, 47, 48, 49, 51, 82) had already drifted outside the
old hardcoded block.

## Data Storage and Persistence

### Blob Content System
```go
// Store custom data in items
func (i *Item) SetBlob(blob string) {
    compressed := util.Compress([]byte(blob))
    i.Blob = util.Encode(compressed)
}

func (i *Item) GetBlob() string {
    if len(i.Blob) == 0 {
        return ""
    }
    
    decoded := util.Decode(i.Blob)
    return string(util.Decompress(decoded))
}

// Temporary data storage
func (i *Item) SetTempData(key string, value any) {
    if i.tempDataStore == nil {
        i.tempDataStore = make(map[string]any)
    }
    
    if value == nil {
        delete(i.tempDataStore, key)
        return
    }
    i.tempDataStore[key] = value
}
```

### File Organization
```go
// Automatic file organization by item ID ranges
func (i *ItemSpec) ItemFolder(baseonly ...bool) string {
    if i.ItemId >= 30000 {
        return "consumables-30000"
    } else if i.ItemId >= 20000 {
        if len(baseonly) > 0 && baseonly[0] {
            return "armor-20000"
        } else {
            return "armor-20000/" + string(i.Type)
        }
    } else if i.ItemId >= 10000 {
        return "weapons-10000"
    } else {
        return "other-0"
    }
}
```

## Integration Patterns

### Character Equipment Integration
```go
// Stat modification when equipped
func (i *Item) StatMod(statName ...string) int {
    if i.ItemId < 1 {
        return 0
    }
    
    itemInfo := i.GetSpec()
    return itemInfo.StatMods.Get(statName...)
}

// Equipment comparison
func (i *Item) IsBetterThan(otherItm Item) bool {
    if otherItm.ItemId < 1 {
        return i.ItemId > 0
    }
    return i.GetSpec().Value > otherItm.GetSpec().Value
}
```

### Quest System Integration
```go
// Automatic quest progress when item obtained
type ItemSpec struct {
    QuestToken string  // Quest progress granted when obtained
}

// Quest integration happens through event system
// when ItemOwnership events are fired
```

## Search and Discovery

### Item Finding Functions
```go
// Multiple search methods
func FindItem(nameOrId string) int {
    if itemId, err := strconv.Atoi(nameOrId); err == nil {
        if itm := New(itemId); itm.ItemId != 0 {
            return itm.ItemId
        }
    }
    return FindItemByName(nameOrId)
}

func FindItemByName(name string) int {
    name = strings.ToLower(name)
    
    // Exact match first
    for _, item := range items {
        if strings.ToLower(item.Name) == name {
            return item.ItemId
        }
    }
    
    // Prefix match
    for _, item := range items {
        if strings.HasPrefix(strings.ToLower(item.Name), name) {
            return item.ItemId
        }
    }
    
    // Contains match
    for _, item := range items {
        if strings.Contains(strings.ToLower(item.Name), name) {
            return item.ItemId
        }
    }
    
    return 0
}
```

### Advanced Item Matching
```go
// Find items in collections with numbering support
func FindMatchIn(itemName string, items ...Item) (pMatch Item, fMatch Item) {
    // Support for !itemId:uuid format for exact identification
    if len(itemName) > 1 && itemName[0] == '!' {
        parts := strings.Split(itemName[1:], ":")
        itemIdMatch, _ := strconv.Atoi(parts[0])
        
        var itemUUIDMatch uuid.UUID
        if len(parts) > 1 {
            itemUUIDMatch, _ = uuid.FromString(parts[1])
        }
        
        for _, itm := range items {
            if !itemUUIDMatch.IsNil() && itm.UUID == itemUUIDMatch {
                return itm, itm
            }
            if itemIdMatch > 0 && itm.ItemId == itemIdMatch {
                return itm, itm
            }
        }
    }
    
    // Support for numbered items (e.g., "sword 2" for second sword)
    itemName, itemNumber := util.GetMatchNumber(itemName)
    
    // With a bauble in the list and no N., findMatchWithBaubles (see the
    // Baubles section); otherwise findMatchInOrder, the list-order rule.
    // Returns partial match and full match separately
}
```

`FindMatchIn`'s numbering comes from `util.GetMatchNumber` (`internal/util/util.go`),
the shared parser behind every targeting command:

- `N.item` (diku-style) and `item#N` (hash-style) both target the Nth match.
- `all.item` returns the sentinel `-1`, meaning "every match", consumed by
  `get` and `drop` for bulk operations.
- Plain `item` defaults to the first match (`N = 1`).

### Unified Inventory Lookup

`Character.FindItem` (`internal/characters/inventory.go`) is the single pool
`look` and `identify` search across backpack **and** equipped items, so
`dagger#2` can resolve to a wielded dagger even when the first match sits in
the backpack. The result reports its source string, `"in your backpack"` or
`"wielded"`, for display.

### Inventory Stacking (display-only)

`SameStack(a, b Item)` (`internal/items/stacking.go`) groups display rows for
the `(xN)` count; underlying storage is unaffected. Two items stack only if
**all** of these match: `ItemId`, `Uses`, `EnchantType`, `EnchantTier`,
`BottleMultiplier` (so two potions aged in different bottles never stack),
and either both carry no `Spec` override or the same override pointer (any
enchanted or renamed item effectively never stacks with another).

### Carry Capacity & Encumbrance

`Character.CarryCapacity()` = `Strength.ValueAdj × Balance.CarryCapacityMultiplier
× (1.0 + mutations.GetCarryCapacityMultiplier(...))`
(`internal/characters/inventory.go`). **`CarryCapacityMultiplier` is absent
from the shipped `config.yaml`** and silently defaults to `0.65`
(`internal/configs/config.balance.misc.go`). Per the project's config
convention, an absent key is a real, meaningful value, not "unset".

Encumbrance is shown only as colored tiers, never raw numbers: light,
moderate, heavy, overburdened, crushed (zero or negative capacity reports
"crushed" rather than dividing by zero). A `{enc}` prompt token is available.
Movement stamina cost scales via `internal/costs.EncumbranceMultiplier` up to
`CostEncumbranceMax` (shipped 5.0) as carried weight approaches and exceeds
capacity (`internal/usercommands/go.go`). Combat swing count is reduced up to
50% over capacity (`internal/combat/combat_helpers.go`).

### Multi-buy and Crafting Targeting

`buy 5 iron ingot` (`internal/usercommands/buy.go`) purchases N copies in one
command, stopping early on insufficient funds or carry capacity rather than
failing the whole batch. `craft <recipe> <item-name>` targets a specific item
for enchanting recipes, searching both backpack and equipped items and
showing a numbered list when the name is ambiguous
(`internal/usercommands/craft.go`).

## Performance Considerations

### Memory Management
- Item specifications loaded once at startup and cached
- Item instances use minimal memory with spec references
- Temporary data cleared automatically on item destruction
- UUID generation optimized for performance

### File Loading Optimization
```go
// Batch loading of all item specifications
func LoadDataFiles() {
    start := time.Now()
    
    tmpItems, err := fileloader.LoadAllFlatFiles[int, *ItemSpec](
        string(configs.GetFilePathsConfig().DataFiles) + "/items"
    )
    if err != nil {
        panic(err)
    }
    
    items = tmpItems
    
    // Load attack messages
    tmpAttackMessages, err := fileloader.LoadAllFlatFiles[ItemSubType, *WeaponAttackMessageGroup](
        string(configs.GetFilePathsConfig().DataFiles) + "/combat-messages"
    )
    if err != nil {
        panic(err)
    }
    
    attackMessages = tmpAttackMessages
    
    mudlog.Info("itemspec.LoadDataFiles()", 
        "itemLoadedCount", len(items),
        "attackMessageCount", len(attackMessages),
        "Time Taken", time.Since(start))
}
```

## Dependencies

- `internal/conditions` - Status effect integration for item usage and worn effects
- `internal/configs` - Configuration management for file paths and settings
- `internal/statmods` - Stat modification system for equipment bonuses
- `internal/uuid` - Unique identification system for item instances
- `internal/util` - Utility functions for dice rolls, compression, and string processing
- `internal/colorpatterns` - Color pattern application for display names
- `internal/fileloader` - YAML file loading and validation system

## Usage Examples

### Creating and Modifying Items
```go
// Create new item instance
sword := items.New(12345)

// Enchant the sword
sword.Enchant(5, 0, map[string]int{"strength": 2}, false)

// Add visual effect
sword.SetAdjective("glowing", true)

// Check if cursed
if sword.IsCursed() {
    sword.Uncurse()
}

// Test for breakage
if sword.BreakTest(10) {
    // Item broke with 10% increased chance
}
```

### Item Searching and Matching
```go
// Find item by name or ID
itemId := items.FindItem("steel sword")

// Find in player inventory
inventory := []items.Item{sword, shield, potion}
partial, exact := items.FindMatchIn("sw", inventory...)

if exact.ItemId > 0 {
    // Found exact match
} else if partial.ItemId > 0 {
    // Found partial match
}
```

### Combat Integration
```go
// Get weapon damage
attacks, dCount, dSides, bonus, critConditions := weapon.GetDiceRoll()

// Get attack messages
messages := items.GetAttackMessage(items.Slashing, 85) // 85% damage = Heavy

// Render one COORDINATED triad: every audience from a single variant index,
// tokens substituted by the core as it renders.
roles := messages.Together.Render(skillLevel, map[items.TokenName]string{
    items.TokenDamage: "15",
    items.TokenActee:  "orc",
}, nil)
```

The name tokens are the canonical four (M4a): `TokenActor`/`TokenActee` name
the two participants in every store, and `TokenActorType`/`TokenActeeType`
carry their ansi colour class. `TokenActor` and `TokenActee` are defined FROM
`narration.TokenActor`/`narration.TokenActee`, so the two vocabularies cannot
drift. The old `TokenSource`/`TokenTarget`/`TokenAttacker`/`TokenDefender`
spellings are gone.

This comprehensive item system provides the foundation for all equipment,
consumables, and objects in GoMud, supporting complex interactions,
modifications, and integration with all other game systems.

---

## Item Comparison Utilities (moved to `internal/itemvalue`)

NPC crafting decisions (crafter mobs deciding whether to craft self-gear,
`gearup` upgrade checks) no longer compare items via functions in this
package. That logic now lives in `internal/itemvalue`, as
`itemvalue.ItemValueDelta(char *characters.Character, profile WeightProfile,
candidate items.Item) SwapDelta` and `itemvalue.IsUpgrade(char, profile,
candidate) bool` (sugar for `ItemValueDelta(...).Score > 0`).

See `internal/itemvalue/context.md` for the weighted scoring model. This
package previously documented a simpler `ItemPower`/`IsUpgrade(current,
candidate ItemSpec)` pair; neither exists in the current source.

---

## Rarity Tiers (Stage 3.0e)

Items intended for the living-economy supply pipeline carry a
`RarityTier` field on their ItemSpec. The tier integer doubles as the
**per-vendor MaxStock cap** when scaled by the vendor's `StockMultiplier`:

```yaml
rarity_tier: 50  # caps shop stock at 50 × mob.stock_multiplier
```

Tier semantics for materials (item IDs 40000+):

| Tier | Meaning         | MaxStock @ mult=1.0 | Examples |
|------|-----------------|---------------------|----------|
| 50   | Common          | 50                  | iron ingot, raw meat, wooden plank, glass vial |
| 40   | Standard        | 40                  | steel ingot, leather strip, healer's root |
| 30   | Regional        | 30                  | lake-iron nodule, blood-moss, wild hare meat |
| 20   | Uncommon        | 20                  | gold wire, mutation catalyst, Stillwater pearl |
| 10   | Rare (reserved) | 10                  | future: legendary materials |

Untiered items (no `rarity_tier`) are quest tokens, lore items, or
anything that should not flow through the caravan/forager pipeline.
`shops.EffectiveMaxStock(itemId, mult)` returns 0 for these — callers
fall back to legacy hardcoded values from the shop YAML.

The mob pointer is intentionally not taken inside `EffectiveMaxStock`
to avoid an import cycle (mobs imports shops). Callers pass
`mob.StockMultiplier` directly as a `float64`.

## Pricing Bands (Stage 3.4)

The `value` field on ItemSpec is the **base gold price**. Final shop
prices are dynamic — `internal/shops/pricing.go` computes a scarcity
multiplier that swings 0.25x (overstocked) to 5.0x (out of stock):

```
ratio = current / restock_qty
ratio >= 3.0          →  0.25x  (PriceFloor)
ratio == 0            →  5.0x   (PriceCeiling)
otherwise             →  0.25 + 4.75 × (1 - ratio/3)²
```

Because the multiplier already encodes scarcity, base values shouldn't
double-encode rarity — they define a sensible midpoint inside which
dynamic pricing operates.

Recommended base-value bands by tier (Approach B, applied 2026-04-30):

| Tier | Band       | Notes |
|------|------------|-------|
| 50   | 1–3g       | Utility outliers allowed (sealed phial 10g, crystalline decanter 30g — bottles whose aging multiplier is the value) |
| 40   | 5–25g      | Raw forage 5g, refined commons 8g, drops/alchemy 12g, premium 25g |
| 30   | 25–75g     | Forage 25–30g, processed 35–50g, drops 50–60g |
| 20   | 80–500g    | Common drops/refined 80–100g, quest specialties 400–500g |

Effective shop sell price = `ceil(value × ScarcityMultiplier(current, restockQty))`.
Player buy-back from NPCs applies `BuyRatio` (default 0.50) on top.

## Supply Pipeline: Caravan & Foragers

Materials enter vendor stock via two NPC-driven supply systems:

**Caravan (Stages 3.0–3.4):** A wagon-as-mob with two handlers (Hob,
Bran) runs a depot loop. At each stop,
`caravan.VisitVendorsInRoom(roomId, wagon, deliveryBuckets, pickupBuckets)`
performs a bidirectional vendor stop: deliver wagon items whose bucket
matches `deliveryBuckets`, then pick up vendor surplus matching
`pickupBuckets`. Pickup is gated by `entry.Current >= entry.MaxStock/2`
(don't extract from a starving vendor). Stock changes persist
immediately via `shops.SaveShop` so a panic mid-cycle doesn't lose
in-flight deliveries.

**Foragers (Stage 3.1):** Territory-bound forager NPCs (Halix in
Fernway South, Kessa in Stillwater Marsh) gather mats from their region
and deliver to the local depot. A behavior-tree state machine drives
five states: resting → traveling → foraging → delivering → recalling.
Recall uses fold-recall, gated on `mob.Character.IsCasting()` so the
cast doesn't reset every idle tick.

**Supply buckets** (see `internal/economy/buckets.go`) group items by
source territory:

| Bucket       | Source                           |
|--------------|----------------------------------|
| `base`       | universal craft staples (iron, leather, thread) |
| `stillwater` | Stillwater Marsh forage (lake-iron, blood-moss) |
| `fernway`    | Fernway South forage (moonpetal, ironbark) |
| `thornwall`  | Thornwall regional drops |
| `overlap`    | items appearing in multiple territories |

Each shop YAML stock entry implicitly inherits its bucket from
`economy.BucketFor(itemId)`. Caravan and forager route configurations
list which buckets to deliver vs. pick up at each stop.

**`mobs.Mob.StockMultiplier`** lets an exceptionally large or small
vendor scale all MaxStock caps. Default 1.0 means MaxStock = RarityTier
exactly. Set this on big trading-post mobs (e.g., 2.0 for a major
depot) or small specialty stalls (e.g., 0.5 for a single-recipe niche
vendor).

---

## Weapon Reach (chunk 4c)

`ItemSpec.Reach` (float64, meters) encodes how much physical space a weapon
needs to operate at full effectiveness. The combat pipeline consults reach
when the attacker is grappling — weapons whose reach exceeds the position's
effective radius are penalized multiplicatively, while weapons that fit the
radius pay no penalty. A greatsword in mount swings awkwardly (the haft
catches the attacker's own body); a dagger in mount stays fully dangerous.
The attack-message vocabulary swaps to a bludgeoning set when the penalty
fires (pommel/hilt strike narration). Full design spec and formula:
`docs/superpowers/specs/completed/2026-05-16-state-chunk-4c-position-weapon-utility-design.md`

**Consumer side:** `internal/combat/reach.go` (T2) — `PositionReachRadius`,
`ReachUtility`, `ShouldBludgeon`, `CalcReachAdjustedItemMult`.

### Default reach by subtype

| Subtype     | Reach (m) | Notes                                    |
|-------------|-----------|------------------------------------------|
| Fist        | 0.10      | Punch length                             |
| Claws       | 0.15      | Fingers extended                         |
| Bite        | 0.15      | Head-mounted, neck-length                |
| Sting       | 0.20      | Abdomen/tail-tip                         |
| Slam        | 0.30      | Bull rush, shoulder-check                |
| Stabbing    | 0.30      | Dagger / shiv family                     |
| Gore        | 0.40      | Horn-tip from skull base                 |
| Wand        | 0.40      | Foot-long focus                          |
| Whipping    | 0.50      | Hand-held whip; mob tail overrides       |
| Sceptre     | 0.60      | Larger ornamented focus                  |
| Bludgeoning | 0.80      | Mace / hammer family                     |
| Cleaving    | 0.90      | Axe family                               |
| Slashing    | 1.00      | Sword family                             |
| Shooting    | 1.00      | Bow/crossbow as club; override compacts  |
| Staff       | 1.50      | Quarterstaff equivalent in close quarters|

**`Bludgeoning` wears two hats.** It's both a weapon carry-subtype (set in
YAML on real mace/hammer items like `steel_warhammer.yaml`, with reach 0.8 m
by default) AND the destination subtype for the bludgeon narration swap in
`internal/combat/combat_helpers.go:buildAttackMessages` (bladed weapons in
grapples render with Bludgeoning vocabulary). The two uses don't conflict —
the swap targets the message-rendering subtype only; the underlying weapon's
carry-subtype is unchanged.

Natural-attack subtypes (Fist through Whipping) stay at or below the default
ground-grapple radius (0.3 m), so they pay no penalty — by design.

### Authoring guidance

1. **Leave `reach` empty** in YAML for normal items of a known subtype.
   The engine falls through to the subtype default via `ResolveReach`.
2. **Set `reach: <meters>` only for outliers** — e.g., an unusually long
   dagger (`reach: 0.5`) or a compact crossbow (`reach: 0.6`).
3. **Use meters**, not abstract units. Real-world references help balance.
4. **New subtypes:** add a `case` in `DefaultReachForSubtype` in
   `internal/items/reach.go` and update this table.
5. Arm length / species reach is intentionally out of scope (chunk 4c
   decision). Weapon reach only.

### Natural-attack subtypes: live for mob basic attacks

The natural-attack `ItemSubType`s — `bite`, `claws`, `slam`, `gore`,
and `sting` — and their combat-message files are now the standard
rendering path for non-human mob melee (Phase 1 non-human attack
messaging). When a species YAML sets `natural_attack:`, `buildWeaponSetup`
in `internal/combat/combat_helpers.go` routes unarmed mob attacks through
that subtype's message pool. Previously these subtypes were defined and
used only for reach accounting or special-case weapon items; they are now
actively selected at runtime for basic attacks on ~30 tagged species.

## Caster Weapon Types

Three weapon subtypes (`ItemSubType`s `wand`, `sceptre`, `staff`) are
purpose-built for spellcasters. Each carries `spell_damage_multiplier` on
`ItemSpec`, independent of the melee `damage_multiplier`, and uses the
`weapon-combat` skill for its (weak) melee output, same as any other
one/two-handed weapon.

| Subtype | Hands | Melee `damage_multiplier` | `spell_damage_multiplier` | `speedmultiplier` | `parryrating` | Notes |
|---------|-------|---------------------------|----------------------------|--------------------|----------------|-------|
| wand    | 1     | 0.40 | 1.30 | 1.2 | 2  | Light, fast |
| sceptre | 1     | 0.55 | 1.25 | 0.9 | 4  | Moderate |
| staff   | 2     | 0.80 | 1.60 | 0.7 | 12 | Defensive, high spell boost |

(Table values are the baseline template items:
`10016-willow_wand.yaml`, `10017-iron_sceptre.yaml`, `10018-oak_staff.yaml`,
not a hardcoded rule; an individual item's fields can differ.)

**`spell_damage_multiplier` is read, not applied, by this package.** The
consuming logic lives in `internal/hooks`:

- `calcSpellDamageForCharacter` (`internal/hooks/combat_shared_helpers.go`) is
  the single unified caster/mob spell-damage function (Stage 38.1), called
  from every direct spell-damage site in `internal/hooks/spell_resolution.go`.
- `spellTickScale` (`internal/hooks/spell_tick_scale.go`), which
  `applySpellCondition` uses for a tick-pool condition, reads the same
  multiplier separately, since that path doesn't route through
  `calcSpellDamageForCharacter`.

A prior version of this documentation named `calcSpellDamage()` and
`calcMobSpellDamage()` as two separate functions in `spell_resolution.go`.
Neither name exists in the current source; the caster and mob damage paths
were unified into `calcSpellDamageForCharacter` before that doc was written.

## U10d Ranged Detune (`detune_migration.go`)

The eight `shooting` templates were rescaled downward in U10d. Editing a
template does not reach items that already exist -- `Item.GetSpec()` returns
`Item.Spec` (persisted as `overrides:`) whenever non-nil and never consults the
template -- so `MigrateDetunedBow` rescales live instances on every user load.

Two things to know before touching it:

- **It multiplies by `newTemplate/oldTemplate`, never assigns the template
  value.** Assigning would delete affix scaling the player bought with instance
  gold, which is the exact bug `SpecBaseline` exists to prevent.
- **`Item.DetuneMigrated` is the authority, not the item's value.** `New()`
  stamps it for the eight detuned ids only. A value-only test has a live false
  positive: an instanced Relic Sidearm (10049) affixed past its old 6.00 would
  be rescaled and lose 63%. The `>= old` value check is only a fallback for
  saves written before the stamp existed.

Adding a new `shooting` weapon means adding it to `preDetuneBowMultipliers` (or
deliberately leaving it out) -- `TestRangedWeaponMultipliers_MatchTheU10dTable`
and `TestPreDetuneBowTable_MatchesTheRealTemplates` both fail otherwise.

## Files

| File | Purpose |
|------|---------|
| `itemspec.go` | The authored `ItemSpec` |
| `items.go` | The `Item` instance and its behaviour |
| `save.go` | Persistence |
| `validation.go` | Spec validation |
| `newitemfile.go` | New-item scaffolding |
| `stacking.go` | Display-only inventory stacking |
| `aging.go` | Potion aging phases and effective aging speed |
| `potion_conditions.go` | `PotionEffectConditionIds` (lighting plan 5c): the condition ids only a potion grants, for the Purging Draught's derived strip set; `PotionMagnitudeApplication`, the magnitude and trigger count a potion applies a scaled condition at, shared by the player and mob drink paths |
| `affixgen.go` | Affix/name generation |
| `bauble.go` | Baubles: `BaubleItemId` (900), `BaubleView`, `SetBaubleResolver`, `IsBauble`; the `GetSpec` overlay that reads a bauble's name, description, value and weight from the catalog in `internal/baubles` |
| `bauble_placement.go` | Where a found bauble lies: `LeaveBaubleAt`, `ClearBaublePlacement`, `BaubleBelongsTo`, `BaubleUntakenFor`, `BaubleSpotSuffix` |
| `spec_baseline.go` | `SpecBaseline`: pre-enchant numeric snapshot, so a tier re-apply cannot wipe affix scaling |
| `detune_migration.go` | U10d ranged-weapon rescale (`MigrateDetunedBow`); idempotent by value threshold, no run-once marker |
| `reach.go` | Weapon reach data |
| `attack_messages.go` / `defensive_messages.go` | Combat message pools. Both render a coordinated triad through `narration.Render`; see below |
| `memory.go` | Memory reporting |
| `test_helpers.go` / `test_helpers_combat.go` | Test fixtures |

Item ids at 40000+ live under `items/materials-40000/` — `Filepath()` routes by
id range, so a materials item filed elsewhere will not load.

## The combat-message store renders one coordinated index

`attack_messages.go` owns `combat-messages/`. Its surface:

```go
func (stm SkillTieredMessages) PoolFor(skillLevel int) []string
func (m TogetherMessages) Render(skillLevel int, tokens map[TokenName]string, pick narration.Picker) narration.Roles
func (m SeparateMessages) Render(skillLevel int, tokens map[TokenName]string, pick narration.Picker) narration.Roles
```

`PoolFor` assembles the cumulative tier union (beginner always, plus expert at
skill 34, plus master at 67) and hands it to the core as plain strings.
Assembly stays here; the core only coordinates the index and substitutes
tokens.

Role mapping, and getting it wrong inverts every combat message in the game:

| Split | Actor | Actee | Observer | ActeeObserver |
|---|---|---|---|---|
| `together` | `ToAttacker` | `ToDefender` | `ToRoom` | *(absent, one observer audience)* |
| `separate` | `ToAttacker` | `ToDefender` | `ToAttackerRoom` | `ToDefenderRoom` |

Those are the GO FIELD names, and they lag the YAML. M4b-1 renamed the authored
keys to the canonical role vocabulary, so a data file now says `actor`,
`actee`, `observer` and, in the `separate` split only, `remote_observer`.
`toattacker`/`todefender`/`toroom`/`toattackerroom`/`todefenderroom` are the
old spellings and no longer load anything. `toattackerroom` became plain
`observer` because it is the same audience as `together`'s room line: the
observers standing with the actor. The same rename covers
`defense-messages/` (`DefenseTogetherMessages`) and `taunt-messages/`
(`combat.TauntMessages`).

**Every audience comes from ONE index.** Picking per role narrates a different
moment to each of them, which shipped twice before (melee defence PR #112,
taunt PR #115) and was live in this store until M3 item 8.

**`Validate` enforces per-tier role equality, and that is not the same as
equal totals.** The union is cumulative, so equal totals with unequal tiers
still pair an expert line against a master one at the same index. Per-tier
equality is exactly equivalent to union equality at all three skill levels.
`minVariants` is 1, not the defence store's 5: 414 of 534 shipped groups hold
fewer than five, and the smallest holds one.

`tools/combat_message_pool_audit.py` reports any group whose pools are
unequal; `tools/combat_message_pad_check.py` proves a content change deleted
and edited nothing while permitting reordering.

**`ConsistentAttackMessages` is gone and must not come back.** It seeded the
index with the weapon's `ItemId`, which worked upstream where pools are equal
but collapsed each pool to one line per intensity per skill tier here.
`consistent_attack_messages_guard_test.go` fails the build if the name
returns.

## Item walkers

`WalkSlice(s []Item, fn func(*Item))` (walk.go) calls fn with a pointer
into s for each item with ItemId above zero. Every store's `WalkItems` is
built on it: `characters.Character` and `Worn`, `users.UserRecord`,
`rooms.Room`, `mobs.Mob`, `shops.ShopInventory`, `guilds.Guild`,
`sealedcrate.Crate`, `modules/auctions.AuctionManager`. The bauble catalog
sweep (`internal/baubles/sweep.go`) reads every live item through them.

## Item behaviour: `behavior:`, fixtures and the holder index (lighting 5e)

- **`ItemSpec.Behavior`** (`behavior:`) names a tree under
  `behaviors/items/` (see `internal/behaviortree/context.md`). It belongs to
  the template: read it through `Item.HasBehavior()` or `GetItemSpec`, never
  an instance's override spec. The boot refuses a name that does not resolve.
- **`ItemSpec.Fixture`** (`fixture: light | darkness`, constants
  `FixtureLight` and `FixtureDarkness`, checked by `Validate`) fixes an item
  to a room's floor. `Item.IsFixture()` reads the template. Every
  floor-removal path refuses a fixture (`actions.ErrFixture`, the `get all`
  and mob sweeps, `steal`, the mob idle floor equip), "On the Ground" and the
  GMCP room contents leave it out, and its tree's output lights the room
  through `internal/itemlight`. Ship a fixture `not_salable`: it is never
  loot.
- **The holder index** (`item_behavior.go`) holds HOLDERS, not items: mob
  instances (`IndexMobHolder`, `DropMobHolder`, `MobHolders`) and rooms
  (`IndexRoomHolder`, `DropRoomHolder`, `RoomHolders`) that may hold a treed
  item. The item tick (`hooks.ItemRoundTick`) visits only these plus every
  online player. `OnRoomHolderIndexed`, set by `internal/hooks`, is called
  once each time a room enters the index, outside the lock, so its fixtures
  are lit before anyone reads the room. `ResetHolderIndexForTest` swaps in an
  empty index.
