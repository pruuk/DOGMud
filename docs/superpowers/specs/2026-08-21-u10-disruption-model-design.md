# U10 — the disruption model

**Status:** design ratified 2026-08-21 (owner answered the four open questions
listed in §6). Roadmap source: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`
§U10. Successor conventions: U6b (one contest seam), U9 (one progression
path).

## 1. Problem

Three resolution sites still predate the arc, and together they are the
third, fourth and fifth instances of the ×0-skill defect the arc exists to
kill:

1. **Concentration** (`CalcConcentrationChance`,
   `internal/characters/cast_helpers.go:48`) is
   `base(50) + Wil/divisor(4) − damagePct`, clamped [5, 95]. **No
   spellcasting term at all** — a master's skill contributes nothing to
   holding a spell together. Two live callers feed it:
   - the **damage path**: `checkConcentrationBreak`
     (`internal/hooks/combat_shared_helpers.go:123`), fired from
     `NewRound_DoCombat_helpers.go:857` when damage lands on a caster; any
     damage ≥1 triggers a roll (`damagePct` floored to 1), so chip damage
     generates rolls;
   - the **position path**: `processFoldRound`
     (`combat_shared_helpers.go:549-581`) rolls once per fold round from
     `position.PositionDisruptionDmgEquiv` (chunk 4f: 13 positions ×
     controller/controlled, equivalents 25–70).
2. **Knockdown** (`internal/combat/skill_moves.go:175`):
   `dice.RollStat(50) < KnockdownChance` — bash/gore/pounce 50, trip 60,
   kick 35 (config). The defender's stats and skills contribute **nothing**;
   the only defence is the control-immunity mutation gate.
3. **Prone recovery** (`Character.AttemptRecovery`,
   `internal/characters/skills.go:47`): after `MinRecoveryRounds`, a free
   roll `25 + 20·ln(dex/25)` (cap 90) vs `dice.RollStat(50)`. No opponent
   term, no skill term.

Knobs `SpellConcentrationBase` and `SpellInitiationWillpowerDivisor` exist
only to feed the dead curve.

## 2. Concentration becomes a contest

**Caster score: `Wil + spellcasting × SkillWeight (5.0)` against a static
difficulty set by what happened.** Resolution is a new
`combat.RunConcentrationContest(casterScore, difficulty)` — the floored twin
of `contest.AgainstDifficulty`, built the same way `RunContest` is: it calls
`contest.RunWithFloors` with a single static entry and is the ONE place the
concentration floor (§2.3) is read. (`AgainstDifficulty` itself is unfloored
and stays without production callers; the site guard gains a row naming the
new seam.) `combat.RunContest` remains the seam for every opposed contest.

### 2.1 Difficulties

| Disruption | Difficulty |
|---|---|
| Damage from a single source | `damagePct × 10`, **no roll below 10%** |
| Position, per (position, role) | `PositionDisruptionDmgEquiv × 10` |

The position lattice keeps chunk 4f's full granularity (owner 08-21):
prone 300, supine 250, clinch 400, mount-controlled 600,
crucifix-controlled 700, guard-bottom 250, and so on — the existing
equivalents times ten, expressed through one conversion, not a re-authored
table. The roadmap's 3-row summary (prone 250 / grappled 300) is superseded
by the lattice it was summarizing; the ×10 rule is the design.

The damage threshold replaces today's "any damage ≥1 rolls": chip damage
below 10% of the pool no longer generates a roll at all. Threshold knob:
`ConcentrationDamageThresholdPct` (default 10).

### 2.2 Hold rates (verified 2026-08-21, opposed-normal model, RollSpread 0.15)

| Caster (score) | vs 250 (prone) | vs 300 (clinch-adjacent) | vs 400 (40% hit / clinch) | vs 700 (crucifix-controlled) |
|---|---|---|---|---|
| novice (Wil 100, sc 5 → 125) | 2% | 2% | 2% | 2% |
| journeyman (Wil 120, sc 25 → 245) | 46% | 17% | 2% | 2% |
| adept (Wil 135, sc 40 → 335) | 91% | 70% | 20% | 2% |
| Meirok (Wil 148, sc 51 → 403) | 98% | 91% | 51% | 2% |

Grapple stays a deliberate caster-killer: below adept it effectively stops
casting, and deep controlled positions stop everyone. **Rejected (roadmap,
modelled): opposed against the disruptor** — it hard-counters casting
outright at the top (Meirok holds 0% vs the Elemental King's grapple), so
endgame bosses would simply switch casting off.

### 2.3 The concentration floor

**`ConcentrationFloor: 0.02`** (owner 08-21), symmetric → hold chance
clamped to [2%, 98%]. The standard `ContestFloor` 0.125 would break a
master 1-in-8 per qualifying disruption — three times worse than the
initiation tax U0 deleted for being annoying. One new knob in
`config.yaml`; applied only inside `RunConcentrationContest`.

### 2.4 What survives unchanged

- **`NoDamageInterrupt`** telegraph semantics: both paths skip flagged
  casts exactly as today.
- **Layering**: the damage path and the per-round position path can each
  break a single cast independently.
- Break messaging (prone/grapple/damage variants) keeps its current
  routing; no raw numbers appear in player text (difficulties are internal).
- Mob casters run the identical path (`processFoldRound` is
  character-generic); mob spellcasting is 1 everywhere, so mob holds are
  driven by Willpower, which is correct.

### 2.5 Deletions

- `CalcConcentrationChance` and its tests.
- `SpellConcentrationBase`, `SpellInitiationWillpowerDivisor` (Go fields,
  validation defaults, `config.yaml` entries, and the stale "U9 removes it"
  comment at `cast_helpers.go:41`).

## 3. Knockdown becomes an opposed contest

The post-hit knockdown roll in `ExecuteSkillMove` becomes
`combat.RunContest` (site added to the owned-site allowlist):

- **Attacker score:** the move's existing attack-score basis
  (stat + governing skill × 5), times a per-move factor.
- **Defender score:** `stat + unarmed-combat × 5` (roadmap; the stat is the
  same one the defender's existing knockdown-relevant physique reads on,
  i.e. Dexterity, matching the melee defence convention).
- **Control immunity** (Living Carapace, Ossified Frame) stays a hard gate
  before the contest, exactly as today.

**Per-move differentiation survives as a score factor, not a percentage.**
`KnockdownChance` (a %) is replaced by `KnockdownFactor` (a multiplier on
the attacker's score), calibrated so the knockdown rate at score parity
equals today's shipped rate:

| Move | old chance | `KnockdownFactor` |
|---|---|---|
| bash / gore / pounce | 50% | **1.000** |
| trip | 60% | **1.055** |
| kick | 35% | **0.921** |

(Verified 2026-08-21 against the opposed-normal model at RollSpread 0.15.)
Away from parity the rate now scales with the matchup, which is the point:
a novice cannot trip a master, and a master's kick floors a novice. Config:
`BashKnockdownChance`/`TripKnockdownChance`/`KickKnockdownChance` are
renamed to `...Factor` with the new values — a knob whose semantics change
must not keep its old name.

Standard `ContestFloor` applies (it is a RunContest site): no knockdown is
ever certain in either direction.

## 4. Prone recovery becomes an opposed contest

`AttemptRecovery` keeps its shape (position gate, `MinRecoveryRounds`
consumption, recovery-penalty condition, FSM transition) and replaces only
the roll:

- **With a living aggro target** (owner 08-21): recoverer
  `Dex + unarmed-combat × 5` vs that opponent's `Dex + unarmed-combat × 5`,
  through `combat.RunContest` (allowlisted site). Standard floor applies,
  so even a hopeless matchup stands eventually (12.5% per round after the
  minimum) and nobody is pinned forever.
- **No living opponent** (out of combat, or the room emptied): stand
  automatically once `MinRecoveryRounds` is consumed. No roll against
  nobody, and no solo Dex curve retained.

The Dex-only log curve (`25 + 20·ln(dex/25)`) is deleted with the roll.

## 5. Progression (U9 seam)

- **Concentration**: a qualifying disruption contest fires ONE
  `OnSkillUse("spellcasting")` on the caster, win or lose — the
  channel-defence convention (owner 08-21). The initiation cast already
  fired its own progression; this is the defence-side event.
- **Knockdown / recovery**: the defender (knockdown) and the recoverer
  (recovery) fire one `OnSkillUse("unarmed-combat")` per contest, win or
  lose, same convention. The attacker side of knockdown fires nothing new —
  the special move's hit already progressed it (U9), and a second event per
  swing is exactly the duplication U9 spent a slice deleting.

## 6. Owner decisions (2026-08-21)

1. **Full position lattice kept**, ×10 conversion — not the 3-row summary.
2. **Recovery opposes the current aggro target; free stand when none.**
3. **`ConcentrationFloor: 0.02`**, own knob.
4. **Concentration fires progression through the U9 seam, win or lose.**

## 7. Done when

Each criterion ships as a test, not prose (U6b lesson):

1. `CalcConcentrationChance` does not exist; no production site computes a
   concentration percentage outside `RunConcentrationContest`.
2. `combat.RunConcentrationContest` is the only concentration resolver, the
   site guard names it, and it is the only reader of `ConcentrationFloor`.
3. The knockdown and recovery rolls are `RunContest` sites on the
   allowlist; `dice.RollStat(50)` appears at neither site.
4. `SpellConcentrationBase` and `SpellInitiationWillpowerDivisor` are gone
   from Go, validation, and `config.yaml`; `ConcentrationFloor`,
   `ConcentrationDamageThresholdPct`, and the three `KnockdownFactor` knobs
   exist with shipped values.
5. A sub-threshold hit (< 10% of pool) on a folding caster produces no
   concentration roll; a supra-threshold hit produces exactly one.
6. Parity calibration: at equal scores the knockdown rates reproduce
   50/60/35 (±2 points, statistical test seeded like the U6b parity gate).
7. Progression: one spellcasting event per concentration contest, one
   unarmed-combat event per knockdown defence and per recovery attempt —
   and none of them double-fire (U9's counters pin this).
