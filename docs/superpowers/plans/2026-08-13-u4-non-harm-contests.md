# U4 — Non-Harm Contests onto the Contest Core: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Revision 2 (2026-08-13)** — rewritten after a three-reviewer adversarial pass.
> Changes from revision 1: the site count was wrong (19, not 21); a claim about Go
> defaults was false and had been baked into two shipped comments; the equivalence
> test was blind to a 2x floor error; `RunWithGlobalFloors` moved from
> `internal/contest` to `internal/combat`; six unowned sites in `search.go` found;
> the `surprise_attack.go` breadcrumb was factually wrong. Details in each section.

**Goal:** Migrate the last 19 opposed-roll sites in the game — stealth, theft,
trap-planting, trap-defusal, shadowing, hidden detection and flee — off
`dice.OpposedRollStat*` and onto `internal/contest`, as a provable no-op; close
the guard hole that lets a new caller opt out of the contest floors through the
arc's own package; and assign the unowned contests this work uncovered.

**Architecture:** Two floor pairs are in play and they are NOT interchangeable.
The 17 non-combat sites use the **global contest pair**
(`MinContestSuccessChance`/`MinContestResistChance`), reached today through
`dice.OpposedRollStat`'s package globals. The 2 flee sites use the **maneuver
pair**. A new wrapper, `combat.RunWithGlobalFloors`, joins its two existing
siblings in `internal/combat/contest_floors.go`; flee moves onto the existing
`combat.RunWithManeuverFloors`. Afterwards `dice.OpposedRollStat` and
`dice.OpposedRollStatWithFloors` have zero production callers.

**Tech Stack:** Go, `internal/contest`, `internal/dice`, `internal/combat`, Go
AST guard tests (`go/ast`, `go/parser`).

---

## READ THIS FIRST — the trap that makes this chunk dangerous

**`_datafiles/config.yaml` ships all three floor pairs at `0.05`:**

```
MinContestSuccessChance: 0.05    MinContestResistChance:  0.05   <- the 17 sites
MinSpellHitChance:       0.05    MinSpellResistChance:    0.05
MinManeuverHitChance:    0.05    MinManeuverResistChance: 0.05   <- flee's 2 sites
```

So in production, **wiring a site to the wrong floor pair is numerically
invisible.** It passes every statistical check and every playtest, because the
numbers are identical. It becomes a live balance bug the instant anyone retunes
one pair — which is exactly what U6 exists to do.

**Under a Go test binary the three pairs are NOT equal, and the difference is
counter-intuitive.** Get this right or you will write a test that proves nothing:

| Pair | Reached via | Value in a test binary |
|---|---|---|
| global | `dice.ContestFloors()` — a package var, `internal/dice/contest_floors.go:25` | **0.05** |
| maneuver | `configs.GetBalanceConfig().MinManeuverHitChance` | **0** |
| spell | `configs.GetBalanceConfig().MinSpellHitChance` | **0** |

A Go test binary never loads `config.yaml`. The `0.05` in
`internal/configs/config.balance.misc.go:148-168` is a **validation fallback**,
not a struct default, and its condition is `if x < 0 || x > 0.50`. The zero value
is neither, so **0 passes validation untouched**. The repo already documents this
at `internal/actions/contest_sign_taunt_test.go:69-71`.

> Revision 1 of this plan asserted "all three Go defaults are 0.05 too" and baked
> that into two shipped source comments. It is false. This is the exact failure
> CLAUDE.md warns about: never quote a Go default as a live value.

| File | Floor pair it must keep using | Wrapper |
|---|---|---|
| `actions/sneak.go`, `shadow.go`, `steal.go`, `plant.go`, `defuse.go` | global | `combat.RunWithGlobalFloors` |
| `usercommands/go.go`, `skill.skullduggery.shadow.go` | global | `combat.RunWithGlobalFloors` |
| `combat/flee.go` | **maneuver** | `RunWithManeuverFloors` (same package) |

---

## Ground truth, verified 2026-08-13 against master `68ba9ad96`

**Correction to the arc memory:** the 17 non-combat sites were recorded as
"unfloored `dice.OpposedRollStat` sites". **They are floored.** Chunk 5.10 gave
the floored roll the natural name, so `dice.OpposedRollStat` reads the package
globals set from config by `main.go:197`. Migrating them to `contest.Run` or
`contest.AgainstDifficulty` would **silently delete their floors** — a real
behaviour change wearing a no-op's clothes.

**Equivalence is exact for the boolean — and ONLY for the boolean.**

```
dice.OpposedRollStat(atk, def)
  -> OpposedRollStatWithFloors(atk, def, minContestSuccess, minContestResist)
  -> OpposedRollStatRaw(atk, def)  ->  OpposedRoll(atk, def, StdDevFor(atk))
       attackRoll  := Roll(atk, stdDev)     // 1st RNG draw
       defenseRoll := Roll(def, stdDev)     // 2nd RNG draw
       success     := attackRoll.Value > defenseRoll.Value

contest.RunWithFloors(atk, []Entry{{Score: def}}, floorS, floorR)
  -> Run(atk, entries)
       stdDev      := dice.StdDevFor(atk)
       attackRoll  := dice.Roll(atk, stdDev)      // 1st RNG draw
       defenseRoll := dice.Roll(def, stdDev)      // 2nd RNG draw
       margin      := attackRoll.Value - defenseRoll.Value
       Success      = Contested && margin > 0
```

Same stdDev derivation, same roll order, same number of RNG draws (2 normal, plus
at most 1 uniform in the floor switch — the `switch` short-circuits, so never 2),
same tie-breaking (`>` on both sides, so an exact tie fails in both).

**The divergence, stated plainly because it will bite someone later.**
`dice.OpposedRollStatWithFloors:109-114` rewrites `attackRoll.Success/.Margin`
and `defenseRoll.Success/.Margin` when a floor flips the outcome, and
`dice.OpposedRoll:112-115` populates them on every call. `contest.Run` uses bare
`dice.Roll`, so **both `RollResult`s carry `.Success = false` and `.Margin = 0`
always**, and `contest.RunWithFloors` never touches them either. This is the trap
already recorded in `internal/combat/context.md` and in the arc memory — it
nearly killed spell-deflection crits in U2.

It is harmless here **only because all 19 sites discard both `RollResult`s and
read nothing but the boolean.** That is the load-bearing fact. Do not reuse this
equivalence proof for any site that reads a margin or a roll.

**All 8 files will orphan their `dice` import.** Each file's `dice.` reference
count exactly equals its migrated-site count, so after each task the `dice`
import must be deleted. The compiler enforces this.

### The 19 sites — 17 global pair, 2 maneuver pair

| # | File | Line | Call | Pair |
|---|---|---|---|---|
| 1–2 | `internal/actions/sneak.go` | 103, 128 | `(sneakScore, observerScore)` | global |
| 3 | `internal/actions/shadow.go` | 175 | `(searchScore, sneakScore)` | global |
| 4–7 | `internal/actions/steal.go` | 172, 350, 404, 509 | mixed | global |
| 8–11 | `internal/actions/plant.go` | 157, 276, 325, 414 | mixed | global |
| 12 | `internal/actions/defuse.go` | 145 | `(defuseScore, trapDifficulty)` | global |
| 13–16 | `internal/usercommands/go.go` | 479, 498, 540, 560 | mixed | global |
| 17 | `internal/usercommands/skill.skullduggery.shadow.go` | 139 | `(targetScore, sneakScore)` | global |
| 18–19 | `internal/combat/flee.go` | 85, 108 | `OpposedRollStatWithFloors` | **maneuver** |

Line numbers are as of `68ba9ad96` and will drift. Locate by the surrounding code
shown in each task, not by line number.

---

## Scope decisions made in this plan

### 1. `surprise_attack.go` is NOT migrated — reassigned to U9

The roadmap assigns `actions/surprise_attack.go:222-225` to U4. It is not
migrated here, and this plan reassigns it to **U9**.

**Its behaviour is not what the roadmap says.** The roadmap calls it "hand-rolled
per-weapon hit resolution". It is not hit resolution at all: the primary weapon
is appended with `hitPenalty: 0.0` (`surprise_attack.go:112-117`), so
`penaltyPct == 0` and `util.Rand(100) < 0` is never true. **Every primary
surprise swing is an unconditional auto-hit with no roll whatsoever.** The
`util.Rand` fires only for offhand and extra-arm swings. It is a self-imposed
multi-weapon penalty, not a contest missing a defender.

**Why not U4:** giving it a defender is a behaviour change, and standing rule 3
contracts U1–U5 as provable no-ops. U3 hit the identical conflict with the
`Position_GrappleTick` z-normalisation, left the code alone, and dropped a
`NOTE(U6)`. Task 11 does the same.

**Why U9, not U6:** U6 is "THE FLIP" — uniform x5, multiplier defence,
margin-scaled mitigation. Every one of those verbs operates on contests already
on the core. U9 is the arc's designated non-contest-to-contest chunk
("concentration becomes a proper contest; knockdown and prone recovery become
opposed rolls") and already carries a modelling table for exactly this kind of
conversion. U6 is also the one chunk marked L with "Yes — all of it" under
behaviour change; loading an unmodelled new contest into it concentrates risk in
the commit the arc most needs to stay revertable.

### 2. `search.go`'s six contests are unowned — assigned, not migrated

**This is the missed-site finding.** U4's goal includes "hidden detection", and
the game implements hidden detection **twice, incompatibly**:

- `usercommands/go.go:540, 560` — `OpposedRollStat(observerScore, hiddenScore)`.
  An opposed contest. **U4 migrates these.**
- `actions/search.go:151, 179` — `dice.RollStat(searchScore); if roll.Value >= 135.0`.
  A flat threshold that **never reads the hider's sneak score at all**.

A hider's skill decides the outcome in one path and is literally ignored in the
other. This is not player-only: `internal/behaviortree/conditions_scout.go:12`
gates a `try_search` btree action, so mobs run `actions.Search` too.

`search.go` has six such sites (lines 81, 100, 119, 151, 179, 213), and
`track.go:121` plus `forager/forage_core.go:126` are the same family.
`contest.AgainstDifficulty`'s own docstring names these by name — U1 built the
helper for them and no chunk ever claimed them. `contest.AgainstDifficulty`
consequently has **zero production callers today**.

**U4 does not migrate them** — converting a flat threshold into a contest is a
behaviour change. Task 11 assigns them explicitly in the roadmap and leaves a
`NOTE` at each site, so they cannot be missed again. Silence is what let
`resolveCharmSpell` slip through U2.

### 3. `dice.OpposedRollStat*` is deprecated, not deleted

Standing rule 4 says delete as you migrate. Deleting these two in U4 is not
possible: **Task 1's equivalence test calls `dice.OpposedRollStat` as its
oracle.** You cannot delete the thing you are measuring against in the chunk that
measures. `internal/dice/contest_floors_test.go` also calls both across 5 sites,
and `OpposedRollStatRaw`/`OpposedRoll` must survive regardless —
`internal/combat/regression_test.go`, `margin_crit_contest_test.go` and
`integration_combat_test.go` call `Raw` directly.

U4 marks both `Deprecated:` and adds them to the floor guard with an
`internal/dice` exemption, so a *new* production caller fails CI. Deletion goes
on U6's checklist, where `RunWithFloors` and the wrapper family are already
slated to be reshaped.

---

## File Structure

**Created:**
- `internal/combat/global_floors_test.go` — equivalence + floor-tracking tests.
- `floor_pair_guard_test.go` (repo root, `package main`) — static guard on floor
  pair *and* attacker direction per site.

**Modified:**
- `internal/combat/contest_floors.go` — add `RunWithGlobalFloors`
- `internal/actions/sneak.go`, `shadow.go`, `steal.go`, `plant.go`, `defuse.go`
- `internal/usercommands/go.go`, `skill.skullduggery.shadow.go`
- `internal/combat/flee.go`
- `contest_floor_guard_test.go` (repo root)
- `internal/dice/contest_floors.go` — `Deprecated:` markers
- `internal/actions/surprise_attack.go`, `search.go`, `track.go`,
  `internal/forager/forage_core.go` — `NOTE` breadcrumbs only
- `internal/actions/shadow_test.go` — one stale comment
- `CLAUDE.md`
- `internal/combat/context.md`, `internal/contest/context.md`,
  `internal/actions/context.md`, `internal/usercommands/context.md`,
  `internal/util/context.md`
- `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, `docs/PATCH_NOTES.md`

---

### Task 0: Branch

- [ ] **Step 1: Create the feature branch**

The repo is currently on `master`. CLAUDE.md forbids committing there directly,
and the Ship-it section assumes this branch exists.

```bash
git checkout -b feature/u4-non-harm-contests
git status
```

Expected: `On branch feature/u4-non-harm-contests`.

---

### Task 1: `combat.RunWithGlobalFloors`

**Why `internal/combat` and not `internal/contest`.** Its two siblings,
`RunWithManeuverFloors` and `RunWithSpellFloors`, live in
`internal/combat/contest_floors.go`, and that file's docstring is the canonical
statement of the three-pair cost-of-failure principle — including the
out-of-combat row this wrapper implements. `internal/contest/context.md` carries
the documented invariant "No config reads, deliberately", and `contest.go:186`
says the package "takes its tunables as parameters by design"; a state-reading
wrapper there would contradict both. U6 reconciles the two floor styles inside
`internal/combat`, so all four mechanisms should already be there.

**Why it still reads `dice.ContestFloors()` and not config directly.**
`internal/behaviortree/actions_skullduggery_test.go:104` pins the floors with
`dice.SetContestFloors(0, 0)`. Reading `configs.GetBalanceConfig()` instead would
silently disconnect that pre-existing test's pin — and would also change the
test-binary value from 0.05 to 0 (see READ THIS FIRST). Both are rule-3
violations. Deleting the `dice` globals belongs to U6.

**Files:**
- Modify: `internal/combat/contest_floors.go`
- Test: `internal/combat/global_floors_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/combat/global_floors_test.go`:

```go
package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dice"
)

// TestRunWithGlobalFloorsTracksDiceGlobals is the discriminating test for the
// trap in the U4 plan: config.yaml ships all three floor pairs at 0.05, so a
// wrapper wired to the WRONG pair is invisible in production. This moves the
// global pair to values nothing else uses and asserts the wrapper follows it.
func TestRunWithGlobalFloorsTracksDiceGlobals(t *testing.T) {
	origS, origR := dice.ContestFloors()
	t.Cleanup(func() { dice.SetContestFloors(origS, origR) })

	// A hopeless attacker: 10 against 1000 never wins on the roll, so the floor
	// is the entire signal.
	const hopeless, overwhelming = 10.0, 1000.0

	dice.SetContestFloors(0.5, 0)
	wins := 0
	for i := 0; i < 4000; i++ {
		if RunWithGlobalFloors(hopeless, overwhelming).Success {
			wins++
		}
	}
	if wins < 1600 || wins > 2400 {
		t.Errorf("floorSuccess=0.5 on a hopeless attack: got %d/4000 wins, want ~2000", wins)
	}

	dice.SetContestFloors(0, 0)
	wins = 0
	for i := 0; i < 4000; i++ {
		if RunWithGlobalFloors(hopeless, overwhelming).Success {
			wins++
		}
	}
	if wins > 40 {
		t.Errorf("floorSuccess=0 on a hopeless attack: got %d/4000 wins, want ~0", wins)
	}
}

// TestRunWithGlobalFloorsMatchesDiceOpposedRollStat is the no-op proof: the
// wrapper must be indistinguishable from the function the 17 sites call today.
//
// TOLERANCE. Revision 1 of this plan used a flat 3% and was blind to a floor
// that was wrong by 2x -- a halved pair moved every case by at most 2.5%. It
// also included a parity case (100 vs 100) with ZERO discriminating power: for
// any symmetric pair, p*(1-f) + (1-p)*f evaluates to exactly 0.5 at p=0.5, so
// that case cannot fail for any floor error at all.
//
// This version drops parity and uses a per-case 4-sigma bound on the difference
// of two proportions, which never flakes and does catch a halved floor.
func TestRunWithGlobalFloorsMatchesDiceOpposedRollStat(t *testing.T) {
	origS, origR := dice.ContestFloors()
	t.Cleanup(func() { dice.SetContestFloors(origS, origR) })
	dice.SetContestFloors(0.05, 0.05)

	cases := []struct {
		name     string
		atk, def float64
	}{
		{"outmatched", 100, 150}, // floor-success dominated
		{"favoured", 150, 100},   // floor-resist dominated
		{"rout", 100, 30},        // floor-resist is the whole signal
	}

	const n = 20000
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			oldWins := 0
			for i := 0; i < n; i++ {
				if ok, _, _, _ := dice.OpposedRollStat(c.atk, c.def); ok {
					oldWins++
				}
			}
			newWins := 0
			for i := 0; i < n; i++ {
				if RunWithGlobalFloors(c.atk, c.def).Success {
					newWins++
				}
			}

			p1 := float64(oldWins) / n
			p2 := float64(newWins) / n
			// Standard error of the difference of two independent proportions.
			se := math.Sqrt(p1*(1-p1)/n + p2*(1-p2)/n)
			if se == 0 {
				se = 1.0 / n // both arms saturated; any difference is real
			}
			z := math.Abs(p1-p2) / se
			if z > 4.0 {
				t.Errorf("atk=%.0f def=%.0f: dice=%.4f, combat=%.4f (z=%.1f) — not equivalent",
					c.atk, c.def, p1, p2, z)
			}
		})
	}
}

// TestRunWithGlobalFloorsDetectsAHalvedFloor proves the equivalence test above
// has real power. It deliberately compares the correct floor against a halved
// one and asserts the 4-sigma bound DOES fire, so a future tolerance loosening
// cannot quietly make the equivalence test vacuous.
func TestRunWithGlobalFloorsDetectsAHalvedFloor(t *testing.T) {
	origS, origR := dice.ContestFloors()
	t.Cleanup(func() { dice.SetContestFloors(origS, origR) })

	const n = 20000
	const atk, def = 100.0, 150.0 // outmatched: floor-success dominated

	dice.SetContestFloors(0.05, 0.05)
	correct := 0
	for i := 0; i < n; i++ {
		if RunWithGlobalFloors(atk, def).Success {
			correct++
		}
	}

	dice.SetContestFloors(0.025, 0.025)
	halved := 0
	for i := 0; i < n; i++ {
		if RunWithGlobalFloors(atk, def).Success {
			halved++
		}
	}

	p1, p2 := float64(correct)/n, float64(halved)/n
	se := math.Sqrt(p1*(1-p1)/n + p2*(1-p2)/n)
	z := math.Abs(p1-p2) / se
	if z <= 4.0 {
		t.Errorf("a halved floor pair must be detectable: correct=%.4f halved=%.4f z=%.1f, want z>4",
			p1, p2, z)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
go test ./internal/combat/ -run 'TestRunWithGlobalFloors' -v
```

Expected: FAIL — `undefined: RunWithGlobalFloors`.

- [ ] **Step 3: Add the wrapper**

Append to `internal/combat/contest_floors.go`:

```go
// ContestFloors is the GLOBAL contest floor pair, for uncertain outcomes that
// are not maneuvers and not spells: sneaking, stealing, planting, defusing,
// shadowing, noticing someone hidden.
//
// It reads dice.ContestFloors() rather than configs.GetBalanceConfig() -- unlike
// ManeuverFloors and SpellFloors, which read config directly. That asymmetry is
// deliberate and load-bearing:
//
//   - main.go seeds the dice globals from Balance.MinContestSuccessChance and
//     Balance.MinContestResistChance, so in production the two routes are the
//     same two config keys and the same values.
//   - internal/behaviortree/actions_skullduggery_test.go pins the floors with
//     dice.SetContestFloors. Reading config here would disconnect that pin.
//   - A Go test binary never loads config.yaml. The global pair therefore
//     measures 0.05 in a test (a real package var), while the maneuver and spell
//     pairs measure 0 -- the 0.05 in config.balance.misc.go is a validation
//     fallback whose condition (< 0 || > 0.50) lets the zero value through.
//     Routing this pair through config would silently change it to 0 under test.
//
// U6 owns collapsing the two routes; do not "tidy" this into a config read.
func ContestFloors() (hit, resist float64) {
	return dice.ContestFloors()
}

// RunWithGlobalFloors contests attackScore against a single defenseScore using
// the GLOBAL contest floor pair.
//
// It is the exact mirror of dice.OpposedRollStat, and exists so the 17
// out-of-combat contests migrated in U4 keep reading the pair they have always
// read.
//
// WHY THIS IS NOT RunWithManeuverFloors OR RunWithSpellFloors. config.yaml ships
// all three pairs at 0.05, so calling the wrong one is invisible in production:
// it passes every test and every playtest, and becomes a live balance bug the
// moment U6 retunes one pair. The pairs say different things about the COST OF A
// SINGLE FAILURE -- a maneuver burns the whole round, an out-of-combat attempt is
// one shot plus a consequence -- and that distinction is what is being preserved,
// not the number. floor_pair_guard_test.go at the repo root is the guard.
//
// The single entry is deliberately unnamed, so the returned Result.Winner is
// always "". Read Result.Contested, never Result.Winner, to ask whether a
// contest happened.
//
// TRANSITIONAL, like its two siblings: U6 may delete or reshape all three.
func RunWithGlobalFloors(attackScore, defenseScore float64) contest.Result {
	hit, resist := ContestFloors()
	return contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, hit, resist)
}
```

- [ ] **Step 4: Add the `dice` import**

`internal/combat/contest_floors.go` currently imports only `configs` and
`contest`. Add:

```go
	"github.com/GoMudEngine/GoMud/internal/dice"
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
go test ./internal/combat/ -run 'TestRunWithGlobalFloors' -v
gofmt -l internal/
```

Expected: all three tests PASS; gofmt silent.

- [ ] **Step 6: Commit**

```bash
git add internal/combat/contest_floors.go internal/combat/global_floors_test.go
git commit -m "feat(combat): add RunWithGlobalFloors, the global contest floor pair (U4)"
```

---

### Task 2: Migrate `actions/sneak.go` (2 sites)

This task establishes the pattern every later migration repeats. Note the call is
**two scalar arguments**, matching `RunWithManeuverFloors` — no `contest.Entry`
slice at the call site.

**Files:**
- Modify: `internal/actions/sneak.go`

- [ ] **Step 1: Replace the player-observer roll**

Find (around line 100):

```go
		sneakScore := CalcSneakScoreVsObserver(char, observer.Character, room)
		observerScore := CalcSearchScore(observer.Character)
		rollHappened = true
		success, _, _, _ := dice.OpposedRollStat(sneakScore, observerScore)
		if !success {
```

Replace with:

```go
		sneakScore := CalcSneakScoreVsObserver(char, observer.Character, room)
		observerScore := CalcSearchScore(observer.Character)
		rollHappened = true
		success := combat.RunWithGlobalFloors(sneakScore, observerScore).Success
		if !success {
```

- [ ] **Step 2: Replace the mob-observer roll**

Find (around line 125):

```go
		sneakScore := CalcSneakScoreVsObserver(char, &m.Character, room)
		observerScore := CalcSearchScore(&m.Character)
		rollHappened = true
		success, _, _, _ := dice.OpposedRollStat(sneakScore, observerScore)
		if !success {
```

Replace with:

```go
		sneakScore := CalcSneakScoreVsObserver(char, &m.Character, room)
		observerScore := CalcSearchScore(&m.Character)
		rollHappened = true
		success := combat.RunWithGlobalFloors(sneakScore, observerScore).Success
		if !success {
```

- [ ] **Step 3: Swap the import**

Delete the `dice` import line. Add, in correct alphabetical position within the
`github.com/GoMudEngine/GoMud/internal/...` group:

```go
	"github.com/GoMudEngine/GoMud/internal/combat"
```

`gofmt` sorts import blocks, so placement matters. In this file `combat` sorts
before `dice`'s old slot. **Do not assume that holds for other files** — Tasks 4,
5 and 7 name the correct neighbour explicitly.

- [ ] **Step 4: Verify no `dice` reference survives**

```bash
grep -n "dice\." internal/actions/sneak.go
```

Expected: no output.

- [ ] **Step 5: Build and test**

```bash
go build ./... && go test ./internal/actions/
gofmt -l internal/actions/
```

Expected: build succeeds, tests pass, gofmt silent.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/sneak.go
git commit -m "refactor(actions): sneak uses the contest core (U4)"
```

---

### Task 3: Migrate the shadow pair (2 sites, 2 files)

**Files:**
- Modify: `internal/actions/shadow.go`
- Modify: `internal/usercommands/skill.skullduggery.shadow.go`

- [ ] **Step 1: Replace the roll in `actions/shadow.go`**

The comment above it names the removed function and points at the wrong file
(`shadowDetectionRoll` lives in `skill.skullduggery.shadow.go`, not `go.go`).
Both are corrected. Find (around line 170):

```go
	// Initial detection roll: target wins if their search score beats the
	// actor's sneak score. Uses the same formula as shadowDetectionRoll in
	// go.go (Per+Search vs Dex+Skullduggery; OpposedRollStat: first arg wins).
	sneakScore := CalcSneakScoreVsObserver(char, targetUser.Character, actor.GetRoom())
	searchScore := CalcSearchScore(targetUser.Character)
	detected, _, _, _ := dice.OpposedRollStat(searchScore, sneakScore)
```

Replace with:

```go
	// Initial detection roll: the TARGET is the attacker here -- they are the
	// one trying to notice -- so the shadowing actor's sneak score is the
	// defending entry. Same formula as shadowDetectionRoll in
	// usercommands/skill.skullduggery.shadow.go (Per+Search vs Dex+Skullduggery).
	sneakScore := CalcSneakScoreVsObserver(char, targetUser.Character, actor.GetRoom())
	searchScore := CalcSearchScore(targetUser.Character)
	detected := combat.RunWithGlobalFloors(searchScore, sneakScore).Success
```

- [ ] **Step 2: Replace the roll in `usercommands/skill.skullduggery.shadow.go`**

**The comment here is TWO lines, not one.** Find (lines 137-139):

```go
	// OpposedRollStat(atk, def) returns true when atk (first arg) wins.
	// Target detects when targetScore beats sneakScore.
	detected, _, _, _ := dice.OpposedRollStat(targetScore, sneakScore)
```

Replace with:

```go
	// The target is the attacker in this contest: Success means they noticed.
	// Target detects when targetScore beats sneakScore.
	detected := combat.RunWithGlobalFloors(targetScore, sneakScore).Success
```

- [ ] **Step 3: Swap the imports in both files**

Both files currently have `configs, dice` in their internal group, so `combat`
replaces `dice` in place and the block stays sorted. Delete `dice`, add:

```go
	"github.com/GoMudEngine/GoMud/internal/combat"
```

- [ ] **Step 4: Verify**

```bash
grep -n "dice\.\|OpposedRollStat" internal/actions/shadow.go internal/usercommands/skill.skullduggery.shadow.go
```

Expected: no output. This also confirms the stale comment naming
`OpposedRollStat` is gone, per Definition of Done #7.

- [ ] **Step 5: Build and test**

```bash
go build ./... && go test ./internal/actions/ ./internal/usercommands/
gofmt -l internal/
```

Expected: build succeeds, tests pass, gofmt silent.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/shadow.go internal/usercommands/skill.skullduggery.shadow.go
git commit -m "refactor(shadow): both shadow detection rolls use the contest core (U4)"
```

---

### Task 4: Migrate `actions/steal.go` (4 sites)

**Files:**
- Modify: `internal/actions/steal.go`

- [ ] **Step 1: Replace the mob-victim steal roll (around line 171)**

Find:

```go
	defenderScore := float64(m.Character.Stats.Perception.ValueAdj)
	success, _, _, _ := dice.OpposedRollStat(attackerScore, defenderScore)
```

Replace with:

```go
	defenderScore := float64(m.Character.Stats.Perception.ValueAdj)
	success := combat.RunWithGlobalFloors(attackerScore, defenderScore).Success
```

- [ ] **Step 2: Replace the player-victim steal roll (around line 349)**

Find:

```go
	defenderScore := float64(targetUser.Character.Stats.Perception.ValueAdj)
	success, _, _, _ := dice.OpposedRollStat(attackerScore, defenderScore)
```

Replace with:

```go
	defenderScore := float64(targetUser.Character.Stats.Perception.ValueAdj)
	success := combat.RunWithGlobalFloors(attackerScore, defenderScore).Success
```

- [ ] **Step 3: Replace the independent detection roll (around line 404)**

Find:

```go
		detected, _, _, _ := dice.OpposedRollStat(searchScore, sneakScore)
```

Replace with:

```go
		detected := combat.RunWithGlobalFloors(searchScore, sneakScore).Success
```

- [ ] **Step 4: Replace the observer roll (around line 509)**

This one **assigns to an existing variable with `=`, not `:=`**. Using `:=` here
shadows `success` inside the `if` block and silently changes control flow.

Find:

```go
	success := true
	if hasObserver {
		success, _, _, _ = dice.OpposedRollStat(attackerScore, highestPerception)
	}
```

Replace with:

```go
	success := true
	if hasObserver {
		success = combat.RunWithGlobalFloors(attackerScore, highestPerception).Success
	}
```

- [ ] **Step 5: Swap the import**

This file's internal group is `... configs, crimes, dice, ...`. `combat` sorts
**before `crimes`**, not into `dice`'s slot. Delete `dice`, insert `combat` after
`configs`.

- [ ] **Step 6: Verify and build**

```bash
grep -n "dice\." internal/actions/steal.go
go build ./... && go test ./internal/actions/
gofmt -l internal/actions/
```

Expected: grep silent, build succeeds, tests pass, gofmt silent.

- [ ] **Step 7: Commit**

```bash
git add internal/actions/steal.go
git commit -m "refactor(actions): all four steal rolls use the contest core (U4)"
```

---

### Task 5: Migrate `actions/plant.go` (4 sites)

`plant.go` mirrors `steal.go` structurally. Do not assume the surrounding code is
identical — verify each Find block.

**Files:**
- Modify: `internal/actions/plant.go`

- [ ] **Step 1: Replace the mob-target plant roll (around line 156)**

Find:

```go
	defenderScore := float64(m.Character.Stats.Perception.ValueAdj)
	success, _, _, _ := dice.OpposedRollStat(attackerScore, defenderScore)
```

Replace with:

```go
	defenderScore := float64(m.Character.Stats.Perception.ValueAdj)
	success := combat.RunWithGlobalFloors(attackerScore, defenderScore).Success
```

- [ ] **Step 2: Replace the player-target plant roll (around line 275)**

Find:

```go
	defenderScore := float64(targetUser.Character.Stats.Perception.ValueAdj)
	success, _, _, _ := dice.OpposedRollStat(attackerScore, defenderScore)
```

Replace with:

```go
	defenderScore := float64(targetUser.Character.Stats.Perception.ValueAdj)
	success := combat.RunWithGlobalFloors(attackerScore, defenderScore).Success
```

- [ ] **Step 3: Replace the independent detection roll (around line 325)**

Find:

```go
		detected, _, _, _ := dice.OpposedRollStat(searchScore, sneakScore)
```

Replace with:

```go
		detected := combat.RunWithGlobalFloors(searchScore, sneakScore).Success
```

- [ ] **Step 4: Replace the observer roll (around line 414)**

Assigns with `=`, not `:=`. Find:

```go
	success := true
	if hasObserver {
		success, _, _, _ = dice.OpposedRollStat(attackerScore, highestPerception)
	}
```

Replace with:

```go
	success := true
	if hasObserver {
		success = combat.RunWithGlobalFloors(attackerScore, highestPerception).Success
	}
```

- [ ] **Step 5: Swap the import**

Same as `steal.go`: internal group is `... configs, crimes, dice, ...`. `combat`
goes **before `crimes`**.

- [ ] **Step 6: Verify and build**

```bash
grep -n "dice\." internal/actions/plant.go
go build ./... && go test ./internal/actions/
gofmt -l internal/actions/
```

Expected: grep silent, build succeeds, tests pass, gofmt silent.

- [ ] **Step 7: Commit**

```bash
git add internal/actions/plant.go
git commit -m "refactor(actions): all four plant rolls use the contest core (U4)"
```

---

### Task 6: Migrate `actions/defuse.go` (1 site, roll vs static difficulty)

**Do NOT use `contest.AgainstDifficulty` here.** It delegates straight to `Run`
and is **unfloored**; this site is floored today. Using it would silently delete
the floor.

**Files:**
- Modify: `internal/actions/defuse.go`

- [ ] **Step 1: Replace the roll (around line 138)**

Find:

```go
	// ── Opposed roll: (Per + skillLevel*25 + kitBonus) vs (difficulty*10) ──

	defuseScore := float64(char.Stats.Perception.ValueAdj) +
		float64(skillLevel)*25.0 +
		float64(kitBonus)
	trapDifficulty := float64(tgt.lockDifficulty) * 10.0

	success, _, _, _ := dice.OpposedRollStat(defuseScore, trapDifficulty)
```

Replace with:

```go
	// ── Contest: (Per + skillLevel*25 + kitBonus) vs (difficulty*10) ──
	//
	// The trap is a static difficulty. Deliberately NOT
	// contest.AgainstDifficulty: that helper is unfloored, and this contest has
	// been floored by the global pair since chunk 5.10.
	defuseScore := float64(char.Stats.Perception.ValueAdj) +
		float64(skillLevel)*25.0 +
		float64(kitBonus)
	trapDifficulty := float64(tgt.lockDifficulty) * 10.0

	success := combat.RunWithGlobalFloors(defuseScore, trapDifficulty).Success
```

- [ ] **Step 2: Swap the import**

Delete `dice`, add `"github.com/GoMudEngine/GoMud/internal/combat"` (sorts first
in this file's internal group).

- [ ] **Step 3: Verify and build**

```bash
grep -n "dice\." internal/actions/defuse.go
go build ./... && go test ./internal/actions/
gofmt -l internal/actions/
```

Expected: grep silent, build succeeds, tests pass, gofmt silent.

- [ ] **Step 4: Commit**

```bash
git add internal/actions/defuse.go
git commit -m "refactor(actions): defuse uses the contest core, keeping its floor (U4)"
```

---

### Task 7: Migrate `usercommands/go.go` (4 sites)

Two pairs with **opposite attacker roles**. Sites 1–2 roll the mover's sneak as
the attack; sites 3–4 roll the mover's search as the attack against
already-hidden occupants. Swapping them compiles and inverts stealth.

**Files:**
- Modify: `internal/usercommands/go.go`

- [ ] **Step 1: Replace the sneaking-past-players roll (around line 477)**

Find:

```go
					sneakScore := actions.CalcSneakScoreVsObserver(user.Character, p.Character, destRoom)
					observerScore := actions.CalcSearchScore(p.Character)
					success, _, _, _ := dice.OpposedRollStat(sneakScore, observerScore)
					if !success {
```

Replace with:

```go
					sneakScore := actions.CalcSneakScoreVsObserver(user.Character, p.Character, destRoom)
					observerScore := actions.CalcSearchScore(p.Character)
					success := combat.RunWithGlobalFloors(sneakScore, observerScore).Success
					if !success {
```

- [ ] **Step 2: Replace the sneaking-past-mobs roll (around line 496)**

Find:

```go
						sneakScore := actions.CalcSneakScoreVsObserver(user.Character, &mob.Character, destRoom)
						observerScore := actions.CalcSearchScore(&mob.Character)
						success, _, _, _ := dice.OpposedRollStat(sneakScore, observerScore)
						if !success {
```

Replace with:

```go
						sneakScore := actions.CalcSneakScoreVsObserver(user.Character, &mob.Character, destRoom)
						observerScore := actions.CalcSearchScore(&mob.Character)
						success := combat.RunWithGlobalFloors(sneakScore, observerScore).Success
						if !success {
```

- [ ] **Step 3: Replace the detecting-hidden-players roll (around line 539)**

Attacker is `observerScore` here, not the sneak score. Find:

```go
					hiddenScore := actions.CalcSneakScoreVsObserver(hiddenP.Character, user.Character, destRoom)
					success, _, _, _ := dice.OpposedRollStat(observerScore, hiddenScore)
					if success {
```

Replace with:

```go
					hiddenScore := actions.CalcSneakScoreVsObserver(hiddenP.Character, user.Character, destRoom)
					success := combat.RunWithGlobalFloors(observerScore, hiddenScore).Success
					if success {
```

- [ ] **Step 4: Replace the detecting-hidden-mobs roll (around line 559)**

Find:

```go
					hiddenScore := actions.CalcSneakScoreVsObserver(&mob.Character, user.Character, destRoom)
					success, _, _, _ := dice.OpposedRollStat(observerScore, hiddenScore)
					if success {
```

Replace with:

```go
					hiddenScore := actions.CalcSneakScoreVsObserver(&mob.Character, user.Character, destRoom)
					success := combat.RunWithGlobalFloors(observerScore, hiddenScore).Success
					if success {
```

- [ ] **Step 5: Swap the import**

This file's internal group is `... configs, conversationadapter, ... dice`.
`combat` sorts **immediately after `configs`**, not into `dice`'s slot. Delete
`dice`, insert `combat` after `configs`.

- [ ] **Step 6: Verify attacker direction, then build**

```bash
grep -n "dice\." internal/usercommands/go.go
grep -n "RunWithGlobalFloors" internal/usercommands/go.go
```

Expected: the first grep silent. The second must show **exactly four** calls —
**two** whose first argument is `sneakScore` and **two** whose first argument is
`observerScore`. If all four read the same way, a direction was inverted. Task 10
makes this a permanent test rather than a one-time eyeball.

```bash
go build ./... && go test ./internal/usercommands/
gofmt -l internal/usercommands/
```

Expected: build succeeds, tests pass, gofmt silent.

- [ ] **Step 7: Commit**

```bash
git add internal/usercommands/go.go
git commit -m "refactor(go): the four movement stealth rolls use the contest core (U4)"
```

---

### Task 8: Migrate `combat/flee.go` (2 sites, MANEUVER pair)

**This is the one file in the chunk that must NOT use `RunWithGlobalFloors`.**
Flee reads `ManeuverFloors()` today and must keep reading it. Both wrappers are
now in this same package, which makes the wrong choice a one-word slip — Task 10
guards it.

**Files:**
- Modify: `internal/combat/flee.go`

- [ ] **Step 1: Replace the mob-blocker roll (around line 82)**

Find:

```go
		blockScore := float64(m.Character.GetEffectiveDexterity() +
			m.Character.GetSkillLevel(skills.UnarmedCombat)*25)
		floorHit, floorResist := ManeuverFloors()
		success, _, _, _ := dice.OpposedRollStatWithFloors(fleeScore, blockScore, floorHit, floorResist)
		if !success {
```

Replace with:

```go
		blockScore := float64(m.Character.GetEffectiveDexterity() +
			m.Character.GetSkillLevel(skills.UnarmedCombat)*25)
		success := RunWithManeuverFloors(fleeScore, blockScore).Success
		if !success {
```

- [ ] **Step 2: Replace the player-blocker roll (around line 105)**

Find:

```go
		blockScore := float64(u.Character.GetEffectiveDexterity() +
			u.Character.GetSkillLevel(skills.UnarmedCombat)*25)
		floorHit, floorResist := ManeuverFloors()
		success, _, _, _ := dice.OpposedRollStatWithFloors(fleeScore, blockScore, floorHit, floorResist)
		if !success {
```

Replace with:

```go
		blockScore := float64(u.Character.GetEffectiveDexterity() +
			u.Character.GetSkillLevel(skills.UnarmedCombat)*25)
		success := RunWithManeuverFloors(fleeScore, blockScore).Success
		if !success {
```

- [ ] **Step 3: Delete the `dice` import**

`RunWithManeuverFloors` is same-package, so nothing is added.

- [ ] **Step 4: Verify the migration is complete**

```bash
grep -n "dice\." internal/combat/flee.go
```

Expected: no output.

```bash
grep -rn "dice\.OpposedRoll[A-Za-z]*(" internal/ modules/ --include=*.go | grep -v _test | grep -v "^internal/dice/"
```

Expected: **no output.** Every production opposed roll now goes through
`internal/contest`.

> The trailing `(` is required. A bare `grep "dice\.OpposedRoll"` matches
> narrative comments in `internal/characters/cast_helpers.go`,
> `internal/combat/avoidance.go` and `internal/contest/contest.go` that are
> historically accurate and must NOT be deleted. Revision 1 of this plan told the
> engineer to expect silence from the bare grep, which would have led to either a
> false "incomplete" verdict or the deletion of load-bearing comments.

```bash
grep -rn "dice\.OpposedRollStatWithFloors(" internal/ modules/ --include=*.go | grep -v _test
```

Expected: no output. (`internal/dice` defines and internally delegates to this
function; the qualified `dice.` prefix plus `(` excludes those.)

- [ ] **Step 5: Build and test**

```bash
go build ./... && go test ./internal/combat/
gofmt -l internal/combat/
```

Expected: build succeeds, tests pass, gofmt silent.

- [ ] **Step 6: Commit**

```bash
git add internal/combat/flee.go
git commit -m "refactor(combat): both flee blocks use the contest core on the maneuver pair (U4)"
```

---

### Task 9: Extend the floor guard to `contest.*` and the deprecated `dice` pair

The guard walks the AST for `dice.OpposedRollStatRaw` and `dice.OpposedRoll`
only, with the callee package hardcoded to `"dice"`. `contest.Run` and
`contest.AgainstDifficulty` are exported and unfloored, so a new caller can opt
out of the floors through the arc's own package and the guard will not notice.

**Files:**
- Modify: `contest_floor_guard_test.go` (repo root, `package main`)
- Modify: `internal/dice/contest_floors.go` (Deprecated markers)

- [ ] **Step 1: Add the `Deprecated:` markers**

In `internal/dice/contest_floors.go`, add as the LAST line of each docstring —
immediately above the `func` line, after a blank `//` line.

Above `func OpposedRollStat`:

```go
//
// Deprecated: use combat.RunWithGlobalFloors. Zero production callers as of U4;
// retained as the U4 equivalence oracle and for internal delegation. U6 deletes
// it.
```

Above `func OpposedRollStatWithFloors`:

```go
//
// Deprecated: use combat.RunWithGlobalFloors, combat.RunWithManeuverFloors or
// combat.RunWithSpellFloors. Zero production callers as of U4. U6 deletes it.
```

- [ ] **Step 2: Replace the two lookup tables with per-package maps**

Replace the existing `unflooredRollExemptions` and `unflooredRollFuncs`
declarations with:

```go
// guardedRollFuncs are the roll entry points a production caller must not reach
// for, keyed by the package they live in.
//
// Two different reasons appear here:
//
//   - dice.OpposedRollStatRaw / dice.OpposedRoll and contest.Run /
//     contest.AgainstDifficulty apply NO floor. That is the original chunk 5.9
//     hazard. internal/contest joined the list in U4: Run and AgainstDifficulty
//     are exported and unfloored, so before U4 a new contest could opt out of
//     the floors through the very package this arc built to prevent that -- and
//     the guard, which hardcoded the callee package to "dice", said nothing.
//   - dice.OpposedRollStat / dice.OpposedRollStatWithFloors ARE floored, and are
//     guarded for a different reason: U4 emptied them of production callers and
//     U6 deletes them. The risk is drift BACK onto the legacy path.
var guardedRollFuncs = map[string]map[string]bool{
	"dice": {
		"OpposedRollStatRaw":         true,
		"OpposedRoll":                true,
		"OpposedRollStat":            true,
		"OpposedRollStatWithFloors":  true,
	},
	"contest": {
		"Run":               true,
		"AgainstDifficulty": true,
	},
}

// guardedRollExemptions lists the callers allowed to reach each guarded entry
// point, with the reason each one genuinely needs it.
//
// Keyed by callee package, then by a repo-relative FILE or DIRECTORY. A caller
// matches if its path equals a key or sits underneath one.
//
// Prefer a FILE key. internal/combat is 30+ files and is the single most likely
// place a new unfloored contest gets written; a directory exemption there would
// blind the guard in the package it most needs to watch. Exactly one call needs
// it, so exactly one file is named.
var guardedRollExemptions = map[string]map[string]string{
	"dice": {
		// Owns the primitives and delegates between them: OpposedRollStat ->
		// OpposedRollStatWithFloors -> OpposedRollStatRaw -> OpposedRoll.
		"internal/dice": "owns the roll primitives and delegates between them",
	},
	"contest": {
		// Melee is the one floor style that floors AFTER the contest rather than
		// inside the roll: resolveDefenseOutcomeCore floors a computed hit
		// CHANCE, not a roll outcome. Reconciling the two styles is an open U6
		// question; until then this single call is correct.
		"internal/combat/combat_helpers.go": "floors after the contest in resolveDefenseOutcomeCore",
	},
}
```

> Revision 1 also listed `"internal/contest"` as an exemption. It is dropped:
> `RunWithFloors` and `AgainstDifficulty` call `Run(...)` as a **bare
> identifier**, not a `contest.`-qualified selector, so the visitor never sees
> them and the entry could never fire. Keeping it would have documented a check
> that does not exist. The corresponding blind spot — a same-package caller
> inside `internal/contest` — is noted in Step 4's docstring.

- [ ] **Step 3: Rewrite the walk to consult the per-package maps**

The exemption check currently runs once per file, before parsing, against a flat
map. It now depends on which package is being called, so it moves inside the AST
visitor. Replace this block:

```go
		dir := filepath.ToSlash(filepath.Dir(rel))
		for exempt := range unflooredRollExemptions {
			if dir == exempt || strings.HasPrefix(dir, exempt+"/") {
				return nil
			}
		}
```

with:

```go
		dir := filepath.ToSlash(filepath.Dir(rel))
```

Then replace the entire `ast.Inspect(file, func(n ast.Node) bool { ... })` call
with:

```go
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if !guardedRollFuncs[pkg.Name][sel.Sel.Name] {
				return true
			}
			if isExempt(rel, dir, guardedRollExemptions[pkg.Name]) {
				return true
			}
			offenders = append(offenders,
				rel+": "+pkg.Name+"."+sel.Sel.Name+" at line "+
					strconv.Itoa(fset.Position(call.Pos()).Line))
			return true
		})
```

- [ ] **Step 4: Add the `isExempt` helper and rewrite the test docstring**

Add above `TestOpposedContestsAreFloored`:

```go
// isExempt reports whether a caller is covered by an exemption set. It matches a
// FILE key exactly, or a DIRECTORY key against the file's directory or any
// directory beneath it.
//
// A nil or missing map yields false, which is the safe default: an unlisted
// callee package exempts nobody.
func isExempt(rel, dir string, exemptions map[string]string) bool {
	for exempt := range exemptions {
		if rel == exempt || dir == exempt || strings.HasPrefix(dir, exempt+"/") {
			return true
		}
	}
	return false
}
```

Then replace the docstring of `TestOpposedContestsAreFloored` (currently
describing a dice-only guard, and closing with "you want OpposedRollStat", which
U4 makes wrong advice) with:

```go
// TestOpposedContestsAreFloored fails when production code reaches for a
// guarded roll entry point without an exemption.
//
// This is the recurrence guard for roadmap chunk 5.10, extended by U4. The
// floors were written for combat, lived in internal/combat/combat_helpers.go,
// and every contest added afterwards silently got the unfloored path --
// stealth, theft, traps, detection, spells and maneuvers. A stat-100 thief
// against a stat-150 mark succeeded 0.9% of the time. Nobody chose that.
//
// The guidance pointed the wrong way too: CLAUDE.md told developers to use
// dice.OpposedRollStat, and before 5.10 that was the UNFLOORED function. Chunk
// 5.10 made the floored roll the default by giving it the natural name; U4 then
// moved every production caller onto the internal/combat wrapper family and
// deprecated the dice pair, so CLAUDE.md was updated again. If you are reading
// this because the guard failed, read the wrapper docs in
// internal/combat/contest_floors.go before adding an exemption.
//
// KNOWN BLIND SPOT: the visitor matches only package-qualified calls
// (pkg.Func). A same-package call inside internal/dice or internal/contest is a
// bare identifier and is invisible here. Those two packages own the primitives
// and compose them internally, which is why that is acceptable rather than a
// gap to close.
//
// Test files are deliberately NOT scanned. Tests probe the raw distribution on
// purpose (see internal/combat/regression_test.go, which asserts on z-scores);
// the risk being guarded is a PRODUCTION contest silently opting out.
//
// If you are adding a caller that genuinely applies its own floors -- as
// combat's resolveAttack does, flooring a computed hit CHANCE rather than a roll
// outcome -- add its FILE to the matching guardedRollExemptions entry with a
// reason. If you cannot write the reason, you want a floored wrapper.
```

- [ ] **Step 5: Update the failure message**

Replace the final `t.Errorf` call with:

```go
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("guarded opposed rolls outside the exemption list:\n  %s\n\n"+
			"Use combat.RunWithGlobalFloors, combat.RunWithManeuverFloors or "+
			"combat.RunWithSpellFloors -- whichever floor pair matches the cost of "+
			"a single failure at this site. If this caller genuinely applies its "+
			"own floors, add its file to the matching guardedRollExemptions entry "+
			"with a reason. If you cannot write the reason, you want a floored "+
			"wrapper.",
			strings.Join(offenders, "\n  "))
	}
```

- [ ] **Step 6: Run the guard — it must PASS**

```bash
go test . -run TestOpposedContestsAreFloored -v
```

Expected: PASS. The only production `contest.Run` caller is
`internal/combat/combat_helpers.go`, exempted by file.

- [ ] **Step 7: Prove the guard bites**

A guard that passes because it matches nothing is worse than none. Temporarily
add this line inside the first function body of `internal/actions/sneak.go`:

```go
	_ = contest.Run(1, []contest.Entry{{Score: 1}})
```

You will also need a temporary `contest` import in that file.

```bash
go test . -run TestOpposedContestsAreFloored
```

Expected: **FAIL**, naming `internal/actions/sneak.go` and `contest.Run`.

Now revert both the line and the import:

```bash
git checkout -- internal/actions/sneak.go
go test . -run TestOpposedContestsAreFloored
git status --short internal/actions/sneak.go
```

Expected: PASS, and `git status --short` prints nothing (the file is tracked and
committed as of Task 2, so `git checkout --` fully restores it).

- [ ] **Step 8: Commit**

```bash
git add contest_floor_guard_test.go internal/dice/contest_floors.go
git commit -m "test(contest): the floor guard sees contest.* and the deprecated dice pair (U4)"
```

---

### Task 10: Guard the floor pair AND the attacker direction, closed rather than opt-in

This answers the trap at the top of this plan. In production all three pairs are
`0.05`, so a site wired to the wrong pair is behaviourally invisible. Nothing
that measures outcomes can catch it. A static check can.

It also guards **attacker direction**, which nothing else does. The 19
replacements are direction-correct today, but U6 will rewrite these very call
sites, and an inversion compiles cleanly and silently flips stealth.

**Files:**
- Create: `floor_pair_guard_test.go` (repo root, `package main`)

- [ ] **Step 1: Write the guard**

Create `floor_pair_guard_test.go`:

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

// siteExpectation declares what a migrated contest site must look like.
type siteExpectation struct {
	pair       string   // "global", "maneuver" or "spell"
	calls      int      // how many floor-wrapper calls the file must contain
	attackArgs []string // sorted identifier names appearing as the FIRST argument
}

// floorPairOwners records the floor pair, call count and attacker direction of
// every migrated contest site.
//
// WHY THIS TEST EXISTS. config.yaml ships MinContestSuccessChance,
// MinSpellHitChance and MinManeuverHitChance all at 0.05. So wiring a site to
// the WRONG pair changes nothing observable in production: it passes every unit
// test, every statistical check and every playtest, and turns into a live
// balance bug the moment U6 retunes one pair independently. No behavioural test
// can catch that. This one reads the source instead.
//
// The pairs encode the COST OF A SINGLE FAILURE, which is the principle the 5.9
// arc settled on -- a maneuver burns the whole round, an out-of-combat attempt
// is one shot plus a consequence. The mapping below is a design statement, not
// an implementation detail.
//
// attackArgs additionally pins WHO IS ATTACKING. usercommands/go.go is the
// dangerous one: two of its four contests roll the mover's sneak as the attack
// and two roll the mover's search, and swapping them compiles and inverts
// stealth for the whole game.
var floorPairOwners = map[string]siteExpectation{
	"internal/actions/sneak.go":  {"global", 2, []string{"sneakScore", "sneakScore"}},
	"internal/actions/shadow.go": {"global", 1, []string{"searchScore"}},
	"internal/actions/steal.go": {"global", 4,
		[]string{"attackerScore", "attackerScore", "attackerScore", "searchScore"}},
	"internal/actions/plant.go": {"global", 4,
		[]string{"attackerScore", "attackerScore", "attackerScore", "searchScore"}},
	"internal/actions/defuse.go": {"global", 1, []string{"defuseScore"}},
	"internal/usercommands/go.go": {"global", 4,
		[]string{"observerScore", "observerScore", "sneakScore", "sneakScore"}},
	"internal/usercommands/skill.skullduggery.shadow.go": {"global", 1, []string{"targetScore"}},

	// Flee is a MANEUVER: contested in combat, costs the round.
	"internal/combat/flee.go": {"maneuver", 2, []string{"fleeScore", "fleeScore"}},
}

// floorPairFuncs maps a wrapper name to the pair it applies.
var floorPairFuncs = map[string]string{
	"RunWithGlobalFloors":   "global",
	"RunWithManeuverFloors": "maneuver",
	"RunWithSpellFloors":    "spell",
}

// TestMigratedSitesKeepTheirFloorPair checks every declared site for the right
// pair, the right number of contests, and the right attacker.
func TestMigratedSitesKeepTheirFloorPair(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var offenders []string
	fset := token.NewFileSet()

	for rel, want := range floorPairOwners {
		path := filepath.Join(root, filepath.FromSlash(rel))

		file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			offenders = append(offenders, rel+
				": cannot parse (renamed or deleted? update floorPairOwners): "+perr.Error())
			continue
		}

		var gotArgs []string
		count := 0
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := wrapperName(call)
			if !ok {
				return true
			}
			gotPair := floorPairFuncs[name]
			count++
			if gotPair != want.pair {
				offenders = append(offenders, rel+": uses the "+gotPair+
					" pair via "+name+", want the "+want.pair+" pair, at line "+
					strconv.Itoa(fset.Position(call.Pos()).Line))
			}
			if len(call.Args) > 0 {
				if id, ok := call.Args[0].(*ast.Ident); ok {
					gotArgs = append(gotArgs, id.Name)
				} else {
					gotArgs = append(gotArgs, "<non-identifier>")
				}
			}
			return true
		})

		if count != want.calls {
			offenders = append(offenders, rel+": found "+strconv.Itoa(count)+
				" floor-wrapper calls, want "+strconv.Itoa(want.calls)+
				" (a site was added, removed, or left on the legacy path)")
		}

		sort.Strings(gotArgs)
		wantArgs := append([]string(nil), want.attackArgs...)
		sort.Strings(wantArgs)
		if strings.Join(gotArgs, ",") != strings.Join(wantArgs, ",") {
			offenders = append(offenders, rel+": attacker arguments are ["+
				strings.Join(gotArgs, " ")+"], want ["+strings.Join(wantArgs, " ")+
				"] — a contest direction was inverted")
		}
	}

	// Closed, not opt-in: any OTHER production file calling a floor wrapper must
	// be declared. Without this the guard would protect exactly these 8 files
	// forever and silently ignore every new contest.
	offenders = append(offenders, undeclaredWrapperCallers(t, root, fset)...)

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("contest sites on the wrong floor pair or direction:\n  %s\n\n"+
			"config.yaml ships all three pairs at 0.05, so a wrong pair is "+
			"invisible to every behavioural test. See the U4 plan for why the "+
			"pairs differ.",
			strings.Join(offenders, "\n  "))
	}
}

// wrapperName returns the floor-wrapper name a call expression invokes.
// It accepts both combat.RunWithGlobalFloors and the same-package bare form.
func wrapperName(call *ast.CallExpr) (string, bool) {
	var name string
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		name = fn.Sel.Name
	case *ast.Ident:
		name = fn.Name
	default:
		return "", false
	}
	if _, ok := floorPairFuncs[name]; !ok {
		return "", false
	}
	return name, true
}

// undeclaredWrapperCallers finds production files that call a floor wrapper but
// are not declared in floorPairOwners. internal/combat/contest_floors.go is
// skipped because it DEFINES them.
func undeclaredWrapperCallers(t *testing.T, root string, fset *token.FileSet) []string {
	t.Helper()

	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		if rel == "internal/combat/contest_floors.go" {
			return nil // defines the wrappers
		}
		if _, declared := floorPairOwners[rel]; declared {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name, ok := wrapperName(call); ok {
				out = append(out, rel+": calls "+name+" at line "+
					strconv.Itoa(fset.Position(call.Pos()).Line)+
					" but is not declared in floorPairOwners")
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	return out
}
```

- [ ] **Step 2: Run it**

```bash
go test . -run TestMigratedSitesKeepTheirFloorPair -v
```

Expected: PASS.

> If it reports undeclared callers, those are the U3-migrated maneuver and spell
> sites that already call `RunWithManeuverFloors` / `RunWithSpellFloors`. That is
> a genuine finding, not a bug in the guard: add each to `floorPairOwners` with
> its real pair, call count and attacker argument, read from the source. Do not
> weaken the closed check to make it quiet.

- [ ] **Step 3: Prove it bites, twice**

Wrong pair — edit `floorPairOwners["internal/combat/flee.go"]`'s pair from
`"maneuver"` to `"global"`:

```bash
go test . -run TestMigratedSitesKeepTheirFloorPair
```

Expected: **FAIL**, "uses the maneuver pair via RunWithManeuverFloors, want the
global pair".

Wrong direction — restore the pair, then change
`floorPairOwners["internal/actions/sneak.go"]`'s `attackArgs` to
`[]string{"sneakScore", "observerScore"}`:

```bash
go test . -run TestMigratedSitesKeepTheirFloorPair
```

Expected: **FAIL**, "attacker arguments are [...] — a contest direction was
inverted".

Now restore both edits and confirm the file is clean. The file is untracked at
this point, so `git diff` cannot help — check the content directly:

```bash
grep -n '"internal/combat/flee.go"' floor_pair_guard_test.go
grep -n '"internal/actions/sneak.go"' floor_pair_guard_test.go
go test . -run TestMigratedSitesKeepTheirFloorPair
```

Expected: flee shows `"maneuver"`, sneak shows two `"sneakScore"` entries, and
the test PASSES.

- [ ] **Step 4: Commit**

```bash
git add floor_pair_guard_test.go
git commit -m "test(contest): guard floor pair, call count and attacker direction (U4)"
```

---

### Task 11: Assign the unowned contests, fix the guidance, ship the docs

- [ ] **Step 1: Breadcrumb `surprise_attack.go`**

In `internal/actions/surprise_attack.go`, find the swing loop (around line 218):

```go
	for _, w := range weapons {
		// Hit check: roll 0-99; if result < penaltyPct → miss this weapon
		penaltyPct := int(w.hitPenalty * 100)
		roll := util.Rand(100)
```

Replace the comment with:

```go
	for _, w := range weapons {
		// Per-weapon SELF-penalty: roll 0-99; if result < penaltyPct, this
		// weapon's swing is dropped.
		//
		// NOTE(U9): surprise attack has NO HIT RESOLUTION. The primary weapon is
		// appended above with hitPenalty 0.0, so penaltyPct is 0 and this roll
		// never fires for it -- every primary surprise swing is an automatic hit
		// regardless of the target. The roll only applies to offhand and
		// extra-arm swings, and there is no defender term anywhere: the target's
		// stats, skills and defences never enter. A surprise attack against a
		// novice and against the Elemental King resolve identically.
		//
		// U4 declined it. Giving it a defender is a behaviour change and U1-U5
		// are contracted as provable no-ops (standing rule 3); U3 made the same
		// call for the Position_GrappleTick z-normalisation. U9 owns it, as the
		// chunk that converts non-contests into contests (concentration,
		// knockdown, prone recovery). Model the shift first -- every surprise
		// attack in the game moves.
		penaltyPct := int(w.hitPenalty * 100)
		roll := util.Rand(100)
```

- [ ] **Step 2: Breadcrumb the static-difficulty family**

In `internal/actions/search.go`, immediately above the Tier 2 hidden-players
block (around line 140):

```go
	// NOTE(unassigned, see UNIFIED_RESOLUTION_ROADMAP "Category B"): the six
	// dice.RollStat threshold checks in this file are the LAST uncertain
	// outcomes off the contest core. The two below are the sharpest problem:
	// they answer "does the observer spot the hider?" with a flat 135 threshold
	// that never reads the hider's sneak score, while
	// usercommands/go.go resolves the SAME question as an opposed contest
	// (observerScore vs hiddenScore). A hider's skill decides the outcome in one
	// path and is ignored in the other. Mobs reach this path too, via
	// behaviortree conditions_scout.go.
	//
	// U4 migrated go.go's opposed version and deliberately did NOT touch these:
	// converting a flat threshold into a contest is a behaviour change, and
	// U1-U5 are provable no-ops. Whichever chunk claims them must reconcile the
	// two implementations, not just move one.
```

In `internal/actions/track.go` above line 121 and
`internal/forager/forage_core.go` above line 126, add:

```go
	// NOTE(unassigned, see UNIFIED_RESOLUTION_ROADMAP "Category B"): a static
	// difficulty check still off the contest core. contest.AgainstDifficulty was
	// built for exactly this and currently has zero production callers.
```

- [ ] **Step 3: Fix `CLAUDE.md`**

`CLAUDE.md` currently tells every future agent: *"For all stat-based rolls use
`dice.RollStat(mean)` or `dice.OpposedRollStat(atk, def)`"*. After U4 that
function has zero production callers and is deprecated. The guard's own docstring
records that CLAUDE.md pointed the wrong way once before, in the opposite
direction; do not let it happen again.

In the "Dice & Rolling System" section, replace the `OpposedRollStat` bullet with:

```markdown
- **For an opposed contest, call the wrapper for your floor pair** in
  `internal/combat/contest_floors.go`: `RunWithGlobalFloors` (out-of-combat:
  stealth, theft, traps, detection), `RunWithManeuverFloors` (maneuvers, flee),
  `RunWithSpellFloors` (spells). All three return a `contest.Result`; read
  `.Success`. Pick by the COST OF A SINGLE FAILURE, not by resemblance.
- `dice.OpposedRollStat` / `OpposedRollStatWithFloors` are **deprecated** — U4
  moved every production caller onto the wrappers and U6 deletes them.
  `contest.Run` and `contest.AgainstDifficulty` are **unfloored**; both root
  guard tests fail a new production caller.
- `dice.RollStat(mean)` is still correct for a single non-contested roll.
```

- [ ] **Step 4: Fix the one stale test comment**

`internal/actions/shadow_test.go:245` documents the migrated site as using the
removed function:

```go
//	Detection:   OpposedRollStat(searchScore, sneakScore);
```

Change to:

```go
//	Detection:   combat.RunWithGlobalFloors(searchScore, sneakScore);
```

> This is a **comment-only** edit to a pre-existing test file. Standing rule 3
> guards against a migration altering *semantics*; a stale comment describing the
> removed model is what Definition of Done #7 requires fixing. Step 9's check is
> written to allow exactly this.

- [ ] **Step 5: Sweep for stale claims across code AND docs**

Revision 1's sweep was `grep -rn "OpposedRollStat" internal/*/context.md docs/`
— it scanned neither `.go` files nor test files, which is the same narrow-grep
failure that hid `resolveCharmSpell` in U2. Use:

```bash
grep -rn "OpposedRollStat" --include=*.go --include=*.md internal/ modules/ CLAUDE.md | grep -v "^internal/dice/"
```

Every remaining hit must be one of: explicitly past-tense historical narrative;
a `Deprecated:` marker; or a guard-test table entry. Anything phrased as current
behaviour is stale. Known targets:

- `internal/util/context.md:124` — "All stat-based resolution goes through
  `internal/dice` (`RollStat`, `OpposedRollStat`)." Stated as current. Fix.
- `internal/combat/context.md:516` — "The only `dice.OpposedRollStatWithFloors`
  callers left in the codebase are the two in `flee.go`, which U4 owns." Goes
  false with Task 8. Fix.

**Do not** rewrite `docs/roadmaps/ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md`. It
is historical design narrative about a past chunk and is out of scope, even where
present-tense.

- [ ] **Step 6: Update `context.md` for all five packages**

- `internal/combat/context.md` — add `RunWithGlobalFloors` and `ContestFloors`
  to the Public API with verified signatures; add a Gotcha:

```markdown
- **Three floor pairs, all shipping at 0.05.** `RunWithGlobalFloors`,
  `RunWithManeuverFloors` and `RunWithSpellFloors` are NOT interchangeable, but
  in production the wrong one is numerically invisible and becomes a balance bug
  when U6 retunes a pair. `floor_pair_guard_test.go` at the repo root guards the
  pair, the call count and the attacker direction of every migrated site.
- **The three pairs are NOT equal in a test binary.** The global pair reads
  `dice.ContestFloors()`, a package var defaulting to 0.05. The maneuver and
  spell pairs read `configs.GetBalanceConfig()`, which is never loaded in a test
  binary, so they measure **0**. The 0.05 in `config.balance.misc.go` is a
  validation fallback (`< 0 || > 0.50`) that the zero value passes untouched.
- **`Result.Floored` is available and currently unread.** These wrappers are the
  single choke point for every out-of-combat contest, so they are the cheapest
  place to instrument the floor-reliance rate the roadmap wants modelled before
  U6.
```

- `internal/contest/context.md` — the `## Files` table currently reads
  `| contest.go | Entry, Result, Run, AgainstDifficulty — the whole package |`.
  It is already stale (omits `RunWithFloors`). Correct it, and add a Gotcha:

```markdown
- **`Run` and `AgainstDifficulty` are UNFLOORED.** Migrating a floored caller
  onto either silently deletes its floor. `contest_floor_guard_test.go` fails any
  new production caller outside `internal/combat/combat_helpers.go`.
- **`AgainstDifficulty` has zero production callers** as of U4. The static-check
  sites it was built for (`actions/search.go` x6, `actions/track.go`,
  `forager/forage_core.go`) are still off the core and unassigned; see the
  roadmap's Category B.
- **`Run` leaves both `RollResult`s' `.Success` and `.Margin` at zero.** It uses
  `dice.Roll`, not `dice.OpposedRoll`. Read `Result.Margin`, never
  `Result.DefenseRoll.Margin`.
```

- `internal/actions/context.md` and `internal/usercommands/context.md` — neither
  currently mentions `dice` or `contest` for these paths, so there is nothing to
  *correct*; the required change is an **addition**. Add `internal/combat` to
  each file's `## Dependencies`, and one line noting that stealth, theft, trap
  and detection contests resolve through `combat.RunWithGlobalFloors`.
  (`tools/context_md_audit.py` detects phantom symbols, not omitted ones, so
  Step 7 will not catch a miss here.)

- [ ] **Step 7: Run the audit**

```bash
python tools/context_md_audit.py
```

Expected: zero phantom symbols in the five packages touched.

- [ ] **Step 8: Update the roadmap**

In `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`:

1. Mark U4 done in the Plans table, matching U3's phrasing. State the real count:
   **19 sites, 17 on the global pair and 2 on the maneuver pair.**
2. Ownership-gaps table: mark `combat/flee.go` **shipped in U4**.
3. Change `actions/surprise_attack.go`'s "Assigned to" cell from **U4** to
   **U9**, and correct its description — it is not "hand-rolled per-weapon hit
   resolution"; the primary weapon carries `hitPenalty 0.0` and never rolls, so
   surprise attack has no hit resolution at all. **Add:** "The user intends to
   redesign this skill/effect rather than only give it a defender term, so U9
   should treat it as a design slice (brainstorm → spec → plan), not a
   mechanical migration. Decided 2026-08-13."
4. Add a new row to the unowned table:

```markdown
| `actions/search.go` x6, `actions/track.go`, `forager/forage_core.go` | Static-difficulty checks still off the core. Two of search.go's (hidden players, hidden mobs) answer the SAME question as `usercommands/go.go`'s opposed hidden-detection contest, but with a flat 135 threshold that ignores the hider's score entirely. `contest.AgainstDifficulty` was built for these and has zero production callers. | **UNASSIGNED** — U4 found them and breadcrumbed each site. Converting them is a behaviour change, so U4 could not claim them. |
```

5. Under "The traps", add:

```markdown
- **All three floor pairs ship at 0.05.** Wiring a contest to the wrong pair
  changes nothing observable in production and becomes a balance bug the moment
  U6 retunes one. Behavioural tests cannot see it; `floor_pair_guard_test.go`
  reads the source instead.
- **The three pairs differ in a TEST binary**, and not the way you would guess:
  global reads a `dice` package var (0.05), maneuver and spell read config
  (never loaded under test, so **0**). Never quote a Go default as a live value.
```

6. Correct the standing description of the migrated sites: they were **floored**
   via `dice.OpposedRollStat`, not unfloored. Chunk 5.10 made the floored roll
   the default name and the earlier notes predate that.
7. Clarify lines 132-135: "Search, track and forage stay static" was written as a
   statement about their *shape*, and was read by U4's first draft as a statement
   about their *ownership*. Say which is meant. They remain unassigned.

- [ ] **Step 9: Add the patch note**

Append to `docs/PATCH_NOTES.md`. Player-facing framing, no raw numbers, no em
dashes:

```markdown
### 2026-08-13

- Sneaking, stealing, planting, defusing, shadowing and slipping away from a
  fight now resolve through the same shared system the rest of the game uses.
  Nothing about how often these succeed has changed. This is groundwork for the
  combat rebalance to come.
```

- [ ] **Step 10: Full verification**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/combat/ ./internal/contest/ ./internal/actions/ ./internal/usercommands/ ./internal/dice/ ./internal/behaviortree/
go test . -run 'TestOpposedContestsAreFloored|TestMigratedSitesKeepTheirFloorPair' -v
```

Expected: gofmt silent, build succeeds, all tests pass.
`internal/behaviortree` is included deliberately: its
`actions_skullduggery_test.go` pins the contest floors via
`dice.SetContestFloors`, and it is the test most likely to notice if the wrapper
stopped reading the `dice` globals.

**Known noise:** the `internal/relationships` test binary is quarantined by
Windows Defender. Expected and unrelated — let CI run it.

- [ ] **Step 11: Confirm the no-op contract held**

```bash
git diff master --stat -- '*_test.go'
```

Expected, and nothing else:

- `contest_floor_guard_test.go` (Task 9, intentional)
- `floor_pair_guard_test.go` (Task 10, new)
- `internal/combat/global_floors_test.go` (Task 1, new)
- `internal/actions/shadow_test.go` (Task 11 Step 4, **comment only**)

Verify that last one really is comment-only:

```bash
git diff master -- internal/actions/shadow_test.go
```

Expected: a single changed line, beginning `//`.

**If any other pre-existing test file was modified, stop.** Standing rule 3 says
that means a modifier moved and the migration was not a no-op. Find out why.

- [ ] **Step 12: Boot test in an isolated worktree on NON-DEFAULT ports**

The user runs the local server continuously. `_datafiles/config.yaml` has
`skip-worktree` set and holds the live port numbers, so copying it verbatim makes
the boot test either fail to bind or silently point at the user's running server
— which would make a broken build look clean.

```bash
git worktree add --detach C:/tmp/dogmud-u4-boot HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-u4-boot/_datafiles/config.yaml
```

Now edit `C:/tmp/dogmud-u4-boot/_datafiles/config.yaml` and change the telnet
port, `HttpPort`, and any other bound port to unused values (e.g. +1000 each).
Then:

```bash
cd C:/tmp/dogmud-u4-boot && timeout 180 go run . > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
grep -iE "bind|address already in use" boot.log                          # want 0
```

**Exit code 124 is the success case** — the timeout fired because the server
stayed up. Do not grep for the bare word `panic`:
`GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`. The third
grep is the check that the test server actually bound its own ports.

Clean up:

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git worktree remove --force C:/tmp/dogmud-u4-boot
```

- [ ] **Step 13: Commit**

```bash
git add internal/actions/surprise_attack.go internal/actions/search.go \
        internal/actions/track.go internal/forager/forage_core.go \
        internal/actions/shadow_test.go CLAUDE.md \
        internal/combat/context.md internal/contest/context.md \
        internal/actions/context.md internal/usercommands/context.md \
        internal/util/context.md \
        docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md docs/PATCH_NOTES.md
git commit -m "docs(contest): U4 ships its docs, assigns the unowned static contests, and fixes the dice guidance"
```

---

## Ship it

Per CLAUDE.md, ship through a PR and merge from the terminal. **`gh` defaults to
the fork parent — always pass `--repo pruuk/DOGMud`.**

```bash
git push -u origin feature/u4-non-harm-contests
gh pr create --repo pruuk/DOGMud --base master --head feature/u4-non-harm-contests --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
gh pr merge  <n> --repo pruuk/DOGMud --merge --delete-branch
```

Use `--merge`, not `--squash`. **A green `gh pr checks --watch` can return
early** — confirm which runs actually re-ran before merging.

**Do not propose a deploy.** The arc is under a deploy gate: nothing ships until
U0–U10 are complete and playtested, by the harness and by the user. U4 is a no-op
and changes nothing a player can see.

---

## Definition of done

1. `grep -rn "dice\.OpposedRoll[A-Za-z]*(" internal/ modules/ --include=*.go | grep -v _test | grep -v "^internal/dice/"` returns nothing.
2. `dice.OpposedRollStat` and `dice.OpposedRollStatWithFloors` have zero production callers and carry `Deprecated:` markers. Both are in the floor guard's watch list with an `internal/dice` exemption.
3. All 17 non-combat sites use `combat.RunWithGlobalFloors`; both flee sites use `RunWithManeuverFloors`. Pair, call count and attacker direction all enforced by `floor_pair_guard_test.go`, which is **closed** — a new file calling a wrapper without being declared fails it.
4. `contest_floor_guard_test.go` fails on a new unexempted `contest.Run` / `contest.AgainstDifficulty` caller, demonstrated in Task 9 Step 7.
5. No pre-existing test file modified except `internal/actions/shadow_test.go`, comment only, verified by diff in Task 11 Step 11.
6. `context.md` accurate for `internal/combat`, `internal/contest`, `internal/actions`, `internal/usercommands`, `internal/util`; `tools/context_md_audit.py` reports zero phantom symbols.
7. No text outside `internal/dice` and the historical roadmaps describes these sites as using `dice.OpposedRollStat` as current behaviour. **`CLAUDE.md` updated.**
8. The unowned static-difficulty contests (`search.go` x6, `track.go`, `forager`) are breadcrumbed in code and recorded as UNASSIGNED in the roadmap. `surprise_attack.go` is breadcrumbed and reassigned to U9, with its behaviour described correctly.
9. Boot test clean on non-default ports, with no bind errors; `gofmt -l internal/ modules/` silent.
