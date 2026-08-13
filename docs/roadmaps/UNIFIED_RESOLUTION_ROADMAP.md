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
assigned deliberately rather than rediscovered at U11's audit.

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
| `combat/flee.go:85`, `:108` | U4 (flee) | **shipped in U4**, on the maneuver pair |
| `combat/grapple.go` (`AttemptGrapple`) | was UNOWNED, **claimed by U3** | **shipped in U3** |
| `combat/submission.go` (`RollSubmissionAttempt`) | was UNOWNED, **claimed by U3** | **shipped in U3** |
| `combat/skill_moves.go` (`ExecuteSkillMove`, 14 callers) | was UNOWNED, **claimed by U3** | **shipped in U3** |
| `hooks/Position_GrappleTick.go` (`processGrapplePair`) | was UNOWNED, **claimed by U3** | **shipped in U3** |
| `hooks/NewRound_MobRoundTick.go` (`tickMobCharmState`, charm-duration reroll) | was unclaimed, **claimed by U3** | **shipped in U3** |

After U3 the only `dice.OpposedRollStatWithFloors` callers left anywhere were the
two in `combat/flee.go`, which U4 owned and shipped. Line numbers are
deliberately gone from this table: they drifted within one chunk of being
written. **As of U4 both `dice.OpposedRollStat` and
`dice.OpposedRollStatWithFloors` have zero production callers and carry
`Deprecated:` markers; U6 deletes them.**

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
| `actions/combat_throttle.go:126` | Cast interrupt is a flat `util.Rand(100) < ThrottleInterruptChance` hanging off a maneuver. It bypasses concentration entirely, so U10's "concentration becomes a contest" does **not** reach it, so a master caster's spellcasting still counts for nothing here after U10 unless this is claimed. | **U10** |
| `actions/surprise_attack.go`, the swing loop | **Corrected in U4: this is not "hand-rolled per-weapon hit resolution". Surprise attack has NO hit resolution at all.** The primary weapon is appended with `hitPenalty: 0.0`, so `penaltyPct` is 0 and `util.Rand(100) < penaltyPct` never fires for it: every primary surprise swing is an unconditional auto-hit. The roll applies only to offhand and extra-arm swings, and it is a SELF-penalty, not a contest. There is no defender term anywhere, so a surprise attack against a novice and against the Elemental King resolve identically. Fits none of the sweep's categories A to D. **The user intends to REDESIGN this skill/effect rather than only give it a defender term, so U10 should treat it as a design slice (brainstorm, then spec, then plan), not a mechanical migration. Decided 2026-08-13.** | **U10** (was U4; U4 declined it because giving it a defender is a behaviour change and U1 to U5 are contracted as provable no-ops) |
| `hooks/Position_GrappleTick.go` z-normalisation | `z = res.Margin / res.AttackRoll.StdDev`, missing the `sqrt(2)` that `combat.ContestCrit` applies. Both sides roll with the attacker's stdDev, so the difference has `stdDev*sqrt(2)`; dividing by `stdDev` alone inflates every drift z by about 41%. Left as-is by U3 so the chunk stays a provable no-op, with a `NOTE(U6)` at the site. | **U6** |
| `actions/search.go` x6, `actions/track.go`, `forager/forage_core.go` | Static-difficulty checks still off the core. Two of `search.go`'s (hidden players, hidden mobs) answer the SAME question as `usercommands/go.go`'s opposed hidden-detection contest, but with a flat 135 threshold that ignores the hider's score entirely, so a hider's skill decides the outcome in one path and is ignored in the other. Mobs reach the search path too, via `behaviortree/actions_scout.go`'s `actTrySearch`, gated by `conditions_scout.go`'s `condRoomHasHiddenEntity`. `contest.AgainstDifficulty` was built for these and has zero production callers. | **UNASSIGNED.** U4 found them and breadcrumbed each site. Converting them is a behaviour change, so U4 could not claim them. Whichever chunk does must reconcile the two implementations, not just move one. |

**Also for U4: the floor guard does not see the new core.** The recurrence guard
`contest_floor_guard_test.go` (repo root, `package main`) walks the AST for
calls to `dice.OpposedRollStatRaw` and `dice.OpposedRoll` only. `contest.Run`
and `contest.AgainstDifficulty` are exported, unfloored, and completely
invisible to it, so a new caller can opt out of the floors through the very
package this arc built. U4 is the chunk that migrates the last of the remaining
sites, so extending the guard's function list (and its exemption for
`internal/combat`, which floors melee afterwards in `resolveDefenseOutcomeCore`)
belongs there. **Done in U4**, with a `internal/dice` exemption for the
deprecated pair, plus a second guard, `floor_pair_guard_test.go`.

> **Correction (U4).** Earlier notes in this file describe U4's sites as
> "unfloored". They were not. Every one of them called `dice.OpposedRollStat`,
> which chunk 5.10 renamed so the FLOORED roll carries the default name;
> `dice.OpposedRollStatRaw` is the unfloored escape hatch. The notes predate that
> rename. U4 was therefore a floored-to-floored migration on the global contest
> pair, which is exactly why it could be a provable no-op. What is genuinely
> unfloored is `contest.Run` and `contest.AgainstDifficulty`, which is why the
> guard now watches them.

**Moved out of B by decision:** knockdown and prone recovery become opposed rolls
against the opponent's stat + unarmed-combat (both currently roll against a flat
`dice.RollStat(50)`). Prone recovery opposes the current aggro target when there
is one and falls back to a static difficulty when there is not. Search, track and
forage stay static — there is genuinely no opponent and inventing one is worse.

> **Clarified 2026-08-13 (U4).** "Search, track and forage stay static" is a
> statement about their **shape**, not about their **ownership**. U4's first draft
> read it as ownership and nearly skipped them entirely. Staying static means
> they resolve against a difficulty rather than against an opponent, which is
> what `contest.AgainstDifficulty` exists for. It does **not** mean they are done,
> and it does not mean they stay off the contest core: they are still
> **UNASSIGNED** and still on flat `dice.RollStat` thresholds. See the row for
> them in the unowned table above.
>
> The two hidden-detection checks in `actions/search.go` are a partial exception
> to "no opponent": there IS an opponent, the hider, and `usercommands/go.go`
> already contests against them. Whichever chunk claims these must decide whether
> those two move to category A rather than staying static.

## Plans

| # | Plan | Size | Depends on | Behaviour change? |
|---|---|---|---|---|
| **U0** | **Delete the spell-initiation gate.** Ships independently, before or beside U1. | S | — | **Yes** (pure win) |
| **U1** | Contest core, bug-compatible. Generalise `runBestOfAllDefense`; **support contest-vs-static-difficulty, not only actor-vs-actor**; normalise the margin sign at the seam; melee migrated onto it. | L | — | **No** |
| **U2** | Spell channel onto the core (**6** sites — review found `resolveCharmSpell` too), preserving ×15 attack / ×0 defence as parameters. | M | U1 | **No** |
| **U3** | ✅ **DONE.** Ranged + taunt onto the core, preserving ×1 and the flat shield bonus. Also claimed and shipped every unowned `ManeuverFloors()` site, plus `TrySpellDeflection` on the spell pair, and deleted the four private floor accessors in `internal/hooks`. | M | U1 | **No** |
| **U4** | ✅ **DONE.** Non-harm contests onto the core: **19 sites, 17 on the global floor pair and 2 on the maneuver pair.** Global: `actions/sneak.go` (x2), `actions/shadow.go`, `actions/steal.go` (x4), `actions/plant.go` (x4), `actions/defuse.go`, `usercommands/go.go` (x4), `usercommands/skill.skullduggery.shadow.go`. Maneuver: both `combat/flee.go` rolls, the last `dice.OpposedRollStatWithFloors` callers, which are maneuvers because they are contested in combat and cost the round. Added `combat.RunWithGlobalFloors` + `combat.ContestFloors`, extended `contest_floor_guard_test.go` to see `contest.Run` / `contest.AgainstDifficulty`, and added `floor_pair_guard_test.go` to pin the pair, call count and attacker direction of every migrated site. **Declined:** `actions/surprise_attack.go`, reassigned to U10 (see below). | M | U1 | **No** |
| **U5a** | Foundation. Build `ApplyCost` + `ApplyHarm` (harm carries a **source actor** from the start). Move the hardcoded 2/4/5 defence costs into config **at their current effective values**. Tests for the three floor rules. Orphaned-docstring cleanup in `characters/character.go`. **No call sites moved.** | M | U1 | **No** |
| **U5b** | Route every pool mutation: ~78 logical sites, ~68 clamp lines deleted. Unfloor health harm. AST guard that no production code mutates a pool directly. **Named behaviour fixes:** the user/mob DoT floor asymmetry, the unguarded mob cast CP debit, the discarded `DeductDefenseStamina` bool. | L | U5a | **Yes, deliberately** (named sites only) |
| **U5c** | **Credit detection.** Immediate, *attributed* death when health drops below 1 at the harm site, replacing the deferred `Die(state.ActorRef{})` round-tick sweep. Sweep stays as a backstop for non-harm paths. | M | U5b | **Yes** |
| **U6** | **THE FLIP.** Uniform ×5, multiplier defence, margin-scaled mitigation, designed defence sets, `avoidance.go` absorbed, tuning package applied. **All legacy parameters deleted.** | L | U2–U5 | **Yes — all of it** |
| **U7** | **The unified cost model.** NEW SLICE, added 2026-08-13; everything below it shifted by one. Applies the spec's single cost formula to every action: flat config base, encumbrance multiplier (physical only), inverse-skill multiplier, per-action modifier. Takes defence cost off the hardcoded 2/4/5 for real. **Must map the companion / reserved-CP interaction before building.** | L | U5, U6 | **Yes** |
| **U8** | New cost surface: ranged, taunt and spell/taunt resistance start costing; skill-less roll on insufficient resource. (Was U7. The inverse-skill cost band moved into the new U7, where the whole formula now lives.) | M | U7 | **Yes** |
| **U9** | Progression layer: events not side effects, both sides, doing vs observing, skill **and** stat on every event. Category C (crafting, salvage) reaches it too. | M | U6 | **Yes** |
| **U10** | **Disruption model.** Concentration becomes a proper contest; knockdown and prone recovery become opposed rolls. | M | U1, U0 | **Yes** |
| **U11** | Docs, `context.md` sweep, **`config.yaml` organisation audit**, **player helpfiles for `quell` and `defy` plus the help-registry and category cleanup**, and the adversarial playtest gate. | M | U8–U10 | — |

### U5 — why it is three slices, and why it is NOT a no-op

**Split into U5a / U5b / U5c on 2026-08-13**, and the no-op contract was
deliberately released for it by the user: *"Rule 1 is a discipline rule, not a
rule you follow when we're finding obviously broken, scattered, and inconsistent
crap."* U5b and U5c change behaviour **at named sites only**, each called out
individually in its PR rather than absorbed silently.

What the inventory actually found, at 137 raw pool-mutation statements across
~78 logical sites (`internal/` only; `modules/` never mutates a pool):

- **Health harm is unfloored at 19 sites and clamped to 0 at 7.** One helper must
  change one group or the other. Unfloored wins: every downstream consumer tests
  `< 1` or `<= 0`, never `== 0`.
- **The user/mob DoT floors are opposite.** `NewRound_AutoHeal.go:217/229`
  (poison and bleed on a **user**) do not clamp; `:393/407` (the same DoTs on a
  **mob**) clamp to 0. Same tick, same buff, different floor.
- **`mobcommands/cast.go:116` has no affordability guard and no clamp.**
  `actions.InitiateCast` never reads Conviction, so a mob can begin a cast at 0
  CP and go negative.
- **`DeductDefenseStamina` returns a bool that `combat_helpers.go:665`
  discards.** A defence the character cannot pay for still wins the best-of-N.

**A CORRECTION, because a subagent got this wrong and it nearly shaped the plan:**
the 7 clamped sites do **not** prevent kills. `NewRound_MobRoundTick.go:124`
carries an always-runs death check testing `Health <= 0`, so a mob clamped to
exactly 0 dies on the next tick. What the clamp actually costs is worse in a
subtler way, and is what U5c exists to fix:

1. **A one-round zombie window** -- the mob sits at 0 HP, targetable and acting,
   until the next tick.
2. **Kill attribution is lost.** That sweep calls `Die(state.ActorRef{}, ...)`
   with an **empty** actor ref, so whoever landed the killing blow through any of
   these paths is anonymous to the death system. Grenades, DoTs and `pathto`
   attrition all kill without credit.
3. **Overkill magnitude is destroyed**, which U6's margin-scaled work wants.

So the fix is not "delete the clamps". It is: unfloor the harm, AND route death
at the damage site with a real killer reference. Those are separate changes,
which is why U5c is its own slice.

`ApplyHarm` therefore takes a **source actor in its signature from U5a onward**,
before any call site moves. Otherwise U5b routes ~78 sites and U5c has to touch
every one of them again.

### U5 — the three floor rules, and what they are NOT

The original line here read "no pool may go negative". That was poorly worded and
would have caused a real defect. Corrected by the user 2026-08-13. There are
**three** rules, not one, and the helper must know whether it is applying a COST
or HARM to pick the right one:

1. **A cost may never drive any pool below 0.** Spell costs, stamina for actions,
   all of it. If the actor cannot pay, that is a separate decision (U8 owns
   "the roll still happens with no skill"), not an overdraw.
2. **Harm floors stamina and conviction at 0.** They stop at empty.
3. **Harm may drive health below 0, and MUST be allowed to. That is how death
   works.** `ApplyHealthChange` (`internal/characters/resources.go`) deliberately
   permits it and the per-round hooks read it. Clamping health at 0 silently
   breaks death processing. Do not "fix" this.

### U7 — the unified cost model

**Added as its own slice 2026-08-13.** It began as "config-ify the defence costs"
inside U5, then as an encumbrance-driven defence cost. Both were rejected as
scope for U5, for two different reasons, and the second rejection is the
interesting one.

**Why it is not part of U5:** it is a behaviour change, and U1 to U5 are
contracted as provable no-ops. Dodge costs `int(2 x 0.9) = 1` stamina today;
under any encumbrance model a light-load dodge is about 2 and a burdened one far
more, so dodge cost rises by roughly 2x to 12x depending on load. That drains
defenders faster, which feeds the resource-depletion penalty curve and U6's
defence model. It needs modelling, not just implementing.

**Why it is not merely "encumbrance-based defence costs":** that design would
have **split the way costs work** -- defence derived from encumbrance while spell
costs stayed authored values. The user caught this. Fixing it properly means
spell cost becomes base x inverse-spellcasting x per-spell modifier, which
cascades into rhetoric costs too, which is a whole model rather than a tweak.

**The model already exists and does not need redesigning.** Spec section 3.1:

```
cost = baseCost(action)              # flat config value
     x encumbranceMultiplier(actor)  # PHYSICAL actions only
     x skillMultiplier(actor, skill) # inverse to skill, narrow band
     x configMultiplier(action)      # per-action tuning knob
```

The apparent split dissolves because **encumbrance is conditional on the action
being physical**, not because defence and spells use different formulas. Spec
3.2 already assigns a skill to every action (dodge to unarmed-combat, parry and
block to weapon-combat, cast to spellcasting, taunt/rally/warcry to rhetoric,
movement to search). Spec 3.3 already specifies the inverse-skill curve: centred
at **rank 35**, band **1.25 down to 0.75**, two joined `sqrt` segments matching
the `SkillMultiplier` idiom.

**User's tuning intent (2026-08-13), which reconciles with that formula:**
config values are **modifiers, not base costs** -- dodge **1.25** (you move your
whole body), parry **1.1**, block **1.15** -- with the bulk carried by
encumbrance, floor about **2** stamina and ceiling about **10** fully burdened.
Mapped onto the formula: base 2 is the floor, an encumbrance multiplier of 1 to 5
produces the 2 to 10 range, and 1.25/1.1/1.15 are the per-action modifier.

**RESOLVED: the 2-to-10 range applies BEFORE the per-defence modifier**, so a
fully burdened dodge reaches `2 x 5 x 1.25 = 12.5`. The ceiling is not a hard
post-modifier clamp, which would have made dodge, parry and block converge at
heavy load and erased the distinction exactly where it matters most.

**Blocking prerequisite:** map the interaction with **companions and reserved
CP** before building. `GetPoolReservation(pool, max)` is already general rather
than companion-specific, and reserve is already excluded from the low-resource
progression path (see the *fyttyn vitality exploit, 2026-04-16* comment). What is
unmapped is how a cost model should treat a reserved pool: does a cost see the
reserve, and does an actor holding a companion pay more or simply have less?

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

> **KEEP `SpellInitiationWillpowerDivisor` until U10.** Despite its name,
> `CalcConcentrationChance` also reads it (`cast_helpers.go:58`). Deleting it in
> U0 silently breaks concentration. U10 removes it when concentration stops using
> it.

### U10 — disruption model

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

Also in U10: **knockdown** and **prone recovery** move from `dice.RollStat(50)`
to opposed rolls against the opponent's stat + unarmed-combat. Removes
`SpellInitiationWillpowerDivisor`.

### U11 — player-facing help for the new defences, and a help-registry cleanup

**`quell` and `defy` are new player-facing vocabulary.** Nothing in the game
teaches them today, and a defence the player cannot look up is a defence they
will not understand losing to. This is not optional polish; it is the same gate
every other content change carries.

Required:

1. **New helpfiles** at `_datafiles/world/dogmud/templates/help/quell.template`
   and `defy.template` -- the **dogmud** layer, not the upstream `default` one.
   Each should say what it defends against, that it costs **Conviction**, and
   which skill drives it (`spellcasting` for quell, `rhetoric` for defy).
2. **Cross-links, both directions.** `help combat` and `help defense` must
   mention quell and defy alongside dodge/parry/block, and the new files must
   link back. Also consider: `help spellcasting`, `help rhetoric`, `help taunt`,
   `help conviction`. A trigger word that appears in no other helpfile is
   undiscoverable -- that is exactly how `stow` became invisible.
3. **Register them in `_datafiles/world/dogmud/keywords.yaml`.** The `help` index
   is hand-maintained and there is **no fallback to the command registry**, so a
   helpfile that is not in that YAML never appears in the topic list. `stow`,
   `tutorial` and `trade` are already missing for this reason.
4. **Update the existing player text.** `TrySpellDeflection` and
   `TryStoicResolve` ship strings saying "deflect" and "resolve"
   (`internal/hooks/spell_resolution.go`,
   `internal/hooks/NewRound_DoCombat_helpers.go`,
   `internal/hooks/charm_spell.go`). U6 renames the mechanics; U11 must not leave
   the old words in player copy.

**Also in this slice, unrelated to the arc: the help category list needs a
cleanup.** `help` groups topics by category, and the categories have drifted into
duplicates and near-duplicates. Two confirmed by audit on 2026-08-13, both caused
by module overlays merging last-write-wins onto a flat map:

- `modules/auctions/files/data-overlays/keywords.yaml` files `auction` under
  **`shop`** (singular) while the main file uses **`shops`** (plural), so the
  rendered index shows two separate headings and the second contains one entry.
- `modules/cleanup/.../keywords.yaml` re-files `bury` and `trash` under
  **`information`**, overriding the main file's `character` and `items`
  placements and leaving those main-file entries dead.

Sweep the full category set for others rather than fixing only these two. See
the broader command/help audit recorded separately, which also covers ~13 dead
upstream helpfiles and the commands missing from the index.

### U11 — `config.yaml` organisation audit

By the time this arc lands, `_datafiles/config.yaml` will have absorbed a great
many contest, floor and channel knobs, added chunk by chunk with no pass over the
file as a whole. U11 owns a sanity check on the file itself, not just on the
knobs this arc touched:

- **Grouping.** Are related knobs adjacent? The floor pairs are the obvious test
  case: `MinContestSuccessChance`, `MinSpellHitChance` and `MinManeuverHitChance`
  are one family expressing one principle (the cost of a single failure) and
  should read as one block, with that principle stated once above them.
- **Ordering.** Sections should follow the shape of the systems they configure,
  and knobs within a section should be ordered deliberately rather than by the
  order chunks happened to land.
- **Comments.** Every knob needs what it does, what changing it costs, and its
  live value's rationale where the value is non-obvious. Several existing
  comments describe the pre-arc model and are now wrong — a comment describing
  the removed model is worse than no comment.
- **Stale and orphaned keys.** Knobs whose readers this arc deleted must go.
  `SpellAttackSkillFactor` and the legacy per-channel skill weights are the
  known ones; U11 should sweep for others rather than trust that list.
- **Drift check.** Flag any knob whose shipped value differs sharply from its Go
  default without a comment explaining why (`SpellDamageScale` ships at 3.12
  against a default of 1.0). Absence is meaningful too: a knob left out of the
  file falls back to its Go default, and `0` is a legal shipped value.

Two traps worth stating here because they have already caused defects.
**The three floor pairs all ship at 0.05**, which makes a wrong-pair wiring
invisible in production — do not "simplify" them into one knob during a tidy-up;
they are one value by coincidence, not by rule. And **a Go test binary never
loads this file**, so config-read knobs measure their struct zero value under
test, not the shipped value and not necessarily the documented default.

This is a documentation and ergonomics pass. **No value changes.** Any retune
found along the way is filed, not applied — a config edit inside a docs chunk is
how an unreviewed balance change ships.

### U6 — the defence sets, inlined because the reference was too thin

The flip table above says `defenceSet` goes to "designed sets 3 / 2 / 2 / 1 / 1"
and the U6 row says "designed defence sets". **That was too thin a pointer.** On
2026-08-13 it caused a real error: an agent grepped for a spell-resist *stamina*
cost, found none, and reported spell resist as a phantom mechanic that did not
exist -- when it is fully designed in the contest spec and costs **CP, not SP**,
which is exactly why the grep was blind. Inlining the table so the roadmap is
readable on its own.

**Source of truth: `2026-08-12-unified-contest-resolution-design.md` section 3.2.**
The applicable defence set is a property of the ATTACK TYPE:

| Attack type | Applicable defences | N |
|---|---|---|
| Melee | dodge, parry, block | 3 |
| Ranged | dodge, block | 2 |
| Spell, physical damage | dodge, block | 2 |
| Spell, mental | **quell** (`Wil + spellcasting x5`) | 1 |
| Taunt / social | **defy** (`Wil + rhetoric x5`) | 1 |

**The five defences are `dodge`, `parry`, `block`, `quell`, `defy`.** All five
are short verbs, deliberately -- they read as one set in a combat log.

Notes that matter for anyone routing costs:

- **`quell` and `defy` are NEW NAMES, chosen 2026-08-13.** Both defences were
  previously called "resist", which collided: the mental defence and the social
  defence shared one word, which would have been ambiguous the moment either
  reached a knob name or player copy. **Quell** is the mental-spell defence
  (you put the working down); **defy** is the social defence (you refuse to rise
  to it).
- **Dodge is REUSED for physical spells.** There is no separate physical-spell
  defence.
- **Parry is deliberately excluded from ranged and physical spells.** You cannot
  parry a bolt. Adding it later is one row in this table and nothing else, which
  is the point of the design.
- **Quell and defy cost CONVICTION, not stamina.** Grepping for a stamina cost
  will find nothing and prove nothing. This is exactly the error that made an
  agent report spell resist as a phantom mechanic on 2026-08-13.
- They exist in code today as `TrySpellDeflection` (→ **quell**) and
  `TryStoicResolve` (→ **defy**), in `internal/combat/avoidance.go`, which U6
  absorbs. **Their shipped player text currently says "deflect" and "resolve"
  and must be rewritten to the new names when U6 lands them** -- see
  `internal/hooks/spell_resolution.go` and
  `internal/hooks/NewRound_DoCombat_helpers.go` for the existing strings.

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
- **Before U8** — ranged and taunt are free today on both sides. Giving them
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
- **All three floor pairs ship at 0.05.** Wiring a contest to the wrong pair
  changes nothing observable in production and becomes a balance bug the moment
  U6 retunes one. Behavioural tests cannot see it; `floor_pair_guard_test.go`
  reads the source instead.
- **The three pairs differ in a TEST binary**, and not the way you would guess:
  global reads a `dice` package var (0.05), maneuver and spell read config
  (never loaded under test, so **0**). Never quote a Go default as a live value.

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

   U11 is where the sweep happens, but the gate is on the arc, not on U11: no plan
   in U0–U10 may merge with its own package docs stale, per standing rule 2.
7. The adversarial playtest gate passes.
