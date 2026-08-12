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

## The sweep — 2026-08-12

Every uncertain outcome in the game, sorted by whether the model can hold it.
Run before the first plan, deliberately, so U1's signature is not designed
against an incomplete picture.

| Category | What | Verdict |
|---|---|---|
| **A** — opposed, actor vs actor | 33 sites + melee: combat, spells, taunt, ranged, grapple, submission, flee, sneak, steal, plant, defuse, shadow, hidden detection | **Fits.** U1–U4. |
| **B** — roll vs static difficulty | `search.go` ×6, `track.go`, `forager`, knockdown, prone recovery | **Fits only if the core supports it** — see U1 scope |
| **C** — flat `util.Rand(100)` percentage | crafting, salvage, spell initiation, concentration break, AI decisions | **Out of the contest core** — a craft is a probability against a recipe, not a contest against an opponent. Must still reach the **progression layer** |
| **D** — not a roll at all | **picklock** — a sequence-matching minigame (`sequenceMatches`, `GetLockRender`); skill sets puzzle difficulty, the player supplies the outcome | **Permanently out of scope.** Document as a deliberate exclusion or someone will "unify" a puzzle into a dice roll |

### Ownership gaps found by review — assign before writing U3/U4

The U2 plan originally missed `resolveCharmSpell` because its verification
grepped one file. Adversarial review caught it, and a second sweep then found
more sites owned by **no chunk's stated scope**. Recording them here so they are
assigned deliberately rather than rediscovered at U10's audit.

The maneuver-floor family. `combat.ManeuverFloors()` has eight callers; the
hooks package reaches the same config pair through its own
`maneuverHitFloor()`/`maneuverResistFloor()` accessors, so those count too:

| Site | Claimed by |
|---|---|
| `actions/combat_taunt.go:128` | U3 (taunt) |
| `usercommands/throw.go:153` | U3 (ranged) |
| `combat/avoidance.go:80` (`TryStoicResolve`) | U3 (conviction defence) |
| `combat/flee.go:84`, `:107` | U4 (flee) |
| **`combat/grapple.go:79`** | **UNOWNED** |
| **`combat/submission.go:79`** | **UNOWNED** |
| **`combat/skill_moves.go:61`** (bash / kick / trip) | **UNOWNED** |
| **`hooks/Position_GrappleTick.go:270`** (via `maneuverHitFloor()`) | **UNOWNED** |
| `hooks/NewRound_MobRoundTick.go:398` (charm-duration reroll, same accessors) | unclaimed; belongs with U3's maneuver work |

Five of those are claimed, four are not.

**`skill_moves.go` is the awkward one.** `ExecuteSkillMove` is shared between
ranged (`combat_fire.go` folds its defence into a scalar and calls it) and
melee's bash/kick/trip. So U3 "migrating ranged" either touches a function that
also drives melee special moves, or forks ranged out of it. Decide that when
writing U3 rather than discovering it mid-implementation.

Also only implicitly covered: `actions/shadow.go`,
`usercommands/skill.skullduggery.shadow.go`, and the hidden-detection checks in
`usercommands/go.go` — plausibly part of U4's "sneak", but not named. Name them.

**Moved out of B by decision:** knockdown and prone recovery become opposed rolls
against the opponent's stat + unarmed-combat (both currently roll against a flat
`dice.RollStat(50)`). Prone recovery opposes the current aggro target when there
is one and falls back to a static difficulty when there is not. Search, track and
forage stay static — there is genuinely no opponent and inventing one is worse.

## Plans

| # | Plan | Size | Depends on | Behaviour change? |
|---|---|---|---|---|
| **U0** | **Delete the spell-initiation gate.** Ships independently, before or beside U1. | S | — | **Yes** (pure win) |
| **U1** | Contest core, bug-compatible. Generalise `runBestOfAllDefense`; **support contest-vs-static-difficulty, not only actor-vs-actor**; normalise the margin sign at the seam; melee migrated onto it. | L | — | **No** |
| **U2** | Spell channel onto the core (**6** sites — review found `resolveCharmSpell` too), preserving ×15 attack / ×0 defence as parameters. | M | U1 | **No** |
| **U3** | Ranged + taunt onto the core, preserving ×1 and the flat shield bonus. **Must also claim the unowned `ManeuverFloors()` sites — see Ownership gaps.** | M | U1 | **No** |
| **U4** | Non-harm contests onto the core: sneak, steal, plant, defuse, flee (contest + progression layers only, no harm layer). | M | U1 | **No** |
| **U5** | Cost + harm helpers. One cost helper, one harm helper. Config-ify the hardcoded 2/4/5 defence costs. No pool may go negative. | M | U1 | **No** (costs unchanged, only routed) |
| **U6** | **THE FLIP.** Uniform ×5, multiplier defence, margin-scaled mitigation, designed defence sets, `avoidance.go` absorbed, tuning package applied. **All legacy parameters deleted.** | L | U2–U5 | **Yes — all of it** |
| **U7** | New cost surface: ranged, taunt and spell/taunt resistance start costing; skill-less roll on insufficient resource; inverse-skill cost band. | M | U5, U6 | **Yes** |
| **U8** | Progression layer: events not side effects, both sides, doing vs observing, skill **and** stat on every event. Category C (crafting, salvage) reaches it too. | M | U6 | **Yes** |
| **U9** | **Disruption model.** Concentration becomes a proper contest; knockdown and prone recovery become opposed rolls. | M | U1, U0 | **Yes** |
| **U10** | Docs, `context.md` sweep, and the adversarial playtest gate. | M | U7–U9 | — |

### U0 — delete the spell-initiation gate

Not a taste call. `CalcInitiationChance` clamps at **95**, and Meirok's computed
value is **1372**:

```
base 60 + willpower 148/4 (=37) + skill 51 × SkillWeight 5 × factor 5 (=1275) = 1372 → 95
```

No amount of skill can ever exceed 95%, so a caster who has mastered the skill
still fails one cast in twenty, forever, each failure carrying a 2-round
`cast-init` cooldown. The gate is not gating anything; it is a permanent 5% tax
that mastery cannot touch. Concentration break already covers the design intent.

Delete: `CalcInitiationChance`, the `cast-init` cooldown (`skill.cast.go:135`,
`:189`, `action_readiness.go:131`), the initiation roll block
(`skill.cast.go:183-195`), and knobs `SpellInitiationBase`,
`SpellInitiationSkillFactor`.

> **KEEP `SpellInitiationWillpowerDivisor` until U9.** Despite its name,
> `CalcConcentrationChance` also reads it (`cast_helpers.go:58`). Deleting it in
> U0 silently breaks concentration. U9 removes it when concentration stops using
> it.

### U9 — disruption model

**Concentration is a contest, not a flat percentage.** Today
`CalcConcentrationChance` is `base(50) + willpower/4 − damagePct`, clamped 5–95,
with **no spellcasting term at all** — the third instance of the ×0
defence-skill bug, after spell defence and ranged defence. A master's skill
contributes nothing to holding a spell together.

Becomes: **`Wil + spellcasting×5` against a static difficulty set by what
happened.** Damage alone is the wrong trigger — being knocked flat or grappled
is more disruptive than a harder hit that leaves you standing.

| Disruption | Difficulty |
|---|---|
| Knocked prone | **250** |
| Grappled | **300** |
| Damage from a single source | **`damagePct × 10`** |

Expressed as a percentage of the pool, damage stays scale-free and the
relationship reads correctly: a 25% hit is about as disruptive as being knocked
down; a 40% hit is worse than being grappled.

Resulting hold rates:

| Caster | Prone | Grappled | 10% hit | 25% hit | 40% hit |
|---|---|---|---|---|---|
| novice (Wil 100, sc 5) | 0% | 0% | 91% | 0% | 0% |
| journeyman (Wil 120, sc 25) | 45% | 7% | 100% | 45% | 0% |
| adept (Wil 135, sc 40) | 95% | 76% | 100% | 95% | 10% |
| Meirok (Wil 148, sc 51) | 99% | 96% | 100% | 99% | 52% |

- **Trigger threshold ~10%.** Below that, no roll at all. Chip damage should not
  generate rolls, and a 10% hit passes for everyone above novice anyway.
- **Concentration needs its own floor knob at ~0.02, or none.** The standard
  contest floor of 0.15 would break a master's concentration 15% of the time on
  every qualifying disruption — three times worse than the initiation tax U0
  deletes for being annoying.
- **Grapple is a deliberate caster-killer.** Below adept it effectively stops
  casting. Intended: it makes grapple a real anti-caster tool and gives
  grapple-behaviour mobs a defined role.
- **Rejected: opposed against the disruptor.** It scales beautifully with content
  but hard-counters casting outright — modelled, Meirok holds **0%** when the
  Elemental King grapples him, so endgame bosses would simply switch casting off.

Also in U9: **knockdown** and **prone recovery** move from `dice.RollStat(50)`
to opposed rolls against the opponent's stat + unarmed-combat. Removes
`SpellInitiationWillpowerDivisor`.

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
6. **Documentation is current, and this is a completion gate, not a slice.**
   - `context.md` is accurate for every package touched: `internal/combat`,
     `internal/actions`, `internal/hooks`, `internal/characters`, `internal/dice`,
     `internal/usercommands`. Verified with `tools/context_md_audit.py`, zero
     phantom symbols.
   - Every comment describing the per-channel resolution this arc removes has
     been corrected or deleted. A comment describing the old model is worse than
     no comment — an agent will code against it.
   - `docs/PATCH_NOTES.md` carries a player-facing entry.
   - Both specs and this roadmap reflect what actually shipped, including any
     decision that changed during implementation.
   - Every new config knob is documented **in `config.yaml` itself**, next to its
     value, with what it does and what changing it costs.

   U10 is where the sweep happens, but the gate is on the arc, not on U10: no plan
   in U0–U9 may merge with its own package docs stale, per standing rule 2.
7. The adversarial playtest gate passes.
