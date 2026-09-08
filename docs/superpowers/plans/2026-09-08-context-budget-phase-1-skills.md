# Context Budget Phase 1: Write the Twelve Skills

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create twelve on-demand skills under `.claude/skills/`, each carrying
procedure lifted from `CLAUDE.md` and from the 64 PROCEDURE memory files, so that
Phase 2 can strip `CLAUDE.md` without losing anything.

**Architecture:** Phase 1 is purely additive. Nothing is deleted, nothing is
edited outside `.claude/skills/`. Material is deliberately duplicated between
CLAUDE.md and the new skills for the duration of this phase; fold-law rule 4 (no
duplication) is enforced in Phase 2, not here. That is what makes this phase
reversible: `rm -rf .claude/skills` restores the prior state exactly.

**Tech Stack:** Markdown. Claude Code skill format: `.claude/skills/<name>/SKILL.md`
with YAML frontmatter carrying `name` and `description`.

**Source spec:** `docs/superpowers/specs/2026-09-08-context-budget-skill-extraction-design.md`

---

## Facts verified against source (2026-09-08)

| Fact | Value | How verified |
|---|---|---|
| Skill format | `<skills-dir>/<name>/SKILL.md`, frontmatter `name` + `description` | inspected `superpowers/5.1.0/skills/brainstorming/` |
| `.claude/skills/` | does not exist yet | `ls -a .claude/` |
| `CLAUDE.md` | 1,094 lines, 44 `##` sections | `wc -l`, `awk` over `^## ` |
| `.gitignore` | does not ignore `.claude/skills/` | `grep -n claude .gitignore` |

### CLAUDE.md section line ranges (verified by `awk` on `^## `)

These are the exact ranges each task lifts from. They are valid only against
`CLAUDE.md` at commit `0e0f3acd9`. If CLAUDE.md has changed, re-derive with:

```bash
awk '/^## /{if(prev)printf "%s | %d-%d\n", prev, start, NR-1; prev=$0; start=NR} END{printf "%s | %d-%d\n", prev, start, NR}' CLAUDE.md
```

| Section | Lines |
|---|---|
| Content Playtest-Review Gate (SOP) | 3-24 |
| Subagent Model Preference | 25-43 |
| Git Workflow | 44-98 |
| Pre-Push SOP | 99-158 |
| Instance Saves & Smoke-Test SOP | 159-205 |
| Shop Persistence (Living Economy) | 206-219 |
| Moderation Persistence | 220-234 |
| Stat & Progression System | 362-425 |
| Dice & Rolling System | 426-453 |
| Balance Lives in config.yaml, Not in Code | 454-487 |
| Unified Damage & Mitigation Pipeline | 488-561 |
| Resource Depletion Penalties | 562-581 |
| Defense Resolution: Best-of-All | 582-592 |
| Combat Design Conventions | 593-599 |
| ID Inventory & Collision Prevention | 609-643 |
| Data File Naming Convention | 733-741 |
| MUD Line Width | 774-776 |
| Player-Facing Messages, No Hard Numbers | 777-785 |
| Quest Re-Grant Prevention SOP | 786-793 |
| Quest NPC Dialogue SOP | 794-799 |
| Dialogue Voice & Trigger Discoverability | 800-813 |
| Quest Item Delivery, give.go Gotcha | 814-825 |
| Dialogue Engine: givesItem | 826-830 |
| Quest Flags System | 831-875 |
| Content Generation Commands | 936-952 |
| AI Testing | 953-1002 |

**Memory dir path** (referred to below as `$MEM`):
`C:\Users\Calabe Davis\.claude\projects\C--Users-Calabe-Davis-workspace-DOGMud\memory\`

---

## File Structure

Each skill is one directory holding one `SKILL.md`. No skill gets supporting
files in this phase; if one grows past roughly 400 lines, that is a signal it
should split, and the splitting decision belongs to a later phase.

```
.claude/skills/
  dogmud-shipping/SKILL.md
  dogmud-deploying/SKILL.md
  dogmud-authoring-content/SKILL.md
  dogmud-authoring-quests/SKILL.md
  dogmud-player-copy/SKILL.md
  dogmud-writing-tests/SKILL.md
  dogmud-playtesting/SKILL.md
  dogmud-persistence/SKILL.md
  dogmud-refactoring/SKILL.md
  dogmud-combat/SKILL.md
  dogmud-progression-model/SKILL.md
  dogmud-balance-config/SKILL.md
```

## Rules that apply to every task

1. **Lift, do not paraphrase.** Copy the CLAUDE.md text verbatim into the skill,
   then adjust only the framing sentence. Paraphrasing is how a rule quietly
   changes meaning. Phase 2 diffs the skill against CLAUDE.md to prove nothing
   was lost, and that diff only works if the body is verbatim.
2. **Cite memory files, do not copy them.** A folded memory file's *rule* goes in
   the skill in one or two lines. Its dated incident narrative stays in the file
   and is cited as `[[filename-without-extension]]`.
3. **INCIDENT and LOG files are never folded.** Cite them if useful. Do not lift
   their content.
4. **The `description` is the trigger.** It is the only thing read when deciding
   whether to load the skill. Write it as concrete task phrases a person would
   actually be doing, not as a topic label.
5. **Commit after each task.** One skill per commit.
6. **No em dashes or en dashes.** This is a standing project writing rule
   (`[[feedback_no_em_dashes_in_prose]]`). CLAUDE.md itself contains some, so
   replace them with commas, colons, or parentheses while lifting rather than
   copying them through. Verify per skill with:

   ```bash
   grep -c "—\|–" .claude/skills/<name>/SKILL.md
   ```
   Expected: 0.
7. **Wrap prose at roughly 80 characters**, matching the house style of
   CLAUDE.md and the project's player-copy rule.
8. **Audit cross-references after lifting.** A pointer that was true inside
   CLAUDE.md is often false inside a skill that carries only three of its
   sections. Task 1 shipped with "see Shop Persistence below" pointing at
   nothing. After writing each skill, run:

   ```bash
   grep -n "below\|above\|earlier\|see the" .claude/skills/<name>/SKILL.md
   ```

   and confirm every hit resolves inside that document. Repoint the ones that
   do not, naming the skill that now owns the material (forward references to
   a skill later in this plan are fine).
9. **Order sections by what the reader needs first, and put the most-skipped
   rule early.** A skill is read under pressure, mid-task, by someone skimming
   headers. Task 3 originally placed the adversarial playtest gate, the rule
   this project skips most often, at section 6 of 8, behind every
   authoring-mechanics section. A reader looking for "how do I make the file"
   would never reach it. The section lists in the tasks below are a starting
   point, not a constraint: if writing a skill reveals that a later section
   should come first, move it and say so in the report.

   The same failure has a second form: a section that promises a checklist and
   delivers half of one. Task 3's `## Before you create a file` covered ID
   collision but not coordinate collision, which sat four sections away. If a
   section names itself as a pre-flight, it must be complete or point forward
   by exact section title.
10. **Fold only the delta.** A memory file often restates a rule the lifted
    CLAUDE.md block already carries. Folding it whole then duplicates the rule
    inside a document whose entire purpose is to remove duplicated context.
    Task 4 shipped the text-versus-hints voice rule twice, with the same
    example, 25 lines apart.

    Before folding, check whether the lifted block already states the rule. If
    it does, fold only what the memory file adds that the block lacks, usually
    a provenance note or a sharper example, and attach it to the existing
    block. **Never edit the lifted text to resolve a duplication**, because the
    verbatim lift is what lets a later phase prove nothing was lost. Cut the
    folded copy, not the lifted one.
11. **Disambiguate near-identical identifiers.** Where a skill ends up
    describing two mechanisms whose names differ by a word and whose shapes
    differ (Task 4: `questExcluded` takes a token list, `questFlagExcluded`
    takes a key-value map, both gate when a dialogue node fires), say plainly
    what each takes and what each gates. Collecting scattered CLAUDE.md
    sections into one document is what makes the collision visible, so it is
    also where the disambiguation belongs.

---

## Task 0: Create the skills directory and the verification script

**Files:**
- Create: `.claude/skills/.gitkeep`
- Create: `tools/verify_skills.py`

- [ ] **Step 1: Create the directory**

```bash
mkdir -p .claude/skills
touch .claude/skills/.gitkeep
```

- [ ] **Step 2: Write the verification script**

Create `tools/verify_skills.py`. Filename-and-frontmatter parser only, no YAML
library, matching the style of `tools/id_inventory.py`.

```python
#!/usr/bin/env python3
"""Verify every .claude/skills/<name>/SKILL.md is well-formed.

Checks, per skill:
  - the directory contains a SKILL.md
  - SKILL.md opens with a --- frontmatter block
  - frontmatter has both `name` and `description`
  - frontmatter `name` matches the directory name
  - `description` is non-empty and under 500 chars

Exits 1 and prints every failure if any check fails.
"""
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
SKILLS = PROJECT_ROOT / ".claude" / "skills"
EXPECTED = [
    "dogmud-shipping", "dogmud-deploying", "dogmud-authoring-content",
    "dogmud-authoring-quests", "dogmud-player-copy", "dogmud-writing-tests",
    "dogmud-playtesting", "dogmud-persistence", "dogmud-refactoring",
    "dogmud-combat", "dogmud-progression-model", "dogmud-balance-config",
]


def parse_frontmatter(text):
    """Return (dict, None) for a closed --- block, or (None, reason) on failure."""
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return None, "no opening --- line"
    out = {}
    for line in lines[1:]:
        if line.strip() == "---":
            return out, None
        if line.startswith(("  ", "\t")) or ":" not in line:
            continue
        key, _, val = line.partition(":")
        out[key.strip()] = val.strip().strip('"').strip("'")
    return None, "frontmatter block is never closed"


def main():
    errors = []
    if not SKILLS.is_dir():
        print("FAIL: .claude/skills does not exist")
        return 1

    found = sorted(p.name for p in SKILLS.iterdir() if p.is_dir())
    valid = set()
    for name in found:
        path = SKILLS / name / "SKILL.md"
        if not path.is_file():
            errors.append(f"{name}: no SKILL.md")
            continue
        try:
            text = path.read_text(encoding="utf-8-sig")
        except (UnicodeDecodeError, OSError) as exc:
            errors.append(f"{name}: SKILL.md unreadable ({exc.__class__.__name__})")
            continue
        fm, reason = parse_frontmatter(text)
        if fm is None:
            errors.append(f"{name}: {reason}")
            continue
        ok = True
        if fm.get("name") != name:
            errors.append(f"{name}: frontmatter name is {fm.get('name')!r}")
            ok = False
        desc = fm.get("description", "")
        if not desc:
            errors.append(f"{name}: empty description")
            ok = False
        elif len(desc) > 500:
            errors.append(f"{name}: description is {len(desc)} chars, over 500")
            ok = False
        if ok:
            valid.add(name)

    present = [n for n in EXPECTED if n in valid]
    missing = [n for n in EXPECTED if n not in found]
    for name in missing:
        errors.append(f"{name}: expected skill not present")

    unexpected = [n for n in found if n not in EXPECTED]
    for name in unexpected:
        errors.append(f"{name}: unexpected directory, not one of the twelve")

    for e in errors:
        print("FAIL:", e)
    print(f"{len(present)}/{len(EXPECTED)} skills present, {len(errors)} errors")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
```

The numerator counts **expected skills that are present**, not directories that
exist. An earlier draft printed `len(found)`, which meant one stray directory
made the script report `1/12 skills present` while also listing all twelve as
absent. Since every task below verifies its work by reading that line, a
contaminated numerator would break the signal exactly where it is relied on. The
`unexpected directory` check exists for the same reason: a directory whose name
is a typo of a real skill would otherwise be invisible, showing up only as the
correctly-spelled skill being absent.

- [ ] **Step 3: Run it and verify it fails for the right reason**

Run: `python tools/verify_skills.py`
Expected: 12 lines of `FAIL: <name>: expected skill not present`, then
`0/12 skills present, 12 errors`, exit code 1.

This is the null-probe check. The script must be seen failing before any skill
exists, or a later green run proves nothing. Confirm the exit code:

```bash
python tools/verify_skills.py; echo "exit=$?"
```
Expected: `exit=1`

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/.gitkeep tools/verify_skills.py
git commit -m "chore: skills dir and a well-formedness check for it"
```

---

## Task 1: dogmud-shipping

**Files:**
- Create: `.claude/skills/dogmud-shipping/SKILL.md`

**Lift from CLAUDE.md:** lines 44-98 (Git Workflow), 99-158 (Pre-Push SOP),
159-205 (Instance Saves & Smoke-Test SOP).

**Fold these memory files** (rule: state each one's rule in 1-2 lines, cite the file):

| File | Rule to state |
|---|---|
| `feedback_gh_defaults_to_upstream_fork_parent` | every `gh` command carries `--repo pruuk/DOGMud` |
| `feedback-gh-defaults-to-upstream-fork-parent` | three ways gh/git leak work onto upstream |
| `feedback_dev_branch_local_only` | only master goes to origin; `development` stays local |
| `feedback_merge_to_master_means_shipped` | merging to master ships; a backlog gate cannot hold it |
| `feedback_no_instance_saves_to_prod` | never commit runtime instance-save directories |
| `feedback_go_run_dot_not_main` | boot with `go run .`, never `go run main.go` |
| `feedback_master_is_main_branch` | master is the integration and production branch |
| `feedback-gh-pr-checks-can-return-early` | `gh pr checks --watch` can return green before slow jobs register |
| `feedback-git-checkout-pathspec-stages-the-revert` | `git checkout <ref> -- <path>` stages the reversion silently |
| `feedback-git-stash-pathspec-noop-then-pop-hits-another-stash` | a no-op stash push then pop hits an unrelated stash |
| `reference_boot_test_in_isolated_worktree` | the detached-worktree boot recipe |
| `reference_gh_cli_now_installed` | gh is installed and authed as pruuk |
| `reference-lint-gate-inverts-on-large-prs` | CI lint goes red on any PR over 300 files |

**Cite but do not fold** (INCIDENT): `reference_reusable_workflow_permissions`,
`reference_advertising_listings_kit`.

- [ ] **Step 1: Note the duplicate-file finding**

`feedback_gh_defaults_to_upstream_fork_parent.md` (underscores) and
`feedback-gh-defaults-to-upstream-fork-parent.md` (hyphens) both exist and cover
the same rule. Do not silently pick one. Read both, fold the union of their
rules, and record in the skill body that two files cover this. Resolving the
duplicate is Phase 3 tombstone work, not this task.

```bash
ls "/c/Users/Calabe Davis/.claude/projects/C--Users-Calabe-Davis-workspace-DOGMud/memory/" | grep -i "gh.defaults"
```
Expected: exactly two filenames, one underscored and one hyphenated.

- [ ] **Step 2: Write the skill**

Create `.claude/skills/dogmud-shipping/SKILL.md` with exactly this frontmatter:

```markdown
---
name: dogmud-shipping
description: Use when committing, pushing, opening or merging a PR, running pre-push checks, or booting the server to verify a change. Covers the gh --repo pinning that stops PRs landing on the upstream fork parent, the pre-push gate order, the detached-worktree boot check, and the instance-save wipe before a smoke test.
---
```

Body sections, in this order:

1. `## The upstream hazard` - lines 44-98 of CLAUDE.md verbatim, plus the two
   `gh-defaults` memory rules. This goes first because it is the most expensive
   mistake in the file.
2. `## Pre-push gate order` - lines 99-158 verbatim.
3. `## Boot check in an isolated worktree` - the boot recipe only (the shell
   block plus the `boot-check.exe` and exit-124 rationale, which is item 5 of
   the Pre-Push SOP), plus `reference_boot_test_in_isolated_worktree`. Note the
   line range 128-158 runs past the boot recipe into items 6 and 7 (push and
   open the PR, delete the stray tag); those belong under section 2, not here.
4. `## Instance saves before a smoke test` - lines 159-205 verbatim.
5. `## Traps that exit 0 while doing the wrong thing` - the four git memory
   rules (checkout pathspec, stash pathspec, `gh pr checks` early return, lint
   gate on large PRs).
6. `## Sources` - a list of every memory file folded, as `[[links]]`.

- [ ] **Step 3: Verify**

Run: `python tools/verify_skills.py`
Expected: `1/12 skills present, 11 errors` and every error is
`expected skill not present`. No error naming `dogmud-shipping`.

Verify the upstream hazard survived the lift, since it is the reason this skill
exists:

```bash
grep -c "pruuk/DOGMud" .claude/skills/dogmud-shipping/SKILL.md
```
Expected: a count of at least 4.

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/dogmud-shipping/SKILL.md
git commit -m "docs(skills): dogmud-shipping"
```

---

## Task 2: dogmud-deploying

**Files:**
- Create: `.claude/skills/dogmud-deploying/SKILL.md`

**Lift from CLAUDE.md:** nothing. This skill has no CLAUDE.md source; all of its
material is in memory files and `docs/guides/DEPLOYMENT_GUIDE.md`.

**Fold:** `feedback_motd_format`, `feedback-skip-worktree-config-leak`,
`reference_docker_build_times`, `reference_motd_location`.

**Cite but do not fold** (INCIDENT/LOG): `reference-droplet-build-cache-and-dockerfile`,
`reference-droplet-root-owned-files-block-git-writes`,
`reference-prod-trustedproxies-caddy-container`, `reference_hotswap_upstream_prs_638_639`,
`reference_prod_perf_baseline`.

**Do not fold** `feedback-owner-does-the-deploys`: it is PREFERENCE and belongs
in the CLAUDE.md working-style block in Phase 2. State it in this skill as a
one-line boundary ("the owner runs deploys; this skill is for preparing one, not
performing one") and cite the file.

- [ ] **Step 1: Read the existing guide before writing**

Run: `wc -l docs/guides/DEPLOYMENT_GUIDE.md && head -40 docs/guides/DEPLOYMENT_GUIDE.md`

If the guide already covers the deploy ritual accurately, the skill points at it
rather than restating it. Record in the skill which sections of the guide are
authoritative and which are stale. Do not duplicate an accurate guide.

**Resolved during execution (2026-09-08):** the guide is 1,012 lines, current,
and contradicted by none of the memory files. Its `TrustedProxies` and
Caddy-as-container section already carries the fix from
`reference-prod-trustedproxies-caddy-container` (the guide records its own
2026-08-21 correction). So the skill defers to the guide for the deploy
lifecycle and carries only what the guide lacks: the 1 CPU / 961 MB operating
ceiling, the verify-the-SHA-before-trusting-a-fast-rebuild step, the
build-cache-growth and root-owned-files failure modes, the MOTD convention, and
the `config.yaml` skip-worktree hazard. This is why this skill is 143 lines
against `dogmud-shipping`'s 313; the difference is deference, not thinness.

- [ ] **Step 2: Write the skill**

```markdown
---
name: dogmud-deploying
description: Use when preparing or reasoning about a deploy to the DOGMud production droplet, or when diagnosing a deploy that behaved oddly. Covers the 1 CPU / 961 MB limits, the unbounded Docker build cache, verifying the checkout SHA before building, root-owned files blocking git writes, and the Caddy container's TrustedProxies range. The owner performs deploys, not Claude.
---
```

Body sections:

1. `## The owner performs deploys` - the boundary, first, so this is not read as
   permission to deploy.
2. `## Droplet limits` - 1 CPU, 961 MB, the disk pressure that forces
   `Logging.LogToFile: false`.
3. `## Verify the checkout before building` - a suspiciously fast rebuild means
   the checkout did not land; compare full SHAs.
4. `## Known deploy failures` - build cache growth, root-owned files, Caddy
   `TrustedProxies` needing `172.18.0.0/16`.
5. `## MOTD` - from `feedback_motd_format` and `reference_motd_location`.
6. `## config.yaml skip-worktree` - from `feedback-skip-worktree-config-leak`.
7. `## Build time baseline` - from `reference_docker_build_times`, attributed
   carefully (see the trap below).
8. `## The guide` - what `docs/guides/DEPLOYMENT_GUIDE.md` authoritatively
   covers, and anything this skill supersedes.
9. `## Sources` - files folded as `[[links]]`, then a separate list for files
   whose operational fix is stated here but whose dated narrative stays put.

**Corrected during execution (2026-09-08).** This list originally named six
sections and had no home for two of the four files it told the implementer to
fold, so the implementer correctly added sections 6 and 7. The error was in this
plan, not in the work.

**Attribution trap found in review.** The first draft asserted that deploy time
holds near 135 to 145 seconds regardless of diff size and cited
`reference_docker_build_times` for it. That file says the opposite: rebuilds ran
roughly 130 to 160 seconds before the 2026-04-18 cleanup and dropped to roughly
100 seconds after the codebase shrank, so its point is that tree size DOES move
the number. The "regardless of diff size" conclusion belongs to
`reference_prod_perf_baseline`, a cite-only running log. The two sources are in
genuine tension and the skill must not adjudicate. Attribute every claim to the
file that actually makes it.

- [ ] **Step 3: Verify**

Run: `python tools/verify_skills.py`
Expected: `2/12 skills present, 10 errors`.

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/dogmud-deploying/SKILL.md
git commit -m "docs(skills): dogmud-deploying"
```

---

## Task 3: dogmud-authoring-content

**Files:**
- Create: `.claude/skills/dogmud-authoring-content/SKILL.md`

**Lift from CLAUDE.md:** lines 3-24 (Content Playtest-Review Gate),
609-643 (ID Inventory), 733-741 (Data File Naming Convention),
936-952 (Content Generation Commands).

**Fold:** `feedback_filename_must_match_name_field`, `feedback_yaml_colon_gotcha`,
`feedback_cardinal_exits_only`, `feedback_noun_keys_space_separated`,
`feedback_verify_ids_before_creating`, `feedback_zone_coord_planning`,
`feedback_no_name_recycling_no_js`, `feedback-room-cartesian-consistency`,
`feedback_ansi_plural_inside_tag`, `reference_world_coordinate_frame_crawl`,
`reference_room_coordinate_and_reciprocity_gotchas`.

**Note:** `feedback_dialogue_filename_convention` was bucketed here by the survey
but is dialogue material. It folds into `dogmud-authoring-quests` (Task 4), not
here. Do not fold it twice.

- [ ] **Step 1: Write the skill**

```markdown
---
name: dogmud-authoring-content
description: Use when creating or editing world YAML - rooms, mobs, items, zones. Covers the filename-must-match-name-field panic, the unquoted-colon gotcha, cardinal-exit and noun-key conventions, running tools/id_inventory.py before picking an ID, world coordinate consistency, and the mandatory adversarial playtest gate that every content plan ends with.
---
```

Body sections:

1. `## Before you create a file` - run `python tools/id_inventory.py`; the
   parallel-agent ID block allocation from lines 609-643.
2. `## Filenames` - lines 733-741 verbatim plus
   `feedback_filename_must_match_name_field`. A mismatch panics at startup.
3. `## YAML traps` - unquoted colons, bare scalar lists, unexported field tags.
4. `## Room and zone conventions` - cardinal exits, space-separated noun keys,
   coordinate consistency, the crawl-based placement rule.
5. `## Naming and room scripts` - the no-name-recycling and no-JS-room-scripts
   rule. Note `feedback_no_name_recycling_no_js` carries two unrelated rules,
   so the section title must cover both or the JS rule becomes unfindable.
6. `## Slash commands` - lines 936-952 verbatim.
7. `## Sources`

**Corrected during execution (2026-09-08).** This list is superseded by the
order actually shipped. Review found the adversarial playtest gate, the rule
this project skips most often, sitting at section 6 of 8 behind every
authoring-mechanics section, where a reader skimming for "how do I make the
file" would never reach it. `## The playtest gate` (lines 3-24 verbatim,
boot-clean never verifies the experience) was moved to section 2, immediately
after the pre-flight. Section 1 was also completed: it had promised a pre-flight
and delivered only the ID-collision half, with coordinate collision four
sections away. See standing rule 9.

- [ ] **Step 2: Verify**

Run: `python tools/verify_skills.py`
Expected: `3/12 skills present, 9 errors`.

Confirm the playtest gate survived the lift:

```bash
grep -c "adversarial" .claude/skills/dogmud-authoring-content/SKILL.md
```
Expected: at least 2.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/dogmud-authoring-content/SKILL.md
git commit -m "docs(skills): dogmud-authoring-content"
```

---

## Task 4: dogmud-authoring-quests

**Files:**
- Create: `.claude/skills/dogmud-authoring-quests/SKILL.md`

**Lift from CLAUDE.md:** lines 786-793 (Quest Re-Grant Prevention),
794-799 (Quest NPC Dialogue SOP), 800-813 (Dialogue Voice),
814-825 (give.go Gotcha), 826-830 (givesItem), 831-875 (Quest Flags System).

**Fold:** `feedback_dialogue_filename_convention`,
`feedback_dialogue_bare_scalar_list_mutes_npc`, `feedback_hint_voice`,
`feedback_quest_engine_event_names`, `feedback_quest_items_not_components`,
`feedback_loot_placement`, `feedback_room_interact_noun_matching`,
`reference_quest_reward_yaml_key_gotcha`.

- [ ] **Step 1: Write the skill**

```markdown
---
name: dogmud-authoring-quests
description: Use when writing or editing a quest, a dialogue tree, or any NPC text. Covers the two ways a dialogue file silently mutes its NPC, the questExcluded end-token rule that stops a completed quest being re-offered, first-person NPC text versus narrator hints, quest flags and branching-quest gating, and the give.go trap where an item transfers before any handler can refuse it.
---
```

Body sections:

1. `## Two ways to mute an NPC` - the filename convention and the bare-scalar
   list. Both fail silently, which is why they lead.
2. `## Re-grant prevention` - lines 786-793 verbatim.
3. `## Discoverability` - lines 794-799 and 800-813 verbatim. Every trigger word
   must appear somewhere the player can see it.
4. `## Voice` - NPC text is first person, hints are narrator. Never a
   third-person self-reference.
5. `## Items` - lines 814-830 verbatim, plus quest items are never
   `is_component`, plus loot placement.
6. `## Quest flags and branching` - lines 831-875 verbatim.
7. `## Verify event names against the loader` - never guess them.
8. `## Sources`

- [ ] **Step 2: Verify**

Run: `python tools/verify_skills.py`
Expected: `4/12 skills present, 8 errors`.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/dogmud-authoring-quests/SKILL.md
git commit -m "docs(skills): dogmud-authoring-quests"
```

---

## Task 5: dogmud-player-copy

**Files:**
- Create: `.claude/skills/dogmud-player-copy/SKILL.md`

**Lift from CLAUDE.md:** lines 774-776 (MUD Line Width),
777-785 (Player-Facing Messages, No Hard Numbers).

**Fold:** `feedback_esl_clear_language_player_copy`,
`feedback_helpfile_loadout_vs_reaction_advice`.

**Do not fold** `feedback_no_em_dashes_in_prose`: PREFERENCE, goes to the
CLAUDE.md working-style block in Phase 2. Cite it here in one line.

- [ ] **Step 1: Write the skill**

```markdown
---
name: dogmud-player-copy
description: Use when writing any text a player will read - room descriptions, combat and spell messages, help files, patch notes, NPC lines, MOTD. Covers the 80-character hard wrap, the rule against showing raw numbers for damage, healing, armor or durations, ESL-clear phrasing, and framing combat advice as pre-combat loadout rather than in-the-moment reaction.
---
```

Body sections:

1. `## Wrap at 80` - lines 774-776 verbatim.
2. `## Never show a raw number` - lines 777-785 verbatim, including the named
   helpers `combat.GetDamageDescription` and `combat.GetHealDescription` and the
   single exception (the `status` stat sheet).
3. `## ESL-clear` - avoid opaque idioms.
4. `## No em or en dashes` - one line, citing the preference file.
5. `## Framing` - loadout advice, not reaction advice.
6. `## Sources`

- [ ] **Step 2: Verify**

Run: `python tools/verify_skills.py`
Expected: `5/12 skills present, 7 errors`.

Confirm both named helpers survived:

```bash
grep -c "GetDamageDescription\|GetHealDescription" .claude/skills/dogmud-player-copy/SKILL.md
```
Expected: at least 2.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/dogmud-player-copy/SKILL.md
git commit -m "docs(skills): dogmud-player-copy"
```

---

## Task 6: dogmud-writing-tests

**Files:**
- Create: `.claude/skills/dogmud-writing-tests/SKILL.md`

**Lift from CLAUDE.md:** nothing. Points at `docs/guides/TESTING_GUIDE.md`.

**Fold:** `feedback_validator_conditional_checks`,
`feedback_verify_served_not_just_written`,
`feedback_verify_test_server_bound_its_port`,
`feedback_yaml_unexported_field_tag_noop`,
`reference-test-binary-config-defaults-differ-from-shipped`.

Also fold these two rules, which live in MEMORY.md's index rather than in a
topic file. Read them from `MEMORY.md` at the "Feedback / Gotchas" section:

- A test binary's CWD is not reliably the package dir. One shared binary means
  relative paths pass or fail by test order. Anchor on `runtime.Caller`.
- Stat setup in a test: set `.Base` then call `Recalculate()`.
- Progression banners read `SKILL ADVANCEMENT` and `STATISTIC INCREASED`.
  Grepping for `SKILL INCREASED` undercounts.
- Roughly 2.3% of attacks fumble and always miss, which makes naive combat
  assertions flaky.

- [ ] **Step 1: Write the skill**

```markdown
---
name: dogmud-writing-tests
description: Use when writing, fixing, or debugging a Go test in DOGMud, especially one that is flaky or passing for the wrong reason. Covers the fact that test binaries load Go config defaults rather than the shipped config.yaml, the shared-binary CWD trap, the 2.3% attack fumble rate that makes combat assertions flaky, and the rule that a null probe must be proven capable of failing before a green run means anything.
---
```

Body sections:

1. `## A null probe must be proven capable of failing` - first, because it is the
   discipline that catches the rest. Sabotage the code, watch the test go red,
   then restore. When two branches are byte-identical, sabotage by line number
   and confirm the failure names the right line.
2. `## Test binaries never load config.yaml` - they get Go defaults, which differ
   sharply from shipped values.
3. `## CWD is not the package dir` - anchor on `runtime.Caller`.
4. `## Known flake sources` - the 2.3% fumble.
5. `## Fixture setup` - set `.Base` then `Recalculate()`.
6. `## Verify the server actually bound its port` and `## Fetch the rendered
   page, do not trust that it was written`.
7. `## Sources`

- [ ] **Step 2: Verify**

Run: `python tools/verify_skills.py`
Expected: `6/12 skills present, 6 errors`.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/dogmud-writing-tests/SKILL.md
git commit -m "docs(skills): dogmud-writing-tests"
```

---

## Task 7: dogmud-playtesting

**Files:**
- Create: `.claude/skills/dogmud-playtesting/SKILL.md`

**Lift from CLAUDE.md:** lines 953-1002 (AI Testing).

**Fold:** `feedback_content_adversarial_playtest_gate_sop`,
`feedback_defer_tuning_to_post_build_playtest`, `feedback_kill_test_servers`,
`feedback_naive_newbie_playtest`, `feedback_testers_observe_only`,
`feedback_verify_human_experience_not_just_boot`,
`reference_lan_access_local_server`, `reference_verify_ansi_colors_via_telnet_port`,
`reference-playtest-harness-restore`,
`reference-thornwall-shops-sleep-plan-playtests-for-daytime`.

**Cite but do not fold** (INCIDENT): `feedback_never_blanket_kill_the_local_server`
(this becomes hook 4 in Phase 3), `reference-multiline-input-concatenated`.

- [ ] **Step 1: Write the skill**

```markdown
---
name: dogmud-playtesting
description: Use when running the DOGMud playtest harness, writing a goals file, or triaging playtest findings. Covers the external harness location and how to restore it when missing, that local runs need an ephemeral goals file and a --checkout, that a combat fixture must survive several rounds or the run comes back partial, that reports are gitignored so findings must be extracted to memory, and the rule never to kill the user's running server.
---
```

Body sections:

1. `## Never kill the user's server` - first. The incident where a wrong PID
   assumption killed a live server mid-test.
2. `## The harness is external` - `../gomud-playtest-harness`, deleted once,
   check before assuming it is there.
3. `## Local versus prod invocation` - lines 953-1002 verbatim.
4. `## Making a run productive` - a combat fixture must survive rounds; Sable in
   Rift Chamber 5000 with `ask sable arena|oasis <gold>`; gold is the difficulty
   dial; to drain stamina, encumber.
5. `## Harness limits` - the AI port caps 3 commands per round and silently drops
   the overflow after echoing it; `playtestrun stop` exits 0 without tearing the
   container down, so use `docker rm -f`.
6. `## After a run` - reports are gitignored; extract findings to memory or they
   are lost. Testers observe only, they never edit code.
7. `## Sources`

- [ ] **Step 2: Verify**

Run: `python tools/verify_skills.py`
Expected: `7/12 skills present, 5 errors`.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/dogmud-playtesting/SKILL.md
git commit -m "docs(skills): dogmud-playtesting"
```

---

## Task 8: dogmud-persistence

**Files:**
- Create: `.claude/skills/dogmud-persistence/SKILL.md`

**Lift from CLAUDE.md:** lines 206-219 (Shop Persistence) and 220-234
(Moderation Persistence), but only the file-location and do-not-wipe halves. The
dynamic-pricing formula and config knobs from 206-219 belong in
`internal/shops/context.md` and are demoted in Phase 2, not folded here.

**Fold:** `reference_living_state_persistence_contract`,
`reference-alt-characters-break-character-scoped-migrations`.

**Unassigned residue decision required.** The spec leaves two files unplaced.
Decide here and record the decision in the skill body:

- `reference_autosave_lock_cost` - recommend folding into this skill's
  `## Autosave cost` section. It describes how autosave cost tracks activity
  rather than world size, which is persistence behavior.
- `feedback_admin_command_wiring_checklist` - recommend it does NOT come here.
  It is a planning-completeness rule, not a persistence rule. Route it to
  `dogmud-refactoring` in Task 9.

**Do not fold** `reference-user-save-location`: it is a FACT that drives no
procedure. Cite it.

- [ ] **Step 1: Write the skill**

```markdown
---
name: dogmud-persistence
description: Use when adding state that must survive a restart, writing a data migration, or deciding whether a directory is safe to wipe. Covers the four rules of the living-state contract, which directories are living state that must never be wiped by the instance-save cleanup (shops, guilds, moderation) versus which are instance overrides that must be, the instance:"skip" tag exception, and the trap where a character-scoped migration marker re-runs per alt.
---
```

Body sections:

1. `## Living state versus instance override` - the distinction that decides
   whether a wipe is safe. `shops/`, `guilds/`, `moderation/` are living state.
   `mobs.instances/`, `rooms.instances/` are overrides.
2. `## The instance:"skip" exception` - fields tagged `instance:"skip"` are not
   shadowed by a stale save. `Room.SpawnInfo` is in this category. Check the
   struct tag before assuming a field is shadowed.
3. `## The living-state contract` - the four rules, now test-enforced.
4. `## Migrations` - scope the marker to the data, not the character. Alts share
   a bank and re-run a character-scoped marker.
5. `## Autosave cost` - tracks activity, not world size.
6. `## Sources`

- [ ] **Step 2: Verify**

Run: `python tools/verify_skills.py`
Expected: `8/12 skills present, 4 errors`.

Confirm the wipe distinction is unambiguous, since getting it backwards destroys
live economy state:

```bash
grep -c "shops\|guilds\|moderation" .claude/skills/dogmud-persistence/SKILL.md
```
Expected: at least 3.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/dogmud-persistence/SKILL.md
git commit -m "docs(skills): dogmud-persistence"
```

---

## Task 9: dogmud-refactoring

**Files:**
- Create: `.claude/skills/dogmud-refactoring/SKILL.md`

**Lift from CLAUDE.md:** nothing.

**Fold:** `feedback_compiler_is_the_dead_code_sweep`,
`feedback_search_for_existing_infrastructure_first`,
`feedback_shallow_copy_shared_pointers`, `feedback_remove_downed_fully`,
`feedback_admin_command_wiring_checklist` (routed here from Task 8).

Also fold these two rules from the user's global instructions, which are
refactoring discipline rather than DOGMud specifics, and which currently exist
only in `~/.claude/CLAUDE.md`:

- Before inventing a mechanism, grep for how the codebase already does that job.
- Verify negatives too. "I grepped and found nothing" is evidence only if the
  grep could have found something.

**Do not fold** `feedback-dont-file-your-own-inconsistency-as-followup` and
`feedback-fix-flaws-dont-revert-the-work`: both PREFERENCE, both go to the
CLAUDE.md working-style block in Phase 2. Cite them.

- [ ] **Step 1: Write the skill**

```markdown
---
name: dogmud-refactoring
description: Use when modifying, removing, or restructuring code that already works, as opposed to writing something new. Covers deleting the Go field first and letting the compiler enumerate its consumers, grepping for existing infrastructure before building a parallel mechanism, the shallow-copy trap where a struct copy shares pointers and maps with its template, doing a full sweep on removal rather than a partial hack-around, and enumerating every wiring step when adding an admin command.
---
```

Body sections:

1. `## Grep for how the codebase already does this job` - a bespoke variant must
   justify itself against the existing one and usually cannot.
2. `## The compiler is the dead-code sweep` - delete the field, then let the
   build enumerate every consumer. Do not hand-hunt call sites.
3. `## Removal means a full sweep` - a partial hack-around leaves dormant code
   that re-arms later.
4. `## Shallow copies share pointers` - a struct copy shares maps and pointers
   with the template it came from.
5. `## Verify negatives` - confirm a search was capable of succeeding before
   concluding absence.
6. `## Admin command wiring` - the full checklist.
7. `## Sources`

- [ ] **Step 2: Verify**

Run: `python tools/verify_skills.py`
Expected: `9/12 skills present, 3 errors`.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/dogmud-refactoring/SKILL.md
git commit -m "docs(skills): dogmud-refactoring"
```

---

## Task 10: dogmud-combat

**Files:**
- Create: `.claude/skills/dogmud-combat/SKILL.md`

This is the largest skill and the only one with two distinct halves. Keep the
halves clearly separated by `##` headings so a reader can skip the one they do
not need.

**Lift from CLAUDE.md:** lines 426-453 (Dice & Rolling System),
488-561 (Unified Damage & Mitigation Pipeline), 562-581 (Resource Depletion
Penalties), 582-592 (Defense Resolution: Best-of-All), 593-599 (Combat Design
Conventions).

**Fold (model half):** `reference_hit_chance_decoupled_from_mitigation`,
`reference-standing-combat-and-balance-facts`, `reference-taunt-hold-aggro-gate`.

**Fold (placement half):** `feedback_combat_logic_goes_in_handleCombatRound`,
`feedback_btree_combat_events_before_legacy_ai`, `feedback_btree_death_actions`,
`feedback_best_of_actions_are_synchronous`, `feedback_target_resolution_uses_actor`,
`feedback_companion_autonomy`, `feedback_combat_quadrant_parity`.

Note `feedback_remove_downed_fully` was bucketed combat-model by the survey but
folds into `dogmud-refactoring` (Task 9). Do not fold it twice.

- [ ] **Step 1: Write the skill**

```markdown
---
name: dogmud-combat
description: Use when touching damage, defence, hit resolution, opposed contests, or mob combat AI. Two halves. The model half covers the five-factor damage formula, the three channels and their shipped config scales, mitigation caps, best-of-all defence resolution, and that combat.RunContest is the single entry point for every opposed contest. The placement half covers where combat logic goes, that btree events fire before legacy AI, and using actions.ResolveTargetActor rather than reimplementing target resolution.
---
```

Body sections:

1. `## Read config.yaml, never the Go defaults` - first, because every number
   below is a config value and several ship far from their default.
   `SpellDamageScale` ships at 3.12 against a default of 1.0.
2. `## The damage formula` - lines 488-561 verbatim, including the five-factor
   formula and both tables. The fifth factor, `GlobalDamageMultiplier`, was
   missing from the docs until 2026-08-04; keep it explicit.
3. `## Mitigation and resource penalties` - lines 562-581 verbatim.
4. `## Defence` - lines 582-592 verbatim, plus hit chance is decoupled from
   mitigation: armor affects damage only, never the to-hit roll.
5. `## Contests and dice` - lines 426-453 verbatim. `combat.RunContest` is the
   only entry point; the per-channel wrappers were deleted in U6.
   Concentration contests are a separate seam with a smaller floor.
6. `## Design conventions` - lines 593-599 verbatim. Prefer multipliers over flat
   bonuses.
7. `## Where combat code goes` - the placement half. New logic goes in
   `handleCombatRound`. Btree combat events fire before `handleMobAIDecision`.
   `mob_die` handlers use `send_room_text`, not `respond`. `*_best_of` actions
   are synchronous and must not go in `delayedActions`. Use
   `actions.ResolveTargetActor`. Companions stay autonomous.
8. `## Sources`

- [ ] **Step 2: Verify**

Run: `python tools/verify_skills.py`
Expected: `10/12 skills present, 2 errors`.

Confirm the fifth damage factor and the single contest entry point both survived,
since both have been lost from documentation before:

```bash
grep -c "GlobalDamageMultiplier" .claude/skills/dogmud-combat/SKILL.md
grep -c "RunContest" .claude/skills/dogmud-combat/SKILL.md
```
Expected: at least 1 and at least 2 respectively.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/dogmud-combat/SKILL.md
git commit -m "docs(skills): dogmud-combat"
```

---

## Task 11: dogmud-progression-model

**Files:**
- Create: `.claude/skills/dogmud-progression-model/SKILL.md`

**Lift from CLAUDE.md:** lines 362-425 (Stat & Progression System).

**Fold:** `reference-stat-progression-faucet-map`,
`reference-stat-valueadj-includes-training-and-mods`.

- [ ] **Step 1: Write the skill**

```markdown
---
name: dogmud-progression-model
description: Use when touching stats, skills, advancement, or resource pools. Covers that stats center on 100 with no soft cap and no player ceiling at all, that rank is StatInfo.Training and use counters are telemetry only, that the chance expression lives in exactly two functions which nothing may recompute, that a save's stat base is not what the game reads because production uses ValueAdj, and that compression must never be reintroduced because it shrinks every resource pool too.
---
```

Body sections:

1. `## ValueAdj, not base` - first, because reading a save file's `base:` and
   reasoning from it is a recurring mistake. Production uses
   `ValueAdj = Base + Training + Mods`.
2. `## No compression, ever` - lines 371-378 verbatim. It was removed 2026-08-02.
   Anything added to `StatInfo.Recalculate()` hits the resource pools too,
   because `HealthMax`, `StaminaMax`, `ConvictionMax` and `ActionPointsMax` are
   all `stats.StatInfo`.
3. `## No player ceiling` - `IncreaseStat` and `IncreaseSkill` have no bound
   check. The only hard ceilings are mob-only. Beware `MobStatCap` and
   `MobSkillCap`, which are legacy, still validated, and enforce nothing.
4. `## The curve` - lines 400-425 verbatim, including that shipped config differs
   from the Go defaults.
5. `## The two functions` - `Character.ProgressionChanceForStat` and
   `ProgressionChanceForSkill`. Nothing may recompute the chance expression. The
   admin dashboard once hand-rolled it and silently dropped every multiplier.
6. `## Which paths train which stat` - the faucet map.
7. `## Sources`

- [ ] **Step 2: Verify**

Run: `python tools/verify_skills.py`
Expected: `11/12 skills present, 1 error`.

```bash
grep -c "ValueAdj" .claude/skills/dogmud-progression-model/SKILL.md
```
Expected: at least 2.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/dogmud-progression-model/SKILL.md
git commit -m "docs(skills): dogmud-progression-model"
```

---

## Task 12: dogmud-balance-config

**Files:**
- Create: `.claude/skills/dogmud-balance-config/SKILL.md`

**Lift from CLAUDE.md:** lines 454-487 (Balance Lives in config.yaml).

**Fold:** `reference_config_yaml_skip_worktree`.

**Cite but do not fold** (INCIDENT): `reference-clean-hit-rate-is-mislabelled`.

- [ ] **Step 1: Write the skill**

```markdown
---
name: dogmud-balance-config
description: Use before hardcoding any balance number, or when retuning how something feels. Covers that 352 balance knobs are declared in internal/configs/config.balance.go and surfaced through _datafiles/config.yaml, that retuning is a config edit rather than a code change, that a Go default is never a live value because several shipped knobs differ sharply, that an absent key is meaningful because 0 is a legal shipped value, and that config.yaml carries skip-worktree so it desyncs in both directions.
---
```

Body sections:

1. `## Look for the knob before editing a literal` - lines 454-487 verbatim.
2. `## Never quote a Go default as a live value` - read `config.yaml`. Absence is
   meaningful; `0` is legal.
3. `## Where knobs are declared` - all fields in `config.balance.go`; the seven
   sibling files hold only defaulting and validation. Grep the YAML tag, not the
   Go field name.
4. `## config.yaml has skip-worktree` - it desyncs both ways. Build a commit from
   the `git show HEAD:` blob, never from disk. `--cacheinfo` clears the
   skip-worktree bit.
5. `## Sources`

- [ ] **Step 2: Verify**

Run: `python tools/verify_skills.py`
Expected: `12/12 skills present, 0 errors`, exit code 0.

```bash
python tools/verify_skills.py; echo "exit=$?"
```
Expected: `exit=0`

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/dogmud-balance-config/SKILL.md
git commit -m "docs(skills): dogmud-balance-config"
```

---

## Task 13: Coverage check

This task proves Phase 1 is complete enough for Phase 2 to safely delete from
CLAUDE.md. It writes no new skill content.

**Files:**
- Create: `docs/superpowers/audits/2026-09-08-skill-coverage.md`

- [ ] **Step 1: Verify every PROCEDURE file is claimed exactly once**

Build the claim list from the skills, and compare it against the 64 PROCEDURE
files from the spec's survey table.

```bash
grep -roh '\[\[[^]]*\]\]' .claude/skills/ | sort | uniq -c | sort -rn | head -20
```

Read the counts. Every line must start with `1`. A line starting with `2` or more
means one memory file is claimed by two skills, which violates fold-law rule 4.

Expected: no memory file is claimed by two skills. If any is, resolve it now.
The known intentional single-claims are: `feedback_dialogue_filename_convention`
in authoring-quests only, `feedback_remove_downed_fully` in refactoring only,
`feedback_admin_command_wiring_checklist` in refactoring only.

- [ ] **Step 2: Verify every CLAUDE.md line range is covered or explicitly deferred**

```bash
awk '/^## /{if(prev)printf "%s | %d-%d\n", prev, start, NR-1; prev=$0; start=NR} END{printf "%s | %d-%d\n", prev, start, NR}' CLAUDE.md
```

For each of the 44 sections, record one of: lifted into skill X, demoted in
Phase 2 to Y, stays in residual CLAUDE.md, or deleted in Phase 2 because
`docs/schemas/` already covers it. Every section gets a disposition. A section
with no disposition is a gap.

The sections with no Phase 1 task, expected to be dispositioned as demote,
stay, or delete: Subagent Model Preference (25-43), Shop Persistence pricing
half (206-219), Moderation Persistence (220-234), NPC Schedules (235-245),
Sleep Mechanics (246-258), NPC Patrols (259-274), NPC to NPC Conversations
(275-309), Map Consistency (310-354), Project Context (355-361), Regen System
(600-608), Codegraph MCP (644-690), Package context.md Convention (691-732),
Command Parsing (742-773), Equipment Slots (876-899), Spell Duration (900-904),
Buff/Ward Spell System (905-916), Inventory & Item Disambiguation (917-935),
Mob Stat Archetypes (1003-1010), Caster Weapon Types (1011-1025), Alchemy &
Potions (1026-1073), Salvage System (1074-1094).

- [ ] **Step 3: Write the audit**

Create `docs/superpowers/audits/2026-09-08-skill-coverage.md` with two tables:
one row per CLAUDE.md section with its disposition, and one row per PROCEDURE
memory file with the skill that claimed it. Note any gap found.

- [ ] **Step 4: Verify the audit is complete**

```bash
grep -c "^|" docs/superpowers/audits/2026-09-08-skill-coverage.md
```
Expected: at least 108 rows (44 CLAUDE.md sections + 64 PROCEDURE files, plus
header rows).

- [ ] **Step 5: Update docs/README.md**

Add a row for the audit, following the existing table format in
`docs/README.md` around line 77.

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/audits/2026-09-08-skill-coverage.md docs/README.md
git commit -m "docs: skill coverage audit closing phase 1"
```

---

## Definition of done for Phase 1

1. `python tools/verify_skills.py` exits 0 with `12/12 skills present, 0 errors`.
2. The coverage audit exists and shows a disposition for all 44 CLAUDE.md
   sections and all 64 PROCEDURE memory files.
3. No file outside `.claude/skills/`, `tools/verify_skills.py`,
   `docs/superpowers/audits/`, and `docs/README.md` has been modified.
4. `CLAUDE.md` and `MEMORY.md` are byte-identical to their state at commit
   `0e0f3acd9`. Verify:

```bash
git diff 0e0f3acd9 --stat -- CLAUDE.md
```
Expected: no output.

Phase 2 (strip CLAUDE.md, shrink MEMORY.md, apply the thirteen demotions) and
Phase 3 (hooks and tombstones) get their own plans, written after Phase 1 lands.
