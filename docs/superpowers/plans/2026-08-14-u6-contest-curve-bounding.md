# U6 Contest Curve Bounding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bound every opposed contest in the game to `[12.5%, 87.5%]` with one config knob, and make a defence return a margin-scaled damage multiplier through a single resolver that every attack channel calls.

**Architecture:** Two phases with a PR boundary. **Phase A** collapses eight floor knobs and three wrapper pairs into one `ContestFloor`, changing only floor values. **Phase B** reorders resolution so the floor gates the winner before crits are derived, replaces the boolean defence outcome with a 50-100% mitigation multiplier, and moves melee, ranged, spell and social channels onto one resolver so `avoidance.go` can be deleted.

**Tech Stack:** Go 1.25, `internal/contest` (leaf resolution core), `internal/combat` (scores, resolution, damage), `internal/hooks` (round wiring, procs), `internal/configs` (balance knobs).

**Spec:** `docs/superpowers/specs/2026-08-14-contest-curve-bounding-design.md` (rev 3)
**Roadmap:** `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md` (slice U6)

---

## Read this before Task 1

**Step 0 already landed and is boot-tested.** Commit `1cbadaa20` reset every NPC
skill to 1 (76 files); `2d2350db4` moved The Sentinel to `statpool: 2000` and The
Core Guardian to `2800`. Boot test: exit 124, 0 panics, 1 `Server Ready`. **Do not
redo these.**

**`internal/contest` must stay config-free.** Its package docstring
(`contest.go:10-13`) states the reason: "A Go test binary never loads
`_datafiles/config.yaml`, so a core that read balance config would be tested
against Go defaults, and any knob that legitimately defaults to zero would make
its assertions vacuously true." **The floor is therefore passed in as a
parameter, never read inside the package.** A combat-side wrapper reads the knob.
This is why Task 3 exists.

**The margin sign is opposite in two places.** `contest.Result.Margin` is
ATTACK-positive; `bestDefenseResult.margin` is DEFENCE-positive. The single
conversion lives at `combat_helpers.go:698` (`best.margin = -res.Margin`). Mixing
them compiles cleanly and puts crit on the losing side.

**`contest.Run` does NOT populate `RollResult.Margin` or `.Success`.** It builds
rolls with `dice.Roll`, not `dice.OpposedRoll`. Reading `res.DefenseRoll.Margin`
yields a silent zero. Read `Result.Margin` and `Result.Success`.

**The +-1 sentinel is load-bearing.** `RunWithFloors` stamps `Margin = +-1` on a
flipped outcome, which `ContestCrit` normalises to a near-zero z. That is the
only reason a floor-granted hit cannot also crit.

**Verified before planning, do not re-litigate:**
- `contest.Result` **already has** a `Floored bool` field (`contest.go:82`).
- `bestDefenseResult` **already has** a `defenseFloor bool` field
  (`combat_helpers.go:84`), currently set only by the melee floor branch.
- `contest.Run` has exactly **one** production caller
  (`combat_helpers.go:686`); `contest.RunWithFloors` has exactly **three**, all
  inside `internal/combat/contest_floors.go`.
- `contest.AgainstDifficulty` has **zero** production callers.
- Reflect, lifesteal and on-hit procs all compute from `res.DamageToTarget`, the
  damage actually dealt after mitigation
  (`NewRound_DoCombat_unified.go:384`, `:403`, `:157`).
- Concentration is a **static-difficulty** roll (roadmap category B) and is
  **out of scope**; `ContestFloor` does not govern it. U10 owns it.
- `search.go`, `track.go` and `forager` stay static by decision
  (roadmap:145-160). The two hidden-detection sites in `search.go` are
  UNASSIGNED and **not claimed here**.

**Arc standing rules:** no balance literal under `internal/` (config only);
`context.md` updates ship in this PR, not a follow-up; delete each old path in
the task that migrates it.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/configs/config.balance.go` | **Modify.** Add `ContestFloor`; delete 8 floor knobs + 2 avoidance multipliers. |
| `internal/configs/config.balance.misc.go` | **Modify.** `ContestFloor` default/validation; delete 8 floor validations. |
| `internal/configs/config.balance.combat.go` | **Modify.** Delete `RhetoricAvoidanceDamageMultiplier` default. |
| `internal/configs/config.balance.spells.go` | **Modify.** Delete `SpellAvoidanceDamageMultiplier` default. |
| `internal/configs/smoke_test.go` | **Modify.** Drop assertions on deleted knobs. |
| `internal/contest/contest.go` | **Modify.** `RunWithFloors` takes ONE floor. |
| `internal/combat/run_contest.go` | **Create.** `RunContest`, the single flooring wrapper. |
| `internal/combat/contest_floors.go` | **Delete.** |
| `internal/dice/contest_floors.go` | **Modify.** Retire the package-var floor route. |
| `main.go` | **Modify.** Delete the `dice.SetContestFloors` seed. |
| `floor_pair_guard_test.go` (repo root) | **Delete.** |
| `contest_floor_guard_test.go` (repo root) | **Modify.** Guard `RunContest` as the entry point. |
| `internal/combat/defence_sets.go` | **Create.** `AttackChannel` and `DefenceSetFor`. |
| `internal/combat/combat_helpers.go` | **Modify.** New resolution order; `hitResolution.damageMult`. |
| `internal/combat/crit_floor.go` | **Modify.** New denominators. |
| `internal/combat/avoidance.go` | **Delete.** |
| `internal/actions/combat_taunt.go` | **Modify.** Drop `TryStoicResolve`; use the resolver. |
| `internal/hooks/spell_resolution.go` | **Modify.** Drop `TrySpellDeflection`; use the resolver. |
| `internal/combat/skill_moves.go` | **Modify.** Partial damage, no partial status effect. |
| `internal/hooks/NewRound_DoCombat_unified.go` | **Modify.** `on_block` damage arg; drift gates. |
| `_datafiles/config.yaml` | **Modify.** One knob in, ten out. |
| `internal/contest/context.md`, `internal/combat/context.md`, `internal/dice/context.md` | **Modify.** |

---

# PHASE A -- one floor

Ends shippable: floor values change, nothing else does.

---

### Task 1: Add the `ContestFloor` knob

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.misc.go`
- Test: `internal/configs/config_contestfloor_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/configs/config_contestfloor_test.go`:

```go
package configs

import "testing"

// A Go test binary never loads config.yaml, so validation defaults are what
// tests see. ContestFloor MUST NOT be allowed to validate to zero: a zero floor
// silently disables the whole mechanism in every Go test, which would make the
// bound property test pass vacuously while the real behaviour was untested.
func TestContestFloor_ZeroIsRejected(t *testing.T) {
	b := Balance{ContestFloor: 0}
	b.Validate()
	if b.ContestFloor != 0.125 {
		t.Fatalf("ContestFloor 0 must be replaced by the 0.125 default, got %v", b.ContestFloor)
	}
}

func TestContestFloor_AboveHalfIsRejected(t *testing.T) {
	b := Balance{ContestFloor: 0.6}
	b.Validate()
	if b.ContestFloor != 0.125 {
		t.Fatalf("ContestFloor above 0.5 stops being a last resort, got %v", b.ContestFloor)
	}
}

func TestContestFloor_LegalValueSurvives(t *testing.T) {
	b := Balance{ContestFloor: 0.2}
	b.Validate()
	if b.ContestFloor != 0.2 {
		t.Fatalf("a legal ContestFloor must survive validation, got %v", b.ContestFloor)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/configs/ -run TestContestFloor -v`
Expected: FAIL, `b.ContestFloor undefined (type Balance has no field or method ContestFloor)`

- [ ] **Step 3: Add the field**

In `internal/configs/config.balance.go`, in the `Balance` struct next to the
existing `MinDefenseChance` / `MinAttackHitChance` declarations (around line 25):

```go
	// ContestFloor is the single symmetric last-resort probability for EVERY
	// opposed contest in the game. A symmetric floor F yields the bound
	// [F, 1-F], so 0.125 means: hopelessly overmatched you still succeed one
	// attempt in eight, hopelessly overmatching you are still stopped one in
	// eight.
	//
	// It replaces eight per-channel knobs whose values encoded the cost of a
	// single failure. That distinction is deliberately discarded in favour of
	// one rule (U6).
	//
	// Governs OPPOSED contests only. Static-difficulty rolls (search, track,
	// forage, concentration) are roadmap category B/C and are not floored.
	ContestFloor ConfigFloat `yaml:"ContestFloor"`
```

- [ ] **Step 4: Add validation**

In `internal/configs/config.balance.misc.go`, inside `Validate()`, next to the
existing floor validations around line 138:

```go
	// Zero is REJECTED, not accepted. A Go test binary never loads config.yaml,
	// so a permissive [0, 0.5) check would leave this at zero and silently
	// disable the floor in every Go test.
	if b.ContestFloor <= 0 || b.ContestFloor > 0.5 {
		b.ContestFloor = 0.125
	}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/configs/ -run TestContestFloor -v`
Expected: PASS, 3 tests

- [ ] **Step 6: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.misc.go internal/configs/config_contestfloor_test.go
git commit -m "feat(config): add the single ContestFloor knob

Zero fails validation deliberately: a Go test binary never loads config.yaml,
so a permissive check would leave the floor at zero and silently disable the
mechanism in every test."
```

---

### Task 2: Collapse `contest.RunWithFloors` to one floor

**Files:**
- Modify: `internal/contest/contest.go:165-182`
- Test: `internal/contest/contest_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/contest/contest_test.go`:

```go
// A symmetric floor bounds the outcome to [F, 1-F] from BOTH directions. The
// hopeless attacker is rescued at rate F; the overwhelming attacker is stopped
// at rate F. One parameter expresses both.
func TestRunWithFloors_SymmetricBoundsBothDirections(t *testing.T) {
	const iterations = 20000
	const floor = 0.125

	hopeless := 0
	for i := 0; i < iterations; i++ {
		if RunWithFloors(10, []Entry{{Score: 10000}}, floor).Success {
			hopeless++
		}
	}
	rate := float64(hopeless) / iterations
	if rate < 0.10 || rate > 0.15 {
		t.Fatalf("hopeless attacker should win about %v of the time, got %v", floor, rate)
	}

	overwhelming := 0
	for i := 0; i < iterations; i++ {
		if !RunWithFloors(10000, []Entry{{Score: 10}}, floor).Success {
			overwhelming++
		}
	}
	rate = float64(overwhelming) / iterations
	if rate < 0.10 || rate > 0.15 {
		t.Fatalf("overwhelming attacker should be stopped about %v of the time, got %v", floor, rate)
	}
}

// A floored outcome must carry the sentinel margin, which is the only reason a
// floor-granted hit cannot also crit.
func TestRunWithFloors_FlippedOutcomeCarriesSentinel(t *testing.T) {
	for i := 0; i < 5000; i++ {
		res := RunWithFloors(10, []Entry{{Score: 10000}}, 0.5)
		if res.Floored {
			if res.Margin != 1 && res.Margin != -1 {
				t.Fatalf("a floored outcome must carry the +-1 sentinel, got %v", res.Margin)
			}
			return
		}
	}
	t.Fatal("no floored outcome observed in 5000 attempts at floor 0.5")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/contest/ -run TestRunWithFloors_Symmetric -v`
Expected: FAIL, `not enough arguments in call to RunWithFloors`

- [ ] **Step 3: Change the signature**

Replace `internal/contest/contest.go:165-182` with:

```go
func RunWithFloors(atkScore float64, entries []Entry, floor float64) Result {
	res := Run(atkScore, entries)
	if !res.Contested {
		return res
	}

	floor = clampFloor(floor)
	if floor <= 0 {
		return res
	}

	// At most ONE flip per call. The attacker either lost and is rescued, or
	// won and is stopped -- never both, because drawing twice would change the
	// outcome distribution.
	if rand.Float64() < floor {
		if res.Success {
			res.Success, res.Margin, res.Floored = false, -1, true
		} else {
			res.Success, res.Margin, res.Floored = true, 1, true
		}
	}

	return res
}
```

Also update the docstring above it: delete the paragraph beginning "It
reproduces `dice.OpposedRollStatWithFloors` exactly" and the TRANSITIONAL
paragraph, replacing both with:

```go
// The floor is a PARAMETER, never read from config here. This package is a
// config-free leaf on purpose (see the package docstring): a core that read
// balance config would be tested against Go defaults. internal/combat.RunContest
// is the one place that reads ContestFloor.
//
// A flipped outcome carries a SENTINEL margin of +1 or -1, not its real margin.
// This is load-bearing: ContestCrit normalises that sentinel to a near-zero z,
// which is the only reason a floor-granted hit cannot also be a critical hit.
```

- [ ] **Step 4: Fix the three existing callers so the package compiles**

In `internal/combat/contest_floors.go`, change the three call sites to pass a
single floor, temporarily using the first of each old pair:

```go
	return contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, hit)
```

(All three are deleted in Task 4; this keeps the tree building in between.)

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/contest/ ./internal/combat/ -v -run "TestRunWithFloors"`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/contest/contest.go internal/contest/contest_test.go internal/combat/contest_floors.go
git commit -m "refactor(contest): RunWithFloors takes one symmetric floor

A symmetric floor F yields the bound [F, 1-F] from one number. The floor stays
a parameter so internal/contest remains a config-free leaf."
```

---

### Task 3: Add `combat.RunContest`, the single flooring wrapper

**Files:**
- Create: `internal/combat/run_contest.go`
- Test: `internal/combat/run_contest_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/combat/run_contest_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
)

// RunContest is the ONLY place ContestFloor is read. Everything else in the
// game reaches the floor through it.
func TestRunContest_AppliesTheConfiguredFloor(t *testing.T) {
	const iterations = 20000
	wins := 0
	for i := 0; i < iterations; i++ {
		if RunContest(10, []contest.Entry{{Score: 10000}}).Success {
			wins++
		}
	}
	rate := float64(wins) / iterations
	if rate < 0.10 || rate > 0.15 {
		t.Fatalf("hopeless attacker should be floored to about 0.125, got %v", rate)
	}
}

func TestRunContest_UncontestedIsUntouched(t *testing.T) {
	res := RunContest(100, nil)
	if res.Contested {
		t.Fatal("no entries means no contest")
	}
	if res.Floored {
		t.Fatal("an uncontested result has no outcome to flip")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/combat/ -run TestRunContest -v`
Expected: FAIL, `undefined: RunContest`

- [ ] **Step 3: Write the wrapper**

Create `internal/combat/run_contest.go`:

```go
package combat

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// RunContest is THE entry point for every opposed contest in the game.
//
// It exists so that Balance.ContestFloor is read in exactly one place. Before
// U6 there were three wrapper pairs (maneuver, spell, global) over eight knobs,
// and because config.yaml shipped all three pairs at similar values, wiring a
// site to the wrong pair was invisible in production and became a live balance
// bug the moment one pair was retuned. One value removes the failure mode
// rather than guarding against it.
//
// The floor is passed down rather than read inside internal/contest, which is a
// config-free leaf by design.
//
// SCOPE: opposed contests only. Static-difficulty rolls -- search, track,
// forage, concentration -- are roadmap categories B and C and are deliberately
// unfloored. Do not route them here to "unify" them.
func RunContest(atkScore float64, entries []contest.Entry) contest.Result {
	return contest.RunWithFloors(atkScore, entries, float64(configs.GetBalanceConfig().ContestFloor))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/combat/ -run TestRunContest -v`
Expected: PASS, 2 tests

- [ ] **Step 5: Commit**

```bash
git add internal/combat/run_contest.go internal/combat/run_contest_test.go
git commit -m "feat(combat): add RunContest, the single flooring entry point"
```

---

### Task 4: Delete `contest_floors.go` and migrate its callers

**Files:**
- Delete: `internal/combat/contest_floors.go`
- Delete: `internal/combat/contest_floors_test.go`
- Delete: `internal/combat/global_floors_test.go`
- Delete: `floor_pair_guard_test.go` (repo root)
- Modify: every caller listed below

- [ ] **Step 1: List every caller**

Run:

```bash
grep -rn "RunWithManeuverFloors\|RunWithSpellFloors\|RunWithGlobalFloors\|ManeuverFloors()\|SpellFloors()\|ContestFloors()" --include=*.go . | grep -v _test
```

Expected: the definitions in `internal/combat/contest_floors.go`, plus these
production sites: `combat/skill_moves.go:61`, `combat/grapple.go:82`,
`combat/submission.go:78`, `combat/flee.go:83,105`,
`combat/avoidance.go:28,75`, `hooks/Position_GrappleTick.go:269`,
`hooks/NewRound_MobRoundTick.go:422`, `actions/combat_taunt.go:128`,
`usercommands/throw.go:215`, `hooks/charm_spell.go:75`,
`hooks/spell_resolution.go:272,703,1266,1286`, and the 17 out-of-combat sites
(sneak x2, shadow x1, steal x4, plant x4, defuse x1, `go.go` x4,
`skill.skullduggery.shadow` x1).

- [ ] **Step 2: Replace every call**

Each of `RunWithManeuverFloors(a, d)`, `RunWithSpellFloors(a, d)` and
`RunWithGlobalFloors(a, d)` becomes:

```go
combat.RunContest(a, []contest.Entry{{Score: d}})
```

Inside `internal/combat` drop the package qualifier. Add
`"github.com/GoMudEngine/GoMud/internal/contest"` to the imports of any file
that did not already have it.

- [ ] **Step 3: Delete the files**

```bash
git rm internal/combat/contest_floors.go internal/combat/contest_floors_test.go internal/combat/global_floors_test.go floor_pair_guard_test.go
```

- [ ] **Step 4: Update the surviving root guard**

`contest_floor_guard_test.go` guards `contest.Run`, `contest.AgainstDifficulty`
and `contest.RunWithFloors` as unfloored primitives production must not reach.
Its exemption list names `internal/combat/contest_floors.go`, which no longer
exists. Replace that exemption with `internal/combat/run_contest.go`, and change
the failure message from

> "Use combat.RunWithGlobalFloors, combat.RunWithManeuverFloors or combat.RunWithSpellFloors"

to

> "Use combat.RunContest"

- [ ] **Step 5: Verify the tree builds and tests pass**

Run: `go build ./... && go test ./internal/combat/ ./internal/contest/ ./internal/actions/ ./internal/hooks/ ./internal/usercommands/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add -A internal/ contest_floor_guard_test.go
git rm --cached floor_pair_guard_test.go 2>/dev/null || true
git commit -m "refactor(combat): delete contest_floors.go, one wrapper replaces three

floor_pair_guard_test.go existed only to police which site used which floor
pair. With one pair there is nothing to police."
```

---

### Task 5: Retire the `dice` package-var floor route

**Files:**
- Modify: `internal/dice/contest_floors.go`
- Modify: `main.go:197-199`
- Modify: `internal/behaviortree/actions_skullduggery_test.go:103-105`

- [ ] **Step 1: Find every reader**

Run: `grep -rn "SetContestFloors\|dice.ContestFloors" --include=*.go .`
Expected: `main.go:197-199` (the seed), `internal/dice/contest_floors.go` (the
definitions), `internal/dice/contest_floors_test.go`, and
`internal/behaviortree/actions_skullduggery_test.go:103-105`, which pins the
floors to `(0, 0)` to de-flake a steal test.

- [ ] **Step 2: Delete the seed in `main.go`**

Remove these three lines (around `main.go:197`):

```go
	dice.SetContestFloors(
		float64(configs.GetBalanceConfig().MinContestSuccessChance),
		float64(configs.GetBalanceConfig().MinContestResistChance),
	)
```

- [ ] **Step 3: Delete the package vars and accessors**

In `internal/dice/contest_floors.go` delete `minContestSuccess`,
`minContestResist`, `ContestFloors()` and `SetContestFloors()`. Leave
`OpposedRollStat` and `OpposedRollStatWithFloors` alone. The roadmap assigns
their deletion to U6 as well, but that belongs with the U4 equivalence test that
uses them as an oracle, and is out of scope here. If they reference the deleted
vars, inline the value `0.05` with a comment naming them as deprecated.

- [ ] **Step 4: Fix the skullduggery test pin**

`actions_skullduggery_test.go:103-105` pinned the floors so a steal test would
not flake. Replace the pin with a config override:

```go
	// Was dice.SetContestFloors(0, 0). The dice package-var floor route is gone;
	// the floor now comes from Balance.ContestFloor via combat.RunContest.
	restore := configs.OverrideBalanceForTest(func(b *configs.Balance) { b.ContestFloor = 0.0001 })
	defer restore()
```

If `configs.OverrideBalanceForTest` does not exist, add it to
`internal/configs/config.go` as a test seam:

```go
// OverrideBalanceForTest mutates the live balance config and returns a function
// restoring it. Tests only.
func OverrideBalanceForTest(mutate func(*Balance)) func() {
	ensureConfigValidated()
	configDataLock.Lock()
	defer configDataLock.Unlock()
	original := configData.Balance
	mutate(&configData.Balance)
	return func() {
		configDataLock.Lock()
		defer configDataLock.Unlock()
		configData.Balance = original
	}
}
```

- [ ] **Step 5: Verify**

Run: `go build ./... && go test ./internal/dice/ ./internal/behaviortree/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(dice): retire the package-var contest floor route

contest_floors.go:141 recorded that the global pair read dice globals while the
other two read config, and assigned collapsing the two routes to U6."
```

---

### Task 6: Delete the eight knobs and ship the config

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.misc.go`
- Modify: `internal/configs/smoke_test.go`
- Modify: `_datafiles/config.yaml`
- Modify: `internal/contest/context.md`, `internal/combat/context.md`, `internal/dice/context.md`

- [ ] **Step 1: Delete the field declarations**

From `internal/configs/config.balance.go` remove `MinAttackHitChance`,
`MinDefenseChance`, `MinSpellHitChance`, `MinSpellResistChance`,
`MinManeuverHitChance`, `MinManeuverResistChance`, `MinContestSuccessChance`,
`MinContestResistChance`. Keep `MinAttackCritChance` and `MinDefenseCritChance`
(those are the 1% CRIT floors and survive).

- [ ] **Step 2: Delete their validations**

From `internal/configs/config.balance.misc.go:138-171` remove the eight
corresponding validation blocks. Keep the `ContestFloor` block from Task 1.

- [ ] **Step 3: Fix the smoke test**

`internal/configs/smoke_test.go:36-43` asserts bounds on `MinDefenseChance` and
`MinAttackHitChance`. Replace both assertions with:

```go
	if cfg.Balance.ContestFloor <= 0 || cfg.Balance.ContestFloor > 0.5 {
		t.Errorf("ContestFloor out of bounds: %v", cfg.Balance.ContestFloor)
	}
```

- [ ] **Step 4: Update `config.yaml`**

**`_datafiles/config.yaml` has `skip-worktree` set and carries four permanent
local overrides** (`HttpPort: 8090`, `LogToFile: true`, `LogLevel: "info"`, an
extra `Playtest:` block). Do not commit those. Procedure:

```bash
cp _datafiles/config.yaml /tmp/config.local.backup
git update-index --no-skip-worktree _datafiles/config.yaml
git checkout HEAD -- _datafiles/config.yaml
# edit the HEAD version only: delete the eight Min*Chance keys and their
# comments; add the block below near where MinDefenseChance was
git add _datafiles/config.yaml
git diff --cached _datafiles/config.yaml     # MUST show only the knob changes
git commit -m "config: one ContestFloor replaces eight floor knobs"
cp /tmp/config.local.backup _datafiles/config.yaml
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml       # expect a leading S
```

The block to add:

```yaml
  # ContestFloor: the single last-resort probability for EVERY opposed contest.
  # A symmetric floor F bounds the outcome to [F, 1-F], so 0.125 means a
  # hopelessly outclassed actor still succeeds one attempt in eight, and an
  # overwhelming one is still stopped one attempt in eight.
  #
  # Replaces MinAttackHitChance, MinDefenseChance, MinSpellHitChance,
  # MinSpellResistChance, MinManeuverHitChance, MinManeuverResistChance,
  # MinContestSuccessChance and MinContestResistChance, whose per-channel values
  # encoded the cost of a single failure. That distinction was deliberately
  # dropped in U6 in favour of one rule.
  #
  # Applies to OPPOSED contests only. Static-difficulty rolls (search, track,
  # forage, concentration) are not floored.
  #
  # Zero is not a legal value: it fails validation and reverts to 0.125.
  ContestFloor: 0.125
```

- [ ] **Step 5: Update the three `context.md` files**

Each must describe `RunContest` as the single entry point and state that
`internal/contest` takes the floor as a parameter to stay config-free. Verify
every symbol named exists:

```powershell
Select-String -Path internal\combat\*.go -Pattern '^(func|type|const|var)\s'
```

- [ ] **Step 6: Full verification**

```bash
gofmt -l internal/ modules/     # must print nothing
go build ./...
go test ./...
```

- [ ] **Step 7: Commit and open the Phase A PR**

```bash
git add -A internal/ docs/
git commit -m "chore(u6): delete the eight floor knobs, update context.md"
git push -u origin <branch>
gh pr create --repo pruuk/DOGMud --base master --head <branch> --fill
```

---

# PHASE B -- one resolver

Ends shippable: resolution order and damage model change together.

---

### Task 7: Carry `Floored` into `bestDefenseResult`

**Files:**
- Modify: `internal/combat/combat_helpers.go:78-85` (struct), `:686-720` (builder)
- Test: `internal/combat/best_defence_floored_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/combat/best_defence_floored_test.go`:

```go
package combat

import "testing"

// runBestOfAllDefense discarded the contest.Result, so the floored flag never
// reached resolution. The new order needs it: a floored outcome must not crit.
func TestRunBestOfAllDefense_CarriesFlooredFlag(t *testing.T) {
	src := newTestChar(t, 10)
	tgt := newTestChar(t, 10000)

	sawFloored := false
	for i := 0; i < 5000; i++ {
		best := runBestOfAllDefense(&AttackResult{}, src, tgt,
			[]string{"dodge"}, 10, false, combatContext{})
		if best.floored {
			sawFloored = true
			break
		}
	}
	if !sawFloored {
		t.Fatal("a hopeless attacker must sometimes be rescued by the floor, and that must be visible as best.floored")
	}
}
```

If a `newTestChar` helper does not already exist in the package, add it to this
file:

```go
func newTestChar(t *testing.T, dex int) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Stats.Dexterity.Base = dex
	c.Stats.Dexterity.Recalculate()
	return c
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/combat/ -run TestRunBestOfAllDefense_CarriesFloored -v`
Expected: FAIL, `best.floored undefined`

- [ ] **Step 3: Add the field and populate it**

In `internal/combat/combat_helpers.go`, add to `bestDefenseResult`:

```go
	floored bool // the contest floor CHANGED this outcome; it must not crit
```

In `runBestOfAllDefense`, change `res := contest.Run(atkScore, entries)` to
`res := RunContest(atkScore, entries)` and add inside the `if res.Contested`
block:

```go
		best.floored = res.Floored
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/combat/ -run TestRunBestOfAllDefense_CarriesFloored -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/combat/combat_helpers.go internal/combat/best_defence_floored_test.go
git commit -m "feat(combat): carry the floored flag out of the contest"
```

---

### Task 8: Reorder resolution so the floor gates the winner

**Files:**
- Modify: `internal/combat/combat_helpers.go:830-1008` (`resolveDefenseOutcomeCore`)
- Test: `internal/combat/resolution_order_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/combat/resolution_order_test.go`:

```go
package combat

import "testing"

// The shipped order resolves crits FIRST and returns, so against a defender who
// reliably defence-crits the attack floor is evaluated on almost nothing. The
// floor must gate the winner before crits are derived.
func TestResolution_FloorIsReachableAgainstACritLockDefender(t *testing.T) {
	const iterations = 20000
	src := newTestChar(t, 100)
	tgt := newTestChar(t, 5000)

	hits := 0
	for i := 0; i < iterations; i++ {
		res := resolveMelee(t, src, tgt)
		if res.hit {
			hits++
		}
	}
	rate := float64(hits) / iterations
	if rate < 0.10 {
		t.Fatalf("floor must reach a hopeless attacker: want about 0.125, got %v", rate)
	}
}

// A floored outcome carries the +-1 sentinel, which normalises to a near-zero
// margin. Neither side may crit off it.
func TestResolution_FlooredOutcomeNeverCrits(t *testing.T) {
	const iterations = 30000
	src := newTestChar(t, 100)
	tgt := newTestChar(t, 5000)

	for i := 0; i < iterations; i++ {
		res := resolveMeleeWithBest(t, src, tgt)
		if res.best.floored && (res.res.crit || res.res.defenseCrit) {
			t.Fatal("a floored outcome must not crit on either side")
		}
	}
}
```

Add the two helpers to the same file:

```go
type meleeOutcome struct {
	res  hitResolution
	best bestDefenseResult
}

func resolveMeleeWithBest(t *testing.T, src, tgt *characters.Character) meleeOutcome {
	t.Helper()
	result := &AttackResult{}
	atkScore := float64(src.GetEffectiveDexterity())
	best := runBestOfAllDefense(result, src, tgt, []string{"dodge"}, atkScore, false, combatContext{})
	return meleeOutcome{
		res:  resolveDefenseOutcome(result, best, src, tgt, calcCritThreshold(src, tgt), false, false),
		best: best,
	}
}

func resolveMelee(t *testing.T, src, tgt *characters.Character) hitResolution {
	t.Helper()
	return resolveMeleeWithBest(t, src, tgt).res
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/combat/ -run TestResolution_ -v`
Expected: FAIL on the first test with a hit rate near 0.02, because the defence
crits and returns before the floor is reached.

- [ ] **Step 3: Restructure the resolver**

In `resolveDefenseOutcomeCore`, keep the fumble block (steps 1) exactly as it
is. Replace everything from `// -- Step 2: Crit resolution` through the end of
the function with:

```go
	// ── Step 2: The floor gates the WINNER, before any crit is derived ──────
	//
	// It used to sit after crit resolution, which returned early on five
	// branches. Against a defender who reliably defence-crits that made the
	// attack floor dead code in exactly the matchup it was written for: The
	// Core Guardian defence-crit 96.8% of swings, so the floor saw the other
	// 3.2%. The floor is now the outer gate and crits resolve inside it.
	// RunContest has ALREADY flipped the winner and stamped the +-1 sentinel by
	// the time we get here, so best.margin is post-flip and this one expression
	// covers both the floored and unfloored cases. A floored hit carries
	// res.Margin +1, which the sign conversion turns into best.margin -1, so it
	// reads as an attack win; a floored save is the mirror. Do not special-case
	// best.floored here -- it matters only for the crit gate below.
	attackWon := best.margin <= 0

	// ── Step 3: Crit, only on the side that actually won, only if not floored ─
	if !best.floored {
		if attackWon && attackCrit {
			res.hit = true
			res.crit = true
			res.damageMult = 1.0
			return res
		}
		if !attackWon && defenseCrit {
			res.hit = false
			res.defenseCrit = true
			res.damageMult = 0.0
			setDefenseCritFlags(result, best)
			sendDefenseMessages(result, best, sourceChar, targetChar, isThirdParty)
			return res
		}
	}

	// ── Step 4: Normal outcome ──────────────────────────────────────────────
	if attackWon {
		res.hit = true
		res.damageMult = 1.0
		return res
	}

	// The defence won. Task 10 replaces this with the margin-scaled multiplier;
	// until then a defensive win is still a clean miss.
	res.hit = false
	res.damageMult = 0.0
	sendDefenseMessages(result, best, sourceChar, targetChar, isThirdParty)
	return res
```

Delete the `bal := configs.GetBalanceConfig()` line at the top of the function
if nothing else in it still reads `bal`, and delete the now-unreachable
`MinAttackHitChance` / `MinDefenseChance` branches.

- [ ] **Step 4: Add `damageMult` to `hitResolution`**

```go
type hitResolution struct {
	hit          bool
	crit         bool
	fumble       bool
	doubleFumble bool
	defenseCrit  bool
	damageMult   float64 // 0.0 fully negated, 1.0 full damage; Task 10 fills the middle
	hitRoll      dice.RollResult
}
```

Set `res.damageMult = 1.0` in the defence-fumble branch and `0.0` in both
fumble branches.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/combat/ -run TestResolution_ -v`
Expected: PASS

- [ ] **Step 6: Fix the twelve `hitroll_test.go` tests**

`internal/combat/hitroll_test.go` has twelve `TestResolveDefenseOutcome_*` tests
asserting `res.hit` as a bool. They still compile; check each still expresses a
true statement under the new order and update the ones about floor placement.

- [ ] **Step 7: Commit**

```bash
git add internal/combat/combat_helpers.go internal/combat/resolution_order_test.go internal/combat/hitroll_test.go
git commit -m "fix(combat): the contest floor now gates the winner before crits

It sat after crit resolution, which returned early on five branches, so against
a defence-crit-locked target the attack floor evaluated on ~3% of swings."
```

---

### Task 9: Rewrite the crit floors for the new denominators

**Files:**
- Modify: `internal/combat/crit_floor.go:97-126`
- Test: `internal/combat/crit_floor_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/combat/crit_floor_test.go`:

```go
// The attack crit floor applies to swings that WON THE CONTEST, and the defence
// crit floor to swings the DEFENCE won. Neither keys on "the attack missed",
// which stops being a meaningful question once a defensive win deals damage.
func TestApplyCritFloors_AttackFloorOnlyOnAttackWins(t *testing.T) {
	res := hitResolution{hit: true, damageMult: 1.0}
	best := bestDefenseResult{defenseType: "dodge", margin: -5}
	promoted := 0
	for i := 0; i < 20000; i++ {
		r := res
		applyCritFloors(&r, &AttackResult{}, best, 1.0, 0.0)
		if r.crit {
			promoted++
		}
	}
	if promoted != 20000 {
		t.Fatalf("a floor of 1.0 must promote every attack win, got %d", promoted)
	}
}

func TestApplyCritFloors_DefenceFloorOnlyOnDefenceWins(t *testing.T) {
	res := hitResolution{hit: true, damageMult: 0.3} // partially deflected: defence won
	best := bestDefenseResult{defenseType: "block", margin: 5}
	promoted := 0
	for i := 0; i < 20000; i++ {
		r := res
		applyCritFloors(&r, &AttackResult{}, best, 0.0, 1.0)
		if r.defenseCrit {
			promoted++
		}
	}
	if promoted != 20000 {
		t.Fatalf("a floor of 1.0 must promote every defence win, got %d", promoted)
	}
}

func TestApplyCritFloors_FlooredOutcomeIsNeverPromoted(t *testing.T) {
	res := hitResolution{hit: true, damageMult: 1.0}
	best := bestDefenseResult{defenseType: "dodge", margin: -1, floored: true}
	for i := 0; i < 5000; i++ {
		r := res
		applyCritFloors(&r, &AttackResult{}, best, 1.0, 1.0)
		if r.crit || r.defenseCrit {
			t.Fatal("a floored outcome must never be promoted to a crit")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/combat/ -run TestApplyCritFloors_ -v`
Expected: FAIL, because the current implementation branches on `res.hit`, so the
partially-deflected case takes the attack branch.

- [ ] **Step 3: Rewrite `applyCritFloors`**

Replace `internal/combat/crit_floor.go:97-126` with:

```go
func applyCritFloors(res *hitResolution, result *AttackResult, best bestDefenseResult, attackFloor, defenseFloor float64) {
	// A fumble is the attacker's own blunder, not a contested outcome.
	if res.fumble {
		return
	}

	// A floored outcome carries the +-1 sentinel margin and represents an
	// outcome the contest did not actually produce. Promoting it to a crit
	// would hand a decisive result to a side that lost the roll.
	if best.floored {
		return
	}

	// Require that a defence was mounted. defenseType "" means the defender
	// never acted, and setDefenseCritFlags would have no flag to set.
	if best.defenseType == "" {
		return
	}

	// DENOMINATORS, decided in U6. The attack floor applies to swings that WON
	// THE CONTEST; the defence floor to swings the DEFENCE won. Before U6 the
	// split was `res.hit` versus a miss, which stops being answerable once a
	// defensive win deals partial damage: every partially deflected swing has
	// res.hit == true while the defence is the side that won.
	//
	// best.margin is DEFENCE-positive.
	if best.margin <= 0 {
		res.crit = ApplyCritFloor(res.crit, attackFloor)
		return
	}

	wasCrit := res.defenseCrit
	res.defenseCrit = ApplyCritFloor(res.defenseCrit, defenseFloor)
	if res.defenseCrit && !wasCrit {
		// Downstream riposte / auto-trip / auto-bash wiring reads the
		// per-defence flags, so a promotion that skipped these would produce a
		// crit that nothing acts on.
		setDefenseCritFlags(result, best)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/combat/ -run TestApplyCritFloors_ -v`
Expected: PASS, 3 tests

- [ ] **Step 5: Commit**

```bash
git add internal/combat/crit_floor.go internal/combat/crit_floor_test.go
git commit -m "fix(combat): crit floors key on who won, not on whether it hit

Once a defensive win deals partial damage, res.hit is true on swings the
defence won, so the old split sent them to the attacker's floor."
```

---

### Task 10: The margin-scaled defence multiplier

**Files:**
- Modify: `internal/combat/combat_helpers.go` (the defence-won branch)
- Create: `internal/combat/defence_multiplier.go`
- Test: `internal/combat/defence_multiplier_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/combat/defence_multiplier_test.go`:

```go
package combat

import "testing"

// A bare defensive win mitigates 50%; mitigation rises linearly with the
// defender's margin and reaches 100% at the crit threshold.
func TestDefenceMitigation_CurveEndpoints(t *testing.T) {
	if got := DefenceMitigation(0); got != 0.5 {
		t.Fatalf("a bare win must mitigate 50%%, got %v", got)
	}
	if got := DefenceMitigation(ContestCritThreshold); got != 1.0 {
		t.Fatalf("at the crit threshold mitigation must reach 100%%, got %v", got)
	}
	if got := DefenceMitigation(ContestCritThreshold * 2); got != 1.0 {
		t.Fatalf("mitigation must clamp at 100%%, got %v", got)
	}
	if got := DefenceMitigation(-5); got != 0.5 {
		t.Fatalf("a negative defence margin is not a defensive win; clamp to the bare 50%%, got %v", got)
	}
}

func TestDefenceMitigation_IsMonotone(t *testing.T) {
	prev := -1.0
	for m := 0.0; m <= ContestCritThreshold; m += 0.05 {
		got := DefenceMitigation(m)
		if got < prev {
			t.Fatalf("mitigation must be monotone in margin: %v then %v", prev, got)
		}
		prev = got
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/combat/ -run TestDefenceMitigation -v`
Expected: FAIL, `undefined: DefenceMitigation`

- [ ] **Step 3: Write the curve**

Create `internal/combat/defence_multiplier.go`:

```go
package combat

// DefenceMitigation maps a defender's NORMALIZED margin onto the fraction of
// damage removed.
//
// Before U6 a defensive win was a clean miss and a spell deflection was a flat
// 0.5, which is two mechanisms answering one question. A bare win now mitigates
// 50%, rising linearly to 100% at ContestCritThreshold. Skill raises the margin,
// so skill raises mitigation continuously rather than in a step.
//
// A defensive CRIT is not this curve: it fully negates and fires the
// counterattack, and is handled before this is reached.
//
// Applied AFTER item mitigation. There is no double count: a crit bypasses item
// mitigation and never receives a defence multiplier, because an attack crit
// beats a non-crit defence outright.
func DefenceMitigation(normalizedDefenceMargin float64) float64 {
	if normalizedDefenceMargin <= 0 {
		return 0.5
	}
	if normalizedDefenceMargin >= ContestCritThreshold {
		return 1.0
	}
	return 0.5 + 0.5*(normalizedDefenceMargin/ContestCritThreshold)
}
```

- [ ] **Step 4: Wire it into the resolver**

In `resolveDefenseOutcomeCore`, replace the Task 8 placeholder defence-won
branch with:

```go
	// The defence won. It no longer produces a clean miss: damage is mitigated
	// 50-100% by the margin. res.hit stays TRUE because damage is dealt, and
	// everything downstream that scales on res.DamageToTarget therefore scales
	// down with it -- reflect, lifesteal and on-hit procs all read the damage
	// actually dealt.
	defMargin, ok := normalizedDefenseMargin(best)
	if !ok {
		defMargin = 0
	}
	res.hit = true
	res.damageMult = 1.0 - DefenceMitigation(defMargin)
	sendDefenseMessages(result, best, sourceChar, targetChar, isThirdParty)
	return res
```

- [ ] **Step 5: Apply the multiplier to damage**

In `internal/combat/combat.go`, where `calcHitDamage` is called (around `:489-526`),
multiply the computed damage by `res.damageMult` before it is written into
`attackResult`. Floor a non-zero multiplier result at 1 so a heavily deflected
swing cannot round to zero:

```go
	if res.damageMult < 1.0 && attackTargetDamage > 0 {
		attackTargetDamage = int(math.Round(float64(attackTargetDamage) * res.damageMult))
		if attackTargetDamage < 1 {
			// Matches CritOrMitigatedDamage:77-79 -- "a hit that lands must do
			// something; 0 reads to the player as a bug." Melee floored at 0
			// before U6 while spells floored at 1; they agree now.
			attackTargetDamage = 1
		}
	}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/combat/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/combat/defence_multiplier.go internal/combat/defence_multiplier_test.go internal/combat/combat_helpers.go internal/combat/combat.go
git commit -m "feat(combat): a defence returns a margin-scaled damage multiplier

A bare defensive win mitigates 50%, rising to 100% at the crit threshold.
Melee damage now floors at 1 like spells, so a heavily deflected swing cannot
round to zero and read as a bug."
```

---

### Task 11: Defence sets as data

**Files:**
- Create: `internal/combat/defence_sets.go`
- Test: `internal/combat/defence_sets_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/combat/defence_sets_test.go`:

```go
package combat

import "testing"

func TestDefenceSetFor_MatchesTheDesignTable(t *testing.T) {
	cases := []struct {
		channel AttackChannel
		want    []string
	}{
		{ChannelMelee, []string{"dodge", "parry", "block"}},
		{ChannelRanged, []string{"dodge", "block"}},
		{ChannelSpellPhysical, []string{"dodge", "block"}},
		{ChannelSpellMental, []string{"quell"}},
		{ChannelSocial, []string{"defy"}},
	}
	for _, c := range cases {
		got := DefenceSetFor(c.channel)
		if len(got) != len(c.want) {
			t.Fatalf("%s: want %v, got %v", c.channel, c.want, got)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("%s: want %v, got %v", c.channel, c.want, got)
			}
		}
	}
}

func TestDefenceSetFor_UnknownChannelIsEmptyNotPanic(t *testing.T) {
	if got := DefenceSetFor("nonsense"); len(got) != 0 {
		t.Fatalf("an unknown channel must yield no defences, got %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/combat/ -run TestDefenceSetFor -v`
Expected: FAIL, `undefined: AttackChannel`

- [ ] **Step 3: Write it**

Create `internal/combat/defence_sets.go`:

```go
package combat

import "github.com/GoMudEngine/GoMud/internal/characters"

// AttackChannel names an attack type. The applicable defence set is a property
// of the channel, which is the whole reason this is data rather than a filter
// function scattered across the resolvers.
type AttackChannel string

const (
	ChannelMelee         AttackChannel = "melee"
	ChannelRanged        AttackChannel = "ranged"
	ChannelSpellPhysical AttackChannel = "spell-physical"
	ChannelSpellMental   AttackChannel = "spell-mental"
	ChannelSocial        AttackChannel = "social"
)

// DefenceSetFor returns the defences that apply to a channel.
//
// Adding a defence to a channel is one row here and nothing else, which is the
// point of the design. Parry is deliberately excluded from ranged and physical
// spells -- you cannot parry a bolt. Dodge is REUSED for physical spells; there
// is no separate physical-spell defence.
//
// quell (Wil + spellcasting x SkillWeight) answers mental spells; defy
// (Wil + rhetoric x SkillWeight) answers social attacks. A set of size one is
// still a contest, not a different mechanism -- that unification is what lets
// avoidance.go be deleted.
func DefenceSetFor(channel AttackChannel) []string {
	switch channel {
	case ChannelMelee:
		return []string{characters.DefenseDodge, characters.DefenseParry, characters.DefenseBlock}
	case ChannelRanged, ChannelSpellPhysical:
		return []string{characters.DefenseDodge, characters.DefenseBlock}
	case ChannelSpellMental:
		return []string{characters.DefenseQuell}
	case ChannelSocial:
		return []string{characters.DefenseDefy}
	default:
		return nil
	}
}
```

- [ ] **Step 4: Add the two new defence constants and scores**

In `internal/characters/combat.go`, next to `DefenseDodge`/`DefenseParry`/
`DefenseBlock`, add:

```go
	DefenseQuell = "quell"
	DefenseDefy  = "defy"
```

and add two cases to `GetDefenseScore`:

```go
	case DefenseQuell:
		// Mental-spell defence. Costs CONVICTION, not stamina.
		spellcasting := float64(c.GetSkillLevel(skills.Spellcasting)) * skillWeight
		return float64(c.Stats.Willpower.ValueAdj) + spellcasting

	case DefenseDefy:
		// Social defence. Costs CONVICTION, not stamina.
		rhetoric := float64(c.GetSkillLevel(skills.Rhetoric)) * skillWeight
		return float64(c.Stats.Willpower.ValueAdj) + rhetoric
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/combat/ ./internal/characters/ -run "TestDefenceSetFor|TestGetDefenseScore" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/combat/defence_sets.go internal/combat/defence_sets_test.go internal/characters/combat.go
git commit -m "feat(combat): defence sets as data, plus quell and defy

The applicable defence set is a property of the attack type. Adding a defence
to a channel is now one row."
```

---

### Task 12: Move spells and social onto the resolver, delete `avoidance.go`

**Files:**
- Delete: `internal/combat/avoidance.go`, `internal/combat/avoidance_test.go`
- Modify: `internal/hooks/spell_resolution.go` (5 call sites + text)
- Modify: `internal/actions/combat_taunt.go:209` (1 call site + text)
- Modify: `internal/combat/contest_sign_test.go`, `internal/actions/contest_sign_taunt_test.go`

- [ ] **Step 1: Confirm the caller set**

Run: `grep -rn "TrySpellDeflection\|TryStoicResolve" --include=*.go .`
Expected: definitions in `avoidance.go:18,65`; callers at
`hooks/spell_resolution.go:400,502,764,1319,1400` and
`actions/combat_taunt.go:209`; tests in `combat/contest_sign_test.go:162,215`
and `actions/contest_sign_taunt_test.go`.

- [ ] **Step 2: Replace each spell call site**

`TrySpellDeflection` returned a damage multiplier of 1.0, 0.5 or 0.0 from a
SECOND independent contest on the defender's Perception. That second roll is the
fragmentation U6 deletes: quell replaces it, and Perception is intentionally no
longer a spell-defence stat. Each call site becomes:

```go
	channel := combat.ChannelSpellMental
	if spellData.TargetDefenseType == "physical" {
		channel = combat.ChannelSpellPhysical
	}
	mult := combat.ResolveChannelDefence(channel, attacker, defender)
	damage = int(float64(damage) * mult)
```

- [ ] **Step 3: Replace the taunt call site**

`actions/combat_taunt.go:209`:

```go
	mult := combat.ResolveChannelDefence(combat.ChannelSocial, attacker, defender)
	damage = int(float64(damage) * mult)
```

- [ ] **Step 4: Add the shared entry point**

Append to `internal/combat/defence_multiplier.go`:

```go
// ResolveChannelDefence runs one opposed contest for a non-physical channel and
// returns the attacker's damage multiplier: 0.0 on a defensive crit, 0.0-0.5 on
// a defensive win, 1.0 when the attack wins.
//
// It is the same contest, floor and mitigation curve the physical pipeline uses;
// only the defence set differs. That is why TrySpellDeflection and
// TryStoicResolve could be deleted rather than ported.
func ResolveChannelDefence(channel AttackChannel, attacker, defender *characters.Character) float64 {
	defences := DefenceSetFor(channel)
	if len(defences) == 0 {
		return 1.0
	}

	atkScore := ChannelAttackScore(channel, attacker)

	entries := make([]contest.Entry, 0, len(defences))
	for _, d := range defences {
		entries = append(entries, contest.Entry{Name: d, Score: defender.GetDefenseScore(d)})
	}

	res := RunContest(atkScore, entries)
	if res.Success {
		return 1.0
	}

	// Defence-positive margin, normalised the same way the physical path does.
	stdDev := res.DefenseRoll.StdDev
	if stdDev <= 0 || res.Floored {
		return 1.0 - DefenceMitigation(0)
	}
	defMargin := -res.Margin / (stdDev * math.Sqrt2)
	if defMargin >= ContestCritThreshold {
		return 0.0 // defensive crit: full negation
	}
	return 1.0 - DefenceMitigation(defMargin)
}

// ChannelAttackScore builds the attacker's score for a non-physical channel.
func ChannelAttackScore(channel AttackChannel, attacker *characters.Character) float64 {
	skillWeight := float64(configs.GetBalanceConfig().SkillWeight)
	switch channel {
	case ChannelSpellMental, ChannelSpellPhysical:
		return float64(attacker.Stats.Willpower.ValueAdj) +
			float64(attacker.GetSkillLevel(skills.Spellcasting))*skillWeight
	case ChannelSocial:
		return float64(attacker.Stats.Charisma.ValueAdj) +
			float64(attacker.GetSkillLevel(skills.Rhetoric))*skillWeight
	default:
		return 0
	}
}
```

- [ ] **Step 5: Delete the old path**

```bash
git rm internal/combat/avoidance.go internal/combat/avoidance_test.go
```

Delete the two tests in `internal/combat/contest_sign_test.go` that call the
removed functions (`TestTrySpellDeflection_FullNegationStillReachable`,
`TestTryStoicResolve_FullNegationStillReachable`), and the corresponding
assertions in `internal/actions/contest_sign_taunt_test.go`.

- [ ] **Step 6: Rewrite the player text**

`avoidance.go`'s shipped text said "deflect" and "resolve". Rename to the quell
and defy vocabulary in `internal/hooks/spell_resolution.go` and
`internal/hooks/NewRound_DoCombat_helpers.go`. A partial outcome must no longer
claim zero damage:

- was: "Your spell is deflected!" -> "%s quells the working, blunting it."
- was: "%s resolves against your taunt!" -> "%s defies you, and the barb loses its edge."

- [ ] **Step 7: Verify**

Run: `go build ./... && go test ./internal/combat/ ./internal/hooks/ ./internal/actions/`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add -A internal/
git commit -m "refactor(combat): fold spell and social defence into the resolver

avoidance.go:52 predicted exactly this. TrySpellDeflection ran a SECOND contest
on the defender's Perception on top of the primary spell roll; quell replaces
it and Perception is intentionally no longer a spell-defence stat."
```

---

### Task 13: Maneuvers take partial damage without partial status

**Files:**
- Modify: `internal/combat/skill_moves.go:52-139`
- Test: `internal/combat/skill_moves_partial_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/combat/skill_moves_partial_test.go`:

```go
package combat

import "testing"

// A maneuver's STATUS EFFECT stays binary -- there is no partially tripped --
// but its damage joins the same partial mechanism as every other channel.
func TestSkillMove_DefendedManeuverDealsDamageButNoStatus(t *testing.T) {
	src := newTestChar(t, 100)
	tgt := newTestChar(t, 5000) // defence wins nearly always

	sawPartial := false
	for i := 0; i < 20000; i++ {
		res := ExecuteSkillMove(src, tgt, SkillMoveTrip)
		if !res.Hit && res.Damage > 0 {
			sawPartial = true
			break
		}
		if !res.Hit && res.StatusApplied {
			t.Fatal("a defended maneuver must not apply its status effect")
		}
	}
	if !sawPartial {
		t.Fatal("a defended maneuver must still deal partial damage")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/combat/ -run TestSkillMove_Defended -v`
Expected: FAIL, because `result.Hit = attackSuccess` is a strict boolean and damage
only fires inside `if attackSuccess`.

- [ ] **Step 3: Restructure `ExecuteSkillMove`**

Replace the `res := RunWithManeuverFloors(attackerScore, defenderScore)` call
(now `RunContest` after Task 4) and the `if attackSuccess` block with:

```go
	res := RunContest(attackerScore, []contest.Entry{{Score: defenderScore}})

	result.Hit = res.Success
	result.StatusApplied = false
	damageMult := 1.0

	if !res.Success {
		// The defence won. The STATUS EFFECT does not land -- there is no
		// partially tripped -- but damage joins the same partial mechanism the
		// physical and spell channels use.
		stdDev := res.DefenseRoll.StdDev
		defMargin := 0.0
		if stdDev > 0 && !res.Floored {
			defMargin = -res.Margin / (stdDev * math.Sqrt2)
		}
		if defMargin >= ContestCritThreshold {
			damageMult = 0.0 // defensive crit: full negation
		} else {
			damageMult = 1.0 - DefenceMitigation(defMargin)
		}
	}

	result.Damage = int(float64(baseDamage) * damageMult)
	if damageMult > 0 && result.Damage < 1 {
		result.Damage = 1
	}

	if res.Success {
		result.StatusApplied = true
		// existing knockdown roll and status application stay here, unchanged
	}
```

Add a `StatusApplied bool` field to `SkillMoveResult` if it does not already
have one.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/combat/ -run TestSkillMove_Defended -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/combat/skill_moves.go internal/combat/skill_moves_partial_test.go
git commit -m "feat(combat): maneuvers deal partial damage on a defended attempt

A partial trip deals damage without the knockdown. The status effect stays
binary; the damage joins the shared partial mechanism."
```

---

### Task 14: Fix the downstream gates that assumed a defence deals zero

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_unified.go:141,144,160-170`
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:128,152`
- Test: `internal/hooks/partial_defence_gates_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/hooks/partial_defence_gates_test.go`:

```go
package hooks

import "testing"

// on_block procs were dispatched with a literal 0 damage on the reasoning that
// "a successful block is a defended swing (res.Hit == false)". Under the
// defence multiplier a blocked swing deals real damage, so a lifesteal-on-block
// proc would heal ratio * 0.
func TestOnBlockProcs_ReceiveTheDamageActuallyDealt(t *testing.T) {
	got := onBlockProcDamage(&SwingEvent{Hit: true, DamageToTarget: 7, DefenseUsed: "block"})
	if got != 7 {
		t.Fatalf("on_block procs must receive the damage actually dealt, got %d", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hooks/ -run TestOnBlockProcs -v`
Expected: FAIL, `undefined: onBlockProcDamage`

- [ ] **Step 3: Fix the `on_block` dispatch**

At `NewRound_DoCombat_unified.go:160-170`, extract the damage argument into a
named helper and pass the real value:

```go
// onBlockProcDamage returns the damage an on_block proc should scale against.
//
// It used to be a literal 0, reasoned from "a successful block is a defended
// swing, so no damage was dealt". U6's defence multiplier falsifies that: a
// blocked swing is mitigated 50-100%, not negated, so a lifesteal-on-block proc
// would have healed ratio * 0 forever.
func onBlockProcDamage(se *SwingEvent) int {
	return se.DamageToTarget
}
```

and change the dispatch to `dispatchItemProcs("on_block", ..., onBlockProcDamage(se))`.

- [ ] **Step 4: Fix the mutation drift gates**

`:144` gates "trickster" drift on `!res.Hit && (Dodge||Parry)`, which under the
multiplier stops firing because a defended swing now has `Hit == true`. Change
both drift gates to key on the defence rather than on the miss:

```go
	// U6: a defended swing now has Hit == true, so these must key on which
	// defence won rather than on the absence of a hit.
	if res.DefenseUsed == characters.DefenseDodge || res.DefenseUsed == characters.DefenseParry {
```

`:141` gates "ironhide" drift on `res.Hit && DamageToTargetReduction > 0`, which
would now fire on every partial deflection. Add `res.DefenseUsed == ""` so it
only counts swings the attacker actually won.

- [ ] **Step 5: Fix darkness message selection**

`NewRound_DoCombat_helpers.go:128,152` switch on `se.DefenseCrit || se.DefenseUsed != ""`
ABOVE `case se.Hit`, so a partially deflected swing narrates as "Your attack is
deflected by something!" implying zero damage. Move the `se.Hit && se.DamageToTarget > 0`
case above the defence case.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/hooks/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/
git commit -m "fix(hooks): downstream gates no longer assume a defence deals zero"
```

---

### Task 15: Player documentation for what U6 changes

U6 makes `quell` and `defy` live player-facing defences and changes what a
successful dodge, parry or block *does*. Both must be documented in this PR.
U11 keeps the broader help sweep and category cleanup; it does not get to be
the reason a shipped mechanic has no helpfile.

**Files:**
- Create: `_datafiles/world/dogmud/templates/help/quell.template`
- Create: `_datafiles/world/dogmud/templates/help/defy.template`
- Modify: `_datafiles/world/dogmud/templates/help/defense.template`
- Modify: `_datafiles/world/dogmud/templates/help/combat.template`
- Modify: `_datafiles/world/dogmud/keywords.yaml`

- [ ] **Step 1: Register the two new topics**

In `_datafiles/world/dogmud/keywords.yaml`, under `help:` -> `command:` ->
`combat:` (the list containing `defense` at line 77), add in alphabetical
position:

```yaml
      - defy
      - quell
```

Then in the `helpAliases:` block near line 229, add:

```yaml
  quell:            [spell-defense, spell defense, magic defense]
  defy:             [social-defense, taunt defense, resist taunt]
```

A helpfile with no registry entry is unreachable. `stow` shipped that way and
is still listed as a discoverability defect.

- [ ] **Step 2: Write `quell.template`**

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">quell</ansi>

Quell is your defence against workings that attack the mind. You
put the spell down before it takes hold, the way you would smother
a flame rather than dodge it.

You do not type quell. Like dodge, parry and block, it happens on
its own when something reaches for you.

<ansi fg="yellow">━━━ How It Works ━━━</ansi>

  <ansi fg="stat">Uses:</ansi> Willpower and your Spellcasting skill
  <ansi fg="stat">Costs:</ansi> Conviction, not stamina
  <ansi fg="stat">Answers:</ansi> Spells that attack the mind

A spell that attacks the body is answered by dodge or block
instead. Which defence applies is decided by the spell, not by you.

<ansi fg="yellow">━━━ Degrees of Success ━━━</ansi>

Quelling is not all or nothing. Put a working down decisively and
it does nothing at all. Only just manage it and the spell still
reaches you, weakened. A barely successful quell is better than
none, and a masterful one is better still.

<ansi fg="yellow">━━━ Improving It ━━━</ansi>

  <ansi fg="stat">Spellcasting:</ansi> the skill that carries most of the weight
  <ansi fg="stat">Willpower:</ansi> improves through use, and raises every mental defence

Casters defend against casters. Studying the art teaches you to
recognise it coming.

<ansi fg="yellow">See Also:</ansi>

  <ansi fg="command">help defy</ansi>, <ansi fg="command">help defense</ansi>, <ansi fg="command">help spellcasting</ansi>
  <ansi fg="command">help conviction</ansi>, <ansi fg="command">help combat</ansi>
```

- [ ] **Step 3: Write `defy.template`**

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">defy</ansi>

Defy is your defence against words meant to wound. You refuse to
rise to it. The taunt lands on someone who will not be moved.

You do not type defy. Like dodge, parry and block, it happens on
its own when someone comes at you with words.

<ansi fg="yellow">━━━ How It Works ━━━</ansi>

  <ansi fg="stat">Uses:</ansi> Willpower and your Rhetoric skill
  <ansi fg="stat">Costs:</ansi> Conviction, not stamina
  <ansi fg="stat">Answers:</ansi> Taunts and other attacks on your composure

Run your conviction dry and your composure goes with it. A fighter
who has been goaded all fight has nothing left to refuse with.

<ansi fg="yellow">━━━ Degrees of Success ━━━</ansi>

Defying is not all or nothing. Refuse a barb outright and it costs
you nothing. Only just hold your temper and it still stings, less
than it would have. Even a poor defiance is worth something.

<ansi fg="yellow">━━━ Improving It ━━━</ansi>

  <ansi fg="stat">Rhetoric:</ansi> the skill that carries most of the weight
  <ansi fg="stat">Willpower:</ansi> improves through use, and raises every mental defence

Knowing how a barb is built is most of knowing how to shrug it off.
The same skill that lets you taunt lets you weather one.

<ansi fg="yellow">See Also:</ansi>

  <ansi fg="command">help quell</ansi>, <ansi fg="command">help defense</ansi>, <ansi fg="command">help rhetoric</ansi>
  <ansi fg="command">help taunt</ansi>, <ansi fg="command">help conviction</ansi>
```

- [ ] **Step 4: Correct `defense.template`**

Three things in it are now false or incomplete.

**(a)** The "Defense Order" list names three defences and uses em dashes, which
the project forbids in player copy. Replace that block with:

```
  <ansi fg="stat">Dodge</ansi>  low stamina cost, always available
  <ansi fg="stat">Parry</ansi>  moderate stamina cost, requires a weapon
  <ansi fg="stat">Block</ansi>  highest stamina cost, requires a shield
  <ansi fg="stat">Quell</ansi>  costs conviction, answers spells that attack the mind
  <ansi fg="stat">Defy</ansi>   costs conviction, answers taunts
```

**(b)** "Each defense costs stamina even if it fails" is now wrong for two of
the five. Change that sentence to:

```
Dodge, parry and block cost stamina even when they fail. Quell and defy cost
conviction instead. Run either pool dry and the defences it pays for collapse,
which is what makes a long fight a war of attrition.
```

**(c)** "Even heavily armored fighters rely on dodge as their first chance to
avoid damage" describes the old all-or-nothing model. Add a new section after
"Defense Order":

```
<ansi fg="yellow">━━━ Degrees of Success ━━━</ansi>

A defence is not all or nothing. Turn a blow aside decisively and it does
nothing at all. Only just manage it and the blow still lands, robbed of most
of its force. The better your defence reads the attack, the less of it reaches
you.

An exceptional defence does more than stop the blow. See Defense Crits under
<ansi fg="command">help combat</ansi>.
```

and change the dodge paragraph's closing words from "as their first chance to
avoid damage" to "as their first chance to take the force out of a blow".

- [ ] **Step 5: Correct `combat.template`**

**(a)** The "Layered Defense System" list at lines 41 to 45 names three
defences. Add:

```
  4. <ansi fg="stat">Quell:</ansi> Willpower + Spellcasting, against spells that
     attack the mind. Costs conviction.
  5. <ansi fg="stat">Defy:</ansi> Willpower + Rhetoric, against taunts. Costs
     conviction.
```

**(b)** Line 28 reads "Can only dodge — no weapon to deflect with". Replace the
em dash with a comma.

**(c)** Add two lines under "Defense Crits", after the riposte/sweep/shield-slam
list, so the crit tier reads as the top of a range rather than the only outcome:

```
A defence that succeeds without being exceptional still helps: it takes the
force out of the blow rather than stopping it outright. The crits below are
what happens when you read the attack almost before it lands.
```

- [ ] **Step 6: Verify every claim in the new copy**

Check each against the code, not against this plan:

```bash
grep -n "DefenseQuell\|DefenseDefy" internal/characters/combat.go   # scores exist
grep -n "PoolConviction" internal/characters/resources.go            # quell/defy cost CP
```

The helpfiles must not state raw numbers. Project rule: player-facing text
describes the feel, never the value.

- [ ] **Step 7: Check the 80-column rule and the dash rule**

```bash
cd _datafiles/world/dogmud/templates/help
awk 'length > 80 {print FILENAME": "FNR": "length}' quell.template defy.template defense.template combat.template
grep -n "—\|–\|&mdash;" quell.template defy.template defense.template combat.template
```

Both must print nothing. ANSI tags do not count toward the visible width, so
judge by rendered text; the awk check is a first pass, not the last word.

- [ ] **Step 8: Verify the topics actually resolve in game**

Boot the server (Task 16 covers the isolated worktree procedure) and run
`help quell`, `help defy`, `help defense` and `help combat`. A helpfile that
exists but is not registered returns "no help found" and looks identical to a
missing file from the player's side.

- [ ] **Step 9: Commit**

```bash
git add _datafiles/world/dogmud/templates/help/ _datafiles/world/dogmud/keywords.yaml
git commit -m "docs(help): quell and defy helpfiles, and correct the defence copy

U6 makes quell and defy live and changes what a successful defence does, so
help defense and help combat both described behaviour the game no longer has.
Registered in keywords.yaml, because an unregistered helpfile is unreachable
and reads to the player as a missing one."
```

---

### Task 16: Full verification and the Phase B PR

- [ ] **Step 1: Formatting and build**

```bash
gofmt -l internal/ modules/     # must print nothing
go build ./...
go test ./...
```

- [ ] **Step 2: Update `docs/PATCH_NOTES.md`**

Add a dated entry in player-facing framing. No raw numbers, no em dashes.
Suggested text:

```markdown
### 2026-08-14

Combat now recognises a glancing blow. A dodge, parry or block that only just
succeeds turns a strike aside rather than erasing it, and a decisive one still
stops it dead. The same rule covers spells and taunts, so a warding mind blunts
a working the way a raised shield blunts a blade.

No opponent is beyond reach any more, and none is a certainty. Even hopelessly
outmatched, you will find an opening now and then, and even overwhelming, you
will be turned aside now and then.
```

- [ ] **Step 3: Boot test in an isolated worktree**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cd C:/tmp/dogmud-boot-check
# set TelnetPort [43333], LocalPort 19999, HttpPort 18090, AIPort 15555,
# LogToFile false in _datafiles/config.yaml
go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1   # exit 124 is SUCCESS
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
cd - && git worktree remove --force C:/tmp/dogmud-boot-check || (rm -rf C:/tmp/dogmud-boot-check && git worktree prune)
```

Do NOT grep the bare word `panic`: the config key
`GamePlay.MapConsistencyEnforce` has the literal VALUE `panic`.

- [ ] **Step 4: Adversarial playtest**

Required by the project's content gate before this is called done. A boot test
verifies the system, never the experience.

```text
/playtest local --checkout <abs> bug-finder 2026-08-03-prepush-sweep.yaml
```

Drive real combat against a trash mob, a mid-tier mob and The Sentinel. Read
every line. Confirm the new partial-deflection text reads correctly, that a
blocked blow that still hurts does not look like a bug, and that maneuver
failures narrate sensibly.

- [ ] **Step 5: Open the PR**

```bash
git push
gh pr create --repo pruuk/DOGMud --base master --head <branch> --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

A green check is not proof: confirm with
`gh run view <id> --repo pruuk/DOGMud --log-failed`.

---

## Out of scope, deliberately

- **The two hidden-detection sites in `actions/search.go`.** They answer the
  same question as `usercommands/go.go`'s opposed contest but with a flat 135
  threshold that ignores the hider entirely. UNASSIGNED at
  `UNIFIED_RESOLUTION_ROADMAP.md:123`; they need a chunk of their own.
- **The rest of `search.go`, `track.go`, `forager`.** Static by decision
  (roadmap:145-160): there is genuinely no opponent.
- **Concentration.** Roadmap U10 owns it, including the 10%-of-pool trigger
  threshold. It is a static-difficulty roll and `ContestFloor` does not govern it.
- **`dice.OpposedRollStat` / `OpposedRollStatWithFloors`.** Deleting them takes
  the U4 equivalence test that uses them as an oracle; that pairing belongs
  together and is not in this plan.
- **`AttemptRecovery` and the knockdown roll.** Roadmap U10 moves them to
  opposed rolls.
- **The broader help sweep and category cleanup.** U11 keeps it. Task 15 covers
  only what U6 itself makes live or makes wrong: the `quell` and `defy`
  helpfiles, and the defence copy that U6 falsifies.
