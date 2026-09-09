# Context Budget Phase 2: Strip CLAUDE.md and Fill the Gaps

**Date:** 2026-09-08
**Status:** design approved, not yet planned
**Full path:** `docs/superpowers/specs/2026-09-08-context-budget-phase-2-design.md`
**Phase 1 spec:** `docs/superpowers/specs/2026-09-08-context-budget-skill-extraction-design.md`
**Phase 1 audit:** `docs/superpowers/audits/2026-09-08-skill-coverage.md`

## Facts verified against source (2026-09-08)

Read from the tree on the date of writing, not recalled.

| Fact | Value | How verified |
|---|---|---|
| `CLAUDE.md` | 1,094 lines, 24.5k tokens | `wc -l`, `/context` |
| `MEMORY.md` | 82 lines, 8.1k tokens | `wc -l`, `/context` |
| Always-loaded memory total | 33.3k tokens | `/context` |
| Skills shipped by Phase 1 | 12, 2,711 lines | `wc -l .claude/skills/*/SKILL.md` |
| `tools/verify_skills.py` | exits 0, `12/12 skills present` | run |
| `CLAUDE.md` since Phase 1 | byte-identical to `0e0f3acd9` | `git diff --stat` empty |

### Table A dispositions (from the Phase 1 audit, re-tallied)

| Disposition | Sections | Lines |
|---|---|---|
| Lifted into a skill, verified faithful | 23 | 635 |
| Delete, verified covered elsewhere | 7 | 130 |
| Demote to a `context.md` | 1 | 45 |
| **Safe subtotal** | **31** | **810** |
| Gap or stale, blocked | 9 | 167 |
| Stays in residual | 4 | 115 |
| **Total** | **44** | **1,092** |

The 44 sections account for 1,092 of `CLAUDE.md`'s 1,094 lines. The remaining two
are the file's title and the blank line under it, which stay.

Deletions across steps 1 and 2 total 977 lines (810 safe plus 167 gapped).

### The nine gapped sections and where they land

| Section | Lines | Destination | Destination size now |
|---|---|---|---|
| Alchemy & Potions System | 48 | `internal/items/context.md` | 943 lines |
| Equipment Slots | 24 | `internal/items/context.md` | " |
| Inventory & Item Disambiguation | 19 | `internal/items/context.md` | " |
| Caster Weapon Types | 15 | `internal/items/context.md` | " |
| Salvage System | 21 | `internal/crafting/context.md` | 277 lines |
| Shop Persistence, pricing half | 14 | `internal/shops/context.md` | 247 lines |
| Buff/Ward Spell System | 12 | `internal/buffs/context.md` | 809 lines |
| Regen System | 9 | `internal/characters/context.md` | 1,806 lines |
| Spell Duration System | 5 | `internal/hooks/context.md` | 1,604 lines |

### Code facts that corrected the plan

| Claim | Verified | Consequence |
|---|---|---|
| `calcSpellDuration` | `internal/hooks/spell_resolution.go:35` | Spell Duration goes to `hooks`, NOT `internal/spells/` as first assumed |
| `RunSalvageContest` | `internal/crafting/difficulty.go:152` | reads `Balance.SalvageFloor` |
| `SalvageFloor` | `config.balance.go:505`, shipped `0.15` at `config.yaml:1551` | the live knob; comment says it "reproduces the 15/85 clamp" |
| `SalvageMinChance`, `SalvageMaxChance`, `SalvageSoftCap` | declared `config.balance.go:529-531`, defaulted in `config.balance.misc.go`, **zero consumers** | CLAUDE.md presents three DEAD knobs as live tuning |
| `GetAgingPhase`, `CalcEffectiveAgingSpeed` | `internal/items/aging.go:31,151` | exist as documented |
| `tools/context_md_audit.py` | exists, 5,523 bytes | finds `context.md` files naming symbols their package does not define |

### Audit-tool baseline for the six destinations

| Package | Result |
|---|---|
| `items` | **3 phantom symbols of 31**: `UseItem`, `ItemPower`, `IsUpgrade` |
| `crafting`, `shops`, `buffs`, `characters`, `hooks` | all documented symbols resolve |

## What Phase 1 left, and what this phase does

Phase 1 was additive: twelve skills exist, nothing was deleted, and `CLAUDE.md` is
unchanged. The token win has therefore not been realised at all. Phase 2 is the
phase that collects it.

The Phase 1 audit changed this phase's shape in two ways the Phase 1 spec did not
anticipate:

1. **Only 31 of 44 sections are safe to delete.** The spec assumed roughly
   thirteen clean demotions to `docs/schemas/` and `context.md`. In fact nine
   sections are gapped or stale: the material is not documented anywhere else,
   so deleting them loses it.
2. **One section is actively wrong, not merely undocumented.** The Salvage
   System section describes a chance curve that U10b-1b replaced with a contest,
   and names three config knobs that nothing reads.

## Decisions

| Decision | Choice |
|---|---|
| Scope | Full: safe deletions AND gap-filling AND residual condensation |
| Gap-fill depth | Proper `context.md` sections following the project convention, verified against source |
| Salvage | Write the correct replacement, do not merely delete the wrong one |
| Codegraph and `context.md` Convention | Condense in place, both stay always-loaded |
| Sequencing | Safe deletions first, then gap-fill batched by destination, then residual |
| Trigger test | Added as step 0, before anything is deleted |
| Integration | Merge as a PR, not direct to master |

## Step 0: prove the skills actually trigger

**Nothing has yet tested that a skill loads on a realistic request.** Phase 1
verified the twelve are well-formed, not that they fire. If triggering is
unreliable, deleting `CLAUDE.md` does not move rules on-demand, it deletes them.
This step exists because that assumption underpins everything else and is
currently unevidenced.

Take at least fifteen realistic prompts spanning the twelve skills, phrased the
way someone would actually type them rather than in the vocabulary of the skill
descriptions. For example:

- "add a mob to Thornwall"
- "why is this test flaky"
- "open a PR for this branch"
- "is it safe to wipe rooms.instances"
- "make fire damage hurt more"
- "the NPC stopped talking"
- "the deploy took way longer than usual"
- "write the room description for the shrine"
- "my stat is not going up"
- "remove the bleedout feature"

For each, record which skill should fire and whether its `description` plausibly
matches. Produce a routing table.

**Any skill that nothing routes to, or that only matches wording nobody would
use, gets its description fixed before its CLAUDE.md source is deleted.** The
table is also the input Phase 3's tripwire block is written against, rather than
guessed at.

## Step 1: delete the 31 verified-safe sections

One sweep, one commit. 810 lines. Nothing is authored and nothing is judged: the
audit's Table A lists exactly which ranges may go.

Review is tractable despite the diff size because the question is narrow: did
anything outside the listed ranges change?

## Step 2: fill the nine gaps, batched by destination

Each batch: write the replacement, verify, delete the CLAUDE.md source, commit.
**Never the other order.** No section is ever in flight with its content nowhere.

### Batch 1: `internal/items/context.md` (four sections, 106 lines)

The largest batch and the only one that is a restructure rather than an
addition. Alchemy, Equipment Slots, Inventory Disambiguation and Caster Weapon
Types become one coherent pass, not four bolted-on blocks.

Verify: `GetAgingPhase` and `CalcEffectiveAgingSpeed` (`aging.go:31,151`), every
`ItemSpec` field named, both ID ranges (buffs 54-76, items 30036-30056 and
40043-40049), and the bottle tier table.

Note the split ownership: `spell_damage_multiplier` is an `ItemSpec` field but is
**applied** in `internal/hooks/spell_resolution.go`, so the items doc states the
field and points at the application site.

Clear the three pre-existing phantom symbols (`UseItem`, `ItemPower`,
`IsUpgrade`) while restructuring this file. They are not ours, but this batch
rewrites the file they live in, and leaving them is the kind of follow-up that
never happens.

### Batch 2: `internal/crafting/context.md` (Salvage, rewritten)

The only genuinely new writing in this phase. Document `RunSalvageContest`
(`difficulty.go:152`) and `SalvageDifficulty`, name `SalvageFloor` as the live
knob, and **explicitly mark `SalvageMinChance`, `SalvageMaxChance` and
`SalvageSoftCap` as dead** so nobody tunes them. That trio is the same shape as
the `MobStatCap` trap the progression skill already warns about.

The writer quotes the code they read. The reviewer re-reads it independently. The
audit tool cannot catch a wrong explanation of a real symbol, so this is the only
guard that works here.

### Batch 3: `internal/shops/context.md` (pricing half, 14 lines)

Eight knobs (`ShopBuyRatio`, `ShopPriceFloor`, `ShopPriceCeiling`,
`ShopAbundanceThreshold`, `ShopMaterialReserve`, `ShopGoldReserveRatio`,
`BarterMaxDiscount`, `BarterMaxBonus`) verified in both `config.balance*.go` and
`config.yaml`, with shipped versus Go-default labelled per the standing rule.

### Batch 4: `internal/buffs/context.md` (Buff/Ward, 12 lines)

Cross-reference `docs/superpowers/audits/2026-08-30-shield-spells-converge-at-the-cap.md`,
which already analyses this magnitude curve and its saturation behaviour.

### Batch 5: `internal/characters/context.md` (Regen, 9 lines)

Six regen knobs and the percentage-of-max rule. Append only.

### Batch 6: `internal/hooks/context.md` (Spell Duration, 5 lines)

`calcSpellDuration(baseFolds, skill, willpower)` at `:35`, with the shield, heal
and DoT scaling divisors. Append only.

### A signal recorded, not acted on

`characters/context.md` (1,806 lines) and `hooks/context.md` (1,604) are already
oversized, and this phase makes them slightly larger. Right-sizing them is
explicitly out of scope per the Phase 1 spec. Record it in the closing audit.

## Step 3: the residual

Target roughly 71 lines.

| Block | Lines | Note |
|---|---|---|
| Project Context | 7 | unchanged |
| Working style | ~14 | the 11 PREFERENCE files, plus Subagent Model Preference folded in: it is a preference about model choice, not a separate topic |
| Tripwires | ~15 | one line per hazard, each naming the skill to load, written against step 0's routing table |
| Codegraph, condensed | ~12 | keeps tool-selection-by-intent, drops the worked examples |
| `context.md` Convention, condensed | ~12 | keeps the must-ship rule and verify-before-you-document, drops the structure template |
| Skills pointer | 1 | the harness already lists all twelve with their descriptions |

**MEMORY.md: 82 to roughly 45 lines.** Current status and Remaining Work are
untouched, because volatile state is what an index is for. Git Workflow extras,
SOPs, and Feedback / Gotchas are replaced by one line pointing at
`.claude/skills/`.

The folded memory **files** are not touched in this phase. Tombstoning is Phase
3, so every existing `[[link]]` still resolves throughout.

## Verification gates

1. **Step 0.** Every one of the twelve skills is the best match for at least one
   realistic prompt. Any that is not gets its description fixed first.
2. **Step 1, deletion fidelity.** Extract removed ranges from the diff and
   compare against Table A. Then **re-derive** coverage after deletion: each
   deleted range's content must still be findable in its claiming skill. The
   check must not depend on the audit whose conclusion it is testing.
3. **Step 2, per batch.** `context_md_audit.py` clean on the touched file, with
   `items` measured against its 3-phantom baseline and expected to reach 0.
   Every symbol grepped. Token coverage from source section to destination.
4. **Step 3.** Every skill named in a tripwire exists, checked against
   `verify_skills.py`'s `EXPECTED` list.
5. **Whole-phase.** Take `CLAUDE.md` at the Phase 1 commit and confirm each of
   its 44 sections is findable in exactly one of: the new `CLAUDE.md`, a skill, a
   `context.md`, or `docs/`.

**No boot test.** Nothing reads `CLAUDE.md` programmatically. Stated so nobody
adds one out of habit.

## Risks

**The asymmetry with Phase 1.** Phase 1 undid with `rm -rf .claude/skills`. This
phase deletes 977 lines from a file the whole repo reads. Git makes it
recoverable, but the failure is silent: a rule vanishes and nobody finds out
until someone makes the mistake it prevented. Every gate above exists for this.

**The audit is one agent's work.** All 810 lines of step 1 rest on one Table A
from one run. Gate 2 re-derives rather than trusts, for that reason.

**The audit tool cannot catch a wrong explanation.** It finds phantom symbols. A
doc can name only real symbols and describe them incorrectly. Batch 2 is the
exposure.

**Triggering is unevidenced.** Addressed by step 0, which is why step 0 is first.

## Out of scope

- Phase 3: hooks, tombstones, and the memory-corpus corrections.
- Right-sizing `internal/combat/context.md` (113 KB),
  `internal/characters/context.md` (92 KB), `internal/hooks/context.md`.
- The source-material defects listed in the Phase 1 audit: four
  self-contradictory or stale memory files, and MEMORY.md's Sable miscitation.
  Correcting those is Phase 3.

## Success criteria

1. `CLAUDE.md` is roughly 71 lines, down from 1,094.
2. `MEMORY.md` is roughly 45 lines, down from 82.
3. Always-loaded memory drops from 33.3k tokens to roughly 7k.
4. `python tools/verify_skills.py` still exits 0.
5. `python tools/context_md_audit.py` reports zero phantom symbols across all six
   destination packages, including `items`.
6. The whole-phase gate passes: all 44 original sections accounted for.
7. Step 0's routing table exists and every skill routes to at least one realistic
   prompt.
