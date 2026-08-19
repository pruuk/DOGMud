# U6b Modelling Gate — Results and Decisions

**Created:** 2026-08-19
**Discharges:** §7 of [`2026-08-19-u6b-finish-the-flip-design.md`](2026-08-19-u6b-finish-the-flip-design.md)
(the mandatory pre-code modelling gate), including the pre-U6 special-move gate
the roadmap raised and U6 shipped without.

**Method.** Three independent modelling groups, run in parallel, each mirroring
the resolution code directly (not prose summaries) and computing every reported
number by script. Where any prompt or spec claim contradicted the code, the
code won and the contradiction is recorded. All three reproduce the arc's
sanity anchors: parity P(win) = 50.0% pre-floor, parity crit = 2.28% of rolls
at threshold 2.0, and the roadmap's own `130 vs 101` special-move example.

| Group | Items | Script | Full tables |
|---|---|---|---|
| A | spell + taunt collapse (1–2) | `tools/balance/u6b_model_spell_taunt.py` (numeric integration, STEPS=8000) | [appendix A](2026-08-19-u6b-modelling-appendix-a-spell-taunt.md) |
| B | ×5 flip, crit tier, ranged, width (3–6) | `tools/balance/u6b_model_moves_ranged.py` (Monte Carlo, seed 42, 2M/cell) | [appendix B](2026-08-19-u6b-modelling-appendix-b-moves-ranged.md) |
| C | counters, unowned family, cost load (7–9) | `tools/balance/u6b_model_counters_family_costs.py` (integration, 6001-pt grid; MC cross-check within 0.06pp) | [appendix C](2026-08-19-u6b-modelling-appendix-c-counters-family-costs.md) |

Shared tiers across all groups: novice 100/5, journeyman 120/25, adept 135/40,
Meirok from the live save (`users/3.yaml`; note his saved base Dex is 98 with
~110 effective from gear, spellcasting 52, ranged-combat 1). Mobs: stats 90,
and see finding D1 — mob SKILL is the gate's biggest discovery.

---

## 1. Verdict

**The core unification survives the numbers.** The spec's biggest stated fear
(a large spell-accuracy cut) does not materialize where it matters, two of the
scariest-sounding changes are near no-ops, and the defence-cost fear dissolves
entirely. **But the gate found one blocking design decision (mob skill),
several factual errors in the spec, and a concentration of new power in
down-tier farming that needs either tuning or an explicit owner acceptance.**

Do not write the implementation plan until the §5 decisions are made.

---

## 2. Fears the modelling DISSOLVED

- **Spell PvE accuracy is fine.** Meirok vs trash keeps 85.5% hit before and
  after; damage-weighted output ratio **0.99×**. The attacker's lost ×15 term
  and the floor ceiling cancel against unskilled defenders. The spec's "large
  cut" row is wrong for PvE.
- **Defence cost load is negligible.** §4.3's new charges add only
  **0.3–0.6 stamina/round** at typical load — the physical defence base ships
  at **1.0**, not the 2.0 the spec's U7 recollection assumed. No tier
  approaches exhaustion or U8's skill-strip in a 30-round fight. (Caveat: one
  strip iteration, if ever reached, is brutal — defended rate 68% → 12.5%,
  +42% damage taken per swing — so this safety is a property of base cost 1.0
  and must be re-checked at any retune.)
- **Flee ×25 → ×5 changes almost nothing.** Both headline cells
  (novice-flees-veteran 12.5%, veteran-flees-trash 87.5%) are floor/ceiling
  pinned before AND after; only novice-vs-trash moves (−8.9pp). The change
  buys tunability and rule-1 compliance, not different outcomes.
- **The width effect is meaningful but not destabilising.** Best-of-3 versus
  one folded scalar: +18.7pp P(defended), −24.5% expected damage at parity,
  peaking −34% at ratio ≈1.2, ~0 at the floor-pinned extremes.
- **Ranged shields work better as a real defence.** Shield worth grows from
  −16…−27% expected damage (flat 15 bonus) to **−41…−46%** (contested block).
  Shieldless defenders take +20–34% more than today, which is the flat bonus's
  removal working as intended.

## 3. What actually changes, by the numbers

- **Caster-vs-caster duels are transformed.** Today they are crit-saturated:
  Meirok vs a same-tier caster is 85.5% hit with **84% of casts critting** —
  which is also why the quell lane has never been observed in play (crit skips
  quell, and at veteran level nearly everything crits). After: 49.7% hit, 2.5%
  crit, damage-weighted **0.19×**. Spell duels become contests instead of
  crit races. The 50%-hit crossover sits almost exactly at score parity
  (defender spellcasting 5/29/47/62 against novice/journeyman/adept/Meirok).
- **Taunt's collapse is a pure ATTACKER buff, not a defence buff.** Its gate
  already contested `Wil + rhetoric×5`, so hit and crit are **bit-identical**
  before and after. What changes: the defender is no longer contested twice,
  and the deleted second defy contest was costing non-crit hits 34% of their
  damage at parity — E[damage multiplier] rises 0.338 → 0.658. The spec's §6
  "cut to crit reliability" row is FALSE for taunt and stands corrected.
- **Specials vs mobs: 2.9× (journeyman) to 5.0× (Meirok) expected damage**,
  almost all of it the new crit tier (75–85% of attempts crit down-tier;
  Meirok's crit multiplier is ×5.45 and bypasses mitigation). Vs same-tier
  players the identical flip is a **nerf**: hit 50% → 23–32%, expected damage
  0.60–0.77×. This is the mob-skill asymmetry made concrete.
- **The floor dominates past ±1.2 score ratio.** Every final rate clamps to
  [12.5%, 87.5%], so beyond mild mismatch the ×5 flip buys crit MARGIN, not
  accuracy — exactly where the down-tier findings below come from.

## 4. The down-tier concentration (owner attention required)

Four independent results stack on the same play pattern — a strong character
farming weaker content:

1. **Counters become a second weapon down-tier.** Defensive crit is
   floor-capped at 87.5%, and every journeyman+ defender vs a trash attacker
   sits at **85.5% defensive crit per incoming swing** → ~32–42 free counter
   damage per round. Important nuance: this degeneracy is **live in melee
   today** (parry→riposte etc.); U6b extends it to every channel rather than
   creating it.
2. **Throw becomes the best down-tier AoE tool in the game**: journeyman
   expected damage per firebomb vs trash goes **12 → 165** (87% hit, 73%
   mitigation-bypassing crit at ×3.25) once it gains a defence set and the
   crit tier.
3. **Submission stun-farming**: crit (stun) vs trash rises from ~2% to
   **18–62%** when crit moves from self-relative z to contest margin.
4. **A crit bash at rank 69 is ~2.7 full-swing equivalents plus knockdown for
   4 stamina** — cooldown-gated, but the biggest single button in the PvE kit.

None of this is necessarily wrong — veterans dominating trash is the design —
but the crit tier concentrates the new power almost entirely in down-tier
play, and it compounds with gold-scaled loot NOT scaling from trash. Options
are in §5.3.

## 5. Decisions required before the implementation plan

### 5.1 BLOCKING: mobs have skill 0 almost everywhere (spec's premise was wrong)

The arc's standing line "every mob is combat skill 1" is true **only through
`GetCombatSkillLevel`'s fallback, which only the melee autoattack path uses**.
Verified: `GetSkillLevel` has no mob fallback, **zero** mob YAMLs author a
`skills:` block, and the special-move callers, `CalcSpellAttack`, and every
other site read `GetSkillLevel` directly. Consequences post-flip, measured:

- **Beast moves collapse**: mob attack score gains nothing from ×5 (5 × 0 = 0)
  while player defenders gain skill×5. Mob specials pin to the 12.5% floor vs
  journeyman+; a novice takes 0.31× today's pounce damage; mob crit ≈0.1%.
  Mobs do NOT "gain the tier simultaneously" as the spec assumed.
- **Mob casters are gutted**: a trash caster pins at the 12.2% floor vs
  journeyman+, with **85.5% of casts ending in defensive crits — each one
  counter fuel** under §4.3 of the spec.

Options (owner call):

| Option | Effect | Cost |
|---|---|---|
| (a) Seed mob skills as data, scaled to tier | Real knob per mob; boss casters stay dangerous | Touches 641 mob files or the spawn path; a balance pass of its own |
| (b) Route ALL mob skill reads through a fallback floor (1) | Minimal; preserves today's shape | ×5 of 1 = 5 points — cosmetic; PvE still eases everywhere players have skills |
| (c) Derive mob skill from statpool (gold-scaled instances included) | One formula; scales with the existing difficulty dial | New mechanism; needs its own mini-model |

The modelling used (b)-equivalent assumptions and still saw the collapse, so
**(b) alone does not preserve mob threat.** Recommendation: (c) for instance
mobs + (a) for authored bosses, but this is a design decision, not a modelling
conclusion.

### 5.2 Fizzle MUST become a partial-damage outcome (confirming spec §8.1)

Every damage-neutral-or-positive spell result above depends on the collapsed
contest granting the standard 0–50% partial damage on defensive wins. If
"fizzle" kept its zero-damage semantics, parity E[multiplier] falls from 0.658
to ~0.53 and the spell channel takes a real hidden nerf. The spec's open
question §8.1 is therefore not cosmetic: **fizzle becomes an ordinary
partial-damage defence outcome**, and only the WORD is a copy question.

### 5.3 The down-tier concentration (§4)

Options: accept as-is (veterans farming trash is intended; loot does not scale
there), damp the crit-rate ceiling (a config cap on P(crit) or a margin
saturation — a new knob), or revisit the dynamic bar's skill-shift direction
when hoisting it (§4.4.3 of the spec). The counter half is already live in
melee today, which argues for "accept and playtest"; the throw and submission
halves are new. Owner call.

### 5.4 Drift: two unacknowledged design changes

- The 5/5 reweight **deletes drift's aggressor edge entirely** (expected drift
  +0.196 steps/round → 0.000 at parity). If the aggressor is meant to keep an
  edge, it must return as an explicit config modifier, not as a skill-weight
  accident.
- Drift routes through `RunContest`, so **12.5% of drift rounds are
  floor-forced Holds** in every variant (the flipped sentinel margin lands in
  the Hold band). Pre-existing, unmentioned by the spec, unchanged by the fix;
  documented here so it is not rediscovered as a U6b regression.

The √2 fix itself is uncontroversial and modelled: it damps per-round drift
swing ~29% on its own; with the reweight the two compound.

### 5.5 Kiting is optimal against exactly the fights counters were built for

Cross-room immunity is worth ~0 down-tier but **36–100% of a shot's damage
against 1.5–2× targets** — the no-counter adjacent room is the best position
in boss fights. The owner already declined a ranged counter; this number is
recorded so that decision is re-affirmed (or not) with the cost known.
Recommendation: accept, and put "boss-kiting from the next room" on the
playtest checklist as a feel item.

## 6. Code defects the gate found (fold into U6b's fix list)

All verified in source by group A; all are "one crit path" violations:

1. **Spell/taunt crit call sites lack melee's `Floored` guard** — a
   floor-promoted win can be promoted again to a crit (~1% of casts). Melee
   guards this; the collapsed contest must too.
2. **The crit-damage multiplier is fed inconsistent rank inputs**: spell passes
   the raw spellcasting rank, taunt passes the ×5-weighted rhetoric (Meirok:
   ×4.6 vs ×15.75 crit damage). Unify on one definition in the shared seam.
3. **Fumble is checked before the success branch** and aborts even winning
   casts/taunts, capping every hit rate at 85.5%. Melee's per-swing fumble has
   the same self-relative shape, so this may be coherent — but it is
   undocumented and the cap surprised the modelling. Document or change,
   deliberately.
4. Taunt's defy leg omits the conviction-depletion multiplier its own gate
   applies (moot after the collapse deletes the leg; listed for completeness).
5. **Throw's attack score is `Dex + skullduggery×5`**, not skullduggery alone,
   and its defender-side ×2.5 is `SkillWeight × 0.5` — coupled to the knob.
   The spec's §2.2 row understates the attacker side.

## 7. Spec corrections this gate forces

- §6 table: taunt's "cut to crit reliability" row is false — replace with
  "taunt: pure attacker buff (double-contest removed), hit/crit unchanged".
- §6 table: "spell attacker large cut" applies to PvP/caster-duels only; PvE
  ≈ 0.99×.
- §2.2: mobs are skill **0** on every non-melee path, not 1 (see §5.1).
- §7 item 9's premise (defence base 2.0) was wrong: physical base ships 1.0;
  the fear it encoded is dissolved rather than confirmed.
- Meirok reference values: saved base Dex 98 (~110 effective via gear),
  spellcasting 52, ranged-combat 1 (so his own ranged rows are floor-bound —
  veteran ranged modelling used the adept tier instead).

---

**Gate status: DISCHARGED**, contingent on the §5 decisions. The
implementation plan may be written once §5.1 (mob skill) is decided; §5.2's
answer is effectively forced; §5.3–5.5 can be decided at plan review without
re-modelling.
