# U10b-1 Progression Firing Convention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give progression one firing rule (one event per resolved roll, a loss pays a fraction, crit and fumble stay a bonus layer), move the last 15 uncertain outcomes onto the contest core, and delete three pieces of dead progression machinery.

**Architecture:** The U9 seam (`internal/progression`) already carries per-side ordinary events plus a single `Exceptional` bonus enum. It deliberately deferred WHEN an event fires to each call site. This plan replaces "the call site's existing rules" with one rule, expressed as a `Multiplier` on the ordinary event, and gives `contest.AgainstDifficulty` its first production callers. Craft and salvage need a difficulty basis designed rather than migrated, because they are flat percentages today.

**Tech Stack:** Go 1.x, `internal/contest`, `internal/combat`, `internal/progression`, `internal/characters`, `internal/configs`, YAML data under `_datafiles/`.

**Spec:** `docs/superpowers/specs/2026-08-26-u10b-1-progression-firing-convention-design.md`

---

## Read this before Task 1

Three facts that will otherwise cost you a day each.

1. **`contest.Result.Success` means the ATTACKER won.** It is NOT safe to read
   `!res.Success` as "the defender won": under `side.ForceCrit` (a sleeping
   victim) the attack wins with `Success == false`. Always gate on
   `out.Defended`. A mirrored test fake will pass either way, so this bug does
   not surface in tests.
2. **`contest.Run` rolls the difficulty side too**, using a `stdDev` derived
   from the ATTACKER's score. Success therefore depends on the RATIO of
   difficulty to score, never on the gap. Every formula in this plan is a ratio
   for that reason.
3. **`OnSkillUseScaled` rolls the skill's primary stat at an unscaled 1.0.**
   Passing a fraction through it damps the skill and leaves the stat at full
   rate. Task 4 exists to fix this; do not skip it and thread a multiplier
   through the existing call.

## File structure

| File | Responsibility | Task |
|---|---|---|
| `internal/configs/config.balance.go` | Declare `ProgressionFailureFraction`, the craft difficulty knobs, and the material tier thresholds | 3, 10, 11 |
| `internal/configs/config.balance.progression.go` | Default and validate the progression knob with a sentinel | 3 |
| `internal/configs/config.balance.shops.go` | Default and validate the craft/material knobs | 10, 11 |
| `internal/characters/progression.go` | Add `OnStatUseScaled`; make the stat half of an ordinary event honour `Multiplier`; delete `OnFirstMobKill` | 4, 16 |
| `internal/progression/event.go` | `Outcome` gains `Defended`; `OrdinaryEvents` scales the losing side | 5 |
| `internal/actions/search.go` | Six threshold checks onto the core; hidden detection reads the hider's score | 6, 7 |
| `internal/actions/track.go` | One threshold check onto the core | 8 |
| `internal/forager/forage_core.go` | One threshold check onto the core | 9 |
| `internal/items/tier.go` (new) | Map an item's gold value to a material tier multiplier | 10 |
| `internal/crafting/crafting.go` | Craft difficulty and score, replacing `CalcSuccessChance` | 11 |
| `internal/crafting/salvage.go` | Salvage difficulty per ingredient unit | 12 |
| `internal/usercommands/go.go` | Delete the stranded mob-follow roll | 15 |
| `internal/progression/seam_guard_test.go` | The guard that keeps new sites on the seam | 2, 18 |

---

## Task 1: Freeze the site census as a test fixture

Everything after this depends on the site list being right. The 2026-08-19
audit is stale and the 2026-08-21 plan under-counted. Establish the truth once,
in a form that fails when it drifts.

**Files:**
- Create: `internal/progression/census_test.go`

- [ ] **Step 1: Write the failing test**

```go
package progression_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot resolves the repository root from this file's own location.
// Test binaries do NOT reliably start in the package directory: all tests
// share one binary, so a relative path passes or fails depending on which
// package ran first. Anchor on runtime.Caller instead.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

// TestProgressionEntryPointCensus pins how many production call sites reach
// progression, so that adding one without classifying it fails here.
func TestProgressionEntryPointCensus(t *testing.T) {
	root := repoRoot(t)
	entryPoints := []string{
		"OnSkillUseScaled(", "OnSkillUse(", "OnStatUse(",
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
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "//") {
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

	const want = 0 // replaced in Step 3 with the measured number
	if count != want {
		t.Fatalf("progression entry-point call sites = %d, want %d.\n"+
			"If you added a site deliberately, classify it in the spec's "+
			"section 3.1 table and update this number in the same commit.",
			count, want)
	}
}
```

- [ ] **Step 2: Run it to see the real number**

Run: `go test ./internal/progression/ -run TestProgressionEntryPointCensus -v`
Expected: FAIL, reporting the actual count. Record that number.

- [ ] **Step 3: Set `want` to the measured number and re-run**

Replace `const want = 0` with the count from Step 2.

Run: `go test ./internal/progression/ -run TestProgressionEntryPointCensus -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/progression/census_test.go
git commit -m "test(u10b-1): pin the progression entry-point census"
```

---

## Task 2: Fix the guard's AST helper before trusting any guard

**The existing helper cannot fail for the bug it names.** In
`internal/combat/contest_site_guard_test.go` the walker does:

```go
case *ast.SelectorExpr:
	consumed[v.Sel] = true
	pkg, ok := v.X.(*ast.Ident)
	if !ok {
		return true
	}
```

`x.Character.OnStatUse(...)` is a **selector on a selector**, so `v.X` is an
`*ast.SelectorExpr`, not an `*ast.Ident`, and the walker bails. That is the
dominant call shape in this codebase. A guard built on this helper ships green
and enforces nothing.

**Files:**
- Modify: `internal/combat/contest_site_guard_test.go`

- [ ] **Step 1: Write a test that proves the helper is blind**

```go
// TestSelectorWalkerSeesChainedCalls pins the bug fixed in U10b-1: the walker
// used to bail on x.Character.OnStatUse(...) because v.X is a SelectorExpr
// rather than an Ident, which is the dominant call shape in this repo. A guard
// built on the old helper passed while enforcing nothing.
func TestSelectorWalkerSeesChainedCalls(t *testing.T) {
	src := `package p
type c struct{}
func (c) OnStatUse(s string, i int) bool { return false }
type x struct{ Character c }
func f(v x) { v.Character.OnStatUse("dexterity", 1) }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if selectorTail(sel) == "OnStatUse" {
			found = true
		}
		return true
	})

	if !found {
		t.Fatal("walker did not see v.Character.OnStatUse; the guard is blind " +
			"to chained selectors and enforces nothing")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/combat/ -run TestSelectorWalkerSeesChainedCalls -v`
Expected: FAIL with "undefined: selectorTail"

- [ ] **Step 3: Add the helper**

```go
// selectorTail returns the final selector name in a chain, so that both
// pkg.Fn(...) and a.b.c.Fn(...) report "Fn". The previous code asserted
// v.X was an *ast.Ident and gave up otherwise, which silently excluded
// every method call reached through a field.
func selectorTail(sel *ast.SelectorExpr) string {
	if sel == nil || sel.Sel == nil {
		return ""
	}
	return sel.Sel.Name
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/combat/ -run TestSelectorWalkerSeesChainedCalls -v`
Expected: PASS

- [ ] **Step 5: Rewrite the existing walker to use it**

Replace the `pkg, ok := v.X.(*ast.Ident); if !ok { return true }` bail in the
existing `inspect` closure so that the chained shape is recorded. Keep the
`consumed[v.Sel] = true` line: it still prevents an ident consumed as a
selector tail from also counting as a bare reference.

- [ ] **Step 6: Run the whole guard package**

Run: `go test ./internal/combat/ -run Guard -v`
Expected: PASS. If a previously-green guard now FAILS, that is the point of
this task: it was blind. Record which sites it newly sees, and do not silence
it; those sites are real work for Task 18.

- [ ] **Step 7: Commit**

```bash
git add internal/combat/contest_site_guard_test.go
git commit -m "test(u10b-1): guard walker must see chained selector calls"
```

---

## Task 3: The `ProgressionFailureFraction` knob, with a sentinel default

An absent YAML key unmarshals to `0`, which is a legal value for a fraction. The
usual idiom in this file therefore cannot default it, and the knob would ship at
zero with failure paying nothing and the whole slice looking inert.

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.progression.go`
- Modify: `_datafiles/config.yaml`
- Test: `internal/configs/progression_failure_fraction_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package configs

import "testing"

// loadBalanceYAML seeds the sentinel exactly as production loading must, then
// unmarshals. The seeding is the whole mechanism: after unmarshal, -1 means the
// key was ABSENT and any value in [0,1] means it was PRESENT. Without it, an
// absent key and an explicit 0 are indistinguishable and the two tests below
// cannot both pass.
func loadBalanceYAML(t *testing.T, doc string) Balance {
	t.Helper()
	b := Balance{ProgressionFailureFraction: -1}
	if err := yaml.Unmarshal([]byte(doc), &b); err != nil {
		t.Fatal(err)
	}
	b.Validate()
	return b
}

// TestProgressionFailureFractionDefaultsWhenAbsent pins the trap: an absent
// YAML key unmarshals to 0, which is a LEGAL value for this knob, so the
// codebase's usual `if x < 0 || x > 1.0 { x = default }` guard can never fire.
func TestProgressionFailureFractionDefaultsWhenAbsent(t *testing.T) {
	b := loadBalanceYAML(t, "RollSpread: 0.15\n") // key deliberately absent

	if got := float64(b.ProgressionFailureFraction); got != 0.35 {
		t.Fatalf("ProgressionFailureFraction with the key ABSENT = %v, want 0.35", got)
	}
}

// TestProgressionFailureFractionZeroIsHonoured pins the other half: an
// EXPLICIT 0 must survive validation, because turning the failure award off is
// a legitimate configuration. The sentinel is what makes this expressible.
func TestProgressionFailureFractionZeroIsHonoured(t *testing.T) {
	b := loadBalanceYAML(t, "ProgressionFailureFraction: 0\n")

	if got := float64(b.ProgressionFailureFraction); got != 0 {
		t.Fatalf("explicit 0 became %v; an explicit off-switch must be honoured", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/configs/ -run ProgressionFailureFraction -v`
Expected: FAIL with "b.ProgressionFailureFraction undefined"

- [ ] **Step 3: Declare the field with a sentinel**

In `config.balance.go`, beside `CritProgressionBonus`:

```go
// ProgressionFailureFraction is the share of an ordinary progression event a
// RESOLVED LOSS earns. A win pays 1.0.
//
// Declared with a sentinel default of -1 rather than defaulted in place,
// because 0 is a legal value here (an explicit off-switch) and an ABSENT yaml
// key also unmarshals to 0. The usual `if x < 0 || x > 1.0` guard in this
// package therefore cannot tell "unset" from "deliberately zero", and the knob
// would ship inert. See the U10b-1 spec section 5.6.
ProgressionFailureFraction ConfigFloat `yaml:"ProgressionFailureFraction"`
```

- [ ] **Step 4: Default it in `validateProgression()`**

```go
// -1 is the "unset" sentinel; see the field's doc comment. Anything in
// [0, 1] is taken at face value, including an explicit 0.
if b.ProgressionFailureFraction < 0 || b.ProgressionFailureFraction > 1.0 {
	b.ProgressionFailureFraction = 0.35
}
```

**Seed the sentinel before unmarshal, or none of this works.** Find where the
package constructs a `Balance` prior to `yaml.Unmarshal` and set
`ProgressionFailureFraction: -1` there. After unmarshal, `-1` means the key was
absent and anything in `[0,1]` means it was present, which is the only way an
explicit `0` can be told apart from a missing key.

If the package has no pre-unmarshal construction point, add one; do not settle
for defaulting a zero value, because that silently discards the off-switch. The
two tests in Step 1 fail if you get this wrong: they are written so that they
cannot both pass without a real sentinel.

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/configs/ -run ProgressionFailureFraction -v`
Expected: PASS (both tests)

- [ ] **Step 6: Add the knob to `_datafiles/config.yaml`**

Next to `CritProgressionBonus`:

```yaml
  ProgressionFailureFraction: 0.35   # Share of an event a resolved LOSS earns
```

- [ ] **Step 7: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.progression.go internal/configs/progression_failure_fraction_test.go _datafiles/config.yaml
git commit -m "feat(u10b-1): ProgressionFailureFraction, defaulted past the zero-key trap"
```

---

## Task 4: A scaled stat entry point

`OnSkillUseScaled` rolls the skill's primary stat at a hardcoded `1.0`, and
`OnStatUse` hardcodes `1.0` into `CheckStatProgression`. The owner ruled that
skill and stat both take the failure fraction, so both need a scaled path.

**Files:**
- Modify: `internal/characters/progression.go`
- Test: `internal/characters/progression_scaled_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package characters

import "testing"

// TestOnStatUseScaledPassesMultiplier pins that the stat half of an ordinary
// progression event honours the event's Multiplier. Before U10b-1 a resolved
// LOSS paid a fractional skill roll and a FULL stat roll, because both
// OnStatUse and OnSkillUseScaled's internal stat call hardcoded 1.0.
func TestOnStatUseScaledPassesMultiplier(t *testing.T) {
	c := newProgressionTestCharacter(t)

	var gotMultiplier float64
	restore := stubCheckStatProgression(func(_ string, _ int, m float64) bool {
		gotMultiplier = m
		return false
	})
	defer restore()

	c.OnStatUseScaled("dexterity", 0, 0.35)

	if gotMultiplier != 0.35 {
		t.Fatalf("CheckStatProgression got multiplier %v, want 0.35", gotMultiplier)
	}
}
```

If the package has no seam for stubbing `CheckStatProgression`, assert instead
on the observable outcome: call `OnStatUseScaled` with a multiplier of `0` and
assert the stat's training does not advance over many trials, then with `1.0`
and assert it does. Use `configs.SetConfigForTest` to pin the curve. Do NOT
invent a fixture; `newProgressionTestCharacter` is the real helper in this
package.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/characters/ -run TestOnStatUseScaledPassesMultiplier -v`
Expected: FAIL with "c.OnStatUseScaled undefined"

- [ ] **Step 3: Add `OnStatUseScaled` and route `OnStatUse` through it**

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

- [ ] **Step 4: Make `OnSkillUseScaled`'s internal stat call honour the multiplier**

Replace its trailing primary-stat block:

```go
	// Auto-track and progress the skill's primary governing stat, at the SAME
	// weight as the skill. This used to be a bare OnStatUse, so a fractional
	// skill award still paid a full-rate stat award.
	if primaryStat := skills.GetSkillPrimaryStat(skillName); primaryStat != "" {
		c.OnStatUseScaled(primaryStat, userId, bonusMultiplier)
	}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/characters/ -v`
Expected: PASS

⚠️ Existing tests may now fail because a scaled skill award no longer pays a
full stat roll. That is the intended change. Read each failure and update the
assertion only where it was pinning the old asymmetry; a failure anywhere else
is a real regression.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/progression.go internal/characters/progression_scaled_test.go
git commit -m "feat(u10b-1): scale the stat half of an ordinary progression event"
```

---

## Task 5: `Outcome` learns who lost, and `OrdinaryEvents` scales them

**Files:**
- Modify: `internal/progression/event.go`
- Test: `internal/progression/event_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestOrdinaryEventsScalesTheLoser pins the U10b-1 rule: one event per resolved
// roll, with the losing side paying ProgressionFailureFraction.
func TestOrdinaryEventsScalesTheLoser(t *testing.T) {
	o := progression.Outcome{
		AttackerSkill: "weapon-combat",
		DefenderSkill: "dodge",
		Defended:      true, // the DEFENDER won, so the attacker is the loser
	}

	evs := progression.OrdinaryEventsScaled(o, 0.35)

	byside := map[progression.Side]float64{}
	for _, e := range evs {
		byside[e.Side] = e.Multiplier
	}

	if byside[progression.SideAttacker] != 0.35 {
		t.Fatalf("attacker (loser) multiplier = %v, want 0.35",
			byside[progression.SideAttacker])
	}
	if byside[progression.SideDefender] != 1.0 {
		t.Fatalf("defender (winner) multiplier = %v, want 1.0",
			byside[progression.SideDefender])
	}
}

// TestOrdinaryEventsNeverReconstructsTheLoserFromSuccess pins the polarity
// trap. contest.Result.Success means the ATTACKER won, but under ForceCrit the
// attack wins with Success == false, so !Success is NOT "the defender won".
// Outcome carries Defended precisely so no caller re-derives this.
func TestOrdinaryEventsNeverReconstructsTheLoserFromSuccess(t *testing.T) {
	o := progression.Outcome{
		AttackerSkill: "weapon-combat",
		DefenderSkill: "dodge",
		Defended:      false, // attacker won, even though a ForceCrit sets Success false
	}

	evs := progression.OrdinaryEventsScaled(o, 0.35)
	for _, e := range evs {
		if e.Side == progression.SideAttacker && e.Multiplier != 1.0 {
			t.Fatalf("attacker multiplier = %v, want 1.0: an undefended attack WON",
				e.Multiplier)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/progression/ -run OrdinaryEvents -v`
Expected: FAIL with "undefined: progression.OrdinaryEventsScaled"

- [ ] **Step 3: Add `Defended` to `Outcome` and add `OrdinaryEventsScaled`**

Add to the `Outcome` struct, beside `Floored`:

```go
	// Defended reports that the DEFENDER won this contest.
	//
	// It is carried rather than derived because contest.Result.Success means
	// the ATTACKER won, and !Success is NOT "the defender won": under
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
// The winner is decided by o.Defended alone. See that field's doc comment for
// why Success is not consulted.
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

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/progression/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/progression/event.go internal/progression/event_test.go
git commit -m "feat(u10b-1): a resolved loss earns a fraction of an ordinary event"
```

---

## Task 6: `search.go`, six threshold checks onto the contest core

Each site currently reads `roll := dice.RollStat(searchScore)` then
`if roll.Value >= N`. Replace each with a contest against `N`. The odds change
as tabled in spec section 5.1; that is accepted.

**Files:**
- Modify: `internal/actions/search.go`
- Test: `internal/actions/search_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestSearchUsesTheContestCore pins that search resolves through
// contest.AgainstDifficulty rather than a bare dice.RollStat threshold, so it
// gets the same crit, fumble and margin semantics as every other contest.
func TestSearchUsesTheContestCore(t *testing.T) {
	root := repoRootForTest(t)
	b, err := os.ReadFile(filepath.Join(root, "internal", "actions", "search.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// Four difficulty checks here. The two hidden-actor checks become OPPOSED
	// contests in Task 7, not difficulty checks, so they are neither counted
	// here nor asserted away here; Task 7's test owns them.
	if got := strings.Count(src, "contest.AgainstDifficulty("); got != 4 {
		t.Errorf("contest.AgainstDifficulty call count = %d, want 4", got)
	}
	// The four converted sites must no longer compare a bare roll to a
	// threshold. Two dice.RollStat calls legitimately remain until Task 7.
	if got := strings.Count(src, "dice.RollStat(searchScore)"); got != 2 {
		t.Errorf("dice.RollStat(searchScore) count = %d, want 2 "+
			"(the two hidden-actor sites, converted in Task 7)", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/actions/ -run TestSearchUsesTheContestCore -v`
Expected: FAIL, reporting the bare `dice.RollStat` and a count of 0

- [ ] **Step 3: Convert all six sites**

Secret exits (threshold 125):

```go
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 125.0).Success {
			result.HiddenExitsFound = append(result.HiddenExitsFound, exitName)
```

Hidden containers (threshold 125):

```go
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 125.0).Success {
			char.AddDiscovery(room.RoomId, containerName)
```

Stashed items (threshold 135):

```go
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 135.0).Success {
			result.StashedItemsFound = append(result.StashedItemsFound, SearchStashedItem{
```

Hidden nouns (threshold 175):

```go
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 175.0).Success {
			char.AddDiscovery(room.RoomId, nounKey)
```

**Leave the two hidden-actor sites (players and mobs) alone in this task.** They
become opposed contests in Task 7, not difficulty checks, so converting them
here would rewrite the same lines twice. The test above accounts for this: it
expects exactly two surviving `dice.RollStat(searchScore)` calls, and Task 7's
test drives those to zero.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/actions/ -v`
Expected: PASS

- [ ] **Step 5: Build**

Run: `go build ./...`
Expected: no output

- [ ] **Step 6: Commit**

```bash
git add internal/actions/search.go internal/actions/search_test.go
git commit -m "refactor(u10b-1): search resolves on the contest core"
```

---

## Task 7: A hider's skill counts in both detection paths

Two of `search.go`'s checks answer "does the observer spot the hider?" against a
flat 135 that never reads the hider's sneak score, while `usercommands/go.go`
resolves the same question as an opposed contest. Mobs reach the broken path via
`behaviortree/actions_scout.go`'s `actTrySearch`.

**This is the slice's deliberate behaviour change.**

**Files:**
- Modify: `internal/actions/search.go`
- Test: `internal/actions/search_hidden_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// TestSearchHiddenDetectionReadsTheHidersScore pins the U10b-1 fix. search.go
// used to answer "does the observer spot the hider?" against a flat 135 that
// ignored the hider entirely, while usercommands/go.go resolved the SAME
// question as an opposed contest. A hider's skill decided the outcome in one
// path and was ignored in the other.
func TestSearchHiddenDetectionReadsTheHidersScore(t *testing.T) {
	const trials = 400

	weakHiderSpotted := 0
	strongHiderSpotted := 0
	for i := 0; i < trials; i++ {
		if resolveHiddenDetectionForTest(150, 50) { // observerScore, hiddenScore
			weakHiderSpotted++
		}
		if resolveHiddenDetectionForTest(150, 250) {
			strongHiderSpotted++
		}
	}

	if strongHiderSpotted >= weakHiderSpotted {
		t.Fatalf("a skilled hider (%d/%d spotted) fared no better than an "+
			"unskilled one (%d/%d): the hider's score is being ignored",
			strongHiderSpotted, trials, weakHiderSpotted, trials)
	}
}
```

Name the extracted helper to match whatever `search.go` ends up exporting; the
assertion is what matters, not the helper's name.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/actions/ -run TestSearchHiddenDetectionReadsTheHidersScore -v`
Expected: FAIL, either to compile or because the two counts are equal

- [ ] **Step 3: Resolve both sites as opposed contests**

Hidden players:

```go
		rolledAgainstSomething = true
		hiddenScore := hiddenActorScore(&p.Character)
		if combat.RunContest(searchScore, []contest.Entry{{Score: hiddenScore}}).Success {
			result.HiddenPlayersFound = append(result.HiddenPlayersFound, pId)
```

Hidden mobs:

```go
		rolledAgainstSomething = true
		hiddenScore := hiddenActorScore(&m.Character)
		if combat.RunContest(searchScore, []contest.Entry{{Score: hiddenScore}}).Success {
			result.HiddenMobsFound = append(result.HiddenMobsFound, mId)
```

Build `hiddenActorScore` to match the score `usercommands/go.go` already
composes for its hidden-detection contest. Read that site and reuse its
composition rather than inventing a second one; the whole point of this task is
that two implementations disagreed.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/actions/ -v`
Expected: PASS

- [ ] **Step 5: Confirm the mob path benefits too**

Run: `go test ./internal/behaviortree/ -v`
Expected: PASS. `actTrySearch` reaches the same function, so no separate change
is needed; confirm by reading `actions_scout.go` and noting it in the commit.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/search.go internal/actions/search_hidden_test.go
git commit -m "fix(u10b-1): a hider's skill counts when someone searches for them"
```

---

## Task 8: `track.go` onto the contest core

**Files:**
- Modify: `internal/actions/track.go`
- Test: `internal/actions/track_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestTrackUsesTheContestCore pins that following a trail resolves through the
// contest core rather than a bare static threshold.
func TestTrackUsesTheContestCore(t *testing.T) {
	root := repoRootForTest(t)
	b, err := os.ReadFile(filepath.Join(root, "internal", "actions", "track.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	if strings.Contains(src, "dice.RollStat(searchScore)") {
		t.Error("track.go still resolves with a bare dice.RollStat threshold")
	}
	if !strings.Contains(src, "contest.AgainstDifficulty(") {
		t.Error("track.go does not reach contest.AgainstDifficulty")
	}
	if strings.Contains(src, `NOTE(unassigned`) {
		t.Error("the Category B breadcrumb should be removed once the site is converted")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/actions/ -run TestTrackUsesTheContestCore -v`
Expected: FAIL

- [ ] **Step 3: Convert the site and delete its breadcrumb**

`track.go` grades a trail against two thresholds (125 and 175). Resolve the
lower one first as `contest.AgainstDifficulty(searchScore, 125.0)`; if it
succeeds, resolve the upper as `contest.AgainstDifficulty(searchScore, 175.0)`
for the better tier. Delete the `NOTE(unassigned, see UNIFIED_RESOLUTION_ROADMAP
"Category B")` comment, since the site is no longer unassigned.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/actions/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/actions/track.go internal/actions/track_test.go
git commit -m "refactor(u10b-1): track resolves on the contest core"
```

---

## Task 9: `forage_core.go` onto the contest core

**Files:**
- Modify: `internal/forager/forage_core.go`
- Test: `internal/forager/forage_core_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestForageUsesTheContestCore pins that a forage attempt resolves through the
// contest core against its per-biome difficulty.
func TestForageUsesTheContestCore(t *testing.T) {
	root := repoRootForTest(t)
	b, err := os.ReadFile(filepath.Join(root, "internal", "forager", "forage_core.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	if strings.Contains(src, "dice.RollStat(a.SearchScore)") {
		t.Error("forage_core.go still resolves with a bare dice.RollStat threshold")
	}
	if !strings.Contains(src, "contest.AgainstDifficulty(") {
		t.Error("forage_core.go does not reach contest.AgainstDifficulty")
	}
	if strings.Contains(src, `NOTE(unassigned`) {
		t.Error("the Category B breadcrumb should be removed once the site is converted")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/forager/ -run TestForageUsesTheContestCore -v`
Expected: FAIL

- [ ] **Step 3: Convert the site**

Replace the `dice.RollStat(a.SearchScore)` threshold comparison with
`contest.AgainstDifficulty(a.SearchScore, biomeDifficulty).Success`, keeping
the existing per-biome difficulty value unchanged. Delete the breadcrumb.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/forager/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/forager/forage_core.go internal/forager/forage_core_test.go
git commit -m "refactor(u10b-1): forage resolves on the contest core"
```

---

## Task 10: Material tier from gold value

`ItemSpec.RarityTier` cannot be used: it is a vendor stock cap, so a higher
value means MORE common; its doc comment claims five values while the data holds
nineteen; and 55 of 208 material files omit it. `ItemSpec.Value` is guaranteed
present because `itemspec.go` runs `if i.Value < 1 { i.AutoCalculateValue() }`.

**Files:**
- Create: `internal/items/tier.go`
- Create: `internal/items/tier_test.go`
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.shops.go`
- Modify: `_datafiles/config.yaml`

- [ ] **Step 1: Write the failing test**

```go
package items

import "testing"

// TestMaterialTierMultiplierBand pins the owner's design: the multiplier runs
// 0.75 for bottom-tier materials to 1.25 for top-tier ones, monotonically in
// gold value.
func TestMaterialTierMultiplierBand(t *testing.T) {
	cheap := MaterialTierMultiplier(1)
	mid := MaterialTierMultiplier(25)
	dear := MaterialTierMultiplier(5000)

	if cheap != 0.75 {
		t.Errorf("bottom-tier multiplier = %v, want 0.75", cheap)
	}
	if dear != 1.25 {
		t.Errorf("top-tier multiplier = %v, want 1.25", dear)
	}
	if !(cheap < mid && mid < dear) {
		t.Errorf("multiplier must rise with value: %v, %v, %v", cheap, mid, dear)
	}
}

// TestMaterialTierMultiplierHandlesZeroValue pins that a value of 0, which
// AutoCalculateValue should prevent but which a hand-authored spec could still
// reach, lands on the cheapest tier rather than panicking or reading as top.
func TestMaterialTierMultiplierHandlesZeroValue(t *testing.T) {
	if got := MaterialTierMultiplier(0); got != 0.75 {
		t.Fatalf("MaterialTierMultiplier(0) = %v, want 0.75", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/items/ -run MaterialTier -v`
Expected: FAIL with "undefined: MaterialTierMultiplier"

- [ ] **Step 3: Implement**

```go
// MaterialTierMultiplier maps an item's gold value onto the craft-difficulty
// band, 0.75 for the cheapest materials up to 1.25 for the dearest.
//
// Gold value, not ItemSpec.RarityTier: RarityTier is a VENDOR STOCK CAP, so a
// higher number means a more common item, inverting the name. Its doc comment
// claims the set is 50/40/30/20/10 while the data holds nineteen distinct
// values, and 55 of 208 material files omit it entirely. Value is guaranteed
// present by AutoCalculateValue.
func MaterialTierMultiplier(goldValue int) float64 {
	b := configs.GetBalanceConfig()
	thresholds := b.MaterialTierGoldThresholds // ascending, len == steps-1
	lo := float64(b.MaterialTierMultiplierMin)
	hi := float64(b.MaterialTierMultiplierMax)

	if len(thresholds) == 0 {
		return lo
	}

	step := 0
	for _, t := range thresholds {
		if goldValue >= t {
			step++
			continue
		}
		break
	}

	return lo + (hi-lo)*float64(step)/float64(len(thresholds))
}
```

- [ ] **Step 4: Declare the knobs**

In `config.balance.go`:

```go
MaterialTierGoldThresholds []int       `yaml:"MaterialTierGoldThresholds"` // Ascending gold values separating material tiers
MaterialTierMultiplierMin  ConfigFloat `yaml:"MaterialTierMultiplierMin"`  // Craft-difficulty multiplier for the cheapest tier (default 0.75)
MaterialTierMultiplierMax  ConfigFloat `yaml:"MaterialTierMultiplierMax"`  // Craft-difficulty multiplier for the dearest tier (default 1.25)
```

Default them in `config.balance.shops.go`, with thresholds
`[]int{5, 15, 40, 120}` giving five tiers. Note the same zero-key trap as Task
3: guard `MaterialTierMultiplierMin` with `<= 0`, not `< 0`.

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/items/ -v`
Expected: PASS

- [ ] **Step 6: Sanity-check the thresholds against real data**

Run:

```bash
grep -rh "^value:" _datafiles/world/dogmud/items/materials-40000/ \
  | awk '{print $2}' | sort -n | uniq -c
```

Confirm the five buckets are populated and that no bucket is empty or holds
almost every material. Adjust the thresholds and re-run before committing. This
is the one failure mode of the gold-value approach: an oddly priced material
lands in the wrong tier.

- [ ] **Step 7: Commit**

```bash
git add internal/items/tier.go internal/items/tier_test.go internal/configs/config.balance.go internal/configs/config.balance.shops.go _datafiles/config.yaml
git commit -m "feat(u10b-1): material tier multiplier from gold value"
```

---

## Task 11: Craft resolves as a contest

**Files:**
- Modify: `internal/crafting/crafting.go`
- Modify: `internal/hooks/NewRound_UserRoundTick.go`
- Modify: `internal/hooks/NewRound_MobRoundTick.go`
- Modify: `internal/mobs/crafter.go`
- Test: `internal/crafting/crafting_contest_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// TestCraftAtRecipeMinimumIsFiftyFifty pins the anchor. At skillLevel equal to
// the recipe minimum with mid-tier materials, score equals difficulty, so the
// contest is 50% - which is exactly the shipped CraftingBaseSuccessChance of 50
// that this replaces.
func TestCraftAtRecipeMinimumIsFiftyFifty(t *testing.T) {
	score := CraftScore(10, 10)              // skillLevel, recipeSkillMinimum
	difficulty := CraftDifficulty(10, 1.0)   // recipeSkillMinimum, materialTierMult

	if score != difficulty {
		t.Fatalf("at the recipe minimum score = %v and difficulty = %v; "+
			"they must be equal so the contest is 50%%", score, difficulty)
	}
}

// TestCraftMasteryReadsTheSameAtEveryTier pins that the per-level bonus is a
// PERCENTAGE, not an addend. Because stdDev derives from the attacker's score,
// success depends on the RATIO of difficulty to score. A flat +5 per level
// would pay 92.8% nine levels above a skill_minimum 0 recipe but only 73% nine
// levels above a skill_minimum 40 one.
func TestCraftMasteryReadsTheSameAtEveryTier(t *testing.T) {
	lowRatio := CraftDifficulty(0, 1.0) / CraftScore(9, 0)
	highRatio := CraftDifficulty(40, 1.0) / CraftScore(49, 40)

	if math.Abs(lowRatio-highRatio) > 0.001 {
		t.Fatalf("difficulty/score ratio nine levels above the minimum is %v on "+
			"a novice recipe and %v on an advanced one; mastery must read the "+
			"same at every tier", lowRatio, highRatio)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/crafting/ -run Craft -v`
Expected: FAIL with "undefined: CraftScore"

- [ ] **Step 3: Implement the two formulas**

```go
// CraftDifficulty is the number a craft attempt is contested against.
//
// Difficulty comes from the MATERIALS: the recipe's skill minimum sets the
// anchor and the dearest ingredient's tier scales it. See the U10b-1 spec
// section 5.1.1.
func CraftDifficulty(recipeSkillMinimum int, materialTierMult float64) float64 {
	b := configs.GetBalanceConfig()
	anchor := float64(b.CraftBaseDifficulty) +
		float64(recipeSkillMinimum)*float64(b.CraftSkillMinWeight)
	return anchor * materialTierMult
}

// CraftScore is the crafter's side of that contest.
//
// The per-level term is a PERCENTAGE of the anchor, not an addend. contest.Run
// derives stdDev from the attacker's score, so success depends on the ratio of
// difficulty to score and never on the gap: a flat addend would make the same
// number of levels above the minimum worth far less on an advanced recipe.
func CraftScore(skillLevel, recipeSkillMinimum int) float64 {
	b := configs.GetBalanceConfig()
	anchor := float64(b.CraftBaseDifficulty) +
		float64(recipeSkillMinimum)*float64(b.CraftSkillMinWeight)
	levels := float64(skillLevel - recipeSkillMinimum)
	return anchor * (1.0 + levels*float64(b.CraftSkillWeightPct))
}
```

Declare `CraftBaseDifficulty` (100), `CraftSkillMinWeight` (5) and
`CraftSkillWeightPct` (0.05) in `config.balance.go`, defaulted in
`config.balance.shops.go`, and add them to `_datafiles/config.yaml`.

- [ ] **Step 4: Route the four call sites through the contest**

At each of `NewRound_UserRoundTick.go`, `NewRound_MobRoundTick.go` and both
sites in `mobs/crafter.go`, replace:

```go
chance := crafting.CalcSuccessChance(sl, recipe.SkillMinimum)
roll := util.Rand(100)
util.LogRoll("Craft", roll, chance)
if roll < chance {
```

with:

```go
tierMult := crafting.DearestIngredientTier(recipe)
res := contest.AgainstDifficulty(
	crafting.CraftScore(sl, recipe.SkillMinimum),
	crafting.CraftDifficulty(recipe.SkillMinimum, tierMult),
)
if res.Success {
```

Add `DearestIngredientTier(recipe)` to `internal/crafting`, returning
`items.MaterialTierMultiplier` of the highest-`Value` ingredient in the recipe.

- [ ] **Step 5: Retire `CalcSuccessChance` and its clamps**

Delete `CalcSuccessChance` once no caller remains, and remove
`CraftingBaseSuccessChance`, `CraftingSkillBonusPerLevel`,
`CraftingMinSuccessChance` and `CraftingMaxSuccessChance` from
`config.balance.go`, their validators, and `_datafiles/config.yaml`. The
contest floor now provides the mercy band the clamps used to.

- [ ] **Step 6: Run to verify it passes**

Run: `go test ./internal/crafting/ ./internal/hooks/ ./internal/mobs/ -v`
Expected: PASS

- [ ] **Step 7: Build**

Run: `go build ./...`
Expected: no output

- [ ] **Step 8: Commit**

```bash
git add internal/crafting/ internal/hooks/NewRound_UserRoundTick.go internal/hooks/NewRound_MobRoundTick.go internal/mobs/crafter.go internal/configs/ _datafiles/config.yaml
git commit -m "feat(u10b-1): craft difficulty comes from the materials"
```

---

## Task 12: Salvage resolves as a contest

**Files:**
- Modify: `internal/crafting/salvage.go`
- Modify: `internal/actions/salvage.go`
- Test: `internal/crafting/salvage_contest_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// TestSalvageDearMaterialsAreHarderToReclaim pins the design: difficulty rides
// on the tier of the ingredient being recovered, so a top-tier material comes
// back less often than a bottom-tier one at the same skill.
func TestSalvageDearMaterialsAreHarderToReclaim(t *testing.T) {
	cheap := SalvageDifficulty(items.MaterialTierMultiplier(1))
	dear := SalvageDifficulty(items.MaterialTierMultiplier(5000))

	if !(dear > cheap) {
		t.Fatalf("dear material difficulty %v is not above cheap %v", dear, cheap)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/crafting/ -run Salvage -v`
Expected: FAIL with "undefined: SalvageDifficulty"

- [ ] **Step 3: Implement and convert both roll sites**

```go
// SalvageDifficulty is the number a per-unit salvage recovery is contested
// against, scaled by the tier of the material being reclaimed.
func SalvageDifficulty(materialTierMult float64) float64 {
	b := configs.GetBalanceConfig()
	return float64(b.CraftBaseDifficulty) * materialTierMult
}
```

Replace `util.Rand(10000) < int(chance*10000)` in `RollSalvageReturns` and
`RollSalvageReturnsFromSpec` with
`contest.AgainstDifficulty(salvageScore, SalvageDifficulty(tierMult)).Success`,
where `salvageScore` is composed the same way `CraftScore` is, from the
salvage skill level.

- [ ] **Step 4: Retire the clamps**

Remove `SalvageMinChance` and `SalvageMaxChance` from `config.balance.go`,
their validators, and `_datafiles/config.yaml`. `ContestFloor` supplies the
mercy band. Keep `SalvageSoftCap`, `SalvageGoldPerRound` and
`SalvageMaxRounds`, which govern skill scaling and duration, not the roll.

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/crafting/ ./internal/actions/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/crafting/salvage.go internal/actions/salvage.go internal/configs/ _datafiles/config.yaml
git commit -m "feat(u10b-1): salvage resolves on the contest core"
```

---

## Task 13: The sixteen skullduggery sites onto the seam

Sites: `actions/steal.go` x3, `actions/plant.go` x3, `actions/shadow.go` x2,
`usercommands/skill.skullduggery.sneak.go` x2, `usercommands/picklock.go` x2,
`actions/defuse.go`, `usercommands/throw.go`, `mobcommands/flee.go`,
`hooks/NewRound_DoCombat_helpers.go`.

Four of these (**both** sneak sites, **both** picklock sites) call
`CheckSkillProgression` directly, bypassing every entry point. The 2026-08-21
plan found only `mobcommands/sneak.go` and would have left the player's sneak
untouched. One of the sneak sites is the **failure** branch.

**Files:**
- Modify: the twelve files listed above
- Test: `internal/progression/skullduggery_seam_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// TestSkullduggerySitesRouteThroughTheSeam pins that no skullduggery award
// reaches progression without passing an entry point the guard can see. Four
// sites used to call CheckSkillProgression directly, including the FAILURE
// branch of player sneak, so they touched no entry point and no guard.
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/progression/ -run Skullduggery -v`
Expected: FAIL, naming both files

- [ ] **Step 3: Convert each site to an `Outcome`**

Replace each bare award with an `Outcome` carrying `Defended` and applied via
`ApplyProgression(progression.OrdinaryEventsScaled(o, failureFraction), ...)`.

⚠️ **Two traps:**

1. **`Outcome` holds exactly one `AttackerSkill`.** A site awarding both a
   combat skill and skullduggery needs TWO `Outcome` values, not one with two
   skills.
2. **`SkillPrimaryStats["skullduggery"] == "dexterity"`, the same as
   weapon-combat.** Awarding both rolls dexterity twice, on top of the
   unconditional attacker stat gain. Check each site for this collision and
   suppress the duplicate stat by leaving the second `Outcome`'s `Stat` empty.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./... 2>&1 | grep -v "^ok" | head -40`
Expected: no failures

- [ ] **Step 5: Commit**

```bash
git add internal/actions/ internal/usercommands/ internal/mobcommands/ internal/hooks/ internal/progression/
git commit -m "refactor(u10b-1): skullduggery progression routes through the seam"
```

---

## Task 14: The mob spell path adopts the player path's gates

The player spell path applies a self-cast progression penalty, zeroes
progression for an area cast that found no targets, and gates on
`spellBonus > 0`. The mob path has none of the three and fires unconditionally
on `CastComplete`.

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go`
- Test: `internal/hooks/spell_progression_parity_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// TestMobSpellProgressionHasThePlayerGates pins that the mob cast path applies
// the same three gates as the player path: the self-cast penalty, zero
// progression for an area cast that found no targets, and spellBonus > 0.
func TestMobSpellProgressionHasThePlayerGates(t *testing.T) {
	root := repoRootForTest(t)
	b, err := os.ReadFile(filepath.Join(root, "internal", "hooks",
		"NewRound_DoCombat_helpers.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	mobBlock := extractMobCastProgressionBlock(t, src)
	for _, gate := range []string{
		"SelfCastProgressionMultiplier",
		"spellBonus > 0",
	} {
		if !strings.Contains(mobBlock, gate) {
			t.Errorf("the mob cast progression block is missing the %q gate "+
				"that the player path applies", gate)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/hooks/ -run TestMobSpellProgressionHasThePlayerGates -v`
Expected: FAIL, naming the missing gates

- [ ] **Step 3: Extract the player path's gating into a shared helper and call it from both**

Do not copy the three gates into the mob path. Extract them into one function
that both paths call, so a future change cannot reintroduce the divergence.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/hooks/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/NewRound_DoCombat_helpers.go internal/hooks/spell_progression_parity_test.go
git commit -m "fix(u10b-1): mob spell progression adopts the player path's gates"
```

---

## Task 15: Delete the stranded mob-follow roll

`go.go:668-698` is the only hostile-pursuit code on the ordinary movement path,
which `go.go:125` refuses outright while the player is in combat. A successful
flee calls `EndAggro` then `MoveToRoom` and commands only charmed mobs to
follow. Pursuit is being redesigned as authored mob behavior in the behavior
unification arc.

**Files:**
- Modify: `internal/usercommands/go.go`
- Test: `internal/usercommands/go_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestNoHostileMobFollowRoll pins that the stranded hostile-follow roll is
// gone. It sat on the ordinary movement path, which refuses movement entirely
// while the player is in combat, so its only reachable window was a stale-aggro
// edge case. Pursuit becomes authored mob behavior; see the behavior
// unification arc.
func TestNoHostileMobFollowRoll(t *testing.T) {
	root := repoRootForTest(t)
	b, err := os.ReadFile(filepath.Join(root, "internal", "usercommands", "go.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "Mob Follow") {
		t.Fatal("go.go still contains the stranded hostile mob-follow roll")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/usercommands/ -run TestNoHostileMobFollowRoll -v`
Expected: FAIL

- [ ] **Step 3: Delete the block**

Remove the whole `if !isSneaking { ... mobInstanceIds := room.GetMobs(rooms.FindFightingPlayer) ... }`
pursuit block. **Keep** the charmed-mob follow block above it, which is a
different mechanism and is live.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/usercommands/ -v && go build ./...`
Expected: PASS, no build output

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/go.go internal/usercommands/go_test.go
git commit -m "refactor(u10b-1): delete the stranded hostile mob-follow roll"
```

---

## Task 16: Delete first-kill progression

**Files:**
- Modify: `internal/characters/progression.go`
- Modify: `internal/hooks/Death_MobKillCredit.go`
- Test: `internal/hooks/death_mobkillcredit_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestFirstKillAwardsNoProgression pins the owner's ruling, made 2026-08-21 and
// reaffirmed 2026-08-26. Kill TRACKING stays: KD.AddMobKill feeds the kill and
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

	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(
		"internal/hooks/Death_MobKillCredit.go")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "AddMobKill") {
		t.Error("kill TRACKING was deleted along with the progression award; " +
			"AddMobKill feeds the bestiary and must stay")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/hooks/ -run TestFirstKillAwardsNoProgression -v`
Expected: FAIL

- [ ] **Step 3: Delete**

Remove `Character.OnFirstMobKill`, both call sites in
`Death_MobKillCredit.go` (killer and party members), and the player-facing
message "Defeating a new foe hones your combat instincts!". Keep
`KD.AddMobKill` and the surrounding kill bookkeeping.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/hooks/ ./internal/characters/ -v && go build ./...`
Expected: PASS, no build output

- [ ] **Step 5: Update the census in Task 1**

The count in `census_test.go` drops. Re-run, record the new number, and update
it in this same commit so the census stays honest.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/progression.go internal/hooks/Death_MobKillCredit.go internal/hooks/death_mobkillcredit_test.go internal/progression/census_test.go
git commit -m "refactor(u10b-1): delete first-kill progression, keep kill tracking"
```

---

## Task 17: Delete the dead crit stubs, re-anchor the decay test, fix stale comments

**Files:**
- Modify: five `_test.go` files carrying `OnCriticalSuccess`/`OnCriticalFailure` stubs
- Modify: `internal/progression/seam_guard_test.go`
- Modify: `internal/combat/defence_multiplier.go`
- Modify: the file holding `TestCritReceivedProgression_DecaysWithRank`

- [ ] **Step 1: Re-anchor the decay test FIRST**

`TestCritReceivedProgression_DecaysWithRank` is load-bearing: it pins the
owner's condition that the bonus roll decays toward the soft cap, via the real
`statProgressionChance`. But it is named and documented after `OnCritReceived`,
a symbol this slice removes. Rename it and rewrite its doc comment to describe
`Outcome.ToughenStat`, keeping the assertion identical. If you delete first, the
next cleanup sweep silently un-pins the condition.

Run: `go test ./internal/characters/ -run DecaysWithRank -v`
Expected: PASS, unchanged behaviour under the new name

- [ ] **Step 2: Delete the stubs**

Remove the `OnCriticalSuccess` and `OnCriticalFailure` no-op methods from the
fake actors in the five `_test.go` files that define them. They implement no
interface: the methods do not exist on `Character` and have zero production
references.

- [ ] **Step 3: Drop them from the allow-list**

In `internal/progression/seam_guard_test.go`, remove `"OnCriticalSuccess"` and
`"OnCriticalFailure"` from the allow-list map. The guard currently vouches for
two symbols that do not exist, which weakens it.

- [ ] **Step 4: Fix the two stale comments in `defence_multiplier.go`**

1. A comment claims a forced-crit defence "was still progressed exactly as on
   the melee path". It is **not** progressed on the melee path. Correct it.
2. A comment cites a `Margin` negation at one line number; the real negations
   are elsewhere in the file. Re-derive the list and correct it, or drop the
   line numbers and describe the behaviour instead, which will not rot.

- [ ] **Step 5: Run everything**

Run: `go test ./... 2>&1 | grep -v "^ok" | head -40`
Expected: no failures

- [ ] **Step 6: Commit**

```bash
git add internal/ 
git commit -m "chore(u10b-1): delete dead crit stubs, re-anchor the decay pin, fix stale comments"
```

---

## Task 18: The guard that keeps new sites on the convention

**Files:**
- Modify: `internal/progression/seam_guard_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestEveryRolledSiteRoutesThroughTheSeam is U10b-1's standing guard: a new
// uncertain-outcome roll must resolve on the contest core and award through the
// seam, not with a bare util.Rand or dice.RollStat threshold.
//
// The allow-list holds the DELIBERATE exceptions from spec section 3.1. Adding
// an entry is a decision, so it needs a comment saying which exception it is.
func TestEveryRolledSiteRoutesThroughTheSeam(t *testing.T) {
	allowed := map[string]string{
		"internal/behaviortree/actions_progression.go": "authored tutorial grant, spec 3.1",
		"internal/hooks/item_procs.go":                 "item procs stay off-core, spec 7",
		"internal/hooks/NewRound_AutoHeal.go":          "regen ticks, deferred to U10b-2",
	}

	offenders := scanForBareUncertaintyRolls(t, repoRootForTest(t))
	for _, o := range offenders {
		if _, ok := allowed[o.File]; !ok {
			t.Errorf("%s:%d resolves an uncertain outcome with %s, off the "+
				"contest core. Route it through contest.AgainstDifficulty or "+
				"combat.RunContest, or add it to the allow-list with a reason.",
				o.File, o.Line, o.Mechanism)
		}
	}
}
```

- [ ] **Step 2: Run it and read the output carefully**

Run: `go test ./internal/progression/ -run TestEveryRolledSiteRoutesThroughTheSeam -v`
Expected: FAIL initially, listing whatever remains.

⚠️ Build `scanForBareUncertaintyRolls` on the **fixed** walker from Task 2. If
this test passes on the first run before you have written the scanner, the
scanner is blind, not the codebase clean.

- [ ] **Step 3: Fix or allow-list each offender, then re-run**

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/progression/seam_guard_test.go
git commit -m "test(u10b-1): guard that new rolled sites stay on the seam"
```

---

## Task 19: Breadcrumbs and roadmap correction

**Files:**
- Modify: `internal/hooks/item_procs.go`, `internal/hooks/NewRound_DoCombat_helpers.go`, `internal/usercommands/target.go`
- Modify: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`

- [ ] **Step 1: Breadcrumb what stays off-core**

Add to `item_procs.go` above the proc gate:

```go
// NOTE(unassigned): this flat util.Rand(100) against an author-set percentage
// decides whether EVERY equipped item's proc fires - lifesteal, stun, drain,
// bleed. It is an uncertain outcome off the contest core. U10b-1 deliberately
// left it (owner, 2026-08-26) because a proc is not a skill use and routing it
// would change how all proc gear feels. Recorded so it is not invisible.
```

Add a matching note to `handleMobWeaponPickup`.

- [ ] **Step 2: Hand the two target-switch sites to U12**

Add to both `ChanceToSwitchTarget` roll sites:

```go
// NOTE(U12): this hand-rolled threshold around combat.ChanceToSwitchTarget is
// an uncertain outcome off the contest core. ChanceToSwitchTarget is a pure
// self-stat formula, so contest.AgainstDifficulty fits it exactly. Assigned to
// U12, whose declared surface is targeting and target-switching.
```

- [ ] **Step 3: Correct the roadmap**

In `UNIFIED_RESOLUTION_ROADMAP.md`, update the Category B row: the real census
is **20 sites, not 8**; name the twelve that carried no breadcrumb; record that
U10b-1 converted 15, U12 owns 2, and 3 stay off-core deliberately.

- [ ] **Step 4: Commit**

```bash
git add internal/hooks/ internal/usercommands/target.go docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md
git commit -m "docs(u10b-1): breadcrumb what stays off-core; correct the Category B census"
```

---

## Task 20: Re-solve the skullduggery multiplier

`SkillProgressionMultipliers[Skullduggery] = 0.83` was solved on measured
play-time rates in U10b-0 Phase D. Adding a failure award to steal, plant and
sneak changes the basis it was fitted against.

- [ ] **Step 1: Record the BEFORE rates**

Run: `python tools/balance/read_combat_analytics.py`

⚠️ The buffer is **cumulative**. Never sum flush lines; read the latest
cumulative figure. Record the skullduggery award rate.

- [ ] **Step 2: Re-solve**

Run: `python tools/balance/u10b_solve_v3.py`

Feed it the new firing conditions (steal, plant and sneak now award on a
resolved loss at `ProgressionFailureFraction`). Record the new multiplier.

- [ ] **Step 3: Apply the new multiplier**

Update `SkillProgressionMultipliers` in `_datafiles/config.yaml`.

- [ ] **Step 4: Commit with the numbers in the message**

```bash
git add _datafiles/config.yaml
git commit -m "balance(u10b-1): re-solve the skullduggery multiplier for the failure award"
```

The commit body must record the before and after rates and the solved value, so
the next person can tell a retune from a regression.

---

## Task 21: Docs, patch notes, and `context.md`

- [ ] **Step 1: Update the affected `context.md` files**

`internal/progression/context.md` gains `OrdinaryEventsScaled` and the
`Defended` field. `internal/crafting/context.md` gains `CraftScore`,
`CraftDifficulty`, `SalvageDifficulty` and loses `CalcSuccessChance`.
`internal/items/context.md` gains `MaterialTierMultiplier`.

⚠️ Verify every symbol you name exists:

```powershell
Select-String -Path internal\crafting\*.go -Pattern '^(func|type|const|var)\s'
```

- [ ] **Step 2: Add a dated `docs/PATCH_NOTES.md` entry**

Player-facing framing. No raw numbers, no em dashes. Cover: failing at something
now teaches you a little; searching, tracking, foraging, crafting and salvaging
are resolved the same way as everything else; how hard a craft is now depends on
the materials you are working with; someone hiding from you is harder to find if
they are good at hiding.

- [ ] **Step 3: Commit**

```bash
git add internal/*/context.md docs/PATCH_NOTES.md
git commit -m "docs(u10b-1): context.md updates and patch notes"
```

---

## Task 22: Pre-push verification and the adversarial playtest

- [ ] **Step 1: Formatting gate**

Run: `gofmt -l internal/ modules/`
Expected: no output

- [ ] **Step 2: Build and full test run**

Run: `go build ./... && go test ./... 2>&1 | grep -v "^ok" | head -40`
Expected: no failures

- [ ] **Step 3: Confirm `LogToFile: false`**

Check `_datafiles/config.yaml`. Note this file has `skip-worktree` set, so build
any commit from the `git show HEAD:` blob, never from disk.

- [ ] **Step 4: Boot test in an isolated detached worktree**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

⚠️ **Exit code 124 is the success case**: the timeout fired because the server
stayed up. Do not grep for the bare word `panic`; `GamePlay.MapConsistencyEnforce`
legitimately has the value `panic`. Clean up with `git worktree remove --force`.

- [ ] **Step 5: Adversarial playtest**

Run: `/playtest local --checkout <abs> bug-finder <goals>.yaml`

The goals file must separate **three signals**, because this slice is not a
no-op in three independent ways:

1. **The convention move.** Does failing at something now visibly teach you a
   little? Does a fumble still feel better than an ordinary miss?
2. **The stealth change.** Is a skilled hider now meaningfully harder to find
   than an unskilled one, from both sides?
3. **Crafting feel.** Does difficulty track the materials? Does mastery read the
   same on a novice recipe and an advanced one? This is the biggest surface and
   the likeliest source of complaints.

⚠️ **The AI port caps 3 commands per round and silently discards the overflow
after echoing it.** A dropped command is indistinguishable from a broken one.
Send one command per batch and verify output before concluding anything.

- [ ] **Step 6: Fix what the playtest finds, then extract findings to memory**

Playtest reports are gitignored. Extract every finding to a memory topic file
before the session ends.

- [ ] **Step 7: Open the PR**

```bash
git push -u origin feature/u10b-1-progression-firing-convention
gh pr create --repo pruuk/DOGMud --base master --head feature/u10b-1-progression-firing-convention --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

⚠️ Always pass `--repo pruuk/DOGMud`. This repo is a fork and `gh` defaults to
the **parent**. A green check is not proof: confirm with
`gh run view <id> --repo pruuk/DOGMud --log-failed`.
