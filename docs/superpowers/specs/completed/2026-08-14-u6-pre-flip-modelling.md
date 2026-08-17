# U6 pre-flip modelling

**Date:** 2026-08-14
**Purpose:** satisfy the three "model it before U6" gates in
`docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, which the arc's standing rules
require before the U6 plan may be written.
**Status:** modelling complete. Gates 1 and 2 DECIDED. Gate 3 open.

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

### Combatant scaling is ASYMMETRIC, and that is the design

This is the thing an earlier draft of this document got wrong, and every number
in it was wrong as a result.

**Players scale through skill. Mobs scale through stats.**

- Every mob in the game has combat skill **1**. `GetCombatSkillLevel` returns 1
  when unset, and not one of the 639 mob YAMLs defines a skill. Mobs cannot
  participate in the skill term at all.
- Mob power lives entirely in `statpool:`, distributed one point at a time
  across the six stats by archetype (`internal/mobs/mobs.go:527-560`). A
  `fighting` mob sends 80% of its pool into Strength / Dexterity / Vitality.
  Final stat = species base + that share.

Shipped pools span more than two orders of magnitude:

| Tier | Real mob | `statpool` | Physical stat (fighting, base ~40) |
|---|---|---:|---:|
| trash | A Wheeling Hawk | 20 | ~45 |
| common | Blind Stalker | 200 | ~93 |
| tough | Hull Warden | 300 | ~120 |
| elite | The Old White | 700 | ~227 |
| boss | The Sentinel | 2400 | ~680 |
| apex | The Core Guardian | 5000 | ~1373 |

A player sits at stat ~100-130 regardless of experience. So the contest is
"player stat + skill x w" against "mob stat + 1 x w", and the mob's *stat* is
the term that runs to four digits. Modelling every mob at stat 100 — which the
first draft did — only ever describes trash, and produces conclusions that are
false for the top two-thirds of the range.

---

## Gate 1 — the special-move skill weight — DECIDED: ship uniform x5

U6's stated action is "uniform x5, parameter deleted". Fourteen sites currently
run at **x1 on both sides**. Applied naively they all move to x5 at once.

Three representative players against the real mob range. Crit threshold is
`2.0 - 0.05 * skillDiff` floored at 1.5, so it saturates at 1.5 by player skill
11.

**New player — stat 100, skill 5** (crit threshold 1.80)

| Mob | mob stat | x1 hit | x1 crit | x5 hit | x5 crit |
|---|---:|---:|---:|---:|---:|
| A Wheeling Hawk | 45 | 99.6% | 79.8% | 99.8% | 84.5% |
| Blind Stalker | 93 | 68.4% | 9.3% | 84.3% | 21.3% |
| Hull Warden | 120 | 23.6% | 0.6% | 50.0% | 3.6% |
| The Old White | 227 | 0.0% | 0.0% | 0.0% | 0.0% |
| The Sentinel | 680 | 0.0% | 0.0% | 0.0% | 0.0% |
| The Core Guardian | 1373 | 0.0% | 0.0% | 0.0% | 0.0% |

**Mid player — stat 115, skill 20** (crit threshold 1.50)

| Mob | mob stat | x1 hit | x1 crit | x5 hit | x5 crit |
|---|---:|---:|---:|---:|---:|
| A Wheeling Hawk | 45 | 99.9% | 94.5% | 100.0% | 98.3% |
| Blind Stalker | 93 | 92.2% | 46.8% | 99.5% | 85.5% |
| Hull Warden | 120 | 68.8% | 15.6% | 97.6% | 68.2% |
| The Old White | 227 | 0.1% | 0.0% | 35.7% | 3.1% |
| The Sentinel | 680 | 0.0% | 0.0% | 0.0% | 0.0% |
| The Core Guardian | 1373 | 0.0% | 0.0% | 0.0% | 0.0% |

**Veteran (Meirok-class) — stat 130, skill 40** (crit threshold 1.50)

| Mob | mob stat | x1 hit | x1 crit | x5 hit | x5 crit |
|---|---:|---:|---:|---:|---:|
| A Wheeling Hawk | 45 | 100.0% | 97.3% | 100.0% | 99.4% |
| Blind Stalker | 93 | 98.2% | 72.5% | 100.0% | 96.5% |
| Hull Warden | 120 | 91.3% | 44.4% | 99.8% | 92.3% |
| The Old White | 227 | 5.5% | 0.1% | 92.0% | 46.2% |
| The Sentinel | 680 | 0.0% | 0.0% | 0.0% | 0.0% |
| The Core Guardian | 1373 | 0.0% | 0.0% | 0.0% | 0.0% |

**Reading.** x5 does three separate things, and only one of them is new:

1. **Against trash it changes nothing that mattered.** A veteran already crit
   the hawk 97.3% of the time at x1. 99.4% is not a different experience.
2. **Against elites it is the whole point.** The veteran versus The Old White
   goes from 5.5% hit — unplayable — to 92% hit / 46% crit. That is the band
   where skill investment starts to read as competence, and at x1 it did not
   exist. The mid player still only manages 35.7% hit there, so the band still
   discriminates.
3. **Against bosses it is inert.** The Sentinel's stat 680 and The Core
   Guardian's 1373 swamp any skill term. 40 skill x 5 is 200 points against a
   550-point stat deficit. Specials do not become a boss-killer, because the
   skill term cannot reach that far.

The earlier draft's conclusion — "uniform x5 turns every special move by an
established character into a guaranteed critical hit" — is only true against
mobs the player was already flattening. It is false everywhere else.

**Decision: apply uniform x5, delete the parameter, no compensation.** This is
the intended shape: a much stronger character dogwalks a weak mob, and mobs with
four-digit pools still compete with skilled players. Nothing in U6 needs to give
mobs real combat skills, and nothing needs a below-5 weight for the special-move
family.

---

## Gate 2 — counterattack frequency — DECIDED: no cap

**This gate is already LIVE. It is not a U6 consequence.**

Defensive crit has been margin-driven since chunk 5.11d, and it sets the flags
consumed in `combat_shared_helpers.go`: parry crit fires **riposte**, dodge crit
fires **auto-trip**, block crit fires **auto-bash**. The threshold is a flat 2.0
on the defender's normalized margin.

How often the mob counterattacks while the player is attacking it, at x5:

| Mob | vs new | vs mid | vs veteran |
|---|---:|---:|---:|
| A Wheeling Hawk | 0.0% | 0.0% | 0.0% |
| Blind Stalker | 0.1% | 0.0% | 0.0% |
| Hull Warden | 2.3% | 0.0% | 0.0% |
| The Old White | 97.8% | 5.1% | 0.0% |
| The Sentinel | 100.0% | 100.0% | 99.9% |
| The Core Guardian | 100.0% | 100.0% | 100.0% |

**Reading.** The mirror of Gate 1, and correct for the same reason. A veteran
takes zero counterattacks from anything up to elite. A new player who wanders
into The Old White eats a riposte on essentially every swing. Everyone,
regardless of skill, gets countered by The Sentinel — because 40 skill x 5 does
not close a 550-point stat gap, and is not supposed to.

**Decision: no independent cap on counterattack frequency.** Being far stronger
should mean the weak mob gets dogwalked; four-digit-pool mobs remain a wall that
skill alone does not climb. The 100% counterattack rate against a boss is the
intended "you are not ready for this" signal, and capping it would blunt the one
thing keeping bosses dangerous to skilled players.

Worth noting for content: the elite band (pool ~700) is where the transition
happens and it is sharp — 97.8% counter for a new player, 0.0% for a veteran.
That is a tuning observation for zone placement, not a code change.

---

## Gate 3 — the crit floor denominator — OPEN

### What the code does today

`MinAttackCritChance` and `MinDefenseCritChance` both ship at **0.01**.
`applyCritFloors` (`internal/combat/crit_floor.go:97`) partitions every
non-fumble swing into exactly one of two branches:

```go
if res.fumble { return }          // attacker's own blunder, neither side

if res.hit {
    res.crit = ApplyCritFloor(res.crit, attackFloor)   // ATTACKER's 1%
    return
}
// the swing missed -> DEFENDER's side
if best.defenseType == "" { return }                   // no defence mounted
res.defenseCrit = ApplyCritFloor(res.defenseCrit, defenseFloor)  // DEFENDER's 1%
```

The two floors are **mutually exclusive and complementary**. The attacker's
floor rolls only on swings that landed. The defender's floor rolls only on
swings that missed. Their denominators are `hitRate` and `1 - hitRate`, and they
sum to 1.

That means the denominator each floor gets is not a constant — it is set by the
matchup. Concretely, using the same combatants as above at x5:

| Matchup | hit rate | attacker floor rolls on | defender floor rolls on |
|---|---:|---:|---:|
| Meirok vs A Wheeling Hawk | 100.0% | 100% of swings | **0% — never** |
| Meirok vs Hull Warden | 99.8% | 99.8% | 0.2% |
| Meirok vs The Old White | 92.0% | 92% | 8% |
| Meirok vs The Sentinel | ~0% | **0% — never** | ~100% |
| New player vs Blind Stalker | 84.3% | 84.3% | 15.7% |

Read the two extreme rows, because they are the ones that show what the floors
are actually for:

- **Meirok vs A Wheeling Hawk.** He never misses, so the hawk's branch is never
  reached. The hawk has no floor-driven riposte at all. Its 1% "even a mook gets
  lucky sometimes" chance does not exist against him.
- **Meirok vs The Sentinel.** He never hits, so *his* branch is never reached.
  He has no floor-driven lucky crit against a boss. His 1% "even against a wall,
  sometimes you land one" does not exist either.

Both floors are already switched off at the extremes today, which is worth
knowing before changing them.

### What U6 breaks

U6 changes `defenceOutcome` from a boolean ("a win is a clean miss") to a
**margin-scaled 50-100% damage multiplier**. There is then no such thing as a
clean miss: every swing lands for at least half damage. Two distinct
consequences:

1. **`res.hit` becomes degenerate.** It is true on essentially every swing, so
   the attacker floor's denominator jumps to ~100% for every matchup. For
   Meirok vs The Sentinel that is 0% to 100% — he gains a 1% lucky-crit chance
   against bosses he does not have today. For a new player vs Blind Stalker,
   84.3% to 100%.
2. **The defence floor stops firing entirely, for everyone.** It lives in the
   `else`. With no misses, that branch is dead code. Floor-driven ripostes,
   auto-trips and auto-bashes disappear from the game. Note this is *only* the
   floor-driven ones — a mob whose real defensive margin clears 2.0 still
   counterattacks, so The Sentinel is unaffected. What vanishes is the tail: the
   Hull Warden's 0.2%-of-swings chance to catch Meirok, the underdog's lucky
   parry. Nothing fails to compile, no test goes red, and it would be noticed
   months later as "mid-tier mobs feel toothless".

### Decisions needed

**(a) Attacker floor denominator.** Options:

| Candidate | Meaning | Effect vs today |
|---|---|---|
| All swings | 1% of every swing crits | Boss matchups gain a floor they lack today |
| Swings that won the contest (`margin > 0`) | Closest to today's meaning | Preserves current behaviour exactly |

**(b) Defence floor trigger.** It cannot stay "the attack missed", because
misses stop existing. The natural replacement is **"the defence won the
contest"**, i.e. `margin < 0` — which is exactly what "miss" meant before the
multiplier, so it preserves today's denominators without special-casing.

Picking `margin > 0` / `margin < 0` for the two floors keeps the existing
partition intact and makes U6 a pure damage-model change with no side effect on
crit rates. Picking "all swings" for the attacker floor is a deliberate,
separate buff and should be decided as one rather than inherited by accident.

---

## Summary

1. **Gate 1 — decided.** Ship uniform x5, delete the parameter. It amplifies
   matchups the player already won, opens the elite band to skill investment,
   and cannot touch four-digit-pool bosses.
2. **Gate 2 — decided.** No cap on counterattack frequency. The extremes are the
   intended signal.
3. **Gate 3 — open.** Two choices to make: the attacker floor's denominator, and
   a defence-floor trigger that does not depend on misses existing. The
   conservative pair is `margin > 0` and `margin < 0`.

## Method note

Figures produced from the model at the top of this document, using the shipped
`RollSpread` of 0.15, the crit-threshold rule in `calcCritThreshold`
(`2.0 - 0.05 * skillDiff`, floored at 1.5), real `statpool:` values from
`_datafiles/world/dogmud/mobs/`, and the archetype distribution in
`internal/mobs/mobs.go:527-560`. Mob physical stat is species base (~40 for a
mid humanoid or beast) plus 80% of the pool split across the three physical
stats. The parity anchor reproducing 2.3% independently is the correctness
check.
