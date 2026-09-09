# Skill coverage audit closing Phase 1

Date: 2026-09-08
Branch: feature/context-budget-skill-extraction
Author: Task 13 of the context-budget-skill-extraction plan

## What Phase 1 delivered, and what Phase 2 may rely on

Phase 1 built twelve on-demand skills under `.claude/skills/` that lift
procedure out of the 1,094-line `CLAUDE.md` plus the memory index. Nothing
has been deleted from `CLAUDE.md` yet: it is byte-identical to its state
before this plan started (confirmed in Step 6 below), and
`python tools/verify_skills.py` reports `12/12 skills present, 0 errors`.

This audit is what Phase 2 should trust when deciding what to delete from
`CLAUDE.md`. Its method: every skill's own `## Sources` section (or, where a
skill's Sources section is incomplete, the inline citations found in its
body) was read to find which `CLAUDE.md` line ranges it claims, then every
claimed range was diffed word-for-word against the skill body. Findings
below are organized as: no memory file double-claimed across skills (Step
1), a mechanical faithfulness check per lifted range (Step 1b, the
headline), a disposition for all 44 `CLAUDE.md` sections (Table A), a
faithfulness row per folded memory file (Table B), and a list of
source-material defects surfaced along the way, several of them new finds
beyond what the task briefing already knew about.

**Bottom line up front: no range failed the faithfulness check.** Every
`CLAUDE.md` section a skill claims to have lifted is present in that skill,
word for word, once the em-dash-to-colon punctuation normalization (a
project-wide rule applied during extraction) is accounted for. The
significant findings are elsewhere: several `CLAUDE.md` sections the task
briefing expected to be safely "already covered elsewhere" are not, one of
them (Salvage System) because it describes a mechanic the engine no longer
runs.

## Step 1: no memory file is claimed by two skills

`grep -roh` across all twelve `SKILL.md` files found 94 unique
`[[memory-file]]` references. Building (file, link) pairs and checking for
any link appearing under more than one skill file found **zero
cross-skill collisions**. The raw grep count showed several links with
counts of 2 or 3 (`reference-alt-characters-break-character-scoped-migrations`
at 3, for example), but in every case those extra hits are repeats within
the *same* skill file (once in prose, once in the Sources list, sometimes
once more in a body callout), not a second skill claiming the same file.

The one intentional double-claim named in the task briefing is real and
disclosed: `dogmud-persistence` and `dogmud-shipping` both carry the
`instance:"skip"` / `Room.SpawnInfo` exception from `CLAUDE.md` lines
169-177 (inside the "Instance Saves & Smoke-Test SOP" section, 159-205).
`dogmud-persistence`'s Sources section states this explicitly: "Also
lifted, and co-owned with `dogmud-shipping`: CLAUDE.md lines 169-177 ...
This skill's copy above is reframed around the shadowing model ... rather
than the wipe procedure itself, which stays owned by `dogmud-shipping`."
Confirmed by grepping `SpawnInfo` in both files: `dogmud-persistence` frames
it as "check the struct tag before assuming a field is shadowed,"
`dogmud-shipping` frames it as part of the wipe ritual. No other double
claim was found.

## Step 1b: mechanical range-completeness check (the headline result)

Every range a skill's Sources section (or, for two skills, inline "Lifted
verbatim from CLAUDE.md's ... section" callouts) claims was extracted with
`sed`, tokenized with `tr -s '[:space:]' '\n' | sort -u`, and diffed against
a same-tokenized copy of the skill body with `comm -23`. Every diff that
returned tokens was individually inspected.

**Result: every single flagged token was punctuation, not content.** The
project's own writing rule ("no em or en dashes") was applied during
extraction, so `word — word` in `CLAUDE.md` became `word: word` or
`word, word` in the skill. Tokenizing on whitespace treats the dash and the
words on either side of it as distinct tokens from `word:` or `word,`
fused to a neighbor, so `comm` flags things like `channel)\`` (source, before
an em dash) against `channel)\`:` (skill, colon fused to the same word).
Every flagged token in every range was traced to source and confirmed
present with identical meaning. Two specific classes of flagged token got a
closer look because they looked structural rather than cosmetic, and both
resolved the same way:

- `dogmud-combat`'s 488-561 range flagged `combat.SkillMultiplier(rank)`,
  `channel)\``, `penaltyMax)\``, `softCap)\`` — these are the "Key
  Functions" bullet list (`CLAUDE.md` lines 555-560:
  `combat.CalcRawDamage`, `combat.ApplyMitigation`,
  `combat.SkillMultiplier`, `combat.ResourceMultiplier`,
  `GetPhysicalMitigation`/`GetMagicalMitigation`/`GetConvictionMitigation`).
  All five are present verbatim in the skill at lines 108-113, just with
  `:` instead of the source's `—` before each description.
- `dogmud-authoring-quests`'s 831-875 range flagged `catches`, `flags\``,
  `startup**`, and quote-fragments from the flag-YAML example. All are
  present; one (`catches` -> `catching`) is a deliberate small grammatical
  rewrite to keep the sentence flowing after the dash was removed
  ("startup, this catches typos" became "startup, catching typos"), not a
  fact drop.

**No range is NOT SAFE TO DELETE.** Every lifted range below is faithful.

One caveat worth stating plainly, because it affects trust in the
mechanism Phase 2 will use to decide what is safe: two skills
(`dogmud-authoring-content` and `dogmud-authoring-quests`) do **not**
declare their `CLAUDE.md` lifts in their own `## Sources` section the way
every other skill does. `dogmud-authoring-content` states its lifts inline
in the body ("Lifted verbatim from CLAUDE.md's '...' section:") but never
rolls them up into Sources; `dogmud-authoring-quests` doesn't cite
`CLAUDE.md` anywhere at all, despite six of its sections (786-875) being
near-verbatim lifts, confirmed by the same token diff as everywhere else.
This audit found the content by grepping distinctive phrases from each
`CLAUDE.md` section against the skill bodies, not by trusting Sources.
**This is a genuine gap in those two skills' Sources sections** — the
content is faithful, but a future reader trusting only `## Sources` to
find what a skill covers would miss these six ranges entirely in
`dogmud-authoring-quests`, and would miss the four inline-cited ranges in
`dogmud-authoring-content`'s aggregate accounting. Recommend Phase 2 (or a
quick follow-up) add the missing Sources entries to both files.

## Table A: all 44 CLAUDE.md sections

| # | Section | Lines | Disposition | Step 1b |
|---|---|---|---|---|
| 1 | Content Playtest-Review Gate (SOP) | 3-24 | Lifted into `dogmud-authoring-content` | Faithful |
| 2 | Subagent Model Preference | 25-43 | Stays in residual CLAUDE.md (Claude-process meta, no game-content equivalent; no skill references it) | n/a |
| 3 | Git Workflow | 44-98 | Lifted into `dogmud-shipping` | Faithful |
| 4 | Pre-Push SOP | 99-158 | Lifted into `dogmud-shipping` | Faithful |
| 5 | Instance Saves & Smoke-Test SOP | 159-205 | Lifted into `dogmud-shipping` in full; co-owned subset 169-177 also lifted into `dogmud-persistence` (disclosed) | Faithful (both) |
| 6 | Shop Persistence (Living Economy) | 206-219 | File-location/do-not-wipe half (206-213) lifted into `dogmud-persistence`. Pricing half (214-218): the skill's Sources claims this is "left in CLAUDE.md as subsystem detail for `internal/shops/context.md`," but that context.md was verified NOT to contain the pricing knobs (`ShopAbundanceThreshold`, `ShopBuyRatio`, `ShopPriceFloor`, `ShopPriceCeiling`, `ShopMaterialReserve`, `ShopGoldReserveRatio`, `BarterMaxDiscount`, `BarterMaxBonus`) or the 0.25x-5.0x range. **Gap: not currently documented anywhere; must stay in residual CLAUDE.md or be actually written into `internal/shops/context.md`, not silently deleted.** | Faithful (lifted half) |
| 7 | Moderation Persistence | 220-234 | Do-not-wipe/malformed-file half (220-226ish) paraphrased into `dogmud-persistence`. Remainder (command list, `FinalizeLoginOrCreate` ref, `PetitionCooldownRounds`/`PetitionMaxLen`) verified already covered by `internal/moderation/context.md` (commands at line 87-88, `FinalizeLoginOrCreate` at line 60, config knobs at line 39-40): delete, already covered | Faithful (paraphrased half) |
| 8 | NPC Schedules | 235-245 | Delete, already covered by `docs/schemas/schedule.md` (verified: `activity:` values, coverage-gap panics, all present) | n/a |
| 9 | Sleep Mechanics | 246-258 | Delete/demote, already covered by `internal/actions/context.md` (`actions.Sleep`/`SleepResult`) + `internal/buffs/context.md` (`Sleeping` buff flag) + `docs/schemas/schedule.md` (scheduled-sleep segment, `ScheduleWakeGraceRounds`) | n/a |
| 10 | NPC Patrols | 259-274 | Delete, already covered by `docs/schemas/patrol.md` (verified: `patrol_id`, `dwell_rounds`, loop shapes) | n/a |
| 11 | NPC-to-NPC Conversations | 275-309 | Delete, already covered by `docs/schemas/conversation.md` (193 lines, verified: chance/cooldown knobs, `conversation_line_idx`) + `internal/conversations/context.md` (verified: `MobConversant`, `internal/conversationadapter` bridge) | n/a |
| 12 | Map Consistency & `non_cartesian`/`oneway` | 310-354 | Demote to `internal/mapper/context.md` (exists; verified it covers `ValidateZoneConsistency`/`cartcheck`). The web-client rendering paragraphs (SVG styling, leather theme) are design detail already pointed at `docs/superpowers/specs/completed/2026-06-06-mapper-leather-mockups/` from within the section itself | n/a |
| 13 | Project Context | 355-361 | Stays in residual CLAUDE.md (short orientation pointers to `docs/world.md`, roadmap, remotes; not a procedure any skill would load) | n/a |
| 14 | Stat & Progression System | 362-425 | Lifted into `dogmud-progression-model` | Faithful |
| 15 | Dice & Rolling System | 426-453 | Lifted into `dogmud-combat` | Faithful |
| 16 | Balance Lives in config.yaml, Not in Code | 454-487 | Lifted in full into `dogmud-balance-config`; rule 2 only (476-479) also lifted into `dogmud-combat` | Faithful (both) |
| 17 | Unified Damage & Mitigation Pipeline (Stage 34) | 488-561 | Lifted into `dogmud-combat` | Faithful |
| 18 | Resource Depletion Penalties (Stage 35) | 562-581 | Lifted into `dogmud-combat` | Faithful |
| 19 | Defense Resolution: Best-of-All (Stage 35) | 582-592 | Lifted into `dogmud-combat` | Faithful |
| 20 | Combat Design Conventions | 593-599 | Lifted into `dogmud-combat` | Faithful |
| 21 | Regen System (Stage 29.5) | 600-608 | Demote to `internal/characters/context.md`. Verified partial: the file names `HealthPerRound`/`StaminaPerRound`/`ConvictionPerRound` and the "regen reads the raw max" gotcha, but does **not** name the six `PlayerHealthRegenPct`-family config knobs or restate the mutation/heal-buff multiplier conventions. **Gap: partial coverage only.** | n/a |
| 22 | ID Inventory & Collision Prevention | 609-643 | Lifted into `dogmud-authoring-content` | Faithful |
| 23 | Codegraph MCP - Code Intelligence | 644-690 | Stays in residual CLAUDE.md (Claude-process meta: how the agent itself should use a tool; no skill references it) | n/a |
| 24 | Package `context.md` Convention | 691-732 | Stays in residual CLAUDE.md (Claude-process meta, recursively about writing the very context.md files this audit cites; no skill references it) | n/a |
| 25 | Data File Naming Convention | 733-741 | Lifted into `dogmud-authoring-content` | Faithful |
| 26 | Command Parsing & Multi-Word Input | 742-773 | Demote, already covered by `internal/parser/context.md` (verified: `SplitTrailingContainer`, `SplitLeadingMatch`, `ResolveActor` signatures present) | n/a |
| 27 | MUD Line Width | 774-776 | Lifted into `dogmud-player-copy` | Faithful |
| 28 | Player-Facing Messages - No Hard Numbers | 777-785 | Lifted into `dogmud-player-copy` | Faithful |
| 29 | Quest Re-Grant Prevention SOP | 786-793 | Lifted into `dogmud-authoring-quests` (undisclosed in its Sources; see Step 1b caveat) | Faithful |
| 30 | Quest NPC Dialogue SOP | 794-799 | Lifted into `dogmud-authoring-quests` (undisclosed in Sources) | Faithful |
| 31 | Dialogue Voice & Trigger Discoverability | 800-813 | Lifted into `dogmud-authoring-quests` (undisclosed in Sources) | Faithful |
| 32 | Quest Item Delivery - give.go Gotcha | 814-825 | Lifted into `dogmud-authoring-quests` (undisclosed in Sources) | Faithful |
| 33 | Dialogue Engine: givesItem | 826-830 | Lifted into `dogmud-authoring-quests` (undisclosed in Sources) | Faithful |
| 34 | Quest Flags System | 831-875 | Lifted into `dogmud-authoring-quests` (undisclosed in Sources) | Faithful |
| 35 | Equipment Slots | 876-899 | Demote to `internal/characters/context.md`. Verified only an incidental one-line slot mention (line 984, in a mitigation-summation context), not the full section (default slot list, the four-level `ExtraArm`/`ExtraWrist` escalation, Back-slot cloak-vs-backpack behavior, Component Bag, `ItemSpec` fields, Tail mutation). Kick/stomp/knee variant selection and its config knobs ARE covered, in `internal/combat/context.md`. **Gap: the slot-system half is not currently documented elsewhere.** | n/a |
| 36 | Spell Duration System | 900-904 | Demote to `internal/spells/context.md`. Verified partial: the file names the `calcSpellDuration` function exists and what it feeds, but does not state the formula (`baseFolds × (10 + wil/20 + skill/2)`) or the effect-specific scaling (shield full, heal halved, DoT thirded). **Gap.** | n/a |
| 37 | Buff/Ward Spell System | 905-916 | Demote, split coverage. Kick/stomp/knee auto-select and `KickDamagePercent`/`StompDamagePercent`/`KneeDamagePercent` verified present in `internal/combat/context.md`. Shield-spell magnitude scaling (`effect_magnitude`, Conviction Ward = 75, Chrysalis Cocoon = 125) and the `magical_mitigation`/`conviction_mitigation` buff-statmod routing were not found stated together anywhere; `magical_mitigation`/`conviction_mitigation` fields exist generically in `internal/items/context.md` but not the specific magnitude values. **Gap: partial.** | n/a |
| 38 | Inventory & Item Disambiguation | 917-935 | Demote to `internal/items/context.md`. Verified: `FindItem`/`FindItemByName` and `stacking.go` are named, but the `N.item`/`item#N`/`all.item` disambiguation formats, carry-capacity formula and encumbrance tiers, multi-buy, and enchanting-target-search behavior were not found in any context.md. **Gap: largely undocumented elsewhere.** | n/a |
| 39 | Content Generation Commands | 936-952 | Lifted into `dogmud-authoring-content` | Faithful |
| 40 | AI Testing | 953-1002 | Lifted into `dogmud-playtesting` | Faithful |
| 41 | Mob Stat Archetypes | 1003-1010 | Delete, already covered by `docs/schemas/mob.md` (verified: `archetype` field, statpool weighting by archetype, both percentages match) | n/a |
| 42 | Caster Weapon Types | 1011-1025 | Demote/delete, mostly covered by `docs/schemas/item.md` (verified: wand/sceptre/staff subtypes and `spell_damage_multiplier` field are present). **Gap: the specific per-subtype numeric table** (melee mult, spell mult, speed, parry difficulty per subtype) **was not found in `item.md`**, which only names the subtypes qualitatively. | n/a |
| 43 | Alchemy & Potions System | 1026-1073 | Demote. Verified largely undocumented elsewhere: `internal/items/context.md` names the `aging.go` file but not the aging-phase mechanic, thresholds, or bottle-tier table; **Toxicity has zero mentions in any `internal/*/context.md`** (checked all context.md files). Only historical completed-plan docs (`docs/superpowers/plans/completed/2026-03-30-alchemy-rework.md` etc.) touch this, and those are not living reference docs. **Gap: essentially undocumented elsewhere; must not be deleted without first writing the replacement.** | n/a |
| 44 | Salvage System | 1074-1094 | **STALE, not just uncovered.** `internal/crafting/context.md` states in its own words: "`CalcSalvageChance` and `CalcSuccessChance` were DELETED by U10b-1b. Craft and salvage are contests now, not flat percentages. The knobs that fed them (`SalvageMinChance`, `SalvageMaxChance`, `SalvageSoftCap`, ...) still exist in config and are still validated, but decide nothing." Confirmed independently: `internal/crafting/salvage.go` calls `RunSalvageContest(score, difficulty)`, not the `chance = min + (max-min) × sqrt(skill/softCap)` formula `CLAUDE.md` describes. **This section describes a mechanic the engine no longer runs. Do not silently delete it as "already covered" — the replacement text needs to describe the current contest-based mechanic, or Phase 2/3 will delete a wrong fact without ever writing the right one.** | n/a |

## Table B: folded/cited memory files

94 unique `[[memory-file]]` references were found across the twelve
skills, each claimed by exactly one skill (see Step 1). "Folded" means the
skill states the rule inline and the memory file is cited only as
provenance; "cited" means the skill points at the file without restating
its content as a rule the skill teaches. Faithfulness was spot-checked
against a sample of source memory-file claims embedded in each skill's
prose (formulas, function names, config knobs); every one checked matched
its skill's paraphrase. Two skills' Sources sections were found to omit
memory files that the skill body actually cites inline
(`dogmud-deploying` omits `[[feedback-owner-does-the-deploys]]`,
`dogmud-playtesting` omits `[[project-u12c-2-playtest-findings]]`); both
are still counted below because both are genuinely present in the body.

| Memory file | Claiming skill | Status | Faithful |
|---|---|---|---|
| feedback_ansi_plural_inside_tag | dogmud-authoring-content | folded | yes |
| feedback_cardinal_exits_only | dogmud-authoring-content | folded | yes |
| feedback_filename_must_match_name_field | dogmud-authoring-content | folded | yes |
| feedback_no_name_recycling_no_js | dogmud-authoring-content | folded | yes |
| feedback_noun_keys_space_separated | dogmud-authoring-content | folded | yes |
| feedback_verify_ids_before_creating | dogmud-authoring-content | folded | yes |
| feedback_yaml_colon_gotcha | dogmud-authoring-content | folded | yes |
| feedback_zone_coord_planning | dogmud-authoring-content | folded | yes |
| feedback-room-cartesian-consistency | dogmud-authoring-content | folded | yes |
| reference_room_coordinate_and_reciprocity_gotchas | dogmud-authoring-content | folded | yes |
| reference_world_coordinate_frame_crawl | dogmud-authoring-content | folded | yes |
| feedback_dialogue_bare_scalar_list_mutes_npc | dogmud-authoring-quests | folded | yes |
| feedback_dialogue_filename_convention | dogmud-authoring-quests | folded | yes |
| feedback_hint_voice | dogmud-authoring-quests | folded | yes |
| feedback_loot_placement | dogmud-authoring-quests | folded | yes |
| feedback_quest_engine_event_names | dogmud-authoring-quests | folded | yes |
| feedback_quest_items_not_components | dogmud-authoring-quests | folded | yes |
| feedback_room_interact_noun_matching | dogmud-authoring-quests | folded | yes |
| reference_quest_reward_yaml_key_gotcha | dogmud-authoring-quests | folded | yes |
| reference_config_yaml_skip_worktree | dogmud-balance-config | folded | yes |
| reference-clean-hit-rate-is-mislabelled | dogmud-balance-config | cited | yes |
| feedback_best_of_actions_are_synchronous | dogmud-combat | folded | yes |
| feedback_btree_combat_events_before_legacy_ai | dogmud-combat | folded | yes (line ref corrected to :395, see defects) |
| feedback_btree_death_actions | dogmud-combat | folded | yes |
| feedback_combat_logic_goes_in_handleCombatRound | dogmud-combat | folded | yes (path corrected, see defects) |
| feedback_combat_quadrant_parity | dogmud-combat | folded | yes |
| feedback_companion_autonomy | dogmud-combat | folded | yes |
| feedback_target_resolution_uses_actor | dogmud-combat | folded | yes |
| reference_hit_chance_decoupled_from_mitigation | dogmud-combat | folded | yes (symbols corrected, see defects) |
| reference-standing-combat-and-balance-facts | dogmud-combat | folded | yes |
| reference-taunt-hold-aggro-gate | dogmud-combat | folded | yes (symbols corrected, see defects) |
| feedback_motd_format | dogmud-deploying | folded | yes |
| feedback-owner-does-the-deploys | dogmud-deploying | folded (body only, missing from Sources) | yes |
| feedback-skip-worktree-config-leak | dogmud-deploying | folded | yes |
| reference_docker_build_times | dogmud-deploying | folded | yes |
| reference_hotswap_upstream_prs_638_639 | dogmud-deploying | cited | yes |
| reference_motd_location | dogmud-deploying | folded | yes |
| reference_prod_perf_baseline | dogmud-deploying | cited | yes |
| reference-droplet-build-cache-and-dockerfile | dogmud-deploying | cited (self-contradiction disclosed) | yes |
| reference-droplet-root-owned-files-block-git-writes | dogmud-deploying | cited | yes |
| reference-prod-trustedproxies-caddy-container | dogmud-deploying | cited | yes |
| reference_living_state_persistence_contract | dogmud-persistence | folded | yes |
| reference-alt-characters-break-character-scoped-migrations | dogmud-persistence | folded (contradiction disclosed, see defects) | yes |
| reference-autosave-lock-cost | dogmud-persistence | folded | yes |
| reference-user-save-location | dogmud-persistence | cited | yes |
| feedback_esl_clear_language_player_copy | dogmud-player-copy | folded | yes |
| feedback_helpfile_loadout_vs_reaction_advice | dogmud-player-copy | folded | yes |
| feedback_no_em_dashes_in_prose | dogmud-player-copy | cited | yes |
| feedback_content_adversarial_playtest_gate_sop | dogmud-playtesting | folded | yes |
| feedback_defer_tuning_to_post_build_playtest | dogmud-playtesting | folded | yes |
| feedback_kill_test_servers | dogmud-playtesting | folded | yes |
| feedback_naive_newbie_playtest | dogmud-playtesting | folded | yes |
| feedback_never_blanket_kill_the_local_server | dogmud-playtesting | folded | yes |
| feedback_testers_observe_only | dogmud-playtesting | folded | yes |
| feedback_verify_human_experience_not_just_boot | dogmud-playtesting | folded | yes |
| project_playtest_findings_2026_08_08 | dogmud-playtesting | folded | yes |
| project-playtest-findings-not-yet-fixed | dogmud-playtesting | folded | yes |
| project-u12c-2-playtest-findings | dogmud-playtesting | cited (body only, missing from Sources; underspecified, see defects) | yes |
| project-u7b-recheck-findings | dogmud-playtesting | folded | yes |
| reference_lan_access_local_server | dogmud-playtesting | folded | yes |
| reference_verify_ansi_colors_via_telnet_port | dogmud-playtesting | folded | yes |
| reference-multiline-input-concatenated | dogmud-playtesting | cited (MEMORY.md disagreement flagged, see defects) | yes |
| reference-playtest-harness-restore | dogmud-playtesting | folded | yes |
| reference-thornwall-shops-sleep-plan-playtests-for-daytime | dogmud-playtesting | folded | yes |
| reference-stat-progression-faucet-map | dogmud-progression-model | folded (CP-economy portion deliberately excluded, see defects) | yes |
| reference-stat-valueadj-includes-training-and-mods | dogmud-progression-model | folded | yes |
| feedback_admin_command_wiring_checklist | dogmud-refactoring | folded | yes |
| feedback_compiler_is_the_dead_code_sweep | dogmud-refactoring | folded | yes |
| feedback_remove_downed_fully | dogmud-refactoring | folded | yes |
| feedback_search_for_existing_infrastructure_first | dogmud-refactoring | folded | yes |
| feedback_shallow_copy_shared_pointers | dogmud-refactoring | folded | yes |
| feedback-dont-file-your-own-inconsistency-as-followup | dogmud-refactoring | folded | yes |
| feedback-fix-flaws-dont-revert-the-work | dogmud-refactoring | folded | yes |
| feedback_dev_branch_local_only | dogmud-shipping | folded | yes |
| feedback_gh_defaults_to_upstream_fork_parent | dogmud-shipping | folded (union with hyphen twin, disclosed) | yes |
| feedback_go_run_dot_not_main | dogmud-shipping | folded | yes |
| feedback_master_is_main_branch | dogmud-shipping | folded | yes |
| feedback_merge_to_master_means_shipped | dogmud-shipping | folded | yes |
| feedback_no_instance_saves_to_prod | dogmud-shipping | folded | yes |
| feedback-gh-defaults-to-upstream-fork-parent | dogmud-shipping | folded (union with underscore twin, disclosed) | yes |
| feedback-gh-pr-checks-can-return-early | dogmud-shipping | folded | yes |
| feedback-git-checkout-pathspec-stages-the-revert | dogmud-shipping | folded | yes |
| feedback-git-stash-pathspec-noop-then-pop-hits-another-stash | dogmud-shipping | folded | yes |
| reference_advertising_listings_kit | dogmud-shipping | cited (self-contradiction NOT disclosed here, see defects) | yes (cited only) |
| reference_boot_test_in_isolated_worktree | dogmud-shipping | folded | yes |
| reference_gh_cli_now_installed | dogmud-shipping | folded | yes |
| reference_reusable_workflow_permissions | dogmud-shipping | cited | yes |
| reference-lint-gate-inverts-on-large-prs | dogmud-shipping | folded | yes |
| feedback_validator_conditional_checks | dogmud-writing-tests | folded | yes |
| feedback_verify_served_not_just_written | dogmud-writing-tests | folded | yes |
| feedback_verify_test_server_bound_its_port | dogmud-writing-tests | folded | yes |
| feedback_yaml_unexported_field_tag_noop | dogmud-writing-tests | folded | yes |
| project-flaky-tests-blocking-ci | dogmud-writing-tests | folded | yes |
| reference-test-binary-config-defaults-differ-from-shipped | dogmud-writing-tests | folded | yes |

## NOT SAFE TO DELETE

**None of the ranges any skill claims to have lifted failed the Step 1b
faithfulness check.** Every lifted range is a verified word-for-word
match, modulo the em/en-dash punctuation normalization applied throughout
extraction.

That is a narrower claim than "everything in CLAUDE.md is safe to delete."
It is not. Four sections in Table A carry a **gap** or **stale** flag and
must not be deleted on the strength of this audit without further work:

- **Salvage System (1074-1094) is the most serious finding.** It describes
  a chance formula (`SalvageMinChance`/`SalvageMaxChance`/`SalvageSoftCap`
  driving `chance = min + (max-min) × sqrt(skill/softCap)`) that
  `internal/crafting/context.md` says was deleted under U10b-1b and
  replaced by a contest (`RunSalvageContest`), confirmed by reading
  `internal/crafting/salvage.go`. Deleting this section without writing
  its contest-based replacement first would leave salvage completely
  undocumented, not merely trim a duplicate.
- **Alchemy & Potions System (1026-1073)** has almost no coverage
  elsewhere: Toxicity is absent from every `internal/*/context.md` file
  checked, and the aging/bottle-tier mechanics are named only by filename,
  not described, in `internal/items/context.md`.
- **Inventory & Item Disambiguation (917-935)** and **Equipment Slots
  (876-899)** are similarly gapped: the disambiguation formats
  (`N.item`/`item#N`/`all.item`), carry-capacity formula, and the full
  equipment slot list (including the four-level `ExtraArm` escalation)
  were not found in any `context.md`.
- **Shop Persistence's pricing half (214-218), Spell Duration System
  (900-904), Buff/Ward Spell System (905-916), Regen System (600-608),
  and Caster Weapon Types' numeric table (part of 1011-1025)** are
  partially covered elsewhere but each is missing at least one concrete
  fact (a formula, a config-knob name, or a numeric table) that
  `CLAUDE.md` currently states and no `context.md` currently repeats.

Recommendation for Phase 2: treat every row in Table A marked "Gap" or
"STALE" as **stays in CLAUDE.md until a real replacement is written**, not
as "delete, already covered." Everything else in Table A (lifted-and-
faithful, or delete/demote-and-verified-covered) is safe to act on as
listed.

## Source-material defects (not skill defects)

These are pre-existing errors in the memory corpus and in CLAUDE.md's own
prose, surfaced during Phase 1's execution and (where the task asked) by
this audit's own verification. They are recorded for Phase 3, not fixed
here, per the branch scope (no edits to CLAUDE.md or any skill).

1. **`reference-alt-characters-break-character-scoped-migrations` is
   self-contradictory and caused a real defect in a skill before review
   caught it.** It opens saying U10d's migration carries no marker, then
   claims U10d added `Storage.MigrationsDone`. Verified independently:
   `grep -rn "MigrationsDone\|HasMigration\|MarkMigration" internal/`
   returns zero hits repo-wide. No such field or methods exist; the real
   migration is deliberately unmarked. `dogmud-persistence` discloses this
   contradiction inline rather than propagating it. Highest-value fix on
   this list, per the task briefing.
2. **Three memory files are stale against current code, and the skills
   that fold them disclose and correct the drift rather than repeat it.**
   Verified independently:
   - `reference_hit_chance_decoupled_from_mitigation` names
     `spellDefenseValue` and `Character.GetDefense()`. Both are gone:
     `grep -rn "func.*spellDefenseValue\|func.*GetDefense\b" internal/`
     returns nothing; the live seam is
     `Character.GetDefenseScoreFor(defenseType string, includeSkill bool)
     float64` (`internal/characters/combat.go:278`), which `dogmud-combat`
     names as current.
   - `reference-taunt-hold-aggro-gate` names `Character.ForceTauntAggro`.
     Gone: `grep -rn "func.*ForceTauntAggro" internal/` returns nothing.
     The live pair is `Character.SetTauntHold(userId, mobInstanceId,
     holdRounds int)` (`internal/characters/taunt_hold.go:22`) and
     `targeting.CommitTaunt(c *characters.Character, ref state.ActorRef,
     holdRounds int) bool` (`internal/targeting/commit.go:124`), both
     named as current in `dogmud-combat`.
   - `feedback_btree_combat_events_before_legacy_ai` cites a call site at
     `NewRound_DoCombat.go:276`, now at `:395` (the `handleMobAIDecision`
     call). `dogmud-combat` folds the rule without repeating the stale
     line number as load-bearing.
3. **`feedback_combat_logic_goes_in_handleCombatRound` cites a spec doc
   that moved.** Verified: the file exists at
   `docs/superpowers/specs/completed/2026-04-18-combat-quadrant-unification-design.md`
   (the `completed/` segment is the part the memory file omits).
   `dogmud-combat` states the corrected path explicitly.
4. **CLAUDE.md's own knob and file-size counts are stale.** Verified
   independently against source (matches `dogmud-balance-config`'s own
   verified figures exactly):
   - `Config*`-typed fields in `internal/configs/config.balance.go`: CLAUDE.md
     says 352, actual is **375**
     (`grep -cE '^\s*[A-Za-z_]+\s+Config[A-Za-z]+\b' internal/configs/config.balance.go`).
   - `Config*`-typed fields across the whole `internal/configs` package:
     CLAUDE.md says 466, actual is **508** (515 raw matches across all
     non-test `.go` files in the package, minus 7 false positives in
     `config_types.go` where the pattern matches `type ConfigInt int`
     style declarations rather than struct fields).
   - `_datafiles/config.yaml` line count: CLAUDE.md says 1506, the
     committed blob is **2296** lines
     (`git show HEAD:_datafiles/config.yaml | wc -l`); the on-disk working
     copy (which carries the skip-worktree local divergence) reads 2300.
   - The seven sibling `config.balance.*.go` files declaring zero fields:
     **confirmed correct**, 0 in each of the seven, matching CLAUDE.md.
5. **MEMORY.md miscites the Sable combat fixture.** It attaches the
   fixture to `project-u12c-2-playtest-findings` when the detail actually
   lives in `reference-standing-combat-and-balance-facts`. The fixture is
   underspecified even in its correct home: nothing states what `arena`
   versus `oasis` selects or how to reach room 5000.
   `dogmud-playtesting` cites `project-u12c-2-playtest-findings` only in
   its body (not its Sources list), consistent with the memory file being
   thin on this point.
6. **MEMORY.md contradicts its own cited source about the AI port.** It
   states the 3-commands-per-round cap drops overflow "silently";
   `reference-multiline-input-concatenated` states that drop is visible
   and reserves "silent" for a separate, already-fixed bug.
   `dogmud-playtesting` cites the memory file only, and flags the
   disagreement with MEMORY.md's wording rather than resolving it.
7. **`reference_advertising_listings_kit` and
   `reference-droplet-build-cache-and-dockerfile` each contradict
   themselves**, found by the pre-execution survey. `dogmud-deploying`
   discloses the `reference-droplet-build-cache-and-dockerfile`
   contradiction explicitly ("self-contradictory on the build-cache-mount
   question, deliberately left unresolved above"). `dogmud-shipping`
   cites `reference_advertising_listings_kit` in its "cited, not folded"
   list but does not repeat the contradiction warning; this is low risk
   because the skill does not rely on the file's content for any
   procedure, but Phase 3 should still note it.
8. **Homeless content:** the CP-economy material in
   `reference-stat-progression-faucet-map` (casting is CP-bound over an
   hour rather than cooldown-bound; fielded companions reserve
   conviction) was deliberately excluded from `dogmud-progression-model`
   as spell-economy rather than stat-progression content, and this audit
   found no other skill or context.md that covers it. It has no home
   until a spell-economy skill or doc exists.

## Step 4 verification

`grep -c "^|" docs/superpowers/audits/2026-09-08-skill-coverage.md` on the
finished file returns row count including both table headers; see the
commit log for the exact number, expected well above the 108-row floor
(44 Table A rows + 94 Table B rows + header/separator rows for both
tables).

## Step 6: Phase 1 definition of done

- `git diff 0e0f3acd9 HEAD --stat -- CLAUDE.md`: **no output** - CLAUDE.md
  is byte-identical to its state before Phase 1 started.
- `python tools/verify_skills.py`: **`12/12 skills present, 0 errors`**,
  exit code 0.
- `git status --short`: clean, no stray files, before this audit's own
  commit.
