# Unified Cost and Harm — design

**Date:** 2026-08-12
**Status:** Design approved in conversation. Companion to
`2026-08-12-unified-contest-resolution-design.md`. Both decompose into plans next.

> **U8 refinement, 2026-08-17:** U7 and U7b changed the implementation facts
> beneath this early arc-level design. The authoritative action-admission and
> insufficient-resource rules now live in
> [`2026-08-17-u8-unified-action-cost-admission-design.md`](2026-08-17-u8-unified-action-cost-admission-design.md).
> In particular, voluntary actions refuse when unaffordable, only
> life-preserving actions partially pay and lose skill, and affordability reads
> the already reserve-clamped current pool rather than subtracting reservation
> a second time.

---

## 1. Scope

This is **Layer 2** of the unified resolution design: one helper that applies
cost and harm, draining any of hp / sp / cp.

The framing matters and was corrected during design. This is not "what does
taking damage cost as a side effect". It is a **single cost-and-harm model**.
Every action costs something from some pool, every harm lands on some pool, and
both go through one place.

The damage magnitude model itself is **already settled** — the three-channel
pipeline, mitigation, and 5.11g crit magnitude. This spec does not redesign it.
It specifies applying it uniformly to every channel, and specifies the cost half
that has never had a model at all.

---

## 2. What exists today

| Actor | Cost today |
|---|---|
| Melee attack | stamina, from the **weapon spec** |
| Defending | stamina — dodge **2**, parry **4**, block **5**, × config multipliers |
| Spell cast | CP, plus an existing `SpellHealthCostMultiplier` |
| Grapple | per-round stamina, controller/controlled multipliers, already encumbrance-scaled |
| Movement | stamina, terrain × encumbrance scaled |
| **Ranged** | **nothing, either side** |
| **Taunt / rally / warcry** | **nothing** |
| **Defending against a spell or taunt** | **nothing** |

Three problems:

1. **The defence base costs 2 / 4 / 5 are hardcoded in Go**
   (`characters/resources.go`, `GetDefenseStaminaCost`). Only the multipliers are
   config. This is a live instance of the anti-pattern the resolution spec
   forbids, sitting in the code being rewritten.
2. **Attack stamina is authored per weapon.** It comes from the weapon spec, so
   every new weapon must remember to set it and a forgotten field silently means
   a free attack.
3. **Whole channels are free.** Ranged costs nothing on either side; taunt and
   spell resistance cost nothing at all.

### 2.1 What is already built and correct

- **`GetPoolReservation(pool, max)`** is a general reservation mechanism, not
  companion-specific. Reserve is already excluded where it matters.
- **Low-resource stat progression already exists**, in `OnRegenTick`:
  `chance = RegenProgressionBase × (1 − ratio)^RegenProgressionCurve`, so closer
  to empty already yields a higher chance, uniformly across all three pools, with
  reserve already excluded. The call sites carry a comment referencing the
  *fyttyn vitality exploit, 2026-04-16*, which is why reserve exclusion is there.
  Mappings: Health → vitality, willpower. Stamina → strength, vitality.
  Conviction → willpower, charisma.
- **`CarryCapacity()` and `GetCarriedWeight()`** exist, so the encumbrance
  modifier has everything it needs. (`ItemSpec.Weight` also exists but this
  design deliberately does not read it — see 3.2.)
- **`grappleEncumbranceMultiplier`** is a working precedent for the encumbrance
  scaling this design generalises.

---

## 3. The cost model

### 3.1 Shape

```
cost = baseCost(action)              # a flat config value per action
     × encumbranceMultiplier(actor)  # physical actions only; heavier load costs more
     × skillMultiplier(actor, skill) # narrow band, inverse to skill
     × configMultiplier(action)      # per-action tuning knob
```

Costs come from **config, not from item data**. Nobody has to remember to set a
stamina number on each new weapon, and no authored item field silently becomes a
balance lever.

### 3.2 Base costs

**Per-item weight derivation was considered and rejected.** Deriving parry cost
from weapon weight and block cost from shield weight is more realistic, but it
turns every `ItemSpec.Weight` ever authored as flavour into a live balance
number — a weapon someone typed `weight: 40` on becomes unusable — and it means
auditing the whole weight distribution before shipping. The simplification is
slightly unrealistic and not horribly so.

**Base costs are flat config values.** Everything else is carried by the two
modifiers.

| Action | Base | Encumbrance | Inverse skill |
|---|---|---|---|
| Melee attack | config | yes | combat skill |
| Parry | config | yes | weapon-combat |
| Block | config | yes | weapon-combat |
| Dodge | config | yes | unarmed-combat |
| Ranged attack | config | yes | ranged-combat |
| Grapple | existing per-round | yes (already) | unarmed-combat |
| Spell cast | existing authored CP cost | no | spellcasting |
| **Quell** (mental spell defence) | fraction of the incoming spell's cost | no | spellcasting |
| **Defy** (social / taunt defence) | fraction of the incoming taunt's cost | no | rhetoric |
| Taunt / rally / warcry | config, CP | no | rhetoric |
| Flee, sneak, other non-harm | config, small | yes | relevant skill |
| **Movement** | existing terrain-scaled cost | yes (already) | **search** |

Movement is included deliberately: it already scales by terrain and encumbrance,
and adding the inverse-skill modifier keyed on **search** makes a practised
traveller move more efficiently. Physical actions take the encumbrance modifier;
purely mental ones (casting, resisting, taunting) do not.

### 3.3 The universal skill multiplier

**One multiplier applied to every cost, scaling inversely with the relevant
skill.** A practised fighter spends less stamina on the same parry.

**The band must stay small.** A wide band means a new player is drained by their
first exchange, which is exactly the failure mode to avoid.

**Neutral is centred at rank 35, not at the soft cap.** A rank-35 character pays
the base cost; below that they pay a penalty, above it they earn a discount.
Rank 100 reaches 0.75 — deliberately generous, because reaching 100 in any skill
takes an extremely long time and should feel like it bought something.

Two `sqrt` segments joined at the centre, matching the curve idiom
`SkillMultiplier` already uses:

```
r <= centre :  1.0 + (max - 1.0) * (1 - sqrt(r / centre))
r >  centre :  1.0 - (1.0 - min) * sqrt((r - centre) / (top - centre))
r >= top    :  min
```

| Rank | Multiplier |
|---:|---:|
| 0 | 1.250 |
| 10 | 1.116 |
| 25 | 1.039 |
| **35** | **1.000** |
| 50 | 0.880 |
| 69 (Meirok) | 0.819 |
| 100 | 0.750 |

Knobs: `CostSkillMultiplierMax` 1.25, `CostSkillMultiplierMin` 0.75,
`CostSkillNeutralRank` 35, `CostSkillFloorRank` 100. Clamped at `min` beyond
`top`, so a hypothetical rank-150 character gains nothing further.

### 3.4 Insufficient resource: policy follows the action

**Life-preserving actions still resolve when the actor cannot pay, but
contribute NO skill to their roll.** U8 defines these as autoattack, defence,
flee and grapple maintenance. Score becomes bare `stat × modifiers`, dropping
the skill term entirely. Voluntary actions pay in full or refuse before
consuming their secondary state.

This was reconsidered during design. Outright autofail was the first proposal
and is worse, because it removes the actor from the contest completely and so
also removes them from the reach of the 5.9 contest floors. Under a skill-less
roll an exhausted defender is punished hard — losing `skill × 5` is a swing of
345 points for a rank-69 character — while the floors still guarantee they are
never simply a free target. Punishing, not hopeless.

It stays load-bearing for resource strategy: a player who reserves nearly all
their CP for companions and enchantments resists mental spells at raw Willpower
with no spellcasting behind it, which against a real caster is close to hopeless
without literally being so.

Rules:

- **Availability reads the current pool directly.** U7 already clamps current
  to the usable, reservation-excluded ceiling. Calling `GetPoolReservation` or
  using `EffectivePoolMax` here would subtract reservation twice.
- **Charge only for the defence actually used** — the one that won the best-of-N.
  Today's `runBestOfAllDefense` already does this
  (`combat_helpers.go:657`, "Deduct stamina only for the winning defense"). It is
  a preserve-don't-break requirement, not new work.
- **An unaffordable defence is NOT skipped.** Today it is `continue`d out of the
  loop entirely (`combat_helpers.go:567`). It must instead roll without its skill
  term, so it stays in the best-of-N set.
- **Never drive a pool negative.** Every deduction clamps at zero, in the one
  shared helper, so this cannot be got wrong per-site. Several current sites do
  `x.Pool -= n` with their own clamping, or none.
- **Messaging must name the resource.** A player who fails because they were
  exhausted needs to know that was why, not just see a failure.

---

## 4. The harm model

Damage magnitude is already settled and is not redesigned here. What this spec
fixes is **which pool harm lands on, and applying the existing model uniformly
to every channel.**

### 4.1 Channel to pool

Conviction damage already drains CP rather than HP (`ExecuteTaunt` does
`target.Char.Conviction -= dmg`). That existing precedent generalises:

| Channel | Primary pool |
|---|---|
| Physical | Health |
| Magical | Health |
| Conviction | Conviction |

A single helper applies harm to the named pool, clamps at zero, and returns what
was actually applied, so callers stop hand-rolling `x.Pool -= dmg` with
inconsistent clamping. Today those subtractions are scattered and each one
re-implements its own floor.

### 4.2 Uniform application

Every channel routes through the same helper: raw damage, item mitigation,
defence multiplier from the contest, crit magnitude. No channel keeps a private
path. This is the half of the harm model that is already designed but only
partly applied.

---

## 5. Progression from resource depletion

**Already built. Do not rebuild it — verify and unify it.**

`OnRegenTick` already implements the curve, the pool-to-stat mappings, and
reserve exclusion. The stated goal, "closer to 0% has a slightly higher chance
than 25%", is already true by construction.

Remaining work is small and specific:

1. **Confirm every depletion path reaches it.** It currently fires from the regen
   tick only. Costs paid through the new model must not bypass it.
2. **Tick-driven, decided.** A character sitting at 10% stamina rolls every
   tick, rather than firing once on crossing below a threshold. Tick-driven
   rewards *staying* depleted; event-driven would reward *reaching* depletion,
   which is gameable by bouncing across the boundary. It is also already built
   and already exploit-hardened. No change needed here.
3. **Route mob depletion through the same path**, so learning by exhaustion is
   not player-only.

---

## 6. Config knobs

> **MANDATORY: every value here is a `_datafiles/config.yaml` edit, not a code
> edit.** See the resolution spec, section 6. The 2 / 4 / 5 defence base costs
> currently hardcoded in `GetDefenseStaminaCost` must move to config as part of
> this work — they are the exact defect this rule exists to prevent.

**New:**

| Knob | Purpose |
|---|---|
| `DodgeBaseStaminaCost` | replaces the hardcoded 2 |
| `ParryBaseStaminaCost` | replaces the hardcoded 4 |
| `BlockBaseStaminaCost` | replaces the hardcoded 5 |
| `AttackBaseStaminaCost` | replaces the per-weapon authored cost |
| `RangedBaseStaminaCost` | ranged currently costs nothing |
| `QuellCostFraction` | fraction of the incoming spell's cost paid to quell it |
| `DefyCostFraction` | fraction of the incoming taunt's cost paid to defy it |
| `TauntBaseConvictionCost` | flat base for taunt / rally / warcry |
| `NonHarmContestBaseCost` | flee, sneak and similar |
| `CostSkillMultiplierMax` / `CostSkillMultiplierMin` | 1.25 / 0.75 |
| `CostSkillNeutralRank` / `CostSkillFloorRank` | 35 / 100 |
| `CostEncumbranceMultiplierMax` | ceiling on the encumbrance penalty |
| `CostTotalMultiplierMax` | clamp on the PRODUCT of all modifiers |

**Existing, reused:** `DodgeMultiplier`, `ParryMultiplier`, `BlockMultiplier`,
`SpellConvictionCostMultiplier`, `SpellHealthCostMultiplier`,
`GrappleStaminaCostPerRound`, `RegenProgressionBase`, `RegenProgressionCurve`.

---

## 7. Traps

1. **Ranged and taunt are free today.** Giving them costs is a real nerf to two
   playstyles that have never paid. Model it before shipping; do not discover it
   in play.
2. **Skill stripping plus reserve is a sharp edge.** A player who reserves CP
   heavily may resist mental spells at raw Willpower with no Spellcasting. That
   is intended, but messaging must name exhaustion rather than imply an
   automatic failure.
3. **Never subtract reservation during affordability.** Current is already
   reserve-clamped by U7; subtracting `GetPoolReservation` or comparing against
   `EffectivePoolMax` double-counts it.
4. **Conviction damage already drains CP.** Do not "fix" it to drain HP for
   consistency with the other channels; it is the precedent this design follows.
5. **Cost multipliers compound.** Encumbrance × skill × per-action config can
   stack into an unaffordable cost for a heavily-laden novice, which becomes
   refuse-everything or skill-less life-preserving actions. Clamp the product,
   not just each factor.
6. **A skill-less roll is not a small penalty.** Dropping the skill term costs a
   rank-69 character 345 points of score. Verify against the 5.9 floors that an
   exhausted defender still lands within the floor band rather than falling so
   far behind that the floor is the only thing they ever get.

7. **Movement cost now depends on `search`.** A skill nobody trains for movement
   suddenly affects travel cost. Check the live distribution of `search` ranks
   before shipping so this does not silently tax every low-search character.

---

## 8. Open, deferred

- **Spell cast costs may need rebalancing** given how much CP is reserved for
  companions and enchantments. Explicitly out of scope here; likely its own chunk
  once the cost model is in and measurable.
Both previously-open items were subsequently decided. U7 owns the shipped
inverse-skill band, whose live values differ from this early proposal. U8 owns
insufficient resource: voluntary actions refuse, while life-preserving actions
partially pay and roll without their skill term (section 3.4).

---

## 9. Success criteria

1. One helper applies all cost; one helper applies all harm. No caller does
   `x.Pool -= n` directly.
2. No balance number hardcoded in `internal/` — including the 2 / 4 / 5 that
   exist there today.
3. No new weapon needs an authored stamina cost; base costs are config values
   modified by encumbrance and skill.
4. Ranged, taunt, and defending against spells and taunts all cost something.
5. Insufficient resource refuses voluntary actions without consuming secondary
   state; life-preserving actions partially pay and drop their skill term. Both
   use the already reserve-clamped current pool and player-legible messaging.
6. **The three floor rules hold.** CORRECTED 2026-08-13; this criterion
   previously read "No pool can be driven negative from any path", which is
   wrong and would cause a real defect if implemented literally. There are three
   rules, and the helper must know whether it is applying a COST or HARM to pick
   the right one:
   - A **cost** may never drive any pool below 0.
   - **Harm** floors stamina and conviction at 0.
   - **Harm may drive health below 0, and MUST be allowed to. That is how death
     works.** `ApplyHealthChange` deliberately permits it, `validatePoolClamps`
     carries an explicit "No lower Health clamp" comment, and the per-round death
     checks read it. Clamping health at 0 destroys overkill magnitude and breaks
     anything measuring how far past zero a blow landed.
7. Depletion progression still fires, still excludes reserve, and now covers mobs.
8. `context.md` updated in the same PR for every package touched, per the
   resolution spec's criterion 7.
9. The adversarial playtest gate passes.
