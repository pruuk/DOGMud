# Slam split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Every subagent brief carries these rules:** edit files with the Edit and Write tools only (`~/.claude/hooks/guard.py` blocks shell writes into any git working tree); `git add` named paths only; never `--no-verify`; no em or en dashes in prose or player text; no colon inside an unquoted YAML value; every `gh` command carries `--repo pruuk/DOGMud`; scratch output goes to the session scratchpad, never `$TMP`. World-data and combat-message changes run the FULL `go test . ./internal/... ./modules/...`.

**Goal:** Each slam species fights with lines written for its body: `slam` for fists and bulk, `trample` for hooves, `lash` for pseudopods and tendrils, `blast` for the energy elementals, `claws` for the smoke elemental, and every line fits 80 columns.

**Architecture:** Three new combat-message files plus a rewritten `slam.yaml`, drafted by agents in the store's record layout and reviewed by the owner as one table per file; then three subtypes wired the way `siphon` was in #538, and the species switched. The smoke elemental's `smother` special move gets its copy and condition text drafted and reviewed here; its code lands in slice 2 with `beast_moves`.

**Tech Stack:** YAML combat-message store, Go wiring in `internal/items`, `internal/species`, `internal/combat`, guards at the repo root and in `internal/items`.

**Spec:** `docs/superpowers/specs/2026-10-16-slam-split-design.md` (#569, owner rulings S1 to S5). Input: `docs/superpowers/audits/2026-10-16-slam-itemname-audit.md`.

---

## Facts verified against source (master `0612700ce`, 2026-10-16)

| # | Fact | Source |
|---|---|---|
| 1 | `slam.yaml` has 8 intensities (`prepare` 15, `wait` 57, `miss` 99, `weak` 141, `normal` 189, `heavy` 249, `critical` 312, `fumble` 372), each `together` with `beginner`, `expert`, `master` tiers of records `{actor, actee, observer}`; 115 records; its header lists the tokens (`{itemname}`, `{actor}`, `{actortype}`, `{actee}`, `{acteetype}`, `{damage}`, `{exitname}`, `{entrancename}`, `{stance}`, `{position}`, `{momentum}`; the audit adds `{bodypart}`) | file |
| 2 | Natural-attack subtypes: `internal/items/itemspec.go:166-178` constants and `:95-109` descriptions; `validNaturalAttacks`, `internal/species/species.go:212-219`; hit categories, `internal/combat/attackresult.go:261-277` (`slam` is `CategoryHitBlunt`, `siphon` `CategoryHitCaster`) | files |
| 3 | `internal/items/natural_weapon_agreement_test.go:16` lists `naturalWeaponPools = {Claws, Bite, Sting, Gore, Slam, Siphon}`; the test also reads every species' `natural_attack` | file |
| 4 | The observer identity guard walks `combat-messages/` (`shipped_narration_data_guard_test.go:895-900`); the banner guard, the idiom guard and the attack-message guards walk the store (`combat_banner_agreement_guard_test.go`, `combat_idiom_guard_test.go`, `consistent_attack_messages_guard_test.go`) | grep |
| 5 | The narration store golden is regenerated with `go test ./internal/narration/... -run TestSnapshotStores -update` after an intentional change (`internal/narration/snapshot_test.go:6,203`) | file |
| 6 | The pad and proposal tools (`tools/combat_message_pad_check.py`, `tools/combat_message_proposals_check.py`) check a batch against a baseline ref and a proposals manifest | file headers |
| 7 | Slam species and their `unarmedname`: spec Fact 1. Who fights unarmed: spec Fact 5 | spec |

## Task 1: Draft the copy (agents, parallel, read-only on code)

Five drafts, each a YAML file written to the session scratchpad plus a review table (`| intensity | tier | role | today | proposed | why |`, "today" empty for a new file):

- `slam.yaml` rewritten for heavy bodies (earth elemental "stone fists", automaton "servo-driven fists"): keeps every record that already reads right for fists and bulk, applies the audit's fixes where they fit this narrower group, fixes the three over-width lines (147, 318, 360).
- `trample.yaml` (horse, "hooves"): kicks, rears, stamps, tramples; may name hooves, legs, head, chest, flank.
- `lash.yaml` (slime "pseudopods", fungal colony "spore tendrils"): whips, smothers, engulfs, splatters; names no feet, head, chest, shoulder or footwork.
- `blast.yaml` (water, air, fire, magma, sand, storm and ice elementals): the element strikes through `{itemname}` ("watery surge", "slicing gust", "scorching strike", "molten slam", "abrasive whirl", "arcing strike", "crystal strike"); names no feet, head, chest, shoulder or footwork; reads right for all seven.
- `smother` copy: `narration/special-moves/smother.yaml` in the special-move store's layout (read `narration/special-moves/throttle.yaml` and its loader for the shape: hit, miss, partial and any other events the move store requires), plus the text of a new `Smothered` condition modelled on `conditions/89-throttled.yaml` (smoke in the lungs, not a grip on the throat).

Every draft: the same intensities and tiers as `slam.yaml` (combat-message files), the record layout with every role describing one moment, every token kept that the line needs, the M6 rules (no "its" for an actor, a space before every ansi tag, the banner lines in the banner style), and every line within 80 visible columns measured by script with 20-character names, the longest `unarmedname` in the group, and the longest value of each token. Each agent reports its width check output and `read N of N` for the files it modelled on.

- [ ] **Step 1:** Dispatch five drafting agents (Opus) in parallel with the rules above.
- [ ] **Step 2:** Controller check of each draft: YAML parses, the record counts per intensity and tier match `slam.yaml`'s structure, the width script passes, a grep finds no forbidden body part in `lash` and `blast`.
- [ ] **Step 3:** Publish one review page with five sections (one per file), the tables, and a counts header.

## Task 2: Owner review (gate)

- [ ] The owner approves or edits each table. Edits are applied to the drafts before Task 3. Record the approval date per file in the plan.

## Task 3: Wire the types and land the copy

**Files:** `internal/items/itemspec.go`, `internal/species/species.go`, `internal/combat/attackresult.go`, `internal/items/natural_weapon_agreement_test.go`, species YAML 15, 16, 26, 36, 38 to 44, the four combat-message files; tests.

- [ ] **Step 1: Failing tests first.** In `internal/species` add `TestSlamSplitAssignments`: species 37 and 45 `slam`; 26 `trample`; 15 and 16 `lash`; 36, 38, 39, 40, 41, 42, 43 `blast`; 44 `claws` with `unarmedname` "smoke-wrought claws". In `internal/combat` add a test that `CategoryForWeaponSubtype` returns `CategoryHitBlunt` for `trample` and `lash` and `CategoryHitCaster` for `blast`. Run: FAIL (undefined constants).
- [ ] **Step 2: Wire.** `items.Trample`, `items.Lash`, `items.Blast` constants and descriptions (siphon's pattern); add them to `validNaturalAttacks` and to `naturalWeaponPools`; the hit categories above. Land the four approved combat-message files (each with a header saying which species use it and what its lines may and may not name). Switch the species' `natural_attack` (and the smoke elemental's `unarmedname`).
- [ ] **Step 3: New guards.** A width test over every line of the four files (the Task 1 measurement, as a Go test in `internal/items`), and a body-part guard: `lash.yaml` and `blast.yaml` contain none of `feet`, `foot`, `head`, `chest`, `shoulder`, `footwork` as words.
- [ ] **Step 4: Render tests.** A slime, a horse, a sand elemental and an automaton each produce a swing line from their own file (`buildWeaponSetup` / the swing render path; model on the existing natural-attack render tests: `grep -ln "natural_attack\|NaturalAttack" internal/combat/*_test.go`).
- [ ] **Step 5: Goldens.** `go test ./internal/narration/... -run TestSnapshotStores -update` once the content is final; read the diff (only the four files' sections change) and quote its size in the commit.
- [ ] **Step 6: Run** `go test . ./internal/... ./modules/...`. Commit `feat(combat): slam splits into slam, trample, lash and blast (#541)`.

## Task 4: Hold the `smother` copy for slice 2

- [ ] Add the approved `smother` narration and `Smothered` condition text to the slice 2 spec's section 5 as an appendix (they land with the move's code, since an unreferenced special-move file or condition would fail the store guards). Commit `docs(combat): smother copy for slice 2 (#541)`.

## Task 5: Docs, gate, playtest, PR

- [ ] **Step 1:** `internal/species/context.md`: the four groups and why (S1), how to choose a type for a new species, a pointer to this spec. `internal/combat/context.md`: the three new types and their categories. Run `python tools/context_md_audit.py` on both packages, one per call.
- [ ] **Step 2:** Gate per `dogmud-shipping`: `gofmt -l`, `go build ./...`, `golangci-lint run --new-from-merge-base=origin/master`, the full suite, the boot check.
- [ ] **Step 3:** Playtest per `dogmud-playtesting`: a two-seat scenario (fighter plus witness) against a horse (Thornwall, Hob or Bran), a slime or fungal colony, and a summoned or oasis elemental; both seats read the same blow from the right file; no line passes 80 columns. File findings.
- [ ] **Step 4:** PR (part of #541), body: the four files with record counts, the review approval, evidence.

## Self-review

- Spec S1 to S5: groups and names (Task 3), smoke on claws (Task 3), smother drafted (Task 1) and held for slice 2 (Task 4), 80 columns (Tasks 1 and 3), full cycle (Tasks 1 to 5). Spec sections 1 to 4 and Testing map to Tasks 1, 3, 4 and 5.
