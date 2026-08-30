# Contest gap compression, and archetype stat distribution

**Date:** 2026-08-30
**Status:** approved in outline, ready for planning
**Owner decisions:** compress the score gap with a configurable exponent, ship
at **0.75**, apply only when the attacker is ahead (§2); and flatten the
archetype stat split toward 0.25 primary / 0.15 non-primary (§2b).

⚠️ **These are two changes in one spec on purpose. Neither is shippable alone:**
compression leaves the Elemental Queen at 97.5% crit, and redistribution ALONE
makes the royal fighters markedly worse (31.5% to 66.7% crit) by taking 76
Dexterity off them. Together they land the roster in a sane band. Any plan built
from this must deliver both before the playtest judges either.

---

## Facts verified against source

Read at `a6d80aa5b` on 2026-08-30. Every number below was measured or read, not
recalled.

| Claim | Verified | Where |
|---|---|---|
| One entry point for opposed contests | **25** production call sites, all `combat.RunContest` | `grep -rn "RunContest(" internal/ \| grep -v _test` |
| Static-difficulty rolls bypass it | `contest.AgainstDifficulty` used directly by search, track | `actions/search.go:150,169,187,298`, `actions/track.go:345` |
| The crit test | `margin/(roll.StdDev*√2) >= bar` | `combat/margin_crit.go:132-139` |
| The crit bar | `2.0 - 0.05*(atkRank-defRank)`, clamped `[1.5, 3.0]` | `combat/crit_bar.go:34-44`, `config.yaml:568-570` |
| Roll spread | `RollSpread: 0.15` | `config.yaml` |
| **Both rolls share ONE stdDev, derived from the ATTACK score** | `stdDev := dice.StdDevFor(atkScore)` then `dice.Roll(e.Score, stdDev)` | `contest/contest.go:97,103` |
| Best defence = smallest attack-positive margin | `if !res.Contested \|\| margin < res.Margin` | `contest/contest.go:109` |
| Skill weight | `SkillWeight: 5.0` | `config.yaml:868` |
| Crits skip mitigation | `if isCrit {...} else { ApplyMitigation(...) }` | `combat/crit_damage.go:96-104` |
| Mob combat skill | 1 for every mob | project notes, consistent with observed data |
| Instance scaling | `scaled = goldPaid × templateStatPool` | `rooms/instances.go:265-276` |
| Oasis tiers | sand/storm `statpool: 1`; royals `statpool: 4` | `mobs/instance_planar_oasis/*.yaml` |
| Pool distribution | fighting = 80% across Str/Dex/Vit; casting = 20% | `mobs/mobs.go:546-560` |
| Pool lands in Base | "Pool points land in Base, not Training" | `mobs/mobs.go` comment |

---

## 1. The problem, with evidence

Crit rate is decided by the **normalized score gap**, and it saturates at a
modest power advantage. Measured against the shipped curve, bar floored at 1.5:

| score ratio | hit rate | crit rate |
|---|---|---|
| 1.00 (parity) | 50.0% | 6.7% |
| 1.20 | 78.4% | 23.8% |
| 1.40 | 91.1% | 43.9% |
| 1.50 | 94.2% | **52.8%** |
| 2.00 | 99.1% | 80.4% |
| 3.00 | 99.9% | 95.0% |
| 5.00 | 100.0% | 98.8% |

> **Model note.** Both rolls draw from ONE standard deviation, taken from the
> attacker's score (`contest.go:97,103`). The normalized margin is therefore
> exactly `N(mean, 1)` with `mean = (A-D)/(0.15·A·√2)`, which is why these
> numbers are clean. An earlier draft of this spec assumed each side rolled with
> its own spread and quoted slightly different figures; the table above is the
> corrected one.

A **50% power edge produces a majority-crit outcome.** That matters more than it
sounds, because a crit is not a bonus but a branch: `crit_damage.go:96-104`
multiplies damage **and skips mitigation entirely**. At skill 48 that is a
**17.6×** swing against a 75%-mitigated target.

Observed live in the planar oasis at 325 gold, from
`_datafiles/logs/combat-analytics.jsonl` after the crit-source instrumentation
landed:

```
crits_by_source: {"rolled": 132, "unlabelled": 2}   52.3% of hits crit
crits_by_source: {"rolled": 43,  "unlabelled": 2}   68.2% of hits crit
```

**Nothing is forcing those crits** — no `sleeping`, no `crit_on_win`. They are
all legitimately rolled. The model is behaving as designed; the design is wrong.

Predicted rates for that instance against Meirok's melee attack score of 455:

| mob | pool | def score | crit today |
|---|---|---|---|
| Sand Elemental (tough) | 650 | 178 | 91.4% |
| Storm Elemental (tough) | 650 | 48 | 99.7% |
| Elemental Queen | 1300 | 92 | 98.8% |
| King / Prince / Princess | 1300 | 352 | 33.4% |

> **Tier correction, owner 2026-08-30.** Sand and storm elementals carry gear
> and were always intended as **tough** (`statpool: 2`). Both were authored
> `statpool: 1` and were therefore spawning at trash tier. Fixed on this branch;
> every oasis figure in this spec uses the corrected pools.

### Two structural causes, both out of scope here

Recorded because a reader will otherwise think this slice fixes them.

1. **`SkillWeight` 5.0 against mob combat skill 1.** Meirok's 48 ranks contribute
   240 of his 455; every mob contributes 5. Most of the gap is skill, not stats.
2. **Archetype decides physical defence more than tier does.** A `casting` mob
   puts 20% of its pool into physical stats split three ways, so Dexterity gets
   **1/15th** of the pool. The Elemental Queen (1300 pool) lands at the same
   physical defence as a sand elemental (325 pool): 92 either way. A four-fold
   tier increase buys her nothing against a melee attacker.

Cause 1 is out of scope. **Cause 2 is IN scope as of the owner's decision on
2026-08-30** and is specified in §2b; the two changes must be evaluated together
because neither is sufficient alone.

---

## 2. The change

Compress the score gap before it is rolled, by raising the DEFENCE toward the
attacker:

```
effectiveDefence = attack - (attack - defence) ^ p        when attack > defence
effectiveDefence = defence                                otherwise
```

with `p` a config knob, shipped **0.80**. `p = 1.0` telescopes to
`attack - (attack - defence) = defence`, an exact identity, so the knob is a
true no-op at its default.

### ⚠️ Why the DEFENCE moves and not the attack

An earlier draft compressed the attacker instead
(`effectiveAttack = defence + (attack-defence)^p`). That is algebraically the
same gap, and it is **wrong here**, because `contest.Run:97` derives the roll
spread from whatever attack score it is handed:

```go
stdDev := dice.StdDevFor(atkScore)   // and the DEFENDER rolls with it too, :103
```

Lowering the attack score therefore shrinks the spread as well, and since crit
is measured in units of that spread, the compression largely cancels itself:

| defence | crit, compressing the ATTACK | crit, compressing the DEFENCE |
|---|---|---|
| 48 | **94.3%** | **28.7%** |
| 140 | 55.6% | 23.4% |
| 276 | 21.5% | 16.0% |

Against a weak defender the attacker-side form barely works at all. Moving the
defence leaves `atkScore` untouched, so the spread stays `0.15 x attack` exactly
as today and only the mean moves — which is the whole intent.

It also removes a side effect nobody wanted: under the attacker-side form, a
strong character's rolls became *more consistent* simply because they were
fighting something weak.

### Why the gap and not the scores

Compressing each score independently (`√attack` vs `√defence`) changes the
meaning of every score in the game and interacts with every other consumer.
Compressing only the **difference** leaves scores untouched everywhere else and
alters exactly one thing: how decisively a mismatch resolves.

### Why "attacker ahead" only

Symmetric compression helps weak attackers by precisely as much as it restrains
strong ones. Measured:

| matchup | current hit | symmetric p=0.5 | ahead-only p=0.5 |
|---|---|---|---|
| underdog 105 vs 185 | **0.6%** | **40.8%** | **0.6%** |

Taking a trash mob from 1-in-160 to 2-in-5 against a strong player is a separate
design decision about whether weak things can meaningfully threaten strong ones.
It is not required to fix crit inflation, and it should not ride along
unannounced. **Ahead-only delivers every benefit against crit with none of it.**

Left as a knob anyway (see §3) because it is a one-line difference and the owner
may want to try it.

### Choosing the exponent

With the corrected formula, compression is far more effective than the earlier
draft implied, so the shipped value moves from 0.75 to **0.80**. Measured
against Meirok (455), post-redistribution defences, as `hit% / crit%`:

| defence | p=1.0 | p=0.95 | p=0.90 | p=0.85 | **p=0.80** | p=0.75 |
|---|---|---|---|---|---|---|
| Storm + redistrib (86) | 100/99 | 100/91 | 98/73 | 94/53 | **88/37** | 81/27 |
| Sand + redistrib (140) | 100/96 | 99/83 | 97/63 | 92/45 | **85/32** | 78/23 |
| Queen + redistrib (168) | 100/93 | 99/77 | 95/57 | 90/41 | **83/29** | 76/22 |
| Royal + redistrib (276) | 97/64 | 92/47 | 87/35 | 80/26 | **74/20** | 69/16 |
| parity (455) | 50/7 | 50/7 | 50/7 | 50/7 | 50/7 | 50/7 |

**Shipped at 0.80 (owner, splitting the difference between 0.75 and 0.85).**
It keeps a strong character clearly dominant on hit rate (74-88%) while pulling
crit off the ceiling into a 20-37% band. 0.85 is gentler (80-94% hit, 26-53%
crit); 0.75 is stronger on crit (16-27%) but drops hit against royal fighters to
69%, which is likely to read as whiffing at content you should beat. Dial in
play; parity is invariant at every value.

**Parity is invariant at every exponent.** Compression only ever touches
mismatches, so it cannot disturb an even fight. That is the property that makes
this safe to ship behind a knob.

`p = 0.5` (true square root) is also modelled and available; it drops Meirok to
55% hit against royal fighters, which is likely to read as whiffing at content
he should beat. Start at 0.75.

---

## 2b. Archetype stat distribution

**Owner decision, 2026-08-30:** *"we should make a change there and have the
archetypes spread stats a bit more evenly like .25 to primary and .15 to
non-primary or similar."*

### Today

`mobs/mobs.go:546-560`, per point of pool:

| archetype | primary group | per stat | non-primary group | per stat |
|---|---|---|---|---|
| `fighting` | Str/Dex/Vit **80%** | 26.7% | Per/Wil/Cha 20% | **6.7%** |
| `casting` | Per/Wil/Cha **80%** | 26.7% | Str/Dex/Vit 20% | **6.7%** |
| `""` | uniform | 16.7% | uniform | 16.7% |

A casting mob's Dexterity therefore receives **one fifteenth** of its pool, and
Dexterity is the whole physical defence term.

### Proposed

Author the weights as the owner stated them and **normalise**, since
`3 × 0.25 + 3 × 0.15 = 1.2`:

```yaml
ArchetypePrimaryStatWeight:   0.25   # per primary stat, before normalisation
ArchetypeSecondaryStatWeight: 0.15   # per non-primary stat
```

Normalised: **0.2083 per primary, 0.1250 per non-primary** — a 62.5 / 37.5
group split against today's 80 / 20. Keeping the raw weights in config and
normalising in code means the author writes the ratio they mean and cannot
create a distribution that does not sum to 1.

`ArchetypeSecondaryStatWeight` equal to the primary weight reproduces the
uniform `""` archetype, so the knob spans the full range from today's
specialisation to no archetype at all.

### ⚠️ The tension, measured

The pool is fixed, so **raising the non-primary share necessarily lowers the
primary share.** Casters gain physical defence; fighters lose it.

Against Meirok's melee attack score of 455:

| mob | def now | def new | crit today | + compression only | + both |
|---|---|---|---|---|---|
| Sand Elemental (fighting, 650) | 178 | **140** | 91.4% | 42.0% | **55.4%** |
| Storm Elemental (casting, 650) | 48 | **86** | 99.7% | 94.2% | **79.6%** |
| Royal fighter (1300) | 352 | **276** | 33.4% | 13.5% | **21.5%** |
| Elemental Queen (casting, 1300) | 92 | **168** | 98.8% | 77.1% | **45.5%** |

Read the two middle columns together. **Redistribution helps the casters and
hurts the fighters**, exactly as the fixed pool requires: the Queen falls from
98.8% to 45.5% while the sand elemental rises from 42.0% to 55.4% and the royal
fighter from 13.5% to 21.5%.

**Read the fighter row carefully: redistribution ALONE makes the royal fighters
markedly worse**, from 31.5% to 66.7% crit, because they surrender 76 points of
Dexterity. Only the two changes together land the roster in a sane band.

**Neither change is shippable without the other.** Compression alone leaves the
Queen at 97.5%; redistribution alone regresses the fighters. That is the whole
reason they share a spec.

### What it does not fix

The **Storm Elemental is still crit ~80% of the time with both changes.** A
casting archetype on a 650 pool yields a defence score of 86 against an attack
score of 455; redistribution nearly doubles its defence (48 to 86) and
compression takes it from 99.7% to 79.6%, but a gap that large cannot be closed
from the contest side alone. The owner's stated preference is to address the
residue on the damage side (ordinary hits plus glancing blows) rather than by
pushing the exponent lower.

### Second-order effect

Flattening the distribution also flattens mob **offence**: a casting mob's
Willpower drops from 26.7% to 20.8% of pool, so its spells hit slightly softer,
and a fighting mob's Strength drops likewise. Archetypes become less distinct in
both directions. That is inherent to the change rather than a defect, but it
should be felt for in the playtest rather than discovered later.

---

## 3. Configuration

```yaml
ContestGapCompression: 0.80   # exponent on the score gap; 1.0 = no compression
ContestGapCompressBothWays: false
```

**Validator shape is load-bearing.** An absent key unmarshals to **0**, and
`|gap|^0 == 1` — every mismatch in the game would collapse to a one-point gap.
The validator must therefore key on `<= 0`, not `< 0`:

```go
if b.ContestGapCompression <= 0 || b.ContestGapCompression > 1.0 {
    b.ContestGapCompression = 1.0
}
```

This is the same trap that left `StealCooldown`, `StealHiddenBonus`,
`ShadowCooldown`, `SneakFailCooldown` and `PackScatterRounds` pinned at zero,
found on 2026-08-30. Values above 1.0 are refused because they would *expand*
gaps, which is not what this knob is for.

---

## 4. Where it applies

`combat.RunContest` (`internal/combat/run_contest.go`) — the single entry point,
**25 production call sites**. Applying it there covers every opposed contest at
once, which is both the appeal and the risk.

### Blast radius, stated plainly

This is **not a melee change.** It reaches melee, ranged, every special move,
spells, taunt/social, steal, plant, sneak, shadow detection, flee, and the
grapple family. Everything that resolves attacker-versus-defender.

Deliberately **not** covered: static-difficulty rolls, which bypass `RunContest`
by design and are documented as roadmap categories B and C —
`contest.AgainstDifficulty` in `actions/search.go` and `actions/track.go`.
`RunContest`'s own docstring forbids routing them here.

**Consequence worth naming:** stealth and theft become harder for a strong thief
against a weak mark, and flee becomes harder for a strong runner. Those are
mismatch-dependent contests too, and they will move. Whether that is desirable
is a judgement the playtest has to make, not something this spec can assert.

---

## 5. What this does NOT fix

- **The crit damage cliff.** Crits still skip mitigation, so a crit is still
  worth up to 17.6× against an armored target. Compression reduces how *often*
  that happens, not what it is worth. Making crits mitigate before multiplying
  is a separate, larger decision.
- **`SkillWeight` 5.0 vs mob skill 1**, the root of the gap.
- **Archetype-driven defence collapse**, which is why the Queen defends like
  trash.

If crit rate is still unsatisfying at `p = 0.75`, the owner's stated preference
is to tune the damage side (ordinary hits plus glancing blows) rather than push
the exponent lower.

---

## 6. Testing

| Area | Test |
|---|---|
| Identity | `p = 1.0` reproduces current outcomes exactly, for a fixed seed |
| Parity invariance | equal scores give 50% hit and the parity crit rate at EVERY exponent |
| Monotonicity | lower `p` never increases crit rate for a fixed matchup |
| Ahead-only | an underdog's hit rate is unchanged from today at every exponent |
| Absent key | empty `Balance{}` validates to 1.0, never 0 |
| Out of range | negative, zero and `> 1.0` all clamp to 1.0 |
| Distribution | a large-sample run reproduces the modelled table within tolerance |
| Guard | `RunContest` remains the only site applying compression |
| Weights normalise | any positive primary/secondary pair produces a distribution summing to 1 |
| Weight identity | equal primary and secondary weights reproduce the uniform `""` archetype exactly |
| Distribution shape | a large spawn sample lands each stat within tolerance of its intended share, per archetype |
| Absent weights | empty `Balance{}` validates to today's 80/20, not to 0/0 |

The modelled tables in §1 and §2 are single-defence approximations. Melee rolls
best-of-all defences and takes the widest margin, so **absolute** shipped rates
will differ; the relative ordering between exponents is what the model
establishes. The distribution test must be written against live dice, not
against the model.

---

## 7. Risks

1. **Largest blast radius of any change in the arc.** One line moves every
   opposed contest. The knob defaulting to identity is the mitigation: it can be
   switched off in production without a rebuild.
2. **Hit rates move as well as crit rates.** Meirok drops to 66% against royal
   fighters at `p = 0.75`. If that reads as whiffing, the answer is a higher
   exponent, not abandoning the approach.
3. **Non-combat contests move too** — steal, sneak, flee. Named in §4 so the
   playtest looks for them rather than being surprised.
4. **Interaction with `ContestFloor` (0.125) is untested.** A compressed gap
   sits closer to the floor, so the floor may bind more often. Worth measuring.
5. **Shipping half of this is worse than shipping none.** Redistribution without
   compression regresses every `fighting` mob's physical defence, and the royal
   fighters were the one tier already behaving reasonably. If the plan is split
   across PRs, they must land together or behind a single switch.
6. **Existing mob instances keep their rolled stats.** Distribution happens once
   at spawn (`mobs.go:546`), so already-spawned instances are unaffected until
   they despawn. An instance-save wipe is needed to see the change on existing
   content, per the standing smoke-test SOP.
