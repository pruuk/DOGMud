# Unified Contest Resolution — design

**Date:** 2026-08-12
**Status:** Design approved. Companion cost-model spec to follow, then decomposition into plans.
**Supersedes in spirit:** the per-channel resolution work in chunks 5.9, 5.10, 5.11d, 5.11e, 5.11g.

---

## 1. The problem

There are **33 opposed-roll call sites across 18 files**, plus melee, which does not
use `dice.OpposedRoll*` at all. None of them share a resolution path.

`internal/dice` is a *dice* primitive, not a *resolution* seam. `OpposedRollStat`
hands back two rolls and a margin and stops. Everything above it — contest
floors, hit, crit, fumble, damage, mitigation, resource cost, progression — is
reassembled by each caller. So every new feature reassembles it slightly
differently, and every cross-cutting change has to go find a scattered list of
sites. That is precisely what 5.9, 5.10, 5.11d, 5.11e and 5.11g each had to do.

Melee is the exception that proves it: `runBestOfAllDefense`
(`combat/combat_helpers.go:542`) already implements the target architecture —
one attack roll contested by N defence rolls, keeping the widest margin. It is
simply not reachable by anything else.

### 1.1 What the drift actually produced

Measured from source on 2026-08-12. Nobody decided this; it accreted.

| Channel | Attacker score | Defender score | Skill weight on defence | Resource cost |
|---|---|---|---|---|
| Melee | `Dex + skill×5` | best-of-3 dodge/parry/block, each `stat + skill×5` | **×5** | stamina, both sides |
| Ranged | `Perception + skill×1` | `Dex + skill×1` + flat shield bonus | **×1** | **none, either side** |
| Spell | `Wil + skill×5×3` | **raw stat, no skill term** | **×0** | CP, attacker only |
| Taunt | `Cha + rhetoric×5` | `Wil + rhetoric×5` | **×5** | none |

Four channels, four answers to "how much does skill count on defence".

Two compounding accidents:

- `SpellAttackSkillFactor: 3` was set 2026-02-25 when `SkillWeight` was 2.0, an
  effective ×6. Chunk 5.11b raised `SkillWeight` to 5.0 on 2026-08-11, silently
  taking caster skill to **×15**. 5.11b's modelling was melee-focused.
- Ranged folds its whole defence into one scalar and passes `DefenseStat: 0`,
  so the shield contribution is a flat number rather than a modifier.

> **Correction, 2026-08-12 (U3).** The ranged ATTACK row read `Perception +
> skill×5` when this spec was written. It is **×1**: `actions/combat_fire.go`
> passes `AttackSkill: rangedRank` raw into `combat.ExecuteSkillMove`, which
> adds skill to stat with no weight applied. Ranged is ×1 on both sides.
>
> More generally, this four-row table is not the whole picture, and the point is
> a rule rather than a number: **the special-move family shares no skill-weight
> convention.** `ExecuteSkillMove`'s 14 callers and `AttemptGrapple` add skill to
> stat raw (×1), `RollSubmissionAttempt` uses `SubSkillWeight` (×1.5), grapple
> drift uses ×2.2 aggressor against ×2.0 defender, `usercommands.Throw` uses ×5
> on skullduggery against ×2.5 on a **stat** with no defence skill term, and
> taunt / `TryStoicResolve` use ×5 on both sides. Section 3.1's "uniform 5.0"
> therefore moves far more than four channels. This paragraph previously claimed
> "five further regimes, from ×1 up to ×2.2", which double-counted against the
> four-row table above and understated the top of the range; the authoritative,
> per-site, measured table is the pre-U6 modelling gate "the special-move family's
> skill weight" in
> [`UNIFIED_RESOLUTION_ROADMAP.md`](../../roadmaps/UNIFIED_RESOLUTION_ROADMAP.md).
> Count from that table if a number is needed, and say what was counted.

Consequence, measured: a spellcasting-30 caster crits a stat-matched defender
**96.7%** of the time, because caster skill is tripled and defender skill is
absent. Melee at parity is 2.2%.

---

## 2. Architecture — three layers

```
  ┌─ Layer 1: CONTEST ───────────────────────────────────────────┐
  │  attacker score  vs  best-of-N defence scores                │
  │  → outcome (hit/miss/crit/fumble), margin, winning defence   │
  │  Used by ALL 34 sites, including non-harm contests.          │
  └──────────────────────────────────────────────────────────────┘
                │
  ┌─ Layer 2: HARM (only damage-dealing callers) ────────────────┐
  │  2a. damage:  raw → item mitigation → defence multiplier     │
  │               → crit magnitude                                │
  │  2b. cost:    what attacker spends, defender spends, and     │
  │               what taking the hit spends   ← SEPARATE SPEC   │
  └──────────────────────────────────────────────────────────────┘
                │
  ┌─ Layer 3: PROGRESSION ───────────────────────────────────────┐
  │  events for both sides, acting and observing                 │
  │  Used by ALL 34 sites.                                        │
  └──────────────────────────────────────────────────────────────┘
```

**Why the split is at the damage boundary.** Layers 1 and 3 apply to every
contest in the game, including sneak, steal, plant and defuse — a critical steal
and a fumbled sneak are real events that deserve crit handling and progression.
Layer 2 only makes sense where damage exists. Forcing a trap defuse through a
signature carrying weapon and mitigation-channel arguments is the pressure that
makes callers defect and hand-roll their own, which is how we got here.

---

## 3. Layer 1 — the contest core

### 3.1 Scores

Both sides: `stat + skill×SkillWeight`, then multiplied by situational
modifiers. **`SkillWeight` is uniform at 5.0 for every channel.** This removes
`SpellAttackSkillFactor` from the attack path and adds a skill term to spell
defence.

> **Gated, 2026-08-12 (U3).** "Uniform 5.0" is a bigger move than the §1.1 table
> shows: the special-move family runs at ×1, ×1.5 and ×2.2 today, so U6 lifts 14
> maneuver sites from ×1 to ×5 on both sides at once. That is unmodelled. See
> the pre-U6 gate in
> [`UNIFIED_RESOLUTION_ROADMAP.md`](../../roadmaps/UNIFIED_RESOLUTION_ROADMAP.md).

Modifiers are **multipliers near 1.0**, not flat addends. This follows the
existing combat convention (`ProneAttackMultiplier`,
`ProneVulnerabilityMultiplier`) and the CLAUDE.md rule that multipliers scale
with character power while flat values break at the extremes. Existing flat
contributions — the ranged shield bonus, `BestParryRating`, `BestBlockRating` —
migrate to multipliers.

### 3.2 Best-of-N defence

One attack roll is contested by every applicable defence; the defence winning by
the widest margin is the one that resolves. The applicable set is a property of
the attack type:

| Attack type | Applicable defences | N |
|---|---|---|
| Melee | dodge, parry, block | 3 |
| Ranged | dodge, block | 2 |
| Spell, physical damage | dodge, block | 2 |
| Spell, mental | resist (`Wil + spellcasting×5`) | 1 |
| Taunt | resist (`Wil + rhetoric×5`) | 1 |

Parry is excluded from ranged deliberately — you cannot parry a bolt. This
preserves today's documented intent in `rangedDefenseScore`.

**Parry against ranged and physical spells is deliberately excluded.** It is
defensible in fiction — an exceptional swordsman could conceivably bat a hurled
stone aside — but it would need a multiplier low enough to make it vanishingly
rare, which buys flavour at the cost of a permanent special case in the defence
set. Left out for simplicity. If it is ever wanted, it arrives as one more entry
in the applicable-defence table and nothing else changes, which is the point of
the design.

### 3.3 Outcome classification

From the **normalized margin**, `margin / (stdDev × √2)`, exactly as
`combat.ContestCrit` does today. Crit at `≥ 2.0`, which reproduces the legacy
~2.3% rate at parity by construction, so evenly matched contests need no retune.

Fumbles remain on the self-relative z-score, matching the explicit 5.11d
decision. Moving them would change failure rates nobody asked to change.

---

## 4. The defence multiplier — the change that collapses two systems

**A defence returns a damage multiplier, not a boolean.**

Today "miss" and "deflect" are unrelated mechanisms. `runBestOfAllDefense`
produces a miss; `TrySpellDeflection` and `TryStoicResolve` separately produce a
0.5 or 0.0 multiplier. That is why chunk 5.11e had to touch both. Under this
design a miss is simply multiplier `0.0` and a deflect is a value in between,
resolved by the same contest.

### 4.1 The curve

Applied **after** item mitigation:

- A bare defensive win mitigates **50%**.
- Mitigation rises linearly with the defender's margin, reaching **100%** at the
  crit threshold.
- A **defensive crit** fully negates, fires the counterattack, and is the only
  thing that can answer a crit attack.

Skill raises the margin, so skill raises mitigation continuously rather than in
a step. This is the "indirectly rewards skill" property.

### 4.2 The resolution lattice

| | Defence fumbles/loses | Defence wins, not a crit | Defence crits |
|---|---|---|---|
| **Attack crits** | full crit: bypasses armour, magnitude applied | **full crit** — a good defence is not enough | compare margins; better crit wins |
| **Attack normal** | full damage, after item mitigation | margin-scaled 50→100% mitigated | no damage |
| **Attack fumbles** | no damage, plus fumble consequences | no damage | no damage |

The load-bearing rule: **a non-crit defence can never answer a crit attack.**
Only a better crit can.

### 4.3 Why a defensive crit still matters under a 100% ceiling

Because negation is the *least* interesting thing it does. Verified in
`hooks/combat_shared_helpers.go`:

| Defensive crit | Effect |
|---|---|
| Parry crit | **Riposte** — free counter-strike at half weapon damage |
| Dodge crit | **Auto-trip**, ignoring cooldown |
| Block crit | **Auto-bash**, ignoring cooldown |

The defensive crit is the **counterattack tier**. Damage avoidance is the
continuous curve; crit is the qualitative tier that unlocks something and
answers crit attacks.

---

## 5. Layer 3 — progression

Both sides progress, from **doing** and from **observing**. Observing is worth
strictly less than doing.

**Every progression event carries BOTH a skill and a stat.** This is explicit
because it is the part most likely to be half-implemented: it is easy to wire the
skill and forget the stat, and the result compiles, plays, and quietly denies
players half the progression the design promises.

### 5.0 The event matrix

Every cell fires both a skill roll and a stat roll for the named party.

| Outcome | Attacker gets | Defender gets |
|---|---|---|
| **Attack crit** | attack skill + attack stat, **bonus multiplier** (doing) | defence skill + channel stat via `OnCritReceived` (vitality / willpower / charisma), **observing rate** |
| **Attack fumble** | attack skill + attack stat, **bonus multiplier** — failure teaches (doing) | defence skill + defence stat, **observing rate** |
| **Defence crit** | attack skill + attack stat, **observing rate** | defence skill + defence stat, **bonus multiplier** (doing) |
| **Defence fumble** | attack skill + attack stat, **observing rate** | defence skill + defence stat, **bonus multiplier** — failure teaches (doing) |
| Ordinary hit or miss | attack skill + attack stat, ordinary `OnSkillUse` / `OnStatUse` | defence skill + defence stat, ordinary rate |

The stat named is the one that fed that side's score in the contest, except on a
crit *received*, where it is the channel's toughening stat per `OnCritReceived`
— you learn to take a hit, not to swing better.

Two knobs govern the two rates, both new:

| Knob | Meaning | Proposed |
|---|---|---|
| `CritProgressionBonus` | multiplier on the progression roll for the party who *did* the crit or fumble | 2.0 |
| `ObservedCritProgressionBonus` | multiplier for the party who *received* or witnessed it | 0.5 |

Anti-exploit note: crit rates exceed 90% in lopsided fights, so a bonus
multiplier applied per crit is effectively a per-swing bonus when farming trash.
The existing curve already damps this — progression chance decays with virtual
rank — but the interaction should be modelled before the numbers are fixed, not
assumed safe.

### 5.1 Use the existing channel — most of this is already built

- `CheckSkillProgression` already takes a `bonusMultiplier`, documented "e.g. 2.0
  for critical successes". Two callers already pass 2.0.
- `OnCritReceived` already progresses **vitality** on a physical crit, willpower
  on magical, charisma on conviction, at 0.25 chance. It is wired at four sites.

The work is routing crit and fumble events through these consistently from one
place, not inventing a mechanism.

### 5.2 Do NOT inflate the use count

An alternative was considered — artificially advancing the skill use count when
crits fire, letting the existing system carry it. **This is backwards.** In
`CheckSkillProgression`, `virtualRank = adjustedUseCount / UsesPerRank`, and
`CalculateProgressionChance` is `base × exp(-decay × rank/softCap)`, which is
monotonically *decreasing* in rank. The use count decays progression; it does not
earn it. Inflating it on a crit would punish critting.

### 5.3 Return events, do not fire side effects

The contest returns the progression events it implies — who, which skill, which
stat, doing or observing, what multiplier — and a thin adapter applies them. If
the contest function fires `OnSkillUse` directly it becomes impure and every test
needs a fully hydrated character. Same single place to maintain, still testable
with plain values.

---

## 6. Tuning package

> **MANDATORY: every number in this section is a `_datafiles/config.yaml` edit,
> not a code edit.** This has been lost before — combat literals were edited
> directly when a knob already existed. Per CLAUDE.md: "Before hardcoding any
> balance number, check whether a knob already exists... If you find yourself
> editing a literal in `internal/` to change how something feels, stop and look
> for the knob. If there genuinely is not one, adding a knob is usually the
> better change than editing the literal."
>
> A pull request in this arc that changes a balance number inside `internal/`
> should be rejected on sight.

### 6.0 The exact knobs

**Existing, to be re-valued in `config.yaml`:**

| Knob | Current | Target | Purpose |
|---|---|---|---|
| `GlobalDamageMultiplier` | 0.5 | **0.325** | the ×0.65 damage cut, in one edit |
| `PhysicalMitigationCap` | 0.75 | **0.85** | item mitigation ceiling |
| `MagicalMitigationCap` | 0.75 | **0.85** | " |
| `ConvictionMitigationCap` | 0.75 | **0.85** | " |
| `SkillWeight` | 5.0 | 5.0 (unchanged) | now applied uniformly by code |
| `SpellAttackSkillFactor` | 3 | **removed from the attack path** | the ×15 accident |

`GlobalDamageMultiplier` is preferred over editing `MeleeDamageScale`,
`SpellDamageScale` and `RhetoricDamageScale` separately: it is one value, it
applies to all three channels by construction, and it cannot drift out of step
the way three edits can. The per-channel scales stay available for deliberate
per-channel asymmetry later.

**New knobs this design requires:**

| Knob | Purpose | Proposed |
|---|---|---|
| `DefenseMitigationFloor` | mitigation at a bare defensive win | 0.50 |
| `DefenseMitigationCeiling` | mitigation just below the crit threshold | 1.00 |
| `CritProgressionBonus` | progression multiplier for doing a crit/fumble | 2.0 |
| `ObservedCritProgressionBonus` | progression multiplier for receiving one | 0.5 |

Per the CLAUDE.md defaulting rule, note which of these may legitimately be `0`
(the two progression bonuses, as off switches) and validate those on `< 0`
rather than `<= 0`, mirroring the 5.9 floors. `DefenseMitigationFloor` and
`DefenseMitigationCeiling` have non-zero defaults and validate on `<= 0`.

### 6.1 Summary of the change

- **Damage ×0.65** (a 35% cut), via `GlobalDamageMultiplier`.
- **Item mitigation cap 0.75 → 0.85**, all three channels.

Modelled in `tools/balance/unified_resolution_model.py` against live behaviour,
including the 5.11g crit magnitude:

| Matchup | Untuned | With package |
|---|---|---|
| parity, light armour | +44% | **−6%** |
| parity, mid armour | +55% | **+1%** |
| parity, best-in-slot | +67% | **−20%** |
| endgame mob hitting a BIS player | +88% | **−24%** |
| player 69 v endgame mob, mid armour | +128% | +48% |
| player 30 v trash mob | +233% | +115% |

Parity lands within a few points of today at every armour level, and best-in-slot
defence gets meaningfully stronger, which is the intent: high-end gear and
defensive buffs become a real investment.

### 6.2 The residual is intended, and verified against real content

The lopsided rows stay high because **a crit bypasses item mitigation entirely**,
so both tuning levers act on the non-crit path while the growth is on the crit
path. Raising the mitigation cap cannot reach the +115% row by construction.

This is the design working as specified. The roadmap already decided: *"no crit
rate cap. A player skilled enough to dogwalk trash should crit it nearly every
hit, and hard."*

Against real endgame content the change is small. Meirok (Dex 110,
weapon-combat 69, attack score 455) versus the shipped bosses:

| Boss stat | Meirok crit rate |
|---|---|
| 100 (ordinary mob) | 94.8% |
| 325 (mid instance) | 24.0% |
| **417 (Elemental King, 325g)** | **4.9%** |
| 537 (top endgame) | 0.2% |

Today crit is self-relative, so it is a flat 2.3% against everything — a rat and
the Core Guardian alike. The change against the fights that matter is 2.3% → 5%.
That is the intended feel: you found the hole in Smaug's scales.

**Second difficulty axis, available but not used.** Giving a boss combat skill
instead of more stats is a very sharp instrument: 20 combat skill on the King
drops Meirok's crit from 4.9% to 0.4%; 40 takes it to zero. Flagged in the 5.11b
analysis as newly usable design space. Not needed now.

---

## 7. Traps — every one of these compiles cleanly if got wrong

Discovered during 5.11g/5.11e implementation on 2026-08-12.

1. **Margin sign is not consistent across the codebase.** `dice.OpposedRoll`
   returns an **attack-positive** margin (`attackRoll.Value − defenseRoll.Value`).
   `bestDefenseResult.margin` is **defence-positive**, which is exactly why
   `normalizedAttackMargin` negates it and `ContestCrit` must not. Mixing them
   puts crit on the losing side and compiles. The unified core must fix ONE
   convention and document it at the seam.
2. **Contest floors overwrite the margin with a ±1 sentinel.**
   `OpposedRollStatWithFloors` replaces the real margin when a floor fires. That
   sentinel normalizes to a near-zero z, so a floor-granted hit cannot also be a
   crit — which is correct and must be preserved. Never floor the margin itself:
   downstream effects scale by it.
3. **`best.margin` is `math.Inf(-1)` when no defence was attempted.** Negated,
   that reads as an infinitely decisive attack and would crit every swing. Detect
   via `defenseType == ""`, never by testing the margin value.
4. **An attack crit forces a hit.** Any crit floor or crit adjustment evaluated
   before the hit outcome is final becomes an undeclared second hit floor,
   leaking through `MinDefenseChance`.
5. **Both rolls use the attacker's stdDev**, so their difference has stdDev
   `× √2`. Dividing by stdDev alone inflates the margin by ~41% and roughly
   triples crit rates.
6. **`StatInfo.Base` is the input; `Value` and `ValueAdj` are derived by
   `Recalculate()`.** Assigning `Value` directly is silently wiped. Test fixtures
   that get this wrong produce raw damage of 0, both branches floor at 1, and
   ratio assertions pass vacuously.
7. **Go test binaries never load `_datafiles/config.yaml`.** Config-read balance
   values are Go defaults under test, and knobs that legitimately default to 0
   make assertions vacuously true. Pure functions in the contest core should take
   their tunables as parameters so tests can pin them.

---

## 8. Open — not settled by this spec

- **The cost model.** What the attacker spends, what a defender spends to
  attempt each defence, and what taking damage costs in hp/sp/cp. Ranged
  currently costs nothing on either side, which is itself part of the drift.
  **Its own spec, to be written next.**
- **The 5.9 contest floors become MORE load-bearing, and the crit floor's
  denominator breaks.** Measured outcome distribution under this design:

  | Matchup | Atk fumble | Atk wins | Defence crit | Graze |
  |---|---|---|---|---|
  | parity | 2.3% | 50.0% | 1.5% | 46.3% |
  | Meirok v Elemental King | 2.3% | 63.3% | 0.5% | 33.8% |
  | badly outclassed | 2.3% | 0.0% | **97.7%** | 0.0% |

  In the middle of the distribution a defensive win no longer zeroes damage, it
  scales it, so swings dealing nonzero damage rise from ~50% to ~96% at parity.
  But at the outclassed end the defender **crits 97.7% of swings**, and a
  defence crit is a true zero — so without `MinAttackHitChance` that attacker
  deals literally nothing, which is *worse* than today's 15%. The floor stops
  being a rounding correction and becomes the sole source of damage in hopeless
  fights, which is precisely what 5.9 designed it for.

  The genuine problem is the **crit floor's denominator**. 5.11e is 1% of
  *hits*, which assumed "hit" was binary. It is now a continuum from full damage
  through a 90%-mitigated graze. Decide during decomposition which denominator
  is meant: swings the attacker won outright (~50% at parity), swings dealing
  any nonzero damage (~96%), or all swings. These differ by 2x at parity and far
  more at the extremes. Do not delete the floors — 5.9 exists because a much
  weaker actor could otherwise not succeed at all, and this design makes that
  more true rather than less.

- **Sequencing.** 34 sites cannot move in one change. Decomposition into a series
  of small plans follows both specs.

### 8.1 Interaction with PR #30

PR #30 (chunks 5.11g + 5.11e) is green and unmerged. It is a **stepping stone,
not a conflict**: `ContestCrit`, `CritDamageMultiplier` and
`CritOrMitigatedDamage` are all load-bearing in this design and survive intact.

One item needs revisiting during decomposition: 5.11e's crit floors are
denominated in *hits*, chosen because an outclassed attacker hits only ~15% of
the time. That reasoning assumed "hit" was binary. Under this design it is a
continuum from full damage through a heavily-mitigated graze, so the denominator
has to be redefined before the knob means anything. See section 8. The knob and
its ordering guarantees are unaffected; only its justification needs rewriting.

---

## 9. Success criteria

1. One contest function reachable by all 34 sites; `dice.OpposedRoll*` is called
   from the core and nowhere else in `internal/actions`, `internal/combat`,
   `internal/hooks` or `internal/usercommands`.
2. Defence skill weight is `×5` in every channel. `SpellAttackSkillFactor` is
   gone from the attack path.
3. `TrySpellDeflection` and `TryStoicResolve` no longer exist as parallel
   mechanisms; both are ordinary defences returning a multiplier.
4. Adding a new contest requires declaring scores, an applicable defence set and
   a channel — no new resolution code.
5. Parity damage-per-swing within ±10% of today at light, mid and BIS armour.
6. **No balance number was changed inside `internal/`.** Every tuning value in
   section 6 moved in `_datafiles/config.yaml`. Verify by diffing: a numeric
   literal changed under `internal/` in this arc is a defect.
7. **Documentation moves with the code.** Every package touched has its
   `context.md` updated in the same PR — not a follow-up — covering at minimum
   its new public API, its file list, and any trap from section 7 that now lives
   in that package. Packages expected to need it: `internal/combat`,
   `internal/actions`, `internal/hooks`, `internal/characters`, `internal/dice`.
   Stale comments in touched files are corrected rather than left, particularly
   any that describe the per-channel resolution this design removes. Chunk 5.12
   found 61 phantom symbols across 22 packages precisely because this was
   treated as optional.
8. The adversarial playtest gate passes, per the CLAUDE.md content SOP.
