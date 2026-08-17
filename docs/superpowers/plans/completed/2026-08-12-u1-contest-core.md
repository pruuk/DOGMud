# U1: Contest Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the "roll one attack against N defences, keep the widest margin" logic out of melee into a shared, pure package that every contest in the game can call — changing no behaviour whatsoever.

**Architecture:** A new leaf package `internal/contest` that depends only on `internal/dice`. It rolls and selects; it does **not** compute scores, spend resources, or know what a Character is. Callers build their own `[]Entry` with fully-modified scores and hand them over. `runBestOfAllDefense` becomes a thin adapter that builds entries, calls `contest.Run`, and maps the result back onto today's `bestDefenseResult`.

**Tech Stack:** Go, `internal/dice` for rolling, `testify/assert` for tests.

---

## Why a new package rather than putting this in `internal/combat`

`internal/combat` imports `characters`, `items`, `mutations` and `configs`. Two
of U1's eventual consumers cannot afford that weight or risk a cycle:
`internal/forager` currently imports only `dice` and `util`, and later plans move
`internal/usercommands` sites onto the core too.

`internal/contest` importing only `internal/dice` is safe for everything and
keeps the core **pure** — no config reads, no character state. That purity is
also what makes it testable: a Go test binary never loads
`_datafiles/config.yaml`, so any core that read balance config would test against
Go defaults and several assertions would be vacuously true.

## The sign convention — decided here, once

`dice.OpposedRoll` returns an **attack-positive** margin
(`attackRoll.Value - defenseRoll.Value`). `bestDefenseResult.margin` is
**defence-positive**. 33 of the 34 sites already use the attack-positive
convention, so the core adopts it and melee is the one that converts.

**`contest.Result.Margin` is ATTACK-POSITIVE. Positive means the attacker won.**

In this plan melee's adapter negates once, at the boundary, so
`bestDefenseResult.margin` keeps its existing defence-positive meaning and **no
existing melee test changes**. U6 deletes `bestDefenseResult` and the negation
with it.

## File Structure

| File | Responsibility |
|---|---|
| `internal/contest/contest.go` (new) | `Entry`, `Result`, `Run` — the whole core |
| `internal/contest/contest_test.go` (new) | Core behaviour tests |
| `internal/contest/context.md` (new) | Package docs, per the CLAUDE.md convention |
| `internal/combat/combat_helpers.go` (modify) | `runBestOfAllDefense` becomes an adapter |
| `internal/combat/context.md` (modify) | Record that melee now delegates |

## Success criteria for U1

1. `go test ./...` passes with **no existing test file modified**. The only test
   changes are new files under `internal/contest/`.
2. `internal/contest` imports nothing but `math` and `internal/dice`.
3. `runBestOfAllDefense` contains no `dice.Roll` call of its own.

---

### Task 1: Create the contest package with the roll-and-select core

**Files:**
- Create: `internal/contest/contest.go`
- Test: `internal/contest/contest_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/contest/contest_test.go`:

```go
package contest

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRun_MarginIsAttackPositive pins the convention the whole arc depends on.
// dice.OpposedRoll is attack-positive and bestDefenseResult is defence-positive;
// mixing them compiles cleanly and puts crit on the losing side. The core picks
// attack-positive because 33 of 34 call sites already use it.
func TestRun_MarginIsAttackPositive(t *testing.T) {
	// Attacker massively outclasses a single defender.
	res := Run(1000, []Entry{{Name: "dodge", Score: 1}})

	assert.True(t, res.Contested, "one entry means a contest happened")
	assert.Greater(t, res.Margin, 0.0, "attacker won, so margin must be POSITIVE")
	assert.Equal(t, "dodge", res.Winner)
}

// TestRun_MarginMatchesTheRolls guards the invariant that the reported margin is
// derived from the two reported rolls. If they ever drift apart, every
// downstream effect that scales by margin silently uses a number that does not
// correspond to what was rolled.
func TestRun_MarginMatchesTheRolls(t *testing.T) {
	for i := 0; i < 500; i++ {
		res := Run(100, []Entry{{Name: "a", Score: 90}, {Name: "b", Score: 110}})
		want := res.AttackRoll.Value - res.DefenseRoll.Value
		assert.InDelta(t, want, res.Margin, 1e-9, "margin must equal atk - def of the reported rolls")
	}
}

// TestRun_KeepsTheBestDefence verifies best-of-N selection: the entry that
// defended most successfully is the one reported, which is the entry with the
// SMALLEST attack-positive margin.
func TestRun_KeepsTheBestDefence(t *testing.T) {
	// "wall" scores so far above the others that it must win essentially always.
	strongWins := 0
	for i := 0; i < 200; i++ {
		res := Run(100, []Entry{
			{Name: "paper", Score: 1},
			{Name: "wall", Score: 10000},
		})
		if res.Winner == "wall" {
			strongWins++
		}
	}
	assert.Equal(t, 200, strongWins, "the far stronger defence must always win the selection")
}

// TestRun_NoEntriesIsNotAContest covers the trap that produced a real bug:
// bestDefenseResult initialises margin to math.Inf(-1) and only overwrites it
// inside the defence loop, so a defender with no defences left +Inf in place.
// Negated under margin-derived crit that reads as an infinitely decisive attack
// and crits EVERY swing. The core must never emit an infinity.
func TestRun_NoEntriesIsNotAContest(t *testing.T) {
	res := Run(100, nil)

	assert.False(t, res.Contested, "no entries means nothing was contested")
	assert.Equal(t, "", res.Winner)
	assert.Equal(t, 0.0, res.Margin, "margin must be a neutral zero, never an infinity")
	assert.False(t, math.IsInf(res.Margin, 0), "margin must never be infinite")
	assert.NotZero(t, res.AttackRoll.StdDev, "the attack roll still happens and is still reported")
}

// TestRun_EmptySliceBehavesLikeNil is the same guard for the other empty form.
func TestRun_EmptySliceBehavesLikeNil(t *testing.T) {
	res := Run(100, []Entry{})
	assert.False(t, res.Contested)
	assert.Equal(t, 0.0, res.Margin)
}

// TestRun_BothSidesUseTheAttackersStdDev pins the normaliser assumption every
// crit calculation in the game depends on: because both rolls share the
// attacker's stdDev, their difference has stdDev*sqrt(2). If a defence were
// rolled with its OWN stdDev, dividing by stdDev*sqrt(2) downstream would be
// wrong and crit rates would shift everywhere.
func TestRun_BothSidesUseTheAttackersStdDev(t *testing.T) {
	res := Run(200, []Entry{{Name: "d", Score: 50}})
	assert.Equal(t, res.AttackRoll.StdDev, res.DefenseRoll.StdDev,
		"defence must be rolled with the ATTACKER's stdDev")
}

// TestRun_ZeroScoreAttackerDoesNotPanic — dice.StdDevFor floors at 1.0 for any
// mean below 1.0, so a zero-score attacker still rolls with a real spread rather
// than degenerating. This pins that the core does not produce NaN when scores
// collapse to zero.
func TestRun_ZeroScoreAttackerDoesNotPanic(t *testing.T) {
	res := Run(0, []Entry{{Name: "d", Score: 0}})
	assert.True(t, res.Contested)
	assert.False(t, math.IsNaN(res.Margin), "margin must not be NaN")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/contest/ -count=1`

Expected: FAIL to build, with `undefined: Run` and `undefined: Entry`.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/contest/contest.go`:

```go
// Package contest is the shared resolution seam for every opposed roll in the
// game: it rolls one attack against N defences and reports which defence did
// best.
//
// It deliberately does NOT compute scores, spend resources, apply mitigation, or
// know what a Character is. Callers build fully-modified scores and hand them
// over. That keeps this package a leaf — it imports only internal/dice — so
// heavy packages and light ones alike can call it without a cycle.
//
// The purity is also what makes it testable. A Go test binary never loads
// _datafiles/config.yaml, so a core that read balance config would be tested
// against Go defaults, and any knob that legitimately defaults to zero would
// make its assertions vacuously true.
package contest

import (
	"github.com/GoMudEngine/GoMud/internal/dice"
)

// Entry is one contestant on the defending side.
//
// Score must be FULLY modified by the caller — every multiplier, penalty and
// bonus already applied. This package does no score maths.
//
// Name identifies the winner to the caller. An empty Name is legal and is how a
// contest against a static difficulty is expressed: one entry, no name.
type Entry struct {
	Name  string
	Score float64
}

// Result is the outcome of one contest.
type Result struct {
	// AttackRoll is the single roll every entry contested. Always populated,
	// even when nothing was contested, so callers can still read its ZScore for
	// legacy self-relative checks such as fumbles.
	AttackRoll dice.RollResult

	// DefenseRoll is the roll of the entry that defended best. Zero when
	// Contested is false.
	DefenseRoll dice.RollResult

	// Margin is ATTACK-POSITIVE: AttackRoll.Value - DefenseRoll.Value.
	// Positive means the attacker won.
	//
	// This is the opposite of internal/combat's bestDefenseResult.margin, which
	// is defence-positive. That mismatch is exactly why mixing the two compiles
	// cleanly and silently puts crit on the losing side. This package is the
	// single convention; converters live at the caller's boundary.
	//
	// Zero when Contested is false — NEVER an infinity. bestDefenseResult
	// initialises its margin to math.Inf(-1) and only overwrites it inside the
	// defence loop, so a defender with no usable defence leaves it there;
	// negated, that reads as an infinitely decisive attack and crits every swing.
	Margin float64

	// Winner is the Name of the entry that defended best. Empty when Contested
	// is false — and note it is ALSO empty for a legitimately-unnamed static
	// difficulty entry, so test Contested, never Winner, to ask whether a
	// contest happened.
	Winner string

	// Contested reports whether any entry was rolled against.
	Contested bool
}

// Run rolls the attack ONCE and contests it against every entry, reporting the
// entry that defended by the widest margin.
//
// The attack is rolled once on purpose: all defences contest the same swing, so
// a defender with three defences gets three chances to beat one roll rather than
// three separate swings to survive.
//
// Every defence is rolled with the ATTACKER's standard deviation. Downstream
// crit maths divides the margin by stdDev*sqrt(2) on the strength of that, so
// rolling a defence with its own spread would silently shift crit rates
// everywhere.
func Run(atkScore float64, entries []Entry) Result {
	stdDev := dice.StdDevFor(atkScore)
	attackRoll := dice.Roll(atkScore, stdDev)

	res := Result{AttackRoll: attackRoll}

	for _, e := range entries {
		defenseRoll := dice.Roll(e.Score, stdDev)
		margin := attackRoll.Value - defenseRoll.Value

		// The best defence is the one that leaves the SMALLEST attack-positive
		// margin. First entry always wins the comparison because Contested is
		// still false.
		if !res.Contested || margin < res.Margin {
			res.Contested = true
			res.Margin = margin
			res.Winner = e.Name
			res.DefenseRoll = defenseRoll
		}
	}

	return res
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/contest/ -count=1 -v`

Expected: PASS for all seven tests.

- [ ] **Step 5: Verify the package is a true leaf**

Run: `go list -deps ./internal/contest/ | grep GoMudEngine`

Expected: exactly two lines — `.../internal/dice` and `.../internal/contest`.
If `characters`, `configs`, `items` or `mutations` appear, the package is not
pure and the import must be removed.

- [ ] **Step 6: Commit**

```bash
git add internal/contest/contest.go internal/contest/contest_test.go
git commit -m "feat(contest): pure roll-and-select core for all opposed contests (U1)

One attack roll contested by N defences, keeping the widest margin. Rolls
and selects only: no score maths, no resource spending, no Character
knowledge, so it stays a leaf importing only internal/dice.

Margin is ATTACK-POSITIVE, matching dice.OpposedRoll and therefore 33 of
the 34 call sites. internal/combat's bestDefenseResult is defence-positive;
that mismatch is why mixing them compiles and silently puts crit on the
losing side. This package is now the single convention.

Margin is a neutral zero when nothing was contested, never math.Inf(-1) as
bestDefenseResult uses today. Negated, that infinity reads as an infinitely
decisive attack and crits every swing.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Add the static-difficulty helper

The sweep found 12 sites that roll against a fixed number with no opponent —
`search.go` ×6, `track.go`, forage, knockdown, prone recovery. They need the core
too, or they stay on a parallel path and the arc leaves a second resolution
system standing.

**Files:**
- Modify: `internal/contest/contest.go`
- Test: `internal/contest/contest_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/contest/contest_test.go`:

```go
// TestAgainstDifficulty covers the 12 sites that contest a fixed number rather
// than an opponent: search, track, forage, knockdown, prone recovery. Without
// this they cannot use the core and stay on a parallel path.
func TestAgainstDifficulty(t *testing.T) {
	// A hopeless attempt against a very high bar.
	res := AgainstDifficulty(10, 10000)
	assert.True(t, res.Contested, "a difficulty check is still a contest")
	assert.Less(t, res.Margin, 0.0, "failing badly means a negative attack-positive margin")

	// A trivial attempt against a very low bar.
	res = AgainstDifficulty(10000, 10)
	assert.Greater(t, res.Margin, 0.0)
}

// TestAgainstDifficulty_HasNoWinnerName documents that a difficulty contest has
// no named defender, which is why callers must test Contested rather than
// Winner to ask whether a contest occurred.
func TestAgainstDifficulty_HasNoWinnerName(t *testing.T) {
	res := AgainstDifficulty(100, 100)
	assert.True(t, res.Contested)
	assert.Equal(t, "", res.Winner, "a static difficulty has no name")
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/contest/ -run TestAgainstDifficulty -count=1`

Expected: FAIL to build with `undefined: AgainstDifficulty`.

- [ ] **Step 3: Write the minimal implementation**

Append to `internal/contest/contest.go`:

```go
// AgainstDifficulty contests a score against a fixed number rather than against
// an opponent — searching a room, following a trail, foraging, recovering from
// prone with nobody holding you down.
//
// It is deliberately the same code path as Run, so a difficulty check produces
// the same crit, fumble and margin semantics as any other contest. The
// alternative — a separate threshold helper — is how the codebase ended up with
// several unrelated ways to decide the same kind of question.
//
// The result has no Winner name. Ask Contested, not Winner, to find out whether
// a contest happened.
func AgainstDifficulty(score, difficulty float64) Result {
	return Run(score, []Entry{{Score: difficulty}})
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/contest/ -count=1`

Expected: PASS (all nine tests).

- [ ] **Step 5: Commit**

```bash
git add internal/contest/contest.go internal/contest/contest_test.go
git commit -m "feat(contest): contest against a static difficulty (U1)

The sweep found 12 sites that roll against a fixed number with no
opponent: search x6, track, forage, knockdown, prone recovery. Routing
them through the same code path as an opposed contest means they get the
same crit, fumble and margin semantics for free. A separate threshold
helper is how the codebase ended up with several unrelated ways to answer
the same kind of question.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Migrate `runBestOfAllDefense` onto the core

This is the no-op task. Every score modifier stays exactly where it is; only the
rolling and selecting move.

**Files:**
- Modify: `internal/combat/combat_helpers.go:540-662`

- [ ] **Step 1: Replace the body of `runBestOfAllDefense`**

Replace the whole function (currently lines 540–662, from the doc comment through
its closing brace) with:

```go
// runBestOfAllDefense rolls every available defense and picks the one that won
// by the widest margin. Returns the best result.
//
// Chunk U1: the rolling and selecting now live in internal/contest. Everything
// this function still does is melee-specific and deliberately stayed here —
// building each defence's score, tracking attempts for stance, checking
// affordability, and spending stamina on the winner.
func runBestOfAllDefense(result *AttackResult, sourceChar *characters.Character, targetChar *characters.Character, defSeq []string, atkScore float64, isThirdParty bool, ctx combatContext) bestDefenseResult {
	bal := configs.GetBalanceConfig()

	entries := make([]contest.Entry, 0, len(defSeq))

	for _, defenseType := range defSeq {
		// Track defense attempt
		result.DefenseAttempts = append(result.DefenseAttempts, DefenseType(defenseType))

		// Stage 9.4: Track defense for stance calculation
		targetChar.IncrementDefenseCount()

		// Check if defender can afford this defense (don't deduct yet)
		cost := targetChar.GetDefenseStaminaCost(defenseType)
		if targetChar.Stamina < cost {
			continue
		}

		// Calculate defense score for this defense type
		defenseScore := targetChar.GetDefenseScore(defenseType)

		// Apply base effectiveness multipliers
		switch defenseType {
		case characters.DefenseDodge:
			defenseScore *= float64(bal.DodgeEffectiveness)
		case characters.DefenseParry:
			defenseScore *= float64(bal.ParryEffectiveness)
		case characters.DefenseBlock:
			defenseScore *= float64(bal.BlockEffectiveness)
		}

		// Stage 7.5: Apply position-based defense penalties. Chunk 4b R1:
		// FSM-driven — Prone/Supine collapse to the legacy "prone"
		// penalty bucket, IsStandingGrapple matches the legacy
		// "clinched" bucket, IsGroundGrapple matches the legacy
		// "grounded" bucket.
		switch {
		case targetChar.IsProne() || targetChar.IsSupine():
			switch defenseType {
			case "dodge":
				defenseScore *= float64(bal.ProneDodgePenalty)
			case "parry":
				defenseScore *= float64(bal.ProneParryPenalty)
			case "block":
				defenseScore *= float64(bal.ProneBlockPenalty)
			}
		case targetChar.IsStandingGrapple():
			switch defenseType {
			case "dodge":
				defenseScore *= float64(bal.ClinchDodgePenalty)
			case "parry":
				defenseScore *= float64(bal.ClinchParryPenalty)
			case "block":
				defenseScore *= float64(bal.ClinchBlockPenalty)
			}
		case targetChar.IsGroundGrapple():
			switch defenseType {
			case "dodge":
				defenseScore *= float64(bal.GroundedDodgePenalty)
			case "parry":
				defenseScore *= float64(bal.GroundedParryPenalty)
			case "block":
				defenseScore *= float64(bal.GroundedBlockPenalty)
			}
		}

		// Rally condition: applies a defense score multiplier from rhetoric shout
		if targetChar.HasCondition(characters.ConditionRally) {
			defenseScore *= 1.0 + targetChar.GetConditionMagnitude(characters.ConditionRally)
		}

		// Stage 8.5: Apply third-party vulnerability penalty
		if isThirdParty {
			defenseScore *= float64(bal.ThirdPartyGrapplePenalty)
		}

		// Stage 8.6: Apply failed grapple defense penalty
		if targetChar.HasCondition(characters.ConditionDefensePenalty) {
			defenseScore *= targetChar.GetConditionMagnitude(characters.ConditionDefensePenalty)
		}

		// Darkness penalty: defender can't see
		if !ctx.targetCanSee {
			defenseScore *= float64(bal.DarknessCombatPenalty)
		}

		// Incorporeal mutation: physical defense bonus (channel-scoped
		// to physical attacks; this function only handles physical
		// swings — spells use a different resolution path).
		defenseScore += mutations.GetPhysicalDefenseBonus(targetChar.Mutations)

		entries = append(entries, contest.Entry{Name: defenseType, Score: defenseScore})
	}

	res := contest.Run(atkScore, entries)

	best := bestDefenseResult{
		hitRoll: res.AttackRoll,
	}
	if res.Contested {
		best.defenseType = res.Winner
		best.defRoll = res.DefenseRoll
		// SIGN CONVERSION, and the only one in melee. contest.Result.Margin is
		// ATTACK-positive; bestDefenseResult.margin is DEFENCE-positive. Negate
		// exactly here and nowhere else. U6 deletes bestDefenseResult and this
		// conversion with it.
		best.margin = -res.Margin
	} else {
		// Preserve the legacy sentinel exactly. normalizedAttackMargin detects
		// "no defence attempted" via defenseType == "" and never reads this
		// value, but resolveDefenseOutcomeCore's `best.margin > 0` check does,
		// and -Inf is what makes an uncontested swing fall through to a hit.
		best.margin = math.Inf(-1)
	}

	// Deduct stamina only for the winning defense
	if best.defenseType != "" {
		targetChar.DeductDefenseStamina(best.defenseType)
	}

	return best
}
```

- [ ] **Step 2: Add the import**

In `internal/combat/combat_helpers.go`, add to the import block:

```go
	"github.com/GoMudEngine/GoMud/internal/contest"
```

- [ ] **Step 3: Verify no behaviour changed**

Run: `go build ./... && go test ./internal/combat/ -count=1`

Expected: PASS, with **no test file edited**. If any melee test fails, the
migration changed behaviour — stop and find out which modifier moved rather than
adjusting the test.

- [ ] **Step 4: Verify the roll left melee**

Run: `sed -n '540,660p' internal/combat/combat_helpers.go | grep -c "dice.Roll"`

Expected: `0`. The function must no longer roll anything itself.

- [ ] **Step 5: Run the full suite**

Run: `go test ./... 2>&1 | grep -v "^ok\|no test files"`

Expected: only the known Windows Defender quarantine of
`internal/relationships` test binaries. Any other failure is a real regression.

- [ ] **Step 6: Verify formatting and commit**

```bash
gofmt -l internal/ modules/
go vet ./internal/combat/ ./internal/contest/
git add internal/combat/combat_helpers.go
git commit -m "refactor(combat): melee rolls through the contest core (U1)

runBestOfAllDefense no longer rolls anything. It builds each defence's
score exactly as before, hands the entries to contest.Run, and maps the
result back. Every score modifier -- effectiveness, position, rally,
third-party, darkness, incorporeal -- stays exactly where it was, because
those are melee's business and not the core's.

Behaviour is unchanged and no test was modified, which is the bar U1-U5
are held to.

The single sign conversion lives at the boundary and is commented as such:
contest.Result.Margin is attack-positive, bestDefenseResult.margin is
defence-positive. The uncontested case still returns the -Inf sentinel
because resolveDefenseOutcomeCore reads it.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Package documentation

Per the CLAUDE.md convention, any work creating a new package MUST ship a
`context.md`, and U1's roadmap rule 2 requires it in the same PR.

**Files:**
- Create: `internal/contest/context.md`
- Modify: `internal/combat/context.md`

- [ ] **Step 1: Write `internal/contest/context.md`**

```markdown
# internal/contest

## Purpose

The shared resolution seam for opposed rolls: roll one attack against N
defences, report which defence did best. Introduced by roadmap U1 to collapse 33
scattered opposed-roll sites (plus melee, which used none of them) onto one path.

It deliberately does **not** compute scores, spend resources, apply mitigation,
decide crits, or know what a Character is. Callers build fully-modified scores
and hand them over. That keeps it a leaf importing only `internal/dice`, so both
heavy packages (`internal/combat`) and light ones (`internal/forager`) can call
it without a cycle.

## Files

| File | Purpose |
|------|---------|
| `contest.go` | `Entry`, `Result`, `Run`, `AgainstDifficulty` — the whole package |

## Core types

```go
type Entry struct {
	Name  string  // identifies the winner; "" is legal for a static difficulty
	Score float64 // FULLY modified by the caller; this package does no score maths
}

type Result struct {
	AttackRoll  dice.RollResult // always populated, even when uncontested
	DefenseRoll dice.RollResult // zero when Contested is false
	Margin      float64         // ATTACK-POSITIVE; zero when uncontested, never infinite
	Winner      string          // Name of the best defence; "" when uncontested
	Contested   bool
}
```

## Public API

- `Run(atkScore float64, entries []Entry) Result` — one attack roll contested by
  every entry, keeping the widest defensive margin.
- `AgainstDifficulty(score, difficulty float64) Result` — the same path against a
  fixed number instead of an opponent.

## Gotchas

- **`Margin` is ATTACK-POSITIVE.** Positive means the attacker won. This is the
  opposite of `internal/combat`'s `bestDefenseResult.margin`, which is
  defence-positive. Mixing the two compiles cleanly and silently puts crit on the
  losing side. `runBestOfAllDefense` performs the one legitimate conversion, at
  its boundary, and says so in a comment.
- **Ask `Contested`, never `Winner`, to find out whether a contest happened.**
  `Winner` is also empty for a legitimately unnamed static-difficulty entry.
- **`Margin` is never an infinity.** `bestDefenseResult` initialises its margin
  to `math.Inf(-1)` and only overwrites it inside its loop, so a defender with no
  usable defence leaves it there — negated, that reads as an infinitely decisive
  attack and crits every swing. This package returns a neutral zero instead.
- **Every defence is rolled with the ATTACKER's stdDev.** Downstream crit maths
  divides the margin by `stdDev * sqrt(2)` on the strength of that. Rolling a
  defence with its own spread would silently shift crit rates everywhere.
- **The attack is rolled ONCE.** All defences contest the same swing, so three
  defences are three chances to beat one roll — not three swings to survive.
- **No config reads, deliberately.** A Go test binary never loads
  `_datafiles/config.yaml`, so a core that read balance config would be tested
  against Go defaults and any knob defaulting to zero would make its assertions
  vacuously true. Tunables are parameters.

## Dependencies

`internal/dice` only. Keep it that way — see Purpose.

## Consumers

`internal/combat` (`runBestOfAllDefense`). Roadmap U2–U4 add the spell, ranged,
taunt and non-harm sites.
```

- [ ] **Step 2: Update `internal/combat/context.md`**

In the Files table, change the `combat_helpers.go` row's description to note the
delegation, and add this line to the same table:

```markdown
| `combat_helpers.go` | Extracted helpers. **`runBestOfAllDefense` no longer rolls — it builds defence scores and delegates to `internal/contest` (U1). It performs the one sign conversion between the core's attack-positive margin and `bestDefenseResult`'s defence-positive one.** |
```

- [ ] **Step 3: Verify the docs describe real symbols**

Run: `python tools/context_md_audit.py 2>&1 | grep -i contest`

Expected: no phantom symbols reported for `internal/contest`.

- [ ] **Step 4: Commit**

```bash
git add internal/contest/context.md internal/combat/context.md
git commit -m "docs(contest): package context for the contest core (U1)

Records the two traps that compile cleanly if got wrong: the margin sign
is attack-positive and the opposite of bestDefenseResult, and Contested
rather than Winner is what says a contest happened.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Self-review notes

- **Spec coverage.** U1's roadmap row calls for three things: generalise
  `runBestOfAllDefense` (Task 3), support contest-vs-static-difficulty (Task 2),
  and normalise the margin sign at the seam (Task 1 defines the convention, Task
  3 converts once at melee's boundary). Melee migration is Task 3.
- **Deliberately NOT in U1.** No behaviour change, no other channel migrated, no
  cost or harm work, no uniform skill weight. Those are U2–U6.
- **The riskiest step is Task 3.** Its whole value is being a no-op, so the
  check that matters is that no existing test needed editing. If one does, the
  migration moved a modifier — find which, do not adjust the test.
