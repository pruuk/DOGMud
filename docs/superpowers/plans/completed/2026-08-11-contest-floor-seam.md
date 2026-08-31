# Contest Floor Seam Consolidation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the floored opposed roll the default by giving it the natural name, and guard the remaining unfloored hatch with an AST test.

**Architecture:** Two ordered, separately committed renames — each provable complete by the compiler — followed by a repo-root AST guard test mirroring `durable_write_guard_test.go`, then documentation.

**Tech Stack:** Go, `go/ast` + `go/parser` for the guard, standard `go test`.

**Spec:** `docs/superpowers/specs/completed/2026-08-11-contest-floor-seam-design.md`

---

## Critical ordering rules — read before touching anything

Two rename hazards. Both are silent. Both are avoided by ordering alone.

**Hazard 1 — identical signatures across the swap.** `OpposedRollStat` and
`OpposedRollStatFloored` are both `func(atk, def float64) (bool, float64,
RollResult, RollResult)`. Renaming `Floored` to `OpposedRollStat` while the old
`OpposedRollStat` still exists leaves five combat tests compiling fine but
exercising the **floored** function while asserting raw-distribution behavior.
No compile error. Therefore: **Task 1 must fully eliminate the identifier
`OpposedRollStat` before Task 2 reintroduces it.**

**Hazard 2 — prefix collision inside Task 2.** `OpposedRollStatFloored` is a
prefix of `OpposedRollStatFlooredWith`. A replace-all of the shorter name would
turn `OpposedRollStatFlooredWith` into `OpposedRollStatWith`. Therefore:
**inside Task 2, rename the longer name FIRST.**

Do not batch these. Do not reorder them.

---

## File Structure

**Modified — Task 1 (`OpposedRollStat` → `OpposedRollStatRaw`):**
- `internal/dice/dice.go` — the definition at :462 and its docstring
- `internal/dice/contest_floors.go` — internal caller at :89
- `internal/dice/dice_test.go` — 3 call sites
- `internal/combat/integration_combat_test.go` — 3 call sites
- `internal/combat/regression_test.go` — 2 call sites

**Modified — Task 2 (`FlooredWith` → `WithFloors`, then `Floored` → `OpposedRollStat`):**
- `internal/dice/contest_floors.go` — both definitions and their docstrings
- `internal/dice/contest_floors_test.go`
- `Floored(` callers (7): `internal/actions/defuse.go`, `plant.go`, `shadow.go`, `sneak.go`, `steal.go`; `internal/usercommands/go.go`, `skill.skullduggery.shadow.go`
- `FlooredWith(` callers (11): `internal/actions/combat_taunt.go`; `internal/combat/avoidance.go`, `flee.go`, `grapple.go`, `skill_moves.go`, `submission.go`; `internal/hooks/charm_spell.go`, `NewRound_MobRoundTick.go`, `Position_GrappleTick.go`, `spell_resolution.go`; `internal/usercommands/throw.go`

**Created — Task 3:**
- `contest_floor_guard_test.go` (repo root, package `main`)

**Modified — Task 4:**
- `internal/dice/context.md`, `internal/dice/README.md`, `CLAUDE.md`

---

### Task 1: Rename the unfloored function to `OpposedRollStatRaw`

**Files:** the five listed under Task 1 above.

- [ ] **Step 1: Rewrite the definition and its docstring**

In `internal/dice/dice.go`, replace the whole block at :455-464. The existing
docstring actively recommends the unfloored path ("Use this for every
attack-vs-defense, spell-vs-resist, grapple, bash, kick, and trip check") — that
sentence is part of the defect and must not survive.

```go
// OpposedRollStatRaw performs a contested check between two stat-based scores
// with NO contest floor applied.
//
// Both sides are rolled with the attacker's standard deviation (StdDevFor(atk))
// so the spread scales proportionally to the attacker's power. Returns the same
// values as OpposedRoll: (success, margin, attackRoll, defenseRoll).
//
// You almost certainly want OpposedRollStat instead, which floors both ends.
// Use this ONLY where the caller applies its own floors -- combat's
// resolveAttack does, because it floors a computed hit CHANCE rather than a
// roll outcome. Calling this without applying a floor recreates the gap that
// roadmap chunk 5.9 was opened to close: a stat-100 thief against a stat-150
// mark succeeded 0.9% of the time.
func OpposedRollStatRaw(atk, def float64) (bool, float64, RollResult, RollResult) {
	return OpposedRoll(atk, def, StdDevFor(atk))
}
```

- [ ] **Step 2: Update the one internal caller**

In `internal/dice/contest_floors.go:89`, inside `OpposedRollStatFlooredWith`:

```go
	success, margin, attackRoll, defenseRoll := OpposedRollStatRaw(atk, def)
```

- [ ] **Step 3: Update the five out-of-package test call sites**

Replace `dice.OpposedRollStat(` with `dice.OpposedRollStatRaw(` in:
- `internal/combat/integration_combat_test.go` (3 sites)
- `internal/combat/regression_test.go` (2 sites)

And the in-package ones, `OpposedRollStat(` → `OpposedRollStatRaw(` in:
- `internal/dice/dice_test.go` (3 sites)

These tests exercise the raw distribution on purpose. Their assertions must not
change — only the function name.

- [ ] **Step 4: Prove the identifier is gone**

Run:
```bash
grep -rn "OpposedRollStat\b" --include=*.go internal/ modules/ | grep -v "OpposedRollStatRaw\|OpposedRollStatFloored"
```
Expected: **no output**. Any hit means a site was missed and Task 2 would
silently repoint it.

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./internal/dice/... ./internal/combat/...`
Expected: PASS. Behavior is unchanged by this task; a failure here means a
rename touched an assertion.

- [ ] **Step 6: Commit**

```bash
git add internal/dice/dice.go internal/dice/contest_floors.go internal/dice/dice_test.go internal/combat/integration_combat_test.go internal/combat/regression_test.go
git commit -m "refactor(dice): rename OpposedRollStat to OpposedRollStatRaw

Step 1 of 2. Frees the natural name so the FLOORED roll can take it, and
eliminates the identifier entirely so step 2's rename is compiler-verified
rather than a silent semantic swap across identical signatures.

Also removes the docstring line recommending this function for every
attack-vs-defense and spell-vs-resist check -- guidance that pointed callers at
the unfloored path and helped produce the 5.9 gap.

No behavior change."
```

---

### Task 2: Give the natural name to the floored roll

**Files:** `internal/dice/contest_floors.go`, `internal/dice/contest_floors_test.go`, plus the 7 `Floored(` and 11 `FlooredWith(` caller files listed above.

**Rename the LONGER name first (Hazard 2).**

- [ ] **Step 1: `OpposedRollStatFlooredWith` → `OpposedRollStatWithFloors`, everywhere**

Definition in `internal/dice/contest_floors.go:88`, its self-reference in the
`OpposedRollStatFloored` docstring, `contest_floors_test.go`, and all 11 caller
files. Update the docstring's opening line to match the new name:

```go
// OpposedRollStatWithFloors is OpposedRollStat with the floors supplied per
// call, for contests whose failure cost differs enough to want their own
// values.
```

- [ ] **Step 2: Verify the longer name is fully gone before continuing**

Run:
```bash
grep -rn "OpposedRollStatFlooredWith" --include=*.go .
```
Expected: **no output**. If anything remains, Step 3's replace will mangle it
into `OpposedRollStatWith`.

- [ ] **Step 3: `OpposedRollStatFloored` → `OpposedRollStat`, everywhere**

Definition in `internal/dice/contest_floors.go:69`, `contest_floors_test.go`,
and all 7 caller files. Rewrite the docstring, which currently defines itself in
terms of the old default:

```go
// OpposedRollStat performs a contested check between two stat-based scores with
// both ends floored. This is the DEFAULT opposed roll: use it for every
// attack-vs-defense, spell-vs-resist, stealth, theft, trap, grapple, bash,
// kick and trip check.
//
// Flooring both ends means neither a hopeless underdog nor an overwhelming
// favourite ever faces a foregone conclusion. If you believe you want the
// unfloored roll, see OpposedRollStatRaw -- and expect to justify it in
// contest_floor_guard_test.go.
//
// When a floor flips the outcome, the margin is reduced to the smallest value
// carrying the new sign. A floor save is a BARE success, not a decisive one, and
// callers that scale an effect by margin must not read it as a rout.
```

- [ ] **Step 4: Build and run the full suite**

Run: `go build ./... && go test ./...`
Expected: PASS. Still no behavior change — every renamed site keeps the exact
function it had before.

- [ ] **Step 5: Confirm the old names are gone**

Run:
```bash
grep -rn "OpposedRollStatFloored" --include=*.go .
```
Expected: **no output**.

- [ ] **Step 6: Check formatting**

Run: `gofmt -l internal/`
Expected: no output. This has its own CI gate and has broken a push before.

- [ ] **Step 7: Commit**

```bash
git add -u internal/
git commit -m "refactor(dice): make the floored opposed roll the default name

Step 2 of 2. OpposedRollStatFloored becomes OpposedRollStat and
OpposedRollStatFlooredWith becomes OpposedRollStatWithFloors, so the name a
developer reaches for is the safe one and the 33 contest call sites get shorter.

The longer name was renamed first: OpposedRollStatFloored is a prefix of
OpposedRollStatFlooredWith, so doing the short one first would have rewritten
FlooredWith into OpposedRollStatWith.

No behavior change; every site keeps the function it already had."
```

---

### Task 3: Guard the raw hatch

**Files:**
- Create: `contest_floor_guard_test.go` (repo root, package `main`)

- [ ] **Step 1: Write the guard**

Mirror `durable_write_guard_test.go` in the same directory — same walk, same
skip list, same AST shape, same package. Full content:

```go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// unflooredRollExemptions lists the packages allowed to call the UNFLOORED
// opposed rolls directly, with the reason each one genuinely needs them.
//
// Keys are repo-relative directories. A file is exempt if its directory matches
// a key or sits underneath one.
var unflooredRollExemptions = map[string]string{
	// Owns both primitives and delegates between them: OpposedRollStat is
	// implemented in terms of OpposedRollStatRaw, which is implemented in terms
	// of OpposedRoll.
	"internal/dice": "owns the roll primitives and delegates between them",
}

// unflooredRollFuncs are the roll functions that apply no contest floor.
var unflooredRollFuncs = map[string]bool{
	"OpposedRollStatRaw": true,
	"OpposedRoll":        true,
}

// TestOpposedContestsAreFloored fails when a package outside the exemption list
// calls an unfloored opposed roll.
//
// This is the recurrence guard for roadmap chunk 5.10. The floors were written
// for combat, lived in internal/combat/combat_helpers.go, and every contest
// added afterwards silently got the unfloored path -- stealth, theft, traps,
// detection, spells and maneuvers. A stat-100 thief against a stat-150 mark
// succeeded 0.9% of the time. Nobody chose that; it was inherited by whichever
// function the author copied from.
//
// Chunk 5.10 made the floored roll the default by giving it the natural name,
// which removes the trap for anyone writing ordinary code. This test covers what
// the rename cannot: OpposedRollStatRaw and OpposedRoll still exist and still
// work, and a future contest can still reach for one.
//
// Test files are deliberately NOT scanned. Tests probe the raw distribution on
// purpose (see internal/combat/regression_test.go); the risk being guarded is a
// PRODUCTION contest silently opting out of the floor.
//
// If you are adding a caller that genuinely applies its own floors -- as
// combat's resolveAttack does, flooring a computed hit chance rather than a roll
// outcome -- add its directory here with a reason. If you cannot write the
// reason, you want OpposedRollStat.
func TestOpposedContestsAreFloored(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var offenders []string
	fset := token.NewFileSet()

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "bin", "_datafiles", "docs", "tools":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		dir := filepath.ToSlash(filepath.Dir(rel))
		for exempt := range unflooredRollExemptions {
			if dir == exempt || strings.HasPrefix(dir, exempt+"/") {
				return nil
			}
		}

		file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			// A syntax error is the compiler's problem to report, not this
			// test's. Skipping it keeps failures attributable.
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !unflooredRollFuncs[sel.Sel.Name] {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "dice" {
				return true
			}
			offenders = append(offenders,
				rel+": dice."+sel.Sel.Name+" at line "+
					strconv.Itoa(fset.Position(call.Pos()).Line))
			return true
		})

		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("unfloored opposed rolls outside the exemption list:\n  %s\n\n"+
			"Use dice.OpposedRollStat (floored by default), or add the package to "+
			"unflooredRollExemptions with a reason.",
			strings.Join(offenders, "\n  "))
	}
}
```

- [ ] **Step 2: Run it — expect PASS**

Run: `go test -run TestOpposedContestsAreFloored .`
Expected: PASS. After Tasks 1-2 there are no production callers outside
`internal/dice`.

- [ ] **Step 3: Mutation-verify — the critical step**

A guard that has never failed is not known to work. Temporarily add to
`internal/actions/steal.go`, inside any function:

```go
	_, _, _, _ = dice.OpposedRollStatRaw(100, 100)
```

Run: `go test -run TestOpposedContestsAreFloored .`
Expected: **FAIL**, naming `internal/actions/steal.go` and the line number.

If it PASSES, the guard is broken — most likely the walk is skipping the file or
the selector match is wrong. Fix the guard before continuing.

- [ ] **Step 4: Revert the mutation**

Run: `git checkout internal/actions/steal.go`
Then re-run: `go test -run TestOpposedContestsAreFloored .`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add contest_floor_guard_test.go
git commit -m "test(dice): guard against unfloored opposed contests

Recurrence guard for roadmap chunk 5.10, in the style of
durable_write_guard_test.go -- which exists because two grep audits of durable
writes came back short and the AST guard immediately found a real miss.

The 5.10 rename makes the floored roll the default for anyone writing ordinary
code. This covers what a rename cannot: OpposedRollStatRaw and OpposedRoll still
exist, and a future contest can still reach for one.

Test files are not scanned -- they probe the raw distribution on purpose. The
risk being guarded is a production contest silently opting out.

Mutation-verified: fails on a planted dice.OpposedRollStatRaw call in
internal/actions/steal.go, naming the file and line."
```

---

### Task 4: Documentation

**Files:** `internal/dice/context.md`, `internal/dice/README.md`, `CLAUDE.md`

- [ ] **Step 1: Update `internal/dice/context.md`**

Find every reference to `OpposedRollStat`, `OpposedRollStatFloored`, and
`OpposedRollStatFlooredWith` and update to the new names. In the Public API
section, state the rule plainly: `OpposedRollStat` is the default and floors both
ends; `OpposedRollStatWithFloors` takes per-contest values; `OpposedRollStatRaw`
applies no floor and is guarded by `contest_floor_guard_test.go`.

Add to Gotchas: the two functions that were swapped share a signature, so any
future rename in this area is not compiler-verified and must be done in the
two-step order recorded in the 5.10 spec.

- [ ] **Step 2: Update `internal/dice/README.md`**

Run `grep -n "OpposedRollStat" internal/dice/README.md` and update each hit to
the new names, preserving the surrounding win-probability discussion.

- [ ] **Step 3: Verify `CLAUDE.md:380`**

It currently reads:
> **For all stat-based rolls use `dice.RollStat(mean)` or `dice.OpposedRollStat(atk, def)`** — no stdDev argument needed

This sentence needed no edit to become correct — it named the unfloored function
before and names the floored one now. Append a short clause recording the escape
hatch so the guidance stays complete:

> `OpposedRollStat` floors both ends by default; `dice.OpposedRollStatRaw` is the
> unfloored escape hatch and is guarded by `contest_floor_guard_test.go`.

- [ ] **Step 4: Commit**

```bash
git add internal/dice/context.md internal/dice/README.md CLAUDE.md
git commit -m "docs(dice): record the default-floored contest contract

CLAUDE.md:380 told developers to use dice.OpposedRollStat, which before 5.10 was
the UNFLOORED function -- the house style guide pointed at the trap. The rename
made that sentence correct without editing it; this adds the escape hatch so the
guidance is complete."
```

---

### Task 5: Acceptance

- [ ] **Step 1: Run the acceptance checks from the spec**

```bash
grep -rn "OpposedRollStatFloored" --include=*.go .          # want: no output
grep -rn "dice\.OpposedRollStatRaw(\|dice\.OpposedRoll(" --include=*.go internal/ modules/ | grep -v _test.go   # want: no output
gofmt -l internal/ modules/                                  # want: no output
go build ./...                                               # want: success
go test ./...                                                # want: PASS
```

- [ ] **Step 2: Update the roadmap**

In `docs/roadmaps/ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md`, set the 5.10 row
in the summary table to `**Done 2026-08-11**`, and append a short outcome note
to the 5.10 section recording what was chosen: default flipped by rename, old
names deleted rather than aliased, guard added and mutation-verified.

- [ ] **Step 3: Update `docs/PATCH_NOTES.md`**

Per pre-push SOP, add a dated entry. This change is invisible to players, so keep
it to a one-line internal note. Player-facing framing, no raw numbers, no em
dashes.

- [ ] **Step 4: Commit and push via PR**

```bash
git add docs/roadmaps/ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md docs/PATCH_NOTES.md
git commit -m "docs(roadmap): 5.10 done -- contest floor seam consolidated"
git push -u origin refactor/5.10-contest-floor-seam
gh pr create --repo pruuk/DOGMud --base master --head refactor/5.10-contest-floor-seam --fill
```

Then read the PR number from the `gh pr create` output and watch it:

```bash
gh pr checks <PR-number-from-above> --repo pruuk/DOGMud --watch
```

Create the branch at the START of Task 1, before any edits:
`git checkout -b refactor/5.10-contest-floor-seam`

**`gh` defaults to the fork PARENT.** Every command must carry
`--repo pruuk/DOGMud`. A bare `gh pr create` opened a PR against upstream on
2026-08-08.

**Do not stage `_datafiles/world/dogmud/rooms/thornwall_city/473.yaml`** or any
untracked file in the working tree — the user is testing admin builder saves and
those changes are theirs, explicitly excluded from commits. Stage named paths
only; never `git add -A`.

---

## Notes for the implementer

- A boot test is **not** required. This chunk touches no YAML data files and no
  loader, and the pre-push boot test exists to catch data-file panics.
- No behavior changes in Tasks 1-2. If any existing test's assertion needs
  editing to pass, stop — that means a rename crossed a semantic boundary and
  the ordering rules were violated.
- Prefer codegraph MCP (`codegraph_node`, `codegraph_callers`) over grep for
  verifying call sites; the counts in this plan were taken 2026-08-11 and the
  index is authoritative if they disagree.
