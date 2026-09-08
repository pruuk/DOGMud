---
name: dogmud-combat
description: Use when touching damage, defence, hit resolution, opposed contests, or mob combat AI. The model half covers the five-factor damage formula, the three channels and their shipped config scales, mitigation caps, best-of-all defence resolution, and that combat.RunContest is the single entry point for every opposed contest. The placement half covers where combat logic goes, that btree events fire before legacy AI, and using actions.ResolveTargetActor rather than reimplementing target resolution.
---

This skill has two halves that fire on the same trigger but serve different
moments. The **model half** (damage formula, channels, mitigation, defence,
contests, dice) is what you need when you are reasoning about what a number
does or should do. The **placement half** (where combat code goes) is what
you need when you are actually writing or moving that code. Skip whichever
half you do not need; the headings below mark the boundary.

## Read config.yaml, never the Go defaults

Every balance number in this document is a config value, not a fixed
constant, and several ship far from their Go default. State the rule before
looking at any number below.

Lifted verbatim from CLAUDE.md's "Balance Lives in config.yaml, Not in Code"
section:

> **Never quote a Go default as a live value.** Defaults are fallbacks applied
> only when the key is absent from `config.yaml`. Several shipped values differ
> sharply from their defaults (`SpellDamageScale` ships at 3.12 against a
> default of 1.0). Read `config.yaml` for what the game actually does.
>
> **Absence is meaningful.** A knob left out of `config.yaml` falls back to its
> Go default, and `0` is a legal shipped value (`StaminaPerStrength: 0`). Do
> not assume a missing key means "unset" or "zero".

Every table in this skill that carries two numbers per row labels which one
is the Go default and which one is shipped. If you ever produce a combat
number without that label, you have reproduced the exact bug this project
keeps warning about.

## The damage formula

Lifted verbatim from CLAUDE.md's "Unified Damage & Mitigation Pipeline
(Stage 34)" section:

### Unified Damage & Mitigation Pipeline (Stage 34)
All damage flows through a three-channel pipeline in `internal/combat/damage_pipeline.go`:

### Damage Formula
All channels use the same unified formula, which has **five** factors, not four:
```
raw = stat × SkillMultiplier(rank) × itemMult × ChannelScale × GlobalDamageMultiplier
```
`GlobalDamageMultiplier` is a master knob applied to every channel
(`damage_pipeline.go:78`). It was missing from this table until 2026-08-04, so
any figure computed from the old four-factor version was wrong by whatever the
knob was set to.

**ChannelScale is a config value, not a constant.** `DamageScale()` reads it per
call from the balance config, so the scales below change whenever `config.yaml`
changes. Two sets of numbers matter and they are not the same:

| Channel    | Go default | Shipped in `config.yaml` (2026-08-04) | Knob |
|------------|-----------|----------------------------------------|------|
| Physical   | 0.30      | **0.52**                               | `MeleeDamageScale` |
| Magical    | 1.00      | **3.12**                               | `SpellDamageScale` |
| Conviction | 1.00      | **3.00**                               | `RhetoricDamageScale` |

`GlobalDamageMultiplier`: Go default 1.0, **shipped 0.5**.

Real math at stat=100, rank=0, using the *shipped* values. Note the third
factor differs per row: Physical and Magical use `itemMult=1.0`, while
Conviction has no item multiplier at all and the 0.5 in its row is the fixed
taunt base.

| Channel    | Calculation                          | Raw    |
|------------|--------------------------------------|--------|
| Physical   | 100 × 1.0 × 1.0 × 0.52 × 0.5         | **26** |
| Magical    | 100 × 1.0 × 1.0 × 3.12 × 0.5         | **156**|
| Conviction | 100 × 1.0 × 0.5 × 3.00 × 0.5         | **75** |

Do not quote the Go defaults as if they were live values. Read `config.yaml`.
Note also that a knob left *absent* from `config.yaml` falls back to its Go
default, so absence is meaningful.

Then `ApplyMitigation(raw, mitigation%, cap)` and `dice.RollStat(final)` for variance.

### Three Channels
| Channel | Stat | Skills | Item Field | Mitigation Method |
|---------|------|--------|-----------|------------------|
| Physical | Strength | weapon/unarmed/ranged-combat | `damage_multiplier` (weapon) | `GetPhysicalMitigation()` |
| Magical | Willpower | spellcasting | `damage_multiplier` (spell) | `GetMagicalMitigation()` |
| Conviction | Charisma | rhetoric | 0.5 (taunt base) | `GetConvictionMitigation()` |

> Note: the `shoot` command path (`ExecuteFire`) uses **Perception** for
> both hit and damage rolls: aimed shots are deliberate-move actions, not
> auto-attack swings. The Strength entry above applies to melee auto-attacks
> and mob basic attacks only.

### Skill Multiplier Curve
`mult = base + (max - base) × sqrt(rank / softCap)`: Config: `SkillMultiplierBase` (1.0), `SkillMultiplierMax` (3.0)

### Item Mitigation Fields (replaces old single DamageReduction)
Items use `physical_mitigation`, `magical_mitigation`, `conviction_mitigation` (integer percentages).
The legacy `DamageReduction` field was fully removed 2026-08-03 (its three
side-jobs migrated: shield classification keys on `physical_mitigation > 0 ||
subtype wearable`, the item-value formula prices the three mitigation fields,
and the dead upstream `Item.Enchant` that wrote it was deleted).

### Mitigation Caps
Default 75% each: `PhysicalMitigationCap`, `MagicalMitigationCap`, `ConvictionMitigationCap`

### Key Functions
- `combat.CalcRawDamage(stat, skillRank, itemMult, channel)`: compute raw damage
- `combat.ApplyMitigation(raw, pct, cap)`: apply percentage reduction
- `combat.SkillMultiplier(rank)`: sqrt curve from config
- `combat.ResourceMultiplier(current, max, penaltyMax)`: smooth resource depletion penalty
- `character.GetPhysicalMitigation()` / `GetMagicalMitigation()` / `GetConvictionMitigation()`: sum equipment

## Mitigation and resource penalties

Lifted verbatim from CLAUDE.md's "Resource Depletion Penalties (Stage 35)"
section:

### Resource Depletion Penalties (Stage 35)
Smooth curve replaces old hard-cutoff stamina penalties. As any resource pool
drains, a multiplier reduces combat effectiveness gradually:
```
mult = 1 - maxPenalty × (1 - ratio)^curve
```
Config knobs: `ResourcePenaltyCurve` (default 2.0), per-pool `HealthPenaltyMax`,
`StaminaPenaltyMax`, `ConvictionPenaltyMax` (all default 0.28).

| Resource % | Multiplier | Penalty |
|-----------|------------|---------|
| 100%      | 1.000      | 0%      |
| 50%       | 0.930      | 7.0%    |
| 25%       | 0.843      | 15.7%   |
| 5%        | 0.747      | ~25%    |
| 0%        | 0.720      | 28%     |

Mapping: Stamina → attack count + hit rate, Health → melee damage,
Conviction → taunt hit/damage + spell damage.

## Defence

Lifted verbatim from CLAUDE.md's "Defense Resolution: Best-of-All (Stage 35)"
section:

### Defense Resolution: Best-of-All (Stage 35)
Defense is resolved by rolling **all** available defenses (dodge, parry, block)
and picking the one that won by the widest margin. This replaces the old
sequential short-circuit approach where dodge was always checked first.
Benefits: every defense type gets fair representation in combat text, and
having multiple defense types is always better (wider net).

**Defense Floor**: `MinDefenseChance` (default 0.15) ensures even massively
outclassed defenders have a 15% chance to avoid any swing. This prevents
fights from feeling like guaranteed hits when stat gaps are large.

### Hit chance is decoupled from mitigation

[[reference_hit_chance_decoupled_from_mitigation]]: hitting with an attack,
a spell, or a taunt is entirely decoupled from the mitigation for that
attack. A quick, lightly armored foe is hard to hit and takes big damage
when hit; a heavily armored foe is slow and easy to hit and takes less
damage when hit. A to-hit or evade roll may read agility-type stats and
defensive skills, and gear may influence it only through purpose-built
avoidance ratings (`ParryRating`, `BlockRating`, `GrappleModifier` on
`ItemSpec`). It must never read a mitigation getter, either directly or
through a helper that folds one in.

**This rule is still true; its evidence in the memory file is stale as of
2026-09-08.** The file names `spellDefenseValue` in
`internal/hooks/spell_resolution.go` and `Character.GetDefense()` as the
things to check are absent. Both were deleted (`spellDefenseValue` under
U6b; `GetDefense()` is gone from `internal/characters`), and the legacy
`DamageReduction` field the file also names was fully removed 2026-08-03,
already noted verbatim above in this skill's "Read config.yaml" and
damage-formula lift; the memory file's caution to check for a live
`DamageReduction` read is accordingly moot too.

The current seam: defence scores for a to-hit roll come from
`Character.GetDefenseScoreFor(defenseType string, includeSkill bool)
float64` (`internal/characters/combat.go:278`). The comment on
`spellAttackChannel` (`internal/hooks/spell_resolution.go`) states this
directly: "the defender's score comes from `GetDefenseScoreFor` via the
seam, the deleted defence-value helper's raw-stat read is gone with the
two-contest gate." `GetDefenseScoreFor` is stat-and-skill based (dodge,
parry, block, quell, defy), not mitigation-based, so the design rule
still holds through the current code, it just holds through a different
symbol than the memory file names. Use `GetEffectiveDexterity()`, not
`Stats.Dexterity.ValueAdj`, in any live roll:
`internal/characters/effective_stats.go` documents that raw `ValueAdj` is
for training/progression/display only. If you add or touch a to-hit roll,
confirm it calls `GetDefenseScoreFor` (or another stat/skill-based score)
and does not read `GetPhysicalMitigation()`, `GetMagicalMitigation()`,
`GetConvictionMitigation()`, or any other mitigation getter. Treat the code
as authoritative over the memory file's specific symbol names.

## Contests and dice

Lifted verbatim from CLAUDE.md's "Dice & Rolling System" section:

### Dice & Rolling System
- **For an opposed contest, call `combat.RunContest`** in
  `internal/combat/run_contest.go`. It is the single entry point for every
  opposed contest in the game and the only place `Balance.ContestFloor` is
  read. Signature:
  `RunContest(atkScore float64, entries []contest.Entry) contest.Result`; for
  one defender pass `[]contest.Entry{{Score: defScore}}` and read `.Success`.
  There is no floor pair to pick any more: U6 deleted the three per-channel
  wrappers (`RunWithGlobalFloors`, `RunWithManeuverFloors`,
  `RunWithSpellFloors`) because the wrong pick was numerically invisible in
  production and would have become a live balance bug on the first retune.
- `dice.OpposedRollStat` / `OpposedRollStatWithFloors` are **deprecated**. U4
  moved every production caller off them and U6 deletes them.
  `contest.Run` and `contest.AgainstDifficulty` are **unfloored**; both root
  guard tests fail a new production caller.
- **Concentration contests are a separate seam.** Damage, position, and
  throttle disruption triggers all run through `combat.RunConcentrationContest`
  (`internal/combat/run_concentration_contest.go`), not `RunContest`. It is
  floored by `Balance.ConcentrationFloor` (0.02), a deliberately smaller mercy
  band than the standard `ContestFloor`.
- `dice.RollStat(mean)` is still correct for a single non-contested roll, with
  no stdDev argument needed.
- These `dice` wrappers automatically apply the global `RollSpread` factor: `stdDev = mean × RollSpread`
- `dice.Roll(mean, stdDev)` / `dice.OpposedRoll(atk, def, stdDev)` are low-level; only use them when variance is NOT stat-proportional (e.g., weapon damage variance from item specs)
- **`RollSpread`** is the single master randomness knob, set in `_datafiles/config.yaml` under `GamePlay.RollSpread` (default **0.15**). Changing it rescales every dice roll in the engine. See `internal/dice/README.md` for win-probability tables.
- Z-score thresholds: `ZScore >= 2.0` = crit; `ZScore <= -2.0` = fumble/backfire (~2.3% each, unaffected by `RollSpread`)
- `util.Rand` / `util.LogRoll` are NOT used for hit or attack checks; only `dice.*` functions

## Design conventions

Lifted verbatim from CLAUDE.md's "Combat Design Conventions" section:

### Combat Design Conventions
- **Prefer multipliers over flat bonuses/penalties.** Multipliers scale with
  character power and are easier to tune. Flat values create balance problems
  at different power levels (too strong at low stats, irrelevant at high stats).
- Prone effects use multipliers: `ProneAttackMultiplier` (default 0.80),
  `ProneVulnerabilityMultiplier` (default 1.15), `ProneDodge/Parry/BlockPenalty`.

## Standing combat facts

Two more facts from the model half, both about mechanics that sit next to
the formulas above rather than inside them.

[[reference-standing-combat-and-balance-facts]] records several standing
facts that outlived the contest-resolution roadmap arc and are not restated
anywhere in CLAUDE.md:

- **A crit is a normalized opposed-roll margin, not a flat rate.** The
  configured threshold (Z-score 2.0, from the "Contests and dice" section
  above) is the bar, not the rate. The familiar 2.3%/6.7% figures hold only
  at parity and say nothing about a mismatched fight. `SkillWeight` ships
  at 5.0, and every mob is combat skill 1.
- **There are five defences, not three.** Dodge, parry, and block are the
  three covered by the "Defence" section above; quell answers mental spells
  and defy answers social attacks, and both cost CP, not SP, which is why
  grepping for a stamina cost on them finds nothing. Exhausted actors still
  act; they just lose the skill term.
- **Instance mobs scale with gold**: `pool = goldPaid × template statpool`,
  capped at 50000, with oasis royalty at times 4. Gold is the difficulty
  dial for any mob spawned this way.
- **`Heal()` is a harm path.** `ComputeTickAmount` goes negative when
  `TickPercent < 0`, so wrapping it over `ApplyRestore` to "tidy" it
  silently deletes every health damage-over-time effect in the game.
- **Reflect lives on the species record** (`return_damage` plus
  `return_damage_channel`), not on buffs and not in the mob file. Equipment
  `return_damage` is a separate mechanism: physical and uncapped.
- **False claims not to remake**: there is no prone/stand death spiral; an
  exhausted defender is not auto-hit; the melee attack score is not
  Dexterity; a mob's `Training` is only the spawn pool.

[[reference-taunt-hold-aggro-gate]]: a taunt pins a target's aggro for
`Balance.TauntHoldRounds` (default 4) via a lock on `Character`
(`tauntHoldUntilRound`, `tauntHoldUserId`, `tauntHoldMobInstanceId`).
`Character.SetAggro` has a gate that, while a hold is active, ignores
`DefaultAttack`/`Shooting`/`SurpriseAttack` re-aggro to a different
target. Before this mechanism existed, ~14 combat archetypes ran a
reactive `attack` btree action on `mob_hurt`/`packmate_hurt`, and that
action's own `SetAggro` call overwrote the taunt every round the instant
an ally hit the taunted target. If you add a new `SetAggro` caller and
aggro "won't switch," check whether a taunt hold is blocking it before
assuming a bug; that is intended. `SpellCast`/`Flee`, same-target sets,
and a newer taunt (which re-locks first) all pass through the gate.
AoE/multi-target harm casts still hit everyone regardless of taunt; that
is inherent to those actions and not something this gate fixes.

**The memory file's setter is stale as of 2026-09-08.** It names
`Character.ForceTauntAggro` as the single call that sets the lock and
then engages. `ForceTauntAggro` was deleted under U12a and split into two
current symbols: `Character.SetTauntHold(userId, mobInstanceId,
holdRounds int)` (`internal/characters/taunt_hold.go`) sets the lock only
and does not engage, and `targeting.CommitTaunt(c *characters.Character,
ref state.ActorRef, holdRounds int) bool` (`internal/targeting/commit.go:124`)
calls `SetTauntHold` first and then commits the engagement, in that
order, because the lock must exist before the commit for the taunt's own
set to pass the gate it just armed. `ExecuteTaunt`
(`internal/actions/combat_taunt.go`) calls `targeting.CommitTaunt` today,
not the deleted helper. A guard test,
`internal/actions/ambush_parity_guard_test.go:335`, actively fails the
build's test suite on any new `ForceTauntAggro` caller and points at
`targeting.CommitTaunt` instead, so following the memory file's old
symbol would write code the test suite rejects. Treat
`SetTauntHold`/`CommitTaunt` as the current pair; the lock mechanics and
gate behavior described above are otherwise unchanged.

## Where combat code goes

The trigger for this half is the same as the model half above (you are
touching damage, defence, hit resolution, contests, or mob combat AI), but
the question changes from "what does the number do" to "where does the code
go." These seven rules are all placement rules, not balance rules.

[[feedback_combat_logic_goes_in_handleCombatRound]]: all future
combat-round logic belongs inside `handleCombatRound`
(`internal/hooks/NewRound_DoCombat_unified.go`) or its eight phase helpers
(`resolveCombatTarget`, `phase1WaitRound`, `rollCombatAttack`,
`applyCombatDamageBonuses`, `dispatchCritAndMessaging`,
`applyCombatProgression`, `fireDefenderBehaviorTrigger`,
`handleAggroAndAssist`, `resolveCombatRound`). Do not add new
quadrant-specific handler functions (`handlePlayerVs*`, `handleMobVs*`); the
four-handler structure was deliberately collapsed on 2026-04-18 to make
parity gaps structurally impossible, after five distinct combat-quadrant
parity bugs accumulated across the four parallel handlers. Genuine
quadrant-specific logic (player-only text dispatch, mob-only behavior
trees, party-only assist) is gated with `atk.IsPlayer()` / `def.IsPlayer()`
checks at the leaf site, plus a `// Divergence #N: <reason>` comment tied to
the numbered list in
`docs/superpowers/specs/completed/2026-04-18-combat-quadrant-unification-design.md`
(the memory file cites this path without the `completed/` segment; the spec
moved there and the numbered list, items 1 to 9 with item 10 struck out, is
otherwise unchanged as of 2026-09-08).
Tests for new combat behavior should drive through `handleCombatRound`
end to end, not call a phase helper directly.

[[feedback_btree_combat_events_before_legacy_ai]]: a mob-attacker behavior
tree event that needs to own this round's action decision must fire before
`handleMobAIDecision` (at `internal/hooks/NewRound_DoCombat.go:395` as of
2026-09-08; the memory file's line 276 now holds unrelated
guard-prisoner/`NoAggroTarget` logic and is stale), not at the top of
`handleCombatRound`. `handleMobAIDecision` returns `true`
when it picks a move and the per-mob loop `continue`s on that, so
`handleCombatRound` never runs for that mob and the legacy hardcoded
priority ladder in `preferredSpell` (shield, then heal, then harm-list)
preempts any archetype self-buff every round. The fix pattern is to fire
the btree event and `continue` on a handled result immediately before the
`handleMobAIDecision` check. Defender-side events (`mob_hurt` and similar)
do not have this problem; they fire inside `handleCombatRound` against a
defender whose AI decisions already happened upstream.

[[feedback_btree_death_actions]]: `mob_die` behavior tree handlers must use
`send_room_text`, not `respond` or any action that looks up the mob
instance. The mob is destroyed shortly after the event fires, so a delayed
or mob-dependent action queued from `mob_die` fires against a dead or nil
mob and produces no output. `set_room_locked` and `spawn_item_in_room` are
fine because they operate on the room, not the dying mob.

[[feedback_best_of_actions_are_synchronous]]: a btree action that relies on
returning `Failure` so a parent selector falls through to the next child
(`cast_best_in_category`, `command_best_of`, and any future `*_best_of`
action) must be registered in `actionRegistry` but must NOT also be
registered in `delayedActions`. The perception-scaled delay wrapper in
`ActionNode.Evaluate` returns `Success` unconditionally after queuing,
which discards the action's real return value and breaks every archetype
that depends on the fallthrough. Both known actions have broken this way
once before; flag a PR that touches both maps for a single action.

[[feedback_target_resolution_uses_actor]]: user-facing target resolution
in commands goes through `actions.ResolveTargetActor(room, name, opts...)`
(`internal/actions/target_resolution.go`), which returns a concrete
`*actions.UserActor` or `*actions.MobActor` (both satisfy `actions.Actor`)
or one of `ErrTargetNotFound`, `ErrTargetVanished`, `ErrTargetSelfExcluded`.
Do not reimplement the `room.FindByName` plus `mobs.GetInstance` /
`users.GetByUserId` plus nil-check chain in a new command; it was
reimplemented roughly 37 times with subtle variations, including a latent
nil-deref crash, before the helper existed. Downstream lookups by an
already-known ID (an Aggro field, an event payload) are not name
resolution and stay as direct `mobs.GetInstance` / `users.GetByUserId`
calls. Do not extend the `Actor` interface to cover mob-only or user-only
behavior; type-assert at the leaf instead
(`target.(*actions.MobActor).Mob...`).

[[feedback_companion_autonomy]]: companions are intentionally autonomous.
Never propose an `order <companion> <command>` or similar direct-control
surface, including for combat abilities. When a parity gap suggests "the
player should be able to make the companion do X," the correct fix is
smarter companion AI that chooses to do X on its own, not a new command.
The existing `companion <name> assist on/off` toggle is the one accepted
exception, because it is a posture knob, not an action dispatch.

[[feedback_combat_quadrant_parity]]: when touching combat code and you
notice a behavioral divergence between the player/mob quadrants that is
not clearly AI, behavior-tree, or dialogue related, treat it as a
potential parity bug. A quick fix (under 30 lines, no new config knob, no
gameplay rebalance, no cross-system ripple) gets fixed inline in a
preceding `fix(combat): parity - <desc>` commit. A non-quick gap gets
logged to `project_pvm_mvp_parity_gaps.md` rather than silently preserved.
An ambiguous case (bug versus intentional) gets a pause and a question
rather than a guess.

## Sources

Model half, lifted from CLAUDE.md:
- "Dice & Rolling System" (lines 426-453)
- "Unified Damage & Mitigation Pipeline (Stage 34)" (lines 488-561)
- "Resource Depletion Penalties (Stage 35)" (lines 562-581)
- "Defense Resolution: Best-of-All (Stage 35)" (lines 582-592)
- "Combat Design Conventions" (lines 593-599)
- "Balance Lives in config.yaml, Not in Code," rule 2 only (lines 476-479)

Model half, folded memory files:
- [[reference_hit_chance_decoupled_from_mitigation]]
- [[reference-standing-combat-and-balance-facts]]
- [[reference-taunt-hold-aggro-gate]]

Placement half, folded memory files:
- [[feedback_combat_logic_goes_in_handleCombatRound]]
- [[feedback_btree_combat_events_before_legacy_ai]]
- [[feedback_btree_death_actions]]
- [[feedback_best_of_actions_are_synchronous]]
- [[feedback_target_resolution_uses_actor]]
- [[feedback_companion_autonomy]]
- [[feedback_combat_quadrant_parity]]
