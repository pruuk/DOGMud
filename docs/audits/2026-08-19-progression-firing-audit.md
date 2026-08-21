# Progression Firing Audit (2026-08-19)

Input for U10b, which will impose one rule on top of this survey: **one
progression event per success, with crit and critical-failure as a separate
bonus on top.** This document does not implement that rule. U9 (the slice
this audit was written for) changes what a progression event CARRIES
(context, magnitude), never WHEN it fires. Every firing condition below is
cataloged as-is, not fixed.

Methodology: grep for every production (non-test, non-comment) call of
`OnSkillUse`, `OnSkillUseScaled`, `OnStatUse`, `CheckSkillProgression`,
`CheckStatProgression`, `OnCritReceived`, `OnCriticalSuccess`,
`OnCriticalFailure`, `TrackSkillUse`, `TrackStatUse` under `internal/` and
`modules/`, then read the surrounding function at each hit to determine the
real firing condition (grep alone cannot tell "on hit" from "on miss").

## Count, and how it differs from the plan's 96/46

The plan text estimated **96 `OnSkillUse`/`OnSkillUseScaled`/`OnStatUse`
sites across 46 files, plus crit/track entry points**. The actual count,
re-verified against source on this date:

- **93** production call sites for the core three
  (`OnSkillUse`/`OnSkillUseScaled`/`OnStatUse`) across **45** files.
- **32** additional sites for the crit/track/check entry points
  (`CheckSkillProgression`, `CheckStatProgression`, `OnCritReceived`,
  `OnCriticalSuccess`, `OnCriticalFailure`, `TrackSkillUse`, `TrackStatUse`).
- **125** total sites across **50** files.

Of the 93 core sites, 2 (`internal/characters/progression.go:229` and `:254`)
are not independent gameplay triggers at all - they are `OnSkillUse`'s own
internal dispatch (`OnSkillUse` calls `OnSkillUseScaled`; `OnSkillUseScaled`
auto-fires `OnStatUse` on the skill's primary stat). They are listed below
and marked as mechanism, not as an independent firing convention, so they
should not be double-counted against the "seven conventions" tally.

I could not reconstruct exactly which methodology produces 96/46 - my
93/45 for the same three functions is close but not identical, and I did
not find a plausible variant of the grep that lands on 96/46 exactly. Every
row below was individually verified against the current file content on
this date, so treat 93/45 (or 125/50 including the crit/track cluster) as
the current, checked number.

**Addenda found while reading, not caught by the task's ten grep
patterns:** reading `internal/characters/progression.go` end to end (rather
than only the lines the grep hit) surfaced two more exported progression
entry points the ten patterns do not name at all:

1. `OnFirstMobKill`, called from two real production sites in
   `internal/hooks/Death_MobKillCredit.go` (lines 61 and 86) on a player's
   first-ever kill of a given mob type (gated on `KD.GetMobKills(mobId)==1`,
   excluded in the `Training` zone).
2. `OnRegenTick` / `CheckRegenProgression`, called from **six** real
   production sites in `internal/hooks/NewRound_AutoHeal.go` (lines 272,
   275, 278 for players; 368, 371, 374 for mobs) - see Finding 5, this is
   substantial enough to warrant its own section below.

Neither `OnFirstMobKill(` nor `OnRegenTick(`/`CheckRegenProgression(` was
one of the ten patterns given, so these sites are invisible to a mechanical
run of the specified grep and would have been silently missed. Counting the
same way the rest of this audit counts (every production call site, plus
the internal plumbing/definition lines the call sites route through, get a
row): 2 `OnFirstMobKill` call sites + 6 `OnRegenTick` call sites + 2
plumbing lines (`CheckRegenProgression` and `OnRegenTick`'s own bodies at
`progression.go:368` and `:424`, neither of which route through any of the
ten original patterns) = 10 new rows. **Adding them: 135 total sites across
52 files** (125 + 10; +1 file for `Death_MobKillCredit.go`, +1 file for
`NewRound_AutoHeal.go`). This is worth flagging on its own - if the task's
own pattern list missed two real, distinct firing conventions (one of them
a completely different mechanism class, passive and tick-based rather than
use-based), other progression entry points not named in this task may
exist uncaught too. I did not do an exhaustive
symbol-level search of every method on `Character` beyond what surfaced
while reading `progression.go` for this audit, so treat "every production
progression call site" as strong-but-not-airtight; a follow-up worth doing
is `codegraph_callers` on every exported method of `Character` whose name
contains "Progression", "Skill", "Stat", or "Regen".

## Distinct firing conventions found: ten

Counting only genuinely distinct "when does this fire" shapes (not
counting the internal-plumbing sites as a convention of their own):

1. **On clean hit only** (melee autoattack, all eleven special moves,
   ranged skill-on-hit half of `shoot`, defence progression's per-swing
   half restricted to a *won* defence).
2. **Always, regardless of outcome** (`throw`, `taunt`, surprise attack,
   shadow, defuse, plant, steal, venom-coat, consider, look, assess,
   `shoot`'s perception stat, buy/sell bartering gated only by the
   caller already having decided "success").
3. **Gated on "a roll happened," win or lose** (`search`, `track`,
   `sneak` via `CheckSkillProgression` when `RollHappened`, flee via
   `contested`).
4. **In-combat always / out-of-combat 50%** (`warcry`/`rally` via
   `awardRhetoricUse`).
5. **Once per defence TYPE per round, only for a defence that actually
   won** (melee per-round path, `processDefenderProgression`).
6. **Every defended swing, independent of the per-round count** (melee
   per-swing path, `sendDefenseMessages`).
7. **Whenever the contest ran, win or lose** (channel/spell defences,
   `resolveChannelDefenceWithRunner` - explicitly documented in source as
   matching the melee *per-round* convention's unconditional-progression
   intent, not the per-swing one).
8. **On success only, difficulty-scaled** (crafting: `usercommands/craft.go`,
   `mobcommands/craft.go`, both round-tick crafters, `salvage`) - the one
   internally consistent cluster in the whole audit, except for
   `mobs/crafter.go`'s two sites which use the plain unscaled call instead of
   the scaled one everywhere else in this cluster (see Category C below).
9. **First-time-only, gated on kill-credit bookkeeping, not a contest**
   (`OnFirstMobKill`, first kill of a given mob type - see the addendum
   above; outside the task's ten grep patterns).
10. **Passive, tick-based, not use-triggered at all** (`OnRegenTick`, every
    3 rounds per resource pool below max, scaled by depletion - see
    Finding 5; also outside the task's ten grep patterns, and the only
    convention here where "did the player do anything" is not part of the
    gate).

That is ten if defence's per-round/per-swing/channel split is counted as
three shapes (which it should be, since Finding 1/3 treat it as a live
three-way disagreement) and the two addenda conventions are included, or
six broad shapes if the combat-adjacent ones are lumped together and the
addenda are set aside as "outside the named scope." The plan text says
seven; I count eight distinct shapes strictly within the task's ten grep
patterns, rising to ten once the two addenda conventions (found by reading
`progression.go` rather than by the specified grep) are included. This is
exactly the kind of miscount U10b exists to end - and the addenda are a
reminder that a firing-condition audit built from a fixed pattern list will
under-count by exactly however many entry points that list forgot.

## Full call-site table

Columns: **Site** (file:line) · **Action** (skill/stat function called) ·
**Skill awarded** · **Stat awarded** · **Fires when** · **Deliberate?**
(yes = source comment/doc explicitly says so; no = looks accidental;
plumbing = internal dispatch, not an independent gameplay trigger) ·
**Notes**.

| Site | Action | Skill awarded | Stat awarded | Fires when | Deliberate? | Notes |
|---|---|---|---|---|---|---|
| `internal/combat/combat.go:77-78` | `OnStatUse` x2 | - | strength, dexterity | Always, every player-vs-mob attacker swing | Yes | Attacker stat use regardless of hit/miss. Phase 2 of the melee duplicate (see Finding 2). |
| `internal/combat/combat.go:87` | `OnSkillUse` | combat skill tag | (chained, see 254) | `attackResult.CleanHit` only | Yes (U6 Task 14 comment) | Deflected swings (partial dmg, defence won) earn nothing. |
| `internal/combat/combat.go:89` | `OnCriticalSuccess` | combat skill tag | - | CleanHit AND crit | Yes | |
| `internal/combat/combat.go:96` | `OnSkillUse` | weapon-combat | - | CleanHit AND dual-wielding a weapon-combat weapon | Yes | Second skill call on the same swing for dual wield. |
| `internal/combat/combat.go:102` | `OnCriticalFailure` | combat skill tag | - | not CleanHit AND fumble | Yes | |
| `internal/combat/combat.go:154-155` | `OnStatUse` x2 | - | strength, dexterity | Always, player-vs-player attacker swing | Yes | Same pattern as :77-78 for PvP. |
| `internal/combat/combat.go:162` | `OnSkillUse` | combat skill tag | - | CleanHit only | Yes | PvP mirror of :87. |
| `internal/combat/combat.go:164` | `OnCriticalSuccess` | combat skill tag | - | CleanHit AND crit | Yes | |
| `internal/combat/combat.go:168` | `OnSkillUse` | weapon-combat | - | CleanHit AND dual-wield | Yes | |
| `internal/combat/combat.go:174` | `OnCriticalFailure` | combat skill tag | - | not CleanHit AND fumble | Yes | |
| `internal/combat/combat.go:186-187` | `OnStatUse` x2 | - | strength, dexterity | Always, mob attacker (`trackMobAttackProgression`) | Yes | Mob mirror of the player attacker stat calls. |
| `internal/combat/combat.go:192` | `OnSkillUse` | per-weapon-hit skill tag | - | `wh.CleanHit`, looped over `result.WeaponHits` | Yes | |
| `internal/combat/combat.go:194` | `OnCriticalSuccess` | per-weapon-hit skill tag | - | `wh.CleanHit` AND `wh.Crit` | Yes | |
| `internal/combat/combat.go:197` | `OnCriticalFailure` | per-weapon-hit skill tag | - | `wh.Fumble` (not CleanHit) | Yes | |
| `internal/combat/combat.go:201` | `OnSkillUse` | unarmed-combat | - | `len(WeaponHits)==0 && result.CleanHit` | Yes | Fallback when no per-weapon breakdown exists. |
| `internal/combat/combat.go:231` | `OnStatUse` | - | dexterity | Always, defender reacting in `AttackMobVsPlayer` | Yes | Defender stat use is unconditional here, unlike defender skill (see defence rows). |
| `internal/combat/combat.go:286` | `OnStatUse` | - | dexterity | Always, defender in `AttackMobVsMob` | Yes | |
| **`internal/combat/combat.go` lines above are "Phase 2" of a two-phase duplicate** | | | | | **No** | **See Finding 2: everything in this file that fires attacker stat/skill/crit progression is executed a second time by `applyCombatProgression` (Phase 5) on the exact same round.** |
| `internal/combat/combat_helpers.go:1227` | `TrackSkillUse` | dodge→unarmed-combat / parry,block→weapon-combat | - | Per-swing, only on a defence that just won that swing (crit-defended or partial-defended) | No (see Finding 1) | Per-swing defender skill, no stat. Called from `sendDefenseMessages`, itself called on every won defensive swing. |
| `internal/combat/combat_helpers.go:1228` | `CheckSkillProgression` | (same as above) | - | Same gate as :1227 | No (see Finding 1) | Bypasses `OnSkillUse`; calls `Track`+`Check` directly, so the primary-stat auto-fire in `OnSkillUseScaled` (progression.go:254) never runs for this path. |
| `internal/combat/defence_multiplier.go:123-124` (`AwardDefenceProgression`, case Dodge) | `OnSkillUse`+`OnStatUse` | unarmed-combat | dexterity | Called once per defence TYPE per round (melee) via `processDefenderProgression`, or whenever a channel contest ran (spell/quell/defy) via `resolveChannelDefenceWithRunner` | Yes for the win-only melee caller; yes-but-different for the always-fires channel caller | Shared function, two very different callers disagree on when to call it (Finding 3). |
| `internal/combat/defence_multiplier.go:126-128` (case Parry) | `OnSkillUse`+`OnStatUse` x2 | weapon-combat | dexterity, strength | Same as above | Same | |
| `internal/combat/defence_multiplier.go:130-131` (case Block) | `OnSkillUse`+`OnStatUse` | weapon-combat | strength | Same as above | Same | |
| `internal/combat/defence_multiplier.go:133-134` (case Quell) | `OnSkillUse`+`OnStatUse` | spellcasting | willpower | Channel path only, whenever the quell contest ran | Yes (documented) | Never reached by the melee per-round caller (quell/defy are channel-only defences). |
| `internal/combat/defence_multiplier.go:136-137` (case Defy) | `OnSkillUse`+`OnStatUse` | rhetoric | willpower | Channel path only, whenever the defy contest ran | Yes (documented) | |
| `internal/hooks/NewRound_DoCombat_helpers.go:44-59` (`processDefenderProgression`, calls `AwardDefenceProgression`) | wrapper | dodge/parry/block per type used | matching stat | Once per round, once per defence TYPE that was actually used (won) in `result.SwingEvents` this round, in fixed Dodge→Parry→Block order | Yes | The "Phase 5" defender-side melee path. Contrast with the per-swing path above (Finding 1) and the channel path (Finding 3). |
| `internal/hooks/NewRound_DoCombat_unified.go:649` | `OnCritReceived` | - | physical stat per `OnCritReceived`'s internal mapping | `res.Hit && res.Crit`, once per round | Yes | Defender-side physical crit-received progression, Phase 5. |
| `internal/hooks/NewRound_DoCombat_unified.go:663` | `OnSkillUse` | per-weapon-hit skill tag | (chained) | `wh.CleanHit`, looped over `res.WeaponHits` | Yes | **Duplicates `combat.go:192`** - same round, same swing data (Finding 2). |
| `internal/hooks/NewRound_DoCombat_unified.go:665` | `OnCriticalSuccess` | per-weapon-hit skill tag | - | `wh.CleanHit && wh.Crit` | Yes | Duplicates `combat.go:194`. |
| `internal/hooks/NewRound_DoCombat_unified.go:668` | `OnCriticalFailure` | per-weapon-hit skill tag | - | `wh.Fumble` | Yes | Duplicates `combat.go:197`. |
| `internal/hooks/NewRound_DoCombat_unified.go:672` | `OnSkillUse` | unarmed-combat | - | `len(WeaponHits)==0 && res.CleanHit` | Yes | Duplicates `combat.go:201`. |
| `internal/hooks/NewRound_DoCombat_unified.go:687` (`emitAttackerStatGain`) | `OnStatUse` | - | strength, dexterity (called twice, once per stat, from `applyCombatProgression`) | Always, every round, attacker side | Yes | Duplicates `combat.go:77-78`/`154-155`/`186-187` (Finding 2). |
| `internal/hooks/NewRound_MobRoundTick.go:496` | `OnSkillUseScaled` | recipe skill, difficulty-scaled | (chained via 254) | Multi-round mob craft, on success roll | Yes | Category C: crafting. Consistent with the other 4 scaled-crafting sites. |
| `internal/hooks/NewRound_UserRoundTick.go:591` | `OnSkillUseScaled` | recipe skill, difficulty-scaled | (chained) | Multi-round player craft, on success roll | Yes | Same cluster. |
| `internal/mobcommands/craft.go:54` | `OnSkillUseScaled` | recipe skill, difficulty-scaled | (chained) | Mob `ImmediateComplete` craft (instant recipe), on success | Yes | Same cluster. |
| `internal/usercommands/craft.go:142` | `OnSkillUseScaled` | recipe skill, difficulty-scaled | (chained) | Player `ImmediateComplete` craft, on success | Yes (comment notes this was a missed spot fixed later) | Same cluster; source comment documents this was once inconsistent with the other three and was fixed. |
| `internal/mobs/crafter.go:505` | `OnSkillUse` (unscaled) | recipe skill | (chained) | Autonomous shopkeeper-mob craft (`executeCraft`, shop-inventory path), on success roll | No | **Breaks the crafting cluster's convention** - every other crafting site uses the difficulty-scaled `OnSkillUseScaled`; this one and the next use plain `OnSkillUse`. New finding, not in the plan's known-divergence list. |
| `internal/mobs/crafter.go:546` | `OnSkillUse` (unscaled) | recipe skill | (chained) | Autonomous shopkeeper-mob craft (`executeCraftLegacy`, backpack path), on success roll | No | Same issue as :505. |
| `internal/actions/salvage.go:166` | `OnSkillUse` | salvage | (chained) | `salvageCorpse`, always once the corpse is committed (item/corpse is consumed even on a zero-yield roll) | Yes | Category C. |
| `internal/actions/salvage.go:252` | `OnSkillUse` | salvage | (chained) | `salvageItem`, always once the item is consumed | Yes | Same. |
| `internal/actions/combat_bash.go:129` | `OnSkillUse` | weapon-combat | (chained) | `result.Hit` | Yes | |
| `internal/actions/combat_drain.go:176` | `OnSkillUse` | unarmed-combat | (chained) | `result.Hit` | Yes | |
| `internal/actions/combat_gore.go:131` | `OnSkillUse` | unarmed-combat | (chained) | `result.Hit` | Yes | |
| `internal/actions/combat_grapple.go:127` | `OnSkillUse` | unarmed-combat | (chained) | `result.Success` (grapple has no separate "Hit" field; Success is its hit-analog) | Yes | Field name differs from the other special moves but the pattern (only-on-hit-equivalent) matches. |
| `internal/actions/combat_hamstring.go:149` | `OnSkillUse` | unarmed-combat | (chained) | `result.Hit` | Yes | |
| `internal/actions/combat_kick.go:187` | `OnSkillUse` | unarmed-combat | (chained) | `result.Hit` | Yes | Covers kick/stomp/knee variants alike. |
| `internal/actions/combat_maul.go:147` | `OnSkillUse` | unarmed-combat | (chained) | `result.Hit` | Yes | |
| `internal/actions/combat_pounce.go:146` | `OnSkillUse` | unarmed-combat | (chained) | `result.Hit` | Yes | |
| `internal/actions/combat_rake.go:146` | `OnSkillUse` | unarmed-combat | (chained) | `result.Hit` | Yes | |
| `internal/actions/combat_throttle.go:172` | `OnSkillUse` | unarmed-combat | (chained) | `result.Hit` | Yes | |
| `internal/actions/combat_trip.go:162` | `OnSkillUse` | unarmed-combat | (chained) | `result.Hit` | Yes | |
| `internal/actions/mutation_venom_coat.go:34` | `OnSkillUse` | weapon-combat | (chained) | Always, on any successful venom-coat cast (self-buff, no roll, combat not required) | Yes | Not a contest at all - a prep move that always "succeeds" once gates pass. |
| `internal/actions/combat_taunt.go:183` | `OnSkillUse` | rhetoric | (chained) | Fumble branch (`res.AttackRoll.ZScore <= -2.0`) | Yes | One of three taunt outcomes, all award. |
| `internal/actions/combat_taunt.go:267` | `OnCritReceived` | - | conviction stat per `OnCritReceived` mapping | `isCrit && target.UserId > 0`, success branch | Yes | Target-side crit-received progression. |
| `internal/actions/combat_taunt.go:270` | `OnSkillUse` | rhetoric | (chained) | Success branch (`res.Success`) | Yes | |
| `internal/actions/combat_taunt.go:325` | `OnSkillUse` | rhetoric | (chained) | Miss branch (fall-through, neither fumble nor success) | Yes | Confirms taunt fires on all three outcomes: fumble, hit, miss. |
| `internal/actions/skill_helpers.go:76` (`awardRhetoricUse`) | `OnSkillUse` | rhetoric | (chained) | `c.IsInCombat()` always, else 50% via `util.Rand(100) < 50` | Yes | Used by warcry and rally. Only shared-probability gate in the whole audit. |
| `internal/actions/consider.go:27` | `OnStatUse` | - | perception | Always, on every `consider` invocation | Yes | Not a contest; a power-ratio read, so "use" is unconditional. |
| `internal/actions/defuse.go:129` | `OnSkillUse` | skullduggery | (chained) | Always, fires BEFORE the disarm contest is even rolled (only gated on having found+consumed a kit) | No (looks accidental relative to steal/plant's identical pre-roll placement, but all three share it, so it reads as a skullduggery-wide house style rather than a one-off bug) | Fires on outcomes that have not happened yet - success or trap-trigger, doesn't matter, the call already ran. |
| `internal/actions/forage.go:142` | `OnSkillUse` | search | (chained) | Only on a successful, valid item find (`coreResult.Found` and item validates) | Yes | Unlike defuse/steal/plant, forage gates on outcome, not just attempt. |
| `internal/actions/plant.go:141` | `OnSkillUse` | skullduggery | (chained) | Always, before the contest roll (`plantOnPlayer`/first path) | No (see defuse note) | Same pre-roll-always pattern as defuse and steal. |
| `internal/actions/plant.go:273` | `OnSkillUse` | skullduggery | (chained) | Always, before the contest roll (second plant path) | No | Same pattern. |
| `internal/actions/plant.go:350` | `OnSkillUse` | skullduggery | (chained) | Always, before the contest roll (third plant path, container) | No | Same pattern. |
| `internal/actions/search.go:243` | `OnSkillUse` | search | (chained) | Gated on `rolledAgainstSomething` (a roll happened against at least one hidden noun this search), independent of whether anything was found | Yes (comment: "anti-botting gate") | Fires even when nothing is discovered, as long as a roll occurred. |
| `internal/actions/sell.go:377` | `OnSkillUse` | bartering | (chained) | Only reached on a successful sale (function returns earlier on failure paths) | Yes | |
| `internal/actions/sell.go:378` | `OnStatUse` | - | charisma (merchant mob) | Same gate as :377 | Yes | Merchant-side charisma use paired with player bartering. |
| `internal/actions/shadow.go:101` | `OnSkillUse` | skullduggery | (chained) | Always, once `shadowMob` begins (no contest - shadowing always starts; a separate detection roll is informational only) | Yes | |
| `internal/actions/shadow.go:150` | `OnSkillUse` | skullduggery | (chained) | Always, once `shadowPlayer` begins | Yes | Same as :101 for the player-target path. |
| `internal/actions/steal.go:156` | `OnSkillUse` | skullduggery | (chained) | Always, before the contest roll (`stealFromMob`) | No (see defuse note) | |
| `internal/actions/steal.go:347` | `OnSkillUse` | skullduggery | (chained) | Always, before the contest roll (`stealFromPlayer`) | No | |
| `internal/actions/steal.go:445` | `OnSkillUse` | skullduggery | (chained) | Always, before the contest roll (`stealFromContainer`) | No | |
| `internal/actions/surprise_attack.go:360` | `OnSkillUse` | skullduggery | (chained) | Always, after resolving all weapon swings regardless of `anyHit` | Yes | Fires even on an all-miss surprise attack. |
| `internal/actions/track.go:128` | `OnSkillUse` | search | (chained) | Every roll fired (skipped only when the target is already visibly present and no roll was needed) | Yes | |
| `internal/actions/buy.go:786` (`postSuccessBookkeeping`) | `OnSkillUse` | bartering | (chained) | Only called from purchase-success branches (3 call sites in `buy.go`) | Yes | |
| `internal/actions/buy.go:789` | `OnStatUse` | - | charisma (merchant mob) | Same gate as :786 | Yes | |
| `internal/hooks/NewRound_DoCombat_helpers.go:321` | `OnSkillUseScaled` | manifestation | (chained) | Player spell `CastComplete`, `spellData.HasSchool(SchoolManifestation)`, AND `spellBonus > 0` | Yes | `spellBonus` is zeroed for a no-target AoE and halved for a self-cast HelpSingle - both explicit guards. |
| `internal/hooks/NewRound_DoCombat_helpers.go:322` | `OnStatUse` | - | charisma | Same gate as :321 | Yes | |
| `internal/hooks/NewRound_DoCombat_helpers.go:324` | `OnSkillUseScaled` | spellcasting | (chained) | Player spell `CastComplete`, non-Manifestation school, `spellBonus > 0` | Yes | |
| `internal/hooks/NewRound_DoCombat_helpers.go:325` | `OnStatUse` | - | willpower | Same gate as :324 | Yes | |
| `internal/hooks/NewRound_DoCombat_helpers.go:469` | `OnSkillUseScaled` | manifestation | (chained) | Mob spell `CastComplete`, Manifestation school | No (new finding - see Category asymmetry below) | **No `spellBonus > 0` guard exists on the mob path.** The self-cast-penalty and AoE-no-target-zeroing guards that gate the player path (:321-325) are absent here; mob spellcasting progression fires unconditionally on `CastComplete`. Not in the plan's known-divergence list. |
| `internal/hooks/NewRound_DoCombat_helpers.go:470` | `OnStatUse` | - | charisma | Same (ungated) as :469 | No | Same asymmetry. |
| `internal/hooks/NewRound_DoCombat_helpers.go:472` | `OnSkillUseScaled` | spellcasting | (chained) | Mob spell `CastComplete`, non-Manifestation | No | Same asymmetry. |
| `internal/hooks/NewRound_DoCombat_helpers.go:473` | `OnStatUse` | - | willpower | Same (ungated) | No | Same asymmetry. |
| `internal/hooks/NewRound_DoCombat_helpers.go:594` | `OnSkillUse` | skullduggery | (chained) | Player flee: `contested && includeSkill` - both a contest happened AND the fleer could afford to bring skill to it | Yes (comment explains both conditions explicitly) | |
| `internal/hooks/spell_resolution.go:823` | `OnCritReceived` | - | magical-channel stat per mapping | Player-cast harm spell, `isCrit`, `spellData.Type` is Harm*, AND `EffectType == "damage"` | Yes | Fires with `target.UserId` which is 0 when the target is a mob - see Notes below the table. |
| `internal/hooks/spell_resolution.go:1448` | `OnCritReceived` | - | magical-channel stat per mapping | Mob-cast harm spell at a player target, `isCrit`, inside the `EffectType == "damage"` case of the resolution switch | Yes | Mob-cast mirror of :823. |
| `internal/usercommands/assess.go:134` | `OnSkillUse` | manifestation | (chained) | Always, at the end of every `assess` invocation (even the "too faint" branch) | Yes | Not a contest - an information command. |
| `internal/usercommands/go.go:388` | `OnSkillUse` | search | (chained) | Every successful room move, gated by `movementTrainsSearch()` - a config-driven probability roll (`MovementSearchTrainChance`), separate from any skill contest | Yes (comment: "a completed move rarely trains search") | Only site in the whole audit gated on a *config knob's own probability roll*, not on a skill/stat contest outcome. |
| `internal/usercommands/look.go:85` | `OnStatUse` | - | perception | Always, whenever `look <target>` resolves an actor | Yes | Not a contest. |
| `internal/usercommands/picklock.go:153` | `CheckSkillProgression` (direct, no `Track`) | skullduggery | - | Auto-pick-from-keyring success branch | Yes (matches the manual path) | Bypasses `OnSkillUse`/`TrackSkillUse` - `SkillUseCount` is never incremented for a picklock success, unlike every other skullduggery site in this audit. |
| `internal/usercommands/picklock.go:262` | `CheckSkillProgression` (direct, no `Track`) | skullduggery | - | Manual entry-matches-sequence success branch | Yes | Same direct-call pattern as :153. |
| `internal/usercommands/shoot.go:197` | `OnStatUse` | - | perception | Always, every `shoot` attempt regardless of hit/miss | Yes | |
| `internal/usercommands/shoot.go:199` | `OnSkillUse` | ranged-combat | (chained) | `hit` only | Yes | |
| `internal/usercommands/skill.skullduggery.sneak.go:75` | `CheckSkillProgression` (direct, no `Track`) | skullduggery | - | Sneak spotted/failure branch, gated on `result.RollHappened` | Yes | Fires on failure as long as a contest happened. |
| `internal/usercommands/skill.skullduggery.sneak.go:85` | `CheckSkillProgression` (direct, no `Track`) | skullduggery | - | Sneak success branch, gated on `result.RollHappened` | Yes | Same direct-call, no-Track pattern as picklock. |
| `internal/usercommands/throw.go:399` | `OnSkillUse` | skullduggery | (chained) | Always, hit or miss, once the throw resolves | Yes | |
| `internal/mobcommands/flee.go:54` | `OnSkillUse` | skullduggery | (chained) | Mob flee: `contested` only (no `includeSkill` gate - mob's `ResolveFleeBlockers` call hardcodes `includeSkill=true`) | Not documented as deliberate the way the player path is | Asymmetric with the player flee gate at `NewRound_DoCombat_helpers.go:594`, which additionally requires `includeSkill`. Plausibly intentional (mobs may not have the same afford-in-full cost gate as players) but the source does not say so explicitly here the way it does for the player path. |
| `internal/mobcommands/sneak.go:19` | `OnSkillUse` | skullduggery | (chained) | Mob sneak, `result.Success` only | Yes (comment: "so mobs can progress skullduggery like players") | Differs from the player sneak path, which fires on `RollHappened` (both success AND spotted-failure) via direct `CheckSkillProgression`, not `OnSkillUse`, and not gated on `Success`. Mobs get narrower firing (success-only) through a different call shape. |
| `internal/behaviortree/actions_progression.go:24` (`actGrantProgression`) | `CheckSkillProgression` with bonus multiplier 1000.0 | scripted skill param | - | Tutorial/scripted trigger only, guarded by a room `set_state` flag to fire once | Yes | Not organic gameplay progression - a forced, guaranteed banner for onboarding. Category D: scripted, not a contest or an activity. |
| `internal/characters/progression.go:215` (`OnStatUse` body) | `TrackStatUse` | - | (the stat passed in) | Every `OnStatUse` call, unconditionally | Plumbing | Internal to `OnStatUse` itself; not an independent site. |
| `internal/characters/progression.go:219` (`OnStatUse` body) | `CheckStatProgression` | - | (the stat passed in) | Every `OnStatUse` call, if `UseSkillProgression` config is on | Plumbing | Same. |
| `internal/characters/progression.go:229` (`OnSkillUse` body) | `OnSkillUseScaled` | - | - | Every `OnSkillUse` call | Plumbing | `OnSkillUse` is a thin wrapper around `OnSkillUseScaled` with multiplier 1.0. Not counted toward the 10 conventions. |
| `internal/characters/progression.go:236` (`OnSkillUseScaled` body) | `TrackSkillUse` | (the skill passed in) | - | Every `OnSkillUseScaled` call, unconditionally | Plumbing | |
| `internal/characters/progression.go:249` (`OnSkillUseScaled` body) | `CheckSkillProgression` | (the skill passed in) | - | Every `OnSkillUseScaled` call, if `UseSkillProgression` config is on | Plumbing | |
| `internal/characters/progression.go:254` (`OnSkillUseScaled` body) | `OnStatUse` | - | the skill's primary stat, if one is mapped | Every `OnSkillUseScaled` call whose skill has a primary stat | Plumbing, but load-bearing | **This is why most `OnSkillUse` sites in this table show "(chained)" in the Stat column** - the primary-stat roll is automatic and invisible at the call site. Any call site that instead calls `CheckSkillProgression` directly (picklock, sneak, `combat_helpers.go:1227-1228`, `actions_progression.go`) skips this entirely and awards no stat progression at all. |
| `internal/characters/progression.go:297` (`OnCriticalSuccess` body) | `TrackSkillUse("critical_success")` | - | - | Every `OnCriticalSuccess` call | Plumbing | Tracks a synthetic "critical_success" counter, separate from the real skill's counter. |
| `internal/characters/progression.go:301` (`OnCriticalSuccess` body) | `CheckSkillProgression` with 2.0 bonus | the `context` string passed in (a skill name) | - | Every `OnCriticalSuccess` call, if progression config on | Plumbing | This is the actual crit-bonus roll for whichever skill triggered the crit. |
| `internal/characters/progression.go:313` (`OnCriticalFailure` body) | `TrackSkillUse("critical_failure")` | - | - | Every `OnCriticalFailure` call | Plumbing | |
| `internal/characters/progression.go:317` (`OnCriticalFailure` body) | `CheckSkillProgression` with 1.0 bonus | the `context` string passed in | - | Every `OnCriticalFailure` call, if progression config on | Plumbing | Fumbles get the same bonus as an ordinary success roll (1.0), not a reduced one. |
| `internal/characters/progression.go:332` (`OnFirstMobKill` body) | `CheckSkillProgression` with 2.0 bonus | "combat" | - | `OnFirstMobKill`, if progression config on | Plumbing | The roll itself; see the two real call sites below, found outside the task's grep pattern list. |
| `internal/hooks/Death_MobKillCredit.go:61` | `OnFirstMobKill` | "combat" (via progression.go:332) | - | First time this player's `KD.GetMobKills(mobId) == 1`, i.e. first-ever kill of that specific mob type, AND not in the `Training` zone | Yes | **Not caught by the task's ten grep patterns** (`OnFirstMobKill(` was never one of them) - found by reading `progression.go` end to end. A ninth distinct firing shape: first-kill-of-type-only, gated on kill-credit bookkeeping rather than a contest roll. See the count correction above. |
| `internal/hooks/Death_MobKillCredit.go:86` | `OnFirstMobKill` | "combat" (via progression.go:332) | - | Same first-kill gate, applied to party members who get kill credit but were not the direct attacker | Yes | Party-credit mirror of :61. |
| `internal/characters/progression.go:368` (`CheckRegenProgression`) | `IncreaseStat` (direct, bypasses `Track`/`OnStatUse` entirely) | - | (the stat passed in) | Called by `OnRegenTick`, once per related stat; own internal roll uses a caller-supplied `chance`, further scaled by mob/mutation multipliers | Plumbing | Not gated on any activity at all - purely a function of how depleted the pool is. Calls `IncreaseStat` directly, so it does not even go through `CheckStatProgression`'s soft-cap/anti-exploit floor logic the way every other stat-progression path in this audit does. |
| `internal/characters/progression.go:424` (`OnRegenTick`) | wrapper, calls `CheckRegenProgression` per related stat | - | (each stat in `relatedStats`) | Every regen tick (every 3 rounds, per `NewRound_AutoHeal.go`'s own docstring), for every pool below 100% | Plumbing | See Finding 5 below - this is a passive, non-use-based tenth firing shape. |
| `internal/hooks/NewRound_AutoHeal.go:272` | `OnRegenTick` | - | vitality, willpower | Every player regen tick, health pool below its (reservation-adjusted) max | Yes | |
| `internal/hooks/NewRound_AutoHeal.go:275` | `OnRegenTick` | - | strength, vitality | Every player regen tick, stamina pool below max | Yes | |
| `internal/hooks/NewRound_AutoHeal.go:278` | `OnRegenTick` | - | willpower, charisma | Every player regen tick, conviction pool below max | Yes | |
| `internal/hooks/NewRound_AutoHeal.go:368` | `OnRegenTick` | - | vitality, willpower | Every mob regen tick, health pool below max | Yes | Mob mirror of :272. |
| `internal/hooks/NewRound_AutoHeal.go:371` | `OnRegenTick` | - | strength, vitality | Every mob regen tick, stamina pool below max | Yes | Mob mirror of :275. |
| `internal/hooks/NewRound_AutoHeal.go:374` | `OnRegenTick` | - | willpower, charisma | Every mob regen tick, conviction pool below max | Yes | Mob mirror of :278. |
| `internal/actions/actor_mob.go:75` | `OnSkillUse` wrapper | - | - | Forwards to `Mob.Character.OnSkillUse` | Plumbing | `MobActor.OnSkillUse` - the dispatch every `actor.OnSkillUse(...)` call in `internal/actions/*` resolves through for a mob actor. |
| `internal/actions/actor_mob.go:79` | `OnStatUse` wrapper | - | - | Forwards to `Mob.Character.OnStatUse` | Plumbing | |
| `internal/actions/actor_mob.go:83` | `OnCriticalSuccess` wrapper | - | - | Forwards to `Mob.Character.OnCriticalSuccess` | Plumbing, and DEAD | Defined on the `Actor` interface but **no production code calls `actor.OnCriticalSuccess(...)` through the interface** - only `Character.OnCriticalSuccess` is called directly, from `combat.go` and `NewRound_DoCombat_unified.go`. New finding. |
| `internal/actions/actor_mob.go:87` | `OnCriticalFailure` wrapper | - | - | Forwards to `Mob.Character.OnCriticalFailure` | Plumbing, and DEAD | Same dead-interface-method finding as :83. |
| `internal/actions/actor_user.go:74` | `OnSkillUse` wrapper | - | - | Forwards to `User.Character.OnSkillUse` | Plumbing | Player-side mirror of `actor_mob.go:75`. |
| `internal/actions/actor_user.go:78` | `OnStatUse` wrapper | - | - | Forwards to `User.Character.OnStatUse` | Plumbing | |
| `internal/actions/actor_user.go:82` | `OnCriticalSuccess` wrapper | - | - | Forwards to `User.Character.OnCriticalSuccess` | Plumbing, and DEAD | Same dead-interface-method finding. |
| `internal/actions/actor_user.go:86` | `OnCriticalFailure` wrapper | - | - | Forwards to `User.Character.OnCriticalFailure` | Plumbing, and DEAD | Same. |

## Substantive findings

### 1. Melee has two defender progression sites, and they disagree on stat

`internal/combat/combat_helpers.go:1227-1228` (`sendDefenseMessages`, called
from `resolveOneSwing` at lines 1121 and 1161) awards skill progression on
**every individual defended swing** that wins - including within a single
round that has multiple swings. `internal/hooks/NewRound_DoCombat_helpers.go
:44-59` (`processDefenderProgression`, called once per round from
`applyCombatProgression`) awards skill progression **once per defence type
per round**, deduplicated across however many swings used that type. Both
paths run on the same round; the per-swing path fires first (during swing
resolution) and the per-round path fires again afterward (during Phase 5).

The two paths also diverge on what they award: the per-round path
(`AwardDefenceProgression`, `defence_multiplier.go:117-139`) calls both
`OnSkillUse` and `OnStatUse` for the matching stat (dexterity for dodge,
dexterity+strength for parry, strength for block). The per-swing path
(`sendDefenseMessages`) calls only `TrackSkillUse`+`CheckSkillProgression`
directly - no stat roll at all, because it bypasses `OnSkillUse` (which is
what would have auto-fired the primary stat per
`progression.go:254`).

This is a duplication defect, not a convention difference - task 9 (per the
plan) deletes the per-swing site. Note for 9's implementer: deleting it also
removes the ONLY site in the melee defence path that currently omits stat
progression, so post-deletion the surviving per-round path becomes the
single source of truth for both skill AND stat, which is a strict increase
in what a defended swing awards, not just a dedup.

### 2. Melee attacker progression fires twice per round

`internal/combat/combat.go`'s four `Attack*` functions (`AttackPlayerVsMob`,
`AttackPlayerVsPlayer`, `AttackMobVsPlayer`'s call into
`trackMobAttackProgression`, `AttackMobVsMob`) run attacker strength,
dexterity, combat-skill, and crit/fumble progression inline. These are
called from `internal/hooks/NewRound_DoCombat_unified.go:314-328`
(`rollCombatAttack`, "Phase 2" per that file's own phase comments). The same
round's `handleCombatRound` then reaches "Phase 5" at line 188
(`applyCombatProgression`), which independently re-derives and re-fires the
identical attacker strength/dexterity (`emitAttackerStatGain`,
`NewRound_DoCombat_unified.go:687`) and per-weapon-hit skill/crit/fumble
progression (`NewRound_DoCombat_unified.go:663-672`) from the same
`AttackResult`.

This was measured this session (per the task brief) at strength 2,
weapon-combat 2, dexterity 4 rolls against an intended 1/1/<=2 - consistent
with a clean double-fire on every attacking swing (dexterity doubles again
because both the attacker's dexterity call and, in the vs-mob/vs-mob
quadrants, a separate defender dexterity call live in the same file). Task 9
deletes the phase-2 copy (the calls inside `combat.go`), leaving
`applyCombatProgression` as the sole attacker-progression path.

### 3. Melee and the channel path disagree on defence timing - an open question for U10b

Melee's per-round defence award (`processDefenderProgression`) only fires
for a defence type that shows up in `result.SwingEvents` with
`DefenseUsed != DefenseNone` - i.e., only a defence that actually **won**
gets progression. `internal/combat/defence_multiplier.go:246`
(`resolveChannelDefenceWithRunner`) awards the winning-quoted defence
`AwardDefenceProgression` **whenever the contest was `Contested`, regardless
of `res.Success`** - i.e., a channel defence that rolled and LOST still gets
progression, as long as it was the best-quoted defence entering the roll.
The source comment at `defence_multiplier.go:184-187` is explicit that this
is intentional: *"The selected defence is CHARGED and PROGRESSED whether or
not it wins, matching both the melee path and the two deleted functions
(which awarded progression unconditionally)"* - but that claim of matching
"the melee path" is about charging cost, not about progression timing; the
melee *per-round* progression path plainly requires `used[d]==true`, which
only happens when the defence type shows up with `DefenseUsed` set, which
only happens on a win (see Finding 1's source, `combat_helpers.go:1191`,
`result.DefenseUsed = DefenseType(best.defenseType)`, set inside the
defended-win branches only).

So both conventions are live, both are deliberate in their own file, and
they contradict each other: melee defence progression requires a win;
channel defence progression does not. This is exactly the kind of question
the task brief says is **U10b's call, not this audit's** - flagging it here
without resolving it.

### 4. Category C - crafting, salvage, forage, search (non-contest / static-difficulty sites)

| Site | Reaches progression? | What it awards |
|---|---|---|
| `usercommands/craft.go:142` | Yes, on success (`ImmediateComplete`) | Recipe skill, difficulty-scaled (`OnSkillUseScaled`) |
| `mobcommands/craft.go:54` | Yes, on success (`ImmediateComplete`) | Recipe skill, difficulty-scaled |
| `NewRound_UserRoundTick.go:591` | Yes, on multi-round success | Recipe skill, difficulty-scaled |
| `NewRound_MobRoundTick.go:496` | Yes, on multi-round success | Recipe skill, difficulty-scaled |
| `mobs/crafter.go:505`, `:546` | Yes, on success | Recipe skill, **unscaled** (breaks the cluster's own convention - new finding, not previously known) |
| `actions/salvage.go:166`, `:252` | Yes, always once committed (item/corpse consumed regardless of yield) | Salvage skill, unscaled, no difficulty multiplier despite salvage having a config-driven skill-scaled yield curve elsewhere in the system |
| `actions/forage.go:142` | Yes, gated on a successful find | Search skill, unscaled |
| `actions/search.go:243` | Yes, gated on "a roll happened" (not on finding anything) | Search skill, unscaled |
| `actions/track.go:128` | Yes, gated on "a roll happened" | Search skill, unscaled |
| `usercommands/go.go:388` | Yes, gated on a separate config-probability roll (`movementTrainsSearch`), independent of any skill contest | Search skill, unscaled |

U10b routes these. The crafting cluster is the most internally consistent
group in the whole audit (difficulty-scaled, success-only) except for the
two `mobs/crafter.go` sites, which quietly downgrade to the unscaled call.
Salvage, forage, search, and track are all unscaled and each gates on a
different condition (commit-always / found / rolled / rolled-via-a-second-
system).

### 5. Passive regen-tick progression is a wholly separate mechanism from everything else in this audit

`OnRegenTick` (`progression.go:424`, called six times from
`internal/hooks/NewRound_AutoHeal.go`) fires **every regen tick (every 3
rounds) for every character, whether or not they did anything at all.**
Every other row in this table is triggered by some player or mob action
(a swing, a cast, a craft, a search, a purchase). This one fires on the
passage of time alone, scaled by how depleted a resource pool currently is
(`chance = RegenProgressionBase * (1 - ratio)^RegenProgressionCurve`).

This matters for U10b's "one event per success" framing because there is no
"success" here to count events against - a resting character with a
partially-drained stamina bar is rolling stat progression every three
rounds purely for being under 100% HP/SP/CP, independent of the nine other
conventions cataloged above, all of which require the character to have
done something. It also bypasses
`CheckStatProgression`'s normal path entirely: `CheckRegenProgression`
calls `IncreaseStat` directly (see the table row for
`progression.go:368`), so none of `CheckStatProgression`'s soft-cap or
anti-exploit floor logic (`progression.go:149-195`, referenced elsewhere in
this codebase as "the anti-exploit floor in `CheckStatProgression`")
applies to a regen-tick stat gain. Whether that is intentional (regen
progression is meant to be a slow trickle regardless of soft-cap
proximity) or an oversight is outside this audit's scope to judge, but
U10b should know the two paths to stat progression (use-triggered via
`CheckStatProgression`, and regen-triggered via bare `IncreaseStat`) do not
share the same guardrails today.

## Recommended convention (for U10b to decide, not this audit)

The project owner's stated shape is: **one progression event per success,
with crit and critical-failure as a separate bonus layered on top of that
one event** - not a separate, independently-gated event of its own. Under
that shape:

- Every "always fires regardless of outcome" site (convention 2 in the list
  above: defuse, plant, steal firing before the roll; taunt on fumble/miss;
  surprise attack on an all-miss; shadow on begin; venom-coat; consider;
  look; assess) would need to either move its call to after the outcome is
  known and gate on success, or be classified as a genuinely non-contested
  action (venom-coat, consider, look, assess have no roll at all, so "on
  success" is vacuous for them and firing on invocation is already correct
  under the target rule).
- The two defence-progression paths (per-swing win-only vs. per-round
  win-only vs. channel win-or-lose) would need to converge on ONE of those
  three shapes. Melee's per-round win-only reading is the closest existing
  match to "one event per success."
- The crit/fumble bonus machinery already exists in roughly the right shape
  (`OnCriticalSuccess`/`OnCriticalFailure` as a second, explicitly-bonused
  call layered after the base `OnSkillUse`) for melee and taunt, but is
  applied inconsistently elsewhere (`OnCritReceived` is defender-side and
  fires from a different trigger - taking a crit, not landing one - so it
  is not really the same "bonus on top" shape and should be looked at
  separately by U10b rather than assumed to already fit the pattern).

This is a recommendation for U10b's design, not a decision made by this
audit.
