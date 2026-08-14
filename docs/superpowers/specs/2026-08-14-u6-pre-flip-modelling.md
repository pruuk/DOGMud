# U6 pre-flip modelling

**Date:** 2026-08-14
**Purpose:** satisfy the three "model it before U6" gates in
`docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, which the arc's standing rules
require before the U6 plan may be written.
**Status:** modelling complete, decisions OPEN. No code changes.

---

## The model, and why to trust it

Contests resolve as: both sides roll, and the margin decides.

```
sigma   = 0.15 * attackerScore          (RollSpread, shipped 0.15)
atkRoll ~ N(attackerScore, sigma)
defRoll ~ N(defenderScore, sigma)       <- the ATTACKER's sigma, both sides
margin  = atkRoll - defRoll  ~  N(atk - def, sigma * sqrt(2))
hit     = margin > 0
crit    = margin / (sigma * sqrt(2))  >=  critThreshold
```

**Validation anchor.** At parity the model returns a **2.3%** crit rate, and
`ContestCritThreshold = 2.0` was chosen specifically to reproduce the legacy
`z >= 2.0` rate at parity "by construction" (`margin_crit.go`). The model
reproducing that number independently is the check that it is right.

Every mob in the game has combat skill **1** (`GetCombatSkillLevel` returns 1
when unset; no mob YAML defines skills), so all player-vs-mob figures below use
skill 1 for the mob.

---

## Gate 1 — the special-move skill weight

U6's stated action is "uniform x5, parameter deleted". Fourteen sites currently
run at **x1 on both sides**. Applied naively they all move to x5 at once.

Player: stat 100 + skill x w. Mob: stat 100 + 1 x w.

| Player skill | BEFORE hit | BEFORE crit | AFTER hit | AFTER crit |
|---:|---:|---:|---:|---:|
| 1  | 50.0% | 2.3%  | 50.0%  | 2.3%  |
| 5  | 57.1% | 5.3%  | 77.5%  | 14.8% |
| 10 | 65.0% | 12.2% | 92.1%  | 44.6% |
| 20 | 77.2% | 22.6% | 98.7%  | 77.0% |
| 30 | 85.4% | 32.7% | 99.7%  | 89.1% |
| 50 | 93.8% | 51.6% | 100.0% | 96.4% |

**Reading.** The roadmap's worked example (weapon-combat 30) goes from `130 vs
101` to `250 vs 105`: **85% hit / 33% crit becomes 99.7% hit / 89% crit**. Nearly
nine bashes in ten would be critical hits. At skill 50 a special move never
misses and crits 96% of the time.

Skill 1 is unchanged, because both sides scale together. The change is a pure
amplifier of an existing skill gap, and mobs cannot participate in it while they
are all pinned at skill 1.

**Decision needed.** Uniform x5 as written turns every special move by an
established character into a guaranteed critical hit. Options, not
recommendations:

1. Apply x5 and accept specials as reliable finishers.
2. Apply x5 but give mobs real combat skills, so the gap is a gap rather than
   an absolute.
3. Use a weight below 5 for the special-move family, contradicting "no legacy
   parameter survives U6".
4. Keep x5 and lower `SkillMultiplier`/damage so a guaranteed crit hurts less.

Option 2 is the one that also fixes the crit-threshold saturation recorded in
`calcCritThreshold`, and the `SkillWeight: 5.0` note in project memory. It is
also the largest piece of work, and it touches every mob in the game.

---

## Gate 2 — counterattack frequency

**This gate is already LIVE. It is not a U6 consequence.**

Defensive crit has been margin-driven since chunk 5.11d, and it sets the flags
consumed in `combat_shared_helpers.go`: parry crit fires **riposte**, dodge crit
fires **auto-trip**, block crit fires **auto-bash**. The threshold is a flat 2.0
on the defender's normalized margin.

| Matchup | Defender crit (counterattack) rate |
|---|---:|
| Even match | 2.3% |
| You outclass by 20% | 0.3% |
| You outclass by 50% | ~0% |
| Punching up 20% | 14.5% |
| Punching up 50% | 63.9% |
| Punching up 100% (boss) | 99.7% |

**Reading.** Attacking something well above your weight means it counterattacks
on **nearly every swing** — while you are also dealing little. The roadmap
estimated 97.7% versus 0.5% at the extremes; the model agrees on shape and
magnitude.

This is the mirror image of the crit behaviour the owner praised on trash: the
same mechanism that makes shredding weak enemies feel good makes punching up
punishing in a way no tuning knob currently touches.

**Decision needed.** U6 multiplies this: raising skill weights widens every gap,
pushing more matchups toward both extremes. Worth deciding whether counterattack
frequency should be capped independently of the crit margin before the flip
widens the spread.

---

## Gate 3 — the crit floor denominator

`MinAttackCritChance` and `MinDefenseCritChance` both ship at **0.01**, and
`applyCritFloors` branches on a **binary** hit:

```go
if res.hit {
    res.crit = ApplyCritFloor(res.crit, attackFloor)   // 1% of HITS
    return
}
// else: the swing missed -> this is the defender's side
// (requires a defence was actually mounted)
```

U6 changes `defenceOutcome` from boolean ("a win is a clean miss") to a
**margin-scaled 50-100% multiplier**. There is then no such thing as a clean
miss, which breaks this branch in two distinct ways:

1. **`res.hit` becomes degenerate.** If every swing lands for at least 50%, the
   `if res.hit` branch is taken essentially always, so the attack crit floor
   applies to ~100% of swings rather than to the current hit rate. At parity
   that roughly doubles its reach; when outclassed it is a far larger jump.
2. **The defence floor stops firing altogether.** It lives in the `else`, i.e.
   the miss branch. With no misses, floor-driven ripostes, auto-trips and
   auto-bashes silently disappear. That is a mechanic vanishing, not a rate
   drifting, and nothing would fail to compile or test.

**Decision needed.** Pick the denominator for each floor, explicitly:

| Candidate | At parity | Note |
|---|---:|---|
| Swings won outright | ~50% | Closest to today's meaning |
| Swings dealing any damage | ~100% | Under a multiplier, nearly all swings |
| All swings | 100% | Simplest, largest change |

The defence floor additionally needs a trigger that is not "the attack missed".
The natural replacement is "the defence won the contest", which under a
multiplier model is `margin < 0` rather than a miss.

---

## Summary of what U6's plan must decide

1. Whether uniform x5 applies to the special-move family, and if so what
   compensates for ~90% crit rates on established characters.
2. Whether counterattack frequency gets its own cap, given it is already at
   99.7% when punching up and U6 widens every gap.
3. The denominator for each crit floor, and a new trigger for the defence floor
   that does not depend on misses existing.

None of these is a coding decision. All three change how combat feels, so they
belong to the owner before a line of U6 is written.

## Method note

Figures produced from the model at the top of this document, using the shipped
`RollSpread` of 0.15 and the crit-threshold rule in `calcCritThreshold`
(`2.0 - 0.05 * skillDiff`, floored at 1.5). The parity anchor reproducing 2.3%
independently is the correctness check.
