# Combat-message reachability slice 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Every subagent brief carries these rules:** edit repo files with the Edit and Write tools only (`~/.claude/hooks/guard.py` H3 blocks shell writes into a git working tree; a generated file is written to the session scratchpad and `cp`'d into place); `git add` named paths only; never `--no-verify`; no em or en dashes in prose, comments or player text; no colon inside an unquoted YAML value; every `gh` command carries `--repo pruuk/DOGMud`; scratch output goes to the session scratchpad by its full path, never `$TMP` or `/tmp`; read files in chunks of at most 400 lines. Any change to world data or a combat-message file runs the FULL `go test . ./internal/... ./modules/...` before its commit.

**Goal:** Every combat line a creature can be sent is reachable and drawn by one rule: engage, wait and swing lines all come from the same weapon type (weapon, else species attack, else bare hands); what can never render is retired; bears maul and cats pounce in ordinary combat; and which special moves a species has is species data, including the smoke elemental's new `smother`.

**Architecture:** One function, `combat.AttackSubtype`, holds the weapon-type rule and every reader calls it. The engage moment is one coordinated draw from the `prepare` pool, sent by the attack commands in place of their hand-written lines. The `separate` split (cross-room blows) leaves the store, its type, its decoder and both render branches, because round combat releases a target in another room before any line is drawn. `shooting.yaml` keeps only `prepare` and `wait`, with the validator exemption read from one function `meleeDisplaySubtype` also reads. A new `beast_moves` species field, checked at load against each move's body requirement, is the only thing the `CanUse*`, readiness and action-entry gates read. `maul` and `pounce` become natural-attack subtypes wired in every place the slam split wired `trample`. `smother` is a mob-only special move built on the throttle model, sharing throttle's cast-interrupt contest through one extracted helper.

**Tech Stack:** Go (`internal/combat`, `internal/items`, `internal/species`, `internal/actions`, `internal/mobcommands`, `internal/usercommands`, `internal/behaviortree`, `internal/actionspec`, `internal/costs`, `internal/conditions`), YAML world data (`_datafiles/world/dogmud/`), root-package guards, the narration store goldens, the playtest harness.

**Spec:** `docs/superpowers/specs/completed/2026-10-16-combat-message-reachability-design.md` (#541, #539; owner rulings A to H; Appendix A). Section 0 (slice 1) shipped in #568 and is not planned here. The slam split (#569, PR #571, `docs/superpowers/specs/completed/2026-10-16-slam-split-design.md`) merges first; this plan is written against its HEAD.

---

## Facts verified against source (feature/slam-split HEAD `4a42e28d5d8653ccb0dc63857981438255d152f8`, 2026-10-16)

Every row was read from the worktree `C:/tmp/dogmud-slam` at that SHA. "Spec" marks the spec fact a row re-verifies; DRIFT or WRONG says what changed.

| # | Fact | Source | Spec fact |
|---|---|---|---|
| 1 | `items.Prepare` is never requested in production: `grep -rn "items\.Prepare\b" --include=*.go . \| grep -v _test` returns nothing, and the same pattern does hit `internal/narration/snapshot_test.go:344`, so it can match. `func GetPreAttackMessage(subType ItemSubType, messageType Intensity) AttackOptions` is `internal/items/attack_messages.go:290`. Production callers: `internal/combat/combat.go:218` (in `GetWaitMessages`), `internal/combat/combat_helpers.go:1737` (Fumble) and `:1747` (CoupDeGrace). `GetWaitMessages` has one caller, `internal/hooks/NewRound_DoCombat_resolution.go:92`, with `items.Wait` | grep, file | 1: holds; DRIFT fumble/coup lines 1736,1746 are now 1737,1747 |
| 2 | 23 files in `_datafiles/world/dogmud/combat-messages/` (bite, blast, bludgeoning, claws, cleaving, generic, gore, lash, maul, pounce, sceptre, shooting, siphon, slam, slashing, stabbing, staff, sting, throttle, trample, unarmed, wand, whipping); every one has `prepare` and `wait` (per-file `grep -E "^  [a-z]+:"`). Only `generic.yaml` adds `coupdegrace`. `maul.yaml` and `pounce.yaml` have all eight required intensities, 345 role lines each, tiers beginner/expert/master everywhere | grep | 2: DRIFT, 23 files, not 20 |
| 3 | Hand-written engage lines: `internal/usercommands/attack.go:286-289` attacker line (inside `if engaged`, `HideNames` at `attackerSight`), `:294-298` room line (`room.SendTextVisual`, gated only by `!isSneaking`, NOT by `engaged`), PvP `:379-382` attacker, `:385-388` target and `:390-393` room (both under `!isSneaking`; the room line tags a PLAYER target with `mobname`); `internal/mobcommands/attack.go:98-100` victim line, `:102-104` room line, `:151-154` mob-vs-mob room line. All on `messaging.CategoryHitMelee` | grep `mortal combat`, `prepares to fight` | 3: holds (lines refined) |
| 4 | `func GetWaitMessages(stepType items.Intensity, sourceChar *characters.Character, targetChar *characters.Character, sourceType SourceTarget, targetType SourceTarget) AttackResult` at `combat.go:214`; pool from `sourceChar.Equipment.Weapon.GetSpec().Subtype` at `:218`; category `CategoryHitMelee` unless armed, `:305-308`. Wait rounds run in `handleCombatWaitRound` after `ConsumeRoundWaiting` (`NewRound_DoCombat_resolution.go:86`). NOT IN SPEC: `GetWaitMessages` has its own `separate` path (exit tokens `:250-273`, `Separate.Render` `:292-296`, `RemoteRoom` `:342-346`) | file | 4: holds; spec missed the wait separate path |
| 5 | `func buildWeaponSetup(sourceChar *characters.Character, targetChar *characters.Character, weapon items.Item, idx, total int) weaponSetup` at `combat_helpers.go:430`: starts at `items.Unarmed` (`:443`), takes species `NaturalAttack` (`:455-457`), then the passed weapon's subtype (`:459-462`). It is called PER WEAPON SLOT (`:323`, from the list built at `:320-395`, empty slots as `items.Item{ItemId: 0}`), so the rule must take a slot, not only the main hand. NOT IN SPEC: a fourth reader, the "too entangled to defend" lines, picks their category from the main-hand weapon only (`combat_helpers.go:686-689`) | file | 5: holds; per-slot detail and fourth reader added |
| 6 | `func meleeDisplaySubtype(weaponSubType items.ItemSubType, weaponReach, posRadius float64) items.ItemSubType` at `combat_helpers.go:1584` (Shooting to Bludgeoning unconditionally `:1585-1587`, bladed to Bludgeoning when `ShouldBludgeon`). One caller, `:1730`. Fumble and coup de grace read `displaySubtype` from it (`:1737,1747`), so they reach the rule through `ws.weaponSubType`. `narration/special-moves/shoot.yaml` exists | file | 6: DRIFT line 1583 is now 1584 |
| 7 | Round combat releases a target in another room: `internal/hooks/NewRound_DoCombat_unified.go:220`, in `resolveCombatTarget`, which `handleCombatRound` calls (`:66`) BEFORE `phase1WaitRound` (`:79`) and the swing, so neither the wait nor the swing path ever renders `separate`. Swing separate branch: `combat_helpers.go:1829-1867` (exit tokens `:1831-1850`, render `:1858-1862`, defender-room line `:1867`, carried through `:1887-1927` and sent `:1986-1988`). 17 `separate:` blocks in dogmud: `generic.yaml` 9, `shooting.yaml` 8 (`grep -c "separate:"`). The default world has 14 more, but its files use the role-first layout the decoder already rejects (`_datafiles/world/default/combat-messages/generic.yaml:41`), so the default world's combat messages cannot load today; this plan leaves the default world alone | file, grep | 7: DRIFT lines 219,1828,1860 are 220,1829,1861 |
| 8 | `validNaturalAttacks` is `internal/species/species.go:214-224`: bite, claws, slam, gore, sting, siphon, trample, lash, blast | file | 8: DRIFT, nine entries, not six |
| 9 | `func CategoryForWeaponSubtype(sub items.ItemSubType) messaging.Category` at `internal/combat/attackresult.go:261`: claws, bite, sting, gore to `CategoryHitNaturalSharp` (`:265-266`); slam, trample, lash to Blunt; blast with the casters | file | 9: holds |
| 10 | A natural attack is wired in six places, per the slam split and `internal/species/context.md:136-155`: subtype constant (`internal/items/itemspec.go:182-184` for trample, lash, blast; siphon `:181`) and description (`:108-111`); `validNaturalAttacks` (`species.go:221-223`); `CategoryForWeaponSubtype` (`attackresult.go:263,271`); `naturalWeaponPools` (`internal/items/natural_weapon_agreement_test.go:16`); `DefaultReachForSubtype` (`internal/items/reach.go:23`, claws `:17-18` at 0.15); plus the comment list in `meleeDisplaySubtype` (`combat_helpers.go:1581-1583`) | file | 10: DRIFT siphon constant 178 is now 181; reach and pools are two more sites |
| 11 | Gates in `internal/combat/ai.go`: `CanUseBash` 291, `CanUseTrip` 312, `CanUseKick` 328, `CanUseGrapple` 340, `CanUseSubmit` 360, `CanUseHamstring` 385 (reads `sp.NaturalAttack` INLINE at `:400`, not a predicate), `CanUseRake` 529, `CanUseMaul` 555, `CanUsePounce` 577, `CanUseGore` 674, `CanUseDrain` 702 (no hands check), `CanUseCast` 728, `CanUseThrottle` 818. Predicates: `SpeciesNaturalAttack` 248, `SpeciesIsFanged` 254, `SpeciesIsClawed` 257, `SpeciesIsHorned` 260, `SpeciesHasLifeDrain` 263, `SpeciesIsQuadrupedPredator` 569. Readiness, `internal/actions/command_readiness.go`: rake 103-104, maul 106-107, throttle 109-110, pounce 112-114, gore 116-117, hamstring 119-129, drain 131-132. Action entries: `combat_rake.go:83`, `combat_maul.go:83`, `combat_throttle.go:95`, `combat_gore.go:78`, `combat_hamstring.go:92` (inline `sp.NaturalAttack`, not a predicate), `combat_pounce.go:93`, `combat_drain.go:96` | grep | 11: PARTLY WRONG, hamstring reads the family inline in both ai.go and its action; the spec omits the pounce and drain entries |
| 12 | `charge` has no AI gate and is mob-only (`internal/actions/divergences.go:188`); scripted on the boar `mobs/cascade_pass_road/9539-a_tusked_forest_boar.yaml:25-28` | file | 12: holds |
| 13 | Natural attacks by species count: bite 10, claws 9 (11, 22, 25, 30, 34, 3, 44, 5, 9), slam 2, gore 2, siphon 2, trample 1, lash 2, blast 7. Mobs (`grep -rlE "^\s*speciesid: N\s*$" mobs`): bears 2 (9541 The Pass-Apex, 9547 The Reach-Alpha), cats 5 (9540, 9544, 9549, 202 Feral Cat, 258 farm cat), serpents 8 | grep | 13: DRIFT claws 9 not 7, slam 2 not 12, cats 5 not 4, serpents 8 not 19 |
| 14 | WRONG in spec section 5: BOTH bear mobs already script `charge` (and `bash`): `9541-the_pass_apex.yaml` and `9547-the_reach_alpha.yaml` combatcommands. The "add charge to the two bear mobs" step is already done | file | section 5 bears |
| 15 | WRONG in spec section 5 table: the smoke elemental (44) is not "none today". It is `claws`, `body_parts: []`, so no hands, and `CanUseRake` passes today; no legs, so no pounce or hamstring. Today: rake | data + ai.go:529 | section 5 row 44 |
| 16 | WRONG in spec section 5: "no hands for every beast move" cannot hold for drain. The vampire (34) has `[arms, hands, legs, eyes, mouth, skin]` and `lifedrain: true`; `CanUseDrain` has no hands check (`ai.go:702-707`) and `internal/combat/context.md:876-881` records "drain is EXEMPT" from the hands rule. Drain's body requirement is lifedrain only | data, file | section 5 rule |
| 17 | `Species` struct `species.go:31-73` has no move list; loader `LoadDataFiles` `:261-286` panics on `validateNaturalAttack`, `validateGoreHasHorns`, `validateSiphonHasLifeDrain`. `characters.Character.HasBodyPart` (`internal/characters/anatomy.go:9-20`) returns false for a nil species or nil body list, while `Species.HasBodyPart` (`species.go:318-333`) fails open; the validator must read the list the way the runtime gate does | file | added |
| 18 | Predicate callers, production (21): `SpeciesIsFanged` ai.go:562,570,825, command_readiness.go:107,110,129, combat_maul.go:83, combat_throttle.go:95; `SpeciesIsClawed` ai.go:536,570, command_readiness.go:104,129; `SpeciesIsHorned` ai.go:681, command_readiness.go:117, combat_gore.go:78; `SpeciesIsQuadrupedPredator` ai.go:584, command_readiness.go:114, combat_pounce.go:93; `SpeciesHasLifeDrain` ai.go:706, command_readiness.go:132, combat_drain.go:96. Tests: `internal/combat/ai_test.go:605-614` (8 calls), `internal/actions/command_readiness_drift_test.go:172-191` (6 string literals). No validator or mutation code calls them (the spec's "other callers keep their meaning" has no subject). After section 5, none has a production caller. `SpeciesNaturalAttack` (`ai.go:248`) is called in production only by three of them (`ai.go:255,258,261`); its one test mention is a test NAME, `TestBuildWeaponSetup_UsesSpeciesNaturalAttack` (`combat_helpers_test.go:13`), not a call. D6 deletes all six. Docs naming them: `internal/actions/context.md:220-221`, `internal/combat/context.md:832-836`, `internal/mobcommands/context.md:41-42`, `internal/usercommands/context.md:75-76` (re-grepped on master `8e4727129`) | grep `\b(SpeciesIsFanged\|...)\(` | added; affects spec section 4 |
| 19 | Test fixtures that seed a beast species (`NaturalAttack: items.(Bite\|Claws\|Gore)` or `LifeDrain: true`), by file: actions/combat_drain_test 4, combat_gore_test 3, combat_hamstring_test 1, combat_maul_test 4, combat_pounce_test 5, combat_rake_test 4, combat_test 4, combat_throttle_test 6, command_readiness_drift_test 8; behaviortree/actions_test 1, archetype_predator_beastmove_test 1; combat/ai_test 30, combat_helpers_test 2; mobcommands/predator_test 1; species/species_siphon_test 1, species_test 4; usercommands/special_move_admission_side_effect_test 4 | grep | added |
| 20 | `SeparateMessages`: type `attack_messages.go:56-61`, `Render` `:173-190` (only user of the `exit` import, `:8`), validator branch `:260-272`, `anyPool` `:281-284`; decoder `internal/items/attack_records.go:23,133-141`; renders `combat.go:292-296`, `combat_helpers.go:1858-1862`. Test users: `combat_banner_agreement_guard_test.go:135-138,170,203-237`; `internal/combat/coup_de_grace_test.go:104-105`; `internal/combat/wait_remote_room_hiding_test.go` (whole file); `internal/items/attack_messages_test.go:74-95,136-148,237-260`; `internal/items/attack_messages_vertical_test.go` (whole file); `internal/items/attack_records_test.go:31,48,67,109-116`; `internal/items/test_helpers_combat.go:102-107`; `internal/narration/snapshot_test.go:340,363-371,431-450`; `messaging_surface_guard_test.go:158` (comment). `remote_observer` stays in use by the movenarration store (`internal/movenarration/store.go:76`, shoot) | grep `\.Separate\b\|SeparateMessages\|ToDefenderRoom\|remote_observer` | added (spec risk row 3) |
| 21 | No `together` line in dogmud combat messages uses `{exitname}` or `{entrancename}` (awk over together blocks of generic.yaml and shooting.yaml: 0); the tokens appear only in `separate` blocks and in each file's header comment | awk | added |
| 22 | Throttle model: `internal/actions/combat_throttle.go` (`ExecuteThrottle` 78-237: anatomy gate 95, `SpecialMoveReady` 99, `admitFullCost` 103, `ClaimSpecialMove` 107, `ExecuteSkillMove` 117-134, bleed 146, `AddCondition(89, false)` 149, cast-interrupt contest 159-197, `RecordSpecialMove` 214, `SetRoundsWaiting(1)` 217-219, `AwardResolved` 226); `internal/mobcommands/throttle.go` (hit 78, cast_interrupt 90 on `CategorySpellDisruption` with `SoundChantBreaksOff`, partial 97-112 with defence label `"throttle lunge"`, miss 119); registry `mobcommands.go:73`; player twin `usercommands/throttle.go`; `narration/special-moves/throttle.yaml` events hit, cast_interrupt, partial, miss; `ClaimSpecialMove` `internal/actions/special_move_cooldown.go:38`; every `CanUse*` refuses on `Cooldowns["special-move"]` | file | added |
| 23 | Wiring lists a special move appears in: `actionspec/action.go:48,125`; `costs/action.go:46`; `costs/action_test.go:38`; `actions/action_readiness.go:29-33` `specialMoveVerbs`; `actions/action_cost_test.go:38`; `behaviortree/actions_mob.go:237-245` `beastMoves`; `mobcommands/command_parity_test.go:25`; drift test `TestSpecialMoveAdmissionOrdering` 149-322, `TestSpecialMoveWrapperAdmission` 595-700 (mob list 597), `runExecuteAndReadExecuted` 723-754, `runExecuteAndReadFlag` from ~1460, `TestSpecialMoves_NoProgressionAwardSitsInsideAHitGate` file list 1648-1652; `move_narration_migration_guard_test.go:48-77`; `shipped_narration_data_guard_test.go:1124-1139`; `condition_apply_path_guard_test.go` allowlist (throttle 117 and 255) | grep | added |
| 24 | Next free condition id: 144 (`python tools/id_inventory.py --alloc conditions 1` prints `reserve IDs 144-144`; gaps 4, 8, 10-14, 16-23 exist and are not reused). `grep -rniE "(conditionid\|conditionids)[^0-9]{0,20}\b144\b" _datafiles internal modules` finds nothing. Blinded is `perception.ConditionIdBlinded = 3` (`internal/state/perception/transitions.go:31`), read by `HasAnyBlindSource` (`internal/characters/sight.go:47-58`); 3's own start lines speak of a spell, so smother must apply it silently and narrate it. Throttled 89: `triggerrate: 1 round`, `triggercount: 3`, `tick_pool: stamina`, `tick_percent: -0.06`, `tick_min: 2`, `silent-start`. `_datafiles/config.yaml` carries only throttle's bleed knobs (842-844), no Throttled tick knob, so Appendix A's Smothered values match 89 exactly and need no config key | file, tool | Appendix A |
| 25 | Special-move store guards: `TestMoveEventKeysAgree` (`move_narration_migration_guard_test.go:461`) fails an authored event no Go site names and a named event the YAML lacks; mob-only moves author no `actor` role (`charge.yaml`, `hamstring.yaml`; guard comment 566-568). Appendix A's smother copy authors `actor` lines for every event, which a mob-only move can never send | file | Appendix A |
| 26 | File-count guards and throttle.yaml references: `bannerGuardFileCount = 23` (`combat_banner_agreement_guard_test.go:24`); `internal/narration/snapshot_test.go:12` comment; `combat_messages.golden` header (23 files, throttle listed); `naturalWeaponSpecialPools = {"throttle", "maul"}` (`natural_weapon_agreement_test.go:20`); `shipped_narration_data_guard_test.go:911` comment. The banner guard fails a file holding no banner record (`combat_banner_agreement_guard_test.go:162-164`), which a prepare-and-wait-only shooting.yaml would be. `grep -rn "throttle" internal --include=*.go` finds no code that requests a `throttle` subtype | grep | 3, added |
| 27 | Scripted beast moves in dogmud combatcommands (18): hamstring on wolves 9538, 9543, 9548, 205, 206, 215, 223 (canine) and cats 9540, 9544, 9549 (feline); drain on summon 304 (vampire); charge on boars 9539, 9545, 207, 208, 216 and both bears. Every one is in its species' Proposed list or is charge. No behaviour archetype names a beast move | YAML walk | 12 |
| 28 | `SelectSpecialMove` keeps the HIGHEST weighted score (`ai.go:190-201`); an unweighted move gets 10. Cats run `ambush_predator` (`ai.go:88-93` on master `8e4727129`: pounce 45, rake 40, hamstring 20, trip 10, no throttle), so throttle (base 50) would score 5 against rake's 18 to 24: cats would essentially never throttle without a profile weight. D4 leaves the profile alone (#574) | file | ruling F |
| 29 | Once cats use `pounce`, `TestNaturalWeaponPoolsNeverPutAVerbAfterTheWeapon` covers `pounce.yaml` and fails on `:240-242` (normal/master: "Your {itemname} flash") and `:285` (heavy/master: "drive your {itemname} forward", a false positive: `forward` is missing from `weaponNotSubject`). Replicated in scratch; the probe "Your {itemname} bites into you!" is caught, so the check can fail. `maul.yaml` passes | script | 4 |
| 30 | Re-measured 2026-10-16 on master `8e4727129` (slam split merged) by `<scratchpad>/measure_width.py`, which mirrors `TestSlamSplitLinesFitEightyColumns` (`internal/items/slam_split_copy_test.go:138`): every role line, names at 20 characters, every other token at its widest value from `slamSplitWidestTokens` (`:32-44`), tags stripped, no token left unfilled. `unarmedname` read from species YAML: bear 3 "claws and teeth" (`3-bear.yaml:12`), feline 11 "claws and fangs" (`11-feline.yaml:13`). `maul.yaml`: 149 of 345 role lines over 80 (widest 124; prepare 23, wait 18, miss 10, weak 14, normal 20, heavy 24, critical 30, fumble 10). `pounce.yaml`: 121 of 345 (widest 116; prepare 21, wait 15, miss 5, weak 12, normal 14, heavy 25, critical 22, fumble 7). For comparison, by the same rule with each file's longest current `unarmedname`: `claws.yaml` 128 of 354 ("smoke-wrought claws"), `bite.yaml` 135 of 345 ("tendrils and maw"); those two are out of this slice's scope. The guard is generic over its `slamSplitFiles` list (`:21`) and reads the longest `unarmedname` of the species using each file (`:93-120`), failing when none does | script, file | 4, D7 |
| 31 | Engage-line guards: `messaging_surface_guard_test.go:1343` registers `usercommands/attack.go\|You prepare to enter into mortal combat with %s.`; `TestNarrationSitesMatchViewpointAudit` (`:1475`) fails a stale entry. `attack_sight_names_test.go` asserts the dark and blinded fallbacks; `attack_announce_test.go` counts `prepares to fight` per recipient. Command tests do not seed the attack store (`seedAllRegistries`, `mobcommands_test.go:73`), so an empty authored line must fall back to today's text | file | 3 |
| 32 | The only smoke elemental mob is 322 Elemental Prince (`mobs/instance_planar_oasis/322-elemental_prince.yaml`): hostile, no `player_attack_immune`, no `aiprofile` (so `default`), `loot_pool` 10030, 20074, activitylevel 30; default `SpecialMoveChance` 30 (`internal/mobs/mobs.go:709-710`) | file | Appendix A |
| 33 | Negative searches: `grep -rn smother --include=*.go .` returns 0 lines while `grep -rln throttle --include=*.go .` returns 59 files; `grep -rn "beast_moves\|BeastMoves" --include=*.go --include=*.yaml .` returns only `ai_test.go:950-958` (a test name), while `beastMoves` (lowercase) hits `behaviortree/actions_mob.go:237`. No smother code and no move-list field exist | grep | added |
| 34 | Test helpers this plan uses, all present: `species.SeedSpeciesForTest` (returns a cleanup func), `items.SeedAttackMessagesForTest` (`internal/items/test_helpers_combat.go:18`), `slamSplitLoadFile`, `slamSplitTag`, `slamSplitTokenRe` (`internal/combat/slam_split_render_test.go`), `repoRootForTest` (`coup_de_grace_test.go`), `defenceFixture` (`defense_affordability_test.go:21`), `dogmudSpecies` (`internal/species/species_night_vision_test.go:20`), `fundSpecialMove`, `newStubActor`, `newTestRoom`, `setCombatPositionParallel` (internal/actions tests), `events.DrainQueuedMessagesForTest`, `getTestUserAndRoom`, `getTestMobAndRoom`, `captureAnnounces`, `countPerRecipient` | grep | added |

Spec facts 14 and 15 (the PR #208 species corrections) belong to slice 1, which shipped in #568, and are not re-verified here.

### Re-verified 2026-10-19 on master `d3b7f9774` (rows above kept as written on 10-16)

All 34 rows were re-checked against the slice 2 worktree. 20 hold, 11 drifted in line numbers only, 3 changed in substance. Read current line numbers from source when executing a step; the deltas that matter:

- **Row 22 CHANGED (Task 11 code):** `combat_throttle.go:216-219` consumes the round with `char.SpendRoundOnAction()` (16b8362db), not `SetRoundsWaiting(1)`. The round hook consumes such a round with no wait line (`NewRound_DoCombat_resolution.go:87,97-99`), and `TestSpecialMoveAdmissionOrdering` counts only that call (`command_readiness_drift_test.go:297-305`). Task 11's smother code is updated to match. `ClaimSpecialMove` is now `special_move_cooldown.go:45`; throttle knobs `config.yaml:843-845`.
- **Row 4 CHANGED (Task 14 expectation):** a round right after a special move prints NO wait line (16b8362db). The playtest must not expect a wait line on the round after maul, pounce or smother. `GetWaitMessages` keeps its signature and lines; its one caller is now `NewRound_DoCombat_resolution.go:102`.
- **Row 20 CHANGED (Task 2 list):** add `shipped_narration_data_guard_test.go:327` (`"todefenderroom": "remote_observer"`) to Task 2's list of `separate` users; check whether movenarration still needs the alias before deleting it. The swing separate render is now `combat_helpers.go:1959-1998`. The new feint path (`sendFeintLines`, `:1625-1627`) sends a cross-room room line but does not read the `Separate` pool, so Task 2's shape holds.
- **Task 1 is not partly done:** e88c7f7af changed only which SKILL sets a swing's message tier and feint (`GetCombatSkillLevelFor(ws.weapon)`, `combat_helpers.go:1897,1926`); `buildWeaponSetup`'s subtype logic (`:430-462`) is unchanged. The entangled-defence reader is now `:691-694`.
- **Drift only:** fumble and coup de grace callers `combat_helpers.go:1863,1873`; `meleeDisplaySubtype :1710` (caller `:1856`, comment list `:1707-1709`); sharp band `attackresult.go:267-268`; charge `divergences.go:187`; drain EXEMPT note `combat/context.md:888-893`; predicate docs `actions/context.md:224-225`, `combat/context.md:844-848`; drift-test ranges `TestSpecialMoveAdmissionOrdering` 149-317, `TestSpecialMoveWrapperAdmission` 596-705 (mob list 598), `runExecuteAndReadExecuted` 728-760, `runExecuteAndReadFlag` 1456; banner guard empty-file check `:159-161`. Condition id 144 is still free (`id_inventory --alloc conditions 1`).

## Owner decisions (decided 2026-10-16; Task 0 Step 1 records them)

Every task below is written to these rulings. Each entry gives the question, the ruling and the reason.

- **D1. Smother's actor lines. DECIDED (a): `smother.yaml` ships without `actor` roles,** like `charge.yaml` and `hamstring.yaml`. Reason: smother is mob-only and a mob has no client, so actor lines have no reader (`internal/mobcommands/throttle.go` sends the actor `NoLine`; Fact 25); shipping them would recreate the unreachable-copy defect #541 removes. The `cast_interrupt` event keeps its `actee` and `observer` lines. If players ever gain smother, actor lines come back with that feature. Applied in Task 10.
- **D2. Engage lines in the dark. DECIDED (a): a participant at full sight reads the prepare engage line; anyone else reads today's plain line with names hidden,** as the wait round does. Reason: a prepare line names the weapon ("their Rusted Cleaver"), which a reader who cannot see must not learn. No new copy. Applied in Task 4.
- **D3. The `pounce.yaml` normal/master record (lines 240-242). APPROVED as written:** actor `You press <actee> with masterful control, <item> flashing!`, actee `<actor> presses you hard, <item> flashing!`, observer `<actor> presses <actee>, <item> flashing!` (exact YAML in Task 8). Reason: today's record makes the weapon the subject of "flash" (the #455 rule) and fails the natural-weapon guard once cats use pounce. D7 rewrites the rest of `pounce.yaml`; this record is part of that rewrite and is already approved, so the Task 8 review table shows it as approved, not for review.
- **D4. Cats' throttle weight. DECIDED: do NOT change the `ambush_predator` AI profile in this slice.** Filed as #574, a sub-issue of the behaviour arc parent #374. Reason: tuning which move a mob picks belongs to the behaviour arc, not to a data-reachability slice. Consequence: cats list throttle in `beast_moves` and the gate allows it, but with no profile weight the AI will rarely pick it (Fact 28). The tests check the gate (`CanUseThrottle` passes for a feline and fails for a rodent, Task 7); the playtest checks that a rat never throttles and does not require a cat to throttle (Task 14).
- **D5. Smother's defence label. APPROVED: `rolling smoke`** (renders "You dodge the Elemental Prince's rolling smoke"). Reason: it reuses the approved miss lines' own words. Applied in Task 11.
- **D6. The attack-family predicates. DECIDED (b): DELETE `SpeciesIsFanged`, `SpeciesIsClawed`, `SpeciesIsHorned` and `SpeciesHasLifeDrain`** (and `SpeciesIsQuadrupedPredator`, deleted under either option) once the gates read `beast_moves`. Reason: no production caller remains (Fact 18), and #563 (anatomy-aware combat text) will want a proper anatomy model rather than attack-family predicates. The same reason covers `SpeciesNaturalAttack`, whose only production callers are those predicates (`ai.go:255,258,261`), so it goes with them. **This supersedes spec section 4's bullet that the predicates "describe the body".** Applied in Task 7 Step 6 and Task 9.
- **D7. maul and pounce width. AMENDED 2026-10-19: NO width rewrite; `maul.yaml` and `pounce.yaml` ship as written.** The owner reviewed both files and ruled them good enough. Reason: every combat hit category is wrapped by the message pipeline at the player's line width (`shouldWrap`, `internal/messaging/pipeline.go:134-138`), so a line over 80 columns wraps onto a second line rather than breaking. (Superseded ruling, 2026-10-16: (b), rewrite every line over 80 through an owner review table; `maul.yaml` 149 of 345 role lines over 80, `pounce.yaml` 121 of 345, Fact 30.) Consequence: Task 8 shrinks to D3's one record plus the natural-weapon guard check, and Task 9 does not add maul and pounce to the slam split's width guard.
- **D8. Patch note copy (Task 12 Step 5). APPROVED,** with the throat bullet naming wolves only (owner confirmed 2026-10-16), since under D4 cats rarely throttle.

---

## Task 0: Owner gate and land the plan

**Files:** `docs/superpowers/plans/2026-10-16-combat-reachability-slice2.md` (this plan), `docs/README.md`.

- [x] **Step 1:** D1 to D8 were put to the owner and answered on 2026-10-16. Each ruling is recorded above under "Owner decisions", with its reason, and every task is written to it. The D8 wording forced by D4 was confirmed the same day (Task 12 Step 5). The plan was approved on 2026-10-16, including `SpeciesNaturalAttack` joining the D6 deletion.
- [ ] **Step 2:** On a branch off master after #571 merges (`git switch -c feature/combat-reachability-slice2 origin/master`), copy this plan to `docs/superpowers/plans/2026-10-16-combat-reachability-slice2.md` and add its row to `docs/README.md` under the 2026-10-16 combat rows (after the slam split rows at lines 113-114):

```markdown
| [`superpowers/plans/2026-10-16-combat-reachability-slice2.md`](superpowers/plans/2026-10-16-combat-reachability-slice2.md) | Plan for slice 2 of the reachability spec over a 34-row facts table: one weapon-type rule (`combat.AttackSubtype`), engage lines from `prepare`, the `separate` split and `shooting.yaml`'s swing lines retired, the default-attack `throttle.yaml` deleted, `beast_moves` species data read by every beast-move gate (the attack-family predicates deleted), `maul.yaml` and `pounce.yaml` rewritten to 80 columns through an owner review, bears on `maul` and cats on `pounce`, and the smoke elemental's mob-only `smother`; owner rulings D1 to D8; guards, docs, gate, playtest |
```

- [ ] **Step 3:** Commit.

```bash
git add docs/superpowers/plans/2026-10-16-combat-reachability-slice2.md docs/README.md
git commit -F - <<'EOF'
docs(combat): plan for reachability slice 2 (#541)

Owner decisions D1 to D8 recorded in the plan.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

## Task 1: One weapon-type rule (spec section 1)

**Files:** `internal/combat/combat_helpers.go`, `internal/combat/combat.go`, new `internal/combat/attack_subtype_test.go`.

- [ ] **Step 1: Failing tests.** Create `internal/combat/attack_subtype_test.go`:

```go
package combat

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// Spec 2026-10-16 #541 section 1: one weapon-type rule. The main-hand
// weapon's subtype, else the species' natural attack, else bare hands.
func TestAttackSubtype_OneRule(t *testing.T) {
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9101: {SpeciesId: 9101, Name: "testwolf", UnarmedName: "jaws", NaturalAttack: items.Bite},
		9102: {SpeciesId: 9102, Name: "testhuman", UnarmedName: "fists"},
	}))
	sword := items.Item{ItemId: 10, Spec: &items.ItemSpec{Type: items.Weapon, Subtype: items.Slashing}}

	human := &characters.Character{SpeciesId: 9102}
	wolf := &characters.Character{SpeciesId: 9101}
	swordsman := &characters.Character{SpeciesId: 9102}
	swordsman.Equipment.Weapon = sword
	armedWolf := &characters.Character{SpeciesId: 9101}
	armedWolf.Equipment.Weapon = sword
	unknown := &characters.Character{SpeciesId: 424242}

	cases := []struct {
		name string
		char *characters.Character
		want items.ItemSubType
	}{
		{"weapon user", swordsman, items.Slashing},
		{"natural attacker", wolf, items.Bite},
		{"bare-handed humanoid", human, items.Unarmed},
		{"a weapon beats the species attack", armedWolf, items.Slashing},
		{"an unknown species is bare-handed", unknown, items.Unarmed},
	}
	for _, c := range cases {
		if got := AttackSubtype(c.char); got != c.want {
			t.Errorf("%s: AttackSubtype = %q, want %q", c.name, got, c.want)
		}
	}

	// The swing path asks the same rule per slot: an armed wolf's empty
	// off-hand still bites.
	if got := buildWeaponSetup(armedWolf, armedWolf, items.Item{}, 1, 2).weaponSubType; got != items.Bite {
		t.Errorf("armed wolf's empty off-hand swings as %q, want bite", got)
	}
}

// preAttackMatchers turns every template of one role of one pre-attack
// intensity into a regexp, tokens as wildcards, tags stripped.
func preAttackMatchers(t *testing.T, g *items.WeaponAttackMessageGroup, intensity items.Intensity,
	role func(items.TogetherMessages) items.SkillTieredMessages) []*regexp.Regexp {
	t.Helper()
	opts, ok := g.Options[intensity]
	if !ok {
		t.Fatalf("%s has no %s pool", g.OptionId, intensity)
	}
	var out []*regexp.Regexp
	s := role(opts.Together)
	for _, tier := range []items.MessageOptions{s.Beginner, s.Expert, s.Master} {
		for _, m := range tier {
			plain := slamSplitTag.ReplaceAllString(string(m), ``)
			out = append(out, regexp.MustCompile(`^`+slamSplitTokenRe.ReplaceAllString(regexp.QuoteMeta(plain), `.*?`)+`$`))
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s %s pool is empty", g.OptionId, intensity)
	}
	return out
}

func matchesAnyRe(line string, res []*regexp.Regexp) bool {
	for _, re := range res {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// Ruling B: a creature waits with its own natural attack. A wolf's wait
// lines come from bite.yaml, in bite's hit band, not from generic.
func TestGetWaitMessages_WolfWaitsWithItsBite(t *testing.T) {
	bite := slamSplitLoadFile(t, items.Bite)
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Bite:    bite,
		items.Generic: slamSplitLoadFile(t, items.Generic),
	}))
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9103: {SpeciesId: 9103, Name: "testwolf", UnarmedName: "jaws", NaturalAttack: items.Bite},
	}))
	actee := preAttackMatchers(t, bite, items.Wait, func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToDefender })

	for range 20 {
		wolf := characters.New()
		wolf.SpeciesId = 9103
		wolf.Name = "Wolf"
		selka := characters.New()
		selka.Name = "Selka"
		res := GetWaitMessages(items.Wait, wolf, selka, Mob, User)
		if len(res.MessagesToTarget) == 0 {
			t.Fatal("the wolf sent its target no wait line")
		}
		msg := res.MessagesToTarget[0]
		if msg.Category != messaging.CategoryHitNaturalSharp {
			t.Errorf("wait line category = %v, want bite's CategoryHitNaturalSharp", msg.Category)
		}
		line := strings.TrimSpace(slamSplitTag.ReplaceAllString(msg.Text, ``))
		if !matchesAnyRe(line, actee) {
			t.Fatalf("wolf wait line is not from bite.yaml: %q", line)
		}
	}
}
```

Run: `go test ./internal/combat/ -run 'TestAttackSubtype_OneRule|TestGetWaitMessages_WolfWaitsWithItsBite' -count=1`. Expected: FAIL, `undefined: AttackSubtype`.

- [ ] **Step 2: The rule.** Add to `internal/combat/combat_helpers.go`, directly above `buildWeaponSetup`:

```go
// AttackSubtype is the one weapon-type rule (spec 2026-10-16 #541 section 1):
// the main-hand weapon's subtype, else the species' natural attack, else
// items.Unarmed. The swing path, the wait and engage lines, and the
// fumble and coup de grace lookups (through buildWeaponSetup) all read it, so
// a creature squares up, waits and swings with the same thing.
func AttackSubtype(char *characters.Character) items.ItemSubType {
	return attackSubtypeWith(char, char.Equipment.Weapon)
}

// attackSubtypeWith is the rule for one weapon slot. buildWeaponSetup asks it
// once per slot, so an off-hand dagger swings as stabbing beside a main-hand
// sword, and an empty slot (ItemId 0) falls to the species attack.
func attackSubtypeWith(char *characters.Character, weapon items.Item) items.ItemSubType {
	if weapon.ItemId > 0 {
		return weapon.GetSpec().Subtype
	}
	if sp := species.GetSpecies(char.SpeciesId); sp != nil && sp.NaturalAttack != "" {
		return sp.NaturalAttack
	}
	return items.Unarmed
}
```

In `buildWeaponSetup` set `weaponSubType: attackSubtypeWith(sourceChar, weapon),` in the `weaponSetup{...}` literal (was `items.Unarmed`), delete the `raceInfo.NaturalAttack` block and its four-line comment (`:451-457`), and delete `ws.weaponSubType = itemSpec.Subtype` inside `if weapon.ItemId > 0` (`:462`).

- [ ] **Step 3: The other readers.** In `GetWaitMessages` (`combat.go`), replace `:218` with:

```go
	subtype := AttackSubtype(sourceChar)
	msgs := items.GetPreAttackMessage(subtype, stepType)
```

and replace the `waitCat` block (`:303-308`) with:

```go
	// Wait-round lines ride the attack's own hit band, the swing path's rule.
	waitCat := CategoryForWeaponSubtype(subtype)
```

In `combat_helpers.go:685-688` (the "too entangled to defend" lines) replace the `hitCat` block with `hitCat := CategoryForWeaponSubtype(AttackSubtype(sourceChar))`.

- [ ] **Step 4: Run** `go test ./internal/combat/ ./internal/hooks/ -count=1`. Expected: PASS. Then the full `go test . ./internal/... ./modules/...`; expected PASS (golden unchanged: the store's text is untouched).
- [ ] **Step 5: Commit.**

```bash
git add internal/combat/combat_helpers.go internal/combat/combat.go internal/combat/attack_subtype_test.go
git commit -F - <<'EOF'
refactor(combat): one weapon-type rule, combat.AttackSubtype (#541)

The swing path, the wait round and the entangled-defence lines now read
one rule: weapon, else species attack, else bare hands. A creature waits
with its own natural attack (ruling B); a bare-handed humanoid waits
from unarmed.yaml, the file its swings already use.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

## Task 2: Retire `separate` (spec section 3, first bullet)

**Files:** `internal/items/attack_messages.go`, `internal/items/attack_records.go`, `internal/items/test_helpers_combat.go`, `internal/combat/combat.go`, `internal/combat/combat_helpers.go`, `_datafiles/world/dogmud/combat-messages/*.yaml` (all 23: separate blocks in generic and shooting, exit-token header lines in all), tests listed in Fact 20, new root `combat_message_reachability_guard_test.go`, `internal/narration/testdata/stores/combat_messages.golden`.

- [ ] **Step 1: Failing guard.** Create `combat_message_reachability_guard_test.go` at the repo root:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Owner ruling C (spec 2026-10-16 #541 section 3): round combat releases a
// target in another room before the wait round or the swing
// (hooks.resolveCombatTarget), so a separate block can never render. The
// decoder rejects one at load; this guard names the file and line. The exit
// tokens only ever filled separate lines, so a line carrying one would read
// "unknown".
func TestCombatMessagesHaveNoSeparateBlock(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "combat-messages", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "no combat-message files: the glob is wrong, not the data")
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		if hit := unreachableCombatLine(string(raw)); hit != "" {
			t.Errorf("%s: %s", f, hit)
		}
	}
}

// unreachableCombatLine returns the first line that can never render, or "".
func unreachableCombatLine(src string) string {
	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "separate:") || strings.Contains(trimmed, "remote_observer:") {
			return fmt.Sprintf("line %d: a separate block cannot render: %s", i+1, trimmed)
		}
		if strings.Contains(line, "{exitname}") || strings.Contains(line, "{entrancename}") {
			return fmt.Sprintf("line %d: an exit token only ever filled a separate line: %s", i+1, trimmed)
		}
	}
	return ""
}

// The guard must be able to fail.
func TestUnreachableCombatLine_CanFail(t *testing.T) {
	for _, src := range []string{
		"options:\n  wait:\n    separate:\n      beginner: []\n",
		"options:\n  wait:\n    together:\n      beginner:\n      - actor: 'You aim through the {exitname}.'\n",
	} {
		if unreachableCombatLine(src) == "" {
			t.Errorf("missed an unreachable line in %q", src)
		}
	}
	if hit := unreachableCombatLine("# {exitname} - a header comment\noptions:\n"); hit != "" {
		t.Errorf("flagged a comment: %s", hit)
	}
}
```

Run: `go test . -run 'TestCombatMessagesHaveNoSeparateBlock|TestUnreachableCombatLine_CanFail' -count=1`. Expected: the probe test PASSES, the store test FAILS on `generic.yaml` and `shooting.yaml` (and on header lines of every file).

- [ ] **Step 2: Strip the data.** Write `<scratchpad>/strip_separate.py` with the Write tool:

```python
import re, sys
# Drops every `    separate:` block (with the 4-space comment line just above
# it) and the `# {exitname}` / `# {entrancename}` header lines. A block ends
# at the first non-blank line indented 4 spaces or less.
header = re.compile(r'^# \{(exitname|entrancename)\}')
src, dst = sys.argv[1], sys.argv[2]
lines = open(src, encoding='utf-8').read().split('\n')
out, skipping = [], False
for line in lines:
    if line.rstrip() == '    separate:':
        if out and out[-1].startswith('    #') and not out[-1].startswith('     '):
            out.pop()
        skipping = True
        continue
    if skipping:
        indent = len(line) - len(line.lstrip(' '))
        if line.strip() == '' or indent > 4:
            continue
        skipping = False
    if header.match(line):
        continue
    out.append(line)
open(dst, 'w', encoding='utf-8', newline='\n').write('\n'.join(out))
```

Then, per file (bash, `S` = the scratchpad full path):

```bash
for f in _datafiles/world/dogmud/combat-messages/*.yaml; do
  python -I "$S/strip_separate.py" "$f" "$S/stripped-$(basename "$f")" && cp "$S/stripped-$(basename "$f")" "$f"
done
grep -c "separate:" _datafiles/world/dogmud/combat-messages/*.yaml | grep -v ":0"   # expect no output (run alone: grep -c exits 1 on zero)
git diff --stat _datafiles/world/dogmud/combat-messages/
```

Expected: only generic.yaml and shooting.yaml lose blocks; every file loses exactly two header lines.

- [ ] **Step 3: The decoder rejects `separate`.** In `internal/items/attack_records.go` delete `separateRoles` (`:23`) and `(*SeparateMessages).UnmarshalYAML` (`:132-141`), and add:

```go
// UnmarshalYAML accepts only `together`. A `separate` block (a blow whose
// participants stand in different rooms) can never render: round combat
// releases a target in another room before the wait round or the swing
// (hooks.resolveCombatTarget), so it is rejected at load rather than shipped
// as dead copy (owner ruling C, spec 2026-10-16 #541 section 3).
func (a *AttackOptions) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var probe map[string]interface{}
	if err := unmarshal(&probe); err != nil {
		return err
	}
	keys := make([]string, 0, len(probe))
	for k := range probe {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k != "together" {
			return fmt.Errorf("unknown key %q (only together; separate is retired)", k)
		}
	}
	var raw struct {
		Together TogetherMessages `yaml:"together"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	a.Together = raw.Together
	return nil
}
```

In `internal/items/attack_messages.go`: delete the `Separate` field (`:33`), the `SeparateMessages` type and comment (`:48-61`), `(SeparateMessages).Render` and comment (`:159-190`), the separate branch of `Validate` (`:260-272`), `anyPool`'s separate wording (keep `anyPool` for the together branch, comment "so an intensity with no together block is skipped"), and the `exit` import. In `internal/items/test_helpers_combat.go` delete the `Separate:` literal (`:102-107`).

- [ ] **Step 4: The render paths.** In `GetWaitMessages` (`combat.go`) delete the `together` variable and the two exit loops (`:247-273`), replace the render block (`:285-296`) with `roles := msgs.Together.Render(skillLevel, tokenReplacements, nil)`, drop `toDefenderRoomMsg`, and in the `SendTrio` call drop `RemoteObserver` and `RemoteRoom` with the `remoteRoom` lookup (`:338-346`); rewrite the comment above `SendTrio` to say the trio carries the one room's observer line. In `buildAttackMessages` (`combat_helpers.go`) delete `together`, the exit loops (`:1829-1850`), the `Separate` branch (keep `roles := msgs.Together.Render(skillLevel, tokenReplacements, nil)`), the variable `toDefenderRoomMsg` and every branch that carries or sends it (`:1867`, `:1887-1889`, `:1908-1910`, `:1917-1919`, `:1926-1928`, `:1986-1988`). Run `go build ./...`; the compiler lists anything missed (dogmud-refactoring: delete the field and let the compiler enumerate).

- [ ] **Step 5: Tests that described the retired path.** Delete `internal/items/attack_messages_vertical_test.go` and `internal/combat/wait_remote_room_hiding_test.go` (both test only the separate render). In `internal/items/attack_messages_test.go` delete `TestSeparateRenderMapsFourRoles`, `TestValidateAllowsAnAbsentSeparateBlock`, `TestValidateRejectsAPartialSeparateBlock` and the separate half of `TestRenderTakesExactlyOneDrawPerMessage` (`:136-148`). In `internal/items/attack_records_test.go` replace the separate fixtures (`:31`, `:48`, `:67`) with together-only ones and replace the `"separate needs remote_observer"` case with:

```go
		"separate is retired": {`
optionid: generic
options:
  weak:
    separate:
      beginner:
      - {actor: 'A', actee: 'D', observer: 'R', remote_observer: 'X'}
`, "option[`weak`]: unknown key \"separate\""},
```

In `internal/combat/coup_de_grace_test.go` delete the two `separate.` rows (`:104-105`). In `combat_banner_agreement_guard_test.go` delete the separate `slotFailures` call (`:134-139`), `opts.Separate.ToAttacker` from `bannerRecordCount` (`:170`), `separateMayOmitBanner` and `TestBannerGuardLetsTheAttackerRoomOmitInSeparate`. In `internal/narration/snapshot_test.go` delete `separateRoles`, `hasSeparate` and the `if hasSeparate[subtype]` block, and change the header line at `:339` to `# separate sections retired (spec 2026-10-16 #541 section 3): round combat never crosses rooms`. In `messaging_surface_guard_test.go:158` rewrite the `remote_observer` reason to name only the movenarration store (`internal/movenarration/store.go` `RemoteObserver`, shoot's arrival events).

- [ ] **Step 6: Goldens.** `go test ./internal/narration/... -run TestSnapshotStores -update -count=1`, then `git diff --stat internal/narration/testdata/stores/`. Expected: only `combat_messages.golden` changes, losing the 136 `separate` rows (`grep -c separate` on the old golden is 136) plus the header line. Read the diff; no together row may change.
- [ ] **Step 7: Run** `gofmt -l internal/ modules/` (expect nothing), then `go test . ./internal/... ./modules/... -count=1`. Expected: PASS, including `TestCombatMessagesHaveNoSeparateBlock`.
- [ ] **Step 8: Commit.**

```bash
git add combat_message_reachability_guard_test.go combat_banner_agreement_guard_test.go messaging_surface_guard_test.go \
  internal/items/attack_messages.go internal/items/attack_records.go internal/items/test_helpers_combat.go \
  internal/items/attack_messages_test.go internal/items/attack_records_test.go internal/items/attack_messages_vertical_test.go \
  internal/combat/combat.go internal/combat/combat_helpers.go internal/combat/coup_de_grace_test.go \
  internal/combat/wait_remote_room_hiding_test.go internal/narration/snapshot_test.go \
  internal/narration/testdata/stores/combat_messages.golden _datafiles/world/dogmud/combat-messages
git commit -F - <<'EOF'
refactor(combat): retire the separate split that never rendered (#541)

Round combat releases a target in another room before the wait round
or the swing, so the 17 separate blocks (generic 9, shooting 8), the
SeparateMessages type, its render and both render branches could never
reach a player. The decoder now rejects a separate block at load, and a
root guard names any that comes back. Golden: 136 separate rows out.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

## Task 3: `shooting.yaml` keeps only `prepare` and `wait` (spec section 3, second bullet)

**Files:** `internal/items/attack_messages.go`, `internal/combat/combat_helpers.go`, `_datafiles/world/dogmud/combat-messages/shooting.yaml`, `combat_banner_agreement_guard_test.go`, `internal/narration/snapshot_test.go`, `internal/items/attack_messages_test.go`, `combat_message_reachability_guard_test.go`, golden.

- [ ] **Step 1: Failing tests.** Append to `internal/items/attack_messages_test.go`:

```go
// Ruling C: a swing with a re-narrated subtype borrows another file's lines,
// so its own file needs only prepare and wait. The validator reads the same
// function the renderer does.
func TestValidateExemptsReNarratedSwingIntensities(t *testing.T) {
	if as, ok := MeleeSwingNarratedAs(Shooting); !ok || as != Bludgeoning {
		t.Fatalf("MeleeSwingNarratedAs(shooting) = %q, %v; want bludgeoning, true", as, ok)
	}
	if _, ok := MeleeSwingNarratedAs(Slashing); ok {
		t.Fatal("slashing swings with its own lines")
	}
	one := SkillTieredMessages{Beginner: MessageOptions{"x"}}
	opts := AttackOptions{Together: TogetherMessages{ToAttacker: one, ToDefender: one, ToRoom: one}}
	bow := &WeaponAttackMessageGroup{OptionId: Shooting, Options: AttackTypes{Prepare: opts, Wait: opts}}
	if err := bow.Validate(); err != nil {
		t.Errorf("shooting with prepare and wait only must validate: %v", err)
	}
	sword := &WeaponAttackMessageGroup{OptionId: Slashing, Options: AttackTypes{Prepare: opts, Wait: opts}}
	if err := sword.Validate(); err == nil {
		t.Error("slashing without its swing intensities must fail")
	}
}
```

Append to `combat_message_reachability_guard_test.go`:

```go
// shooting.yaml's swing intensities can never render: a bow swung in melee
// narrates as bludgeoning (items.MeleeSwingNarratedAs).
func TestShootingKeepsOnlyPrepareAndWait(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("_datafiles", "world", "dogmud", "combat-messages", "shooting.yaml"))
	require.NoError(t, err)
	var g struct {
		Options map[string]any `yaml:"options"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &g))
	require.Contains(t, g.Options, "prepare")
	require.Contains(t, g.Options, "wait")
	for k := range g.Options {
		require.Contains(t, []string{"prepare", "wait"}, k, "shooting.yaml authors %q, which never renders", k)
	}
}
```

(add `"gopkg.in/yaml.v3"` to that file's imports). Run: `go test ./internal/items/ -run TestValidateExempts -count=1` FAILS (`undefined: MeleeSwingNarratedAs`); `go test . -run TestShootingKeepsOnlyPrepareAndWait -count=1` FAILS on `miss`.

- [ ] **Step 2: The shared rule.** Add to `internal/items/attack_messages.go`:

```go
// MeleeSwingNarratedAs reports the subtype whose swing lines a melee swing
// with sub always borrows, and true; or "", false when sub swings with its
// own lines. A bow, crossbow, sling or gun swung in melee is a club, so
// combat.meleeDisplaySubtype narrates it as bludgeoning and shooting.yaml
// keeps only prepare and wait (owner ruling C, spec 2026-10-16 #541
// section 3). Validate reads this same function, so the renderer and the
// validator cannot drift.
func MeleeSwingNarratedAs(sub ItemSubType) (ItemSubType, bool) {
	if sub == Shooting {
		return Bludgeoning, true
	}
	return "", false
}

// swingIntensities are the intensities a swing renders. A subtype that
// MeleeSwingNarratedAs re-narrates may omit them.
var swingIntensities = []Intensity{Miss, Weak, Normal, Heavy, Critical, Fumble}
```

In `Validate` replace `optionsToCheck` with:

```go
	optionsToCheck := []Intensity{Prepare, Wait}
	if _, reNarrated := MeleeSwingNarratedAs(w.OptionId); !reNarrated {
		optionsToCheck = append(optionsToCheck, swingIntensities...)
	}
```

In `meleeDisplaySubtype` (`combat_helpers.go:1585-1587`) replace the Shooting check with:

```go
	if as, ok := items.MeleeSwingNarratedAs(weaponSubType); ok {
		return as
	}
```

and point rule 1 of its doc comment at `items.MeleeSwingNarratedAs`.

- [ ] **Step 3: The data.** Write `<scratchpad>/keep_pre_attack.py`:

```python
import sys
src, dst = sys.argv[1], sys.argv[2]
out = []
for line in open(src, encoding='utf-8').read().split('\n'):
    if line.rstrip() == '  miss:':
        break
    out.append(line)
open(dst, 'w', encoding='utf-8', newline='\n').write('\n'.join(out).rstrip('\n') + '\n')
```

```bash
f=_datafiles/world/dogmud/combat-messages/shooting.yaml
python -I "$S/keep_pre_attack.py" "$f" "$S/shooting.yaml" && cp "$S/shooting.yaml" "$f"
grep -nE "^  [a-z]+:" "$f"    # expect exactly: prepare, wait
```

Add one header comment under `optionid: shooting`: `# Only prepare and wait: a bow swung in melee narrates as bludgeoning (items.MeleeSwingNarratedAs), and the shoot command has its own lines (narration/special-moves/shoot.yaml).`

- [ ] **Step 4: Guards that assumed every file swings.** In `combat_banner_agreement_guard_test.go` make the empty-file check apply only to a file that swings:

```go
		if _, swings := g.Options[items.Critical]; swings && bannerRecordCount(&g) == 0 {
			t.Errorf("%s: no record carries a banner word, the guard checked nothing", name)
		}
```

In `internal/narration/snapshot_test.go`, first line inside `for _, intensity := range intensityList {`:

```go
			// A re-narrated subtype authors no swing lines; GetPreAttackMessage
			// would hand back generic's and record them twice.
			if _, reNarrated := items.MeleeSwingNarratedAs(items.ItemSubType(subtype)); reNarrated &&
				intensity != items.Prepare && intensity != items.Wait {
				continue
			}
```

- [ ] **Step 5: Goldens.** `go test ./internal/narration/... -run TestSnapshotStores -update -count=1`; `git diff --stat internal/narration/testdata/stores/`. Expected: only `combat_messages.golden`, losing the shooting swing rows (miss to fumble) and nothing else.
- [ ] **Step 6: Run** `go test . ./internal/... ./modules/... -count=1`. Expected PASS.
- [ ] **Step 7: Commit** `refactor(combat): shooting.yaml keeps only prepare and wait (#541)` with the paths above and a body naming `items.MeleeSwingNarratedAs` and the golden row count removed.

## Task 4: Engage lines from `prepare` (spec section 2, rulings A and D2)

**Files:** `internal/combat/combat.go`, new `internal/combat/engage_messages.go`, new `internal/combat/engage_messages_test.go`, `internal/usercommands/attack.go`, `internal/mobcommands/attack.go`, new tests in `internal/usercommands/attack_engage_test.go` and `internal/mobcommands/attack_engage_test.go`, `messaging_surface_guard_test.go` (if its entry goes stale).

- [ ] **Step 1: Failing tests.** Create `internal/combat/engage_messages_test.go`:

```go
package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// Ruling A: a wolf squares up from bite.yaml's prepare pool, one draw for
// all three readers, in bite's hit band.
func TestEngageMessages_WolfSquaresUpFromBite(t *testing.T) {
	bite := slamSplitLoadFile(t, items.Bite)
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Bite: bite, items.Generic: slamSplitLoadFile(t, items.Generic),
	}))
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9301: {SpeciesId: 9301, Name: "testwolf", UnarmedName: "jaws", NaturalAttack: items.Bite},
	}))
	actee := preAttackMatchers(t, bite, items.Prepare, func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToDefender })
	room := preAttackMatchers(t, bite, items.Prepare, func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToRoom })

	for range 20 {
		wolf := characters.New()
		wolf.SpeciesId = 9301
		wolf.Name = "Wolf"
		selka := characters.New()
		selka.Name = "Selka"
		lines := EngageMessages(wolf, selka, Mob, User)
		if lines.Category != messaging.CategoryHitNaturalSharp {
			t.Fatalf("engage category = %v, want CategoryHitNaturalSharp", lines.Category)
		}
		if l := strings.TrimSpace(slamSplitTag.ReplaceAllString(lines.Actee, ``)); !matchesAnyRe(l, actee) {
			t.Fatalf("defender's engage line is not from bite.yaml's prepare pool: %q", l)
		}
		if l := strings.TrimSpace(slamSplitTag.ReplaceAllString(lines.Observer, ``)); !matchesAnyRe(l, room) {
			t.Fatalf("room's engage line is not from bite.yaml's prepare pool: %q", l)
		}
	}
}

// The engage line is drawn by AttackSubtype directly, not
// meleeDisplaySubtype: a bow user squares up with shooting.yaml's own
// prepare lines, in the ranged band.
func TestEngageMessages_BowUserUsesShootingPrepare(t *testing.T) {
	shooting := slamSplitLoadFile(t, items.Shooting)
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Shooting: shooting, items.Generic: slamSplitLoadFile(t, items.Generic),
	}))
	actor := preAttackMatchers(t, shooting, items.Prepare, func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToAttacker })

	archer := characters.New()
	archer.Name = "Rurik"
	archer.Equipment.Weapon = items.Item{ItemId: 11, Spec: &items.ItemSpec{Type: items.Weapon, Subtype: items.Shooting, Name: "longbow"}}
	target := characters.New()
	target.Name = "Selka"
	lines := EngageMessages(archer, target, User, User)
	if lines.Category != messaging.CategoryHitRanged {
		t.Fatalf("bow engage category = %v, want CategoryHitRanged", lines.Category)
	}
	if l := strings.TrimSpace(slamSplitTag.ReplaceAllString(lines.Actor, ``)); !matchesAnyRe(l, actor) {
		t.Fatalf("bow user's engage line is not from shooting.yaml's prepare pool: %q", l)
	}
}

// D2 (a): a reader below full sight keeps the plain line with names hidden,
// because an authored line names the weapon.
func TestEngageLineFor_DarkReaderGetsThePlainLine(t *testing.T) {
	authored := "Wolf bares their jaws at you."
	plain := "Wolf prepares to fight you!"
	if got := EngageLineFor(messaging.SightFull, authored, plain, []string{"Wolf"}); got != authored {
		t.Errorf("full sight: got %q, want the authored line", got)
	}
	for _, d := range []messaging.SightDecision{messaging.SightShapes, messaging.SightNone} {
		got := EngageLineFor(d, authored, plain, []string{"Wolf"})
		if strings.Contains(got, "jaws") || strings.Contains(got, "Wolf") {
			t.Errorf("sight %v: %q names the weapon or the attacker", d, got)
		}
	}
	if got := EngageLineFor(messaging.SightFull, "", plain, []string{"Wolf"}); got != plain {
		t.Errorf("an unloaded store must fall back to the plain line, got %q", got)
	}
}
```

Run: `go test ./internal/combat/ -run 'TestEngage' -count=1`. Expected: FAIL, `undefined: EngageMessages`. (`items.ItemSpec` has a `Name` field, `internal/items/itemspec.go:325`.)

- [ ] **Step 2: Shared tokens.** In `combat.go` extract the token map of `GetWaitMessages` into:

```go
// preAttackTokens fills the tokens a prepare or wait line carries: the
// weapon or species unarmed name, both names (a mob's by GetMobName), and
// the stance, position and momentum words.
func preAttackTokens(source, target *characters.Character, sourceType, targetType SourceTarget) map[items.TokenName]string {
	// GetSpecies returns nil for an unknown SpeciesId; default before use.
	unarmedName := "fists"
	if raceInfo := species.GetSpecies(source.SpeciesId); raceInfo != nil {
		unarmedName = raceInfo.UnarmedName
	}
	tokens := map[items.TokenName]string{
		items.TokenItemName:     unarmedName,
		items.TokenActor:        source.Name,
		items.TokenActorType:    string(sourceType) + `name`,
		items.TokenActee:        target.Name,
		items.TokenActeeType:    string(targetType) + `name`,
		items.TokenUsesLeft:     `[Invalid]`,
		items.TokenDamage:       `[Invalid]`,
		items.TokenEntranceName: `unknown`,
		items.TokenExitName:     `unknown`,
		items.TokenStance:       source.CalculateStanceString(),
		items.TokenPosition:     source.CalculatePositionString(),
		items.TokenMomentum:     source.CalculateMomentumString(),
	}
	if source.Equipment.Weapon.ItemId > 0 {
		tokens[items.TokenItemName] = source.Equipment.Weapon.DisplayName()
	}
	if sourceType == Mob {
		tokens[items.TokenActor] = source.GetMobName(0).String()
	}
	if targetType == Mob {
		tokens[items.TokenActee] = target.GetMobName(0).String()
	}
	return tokens
}
```

`GetWaitMessages` keeps `sourceChar.IncrementAttackCount()` BEFORE it calls `preAttackTokens` (the stance word reads the count), then renders `msgs.Together.Render(sourceChar.GetCombatSkillLevel(), preAttackTokens(sourceChar, targetChar, sourceType, targetType), nil)`.

- [ ] **Step 3: The engage draw.** Create `internal/combat/engage_messages.go`:

```go
package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// EngageLines is one coordinated draw from the attacker's prepare pool: the
// attacker's, the defender's and the room's line for the same moment, and
// the hit band they ride.
type EngageLines struct {
	Category messaging.Category
	Actor    string
	Actee    string
	Observer string
}

// EngageMessages draws the engage moment from the attacker's prepare pool
// (owner ruling A, spec 2026-10-16 #541 section 2). The pool is chosen by
// AttackSubtype directly, never meleeDisplaySubtype, so a bow user squares
// up with shooting.yaml's own prepare lines, as its wait lines already do.
// It does not count as an attack for the stance word (only a wait round
// does). An unloaded store returns empty lines and the caller keeps its
// plain line.
func EngageMessages(attacker, defender *characters.Character, attackerType, defenderType SourceTarget) EngageLines {
	subtype := AttackSubtype(attacker)
	msgs := items.GetPreAttackMessage(subtype, items.Prepare)
	roles := msgs.Together.Render(attacker.GetCombatSkillLevel(),
		preAttackTokens(attacker, defender, attackerType, defenderType), nil)
	return EngageLines{
		Category: CategoryForWeaponSubtype(subtype),
		Actor:    roles.Actor,
		Actee:    roles.Actee,
		Observer: roles.Observer,
	}
}

// EngageLineFor picks what one participant reads: the authored engage line
// at full sight, otherwise the plain line with the other party's name hidden
// at that reader's sight (D2: an authored line names the weapon, which a
// reader who cannot see must not learn; the wait round makes the same call
// in hooks.handleCombatWaitRound). An empty authored line also falls back.
func EngageLineFor(sight messaging.SightDecision, authored, plain string, names []string) string {
	if sight == messaging.SightFull && authored != "" {
		return authored
	}
	return messaging.HideNames(plain, names, sight)
}
```

This is D2 (a) as decided: the authored line only at full sight, so a reader below full sight never learns the weapon.

- [ ] **Step 4: Player sites** (`internal/usercommands/attack.go`; add the `combat` import). The mob branch, replacing `:286-298`:

```go
			engage := combat.EngageMessages(user.Character, &m.Character, combat.User, combat.Mob)

			if engaged {
				user.SendText(engage.Category, combat.EngageLineFor(attackerSight, engage.Actor,
					fmt.Sprintf(`You prepare to enter into mortal combat with %s.`, mName),
					[]string{m.Character.Name}))
			}

			sendMeleeAmbushDenial(user, ambushDenied)

			if !isSneaking {
				roomLine := engage.Observer
				if roomLine == `` {
					roomLine = fmt.Sprintf(`<ansi fg="username">%s</ansi> prepares to fight %s.`, user.Character.Name, mName)
				}
				room.SendTextVisualHidingNames(engage.Category, roomLine,
					[]string{user.Character.Name, m.Character.Name}, user.UserId)
			}
```

The PvP branch, replacing `:377-396`:

```go
			engage := combat.EngageMessages(user.Character, p.Character, combat.User, combat.User)

			// #214: each side's own line hides the other's name at the
			// READER's sight, as target.go's shift-focus lines do.
			user.SendText(engage.Category, combat.EngageLineFor(messaging.ParticipantSight(user.Character, room), engage.Actor,
				fmt.Sprintf(`You prepare to enter into mortal combat with <ansi fg="username">%s</ansi>.`, p.Character.Name),
				[]string{p.Character.Name}))

			sendMeleeAmbushDenial(user, pvpAmbushDenied)

			if !isSneaking {
				p.SendText(engage.Category, combat.EngageLineFor(messaging.ParticipantSight(p.Character, room), engage.Actee,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> prepares to fight you!`, user.Character.Name),
					[]string{user.Character.Name}))

				roomLine := engage.Observer
				if roomLine == `` {
					roomLine = fmt.Sprintf(`<ansi fg="username">%s</ansi> prepares to fight <ansi fg="username">%s</ansi>.`, user.Character.Name, p.Character.Name)
				}
				room.SendTextVisualHidingNames(engage.Category, roomLine,
					[]string{user.Character.Name, p.Character.Name}, user.UserId, attackPlayerId)
			}
```

(The PvP fallback room line now tags the player target `username`, fixing the `mobname` tag of Fact 3.)

- [ ] **Step 5: Mob sites** (`internal/mobcommands/attack.go`; add the `combat` import). Replace `:91-104` (inside `if engaged && !isSneaking && !alreadyFighting`):

```go
				// The engage moment is one draw from the mob's prepare pool
				// (spec 2026-10-16 #541 section 2). attack.go makes no
				// SendTrio call, so each reader's sight is applied here.
				engage := combat.EngageMessages(&mob.Character, u.Character, combat.Mob, combat.User)
				sight := messaging.ParticipantSight(u.Character, room)
				u.SendText(engage.Category, combat.EngageLineFor(sight, engage.Actee,
					fmt.Sprintf(`<ansi fg="mobname">%s</ansi> prepares to fight you!`, mob.Character.Name),
					[]string{mob.Character.Name}))

				roomLine := engage.Observer
				if roomLine == `` {
					roomLine = fmt.Sprintf(`<ansi fg="mobname">%s</ansi> prepares to fight <ansi fg="username">%s</ansi>.`, mob.Character.Name, u.Character.Name)
				}
				room.SendTextVisualHidingNames(engage.Category, roomLine,
					[]string{mob.Character.Name, u.Character.Name}, u.UserId)
```

Replace `:151-154`:

```go
			if engaged && !isSneaking && !alreadyFighting {
				engage := combat.EngageMessages(&mob.Character, &m.Character, combat.Mob, combat.Mob)
				roomLine := engage.Observer
				if roomLine == `` {
					roomLine = fmt.Sprintf(`<ansi fg="mobname">%s</ansi> prepares to fight <ansi fg="mobname">%s</ansi>.`, mob.Character.Name, m.Character.Name)
				}
				room.SendTextVisualHidingNames(engage.Category, roomLine,
					[]string{mob.Character.Name, m.Character.Name})
			}
```

The `engaged`, `!isSneaking` and `alreadyFighting` gates (the double-announce guards, `mobcommands/attack.go:84,133-134`, `characters/character.go:285-301`) are untouched.

- [ ] **Step 6: Site tests.** Create `internal/usercommands/attack_engage_test.go`:

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// engageMarkerStore seeds a prepare pool whose lines are easy to find. Every
// subtype falls back to Generic, so the marker fires for any attacker.
func engageMarkerStore(t *testing.T) {
	t.Helper()
	one := func(s string) items.SkillTieredMessages {
		return items.SkillTieredMessages{Beginner: items.MessageOptions{items.ItemMessage(s)}}
	}
	marker := items.AttackOptions{Together: items.TogetherMessages{
		ToAttacker: one(`ENGAGE-ACTOR you square up to {actee}.`),
		ToDefender: one(`ENGAGE-ACTEE {actor} squares up to you.`),
		ToRoom:     one(`ENGAGE-ROOM {actor} squares up to {actee}.`),
	}}
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Generic: {OptionId: items.Generic, Options: items.AttackTypes{items.Prepare: marker}},
	}))
}

// Ruling A: in a lit room the attacker and the room read the prepare line,
// and the hand-written line is gone.
func TestAttack_Lit_EngageLineComesFromThePreparePool(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	engageMarkerStore(t)
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	t.Cleanup(func() { user.Character.EndAggro() })
	witness := users.GetByUserId(2)
	require.NotNil(t, witness)
	events.DrainQueuedMessagesForTest(user.UserId)
	events.DrainQueuedMessagesForTest(witness.UserId)

	handled, err := Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "ENGAGE-ACTOR", "the attacker reads the prepare line: %q", out)
	require.NotContains(t, out, "mortal combat", "the hand-written line is retired in the light: %q", out)

	seen := strings.Join(events.DrainQueuedMessagesForTest(witness.UserId), "\n")
	if witness.Character.RoomId == user.Character.RoomId {
		require.Contains(t, seen, "ENGAGE-ROOM", "the witness reads the prepare room line: %q", seen)
		require.NotContains(t, seen, "prepares to fight", "the hand-written room line is retired: %q", seen)
	}
}
```

Create `internal/mobcommands/attack_engage_test.go`:

```go
package mobcommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mob-versus-mob room line comes from the prepare pool, once per
// reader, and the double-announce guard still holds on a repeat command.
func TestAttackMob_EngageRoomLineComesFromThePreparePool(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	one := func(s string) items.SkillTieredMessages {
		return items.SkillTieredMessages{Beginner: items.MessageOptions{items.ItemMessage(s)}}
	}
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Generic: {OptionId: items.Generic, Options: items.AttackTypes{items.Prepare: {Together: items.TogetherMessages{
			ToAttacker: one(`ENGAGE-ACTOR`), ToDefender: one(`ENGAGE-ACTEE`), ToRoom: one(`ENGAGE-ROOM {actor} faces {actee}.`),
		}}}},
	}))

	attacker, room := getTestMobAndRoom(t)
	var targetInstanceId int
	for _, id := range room.GetMobs() {
		if id != attacker.InstanceId {
			targetInstanceId = id
			break
		}
	}
	require.NotZero(t, targetInstanceId, "need a second mob in the room to attack")
	captured, mu, done := captureAnnounces(t)
	defer done()
	target := fmt.Sprintf("#%d", targetInstanceId)

	_, err := Attack(target, attacker, room)
	require.NoError(t, err)
	require.Equal(t, 1, countPerRecipient(t, captured, mu, "ENGAGE-ROOM"))
	assert.Zero(t, countPerRecipient(t, captured, mu, "prepares to fight"))

	_, err = Attack(target, attacker, room)
	require.NoError(t, err)
	assert.Equal(t, 1, countPerRecipient(t, captured, mu, "ENGAGE-ROOM"),
		"re-attacking a mob already being fought must NOT announce a second time")
}
```

- [ ] **Step 7: Run** `go test ./internal/combat/ ./internal/usercommands/ ./internal/mobcommands/ -count=1`, then `go test . -run 'TestNarrationSitesMatchViewpointAudit|TestEveryTextSurfaceIsRegistered|TestEveryTrioLiteralNamesAllThreeRoles' -count=1`. If the viewpoint audit reports `usercommands/attack.go|You prepare to enter into mortal combat with %s.` stale, remove that entry (`messaging_surface_guard_test.go:1343`) and leave the guard's own convention comment in its place: `// usercommands/attack.go's engage line moved to combat.EngageMessages (#541 slice 2): the prepare pool's coordinated draw, with this literal kept only as the dark fallback. Removed rather than re-keyed: no gap was fixed.` Then the full suite; expected PASS.
- [ ] **Step 8: Commit** `feat(combat): engage lines come from the prepare pool (#541)`, paths named, body: ruling A, D2 (a) (full sight reads the prepare line, anyone else the plain line with names hidden), double-announce guards untouched, fallback keeps tests that do not load the store on today's text.

## Task 5: Delete the default-attack `throttle.yaml` (spec section 3, third bullet; ruling D)

**Files:** `_datafiles/world/dogmud/combat-messages/throttle.yaml` (deleted), `combat_banner_agreement_guard_test.go`, `internal/narration/snapshot_test.go`, `internal/items/natural_weapon_agreement_test.go`, `shipped_narration_data_guard_test.go`, `combat_message_reachability_guard_test.go`, golden.

- [ ] **Step 1: Failing test.** Append to `combat_message_reachability_guard_test.go`:

```go
// Ruling D: throttle is a special move only. No species attacks with it, so
// a default-attack throttle.yaml could never render; its special-move
// narration stays.
func TestThrottleIsASpecialMoveOnly(t *testing.T) {
	_, err := os.Stat(filepath.Join("_datafiles", "world", "dogmud", "combat-messages", "throttle.yaml"))
	require.True(t, os.IsNotExist(err), "combat-messages/throttle.yaml must not exist")
	_, err = os.Stat(filepath.Join("_datafiles", "world", "dogmud", "narration", "special-moves", "throttle.yaml"))
	require.NoError(t, err, "the throttle special move keeps its narration")
}
```

Run `go test . -run TestThrottleIsASpecialMoveOnly -count=1`: FAIL.

- [ ] **Step 2: Delete and re-count.** `git rm _datafiles/world/dogmud/combat-messages/throttle.yaml`. Set `bannerGuardFileCount = 22` and its comment to `22 since the slam split (#541) added trample, lash and blast and slice 2 retired the default-attack throttle.yaml`. In `snapshot_test.go:12-13` write `22 files (weapon subtypes; the slam split, #541, added trample, lash and blast; slice 2 removed throttle on <commit date>)` with the real date. In `natural_weapon_agreement_test.go:20` set `naturalWeaponSpecialPools = []string{"maul"}` (Task 9 removes the list). In `shipped_narration_data_guard_test.go:911` write `gore.yaml, maul.yaml and pounce.yaml (throttle.yaml left combat-messages in #541 slice 2; its drain.yaml became siphon.yaml, ...)`, keeping the rest of the sentence.
- [ ] **Step 3: Goldens** (`-run TestSnapshotStores -update`): only `combat_messages.golden` changes (header 22, `throttle` gone from the subtype list, its rows gone). Quote the removed row count in the commit.
- [ ] **Step 4: Run** the full suite; PASS. Commit `chore(combat): retire the default-attack throttle.yaml (#539, #541)`.

## Task 6: `beast_moves` species data and its validator (spec section 5, rulings E and F)

**Files:** `internal/species/species.go`, 15 species YAML files, new `internal/species/species_beast_moves_test.go`.

- [ ] **Step 1: Failing tests.** Create `internal/species/species_beast_moves_test.go`:

```go
package species

import (
	"reflect"
	"strings"
	"testing"
)

// Spec 2026-10-16 #541 section 5, the Proposed column as reviewed, with the
// slam split's smoke elemental row. A species missing here has no beast
// moves.
var proposedBeastMoves = map[int][]string{
	2:  {"maul", "throttle", "pounce", "hamstring"}, // canine: wolves do all four
	3:  {"rake"},                                    // bear: a swipe; charge stays scripted
	6:  {"gore"},                                    // boar
	7:  {"gore"},                                    // deer
	9:  {"rake", "pounce"},                          // raptor: a stooping strike
	10: {"hamstring"},                               // rodent: ankle-biters
	11: {"rake", "pounce", "hamstring", "throttle"}, // feline: the throat crusher
	17: {"pounce"},                                  // arachnid: jumping spiders
	21: {"maul"},                                    // reptile: the crocodile's roll
	22: {"rake"},                                    // bat
	24: {"throttle", "pounce"},                      // mustelid: a neck bite
	32: {"drain"},                                   // wraith
	33: {"drain"},                                   // spectre
	34: {"drain"},                                   // vampire (hands allowed: drain is exempt)
	44: {"smother"},                                 // smoke elemental (Appendix A)
}

func TestBeastMovesMatchTheReviewedTable(t *testing.T) {
	all := dogmudSpecies(t)
	for id, sp := range all {
		want := proposedBeastMoves[id]
		got := sp.BeastMoves
		if len(want) == 0 && len(got) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("species %d (%s) beast_moves = %v, want %v", id, sp.Name, got, want)
		}
	}
	for id := range proposedBeastMoves {
		if all[id] == nil {
			t.Errorf("species %d in the table has no file", id)
		}
	}
}

// Every shipped list passes the load validator.
func TestShippedBeastMovesValidate(t *testing.T) {
	for id, sp := range dogmudSpecies(t) {
		if err := validateBeastMoves(sp); err != nil {
			t.Errorf("species %d: %v", id, err)
		}
	}
}

// The validator must be able to fail, once per rule.
func TestValidateBeastMoves_CanFail(t *testing.T) {
	cases := map[string]struct {
		sp   Species
		want string
	}{
		"unknown move":   {Species{SpeciesId: 1, Name: "x", BeastMoves: []string{"charge"}}, "unknown beast move"},
		"listed twice":   {Species{SpeciesId: 1, Name: "x", BodyParts: []string{"legs"}, BeastMoves: []string{"rake", "rake"}}, "twice"},
		"hands":          {Species{SpeciesId: 1, Name: "x", BodyParts: []string{"arms", "hands"}, BeastMoves: []string{"rake"}}, "without hands"},
		"pounce no legs": {Species{SpeciesId: 1, Name: "x", BodyParts: []string{"mouth"}, BeastMoves: []string{"pounce"}}, "legs"},
		"gore no horns":  {Species{SpeciesId: 1, Name: "x", BodyParts: []string{"legs"}, BeastMoves: []string{"gore"}}, "horns"},
		"drain no flag":  {Species{SpeciesId: 1, Name: "x", BeastMoves: []string{"drain"}}, "lifedrain"},
	}
	for name, c := range cases {
		err := validateBeastMoves(&c.sp)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one containing %q", name, err, c.want)
		}
	}
	vampire := Species{SpeciesId: 34, Name: "vampire", BodyParts: []string{"arms", "hands"}, LifeDrain: true, BeastMoves: []string{"drain"}}
	if err := validateBeastMoves(&vampire); err != nil {
		t.Errorf("a hands-bearing lifedrain species must drain: %v", err)
	}
}
```

Run `go test ./internal/species/ -count=1`: FAIL (`unknown field BeastMoves`, `undefined: validateBeastMoves`).

- [ ] **Step 2: Field, validator, loader.** In `Species` (`species.go`, after `LifeDrain`):

```go
	// BeastMoves is the special moves this species can use, AI and player
	// alike. A default attack is what a creature does most of the time; a
	// beast move is rarer or a finisher (rulings E and F). The reviewed table
	// is spec 2026-10-16 #541 section 5, pinned by
	// species_beast_moves_test.go. Every beast-move gate (combat.CanUse*,
	// actions.CommandIsReady, each Execute* entry) reads this list, never the
	// attack family; validateBeastMoves checks each move's body at load.
	BeastMoves []string `yaml:"beast_moves,omitempty"`
```

After `validateNaturalAttack`, add (and `"slices"` to the imports):

```go
// beastMoveBody is what a body needs for one beast move: each move keeps the
// anatomy check its gate has always made. drain needs lifedrain and allows
// hands, because armed undead drain (combat/context.md, the hands rule).
type beastMoveBody struct {
	noHands, legs, horns, lifeDrain bool
}

// beastMoveBodies names every move beast_moves may list. charge is absent on
// purpose: it has no AI gate and stays scripted through combatcommands
// (spec section 5).
var beastMoveBodies = map[string]beastMoveBody{
	"rake":      {noHands: true},
	"maul":      {noHands: true},
	"throttle":  {noHands: true},
	"smother":   {noHands: true},
	"pounce":    {noHands: true, legs: true},
	"hamstring": {noHands: true, legs: true},
	"gore":      {noHands: true, horns: true},
	"drain":     {lifeDrain: true},
}

// validateBeastMoves rejects an unknown move, a repeated one, and a move
// whose body this species lacks. It reads BodyParts the way
// characters.Character.HasBodyPart does (no list means no parts), so the
// load check and the runtime gate cannot disagree.
func validateBeastMoves(s *Species) error {
	seen := map[string]bool{}
	for _, move := range s.BeastMoves {
		body, ok := beastMoveBodies[move]
		if !ok {
			return fmt.Errorf("species %d (%s): unknown beast move %q", s.SpeciesId, s.Name, move)
		}
		if seen[move] {
			return fmt.Errorf("species %d (%s): beast move %q listed twice", s.SpeciesId, s.Name, move)
		}
		seen[move] = true
		has := func(part string) bool { return slices.Contains(s.BodyParts, part) }
		switch {
		case body.noHands && has("hands"):
			return fmt.Errorf("species %d (%s): beast move %q needs a body without hands", s.SpeciesId, s.Name, move)
		case body.legs && !has("legs"):
			return fmt.Errorf("species %d (%s): beast move %q needs legs", s.SpeciesId, s.Name, move)
		case body.horns && !has("horns"):
			return fmt.Errorf("species %d (%s): beast move %q needs horns", s.SpeciesId, s.Name, move)
		case body.lifeDrain && !s.LifeDrain:
			return fmt.Errorf("species %d (%s): beast move %q needs lifedrain: true", s.SpeciesId, s.Name, move)
		}
	}
	return nil
}

// HasBeastMove reports whether this species lists move in beast_moves. A nil
// species lists nothing.
func (s *Species) HasBeastMove(move string) bool {
	return s != nil && slices.Contains(s.BeastMoves, move)
}
```

In `LoadDataFiles`, after the siphon check: `if err := validateBeastMoves(s); err != nil { panic(err) }`.

- [ ] **Step 3: Data.** Add one line after `natural_attack:` (for 32 and 33, after `lifedrain:`) in each file, exactly as the table above, e.g. `beast_moves: [maul, throttle, pounce, hamstring]` in `2-canine.yaml`, `beast_moves: [rake]` in `3-bear.yaml`, `beast_moves: [gore]` in `6-boar.yaml` and `7-deer.yaml`, `beast_moves: [rake, pounce]` in `9-raptor.yaml`, `beast_moves: [hamstring]` in `10-rodent.yaml`, `beast_moves: [rake, pounce, hamstring, throttle]` in `11-feline.yaml`, `beast_moves: [pounce]` in `17-arachnid.yaml`, `beast_moves: [maul]` in `21-reptile.yaml`, `beast_moves: [rake]` in `22-bat.yaml`, `beast_moves: [throttle, pounce]` in `24-mustelid.yaml`, `beast_moves: [drain]` in `32-wraith.yaml`, `33-spectre.yaml`, `34-vampire.yaml`, `beast_moves: [smother]` in `44-smoke_elemental.yaml`. No gate reads the list yet, so play is unchanged.
- [ ] **Step 4: Run** the full suite (world data changed). Expected PASS. Commit `feat(species): beast_moves lists each species' special moves (#541)`.

## Task 7: The gates read the list, and the family predicates go (spec section 5; D4, D6)

**Files:** `internal/combat/ai.go`, `internal/combat/ai_test.go`, `internal/actions/command_readiness.go`, `internal/actions/combat_{rake,maul,throttle,gore,hamstring,pounce,drain}.go`, `internal/actions/command_readiness_drift_test.go`, the fixture files of Fact 19, new `internal/combat/beast_move_gates_test.go`, new root `beast_move_gate_guard_test.go`, `internal/actions/context.md`, `internal/combat/context.md`, `internal/mobcommands/context.md`, `internal/usercommands/context.md`.

D4: this task does not touch `aiProfiles` (the `ambush_predator` weights stay; #574). D6 (b): once every gate reads the list, Step 6 deletes the attack-family predicates; this supersedes spec section 4's bullet that they "describe the body".

- [ ] **Step 1: Failing tests.** Create `internal/combat/beast_move_gates_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// beastMoveGates is every CanUse* gate that species data decides.
var beastMoveGates = map[string]func(*characters.Character) bool{
	"rake": CanUseRake, "maul": CanUseMaul, "pounce": CanUsePounce, "gore": CanUseGore,
	"throttle": CanUseThrottle, "hamstring": CanUseHamstring, "drain": CanUseDrain,
}

// Spec section 6: a species whose list omits a move cannot use it even with
// the matching attack family, and a listed move opens the gate. Both halves,
// so the test fails whichever way a gate stops reading the list.
func TestBeastMoveGatesReadTheSpeciesList(t *testing.T) {
	all := []string{"rake", "maul", "pounce", "gore", "throttle", "hamstring", "drain"}
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		9401: {SpeciesId: 9401, Name: "unlisted", BodyParts: []string{"legs", "mouth", "horns"}, NaturalAttack: items.Bite, LifeDrain: true},
		9402: {SpeciesId: 9402, Name: "listed", BodyParts: []string{"legs", "mouth", "horns"}, NaturalAttack: items.Bite, LifeDrain: true, BeastMoves: all},
	}))
	unlisted := &characters.Character{SpeciesId: 9401}
	listed := &characters.Character{SpeciesId: 9402}
	for move, gate := range beastMoveGates {
		if gate(unlisted) {
			t.Errorf("%s: a species that does not list it can use it", move)
		}
		if !gate(listed) {
			t.Errorf("%s: a species that lists it, with the body for it, cannot use it", move)
		}
	}
}

// The table in practice: the gate lets a cat throttle and refuses a rat, a
// bear no longer pounces, a serpent does nothing but bite. (D4: whether the
// cat's AI ever picks throttle is profile tuning, #574; this tests the gate.)
func TestBeastMoveGates_TableExamples(t *testing.T) {
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		11: {SpeciesId: 11, Name: "feline", BodyParts: []string{"legs", "eyes", "mouth", "skin", "tail"}, NaturalAttack: items.Claws,
			BeastMoves: []string{"rake", "pounce", "hamstring", "throttle"}},
		10: {SpeciesId: 10, Name: "rodent", BodyParts: []string{"legs", "eyes", "mouth", "skin", "tail"}, NaturalAttack: items.Bite,
			BeastMoves: []string{"hamstring"}},
		3: {SpeciesId: 3, Name: "bear", BodyParts: []string{"arms", "legs", "eyes", "mouth", "skin"}, NaturalAttack: items.Claws,
			BeastMoves: []string{"rake"}},
		8: {SpeciesId: 8, Name: "serpent", BodyParts: []string{"eyes", "mouth", "skin"}, NaturalAttack: items.Bite},
	}))
	cat, rat, bear, snake := &characters.Character{SpeciesId: 11}, &characters.Character{SpeciesId: 10},
		&characters.Character{SpeciesId: 3}, &characters.Character{SpeciesId: 8}
	if !CanUseThrottle(cat) || CanUseThrottle(rat) {
		t.Error("the gate must allow a cat's throttle and refuse a rat's")
	}
	if CanUsePounce(bear) || CanUseHamstring(bear) || !CanUseRake(bear) {
		t.Error("a bear rakes; it does not pounce or hamstring")
	}
	for move, gate := range beastMoveGates {
		if gate(snake) {
			t.Errorf("a serpent must not %s", move)
		}
	}
}
```

Create `beast_move_gate_guard_test.go` at the repo root:

```go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// familyReads are the reads that decide a beast move by attack family
// instead of by Species.BeastMoves (spec 2026-10-16 #541 section 5).
var familyReads = map[string]bool{
	"SpeciesIsFanged": true, "SpeciesIsClawed": true, "SpeciesIsHorned": true,
	"SpeciesIsQuadrupedPredator": true, "SpeciesHasLifeDrain": true,
	"SpeciesNaturalAttack": true, "NaturalAttack": true,
}

// familyReadsIn names each family read inside a gate function of src.
func familyReadsIn(t *testing.T, src string, isGate func(string) bool) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	require.NoError(t, err)
	var out []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !isGate(fn.Name.Name) {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && familyReads[id.Name] {
				out = append(out, fn.Name.Name+" reads "+id.Name)
			}
			return true
		})
	}
	return out
}

// Every beast-move gate reads the species list. The three sync points:
// CanUse* in ai.go, CommandIsReady, and each Execute* entry.
func TestBeastMoveGatesNeverReadTheAttackFamily(t *testing.T) {
	beastGate := func(name string) bool {
		for _, m := range []string{"Rake", "Maul", "Pounce", "Gore", "Throttle", "Hamstring", "Drain", "Smother"} {
			if name == "CanUse"+m || name == "Execute"+m {
				return true
			}
		}
		return name == "CommandIsReady"
	}
	files := []string{"internal/combat/ai.go", "internal/actions/command_readiness.go"}
	actionFiles, err := filepath.Glob("internal/actions/combat_*.go")
	require.NoError(t, err)
	require.NotEmpty(t, actionFiles)
	files = append(files, actionFiles...)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		for _, hit := range familyReadsIn(t, string(raw), beastGate) {
			t.Errorf("%s: %s; beast moves come from Species.BeastMoves", f, hit)
		}
	}
}

func TestFamilyReadsIn_CanFail(t *testing.T) {
	src := "package p\nfunc CanUseRake(c int) bool { return SpeciesIsClawed(c) }\n" +
		"func ExecuteHamstring(a int) { if sp.NaturalAttack != 1 {} }\nfunc other() { SpeciesIsFanged(1) }\n"
	got := familyReadsIn(t, src, func(n string) bool { return n == "CanUseRake" || n == "ExecuteHamstring" })
	require.Len(t, got, 2, "two gate reads and none from a non-gate: %v", got)
}

// scriptedBeastMoveFindings names each scripted combatcommands beast move
// the mob's species does not list: the action gate would refuse it in
// silence every time.
func scriptedBeastMoveFindings(mobCommands map[string][]string, mobSpecies map[string]int, lists map[int][]string) []string {
	beast := map[string]bool{"rake": true, "maul": true, "pounce": true, "gore": true,
		"throttle": true, "hamstring": true, "drain": true, "smother": true}
	var out []string
	for file, cmds := range mobCommands {
		for _, c := range cmds {
			verb := strings.Fields(c + " ")[0]
			if !beast[verb] {
				continue
			}
			listed := false
			for _, m := range lists[mobSpecies[file]] {
				listed = listed || m == verb
			}
			if !listed {
				out = append(out, file+": scripts "+verb+" but species does not list it")
			}
		}
	}
	sort.Strings(out)
	return out
}

func TestScriptedBeastMovesAreInTheSpeciesList(t *testing.T) {
	lists := map[int][]string{}
	spFiles, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "species", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, spFiles)
	for _, f := range spFiles {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		var sp struct {
			Id    int      `yaml:"speciesid"`
			Moves []string `yaml:"beast_moves"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &sp), f)
		lists[sp.Id] = sp.Moves
	}
	cmds, species := map[string][]string{}, map[string]int{}
	require.NoError(t, filepath.WalkDir(filepath.Join("_datafiles", "world", "dogmud", "mobs"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".yaml" {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var m struct {
			CombatCommands []string `yaml:"combatcommands"`
			Character      struct {
				SpeciesId int `yaml:"speciesid"`
			} `yaml:"character"`
		}
		if yaml.Unmarshal(raw, &m) == nil {
			cmds[p], species[p] = m.CombatCommands, m.Character.SpeciesId
		}
		return nil
	}))
	require.Greater(t, len(cmds), 100, "the mob walk found too few files to mean anything")
	for _, f := range scriptedBeastMoveFindings(cmds, species, lists) {
		t.Error(f)
	}
}

func TestScriptedBeastMoveFindings_CanFail(t *testing.T) {
	got := scriptedBeastMoveFindings(map[string][]string{"wolf.yaml": {"rake", "charge", "emote howls"}},
		map[string]int{"wolf.yaml": 2}, map[int][]string{2: {"maul"}})
	require.Equal(t, []string{"wolf.yaml: scripts rake but species does not list it"}, got)
}
```

Run `go test ./internal/combat/ -run TestBeastMoveGates -count=1` and `go test . -run 'TestBeastMoveGatesNeverReadTheAttackFamily|TestFamilyReadsIn_CanFail|TestScripted' -count=1`. Expected: the gate tests and the family guard FAIL; both probes and the scripted-move guard PASS (Fact 27).

- [ ] **Step 2: ai.go.** Add, after `SpeciesHasLifeDrain` (Step 6 deletes the predicates around it, so it then stands where they stood):

```go
// SpeciesHasBeastMove reports whether the character's species lists move in
// beast_moves. It is the only species read a beast-move gate makes; no gate
// decides a move by attack family. Exported for the readiness and
// action-entry gates in internal/actions.
func SpeciesHasBeastMove(char *characters.Character, move string) bool {
	return species.GetSpecies(char.SpeciesId).HasBeastMove(move)
}
```

Above `CanUseBash` (the "Viability checks" block) add:

```go
// Beast moves (rake, maul, pounce, gore, throttle, hamstring, drain,
// smother) come from species data, Species.BeastMoves, never from the attack
// family: a default attack is what a creature does most of the time and a
// special move is its rarer flourish (spec 2026-10-16 #541, rulings E and
// F). Each gate keeps its runtime anatomy check (no hands, legs) beside the
// list. beast_move_gate_guard_test.go fails a gate that reads the family.
```

Gates become (bodies after the cooldown check):

```go
// CanUseHamstring
	if !char.HasBodyPart("legs") || char.HasBodyPart("hands") {
		return false
	}
	return SpeciesHasBeastMove(char, "hamstring")
// CanUseRake
	if char.HasBodyPart("hands") {
		return false // beast natural-weapon moves are for true beasts, not tool-users
	}
	return SpeciesHasBeastMove(char, "rake")
// CanUseMaul, CanUseGore, CanUseThrottle: the same shape with "maul", "gore", "throttle".
// CanUsePounce
	if char.IsGrappling() {
		return false // can't leap from a clinch
	}
	if char.HasBodyPart("hands") || !char.HasBodyPart("legs") {
		return false
	}
	return SpeciesHasBeastMove(char, "pounce")
// CanUseDrain
	return SpeciesHasBeastMove(char, "drain")
```

Rewrite each gate's doc comment to say the move is "listed in the species' beast_moves" in place of "requires a fanged/clawed/horned natural attack". Delete `SpeciesIsQuadrupedPredator` (`:564-571`, D6: it was a pounce gate). The other predicates still have callers until Step 3; Step 6 deletes them.

- [ ] **Step 3: Readiness and entries.** `command_readiness.go` cases become:

```go
	case "rake":
		return char.IsInCombat() && !char.HasBodyPart("hands") && combat.SpeciesHasBeastMove(char, "rake")
	case "maul":
		return char.IsInCombat() && !char.HasBodyPart("hands") && combat.SpeciesHasBeastMove(char, "maul")
	case "throttle":
		return char.IsInCombat() && !char.HasBodyPart("hands") && combat.SpeciesHasBeastMove(char, "throttle")
	case "pounce":
		return char.IsInCombat() && !char.IsGrappling() && !char.HasBodyPart("hands") &&
			char.HasBodyPart("legs") && combat.SpeciesHasBeastMove(char, "pounce")
	case "gore":
		return char.IsInCombat() && !char.HasBodyPart("hands") && combat.SpeciesHasBeastMove(char, "gore")
	case "hamstring":
		if !char.IsInCombat() || !char.HasBodyPart("legs") || char.HasBodyPart("hands") {
			return false
		}
		return combat.SpeciesHasBeastMove(char, "hamstring")
	case "drain":
		return char.IsInCombat() && combat.SpeciesHasBeastMove(char, "drain")
```

Action entries (one line each, same line count, so the `condition_apply_path_guard_test.go` keys do not move; re-run that guard to confirm):

```go
// combat_rake.go:83
	if char.HasBodyPart("hands") || !combat.SpeciesHasBeastMove(char, "rake") {
// combat_maul.go:83
	if char.HasBodyPart("hands") || !combat.SpeciesHasBeastMove(char, "maul") {
// combat_throttle.go:95
	if char.HasBodyPart("hands") || !combat.SpeciesHasBeastMove(char, "throttle") {
// combat_gore.go:78
	if char.HasBodyPart("hands") || !combat.SpeciesHasBeastMove(char, "gore") {
// combat_pounce.go:93
	if char.HasBodyPart("hands") || !char.HasBodyPart("legs") || !combat.SpeciesHasBeastMove(char, "pounce") {
// combat_drain.go:96
	if !combat.SpeciesHasBeastMove(char, "drain") {
// combat_hamstring.go:91-92 (the `sp :=` line goes; drop the now-unused species and items imports)
	if char.HasBodyPart("hands") || !combat.SpeciesHasBeastMove(char, "hamstring") {
```

Update each entry's comment and each result field's doc comment (`NotFanged`, `NotClawed`, `NotHorned`, `NotPredator`, `NotBeast`, `NotLifeDrainer`) to "the species does not list this move in beast_moves, or the body lacks what it needs". The field names and the player refusal texts stay.

- [ ] **Step 4: Drift test.** In `TestSpecialMoveAdmissionOrdering` set the gate expressions exactly to the Step 3 lines (`"char.HasBodyPart(\"hands\") || !combat.SpeciesHasBeastMove(char, \"rake\")"`, and so on, hamstring keeping its `NoLegs` row before it). In `TestCommandReadinessDrift` give the seeded species lists: 7304 `BeastMoves: []string{"rake", "pounce", "hamstring"}`, 7305 `{"maul", "throttle", "pounce", "hamstring"}`, 7306 `{"gore"}`, 7307 `{"drain"}`, 7309 `{"maul", "throttle"}`, 7310 to 7312 the same lists as their identity (hands must still block). Add species `7314: {SpeciesId: 7314, Name: "unlisted-clawed-test", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Claws}` and three rows that must be not-ready: `{"rake_notlisted", "rake", ...SpeciesId = 7314..., false, "NotClawed"}`, `{"pounce_notlisted", "pounce", ..., false, "NotPredator"}`, `{"hamstring_notlisted", "hamstring", ..., false, "NotBeast"}`, each with a registered target as `throttle_notfanged` does.
- [ ] **Step 5: Fixture sweep.** Run `go test ./internal/actions/... ./internal/combat/... ./internal/behaviortree/... ./internal/mobcommands/... ./internal/usercommands/... -count=1`. For each failure in the files of Fact 19, give the seeded species the moves its old family granted it, so the test keeps its meaning: a bite fixture with legs `{"maul", "throttle", "pounce", "hamstring"}`, without legs `{"maul", "throttle"}`; a claws fixture with legs `{"rake", "pounce", "hamstring"}`, without legs `{"rake"}`; gore `{"gore"}`; lifedrain `{"drain"}`. A fixture that asserts a species CANNOT use a move (for example `TestThrottle_NotFanged`'s feline) keeps a list without that move. `ai_test.go`'s `TestBeastIdentityPredicates` is left for Step 6, which deletes it. Re-run until PASS.
- [ ] **Step 6: Delete the attack-family predicates (D6 (b)).** First prove the caller search can match, BEFORE deleting anything:

```bash
P='\b(SpeciesIsFanged|SpeciesIsClawed|SpeciesIsHorned|SpeciesHasLifeDrain|SpeciesIsQuadrupedPredator|SpeciesNaturalAttack)\('
grep -rnE "$P" --include=*.go . | grep -v _test.go    # expect ONLY the five definitions left (Step 2 deleted
                                                       # SpeciesIsQuadrupedPredator) and the three
                                                       # SpeciesNaturalAttack calls inside them (ai.go:248-263)
grep -rnE "$P" --include=*.go . | grep _test.go       # expect ai_test.go:605-614 (TestBeastIdentityPredicates)
```

The first command must print the definitions: that is the proof the pattern matches the symbols it is meant to catch (on master `8e4727129`, before Task 7, the same pattern hits `ai.go:248,254,255,257,258,260,261,263,569` as definitions and calls, plus the gate calls of Fact 18: proof it can match). If it prints any other production line, a gate was missed in Steps 2 and 3: fix that first. Then, in `internal/combat/ai.go`, delete `SpeciesNaturalAttack`, `SpeciesIsFanged`, `SpeciesIsClawed`, `SpeciesIsHorned` and `SpeciesHasLifeDrain` (`:248-266` on master; re-read the lines first), rewrite the section header above them (`:244-246`, "Beast-identity predicates (single source of truth for beast-move gating)") to head `SpeciesHasBeastMove` as the one species read the gates make, and drop any import only they used (`go build ./...` says). In `internal/combat/ai_test.go` delete `TestBeastIdentityPredicates` (`:592-615`). The drift test's string literals for the old gate expressions (`command_readiness_drift_test.go:172-191`) were already rewritten in Step 4. Run the same two greps again: expect no output from either (run each alone: an empty grep exits 1). Keep the names in the root guard's `familyReads` map (Step 1): it then guards against any of them coming back.

Docs that name them (Fact 18): `internal/combat/context.md:832-836` becomes one line, "Beast moves are decided by `SpeciesHasBeastMove` (the species' `beast_moves`); the attack-family predicates were deleted in #541 slice 2 (D6): #563 will want a proper anatomy model"; `internal/actions/context.md:220-221`, `internal/mobcommands/context.md:41-42` and `internal/usercommands/context.md:75-76` name `combat.SpeciesHasBeastMove` in place of the predicate list. Then `grep -rnE "SpeciesIs(Fanged|Clawed|Horned|QuadrupedPredator)|SpeciesHasLifeDrain|SpeciesNaturalAttack" internal modules --include=*.md` prints nothing (run alone), and `python tools/context_md_audit.py internal/combat`, then `internal/actions`, `internal/mobcommands`, `internal/usercommands`, one per call, report no phantom symbol.
- [ ] **Step 7: Bears' charge (spec section 5).** Already scripted on 9541 and 9547 (Fact 14); no data change. `TestScriptedBeastMovesAreInTheSpeciesList` is the record.
- [ ] **Step 8: Run** the full suite; PASS. Commit `feat(combat): beast-move gates read species beast_moves (#541)` with the files above named; body: the three sync points, the drift rows added, D6 (the attack-family predicates and `SpeciesNaturalAttack` deleted, no production caller left, superseding spec section 4's "describe the body" bullet), D4 (no AI profile change, #574), the bird and serpent losing their family moves per the table.

## Task 8: `pounce.yaml`'s D3 record (D3; D7 amended)

**D7 AMENDED 2026-10-19: Steps 1 to 5 below (measure, draft, check, owner gate, apply) are NOT run.** The owner reviewed `maul.yaml` and `pounce.yaml` and ships them as written (see D7). What remains of this task:

- [ ] **Step A: D3.** Replace `pounce.yaml`'s normal/master record (today's lines 240-242; re-locate by content, "flash") with the D3-approved YAML shown in Step 2 below, by Edit. Nothing else in either file changes.
- [ ] **Step B: The `forward` false positive.** Fact 29: `pounce.yaml`'s heavy/master line "drive your {itemname} forward" trips `TestNaturalWeaponPoolsNeverPutAVerbAfterTheWeapon` once cats use pounce (Task 9). Add `"forward": true,` to `weaponNotSubject` beside `"home"` (a direction word, like "down" and "up") and add `` `You drive your {itemname} forward and crash in!` `` to the no-verb list of `TestSubjectVerbAfterItemName_CatchesShippedDefects` so the entry is exercised. Re-run the scratch replica of `subjectVerbAfterItemName` over both files: any OTHER hit goes to the owner as a copy question, never into the allowlist.
- [ ] **Step C: Goldens and commit.** Steps 6 to 8 below apply, with the commit message `content(combat): pounce's flashing record names the creature, not its claws (#541, D3)` and a body naming the D3 record and the `forward` guard word.

(The original steps follow for the record; skip Steps 1 to 5.)

Modelled on the slam split's copy process (`docs/superpowers/plans/completed/2026-10-16-slam-split.md` Tasks 1 to 3: agents draft, controller checks, owner reviews one table per file, then the approved copy lands with a width guard). It runs before Task 9 switches bears and cats onto these files, so no player ever reads an over-width maul or pounce line.

**Files:** `_datafiles/world/dogmud/combat-messages/maul.yaml`, `_datafiles/world/dogmud/combat-messages/pounce.yaml`, `internal/narration/testdata/stores/combat_messages.golden`. Scratch (session scratchpad, full path): `measure_width.py`, `maul-draft.yaml`, `pounce-draft.yaml`, `maul-review.md`, `pounce-review.md`.

- [ ] **Step 1: Measure.** Write `<scratchpad>/measure_width.py` with the Write tool, mirroring `TestSlamSplitLinesFitEightyColumns` (`internal/items/slam_split_copy_test.go:138-148`) exactly: walk every role line of a file (`options` / intensity / `together` / tier / record / role), replace `{itemname}` with the `unarmedname` given, every other token with its value in `slamSplitWidestTokens` (`:32-44`: names 20 characters, `{position}` "grappling on the ground", `{momentum}` "on the defensive", `{stance}` "aggressive", `{bodypart}` "right shoulder", `{damage}` "999"), strip `</?ansi[^>]*>`, count characters, fail any token left unfilled. Read the two `unarmedname` values from the species files at run time, not from this plan (on master: bear 3 "claws and teeth", feline 11 "claws and fangs"; Fact 30). Run it on today's files with `python -I`. Expected (Fact 30): `maul.yaml` 149 of 345 over 80, `pounce.yaml` 121 of 345. A count of 0 means the script is broken, not the data: that run is the probe that the measure can fail.
- [ ] **Step 2: Draft (two Opus agents in parallel, read-only on code).** One agent per file. Each writes `<scratchpad>/<file>-draft.yaml` (the whole file, same intensities, tiers and record counts as today) and `<scratchpad>/<file>-review.md`, a table `| intensity | tier | # | role | today | proposed | cols | why |` holding every record with any role over 80, each role of that record on its own row (all three roles shown even when only one changes, so the reviewer reads the moment whole). Rules every agent brief carries, beside the standing ones at the top of this plan:
  - Rewrite only records with a role over 80 columns by Step 1's measure. A record whose roles all fit is copied unchanged.
  - Each record is ONE moment told three ways: actor, actee and observer describe the same blow, same body part, same outcome. Changing one role means re-reading the other two.
  - The #455 rule: `{itemname}` is never the subject of a verb ("Your {itemname} flash" is wrong); the creature does the action ("You rake with your {itemname}"). The draft is checked against `subjectVerbAfterItemName` (`internal/items/natural_weapon_agreement_test.go`) by a scratch replica in Step 3.
  - Banner agreement: a record whose roles carry a banner word (`<ansi fg="...-bold">WORD</ansi>`) carries the same word in every role (`combat_banner_agreement_guard_test.go:45,145`).
  - No retired idiom: nothing matching `retiredCombatIdiom` (`combat_idiom_guard_test.go:18`: clip, goes wide, slips most, sliver, footing, overrun, carries straight past).
  - ESL-clear plain English, no raw numbers beyond the `{damage}` token where today's line carries it, no em or en dashes, no "its" for an actor, a space before every ansi tag, every token the line needs kept.
  - Bodies: maul is a bear (swiping claws, a crushing bite, its weight); pounce is a big cat (a leap, raking claws, fangs). Neither names hands, fists or a held weapon.
  - D3: `pounce.yaml`'s normal/master record at lines 240-242 is ALREADY APPROVED and goes in exactly as below; the review table lists it marked "approved (D3)", not for review:

```yaml
      - actor: 'You press <ansi fg="{acteetype}">{actee}</ansi> with masterful control, <ansi fg="item">{itemname}</ansi> flashing!'
        actee: '<ansi fg="{actortype}">{actor}</ansi> presses you hard, <ansi fg="item">{itemname}</ansi> flashing!'
        observer: '<ansi fg="{actortype}">{actor}</ansi> presses <ansi fg="{acteetype}">{actee}</ansi>, <ansi fg="item">{itemname}</ansi> flashing!'
```

  Each agent reports its width-script output and `read N of N` for the files it modelled on (the file it rewrites, `slam.yaml` or `trample.yaml` for the slam split's register, and `narration/special-moves/pounce.yaml` / `maul.yaml` for the move's own voice), each printed in chunks of at most 400 lines.
- [ ] **Step 3: Controller check** of each draft: it parses; intensities, tiers and record counts per tier match today's file; `measure_width.py` on the draft reports 0 over 80; the scratch replica of `subjectVerbAfterItemName` (with `weaponNotSubject` copied from the test file) reports nothing, and catches the probe `Your {itemname} bites into you!`; the banner words agree per record; `retiredCombatIdiom` finds nothing; `grep -nP "[\x{2013}\x{2014}]"` finds nothing. Records under 80 are byte-identical to today's (diff the draft against the file and confirm every changed record appears in the review table).
- [ ] **Step 4: Owner gate.** Open both review tables for the owner (`code "C:\Users\Calabe Davis\workspace\DOGMud" "<scratchpad>\maul-review.md"`, the same for pounce), with a counts header per file (records reviewed, lines over 80 before and after). The owner approves or edits each table; apply edits to the drafts, re-run Step 3, and record the approval date per file here: maul ____, pounce ____. Do not start Step 5 before both are approved.
- [ ] **Step 5: Apply.** `cp` each approved draft over its file in `_datafiles/world/dogmud/combat-messages/` (H3: no shell write into the tree except `cp` from scratch). `git diff --stat` shows only the two files.
- [ ] **Step 6: Goldens.** `go test ./internal/narration/... -run TestSnapshotStores -update -count=1`, then `git diff --stat internal/narration/testdata/stores/`: only `combat_messages.golden` changes, and only in maul and pounce rows. Read the diff; quote its size in the commit.
- [ ] **Step 7: Run** the full `go test . ./internal/... ./modules/... -count=1` (combat-message data changed). PASS. The Go width guard over these two files lands in Task 9 Step 1, because it reads the longest `unarmedname` of the species USING each file (`slamSplitLongestItemName`, `slam_split_copy_test.go:93-120`) and no species uses maul or pounce until Task 9.
- [ ] **Step 8: Commit.**

```bash
git add _datafiles/world/dogmud/combat-messages/maul.yaml _datafiles/world/dogmud/combat-messages/pounce.yaml \
  internal/narration/testdata/stores/combat_messages.golden
git commit -F - <<'EOF'
content(combat): maul and pounce lines fit 80 columns (#541)

Owner ruling D7: every maul.yaml and pounce.yaml line over 80 visible
columns at 20-character names is rewritten before bears and cats use
the files (maul <N> records, pounce <N>; 149 and 121 role lines were
over). Each record keeps one moment across its three roles; the weapon
is never the subject of a verb (#455). pounce's normal/master record is
the D3-approved text. Owner review approved <date>. Golden: <rows>.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

## Task 9: `maul` and `pounce` as default attacks (spec section 4; D6, D7)

**Files:** `internal/items/itemspec.go`, `internal/species/species.go`, `internal/combat/attackresult.go`, `internal/items/reach.go`, `internal/items/reach_test.go`, `internal/items/natural_weapon_agreement_test.go`, `internal/items/slam_split_copy_test.go`, `internal/combat/combat_helpers.go` (comment), `_datafiles/world/dogmud/species/3-bear.yaml`, `11-feline.yaml`, new `internal/species/species_maul_pounce_test.go`, new `internal/combat/maul_pounce_render_test.go`.

Spec section 4's bullet that the attack-family predicates "describe the body" is superseded by D6 (b): Task 7 Step 6 deleted them, so this task neither extends nor tests them.

- [ ] **Step 1: Failing tests.** `internal/species/species_maul_pounce_test.go`:

```go
package species

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Ruling D: bears' default attack is maul, cats' is pounce.
func TestBearsMaulAndCatsPounce(t *testing.T) {
	all := dogmudSpecies(t)
	if got := all[3].NaturalAttack; got != items.Maul {
		t.Errorf("bear natural_attack = %q, want maul", got)
	}
	if got := all[11].NaturalAttack; got != items.Pounce {
		t.Errorf("feline natural_attack = %q, want pounce", got)
	}
	for _, sub := range []items.ItemSubType{items.Maul, items.Pounce} {
		if err := validateNaturalAttack(&Species{SpeciesId: 1, Name: "x", NaturalAttack: sub}); err != nil {
			t.Errorf("%s must be a valid natural attack: %v", sub, err)
		}
	}
}
```

`internal/combat/maul_pounce_render_test.go`, modelled on `TestSlamSplit_EachBodyRendersFromItsOwnFile`:

```go
package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// A bear's swing comes from maul.yaml and a cat's from pounce.yaml, both in
// the natural-sharp band; claws and generic are loaded too, so a species
// left on claws would match the wrong file.
func TestMaulAndPounceRenderFromTheirOwnFiles(t *testing.T) {
	subs := []items.ItemSubType{items.Maul, items.Pounce, items.Claws, items.Generic}
	pools := map[items.ItemSubType]*items.WeaponAttackMessageGroup{}
	for _, sub := range subs {
		pools[sub] = slamSplitLoadFile(t, sub)
	}
	t.Cleanup(items.SeedAttackMessagesForTest(pools))
	cases := []struct {
		file string
		want items.ItemSubType
	}{{"3-bear.yaml", items.Maul}, {"11-feline.yaml", items.Pounce}}
	seeded := map[int]*species.Species{}
	for _, c := range cases {
		sp := slamSplitLoadSpecies(t, c.file)
		seeded[sp.SpeciesId] = sp
	}
	t.Cleanup(species.SeedSpeciesForTest(seeded))
	for _, c := range cases {
		sp := slamSplitLoadSpecies(t, c.file)
		matchers := slamSplitActorMatchers(pools[c.want])
		t.Run(sp.Name, func(t *testing.T) {
			if got := CategoryForWeaponSubtype(c.want); got != messaging.CategoryHitNaturalSharp {
				t.Fatalf("%s band = %v, want CategoryHitNaturalSharp", c.want, got)
			}
			src, tgt := defenceFixture(1000)
			src.SpeciesId = sp.SpeciesId
			src.Name = "Rurik"
			tgt.Name = "Selka"
			tgt.HealthMax.Base = 100
			tgt.HealthMax.Recalculate()
			ws := buildWeaponSetup(src, tgt, items.Item{}, 0, 1)
			if ws.weaponSubType != c.want {
				t.Fatalf("%s swings as %q, want %q", sp.Name, ws.weaponSubType, c.want)
			}
			for range 20 {
				result := &AttackResult{}
				buildAttackMessages(result, src, tgt, ws, swingDamageParams{dmgMean: 20},
					10, 0, 0, 0, User, User, "", false, false)
				if len(result.MessagesToSource) == 0 {
					t.Fatalf("%s built no attacker line", sp.Name)
				}
				line := strings.TrimSpace(slamSplitTag.ReplaceAllString(result.MessagesToSource[0].Text, ``))
				if !slamSplitMatchesAny(line, matchers) {
					t.Fatalf("%s line is not from %s.yaml: %q", sp.Name, c.want, line)
				}
			}
		})
	}
}
```

Add `Maul: 0.15, Pounce: 0.15` to the table in `reach_test.go`.

**D7 AMENDED 2026-10-19: skip this paragraph; maul and pounce are NOT added to the width guard, and Step 3 runs `TestNaturalWeaponPoolsNeverPutAVerbAfterTheWeapon` only.** The width guard (D7): in `internal/items/slam_split_copy_test.go` set `var slamSplitFiles = []ItemSubType{Slam, Trample, Lash, Blast, Maul, Pounce}` and its comment to `the four files the split wrote or rewrote, and maul and pounce, rewritten to 80 columns before bears and cats used them (#541 slice 2, owner ruling D7)`. The guard is generic (Fact 30): it measures each listed file with the longest `unarmedname` of the species using it, so after Step 2 it reads "claws and teeth" for maul and "claws and fangs" for pounce from species data. Its can-fail probe already exists (`TestSlamSplitWidthCheck_CanFail`, `:151-156`); the pre-rewrite files were the live proof (Task 8 Step 1: 149 and 121 over). Before Step 2 this guard also fails, with `no dogmud species uses "maul"`, so it is part of the failing set.

Run: FAIL (`undefined: items.Maul`).

- [ ] **Step 2: Wire the two subtypes** in every place the slam split wired trample:

```go
// internal/items/itemspec.go, const block after Blast:
	Maul        ItemSubType = "maul"    // bears: a swipe and a crushing bite (#541 slice 2)
	Pounce      ItemSubType = "pounce"  // big cats: leaping claws and fangs (#541 slice 2)
// ItemSubtypes(), after Blast:
		{string(Maul), `A bear's swiping claws and crushing bite.`, 0, 0, 0},
		{string(Pounce), `A big cat's leaping claws and fangs.`, 0, 0, 0},
// internal/species/species.go validNaturalAttacks:
	items.Maul:    {},
	items.Pounce:  {},
// internal/combat/attackresult.go:
	case items.Claws, items.Bite, items.Sting, items.Gore, items.Maul, items.Pounce:
		return messaging.CategoryHitNaturalSharp
// internal/items/reach.go:
	case Claws, Maul, Pounce:
		return 0.15 // maul and pounce strike with claws too (#541 slice 2)
// internal/items/natural_weapon_agreement_test.go:
var naturalWeaponPools = []ItemSubType{Claws, Bite, Sting, Gore, Slam, Siphon, Trample, Lash, Blast, Maul, Pounce}
```

Delete `naturalWeaponSpecialPools` and its loop in `naturalWeaponPoolFiles` (maul is now a natural pool; throttle's file is gone). Add `Maul, Pounce` to the natural-blunt list in `meleeDisplaySubtype`'s comment. Set `natural_attack: maul` in `3-bear.yaml` and `natural_attack: pounce` in `11-feline.yaml`.

- [ ] **Step 3: The natural-weapon guard now covers pounce.** `go test ./internal/items/ -run 'TestNaturalWeaponPoolsNeverPutAVerbAfterTheWeapon|TestSlamSplitLinesFitEightyColumns' -count=1` must PASS: Task 8 already replaced the `pounce.yaml:240-242` record with the D3-approved text and checked every draft line against the same rule. If the guard still reports `forward` after `{itemname}` (the false positive at today's `:285`, Fact 29) because an approved line kept "drive your {itemname} forward", add `"forward": true,` to `weaponNotSubject` beside `"home"` (a direction word, like "down" and "up") and add `` `You drive your {itemname} forward and crash in!` `` to the no-verb list of `TestSubjectVerbAfterItemName_CatchesShippedDefects` so the entry is exercised; if no approved line carries it, leave the allowlist alone. Any other hit is a Task 8 defect: fix the copy through the owner, not the guard.
- [ ] **Step 4: Goldens.** `go test ./internal/narration/... -run TestSnapshotStores -count=1` (no `-update`) PASSES: switching the species changes no store text (Task 8 landed the copy).
- [ ] **Step 5: Run** the full suite; PASS. Commit `feat(combat): bears maul and cats pounce in ordinary combat (#539, #541)` with the files above named; body: the six wiring sites, the width guard extended to maul and pounce (D7), the `forward` guard word if it was needed.

## Task 10: The `Smothered` condition and the `smother` narration (Appendix A; D1)

**Files:** new `_datafiles/world/dogmud/conditions/144-smothered.yaml`, new `_datafiles/world/dogmud/narration/special-moves/smother.yaml`, `internal/conditions/ids.go`. These land in the same commit as Task 11, because `TestMoveEventKeysAgree` fails an event no Go site names; Task 10 stages, Task 11 commits.

- [ ] **Step 1: Condition.** Write `_datafiles/world/dogmud/conditions/144-smothered.yaml` (Appendix A with the id; tick values equal Throttled 89, Fact 24):

```yaml
conditionid: 144
name: Smothered
description: Smoke fills your lungs, and you can barely breathe.
start_actee: Smoke pours down your throat and you choke on it.
trigger_actee: You cough up smoke and fight for breath.
end_actee: You cough out the last of the smoke and breathe freely again.
triggerrate: 1 round
triggercount: 3
tick_pool: stamina
tick_percent: -0.06
tick_min: 2
flags:
  # The smother combat move narrates the smoke as it lands, and it has to
  # apply the condition synchronously, so Condition_ApplyConditions never
  # sees this condition and the start_actee line above is not its own line
  # to send.
  - silent-start
```

In `internal/conditions/ids.go` add `ConditionIdSmothered = 144 // smoke in the lungs; the smother move applies it (#541 slice 2)`.

- [ ] **Step 2: Narration.** Write `_datafiles/world/dogmud/narration/special-moves/smother.yaml`. Per D1 (a) it is Appendix A's file with every `actor:` block removed and the header line "Mob-only ... no actor role: a mob has no client, as charge.yaml and hamstring.yaml" added; `cast_interrupt` keeps its `actee` and `observer` lines:

```yaml
moveid: smother
# Mob-only: the smoke elemental's special move (slam split spec, section 3).
# No actor role: a mob has no client (as charge.yaml and hamstring.yaml).
# Variant N of every role describes one moment. On a hit the target is
# Blinded (condition 3) and Smothered (144); the hit lines say both.
# The attacker has no body: smoke and embers only.
events:
  hit:
    actee:
      - '{actor} pours into your eyes and lungs! (<ansi fg="damage">{damage}</ansi>)'
      - '{actor} fills your eyes and throat with smoke! (<ansi fg="damage">{damage}</ansi>)'
      - '{actor} wraps you in choking, blinding smoke! (<ansi fg="damage">{damage}</ansi>)'
      - '{actor}''s embers blind you as you choke! (<ansi fg="damage">{damage}</ansi>)'
      - '{actor} smothers you. You gasp, blinded! (<ansi fg="damage">{damage}</ansi>)'
    observer:
      - '{actor} pours into {actee}''s eyes and lungs!'
      - '{actor} fills {actee}''s eyes and throat with smoke!'
      - '{actor} wraps {actee} in choking, blinding smoke!'
      - '{actor}''s embers blind {actee}, who chokes!'
      - '{actor} smothers {actee}, who gasps, blinded!'
  # Smoke chokes a spell off mid-cast, as throttle does (combat_throttle.go).
  # It rides the shared special-move cooldown, so at most once per window.
  cast_interrupt:
    actee:
      - '{actor}''s smoke chokes your spell off mid-word!'
      - 'Your spell collapses as you cough and gasp!'
    observer:
      - '{actor}''s smoke chokes {actee}''s spell off mid-word!'
      - '{actee} coughs and gasps, and their spell collapses!'
  # A partial applies no condition (conditions ride on hit only),
  # so these lines claim neither blindness nor choking.
  partial:
    actee:
      - 'You turn aside, but {actor}''s embers sting! (<ansi fg="damage">{damage}</ansi>)'
      - 'You hold your breath; {actor} still stings! (<ansi fg="damage">{damage}</ansi>)'
      - 'You twist away, but {actor} singes you! (<ansi fg="damage">{damage}</ansi>)'
    observer:
      - '{actee} turns aside, but {actor}''s embers sting!'
      - '{actee} holds their breath, but {actor} still stings!'
      - '{actee} twists away, but {actor} singes them!'
  miss:
    actee:
      - '{actor} billows toward your face, but misses!'
      - '{actor}''s smoke drifts past you!'
      - 'You duck under {actor}''s rolling smoke!'
      - 'You step clear before {actor} can engulf you!'
    observer:
      - '{actor} billows toward {actee}''s face, but misses!'
      - '{actor}''s smoke drifts past {actee}!'
      - '{actee} ducks under {actor}''s rolling smoke!'
      - '{actee} steps clear before {actor} can engulf them!'
```

This is Appendix A (lines 153 to 221 of the spec) less its `actor` lines, per D1 (a); if players ever gain smother, the actor lines come back with that feature. Every Appendix A line was measured at 20-character names and `{damage}` as "devastating wounds": 42 of 42 fit 80 columns, so the 28 shipped here (42 less the 14 actor lines) do too. This is approved copy; nothing new is authored.

## Task 11: The `smother` move (Appendix A; D5)

**Files:** `internal/actionspec/action.go`, `internal/costs/action.go`, `internal/costs/action_test.go`, new `internal/actions/choke_interrupt.go`, `internal/actions/combat_throttle.go`, new `internal/actions/combat_smother.go`, new `internal/actions/combat_smother_test.go`, `internal/actions/command_readiness.go`, `internal/actions/action_readiness.go`, `internal/actions/divergences.go`, `internal/actions/action_cost_test.go`, `internal/actions/command_readiness_drift_test.go`, new `internal/mobcommands/smother.go`, `internal/mobcommands/mobcommands.go`, `internal/mobcommands/command_parity_test.go`, `internal/behaviortree/actions_mob.go`, `internal/combat/ai.go`, `internal/combat/beast_move_gates_test.go`, `internal/items/defensive_messages_newly_defendable_test.go`, `condition_apply_path_guard_test.go`, `move_narration_migration_guard_test.go`, `shipped_narration_data_guard_test.go`, goldens, plus Task 10's two files.

- [ ] **Step 1: Failing tests.** Create `internal/actions/combat_smother_test.go`:

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const smokeSpeciesForTest = 7444

func seedSmotherFixture(t *testing.T, targetID, targetDex int) *mobs.Mob {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		perception.ConditionIdBlinded:   {ConditionId: perception.ConditionIdBlinded, Name: "Blinded", TriggerCount: 3, RoundInterval: 1},
		conditions.ConditionIdSmothered: {ConditionId: conditions.ConditionIdSmothered, Name: "Smothered", TriggerCount: 3, RoundInterval: 1},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		smokeSpeciesForTest: {SpeciesId: smokeSpeciesForTest, Name: "smoke-test", BodyParts: []string{}, BeastMoves: []string{"smother"}},
		7445:                {SpeciesId: 7445, Name: "clawed-unlisted", BodyParts: []string{"legs"}},
	}))
	target := &mobs.Mob{InstanceId: targetID}
	target.Character.Name = "Target"
	target.Character.HealthMax.Value = 10_000
	target.Character.Health = 10_000
	target.Character.StaminaMax.Value = 10_000
	target.Character.Stamina = 10_000
	target.Character.Stats.Dexterity.ValueAdj = targetDex
	target.Character.Conditions = conditions.New()
	setCombatPositionParallel(&target.Character, position.Standing)
	mobs.SetInstanceForTest(targetID, target)
	t.Cleanup(func() { mobs.SetInstanceForTest(targetID, nil) })
	return target
}

func newSmoke(targetID int) *characters.Character {
	char := characters.New()
	char.SpeciesId = smokeSpeciesForTest
	char.Stats.Dexterity.ValueAdj = 500
	char.SetAggro(0, targetID, characters.DefaultAttack)
	fundSpecialMove(char)
	return char
}

func TestSmother_UnlistedSpeciesIsRefused(t *testing.T) {
	seedSmotherFixture(t, 7460, 1)
	char := newSmoke(7460)
	char.SpeciesId = 7445
	res := ExecuteSmother(newStubActor(char, newTestRoom()))
	assert.True(t, res.NotSmoke)
	assert.False(t, res.Executed)
}

// Appendix A: a hit blinds and smothers; anything else applies neither.
func TestSmother_HitBlindsAndSmothers_OtherwiseNothing(t *testing.T) {
	sawHit, sawOther := false, false
	for _, dex := range []int{1, 5_000} {
		target := seedSmotherFixture(t, 7461, dex)
		for i := 0; i < 100; i++ {
			target.Character.Conditions = conditions.New()
			char := newSmoke(7461)
			res := ExecuteSmother(newStubActor(char, newTestRoom()))
			require.True(t, res.Executed)
			blind := target.Character.Conditions.TriggersLeft(perception.ConditionIdBlinded) > 0
			choking := target.Character.Conditions.TriggersLeft(conditions.ConditionIdSmothered) > 0
			if res.MoveResult.Hit {
				sawHit = true
				assert.True(t, blind && choking, "a hit must blind and smother")
			} else {
				sawOther = true
				assert.False(t, blind || choking, "a partial or miss must apply neither")
			}
		}
	}
	require.True(t, sawHit, "no hit in 200 tries: the fixture is broken")
	require.True(t, sawOther, "no partial or miss in 200 tries: the fixture is broken")
}

// Appendix A: the cast interrupt rides the shared special-move cooldown, so
// a second smother inside the window is refused before it can choke again.
func TestSmother_SecondSmotherInsideTheCooldownIsRefused(t *testing.T) {
	seedSmotherFixture(t, 7462, 1)
	char := newSmoke(7462)
	actor := newStubActor(char, newTestRoom())
	first := ExecuteSmother(actor)
	require.True(t, first.Executed)

	second := ExecuteSmother(actor)
	assert.False(t, second.Executed)
	assert.True(t, second.OnCooldown, "the second smother must be refused on the shared cooldown")
	assert.False(t, CommandIsReady(actor, "smother"), "readiness agrees")
}
```

(Check `conditions.SeedConditionsForTest`'s field names against `combat_throttle_test.go:101-104`, which uses `TriggerCount` and `RoundInterval`. If `TriggersLeft` lives on a different receiver, use the one `HasAnyBlindSource` uses: `c.Conditions.TriggersLeft(id)`.)

Add to `internal/combat/beast_move_gates_test.go`: `"smother": CanUseSmother` in `beastMoveGates`, `"smother"` in `all`, and:

```go
func TestScoreSmother_FavoursCastersAndSightedTargets(t *testing.T) {
	mob := &mobs.Mob{}
	sighted := characters.New()
	if ScoreSmother(mob, sighted) <= 0 {
		t.Fatal("smother must score against a sighted target")
	}
}
```

(import `internal/mobs`). Run: FAIL (`undefined: ExecuteSmother`, `CanUseSmother`).

- [ ] **Step 2: Registries.** `actionspec/action.go`: `ActionSmother Action = \`smother\`` after `ActionThrottle`, and `ActionSmother: {Skill: skills.UnarmedCombat, SkillSource: SkillFixed, Physical: true},` in the spec map. `costs/action.go`: `ActionSmother = actionspec.ActionSmother`. `costs/action_test.go`: `{ActionSmother, "smother", skills.UnarmedCombat, SkillFixed, true},`.
- [ ] **Step 3: One cast-interrupt contest.** Create `internal/actions/choke_interrupt.go`, moving throttle's contest (`combat_throttle.go:159-197`) unchanged in meaning:

```go
package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// chokeCastInterrupt is the cast interrupt a choking move carries (throttle,
// smother): an opposed contest through the concentration seam, the caster's
// hold (Willpower plus spellcasting) against the attacker's grip (Dexterity
// plus unarmed combat), floored only by ConcentrationFloor. A telegraphed
// NoDamageInterrupt cast is not contestable. The hold takes the sight ramp
// (the caster needs to see to hold the weave); the grip takes none. The
// defender's concentration award fires win or lose. Callers run it on a hit
// only, after ClaimSpecialMove, so it fires at most once per special-move
// cooldown window. Returns true when the spell was cut off.
func chokeCastInterrupt(actor Actor, char, targetChar *characters.Character, sightRoom messaging.RoomVisibility) bool {
	if !targetChar.IsCasting() {
		return false
	}
	if cs, ok := targetChar.Activity.CastingData(); ok {
		if sd := spells.GetSpell(cs.SpellId); sd != nil && sd.NoDamageInterrupt {
			return false
		}
	}
	cfg := configs.GetBalanceConfig()
	w := float64(cfg.SkillWeight)
	grip := float64(char.Stats.Dexterity.ValueAdj) + float64(char.GetSkillLevel(skills.UnarmedCombat))*w
	hold := float64(targetChar.Stats.Willpower.ValueAdj) + float64(targetChar.GetSkillLevel(skills.Spellcasting))*w
	hold *= messaging.SightMult(targetChar, sightRoom)
	res := combat.RunConcentrationContest(hold, grip)
	targetChar.AwardResolved(targetChar.GetUserId(), res.Success, targetChar.CandidateFor(string(skills.Spellcasting)))
	if res.Success {
		return false
	}
	return InterruptTargetCast(targetChar, ActorRefOf(actor))
}
```

In `ExecuteThrottle` replace `:150-197` (the `if target.Char.IsCasting() { ... }` block and its comment) with `interrupted = chokeCastInterrupt(actor, char, target.Char, sightRoom)` under a two-line comment pointing at the helper; lines 146 and 149 do not move. Drop now-unused imports (`messaging`, `spells`, `state`). In the drift test change throttle's postCommitCallees `"InterruptTargetCast"` to `"chokeCastInterrupt"`. Run `go test ./internal/actions/ -run Throttle -count=1`: PASS.

- [ ] **Step 4: The action.** Create `internal/actions/combat_smother.go`:

```go
package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// SmotherResult holds the outcome of a smother attempt for the mob command.
type SmotherResult struct {
	Cost       characters.CostCommitResult
	Target     AggroTarget
	MoveResult combat.SkillMoveResult
	Counter    combat.CounterResult
	Executed   bool
	OnCooldown bool
	Crafting   bool
	NoTarget   bool
	// NotSmoke is true when the species does not list smother in
	// beast_moves, or the body has hands. Reachable only through a
	// combatcommands or btree dispatch; CanUseSmother gates the AI path.
	NotSmoke        bool
	InterruptedCast bool
}

// ExecuteSmother is the smoke elemental's special move (Appendix A of spec
// 2026-10-16 #541), mob-only: smoke pours into the target's eyes and lungs.
// It is built on ExecuteThrottle: the same cooldown, cost and skill-move
// seam; on a hit the target is Blinded (condition 3) and Smothered (144),
// and a casting target faces the shared choke contest (chokeCastInterrupt),
// at most once per special-move cooldown window. A partial or a miss applies
// nothing. No bleed: smoke has no fangs.
func ExecuteSmother(actor Actor) SmotherResult {
	char := actor.GetCharacter()

	if char.IsActing() {
		return SmotherResult{Crafting: true}
	}
	target := resolveActionTarget(actor, char)
	if !target.Found {
		return SmotherResult{NoTarget: true}
	}
	if char.HasBodyPart("hands") || !combat.SpeciesHasBeastMove(char, "smother") {
		return SmotherResult{NotSmoke: true}
	}

	cfg := configs.GetBalanceConfig()
	if !SpecialMoveReady(char) {
		return SmotherResult{OnCooldown: true}
	}
	cost := admitFullCost(actor, costs.ActionSmother, characters.PoolStamina, float64(cfg.SpecialMoveBaseStaminaCost))
	if cost.Status == characters.CostRefused {
		return SmotherResult{Cost: cost}
	}
	if !ClaimSpecialMove(char) {
		return SmotherResult{Cost: cost, OnCooldown: true}
	}
	commitMeleeEngagement(actor)

	sightRoom := combat.SightRoom(actor.GetRoom())
	result := combat.ExecuteSkillMove(combat.SkillMoveParams{
		Attacker: char,
		Defender: target.Char,
		Shape:    combatvocab.Melee(combatvocab.TargetSingle),
		Room:     sightRoom,
		Attack: combat.AttackSide{
			Stat: char.GetEffectiveDexterity(), StatName: "dexterity",
			Skill: skills.UnarmedCombat, SkillRank: char.GetSkillLevel(skills.UnarmedCombat),
			Mult:      combat.SituationalAttackMult(char, sightRoom, combatvocab.Melee(combatvocab.TargetSingle)),
			ForceCrit: combat.SleepingForceCrit(target.Char),
		},
		DamagePercent:   float64(cfg.KickDamagePercent),
		KnockdownFactor: 0, // no knockdown: blindness and choking instead
		DamageStat:      char.Stats.Strength.ValueAdj,
	})
	counter := counterSkillMoveExit(actor, target.Char, result, combatvocab.Melee(combatvocab.TargetSingle), true)

	interrupted := false
	if result.Hit {
		// Both apply synchronously, narrated by the hit line (Blinded's own
		// start lines speak of a spell; Smothered is silent-start).
		_ = target.Char.AddCondition(perception.ConditionIdBlinded, false)
		_ = target.Char.AddCondition(conditions.ConditionIdSmothered, false)
		interrupted = chokeCastInterrupt(actor, char, target.Char, sightRoom)
	}

	sourceType := combat.User
	if !actor.IsPlayer() {
		sourceType = combat.Mob
	}
	targetType := combat.User
	if target.MobInstanceId > 0 {
		targetType = combat.Mob
	}
	combat.RecordSpecialMove(sourceType, targetType, "smother", result.Hit, result.Damage, char, target.Char, util.GetRoundCount())

	// Re-verified 2026-10-19 (16b8362db): a special move consumes its round
	// with SpendRoundOnAction, which the round hook consumes silently, and
	// TestSpecialMoveAdmissionOrdering counts only that call.
	if char.CombatPhase != nil {
		char.SpendRoundOnAction()
	}
	actor.AwardResolved(result.Hit, actor.GetCharacter().CandidateFor(string(skills.UnarmedCombat)))

	return SmotherResult{
		Cost:            cost,
		Target:          target,
		MoveResult:      result,
		Counter:         counter,
		Executed:        true,
		InterruptedCast: interrupted,
	}
}
```

- [ ] **Step 5: Gates and lists.** `ai.go`:

```go
// CanUseSmother reports whether the actor can smother a target: the smoke
// elemental's special move, listed in its species' beast_moves.
func CanUseSmother(char *characters.Character) bool {
	if _, exists := char.Cooldowns["special-move"]; exists {
		return false
	}
	if char.HasBodyPart("hands") {
		return false
	}
	return SpeciesHasBeastMove(char, "smother")
}

// ScoreSmother is the AI priority for smother: highest against a caster (the
// smoke can choke the spell off) and lowest against a target already blind.
func ScoreSmother(mob *mobs.Mob, target *characters.Character) int {
	score := 50
	if target.IsCasting() {
		score += 30
	}
	if target.HasAnyBlindSource() {
		score -= 30
	}
	if score < 0 {
		score = 0
	}
	return score
}
```

and in `SelectSpecialMove` after throttle: `if CanUseSmother(&mob.Character) { moveScores["smother"] = ScoreSmother(mob, target) }`. `command_readiness.go`: `case "smother": return char.IsInCombat() && !char.HasBodyPart("hands") && combat.SpeciesHasBeastMove(char, "smother")`. `action_readiness.go` `specialMoveVerbs`: add `"smother": true`. `behaviortree/actions_mob.go` `beastMoves`: add `"smother": true,`. `divergences.go` `mobOnlyCommands`: `"smother": "mob-ai: the smoke elemental's special move; no player can be a smoke elemental",`.
- [ ] **Step 6: The mob command.** Create `internal/mobcommands/smother.go`, the shape of `mobcommands/throttle.go` with: `actions.ExecuteSmother`, `res.NotSmoke` swallowed silently, `sendMoveEvent("smother", "hit", ids, aud, messaging.CategoryHitNaturalSharp, damageTokens)`, `sendMoveEvent("smother", "cast_interrupt", ids, aud, messaging.CategorySpellDisruption, nil, messaging.SoundChantBreaksOff)` when `res.InterruptedCast`, the partial branch with `renderMoveEvent("smother", "partial", ids, damageTokens)` and `moveDefenceLines(mob, room, target, result.Defence, "rolling smoke")` (D5, approved: "You dodge the Elemental Prince's rolling smoke"), the defended branch with the same label, `sendMoveEvent("smother", "miss", ids, aud, messaging.CategoryHitNaturalSharp, nil)`, then `actions.DispatchCounterMessages`. Doc comment: `// Smother is the smoke elemental's special move (spec 2026-10-16 #541, Appendix A): smoke pours into the target's eyes and lungs, blinding and choking it, and can choke off a spell.` Register `"smother": {Smother, false},` in `mobcommands.go` after `"throttle"`.
- [ ] **Step 7: Guard lists.** Drift test: add to `TestSpecialMoveAdmissionOrdering` `{"combat_smother.go", "ExecuteSmother", "SmotherResult", "costs.ActionSmother", []specialMoveEarlyReturn{{"char.IsActing()", "Crafting"}, {"!target.Found", "NoTarget"}, {"char.HasBodyPart(\"hands\") || !combat.SpeciesHasBeastMove(char, \"smother\")", "NotSmoke"}}, []string{"combat.ExecuteSkillMove", "target.Char.AddCondition", "chokeCastInterrupt", "combat.RecordSpecialMove", "actor.AwardResolved"}, true}`; `mobWrappers` and its name map: `"smother": "Smother"`; `runExecuteAndReadExecuted` and `runExecuteAndReadFlag` cases for smother (Crafting, OnCooldown, NoTarget, NotSmoke); `physicalSpecial["smother"] = true`; a seeded species `7313: {SpeciesId: 7313, Name: "smoke-test", BodyParts: []string{}, BeastMoves: []string{"smother"}}` and rows `smother_ready`, `smother_crafting`, `smother_cooldown`, `smother_no_aggro`, `smother_notlisted` (SpeciesId 7304, reason `NotSmoke`), modelled on the throttle rows; `combat_smother.go` in the file list of `TestSpecialMoves_NoProgressionAwardSitsInsideAHitGate`. `action_cost_test.go`: `{file: "combat_smother.go", name: "ExecuteSmother"},`. `command_parity_test.go`: `"smother"` in `supported`. `move_narration_migration_guard_test.go`: `"internal/mobcommands/smother.go",` in `migratedNarrationLiteralFiles` (fix the "thirteen" count in its comment to fourteen). `shipped_narration_data_guard_test.go`: `"narration/special-moves/smother.yaml": true,` and "fourteen shipped files" to "fifteen". `defensive_messages_newly_defendable_test.go`: `{"rolling smoke", meleeDefences},`. Run `go test . -run TestPlayerConditionsTravelTheEventPath -count=1` (`condition_apply_path_guard_test.go:367`) and add the two reported keys with: `"internal/actions/combat_smother.go|<line>": "condition 144 is silent-start; the smother move narrates the smoke as it lands and must apply it in the same tick"` and `"internal/actions/combat_smother.go|<line>": "Blinded (3) carries spell start lines; the smother hit line narrates the blinding, and it must land in the same tick so the target's next line is judged blind"`, with the line numbers the guard prints.
- [ ] **Step 8: Goldens** (`-run TestSnapshotStores -update`): `special_moves.golden` gains the smother rows, `conditions.golden` gains condition 144. Read both diffs; nothing else may move.
- [ ] **Step 9: Run** the full suite; PASS. Commit (Task 10's files included):

```bash
git add _datafiles/world/dogmud/conditions/144-smothered.yaml _datafiles/world/dogmud/narration/special-moves/smother.yaml \
  internal/conditions/ids.go internal/actionspec/action.go internal/costs/action.go internal/costs/action_test.go \
  internal/actions/choke_interrupt.go internal/actions/combat_throttle.go internal/actions/combat_smother.go \
  internal/actions/combat_smother_test.go internal/actions/command_readiness.go internal/actions/action_readiness.go \
  internal/actions/divergences.go internal/actions/action_cost_test.go internal/actions/command_readiness_drift_test.go \
  internal/mobcommands/smother.go internal/mobcommands/mobcommands.go internal/mobcommands/command_parity_test.go \
  internal/behaviortree/actions_mob.go internal/combat/ai.go internal/combat/beast_move_gates_test.go \
  internal/items/defensive_messages_newly_defendable_test.go condition_apply_path_guard_test.go \
  move_narration_migration_guard_test.go shipped_narration_data_guard_test.go \
  internal/narration/testdata/stores/special_moves.golden internal/narration/testdata/stores/conditions.golden
git commit -F - <<'EOF'
feat(combat): smother, the smoke elemental's special move (#541)

Mob-only, built on throttle: a hit blinds (3) and smothers (144, new);
a partial applies nothing; a casting target faces the choke contest,
now one helper shared with throttle, at most once per special-move
cooldown (a test refuses a second smother inside it). The AI gate reads
the species' beast_moves. Copy from Appendix A without actor lines
(owner ruling D1: a mob has no client); defence label "rolling smoke"
(D5).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

## Task 12: Docs and code comments (spec section 7, ruling H)

**Files:** `internal/species/context.md`, `internal/combat/context.md`, `internal/actions/context.md`, `internal/items/context.md`, `internal/mobcommands/context.md`, `internal/conditions/context.md`, `internal/exit/context.md`, `docs/PATCH_NOTES.md`.

- [ ] **Step 1: `internal/species/context.md`.** In "Natural-Attack Subtype": valid values gain `maul`, `pounce`; a short "Bears and cats (#541 slice 2)" paragraph (bear 3 `maul` "claws and teeth", feline 11 `pounce` "claws and fangs", both natural-sharp). New section "Beast moves (`beast_moves`)": the principle (ruling E, already stated above it: link it), `natural_attack` is the default and `beast_moves` the special moves, the reviewed table (copy the Proposed column), the validator's body rules (no hands for every move but drain; legs for pounce and hamstring; horns for gore; lifedrain for drain; unknown and repeated names fail the load), that charge stays scripted, and "how to add a species": pick the default by body, then list only the moves the creature really makes, then run `go test ./internal/species/`. Update "Choosing a natural attack" step 3 to say the six wiring sites list now has eleven worked examples. Files table: `species_beast_moves_test.go`, `species_maul_pounce_test.go`.
- [ ] **Step 2: `internal/combat/context.md`.** "Natural-Attack Subtype Resolution": `AttackSubtype` is the one weapon-type rule and who reads it; engage lines from `prepare` (`EngageMessages`, `EngageLineFor`, the dark fallback D2); `separate` retired and why; `items.MeleeSwingNarratedAs`. "Beast Moveset" table: Gate column becomes "listed in beast_moves" for every move, a `smother` row (mob-only; Blinded 3 plus Smothered 144; cast interrupt via the shared choke contest), and the line Task 7 Step 6 wrote (the attack-family predicates deleted under D6, `SpeciesHasBeastMove` the one species read). The AI profile table is unchanged: `ambush_predator` carries no throttle weight (D4, #574), so say that a cat's throttle is allowed by the gate but rarely chosen.
- [ ] **Step 3: The others.** `internal/actions/context.md`: readiness and every `Execute*` entry read `combat.SpeciesHasBeastMove` (Task 7 Step 6 replaced the predicate list; check it reads whole); `chokeCastInterrupt`; `ExecuteSmother`; Combat specials file row gains `combat_smother.go`, `choke_interrupt.go`. `internal/items/context.md` (`:35`, `:500-531`): drop `SeparateMessages`, say `AttackOptions` decodes `together` only and rejects `separate`, add `MeleeSwingNarratedAs`. `internal/mobcommands/context.md`: `smother.go`. `internal/conditions/context.md`: `ConditionIdSmothered`. `internal/exit/context.md`: `PhraseVerticalExits` no longer has an items caller. Run `python tools/context_md_audit.py internal/species`, then each of `internal/combat`, `internal/actions`, `internal/items`, `internal/mobcommands`, `internal/conditions`, `internal/exit`, one per call; fix any phantom symbol.
- [ ] **Step 4: Code comments** (ruling H): confirm the `BeastMoves` field comment (Task 6) and the block comment above the `CanUse*` gates (Task 7) are in place; `grep -n "BeastMoves\|beast_moves" internal/species/species.go internal/combat/ai.go`.
- [ ] **Step 5: `docs/PATCH_NOTES.md`** (D8: approved as drafted, one line pending, see the note under the block). New top entry, dated the day the PR opens:

```markdown
## 2026-10-17: Every creature squares up its own way

- When a fight begins, the first line now shows how your foe gets ready,
  with its own weapon, teeth or claws, and everyone watching reads the same
  moment.
- A creature waiting for an opening now waits in its own way.
- Bears now maul and big cats pounce in their ordinary attacks.
- Each kind of beast now has its own special moves. Wolves can go for the
  throat, rats nip at ankles, and snakes simply bite.
- The smoke elemental can pour itself into your eyes and lungs, blinding and
  choking you for a short time, and its smoke can choke off a spell.
```

Owner approved this text on 2026-10-16. The throat bullet names wolves only, because D4 leaves the cats' AI profile without a throttle weight (#574), so a cat rarely throttles (Fact 28).

Check every line is within 80 columns, has no number and no dash.
- [ ] **Step 6: Commit** `docs(combat): reachability slice 2 in the package docs and patch notes (#541)`.

## Task 13: Ship gate (dogmud-shipping)

- [ ] **Step 1:** `gofmt -l internal/ modules/` prints nothing.
- [ ] **Step 2:** `go build ./...` succeeds.
- [ ] **Step 3:** `golangci-lint run --new-from-merge-base=origin/master` reports `0 issues`.
- [ ] **Step 4:** `go test . ./internal/... ./modules/... -count=1` passes; paste the tail.
- [ ] **Step 5:** `grep -n "LogToFile" _datafiles/config.yaml` shows `LogToFile: false`.
- [ ] **Step 6: Boot check** in an isolated worktree, ports private, override in the scratchpad:

```bash
S="<session scratchpad, full path>"
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
printf 'Network:\n  TelnetPort: [43333]\n  LocalPort: 19999\n  HttpPort: 18090\n  AIPort: 45555\n' > "$S/boot-override.yaml"
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
CONFIG_PATH="$S/boot-override.yaml" timeout 180 ./boot-check.exe > "$S/boot.log" 2>&1   # exit 124 = stayed up
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" "$S/boot.log"   # want 0 (run alone)
grep -c "Server Ready" "$S/boot.log"                                          # want 1
grep -n "species.LoadDataFiles\|attackMessageCount" "$S/boot.log"             # 22 attack files, species loaded
```

Then `git worktree remove --force C:/tmp/dogmud-boot-check` (PowerShell `Remove-Item -Recurse -Force` if Windows holds the exe).

## Task 14: Playtest (dogmud-playtesting)

**Files:** new `tools/playtest/scenarios/beast-moves.yaml`, `tools/playtest/goals/scenarios/beast-moves/defender.yaml`, `tools/playtest/goals/scenarios/beast-moves/witness.yaml`, `tools/playtest/goals/2026-10-17-beast-moves-admin.yaml`.

- [ ] **Step 1: Fixture check.** The Ironwind Steppe holds every creature the spec names (Fact 32 and the mob files): Feral Cat 202 (rooms 3001, 3007, 3009; hostile, `ambush_predator`, `generic_fighter`), Steppe Wolf 205 (3013, 3015, 3016; `predator`), Steppe Rat 200 (3000 to 3002; `skirmisher`). Before writing the goals, read rooms 3001 and 3015 for exits and light, and confirm the steppe's day lighting; carry a torch (item 20096, as the slam split lash run did) if any room reads dark.
- [ ] **Step 2: Two-seat scenario** `tools/playtest/scenarios/beast-moves.yaml` (party mode, `defender` and `witness`, both `bug-finder`, 40 minutes), modelled on `slam-split-lash.yaml`: the defender fights the Feral Cat in 3001 for at least six rounds, then a Steppe Wolf in 3015 for six, then a Steppe Rat for four; the witness walks into each room once the fight is on and does nothing. Group goals: (a) the engage line, each wait line and each swing read as one moment from both seats, and the witness's line names the defender; (b) the cat's lines are pounce lines (leaps, claws and fangs), the wolf's bite lines, no line names a weapon the creature lacks; (c) every special move is quoted with its round; the rat never throttles, mauls or pounces (any one is a finding); a cat's throttle is NOT required (D4: its gate allows throttle, but with no `ambush_predator` weight its AI rarely picks it, #574; the gate is proven by `TestBeastMoveGates_TableExamples`, Task 7), so report a cat throttle if one happens and do not report its absence; a cat's rake, pounce or hamstring and a wolf's maul, throttle, pounce or hamstring are expected moves; (d) no curly-brace token, doubled or missing name, lowercase start, empty line, or a line wrapping mid-word. Defender goals: `ephemeral.profile: mid` (survives the wolf), `start_room: 3001`, `grant_items: [20096]`, wear the torch, `remove` the weapon so fights last, drink a salve below a third of health.
- [ ] **Step 3: Admin single run** `tools/playtest/goals/2026-10-17-beast-moves-admin.yaml` (`profile: admin`, `start_room: 4160`, 35 minutes), modelled on `2026-10-16-slam-split-admin.yaml`: `mob spawn 202` and fight eight rounds, quoting every line and every special move (a cat throttle is not required, D4); `mob spawn 9541` (The Pass-Apex, a bear: its swings are maul lines, every line fits 80 columns, it rakes and charges, it never pounces or hamstrings); `mob spawn 322` (the Elemental Prince, the only smoke elemental): first `look` to confirm it spawned outside its instance and that `attack prince` is accepted (a refusal is the finding to report, then skip the rest of this goal), then fight up to twelve rounds with `mob heal` as needed, quoting every smother line, whether the defender reads that they are blinded and choking, the Smothered per-round line, and whether a second smother ever lands within a few rounds of the first. A cast is the way to see the interrupt: the admin profile casts any spell it knows mid-fight.
- [ ] **Step 4: Run** `/playtest-scenario --checkout <abs path of the branch checkout> tools/playtest/scenarios/beast-moves.yaml` and `/playtest local --checkout <abs> feature-tester 2026-10-17-beast-moves-admin.yaml`. Confirm the containers are gone afterwards (`playtestrun stop` leaves them up; `docker rm -f` by name of the run's own container).
- [ ] **Step 5: Findings to issues** (reports are gitignored): search `gh issue list --repo pruuk/DOGMud --search "<words>"` first; file new ones with `gh issue create --repo pruuk/DOGMud --label bug --label from-playtest --label area:combat`. A maul or pounce line wrapping past 80 columns is a finding against Task 8 (D7 rewrote both files). A cat that never throttles is not a finding (D4, #574). Record harness lessons in memory only.
- [ ] **Step 6: Commit** the four goal and scenario files, `test(playtest): beast moves two-seat and admin runs (#541)`, the body naming the fixtures and what the run found.

## Task 15: PR

- [ ] **Step 1:** `git push -u origin feature/combat-reachability-slice2`.
- [ ] **Step 2:** Open the PR; the body goes through a file written with Write to the scratchpad:

```bash
gh pr create --repo pruuk/DOGMud --base master --head feature/combat-reachability-slice2 \
  --title "Combat reachability slice 2: one weapon rule, engage lines, beast moves, smother" \
  --body-file "$S/pr-body.md"
```

`pr-body.md` says "Part of #541." and "Part of #539." (never a closing keyword: issue #541 closes by hand once the owner agrees nothing remains, and #222 shows prose about a future plan auto-closes); lists the owner rulings D1 to D8 as recorded in this plan (including D4's follow-up #574 under #374, and the D6 deletion superseding spec section 4's predicate bullet); the behaviour changes a player sees (engage lines, wait lines in the creature's own way, bare-handed humanoids now waiting from `unarmed.yaml`, bears maul, cats pounce, maul and pounce lines within 80 columns, the per-species move table with what the bird, serpent and rodent lost, smother); what was retired (17 separate blocks, shooting.yaml's swing lines, the default-attack throttle.yaml) with golden row counts; the new files by full path; the gate evidence (Task 13 tails) and the playtest result with issue numbers; and ends with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
- [ ] **Step 3:** Read the URL `gh` prints; it must say `pruuk/DOGMud`. `gh pr checks <n> --repo pruuk/DOGMud --watch`, then confirm with `gh run list --repo pruuk/DOGMud` that every expected workflow ran. The owner merges and deploys; do not deploy.

---

## Risks

| Risk | Answer |
|---|---|
| Switching bears and cats changes which moves they qualify for (spec) | Tasks 6 and 7 move the moves to data before Task 9 switches the attack; `TestBeastMovesMatchTheReviewedTable` pins the result |
| An engage line repeats a weapon name a reader cannot see (spec) | D2 (a): below full sight the plain line is sent with names hidden, as the wait round does; `TestEngageLineFor_DarkReaderGetsThePlainLine` and the existing dark and blinded attack tests hold it |
| Retiring `separate` breaks a path not yet found (spec) | Fact 20 lists every use; the decoder rejects the key at load; the compiler enumerates the rest once the type is gone; the full suite and the boot check run |
| The default world's combat messages carry `separate` | They already fail to load (role-first layout, Fact 7); the default world is out of scope and untouched |
| Some 83 test fixtures decide beast moves by family | Task 7 Step 5 gives each fixture its old family's moves, keeping every test's meaning; the drift test gains `_notlisted` rows proving the list is read |
| Bare-handed humanoids' wait and engage lines move from generic.yaml to unarmed.yaml, and the entangled-defence lines change colour for natural attackers | A direct consequence of ruling B and the one rule; named in the PR body and checked in the playtest |
| Refactoring throttle's cast interrupt into a helper changes a guarded file | The helper is a move of the same code; throttle's tests and the drift test's callee row catch a drift; allowlist line keys for `combat_throttle.go:146,149` do not move |
| The Prince spawned outside its instance may behave oddly or refuse an attack | The admin goal checks the spawn and the attack first and reports a refusal as the finding; the unit tests cover smother regardless |
| Cats rarely throttle: the gate allows it but `ambush_predator` has no throttle weight (Fact 28) | D4: the profile is not changed in this slice; the tuning is #574 under the behaviour arc #374. Tests prove the gate, the playtest does not require a cat throttle, and the patch note stops promising it (D8, confirmed) |
| maul.yaml and pounce.yaml exceed 80 columns at 20-character names (149 and 121 of 345 role lines, Fact 30) | D7 AMENDED 2026-10-19: accepted. The owner ships both files as written; combat hit categories wrap at the player's line width (`shouldWrap`), so an over-80 line wraps rather than breaks. (Was D7 (b): rewrite through an owner review and extend the width guard.) |
| Deleting the predicates removes something #563 might have reused | D6 (b): #563 wants a proper anatomy model, not attack-family predicates; Task 7 Step 6 greps (with a pre-deletion proof the pattern matches) that no caller remains, and the root guard keeps the names from returning to a gate |
| The D7 rewrite drifts from a record's one moment or reintroduces a #455 defect (moot after the 2026-10-19 D7 amendment: no rewrite) | Task 8 Step 3 checks each draft by the same rules the guards apply (width, subject-verb, banner, idiom, dashes) before the owner sees it; Task 9 Step 3 runs the real natural-weapon guard once cats use pounce |

## Self-review

- Spec section 1 (one rule, its callers, the test): Task 1. Fumble and coup de grace reach it through `buildWeaponSetup` (Fact 6); the fourth reader (entangled lines) is included.
- Section 2 (engage from prepare, by `AttackSubtype` not `meleeDisplaySubtype`, sight hiding, guards untouched): Task 4.
- Section 3 (separate retired with type, render and both branches; shooting exemption read from one function; throttle.yaml deleted with guard lists and goldens; counts re-recorded): Tasks 2, 3, 5.
- Section 4 (maul, pounce subtypes and descriptions, `validNaturalAttacks`, natural-sharp band, plus reach and pools; bear and feline switched): Task 9, after Task 8 brings both files within 80 columns. Its bullet that the predicates "describe the body" is superseded by D6 (b): Task 7 Step 6 deletes them.
- Section 5 (the field, the validator with unknown names and body rules, the gates at all three sync points, the table including row 44, bears' charge, scripted moves unchanged): Tasks 6 and 7; corrections in Facts 14 to 16.
- Section 6 (unit tests for the three cases, wolf engage and wait from bite, bear from maul, cat from pounce, bow from shooting prepare; the table test; the gate-reads-the-list guard; goldens only for deliberate changes; boot and playtest): Tasks 1, 4, 6, 7, 8, 9, 13, 14.
- Section 7 and ruling H (three named context.md files plus four more the change touches, the two code comments, patch notes): Task 12, with the predicate mentions in four context.md files handled in Task 7 Step 6.
- Appendix A (narration, Smothered at 144 with 89's values, Blinded on hit, partial nothing, cast interrupt on the shared cooldown with the second-smother test, the AI gate reading beast_moves): Tasks 10 and 11.
- Owner rulings, each to its task:
  - D1 (a) smother.yaml without actor roles, cast_interrupt keeps actee and observer: Task 10 Step 2; Task 11 commit body.
  - D2 (a) authored engage line at full sight only: Task 4 Steps 1 and 3 (`EngageLineFor`, `TestEngageLineFor_DarkReaderGetsThePlainLine`).
  - D3 pounce 240-242 record approved: Task 8 Step 2 (lands with the D7 rewrite, shown as approved); Task 9 Step 3 runs the natural-weapon guard over it.
  - D4 no AI profile change (#574 under #374): Task 7 header and Step 8 body (no `aiProfiles` edit), `TestBeastMoveGates_TableExamples` proves the gate, Task 12 Step 2 (context.md), Task 14 Steps 2, 3 and 5 (no cat throttle required, a rat never throttles), Risks.
  - D5 "rolling smoke": Task 11 Steps 6 and 7 (`moveDefenceLines` label, `defensive_messages_newly_defendable_test.go`).
  - D6 (b) predicates and `SpeciesNaturalAttack` deleted: Task 7 Steps 2 and 6 (grep with a pre-deletion proof, test deletion, four context.md files); Task 9 no longer touches them.
  - D7 (b) maul and pounce rewritten to 80 columns: Task 8 (measure, draft, check, owner gate, apply, golden), Task 9 Step 1 (width guard over both files), Fact 30 (re-measured counts).
  - D8 patch note approved with the wolves-only throat bullet: Task 12 Step 5.
- Every guard added demonstrates it can fail: `TestUnreachableCombatLine_CanFail`, `TestValidateBeastMoves_CanFail`, `TestFamilyReadsIn_CanFail`, `TestScriptedBeastMoveFindings_CanFail`, the both-halves `TestBeastMoveGatesReadTheSpeciesList`, the `_notlisted` drift rows, `TestSlamSplitWidthCheck_CanFail` plus the pre-rewrite measure (Task 8 Step 1) for the width guard extended to maul and pounce, the Task 7 Step 6 pre-deletion grep, and the subject-verb probe (Task 8 Step 3, and Task 9 Step 3 if `forward` is allowlisted).
- New player text: the Task 8 maul and pounce rewrites (owner review table, D7) including D3's three approved pounce lines, D5 (the defence label, approved) and D8 (patch notes, approved with one bullet pending). Engage lines come from existing prepare pools; smother's copy is Appendix A (approved) less its actor lines.
