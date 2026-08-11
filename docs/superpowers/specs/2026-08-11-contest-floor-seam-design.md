# Contest Floor Seam Consolidation — Design

**Roadmap chunk:** 5.10, `docs/roadmaps/ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md`
**Date:** 2026-08-11
**Status:** approved in conversation; awaiting spec review

## Problem

Whether an opposed contest is floored is decided at each call site by which
function the author happened to call. The two candidates have **identical
signatures** and near-identical names:

```go
func OpposedRollStat(atk, def float64)        (bool, float64, RollResult, RollResult)
func OpposedRollStatFloored(atk, def float64) (bool, float64, RollResult, RollResult)
```

Nothing enforces the choice and nothing makes the safe one obvious. That shape
is what produced the original gap: the floors were written for combat, lived in
`internal/combat/combat_helpers.go`, and every contest added afterwards silently
got the unfloored path — stealth, theft, traps, detection, spells and maneuvers,
found only when chunk 5.9 went looking.

Chunks 5.9a-c converted the call sites. They did not change the shape that
produced the problem.

**The project's own guidance currently points at the trap.** `CLAUDE.md:380`
says "For all stat-based rolls use `dice.RollStat(mean)` or
`dice.OpposedRollStat(atk, def)`" — and `OpposedRollStat` is today the
**unfloored** function. A developer following the house style guide writes an
unfloored contest. That is the clearest available evidence that the naming, not
the call sites, is the defect: the guidance is right about intent and wrong
about outcome purely because the safe behavior lives under the longer name.
After the rename that sentence becomes correct with no edit.

## Current state (measured 2026-08-11)

| Function | Non-test callers | Note |
|---|---|---|
| `OpposedRollStatFloored` | 17 | the real path |
| `OpposedRollStatFlooredWith` | 16 | per-contest floors (spells, maneuvers) |
| `OpposedRollStat` | **0** | exported, unfloored, and the most natural name |
| `OpposedRoll` | **0** | exported, unfloored, low-level |

Test callers of the unfloored functions, all outside package `dice`:
five sites in `internal/combat/integration_combat_test.go` and
`internal/combat/regression_test.go`. They exercise the raw distribution and
z-score regressions deliberately, and are legitimate.

Combat's third way is also legitimate and stays: `resolveAttack`
(`internal/combat/combat_helpers.go:841`) applies `MinAttackHitChance` and
`MinDefenseChance` to a computed **hit chance**, not to a roll outcome, so it
cannot route through the dice-level floor.

## Goal

One seam where floor policy lives, such that adding a contest cannot
accidentally opt out. The unsafe path must be something a developer types on
purpose and justifies.

## Design

### 1. Flip the default by renaming

| Before | After | Meaning |
|---|---|---|
| `OpposedRollStatFloored` | **`OpposedRollStat`** | the default; both ends floored |
| `OpposedRollStatFlooredWith` | **`OpposedRollStatWithFloors`** | per-contest floors |
| `OpposedRollStat` | **`OpposedRollStatRaw`** | no floor; caller must apply its own |

`OpposedRollStat` is the name a developer reaches for, so it must be the safe
one. After the rename the 33 contest sites get shorter, and the unfloored path
announces itself.

`OpposedRollStatRaw` keeps a doc comment stating that it applies no floor, that
the caller must apply its own as `resolveAttack` does, and that using it without
one recreates the 5.9 gap.

`OpposedRollStatFloored` and `OpposedRollStatFlooredWith` are **deleted**, not
kept as aliases. Retaining them would reintroduce the two-names-for-one-thing
confusion this chunk exists to remove.

### 2. The rename must happen in two ordered steps

**This is the load-bearing detail of the whole change.** The two functions have
identical signatures, so a swap is *not* compiler-verified. Renaming
`OpposedRollStatFloored` to `OpposedRollStat` while the old `OpposedRollStat`
still exists would leave the five combat tests silently calling the **floored**
function while asserting on raw-distribution behavior — a behavior change with
no compile error, in exactly the tests meant to detect behavior changes.

Required order:

- **Step 1:** rename `OpposedRollStat` to `OpposedRollStatRaw` everywhere,
  including the in-package caller in `contest_floors.go:89`, the internal
  delegate at `dice.go:462`, and the five combat tests. After this step the
  identifier `OpposedRollStat` does not exist anywhere. The compiler proves the
  rename is complete. Behavior is unchanged; the build is green.
- **Step 2:** rename `OpposedRollStatFloored` to `OpposedRollStat` and
  `OpposedRollStatFlooredWith` to `OpposedRollStatWithFloors`. The old names no
  longer exist, so the compiler again proves completeness. Behavior is unchanged.

Each step is independently compilable and independently committable. Doing them
in one edit forfeits the compiler's verification and must not be done.

### 3. Guard the remaining hatch

A new AST guard test at the repo root, `contest_floor_guard_test.go`, package
`main`, mirroring `durable_write_guard_test.go`. That precedent exists because
two grep audits of durable writes came back short and the AST guard immediately
found a real miss.

The guard fails when a call to `dice.OpposedRollStatRaw` or `dice.OpposedRoll`
appears outside an allow-list.

**Allow-list at introduction:**

- `internal/dice` — owns the primitives and delegates between them.

No production code outside `dice` calls either function today, so the list
starts nearly empty. That is the point: a new entry requires writing down a
reason.

**`_test.go` files are skipped**, with the reason recorded in the guard: tests
legitimately probe the raw distribution, and the risk being guarded is a
*production* contest silently opting out of the floor. A test asserting on raw
dice behavior is not a gameplay gap.

### 4. Documentation

- `internal/dice/context.md` — update the public API section to the new names
  and state the default-safe rule.
- `internal/dice/README.md` — update any named references.
- `CLAUDE.md` already advises "use `dice.RollStat(mean)` or
  `dice.OpposedRollStat(atk, def)`". That advice becomes correct-by-default
  rather than a trap; verify the wording still reads right and note the raw
  escape hatch.

## Testing

Per project SOP, every new test must be **mutation-verified**: confirmed to fail
against code that lacks the fix.

1. **Guard test self-check.** Temporarily introduce a `dice.OpposedRollStatRaw`
   call in a non-exempt package and confirm the guard fails, naming the file.
   Revert. A guard that has never failed is not known to work.
2. **Existing floor tests** continue to pass under the new names, proving the
   rename was mechanical.
3. **The five combat tests** keep asserting raw-distribution behavior under
   `OpposedRollStatRaw`. If any of them changes result, step 1 was done wrong.
4. **Full `go test ./...`** after each of the two rename steps, not only at the
   end — the point of splitting them is to have two verified checkpoints.

## Acceptance

- `grep -rn "OpposedRollStatFloored" --include=*.go` returns nothing.
- `dice.OpposedRollStatRaw` and `dice.OpposedRoll` have zero non-test callers
  outside `internal/dice`.
- The guard test exists, is mutation-verified, and its allow-list carries a
  written reason per entry.
- `gofmt -l internal/` is clean and `go build ./...` passes.

## Non-goals

- Changing any floor **value**. 5.9a-c settled those (0.05 non-combat, 0.05
  spells and maneuvers, 0.15 combat) and this chunk is a refactor of a settled
  policy, per the 5.10 boundary.
- Changing `resolveAttack`'s inline hit-chance floors. They floor a different
  quantity and correctly do not route through dice.
- Touching `dice.Roll` / `dice.RollStat` (23 non-test callers). They are single
  rolls for damage variance, not contests, and are out of scope.

## Risks

- **Silent semantic swap** if the two rename steps are merged. Mitigated by the
  ordering above, which is mandatory rather than advisory.
- **Guard drift**, where the allow-list grows without scrutiny. Mitigated by
  requiring a written reason per entry, following the durable-write guard, whose
  entries each state why the package is genuinely not living state.
- **Low residual risk overall**: zero production callers move semantics, and
  both steps are compiler-verified.
