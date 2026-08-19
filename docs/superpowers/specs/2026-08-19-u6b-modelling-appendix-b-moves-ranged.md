# U6b modelling gate — fragment B: special moves ×5, new crit tier, ranged, defence-set width

Spec: `docs/superpowers/specs/2026-08-19-u6b-finish-the-flip-design.md` §7 items 3–6.
Script: `tools/balance/u6b_model_moves_ranged.py` (self-contained; Monte Carlo,
numpy, seed 42, **2,000,000 samples per cell**). Every number below is from a
run of that script on 2026-08-19, branch `feature/u9-progression-layer`.

**Model fidelity** (mirrored from code, not prose): one attack roll
Normal(A, s), s = A×0.15 (`dice.StdDevFor`, floor 1.0); every defence rolled at
the **attacker's** s and contesting the **same** attack roll (correlation
modelled exactly by joint sampling); best defence = smallest attack-positive
margin (`contest.Run`); `RunWithFloors` flips the outcome with p = ContestFloor
0.125 and stamps a ±1 sentinel (floored wins cannot margin-crit; floored losses
always deal 50%); crit = margin/(s√2) ≥ bar plus the 1%-of-wins `ApplyCritFloor`
promotion both sides; defence multiplier per `defenceDamageMultiplier`
(1.0 win / 0.5 floored / clamp(0.5 − 0.25·z_def) rolled / 0.0 def-crit);
crit damage per `CritOrMitigatedDamage`: bypasses mitigation, ×(2.0 + 0.05·rank).

**Assumptions declared**: player gear parryrating 5, shield blockrating 15
(median of shipped items); mitigation 0% for trash mobs, 25% for player
defenders; all tier stats equal at the tier value. Meirok from `users/3.yaml`:
Str 115, Dex ~110 effective, Per 101, weapon-combat 69, unarmed 57,
**ranged-combat 1** (as saved). Mobs: stats 90, `GetCombatSkillLevel()` = 1
(fallback), but raw `GetSkillLevel(unarmed)` = **0** — see finding B3.

## Sanity anchors (all pass)

- Parity single-defence P(win) pre-floor = 50.0% (want 50%).
- Parity unconditional margin-crit at bar 2.0 = 2.3% (want ~2.3%).
- ContestFloor 0.125 clamps every final hit rate to **[12.5%, 87.5%]** — this
  band dominates every lopsided cell below.

## Roadmap anchor (item 3 requirement)

Weapon-combat-30 player vs mob: `130 vs 101` → `250 vs 105`.

| | score | P(hit) post-floor | pre-floor | P(crit per attempt) |
|---|---|---|---|---|
| BEFORE | 130 vs 101 | **76.5%** | 85.3% | — (no crit exists) |
| AFTER | 250 vs 105 | **87.2%** (ceiling) | 99.7% | **67.4%** const-2.0 / **78.1%** dyn bar (pins at 1.5) |

## Item 3 + 4 — special moves (bash-shape), player attacker, compounded

E[dmg] in units of raw pre-mitigation damage. Ratio = AFTER/BEFORE with the ×5
flip AND the crit tier landing together, as they do.

| tier | opponent | A_before vs D | A_after vs D(best) | P(hit) B→A | P(crit) c2.0 | P(crit) dyn (bar) | E[dmg] B | E[dmg] A (c2.0/dyn) | ratio (c2.0/dyn) |
|---|---|---|---|---|---|---|---|---|---|
| novice | trash mob | 105 vs 91 | 125 vs 85.5 | 67.7% → 82.4% | 27.2% | 33.6% (1.80) | 0.803 | 1.246 / 1.325 | **1.55× / 1.65×** |
| novice | same-tier player | 105 vs 105 | 125 vs 147.0 | 50.0% → 22.9% | 0.3% | 0.3% (2.00) | 0.498 | 0.301 / 0.301 | **0.60×** |
| journeyman | trash mob | 145 vs 91 | 245 vs 85.5 | 84.6% → 87.4% | 75.1% | 82.5% (1.50) | 0.919 | 2.628 / 2.793 | **2.86× / 3.04×** |
| journeyman | same-tier player | 145 vs 145 | 245 vs 273.0 | 50.0% → 27.0% | 0.3% | 0.3% (2.00) | 0.497 | 0.346 / 0.345 | **0.70×** |
| adept | trash mob | 175 vs 91 | 335 vs 85.5 | 86.7% → 87.5% | 81.8% | 85.6% (1.50) | 0.932 | 3.393 / 3.505 | **3.64× / 3.76×** |
| adept | same-tier player | 175 vs 175 | 335 vs 367.5 | 50.0% → 28.2% | 0.4% | 0.4% (2.00) | 0.497 | 0.360 / 0.360 | **0.72×** |
| meirok | trash mob | 184 vs 91 | 460 vs 85.5 | 86.9% → 87.5% | 84.6% | 86.6% (1.50) | 0.934 | 4.702 / 4.793 | **5.04× / 5.13×** |
| meirok | same-tier player | 184 vs 179 | 460 vs 496.1 | 53.9% → 32.2% | 0.5% | 0.5% (2.00) | 0.522 | 0.402 / 0.402 | **0.77×** |

Abstract single-defence sweep (D = r×A; scale-invariant in A at fixed RollSpread):

| r | P(hit) post-floor | P(crit@2.0) | E[def mult] |
|---|---|---|---|
| 0.5 | 86.8% | 56.3% | 0.933 |
| 0.8 | 74.6% | 13.3% | 0.852 |
| 1.0 | 50.0% | 2.5% | 0.663 |
| 1.2 | 25.5% | 0.4% | 0.415 |
| 1.5 | 13.2% | 0.1% | 0.181 |
| 2.0 | 12.5% | 0.1% | 0.125 |

### Finding B1 — the asymmetry, quantified

The hit-rate delta vs mobs is small (already 85–87%, ceiling-clamped); **the
crit tier is where the entire PvE buff lives**. From journeyman up, 75–85% of
special-move *attempts* vs a trash mob crit (≈95%+ of hits), each bypassing
mitigation at ×(2.0 + 0.05·rank) — Meirok's rank 69 gives ×5.45. Compounded:
special moves vs trash mobs deal **2.9× (journeyman) to 5.0× (Meirok)** today's
expected damage.

Against players the SAME change is a **nerf to the attacker**: hit rate falls
from ~50% to 23–32% (defence sets grow faster than attack — three entries, ×5
skill, block at 1.05 effectiveness + ratings) and expected damage lands at
**0.60–0.77×** today's. So no, ×5 + crit does not make special moves strictly
dominate: every point of the buff concentrates in player-vs-mob, which is
exactly the asymmetry §6 of the spec predicts, and it is very large.

### Against-the-spec note (B1a)

The PvE crit rates are not *new* behaviour — they match what melee autoattack
already does vs skill-1 mobs under margin crit (melee's dynamic bar is even
softer). This is convergence, not a novel spike. But: a bash also carries
knockdown + status, and at rank 69 a crit bash = 0.5 (BashDamagePercent) ×
5.45 = **2.7 full-swing equivalents plus a knockdown**, gated only by a 4-
stamina cost and the special-move cooldown. Post-U6b the special-move button is
unambiguously the biggest single hit in the PvE kit. Whether cooldown pacing
carries that is a tuning/playtest question, not a modelling one — flagged.

## Item 4b — beast moves, MOB attacker vs player tiers

| defender tier | D before (scalar) | D after (best of 3) | P(hit) B→A | P(crit) c2.0 | P(crit) dyn (bar) | E[dmg] ratio A/B |
|---|---|---|---|---|---|---|
| novice | 105 | 147.0 | 28.7% → **12.5%** | 0.1% | 0.1% (2.20) | **0.31×** |
| journeyman | 145 | 273.0 | 12.6% → 12.5% | 0.1% | 0.1% (3.20) | 0.86× |
| adept | 175 | 367.5 | 12.5% → 12.5% | 0.1% | 0.1% (3.95) | 1.01× |
| meirok | 179 | 496.1 | 12.4% → 12.5% | 0.1% | 0.1% (5.40) | 1.02× |

### Finding B2 — mobs do NOT "gain the tier simultaneously"

**Code contradiction with the task framing**: skill-move callers pass
`char.GetSkillLevel(skills.UnarmedCombat)` (`combat_pounce.go`), and
`GetSkillLevel` (`characters/skills.go:166`) returns **0** for a mob with no
skills block — the combat-skill-**1** fallback lives only in
`GetCombatSkillLevel`, which these callers do not use for the attack side. So a
mob's special-move attack score is 90 both before AND after (×5 of zero is
zero), while the player defender's set quintuples. Result: beast-move hit rates
pin to the 12.5% ContestFloor against everyone above fresh-novice, a novice
takes **0.31×** today's pounce damage, and the "new crit tier" fires for mobs
at ~0.1% per attempt (the 1% floor times their win rate) under either bar. The
hoisted dynamic bar is doubly irrelevant for mob attackers — it *rises* to
2.2–5.4 against skilled players. Beast moves become floor-only chip damage;
if that is not intended, U6b needs a mob-side skill answer (e.g. seeding mob
skills, or scoring mob attacks off `GetCombatSkillLevel`), and that decision
belongs in the spec, not in the implementation.

## Item 5 — ranged: flat shield bonus → real block

Same-tier attacker (Per + ranged; ×1 before, ×5 after) vs same-tier defender.
Meirok rows use his real ranged-combat 1, so his attack is 102→106 and his own
defence dominates (ceiling); read novice–adept for the trend.

| tier | shield? | D before | D after (best) | P(defended) B→A | E[dmg] B→A | shield worth |
|---|---|---|---|---|---|---|
| novice | no | 105 | 118.8 | 50.0% → 43.0% | 0.498 → 0.600 | — |
| novice | yes | 120 | 147.0 | 68.8% → 74.5% | 0.363 → 0.322 | AFTER: **−46.3%** dmg vs no-shield (BEFORE: −27%) |
| journeyman | no | 145 | 232.8 | 50.0% → 43.0% | 0.498 → 0.639 | — |
| journeyman | yes | 160 | 273.0 | 64.0% → 69.3% | 0.400 → 0.376 | AFTER: **−41.2%** (BEFORE: −20%) |
| adept | no | 175 | 318.2 | 50.0% → 43.0% | 0.498 → 0.669 | — |
| adept | yes | 190 | 367.5 | 61.7% → 67.9% | 0.418 → 0.394 | AFTER: **−41.2%** (BEFORE: −16%) |
| meirok | no | 179 | 375.2 | 87.5% → 87.5% | 0.098 → 0.095 | — |
| meirok | yes | 194 | 496.1 | 87.5% → 87.5% | 0.094 → 0.095 | ~0% (both at ceiling) |

### Finding B3 — the shield gets BETTER, the shieldless get worse

A shield is worth strictly more after the flip: **−41% to −46%** of incoming
ranged expected damage (vs −16% to −27% from the flat 15 today), and the gap
widens with skill because a flat 15 shrinks relative to growing scores while
block's ×5 skill term does not. The spec's direction (delete the flat bonus,
gate block on equipment) is supported. The cost: a **shieldless** defender vs a
same-tier shooter takes **+20% to +34% more** expected damage than today
(0.498 → 0.600–0.669), because lone dodge at 0.95 effectiveness sits below the
attacker's score at parity and the shooter now crits. Combined with cross-room
shots having no counter (§4.4.4), kiting shooters vs unshielded targets is the
matchup to watch in the playtest.

## Item 6 — pure defence-set width (same capability, widths 1/2/3)

Equal scores D = r×A per entry, no effectiveness, all contesting one attack roll.

| r | width | P(defended) | E[def mult] | dmg vs width-1 |
|---|---|---|---|---|
| 0.8 | 1 | 25.4% | 0.852 | — |
| 0.8 | 2 | 33.2% | 0.799 | −6.2% |
| 0.8 | 3 | 38.4% | 0.762 | −10.6% |
| 1.0 | 1 | 50.0% | 0.663 | — |
| 1.0 | 2 | 62.4% | 0.559 | **−15.7%** |
| 1.0 | 3 | 68.7% | 0.500 | **−24.5%** |
| 1.2 | 1 | 74.5% | 0.416 | — |
| 1.2 | 2 | 82.2% | 0.316 | −24.0% |
| 1.2 | 3 | 84.6% | 0.273 | **−34.2%** |
| 1.5 | 3 | 87.5% | 0.138 | −23.4% |
| 2.0 | 3 | 87.5% | 0.125 | ~0% (floor-pinned) |

(0.5 rows omitted: width worth <1% when the attacker dominates.)

### Finding B4 — width is worth about a quarter of a swing at parity

Handing a defender best-of-3 instead of one scalar is **+18.7pp P(defended)
and −24.5% expected damage at parity**, peaking at −34% when the defender is
modestly ahead (r≈1.2), and near-zero at the extremes where ContestFloor
decides everything. This is the standalone "wider net" bonus the synthesis
should subtract before attributing gains to the ×5 weight: in the item-3 PvP
rows, roughly a third of the defender's improvement is width, the rest is
weight + effectiveness + gear ratings.

## Cross-cutting observations

- **ContestFloor 0.125 is the third actor in every table.** Post-flip, every
  matchup past roughly ±1.2 score ratio is floor-dominated: PvE hit rates
  saturate at 87.5% (so ×5 buys crit margin, not accuracy), and mobs keep a
  12.5% chip chance with a guaranteed-50%-damage floored save forever. The
  adept/Meirok beast-move rows are ~1.0× ratio *only because they were already
  pure floor before*.
- **The dynamic-bar hoisting (§4.4.3) matters mainly at low skill vs mobs**
  (novice 27→34%, journeyman 75→82% crit); at parity both bars read 2.0 and
  for mob attackers the margin, not the bar, forbids crits either way. Model
  shows no balance blocker to hoisting; its real payoff is buff consistency
  (Accuracy/Blink), not rates.
- ExecuteSkillMove's BEFORE already applies the partial-damage defence curve to
  its scalar defence, so "before" is not binary miss — the model reflects that.
- Not modelled (out of this fragment's scope): resource-depletion multipliers
  (none exist on these channels today, §2.3), min-damage-1 clamp (negligible),
  mutation dodge modifiers, Accuracy/Blink buffs, defence-cost skill-stripping
  when exhausted (U8) — all attempts assume the defender can pay.
