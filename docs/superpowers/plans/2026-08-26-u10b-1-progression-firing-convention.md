# U10b-1 Progression Firing Convention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give progression one firing rule (one event per resolved command, a loss pays a fraction, crit and fumble stay a bonus layer), wire that rule into every path that awards, move the last uncertain outcomes onto the contest core, and delete three pieces of dead progression machinery.

**Architecture:** The U9 seam (`internal/progression`) already carries per-side ordinary events plus a single `Exceptional` bonus enum, and deliberately deferred WHEN an event fires to each call site. This plan replaces "the call site's existing rules" with one rule, expressed as a `Multiplier` on the ordinary event. Craft and salvage get a designed difficulty basis; everything else keeps its numbers except where the spec says otherwise.

**Tech Stack:** Go, `internal/contest`, `internal/combat`, `internal/progression`, `internal/characters`, `internal/crafting`, `internal/configs`, YAML under `_datafiles/`.

**Spec:** `docs/superpowers/specs/2026-08-26-u10b-1-progression-firing-convention-design.md`

---

## Read this before Task 1

This plan is a rewrite. The previous version failed a three-lens blind adversarial review. These are the findings that killed it; do not reintroduce them.

1. **The seam is not the feature.** The previous plan converted fifteen roll sites and never applied the firing rule to any of them, and added `Outcome.Defended` without populating it in production. Phase B and the "award under the rule" steps in Phase C are the actual deliverable. If a failed craft still teaches nothing at the end, the slice failed no matter how green the build is.
2. **`contest.Result.Success` means the ATTACKER won.** `!res.Success` is NOT "the defender won": under `side.ForceCrit` (a sleeping victim) the attack wins with `Success == false`. Gate on `Defended`. A mirrored test fake passes either way, so this bug does not surface in tests.
3. **`contest.AgainstDifficulty` and `contest.Run` apply NO floor.** `combat.RunContest` is the only place `ContestFloor` is read, and its doc comment forbids routing static-difficulty rolls through it. Craft and salvage therefore call `contest.RunWithFloors` directly with their own floor.
4. **Both `AgainstDifficulty` and `RunWithFloors` are on the root guard's watch list**, and every contest site must be owned. Each conversion adds its `guardedRollExemptions` and `contestSiteOwners` entries **in the same step**, or the build breaks several commits later.
5. **`repoRootForTest` is package-local.** It exists only in `internal/combat`. Go has no cross-package test helpers. Task 1 creates one per package that needs it.
6. **`OnSkillUseScaled` rolls the primary stat at an unscaled 1.0**, and separately grants mutation cluster drift and emits the `SkillUsed` quest event at full weight. Tasks 4 and 5 fix all three.

### Floor semantics, exactly

`RunWithFloors` flips the outcome with probability `f` on **every** call, not only at the extremes:

```
P' = P(1-f) + (1-P)f = P + f(1 - 2P)
```

So `P'` spans `[f, 1-f]`, and `P' == P` at `P = 0.5`. With `CraftFloor 0.05` the extremes land exactly on today's `[5%, 95%]` clamp. The mid-range additionally compresses by `(1 - 2f)`: a true 80% reads as 77%. **Do not describe this as identical to the old clamp.** The extremes match; the interior is smoothed.

A floored result sets `res.Floored`, and `BonusEvents` returns nothing when `Floored` is set, so a floored craft awards its ordinary event and no crit or fumble bonus. That is correct and intended.

## File structure

| File | Responsibility | Task |
|---|---|---|
| `internal/*/testsupport_test.go` (several) | Package-local `repoRootForTest` | 1 |
| `internal/combat/contest_site_guard_test.go` | The AST walker, fixed | 2 |
| `internal/configs/config.balance.go` + siblings | `ProgressionFailureFraction`, `CraftFloor`, `SalvageFloor`, craft and material knobs | 3, 12, 14, 16 |
| `internal/characters/progression.go` | `OnStatUseScaled`, scaled side effects, delete `OnFirstMobKill` | 4, 5, 20 |
| `internal/progression/event.go` | `Outcome.Defended`, `OrdinaryEventsScaled` | 6 |
| `internal/combat/defence_multiplier.go` | Populate `Defended` on the channel path; stale comments | 7, 21 |
| `internal/hooks/NewRound_DoCombat_unified.go` | Populate `Defended` on the melee path | 7 |
| `internal/actions/search.go` | Four difficulty checks, two opposed checks, one award | 8, 9, 10 |
| `internal/actions/track.go`, `internal/forager/forage_core.go` | Convert and award | 11 |
| `internal/items/material_tier.go` (new) | Authored tier to multiplier | 12 |
| `internal/crafting/crafting.go` | Deterministic ingredient selection, craft score and difficulty | 13, 14 |
| `internal/crafting/salvage.go` | Salvage difficulty and floor | 16 |
| `internal/usercommands/go.go` | Delete the stranded mob-follow roll | 19 |
| `internal/progression/seam_guard_test.go` | The standing guard | 22 |

---

# Phase A: foundations

## Task 1: Package-local `repoRootForTest`, and the census fixture

`repoRootForTest` exists once, unexported, in `internal/combat`. Eight other packages need it. Go has no cross-package test helpers, so each gets its own copy.

**Files:**
- Create: `internal/progression/testsupport_test.go`
- Create: `internal/actions/testsupport_test.go`
- Create: `internal/forager/testsupport_test.go`
- Create: `internal/hooks/testsupport_test.go`
- Create: `internal/usercommands/testsupport_test.go`
- Create: `internal/crafting/testsupport_test.go`
- Create: `internal/configs/testsupport_test.go`
- Create: `internal/items/testsupport_test.go`
- Create: `internal/progression/census_test.go`

**Use the same name, `repoRootForTest`, in every package.** They are separate
package-local helpers, so the name cannot collide across packages, and one name
means later tasks never have to remember which variant a package has.
`internal/combat` already has one; do not add a second there.

- [ ] **Step 1: Write the helper, once per package**

Identical body in each file, with the package clause changed to match. For
`internal/progression/testsupport_test.go`:

```go
package progression_test

import (
	"path/filepath"
	"runtime"
	"testing"
)

// repoRootForTest resolves the repository root from this file's own location.
//
// Test binaries do NOT reliably start in the package directory: all tests share
// one binary, so a relative path passes or fails depending on which package ran
// first. Anchor on runtime.Caller instead.
//
// Duplicated per package on purpose. Go test helpers are not visible across
// packages, and the alternative (an exported helper in a non-test file) would
// ship test-only code in the binary.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}
```

For the other five files use `package actions`, `package forager`, `package hooks`,
`package usercommands`, `package crafting` respectively. **Check each package's
existing `_test.go` files first** and match whatever clause they already use
(`x` or `x_test`), or the file will not compile alongside them.

- [ ] **Step 2: Write the census test**

```go
package progression_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProgressionEntryPointCensus pins how many production call sites reach
// progression, so adding one without classifying it against spec section 3.1
// fails here.
//
// This number MOVES during this slice. Tasks that change it say so and update
// it in the same commit.
func TestProgressionEntryPointCensus(t *testing.T) {
	root := repoRootForTest(t)
	entryPoints := []string{
		"OnSkillUseScaled(", "OnSkillUse(", "OnStatUse(", "OnStatUseScaled(",
		"CheckSkillProgression(", "CheckStatProgression(",
		"OnCritReceived(", "TrackSkillUse(", "TrackStatUse(",
		"OnRegenTick(", "CheckRegenProgression(",
	}

	count := 0
	for _, dir := range []string{"internal", "modules"} {
		err := filepath.Walk(filepath.Join(root, dir),
			func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				if !strings.HasSuffix(path, ".go") ||
					strings.HasSuffix(path, "_test.go") {
					return nil
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				for _, line := range strings.Split(string(b), "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "//") {
						continue
					}
					for _, ep := range entryPoints {
						count += strings.Count(line, ep)
					}
				}
				return nil
			})
		if err != nil {
			t.Fatal(err)
		}
	}

	const want = 0 // replaced in Step 4 with the measured number
	if count != want {
		t.Fatalf("progression entry-point call sites = %d, want %d.\n"+
			"If you added a site deliberately, classify it against spec "+
			"section 3.1 and update this number in the same commit.",
			count, want)
	}
}
```

- [ ] **Step 3: Run it to learn the real number**

Run: `go test ./internal/progression/ -run TestProgressionEntryPointCensus -v`
Expected: FAIL, reporting the actual count. Record it.

- [ ] **Step 4: Set `want` and re-run**

Run: `go test ./internal/progression/ -run TestProgressionEntryPointCensus -v`
Expected: PASS

- [ ] **Step 5: Verify every new helper compiles**

Run: `go test ./internal/progression/ ./internal/actions/ ./internal/forager/ ./internal/hooks/ ./internal/usercommands/ ./internal/crafting/ ./internal/configs/ ./internal/items/ 2>&1 | grep -v "^ok"`
Expected: no output. A redeclaration error here means that package already had
a `repoRootForTest`; delete your copy rather than renaming it.

- [ ] **Step 6: Commit**

```bash
git add internal/progression/ internal/actions/testsupport_test.go internal/forager/testsupport_test.go internal/hooks/testsupport_test.go internal/usercommands/testsupport_test.go internal/crafting/testsupport_test.go
git commit -m "test(u10b-1): package-local repoRootForTest and the progression census"
```

---

## Task 2: Make the contest-site guard walker see chained calls

`internal/combat/contest_site_guard_test.go`'s walker does:

```go
case *ast.SelectorExpr:
	consumed[v.Sel] = true
	pkg, ok := v.X.(*ast.Ident)
	if !ok {
		return true
	}
```

It records **package-qualified** calls, where `v.X` is an `*ast.Ident`. That is
correct for `contest.AgainstDifficulty`. It is blind to
`x.Character.OnStatUse(...)`, a selector on a selector, which is the dominant
progression call shape and which Task 22's guard must see.

⚠️ **Do not make the walker match on tail name alone.** That would turn every
`foo.Run(...)` under `internal/` into a "contest site" and bury the engineer in
false positives. Add a *separate* recogniser for the chained shape and leave the
package-qualified path exactly as it is.

**Files:**
- Modify: `internal/combat/contest_site_guard_test.go`

- [ ] **Step 1: Write a test that proves the blindness, on the REAL walker**

```go
// TestWalkerSeesChainedMethodCalls pins the gap fixed in U10b-1. The
// package-qualified path (contest.AgainstDifficulty) was always recorded; a
// method reached through a field (x.Character.OnStatUse) was not, because the
// walker asserted v.X was an *ast.Ident and bailed otherwise. Task 22's guard
// needs the chained shape.
func TestWalkerSeesChainedMethodCalls(t *testing.T) {
	src := `package p
func f(v x) {
	contest.AgainstDifficulty(1, 2)
	v.Character.OnStatUse("dexterity", 1)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]bool{}
	for _, name := range collectQualifiedCalls(file) {
		got[name] = true
	}
	for _, name := range collectChainedMethodCalls(file) {
		got[name] = true
	}

	if !got["contest.AgainstDifficulty"] {
		t.Error("lost the package-qualified shape; that path must not change")
	}
	if !got["OnStatUse"] {
		t.Error("walker is blind to v.Character.OnStatUse: a guard built on it " +
			"ships green while enforcing nothing")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/combat/ -run TestWalkerSeesChainedMethodCalls -v`
Expected: FAIL with "undefined: collectQualifiedCalls" (and `collectChainedMethodCalls`)

- [ ] **Step 3: Add the two recognisers**

```go
// collectQualifiedCalls returns pkg.Fn names for package-qualified calls only.
// This is the existing behaviour, extracted so the chained recogniser below can
// sit beside it without changing it.
func collectQualifiedCalls(node ast.Node) []string {
	var out []string
	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		out = append(out, pkg.Name+"."+sel.Sel.Name)
		return true
	})
	return out
}

// collectChainedMethodCalls returns the tail name of calls reached through a
// field, such as v.Character.OnStatUse -> "OnStatUse".
//
// Deliberately separate from collectQualifiedCalls: matching on tail name alone
// across the whole tree would classify every foo.Run(...) as a contest site.
// Callers pair this with an explicit name set of what they are looking for.
func collectChainedMethodCalls(node ast.Node) []string {
	var out []string
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if _, isIdent := sel.X.(*ast.Ident); isIdent {
			return true // package-qualified; the other collector owns it
		}
		out = append(out, sel.Sel.Name)
		return true
	})
	return out
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/combat/ -run TestWalkerSeesChainedMethodCalls -v`
Expected: PASS

- [ ] **Step 5: Confirm the existing guards are untouched**

Run: `go test ./internal/combat/ -run "TestEveryContestSiteIsOwned|TestEveryChannelUsesUniformDefenceSkillWeight|TestNoLegacySkillWeightLiteralSurvives" -v`
Expected: PASS, all three, unchanged.

⚠️ Use these exact names. `-run Guard` matches none of them and prints
`no tests to run`, which reads as success.

- [ ] **Step 6: Commit**

```bash
git add internal/combat/contest_site_guard_test.go
git commit -m "test(u10b-1): recognise chained method calls without breaking qualified ones"
```

---

## Task 3: `ProgressionFailureFraction`, with a pre-unmarshal sentinel

An absent YAML key unmarshals to `0`, which is a **legal** value for this knob
(an explicit off-switch). The usual `if x < 0 || x > 1.0` guard can therefore
never distinguish "unset" from "deliberately zero", and the knob would ship at
zero with failure paying nothing.

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.progression.go`
- Modify: `internal/configs/configs.go` (the pre-unmarshal construction point)
- Modify: `_datafiles/config.yaml`
- Create: `internal/configs/progression_failure_fraction_test.go`

- [ ] **Step 1: Find the pre-unmarshal construction point**

Run: `grep -n "tmpConfigData\|Config{}" internal/configs/configs.go | head`

`ReloadConfig` builds a fresh `Config{}` before unmarshalling. That is where the
sentinel is seeded. Note its line number; Step 4 edits it.

- [ ] **Step 2: Write the failing tests**

```go
package configs

import (
	"testing"

	"gopkg.in/yaml.v2"
)

// seedAndLoad mirrors what ReloadConfig must do: seed the sentinel BEFORE
// unmarshal, so that afterwards -1 means the key was absent and any value in
// [0,1] means it was present.
func seedAndLoad(t *testing.T, doc string) Balance {
	t.Helper()
	b := Balance{ProgressionFailureFraction: -1}
	if err := yaml.Unmarshal([]byte(doc), &b); err != nil {
		t.Fatal(err)
	}
	b.Validate()
	return b
}

func TestProgressionFailureFractionDefaultsWhenAbsent(t *testing.T) {
	b := seedAndLoad(t, "RollSpread: 0.15\n")
	if got := float64(b.ProgressionFailureFraction); got != 0.35 {
		t.Fatalf("with the key ABSENT = %v, want 0.35", got)
	}
}

func TestProgressionFailureFractionZeroIsHonoured(t *testing.T) {
	b := seedAndLoad(t, "ProgressionFailureFraction: 0\n")
	if got := float64(b.ProgressionFailureFraction); got != 0 {
		t.Fatalf("explicit 0 became %v; the off-switch must be honoured", got)
	}
}

// TestReloadConfigSeedsTheSentinel is the one that actually proves production
// is wired. The two tests above only prove Validate() behaves correctly given a
// seeded struct; without this, both pass while ReloadConfig never seeds and the
// knob ships inert.
func TestReloadConfigSeedsTheSentinel(t *testing.T) {
	root := repoRootForTest(t)
	b, err := os.ReadFile(filepath.Join(root, "internal", "configs", "configs.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "ProgressionFailureFraction: -1") {
		t.Fatal("ReloadConfig does not seed the ProgressionFailureFraction " +
			"sentinel before unmarshal, so an absent key is indistinguishable " +
			"from an explicit 0 and the knob ships inert")
	}
}
```

Task 1 created this package's copy.

- [ ] **Step 3: Run to verify all three fail**

Run: `go test ./internal/configs/ -run ProgressionFailureFraction -v; go test ./internal/configs/ -run TestReloadConfigSeedsTheSentinel -v`
Expected: FAIL, "b.ProgressionFailureFraction undefined"

- [ ] **Step 4: Declare, seed, default**

In `config.balance.go`, beside `CritProgressionBonus`:

```go
// ProgressionFailureFraction is the share of an ordinary progression event a
// RESOLVED LOSS earns. A win pays 1.0.
//
// Uses a -1 "unset" sentinel seeded before unmarshal, because 0 is a legal
// value here (an explicit off-switch) and an ABSENT yaml key also unmarshals to
// 0. The usual `if x < 0 || x > 1.0` guard cannot tell them apart, so without
// the sentinel this knob ships inert. See the U10b-1 spec section 5.6.
ProgressionFailureFraction ConfigFloat `yaml:"ProgressionFailureFraction"`
```

In `configs.go`, at the construction point from Step 1:

```go
tmpConfigData := Config{}
// U10b-1: 0 is a legal value for this knob, so an absent key must arrive as
// something else. See the field's doc comment.
tmpConfigData.GamePlay.Balance.ProgressionFailureFraction = -1
```

Adjust the field path to match the real struct nesting.

In `validateProgression()`:

```go
// -1 is the "unset" sentinel seeded by ReloadConfig. Anything in [0,1] is
// taken at face value, including an explicit 0.
if b.ProgressionFailureFraction < 0 || b.ProgressionFailureFraction > 1.0 {
	b.ProgressionFailureFraction = 0.35
}
```

- [ ] **Step 5: Run to verify all three pass**

Run: `go test ./internal/configs/ -v`
Expected: PASS

- [ ] **Step 6: Add the knob to `_datafiles/config.yaml`**

```yaml
  ProgressionFailureFraction: 0.35   # Share of an event a resolved LOSS earns
```

⚠️ `_datafiles/config.yaml` has `skip-worktree` set. Do not `git add` it from
disk in this or any later task; build the committed version from the
`git show HEAD:` blob per CLAUDE.md, or the commit will carry unrelated local
drift.

- [ ] **Step 7: Commit**

```bash
git add internal/configs/
git commit -m "feat(u10b-1): ProgressionFailureFraction, defaulted past the zero-key trap"
```

---

## Task 4: `OnStatUseScaled`, so the stat half honours the multiplier

**Files:**
- Modify: `internal/characters/progression.go`
- Create: `internal/characters/progression_scaled_test.go`

- [ ] **Step 1: Write the failing test**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// TestOnStatUseScaledRespectsMultiplier pins that the stat half of an ordinary
// progression event honours the event's Multiplier. Before U10b-1 a resolved
// LOSS paid a fractional skill roll and a FULL stat roll, because OnStatUse
// hardcoded 1.0 into CheckStatProgression.
func TestOnStatUseScaledRespectsMultiplier(t *testing.T) {
	configs.SetConfigForTest(t)

	const trials = 600

	zero := newProgressionTestCharacter(t)
	full := newProgressionTestCharacter(t)
	for i := 0; i < trials; i++ {
		zero.OnStatUseScaled("dexterity", 0, 0.0)
		full.OnStatUseScaled("dexterity", 0, 1.0)
	}

	if got := zero.Stats.Dexterity.Training; got != 0 {
		t.Fatalf("multiplier 0.0 advanced dexterity training to %d; the "+
			"multiplier is being ignored", got)
	}
	if got := full.Stats.Dexterity.Training; got == 0 {
		t.Fatal("multiplier 1.0 never advanced dexterity training over 600 " +
			"trials; the test fixture is not exercising progression")
	}
}
```

⚠️ Both assertions matter. The first alone would pass against a function that
never progresses anything.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/characters/ -run TestOnStatUseScaledRespectsMultiplier -v`
Expected: FAIL with "zero.OnStatUseScaled undefined"

- [ ] **Step 3: Add `OnStatUseScaled`, route `OnStatUse` through it**

```go
// OnStatUseScaled is OnStatUse with an explicit progression multiplier.
//
// It exists because an ordinary progression event carries a Multiplier and the
// stat half used to ignore it: a resolved LOSS paid a fractional skill roll and
// a full-rate stat roll. See the U10b-1 spec section 3.3.
func (c *Character) OnStatUseScaled(statName string, userId int, multiplier float64) bool {
	c.TrackStatUse(statName)
	mudlog.Debug("Progression", "event", "stat_use", "stat", statName,
		"multiplier", fmt.Sprintf("%.2f", multiplier), "character", c.Name)

	if configs.GetGamePlayConfig().UseSkillProgression {
		return c.CheckStatProgression(statName, userId, multiplier)
	}
	return false
}

// OnStatUse is OnStatUseScaled at full weight.
func (c *Character) OnStatUse(statName string, userId int) bool {
	return c.OnStatUseScaled(statName, userId, 1.0)
}
```

- [ ] **Step 4: Make `OnSkillUseScaled`'s primary-stat call honour the multiplier**

```go
	// Auto-track and progress the skill's primary governing stat, at the SAME
	// weight as the skill. This used to be a bare OnStatUse, so a fractional
	// skill award still paid a full-rate stat award.
	if primaryStat := skills.GetSkillPrimaryStat(skillName); primaryStat != "" {
		c.OnStatUseScaled(primaryStat, userId, bonusMultiplier)
	}
```

⚠️ Existing callers pass multipliers **above** 1.0: crit bonus events pass
`CritProgressionBonus` (2.0 shipped) and `actGrantProgression` passes **1000.0**.
After this change those scale the stat too. That is consistent with "both halves
take the fraction", but it is a behaviour change on the bonus path. Note it in
the commit body and re-check `TestCritReceivedProgression_DecaysWithRank` still
passes.

- [ ] **Step 5: Run the package**

Run: `go test ./internal/characters/ -v`
Expected: PASS

If a test fails because a scaled skill award no longer pays a full stat roll,
that is the intended change; update the assertion only where it pinned the old
asymmetry.

- [ ] **Step 6: Update the census**

Task 1's count changed. Re-run, record, update `want` in the same commit.

- [ ] **Step 7: Commit**

```bash
git add internal/characters/ internal/progression/census_test.go
git commit -m "feat(u10b-1): scale the stat half of an ordinary progression event"
```

---

## Task 5: Scale the two unscaled side effects

`OnSkillUseScaled` also grants **mutation cluster drift** at a full
`MutationAffinityPerSkillUse` and emits the **`SkillUsed` quest event**, both
unconditionally. Awarding on losses roughly doubles how often it is called, so
untouched this roughly doubles mutation acquisition and turns every "use this
skill N times" quest into "fail at it N times".

**Files:**
- Modify: `internal/characters/progression.go`
- Create: `internal/characters/progression_side_effects_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestFractionalAwardScalesMutationDrift pins that a fractional progression
// event grants a fractional share of mutation cluster affinity. Without this a
// resolved loss pays a full affinity tick, roughly doubling mutation
// acquisition across the game.
func TestFractionalAwardScalesMutationDrift(t *testing.T) {
	configs.SetConfigForTest(t)

	full := newProgressionTestCharacter(t)
	frac := newProgressionTestCharacter(t)

	full.OnSkillUseScaled("weapon-combat", 0, 1.0)
	frac.OnSkillUseScaled("weapon-combat", 0, 0.35)

	fullAff := totalClusterAffinityForTest(full)
	fracAff := totalClusterAffinityForTest(frac)

	if fullAff == 0 {
		t.Fatal("full award granted no cluster affinity; fixture is not " +
			"exercising the drift path")
	}
	if fracAff >= fullAff {
		t.Fatalf("fractional award granted %v affinity vs %v for a full "+
			"award; the multiplier is being ignored", fracAff, fullAff)
	}
}
```

Write `totalClusterAffinityForTest` in the same file, summing whatever the
character exposes for cluster affinity. Confirm the field name first:

Run: `grep -n "AddClusterAffinity" -A 4 internal/characters/*.go | head -20`

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/characters/ -run TestFractionalAwardScalesMutationDrift -v`
Expected: FAIL, fractional affinity equals full affinity

- [ ] **Step 3: Scale the drift**

```go
	// Mutation-graph drift: cluster-relevant skill use nudges affinity, at the
	// SAME weight as the event. A resolved loss must not buy a full tick, or
	// the U10b-1 failure fraction roughly doubles mutation acquisition.
	if clusters := mutations.ClustersForSkill(skillName); clusters != nil {
		amt := float64(configs.GetBalanceConfig().MutationAffinityPerSkillUse) * bonusMultiplier
		for _, cl := range clusters {
			c.AddClusterAffinity(cl, amt)
		}
	}
```

- [ ] **Step 4: Decide and implement the quest-event rule**

The `SkillUsed` event feeds quest `skill_use` counters, which are integers. A
fractional counter is meaningless, so scaling the magnitude is not available.
**Emit it only for a full-weight event:**

```go
	// Quest skill_use counters are integers, so a fractional award cannot pay a
	// fractional tick. A resolved LOSS does not advance a "use this skill N
	// times" quest -- otherwise such a quest becomes "fail at it N times".
	if bonusMultiplier >= 1.0 {
		events.AddToQueue(events.SkillUsed{UserId: userId, Skill: skillName})
	}
```

Confirm the real event construction before editing:

Run: `grep -n "SkillUsed{" internal/characters/progression.go`

- [ ] **Step 5: Add a test for the quest gate**

```go
// TestFractionalAwardDoesNotAdvanceSkillUseQuests pins that a resolved loss
// does not tick an integer quest counter. "Use this skill N times" must not
// become "fail at it N times".
func TestFractionalAwardDoesNotAdvanceSkillUseQuests(t *testing.T) {
	configs.SetConfigForTest(t)

	before := drainSkillUsedEventsForTest(t)
	newProgressionTestCharacter(t).OnSkillUseScaled("weapon-combat", 1, 0.35)
	after := drainSkillUsedEventsForTest(t)

	if after != before {
		t.Fatalf("a fractional award emitted %d SkillUsed events", after-before)
	}
}
```

Implement `drainSkillUsedEventsForTest` against the real event queue API; check
how existing tests in this repo observe queued events before inventing one.

- [ ] **Step 6: Run**

Run: `go test ./internal/characters/ ./internal/quests/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/characters/
git commit -m "feat(u10b-1): scale mutation drift and gate quest ticks on full-weight events"
```

---

## Task 6: `Outcome.Defended` and `OrdinaryEventsScaled`

**Files:**
- Modify: `internal/progression/event.go`
- Create: `internal/progression/event_scaled_test.go`

⚠️ `internal/progression/event_test.go` is `package progression`. Put this in a
new file declared `package progression_test` so the qualified references below
compile, or drop the qualifiers.

- [ ] **Step 1: Write the failing tests**

```go
package progression_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/progression"
)

func TestOrdinaryEventsScalesTheLoser(t *testing.T) {
	o := progression.Outcome{
		AttackerSkill: "weapon-combat",
		DefenderSkill: "dodge",
		Defended:      true, // the DEFENDER won, so the attacker lost
	}

	got := map[progression.Side]float64{}
	for _, e := range progression.OrdinaryEventsScaled(o, 0.35) {
		got[e.Side] = e.Multiplier
	}

	if got[progression.SideAttacker] != 0.35 {
		t.Errorf("attacker (loser) multiplier = %v, want 0.35", got[progression.SideAttacker])
	}
	if got[progression.SideDefender] != 1.0 {
		t.Errorf("defender (winner) multiplier = %v, want 1.0", got[progression.SideDefender])
	}
}

// TestOrdinaryEventsScalesTheDefenderWhenTheAttackWon is the mirror. Without
// it, a stub that returns OrdinaryEvents unchanged passes the test above's
// defender assertion by accident, since OrdinaryEvents hardcodes 1.0.
func TestOrdinaryEventsScalesTheDefenderWhenTheAttackWon(t *testing.T) {
	o := progression.Outcome{
		AttackerSkill: "weapon-combat",
		DefenderSkill: "dodge",
		Defended:      false, // the ATTACKER won
	}

	got := map[progression.Side]float64{}
	for _, e := range progression.OrdinaryEventsScaled(o, 0.35) {
		got[e.Side] = e.Multiplier
	}

	if got[progression.SideAttacker] != 1.0 {
		t.Errorf("attacker (winner) multiplier = %v, want 1.0", got[progression.SideAttacker])
	}
	if got[progression.SideDefender] != 0.35 {
		t.Errorf("defender (loser) multiplier = %v, want 0.35", got[progression.SideDefender])
	}
}
```

- [ ] **Step 2: Run to verify both fail**

Run: `go test ./internal/progression/ -run OrdinaryEventsScale -v`
Expected: FAIL, "undefined: progression.OrdinaryEventsScaled"

- [ ] **Step 3: Add the field and the function**

Add to `Outcome`, beside `Floored`:

```go
	// Defended reports that the DEFENDER won this contest.
	//
	// Carried rather than derived because contest.Result.Success means the
	// ATTACKER won, and !Success is NOT "the defender won": under
	// side.ForceCrit the attack wins with Success == false. Every attempt to
	// reconstruct this predicate at a call site has been a bug, and a mirrored
	// test fake passes either way. Set it from combat's out.Defended.
	Defended bool
```

```go
// OrdinaryEventsScaled is OrdinaryEvents with the U10b-1 firing rule applied:
// the side that LOST a resolved contest earns failureFraction of an event
// rather than nothing.
//
// The winner is decided by o.Defended alone. See that field's doc comment.
func OrdinaryEventsScaled(o Outcome, failureFraction float64) []Event {
	evs := OrdinaryEvents(o)
	for i := range evs {
		lost := (evs[i].Side == SideAttacker && o.Defended) ||
			(evs[i].Side == SideDefender && !o.Defended)
		if lost {
			evs[i].Multiplier = failureFraction
		}
	}
	return evs
}
```

- [ ] **Step 4: Run to verify both pass**

Run: `go test ./internal/progression/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/progression/
git commit -m "feat(u10b-1): a resolved loss earns a fraction of an ordinary event"
```

---

# Phase B: wire the rule into combat

## Task 7: Populate `Defended` in production

Without this the fraction reaches only hand-converted sites, and the melee,
channel and spell paths keep awarding win-only. This is the task the previous
plan omitted entirely.

**Files:**
- Modify: `internal/combat/defence_multiplier.go`
- Modify: `internal/hooks/NewRound_DoCombat_unified.go`
- Create: `internal/hooks/defended_wiring_test.go`

- [ ] **Step 1: Find every `Outcome` construction in production**

Run: `grep -rn "progression.Outcome{" internal/ --include=*.go | grep -v _test.go`

Every one of these must either set `Defended` or be justified in the commit as
a site with no defender.

- [ ] **Step 2: Write the failing test**

```go
// TestEveryProductionOutcomeSetsDefended pins that no production Outcome is
// built without deciding who won. Outcome.Defended drives the U10b-1 failure
// fraction, and an unset field silently means "the attacker won", which awards
// a full event to a side that lost.
func TestEveryProductionOutcomeSetsDefended(t *testing.T) {
	root := repoRootForTest(t)

	// Sites with genuinely no defender. Each needs a written reason.
	exempt := map[string]string{
		"internal/actions/combat_fire.go":        "self-targeted, no defender",
		"internal/combat/skill_moves.go":         "defender-only award, no contest side",
		"internal/hooks/combat_shared_helpers.go": "spellcasting practice award, no defender",
	}

	for _, hit := range findOutcomeLiteralsForTest(t, root) {
		if _, ok := exempt[hit.File]; ok {
			continue
		}
		if !hit.SetsDefended {
			t.Errorf("%s:%d builds a progression.Outcome without setting "+
				"Defended; the failure fraction cannot be applied and the "+
				"losing side is paid as a winner", hit.File, hit.Line)
		}
	}
}
```

Implement `findOutcomeLiteralsForTest` with `go/parser`, using
`collectQualifiedCalls`'s style from Task 2: find `progression.Outcome`
composite literals and report whether a `Defended` key appears. Verify the
exemption list against Step 1's grep before trusting the three entries above.

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/hooks/ -run TestEveryProductionOutcomeSetsDefended -v`
Expected: FAIL, naming the melee and channel sites

- [ ] **Step 4: Set `Defended` on the channel path**

`defence_multiplier.go` already computes `out.Defended`. Carry it into the
`Outcome` it builds, and switch the award from `OrdinaryEvents` to
`OrdinaryEventsScaled` with the configured fraction.

- [ ] **Step 5: Set `Defended` on the melee path**

In `NewRound_DoCombat_unified.go`, the attacker and defender `Outcome`s are
built from `AttackResult`. Derive `Defended` from the same swing data
`defenceTypesUsed` reads, never from `!Success`.

- [ ] **Step 6: Run**

Run: `go test ./internal/hooks/ ./internal/combat/ ./internal/progression/ -v`
Expected: PASS

- [ ] **Step 7: Verify the rule actually reaches a losing swing**

```go
// TestLosingSwingAwardsTheFraction is the end-to-end proof that the slice's
// headline rule is wired, not merely available.
func TestLosingSwingAwardsTheFraction(t *testing.T) {
	// Drive a defended swing and assert the attacker's ordinary event carries
	// ProgressionFailureFraction, not 1.0 and not absent.
}
```

Fill this in against the real combat fixtures in `internal/hooks`. Do not skip
it: without it, Tasks 4 through 7 are provably wired only in unit isolation.

- [ ] **Step 8: Commit**

```bash
git add internal/combat/defence_multiplier.go internal/hooks/
git commit -m "feat(u10b-1): populate Defended and award the fraction on losing sides"
```

---

# Phase C: convert and wire the static-difficulty sites

## Task 8: `search.go`'s four difficulty checks

**Files:**
- Modify: `internal/actions/search.go`
- Modify: `contest_floor_guard_test.go` (repo root)
- Modify: `internal/combat/contest_site_guard_test.go`
- Create: `internal/actions/search_contest_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestSearchDifficultyChecksUseTheCore pins that search's four
// non-actor checks resolve through the contest core. The two hidden-ACTOR
// checks become opposed contests in Task 9 and are not counted here.
func TestSearchDifficultyChecksUseTheCore(t *testing.T) {
	root := repoRootForTest(t)
	b, err := os.ReadFile(filepath.Join(root, "internal", "actions", "search.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	if got := strings.Count(src, "contest.AgainstDifficulty("); got != 4 {
		t.Errorf("contest.AgainstDifficulty count = %d, want 4", got)
	}
	if got := strings.Count(src, "dice.RollStat(searchScore)"); got != 2 {
		t.Errorf("dice.RollStat(searchScore) count = %d, want 2 "+
			"(the hidden-actor sites, converted in Task 9)", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/actions/ -run TestSearchDifficultyChecksUseTheCore -v`
Expected: FAIL, counts 0 and 6

- [ ] **Step 3: Convert the four**

Secret exits and hidden containers (125), stashed items (135), hidden nouns
(175). Each becomes:

```go
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 125.0).Success {
```

with the threshold per site: `125.0`, `125.0`, `135.0`, `175.0`.

- [ ] **Step 4: Add the guard entries IN THIS STEP**

In `contest_floor_guard_test.go`, under `guardedRollExemptions["contest"]`:

```go
"internal/actions/search.go": "U10b-1: static-difficulty checks, deliberately unfloored per RunContest's scope comment",
```

In `internal/combat/contest_site_guard_test.go`, under `contestSiteOwners`, add
one entry per `file:func` the guard reports. Run the guard to learn the exact
keys rather than guessing them:

Run: `go test . ./internal/combat/ -run "Floored|Owned" -v`

- [ ] **Step 5: Run both guards and the package**

Run: `go test . ./internal/combat/ ./internal/actions/ -run "Floored|Owned|Search" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/actions/search.go internal/actions/search_contest_test.go contest_floor_guard_test.go internal/combat/contest_site_guard_test.go
git commit -m "refactor(u10b-1): search's difficulty checks resolve on the contest core"
```

---

## Task 9: A hider's skill counts when someone searches for them

The two hidden-actor checks use a flat threshold that never reads the hider's
score, while `usercommands/go.go` resolves the same question as an opposed
contest. Mobs reach the broken path via `behaviortree/actions_scout.go`'s
`actTrySearch`.

**This is the slice's deliberate behaviour change.**

**Files:**
- Modify: `internal/actions/search.go`
- Modify: `internal/actions/skill_helpers.go` (or wherever the shared score lands)
- Create: `internal/actions/search_hidden_test.go`

- [ ] **Step 1: Read `go.go`'s existing composition and decide where it lives**

Run: `grep -n "hiddenScore\|observerScore" internal/usercommands/go.go`

`go.go` has **four** hidden-detection sites. Read them, confirm they share one
score composition, and **lift that composition into a shared exported helper**
that both `go.go` and `search.go` call. Do not write a second copy in
`internal/actions`: two implementations disagreeing is the bug this task
exists to fix, and duplicating it would merely move the disagreement.

Record in the commit which package the helper landed in and why.

- [ ] **Step 2: Write the failing test**

```go
// TestSearchHiddenDetectionReadsTheHidersScore pins the U10b-1 fix: search used
// to answer "does the observer spot the hider?" against a flat threshold that
// ignored the hider, while go.go resolved the same question as an opposed
// contest.
func TestSearchHiddenDetectionReadsTheHidersScore(t *testing.T) {
	configs.SetConfigForTest(t)
	const trials = 500

	weakSpotted, strongSpotted := 0, 0
	for i := 0; i < trials; i++ {
		if combat.RunContest(150, []contest.Entry{{Score: 50}}).Success {
			weakSpotted++
		}
		if combat.RunContest(150, []contest.Entry{{Score: 250}}).Success {
			strongSpotted++
		}
	}

	if strongSpotted >= weakSpotted {
		t.Fatalf("skilled hider spotted %d/%d vs unskilled %d/%d: the hider's "+
			"score is not being read", strongSpotted, trials, weakSpotted, trials)
	}
}
```

Then add a second test asserting `search.go` no longer contains
`dice.RollStat(searchScore)` at all, so the source-level change is pinned too.

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/actions/ -run TestSearchHiddenDetectionReadsTheHidersScore -v`
Expected: FAIL on the source-level assertion

- [ ] **Step 4: Convert both sites**

```go
		rolledAgainstSomething = true
		if combat.RunContest(searchScore,
			[]contest.Entry{{Score: HiddenActorScore(&p.Character)}}).Success {
```

and the same for the mob loop with `&m.Character`.

⚠️ These are **opposed** contests, so `combat.RunContest` is correct here and
they need no `guardedRollExemptions` entry. They do need `contestSiteOwners`
entries.

- [ ] **Step 5: Run everything that touches this path**

Run: `go test ./internal/actions/ ./internal/usercommands/ ./internal/behaviortree/ ./internal/combat/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/actions/ internal/usercommands/ internal/combat/contest_site_guard_test.go
git commit -m "fix(u10b-1): a hider's skill counts when someone searches for them"
```

---

## Task 10: `search` awards under the rule, ONE event per command

`search.go` fires **one** `OnSkillUse(Search)` per invocation today, gated on
`rolledAgainstSomething`. Six rolls must still pay one event, or a rich room
pays five times a bare one.

**Files:**
- Modify: `internal/actions/search.go`
- Create: `internal/actions/search_award_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestSearchAwardsOneEventPerCommand pins spec section 5.5.2: a command that
// runs several rolls awards ONE event, not one per roll. A room with five
// hidden things must not pay five times a room with one.
func TestSearchAwardsOneEventPerCommand(t *testing.T) {
	// Drive Search against a room with several hidden things and assert
	// exactly one Search award, then against a room with one and assert the
	// same count.
}

// TestSearchThatFindsNothingStillAwardsTheFraction pins the firing rule: a
// command that resolved at least one roll and won none pays
// ProgressionFailureFraction, not nothing.
func TestSearchThatFindsNothingStillAwardsTheFraction(t *testing.T) {
	// Drive Search against a room with a hidden thing far above the searcher's
	// score and assert the award carries the fraction.
}
```

Fill both in against the real fixtures in `internal/actions`; `search_test.go`
already builds rooms and actors, so follow its setup rather than inventing one.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/actions/ -run TestSearch.*Award -v`
Expected: FAIL

- [ ] **Step 3: Replace the award**

```go
	// U10b-1: ONE event per resolved COMMAND, not per internal roll. search
	// runs up to six checks; a rich room must not pay six times a bare one.
	// A command that rolled and won nothing pays the failure fraction.
	if rolledAgainstSomething {
		mult := 1.0
		if !foundAnything {
			mult = float64(configs.GetBalanceConfig().ProgressionFailureFraction)
		}
		actor.OnSkillUseScaled(string(skills.Search), actor.GetUserId(), mult)
	}
```

Set `foundAnything` where each check succeeds, alongside the existing
`result.*Found` appends.

- [ ] **Step 4: Run**

Run: `go test ./internal/actions/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/actions/search.go internal/actions/search_award_test.go
git commit -m "feat(u10b-1): search awards one event per command, fraction on a fruitless search"
```

---

## Task 11: `track` and `forage` convert and award

**Files:**
- Modify: `internal/actions/track.go`
- Modify: `internal/forager/forage_core.go`
- Modify: `contest_floor_guard_test.go`, `internal/combat/contest_site_guard_test.go`
- Create: `internal/actions/track_contest_test.go`, `internal/forager/forage_contest_test.go`

- [ ] **Step 1: Write the failing tests**

For each file, assert the bare `dice.RollStat` threshold is gone, that
`contest.AgainstDifficulty` is reached, that the `NOTE(unassigned` breadcrumb is
removed, and that a resolved failure awards the fraction rather than nothing.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/actions/ ./internal/forager/ -run "Track|Forage" -v`
Expected: FAIL

- [ ] **Step 3: Convert `track.go`**

⚠️ `track.go` grades a trail against **two** thresholds (125 and 175) from a
**single** roll, and stores it in `result.RollValue` for tier introspection.
Preserve that shape: take **one** `contest.AgainstDifficulty(searchScore, 125.0)`
and grade the tier from its `Margin`, rather than rolling twice. Two rolls would
change the tier distribution silently. Update `RollValue`'s doc comment to say
what it now holds, or remove it if nothing reads it:

Run: `grep -rn "RollValue" internal/ --include=*.go | grep -v _test.go`

- [ ] **Step 4: Convert `forage_core.go`**

Replace the threshold comparison with
`contest.AgainstDifficulty(a.SearchScore, biomeDifficulty).Success`, keeping the
existing per-biome difficulty unchanged.

- [ ] **Step 5: Award under the rule in both**

Both currently award unconditionally or on a find. Make each award once per
command, at full weight on a find and `ProgressionFailureFraction` otherwise.

- [ ] **Step 6: Guard entries, then run**

Add both files to `guardedRollExemptions["contest"]` with reasons and their
`file:func` keys to `contestSiteOwners`.

Run: `go test . ./internal/combat/ ./internal/actions/ ./internal/forager/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/actions/track.go internal/forager/ contest_floor_guard_test.go internal/combat/contest_site_guard_test.go
git commit -m "refactor(u10b-1): track and forage on the core, awarding under the rule"
```

---

## Task 12: The authored material tier

**Files:**
- Modify: `internal/items/itemspec.go`
- Create: `internal/items/material_tier.go`
- Create: `internal/items/material_tier_test.go`
- Modify: `internal/configs/config.balance.go`, `config.balance.shops.go`
- Modify: `_datafiles/config.yaml`

- [ ] **Step 1: Write the failing tests**

```go
// TestMaterialTierMultiplierBand pins the owner's design: five buckets mapping
// onto 0.75 for the lowest tier through 1.25 for the highest.
func TestMaterialTierMultiplierBand(t *testing.T) {
	configs.SetConfigForTest(t)

	if got := MaterialTierMultiplier(1); got != 0.75 {
		t.Errorf("tier 1 = %v, want 0.75", got)
	}
	if got := MaterialTierMultiplier(5); got != 1.25 {
		t.Errorf("tier 5 = %v, want 1.25", got)
	}
	if got := MaterialTierMultiplier(3); got != 1.0 {
		t.Errorf("tier 3 = %v, want 1.0", got)
	}
}

// TestUntieredMaterialIsNeutral pins the owner's ruling: a material with no
// authored tier is NEUTRAL, not cheapest. Partial coverage must never silently
// make a recipe easy.
func TestUntieredMaterialIsNeutral(t *testing.T) {
	configs.SetConfigForTest(t)

	if got := MaterialTierMultiplier(0); got != 1.0 {
		t.Fatalf("untiered material = %v, want 1.0 (neutral)", got)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/items/ -run MaterialTier -v`
Expected: FAIL, "undefined: MaterialTierMultiplier"

- [ ] **Step 3: Add the field and the function**

On `ItemSpec`:

```go
MaterialTier int `yaml:"material_tier,omitempty"` // 1 (common) to 5 (rarest). Drives craft difficulty. 0 = untiered, treated as neutral. NOT RarityTier, which is a vendor stock cap.
```

```go
// MaterialTierMultiplier maps an authored material tier onto the craft
// difficulty band, 0.75 for the commonest through 1.25 for the rarest.
//
// Tier 0 means UNTIERED and returns the neutral 1.0, never the cheapest tier:
// partial coverage must not silently make a recipe easy. A guard blocks NEW
// materials from being authored without a tier; see material_tier_guard_test.go.
//
// Deliberately not ItemSpec.RarityTier, which is a vendor stock cap where a
// higher number means MORE common, and deliberately not gold value, which was
// measured across all 126 recipes and found uncorrelated with recipe tier.
func MaterialTierMultiplier(tier int) float64 {
	b := configs.GetBalanceConfig()
	lo := float64(b.MaterialTierMultiplierMin)
	hi := float64(b.MaterialTierMultiplierMax)
	const tiers = 5

	if tier < 1 || tier > tiers {
		return lo + (hi-lo)/2.0 // untiered: neutral
	}
	return lo + (hi-lo)*float64(tier-1)/float64(tiers-1)
}
```

Declare `MaterialTierMultiplierMin` (0.75) and `MaterialTierMultiplierMax`
(1.25), defaulted in `config.balance.shops.go`. Guard each with `<= 0`, not
`< 0`, per Task 3's trap.

- [ ] **Step 4: Add the guard that blocks NEW untiered materials**

```go
// TestNewMaterialsDeclareATier grandfathers the materials that predate U10b-1
// and fails any NEW one authored without material_tier. Backfilling the
// grandfathered list is a recorded follow-up, not part of this slice.
func TestNewMaterialsDeclareATier(t *testing.T) {
	root := repoRootForTest(t)
	dir := filepath.Join(root, "_datafiles", "world", "dogmud", "items", "materials-40000")

	grandfathered := map[string]bool{
		// populated in Step 5 from the real scan
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "material_tier:") {
			continue
		}
		if grandfathered[e.Name()] {
			continue
		}
		t.Errorf("%s has no material_tier. New materials must declare one "+
			"(1 common to 5 rarest); it drives craft difficulty. If this file "+
			"predates U10b-1, add it to the grandfathered list with the others.",
			e.Name())
	}
}
```

- [ ] **Step 5: Populate the grandfathered list from the real scan**

Run:

```bash
grep -L "material_tier:" _datafiles/world/dogmud/items/materials-40000/*.yaml \
  | xargs -n1 basename | sort
```

Paste the result into `grandfathered`. Every entry is a deliberate debt, not a
pass.

- [ ] **Step 6: Run**

Run: `go test ./internal/items/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/items/ internal/configs/
git commit -m "feat(u10b-1): authored material tier, neutral when absent, guarded for new materials"
```

---

## Task 13: Deterministic ingredient resolution

`items.FindSpecByComponentTag` iterates a **Go map**. Four items share
`component_tag: bottle`, so resolving a recipe's tag through it would re-roll
craft difficulty on every attempt, swinging an alchemy craft between roughly 50%
and 88% with no observable cause.

**Files:**
- Modify: `internal/crafting/crafting.go`
- Create: `internal/crafting/ingredient_selection_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestIngredientSelectionIsDeterministic pins that repeated resolution of the
// same recipe against the same inventory yields the same items. Four items
// share component_tag "bottle", and FindSpecByComponentTag iterates a Go map,
// so a naive lookup re-rolls craft difficulty on every attempt.
func TestIngredientSelectionIsDeterministic(t *testing.T) {
	recipe := testRecipeWithBottleForTest(t)
	inv := testInventoryWithAllBottlesForTest(t)

	first := SelectIngredientItems(recipe, inv)
	for i := 0; i < 50; i++ {
		got := SelectIngredientItems(recipe, inv)
		if !sameItemIdsForTest(first, got) {
			t.Fatalf("selection %d differs from the first: %v vs %v",
				i, itemIdsForTest(first), itemIdsForTest(got))
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/crafting/ -run TestIngredientSelectionIsDeterministic -v`
Expected: FAIL, "undefined: SelectIngredientItems"

- [ ] **Step 3: Implement deterministic selection**

```go
// SelectIngredientItems resolves a recipe's ingredient tags to the CONCRETE
// items that will be consumed, deterministically.
//
// Determinism matters twice. items.FindSpecByComponentTag iterates a Go map, so
// tag lookup is order-randomised and four items share component_tag "bottle" --
// a naive lookup re-rolls craft difficulty every attempt. And difficulty must
// ride on what the player actually consumes, not on the recipe's declared tag,
// or the player-facing claim that materials set difficulty is false.
//
// Ties break on the lowest ItemId so the choice is stable across runs.
func SelectIngredientItems(recipe *Recipe, inv Inventory) []*items.Item {
	// ...
}
```

Sort candidates by `ItemId` before choosing, and have `ConsumeIngredients` take
this function's output so the roll and the consumption cannot disagree.

- [ ] **Step 4: Run**

Run: `go test ./internal/crafting/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/crafting/
git commit -m "fix(u10b-1): deterministic ingredient selection for craft difficulty"
```

---

## Task 14: Craft resolves as a floored contest, and awards under the rule

**Files:**
- Modify: `internal/crafting/crafting.go`
- Modify: `internal/hooks/NewRound_UserRoundTick.go`, `NewRound_MobRoundTick.go`
- Modify: `internal/mobs/crafter.go`
- Modify: `internal/crafting/crafting_test.go`, `integration_crafting_test.go`, `internal/mobs/crafter_test.go`
- Modify: `contest_floor_guard_test.go`, `internal/combat/contest_site_guard_test.go`
- Create: `internal/crafting/craft_contest_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// TestCraftAtRecipeMinimumIsFiftyFifty pins the anchor: at the recipe minimum
// with a neutral material tier, score equals difficulty, so the contest is 50%.
// That is exactly the shipped CraftingBaseSuccessChance of 50 this replaces.
func TestCraftAtRecipeMinimumIsFiftyFifty(t *testing.T) {
	configs.SetConfigForTest(t)

	score := CraftScore(10, 10)
	difficulty := CraftDifficulty(1.0)

	if score == 0 || difficulty == 0 {
		t.Fatal("score or difficulty is zero; the config knobs are not wired")
	}
	if score != difficulty {
		t.Fatalf("score %v != difficulty %v at the recipe minimum", score, difficulty)
	}
}

// TestCraftMasteryReadsTheSameAtEveryTier pins that the per-level bonus is a
// PERCENTAGE. Success depends on the RATIO of difficulty to score, so a flat
// addend would pay 92.8% nine levels above a skill_minimum 0 recipe and 73%
// nine levels above a skill_minimum 40 one.
func TestCraftMasteryReadsTheSameAtEveryTier(t *testing.T) {
	configs.SetConfigForTest(t)

	low := CraftDifficulty(1.0) / CraftScore(9, 0)
	high := CraftDifficulty(1.0) / CraftScore(49, 40)

	if math.IsNaN(low) || math.IsNaN(high) || low == 0 {
		t.Fatal("ratio is NaN or zero; the config knobs are not wired")
	}
	if math.Abs(low-high) > 0.001 {
		t.Fatalf("ratio nine levels above the minimum is %v on a novice "+
			"recipe and %v on an advanced one", low, high)
	}
}

// TestFailedCraftAwardsTheFraction is the case the whole slice is justified by.
// A failed craft consumes the materials; before U10b-1 it also taught nothing.
func TestFailedCraftAwardsTheFraction(t *testing.T) {
	// Drive a craft that fails and assert the recipe skill received an award
	// carrying ProgressionFailureFraction.
}
```

⚠️ The zero and NaN guards are load-bearing. Without them both formula tests
pass against unwired knobs, since `0 == 0` and `NaN != NaN` is not `> 0.001`.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/crafting/ -run Craft -v`
Expected: FAIL, "undefined: CraftScore"

- [ ] **Step 3: Implement the formulas**

```go
// CraftDifficulty is the number a craft attempt is contested against.
//
// Difficulty comes from the MATERIALS. recipe.SkillMinimum is deliberately
// absent: requiring 50% at every recipe's minimum AND equal mastery at every
// tier forces any SkillMinimum term to appear on both sides and cancel. It
// still gates discovery and attempts. See spec section 5.1.1.1.
func CraftDifficulty(materialTierMult float64) float64 {
	return float64(configs.GetBalanceConfig().CraftBaseDifficulty) * materialTierMult
}

// CraftScore is the crafter's side of that contest.
//
// The per-level term is a PERCENTAGE of the base, not an addend, because
// contest.Run derives stdDev from the attacker's score and success therefore
// depends on the ratio of difficulty to score, never on the gap.
func CraftScore(skillLevel, recipeSkillMinimum int) float64 {
	b := configs.GetBalanceConfig()
	levels := float64(skillLevel - recipeSkillMinimum)
	return float64(b.CraftBaseDifficulty) * (1.0 + levels*float64(b.CraftSkillWeightPct))
}
```

Declare `CraftBaseDifficulty` (100), `CraftSkillWeightPct` (0.05) and
`CraftFloor` (0.05). `CraftBaseDifficulty`'s doc comment must say it cancels out
of the odds and exists only to keep both numbers in stat range so
`dice.StdDevFor` does not hit its `mean < 1.0` floor, so the pending config
audit does not flag it as orphaned.

- [ ] **Step 4: Route the four call sites**

```go
tierMult := crafting.DearestIngredientTier(selected)   // from Task 13's items
res := contest.RunWithFloors(
	crafting.CraftScore(sl, recipe.SkillMinimum),
	[]contest.Entry{{Score: crafting.CraftDifficulty(tierMult)}},
	float64(configs.GetBalanceConfig().CraftFloor),
)
if res.Success {
```

⚠️ `RunWithFloors`, not `AgainstDifficulty`: craft needs the mercy band its
`[5%, 95%]` clamp provides today. Extremes match exactly; the interior
compresses by `(1 - 2f)`.

- [ ] **Step 5: Award on BOTH branches**

The success branch keeps its difficulty-scaled award. The failure branch gains
one at `ProgressionFailureFraction`. This is the deliverable, not a detail.

- [ ] **Step 6: Retire `CalcSuccessChance` and fix its test callers**

Delete `CalcSuccessChance` and the four `Crafting*SuccessChance` knobs. Three
test files break and must be updated in this task:

- `internal/crafting/crafting_test.go` (`TestCalcSuccessChance`)
- `internal/crafting/integration_crafting_test.go` (four calls)
- `internal/mobs/crafter_test.go`, where `forceCraftSuccess`/`forceCraftFailure`
  drive the deleted knobs by name through `configs.AddOverlayOverrides`. **Map
  keys are strings, so this will compile and silently stop working.** Replace
  the determinism lever with `CraftFloor` and an extreme `CraftBaseDifficulty`,
  and assert the forced outcome actually occurs.

- [ ] **Step 7: Guard entries, then run**

Run: `go test . ./internal/combat/ ./internal/crafting/ ./internal/hooks/ ./internal/mobs/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/crafting/ internal/hooks/ internal/mobs/ internal/configs/ contest_floor_guard_test.go internal/combat/contest_site_guard_test.go
git commit -m "feat(u10b-1): craft difficulty from materials, floored, awarding on failure"
```

---

## Task 15: Mob crafters award on the same rule

`mobs/crafter.go` awards `OnSkillUse` inside its success branch only, at two
sites. Leaving it reintroduces exactly the inconsistency this slice removes.

**Files:**
- Modify: `internal/mobs/crafter.go`
- Create: `internal/mobs/crafter_progression_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestMobCrafterAwardsOnFailure pins that mob crafters fire under the same rule
// as players. Awarding only on success is the firing-condition inconsistency
// U10b-1 exists to remove.
func TestMobCrafterAwardsOnFailure(t *testing.T) {
	// Force a failed mob craft and assert the recipe skill received an award
	// carrying ProgressionFailureFraction.
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/mobs/ -run TestMobCrafterAwardsOnFailure -v`
Expected: FAIL

- [ ] **Step 3: Award on both branches at both sites**

- [ ] **Step 4: Run, and check the economy effect**

Run: `go test ./internal/mobs/ ./internal/shops/ -v`
Expected: PASS

⚠️ Mob crafters ship at skill 1 and restock shops. Note in the commit what the
new success rate is at skill 1 against a neutral tier, since dynamic pricing
keys on the stock-to-restock ratio and a throughput drop pins goods near the
price ceiling.

- [ ] **Step 5: Commit**

```bash
git add internal/mobs/
git commit -m "feat(u10b-1): mob crafters award under the same firing rule as players"
```

---

## Task 16: Salvage resolves as a floored contest

**Files:**
- Modify: `internal/crafting/salvage.go`
- Modify: `internal/actions/salvage.go`
- Modify: `internal/configs/config.balance.go`, `config.balance.misc.go`
- Create: `internal/crafting/salvage_contest_test.go`

⚠️ `SalvageMinChance`/`SalvageMaxChance` default in `config.balance.misc.go`,
not `shops.go`, and `internal/actions/salvage.go` reads both.

- [ ] **Step 1: Write the failing tests**

```go
// TestSalvageDearMaterialsAreHarderToReclaim pins that difficulty rides on the
// tier of the material being recovered.
func TestSalvageDearMaterialsAreHarderToReclaim(t *testing.T) {
	configs.SetConfigForTest(t)

	cheap := SalvageDifficulty(items.MaterialTierMultiplier(1))
	dear := SalvageDifficulty(items.MaterialTierMultiplier(5))

	if cheap == 0 || dear == 0 {
		t.Fatal("difficulty is zero; the config knobs are not wired")
	}
	if dear <= cheap {
		t.Fatalf("dear %v is not above cheap %v", dear, cheap)
	}
}

// TestSalvageKeepsItsCeiling pins the economy guard. Uncapped salvage at high
// skill would retain ~99.9% of materials against 80.75% today, roughly a 250x
// reduction in the crafting material sink, on the exact loop that farms
// crafting skill. SalvageFloor 0.15 reproduces today's 85% ceiling.
func TestSalvageKeepsItsCeiling(t *testing.T) {
	configs.SetConfigForTest(t)
	const trials = 4000

	won := 0
	for i := 0; i < trials; i++ {
		if RollSalvageUnit(SalvageScore(50), SalvageDifficulty(1.0)).Success {
			won++
		}
	}

	if rate := float64(won) / trials; rate > 0.90 {
		t.Fatalf("per-unit recovery at skill 50 is %.3f; the floor is not "+
			"capping it and the material sink is gone", rate)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/crafting/ -run Salvage -v`
Expected: FAIL

- [ ] **Step 3: Implement, mirroring craft**

Add `SalvageScore`, `SalvageDifficulty` and a `RollSalvageUnit` that calls
`contest.RunWithFloors` with `SalvageFloor` (0.15).

⚠️ `RollSalvageReturns(ingredients []RecipeIngredient, chance float64)` and
`RollSalvageReturnsFromSpec(returns []items.SalvageReturn, chance float64)`
currently take a precomputed chance, and `RecipeIngredient` carries no item
data. **Both signatures change** to take the salvage skill level and resolve
each ingredient's tier. Update both callers in `internal/actions/salvage.go`.

- [ ] **Step 4: Retire the clamps**

Remove `SalvageMinChance` and `SalvageMaxChance` from `config.balance.go`,
`config.balance.misc.go`, `_datafiles/config.yaml`, and the two reads in
`internal/actions/salvage.go`. Keep `SalvageSoftCap`, `SalvageGoldPerRound` and
`SalvageMaxRounds`, which govern skill scaling and duration.

- [ ] **Step 5: Guard entries, then run**

Run: `go test . ./internal/combat/ ./internal/crafting/ ./internal/actions/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/crafting/salvage.go internal/actions/salvage.go internal/configs/ contest_floor_guard_test.go internal/combat/contest_site_guard_test.go
git commit -m "feat(u10b-1): salvage on the core, floored to keep the material sink"
```

---

# Phase D: skullduggery and spell parity

## Task 17: The sixteen skullduggery sites

`actions/steal.go` x3, `actions/plant.go` x3, `actions/shadow.go` x2,
`usercommands/skill.skullduggery.sneak.go` x2, `usercommands/picklock.go` x2,
`actions/defuse.go`, `usercommands/throw.go`, `mobcommands/flee.go`,
`hooks/NewRound_DoCombat_helpers.go`.

Four bypass every entry point by calling `CheckSkillProgression` directly: both
sneak sites (one is the **failure** branch) and both picklock sites.

**Files:**
- Modify: the twelve files above
- Create: `internal/progression/skullduggery_seam_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestSkullduggerySitesRouteThroughTheSeam pins that no skullduggery award
// reaches progression without an entry point the guard can see. Four sites
// called CheckSkillProgression directly, including the FAILURE branch of player
// sneak, so they touched no entry point and no guard.
func TestSkullduggerySitesRouteThroughTheSeam(t *testing.T) {
	root := repoRootForTest(t)
	files := []string{
		"internal/usercommands/skill.skullduggery.sneak.go",
		"internal/usercommands/picklock.go",
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "CheckSkillProgression(") {
			t.Errorf("%s calls CheckSkillProgression directly, bypassing the seam", f)
		}
	}
}

// TestSkullduggerySitesAwardOnFailure pins the rule at a representative site.
// The source assertion above proves routing; this proves the fraction is
// actually applied, which routing alone does not.
func TestSkullduggerySitesAwardOnFailure(t *testing.T) {
	// Drive a failed steal and assert the skullduggery award carries
	// ProgressionFailureFraction.
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/progression/ -run Skullduggery -v`
Expected: FAIL

- [ ] **Step 3: Convert each site**

Build an `Outcome` per site, set `Defended` from the real contest result, and
apply via `ApplyProgression(progression.OrdinaryEventsScaled(o, fraction), ...)`.

⚠️ **Two traps:**

1. `Outcome` holds exactly one `AttackerSkill`. A site awarding both a combat
   skill and skullduggery needs **two** `Outcome` values.
2. `SkillPrimaryStats["skullduggery"] == "dexterity"`, the same as
   weapon-combat, so awarding both rolls dexterity twice on top of the
   unconditional attacker stat gain. Leave the second `Outcome`'s `Stat` empty.

- [ ] **Step 4: Run everything**

Run: `go test ./... 2>&1 | grep -v "^ok" | head -40`
Expected: no failures

- [ ] **Step 5: Update the census, then commit**

```bash
git add internal/actions/steal.go internal/actions/plant.go internal/actions/shadow.go internal/actions/defuse.go internal/usercommands/skill.skullduggery.sneak.go internal/usercommands/picklock.go internal/usercommands/throw.go internal/mobcommands/flee.go internal/hooks/NewRound_DoCombat_helpers.go internal/progression/
git commit -m "refactor(u10b-1): skullduggery progression routes through the seam"
```

⚠️ Explicit paths, never `git add internal/`. Eleven code-touching tasks precede
this one and a directory add would sweep their strays into this commit.

---

## Task 18: The mob spell path adopts the player path's gates

The player path applies a self-cast penalty, zeroes progression for an area cast
that found no targets, and gates on `spellBonus > 0`. The mob path has none.

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go`
- Create: `internal/hooks/spell_progression_parity_test.go`

- [ ] **Step 1: Write the failing test**

⚠️ Do **not** assert that the mob block contains the gate literals. Step 3
extracts them into a shared helper, after which the mob block contains a call,
not the literals, so such a test fails after a correct implementation. Assert on
**behaviour**:

```go
// TestMobSelfCastIsPenalisedLikePlayers pins parity on gate one.
func TestMobSelfCastIsPenalisedLikePlayers(t *testing.T) {
	// Drive a mob self-cast and a player self-cast; assert both awards carry
	// SelfCastProgressionMultiplier.
}

// TestMobAreaCastWithNoTargetsAwardsNothing pins parity on gate two.
func TestMobAreaCastWithNoTargetsAwardsNothing(t *testing.T) {
	// Drive a mob area cast that finds no targets; assert no award.
}

// TestMobCastWithZeroBonusAwardsNothing pins parity on gate three.
func TestMobCastWithZeroBonusAwardsNothing(t *testing.T) {
	// Drive a mob cast with spellBonus 0; assert no award.
}
```

- [ ] **Step 2: Run to verify all three fail**

Run: `go test ./internal/hooks/ -run TestMob.*Cast -v`
Expected: FAIL

- [ ] **Step 3: Extract one helper, call it from both paths**

Do not copy the gates into the mob path; a future change would diverge again.

- [ ] **Step 4: Run**

Run: `go test ./internal/hooks/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/
git commit -m "fix(u10b-1): mob spell progression adopts the player path's gates"
```

---

# Phase E: deletions

## Task 19: Delete the stranded mob-follow roll

**Files:**
- Modify: `internal/usercommands/go.go`
- Create: `internal/usercommands/go_mobfollow_test.go`

⚠️ `internal/usercommands/go_test.go` does **not** exist; create the file above.

- [ ] **Step 1: Write the failing test**

```go
// TestNoHostileMobFollowRoll pins that the stranded hostile-follow roll is gone.
// It sat on the ordinary movement path, which refuses movement entirely while
// the player is in combat, so its only reachable window was a stale-aggro edge
// case. Pursuit becomes authored mob behavior in the behavior unification arc.
func TestNoHostileMobFollowRoll(t *testing.T) {
	root := repoRootForTest(t)
	b, err := os.ReadFile(filepath.Join(root, "internal", "usercommands", "go.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	if strings.Contains(src, "Mob Follow") {
		t.Error("go.go still contains the stranded hostile mob-follow roll")
	}
	// The pursuit loop sat INSIDE the !isSneaking block, which also fires
	// room_enter behaviors. Deleting the wrapper would silently kill those.
	if !strings.Contains(src, "TryRoomBehavior") {
		t.Error("room_enter behavior firing was deleted along with the pursuit " +
			"loop; only the GetMobs(FindFightingPlayer) loop should go")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/usercommands/ -run TestNoHostileMobFollowRoll -v`
Expected: FAIL on the first assertion

- [ ] **Step 3: Delete ONLY the pursuit loop**

Remove the `mobInstanceIds := room.GetMobs(rooms.FindFightingPlayer)` loop and
its body. **Keep** the `if !isSneaking {` wrapper, the charmed-mob follow above
it, and the `behaviortree.TryRoomBehavior(destRoom.RoomId, ...)` call after it.

- [ ] **Step 4: Run**

Run: `go test ./internal/usercommands/ ./internal/behaviortree/ -v && go build ./...`
Expected: PASS, no build output

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/go.go internal/usercommands/go_mobfollow_test.go
git commit -m "refactor(u10b-1): delete the stranded hostile mob-follow roll"
```

---

## Task 20: Delete first-kill progression

**Files:**
- Modify: `internal/characters/progression.go`
- Modify: `internal/hooks/Death_MobKillCredit.go`
- Create: `internal/hooks/first_kill_progression_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestFirstKillAwardsNoProgression pins the owner's ruling, made 2026-08-21 and
// reaffirmed 2026-08-26. Kill TRACKING stays: AddMobKill feeds the kill and
// bestiary displays and is not progression.
func TestFirstKillAwardsNoProgression(t *testing.T) {
	root := repoRootForTest(t)
	for _, f := range []string{
		"internal/characters/progression.go",
		"internal/hooks/Death_MobKillCredit.go",
	} {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "OnFirstMobKill") {
			t.Errorf("%s still references OnFirstMobKill", f)
		}
	}

	b, err := os.ReadFile(filepath.Join(root,
		filepath.FromSlash("internal/hooks/Death_MobKillCredit.go")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "AddMobKill") {
		t.Error("kill TRACKING was deleted with the award; AddMobKill feeds " +
			"the bestiary and must stay")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/hooks/ -run TestFirstKillAwardsNoProgression -v`
Expected: FAIL

- [ ] **Step 3: Delete**

Remove `Character.OnFirstMobKill`, both call sites (killer and party members),
and the message "Defeating a new foe hones your combat instincts!". Keep
`KD.AddMobKill`.

- [ ] **Step 4: Run, update the census, commit**

```bash
git add internal/characters/progression.go internal/hooks/ internal/progression/census_test.go
git commit -m "refactor(u10b-1): delete first-kill progression, keep kill tracking"
```

---

## Task 21: Dead crit stubs, the decay pin, and stale comments

- [ ] **Step 1: Re-anchor the decay test FIRST**

`TestCritReceivedProgression_DecaysWithRank` (`internal/characters/progression_faucet_test.go`)
pins the owner's decay condition through the real chance function, reached via
its local `critReceivedChanceForTest` wrapper. It is named and documented after
`OnCritReceived`, which this task removes. Rename it and rewrite its doc comment
to describe `Outcome.ToughenStat`, keeping the assertion identical.

⚠️ The spec's earlier reference to a symbol named `statProgressionChance` was
wrong; the real production entry point is `Character.ProgressionChanceForStat`.

Run: `go test ./internal/characters/ -run DecaysWithRank -v`
Expected: PASS, unchanged behaviour under the new name

- [ ] **Step 2: Delete the stubs from ALL NINE files**

```
internal/actions/consider_test.go     internal/actions/salvage_test.go
internal/actions/economy_test.go      internal/actions/scan_test.go
internal/actions/forage_test.go       internal/actions/search_test.go
internal/actions/sleep_test.go        internal/actions/track_test.go
internal/hooks/spell_foldanchor_test.go
```

⚠️ **Nine, not five.** An earlier draft of this plan said five. The methods
implement no interface, so deleting only some leaves the rest compiling and
invisible.

Verify none remain: `grep -rl "func.*OnCritical" internal/` → no output.

- [ ] **Step 3: Fix the DENY-list, not the allow-list**

`internal/progression/seam_guard_test.go` has two maps:
`allowedDirectProgression` (allow-list, keyed by file) and `progressionCalls`
(the **deny**-list of guarded method names). `"OnCriticalSuccess"` and
`"OnCriticalFailure"` sit in **`progressionCalls`**. Remove them from there.

⚠️ Removing rows from `allowedDirectProgression` instead would silently break
the U9 guard.

- [ ] **Step 4: Fix the two stale comments in `defence_multiplier.go`**

1. The comment claiming a forced-crit defence "was still progressed exactly as
   on the melee path" is wrong: the melee path returns early on `forceCrit`
   before `DefenseUsed` is stamped, so it is not progressed there, while the
   channel path's `AwardDefenceProgression` fires unconditionally including
   under `ForceCrit`.
2. The comment citing a `Margin` negation at line 307 points at an unrelated
   function. The real negations are at 506 and 564. Prefer describing the
   behaviour to citing line numbers, which rot.

- [ ] **Step 5: Run and commit**

```bash
git add internal/actions/ internal/hooks/ internal/characters/ internal/progression/ internal/combat/defence_multiplier.go
git commit -m "chore(u10b-1): delete dead crit stubs, re-anchor the decay pin, fix stale comments"
```

---

# Phase F: guard, docs, verify

## Task 22: The standing guard

**Files:**
- Modify: `internal/progression/seam_guard_test.go`

- [ ] **Step 1: Define what the scanner looks for, narrowly**

⚠️ `util.Rand(` has **210** production call sites and `dice.RollStat(` has 14. A
scanner that flags every `util.Rand` turns Step 3 into a multi-day triage of
loot tables, spawn rolls and weather. Scope it to the shape this slice cares
about: a roll whose result is **compared against a threshold to decide an
action's success**, which in practice means `dice.RollStat(x)` followed by a
comparison, and `util.Rand(n)` compared against a computed chance variable.

Write the recogniser to report `file:line:mechanism`, and accept that it will
have false positives. The allow-list is how they are dispositioned.

- [ ] **Step 2: Write the guard**

```go
// TestEveryRolledSiteRoutesThroughTheSeam is U10b-1's standing guard: a new
// uncertain-outcome roll must resolve on the contest core, not with a bare
// util.Rand or dice.RollStat threshold.
//
// The allow-list holds DELIBERATE exceptions from spec section 3.1 and 7. Each
// entry needs a reason; adding one is a decision, not a formality.
func TestEveryRolledSiteRoutesThroughTheSeam(t *testing.T) {
	allowed := map[string]string{
		"internal/behaviortree/actions_progression.go": "authored tutorial grant, spec 3.1",
		"internal/hooks/item_procs.go":                 "item procs stay off-core, spec 7 (owner, 2026-08-26)",
		"internal/hooks/NewRound_AutoHeal.go":          "regen ticks, deferred to U10b-2",
		"internal/hooks/NewRound_DoCombat_helpers.go":  "mob weapon pickup and target switch; the latter is U12's surface",
		"internal/usercommands/target.go":              "target switch, assigned to U12",
	}

	for _, o := range scanForBareUncertaintyRolls(t, repoRootForTest(t)) {
		if _, ok := allowed[o.File]; !ok {
			t.Errorf("%s:%d resolves an uncertain outcome with %s, off the "+
				"contest core. Route it through contest.AgainstDifficulty, "+
				"contest.RunWithFloors or combat.RunContest, or add it to the "+
				"allow-list with a reason.", o.File, o.Line, o.Mechanism)
		}
	}
}
```

- [ ] **Step 3: Prove the scanner is not blind before trusting a pass**

Add a unit test that feeds the scanner a source string containing a known bare
threshold roll and asserts it is reported. **If the guard passes on its first
run before this test exists, assume the scanner is blind, not the tree clean.**

- [ ] **Step 4: Run, disposition each offender, commit**

```bash
git add internal/progression/seam_guard_test.go
git commit -m "test(u10b-1): guard that new rolled sites stay on the contest core"
```

---

## Task 23: Breadcrumbs and the roadmap

- [ ] **Step 1: Breadcrumb what stays off-core**

Add a `NOTE(unassigned)` to `item_procs.go`'s proc gate explaining that one flat
`util.Rand(100)` decides whether every equipped item's lifesteal, stun, drain
and bleed fires, that it is off the contest core, and that U10b-1 left it
deliberately (owner, 2026-08-26). Add a matching note to
`handleMobWeaponPickup`.

- [ ] **Step 2: Hand the two target-switch sites to U12**

Add a `NOTE(U12)` at both `ChanceToSwitchTarget` roll sites saying the formula
is a pure self-stat check that `contest.AgainstDifficulty` fits exactly, and
that targeting is U12's declared surface.

- [ ] **Step 3: Correct the roadmap**

Update the Category B row in `UNIFIED_RESOLUTION_ROADMAP.md`: the real census is
**20 sites, not 8**; name the twelve that carried no breadcrumb; record that
U10b-1 converted 15, U12 owns 2, and 3 stay off-core deliberately. Note that
`contest.AgainstDifficulty` being callerless was **not** a gap to close for its
own sake, since static-difficulty rolls are deliberately unfloored.

- [ ] **Step 4: Commit**

```bash
git add internal/hooks/item_procs.go internal/hooks/NewRound_DoCombat_helpers.go internal/usercommands/target.go docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md
git commit -m "docs(u10b-1): breadcrumb what stays off-core; correct the Category B census"
```

---

## Task 24: Re-solve every fitted multiplier

Awarding on losses raises attacker events per swing by roughly **26%** and
defender events by roughly **47%**. Every fitted multiplier is invalidated, not
just skullduggery.

- [ ] **Step 1: Record the BEFORE rates**

Run: `python tools/balance/read_combat_analytics.py`

⚠️ The buffer is **cumulative**. Never sum flush lines; read the latest
cumulative figure.

- [ ] **Step 2: Re-solve**

Run: `python tools/balance/u10b_solve_v3.py`

Feed it the new firing conditions. Re-solve weapon-combat, dodge, parry, block,
spellcasting, rhetoric and skullduggery, or record why a given one is exempt.

- [ ] **Step 3: Apply and commit with the numbers in the body**

```bash
git commit -m "balance(u10b-1): re-solve skill multipliers for the failure award"
```

The body must record before and after per skill, so the next reader can tell a
retune from a regression.

---

## Task 25: Docs, `context.md`, patch notes

- [ ] **Step 1: Update the affected `context.md` files**

`internal/progression` gains `OrdinaryEventsScaled` and `Outcome.Defended`.
`internal/crafting` gains `CraftScore`, `CraftDifficulty`, `SalvageDifficulty`,
`SelectIngredientItems`, and loses `CalcSuccessChance`. `internal/items` gains
`MaterialTierMultiplier` and `ItemSpec.MaterialTier`. `internal/characters`
gains `OnStatUseScaled`.

⚠️ Verify every symbol exists before naming it:

```powershell
Select-String -Path internal\crafting\*.go -Pattern '^(func|type|const|var)\s'
```

- [ ] **Step 2: Add a dated `docs/PATCH_NOTES.md` entry**

Player-facing framing, no raw numbers, no em dashes, wrapped at 80. Cover:
failing at something now teaches you a little; searching, tracking and foraging
resolve like everything else; how hard a craft is depends on the materials you
work with; someone good at hiding is harder to find.

⚠️ Do **not** promise that mobs will now chase you. Task 19 deletes a roll; it
adds no pursuit.

- [ ] **Step 3: Commit**

---

## Task 26: Pre-push verification and the adversarial playtest

- [ ] **Step 1: Formatting**

Run: `gofmt -l internal/ modules/`
Expected: no output

- [ ] **Step 2: Build and full test run**

Run: `go build ./... && go test ./... 2>&1 | grep -v "^ok" | head -40`
Expected: no failures

- [ ] **Step 3: Confirm `Logging.LogToFile: false`**

⚠️ `_datafiles/config.yaml` has `skip-worktree`. Build the commit from the
`git show HEAD:` blob, never from disk.

- [ ] **Step 4: Boot test in an isolated detached worktree**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

⚠️ **Exit code 124 is the success case.** Do not grep for the bare word
`panic`; `GamePlay.MapConsistencyEnforce` legitimately has the value `panic`.
Clean up with `git worktree remove --force`.

- [ ] **Step 5: Adversarial playtest, reporting FOUR signals separately**

Run: `/playtest local --checkout <abs> bug-finder <goals>.yaml`

1. **The convention move.** Does failing at something visibly teach a little?
2. **The stealth change.** Is a skilled hider meaningfully harder to find, from
   both sides?
3. **Crafting feel.** Does difficulty track the materials? Does mastery read the
   same on a novice recipe and an advanced one? Biggest surface, likeliest
   source of complaints.
4. **Search, track and forage odds.** Weak searchers improve sharply; experts
   lose near-certainty.

Also watch the economy: the craft-then-salvage material sink, and whether shop
restock throughput holds up with mob crafters at skill 1.

⚠️ **The AI port caps 3 commands per round and silently discards the overflow
after echoing it.** A dropped command looks exactly like a broken one. Send one
command per batch and verify output before concluding anything.

- [ ] **Step 6: Fix findings, extract them to memory**

Playtest reports are gitignored. Extract every finding to a memory topic file
before the session ends.

- [ ] **Step 7: Open the PR**

```bash
git push -u origin feature/u10b-1-progression-firing-convention
gh pr create --repo pruuk/DOGMud --base master --head feature/u10b-1-progression-firing-convention --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

⚠️ Always pass `--repo pruuk/DOGMud`; this repo is a fork and `gh` defaults to
the parent. A green check is not proof: confirm with
`gh run view <id> --repo pruuk/DOGMud --log-failed`.
