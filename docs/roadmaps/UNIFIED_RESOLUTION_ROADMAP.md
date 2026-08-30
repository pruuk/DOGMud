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
| `actions/combat_throttle.go:126` | Cast interrupt is a flat `util.Rand(100) < ThrottleInterruptChance` hanging off a maneuver. It bypasses concentration entirely, so U10's "concentration becomes a contest" does **not** reach it, so a master caster's spellcasting still counts for nothing here after U10 unless this is claimed. | **U10 — CLAIMED and SHIPPED 2026-08-21.** Now a fourth `RunConcentrationContest` trigger; `ThrottleInterruptChance` is deleted. |
| `actions/surprise_attack.go`, the swing loop | **Corrected in U4: this is not "hand-rolled per-weapon hit resolution". Surprise attack has NO hit resolution at all.** The primary weapon is appended with `hitPenalty: 0.0`, so `penaltyPct` is 0 and `util.Rand(100) < penaltyPct` never fires for it: every primary surprise swing is an unconditional auto-hit. The roll applies only to offhand and extra-arm swings, and it is a SELF-penalty, not a contest. There is no defender term anywhere, so a surprise attack against a novice and against the Elemental King resolve identically. Fits none of the sweep's categories A to D. **The user intends to REDESIGN this skill/effect rather than only give it a defender term, so this should be treated as a design slice (brainstorm, then spec, then plan), not a mechanical migration. Decided 2026-08-13.** | **U10d** (split out of U10 2026-08-21; was U4 before that — U4 declined it because giving it a defender is a behaviour change and U1 to U5 are contracted as provable no-ops) |
| `hooks/Position_GrappleTick.go` z-normalisation | `z = res.Margin / res.AttackRoll.StdDev`, missing the `sqrt(2)` that `combat.ContestCrit` applies. Both sides roll with the attacker's stdDev, so the difference has `stdDev*sqrt(2)`; dividing by `stdDev` alone inflates every drift z by about 41%. Left as-is by U3 so the chunk stays a provable no-op, with a `NOTE(U6)` at the site. | **U6** |
| `actions/search.go` x6, `actions/track.go`, `forager/forage_core.go` | Static-difficulty checks still off the core. Two of `search.go`'s (hidden players, hidden mobs) answer the SAME question as `usercommands/go.go`'s opposed hidden-detection contest, but with a flat 135 threshold that ignores the hider's score entirely, so a hider's skill decides the outcome in one path and is ignored in the other. Mobs reach the search path too, via `behaviortree/actions_scout.go`'s `actTrySearch`, gated by `conditions_scout.go`'s `condRoomHasHiddenEntity`. `contest.AgainstDifficulty` was built for these and has zero production callers. | **U10b-1b.** Phase A converted `search.go`'s four non-stealth tiers and `forager/forage_core.go` to `contest.AgainstDifficulty` (score 100 vs a 125 target: 4.8% -> ~11.9%; score 175: 97.2% -> ~91.1%). `actions/track.go` is **DEFERRED with a written reason**: its single roll feeds a 125/135/175 LADDER, and contesting only the gate decouples the bands from it (measured: at score 100, 73.8% of successful reads carry a roll below 125). The two hidden-entity checks are **Phase C**, and go to `combat.RunContest` rather than `AgainstDifficulty` because they are genuinely opposed. |

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
> them in the table above.
>
> **Updated 2026-08-28 (U10b-1b Phase A):** no longer unassigned, and no longer
> all on flat thresholds. `search.go`'s four non-stealth tiers and
> `forager/forage_core.go` are converted. `track.go` remains on `dice.RollStat`
> DELIBERATELY, and the reason is in the file: a contested gate plus raw-roll
> quality bands decouples a ladder that used to be nested by construction.
>
> The two hidden-detection checks in `actions/search.go` are a partial exception
> to "no opponent": there IS an opponent, the hider, and `usercommands/go.go`
> already contests against them. Whichever chunk claims these must decide whether
> those two move to category A rather than staying static.

## Plans

> **Status as of 2026-08-29: everything through U10d is MERGED. Only U12 then
> U11 remain, in that order. Nothing is deployed — prod is still `7c64c228c`,
> and the deploy gate is the whole arc plus a playtest.**
>
> ⚠️ **Every ✅ here now carries its merge evidence — a PR number, a commit, or
> a named reason there is no single one. Trust the evidence, not the tick.**
> Six rows (U0, U1, U2, U5c, U6, U7, U10b) sat unmarked for weeks while their
> code was long since merged, which made this table actively misleading: it read
> as though a third of the arc was outstanding. Anyone asking "is the arc
> complete?" should check the evidence column and, for anything load-bearing,
> confirm in source — that is how U8 was nearly missed in both directions.
>
> Three stages have no single branch to point at and say so in their rows: **U6**
> (landed in pieces, certified by U6b), **U10b** (five sub-slices), and **U1**
> (commits, no PR of its own).

| # | Plan | Size | Depends on | Behaviour change? |
|---|---|---|---|---|
| **U0** | ✅ **DONE 2026-08-12.** **Delete the spell-initiation gate.** `SpellInitiationBase` and `SpellInitiationSkillFactor` are gone from `internal/` (0 references) and `_datafiles/config.yaml` records the removal at the spellcasting block. Ships independently, before or beside U1. | S | — | **Yes** (pure win) |
| **U1** | ✅ **DONE** (`83683376c` feat(contest): pure roll-and-select core; `27529b011` its `context.md`). Contest core, bug-compatible. Generalise `runBestOfAllDefense`; **support contest-vs-static-difficulty, not only actor-vs-actor**; normalise the margin sign at the seam; melee migrated onto it. The package is `internal/contest`. | L | — | **No** |
| **U2** | ✅ **DONE** (PR #32, merge `f36a44423`). Spell channel onto the core (**6** sites — review found `resolveCharmSpell` too), preserving ×15 attack / ×0 defence as parameters. | M | U1 | **No** |
| **U3** | ✅ **DONE.** Ranged + taunt onto the core, preserving ×1 and the flat shield bonus. Also claimed and shipped every unowned `ManeuverFloors()` site, plus `TrySpellDeflection` on the spell pair, and deleted the four private floor accessors in `internal/hooks`. | M | U1 | **No** |
| **U4** | ✅ **DONE.** Non-harm contests onto the core: **19 sites, 17 on the global floor pair and 2 on the maneuver pair.** Global: `actions/sneak.go` (x2), `actions/shadow.go`, `actions/steal.go` (x4), `actions/plant.go` (x4), `actions/defuse.go`, `usercommands/go.go` (x4), `usercommands/skill.skullduggery.shadow.go`. Maneuver: both `combat/flee.go` rolls, the last `dice.OpposedRollStatWithFloors` callers, which are maneuvers because they are contested in combat and cost the round. Added `combat.RunWithGlobalFloors` + `combat.ContestFloors`, extended `contest_floor_guard_test.go` to see `contest.Run` / `contest.AgainstDifficulty`, and added `floor_pair_guard_test.go` to pin the pair, call count and attacker direction of every migrated site. **Declined:** `actions/surprise_attack.go`, reassigned to U10, then split out again 2026-08-21 as **U10d** (see below). | M | U1 | **No** |
| **U5a** | ✅ **DONE** (PR #36). Foundation. Build `ApplyCost` + `ApplyHarm` (harm carries a **source actor** from the start). Move the hardcoded 2/4/5 defence costs into config **at their current effective values**. Tests for the three floor rules. Orphaned-docstring cleanup in `characters/character.go`. **No call sites moved.** | M | U1 | **No** |
| **U5b** | ✅ **DONE** (U5b-1 PR #37, U5b-2). Routed every pool mutation: ~78 logical sites, ~68 clamp lines deleted. Health harm unfloored everywhere, with `Character.DisplayHealth()` added as the single display-layer clamp. AST guard that no production code mutates a pool directly. `DeductStamina` and `DeductDefenseStamina` deleted. **Named behaviour fixes:** the user/mob DoT floor asymmetry, the unguarded mob cast CP debit, and the defence affordability gate that dropped exhausted defenders onto the `MinDefenseChance` last resort. **Disclosed gap:** `GetDefenseScore` has no resource term, so until U8 an exhausted defender defends as well as a rested one. | L | U5a | **Yes, deliberately** (named sites only) |
| **U5c** | ✅ **DONE** (PR #46, merge `670fc2e02`). **Credit detection.** Immediate, *attributed* death when health drops below 1 at the harm site, replacing the deferred `Die(state.ActorRef{})` round-tick sweep. Sweep stays as a backstop for non-harm paths. ⚠️ **Disclosed gap, still unowned by any stage:** `buffs.Buff` carries a free-text `Source` but no applier actor, so DoT, poison and toxicity harm passes `state.ActorRef{}` (`NewRound_AutoHeal.go:88`, `:226`) and those deaths name no killer. Verified 2026-08-29 to have **no live consequence** — the gold-transfer branch needs `GoldLossFraction > 0`, which only subdue/cripple set, and the bounty `killGuard` branch needs a faction guard landing the killing DoT tick, which no guard in content applies. It is latent, not broken: it bites the day a bounty guard gets a poison attack. | M | U5b | **Yes** |
| **U6** | ✅ **DONE, but NOT as one branch — do not go looking for a `u6-the-flip` PR.** It was redesigned 2026-08-14 and landed in pieces: `3bd490beb` (the single `ContestFloor` knob replacing eight), `dc2552e7f` (floor the winner before deriving crit, second floor deleted), modelling in PR #47, and it is **finished by U6b** (PR #53) — see that row, which is the one that certifies every channel is across. **THE FLIP.** Uniform ×5, multiplier defence, margin-scaled mitigation, designed defence sets, `avoidance.go` absorbed, tuning package applied. **All legacy parameters deleted.** | L | U2–U5 | **Yes — all of it** |
| **U7** | ✅ **DONE** (PR #48, merge `1f6251fb6`). **The unified cost model.** NEW SLICE, added 2026-08-13; everything below it shifted by one. Applies the spec's single cost formula to every action: flat config base, encumbrance multiplier (physical only), inverse-skill multiplier, per-action modifier. Takes defence cost off the hardcoded 2/4/5 for real. **Must map the companion / reserved-CP interaction before building.** | L | U5, U6 | **Yes** |
| **U7b** | ✅ **DONE.** **The reservation ceiling.** NEW SLICE, added 2026-08-15 at the owner's request; runs immediately after U7. Cap total reservation on a pool at 50-75% of its max and **refuse the breaching action** rather than letting it succeed and clamp: wielding or equipping a reserving item, enchanting, summoning, conjuring, raising. **Companions are in scope** and are the reason this cannot wait: they reserve on prod today. Also re-examines five raw-max reads whose U7-review rejection rested on the false premise that mobs cannot reserve. **Must precede U8**, which is what turns an over-reserved pool from cosmetic into crippling. | M | U7 | **Yes** |
| **U8** | ✅ **MERGED 2026-08-18 as `15a5fc94d` (PR #51).** **Unified action-cost admission.** Shoot, reload, taunt, rally, warcry, sneak, throw, grapple initiation and the full bash/trip/kick/beast-move family use the U7 model. Voluntary actions pay in full or refuse; autoattack, defence, flee and grapple maintenance remain life-preserving partial-pay actions and omit skill when short. One non-mutating quote/commit seam owns admission with player/mob parity. Quell and defy use coordinated data-backed narration. Task 14 passed recurrence and bypass guards, the complete race suite, isolated boot, and the mandatory adversarial playtest matrix on the feature branch; no integration has occurred yet. | L | **U7b** | **Yes** |
| **U9** | ✅ **DONE 2026-08-19.** Progression layer. Pure `internal/progression` returns events; one applier fires them; melee, channel defences, spells and taunt all routed. Registry moved to `internal/actionspec` with a `Stat` override. `SpellData.PrimaryStat` made load-bearing and the 14 manifestation-school files corrected to charisma. **Five defects fixed**, four of them duplications nobody knew about: melee attack progression fired twice per round (measured 2/2/4 against an intended 1/1/2), melee DEFENCE progression fired once per defended swing on top of once per round, every spell cast double-rolled the caster's stat, and crit-received ran a flat rank-free chance. Faucet closed on both halves: vitality had **zero** use-tracking anywhere, so its rank never moved. Spec: [`2026-08-19-u9-progression-layer-design`](../superpowers/specs/2026-08-19-u9-progression-layer-design.md). | M | U6 | **Yes** |
| **U6b** | ✅ **DONE 2026-08-19 on `feature/u6b-finish-the-flip` (Tasks 1-19; the Task 21 adversarial playtest gate precedes handoff).** The flip is finished: every attack channel — melee, ranged, spell, taunt/social, all 16 special moves, throw, steal/plant, sneak/shadow detection, flee and the grapple family — resolves through ONE seam. `combat.ResolveChannelAttack` + caller-supplied `AttackSide` run one contest per action against the channel's equipment-gated defence set (`DefenceEntriesFor`; melee keeps its scoring loop but consumes the same name builder). ONE crit mechanism everywhere: `CritBarFor(atkRank, defRank)` pairs the attack skill against the WINNING defence's skill (slope/floor/ceiling knobs), so the 16 attacks that could never crit now can, and the sleeping-victim ForceCrit reaches every channel. The counterattack tier is channel-wide and REACH-gated (`ExecuteCounter` + `CounterDamagePercent`; the cross-room shot stays the one uncounterable attack; defy answers with a counter-taunt), with channel-correct narration pools (`counter-melee/ranged/quell/defy`). `SituationalAttackMult` unifies prone + depletion accuracy modifiers. Deleted for good: `SpellAttackSkillFactor`, `spellDefenseValue`, `rangedDefenseScore`, `RangedShieldDefenseBonus`, `SubSkillWeight`, `SubCritZThreshold`, `StealSkillMultiplier`, the `if !isCrit` defence skip, and the Accuracy/Blink stowaway flags. Guarded by the three Task 18 tests in `internal/combat/contest_site_guard_test.go` (see "Done when" 2) and a Task 19 statistical parity gate (200k live-dice swings within ±0.4% of the analytic anchor). U7/U8 costs and U9 progression ride the seam. Spec: [`2026-08-19-u6b-finish-the-flip-design`](../superpowers/specs/2026-08-19-u6b-finish-the-flip-design.md). | L | U9 | **Yes** |
| **U10** | ✅ **SHIPPED 2026-08-21.** **Disruption model.** Concentration becomes a proper contest (`combat.RunConcentrationContest`, damage/position/throttle triggers, `ConcentrationFloor` 0.02); knockdown and prone recovery become opposed rolls. Throttle's cast interrupt claimed as a fourth concentration trigger. Knockdown shipped as a named rebalance (old `*KnockdownChance` thresholds delivered ~50/91/2.3% despite the shipped yaml claiming 50/60/35 — bash was accidentally correct, trip and kick were not; the new `*KnockdownFactor` contest delivers the intended rates). Surprise-attack redesign split out as **U10d**. See the "Outcome" annotation under the U10 write-up below. | M | U1, U0 | **Yes** |
| **U10b** | ✅ **DONE 2026-08-29, across FIVE sub-slices** — U10b-0 (rank from training, PRs #55–#60), **U10b-1** firing convention (PR #70), **U10b-1b** resolution onto the core (PR #74), **U10b-2** deferred faucets (PR #75), **U10b-3** difficulty from progression to discovery (PR #76). There is no single U10b merge; this row is the parent. **Progression firing consistency.** Added 2026-08-19. Progression fires under **8 to 10 different conditions** today with no convention (audit: [`2026-08-19-progression-firing-audit`](../audits/2026-08-19-progression-firing-audit.md), 135 sites across 52 files). Adopt one rule: **one event per success, with crit and critical-failure as a separate bonus on top**. Also routes Category C (crafting, salvage, forage) through the U9 seam, and reconciles the melee-versus-channel defence divergence: melee awards a defence only when one registered, the channel path awards win or lose. U9 changed what events CARRY and deliberately changed none of these firing conditions. **Note (2026-08-21): U10's new/claimed sites (concentration's three original triggers plus throttle, knockdown's defender resist, prone recovery) already ship on the success-only half of this convention; the crit / critical-failure bonus layer for those same sites is still owed here.** **Note (2026-08-25, from the U10d spec): the SKULLDUGGERY / stealth family is unclaimed by any slice and belongs here. Skullduggery has 17 progression sites and NONE is on the U9 seam — U9 routed melee, channel defences, spells and taunt; Category C above is crafting/salvage/forage. U10d converts exactly one (its own, in the file it deletes). The remaining **16** are bare `OnSkillUse` / `CheckSkillProgression` calls: `actions/steal.go` x3, `actions/plant.go` x3, `actions/shadow.go` x2, `usercommands/skill.skullduggery.sneak.go` x2, `usercommands/picklock.go` x2, `actions/defuse.go`, `usercommands/throw.go`, `mobcommands/flee.go`, `hooks/NewRound_DoCombat_helpers.go`. Beware the naming collision: **U10b-0** (rank-from-training) shipped 2026-08-24 as `d29996d4d`; THIS row has never been started.** **Also assigned here (owner, 2026-08-25): RANGED-COMBAT progression. Its only ordinary award is a bare `OnSkillUse` in the PLAYER wrapper (`usercommands/shoot.go:199`), so **mob archers earn no ranged-combat progression at all**. Not a free fix: giving it to them makes mob archers scale over an instance's life (gated by `MobSkillTrainingCap`), which is a live difficulty change across every archer mob at once. Deliberately kept out of U10d so that slice's playtest is not disentangling ambush feel from archer-mob scaling.** | M | U9, U10 | **Yes** |
| **U10c** | ✅ **SHIPPED 2026-08-24.** **Charm redesign.** Delivered in four slices (A `9b8fa2d51`, B `41d50717b`, C `d324507d3`, D). Charm was never outside the seam — the premise this row was written on was wrong. `spellAttackChannel` maps an ABSENT `target_defense_type` to `ChannelSpellMental`, so charm was routed all along; what escaped was that the seam's verdict got **discarded** and a second private `RunContest` in `resolveCharmSpell` decided the outcome, resolving one cast twice and narrating both. Charm now reads the `ChannelSocial` contest its cast already runs. **Defence stat decided: defy** (Willpower + rhetoric), not Charisma — the attack side is Charisma, so the defence must not be. The skill weight of 25 is gone with the hand-built score. The re-roll ladder is deleted and duration is bought once with the MARGIN of the winning contest, between `CharmDurationMinRounds` and `CharmDurationMaxRounds`; the player is never told how long. Expiry is a grudge gated on the owner being present and not link-dead. Slice D also restored the in-combat penalty slice B had deleted against spec 4.1, and added the `EverCharmed` instance-save guard so an ex-companion cannot bake its owner's gear into a world mob. | M | U9 | **Yes** |
| **U10d** | ✅ **SHIPPED 2026-08-25.** **Surprise-attack redesign.** Split from U10 2026-08-21 (owner). Delivered in **49 commits** on `feature/u10d-surprise-attack-redesign`, merged as **PR #69** (`067c26b60`). Spec + plan written 2026-08-25 after three blind adversarial review rounds: [`spec`](../superpowers/specs/2026-08-25-u10d-surprise-attack-redesign-design.md) · [`plan`](../superpowers/plans/2026-08-25-u10d-surprise-attack-redesign.md). **Shipped shape: ONE contested opening strike**, flagged by `Aggro.Type == SurpriseAttack` and consumed once (`openingStrikeLeft` in `calculateCombat`); a clean win crits and stacks `CritDamageMultiplier(skullduggery)` × `SurpriseOpeningStrikeMultiplier`. `actions.SurpriseAttack` and its 389-line uncontested multi-weapon volley are **deleted**. Stealth breaks immediately, implemented by **deleting** the `Awareness_Cascades` branch that preserved `Hidden`; the whole `SurpriseLeft` / `OnCombatRoundEnd` / `OnEndOfRoundIfSurprise` surface went with it rather than being repaired. Same-room ranged shot via `AttackSide.CritOnWin` + `SurpriseRangedStrikeMultiplier`, revealing the shooter explicitly and burning the shared `special-move` cooldown; cross-room shots stay ordinary. The two crit-on-win paths are pinned equivalent by `TestCritOnWin_MeleeAndChannelAgree`. Skullduggery moved onto the U9 seam, once per surprise round/shot on a clean hit, carried out on `AttackResult.WasSurpriseAttack` because `Aggro.Type` is demoted before `applyCombatProgression` runs. `EngageAggroType` now returns `(aggroType, surpriseOnCooldown)` so a **refused melee** opener speaks, matching the ranged one. Also folds in a **ranged economy rebalance**: eight bow/crossbow/sling/firearm multipliers detuned onto the melee scale (top bow 7.50 → **2.75**), compensated by `RangedUnengagedDamageMultiplier` **2.75** when nothing in the room targets the shooter, resolved by a **room scan** (see the `Attackers()` finding in U11). Player saves are rescaled by an unmarked, value-guarded per-load migration reaching backpack, component bag, bandolier, pet inventory, equipment (`characters.MigrateDetunedRangedWeapons`) and the account bank (`users.Storage.MigrateDetunedRangedWeapons`); world state (mob instances, shop stock, room containers) is deliberately **not** swept and decays instead. Three knobs added (`SurpriseOpeningStrikeMultiplier` 1.0, `SurpriseRangedStrikeMultiplier` 0.5, `RangedUnengagedDamageMultiplier` 2.75), five deleted (the `SurpriseAttack*Penalty` family, inert at 0.0). New `help ambush` topic, registered in `keywords.yaml` under `combat` with aliases (surprise attack, backstab, sneak attack …) and `help hide` now aliased to `sneak`. **Adversarial playtest DONE 2026-08-25** (bug-finder, ephemeral local run `68affe820bb1164e`), satisfying the content SOP. Mechanics all PASS: ONE `*[SURPRISE ATTACK]*` line per round verified on a **three-weapon-slot** character (the hardest case for the one-blow scope decision), the ranged reveal fires, `reload` genuinely spends the shared recovery (confirmed from the cooldown table, not by timing), and the `ambush` help renders 83 lines with zero over 80 columns, zero dashes and no raw numbers. 🔴 **The damage-band vocabulary SATURATES: an ordinary shot and an ambush shot both print `(devastating wounds)`, so the deliberate ordering of the three new multipliers is imperceptible to a player.** Tuning follow-up, not a correctness bug, and measured only against a target that dies in one round. **Four checks went UNVERIFIED** because that fixture mob was far too weak for the `veteran` profile — a **defended** opening strike, the in-game denial copy, whether the unengaged bonus is felt, and the migration on an already-owned bow — all recorded on the owner punch list for test day; each needs a mob that survives a round. One copy defect was found and fixed (`16300f7bf`): the patch notes promised a melee reveal sentence that `sendMeleeAmbushDenial` deliberately never sends on the success path. U11 remains the arc's closing gate. **🔴 Four defects found by review that would have shipped SILENTLY:** `combatphase.SurpriseLeft` has never been true in production (`TransitionToEngaging` drops its `TransitionReason`) so the boundary is DELETED not repaired; `Character.Attackers()` is always empty (`RegisterMachine` has no production callers) so the unengaged bonus needed a room scan instead; a round-scoped `critOnWin` at `combat.go:466` would have reinstated the retired every-swing design; and `applyCombatProgression` runs after `Aggro.Type` is demoted, so the surprise flag is carried out on `AttackResult`. See the spec sections 1.1 and 2.8.3 for the evidence. | M | U1, U0 | **Yes** |
| **U12** | ✅ **DONE 2026-08-29, across SIX slices.** U12a (PR #83) · U12b (#84/#85) · U12c-0 + U12c-0b (`8f6ce50c0`, `7b041f995`) · **U12c-1 (#86)** · **U12c-2 part one (#87)** · **U12c-2 part two (#89)**. There is no single U12 merge; this row is the parent. **`Character.Aggro` is DELETED**, along with the `Aggro` struct, both accessor fallbacks and `combat_state_compat.go` (renamed `engagement_storage.go`). Everything `AggroType` was doing moved to the machine that models it: `Flee`→`Disengaging`, `SpellCast`→`activity.CastingData`, `Shooting`→derived from the equipped weapon, `SurpriseAttack`→`CombatPhase.OpeningUnspent`, `RoundsWaiting`→`CombatPhase` with the required two-counter note. `ConsumeOpeningStrike` finally has its production caller. ⚠️ **DEVIATION from spec §6.3.6: `AggroType` and `SpellAggroInfo` SURVIVE** as call PARAMETERS (the kind of engagement a commit is starting; a cast's aim into `SetCast`). Neither is stored state any more, which was the actual problem; eliminating them means replacing `AggroType` with a combatphase trigger at every `SetAggro` call site, which is its own slice. ⚠️ **`SetAggro`/`EndAggro` also SURVIVE** as the storage primitives — spec §5 said to delete them and was WRONG; §6.3.7 records the correction. The enforced rule is a CALLER restriction. 🔴 **The adversarial playtest found a false patch-note claim I had already written** (ambush follow-up pacing; no DOGMud weapon sets `waitrounds` and `phase1WaitRound` guarantees the budget is 0, so there was no change — retracted in `4edde2443`) **and a one-day-old flee regression on master** from U12c-0/0b (a retarget lands in `Engaging`, which had no edge to `Disengaging`, so anything retargeting you every round blocked `flee` forever; shipped separately as **#88**). It also cleared a U10d UNVERIFIED check (the defended ambush). 🪤 **Both hid because every practice target in the game died in ONE round**; the Drill Yard now carries the tutorial's unkillable Straw Effigy (`227e96088`). 📌 **A site NO GUARD COULD SEE:** `world.go` at the repo ROOT — the U12c-1 and U12c-2 guards walk `internal/` only. Found by the compiler. |
| **U11** | Docs, `context.md` sweep, **`config.yaml` organisation audit**, **player helpfiles for `quell` and `defy` plus the help-registry and category cleanup**, and the adversarial playtest gate. **Runs LAST, after U12** — the gate is the arc's closer, so no code slice may land after it. **U11 must also ship the "Done when" list below AS A TEST.** U6 was declared done in 2026-08 with two of its criteria false, and because they were prose in a roadmap nothing failed; the gap survived three further slices before U9 tripped over it. U6b already expressed criterion 2 as tests (the Task 18 guards in `contest_site_guard_test.go` — see the annotation under "Done when"); U11 keeps the obligation for the remaining criteria. **Six defects handed to U11 by U10d's reviews (2026-08-25), none of them about surprise attack — see the "U11 inbox from U10d" list below the plan table.** | M | U6b, U8–U10d, U12 | — |

### U11 inbox from U10d — six findings, FIVE still open

Five of the six are not about surprise attack, and **one (#6) is now RESOLVED by
U12c-2**. Of the five that remain, #3 is a correction (do NOT act on it) and #2 is
a recorded pattern rather than a bug, so **U11 has THREE things to actually do
here: #1, #4 and #5.**

U10d's blind reviews turned up defects in code the slice did not touch. They are
recorded here so they meet a reader, and each was re-verified rather than carried
over on the review's word. Two of the six did **not** survive that check and are
written down as corrections, because a false entry in a permanent record is worse
than a missing one.

**RE-VERIFIED AGAINST MASTER 2026-08-29, after U12 closed.** Every path, line
number and count below was re-checked at that point. Four entries needed
correcting and one is now RESOLVED; the corrections are marked inline so a reader
can see what moved rather than trusting a five-day-old list. U12c-2 renamed
`internal/characters/combat_state_compat.go` to `engagement_storage.go`, which is
the source of most of the drift.

1. **`combatphase.RegisterMachine` has ZERO production callers, so
   `Character.Attackers()` is always empty — and prone recovery has never been
   contested.** `RegisterMachine` is defined in all five `internal/state/*`
   machines (`activity.go:154`, `awareness.go:124`, `combatphase.go:182`,
   `life.go:121`, `position.go:277`) and every call site repo-wide is in a
   `_test.go`. `internal/characters/validate.go` builds the machines directly
   and never registers them, so `machineRegistry` is empty, `lookupMachine`
   always returns nil, and the single `RecordInboundAttacker` call site
   (`combatphase.go:234`, behind `if target := lookupMachine(d.Target); target
   != nil`) never runs. `Character.Attackers()`
   (`internal/characters/character.go:799-805`) nevertheless promises it
   *"replaces room-scan loops for 'who's attacking me?'"*. **It has an existing
   silently-inert consumer:** `recoveryContest`
   (`internal/hooks/recovery_contest.go:23`) iterates `ch.Attackers()`, finds
   nothing, returns `nil`, and `nil` is documented in that function as *a free
   stand* — so prone recovery is uncontested for everyone, always, whatever U10
   intended. `CombatPhase_CompanionAssist.go`'s reactive
   `SubscribeAttackersChange` path is likewise dead (companion assist runs on
   the polling fallback), as are `NotifyTargetDied` / `NotifySelfDied`.
   **A second, independent break sits behind the first:** `SetAggro` passes
   `Actor: state.ActorRef{UserId: c.userId}`
   (**`internal/characters/engagement_storage.go:167`** — the file was
   `combat_state_compat.go:146` until U12c-2 renamed it) and nothing ever calls
   `SetUserId` on a mob, so a mob's ref is the zero value and
   `RecordInboundAttacker` early-returns on `ActorRef.IsZero()`. **Even a
   repaired registry would never record a mob attacker.** U10d dodged this by
   resolving its unengaged-ranged rule with a live room scan
   (`actions/combat_fire.go`, `shooterIsUnengaged`) rather than `Attackers()`.
   **U11 decision needed: wire it up or delete it.** It must not keep sitting
   in the tree looking usable. Documented at the code in
   `internal/state/combatphase/context.md` and
   `internal/state/awareness/context.md` (the awareness twin has the same gap).

2. **Alt characters are a real hazard for Character-scoped migrations — and
   U10d's migration deliberately avoids it. NOT a live bug; recorded as the
   pattern.** The architecture claim checks out: alts persist as
   `<userId>.alts.yaml` (`internal/users/character_index.go:78-79`), each with
   its own `MiscData`, while `ItemStorage` lives on `UserRecord`
   (`internal/users/userrecord.go:46`) — one bank per **account**. A
   Character-scoped run-once marker would therefore let the shared bank be
   migrated once per alt. But the review's claim that U10d's bow detune does
   this is **backwards**: both `Storage.MigrateDetunedRangedWeapons`
   (`internal/users/storage_migrate.go:40`) and
   `Character.MigrateDetunedRangedWeapons`
   (`internal/characters/migrate_detuned_bows.go:64`) carry **no marker at all**,
   for exactly this reason, and correctness rests on per-item idempotence
   (`item.DetuneMigrated` plus a value guard) instead. `MigrateEnchantments`
   (`internal/characters/migrate_enchantments.go`) is likewise unguarded and
   idempotent by construction through `enchantments.ApplyTier`. **That is the
   pattern to copy for the next save migration: make it idempotent, not
   marked.** Recorded as a gotcha in `internal/users/context.md`.

3. **CORRECTION — `SubGoldLossFraction` is NOT pinned at 0.0.** The review
   reported it inert, reasoning that the validator
   (`config.balance.combat.go:222-224`, `if x < 0 || x > 1.0 { x = 0.20 }`)
   cannot fire on the zero an absent key unmarshals to. The validator shape is
   real, but the premise is not: `sub_gold_loss_fraction: 0.20` **is present**
   in `_datafiles/config.yaml:925`, under `Balance:`, matching the field's
   `yaml:"sub_gold_loss_fraction"` tag. The subdue/cripple gold transfer
   (`internal/combat/submission_outcome.go:201`) runs at the intended 0.20. **Do
   not "fix" it.** What IS worth keeping is the underlying trap, which has
   already cost the project once: **a knob with a non-zero advertised default,
   a `< 0 || > 1.0`-shaped validator, and no key in `config.yaml` is silently
   inert at zero** — that is precisely how the five `SurpriseAttack*Penalty`
   knobs U10d deleted came to auto-hit every limb. A sweep of the remaining
   instances of that validator shape found no other live case
   (`MinAttackCritChance` / `MinDefenseCritChance` treat 0 as a deliberate
   off-switch and are written explicitly; `EquipmentDropChance`'s coded default
   IS 0.0). **U11's config audit should still sweep for the pattern**, since the
   sweep above covered only that one validator shape.

4. **`u8ActionHelpPaths` is a hand-maintained allowlist, and it is the wrong
   way round.** `internal/templates/u8_help_test.go` lists **28** paths (re-counted
   2026-08-29; was 27 at U10d, 26 before it added `help/ambush`) against **454**
   files in `_datafiles/world/dogmud/templates/help/`. **FOUR** tests iterate only
   that list, not three — `TestU8ActionAdmissionHelpTemplatesProcess`,
   `TestU8ActionAdmissionHelpStatesExactPolicyWithoutTuning`,
   `TestU8ActionHelpCrossReferencesResolve` — so **426 help templates get no
   parse check, no numeric-disclosure check and no cross-reference check at
   all.** Both files U10d edited that fell outside the list were exactly where
   its surviving copy defects lived. Structurally this is the same failure as
   `stow` going invisible in the 2026-08-03 helpfile audit. **The fix is to
   invert it:** walk every `help/*.template` and keep a shrinking, commented
   exception list instead. Sits naturally with U11's help-registry cleanup.

5. **Two stale special-move cooldown comments that disagree with each other and
   with the code.** `internal/configs/config.balance.go:246` reads
   `// Shared cooldown rounds for bash/trip/kick (default 5)`;
   `_datafiles/config.yaml:660-664` reads *"Bash, trip, and kick all share a
   cooldown … Spellcasting also shares this cooldown slot … Example: 4 = after a
   bash, must wait 4 rounds before bash/trip/kick/cast."* Neither is right:
   **46 non-test `.go` files under `internal/` and `modules/` reference the
   `special-move` key** (re-counted 2026-08-29; was 44 at U10d. `grep -rl
   "special-move" internal/ modules/ --include=*.go | grep -v _test.go`), and U10d added the melee and ranged
   ambush openers to that population. The `config.yaml` comment is the worse of
   the two because it is what a tuner reads. **Left unfixed on purpose:**
   `config.yaml` has `skip-worktree`, so editing it needs the
   commit-from-`git show HEAD:` procedure, and fixing only the Go half would
   leave the pair still disagreeing. Fix both together in U11's config audit.

6. ✅ **RESOLVED by U12c-2 (PR #89) — `TransitionToEngaging` no longer drops its
   `TransitionReason`, and the field it failed to populate is gone.**

   The original finding: `TransitionToEngaging` took `(d EngagingData, r
   state.TransitionReason)` and did `m.engaging = &d`, never copying `r` into
   `d.Reason`. That is what made `SurpriseLeft` false in production for its
   entire life. U10d deleted the only consumer, leaving it latent, and recorded
   that any future consumer would hit it.

   Both halves are now closed, and NOT by repairing the copy:

   - **`EngagingData.Reason` is DELETED.** It had zero readers and the one
     production construction site never set it, so U12c-2 settled U10d's
     deferred either/or with the second branch: it was dead, so it went. There
     is no longer a field for a future consumer to hit.
   - **The `r` PARAMETER is now genuinely live.** `combatphase.go:329` reads
     `r.Trigger == TriggerSurpriseAttack` to arm the ambush opening, and
     `:368` reads it to gate the flee veto. Keying on the trigger rather than
     on stored state is what let the opening survive a retarget correctly.

   ⚠️ **Do not "restore" the Reason copy.** The struct comment at
   `EngagingData` warns against repurposing either the deleted field or the
   live parameter as a home for an engagement-kind enum: that moves the
   demotion bug U12c-2 removed rather than killing it.

   **No U11 action required.** Kept here rather than deleted so a reader who
   remembers the finding can see how it closed.
---

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
- **The defence affordability gate drops an exhausted defender out of the
  contest entirely.** `combat_helpers.go` `continue`d any defence the character
  could not pay for out of the best-of-N candidate set. With every defence
  unaffordable the entry list came out empty, `contest.Run` reported
  uncontested, and the swing fell through to the `MinDefenseChance` last resort:
  a flat 15% save, always narrated as a **dodge**, and never able to
  defence-crit (`crit_floor.go` returns early when `defenseType == ""`).
  **CORRECTED 2026-08-13:** an earlier draft of this bullet claimed
  `DeductDefenseStamina` returned a bool that the call site discarded, so "a
  defence the character cannot pay for still wins the best-of-N". That was never
  a live bug. Candidates were gated *before* the contest, so the winner was
  affordable by construction and that return value was unreachable-false.
  U5b-2 removed the gate and charges the winner with `ApplyCostPartial`.

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
   all of it. If the actor cannot pay, that is a separate decision: U8 refuses
   voluntary actions, while life-preserving autoattack, defence, flee and
   grapple maintenance still resolve without their skill term. Neither policy
   permits overdraw.
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

**RESOLVED by U7: have less, not pay more.** `Character.EffectivePoolMax(pool)`
returns `max - reservation` and is used for percentage-of-max thresholds and
ratio computations. Costs and affordability checks must NEVER use it, because
`RecalculateStats` already clamps the current value, so a cost reading the
effective max would subtract the reservation twice. Regeneration deliberately
still reads the raw max.

**ACCEPTED, not a bug: encumbrance barely matters outside combat.** The
2026-08-15 playtest measured out-of-combat stamina regen at about +8 per round
against a worst-case move of 3, so a crushed traveller GAINS stamina by walking.
Closing that would mean cutting out-of-combat regen or making travel much
dearer, and both pull against the deliberate decision to make ordinary travel
cheaper than it was. **Owner call: accept it.** Stamina is a combat resource
whose travel dimension is flavour. Do not re-file this as a defect.

Two things WERE fixed off the back of that run: movement now banks its
fractional remainder like every other priced action, instead of ceiling each
move independently and flattening a 5x multiplier into a single step; and
`flee` was brought onto the cost model, having been a flat charge with no
encumbrance term, so escaping fully laden cost exactly what escaping empty did.

### U7b — the reservation ceiling

**Added 2026-08-15 at the owner's request**, immediately after U7 and before U8.

**Scope.** Total reservation on a pool is capped at somewhere between 50% and
75% of that pool's max, to be modelled rather than guessed, and any action that
would push past the cap is **REFUSED** rather than allowed to succeed and clamp.
Refusal applies at the breaching action: wielding or equipping a reserving item,
enchanting, summoning, conjuring, raising. Players and NPCs alike.

**Why it cannot wait, and why it is not only a player concern.** Companions
reserve on prod today: hand one enchanted items and the reserved portion shows
in its resource bars. `Character.GetPoolReservation` iterates equipment with **no
`IsMob` gate whatsoever**, so any character with reserving gear reserves. Two
consequences:

1. **Companion reservation must be clamped too**, and the mob equip-if-better
   path must handle a refusal gracefully. `Character.Wear` already returns a
   `failureReason`; `rooms.go` and `bountyhunter.go` check it, but
   `internal/actions/sell.go` discards it and would fail silently. Note
   `give.go` queues `GiftAccepted` **before** the equip decision, so a decline
   currently reads as accept-then-return.
2. **FOUR raw-max reads must move to `EffectivePoolMax`, not six.** The U7-review
   rejection rested on "mobs cannot carry a reservation", which is false for
   companions: Meirok's two golems wear enchanted gear reserving their own
   health and conviction, on the live save. Genuinely affected, all self-side:
   the three reads in `internal/combat/ai.go` (`ScoreGrapple`, `ScoreDrain`,
   `preferredSpell`) and `hpPercent` in `behaviortree/actions_archer.go`.
   **NOT affected:** `behaviortree/conditions_mob.go` and
   `action_cast_best_in_category.go` both go through
   `mobs.FindPackmatesInRoom`, which **skips charmed mobs**
   (`internal/mobs/packmates.go:42`), and every companion is charmed. Correct
   their comments rather than their code. Magnitude scales with the cap: at a
   66% cap a fully geared companion would read as permanently at 34% health.

**The cap cannot be enforced only on the adding action.** Research 2026-08-15
found several ways reservation rises with **no player action to refuse**, and
the first is not an edge case:

- **Enchant tier-up.** `internal/hooks/NewRound_UserRoundTick.go` rolls
  `EnchantTierUpBaseChance` in combat and increments the tier, which **doubles
  the reserve fraction at low tiers**. A character at 74% can cross the cap
  mid-fight having done nothing. The only coherent design is to skip the
  tier-up when it would breach and say why. This was named nowhere in the
  original spec.
- `ConditionEnchantWithdrawal` shrinks the pool max *after* the reservation
  clamp, so it raises the reservation share without touching the numerator.
- `BodyConvictionScale` deepens as body-pole mutations accrue. Live on Meirok
  today at 0.894.
- `MigrateEnchantments` on login re-applies enchantment definitions, so
  retuning any tier's `reserve_pct` grows existing reservations at load.
- `companion_reserve_backfill.go` stamps a reserve on legacy zero-reserve
  companions at login with no budget check at all.

**`CanAffordCompanion` is weaker than the spec claimed.**
`CompanionCastingFloorPct` defaults to 0.0 and is absent from `config.yaml`, so
the check reduces to "total conviction reservation must not exceed the max",
i.e. a **100% cap, not a partial budget**, and conviction only. Health and
stamina have **no gate anywhere**. Two spawn paths (brood-mother, homunculus)
never call it at all.

**Why it must precede U8.** U8's skill-strip on insufficient resource is what
turns an over-reserved pool from cosmetic into crippling. Shipping the strip
without the ceiling makes a zero-reserved pool a permanent triple-digit defence
penalty.

**Related U7 guard, already shipped.** `EffectivePoolMax` floors at 1 rather
than 0, because every consumer reads a zero max as "no penalty at all", which
made total reservation the *strongest* state in the game on five separate
curves. That floor is belt and braces; this ceiling is the real fix.

#### Outcome (shipped 2026-08-15)

The cap landed at **66%** (`PoolReservationCapPct`, absent from `config.yaml`
and falling through to its Go default), per pool, on all three pools, for
players and companions alike. `internal/characters/reservation.go` owns the
whole surface: `ReservationCap`, `WouldBreachReservationCap`,
`ReservationOverages` / `ReservationSnapshot.Worsened`, `ItemReserveOnPool`,
`EnchantReserveAt`, `ReserveShareBand`, `ReservationBandName` and
`ReservationRefusal`. Eight enforcement sites consult it: equip
(`characters.Wear`), the enchanting pre-flight in `craft.go`, the enchanting
craft completion, the combat tier-up roll, `resolveCompanionSummon` (summon,
conjure and raise alike), `resolveCharmSpell`, and the brood-mother and
homunculus auto-spawns. `CanAffordCompanion` is deleted; with
`CompanionCastingFloorPct` at 0 it had reduced to a 100% cap on conviction
alone. Grandfathering is delivered by an overage snapshot rather than a cap
test, so a character already over the ceiling can still swap one reserving item
for another of the same weight.

Companion power was rebuilt alongside it. `CalcCompanionPool` is
`(charisma + manifestation x 5)`, averaged with the corpse pool for raises, with
the pet multiplier applied **after** the average so the tiers stay
proportionally separated at every corpse size. `summon_pet_multiplier` replaced
`summon_base_pool`, and `summon_scaling_divisor` and
`summon_conviction_reserve` are gone: reservation is now derived as
`round(CompanionReserveDefault x petMultiplier)` and is never authored. Thirteen
summon spells were re-costed and five summon mobs had behaviour defects fixed.
`refreshCompanionReserves` replaced the zero-only login backfill with a full
recompute, and discloses it when the result leaves the owner further over.

**Three places this extended the spec, all deliberate.**

1. **`summon_conviction_reserve` was deleted rather than kept as an override.**
   D12 named only the other two, but with reservation derived from the pet
   multiplier an authorable value beside it is a second source of truth that
   drifts on the first retune. If per-spell overrides are wanted later, that is
   a field to add back with a documented precedence rule.
2. **`CalcCompanionStatPool` was renamed and kept, not deleted.** Its other
   caller is `behaviortree.actSummonCompanion`, which spawns authored boss adds
   tuned against its exact curve. It is now `CalcSpawnPoolFromBase`, and
   `ManifestStatScaleChaFactor` / `ManifestStatScaleSkillFactor` stay alive to
   serve it.
3. **`CompanionReserveDefault` kept its name.** The spec called the knob
   `CompanionReserveBase`; no such field ever existed. The new *function* is
   named `CompanionReserveBase`, which is where the spec's word landed.

**Two carried concerns.** `HomunculusConvictionReserve` dropped from 1000 to
**300**, because at 1000 the ceiling made a crafting apex unfieldable by exactly
the crafter it was built for. 300 still needs roughly 455 conviction max to fit
under a 66% cap, and nearer 500 once the rank-0 rider penalty applies, so the
refusal is **spoken** rather than silent and crib-sheet check 10b targets it.
Separately, the inverse-skill rider is a penalty at low rank on **both** the
item and the companion side, so a character who is a novice at enchanting and
manifestation both pays it twice on two different pools. Each half is settled;
the compounding is not, and is a playtest question rather than a code change.

### U8 — unified action-cost admission

**Designed 2026-08-17.** Source of truth:
[`2026-08-17-u8-unified-action-cost-admission-design.md`](../superpowers/specs/2026-08-17-u8-unified-action-cost-admission-design.md).

**Status 2026-08-18:** implementation and validation complete on the feature
branch; integration pending. Tasks 1 through 14 passed there, including full
recurrence and bypass guards, the complete race suite, isolated boot, and the
required adversarial in-game matrix. The live gate also drove regression fixes
for coalesced Telnet login input, terminal flee-admission races, and
out-of-combat flee during a pending cast. No merge or other integration into
`master` has occurred yet.

U8 is both a new cost surface and the consolidation slice for action admission.
It extends the existing U7 action registry and calculator through one
non-mutating quote followed by an explicit full or partial commit. It does
**not** put pool mutation into `combat.RunContest`, and no caller may reproduce
the cost formula.

**Full-pay or refuse:** shoot, reload, taunt, rally, warcry, sneak, throw,
grapple initiation, bash, trip, kick, hamstring, rake, maul, pounce, gore,
drain and throttle. Refusal happens after read-only validity gates but before
cooldown, ammunition, item use, round consumption, awareness transition or
effect.

**Partial-pay and resolve without skill when short:** autoattack, the selected
defence, flee, and both sides of grapple maintenance. These are the actions
whose refusal leaves an exhausted actor helpless rather than merely choosing a
different move. Quell and defy already charge Conviction through U7; U8 only
adds their insufficient-resource consequence.

Grapple maintenance joins the registry as physical + Unarmed Combat. Preserve
the controller/controlled cost ratio by applying its existing role multiplier
to the base before the U7 product clamp. Either participant can independently
lose its skill term in the drift contest.

Quell and defy also close their disclosed narration gap in U8. Add
`defense-messages/quell.yaml` and `defy.yaml` on the existing weak/normal/heavy
shape, with at least five coordinated defender/attacker/room variants per band,
and route the live spell and social paths through them. This is content parity,
not the broader deferred combat-message unification.

Reload is deliberately included: it is a physical Ranged Combat action, and
loading a weapon is tiring even though its outcome is deterministic. `charge`
inherits trip and `howl` inherits taunt; aliases never receive duplicate cost
entries.

**Balance gate.** A cooldown-gated physical manoeuvre should cost modestly more
than one ordinary swing but no more than four ordinary swings for the same
typical character, load and skill. Model ranged shoot-plus-reload cycles,
rhetoric sustainability and grapple's combined cost/effectiveness pressure
before choosing shipped values.

**Documentation is not deferred to U11.** U8 updates affected helpfiles,
every touched package's `context.md`, config comments, patch notes and this
roadmap, then runs the adversarial in-game playtest required for new
player-facing copy.

**Recorded follow-up, out of scope:** audit and remove dead mutation-active command
registrations, implementations, tests, help and config for dead remnants after
the mutation removals. Surprise attack remains U10d's redesign (split out of
U10 2026-08-21).

**Recorded follow-up, out of scope:** unify fragmented combat and action
messaging after the resolution arc. U8 moves quell and defy onto the existing
defence-message data shape but does not redesign broader message ownership.

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

#### Outcome (SHIPPED 2026-08-21)

Landed as planned, with the lattice difficulty table re-ratified over the
corrected knockdown numbers rather than the original 250-value estimate: the
per-(position, control-role) lattice from
`position.PositionDisruptionDmgEquiv` is fed into
`combat.RunConcentrationContest` at **×10 as-is**, which puts prone at
**300**, harsher than this section's original 250 estimate, and deep grapple
holds at 600-700. Damage-path trigger threshold shipped at **10%**
(`ConcentrationDamageThresholdPct`) exactly as specced: chip damage never
rolls at all. `ConcentrationFloor` shipped at **0.02**, the ~none option this
section called for, as the one place that floor is read
(`internal/combat/run_concentration_contest.go`). `CalcConcentrationChance`,
`SpellConcentrationBase`, `SpellInitiationWillpowerDivisor` are deleted.

**Throttle claimed.** The unowned-sites sweep (`actions/combat_throttle.go`)
is now a fourth concentration trigger: an opposed contest through the same
`RunConcentrationContest` seam, live opposing score (throttler's grip)
instead of a static difficulty, replacing the flat `ThrottleInterruptChance`
coin flip.

**Knockdown shipped as a NAMED REBALANCE, not a straight migration.** The old
`*KnockdownChance` knobs (shipped yaml 50 / 60 / 35; the bash Go default
was 40) read as intended
percentages but were actually thresholds on a `Normal(50, 7.5)` roll, so the
true delivered rates were roughly **50% bash / 91% trip / 2.3% kick** — trip
was nearly guaranteed and kick was nearly dead. The opposed contest
(`p.Attack.score()×KnockdownFactor` vs. defender `Dex +
unarmed×SkillWeight`, standard `ContestFloor`) with `BashKnockdownFactor`
1.0 / `TripKnockdownFactor` 1.057 / `KickKnockdownFactor` 0.924 delivers the
originally-intended 50/60/35 shape at parity instead of the true old rates.

**Recovery shipped contested-or-free**, not contested-always: `AttemptRecovery`
takes an injected `contestWin func() bool`; the caller
(`internal/hooks/recovery_contest.go`) builds an opposed
`Dex+unarmed×SkillWeight` contest against the strongest living same-room
attacker when one exists, or passes `nil` for an automatic free stand when
nobody is holding the recoverer down. The manual `stand` command stays the
separate, deliberate **paid** exit: uncontested, spends stamina, always
succeeds.

**Progression at every new/claimed site is success-only**, per U10b's
convention adopted early: one event on a held/resisted/stood contest,
nothing on a loss. The crit / critical-failure bonus layer for these sites
is still owed and lands with U10b, not U10.

**Surprise-attack split out as U10d**, owner 2026-08-21 — see the row below.
Its redesign needs its own brainstorm → spec → plan cycle (decided
2026-08-13) and does not ship with U10.

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
- **Before U8** — shoot, reload, taunt, rally, warcry, sneak, throw, grapple
  initiation and the special-move family are free today. Giving them costs is a
  real nerf to playstyles that have never paid. Model the entire matrix,
  including shoot-plus-reload cycles, rhetoric sustainability and grapple
  maintenance. A cooldown-gated physical manoeuvre targets more than one but
  no more than four ordinary swings for the same typical actor, load and skill.

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

   > ✅ **MET as of U6b (2026-08-19) — and this time pinned by tests, not
   > prose.** `SpellAttackSkillFactor` is deleted outright, along with
   > `spellDefenseValue`, `rangedDefenseScore`, `RangedShieldDefenseBonus`,
   > `SubSkillWeight`, `SubCritZThreshold` and `StealSkillMultiplier`. The
   > U6b Task 18 guards in `internal/combat/contest_site_guard_test.go` hold
   > the criterion: `TestEveryChannelUsesUniformDefenceSkillWeight` asserts
   > the single `SkillWeight` on both sides of every channel,
   > `TestNoLegacySkillWeightLiteralSurvives` fails any reintroduced
   > per-channel multiplier literal, and `TestEveryContestSiteIsOwned` fails
   > any new `RunContest` call site that has not been claimed by an owner.
   > (Criterion 2 needs no further U11 test; the remaining criteria still do
   > — see the U11 row.)
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

## Pre-deploy manual playtest (owner, blocking deploy)

The AI-harness gate above is necessary but not sufficient: the owner runs a
manual pass with Meirok on the local server before anything deploys. The full
checklist lives in `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md` and grows as
U7-U12 land. Two items are called out here because they are MANDATORY, not
optional feel checks:

1. **The Elemental Queen (planar oasis instance, ~300g).** The 2026-08-15
   caster-brain fix made her actually cast for the first time: she now opens
   with chrysalis-cocoon, throws conviction-spike and conviction-barrage, and
   flees below 30% health. That is a real difficulty change to a PAID fight,
   and it was never playtested at authoring because the archetype was dead.
   Fight her with Meirok before deploy. The same fight is the designated
   live-verification instrument for QUELL (her conviction spells are mental,
   she has the pool to survive long enough to cast repeatedly, and unlike the
   bandit caster's Mind Fog her spells deal real damage, so quell costs and
   partial mitigation are observable).
2. **The arc's floor and ceiling at veteran power.** No previous session could
   observe the 12.5% bounds because trash dies in one round. The queen's
   gold-scaled pool is the instrument here too: in a ten-plus-round fight,
   Meirok should occasionally be stopped even while dominating, and she
   should occasionally get through even while losing.
