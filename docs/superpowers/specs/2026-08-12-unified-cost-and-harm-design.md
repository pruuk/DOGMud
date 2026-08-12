# Unified Cost and Harm — design

**Date:** 2026-08-12
**Status:** Design approved in conversation. Companion to
`2026-08-12-unified-contest-resolution-design.md`. Both decompose into plans next.

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
2. **Costs are derived from the wrong things.** Attack stamina comes from the
   weapon spec as an authored number, so it does not track the weapon's actual
   physical properties and every new weapon must remember to set it.
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
- **`ItemSpec.Weight`**, `CarryCapacity()`, `GetCarriedWeight()` all exist.
- **`grappleEncumbranceMultiplier`** is a working precedent for the encumbrance
  scaling this design generalises.

---

## 3. The cost model

### 3.1 Shape

```
cost = baseCost(action)              # derived from physical properties
     × encumbranceMultiplier(actor)  # heavier load costs more
     × skillMultiplier(actor, skill) # small band, inverse to skill
     × configMultiplier(action)      # per-action tuning knob
```

Costs are **derived, not authored**. A weapon's parry cost follows from its
weight; nobody has to remember to set a number on each new item.

### 3.2 Base cost derivation

| Action | Derived from |
|---|---|
| Melee attack | weapon weight |
| Parry | **weapon weight** — a heavier blade is harder to interpose |
| Block | **shield weight** |
| Dodge | **encumbrance alone** — no item interposed, so what matters is what you carry |
| Ranged attack | **weapon damage multiplier** — a heavier bow takes more effort to draw, a crossbow more to crank |
| Grapple | already per-round, keep, route through this model |
| Spell cast | existing authored CP cost, unchanged |
| Spell resist (mental) | **CP, derived from the incoming spell's cost** — resisting a big working costs more than shrugging off a cantrip |
| Taunt / rally / warcry | flat config base, then the standard multipliers |
| Flee, sneak, and other non-harm contests | small base, encumbrance-scaled |

### 3.3 The universal skill multiplier

**One multiplier applied to every cost, scaling inversely with the relevant
skill.** A practised fighter spends less stamina on the same parry.

**The band must stay small.** A wide band means a new player is drained by their
first exchange, which is exactly the failure mode to avoid. Proposed:

| Skill rank | Cost multiplier |
|---:|---:|
| 0 | 1.25 |
| 25 | 1.10 |
| 50 (soft cap) | 1.00 |
| 69 | 0.94 |
| 100 | 0.85 |

Knobs `CostSkillMultiplierMax` (1.25) and `CostSkillMultiplierMin` (0.85),
interpolated on the same `sqrt(rank/softCap)` curve `SkillMultiplier` already
uses, so cost and damage share a curve shape.

**Confirm before implementing:** the exact band. It is deliberately narrow and
the numbers above are a proposal, not a decision.

### 3.4 Insufficient resource means autofail

**If the actor cannot pay, the action automatically fails.** Not a reduced
effect, not a partial roll — a failure.

This is load-bearing, not incidental. It makes resource reservation a genuine
strategic cost: a player who reserves nearly all their CP for companions and
enchantments has little left to resist a mental spell, and will simply fail to
resist. That tension is the point.

Two consequences to implement deliberately:

- **Availability is measured with reserve excluded**, consistently with
  `OnRegenTick`. Use `GetPoolReservation` everywhere; do not read the raw pool.
- **An autofailed defence still consumes the attempt.** It cannot be free, or
  running dry becomes a way to dodge the cost of defending. It contributes no
  defence roll to the best-of-N set.

**Open:** whether an autofailed defence should still be *offered* to the
best-of-N set as an automatic loss, or excluded from the set entirely. Excluding
it means a defender out of stamina falls back to their remaining affordable
defences, which reads better. Recorded as the intended behaviour; confirm.

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
| `AttackCostPerWeaponWeight` | weight → stamina conversion for attack and parry |
| `BlockCostPerShieldWeight` | weight → stamina for block |
| `RangedCostPerDamageMultiplier` | draw effort scaling |
| `SpellResistCostFraction` | fraction of the incoming spell's cost paid to resist |
| `TauntBaseConvictionCost` | flat base for taunt / rally / warcry |
| `NonHarmContestBaseCost` | flee, sneak and similar |
| `CostSkillMultiplierMax` / `CostSkillMultiplierMin` | the narrow inverse-skill band |
| `CostEncumbranceMultiplierMax` | ceiling on the encumbrance penalty |

**Existing, reused:** `DodgeMultiplier`, `ParryMultiplier`, `BlockMultiplier`,
`SpellConvictionCostMultiplier`, `SpellHealthCostMultiplier`,
`GrappleStaminaCostPerRound`, `RegenProgressionBase`, `RegenProgressionCurve`.

---

## 7. Traps

1. **Ranged and taunt are free today.** Giving them costs is a real nerf to two
   playstyles that have never paid. Model it before shipping; do not discover it
   in play.
2. **Autofail plus reserve is a sharp edge.** A player who reserves CP heavily
   could find themselves unable to resist *any* mental spell. That is intended,
   but it needs to be legible to the player — the messaging must say the resource
   was the reason, not just report a failure.
3. **Never read a raw pool for affordability.** Always subtract
   `GetPoolReservation`. The `OnRegenTick` call sites carry an exploit-fix comment
   for exactly this.
4. **Conviction damage already drains CP.** Do not "fix" it to drain HP for
   consistency with the other channels; it is the precedent this design follows.
5. **Cost multipliers compound.** Encumbrance × skill × per-action config can
   stack into an unaffordable cost for a heavily-laden novice, which silently
   becomes autofail-everything. Clamp the product, not just each factor.
6. **`ItemSpec.Weight` is authored per item** and has never fed combat. Sanity
   check the existing distribution before making costs proportional to it — a
   weapon authored at weight 40 as flavour would become unusable.

---

## 8. Open, deferred

- **Spell cast costs may need rebalancing** given how much CP is reserved for
  companions and enchantments. Explicitly out of scope here; likely its own chunk
  once the cost model is in and measurable.
- The exact inverse-skill band (section 3.3).
- Whether an unaffordable defence is excluded from the best-of-N set or included
  as an automatic loss (section 3.4).

---

## 9. Success criteria

1. One helper applies all cost; one helper applies all harm. No caller does
   `x.Pool -= n` directly.
2. No balance number hardcoded in `internal/` — including the 2 / 4 / 5 that
   exist there today.
3. Costs are derived from physical properties, so a new weapon needs no authored
   stamina cost.
4. Ranged, taunt, and defending against spells and taunts all cost something.
5. Insufficient resource autofails, measured with reserve excluded, with
   player-legible messaging saying why.
6. Depletion progression still fires, still excludes reserve, and now covers mobs.
7. `context.md` updated in the same PR for every package touched, per the
   resolution spec's criterion 7.
8. The adversarial playtest gate passes.
