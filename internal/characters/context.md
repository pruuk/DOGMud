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

### The roll resolution and `ProgressionChanceFloor` (U10b-0 Phase B)

Every progression roll uses `progressionRollDenominator` (1,000,000) rather than
the 10,000 it used to. At 10,000, any chance below 0.01% produced a threshold of
exactly zero: the stat was not slow, it was **sealed**, and could never progress
again. Measured against `_datafiles/world/dogmud/users/3.yaml` with the shipped
config, two of that character's six stats were in that state (strength
3.98e-05, dexterity 9.25e-12).

`Balance.ProgressionChanceFloor` (shipped 1e-5) is applied as the last step of
`ProgressionChanceForStat` and of `ProgressionChanceForSkill`, after
every multiplier.

**Both halves are required and each fixed a different stat.** Resolution alone
revived strength (threshold 39) but left dexterity dead; the floor is what
revived dexterity (threshold 10). At the old resolution the floor itself would
have quantised to zero.

Two things to preserve if you touch this:

- **`chance > 0` guards both floors.** The mob hard-cap short-circuits return a
  genuine zero meaning "this mob may not progress at all", and the floor must
  not resurrect that. Pinned by
  `TestStatProgressionChance_FloorDoesNotResurrectAHardZero`.
- **The floor is applied in the chance expression, not at the roll site**, so
  every caller of that expression — `CheckStatProgression`, `OnCritReceived`,
  the faucet test — sees the same guarantee.

The knob's validator uses `<= 0`, not the `< 0` idiom the deliberate
off-switches use, because a config that omits it must get the floor back rather
than lose it. That distinction is not academic: `ObservedCritProgressionBonus`
uses `< 0` and is therefore sitting at 0 on any config that omits the key.


### Regen-Based Stat Progression
Every regen tick (every 3 rounds), each resource pool has a small chance to
trigger stat progression based on how depleted it is. This replaced the old
hard 25%-threshold `OnLowResource` system.

**Formula:** `chance = RegenProgressionBase × (1 - current/effectiveMax) ^ RegenProgressionCurve`

**Config knobs:** `RegenProgressionBase` (default 0.01), `RegenProgressionCurve` (default 3.0)

**Resource → Stat Mappings:**
- Health → Vitality, Willpower (enduring injury toughens body + mind)
- Stamina → Strength, Vitality (exertion builds power + endurance)
- Conviction → Willpower, Charisma (mental strain sharpens will + presence)

The existing `StatProgressionMultipliers` still apply on top.
Mob progression uses `MobProgressionRate` as a multiplier.

**Key methods:**
- `OnRegenTick(pool Pool, relatedStats []string, userId int)` — derives the chance, calls CheckRegenProgression per stat
- `regenTickChance(pool Pool) float64` — the chance one tick presents, or 0 for no roll
- `CheckRegenProgression(statName, userId, chance)` — applies mob gating, multipliers, rolls

**The ratio measures the REACHABLE pool, not the raw one (U10b-0 Phase B).**
A character sitting at their reserved cap is not depleted, they are full, so the
denominator is raw max minus `GetPoolReservation`. Reading the raw max there
turns a large reservation into a permanent progression farm — the fyttyn
vitality exploit, 2026-04-16.

`OnRegenTick` takes a `Pool` rather than a `(current, max)` pair specifically so
no caller can get that wrong. Six call sites used to compute the adjusted max by
hand, correctly, with nothing pinning it.

Two traps if you touch this:

- **Do not reach for `EffectivePoolMax`.** It is floored at 1 and never returns
  0, so a fully reserved pool would present `max=1, current=0, ratio=0` — the
  *maximum* chance, permanently. That is the same faucet inverted.
  `regenTickChance` returns 0 for a pool with nothing reachable in it, pinned by
  `TestRegenTickChance_TotallyReservedPoolIsZeroNotMaximum`.
- **Only the progression RATIO moved to the effective max.** The regen AMOUNT
  (`HealthPerRound` / `StaminaPerRound` / `ConvictionPerRound` in
  `resources.go`) deliberately still reads the raw max. See `EffectivePoolMax`'s
  own doc comment.

**Regen progression is NOT floored.** `ProgressionChanceFloor` applies to the
two rank-driven sites only. This chance is deliberately proportional to
depletion and is *supposed* to vanish as the pool fills; flooring it would lift
a near-full-pool chance by orders of magnitude.

**U9: `CheckRegenProgression` now damps by rank.** It used to derive its
chance from pool depletion alone and never fall no matter how much the stat
had already grown, which let a character grind a stat (vitality especially,
since regen is its only progression source) at low health forever -- the
`fyttyn` mechanism (`internal/migration/0.16.0.go`). The unexported
`regenDamperFactor(statName) float64` multiplies the depletion chance by
`CalculateProgressionChance(virtualRank, StatProgressionSoftCap) /
BaseProgressionChance`, so it is exactly `1.0` at rank 0 (a fresh character's
passive growth is unchanged) and falls as the stat's rank climbs. **Rank is
`StatInfo.Training` since U10b-0 Phase C**, not a use counter. It
returns `0` if `BaseProgressionChance <= 0` rather than dividing by zero.
`OnRegenTick` now calls `TrackStatUse` for each related stat *before* rolling,
which is the load-bearing half -- without it vitality's rank stayed at 0
regardless of value, since nothing else in production calls
`OnStatUse("vitality", ...)`.

### The Contest Progression Seam (U9)

`internal/progression` is the pure event layer for contest-driven
progression (melee and the three channel defences: quell, defy, spell). It
computes `[]progression.Event` from the plain facts of one resolved contest
and fires nothing itself. `characters.ApplyProgression` is the single place
those events get applied to a real character -- see
`internal/progression/context.md` for the full event/outcome model.

```go
// ApplyProgression applies every event belonging to one side (an Outcome
// produces events for both sides; callers invoke this once per side). round
// is util.GetRoundCount(); pass 0 for non-combat callers to always claim.
func (c *Character) ApplyProgression(events []progression.Event, side progression.Side, userId int, round uint64)

// ClaimedBonusThisRound reports whether a bonus (crit/fumble/observed) event
// already fired for this skill this round, for any class. Exported so tests
// in internal/combat and internal/hooks can assert the bonus tier ran, since
// bonus events leave no other observable trace (they deliberately do not
// always track a use count -- see the Gotchas below).
func (c *Character) ClaimedBonusThisRound(skillName string) bool

// ToughenStatFor maps a damage channel to the stat RECEIVING a crit in that
// channel trains: "physical" -> vitality, "magical" -> willpower,
// "conviction" -> charisma. Exported because internal/combat and
// internal/hooks fill Outcome.ToughenStat with it. Unrecognised channel
// returns "".
func ToughenStatFor(damageChannel string) string
```

`OnCritReceived(damageChannel, userId)` still exists, but as of U9 it has
**zero production callers** -- every former call site (the player- and
mob-caster magical-crit branches in `internal/hooks/spell_resolution.go`,
and the parallel conviction site in `internal/actions/combat_taunt.go`) was
rewritten to build a `progression.Outcome{ToughenStat: ...}`, take
`progression.BonusEvents`, and call `ApplyProgression` directly instead -- see
the seam guard note below. The method's own implementation was also moved OFF
`CheckRegenProgression` onto `CheckStatProgression` (the decayed curve) and
now calls `TrackStatUse` before rolling, for the same rank-independence
reason as the regen damper above; it is kept as a documented API for any
future non-contest crit-received site, not because anything calls it today.

**`OnCriticalSuccess` and `OnCriticalFailure` were DELETED in U9.** Anything
that still cites them is describing pre-U9 behaviour. Crit and fumble
progression for contest paths now flows exclusively through
`progression.BonusEvents` + `ApplyProgression`.

#### Gotchas (progression seam)

- **Contest-path callers (`internal/combat`, `internal/hooks`) may NOT call a
  progression primitive directly.** `internal/progression/seam_guard_test.go`
  is an AST test that fails any call to `OnSkillUse`, `OnSkillUseScaled`,
  `OnStatUse`, `CheckSkillProgression`, `CheckStatProgression`,
  `OnCritReceived`, `OnCriticalSuccess`, `OnCriticalFailure`,
  `TrackSkillUse`, `TrackStatUse`, or `CheckRegenProgression` from those two
  packages outside a small, commented allow-list. Route through
  `progression.Outcome` + `ApplyProgression` instead. The guard does not walk
  `internal/actions`, so `combat_taunt.go`'s conviction crit site is routed
  the same way by convention, not by enforcement.
  The ~93 non-contest call sites elsewhere (craft, salvage, forage, search,
  steal, and the rest) are deliberately untouched by U9 and are not covered
  by this guard; routing them is a later slice's job.
- **`ApplyProgression`'s ordinary path is NOT a new code path.** For
  `progression.ClassOrdinary` events it calls `OnSkillUseScaled` (tracking,
  mutation cluster drift, the `SkillUsed` quest event, the primary-stat roll)
  exactly as pre-U9 call sites did directly. Only bonus-class events go
  through the new `applyBonusProgression` path.
- **Bonus events track a use count ONLY for `progression.ClassObserved`.**
  This dates from when rank came from the use counter, and
  `CalculateProgressionChance` is monotonically DECREASING in rank, so tracking
  a crit or fumble would have punished the very achievement the bonus rewards.
  **Since U10b-0 Phase C rank is `Training` / the skill level, so tracking no
  longer affects difficulty at all** -- the counters are telemetry. The
  asymmetry is retained because the dashboard still reports those counts, but it
  no longer has a balance consequence. The observed party has no achievement to
  punish, and for the crit-received toughening stat specifically, tracking
  is the ONLY thing that ever moves that stat's rank (nothing else calls
  `OnStatUse` for e.g. vitality). This is enforced in the unexported
  `applyBonusProgression`, keyed off `ev.Class == progression.ClassObserved`.
- **Bonus events are deduped once per round per (skill, stat, class) via the
  unexported `claimBonusProgression`**, backed by the unexported
  `bonusProgressionRound map[string]uint64` field on `Character`
  (transient, not persisted). Ordinary per-swing events are deliberately NOT
  deduped -- only the bonus tier needed protection from a margin-driven crit
  rate firing on nearly every swing of a lopsided fight. A `round` of `0`
  always claims, so non-combat callers are never silently suppressed. The key
  includes the stat, not just the skill, because an observed crit and an
  observed fumble on the same skill can carry different stats (toughening
  stat vs. defence stat) and must not consume each other's slot.
- **`ApplyProgression` rolls a stat a second time only when `ev.Stat` differs
  from the skill's own primary stat** (`skills.GetSkillPrimaryStat(ev.Skill)`).
  `OnSkillUseScaled` already rolled the primary stat, so this guards against a
  double roll for the common case and only fires for the two cases that
  genuinely diverge: a spell's own `primarystat` override, and the
  crit-received toughening stat.

### Equipment System (`worn.go`)
- **Equipment slots**: Weapon, Offhand, Head, Neck, Body, Belt, Gloves, Ring, Legs, Feet
- **Stat modifications**: Equipment provides stat bonuses aggregated across all slots
- **Item management**: Worn item tracking and validation

### Character States and Modifiers
- **Aggro system** (`aggro.go`): Combat targeting and threat management
- **Buffs integration**: Status effects that modify character capabilities
- **Cooldowns** (`cooldowns.go`): Time-based ability restrictions.
  `CooldownReady` is the read-only admission query; `TryCooldown` consumes only
  after successful admission.
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

// Same value, keyed by the pool's plain name, FOR TEMPLATES ONLY. text/template
// cannot convert an untyped string constant to Pool, so the typed form above is
// unreachable from status.template.
func (c *Character) EffectivePoolMaxNamed(pool string) int

func (c *Character) ApplyCost(pool Pool, amount int) bool
func (c *Character) ApplyCostPartial(pool Pool, amount int) CostResult

// Quote is read-only; commit is owner-bound, snapshot-validated, and single-use.
// The valid owner's first commit attempt consumes the quote, even if snapshot
// validation rejects it; a wrong-owner attempt cannot consume it.
func (c *Character) QuoteActionCost(req ActionCostRequest) CostQuote
func (q CostQuote) Affordable() bool
func (c *Character) CommitCost(q CostQuote, policy CostPolicy) CostCommitResult

// CostFullOrRefuse is atomic refusal; CostPartial writes off unpaid whole debt.
// Status is CostNoCharge, CostPaid, CostPartiallyPaid, or CostRefused.
func (r CostCommitResult) Short() bool

// ApplyCostFloat charges a FRACTIONAL cost, banking the sub-integer remainder
// in the per-character, per-pool carry so the average converges (U7 Task 3).
// It delegates the deduction to ApplyCostPartial and remains available for
// legacy or specialized callers. Registered action costs use the read-only
// QuoteActionCost path above and CommitCost for atomic admission and charging;
// both paths retain fractional carry. See Gotchas.
func (c *Character) ApplyCostFloat(pool Pool, amount float64) CostResult

// ApplyCostFloatOrRefuse banks the remainder exactly as ApplyCostFloat does but
// pays the whole part IN FULL or refuses outright. The affordability decision is
// taken before anything is written, so a refused action leaves both the pool and
// the carry untouched and cannot accumulate debt. Movement is the caller.
func (c *Character) ApplyCostFloatOrRefuse(pool Pool, amount float64) bool

// Does not initialize or prune Cooldowns.
func (c *Character) CooldownReady(trackingTag string) bool

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
- **Cost policy is explicit at commit.** U8 voluntary named actions use
  `QuoteActionCost` plus `CommitCost(..., CostFullOrRefuse)` before secondary
  state. Autoattack, the winning defence, flee, and grapple maintenance use
  `CostPartial` because exhaustion must weaken rather than suppress those
  life-preserving resolutions. Lower-level `ApplyCost*` methods remain for
  legacy and non-U8 consumers; do not bypass the action quote for a registered
  action.
- **A green pool-mutation guard does not mean every pool write is routed.**
  `resources.go` is exempt as a FILE, so `Heal()`'s writes are invisible and so
  are its three production callers (`actions/combat_drain.go:126`, `:281`,
  `hooks/item_procs.go:99`). They retire with `Heal` in U5c.
- **`CostCommitResult.Short()` strips only the governing skill term** for a
  partially paid autoattack, winning defence, flee, or grapple participant. The
  action still resolves and does not inherit unpaid debt.
- **`ApplyCostFloat` and `CommitCost` bank a fractional remainder, and the bank is the reason
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
  safe only because it is lazily allocated by `ApplyCostFloat` or `CommitCost`
  and a template's `Character` is never charged. Anything that charges a
  template through either path (a balance preview tool, an offline simulator)
  would allocate the map ON THE TEMPLATE and hand every instance spawned
  afterwards the same shared carry. Re-make it alongside `PlayerDamage` before
  doing that.
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
- **Legacy deductors no longer exist.** Registered actions use
  `QuoteActionCost` and `CommitCost`. Autoattack prices its pre-resolution swing
  plan once; defence quotes every candidate and commits only the winner; flee
  commits once before its asynchronous blocker resolution; grapple maintenance
  commits each participant independently before drift. `DeductActionPoints` is
  a different pool entirely (see the ActionPoints note above).
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
  multiplier and the `MovementMaxStaminaCost` cap, in that order. It returns a
  **float**, and `go.go` charges it through `ApplyCostFloatOrRefuse` so the
  remainder is banked and movement still refuses when unaffordable. It used to
  return an int and ceil each move independently, which flattened the whole
  1.0-to-5.0 encumbrance range into three distinct prices, measured in-game as a
  single step with flat shoulders. `MovementCostFloor` was deleted with the
  ceiling: a banked sub-1 charge is not free, and any floor at or above 1
  re-flattens the curve it was meant to protect. The encumbrance term it replaced was written inline here and
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
- **Map a defence through `DefensePool` and its registered action.**
  There are FIVE defence constants. `DefenseDodge` / `DefenseParry` /
  `DefenseBlock` cost stamina; U6 added `DefenseQuell` (mental-spell defence,
  `Willpower + spellcasting × SkillWeight`) and `DefenseDefy` (social defence,
  `Willpower + rhetoric × SkillWeight`), and both cost **conviction**. Grepping
  for a stamina cost on either finds nothing and proves nothing.

  ```go
  func DefensePool(defenseType string) Pool                      // legacy compatibility
  func (c *Character) QuoteDefenseCost(defenseType string) (CostQuote, bool)
  func (c *Character) QuoteActionCost(req ActionCostRequest) CostQuote
  func (c *Character) CommitCost(q CostQuote, policy CostPolicy) CostCommitResult
  ```

  The quote preserves fractional carry and per-defence modifiers. Melee and
  channel resolvers quote all eligible candidates without mutation, decide the
  best contest result, then commit that winner with `CostPartial`. No production
  resolver charges through the legacy integer helper.

  The pairing matters independently: pool and amount must be read off the SAME
  defence name. An unrecognised name maps to `PoolStamina` at cost 0, so the pair
  charges nothing rather than draining an arbitrary pool.
- **`GetDefenseSequence` was deleted by U6b Task 2** (a tombstone comment
  marks the site in `combat.go`). Its equipment gate — dodge always; parry and
  block by weapon, dual-wield and shield — lives on as the only remaining copy
  inside `combat.DefenceEntriesFor` (`internal/combat/defence_sets.go`), which
  now builds the defence-name set for every channel, melee included. The
  per-channel defence table itself is `combat.DefenceSetFor`.

### Combat and Interaction Systems
- **Kill/Death statistics** (`kdstats.go`): PvP and PvE combat tracking
- **Charm system** (`charminfo.go`): Mind control and pet mechanics
- **Mob mastery** (`mobmastery.go`): Character proficiency with specific creature types
- **Shop system** (`shop.go`): NPC merchant capabilities with restocking mechanics

### Character Presentation
- **Formatted names** (`formattedname.go`): Rich text rendering with adjectives and color coding
- **Adjectives system**: Visual indicators for character states (sleeping, charmed, poisoned, prone, etc.)
- **Quest indicators**: Visual markers for quest-relevant NPCs

### Pool Reservation and the Ceiling (U7b, `reservation.go`)

Some gear and every fielded companion **reserve** part of a pool: the points are
still counted in the max but can never be spent. `GetPoolReservation`
(`validate.go`) is the total; `reservation.go` owns the ceiling on that total,
the per-item arithmetic behind it, and the words shown to the player.

Total reservation on a pool is capped at `PoolReservationCapPct` (Go default
0.66, absent from `config.yaml`) of that pool's max, per pool. The breaching
action is **refused** rather than allowed through and clamped, and a character
already over the ceiling keeps everything they have: only additions are refused.

```go
// The ceiling.
func (c *Character) ReservationCap(p Pool) int
func (c *Character) WouldBreachReservationCap(p Pool, added int) bool

// Before/after overage, for the seams that cannot price their own delta.
type ReservationSnapshot struct{ Health, Stamina, Conviction int }
func (c *Character) ReservationOverages() ReservationSnapshot
func (before ReservationSnapshot) Worsened(after ReservationSnapshot) (Pool, bool)

// Per-item arithmetic, shared with GetPoolReservation so the total and the
// single-item figure cannot drift apart.
func (c *Character) ItemReserveOnPool(itm items.Item, p Pool) int
func (c *Character) EnchantReserveAt(enchantType string, tier int, hands int, p Pool) int

// Raw reservation + pool max together, for the equip disclosure. Distinct from
// ReservationSnapshot, which records overage past the cap.
type ReservationTotals struct {
	Health, Stamina, Conviction          int
	HealthMax, StaminaMax, ConvictionMax int
}
func (c *Character) ReservationTotals() ReservationTotals

// The equip line and its mirror on remove, or "" when no pool's reserved SHARE
// moved in that direction. Callers must treat empty as "say nothing".
func (c *Character) ReservationIncreaseNotice(before ReservationTotals) string
func (c *Character) ReservationDecreaseNotice(before ReservationTotals) string

// Player-facing words. Never a raw number.
func ReserveShareBand(reserve, maxPool int) string
func (c *Character) ReservationBandName(pool string) string
func (c *Character) ReservationRefusal(p Pool, added int) string
```

### Companions (`companions.go`)

```go
const ManifestationPoolCoefficient = 5

// Companion stat pool. base = charisma + manifestationSkill*5; for a
// corpse-consuming raise the corpse pool is averaged into base FIRST, and the
// pet multiplier is applied to the result. Floors at 1.
func CalcCompanionPool(charisma, manifestationSkill int, petMultiplier float64, corpsePool int) int

// The behaviour-tree add scaler. NOT the companion formula.
func CalcSpawnPoolFromBase(baseStatPool, charisma, manifestationSkill int) int

// round(CompanionReserveDefault * petMultiplier). A multiplier <= 0 means
// "unscaled" and returns the raw default.
func CompanionReserveBase(petMultiplier float64) int

// Applies the manifestation-skill and Manifester-mutation reductions and then
// composes the U7 inverse-skill rider on top. Floors at 1.
func (c *Character) CalcCompanionReserve(baseCost int) int
```

`CanAffordCompanion` was **deleted** in U7b. With `CompanionCastingFloorPct`
defaulting to 0 it reduced to a 100% cap on conviction alone, which the ceiling
now supersedes on all three pools.

### Gotchas

1. **`GetPoolReservation` has no `IsMob` gate.** Companions reserve, and they do
   so on prod today: hand a companion enchanted gear and the reserved portion
   shows in its bars. Any code that assumes reservation is player-only is wrong.
2. **`Worsened`, not a cap test, at the equip seam.** A cap test would refuse an
   already-over-cap character an equal-for-equal swap, and so force them to
   strip. Grandfathering (D4) rules that out. `ReservationSnapshot` records
   overage only, never signed headroom, so one pool improving cannot mask
   another pool breaching.
3. **`Wear` reverts by restoring the whole `Worn` value.** That is only sound
   because the placement helpers touch nothing outside `c.Equipment`.
   `SortComponentItems` was moved out of `wearArmorSlot` into `Wear` for exactly
   this reason. Anything new added to a placement helper that mutates state
   outside `c.Equipment` breaks the revert silently.
4. **`CalcCompanionPool` applies the multiplier AFTER the corpse average.**
   Folding it into the base collapses the pet tiers as corpses grow: under the
   old shape five times the price bought about 15% more pet at a large corpse.
5. **`CalcSpawnPoolFromBase` is NOT the companion formula.** It is the
   behaviour-tree add scaler, its only caller is
   `behaviortree.actSummonCompanion`, and its callers are authored boss
   encounters tuned against its exact curve. Moving them onto the companion
   formula would nerf the Sentinel's adds roughly fivefold.
6. **`CalcCompanionReserve` composes the U7 rider onto the existing reduction,
   it never replaces it.** Replacing is strictly worse at every rank: the U7
   curve bottoms at 0.40 while the existing reduction already reaches 0.45 at
   manifestation 55. Composed, it is a 10% penalty at rank 0 and a discount
   past rank 25, which is deliberate.
7. **ONE ladder, two vocabularies.** `reserveLadder` in `reservation.go` holds
   both spellings of every rung side by side, `reserveRungOf` holds the only
   copy of the edges, and both are keyed to the **cap**, not the pool, so the
   words report remaining headroom. `ReserveShareBand` returns the prose half
   (it has to read inside a sentence), `ReservationBandName` the short half (the
   status sheet's column is 13 wide). Change a rung and you change both halves
   of it, in one place. They were keyed differently until 2026-08-16 and the
   result was three vocabularies contradicting each other at the same instant:
   the equip line said "a significant portion" of health and "a heavy share" of
   conviction while the sheet, one line away, said `heavy` and `near limit`.
   Separately, the sheet was cap-keyed on its top rung only, which made the row
   read `notable` through three consecutive refusals. `near limit` is the rung
   of warning before `at limit`.
8. **`EnchantReserveAt` scales the enchantment share by the wearer's enchanting
   rank, and the item's own `reserve_*_pct` not at all.** The rider is applied
   to the percentage before the floor, so it cannot be rounded away on a small
   pool. Calling `enchantments.GetTierReservePct` directly gives an unridden
   figure and will disagree with the character's real total.
9. **The two notices compare SHARES, not points.** `reserve_*_pct` is a
   percentage of the pool max, so any item that raises a pool raises the
   reserved points on gear already worn. A points comparison would announce a
   reservation increase for a plain +Vitality helmet, which is why
   `ReservationTotals` carries the maxima alongside the reserves. `shareShrank`
   is written out rather than expressed as `!shareGrew`: an unchanged share is
   neither, and the negation would make the remove line fire on every remove.
10. **Bars measure `EffectivePoolMax`, everywhere.** The prompt gauge
    (`users.renderVitalBar`), the `status` vitals row and the web client's
    `availablePct()` all divide by the reachable pool. Drawing the reserved
    share as a distinct band inside a ten-block ASCII gauge does not work:
    `internal/util`'s downgrade table maps both the filled block and the
    crosshatch to `#`, so the reserved band read as filled and a bleeding
    character saw a full bar.
11. **Reservation messages name only the sources actually loaded.**
    `reserveSourcesOn` walks the same two places `GetPoolReservation` totals,
    and `subject()` / `verb()` / `remedies()` build the sentence from what it
    finds; `ReservationHolders` exports the subject and verb for
    `internal/hooks`. A fixed "Your gear and bonds" told a 2026-08-16 playtest
    character holding one companion and wearing nothing that its gear was the
    problem, and the fixed remedy list told it to take off gear it did not
    have. Note the two gear kinds are tracked apart, because disenchanting
    cannot help a pinnacle item whose own spec reserves.

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

### Automatic Recovery System (U10: opposed contest, not a solo stat curve)
`AttemptRecovery(contestWin func() bool) (bool, bool)` is the once-per-round
FREE stand attempt for a prone/supine character. The old solo Dex-log curve
(`min(90, 25 + 20*ln(dex/25))` vs `dice.RollStat(50)`) is gone.

```go
func (c *Character) AttemptRecovery(contestWin func() bool) (bool, bool) {
    if !c.IsProne() && !c.IsSupine() {
        return false, false
    }

    // MinRecoveryRounds gate, read from ProneData/SupineData, unchanged.
    if minRounds > 0 {
        c.Position.ConsumeRecoveryRound()
        c.AddCondition(ConditionRecoveryPenalty, 1, 1.0, "prone recovery")
        return false, false
    }

    success := true
    if contestWin != nil {
        success = contestWin()
        if success {
            c.OnSkillUse(string(skills.UnarmedCombat), c.GetUserId())
        }
    }

    if success {
        c.Position.TransitionToStanding(state.TransitionReason{Trigger: position.TriggerRecoveryRoll})
    } else {
        c.AddCondition(ConditionRecoveryPenalty, 1, 1.0, "prone recovery")
    }

    return true, success
}
```

**Contested vs. free — caller decides:**
- `contestWin == nil` → automatic stand once `MinRecoveryRounds` is consumed.
  No roll, no progression. This is the case when nobody has aggro on the
  recoverer.
- `contestWin != nil` → the caller-built opposed contest (see
  `internal/hooks/recovery_contest.go`) against whoever is holding the
  character down. **Success-only progression**: exactly one
  `OnSkillUse(UnarmedCombat)` fires on a WON contest; a lost contest or a
  free stand fires nothing.
- `AttemptRecovery` itself never touches `internal/combat` or `internal/contest`
  — it only calls the injected closure. All contest-building (score formula,
  opponent selection, `combat.RunContest`) lives in the caller, which is why
  the site-guard allowlist entry is on `recoveryContest`, not on this method.

**Integration with Combat Hooks:**
Called every round in `NewRound_UserRoundTick` and `NewRound_MobRoundTick`,
both of which pass `recoveryContest(...)` (nil when nobody qualifies):

```go
if attemptMade, success := user.Character.AttemptRecovery(recoveryContest(user.Character)); attemptMade {
    if success {
        user.SendText(messaging.CategorySystem, "You scramble to your feet!")
        room.SendText("<user> clambers to their feet in a rushed panic.", user.UserId)
    } else {
        user.SendText(messaging.CategorySystem, "You attempt to stand, but slip back down!")
        room.SendText("<user> attempts to stand, but slips and falls.", user.UserId)
    }
}
```

The manual `stand` command (`internal/usercommands/stand.go`) is the separate,
deliberate PAID exit: it spends stamina for an uncontested stand and does not
call `AttemptRecovery` at all.

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

## Species Base Hydration, and the Two Accessors U10b-0 Added

`Validate` pass 1 fills in a stat's `Base` from the species record. The test is
**`Base == 0` AND `!BaseAuthored`** — both halves matter and removing either
breaks something:

- Drop `Base == 0` and every rolled character has its stats overwritten with the
  species baseline on the next `Validate`.
- Drop `!BaseAuthored` and a stat the data deliberately set to zero gets its
  species baseline handed back. Two mob templates rely on this: a scrubland
  dog's willpower and a scavenger bird's vitality were authored as the exact
  negation of their species base, so folding that into `base:` produces a real
  zero. See `stats.StatInfo.BaseAuthored`, which is set by the yaml.v2
  unmarshaler when a `base:` key was actually present.

Two accessors on `Character` back the progression re-key:

```go
func (c *Character) GetStatTraining(statName string) int
func (c *Character) StatPoolTotal() int
```

`GetStatTraining` is the progression curve's rank input. Unlike `GetStatValue`
it excludes `Base` (the baseline started with) and `Mods` (equipment and
spells), so difficulty depends only on gains actually made — equipping a stat
item must never make that stat harder to train.

`StatPoolTotal` is "how much creature is there":
`sum(Base) - speciesBase + sum(Training)`. Four systems need that number —
`assess`'s essence bands, `corpseRaisePool`, charm resistance, and the charm
re-roll contest — and all four used to open-code it as a sum of `.Training`,
which only worked while a mob's authored stats and spawn pool both lived there.
The expression is invariant across both U10b-0 data moves, which is why the
accessor could land before them. Species baselines are not uniform (0 to 6000
across the roster), so the subtraction is a per-species lookup, and a nil
species record contributes no baseline because such a character's `Base` was
never hydrated either.

One consequence worth knowing: `assess` does not filter player corpses (`raise`
does, on `UserId != 0`), and a player's `Base` is a gaussian roll rather than a
species baseline, so a player corpse now reports the whole character rather than
only its trained points. Deliberate.

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
composition and before the cap. (There is no floor any more; movement banks its
fractional remainder instead.)

### Integration with Combat Phase

`Awareness_Cascades.go` registers an `AfterTransition` callback on the
Combat Phase machine. On `Idle → Engaging` it triggers the reveal cascade
(`Hidden → Revealing → Visible`), so an attacker who was hiding is visible
the moment they engage — no retaliation required, and no grace period for
a surprise attack. The ambusher still gets their opening strike, which
keys off `Aggro.Type` rather than `IsHidden()`.

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

### Concentration is a contest, not a solo curve (U10)

`CalcConcentrationChance` (the old Willpower-divisor curve) is **deleted**.
Concentration is now resolved by `combat.RunConcentrationContest(casterScore,
disruption)` (`internal/combat/run_concentration_contest.go`), the one place
`Balance.ConcentrationFloor` (0.02) is read. The caster is always the attack
side; `casterScore` is `concentrationScore()`
(`internal/hooks/combat_shared_helpers.go`): `Wil.ValueAdj + spellcasting ×
SkillWeight`. Success = the caster HELD.

**Three independent disruption triggers, all through the same contest:**

1. **Damage-path** (`checkConcentrationBreak` in
   `internal/hooks/combat_shared_helpers.go`): fires when the caster takes
   damage mid-cast, but only at or above `ConcentrationDamageThresholdPct` (10) of
   max HP — chip damage never rolls at all. `disruption = damagePct * 10`.
2. **Position-path** (`processFoldRound`, chunk 4f): fires every fold round
   when the caster is not `Standing`. `disruption` comes from
   `position.PositionDisruptionDmgEquiv(pos, role) * 10` —
   `internal/state/position/disruption.go`. Standing returns 0 (the contest
   is skipped entirely in that case).
3. **Throttle-path** (`ExecuteThrottle`, `internal/actions/combat_throttle.go`):
   a live opposing score instead of a static difficulty — the throttler's
   grip (`Dex + unarmed-combat × SkillWeight`) against the target's hold.
   Telegraphed `NoDamageInterrupt` casts are exempt from all three triggers.

All three can break the same cast in a single round (layered disruption).
Progression is success-only: one `OnSkillUse`/`ApplyProgression` event on a
HELD contest, nothing on a broken one. Tests live in
`internal/characters/casting_test.go`.

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
- `internal/progression`: Pure contest-progression event layer (U9);
  `ApplyProgression` applies its `[]Event` output
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

46 non-test files. Grouped by what they own:

| Group | Files |
|-------|-------|
| Core | `character.go`, `validate.go`, `migrations.go`, `overrides.go`, `description.go`, `formattedname.go` |
| Stats & progression | `progression.go`, `skills.go`, `effective_stats.go`, `statmods`-adjacent helpers, `mobmastery.go`, `kdstats.go` |
| Resources & conditions | `pools.go`, `reservation.go`, `resources.go`, `conditions.go`, `cooldowns.go`, `buffs.go`, `sight.go` |
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

## Progression rank: trained points, not uses (U10b-0 Phase C)

The chance that a stat or skill improves is keyed to **how far it has already
come**, never to how often it has been used:

- **stat rank** = `GetStatTraining(stat)`, i.e. `StatInfo.Training`
- **skill rank** = `c.Skills[name]`, the level itself

Four sites read it, and all four must agree: `ProgressionChanceForStat`,
`ProgressionChanceForSkill`, `CheckStatProgression`'s debug log, and
`regenDamperFactor`.

Three things this removed, each deliberate:

- **The use counters no longer feed the curve.** Keying on them punished
  frequency: a stat used constantly exhausted its curve while one used rarely
  stayed cheap forever. Counters are still written to saves and shown on the
  admin dashboard, so they remain useful telemetry, and `UsesPerRank` is kept
  only for that display.
- **The anti-exploit value floor is gone** ("if the value exceeds the soft cap,
  use the value as the rank"). It existed because a counter could be low while
  the value was high; that cannot happen when the rank IS the gains.
- **Equipment can no longer make a stat harder to train.** The floor read
  `GetStatValue`, which includes `Mods`, so wearing a stat item raised your own
  difficulty. `Training` excludes both `Base` and `Mods` by construction.

`StatProgressionSoftCap` is **50** trained points, which reproduces both
documented anchors: a fresh stat is ~27% per use and a stat with 50 trained
points ~1.3%. `SkillSoftCap` stays 50 because skills already keyed on level
above it.

`ProgressionChanceForSkill` was extracted from `CheckSkillProgression` in this
phase so the expression has one home. An inline copy is what let the admin
dashboard's chance display drift from production.

Phase E **exported both**, because the dashboard lives in `internal/web` and
could not otherwise call them; it now does, at `bonusMultiplier = 1.0`. Do not
re-introduce a second copy of either expression anywhere.

`ProgressionRollThreshold(chance) int` was exported in the same phase. It is
the `int(chance * progressionRollDenominator)` arithmetic every roll site
compares against, and the dashboard's dead-stat alarm asks the same question
through it rather than hard-coding the resolution. The denominator itself stays
unexported on purpose: a caller that knows the arithmetic cannot drift from
production if the resolution moves again.
