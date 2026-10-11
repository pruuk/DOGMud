# Messaging M6 Slice 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A condition remembers who put it there, so a damage-over-time kill credits its caster (#240), a spell that lands a condition prints one line per audience from the condition's own data (#370 ledger row 1), and shield and heal spells each land their own condition in a family where the newest replaces the older (#338, #370 ledger row 2).

**Architecture:** The caster rides on the existing `events.Condition` event (new `Caster`, `CasterCrit`) through one new door per holder kind (`UserRecord.QueueCondition`, `Mob.QueueCondition`), and on the synchronous character door as `AddConditionMagnitudeBy`; `ApplyConditions` stamps it on the record (`Conditions.Stamp`). The round ticks pass `condition.Caster` to the existing `ApplyHarm` and credit a mob's damage map through `creditMobHarm`, which is `creditSpellDamage`'s rule by ref. Start lines go through the existing `Narrate` door with a caster (`NarrateCast`, new `start_actor`) and the existing sight senders (`HideNames`, `SendTextVisualHidingNames`, the darkness snapshot). Families are a closed `family` field the add primitives enforce with the existing `Discard`. Wards and heals are ordinary condition YAML named by each spell's `condition_ids`.

**Tech Stack:** Go 1.25, testify, GoMud engine packages under `internal/`, YAML content and help templates under `_datafiles/world/dogmud/`.

**Spec:** `docs/superpowers/specs/2026-10-07-messaging-m6-slice1-casters-shields-heals-design.md` (APPROVED by the owner 2026-10-07; rulings R1 to R11 are not relitigated here). Related: `docs/superpowers/specs/2026-08-31-messaging-unification-design.md`, `docs/superpowers/audits/messaging-m6-content-ledger.md`.

**Branch:** implement on `feat/messaging-m6-slice1`, cut from master `64105e41b`. The PR body says "Refs #370, Refs #338, Refs #240" and names what it resolves in words, never with a closing keyword.

---

## How this plan was verified

The plan was first written against master `0d0732339`. While it was being dry-run, #462 (for #458) landed and reshaped the condition start block this slice rewrites (`startUnseenBy`, `conditionMobNames`, `mobSeenName`; `mobPlainName` deleted), so every task was rebased onto `64105e41b` and the whole dry run repeated there; everything below is `64105e41b`'s. Every task was first built test-first in a throwaway worktree (under the session scratchpad): each new test was run against the old code and failed for the reason its step gives, then passed after its implementation step. Then the plan itself was dry-run: a script read THIS DOCUMENT, applied every task's blocks in order to a second fresh `64105e41b` worktree (each "replace this exact text" block must match exactly once at its turn, as the Edit tool requires), ran each task's "expect FAIL" command before its implementation step and its "expect PASS" commands after, and ran `go build ./...`, `go vet ./...` and `go test . -count=1` at the end of every task. Then the whole tree was run (see "Integrated dry run"). Both worktrees were removed afterwards. Line numbers below are master `64105e41b`'s; every edit is anchored by quoted text, never by a number alone.

## Facts verified against source (master `64105e41b`)

Built by grepping and reading for this plan, not copied from the spec.

### Records, doors and the event

| Fact | Source |
|---|---|
| `conditions.Condition` has `ConditionId`, `Source`, `OnStartWaiting`, `Permanent`, `RoundCounter`, `TriggersLeft`, `TickAmount`, `Magnitude`, `Stacks`, `LightTrim`, `LightOutput`, `Hooded`; no caster | `internal/conditions/conditions.go:14-44` |
| Primitive doors: `AddConditionScaled` `:292` (wraps `addConditionScaled` `:304`), `AddConditionMagnitude` `:363`, `RefreshCondition` `:407`, `AddCondition` `:435`; `Discard` (deletes with no end line) `:145` | `internal/conditions/conditions.go` |
| A re-add of a held id overwrites `TriggersLeft`, `RoundCounter`, `Permanent` in place | `conditions.go:326-330` (scaled), `:462-466` (plain); magnitude door `:374-388` |
| Call sites: `.AddCondition(` 51 non-test / 327 test, `.AddConditionMagnitude(` 33 / 134, `.AddConditionScaled(` 13 / 9 | `grep -rn` over `internal`, `modules` |
| `events.Condition` has `UserId`, `MobInstanceId`, `ConditionId`, `Source`, `DurationMult`, `Magnitude`, `Triggers`, `TickScale`, `LifeEpoch`; no caster. `events` imports nothing that imports it back from `state` (`state` imports no GoMud package) | `internal/events/eventtypes.go:20-44`; `go list -f '{{.Imports}}' ./internal/state` |
| Queue doors: `UserRecord.AddCondition` `:422`, `AddConditionScaled` `:437`, `AddConditionMagnitude` `:462`, `AddConditionTickScaled` `:477`; `Mob` twins `:868`, `:883`, `:899`, `:912` | `internal/users/userrecord.go`, `internal/mobs/mobs.go` |
| `Character.AddConditionMagnitude` stamps `Source` by looping `GetConditions` | `internal/characters/conditions.go:259-273` (loop `:264-266`) |
| `applySpellCondition` reaches the target through `spellConditionTarget{AddCondition, AddConditionMagnitude, AddConditionTickScaled}` | `internal/hooks/light_spell.go:57-83` |
| User saves persist conditions (`yaml:"conditions,omitempty"`), and so does a copyover file; both are yaml.v2. A mob's `InstanceId` is runtime only | `internal/characters/character.go:147`; `internal/users/users.go:24`, `copyover.go:16`; `internal/mobs/mobs.go:88` |
| `state.ActorRef{UserId, MobInstanceId}` has `IsZero` (yaml.v2 and v3 honour it for `omitempty`) and no yaml tags; nothing persists one today | `internal/state/transition.go:7-25`; dry-run test `TestCaster_PlayerSurvivesASaveAndAMobDoesNot` |

### The apply hook and narration

| Fact | Source |
|---|---|
| `wasAlreadyActive` is read before the add; a refresh tells no start | `internal/hooks/Condition_ApplyConditions.go:88`, gate `:141` |
| `hideOnHidden` suppresses a hide's start room line on an already hidden holder | `Condition_ApplyConditions.go:93`, used `:172` |
| Every condition room line skips the readers who do not perceive a hidden holder: `startUnseenBy` is taken before the add; `conditionLineUnseenBy` returns `playersNotPerceiving` for a hidden holder; `conditionMobNames` names a mob holder through `mobSeenName` (#458, landed in #462) | `Condition_ApplyConditions.go:97`, `:269-275`, `:296-302` |
| The darkness snapshot is taken before the add (`darknessStartSnapshot`) | `Condition_ApplyConditions.go:115`, `:231-240` |
| The three adds; only the magnitude door passes `evt.Source` | `Condition_ApplyConditions.go:118`, `:120`, `:122` |
| Holder line `holder.SendText`; room line `sendConditionStartRoomText` (snapshot, else `SendTextVisualHidingNames`) excluding the holder and `startUnseenBy` | `Condition_ApplyConditions.go:166`, `:188-189`, `:253-259` |
| `Narration(p)` fills `Actee` and `Observer` only; `Narrate(p, holder, holderPlain)` fills `{actee}`; `validateNarration` checks start, trigger, end | `internal/conditions/narration.go:28-39`, `:45-50`, `:78-97` |
| `TokenContext` already carries `ActorName` / `ActorPlainName` (`{actor}`, `{actor_plain}`) | `internal/textutil/tokens.go:14-19` |
| `StartUserNotice` falls back to "`<Name>` takes effect." for every non-secret, non-quiet, non-silent-start spec | `internal/conditions/notice.go:31-49` |
| `messaging.HideNames(text, names, decision)` hides each name at shapes or none; `Room.ParticipantSight(userId)` judges a participant | `internal/messaging/hidenames.go:55`; `internal/rooms/rooms.go:356` |
| `mobDisplayName(m, room, viewer)` reads "something" for a hidden mob the viewer does not perceive; `mobSeenName` is it without the hidden check; `mobHiddenFrom` is the test | `internal/hooks/NewRound_DoCombat_helpers.go:405`, `:415`, `:427` |
| The spell trio is sent after the conditions queue, under "KNOWN AND DEFERRED ... M6 merges them" | `internal/hooks/spell_help_effects.go:83-113` (comment `:91`) |
| `spell.ConditionIds` is read by `applySpellConditionEffect` only | `spell_help_effects.go:84`; `grep -rn '\.ConditionIds' internal` |
| Help spells never crit (`uncontestedSpellResult`); only the 6 harmful condition spells can | `internal/hooks/spell_effects.go:449-451` |
| `critTag()` is the one crit marker | `spell_effects.go:145-150` |

### Ticks, harm and credit (#240)

| Fact | Source |
|---|---|
| Ticks harm anonymously: mob `ApplyHarm(..., state.ActorRef{})` for health, stamina, conviction | `internal/hooks/NewRound_MobRoundTick.go:250`, `:269`, `:275` |
| Same for players | `internal/hooks/NewRound_UserRoundTick.go:334`, `:355`, `:361` |
| `ApplyHarm` puts `source` in `CharacterDied.KillerUserId/KillerMobInstanceId` and sets `DeathQueued` | `internal/characters/pools.go:644-691` (`:672-687`) |
| The backstops kill only a character at zero NOT `DeathQueued`; a tick goes through `ApplyHarm`, so it never reaches them | `NewRound_MobRoundTick.go:149-155`; `Condition_ApplyConditions.go:207-218` (no tick runs there); `CharacterDied_RouteDeath.go` `shouldSweepReap` |
| `Die` snapshots `PlayerDamage` into `DeadData.DamageMap` | `internal/characters/die.go:39`, `:54` |
| Every mob-death consumer reads the damage map, not the killer ref | `Death_MobKillCredit.go`, `MobDeath_FactionRep.go:37`, `MobDeath_QuestNotify.go:19`, `MobDeath_BountyClaim.go`, `MobDeath_ItemProcs.go`, `Death_MobLoot.go` |
| `TrackPlayerDamage` producers: melee, charmed melee, PvP melee, shoot, `creditSpellDamage` (mob targets only); none for a tick | `internal/characters/engagement_storage.go:79`; `internal/hooks/spell_effects.go:233-248` |
| Murder upgrade: `FindRecentAssault` / `FindRecentUnknownAssault` from `MobDeathFactionRep`, keyed by user id and victim, no live user needed | `MobDeath_FactionRep.go:81`, `:137`; `internal/crimes/crimes.go:291`, `:341` |
| A player victim's bounty claim reads the killer ref first; an offline killer gets `MarkExpired` (as today with a zero ref and an empty map) | `PlayerDeath_BountyResolve.go:132`, `:155-158` |
| Bleeds are added through the character door by drain `:147`, `:314`, hamstring `:138`, maul `:132`, rake `:132`, throttle `:146`, the item proc `:359`; the spell dot `spell_effects.go:348` | `internal/actions/combat_*.go`, `internal/behaviortree/actions_item_proc.go`, `internal/hooks/spell_effects.go` |

### Shields, heals and the AI

| Fact | Source |
|---|---|
| `applySpellShield` lands hard-coded 119 via the character door: bonus `(stat + round(skill x SkillWeight)) / 3 x magnitude/100`, min 1, duration `calcSpellDuration` | `internal/hooks/spell_help_effects.go:175-215` (`:195`) |
| `applySpellHeal` lands hard-coded 120, `regenMult = max(magnitude, 1)` for `calcSpellDuration/2` (min 6) | `spell_help_effects.go:126-166` (`:144`) |
| `calcSpellDuration = baseFolds x (10 + willpower/20 + skill/2)`, min 10 | `internal/hooks/spell_resolution.go:30-39` |
| 119 "Minor Shield": `mitigation_flat: magnitude`, silent start, end "Your Minor Shield dissipates."; 120 "Regenerating": `regen_mult: magnitude`, silent start | `_datafiles/world/dogmud/conditions/119-minor_shield.yaml`, `120-regenerating.yaml` |
| `mitigation_flat` feeds physical mitigation only; magical and conviction read gear, mutations and the statmods | `internal/characters/combat.go:187`, `:231`, `:267` |
| Multipliers multiply across records in `Effect`, flats sum | `internal/conditions/effects.go:219-271` |
| `regen_mult` is read every third round (`RoundNumber%3`), on `HealthPerRound` (`PlayerHealthRegenPct` ships 0.02) | `internal/hooks/NewRound_AutoHeal.go:35`, `:203-232`; `config.yaml:1260` |
| Mob AI "already shielded" = `HasEffect(mitigation_flat)`; no heal branch | `internal/combat/ai.go:742`; `internal/behaviortree/action_cast_best_in_category.go:221-230` |
| 120 is used by the mob consume path | `internal/mobcommands/consume.go:46`, `:58` |
| Shield spells: conviction-ward mag 75, folds 4, cost 30, no `condition_ids`; chrysalis-cocoon mag 125, folds 8, cost 60, dead `[52]` | `_datafiles/world/dogmud/spells/` |
| Heal spells: heal (Mend Flesh) 3/4/25, mend-wounds 5/4/40, mend-all 3/10/50 area, communion-of-flesh 4/12/100 area, mass-mend 5/16/120 area dead `[33]`, repair-pulse 8/2/15 `mob_only` (magnitude/folds/cost) | `_datafiles/world/dogmud/spells/` |
| 20 spells use `effect_type: condition` (14 help, 6 harmful); their conditions all author `start_actee`, 8 author `start_observer`; 3 start lines hold an escaped em dash (28, 34, 37) | `_datafiles/world/dogmud/spells/*.yaml`, `conditions/` |
| Highest condition id 134; `tools/id_inventory.py --alloc conditions` hands out 135 and up | `_datafiles/world/dogmud/conditions/`; dry-run run of the tool |
| Shipped balance: `SkillWeight 5.0`, mitigation caps 0.75 (Go defaults 2.0 and 0.75) | `git show HEAD:_datafiles/config.yaml` lines 1155, 1199-1201 |
| Local saves `users/10.yaml`, `11.yaml`, `12.yaml` (untracked, main checkout) hold the trigger text "Minor Shield" | `grep -l` in the main checkout |

## Spec facts that were wrong or imprecise

- **Line numbers.** Two PRs since the spec moved most of them: adders are `conditions.go:435, 292, 363, 407` (spec `419, 276, 347, 391`); re-add overwrites `:326-330, :367-388, :462-466`; `wasAlreadyActive` `Condition_ApplyConditions.go:88` and `:141`; the `Source` door `:117-123`; the ticks `NewRound_MobRoundTick.go:250, 269, 275` and `NewRound_UserRoundTick.go:334, 355, 361`; the TriggerNow backstop `:216`; the shield `spell_help_effects.go:175-215`, the heal `:126-166`, the trio `:83-113`; the ward id `ids.go:16` (spec `:13`); the AI test `action_cast_best_in_category.go:221-230`; `ApplyHarm`'s event `pools.go:680-687`. None changes the design.
- **"The two zero-health backstops pass the caster of the condition whose tick brought health to zero."** No tick can reach either. Every tick goes through `ApplyHarm`, which queues the attributed death and sets `DeathQueued`, and both backstops reap only a character that is NOT `DeathQueued` (`shouldSweepReap`). The TriggerNow backstop runs no tick at all. They stay anonymous (D3).
- **"Kill credit then flows through the existing paths (DamageMap, ...) with no new credit rule."** Only half true. The killer ref reaches `DeadData.Killer`, which no mob-death consumer reads; every one reads the damage map, and no tick writes it. A tick on a mob must also call `TrackPlayerDamage`, which `creditSpellDamage` already does for direct spells; the plan reuses that rule by ref (`creditMobHarm`), so it is not a new credit rule (D4).
- **R10 "Credit is recorded for a player caster who has ... logged out: kill credit, quest credit and the crime record."** The crime, faction rep and auto-bounty are recorded for an offline caster (`MobDeath_FactionRep` needs no live user). The kill count (`Death_MobKillCredit`), quest credit (`MobDeath_QuestNotify.go:19`), the mob bounty claim and auto-loot gold need a loaded user record and skip an offline caster silently today; the plan does not change them. Called out for the owner (D5), as the spec asks.
- **"Every door that lands a condition takes it" (the caster).** The ~500 add call sites stay as they are; the caster rides the event and two caster doors (D1).
- **The ledger's "17" condition spells** is 20; the spec already corrected this. Confirmed: 20.
- **`internal/conditions/context.md` claims Chrysalis Cocoon grants condition 52.** It never did: `applySpellShield` does not read `condition_ids`. Task 12 rewrites that section.
- **The help pages for Conviction Ward** say Base Folds 3 and Conv. Cost 3 (YAML 4 and 30); **Chrysalis Cocoon's** says cost 8 (YAML 60). Tasks 9 and 10 make every touched help page match its YAML.

## Design points where the code forced a different choice than the spec (for the owner)

- **D1. The caster's doors.** Rather than add a caster parameter to every add door (51 + 327 `AddCondition` sites, 33 + 134 `AddConditionMagnitude`), the caster rides on `events.Condition` (`Caster`, `CasterCrit`) through one new door per holder (`UserRecord.QueueCondition`, `Mob.QueueCondition`, which the old four doors now call), and on the synchronous character door as `AddConditionMagnitudeBy` (the old door is that door with no caster). `ApplyConditions` stamps `Source` and `Caster` on every landing (`Conditions.Stamp`), so `Source` reaches the record on every door, as the spec asks. A producer that passes no caster leaves a zero caster, which is the spec's behaviour.
- **D2. Combat bleeds name their attacker.** The spec lists potions, hazards, mutations and mob consume as casterless. Bleeds from drain, hamstring, maul, rake, throttle and the item proc have an attacker, so they carry it (`actions.ActorRefOf`, `procOwnerRef`): a bleed that kills credits the attacker, as #240 asks ("a poison or bleed kill"). A stacking bleed is one record, so it names its newest applier.
- **D3. The backstops stay anonymous.** See "Spec facts that were wrong". No code change.
- **D4. Tick credit on a mob.** A mob's health tick calls `creditMobHarm(mob, caster, amount)` before `ApplyHarm`: a player caster is credited, a mob charmed by a player credits that player, anything else nobody, exactly `creditSpellDamage`'s rule, which now delegates to it. A player victim's damage map is left alone, as a direct spell leaves it; the killer ref still reaches `PlayerDeath_BountyResolve`.
- **D5. An offline caster's kill (R10).** Recorded: faction rep, the murder upgrade (when a witness identified the assault), the auto-bounty, the killer ref. Not recorded, because the consumer needs a live user record and skips silently: the kill count, quest credit, the mob bounty claim (stays open), auto-loot gold, item procs (correct: a proc needs a live body). Loading an offline save to credit a quest is out of this slice; the owner may want an issue for it.
- **D6. One line per audience, and when the spell keeps its trio.** The spell drops its generic lines only when every condition it lands will tell all three audiences with authored text (`ConditionSpec.NarratesCastStart`): authored `start_actee` and `start_observer`, and `start_actor` unless self-cast; the generic "takes effect" fallback does not count. The judgement is made at cast time, before the event lands. A landing the event later refuses (poison immunity, a holder who died meanwhile, a stealth hide another hide outranks) prints nothing at all, where it used to print the trio over nothing. A root guard (Task 11) holds every shipped spell-landed condition to the rule, so in shipped play every fresh cast tells each audience once.
- **D7. Names per reader.** `{actor}` and `{actee}` in a private line are rendered for that reader (a see-hidden caster reads a hidden mob's name, as in a spell's own lines) and hidden by that reader's sight; the room line excludes both caster and holder and keeps #458's rule: it skips every reader who did not perceive a hidden holder before the landing (`startUnseenBy`), so it can name the holder (`conditionMobNames`). A hidden mob caster reads "something" in the room line. A cast with no caster fills `{actor}` with "something". No shipped start line uses `{actor}` (the 20 conditions' holder and room lines read right whether self-cast or not without it); `{actor}` in a trigger or end line fails the load, because no caster is known there.
- **D8. The replacement line.** "Your Conviction Ward fades as Conviction Bulwark takes hold." to the holder and "Bobrick's Conviction Ward fades as Conviction Bulwark takes hold." to the room (sight-gated, names hidden at shapes, skipping `startUnseenBy`), sent just before the new ward's start lines. The old record is discarded with `Discard`, so its end line never fires. The primitives enforce the family on every door, so even a synchronous add replaces.
- **D9. Wards and heals land through the event.** Both used the synchronous character door; they now queue `events.Condition` like every condition spell, so the ward or heal is held once the event flushes (the same turn), not inside the resolution pass. Tests land the queue explicitly (`landQueuedConditions`).
- **D10. Ward numbers (the dry run).** The shield formula stays. Strength rises with the spell's magnitude 75 / 100 / 125 and the kinds blocked; duration with base folds 4 / 6 / 8; cost 30 / 45 / 60. At willpower 100 (shipped `SkillWeight` 5.0), mitigation points per kind (each point is 1% before the 0.75 channel cap):

  | Spellcasting | Ward (physical) | Bulwark (physical, spell) | Cocoon (all three) | Durations in rounds |
  |---|---|---|---|---|
  | 1 | 26 | 35 | 44 | 62 / 93 / 124 |
  | 10 | 38 | 50 | 63 | 80 / 120 / 160 |
  | 25 | 56 | 75 | 94 (capped at 75) | 110 / 165 / 220 |
  | 50 | 87 (capped) | 116 (capped) | 145 (capped) | 160 / 240 / 320 |

  Derived as `round(floor((100 + round(5 x skill)) / 3) x magnitude / 100)` and `round(folds x (15 + skill / 2))`. Conviction Ward's and Chrysalis Cocoon's own numbers are unchanged from today; the Bulwark sits between them. **For the owner:** the existing formula reaches the 0.75 cap early (Cocoon from Spellcasting 16, Bulwark from 25), so past those ranks the ordering shows only in kinds and duration. Retuning the formula is outside this slice.
- **D11. Heal numbers (the dry run).** Each heal's multiplier is the spell's `effect_magnitude` (as today) and its duration is its condition's `triggercount`, written for an untrained caster of willpower 100 and scaled by the caster's `calcSpellDuration` term, `(10 + willpower/20 + skill/2) / 15` (`healTriggers`). Today a heal lasted half its spell's fold-scaled duration, so duration could not differ from casting time; the condition carries it now. Healing per cast at willpower 100, as a share of maximum health in a fight (multiplier x 2% x one regen tick per three rounds; at rest the first 1x is natural regen):

  | Heal | x | Base rounds | Spellcasting 10: rounds, healing (today) | Spellcasting 50: rounds, healing (today) |
  |---|---|---|---|---|
  | Mend Flesh (single, long and gentle) | 2 | 45 | 60, 80% (40, 78% at x3) | 120, 160% (80, 156%) |
  | Mend Wounds (single, short and strong) | 6 | 25 | 33, 132% (40, 130% at x5) | 67, 264% (80, 260%) |
  | Mend All (area, short and light) | 3 | 30 | 40, 78% (100, 198%) | 80, 156% (200, 396%) |
  | Communion of Flesh (area, long and steady) | 4 | 90 | 120, 320% (same) | 240, 640% (same) |
  | Mass Mend (area, short and strong) | 8 | 55 | 73, 384% (160, 530% at x5) | 147, 784% (320, 1060%) |
  | Arc-Weld Repair (`repair-pulse`, mob only) | 8 | 15 | 20, 96% (same) | 40, 208% (same) |

  Mend Flesh, Mend Wounds and Communion keep today's total; Mend Wounds and Mass Mend now heal faster per round than anything else. **For the owner:** Mend All drops from about 2x to about 0.8x a full bar per cast, and Mass Mend from about 5x to about 4x, because their old durations came from long fold counts (10 and 16). The identities (short and light, top area) needed that; if the totals matter more, raise their `triggercount` in the condition YAML.
- **D12. Condition 119 is renamed in place** (file `119-conviction_ward.yaml`, constant `ConditionIdConvictionWard`), so a save holding 119 loads as a Conviction Ward. 52 Chrysalis Shell and 120 Regenerating are unchanged and out of both families. Vital Surge (32) and Chrysalis Regeneration (33) join the heal family and gain authored start lines.
- **D13. Help and aliases.** New `help wards` and `help healing-spells` pages (with `keywords.yaml` aliases) state the one-at-a-time rule; every touched spell help page now matches its YAML and carries no dash. `repair-pulse.template` still calls the spell "Repair Pulse" (YAML "Arc-Weld Repair"): pre-existing, not touched.

## Guards and package tests the dry run tripped, and how each task handles them

- **`condition_apply_path_guard_test.go` (root), line-keyed.** Task 6 re-keys `Condition_ApplyConditions.go|118,120,122` to `|122,124,126`. Task 7 widens its pattern to `AddConditionMagnitudeBy` (the bleeds and the dot move to it, and the guard would otherwise stop seeing them) and re-keys `spell_effects.go|348` to `|355`. Task 8 deletes the two `light_spell.go` rows (that file no longer calls a magnitude door) and re-keys `spell_help_effects.go|195,144` to `|220,169`. Task 9 deletes the shield row and re-keys the heal row to `|176`; Task 10 deletes the heal row.
- **`internal/actions/command_readiness_drift_test.go` `TestSpecialMoveAdmissionOrdering`** pins the exact call identity `target.Char.AddConditionMagnitude`; Task 7 updates the five rows to `AddConditionMagnitudeBy`.
- **`shipped_narration_data_guard_test.go` `TestObserverIdentityTagsAreAnonymizable`** needs every condition file whose observer lines carry `{actee}` in its map: Task 9 re-keys 119 and adds 135, 136; Task 10 adds 137 to 142; Task 11 adds the 12 conditions that gained a `start_observer`.
- **`internal/narration` `TestSnapshotStores`** golden: Task 9 adds `start_actor` rows to the builder; Tasks 9, 10 and 11 regenerate `conditions.golden` (and Task 9 `spells.golden`, for the new spell) with `-update` after checking the diff holds only their own conditions.
- **`internal/devtools` `TestHelpFileCompleteness_Spells`** fails for a spell with no help page; Task 9 adds `conviction-bulwark.template`.
- **Hooks tests that read a shield or heal record synchronously** (`spell_shield_test.go`, `spell_heal_test.go`, `spell_help_parity_test.go`, `spell_selfcast_test.go`, `hidden_mob_spell_room_lines_test.go`, `spell_help_area_test.go`, `nonharm_mob_shortcut_test.go`, `spell_channel_sight_test.go`) now land the queue first (D9), and the shield's room line is the ward's own. `spell_effect_fixture_test.go` drains leftover condition events, or a previous test's queued ward lands in the next.
- **Tests that name "Minor Shield"** (`hooks_test.go`, `death_strip_end_lines_test.go`, comments in five more) follow the rename (Task 9).
- **`TestDefensiveCaster_CocoonActive_SingleEnemy_CastsHarmSingle`** seeded the dead condition 52; Task 9 rewrites it to `..._AnyWardActive_...`, seeding a different ward, which only the family test sees.
- **`internal/hooks/shroud_hide_test.go`, `spell_tick_scale_test.go`** call `applySpellCondition`; Task 8 adds the two new arguments.
- **Checked and untouched:** `messaging_surface_guard_test.go` and `sight_penalty_guard_test.go` key `spell_resolution.go` lines; Task 10's one edit there keeps the line count. `condition_notice_guard_test.go` (every condition authors start and end) passes for every new condition. `golangci-lint run --new-from-rev=master` reports 0 issues on the whole branch.

## File map, order and parallel lanes

| Task | What | Files |
|---|---|---|
| 1 | The caster on a condition record (#240) | `internal/conditions/caster_test.go`, `internal/conditions/conditions.go`, `internal/conditions/caster.go` |
| 2 | Condition families (#338, R4, R6) | `internal/conditions/family_test.go`, `internal/conditions/conditionspec.go`, `internal/conditions/family.go`, `internal/conditions/conditions.go` |
| 3 | The caster's start line (`start_actor`, `{actor}`) | `internal/conditions/narration_caster_test.go`, `internal/conditions/conditionspec.go`, `internal/conditions/notice.go`, `internal/conditions/narration.go` |
| 4 | Ward effects for spells and words | `internal/characters/ward_mitigation_test.go`, `internal/conditions/effects.go`, `internal/characters/combat.go` |
| 5 | The caster's doors | `internal/users/queue_condition_test.go`, `internal/mobs/mob_queue_condition_test.go`, `internal/characters/condition_caster_test.go`, `internal/events/eventtypes.go`, `internal/users/userrecord.go`, `internal/mobs/mobs.go`, `internal/characters/conditions.go` |
| 6 | One start line per audience, and the replacement line (#370 row 1, R6, R11) | `internal/hooks/condition_cast_lines_test.go`, `internal/hooks/condition_cast_lines.go`, `internal/hooks/Condition_ApplyConditions.go`, `internal/hooks/spell_effects.go`, `condition_apply_path_guard_test.go` |
| 7 | A damage-over-time kill credits its caster (#240, R5, R10) | `internal/hooks/condition_tick_credit_test.go`, `internal/actions/actor_ref_test.go`, `internal/actions/combat_drain_test.go`, `internal/actions/combat_throttle_test.go`, `internal/behaviortree/item_proc_effects_test.go`, `internal/actions/actor_ref.go`, `internal/actions/combat_drain.go`, `internal/actions/combat_hamstring.go`, `internal/actions/combat_maul.go`, `internal/actions/combat_rake.go`, `internal/actions/combat_throttle.go`, `internal/behaviortree/actions_item_proc.go`, `internal/hooks/NewRound_MobRoundTick.go`, `internal/hooks/NewRound_UserRoundTick.go`, `internal/hooks/spell_effects.go`, `internal/hooks/spell_damage_credit_test.go`, `internal/actions/command_readiness_drift_test.go`, `condition_apply_path_guard_test.go` |
| 8 | A condition spell drops its generic trio (R11) | `internal/hooks/spell_condition_lines_test.go`, `internal/hooks/light_spell.go`, `internal/hooks/spell_help_effects.go`, `internal/hooks/shroud_hide_test.go`, `internal/hooks/spell_tick_scale_test.go`, `condition_apply_path_guard_test.go` |
| 9 | Three wards (#338, R3, R7) | `spell_condition_data_guard_test.go`, `internal/behaviortree/defensive_caster_archetype_integration_test.go`, `internal/hooks/spell_shield_test.go`, `_datafiles/world/dogmud/conditions/119-conviction_ward.yaml`, `_datafiles/world/dogmud/conditions/135-conviction_bulwark.yaml`, `_datafiles/world/dogmud/conditions/136-chrysalis_cocoon.yaml`, `_datafiles/world/dogmud/spells/conviction-bulwark.yaml`, `_datafiles/world/dogmud/spells/conviction-ward.yaml`, `_datafiles/world/dogmud/spells/chrysalis-cocoon.yaml`, `_datafiles/world/dogmud/templates/help/conviction-ward.template`, `_datafiles/world/dogmud/templates/help/conviction-bulwark.template`, `_datafiles/world/dogmud/templates/help/chrysalis-cocoon.template`, `_datafiles/world/dogmud/templates/help/wards.template`, `_datafiles/world/dogmud/keywords.yaml`, `internal/conditions/ids.go`, `internal/conditions/test_helpers.go`, `internal/hooks/spell_help_effects.go`, `internal/behaviortree/action_cast_best_in_category.go`, `internal/combat/ai.go`, `internal/conditions/effects_test.go`, `internal/conditions/records_test.go`, `internal/characters/conditions_pin_test.go`, `internal/combat/combat_helpers_test.go`, `internal/behaviortree/pure_caster_archetype_integration_test.go`, `internal/hooks/condition_notice_test.go`, `internal/hooks/death_strip_end_lines_test.go`, `internal/hooks/hooks_test.go`, `internal/hooks/NewRound_DoCombat_parity_test.go`, `internal/hooks/hidden_mob_spell_room_lines_test.go`, `internal/hooks/spell_effect_fixture_test.go`, `internal/hooks/spell_help_parity_test.go`, `internal/hooks/spell_selfcast_test.go`, `condition_apply_path_guard_test.go`, `shipped_narration_data_guard_test.go`, `internal/narration/snapshot_test.go`, `internal/narration/testdata/stores/conditions.golden`, `internal/narration/testdata/stores/spells.golden`, `_datafiles/world/dogmud/conditions/119-minor_shield.yaml` |
| 10 | Six heals (#338, R8, R9) | `spell_condition_data_guard_test.go`, `internal/hooks/spell_heal_test.go`, `_datafiles/world/dogmud/conditions/137-mend_flesh.yaml`, `_datafiles/world/dogmud/conditions/138-mend_wounds.yaml`, `_datafiles/world/dogmud/conditions/139-mend_all.yaml`, `_datafiles/world/dogmud/conditions/140-communion_of_flesh.yaml`, `_datafiles/world/dogmud/conditions/141-mass_mend.yaml`, `_datafiles/world/dogmud/conditions/142-arc_weld_repair.yaml`, `_datafiles/world/dogmud/conditions/32-vital_surge.yaml`, `_datafiles/world/dogmud/conditions/33-chrysalis_regeneration.yaml`, `_datafiles/world/dogmud/spells/heal.yaml`, `_datafiles/world/dogmud/spells/mend-wounds.yaml`, `_datafiles/world/dogmud/spells/mend-all.yaml`, `_datafiles/world/dogmud/spells/communion-of-flesh.yaml`, `_datafiles/world/dogmud/spells/mass-mend.yaml`, `_datafiles/world/dogmud/spells/repair-pulse.yaml`, `_datafiles/world/dogmud/templates/help/heal.template`, `_datafiles/world/dogmud/templates/help/mend-wounds.template`, `_datafiles/world/dogmud/templates/help/mend-all.template`, `_datafiles/world/dogmud/templates/help/communion-of-flesh.template`, `_datafiles/world/dogmud/templates/help/mass-mend.template`, `_datafiles/world/dogmud/templates/help/healing-spells.template`, `_datafiles/world/dogmud/keywords.yaml`, `internal/hooks/spell_help_effects.go`, `internal/hooks/spell_resolution.go`, `internal/hooks/nonharm_mob_shortcut_test.go`, `internal/hooks/spell_channel_sight_test.go`, `internal/hooks/spell_help_area_test.go`, `condition_apply_path_guard_test.go`, `shipped_narration_data_guard_test.go`, `internal/narration/testdata/stores/conditions.golden` |
| 11 | Every spell-landed condition tells its own start (#370 row 1) | `spell_condition_data_guard_test.go`, `_datafiles/world/dogmud/conditions/1-illumination.yaml`, `_datafiles/world/dogmud/conditions/2-stunned.yaml`, `_datafiles/world/dogmud/conditions/3-blinded.yaml`, `_datafiles/world/dogmud/conditions/26-conviction_surge.yaml`, `_datafiles/world/dogmud/conditions/27-iron_will.yaml`, `_datafiles/world/dogmud/conditions/28-chrysalis_haste.yaml`, `_datafiles/world/dogmud/conditions/30-nerve_disruption.yaml`, `_datafiles/world/dogmud/conditions/31-empathic_shroud.yaml`, `_datafiles/world/dogmud/conditions/32-vital_surge.yaml`, `_datafiles/world/dogmud/conditions/33-chrysalis_regeneration.yaml`, `_datafiles/world/dogmud/conditions/34-skill_attunement.yaml`, `_datafiles/world/dogmud/conditions/35-mutation_catalyst.yaml`, `_datafiles/world/dogmud/conditions/36-psychic_anchor.yaml`, `_datafiles/world/dogmud/conditions/37-sensory_overload.yaml`, `_datafiles/world/dogmud/conditions/38-conviction_armor.yaml`, `_datafiles/world/dogmud/conditions/41-mind_fog.yaml`, `_datafiles/world/dogmud/conditions/53-veil_sight.yaml`, `_datafiles/world/dogmud/conditions/128-night_sight.yaml`, `_datafiles/world/dogmud/conditions/129-heat_sight.yaml`, `_datafiles/world/dogmud/conditions/131-chrysalis_pall.yaml`, `shipped_narration_data_guard_test.go`, `internal/narration/testdata/stores/conditions.golden` |
| 12 | Docs, patch notes and the guard's README row | `internal/conditions/context.md`, `internal/characters/context.md`, `internal/events/context.md`, `internal/users/context.md`, `internal/mobs/context.md`, `internal/actions/context.md`, `internal/behaviortree/context.md`, `internal/combat/context.md`, `internal/hooks/context.md`, `docs/PATCH_NOTES.md`, `docs/README.md` |
| 13 | Whole-tree verification and boot check | (no files) |
| 14 | Playtest (controller) | (no files) |

**Order.** Run the tasks in number order. Tasks 1 to 3 share `internal/conditions`; 5 needs 1 (`Stamp`); 6 needs 1, 2, 3, 5; 7 needs 5 and 6 (the guard file); 8 needs 6 and 7; 9 needs 4, 6, 7, 8; 10 needs 9 (the guard file, `spell_help_effects.go`); 11 needs 10; 12 documents all of them.

**Parallel lanes.** Run a task beside another only when their files AND Go packages are disjoint, because a half-applied task breaks the other's compile. Safe pairs: Task 4 (`characters`, one `conditions` file) beside Task 3 only if Task 3's `conditions` edits land first; Task 12 (docs only) beside Task 11 (YAML and two root tests). Everything else runs in sequence: the condition record, the hook and the root guards are shared by nearly every task.

New files: `internal/conditions/caster.go`, `family.go`, their tests, `internal/hooks/condition_cast_lines.go`, `internal/actions/actor_ref.go` and the other new tests are package files. The one new root guard, `spell_condition_data_guard_test.go`, gets a `docs/README.md` row (Task 12). Eight new condition files (135 to 142), one spell (`conviction-bulwark.yaml`) and three help pages (`conviction-bulwark`, `wards`, `healing-spells`) are content.

## Rules for every task

- Edit files with the Edit tool (or Write for a new file, or a whole-file rewrite the step gives in full). Never `sed -i`, never a Python read-modify-write. Never `git add -A` or `git add .`: stage the named paths the task lists.
- Each "Replace in `path`" block below is one Edit call: `old_string` is the first block, `new_string` the second, and the old text matches exactly once at that point. Apply a file's blocks in the order given.
- `gofmt -l` on every touched Go file prints nothing; run `gofmt -w <file>` if it does.
- Run targeted tests, then the root guards: `go test . -count=1` (about a minute) at the end of every task.
- `grep -c` exits 1 on zero matches; run an "expect zero" check on its own line, never inside an `&&` chain.
- No em dash or en dash in any Go string, comment, YAML text, template, doc or commit message.
- Player text: 80 columns or less after names are filled, no raw numbers for damage, healing, protection or durations, ESL-clear, no line ending in a colon (`dogmud-player-copy`). Help pages may show fold counts and costs, as every spell page does.
- Sight-aware narration: a line that names a party goes through `HideNames` or a `...HidingNames` sender at the reader's sight; never `Room.SendText` for something seen.
- Balance numbers come from `git show HEAD:_datafiles/config.yaml`, never a Go default. No task edits `_datafiles/config.yaml` (skip-worktree).
- `context.md` is updated for every API change (Task 12 does all of them; a task executed alone must still leave its package's `context.md` true, so if Task 12 is not run in the same PR, move its block for that package into the task).
- Commit messages end with a blank line, then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. No "closes", "fixes" or "resolves #N" anywhere, commit or PR; write "Refs #N".
- A test fails first: each task's first run is its failing run, and the expected failure is given. If a new test passes before its implementation, stop and find out why. Exceptions by design, each named in its step: pins that hold before and after (`TestStamp_UnheldIsANoOp`, `TestUserRecord_OldDoorsQueueNoCaster`, `TestConditionCast_SelfCastTellsTheHolderAndTheRoom`, `TestDotTick_NoCasterHarmsAnonymously`, `TestSpellConditionLines_ASilentConditionKeepsTheTrio`, `TestSpellConditionLines_ARecastOfAnActiveConditionKeepsTheTrio`, `TestSpellConditionIdsAreReadByTheirEffectType`).

---

### Task 1: The caster on a condition record (#240)

`conditions.Condition` gains `Caster state.ActorRef` and `Conditions.Stamp`, which writes the applier's source and caster on a held record; the newest application owns it. Only a player caster is saved: a mob's instance id names a different creature after a restart.

- [ ] **Step 1: Write the failing test.**

Create `internal/conditions/caster_test.go`:

```go
package conditions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yamlv2 "gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"
)

const casterTestConditionId = 9311

func seedCasterTestSpec(t *testing.T) {
	t.Helper()
	restore := SeedConditionsForTest(map[int]*ConditionSpec{
		casterTestConditionId: {ConditionId: casterTestConditionId, Name: "Caster Test", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
	})
	t.Cleanup(restore)
}

// Stamp writes the newest applier onto the held record: a re-application
// by someone else takes the record (spec section 1, "the newest application
// owns the record"), and a zero caster (a potion re-applying it) clears it.
func TestStamp_NewestApplicationOwnsTheRecord(t *testing.T) {
	seedCasterTestSpec(t)
	bs := New()
	require.True(t, bs.AddCondition(casterTestConditionId, false))

	bs.Stamp(casterTestConditionId, "spell", state.ActorRef{UserId: 7})
	got := bs.GetConditions(casterTestConditionId)
	require.Len(t, got, 1)
	assert.Equal(t, state.ActorRef{UserId: 7}, got[0].Caster)
	assert.Equal(t, "spell", got[0].Source)

	require.True(t, bs.AddCondition(casterTestConditionId, false))
	bs.Stamp(casterTestConditionId, "spell", state.ActorRef{MobInstanceId: 42})
	assert.Equal(t, state.ActorRef{MobInstanceId: 42}, bs.GetConditions(casterTestConditionId)[0].Caster)

	bs.Stamp(casterTestConditionId, "potion", state.ActorRef{})
	assert.True(t, bs.GetConditions(casterTestConditionId)[0].Caster.IsZero())
	assert.Equal(t, "potion", bs.GetConditions(casterTestConditionId)[0].Source)
}

// Stamp on a record that is not held does nothing and does not panic.
func TestStamp_UnheldIsANoOp(t *testing.T) {
	seedCasterTestSpec(t)
	bs := New()
	bs.Stamp(casterTestConditionId, "spell", state.ActorRef{UserId: 7})
	assert.Empty(t, bs.List)
}

// A player caster survives a save (user saves and copyover files are
// yaml.v2); a mob caster does not, because Mob.InstanceId is runtime only and
// would name a different creature after a restart (spec section 1,
// "Persistence"). A record with no caster writes no caster key at all.
func TestCaster_PlayerSurvivesASaveAndAMobDoesNot(t *testing.T) {
	list := []*Condition{
		{ConditionId: 1, TriggersLeft: 3, Caster: state.ActorRef{UserId: 7}},
		{ConditionId: 2, TriggersLeft: 3, Caster: state.ActorRef{MobInstanceId: 42}},
		{ConditionId: 3, TriggersLeft: 3},
	}

	for name, rt := range map[string]struct {
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		"yaml.v2": {yamlv2.Marshal, yamlv2.Unmarshal},
		"yaml.v3": {yamlv3.Marshal, yamlv3.Unmarshal},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := rt.marshal(list)
			require.NoError(t, err)
			assert.NotContains(t, string(out), "42", "a mob caster's instance id is never written")
			assert.Equal(t, 1, strings.Count(string(out), "caster:"),
				"only the player caster writes a caster key; a stripped mob caster and no caster write none")

			var back []*Condition
			require.NoError(t, rt.unmarshal(out, &back))
			require.Len(t, back, 3)
			assert.Equal(t, state.ActorRef{UserId: 7}, back[0].Caster)
			assert.True(t, back[1].Caster.IsZero(), "a mob caster reads as no caster after a load")
			assert.True(t, back[2].Caster.IsZero())
			assert.Equal(t, 3, back[1].TriggersLeft, "the rest of the record round-trips")
		})
	}
}

// A save written before a mob caster was stripped (a hand edit, an old
// copyover file) still loads with no caster.
func TestCaster_LoadDropsAMobCasterFromAnOldSave(t *testing.T) {
	src := "conditionid: 2\ntriggersleft: 3\ncaster:\n  userid: 0\n  mobinstanceid: 42\n"
	var c Condition
	require.NoError(t, yamlv2.Unmarshal([]byte(src), &c))
	assert.True(t, c.Caster.IsZero())
	assert.Equal(t, 2, c.ConditionId)
}
```

- [ ] **Step 2: Run it to see it fail.**

Run: `go test ./internal/conditions/ -run "Stamp|Caster" -count=1`
Expected: FAIL, build failed: `bs.Stamp undefined (type Conditions has no field or method Stamp)` and `unknown field Caster in struct literal of type Condition`.

- [ ] **Step 3: Add the field and the stamp.**

Replace in `internal/conditions/conditions.go` (1 of 2):

```go
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/state"
)
```

Replace in `internal/conditions/conditions.go` (2 of 2):

```go
	Hooded      bool      `yaml:"hooded,omitempty"`
}
```

with:

```go
	Hooded      bool      `yaml:"hooded,omitempty"`

	// Caster is who put this record on its holder: a spell's caster, the
	// attacker whose blow opened a bleed. Zero for a record nothing cast (a
	// potion, a hazard, a mutation, gear). The newest application owns the
	// record (Stamp). Only a player caster is saved; see caster.go.
	Caster state.ActorRef `yaml:"caster,omitempty"`
}
```

Create `internal/conditions/caster.go`:

```go
package conditions

import "github.com/GoMudEngine/GoMud/internal/state"

// Stamp records who applied a held record and from what, after an add door
// landed it (messaging M6 slice 1, section 1). The newest application owns
// the record: a re-application by a different caster takes it over, and a
// re-application with a zero caster (a potion re-applying a spell's record)
// leaves it with none. Source is stamped beside it, so every door, not only
// the magnitude one, carries the applier's source. A record not held is left
// alone.
func (bs *Conditions) Stamp(conditionId int, source string, caster state.ActorRef) {
	idx, ok := bs.conditionIds[conditionId]
	if !ok {
		return
	}
	bs.List[idx].Source = source
	bs.List[idx].Caster = caster
}

// plainCondition is Condition without its YAML methods, so MarshalYAML and
// UnmarshalYAML can hand the struct to the encoder without recursing.
type plainCondition Condition

// MarshalYAML writes a record with its caster's player half only. A mob's
// InstanceId is runtime only (mobs.Mob.InstanceId is `yaml:"-"`), so after a
// restart the same number names a different creature, or none: a saved mob
// caster would credit a stranger. A record whose caster was a mob is saved
// with no caster.
func (b Condition) MarshalYAML() (interface{}, error) {
	p := plainCondition(b)
	p.Caster.MobInstanceId = 0
	return p, nil
}

// UnmarshalYAML loads a record and drops any mob caster it carries, for a
// save written before MarshalYAML stripped it. The yaml.v2 form, which
// yaml.v3 also honours.
func (b *Condition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var p plainCondition
	if err := unmarshal(&p); err != nil {
		return err
	}
	*b = Condition(p)
	b.Caster.MobInstanceId = 0
	return nil
}
```

- [ ] **Step 4: Run the tests and the packages that save conditions.**

Run: `go test ./internal/conditions/ -run "Stamp|Caster" -count=1`
Expected: PASS. `TestStamp_UnheldIsANoOp` is a pin and would pass alone; the file failed to build before.

Run: `go test ./internal/conditions/ ./internal/characters/ ./internal/users/ -count=1`
Expected: PASS. `ok` for all three.

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 5: Commit.**

```bash
git add internal/conditions/caster_test.go \
  internal/conditions/conditions.go \
  internal/conditions/caster.go
git commit -F - <<'EOF'
feat(conditions): a condition record remembers its caster (#240)

Condition.Caster names who applied a record, and Conditions.Stamp writes
the applier's source and caster on the held record: the newest
application owns it. Only a player caster is saved; a mob's instance id
is runtime only, so MarshalYAML and UnmarshalYAML drop it.

Refs #240, Refs #370.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Condition families (#338, R4, R6)

A closed `family` (`ward`, `heal`) on `ConditionSpec`. The add primitives discard every other record of the family before landing a new one, after their refusal checks; `FamilyRivals` names what an add would replace and `HasFamily` answers the mob AI.

- [ ] **Step 1: Write the failing test.**

Create `internal/conditions/family_test.go`:

```go
package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	familyTestWardA  = 9321 // ward family
	familyTestWardB  = 9322 // ward family
	familyTestHeal   = 9323 // heal family
	familyTestPotion = 9324 // no family, a potion-like statmod record
)

func seedFamilyTestSpecs(t *testing.T) {
	t.Helper()
	restore := SeedConditionsForTest(map[int]*ConditionSpec{
		familyTestWardA:  {ConditionId: familyTestWardA, Name: "Ward A", Family: FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
		familyTestWardB:  {ConditionId: familyTestWardB, Name: "Ward B", Family: FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
		familyTestHeal:   {ConditionId: familyTestHeal, Name: "Heal", Family: FamilyHeal, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
		familyTestPotion: {ConditionId: familyTestPotion, Name: "Potion", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
	})
	t.Cleanup(restore)
}

// R4 and R6: landing a ward removes every other ward first, on every add
// door, and the newest stands alone.
func TestFamily_NewestReplacesTheOlderOnEveryDoor(t *testing.T) {
	seedFamilyTestSpecs(t)

	doors := map[string]func(bs *Conditions, id int) bool{
		"AddCondition":          func(bs *Conditions, id int) bool { return bs.AddCondition(id, false) },
		"AddConditionScaled":    func(bs *Conditions, id int) bool { return bs.AddConditionScaled(id, 2.0) },
		"AddConditionMagnitude": func(bs *Conditions, id int) bool { return bs.AddConditionMagnitude(id, 4, 10) },
	}
	for name, add := range doors {
		t.Run(name, func(t *testing.T) {
			bs := New()
			require.True(t, add(&bs, familyTestWardA))
			require.True(t, add(&bs, familyTestPotion))
			require.True(t, add(&bs, familyTestWardB))

			assert.False(t, bs.HasCondition(familyTestWardA), "the older ward is gone")
			assert.True(t, bs.HasCondition(familyTestWardB), "the newest ward stands")
			assert.True(t, bs.HasCondition(familyTestPotion), "a record with no family is untouched")
			assert.Len(t, bs.List, 2)
		})
	}
}

// Re-landing the same id is a refresh, not a replacement: the record stays
// and nothing else is removed.
func TestFamily_SameIdIsARefresh(t *testing.T) {
	seedFamilyTestSpecs(t)
	bs := New()
	require.True(t, bs.AddConditionMagnitude(familyTestWardA, 4, 10))
	require.True(t, bs.AddConditionMagnitude(familyTestWardA, 6, 12))

	require.Len(t, bs.List, 1)
	assert.Equal(t, 6, bs.List[0].TriggersLeft)
	assert.Equal(t, 12.0, bs.List[0].Magnitude)
}

// Families are separate: a heal does not replace a ward.
func TestFamily_DifferentFamiliesCoexist(t *testing.T) {
	seedFamilyTestSpecs(t)
	bs := New()
	require.True(t, bs.AddCondition(familyTestWardA, false))
	require.True(t, bs.AddCondition(familyTestHeal, false))
	assert.True(t, bs.HasCondition(familyTestWardA))
	assert.True(t, bs.HasCondition(familyTestHeal))
}

// FamilyRivals names the live records an add of conditionId would replace,
// for the replacement line; an expired, unpruned rival is not news.
func TestFamilyRivals_ListsLiveRivalsOnly(t *testing.T) {
	seedFamilyTestSpecs(t)
	bs := New()
	assert.Empty(t, bs.FamilyRivals(familyTestWardB), "nothing held")

	require.True(t, bs.AddCondition(familyTestWardA, false))
	require.True(t, bs.AddCondition(familyTestPotion, false))
	rivals := bs.FamilyRivals(familyTestWardB)
	require.Len(t, rivals, 1)
	assert.Equal(t, familyTestWardA, rivals[0].ConditionId)

	assert.Empty(t, bs.FamilyRivals(familyTestWardA), "the same id is a refresh, never its own rival")
	assert.Empty(t, bs.FamilyRivals(familyTestPotion), "a record with no family has no rivals")

	bs.RemoveCondition(familyTestWardA)
	assert.Empty(t, bs.FamilyRivals(familyTestWardB), "an expired rival is left to the prune pass")
}

// HasFamily is the mob AI's "already shielded" and "already healing" test.
func TestHasFamily(t *testing.T) {
	seedFamilyTestSpecs(t)
	bs := New()
	assert.False(t, bs.HasFamily(FamilyWard))
	require.True(t, bs.AddCondition(familyTestWardA, false))
	assert.True(t, bs.HasFamily(FamilyWard))
	assert.False(t, bs.HasFamily(FamilyHeal))
	bs.RemoveCondition(familyTestWardA)
	assert.False(t, bs.HasFamily(FamilyWard), "an expired record is not held")
}

// A family name the engine does not know is a typo that would make the
// record stack silently; the load refuses it.
func TestValidate_RefusesAnUnknownFamily(t *testing.T) {
	spec := &ConditionSpec{ConditionId: 9325, Name: "Typo", Family: "wards"}
	err := spec.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"wards"`)

	for _, f := range AllFamilies {
		ok := &ConditionSpec{ConditionId: 9326, Name: "Fine", Family: f}
		assert.NoError(t, ok.Validate(), f)
	}
}
```

- [ ] **Step 2: Run it to see it fail.**

Run: `go test ./internal/conditions/ -run Family -count=1`
Expected: FAIL, build failed: `unknown field Family in struct literal of type ConditionSpec`, `undefined: FamilyWard`.

- [ ] **Step 3: Add the family field, its validation and the rival discard.**

Replace in `internal/conditions/conditionspec.go` (1 of 2):

```go

	// YAML text fields: flavor text sent by the engine (replaces JS messaging).
```

with:

```go

	// Family names the set this record replaces within instead of stacking:
	// FamilyWard or FamilyHeal (family.go). Empty for every other record.
	Family string `yaml:"family,omitempty"`

	// YAML text fields: flavor text sent by the engine (replaces JS messaging).
```

Replace in `internal/conditions/conditionspec.go` (2 of 2):

```go

	// Validate tick fields
```

with:

```go

	if err := b.validateFamily(); err != nil {
		return err
	}

	// Validate tick fields
```

Create `internal/conditions/family.go`:

```go
package conditions

import (
	"fmt"
	"slices"
)

// A family is a set of conditions that replace one another instead of
// stacking (messaging M6 slice 1, section 3; owner rulings R4 and R6): the
// shield spells' wards, and the heal spells' heals. Landing a record whose
// spec names a family removes every other record of that family on the same
// holder first, so the newest stands alone. A record with no family (a
// potion, a salve, an item, a mutation) is never touched, so it keeps
// stacking with a ward or a heal.
const (
	FamilyWard = "ward"
	FamilyHeal = "heal"
)

// AllFamilies is every family the engine understands. The load refuses any
// other name: a misspelt family would let the record stack silently.
var AllFamilies = []string{FamilyWard, FamilyHeal}

// validateFamily refuses a family the engine does not declare.
func (b *ConditionSpec) validateFamily() error {
	if b.Family == "" || slices.Contains(AllFamilies, b.Family) {
		return nil
	}
	return fmt.Errorf("conditionId %d (%s) names unknown family %q; see conditions.AllFamilies", b.ConditionId, b.Name, b.Family)
}

// HasFamily reports whether any held, unexpired record belongs to family.
// The mob AI's "already shielded" and "already healing" test.
func (bs *Conditions) HasFamily(family string) bool {
	if family == "" {
		return false
	}
	for _, b := range bs.List {
		if b.Expired() {
			continue
		}
		if spec := GetConditionSpec(b.ConditionId); spec != nil && spec.Family == family {
			return true
		}
	}
	return false
}

// FamilyRivals lists the held, unexpired records an add of conditionId would
// replace: every other id of the same family. Empty for a spec with no family
// and for the same id (a refresh). The apply hook reads it BEFORE the add, to
// tell the holder which ward or heal gave way.
func (bs *Conditions) FamilyRivals(conditionId int) []*Condition {
	spec := GetConditionSpec(conditionId)
	if spec == nil || spec.Family == "" {
		return nil
	}
	var out []*Condition
	for _, b := range bs.List {
		if b.ConditionId == conditionId || b.Expired() {
			continue
		}
		if other := GetConditionSpec(b.ConditionId); other != nil && other.Family == spec.Family {
			out = append(out, b)
		}
	}
	return out
}

// discardFamilyRivals deletes every other record of spec's family, live or
// expired, with no end line (Discard): the hook tells the replacement line
// in its place. Called by the add primitives after their refusal checks, so
// a refused add (poison immunity) keeps the rival.
func (bs *Conditions) discardFamilyRivals(spec *ConditionSpec) {
	if spec.Family == "" {
		return
	}
	for _, b := range slices.Clone(bs.List) {
		if b.ConditionId == spec.ConditionId {
			continue
		}
		if other := GetConditionSpec(b.ConditionId); other != nil && other.Family == spec.Family {
			bs.Discard(b.ConditionId)
		}
	}
}
```

Replace in `internal/conditions/conditions.go` (1 of 2):

```go

		triggers := int(float64(conditionInfo.TriggerCount) * durationMult)
```

with:

```go

		// A ward or a heal replaces its family's other record (family.go).
		bs.discardFamilyRivals(conditionInfo)

		triggers := int(float64(conditionInfo.TriggerCount) * durationMult)
```

Replace in `internal/conditions/conditions.go` (2 of 2):

```go
			return false
		}

		newCondition := Condition{
```

with:

```go
			return false
		}

		// A ward or a heal replaces its family's other record (family.go).
		bs.discardFamilyRivals(conditionInfo)

		newCondition := Condition{
```

- [ ] **Step 4: Run the package and the root guards.**

Run: `go test ./internal/conditions/ -count=1`
Expected: PASS. `ok`.

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 5: Commit.**

```bash
git add internal/conditions/family_test.go \
  internal/conditions/conditionspec.go \
  internal/conditions/family.go \
  internal/conditions/conditions.go
git commit -F - <<'EOF'
feat(conditions): ward and heal families replace instead of stacking (#338)

A condition may name a family (ward or heal). Landing one discards every
other record of its family on the holder first, with no end line, after
the refusal checks. FamilyRivals names what an add would replace, for the
replacement line; HasFamily is the mob AI's "already up" test. Records
with no family keep stacking.

Refs #338, Refs #370.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 3: The caster's start line (`start_actor`, `{actor}`)

`ConditionSpec.StartActorText` (`start_actor`) is the caster's line; `NarrateCast` fills `{actor}`; `NarratesCastStart` is the rule that drops a spell's generic trio (R11). `{actor}` outside the start phase fails the load.

- [ ] **Step 1: Write the failing test.**

Create `internal/conditions/narration_caster_test.go`:

```go
package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wardSpecForNarration() *ConditionSpec {
	return &ConditionSpec{
		ConditionId:    502,
		Name:           "Test Ward",
		StartActorText: "A ward settles over {actee}.",
		StartUserText:  "A ward settles over you.",
		StartRoomText:  "A ward settles over {actee_plain}.",
	}
}

// start_actor is the caster's line (messaging M6 slice 1, section 2): the
// Actor variant of the start phase. No other phase has a caster line.
func TestNarration_StartActorIsTheCasterLine(t *testing.T) {
	s := wardSpecForNarration()
	v := s.Narration(PhaseStart)
	require.Len(t, v.Actor, 1)
	assert.Equal(t, "A ward settles over {actee}.", v.Actor[0])
	assert.Empty(t, s.Narration(PhaseEnd).Actor)
	assert.Empty(t, s.Narration(PhaseTrigger).Actor)
}

// The caster line obeys the same silences as the holder line: a secret,
// quiet or silent-start record says nothing at start.
func TestNarration_StartActorFollowsTheNoticeSilences(t *testing.T) {
	for _, mut := range []func(*ConditionSpec){
		func(s *ConditionSpec) { s.Secret = true },
		func(s *ConditionSpec) { s.Flags = []Flag{Quiet} },
		func(s *ConditionSpec) { s.Flags = []Flag{SilentStart} },
	} {
		s := wardSpecForNarration()
		mut(s)
		assert.Empty(t, s.StartActorNotice())
		assert.Empty(t, s.Narration(PhaseStart).Actor)
	}
}

// NarrateCast fills {actor} with the caster and {actee} with the holder in
// every start line; Narrate is NarrateCast with no caster.
func TestNarrateCast_FillsTheCasterAndTheHolder(t *testing.T) {
	s := wardSpecForNarration()
	s.StartUserText = "{actor} wraps a ward around you."
	roles := s.NarrateCast(PhaseStart, "Bob", "Bob", "Alice", "Alice")
	assert.Equal(t, "A ward settles over Bob.", roles.Actor)
	assert.Equal(t, "Alice wraps a ward around you.", roles.Actee)
	assert.Equal(t, "A ward settles over Bob.", roles.Observer)

	plain := wardSpecForNarration().Narrate(PhaseStart, "Bob", "Bob")
	assert.Equal(t, "A ward settles over you.", plain.Actee)
}

// NarratesCastStart is the rule that drops a spell's generic "takes effect"
// trio (R11): only when the condition's own start lines will reach each
// audience the trio served.
func TestNarratesCastStart(t *testing.T) {
	s := wardSpecForNarration()
	assert.True(t, s.NarratesCastStart(true), "self-cast: the holder line is the caster's line")
	assert.True(t, s.NarratesCastStart(false))

	noActor := wardSpecForNarration()
	noActor.StartActorText = ""
	assert.True(t, noActor.NarratesCastStart(true), "a self-cast needs no caster line")
	assert.False(t, noActor.NarratesCastStart(false), "a cast on another with no caster line keeps the trio")

	silent := wardSpecForNarration()
	silent.Flags = []Flag{SilentStart}
	assert.False(t, silent.NarratesCastStart(true))
	assert.False(t, silent.NarratesCastStart(false))

	noRoom := wardSpecForNarration()
	noRoom.StartRoomText = ""
	assert.False(t, noRoom.NarratesCastStart(true), "the room would lose its line")

	generic := wardSpecForNarration()
	generic.StartUserText = ""
	assert.False(t, generic.NarratesCastStart(true), "the generic takes-effect fallback is not authored text")
}

// {actor} names the caster, who is known only at start. Trigger and end lines
// are narrated with no caster, so the load refuses {actor} there rather than
// render an empty name.
func TestValidate_RefusesTheActorTokenOutsideTheStartPhase(t *testing.T) {
	for _, mut := range []func(*ConditionSpec){
		func(s *ConditionSpec) { s.TriggerUserText = "{actor}'s ward hums." },
		func(s *ConditionSpec) { s.TriggerRoomText = "{actor_plain}'s ward hums." },
		func(s *ConditionSpec) { s.EndUserText = "{actor}'s ward fades." },
		func(s *ConditionSpec) { s.EndRoomText = "{actor}'s ward fades." },
	} {
		s := wardSpecForNarration()
		mut(s)
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "{actor}")
	}
	assert.NoError(t, wardSpecForNarration().Validate())
}

// start_actor is checked like every other line: an unknown token fails the
// load.
func TestValidate_ChecksStartActorTokens(t *testing.T) {
	s := wardSpecForNarration()
	s.StartActorText = "A ward settles over {target}."
	assert.Error(t, s.Validate())
}
```

- [ ] **Step 2: Run it to see it fail.**

Run: `go test ./internal/conditions/ -run "Narrat|Validate" -count=1`
Expected: FAIL, build failed: `unknown field StartActorText`, `s.StartActorNotice undefined`, `s.NarrateCast undefined`, `s.NarratesCastStart undefined`.

- [ ] **Step 3: Add the caster line.**

Replace in `internal/conditions/conditionspec.go` (1 of 2):

```go
	// the holder while a spell's means the spell's target.
	StartUserText   string `yaml:"start_actee,omitempty"`
```

with:

```go
	// the holder while a spell's means the spell's target.
	//
	// start_actor is the CASTER's line when the caster is someone other than
	// the holder (messaging M6 slice 1, section 2): it names the holder with
	// {actee}. Start lines may also name the caster with {actor}; trigger and
	// end lines may not, because no caster is known when they are told.
	StartActorText  string `yaml:"start_actor,omitempty"`
	StartUserText   string `yaml:"start_actee,omitempty"`
```

Replace in `internal/conditions/conditionspec.go` (2 of 2):

```go
	for _, text := range []string{
		b.StartUserText, b.StartRoomText,
		b.TriggerUserText, b.TriggerRoomText,
```

with:

```go
	for _, text := range []string{
		b.StartActorText, b.StartUserText, b.StartRoomText,
		b.TriggerUserText, b.TriggerRoomText,
```

Replace in `internal/conditions/notice.go` (1 of 1):

```go
	return fmt.Sprintf("%s takes effect.", b.Name)
}
```

with:

```go
	return fmt.Sprintf("%s takes effect.", b.Name)
}

// StartActorNotice is the line the CASTER reads when this condition lands on
// someone else: the authored start_actor, silenced by the same flags as
// StartUserNotice. There is no generic fallback: a condition with no caster
// line keeps the spell's own lines instead (NarratesCastStart).
func (b *ConditionSpec) StartActorNotice() string {
	if b.Secret || b.hasFlag(Quiet) || b.hasFlag(SilentStart) {
		return ""
	}
	return b.StartActorText
}

// NarratesCastStart reports whether this condition's own start lines will
// tell every audience of a spell landing it, so the spell drops its generic
// "takes effect" lines (owner ruling R11): an authored holder line and room
// line, and, when the caster is someone else, an authored caster line. The
// generic "<Name> takes effect." fallback does not count, and neither does a
// silent start. selfCast: the caster is the holder, so the holder line is the
// caster's line and no start_actor is needed. A refresh of a record already
// held narrates no start, and the caller checks that separately.
func (b *ConditionSpec) NarratesCastStart(selfCast bool) bool {
	if b.StartUserNotice() == "" || b.StartUserText == "" || b.StartRoomText == "" {
		return false
	}
	return selfCast || b.StartActorNotice() != ""
}
```

Replace in `internal/conditions/narration.go` (1 of 3):

```go
	"fmt"

```

with:

```go
	"fmt"
	"strings"

```

Replace in `internal/conditions/narration.go` (2 of 3):

```go
// The holder's line is the ACTEE: the condition happens to them. The room's line is
// the Observer. Actor is empty and reserved for the caster, which M6 authors
// once events.Condition carries a caster (owner ruling, 2026-09-12). Start and End
// go through StartUserNotice / EndUserNotice, so the secret, hidden,
// silent-start and generic-fallback rules stay in their one door.
func (b *ConditionSpec) Narration(p Phase) narration.Variants {
	var holder, room string
	switch p {
	case PhaseStart:
		holder, room = b.StartUserNotice(), b.StartRoomText
	case PhaseTrigger:
		holder, room = b.TriggerUserText, b.TriggerRoomText
	case PhaseEnd:
		holder, room = b.EndUserNotice(), b.EndRoomText
	}
	return narration.Variants{Actee: textutil.Pool(holder), Observer: textutil.Pool(room)}
}

// Narrate renders one phase for its audiences. It takes the HOLDER, not a
// token context: the holder is always the actee (a condition happens to them),
// and building the context here means no call site can put the name in the
// wrong slot. Actor stays empty until events.Condition carries a caster (M6).
func (b *ConditionSpec) Narrate(p Phase, holderName, holderPlainName string) narration.Roles {
	return textutil.Narrate(b.Narration(p), textutil.TokenContext{
		ActeeName:      holderName,
```

with:

```go
// The holder's line is the ACTEE: the condition happens to them. The room's line is
// the Observer. Actor is the CASTER's line, start_actor, and exists only at
// start (messaging M6 slice 1, section 2). Start and End go through
// StartUserNotice / StartActorNotice / EndUserNotice, so the secret, hidden,
// silent-start and generic-fallback rules stay in their one door.
func (b *ConditionSpec) Narration(p Phase) narration.Variants {
	var caster, holder, room string
	switch p {
	case PhaseStart:
		caster, holder, room = b.StartActorNotice(), b.StartUserNotice(), b.StartRoomText
	case PhaseTrigger:
		holder, room = b.TriggerUserText, b.TriggerRoomText
	case PhaseEnd:
		holder, room = b.EndUserNotice(), b.EndRoomText
	}
	return narration.Variants{Actor: textutil.Pool(caster), Actee: textutil.Pool(holder), Observer: textutil.Pool(room)}
}

// Narrate renders one phase for its audiences with no caster. It takes the
// HOLDER, not a token context: the holder is always the actee (a condition
// happens to them), and building the context here means no call site can put
// the name in the wrong slot.
func (b *ConditionSpec) Narrate(p Phase, holderName, holderPlainName string) narration.Roles {
	return b.NarrateCast(p, holderName, holderPlainName, "", "")
}

// NarrateCast is Narrate with the record's caster, which fills {actor} and
// {actor_plain}. Only the start phase may name the caster (validateNarration).
func (b *ConditionSpec) NarrateCast(p Phase, holderName, holderPlainName, casterName, casterPlainName string) narration.Roles {
	return textutil.Narrate(b.Narration(p), textutil.TokenContext{
		ActorName:      casterName,
		ActorPlainName: casterPlainName,
		ActeeName:      holderName,
```

Replace in `internal/conditions/narration.go` (3 of 3):

```go
func (b *ConditionSpec) validateNarration() error {
	phases := []struct{ name, user, room string }{
		{"start", b.StartUserText, b.StartRoomText},
		{"trigger", b.TriggerUserText, b.TriggerRoomText},
		{"end", b.EndUserText, b.EndRoomText},
	}
	for _, ph := range phases {
		if ph.user == "" && ph.room == "" {
			continue
		}
		v := narration.Variants{Actee: textutil.Pool(ph.user), Observer: textutil.Pool(ph.room)}
		// No expected role set: a condition may legitimately author only a holder
```

with:

```go
func (b *ConditionSpec) validateNarration() error {
	phases := []struct{ name, actor, user, room string }{
		{"start", b.StartActorText, b.StartUserText, b.StartRoomText},
		{"trigger", "", b.TriggerUserText, b.TriggerRoomText},
		{"end", "", b.EndUserText, b.EndRoomText},
	}
	for _, ph := range phases {
		if ph.actor == "" && ph.user == "" && ph.room == "" {
			continue
		}
		// {actor} is the caster, known only when the record lands. A trigger
		// or end line is told with no caster, so it would render empty.
		if ph.name != "start" {
			for _, text := range []string{ph.user, ph.room} {
				if strings.Contains(text, narration.TokenActor) || strings.Contains(text, narration.TokenActorPlain) {
					return fmt.Errorf("conditionId %d (%s) %s text names the caster with {actor}; only start lines know the caster", b.ConditionId, b.Name, ph.name)
				}
			}
		}
		v := narration.Variants{Actor: textutil.Pool(ph.actor), Actee: textutil.Pool(ph.user), Observer: textutil.Pool(ph.room)}
		// No expected role set: a condition may legitimately author only a holder
```

- [ ] **Step 4: Run the package and the root guards.**

Run: `go test ./internal/conditions/ -count=1`
Expected: PASS. `ok` (the older `TestNarrationStartPutsTheHolderInActeeAndUsesTheNotice` still finds no Actor line on a spec with no `start_actor`).

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 5: Commit.**

```bash
git add internal/conditions/narration_caster_test.go \
  internal/conditions/conditionspec.go \
  internal/conditions/notice.go \
  internal/conditions/narration.go
git commit -F - <<'EOF'
feat(conditions): start_actor and {actor}, the caster's start line

A condition may author start_actor, the line its caster reads when it
lands on someone else, and its start lines may name the caster with
{actor} (NarrateCast). Trigger and end lines may not: no caster is known
there. NarratesCastStart is the rule a spell uses to drop its generic
trio.

Refs #370.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Ward effects for spells and words

Two effect kinds, `mitigation_magical` and `mitigation_conviction`, summed into magical and conviction mitigation beside the statmods, as `mitigation_flat` is into physical.

- [ ] **Step 1: Write the failing test.**

Create `internal/characters/ward_mitigation_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	wardTestPhysical = 9331 // mitigation_flat only
	wardTestSpell    = 9332 // mitigation_flat and mitigation_magical
	wardTestAll      = 9333 // all three kinds
	wardTestPotion   = 9334 // a magical_mitigation statmod, no family
)

func seedWardMitigationSpecs(t *testing.T) {
	t.Helper()
	mag := conditions.EffectValue{UsesMagnitude: true}
	restore := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		wardTestPhysical: {ConditionId: wardTestPhysical, Name: "Physical Ward", Family: conditions.FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectMitigationFlat: mag}},
		wardTestSpell: {ConditionId: wardTestSpell, Name: "Spell Ward", Family: conditions.FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectMitigationFlat: mag, conditions.EffectMitigationMagical: mag}},
		wardTestAll: {ConditionId: wardTestAll, Name: "All Ward", Family: conditions.FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectMitigationFlat: mag, conditions.EffectMitigationMagical: mag, conditions.EffectMitigationConviction: mag}},
		wardTestPotion: {ConditionId: wardTestPotion, Name: "Mind Potion", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10,
			StatMods: statmods.StatMods{"magical_mitigation": 15}},
	})
	t.Cleanup(restore)
}

type mitigations struct{ physical, magical, conviction float64 }

func mitigationOf(c *Character) mitigations {
	return mitigations{c.GetPhysicalMitigation(), c.GetMagicalMitigation(), c.GetConvictionMitigation()}
}

// Each ward adds its magnitude, in points, to exactly the channels it lists
// and to no other (spec section 4; damage types physical, mental and social
// meet these three channels, combat.MitigationChannelFor).
func TestWardEffects_FeedOnlyTheirOwnChannels(t *testing.T) {
	seedWardMitigationSpecs(t)
	for _, tc := range []struct {
		id   int
		want mitigations
	}{
		{wardTestPhysical, mitigations{0.20, 0, 0}},
		{wardTestSpell, mitigations{0.20, 0.20, 0}},
		{wardTestAll, mitigations{0.20, 0.20, 0.20}},
	} {
		c := pinCharacter()
		require.NoError(t, c.AddConditionMagnitude(tc.id, 10, 20, "test"))
		got := mitigationOf(c)
		assert.InDelta(t, tc.want.physical, got.physical, 1e-9, "physical, ward %d", tc.id)
		assert.InDelta(t, tc.want.magical, got.magical, 1e-9, "magical, ward %d", tc.id)
		assert.InDelta(t, tc.want.conviction, got.conviction, 1e-9, "conviction, ward %d", tc.id)
	}
}

// R4: a potion's mitigation still sums on top of a ward of the same kind.
func TestWardEffects_APotionStillSumsWithAWard(t *testing.T) {
	seedWardMitigationSpecs(t)
	c := pinCharacter()
	require.NoError(t, c.AddConditionMagnitude(wardTestSpell, 10, 20, "test"))
	require.NoError(t, c.AddCondition(wardTestPotion, false))
	assert.InDelta(t, 0.35, c.GetMagicalMitigation(), 1e-9)
}
```

- [ ] **Step 2: Run it to see it fail.**

Run: `go test ./internal/characters/ -run WardEffects -count=1`
Expected: FAIL, build failed: `undefined: conditions.EffectMitigationMagical`, `undefined: conditions.EffectMitigationConviction`.

- [ ] **Step 3: Add the kinds and read them.**

Replace in `internal/conditions/effects.go` (1 of 2):

```go
	EffectMitigationFlat EffectKind = "mitigation_flat" // flat physical mitigation points (wards)
	EffectPoolMaxPct     EffectKind = "pool_max_pct"    // fraction taken off a pool maximum; the pool rides on Condition.Source
	EffectAttacksCap     EffectKind = "attacks_cap"     // upper bound on swings per round
	// EffectNightVisionStrength is how far DOWN the scale an observer's usable
```

with:

```go
	EffectMitigationFlat EffectKind = "mitigation_flat" // flat physical mitigation points (wards)
	// EffectMitigationMagical and EffectMitigationConviction are a ward's
	// flat points against spell (mental) and social damage, the siblings of
	// mitigation_flat (messaging M6 slice 1, section 4). They sum with the
	// magical_mitigation and conviction_mitigation statmods, so a potion
	// still adds on top of a ward.
	EffectMitigationMagical    EffectKind = "mitigation_magical"
	EffectMitigationConviction EffectKind = "mitigation_conviction"
	EffectPoolMaxPct           EffectKind = "pool_max_pct" // fraction taken off a pool maximum; the pool rides on Condition.Source
	EffectAttacksCap           EffectKind = "attacks_cap"  // upper bound on swings per round
	// EffectNightVisionStrength is how far DOWN the scale an observer's usable
```

Replace in `internal/conditions/effects.go` (2 of 2):

```go
	EffectDamageMult, EffectDefenseMult, EffectDodgeMult, EffectRegenMult,
	EffectMitigationFlat, EffectPoolMaxPct, EffectAttacksCap,
	EffectNightVisionStrength, EffectInfraReach, EffectLightStrength,
```

with:

```go
	EffectDamageMult, EffectDefenseMult, EffectDodgeMult, EffectRegenMult,
	EffectMitigationFlat, EffectMitigationMagical, EffectMitigationConviction,
	EffectPoolMaxPct, EffectAttacksCap,
	EffectNightVisionStrength, EffectInfraReach, EffectLightStrength,
```

Replace in `internal/characters/combat.go` (1 of 2):

```go
	nonGearMit += c.StatMod("magical_mitigation")

```

with:

```go
	nonGearMit += c.StatMod("magical_mitigation")
	// A ward that blocks spell damage (messaging M6 slice 1, section 4).
	nonGearMit += int(c.Conditions.Effect(conditions.EffectMitigationMagical))

```

Replace in `internal/characters/combat.go` (2 of 2):

```go
	nonGearMit += c.StatMod("conviction_mitigation")

```

with:

```go
	nonGearMit += c.StatMod("conviction_mitigation")
	// A ward that blocks social damage (messaging M6 slice 1, section 4).
	nonGearMit += int(c.Conditions.Effect(conditions.EffectMitigationConviction))

```

- [ ] **Step 4: Run the packages and the root guards.**

Run: `go test ./internal/characters/ ./internal/conditions/ -count=1`
Expected: PASS. `ok` for both.

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 5: Commit.**

```bash
git add internal/characters/ward_mitigation_test.go \
  internal/conditions/effects.go \
  internal/characters/combat.go
git commit -F - <<'EOF'
feat(conditions): mitigation_magical and mitigation_conviction ward effects

A ward can now block spell (mental) and social damage as well as
physical: two effect kinds summed into magical and conviction mitigation
beside the statmods, so a potion still adds on top.

Refs #338.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: The caster's doors

`events.Condition` gains `Caster` and `CasterCrit`; `UserRecord.QueueCondition` and `Mob.QueueCondition` queue a producer-filled event (the four old doors become calls to it); `Character.AddConditionMagnitudeBy` is the synchronous door with a caster (D1).

- [ ] **Step 1: Write the failing tests.**

Create `internal/users/queue_condition_test.go`:

```go
package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// QueueCondition is the one door that carries a caster to the apply hook
// (messaging M6 slice 1, section 1): the producer fills the event, the door
// stamps the holder and the life epoch, whatever the producer wrote there.
func TestUserRecord_QueueConditionStampsTheHolderAndKeepsTheCaster(t *testing.T) {
	const userId = 9932
	events.DrainQueuedConditionsForTest(userId)

	u := &UserRecord{UserId: userId, Character: &characters.Character{LifeEpoch: 4}}
	u.QueueCondition(events.Condition{
		UserId: 1, MobInstanceId: 77, LifeEpoch: 99, // overwritten by the door
		ConditionId: 38, Source: "spell", Magnitude: 12, Triggers: 5,
		Caster: state.ActorRef{UserId: 3}, CasterCrit: true,
	})

	queued := events.DrainQueuedConditionsForTest(userId)
	require.Len(t, queued, 1)
	got := queued[0]
	assert.Equal(t, userId, got.UserId)
	assert.Zero(t, got.MobInstanceId)
	assert.Equal(t, uint64(4), got.LifeEpoch)
	assert.Equal(t, state.ActorRef{UserId: 3}, got.Caster)
	assert.True(t, got.CasterCrit)
	assert.Equal(t, 12.0, got.Magnitude)
	assert.Equal(t, 5, got.Triggers)
	assert.Equal(t, "spell", got.Source)
}

// The older doors still queue exactly what they did, with no caster.
func TestUserRecord_OldDoorsQueueNoCaster(t *testing.T) {
	const userId = 9933
	events.DrainQueuedConditionsForTest(userId)
	u := &UserRecord{UserId: userId}
	u.AddCondition(5, "potion")
	u.AddConditionMagnitude(119, 4, 9, "spell")
	u.AddConditionTickScaled(32, 1.5, "spell")

	queued := events.DrainQueuedConditionsForTest(userId)
	require.Len(t, queued, 3)
	for _, e := range queued {
		assert.True(t, e.Caster.IsZero(), "condition %d", e.ConditionId)
	}
	assert.Equal(t, 1.5, queued[2].TickScale)
	assert.Equal(t, 9.0, queued[1].Magnitude)
}
```

Create `internal/mobs/mob_queue_condition_test.go`:

```go
package mobs

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mob twin of UserRecord.QueueCondition: the door stamps the mob as the
// holder and its life epoch, and the caster rides through.
func TestMobQueueConditionStampsTheHolderAndKeepsTheCaster(t *testing.T) {
	m := &Mob{InstanceId: 88103}
	m.Character.LifeEpoch = 2
	events.DrainQueuedMobConditionsForTest(88103)

	m.QueueCondition(events.Condition{
		UserId: 5, LifeEpoch: 40, // overwritten by the door
		ConditionId: 2, Source: "spell", Caster: state.ActorRef{UserId: 6},
	})

	got := events.DrainQueuedMobConditionsForTest(88103)
	require.Len(t, got, 1)
	assert.Zero(t, got[0].UserId)
	assert.Equal(t, 88103, got[0].MobInstanceId)
	assert.Equal(t, uint64(2), got[0].LifeEpoch)
	assert.Equal(t, state.ActorRef{UserId: 6}, got[0].Caster)
}
```

Create `internal/characters/condition_caster_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The synchronous magnitude door carries a caster too: the spell dot and the
// combat bleeds land through it, never through the event (spec section 1).
func TestAddConditionMagnitudeBy_StampsTheCasterAndTheSource(t *testing.T) {
	defer conditions.SeedConditionRecordsForTest()()
	c := pinCharacter()
	caster := state.ActorRef{UserId: 12}

	require.NoError(t, c.AddConditionMagnitudeBy(conditions.ConditionIdPoisoned, 5, -3, "spell", caster))
	got := c.GetConditions(conditions.ConditionIdPoisoned)
	require.Len(t, got, 1)
	assert.Equal(t, caster, got[0].Caster)
	assert.Equal(t, "spell", got[0].Source)

	// The old door is the same door with no caster: the newest application
	// owns the record, so a casterless re-application leaves none.
	require.NoError(t, c.AddConditionMagnitude(conditions.ConditionIdPoisoned, 5, -3, "potion"))
	assert.True(t, c.GetConditions(conditions.ConditionIdPoisoned)[0].Caster.IsZero())
}

// A stacking record (a bleed) is one record, so its caster is its newest
// applier's.
func TestAddConditionMagnitudeBy_AStackTakesTheNewestCaster(t *testing.T) {
	defer conditions.SeedConditionRecordsForTest()()
	c := pinCharacter()
	require.NoError(t, c.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, 3, -2, "rake", state.ActorRef{MobInstanceId: 4}))
	require.NoError(t, c.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, 3, -2, "maul", state.ActorRef{UserId: 9}))
	got := c.GetConditions(conditions.ConditionIdBleeding)
	require.Len(t, got, 1)
	assert.Len(t, got[0].Stacks, 2)
	assert.Equal(t, state.ActorRef{UserId: 9}, got[0].Caster)
}
```

- [ ] **Step 2: Run them to see them fail.**

Run: `go vet ./internal/users/ ./internal/mobs/ ./internal/characters/`
Expected: FAIL: `c.AddConditionMagnitudeBy undefined`, `m.QueueCondition undefined`, `u.QueueCondition undefined`.

- [ ] **Step 3: Add the event fields and the doors.**

Replace in `internal/events/eventtypes.go` (1 of 2):

```go
	"github.com/GoMudEngine/GoMud/internal/skills"
)
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
)
```

Replace in `internal/events/eventtypes.go` (2 of 2):

```go
	LifeEpoch uint64
}
```

with:

```go
	LifeEpoch uint64
	// Caster is who cast the condition (messaging M6 slice 1). ApplyConditions
	// stamps it on the record, a damage-over-time tick credits it with the
	// harm, and the start lines name it. Zero for a potion, hazard, mutation
	// or anything else nobody cast.
	Caster state.ActorRef
	// CasterCrit puts the critical marker on the caster's start line, which
	// replaces the spell's own "takes effect" line (owner ruling R11).
	CasterCrit bool
}
```

Replace in `internal/users/userrecord.go` (1 of 4):

```go

func (u *UserRecord) AddCondition(conditionId int, source string) {

	events.AddToQueue(events.Condition{
		UserId:      u.UserId,
		ConditionId: conditionId,
		Source:      source,
		LifeEpoch:   u.lifeEpoch(),
	})

}
```

with:

```go

// QueueCondition queues a condition the producer has filled in, stamping this
// user as the holder and the current life epoch. It is the door a caster
// rides through (messaging M6 slice 1): a spell sets Caster and CasterCrit,
// and every other field means what it means on events.Condition. The other
// add doors here are this door with their own fields set.
func (u *UserRecord) QueueCondition(evt events.Condition) {
	evt.UserId = u.UserId
	evt.MobInstanceId = 0
	evt.LifeEpoch = u.lifeEpoch()
	events.AddToQueue(evt)
}

func (u *UserRecord) AddCondition(conditionId int, source string) {
	u.QueueCondition(events.Condition{ConditionId: conditionId, Source: source})
}
```

Replace in `internal/users/userrecord.go` (2 of 4):

```go

	events.AddToQueue(events.Condition{
		UserId:       u.UserId,
		ConditionId:  conditionId,
		Source:       source,
		DurationMult: durationMult,
		LifeEpoch:    u.lifeEpoch(),
	})

}
```

with:

```go

	u.QueueCondition(events.Condition{ConditionId: conditionId, Source: source, DurationMult: durationMult})
}
```

Replace in `internal/users/userrecord.go` (3 of 4):

```go
func (u *UserRecord) AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) {
	events.AddToQueue(events.Condition{
		UserId:      u.UserId,
		ConditionId: conditionId,
		Source:      source,
		Triggers:    triggers,
		Magnitude:   magnitude,
		LifeEpoch:   u.lifeEpoch(),
	})
}
```

with:

```go
func (u *UserRecord) AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) {
	u.QueueCondition(events.Condition{ConditionId: conditionId, Source: source, Triggers: triggers, Magnitude: magnitude})
}
```

Replace in `internal/users/userrecord.go` (4 of 4):

```go
func (u *UserRecord) AddConditionTickScaled(conditionId int, scale float64, source string) {
	events.AddToQueue(events.Condition{
		UserId:      u.UserId,
		ConditionId: conditionId,
		Source:      source,
		TickScale:   scale,
		LifeEpoch:   u.lifeEpoch(),
	})
}
```

with:

```go
func (u *UserRecord) AddConditionTickScaled(conditionId int, scale float64, source string) {
	u.QueueCondition(events.Condition{ConditionId: conditionId, Source: source, TickScale: scale})
}
```

Replace in `internal/mobs/mobs.go` (1 of 2):

```go

func (m *Mob) AddCondition(conditionId int, source string) {

	events.AddToQueue(events.Condition{
		MobInstanceId: m.InstanceId,
		ConditionId:   conditionId,
		Source:        source,
		LifeEpoch:     m.Character.LifeEpoch,
	})

}
```

with:

```go

// QueueCondition queues a condition the producer has filled in, stamping this
// mob as the holder and its current life epoch: the mob twin of
// UserRecord.QueueCondition, and the door a caster rides through (messaging
// M6 slice 1). The other add doors here are this door with their own fields
// set.
func (m *Mob) QueueCondition(evt events.Condition) {
	evt.UserId = 0
	evt.MobInstanceId = m.InstanceId
	evt.LifeEpoch = m.Character.LifeEpoch
	events.AddToQueue(evt)
}

func (m *Mob) AddCondition(conditionId int, source string) {
	m.QueueCondition(events.Condition{ConditionId: conditionId, Source: source})
}
```

Replace in `internal/mobs/mobs.go` (2 of 2):

```go
	}
	events.AddToQueue(events.Condition{
		MobInstanceId: m.InstanceId,
		ConditionId:   conditionId,
		Source:        source,
		DurationMult:  durationMult,
		LifeEpoch:     m.Character.LifeEpoch,
	})
}

// AddConditionMagnitude queues a record with an exact trigger count and a
// per-instance magnitude through the event path, the mob twin of
// UserRecord.AddConditionMagnitude.
func (m *Mob) AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) {
	events.AddToQueue(events.Condition{
		MobInstanceId: m.InstanceId,
		ConditionId:   conditionId,
		Source:        source,
		Triggers:      triggers,
		Magnitude:     magnitude,
		LifeEpoch:     m.Character.LifeEpoch,
	})
}

// AddConditionTickScaled queues a tick_pool condition whose per-round amount
// is scaled by scale, the mob twin of UserRecord.AddConditionTickScaled.
func (m *Mob) AddConditionTickScaled(conditionId int, scale float64, source string) {
	events.AddToQueue(events.Condition{
		MobInstanceId: m.InstanceId,
		ConditionId:   conditionId,
		Source:        source,
		TickScale:     scale,
		LifeEpoch:     m.Character.LifeEpoch,
	})
}
```

with:

```go
	}
	m.QueueCondition(events.Condition{ConditionId: conditionId, Source: source, DurationMult: durationMult})
}

// AddConditionMagnitude queues a record with an exact trigger count and a
// per-instance magnitude through the event path, the mob twin of
// UserRecord.AddConditionMagnitude.
func (m *Mob) AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) {
	m.QueueCondition(events.Condition{ConditionId: conditionId, Source: source, Triggers: triggers, Magnitude: magnitude})
}

// AddConditionTickScaled queues a tick_pool condition whose per-round amount
// is scaled by scale, the mob twin of UserRecord.AddConditionTickScaled.
func (m *Mob) AddConditionTickScaled(conditionId int, scale float64, source string) {
	m.QueueCondition(events.Condition{ConditionId: conditionId, Source: source, TickScale: scale})
}
```

Replace in `internal/characters/conditions.go` (1 of 1):

```go
// new stack's rounds. source overwrites the held record's Source on every
// call, so a stacking record carries its last applier's source.
func (c *Character) AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) error {
	conditionId = int(math.Abs(float64(conditionId)))
	if !c.Conditions.AddConditionMagnitude(conditionId, triggers, magnitude) {
		return fmt.Errorf(`failed to add condition. target: "%s" conditionId: %d`, c.Name, conditionId)
	}
	for _, b := range c.Conditions.GetConditions(conditionId) {
		b.Source = source
	}
	refused := c.hideForStealthRecord(conditionId)
```

with:

```go
// new stack's rounds. source overwrites the held record's Source on every
// call, so a stacking record carries its last applier's source. It is
// AddConditionMagnitudeBy with no caster.
func (c *Character) AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string) error {
	return c.AddConditionMagnitudeBy(conditionId, triggers, magnitude, source, state.ActorRef{})
}

// AddConditionMagnitudeBy is AddConditionMagnitude with the record's caster
// (messaging M6 slice 1): the spell dot and the combat bleeds name who
// opened them, so a tick that kills credits that caster. Source and caster
// are stamped on every call (Conditions.Stamp), so a stacking record carries
// its newest applier's.
func (c *Character) AddConditionMagnitudeBy(conditionId int, triggers int, magnitude float64, source string, caster state.ActorRef) error {
	conditionId = int(math.Abs(float64(conditionId)))
	if !c.Conditions.AddConditionMagnitude(conditionId, triggers, magnitude) {
		return fmt.Errorf(`failed to add condition. target: "%s" conditionId: %d`, c.Name, conditionId)
	}
	c.Conditions.Stamp(conditionId, source, caster)
	refused := c.hideForStealthRecord(conditionId)
```

- [ ] **Step 4: Run the packages and the root guards.**

Run: `go test ./internal/users/ ./internal/mobs/ ./internal/characters/ ./internal/events/ -count=1`
Expected: PASS. `ok` for all four.

Run: `go build ./...`
Expected: PASS. no output.

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 5: Commit.**

```bash
git add internal/users/queue_condition_test.go \
  internal/mobs/mob_queue_condition_test.go \
  internal/characters/condition_caster_test.go \
  internal/events/eventtypes.go \
  internal/users/userrecord.go \
  internal/mobs/mobs.go \
  internal/characters/conditions.go
git commit -F - <<'EOF'
feat(conditions): the caster rides the condition event and the magnitude door

events.Condition carries Caster and CasterCrit. UserRecord.QueueCondition
and Mob.QueueCondition queue a condition the producer filled in, stamping
the holder and life epoch; the four older doors now call them. The
synchronous door gains AddConditionMagnitudeBy, which stamps the caster;
AddConditionMagnitude is it with none.

Refs #240, Refs #370.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: One start line per audience, and the replacement line (#370 row 1, R6, R11)

`ApplyConditions` stamps source and caster on every landing, tells the family replacement line, and sends the start lines through `narrateConditionStart`: `start_actor` to a caster who is someone else, `start_actee` to the holder, `start_observer` to the room, which excludes both. Names are rendered and hidden per reader (D7); the crit marker rides the caster's line.

- [ ] **Step 1: Write the failing test.**

Create `internal/hooks/condition_cast_lines_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Condition ids clear of the fixture (100, 101), the narration conditions
// (7001-7013) and the notice conditions (7101-7105).
const (
	castWardAId   = 7201 // ward family, all three start lines
	castWardBId   = 7202 // ward family, all three start lines
	castCallerId  = 7203 // names the caster in the holder and room lines
	castNoActorId = 7204 // start_actee and start_observer only
	castHeatId    = 7205 // infravision, so a reader in the dark makes out shapes
	castSightId   = 7206 // see-hidden, so a reader perceives a hidden mob
)

func seedCastLineConditions() func() {
	return conditions.SeedConditionsForTest(castLineSpecs())
}

func castLineSpecs() map[int]*conditions.ConditionSpec {
	return map[int]*conditions.ConditionSpec{
		castWardAId: {ConditionId: castWardAId, Name: "Test Ward", Family: conditions.FamilyWard, RoundInterval: 1, TriggerCount: 10,
			StartActorText: "A ward settles over {actee}.", StartUserText: "A ward settles over you.", StartRoomText: "A ward settles over {actee_plain}.",
			EndUserText: "Your Test Ward fades.", EndRoomText: "The ward around {actee_plain} fades."},
		castWardBId: {ConditionId: castWardBId, Name: "Test Bulwark", Family: conditions.FamilyWard, RoundInterval: 1, TriggerCount: 10,
			StartActorText: "A bulwark rises around {actee}.", StartUserText: "A bulwark rises around you.", StartRoomText: "A bulwark rises around {actee_plain}.",
			EndUserText: "Your Test Bulwark fades.", EndRoomText: "The bulwark around {actee_plain} fades."},
		castCallerId: {ConditionId: castCallerId, Name: "Test Mark", RoundInterval: 1, TriggerCount: 10,
			StartActorText: "You mark {actee}.", StartUserText: "{actor} marks you.", StartRoomText: "{actor_plain} marks {actee_plain}.",
			EndUserText: "The mark fades."},
		castNoActorId: {ConditionId: castNoActorId, Name: "Test Quietcast", RoundInterval: 1, TriggerCount: 10,
			StartUserText: "You feel quiet.", StartRoomText: "{actee_plain} looks quiet.", EndUserText: "The quiet fades."},
		castHeatId: {ConditionId: castHeatId, Name: "Test Cast Heat Eyes", Secret: true,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
		castSightId: {ConditionId: castSightId, Name: "Test Cast True Sight", Secret: true,
			Flags: []conditions.Flag{conditions.SeeHidden}},
	}
}

// seedCastObserver adds user 3, Orin, to room 1 as a third party who is
// neither caster nor holder.
func seedCastObserver(t *testing.T) {
	t.Helper()
	observer := users.NewTestUser(3, "observer", "Orin", 1003)
	observer.Character.RoomId = 1
	restore := users.SeedUsersForTest(map[int]*users.UserRecord{
		1: users.GetByUserId(1), 2: users.GetByUserId(2), 3: observer,
	})
	t.Cleanup(restore)
	rooms.LoadRoom(1).AddPlayer(3)
}

func drainCastParties() {
	drainPlain(1)
	drainPlain(2)
	drainPlain(3)
}

// Cast on another: the caster reads start_actor, the holder start_actee and
// the room start_observer, each exactly once (spec section 2's table).
func TestConditionCast_OnAnotherTellsEachAudienceOneLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90) // pin fully lit
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardAId, Source: "spell", Caster: state.ActorRef{UserId: 1}})

	caster, holder, room := drainPlain(1), drainPlain(2), drainPlain(3)
	assert.Equal(t, []string{"A ward settles over Bobrick."}, caster)
	assert.Equal(t, []string{"A ward settles over you."}, holder)
	assert.Equal(t, []string{"A ward settles over Bobrick."}, room)

	got := users.GetByUserId(2).Character.GetConditions(castWardAId)
	require.Len(t, got, 1)
	assert.Equal(t, state.ActorRef{UserId: 1}, got[0].Caster, "the record remembers its caster")
	assert.Equal(t, "spell", got[0].Source, "every door stamps the source")
}

// Self-cast: the holder line is the caster's line, so start_actor is never
// told, and the room reads start_observer.
func TestConditionCast_SelfCastTellsTheHolderAndTheRoom(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 1, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 1}})

	assert.Equal(t, []string{"A ward settles over you."}, drainPlain(1))
	assert.Equal(t, []string{"A ward settles over Aliceia."}, drainPlain(3))
}

// The crit marker the spell trio carried moves to the caster's line, and to
// the holder's line on a self-cast.
func TestConditionCast_CritMarkerRidesTheCasterLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 1}, CasterCrit: true})
	assert.Equal(t, []string{"A ward settles over Bobrick. [CRIT!]"}, drainPlain(1))
	assert.Equal(t, []string{"A ward settles over you."}, drainPlain(2))
	assert.Equal(t, []string{"A ward settles over Bobrick."}, drainPlain(3))

	ApplyConditions(events.Condition{UserId: 1, ConditionId: castWardBId, Caster: state.ActorRef{UserId: 1}, CasterCrit: true})
	assert.Equal(t, []string{"A bulwark rises around you. [CRIT!]"}, drainPlain(1))
}

// {actor} is filled from the caster and hidden from a reader who makes out
// shapes only; the caster's own line hides the holder the same way.
func TestConditionCast_NamesAreHiddenFromAReaderAtShapes(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	darken(t, 1)
	for _, uid := range []int{1, 2, 3} {
		require.True(t, users.GetByUserId(uid).Character.Conditions.AddCondition(castHeatId, true))
	}
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castCallerId, Caster: state.ActorRef{UserId: 1}})

	assert.Equal(t, []string{"You mark a figure."}, drainPlain(1))
	assert.Equal(t, []string{"A figure marks you."}, drainPlain(2))
	assert.Equal(t, []string{"A figure marks a figure."}, drainPlain(3))
}

// In the light the holder and the room read the caster's name.
func TestConditionCast_ActorTokenNamesTheCasterInTheLight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castCallerId, Caster: state.ActorRef{UserId: 1}})

	assert.Equal(t, []string{"You mark Bobrick."}, drainPlain(1))
	assert.Equal(t, []string{"Aliceia marks you."}, drainPlain(2))
	assert.Equal(t, []string{"Aliceia marks Bobrick."}, drainPlain(3))
}

// A caster who is a mob has no client: the holder and the room still read
// their lines.
func TestConditionCast_AMobCasterNamesTheMob(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castCallerId, Caster: state.ActorRef{MobInstanceId: 100}})

	assert.Equal(t, []string{"Skeleton marks you."}, drainPlain(2))
	assert.Equal(t, []string{"Skeleton marks Bobrick."}, drainPlain(3))
	assert.Equal(t, []string{"Skeleton marks Bobrick."}, drainPlain(1), "a bystander who is neither party reads only the room line")
}

// R6: a second ward replaces the first, and the holder and the room read the
// replacement line in place of the old ward's end line.
func TestConditionCast_ANewWardReplacesTheOldWithTheReplacementLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 2}})
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardBId, Caster: state.ActorRef{UserId: 2}})
	PruneConditions(events.NewTurn{TurnNumber: 1})

	holder := users.GetByUserId(2).Character
	assert.False(t, holder.HasCondition(castWardAId))
	assert.True(t, holder.HasCondition(castWardBId))
	assert.Equal(t, []string{"Your Test Ward fades as Test Bulwark takes hold.", "A bulwark rises around you."}, drainPlain(2))
	assert.Equal(t, []string{"Bobrick's Test Ward fades as Test Bulwark takes hold.", "A bulwark rises around Bobrick."}, drainPlain(3))
}

// Re-landing the same ward is a refresh: no start lines and no replacement
// line, as before (wasAlreadyActive).
func TestConditionCast_RecastOfTheSameWardIsASilentRefresh(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 1}})
	drainCastParties()
	ApplyConditions(events.Condition{UserId: 2, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 3}})

	assert.Empty(t, drainPlain(1))
	assert.Empty(t, drainPlain(2))
	assert.Empty(t, drainPlain(3))
	assert.Equal(t, state.ActorRef{UserId: 3}, users.GetByUserId(2).Character.GetConditions(castWardAId)[0].Caster,
		"the newest application owns the record")
}

// A condition with no start_actor tells the caster nothing itself; the spell
// keeps its own line for that case (NarratesCastStart), so this hook must not
// invent one.
func TestConditionCast_NoCasterLineAuthoredTellsTheCasterNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer seedCastLineConditions()()
	seedCastObserver(t)
	rooms.LoadRoom(1).Lamp = rooms.LampPtr(90)
	drainCastParties()

	ApplyConditions(events.Condition{UserId: 2, ConditionId: castNoActorId, Caster: state.ActorRef{UserId: 1}})
	assert.Empty(t, drainPlain(1))
	assert.Equal(t, []string{"You feel quiet."}, drainPlain(2))
}

// A hidden mob holder: a caster who perceives it reads its name in the
// caster line, as a spell's own lines read (mobDisplayName's viewer rule,
// #382); a reader who does not perceive it reads no room line at all (#458).
func TestConditionCast_ACasterWhoPerceivesAHiddenHolderReadsItsName(t *testing.T) {
	m := litRoomOneWithHiddenSkeleton(t)
	t.Cleanup(seedCastLineConditions())
	caster := users.GetByUserId(1)
	require.True(t, caster.Character.Conditions.AddCondition(castSightId, true))
	require.True(t, caster.Character.Perceives(&m.Character))
	drainPlain(1)
	drainPlain(2)

	ApplyConditions(events.Condition{MobInstanceId: 100, ConditionId: castWardAId, Caster: state.ActorRef{UserId: 1}})

	assert.Equal(t, []string{"A ward settles over Skeleton."}, drainPlain(1))
	assert.Empty(t, drainPlain(2), "a reader who does not perceive the holder reads nothing")
}
```

- [ ] **Step 2: Run it to see it fail.**

Run: `go test ./internal/hooks/ -run ConditionCast -count=1`
Expected: FAIL: the caster reads the room line, not a caster line (`OnAnother`, `NoCasterLine`, `AMobCaster`); `{actor}` renders empty ("marks you."); no replacement line; the record keeps no caster; the see-hidden caster reads no line. `TestConditionCast_SelfCastTellsTheHolderAndTheRoom` passes before and after (a pin).

- [ ] **Step 3: Add the start-line helpers and call them from the hook.**

Create `internal/hooks/condition_cast_lines.go`:

```go
package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Messaging M6 slice 1, section 2: a condition's start is one line per
// audience, written by the condition's data and naming its caster. The
// caster reads start_actor, the holder start_actee, the room start_observer.
// These helpers are ApplyConditions' half of that rule; the spell's half
// (dropping its generic trio) is applySpellConditionEffect.

// critMarker is what a spell's caster line carries on a critical cast
// (spellEffectCtx.critTag). A condition's caster line carries it too, once it
// replaces the spell's own line.
const critMarker = ` <ansi fg="yellow">[CRIT!]</ansi>`

// conditionParty is one side of a condition's start lines: its tagged and
// plain names as a reader who perceives it reads them, its room, its client
// when it is an online player, and the mob when it is one.
type conditionParty struct {
	name, plain string
	roomId      int
	user        *users.UserRecord
	mob         *mobs.Mob
}

// namesFor is the party's names as one reader reads them (viewer 0 is the
// room as a whole). A hidden mob reads "something" to a reader who does not
// perceive it and its own name to one who does (mobDisplayName's viewer
// rule, #382), as a spell's own lines read.
func (p conditionParty) namesFor(viewerUserId int) (name, plain string) {
	if p.mob == nil {
		return p.name, p.plain
	}
	r := rooms.LoadRoom(p.mob.Character.RoomId)
	if r == nil {
		return p.name, p.plain
	}
	if mobHiddenFrom(p.mob, viewerUserId) {
		return messaging.StripNameAdjectives(mobDisplayName(p.mob, r, viewerUserId)), messaging.UnseenNoun(messaging.SightNone)
	}
	return messaging.StripNameAdjectives(mobSeenName(p.mob, r, viewerUserId)), p.mob.Character.GetCharacterName(false)
}

// conditionPartyOf resolves a holder or a caster by ref. A player's names are
// GetCharacterName's; a mob's are conditionMobNames', which name a hidden mob
// too: a holder's room line reaches only the readers who perceive it (#458),
// and namesFor hides a mob from any other reader. ok is false for nobody, an
// offline player or a gone mob.
func conditionPartyOf(ref state.ActorRef) (conditionParty, bool) {
	if ref.UserId != 0 {
		u := users.GetByUserId(ref.UserId)
		if u == nil {
			return conditionParty{}, false
		}
		return conditionParty{
			name:   u.Character.GetCharacterName(true),
			plain:  u.Character.GetCharacterName(false),
			roomId: u.Character.RoomId,
			user:   u,
		}, true
	}
	if ref.MobInstanceId != 0 {
		m := mobs.GetInstance(ref.MobInstanceId)
		if m == nil {
			return conditionParty{}, false
		}
		name, plain := conditionMobNames(m)
		return conditionParty{name: name, plain: plain, roomId: m.Character.RoomId, mob: m}, true
	}
	return conditionParty{}, false
}

// conditionSelfCast reports whether the condition's caster is its holder.
func conditionSelfCast(evt events.Condition) bool {
	c := evt.Caster
	return (c.UserId != 0 && c.UserId == evt.UserId) ||
		(c.MobInstanceId != 0 && c.MobInstanceId == evt.MobInstanceId)
}

// conditionReaderSight is how well a participant sees the other party of a
// start line: the decision the pre-landing snapshot recorded for them when
// there is one (a darkness source's start, owner rule 2026-10-05), else the
// room as it is now.
func conditionReaderSight(r *rooms.Room, snap rooms.VisualSnapshot, userId int) messaging.SightDecision {
	if d, ok := snap[userId]; ok {
		return d
	}
	return r.ParticipantSight(userId)
}

// narrateConditionStart sends a landing condition's start lines: start_actor
// to a caster who is someone else and online, start_actee to a player
// holder, start_observer to the room (unless hideOnHidden). {actor} is the
// caster, "something" when nobody cast it. Each private line hides the other
// party's name from a reader who cannot make them out, as SendTrio does, and
// the room line hides both, excludes both, and skips unseenBy, the readers
// who did not perceive the holder before it landed (conditionLineUnseenBy,
// #458). The crit marker rides the caster's line, which on a self-cast is
// the holder's.
func narrateConditionStart(spec *conditions.ConditionSpec, evt events.Condition, holder conditionParty,
	snap rooms.VisualSnapshot, hideOnHidden bool, unseenBy []int) {

	selfCast := conditionSelfCast(evt)
	caster, casterKnown := conditionParty{}, false
	if !selfCast {
		caster, casterKnown = conditionPartyOf(evt.Caster)
	}
	// {actor} for a reader: the caster, the holder on a self-cast, and
	// "something" when nobody cast it.
	casterNamesFor := func(viewerUserId int) (string, string) {
		switch {
		case selfCast:
			return holder.name, holder.plain
		case casterKnown:
			return caster.namesFor(viewerUserId)
		}
		unseen := messaging.UnseenNoun(messaging.SightNone)
		return unseen, unseen
	}
	crit := ""
	if evt.CasterCrit {
		crit = critMarker
	}
	r := rooms.LoadRoom(holder.roomId)

	// The holder is the ACTEE: the condition happens to them. A mob holder
	// has no client, so it reads nothing.
	if holder.user != nil {
		casterName, casterPlain := casterNamesFor(holder.user.UserId)
		line := spec.NarrateCast(conditions.PhaseStart, holder.name, holder.plain, casterName, casterPlain).Actee
		if line != "" {
			if selfCast {
				line += crit
			}
			if casterKnown && r != nil {
				line = messaging.HideNames(line, []string{casterPlain}, conditionReaderSight(r, snap, holder.user.UserId))
			}
			holder.user.SendText(messaging.CategoryConditionApply, line)
		}
	}

	if casterKnown && caster.user != nil {
		holderName, holderPlain := holder.namesFor(caster.user.UserId)
		line := spec.NarrateCast(conditions.PhaseStart, holderName, holderPlain, caster.name, caster.plain).Actor
		if line != "" {
			line += crit
			if r != nil {
				line = messaging.HideNames(line, []string{holderPlain}, conditionReaderSight(r, snap, caster.user.UserId))
			}
			caster.user.SendText(messaging.CategoryConditionApply, line)
		}
	}

	casterName, casterPlain := casterNamesFor(0)
	roles := spec.NarrateCast(conditions.PhaseStart, holder.name, holder.plain, casterName, casterPlain)
	// Visual, not audio: start text describes what the room SEES. HidingNames,
	// because a start line may author a bare {actee_plain} or {actor_plain},
	// which tag-based Anonymize cannot see.
	if roles.Observer != "" && !hideOnHidden && r != nil {
		names := []string{holder.plain}
		exclude := append([]int{}, unseenBy...)
		if holder.user != nil {
			exclude = append(exclude, holder.user.UserId)
		}
		if casterKnown {
			names = append(names, caster.plain)
			if caster.user != nil {
				exclude = append(exclude, caster.user.UserId)
			}
		}
		sendConditionStartRoomText(r, snap, roles.Observer, names, exclude...)
	}
}

// familyRivalNames names the held records a landing condition will replace
// (conditions.Conditions.FamilyRivals), read BEFORE the add discards them.
func familyRivalNames(held *conditions.Conditions, conditionId int) []string {
	var names []string
	for _, b := range held.FamilyRivals(conditionId) {
		if spec := conditions.GetConditionSpec(b.ConditionId); spec != nil {
			names = append(names, spec.Name)
		}
	}
	return names
}

// narrateFamilyReplacement tells the holder and the room which ward or heal
// gave way to the one landing (owner ruling R6), in place of the old record's
// end line, which the discard never tells. The room line skips unseenBy, as
// every condition room line does (#458).
func narrateFamilyReplacement(spec *conditions.ConditionSpec, replaced []string, holder conditionParty, unseenBy []int) {
	r := rooms.LoadRoom(holder.roomId)
	for _, old := range replaced {
		if holder.user != nil {
			holder.user.SendText(messaging.CategoryConditionApply,
				fmt.Sprintf(`Your %s fades as %s takes hold.`, old, spec.Name))
		}
		if r == nil {
			continue
		}
		exclude := append([]int{}, unseenBy...)
		if holder.user != nil {
			exclude = append(exclude, holder.user.UserId)
		}
		r.SendTextVisualHidingNames(messaging.CategoryConditionApply,
			fmt.Sprintf(`%s's %s fades as %s takes hold.`, holder.name, old, spec.Name),
			[]string{holder.plain}, exclude...)
	}
}
```

Replace in `internal/hooks/Condition_ApplyConditions.go` (1 of 2):

```go
	startSnap := darknessStartSnapshot(conditionInfo, targetChar, wasAlreadyActive)
	var addErr error
```

with:

```go
	startSnap := darknessStartSnapshot(conditionInfo, targetChar, wasAlreadyActive)
	// A ward or a heal replaces its family's other record (owner rulings R4,
	// R6); the add discards it, so its name is read first for the
	// replacement line.
	replaced := familyRivalNames(&targetChar.Conditions, evt.ConditionId)
	var addErr error
```

Replace in `internal/hooks/Condition_ApplyConditions.go` (2 of 2):

```go

	// A tick_pool condition's per-round amount is computed here, where the
	// record now exists, on a fresh application and a refresh alike, at the
	// applier's scale (a spell's caster scale; 0, meaning 1.0, for the rest).
	setTickAmountAtApply(targetChar, conditionInfo, evt.ConditionId, evt.TickScale)

	//
	// Send the start notice (authored, or the generic line; a secret condition is
	// silent) only on first application, not on refresh.
	//
	// A mob holder has no client, so only room text can reach anyone; without
	// it there is nothing to render and the name and room lookups are skipped.
	startText := conditionInfo.Narration(conditions.PhaseStart)
	holderCanRead := evt.UserId != 0 && len(startText.Actee) > 0
	if !wasAlreadyActive && (holderCanRead || len(startText.Observer) > 0) {
		var charName, charPlainName string
		var holder *users.UserRecord
		var roomId, excludeId int

		if evt.UserId != 0 {
			if u := users.GetByUserId(evt.UserId); u != nil {
				charName = u.Character.GetCharacterName(true)
				charPlainName = u.Character.GetCharacterName(false)
				roomId = u.Character.RoomId
				excludeId = u.UserId
				holder = u
			}
		} else if evt.MobInstanceId != 0 {
			if m := mobs.GetInstance(evt.MobInstanceId); m != nil {
				charName, charPlainName = conditionMobNames(m)
				roomId = m.Character.RoomId
			}
		}

		if charName != "" {
			roles := conditionInfo.Narrate(conditions.PhaseStart, charName, charPlainName)
			// The holder is the ACTEE: the condition happens to them. A mob holder
			// has no client, so its line is rendered and dropped.
			if roles.Actee != "" && holder != nil {
				holder.SendText(messaging.CategoryConditionApply, roles.Actee)
			}
			// Visual, not audio. Start text describes what the room SEES
			// ("A warm glow surrounds Alice"), and Room.SendText is never
			// sight-gated, so it reached blind and unsighted observers. M2
			// fixed the same defect for a spell's cast_observer line.
			if roles.Observer != "" && !hideOnHidden {
				if r := rooms.LoadRoom(roomId); r != nil {
					// HidingNames, not plain SendTextVisual. Sight-gating alone
					// leaves the line leaning on tag-based Anonymize, which by
					// its own docstring only strips identity TAGS and cannot
					// see a bare name. M4d PR 1 filed that as a latent defect
					// waiting for the first bare name to be authored; condition
					// 115 authors `{actee_plain}` ("{actee_plain} is raked
					// open, blood welling from ragged claw-wounds."), so it was
					// live. Read in play 2026-09-21: "Cave Crawler is raked
					// open..." among lines that otherwise all said "A figure".
					//
					// Only to the players who perceived the holder before it
					// landed (#458): a sneak-hidden player quitting read to
					// the room as "Ordel Quist sits down and begins to
					// meditate."
					sendConditionStartRoomText(r, startSnap,
						roles.Observer, []string{charPlainName}, append(startUnseenBy, excludeId)...)
				}
			}
		}
	}
```

with:

```go

	// The record remembers who applied it and from what, on every door
	// (messaging M6 slice 1, section 1): the newest application owns it.
	targetChar.Conditions.Stamp(evt.ConditionId, evt.Source, evt.Caster)

	// A tick_pool condition's per-round amount is computed here, where the
	// record now exists, on a fresh application and a refresh alike, at the
	// applier's scale (a spell's caster scale; 0, meaning 1.0, for the rest).
	setTickAmountAtApply(targetChar, conditionInfo, evt.ConditionId, evt.TickScale)

	holder, holderKnown := conditionPartyOf(state.ActorRef{UserId: evt.UserId, MobInstanceId: evt.MobInstanceId})
	if holderKnown && len(replaced) > 0 {
		narrateFamilyReplacement(conditionInfo, replaced, holder, startUnseenBy)
	}

	//
	// Send the start lines (authored, or the generic holder line; a secret
	// condition is silent) only on first application, not on refresh. One line
	// per audience: the caster, the holder and the room (narrateConditionStart).
	//
	// A mob holder has no client, so with no caster line and no room text
	// there is nothing to render.
	startText := conditionInfo.Narration(conditions.PhaseStart)
	holderCanRead := evt.UserId != 0 && len(startText.Actee) > 0
	casterCanRead := evt.Caster.UserId != 0 && len(startText.Actor) > 0
	if !wasAlreadyActive && holderKnown && (holderCanRead || casterCanRead || len(startText.Observer) > 0) {
		// The room line is visual and hides names (sendConditionStartRoomText):
		// condition 115 authors a bare {actee_plain}, which tag-based Anonymize
		// cannot see; read in play 2026-09-21 as "Cave Crawler is raked
		// open..." among lines that otherwise all said "A figure". It goes
		// only to the players who perceived the holder before it landed
		// (startUnseenBy, #458).
		narrateConditionStart(conditionInfo, evt, holder, startSnap, hideOnHidden, startUnseenBy)
	}
```

Replace in `internal/hooks/spell_effects.go` (1 of 1):

```go
	if c.out.AttackerCrit {
		return ` <ansi fg="yellow">[CRIT!]</ansi>`
	}
```

with:

```go
	if c.out.AttackerCrit {
		return critMarker
	}
```

- [ ] **Step 4: Re-key the apply-path guard (the rival read moved the three adds down four lines).**

Replace in `condition_apply_path_guard_test.go` (1 of 1):

```go
var conditionApplyPathAllowlist = map[string]string{
	// ── The sanctioned consumer of the event ────────────────────────────────
	"internal/hooks/Condition_ApplyConditions.go|118": "this IS the hook the event feeds; it is where every routed condition is finally applied",
	"internal/hooks/Condition_ApplyConditions.go|120": "this IS the hook the event feeds; it is where every routed condition is finally applied",
	"internal/hooks/Condition_ApplyConditions.go|122": "this IS the hook the event feeds; it is where every routed condition is finally applied",

```

with:

```go
var conditionApplyPathAllowlist = map[string]string{
	// ── The sanctioned consumer of the event (re-keyed messaging M6 slice 1,
	// when the family rival read landed above the adds) ──────────────────
	"internal/hooks/Condition_ApplyConditions.go|122": "this IS the hook the event feeds; it is where every routed condition is finally applied",
	"internal/hooks/Condition_ApplyConditions.go|124": "this IS the hook the event feeds; it is where every routed condition is finally applied",
	"internal/hooks/Condition_ApplyConditions.go|126": "this IS the hook the event feeds; it is where every routed condition is finally applied",

```

- [ ] **Step 5: Run hooks and the root guards.**

Run: `go test ./internal/hooks/ -count=1`
Expected: PASS. `ok`.

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 6: Commit.**

```bash
git add internal/hooks/condition_cast_lines_test.go \
  internal/hooks/condition_cast_lines.go \
  internal/hooks/Condition_ApplyConditions.go \
  internal/hooks/spell_effects.go \
  condition_apply_path_guard_test.go
git commit -F - <<'EOF'
feat(hooks): one condition start line per audience, naming the caster (#370)

ApplyConditions stamps every landed record with its source and caster,
tells the holder and the room which ward or heal gave way, and sends the
start lines through narrateConditionStart: start_actor to a caster who is
someone else, start_actee to the holder, start_observer to the room,
which now excludes the caster. Each private line renders and hides the
other party for that reader; the crit marker rides the caster's line.

Refs #370.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: A damage-over-time kill credits its caster (#240, R5, R10)

The round ticks harm in `condition.Caster`'s name; a mob's health tick credits the caster in its damage map through `creditMobHarm` (`creditSpellDamage`'s rule by ref, D4). The spell dot and every combat bleed land through `AddConditionMagnitudeBy` with their caster (D2). The backstops stay anonymous (D3).

- [ ] **Step 1: Write the failing tests.**

Create `internal/hooks/condition_tick_credit_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #240: a damage-over-time tick harms in its caster's name, so a tick that
// kills queues the death with the caster as killer, and a tick on a mob
// credits the caster in the damage map every mob-death consumer reads
// (Death_MobKillCredit, MobDeath_FactionRep, MobDeath_QuestNotify, the
// bounty claim, item procs).

// poisonMobWithCaster gives the fixture's Skeleton (mob 100, 50 health) the
// spell dot at amount per round, cast by caster.
func poisonMobWithCaster(t *testing.T, amount int, caster state.ActorRef) *mobs.Mob {
	t.Helper()
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	require.NoError(t, mob.Character.AddConditionMagnitudeBy(conditions.ConditionIdPoisoned, 3, -float64(amount), "spell", caster))
	events.DrainQueuedCharacterDiedForTest()
	return mob
}

func TestDotTick_OnAMobCreditsThePlayerCaster(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	mob := poisonMobWithCaster(t, 7, state.ActorRef{UserId: 1})

	tickMobConditions(mob, 100)

	assert.Equal(t, map[int]int{1: 7}, mob.Character.PlayerDamage, "the caster is credited with the tick")
}

func TestDotTick_ALethalTickOnAMobNamesTheCasterAsKiller(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	mob := poisonMobWithCaster(t, 80, state.ActorRef{UserId: 1})

	tickMobConditions(mob, 100)

	died := events.DrainQueuedCharacterDiedForTest()
	require.Len(t, died, 1)
	assert.Equal(t, 1, died[0].KillerUserId)
	assert.Equal(t, map[int]int{1: 80}, mob.Character.PlayerDamage)
}

// R10: a caster who has logged out is still credited; the damage map and the
// killer ref are keyed by user id and need no live record.
func TestDotTick_AnOfflineCasterIsStillCredited(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	require.Nil(t, users.GetByUserId(77), "user 77 must be offline for this test")
	mob := poisonMobWithCaster(t, 80, state.ActorRef{UserId: 77})

	tickMobConditions(mob, 100)

	died := events.DrainQueuedCharacterDiedForTest()
	require.Len(t, died, 1)
	assert.Equal(t, 77, died[0].KillerUserId)
	assert.Equal(t, map[int]int{77: 80}, mob.Character.PlayerDamage)
}

// A charmed mob's dot credits the player who charmed it, as its spell damage
// does (creditSpellDamage); a free mob's credits no player but is still the
// killer.
func TestDotTick_AMobCasterCreditsItsCharmerOrNobody(t *testing.T) {
	for _, charmed := range []bool{true, false} {
		cleanup := seedAllRegistries()
		restore := conditions.SeedConditionRecordsForTest()
		casterMob := &mobs.Mob{MobId: 2, InstanceId: 300, Character: characters.Character{Name: "Merchant", RoomId: 1}}
		if charmed {
			casterMob.Character.Charmed = characters.NewCharm(2, -1, "")
		}
		restoreMobs := mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{100: mobs.GetInstance(100), 300: casterMob})
		mob := poisonMobWithCaster(t, 80, state.ActorRef{MobInstanceId: 300})

		tickMobConditions(mob, 100)

		died := events.DrainQueuedCharacterDiedForTest()
		require.Len(t, died, 1)
		assert.Equal(t, 300, died[0].KillerMobInstanceId)
		if charmed {
			assert.Equal(t, map[int]int{2: 80}, mob.Character.PlayerDamage, "the charmer is credited")
		} else {
			assert.Empty(t, mob.Character.PlayerDamage, "a free mob credits no player")
		}
		restoreMobs()
		restore()
		cleanup()
	}
}

// A record with no caster (a hazard, a potion, a mob caster lost to a
// restart, which the load strips) harms anonymously, as before.
func TestDotTick_NoCasterHarmsAnonymously(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	mob := poisonMobWithCaster(t, 80, state.ActorRef{})

	tickMobConditions(mob, 100)

	died := events.DrainQueuedCharacterDiedForTest()
	require.Len(t, died, 1)
	assert.Zero(t, died[0].KillerUserId)
	assert.Zero(t, died[0].KillerMobInstanceId)
	assert.Empty(t, mob.Character.PlayerDamage)
}

// The player tick passes the caster too: a dot kill on a player names its
// caster, which the bounty resolver reads (PlayerDeath_BountyResolve).
func TestDotTick_ALethalTickOnAPlayerNamesTheCaster(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()
	victim := users.GetByUserId(2)
	victim.Character.HealthMax.Value = 20
	victim.Character.Health = 20
	require.NoError(t, victim.Character.AddConditionMagnitudeBy(conditions.ConditionIdPoisoned, 3, -50, "spell", state.ActorRef{UserId: 1}))
	events.DrainQueuedCharacterDiedForTest()

	UserRoundTick(events.NewRound{RoundNumber: 1})

	var killers []int
	for _, d := range events.DrainQueuedCharacterDiedForTest() {
		if d.UserId == 2 {
			killers = append(killers, d.KillerUserId)
		}
	}
	assert.Equal(t, []int{1}, killers)
}
```

Create `internal/actions/actor_ref_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
)

// idStubActor is stubActor with an identity, so a test can see which actor a
// bleed's caster names. stubActor itself answers 0 for both ids.
type idStubActor struct {
	*stubActor
	userId, mobInstanceId int
}

func (a *idStubActor) GetUserId() int        { return a.userId }
func (a *idStubActor) GetMobInstanceId() int { return a.mobInstanceId }

func TestActorRefOf(t *testing.T) {
	assert.Equal(t, state.ActorRef{UserId: 5}, ActorRefOf(&idStubActor{stubActor: newStubActor(nil, nil), userId: 5}))
	assert.Equal(t, state.ActorRef{MobInstanceId: 9}, ActorRefOf(&idStubActor{stubActor: newStubActor(nil, nil), mobInstanceId: 9}))
	assert.True(t, ActorRefOf(nil).IsZero())
}
```

Replace in `internal/actions/combat_drain_test.go` (1 of 3):

```go
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state/position"
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
```

Replace in `internal/actions/combat_drain_test.go` (2 of 3):

```go

		res = ExecuteDrain(newStubActor(char, newTestRoom()))
		if res.Executed && res.MoveResult.Hit {
```

with:

```go

		res = ExecuteDrain(&idStubActor{stubActor: newStubActor(char, newTestRoom()), mobInstanceId: 4242})
		if res.Executed && res.MoveResult.Hit {
```

Replace in `internal/actions/combat_drain_test.go` (3 of 3):

```go
		assert.Less(t, held[0].TickAmount, 0, "drain's Bleeding record must carry a negative tick snapshot")
	}
```

with:

```go
		assert.Less(t, held[0].TickAmount, 0, "drain's Bleeding record must carry a negative tick snapshot")
		assert.Equal(t, state.ActorRef{MobInstanceId: 4242}, held[0].Caster,
			"the bleed names its attacker, so a bleed kill credits them (#240)")
	}
```

Replace in `internal/actions/combat_throttle_test.go` (1 of 3):

```go
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
```

Replace in `internal/actions/combat_throttle_test.go` (2 of 3):

```go
	for i := 0; i < 100; i++ {
		char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
		char.Cooldowns = characters.Cooldowns{}

		res = ExecuteThrottle(newStubActor(char, newTestRoom()))
		if res.Executed && res.MoveResult.Hit {
			hitSeen = true
			break
		}
```

with:

```go
	for i := 0; i < 100; i++ {
		char.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)
		char.Cooldowns = characters.Cooldowns{}

		res = ExecuteThrottle(&idStubActor{stubActor: newStubActor(char, newTestRoom()), userId: 31})
		if res.Executed && res.MoveResult.Hit {
			hitSeen = true
			break
		}
```

Replace in `internal/actions/combat_throttle_test.go` (3 of 3):

```go
		assert.Less(t, held[0].TickAmount, 0, "throttle's Bleeding record must carry a negative tick snapshot")
	}
```

with:

```go
		assert.Less(t, held[0].TickAmount, 0, "throttle's Bleeding record must carry a negative tick snapshot")
		assert.Equal(t, state.ActorRef{UserId: 31}, held[0].Caster,
			"the bleed names its attacker, so a bleed kill credits them (#240)")
	}
```

Replace in `internal/behaviortree/item_proc_effects_test.go` (1 of 4):

```go
	"github.com/GoMudEngine/GoMud/internal/rooms"
)
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
)
```

Replace in `internal/behaviortree/item_proc_effects_test.go` (2 of 4):

```go
	target := characters.New()
	if !procApplyCondition(target, map[string]float64{"condition": 1, "duration": 6, "magnitude": 12}) {
		t.Fatal("apply_condition should execute")
```

with:

```go
	target := characters.New()
	owner := characters.New()
	owner.MobInstanceId = 515
	if !procApplyCondition(owner, target, map[string]float64{"condition": 1, "duration": 6, "magnitude": 12}) {
		t.Fatal("apply_condition should execute")
```

Replace in `internal/behaviortree/item_proc_effects_test.go` (3 of 4):

```go
		t.Fatalf("expected one stack of 6 rounds at -12, got %+v", held[0].Stacks)
	}
}

```

with:

```go
		t.Fatalf("expected one stack of 6 rounds at -12, got %+v", held[0].Stacks)
	}
	if held[0].Caster != (state.ActorRef{MobInstanceId: 515}) {
		t.Fatalf("the bleed must name the item's owner as its caster (#240), got %+v", held[0].Caster)
	}
}

```

Replace in `internal/behaviortree/item_proc_effects_test.go` (4 of 4):

```go
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	if procApplyCondition(characters.New(), map[string]float64{"condition": 1, "duration": 6, "magnitude": 12}) {
		t.Fatal("procApplyCondition must return false when the Bleeding spec is missing")
	}
}

func TestProcApplyCondition_NilAndUnknown(t *testing.T) {
	if procApplyCondition(nil, map[string]float64{"condition": 1}) {
		t.Fatal("nil target must not execute")
	}
	if procApplyCondition(characters.New(), map[string]float64{"condition": 99}) {
		t.Fatal("unknown condition id must not execute")
```

with:

```go
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{}))
	if procApplyCondition(characters.New(), characters.New(), map[string]float64{"condition": 1, "duration": 6, "magnitude": 12}) {
		t.Fatal("procApplyCondition must return false when the Bleeding spec is missing")
	}
}

func TestProcApplyCondition_NilAndUnknown(t *testing.T) {
	if procApplyCondition(characters.New(), nil, map[string]float64{"condition": 1}) {
		t.Fatal("nil target must not execute")
	}
	if procApplyCondition(characters.New(), characters.New(), map[string]float64{"condition": 99}) {
		t.Fatal("unknown condition id must not execute")
```

- [ ] **Step 2: Run them to see them fail.**

Run: `go test ./internal/hooks/ -run DotTick -count=1`
Expected: FAIL: `KillerUserId` 0 and an empty `PlayerDamage` where the caster is expected, in five tests. `TestDotTick_NoCasterHarmsAnonymously` passes (a pin).

Run: `go vet ./internal/actions/ ./internal/behaviortree/`
Expected: FAIL: `undefined: ActorRefOf`; `too many arguments in call to procApplyCondition`.

- [ ] **Step 3: Name the attacker on every bleed.**

Create `internal/actions/actor_ref.go`:

```go
package actions

import "github.com/GoMudEngine/GoMud/internal/state"

// ActorRefOf names an actor for harm attribution: a player by user id, a mob
// by instance id, nobody for nil. A combat move's bleed carries it as the
// record's caster, so a bleed that kills credits the attacker (#240).
func ActorRefOf(a Actor) state.ActorRef {
	if a == nil {
		return state.ActorRef{}
	}
	return state.ActorRef{UserId: a.GetUserId(), MobInstanceId: a.GetMobInstanceId()}
}
```

Replace in `internal/actions/combat_drain.go` (1 of 2):

```go
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.DrainBleedStrengthDivisor, cfg.DrainBleedMin)
		_ = target.Char.AddConditionMagnitude(conditions.ConditionIdBleeding, int(cfg.DrainBleedRounds), -float64(bleedDmg), "drain")
	}
```

with:

```go
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.DrainBleedStrengthDivisor, cfg.DrainBleedMin)
		_ = target.Char.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, int(cfg.DrainBleedRounds), -float64(bleedDmg), "drain", ActorRefOf(actor))
	}
```

Replace in `internal/actions/combat_drain.go` (2 of 2):

```go
			pr.BleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.DrainBleedStrengthDivisor, cfg.DrainBleedMin)
			_ = target.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, int(cfg.DrainBleedRounds), -float64(pr.BleedDmg), "drain")
		}
```

with:

```go
			pr.BleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.DrainBleedStrengthDivisor, cfg.DrainBleedMin)
			_ = target.Character.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, int(cfg.DrainBleedRounds), -float64(pr.BleedDmg), "drain", ActorRefOf(actor))
		}
```

Replace in `internal/actions/combat_hamstring.go` (1 of 1):

```go
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.HamstringBleedStrengthDivisor, cfg.HamstringBleedMin)
		_ = target.Char.AddConditionMagnitude(conditions.ConditionIdBleeding, int(cfg.HamstringBleedRounds), -float64(bleedDmg), "hamstring")
	}
```

with:

```go
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.HamstringBleedStrengthDivisor, cfg.HamstringBleedMin)
		_ = target.Char.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, int(cfg.HamstringBleedRounds), -float64(bleedDmg), "hamstring", ActorRefOf(actor))
	}
```

Replace in `internal/actions/combat_maul.go` (1 of 1):

```go
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.MaulBleedStrengthDivisor, cfg.MaulBleedMin)
		_ = target.Char.AddConditionMagnitude(conditions.ConditionIdBleeding, int(cfg.MaulBleedRounds), -float64(bleedDmg), "maul")
	}
```

with:

```go
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.MaulBleedStrengthDivisor, cfg.MaulBleedMin)
		_ = target.Char.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, int(cfg.MaulBleedRounds), -float64(bleedDmg), "maul", ActorRefOf(actor))
	}
```

Replace in `internal/actions/combat_rake.go` (1 of 1):

```go
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.RakeBleedStrengthDivisor, cfg.RakeBleedMin)
		_ = target.Char.AddConditionMagnitude(conditions.ConditionIdBleeding, int(cfg.RakeBleedRounds), -float64(bleedDmg), "rake")
	}
```

with:

```go
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.RakeBleedStrengthDivisor, cfg.RakeBleedMin)
		_ = target.Char.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, int(cfg.RakeBleedRounds), -float64(bleedDmg), "rake", ActorRefOf(actor))
	}
```

Replace in `internal/actions/combat_throttle.go` (1 of 1):

```go
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.ThrottleBleedStrengthDivisor, cfg.ThrottleBleedMin)
		_ = target.Char.AddConditionMagnitude(conditions.ConditionIdBleeding, int(cfg.ThrottleBleedRounds), -float64(bleedDmg), "throttle")

```

with:

```go
		bleedDmg = bleedPerRound(char.Stats.Strength.ValueAdj, cfg.ThrottleBleedStrengthDivisor, cfg.ThrottleBleedMin)
		_ = target.Char.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, int(cfg.ThrottleBleedRounds), -float64(bleedDmg), "throttle", ActorRefOf(actor))

```

Replace in `internal/behaviortree/actions_item_proc.go` (1 of 4):

```go
	case `apply_condition`:
		executed = procApplyCondition(ev.Other, p)
	}
```

with:

```go
	case `apply_condition`:
		executed = procApplyCondition(owner, ev.Other, p)
	}
```

Replace in `internal/behaviortree/actions_item_proc.go` (2 of 4):

```go

// procApplyCondition applies the Bleeding record to the target. Params:
// condition (1=bleeding; the switch is the extension point for future
// condition ids; only bleeding is wired here, YAGNI), duration (the stack's
// rounds, default 4 if unset/<1), magnitude (per-round health loss, default 2
// if unset/<1). Each proc that fires adds one stack; see the Stacking flag.
// Unknown condition ids do not execute (so the branch's cooldown isn't
// armed).
func procApplyCondition(target *characters.Character, params map[string]float64) bool {
	if target == nil {
```

with:

```go

// procApplyCondition applies owner's Bleeding record to the target. Params:
// condition (1=bleeding; the switch is the extension point for future
// condition ids; only bleeding is wired here, YAGNI), duration (the stack's
// rounds, default 4 if unset/<1), magnitude (per-round health loss, default 2
// if unset/<1). Each proc that fires adds one stack; see the Stacking flag.
// Unknown condition ids do not execute (so the branch's cooldown isn't
// armed).
func procApplyCondition(owner, target *characters.Character, params map[string]float64) bool {
	if target == nil {
```

Replace in `internal/behaviortree/actions_item_proc.go` (3 of 4):

```go
	case 1:
		return target.AddConditionMagnitude(conditions.ConditionIdBleeding, dur, -mag, "itemproc") == nil
	}
```

with:

```go
	case 1:
		return target.AddConditionMagnitudeBy(conditions.ConditionIdBleeding, dur, -mag, "itemproc", procOwnerRef(owner)) == nil
	}
```

Replace in `internal/behaviortree/actions_item_proc.go` (4 of 4):

```go
	return true
}

```

with:

```go
	return true
}

// procOwnerRef names an item proc's owner as procStealPool does, for the
// caster of a bleed the proc opens: a bleed that kills credits the owner
// (#240). Nobody for a nil owner.
func procOwnerRef(owner *characters.Character) state.ActorRef {
	if owner == nil {
		return state.ActorRef{}
	}
	return state.ActorRef{UserId: owner.GetUserId(), MobInstanceId: owner.MobInstanceId}
}

```

- [ ] **Step 4: Harm and credit in the caster's name.**

Replace in `internal/hooks/NewRound_MobRoundTick.go` (1 of 2):

```go
					//
					// DoT conditions carry no applier, so the harm source is anonymous
					// (state.ActorRef{}). See ApplyHarm's docstring.
					switch mobConditionSpec.TickPool {
					case "health":
						if tickAmt > 0 {
							mob.Character.ApplyRestore(characters.PoolHealth, tickAmt)
						} else if tickAmt < 0 {
							mob.Character.ApplyHarm(characters.PoolHealth, -tickAmt, state.ActorRef{})
							cancelCraftOrSalvageOnDamage(&mob.Character)
```

with:

```go
					//
					// The harm is the record's caster's (#240): a tick that kills
					// names them, and a health tick credits them in the damage
					// map every mob-death consumer reads. A record nobody cast
					// harms anonymously, as before.
					switch mobConditionSpec.TickPool {
					case "health":
						if tickAmt > 0 {
							mob.Character.ApplyRestore(characters.PoolHealth, tickAmt)
						} else if tickAmt < 0 {
							creditMobHarm(mob, condition.Caster, -tickAmt)
							mob.Character.ApplyHarm(characters.PoolHealth, -tickAmt, condition.Caster)
							cancelCraftOrSalvageOnDamage(&mob.Character)
```

Replace in `internal/hooks/NewRound_MobRoundTick.go` (2 of 2):

```go
						} else if tickAmt < 0 {
							mob.Character.ApplyHarm(characters.PoolStamina, -tickAmt, state.ActorRef{})
						}
					case "conviction":
						if tickAmt > 0 {
							mob.Character.ApplyRestore(characters.PoolConviction, tickAmt)
						} else if tickAmt < 0 {
							mob.Character.ApplyHarm(characters.PoolConviction, -tickAmt, state.ActorRef{})
						}
```

with:

```go
						} else if tickAmt < 0 {
							mob.Character.ApplyHarm(characters.PoolStamina, -tickAmt, condition.Caster)
						}
					case "conviction":
						if tickAmt > 0 {
							mob.Character.ApplyRestore(characters.PoolConviction, tickAmt)
						} else if tickAmt < 0 {
							mob.Character.ApplyHarm(characters.PoolConviction, -tickAmt, condition.Caster)
						}
```

Replace in `internal/hooks/NewRound_UserRoundTick.go` (1 of 2):

```go
							//
							// DoT conditions carry no applier, so the harm source is
							// anonymous (state.ActorRef{}). See ApplyHarm's docstring.
							switch trigConditionSpec.TickPool {
							case "health":
								if tickAmt > 0 {
									user.Character.ApplyRestore(characters.PoolHealth, tickAmt)
								} else if tickAmt < 0 {
									user.Character.ApplyHarm(characters.PoolHealth, -tickAmt, state.ActorRef{})
									// Damage is damage: wake a sleeper and drop
```

with:

```go
							//
							// The harm is the record's caster's (#240), so a tick
							// that kills names them; a record nobody cast harms
							// anonymously, as before. A player victim's damage map
							// is left alone, as a spell's direct damage leaves it
							// (creditSpellDamage credits mob victims only).
							switch trigConditionSpec.TickPool {
							case "health":
								if tickAmt > 0 {
									user.Character.ApplyRestore(characters.PoolHealth, tickAmt)
								} else if tickAmt < 0 {
									user.Character.ApplyHarm(characters.PoolHealth, -tickAmt, condition.Caster)
									// Damage is damage: wake a sleeper and drop
```

Replace in `internal/hooks/NewRound_UserRoundTick.go` (2 of 2):

```go
								} else if tickAmt < 0 {
									user.Character.ApplyHarm(characters.PoolStamina, -tickAmt, state.ActorRef{})
								}
							case "conviction":
								if tickAmt > 0 {
									user.Character.ApplyRestore(characters.PoolConviction, tickAmt)
								} else if tickAmt < 0 {
									user.Character.ApplyHarm(characters.PoolConviction, -tickAmt, state.ActorRef{})
								}
```

with:

```go
								} else if tickAmt < 0 {
									user.Character.ApplyHarm(characters.PoolStamina, -tickAmt, condition.Caster)
								}
							case "conviction":
								if tickAmt > 0 {
									user.Character.ApplyRestore(characters.PoolConviction, tickAmt)
								} else if tickAmt < 0 {
									user.Character.ApplyHarm(characters.PoolConviction, -tickAmt, condition.Caster)
								}
```

Replace in `internal/hooks/spell_effects.go` (1 of 3):

```go
func creditSpellDamage(c spellEffectCtx, dmg int) {
	m := c.targetMob()
	if m == nil || dmg <= 0 {
		return
	}
	if u := c.casterUser(); u != nil {
		m.Character.TrackPlayerDamage(u.UserId, dmg)
		return
	}
	if cm := c.casterMob(); cm != nil {
		if charmedUserId := cm.Character.GetCharmedUserId(); charmedUserId > 0 {
			m.Character.TrackPlayerDamage(charmedUserId, dmg)
		}
```

with:

```go
func creditSpellDamage(c spellEffectCtx, dmg int) {
	creditMobHarm(c.targetMob(), c.casterRef(), dmg)
}

// creditMobHarm is creditSpellDamage's rule by ref, shared with a
// damage-over-time tick (#240): a player is credited with dmg on the mob, a
// mob charmed by a player credits that player, and anything else credits
// nobody. The player need not be online: the damage map is keyed by id.
func creditMobHarm(m *mobs.Mob, caster state.ActorRef, dmg int) {
	if m == nil || dmg <= 0 {
		return
	}
	if caster.UserId != 0 {
		m.Character.TrackPlayerDamage(caster.UserId, dmg)
		return
	}
	if caster.MobInstanceId != 0 {
		if cm := mobs.GetInstance(caster.MobInstanceId); cm != nil {
			if charmedUserId := cm.Character.GetCharmedUserId(); charmedUserId > 0 {
				m.Character.TrackPlayerDamage(charmedUserId, dmg)
			}
		}
```

Replace in `internal/hooks/spell_effects.go` (2 of 3):

```go
//
// Unlike damage and knockdown, the dot does NOT credit its caster in the
// mob's PlayerDamage (creditSpellDamage): the ticks harm with an anonymous
// source, so a dot kill still counts for no one. Crediting it needs the
// caster carried on the condition record; that is a filed follow-up.
func applySpellDot(c spellEffectCtx) int {
```

with:

```go
//
// The record carries its caster (#240): each tick harms in the caster's name
// and credits them on a mob (creditMobHarm), so a dot kill counts for them.
func applySpellDot(c spellEffectCtx) int {
```

Replace in `internal/hooks/spell_effects.go` (3 of 3):

```go
	// and nothing that did not happen may be narrated.
	afflicted := tc.AddConditionMagnitude(conditions.ConditionIdPoisoned, dotDuration, -float64(dotAmount), "spell") == nil
	commitHarmfulSpellAggro(c, fresh)
```

with:

```go
	// and nothing that did not happen may be narrated.
	afflicted := tc.AddConditionMagnitudeBy(conditions.ConditionIdPoisoned, dotDuration, -float64(dotAmount), "spell", c.casterRef()) == nil
	commitHarmfulSpellAggro(c, fresh)
```

- [ ] **Step 5: Update the tests and guards that pin the old shape.**

Replace in `internal/hooks/spell_damage_credit_test.go` (1 of 1):

```go
// spellCreditEffects are the harmful effects that land damage on the spot.
// The dot is absent on purpose: its ticks harm anonymously (see applySpellDot).
func spellCreditEffects() []struct {
```

with:

```go
// spellCreditEffects are the harmful effects that land damage on the spot.
// The dot is absent: it credits per tick (condition_tick_credit_test.go).
func spellCreditEffects() []struct {
```

Replace in `internal/actions/command_readiness_drift_test.go` (1 of 2):

```go
			{"sp == nil || (sp.NaturalAttack != items.Bite && sp.NaturalAttack != items.Claws) || char.HasBodyPart(\"hands\")", "NotBeast"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddConditionMagnitude", "combat.RecordSpecialMove", "actor.AwardResolved"}, true},
		{"combat_rake.go", "ExecuteRake", "RakeResult", "costs.ActionRake", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsClawed(char)", "NotClawed"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddConditionMagnitude", "combat.RecordSpecialMove", "actor.AwardResolved"}, true},
		{"combat_maul.go", "ExecuteMaul", "MaulResult", "costs.ActionMaul", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsFanged(char)", "NotFanged"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddConditionMagnitude", "combat.RecordSpecialMove", "actor.AwardResolved"}, true},
		{"combat_pounce.go", "ExecutePounce", "PounceResult", "costs.ActionPounce", []specialMoveEarlyReturn{
```

with:

```go
			{"sp == nil || (sp.NaturalAttack != items.Bite && sp.NaturalAttack != items.Claws) || char.HasBodyPart(\"hands\")", "NotBeast"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddConditionMagnitudeBy", "combat.RecordSpecialMove", "actor.AwardResolved"}, true},
		{"combat_rake.go", "ExecuteRake", "RakeResult", "costs.ActionRake", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsClawed(char)", "NotClawed"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddConditionMagnitudeBy", "combat.RecordSpecialMove", "actor.AwardResolved"}, true},
		{"combat_maul.go", "ExecuteMaul", "MaulResult", "costs.ActionMaul", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsFanged(char)", "NotFanged"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddConditionMagnitudeBy", "combat.RecordSpecialMove", "actor.AwardResolved"}, true},
		{"combat_pounce.go", "ExecutePounce", "PounceResult", "costs.ActionPounce", []specialMoveEarlyReturn{
```

Replace in `internal/actions/command_readiness_drift_test.go` (2 of 2):

```go
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"!combat.SpeciesHasLifeDrain(char)", "NotLifeDrainer"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddConditionMagnitude", "char.Heal", "combat.RecordSpecialMove", "actor.AwardResolved"}, true},
		{"combat_throttle.go", "ExecuteThrottle", "ThrottleResult", "costs.ActionThrottle", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsFanged(char)", "NotFanged"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddConditionMagnitude", "target.Char.AddCondition", "InterruptTargetCast", "combat.RecordSpecialMove", "actor.AwardResolved"}, true},
	}
```

with:

```go
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"!combat.SpeciesHasLifeDrain(char)", "NotLifeDrainer"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddConditionMagnitudeBy", "char.Heal", "combat.RecordSpecialMove", "actor.AwardResolved"}, true},
		{"combat_throttle.go", "ExecuteThrottle", "ThrottleResult", "costs.ActionThrottle", []specialMoveEarlyReturn{
			{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"},
			{"char.HasBodyPart(\"hands\") || !combat.SpeciesIsFanged(char)", "NotFanged"},
		}, []string{"combat.ExecuteSkillMove", "target.Char.AddConditionMagnitudeBy", "target.Char.AddCondition", "InterruptTargetCast", "combat.RecordSpecialMove", "actor.AwardResolved"}, true},
	}
```

Replace in `condition_apply_path_guard_test.go` (1 of 4):

```go
	// when it gained its shield case, and again Task 5 when its doc comment
	// grew and the per-pairing fallthrough became the default arm) ──────
	"internal/hooks/spell_effects.go|348": "former combat condition (spell dot): silent-start record, the spell narrates the affliction; must apply synchronously so the refusal is known to the narrator",

```

with:

```go
	// when it gained its shield case, and again Task 5 when its doc comment
	// grew and the per-pairing fallthrough became the default arm; moved to
	// AddConditionMagnitudeBy and re-keyed messaging M6 slice 1, when the
	// dot began to carry its caster and creditMobHarm landed above it) ──────
	"internal/hooks/spell_effects.go|355": "former combat condition (spell dot): silent-start record, the spell narrates the affliction; must apply synchronously so the refusal is known to the narrator",

```

Replace in `condition_apply_path_guard_test.go` (2 of 4):

```go

var conditionAddCallPattern = regexp.MustCompile(`\.(AddCondition(?:Scaled|Magnitude)?)\(`)

```

with:

```go

// AddConditionMagnitudeBy is the character door with a caster (messaging M6
// slice 1); it applies in place exactly as AddConditionMagnitude does.
var conditionAddCallPattern = regexp.MustCompile(`\.(AddCondition(?:Scaled|Magnitude(?:By)?)?)\(`)

```

Replace in `condition_apply_path_guard_test.go` (3 of 4):

```go
func isEventPathCall(src string, method string, openParen int) (eventPath bool, parsed bool) {
	if method == "AddConditionMagnitude" {
		// The character door and the user door share a four-argument shape
```

with:

```go
func isEventPathCall(src string, method string, openParen int) (eventPath bool, parsed bool) {
	if method == "AddConditionMagnitude" || method == "AddConditionMagnitudeBy" {
		// The character door and the user door share a four-argument shape
```

Replace in `condition_apply_path_guard_test.go` (4 of 4):

```go
				advice := fmt.Sprintf("Route it through users.UserRecord.AddCondition / AddConditionScaled (or the mobs.Mob / actions.Actor equivalent, which all take a source string), or add %q to conditionApplyPathAllowlist with a reason.", key)
				if method := src[loc[2]:loc[3]]; method == "AddConditionMagnitude" {
					rule = "the character door applies in place and the user door queues the event, but they share one four-argument shape, so this guard reads every AddConditionMagnitude call as a direct add (the safe reading). Record why in conditionApplyPathAllowlist, or confirm the call is the user door and record that instead."
```

with:

```go
				advice := fmt.Sprintf("Route it through users.UserRecord.AddCondition / AddConditionScaled (or the mobs.Mob / actions.Actor equivalent, which all take a source string), or add %q to conditionApplyPathAllowlist with a reason.", key)
				if method := src[loc[2]:loc[3]]; method == "AddConditionMagnitude" || method == "AddConditionMagnitudeBy" {
					rule = "the character door applies in place and the user door queues the event, but they share one four-argument shape, so this guard reads every AddConditionMagnitude call as a direct add (the safe reading). Record why in conditionApplyPathAllowlist, or confirm the call is the user door and record that instead."
```

- [ ] **Step 6: Run the packages and the root guards.**

Run: `go test ./internal/hooks/ ./internal/actions/ ./internal/behaviortree/ -count=1`
Expected: PASS. `ok` for all three (actions takes about a minute).

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 7: Commit.**

```bash
git add internal/hooks/condition_tick_credit_test.go \
  internal/actions/actor_ref_test.go \
  internal/actions/combat_drain_test.go \
  internal/actions/combat_throttle_test.go \
  internal/behaviortree/item_proc_effects_test.go \
  internal/actions/actor_ref.go \
  internal/actions/combat_drain.go \
  internal/actions/combat_hamstring.go \
  internal/actions/combat_maul.go \
  internal/actions/combat_rake.go \
  internal/actions/combat_throttle.go \
  internal/behaviortree/actions_item_proc.go \
  internal/hooks/NewRound_MobRoundTick.go \
  internal/hooks/NewRound_UserRoundTick.go \
  internal/hooks/spell_effects.go \
  internal/hooks/spell_damage_credit_test.go \
  internal/actions/command_readiness_drift_test.go \
  condition_apply_path_guard_test.go
git commit -F - <<'EOF'
fix(hooks): a poison or bleed kill credits its caster (#240)

The round ticks harm in the record's caster's name, so a lethal tick
names the caster as killer, and a mob's health tick credits the caster in
the damage map every mob-death consumer reads (creditMobHarm, the
creditSpellDamage rule by ref). The spell dot and every combat bleed name
their caster through AddConditionMagnitudeBy. The apply-path guard now
sees that door too.

Refs #240.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 8: A condition spell drops its generic trio (R11)

`applySpellCondition` fills one `events.Condition` carrying the caster and the crit and queues it through `QueueCondition`; `applySpellConditionEffect` drops its trio when every condition it lands will tell its own start (`spellConditionsNarrateStart`). A silent condition and a re-cast keep the trio.

- [ ] **Step 1: Write the failing test.**

Create `internal/hooks/spell_condition_lines_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Owner ruling R11 at the spell: a condition spell whose condition authors
// its start lines prints those lines and not the spell's generic "takes
// effect" trio, so every audience reads exactly one line.

const (
	spellLineAuthoredId = 7211 // all three start lines
	spellLineSilentId   = 7212 // silent-start
)

// seedSpellLineConditions adds the two specs to the fixture's registry,
// keeping the fixture's own condition 100 and the condition records.
func seedSpellLineConditions(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		100: conditions.GetConditionSpec(100),
		spellLineAuthoredId: {ConditionId: spellLineAuthoredId, Name: "Test Aura", RoundInterval: 1, TriggerCount: 10,
			StartActorText: "An aura blooms around {actee}.", StartUserText: "An aura blooms around you.",
			StartRoomText: "An aura blooms around {actee_plain}.", EndUserText: "Your aura fades."},
		spellLineSilentId: {ConditionId: spellLineSilentId, Name: "Test Hush", RoundInterval: 1, TriggerCount: 10,
			Flags: []conditions.Flag{conditions.SilentStart}, EndUserText: "The hush lifts."},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
}

// applyQueuedConditions runs ApplyConditions over every condition event the
// spell queued for userId, as the event loop would.
func applyQueuedConditions(t *testing.T, userId int) []events.Condition {
	t.Helper()
	queued := events.DrainQueuedConditionsForTest(userId)
	for _, evt := range queued {
		ApplyConditions(evt)
	}
	return queued
}

func auraSpell(conditionId int) *spells.SpellData {
	s := conditionSpellForParityTest()
	s.Name = "Aura"
	s.ConditionIds = []int{conditionId}
	return s
}

func TestSpellConditionLines_CastOnAnotherIsOneLinePerAudience(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineAuthoredId)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	queued := applyQueuedConditions(t, 2)

	require.Len(t, queued, 1)
	assert.Equal(t, state.ActorRef{UserId: 1}, queued[0].Caster, "the spell names its caster on the event")
	assert.Equal(t, []string{"An aura blooms around Bobrick."}, drainPlain(1))
	assert.Equal(t, []string{"An aura blooms around you."}, drainPlain(2))
	assert.Equal(t, []string{"An aura blooms around Bobrick."}, drainPlain(3))
}

func TestSpellConditionLines_SelfCastIsOneLinePerAudience(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineAuthoredId)

	resolveAgainstPlayer(f.casterUser, f.casterUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	applyQueuedConditions(t, 1)

	assert.Equal(t, []string{"An aura blooms around you."}, drainPlain(1))
	assert.Equal(t, []string{"An aura blooms around Aliceia."}, drainPlain(3))
}

// A mob caster's condition on a player: the player and the room read the
// condition's lines; the mob has no client.
func TestSpellConditionLines_MobCasterOnAPlayer(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineAuthoredId)

	resolveMobSpellAgainstPlayer(f.casterMob, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, &f.casterMob.Character, nil), 0)
	queued := applyQueuedConditions(t, 2)

	require.Len(t, queued, 1)
	assert.Equal(t, state.ActorRef{MobInstanceId: 100}, queued[0].Caster)
	assert.Equal(t, []string{"An aura blooms around you."}, drainPlain(2))
	assert.Equal(t, []string{"An aura blooms around Bobrick."}, drainPlain(3))
}

// The crit marker leaves the dropped trio and rides the caster's line.
func TestSpellConditionLines_CritMarkerMovesToTheCasterLine(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackCrit())
	seedSpellLineConditions(t)
	spell := hexSpellForConditionTest()
	spell.ConditionIds = []int{spellLineAuthoredId}

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	applyQueuedConditions(t, 2)

	assert.Equal(t, []string{"An aura blooms around Bobrick. [CRIT!]"}, drainPlain(1))
	assert.Equal(t, []string{"An aura blooms around you."}, drainPlain(2))
}

// A silent-start condition tells nobody, so the spell keeps its trio.
func TestSpellConditionLines_ASilentConditionKeepsTheTrio(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineSilentId)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	applyQueuedConditions(t, 2)

	assert.Equal(t, []string{"Your Aura takes effect on Bobrick!"}, drainPlain(1))
	assert.Equal(t, []string{"Aliceia's Aura takes effect on you!"}, drainPlain(2))
	assert.Equal(t, []string{"Aliceia's Aura settles over Bobrick."}, drainPlain(3))
}

// A re-cast of a condition already held is a refresh: its start lines are
// suppressed, so the trio tells each audience the spell renewed it.
func TestSpellConditionLines_ARecastOfAnActiveConditionKeepsTheTrio(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineAuthoredId)
	require.NoError(t, f.targetUser.Character.AddCondition(spellLineAuthoredId, false))

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	applyQueuedConditions(t, 2)

	assert.Equal(t, []string{"Your Aura takes effect on Bobrick!"}, drainPlain(1))
	assert.Equal(t, []string{"Aliceia's Aura takes effect on you!"}, drainPlain(2))
	assert.Equal(t, []string{"Aliceia's Aura settles over Bobrick."}, drainPlain(3))
}
```

- [ ] **Step 2: Run it to see it fail.**

Run: `go test ./internal/hooks/ -run SpellConditionLines -count=1`
Expected: FAIL: each audience reads two lines (the trio, then the condition's), and the queued event has a zero `Caster`. The silent and re-cast tests pass (pins).

- [ ] **Step 3: Carry the caster and drop the trio.**

Replace in `internal/hooks/light_spell.go` (1 of 2):

```go
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
)
```

Replace in `internal/hooks/light_spell.go` (2 of 2):

```go
type spellConditionTarget interface {
	AddCondition(conditionId int, source string)
	AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string)
	AddConditionTickScaled(conditionId int, scale float64, source string)
}

// applySpellCondition applies one of a spell's conditions to its target: a
// magnitude-scaled light or sight at the caster's scaled value and duration,
// a heal- or damage-over-time at the caster's spellTickScale (the apply hook
// computes the amount where it lands), anything else at its authored values.
// Every door queues events.Condition, so the holder reads the start notice
// either way.
func applySpellCondition(target spellConditionTarget, spellData *spells.SpellData, caster *characters.Character, conditionId int) {
	if mag, ok := shroudSpellApplication(spellData, caster, conditionId); ok {
		target.AddConditionMagnitude(conditionId, 0, mag, "spell")
		return
	}
	if mag, trig, ok := magnitudeSpellApplication(spellData, caster, conditionId); ok {
		target.AddConditionMagnitude(conditionId, trig, mag, "spell")
		return
	}
	if spec := conditions.GetConditionSpec(conditionId); spec != nil && spec.TickPool != "" {
		target.AddConditionTickScaled(conditionId, spellTickScale(caster), "spell")
		return
	}
	target.AddCondition(conditionId, "spell")
}
```

with:

```go
type spellConditionTarget interface {
	QueueCondition(evt events.Condition)
}

// applySpellCondition applies one of a spell's conditions to its target: a
// magnitude-scaled light or sight at the caster's scaled value and duration,
// a heal- or damage-over-time at the caster's spellTickScale (the apply hook
// computes the amount where it lands), anything else at its authored values.
// The event names the caster (casterRef) and carries the crit marker for the
// caster's start line (messaging M6 slice 1), so the holder reads the start
// lines with the caster's name, and a tick that kills credits the caster.
func applySpellCondition(target spellConditionTarget, spellData *spells.SpellData, caster *characters.Character, conditionId int,
	casterRef state.ActorRef, crit bool) {
	evt := events.Condition{ConditionId: conditionId, Source: "spell", Caster: casterRef, CasterCrit: crit}
	if mag, ok := shroudSpellApplication(spellData, caster, conditionId); ok {
		evt.Magnitude = mag
	} else if mag, trig, ok := magnitudeSpellApplication(spellData, caster, conditionId); ok {
		evt.Magnitude, evt.Triggers = mag, trig
	} else if spec := conditions.GetConditionSpec(conditionId); spec != nil && spec.TickPool != "" {
		evt.TickScale = spellTickScale(caster)
	}
	target.QueueCondition(evt)
}
```

Replace in `internal/hooks/spell_help_effects.go` (1 of 2):

```go
	casterHidden, targetHidden := c.hiddenFromRoom()
	if target := spellConditionTargetOf(c.target); target != nil {
		for _, conditionId := range c.spell.ConditionIds {
			applySpellCondition(target, c.spell, c.casterChar, conditionId)
		}
	}
	if c.spell.IsHarm() {
		commitHarmfulSpellAggro(c, fresh)
	}
	// KNOWN AND DEFERRED: a condition with authored start text also narrates
	// this moment through the event applySpellCondition queues, so an
	// audience can read it twice. The messaging arc's M6 merges them.
	if c.selfCast() {
```

with:

```go
	casterHidden, targetHidden := c.hiddenFromRoom()
	// Read before the conditions land, as the apply hook's refresh test is:
	// a condition already held narrates no start.
	narrated := spellConditionsNarrateStart(c)
	if target := spellConditionTargetOf(c.target); target != nil {
		for _, conditionId := range c.spell.ConditionIds {
			applySpellCondition(target, c.spell, c.casterChar, conditionId, c.casterRef(), narrated && c.out.AttackerCrit)
		}
	}
	if c.spell.IsHarm() {
		commitHarmfulSpellAggro(c, fresh)
	}
	// Owner ruling R11: when the conditions' own start lines tell every
	// audience, they are the only lines (narrateConditionStart) and the
	// generic trio below is not sent. A silent condition, or a re-cast of one
	// already held, keeps the trio.
	if narrated {
		return 0
	}
	if c.selfCast() {
```

Replace in `internal/hooks/spell_help_effects.go` (2 of 2):

```go
			roomName(casterHidden, casterName), c.spell.Name, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// applySpellHeal is the one heal applier (slice 3b): a Regenerating record
```

with:

```go
			roomName(casterHidden, casterName), c.spell.Name, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// spellConditionsNarrateStart reports whether every condition the spell lands
// will tell its own start to every audience (ConditionSpec.NarratesCastStart),
// so the spell sends no generic "takes effect" lines of its own (owner ruling
// R11). A condition the target already holds is a refresh with no start
// lines, so it keeps the spell's lines; so does a spell that lands nothing.
func spellConditionsNarrateStart(c spellEffectCtx) bool {
	if len(c.spell.ConditionIds) == 0 {
		return false
	}
	for _, conditionId := range c.spell.ConditionIds {
		spec := conditions.GetConditionSpec(conditionId)
		if spec == nil || c.targetChar().HasCondition(conditionId) || !spec.NarratesCastStart(c.selfCast()) {
			return false
		}
	}
	return true
}

// applySpellHeal is the one heal applier (slice 3b): a Regenerating record
```

- [ ] **Step 4: Update the callers and the guard.**

Replace in `internal/hooks/shroud_hide_test.go` (1 of 1):

```go

	applySpellCondition(holder, spell, caster.Character, conditions.ConditionIdEmpathicShroud)
	q := events.DrainQueuedConditionsForTest(holder.UserId)
```

with:

```go

	applySpellCondition(holder, spell, caster.Character, conditions.ConditionIdEmpathicShroud, state.ActorRef{}, false)
	q := events.DrainQueuedConditionsForTest(holder.UserId)
```

Replace in `internal/hooks/spell_tick_scale_test.go` (1 of 2):

```go
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
```

Replace in `internal/hooks/spell_tick_scale_test.go` (2 of 2):

```go
		t.Run(tg.name, func(t *testing.T) {
			applySpellCondition(tg.door, spell, caster, spellPathTickConditionId)
			q := tg.drain()
			require.Len(t, q, 1)
			assert.Equal(t, want, q[0].TickScale, "a tick_pool condition carries the caster's scale")

			applySpellCondition(tg.door, spell, caster, spellPathPlainConditionId)
			q = tg.drain()
			require.Len(t, q, 1)
			assert.Zero(t, q[0].TickScale, "a non-ticking condition carries no scale")

			applySpellCondition(tg.door, spell, caster, spellPathGlowConditionId)
			q = tg.drain()
```

with:

```go
		t.Run(tg.name, func(t *testing.T) {
			applySpellCondition(tg.door, spell, caster, spellPathTickConditionId, state.ActorRef{}, false)
			q := tg.drain()
			require.Len(t, q, 1)
			assert.Equal(t, want, q[0].TickScale, "a tick_pool condition carries the caster's scale")

			applySpellCondition(tg.door, spell, caster, spellPathPlainConditionId, state.ActorRef{}, false)
			q = tg.drain()
			require.Len(t, q, 1)
			assert.Zero(t, q[0].TickScale, "a non-ticking condition carries no scale")

			applySpellCondition(tg.door, spell, caster, spellPathGlowConditionId, state.ActorRef{}, false)
			q = tg.drain()
```

Replace in `condition_apply_path_guard_test.go` (1 of 3):

```go
	// when the party rule moved to actions.HelpCharmAlly and the parties
	// import left spell_help_effects.go) ────
	"internal/hooks/spell_help_effects.go|195": "former combat condition (ward): silent-start record, the spell narrates; must apply synchronously so the same resolution pass sees it",

```

with:

```go
	// when the party rule moved to actions.HelpCharmAlly and the parties
	// import left spell_help_effects.go; re-keyed again messaging M6 slice 1,
	// when spellConditionsNarrateStart landed above it) ────
	"internal/hooks/spell_help_effects.go|220": "former combat condition (ward): silent-start record, the spell narrates; must apply synchronously so the same resolution pass sees it",

```

Replace in `condition_apply_path_guard_test.go` (2 of 3):

```go
	// Task 7, same import shift as above; re-keyed again 3b playtest fix,
	// same import shift as above) ───────
	"internal/hooks/spell_help_effects.go|144": "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
	"internal/mobcommands/consume.go|46":       "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
```

with:

```go
	// Task 7, same import shift as above; re-keyed again 3b playtest fix,
	// same import shift as above; re-keyed again messaging M6 slice 1, same
	// shift as above) ───────
	"internal/hooks/spell_help_effects.go|169": "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
	"internal/mobcommands/consume.go|46":       "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
```

Replace in `condition_apply_path_guard_test.go` (3 of 3):

```go
	"internal/hooks/spell_effects.go|355": "former combat condition (spell dot): silent-start record, the spell narrates the affliction; must apply synchronously so the refusal is known to the narrator",

	// ── light spells (lighting plan 5a): an EVENT door, not the silent
	// character door. applySpellCondition's target is a spellConditionTarget,
	// which the silent Character door cannot satisfy (its
	// AddConditionMagnitude returns an error); its implementers are
	// *users.UserRecord and *mobs.Mob, both of which queue events.Condition,
	// so Condition_ApplyConditions runs and narrates the start ─────────────
	// Re-keyed lighting plan 5c, when the hook became magnitudeSpellApplication
	// and also scales nightvision and infra reach, and again when its formula
	// moved to conditions.SpellScaledMagnitude, and again when the
	// spellConditionTarget interface gained AddConditionTickScaled (parity
	// slice 2).
	"internal/hooks/light_spell.go|71": "Empathic Shroud at the caster's shroud score (#444): the EVENT door (users.UserRecord / mobs.Mob AddConditionMagnitude both queue events.Condition); listed only because arity cannot tell it from the silent character door",
	"internal/hooks/light_spell.go|75": "light, sight or darkness spell at the caster's scaled magnitude and triggers: the EVENT door (users.UserRecord / mobs.Mob AddConditionMagnitude both queue events.Condition); listed only because arity cannot tell it from the silent character door",

```

with:

```go
	"internal/hooks/spell_effects.go|355": "former combat condition (spell dot): silent-start record, the spell narrates the affliction; must apply synchronously so the refusal is known to the narrator",

```

- [ ] **Step 5: Run hooks and the root guards.**

Run: `go test ./internal/hooks/ -count=1`
Expected: PASS. `ok`.

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 6: Commit.**

```bash
git add internal/hooks/spell_condition_lines_test.go \
  internal/hooks/light_spell.go \
  internal/hooks/spell_help_effects.go \
  internal/hooks/shroud_hide_test.go \
  internal/hooks/spell_tick_scale_test.go \
  condition_apply_path_guard_test.go
git commit -F - <<'EOF'
feat(hooks): a condition spell tells one line per audience (#370)

applySpellCondition queues one event carrying the caster and the crit
marker, and applySpellConditionEffect drops its generic takes-effect trio
when the condition's own start lines will tell every audience. A silent
condition, or a re-cast of one already held, keeps the trio.

Refs #370.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 9: Three wards (#338, R3, R7)

Conviction Ward (119, renamed from Minor Shield), the new Conviction Bulwark (135) and Chrysalis Cocoon (136) are ward-family conditions named by their spells' `condition_ids`; `applySpellShield` lands the spell's own ward through the event (D9) and keeps the shield formula (D10). The mob AI's "already shielded" is the ward family.

- [ ] **Step 1: Write the failing tests.**

Create `spell_condition_data_guard_test.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"gopkg.in/yaml.v3"
)

// Messaging M6 slice 1: the shipped spells and the conditions they land.
// Read from the YAML on disk, so a data edit that breaks a rule fails here
// and names the file.

// shippedSpells loads every dogmud spell, keyed by spell id.
func shippedSpells(t *testing.T) map[string]*spells.SpellData {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "spells", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no spell files found: %v", err)
	}
	out := map[string]*spells.SpellData{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var s spells.SpellData
		if err := yaml.Unmarshal(raw, &s); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out[s.SpellId] = &s
	}
	return out
}

// shippedConditions loads every dogmud condition, keyed by id.
func shippedConditions(t *testing.T) map[int]*conditions.ConditionSpec {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "conditions", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no condition files found: %v", err)
	}
	out := map[int]*conditions.ConditionSpec{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var c conditions.ConditionSpec
		if err := yaml.Unmarshal(raw, &c); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out[c.ConditionId] = &c
	}
	return out
}

func sortedSpellIds(all map[string]*spells.SpellData) []string {
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// conditionIdReaders are the effect types whose applier reads condition_ids
// (internal/hooks: applySpellConditionEffect, applySpellShield,
// applySpellHeal). On any other effect type the list is dead data that
// promises the player something the spell never does: Chrysalis Cocoon's
// [52] and Mass Mend's [33] were exactly that.
var conditionIdReaders = []string{"condition", "shield", "heal"}

func TestSpellConditionIdsAreReadByTheirEffectType(t *testing.T) {
	all := shippedSpells(t)
	conds := shippedConditions(t)
	var problems []string
	for _, id := range sortedSpellIds(all) {
		s := all[id]
		if len(s.ConditionIds) == 0 {
			continue
		}
		if !slices.Contains(conditionIdReaders, s.EffectType) {
			problems = append(problems, fmt.Sprintf("%s: effect_type %q never reads condition_ids %v", id, s.EffectType, s.ConditionIds))
		}
		for _, cid := range s.ConditionIds {
			if conds[cid] == nil {
				problems = append(problems, fmt.Sprintf("%s: condition %d does not exist", id, cid))
			}
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d spell condition_ids problems:\n  - %s", len(problems), strings.Join(problems, "\n  - "))
	}
}

// familyFor is the family each applier's own condition must belong to, and
// the effect it must read from the applier's magnitude.
var familyFor = map[string]struct {
	family string
	reads  conditions.EffectKind
}{
	"shield": {conditions.FamilyWard, conditions.EffectMitigationFlat},
}

// R3, R4 and R7: each shield spell lands its own condition, one per spell,
// in the ward family, reading the spell's strength from its magnitude.
func TestShieldAndHealSpellsLandTheirOwnCondition(t *testing.T) {
	all := shippedSpells(t)
	conds := shippedConditions(t)
	owner := map[int]string{}
	var problems []string
	for _, id := range sortedSpellIds(all) {
		s := all[id]
		want, ok := familyFor[s.EffectType]
		if !ok {
			continue
		}
		if len(s.ConditionIds) != 1 {
			problems = append(problems, fmt.Sprintf("%s: a %s spell names exactly one condition, has %v", id, s.EffectType, s.ConditionIds))
			continue
		}
		cid := s.ConditionIds[0]
		c := conds[cid]
		if c == nil {
			continue // reported by TestSpellConditionIdsAreReadByTheirEffectType
		}
		if c.Family != want.family {
			problems = append(problems, fmt.Sprintf("%s: condition %d (%s) is family %q, want %q", id, cid, c.Name, c.Family, want.family))
		}
		if v, ok := c.Effects[want.reads]; !ok || !v.UsesMagnitude {
			problems = append(problems, fmt.Sprintf("%s: condition %d (%s) must read %s from the spell's magnitude", id, cid, c.Name, want.reads))
		}
		if prev, dup := owner[cid]; dup {
			problems = append(problems, fmt.Sprintf("%s: condition %d is also %s's; each spell has its own", id, cid, prev))
		}
		owner[cid] = id
	}
	if len(problems) > 0 {
		t.Fatalf("%d shield or heal spell problems:\n  - %s", len(problems), strings.Join(problems, "\n  - "))
	}
}

// R7: three wards, weakest to strongest, each blocking more kinds of damage
// than the last. Strength and cost rise with the kinds.
func TestWardRosterHoldsItsOrdering(t *testing.T) {
	all := shippedSpells(t)
	conds := shippedConditions(t)
	roster := []struct {
		spellId string
		kinds   []conditions.EffectKind
	}{
		{"conviction-ward", []conditions.EffectKind{conditions.EffectMitigationFlat}},
		{"conviction-bulwark", []conditions.EffectKind{conditions.EffectMitigationFlat, conditions.EffectMitigationMagical}},
		{"chrysalis-cocoon", []conditions.EffectKind{conditions.EffectMitigationFlat, conditions.EffectMitigationMagical, conditions.EffectMitigationConviction}},
	}
	every := []conditions.EffectKind{conditions.EffectMitigationFlat, conditions.EffectMitigationMagical, conditions.EffectMitigationConviction}
	prevMag, prevCost := 0, 0
	for _, w := range roster {
		s := all[w.spellId]
		if s == nil || len(s.ConditionIds) != 1 || conds[s.ConditionIds[0]] == nil {
			t.Fatalf("%s: missing, or not landing exactly one shipped condition", w.spellId)
		}
		c := conds[s.ConditionIds[0]]
		for _, k := range every {
			_, has := c.Effects[k]
			if has != slices.Contains(w.kinds, k) {
				t.Errorf("%s: ward %d (%s) blocks %s = %v, want %v", w.spellId, c.ConditionId, c.Name, k, has, !has)
			}
		}
		if s.EffectMagnitude <= prevMag || s.Cost <= prevCost {
			t.Errorf("%s: magnitude %d and cost %d must both exceed the weaker ward's %d and %d",
				w.spellId, s.EffectMagnitude, s.Cost, prevMag, prevCost)
		}
		prevMag, prevCost = s.EffectMagnitude, s.Cost
	}
}
```

Replace in `internal/behaviortree/defensive_caster_archetype_integration_test.go` (1 of 5):

```go

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
```

with:

```go

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
```

Replace in `internal/behaviortree/defensive_caster_archetype_integration_test.go` (2 of 5):

```go
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle, Cost: 60, BaseFolds: 8,
			EffectType: "shield", EffectMagnitude: 125, ConditionIds: []int{52},
			Categories: []string{"self_defense"},
```

with:

```go
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle, Cost: 60, BaseFolds: 8,
			EffectType: "shield", EffectMagnitude: 125, ConditionIds: []int{136},
			Categories: []string{"self_defense"},
```

Replace in `internal/behaviortree/defensive_caster_archetype_integration_test.go` (3 of 5):

```go

// TestDefensiveCaster_CocoonActive_SingleEnemy_CastsHarmSingle verifies the
// cast_best_in_category "already active" semantics from
// action_cast_best_in_category.go: spellEffectAlreadyActive skips a
// candidate when ANY of its ConditionIds is already present on the caster.
// chrysalis-cocoon carries ConditionIds:[52], so seeding condition 52 (not the Minor
// Shield record, which is a different, EffectType=="shield" check that
// chrysalis-cocoon also has but isn't required to trip this skip) is
// sufficient to remove it as a self_defense candidate. With no other
// self_defense spell known, that branch has no candidates and Fails;
// with a single enemy (no room setup => multiple_enemies Fails), the
// selector reaches harm_single and casts conviction-spike.
func TestDefensiveCaster_CocoonActive_SingleEnemy_CastsHarmSingle(t *testing.T) {
	defer seedDefensiveCasterSpells(t)()
```

with:

```go

// TestDefensiveCaster_AnyWardActive_SingleEnemy_CastsHarmSingle verifies the
// cast_best_in_category "already active" semantics from
// action_cast_best_in_category.go: spellEffectAlreadyActive skips a shield
// spell when ANY ward is up, the ward family (messaging M6 slice 1), not
// only the spell's own. A new ward would replace the old one, so casting
// chrysalis-cocoon over a Conviction Bulwark only swaps wards. The seeded
// ward is a different id from the cocoon's own (136) and declares no
// mitigation_flat, so neither the old ConditionIds branch nor the old
// HasEffect(mitigation_flat) test could see it. With no other self_defense
// spell known, that branch has no candidates and Fails; with a single enemy
// (no room setup => multiple_enemies Fails), the selector reaches
// harm_single and casts conviction-spike.
func TestDefensiveCaster_AnyWardActive_SingleEnemy_CastsHarmSingle(t *testing.T) {
	defer seedDefensiveCasterSpells(t)()
```

Replace in `internal/behaviortree/defensive_caster_archetype_integration_test.go` (4 of 5):

```go

	// Condition 52 is what chrysalis-cocoon's cast actually grants; seeding it
	// is what makes spellEffectAlreadyActive skip the spell via the
	// ConditionIds branch.
	defer seedConditionOnChar(t, &mob.Character, 52)()

```

with:

```go

	// A Conviction Bulwark (135) is up: any ward makes
	// spellEffectAlreadyActive skip every shield spell.
	defer seedWardOnChar(t, &mob.Character, 135)()

```

Replace in `internal/behaviortree/defensive_caster_archetype_integration_test.go` (5 of 5):

```go
	if !strings.HasPrefix(cmd, "cast conviction-spike") {
		t.Fatalf("cocoon-active, single-enemy caster should cast conviction-spike (harm_single), got %q", cmd)
	}
}
```

with:

```go
	if !strings.HasPrefix(cmd, "cast conviction-spike") {
		t.Fatalf("ward-active, single-enemy caster should cast conviction-spike (harm_single), got %q", cmd)
	}
}

// seedWardOnChar puts a ward-family record with no effects on char, so only
// the ward family test can see it.
func seedWardOnChar(t *testing.T, char *characters.Character, conditionId int) func() {
	t.Helper()
	cleanup := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		conditionId: {ConditionId: conditionId, Name: "Test Bulwark", Family: conditions.FamilyWard},
	})
	char.Conditions.List = append(char.Conditions.List, &conditions.Condition{ConditionId: conditionId, TriggersLeft: 5})
	char.Conditions.Validate(true)
	return cleanup
}
```

Replace in `internal/hooks/spell_shield_test.go` (1 of 5):

```go
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/stretchr/testify/assert"
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
```

Replace in `internal/hooks/spell_shield_test.go` (2 of 5):

```go

// shieldRecord returns c's one Minor Shield record, or fails the test.
func shieldRecord(t *testing.T, c *characters.Character) *conditions.Condition {
	t.Helper()
	recs := c.GetConditions(conditions.ConditionIdMinorShield)
	require.Len(t, recs, 1, "the shield must leave one Minor Shield record")
	return recs[0]
```

with:

```go

// landQueuedConditions applies every queued condition event as the event
// loop would. A ward or a heal lands through events.Condition (messaging M6
// slice 1), so it is held only once its event is applied.
func landQueuedConditions() {
	for _, evt := range events.DrainQueuedMobConditionsForTest(0) { // zero drains every holder
		ApplyConditions(evt)
	}
}

// shieldRecord lands the queued ward and returns c's one Conviction Ward
// record (the ward a shield spell that names none lands), or fails the test.
func shieldRecord(t *testing.T, c *characters.Character) *conditions.Condition {
	t.Helper()
	landQueuedConditions()
	recs := c.GetConditions(conditions.ConditionIdConvictionWard)
	require.Len(t, recs, 1, "the shield must leave one Conviction Ward record")
	return recs[0]
```

Replace in `internal/hooks/spell_shield_test.go` (3 of 5):

```go
	assert.Equal(t, calcSpellDuration(4, 3, 100), rec.TriggersLeft)
	assert.Equal(t, 1, countContaining(drainPlain(3), "A shimmering barrier surrounds Ghoul"))
}
```

with:

```go
	assert.Equal(t, calcSpellDuration(4, 3, 100), rec.TriggersLeft)
	assert.Equal(t, state.ActorRef{UserId: 1}, rec.Caster, "the ward remembers its caster")
	assert.Equal(t, []string{"A faint ward of conviction shimmers around Ghoul."}, drainPlain(3))
	assert.Equal(t, []string{"Your conviction hardens into a ward around Ghoul."}, drainPlain(1),
		"the ward's caster line is the caster's only line")
}
```

Replace in `internal/hooks/spell_shield_test.go` (4 of 5):

```go
	assert.Equal(t, parityShieldBonus(), shieldRecord(t, f.targetUser.Character).Magnitude)
	assert.Equal(t, 1, countContaining(drainPlain(2), "A shimmering magical barrier forms around you"))
}
```

with:

```go
	assert.Equal(t, parityShieldBonus(), shieldRecord(t, f.targetUser.Character).Magnitude)
	assert.Equal(t, []string{"A ward of hardened conviction settles around you."}, drainPlain(2))
}
```

Replace in `internal/hooks/spell_shield_test.go` (5 of 5):

```go
		"a crit must not strengthen a shield")
}

```

with:

```go
		"a crit must not strengthen a shield")
}

const testBulwarkId = 7221 // a second ward, physical and spell

// seedTestBulwark adds a second ward to the fixture's registry, keeping the
// fixture's condition 100 and the condition records (119 among them).
func seedTestBulwark(t *testing.T) {
	t.Helper()
	mag := conditions.EffectValue{UsesMagnitude: true}
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		100: conditions.GetConditionSpec(100),
		testBulwarkId: {ConditionId: testBulwarkId, Name: "Test Bulwark", Family: conditions.FamilyWard,
			RoundInterval: 1, TriggerCount: 10,
			Effects: map[conditions.EffectKind]conditions.EffectValue{
				conditions.EffectMitigationFlat: mag, conditions.EffectMitigationMagical: mag},
			StartActorText: "A bulwark rises around {actee}.", StartUserText: "A bulwark rises around you.",
			StartRoomText: "A bulwark rises around {actee_plain}.", EndUserText: "Your Test Bulwark fades."},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
}

// R3 and R7: a shield spell lands its own ward, named by its condition_ids,
// and that ward's kinds all read the one shield strength.
func TestSpellShield_LandsTheSpellsOwnWard(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedTestBulwark(t)
	spell := shieldSpellForParityTest()
	spell.ConditionIds = []int{testBulwarkId}

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)
	landQueuedConditions()

	c := f.targetUser.Character
	require.True(t, c.HasCondition(testBulwarkId))
	assert.False(t, c.HasCondition(conditions.ConditionIdConvictionWard))
	assert.Equal(t, parityShieldBonus(), c.Conditions.Effect(conditions.EffectMitigationFlat))
	assert.Equal(t, parityShieldBonus(), c.Conditions.Effect(conditions.EffectMitigationMagical))
	assert.Zero(t, c.Conditions.Effect(conditions.EffectMitigationConviction), "a ward blocks only what it lists")
}

// R4 and R6: a second ward replaces the first, announced to the holder.
func TestSpellShield_ASecondWardReplacesTheFirst(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedTestBulwark(t)
	ward := shieldSpellForParityTest()
	bulwark := shieldSpellForParityTest()
	bulwark.ConditionIds = []int{testBulwarkId}

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, ward,
		spellAttackSideFor(ward, f.casterUser.Character, nil), ward.EffectMagnitude)
	landQueuedConditions()
	drainPlain(2)
	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, bulwark,
		spellAttackSideFor(bulwark, f.casterUser.Character, nil), bulwark.EffectMagnitude)
	landQueuedConditions()

	c := f.targetUser.Character
	assert.False(t, c.HasCondition(conditions.ConditionIdConvictionWard))
	assert.True(t, c.HasCondition(testBulwarkId))
	assert.Equal(t, []string{"Your Conviction Ward fades as Test Bulwark takes hold.", "A bulwark rises around you."}, drainPlain(2))
}

// A re-cast of a ward already held is a refresh: the ward's start lines stay
// quiet, so the spell's own lines tell each audience it was renewed.
func TestSpellShield_ARecastOfTheSameWardKeepsTheSpellsLines(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	spell := shieldSpellForParityTest()
	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)
	landQueuedConditions()
	drainPlain(1)
	drainPlain(2)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)
	landQueuedConditions()

	assert.Equal(t, []string{"A shimmering magical barrier forms around you, bolstering your defenses."}, drainPlain(2))
}

```

- [ ] **Step 2: Run them to see them fail.**

Run: `go test . -run "TestShieldAndHealSpellsLandTheirOwnCondition|TestWardRosterHoldsItsOrdering|TestSpellConditionIdsAreReadByTheirEffectType" -count=1`
Expected: FAIL: `chrysalis-cocoon: condition 52 (Chrysalis Shell) is family "", want "ward"`, `conviction-ward: a shield spell names exactly one condition, has []`, `conviction-ward: missing, or not landing exactly one shipped condition`. `TestSpellConditionIdsAreReadByTheirEffectType` passes (a pin against dead data returning).

Run: `go test ./internal/behaviortree/ -run AnyWardActive -count=1`
Expected: FAIL: the caster casts `chrysalis-cocoon` over a different ward.

Run: `go vet ./internal/hooks/`
Expected: FAIL: `undefined: conditions.ConditionIdConvictionWard`.

- [ ] **Step 3: Rename the 119 file.**

Run: `git mv _datafiles/world/dogmud/conditions/119-minor_shield.yaml _datafiles/world/dogmud/conditions/119-conviction_ward.yaml`
Expected: no output.

- [ ] **Step 4: Write the wards, the new spell and the spells' wards.**

Write the whole file `_datafiles/world/dogmud/conditions/119-conviction_ward.yaml`:

```yaml
conditionid: 119
name: Conviction Ward
description: A ward of hardened belief turns aside some of the physical
  blows that reach you. It does nothing against spells or harsh words. A
  newer ward takes its place rather than adding to it.
# The ward family (messaging M6 slice 1, owner rulings R4 and R6): a new
# ward replaces this one instead of stacking. Potions and gear still stack.
family: ward
triggerrate: 1 round
# A floor for the born-dead guard; the spell sets the real count from its
# caster (applySpellShield).
triggercount: 10
effects:
  mitigation_flat: magnitude
start_actor: "Your conviction hardens into a ward around {actee}."
start_actee: "A ward of hardened conviction settles around you."
start_observer: "A faint ward of conviction shimmers around {actee}."
end_actee: "Your Conviction Ward fades."
end_observer: "The ward around {actee} fades."
```

Create `_datafiles/world/dogmud/conditions/135-conviction_bulwark.yaml`:

```yaml
conditionid: 135
name: Conviction Bulwark
description: A bulwark of belief blunts physical blows and spells alike. It
  does nothing against harsh words. A newer ward takes its place rather than
  adding to it.
family: ward
triggerrate: 1 round
# A floor for the born-dead guard; the spell sets the real count from its
# caster (applySpellShield).
triggercount: 10
effects:
  mitigation_flat: magnitude
  mitigation_magical: magnitude
start_actor: "Your belief rises into a bulwark around {actee}."
start_actee: "A bulwark of belief rises around you."
start_observer: "A shimmering bulwark of belief rises around {actee}."
end_actee: "Your Conviction Bulwark fades."
end_observer: "The bulwark around {actee} fades."
```

Create `_datafiles/world/dogmud/conditions/136-chrysalis_cocoon.yaml`:

```yaml
conditionid: 136
name: Chrysalis Cocoon
description: A shell of living Chrysalis blunts physical blows, spells and
  harsh words alike. A newer ward takes its place rather than adding to it.
family: ward
triggerrate: 1 round
# A floor for the born-dead guard; the spell sets the real count from its
# caster (applySpellShield).
triggercount: 10
effects:
  mitigation_flat: magnitude
  mitigation_magical: magnitude
  mitigation_conviction: magnitude
start_actor: "Living Chrysalis weaves a cocoon around {actee}."
start_actee: "Living Chrysalis weaves a cocoon around you."
start_observer: "Tendrils of living Chrysalis weave a cocoon around {actee}."
end_actee: "Your Chrysalis Cocoon splits and flakes away."
end_observer: "The cocoon around {actee} splits and flakes away."
```

Create `_datafiles/world/dogmud/spells/conviction-bulwark.yaml`:

```yaml
spellid: conviction-bulwark
name: Conviction Bulwark
aliases: [bulwark]
description: Raises belief into a bulwark that blunts blows and spells.
attack_type: none
damage_type: non_harm
targeting: single
schools:
  - enhancement
cost: 45
waitrounds: 1
difficulty: 15
primarystat: willpower
base_folds: 6
effect_type: shield
effect_magnitude: 100
# The ward this spell lands: physical and spell, the middle of the three
# (messaging M6 slice 1, owner ruling R7).
condition_ids:
  - 135
categories:
  - self_defense
cast_actor: "You raise your belief into a bulwark."
cast_observer: "{actor} lifts their hands as belief gathers into a bulwark."
```

Replace in `_datafiles/world/dogmud/spells/conviction-ward.yaml` (1 of 1):

```yaml
effect_magnitude: 75
categories:
```

with:

```yaml
effect_magnitude: 75
# The ward this spell lands: physical only, the weakest of the three
# (messaging M6 slice 1, owner ruling R7).
condition_ids:
  - 119
categories:
```

Replace in `_datafiles/world/dogmud/spells/chrysalis-cocoon.yaml` (1 of 1):

```yaml
effect_magnitude: 125
condition_ids:
  - 52
categories:
```

with:

```yaml
effect_magnitude: 125
# The ward this spell lands: physical, spell and social, the strongest of
# the three (messaging M6 slice 1, owner ruling R7).
condition_ids:
  - 136
categories:
```

- [ ] **Step 5: Write the help pages and the alias.**

Replace in `_datafiles/world/dogmud/templates/help/conviction-ward.template` (1 of 1):

```text

The <ansi fg="command">Conviction Ward</ansi> spell weaves spiritual energy into a fast, lightweight
barrier around a target. It is cheaper and quicker to cast than heavier
shield spells, making it ideal for use in the middle of a fight.

The ward's protective strength scales with the caster's willpower and
spellcasting skill. It provides physical damage mitigation only.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">cast conviction-ward</ansi>              Wards yourself.
  <ansi fg="command">cast conviction-ward [target]</ansi>     Wards a nearby ally.

<ansi fg="yellow">Base Folds:  </ansi> <ansi fg="white-bold">3</ansi>
<ansi fg="yellow">Target:      </ansi> <ansi fg="white-bold">Single ally (or self)</ansi>
<ansi fg="yellow">Defense:     </ansi> <ansi fg="white-bold">None — help spells always apply</ansi>
<ansi fg="yellow">Duration:    </ansi> <ansi fg="white-bold">Scales with spellcasting skill</ansi>
<ansi fg="yellow">Strength:    </ansi> <ansi fg="white-bold">Modest, and the quickest to raise</ansi>
<ansi fg="yellow">Mitigation:  </ansi> <ansi fg="white-bold">Physical only</ansi>
<ansi fg="yellow">Conv. Cost:  </ansi> <ansi fg="white-bold">3</ansi>
<ansi fg="yellow">School:      </ansi> <ansi fg="white-bold">Enhancement</ansi>

<ansi fg="white-bold">Notes:</ansi>
  - Lighter than Chrysalis Cocoon but provides physical protection only.
  - A good choice when conviction is running low mid-fight.
  - Casting again while a ward is active overwrites the existing one.
  - Crits boost the barrier's strength by roughly half again.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help cast</ansi>, <ansi fg="command">help spells</ansi>, <ansi fg="command">help chrysalis-cocoon</ansi>

```

with:

```text

The <ansi fg="command">Conviction Ward</ansi> spell hardens belief into a light ward around a
target. It is the cheapest and quickest ward to cast, which makes it a good
choice in the middle of a fight.

The ward turns aside part of every physical blow. It does nothing against
spells or harsh words. Its strength and how long it lasts grow with the
caster's willpower and spellcasting skill.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">cast conviction-ward</ansi>              Wards yourself.
  <ansi fg="command">cast conviction-ward [target]</ansi>     Wards a nearby ally.

<ansi fg="yellow">Base Folds:  </ansi> <ansi fg="white-bold">4</ansi>
<ansi fg="yellow">Target:      </ansi> <ansi fg="white-bold">Single ally (or self)</ansi>
<ansi fg="yellow">Defense:     </ansi> <ansi fg="white-bold">None, help spells always apply</ansi>
<ansi fg="yellow">Duration:    </ansi> <ansi fg="white-bold">Scales with spellcasting skill</ansi>
<ansi fg="yellow">Strength:    </ansi> <ansi fg="white-bold">The weakest of the three wards</ansi>
<ansi fg="yellow">Blocks:      </ansi> <ansi fg="white-bold">Physical damage only</ansi>
<ansi fg="yellow">Conv. Cost:  </ansi> <ansi fg="white-bold">30</ansi>
<ansi fg="yellow">School:      </ansi> <ansi fg="white-bold">Enhancement</ansi>

<ansi fg="white-bold">Notes:</ansi>
  - A target holds one ward at a time. A new ward replaces the old one.
  - Potions, gear and mutations that protect you still add on top.
  - Casting it again on the same target renews it.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help wards</ansi>, <ansi fg="command">help conviction-bulwark</ansi>, <ansi fg="command">help chrysalis-cocoon</ansi>

```

Create `_datafiles/world/dogmud/templates/help/conviction-bulwark.template`:

```text
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">conviction-bulwark</ansi> spell

The <ansi fg="command">Conviction Bulwark</ansi> spell raises belief into a bulwark around a
target. It costs more than a Conviction Ward and takes longer to cast, but it
is stronger and it also blunts spells.

The bulwark turns aside part of every physical blow and every spell. It
does nothing against harsh words. Its strength and how long it lasts grow
with the caster's willpower and spellcasting skill.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">cast conviction-bulwark</ansi>              Wards yourself.
  <ansi fg="command">cast conviction-bulwark [target]</ansi>     Wards a nearby ally.

<ansi fg="yellow">Base Folds:  </ansi> <ansi fg="white-bold">6</ansi>
<ansi fg="yellow">Target:      </ansi> <ansi fg="white-bold">Single ally (or self)</ansi>
<ansi fg="yellow">Defense:     </ansi> <ansi fg="white-bold">None, help spells always apply</ansi>
<ansi fg="yellow">Duration:    </ansi> <ansi fg="white-bold">Scales with spellcasting skill</ansi>
<ansi fg="yellow">Strength:    </ansi> <ansi fg="white-bold">The middle of the three wards</ansi>
<ansi fg="yellow">Blocks:      </ansi> <ansi fg="white-bold">Physical damage and spells</ansi>
<ansi fg="yellow">Conv. Cost:  </ansi> <ansi fg="white-bold">45</ansi>
<ansi fg="yellow">School:      </ansi> <ansi fg="white-bold">Enhancement</ansi>

<ansi fg="white-bold">Notes:</ansi>
  - A target holds one ward at a time. A new ward replaces the old one.
  - Potions, gear and mutations that protect you still add on top.
  - Casting it again on the same target renews it.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help wards</ansi>, <ansi fg="command">help conviction-ward</ansi>, <ansi fg="command">help chrysalis-cocoon</ansi>
```

Replace in `_datafiles/world/dogmud/templates/help/chrysalis-cocoon.template` (1 of 2):

```text

The <ansi fg="command">Chrysalis Cocoon</ansi> spell envelops a target in a shimmering chrysalis of
hardened conviction energy. It is the most powerful shield spell available,
providing substantial protection against physical, magical, and rhetoric
damage alike.

The cocoon's strength scales with the caster's willpower and spellcasting
skill. It costs more than lighter wards but lasts considerably longer.

```

with:

```text

The <ansi fg="command">Chrysalis Cocoon</ansi> spell weaves a shell of living Chrysalis around a
target. It is the strongest ward, and the most costly.

The cocoon turns aside part of every physical blow, every spell and every
harsh word. Its strength and how long it lasts grow with the caster's
willpower and spellcasting skill, and it lasts longer than the lighter wards.

```

Replace in `_datafiles/world/dogmud/templates/help/chrysalis-cocoon.template` (2 of 2):

```text
<ansi fg="yellow">Target:      </ansi> <ansi fg="white-bold">Single ally (or self)</ansi>
<ansi fg="yellow">Defense:     </ansi> <ansi fg="white-bold">None — help spells always apply</ansi>
<ansi fg="yellow">Duration:    </ansi> <ansi fg="white-bold">Scales with spellcasting skill (long)</ansi>
<ansi fg="yellow">Strength:    </ansi> <ansi fg="white-bold">The strongest ward available</ansi>
<ansi fg="yellow">Mitigation:  </ansi> <ansi fg="white-bold">Physical, magical, and conviction damage</ansi>
<ansi fg="yellow">Conv. Cost:  </ansi> <ansi fg="white-bold">8</ansi>
<ansi fg="yellow">School:      </ansi> <ansi fg="white-bold">Enhancement</ansi>

<ansi fg="white-bold">Notes:</ansi>
  - Provides the broadest protection of any shield spell, covering all
    three damage channels.
  - Worth the cost when preparing for a tough fight or a caster enemy.
  - Crits boost the cocoon's strength by roughly half again.
  - Casting again while a cocoon is active overwrites the existing one.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help cast</ansi>, <ansi fg="command">help spells</ansi>, <ansi fg="command">help conviction-ward</ansi>

```

with:

```text
<ansi fg="yellow">Target:      </ansi> <ansi fg="white-bold">Single ally (or self)</ansi>
<ansi fg="yellow">Defense:     </ansi> <ansi fg="white-bold">None, help spells always apply</ansi>
<ansi fg="yellow">Duration:    </ansi> <ansi fg="white-bold">Scales with spellcasting skill (long)</ansi>
<ansi fg="yellow">Strength:    </ansi> <ansi fg="white-bold">The strongest of the three wards</ansi>
<ansi fg="yellow">Blocks:      </ansi> <ansi fg="white-bold">Physical damage, spells and harsh words</ansi>
<ansi fg="yellow">Conv. Cost:  </ansi> <ansi fg="white-bold">60</ansi>
<ansi fg="yellow">School:      </ansi> <ansi fg="white-bold">Enhancement</ansi>

<ansi fg="white-bold">Notes:</ansi>
  - A target holds one ward at a time. A new ward replaces the old one.
  - Potions, gear and mutations that protect you still add on top.
  - Worth the cost before a hard fight or against a caster.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help wards</ansi>, <ansi fg="command">help conviction-ward</ansi>, <ansi fg="command">help conviction-bulwark</ansi>

```

Create `_datafiles/world/dogmud/templates/help/wards.template`:

```text
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">wards</ansi>

A ward is a spell that turns aside part of the damage that reaches its
target. Each ward blocks certain kinds of damage and no others.

  <ansi fg="command">Conviction Ward</ansi>     Physical blows. The weakest and cheapest.
  <ansi fg="command">Conviction Bulwark</ansi>  Physical blows and spells.
  <ansi fg="command">Chrysalis Cocoon</ansi>    Physical blows, spells and harsh words. The strongest.

A target holds one ward at a time. Casting a different ward replaces the one
already there, and the target is told which ward gave way. Casting the same
ward again renews it.

Wards do not cancel other protection. Potions, armor, gear and mutations
that guard you still add their protection on top of a ward.

Type <ansi fg="command">conditions</ansi> to see which ward is on you.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help conviction-ward</ansi>, <ansi fg="command">help conviction-bulwark</ansi>,
          <ansi fg="command">help chrysalis-cocoon</ansi>, <ansi fg="command">help conditions</ansi>
```

Replace in `_datafiles/world/dogmud/keywords.yaml` (1 of 1):

```yaml
  darkness:         [umbral, pall]
  seasons:          [season, winter, summer, spring, autumn, calendar]
```

with:

```yaml
  darkness:         [umbral, pall]
  wards:            [ward, warding, shield spell, shield spells]
  seasons:          [season, winter, summer, spring, autumn, calendar]
```

- [ ] **Step 6: Rename the constant, land the spell's ward, and read the ward family in the AI.**

Replace in `internal/conditions/ids.go` (1 of 1):

```go
	ConditionIdRecovering        = 118 // prone recovery penalty, one round
	ConditionIdMinorShield       = 119
	ConditionIdRegenerating      = 120
```

with:

```go
	ConditionIdRecovering        = 118 // prone recovery penalty, one round
	ConditionIdConvictionWard    = 119 // the weakest ward, and the shield fallback (messaging M6 slice 1)
	ConditionIdRegenerating      = 120
```

Replace in `internal/conditions/test_helpers.go` (1 of 1):

```go
		{ConditionId: ConditionIdRecovering, Name: "Recovering", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 1, Flags: []Flag{Quiet}, Effects: map[EffectKind]EffectValue{EffectAttacksCap: {Literal: 1}}},
		{ConditionId: ConditionIdMinorShield, Name: "Minor Shield", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: mag}, EndUserText: "Your Minor Shield dissipates.", EndRoomText: "{actee}'s Minor Shield dissipates."},
		{ConditionId: ConditionIdRegenerating, Name: "Regenerating", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectRegenMult: mag}, EndUserText: "The healing magic in your wounds runs its course."},
```

with:

```go
		{ConditionId: ConditionIdRecovering, Name: "Recovering", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 1, Flags: []Flag{Quiet}, Effects: map[EffectKind]EffectValue{EffectAttacksCap: {Literal: 1}}},
		{ConditionId: ConditionIdConvictionWard, Name: "Conviction Ward", Family: FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Effects: map[EffectKind]EffectValue{EffectMitigationFlat: mag}, StartActorText: "Your conviction hardens into a ward around {actee}.", StartUserText: "A ward of hardened conviction settles around you.", StartRoomText: "A faint ward of conviction shimmers around {actee}.", EndUserText: "Your Conviction Ward fades.", EndRoomText: "The ward around {actee} fades."},
		{ConditionId: ConditionIdRegenerating, Name: "Regenerating", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10, Flags: []Flag{SilentStart}, Effects: map[EffectKind]EffectValue{EffectRegenMult: mag}, EndUserText: "The healing magic in your wounds runs its course."},
```

Replace in `internal/hooks/spell_help_effects.go` (1 of 5):

```go
	"github.com/GoMudEngine/GoMud/internal/rooms"
)
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
)
```

Replace in `internal/hooks/spell_help_effects.go` (2 of 5):

```go
	for _, conditionId := range c.spell.ConditionIds {
		spec := conditions.GetConditionSpec(conditionId)
		if spec == nil || c.targetChar().HasCondition(conditionId) || !spec.NarratesCastStart(c.selfCast()) {
			return false
		}
	}
	return true
}
```

with:

```go
	for _, conditionId := range c.spell.ConditionIds {
		if !spellConditionNarratesStart(c, conditionId) {
			return false
		}
	}
	return true
}

// spellConditionNarratesStart is spellConditionsNarrateStart for one
// condition: a fresh landing whose own start lines tell every audience.
func spellConditionNarratesStart(c spellEffectCtx, conditionId int) bool {
	spec := conditions.GetConditionSpec(conditionId)
	return spec != nil && !c.targetChar().HasCondition(conditionId) && spec.NarratesCastStart(c.selfCast())
}
```

Replace in `internal/hooks/spell_help_effects.go` (3 of 5):

```go

// applySpellShield is the one shield applier (slice 3b): a Minor Shield
// record on the target worth a third of the caster's primarystat plus its
// weighted cast skill (at least one), scaled by the spell's magnitude (100
// is 1x), for the full universal spell duration. Every pairing gains it: a
// shield on a charmed pet, or from a creature onto a player, applied
// nothing (audit rows 3 and 15). The dead player-to-player crit bump is
// gone (owner ruling 3).
func applySpellShield(c spellEffectCtx) int {
```

with:

```go

// applySpellShield is the one shield applier (slice 3b): the spell's own
// ward (spellWardConditionId) on the target, worth a third of the caster's
// primarystat plus its weighted cast skill (at least one), scaled by the
// spell's magnitude (100 is 1x), for the full universal spell duration. That
// one strength feeds every kind of damage the ward blocks (messaging M6
// slice 1, owner ruling R7). The ward travels the condition event, which
// replaces any other ward the target holds and names the caster; when the
// ward's own start lines tell every audience, they are the only lines (owner
// ruling R11), and a re-cast of a ward already held keeps the lines below.
// Every pairing gains it: a shield on a charmed pet, or from a creature onto
// a player, applied nothing (audit rows 3 and 15). The dead player-to-player
// crit bump is gone (owner ruling 3).
func applySpellShield(c spellEffectCtx) int {
```

Replace in `internal/hooks/spell_help_effects.go` (4 of 5):

```go
	_, targetHidden := c.hiddenFromRoom()
	_ = c.targetChar().AddConditionMagnitude(conditions.ConditionIdMinorShield, duration, float64(shieldBonus), "spell")
	if c.selfCast() {
```

with:

```go
	_, targetHidden := c.hiddenFromRoom()
	wardId := spellWardConditionId(c.spell)
	narrated := spellConditionNarratesStart(c, wardId)
	if target := spellConditionTargetOf(c.target); target != nil {
		target.QueueCondition(events.Condition{ConditionId: wardId, Source: "spell",
			Triggers: duration, Magnitude: float64(shieldBonus), Caster: c.casterRef()})
	}
	if narrated {
		return 0
	}
	if c.selfCast() {
```

Replace in `internal/hooks/spell_help_effects.go` (5 of 5):

```go
			`A shimmering barrier surrounds %s.`, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// applySpellPurge is the one purge applier (slice 3b): it cancels every
```

with:

```go
			`A shimmering barrier surrounds %s.`, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// spellWardConditionId is the ward a shield spell lands: its one
// condition_ids entry, a ward-family record (the root guard
// TestShieldAndHealSpellsLandTheirOwnCondition holds the shipped spells to
// that), or Conviction Ward for a spell that names none.
func spellWardConditionId(spell *spells.SpellData) int {
	if len(spell.ConditionIds) > 0 {
		return spell.ConditionIds[0]
	}
	return conditions.ConditionIdConvictionWard
}

// applySpellPurge is the one purge applier (slice 3b): it cancels every
```

Replace in `internal/behaviortree/action_cast_best_in_category.go` (1 of 1):

```go
// grant is already on the character. Branches:
//   - spell.ConditionIds non-empty: skip if any is active (HasCondition)
//   - spell.EffectType == "shield": skip if the Minor Shield record (condition
//     119, see _datafiles/world/dogmud/conditions/119-minor_shield.yaml) is
//     already granting mitigation. Spell resolution lands shield-type
//     casts via AddConditionMagnitude(ConditionIdMinorShield, ...) — checked here via
//     the Conditions.HasEffect(EffectMitigationFlat) door, NOT
//     Character.HasShield(), which checks for equipped shield items or
//     species natural-bash, neither of which is what this spell grants.
//
// If neither mechanism matches, returns false (conservative — may recast but
// won't silently stall the tree).
func spellEffectAlreadyActive(char *characters.Character, sd *spells.SpellData) bool {
	for _, bid := range sd.ConditionIds {
		if char.HasCondition(bid) {
			return true
		}
	}
	if sd.EffectType == "shield" && char.Conditions.HasEffect(conditions.EffectMitigationFlat) {
		return true
	}
	return false
```

with:

```go
// grant is already on the character. Branches:
//   - spell.EffectType == "shield": skip if any ward is active, the ward
//     family (conditions.FamilyWard; messaging M6 slice 1). Each shield spell
//     lands its own ward and a new ward replaces the old, so a second ward
//     would only swap one for another.
//   - spell.EffectType == "heal": skip if any heal is active, the heal
//     family, for the same reason.
//   - spell.ConditionIds non-empty: skip if any is active (HasCondition)
//
// If no branch matches, returns false (conservative: may recast but won't
// silently stall the tree).
func spellEffectAlreadyActive(char *characters.Character, sd *spells.SpellData) bool {
	switch sd.EffectType {
	case "shield":
		return char.Conditions.HasFamily(conditions.FamilyWard)
	case "heal":
		return char.Conditions.HasFamily(conditions.FamilyHeal)
	}
	for _, bid := range sd.ConditionIds {
		if char.HasCondition(bid) {
			return true
		}
	}
	return false
```

Replace in `internal/combat/ai.go` (1 of 1):

```go
// preferredSpell returns the spell ID the mob should cast this round.
// Priority: (1) minor-shield if unshielded, (2) heal-self if < 30% HP, (3) harm spells.
func preferredSpell(mob *mobs.Mob) string {
	// Shield self if not already shielded
	if !mob.Character.Conditions.HasEffect(conditions.EffectMitigationFlat) {
		if _, has := mob.Character.SpellBook["conviction-ward"]; has {
```

with:

```go
// preferredSpell returns the spell ID the mob should cast this round.
// Priority: (1) conviction-ward if no ward is up, (2) heal-self if < 30% HP, (3) harm spells.
func preferredSpell(mob *mobs.Mob) string {
	// Ward self if no ward is up (the ward family, messaging M6 slice 1)
	if !mob.Character.Conditions.HasFamily(conditions.FamilyWard) {
		if _, has := mob.Character.SpellBook["conviction-ward"]; has {
```

- [ ] **Step 7: Follow the rename and the new lines in older tests.**

Replace in `internal/conditions/effects_test.go` (1 of 1):

```go
		ConditionIdWarcry, ConditionIdRally, ConditionIdOffBalance, ConditionIdRecovering,
		ConditionIdMinorShield, ConditionIdRegenerating, ConditionIdPoisoned, ConditionIdBleeding,
		ConditionIdEnchantWithdrawal,
```

with:

```go
		ConditionIdWarcry, ConditionIdRally, ConditionIdOffBalance, ConditionIdRecovering,
		ConditionIdConvictionWard, ConditionIdRegenerating, ConditionIdPoisoned, ConditionIdBleeding,
		ConditionIdEnchantWithdrawal,
```

Replace in `internal/conditions/records_test.go` (1 of 1):

```go
		ConditionIdWarcry, ConditionIdRally, ConditionIdOffBalance, ConditionIdRecovering,
		ConditionIdMinorShield, ConditionIdRegenerating, ConditionIdPoisoned, ConditionIdBleeding,
		ConditionIdEnchantWithdrawal,
```

with:

```go
		ConditionIdWarcry, ConditionIdRally, ConditionIdOffBalance, ConditionIdRecovering,
		ConditionIdConvictionWard, ConditionIdRegenerating, ConditionIdPoisoned, ConditionIdBleeding,
		ConditionIdEnchantWithdrawal,
```

Replace in `internal/characters/conditions_pin_test.go` (1 of 1):

```go
	before := c.GetPhysicalMitigation()
	_ = c.AddConditionMagnitude(conditions.ConditionIdMinorShield, 10, 12, "pin") // SETUP
	c.Conditions.Validate(true)
```

with:

```go
	before := c.GetPhysicalMitigation()
	_ = c.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 10, 12, "pin") // SETUP
	c.Conditions.Validate(true)
```

Replace in `internal/combat/combat_helpers_test.go` (1 of 2):

```go

	// The Minor Shield record's mitigation_flat effect feeds
	// GetPhysicalMitigation()'s non-gear term, so it sets mitigation without
```

with:

```go

	// The Conviction Ward record's mitigation_flat effect feeds
	// GetPhysicalMitigation()'s non-gear term, so it sets mitigation without
```

Replace in `internal/combat/combat_helpers_test.go` (2 of 2):

```go
		if mitigationPct > 0 {
			_ = c.AddConditionMagnitude(conditions.ConditionIdMinorShield, 100, float64(mitigationPct), "test")
		}
```

with:

```go
		if mitigationPct > 0 {
			_ = c.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 100, float64(mitigationPct), "test")
		}
```

Replace in `internal/behaviortree/pure_caster_archetype_integration_test.go` (1 of 3):

```go
	m.Character.Conviction = 500
	// Base, not just Value: the two tests below that apply Minor Shield via
	// AddConditionMagnitude trigger a full Character.Validate(), which recomputes
```

with:

```go
	m.Character.Conviction = 500
	// Base, not just Value: the two tests below that apply Conviction Ward via
	// AddConditionMagnitude trigger a full Character.Validate(), which recomputes
```

Replace in `internal/behaviortree/pure_caster_archetype_integration_test.go` (2 of 3):

```go

	// Activate iron-will (condition 27) and conviction-ward (the Minor Shield
	// record). seedConditionOnChar replaces the whole spec map with just {27}, so
	// SeedConditionRecordsForTest must run AFTER it to add Minor Shield's
	// spec back in (additive) before AddConditionMagnitude needs it.
	defer seedConditionOnChar(t, &mob.Character, 27)()
	defer conditions.SeedConditionRecordsForTest()()
	_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdMinorShield, 20, 75, "test")
	// AddConditionMagnitude validates the embedded Character directly, which
```

with:

```go

	// Activate iron-will (condition 27) and conviction-ward (the Conviction Ward
	// record). seedConditionOnChar replaces the whole spec map with just {27}, so
	// SeedConditionRecordsForTest must run AFTER it to add Conviction Ward's
	// spec back in (additive) before AddConditionMagnitude needs it.
	defer seedConditionOnChar(t, &mob.Character, 27)()
	defer conditions.SeedConditionRecordsForTest()()
	_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 20, 75, "test")
	// AddConditionMagnitude validates the embedded Character directly, which
```

Replace in `internal/behaviortree/pure_caster_archetype_integration_test.go` (3 of 3):

```go
	// seedConditionOnChar replaces the whole spec map with just {27}, so
	// SeedConditionRecordsForTest must run AFTER it to add Minor Shield's
	// spec back in (additive) before AddConditionMagnitude needs it.
	defer seedConditionOnChar(t, &mob.Character, 27)()
	defer conditions.SeedConditionRecordsForTest()()
	_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdMinorShield, 20, 75, "test")
	// AddConditionMagnitude validates the embedded Character directly, which
```

with:

```go
	// seedConditionOnChar replaces the whole spec map with just {27}, so
	// SeedConditionRecordsForTest must run AFTER it to add Conviction Ward's
	// spec back in (additive) before AddConditionMagnitude needs it.
	defer seedConditionOnChar(t, &mob.Character, 27)()
	defer conditions.SeedConditionRecordsForTest()()
	_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 20, 75, "test")
	// AddConditionMagnitude validates the embedded Character directly, which
```

Replace in `internal/hooks/condition_notice_test.go` (1 of 1):

```go
// a future caller (a spell or item) that wants the start notice through the
// queue instead. Minor Shield is silent-start, so no start line is expected;
// this only pins that the exact trigger count and the magnitude-derived
// effect both survive the trip through ApplyConditions.
func TestConditionNotice_MagnitudeEventAppliesSilently(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	assert.Equal(t, events.Continue, ApplyConditions(events.Condition{UserId: 1, ConditionId: conditions.ConditionIdMinorShield, Triggers: 7, Magnitude: 9, Source: "test"}))

	holder := users.GetByUserId(1)
	require.NotNil(t, holder)
	assert.Equal(t, 7, holder.Character.Conditions.TriggersLeft(conditions.ConditionIdMinorShield))
	assert.Equal(t, float64(9), holder.Character.Conditions.Effect(conditions.EffectMitigationFlat))
```

with:

```go
// a future caller (a spell or item) that wants the start notice through the
// queue instead. This only pins that the exact trigger count and the
// magnitude-derived effect both survive the trip through ApplyConditions.
func TestConditionNotice_MagnitudeEventAppliesSilently(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	assert.Equal(t, events.Continue, ApplyConditions(events.Condition{UserId: 1, ConditionId: conditions.ConditionIdConvictionWard, Triggers: 7, Magnitude: 9, Source: "test"}))

	holder := users.GetByUserId(1)
	require.NotNil(t, holder)
	assert.Equal(t, 7, holder.Character.Conditions.TriggersLeft(conditions.ConditionIdConvictionWard))
	assert.Equal(t, float64(9), holder.Character.Conditions.Effect(conditions.EffectMitigationFlat))
```

Replace in `internal/hooks/death_strip_end_lines_test.go` (1 of 4):

```go
	require.NoError(t, u.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 1, -5, "claws"))
	require.NoError(t, u.Character.AddConditionMagnitude(conditions.ConditionIdMinorShield, 10, 3, "spell"))
	u.Character.Health = 1
```

with:

```go
	require.NoError(t, u.Character.AddConditionMagnitude(conditions.ConditionIdBleeding, 1, -5, "claws"))
	require.NoError(t, u.Character.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 10, 3, "spell"))
	u.Character.Health = 1
```

Replace in `internal/hooks/death_strip_end_lines_test.go` (2 of 4):

```go
	assert.False(t, u.Character.HasCondition(conditions.ConditionIdBleeding), "the stripped bleed is gone after the respawn")
	assert.False(t, u.Character.HasCondition(conditions.ConditionIdMinorShield), "the stripped shield is gone after the respawn")

```

with:

```go
	assert.False(t, u.Character.HasCondition(conditions.ConditionIdBleeding), "the stripped bleed is gone after the respawn")
	assert.False(t, u.Character.HasCondition(conditions.ConditionIdConvictionWard), "the stripped shield is gone after the respawn")

```

Replace in `internal/hooks/death_strip_end_lines_test.go` (3 of 4):

```go
		"the respawned player must not read the stripped bleed's end line")
	assert.Equal(t, 0, countContaining(holderLines, "Minor Shield dissipates"),
		"nor the stripped shield's")
	assert.Equal(t, 0, countContaining(roomLines, "Minor Shield dissipates"),
		"and the room must not see it either")
```

with:

```go
		"the respawned player must not read the stripped bleed's end line")
	assert.Equal(t, 0, countContaining(holderLines, "Conviction Ward fades"),
		"nor the stripped shield's")
	assert.Equal(t, 0, countContaining(roomLines, "ward around"),
		"and the room must not see it either")
```

Replace in `internal/hooks/death_strip_end_lines_test.go` (4 of 4):

```go

	require.NoError(t, u.Character.AddConditionMagnitude(conditions.ConditionIdMinorShield, 10, 3, "spell"))
	u.Character.CancelConditionsWithFlag(conditions.All)
	drainPlain(1)

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(1), "Your Minor Shield dissipates."))
}
```

with:

```go

	require.NoError(t, u.Character.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 10, 3, "spell"))
	u.Character.CancelConditionsWithFlag(conditions.All)
	drainPlain(1)

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(1), "Your Conviction Ward fades."))
}
```

Replace in `internal/hooks/hooks_test.go` (1 of 5):

```go

// ─── Minor Shield record ──────────────────────────────────────────────────────
//
// Minor Shield used to be a combat-condition enum entry, decremented twice
// per round: once by the enum's own tick in the round ticks and again by
```

with:

```go

// ─── Conviction Ward record ──────────────────────────────────────────────────────
//
// Conviction Ward used to be a combat-condition enum entry, decremented twice
// per round: once by the enum's own tick in the round ticks and again by
```

Replace in `internal/hooks/hooks_test.go` (2 of 5):

```go

	_ = u.Character.AddConditionMagnitude(conditions.ConditionIdMinorShield, 1, 10.0, "test")
	assert.Equal(t, 10.0, u.Character.Conditions.Effect(conditions.EffectMitigationFlat),
```

with:

```go

	_ = u.Character.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 1, 10.0, "test")
	assert.Equal(t, 10.0, u.Character.Conditions.Effect(conditions.EffectMitigationFlat),
```

Replace in `internal/hooks/hooks_test.go` (3 of 5):

```go

	assert.False(t, u.Character.HasCondition(conditions.ConditionIdMinorShield),
		"a 1-round shield must be gone after one round tick and one prune pass")
	assert.Equal(t, 1, countContaining(drainPlain(1), "Your Minor Shield dissipates."))
}
```

with:

```go

	assert.False(t, u.Character.HasCondition(conditions.ConditionIdConvictionWard),
		"a 1-round shield must be gone after one round tick and one prune pass")
	assert.Equal(t, 1, countContaining(drainPlain(1), "Your Conviction Ward fades."))
}
```

Replace in `internal/hooks/hooks_test.go` (4 of 5):

```go

	_ = u.Character.AddConditionMagnitude(conditions.ConditionIdMinorShield, 2, 10.0, "test")
	require.Equal(t, 2, u.Character.Conditions.TriggersLeft(conditions.ConditionIdMinorShield))

	UserRoundTick(events.NewRound{RoundNumber: 1})
	DoCombat(events.NewRound{RoundNumber: 1})

	assert.Equal(t, 1, u.Character.Conditions.TriggersLeft(conditions.ConditionIdMinorShield),
		"the shield must decay exactly once per round")
```

with:

```go

	_ = u.Character.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 2, 10.0, "test")
	require.Equal(t, 2, u.Character.Conditions.TriggersLeft(conditions.ConditionIdConvictionWard))

	UserRoundTick(events.NewRound{RoundNumber: 1})
	DoCombat(events.NewRound{RoundNumber: 1})

	assert.Equal(t, 1, u.Character.Conditions.TriggersLeft(conditions.ConditionIdConvictionWard),
		"the shield must decay exactly once per round")
```

Replace in `internal/hooks/hooks_test.go` (5 of 5):

```go
	mob := mobs.GetInstance(100)
	_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdMinorShield, 3, 10.0, "test")

	evt := events.NewRound{RoundNumber: 1}
	MobRoundTick(evt)

	// The record should still exist but its trigger count decremented.
	assert.True(t, mob.Character.HasCondition(conditions.ConditionIdMinorShield))
	assert.Equal(t, 2, mob.Character.Conditions.TriggersLeft(conditions.ConditionIdMinorShield))
}
```

with:

```go
	mob := mobs.GetInstance(100)
	_ = mob.Character.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 3, 10.0, "test")

	evt := events.NewRound{RoundNumber: 1}
	MobRoundTick(evt)

	// The record should still exist but its trigger count decremented.
	assert.True(t, mob.Character.HasCondition(conditions.ConditionIdConvictionWard))
	assert.Equal(t, 2, mob.Character.Conditions.TriggersLeft(conditions.ConditionIdConvictionWard))
}
```

Replace in `internal/hooks/NewRound_DoCombat_parity_test.go` (1 of 3):

```go

// ─── Gap 4: MvP Minor Shield single-application ───────────────────────────────

// TestMvP_ConditionShieldAppliedOnceNotDoubleDipped locks the deletion of
// the inline ConditionShield reduction in handleMobVsPlayer (Gap 4). The
// magnitude is already added inside the mitigation layer
// (GetPhysicalMitigation; the legacy GetDefense path was removed
// 2026-08-03). The deleted block was applying a *second* reduction equal
// to half the magnitude on top of that. Minor Shield is now the condition record
// (ConditionIdMinorShield, Task 6) rather than the enum condition, but the
// mitigation-layer contract this test pins is unchanged.
```

with:

```go

// ─── Gap 4: MvP Conviction Ward single-application ───────────────────────────────

// TestMvP_ConditionShieldAppliedOnceNotDoubleDipped locks the deletion of
// the inline ConditionShield reduction in handleMobVsPlayer (Gap 4). The
// magnitude is already added inside the mitigation layer
// (GetPhysicalMitigation; the legacy GetDefense path was removed
// 2026-08-03). The deleted block was applying a *second* reduction equal
// to half the magnitude on top of that. Conviction Ward is now the condition record
// (ConditionIdConvictionWard, Task 6) rather than the enum condition, but the
// mitigation-layer contract this test pins is unchanged.
```

Replace in `internal/hooks/NewRound_DoCombat_parity_test.go` (2 of 3):

```go

	// Apply the Minor Shield record with magnitude 30 (this is the
	// integer-percent value the spell stores; the magnitude maps 1:1 into
	// the mitigation percentage at characters/combat.go:185).
	const magnitude float64 = 30
	_ = defUser.Character.AddConditionMagnitude(conditions.ConditionIdMinorShield, 10, magnitude, "test")

```

with:

```go

	// Apply the Conviction Ward record with magnitude 30 (this is the
	// integer-percent value the spell stores; the magnitude maps 1:1 into
	// the mitigation percentage at characters/combat.go:185).
	const magnitude float64 = 30
	_ = defUser.Character.AddConditionMagnitude(conditions.ConditionIdConvictionWard, 10, magnitude, "test")

```

Replace in `internal/hooks/NewRound_DoCombat_parity_test.go` (3 of 3):

```go
	// not leak into the condition-derived mitigation number. That premise no
	// longer holds post-Task-6: Minor Shield IS a Conditions record now, so
	// clearing Conditions clears the shield itself. There is nothing left to
```

with:

```go
	// not leak into the condition-derived mitigation number. That premise no
	// longer holds post-Task-6: Conviction Ward IS a Conditions record now, so
	// clearing Conditions clears the shield itself. There is nothing left to
```

Replace in `internal/hooks/hidden_mob_spell_room_lines_test.go` (1 of 4):

```go
	// spell names is reseeded here, and the records a dot, heal and shield
	// apply (121 Poisoned, Regenerating, Minor Shield) are added on top.
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
```

with:

```go
	// spell names is reseeded here, and the records a dot, heal and shield
	// apply (121 Poisoned, Regenerating, Conviction Ward) are added on top.
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
```

Replace in `internal/hooks/hidden_mob_spell_room_lines_test.go` (2 of 4):

```go
		veto bool
		want string
```

with:

```go
		veto bool
		// want empty: the line is the condition's own, which skips a reader
		// who does not perceive the holder (#458).
		want string
```

Replace in `internal/hooks/hidden_mob_spell_room_lines_test.go` (3 of 4):

```go
		{"shield", &spells.SpellData{SpellId: "test-shield", Name: "Ward", EffectType: "shield"},
			false, "A shimmering barrier surrounds something."},
		{"purge", &spells.SpellData{SpellId: "test-purge", Name: "Cleanse", EffectType: "purge"},
```

with:

```go
		{"shield", &spells.SpellData{SpellId: "test-shield", Name: "Ward", EffectType: "shield"},
			false, ""},
		{"purge", &spells.SpellData{SpellId: "test-purge", Name: "Cleanse", EffectType: "purge"},
```

Replace in `internal/hooks/hidden_mob_spell_room_lines_test.go` (4 of 4):

```go
				actions.NewMobActorInRoom(m, room), room, tc.spell, 10, spellContestAttackWin()))

			assert.NotZero(t, countContaining(drainPlain(1), "Skeleton"),
				"the see-hidden caster still reads the mob's name in its own line")
			requireUnnamed(t, drainPlain(2), tc.want)
```

with:

```go
				actions.NewMobActorInRoom(m, room), room, tc.spell, 10, spellContestAttackWin()))
			landQueuedConditions() // a ward or a heal lands through the condition queue

			assert.NotZero(t, countContaining(drainPlain(1), "Skeleton"),
				"the see-hidden caster still reads the mob's name in its own line")
			if tc.want == "" {
				assert.Empty(t, drainPlain(2), "a reader who does not perceive the holder reads no condition line")
				return
			}
			requireUnnamed(t, drainPlain(2), tc.want)
```

Replace in `internal/hooks/spell_effect_fixture_test.go` (1 of 1):

```go
	events.DrainQueuedPlayerAttackedMobsForTest(0)
	return f
```

with:

```go
	events.DrainQueuedPlayerAttackedMobsForTest(0)
	// A ward or a heal lands through the condition queue (messaging M6 slice
	// 1); an earlier test's leftover event must not land in this one.
	events.DrainQueuedMobConditionsForTest(0) // zero drains every holder
	return f
```

Replace in `internal/hooks/spell_help_parity_test.go` (1 of 1):

```go
		{name: "shield", spell: shieldSpellForParityTest,
			roomWord: "shimmering barrier surrounds", selfWord: "shimmering barrier surrounds",
			check: func(t *testing.T, f *spellParityFixture, p spellParityPairing) {
```

with:

```go
		{name: "shield", spell: shieldSpellForParityTest,
			roomWord: "ward of conviction shimmers around", selfWord: "ward of conviction shimmers around",
			check: func(t *testing.T, f *spellParityFixture, p spellParityPairing) {
```

Replace in `internal/hooks/spell_selfcast_test.go` (1 of 1):

```go
		ms.cast(f, shieldSpellForParityTest())

		assert.Equal(t, 1, countContaining(drainPlain(3), "A shimmering barrier surrounds Skeleton"))
		require.Len(t, f.records, 1, "a self-cast is recorded")
```

with:

```go
		ms.cast(f, shieldSpellForParityTest())
		landQueuedConditions()

		assert.Equal(t, 1, countContaining(drainPlain(3), "A faint ward of conviction shimmers around Skeleton"))
		require.Len(t, f.records, 1, "a self-cast is recorded")
```

- [ ] **Step 8: Update the guards and the narration snapshot builder.**

Replace in `condition_apply_path_guard_test.go` (1 of 2):

```go

	// ── former combat condition: Minor Shield is now one record (Task 6;
	// re-keyed messaging M4b-2 Task 7 when spellAttackShape's deletion
	// shrank and shifted every later line in that file; re-keyed again
	// Task 10 when the non-harm-at-a-mob shortcut shifted every later line;
	// re-keyed again Task 10's follow-up when the shortcut's comment grew;
	// re-keyed again counters slice Task 3 when the drain-area counter
	// dispatch loop was deleted; re-keyed again messaging M4d Task 6 when the
	// default case's self-cast line moved onto SendTrio and grew a comment;
	// re-keyed again messaging M4d PR 3 Task 3 when the purge/heal/condition
	// self-cast branches above the shield case moved onto SendTrio;
	// re-keyed again parity slice 2 when the post-queue tick snapshot blocks
	// and the mutations import were deleted; re-keyed again spell effects 3a
	// Task 1 when the arms functions took their context headers and the MP
	// switch moved out of its resolver; re-keyed again spell effects 3a Task 2
	// when the three damage arms moved into applySpellDamage; re-keyed again
	// spell effects 3a Task 4 when the three knockdown arms moved into
	// applySpellKnockdown; re-keyed again spell effects 3a Task 5 when the
	// resolvers' backfire, interrupt and record blocks moved into
	// spell_effects.go; re-keyed again parity slice 3a Task 7 when the
	// one-contest guard began parsing spell_effects.go too; re-keyed again
	// parity slice 3a Task 8 when maybeInterruptSpellOnMob was deleted;
	// re-keyed again parity slice 3b Task 1 when the resolvers' help-spell
	// shortcuts collapsed into resolveHelpSpell, and again parity slice 3b
	// Task 2 when the three condition arms moved into
	// applySpellConditionEffect, and again Task 3 when the heal arms it moved
	// into applySpellHeal shifted every later line in spell_resolution.go;
	// the PP row MOVED into applySpellShield by Task 4, which replaced the
	// player-to-player shield arm and gained PM, MM and MP; the MS row stays
	// in spell_resolution.go, re-keyed for the same deletion's shift, and
	// again Task 5 when the per-pairing arms above it were deleted; the MS
	// row DELETED by Task 6, when applyMobSelfEffect's switch was deleted
	// and a mob's self-cast shield reached applySpellShield's row; re-keyed
	// again Task 7, when spellHelpAreaTargets' mobs, parties and rooms
	// imports shifted spell_help_effects.go; re-keyed again 3b playtest fix,
	// when the party rule moved to actions.HelpCharmAlly and the parties
	// import left spell_help_effects.go; re-keyed again messaging M6 slice 1,
	// when spellConditionsNarrateStart landed above it) ────
	"internal/hooks/spell_help_effects.go|220": "former combat condition (ward): silent-start record, the spell narrates; must apply synchronously so the same resolution pass sees it",

	// ── former combat condition: Regenerating is now one record (Task 7;
```

with:

```go

	// ── former combat condition: Regenerating is now one record (Task 7;
```

Replace in `condition_apply_path_guard_test.go` (2 of 2):

```go
	// Task 7, same import shift as above; re-keyed again 3b playtest fix,
	// same import shift as above; re-keyed again messaging M6 slice 1, same
	// shift as above) ───────
	"internal/hooks/spell_help_effects.go|169": "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
	"internal/mobcommands/consume.go|46":       "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
```

with:

```go
	// Task 7, same import shift as above; re-keyed again 3b playtest fix,
	// same import shift as above; re-keyed again messaging M6 slice 1, when
	// spellConditionsNarrateStart landed above it and again when the shield
	// arm moved to the event door and its Minor Shield row was deleted) ───────
	"internal/hooks/spell_help_effects.go|176": "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
	"internal/mobcommands/consume.go|46":       "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
```

Replace in `shipped_narration_data_guard_test.go` (1 of 1):

```go
	"conditions/116-terrified.yaml":        true,
	"conditions/119-minor_shield.yaml":     true,
	// Lighting plan 5c: the vision spells and the tincture, narrated by the
```

with:

```go
	"conditions/116-terrified.yaml":        true,
	"conditions/119-conviction_ward.yaml":  true,
	// Messaging M6 slice 1: the other two wards, narrated by the same start
	// (narrateConditionStart, through sendConditionStartRoomText) and end
	// (sendConditionEndRoomText) senders as 119, each with the holder's plain
	// name handed to HideNames.
	"conditions/135-conviction_bulwark.yaml": true,
	"conditions/136-chrysalis_cocoon.yaml":   true,
	// Lighting plan 5c: the vision spells and the tincture, narrated by the
```

Replace in `internal/narration/snapshot_test.go` (1 of 1):

```go
			roles := spec.Narrate(ph.p, kindBNoTarget.ActorName, kindBNoTarget.ActorPlainName)
			if roles.Actee != "" {
```

with:

```go
			roles := spec.Narrate(ph.p, kindBNoTarget.ActorName, kindBNoTarget.ActorPlainName)
			// start_actor (messaging M6 slice 1) is the caster's line, which
			// names the holder; only the start phase has one.
			if roles.Actor != "" {
				fmt.Fprintf(&b, "condition|%d|start_actor => %s\n", id, roles.Actor)
			}
			if roles.Actee != "" {
```

- [ ] **Step 9: Regenerate the narration goldens and read the diff.**

Run: `go test ./internal/narration/ -run TestSnapshotStores -update -count=1`
Expected: `ok`.

Run: `git diff --stat internal/narration/testdata/stores/`
Expected: only `conditions.golden` and `spells.golden`. The conditions diff holds only 119 (start lines and the renamed end lines), 135 and 136, and three `start_actor` rows; the spells diff only the two `conviction-bulwark` cast lines.

- [ ] **Step 10: Run everything the wards touch.**

Run: `go test ./internal/conditions/ ./internal/characters/ ./internal/combat/ ./internal/behaviortree/ ./internal/hooks/ ./internal/narration/ ./internal/devtools/ -count=1`
Expected: PASS. `ok` for all seven.

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 11: Commit.**

```bash
git add spell_condition_data_guard_test.go \
  internal/behaviortree/defensive_caster_archetype_integration_test.go \
  internal/hooks/spell_shield_test.go \
  _datafiles/world/dogmud/conditions/119-conviction_ward.yaml \
  _datafiles/world/dogmud/conditions/135-conviction_bulwark.yaml \
  _datafiles/world/dogmud/conditions/136-chrysalis_cocoon.yaml \
  _datafiles/world/dogmud/spells/conviction-bulwark.yaml \
  _datafiles/world/dogmud/spells/conviction-ward.yaml \
  _datafiles/world/dogmud/spells/chrysalis-cocoon.yaml \
  _datafiles/world/dogmud/templates/help/conviction-ward.template \
  _datafiles/world/dogmud/templates/help/conviction-bulwark.template \
  _datafiles/world/dogmud/templates/help/chrysalis-cocoon.template \
  _datafiles/world/dogmud/templates/help/wards.template \
  _datafiles/world/dogmud/keywords.yaml \
  internal/conditions/ids.go \
  internal/conditions/test_helpers.go \
  internal/hooks/spell_help_effects.go \
  internal/behaviortree/action_cast_best_in_category.go \
  internal/combat/ai.go \
  internal/conditions/effects_test.go \
  internal/conditions/records_test.go \
  internal/characters/conditions_pin_test.go \
  internal/combat/combat_helpers_test.go \
  internal/behaviortree/pure_caster_archetype_integration_test.go \
  internal/hooks/condition_notice_test.go \
  internal/hooks/death_strip_end_lines_test.go \
  internal/hooks/hooks_test.go \
  internal/hooks/NewRound_DoCombat_parity_test.go \
  internal/hooks/hidden_mob_spell_room_lines_test.go \
  internal/hooks/spell_effect_fixture_test.go \
  internal/hooks/spell_help_parity_test.go \
  internal/hooks/spell_selfcast_test.go \
  condition_apply_path_guard_test.go \
  shipped_narration_data_guard_test.go \
  internal/narration/snapshot_test.go \
  internal/narration/testdata/stores/conditions.golden \
  internal/narration/testdata/stores/spells.golden
# the rename is already staged by git mv: _datafiles/world/dogmud/conditions/119-minor_shield.yaml
git commit -F - <<'EOF'
feat(spells): three wards, each landing its own condition (#338)

Conviction Ward (119, renamed from Minor Shield) blocks physical damage,
the new Conviction Bulwark (135) physical and spell, Chrysalis Cocoon
(136) all three. Each spell names its ward in condition_ids;
applySpellShield lands it through the condition event, so the ward tells
its own start lines and replaces any other ward. The mob AI waits while
any ward is up. Help pages match the spells, and a root guard holds the
roster.

Refs #338, Refs #370.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 10: Six heals (#338, R8, R9)

Each heal spell lands its own heal-family condition (137 to 142) with `regen_mult: magnitude` for the condition's own duration scaled by the caster (`healTriggers`, D11); Vital Surge and Chrysalis Regeneration join the family. Mass Mend's dead `[33]` goes.

- [ ] **Step 1: Write the failing tests.**

Replace in `spell_condition_data_guard_test.go` (1 of 2):

```go
	"shield": {conditions.FamilyWard, conditions.EffectMitigationFlat},
}

// R3, R4 and R7: each shield spell lands its own condition, one per spell,
// in the ward family, reading the spell's strength from its magnitude.
func TestShieldAndHealSpellsLandTheirOwnCondition(t *testing.T) {
```

with:

```go
	"shield": {conditions.FamilyWard, conditions.EffectMitigationFlat},
	"heal":   {conditions.FamilyHeal, conditions.EffectRegenMult},
}

// R3, R4, R7 and R8: each shield and heal spell lands its own condition,
// one per spell, in the ward or heal family, reading the spell's strength
// (a ward's points, a heal's multiplier) from its magnitude.
func TestShieldAndHealSpellsLandTheirOwnCondition(t *testing.T) {
```

Replace in `spell_condition_data_guard_test.go` (2 of 2):

```go
		prevMag, prevCost = s.EffectMagnitude, s.Cost
	}
}

```

with:

```go
		prevMag, prevCost = s.EffectMagnitude, s.Cost
	}
}

// healIdentity is one heal spell as the player meets it: its multiplier
// (effect_magnitude), its authored duration (its condition's triggercount)
// and whether it lands on everyone (targeting area).
type healIdentity struct {
	mult, rounds int
	area         bool
}

func shippedHeal(t *testing.T, all map[string]*spells.SpellData, conds map[int]*conditions.ConditionSpec, id string) healIdentity {
	t.Helper()
	s := all[id]
	if s == nil || len(s.ConditionIds) != 1 || conds[s.ConditionIds[0]] == nil {
		t.Fatalf("%s: missing, or not landing exactly one shipped condition", id)
	}
	return healIdentity{mult: s.EffectMagnitude, rounds: conds[s.ConditionIds[0]].TriggerCount, area: string(s.Targeting) == "area"}
}

// R9: the heal roster's identities. Mend Flesh is long and gentle, Mend
// Wounds short and strong; Mend All is the short, light area heal,
// Communion of Flesh the long, steady one and Mass Mend the short, strong
// one at the top. Chrysalis Regeneration (33) and Vital Surge (32) are in
// the heal family; Regenerating (120), the mob feeding record, is not.
func TestHealRosterHoldsItsIdentities(t *testing.T) {
	all := shippedSpells(t)
	conds := shippedConditions(t)
	flesh := shippedHeal(t, all, conds, "heal")
	wounds := shippedHeal(t, all, conds, "mend-wounds")
	mendAll := shippedHeal(t, all, conds, "mend-all")
	communion := shippedHeal(t, all, conds, "communion-of-flesh")
	mass := shippedHeal(t, all, conds, "mass-mend")

	if flesh.area || wounds.area || !mendAll.area || !communion.area || !mass.area {
		t.Errorf("targeting: Mend Flesh and Mend Wounds single, the other three area: %+v %+v %+v %+v %+v", flesh, wounds, mendAll, communion, mass)
	}
	if !(flesh.mult < wounds.mult && flesh.rounds > wounds.rounds) {
		t.Errorf("Mend Flesh must be gentler and longer than Mend Wounds: %+v vs %+v", flesh, wounds)
	}
	if !(communion.rounds > mendAll.rounds && communion.rounds > mass.rounds) {
		t.Errorf("Communion of Flesh must be the longest area heal: %+v vs %+v, %+v", communion, mendAll, mass)
	}
	if !(mass.mult > communion.mult && mass.mult > mendAll.mult) {
		t.Errorf("Mass Mend must be the strongest area heal: %+v vs %+v, %+v", mass, communion, mendAll)
	}
	total := func(h healIdentity) int { return h.mult * h.rounds }
	if !(total(mendAll) < total(communion) && total(mendAll) < total(mass)) {
		t.Errorf("Mend All must be the lightest area heal in all: %d vs %d, %d", total(mendAll), total(communion), total(mass))
	}
	for _, id := range []int{32, 33} {
		if c := conds[id]; c == nil || c.Family != conditions.FamilyHeal {
			t.Errorf("condition %d must be in the heal family", id)
		}
	}
	if c := conds[conditions.ConditionIdRegenerating]; c == nil || c.Family != "" {
		t.Errorf("condition %d (the feeding record) must stay out of the heal family", conditions.ConditionIdRegenerating)
	}
}

```

Replace in `internal/hooks/spell_heal_test.go` (1 of 4):

```go
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/stretchr/testify/assert"
```

with:

```go
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/stretchr/testify/assert"
```

Replace in `internal/hooks/spell_heal_test.go` (2 of 4):

```go
// parityHealRounds is the heal's duration on the fixture's equalised caster
// (base folds 4, skill 3, willpower 100): half the universal duration,
// floored at six. 4 x (10 + 5 + 1.5) = 66, so 33.
func parityHealRounds() int {
	rounds := calcSpellDuration(4, 3, 100) / 2
	if rounds < 6 {
		rounds = 6
	}
	return rounds
}

// regenRecord returns c's one Regenerating record, or fails the test.
func regenRecord(t *testing.T, c *characters.Character) *conditions.Condition {
	t.Helper()
	recs := c.GetConditions(conditions.ConditionIdRegenerating)
```

with:

```go
// parityHealRounds is the heal's duration on the fixture's equalised caster
// (skill 3, willpower 100) for the Regenerating record a heal spell that
// names no condition lands: its authored 10 triggers scaled by
// (10 + 5 + 1.5) / 15, so 11.
func parityHealRounds() int {
	return healTriggers(conditions.ConditionIdRegenerating, 100, 3)
}

// regenRecord lands the queued heal and returns c's one Regenerating record,
// or fails the test.
func regenRecord(t *testing.T, c *characters.Character) *conditions.Condition {
	t.Helper()
	landQueuedConditions()
	recs := c.GetConditions(conditions.ConditionIdRegenerating)
```

Replace in `internal/hooks/spell_heal_test.go` (3 of 4):

```go
	assert.Equal(t, 3.0, rec.Magnitude)
	assert.Equal(t, parityHealRounds(), rec.TriggersLeft)
	assert.Equal(t, 1, countContaining(drainPlain(2), "Mend envelops you in healing energy."))
```

with:

```go
	assert.Equal(t, 3.0, rec.Magnitude)
	assert.Equal(t, 11, parityHealRounds(), "the fixture's arithmetic")
	assert.Equal(t, parityHealRounds(), rec.TriggersLeft)
	assert.Equal(t, state.ActorRef{MobInstanceId: 100}, rec.Caster, "the heal remembers its caster")
	assert.Equal(t, 1, countContaining(drainPlain(2), "Mend envelops you in healing energy."))
```

Replace in `internal/hooks/spell_heal_test.go` (4 of 4):

```go
	assert.Empty(t, events.DrainQueuedHealedForTest(0), "only a player healing a mob is tended")
}

```

with:

```go
	assert.Empty(t, events.DrainQueuedHealedForTest(0), "only a player healing a mob is tended")
}

const (
	testGentleHealId = 7231 // heal family, long and gentle
	testStrongHealId = 7232 // heal family, short and strong
	testSalveId      = 7233 // no family, a healthrecovery statmod like the Healing Salve
)

// seedTestHeals adds two heals and a salve to the fixture's registry,
// keeping the fixture's condition 100 and the condition records.
func seedTestHeals(t *testing.T) {
	t.Helper()
	mag := conditions.EffectValue{UsesMagnitude: true}
	heal := func(id, triggers int, name, start string) *conditions.ConditionSpec {
		return &conditions.ConditionSpec{ConditionId: id, Name: name, Family: conditions.FamilyHeal,
			RoundInterval: 1, TriggerCount: triggers,
			Effects:        map[conditions.EffectKind]conditions.EffectValue{conditions.EffectRegenMult: mag},
			StartActorText: start + " settles on {actee}.", StartUserText: start + " settles on you.",
			StartRoomText: start + " settles on {actee_plain}.", EndUserText: start + " fades."}
	}
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		100:              conditions.GetConditionSpec(100),
		testGentleHealId: heal(testGentleHealId, 45, "Test Gentle", "A gentle heal"),
		testStrongHealId: heal(testStrongHealId, 15, "Test Strong", "A strong heal"),
		testSalveId: {ConditionId: testSalveId, Name: "Test Salve", RoundInterval: 1, TriggerCount: 100,
			StatMods: statmods.StatMods{"healthrecovery": 3}, StartUserText: "The salve warms.", EndUserText: "The salve cools."},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
}

func healSpellLanding(conditionId, magnitude int) *spells.SpellData {
	s := healSpellForParityTest()
	s.ConditionIds = []int{conditionId}
	s.EffectMagnitude = magnitude
	return s
}

// R8 and R9: a heal spell lands its own heal, at its own multiplier and its
// own duration scaled by the caster, and its lines are the heal's own.
func TestSpellHeal_LandsTheSpellsOwnHeal(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedTestHeals(t)
	spell := healSpellLanding(testGentleHealId, 2)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)
	landQueuedConditions()

	recs := f.targetUser.Character.GetConditions(testGentleHealId)
	require.Len(t, recs, 1)
	assert.Equal(t, 2.0, recs[0].Magnitude)
	assert.Equal(t, 50, recs[0].TriggersLeft, "45 authored rounds x (10 + 5 + 1.5) / 15, rounded")
	assert.False(t, f.targetUser.Character.HasCondition(conditions.ConditionIdRegenerating))
	assert.Equal(t, []string{"A gentle heal settles on Bobrick."}, drainPlain(1))
	assert.Equal(t, []string{"A gentle heal settles on you."}, drainPlain(2))
	assert.Equal(t, []string{"A gentle heal settles on Bobrick."}, drainPlain(3))
}

// R4 and R6: a second heal spell replaces the first, announced; a salve
// stacks with either.
func TestSpellHeal_ASecondHealReplacesTheFirstAndASalveStacks(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedTestHeals(t)
	c := f.targetUser.Character
	require.NoError(t, c.AddCondition(testSalveId, false))
	gentle, strong := healSpellLanding(testGentleHealId, 2), healSpellLanding(testStrongHealId, 6)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, gentle,
		spellAttackSideFor(gentle, f.casterUser.Character, nil), gentle.EffectMagnitude)
	landQueuedConditions()
	drainPlain(2)
	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, strong,
		spellAttackSideFor(strong, f.casterUser.Character, nil), strong.EffectMagnitude)
	landQueuedConditions()

	assert.False(t, c.HasCondition(testGentleHealId))
	assert.True(t, c.HasCondition(testStrongHealId))
	assert.True(t, c.HasCondition(testSalveId), "a salve is no heal spell, so it stays")
	assert.Equal(t, 6.0, c.Conditions.Effect(conditions.EffectRegenMult), "one heal multiplier, never two multiplied")
	assert.Equal(t, []string{"Your Test Gentle fades as Test Strong takes hold.", "A strong heal settles on you."}, drainPlain(2))
}

```

- [ ] **Step 2: Run them to see them fail.**

Run: `go test . -run "TestShieldAndHealSpellsLandTheirOwnCondition|TestHealRosterHoldsItsIdentities" -count=1`
Expected: FAIL: `heal: a heal spell names exactly one condition, has []` (and four more), `mass-mend: condition 33 (Chrysalis Regeneration) is family "", want "heal"`.

Run: `go vet ./internal/hooks/`
Expected: FAIL: `undefined: healTriggers`.

- [ ] **Step 3: Write the heals and give each spell its heal.**

Create `_datafiles/world/dogmud/conditions/137-mend_flesh.yaml`:

```yaml
conditionid: 137
name: Mend Flesh
description: Gentle healing magic knits your wounds closed, slowly but for a
  long while. Another healing spell takes its place rather than adding to it.
# The heal family (messaging M6 slice 1, owner rulings R4 and R6): a new heal
# spell replaces this one instead of stacking. Potions and salves still stack.
family: heal
triggerrate: 1 round
# The duration at an untrained caster of average willpower; the spell scales
# it by its caster (applySpellHeal, healTriggers). Long and gentle (R9).
triggercount: 45
effects:
  regen_mult: magnitude
start_actor: "Your healing magic settles gently into {actee}'s wounds."
start_actee: "Gentle healing magic settles into your wounds."
start_observer: "A soft glow settles over {actee}'s wounds."
end_actee: "The gentle healing in your wounds runs its course."
```

Create `_datafiles/world/dogmud/conditions/138-mend_wounds.yaml`:

```yaml
conditionid: 138
name: Mend Wounds
description: Strong healing magic closes your wounds quickly, but it does not
  last long. Another healing spell takes its place rather than adding to it.
family: heal
triggerrate: 1 round
# The duration at an untrained caster of average willpower; the spell scales
# it by its caster (applySpellHeal, healTriggers). Short and strong (R9).
triggercount: 25
effects:
  regen_mult: magnitude
start_actor: "Your healing magic surges into {actee}'s wounds."
start_actee: "Healing magic surges into your wounds."
start_observer: "A bright glow surges over {actee}'s wounds."
end_actee: "The surge of healing in your wounds runs its course."
```

Create `_datafiles/world/dogmud/conditions/139-mend_all.yaml`:

```yaml
conditionid: 139
name: Mend All
description: A light wash of healing magic eases your wounds for a short
  while. Another healing spell takes its place rather than adding to it.
family: heal
triggerrate: 1 round
# The duration at an untrained caster of average willpower; the spell scales
# it by its caster (applySpellHeal, healTriggers). Short and light (R9).
triggercount: 30
effects:
  regen_mult: magnitude
start_actor: "A light wash of your healing magic eases {actee}'s wounds."
start_actee: "A light wash of healing magic eases your wounds."
start_observer: "A light wash of healing settles over {actee}."
end_actee: "The light wash of healing fades from your wounds."
```

Create `_datafiles/world/dogmud/conditions/140-communion_of_flesh.yaml`:

```yaml
conditionid: 140
name: Communion of Flesh
description: A deep, steady healing hums through your body and keeps mending
  you for a long while. Another healing spell takes its place rather than
  adding to it.
family: heal
triggerrate: 1 round
# The duration at an untrained caster of average willpower; the spell scales
# it by its caster (applySpellHeal, healTriggers). Long and steady (R9).
triggercount: 90
effects:
  regen_mult: magnitude
start_actor: "Your shared healing hums steadily through {actee}."
start_actee: "A deep, steady healing hums through your body."
start_observer: "A steady green glow hums around {actee}."
end_actee: "The steady healing hum in your body fades."
```

Create `_datafiles/world/dogmud/conditions/141-mass_mend.yaml`:

```yaml
conditionid: 141
name: Mass Mend
description: A powerful tide of healing magic closes your wounds fast for a
  short while. Another healing spell takes its place rather than adding to it.
family: heal
triggerrate: 1 round
# The duration at an untrained caster of average willpower; the spell scales
# it by its caster (applySpellHeal, healTriggers). Short and strong, the top
# area heal (R9).
triggercount: 55
effects:
  regen_mult: magnitude
start_actor: "A tide of your healing magic washes over {actee}."
start_actee: "A powerful tide of healing magic washes over you."
start_observer: "A tide of warm green light washes over {actee}."
end_actee: "The tide of healing magic ebbs from your wounds."
```

Create `_datafiles/world/dogmud/conditions/142-arc_weld_repair.yaml`:

```yaml
conditionid: 142
name: Arc-Weld Repair
description: Arc-light is welding your cracked plating back together.
  Another healing effect cast on you takes its place rather than adding to it.
family: heal
triggerrate: 1 round
# The duration at an untrained caster of average willpower; the spell scales
# it by its caster (applySpellHeal, healTriggers). As before slice 1: a
# construct's mending beam, cast only by the Repair Frame.
triggercount: 15
effects:
  regen_mult: magnitude
start_actor: "Your arc-light starts welding {actee}'s plating back together."
start_actee: "Arc-light starts welding your plating back together."
start_observer: "Hard blue-white arc-light welds {actee}'s plating back together."
end_actee: "The arc-light welding your plating sputters out."
```

Replace in `_datafiles/world/dogmud/conditions/32-vital_surge.yaml` (1 of 1):

```yaml
name: Vital Surge
description: Chrysalis energy steadily mends your body over time.
```

with:

```yaml
name: Vital Surge
# The heal family (messaging M6 slice 1, owner ruling R9): any heal spell
# replaces any other.
family: heal
description: Chrysalis energy steadily mends your body over time.
```

Replace in `_datafiles/world/dogmud/conditions/33-chrysalis_regeneration.yaml` (1 of 1):

```yaml
name: Chrysalis Regeneration
description: Deep Chrysalis energy rapidly regenerates your body.
```

with:

```yaml
name: Chrysalis Regeneration
# The heal family (messaging M6 slice 1, owner ruling R9): any heal spell
# replaces any other.
family: heal
description: Deep Chrysalis energy rapidly regenerates your body.
```

Replace in `_datafiles/world/dogmud/spells/heal.yaml` (1 of 1):

```yaml
effect_type: heal
effect_magnitude: 3
categories:
```

with:

```yaml
effect_type: heal
effect_magnitude: 2
# The heal this spell lands, its multiplier on the holder's natural healing
# (effect_magnitude) and its own duration (the condition's triggercount),
# per the slice 1 identity table (messaging M6 slice 1, owner ruling R9):
# long and gentle.
condition_ids:
  - 137
categories:
```

Replace in `_datafiles/world/dogmud/spells/mend-wounds.yaml` (1 of 1):

```yaml
effect_type: heal
effect_magnitude: 5
categories:
```

with:

```yaml
effect_type: heal
effect_magnitude: 6
# The heal this spell lands, its multiplier on the holder's natural healing
# (effect_magnitude) and its own duration (the condition's triggercount),
# per the slice 1 identity table (messaging M6 slice 1, owner ruling R9):
# short and strong.
condition_ids:
  - 138
categories:
```

Replace in `_datafiles/world/dogmud/spells/mend-all.yaml` (1 of 1):

```yaml
effect_magnitude: 3
cast_actor: "You radiate Chrysalis healing energy outward to all nearby allies."
```

with:

```yaml
effect_magnitude: 3
# The heal this spell lands, its multiplier on the holder's natural healing
# (effect_magnitude) and its own duration (the condition's triggercount),
# per the slice 1 identity table (messaging M6 slice 1, owner ruling R9):
# short and light.
condition_ids:
  - 139
cast_actor: "You radiate Chrysalis healing energy outward to all nearby allies."
```

Replace in `_datafiles/world/dogmud/spells/communion-of-flesh.yaml` (1 of 1):

```yaml
effect_magnitude: 4
cast_actor: "You open yourself to the Chrysalis, sharing your life force with all nearby."
```

with:

```yaml
effect_magnitude: 4
# The heal this spell lands, its multiplier on the holder's natural healing
# (effect_magnitude) and its own duration (the condition's triggercount),
# per the slice 1 identity table (messaging M6 slice 1, owner ruling R9):
# long and steady.
condition_ids:
  - 140
cast_actor: "You open yourself to the Chrysalis, sharing your life force with all nearby."
```

Replace in `_datafiles/world/dogmud/spells/mass-mend.yaml` (1 of 2):

```yaml
aliases: [massmend]
description: Heals all nearby allies and grants sustained regeneration.
attack_type: none
```

with:

```yaml
aliases: [massmend]
description: Washes all nearby allies in a strong, short tide of healing.
attack_type: none
```

Replace in `_datafiles/world/dogmud/spells/mass-mend.yaml` (2 of 2):

```yaml
effect_type: heal
effect_magnitude: 5
condition_ids:
  - 33
cast_actor: "You gather deep Chrysalis energy and begin radiating it outward."
```

with:

```yaml
effect_type: heal
effect_magnitude: 8
# The heal this spell lands, its multiplier on the holder's natural healing
# (effect_magnitude) and its own duration (the condition's triggercount),
# per the slice 1 identity table (messaging M6 slice 1, owner ruling R9):
# short and strong, the top area heal.
condition_ids:
  - 141
cast_actor: "You gather deep Chrysalis energy and begin radiating it outward."
```

Replace in `_datafiles/world/dogmud/spells/repair-pulse.yaml` (1 of 1):

```yaml
effect_magnitude: 8
cast_actor: "Arc-light gathers between your manipulators."
```

with:

```yaml
effect_magnitude: 8
# The heal this spell lands, its multiplier on the holder's natural healing
# (effect_magnitude) and its own duration (the condition's triggercount),
# per the slice 1 identity table (messaging M6 slice 1, owner ruling R9):
# as before slice 1.
condition_ids:
  - 142
cast_actor: "Arc-light gathers between your manipulators."
```

- [ ] **Step 4: Write the help pages and the alias.**

Replace in `_datafiles/world/dogmud/templates/help/heal.template` (1 of 2):

```text

The <ansi fg="command">Mend Flesh</ansi> spell weaves restorative energy through a target's body,
mending wounds and restoring vitality. Cast it on yourself or a nearby ally.
The healing resolves the moment folds are completed — there is no delay.

Healing power scales with your willpower and spellcasting skill. A critical
success (unusually high opposed roll) roughly doubles the amount healed.

```

with:

```text

The <ansi fg="command">Mend Flesh</ansi> spell settles gentle healing magic into a target's
wounds. It does not heal at once: the target heals faster than normal for a
long while, a little at a time. Cast it on yourself or a nearby ally.

It is the gentlest and longest of the single target heals. How long it lasts
grows with your willpower and spellcasting skill.

```

Replace in `_datafiles/world/dogmud/templates/help/heal.template` (2 of 2):

```text
<ansi fg="yellow">Target:      </ansi> <ansi fg="white-bold">Single ally (or self)</ansi>
<ansi fg="yellow">Defense:     </ansi> <ansi fg="white-bold">None — help spells always apply</ansi>
<ansi fg="yellow">Conv. Cost:  </ansi> <ansi fg="white-bold">25</ansi>
<ansi fg="yellow">School:      </ansi> <ansi fg="white-bold">Vital</ansi>

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help cast</ansi>, <ansi fg="command">help spells</ansi>, <ansi fg="command">help conviction-ward</ansi>

```

with:

```text
<ansi fg="yellow">Target:      </ansi> <ansi fg="white-bold">Single ally (or self)</ansi>
<ansi fg="yellow">Defense:     </ansi> <ansi fg="white-bold">None, help spells always apply</ansi>
<ansi fg="yellow">Healing:     </ansi> <ansi fg="white-bold">Gentle, over a long time</ansi>
<ansi fg="yellow">Conv. Cost:  </ansi> <ansi fg="white-bold">25</ansi>
<ansi fg="yellow">School:      </ansi> <ansi fg="white-bold">Vital</ansi>

<ansi fg="white-bold">Notes:</ansi>
  - A target holds one healing spell at a time. A new one replaces the old.
  - Salves, potions and food that heal you still add on top.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help healing-spells</ansi>, <ansi fg="command">help mend-wounds</ansi>, <ansi fg="command">help cast</ansi>

```

Replace in `_datafiles/world/dogmud/templates/help/mend-wounds.template` (1 of 2):

```text

The <ansi fg="command">Mend Wounds</ansi> spell concentrates Chrysalis healing energy into a
single target, rapidly closing wounds and restoring vitality. The
fundamental single-target healing spell — efficient, reliable, and
the first healing art many casters master.

```

with:

```text

The <ansi fg="command">Mend Wounds</ansi> spell surges strong healing magic into a single
target. The target heals much faster than normal, but only for a short
while. Use it when a wound needs closing now.

How long it lasts grows with your willpower and spellcasting skill.

```

Replace in `_datafiles/world/dogmud/templates/help/mend-wounds.template` (2 of 2):

```text
  <ansi fg="yellow">Conv. Cost:  </ansi> 40
  <ansi fg="yellow">Effect:      </ansi> Restores health to a single target

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help spells</ansi>, <ansi fg="command">help cast</ansi>

```

with:

```text
  <ansi fg="yellow">Conv. Cost:  </ansi> 40
  <ansi fg="yellow">Healing:     </ansi> Strong, over a short time

<ansi fg="white-bold">Notes:</ansi>
  - A target holds one healing spell at a time. A new one replaces the old.
  - Salves, potions and food that heal you still add on top.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help healing-spells</ansi>, <ansi fg="command">help heal</ansi>, <ansi fg="command">help cast</ansi>

```

Replace in `_datafiles/world/dogmud/templates/help/mend-all.template` (1 of 2):

```text

The <ansi fg="command">Mend All</ansi> spell radiates gentle healing energy outward, mending the
wounds of every ally present in the room. A cornerstone of group
support, learned from those who have devoted themselves to the healing
arts at sacred sanctuaries.

```

with:

```text

The <ansi fg="command">Mend All</ansi> spell sends a light wash of healing over every ally in
the room. Each of them heals a little faster for a short while. It is the
lightest of the group heals, and the cheapest.

```

Replace in `_datafiles/world/dogmud/templates/help/mend-all.template` (2 of 2):

```text
  <ansi fg="yellow">Conv. Cost:  </ansi> 50
  <ansi fg="yellow">Effect:      </ansi> Heals all allies in the room

<ansi fg="white-bold">Notes:</ansi>
  - Also costs a small amount of the caster's own health to cast.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help spells</ansi>, <ansi fg="command">help cast</ansi>

```

with:

```text
  <ansi fg="yellow">Conv. Cost:  </ansi> 50
  <ansi fg="yellow">Healing:     </ansi> Light, over a short time, for all allies here

<ansi fg="white-bold">Notes:</ansi>
  - Also costs a small amount of the caster's own health to cast.
  - Each ally holds one healing spell at a time. A new one replaces the old.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help healing-spells</ansi>, <ansi fg="command">help cast</ansi>

```

Replace in `_datafiles/world/dogmud/templates/help/communion-of-flesh.template` (1 of 2):

```text

The <ansi fg="command">Communion of Flesh</ansi> spell radiates deep Chrysalis healing energy
outward, mending the wounds of all nearby allies simultaneously. The
caster draws on their own vitality to fuel this powerful area heal —
the spell costs a small measure of the caster's own health.

```

with:

```text

The <ansi fg="command">Communion of Flesh</ansi> spell shares the caster's life force with
every ally in the room. Each of them heals steadily for a long while. The
caster pays for it with a small measure of their own health.

```

Replace in `_datafiles/world/dogmud/templates/help/communion-of-flesh.template` (2 of 2):

```text
  <ansi fg="yellow">Conv. Cost:  </ansi> 100
  <ansi fg="yellow">Effect:      </ansi> Heals all allies in the room

<ansi fg="white-bold">Notes:</ansi>
  - This spell also costs a small amount of the caster's own health.
  - Requires some difficulty to cast — spellcasting skill matters.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help spells</ansi>, <ansi fg="command">help cast</ansi>

```

with:

```text
  <ansi fg="yellow">Conv. Cost:  </ansi> 100
  <ansi fg="yellow">Healing:     </ansi> Steady, over a long time, for all allies here

<ansi fg="white-bold">Notes:</ansi>
  - This spell also costs a small amount of the caster's own health.
  - Each ally holds one healing spell at a time. A new one replaces the old.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help healing-spells</ansi>, <ansi fg="command">help cast</ansi>

```

Replace in `_datafiles/world/dogmud/templates/help/mass-mend.template` (1 of 2):

```text

The <ansi fg="command">Mass Mend</ansi> spell radiates Chrysalis healing energy outward, mending
the wounds of every ally in the room and leaving each of them with a
sustained regeneration effect that continues to restore health over
time. The caster draws on their own vitality to fuel this potent heal.

```

with:

```text

The <ansi fg="command">Mass Mend</ansi> spell sends a strong tide of healing over every ally in
the room. Each of them heals much faster for a short while. It is the
strongest of the group heals. The caster pays for it with a portion of
their own health.

```

Replace in `_datafiles/world/dogmud/templates/help/mass-mend.template` (2 of 2):

```text
  <ansi fg="yellow">Conv. Cost:  </ansi> 120
  <ansi fg="yellow">Effect:      </ansi> Heals all allies in the room, then grants regeneration

<ansi fg="white-bold">Notes:</ansi>
  - Also costs a portion of the caster's own health to cast.
  - Combines immediate healing with a lasting regeneration condition.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help spells</ansi>, <ansi fg="command">help cast</ansi>

```

with:

```text
  <ansi fg="yellow">Conv. Cost:  </ansi> 120
  <ansi fg="yellow">Healing:     </ansi> Strong, over a short time, for all allies here

<ansi fg="white-bold">Notes:</ansi>
  - Also costs a portion of the caster's own health to cast.
  - Each ally holds one healing spell at a time. A new one replaces the old.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help healing-spells</ansi>, <ansi fg="command">help cast</ansi>

```

Create `_datafiles/world/dogmud/templates/help/healing-spells.template`:

```text
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">healing-spells</ansi>

A healing spell does not close wounds at once. It makes its target heal
faster than normal for a while, in battle as well as at rest. Each healing
spell has its own pace.

  <ansi fg="command">Mend Flesh</ansi>          One ally. Gentle, and lasts a long time.
  <ansi fg="command">Mend Wounds</ansi>         One ally. Strong, but over quickly.
  <ansi fg="command">Mend All</ansi>            Everyone here. Light, and over quickly.
  <ansi fg="command">Communion of Flesh</ansi>  Everyone here. Steady, and lasts a long time.
  <ansi fg="command">Mass Mend</ansi>           Everyone here. Strong, but over quickly.

Vital Surge and Chrysalis Regeneration are healing spells too.

A target holds one healing spell at a time. Casting a different one
replaces the one already there, and the target is told which gave way.
Casting the same one again renews it.

Healing spells do not cancel other healing. Salves, potions, food and
mutations that help you heal still add on top.

Type <ansi fg="command">conditions</ansi> to see which healing spell is on you.

<ansi fg="magenta-bold">See also:</ansi> <ansi fg="command">help heal</ansi>, <ansi fg="command">help mend-wounds</ansi>, <ansi fg="command">help mend-all</ansi>,
          <ansi fg="command">help communion-of-flesh</ansi>, <ansi fg="command">help mass-mend</ansi>
```

Replace in `_datafiles/world/dogmud/keywords.yaml` (1 of 1):

```yaml
  wards:            [ward, warding, shield spell, shield spells]
  seasons:          [season, winter, summer, spring, autumn, calendar]
```

with:

```yaml
  wards:            [ward, warding, shield spell, shield spells]
  healing-spells:   [heals, heal spells, healing spell]
  seasons:          [season, winter, summer, spring, autumn, calendar]
```

- [ ] **Step 5: Land the spell's heal.**

Replace in `internal/hooks/spell_help_effects.go` (1 of 3):

```go

// applySpellHeal is the one heal applier (slice 3b): a Regenerating record
// on the target at the spell's magnitude as a regen multiplier (floored at
// 1x) for half the universal spell duration (floored at six rounds), read
// from the caster's primarystat and cast skill. Every pairing gains it; a
// mob's heal on a player used to apply nothing (audit row 3). The dead
// player-to-player crit boost is gone (owner ruling 3).
//
```

with:

```go

// applySpellHeal is the one heal applier (slice 3b): the spell's own heal
// (spellHealConditionId) on the target at the spell's magnitude as a regen
// multiplier (floored at 1x), for the heal's own duration scaled by the
// caster (healTriggers). Each heal spell has its own multiplier and duration
// (messaging M6 slice 1, owner rulings R8 and R9). The heal travels the
// condition event, which replaces any other heal the target holds and names
// the caster; when the heal's own start lines tell every audience, they are
// the only lines (owner ruling R11), and a re-cast of a heal already held
// keeps the lines below. Every pairing gains it; a mob's heal on a player
// used to apply nothing (audit row 3). The dead player-to-player crit boost
// is gone (owner ruling 3).
//
```

Replace in `internal/hooks/spell_help_effects.go` (2 of 3):

```go
	}
	durationRounds := calcSpellDuration(c.spell.BaseFolds, skill, stat) / 2
	if durationRounds < 6 {
		durationRounds = 6
	}
	casterName, targetName := c.casterName(), c.targetName()
	casterHidden, targetHidden := c.hiddenFromRoom()
	if u, m := c.casterUser(), c.targetMob(); u != nil && m != nil {
		events.AddToQueue(events.Healed{HealerUserId: u.UserId, MobInstanceId: m.InstanceId})
	}
	_ = c.targetChar().AddConditionMagnitude(conditions.ConditionIdRegenerating, durationRounds, regenMult, "heal spell")
	if c.selfCast() {
```

with:

```go
	}
	healId := spellHealConditionId(c.spell)
	casterName, targetName := c.casterName(), c.targetName()
	casterHidden, targetHidden := c.hiddenFromRoom()
	if u, m := c.casterUser(), c.targetMob(); u != nil && m != nil {
		events.AddToQueue(events.Healed{HealerUserId: u.UserId, MobInstanceId: m.InstanceId})
	}
	narrated := spellConditionNarratesStart(c, healId)
	if target := spellConditionTargetOf(c.target); target != nil {
		target.QueueCondition(events.Condition{ConditionId: healId, Source: "heal spell",
			Triggers: healTriggers(healId, stat, skill), Magnitude: regenMult, Caster: c.casterRef()})
	}
	if narrated {
		return 0
	}
	if c.selfCast() {
```

Replace in `internal/hooks/spell_help_effects.go` (3 of 3):

```go
			`A shimmering barrier surrounds %s.`, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// spellWardConditionId is the ward a shield spell lands: its one
```

with:

```go
			`A shimmering barrier surrounds %s.`, roomName(targetHidden, targetName))),
	}, spellAudience(c.casterUser(), casterName, c.targetUser(), targetName, c.room))
	return 0
}

// spellHealConditionId is the heal a heal spell lands: its one
// condition_ids entry, a heal-family record (the root guard
// TestShieldAndHealSpellsLandTheirOwnCondition holds the shipped spells to
// that), or Regenerating for a spell that names none.
func spellHealConditionId(spell *spells.SpellData) int {
	if len(spell.ConditionIds) > 0 {
		return spell.ConditionIds[0]
	}
	return conditions.ConditionIdRegenerating
}

// spellDurationTerm is the caster's part of every universal spell duration
// (calcSpellDuration): 10 + willpower/20 + spellcasting skill/2.
func spellDurationTerm(spellcastingSkill int, willpower int) float64 {
	return 10.0 + float64(willpower)/20.0 + float64(spellcastingSkill)/2.0
}

// untrainedDurationTerm is spellDurationTerm for an untrained caster at the
// stat centre of 100: the caster a heal's authored duration is written for.
var untrainedDurationTerm = spellDurationTerm(0, 100)

// healTriggers is how many rounds a heal lasts from this caster: the heal
// condition's authored trigger count, which is its duration for an untrained
// caster of average willpower, scaled by how much longer this caster's spells
// last (spellDurationTerm), at least one. A heal's identity, long or short,
// is its own data, so each heal can be gentle and long or strong and short
// whatever its casting time (owner ruling R9); the caster still lengthens
// every heal as before.
func healTriggers(conditionId int, willpower int, spellcastingSkill int) int {
	spec := conditions.GetConditionSpec(conditionId)
	if spec == nil {
		return 1
	}
	return max(1, int(math.Round(float64(spec.TriggerCount)*spellDurationTerm(spellcastingSkill, willpower)/untrainedDurationTerm)))
}

// spellWardConditionId is the ward a shield spell lands: its one
```

Replace in `internal/hooks/spell_resolution.go` (1 of 1):

```go
	}
	duration := float64(baseFolds) * (10.0 + float64(willpower)/20.0 + float64(spellcastingSkill)/2.0)
	if duration < 10 {
```

with:

```go
	}
	duration := float64(baseFolds) * spellDurationTerm(spellcastingSkill, willpower)
	if duration < 10 {
```

- [ ] **Step 6: Land the queue in older heal tests, and update the guards.**

Replace in `internal/hooks/nonharm_mob_shortcut_test.go` (1 of 1):

```go
	// applied rather than asserting on Health, which the applier never
	// touches directly.
	if !mob.Character.HasCondition(conditions.ConditionIdRegenerating) {
```

with:

```go
	// applied rather than asserting on Health, which the applier never
	// touches directly. It travels the condition event, so it is landed
	// first (messaging M6 slice 1).
	landQueuedConditions()
	if !mob.Character.HasCondition(conditions.ConditionIdRegenerating) {
```

Replace in `internal/hooks/spell_channel_sight_test.go` (1 of 2):

```go
	require.False(t, caster.Character.IsCasting(), "the completed help fold clears the cast")
	require.True(t, caster.Character.HasCondition(conditions.ConditionIdRegenerating), "the heal landed")
```

with:

```go
	require.False(t, caster.Character.IsCasting(), "the completed help fold clears the cast")
	landQueuedConditions() // the heal travels the condition event
	require.True(t, caster.Character.HasCondition(conditions.ConditionIdRegenerating), "the heal landed")
```

Replace in `internal/hooks/spell_channel_sight_test.go` (2 of 2):

```go
	require.False(t, caster.Character.IsCasting(), "the help fold resolves and clears")
	require.True(t, caster.Character.HasCondition(conditions.ConditionIdRegenerating), "the heal landed")
```

with:

```go
	require.False(t, caster.Character.IsCasting(), "the help fold resolves and clears")
	landQueuedConditions() // the heal travels the condition event
	require.True(t, caster.Character.HasCondition(conditions.ConditionIdRegenerating), "the heal landed")
```

Replace in `internal/hooks/spell_help_area_test.go` (1 of 1):

```go

func holdsRegen(c *characters.Character) bool {
	return len(c.GetConditions(conditions.ConditionIdRegenerating)) > 0
```

with:

```go

// holdsRegen lands the queued heals (a heal travels the condition event,
// messaging M6 slice 1) and reports whether c holds the Regenerating record.
func holdsRegen(c *characters.Character) bool {
	landQueuedConditions()
	return len(c.GetConditions(conditions.ConditionIdRegenerating)) > 0
```

Replace in `condition_apply_path_guard_test.go` (1 of 1):

```go
	// Task 7, same import shift as above; re-keyed again 3b playtest fix,
	// same import shift as above; re-keyed again messaging M6 slice 1, when
	// spellConditionsNarrateStart landed above it and again when the shield
	// arm moved to the event door and its Minor Shield row was deleted) ───────
	"internal/hooks/spell_help_effects.go|176": "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
	"internal/mobcommands/consume.go|46":       "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
	"internal/mobcommands/consume.go|58":       "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",

```

with:

```go
	// Task 7, same import shift as above; re-keyed again 3b playtest fix,
	// same import shift as above; the spell row DELETED messaging M6 slice 1,
	// when each heal spell began landing its own heal through the condition
	// event, leaving the two feeding rows) ───────
	"internal/mobcommands/consume.go|46": "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",
	"internal/mobcommands/consume.go|58": "former combat condition (regen): silent-start record, the spell or the feeding narrates; must apply synchronously",

```

Replace in `shipped_narration_data_guard_test.go` (1 of 1):

```go
	"conditions/136-chrysalis_cocoon.yaml":   true,
	// Lighting plan 5c: the vision spells and the tincture, narrated by the
```

with:

```go
	"conditions/136-chrysalis_cocoon.yaml":   true,
	// And the heal spells' own heals, narrated by the same start sender.
	"conditions/137-mend_flesh.yaml":         true,
	"conditions/138-mend_wounds.yaml":        true,
	"conditions/139-mend_all.yaml":           true,
	"conditions/140-communion_of_flesh.yaml": true,
	"conditions/141-mass_mend.yaml":          true,
	"conditions/142-arc_weld_repair.yaml":    true,
	// Lighting plan 5c: the vision spells and the tincture, narrated by the
```

- [ ] **Step 7: Regenerate the conditions golden and read the diff.**

Run: `go test ./internal/narration/ -run TestSnapshotStores -update -count=1`
Expected: `ok`.

Run: `git diff --stat internal/narration/testdata/stores/`
Expected: only `conditions.golden`, adding the rows of 137 to 142.

- [ ] **Step 8: Run hooks, the narration store and the root guards.**

Run: `go test ./internal/hooks/ ./internal/narration/ ./internal/devtools/ -count=1`
Expected: PASS. `ok` for all three.

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 9: Commit.**

```bash
git add spell_condition_data_guard_test.go \
  internal/hooks/spell_heal_test.go \
  _datafiles/world/dogmud/conditions/137-mend_flesh.yaml \
  _datafiles/world/dogmud/conditions/138-mend_wounds.yaml \
  _datafiles/world/dogmud/conditions/139-mend_all.yaml \
  _datafiles/world/dogmud/conditions/140-communion_of_flesh.yaml \
  _datafiles/world/dogmud/conditions/141-mass_mend.yaml \
  _datafiles/world/dogmud/conditions/142-arc_weld_repair.yaml \
  _datafiles/world/dogmud/conditions/32-vital_surge.yaml \
  _datafiles/world/dogmud/conditions/33-chrysalis_regeneration.yaml \
  _datafiles/world/dogmud/spells/heal.yaml \
  _datafiles/world/dogmud/spells/mend-wounds.yaml \
  _datafiles/world/dogmud/spells/mend-all.yaml \
  _datafiles/world/dogmud/spells/communion-of-flesh.yaml \
  _datafiles/world/dogmud/spells/mass-mend.yaml \
  _datafiles/world/dogmud/spells/repair-pulse.yaml \
  _datafiles/world/dogmud/templates/help/heal.template \
  _datafiles/world/dogmud/templates/help/mend-wounds.template \
  _datafiles/world/dogmud/templates/help/mend-all.template \
  _datafiles/world/dogmud/templates/help/communion-of-flesh.template \
  _datafiles/world/dogmud/templates/help/mass-mend.template \
  _datafiles/world/dogmud/templates/help/healing-spells.template \
  _datafiles/world/dogmud/keywords.yaml \
  internal/hooks/spell_help_effects.go \
  internal/hooks/spell_resolution.go \
  internal/hooks/nonharm_mob_shortcut_test.go \
  internal/hooks/spell_channel_sight_test.go \
  internal/hooks/spell_help_area_test.go \
  condition_apply_path_guard_test.go \
  shipped_narration_data_guard_test.go \
  internal/narration/testdata/stores/conditions.golden
git commit -F - <<'EOF'
feat(spells): six heals, each with its own pace (#338)

Every heal spell lands its own heal-family condition: a regen multiplier
from the spell and a duration from the condition, scaled by the caster
(healTriggers). Mend Flesh is long and gentle, Mend Wounds short and
strong, Mend All short and light, Communion of Flesh long and steady,
Mass Mend short and strong. Vital Surge and Chrysalis Regeneration join
the family, so any heal spell replaces any other; salves and potions
still stack. Help pages match.

Refs #338, Refs #370.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 11: Every spell-landed condition tells its own start (#370 row 1)

The 20 conditions the `effect_type: condition` spells land each gain a `start_actor` line, and the 12 with no room line gain a `start_observer`, so `NarratesCastStart` holds for all of them and no shipped spell falls back to its trio. Holder lines are reviewed to read right whether self-cast or not (Empathic Shroud's "You wrap yourself" becomes neutral) and lose their three dashes.

- [ ] **Step 1: Write the failing guard.**

Replace in `spell_condition_data_guard_test.go` (1 of 1):

```go
		t.Errorf("condition %d (the feeding record) must stay out of the heal family", conditions.ConditionIdRegenerating)
	}
}

```

with:

```go
		t.Errorf("condition %d (the feeding record) must stay out of the heal family", conditions.ConditionIdRegenerating)
	}
}

// R1, R2 and R11: a condition a spell lands tells its own start, one line
// per audience: start_actor for a caster who is someone else, start_actee
// for the holder and start_observer for the room
// (ConditionSpec.NarratesCastStart), so the spell's generic "takes effect"
// trio never has to stand in. A silent-start condition would bring the trio
// back, so none may be landed by a spell.
func TestSpellConditionsNarrateTheirOwnStart(t *testing.T) {
	all := shippedSpells(t)
	conds := shippedConditions(t)
	var problems []string
	for _, id := range sortedSpellIds(all) {
		s := all[id]
		if !slices.Contains(conditionIdReaders, s.EffectType) {
			continue
		}
		for _, cid := range s.ConditionIds {
			c := conds[cid]
			if c == nil {
				continue // reported by TestSpellConditionIdsAreReadByTheirEffectType
			}
			if !c.NarratesCastStart(false) {
				problems = append(problems, fmt.Sprintf("%s: condition %d (%s) must author start_actor, start_actee and start_observer and not be silent-start",
					id, cid, c.Name))
			}
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d spell-landed conditions cannot tell their own start:\n  - %s", len(problems), strings.Join(problems, "\n  - "))
	}
}

```

- [ ] **Step 2: Run it to see it fail.**

Run: `go test . -run TestSpellConditionsNarrateTheirOwnStart -count=1`
Expected: FAIL: 20 problems, `chrysalis-glow: condition 1 (Illumination) must author start_actor, start_actee and start_observer and not be silent-start` and one for each other condition spell.

- [ ] **Step 3: Author the start lines.**

Replace in `_datafiles/world/dogmud/conditions/1-illumination.yaml` (1 of 1):

```yaml
  - cancellable
start_actee: "A warm glow surrounds you."
```

with:

```yaml
  - cancellable
start_actor: "A warm glow blooms around {actee}."
start_actee: "A warm glow surrounds you."
```

Replace in `_datafiles/world/dogmud/conditions/2-stunned.yaml` (1 of 1):

```yaml
  - no-combat
start_actee: '<ansi fg="yellow">You are stunned! You cannot attack.</ansi>'
```

with:

```yaml
  - no-combat
start_actor: "You leave {actee} stunned and staggering."
start_actee: '<ansi fg="yellow">You are stunned! You cannot attack.</ansi>'
```

Replace in `_datafiles/world/dogmud/conditions/3-blinded.yaml` (1 of 1):

```yaml
  perception: -40
start_actee: '<ansi fg="yellow">Your vision goes dark. You have been blinded!</ansi>'
```

with:

```yaml
  perception: -40
start_actor: "Darkness swallows {actee}'s sight."
start_actee: '<ansi fg="yellow">Your vision goes dark. You have been blinded!</ansi>'
```

Replace in `_datafiles/world/dogmud/conditions/26-conviction_surge.yaml` (1 of 1):

```yaml
  strength: 15
start_actee: Conviction surges through your limbs, empowering your strikes.
end_actee: The surge of conviction fades from your limbs.
```

with:

```yaml
  strength: 15
start_actor: "Conviction surges through {actee}'s limbs."
start_actee: Conviction surges through your limbs, empowering your strikes.
start_observer: "{actee} moves with a sudden surge of strength."
end_actee: The surge of conviction fades from your limbs.
```

Replace in `_datafiles/world/dogmud/conditions/27-iron_will.yaml` (1 of 1):

```yaml
  conviction_mitigation: 10
start_actee: Your mind hardens like iron, resolute and unyielding.
end_actee: The iron resolve in your mind softens back to normal.
```

with:

```yaml
  conviction_mitigation: 10
start_actor: "{actee}'s mind hardens like iron."
start_actee: Your mind hardens like iron, resolute and unyielding.
start_observer: "{actee}'s jaw sets with iron resolve."
end_actee: The iron resolve in your mind softens back to normal.
```

Replace in `_datafiles/world/dogmud/conditions/28-chrysalis_haste.yaml` (1 of 1):

```yaml
  dexterity: 10
start_actee: "Chrysalis energy floods your nerves \u2014 everything feels faster."
end_actee: The Chrysalis haste fades and your movements return to normal.
```

with:

```yaml
  dexterity: 10
start_actor: "Chrysalis energy floods {actee}'s nerves."
start_actee: "Chrysalis energy floods your nerves, and everything feels faster."
start_observer: "{actee} begins to move with uncanny speed."
end_actee: The Chrysalis haste fades and your movements return to normal.
```

Replace in `_datafiles/world/dogmud/conditions/30-nerve_disruption.yaml` (1 of 1):

```yaml
  strength: -10
start_actee: Your muscles twitch uncontrollably as nerve signals misfire.
end_actee: Your nerves settle and control returns to your limbs.
```

with:

```yaml
  strength: -10
start_actor: "You scramble the nerves in {actee}'s limbs."
start_actee: Your muscles twitch uncontrollably as nerve signals misfire.
start_observer: "{actee}'s muscles twitch and spasm."
end_actee: Your nerves settle and control returns to your limbs.
```

Replace in `_datafiles/world/dogmud/conditions/31-empathic_shroud.yaml` (1 of 1):

```yaml
  - cancel-on-combat
start_actee: You wrap yourself in a psionic shroud, fading from perception.
start_observer: "{actee} seems to shimmer and fade from view."
# No end_user_text — like the Hidden condition, you are NOT told when the shroud
# lapses (you can't know who or what perceived you). Room observers still get
# end_room_text since, from their POV, you visibly shimmer back into sight.
end_observer: "{actee} shimmers back into view."
```

with:

```yaml
  - cancel-on-combat
start_actor: "You wrap {actee} in a psionic shroud."
start_actee: "A psionic shroud wraps around you, and you fade from perception."
start_observer: "{actee} seems to shimmer and fade from view."
# No end_actee: like the Hidden condition, you are NOT told when the shroud
# lapses (you can't know who or what perceived you). Room observers still get
# end_observer since, from their POV, you visibly shimmer back into sight.
end_observer: "{actee} shimmers back into view."
```

Replace in `_datafiles/world/dogmud/conditions/32-vital_surge.yaml` (1 of 1):

```yaml
triggercount: 9
start_actee: Chrysalis energy suffuses your body with a warm, mending pulse.
end_actee: The vital surge fades.
```

with:

```yaml
triggercount: 9
start_actor: "Chrysalis energy pulses into {actee}'s body."
start_actee: Chrysalis energy suffuses your body with a warm, mending pulse.
start_observer: "A warm pulse of light settles over {actee}."
end_actee: The vital surge fades.
```

Replace in `_datafiles/world/dogmud/conditions/33-chrysalis_regeneration.yaml` (1 of 1):

```yaml
triggercount: 8
start_actee: Deep Chrysalis energy wraps around you, accelerating your body's healing.
end_actee: The Chrysalis regeneration fades.
```

with:

```yaml
triggercount: 8
start_actor: "Deep Chrysalis energy wraps around {actee}."
start_actee: Deep Chrysalis energy wraps around you, accelerating your body's healing.
start_observer: "Deep green light wraps around {actee}."
end_actee: The Chrysalis regeneration fades.
```

Replace in `_datafiles/world/dogmud/conditions/34-skill_attunement.yaml` (1 of 1):

```yaml
  - skill-progress
start_actee: "Your mind opens and sharpens \u2014 every action feels more instructive."
end_actee: The heightened attunement fades from your mind.
```

with:

```yaml
  - skill-progress
start_actor: "{actee}'s mind opens to learning."
start_actee: "Your mind opens and sharpens. Every action feels more instructive."
start_observer: "{actee} looks suddenly sharp and attentive."
end_actee: The heightened attunement fades from your mind.
```

Replace in `_datafiles/world/dogmud/conditions/35-mutation_catalyst.yaml` (1 of 1):

```yaml
  - mutation-rate
start_actee: "Chrysalis energy surges through your cells, accelerating mutation."
end_actee: The mutagenic catalyst fades from your system.
```

with:

```yaml
  - mutation-rate
start_actor: "Chrysalis energy surges through {actee}'s cells."
start_actee: "Chrysalis energy surges through your cells, accelerating mutation."
start_observer: "{actee}'s skin ripples faintly with Chrysalis light."
end_actee: The mutagenic catalyst fades from your system.
```

Replace in `_datafiles/world/dogmud/conditions/36-psychic_anchor.yaml` (1 of 1):

```yaml
  - no-go
start_actee: "A psionic anchor locks onto your mind, rooting you in place!"
end_actee: The psychic anchor releases its grip on your mind.
```

with:

```yaml
  - no-go
start_actor: "Your psionic anchor locks onto {actee}'s mind."
start_actee: "A psionic anchor locks onto your mind, rooting you in place!"
start_observer: "{actee} stiffens, rooted to the spot."
end_actee: The psychic anchor releases its grip on your mind.
```

Replace in `_datafiles/world/dogmud/conditions/37-sensory_overload.yaml` (1 of 1):

```yaml
  dexterity: -15
start_actee: "Your senses explode with input \u2014 everything is too bright, too loud!"
end_actee: The sensory overload subsides and your senses normalize.
```

with:

```yaml
  dexterity: -15
start_actor: "You flood {actee}'s senses until they overload."
start_actee: "Your senses explode with input. Everything is too bright, too loud!"
start_observer: "{actee} reels, overwhelmed by something unseen."
end_actee: The sensory overload subsides and your senses normalize.
```

Replace in `_datafiles/world/dogmud/conditions/38-conviction_armor.yaml` (1 of 1):

```yaml
  vitality: 30
start_actee: Solidified conviction wraps around your body like armor.
```

with:

```yaml
  vitality: 30
start_actor: "Solidified conviction wraps around {actee} like armor."
start_actee: Solidified conviction wraps around your body like armor.
```

Replace in `_datafiles/world/dogmud/conditions/41-mind_fog.yaml` (1 of 1):

```yaml
  willpower: -10
start_actee: A thick fog descends over your thoughts...
end_actee: The fog lifts from your mind and clarity returns.
```

with:

```yaml
  willpower: -10
start_actor: "A thick fog descends over {actee}'s thoughts."
start_actee: A thick fog descends over your thoughts...
start_observer: "{actee}'s eyes glaze over."
end_actee: The fog lifts from your mind and clarity returns.
```

Replace in `_datafiles/world/dogmud/conditions/53-veil_sight.yaml` (1 of 1):

```yaml
description: Your perception pierces the Veil, revealing hidden creatures.
start_actee: Your sight pierces the Veil and hidden things show.
end_actee: The Veil closes again and hidden things slip away.
```

with:

```yaml
description: Your perception pierces the Veil, revealing hidden creatures.
start_actor: "{actee}'s sight pierces the Veil."
start_actee: Your sight pierces the Veil and hidden things show.
start_observer: "{actee}'s eyes take on a pale, far-seeing gleam."
end_actee: The Veil closes again and hidden things slip away.
```

Replace in `_datafiles/world/dogmud/conditions/128-night_sight.yaml` (1 of 1):

```yaml
  until the light fades or the spell wears off.
start_actee: Your eyes sharpen for faint light.
```

with:

```yaml
  until the light fades or the spell wears off.
start_actor: "{actee}'s eyes sharpen for faint light."
start_actee: Your eyes sharpen for faint light.
```

Replace in `_datafiles/world/dogmud/conditions/129-heat_sight.yaml` (1 of 1):

```yaml
  by sight suffers until the light fades or the spell wears off.
start_actee: Warmth blooms into pale shapes wherever something lives.
```

with:

```yaml
  by sight suffers until the light fades or the spell wears off.
start_actor: "Warmth blooms into pale shapes before {actee}'s eyes."
start_actee: Warmth blooms into pale shapes wherever something lives.
```

Replace in `_datafiles/world/dogmud/conditions/131-chrysalis_pall.yaml` (1 of 1):

```yaml
  - cancellable
start_actee: "A pall of dark spores gathers around you."
```

with:

```yaml
  - cancellable
start_actor: "A pall of dark spores gathers around {actee}."
start_actee: "A pall of dark spores gathers around you."
```

- [ ] **Step 4: Add the new room lines to the anonymizer guard.**

Replace in `shipped_narration_data_guard_test.go` (1 of 1):

```go
	"conditions/142-arc_weld_repair.yaml":    true,
	// Lighting plan 5c: the vision spells and the tincture, narrated by the
```

with:

```go
	"conditions/142-arc_weld_repair.yaml":    true,
	// And the spell-landed conditions that gained a start_observer line so a
	// spell's generic trio is never needed (owner ruling R11), narrated by
	// the same start sender.
	"conditions/26-conviction_surge.yaml":       true,
	"conditions/27-iron_will.yaml":              true,
	"conditions/28-chrysalis_haste.yaml":        true,
	"conditions/30-nerve_disruption.yaml":       true,
	"conditions/32-vital_surge.yaml":            true,
	"conditions/33-chrysalis_regeneration.yaml": true,
	"conditions/34-skill_attunement.yaml":       true,
	"conditions/35-mutation_catalyst.yaml":      true,
	"conditions/36-psychic_anchor.yaml":         true,
	"conditions/37-sensory_overload.yaml":       true,
	"conditions/41-mind_fog.yaml":               true,
	"conditions/53-veil_sight.yaml":             true,
	// Lighting plan 5c: the vision spells and the tincture, narrated by the
```

- [ ] **Step 5: Regenerate the conditions golden and read the diff.**

Run: `go test ./internal/narration/ -run TestSnapshotStores -update -count=1`
Expected: `ok`.

Run: `git diff --stat internal/narration/testdata/stores/`
Expected: only `conditions.golden`: 20 `start_actor` rows, 12 `start_observer` rows, and the three reworded holder lines (28, 34, 37) and Empathic Shroud's.

- [ ] **Step 6: Run the root guards and the narration store.**

Run: `go test ./internal/narration/ ./internal/hooks/ -count=1`
Expected: PASS. `ok` for both.

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 7: Commit.**

```bash
git add spell_condition_data_guard_test.go \
  _datafiles/world/dogmud/conditions/1-illumination.yaml \
  _datafiles/world/dogmud/conditions/2-stunned.yaml \
  _datafiles/world/dogmud/conditions/3-blinded.yaml \
  _datafiles/world/dogmud/conditions/26-conviction_surge.yaml \
  _datafiles/world/dogmud/conditions/27-iron_will.yaml \
  _datafiles/world/dogmud/conditions/28-chrysalis_haste.yaml \
  _datafiles/world/dogmud/conditions/30-nerve_disruption.yaml \
  _datafiles/world/dogmud/conditions/31-empathic_shroud.yaml \
  _datafiles/world/dogmud/conditions/32-vital_surge.yaml \
  _datafiles/world/dogmud/conditions/33-chrysalis_regeneration.yaml \
  _datafiles/world/dogmud/conditions/34-skill_attunement.yaml \
  _datafiles/world/dogmud/conditions/35-mutation_catalyst.yaml \
  _datafiles/world/dogmud/conditions/36-psychic_anchor.yaml \
  _datafiles/world/dogmud/conditions/37-sensory_overload.yaml \
  _datafiles/world/dogmud/conditions/38-conviction_armor.yaml \
  _datafiles/world/dogmud/conditions/41-mind_fog.yaml \
  _datafiles/world/dogmud/conditions/53-veil_sight.yaml \
  _datafiles/world/dogmud/conditions/128-night_sight.yaml \
  _datafiles/world/dogmud/conditions/129-heat_sight.yaml \
  _datafiles/world/dogmud/conditions/131-chrysalis_pall.yaml \
  shipped_narration_data_guard_test.go \
  internal/narration/testdata/stores/conditions.golden
git commit -F - <<'EOF'
content(conditions): every spell-landed condition tells its own start (#370)

The 20 conditions the condition spells land each gain a start_actor line
for the caster, and the 12 with no room line gain a start_observer, so a
spell never falls back to its generic trio. Holder lines read right
whether self-cast or not, and three lose a dash. A root guard keeps it
so.

Refs #370.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 12: Docs, patch notes and the guard's README row

Every package whose API changed gets its `context.md` brought true; `docs/PATCH_NOTES.md` gets the player-facing entry with the trigger warning (spec section 4); `docs/README.md` gets the root guard's row.

- [ ] **Step 1: Bring each context.md true.**

Replace in `internal/conditions/context.md` (1 of 8):

```markdown
| Recovery penalty (standing up) | Recovering | 118 |
| Shield | Minor Shield | 119 |
| Regenerating | Regenerating | 120 |
```

with:

```markdown
| Recovery penalty (standing up) | Recovering | 118 |
| Shield | Conviction Ward (renamed from Minor Shield, messaging M6 slice 1) | 119 |
| Regenerating | Regenerating | 120 |
```

Replace in `internal/conditions/context.md` (2 of 8):

```markdown
| `defense_mult` | the defense score, same file |
| `mitigation_flat` | `Character.GetPhysicalMitigation`, `internal/characters/combat.go`; `internal/combat/ai.go` and `internal/behaviortree/action_cast_best_in_category.go` ask `HasEffect` before casting a ward |
| `dodge_mult` | `Character.GetDefenseScoreFor`, `internal/characters/combat.go` (`GetDefenseScore` is a one-line wrapper over it; no shipped producer, and the seam is kept because `Effect` is a product with identity 1.0) |
```

with:

```markdown
| `defense_mult` | the defense score, same file |
| `mitigation_flat` | `Character.GetPhysicalMitigation`, `internal/characters/combat.go` |
| `mitigation_magical` | `Character.GetMagicalMitigation`, same file (a ward that blocks spells, messaging M6 slice 1) |
| `mitigation_conviction` | `Character.GetConvictionMitigation`, same file (a ward that blocks social damage) |
| `dodge_mult` | `Character.GetDefenseScoreFor`, `internal/characters/combat.go` (`GetDefenseScore` is a one-line wrapper over it; no shipped producer, and the seam is kept because `Effect` is a product with identity 1.0) |
```

Replace in `internal/conditions/context.md` (3 of 8):

```markdown
    Hooded         bool      // plan 5a: a hooded light sheds nothing
}
```

with:

```markdown
    Hooded         bool      // plan 5a: a hooded light sheds nothing
    Caster         state.ActorRef // M6 slice 1: who applied it; only a player caster is saved
}
```

Replace in `internal/conditions/context.md` (4 of 8):

```markdown
    EffectMitigationFlat EffectKind = "mitigation_flat" // flat physical mitigation points (wards)
    EffectPoolMaxPct     EffectKind = "pool_max_pct"    // fraction taken off a pool maximum; the pool rides on Condition.Source
```

with:

```markdown
    EffectMitigationFlat EffectKind = "mitigation_flat" // flat physical mitigation points (wards)
    EffectMitigationMagical    EffectKind = "mitigation_magical"    // a ward's flat points against spell (mental) damage
    EffectMitigationConviction EffectKind = "mitigation_conviction" // a ward's flat points against social damage
    EffectPoolMaxPct     EffectKind = "pool_max_pct"    // fraction taken off a pool maximum; the pool rides on Condition.Source
```

Replace in `internal/conditions/context.md` (5 of 8):

```markdown
into a window wider than the better one grants; everything else
(`mitigation_flat`, `pool_max_pct`) sums with identity `0`. It never calls
`HasFlag` with `expire=true`, so reading it has no side effect.
```

with:

```markdown
into a window wider than the better one grants; everything else
(`mitigation_flat`, `mitigation_magical`, `mitigation_conviction`,
`pool_max_pct`) sums with identity `0`. It never calls
`HasFlag` with `expire=true`, so reading it has no side effect.
```

Replace in `internal/conditions/context.md` (6 of 8):

```markdown

### Magnitude Records (`AddConditionMagnitude`)

`Conditions.AddConditionMagnitude(conditionId int, triggers int, magnitude float64) bool`
is the writer door for every record that used to be a hand-rolled combat
condition (Minor Shield, Regenerating, Poisoned, Bleeding). It refreshes or
adds the condition via `addConditionScaled(conditionId, 1.0)`, then, if
```

with:

```markdown

### The caster on a record (messaging M6 slice 1)

`Condition.Caster state.ActorRef` (yaml `caster`) is who put the record on
its holder: a spell's caster, the attacker whose blow opened a bleed, zero
for a potion, hazard, mutation or gear. `Conditions.Stamp(conditionId,
source, caster)` writes it and `Source` on the held record after an add door
lands it; the newest application owns the record, so a re-application by
someone else takes it over and a casterless one clears it. A stacking record
is one record, so it carries its newest applier's caster.

Only a player caster is saved. `Condition.MarshalYAML` writes the caster's
`UserId` and zeroes its `MobInstanceId` (a mob's instance id is runtime only
and names a different creature after a restart), and `UnmarshalYAML` drops a
mob caster from an old save. Both go through `plainCondition`, the struct
without its methods, so neither recurses.

### Families (messaging M6 slice 1)

`ConditionSpec.Family` (yaml `family`) is `FamilyWard` (`ward`),
`FamilyHeal` (`heal`) or empty (`family.go`; `AllFamilies` is the closed
set, and `validateFamily` refuses any other name at load). Landing a record
whose spec names a family discards every other record of that family on the
holder first, inside `addConditionScaled` and `AddCondition`
(`discardFamilyRivals`, through `Discard`, so no end line is told), after
the refusal checks, so a refused add keeps its rival. Re-landing the same id
is a refresh. A record with no family is never touched, so potions, salves,
items and mutations keep stacking with a ward or a heal.

`FamilyRivals(conditionId)` lists the live records an add would replace;
the apply hook reads it BEFORE the add for its replacement line.
`HasFamily(family)` is the mob AI's "already shielded" and "already
healing" test.

### Caster lines (messaging M6 slice 1)

`ConditionSpec.StartActorText` (yaml `start_actor`) is the caster's line
when the caster is someone other than the holder; it names the holder with
`{actee}`. `StartActorNotice()` silences it like `StartUserNotice()` (secret,
quiet, silent-start) with no generic fallback. `Narration(PhaseStart)`
carries it as the `Actor` variant, and `NarrateCast(phase, holderName,
holderPlain, casterName, casterPlain)` fills `{actor}` with the caster;
`Narrate` is `NarrateCast` with no caster. Only start lines may name the
caster: `validateNarration` refuses `{actor}` in trigger and end text.
`NarratesCastStart(selfCast)` reports whether the condition's own start
lines tell every audience a spell's generic trio served (authored
`start_actee` and `start_observer`, and `start_actor` unless the caster is
the holder); a spell drops its trio only then (owner ruling R11).

### Magnitude Records (`AddConditionMagnitude`)

`Conditions.AddConditionMagnitude(conditionId int, triggers int, magnitude float64) bool`
is the writer door for every record that used to be a hand-rolled combat
condition (Conviction Ward, Regenerating, Poisoned, Bleeding). It refreshes or
adds the condition via `addConditionScaled(conditionId, 1.0)`, then, if
```

Replace in `internal/conditions/context.md` (7 of 8):

````markdown

Two live spells use `effect_type: shield`: `conviction-ward`
(`effect_magnitude: 75`) and `chrysalis-cocoon` (`effect_magnitude: 125`), both
defined under `_datafiles/world/dogmud/spells/`. `applySpellShield` in `internal/hooks/spell_help_effects.go` is the single
handler for every shield spell, whoever casts it and whoever it lands on
(parity slice 3b; a shield on a pet, or from a creature, applied nothing
before it).

```go
shieldBonus := (stat + weightedSkill) / 3
if c.magnitude > 0 {
    shieldBonus = int(math.Round(float64(shieldBonus) * float64(c.magnitude) / 100.0))
}
_ = c.targetChar().AddConditionMagnitude(conditions.ConditionIdMinorShield, duration, float64(shieldBonus), "spell")
```

`stat` and the skill come from `spellCasterStatAndSkill`: the spell's
primarystat through `CasterStatValue`, and the school's cast skill.
`weightedSkill` is that skill times `SkillWeight` (ships 5.0 against a Go
default of 2.0). `magnitude` is `spellData.EffectMagnitude`, and 100 is the
1.0x baseline: a spell carrying `effect_magnitude: 75` applies 0.75 of the
base roll, one carrying 125 applies 1.25x. A shield does not crit. The
player path used to carry a x1.5 crit bump no cast could reach: both
shipped shields are `attack_type: none`, so every resolver takes them
through `resolveHelpSpell` with `uncontestedSpellResult()` and no roll;
slice 3b deleted the bump (owner ruling, 2026-09-28). A future crit would
need a real roll: the static-difficulty seam
`contest.AgainstDifficulty(score, difficulty)` (`internal/contest/contest.go`)
already exists and is used by search, track, and forage checks; no spell path
calls it. Duration is computed
by the unexported `calcSpellDuration(baseFolds, spellcastingSkill, willpower)`
in the same file, not by anything in this package:
`duration = baseFolds * (10 + willpower/20 + spellcastingSkill/2)`.

**Where the magnitude actually lands.** The 119 Minor Shield record does not
touch `magical_mitigation` or `conviction_mitigation` at all. Its declared
effect is `mitigation_flat: magnitude`, and the only reader of that kind on a
character is `Character.GetPhysicalMitigation()`
(`internal/characters/combat.go`), which takes it through
`c.Conditions.Effect(conditions.EffectMitigationFlat)` and sums it with gear
`physical_mitigation`, mutation natural armor, and species natural armor, then
clamps the total at `PhysicalMitigationCap`. So both "magical" ward spells buy
physical mitigation through the Minor Shield record, not magical or conviction
mitigation.

A shield spell can separately carry `condition_ids`,
and those conditions use the ordinary statmod path described above instead:
`chrysalis-cocoon` grants `condition_ids: [52]` (Chrysalis Shell, in
`_datafiles/world/dogmud/conditions/52-chrysalis_shell.yaml`), whose
`statmods: {magical_mitigation: 15, conviction_mitigation: 15}` are summed by
`Conditions.StatMod()` and read by `Character.GetMagicalMitigation()` /
`GetConvictionMitigation()` through `c.StatMod("magical_mitigation")` /
`c.StatMod("conviction_mitigation")`. `conviction-ward` sets no `condition_ids`, so
it grants no magical or conviction mitigation at all despite its name.

````

with:

````markdown

Three live spells use `effect_type: shield` (messaging M6 slice 1, owner
ruling R7), each landing its own ward through its one `condition_ids`
entry:

| Spell | `effect_magnitude` | Ward | Blocks |
|---|---|---|---|
| `conviction-ward` | 75 | 119 Conviction Ward | physical |
| `conviction-bulwark` | 100 | 135 Conviction Bulwark | physical, spell |
| `chrysalis-cocoon` | 125 | 136 Chrysalis Cocoon | physical, spell, social |

`applySpellShield` in `internal/hooks/spell_help_effects.go` is the single
handler for every shield spell, whoever casts it and whoever it lands on:

```go
shieldBonus := (stat + weightedSkill) / 3
if c.magnitude > 0 {
    shieldBonus = int(math.Round(float64(shieldBonus) * float64(c.magnitude) / 100.0))
}
target.QueueCondition(events.Condition{ConditionId: wardId, Source: "spell",
    Triggers: duration, Magnitude: float64(shieldBonus), Caster: c.casterRef()})
```

`stat` and the skill come from `spellCasterStatAndSkill`: the spell's
primarystat through `CasterStatValue`, and the school's cast skill.
`weightedSkill` is that skill times `SkillWeight` (ships 5.0 against a Go
default of 2.0). `magnitude` is `spellData.EffectMagnitude`, and 100 is the
1.0x baseline. A shield does not crit (owner ruling, 2026-09-28). Duration
is the unexported `calcSpellDuration(baseFolds, spellcastingSkill,
willpower)` in `internal/hooks/spell_resolution.go`:
`duration = baseFolds * (10 + willpower/20 + spellcastingSkill/2)`.

**Where the magnitude lands.** The ward record carries the one strength,
and each effect kind it declares reads it (`magnitude`):
`mitigation_flat` feeds `Character.GetPhysicalMitigation()`,
`mitigation_magical` feeds `GetMagicalMitigation()` and
`mitigation_conviction` feeds `GetConvictionMitigation()`
(`internal/characters/combat.go`), each summed with gear, mutations and the
matching statmod, so a potion's `magical_mitigation` still adds on top. The
total is clamped downstream at the channel's mitigation cap. The wards are
the `ward` family (below), so a target holds one at a time; condition 52
Chrysalis Shell is no longer named by any spell and is left unchanged.

````

Replace in `internal/conditions/context.md` (8 of 8):

```markdown
| `conditionspec.go` | The authored `ConditionSpec` and its loader |
| `notice.go` | The player-side start/end notice resolver and `SilentNoticeConditions` |
| `narration.go` | The narration door: `Phase`, `Narration`, `Narrate`, `AuthoredStartLine`, `validateNarration` |
| `conditions.go` | Held condition instances (`Condition`, `Conditions`), flags, stat mods, `AddConditionMagnitude`, `GetDurations` |
| `tick.go` | `ComputeTickAmount`, the tick-pool amount formula |
| `stacks.go` | `Stack`, the stacking tick (`addStack`, `syncStacks`, `tickStacks`), `tickAmountFor`, `DisplayName` |
| `effects.go` | `EffectKind`, the closed effects vocabulary, `Conditions.Effect` / `Conditions.HasEffect` / `Conditions.EffectValues` (lighting plan 5c), `ScaledKinds` / `ConditionSpec.ScaledKind` (lighting plan 5c) |
| `scaled_magnitude.go` | `SpellScaledMagnitude` (lighting plan 5c): the base + stat/D1 + skill/D2 value a spell applies a scaled kind at, capped; `CapScaledMagnitude`, the cap both spell and potion apply (infra reach at `LightInfraReachCap`, nightvision at `configs.LightWindowShiftCap`); `SpellScaledTriggers` (lighting plan 5d), the trigger count, where darkness reads its own `LightDarknessSpellDuration*` trio and the other kinds share the light trio; `NewCharacterSpellStat` / `NewCharacterSpellSkill`, the numbers the admin `setcondition` command evaluates it at |
| `light.go` | Lighting plan 5a: `LightTrim`, `Condition.LightMax` / `LightNow` / `SetLightOutput` / `ResetLight`, `Conditions.LightSources`; plan 5d: `Conditions.DarknessSources`, `Conditions.LightAndDarknessSources` |
| `ids.go` | The record ids the engine names in code: `ConditionIdWarcry` (79) through `ConditionIdEnchantWithdrawal` (123) |
| `test_helpers.go` | Test fixtures: `SeedConditionsForTest` (replaces the registry) and `SeedConditionRecordsForTest` (adds 79, 80 and 117 to 123 on top of whatever is already seeded) |
```

with:

```markdown
| `conditionspec.go` | The authored `ConditionSpec` and its loader |
| `notice.go` | The player-side start/end notice resolver (`StartActorNotice` for the caster's line), `NarratesCastStart` and `SilentNoticeConditions` |
| `narration.go` | The narration door: `Phase`, `Narration`, `Narrate`, `NarrateCast` (the caster fills `{actor}`), `AuthoredStartLine`, `validateNarration` |
| `conditions.go` | Held condition instances (`Condition`, `Conditions`), flags, stat mods, `AddConditionMagnitude`, `GetDurations` |
| `tick.go` | `ComputeTickAmount`, the tick-pool amount formula |
| `stacks.go` | `Stack`, the stacking tick (`addStack`, `syncStacks`, `tickStacks`), `tickAmountFor`, `DisplayName` |
| `effects.go` | `EffectKind`, the closed effects vocabulary, `Conditions.Effect` / `Conditions.HasEffect` / `Conditions.EffectValues` (lighting plan 5c), `ScaledKinds` / `ConditionSpec.ScaledKind` (lighting plan 5c) |
| `scaled_magnitude.go` | `SpellScaledMagnitude` (lighting plan 5c): the base + stat/D1 + skill/D2 value a spell applies a scaled kind at, capped; `CapScaledMagnitude`, the cap both spell and potion apply (infra reach at `LightInfraReachCap`, nightvision at `configs.LightWindowShiftCap`); `SpellScaledTriggers` (lighting plan 5d), the trigger count, where darkness reads its own `LightDarknessSpellDuration*` trio and the other kinds share the light trio; `NewCharacterSpellStat` / `NewCharacterSpellSkill`, the numbers the admin `setcondition` command evaluates it at |
| `light.go` | Lighting plan 5a: `LightTrim`, `Condition.LightMax` / `LightNow` / `SetLightOutput` / `ResetLight`, `Conditions.LightSources`; plan 5d: `Conditions.DarknessSources`, `Conditions.LightAndDarknessSources` |
| `caster.go` | Messaging M6 slice 1: `Conditions.Stamp` (source and caster on a held record) and `Condition.MarshalYAML` / `UnmarshalYAML` (a mob caster is never saved) |
| `family.go` | Messaging M6 slice 1: `FamilyWard`, `FamilyHeal`, `AllFamilies`, `Conditions.HasFamily` / `FamilyRivals`, and the rival discard the add primitives call |
| `ids.go` | The record ids the engine names in code: `ConditionIdWarcry` (79) through `ConditionIdEnchantWithdrawal` (123); 119 is `ConditionIdConvictionWard` |
| `test_helpers.go` | Test fixtures: `SeedConditionsForTest` (replaces the registry) and `SeedConditionRecordsForTest` (adds 79, 80 and 117 to 123 on top of whatever is already seeded) |
```

Replace in `internal/characters/context.md` (1 of 2):

```markdown
  spells do not crit). `internal/hooks/spell_help_effects.go`'s
  `applySpellHeal` calls `c.targetChar().AddConditionMagnitude(conditions.ConditionIdRegenerating,
  durationRounds, regenMult, "heal spell")` with `durationRounds =
  calcSpellDuration(...)/2`, floored at 6 rounds. Each round after that,
  `NewRound_AutoHeal.go` reads `Conditions.HasEffect(conditions.EffectRegenMult)` and
```

with:

```markdown
  spells do not crit). `internal/hooks/spell_help_effects.go`'s
  `applySpellHeal` queues the spell's own heal condition (its one
  `condition_ids` entry, a `regen_mult: magnitude` record in the heal family;
  messaging M6 slice 1) for `healTriggers` rounds: the heal's authored
  `triggercount` scaled by the caster. Each round after that,
  `NewRound_AutoHeal.go` reads `Conditions.HasEffect(conditions.EffectRegenMult)` and
```

Replace in `internal/characters/context.md` (2 of 2):

```markdown
(`combat.AttemptCritDisarm`) uses it; before, a full pack lost the weapon.

```

with:

```markdown
(`combat.AttemptCritDisarm`) uses it; before, a full pack lost the weapon.

## The caster door (messaging M6 slice 1)

`AddConditionMagnitudeBy(conditionId, triggers, magnitude, source, caster
state.ActorRef) error` is `AddConditionMagnitude` with the record's caster,
stamped through `conditions.Conditions.Stamp`; `AddConditionMagnitude` is
the same door with a zero caster. The spell dot (`applySpellDot`) and every
combat bleed (drain, hamstring, maul, rake, throttle and the item proc) land
through it, so a tick that kills credits the caster (#240).

The ward effects `mitigation_magical` and `mitigation_conviction` are read by
`GetMagicalMitigation` and `GetConvictionMitigation` through
`Conditions.Effect`, beside the statmods they sum with, as `mitigation_flat`
is by `GetPhysicalMitigation`.

```

Replace in `internal/events/context.md` (1 of 1):

```markdown

`Condition` (`eventtypes.go`) carries `TickScale float64` (tick amount at
```

with:

```markdown

`Condition` also carries `Caster state.ActorRef` and `CasterCrit bool`
(messaging M6 slice 1): who cast it, which `hooks.ApplyConditions` stamps
on the record and names in the start lines, and whether the crit marker
rides the caster's start line. `users.UserRecord.QueueCondition` and
`mobs.Mob.QueueCondition` are the doors that carry them.

`Condition` (`eventtypes.go`) carries `TickScale float64` (tick amount at
```

Replace in `internal/users/context.md` (1 of 1):

```markdown

## Usage Examples
```

with:

```markdown

`QueueCondition(evt events.Condition)` (messaging M6 slice 1) queues a
condition the producer filled in, stamping this user as the holder and the
current life epoch over whatever the producer wrote there. It is the door a
caster rides through (`Caster`, `CasterCrit`), and the four add doors above
are now this door with their own fields set.

## Usage Examples
```

Replace in `internal/mobs/context.md` (1 of 1):

```markdown

// Command execution through Input events
```

with:

```markdown

// QueueCondition (messaging M6 slice 1) is the mob twin of
// UserRecord.QueueCondition: it queues a condition the producer filled in,
// stamping the mob as the holder and its life epoch. A spell's caster rides
// through it (Caster, CasterCrit); the four add doors above are this door
// with their own fields set.
func (m *Mob) QueueCondition(evt events.Condition)

// Command execution through Input events
```

Replace in `internal/actions/context.md` (1 of 2):

```markdown

## Caller Integration
```

with:

```markdown

## `ActorRefOf` (messaging M6 slice 1)

`ActorRefOf(a Actor) state.ActorRef` names an actor for harm attribution (a
player by user id, a mob by instance id, nobody for nil). The combat moves'
bleeds (drain, the drain area, hamstring, maul, rake, throttle) pass it to
`Character.AddConditionMagnitudeBy` as the record's caster, so a bleed that
kills credits the attacker (#240).

## Caller Integration
```

Replace in `internal/actions/context.md` (2 of 2):

```markdown
|-------|-------|
| Actor abstraction | `actor.go`, `actor_user.go`, `actor_mob.go` |
| Readiness gates | `action_readiness.go`, `command_readiness.go` |
```

with:

```markdown
|-------|-------|
| Actor abstraction | `actor.go`, `actor_user.go`, `actor_mob.go`, `actor_ref.go` (`ActorRefOf`) |
| Readiness gates | `action_readiness.go`, `command_readiness.go` |
```

Replace in `internal/behaviortree/context.md` (1 of 2):

```markdown
- **`proc(effect, ...)`** runs `procLifesteal`, `procStealPool`,
  `procAoeStun` or `procApplyCondition` (moved unchanged from hooks) for
  `itemHolder(ctx)` against `ctx.Event.Proc` (`ProcEvent{Other, Room,
```

with:

```markdown
- **`proc(effect, ...)`** runs `procLifesteal`, `procStealPool`,
  `procAoeStun` or `procApplyCondition` (moved from hooks; it takes the
  owner and opens its bleed in the owner's name through `procOwnerRef`, so
  a bleed kill credits the owner, messaging M6 slice 1) for
  `itemHolder(ctx)` against `ctx.Event.Proc` (`ProcEvent{Other, Room,
```

Replace in `internal/behaviortree/context.md` (2 of 2):

```markdown
  `TestItemProcParity` holds the trees to it.

```

with:

```markdown
  `TestItemProcParity` holds the trees to it.

## Wards and heals already up (messaging M6 slice 1)

`cast_best_in_category` skips a candidate whose effect is already on the
caster (`spellEffectAlreadyActive`, `action_cast_best_in_category.go`): a
shield spell while any ward is up, a heal spell while any heal is
(`Conditions.HasFamily` with `conditions.FamilyWard` or `FamilyHeal`), and
any other spell while one of its `ConditionIds` is held. Each shield and
heal spell lands its own condition, and a new one only replaces the old, so
casting a second ward or heal would swap one for another.

```

Replace in `internal/combat/context.md` (1 of 2):

```markdown
shield condition a SECOND time on every combat round while the condition tick
had already decremented it once. Minor Shield is condition 119 now, decays once a
round with every other record, and narrates its end wherever it ends rather
```

with:

```markdown
shield condition a SECOND time on every combat round while the condition tick
had already decremented it once. The ward is condition 119 now (Conviction Ward
since messaging M6 slice 1; 135 and 136 are the other two wards), decays once a
round with every other record, and narrates its end wherever it ends rather
```

Replace in `internal/combat/context.md` (2 of 2):

```markdown
      See `characters/context.md` for details.
12. **Minor Shield reduction**: NOT flat damage off the top. It contributes
    mitigation POINTS, read by `Character.GetPhysicalMitigation` through
    `Conditions.Effect(conditions.EffectMitigationFlat)`, summed with gear, natural
    armor, species armor and the `physical_mitigation` statmods, and the whole
    sum is divided by 100 to become the mitigation FRACTION the damage
    pipeline applies. The 119 Minor Shield record declares
    `mitigation_flat: magnitude`.
13. **Adrenaline Surge** — mutation check for bonus damage.
```

with:

```markdown
      See `characters/context.md` for details.
12. **Ward reduction**: NOT flat damage off the top. It contributes
    mitigation POINTS, read by `Character.GetPhysicalMitigation` through
    `Conditions.Effect(conditions.EffectMitigationFlat)`, summed with gear, natural
    armor, species armor and the `physical_mitigation` statmods, and the whole
    sum is divided by 100 to become the mitigation FRACTION the damage
    pipeline applies. Every ward (119, 135, 136) declares
    `mitigation_flat: magnitude`; 135 and 136 also declare
    `mitigation_magical` and 136 `mitigation_conviction`, read by the
    magical and conviction mitigations the same way. `preferredSpell`
    (`ai.go`) casts `conviction-ward` when no ward is up:
    `Conditions.HasFamily(conditions.FamilyWard)`.
13. **Adrenaline Surge** — mutation check for bonus damage.
```

Replace in `internal/hooks/context.md` (1 of 4):

```markdown
`ok` is false for any other condition, which keeps its authored application.
`applySpellCondition(target, spellData, caster, conditionId)` is the one door
the one spell-condition applier, `applySpellConditionEffect`
(`spell_help_effects.go`, every pairing since parity slice 3b), calls: a magnitude-scaled light or sight
goes through `AddConditionMagnitude`; a `tick_pool` condition (a heal- or
damage-over-time) goes through `AddConditionTickScaled` at
`spellTickScale(caster)` (`spell_tick_scale.go`, tick amount at apply,
2026-09-28); anything else through `AddCondition`. All three sit on the small
`spellConditionTarget` interface a `*users.UserRecord` and a `*mobs.Mob` both
satisfy. The record then trims to its HOLDER's eyes, who may not be the
```

with:

```markdown
`ok` is false for any other condition, which keeps its authored application.
`applySpellCondition(target, spellData, caster, conditionId, casterRef,
crit)` is the one door the one spell-condition applier,
`applySpellConditionEffect` (`spell_help_effects.go`, every pairing since
parity slice 3b), calls. It fills one `events.Condition` (source "spell",
the caster ref and the crit marker, messaging M6 slice 1): a
magnitude-scaled light or sight sets `Magnitude` and `Triggers`; a
`tick_pool` condition (a heal- or damage-over-time) sets `TickScale` to
`spellTickScale(caster)` (`spell_tick_scale.go`, tick amount at apply,
2026-09-28); anything else keeps its authored values. It queues the event
through the `spellConditionTarget` interface's one method,
`QueueCondition`, which a `*users.UserRecord` and a `*mobs.Mob` both
satisfy. The record then trims to its HOLDER's eyes, who may not be the
```

Replace in `internal/hooks/context.md` (2 of 4):

```markdown
- **Shield: full duration, no divisor.** `applySpellShield`
  (`spell_help_effects.go`) passes `calcSpellDuration(...)` unmodified to
  `AddConditionMagnitude(conditions.ConditionIdMinorShield, duration, ...)` as the trigger
  count (record 119 ticks once a round, so triggers and rounds coincide).
- **Heal: `/2`, floored at 6.** `applySpellHeal` (`spell_help_effects.go`)
  computes `calcSpellDuration(...) / 2`, then clamps `durationRounds < 6` up
  to 6, before
  `AddConditionMagnitude(conditions.ConditionIdRegenerating, durationRounds, regenMult, ...)`.
- **DoT: `/3`, floored at 3.** `applySpellDot` (`spell_effects.go`) computes
```

with:

```markdown
- **Shield: full duration, no divisor.** `applySpellShield`
  (`spell_help_effects.go`) queues the spell's own ward
  (`spellWardConditionId`) with `calcSpellDuration(...)` unmodified as the
  trigger count (every ward ticks once a round, so triggers and rounds
  coincide).
- **Heal: the heal's own duration, scaled by the caster.** `applySpellHeal`
  (`spell_help_effects.go`) queues the spell's own heal
  (`spellHealConditionId`) for `healTriggers(conditionId, willpower,
  skill)` rounds: the heal's authored `triggercount` (its duration for an
  untrained caster of willpower 100) times `spellDurationTerm(skill,
  willpower) / spellDurationTerm(0, 100)`, at least one. `spellDurationTerm`
  is `calcSpellDuration`'s caster term, `10 + willpower/20 + skill/2`
  (messaging M6 slice 1, owner ruling R9).
- **DoT: `/3`, floored at 3.** `applySpellDot` (`spell_effects.go`) computes
```

Replace in `internal/hooks/context.md` (3 of 4):

```markdown
`applySpellConditionEffect` (every named condition through
`applySpellCondition`'s event door), `applySpellHeal` (a Regenerating
record), `applySpellShield` (a Minor Shield record) and `applySpellPurge`
(cancels every poison). `applySpellDefaultEffect` serves an effect with no
```

with:

```markdown
`applySpellConditionEffect` (every named condition through
`applySpellCondition`'s event door), `applySpellHeal` (the spell's own heal
condition), `applySpellShield` (the spell's own ward) and `applySpellPurge`
(cancels every poison). `applySpellDefaultEffect` serves an effect with no
```

Replace in `internal/hooks/context.md` (4 of 4):

```markdown
round); `TestItemProcParity` holds the tree path to it.

```

with:

```markdown
round); `TestItemProcParity` holds the tree path to it.

## Casters, start lines and DoT credit (messaging M6 slice 1)

`ApplyConditions` stamps every landed record with the event's `Source` and
`Caster` (`conditions.Conditions.Stamp`). Before the add it reads the
record's family rivals (`familyRivalNames`); after it, the holder and the
room read `narrateFamilyReplacement`'s line ("Your Conviction Ward fades as
Conviction Bulwark takes hold.") in place of the replaced record's end line.

`narrateConditionStart` (`condition_cast_lines.go`) tells one start line per
audience: `start_actor` to a caster who is someone else and online,
`start_actee` to a player holder, `start_observer` to the room, which
excludes both and skips `startUnseenBy` (#458; the replacement line skips it
too). `conditionPartyOf` resolves the holder and the caster, a mob through
`conditionMobNames`; `conditionParty.namesFor(viewer)` hides a hidden mob
from a participant who does not perceive it and names it to one who does,
as a spell's own lines do. Each private line
hides the other party's name at that reader's sight
(`conditionReaderSight`: the pre-landing snapshot for a darkness, else
`ParticipantSight`). `critMarker` rides the caster's line, which on a
self-cast is the holder's.

`applySpellConditionEffect`, `applySpellShield` and `applySpellHeal` drop
their generic lines when `spellConditionNarratesStart` holds: a fresh landing
of a condition whose own start lines tell every audience
(`ConditionSpec.NarratesCastStart`). A silent condition, or a re-cast of one
already held, keeps them (owner ruling R11).

The round ticks harm in the record's caster's name:
`ApplyHarm(pool, amount, condition.Caster)` for health, stamina and
conviction, player and mob. A mob's health tick also credits the caster in
its damage map through `creditMobHarm`, `creditSpellDamage`'s rule by ref (a
player, or a mob charmed by one). The player need not be online: the damage
map and the killer ref are keyed by id. The zero-health backstops stay
anonymous: a tick routes through `ApplyHarm`, which queues an attributed
death and sets `DeathQueued`, so no tick reaches them.

```

- [ ] **Step 2: Patch notes and the README row.**

Replace in `docs/PATCH_NOTES.md` (1 of 1):

```markdown
# DOGMud Patch Notes

```

with:

```markdown
# DOGMud Patch Notes

## Unreleased: Wards, heals and who cast them

- There are now three wards, and each says what it stops. Conviction Ward
  turns aside physical blows only. The new Conviction Bulwark also blunts
  spells. Chrysalis Cocoon, the strongest, blunts blows, spells and harsh
  words alike. Type `conditions` to see which ward is on you.
- You hold one ward at a time. Casting a different ward replaces the one
  already there, and you are told which gave way. Potions, armor and
  mutations that protect you still add on top of a ward.
- Every healing spell now works over time, and each has its own pace. Mend
  Flesh is gentle and long. Mend Wounds is strong and short. Of the group
  heals, Mend All is light and short, Communion of Flesh is steady and
  long, and Mass Mend is the strongest and short. You hold one healing
  spell at a time, as with wards; salves, potions and food still add on
  top.
- A spell that lays an effect on someone now gives one clear line to each
  person there: the caster, the target and the room each read their own
  line, written for that effect and naming the caster.
- A poison or bleed that kills now counts as your kill, even if you have
  walked away, logged out or died since you cast it. It counts for
  reputation, crimes and bounties as any other kill would.
- If you use a trigger that watches for "Your Minor Shield dissipates.",
  change it: that line is now "Your Conviction Ward fades." A web client
  trigger on the status "Minor Shield" should now look for the ward's name.

```

Replace in `docs/README.md` (1 of 1):

```markdown
| [`superpowers/specs/2026-10-07-messaging-m6-slice1-casters-shields-heals-design.md`](superpowers/specs/2026-10-07-messaging-m6-slice1-casters-shields-heals-design.md) | Design for Messaging M6 slice 1 (#370 rows 1 and 2, #338, #240). Conditions record their caster, so a DoT kill credits the caster wherever they are, and a spell-applied condition prints one line per audience from its own data (new `start_actor`, `{actor}` in the target and room lines). A new condition `family` makes the newest ward or heal replace the older one while potions, items and mutations still stack. Three wards (Conviction Ward physical, new Conviction Bulwark physical and spell, Chrysalis Cocoon all three) and six heals each land their own condition; "Minor Shield" is retired and help text is made true |
```

with:

```markdown
| [`superpowers/specs/2026-10-07-messaging-m6-slice1-casters-shields-heals-design.md`](superpowers/specs/2026-10-07-messaging-m6-slice1-casters-shields-heals-design.md) | Design for Messaging M6 slice 1 (#370 rows 1 and 2, #338, #240). Conditions record their caster, so a DoT kill credits the caster wherever they are, and a spell-applied condition prints one line per audience from its own data (new `start_actor`, `{actor}` in the target and room lines). A new condition `family` makes the newest ward or heal replace the older one while potions, items and mutations still stack. Three wards (Conviction Ward physical, new Conviction Bulwark physical and spell, Chrysalis Cocoon all three) and six heals each land their own condition; "Minor Shield" is retired and help text is made true |
| [`../spell_condition_data_guard_test.go`](../spell_condition_data_guard_test.go) | Messaging M6 slice 1 content guard over the shipped spells and conditions: every spell's `condition_ids` is read by its effect type (no dead data like Chrysalis Cocoon's old `[52]`), each shield and heal spell lands its own ward- or heal-family condition reading its magnitude, the three wards and five heals hold their owner-ruled ordering, and every spell-landed condition tells its own start (`start_actor`, `start_actee`, `start_observer`) so a spell never falls back to its generic trio |
```

- [ ] **Step 3: Audit the docs.**

Run: `python tools/context_md_audit.py`
Expected: the same list as on master; none of the packages above is newly listed.

- [ ] **Step 4: Run the root guards.**

Run: `go test . -count=1`
Expected: PASS. `ok`.

- [ ] **Step 5: Commit.**

```bash
git add internal/conditions/context.md \
  internal/characters/context.md \
  internal/events/context.md \
  internal/users/context.md \
  internal/mobs/context.md \
  internal/actions/context.md \
  internal/behaviortree/context.md \
  internal/combat/context.md \
  internal/hooks/context.md \
  docs/PATCH_NOTES.md \
  docs/README.md
git commit -F - <<'EOF'
docs: messaging M6 slice 1 context, patch notes and guard row

context.md for conditions, characters, events, users, mobs, actions,
behaviortree, combat and hooks; the player-facing patch note, including
the Minor Shield trigger warning; and the README row for
spell_condition_data_guard_test.go.

Refs #370, Refs #338, Refs #240.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---
### Task 13: Whole-tree verification and boot check

- [ ] **Step 1: Format.**

Run: `gofmt -l ./internal ./modules *.go`
Expected: no output.

- [ ] **Step 2: Build and vet.**

Run: `go build ./... && go vet ./...`
Expected: no output.

- [ ] **Step 3: Full suite.**

Run: `go test ./... -count=1` (a few minutes)
Expected: every package `ok`. A failure is a finding: fix it in the task it belongs to.

- [ ] **Step 4: Lint, new issues only.**

Run: `golangci-lint run --new-from-rev=$(git merge-base master HEAD) ./...`
Expected: `0 issues.`

- [ ] **Step 5: No dashes in what the branch added.**

Run (on its own line; zero matches expected): `git diff $(git merge-base master HEAD) -- '*.go' '*.yaml' '*.template' '*.md' | grep '^+' | grep -nP '[\x{2013}\x{2014}]'`

Diff against the merge base, not `master`: master moves while the branch is open, and a plain `git diff master` reports its new lines as yours (it did, once, in this plan's dry run).
Expected: no output.

- [ ] **Step 6: The config bit.**

Run: `git diff $(git merge-base master HEAD) --stat -- _datafiles/config.yaml`
Expected: no output (no task touches it).

- [ ] **Step 7: Boot check** (`dogmud-shipping`, step 6 of the pre-push SOP), on ports clear of the owner's server. Write a scratch override file holding

```yaml
Network:
  TelnetPort: [43333]
  LocalPort: 19999
  HttpPort: 18090
```

then, in a detached worktree of the branch head: `go build -o boot-check.exe .` and `CONFIG_PATH=<that file> timeout 180 ./boot-check.exe > boot.log 2>&1`.
Expected: exit 124 (the server stayed up); `grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log` prints 0; `grep -c "Server Ready" boot.log` prints 1; the log's `conditionSpec.LoadDataFiles()` line reads `loadedCount=128` (120 on master plus 135 to 142). Kill nothing by name: the timeout ends the process. Remove the worktree afterwards.

---

### Task 14: Playtest (executed by the controller, not a subagent)

Player-facing content changed (condition and spell YAML, help pages), so the slice closes with the adversarial playtest the arc spec requires. Run it against a local build of the branch with the harness, following `dogmud-playtesting` (wipe instance saves with the server down; kill only your own server, by PID). Harness lessons from recent runs are in memory (`project-sight-followups-playtest-harness-lessons-2026-10-09`). Goals, not mechanics:

- **DoT credit (#240).** A player casts a poison (or opens a bleed with a combat move) on a mob in a dark room, walks out and logs out before it dies. Back in: the kill shows in faction reputation and, if a guard saw the assault, as a crime; the kill count and quest credit do not move (D5, by design this slice). Repeat with the caster still online in the next room: kill count and quest credit move too.
- **One line per audience (#370 row 1).** Cast a buff (Iron Will) and a harmful condition spell (Mind Fog) on another player with a third watching, lit and at shapes (heat sight in the dark): the caster, the target and the watcher each read exactly one line; at shapes the names read "a figure". A crit Mind Fog carries `[CRIT!]` on the caster's line only. A self-cast reads one line to the caster and one to the room.
- **Wards (#338).** Cast Conviction Ward, then Conviction Bulwark, then Chrysalis Cocoon on one target: each new ward replaces the last with "Your X fades as Y takes hold."; `conditions` names the ward and its description says what it blocks. Against each ward, take a physical blow, a spell (a mental damage spell) and a taunt (social): each ward reduces exactly what it lists. Drink a Mindshield Elixir over the Bulwark: both show in `conditions` and both protect.
- **Heals (#338).** Cast Mend Flesh, then Mend Wounds: the second replaces the first, announced. Apply a Healing Salve over a heal: both stay. Watch a fight: Mend Wounds closes wounds fast and ends soon; Mend Flesh heals slowly for longer.
- **The mob AI.** A ward-casting mob (Temple Priest Seren, mob 344) does not recast a ward while one is up, and casts one when it lapses.
- **Help.** `help wards`, `help healing-spells`, `help conviction-bulwark`, `help mass-mend` read true and fit 80 columns.

File every finding as a GitHub issue on `pruuk/DOGMud` (`--repo pruuk/DOGMud`; exploit-class leaks stay out of public issues). The PR body lists #240, #338 and the two ledger rows of #370 as resolved in words, without a closing keyword.

---

## Integrated dry run

The plan was dry-run twice, the second time in full on master `64105e41b` after #462 landed (see "How this plan was verified").

- **From this document.** A script parsed this file and applied Tasks 1 to 12 in order to a fresh `64105e41b` worktree: all 213 "Replace" blocks matched exactly once at their turn, the 29 Create and whole-file blocks wrote, the one `git mv` ran, and each task's commit block staged and committed cleanly (a clean `git status` after every commit, so each task's `git add` list is exactly its changes). Every "Expected: FAIL" run failed (15 of 15) and every "Expected: PASS" run passed (25 of 25); `go build ./...`, `go vet ./...` and `gofmt -l` were clean after every task, and `go test . -count=1` passed at the end of each. The resulting tree is byte-identical (`git diff`) to the tree the tasks were first built in.
- **The failures were the stated ones.** Spot-checked in the log: Task 6's see-hidden caster read no caster line and the room line went to the caster; Task 8's audiences each read two lines and the event carried a zero caster; Task 9's hooks build stopped at `undefined: conditions.ConditionIdConvictionWard`; Task 10's at `undefined: healTriggers`; Task 11's guard listed all 20 conditions.
- **Whole tree (Task 13).** `gofmt -l` printed nothing; `go vet ./...` was clean; `go test ./... -count=1` passed with 132 packages `ok` and none failing; `golangci-lint run --new-from-rev=64105e41b ./...` reported `0 issues.`; the dash scan of the added lines found nothing; `config.yaml` untouched; `tools/context_md_audit.py` printed the same list as master. Boot check on ports 43333/19999/18090: exit 124, 0 panics, 1 `Server Ready`, `loadedCount=128` conditions.

The dry runs forced these corrections, all folded into the tasks above (no design change beyond D7's #458 clause):

- **#462 landed mid-plan.** It rewrote the start block Task 6 replaces. Task 6 now passes `startUnseenBy` into `narrateConditionStart` and `narrateFamilyReplacement`, names a mob holder through `conditionMobNames` and hides it per reader in `namesFor` (`mobPlainName` no longer exists), and re-keys the guard from `|118,120,122` to `|122,124,126`. Task 9's hidden-mob shield case now expects silence for a reader who does not perceive the holder, not "something".
- **Task 6:** the first cut sent the room line to the caster too; the room now excludes the caster (`TestConditionCast_NoCasterLineAuthoredTellsTheCasterNothing` caught it). A see-hidden caster read "something" for a hidden mob holder; `conditionParty.namesFor` renders per reader.
- **Task 7:** `TestSpecialMoveAdmissionOrdering` pins call identities, and the apply-path guard's pattern could not see `AddConditionMagnitudeBy`; both updated in the task. The item proc's doc comment keeps its line count so the guard's `actions_item_proc.go` keys stay put.
- **Task 9:** a shield spell that names no ward still lands 119, so its trio decision reads that ward (`spellConditionNarratesStart(c, wardId)`), not `condition_ids`; the parity fixture now drains leftover condition events, or one test's queued ward landed in the next; the narration builder gained `start_actor` rows.
- **Task 11:** Empathic Shroud's YAML comment carried a dash and named the old key `end_user_text`; fixed in place.
- **Task 13:** the dash scan and lint diff against the merge base (above).

## Self-review against the spec

- **Section 1, the caster:** Task 1 (record, persistence), Task 5 (event and doors), Task 6 (stamped on every door, newest owns), Task 7 (ticks and credit; backstops per D3; consumers per D5). R10's offline caster: recorded for rep, crime and bounty; the consumers that need a live user are named in D5, not silently skipped.
- **Section 2, one line per audience:** Task 3 (`start_actor`, `{actor}`, `NarratesCastStart`), Task 6 (the hook, the crit marker, the darkness snapshot judging every line), Task 8 (the spell drops its trio; silent and re-cast keep it), Task 11 (all 20 conditions authored, guarded).
- **Section 3, families:** Task 2 (field, rule, rivals), Task 6 (replacement line), Task 9 (AI ward family), Task 10 (heal family, AI heal branch landed in Task 9's AI change).
- **Section 4, wards:** Task 4 (effect kinds), Task 9 (three wards, 119 renamed, cocoon's `[52]` removed, numbers in D10). **Section 5, heals:** Task 10 (six heals, 32 and 33 in the family, 120 out, mass-mend's `[33]` removed, numbers in D11).
- **Section 6, help and copy:** Tasks 9 and 10 (pages true, `help wards`, `help healing-spells`), Task 12 (patch notes with the trigger warning). **Section 7, display:** no web change; GMCP `Type` now shows the real source because every door stamps it (Task 6).
- **Testing section:** caster threading and persistence (Tasks 1, 5, 6); credit with the caster present, away, offline and a gone mob (Task 7); lines, crit, hidden names, trio kept (Tasks 6, 8); families with potion and salve (Tasks 2, 4, 9, 10); each ward's kinds (Tasks 4, 9); heal identities (Task 10); the dead-data guard (Tasks 9 to 11); boot and gates (Task 13).
- **Placeholders:** none; every step carries its code or its command. **Names:** `Stamp`, `HasFamily`, `FamilyRivals`, `NarrateCast`, `NarratesCastStart`, `StartActorNotice`, `QueueCondition`, `AddConditionMagnitudeBy`, `ActorRefOf`, `creditMobHarm`, `healTriggers`, `spellDurationTerm`, `spellWardConditionId`, `spellHealConditionId`, `spellConditionNarratesStart`, `landQueuedConditions` are each defined in exactly one task and used only after it.
- **Out of scope, per spec:** quest Actee text (#375), ledger rows 36 to 59, reflect riders, the per-tick heal pool, a mob caster that survives a restart.
