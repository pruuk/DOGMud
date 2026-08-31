# U10 — the disruption model

**Status:** design ratified 2026-08-21; **rev 3** after TWO three-reviewer
blind adversarial passes (rev 1: 35 findings; rev 2: 45 findings) and the
owner rulings each surfaced. Rev 3 headlines: the knockdown conversion is a
**named rebalance, not a preservation** (the old config lied about its own
rates — see §4); the throttle interrupt resolves through the concentration
seam at the 2% floor; manual `stand` is documented as the deliberate
pay-stamina bypass of the recovery contest; an adversarial playtest gate is
part of the slice. Roadmap source:
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
     (`damagePct` floored to 1), so chip damage generates rolls. DoT and
     condition ticks never route through this path — the chip threshold in
     §2.1 creates no new DoT immunity.
   - the **position path**: `processFoldRound`
     (`combat_shared_helpers.go:549-581`) rolls once per fold round from
     `position.PositionDisruptionDmgEquiv` (chunk 4f: 13 positions ×
     controller/controlled, equivalents 25–70).
2. **The throttle cast-interrupt** (`internal/actions/combat_throttle.go:151`,
   inside `ExecuteThrottle`): a landed throttle on a casting victim cancels
   the cast on a flat `ThrottleInterruptChance` coin (**75%**, Go default —
   the knob never shipped in `config.yaml`). It bypasses concentration
   entirely. Throttle is **mob-only by anatomy**: the gate at
   `combat_throttle.go:92` requires a handless, fanged species, so no
   player can ever use it — the interrupt is something done TO casters.
3. **Knockdown** (`internal/combat/skill_moves.go:175`, inside
   `executeSkillMoveWithRunner`): `dice.RollStat(50) < KnockdownChance`.
   **The config has been lying about its own rates**: `RollStat(50)` is
   Normal(50, 7.5), not a d100, so the knobs are thresholds on a normal
   curve. Shipped values produce TRUE live rates of bash/gore/pounce
   **50%**, trip **~91%** (near-guaranteed, not the claimed 60%), kick
   **~2.3%** (effectively never, not the claimed 35%). The defender's
   stats and skills contribute nothing either way; the only defence is the
   control-immunity mutation gate.
4. **Prone recovery** (`Character.AttemptRecovery`,
   `internal/characters/skills.go:47`): after `MinRecoveryRounds`, a solo
   roll (chance `25 + 20·ln(dex/25)`, cap 90, vs Normal(50, 7.5) — ~64%
   per round at DEX 100). No opponent term, no skill term. Note the manual
   `stand` command is separate and stays: it buys an **uncontested** stand
   for stamina (§5).

Knobs that exist only to feed dead paths: `SpellConcentrationBase`,
`SpellInitiationWillpowerDivisor`, `ThrottleInterruptChance`, and the three
`*KnockdownChance` thresholds.

**Explicitly OUT of this slice: the surprise-attack redesign.** The roadmap
had assigned it to U10; owner decision 2026-08-21 splits it out as **U10d**
(its own brainstorm → spec → plan, per the 2026-08-13 decision that it is a
redesign, not a migration). This slice adds the U10d row to the roadmap
(and re-homes the roadmap's `:121` and `:173` references), touching nothing
in `surprise_attack.go`.

## 2. Concentration becomes a contest

**The caster is the attack side of every concentration contest:
`Wil + spellcasting × SkillWeight (5.0)` against a disruption score.** For
the damage and position paths the disruption is a static difficulty; for
the throttle interrupt it is the throttler's live score (§3). Resolution is
a new `combat.RunConcentrationContest(casterScore, disruption)` — built
exactly like `RunContest`: it calls `contest.RunWithFloors` with a single
entry and is the ONE place `ConcentrationFloor` is read. `Success` = the
caster HELD, in every case. Guard obligations, both: a row in
`internal/combat/contest_site_guard_test.go` AND a file exemption in the
root-level `contest_floor_guard_test.go` (`guardedRollExemptions["contest"]`).

### 2.1 Static difficulties

| Disruption | Difficulty |
|---|---|
| Damage from a single source | `damagePct × 10`, **no roll below 10%** |
| Position, per (position, role) | `PositionDisruptionDmgEquiv × 10` |

The position lattice keeps chunk 4f's full granularity at face value
(owner 08-21, **re-ratified over the corrected table below**): supine and
guard-bottom 250, **prone 300**, clinch 400, mount-controlled 600,
crucifix-controlled 700 — the existing equivalents times ten, one
conversion rule, no new knob. This is deliberately HARSHER for low/mid
casters than the roadmap's 3-row summary intended (its "prone 250" row
would have given a journeyman 46% instead of the shipped 16%); the owner
chose the lattice's relativities with the corrected numbers in hand.
Retuning later means editing the lattice values, not the conversion.

The damage threshold replaces today's "any damage ≥1 rolls". **Named
consequence:** fast weak attackers lose their anti-caster role — a swarm of
sub-10% hits never threatens a cast (today each chip hit broke a Wil-100
caster ~26% of the time). Ratified with the threshold. Knob:
`ConcentrationDamageThresholdPct` (default 10; validation rewrites values
below 1, so "roll on any hit" is expressed as 1, not 0).

### 2.2 Hold rates — engine model

The engine rolls **both sides with the attacker's stdDev**
(`contest.go:97-103`), so the margin's σ is
`RollSpread × casterScore × √2`, and the floor **flips** outcomes
(probability `f`, observed `p + f(1−2p)`) rather than clamping:

| Caster (score) | 250 supine/guard-btm | **300 PRONE** | 400 clinch | 600 mount-ctl | 700 crucifix-ctl |
|---|---|---|---|---|---|
| novice (125) | 2% | 2% | 2% | 2% | 2% |
| journeyman (245) | 46% | **16%** | 2% | 2% | 2% |
| adept (335) | 87% | **68%** | 19% | 2% | 2% |
| Meirok (403) | 94% | **87%** | 51% | 3% | 2% |

Damage column examples: a 25% hit reads as the first column; a 40% hit as
the clinch column.

Grapple stays a deliberate caster-killer, and deep controlled positions
stop everyone. **Rejected (roadmap, modelled): opposed against the
disruptor** for the passive position/damage paths — it hard-counters
casting outright at the top. (The throttle IS opposed — §3 — because it is
an active move by a specific attacker, not ambient state; it still runs at
the concentration floor.)

### 2.3 The concentration floor

**`ConcentrationFloor: 0.02`** (owner 08-21), symmetric flip: observed hold
rates live in [~2%, ~98%]. The standard `ContestFloor` 0.125 would break a
master 1-in-8 per qualifying disruption — including per throttle, which is
why the throttle contest ALSO runs at this floor (owner 08-21). One new
knob, read only inside `RunConcentrationContest`.

### 2.4 What survives unchanged

- **`NoDamageInterrupt`** telegraph semantics: both concentration paths
  skip flagged casts as today, and the throttle interrupt now honors the
  flag too (a small deliberate fix — the old coin ignored it; only
  `core-discharge`/`core-drain` carry the flag, and since no player can
  throttle, no player counter-play is lost).
- **Layering**: damage path and per-round position path can each break a
  single cast independently.
- Break messaging keeps its routing; no raw numbers in player text.
- Mob casters run the identical position path.

### 2.5 Deletions and stale-copy sweep

- `CalcConcentrationChance`, its tests, the stale comment
  (`cast_helpers.go:41-43`).
- `SpellConcentrationBase`, `SpellInitiationWillpowerDivisor` (fields
  `config.balance.go:487-488`, defaults `config.balance.spells.go:18-23`,
  `config.yaml` ~1247-1255), `ThrottleInterruptChance` (field :248,
  defaults `config.balance.misc.go:251-256`).
- Stale prose that must not survive: `disruption.go` comments;
  `internal/state/{position,control,activity}`, `internal/characters`,
  `internal/hooks`, `internal/actions` context.md references;
  `config.yaml:540-541` ("concentration ... not floored" becomes false);
  the `AgainstDifficulty` doc example ("recovering from prone with nobody
  holding you down" — now a free stand, no roll); helpfiles
  `grapple.template:116`, `cast.template:16`, `spell.template:18`,
  `throttle.template:27`, `prone.template:40,53-54`, `stand.template:20,29`;
  `docs/roadmaps/CURRENT_BACKLOG.md:26-34`.

## 3. The throttle interrupt: opposed, through the concentration seam

Owner decisions 2026-08-21: opposed roll, at the concentration floor.

- **Caster (attack side): `Wil + spellcasting × SkillWeight`.**
- **Throttler (the disruption entry): `Dex + unarmed-combat × SkillWeight`.**
- Resolved by `combat.RunConcentrationContest(holdScore, gripScore)` —
  same seam, same floor, `Success` = held; on a loss the existing
  `InterruptTargetCast` path fires (50% conviction refund, unchanged).
- At parity ~50% interrupt vs today's flat 75%: throttling a peer caster
  gets harder, up-tier much harder, down-tier easier. Deliberate.
- `ThrottleInterruptChance` dies. The existing
  `TestThrottle_CastInterrupt` (`combat_throttle_test.go:165-266`) pins the
  coin via a string-keyed override of the deleted knob and must be
  reworked onto the contest, not left to rot.
- Progression: the caster fires one spellcasting event **only on a
  successful hold** (§6). The throttler's own progression came from the
  move's hit resolution; no new attacker event.

## 4. Knockdown becomes an opposed contest — A NAMED REBALANCE

The post-hit knockdown roll becomes `combat.RunContest` (standard floor;
site allowlisted):

- **Attacker score:** `p.Attack.score() × KnockdownFactor`.
- **Defender score:** `Dex + unarmed-combat × SkillWeight`.
- **Control immunity** stays a hard gate; factor 0 = no knockdown
  component (no contest, no progression).

**Anchoring (owner-ratified 08-21): the INTENDED rates, not the accidental
live ones.** The old knobs claimed 50/60/35 but delivered 50/~91/~2.3
(§1.3) — trip was near-certain and kick knockdown effectively did not
exist. The factors make the contest deliver the intended numbers at score
parity, which means **this ships as a rebalance**: trip drops from
near-guaranteed to a real contest, kick knockdown starts actually
happening. The PR names it; nothing claims preservation.

| Move | intended (ratified) | old TRUE live rate | `KnockdownFactor` |
|---|---|---|---|
| bash / gore / pounce | 50% | 50% | **1.000** |
| trip | 60% | ~91% | **1.057** |
| kick | 35% | ~2.3% | **0.924** |

(Factors solved on the engine model; the `ContestFloor` flip pulls
observed parity rates slightly toward even — `p + F(1−2p)` — which the
calibration test encodes.) Skill-gap behaviour at factor 1.0: novice-150
vs master-500 knocks down 12.5% (floor-bound); parity 50%; master vs
novice ~87%. The old rolls had every matchup identical.

Renames: the three `*KnockdownChance` knobs (ConfigInt thresholds) become
`*KnockdownFactor` (ConfigFloat) — a knob whose semantics change must not
keep its old name. `SkillMoveParams.KnockdownChance int` becomes
`KnockdownFactor float64`. **Fourteen struct-literal setters + two
local-variable feeds across eleven `internal/actions` files,
`internal/combat/counter.go:119`, and the counter-tier auto-trip/auto-bash
at `internal/hooks/combat_shared_helpers.go:299,358`** all move, plus five
`internal/combat` test files (several of which encode "guaranteed
knockdown", a semantics only reproducible with the contest floor pinned
off and an overwhelming factor).

## 5. Prone recovery: contested when held down, and the `stand` bargain

Two ways off the ground, by design (owner 08-21):

1. **The manual `stand` command is the paid, UNCONTESTED exit** — it costs
   stamina and stands you regardless of who is on you (existing behavior,
   now named as deliberate: paying stamina buys certainty).
2. **The free automatic recovery is what U10 contests.** `AttemptRecovery`
   keeps its shape (position gate, `MinRecoveryRounds`,
   `ConditionRecoveryPenalty`, FSM transition); only the roll changes:
   - **Contested when someone is holding you down**: any living, same-room
     actor in the recoverer's inbound-attacker set — via the prescribed
     accessor `Character.Attackers()` (`character.go:795`; direct `Aggro`
     reads are forbidden, `characters/context.md:961-964`). Opponent = the
     strongest such holder by recovery score. Both sides score
     `Dex + unarmed-combat × SkillWeight`; resolved through
     `combat.RunContest` (standard floor).
   - **Nobody attacking the recoverer**: stand automatically once
     `MinRecoveryRounds` is consumed. The solo Dex curve is deleted.

**Feel numbers, correctly framed:** the contest binds mobs and players who
cannot (or will not) pay stamina. At parity the free path stands you ~50%
per round (old opponent-blind curve: ~64% at DEX 100). Against a hopeless
gap the floor stands you 12.5% per round — mean ~8 rounds on the mat after
the minimum — but a player in that spot can always buy the exit with
`stand`. Accepted; post-arc playtest is the retune point.

## 6. Progression (U9 seam) — SUCCESS-ONLY

Owner decision 2026-08-21 (superseding the same-day win-or-lose call):
**one event per contest SUCCESS**:

- **Concentration** (damage path, position path, throttle defence): the
  caster fires ONE `OnSkillUse("spellcasting")` only when the hold
  succeeds. The position path fires per HELD fold round — the same
  per-round cadence melee progression already has; farming it requires a
  live aggressor actively hurting you, and the auto/free stand removes the
  idle case.
- **Knockdown defence:** the defender fires `OnSkillUse("unarmed-combat")`
  only on a RESIST. Note the channel seam ALREADY fires the defender's
  winning-defence skill once per contested swing win-or-lose
  (`AwardDefenceProgression`, `defence_multiplier.go:363-373`) — that
  existing award is untouched; the knockdown resist event is additional
  and tests must expect both.
- **Recovery:** the recoverer fires `OnSkillUse("unarmed-combat")` only on
  a contested STAND. Free stands fire nothing.
- No new attacker-side events anywhere.

**What U10b still owns here:** its full convention is "one event per
success **with crit and critical-failure as a separate bonus on top**."
U10 ships the success half only; the bonus layer for these sites lands
with U10b's sweep. The U10b roadmap row is annotated accordingly (rev 2's
"inherits nothing" claim was wrong).

## 7. Owner decisions (2026-08-21; latest-in-day wins)

1. **Full position lattice ×10, no scale knob** — re-ratified over the
   corrected (harsher) table.
2. **Recovery contested only by actors attacking the recoverer; free stand
   otherwise. Manual `stand` stays the paid uncontested exit.**
3. **`ConcentrationFloor: 0.02`**, own knob — governing the throttle
   contest too.
4. **Progression success-only** (U10b's success half from birth; bonus
   layer stays U10b's).
5. **Throttle interrupt = opposed roll through the concentration seam.**
6. **Surprise attack split out as U10d.**
7. **Knockdown anchors to the INTENDED 50/60/35 as a named rebalance** —
   the accidental live rates (50/~91/~2.3) are a latent config bug, not a
   baseline to preserve.
8. **Adversarial playtest gate ships in the slice.**

## 8. Done when

Each criterion ships as a test (U6b lesson):

1. `CalcConcentrationChance` does not exist; no production site computes a
   concentration percentage outside `RunConcentrationContest`.
2. `combat.RunConcentrationContest` is the only concentration resolver,
   both guards name it, and a source-scan test (Go files only) asserts it
   is the only reader of `ConcentrationFloor`.
3. The knockdown, recovery, and throttle rolls are seam sites;
   `dice.RollStat(50` appears at neither converted site and `util.Rand` no
   longer decides the throttle interrupt.
4. The four dead knobs are gone from Go and (where present) yaml; the five
   new knobs exist with shipped values. (Asserted against the HEAD blob via
   `git show`, never the skip-worktree disk copy.)
5. A sub-threshold hit on a folding caster produces no concentration roll;
   a supra-threshold hit produces exactly one.
6. Parity calibration: knockdown reproduces 50/60/35 within
   `p + F(1−2p)` ± 2 points (statistical, 200k/cell).
7. Progression: exactly one event on a WON contest and zero extra on a
   lost one, for concentration (all three triggers), knockdown defence
   (over the seam's existing per-contest award), and recovery — asserted
   by `GetSkillUseCount` deltas, not prose.
8. The adversarial playtest ran against the finished slice and its
   findings were dispositioned before handoff.
