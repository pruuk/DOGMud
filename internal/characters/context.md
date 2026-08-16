# DOGMud Characters Package Context

## Overview
The `internal/characters` package is the core character system for DOGMud, handling both player characters (PCs) and non-player characters (NPCs/mobs). It provides a comprehensive character model with stats, equipment, skills, combat mechanics, and various character states.

**DOGMud Differences from upstream GoMud:**
- Level system disabled — progression is skill/stat-use-based
- Mana removed — spells use Conviction resource pool
- Three resource pools: Health, Stamina, Conviction
- Six stats renamed: Strength, Dexterity, Perception, Vitality, Willpower, Charisma
- Species system replaces races (all players are Human)
- 10 core DOG skills + 15 legacy GoMud skills coexist

## Key Components

### Core Character Structure (`character.go`)
- **Character struct**: The main character entity containing all character data
- **Character creation and management**: Factory functions and lifecycle management
- **Stat calculations**: Dynamic stat computation with buffs, equipment, and species modifiers
- **Skill-based progression**: Skills and stats improve through use (`progression.go`)
- **Persistence**: Character data serialization/deserialization

### Character Statistics System
- **Six core stats**: Strength, Dexterity, Perception, Vitality, Willpower, Charisma
- **Stat scaling**: Stats over 100 use `SQRT(overage)*2` formula for diminishing returns
- **Dynamic modifiers**: Equipment, buffs, pets, and mutations affect final stats
- **Use-based improvement**: Stats improve organically through gameplay

**Gear-effectiveness integration (chunk 2.2a):** `Character.StatMod()` multiplies
the Equipment portion of `Mods` by `mutations.GearEffectivenessMultiplier(c.Mutations)`
before summing with Buffs and Pet contributions. This cascades through `RecalculateStats()`
into all downstream consumers (stat values, mitigation, recovery, skills, spells).

### Skill System (`progression.go`)
- **Use-based progression**: Skills improve through gameplay use, not training points
- **Exponential decay curve**: ~50% chance at rank 0, ~2.5% at soft cap (rank 50)
- **Skill aliasing**: `skillNameMap` supports mapping legacy skill names to DOG equivalents
- **16 core DOG skills**: combat (weapon-combat, unarmed-combat, ranged-combat, spellcasting, rhetoric) + non-combat (skullduggery, search, bartering, blacksmithing, alchemy, tailoring, cooking, jewelcrafting, enchanting, salvage, manifestation)
- **15 legacy GoMud skills**: Still functional alongside DOG skills
- **Combat skill routing**: `GetCombatSkillTag()` selects weapon-appropriate skill:
  weapon → `weapon-combat`, unarmed/fists → `unarmed-combat`,
  ranged (subtype `shooting`) → `ranged-combat`

### Difficulty-Scaled Progression
`OnSkillUseScaled(skillName, userId, bonusMultiplier)` accepts a difficulty
bonus that flows into `CheckSkillProgression`. `OnSkillUse` delegates with
1.0 for backwards compatibility. Spell resolution passes
`1.0 + difficulty * SpellDifficultyProgressionScale`, craft completion passes
`1.0 + skillMinimum * CraftDifficultyProgressionScale`.

### Regen-Based Stat Progression
Every regen tick (every 3 rounds), each resource pool has a small chance to
trigger stat progression based on how depleted it is. This replaced the old
hard 25%-threshold `OnLowResource` system.

**Formula:** `chance = RegenProgressionBase × (1 - current/max) ^ RegenProgressionCurve`

**Config knobs:** `RegenProgressionBase` (default 0.01), `RegenProgressionCurve` (default 3.0)

**Resource → Stat Mappings:**
- Health → Vitality, Willpower (enduring injury toughens body + mind)
- Stamina → Strength, Vitality (exertion builds power + endurance)
- Conviction → Willpower, Charisma (mental strain sharpens will + presence)

The existing `StatProgressionMultipliers` still apply on top.
Mob progression uses `MobProgressionRate` as a multiplier.

**Key methods:**
- `OnRegenTick(current, max, relatedStats, userId)` — computes chance, calls CheckRegenProgression per stat
- `CheckRegenProgression(statName, userId, chance)` — applies mob gating, multipliers, rolls

### Equipment System (`worn.go`)
- **Equipment slots**: Weapon, Offhand, Head, Neck, Body, Belt, Gloves, Ring, Legs, Feet
- **Stat modifications**: Equipment provides stat bonuses aggregated across all slots
- **Item management**: Worn item tracking and validation

### Character States and Modifiers
- **Aggro system** (`aggro.go`): Combat targeting and threat management
- **Buffs integration**: Status effects that modify character capabilities
- **Cooldowns** (`cooldowns.go`): Time-based ability restrictions
- **Prone system** (Stage 7.5): Knockdown condition with stat-based recovery mechanics

### Resource Pools
- **Health**: Physical hitpoints, based on Vitality
- **Stamina**: Physical endurance, based on Vitality (used for movement and combat actions)
- **Conviction**: Mental/magical resource, based on Willpower + Charisma (used for spells)
- Mana has been removed entirely

#### Pool mutation API

`internal/characters/pools.go` (chunk U5a) holds the primitives every pool
mutation routes through. U5a added them; U5b-1 routed the call sites and deleted
the hand-rolled clamp that used to sit beside each one. Direct writes to
`c.Health` / `c.Stamina` / `c.Conviction` are now the exception and are guarded
(see Gotchas).

```go
// Pool identifies one of the three resource pools. Deliberately a string,
// matching the vocabulary already used by GetPoolReservation and
// BuffSpec.TickPool.
type Pool string

const (
	PoolHealth     Pool = "health"
	PoolStamina    Pool = "stamina"
	PoolConviction Pool = "conviction"
)

// CostResult reports what a partial cost charge actually did.
type CostResult struct {
	Charged int  // amount actually taken from the pool
	Short   bool // the actor could not pay in full
}

func (c *Character) PoolValue(p Pool) int
func (c *Character) CanAfford(pool Pool, amount int) bool

// EffectivePoolMax is poolMax - GetPoolReservation, floored at 1 (U7 Task 11).
// The denominator for every percentage-OF-MAX threshold. NEVER for affordability.
// It never returns 0, deliberately: see the Gotchas note on the floor.
func (c *Character) EffectivePoolMax(p Pool) int

func (c *Character) ApplyCost(pool Pool, amount int) bool
func (c *Character) ApplyCostPartial(pool Pool, amount int) CostResult

// ApplyCostFloat charges a FRACTIONAL cost, banking the sub-integer remainder
// in the per-character, per-pool carry so the average converges (U7 Task 3).
// Delegates the deduction to ApplyCostPartial. THE ENTRY POINT FOR EVERY U7
// COST; the integer pair erases the per-action modifiers. See Gotchas.
func (c *Character) ApplyCostFloat(pool Pool, amount float64) CostResult

func (c *Character) ApplyHarm(pool Pool, amount int, source state.ActorRef) int
func (c *Character) ApplyRestore(pool Pool, amount int) int

// DisplayHealth returns Health clamped at 0 for player-facing output (U5b-2).
// The model stores overkill; the wire must not show it.
func (c *Character) DisplayHealth() int
```

`DisplayHealth()` is the only sanctioned clamp. U5b-2 routed every display
surface through it: the nine reads in `internal/users/userrecord.prompt.go`,
`modules/gmcp/gmcp.Char.go` (own vitals and the enemy list),
`modules/playtest/beacons.go`, and the `healthStr` helper in
`internal/templates/templatesfunctions.go`. `renderVitalBar`,
`targetHealthDesc` and `modules/gmcp/gmcp.Party.go` already clamp internally and
were deliberately left alone.

`source` is `internal/state.ActorRef`. Unexported support: `poolMax`, `setPool`,
and `applyVitalChange` (the single signed pipeline behind harm and restore).
`setPool` is unexported on purpose so no caller can bypass the floor rules.

#### Gotchas

- **Three floor rules, and they are not symmetric.** A cost never drives a pool
  below 0. Harm floors stamina and conviction at 0. Harm does **not** floor
  health. The health rule exists to preserve overkill magnitude for margin-scaled
  work and because `validatePoolClamps` carries an explicit "No lower Health
  clamp" comment -- **not** because death detection reads the negative value,
  which it does not (every gate tests `< 1` or `<= 0`).
- **Health stores overkill; stamina and conviction do not.** `ApplyHarm` floors
  stamina and conviction at 0 and deliberately does NOT floor health, so a
  killing blow leaves a negative value that U6 reads for magnitude.
  `validate.go` carries a matching explicit "No lower Health clamp". Clamping
  belongs at the display layer: call `Character.DisplayHealth()`, never re-add a
  floor here. As of U5b-2 all seven remaining per-site floors are gone, so this
  is uniform.
- **`ApplyCost` vs `ApplyCostPartial` is not a style choice.** Refuse where a
  meaningful alternative action remains (movement, stand, spellcasting, mutation
  special moves); charge partially where refusal would leave the actor helpless
  (auto-attack, dodge/parry/block, grapple upkeep, flee). The split is NOT
  volitional-vs-involuntary and NOT "uses a cooldown"; both framings were tried
  and both are provably false. Death from exhaustion was tried in this game and
  players hated it.
- **A green pool-mutation guard does not mean every pool write is routed.**
  `resources.go` is exempt as a FILE, so `Heal()`'s writes are invisible and so
  are its three production callers (`actions/combat_drain.go:126`, `:281`,
  `hooks/item_procs.go:99`). They retire with `Heal` in U5c.
- **`CostResult.Short` is what a later chunk reads to strip the skill term.**
  The penalty for being short is a worse roll, not a lost action.
- **`ApplyCostFloat` banks a fractional remainder, and the bank is the reason
  U7's tuning is visible at all.** Every U7 cost is
  `base x encumbrance x inverse-skill x per-action modifier`, so it is almost
  never a whole number, and the pools are ints. Round each action and the small
  factors vanish: dodge, parry and block collapse onto the SAME integer for a
  low-skill character at every base this game would ship, which makes the
  modifiers decoration. The bank is `costCarry map[Pool]float64`, an UNEXPORTED
  field on `Character` (`character.go`), per pool and per character. Each call
  adds the amount to the carry, charges `floor` of the total through
  `ApplyCostPartial`, and leaves the fraction behind. Cumulative charged is
  therefore `floor(cumulative amount)`: it under-charges by strictly less than
  one over any run of actions, and that bound does not grow.

  **What resets it:** nothing in the game does, deliberately, and nothing needs
  to. It is **NOT persisted** (an in-flight fraction is worth less than the byte
  it would cost in a save file, and a stale one after a reload would be
  indistinguishable from a rounding bug), so it starts empty on every load,
  spawn and relog. There is no `yaml:"-"` tag on purpose: an unexported field is
  already invisible to the marshaller, and a yaml tag on one is a silent no-op
  that misleads the next reader.

  **Two contracts that look wrong and are not:**

  - A charge that floors to zero is **not `Short`**, even on an empty pool.
    `Short` means "the actor could not pay what this action demanded", and a
    floored charge of zero demanded nothing; the whole amount went into the
    bank. Returning true would penalise a free action, and would then penalise
    the SAME fraction again on the later action that floors it to one or more.
  - Once the carry hands a whole number to the pool, the full floored amount is
    removed from the carry **even when the pool was too empty to cover it**, and
    the unpaid part is written off rather than re-banked. Otherwise an exhausted
    actor accumulates unbounded debt and is slammed with the backlog on the
    first tick their pool refills. Being short already costs the skill term; it
    must not also become a loan.

  **A non-finite amount is free and banks nothing.** NaN and infinity are
  reachable here because a cost is a product of four config-sourced floats, and
  letting one through poisons that pool's carry PERMANENTLY: NaN survives the
  floor, every later charge floors to NaN, `int(NaN)` converts to the minimum
  int64, and `ApplyCostPartial` reads that as non-positive and charges nothing.
  One bad value makes the pool cost-free for the rest of the session with no
  log, no panic and no failing test. Note `amount <= 0` alone does NOT catch NaN
  (NaN fails every comparison), which is why the guard tests it explicitly.

  **Template invariant.** `mobs.newMobByIdInternal` shallow-copies the mob
  template (`mob := *m`) and re-makes `PlayerDamage` on the next line precisely
  because a shallow copy shares maps. `costCarry` is NOT re-made there, and is
  safe only because it is lazily allocated by `ApplyCostFloat` and a template's
  `Character` is never charged. Anything that charges a template (a balance
  preview tool, an offline simulator) would allocate the map ON THE TEMPLATE and
  hand every instance spawned afterwards the same shared carry. Re-make it
  alongside `PlayerDamage` before doing that.
- **`CanAfford` reads the RAW pool, not reserve-excluded, and that is correct.**
  `RecalculateStats` already clamps the CURRENT pool to `max - reserve` every
  round, so a cost that subtracted the reservation a second time would charge the
  reserve twice. A companion or enchantment holder should **have less, not pay
  more**. The original 2026-08-12 U7 spec contained exactly that double
  subtraction and it was deleted for this reason; do not re-add it.
- **`EffectivePoolMax` is the denominator for percentage-OF-MAX thresholds, and
  ONLY for those.** A threshold taken off the raw max is compared against a
  reserve-clamped current value, which is the same double charge from the other
  direction. That was a live bug: `stand` demanded `StandMinStamina` (0.15) of
  the RAW max, so a 30%-reserved character was asked for 21.4% of the pool they
  could actually fill, and past **85% reservation** the gate demanded more
  stamina than the pool could ever hold -- a permanent lockout reported as
  exhaustion, which resting cannot fix. U7 Task 11 routed `stand` and every
  `combat.ResourceMultiplier` denominator through it. The refusal message now
  discloses reservation in a descriptive band (`reserveShareBand` in
  `internal/usercommands/assess.go`), never a raw number.
- **`EffectivePoolMax` is floored at 1, NOT at 0, and it never returns 0.**
  Total reservation is reachable (stacked Chrysalis enchantments; a two-handed
  item doubles its reserve share). Every consumer treats a non-positive max as
  "no penalty at all" and bails to the neutral answer -- `ResourceMultiplier`
  returns `1.0`, `IsLowGrappleStamina` returns `false`,
  `grappleStaminaMultiplier` returns `1.0` -- so a floor of 0 gave a character
  with a permanently EMPTY pool full swing count, full hit chance and full melee
  damage, the exact inversion of the pre-U7 behaviour. A floor of 1 makes that
  character compute ratio `0/1 = 0` and take the MAXIMUM depletion penalty. It
  matches the pool-max clamp `validatePoolClamps` already applies
  (`validate.go:135-137`). The `if eff <= 0` guards at the call sites are
  therefore dead code, kept as belt and braces.
  One consequence is intended and must not be "fixed": `stand` computes
  `int(1 * StandMinStamina) = 0`, so a fully reserved character stands for free.
  There is no stamina left to charge, and refusing would recreate the permanent
  floor-lockout Task 11 removed.
- **Regen deliberately still reads the RAW max.** `HealthPerRound`,
  `StaminaPerRound` and `ConvictionPerRound` in `resources.go` are the named
  exception: making them reserve-aware is a NERF to reserved characters, and the
  faster refill relative to the usable pool is what offsets the depletion penalty
  they carry. Each carries a comment saying so. It is not drift.
- **Harm and restore are one signed pipeline** (`applyVitalChange`) behind two
  positive-only wrappers. Sign inversion is this codebase's signature failure
  mode; the wrappers exist so no call site can get the direction wrong.
- **Both return the APPLIED delta**, which differs from the requested amount when
  a floor or ceiling bites. A caller keeping a result struct in sync must add the
  return value.
- **`ApplyHarm`'s source is not universally available.** Direct combat, spell and
  maneuver sites have an actor; damage-over-time, toxicity and attrition sites do
  not, because `buffs.Buff` has no applier field. Those pass the zero value.
- **`Heal()` is a HARM path at two call sites.** `buffs.ComputeTickAmount`
  returns a negative value for `TickPercent < 0`. Do NOT make `Heal` a thin
  wrapper over `ApplyRestore` -- `ApplyRestore` no-ops on non-positive input, so
  that would silently delete every health damage-over-time buff. U5b-1 split the
  two signed call sites; U5c retires `Heal`.
- **`ApplyHealthChange` is a wrapper, not a legacy path.** It owns the
  `CancelCombatBuffs` on crossing below zero, which reaches `Validate(true)` and
  a full stat recalculation, and 8 melee call sites depend on it. `ApplyHarm`
  deliberately does not do this. Do not add new callers, and do not "simplify" it
  into `ApplyHarm`.
- **Direct pool writes are guarded.** `pool_mutation_guard_test.go` at the repo
  root fails any production assignment to `.Health`/`.Stamina`/`.Conviction`
  outside five declared exemption classes: the primitives themselves, the clamp
  layer, construction/spawn, admin commands, and a test fixture that compiles
  into the binary. A temporary sixth block holds the sites U5b-2 and U5c still
  owe. Add a file there only with a written reason; if you cannot write one, you
  want a primitive.
- **No direct pool mutation emits an event**, and neither primitive does. The two
  indirect emitters (`ApplyHealthChange` via `Validate`, and `Life_Cascades`'
  respawn set) are deliberate.
- **ActionPoints is a fourth pool and is NOT in `Pool`.** It is an inherited
  GoMud movement throttle, redundant with stamina movement costs, and a deletion
  candidate. Movement is a two-pool transaction with a hand-rolled refund.
- **`DeductStamina` and `DeductDefenseStamina` no longer exist.** U5b-2 deleted
  both. `flee` and the defence charge now call `ApplyCostPartial` directly, and
  movement (`usercommands/go.go`) calls `ApplyCost`. U7 Task 7 then deleted the
  last two, `GetAttackStaminaCost` and `DeductAttackStamina`: the attacker's
  cost was charged ONCE PER ROUND by the four combat wrappers however many
  swings the round contained, while the defender paid on every incoming swing.
  Attacks are priced per swing now by `combat.ChargeAttackCost`, through the
  same `costs.Calc` composition the defences use. `DeductActionPoints` is a
  different pool entirely (see the ActionPoints note above).
- **Defence costs are one config formula, not per-defence Go arithmetic.** U7
  Task 6 deleted `GetDefenseStaminaCost` and with it the three per-defence base
  knobs (`DodgeBaseStaminaCost`, `ParryBaseStaminaCost`,
  `BlockBaseStaminaCost`). All five defences now price through `costs.Calc`:

  - dodge / parry / block: `DefenceBaseStaminaCost` × encumbrance ×
    inverse-skill × `{Dodge,Parry,Block}CostModifier` (1.25 / 1.10 / 1.15).
  - quell / defy: `QuellBaseConvictionCost` / `DefyBaseConvictionCost` ×
    inverse-skill, modifier a neutral 1.0. **No encumbrance term** — their
    `costs.ActionQuell` / `ActionDefy` registry rows are `Physical: false`, and
    that row is the only thing keeping it off them.

  Every action with a governing skill takes the inverse-skill discount, mental
  and social included: quell is governed by spellcasting and defy by rhetoric.
- **Movement is priced by the same formula.** U7 Task 8 put
  `GetMovementStaminaCost` on `costs.Calc`: `MovementBaseStaminaCost` × terrain ×
  encumbrance × inverse-skill (governing skill `search`, from the
  `costs.ActionMove` registry row), then the mutation speed modifier, the hidden
  multiplier, the `MovementMaxStaminaCost` cap and a `MovementCostFloor` floor,
  in that order. The encumbrance term it replaced was written inline here and
  was flat 1.0 until the actor **exceeded** carry capacity, so it priced nothing
  for anyone not deliberately overloaded. The base drops to **0.5** to pay for
  the curve now charging from the first pound: ordinary travel gets slightly
  cheaper, travel at capacity markedly dearer. Terrain rides inside `Base`
  because `Calc` clamps the product of the actor-derived multipliers and terrain
  is a property of the move, not the actor; the clamp is inert for movement
  either way (5.0 × 1.10 = 5.5 against a 6.0 ceiling) and
  `MovementMaxStaminaCost` is the real cap.
- **`EncumbranceTier(carried, capacity float64) (label, color string)`** is the
  ONE place carried weight is turned into something a player sees. It is a
  package-level function, not a method, because callers already hold the two
  floats and some of them (the `encumbranceQuality` template func) do not hold a
  `*Character` at all. Two consumers today: the `inventory` command and the
  `status` sheet. It lives here rather than beside either of them precisely
  because it has two: a second copy of the thresholds would drift, and the drift
  would be invisible, since both copies would render a plausible word, just not
  the same word for the same load. It returns a WORD and never a number, and a
  capacity of `<= 0` reports `crushed` (correct reading, and it keeps the
  division safe). Now that weight prices every physical action, this word is a
  balance readout, so it is under the no-hard-numbers rule.
- **Charge a defence through the PAIR: `DefensePool` + `GetDefenseCostFloat`.**
  There are FIVE defence constants. `DefenseDodge` / `DefenseParry` /
  `DefenseBlock` cost stamina; U6 added `DefenseQuell` (mental-spell defence,
  `Willpower + spellcasting × SkillWeight`) and `DefenseDefy` (social defence,
  `Willpower + rhetoric × SkillWeight`), and both cost **conviction**. Grepping
  for a stamina cost on either finds nothing and proves nothing.

  ```go
  func DefensePool(defenseType string) Pool                      // quell/defy -> PoolConviction
  func (c *Character) GetDefenseCostFloat(defenseType string) float64
  func (c *Character) GetDefenseCost(defenseType string) int     // tests only; see below
  func (c *Character) ApplyCostFloat(pool Pool, amount float64) CostResult
  ```

  **Use the FLOAT pair.** `GetDefenseCost` truncates, and at the shipped base and
  modifiers all three physical defences floor to the same `1` — the per-defence
  tuning simply vanishes. That was a live bug: `combat.ResolveChannelDefence`
  charged through the integer entry point until U7, so blocking a
  `target_defense_type: physical` spell (eleven shipped spells set it) cost 1
  instead of 1.2604 and was indistinguishable from dodging. `ApplyCostFloat`
  banks the sub-integer remainder, so the difference survives as an average. No
  production caller of `GetDefenseCost` remains.

  The pairing matters independently: pool and amount must be read off the SAME
  defence name. An unrecognised name maps to `PoolStamina` at cost 0, so the pair
  charges nothing rather than draining an arbitrary pool.
- **`GetDefenseSequence` is melee-only and does not know about quell or defy.**
  It derives dodge/parry/block from equipment. The per-channel defence set lives
  in `combat.DefenceSetFor` instead.

### Combat and Interaction Systems
- **Kill/Death statistics** (`kdstats.go`): PvP and PvE combat tracking
- **Charm system** (`charminfo.go`): Mind control and pet mechanics
- **Mob mastery** (`mobmastery.go`): Character proficiency with specific creature types
- **Shop system** (`shop.go`): NPC merchant capabilities with restocking mechanics

### Character Presentation
- **Formatted names** (`formattedname.go`): Rich text rendering with adjectives and color coding
- **Adjectives system**: Visual indicators for character states (sleeping, charmed, poisoned, prone, etc.)
- **Quest indicators**: Visual markers for quest-relevant NPCs

## Stage 7.5: Prone Condition System

### Prone State Fields
The prone condition is tracked via three fields in the `Character` struct:

```go
Prone                    bool   `yaml:"-"`  // Currently knocked down
ProneRoundsRemaining     int    `yaml:"-"`  // Minimum prone duration counter
RecoveryPenaltyThisRound bool   `yaml:"-"`  // Limits attacks to 1 during recovery attempt
```

**Field Descriptions:**
- `Prone`: Boolean flag indicating character is knocked to the ground
- `ProneRoundsRemaining`: Countdown for minimum prone duration (set to 2 when knocked down)
  - Must reach 0 before auto-recovery attempts begin
  - Decremented each round in combat hook processing
- `RecoveryPenaltyThisRound`: Flag set during failed recovery attempts
  - Reduces character's attacks to 1 for the current round
  - Represents struggling to stand while fighting
  - Cleared at end of each round tick

### Prone Adjective Display
The `GetAdjectives()` method in `character.go` includes "prone" when `c.Prone == true`:

```go
func (c *Character) GetAdjectives() []string {
    retAdjectives := []string{}

    if c.Health < 1 {
        retAdjectives = append(retAdjectives, `downed`)
    }
    if c.Prone {
        retAdjectives = append(retAdjectives, `prone`)
    }
    // ... other adjectives
}
```

This makes prone status visible in character descriptions and room listings.

### Automatic Recovery System
The `AttemptRecovery(statValue int)` method implements stat-based recovery with logarithmic scaling:

```go
func (c *Character) AttemptRecovery(statValue int) (bool, bool) {
    // Returns: (attemptMade, success)

    if !c.Prone {
        return false, false  // Not prone, no recovery needed
    }

    if c.ProneRoundsRemaining > 0 {
        c.ProneRoundsRemaining--
        c.RecoveryPenaltyThisRound = true
        return false, false  // Still in minimum duration, no messages
    }

    // Calculate recovery chance: min(90, 25 + 20 × ln(stat/25))
    chance := 25.0 + 20.0*math.Log(float64(statValue)/25.0)
    if chance > 90.0 {
        chance = 90.0  // Cap at 90% to keep some uncertainty
    }

    roll := dice.Roll(50, 15.0)
    success := roll.Value < chance

    if success {
        c.Prone = false
        c.ProneRoundsRemaining = 0
    } else {
        c.RecoveryPenaltyThisRound = true
    }

    return true, success  // Attempt made, return success status
}
```

**Recovery Formula Rationale:**
- Logarithmic scaling provides smooth progression without overpowering high stats
- 25 stat (low) = 25% chance, 100 stat (average) = 53%, 300 stat (high) = 75%
- 90% cap maintains tactical uncertainty even at extreme stats
- Generic `statValue` parameter allows future use for other conditions (grapple uses Strength, etc.)

**Integration with Combat Hooks:**
Called in `NewRound_UserRoundTick` and `NewRound_MobRoundTick`:

```go
// After cooldown ticks, attempt recovery if prone
if attemptMade, success := user.Character.AttemptRecovery(user.Character.Stats.Dexterity.ValueAdj); attemptMade {
    if success {
        user.SendText("You scramble to your feet!")
        room.SendText("<user> clambers to their feet in a rushed panic.", user.UserId)
    } else {
        user.SendText("You attempt to stand, but slip back down!")
        room.SendText("<user> attempts to stand, but slips and falls.", user.UserId)
    }
}

// Clear recovery penalty flag at end of round
user.Character.RecoveryPenaltyThisRound = false
```

### Cooldown System Usage
The cooldown system (`cooldowns.go`) is used for special combat moves:

**Special Move Cooldown:**
- Key: `"combat-special"`
- Duration: 5 rounds (config: `SpecialMoveCooldown`)
- Shared across bash, trip, and kick commands

**Usage Pattern in Commands:**
```go
// Check cooldown before executing special move
if !user.Character.Cooldowns.Try("combat-special", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
    user.SendText(fmt.Sprintf("You can't use special moves yet! (%d rounds remaining)",
        user.Character.Cooldowns.Get("combat-special")))
    return true, nil
}

// Execute special move...
```

**Cooldown Mechanics:**
- Stored in `Character.Cooldowns` map (map[string]int)
- Auto-decremented via `RoundTick()` called in combat hooks
- `Try(key, period)` checks if cooldown expired and resets if action performed
- `Get(key)` returns remaining rounds for display purposes

## Mitigation System (Three Channels)

The character package provides three mitigation getter methods that compute
total damage reduction across all equipped items and modifications:

**Three Methods:**
- `GetPhysicalMitigation()` — defends against physical damage
- `GetMagicalMitigation()` — defends against spells
- `GetConvictionMitigation()` — defends against taunt/conviction damage

**Gear-effectiveness integration (chunk 2.2a):** Each method separates
gear-derived contributions (equipment slot mitigation) from non-gear
contributions (natural armor from mutations, species baseline, shield spell
magnitude, buff stat mods). The gear portion is multiplied by
`mutations.GearEffectivenessMultiplier(c.Mutations)` before summing.

**Slot coverage:** All 25 equipment slots are included in the three
mitigation getters, completed during chunk 2.2a:
- Physical mitigation: Shoulders, Back, Wrist1/2, Ring, Ring2, ExtraWrist1-4,
  ExtraArm3-4, ComponentBag (all physical-type armor items).
- Magical mitigation: same slots (all items can carry magical mitigation).
- Conviction mitigation: same slots.

This ensures characters with many-armed mutations or high-value jewelry can
leverage their full equipment potential for defense.

## Intrinsic Mutations (chunk 2.5)

`Character.ApplyIntrinsicMutations(species *species.Species)` merges
the species's intrinsic mutations additively into `Character.Mutations`.
No-op on nil species or empty intrinsic map. Cap-aware via
`MutationMaxRank = 4` (matches chunk-2.2a convention; no per-mutation
max field exists today).

Called once at character init AFTER all other mutation logic:
1. Curated SpawnMutations from mob YAML (mob spawn only)
2. Random-roll mutation acquisition (mob spawn + player round tick)
3. Persistent acquired mutations from save file (players only)
4. `ApplyIntrinsicMutations(species)` — this call

Stacks ADDITIVELY: a wolf species with `intrinsic_mutations: { tail: 1 }`
that also rolls `tail` rank 1 ends up with effective rank 2 in
`Character.Mutations`.

File: `internal/characters/intrinsic.go`

Design: `docs/superpowers/specs/2026-05-12-mob-aliveness-2.5-mutations-on-mobs-design.md`

## Key Features

### Character Persistence
- YAML-based character data storage
- Automatic saving with configurable intervals
- Character creation timestamps and history tracking
- Room history for movement tracking

### Dynamic Stat System
- Base stats from species definitions
- Equipment stat modifications
- Buff/debuff effects
- Use-based stat improvement through gameplay
- Calculated maximums for Health, Stamina, and Conviction

### Social and Economic Systems
- Gold and banking system
- Player shops and merchant NPCs
- Clan membership support
- Pet ownership and management
- Quest progress tracking

## Shop Inventory Decoupling (Living Economy)

Merchant NPCs separate trade inventory from character inventory:

- **`ShopInventory`** (in `internal/shops/`) is the live trade state — stock
  levels, dynamic prices, NPC gold for transactions, restock timers. This is
  what `buy`/`sell` commands interact with.
- **`Character.Shop`** (the legacy `[]ShopItem` slice) remains as template /
  seed data and a fallback for non-migrated merchants. It is NOT the live
  inventory.
- **`Character.Gold`** is the NPC's personal gold (loot on death). NPC gold
  for trade transactions is tracked in `ShopInventory.Gold`, not here.
- **`Character.Items`** (backpack) is NOT used for merchant trade stock.
  Crafter mobs do use the backpack transiently to hold raw materials between
  restock and craft, but finished goods go directly into `ShopInventory`.

When reading or writing merchant code, always distinguish between these three
gold/inventory sources to avoid double-counting or routing items to the
wrong pool.

## Combat Phase Machine Integration (chunk 0)

### New field: CombatPhase

```go
CombatPhase *combatphase.Machine `yaml:"-"`
```

Initialized in `New()` and lazily in `Validate()` (for characters loaded
from YAML without a direct `New()` path). `RegisterMachine` is called
immediately after allocation so inbound-attacker tracking is active from
the first combat action.

### New flag: NonCombatant

```go
NonCombatant bool `yaml:"non_combatant,omitempty"`
```

`true` = character is immune to combat targeting (shops, quest-givers,
etc.). Set from `Mob.NonCombatant` during `Mob.Validate()` for mob
characters; set directly in player creation for any exempt player
archetype.

```go
func (c *Character) IsCombatant() bool { return !c.NonCombatant }
```

The `RegisterCombatantVeto` wiring in `CombatPhase_Vetoes.go` calls this
to block `TransitionToEngaging` for non-combatants.

### Internal guard: combatPhaseWired

```go
combatPhaseWired bool `yaml:"-"`
```

Set to `true` the first time `fireCharacterCreated` runs. The `Validate()`
path checks this flag to avoid double-firing `OnCharacterCreated` callbacks
when `Validate()` is called multiple times during a character's lifetime.

### Predicate methods

All read from `CombatPhase` exclusively; they do not read the legacy
`Aggro` field.

```go
func (c *Character) IsEngaged() bool
    // true when Combat Phase == Engaged (actively fighting)

func (c *Character) IsInCombat() bool
    // true when Combat Phase != Idle (any non-idle combat state)

func (c *Character) IsDisengaging() bool
    // true when Combat Phase == Disengaging (flee in progress)

func (c *Character) EngagedTarget() state.ActorRef
    // current target when Engaged; zero when not Engaged

func (c *Character) CurrentCombatTarget() state.ActorRef
    // current target across all non-Idle states (Engaging/Engaged/Disengaging)

func (c *Character) Attackers() []state.ActorRef
    // snapshot of inbound attacker list from CombatPhase
```

### Legacy Aggro field (compat surface)

The `Aggro *Aggro` field is kept in `combat_state_compat.go` for the
~200 direct field reads in usercommands, hooks, combat, and mob-commands
that were not migrated in chunk 0. **Do not add new reads against
`Character.Aggro`** — use the predicate methods above.

All writes go through `SetAggro` / `EndAggro`, which dual-write to both
`Aggro` and `CombatPhase.TransitionToEngaging` / `ForceIdle`. Direct
mutation of `Character.Aggro` (bypassing the wrappers) is forbidden.

Field removal is scheduled for a cleanup chunk after chunks 1-5 land and
the remaining reads are migrated.

### OnCharacterCreated callback registry

```go
func OnCharacterCreated(fn func(*Character))
```

Registers a callback that fires once per `Character` the first time it
is fully initialized (after `New()` or on first `Validate()` if loaded
from YAML). Used by the hooks package to wire state-machine vetoes and
observers without creating an import cycle (characters cannot import
hooks; hooks import characters).

Current registrations (all in `internal/hooks/`):
- `wireCombatPhaseVetoes` — wires the seven veto closures
- `wireCombatPhaseBtreeEvents` — wires the btree transition cascade
- `wireCompanionAssist` — subscribes to Attackers-change events

## Awareness Machine Integration (chunk 1)

### New field: Awareness

```go
Awareness *awareness.Machine `yaml:"-"`
```

Initialized in `New()` and lazily in `Validate()` (for characters loaded
from YAML without a direct `New()` path). The awareness machine tracks
whether a character is currently hidden and coordinates state transitions
for sneak attempts, detection, and revealing. It operates independently
of Combat Phase but cascades through the same hook framework.

### New predicate: IsHidden()

```go
func (c *Character) IsHidden() bool
    // true when Awareness == Hidden
    // replacement for the old HasBuffFlag(buffs.Hidden) pattern
```

The only canonical way to check if a character is hidden. It reads directly
from the Awareness machine's state, not from buff #9 (which is now a
side-effect carrier only).

### Cascade pattern: Awareness to Buff #9

The `Awareness_Cascades.go` hook ensures buff #9 ("Hidden" status effect)
stays synchronized with the Awareness machine:

- When Awareness transitions to `Hidden` state, the hook applies buff #9
  to the character (providing stat mods and room broadcast text).
- When Awareness transitions away from `Hidden`, the hook removes buff #9.

This maintains backward compatibility with systems that check for buff #9
while keeping the Awareness machine as the canonical state source.

### Hidden movement stamina scaling

When a character is `Hidden`, movement stamina cost is multiplied by
`HiddenMoveStaminaMultiplier` (default and shipped value both 3.0). This
is read in `GetMovementStaminaCost()` and applied after the shared cost
composition and before the cap and floor.

### Integration with Combat Phase

The Awareness machine subscribes to Combat Phase's `OnEndOfRoundIfSurprise`
callback (wired in `Awareness_Cascades.go`). When a surprise engagement
completes its first round of swings, the Awareness machine triggers a
reveal cascade (`Hidden → Revealing → Visible`), forcing surprise-attacked
sneakers out of hiding. The full cascade completes before the next round
begins, ensuring surprised attackers are visible for retaliation.

### Logout cleanup

The `Logout_AwarenessCleanup.go` hook calls `ForceVisible()` on logout,
ensuring the awareness machine doesn't leak state or block future character
reuses (edge case safety).

## Attributed death (U5c)

`ApplyHarm` does not just move a pool. When `pool == PoolHealth` and the change
drives health below 1 on a live character, it sets `DeathQueued` and queues an
`events.CharacterDied` carrying the killer and the overkill magnitude. The death
itself is resolved by `hooks.RouteAttributedDeath`, never here.

**Why queued rather than killed inline:** `Die` fires its observers
synchronously, and `Death_MobInstanceCleanup` despawns the mob inside that call.
Killing at the harm site would remove instances from under any loop damaging
several targets — `usercommands.Throw`'s AoE loop is a live example.

**`DeathQueued` is NOT "dying".** Two states, and conflating them breaks the
engine's death backstops:

| State | Test | Used by |
|---|---|---|
| dying | `Health < 1 && IsAlive()` | combat targeting, coup de grace rendering |
| death queued | `DeathQueued` | the backstop sweeps, which skip on THIS and never on health |

A character reaped by a sweep is dying but **not** queued: it reached zero
without going through `ApplyHarm`. A sweep skipping on "dying" would skip
exactly the population it exists to reap.

`DeathQueued` also makes the killing blow fire exactly once. A second lethal blow
the same round still lands and still counts toward the damage map, but it does
not re-queue and does not re-attribute.

**`ApplyHealthChange` takes a source and it is required.** It wraps `ApplyHarm`,
and all eight of `combat.go`'s damage sites go through it, so a wrapper that
supplied an empty ref would make every melee death anonymous. A zero ref is
still correct for genuinely sourceless harm; it just cannot be imposed by the
wrapper.

**`Die`'s idempotence is mob-only.** Mobs stay at `Dead`, so the `!IsAlive()`
guard stops a second call. Players cascade `Dead → Respawning → Alive` and are
alive again when `Die` returns, so a second call re-runs the entire death
cascade. See `die.go`.

## Life Machine Integration (chunk 2)

### New field: Life

```go
Life *life.Machine `yaml:"-"`
```

Initialized in `New()` and lazily in `Validate()` (for characters
loaded from YAML without a direct `New()` path). `RegisterMachine`
is called immediately after allocation. The Life machine is the
canonical source of truth for "is this character alive?".

### Predicate methods

```go
func (c *Character) IsAlive() bool
    // true when Life == Alive

func (c *Character) IsDead() bool
    // true when Life == Dead

func (c *Character) IsRespawning() bool
    // true when Life == Respawning (player only)
```

Note: these predicates call through to the Life machine. Tests that
exercise code paths gated by these predicates must initialize the
Life machine (via `Validate()` or direct `NewMachine()` assignment)
or the call will panic on a nil pointer.

### Die helper (die.go)

```go
func (c *Character) Die(killer state.ActorRef, trigger string)
```

Chains all Life transitions in the correct order. Players complete
all three states (`Dead → Respawning → Alive`) same-tick via
synchronous `AfterTransition` observer chains. Mobs only transition
to `Dead`; the instance-cleanup observer fires synchronously and
despawns the mob.

Callers MUST pre-check before calling `Die`:
1. `ReviveOnDeath` buff (prevents death; callers bail early if set)
2. `LastSuicideRound` dedupe (if the call site can double-fire)
3. Shadow Realm zone guard (player call sites only)

`Die` is idempotent: if the Life machine is already `Dead` or
`Respawning` it returns immediately without firing observers.

### ResolveRespawnRoom (respawn_home.go)

```go
func (c *Character) ResolveRespawnRoom() int
```

Reads the player's `"home"` setting, looks it up in
`HomeLocations`, and falls back to `"default"` (Sanctum Basin
entrance, room 0) if unset or unrecognized.

`HomeLocations` maps setting key → room ID. `HomeLocationNames`
maps setting key → display string. Both are exported maps consumed
by `sethome.go` (key validation) and by `Respawn_PlayerTeleport.go`
(destination resolution).

Current entries:

| Key | Room ID | Display Name |
|-----|---------|--------------|
| `"default"` | 0 | Sanctum Basin |
| `"thornwall"` | 468 | Thornwall City (Temple Interior) |
| `"stillwater"` | 4123 | Stillwater (Temple of Stillwater) |

### MobInstanceId field

```go
MobInstanceId int `yaml:"-"`
```

Non-persisted field set to the mob's live `InstanceId` at
character initialization. Used as a cheap gating check in Life
machine observers (`c.MobInstanceId != 0` = mob) without requiring
a cast or registry lookup.

### OnCharacterCreated additions (chunk 2)

The `OnCharacterCreated` registry gains Life-machine wire callbacks.
New registrations (all in `internal/hooks/`):
- `wireLifeMachine` — registers the Life machine and all Death +
  Respawn observer chains

## Activity Machine Integration (chunk 3)

### New field: Activity

```go
Activity *activity.Machine `yaml:"-"`
```

Initialized in `New()` and nil-guarded in `Validate()` (for characters
loaded from YAML without a direct `New()` path). The Activity machine
is the canonical source of truth for "what multi-round action is this
character locked into right now?"

### Predicate methods

```go
func (c *Character) IsFree() bool
    // true when Activity == Free (no activity in flight)

func (c *Character) IsCasting() bool
    // true when Activity == Casting
    // replaces the old c.CastingState != nil check

func (c *Character) IsCrafting() bool
    // true when Activity == Crafting
    // replaces the old c.CraftingState != nil check

func (c *Character) IsSalvaging() bool
    // true when Activity == Salvaging

func (c *Character) IsActing() bool
    // true when Activity != Free (any non-Free state)
    // canonical "is busy" gate replacing the old IsCrafting() gate
    // at special-moves check sites (13 call sites rewired in chunk 3)
```

`IsActing()` is preferred for "should this action be blocked because
the character is busy?" checks. Use the specific predicates only when
you need to distinguish which activity is running (e.g., the craft
command's own re-entrancy check).

### OnCharacterCreated additions (chunk 3)

The `OnCharacterCreated` registry gains the Activity machine wire
callback. New registration (in `internal/hooks/`):
- `wireActivityCrossMachineCascades` — subscribes `activity_life_dead`
  observer to the Life machine; wires the Activity machine's identity
  via `RegisterMachine`.

### Sunset notes (chunk 3)

The following fields and files were deleted in chunk 3:
- `Character.CastingState *characters.CastingState` field
- `Character.CraftingState *characters.CraftingState` field
- `internal/characters/casting.go` — `CastingState` struct
- `internal/characters/crafting.go` — `CraftingState` struct
- `CraftingState.MiscData["salvage_item_uuid"]` key pattern

All call sites that read `c.CastingState != nil` or
`c.CraftingState != nil` were migrated to `IsCasting()` / `IsCrafting()`
/ `IsSalvaging()` / `IsFree()` / `IsActing()` predicates.

## Position Machine Integration (chunk 4a — scaffold)

### New field: Position

```go
Position *position.Machine `yaml:"-"`
```

Initialized in `New()` and nil-guarded in `Validate()` (for characters
loaded from YAML without a direct `New()` path). The Position machine
is the sole source of truth for body geometry and grapple state. Chunk
4a scaffolded the machine; **chunk 4b completed the full cutover**:
all production writers (W1-W8), all readers (R1-R6 including R4 Life
cascade pre-wire deletion), and the legacy field sunsets (S1-S5) have
all shipped. The legacy `CombatPosition` enum, `PositionRoundsMin`
field, `GrappleControllerId` field, `ConditionGrappleController`
constant, and `internal/characters/combatposition.go` are deleted.

### Predicate methods (chunk 4a + 4b)

**Chunk 4a — 19 predicates** in `position_predicates.go` delegate to
the underlying machine with nil guards. Nil-guard convention:
`IsStanding()` returns `true` on a nil machine (matches `NewMachine()`
default); all others return `false`.

14 per-state predicates: `IsStanding`, `IsProne`, `IsSupine`,
`IsClinch`, `IsBackStanding`, `IsMount`, `IsSideControl`,
`IsKneeOnBelly`, `IsNorthSouth`, `IsCrucifix`, `IsBackGround`,
`IsHalfGuard`, `IsGuard`, `IsTurtle`.

5 rollup predicates: `IsGrappling`, `IsStandingGrapple`,
`IsGroundGrapple`, `IsTopDominant`, `IsOnFloor`.

**Chunk 4b-fixup-2 — control-axis predicates and helpers:**

- `IsController()` — true when `Character.Control.State() ==
  control.Controlling`. Reads the `internal/state/control` FSM on
  `Character.Control *control.Machine`. Replaced the deleted
  `HasCondition(ConditionGrappleController)` check (S4 shipped).
- `IsBeingControlled()` — true when `Character.Control.State() ==
  control.Controlled` (symmetric to `IsController`).
- `IsLowGrappleStamina()` — true when stamina fraction is below
  `GrappleStaminaLowThreshold` (config, default 0.25). Used by
  `mob_low_grapple_stamina` btree primitive and by
  `Position_Messaging` for the once-per-grapple "you're getting
  gassed" warning.
- `GetPositionSpeedMultiplier()` — replaces the deleted
  `CombatPosition.GetSpeedMultiplier()` helper (S5 shipped). Switches
  on `Position.State()`: Standing 1.0, Prone/Supine/Turtle 0.5,
  Clinch/BackStanding 0.6, ground grapples 0.3.

**Legacy enum — fully removed (T21 sunset, 2026-05-16):** the
`CombatPosition` enum, its `IsGroundPosition()` / `IsGrapplePosition()`
/ `GetSpeedMultiplier()` / `GetPositionColor()` helpers, the
`PositionRoundsMin` / `GrappleControllerId` fields, the
`ConditionGrappleController` constant, and the file
`internal/characters/combatposition.go` are all deleted. The mapping
table below is kept for historical reference (chunk 4c/4d/4e writers
should use these predicates from day one):

| Deleted legacy API | Current FSM predicate |
|--------------------|-----------------------|
| `== PositionProne` | `IsProne() \|\| IsSupine()` |
| `== PositionClinched` | `IsStandingGrapple()` |
| `== PositionGrounded` | `IsGroundGrapple()` |
| `!= PositionStanding` | `!IsStanding()` |
| `.IsGrapplePosition()` | `IsGrappling()` |
| `.IsGroundPosition()` | `IsOnFloor()` |
| `.GetSpeedMultiplier()` | `GetPositionSpeedMultiplier()` |
| `HasCondition(GrappleController)` | `IsController()` |

Position predicates also drive the chunk-4c reach utility
(`internal/combat/reach.go`): `IsGrappling()` + `State()` determine the
grapple radius, which scales weapon damage per swing.

### Prompt helpers (chunk 4b R6)

The `{pos}` prompt-token cutover added two private helpers in
`internal/users/userrecord.prompt.go`:

- `positionPromptColor(position.State) string` — returns the ANSI
  color name. Standing white, Prone/Supine yellow, Clinch/BackStanding
  orange, ground grapples red. Replaces the legacy
  `CombatPosition.GetPositionColor()`.
- `positionPromptAbbrev(position.State) string` — abbreviates long
  state names: BackStanding→B.Std, BackGround→B.Gnd, SideControl→SC,
  KneeOnBelly→KOB, NorthSouth→N-S, HalfGuard→H.Gd. Other states
  render verbatim via `State.String()`.

These live in the users package (not characters) because they format
the prompt-substitution output, not the underlying state.

### Chunk-4d submission fields (T2, T5)

Three fields added to the `Character` struct in chunk 4d:

**`SubmissionPolicy SubmissionPolicy`** — controller-side disposition
that resolves when the attempter locks a submission. Four values:
`PolicyMercy` / `PolicySubdue` / `PolicyCripple` / `PolicyLethal`.
Default for players: `PolicySubdue`. Set via `set submission` command.
Mob defaults are archetype-driven via
`DefaultSubmissionPolicyForArchetype(archetype)`.

Persisted: `yaml:"submission_policy,omitempty"`.

**`SurrenderPolicy SurrenderPolicy`** — controlled-side tap signal. A
struct `{Mode SurrenderMode, HpPctThreshold int}`. Three modes:
`SurrenderNever` / `SurrenderAlways` / `SurrenderAutoTap` (fires when
HP% drops below `HpPctThreshold`). Default for players:
`SurrenderAutoTap` at 15%. Set via `set surrender` command.

Persisted: `yaml:"surrender_policy,omitempty"`.

Only `mercy` policy on the attempter consults `SurrenderPolicy`. The
other three policies proceed regardless of the defender's tap signal.

**`LastDriftRoll DriftRollSnapshot`** — runtime-only snapshot of the
most recent per-round grapple drift roll. Written by
`Position_GrappleTick.go` at the end of each grapple round; read by
`Position_SubmissionTick.go` to decide whether a sub-attempt window is
open without re-rolling. The snapshot includes:
- `Round uint64` — the round number when the snapshot was taken
  (used by `EvaluateSubAttempt` to reject stale data).
- `MarginAttacker float64` — attacker-side drift margin.
- `AttackerZScore float64` — controller's z-score from the drift roll.
- `DefenderZScore float64` — controlled side's z-score.

Not persisted: `yaml:"-"`.

**`LastSubmissionAttempted int`** — round-robin index into the current
position's sub pool (`TopSubmissionsForPosition` or
`BottomSubmissionsForPosition`). Advanced by
`pickSubmissionRoundRobin` each time a sub attempt fires so the same
sub type is not hammered every round. Not persisted: `yaml:"-"`.

### CalcConcentrationChance (cast_helpers.go)

```go
func CalcConcentrationChance(willpower, damagePct int) int
```

Returns the % chance (0-100) that a caster maintains concentration given
their Willpower and an incoming disruption expressed as a percentage of max
HP. Higher Willpower → higher chance to hold; higher `damagePct` →
lower chance to hold. The formula uses a Willpower divisor and a flat base,
both tunable via config.

**Consumed by two independent disruption paths:**

1. **Damage-path** (`checkConcentrationBreak` in
   `internal/hooks/combat_shared_helpers.go`): fires when the caster
   takes damage mid-cast. `damagePct = (damage * 100) / maxHP`.
2. **Position-path** (`processFoldRound`, chunk 4f): fires every fold
   round when the caster is not `Standing`. `damagePct` comes from
   `position.PositionDisruptionDmgEquiv(pos, role)` —
   `internal/state/position/disruption.go`. Standing returns 0 (call
   to `CalcConcentrationChance` is skipped entirely in that case).

Both paths call `characters.CalcConcentrationChance` with the same
Willpower curve; both can break a cast in the same round (layered
disruption). Tests live in `internal/characters/casting_test.go`.

### OnCharacterCreated additions (chunk 4a + 4b)

The `OnCharacterCreated` registry gains four Position-related wire
callbacks across chunks 4a and 4b:

- **4a `wirePositionCrossMachineCascades`** — subscribes the
  `position_life_dead` observer to the Life machine; handles
  `Alive → Dead` cascade that resets Position to `Standing`.
- **4b `wirePositionGrappleTick`** — registers the per-round drift
  observer that fires opposed control rolls + grapple stamina cost +
  threshold-triggered position transitions.
- **4b `wirePositionMessaging`** — registers the per-round messaging
  observer that fires gradient ("getting controlled"), transition
  ("you scramble out of mount"), and stamina-warning text with
  per-grapple cooldowns.
- **4b `wirePositionConsistencyCheck`** — registers the periodic
  invariant checker (`ValidateGrapplePair`) that catches pair drift
  (e.g. controller's partner ref doesn't match controlled's ref).

## Presence Machine Integration (chunk 5)

### New field: Presence

```go
Presence *presence.Machine `yaml:"-"`
```

Initialized in `New()` (player path → `NewPlayerPresence()`, starts in
`Connecting`) and in `mobs.Mob.Validate()` after the shallow copy (mob
path → `NewMobPresence()`, starts in `Spawning`). The field is
nil-guarded at all consumers via `m.State()` which returns `Active` on
a nil machine. Not persisted: presence is transient session state that
resets on disconnect/respawn.

The Presence machine is the single canonical source for "is this
character meaningfully present?" — replacing the ad-hoc
`ManualAFK`/`BoredomCounter` fields that were removed in chunk 5.

### CancelAllScheduled helper (T8)

```go
func (c *Character) CancelAllScheduled()
```

Called by the scheduler-cancel observer when Presence enters a terminal
state (`Disconnected` for players, `Despawning` for mobs). Cancels all
pending scheduled transitions across all machines on this character
(Activity casting/crafting timers, Position recovery timers, etc.).
Wired by `hooks.wirePresenceSchedulerObserver` via `OnCharacterCreated`.

### OnCharacterCreated additions (chunk 5)

New registrations (all in `internal/hooks/`):
- `wirePresenceMobVetoes` — registers `Active→Dormant` and
  `Active→Despawning` vetoes that return `ErrVetoed` when
  `IsEssential() || IsCharmed()`.
- `wireCombatPhasePresenceVeto` — populates
  `CombatPhase.RegisterTargetPresenceCheck` with a closure that blocks
  `Idle→Engaging` for `Disconnected`/`Despawning` targets.
- `wirePresenceSchedulerObserver` — fires `CancelAllScheduled()` on
  terminal-state entry.

## Perception Machine Integration (chunk 6)

### New field: Perception

```go
Perception *perception.Machine `yaml:"-"`
```

Initialized in `New()` and nil-guarded in `Validate()` (for characters
loaded from YAML without a direct `New()` path). Also unconditionally
overwritten in `mobs.Mob.Validate()` after the shallow copy, and reset
to nil in `Character.ResetForMobInstance()` so fresh mob instances get
their own machine. Not persisted: perception state is transient and
reconstructed from active buffs/conditions at runtime.

The Perception machine tracks whether a character can see — `Sighted`
(default) or `Blinded` (any of three active sources: Buff 3, Buff 77,
or ConditionBlinded). Chunk 6 ships DORMANT: transitions fire correctly
via `AddBuff`/`RemoveBuff`/`AddCondition`/`RemoveCondition`, but no
consumer reads `Perception.State()` yet. The future messaging framework
chunk wires this into broadcast gating (visual broadcasts suppressed
while Blinded), infrared "red shapes" rendering, and look-command
blocking. See `internal/state/perception/context.md` for full details
and `messaging-framework-chunk` project memory for the successor scope.

### IsBlinded predicate

No `IsBlinded()` predicate ships in chunk 6 — the dormant design omits
it intentionally to avoid readers being added before the messaging
framework context is in place. The predicate will land in the messaging
framework chunk alongside the first real consumer.

### HasAnyBlindSource helper (sight.go)

`Character.HasAnyBlindSource()` in `internal/characters/sight.go` checks
all three blind sources and returns true if any is currently active. Used
by the expire-paths in `RemoveBuff` and `RemoveCondition` to determine
whether to fire `Blinded→Sighted` when one of multiple overlapping
sources clears. Uses `Buffs.TriggersLeft(id) > 0` rather than
`HasBuff(id)` — see `internal/state/perception/context.md` for the
implementation-detail rationale.

## Dependencies
- `internal/stats`: Core statistics definitions
- `internal/items`: Item system integration
- `internal/buffs`: Status effect system
- `internal/species`: Character species definitions
- `internal/skills`: Skill system integration
- `internal/spells`: Magic system integration
- `internal/quests`: Quest system integration
- `internal/pets`: Pet system integration
- `internal/gametime`: Time-based mechanics
- `internal/colorpatterns`: Text formatting and colors
- `internal/state/combatphase`: Combat Phase state machine (chunk 0)
- `internal/state/awareness`: Awareness state machine (chunk 1)
- `internal/state/life`: Life state machine (chunk 2)
- `internal/state/activity`: Activity state machine (chunk 3)
- `internal/state/position`: Position state machine (chunks 4a + 4b)
- `internal/state/presence`: Presence state machine (chunk 5)
- `internal/state/perception`: Perception state machine (chunk 6)

## Files

45 non-test files. Grouped by what they own:

| Group | Files |
|-------|-------|
| Core | `character.go`, `validate.go`, `migrations.go`, `overrides.go`, `description.go`, `formattedname.go` |
| Stats & progression | `progression.go`, `skills.go`, `effective_stats.go`, `statmods`-adjacent helpers, `mobmastery.go`, `kdstats.go` |
| Resources & conditions | `pools.go`, `resources.go`, `conditions.go`, `cooldowns.go`, `buffs.go`, `sight.go` |
| Inventory & gear | `inventory.go`, `inventory_handle.go`, `worn.go`, `hand_slots.go`, `anatomy.go`, `masterwork.go`, `migrate_enchantments.go` |
| Combat | `combat.go`, `combat_state_compat.go`, `combat_tokens.go`, `position_predicates.go`, `taunt_hold.go`, `submission_policy.go`, `die.go`, `respawn_home.go` |
| Casting | `cast_helpers.go`, `spells.go` |
| Mutation | `intrinsic.go`, `bloom.go`, `bloom_mutation.go`, `chrysifier.go`, `mutation_scour.go` |
| Social & economy | `companions.go`, `charminfo.go`, `shop.go`, `quests.go`, `alts.go` |
| Justice | `arrest_policy.go` |

## Gotcha: the position-migration table below is history, not API

The `CombatPosition` mapping table further down lists `IsGrapplePosition()`,
`IsGroundPosition()` and `GetPositionColor()` in its **left** column. Those are
the retired API; they do not exist. The live predicates are in
`position_predicates.go` — `IsGrappling()`, `IsStandingGrapple()`,
`IsGroundGrapple()`, `IsOnFloor()`, `IsBackGround()`, and the rest. Likewise
there is no `IsBlinded()`; use `HasAnyBlindSource()` (`sight.go`).
