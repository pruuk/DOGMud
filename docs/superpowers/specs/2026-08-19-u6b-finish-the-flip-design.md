# U6b — Finish the Flip

**Created:** 2026-08-19
**Revised:** 2026-08-19, after a blind adversarial completeness audit found the
first draft's foundation wrong in the same way U6's was. See §12.
**Arc:** [`UNIFIED_RESOLUTION_ROADMAP.md`](../../roadmaps/UNIFIED_RESOLUTION_ROADMAP.md) (U0–U12)
**Parent spec:** [`2026-08-12-unified-contest-resolution-design.md`](2026-08-12-unified-contest-resolution-design.md)
**Depends on:** U9
**Blocks:** U11 (the arc's closer cannot run while its own criteria are false)
**Size:** XL
**Behaviour change:** Yes, the largest in the arc

---

## 1. Why this exists

U6 was "THE FLIP": uniform ×5 skill weight, multiplier defence, designed
defence sets, **all legacy parameters deleted**. Two of this arc's own "Done
when" criteria have been false ever since:

> 2. Defence skill weight is ×5 in every channel; `SpellAttackSkillFactor` is
>    gone from the attack path.

Both are prose in a roadmap, so nothing failed when they stopped being true.
The gap survived U7, U7b, U8 and most of U9.

The owner's framing is the requirement:

> "It says right on the tin UNIFY. melee/ranged/spell/taunt/special/non-combat
> all the same hit path, the same crit path, the same progression path, the
> same damage path (if applicable)."

U9 delivered the progression path for the contest channels. This delivers the
rest, **and it does not get to declare a channel converted without evidence** —
which is exactly what the first draft of this spec did (§12).

---

## 2. Measured state, 2026-08-19

**Verified in source. The first draft of this table was wrong about two rows
and is corrected here.** Treat every claim below as checkable and check it.

### 2.1 The five named channels

| Channel | Contests | Crit from | Defence set source | Crit bar | Atk skill | Def skill |
|---|---|---|---|---|---|---|
| **Melee** | 1 | contest margin | **its own** `GetDefenseSequence` | **dynamic** | ×5 | ×5 |
| **Taunt** | **2** | **the GATE's margin** | `DefenceSetFor`, **skipped on crit** | const 2.0 | ×5 | ×5 (contested **twice**) |
| **Spell** | **2** | **the GATE's margin** | `DefenceSetFor`, **skipped on crit** | const 2.0 | **×15** | **×0** on the gate |
| **Ranged** | 1 | **NO CRIT AT ALL** | **none**, flat scalar | — | ×1 | ×1 + flat shield |
| **Special ×14** | 1 | **NO CRIT AT ALL** | **none**, flat scalar | — | ×1 | ×1 |

**Not one channel is fully converted.** Melee is closest and still has its own
defence-set builder and its own crit bar.

Evidence for each non-obvious claim:

- **Taunt is a two-contest channel.** `actions/combat_taunt.go`: `runTauntContest`
  is the hit gate; `isCrit := combat.AttackContestCrit(res.Margin, ...)` takes
  crit from that gate; `if !isCrit { defence = combat.ResolveChannelDefence(
  combat.ChannelSocial, ...) }` runs defy only when the attack did not crit.
  Structurally identical to the spell defect. The defender's `Wil + rhetoric×5`
  is therefore contested **twice**, and the two contests do not even score the
  attacker identically — the gate applies a conviction-depletion multiplier
  that `ChannelAttackScore` omits.
- **Melee does not consume `DefenceSetFor`.** `combat/defence_sets.go` says so
  in its own doc comment, in capitals: "**MELEE DOES NOT**. `runBestOfAllDefense`
  still builds its own equipment-derived defSeq from
  `characters.GetDefenseSequence`."
- **Melee's crit bar is dynamic.** `combat/combat_helpers.go` `calcCritThreshold`:
  2.0 base, 1.5 with the Accuracy buff, 2.5 against Blink, then shifted by
  combat-skill difference with a 1.5 floor. Every other channel tests the const
  `ContestCritThreshold = 2.0` (`combat/margin_crit.go`). Melee's *defensive*
  bar is a separate hardcoded 2.0.
- **`ExecuteSkillMove` computes no crit.** `SkillMoveResult` is
  `{Hit, Damage, StatusApplied, KnockedDown, TargetMaxHP}`; `AttackContestCrit`
  appears in that file zero times. **16 attacks cannot critically hit**: all 14
  special moves plus aimed `shoot`, which routes through the same function.
- **The counterattack tier is melee-only.** `ParryCritDetected`,
  `DodgeCritDetected`, `BlockCritDetected` are set in one place, the melee swing
  path, and consumed in `hooks/combat_shared_helpers.go`. Four of five channels
  have the defensive-crit curve without the tier the parent spec says a
  defensive crit **is**.

### 2.2 The unowned family — combat contests belonging to no channel and no slice

Each resolves through `combat.RunContest`, is a real opposed combat contest,
and appears in **neither U6b's original scope nor any U10/U10b/U10c/U12 row**.
This is the same shape of gap that survived four slices.

| Mechanic | Skill weight | Crit | Other divergence |
|---|---|---|---|
| **Flee** (`combat/flee.go`) | **×25 both sides, hardcoded literals in `internal/`** | none | also violates standing rule 1 |
| **Grapple initiation** (`combat/grapple.go`) | ×1 both | **self-relative z-score**, pre-5.11d | hardcoded prone modifiers; scalar defence |
| **Grapple drift** (`hooks/Position_GrappleTick.go`) | ×2.2 / ×2.0 hardcoded | — | different score SHAPE (0.7·Str+0.3·Dex); **the `NOTE(U6)` missing-√2 normalisation defect is still live** and inflates every drift z by ~41% |
| **Submission** (`combat/submission.go`) | `SubSkillWeight` **1.5** | self-relative z | a surviving per-channel legacy factor, absent from the deletion table |
| **Throw / AoE** (`usercommands/throw.go`) | atk skullduggery ×5, def **a STAT as pseudo-skill**, skill ×0 | **none** | no defence set, no damage multiplier, binary full damage |
| **Steal** (`actions/steal.go`) | `StealSkillMultiplier`; defender **raw Perception, ×0** | none | same ×0-defence class as the spell gate |
| **Sneak / hidden detection** (`actions/skill_helpers.go`, `usercommands/go.go`) | `SkillMultiplier(skill)×25.0` — a **nonlinear sqrt curve times 25** | — | a sixth skill-weight regime |

**The grapple-drift √2 defect was assigned to U6 by the roadmap's own unowned
table and U6 did not do it.** It is picked up here.

### 2.3 Situational modifiers are per-channel accidents

| Modifier | Applies to |
|---|---|
| Prone attacker / vulnerable target multipliers | **melee only** |
| Prone DEFENCE penalties | melee's candidate builder only, **not** `GetDefenseScoreFor` |
| Sleeping auto-crit (`forceCrit`) | **melee only**, despite `CLAUDE.md` promising "the entire first round of attacks against them auto-crits" |
| Resource depletion | melee swings/damage, taunt, spell damage. **Nothing on ranged or special moves.** |
| Encumbrance | melee swing count and grapple drift only |

So after a naive U6b a prone defender would dodge a bolt at full score while
dodging a sword at penalty, and a sleeping victim would be auto-crit by swords
but not by the 16 attacks newly able to crit.

---

## 3. The target

```
score      attacker = stat + skill x SkillWeight, times SHARED situational modifiers
           defender = per-defence, from ONE defence-set source
contest    combat.RunContest(atkScore, entries)     -- ONE contest, always runs
crit       from that contest's margin, against ONE threshold mechanism
damage     defenceDamageMultiplier(res)             -- same curve everywhere
counter    defensive crit earns a counter, gated on reach (4.3)
progression events from the same margin
```

**One contest. It always runs. Crit from its margin, against one bar. Every
channel has a crit tier. One defence-set source. No mechanic resolves an
opposed combat contest outside this shape.**

### 3.1 What stays per-channel, deliberately

Unification is of the **shape**, not of every value:

- **The attacking STAT differs.** Melee autoattack Dexterity, aimed `shoot`
  Perception (deliberate: an aimed shot is not a swing), spells the spell's
  `primarystat` (U9), taunt Charisma. Only the *weight* on the skill term is
  uniform.
- **Defence sets differ.** You cannot parry a bolt.
- **`ChannelScale` damage constants differ**, and are config.
- **Fumble CONSEQUENCES differ** (melee miss, spell backfire, taunt CP
  self-damage, throw self-detonation). That is flavour and it stays. Fumble
  *derivation* is uniform.

### 3.2 Category C and D stay out, and the first draft's claim about them was wrong

Crafting, salvage and the flat `util.Rand` sites are Category C: a craft is a
probability against a recipe, not a contest against an opponent. `picklock` is
Category D, a puzzle, permanently out.

**Correction:** the first draft claimed the owner's "non-combat" was already
satisfied because those sites "already resolve through `contest.Run` /
`contest.AgainstDifficulty`". **That is false.** `contest.AgainstDifficulty` has
**zero production callers**, and search (×6), track and forage are still flat
`dice.RollStat` thresholds — including two hidden-detection checks that answer
the same question as `go.go`'s opposed contest using a flat 135 that ignores
the hider entirely. The roadmap marks these **UNASSIGNED** and they remain so.
They are Category B (roll vs static difficulty) and belong in a named slice;
**U6b does not silently absorb them, and does not pretend they are done.**

---

## 4. Scope

### 4.1 Collapse every hit gate

**Spell and taunt both** run a binary gate then a separate defence contest. Both
collapse: the channel defence contest becomes THE contest.

- `spellDefenseValue`, `runPlayerSpellContest`, `runMobSpellContest` and
  `runTauntContest` are deleted.
- Entries come from `DefenceSetFor(channel)`.
- `isCrit` comes from that contest's margin.
- **There are FIVE `if !isCrit` defence skips in `spell_resolution.go`, not
  one**, plus taunt's. An implementer deleting "the" skip will miss four.
- The "fizzles" outcome becomes an ordinary defence outcome.
- `CalcSpellAttack` drops `SpellAttackSkillFactor`; **the knob is deleted**.

**Which attack score survives is a decision, not a detail.** U9 moved the spell
attack roll onto `primarystat`, but `ChannelAttackScore` — the only score
builder `ResolveChannelDefence` knows — hardcodes Willpower + Spellcasting, and
`channelAttackSkillAndStat` is deliberately locked to it. Routing the collapsed
contest through `ResolveChannelDefence` as-is would make a manifestation spell
**attack with Willpower while rolling and progressing Charisma**, quietly
reverting U9 on the half of resolution that decides hit and crit.
**`ChannelAttackScore` must take the spell's `primarystat` and skill**, and
`channelAttackSkillAndStat` must follow it. Taunt's gate also applies a
conviction-depletion multiplier that `ChannelAttackScore` omits; the surviving
score keeps it.

### 4.2 One defence-set source, with equipment gating

Melee builds its set from `characters.GetDefenseSequence` (equipment-gated:
parry only when armed, parry twice when dual-wielding, block only with a
shield). The channel path builds from `DefenceSetFor` with **no equipment gate
at all**.

Left as-is, U6b hands a shieldless bare-handed defender a **block** against a
bolt or a spell while that same defender cannot block a sword. That is an
exploit surface, not an inconsistency.

**`DefenceSetFor` becomes the single source, and it gains the equipment gate.**
Melee's builder is deleted, with its two melee-only behaviours preserved
explicitly: the dual-wield double-parry entry, and `filterDefensesForThirdParty`
for grapple situations.

### 4.3 Ranged and the special-move family: real defence sets, with cost and progression

`ExecuteSkillMove` takes pre-computed scalars and builds one defender entry.
It takes a **channel** instead and builds entries from `DefenceSetFor`.

- All 14 callers migrate; they pass the skill and the seam applies `SkillWeight`.
- `rangedDefenseScore` and `RangedShieldDefenseBonus` are deleted; block as a
  real contested defence supersedes the flat bonus.
- **Route through `ResolveChannelDefence`, not a hand-built entry list.** This
  is the finding most likely to be missed. `ResolveChannelDefence` quotes and
  charges the winning defence, strips the skill term when the defender cannot
  pay (U8), awards defence progression, and fires the crit/fumble bonus tier
  (U9). Feeding entries straight to `RunContest` would make dodging a bash
  **free, un-stripped when exhausted, and unprogressed**, while the identical
  dodge against a sword costs stamina and trains — re-opening the drift U7/U8
  and U6 Task 12 closed.

### 4.4 Crit: one path, one bar, one tier

1. **`ExecuteSkillMove` gains crit and fumble.** `SkillMoveResult` gains
   `Crit bool` and `Fumble bool`. 16 attacks gain a tier they have never had.
   **Specify the damage semantics**: melee and taunt crits bypass mitigation
   and scale by rank via `CritOrMitigatedDamage`. A bash crit must do the same
   or the damage half of "same crit path" is unwritten.
2. **Spell and taunt crit move to the defence contest's margin**, falling out
   of §4.1.
3. **ONE crit threshold mechanism.** Melee's dynamic bar (Accuracy 1.5, Blink
   2.5, skill-difference shift) either becomes the shared bar for every channel
   or is deleted in favour of the constant. **It cannot stay melee-only**:
   otherwise Accuracy buffs sword crits and not spell crits, and Blink protects
   against swords only. Recommendation: hoist the dynamic bar into the shared
   crit seam, because buffs that work on one channel only are a bug players
   will find. Melee's separate hardcoded defensive bar goes with it.
4. **The counterattack tier extends to every channel, gated on REACH.**
   Owner decision.

   **Do NOT model the counter as "interrupting" the attack.** By the time quell
   answers, the spell has already resolved; there is nothing to interrupt. A
   defensive crit is not prevention. It is a decisive defence that leaves an
   opening, and the counter is what you do with the opening.

   | Channel | Attacker's position | Counter |
   |---|---|---|
   | Melee, taunt, special moves, same-room shot | same room | **melee counter** (riposte's mechanism, already built) |
   | Spell | **same room only** (no adjacent-room targeting exists) | melee counter |
   | **Defy** | same room | **counter-taunt, REPLACING the melee counter** (owner) |
   | **Ranged, cross-room** | adjacent room | **none** |

   The cross-room shot is the one case with no counter, and that is coherent:
   you cannot punch someone in the next room. Document it as designed. A
   wielding-dependent ranged counter was **considered and declined** (owner):
   a second conditional counter path for a narrow case.

   **Bound the recursion.** Dodge-crit auto-trip and block-crit auto-bash are
   themselves `ExecuteSkillMove` callers. Once §4.3 gives those a defence set
   and §4.4 gives them crit, a counter can be defensively crit, which earns a
   counter. **Counters do not themselves earn counters.** State it and test it.
5. **Fumble derivation stays self-relative** (`ZScore <= -threshold`), and
   `ExecuteSkillMove`'s new fumble needs a consequence chosen from §3.1's list.

### 4.5 The unowned family

All seven from §2.2 convert: **flee, grapple initiation, grapple drift,
submission, throw, steal, sneak/hidden-detection.**

- Uniform ×5 on both sides; every hardcoded weight literal (×25, ×2.2, ×2.0,
  `SubSkillWeight`, `StealSkillMultiplier`, `SkillMultiplier×25`) deleted.
- A defence set where an opposed defence is meaningful; a documented reason
  where it is not.
- Crit and fumble from the contest margin, replacing the self-relative z-scores
  in grapple and submission.
- **The grapple-drift missing-√2 normalisation is fixed here**, discharging a
  U6 obligation the roadmap recorded and U6 dropped.
- `throw` gains a defence set and a damage multiplier; today it is binary
  full-damage with no defence.

### 4.6 Shared situational-modifier layer

§2.3's table becomes one layer applied by the seam. At minimum, prone
(attack, vulnerability and DEFENCE), sleeping auto-crit, resource depletion and
encumbrance apply per a **declared per-channel table** rather than per-channel
accident. Where a modifier is deliberately channel-specific, the table says so.

### 4.7 Deletions

`SpellAttackSkillFactor`, `spellDefenseValue`, `runPlayerSpellContest`,
`runMobSpellContest`, `runTauntContest`, `rangedDefenseScore`,
`RangedShieldDefenseBonus`, `SubSkillWeight`, `StealSkillMultiplier`, melee's
`GetDefenseSequence` set-builder, all six `if !isCrit` skips, and every
hardcoded skill-weight literal in the §2.2 family.

**Standing rule 5 — "no legacy parameter survives U6" — is what this discharges.**

---

## 5. Out of scope, with reasons

| Deferred to | What | Why it is genuinely separate |
|---|---|---|
| **U10** | Concentration, knockdown, prone recovery | Disruption is its own model with its own difficulty table, already specced |
| **U10b** | Progression firing consistency, Category C routing, the defence-award divergence | Firing conditions, not resolution |
| **U10c** | Charm (×25 weight, defence stat) | Being rewritten wholesale there; converting it twice is waste |
| **NEW, must be named** | Category B: search ×6, track, forage, and the two flat hidden-detection checks | Roll-vs-difficulty, not opposed. **Currently UNASSIGNED and must not stay so** |
| **U11** | Messaging unification | Owner: known, different slice |
| **U12** | Targeting simplification | Simplification pass |

**Messaging for NEW mechanics is NOT deferred.** 16 attacks becoming
defendable, and four channels gaining counters, need their attacker/defender/
room triads in this slice. The broader message-ownership rework is U11's; text
for a mechanic this slice invents is this slice's, per the gap U8 had to close
for quell and defy.

---

## 6. Behaviour changes, named

> **Corrected 2026-08-19 by the modelling gate** — see
> [`2026-08-19-u6b-modelling.md`](2026-08-19-u6b-modelling.md) §7. The first
> version of this table over-claimed on spell PvE and was simply wrong about
> taunt.

| Change | Direction | Who |
|---|---|---|
| Spell attacker skill ×15 → ×5, defender gains `spellcasting×5` | **PvE ≈ unchanged (0.99×). Caster-vs-caster transformed**: 85.5%/84%-crit → 49.7%/2.5%, 0.19× | All |
| Taunt: defender no longer contested twice | **Pure ATTACKER buff** — hit/crit bit-identical; E[mult] 0.338 → 0.658 | All |
| Spell crits now face quell | Cut to crit reliability (spell ONLY; taunt already did) | All |
| Ranged + 14 special moves: skill ×1 → ×5 both sides | Increase | All |
| Ranged + 14 special moves: real defence SET | Increase to defenders | All |
| **16 attacks gain a crit tier that never existed** | **Large increase** | All |
| Counterattack tier on 4 new channels, reach-gated | Increase to defenders | All |
| One crit bar: Accuracy/Blink/skill-shift apply everywhere or nowhere | Depends on §4.4.3 | All |
| Equipment gate on channel defences (no shieldless blocks) | **Cut** to an exploit | Defenders |
| Defending a bash/shot now COSTS stamina and trains | Cut, and a new drain | Defenders |
| Flee/grapple/drift/submission/throw/steal/sneak to ×5 | Mixed, large | All |
| Grapple drift √2 fix: every drift z falls ~41% | **Large cut** to drift swing | All |
| Shared situational modifiers reach channels that had none | Mixed | All |

**Mobs all carry combat skill 1**, so every ×1→×5 change widens the
player-versus-mob gap far more than player-versus-player. That asymmetry is the
single most important thing for modelling to quantify.

---

## 7. Modelling gate — MANDATORY, before any code

> ✅ **DISCHARGED 2026-08-19**, contingent on decisions:
> [`2026-08-19-u6b-modelling.md`](2026-08-19-u6b-modelling.md). Three scripts
> under `tools/balance/u6b_model_*.py`, anchors reproduced, appendices A–C
> alongside. **One BLOCKING decision before the plan: mobs are skill 0 on
> every non-melee path** (zero mob YAMLs author skills; the skill-1 fallback
> lives only in `GetCombatSkillLevel`, which only melee autoattack uses), so
> as-specced the flip guts mob specials AND mob casters against skilled
> players. Modelling doc §5.1 has the options. Also forced: fizzle becomes a
> partial-damage outcome (§5.2 — the neutral spell results depend on it), and
> the drift reweight deletes the aggressor's edge (§5.4) unless it returns as
> an explicit config modifier.

No weight changes until this is done and reviewed. It discharges a gate the
roadmap raised before U6 and that U6 shipped without.

1. **Spell accuracy** across novice → Meirok, versus trash / parity / boss.
   Attacker loses two thirds of its skill term while the defender gains one
   from zero; report the NET.
2. **Spell and taunt crit rates**, now that crit faces a real defence.
3. **The 14 special moves at ×5 both sides.** Reproduce the roadmap's own
   `130 vs 101` → `250 vs 105` example across the skill range.
4. **The new crit tier on 16 attacks**, combined with item 3 — they compound.
   Include the beast moves, which mobs use heavily.
5. **Ranged**, losing the flat shield bonus and gaining block.
6. **Defence-set width**: a bash answered by up to three defences rather than
   one scalar.
7. **Counter frequency per channel.** Defensive crit is margin-driven, so it
   rises sharply when the defender outclasses the attacker. Include what
   cross-room immunity is worth to a kiting shooter.
8. **The §2.2 family at ×5**, especially flee (×25 → ×5 is a *cut* of 80% to
   both sides) and drift (√2 fix plus reweighting, compounding).
9. **Defence cost load.** Defending bashes and shots now costs stamina;
   quantify the added drain across a fight.

Use `tools/balance/unified_resolution_model.py`. **Deliverable: a modelling
document reviewed before the implementation plan is written.**

---

## 8. Open questions for spec review

1. **Does "fizzle" survive as player copy?** Mechanically it becomes an
   ordinary defence win.
2. **Does `shoot` keep Perception?** This spec assumes yes; only the weight
   unifies, not the stat.
3. **§4.4.3: hoist melee's dynamic crit bar, or delete it?** This spec
   recommends hoisting, because a buff that works on one channel only is a bug.
4. **Where do the Category B sites (search/track/forage) go?** They need a
   named slice. Not U6b, but not nowhere.
5. **Does `throw` get a defence set, or is AoE deliberately undefendable?**
   A case exists that area damage should not be dodgeable one-target-at-a-time.

---

## 9. The process defect, and a guard that would actually work

U6 was declared done while its own criteria were false, and nothing detected it
for four slices. **The first draft of THIS spec then made the same error**,
declaring taunt converted (§12).

Prose does not fail. Neither does a test that only knows about channels.

**The guard must enumerate `combat.RunContest` CALL SITES, not channels.**

```go
// Every combat.RunContest call site must be behind a channel seam, or on this
// allowlist naming the slice that owns it. A guard that enumerates CHANNELS
// only protects channels somebody remembered to name -- which is how spell's
// x0 defence survived U6, and how flee, grapple, drift, submission, throw and
// steal survived four slices without appearing on any roadmap row.
var contestSiteOwners = map[string]string{ /* file:func -> owning slice */ }

func TestEveryContestSiteIsOwned(t *testing.T)
func TestEveryChannelUsesUniformDefenceSkillWeight(t *testing.T)
func TestNoLegacySkillWeightLiteralSurvives(t *testing.T) // greps for the literals, not just knobs
```

The third matters because `SubSkillWeight` is a knob but flee's ×25 and drift's
×2.2 are **literals**, which no knob-grep can see.

**U11 inherits all three**, and every remaining "Done when" criterion should be
expressed as a test before the arc is declared finished.

---

## 10. Testing

- **Per-mechanic contest-shape tests**: one contest, entries from the single
  defence-set source, crit from that margin, against the shared bar.
- **No-opt-out test**: no mechanic skips its defence contest on a crit. This is
  the specific regression that produced both the spell and taunt shapes.
- **Counter-recursion bound**: a counter cannot earn a counter.
- **Equipment-gate test**: a shieldless defender cannot block, on any channel.
- **Defence cost/progression parity**: dodging a bash costs and trains exactly
  what dodging a sword does.
- **The three §9 guards.**
- **Parity damage-per-swing within ±10%** where not deliberately changed, per
  the arc's completion criterion 5.
- Expect many existing tests to need updating; each named in the PR with what
  it was pinning.
- Isolated boot test per the pre-push SOP.

---

## 11. Playtest gate

**Required.** This changes how every attack in the game resolves, adds defence
narration to 16 actions, and adds counters to four channels.

The owner's pre-deploy manual pass gains: spell accuracy at veteran level,
special moves being defended for the first time, counters firing on non-melee
channels, and whether the widened player-versus-mob gap reads as mastery or as
trivialisation.

Merging is not deploying. Nothing deploys until the arc is done and tested with
the AI harness plus a full manual pass.

---

## 12. What the audit found, and why this spec was rewritten

A blind adversarial completeness audit was run against the first draft with one
question: *after this ships, what will still not be unified?*

It found the draft's "measured state" table — labelled "verified against source,
not inferred" — **declared taunt converted when taunt is a two-contest channel
whose crit skips its own defence.** That is the identical failure the spec was
written to end, committed inside the document condemning it.

It also found: the crit threshold is two mechanisms; melee never adopts
`DefenceSetFor` and the channel path has no equipment gate; an unowned family
of seven combat contests belongs to no slice; §4.2 would have made channel
defences free and unprogressed; the primarystat/Willpower conflict would have
reverted U9's work on the hit contest; situational modifiers are per-channel
accidents; five `if !isCrit` skips exist rather than one; counter recursion was
unbounded; and the proposed guards would have caught none of it.

Recorded here rather than quietly fixed, because the lesson is the point: **a
completeness audit by something that did not write the spec is now part of how
this arc closes a slice**, not an optional extra.
