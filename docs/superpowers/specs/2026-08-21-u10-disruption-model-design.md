# U10 — the disruption model

**Status:** design ratified 2026-08-21; **rev 2 after the three-reviewer
adversarial pass** (13 + 12 + 10 verified findings). Rev 2 corrections the
reader must not miss: the hold-rate table is recomputed on the ENGINE's
variance model and relabeled (rev 1 mislabeled prone as 250); the throttle
cast-interrupt is claimed (rev 1 dropped a roadmap-assigned site); surprise
attack is explicitly split out as U10d; progression is **success-only**,
superseding rev 1's win-or-lose. Roadmap source:
`docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md` §U10 + the unowned-sites table
(~line 115). Conventions: U6b (one contest seam), U9 (one progression path).

## 1. Problem

Four resolution sites still predate the arc — the third through sixth
instances of the ×0-skill defect the arc exists to kill:

1. **Concentration** (`CalcConcentrationChance`,
   `internal/characters/cast_helpers.go:48`) is
   `base(50) + Wil/divisor(4) − damagePct`, clamped [5, 95]. **No
   spellcasting term at all.** Two live callers feed it:
   - the **damage path**: `checkConcentrationBreak`
     (`internal/hooks/combat_shared_helpers.go:123`), fired from
     `handlePlayerConcentrationBreak` (`NewRound_DoCombat_helpers.go:857`)
     when damage lands on a casting **player** (mobs have no damage-path
     break today; U10 preserves that asymmetry). Any damage ≥1 rolls
     (`damagePct` floored to 1), so chip damage generates rolls.
   - the **position path**: `processFoldRound`
     (`combat_shared_helpers.go:549-581`) rolls once per fold round from
     `position.PositionDisruptionDmgEquiv` (chunk 4f: 13 positions ×
     controller/controlled, equivalents 25–70).
2. **The throttle cast-interrupt** (`internal/actions/combat_throttle.go:151`,
   inside `ExecuteThrottle`): a landed throttle on a casting victim cancels
   the cast on a flat `ThrottleInterruptChance` coin (**75%**, Go default —
   the knob is absent from `config.yaml`). It bypasses concentration
   entirely; the caster's Willpower and skill count for nothing.
3. **Knockdown** (`internal/combat/skill_moves.go:175`, inside
   `executeSkillMoveWithRunner`): `dice.RollStat(50) < KnockdownChance` —
   flat per-move percentages (shipped bash/gore/pounce 50, trip 60, kick 35;
   the Go default for bash is 40 — the shipped yaml values are the anchor).
   The defender's stats and skills contribute **nothing**; the only defence
   is the control-immunity mutation gate.
4. **Prone recovery** (`Character.AttemptRecovery`,
   `internal/characters/skills.go:47`): after `MinRecoveryRounds`, a solo
   roll `25 + 20·ln(dex/25)` (cap 90) vs `dice.RollStat(50)`. No opponent
   term, no skill term.

Knobs that exist only to feed dead paths: `SpellConcentrationBase`,
`SpellInitiationWillpowerDivisor`, `ThrottleInterruptChance`, and the three
`*KnockdownChance` percentages.

**Explicitly OUT of this slice: the surprise-attack redesign.** The roadmap
had assigned it to U10; owner decision 2026-08-21 splits it out as **U10d**
(its own brainstorm → spec → plan, per the 2026-08-13 decision that it is a
redesign, not a migration). This slice adds the U10d row to the roadmap and
changes nothing in `surprise_attack.go`.

## 2. Concentration becomes a contest

**Caster score: `Wil + spellcasting × SkillWeight (5.0)` against a static
difficulty set by what happened.** Resolution is a new
`combat.RunConcentrationContest(casterScore, difficulty)` — the floored twin
of `contest.AgainstDifficulty`, built the same way `RunContest` is: it calls
`contest.RunWithFloors` with a single static entry and is the ONE place the
concentration floor (§2.3) is read. (`AgainstDifficulty` itself is unfloored
and keeps zero production callers.) Guard obligations, both of them: a row in
`internal/combat/contest_site_guard_test.go` AND an exemption in the
root-level `contest_floor_guard_test.go` (`guardedRollExemptions["contest"]`),
which fails any new `contest.RunWithFloors` caller file it does not know.

### 2.1 Difficulties

| Disruption | Difficulty |
|---|---|
| Damage from a single source | `damagePct × 10`, **no roll below 10%** |
| Position, per (position, role) | `PositionDisruptionDmgEquiv × 10` |

The position lattice keeps chunk 4f's full granularity at face value
(owner 08-21, **re-ratified over the corrected table below**): supine and
guard-bottom 250, **prone 300**, clinch 400, mount-controlled 600,
crucifix-controlled 700, and so on — the existing equivalents times ten,
one conversion rule, no new knob. This is deliberately HARSHER for low/mid
casters than the roadmap's 3-row summary intended (its "prone 250" row would
have given a journeyman 46% instead of the shipped 16%); the owner chose the
lattice's relativities with the corrected numbers in hand. Retuning later
means editing the lattice values, not the conversion.

The damage threshold replaces today's "any damage ≥1 rolls": chip damage
below 10% of the pool no longer generates a roll at all. **Named
consequence:** fast weak attackers lose their anti-caster role entirely — a
swarm of sub-10% hits never threatens a cast (today each chip hit broke a
Wil-100 caster ~26% of the time). Ratified with the threshold. Knob:
`ConcentrationDamageThresholdPct` (default 10).

### 2.2 Hold rates — ENGINE model (recomputed rev 2)

Rev 1's table was computed with independent per-side variances and labeled
"verified"; the engine rolls **both sides with the attacker's stdDev**
(`contest.go:97-103`), so the margin's σ is `RollSpread × casterScore × √2`,
and the floor **flips** outcomes (probability `f`, observed
`p + f(1−2p)`) rather than clamping. Correct numbers, correct labels:

| Caster (score) | 250 supine/guard-btm | **300 PRONE** | 400 clinch | 600 mount-ctl | 700 crucifix-ctl |
|---|---|---|---|---|---|
| novice (125) | 2% | 2% | 2% | 2% | 2% |
| journeyman (245) | 46% | **16%** | 2% | 2% | 2% |
| adept (335) | 87% | **68%** | 19% | 2% | 2% |
| Meirok (403) | 94% | **87%** | 51% | 3% | 2% |

Damage column examples (difficulty = pct×10): a 25% hit (250) reads as the
first column; a 40% hit (400) as the clinch column — Meirok holds a 40% hit
about half the time, a journeyman never does.

Grapple stays a deliberate caster-killer, and deep controlled positions stop
everyone. **Rejected (roadmap, modelled): opposed against the disruptor** for
the passive position/damage paths — it hard-counters casting outright at the
top, so endgame bosses would switch casting off. (The throttle interrupt IS
opposed — see §3 — because it is an active move by a specific attacker, not
ambient state.)

### 2.3 The concentration floor

**`ConcentrationFloor: 0.02`** (owner 08-21), symmetric flip: with
probability 0.02 the outcome inverts, so observed hold rates live in
[~2%, ~98%] — no hold is certain and no caster is hopeless. The standard
`ContestFloor` 0.125 would break a master 1-in-8 per qualifying disruption.
One new knob, read only inside `RunConcentrationContest`.

### 2.4 What survives unchanged

- **`NoDamageInterrupt`** telegraph semantics: both concentration paths skip
  flagged casts exactly as today. (The throttle interrupt honors it too —
  see §3.)
- **Layering**: the damage path and the per-round position path can each
  break a single cast independently.
- Break messaging keeps its current routing; no raw numbers in player text.
- Mob casters run the identical position path; mob spellcasting is 1
  everywhere, so mob holds are Willpower-driven, which is correct.

### 2.5 Deletions

- `CalcConcentrationChance`, its tests, and the stale "U9 removes it"
  comment (`cast_helpers.go:41-43`).
- `SpellConcentrationBase`, `SpellInitiationWillpowerDivisor` (Go fields at
  `config.balance.go:487-488`, defaults at `config.balance.spells.go:18-22`,
  `config.yaml` ~1201-1209).
- Stale doc references in `internal/state/position/disruption.go` comments
  and the `internal/state/{position,control,activity}` +
  `internal/characters` + `internal/hooks` context.md files.

## 3. The throttle interrupt becomes an OPPOSED contest

Owner decision 2026-08-21: unlike the passive disruption difficulties, a
throttle is an active move by a specific attacker, so it resolves as a true
opposed roll through `combat.RunContest` (standard `ContestFloor`):

- **Throttler:** `Dex + unarmed-combat × SkillWeight`
- **Caster:** `Wil + spellcasting × SkillWeight`
- Contest success = the cast is interrupted (existing
  `InterruptTargetCast` path, 50% conviction refund — unchanged).
- `NoDamageInterrupt` casts skip the contest entirely (today the flat coin
  does not check the flag; a telegraphed boss cast should not be
  throttle-cancellable when incidental damage cannot break it either — this
  is a deliberate small behaviour fix, called out in the PR).
- `ThrottleInterruptChance` is deleted (field `config.balance.go:248`,
  defaults `config.balance.misc.go:251-256`; it never shipped in
  `config.yaml`). At parity the contest gives ~50% vs today's flat 75%:
  throttling a peer caster gets harder, throttling up-tier much harder,
  down-tier easier. Deliberate.
- Progression: the caster fires one spellcasting event **only on a
  successful hold** (§6). The throttler's unarmed use was already progressed
  by the throttle's own hit resolution; no new attacker event.

## 4. Knockdown becomes an opposed contest

The post-hit knockdown roll in `executeSkillMoveWithRunner` becomes
`combat.RunContest` (standard floor; site allowlisted):

- **Attacker score:** `p.Attack.score() × KnockdownFactor` (the move's
  existing attack basis times a per-move factor).
- **Defender score:** `Dex + unarmed-combat × SkillWeight`.
- **Control immunity** stays a hard gate before the contest; factor 0 means
  the move has no knockdown component (no contest, no progression).

**Per-move differentiation survives as a score factor**, parity-anchored to
the SHIPPED percentages (engine model, rev 2 exact values):

| Move | shipped chance | `KnockdownFactor` |
|---|---|---|
| bash / gore / pounce | 50% | **1.000** |
| trip | 60% | **1.057** |
| kick | 35% | **0.924** |

Named consequence of anchoring to shipped yaml (50) rather than the old Go
default (40): a deployment that omitted the key shifts bash 40%→50% at
parity. Accepted — prod ships the key.

Skill-gap behaviour (factor 1.0, `ContestFloor` 0.125 flip): novice-150 vs
master-500 knocks down **12%**; parity **50%**; master-500 vs novice-150
**87%**. A novice cannot reliably trip a master; nothing is certain either
way. Old flat rolls had every matchup identical.

Renames: `BashKnockdownChance`/`TripKnockdownChance`/`KickKnockdownChance`
(ConfigInt, %) become `BashKnockdownFactor`/`TripKnockdownFactor`/
`KickKnockdownFactor` (ConfigFloat) — a knob whose semantics change must not
keep its old name. `SkillMoveParams.KnockdownChance int` becomes
`KnockdownFactor float64`; **all fifteen production setters move** (ten
`internal/actions/combat_*.go` including `combat_throttle.go:125`,
`internal/combat/counter.go:119`, and the counter-tier auto-trip/auto-bash
at `internal/hooks/combat_shared_helpers.go:299,358`), plus five
`internal/combat` test files that construct the field.

## 5. Prone recovery becomes an opposed contest

`AttemptRecovery` keeps its shape (position gate, `MinRecoveryRounds`
consumption, `ConditionRecoveryPenalty`, FSM transition) and replaces only
the roll:

- **Contested when someone is actually holding you down**, defined as: any
  living actor in the recoverer's room whose **aggro is ON the recoverer**
  (rev 2 fix — rev 1 keyed on the recoverer's OWN aggro, which free-stood a
  passive victim mid-fight and invited a drop-target-stand-retarget loop).
  Opponent = the strongest such holder by recovery score. Both sides score
  `Dex + unarmed-combat × SkillWeight`; resolved through `combat.RunContest`
  (standard floor).
- **Nobody aggroing the recoverer** (out of combat, room emptied, or purely
  one-sided aggro FROM the recoverer): stand automatically once
  `MinRecoveryRounds` is consumed. The solo Dex curve is deleted.

**Feel numbers, stated (rev 2):** at parity the contest stands you ~50% per
round (old curve at DEX 100: ~64%) — expected time down rises slightly,
~2 rounds after the minimum. Against a hopeless gap the floor stands you
12.5% per round: **mean ~8 rounds on the mat after the minimum**, taking one
attack per round (`ConditionRecoveryPenalty` re-applies each failed round).
That is a real boss-fight feel change from today's opponent-blind ~64%;
accepted for now, tuned post-arc-playtest if it plays badly (the lever is
`ContestFloor` or a recovery-specific floor added then, not now).

## 6. Progression (U9 seam) — SUCCESS-ONLY

Owner decision 2026-08-21 (superseding the same-day win-or-lose call):
**U10's contests adopt U10b's convention from birth — one event per
SUCCESS**, so U10b inherits nothing to rework here:

- **Concentration** (damage path, position path, throttle defence): the
  caster fires ONE `OnSkillUse("spellcasting")` only when the hold
  SUCCEEDS.
- **Knockdown defence:** the defender fires `OnSkillUse("unarmed-combat")`
  only when they RESIST the knockdown.
- **Recovery:** the recoverer fires `OnSkillUse("unarmed-combat")` only when
  they STAND. A free (uncontested) stand fires nothing.
- No new attacker-side events anywhere: the throttle/bash/trip/kick hit
  already progressed through U9's seam; a second event per swing is the
  duplication U9 deleted.

U10b's roadmap row should note these sites are already on its convention.

## 7. Owner decisions (2026-08-21, superseding order within the day: latest wins)

1. **Full position lattice kept, ×10, no scale knob** — re-ratified over the
   corrected (harsher) table in §2.2.
2. **Recovery contested only by actors aggroing the recoverer; free stand
   otherwise.**
3. **`ConcentrationFloor: 0.02`**, own knob.
4. **Progression success-only** (U10b's convention from birth; supersedes
   the morning's win-or-lose answer).
5. **Throttle interrupt = opposed roll** (throttler Dex+unarmed vs caster
   Wil+spellcasting), claimed by U10.
6. **Surprise attack split out as U10d** — roadmap row added, nothing in
   this slice touches it.

## 8. Done when

Each criterion ships as a test (U6b lesson):

1. `CalcConcentrationChance` does not exist; no production site computes a
   concentration percentage outside `RunConcentrationContest`.
2. `combat.RunConcentrationContest` is the only concentration resolver, both
   guards name it, and a source-scan test asserts it is the only reader of
   `ConcentrationFloor`.
3. The knockdown, recovery, and throttle rolls are `RunContest` sites on the
   allowlist; `dice.RollStat(50)` appears at neither converted site and
   `util.Rand` no longer decides the throttle interrupt.
4. `SpellConcentrationBase`, `SpellInitiationWillpowerDivisor`, and
   `ThrottleInterruptChance` are gone from Go and (where present) yaml;
   `ConcentrationFloor`, `ConcentrationDamageThresholdPct`, and the three
   `KnockdownFactor` knobs exist with shipped values. (Asserted against the
   HEAD blob via `git show`, not the skip-worktree disk copy.)
5. A sub-threshold hit (< 10% of pool) on a folding caster produces no
   concentration roll; a supra-threshold hit produces exactly one.
6. Parity calibration: at equal scores the knockdown rates reproduce
   50/60/35 within the floor-adjusted expectation `p + F(1−2p)` ± 2 points
   (statistical, 200k/cell).
7. Progression: exactly one event on a WON contest and zero on a lost one,
   for concentration, knockdown defence, and recovery — asserted by
   `SkillUseCount` deltas, not prose.
