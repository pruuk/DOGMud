# U6b modelling gate — Group A: spell + taunt hit-gate collapse (§7 items 1–2)

**Status: DONE_WITH_CONCERNS** (three results argue against, or force decisions on, the spec).

Script: `C:\Users\Calabe Davis\workspace\DOGMud\tools\balance\u6b_model_spell_taunt.py`
(branch `feature/u9-progression-layer`; run with `python tools/balance/u6b_model_spell_taunt.py`).
Method: **deterministic numeric integration** (midpoint rule over the attacker's
self-relative z, STEPS=8000 over [-8,8], closed-form normal conditionals for the
defender roll and the partial-damage expectation). No Monte Carlo. Every number
below is script output.

Sanity anchors (script asserts, printed): parity raw P(win)=0.5000; rolled crit
= 2.28% of rolls at parity (target 2.275%, i.e. ~4.55% of contested wins);
floored parity P(success)=0.5000.

Shipped values verified against `git show HEAD:_datafiles/config.yaml` (not
disk): RollSpread 0.15, SkillWeight 5.0, SpellAttackSkillFactor 3, ContestFloor
0.125, Quell/DefyEffectiveness 1.0, MinAttackCritChance 0.01,
MinDefenseCritChance 0.01, ConvictionPenaltyMax 0.28, ResourcePenaltyCurve 2.0,
CritDamageBase 2.0, CritDamagePerSkill 0.05. ContestCritThreshold 2.0 is a Go
const. Meirok from `users/3.yaml`: Wil 148 (113+35), Cha 123 (93+30),
**spellcasting 52** (prompt said ~51; the save says 52 — save wins), rhetoric 55.

## Code semantics the model mirrors (where code contradicted prompt/spec, code won)

1. **Fumble is checked FIRST** in both `spell_resolution.go` and
   `combat_taunt.go`: `AttackRoll.ZScore <= -2.0` aborts even a *winning*
   contest (backfire / CP self-damage). Self-relative, so a flat ~2.28% per
   attempt, and it caps every hit rate at **85.5%**, not the 87.5% the floor
   alone would give.
2. **`RunWithFloors` flips the outcome** with p=0.125 (one draw) and stamps a
   ±1 sentinel margin. A floored win cannot be a *rolled* crit — but the
   spell/taunt call sites have **no `Floored` guard around
   `AttackContestCrit`** (melee's `applyCritFloors` does have one), so a
   floored win can still be floor-PROMOTED to a crit at 1%. Modelled as coded.
   This is a live melee-vs-channel divergence the spec's "one crit path" should
   name.
3. **Quell/defy damage multiplier** (`defenceDamageMultiplier`): attack win
   1.0; floored save exactly 0.5; defensive crit (normalized defence margin ≥
   2.0, plus 1% promotion, never on floored outcomes) 0.0; rolled defensive win
   `0.5 − 0.25×dm`, dm∈[0,2).
4. **Taunt's gate already contests Wil + rhetoric×5** — for taunt the collapse
   does NOT change the hit contest at all (defy is the same score ×1.0). The
   spec's §6 row "spell and taunt crits now face their defence — cut to crit
   reliability" is **true only for spell**; taunt's crit rate is bit-identical
   before and after (see tables). The defy contest also omits the
   conviction-depletion multiplier the gate applies (code omission the spec
   notes; the collapse keeps convMult per §4.1, which at full CP changes
   nothing and at low CP makes the single contest slightly *harder* than
   today's defy leg was).
5. `CalcSpellAttack` = `stat + round(skill×5)×3` (the round is modelled).
   `ChannelAttackScore` hardcodes Willpower+spellcasting for the quell leg —
   modelled with Wil-casters so the scores coincide; the §4.1 primarystat
   decision does not change these numbers for Wil-primary spells.
6. **Crit damage rank divergence**: the spell path passes the RAW spellcasting
   rank to `CritDamageMultiplier` (Meirok crit = ×4.6) but the taunt path
   passes the ×5-WEIGHTED rhetoric (`int(attackerRhetoric)` — Meirok crit =
   **×15.75**). Pre-existing, not created by U6b, but it decides how much a
   crit-rate change is worth per channel (see damage-weighted table).

Definitions: P(hit) = full-damage success per attempt (BEFORE spell: gate
success; taunt/AFTER: contest success). E[mult] = expected damage multiplier
per attempt (fumble and fizzle = 0; AFTER includes the new 0–50% partial damage
on defensive wins; a crit counts as 1.0 — see the damage-weighted table for the
corrected view). "Mirror" = a same-tier caster/defender. Trash mob = stats 90,
all skills 1. Ratio rows apply defender = r × attacker in every contest
(outcomes depend only on d/a because σ = 0.15 × attacker score).

## 1. Spell — real defenders

| Attacker | BEFORE hit / crit / defcrit / E[mult] | AFTER hit / crit / defcrit / E[mult] | after/before (hit, crit, E[mult]) |
|---|---|---|---|
| novice vs trash | 85.0% / 54.0% / 0.1% / 0.811 | 77.1% / 17.4% / 0.1% / 0.863 | 0.91x, 0.32x, 1.06x |
| journeyman vs trash | 85.5% / 83.6% / 0.0% / 0.854 | 85.5% / 70.9% / 0.0% / 0.916 | 1.00x, 0.85x, 1.07x |
| adept vs trash | 85.5% / 84.6% / 0.0% / 0.855 | 85.5% / 79.5% / 0.0% / 0.916 | 1.00x, 0.94x, 1.07x |
| Meirok vs trash | 85.5% / 84.9% / 0.0% / 0.855 | 85.5% / 82.0% / 0.0% / 0.916 | 1.00x, 0.97x, 1.07x |
| novice vs mirror | 84.4% / 44.8% / 1.0% / 0.711 | 49.7% / 2.5% / 1.7% / 0.658 | 0.59x, 0.06x, 0.93x |
| journeyman vs mirror | 85.5% / 81.6% / 0.1% / 0.842 | 49.7% / 2.5% / 1.7% / 0.658 | 0.58x, 0.03x, 0.78x |
| adept vs mirror | 85.5% / 83.5% / 0.0% / 0.848 | 49.7% / 2.5% / 1.7% / 0.658 | 0.58x, 0.03x, 0.78x |
| Meirok vs mirror | 85.5% / 84.0% / 0.0% / 0.850 | 49.7% / 2.5% / 1.7% / 0.658 | 0.58x, 0.03x, 0.77x |

Vs plain non-caster (Wil 100, spellcasting 0): novice 84.4%→74.0% hit,
44.8%→13.3% crit; journeyman and up stay pinned at the 85.5% ceiling with crit
68.6–81.5% and E[mult] ×1.07.

**Headline:** a Meirok-tier caster vs a **parity defender** goes from
**49.7% hit / 2.5% crit** to **49.7% / 2.5%** — per-contest parity is
unchanged by construction. The number that actually moves is the **mirror
match**: today Meirok's gate (score 928, ×15 skill) contests the other
caster's raw Wil 148 (ratio 0.16), so Meirok hits **85.5% and crits 84.0% of
casts**; after the collapse the same matchup is **49.7% hit / 2.5% crit**. The
"how do I ever miss?" state and the near-total crit saturation of today's
caster-vs-caster spells is the thing being deleted. Against trash mobs the
collapse is nearly invisible (hit unchanged at the floor ceiling, E[mult]
+7% from the new partial damage; crit −3% at Meirok, −68% at novice).

**Today's crit saturation also explains the "unobserved quell lane"**: since
crit skips quell and veteran gate crits run 82–85%, quell currently runs on
almost no veteran cast. After the collapse quell decides every cast.

## 2. Spell — ratio sweep (per-contest, tier-independent)

| d/a | BEFORE hit / crit / defcrit / E[mult] | AFTER hit / crit / defcrit / E[mult] | E[mult] after/before |
|---|---|---|---|
| 0.5 | 85.1% / 56.2% / 0.0% / 0.832 | 85.1% / 56.2% / 0.0% / 0.914 | 1.10x |
| 0.8 | 74.0% / 13.3% / 0.2% / 0.650 | 74.0% / 13.3% / 0.2% / 0.842 | 1.30x |
| 1.0 | 49.7% / 2.5% / 1.1% / 0.338 | 49.7% / 2.5% / 1.7% / 0.658 | 1.95x |
| 1.2 | 25.2% / 0.4% / 3.3% / 0.107 | 25.2% / 0.4% / 11.7% / 0.412 | 3.86x |
| 1.5 | 12.9% / 0.1% / 7.2% / 0.024 | 12.9% / 0.1% / 54.3% / 0.178 | 7.29x |
| 2.0 | 12.2% / 0.1% / 10.5% / 0.016 | 12.2% / 0.1% / 85.2% / 0.122 | 7.48x |

At equal per-contest ratio the collapse never lowers expected damage — the
partial-damage-on-defence-win semantics roughly double E[mult] at parity and
carry a floor-bound underdog from 1.6% to 12.2% of a hit per cast. What the
collapse changes is *which ratio you are at*: the gate ratio (raw Wil vs ×15)
becomes the quell ratio (skill×5 both sides), which is what produces the
mirror-row collapse in table 1. Note the defensive-crit column at high ratios:
under §4.4 every one of those is counter fuel (85% of non-floored losses at
2.0×).

## 3. Crossover — the spec's key worry, quantified

Minimal defender spellcasting rank (defender Wil = 100) where AFTER P(hit)
drops below 50%:

| Attacker | AFTER attack score | crossover sc rank | score-parity rank | BEFORE equivalent |
|---|---|---|---|---|
| novice | 125 | **5** | 5.0 | gate needed defender raw Wil 175 |
| journeyman | 245 | **29** | 29.0 | raw Wil 495 |
| adept | 335 | **47** | 47.0 | raw Wil 735 |
| Meirok | 408 | **62** | 61.6 | raw Wil 928 |

The crossover sits essentially AT score parity (fumble-first and the symmetric
floor cancel to within a rank). Reading: the attacker's lost ×10 skill weight
and the defender's gained ×5 exactly telescope into "spell accuracy becomes an
ordinary opposed contest". A same-rank caster defender is a coin flip at every
tier; a defender ~10 ranks ahead pushes a caster to ~25% full hits (though
still 0.41 E[mult] via partials). BEFORE, no realistic character could ever
reach the 50% line (raw Wil 928).

## 4. Taunt — full CP and 50% CP

Taunt hit and crit are **identical before and after** (same contest scores);
the entire net is damage-side, from removing the second (defy) contest and
adding partial damage:

| Scenario (full CP) | BEFORE hit / crit / E[mult] | AFTER hit / crit / E[mult] | E[mult] ratio |
|---|---|---|---|
| novice vs trash | 77.1% / 17.4% / 0.697 | 77.1% / 17.4% / 0.863 | 1.24x |
| Meirok vs trash | 85.5% / 81.7% / 0.853 | 85.5% / 81.7% / 0.916 | 1.07x |
| tier vs mirror (parity) | 49.7% / 2.5% / 0.338 | 49.7% / 2.5% / 0.658 | 1.95x |
| Meirok vs Meirok (Wil 148 + rhet 55×5 = 423 def) | 41.0% / 1.3% / 0.245 | 41.0% / 1.3% / 0.581 | 2.37x |

At 50% CP (convMult 0.930): parity-mirror 39.3% hit both models, E[mult]
0.265 → 0.565 (2.14x); Meirok mirror 31.0% hit, 0.184 → 0.481 (2.61x).

**What the double contest costs the taunter today** (E[mult] on a non-crit
hit = the defy leg alone): 0.933 at defy ratio 0.5, 0.852 at 0.8, **0.663 at
parity**, 0.416 at 1.2, 0.181 at 1.5, 0.125 at 2.0. Removing it is worth
+50–100% expected damage against peers and up to ~8x against stronger
defenders (where BEFORE the defender won both contests almost every time).

## 5. Damage-weighted net (crits at their real value)

E[mult] counts a crit as 1.0; in code a crit bypasses mitigation and scales by
`CritDamageMultiplier`. Units: one raw unmitigated hit per attempt.

Target mitigation 0% / 30%:

| Attacker | SPELL vs trash | SPELL vs mirror | TAUNT vs trash | TAUNT vs mirror |
|---|---|---|---|---|
| novice | 1.49→1.08 (0.73x) / 0.62x | 1.27→0.69 (**0.54x**) / 0.42x | 1.09→1.26 (1.15x) | 0.39→0.71 (1.81x) |
| journeyman | 2.73→2.51 (0.92x) / 0.90x | 2.68→0.71 (**0.27x**) / 0.20x | 5.99→6.06 (1.01x) | 0.52→0.84 (1.62x) |
| adept | 3.39→3.30 (0.97x) / 0.96x | 3.35→0.73 (**0.22x**) / 0.16x | 9.60→9.66 (1.01x) | 0.61→0.93 (1.53x) |
| Meirok | 3.91→3.87 (0.99x) / 0.98x | 3.88→0.75 (**0.19x**) / 0.14x | 12.91→12.97 (1.00x) | 0.44→0.78 (1.76x) |

So in real damage terms: **spell PvE is flat (0.92–0.99x)** — the crit-rate
drop and the partial-damage gain cancel almost exactly against mobs;
**spell caster-vs-caster loses 73–86%** of expected damage (a deliberate
deletion of the crit-saturated state, but the magnitude should be a conscious
owner decision); **taunt gains everywhere** (1.0–1.15x PvE, 1.5–1.8x vs
peers). Note the taunt "vs trash" absolute values (Meirok ~12.9 raw hits per
taunt): that is today's behaviour, driven by ~82% crit × the ×15.75 weighted-
rank crit multiplier — the finding-6 rank divergence, pre-existing.

## 6. PvM reverse direction — mob casters/howlers vs players

| Scenario | BEFORE hit / defcrit / E[mult] | AFTER hit / defcrit / E[mult] |
|---|---|---|
| mob spell vs novice | 56.3% / 14.3% / 0.194 | 17.3% / 25.3% / 0.294 |
| mob spell vs journeyman | 31.0% / 26.6% / 0.044 | 12.2% / 85.5% / 0.122 |
| mob spell vs Meirok | 14.2% / 12.3% / 0.019 | 12.2% / 85.5% / 0.122 |
| mob howl vs Meirok | 12.2% / 10.6% / 0.016 | 12.2% / 85.5% / 0.122 |

**A trash caster's full-hit rate vs a novice player falls 56.3% → 17.3%**, and
vs journeyman-and-up it is pinned to the 12.2% floor with **85.5% of casts
ending in a defensive crit** — which under §4.4 grants the player a melee
counter. Mob casters go from a real threat to novices to counter-attack
batteries. Any mob that is *supposed* to land spells needs either real
spellcasting ranks or a tuned QuellEffectiveness; "all mobs have combat
skill 1" makes the ×15→×5 cut land hardest on the mobs' own casters.

## Results that argue AGAINST (or force decisions on) the spec

1. **The spec's stated worry (item 1) is confirmed but mislocated.** Hit rate
   vs mobs and non-casters barely moves (journeyman+ stay at the 85.5%
   ceiling). The real collapse is caster-vs-caster: 85.5%/84% hit/crit → a
   coin flip with 2.5% crit, and 0.19x damage for Meirok. If caster-vs-caster
   spells feeling "ordinary" is intended, fine — but it is a −81% damage swing
   at veteran level and should be owner-signed, not implied.
2. **The "cut to crit reliability" for taunt is false**: taunt's gate already
   contested the full defended score, so its crit rate is bit-identical
   before/after. Taunt's net is a pure attacker buff (1.5–1.9x vs peers). The
   §6 behaviour table should say so.
3. **Mob casters are gutted** (finding 6 above). The spec's §6 note that
   ×1→×5 changes "widen the player-versus-mob gap" undersells the reverse
   direction for THIS slice: the ×15→×5 cut plus quell-with-skill makes trash
   casters unable to threaten even novices, and turns 85% of their casts into
   player counter fuel once §4.4 lands. Needs a mob-side compensator decision.
4. **Two crit-floor asymmetries survive the collapse unless named**: (a)
   spell/taunt sites lack melee's `Floored` guard, so floored wins can be
   floor-promoted to crits at 1%; (b) the spell path feeds
   `CritDamageMultiplier` the raw rank while taunt feeds the ×5-weighted rank
   (Meirok ×4.6 vs ×15.75). Both are "one crit path" violations U6b's §4.4
   should either fix or allowlist.
5. **The partial-damage semantics are load-bearing.** All the "collapse is
   damage-neutral-or-positive" results depend on the AFTER shape granting
   0–50% damage on a rolled defensive win (the fizzle → ordinary-defence
   change). If implementation kept fizzle at zero damage, parity E[mult] drops
   from 0.658 to ~0.53 and every after/before damage ratio falls accordingly.

## Five most decision-relevant numbers

1. Meirok caster vs same-tier caster: **85.5% hit / 84.0% crit → 49.7% / 2.5%**
   (hit 0.58x, crit 0.03x; damage-weighted **0.19x**).
2. Spell PvE (Meirok vs trash): hit unchanged 85.5%, crit 84.9%→82.0%,
   damage-weighted **0.99x** — the collapse is nearly invisible against mobs.
3. AFTER hit drops below 50% at defender spellcasting ≈ score parity:
   ranks **5 / 29 / 47 / 62** for novice/journeyman/adept/Meirok (defender
   Wil 100); BEFORE needed impossible raw Wil 175/495/735/928.
4. Taunt at parity: hit/crit unchanged, **E[mult] 0.338 → 0.658 (1.95x)**;
   the deleted defy leg was costing a non-crit hit 34% of its damage at parity.
5. Trash mob casting at any journeyman+ player AFTER: **12.2% full hits,
   85.5% defensive crits** (each a §4.4 counter) — mob casters need a
   compensator or they become counter batteries.
