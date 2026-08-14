# Contest curve bounding

**Date:** 2026-08-14
**Status:** design approved in conversation, spec pending owner review
**Supersedes:** the U6 "margin-scaled defence multiplier" element and Gate 3 in
`docs/superpowers/specs/2026-08-14-u6-pre-flip-modelling.md`
**Arc:** unified contest resolution, `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`

---

## Problem

Combat outcomes today depend only on the score RATIO, and the Gaussian roll
gives that ratio exponentially thin tails. Because sigma is proportional to the
attacker's score, the sizes cancel:

```
mu_z = (a - d) / (0.15 * a * sqrt(2)) = (1 - d/a) / 0.212
```

Everything therefore resolves inside roughly plus or minus 40% of parity. At
ratio 1.5 the underdog is at 0.9%; at 2.0 the probability is numerically zero.
Expected damage per swing against The Sentinel and The Core Guardian is exactly
0.00, so the trash-to-apex balance band is **infinite**. That is the reason
content placement has been hard to reason about, and it is what this design
fixes.

Two supporting facts, both verified against shipped data:

- **Players scale through skill, mobs through stats.** Every mob has combat
  skill 1 (`GetCombatSkillLevel` returns 1 when unset; no mob YAML defines a
  skills block). Mob power lives in `statpool:`, distributed one point at a time
  across the six stats by archetype (`internal/mobs/mobs.go:527-560`). Shipped
  pools run 20 to 5000, so a boss carries a four-digit physical stat while a
  player sits at 100-130.
- **Variance is the wrong lever, provably.** For Meirok (330) against The Core
  Guardian (1378), widening sigma to 0.30 x attacker still gives 0.0000%.
  Even sigma = 1.0 x max(a, d), where the roll is pure noise, only reaches
  29.5%. `RollSpread` cannot fix this.

---

## Design

Two changes. Neither touches how scores are computed, only how they enter the
contest and how the outcome is resolved.

### Change 1: a compression knee at the contest seam

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

**Why 350 specifically.** It sits above the player contest-score range
(125 to 330 for the three modelled profiles), so nothing a player does is
compressed and progression is untouched. Lowering it into the player range
widens the mid-band for everyone but flattens progression above it; that
trade was considered and declined.

**Contest seam only.** Resource pools, the status sheet, damage calculation and
progression all keep reading raw stats. `CLAUDE.md` forbids reintroducing
compression in `StatInfo.Recalculate()`, where it silently shrank every resource
pool by about 40% before removal on 2026-08-02. That prohibition stands and this
is not it. This distinction must survive in the code comments or it will be
re-litigated.

`ContestKneeSlope: 1.0` disables the knee exactly, so it can ship dark.

**Sigma is computed from the TRANSFORMED attacker score**, not the raw one. Both
the attack roll and every defence roll use `dice.StdDevFor(atkScore)`, so if the
knee were applied after sigma was derived, a compressed 404 would be rolled with
a sigma sized for 1378 and the contest would become almost pure noise. Transform
first, then derive sigma. This is the single easiest way to get the change wrong
and it would not fail to compile.

### Change 2: floors resolve before crits

`MinAttackHitChance` and `MinDefenseChance` both ship at 0.15, but they sit in
step 3 of `resolveDefenseOutcomeCore`, after crit resolution has already
returned. When Meirok attacks The Sentinel the defence crits on 97.6% of swings
and returns at step 2, so the attack floor only ever evaluates on the remaining
2.4%. Both floors are dead code in exactly the matchups they were written for.

A new player's 2.2% hit rate against The Core Guardian today is not the floor.
It is defence FUMBLES: the winning block roll landing two standard deviations
low while still beating dodge and parry.

New order:

```
1. fumbles          unchanged, absolute
2. organic winner   margin > 0 means the defence won (defence-positive convention)
3. floor flip       defence won -> MinAttackHitChance (0.15) grants the hit
                    attack won  -> MinDefenseChance   (0.15) grants the save
                    a flipped outcome is marked FLOORED
4. crit             only when NOT floored, on the side that actually won:
                      hit  and normalized margin >= critThreshold -> attack crit
                      save and normalized margin >= 2.0           -> defence crit
                                                                     + counterattack
5. crit floors      the existing 1% pair, unchanged
```

Step 4's exclusion of floored outcomes has precedent: `contest.RunWithFloors`
already overwrites the margin with plus or minus 1 when a floor fires, so a
floor-granted hit cannot also crit (documented on `ContestCrit`). This makes the
melee path agree with that rule instead of contradicting it.

**Symmetric, not defence-only.** Capping only the defence-crit bypass preserves
the trash dogwalk exactly and produces identical boss numbers, and was the
earlier recommendation. Capping both sides costs about 15% of trash damage and
buys a hard universal bound: hit rate inside [15%, 85%] for every pairing in the
game, up to fumbles. **Fumbles are the one deliberate bypass** and stay absolute,
so the realised range is about [14.6%, 83.2%]: an attack fumble (about 2.3%)
always misses and a defence fumble always hits, both regardless of the floors.
That is why the measured trash hit rate below is 83.0% rather than 85.0%. The
property test must allow for it rather than assert the clean bound.
The owner chose the bound, on the stated condition that
trash remains strongly asymmetric. It does: a veteran still deals 3.30x a parity
swing to A Wheeling Hawk with 82% of blows landing as mitigation-bypassing
crits.

---

## Expected outcomes

Monte Carlo of the exact resolution order (200k swings per cell), including
fumbles, best-of-three defence, crit short-circuits and both floors.

Veteran (stat 130, all skills 40, contest score 330), today to after:

| Mob | pool | hit | crit | counter | E |
|---|---:|---|---|---|---|
| A Wheeling Hawk | 20 | 97.5 to 83.0 | 96.7 to 82.3 | 0.0 to 0.0 | 3.88 to 3.30 |
| Blind Stalker | 200 | 96.7 to 83.2 | 91.2 to 77.6 | 0.0 to 0.0 | 3.70 to 3.16 |
| Hull Warden | 300 | 95.4 to 82.9 | 83.1 to 70.6 | 0.0 to 0.0 | 3.45 to 2.95 |
| The Old White | 700 | 75.5 to 72.1 | 22.8 to 19.4 | 0.2 to 0.2 | 1.44 to 1.30 |
| The Sentinel | 2400 | 0.0 to 21.6 | 0.0 to 0.2 | 97.6 to 12.4 | 0.00 to 0.22 |
| The Core Guardian | 5000 | 0.7 to 17.0 | 0.0 to 0.0 | 97.0 to 26.5 | 0.01 to 0.17 |

New player (stat 100, all skills 5, contest score 125):

| Mob | hit | counter |
|---|---|---|
| A Wheeling Hawk | 93.2 to 82.8 | 0.0 to 0.0 |
| Blind Stalker | 63.6 to 62.7 | 0.4 to 0.4 |
| Hull Warden | 31.8 to 32.6 | 5.0 to 4.3 |
| The Old White | 0.1 to 14.6 | 97.5 to 83.0 |
| The Core Guardian | 2.2 to 14.7 | 95.5 to 83.1 |

E is expected damage per swing relative to a parity swing, including the crit
multiplier (`2.0 + 0.05 * skill`, which also bypasses mitigation). The
trash-to-apex band goes from infinite to **19x**.

An outmatched new player is still countered on 83% of swings. That is the cap,
and it is the intended "you should flee" signal rather than a defect.

---

## Scope by channel

The two changes have different reach, and the difference is not arbitrary.

**Change 1 reaches everything.** Applied inside `contest.Run` and
`RunWithFloors`, which every channel has routed through since U1-U4: melee,
ranged, spells and quell, taunt and defy, maneuvers, flee, grapple, and the 17
out-of-combat contests. There is nothing per-channel to write.

**Change 2 is melee-only, because only melee has the defect.**
`contest.RunWithFloors` already applies the floor to the organic result and
stamps `Margin` as plus or minus 1, and `ContestCrit` / `DefenseContestCrit`
normalise that sentinel to a near-zero z, so a floored outcome cannot crit.
That is exactly the ordering Change 2 introduces. Change 2 therefore does not
add a rule; it makes `resolveDefenseOutcomeCore` agree with the path quell and
defy already use.

**The bounds differ per channel, deliberately.** The floor value tracks the COST
OF A SINGLE FAILURE, so the cheap-to-repeat action gets the generous floor:

| Channel | Floor pair | Resulting bound |
|---|---|---|
| Melee swing | 0.15 / 0.15 | [15%, 85%] |
| Spell, and quell | 0.05 / 0.05 | [5%, 95%] |
| Maneuver: taunt/defy, grapple, submission, skill moves, flee | 0.05 / 0.05 | [5%, 95%] |
| Out-of-combat contests | 0.05 / 0.05 | [5%, 95%] |

After the knee, melee becomes the **tightest**-bounded channel in the game and
every other channel the loosest, which inverts the situation that prompted this
design. Two things make that acceptable and it is being left alone:

- **Spells and taunts already carry partial magnitude that melee lacks.**
  `TrySpellDeflection` and `TryStoicResolve` return 1.0, then
  `SpellAvoidanceDamageMultiplier` / `RhetoricAvoidanceDamageMultiplier` (both
  0.50), then 0.0 on a defensive crit. A successful defence only halves damage;
  only a crit fully negates. That channel already degrades gracefully at the
  extremes.
- **No caster boss exists to expose the mirror problem.** The largest `casting`
  archetype pool in shipped content is 200 (The Unfolded); all six high-pool
  mobs are `fighting`. A four-digit casting boss would create a mental wall that
  melee walks through, and the knee would cover it automatically if one is ever
  authored.

Raising the spell and maneuver pairs toward 0.15 so all channels bound alike was
considered and declined: the cost-of-failure reasoning is sound and the partial
magnitude tiers already cushion those channels.

**Stale instruction this invalidates.** `internal/combat/avoidance.go:52` reads
"U6 removes BOTH TryStoicResolve and TrySpellDeflection as parallel mechanisms,
folding them into the defence multiplier". The defence multiplier is dropped, so
that instruction is void. U6 must either keep both functions as they stand or
unify them by some other route, and **this comment has to be rewritten in the
same PR** or it will misdirect the next reader.

---

## Settled, explicitly not open

**Spells outperform melee against fighting bosses.** A `fighting` archetype puts
80% of its pool into Str/Dex/Vit and 20% across Per/Wil/Cha, so The Core
Guardian is Dexterity about 1373 but Willpower about 373. Melee contests
Dexterity, quell contests Willpower, so a veteran lands 0.6% of melee swings and
28.2% of spells on the Guardian today. Spells are on cooldown and fire only
every few rounds, so the higher per-cast rate is paid for in tempo. Intended
counterplay to a bruiser, not a defect.

**Block is the strongest defence.** `BlockEffectiveness` is 1.05, the only
defence multiplier above 1.0, and a fighting mob's block score uses
`(Str+Dex)/2` which equals its Dexterity. Block requires a shield, which costs an
offhand weapon and therefore attacks. Paying for the best defence with lost
offence is the intended trade. The knee compresses the three defence scores to
within about 7 points of each other at boss stats, so the defence mix goes from
95% block to roughly 26/27/32, which is a readability improvement in the combat
log rather than a balance change.

---

## Consequences for U6

- **Drop the margin-scaled defence multiplier.** U6 planned to turn
  `defenceOutcome` from a boolean into a 50-100% damage multiplier. The knee
  reaches the same goal without it, and the binary hit survives.
- **Gate 3 dissolves.** It asked what denominator the crit floors get once clean
  misses stop existing. Misses still exist, so the question does not apply.
  Change 2 answers the related ordering problem instead.
- **Gates 1 and 2 stay decided.** Uniform x5 ships and counterattack gets no
  independent cap, because the floor pair now caps it at 85% as a consequence of
  being reachable at all.

---

## Implementation

**Files**

- `internal/configs/config.balance.go` -- declare `ContestKneeScore` and
  `ContestKneeSlope`.
- `internal/configs/config.balance.combat.go` -- defaults and validation
  (score >= 1, slope in (0, 1]).
- `internal/contest/` -- the transform, applied inside `Run` and `RunWithFloors`
  to `attackScore` and to every `Entry.Score` before sigma is derived. Every
  consumer inherits it from there: melee (`runBestOfAllDefense` builds its
  entries then calls `contest.Run`), ranged, spells, taunt, maneuvers, flee,
  grapple, and the 17 out-of-combat contests. Applying it at each call site
  instead would guarantee a missed one.
- `internal/combat/combat_helpers.go` -- restructure
  `resolveDefenseOutcomeCore` to the new branch order.
- `_datafiles/config.yaml` -- ship the two knobs with commentary.
- `context.md` for both packages.

**Testing**

- Property test: for a wide sweep of attacker and defender scores, simulated hit
  rate stays inside [0.14, 0.86]. This is the guarantee the whole design rests
  on, so it gets a test rather than a comment. The tolerance beyond [0.15, 0.85]
  is the fumble bypass described above; do not tighten it without first removing
  that bypass.
- Property test: a floored outcome never reports a crit on either side.
- Regression: parity remains 50% hit and the parity crit rate is unchanged, so
  the calibration anchor behind `ContestCritThreshold = 2.0` still holds.
- Regression: with `ContestKneeSlope: 1.0` every existing combat test passes
  unchanged, which is what makes shipping dark safe.
- Table test pinning the veteran-versus-Sentinel and veteran-versus-Hawk rows
  above, so a future change to the resolution order shows up as a diff in
  numbers rather than in feel.

**Verification beyond tests**

Per the project's content gate, a boot test proves nothing about how this feels.
This needs an adversarial playtest pass before it is called done, and it is
inside the arc's no-deploy window regardless.

---

## Method note

Figures come from a Monte Carlo (200k swings per cell, seed 20260814) of the
exact order in `resolveDefenseOutcomeCore`, not a closed form, so fumbles, the
crit short-circuits, best-of-three defence with independent rolls, and both 15%
floors are all included. World inputs are real: `statpool:` values from
`_datafiles/world/dogmud/mobs/`, the archetype split from
`internal/mobs/mobs.go:527-560`, defence formulas from
`Character.GetDefenseScore`, the melee attack score from
`combat_helpers.go:408` (Dexterity plus combat skill times `SkillWeight`, not
Strength), and the effectiveness multipliers and floors from `config.yaml`.
Mob physical stat is species base (about 40 for a mid humanoid or beast) plus
80% of the pool split across the three physical stats.
