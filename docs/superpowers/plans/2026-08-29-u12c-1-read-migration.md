# U12c-1 — Read Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Point every mechanical `.Aggro` read at the accessors that already exist, so U12c-2 can delete the field behind a seam instead of sweeping 241 sites while also changing behaviour.

**Architecture:** Purely mechanical, in three shapes. `IsInCombat()` and `CurrentCombatTarget()` already exist and already prefer `CombatPhase`; U12c-0 and U12c-0b made them agree with `Aggro` in every state, so this migration moves reads onto seams that are already correct. `ResolveAggroTarget` is re-signatured to take a `state.ActorRef`. Guard-driven exactly like U12b, with an allowlist that shrinks to empty.

**Tech Stack:** Go, `testify/assert` + `require`, `go/ast` for the guard.

**Spec:** [`2026-08-29-u12-unified-targeting-design`](../specs/2026-08-29-u12-unified-targeting-design.md) §6.2

**Branch:** `feature/u12c-1-read-migration` (already created off master).

---

## 0. Facts verified against source

Read at master `e44b77d48` on 2026-08-29, AFTER U12c-0 and U12c-0b merged.

| Fact | Value |
|---|---|
| Non-test `.Aggro` references, whole repo | **311** |
| …excluding `internal/characters/` and `internal/targeting/`, which legitimately read it | **271** |
| **This slice's scope** (nil-checks + target reads + whole-struct) | **~241 sites across 62 files** |
| …`!= nil` / `== nil` | 76 / 41 |
| …`.UserId` / `.MobInstanceId` | 53 / 53 |
| …`.Aggro)` passed whole to `ResolveAggroTarget` | 18 |
| **NOT this slice** (U12c-2 owns them) | `.Type` 16 · `.RoundsWaiting` 20 · `.SpellInfo` 1 |
| Biggest files | `hooks/NewRound_DoCombat.go` 30 · `hooks/NewRound_DoCombat_unified.go` 22 · `actions/cast.go` 15 · `hooks/NewRound_DoCombat_helpers.go` 14 · `behaviortree/actions_archer.go` 14 · `actions/command_readiness.go` 14 · `rooms/rooms.go` 10 |
| `IsInCombat()` | `character.go:746` — `CombatPhase.IsInCombat() \|\| c.Aggro != nil` |
| `CurrentCombatTarget()` | `character.go:783` — `CombatPhase.CurrentTarget()` if non-zero, else the `Aggro` fallback |
| `ResolveAggroTarget` | `func ResolveAggroTarget(aggro *characters.Aggro) AggroTarget` — `actions/combat_helpers.go:23` |

### 0.1 Why the accessors are safe to migrate onto NOW, and were not before

This slice was blocked twice, and both blockers are gone:

- **U12c-0** fixed the stale retarget: `Engaged → Engaging` (and U12c-0b added
  `Engaging → Engaging`), so `CurrentCombatTarget()` follows a target switch
  instead of returning the previous enemy.
- **U12c-0b** made a refused transition refuse the whole commit, so `Aggro` can
  no longer hold a target the machine rejected.

Task 1 proves that equivalence rather than assuming it. **Do not skip it.**
Migrating 241 reads onto an accessor that disagrees with them is how this slice
turns from mechanical into behavioural without anyone noticing.

### 0.2 The three shapes, and nothing else

| # | Before | After |
|---|---|---|
| 1 | `c.Aggro != nil` | `c.IsInCombat()` |
| 2 | `c.Aggro == nil` | `!c.IsInCombat()` |
| 3 | `c.Aggro.UserId` / `c.Aggro.MobInstanceId` | `c.CurrentCombatTarget().UserId` / `.MobInstanceId` |
| 4 | `ResolveAggroTarget(c.Aggro)` | `ResolveAggroTarget(c.CurrentCombatTarget())` |

⚠️ **Leave `.Type`, `.RoundsWaiting` and `.SpellInfo` alone.** They are U12c-2's,
and touching them here mixes a mechanical slice with a behavioural one.

⚠️ **Watch for repeated calls.** `c.Aggro.UserId` and `c.Aggro.MobInstanceId` on
adjacent lines become two `CurrentCombatTarget()` calls. Hoist into a local
`ref := c.CurrentCombatTarget()` when both are read in the same block — the
accessor is cheap but not free, and two calls read worse than one.

⚠️ **A nil-check guarding a field read collapses.** `if c.Aggro != nil &&
c.Aggro.UserId == x` becomes `if c.CurrentCombatTarget().UserId == x`, because
the accessor returns a zero `ActorRef` rather than panicking. Do not mechanically
translate the guard and leave it redundant.

---

## Task 1: Prove the accessors match the raw reads

**Files:**
- Create: `internal/characters/accessor_equivalence_test.go`

- [ ] **Step 1: Write the equivalence test**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// U12c-1 depends entirely on this: IsInCombat() and CurrentCombatTarget() must
// agree with the raw .Aggro reads they are about to replace at ~241 sites.
//
// They did NOT agree before U12c-0 (a retarget left CombatPhase stale) or
// before U12c-0b (a vetoed commit left Aggro holding a rejected target). This
// test is what makes the migration mechanical rather than behavioural.
func TestAccessors_AgreeWithRawAggroReads(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Character)
	}{
		{"idle", func(c *Character) {}},
		{"engaged with a mob", func(c *Character) {
			c.SetAggro(0, 100, DefaultAttack)
		}},
		{"engaged with a player", func(c *Character) {
			c.SetAggro(7, 0, DefaultAttack)
		}},
		{"after a retarget", func(c *Character) {
			c.SetAggro(0, 100, DefaultAttack)
			for i := 0; i < 10; i++ {
				c.CombatPhase.OnRoundTick()
			}
			c.SetAggro(0, 200, DefaultAttack)
		}},
		{"after a release", func(c *Character) {
			c.SetAggro(0, 100, DefaultAttack)
			c.EndAggro()
		}},
		{"surprise engagement", func(c *Character) {
			c.SetAggro(0, 100, SurpriseAttack)
		}},
		{"no combat phase machine", func(c *Character) {
			c.CombatPhase = nil
			c.SetAggro(0, 100, DefaultAttack)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := New()
			tc.setup(c)

			assert.Equal(t, c.Aggro != nil, c.IsInCombat(),
				"IsInCombat() must agree with `Aggro != nil`")

			want := state.ActorRef{}
			if c.Aggro != nil {
				want = state.ActorRef{
					UserId:        c.Aggro.UserId,
					MobInstanceId: c.Aggro.MobInstanceId,
				}
			}
			assert.Equal(t, want, c.CurrentCombatTarget(),
				"CurrentCombatTarget() must agree with the raw Aggro ids")
		})
	}
}

// A vetoed commit is the case U12c-0b fixed. Pinned separately because it is
// the one where the two stores could previously disagree while both non-zero,
// which the table above cannot express.
func TestAccessors_AgreeAfterAVetoedCommit(t *testing.T) {
	c := New()
	c.SetAggro(0, 100, DefaultAttack)
	c.CombatPhase.RegisterTargetLifeCheck(func(state.ActorRef) bool { return false })

	c.SetAggro(0, 200, DefaultAttack)

	require.NotNil(t, c.Aggro)
	assert.Equal(t, 100, c.Aggro.MobInstanceId, "the refused commit changed nothing")
	assert.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId,
		"and the accessor agrees")
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/characters/ -run TestAccessors_ -v`
Expected: PASS, eight subtests plus the veto case.

⚠️ **If any case fails, STOP.** The migration is not safe and the disagreement
must be understood first — that is precisely what this task exists to detect.

- [ ] **Step 3: Commit**

```bash
git add internal/characters/accessor_equivalence_test.go
git commit -m "test(characters): pin accessor equivalence before the read migration"
```

---

## Task 2: The migration guard

Same pattern as U12b: an allowlist that shrinks to empty and fails on stale entries.

**Files:**
- Create: `internal/characters/aggro_reader_guard_test.go`

- [ ] **Step 1: Write the guard**

```go
package characters_test

// U12c-1 read-migration guard. Outside internal/characters and
// internal/targeting, nothing may read a combat target's identity or
// in-combat-ness from the Aggro struct directly: use IsInCombat() and
// CurrentCombatTarget().
//
// It matches ONLY the shapes this slice migrates. Aggro.Type,
// Aggro.RoundsWaiting and Aggro.SpellInfo are U12c-2's and are deliberately
// NOT matched here.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// notYetMigrated lists files still reading Aggro directly. DELETE entries as
// you migrate them.
//
// START IT EMPTY. Step 2 fills it from the guard's own failure output, which
// is the only list guaranteed to match what this matcher sees. Do NOT populate
// it from a grep: grep matches comments, the AST parser (mode 0) drops them, so
// a grep-derived list contains phantom entries that then fail the stale check.
// internal/state/combatphase/combatphase.go is exactly such a phantom.
var notYetMigrated = map[string]bool{}

func internalDirForReaderGuard(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Dir(filepath.Dir(thisFile))
}

// aggroIdentityReaders returns file -> positions of every direct read of
// Aggro's identity or nil-ness outside the two exempt packages.
func aggroIdentityReaders(t *testing.T) map[string][]string {
	t.Helper()
	internalDir := internalDirForReaderGuard(t)
	found := map[string][]string{}

	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(internalDir, path)
		require.NoError(t, relErr)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "characters/") || strings.HasPrefix(rel, "targeting/") {
			return nil
		}

		fset := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Aggro" {
				return true
			}
			// `x.Aggro` reached here. Report it unless the enclosing
			// expression is one of U12c-2's fields.
			found[rel] = append(found[rel], fset.Position(sel.Pos()).String())
			return true
		})
		return nil
	})
	require.NoError(t, err)
	return found
}

func TestNoDirectAggroIdentityReadsOutsideTheSeam(t *testing.T) {
	readers := aggroIdentityReaders(t)

	var unexpected []string
	for file := range readers {
		if !notYetMigrated[file] {
			unexpected = append(unexpected, file)
		}
	}
	sort.Strings(unexpected)
	assert.Empty(t, unexpected,
		"these files read Aggro directly and are not on the U12c-1 allowlist. "+
			"Use IsInCombat() / CurrentCombatTarget(): %s",
		strings.Join(unexpected, ", "))

	var stale []string
	for file := range notYetMigrated {
		if len(readers[file]) == 0 {
			stale = append(stale, file)
		}
	}
	sort.Strings(stale)
	assert.Empty(t, stale,
		"these files are on the U12c-1 allowlist but no longer read Aggro. "+
			"Delete their entries: %s", strings.Join(stale, ", "))
}
```

⚠️ **The matcher above reports EVERY `x.Aggro`, including `.Type`,
`.RoundsWaiting` and `.SpellInfo`, which this slice does not migrate.** That is
deliberate: those files stay on the allowlist until U12c-2 clears them, so the
allowlist will NOT reach empty in this slice. Adjust the done-when accordingly —
U12c-1 finishes when every entry that remains is there **only** for a
U12c-2-owned field.

- [ ] **Step 2: Let the guard populate its own allowlist**

Run: `go test ./internal/characters/ -run TestNoDirectAggroIdentityReads 2>&1 | head -40`

Expected: FAIL, listing every offending file. That list **is** the allowlist,
and it is authoritative because the guard produced it with the same AST matcher
it will use to check. Paste the reported files into `notYetMigrated` as
`"path": true,` entries, then re-run.

Expected on the re-run: PASS, with neither an unexpected nor a stale list.

A grep-derived list would be roughly 63 files, but grep sees comments and the
parser does not, so at least one entry
(`state/combatphase/combatphase.go`) would be a phantom that immediately fails
the stale check. Use the guard's output, not a grep's.

- [ ] **Step 3: Prove the guard can fail**

Add `_ = func(c *characters.Character) bool { return c.Aggro != nil }` to
`internal/mobs/mobs.go`, run the test, confirm it reports `mobs/mobs.go`, then
remove it. `internal/mobs` is chosen because it already imports `characters`
without a cycle.

- [ ] **Step 4: Commit**

```bash
git add internal/characters/aggro_reader_guard_test.go
git commit -m "test(characters): U12c-1 read-migration guard"
```

---

## Task 3: Migrate, one package group per commit

**Every group has the identical five steps.** Repeated in full per group because groups may be done out of order.

- [ ] **Step 1: Enumerate**

```bash
grep -rn "\.Aggro != nil\|\.Aggro == nil\|\.Aggro\.UserId\|\.Aggro\.MobInstanceId\|\.Aggro)" \
  --include=*.go <the group's paths> | grep -v "_test\.go"
```

Read every hit before editing any of them.

- [ ] **Step 2: Apply the shapes from §0.2**

Hoist `ref := c.CurrentCombatTarget()` when a block reads both ids. Collapse
nil-guards that only existed to protect a field read.

- [ ] **Step 3: Delete the file's entries from `notYetMigrated`** — unless the
file still holds a `.Type` / `.RoundsWaiting` / `.SpellInfo` read, in which case
it stays and you note why.

- [ ] **Step 4: Verify**

```bash
gofmt -l internal/ && go build ./... && go test ./internal/characters/ <the packages touched>
```

- [ ] **Step 5: Commit**

### Group A: `internal/rooms` + `internal/goals` + `internal/planners` + `internal/seeders` (19 sites)

Paths: `internal/rooms/ internal/goals/ internal/planners/ internal/seeders/`

These are the least combat-entangled and make a good first pass to shake out the
shapes. `rooms/rooms.go` alone holds 10.

### Group B: `modules/gmcp` (5 sites)

Paths: `modules/gmcp/`

`gmcp.Char.go` reads `Aggro.MobInstanceId` for the client's combat payload;
`gmcp.Room.go` uses `Aggro != nil` twice for an "in combat" flag. Note these are
under `modules/`, which the guard does NOT walk — migrate them anyway, and say
so in the commit, because the guard cannot enforce it.

### Group C: `internal/behaviortree` (34 sites)

Paths: `internal/behaviortree/`

`actions_archer.go` holds 14 of them.

### Group D: `internal/actions` (~40 sites)

Paths: `internal/actions/`

`cast.go` 15, `command_readiness.go` 14, `combat_fire.go` 8, `combat_taunt.go` 5.

⚠️ `melee_target.go` builds a synthetic `&characters.Aggro{...}` to pass to
`ResolveAggroTarget` (`actionTarget()`). Task 4 re-signatures that function; do
this group AFTER Task 4 or leave that one site for it.

### Group E: `internal/combat` + `internal/usercommands` + `internal/mobcommands` (~12 sites)

Paths: `internal/combat/ internal/usercommands/ internal/mobcommands/`

### Group F: `internal/hooks` (~70 sites, the bulk)

Paths: `internal/hooks/`

`NewRound_DoCombat.go` 30, `NewRound_DoCombat_unified.go` 22,
`NewRound_DoCombat_helpers.go` 14, `combat_retarget.go` 8.

⚠️ `combat_retarget.go` reads `attackingPlayer.Character.Aggro` and
`attackingMob.Character.Aggro` to ask "is this actor attacking us?". That is
`IsAggro(userId, mobInstanceId)`, which already exists
(`combat_state_compat.go:179`) and already handles the SpellCast target lists.
Use it rather than hand-rolling the comparison.

---

## Task 4: Re-signature `ResolveAggroTarget`

**Files:**
- Modify: `internal/actions/combat_helpers.go:23`
- Modify: the 18 call sites

- [ ] **Step 1: Change the signature**

```go
// ResolveAggroTarget resolves a combat target reference into the concrete
// actor behind it.
//
// U12c-1: takes a state.ActorRef rather than a *characters.Aggro, so callers
// pass CurrentCombatTarget() and nothing outside internal/characters needs to
// hold the Aggro struct. A zero ref returns Found: false, which is what a nil
// Aggro used to mean.
func ResolveAggroTarget(ref state.ActorRef) AggroTarget {
	if ref.IsZero() {
		return AggroTarget{Found: false}
	}

	// Try mob target first
	if ref.MobInstanceId > 0 {
```

…continuing with `ref.MobInstanceId` and `ref.UserId` in place of
`aggro.MobInstanceId` and `aggro.UserId` throughout the body.

- [ ] **Step 2: Update all 18 call sites**

```bash
grep -rn "ResolveAggroTarget(" --include=*.go internal/ | grep -v "_test\.go"
```

Each `ResolveAggroTarget(x.Aggro)` becomes
`ResolveAggroTarget(x.CurrentCombatTarget())`.

`actions/melee_target.go`'s `actionTarget()` currently builds a synthetic
`&characters.Aggro{UserId: ..., MobInstanceId: ...}` purely to satisfy the old
signature. It becomes:

```go
func (a *stagedMeleeActor) actionTarget() AggroTarget {
	return ResolveAggroTarget(state.ActorRef{
		UserId:        a.target.UserId,
		MobInstanceId: a.target.MobInstanceId,
	})
}
```

which removes the last reason for anything outside `internal/characters` to
construct an `Aggro`.

- [ ] **Step 3: Verify**

Run: `gofmt -l internal/ && go build ./... && go test ./internal/...`
Expected: all green.

- [ ] **Step 4: Commit**

---

## Task 5: Verification, patch notes, boot, PR

- [ ] **Step 1: Full sweep**

Run: `gofmt -l internal/ modules/ main.go && go build ./... && go test ./internal/...`

- [ ] **Step 2: Confirm the remaining allowlist is U12c-2-only**

```bash
grep -rn "\.Aggro != nil\|\.Aggro == nil\|\.Aggro\.UserId\|\.Aggro\.MobInstanceId\|\.Aggro)" \
  --include=*.go internal/ modules/ | grep -v "_test\.go" \
  | grep -v "^internal/characters/\|^internal/targeting/"
echo "(no output = every identity read is migrated)"
```

- [ ] **Step 3: Add a dated patch-notes entry**

This slice changes nothing a player can see. Say so plainly, in one short
paragraph, rather than inventing an improvement — the same framing the U12a and
U12b entries use.

- [ ] **Step 4: Boot test in an isolated detached worktree**

Exit code 124 is the success case.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

- [ ] **Step 5: Push and open the PR, then watch master**

```bash
git push -u origin feature/u12c-1-read-migration
gh pr create --repo pruuk/DOGMud --base master --head feature/u12c-1-read-migration --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

⚠️ After merging, watch **`Build and release` on master**. PR #83 was green here
and master went red.

---

## Done when

1. `TestAccessors_AgreeWithRawAggroReads` passes for every state.
2. No file outside `internal/characters` and `internal/targeting` reads
   `Aggro != nil`, `Aggro == nil`, `Aggro.UserId`, `Aggro.MobInstanceId`, or
   passes `Aggro` whole — verified by the Step 2 grep returning nothing.
3. `ResolveAggroTarget` takes a `state.ActorRef`, and nothing outside
   `internal/characters` constructs an `Aggro`.
4. Every entry left on `notYetMigrated` is there ONLY for a `.Type`,
   `.RoundsWaiting` or `.SpellInfo` read, which U12c-2 owns.
5. `go test ./internal/...` green, boot clean, PR green, **and master green
   after merge**.
6. **No behaviour change.** No guard added or removed, no ordering changed. If a
   test fails, the accessors disagree with the raw reads somewhere and Task 1
   missed a case — STOP and report rather than adjusting the test.
