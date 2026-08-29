# U12b — Targeting Sweep Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move every remaining production `SetAggro` / `EndAggro` call site onto `internal/targeting`, so that outside `internal/characters` and `internal/targeting` nothing writes a combat target directly.

**Architecture:** Purely mechanical. `Commit`, `CommitAfter`, `Release` and `CommitTaunt` already exist and already delegate to `SetAggro`/`EndAggro`, so every migrated site keeps identical behaviour. The work is a call-site rewrite in five known shapes, policed by a repo-wide AST guard whose allowlist shrinks to empty as the sweep proceeds. No design decisions remain open.

**Tech Stack:** Go, `testify/assert` + `require`, `go/ast` + `go/parser` for the guard (the pattern already used by `internal/combat/contest_site_guard_test.go`).

**Spec:** [`2026-08-29-u12-unified-targeting-design`](../specs/2026-08-29-u12-unified-targeting-design.md) §5
**Predecessor:** [`2026-08-29-u12a-targeting-seam`](2026-08-29-u12a-targeting-seam.md), merged as `89d67ff2e` (PR #81)

---

## 0. Facts verified against source

Read at merged HEAD `89d67ff2e` on 2026-08-29. Re-verify before depending on any of it.

| Fact | Value |
|---|---|
| Remaining `SetAggro` sites, non-test, outside `internal/targeting` | **45** |
| Remaining `EndAggro` sites, non-test, outside `internal/targeting` | **44** |
| Total to migrate | **88** across **37 files** (one `SetAggro` and one `EndAggro` are the definitions themselves) |
| Biggest concentrations | `hooks/spell_resolution.go` 13 · `hooks/NewRound_DoCombat_unified.go` 9 · `hooks/combat_retarget.go` 8 · `mobcommands/go.go` 4 · `hooks/Death_InboundAggroCleanup.go` 4 |
| By package | hooks 54 · mobcommands 11 · usercommands 10 · behaviortree 5 · actions 2 · justice 1 · combat 1 · characters 1 |
| Third-argument shapes, production only | `characters.DefaultAttack` ×34 · a variable (`aggroType`, `pvpAggroType`, `mobAggroType`) ×6 · `mob.Character.Aggro.Type` ×1 · one multiline call (`behaviortree/actions_party.go:299`) |
| Variadic (4-arg) sites | **exactly 2**: `hooks/NewRound_DoCombat_helpers.go:1271` and `usercommands/target.go:181`, both passing `1` |
| Production sites passing `Flee` or `SpellCast` | **NONE.** Both appear only in tests (`hooks/hooks_test.go:2076`, `characters/taunt_hold_test.go:98`) |
| In-`characters` caller of `EndAggro` | **1**: `characters/charminfo.go:51`, inside `(*Character).Charm` |
| Guard precedent to copy | `internal/combat/contest_site_guard_test.go` — repo walk, allowlist keyed `file:func` |

### 0.1 Two corrections to the spec, forced by the facts

**1. `SetAggro` and `EndAggro` are NOT deleted.** Spec §5 step 4 and U12a's done-criterion both said they would be. They cannot be: `(*Character).Charm` (`charminfo.go:46-53`) calls `c.EndAggro()` when the charmer was the current target, and `internal/characters` can never import `internal/targeting`. Deleting the methods would leave that with nothing to call.

They instead become what they already are in practice: **the package-internal storage primitives**, which `targeting` drives from outside and `characters` may call from inside. What U12b enforces is the *caller* restriction — no package other than `internal/characters` and `internal/targeting` may call them. That is a stronger and more honest statement than "deleted", because it survives U12c, where the two methods are exactly where the dual-write collapse happens.

**2. No new `Reason` values are needed.** The existing `aggroTypeFor` maps `ReasonSurprise` to `SurpriseAttack` and everything else to `DefaultAttack`. That is sufficient because no production site passes `Flee` or `SpellCast`. Do not add `ReasonFlee`: `Aggro.Type == Flee` is *read* in three places but written only by tests, and inventing a writer would be a behaviour change.

### 0.2 The five call-site shapes, and nothing else

Every one of the 88 sites is one of these. If you meet a sixth, stop and report it rather than improvising.

| # | Before | After |
|---|---|---|
| 1 | `X.SetAggro(0, mobId, characters.DefaultAttack)` | `targeting.Commit(X, state.ActorRef{MobInstanceId: mobId}, targeting.ReasonAttack)` |
| 2 | `X.SetAggro(userId, 0, characters.DefaultAttack)` | `targeting.Commit(X, state.ActorRef{UserId: userId}, targeting.ReasonAttack)` |
| 3 | `X.SetAggro(u, m, someAggroTypeVar)` | `targeting.Commit(X, state.ActorRef{UserId: u, MobInstanceId: m}, targeting.ReasonForAggroType(someAggroTypeVar))` |
| 4 | `X.SetAggro(u, m, t, 1)` | `targeting.CommitAfter(X, state.ActorRef{UserId: u, MobInstanceId: m}, targeting.ReasonForAggroType(t), 1)` |
| 5 | `X.EndAggro()` | `targeting.Release(X, targeting.ReasonDisengage)` |

**Receiver gotcha, in every task:** `users.UserRecord.Character` is already a `*characters.Character`, so pass `user.Character`. `mobs.Mob.Character` is a VALUE, so pass `&mob.Character`. The compiler catches this, but knowing it saves a cycle.

**`Release` ignores its `Reason` today.** Keep passing `ReasonDisengage` anyway. The parameter is not decorative: U12c has to decide what `TransitionToEngaging`/`ForceIdle` do with a `TransitionReason` (spec §6.5), and that decision consumes exactly this argument. Dropping it now would mean touching all 44 sites again later.

---

## Task 1: The sweep guard, with a shrinking allowlist

Write the guard FIRST, allowlisting every file that has not been migrated yet. It stays green throughout, and removing entries is how progress is measured. This is the `contest_site_guard_test.go` pattern.

**Files:**
- Create: `internal/characters/aggro_writer_guard_test.go`

- [ ] **Step 1: Write the guard**

```go
package characters_test

// U12b sweep guard. Outside internal/characters and internal/targeting,
// nothing may write a combat target directly: every engagement goes through
// targeting.Commit / CommitAfter / CommitTaunt / Release.
//
// The allowlist is keyed by FILE and shrinks to empty as the sweep proceeds.
// It is deliberately not keyed by package: a package-level entry would hide a
// newly-added direct write inside an already-listed package, which is the
// failure mode contest_site_guard_test.go was written to avoid.
//
// internal/characters is exempt because it OWNS the storage: (*Character).Charm
// clears aggro internally and cannot import targeting (targeting imports
// characters). internal/targeting is exempt because Commit and Release are
// implemented in terms of these methods.

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

// notYetSwept lists files still holding a direct write. DELETE entries as you
// migrate them. When this is empty, U12b is done.
var notYetSwept = map[string]bool{
	"actions/combat_fire.go":                  true,
	"actions/mutation_cocoon.go":              true,
	"behaviortree/actions_forager.go":         true,
	"behaviortree/actions_mob.go":             true,
	"behaviortree/actions_party.go":           true,
	"combat/combat.go":                        true,
	"hooks/Death_InboundAggroCleanup.go":      true,
	"hooks/Death_MobKillCredit.go":            true,
	"hooks/NewRound_DoCombat.go":              true,
	"hooks/NewRound_DoCombat_helpers.go":      true,
	"hooks/NewRound_DoCombat_unified.go":      true,
	"hooks/NewRound_IdleMobs.go":              true,
	"hooks/NewRound_MobRoundTick.go":          true,
	"hooks/Respawn_PlayerTeleport.go":         true,
	"hooks/charm_spell.go":                    true,
	"hooks/chrysifier_homunculus.go":          true,
	"hooks/combat_retarget.go":                true,
	"hooks/companion_follow.go":               true,
	"hooks/companion_summon.go":               true,
	"hooks/manifester_companions.go":          true,
	"hooks/spell_foldrecall.go":               true,
	"hooks/spell_resolution.go":               true,
	"justice/arrest.go":                       true,
	"mobcommands/attack.go":                   true,
	"mobcommands/break.go":                    true,
	"mobcommands/cast.go":                     true,
	"mobcommands/flee.go":                     true,
	"mobcommands/go.go":                       true,
	"mobcommands/shoot.go":                    true,
	"usercommands/attack.go":                  true,
	"usercommands/break.go":                   true,
	"usercommands/dismiss.go":                 true,
	"usercommands/go.go":                      true,
	"usercommands/shoot.go":                   true,
	"usercommands/target.go":                  true,
	"usercommands/throw.go":                   true,
}

func internalDirForSweepGuard(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must resolve this test file")
	return filepath.Dir(filepath.Dir(thisFile))
}

func TestNoDirectAggroWritesOutsideTheSeam(t *testing.T) {
	internalDir := internalDirForSweepGuard(t)

	offenders := map[string][]string{}

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

		// The two packages that legitimately touch the storage.
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
			if !ok {
				return true
			}
			if sel.Sel.Name == "SetAggro" || sel.Sel.Name == "EndAggro" {
				offenders[rel] = append(offenders[rel],
					fset.Position(sel.Pos()).String())
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)

	var unexpected []string
	for file := range offenders {
		if !notYetSwept[file] {
			unexpected = append(unexpected, file)
		}
	}
	sort.Strings(unexpected)

	assert.Empty(t, unexpected,
		"these files write a combat target directly and are not on the U12b "+
			"allowlist. Use targeting.Commit / CommitAfter / CommitTaunt / "+
			"Release instead: %s", strings.Join(unexpected, ", "))

	// The allowlist must not rot: an entry naming a file that no longer has a
	// direct write is a stale entry hiding nothing, and it must be deleted so
	// the list keeps meaning something.
	var stale []string
	for file := range notYetSwept {
		if len(offenders[file]) == 0 {
			stale = append(stale, file)
		}
	}
	sort.Strings(stale)
	assert.Empty(t, stale,
		"these files are on the U12b allowlist but no longer write aggro "+
			"directly. Delete their entries from notYetSwept: %s",
		strings.Join(stale, ", "))
}
```

- [ ] **Step 2: Run it — it must pass, and the stale check must be empty**

Run: `go test ./internal/characters/ -run TestNoDirectAggroWritesOutsideTheSeam -v`
Expected: PASS. If the stale list is non-empty, the allowlist above has a typo — fix the path rather than deleting the entry.

- [ ] **Step 3: Prove the guard can fail**

Temporarily add `_ = func(c *characters.Character) { c.EndAggro() }` to `internal/skills/skills.go`, run the test, confirm it reports `skills/skills.go`, then remove it. A guard that has never failed is not evidence.

- [ ] **Step 4: Commit**

```bash
git add internal/characters/aggro_writer_guard_test.go
git commit -m "test(characters): U12b sweep guard with a shrinking allowlist"
```

---

## Task 2: The divergence test

**Files:**
- Create: `internal/targeting/divergence_test.go`

- [ ] **Step 1: Write the test**

```go
package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two stores are kept in sync by a dual-write inside SetAggro, an
// invariant held by convention with nothing enforcing it. U12c deletes one of
// them; until then this is what stops them drifting while 88 call sites move.
func TestCommitAndRelease_KeepTheStoresInAgreement(t *testing.T) {
	cases := []struct {
		name string
		act  func(*characters.Character)
		// inCombat is what CombatPhase must report afterwards.
		inCombat bool
	}{
		{"commit to a mob", func(c *characters.Character) {
			Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)
		}, true},
		{"commit to a player", func(c *characters.Character) {
			Commit(c, state.ActorRef{UserId: 7}, ReasonAttack)
		}, true},
		{"commit a surprise", func(c *characters.Character) {
			Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonSurprise)
		}, true},
		{"commit after a wait", func(c *characters.Character) {
			CommitAfter(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack, 1)
		}, true},
		{"commit a taunt", func(c *characters.Character) {
			CommitTaunt(c, state.ActorRef{UserId: 7}, 4)
		}, true},
		{"release", func(c *characters.Character) {
			Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)
			Release(c, ReasonDisengage)
		}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := characters.New()
			require.NotNil(t, c.CombatPhase)

			tc.act(c)

			assert.Equal(t, tc.inCombat, c.CombatPhase.IsInCombat(),
				"CombatPhase must agree with the Aggro write")
			assert.Equal(t, tc.inCombat, c.Aggro != nil,
				"Aggro must agree with the CombatPhase transition")
			if tc.inCombat {
				e := EngagementOf(c)
				assert.False(t, e.Target.IsZero(),
					"a committed engagement must report a target")
			}
		})
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/targeting/ -run TestCommitAndRelease_KeepTheStoresInAgreement -v`
Expected: PASS, six subtests.

- [ ] **Step 3: Commit**

```bash
git add internal/targeting/divergence_test.go
git commit -m "test(targeting): pin the Aggro/CombatPhase dual-write invariant"
```

---

## Tasks 3-9: the sweep, one package group per task

**Every one of these tasks has the identical five steps.** They are repeated in full per task rather than cross-referenced, because tasks may be executed out of order.

For each task:

- [ ] **Step 1: Enumerate the sites**

Run the enumerate command given in the task. Read every hit before editing any of them.

- [ ] **Step 2: Rewrite each site using the shapes in §0.2**

Add `"github.com/GoMudEngine/GoMud/internal/state"` and
`"github.com/GoMudEngine/GoMud/internal/targeting"` to the file's imports if
absent. Remove `"github.com/GoMudEngine/GoMud/internal/characters"` ONLY if the
compiler says it is now unused — several files use it for other things.

Pass `user.Character` for players and `&mob.Character` for mobs.

- [ ] **Step 3: Delete the file's entries from `notYetSwept`**

In `internal/characters/aggro_writer_guard_test.go`.

- [ ] **Step 4: Verify**

Run: `gofmt -l internal/ && go build ./... && go test ./internal/characters/ ./internal/targeting/ <the package(s) touched>`
Expected: gofmt silent, build OK, all tests pass INCLUDING the sweep guard with no stale entries.

- [ ] **Step 5: Commit**

```bash
git add -A internal/
git commit -m "refactor(<pkg>): <n> targeting sites onto the seam"
```

### Task 3: `internal/actions` (2 sites) and `internal/combat` (1 site)

Enumerate: `grep -n "SetAggro(\|EndAggro()" internal/actions/combat_fire.go internal/actions/mutation_cocoon.go internal/combat/combat.go`

`combat/combat.go:409` is the surprise demotion inside `calculateCombat`. It is shape 1/2 with `characters.DefaultAttack`. **Do NOT replace it with `ConsumeOpeningStrike`** — that is U12c's behavioural change, and doing it here would make this slice behavioural.

Note `internal/combat` already imports `internal/targeting` (`targeting_wiring.go`), so no import cycle appears.

### Task 4: `internal/justice` (1 site) and `internal/behaviortree` (5 sites)

Enumerate: `grep -n "SetAggro(\|EndAggro()" internal/justice/arrest.go internal/behaviortree/actions_forager.go internal/behaviortree/actions_mob.go internal/behaviortree/actions_party.go`

`behaviortree/actions_party.go:299` is the one multiline call. It becomes:

```go
	targeting.Commit(&self.Character, state.ActorRef{
		UserId:        leaderChar.CurrentCombatTarget().UserId,
		MobInstanceId: leaderChar.CurrentCombatTarget().MobInstanceId,
	}, targeting.ReasonAttack)
```

Note this calls `CurrentCombatTarget()` twice, exactly as the original did. Do not "improve" it by hoisting into a variable: that is a behaviour change if the accessor is not pure, and checking that is not this slice's job.

### Task 5: `internal/usercommands` (10 sites)

Enumerate: `grep -rn "SetAggro(\|EndAggro()" internal/usercommands/ | grep -v _test`

`usercommands/target.go:181` is one of the two variadic sites — shape 4, `CommitAfter(..., 1)`.
`usercommands/attack.go` passes `aggroType` and `pvpAggroType` — shape 3.

⚠️ `usercommands/attack.go` is a row in `ambush_parity_guard_test.go`'s path table with `setAggroCalls: 2`. That guard already accepts `targeting.Commit`, and rule 2 requires the aggro-type argument to be a bare identifier optionally wrapped in ONE `targeting.ReasonForAggroType(...)`. Shape 3 satisfies it. Run `go test ./internal/actions/ -run TestAmbushParity` as part of Step 4.

### Task 6: `internal/mobcommands` (11 sites)

Enumerate: `grep -rn "SetAggro(\|EndAggro()" internal/mobcommands/ | grep -v _test`

`mobcommands/attack.go` passes `aggroType` and `mobAggroType` — shape 3, and it is also an `ambush_parity_guard_test.go` row with `setAggroCalls: 2`. Run that guard in Step 4.

### Task 7: `internal/hooks`, spell group (14 sites)

Enumerate: `grep -n "SetAggro(\|EndAggro()" internal/hooks/spell_resolution.go internal/hooks/spell_foldrecall.go`

All are shape 1, 2 or 5. `spell_resolution.go` has 13 and is 1719 lines; work top to bottom and re-run the enumerate command afterwards to confirm zero remain.

### Task 8: `internal/hooks`, round group (17 sites)

Enumerate: `grep -n "SetAggro(\|EndAggro()" internal/hooks/NewRound_DoCombat.go internal/hooks/NewRound_DoCombat_unified.go internal/hooks/NewRound_DoCombat_helpers.go internal/hooks/NewRound_MobRoundTick.go internal/hooks/NewRound_IdleMobs.go`

`NewRound_DoCombat_helpers.go:1271` is the second variadic site AND passes `mob.Character.Aggro.Type`, which is not a bare identifier. Shape 4 with the type read hoisted:

```go
	prevType := mob.Character.Aggro.Type
	targeting.CommitAfter(&mob.Character,
		state.ActorRef{UserId: newTargetId},
		targeting.ReasonForAggroType(prevType), 1)
```

Read `mob.Character.Aggro.Type` into `prevType` BEFORE the call: `CommitAfter` overwrites `Aggro`, so evaluating it inline is fine in Go (arguments evaluate first) but the hoist makes the ordering obvious to the next reader.

Leave the `Aggro.Type == characters.Flee` READS at `:907` and `:933` alone. They read; they do not write. U12c owns them.

### Task 9: `internal/hooks`, remaining 23 sites

Enumerate: `grep -rn "SetAggro(\|EndAggro()" internal/hooks/ | grep -v _test`

At this point only `combat_retarget.go` (8), `Death_InboundAggroCleanup.go` (4), `companion_summon.go` (3), `charm_spell.go` (3), `Respawn_PlayerTeleport.go`, `manifester_companions.go`, `Death_MobKillCredit.go`, `companion_follow.go`, `chrysifier_homunculus.go` should remain.

`combat_retarget.go:26-27` READS `Aggro.Type` against `SpellCast` and `Flee`. Leave the reads; migrate only the three writes at `:91`, `:108`, `:123`.

---

## Task 10: Close out the allowlist and document the correction

**Files:**
- Modify: `internal/characters/aggro_writer_guard_test.go`
- Modify: `internal/targeting/context.md`
- Modify: `docs/superpowers/specs/2026-08-29-u12-unified-targeting-design.md`

- [ ] **Step 1: Assert the allowlist is empty**

`notYetSwept` must now be `map[string]bool{}`. Replace the map literal and add above it:

```go
// EMPTY, and it must stay empty. U12b swept all 88 sites. A new entry here is
// not a migration aid any more, it is a hole in the seam: add the call through
// internal/targeting instead.
var notYetSwept = map[string]bool{}
```

- [ ] **Step 2: Run the whole sweep**

Run: `gofmt -l internal/ modules/ main.go && go build ./... && go test ./internal/...`
Expected: gofmt silent, build OK, everything green.

- [ ] **Step 3: Correct the spec**

Spec §5 step 4 says "Delete `SetAggro` and `EndAggro`." Replace with the caller-restriction rule and the reason, per §0.1 above. Also correct U12a's plan done-criterion 9, which says the same thing.

- [ ] **Step 4: Update `internal/targeting/context.md`**

Add to Gotchas: `characters.SetAggro` and `EndAggro` still exist as the storage primitives and are legal to call from `internal/characters` and `internal/targeting` only, enforced by `TestNoDirectAggroWritesOutsideTheSeam`. Name `(*Character).Charm` as the one in-`characters` caller.

- [ ] **Step 5: Commit**

```bash
git add -A internal/ docs/
git commit -m "refactor(targeting): U12b sweep complete, allowlist empty"
```

---

## Task 11: Patch notes, boot test, PR

**Files:**
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Add a dated entry**

Player-facing framing, no raw numbers, no em dashes. Like U12a, this changes nothing a player can see; say so plainly rather than inventing an improvement.

- [ ] **Step 2: Pre-push gate**

```bash
gofmt -l internal/ modules/ main.go     # must print nothing
go build ./...
go test ./internal/...
```

- [ ] **Step 3: Boot test in an isolated detached worktree**

Exit code 124 is the success case.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Clean up with `git worktree remove --force C:/tmp/dogmud-boot-check`; if Windows holds a lock, `rm -rf` then `git worktree prune`.

- [ ] **Step 4: Push and open the PR**

```bash
git push -u origin feature/u12b-targeting-sweep
gh pr create --repo pruuk/DOGMud --base master --head feature/u12b-targeting-sweep --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

---

## Done when

1. `TestNoDirectAggroWritesOutsideTheSeam` passes with `notYetSwept` **empty**, and neither the unexpected nor the stale list can be non-empty.
2. `grep -rn "SetAggro(\|EndAggro()" internal/ modules/ --include=*.go | grep -v _test | grep -v "^internal/characters/\|^internal/targeting/"` returns nothing.
3. `SetAggro` and `EndAggro` still EXIST, called only from `internal/characters` and `internal/targeting`. They are not deleted; §0.1 explains why, and the spec has been corrected.
4. The ambush parity guard still passes on all three of its rows.
5. The divergence test passes for all six commit/release shapes.
6. `go test ./internal/...` green, boot clean, PR green.
7. No behavioural change: no site gained or lost a guard, an ordering, or an argument. `ConsumeOpeningStrike` still has no production caller — that is U12c.
