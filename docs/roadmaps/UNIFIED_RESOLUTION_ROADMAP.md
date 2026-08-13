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
| `skillWeight` | per-channel 5 / 1 / 0 / 15 (**incomplete: this covers four channels only. The special-move family shares no skill-weight convention at all, so there is no fifth number to add here. See the pre-U6 gate for the measured, per-site table.**) | uniform 5.0, **parameter deleted** |
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

The maneuver-floor family. When this table was written `combat.ManeuverFloors()`
had eight callers, and the hooks package reached the same config pair through
its own `maneuverHitFloor()`/`maneuverResistFloor()` accessors, so those counted
too. **U3 claimed and shipped every unowned row below**, and deleted the hooks
accessors; `internal/hooks` no longer reads the floors and no longer imports
`internal/contest`.

| Site | Claimed by | State |
|---|---|---|
| `actions/combat_taunt.go` (`ExecuteTaunt`) | U3 (taunt) | **shipped in U3** |
| `usercommands/throw.go` (`Throw`) | U3 (ranged) | **shipped in U3** |
| `combat/avoidance.go` (`TryStoicResolve`) | U3 (conviction defence) | **shipped in U3** |
| `combat/avoidance.go` (`TrySpellDeflection`) | U3, on the SPELL pair | **shipped in U3** |
| `combat/flee.go:85`, `:108` | U4 (flee) | still on `dice.OpposedRollStatWithFloors` |
| `combat/grapple.go` (`AttemptGrapple`) | was UNOWNED, **claimed by U3** | **shipped in U3** |
| `combat/submission.go` (`RollSubmissionAttempt`) | was UNOWNED, **claimed by U3** | **shipped in U3** |
| `combat/skill_moves.go` (`ExecuteSkillMove`, 14 callers) | was UNOWNED, **claimed by U3** | **shipped in U3** |
| `hooks/Position_GrappleTick.go` (`processGrapplePair`) | was UNOWNED, **claimed by U3** | **shipped in U3** |
| `hooks/NewRound_MobRoundTick.go` (`tickMobCharmState`, charm-duration reroll) | was unclaimed, **claimed by U3** | **shipped in U3** |

After U3 the only `dice.OpposedRollStatWithFloors` callers left anywhere are the
two in `combat/flee.go`, which U4 owns. Line numbers are deliberately gone from
this table: they drifted within one chunk of being written.

**`skill_moves.go` was the awkward one.** `ExecuteSkillMove` is shared between
ranged (`actions/combat_fire.go` folds its defence into a scalar and calls it)
and melee's bash/kick/trip. **Resolved in U3 by migrating the shared function
rather than forking ranged out of it**: one call site, all 14 callers, no
duplicate resolution path. The cost of that choice is that the ranged/melee
skill-weight question is now a property of one function; see the new pre-U6
modelling gate below.

Also only implicitly covered: `actions/shadow.go`,
`usercommands/skill.skullduggery.shadow.go`, and the hidden-detection checks in
`usercommands/go.go`, plausibly part of U4's "sneak", but not named. **Now
named: they are listed explicitly in U4's row in the Plans table.**

### Newly-found unowned uncertain outcomes (2026-08-12, U3 task 6)

The sweep above searched for the floor accessors, so it saw only sites that
already knew they were contests. Searching for the *concepts* instead found
three more that no chunk claims:

| Site | What | Assigned to |
|---|---|---|
| `actions/combat_throttle.go:126` | Cast interrupt is a flat `util.Rand(100) < ThrottleInterruptChance` hanging off a maneuver. It bypasses concentration entirely, so U9's "concentration becomes a contest" does **not** reach it, so a master caster's spellcasting still counts for nothing here after U9 unless this is claimed. | **U9** |
| `actions/surprise_attack.go:222-225` | Hand-rolled per-weapon hit resolution: `util.Rand(100) < penaltyPct` per swing, which never contests the defender at all. Fits none of the sweep's categories A to D (it is opposed in intent, flat-percentage in implementation, and has no defender term). | **U4** |
| `hooks/Position_GrappleTick.go` z-normalisation | `z = res.Margin / res.AttackRoll.StdDev`, missing the `sqrt(2)` that `combat.ContestCrit` applies. Both sides roll with the attacker's stdDev, so the difference has `stdDev*sqrt(2)`; dividing by `stdDev` alone inflates every drift z by about 41%. Left as-is by U3 so the chunk stays a provable no-op, with a `NOTE(U6)` at the site. | **U6** |

**Also for U4: the floor guard does not see the new core.** The recurrence guard
`contest_floor_guard_test.go` (repo root, `package main`) walks the AST for
calls to `dice.OpposedRollStatRaw` and `dice.OpposedRoll` only. `contest.Run`
and `contest.AgainstDifficulty` are exported, unfloored, and completely
invisible to it, so a new caller can opt out of the floors through the very
package this arc built. U4 is the chunk that migrates the remaining unfloored
sites, so extending the guard's function list (and its exemption for
`internal/combat`, which floors melee afterwards in `resolveDefenseOutcomeCore`)
belongs there.

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
| **U3** | ✅ **DONE.** Ranged + taunt onto the core, preserving ×1 and the flat shield bonus. Also claimed and shipped every unowned `ManeuverFloors()` site, plus `TrySpellDeflection` on the spell pair, and deleted the four private floor accessors in `internal/hooks`. | M | U1 | **No** |
| **U4** | Non-harm contests onto the core: sneak, steal, plant, defuse, flee (contest + progression layers only, no harm layer). **Named explicitly:** `combat/flee.go:85`, `:108` (the last `dice.OpposedRollStatWithFloors` callers left); `actions/shadow.go`; `usercommands/skill.skullduggery.shadow.go`; the hidden-detection checks in `usercommands/go.go`; `actions/surprise_attack.go:222-225`. **Also extend `contest_floor_guard_test.go` to see `contest.Run` / `contest.AgainstDifficulty`.** | M | U1 | **No** |
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

- **Before U6: the special-move family's skill weight. NEW, added by U3.**
  The flip table above says `skillWeight` goes from "per-channel 5 / 1 / 0 / 15"
  to "uniform 5.0". That table describes melee, ranged, spell and taunt. It does
  not describe most of what U3 migrated. `grep -rn "SkillWeight"
  internal/actions/ internal/combat/` returns **zero** hits in any of
  `ExecuteSkillMove`'s 14 callers: the function adds `AttackSkill` and
  `DefenseSkill` to the stats raw. Measured from source, 2026-08-12:

  | Site | Attack weight | Defence weight |
  |---|---|---|
  | `ExecuteSkillMove` (bash / kick / trip / gore / hamstring / maul / pounce / rake / throttle / drain ×2, riposte-trip, auto-bash) | ×1 | ×1 |
  | `ExecuteFire` (ranged) | ×1 | ×1 + flat shield bonus, folded into one scalar with `DefenseStat: 0` |
  | `AttemptGrapple` | ×1 | ×1 |
  | `RollSubmissionAttempt` | ×`SubSkillWeight` (1.5) | ×1.5 |
  | `processGrapplePair` (grapple drift) | ×2.2 aggressor / ×2.0 defender | same |
  | `usercommands.Throw` (AoE grenade) | ×`SkillWeight` (5) on skullduggery | ×2.5 on **perception, a stat** (`SkillWeight × 0.5`); no defence skill term at all |
  | taunt / `TryStoicResolve` | ×5 | ×5 |

  **The rule, not the count: these sites share no skill-weight convention, and
  the table above is the authority.** Read down the two weight columns and no two
  consecutive rows have to agree. Deliberately stated as a rule, because a count
  that is right today is the next stale comment: this line said "six" while the
  table it summarised showed five distinct weight pairs (×1/×1, ×1.5/×1.5,
  ×2.2/×2.0, ×5/×2.5, ×5/×5), with ranged differing structurally rather than
  numerically. Add the other channels already on the core, spell at ×15/×0 and
  melee at ×5/×5, and the pairing changes again. Count from the table when a
  number is actually needed, and say what you counted.

  What matters for the gate is not the total but that it is greater than one.
  U6's
  stated action is "uniform ×5, parameter deleted". Applied naively that moves
  14 sites from ×1 to ×5 on both sides at once. Against mobs, which all carry
  combat skill 1, a weapon-combat-30 player's bash goes from `130 vs 101` to
  `250 vs 105`. Nobody has modelled that, because it is not in the table the
  modelling was done against. **Model it before U6. Do not fix it in U3:** U3 is
  a provable no-op by contract, and a weight change is a behaviour change.

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
