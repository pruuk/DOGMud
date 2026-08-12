# Unified Resolution Roadmap

**Created:** 2026-08-12
**Specs:** [`unified-contest-resolution-design`](../superpowers/specs/2026-08-12-unified-contest-resolution-design.md)
· [`unified-cost-and-harm-design`](../superpowers/specs/2026-08-12-unified-cost-and-harm-design.md)
**Model:** `tools/balance/unified_resolution_model.py`
**Foundation:** PR #30 (chunks 5.11g + 5.11e), merged as `63f21e8ee`

---

## Why

33 opposed-roll call sites across 18 files, plus melee, which uses none of them.
None share a resolution path, because `internal/dice` is a dice primitive rather
than a resolution seam. Every caller reassembles floors, hit, crit, fumble,
damage, cost and progression slightly differently.

Measured drift, skill weight on the **defending** side: melee ×5, ranged ×1,
spell ×0, taunt ×5. A spellcasting-30 caster crits a stat-matched defender 96.7%
of the time; melee at parity is 2.2%.

`runBestOfAllDefense` already *is* the target architecture. Nothing else can
reach it.

---

## Sequencing: refactor first, flip once

**Every plan up to U6 is a provable no-op.** The shared core is built
bug-compatible with today, reproducing current behaviour including its
inconsistencies, by taking them as **parameters** rather than as separate code
paths. Existing tests must pass unchanged at every step.

| Parameter | During migration | At the flip (U6) |
|---|---|---|
| `skillWeight` | per-channel 5 / 1 / 0 / 15 | uniform 5.0, **parameter deleted** |
| `defenceOutcome` | `boolean` — a win is a clean miss | `multiplier` — margin-scaled 50–100% |
| `defenceSet` | per-channel as today | designed sets 3 / 2 / 2 / 1 / 1 |

There is only ever **one** implementation. Each site's old path is deleted the
moment it migrates — no dual maintenance, no final sweep. The behaviour change
then lands as a single reviewable, revertable commit.

**Rejected:** building a parallel system for a big-bang cutover (two
implementations to keep in sync, all risk concentrated at an unrevertable
cutover), and migrating channel-by-channel with semantics changing as you go
(leaves channels on different semantics for most of the arc — the same
half-converted failure that produced this mess).

---

## Plans

| # | Plan | Size | Depends on | Behaviour change? |
|---|---|---|---|---|
| **U1** | Contest core, bug-compatible. Generalise `runBestOfAllDefense`; normalise the margin sign at the seam; melee migrated onto it. | L | — | **No** |
| **U2** | Spell channel onto the core (4 sites), preserving ×15 attack / ×0 defence as parameters. | M | U1 | **No** |
| **U3** | Ranged + taunt onto the core, preserving ×1 and the flat shield bonus. | M | U1 | **No** |
| **U4** | Non-harm contests onto the core: sneak, steal, plant, defuse, flee (contest + progression layers only, no harm layer). | M | U1 | **No** |
| **U5** | Cost + harm helpers. One cost helper, one harm helper. Config-ify the hardcoded 2/4/5 defence costs. No pool may go negative. | M | U1 | **No** (costs unchanged, only routed) |
| **U6** | **THE FLIP.** Uniform ×5, multiplier defence, margin-scaled mitigation, designed defence sets, `avoidance.go` absorbed, tuning package applied. **All legacy parameters deleted.** | L | U2–U5 | **Yes — all of it** |
| **U7** | New cost surface: ranged, taunt and spell/taunt resistance start costing; skill-less roll on insufficient resource; inverse-skill cost band. | M | U5, U6 | **Yes** |
| **U8** | Progression layer: events not side effects, both sides, doing vs observing, skill **and** stat on every event. | M | U6 | **Yes** |
| **U9** | Docs, `context.md` sweep, and the adversarial playtest gate. | M | U7, U8 | — |

### Modelling gates, before the plan they guard

- **Before U6** — counterattack frequency. A defensive crit both negates damage
  *and* fires riposte / auto-trip / auto-bash, and its rate is now margin-driven:
  2.3% flat today, versus **97.7%** when attacking far above your level and 0.5%
  when you outclass. Attacking something stronger means dealing nothing *and*
  eating a counter-strike on nearly every swing. A 42× swing that no part of the
  tuning package touches. Model it before flipping, not after.
- **Before U6** — the crit floor's denominator. 5.11e is "1% of hits", which
  assumed hit was binary. It becomes a continuum. Candidates: swings won outright
  (~50% at parity), swings dealing any damage (~96%), or all swings. They differ
  by 2× at parity and far more at the extremes.
- **Before U7** — ranged and taunt are free today on both sides. Giving them
  costs is a real nerf to two playstyles that have never paid.

---

## Standing rules for every plan in this arc

1. **No balance number inside `internal/`.** Every tuning value is a
   `_datafiles/config.yaml` edit. A numeric literal changed under `internal/` is
   a defect. This has happened before.
2. **`context.md` updates ship in the same PR**, not as a follow-up. Chunk 5.12
   found 61 phantom symbols across 22 packages because this was treated as
   optional.
3. **U1–U5 must be provable no-ops.** If an existing test needs changing, that is
   a signal the migration altered behaviour — stop and find out why.
4. **Delete as you migrate.** A site's old resolution path goes in the same plan
   that moves it.
5. **No legacy parameter survives U6.** If one does, the arc failed at its goal.

---

## The traps

Full list in spec section 7. The two that compile cleanly and silently:

- **Margin sign is opposite in two places.** `dice.OpposedRoll` returns an
  **attack-positive** margin; `bestDefenseResult.margin` is **defence-positive**.
  `normalizedAttackMargin` negates; `ContestCrit` must not. U1 fixes one
  convention at the seam and documents it.
- **An attack crit forces a hit.** Any crit adjustment evaluated before the hit
  outcome is final becomes an undeclared second hit floor leaking through
  `MinDefenseChance`.

---

## Done when

1. `dice.OpposedRoll*` is called from the core and nowhere else in
   `internal/actions`, `internal/combat`, `internal/hooks`, `internal/usercommands`.
2. Defence skill weight is ×5 in every channel; `SpellAttackSkillFactor` is gone
   from the attack path.
3. `TrySpellDeflection` and `TryStoicResolve` no longer exist as parallel
   mechanisms.
4. Adding a new contest requires declaring scores, a defence set and a channel —
   no new resolution code.
5. Parity damage-per-swing within ±10% of today at light, mid and BIS armour.
6. The adversarial playtest gate passes.
