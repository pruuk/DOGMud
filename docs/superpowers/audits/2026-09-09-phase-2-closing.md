# Phase 2 closing audit: nothing lost

Date: 2026-09-09
Branch: feature/context-budget-skill-extraction
Baseline commit for "original": 0e0f3acd9 (CLAUDE.md at 1,094 lines, 44
sections)

## What Phase 2 did

Phase 2 took the 1,094-line, always-loaded `CLAUDE.md` and cut it to 84 lines
by moving its 44 sections to where a reader would actually need them: 31
sections were deleted because a skill written in Phase 1 (or an existing
`docs/` file) already carried the same material, 9 were rewritten into six
`internal/*/context.md` files that are loaded only when that package is
touched, and 4 were condensed and kept in place because they apply on every
session regardless of task. Alongside the move, each section was checked
against the current source rather than copied verbatim, which is how this
phase also found and corrected several defects in CLAUDE.md's own text (see
below) and cleared three phantom symbols from `internal/items/context.md`.

## Table A: all 44 original sections

Every row below was checked with a grep run in this session against the
current tree, not assumed from the plan or from memory. The "Verified by"
column names the exact string grepped and, where useful, the command shape.

| # | Section (original heading) | Original lines | Current home | Verified by |
|---|---|---|---|---|
| 1 | Content Playtest-Review Gate (SOP) | 3-24 | `dogmud-authoring-content`, `dogmud-playtesting` skills | grep `"adversarial playtest-harness review"` hits both SKILL.md files |
| 2 | Subagent Model Preference | 25-43 | `CLAUDE.md` (Working style) | grep `"default up to sonnet"` hits CLAUDE.md |
| 3 | Git Workflow | 44-98 | `dogmud-shipping` skill, `docs/guides/github_guide.md` | grep `"gh DEFAULTS TO THE PARENT"` / `"no-ff"` hit both |
| 4 | Pre-Push SOP | 99-158 | `dogmud-shipping` skill | grep `"gofmt -l internal"` and `"boot-check.exe"` both hit SKILL.md |
| 5 | Instance Saves & Smoke-Test SOP | 159-205 | `dogmud-persistence` skill | grep `"restoreSkipTaggedFields"` hits SKILL.md |
| 6 | Shop Persistence (Living Economy) | 206-219 | `internal/shops/context.md` | grep `"ShopAbundanceThreshold"` hits context.md, not the skill |
| 7 | Moderation Persistence | 220-234 | `internal/moderation/context.md`, `dogmud-persistence`; the trailing non_combatant line lives in `internal/mobs/context.md` | grep `"petitions.yaml"` / `"bans.yaml"` and separately `"non_combatant: true"` |
| 8 | NPC Schedules | 235-245 | `docs/schemas/schedule.md` | grep `"schedule_id"` hits schedule.md |
| 9 | Sleep Mechanics | 246-258 | `docs/schemas/schedule.md` | grep `"ScheduleWakeGraceRounds"` hits schedule.md |
| 10 | NPC Patrols | 259-274 | `docs/schemas/patrol.md` | grep `"ScheduleMaxPathRetries"` / `"yo-yo"` hit patrol.md |
| 11 | NPC to NPC Conversations | 275-309 | `docs/schemas/conversation.md` | grep `"ConversationBaseChancePct"` hits conversation.md |
| 12 | Map Consistency & non_cartesian/oneway | 310-354 | `internal/mapper/context.md` | grep `"MapConsistencyEnforce"` / `"cartcheck"` hit context.md |
| 13 | Project Context | 355-361 | `CLAUDE.md` (Project Context) | grep `"DOGMud (Delusions of Grandeur)"` hits CLAUDE.md line 4 |
| 14 | Stat & Progression System | 362-425 | `dogmud-progression-model` skill | grep `"StatProgressionSoftCap"` / `"ProgressionChanceForStat"` hit SKILL.md |
| 15 | Dice & Rolling System | 426-453 | `dogmud-combat` skill | grep `"combat.RunContest"` / `"RunConcentrationContest"` hit SKILL.md |
| 16 | Balance Lives in config.yaml, Not in Code | 454-487 | `dogmud-balance-config` skill | grep `"352 balance knobs"` hits SKILL.md (count is stale, see Defects) |
| 17 | Unified Damage & Mitigation Pipeline | 488-561 | `dogmud-combat` skill | grep `"GlobalDamageMultiplier"` / `"ChannelScale"` hit SKILL.md |
| 18 | Resource Depletion Penalties | 562-581 | `dogmud-combat` skill | grep `"ResourcePenaltyCurve"` hits SKILL.md |
| 19 | Defense Resolution: Best-of-All | 582-592 | `dogmud-combat` skill | grep `"MinDefenseChance"` hits SKILL.md |
| 20 | Combat Design Conventions | 593-599 | `dogmud-combat` skill | grep `"ProneAttackMultiplier"` hits SKILL.md |
| 21 | Regen System | 600-608 | `internal/characters/context.md` | grep `"PlayerHealthRegenPct"` / `"HealthPerRound"` hit context.md |
| 22 | ID Inventory & Collision Prevention | 609-643 | `dogmud-authoring-content` skill | grep `"id_inventory.py --alloc"` hits SKILL.md |
| 23 | Codegraph MCP - Code Intelligence | 644-690 | `CLAUDE.md` (Codegraph MCP, condensed) | grep `"codegraph_context"` / `"codegraph_trace"` hit CLAUDE.md |
| 24 | Package context.md Convention | 691-732 | `CLAUDE.md` (Package context.md Convention, condensed) | grep `"context_md_audit.py"` / `"phantom symbols"` hit CLAUDE.md |
| 25 | Data File Naming Convention | 733-741 | `dogmud-authoring-content` skill | grep `"ConvertForFilename"` hits SKILL.md |
| 26 | Command Parsing & Multi-Word Input | 742-773 | `internal/parser/context.md` | grep `"SplitTrailingContainer"` / `"SplitLeadingMatch"` hit context.md |
| 27 | MUD Line Width | 774-776 | `dogmud-player-copy` skill | grep `"80 characters per line"` hits SKILL.md |
| 28 | Player-Facing Messages - No Hard Numbers | 777-785 | `dogmud-player-copy` skill | grep `"GetDamageDescription"` / `"GetHealDescription"` hit SKILL.md |
| 29 | Quest Re-Grant Prevention SOP | 786-793 | `dogmud-authoring-quests` skill | grep `"questExcluded"` hits SKILL.md |
| 30 | Quest NPC Dialogue SOP | 794-799 | `dogmud-authoring-quests` skill | grep `'"quest" and "task" in its triggers'` hits SKILL.md |
| 31 | Dialogue Voice & Trigger Discoverability | 800-813 | `dogmud-authoring-quests` skill | grep `"3rd-person self-references"` hits SKILL.md |
| 32 | Quest Item Delivery - give.go Gotcha | 814-825 | `dogmud-authoring-quests` skill | grep `"give.go transfers"` / `"return_item"` hit SKILL.md |
| 33 | Dialogue Engine: givesItem | 826-830 | `dogmud-authoring-quests` skill | grep `"givesItem: <itemId>"` hits SKILL.md |
| 34 | Quest Flags System | 831-875 | `dogmud-authoring-quests` skill | grep `"questFlagRequired"` / `"Branching Quest SOP"` hit SKILL.md |
| 35 | Equipment Slots | 876-899 | `internal/items/context.md` | grep `"tailsweep"` / `"Arm 3 + Wrist 3"` hit context.md |
| 36 | Spell Duration System | 900-904 | `internal/spells/context.md` (formula also cross-referenced in buffs/characters/hooks) | grep `"calcSpellDuration"` hits spells/context.md |
| 37 | Buff/Ward Spell System | 905-916 | `internal/buffs/context.md` | grep `"Chrysalis Cocoon"` / `"effect_magnitude"` hit context.md |
| 38 | Inventory & Item Disambiguation | 917-935 | `internal/items/context.md` | grep `"diku-style"` / `"item#N"` hit context.md |
| 39 | Content Generation Commands | 936-952 | `docs/guides/CONTENT_GENERATION_GUIDE.md`, `dogmud-authoring-content` skill | grep `"new-mob"` / `"zone-sketch"` hit both |
| 40 | AI Testing | 953-1002 | `dogmud-playtesting` skill | grep `"playtestrun"` / `"ephemeral:"` hit SKILL.md |
| 41 | Mob Stat Archetypes | 1003-1010 | `docs/schemas/mob.md` | grep `"archetype: fighting"` hits mob.md |
| 42 | Caster Weapon Types | 1011-1025 | `internal/items/context.md`, `docs/schemas/item.md` | grep `"spell_damage_multiplier"` / `"sceptre"` hit both |
| 43 | Alchemy & Potions System | 1026-1073 | `internal/items/context.md` | grep `"BottleAgingMultiplier"` / `"witcher-style"` hit context.md |
| 44 | Salvage System | 1074-1094 | `internal/crafting/context.md` | grep `"salvage_returns"` / `"SalvageMinChance"` hit context.md |

## Table B: sizes before and after

"Before" for `CLAUDE.md` and the six `context.md` files is read from commit
`0e0f3acd9`, the last commit before Phase 2 touched anything. "Before" for
`MEMORY.md` (83 lines) is as given in this task's brief; `MEMORY.md` lives
outside this git repository (under the Claude Code memory directory) and has
no retained history to re-derive that figure from independently, so it is
reported, not re-verified.

| File | Before (lines / bytes) | After (lines / bytes) |
|---|---|---|
| `CLAUDE.md` | 1,094 / 60,309 | 84 / 4,163 |
| `MEMORY.md` | 83 / not available | 62 / 13,116 |
| `internal/items/context.md` | 943 | 1,169 |
| `internal/crafting/context.md` | 277 | 353 |
| `internal/shops/context.md` | 247 | 345 |
| `internal/buffs/context.md` | 809 | 909 |
| `internal/characters/context.md` | 1,806 | 1,861 |
| `internal/hooks/context.md` | 1,604 | 1,647 |

## Tooling gate (Step 2)

```
$ python tools/verify_skills.py; echo exit=$?
12/12 skills present, 0 errors
exit=0
```

```
$ for p in items crafting shops buffs characters hooks; do
    printf "%-12s " "$p"; python tools/context_md_audit.py internal/$p | tail -1
  done
items        All documented symbols resolve.
crafting     All documented symbols resolve.
shops        All documented symbols resolve.
buffs        All documented symbols resolve.
characters   All documented symbols resolve.
hooks        All documented symbols resolve.
```

`internal/items/context.md` began this phase carrying three phantom symbols
(`UseItem`, `ItemPower`, `IsUpgrade`) that no longer existed in the package.
The audit script now reports it clean along with the other five.

## NOTHING LOST

**All 44 original sections are accounted for.** Every row in Table A has a
verified current home and a grep string that a reader can re-run to confirm
it. No section was found with no home; none needed to be reported as lost.

## Defects found in CLAUDE.md's own text while relocating it

Rewriting each section against the live source, rather than copying it
verbatim, surfaced defects that had been sitting in the always-loaded file
undetected. Recorded here because the same wording may have been copied
elsewhere.

- It named `calcSpellDamage()` and `calcMobSpellDamage()` as where
  `spell_damage_multiplier` is applied. Neither function exists. The real
  one is `calcSpellDamageForCharacter`, confirmed at
  `internal/hooks/combat_shared_helpers.go:35`.
- Its Extra Arms penalty table used the Go defaults
  (`MutationLevel2/3/4Multiplier` 1.5/2.0/2.5) as if they were live values.
  Shipped `config.yaml:1580-1582` sets them to 1.6/2.5/4.0, confirmed by
  direct read. This broke the rule its own "Balance Lives in config.yaml"
  section stated about four hundred lines away in the same file.
- It documented Extra Arms levels 2 through 4 as reachable. The mutation
  ships `max_rank: 1`, confirmed in the mutation's own YAML
  (`max_rank: 1  # connective-tissue bridge: apex-class, binary (no
  deepen)`), so only level 1 is currently attainable.
- It summarised `GetToxicityMax()` without its Alchemy-skill term, leaving
  the formula incomplete.
- Its Salvage section described a chance curve that U10b-1b already
  replaced with a contest (`internal/crafting/context.md` now documents
  `difficulty.go`'s craft/salvage contest, not a flat percentage curve),
  and it named three knobs as live tuning that turned out to have zero
  consumers (see the dead-knob list below).
- It claimed "Shield duration: crits +50% strength" as an unconditional
  rule. That branch is unreachable for both spells that currently ship a
  shield effect.
- Its counts were stale. It said 352 balance knobs, 466 `Config*`-typed
  fields, and a 1,506-line `config.yaml`. A fresh grep in this session
  against the current tree instead counts 397 typed fields in
  `internal/configs/config.balance.go` alone (pattern:
  `^\s*[A-Za-z][A-Za-z0-9_]*\s+(ConfigFloat|ConfigInt|ConfigBool|
  ConfigString|ConfigSecret|ConfigSliceString)\b`), 536 such fields across
  the whole config package excluding `_test.go` files, and 2,300 lines in
  `_datafiles/config.yaml` (`wc -l` and `grep -c ""` agree). Whatever the
  exact right number, the direction of the finding holds either way: the
  counts in the original file were stale and nobody had re-run them in
  months. This audit's own counts should likewise be treated as true only
  as of 2026-09-09, not carried forward without re-checking.

## Dead config knobs found

Six balance knobs were found with zero consumers outside their own
declaration (`internal/configs/config.balance.go`) and defaulting logic
(the matching `config.balance.*.go` validator), confirmed by grepping each
name across `internal/**/*.go` excluding `_test.go` files:

- `SalvageMinChance`, `SalvageMaxChance`, `SalvageSoftCap`
- `ShopMaterialReserve`, `BarterMaxDiscount`, `BarterMaxBonus`

All six are nonetheless SET in `_datafiles/config.yaml` with explanatory
comments (for example `BarterMaxDiscount: 0.15  # Max fractional buy-price
discount from bartering`), which makes them read as live tuning to anyone
skimming the file. They are not: `internal/actions/buy.go:534` and
`internal/actions/sell.go:285` each hard-code the literal `0.15` for the
bartering discount/bonus rather than reading `BarterMaxDiscount` /
`BarterMaxBonus`, confirmed by reading both call sites directly.

## Recorded, not acted on

`internal/characters/context.md` (1,806 lines before this work) and
`internal/hooks/context.md` (1,604 lines before this work) remain the two
largest `context.md` files in the tree and grew slightly during this phase
(to 1,861 and 1,647 respectively) because content that belonged there had
to be added correctly. Right-sizing either file was out of scope for Phase
2, which was about relocating CLAUDE.md content, not auditing every
oversized context.md in the repo.

## For Phase 3

The following were surfaced during this phase's work but belong to memory
hygiene, not to the CLAUDE.md relocation this phase covers, and are handed
off rather than fixed here:

- Four self-contradictory or stale memory files identified during this
  phase's review pass.
- `MEMORY.md`'s Sable miscitation (the Rift Chamber 5000 pointer under
  `project-u12c-2-playtest-findings`) and its AI-port contradiction (the
  "AI port caps 3 cmds/round" line versus other multiline-input notes).
- The homeless CP-economy material recorded in
  `reference-stat-progression-faucet-map` that never found a home in a
  package `context.md` or skill.
- The non-contested spell buffing slice recorded in memory as
  `project-non-contested-spell-buff-unification`, which is design work, not
  documentation relocation, and was left for its own plan.
