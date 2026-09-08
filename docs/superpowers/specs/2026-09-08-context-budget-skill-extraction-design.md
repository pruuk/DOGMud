# Context Budget: Extracting Skills from CLAUDE.md and MEMORY.md

**Date:** 2026-09-08
**Status:** design approved, not yet planned
**Full path:** `docs/superpowers/specs/2026-09-08-context-budget-skill-extraction-design.md`

## Facts verified against source (2026-09-08)

Every number below was read from the tree on the date of writing, not recalled.

| Fact | Value | How verified |
|---|---|---|
| `CLAUDE.md` (project) | 61,403 bytes / 1,094 lines | `wc -c -l CLAUDE.md` |
| `MEMORY.md` | 17,591 bytes / 82 lines | `wc` |
| Always-loaded total | approx 79 KB, approx 20k tokens | sum of the two |
| Always-loaded target after | approx 15 KB (CLAUDE.md approx 5 KB + MEMORY.md approx 10 KB) | derived, not measured |
| Memory dir | 315 `.md` files, 1,621,604 bytes | `ls`, `wc -c` |
| `feedback_*` / `feedback-*` | 70 files, 140,384 bytes | `ls`, `wc -c` |
| `reference_*` / `reference-*` | 32 files, 192,358 bytes | same |
| `project*` | 211 files, 1,252,738 bytes | same |
| Largest memory file | `reference_prod_perf_baseline.md`, 94,561 bytes | `ls -S` |
| Wikilinks in memory dir | 706 total, 306 distinct targets | grep over `[[...]]` |
| Inbound links to `feedback*` targets | 176 links, 75 distinct targets, across 73 files | same |
| `.claude/skills/` | does not exist | `ls -a .claude/` |
| `.claude/settings.json` | does not exist | `cat` returned absent |
| `.claude/settings.local.json` | exists, gitignored at `.gitignore:55` | `test -f`, `grep -n` |
| `.gitignore` `.claude` entries | `settings.local.json`, `worktrees/`, `projects/`, `scheduled_tasks.lock` | `.gitignore:55,56,127,128` |
| `.claude/commands/` | 8 files | `ls` |
| `upstream` push URL | `DISABLED` | `git remote -v` |
| `docs/schemas/` | 12 files incl. `schedule.md`, `patrol.md`, `conversation.md`, `mob.md` | `ls` |
| `docs/guides/` | 5 files incl. `github_guide.md`, `DEPLOYMENT_GUIDE.md`, `TESTING_GUIDE.md` | `ls` |
| Packages with `context.md` | 15 of 17 checked have one | per-dir `test -f` |
| `internal/schedules`, `internal/patrols` | do not exist; that code is in `internal/mobs` | `ls internal/`, grep for `schedule_id` / `patrol_id` |
| `internal/combat/context.md` | 113,923 bytes | `wc -c` |
| `internal/characters/context.md` | 92,304 bytes | `wc -c` |
| `internal/mobs/context.md` | 45,334 bytes | `wc -c` |
| `internal/items/context.md` | 32,788 bytes | `wc -c` |
| `internal/parser/context.md` | 3,894 bytes | `wc -c` |

### Survey classification

Two subagents read all 102 `feedback*` and `reference*` files in full and
classified each one.

| KIND | feedback | reference | total |
|---|---|---|---|
| PROCEDURE | 53 | 11 | 64 |
| PREFERENCE | 11 | 0 | 11 |
| FACT | 2 | 12 | 14 |
| INCIDENT | 4 | 8 | 12 |
| LOG | 0 | 1 | 1 |

## Problem

Roughly 79 KB (approx 20k tokens) loads into every session before the user types
anything. Most of it is not state. It is procedure: how to ship, how to author a
room, how the damage pipeline composes, how to run a boot check. That material
matters only when the matching task is underway, and a skill is the mechanism
for load-on-demand procedure.

`MEMORY.md` alone has only about 5 KB to give up. Its Current-status and
Remaining-work sections are genuinely volatile state, which is what an index is
for. The bulk of the extractable material sits in `CLAUDE.md`.

## Decision summary

| Decision | Choice |
|---|---|
| Scope | Both `CLAUDE.md` and `MEMORY.md` |
| Skill location | `.claude/skills/` in the repo, committed |
| Carve axis | Hybrid: task-verb skills, plus cross-cutting model skills |
| Hazard coverage | `PreToolUse` hooks for the four with incident history, plus a tripwire block |
| Folded memory files | Tombstoned, not deleted |

## The twelve skills

Task-verb skills fire on intent. Model skills carry knowledge that spans
packages, so no single `context.md` can own it.

| Skill | Fires when | Absorbs |
|---|---|---|
| `dogmud-shipping` | commit, push, PR, merge, pre-push | Git Workflow, gh `--repo` pinning, Ship-via-PR, Pre-Push SOP, Instance-Saves and Smoke-Test SOP; 10 feedback + 6 reference files |
| `dogmud-deploying` | deploying to the droplet | 3 feedback + 6 reference files; points at `docs/guides/DEPLOYMENT_GUIDE.md` |
| `dogmud-authoring-content` | new room / mob / item YAML, zone work | Content Playtest-Review Gate, ID Inventory, Data File Naming, Content Generation Commands; 10 feedback + 2 reference files |
| `dogmud-authoring-quests` | quests, dialogue, NPC text | Quest Re-Grant Prevention, Quest NPC Dialogue SOP, Dialogue Voice, give.go gotcha, givesItem, Quest Flags, Branching Quest SOP; 6 feedback + 1 reference |
| `dogmud-player-copy` | writing anything a player reads | MUD Line Width, No Hard Numbers; 3 feedback files |
| `dogmud-writing-tests` | writing or debugging a Go test | 4 feedback + 1 reference file; points at `docs/guides/TESTING_GUIDE.md` |
| `dogmud-playtesting` | running the harness, triaging findings | AI Testing; 7 feedback + 5 reference files |
| `dogmud-persistence` | adding persisted state, writing a migration | Shop Persistence and Moderation Persistence (the file-location and do-not-wipe halves), living-state contract, alt-migration trap |
| `dogmud-refactoring` | modifying code that already works | 4 unbucketed feedback files: compiler-is-the-dead-code-sweep, search-for-existing-infrastructure-first, shallow-copy-shared-pointers, finish-the-sibling-paths |
| `dogmud-combat` | touching damage, defence, contests, mob AI placement | Two sections. Model: Damage Pipeline, Resource Depletion, Defense Resolution, Combat Design Conventions, Dice and Rolling. Placement: 8 feedback files on where combat and btree logic goes |
| `dogmud-progression-model` | touching stats, skills, advancement | Stat and Progression System; 2 reference files |
| `dogmud-balance-config` | changing any tunable number | Balance Lives in config.yaml; 2 reference files |

The file counts in the Absorbs column are **bucket** counts from the survey, not
fold counts. A bucket contains every file about that topic; only its PROCEDURE
members fold into the skill, per rule 1 of the fold law. The INCIDENT and LOG
members of a bucket stay as memory files and are cited by the skill, not copied
into it.

### Unassigned residue

Two PROCEDURE files bucketed to `none` and do not obviously belong to any of the
twelve. Their home is a plan-time decision, not a design-time one, and naming
them here stops them being silently dropped:

- `feedback_admin_command_wiring_checklist` (plans must enumerate every admin
  command wiring step). Candidate homes: `dogmud-refactoring`, or a new
  planning-discipline home, or the residual CLAUDE.md.
- `reference_autosave_lock_cost` (autosave lock-hold cost across three chunks).
  Labelled PROCEDURE by the survey but reads closer to a performance record.
  Candidate homes: `dogmud-persistence`, or reclassify to LOG and leave it.

`feedback-never-python-read-modify-write-any-file` is also bucketed `none` and is
deliberately not folded: it becomes hook 2 and a tripwire line.

### Why working-style and memory-maintenance are not skills

Both fire unprompted. Nobody announces "I am about to edit MEMORY.md" or "I am
about to choose a proposal length," so a skill that must first be recognised
cannot cover them. They stay in always-loaded context, which is what
always-loaded context is for.

## Residual CLAUDE.md

Target: approx 70 lines, down from 1,094. Six blocks.

1. **Project Context.** What DOGMud is, `docs/world.md`, the roadmap, the remotes.
2. **Working style.** The 11 PREFERENCE files, one line each: subagent-driven
   execution, PowerShell for Windows process work, no focus-stealing console
   windows, no em or en dashes, 30 to 40 line proposals, visual companion is a
   standing yes, a found flaw widens scope rather than cutting it, fix flaws
   rather than reverting, finish sibling paths you made inconsistent, the owner
   runs all deploys, Fable outranks Opus.
3. **Tripwires.** One line per hazard, each naming the skill to load. This is the
   safety net for everything that moved out.
4. **The `context.md` convention.** Stays. It governs where new subsystem
   knowledge lands. Without it the skills re-accumulate everything.
5. **Codegraph guidance.** Approx 4 lines. Fires unprompted, before code is written.
6. **Where the skills live.** One line. The harness already lists every skill
   name and description automatically, so a full index here would be dead
   duplication.

## Hooks

`.claude/settings.json`, a new committed file. `PreToolUse` on Bash and
PowerShell, matched against the command string.

| # | Blocks | Incident behind it |
|---|---|---|
| 1 | `gh` subcommands that accept a repo, without `--repo pruuk/DOGMud` | a PR opened on `GoMudEngine/GoMud`, 2026-08-08 |
| 2 | python `open(..., 'w')` or shell truncation against any `.md` | MEMORY.md destroyed twice |
| 3 | `git add -A` and `git add .` | recorded in the index as a trap |
| 4 | blanket process-kill matching the game server | the user's live server killed mid-test |

No hook for `git push upstream`: that remote already has its push URL set to
`DISABLED`, verified above. The live exposure is `gh`, which ignores remotes and
resolves the fork parent through the API. Hook 1 covers it.

### Known limits, stated rather than discovered later

- Hooks match text. An unusual spelling slips through. They lower the failure
  rate, they do not eliminate it.
- They fire on the user too. Each needs a documented escape phrase.
- `settings.json` is committed, so these apply to every agent in the repo.

## context.md demotions

Several CLAUDE.md sections duplicate documents that already exist. Those are
deleted, not rewritten, with the owning skill carrying a pointer.

| CLAUDE.md section | Destination | Action |
|---|---|---|
| NPC Schedules | `docs/schemas/schedule.md` | delete from CLAUDE.md |
| NPC Patrols | `docs/schemas/patrol.md` | delete |
| NPC to NPC Conversations | `docs/schemas/conversation.md` plus `internal/conversations/context.md` | delete |
| Mob Stat Archetypes | `docs/schemas/mob.md` | delete |
| Shop Persistence (pricing formula and knobs) | `internal/shops/context.md` | demote |
| Moderation Persistence | `internal/moderation/context.md` | demote |
| Map Consistency and `non_cartesian` | `internal/mapper/context.md` | demote |
| Equipment Slots, Caster Weapon Types, Alchemy and Potions, Inventory Disambiguation | `internal/items/context.md` | demote |
| Salvage System | `internal/crafting/context.md` | demote |
| Spell Duration, Buff and Ward Spells | `internal/spells/context.md`, `internal/buffs/context.md` | demote |
| Command Parsing and Multi-Word Input | `internal/parser/context.md` | demote |
| Sleep Mechanics | `internal/mobs/context.md` | demote |
| Regen System | `internal/characters/context.md` | demote |

## MEMORY.md, after

Target: approx 40 lines / 10 KB, from 82 lines / 17.6 KB.

| Section | Fate |
|---|---|
| Current status | stays |
| Remaining Work | stays |
| Repo Pointers | stays, minus the Reference-notes sub-list that folds |
| Git Workflow extras | folds into `dogmud-shipping` |
| SOPs | folds across skills |
| Feedback / Gotchas | folds across skills |

`COMPLETED.md` is untouched.

## The fold law

Five rules, so future triage does not need re-litigating.

1. **PROCEDURE folds. LOG and INCIDENT never fold.** A dated finding is
   evidence. A rule is reusable. Only rules become skills.
2. **A FACT folds only if a skill needs it to act.**
   `reference-user-save-location` drives no procedure, so it stays a memory file.
3. **PREFERENCE goes to the CLAUDE.md working-style block, never to a skill.**
   It fires unprompted.
4. **Nothing is duplicated.** If it is in a skill it is not also in CLAUDE.md,
   MEMORY.md, or a `context.md`.
5. **An oversized `context.md` is a refactor signal, not a place to append.**

## Tombstones

A folded memory file is not deleted. Its body is replaced by a stub that keeps
the original frontmatter, names the skill that now owns the procedure, and
retains the dated provenance below a rule.

```markdown
---
name: <unchanged>
description: FOLDED into skill dogmud-shipping on 2026-09-08 - <original one-liner>
metadata:
  type: feedback
---

**This procedure now lives in the skill `dogmud-shipping`.** Load that skill
rather than acting on this file. Kept for provenance and for the 176 inbound
wikilinks.

---

<original body, unchanged>
```

Rationale: the memory dir is already load-on-demand, so deletion buys no context
tokens. It only buys tidiness, and it would cost 176 link rewrites across 73
files plus the dated provenance those files carry.

## Phasing

This is too large for one plan chunk: twelve skills, four hooks, thirteen
demotions, two file rewrites, and up to 64 tombstones. It should phase, and each
phase must leave the repo in a working state:

1. **Skills only.** Write the twelve skills. Nothing is deleted yet, so the
   material is briefly duplicated. Verifiable in isolation: each skill loads and
   reads correctly.
2. **Cut over.** Strip CLAUDE.md to the six blocks, shrink MEMORY.md, apply the
   demotions. This is the phase that removes duplication, so rule 4 of the fold
   law is checked here.
3. **Hooks and tombstones.** Add `.claude/settings.json`, live-test each hook,
   then tombstone the folded memory files.

Phase 1 is reversible and low-risk. Phase 2 is the one that can lose material,
so it wants the semantic-compare discipline rather than a positional diff.

## Out of scope

- Right-sizing `internal/combat/context.md` (113 KB) and
  `internal/characters/context.md` (92 KB). Their size is a signal that those
  packages need a rewrite pass. That is a separate job and must not be smuggled
  into this one. Recorded here so it is not lost.
- The 211 `project*` memory files. They are dated work records and stay.
- Two memory files the survey found self-contradictory:
  `reference_advertising_listings_kit.md` (says the Grapevine MSSP checker is the
  validator of record, then says it is dead) and
  `reference-droplet-build-cache-and-dockerfile.md` (rejects a build-cache mount,
  then reverses that later in the same file). Worth a separate fix.

## Success criteria

1. Always-loaded context drops from approx 79 KB to approx 15 KB, measured by
   `wc -c` on `CLAUDE.md` plus `MEMORY.md`.
2. Every one of the 64 PROCEDURE files is reachable from exactly one skill, a
   hook, or the residual CLAUDE.md. The two named in Unassigned residue have
   been placed.
3. All 706 wikilinks still resolve. No file is deleted.
4. The four hooks each block their hazard in a live test, and each has a
   documented escape phrase.
5. No rule exists in two places. Spot-checked against the fold law's rule 4.
