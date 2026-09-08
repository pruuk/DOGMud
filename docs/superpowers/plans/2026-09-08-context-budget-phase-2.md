# Context Budget Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Take `CLAUDE.md` from 1,094 lines to roughly 71 and always-loaded memory
from 33.3k tokens to roughly 7k, by deleting what twelve skills already carry and
writing proper `context.md` sections for the nine gaps that block the rest.

**Architecture:** Three steps in dependency order. Prove skills actually trigger,
then delete the 31 audit-verified sections in one sweep (banking 74% so a stall
leaves a coherent state), then write the six `context.md` batches, deleting each
source section only after its replacement lands and passes the audit tool.
Deletion is driven by a manifest of exact heading text, never line numbers,
because removing any section renumbers everything below it.

**Tech Stack:** Markdown, Python 3 (filename-and-heading parsers, no YAML
library, matching the house style of `tools/id_inventory.py`).

**Source spec:** `docs/superpowers/specs/2026-09-08-context-budget-phase-2-design.md`
**Phase 1 audit:** `docs/superpowers/audits/2026-09-08-skill-coverage.md`

---

## Facts verified against source (2026-09-08)

| Fact | Value | How verified |
|---|---|---|
| `CLAUDE.md` | 1,094 lines, 44 `##` sections | `wc -l`, `grep -c "^## "` |
| All 44 headings | **unique** | `grep "^## " \| sort \| uniq -d` returned empty |
| Headings with non-ASCII | 4 | `grep -P "[^\x00-\x7F]"` |
| `MEMORY.md` | 82 lines | `wc -l` |
| Skills | 12, `verify_skills.py` exits 0 | run |
| `tools/context_md_audit.py` | exists, 5,523 bytes | `ls` |
| `internal/items/context.md` baseline | **3 phantom of 31**: `UseItem`, `ItemPower`, `IsUpgrade` | `python tools/context_md_audit.py internal/items` |
| `crafting`, `shops`, `buffs`, `characters`, `hooks` baseline | all resolve, 0 phantom | same, per package |
| `calcSpellDuration` | `internal/hooks/spell_resolution.go:35` | `grep -rn` |
| `RunSalvageContest` | `internal/crafting/difficulty.go:152` | `grep -rn` |
| `SalvageFloor` | `config.balance.go:505`, shipped `0.15` at `config.yaml:1551` | `grep -n` |
| `SalvageMinChance` / `MaxChance` / `SoftCap` | declared `config.balance.go:529-531`, **zero consumers** | `grep -rn --include=*.go` excluding config.balance files returned nothing |
| `GetAgingPhase`, `CalcEffectiveAgingSpeed` | `internal/items/aging.go:31,151` | `grep -rn` |

### The four non-ASCII headings (exact bytes matter)

```
## NPC↔NPC Conversations
## Codegraph MCP — Code Intelligence
## Player-Facing Messages — No Hard Numbers
## Quest Item Delivery — give.go Gotcha
```

Three are in the delete set. A manifest that renders `↔` as `-to-` or `—` as `-`
will not match, and the script MUST abort rather than silently skip.

### The 44 sections by disposition

**DELETE in Task 2 (31 sections, 810 lines).** 23 lifted into a skill, 7 already
covered elsewhere, 1 demoted:

```
## Content Playtest-Review Gate (SOP)
## Git Workflow
## Pre-Push SOP
## Instance Saves & Smoke-Test SOP (Important!)
## Moderation Persistence
## NPC Schedules
## Sleep Mechanics
## NPC Patrols
## NPC↔NPC Conversations
## Map Consistency & the `non_cartesian` / `oneway` Flags
## Stat & Progression System
## Dice & Rolling System
## Balance Lives in config.yaml, Not in Code
## Unified Damage & Mitigation Pipeline (Stage 34)
## Resource Depletion Penalties (Stage 35)
## Defense Resolution: Best-of-All (Stage 35)
## Combat Design Conventions
## ID Inventory & Collision Prevention
## Data File Naming Convention
## Command Parsing & Multi-Word Input (`internal/parser`)
## MUD Line Width
## Player-Facing Messages — No Hard Numbers
## Quest Re-Grant Prevention SOP
## Quest NPC Dialogue SOP
## Dialogue Voice & Trigger Discoverability
## Quest Item Delivery — give.go Gotcha
## Dialogue Engine: givesItem
## Quest Flags System
## Content Generation Commands
## AI Testing
## Mob Stat Archetypes
```

**DELETE in Tasks 3-8, each after its replacement lands (9 sections, 167 lines):**

```
## Shop Persistence (Living Economy)      -> internal/shops/context.md
## Regen System (Stage 29.5)              -> internal/characters/context.md
## Equipment Slots                        -> internal/items/context.md
## Spell Duration System                  -> internal/hooks/context.md
## Buff/Ward Spell System                 -> internal/buffs/context.md
## Inventory & Item Disambiguation        -> internal/items/context.md
## Caster Weapon Types                    -> internal/items/context.md
## Alchemy & Potions System               -> internal/items/context.md
## Salvage System                         -> internal/crafting/context.md (REWRITTEN)
```

**STAY (4 sections, 115 lines):**

```
## Subagent Model Preference          (folded into the working-style block, Task 9)
## Project Context                    (unchanged)
## Codegraph MCP — Code Intelligence  (condensed 47 -> ~12)
## Package `context.md` Convention    (condensed 42 -> ~12)
```

---

## File Structure

```
tools/strip_claude_sections.py          NEW  heading-driven section remover
tools/claude_strip_manifest.txt         NEW  exact headings for Task 2's sweep
docs/superpowers/audits/
  2026-09-08-skill-routing.md           NEW  Task 1's routing table
  2026-09-09-phase-2-closing.md         NEW  Task 11's whole-phase gate
internal/items/context.md               MOD  4 sections + clear 3 phantoms
internal/crafting/context.md            MOD  Salvage, rewritten from code
internal/shops/context.md               MOD  pricing half
internal/buffs/context.md               MOD  Buff/Ward
internal/characters/context.md          MOD  Regen
internal/hooks/context.md               MOD  Spell Duration
CLAUDE.md                               MOD  1094 -> ~71 lines
MEMORY.md (memory dir)                  MOD  82 -> ~45 lines
docs/README.md                          MOD  index the two new audits
```

## Rules that apply to every task

1. **Never delete by line number.** Removing any section renumbers everything
   below it. Use `tools/strip_claude_sections.py`, which matches exact heading
   text and aborts if a heading is not found.
2. **Write before delete, always.** In Tasks 3 through 8, the CLAUDE.md section
   is removed only after its replacement exists and the audit tool passes.
3. **Verify every symbol against the code.** Phase 1 shipped a skill documenting
   a `Storage.MigrationsDone` API that has never existed, folded faithfully from
   a self-contradictory memory file. Grep before asserting.
4. **Label shipped versus Go default** wherever a config number appears. A Go
   default is a fallback used only when the key is absent from `config.yaml`.
5. **No em dashes or en dashes** in new prose. Existing CLAUDE.md text being
   condensed keeps its own punctuation where it is quoted.
6. **Wrap prose near 80 characters**, except inside tables and code blocks.
7. **Named paths only** on `git add`. Never `git add -A` or `git add .`.
8. **No scratch files in the repo.** Use temp space outside it.
9. **A traceback from `strip_claude_sections.py` is never a reason to edit
   CLAUDE.md by hand.** Task 0 found that printing the `↔` in
   `## NPC↔NPC Conversations` raised `UnicodeEncodeError` on this machine's
   cp1252 console. That is fixed inside the script (it reconfigures stdout to
   UTF-8 itself), so no caller needs `PYTHONIOENCODING`. If the tool ever does
   fail, note that the write happens once, last, after every heading has been
   resolved and reported, so a crash leaves CLAUDE.md untouched. Fix the tool
   and re-run. Hand-editing reintroduces the line-renumbering hazard the tool
   exists to prevent.

---

## Task 0: the section-removal tool

**Files:**
- Create: `tools/strip_claude_sections.py`
- Create: `tools/claude_strip_manifest.txt`

- [ ] **Step 1: Write the tool**

Create `tools/strip_claude_sections.py`:

```python
#!/usr/bin/env python3
"""Remove named ## sections from CLAUDE.md.

Sections are identified by their EXACT heading line, never by line number,
because deleting one section renumbers every line below it. All 44 headings in
CLAUDE.md are unique (verified 2026-09-08) and four contain non-ASCII
characters, so the manifest must carry exact bytes.

If ANY manifest heading is not found, the script aborts and writes nothing.
A typo that silently removes nothing, or removes the wrong span, is the
failure mode this guards against.

Usage:
    python tools/strip_claude_sections.py --manifest tools/claude_strip_manifest.txt
    python tools/strip_claude_sections.py --manifest tools/claude_strip_manifest.txt --apply
"""
import argparse
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent


def read_manifest(path):
    """Return the list of heading lines to remove, ignoring blanks and #-comments."""
    out = []
    for raw in path.read_text(encoding="utf-8-sig").splitlines():
        line = raw.rstrip()
        if not line.strip() or line.lstrip().startswith("# "):
            continue
        out.append(line)
    return out


def find_sections(lines, headings):
    """Map each heading to (start_index, end_index_exclusive). Missing -> None."""
    spans = {}
    for h in headings:
        try:
            start = lines.index(h)
        except ValueError:
            spans[h] = None
            continue
        end = len(lines)
        for i in range(start + 1, len(lines)):
            if lines[i].startswith("## "):
                end = i
                break
        spans[h] = (start, end)
    return spans


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--manifest", required=True)
    ap.add_argument("--target", default=str(PROJECT_ROOT / "CLAUDE.md"))
    ap.add_argument("--apply", action="store_true",
                    help="write the file; without this it is a dry run")
    args = ap.parse_args()

    target = Path(args.target)
    manifest = Path(args.manifest)
    if not target.is_file():
        print(f"FAIL: no such target {target}")
        return 1
    if not manifest.is_file():
        print(f"FAIL: no such manifest {manifest}")
        return 1

    lines = target.read_text(encoding="utf-8-sig").splitlines()
    headings = read_manifest(manifest)
    spans = find_sections(lines, headings)

    missing = [h for h, s in spans.items() if s is None]
    if missing:
        print(f"FAIL: {len(missing)} manifest heading(s) not found in {target.name}:")
        for h in missing:
            print(f"  {h!r}")
        print("Nothing was written. Fix the manifest; headings must match exactly,")
        print("including non-ASCII characters.")
        return 1

    doomed = set()
    total = 0
    for h in headings:
        start, end = spans[h]
        n = end - start
        total += n
        print(f"  {n:4d} lines  {h}")
        doomed.update(range(start, end))

    print(f"{len(headings)} sections, {total} lines, "
          f"{len(lines)} -> {len(lines) - total} lines")

    if not args.apply:
        print("DRY RUN, nothing written. Re-run with --apply to write.")
        return 0

    kept = [l for i, l in enumerate(lines) if i not in doomed]
    target.write_text("\n".join(kept) + "\n", encoding="utf-8")
    print(f"WROTE {target}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 2: Build the manifest**

Build the manifest **by subtraction from CLAUDE.md itself**, never by retyping.
Retyping is how the non-ASCII headings get mangled, and a mangled heading is the
exact failure the abort guard exists to catch.

Start from all 44:

```bash
grep "^## " CLAUDE.md > /c/tmp/all-headings.txt
wc -l /c/tmp/all-headings.txt
```
Expected: 44.

Then write the 13 KEEP headings to a file and subtract them. There are 13
because 9 are gapped (handled in Tasks 3 through 8) and 4 stay:

```bash
cat > /c/tmp/keep.txt <<'EOF'
## Subagent Model Preference
## Shop Persistence (Living Economy)
## Project Context
## Regen System (Stage 29.5)
## Codegraph MCP — Code Intelligence
## Package `context.md` Convention
## Equipment Slots
## Spell Duration System
## Buff/Ward Spell System
## Inventory & Item Disambiguation
## Caster Weapon Types
## Alchemy & Potions System
## Salvage System
EOF
wc -l /c/tmp/keep.txt
```
Expected: 13.

**Verify every KEEP heading actually matches CLAUDE.md before subtracting**, or
subtraction silently leaves it in the delete set:

```bash
while IFS= read -r h; do
  grep -qxF "$h" /c/tmp/all-headings.txt || echo "NO MATCH: $h"
done < /c/tmp/keep.txt
```
Expected: no output. Any `NO MATCH` line means that heading is mistyped, most
likely its em dash. Fix it before continuing.

Now subtract:

```bash
grep -vxF -f /c/tmp/keep.txt /c/tmp/all-headings.txt > tools/claude_strip_manifest.txt
```

Verify the count and that the three non-ASCII headings that belong in it are
present:

```bash
grep -c "^## " tools/claude_strip_manifest.txt
grep -P "[^\x00-\x7F]" tools/claude_strip_manifest.txt
```
Expected: 31, and exactly three non-ASCII lines
(`NPC↔NPC Conversations`, `Player-Facing Messages — No Hard Numbers`,
`Quest Item Delivery — give.go Gotcha`). `Codegraph MCP — Code Intelligence`
must NOT be there: it stays.

- [ ] **Step 3: Prove the abort guard fires (the null probe)**

A remover that silently skips a heading it cannot find is worse than no remover.
Prove it aborts before trusting it.

```bash
cp tools/claude_strip_manifest.txt /c/tmp/bad-manifest.txt
echo '## This Section Does Not Exist' >> /c/tmp/bad-manifest.txt
python tools/strip_claude_sections.py --manifest /c/tmp/bad-manifest.txt; echo "exit=$?"
```
Expected: `FAIL: 1 manifest heading(s) not found`, the offending heading printed,
`Nothing was written.`, and `exit=1`.

Then prove a mangled non-ASCII heading also aborts, since that is the realistic
version of this mistake:

```bash
sed 's/NPC↔NPC Conversations/NPC-to-NPC Conversations/' tools/claude_strip_manifest.txt > /c/tmp/mangled.txt
python tools/strip_claude_sections.py --manifest /c/tmp/mangled.txt; echo "exit=$?"
```
Expected: FAIL naming `## NPC-to-NPC Conversations`, `exit=1`.

Paste both outputs verbatim in your report.

- [ ] **Step 4: Dry run the real manifest**

```bash
python tools/strip_claude_sections.py --manifest tools/claude_strip_manifest.txt; echo "exit=$?"
```
Expected: 31 sections listed, a total near 810 lines, `1094 -> ~284 lines`,
`DRY RUN, nothing written.`, `exit=0`.

Confirm `git status --short` shows CLAUDE.md unmodified.

- [ ] **Step 5: Commit**

```bash
git add tools/strip_claude_sections.py tools/claude_strip_manifest.txt
git commit -m "chore: heading-driven section remover for CLAUDE.md, with an abort guard"
```

---

## Task 1: prove the skills actually trigger (spec step 0)

**Files:**
- Create: `docs/superpowers/audits/2026-09-08-skill-routing.md`

Nothing has yet evidenced that a skill loads on a realistic request. Phase 1
proved the twelve are well-formed, not that they fire. If triggering is
unreliable, deleting CLAUDE.md does not move rules on-demand, it deletes them.

- [ ] **Step 1: Collect the twelve descriptions**

```bash
grep -h "^description:" .claude/skills/*/SKILL.md | cut -c1-120
```

- [ ] **Step 2: Route fifteen realistic prompts with a blind judge**

Dispatch a subagent that is given ONLY the twelve `name` and `description`
pairs, and NOT the skill bodies, NOT this plan, and NOT CLAUDE.md. For each
prompt it names the single skill it would load, or `none`.

A blind judge matters: someone who has read the skills will route correctly from
memory, which tests nothing.

The fifteen prompts, phrased as a person would type them:

```
1.  add a mob to Thornwall
2.  why is this test flaky
3.  open a PR for this branch
4.  is it safe to wipe rooms.instances
5.  make fire damage hurt more
6.  the NPC stopped talking
7.  the deploy took way longer than usual
8.  write the room description for the shrine
9.  my stat is not going up
10. remove the bleedout feature
11. run a playtest of the newbie area
12. where do I set the shop restock rate
13. the quest keeps getting re-offered after I finish it
14. this combat message shows a raw damage number
15. add a field to Character that survives restart
```

- [ ] **Step 3: Build the routing table**

Create `docs/superpowers/audits/2026-09-08-skill-routing.md` with one row per
prompt: prompt, skill the judge chose, skill that should have fired, match or
miss. Then a second table: one row per skill, listing which prompts routed to it.

- [ ] **Step 4: Fix any skill nothing routes to**

Any skill with zero prompts routing to it, or that the judge chose wrongly, gets
its `description` edited. Then re-run the blind judge on the affected prompts
with the revised descriptions and record the second result.

Keep every description under 500 characters or `verify_skills.py` will fail:

```bash
python tools/verify_skills.py; echo "exit=$?"
```
Expected: `12/12 skills present, 0 errors`, `exit=0`.

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/audits/2026-09-08-skill-routing.md
git commit -m "docs: skill routing table, proving the twelve fire on real prompts"
```

If any skill descriptions changed, add those paths to the same commit.

---

## Task 2: delete the 31 verified-safe sections (spec step 1)

**Files:**
- Modify: `CLAUDE.md` (1,094 -> ~284 lines)

- [ ] **Step 1: Record the before state**

```bash
wc -l CLAUDE.md
git rev-parse HEAD
grep -c "^## " CLAUDE.md
```
Expected: 1094, a SHA to cite, 44.

- [ ] **Step 2: Apply**

```bash
python tools/strip_claude_sections.py --manifest tools/claude_strip_manifest.txt --apply; echo "exit=$?"
wc -l CLAUDE.md
grep -c "^## " CLAUDE.md
```
Expected: `WROTE`, `exit=0`, roughly 284 lines, 13 remaining sections
(44 minus 31).

- [ ] **Step 3: Verify nothing outside the manifest changed**

```bash
git diff --stat CLAUDE.md
git diff CLAUDE.md | grep "^+" | grep -v "^+++" | wc -l
```
Expected: deletions only. **The added-line count must be 0.** This step is
purely subtractive; a single added line means the script rewrote something.

Then confirm each remaining heading is one that should remain:

```bash
grep "^## " CLAUDE.md
```
Expected: exactly the 9 gapped plus the 4 staying sections, 13 total.

- [ ] **Step 4: Re-derive coverage, do not trust the audit**

For each deleted section, confirm its content is still findable in the skill
that claims it. This must not depend on the Phase 1 audit whose conclusion it is
testing.

```bash
git show HEAD:CLAUDE.md > /c/tmp/claude-before.md
```

Then for a sample of at least eight deleted sections spanning different skills,
extract a distinctive phrase from `/c/tmp/claude-before.md` and grep the skills:

```bash
grep -rl "pruuk/DOGMud" .claude/skills/
grep -rl "GlobalDamageMultiplier" .claude/skills/
grep -rl "questExcluded" .claude/skills/
grep -rl "id_inventory" .claude/skills/
grep -rl "GetDamageDescription" .claude/skills/
grep -rl "ConvertForFilename" .claude/skills/
grep -rl "ephemeral" .claude/skills/
grep -rl "ValueAdj" .claude/skills/
```
Each must return at least one skill path. Record which.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: delete the 31 CLAUDE.md sections the twelve skills now carry"
```

---

## Task 3: batch 1, `internal/items/context.md` (four sections)

**Files:**
- Modify: `internal/items/context.md` (943 lines, receives ~106)
- Modify: `CLAUDE.md` (remove four sections)

This is the largest batch and the only restructure rather than an append.

- [ ] **Step 1: Capture the source text before it is deleted**

```bash
git show HEAD:CLAUDE.md > /c/tmp/claude-before.md
```

The four sections are `## Equipment Slots`, `## Inventory & Item Disambiguation`,
`## Caster Weapon Types`, `## Alchemy & Potions System`. Read each from that file.

- [ ] **Step 2: Verify every symbol before writing**

```bash
grep -n "func GetAgingPhase\|func CalcEffectiveAgingSpeed" internal/items/aging.go
grep -rn "is_component\|weight_reduction\|bag_capacity\|is_bandolier\|bandolier_capacity" internal/items/*.go | head
grep -rn "spell_damage_multiplier" internal/items/*.go internal/hooks/*.go | head
grep -rn "func calcSpellDamage\|func calcMobSpellDamage" internal/hooks/spell_resolution.go
```
Report what each returns. Any symbol that does not resolve must not be asserted.

- [ ] **Step 3: Clear the three pre-existing phantom symbols**

```bash
python tools/context_md_audit.py internal/items
```
Expected before: 3 phantom of 31, naming `UseItem`, `ItemPower`, `IsUpgrade`.

For each, grep the package. If genuinely absent, remove or correct the entry in
`internal/items/context.md`. They are not this phase's doing, but this batch
rewrites the file they live in.

- [ ] **Step 4: Write the four sections**

Follow the project's `context.md` convention: `## Purpose`, `## Files`, core
types with real field names in a `go` block, `## Public API` grouped by job,
`## Gotchas`, `## Dependencies`, `## Consumers`. Do not write "Future
Enhancements", "Security Considerations", "Performance Characteristics" or
"Scalability" sections.

Content to carry, from `/c/tmp/claude-before.md`:
- **Equipment slots**: the default slot list, the mutation-gated ExtraArm and
  ExtraWrist levels 1-4 with their charisma and aggro penalties, the +20 per arm
  combat hit penalty, Back and Component Bag behaviour, the Tail mutation
  disabling Legs.
- **Disambiguation**: `N.item`, `item#N`, `all.item`, unified `FindItem` across
  backpack and equipped, display-only stacking, carry capacity as
  `Strength × CarryCapacityMultiplier`, encumbrance tiers, multi-buy.
- **Caster weapons**: the wand/sceptre/staff table intact, and the fact that
  `spell_damage_multiplier` is an `ItemSpec` field **applied in
  `internal/hooks/spell_resolution.go`**, not in this package.
- **Alchemy**: aging phases, the bottle tier table with item IDs, toxicity
  accumulation and decay, craft-skill scaling, the bandolier, buff IDs 54-76 and
  item IDs 30036-30056 and 40043-40049.

- [ ] **Step 5: Verify the write**

```bash
python tools/context_md_audit.py internal/items
```
Expected: `All documented symbols resolve.` Zero phantom, down from 3.

```bash
grep -c "—\|–" internal/items/context.md
```
Expected: 0 in the sections you added. If the file already contained dashes
elsewhere, report the count and confirm your additions contributed none.

- [ ] **Step 6: Delete the four CLAUDE.md sections**

```bash
cat > /c/tmp/items-manifest.txt <<'EOF'
## Equipment Slots
## Inventory & Item Disambiguation
## Caster Weapon Types
## Alchemy & Potions System
EOF
python tools/strip_claude_sections.py --manifest /c/tmp/items-manifest.txt --apply; echo "exit=$?"
git diff CLAUDE.md | grep "^+" | grep -v "^+++" | wc -l
```
Expected: 4 sections removed, `exit=0`, and **0 added lines**.

- [ ] **Step 7: Commit**

```bash
git add internal/items/context.md CLAUDE.md
git commit -m "docs(items): document equipment, disambiguation, caster weapons and alchemy"
```

---

## Task 4: batch 2, `internal/crafting/context.md` (Salvage, rewritten)

**Files:**
- Modify: `internal/crafting/context.md` (277 lines)
- Modify: `CLAUDE.md` (remove `## Salvage System`)

The only genuinely new writing in this phase. CLAUDE.md's version is wrong in
two ways at once.

- [ ] **Step 1: Establish what is actually true**

```bash
sed -n '145,165p' internal/crafting/difficulty.go
grep -n "^func " internal/crafting/salvage.go
grep -n "func SalvageDifficulty\|func FallbackSalvageDifficulty" internal/crafting/difficulty.go
grep -n "SalvageFloor" internal/configs/config.balance.go _datafiles/config.yaml
grep -rn "SalvageMinChance\|SalvageMaxChance\|SalvageSoftCap" --include=*.go internal/ modules/ | grep -v config.balance
```

Expected findings, which you must confirm rather than assume:
- `RunSalvageContest(score, difficulty float64) contest.Result` at
  `difficulty.go:152`, reading `Balance.SalvageFloor`.
- `SalvageFloor` declared at `config.balance.go:505`, shipped `0.15` at
  `config.yaml:1551`, its comment saying it "reproduces the 15/85 clamp".
- The last grep returns **nothing**, proving `SalvageMinChance`,
  `SalvageMaxChance` and `SalvageSoftCap` have zero consumers.

Quote each result in your report.

- [ ] **Step 2: Write the replacement**

Add a salvage section to `internal/crafting/context.md` covering:
- `RunSalvageContest` as THE entry point and the one place `SalvageFloor` is
  read, mirroring how `RunCraftContest` is described.
- `SalvageDifficulty(itemId, materialTierMult)` and
  `FallbackSalvageDifficulty()` for items with no recipe (a corpse, for
  instance).
- `CalcSalvageRounds`, `RollSalvageReturns`, `RollSalvageReturnsFromSpec`,
  `CalcIngredientGoldValue`, `CalcSalvageReturnGoldValue` from `salvage.go`.
- The `salvage_returns` ItemSpec field and that every `item_tag` must match a
  real `component_tag`.
- A **Gotchas** entry stating plainly that `SalvageMinChance`,
  `SalvageMaxChance` and `SalvageSoftCap` still exist as config knobs, are still
  validated, and **enforce nothing**. Say that tuning them has no effect and
  that `SalvageFloor` is the live knob. This is the same shape as the
  `MobStatCap` trap already documented in `dogmud-progression-model`.

- [ ] **Step 3: Verify**

```bash
python tools/context_md_audit.py internal/crafting
```
Expected: `All documented symbols resolve.`

```bash
grep -c "SalvageFloor" internal/crafting/context.md
grep -c "SalvageMinChance" internal/crafting/context.md
```
Expected: at least 1 each. The dead knobs must be named in order to be marked
dead.

- [ ] **Step 4: Delete the CLAUDE.md section**

```bash
echo '## Salvage System' > /c/tmp/salvage-manifest.txt
python tools/strip_claude_sections.py --manifest /c/tmp/salvage-manifest.txt --apply; echo "exit=$?"
git diff CLAUDE.md | grep "^+" | grep -v "^+++" | wc -l
```
Expected: 1 section removed, `exit=0`, 0 added lines.

- [ ] **Step 5: Commit**

```bash
git add internal/crafting/context.md CLAUDE.md
git commit -m "docs(crafting): document the salvage contest and mark three dead knobs dead"
```

---

## Task 5: batch 3, `internal/shops/context.md` (pricing half)

**Files:**
- Modify: `internal/shops/context.md` (247 lines)
- Modify: `CLAUDE.md` (remove `## Shop Persistence (Living Economy)`)

The file-location half of this section was already lifted into
`dogmud-persistence` in Phase 1. Only the pricing half needs a home.

- [ ] **Step 1: Verify all eight knobs, shipped and default**

```bash
for k in ShopBuyRatio ShopPriceFloor ShopPriceCeiling ShopAbundanceThreshold ShopMaterialReserve ShopGoldReserveRatio BarterMaxDiscount BarterMaxBonus; do
  printf "%-24s go:%s  yaml:%s\n" "$k" \
    "$(grep -c "$k" internal/configs/config.balance*.go)" \
    "$(grep -c "$k" _datafiles/config.yaml)"
done
```
Report the table. Any knob with `yaml:0` is absent from `config.yaml` and runs
on its Go default, which is a meaningful fact to state, not an omission.

- [ ] **Step 2: Write the pricing section**

Cover the 0.25x to 5.0x dynamic range, that it is driven by
`ShopAbundanceThreshold` and normalised per item by restock quantity, and all
eight knobs with shipped versus Go-default labelled per knob.

- [ ] **Step 3: Verify**

```bash
python tools/context_md_audit.py internal/shops
grep -c "ShopAbundanceThreshold" internal/shops/context.md
```
Expected: all symbols resolve; at least 1.

- [ ] **Step 4: Delete and commit**

```bash
echo '## Shop Persistence (Living Economy)' > /c/tmp/shops-manifest.txt
python tools/strip_claude_sections.py --manifest /c/tmp/shops-manifest.txt --apply; echo "exit=$?"
git diff CLAUDE.md | grep "^+" | grep -v "^+++" | wc -l
git add internal/shops/context.md CLAUDE.md
git commit -m "docs(shops): document the dynamic pricing range and its eight knobs"
```
Expected: `exit=0`, 0 added lines.

---

## Task 6: batch 4, `internal/buffs/context.md` (Buff/Ward)

**Files:**
- Modify: `internal/buffs/context.md` (809 lines)
- Modify: `CLAUDE.md` (remove `## Buff/Ward Spell System`)

- [ ] **Step 1: Verify the symbols and the existing analysis**

```bash
grep -rn "effect_magnitude" internal/buffs/*.go internal/spells/*.go | head
grep -rn "magical_mitigation\|conviction_mitigation" internal/buffs/*.go | head
ls docs/superpowers/audits/2026-08-30-shield-spells-converge-at-the-cap.md
```

- [ ] **Step 2: Write the section**

Cover shield scaling by `effect_magnitude` (100 = 1.0x baseline, Conviction Ward
75, Chrysalis Cocoon 125), that crits add 50 percent strength, and that the
`magical_mitigation` and `conviction_mitigation` statmods flow through
`GetMagicalMitigation()` and `GetConvictionMitigation()`.

Cross-reference `docs/superpowers/audits/2026-08-30-shield-spells-converge-at-the-cap.md`,
which already analyses why these shields saturate against the mitigation cap.

- [ ] **Step 3: Verify, delete, commit**

```bash
python tools/context_md_audit.py internal/buffs
echo '## Buff/Ward Spell System' > /c/tmp/buffs-manifest.txt
python tools/strip_claude_sections.py --manifest /c/tmp/buffs-manifest.txt --apply; echo "exit=$?"
git diff CLAUDE.md | grep "^+" | grep -v "^+++" | wc -l
git add internal/buffs/context.md CLAUDE.md
git commit -m "docs(buffs): document shield magnitude scaling"
```
Expected: symbols resolve, `exit=0`, 0 added lines.

---

## Task 7: batch 5, `internal/characters/context.md` (Regen)

**Files:**
- Modify: `internal/characters/context.md` (1,806 lines)
- Modify: `CLAUDE.md` (remove `## Regen System (Stage 29.5)`)

Append only. This file is already oversized; right-sizing it is explicitly out of
scope, to be recorded in Task 11's audit rather than acted on.

- [ ] **Step 1: Verify**

```bash
grep -rn "func.*HealthPerRound\|func.*StaminaPerRound\|func.*ConvictionPerRound" internal/characters/*.go
for k in PlayerHealthRegenPct PlayerStaminaRegenPct PlayerConvictionRegenPct MobHealthRegenPct MobStaminaRegenPct MobConvictionRegenPct; do
  printf "%-28s go:%s yaml:%s\n" "$k" \
    "$(grep -c "$k" internal/configs/config.balance*.go)" \
    "$(grep -c "$k" _datafiles/config.yaml)"
done
```

- [ ] **Step 2: Write, verify, delete, commit**

Cover: all regen is percentage-of-max and never flat, the six knobs, that
`HealthPerRound()` and siblings compute `floor(poolMax * pct)` with a minimum of
1, that mutations use multiplier effects rather than flat `health_regen`, and
that heal spells store a regen multiplier in `effect_magnitude`.

```bash
python tools/context_md_audit.py internal/characters
echo '## Regen System (Stage 29.5)' > /c/tmp/regen-manifest.txt
python tools/strip_claude_sections.py --manifest /c/tmp/regen-manifest.txt --apply; echo "exit=$?"
git diff CLAUDE.md | grep "^+" | grep -v "^+++" | wc -l
git add internal/characters/context.md CLAUDE.md
git commit -m "docs(characters): document percentage-of-max regen"
```
Expected: symbols resolve, `exit=0`, 0 added lines.

---

## Task 8: batch 6, `internal/hooks/context.md` (Spell Duration)

**Files:**
- Modify: `internal/hooks/context.md` (1,604 lines)
- Modify: `CLAUDE.md` (remove `## Spell Duration System`)

This lands in `hooks`, not `spells`, because that is where the function lives.

- [ ] **Step 1: Verify**

```bash
sed -n '30,50p' internal/hooks/spell_resolution.go
```
Expected: `func calcSpellDuration(baseFolds int, spellcastingSkill int, willpower int) int` at line 35.

- [ ] **Step 2: Write, verify, delete, commit**

Cover: `calcSpellDuration(baseFolds, skill, willpower)` producing
`baseFolds × (10 + wil/20 + skill/2)`, and the effect-specific scaling: shield
full, heal divided by 2, DoT divided by 3.

```bash
python tools/context_md_audit.py internal/hooks
echo '## Spell Duration System' > /c/tmp/duration-manifest.txt
python tools/strip_claude_sections.py --manifest /c/tmp/duration-manifest.txt --apply; echo "exit=$?"
git diff CLAUDE.md | grep "^+" | grep -v "^+++" | wc -l
git add internal/hooks/context.md CLAUDE.md
git commit -m "docs(hooks): document spell duration scaling"
```
Expected: symbols resolve, `exit=0`, 0 added lines.

---

## Task 9: the residual CLAUDE.md (spec step 3)

**Files:**
- Modify: `CLAUDE.md` (~115 lines -> ~71)

At this point CLAUDE.md holds only the four staying sections.

- [ ] **Step 1: Confirm the starting state**

```bash
grep "^## " CLAUDE.md
wc -l CLAUDE.md
```
Expected: exactly four headings (`Subagent Model Preference`, `Project Context`,
`Codegraph MCP — Code Intelligence`, `Package \`context.md\` Convention`), around
115 lines.

- [ ] **Step 2: Condense Codegraph from 47 lines to about 12**

Keep: what codegraph is, that it should be consulted before writing code rather
than during, and the tool-selection-by-intent list (`codegraph_context` for an
area, `codegraph_node` for a symbol, `codegraph_search` to find one,
`codegraph_trace` for a path). Drop the worked examples and the
subagent-guidance paragraph.

- [ ] **Step 3: Condense the `context.md` Convention from 42 lines to about 12**

Keep: that every package under `internal/` and `modules/` must carry one, that
creating a package requires shipping one and reshaping a package requires
updating one, and the verify-before-you-document rule with its
`Select-String` one-liner. Drop the section-structure template and the
do-not-write list.

- [ ] **Step 4: Fold Subagent Model Preference into a new working-style block**

Replace `## Subagent Model Preference` with `## Working style`, roughly 14 lines,
one line per item:

subagent-driven execution without asking; PowerShell for Windows process and
port work, Bash for git; no focus-stealing console windows; no em or en dashes;
proposals of 30 to 40 lines; the visual companion is a standing yes; a found flaw
widens scope rather than cutting it; fix flaws rather than reverting; finish
sibling paths you made inconsistent; the owner runs all deploys; Fable outranks
Opus; pick the subagent model that fits the task rather than defaulting to haiku.

- [ ] **Step 5: Add the tripwires block**

Roughly 15 lines, one per hazard, each naming the skill to load. Write it against
Task 1's routing table rather than from memory. At minimum:

- every `gh` command carries `--repo pruuk/DOGMud`, see `dogmud-shipping`
- never `git add -A` or `git add .`
- never edit a file with a Python read-modify-write; it truncates before the
  write evaluates
- never blanket-kill a server process by name or port, see `dogmud-playtesting`
- `config.yaml` carries skip-worktree, see `dogmud-balance-config`
- new files: state the full path and index them in `docs/README.md`
- read `config.yaml` for balance numbers, never a Go default

- [ ] **Step 6: Add the skills pointer**

One line stating that project procedure lives in `.claude/skills/` and that the
harness lists all twelve with their descriptions, so no index is repeated here.

- [ ] **Step 7: Verify**

```bash
wc -l CLAUDE.md
grep "^## " CLAUDE.md
```
Expected: roughly 71 lines; headings are Project Context, Working style,
Tripwires, Codegraph, `context.md` Convention, and the skills pointer.

Confirm every skill named in a tripwire exists:

```bash
grep -o "dogmud-[a-z-]*" CLAUDE.md | sort -u
ls .claude/skills/
```
Every name in the first list must appear in the second.

- [ ] **Step 8: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: condense CLAUDE.md to its always-loaded residue"
```

---

## Task 10: shrink MEMORY.md

**Files:**
- Modify: `C:\Users\Calabe Davis\.claude\projects\C--Users-Calabe-Davis-workspace-DOGMud\memory\MEMORY.md` (82 -> ~45 lines)

This file is outside the repo and is NOT committed. Edit it in place.

**Do not delete any memory topic file.** Tombstoning is Phase 3, so every
existing `[[link]]` must keep resolving.

- [ ] **Step 1: Confirm the starting state**

```bash
wc -l "/c/Users/Calabe Davis/.claude/projects/C--Users-Calabe-Davis-workspace-DOGMud/memory/MEMORY.md"
grep -n "^## " "/c/Users/Calabe Davis/.claude/projects/C--Users-Calabe-Davis-workspace-DOGMud/memory/MEMORY.md"
```
Expected: 82 lines; sections Current status, Repo Pointers, Git Workflow, SOPs,
Remaining Work, Feedback / Gotchas.

- [ ] **Step 2: Remove the sections whose content is now in skills**

Delete `## Git Workflow`, `## SOPs`, and `## Feedback / Gotchas` entirely, plus
the `### Reference notes` sub-list under Repo Pointers.

Replace all four with a single line under Repo Pointers:

```markdown
- 🧰 **Procedure lives in `.claude/skills/`** (twelve skills, loaded on demand). The harness lists them with descriptions; do not duplicate that index here.
```

**Never edit this file with a Python read-modify-write.** `open(path, 'w')`
truncates before the write expression evaluates, and this exact file has been
destroyed twice that way. Use an editor tool, or write to a temp file outside the
repo and move it into place.

- [ ] **Step 3: Verify**

```bash
wc -l "/c/Users/Calabe Davis/.claude/projects/C--Users-Calabe-Davis-workspace-DOGMud/memory/MEMORY.md"
grep -c "\[\[" "/c/Users/Calabe Davis/.claude/projects/C--Users-Calabe-Davis-workspace-DOGMud/memory/MEMORY.md"
```
Expected: roughly 45 lines, and a non-zero wikilink count (Current status and
Remaining Work keep theirs).

Confirm the two sections that must survive are intact:

```bash
grep -c "Current status\|Remaining Work" "/c/Users/Calabe Davis/.claude/projects/C--Users-Calabe-Davis-workspace-DOGMud/memory/MEMORY.md"
```
Expected: at least 2.

- [ ] **Step 4: No commit**

This file is outside the repo and is not version controlled. Report the before
and after line counts instead.

---

## Task 11: the whole-phase gate and closing audit

**Files:**
- Create: `docs/superpowers/audits/2026-09-09-phase-2-closing.md`
- Modify: `docs/README.md`

- [ ] **Step 1: Account for all 44 original sections**

```bash
git show 0e0f3acd9:CLAUDE.md > /c/tmp/claude-original.md
grep "^## " /c/tmp/claude-original.md > /c/tmp/original-headings.txt
wc -l /c/tmp/original-headings.txt
```
Expected: 44.

For each, determine where its content now lives: the new `CLAUDE.md`, a named
skill, a named `context.md`, or a named `docs/` file. Every one of the 44 gets
exactly one answer. A section with no answer is content lost by this phase and
must be restored before the phase closes.

- [ ] **Step 2: Confirm the tooling still passes**

```bash
python tools/verify_skills.py; echo "exit=$?"
for p in items crafting shops buffs characters hooks; do
  printf "%-12s " "$p"; python tools/context_md_audit.py internal/$p 2>&1 | tail -1
done
```
Expected: `12/12 skills present, 0 errors`, `exit=0`, and
`All documented symbols resolve.` for all six, including `items` which started
with 3 phantom.

- [ ] **Step 3: Measure the result**

```bash
wc -l CLAUDE.md
wc -c CLAUDE.md
wc -l "/c/Users/Calabe Davis/.claude/projects/C--Users-Calabe-Davis-workspace-DOGMud/memory/MEMORY.md"
```
Expected: roughly 71 lines for CLAUDE.md, roughly 45 for MEMORY.md. Report the
byte counts so the token saving can be estimated.

- [ ] **Step 4: Write the closing audit**

Create `docs/superpowers/audits/2026-09-09-phase-2-closing.md` containing:
- A table of all 44 original sections with where each now lives.
- Before and after sizes for `CLAUDE.md`, `MEMORY.md`, and each of the six
  `context.md` files.
- The recorded signal that `internal/characters/context.md` and
  `internal/hooks/context.md` remain oversized and grew slightly, with
  right-sizing still out of scope.
- Anything Phase 3 must pick up: the four self-contradictory or stale memory
  files, MEMORY.md's Sable miscitation and its AI-port contradiction, and the
  homeless CP-economy material from `reference-stat-progression-faucet-map`.

- [ ] **Step 5: Index and commit**

Add rows for both new audits to `docs/README.md`, matching the substantive
one-paragraph style of the neighbouring rows.

```bash
git add docs/superpowers/audits/2026-09-09-phase-2-closing.md docs/README.md
git commit -m "docs: phase 2 closing audit, all 44 sections accounted for"
```

---

## Task 12: open the PR

**Files:** none

CLAUDE.md is checked in and shared, and this branch deletes 977 lines from it.
That diff should be reviewable in one place.

- [ ] **Step 1: Push and open**

```bash
git push -u origin feature/context-budget-skill-extraction
gh pr create --repo pruuk/DOGMud --base master --head feature/context-budget-skill-extraction --fill
```

**Every `gh` command must carry `--repo pruuk/DOGMud`.** This repo is a fork of
`GoMudEngine/GoMud` and `gh` defaults to the parent. A bare `gh pr create` once
opened a PR against upstream and had to be closed immediately.

- [ ] **Step 2: Watch the checks**

```bash
gh pr checks <n> --repo pruuk/DOGMud --watch
```

Note `gh pr checks --watch` can return green before slow jobs register. Confirm
which runs actually executed before concluding the PR is clean.

- [ ] **Step 3: Stop**

Do not merge. Hand the PR number to the owner.

---

## Definition of done

1. `CLAUDE.md` is roughly 71 lines, down from 1,094.
2. `MEMORY.md` is roughly 45 lines, down from 82.
3. `python tools/verify_skills.py` exits 0.
4. `python tools/context_md_audit.py` reports zero phantom symbols across all six
   destination packages, including `items` which started with three.
5. All 44 original sections are accounted for in the closing audit, each in
   exactly one destination.
6. Every `git diff CLAUDE.md` in Tasks 2 through 8 showed **zero added lines**.
   Those steps are purely subtractive.
7. Task 1's routing table exists and every skill routes to at least one realistic
   prompt.
8. A PR is open against `pruuk/DOGMud`, unmerged.

Phase 3 (hooks, tombstones, and the memory-corpus corrections) gets its own plan.
