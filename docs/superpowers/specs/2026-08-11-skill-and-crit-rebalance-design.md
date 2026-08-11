# Skill and Crit Rebalance — Design

**Roadmap chunk:** 5.11, `docs/roadmaps/ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md`
**Date:** 2026-08-11
**Status:** design decisions approved in conversation; awaiting spec review
**Model:** `tools/balance/` (`skill_model.py`, `real_examples.py`, `matrix.py`)

## Goal

Make skill govern **whether you connect** and **how often and how hard you
crit**, while damage magnitude stays mostly stats, gear and spell choice.
Priority: Skill > Gear (close) > Stats, mutations affecting all.

## Why

Measured against live data (`_archive/prod-users/users/24.yaml`, the 100g Arena
Champion, the 325g Elemental King):

| Matchup | Ratio | P(hit) |
|---|---:|---:|
| newbie vs newbie mob | 1.04x | 54.8% |
| skill 25 vs champion | 0.75x | 19.2% |
| Meirok (wc 69) vs king | **0.59x** | **15.0%** |

The player gets *more* outclassed as the game progresses. Meirok holds the
highest combat skill on the server and lands exactly `MinAttackHitChance` — the
5.9 floor is the only thing keeping him in that fight. Skill contributes
`skill * SkillWeight` = +138; the King's Dexterity alone is 417.

Separately, crit is currently keyed on the attacker's **self-relative z-score**
(`zScore = (value - mean) / stdDev`), which is statistically independent of the
opponent. Winning decisively cannot make you crit. Every situational modifier is
therefore hand-injected into `calcCritThreshold`, a single scalar with about 1.0
of headroom, where skill, Accuracy, Blink and grapple all collide and saturate.

## Decisions

1. **`SkillWeight` 2.0 -> 5.0.** Not a mob stat-pool nerf. See the matrix in the
   roadmap: SW 5 gives a clean diagonal, both levers together overcorrect into a
   stomp, and the pool nerf alone flattens toward 50/50 while requiring every
   already-balanced instance mob to be re-tuned.
2. **Crit derives from the normalized margin of the opposed roll.**
3. **Positional modifiers move into the attack/defence scores**, not the crit
   threshold.
4. **No crit rate cap.**
5. **Crit floors at 1% of HITS, both directions**, applied after the hit is
   final.
6. **Crit damage scales with skill**, on top of the existing mitigation bypass.

## Traps — read before writing code

Every one of these is silent. They are the reason this needs a spec.

**T1. `best.margin` is DEFENCE-positive.** `runBestOfAllDefense` computes
`margin := defenseRoll.Value - attackRoll.Value` and keeps the **largest**, so
`best.margin > 0` means the defence won. The attacker's margin is
`-best.margin`. Getting this backwards inverts crit onto the losing side and
still compiles.

**T2. `best.margin` is `math.Inf(-1)` when no defence was attempted.** It is
initialised to `math.Inf(-1)` and only overwritten inside the defence loop. A
defender with no stamina, or an empty defence sequence, leaves it at negative
infinity — which under margin-derivation reads as an infinitely decisive attack
and would crit **every** swing. Detect this with `best.defenseType == ""`, not
by testing the margin.

**T3. Both rolls share the attacker's stdDev.** `atkStdDev :=
dice.StdDevFor(atkScore)` is used for the attack roll *and* every defence roll.
So the difference of the two has standard deviation `atkStdDev * sqrt(2)`, and
that — not `atkStdDev` — is the correct normaliser.

**T4. `forceCrit` currently mutates the z-score.** `resolveDefenseOutcome` does
`best.hitRoll.ZScore = critThreshold + 0.5` to force a crit. Once crit no longer
reads that field the hack becomes a no-op that fails silently. Rework it as an
explicit flag.

**T5. Crit floor ordering is load-bearing.** An attack crit currently *forces* a
hit ("Attack crit vs normal defense: always hits"). A crit floor evaluated
before hit resolution therefore becomes a 1% **hit-floor bypass**, pushing the
scrub above the 15% we tuned in 5.9 and leaking through `MinDefenseChance`. The
crit floor must run after the hit is final and must never flip a miss to a hit.

**T6. Do not floor the margin itself.** A floor-saved contest already carries
margin +/-1 by the 5.9 convention that a floor save is a bare success. Flooring
the margin would corrupt every consumer that scales an effect by it — the exact
bug that convention exists to prevent. Floor the crit *decision*.

## Design

### A. `SkillWeight` 2.0 -> 5.0

A single value in `_datafiles/config.yaml`. It reaches **20 call sites**, wider
than melee: taunt/conviction (`combat_taunt.go`), spell casting
(`cast_helpers.go` x3), spell deflection (`avoidance.go`), dodge/parry/block
(`characters/combat.go`), attack score (`combat_helpers.go`) and the `consider`
estimator (`calculations.go`). It is used uniformly as `stat + skill * weight`
everywhere, so unlike `SkillMultiplier` there is no `*25` coupling to break.

**This is asymmetric in effect.** Defence is also `stat + skill * SkillWeight`,
and every mob carries combat skill 1 (no `skills` block; `GetCombatSkillLevel()`
floors at 1). Raising the weight lifts the player on offence *and* defence while
handing mobs +3 points. It is a global player buff, accepted deliberately.

**Accepted consequences:** early game gets easier (newbie vs newbie mob
54.8% -> 65.9%), judged good. PvP would amplify skill gaps — not a concern,
DOGMud has no PvP.

### B. Positional modifiers move into the scores

Today prone is a multiplier on attack score (`ProneAttackMultiplier`,
`ProneVulnerabilityMultiplier` in `calcAttackScore`) while grapple is a
subtraction on the crit threshold. Same category of effect, two mechanisms.

Move the grapple-position effects out of `calcCritThreshold` and into the
attack/defence scores alongside prone. Under margin-derivation they then feed
crit automatically, because they move the margin.

**Buff modifiers stay on the threshold.** Accuracy and Blink are not positional
— they are "this character crits more/less readily", which is exactly what a
threshold expresses. Keeping them there is coherent, and it leaves
`calcCritThreshold` with a single job instead of five.

### C. Margin-derived crit

Replace the criterion. Conceptually:

```
attackerMargin   = -best.margin                    // T1
normalisedMargin = attackerMargin / (atkStdDev * math.Sqrt2)   // T3
attackCrit       = normalisedMargin >= critThreshold
```

**Threshold 2.0 is calibrated by construction.** At parity the normalised margin
is standard normal, so a 2.0 threshold reproduces today's 2.3% crit rate exactly.
Even fights need no retune; only mismatched ones change.

**No defence attempted (T2):** when `best.defenseType == ""` there is no defence
roll and no margin. Contest the attack roll against the defender's nominal
defence score so an undefended swing still has a defined, non-infinite margin.

**Defensive crits mirror it:** `defenseCrit` uses `+best.margin` normalised the
same way against `defCritThreshold`.

**No cap.** A player skilled enough to dogwalk trash crits it nearly every hit.
At a 2x score advantage that is roughly 64% of hits, rising further with
advantage — intended.

### D. Crit floors

1% of **hits**, both directions, as an independent last-resort roll after the
hit outcome is final.

Not 1% of swings: a scrub hits about 15% of the time, so 1% of swings would
require roughly 6.7% of their hits to crit — a *higher* per-hit rate than an even
match gets at 2.3%, which is incoherent. 1% of hits also composes cleanly with
the hit floor as two independent last-resorts, each sized to the cost of the
failure it protects, matching the 5.9 rule.

Two new balance knobs, separately tunable, mirroring how 5.9 split its floors.

### E. Skill-scaled crit damage

Crit magnitude currently has **no knob at all**. `calcHitDamage` implements a
crit purely as *bypass mitigation*, so crit worth is set by the defender's
armour: 1.0x against an unarmoured target, 4.0x at the 75% mitigation cap. A
maximally skilled attacker critting an unarmoured target lands a normal hit.

**Keep the bypass, and add a multiplier on top.** The bypass is retained
deliberately: defence is best-of-all, so a skill difference already bites harder
on the defensive side for players, and the bypass is what lets a crit answer
heavy armour.

**Formula — linear in skill:**

```
critDamageMultiplier = CritDamageBase + CritDamagePerSkill * skillRank
```

with **`CritDamageBase` 2.0** and **`CritDamagePerSkill` 0.05**. Applied on top
of the mitigation bypass, so the damage a crit deals relative to a normal hit is
`critDamageMultiplier / (1 - mitigation)`.

| Skill | Multiplier |
|---:|---:|
| 1 | 2.05x |
| 25 | 3.25x |
| 50 | 4.50x |
| 69 (Meirok) | 5.45x |
| 100 | 7.00x |

**Linear, not sqrt, and that is the point.** A sqrt curve puts a skill-25 player
at 4.12x — 75% of Meirok's value for 36% of the skill — which reintroduces the
exact complaint chunk 5.7 opened with: investment past the middle stops paying.
Linear makes every skill point worth the same and keeps climbing past 69, which
is what "skill past the soft cap must do something" requires. It is also the
easiest shape to explain to a player.

**No runaway risk despite rate and magnitude both rising with skill.** Crit rate
is bounded by 1.0, so the damage-per-hit multiplier converges to
`critDamageMultiplier` itself. The worst case is bounded and predictable rather
than compounding.

**Correction to an earlier reading in this analysis:** instance mobs are *not*
unarmoured. `rooms.go:879` generates an **affix-scaled** item from each
`loot_pool` entry and calls `Wear` on it whenever `gold_paid > 0`. The Elemental
King therefore equips a Volcanic Plate (base `physical_mitigation: 12`, scaled
by an `sqrt(goldPaid) * LootBudgetScalar` budget at +1% per affix point), giving
roughly 15% at a 325g buy-in. The Arena Champion likewise equips its pool.

## Decomposition

Ordered so each slice is independently shippable and testable.

- **5.11b — `SkillWeight` 2.0 -> 5.0.** Config only. Ships and plays on its own;
  it is the fix for the endgame gap and carries no crit risk.
- **5.11c — Move positional modifiers into the scores.** Behaviour-preserving
  in spirit, but not literally: a threshold shift and a score shift are not
  numerically identical. Expect and document the delta.
- **5.11d — Margin-derived crit**, including T2's no-defence case, the defensive
  mirror, and T4's `forceCrit` rework.
- **5.11e — Crit floors**, 1% of hits both directions, after hit resolution.
- **5.11f — Skill-scaled crit damage multiplier.**
- **5.11g — Docs.** `internal/combat/context.md`, `internal/dice/context.md`,
  CLAUDE.md's combat sections, `docs/PATCH_NOTES.md`. Overlaps chunk 5.12.

## Testing

Every new test **mutation-verified** — confirmed to fail against code lacking
the fix. Specifically required:

- **T1 sign:** a test where the attacker decisively wins must crit, and one
  where the defender decisively wins must not. A sign inversion passes a
  symmetric test, so these must be asymmetric.
- **T2 no-defence:** a defender with zero stamina must not produce a guaranteed
  crit. This is the test that catches `math.Inf(-1)`.
- **T3 normaliser:** at parity, crit rate must measure ~2.3% over a large
  sample. A missing `sqrt(2)` shifts it detectably.
- **T5 ordering:** the crit floor must never convert a miss into a hit. Assert
  hit rate is unchanged when the crit floor is set to 1.0.
- **Parity preservation:** an even matchup's crit rate before and after the
  change must agree within sampling error.

## Playtest gate

5.11 is a combat-feel change, so per the CLAUDE.md content gate it needs an
in-game adversarial pass before handoff: a harness sweep plus the user's own
manual play in parallel. The model can show the math is coherent; it cannot show
whether crits *feel* like the skill payoff. Run it after 5.11f.

**Watch item — the endgame swing is large.** Decomposed for Meirok vs the
Elemental King:

| | today | after | change |
|---|---:|---:|---:|
| P(hit) | 15.0% | 59.5% | **3.95x** |
| crit rate (of hits) | 6.7% | 4.9% | — |
| a crit is worth | 1.18x | **6.41x** | — |
| damage per landed hit | 1.012 | 1.265 | 1.25x |
| **total damage output** | 0.152 | 0.752 | **4.94x** |

The two effects multiply to roughly **5x** total output. Two mitigating facts:
the baseline is 15.0%, which is *exactly* `MinAttackHitChance` — the fight is
currently impossible, not merely hard, so this converts impossible to hard. And
Meirok's crit *rate* actually falls (6.7% -> 4.9%), because the current skill-diff
threshold saturates at a skill advantage of 10 and hands him the full bonus
regardless of how badly he is losing the contest; margin-derivation prices it
honestly. The damage gain comes from crits being *worth* something, not from
critting more often.

Even so, 5x is large enough that it must be seen in play rather than trusted
from a table. **This is the reason 5.11b ships alone first** — fight the King
with only `SkillWeight` changed, before any crit code exists.

## Non-goals

- **`SkillMultiplier` and the `*25` contest consumers** (old 5.11d). The model
  does not justify the risk: lowering `SkillMultiplierMax` 3.0 -> 1.5 drops a
  rank-50 thief from 50% to 21.9% sneak success. Optional and last.
- **Mob stat-pool retuning.** Explicitly rejected in favour of `SkillWeight`.
- **Above-cap `SkillMultiplier` curve** (old 5.11e). Low stakes once skill's
  contribution moves into hit and crit.

## Risks

- **Blast radius.** `SkillWeight` touches spells, taunts and deflection as well
  as melee. Mitigated by shipping 5.11b alone first and playing it.
- **No cap means crit approaches certainty** in lopsided fights. Combat
  narration and damage messaging should be checked for readability when nearly
  every swing crits.
- **Two systems changing at once.** Sequencing keeps `SkillWeight` separate from
  the crit rework so a regression is attributable.
