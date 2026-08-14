# Contest curve bounding

**Date:** 2026-08-14
**Status:** design approved in conversation, spec pending owner review
**Arc:** unified contest resolution, `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`
**Companion:** `docs/superpowers/specs/2026-08-14-u6-pre-flip-modelling.md` holds
the frontier data behind the knee choice.

This is the U6 design. It replaces the "uniform x5 plus multiplier defence" plan
with three changes that together bound the system, and it keeps the roadmap's
defence multiplier rather than dropping it.

---

## Problem

Combat outcomes depend only on the score RATIO, and the Gaussian roll gives that
ratio exponentially thin tails. Because sigma is proportional to the attacker's
score, the sizes cancel:

```
mu_z = (a - d) / (0.15 * a * sqrt(2)) = (1 - d/a) / 0.212
```

Everything therefore resolves inside roughly plus or minus 40% of parity. At
ratio 1.5 the underdog is at 0.9%; at 2.0 the probability is numerically zero.
Expected damage per swing against The Sentinel and The Core Guardian is exactly
0.00, so the trash-to-apex balance band is **infinite**. That is why content
placement has been so hard to reason about.

Two supporting facts, both verified against shipped data:

- **Players scale through skill, mobs through stats.** Every mob has combat
  skill 1 (`GetCombatSkillLevel` returns 1 when unset; no mob YAML defines a
  skills block). Mob power lives in `statpool:`, distributed one point at a time
  across the six stats by archetype (`internal/mobs/mobs.go:527-560`). Shipped
  pools run 20 to 5000, so a boss carries a four-digit physical stat while a
  player sits at 100-130.
- **Variance is the wrong lever, provably.** For Meirok (330) against The Core
  Guardian (1378), widening sigma to 0.30 x attacker still gives 0.0000%. Even
  sigma = 1.0 x max(a, d), where the roll is pure noise, only reaches 29.5%.
  `RollSpread` cannot fix this.

---

## Change 1: a compression knee at the contest seam

Applied to the final contest score, immediately before the roll, and nowhere
else:

```
contestScore(s) = s                              if s <= ContestKneeScore
                  K + (s - K) * ContestKneeSlope otherwise
```

Shipped values: `ContestKneeScore: 350`, `ContestKneeSlope: 0.05`.

**Why a knee rather than a smooth compression.** Any transform that sees only
the ratio cannot help the boss without hurting trash: A Wheeling Hawk sits at
mu_z = +4.0 and boss viability needs |mu_z| near 1.5, so a symmetric squash that
maps -15 to -1.5 also maps +4 to about +1.4 and deletes the dogwalk. Flat
`ln(score)` takes a veteran's crit rate on trash from 99.4% to 51.4% for exactly
this reason, and a box-cox exponent or a tanh margin squash sit on the same
losing frontier. A knee escapes it because it sees ABSOLUTE score: the hawk (50)
is below the knee and untouched, the Guardian (1378) is far above and gets
flattened. Same function, different treatment, because absolute score
distinguishes them where the ratio cannot.

**Why 350.** It sits above the player contest-score range (125 to 330 for the
three modelled profiles), so nothing a player does is compressed and progression
is untouched. Lowering it into the player range widens the mid-band for everyone
but flattens progression above it; that trade was considered and declined.

**Contest seam only.** Resource pools, the status sheet, damage calculation and
progression all keep reading raw stats. `CLAUDE.md` forbids reintroducing
compression in `StatInfo.Recalculate()`, where it silently shrank every resource
pool by about 40% before removal on 2026-08-02. That prohibition stands and this
is not it. The distinction must survive in the code comments or it will be
re-litigated.

`ContestKneeSlope: 1.0` disables the knee exactly, so it can ship dark.

**Sigma is computed from the TRANSFORMED attacker score**, not the raw one. Both
the attack roll and every defence roll use `dice.StdDevFor(atkScore)`, so if the
knee were applied after sigma was derived, a compressed 404 would be rolled with
a sigma sized for 1378 and the contest would become almost pure noise. Transform
first, then derive sigma. This is the single easiest way to get the change wrong
and it would not fail to compile.

---

## Change 2: one floor for every contest in the game

**`ContestFloor: 0.125`, a single symmetric value, replacing eight knobs.**

A symmetric floor `F` yields the bound `[F, 1-F]` automatically, so one number
gives **12.5% / 87.5%**: if you are hopelessly overmatched you still succeed one
attempt in eight, and if you hopelessly overmatch you are still stopped one
attempt in eight.

Deleted knobs:

| Deleted | Was |
|---|---|
| `MinAttackHitChance`, `MinDefenseChance` | 0.15 / 0.15, melee |
| `MinSpellHitChance`, `MinSpellResistChance` | 0.05 / 0.05 |
| `MinManeuverHitChance`, `MinManeuverResistChance` | 0.05 / 0.05 |
| `MinContestSuccessChance`, `MinContestResistChance` | 0.05 / 0.05 |

Deleted functions, all in `internal/combat/contest_floors.go`: `ManeuverFloors`,
`SpellFloors`, `ContestFloors`, `RunWithManeuverFloors`, `RunWithSpellFloors`,
`RunWithGlobalFloors`. The file's whole reason for existing was keeping three
pairs straight, and with one value `contest.Run` applies it internally.
`floor_pair_guard_test.go` polices which site uses which pair and is deleted
with them.

This also resolves the split flagged at `contest_floors.go:141`, where the
global pair reads `dice.ContestFloors()` (seeded from config in `main.go`,
pinned in tests by `dice.SetContestFloors`) while the other two read config
directly. That comment already says "U6 owns collapsing the two routes".

**The previous per-channel values encoded the cost of a single failure** -- a
melee swing is a fraction of a round and got 0.15, a spell or maneuver burns the
whole round and got 0.05. That reasoning is deliberately discarded in favour of
one rule. The consequence outside combat is accepted explicitly: a novice can
pick any pocket in the game one try in eight, and a blind guard spots a master
thief one time in eight. Owner's call, 2026-08-14: most of the time the reckless
attempt fails and is punished, so the exposure is acceptable.

---

## Change 3: floors resolve before crits, and a defence returns a multiplier

### 3a. The ordering defect

`MinAttackHitChance` and `MinDefenseChance` sit in step 3 of
`resolveDefenseOutcomeCore`, after crit resolution has already returned. When
Meirok attacks The Sentinel the defence crits on 97.6% of swings and returns at
step 2, so the attack floor only ever evaluates on the remaining 2.4%. Both
floors are dead code in exactly the matchups they were written for.

A new player's 2.2% hit rate against The Core Guardian today is not the floor.
It is defence FUMBLES: the winning block roll landing two standard deviations
low while still beating dodge and parry.

Note this defect is **melee-only**. `contest.RunWithFloors` already applies the
floor to the organic result and stamps `Margin` as plus or minus 1, which
`ContestCrit` normalises to a near-zero z so a floored outcome cannot crit. The
spell and maneuver paths have always had the correct order. Change 3 makes melee
agree with them.

### 3b. The defence multiplier, reinstated

This is roadmap line 37 and section 4 of
`2026-08-12-unified-contest-resolution-design.md`, and it is kept rather than
dropped. Today "miss" and "deflect" are unrelated mechanisms:
`runBestOfAllDefense` produces a miss while `TrySpellDeflection` and
`TryStoicResolve` separately produce a flat 0.5 or 0.0 multiplier. That is two
systems answering one question.

- A bare defensive win mitigates **50%**.
- Mitigation rises linearly with the defender's margin, reaching **100%** at the
  crit threshold.
- A **defensive crit** fully negates, fires the counterattack (parry to riposte,
  dodge to auto-trip, block to auto-bash) and is the only thing that can answer
  a crit attack.

A non-crit defence can never answer a crit attack; only a better crit can. The
full lattice is section 4.2 of the original design doc and is unchanged.

`avoidance.go` is absorbed: `TrySpellDeflection` and `TryStoicResolve` are
deleted, along with `SpellAvoidanceDamageMultiplier` and
`RhetoricAvoidanceDamageMultiplier`, whose flat 0.50 becomes the bottom of the
margin-scaled curve. The comment at `avoidance.go:52` that predicts exactly this
is therefore correct and should be honoured rather than rewritten.

### 3c. The resulting order

```
1. fumbles          unchanged, absolute
2. organic winner   margin > 0 means the defence won (defence-positive convention)
3. floor flip       either side flips on ContestFloor (0.125)
                    a flipped outcome is marked FLOORED
4. crit             only when NOT floored, on the side that actually won
5. damage mapping   attack crit  -> crit magnitude, bypasses mitigation
                    attack won   -> full damage after item mitigation
                    defence won  -> margin-scaled 50-100% mitigated
                    defence crit -> zero damage plus counterattack
6. crit floors      the existing 1% pair, unchanged
```

**Floored outcomes map through the sentinel, which is already the rule.** A
floored hit carries margin +1, normalises to about zero, and reads as an attack
that barely won: full damage. A floored save carries margin -1 and reads as a
bare defensive win: 50% mitigation. Neither can crit.

---

## Expected outcomes

Monte Carlo of the exact resolution order, 200k swings per cell, seed 20260814.
`conn` is the share of swings dealing any damage, `part` the share that were
partially deflected, `E` expected damage relative to a full non-crit hit.

**Veteran** (stat 130, all skills 40, contest score 330):

| Mob | conn | crit | part | counter | E |
|---|---|---|---|---|---|
| A Wheeling Hawk | 83.0 to 97.7 | 82.3 to 84.7 | 12.2 | 0.0 to 0.0 | 3.30 to 3.46 |
| Blind Stalker | 82.9 to 97.7 | 77.3 to 79.6 | 12.3 | 0.0 to 0.0 | 3.15 to 3.30 |
| Hull Warden | 83.0 to 97.7 | 70.5 to 72.6 | 12.3 | 0.0 to 0.0 | 2.95 to 3.09 |
| The Old White | 72.1 to 97.8 | 18.8 to 19.5 | 24.1 | 0.0 to 0.0 | 1.29 to 1.43 |
| The Sentinel | 14.6 to 85.7 | 0.0 to 0.1 | 65.9 | 83.1 to 12.1 | 0.15 to 0.36 |
| The Core Guardian | 15.5 to 71.1 | 0.0 to 0.0 | 56.4 | 82.3 to 26.6 | 0.15 to 0.26 |

**New player** (stat 100, all skills 5, contest score 125):

| Mob | conn | part | counter | E |
|---|---|---|---|---|
| A Wheeling Hawk | 82.9 to 97.7 | 12.5 | 0.0 to 0.0 | 1.56 to 1.67 |
| Blind Stalker | 62.6 to 97.6 | 33.9 | 0.1 to 0.1 | 0.69 to 0.84 |
| Hull Warden | 32.5 to 93.9 | 62.4 | 3.7 to 3.8 | 0.33 to 0.51 |
| The Old White | 14.6 to 12.3 | 0.1 | 83.0 to 85.4 | 0.15 to 0.12 |
| The Core Guardian | 16.5 to 12.3 | 0.0 | 81.2 to 85.4 | 0.16 to 0.12 |

Damage band, trash to apex: **infinite today, 13.3x after** for a veteran.

Three things to read from this:

- **The dogwalk survives.** A veteran still deals 3.46x a full hit to trash, with
  85% of swings landing as mitigation-bypassing crits.
- **Bosses become fights.** Counterattack against The Sentinel falls from 83% to
  12%, and expected damage more than doubles. The `part` column is where that
  comes from: 66% of swings are partial deflections rather than clean misses.
- **A hopeless matchup is still hopeless, and slightly more so.** A new player
  against an apex boss goes from E 0.16 to 0.12, because the floor moved from
  0.15 to 0.125 and the defence crits too often to reach the partial tier. That
  is the intended "you should flee" signal.

The `part` column is also the flavour win: partial deflections range from 12% of
swings against trash to 66% against a boss, so the new combat text varies
meaningfully by matchup instead of being uniform.

---

## Settled, explicitly not open

**Spells outperform melee against fighting bosses.** A `fighting` archetype puts
80% of its pool into Str/Dex/Vit and 20% across Per/Wil/Cha, so The Core
Guardian is Dexterity about 1373 but Willpower about 373. Melee contests
Dexterity, quell contests Willpower, so a veteran lands 0.6% of melee swings and
28.2% of spells on the Guardian today. Spells are on cooldown and fire only every
few rounds, so the higher per-cast rate is paid for in tempo. Intended
counterplay to a bruiser, not a defect. Note also that no caster boss exists to
create the mirror problem: the largest `casting` pool in shipped content is 200
(The Unfolded), and all six high-pool mobs are `fighting`.

**Block is the strongest defence.** `BlockEffectiveness` is 1.05, the only
defence multiplier above 1.0, and a fighting mob's block score uses
`(Str+Dex)/2` which equals its Dexterity. Block requires a shield, which costs an
offhand weapon and therefore attacks. Paying for the best defence with lost
offence is the intended trade. The knee compresses the three defence scores to
within about 7 points of each other at boss stats, so the defence mix goes from
95% block to roughly 26/27/32, which is a combat-log readability improvement
rather than a balance change.

---

## Consequences for U6

- **Gates 1 and 2 stay decided.** Uniform x5 ships, and counterattack gets no
  independent cap because the floor now caps it at 87.5% by being reachable at
  all.
- **Gate 3 dissolves, restated correctly.** It asked what denominator the crit
  floors get once `res.hit` becomes degenerate under a defence multiplier.
  Change 3 removes the question by restructuring around WHO WON rather than
  whether it hit: the floors gate the winner, and damage is mapped afterwards.
- **Standing rule 5 is satisfied in full.** Eight floor knobs, two avoidance
  multipliers, three wrapper pairs and a guard test are deleted.

---

## Implementation

**Files**

- `internal/configs/config.balance.go` -- add `ContestFloor`, `ContestKneeScore`,
  `ContestKneeSlope`; delete the eight floor knobs and the two avoidance
  multipliers.
- `internal/configs/config.balance.combat.go` and `.misc.go` -- defaults and
  validation. `ContestFloor` in [0, 0.5), knee score >= 1, slope in (0, 1].
- `internal/contest/contest.go` -- apply the knee to `atkScore` and every
  `Entry.Score` **before sigma is derived**, and apply the single floor
  internally. Every consumer inherits both: melee (`runBestOfAllDefense` builds
  entries then calls `contest.Run`), ranged, spells, taunt, maneuvers, flee,
  grapple, and the 17 out-of-combat contests. Applying at each call site instead
  would guarantee a missed one.
- `internal/combat/contest_floors.go` -- delete.
- `internal/combat/avoidance.go` -- delete, per its own `:52` note.
- `internal/combat/combat_helpers.go` -- restructure `resolveDefenseOutcomeCore`
  to the order in 3c and return a damage multiplier rather than a boolean.
- `internal/hooks/spell_resolution.go`, `NewRound_DoCombat_helpers.go` --
  rewrite the "deflect" and "resolve" player text to the quell and defy names
  and to the partial-mitigation wording.
- `floor_pair_guard_test.go` (repo root) -- delete.
- `_datafiles/config.yaml` -- three knobs in, ten out, with commentary.
- `context.md` for `internal/contest` and `internal/combat`.

**Testing**

- Property test: over a wide sweep of attacker and defender scores, simulated
  success rate stays inside [0.115, 0.885]. The tolerance beyond
  [0.125, 0.875] is the fumble bypass, which stays absolute by design; do not
  tighten it without first removing that bypass.
- Property test: a floored outcome never reports a crit on either side.
- Property test: defence mitigation is monotone in margin and lands in
  [0.5, 1.0] for a non-crit defensive win.
- Regression: parity remains 50% success and the parity crit rate is unchanged,
  so the calibration anchor behind `ContestCritThreshold = 2.0` still holds.
- Regression: with `ContestKneeSlope: 1.0` and `ContestFloor: 0.15`, the melee
  hit rates match today's within noise. This is what makes shipping dark safe.
- Table test pinning the veteran rows above so a future change to the resolution
  order shows up as a diff in numbers rather than in feel.

**Verification beyond tests**

A boot test proves nothing about how this feels. Per the project's content gate
this needs an adversarial playtest pass before it is called done, and it is
inside the arc's no-deploy window regardless.

---

## Method note

Figures come from a Monte Carlo (200k swings per cell, seed 20260814,
`C:/tmp/u6sim/package.py`) of the exact order in `resolveDefenseOutcomeCore`,
not a closed form, so fumbles, the crit short-circuits, best-of-three defence
with independent rolls, and the floors are all included. World inputs are real:
`statpool:` values from `_datafiles/world/dogmud/mobs/`, the archetype split
from `internal/mobs/mobs.go:527-560`, defence formulas from
`Character.GetDefenseScore`, the melee attack score from `combat_helpers.go:408`
(Dexterity plus combat skill times `SkillWeight`, **not** Strength), and the
effectiveness multipliers and floors from `config.yaml`. Mob physical stat is
species base (about 40 for a mid humanoid or beast) plus 80% of the pool split
across the three physical stats.
