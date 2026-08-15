# U7 unified cost model — pre-build findings and decisions

Companion to `2026-08-12-unified-cost-and-harm-design.md`. That spec was written
before U5a/U5b/U5c and U6 landed. Three of its central claims are now false, its
blocking prerequisite is resolved here, and the owner settled the open tuning
questions on 2026-08-15. **Read this document with the original, not instead of
it: where the two disagree, this one is current.**

Everything below was verified against the working tree, not inferred.

---

## 1. Corrections to the 2026-08-12 spec

### 1.1 The reserve rule is wrong and must not be implemented

Spec §3.4 bullet 1 and §7 trap 3 say availability must be measured by subtracting
`GetPoolReservation`, "never read the raw pool". **Implementing that literally
would subtract the reserve twice.**

`RecalculateStats` (`internal/characters/validate.go`, the pool-reservation
clamping block) already clamps the **current** pool to `max - reserve`, and
`Validate()` runs every round for every player (`NewRound_UserRoundTick.go`) and
every mob (`NewRound_MobRoundTick.go`). So `c.Stamina` and `c.Conviction` are
already reserve-excluded when any cost reads them. The `OnRegenTick` precedent
the spec cites subtracts reserve from the **max** to form a **ratio**, which is a
different operation from an affordability check against an already-clamped
current value.

**Decision: costs read the current pool as they do today.** `ApplyCost`,
`ApplyCostPartial` and `CanAfford` stay reserve-agnostic by design. Delete the
rule rather than implement it.

### 1.2 The unaffordable-defence `continue` is already gone

Spec §3.4 bullet 3 says an unaffordable defence is skipped with a `continue`.
U5b-2 removed it; `runBestOfAllDefense` carries an explicit comment recording
that an exhausted actor still acts. What is genuinely still missing is stripping
the **skill term** from that roll, which the roadmap assigns to U8.

### 1.3 The pool-mutation cleanup is finished, not pending

Spec success criterion 1 ("one helper applies all cost… no caller does
`x.Pool -= n`") is already satisfied. `pool_mutation_guard_test.go` at the repo
root is an AST walker that fails the build on any direct pool write outside a
documented exemption list, and it passes today. U7 inherits this, it does not
build it.

### 1.4 The defence base costs are already config

Spec §6 says the hardcoded 2/4/5 "must move to config as part of this work".
U5a already moved them: `DodgeBaseStaminaCost` 2, `ParryBaseStaminaCost` 4,
`BlockBaseStaminaCost` 5 in `_datafiles/config.yaml`. What remains is retuning
them and repurposing `DodgeMultiplier`/`ParryMultiplier`/`BlockMultiplier`
(all 0.9 today, read only by `GetDefenseStaminaCost`) into the per-action
modifier slot.

### 1.5 Trap 2 is created here, not inherited

§7 trap 2 warns a heavily-reserved player "could find themselves unable to
resist any mental spell". That cannot happen today, because defences have no
affordability gate at all. U7/U8 **create** this edge; it is not pre-existing.

---

## 2. The blocking prerequisite, resolved

The roadmap made U7 conditional on mapping companions and reserved pools:
*does a cost see the reserve, and does an actor holding a companion pay more or
simply have less?*

**Answer: they have less, and that is already implemented** by the per-round
clamp in §1.1. No cost-side change is needed for it to be true.

**But "pay more" is also happening today, accidentally.** Several
percentage-of-max consumers read the **raw** max while the current value is
reserve-clamped, so a companion or enchantment holder is silently taxed:

| Site | Effect on a reserved actor |
|---|---|
| `usercommands/stand.go` — `StandStaminaCost` and `StandMinStamina` both off raw `StaminaMax.Value`, tested against the clamped current | **Live bug.** At high stamina reservation the gate becomes unreachable and the player can never stand, told "You're too exhausted to stand!" — which is false, since resting cannot fix it |
| `combat/combat_helpers.go` (swing count, melee damage, stamina mult), `hooks/combat_shared_helpers.go` (spell damage), `actions/combat_taunt.go` (taunt damage), `combat/calculations.go`, `hooks/Position_GrappleTick.go`, `characters/position_predicates.go` — all `ResourceMultiplier(current, RAW max, …)` | Permanently sits on the depletion penalty curve, unable to reach ratio 1.0. A 39%-reserved character pays a standing ~4% effectiveness penalty at "full" pools; a fully-reserved one pays the full 28% forever |
| `characters/resources.go` — `HealthPerRound`/`StaminaPerRound`/`ConvictionPerRound` off raw max | Regenerates faster relative to the usable pool. An **offsetting buff**, not a tax |

**Decisions:**

1. U7 adds `Character.EffectivePoolMax(pool)` = `poolMax - GetPoolReservation(...)`
   and routes the **percentage-of-max** consumers through it, starting with the
   `stand` lockout. Costs never use it.
2. **Regen keeps reading the raw max.** Changing it is a nerf to reserved
   characters and it currently offsets the penalty above. Left deliberately, not
   by drift.
3. Failure messaging must name **reservation** rather than exhaustion where that
   is the real cause. `usercommands/assess.go` already discloses reserve in
   descriptive bands and is the model to copy.
4. **Flagged for U8, not U7:** `CompanionCastingFloorPct` defaults to 0.0 and is
   absent from `config.yaml`, and the summon-time budget check is never
   re-applied when gear is equipped afterwards, so a reserved conviction pool of
   zero is reachable. Harmless today (defences are ungated); under U8's
   skill-strip it becomes a permanent triple-digit defence penalty. The owner's
   reservation ceiling (§2.1) is the fix, and it must land before U8.

### 2.1 The reservation ceiling (owner, 2026-08-15) — arc-scoped, slice TBD

**Total reservation on a pool is capped at somewhere between 50% and 75% of that
pool's max, and any action that would push it past the cap is REFUSED** rather
than allowed to succeed and clamp. Refusal applies at the breaching action:
wielding or equipping a reserving item, enchanting, summoning, conjuring,
raising, and anything else that adds a reservation. Applies to players and NPCs
alike.

This supersedes the piecemeal gate described above: `CanAffordCompanion` checks a
budget only at summon time, so gear equipped afterwards is unchecked, and the
three reservation sources (Chrysalis enchantments, pinnacle-item `reserve_*_pct`,
companions) are summed with no cap at all today.

Not necessarily in U7, but **must be in the arc, and must precede U8's
skill-strip**, which is what turns an over-reserved pool from cosmetic into
crippling.

Open work before it can be built:

- **Where the cap falls (50% vs 75%) needs the live distribution**, not a guess.
  A single tier-4 conviction enchant is 8% (doubled to 16% on a two-hander), a
  pinnacle item can take 15% of all three pools at once, and one companion of a
  280-base spell at manifestation 48 is about 31% of a 470 pool. Two companions
  plus one enchant already approaches 75%.
- **Characters already over the cap.** Reservation can exceed the max today, so
  live saves may breach on load. Decide: grandfather them, force-release the
  newest reservation, or clamp and warn.
- **Passive breach.** A character can cross the cap without acting, by losing max
  pool (stat drain, removing a stat-boosting item). The cap cannot only be
  enforced on the adding action.
- **Per pool, not per character.** Some items reserve two or three pools at once.
- **Messaging** must name reservation as the reason, per §2 decision 3.

---

## 3. Owner decisions, 2026-08-15

### 3.1 Charging cadence: per action, both sides

Today an attacker pays **once per round** regardless of how many weapons and
swings resolve, while a defender pays **once per incoming swing**. A 12-swing
attacker therefore spends about 3 stamina to force 12 defence charges, roughly a
16:1 ratio.

**Decision: charge per action on both sides.** Every swing pays an attack cost,
every defence mounted pays a defence cost, every spell and taunt pays on use.
This is a real change to the attack economy and forces the attack base cost far
below today's per-round values; the tuning modelling sizes it.

### 3.2 The encumbrance curve

```
r = carriedWeight / carryCapacity, clamped to [0, 1]

r <= 0.75 :  1.0 + 1.0 * (r / 0.75)           # 1.0 empty  ->  2.0 at the knee
r >  0.75 :  2.0 + 3.0 * ((r - 0.75) / 0.25)  # 2.0 knee   ->  5.0 at capacity
r >= 1.0  :  5.0                               # clamped above capacity
```

Gentle to the 75% knee, punishing from there to capacity. A realistically
equipped character sits near 1.5. Knee ratio, knee multiplier and ceiling are all
config knobs, not literals.

This replaces the two candidate readings the modelling tested. The
"only over-capacity characters pay" reading was rejected on evidence: it leaves
the multiplier at exactly 1.00 for every real character and *cuts* the veteran's
defence cost to 0.56x today's, delivering none of the intended pressure.

### 3.3 What takes which modifier

- **Physical** actions take the encumbrance multiplier. **Mental and social**
  actions do not.
- **Every** action with an associated skill takes the inverse-skill multiplier,
  mental and social included.

### 3.4 Decisions taken on the modelling, 2026-08-15 (second pass)

- **No per-round defence brake.** A defender pays per incoming swing with no
  governor. Being swarmed by many small enemies *should* overwhelm even a
  powerful character; that is the intended incentive against overreach. The
  requirement is that the drain be **tuned**, not capped: if a realistic gang
  empties a defender in about seven rounds, the numbers move, the mechanism does
  not gain a brake.
- **Spell lockouts from the cost penalty are INTENDED.** A caster whose pool sits
  between a spell's cost and 1.25x that cost loses access until they train. This
  already happens in production through reserved pools: Meirok cannot cast
  several spells he knows. Full inverse-skill band on spells; no discount-only
  exception.
- **Dodge is deliberately the most expensive defence.** Moving your whole body is
  tiring. Keep dodge 1.25 / parry 1.10 / block 1.15 on a shared base, and accept
  that this inverts today's 1/3/4 ordering.
- **Movement adopts the shared curve**, with `MovementBaseStaminaCost` dropping
  from 2.0 to **0.5** and a **floor of 1** on the final cost. Terrain stays its
  own separate multiplier (`BiomeInfo.MovementCost`, 1.0 normal / 2.0 rough,
  applied in `GetMovementStaminaCost` from `usercommands/go.go`), unchanged. The
  low base plus the floor is what keeps ordinary travel affordable while leaving
  a real penalty near capacity.
- **Summoning, conjuring and raising take the inverse-skill multiplier like
  everything else**, which makes companions cheaper for skilled summoners and
  partially offsets the §2.1 reservation ceiling. The two must be modelled
  together before either ships: the cost side and the reserve side both scale
  with manifestation, so a skilled summoner gains twice.
- **Movement trains `search`, at a deliberately rare rate.** Today `go.go` trains
  nothing, so a search-keyed cost modifier would be a permanent penalty for the
  99 of 108 live characters at rank 1, with no way to earn the discount by
  travelling. Movement therefore grants search progression, but **rarely**:
  search is already easy to raise through forage, search and track, and walking
  must not become the dominant path or every character ends up a grandmaster
  tracker with everything else lagging.

  ⚠️ **Implementation trap.** `CheckSkillProgression` derives its decay from the
  skill's USE COUNT (`virtualRank = useCount / UsesPerRank`), and `TrackSkillUse`
  is what increments it. Calling `OnSkillUse(search)` on every room move would
  pile up tens of thousands of uses and **exhaust the decay curve, poisoning
  forage-based training**. The rare rate must come from gating whether a use is
  recorded at all (or from checking progression without tracking), not from
  scaling the odds on a use that is still counted every step.

  Second-order effect, accepted deliberately: `search` also feeds hidden-creature
  detection on room entry (Perception + Search vs Dex + Skullduggery in `go.go`)
  and foraging yields, so travel-trained search slowly raises everyone's
  detection and makes sneaking harder against well-travelled characters.

### 3.4.1 Correction: in-combat regen is 3x slower than first modelled

The first modelling pass reported in-combat stamina regen as **2 per round**. It
is not. `AutoHeal` (`internal/hooks/NewRound_AutoHeal.go`) returns early unless
`RoundNumber%3 == 0`, so the regen tick fires **once every three rounds**, and
the in-combat branch then takes a quarter of it. Effective in-combat regen is
therefore roughly **0.67 stamina per round**, not 2.

Every drain figure produced before this correction is optimistic. The corrected
tables are what the tuning decision must rest on.

### 3.5 Coverage

Everything must cost something by the end of the arc, but **not necessarily in
this slice**, provided U7 lands a mechanism generic enough to wire the rest to.
So U7 builds the formula and the registration surface; the thirteen currently
free special moves, grapple initiation and sneaking can be wired later.

**Requirement this places on the design:** adding a cost to a new action must be
a config edit plus a table entry naming its base, its pool, its skill, and
whether it is physical. Not a code change at each call site.

---

## 5. Scope, restated for the plan

**U7 builds:**

1. `CostSkillMultiplier` — the two-segment inverse-skill curve. New; the existing
   `SkillMultiplier` is a different curve for damage scaling.
2. A reusable encumbrance-for-cost multiplier implementing §3.2. One exists
   inline in `GetMovementStaminaCost` and is not factored out.
3. An action-to-(base, pool, skill, physical?) registration table so new costs
   are data, per §3.4.
4. Per-action charging on both sides (§3.1), including moving `DeductAttackStamina`
   into the swing loop and re-basing the attack cost off config rather than the
   per-weapon `staminacost` field.
5. `EffectivePoolMax` and the percentage-of-max routing, including the `stand`
   lockout fix (§2).
6. The product clamp `CostTotalMultiplierMax` (spec trap 5) — no such safety
   exists anywhere in the cost path today.
7. Moving the two hardcoded mutation-active stamina literals (8 and 10, in
   `actions/mutation_venom_coat.go` and `actions/mutation_cocoon.go`) to config;
   they are live "balance number inside `internal/`" violations.
8. Routing the two raw-pool affordability reads (`usercommands/skill.cast.go`,
   `actions/action_readiness.go`) onto `CanAfford`.

**Deliberately NOT in U7:**

- The skill-strip on insufficient resource (U8, per the roadmap).
- Ranged and taunt/rally/warcry costs (U8).
- The thirteen free special moves, grapple initiation, sneak (§3.4 — later, on
  the mechanism U7 lands).
- **Quell/defy as a fraction of the incoming action's cost.** The spec asks for
  it; U6 shipped them as flat config costs. Converting requires threading an
  incoming-cost value through `ResolveChannelDefence` and its seven call sites,
  and both defences are still unobserved in live play. Deferred until the
  Elemental Queen playtest produces evidence.
- Reserve-aware regen (§2 decision 2).

---

## 6. Risks carried into the build

1. **This is the second consecutive nerf to defending.** U6 made a successful
   defence deflect rather than erase, so a defender already takes damage on every
   swing; U7 now makes them pay per swing as well. The U6 side is documented as
   structural and not tunable, so U7's multipliers are the only lever.
2. **Mobs get encumbrance nearly free.** Only 62 of 641 mob templates carry any
   equipment, so about 90% compute a carried weight of zero and sit at multiplier
   1.0 while an equipped player sits near 1.5. Under the decided curve the gap is
   modest, and mob difficulty is bought with gold-scaled stat pools rather than
   gear, so U7 ships without a synthetic mob load — but this goes on the
   pre-deploy playtest list, and if it reads as unfair the fix is one archetype
   table.
3. **In-combat regen is a flat 2 per round for everyone**, newbie to veteran,
   because `StaminaPerRound()/4` truncates twice. Regen is therefore no brake on
   any of this, and bigger pools are the only defence against a bigger bill.
4. **Rounding erases the per-action dials.** The 1.25/1.1/1.15 modifiers span
   14%, which `int()` truncation collapses entirely at small costs. Use
   `math.Ceil`, matching movement.
5. **The dodge premium rarely gets charged.** Best-of-N charges the winning
   defence, and parry and block score off weapon-combat plus item ratings while
   dodge scores off unarmed-combat, so block usually wins. "Moving your whole
   body is expensive" is real in the formula and close to invisible in play.
