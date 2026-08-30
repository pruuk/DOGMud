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

| score ratio | crit rate |
|---|---|
| 1.00 (parity) | 6.7% |
| 1.20 | 21.9% |
| 1.40 | 43.0% |
| 1.50 | **53.3%** |
| 2.00 | 86.1% |
| 3.00 | 98.6% |

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

| mob | pool | def score | ratio | crit |
|---|---|---|---|---|
| Storm Elemental | 325 | 27 | 17.1× | ~100% |
| Sand Elemental | 325 | 92 | 4.96× | ~100% |
| Elemental Queen | 1300 | 92 | 4.96× | ~100% |
| King / Prince / Princess | 1300 | 352 | 1.29× | ~32% |

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

Compress the score gap before it is rolled:

```
effectiveAttack = defence + (attack - defence) ^ p        when attack > defence
effectiveAttack = attack                                  otherwise
```

with `p` a config knob, shipped **0.75**. `p = 1.0` is exactly today's
behaviour, so the knob is a true identity at its default.

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

### Measured effect at the shipped value

`p = 0.75`, ahead-only:

| matchup | hit% now → then | crit% now → then |
|---|---|---|
| Meirok vs Sand ellie | 100 → 100 | **100 → 82** |
| Meirok vs royal fighter | 88 → 66 | 31 → 12 |
| mid vs trash | 99 → 86 | **75 → 27** |
| parity | 50 → 50 | 2.3 → 2.3 |
| underdog | 0.6 → 0.6 | 0 → 0 |

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

| mob | Dex now | Dex new | crit now | crit new | new + `p=0.75` |
|---|---|---|---|---|---|
| Elemental Queen (casting, 1300) | 87 | **162** | 99.9% | 97.5% | **44.8%** |
| Royal fighter (1300) | 347 | **271** | 31.5% | **66.7%** | **19.7%** |
| Sand Elemental (fighting, 325) | 87 | 68 | 99.9% | 100% | 91.4% |
| Storm Elemental (casting, 325) | 22 | 41 | 100% | 100% | 98.6% |

**Read the fighter row carefully: redistribution ALONE makes the royal fighters
markedly worse**, from 31.5% to 66.7% crit, because they surrender 76 points of
Dexterity. Only the two changes together land the roster in a sane band.

**Neither change is shippable without the other.** Compression alone leaves the
Queen at 97.5%; redistribution alone regresses the fighters. That is the whole
reason they share a spec.

### What it does not fix

The **Storm Elemental stays at ~99% crit even with both changes.** A 325 pool
under a casting archetype yields Dexterity 41 against an attack score of 455;
no redistribution of a pool that small closes a gap that large. Trash remaining
highly crittable is arguably correct, and the owner's stated preference is to
address it on the damage side (ordinary hits plus glancing blows) rather than by
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
ContestGapCompression: 0.75   # exponent on the score gap; 1.0 = no compression
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
