# DOGMud Combat System Context

## Overview

The DOGMud combat system provides comprehensive turn-based combat mechanics with support for player vs player, player vs mob, and mob vs mob encounters. It features skill-based damage calculations, layered defenses (dodge/parry/block), dual wielding, critical hits, special combat moves with knockdown mechanics, prone condition system, backstab mechanics, pet participation, alignment-based consequences, and detailed combat messaging with cross-room attack support.

**DOGMud Differences from upstream GoMud:**
- Combat uses skill ranks (weapon-combat, unarmed-combat, ranged-combat) instead of character Level
- No Level-based combat formulas — all calculations use stats and skills
- Weapon-aware skill routing: melee → weapon-combat, bows → ranged-combat, fists → unarmed-combat
- Stat names: Dexterity (was Speed), Perception (was Smarts), Willpower (was Mysticism)
- Layered defense system with stamina costs (dodge/parry/block)
- Prone condition with combat modifiers and stat-based recovery
- Special combat moves (bash/trip/kick) with shared cooldowns
- Target switching mid-combat

## Architecture

The combat system is built around several key components:

### Core Components

**Combat Resolution Engine:**
- Turn-based combat with dexterity-based attack frequency
- Multi-attack system based on dexterity differentials
- Weapon-based damage calculations with species bonuses
- Defense reduction and damage mitigation
- Critical hit system with buff effects

**Attack Result System:**
- Comprehensive result tracking for damage, hits, and effects
- Multi-target messaging system for attacker, defender, and rooms
- Support for cross-room combat with directional messaging
- Buff application tracking for combat effects

**Combat Calculations (`calculations.go`):**
- Hit chance calculations based on dexterity statistics
- Critical hit probability based on stats and combat skill ranks
- Damage reduction through defense statistics
- Power ranking system for combat assessment
- Alignment change calculations for PvP consequences

## Combat Skill Resolution

### Weapon-Aware Skill Routing
```go
// GetCombatSkillTag() selects the appropriate DOG combat skill:
// - Equipped ranged weapon (bow/crossbow) → ranged-combat
// - Equipped melee weapon (sword/axe/etc.) → weapon-combat
// - No weapon or claws → unarmed-combat

// GetCombatSkillLevel() returns the rank of the appropriate skill:
// 1. Try the weapon-appropriate DOG skill
// 2. Fall back to legacy Brawling skill
// 3. Minimum return: 1
```

### Combat Types
- Player vs Mob combat with damage tracking
- Player vs Player combat with alignment consequences
- Mob vs Player combat with AI integration
- Mob vs Mob combat with charm attribution

### Natural-Attack Subtype Resolution

When an attacker has no equipped weapon, `buildWeaponSetup`
(`combat_helpers.go`) resolves the attack-message subtype from the
attacker's species `NaturalAttack` field (via `species.GetSpecies`).
If non-empty, that subtype (`bite`, `claws`, `slam`, `gore`, `sting`)
drives message selection instead of the generic-punch fallback
(`items.Unarmed`). An equipped weapon's own `Subtype` always takes
precedence — this path only fires when the weapon slot is empty. This
is what makes non-human mobs' basic attacks read as bites or claws
instead of punches.

## Key Features

### Advanced Combat Mechanics
- Dexterity-based multiple attacks per round
- Dual wielding with skill-based penalties
- Backstab mechanics with guaranteed critical hits
- Pet participation in combat (20% chance)
- Cross-room combat support with directional messaging

### Hit Calculation System
```go
// Hit chance based on dexterity statistics
// hitChance = 30 + (attackDex / (attackDex + defendDex)) * 70
// Clamped between 5% and 95%
```

### Critical Hit System
```go
// Critical hit probability uses combat skill ranks, not Level
// Base crit chance modified by:
// - Strength + Dexterity stats
// - Combat skill rank differential
// - Accuracy buff (doubles crit chance)
// - Blink buff on target (halves crit chance)
// - Grapple position: `c.IsController()` + IsStandingGrapple -0.2, IsGroundGrapple -0.4 (chunk 4b R1)
// - Backstab: guaranteed crit on first pass
```

### Dual Wielding
- Skill level 2: 50% chance to use both weapons
- Skill level 3+: Always dual wield
- Claws (natural weapons): Always dual wield
- Hit penalty: continuous scale from 50 (skill 0) to 10 (skill 50)
  - Formula: `penalty = 50 - (dualWieldLevel / 50.0) * 40`
  - Floor: minimum 10% penalty
- Extra arms (mutation): `+20` penalty for 3rd weapon, `+40` for 4th

## Stage 7.5: Tactical Combat Enhancements

### Layered Defense System (Best-of-All)
Combat defense uses a best-of-all system where ALL available defenses are
rolled and the one that won by the widest margin is selected. This replaced
the old sequential short-circuit approach. Benefits: every defense type gets
fair representation in combat text, and having multiple defense types is
always better (wider net).

**Defense Types:**
1. **Dodge** - Unarmed Combat + Dexterity, dearest to mount, always available
   - Reduced by heavy armor and encumbrance
   - -50% effectiveness when prone
2. **Parry** - Weapon Combat + weapon parry rating, cheapest, requires weapon
   - Two-handed weapons get bonus to parry
   - Can parry with either hand when dual wielding
   - -30% effectiveness when prone
3. **Block** - Weapon Combat + shield block rating, requires shield
   - Highly effective; priced between parry and dodge
   - -20% effectiveness when prone (shield still works from ground)

**Defense Floor:** `ContestFloor` (0.125) is symmetric and lives inside
`RunContest`, so a massively outclassed defender still saves at the floor rate
and a massively outclassing attacker is still stopped at the same rate. U6 Task
8 deleted the old melee-only `MinDefenseChance` / `MinAttackHitChance` pair;
they were applied after crit resolution had already returned, so the attack
floor was only ever evaluated on the swings a defence crit did not consume.

**Defence Costs:**
- Only the winning defense (best margin) costs anything — losing defenses are free
- The winner is charged with `ApplyCostFloat` (which delegates to
  `ApplyCostPartial`), so a defender who cannot pay in full still defends and
  simply pays what is left (U5b-2)
- Defence costs are one config formula, not per-defence Go arithmetic. U7 Task 6
  deleted `GetDefenseStaminaCost` and the three per-defence base knobs
  (`DodgeBaseStaminaCost` / `ParryBaseStaminaCost` / `BlockBaseStaminaCost`)
  with it. The price is now `costs.Calc`: `DefenceBaseStaminaCost` (1.0) ×
  encumbrance × inverse-skill × `{Dodge,Parry,Block}CostModifier` (1.25 / 1.10 /
  1.15). For an unladen rank-1 defender that is dodge **1.370**, block
  **1.2604**, parry **1.2056**.
- **Charge through `GetDefenseCostFloat` + `ApplyCostFloat`, never the integer
  pair.** All three of those numbers truncate to `1`, so an integer charge erases
  the modifiers entirely. That was live: `ResolveChannelDefence` used the integer
  entry point until U7, and `DefenceSetFor` routes dodge and block there for
  `ChannelRanged` and `ChannelSpellPhysical` (eleven shipped spells declare
  `target_defense_type: physical`), so block against a physical spell was charged
  1 rather than 1.2604.
- `ResourceMultiplier` is attack-side only. It does NOT penalise defence today —
  see the exhaustion gotcha under "Contest core" below.

**Implementation:**
- `runBestOfAllDefense()` in `combat_helpers.go` builds each defense's score and
  delegates the rolling and selection to `RunContest` (U1, floored in U6). It no
  longer rolls anything itself.
- `resolveDefenseOutcome()` processes the best result. Order is fumbles →
  winner (already decided, and already floored, by `RunContest`) → crit →
  normal outcome. Crit fires only for the side that WON, and never on a floored
  outcome, whose margin is the ±1 sentinel rather than a real one.
- One floor, `ContestFloor`, applied once inside `RunContest`. There is no
  second post-crit floor any more.
- Defense crit detection (z > 2.0): parry crit → disarm, dodge crit → grapple opportunity

### Attack cost is charged PER SWING (U7 Task 7)

`ChargeAttackCost(attacker *characters.Character, swings int) characters.CostResult`
in `attack_cost.go` is the attacker-side price. **Where it is charged:** in each
of the four wrappers in `combat.go` (`AttackPlayerVsMob`, `AttackPlayerVsPlayer`,
`AttackMobVsPlayer`, `AttackMobVsMob`), immediately after `calculateCombat`
returns, off `attackResult.SwingsThrown`. It is NOT charged inside
`calculateCombat` and not inside the weapon or swing loops.

```go
attackResult := calculateCombat(user.Character, &mob.Character, User, Mob, ctx)
ChargeAttackCost(user.Character, attackResult.SwingsThrown)
```

One swing is priced by the unexported `attackCostPerSwing`, which is the same
`costs.Calc` composition the five defences use:
`AttackBaseStaminaCost` × encumbrance × inverse-skill × `AttackCostModifier`.
The whole round is then charged as `perSwing × swings` through
`Character.ApplyCostFloat`, never `ApplyCostPartial`. The per-swing figure is a
product of four config floats and is rarely a whole number, so an integer charge
per round would erase the encumbrance and skill terms this task exists to
introduce.

Points that bite:

- **`SwingsThrown` counts swings THROWN, not swings that landed.** It is
  incremented in `calculateCombat` before resolution and outside the per-swing
  flag reset, and it accumulates across every weapon in the round because
  `attackResult` outlives the weapon loop. A missed swing is effort spent.
- **The skill rank comes from `GetCombatSkillLevel`**, which picks the skill
  matching the EQUIPPED weapon (weapon / unarmed / ranged combat), not the
  registry's nominal `skills.WeaponCombat`. An unarmed brawler's practice
  discounts their swings. It returns a minimum of 1, so a fresh character lands
  on the rank-1 multiplier rather than the rank-0 one.
- **A nil attacker or a non-positive swing count charges nothing** and returns
  the zero `CostResult`. Zero swings is a real state (no weapons to swing, target
  already gone) and is deliberately not `Short`: nothing was demanded.
- **This replaced a once-per-round `DeductAttackStamina` call** in each of the
  four wrappers. A twelve-swing build attacked twelve times for the price of one
  while the defender paid on every incoming swing, which is the single largest
  reason offence was effectively free next to defence.
- The returned `CostResult` is what U8 reads to strip the skill term from an
  attacker who could not pay in full. Discarding it is safe today, not later.

### Defence sets are a property of the channel (U6 Task 11)

`defence_sets.go` holds the whole table. `DefenceSetFor(channel) []string`
returns the defence names that apply to an `AttackChannel`:

| `AttackChannel` | Defences | N |
|---|---|---|
| `ChannelMelee` | dodge, parry, block | 3 |
| `ChannelRanged` | dodge, block | 2 |
| `ChannelSpellPhysical` | dodge, block | 2 |
| `ChannelSpellMental` | **quell** | 1 |
| `ChannelSocial` | **defy** | 1 |

Adding a defence to a channel is one row here and nothing else. Parry is
deliberately excluded from ranged and physical spells — you cannot parry a bolt.
Dodge is REUSED for physical spells; there is no separate physical-spell
defence. An unknown channel returns nil, not the melee set.

**quell and defy are NEW player-facing verbs** (chosen 2026-08-13; both were
previously called "resist", which collided). `characters.DefenseQuell` scores
`Willpower + spellcasting × SkillWeight` and answers a mental spell;
`characters.DefenseDefy` scores `Willpower + rhetoric × SkillWeight` and answers
a social attack. **Both cost CONVICTION, not stamina** — grepping for a stamina
cost finds nothing and proves nothing.

A set of size one is still a contest, not a different mechanism. That
unification is what let `avoidance.go` be deleted in Task 12.

**Wired for three channels, not for melee.** `ResolveChannelDefence`
(`defence_multiplier.go`) consumes this table for `ChannelSpellMental`,
`ChannelSpellPhysical` and `ChannelSocial` — the five spell sites in
`internal/hooks/spell_resolution.go` and the taunt site in
`internal/actions/combat_taunt.go`. **Melee does not.** `runBestOfAllDefense`
still builds its own equipment-derived `defSeq` from
`characters.GetDefenseSequence`, so editing the `ChannelMelee` row here changes
nothing on its own.

That split is what makes the checklist below read the way it does. Almost every
remaining item is a MELEE-pipeline consumer, and quell and defy cannot reach the
melee pipeline, so those items are latent-if-wired rather than live.

**Every consumer downstream of a defence name was written against a CLOSED
three-way set.** All of them are `switch`/map/`==` over a string-family type, so
a `"quell"` or `"defy"` value falls through silently — **not one of these is a
compile error**. Audited 2026-08-15; Task 12 status against each, worst first.

1. **Defender progression.** **FIXED.** `combat.AwardDefenceProgression`
   (`defence_multiplier.go`) is now THE per-defence mapping for all five, and
   both `ResolveChannelDefence` and `hooks.processDefenderProgression` call it
   rather than each carrying a switch. Note the stat change on the spell row:
   the deleted `TrySpellDeflection` awarded **perception**, because perception
   was what it contested; quell contests **willpower**, so willpower is what it
   trains. Losing perception as a spell-defence stat is the intended outcome of
   the unification, not a cost of it.
2. **`sendDefenseMessages` progresses the WRONG skill and prints broken
   grammar.** **FIXED defensively, still unreachable.** Its switch
   (`combat_helpers.go`) can still leave `skillToProgress` and `defenseVerb`
   empty for an unmatched defence, but both are now guarded: the progression
   call is skipped rather than rolling `TrackSkillUse("")` and banner-ing a
   nameless levelup, and the verb falls back to `"counter"` rather than
   formatting `"Grimwald s your attack!"`. `itemsDefenseType` deliberately still
   falls through to the zero value, which `items.GetDefenseMessage` already
   handles by returning an empty set.
3. **The cost path is stamina-only.** **FIXED.** `characters.DefensePool` +
   `Character.GetDefenseCostFloat` are the pair to charge through; `DefensePool`
   maps quell and defy to `PoolConviction` and everything else to `PoolStamina`,
   and `GetDefenseCostFloat` prices the two off `QuellBaseConvictionCost` /
   `DefyBaseConvictionCost`. `runBestOfAllDefense` and `ResolveChannelDefence`
   are both on the pair, so the old shape survives nowhere.
   `GetDefenseStaminaCost` — the stamina-only function that returned 0 for quell
   and defy, which is the trap the pair replaces — was DELETED in U7 Task 6.
4. **Analytics and the admin dashboard lose the swing entirely.** **DEFERRED,
   and not currently reachable.** `analytics.go` is fed from
   `AttackResult.SwingEvents`, which only the melee pipeline populates;
   `ResolveChannelDefence` does not build an `AttackResult` at all. So spell and
   social defences are invisible to `combatstats` — which was equally true of
   `TrySpellDeflection` and `TryStoicResolve`, so this is not a regression. It
   becomes live the moment quell or defy is wired into melee.
5. **Effectiveness knobs: FIXED. Positional knobs: DEFERRED.**
   `QuellEffectiveness` and `DefyEffectiveness` exist (Task 11 added them,
   default 1.0) and `ResolveChannelDefence` applies all five through
   `defenceEffectiveness`. The prone / clinch / grounded switches in
   `runBestOfAllDefense` were converted from bare string literals to the
   `characters.Defense*` constants, but still have no quell/defy arms: giving
   them one needs `ProneQuellPenalty` and five more knobs that do not exist, and
   the defences do not run through that function anyway. Disclosed in a comment
   at the switch.
6. **A quell/defy defensive crit has nowhere to record itself.** **DEFERRED, not
   reachable.** `AttackResult` still declares only `ParryCritDetected` /
   `DodgeCritDetected` / `BlockCritDetected`. `ResolveChannelDefence` returns a
   0.0 multiplier for a defensive crit, which fully negates, but there is no
   `AttackResult` in that path so `applyCritEffects` (riposte / sweep / shield
   slam) has nothing to fire from. Same as pre-U6 behaviour.
7. **The two parallel `DefenseType` enums are still three-valued.**
   **DELIBERATELY UNCHANGED.** `combat.DefenseType` (`attackresult.go`) and
   `items.DefenseType` (`internal/items/defensive_messages.go`) still mirror only
   dodge/parry/block. Task 12 did not need them, because the non-physical
   channels never construct an `AttackResult`. Keep it that way until there is
   message data behind the constants: `case combat.DefenseQuell:` failing to
   compile is a LOUD failure, and loud beats silent.
8. **`PowerScore` averages three defences.** **DEFERRED.** `calculations.go`
   computes `(dodge + parry + block) / 3.0`, under-weighting a character built on
   mental or social defence (feeds `modules/leaderboards`).
9. **`CategoryForDefenseVerb` falls back to `CategoryDodge`.** **DEFERRED.** Not
   a one-line fix: `internal/messaging` has no `CategoryQuell` / `CategoryDefy`,
   and `ansi-aliases.yaml` has no colour keys for them. Unreachable today for the
   same reason as 4 and 6.
10. **`DriftFromCombat("trickster", ...)`** (`NewRound_DoCombat_unified.go`)
    tests `DefenseUsed == DefenseDodge || == DefenseParry` by literal, so quell
    and defy never signal "evaded a blow". **DEFERRED**; flavour only, and
    unreachable today.

The help-alias and helpfile gaps were closed after Task 12. The remaining
content gap is owned by U8: add `quell.yaml` / `defy.yaml` under
`_datafiles/world/dogmud/defense-messages/`, extend the three-valued message
type mapping, and route the live spell and social paths through five coordinated
variants per weak/normal/heavy band. `GetDefenseMessage` currently returns empty
for either unknown key, so the missing data degrades to hardcoded narration
rather than breaking. Broader combat-message unification remains deferred.

### `ResolveChannelDefence` — the non-melee defence resolver (U6 Task 12)

```go
func ResolveChannelDefence(channel AttackChannel, attacker, defender *characters.Character) float64
func ChannelAttackScore(channel AttackChannel, attacker *characters.Character) float64
func AwardDefenceProgression(c *characters.Character, userId int, defenceType string)
```

`ResolveChannelDefence` runs ONE opposed contest and returns the ATTACKER's
damage multiplier: `1.0` when the attack wins, `0.0` on a defensive crit, and
between `0.0` and `0.5` on an ordinary defensive win, off the same
`DefenceMitigation` curve melee uses.

It replaces `TrySpellDeflection` and `TryStoicResolve`, which each ran a SECOND
independent contest on top of their channel's primary roll, on different stats,
and returned a flat configured multiplier. `SpellAvoidanceDamageMultiplier` and
`RhetoricAvoidanceDamageMultiplier` were deleted with them; the endpoints of the
curve are structural, so there is nothing left to tune there. (Both keys may
still sit in `config.yaml`; `yaml.Unmarshal` is non-strict, so an orphan key is
ignored rather than fatal.)

Things that bite:

- **`ChannelAttackScore` returns 0 for melee and ranged**, on purpose. Those
  build their score in `calcAttackScore` with weapon, reach, position and
  resource terms this function cannot see.
- **Both spell channels share one attack score.** The channel decides what
  DEFENDS, not what powers the attack: a physical-flavoured spell is still cast
  with willpower and spellcasting, it is simply dodged rather than quelled.
- **The mounted defence is charged and progressed WIN OR LOSE**, matching melee
  (`runBestOfAllDefense` charges the best defence every swing) and matching the
  two deleted functions (which awarded progression unconditionally).
- **The margin is negated exactly once, here.** `contest.Result.Margin` is
  ATTACK-positive and everything after the win check is the defender's. Do NOT
  copy the negation from `normalizedDefenseMargin`, which reads
  `bestDefenseResult.margin` — already DEFENCE-positive. The conventions are
  opposites, mixing them compiles cleanly, and the crit lands on the losing side.
  `contest_sign_test.go` is the guard.
- **A FLOORED save returns the bare 0.5 and never the curve**, same rule as
  melee: the ±1 sentinel is in raw score units, not sigma.

### A defensive win is a PARTIAL DEFLECTION, not a clean miss (U6 Task 10)

**`res.hit == true` no longer means "the defence failed."** A defensive win now
lands as a partially deflected hit and carries a `damageMult` between 0.0 and
0.5. Anything that reads `res.hit` (or `AttackResult.Hit`) as a proxy for "the
attack got through" is asking the wrong question — U6 Task 14 added the signals
that answer it: `hitResolution.defended` is true exactly on the deflection path
(defence won, partial damage through; false on every clean-win, fumble, and
defensive-crit path), and `AttackResult.CleanHit` / `WeaponHitInfo.CleanHit`
aggregate "at least one swing actually won the contest" across the round the
way `Hit` aggregates "damage was dealt". Progression, sounds and weapon break
key on `CleanHit`; damage-scaled consumers (lifesteal, on-hit procs, wimpy)
stay on `Hit`; momentum is per-swing and keys on `res.hit && !res.defended`
inside the swing loop, not on the round aggregate.

`DefenceMitigation(normalizedDefenceMargin)` in `defence_multiplier.go` is the
curve: a bare win removes **50%**, rising linearly to **100%** at
`ContestCritThreshold`. Skill raises the margin, so skill raises mitigation
continuously instead of in a step. A defensive **crit** is not on this curve —
it fully negates and fires the counterattack.

Before this, a defensive win was zero damage while `TrySpellDeflection` and
`TryStoicResolve` produced a flat 0.5 or 0.0 for spells and taunts: two
mechanisms answering one question. Task 12 folded those channels onto this curve
via `ResolveChannelDefence` and deleted both functions.

Four things that bite:

- **`hitResolution.damageMult` has no safe zero value.** Its zero is 0.0, which
  deletes the swing's damage. Every return path in `resolveDefenseOutcomeCore`
  sets it explicitly; a new path that forgets fails silently.
- **A FLOORED save takes the bare 0.5 and never the curve.** The ±1 sentinel is
  in raw score units, not standard deviations, so normalising it yields
  `1/(stdDev*√2)` — about 0.05 z at typical scores, but **0.71 z** when
  `StdDevFor` clamps at its 1.0 floor for a very weak defender, which would hand
  the weakest defender the biggest floored save. `resolveDefenseOutcomeCore`
  special-cases `best.floored`; `TestFlooredSentinelDoesNotNormaliseToZero`
  records the actual numbers.
- **A floor-promoted defence crit must re-negate.** `applyCritFloors` runs after
  the multiplier is set, so promoting a defensive win to a defence crit also
  resets `res.hit`/`damageMult`. Without that, a floor-promoted crit would deal
  partial damage where a rolled one deals none.
- **U6 Task 13 extended this to skill moves.** `ExecuteSkillMove`
  (`skill_moves.go`) now runs its damage through `defenceDamageMultiplier` too,
  so `SkillMoveResult.Hit == false` with `Damage > 0` is a legal pair — a
  defended bash/trip/kick still lands partial damage, and `Damage` is the
  contest-scaled amount actually applied to the defender's pool, never the
  unscaled base. The maneuver's STATUS EFFECT stays binary, though:
  `StatusApplied` and `KnockedDown` can only be true when `Hit == true` — there
  is no "partially tripped."

The multiplier is applied in `calculateCombat` (`combat.go`), **after**
`calcHitDamage` rather than folded into `sdp.dmgMean`: `dice.RollStat` derives
its spread from the mean it is handed, so scaling the mean would shrink the
variance too and make deflected hits artificially consistent. A non-zero
multiplier floors the result at 1 damage, matching `CritOrMitigatedDamage`'s
rule that a hit which lands must do something (`calcHitDamage` itself floors at
0, not 1).

### Prone / Supine Knockdown System (Position FSM, chunks 4a + 4b)

Characters can be knocked to the ground by special combat moves
(bash/trip/kick) or spell knockdowns, applying severe combat penalties.
Chunk 4a split the legacy single "prone" state into **Prone** (face-down)
and **Supine** (face-up); chunk 4b cut over every writer and most readers
to the new Position FSM.

**Down state (Position FSM):**
- Canonical source: `Character.Position` (`*position.Machine`). Use
  `c.IsProne()` and `c.IsSupine()` predicates; rollup `c.IsOnFloor()`
  covers either + ground grapples.
- Per-state data: `ProneData` / `SupineData` carry
  `MinRecoveryRounds` (replaces legacy `PositionRoundsMin`),
  `KnockdownSource` (the attacker's `ActorRef`), and the
  `TransitionReason`.
- The legacy `CombatPosition` / `PositionRoundsMin` parallel-writes are
  removed (T21 sunset). The legacy enum collapsed Prone/Supine into one
  bucket; the FSM distinguishes them.
- Visual indicator: "prone" adjective added via `GetAdjectives()` (still
  enum-driven; helpfile content enhancement deferred to chunk 4f).

**Why split Prone vs Supine?** Submission paths and recovery
mechanics diverge: Prone is back-take-vulnerable and harder to
recover from; Supine can pull guard (`TransitionToGuard`) and recovers
more easily. Mechanically (4b): both states share the same combat
penalty profile via `IsProne() || IsSupine()` reads in
`combat_helpers.go` (R1). Submission-engine divergence is chunk 4d.

**Combat modifiers** (applied in `combat_helpers.go`, all migrated to
`IsProne() || IsSupine()` in chunk 4b R1):
- Attacker prone: `dmgMean *= ProneDamagePenalty` (config),
  `attackScore *= ProneAttackMultiplier`
- Defender prone: `attackScore *= ProneVulnerabilityMultiplier`
- Dodge/parry/block penalties: `ProneDodgePenalty` / `ProneParryPenalty`
  / `ProneBlockPenalty` (defense penalty switch reads
  `IsProne() || IsSupine()` → prone bucket).

**Behavioral restrictions** (chunk 4b R3, reads
`IsProne() || IsSupine()`):
- Cannot flee from combat (`mobcommands/flee.go` + `handlePlayerFlee`
  apply a 0.5x flee-score penalty; grapple states block flee entirely).
- Cannot move between rooms (enforced in movement commands —
  unmigrated reader, scheduled in the broader sweep).

**Recovery mechanics** (chunk 4b W6 / W7 cutover):

1. **Automatic recovery** — stat-based logarithmic formula
   `min(90, 25 + 20 × ln(DEX/25))`. Implemented in
   `Character.AttemptRecovery(statValue int)`. Gates on
   `IsProne() || IsSupine()` and reads `MinRecoveryRounds` from
   `ProneData` / `SupineData`. Decrements via
   `Position.ConsumeRecoveryRound()` (mutates the per-state slot in
   place). On success fires `Position.TransitionToStanding(TriggerRecoveryRoll)`.
   Called every round via `NewRound_UserRoundTick` and `NewRound_MobRoundTick`.
   Failed attempts add `ConditionRecoveryPenalty` (limits attacks to 1).

2. **Manual recovery** — `stand` command (`internal/usercommands/stand.go`)
   - Costs `StandStaminaCost` (config, 15% of max). Requires
     `StandMinStamina` remaining.
   - Bypasses `MinRecoveryRounds`. Fires
     `Position.TransitionToStanding(TriggerStandCommand)` BEFORE
     deducting stamina so an FSM-edge failure bails without charge.
   - The legacy `CombatPosition` / `PositionRoundsMin` parallel-writes
     are removed (T21 sunset); the FSM transition is the sole write.

### Grapple mechanics (Position FSM control axis, chunk 4b)

The 11 grapple states (Clinch, BackStanding, Mount, SideControl,
KneeOnBelly, NorthSouth, Crucifix, BackGround, HalfGuard, Guard,
Turtle) live in `internal/state/position/`. Per-round control drift
is resolved through outcome rolls (Hold / Advance / Degrade / Reversal /
Escape), tracked via `GrappleData.IsControllerRole` bool. Canonical
documentation: `internal/state/position/context.md`. Brief summary of
how the combat package interacts:

- **Per-round drift** — `Position_GrappleTick.go` (hooks package) fires
  the opposed Strength + Unarmed-combat roll each round, scaled by
  stamina + encumbrance curves. The roll result resolves via
  `position.ResolveOutcome` to one of five tiers (Hold / Advance /
  Degrade / Reversal / Escape), which may trigger position transitions.
  See `internal/state/position/context.md` for the outcome model.
- **Per-round stamina cost** — `GrappleStaminaCostPerRound` × a
  per-role multiplier (controller 1.0x, controlled 2.0x by default;
  asymmetry is the "smother" feedback loop).
- **Third-party defense filter** — `IsThirdPartyAttack` (chunk 4b R2)
  now reads `target.IsGrappling()` + `GrappleData.Partner` instead of
  the deleted `CombatPosition.IsGrapplePosition()` + `GrappleControllerId`
  fields. Zero-Partner (solo Turtle) preserves the "no controller → not
  third-party" semantics; 4e refines.
- **Crit-threshold bonuses** — controller in any grapple grants a
  crit boost (-0.2 standing, -0.4 ground); reads `c.IsController()`
  (chunk 4b R1, replaces `HasCondition(ConditionGrappleController)`).

The legacy `CombatPosition` enum is fully removed (T21 sunset). All
readers in `combat_helpers.go`, `ai.go`, `grapple.go`, and across
the codebase were migrated in the chunk-4b reader sweep before deletion.

### Special Combat Moves
Three tactical combat abilities with knockdown mechanics and shared cooldown:

**Bash Command** (`usercommands/bash.go`):
- Requirements: Shield equipped (checked via `HasShield()`), in active combat
- Damage: 50% of Strength stat (config: `BashDamagePercent`)
- Knockdown chance: 40% base (config: `BashKnockdownChance`)
- Opposed check: Weapon Combat + Strength vs. target's combat skill + Dexterity
- Skill progression: Weapon Combat
- On knockdown: Sets `target.Prone = true`, `ProneRoundsRemaining = 2`

**Trip Command** (`usercommands/trip.go`):
- Requirements: In active combat
- Damage: 25% of Strength stat (config: `TripDamagePercent`)
- Knockdown chance: 60% base (config: `TripKnockdownChance`) - highest of the three
- Opposed check: Unarmed Combat + Dexterity vs. target's combat skill + Dexterity
- Skill progression: Unarmed Combat
- Tactical: Low damage, high setup potential

**Kick Command** (`usercommands/kick.go`):
- Requirements: In active combat
- Damage: 40% of Strength stat (config: `KickDamagePercent`)
- Knockdown chance: 35% base (config: `KickKnockdownChance`)
- Opposed check: Unarmed Combat + Strength vs. target's combat skill + Dexterity
- Skill progression: Unarmed Combat
- Balanced: Moderate damage and knockdown

**Shared Cooldown System:**
- All three moves share a single cooldown (config: `SpecialMoveCooldown`, currently 4 rounds)
- Tracked in `Character.Cooldowns` map with key "combat-special"
- Cooldowns automatically decrement via `RoundTick()` called in combat hooks

**Intentional: Cooldown-blocked specials still initiate combat.**
If a player opens a fight with a special move (kick, bash, trip) while
on cooldown, the move itself fizzles but combat still starts. This is
by design — the player committed to an aggressive action and the target
noticed. This prevents risk-free cooldown probing (try special on a
passive mob, walk away if on cooldown, repeat). The player learns to
track their cooldown timing.
- Prevents knockdown spam, encourages tactical timing

### Anatomy-Gated Special Moves (non-human combat, Phase 2)

Special moves are gated by the actor's species anatomy so non-humanoid
creatures can't use human techniques. The predicate is
`Character.HasBodyPart(part)` (reads the species `body_parts` list).
The `CanUse*` viability checks in `ai.go` enforce:

- **grapple / submit** → require `arms` (a beast with no arms can't
  seize or apply a submission hold). `submit` is additionally
  transitively gated (a controlling ground grapple already needs arms).
- **bash** → requires `(HasShield OR NaturalBash) AND (arms OR
  NaturalBash)`. `NaturalBash` species (golems, elementals) bash with
  their whole body, bypassing both the shield and arms requirements.
  Note `HasShield()` already returns true for `NaturalBash` species.
- **trip / kick** → require `legs`. Wolves/felines (legged) trip and
  kick naturally; serpents/oozes are blocked.
- **hamstring** → requires `legs` AND a `Bite`/`Claws` natural attack.
  It is a beast move (a low slash/bite to the leg tendons), now woken
  into AI selection via `CanUseHamstring`/`ScoreHamstring` and weighted
  in the `aggressive`/`brawler`/`default` AI profiles. `hamstring` is a
  registered mob command, so `handleMobAIDecision` dispatches it like
  the others.

**Three sync points per gated move** (the `// SYNC POINT` contract):
the AI `CanUse*` gate (`ai.go`), the parity check
`actions.CommandIsReady` (`command_readiness.go`), and the action entry
`actions.Execute*` (`combat_*.go`) must all agree. The action entry is
defense-in-depth (unreachable for players, who are always humanoid);
`TestCommandReadinessDrift` rows (`*_no_arms`/`*_no_legs`) assert the
gate and readiness stay in sync.

**Retired: the `bite` special.** Biting is now the Phase-1 basic attack
for fanged species (see Natural-Attack Subtype Resolution above), so the
dedicated `bite` special move (`actions.ExecuteBite` + the `bite` mob
command) was removed. `items.Bite` (the natural-attack subtype) and
`toxic-bite` (a mutation move) are unaffected.

### Beast Moveset (Phase 3)

Six beast special moves with full player↔mob command parity. Identity
is gated at **three sync points** (the `// SYNC POINT` contract), the
same pattern as Phase 2: the AI `CanUse*` viability check in `ai.go`,
`CommandIsReady` in `command_readiness.go`, and the action entry
`Execute*` (defense-in-depth, unreachable for ordinary humanoid players).
`command_readiness_drift_test.go` rows assert all three sites agree.

**Exported identity predicates** (single source of truth, `combat/ai.go`):
- `SpeciesIsFanged` — species has a `bite`/`fangs` natural attack
- `SpeciesIsClawed` — species has a `claws` natural attack
- `SpeciesIsHorned` — species has `horns` in its `body_parts`
- `SpeciesHasLifeDrain` — species carries `lifedrain: true`
- `SpeciesIsQuadrupedPredator` — has `legs` AND (fanged OR clawed);
  the gate for `pounce`

| Move | Gate | Mechanic |
|------|------|----------|
| `rake` | clawed | Damage + short bleed (`claws.yaml` messages reused) |
| `maul` | fanged | Heavier damage + stronger bleed (`maul.yaml`) |
| `pounce` | quadruped predator, not already grappling | Leap opener: knockdown + damage, no bleed (`pounce.yaml`) |
| `gore` | horned (`horns` body part — load-validated) | Charge: damage + knockback (`gore.yaml`) |
| `drain` | `LifeDrain` flag (vampire) | Lifesteal: bleed target, heal attacker = `damage × DrainHealRatio` (0.75) via `Character.Heal` (`drain.yaml`) |
| `throttle` | fanged | Damage + `ConditionBleeding` + Throttled buff #89 (stamina DoT) + high-probability cast interrupt via shared `actions.InterruptTargetCast` helper (`ThrottleInterruptChance` 0.75), which reuses the engine's existing `activity.TriggerCastCancel` cancel path (+ conviction refund). No new silence flag. (`throttle.yaml`) |

**New species field:** `LifeDrain bool` (yaml `lifedrain`). Vampire
(species 34) carries `lifedrain: true`; the boar (species 6) received
`horns` added to its `body_parts`. Load validation: a species whose
`natural_attack` is `gore` must declare `horns` in `body_parts` — the
server panics at startup if the field is missing.

**New AI profiles:** `predator` (fanged hunters), `ambush_predator`
(clawed stalkers), `brute` (boars/bears). The new moves are also
weighted into the existing `default` and `aggressive` profiles.

**New config knobs** (`Balance` section):

| Knob | Default | Effect |
|------|---------|--------|
| `DrainHealRatio` | 0.75 | Fraction of drain damage returned as healing to the attacker |
| `ThrottleInterruptChance` | 0.75 | Probability throttle breaks the target's active cast |

**New buff:** `89-throttled.yaml` — stamina tick DoT, no special flags.

### Beast Moveset Refinements (Phase 4)

**The `hands` rule (true-beast gate).** The six beast natural-weapon moves
— `rake`, `maul`, `pounce`, `gore`, `throttle`, and `hamstring` — additionally
require the actor to have **no `hands` body part**, applied at all three sync
points (`CanUse*`, `Execute*`, `CommandIsReady`) + `*_hashands` drift rows. This
is the symmetric counterpart to Phase 2's "humanoid technique moves require
`arms`": humanoid moves need `arms`; beast natural-weapon moves need no `hands`.
Effect: tool-using (monster-)humanoids that carry a beast `natural_attack`
(goblin/skeleton/vampire — they have `hands`) keep grapple/bash + their
claw/bite BASIC-attack messaging but can NOT rake/pounce/etc.; **bears** (`arms`
but no `hands`) stay maulers. **`drain` is EXEMPT** — it is `LifeDrain`-flag
gated, not anatomy/`natural_attack` gated, so armed undead (vampire, wraith,
spectre) still drain.

**New AI profiles:** `skirmisher` (small fanged vermin — rats/insects:
hamstring/trip dominant, light maul) and `serpent` (legless fanged —
snakes/worms: maul/throttle, no pounce/hamstring). Species→profile assignment
across the beast mobs: canine/reptile/mustelid→`predator`,
feline/bat/raptor/arachnid→`ambush_predator`, boar/bear/deer→`brute`,
rodent/insectoid→`skirmisher`, serpent/worm/fish/carnivorous-plant→`serpent`;
slimes/elementals/humanoids stay `default`.

**Content retags:** `deer` (species 7) → `gore` + `horns` (antlered charge);
`wraith` (32) + `spectre` (33) → `lifedrain: true`. Wraith/spectre are
`pure_caster`/`aiprofile: caster` with spellbooks; since `handleMobAIDecision`
runs `ChooseCastAction` FIRST and only falls to `ChooseSpecialMove` when no
spell is castable, `drain` is their natural OCCASIONAL fallback (their only
viable special — no legs/arms). The `caster` profile carries a modest
`drain: 15` weight.

**Inert combatcommand cleanup:** mob `combatcommands` that listed a move the
mob's anatomy now forbids (a no-op under gating) were removed — e.g.
`sump_dweller`'s `bash` (aberration, no arms/naturalbash) and two legless
elementals' `trip`. Valid ones kept (wolves' `hamstring`; naturalbash
elementals' `bash`).

### Target Switching
Players can switch combat targets mid-fight using `attack <new-target>`:

**Implementation** (`usercommands/attack.go`):
- When already in combat (`user.Character.Aggro != nil`), retargets to new enemy
- Validates new target is in room and attackable
- Updates `Aggro.UserId` or `Aggro.MobInstanceId` to new target
- Automatic retargeting when current target dies (handled in combat death processing)
- Party coordination: Multiple players can target same enemy

**Messaging:**
- Informs user of target switch: "You shift your focus to <new-target>!"
- Room sees: "<player> shifts their focus to <new-target>!"

## Combat Messaging System
- Dynamic message selection based on damage percentage
- Token-based message customization ({source}, {target}, {weapon}, etc.)
- Separate messaging for same-room vs cross-room combat
- Critical hit and backstab message highlighting

## Power Ranking System
```
Combat assessment weights:
- Damage output: 40%
- Dexterity comparison: 30%
- Health comparison: 20%
- Defense comparison: 10%
```

## Power Scoring & Gear Contribution

`combat.PowerScore(char)` combines six terms: Offense, Defense,
Durability, Skills, Mutations, and KD ratio. Equipment
contribution flows through the standard pipes; there is no
separate "gear quality" axis.

| PowerScore term | Equipment field(s) that feed it |
|---|---|
| Offense (physAtk per-swing) | weapon `DamageMultiplier`, `SpeedMultiplier`; offhand + ExtraArm weapons |
| Offense (magAtk caster) | equipped weapon `SpellDamageMultiplier` |
| Offense (any stat-derived) | equipment `StatMods` → `Stats.X.ValueAdj` |
| Defense (mitigation) | equipment `PhysicalMitigation` / `MagicalMitigation` / `ConvictionMitigation` summed by `char.Get*Mitigation()` |
| Defense (avoidance) | equipment-driven dodge/parry/block via `char.GetDefenseScore(...)` |
| Durability | `char.HealthMax.Value` / `StaminaMax.Value` / `ConvictionMax.Value` — all reflect equipment stat boosts |
| Skills | not gear-driven |
| Mutations | not gear-driven |
| KD ratio | not gear-driven |

A player swapping a steel sword for an iron one will see
PowerScore drop because (a) the weapon's `DamageMultiplier`
changes (physAtk) and (b) any stat-mod difference flows through
`ValueAdj` into multiple terms. The Incorporeal mutation (chunk
2.2a) further scales gear contributions via
`mutations.GearEffectivenessMultiplier` — an ethereal wraith's
PowerScore reflects gear at the rank-determined fraction.

Consumers: `actions.Consider` (player + mob `consider`),
behavior tree conditions `target_power_ratio_above` and
`target_power_ratio_below`, behavior tree action
`target_weakest_mob_in_room`.

## Contest core (chunks U1 to U6)

Every floored opposed roll in this package, in `internal/actions`, in
`internal/forager`, in `internal/hooks` and in `internal/usercommands`
now resolves through `internal/contest`. Callers do not reach that
package directly: they go through `RunContest` in `run_contest.go`,
which reads the one symmetric floor and hands it to
`contest.RunWithFloors`.

### Public API

```go
// run_contest.go: the single flooring entry point (U6). Reads
// Balance.ContestFloor -- the only place in the game that does.
func RunContest(atkScore float64, entries []contest.Entry) contest.Result

// margin_crit.go / crit_floor.go: crit derived from the contest margin.
const ContestCritThreshold = 2.0
func ContestCrit(margin float64, roll dice.RollResult) bool
func AttackContestCrit(margin float64, roll dice.RollResult) bool
func DefenseContestCrit(margin float64, roll dice.RollResult) bool
```

There is nothing to choose any more. U1 to U4 shipped three wrapper
pairs (`RunWithManeuverFloors`, `RunWithSpellFloors`,
`RunWithGlobalFloors`) over eight per-channel knobs, and picking between
them was a statement about the cost of a single failure. U6 deleted all
three along with `contest_floors.go`: config shipped the pairs at
similar values, so the wrong pick was invisible in production and would
have become a live balance bug the first time one pair was retuned.

Single-defender sites pass one unnamed entry, so `Result.Winner` is
`""` for them. Ask `Result.Contested`, never `Result.Winner`, to find
out whether a contest happened.

Melee is no longer an exception. U6 pointed `runBestOfAllDefense` at
`RunContest` and deleted the post-crit `MinDefenseChance` /
`MinAttackHitChance` pair from `resolveDefenseOutcomeCore`, so melee is
floored once, in the same place as everything else.

Callers today: `ExecuteSkillMove`, `AttemptGrapple`,
`RollSubmissionAttempt`, `ResolveChannelDefence`,
`actions.ExecuteTaunt`, `usercommands.Throw`,
`hooks.processGrapplePair` (grapple drift), `hooks.tickMobCharmState`
(charm reroll), the spell sites in `hooks/spell_resolution.go` +
`hooks/charm_spell.go`, both flee rolls in `flee.go`, and the U4
out-of-combat sites in `actions/sneak.go`, `actions/shadow.go`,
`actions/steal.go`, `actions/plant.go`, `actions/defuse.go`,
`usercommands/go.go` and
`usercommands/skill.skullduggery.shadow.go`. `dice.OpposedRollStat` and
`dice.OpposedRollStatWithFloors` have zero production callers and carry
`Deprecated:` markers; U6 deletes them.

### Gotchas

- **`calculateCombat` takes BOTH combatants as `*characters.Character`, and they
  must stay pointers.** The signature is
  `calculateCombat(sourceChar *characters.Character, targetChar *characters.Character, sourceType SourceTarget, targetType SourceTarget, ctx combatContext) AttackResult`,
  changed from value parameters in U7 Task 1. It took its combatants BY VALUE
  from the day it was written, so every wrapper handed it a copy and every
  in-place mutation a callee made was written to that copy and discarded on
  return. The costly one was the defence charge: `runBestOfAllDefense` charges
  the defender in-place, which means melee dodge, parry and block cost nothing
  in production for the entire life of the code. The attacker's cost only ever
  survived because the wrappers charge it themselves, OUTSIDE this function, and
  damage only survived because it travels home in `AttackResult` and the wrapper
  applies it to the real character. Reverting also re-disables three writes that
  only work through the pointer: cross-round momentum (`UpdateMomentum`), the
  `SurpriseAttack`-to-`DefaultAttack` demotion in `SetAggro`, and defender
  skill-use tracking on mobs. **Nothing about that failure is visible.** The
  compiler is happy either way, and a test asserting that a charge was
  *requested* (`ApplyCostPartial` reports `Charged: 4`) still passes while the
  real character's stamina never moves. Do not "simplify" the parameters back.
- **`runBestOfAllDefense` has no affordability gate, on purpose.** Every defence
  in the sequence enters the contest regardless of the defender's stamina, and
  only the winner is charged, partially. Re-adding a gate would drop an
  exhausted defender out of the contest entirely, leaving them nothing but the
  uncontested fall-through, which since U6 Task 8 is an unconditional hit --
  the old flat save that used to catch that case is gone. Defence attempts and
  stance counting happen above where the gate
  used to be, so they are unaffected either way.
- **Exhaustion currently costs a defender nothing.** `GetDefenseScore` has no
  resource term and every `ResourceMultiplier` caller is attack-side, so between
  U5b-2 and U8 a 0-stamina defender defends exactly as well as a rested one.
  That is a known, temporary, deliberate gap; U8 strips the skill term. Do not
  "fix" it by re-adding a gate.
- **Read `contest.Result.Margin`. Never a `dice.RollResult`'s
  `.Margin`.** The core rolls each side with `dice.Roll`, which does not
  populate `RollResult.Margin`, so `res.AttackRoll.Margin` and
  `res.DefenseRoll.Margin` are always zero. Reading one compiles, passes
  every test, and silently disables crits on that path. This nearly
  shipped in U2 on spell deflection.
- **`Result.Margin` is ATTACK-positive.** Pass it **unnegated** for an
  attacker's crit check (`actions/combat_taunt.go`, the spell sites in
  `internal/hooks`); pass **`-res.Margin`** for a defender's
  (`ResolveChannelDefence`). Mixing the conventions
  compiles cleanly and puts the crit on the losing side. Note that
  `bestDefenseResult.margin` uses the OPPOSITE convention
  (defence-positive) because `runBestOfAllDefense` flips it once at the
  seam, which is why `normalizedAttackMargin` negates and `ContestCrit`
  must not.
- **A floored outcome carries a sentinel margin of `+1` / `-1`**, not the
  real one, and `Result.Floored` records it. The sentinel normalises to a
  near-zero z, which is the only reason a hit handed out by the floor
  cannot also crit. Do not "restore" the real margin.
- **`ContestFloor` is NON-ZERO in a test binary.** This is a behaviour change at
  U6 and it has already broken one test. A Go test binary never loads
  `_datafiles/config.yaml`, so the old maneuver and spell pairs measured **0**
  there and a test could ignore them; `Balance.Validate` replaces a zero or
  out-of-range `ContestFloor` with **0.125**, so the floor is live in every test
  binary. A test that needs a genuine contest on every iteration must pin it:
  `c := configs.GetConfig(); c.Balance.ContestFloor = 0; configs.SetConfigForTest(t, c)`
  (`SetConfigForTest` assigns without validating, so the zero survives). Pinning
  `dice.SetContestFloors` no longer reaches this path at all.
- **`Result.Floored` is read by `defenceDamageMultiplier`** (`defence_multiplier.go`,
  reached via `ResolveChannelDefence` and `ExecuteSkillMove`), which takes the
  bare 0.5 multiplier on a floored save rather than feeding the ±1 sentinel
  margin into the curve. `RunContest` is still the single choke point for every
  opposed contest in the game, so it remains the cheapest place to instrument
  the floor-reliance rate the roadmap wants modelled.
- **`SkillMoveParams.AttackSkill` and `.DefenseSkill` are RAW skill
  levels with NO `SkillWeight` applied.** `ExecuteSkillMove` adds them
  straight to the stats (`AttackSkill + AttackStat` vs `DefenseSkill +
  DefenseStat`), so every one of its callers runs at an effective
  weight of ×1 on both sides. That is deliberate today and is NOT what
  the arc's flip table assumes, which is why U6's "uniform ×5" needs a
  modelling gate before it lands (see the roadmap).
- **`DefenseStat: 0` is a real pattern, not an oversight.**
  `actions/combat_fire.go` (ranged) folds the defender's whole defence
  (Dexterity, combat skill, and a flat shield bonus when an offhand with
  a block rating is worn) into a single scalar via `rangedDefenseScore`
  and passes it as `DefenseSkill`, leaving `DefenseStat` zero. Anything
  that reweights `DefenseSkill` reweights the defender's Dexterity and
  the shield bonus along with it.

## Dependencies

- `internal/characters` - Character stats, equipment, and abilities
- `internal/contest` - The shared contest core (rolling + best-of-N selection)
- `internal/items` - Weapon specifications and combat messaging
- `internal/users` - Player character management and state
- `internal/mobs` - NPC character management and AI integration
- `internal/buffs` - Status effects that modify combat
- `internal/skills` - Skill system for combat skills and dual wielding
- `internal/species` - Species bonuses and unarmed combat specifications
- `internal/rooms` - Room management for cross-room combat
- `internal/util` - Dice rolling and random number generation
- `internal/configs` - Configuration for combat behavior and messaging

## Combat Analytics (Stage 30.1–30.2)

### Architecture
The analytics subsystem uses a ring buffer (`eventBuffer`) to capture every
combat action in real time. When the buffer reaches `maxEvents` (configured
via `Analytics.MaxEvents`), the oldest event is dropped (FIFO). A periodic
flush cycle (controlled by `Analytics.FlushIntervalSec`) aggregates the
buffer into an `AnalyticsSummary` and writes it as a single JSONL line to
the configured log path. The log is rotated by lumberjack (50 MB, 10
backups, compressed).

### CombatEvent Fields
- `SourceType` / `TargetType` — `User` or `Mob`
- `AttackType` — e.g. "unarmed", "weapon", "spell", "bash", "kick", "trip"
- `Hit`, `Crit`, `Fumble`, `Backfire`, `Fizzle` — outcome booleans
- `DamageDealt`, `DamageReduced` — integer damage values
- `DefenseUsed` — "dodge", "parry", "block", or ""
- `AttackZScore`, `DefenseZScore` — z-scores from opposed rolls
- `SourcePosition`, `TargetPosition` — "standing", "prone", etc.
- `SourceIsGrappleController`, `TargetIsGrappleController` — booleans
- `RoundNumber` — combat round counter

### AnalyticsSummary Fields
Aggregated totals (hits, misses, crits, fumbles, backfires, fizzles, total
damage), per-attack-type breakdowns (`ByAttackType`), defense success
counts (dodge/parry/block), matchup counts (PvM/MvP/PvP/MvM), position
hit rates, grapple controller hit rates, average z-scores, and round range.

### Recording Functions
- `RecordAttack()` — standard auto-attacks
- `RecordSpecialMove()` — bash, kick, trip, grapple, mutations
- `RecordSpell()` — spell resolution events

### Query Functions (Stage 30.2)
- `GetSummary()` — full aggregated summary of all buffered events
- `GetFilteredSummary(attackType)` — summary filtered to one attack type
- `GetBufferLen()` — current event count in buffer
- `ResetBuffer()` — clears buffer, returns count cleared
- `ExportNow()` — immediate flush to log file
- `GetAttackTypes()` — map of attack type → event count

### Admin Command: `combatstats` (Stage 30.2)
Subcommands: `summary [type]`, `types`, `matchups`, `defense`, `position`,
`reset`, `export`. All output uses `templates.GetTable()` for tabular
display. See `internal/usercommands/admin.combatstats.go`.

## Typical Combat Round Walkthrough (Player vs Mob)

This traces one full round for a player with sword + shield fighting an
armed mob with a shield. The player uses bash on cooldown and occasionally
casts offensive spells. Both characters are standing, not grappled.

### 1. Round Tick Fires: `DoCombat()`

`hooks/NewRound_DoCombat.go` — The engine emits a `NewRound` event each
tick. `DoCombat` is the listener:

```
DoCombat(evt)
  handlePlayerCombat(evt)    // all players act first
  handleMobCombat(evt)       // then all mobs act
  handleAffected(...)        // death/disable resolution
```

### 2. Player's Turn: `handlePlayerCombat()`

`hooks/NewRound_DoCombat.go` — Loops every online user. For each player
in combat:

#### 2a. NoCombat Buff Check
If the player has a `NoCombat` buff flag, skip the entire combat turn
(including shield decay). This check happens before anything else.

#### 2b. Shield Decay
`handlePlayerShieldDecay(user)` — If the player has a `ConditionShield`
(from Minor Shield spell), its duration ticks down. At 0, removed.

#### 2c. Fold Casting Check
`handlePlayerFoldCasting(user, userId)` — If the player typed
`cast fireball` last round, `c.IsCasting()` is true (Activity machine
is in Casting state):

1. Prone/disabled check — breaks concentration immediately.
2. Conviction cost — proportional to folds gained this round:
   `roundCost = (totalConvictionCost * foldDelta) / foldsNeeded`.
   If conviction is too low, the spell fizzles.
3. Fold accumulation — folds double each iteration:
   0 -> 1 -> 2 -> 4 -> 8 -> ... until `foldsNeeded` is reached.
4. When complete, calls `resolveSpell()`:
   - Harm spell vs mob: `spellAttack = Willpower + SpellcastingSkill`,
     opposed roll vs target's defense (usually Willpower).
   - Damage: `CalcRawDamage(Willpower, spellcastingRank,
     spellDmgMult * weaponSpellDmgMult, ChannelMagical)`.
   - Applies `GetMagicalMitigation()` from target's gear.
   - Z-score <= -2.0 = backfire (damages caster).
   - Z-score >= 2.0 = crit (double damage, bypasses mitigation).
5. Triggers spell discovery chance, skill progression.
6. Returns `true` — player skips normal melee this round.

If the player is NOT casting, flow continues to melee.

#### 2d. Aggro Check
If `user.Character.Aggro == nil`, skip (player not in combat).

#### 2e. Cancel Combat-Incompatible Buffs
`CancelBuffsWithFlag(buffs.CancelIfCombat)` — strips buffs like stealth.

#### 2f. Flee Check
`handlePlayerFlee(user, uRoom, userId)` — if the player typed `flee`,
attempts escape. On success, player leaves combat.

#### 2g. PvM Dispatch: `handlePlayerVsMob()`

`hooks/NewRound_DoCombat_helpers.go` — The main player attack sequence:

1. **Target validation** — mob exists, same room (or reachable via exit
   for ranged).
2. **Rounds waiting** — If `RoundsWaiting > 0` (from bash or weapon
   wind-up), decrement and show a preparation message via
   `GetWaitMessages()`. This is how bash costs a round: last round the
   player used bash, it set `RoundsWaiting = 1`, so this round the
   player only sees a flavor message. Skip to mob's turn.
3. **Grapple progression** — `processGrappleProgression()` handles
   clinch to ground advancement.
4. **Moon mods** — `applyMoonMods()` temporarily adjusts stats for
   mutated characters based on moon phase.
5. **THE ATTACK** — `combat.AttackPlayerVsMob(user, defMob)` (see
   Section 3 below).
6. **Post-attack bonuses:**
   - Conviction Surge buff: +15% damage if active.
   - Adrenaline Surge mutation: bonus damage when low HP.
7. **Crit effects** — `applyPvMCritEffects()`: parry crits attempt
   disarm, dodge crits create grapple opportunity.
8. **Apply buffs** from attack result (crit buffs on target).
9. **Dispatch messages** to player, room, defender room.
10. **Mob concentration break** — if the mob was casting and got hit,
    roll Willpower vs damage% to see if concentration holds.
11. **Scripting hook** — `onHurt` mob script fires.
12. **Hostility** — mob's group becomes hostile to the player.
13. **Mob retaliates** — if mob wasn't already aggro'd, it attacks back.
14. **End check** — if either is at 0 HP, end aggro. If player won,
    `handleAutoRetargetPlayer()` finds the next mob.

### 3. The Core Attack: `combat.AttackPlayerVsMob()`

`combat/combat.go` — Wrapper that calls `calculateCombat()` then applies
side effects:

```
attackResult = calculateCombat(user.Character, &mob.Character, User, Mob, ctx)
ChargeAttackCost(user.Character, attackResult.SwingsThrown)  // U7: PER SWING
mob.Character.ApplyHealthChange(-totalDmg)
mob.Character.TrackPlayerDamage(userId, dmg)  // loot attribution
user.Character.OnStatUse("strength")          // progression
user.Character.OnStatUse("dexterity")         // progression
if hit: user.Character.OnSkillUse(combatSkill)
if crit: user.Character.OnCriticalSuccess(combatSkill)
if fumble: user.Character.OnCriticalFailure(combatSkill)
if dualWield: extra OnSkillUse(WeaponCombat)
user.PlaySound("hit-other"/"miss", "combat")  // MSP sound events
```

### 3a. Inside `calculateCombat()` — Step by Step

`combat/combat.go` — Orchestrator calling helpers in `combat_helpers.go`:

**Step 0: StatMod Bonuses**
```
statModDBonus = sourceChar.StatMod("damage")   // flat bonus to damage
extraAttacks  = sourceChar.StatMod("attacks")  // extra attack passes
backstabCrit  = (Aggro.Type == BackStab)        // first pass auto-crits
```

**Step 1: Attack Count** — `calcAttackCount()`
```
attackCount = 1 + floor(Dexterity / 50) + extraAttacks
  e.g., DEX 120: 1 + 2 + 0 = 3 attack passes

Then multiplied by:
  ResourceMultiplier(stamina)    // smooth penalty as stamina drains
  encumbrance penalty            // if overloaded
  minimum 1
```

**Step 2: Collect Weapons** — `collectAttackWeapons()`
- Main hand: sword (ItemId > 0) — added.
- Offhand: shield (Type = Shield, NOT Weapon) — not added.
- Extra arms (mutation): `ExtraArm1`/`ExtraArm2` added as additional weapons.
- Result: 1 weapon (the sword). No dual-wield penalty applies.

**Step 3: Per-pass loop** (3 passes for DEX 120):

For each pass, for each weapon (just the sword):

**Step 3a: Build Weapon Setup** — `buildWeaponSetup()`
```
ws.attacks     = weapon.GetDistributionDamage() attacks (e.g. 2)
ws.baseDmg     = weapon base damage
ws.weaponDmgMult = item's damage_multiplier (e.g. 1.2)
ws.weaponSpeed = item's speed multiplier (e.g. 1.0)
ws.attacks     = GetModifiedAttackCount(attacks, speed)  // skill-modified
ws.attacks    *= c.GetPositionSpeedMultiplier()         // position modifier (Position FSM, chunk 4b R1)
// ConditionRecoveryPenalty: forces attacks = 1
// Racial bonus: weapon.StatMod(RacialBonusPrefix + targetSpecies)
// Hard cap: max 4 swings per weapon per pass
```

**Step 3b: Build Damage Params** — `buildDamageParams()`
```
rawDmg = CalcRawDamage(Strength, combatSkillLevel, weaponDmgMult,
                        ChannelPhysical)
       = Strength * SkillMultiplier(rank) * weaponDmgMult * 0.30

Example at STR=120, rank=10, weaponDmgMult=1.2:
  SkillMultiplier(10) = 1.0 + (3.0-1.0) * sqrt(10/50) = 1.894
  rawDmg = 120 * 1.894 * 1.2 * 0.30 = 81.8

dmgMean = ApplyMitigation(rawDmg, mob.GetPhysicalMitigation(), 0.75)
  e.g., mob has 20% phys mitigation: dmgMean = 81.8 * 0.80 = 65.4

Further multiplied by:
  ResourceMultiplier(health)         // HP-based melee penalty
  ProneAttackMultiplier (if prone)   // 0.80x
  Mutation damage multiplier         // if any
```

`swingDamageParams` carries NO variance field. Spread is derived at the
point of roll, from the mean actually being rolled, by `dice.RollStat`
(`stdDev = mean * RollSpread`). A stored variance cannot stay correct here:
the struct holds two means (`dmgMean` post-mitigation, `rawDmgForCrit`
pre-mitigation), and the modifiers above move both after the fact.

**Step 3c: Per-swing loop** (e.g. 2 swings per pass):

For each swing:

**i. Attack Score** — `calcAttackScore()`
```
attackScore = Dexterity + combatSkillLevel - dualWieldPenalty
            = 120 + 10 - 0 = 130

Then multiplied by:
  ResourceMultiplier(stamina)          // stamina penalty
  ProneAttackMultiplier (if prone)     // 0.80x
  ProneVulnerabilityMultiplier         // 1.15x if target is prone
```

**ii. Fumble Check**
```
initialAttackRoll = dice.RollStat(130)
  mean=130, stdDev = 130 * RollSpread = 19.5
if ZScore <= -2.0: FUMBLE (miss, ~2.3% chance)
```

**iii. Best-of-All Defense** — `runBestOfAllDefense()`

The mob has defenses: dodge (always), parry (has weapon), block (has
shield). All three are rolled simultaneously:

```
For each defense in [dodge, parry, block]:
  1. Track the attempt and bump the stance counter. NOTHING is charged here,
     and there is no affordability gate (U5b-2) — every defence enters the
     contest and only the winner pays, partially, further down.
  2. defenseScore = mob.GetDefenseScore(defenseType)
       dodge: DEX-based
       parry: weapon parry rating
       block: shield block rating
  3. Multiply by effectiveness (DodgeEffectiveness, ParryEffectiveness,
     BlockEffectiveness from config).
  4. Multiply by prone penalties if applicable.
  5. Hand the scores to RunContest: ONE attack roll contested by every defence
     entry, with ContestFloor applied to the outcome. runBestOfAllDefense no
     longer rolls anything (U1) and no longer floors anything itself (U6).
  6. The core returns an ATTACK-positive margin; runBestOfAllDefense flips it
     once at the seam so bestDefenseResult.margin stays DEFENCE-positive
     (margin = defenseRoll.Value - hitRoll.Value).
  7. Keep the defense with the HIGHEST margin.
```

**iv. Resolve Defense** — `resolveDefenseOutcome()`
```
Fumbles first (unchanged, self-relative z <= -2.0), then:

attackWon := best.margin <= 0
  ContestFloor has ALREADY flipped the winner and stamped the +-1 sentinel
  inside RunContest, so this one expression covers floored and unfloored alike.
  There is no second floor here. Do not add one.

if not floored:
  attackWon and attackCrit -> CRIT HIT
  defence won and defenseCrit -> defence crit (disarm / grapple opportunity)
  Crit only ever fires for the side that WON. A floored outcome carries the
  sentinel, not a real margin, so promoting it would hand a decisive result to
  the side that lost the roll. forceCrit (sleeping victim) is exempt from both
  gates: it is decided before the roll, so it forces the win too.

attackWon -> HIT
otherwise -> defence succeeded. Send the defence message, progress the defence
             skill. A floored save names its REAL winning defence, because the
             contest picked one -- the deleted last-resort path always claimed
             a dodge.
```

**v. Momentum** — `sourceChar.UpdateMomentum(hit)` — consecutive
hits/misses affect stance display text.

**vi. If HIT** — `calcHitDamage()`
```
The crit flag is decided during hitroll resolution (see margin_crit.go) and
passed in. calcHitDamage does NOT re-derive it.

  CRIT: damage = dice.RollStat(rawDmgForCrit * critDmgMult)  // PRE-mitigation!
        Apply crit buffs to target.

Normal hit:
  damage = dice.RollStat(dmgMean)  // POST-mitigation
  Round to nearest int, minimum 0.

RollStat derives stdDev from the mean it is given, so each branch gets a
spread proportional to its own mean. Do not reintroduce a shared,
pre-computed variance — the crit branch then inherits the mitigated mean's
(narrower) spread.

For the same reason, chunk 5.11g's critDmgMult multiplies the MEAN, before
the roll. Scaling the rolled result instead would stretch the spread by the
multiplier and make high-skill crits wildly swingier.
```

**vii. Build Messages** — `buildAttackMessages()`
```
Select message template by weapon subtype + damage percentage.
Apply token replacements ({source}, {target}, {itemname},
  {damage description}, {stance}, {position}, {momentum}).
Wrap in *** *** for crits, !!! !!! for fumbles.
Send to attacker, defender, room observers.
```

**viii. Pet Damage** — `applyPetDamage()` — 20% chance the player's pet
joins in with bonus damage.

**Step 4: Accumulate** — all damage from all passes, swings, and weapons
adds up in `AttackResult.DamageToTarget`.

### 4. Mob's Turn: `handleMobCombat()`

`hooks/NewRound_DoCombat.go` — Loops every mob instance:

#### 4a. Pre-checks
Mob alive? Has aggro? Not in NoCombat buff? Load room. Cancel
combat-incompatible buffs. Shield decay (symmetric with player —
inline in `handleMobCombat`, not extracted to a helper).

#### 4b. Fold Casting Check
`handleMobFoldCasting(mob, mobRoom)` — Same fold system as players. If
`mob.Character.IsCasting()` is true (Activity machine is in Casting state),
folds accumulate. On completion, spell resolves via `resolveMobSpell()`.

#### 4c. AI Decision: `handleMobAIDecision()`

`hooks/NewRound_DoCombat_helpers.go` — The mob decides whether to use a
special ability instead of basic melee:

```
if rand(100) < mob.ActivityLevel:
  1. Try ChooseCastAction(mob) -- picks a spell if available, off cooldown
  2. If no spell: ChooseSpecialMove(mob, target) -- bash, grapple, etc.
  3. If chosen: mob.Command(chosenMove), return true (skip melee)

Fallback: CombatCommands list
  if rand(100) < mob.ActivityLevel:
    Pick random CombatCommand, execute, return true
```

The mob might bash the player (using the same `Bash()` function, which
sets `RoundsWaiting = 1` on the mob's aggro), cast a spell, or execute a
scripted combat command.

#### 4d. MvP Dispatch: `handleMobVsPlayer()`

`hooks/NewRound_DoCombat_helpers.go` — If the mob does normal melee:

1. **Target validation** — player exists, same room.
2. **Downed grace** — `handleMobDownedGrace()`: if the player is
   disabled (HP/STA/CONV <= 0), the mob circles for
   `CoupDeGraceRounds` before delivering a finishing blow.
3. **Hidden check** — can't hit hidden players.
4. **Reciprocal aggro** — if player wasn't already fighting this mob,
   set their aggro.
5. **Party auto-attack** — `handlePartyAutoAttack()`: party members
   auto-engage the mob.
6. **Grapple progression** — same as player side.
7. **Target switch AI** — 10% chance the mob switches to a different
   player in the room (requires combat skill >= 30).
8. **Weapon pickup** — if disarmed, tries to equip a weapon from
   inventory.
9. **Rounds waiting** — if mob used bash last round, show wind-up
   message and skip this round.
10. **Moon mods** — apply to the defending player (mutated characters
    get stat boosts).
11. **THE ATTACK** — `combat.AttackMobVsPlayer(mob, defUser)`:
    Same `calculateCombat()` pipeline but with Mob as source, User as
    target. `MobDamageMultiplier` config scales mob damage. Player's
    defense sequence: dodge (DEX), parry (weapon), block (shield) —
    the shield is critical here, it provides `DefenseBlock` with a
    high effectiveness.
    - **Defender progression:** Player gets `OnStatUse("dexterity")`
      for reacting to attacks.
    - **Resource depletion progression:** Moved to regen tick in
      `NewRound_AutoHeal.go` — smooth curve replaces old 25% threshold.
      See `characters/context.md` for details.
12. **Minor Shield reduction** — if player has `ConditionShield`, flat
    damage reduction.
13. **Adrenaline Surge** — mutation check for bonus damage.
14. **Crit effects (defender is player):**
    - Parry crit: player attempts to disarm the mob.
    - Dodge crit: player gets a grapple opportunity.
15. **Charmed mob assist** — charmed mobs in room help the player.
16. **Apply buffs and messages** to player and room.
17. **Concentration break** — if player was casting and got hit,
    Willpower vs damage% check to see if concentration holds.
18. **Offhand break** — chance the player's shield gets damaged.
19. **Mob attacker progression** (Stage 38.3) — if `MobProgressionEnabled`:
    - `mob.Character.OnStatUse("strength")` / `OnStatUse("dexterity")`
    - If hit: `mob.Character.OnSkillUse(combatSkill)`
    - If crit: `mob.Character.OnCriticalSuccess(combatSkill)`
    - If fumble: `mob.Character.OnCriticalFailure(combatSkill)`
20. **End check** — if either at 0 HP, end aggro.

### 5. Resolution: `handleAffected()`

`hooks/NewRound_DoCombat.go` — After all player and mob combat resolves:

**For each affected player:**
```
if Health <= -10 OR Stamina <= -10 OR Conviction <= -10:
  user.Command("suicide")  // death: drops items, land of the dead

else if IsDisabled() (any pool <= 0):
  events.AddToQueue(PlayerDrop)  // drop items, bleeding out state
```

**For each affected mob:**
```
if Health < 1:
  mob.Command("suicide")  // death: drops loot, despawns
```

### 6. Bash Cooldown Flow (Specific Example)

**Round N:** Player types `bash`.
- `usercommands.Bash()` executes immediately (not during the round tick).
- Checks shield equipped, checks `special-move` cooldown.
- Opposed roll: `(WeaponCombat + Strength)` vs `(mobSkill + DEX)`.
- If hit: `CalcRawDamage(STR, weaponCombatRank, BashDamagePercent,
  ChannelPhysical)` with mitigation applied.
- If knockdown roll succeeds: mob goes `PositionProne` for 2+ rounds.
- Sets `user.Character.Aggro.RoundsWaiting = 1` (the attack cost).

**Round N+1:** `handlePlayerVsMob()` fires during the tick.
- `RoundsWaiting > 0`: decrements to 0, shows wind-up message via
  `GetWaitMessages()`. Player does NOT get a normal attack this round.

**Round N+2:** Player attacks normally again (`RoundsWaiting == 0`).
- Full `calculateCombat()` runs.
- If mob is still `PositionProne` from the bash:
  - Player gets `ProneVulnerabilityMultiplier` (1.15x) bonus to attack.
  - Mob defense scores get `ProneDodge/Parry/BlockPenalty`.
  - Mob attack gets `ProneAttackMultiplier` (0.80x) and
    `ProneDamagePenalty`.

### 7. Difficulty Display (`descriptions.go`)

The `GetDifficultyDescription(difficulty int)` function converts spell
difficulty integers (0-75) into qualitative labels for player-facing display:
trivial, simple, moderate, challenging, demanding, formidable, masterwork.
Used in spell UX to communicate challenge without exposing numeric difficulty
values directly.

### 8. File Map After Refactor (Stage 37.1a+)

> **Historical.** This table records the Stage 37.1a refactor and is not
> maintained. The canonical file list is the `## Files` table further down; when
> the two disagree, that one is right.

| File | Contents |
|------|----------|
| `combat/combat.go` | `calculateCombat()` orchestrator (~80 lines), `AttackPlayerVsMob`, `AttackPlayerVsPlayer`, `AttackMobVsPlayer`, `AttackMobVsMob`, `GetWaitMessages` |
| `combat/combat_helpers.go` | Extracted helpers: `calcAttackCount`, `collectAttackWeapons`, `buildWeaponSetup`, `buildDamageParams`, `calcAttackScore`, `calcCritThreshold`, `calcDualWieldPenalty`, `filterDefensesForThirdParty`, `runBestOfAllDefense`, `resolveDefenseOutcome`, `calcHitDamage`, `buildAttackMessages`, `applyPetDamage` |
| `combat/damage_pipeline.go` | `CalcRawDamage`, `ApplyMitigation`, `SkillMultiplier`, `ResourceMultiplier`, `MitigationCap`, `DamageScale` |
| `combat/crit_damage.go` | `CritDamageMultiplier`, `CritOrMitigatedDamage` |
| `combat/margin_crit.go` | `ContestCrit`, `ContestCritThreshold` |
| `combat/crit_floor.go` | `ApplyCritFloor`, `AttackContestCrit`, `DefenseContestCrit`, `AttackCritFloor`, `DefenseCritFloor` |
| `combat/attackresult.go` | `AttackResult` struct (includes `Hit`/`CleanHit` — dealt damage vs. won the contest, see the Task 10/14 section — `DefenseAttempts`, `AttackZScore`, `DefenseZScore`, `ParryCritDetected`, `DodgeCritDetected`) and message helpers |
| `combat/ai.go` | `ChooseSpecialMove`, `ChooseCastAction`, `GetAIProfile`, AI profiles, viability checks (`CanUseBash`, `CanUseKick`, etc.), scoring functions |
| `combat/criteffects.go` | `AttemptCritDisarm`, `SetGrappleOpportunity`, `HasGrappleOpportunity`, `GetGrappleOpportunityBonus`, `ClearGrappleOpportunity` |
| `combat/grapple.go` | `AttemptGrapple`, `ApplyGrappleResult`, `CheckClinchProgression`, `CheckGroundedEscape`, `ApplyPositionProgression`, `IsThirdPartyAttack` |
| `combat/grapple_move.go` | `ExecuteGrappleMove`, `GrappleMoveResult`, `GrappleMoveDisarmWeapon` |
| `combat/skill_moves.go` | `ExecuteSkillMove`, `SkillMoveResult`, `SkillMoveParams` |
| `combat/calculations.go` | Hit chance, crit probability, power ranking, alignment calculations |
| `combat/descriptions.go` | `GetDamageDescription`, `GetHealDescription`, `GetDifficultyDescription` helpers |
| `combat/taunt_messages.go` | Taunt/conviction combat messages |
| `combat/analytics.go` | Ring buffer, `CombatEvent`, `AnalyticsSummary`, recording + query functions |
| `hooks/NewRound_DoCombat.go` | `DoCombat`, `handlePlayerCombat` (~50 lines), `handleMobCombat` (~50 lines), `processGrappleProgression`, `handleAffected`, `applyMoonMods` |
| `hooks/NewRound_DoCombat_helpers.go` | All extracted helpers: `handlePlayerShieldDecay`, `handlePlayerFoldCasting`, `handleMobFoldCasting`, `handlePlayerFlee`, `handlePlayerVsPlayer`, `handlePlayerVsMob`, `handleMobVsPlayer`, `handleMobVsMob`, `handleMobAIDecision`, `handleMobTargetSwitch`, `handleMobWeaponPickup`, `handleMobDownedGrace`, `handlePartyAutoAttack`, `handleCharmedMobAssist`, `handleAutoRetargetPlayer`, `handlePlayerConcentrationBreak`, `dispatchCombatMessages`, `handleOffhandBreakUserDef`, `handleOffhandBreakMobDef` |
| `hooks/combat_shared_helpers.go` | `simulateFoldRound`, `calcFoldConvictionCost`, `advanceFolds`, `checkConcentrationBreak`, `tryWeaponBreak`, `applyCritEffects`, `CritEffectResult`, `calcSpellDamageForCharacter` |
| `hooks/spell_resolution.go` | `resolveSpell`, `resolveAgainstMob`, `resolveAgainstPlayer`, `applyPlayerEffect` |

---

## Position hit modifiers (chunk 4e)

After `calcAttackScore` returns, `applyPositionHitModifiers(source, target)`
multiplies the score by the two `internal/state/position/modifiers.go`
lookups (attacker-self × target-side). Both default to 1.0 outside grapples,
so standing-vs-standing combat is mathematically unchanged.

In-grapple effect: Mount controller swinging at controlled = 1.32×;
third party attacking a mounted defender = 1.20×; mounted defender
swinging back = 0.74×.

### Outside-damage hooks (chunk 4e §5 + §7)

After each Attack* function applies damage to its target, two hooks
fire at the bottom of the damage block (guarded by DamageToTarget > 0):

- `chunk4eApplyOutsideHitDisruption(attacker, target)` — if target is a
  grapple controller AND attacker is not the partner (per
  `IsThirdPartyAttack`), shifts target's Control state one step toward
  Neutral. Deduped per round via `Character.OutsideHitDisruptedRound`.
  Gated by `Balance.ControlDegradeOnOutsideHit`.

- `chunk4eAccumulateSubInterruptDamage(attacker, target, damage, isCrit)` —
  if attacker is a third party AND the hit is a crit OR damage ≥
  `SubInterruptDamageThresholdPct × HealthMax`, adds to
  `Character.SubInterruptDamageThisRound`. Position_SubmissionTick reads
  the accumulator; if > 0 when a sub fires, forces Bad-tier outcome.

---

## Weapon Reach Utility (chunk 4c)

When a character is grappling, long weapons cannot be swung freely —
the haft catches the attacker's own body. Chunk 4c adds a multiplicative
damage penalty that scales with how much a weapon's reach exceeds the
grapple's effective radius. Short weapons (daggers, fists) pay no
penalty; polearms in mount are severely degraded. All logic lives in
`internal/combat/reach.go`.

### Functions

**`PositionReachRadius(s position.State) float64`**
Returns the effective constraint radius (meters) for a given position:
- Non-grapple states → 0.0 (no penalty, any weapon)
- Clinch, BackStanding → `ReachStandingGrappleRadius` (default 0.5 m)
- All ground grapple states (Mount, Guard, etc.) → `ReachGroundGrappleRadius`
  (default 0.3 m)

**`ReachUtility(weaponReach, posRadius float64) float64`**
Returns `min(1.0, posRadius / weaponReach)`, floored at `ReachUtilityFloor`
(default 0.15). A dagger (0.30 m) in mount (radius 0.30 m) scores 1.0 —
full damage. A sword (1.00 m) in mount scores 0.30 — 30% of normal.

**`ShouldBludgeon(weaponReach, posRadius float64) bool`**
True when `weaponReach > posRadius` (i.e., weapon exceeds grapple radius).
Drives the narration swap described below.

**`CalcReachAdjustedItemMult(weapon Item, attacker *Character) float64`**
Pipeline-integration helper called in `combat_helpers.go:buildWeaponSetup`
for every swing. Resolves weapon reach (via `items.ResolveReach`), reads
the position radius via `PositionReachRadius(attacker.Position.State())`,
and returns the adjusted `DamageMultiplier` (weapon's base multiplier
scaled by `ReachUtility`). Natural-attack paths (mob unarmed, claws, bite)
use `items.ResolveNaturalReach(subtype)` directly and do not go through
this helper.

### Bludgeon narration

When `ShouldBludgeon` fires in `combat_helpers.go:buildAttackMessages`,
the message subtype is swapped to `Bludgeoning` before calling
`items.GetAttackMessage`. Effect: "you slam the iron sword's pommel into
the bandit's ribs" instead of slashing narration. Damage math is unchanged;
the swap is cosmetic only.

**Exempt subtypes (no swap):**
- Natural-blunt: Fist, Claws, Bite, Sting, Slam, Gore, Whipping
- Caster: Wand, Sceptre, Staff

**Affected subtypes (swap fires):** Slashing, Cleaving, Stabbing, Shooting.

### Balance knobs (`Balance` section in config)

| Knob | Default | Effect |
|------|---------|--------|
| `ReachStandingGrappleRadius` | 0.5 m | Constraint radius for Clinch / BackStanding. Weapons longer than this are penalised. |
| `ReachGroundGrappleRadius` | 0.3 m | Tighter constraint for ground grapples (Mount, Guard, etc.). |
| `ReachUtilityFloor` | 0.15 | Minimum damage multiplier from the reach curve. Prevents total nullification. |

---

## Submission System (chunk 4d)

The submission system is an automatic, engine-fired mechanic — no player
command triggers it. Once per grapple round, after `Position_GrappleTick`
stashes the drift-roll snapshot, `Position_SubmissionTick.go` checks
whether either side of the pair has a sub-attempt window open and, if so,
rolls a fresh opposed check and applies the outcome.

### Key files

- `internal/combat/submission.go` — `RollSubmissionAttempt`, tier
  classification (`ClassifySubmissionTier`), `SubmissionTier` enum,
  `SubmissionAttemptResult` struct, `Role` enum (RoleTop / RoleBottom).
- `internal/combat/submission_outcome.go` — `ResolveSubmissionOutcome`,
  policy dispatch helpers (`applyBadTier`, `applyMercyRelease`,
  `applyDeathCascade`, `applyBrokenLimbBuff`, `applyStunnedBuff`).
  Also houses `RegisterSubmissionMessaging` — the callback registration
  point for T11 narration hooks (avoids a combat → hooks import cycle).
- `internal/hooks/Position_SubmissionTick.go` — per-round observer.
  Iterates active characters, gates on the controller side, calls
  `EvaluateSubAttempt` to decide role + eligibility, then calls
  `RollSubmissionAttempt` + `ResolveSubmissionOutcome`.
  See `internal/hooks/context.md` "Position_SubmissionTick" for the
  full observer walkthrough.

### Roll formula

`RollSubmissionAttempt` fires a SEPARATE opposed roll from the chunk-4b
drift roll. Drift gates the opportunity window; this roll resolves the
attempt:

```
attackerScore = attempter.Strength
              + attempter.UnarmedCombatSkill × SubSkillWeight
defenderScore = recipient.Strength
              + recipient.Vitality
              + recipient.UnarmedCombatSkill × SubSkillWeight
```

Both sides are rolled by the shared contest core: `RollSubmissionAttempt`
calls `RunContest(atkScore, []contest.Entry{{Score: defScore}})`, so the
one symmetric floor applies here like everywhere else. The attacker's
z-score determines the tier (see below).

Note the skill weight: the sub roll multiplies unarmed combat by
`SubSkillWeight` (1.5) on **both** sides. That is its own regime, shared
with nothing else. See "Contest core (chunks U1 to U6)" above.

### Tier classification

`ClassifySubmissionTier(success bool, attackerZ float64) SubmissionTier`
maps the roll result to one of four tiers:

| Tier | Condition | Effect |
|------|-----------|--------|
| `SubTierBad` | Attacker failed AND attackerZ < `SubBadZThreshold` | Attempter falls Prone; grapple breaks to Standing. |
| `SubTierNeutral` | Attacker failed, z >= threshold | No effect; grapple continues. |
| `SubTierSuccess` | Attacker succeeded, z < `SubCritZThreshold` | Apply attempter's `SubmissionPolicy`. |
| `SubTierCrit` | Attacker succeeded, z >= `SubCritZThreshold` | Apply policy + apply Stunned buff (id 84) to recipient when policy is mercy. |

### Policy outcome ladder

`ResolveSubmissionOutcome` dispatches to per-policy helpers based on
`attempter.SubmissionPolicy`:

| Policy | Outcome |
|--------|---------|
| `mercy` | Clean grapple break; both return to Standing. Crit: recipient gets 1-round Stunned buff (id 84). Honors `SurrenderPolicy` tap signal. |
| `subdue` | Death cascade with `NoDeprogression = true`, `GoldLossFraction = SubGoldLossFraction`. Defender wakes at temple with no stat decay. |
| `cripple` | Same as subdue + broken-limb buff (id 83) applied to the body part targeted by the submission type. Choke subs (RNC, Triangle, Anaconda) degrade to subdue because chokes don't break limbs. |
| `lethal` | Full death cascade; `NoDeprogression = false`. Standard stat decay applies. |

**Note on SurrenderPolicy:** Only `mercy` policy consults the defender's
tap signal. Subdue / cripple / lethal proceed regardless of the defender's
`SurrenderPolicy`. This is a deliberate realism call — a killer doesn't
stop because you tap.

### Choke-degradation rule

When `SubmissionPolicy == PolicyCripple` and the sub type is a choke
(`position.CrippleBodyPart(subType) == ""`), `effectivePolicy` degrades
to `PolicySubdue`. This prevents choke-class subs from triggering the
broken-limb buff that requires a physical joint target. The degradation
applies silently — the attempter intended cripple but the choke just
subdues.

### Bottom-sub asymmetry

Top-subs (controller side) open when `MarginAttacker > SubmissionAttemptAlpha`.
Bottom-subs (controlled side) open when the DEFENDER's margin exceeds
alpha OR when `DefenderZScore >= SubmissionAttemptCritZ`. The crit
shortcut lets a dominated-but-lucky controlled fighter occasionally fire
a reversal even when consistently losing drift rolls — by design sparser
than top-subs.

### New buffs

| ID | Name | Duration | Source | Effect |
|----|------|----------|--------|--------|
| 83 | Broken Limb | ~3600 rounds (~1 hr play) | Cripple sub outcome | Reduces combat effectiveness; persists across respawn; cannot be dispelled early |
| 84 | Submission Stunned | 1 round | Crit sub tier (mercy policy only) | Brief combat stagger; auto-clears next round |

### Life cascade integration

Subdue and cripple submissions call `victim.Life.TransitionToDead`
directly (not `victim.Die`) so the new `DeadData` fields can be
populated before observers fire:

- `NoDeprogression bool` — `true` for subdue/cripple; `Death_PlayerCleanup`
  skips stat-decay when this is set.
- `GoldLossFraction float64` — set to `SubGoldLossFraction` (default 0.20);
  `Death_PlayerAnnouncement` transfers this fraction of the defender's
  gold to the attacker.

Lethal submissions use `victim.Die()` with `NoDeprogression = false` —
the normal decay path.

`TriggerSubmission` is the trigger constant added to `internal/state/life/`
transitions in T7. See `internal/state/life/context.md` for the
`DeadData` struct documentation.

### Balance knobs (`Balance` section in config)

Six knobs control submission window eligibility and tier classification.
The `SubSkillWeight` default is 1.5 (not 1.0 as the T3 plan assumed —
updated at validation time).

| Knob | Default | Effect |
|------|---------|--------|
| `SubmissionAttemptAlpha` | 1.0 | Min drift-margin z-score to open a sub window on either side of the grapple. |
| `SubmissionAttemptCritZ` | 2.0 | Defender drift z >= this opens a bottom-sub window regardless of margin. |
| `SubSkillWeight` | 1.5 | Unarmed-combat skill contribution multiplier in the sub roll. |
| `SubBadZThreshold` | -1.0 | Sub-roll z-score below which the bad tier fires (attempter falls Prone). |
| `SubCritZThreshold` | 2.0 | Sub-roll z-score at or above which the crit tier fires (recipient stunned). |
| `SubGoldLossFraction` | 0.20 | Fraction of defender's carried gold transferred to attacker on subdue/cripple. |

See `internal/configs/context.md` "Submission System (chunk 4d)" for
the config-level documentation and `internal/state/position/context.md`
for the submission-type mapping and eligibility predicates.

---

## First-hit crit on sleepers (chunk 3.3)

The damage pipeline accepts a `forceCrit bool` parameter on
`resolveDefenseOutcome` (in `combat_helpers.go`) that bypasses
the Z-score threshold and bumps to threshold+0.5 (clearly-crit).

`handleCombatRound` receives `forceCrit` from
`NewRound_DoCombat.snapshotSleepingVictims()` — a start-of-round
snapshot of all players + mobs with the Sleeping flag. All
damage events in the round against snapshotted victims force-crit,
even after cancel-on-damage clears the buff mid-round.

The snapshot approach means: the sleeper is asleep at round start →
the entire first round crits → the first hit wakes them
(`CancelOnDamage` fires) → subsequent hits in the same round still
force-crit from the snapshot, but the sleeper is now active and can
start fighting back next round.

Other future first-hit-crit triggers (surprise attack, backstab)
can add parallel snapshot checks at the same start-of-round site.

## Files

| File | Purpose |
|------|---------|
| `combat.go` | Round resolution entry points. **`calculateCombat` takes both combatants as POINTERS (U7 Task 1). See the gotcha under "Contest core"; value parameters silently switched the whole melee defence cost model off.** The four wrappers charge the attacker after it returns, via `ChargeAttackCost(char, attackResult.SwingsThrown)`. |
| `combat_helpers.go` | Extracted helpers. **`runBestOfAllDefense` no longer rolls — it builds defence scores and delegates to `internal/contest` (U1). It performs the one sign conversion between the core's attack-positive margin and `bestDefenseResult`'s defence-positive one.** |
| `damage_pipeline.go` | The unified three-channel damage + mitigation pipeline |
| `margin_crit.go` | Normalized opposed-roll margin, the source of the crit flag. `normalizedAttackMargin`/`normalizedDefenseMargin` serve melee (5.11d); `ContestCrit` serves spell + conviction (5.11g). **The two take opposite margin sign conventions — read the doc comments before touching either.** |
| `crit_floor.go` | Crit floors, 1% both directions (5.11e). **U6 Task 9 changed the DENOMINATORS: the attack floor applies to swings that WON THE CONTEST and the defence floor to swings the DEFENCE won, keyed on `best.margin` (defence-positive, so `<= 0` is an attack win), not on `res.hit`.** The old hit/miss split stops being answerable once a defensive win deals partial damage, because a deflected swing then has `res.hit == true` while the defence won. A floored outcome and an uncontested swing (`defenseType == ""`) are promoted by neither floor. **`applyCritFloors` must stay the LAST thing `resolveDefenseOutcome` does** — an attack crit forces a hit, so flooring earlier becomes an undeclared second hit floor stacked on `ContestFloor`. **U6 Task 10:** a promotion to a defence crit now also clears `res.hit` and `res.damageMult`, because an ordinary defensive win arrives here already landing partial damage. |
| `defence_multiplier.go` | `DefenceMitigation` — the margin-scaled damage reduction a defensive win now earns (U6 Task 10). 50% at a bare win, 100% at `ContestCritThreshold`. Its 0.5 and its threshold are STRUCTURAL, not config knobs: the threshold is the point the curve has to meet so that full negation by a defensive crit is continuous with it rather than a cliff. Also `ResolveChannelDefence` / `ChannelAttackScore` / `AwardDefenceProgression` (U6 Task 12) — the resolver the spell and social channels use in place of the deleted `avoidance.go`. **U6 Task 13** extracted `defenceDamageMultiplier(res contest.Result) float64` from `ResolveChannelDefence`'s tail — it converts a finished opposed contest into the attacker's damage multiplier (1.0 attack win, 0.0 defensive crit, exactly 0.5 on a floored save, 0.0-0.5 off the curve otherwise) and is now the ONE place the sign negation, the floored sentinel, and the sqrt(2) normaliser live; both `ResolveChannelDefence` and `skill_moves.go`'s `ExecuteSkillMove` call it. |
| `crit_damage.go` | `CritDamageMultiplier` (skill-scaled crit worth) and `CritOrMitigatedDamage` (5.11g) |
| `calculations.go` | Core combat maths |
| `run_contest.go` | `RunContest`, the single entry point for every opposed contest, wrapping `internal/contest`. The one place `Balance.ContestFloor` is read. U6 deleted the three floor-pair wrappers this replaced. |
| `defence_sets.go` | `AttackChannel` + `DefenceSetFor` — which defences apply to which attack type, as data (U6 Task 11). Consumed by `ResolveChannelDefence` for the three non-melee channels; melee still builds its own `defSeq`. See "Defence sets are a property of the channel" below. |
| `attack_cost.go` | `ChargeAttackCost(attacker, swings)` — the attacker-side price, U7 Task 7. One swing costs `AttackBaseStaminaCost` × encumbrance × inverse-skill × `AttackCostModifier` through `costs.Calc`, the same composition the five defences use, charged `× swings` through `ApplyCostFloat`. **This replaced a once-per-round `DeductAttackStamina` call in each of the four wrappers**: a twelve-swing build attacked twelve times for the price of one while the defender paid on every incoming swing, which is what made offence effectively free next to defence. Skill rank comes from `GetCombatSkillLevel` (weapon-appropriate, minimum 1), not the registry's nominal `skills.WeaponCombat`. A nil attacker or non-positive swing count charges nothing and is not `Short`. |
| `attackresult.go` | The result value passed back to callers. **`SwingsThrown`** counts every swing resolved in the round ACROSS ALL WEAPONS and, like `Hit`/`CleanHit`, is never cleared by the per-swing flag reset (which clears `Crit`/`Fumble`/`DoubleFumble` only) — `ChargeAttackCost` prices the round off it. |
| `criteffects.go` | Critical and fumble effects |
| `descriptions.go` | `GetDamageDescription` / `GetHealDescription` — descriptive, never numeric |
| `skill_moves.go` | Skill-driven combat moves (bash/trip/kick/...). **U6 Task 13:** `ExecuteSkillMove` scales damage through `defenceDamageMultiplier` instead of gating it on `attackSuccess` alone, so `SkillMoveResult.Hit == false` with `Damage > 0` is a legal pair (a defended attempt still lands partial damage), and `Damage` is the contest-scaled amount actually applied to the defender's health pool, not the unscaled base. `SkillMoveResult` gained `StatusApplied bool`, which stays binary — true only when `Hit == true`. |
| `grapple.go` / `grapple_move.go` | The grappling state machine and transitions |
| `submission.go` / `submission_outcome.go` | Submissions and their resolution |
| `reach.go` | Weapon reach and its interaction with clinch |
| `flee.go` / `flight.go` | Disengaging and flight movement |
| `taunt_messages.go` | Rhetoric channel messaging |
| `ai.go` | Combat-side AI helpers |
| `analytics.go` | Combat statistics collection |

All damage flows through `damage_pipeline.go`. Never emit a raw number to a
player — use `descriptions.go`.
