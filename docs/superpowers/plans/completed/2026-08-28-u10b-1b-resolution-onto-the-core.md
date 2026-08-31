# U10b-1b: Resolution onto the Contest Core — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the last 15 uncertain outcomes off raw rolls and onto the contest core, giving craft and salvage a real difficulty for the first time.

**Architecture:** Three independently shippable phases. **A** converts the static-threshold sites (search ×4, track, forage) to `contest.AgainstDifficulty` — pure conversion, odds shift but nothing is redesigned. **B** designs craft and salvage a difficulty basis and routes them through two NEW named seams, `crafting.RunCraftContest` and `crafting.RunSalvageContest` — one reader per floor, mirroring `combat.RunContest`. This is where `material_tier` finally does something. **C** reconciles search's flat-135 hidden detection with `go.go`'s opposed form — the slice's one deliberate behaviour change, isolated last so a playtest can attribute it.

**Tech Stack:** Go, `internal/contest` (config-free core), `internal/combat` (the floor seams), `internal/crafting`, `internal/items`.

---

## Facts verified against source (2026-08-28)

Built by grepping **before** writing this plan, per the global read-the-code rule. Every row was read from the file named, not recalled.

| Fact | Where | Verified |
|---|---|---|
| `AgainstDifficulty(score, difficulty float64) Result` — delegates to `Run` | `internal/contest/contest.go:132` | ✓ |
| `RunWithFloors(atkScore float64, entries []Entry, floor float64) Result` | `internal/contest/contest.go:150` | ✓ |
| `AgainstDifficulty` has **ZERO** production callers | grep `internal/`, control-checked against `RarityTier` | ✓ |
| `RunWithFloors` has exactly 2 production callers | `run_contest.go:24`, `run_concentration_contest.go:18` | ✓ |
| Root floor guard lives at the **REPO ROOT**, package `main` | `./contest_floor_guard_test.go` | ✓ |
| `guardedRollFuncs` watches `contest.{Run,AgainstDifficulty,RunWithFloors}` | root guard `:29` | ✓ |
| `guardedRollExemptions` keyed **callee package → repo-relative FILE or DIR → reason** | root guard `:53` | ✓ |
| `contestSiteOwners` keyed **`file:func`**, not file | `internal/combat/contest_site_guard_test.go:47` | ✓ |
| `CraftBaseDifficulty`, `CraftSkillMinWeight`, `CraftFloor`, `SalvageFloor` — **ALL NEW**, zero declarations | grep `internal/configs/` | ✓ |
| `GetRecipeByOutputItemId(itemId int) *RecipeSpec` — map-indexed, lazily built | `internal/crafting/crafting.go:133` | ✓ |
| `items.MaterialTierMultiplier(tier int) float64` — shipped PR #73 | `internal/items/material_tier.go` | ✓ |
| `internal/crafting` **already imports** `internal/items`; `items` does NOT import `crafting` — no cycle | `crafting.go:13` | ✓ |
| `CalcSearchScore = Perception.ValueAdj + SkillMultiplier(search)*25` | `internal/actions/skill_helpers.go:87` | ✓ |
| `CalcDetectionScore(c)` and `CalcSneakScoreVsObserver(sneaker, observer, room)` | `internal/actions/skill_helpers.go:67,53` | ✓ |
| `CalcSuccessChance(skill, min) = base + (skill-min)*bonus`, clamped | `internal/crafting/crafting.go:490` | ✓ |
| Shipped clamps: `CraftingMinSuccessChance` 5 / `Max` 95; `SalvageMinChance` 0.15 / `Max` 0.85 | `config.yaml`, `config.balance.shops.go:88` | ✓ |
| `internal/actions` already imports `internal/combat` | `skill_helpers.go` uses `combat.SkillMultiplier` | ✓ |

**Per-skill primary stats** (`internal/skills/skills.go:326`) — craft score varies by discipline, which the spec's `primaryStat` shorthand hides:

| skill | stat | | skill | stat |
|---|---|---|---|---|
| blacksmithing | strength | | jewelcrafting | dexterity |
| alchemy | perception | | enchanting | perception |
| tailoring | dexterity | | salvage | perception |
| cooking | perception | | search | perception |

**The 15 sites, located exactly:**

| Site | File:line | Phase |
|---|---|---|
| secret exits (125) | `internal/actions/search.go:99` | A |
| hidden containers (125) | `internal/actions/search.go:118` | A |
| stashed items (135) | `internal/actions/search.go:137` | A |
| hidden nouns (175) | `internal/actions/search.go:249` | A |
| track (125/175) | `internal/actions/track.go:135` | A |
| forage | `internal/forager/forage_core.go:132` | A |
| craft, user tick | `internal/hooks/NewRound_UserRoundTick.go:517` | B |
| craft, mob tick | `internal/hooks/NewRound_MobRoundTick.go:531` | B |
| craft, crafter ×2 | `internal/mobs/crafter.go:480`, `:546` | B |
| salvage, action | `internal/actions/salvage.go:274` | B |
| salvage, core ×2 | `internal/crafting/salvage.go:48`, `:69` | B |
| hidden PLAYERS (135) | `internal/actions/search.go:187` | **C** |
| hidden MOBS (135) | `internal/actions/search.go:215` | **C** |

---

## File structure

| File | Responsibility | Phase |
|---|---|---|
| `internal/actions/search.go` | modify 4 sites (A) then 2 (C) | A, C |
| `internal/actions/track.go` | modify 1 site | A |
| `internal/forager/forage_core.go` | modify 1 site | A |
| `internal/crafting/difficulty.go` | **new** — `CraftScore`, `CraftDifficulty`, `SalvageDifficulty`, `DearestMaterialTier`, and the two floor seams `RunCraftContest` / `RunSalvageContest`. One home for the formula AND the only place CraftFloor/SalvageFloor are read | B |
| `internal/crafting/difficulty_test.go` | **new** — pins the 50%-at-minimum anchor and the mastery curve | B |
| `internal/crafting/salvage.go` | modify 2 sites | B |
| `internal/actions/salvage.go` | modify 1 site | B |
| `internal/hooks/NewRound_UserRoundTick.go` | modify 1 site | B |
| `internal/hooks/NewRound_MobRoundTick.go` | modify 1 site | B |
| `internal/mobs/crafter.go` | modify 2 sites | B |
| `internal/configs/config.balance.go` | declare 4 knobs | B |
| `internal/configs/config.balance.shops.go` | default 4 knobs | B |
| `_datafiles/config.yaml` | ship 4 knobs (⚠️ skip-worktree, see below) | B |
| `contest_floor_guard_test.go` (repo root) | exemption rows | A, B |
| `internal/combat/contest_site_guard_test.go` | owner rows | C |

⚠️ **`_datafiles/config.yaml` has `skip-worktree`.** Build the commit from the `git show HEAD:` blob, never from disk, and re-check `git ls-files -v` afterwards — `git update-index --cacheinfo` CLEARS the bit.

---

## Phase A — Category B conversions

**These are NOT no-ops.** `contest.Run` rolls the difficulty side too, at a stdDev derived from the attacker's score, so success depends on the RATIO of difficulty to score and outcomes compress toward 50%. At `RollSpread: 0.15` against the 125 threshold: score 100 goes 4.8% → 11.9%, score 175 goes 97.2% → 91.1%. Accepted deliberately (spec 5.1).

### Task A1: Convert search.go's four non-stealth sites

**Files:**
- Modify: `internal/actions/search.go:99, 118, 137, 249`
- Modify: `contest_floor_guard_test.go` (repo root)
- Test: `internal/actions/search_contest_test.go` (new)

- [ ] **Step 1: Write the failing test**

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
)

// TestSearchTiersUseTheContestCore pins that a search tier resolves as a
// CONTEST, not a threshold compare. The observable difference is that the
// difficulty is ROLLED too, so an outclassed searcher is no longer near-hopeless
// and an expert is no longer near-certain.
//
// Asserted as a probability band rather than a single outcome because a contest
// is stochastic by construction. The band is wide enough that only a genuine
// wiring error fails it.
func TestSearchTiersUseTheContestCore(t *testing.T) {
	const trials = 4000
	wins := 0
	for i := 0; i < trials; i++ {
		if contest.AgainstDifficulty(100, 125).Success {
			wins++
		}
	}
	rate := float64(wins) / float64(trials)
	// Threshold form gives 4.8%; contest form gives ~11.9% (spec 5.1 table).
	if rate < 0.07 || rate > 0.18 {
		t.Fatalf("score 100 vs difficulty 125 won %.1f%%, want ~11.9%% "+
			"(the contest form). Below 7%% means it is still a threshold compare.", rate*100)
	}
}
```

- [ ] **Step 2: Run it to verify it passes** (this pins the CORE, not yet the call site)

Run: `go test ./internal/actions/ -run TestSearchTiersUseTheContestCore -v`
Expected: PASS. It documents the target behaviour; A1's real check is Step 5.

- [ ] **Step 3: Convert the four sites**

Replace each of the four `roll := dice.RollStat(searchScore)` / `if roll.Value >= N.0 {` pairs with a contest. The four thresholds are **125, 125, 135, 175** in file order. Example, `search.go:99` (secret exits):

```go
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 125.0).Success {
			result.HiddenExitsFound = append(result.HiddenExitsFound, exitName)
```

Apply the identical shape at `:118` (125), `:137` (135) and `:249` (175). **Do NOT touch `:187` or `:215`** — those are the hidden-detection pair and belong to Phase C.

Add the import `"github.com/GoMudEngine/GoMud/internal/contest"`. Remove the `dice` import **only if** no other use remains in the file — check with `grep -n "dice\." internal/actions/search.go`.

- [ ] **Step 4: Add the guard exemption**

In `contest_floor_guard_test.go`, inside the `"contest"` map:

```go
		// U10b-1b Phase A: search's static-difficulty tiers are Category B and
		// deliberately unfloored — contest.RunContest's doc comment forbids
		// routing them through the opposed floor seam.
		"internal/actions/search.go": "U10b-1b: static-difficulty search tiers, deliberately unfloored (Category B)",
```

- [ ] **Step 5: Run the guards and the package**

Run: `go test ./ -run TestOpposedContestsAreFloored -v && go test ./internal/actions/`
Expected: both PASS. Without the exemption, the root guard names `search.go`.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/search.go internal/actions/search_contest_test.go contest_floor_guard_test.go
git commit -m "refactor(search): four tiers onto contest.AgainstDifficulty"
```

### Task A2: Convert track.go

**Files:**
- Modify: `internal/actions/track.go:135`
- Modify: `contest_floor_guard_test.go`

- [ ] **Step 1: Read the surrounding block**

Run: `sed -n '130,165p' internal/actions/track.go`
`result.RollValue` is consumed downstream — find every reader before changing its meaning:
Run: `grep -rn "RollValue" --include=*.go internal/ | grep -v _test`

- [ ] **Step 2: Convert, preserving RollValue**

`contest.Result` carries the attacker's roll. Keep `result.RollValue` populated so downstream tier comparisons still work:

```go
	res := contest.AgainstDifficulty(searchScore, 125.0)
	result.RollValue = res.AttackRoll.Value
```

⚠️ **Verify the field name before writing it** — run `grep -n "AttackRoll\|type Result struct" -A 12 internal/contest/contest.go`. If track compares `RollValue` against BOTH 125 and 175, it needs **two** contests (one per tier), not one roll compared twice: a single roll reused across tiers is a threshold compare wearing a contest's clothes.

- [ ] **Step 3: Add the exemption**

```go
		"internal/actions/track.go": "U10b-1b: static-difficulty track tiers, deliberately unfloored (Category B)",
```

- [ ] **Step 4: Run**

Run: `go test ./ -run TestOpposedContestsAreFloored && go test ./internal/actions/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/actions/track.go contest_floor_guard_test.go
git commit -m "refactor(track): tier checks onto contest.AgainstDifficulty"
```

### Task A3: Convert forage_core.go

**Files:**
- Modify: `internal/forager/forage_core.go:132`
- Modify: `contest_floor_guard_test.go`

- [ ] **Step 1: Convert**

```go
	if !contest.AgainstDifficulty(a.SearchScore, difficulty).Success {
		return ForageResult{}
	}
	return ForageResult{Found: true, ItemId: pool[util.Rand(len(pool))]}
```

Keep `util` imported — `util.Rand` still picks the item from the pool.

- [ ] **Step 2: Add the exemption**

```go
		"internal/forager/forage_core.go": "U10b-1b: static-difficulty forage check, deliberately unfloored (Category B)",
```

- [ ] **Step 3: Run**

Run: `go test ./ -run TestOpposedContestsAreFloored && go test ./internal/forager/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/forager/forage_core.go contest_floor_guard_test.go
git commit -m "refactor(forage): difficulty check onto contest.AgainstDifficulty"
```

### Task A4: Retire the six stale breadcrumbs

**Files:**
- Modify: `internal/actions/search.go`, `internal/actions/track.go`, `internal/forager/forage_core.go`

- [ ] **Step 1: Rewrite each `NOTE(ASSIGNED TO U10b-1b, Category B)` block**

They currently say the site is still off the core. After A1–A3 four of the six are converted, so each note must say what is now true and what remains. In `search.go` the note must ALSO say the two stealth sites are still pending Phase C — do not delete it wholesale.

- [ ] **Step 2: Verify no stale claims remain**

Run: `grep -rn "ASSIGNED TO U10b-1b" --include=*.go internal/`
Expected: only the two Phase-C stealth references in `search.go`.

- [ ] **Step 3: Commit**

```bash
git add internal/actions/search.go internal/actions/track.go internal/forager/forage_core.go
git commit -m "docs: retire the Category B breadcrumbs Phase A converted"
```

---

## Phase B — Craft and salvage get a difficulty

### Task B1: The four config knobs

**Files:**
- Modify: `internal/configs/config.balance.go` (declare, in the CRAFTING block ~line 476)
- Modify: `internal/configs/config.balance.shops.go` (default, after the crafting block ~line 98)
- Test: `internal/configs/config.balance.craft_difficulty_test.go` (new)

- [ ] **Step 1: Write the failing test**

```go
package configs

import "testing"

// TestCraftDifficultyKnobDefaults pins the spec 5.1.1 anchors. CraftBaseDifficulty
// is 100 because 100 is the human stat baseline: a baseline crafter at exactly
// the recipe minimum then scores 100+min*5 against a difficulty of 100+min*5,
// which is 50% — today's shipped CraftingBaseSuccessChance, reproduced with no
// special case.
func TestCraftDifficultyKnobDefaults(t *testing.T) {
	var b Balance
	b.ValidateConfig()
	if b.CraftBaseDifficulty != 100 {
		t.Errorf("CraftBaseDifficulty = %v, want 100 (the human stat baseline)", b.CraftBaseDifficulty)
	}
	if b.CraftSkillMinWeight != 5 {
		t.Errorf("CraftSkillMinWeight = %v, want 5 (SkillWeight, as everywhere)", b.CraftSkillMinWeight)
	}
	if b.CraftFloor != 0.05 {
		t.Errorf("CraftFloor = %v, want 0.05 (reproduces the 5%%/95%% clamp)", b.CraftFloor)
	}
	if b.SalvageFloor != 0.15 {
		t.Errorf("SalvageFloor = %v, want 0.15 (reproduces the 15%%/85%% clamp)", b.SalvageFloor)
	}
}
```

⚠️ Confirm the validation entry point's real name first: `grep -n "func (b \*Balance)" internal/configs/config.balance*.go | head`. Use whatever it actually is, not `ValidateConfig` on faith.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/configs/ -run TestCraftDifficultyKnobDefaults -v`
Expected: FAIL, `b.CraftBaseDifficulty undefined`

- [ ] **Step 3: Declare the knobs**

In `config.balance.go`, inside the `── CRAFTING ──` block:

```go
	// U10b-1b: craft is an ordinary contest. Difficulty is
	// (CraftBaseDifficulty + SkillMinimum*CraftSkillMinWeight) * materialTierMult.
	// Base is 100 because 100 is the human stat baseline, so a baseline crafter
	// at the recipe minimum sits at exactly 50% with no special case.
	CraftBaseDifficulty ConfigInt   `yaml:"CraftBaseDifficulty"` // Difficulty anchor = the human stat baseline (default 100)
	CraftSkillMinWeight ConfigInt   `yaml:"CraftSkillMinWeight"` // Difficulty added per point of recipe SkillMinimum (default 5)
	CraftFloor          ConfigFloat `yaml:"CraftFloor"`          // Mercy band for craft contests; reproduces the old 5/95 clamp (default 0.05)
	SalvageFloor        ConfigFloat `yaml:"SalvageFloor"`        // Mercy band for salvage contests; reproduces the old 15/85 clamp (default 0.15)
```

- [ ] **Step 4: Default them**

In `config.balance.shops.go`, after the crafting defaults:

```go
	if b.CraftBaseDifficulty <= 0 {
		b.CraftBaseDifficulty = 100
	}
	if b.CraftSkillMinWeight <= 0 {
		b.CraftSkillMinWeight = 5
	}
	// Both floors use the <=0 idiom deliberately: a 0 floor would DELETE the
	// mercy band, and the spec (5.1.1.2) is explicit that removing it and
	// replacing it with nothing is what an earlier draft got wrong. Uncapped
	// salvage is the worse half — at skill 50 the craft-then-salvage loop would
	// retain ~99.9% against 80.75% today, a ~250x cut in the material sink.
	if b.CraftFloor <= 0 {
		b.CraftFloor = 0.05
	}
	if b.SalvageFloor <= 0 {
		b.SalvageFloor = 0.15
	}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/configs/ -run TestCraftDifficultyKnobDefaults -v`
Expected: PASS

- [ ] **Step 6: Ship them in config.yaml**

Add under the crafting section. ⚠️ Build the staged blob from `git show HEAD:_datafiles/config.yaml`, apply the same edit to disk AND to the blob, stage via `git hash-object -w` + `git update-index --cacheinfo`, then **re-assert `git ls-files -v _datafiles/config.yaml` shows `S`**.

- [ ] **Step 7: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.shops.go internal/configs/config.balance.craft_difficulty_test.go
git commit -m "feat(config): craft difficulty and the two mercy floors"
```

### Task B2: `internal/crafting/difficulty.go`

**Files:**
- Create: `internal/crafting/difficulty.go`
- Create: `internal/crafting/difficulty_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package crafting

import (
	"math"
	"testing"
)

// TestCraftDifficultyAnchor pins spec 5.1.1: a baseline crafter (stat 100) at
// exactly the recipe minimum, on neutral-tier materials, is a coin flip. That
// reproduces today's shipped CraftingBaseSuccessChance of 50 without a special
// case, which is the whole reason CraftBaseDifficulty is 100.
func TestCraftDifficultyAnchor(t *testing.T) {
	for _, min := range []int{0, 10, 40} {
		score := CraftScore(100, min)          // stat 100, skill exactly at the minimum
		diff := CraftDifficulty(min, 1.0)      // neutral material tier
		if math.Abs(score-diff) > 0.001 {
			t.Errorf("skill_minimum %d: score %v != difficulty %v; a baseline "+
				"crafter at the minimum must be exactly 50/50", min, score, diff)
		}
	}
}

// TestMaterialTierMovesDifficulty pins that the material half is live: the same
// recipe is harder with rarer materials. Before the PR #73 backfill every
// multiplier was 1.0 and this axis did nothing.
func TestMaterialTierMovesDifficulty(t *testing.T) {
	common := CraftDifficulty(20, 0.75)
	neutral := CraftDifficulty(20, 1.0)
	rarest := CraftDifficulty(20, 1.25)
	if !(common < neutral && neutral < rarest) {
		t.Fatalf("material tier must raise difficulty monotonically: %v / %v / %v",
			common, neutral, rarest)
	}
}

// TestCraftDifficultyIsRatioShaped pins the trap in spec 5.1: stdDev derives
// from the ATTACKER's score and is reused for the difficulty roll, so outcomes
// depend on the RATIO of difficulty to score and never on the gap. A flat
// addend would pay 92.8% nine levels above a skill_minimum 0 recipe but only
// 73% nine levels above a skill_minimum 40 one.
func TestCraftDifficultyIsRatioShaped(t *testing.T) {
	lowRatio := CraftScore(100, 9) / CraftDifficulty(0, 1.0)
	highRatio := CraftScore(100, 49) / CraftDifficulty(40, 1.0)
	if math.Abs(lowRatio-highRatio) > 0.06 {
		t.Fatalf("nine levels above the minimum must feel similar at both ends: "+
			"ratios %v vs %v", lowRatio, highRatio)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/crafting/ -run "CraftDifficulty|MaterialTier" -v`
Expected: FAIL, `undefined: CraftScore`

- [ ] **Step 3: Implement**

```go
package crafting

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
)

// CraftScore composes the crafter's side the way EVERY score in this game is
// composed: stat + skill * SkillWeight. Spec 5.1.1.1 is explicit that a bespoke
// formula has to justify itself against this one, and the draft that tried put
// the recipe anchor on BOTH sides so it cancelled exactly — deleting
// SkillMinimum from the odds and ignoring the crafter's stat entirely.
//
// stat is the PRIMARY STAT OF THE DISCIPLINE, which differs per craft:
// blacksmithing reads strength, alchemy/cooking/enchanting perception,
// tailoring/jewelcrafting dexterity (skills.SkillPrimaryStats). Callers resolve
// it; this function must not guess.
func CraftScore(stat float64, skillLevel int) float64 {
	w := float64(configs.GetBalanceConfig().SkillWeight)
	return stat + float64(skillLevel)*w
}

// CraftDifficulty is the recipe's side. materialTierMult comes from
// items.MaterialTierMultiplier over the DEAREST ingredient actually being
// consumed (spec 5.1.1.3 — resolve concrete items, never a component_tag).
func CraftDifficulty(skillMinimum int, materialTierMult float64) float64 {
	b := configs.GetBalanceConfig()
	base := float64(b.CraftBaseDifficulty)
	perMin := float64(b.CraftSkillMinWeight)
	return (base + float64(skillMinimum)*perMin) * materialTierMult
}

// RunCraftContest is THE entry point for every craft resolution, and the ONE
// place Balance.CraftFloor is read.
//
// 🔴 DO NOT CALL contest.RunWithFloors DIRECTLY FROM A CRAFT SITE. This mirrors
// combat.RunContest, whose doc comment records why: before U6 there were three
// wrapper pairs over eight floor knobs, and because config.yaml shipped them at
// similar values, wiring a site to the WRONG pair was invisible in production
// and became a live balance bug the moment one pair was retuned. Craft and
// salvage floors ship at 0.05 and 0.15 — close enough that passing the wrong
// one would not show up in any test, and would surface only as "crafting feels
// off" after a retune. One named reader per floor policy removes the failure
// mode instead of guarding against it.
func RunCraftContest(score, difficulty float64) contest.Result {
	return contest.RunWithFloors(score, []contest.Entry{{Score: difficulty}},
		float64(configs.GetBalanceConfig().CraftFloor))
}

// RunSalvageContest is THE entry point for every salvage resolution, and the
// ONE place Balance.SalvageFloor is read. See RunCraftContest for why this is a
// named seam rather than an inline call.
func RunSalvageContest(score, difficulty float64) contest.Result {
	return contest.RunWithFloors(score, []contest.Entry{{Score: difficulty}},
		float64(configs.GetBalanceConfig().SalvageFloor))
}

// SalvageDifficulty is the CRAFT difficulty of the item being taken apart:
// as hard to unmake as it was to make (owner, 2026-08-26).
//
// Reading the recipe of the item BEING CONSUMED avoids the trap in an earlier
// draft, which needed the tier of a material that does not exist yet (it is
// being created by the salvage) and could only have found it through
// items.FindSpecByComponentTag — a Go-map iteration that would re-roll the tier
// on every attempt.
//
// Returns ok=false when the item has no recipe. That path is UNREACHABLE today:
// zero items carry salvage_returns, so only crafted items are salvageable.
// Callers must still handle it.
func SalvageDifficulty(itemId int, materialTierMult float64) (float64, bool) {
	r := GetRecipeByOutputItemId(itemId)
	if r == nil {
		return 0, false
	}
	return CraftDifficulty(r.SkillMinimum, materialTierMult), true
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/crafting/ -run "CraftDifficulty|MaterialTier" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/crafting/difficulty.go internal/crafting/difficulty_test.go
git commit -m "feat(crafting): craft and salvage difficulty formula"
```

### Task B3: Deterministic dearest-ingredient resolution

**Files:**
- Modify: `internal/crafting/difficulty.go`
- Modify: `internal/crafting/difficulty_test.go`

🔴 **Spec 5.1.1.3 is a hard constraint.** `items.FindSpecByComponentTag` iterates a Go map and **four items share `component_tag: bottle`** (Clay Flask, Glass Vial, Sealed Phial, Crystalline Decanter). Resolving a recipe's tag through it re-rolls the tier every attempt, swinging an alchemy craft between ~50% and ~88% with no cause a player can observe. It is also wrong in principle: difficulty must ride on the item the player actually consumed.

- [ ] **Step 1: Write the failing test**

```go
// TestDearestTierIsDeterministic pins spec 5.1.1.3. Four items share
// component_tag "bottle", so a map-order resolver would return a different tier
// on different runs. Same inputs must give the same answer, every time.
func TestDearestTierIsDeterministic(t *testing.T) {
	consumed := []int{40043, 40006, 40044, 40045} // clay, glass, phial, decanter
	first := DearestMaterialTier(consumed)
	for i := 0; i < 200; i++ {
		if got := DearestMaterialTier(consumed); got != first {
			t.Fatalf("tier flipped between calls: %v then %v — this is the "+
				"FindSpecByComponentTag map-order trap", first, got)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/crafting/ -run TestDearestTier -v`
Expected: FAIL, `undefined: DearestMaterialTier`

- [ ] **Step 3: Implement over CONCRETE item ids**

```go
// DearestMaterialTier returns the highest MaterialTier among the items that
// will actually be consumed, as a multiplier. Takes CONCRETE item ids, never
// component tags — see the 5.1.1.3 note above.
//
// An empty list, or items that are all untiered, yields the neutral 1.0.
func DearestMaterialTier(consumedItemIds []int) float64 {
	best := 0
	for _, id := range consumedItemIds {
		spec := items.GetItemSpec(id)
		if spec == nil {
			continue
		}
		if spec.MaterialTier > best {
			best = spec.MaterialTier
		}
	}
	return items.MaterialTierMultiplier(best)
}
```

⚠️ Confirm the spec-lookup function's real name before writing it:
Run: `grep -n "func GetItemSpec\|func FindSpec" internal/items/*.go`

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/crafting/ -run TestDearestTier -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/crafting/difficulty.go internal/crafting/difficulty_test.go
git commit -m "feat(crafting): deterministic dearest-ingredient tier resolution"
```

### Task B4: Route the four craft sites

**Files:**
- Modify: `internal/hooks/NewRound_UserRoundTick.go:517`
- Modify: `internal/hooks/NewRound_MobRoundTick.go:531`
- Modify: `internal/mobs/crafter.go:480`, `:546`
- Modify: `contest_floor_guard_test.go`

- [ ] **Step 1: Extract `SelectIngredients` from `ConsumeIngredients`**

**Verified 2026-08-28:** `crafting.ConsumeIngredients(inv, componentInv, recipe)`
(`crafting.go:262`) already walks the player's ACTUAL inventory in order,
matching each item by `componentTagOf(item)` against the recipe's needed tags.
That traversal **is** the deterministic resolution 5.1.1.3 asks for, and its
inventory ordering is exactly settled decision 8 ("bottle tiebreak must be
INVENTORY ORDER, not lowest ItemId" — Glass Vial is 40006, so lowest-id always
wins and orders the rest inverse to quality, deleting the potion aging axis).

So do NOT write a second resolver. Extract the selection half:

```go
// SelectIngredients reports the CONCRETE items ConsumeIngredients would take,
// without taking them. Sharing one traversal is what stops the roll and the
// consumption disagreeing: difficulty must ride on the item the player actually
// spends, not on the recipe's declared tag (spec 5.1.1.3).
//
// Order is inventory order, component bag first, mirroring ConsumeIngredients
// exactly. That ordering is load-bearing — it is the bottle tiebreak.
func SelectIngredients(inv []items.Item, componentInv []items.Item, recipe *RecipeSpec) []items.Item
```

Then make `ConsumeIngredients` call it, so the two can never drift. Pass the
result to `DearestMaterialTier` (Task B3) before the roll.

⚠️ Write a test that pins the shared traversal: given an inventory holding a
Clay Flask (tier 1) and a Crystalline Decanter (tier 4) in that order, the item
`SelectIngredients` names must be the item `ConsumeIngredients` removes.

- [ ] **Step 2: Replace the percentage roll at each site**

Each site currently reads:

```go
	chance := crafting.CalcSuccessChance(sl, recipe.SkillMinimum)
	// ... util.Rand(100) < chance
```

Replace with:

```go
	stat := float64(char.GetStatValue(skills.GetSkillPrimaryStat(recipe.Skill)))
	score := crafting.CraftScore(stat, sl)
	diff := crafting.CraftDifficulty(recipe.SkillMinimum, crafting.DearestMaterialTier(consumedIds))
	won := crafting.RunCraftContest(score, diff).Success
```

🔴 **Call the named seam, never `contest.RunWithFloors` directly.** The site must
not be able to name a floor at all — that is what stops a craft site being wired
to `SalvageFloor` (0.15 against 0.05, invisible in every test).

⚠️ `GetStatValue` returns 0 for an unrecognised stat name. If `recipe.Skill` is ever a skill with no `SkillPrimaryStats` row the score collapses to the skill term alone. Add a fallback to perception and a `mudlog.Warn`, rather than silently crafting at a wrong score.

- [ ] **Step 3: No exemption needed at these sites**

Because they call `crafting.RunCraftContest` rather than `contest.*`, the root
guard never sees them. **Exactly one** exemption covers all seven craft and
salvage sites, added in Task B2:

```go
		// Defines crafting.RunCraftContest and RunSalvageContest — the ONE
		// readers of CraftFloor and SalvageFloor respectively, mirroring
		// combat.RunContest for ContestFloor.
		"internal/crafting/difficulty.go": "defines crafting.RunCraftContest and RunSalvageContest — the ONE CraftFloor/SalvageFloor readers",
```

That the exemption list stays at one line is the signal the seam is right. If
you find yourself adding a second file, a site is calling `contest.*` directly
and has been handed a floor it should not be able to name.

- [ ] **Step 4: Run**

Run: `go test ./ -run TestOpposedContestsAreFloored && go test ./internal/hooks/ ./internal/mobs/ ./internal/crafting/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/NewRound_UserRoundTick.go internal/hooks/NewRound_MobRoundTick.go internal/mobs/crafter.go contest_floor_guard_test.go
git commit -m "refactor(craft): four sites onto contest.RunWithFloors with a real difficulty"
```

### Task B5: Route the three salvage sites

**Files:**
- Modify: `internal/crafting/salvage.go:48`, `:69`
- Modify: `internal/actions/salvage.go:274`
- Modify: `contest_floor_guard_test.go`

- [ ] **Step 1: Replace the per-ingredient chance**

`crafting/salvage.go` currently rolls `util.Rand(10000) < int(chance*10000)` per ingredient off a sqrt curve. Replace the curve with the contest:

```go
	diff, ok := SalvageDifficulty(itemId, tierMult)
	if !ok {
		diff = fallbackSalvageDifficulty(itemSpec) // Step 3
	}
	won := RunSalvageContest(score, diff).Success
```

🔴 Named seam again, for the same reason: the site must not be able to name a
floor. `actions/salvage.go` calls `crafting.RunSalvageContest`.

where `score = perception + salvageSkill*SkillWeight` via `CraftScore`.

- [ ] **Step 2: Retire the sqrt curve deliberately**

Spec 5.1.1.4 records that a blind review proved **no single base value reproduces today's curve** (skill 0 needs 123, skill 15 needs ~172, skill 25 needs ~199) because a contest is a normal CDF of a ratio and saturates earlier. The curve is **replaced**, not approximated. Delete `SalvageMinChance`/`SalvageMaxChance` usage from the roll path but LEAVE the knobs declared — other code and `config.yaml` still reference them, and removing a knob is its own change.

Run `grep -rn "SalvageMinChance\|SalvageMaxChance" --include=*.go internal/` and confirm every remaining reader before deleting anything.

- [ ] **Step 3: Build the unreachable fallback**

```go
// fallbackSalvageDifficulty prices an item with no recipe. UNREACHABLE TODAY:
// zero items in _datafiles/world/dogmud/items/ carry salvage_returns, so only
// crafted items are salvageable. Built because the code path must exist, and
// deliberately NOT tuned — tuning an unreachable path invents a balance claim
// nobody can verify.
func fallbackSalvageDifficulty(spec *items.ItemSpec) float64 {
	b := configs.GetBalanceConfig()
	return float64(b.CraftBaseDifficulty)
}
```

- [ ] **Step 4: No new exemptions**

Covered by the single `internal/crafting/difficulty.go` entry from Task B2. If
the root guard names `salvage.go`, a site is still calling `contest.*` directly
— fix the call, do not add an exemption.

- [ ] **Step 5: Run**

Run: `go test ./ -run TestOpposedContestsAreFloored && go test ./internal/crafting/ ./internal/actions/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/crafting/salvage.go internal/actions/salvage.go contest_floor_guard_test.go
git commit -m "refactor(salvage): onto the contest core, difficulty from the item's own recipe"
```

### Task B6: Retire `CalcSuccessChance` if it is now dead

**Files:**
- Modify: `internal/crafting/crafting.go:487-504`

- [ ] **Step 1: Check for surviving callers**

Run: `grep -rn "CalcSuccessChance" --include=*.go . | grep -v _test`
The compiler is the dead-code sweep — if only the definition remains, delete it and its test. If a caller survives (e.g. a UI that PREVIEWS the odds), keep it and add a doc comment saying it is display-only and no longer decides outcomes.

- [ ] **Step 2: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git commit -am "refactor(crafting): retire the flat success percentage"
```

---

## Phase C — Hidden detection (the one behaviour change)

🔴 **Isolate this in its own commit and call it out in the playtest goals.** The playtest must be able to separate "the convention moved" from "stealth got better against searchers".

### Task C1: Reconcile search.go onto the opposed form

**Files:**
- Modify: `internal/actions/search.go:187` (hidden players), `:215` (hidden mobs)
- Modify: `internal/combat/contest_site_guard_test.go`
- Test: `internal/actions/search_stealth_test.go` (new)

- [ ] **Step 1: Write the failing test**

```go
// TestHiddenDetectionReadsTheHidersSkill pins the substance of spec 5.2: the
// flat 135 threshold NEVER read the hider's sneak score, while go.go resolved
// the identical question as an opposed contest. A hider's skill decided the
// outcome in one path and was ignored in the other.
//
// Asserted as "a better hider is found less often", not as an exact rate,
// because the contest is stochastic.
func TestHiddenDetectionReadsTheHidersSkill(t *testing.T) {
	// ... build one observer and two hiders, identical but for skullduggery rank
	// ... run each pairing N times through the search path
	// ... require foundExpert < foundNovice by a clear margin
}
```

Fill in the fixture using the same construction the existing `internal/actions` tests use — check `grep -ln "characters.Character{" internal/actions/*_test.go` for the closest existing example and follow it.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/actions/ -run TestHiddenDetectionReadsTheHidersSkill -v`
Expected: FAIL — the flat threshold ignores the hider, so both rates match.

- [ ] **Step 3: Convert both sites to go.go's exact form**

`go.go:610-622` is the reference implementation. Mirror it rather than inventing a variant:

```go
		observerScore := CalcDetectionScore(char)
		hiddenScore := CalcSneakScoreVsObserver(&p.Character, char, room)
		if combat.RunContest(observerScore, []contest.Entry{{Score: hiddenScore}}).Success {
			result.HiddenPlayersFound = append(result.HiddenPlayersFound, pId)
```

and the mob twin at `:215`. Note this uses `combat.RunContest` (floored by `ContestFloor`), NOT `contest.AgainstDifficulty` — it is a genuine opposed contest, so it belongs on the opposed seam. `internal/actions` already imports `internal/combat`.

- [ ] **Step 4: Add the ownership rows**

In `internal/combat/contest_site_guard_test.go`:

```go
	"internal/actions/search.go:Search": "U10b-1b: hidden detection reconciled onto the opposed form that go.go already used (spec 5.2)",
```

⚠️ Key on the **enclosing function's real name** — confirm with `grep -n "^func " internal/actions/search.go`.

- [ ] **Step 5: Run**

Run: `go test ./internal/actions/ ./internal/combat/ -run "Hidden|ContestSite"`
Expected: PASS

- [ ] **Step 6: Verify mobs reach the fixed path too**

`behaviortree/actions_scout.go`'s `actTrySearch` reaches this code, gated by `condRoomHasHiddenEntity`. Confirm it still compiles and behaves:
Run: `go test ./internal/behaviortree/`

- [ ] **Step 7: Commit**

```bash
git add internal/actions/search.go internal/actions/search_stealth_test.go internal/combat/contest_site_guard_test.go
git commit -m "fix(search): hidden detection reads the hider's skill"
```

### Task C2: Retire the reconciliation breadcrumb

**Files:**
- Modify: `internal/actions/search.go:158-176`

- [ ] **Step 1: Replace the NOTE block**

It says the two implementations disagree and "whichever chunk claims them must reconcile the two implementations". They are now reconciled — say so, name go.go as the shared form, and keep the warning that this was a deliberate behaviour change so a future reader does not "restore" the flat threshold.

- [ ] **Step 2: Commit**

```bash
git commit -am "docs(search): the two hidden-detection paths are reconciled"
```

---

## Phase D — Ship

### Task D1: Patch notes

**Files:**
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Write a dated entry**

Player-facing framing, no raw numbers, no em dashes, wrapped at 80. Three real changes to describe: crafting difficulty now depends on the materials and on your own aptitude; salvage is as hard as the making was; hiding now works against someone searching for you.

- [ ] **Step 2: Commit**

```bash
git add docs/PATCH_NOTES.md
git commit -m "docs: patch notes for U10b-1b"
```

### Task D2: Full gates

- [ ] **Step 1: gofmt**

Run: `out=$(gofmt -l internal/ modules/); [ -z "$out" ] && echo clean || echo "DIRTY: $out"`
⚠️ `gofmt -l` exits 0 even while listing files. Test the OUTPUT, never the exit code.

- [ ] **Step 2: Build and test**

Run: `go build ./... && go test ./...`
Expected: exit 0. Capture output and exit code in ONE run.

- [ ] **Step 3: The drift gate**

Run: `DOGMUD_BOOT_SMOKE=1 go test -count=1 -run TestSmoke_NoNewSilentlyIgnoredYAMLKeys ./`
Expected: PASS, `new: 0`. ⚠️ These smoke tests SKIP by default; a bare `go test ./` proves nothing.

- [ ] **Step 4: Boot test in an isolated detached worktree**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe . && timeout 180 ./boot-check.exe > boot.log 2>&1
```
Expected: **exit 124** (the timeout firing IS success), 0 crashes, 1 `Server Ready`.
Never grep the bare word `panic` — `MapConsistencyEnforce` legitimately has the value `panic`.

- [ ] **Step 5: PR**

```bash
git push -u origin feature/u10b-1b-resolution-onto-the-core
gh pr create --repo pruuk/DOGMud --base master --head feature/u10b-1b-resolution-onto-the-core --fill
```
⚠️ Always pass `--repo pruuk/DOGMud`; `gh` defaults to the fork PARENT.

### Task D3: Adversarial playtest

The odds of crafting, salvage, search, track and forage all moved, and stealth changed deliberately. Boot-clean verifies the system, never the experience.

- [ ] **Step 1: Write a goals file** at `tools/playtest/goals/2026-08-28-u10b-1b-resolution.yaml` with `ephemeral:` set, targeting: does craft difficulty track the recipe AND the materials (spec signal 3); does a novice salvager retain roughly twice what they used to; can a hidden player now evade a searcher.

- [ ] **Step 2: Run it**

```text
/playtest local --checkout <abs> bug-finder 2026-08-28-u10b-1b-resolution.yaml
```

- [ ] **Step 3: Extract findings to memory** — playtest reports are gitignored.

---

## Self-review

**Spec coverage:** 5.1 conversions → A1–A3. 5.1.1 craft basis → B1, B2, B4. 5.1.1.1 no bespoke formula → B2 Step 3 comment. 5.1.1.2 floors not deleted clamps → B1 Step 4, B5 Step 2. 5.1.1.3 deterministic ingredients → B3. 5.1.1.4 salvage difficulty → B2, B5. 5.1.2 material tier → shipped PR #73, consumed in B3. 5.2 hidden detection → C1. Guards → every task that adds a caller.

**Known gaps, stated rather than hidden:**
- **Task C1 Step 1 has a test skeleton, not full code.** The fixture depends on the existing `internal/actions` test helpers, which the executing engineer must read first. Flagged rather than invented, because guessing the fixture shape is how a plan ships a test that cannot compile.
- **Task A2 carries a real unknown**: whether track compares one roll against two tiers. If it does, it needs two contests, not one roll compared twice. The step says so and says how to check.
- 5.3 (skullduggery ×16), 5.4 (deletions), 5.5 (mob-spell gate) are **slice 1 work, already shipped** — not in this plan.

**Ordering note:** B3 (`DearestMaterialTier`) is written before B4, but B4 Step 1 extracts `SelectIngredients`, which B3's callers need. Either land B4 Step 1 first, or accept that B3 ships with only its determinism test until B4 wires it. Prefer the former — a helper with no caller is how dead code gets born.

**Balance risk carried into the playtest, from the spec's own tables:**
- Search score 100 goes 4.8% → 11.9%; score 175 goes 97.2% → 91.1%. Weak searchers gain, experts lose certainty.
- Salvage novices retain roughly **twice** what they do today (~0.12 → ~0.25 per cycle). Masters land close to today (~0.81 → ~0.78). Watch the material sink.
- A masterwork recipe needs ~30 levels above its minimum to become routine, where a simple one needs ~9. That is the intent, not a bug.
