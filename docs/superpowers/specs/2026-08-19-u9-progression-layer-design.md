# U9 — The Progression Layer

**Created:** 2026-08-19
**Arc:** [`UNIFIED_RESOLUTION_ROADMAP.md`](../../roadmaps/UNIFIED_RESOLUTION_ROADMAP.md) (U0–U12)
**Parent spec:** [`2026-08-12-unified-contest-resolution-design.md`](2026-08-12-unified-contest-resolution-design.md) §5 (Layer 3)
**Depends on:** U6 (the flip), U8 (the action registry)
**Size:** M
**Behaviour change:** Yes, named and bounded (see §7)

---

## 1. Why

Layer 3 of the contest design is the last unbuilt layer. The roadmap row reads
"progression layer: events not side effects, both sides, doing vs observing,
skill **and** stat on every event."

Reading the code, three things are true that the row does not say:

1. **U6 built more of this than the roadmap admits.** `OnSkillUseScaled`
   already fires the skill's primary stat via `skills.GetSkillPrimaryStat`, so
   "skill and stat on every event" is already true at every ordinary
   `OnSkillUse` site (93 production `OnSkillUse` / `OnSkillUseScaled` /
   `OnStatUse` call sites across 45 files, or 135 across 52 counting the crit, track and regen entry points; audited 2026-08-19). `combat.AwardDefenceProgression` is already THE
   five-defence mapping, and `processDefenderProgression` already dedupes once
   per defence type per round.
2. **U8 built the registry this slice needs.** `internal/costs/action.go`
   declares `Spec{Skill, SkillSource, Physical}` for 27 actions and already
   answers "which skill does this action exercise?" in one data table,
   including the `SkillEquippedCombat` case for weapon-versus-unarmed
   autoattack. Building a second table would be the drift this arc exists to
   remove.
3. **There is a live progression faucet, and it is structurally the fyttyn
   exploit.** See §3.

So U9 is smaller than the row implies in one direction and larger in another.
It is not "wire progression"; it is "give progression one shape, close the
faucet, and make the spell record load-bearing."

---

## 2. Scope

**In:**

- The event seam (§4): a pure `internal/progression` package returning events,
  a single applier, and the contest paths routed through it.
- One action registry (§4.2): `costs/action.go` moves to `internal/actionspec`
  and gains an optional `Stat` override.
- The event matrix (§5): observing events on both sides, crit and fumble both
  earning a bonus.
- The faucet fix (§3): every progression event runs the decayed curve.
- Two new config knobs, replacing one hardcoded literal (§6).
- `SpellData.PrimaryStat` wired to both resolution and progression, caster side
  only, with the manifestation family corrected to `charisma` (§8).
- Five defects fixed in passing (§7.3), including melee double-progression on
  BOTH the attack and the defence side.
- The firing-condition audit (§9), which is a deliverable, not a refactor.

**Out, and deliberately so:**

| Deferred to | What |
|---|---|
| **U10** | Concentration and disruption. Untouched here. |
| **U10b** (new) | The firing-condition consistency pass. Category C (crafting, salvage, forage) routing. |
| **U10c** (new) | The charm redesign: real duration, re-roll ladder removed, skill weight returned to uniform 5.0. |
| **U11** | Helpfiles, config organisation, the arc's adversarial playtest gate. |

**U9 requires no adversarial playtest of its own.** It adds no player-facing
content or new prose. The arc's U11 gate and the owner's pre-deploy pass cover
it. Owner decision, 2026-08-19.

---

## 3. The faucet, and why this slice is urgent

There are **two** independent reasons vitality progression does not decay, and
the first draft of this spec found only one of them. Adversarial review found
the second, which is the load-bearing one.

**Reason one: the chance never consults rank.** `Character.OnCritReceived`
routes through `CheckRegenProgression`, which was written for regen ticks. It
applies mob gating, the per-stat multiplier and the mutation multiplier — but it
**never calls `CalculateProgressionChance` and never applies
`StatProgressionRate`**. The chance is a flat 0.25 at every virtual rank.

**Reason two: the rank never moves.** `CheckStatProgression` derives
`virtualRank` from `GetStatUseCount(statName)` (`progression.go:163`). Measured
2026-08-19, production `OnStatUse` call sites by stat:

| dexterity | strength | willpower | perception | charisma | **vitality** |
|---|---|---|---|---|---|
| 7 | 6 | 4 | 4 | 4 | **0** |

**Nothing in the game tracks vitality use**, and `CheckRegenProgression` — the
only path that progresses it — does not track either. So vitality's virtual
rank is pinned at 0 until the anti-exploit floor engages at value 150, and
below that its progression chance is **constant regardless of how much vitality
the character already has**.

That is not merely the same *shape* as the exploit `internal/migration/0.16.0.go`
exists to freeze:

> Player fyttyn ground raw vitality to 411 [...]

It is the *mechanism* of it. Fixing reason one alone would have taken the
headline number from 25% to a flat 13.5% and left the rank-independence intact,
while the spec claimed a curve that decayed to 0.67%.

U6 made it worse again. Crit is margin-driven now, so crit rates against a badly
outclassed defender approach certainty: standing in front of something far
stronger than you is a no-decay tap whose rate rises with how outmatched you are.

### 3.1 The fix, three parts

1. **Crit-received runs the decayed curve.** `OnCritReceived` calls
   `CheckStatProgression` at the observing multiplier, like every other event.
2. **Both `CheckRegenProgression` callers track the use.** `OnCritReceived` and
   `OnRegenTick` call `TrackStatUse` before rolling, so the counter that decays
   the curve actually moves. Owner decision, 2026-08-19: do both, because
   fixing only the crit path leaves open the exact low-health grind fyttyn used.
3. **`CheckRegenProgression` itself consults rank.** Added 2026-08-19 after
   parts 1 and 2 were already agreed, because verification showed they were not
   enough on their own.

**Why part 3 exists, and why the first draft of this section was wrong.**
`CheckRegenProgression` contains **zero** references to rank or use count: its
chance is `base × (1 − ratio)^curve` from pool depletion, times the per-stat and
mutation multipliers, and nothing else. So parts 1 and 2 alone make the *counter*
move — which correctly decays crit-received, since that now routes through
`CheckStatProgression` — while **the regen roll itself stays flat forever**.

An earlier draft of this spec claimed part 2 made willpower, charisma and
strength "decay with use for the first time" and that "veteran growth slows
sharply". That was false. The low-health grind *is* the regen path, and it would
have survived untouched while this document claimed the faucet was closed.

**The form.** Rather than introduce a second curve or a new knob, the regen
chance is multiplied by the existing curve, normalised against its own base:

```
decayFactor = CalculateProgressionChance(virtualRank, StatProgressionSoftCap)
              / BaseProgressionChance
```

`CalculateProgressionChance` returns exactly `BaseProgressionChance` at rank 0,
so the factor is **1.0 for a fresh character and the change is a pure veteran
nerf** — nobody's early game gets slower. It reaches `exp(-3) ≈ 0.05` at the
soft cap. Guard the division: a `BaseProgressionChance` of 0 must yield a factor
of 0, not a NaN.

| Vitality virtual rank | Regen chance, before | Regen chance, after |
|---|---|---|
| 0 | 1.00× | **1.00×** (unchanged) |
| 75 | 1.00× | **0.22×** |
| 150 (soft cap) | 1.00× | **0.05×** |

**Blast radius, stated honestly.** `OnRegenTick` maps health to vitality and
willpower, stamina to strength and vitality, and conviction to willpower and
charisma. Regen is also the **only** source of vitality progression in the game,
so vitality slows the most. This is the largest single behaviour change in U9
and it is listed in §7.1.

With both halves, the rate a character actually experiences:

| Vitality virtual rank | Before | After |
|---|---|---|
| 0 | 25.0% | 13.5% |
| 75 | 25.0% | 3.0% |
| 150 (soft cap) | 25.0% | 0.67% |

and, unlike the first draft's version of this table, **the rank column is now
reachable**, because half two makes the counter move.

Computed from shipped `config.yaml` values: `BaseProgressionChance` 0.12,
`ProgressionDecayBelowCap` 3.0, `StatProgressionSoftCap` 150,
`StatProgressionRate` 2.25, `ObservedCritProgressionBonus` 0.5. Per-stat and
mutation multipliers apply on top in both columns and cancel out of the
comparison.

`CheckRegenProgression` keeps its depletion-derived chance — how empty the pool
is remains the thing that drives passive growth. What changes is that the result
is now damped by rank, and that its callers record the stat was exercised.

---

## 4. Architecture

### 4.1 The seam

```
internal/actionspec  (new leaf)      Spec{Skill, SkillSource, Physical, Stat}
        ^                            <- costs/action.go moves here verbatim
   +----+----+
internal/costs    internal/progression  (leaf: imports skills + actionspec)
                        ^                Event{Side, Skill, Stat, Class, Multiplier}
                        |                EventsForContest(...) []Event
                  internal/characters     ApplyProgression(events, userId)
                        ^
                  internal/combat         builds events, hands them to the applier
```

Rules that make this hold:

- **`progression` is pure.** An `Event` carries plain values: no
  `*characters.Character`, no `*rooms.Room`, no config read. This is spec §5.3's
  stated reason for the seam — the matrix becomes table-testable without a
  hydrated character, and a Go test binary never loads `config.yaml` anyway.
- **`Side`, not an actor pointer.** `Side` is `SideAttacker` / `SideDefender`.
  The caller already knows who is who. Putting a character in the event forces
  `progression` to import `characters`, which kills the leaf property and
  creates a cycle with the applier.
- **`characters.ApplyProgression` is the only thing that fires
  `OnSkillUseScaled` / `OnStatUse` for contest events.** Every existing entry
  point stays public and working, so the non-contest call sites do not move in
  U9.

### 4.2 One registry

`costs/action.go` moves to `internal/actionspec` **unchanged**, and `costs.Spec`
becomes a type alias so no existing call site changes. One field is added:

```go
Stat string // override; empty means skills.SkillPrimaryStats[Skill]
```

Empty for all 27 registered actions, because each already wants its skill's
primary stat. The field exists for the two genuine divergences: a spell's
`primarystat` (§8) and the crit-received toughening stat (§5).

**Why move rather than extend in place.** `costs` importing `progression`, or
the reverse, is a cycle waiting to happen, and a package named for cost that
also owns progression's data is precisely the "name stops matching scope" drift
this arc removes. The move is one file.

**Spells stay out of the registry.** There is no `ActionCast`; spell cost is
authored per-spell on `SpellData.Cost`. The spell path builds its `Spec` at the
call site from `SpellData`, with `primarystat` filling `Stat`. Same mechanism,
not a second one.

---

## 5. The event matrix

**The governing rule of this slice: U9 changes what an event *carries*, never
*when* it fires.** Every existing firing condition is preserved exactly. The
audit (§9) records them and U10b decides which are deliberate. The only new
events are the observing cells, which fire where nothing fires today, so U9 is
additive on that side and a correction on the others.

Both a skill roll and a stat roll on every cell. `B` = `CritProgressionBonus`,
`O` = `ObservedCritProgressionBonus`.

| Outcome | Attacker gets | Defender gets |
|---|---|---|
| Attack crit | attack skill + attack stat @ **B** | defence skill + **toughening stat** @ **O** |
| Attack fumble | attack skill + attack stat @ **B** | defence skill + defence stat @ **O** |
| Defence crit | attack skill + attack stat @ **O** | defence skill + defence stat @ **B** |
| Defence fumble | attack skill + attack stat @ **O** | defence skill + defence stat @ **B** |
| Ordinary hit or miss | attack skill + attack stat @ 1.0 | defence skill + defence stat @ 1.0 |

The stat is the registry's — the skill's primary stat unless overridden. The one
exception is **crit received**, which uses the existing `OnCritReceived` channel
mapping: physical to vitality, magical to willpower, conviction to charisma. You
learn to take a hit, not to swing better.

**Bonus events are additive, not replacements.** The ordinary event still fires
and the crit or fumble event lands on top. That is both what melee does today
and the shape U10b will generalise, so it becomes the documented rule rather
than an accident.

**Fumbles earn the bonus.** Spec §5.0 says failure teaches, and the shipped
message ("You learn from your mistake!") already implies a bonus that
`OnCriticalFailure` does not currently pay — it passes 1.0.

**Crit and fumble are determined by MARGIN, not by a self-relative z-score.**
Since 5.11d the engine decides a contest crit from the normalized opposed-roll
margin: `defence_multiplier.go:307` uses `DefenseContestCrit(-res.Margin,
res.DefenseRoll)`, which tests `margin / (stdDev × √2)` against
`ContestCritThreshold` (`margin_crit.go:90,126`). Any consumer that re-derives
crit from `AttackRoll.ZScore` will fire the bonus tier on a **different set of
swings than the game narrates as crits** — two mechanisms answering one
question, which is what this arc exists to delete. The seam must read the
existing helpers, or the already-computed `ChannelDefenceResult.DefensiveCrit`,
and must never re-derive.

### 5.1 The four rules

1. **Every event runs the decayed curve.** See §3.
2. **Bonus events dedupe once per round**, per side, per skill. Ordinary
   per-swing events are untouched, so normal melee progression rates do not
   move. This mirrors `processDefenderProgression`'s existing
   once-per-defence-type-per-round idiom.
3. **A floored outcome fires ordinary events but never a bonus.** Reads
   `contest.Result.Floored`, which U6 added for exactly this class of question.
   Rationale: participation still teaches, but a crit the dice did not produce
   is not an exceptional event. Without this, a hopeless defender banks a bonus
   on 12.5% of swings from a boss they cannot touch, and the faucet rate rises
   with how outmatched they are.
4. **Partial-pay actions progress in full.** U8 lets an exhausted actor
   autoattack, defend, flee and maintain a grapple without the skill term.
   Exhaustion is a combat-effectiveness penalty, not a progression penalty.
5. **Bonus events do not track a use — EXCEPT the observed one.** Added
   2026-08-19 during execution, after routing melee through the seam revealed
   the blanket rule was too broad.

   The parent spec's rule is that a bonus event must not inflate the use
   counter, because the counter becomes a virtual rank and the curve
   *decreases* with rank — so tracking a crit would punish critting. That
   reasoning is sound for `ClassCrit` and `ClassFumble`, which belong to the
   party who *did* the thing.

   It does not hold for `ClassObserved`. You do not want to be crit, so there
   is no achievement to punish, and the crit-received toughening event is
   precisely the thing whose tracking makes vitality's rank move — the
   mechanism §3 exists to close. Routing melee through the seam had silently
   dropped that tracking while the spell path, still unrouted, kept it, so the
   two disagreed on a rule U9 itself introduced.

   **The rule is therefore keyed on Class: doer events do not track, observed
   events do.** Owner decision, 2026-08-19: *"The whole point of this effort is
   to unify."*

### 5.2 Observers are participants only

Spec §5.0 says "received or witnessed". U9 reads that as the two sides of the
contest and nothing else. Room spectators are excluded deliberately: a room scan
per crit costs real time in busy rooms, and it makes standing in a room doing
nothing a progression source.

### 5.3 Mob parity

Events fire for mobs as they do for players, under the existing
`MobProgressionEnabled` and `MobProgressionRate` gates. No new gate, no new
branch. Parity across all four combat quadrants is a standing project
convention.

---

## 6. Config

Two knobs in `_datafiles/config.yaml`, both validated `>= 0`, both legal at `0`
as off-switches.

| Knob | Value | Note |
|---|---|---|
| `CritProgressionBonus` | 2.0 | Replaces the **hardcoded 2.0 literals** in `internal/characters/progression.go`, per standing rule 1: no balance number inside `internal/`. |
| `ObservedCritProgressionBonus` | 0.5 | New. |

There are **two** such literals, not one. `progression.go:301`
(`OnCriticalSuccess`) is deleted by §7.3 anyway, but `progression.go:332`
(`OnFirstMobKill`, the first-kill-of-a-mob-type bonus) carries its own bare
`2.0` and would have survived. It reads the knob too. Adversarial review caught
this; without it §6 would have claimed rule-1 compliance it had not achieved.

`buffSkillMult = 2.0` at `progression.go:105` is deliberately **left alone**. It
is the Skill Attunement buff's doubling, a buff effect rather than a crit
multiplier, and folding it into `CritProgressionBonus` would couple two
unrelated things to one knob. It is a separate rule-1 item, filed rather than
fixed here.

Starting values are the parent spec's proposals. Tuning is deferred to the
post-build playtest, per project convention.

Both knobs are documented in `config.yaml` beside their values, with what they
do and what changing them costs, per the arc's completion gate.

---

## 7. Behaviour changes, named

U1–U5 were contracted as provable no-ops. U9 is not one, and the changes are
enumerated here so each can be called out individually in the PR.

### 7.1 Rates that move

| Change | Direction | Who |
|---|---|---|
| Crit-received stat progression moves onto the decayed curve | Large cut, growing with rank (§3) | Players and mobs |
| Crit-received also awards the **defence skill**, which it never did | Small increase | Players and mobs |
| Fumbles earn `CritProgressionBonus` instead of 1.0 | Increase | Both sides |
| Observing events on defence crit / defence fumble / attack fumble | Increase, new events | Both sides |
| Melee ATTACK double-progression deleted (§7.3) | **Large cut**, roughly halves melee attacker rates | Players and mobs |
| Melee DEFENCE double-progression deleted (§7.3) | **Large cut**, scales with swings defended per round | Players and mobs |
| Regen and crit-received paths now TRACK the stat use (§3.1 part 2) | Enables rank decay; no direct rate change of its own | Players and mobs |
| Regen progression damped by rank (§3.1 part 3) | **Largest change in U9.** Unchanged at rank 0, ~0.22x at rank 75, ~0.05x at the soft cap. Vitality slows most, being regen-only | Players and mobs |
| Bonus events dedupe once per round | Cut in multi-swing rounds | Both sides |
| Floored outcomes no longer earn bonuses | Cut at extreme mismatch | Both sides |
| Caster stat progression halves (double-roll fixed, §7.3) | Cut | Players and mobs |

### 7.2 The `primarystat` change

Covered in §8. Expected delta is near zero and is verified file-by-file rather
than asserted.

### 7.3 Five defects fixed in passing

- **Melee DEFENCE progression fires twice per round too.** Found by adversarial
  review 2026-08-19, and it is the symmetric half of the attack duplication
  below. An earlier draft of this spec documented this path as correct.

  `internal/combat/combat_helpers.go:1227-1229`, inside `sendDefenseMessages`,
  fires on **every defended swing**:

  ```go
  if skillToProgress != "" {
      targetChar.TrackSkillUse(skillToProgress)
      targetChar.CheckSkillProgression(skillToProgress, targetChar.GetUserId(), 1.0)
  }
  ```

  while `processDefenderProgression` → `AwardDefenceProgression` → `OnSkillUse`
  independently awards the same skill once per defence type at end of round —
  and `OnSkillUse` also rolls the primary stat, which the per-swing path does
  not.

  So a defender who dodges four swings takes five skill rolls, not one. The
  per-swing `CheckSkillProgression` call is deleted; `TrackSkillUse` is deleted
  with it, since `OnSkillUse` already tracks. **`sendDefenseMessages` must not
  be added to the U9 guard test's allow-list** — allow-listing it is how this
  defect becomes permanent.



- **Melee progression fires twice per round.** `internal/hooks/NewRound_DoCombat_unified.go`
  runs progression in two phases against the same actors in the same round:

  | Phase | Call | What it awards the attacker |
  |---|---|---|
  | 2 | `rollCombatAttack` → `combat.AttackPlayerVsMob` (`combat.go:76-103`), and `trackMobAttackProgression` (`combat.go:185`) for the mob quadrants | `OnStatUse("strength")`, `OnStatUse("dexterity")`, `OnSkillUse(combatSkill)`, `OnCriticalSuccess` |
  | 5 | `applyCombatProgression` (`:188`) | `emitAttackerStatGain("strength")`, `emitAttackerStatGain("dexterity")`, per-`WeaponHits` `OnSkillUse` + `OnCriticalSuccess` / `OnCriticalFailure` |

  Both are unconditional. `applyCombatProgression`'s doc comment describes
  itself as the path that handles all four quadrants including their
  divergences, which reads as the unified orchestrator having been added
  without the per-quadrant calls being deleted.

  Net effect, if confirmed: roughly **two strength rolls, two skill rolls and
  four or more dexterity rolls per round** where the design intends one each.
  Dexterity compounds because `OnSkillUseScaled` fires the skill's primary stat
  and both weapon-combat and unarmed-combat map to dexterity. Crits pay the
  bonus twice. `AttackMobVsPlayer:230` additionally gives the **defender** an
  `OnStatUse("dexterity")` on top of `AwardDefenceProgression`'s dodge award.

  **This is read from source, not measured.** The plan's first code task is an
  instrumented test that counts actual `SkillUseCount` and `StatUseCount` deltas
  across one round in all four quadrants, before anything is deleted. If the
  duplication is real, Phase 2's progression is deleted and Phase 5 becomes the
  single path, which is where the U9 seam plugs in anyway. If it is somehow not
  real, the plan stops and the spec is corrected.

  This is the largest rate change in U9 and it is a **cut**, so it compounds
  with §5.1 rule 2's dedupe. The two together must be assessed at playtest
  before `CritProgressionBonus` is tuned.



- **Spell double-roll.** `internal/hooks/NewRound_DoCombat_helpers.go:319-326`
  (user) and `:468-474` (mob) call `OnStatUse("charisma")` or
  `OnStatUse("willpower")` immediately after `OnSkillUseScaled`, which already
  fires the same stat via `GetSkillPrimaryStat` (manifestation maps to charisma,
  spellcasting to willpower). Every cast has taken two stat rolls instead of one,
  on both branches, for players and mobs.
- **Phantom skill keys.** `OnCriticalSuccess` and `OnCriticalFailure` call
  `TrackSkillUse("critical_success")` / `("critical_failure")`, writing counters
  into every save that nothing reads. Deleted. Existing keys are left inert
  rather than migrated: they cost one map entry and affect nothing.
- **Hardcoded spellcasting rank in a DoT duration.**
  `internal/hooks/spell_resolution.go:1451` reads
  `GetSkillLevel(skills.Spellcasting)` even for manifestation-school spells.
  Corrected as part of §8's unification.

---

## 8. `SpellData.PrimaryStat` — wiring the intent

The field is declared at `internal/spells/spells.go:31` with the comment "Stat
used for spell rolls and progression", is parsed by 58 of 59 spell files, and is
**read by zero Go code**. Spell resolution hardcodes Willpower instead. The
field records a design intent that was never connected.

Owner direction, 2026-08-15: the spell's `primarystat` and a skill's primary
stat *"arguably should be unified and both drive progression."* Owner direction,
2026-08-19: full unification, and *"all Willpower is not correct."*

### 8.1 What changes

`primarystat` becomes load-bearing on the **caster side only**:

- It supplies the `Stat` override for the cast's progression event.
- It replaces the **11 caster-side `Stats.Willpower.ValueAdj` reads** in
  `internal/hooks/spell_resolution.go`: lines 85, 526, 725, 922, 980, 991 (user
  path) and 1147, 1295, 1326, 1337, 1451 (mob path). These drive spell attack,
  DoT duration, shield strength and buff duration.

**Defender-side Willpower is untouched.** `spell_resolution.go:1060` is quell
and `internal/hooks/charm_spell.go:60-63` is the charm target's resist. U6 owns
the defence set; `primarystat` has no business there. Conflating the two would
silently move quell off the stat U6 designed it around.

### 8.2 Data

**Fourteen** manifestation-school files get `primarystat: charisma`. Verified by
reading the `schools:` block of all 59 files, not by filename:

- Thirteen change from `willpower`, all `effect_type: none`: `conjure-air`,
  `conjure-earth`, `conjure-fire`, `conjure-magma`, `conjure-water`,
  `raise-golem`, `raise-skeleton`, `raise-spectre`, `raise-vampire`,
  `raise-wraith`, `raise-zombie`, `summon-hive-swarm`, `summon-steppe-spirit`.
- `charm.yaml` **gains the field**, which it is alone in never having declared
  (58 of 59 files declare it). Its school is already `manifestation`, so nothing
  else about it moves. It is `effect_type: charm`, not `none`.

> `veil-rend` is **not** in this set. An early scoping pass matched it on a
> filename grep for summon/conjure/raise; its `schools:` block does not contain
> manifestation. Named here because the mistake is easy to repeat.

**Expected behaviour delta is near zero.** The thirteen `effect_type: none`
files never reach the damage, shield or DoT-duration Willpower reads; their
power already runs on `characters.CalcCompanionPool`, which U7b defined as
Charisma plus manifestation. Charm resolves in its own path
(`internal/hooks/charm_spell.go`) and already scores `Charisma + Manifestation`
in Go. Correcting the data makes it describe behaviour the game already has.

**This is verified file-by-file in the plan, not asserted.** A summon that turns
out to reach a Willpower-derived path is a balance change and must be found
before merge, not after.

### 8.3 Validator

`primarystat` becomes **required** and must name one of the six stats, enforced
at spell load. The field is load-bearing now, so a typo must fail at boot rather
than silently falling back to a default. This matches the project's existing
convention that authored content panics at startup on an unresolved reference.

**The upstream default world must be fixed too, or a fresh checkout panics.**
Adversarial review found that `_datafiles/world/default/spells/` holds 8 spell
files and **none of them declares `primarystat`**, while
`internal/configs/config.filepaths.go:23` defaults `DataFiles` to
`_datafiles/world/default` whenever the key is absent. Our shipped
`config.yaml` sets it to the dogmud tree, so our own boot test would have passed
and hidden this — a fresh checkout, a stripped container config or an ephemeral
playtest env would have boot-panicked.

Owner decision, 2026-08-19: **add `primarystat` to all 8 default files** rather
than softening the validator. A silent fallback is precisely what let this field
mean nothing for its entire life, and it would swallow typos as well as
omissions.

---

## 9. The audit deliverable

`docs/audits/2026-08-19-progression-firing-audit.md`, indexed in
`docs/README.md`.

Progression fires under at least seven different conditions today, with no
convention:

| Path | Skill fires when |
|---|---|
| Melee autoattack | `CleanHit` only (U6 Task 14) |
| Special moves (bash, kick, trip, gore, hamstring, maul, pounce, rake, drain, throttle, grapple) | on hit only |
| `shoot` | skill on hit, but the `perception` stat on **every** shot |
| `throw` | always, hit or miss |
| `taunt` | always, all three outcomes |
| `warcry` / `rally` | always in combat, **50%** out of combat |
| Melee defences, per-round path | once per defence type per round (`processDefenderProgression`) |
| Melee defences, per-swing path | **every defended swing** (`combat_helpers.go:1228`) — the duplicate deleted by §7.3 |
| Channel defences (spell, social) | **whenever the contest ran, win or lose** (`defence_multiplier.go:246`) |

The audit records every site, its firing condition, what it awards, and whether
the divergence looks deliberate or accidental. It is what U10b and U10c act on.
**U9 changes none of these**, except by deleting the duplicated per-swing melee
defence roll, which is a defect rather than a firing convention.

Two open questions go into the audit rather than being decided here:

1. Spec §5.0's ordinary row awards the defender on hit *or* miss, while melee
   today awards only a defence that actually registered.
2. **Melee and the channel path already disagree with each other**: melee gates
   on a defence having registered, while `resolveChannelDefenceWithRunner`
   awards the best defence whether it won or lost. Two live conventions for one
   question.

Both are firing conditions, so both are U10b's call.

### 9.1 A resolution-layer divergence found during execution — U10b

Surfaced 2026-08-19 by the guard test in §10, when the owner asked why the
spell path needed its own crit handling at all.

**A critting spell skips the defence contest entirely.**
`internal/hooks/spell_resolution.go` reads:

```go
defence := combat.ChannelDefenceResult{DamageMultiplier: 1}
if !isCrit {
    defence = runPlayerSpellDefence(spellAttackChannel(spellData), ...)
    ...
}
```

`isCrit` is decided from the attack roll BEFORE any defence is contested, and
on a crit `runPlayerSpellDefence` never runs. So **quell cannot answer a
critting spell**, the defender earns no defence progression from it, and the
crit is applied at full magnitude with no contest.

That is precisely the trap the parent contest spec's own list names: *"An attack
crit forces a hit. Any crit adjustment evaluated before the hit outcome is final
becomes an undeclared second hit floor."* Melee does not work this way — there,
crit is derived from the contest margin, so the contest always runs.

**U9 does not change it**, because letting quell mitigate critting spells alters
spell damage against every defender with the skill, which is a balance change
needing modelling and a playtest. It is recorded here as **U10b's**, alongside
the firing-condition work, since it is the same class of problem: two paths
answering one question differently.

U9 did unify the **progression** half — all three previously-unrouted
crit-received sites (two in `spell_resolution.go`, one in `combat_taunt.go`) now
go through the seam, so what the event carries is consistent even while when the
contest runs is not.

---

## 10. Testing

- **Table tests on `progression.EventsForContest`** — all five matrix rows, both
  sides, floored and unfloored, using plain values. This is the payoff for
  keeping the package pure.
- **Guard test** that `internal/combat` and the combat hooks fire progression
  only through `characters.ApplyProgression`, on the model of U5b's
  pool-mutation AST guard. Scoped to the contest paths; the ~50 non-contest
  sites are explicitly allowed and listed.
- **Rate regression** pinning events-per-round and effective chance for the
  crit-received path at virtual ranks 0, 75 and 150, so a later retune cannot
  quietly reopen the faucet. Note that a Go test binary never loads
  `config.yaml`, so the test must inject balance values rather than read them.
- **Registry move** is covered by the existing `costs` tests continuing to pass
  unchanged through the alias.
- Existing progression tests must pass unchanged except where §7 names the
  change.

---

## 11. Documentation

Per standing rule 2, all of this ships in the same PR.

- `context.md` for `internal/progression` (new), `internal/actionspec` (new),
  `internal/costs`, `internal/characters`, `internal/combat`, `internal/spells`.
  Every symbol verified to exist before it is documented.
- `internal/spells/context.md` currently records `PrimaryStat` as "INERT.
  Parsed, never read." That line must go.
- `docs/PATCH_NOTES.md` — a dated, player-facing entry. No raw numbers.
- `_datafiles/config.yaml` — both new knobs documented beside their values.
- `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`:
  - U9 row rewritten to what shipped.
  - **New U10b row:** progression firing consistency pass, plus Category C
    routing. After U10, before U12.
  - **New U10c row:** charm redesign. See §12.
  - U8 row corrected. It still reads "integration pending"; U8 merged as
    `15a5fc94d` on 2026-08-18.
- `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md` — one item: progression banner
  frequency across a real ten-round fight, since the rates in §7.1 move in both
  directions and the felt effect is what matters.

---

## 12. U10c — the charm redesign (recorded, not built here)

Found while scoping §8. `internal/hooks/charm_spell.go:51` scores charm as
`Charisma + GetSkillLevel(Manifestation) * 25`. That is a **skill weight of 25**
against U6's uniform 5.0, so it survived the flip and violates standing rule 5
("no legacy parameter survives U6").

**Why it is high, per the owner, 2026-08-19:** the original design had the
charmed entity re-roll a resist periodically, at escalating difficulty. That
ladder exists — `internal/hooks/NewRound_MobRoundTick.go`, `tickMobCharmState`:

```go
effectiveness := 1.0 - float64(comp.CharmRerolls)*0.01*float64(comp.CharmRerolls)
if effectiveness < 0.50 { effectiveness = 0.50 }
```

The penalty is quadratic in re-roll count but **floors at 50%**, reached after
eight renewals, so even on its own terms it would stop escalating.

**But it never runs at all.** The whole re-roll block is gated on
`comp.CharmDuration > 0`, and the success branch of `resolveCharmSpell`
(`charm_spell.go:82-98`) calls `Charm(user.UserId, 99999, "")` and builds a
`CompanionInfo` that **never sets `CharmDuration`**, so it is zero. The only
other assignment to that field is inside the ladder itself, which cannot reach
it. A repo-wide search finds no third writer.

So a freshly charmed mob today is charmed for 99999 rounds with no resist check,
ever. The ladder can only fire on a save carrying a non-zero `charm_duration`
from an older build. The ×25 weight is therefore buying survivability against a
mechanic that does not run — a pure leftover, not a compensating balance.

**No migration is needed.** Owner, 2026-08-19: no veteran player uses charm,
because the conjure and raise options are strictly better, so U10c is free to
rewrite how the spell works without preserving any live charm state.

**Owner decision:** redesign charm with a real duration, remove the re-roll
ladder, and return the weight to uniform 5.0. Its own slice because it is a
mechanics redesign rather than a progression change, and because dropping a
four-figure attack score to a three-figure one needs modelling and a playtest,
not a sweep inside another slice.

### 12.1 U10c also owns charm's DEFENCE stat

Charm is the **only manifestation spell that attacks another character** (owner,
2026-08-19). The other thirteen create or raise companions and declare
`target_defense_type: none`, so they contest nothing at all. This is also why
§8.1 can leave `ChannelAttackScore` on Willpower safely: no manifestation spell
ever reaches it.

That makes charm's defence a real design question rather than a consistency one.
It currently resists on `Willpower + 10% of the target's total stat training`
(`charm_spell.go:63-64`), and **either Willpower or Charisma is defensible**:
charm is a working pressed against a mind, which argues Willpower, but it is
also an act of social domination whose attack side already runs on Charisma,
which argues the defender's Charisma should answer it.

U9 changes nothing here. It is recorded because U10c is rewriting this exact
function and should decide deliberately rather than preserving the current stat
by inertia.

Note the interaction with the U6 defence set: charm does not use one. It never
reaches `ResolveChannelDefence`, so neither `quell` nor `defy` answers it. If
U10c wants charm answered by a named defence rather than a bespoke score, that
is a row in `DefenceSetFor` plus a channel — a larger change than swapping the
stat, and worth costing separately.

---

## 13. Open items for spec review

1. Category C's home. Recorded as **U10b** per owner direction. Worth one more
   look: U10 is disruption-shaped, and crafting and salvage are not, so U10b is
   the better fit and is where this spec puts it.
2. Whether the `actionspec` move should carry `SkillSource` and `Physical` with
   it, or whether those stay cost-specific. This spec moves the whole struct,
   since splitting it would put two halves of one action's definition in two
   packages.
