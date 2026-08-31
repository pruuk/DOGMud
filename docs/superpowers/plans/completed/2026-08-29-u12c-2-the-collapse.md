# U12c-2 — The Collapse Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete `Character.Aggro` and everything that only exists to serve it, leaving `CombatPhase` as the single store for "am I fighting, and who".

**Architecture:** Nothing new is built. `internal/targeting` (U12a) and the accessors (U12c-1) already exist and every mechanical read is already on them. This slice closes the two writers still off the seam, moves the four jobs `AggroType` was doing to the machines that already model them, migrates 524 test-fixture reads, and then deletes the field. It is the last code slice of the arc.

**Tech Stack:** Go, `testify/assert` + `require`, `go/ast` for the guards, the playtest harness for the closing gate.

**Spec:** [`2026-08-29-u12-unified-targeting-design`](../specs/2026-08-29-u12-unified-targeting-design.md) §6.3

**Branch:** `feature/u12c-2-the-collapse`, off `master` at `c040786d3` or later.

---

## 0. Facts verified against source

Read at master `c040786d3` on 2026-08-29, immediately after U12c-1 merged (PR #86).

| Fact | Value |
|---|---|
| `Aggro` field declaration | `character.go:150` — **`yaml:"-"`, so it is NOT persisted.** Deleting it is not a save-format change and needs no migration |
| `Aggro` struct | `combat_state_compat.go:36` — `Type`, `MobInstanceId`, `UserId`, `SpellInfo`, `ExitName`, `RoundsWaiting` |
| `SpellAggroInfo` | `combat_state_compat.go:25` — `SpellId`, `SpellRest`, `TargetUserIds`, `TargetMobInstanceIds` |
| `activity.CastingData` | `activity/activity.go:45` — carries all four of the above **plus** `Reason`, `FoldsNeeded`, `FoldsAccumulated`, `FoldsPerRound`, `TotalConvictionCost`, `ConvictionSpent`. **Strict superset confirmed** |
| `combat_state_compat.go` | **221 lines** |
| **Production `.Aggro` reads left after U12c-1** | **`.Type` 13 · `.RoundsWaiting` 8 · `.SpellInfo` 3 · nil-guards protecting those** |
| **`.Aggro` occurrences in TEST files** | 🔴 **524, across 87 files** — the dominant cost of this slice |
| …of those, **writes** `\.Aggro = ` | **106**, of which **94 are `= nil`** |
| …reads | `.UserId` 45 · `.RoundsWaiting` 38 · `== nil` 29 · `.MobInstanceId` 28 · `!= nil` 22 · `.Type` 13 |
| Test files using only `SetAggro`/`EndAggro` (these SURVIVE, step 7) | 24 |
| `RoundsWaiting` non-test sites | **34** — 20 special-move `= 1` writes, seeded in `SetAggro:158` and `SetCast:212`, decremented once in `NewRound_DoCombat_resolution.go:46` |
| `EngagingData.Reason` | `combatphase.go:41` — **ZERO readers anywhere.** Confirmed dead |
| `EngagingData{}` construction sites | **1** production: `combat_state_compat.go:129` |
| `ConsumeOpeningStrike` | `targeting/engagement.go:139` — **still zero production callers** |
| `openingStrikeLeft` | local in `calculateCombat`, `combat.go:406` |
| `TODO Task 18` | `NewRound_DoCombat_helpers.go:` **906** — ⚠️ the spec says 904; it moved in U12c-1 |
| `Machine` struct | `combatphase.go:77` — `inner`, `self`, `engaging`, `engaged`, `disengaging`, `attackers`, `attackersChangeListeners`, `vetoes`, `tickEventListeners` |
| `OnRoundTick` | `combatphase.go:288` — `Engaging` decrements `RoundsUntil`; **`Engaged` is a deliberate no-op** |
| `Character.IsCasting()` | `spells.go:83` — exists, nil-guards `c.Activity` |
| `CastingData()` | `activity/activity.go:112`, on the **Activity machine**, returns `(CastingData, bool)`. There is **no** `Character.CastingData()` today |

### 0.1 Two off-seam writers U12b missed. Both BLOCK the deletion.

U12b was contracted to land all 90 write sites on the seam. Two did not land, and
each one is load-bearing for a different deliverable below. **Find them first;
neither is optional.**

1. 🔴 **`hooks/NewRound_DoCombat_unified.go:929`** constructs a raw
   `&characters.Aggro{Type: characters.DefaultAttack}` for an MvM defender. It
   sets a **targetless** engagement, which `targeting.Commit` with a zero ref
   does not reproduce — `Commit` refuses a zero ref outright. **The field
   cannot be deleted while this stands.** Task 1 owns it.
2. 🔴 **`characters.SetCast`** (`spells.go:208`) assigns `c.Aggro` directly and
   **never touches `CombatPhase`**, so calling it over a live engagement leaves
   the two stores disagreeing. Pinned by
   `TestAccessors_KnownDisagreement_SetCastOverALiveEngagement`. Its only
   production caller is `mobcommands/aid.go:81`. Task 4 owns it.

### 0.2 What `SetCast` actually is, before you design around it

Verified 2026-08-29, because the spec's "SpellCast→`activity.Casting`" line
reads as if it were a live casting path. It is not:

- `SetCast` has **one** production caller, `mobcommands/aid.go:81`.
- `Aid` requires `room.IsCalm()` and a target at `Health <= 0`. It is a **heal
  on a downed player**, not an offensive cast.
- **Nothing resolves a `SetCast` aggro into a spell effect.** `Aggro.SpellInfo`
  is read in exactly three places: `IsAggro`, `targeting.EngagementOf`, and
  `Death_InboundAggroCleanup`. None applies a spell.
- Real offensive casting does **not** use this path at all. It commits an
  ordinary engagement at `mobcommands/cast.go:184` and tracks the cast in
  `Activity`.

So `SpellCast` is a vestigial fifth `AggroType` serving one healing command.
Task 4 is smaller than the spec implies, but must not be skipped: the
`Death_InboundAggroCleanup` read is real and cancels an in-flight aid when its
target dies.

### 0.3 Deliberately NOT in this slice

- **Unifying `RoundsWaiting` with `RoundsUntil`.** Spec §6.3.1 defers it with a
  written reason: the two decrement at different moments in the round, so
  collapsing them shortens every wind-up by one round unless compensated. It is
  a balance change wearing a refactor's clothes. **Its own post-arc slice.**
- **Adopting `IsAggro()` in `hooks/combat_retarget.go`.** U12c-1 declined it.
  `IsAggro` also matches the SpellCast target lists, which those two scans do
  not consider — but note the scans never see a caster anyway, because
  `FindFighting` (`rooms.go:1508`) filters on a non-zero id and a SpellCast
  aggro has both zero. Adopting it is therefore a **no-op today** and a
  behaviour widening the day `SpellCast` means anything. Leave it.

---

## Task 1: Close the raw `Aggro` construction in the MvM defender path

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_unified.go` (~line 925)
- Create: `internal/characters/aggro_construction_guard_test.go`

The MvM branch of `handleAggroAndAssist` sets a targetless placeholder and then
issues `attack #<id>`, which commits the real target a moment later. The PvM
branch immediately above issues its command with **no** placeholder. Task 1
makes MvM match PvM.

- [ ] **Step 1: Write the failing guard**

A construction guard, not a fixture test: the invariant is "nothing outside
`internal/characters` builds an `Aggro`", which is exactly what blocks Task 9.
An AST walk states that directly and cannot be satisfied by accident.

Create `internal/characters/aggro_construction_guard_test.go`:

```go
package characters_test

// U12c-2 Task 1. Nothing outside internal/characters may CONSTRUCT an Aggro.
//
// U12b was contracted to land all 90 write sites on the seam and this one did
// not land: hooks/NewRound_DoCombat_unified.go built a targetless
// &characters.Aggro{Type: DefaultAttack} for an MvM defender. targeting.Commit
// refuses a zero ref, so no seam call reproduces it -- which is why U12c-1 left
// it alone rather than smuggle a behaviour change into a mechanical slice.
//
// It blocks deleting the field, so it is the first thing U12c-2 removes.

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

func TestNothingOutsideCharactersConstructsAnAggro(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	internalDir := filepath.Dir(filepath.Dir(thisFile))

	var offenders []string
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
		if strings.HasPrefix(rel, "characters/") {
			return nil
		}

		fset := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := lit.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Aggro" {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "characters" {
				offenders = append(offenders, fset.Position(lit.Pos()).String())
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)

	sort.Strings(offenders)
	assert.Empty(t, offenders,
		"these sites construct a characters.Aggro directly. Use the "+
			"targeting seam: %s", strings.Join(offenders, ", "))
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/characters/ -run TestNothingOutsideCharactersConstructsAnAggro -v`
Expected: FAIL, naming `hooks/NewRound_DoCombat_unified.go` at roughly line 929.

If it reports nothing, the guard is broken, not the codebase — a walk that
matches no files and a genuinely clean tree both return empty. Confirm the
search could have succeeded before believing it.

- [ ] **Step 3: Read the site and its PvM sibling**

Run:
```bash
sed -n '870,935p' internal/hooks/NewRound_DoCombat_unified.go
```

Confirm the PvM branch (`atk.IsPlayer() && !def.IsPlayer()`) issues
`defMob.Command("attack @<id>")` with no placeholder, and the MvM branch
(`default:`) issues `defMob.Command("attack #<id>")` **with** one.

- [ ] **Step 4: Delete the placeholder**

Replace:

```go
		defMob := asMob(def)
		if !defChar.IsInCombat() {
			// U12c-1: a raw Aggro construction, deliberately LEFT as-is. It
			// sets a targetless placeholder that no seam call reproduces —
			// targeting.Commit with a zero ref is not the same thing — so
			// replacing it here would be a behaviour change in a mechanical
			// slice. U12c-2 owns it, and must resolve it before the field
			// can be deleted.
			defChar.Aggro = &characters.Aggro{
				Type: characters.DefaultAttack,
			}
			defMob.Command(fmt.Sprintf("attack #%d", atk.GetMobInstanceId()))
		}
```

with:

```go
		defMob := asMob(def)
		if !defChar.IsInCombat() {
			// U12c-2: the targetless &Aggro{} placeholder that used to sit
			// here is gone. It was the last production construction of an
			// Aggro outside internal/characters, and no seam call reproduces
			// it (targeting.Commit refuses a zero ref). It was never
			// load-bearing: the PvM branch above issues the same kind of
			// command with no placeholder, and the queued attack command is
			// what actually commits the engagement in both.
			defMob.Command(fmt.Sprintf("attack #%d", atk.GetMobInstanceId()))
		}
```

- [ ] **Step 5: Verify**

Run: `gofmt -l internal/ && go build ./... && go test ./internal/characters/ ./internal/hooks/ -count=1`
Expected: all pass. Watch specifically for a now-unused `characters` import in
`NewRound_DoCombat_unified.go`; the compiler will name it.

- [ ] **Step 6: Confirm nothing else constructs an Aggro**

Run:
```bash
grep -rn "&Aggro{\|&characters\.Aggro{" --include=*.go internal/ modules/ | grep -v "_test\.go"
```
Expected: only `internal/characters/combat_state_compat.go` (`SetAggro`) and
`internal/characters/spells.go` (`SetCast`). Task 4 takes the second.

- [ ] **Step 7: Commit**

```bash
git add internal/hooks/NewRound_DoCombat_unified.go internal/characters/aggro_construction_guard_test.go
git commit -m "refactor(hooks): drop the raw Aggro placeholder in the MvM defender path"
```

---

## Task 2: Move `RoundsWaiting` onto the combat phase machine

**Files:**
- Modify: `internal/state/combatphase/combatphase.go`
- Modify: `internal/characters/combat_state_compat.go:158`
- Modify: `internal/hooks/NewRound_DoCombat_resolution.go:42-46`
- Modify: `internal/hooks/NewRound_DoCombat_unified.go:284`
- Modify: the 20 special-move `= 1` writes (see Step 5)
- Modify: `internal/users/userrecord.prompt.go:707-708`
- Test: `internal/state/combatphase/rounds_waiting_test.go` (create)

It becomes a **Machine-level field**, not per-state data: the ~20 special-move
writes need it to survive `Engaging → Engaged`, and per-state data does not.

- [ ] **Step 1: Write the failing test**

```go
package combatphase

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundsWaiting_SurvivesEngagingToEngaged(t *testing.T) {
	m := NewMachine()
	target := state.ActorRef{MobInstanceId: 100}

	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: target, RoundsUntil: 1},
		state.TransitionReason{Trigger: TriggerAttackCommand}))

	m.SetRoundsWaiting(3)
	assert.Equal(t, 3, m.RoundsWaiting())

	m.OnRoundTick() // RoundsUntil 1 -> 0, advances to Engaged
	require.Equal(t, Engaged, m.State())

	assert.Equal(t, 3, m.RoundsWaiting(),
		"RoundsWaiting is the ACTOR's round budget and must outlive the "+
			"wind-up; the ~20 special-move writes set it while Engaged")
}

func TestRoundsWaiting_ClearedOnIdle(t *testing.T) {
	m := NewMachine()
	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: state.ActorRef{MobInstanceId: 100}, RoundsUntil: 0},
		state.TransitionReason{Trigger: TriggerAttackCommand}))
	m.SetRoundsWaiting(5)

	m.ForceIdle(state.TransitionReason{Trigger: TriggerForceIdle})

	assert.Equal(t, Idle, m.State())
	assert.Zero(t, m.RoundsWaiting(),
		"EndAggro used to nil the whole Aggro struct, so the counter died "+
			"with the engagement; Idle must preserve that exactly")
}

func TestRoundsWaiting_DecrementStopsAtZero(t *testing.T) {
	m := NewMachine()
	m.SetRoundsWaiting(1)
	assert.True(t, m.ConsumeRoundWaiting(), "1 -> 0 consumes the round")
	assert.Zero(t, m.RoundsWaiting())
	assert.False(t, m.ConsumeRoundWaiting(), "already zero: nothing to consume")
	assert.Zero(t, m.RoundsWaiting(), "and it must not go negative")
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/state/combatphase/ -run TestRoundsWaiting -v`
Expected: FAIL to compile — `SetRoundsWaiting`, `RoundsWaiting`,
`ConsumeRoundWaiting` undefined.

- [ ] **Step 3: Add the field, the accessors, and the REQUIRED comment block**

The comment block is a **deliverable of this slice, not a nicety** (spec
§6.3.1). Add to `internal/state/combatphase/combatphase.go`, immediately above
the `Machine` struct:

```go
// TWO round counters live on this machine, and they are NOT the same thing.
// Nothing said so before U12c-2, so each one read like the only one.
//
//   RoundsUntil    (EngagingData.RoundsUntil) is the ENGAGEMENT WIND-UP: how
//                  many rounds before the engagement becomes active.
//                  OnRoundTick decrements it and calls advanceToEngaged() at
//                  zero, which is also what fires the mob_engaged
//                  behaviour-tree event. It exists ONLY in Engaging.
//
//   roundsWaiting  (this field) is the ACTOR'S ROUND BUDGET: how many rounds
//                  before this actor may act again.
//                  handleCombatWaitRound decrements it LATER IN THE SAME
//                  ROUND, and emits the wait messages.
//
// They are seeded identically by the commit path, so during wind-up they march
// in lockstep. That is a coincidence of seeding, not shared identity.
//
// They diverge in Engaged ON PURPOSE: RoundsUntil does not exist there, while
// the ~20 special-move `= 1` writes need a counter that still works once
// engaged. That is why roundsWaiting is a MACHINE field and not state data.
//
// OnRoundTick's Engaged branch is a DELIBERATE no-op. Making it decrement is
// the first step of unifying the two counters, not a bug fix.
//
// ⚠️ Unifying them is DEFERRED with a written reason (spec §6.3.1): the two
// decrements happen at different moments in the round, so one counter shortens
// every weapon wind-up and every special-move recovery by one round unless
// compensated by seeding 2 where the code says 1. That is a balance change
// wearing a refactor's clothes. It is its own post-arc slice, and it must also
// relocate advanceToEngaged() and verify mob_engaged still fires at the same
// point.
```

Add the field to `Machine`:

```go
	tickEventListeners       []func(name string, r state.TransitionReason)
	roundsWaiting            int // see the two-counter note above
```

Add the three methods:

```go
// RoundsWaiting reports the actor's remaining round budget.
func (m *Machine) RoundsWaiting() int { return m.roundsWaiting }

// SetRoundsWaiting sets the actor's round budget. Negative values are clamped
// to zero; every caller means "wait at least this long", never "act early".
func (m *Machine) SetRoundsWaiting(n int) {
	if n < 0 {
		n = 0
	}
	m.roundsWaiting = n
}

// ConsumeRoundWaiting decrements the budget by one and reports whether this
// round was consumed by the wait. False means the actor is free to act.
//
// Replaces the `if Aggro.RoundsWaiting <= 0 { return false }; RoundsWaiting--`
// pair in handleCombatWaitRound, so the guard and the decrement can no longer
// drift apart.
func (m *Machine) ConsumeRoundWaiting() bool {
	if m.roundsWaiting <= 0 {
		return false
	}
	m.roundsWaiting--
	return true
}
```

Clear it in `ForceIdle`. Find the method and add `m.roundsWaiting = 0` beside
the existing per-state data clears, so Idle wipes it exactly as `EndAggro`
nilling the struct used to.

- [ ] **Step 4: Run the test**

Run: `go test ./internal/state/combatphase/ -run TestRoundsWaiting -v`
Expected: PASS, three tests.

- [ ] **Step 5: Move every production site**

Seeding, `combat_state_compat.go` — inside the `if c.CombatPhase != nil` block
after the successful transition, add:

```go
		c.CombatPhase.SetRoundsWaiting(combatAddlWaitRounds)
```

Leave the `Aggro` write alone for now; Task 9 deletes it.

The consumer, `internal/hooks/NewRound_DoCombat_resolution.go:42-46`, becomes:

```go
	if attackerChar.CombatPhase == nil || !attackerChar.CombatPhase.ConsumeRoundWaiting() {
		return false
	}
	mudlog.Debug(`RoundsWaiting`, `User`, attackerChar.Name,
		`Rounds`, attackerChar.CombatPhase.RoundsWaiting())
```

⚠️ The debug line now logs the value **after** the decrement, where it used to
log before. Say so in the commit; it is a log-only change but a reviewer will
otherwise read it as an off-by-one.

The gate at `NewRound_DoCombat_unified.go:284` becomes:

```go
	if atkChar.CombatPhase == nil || atkChar.CombatPhase.RoundsWaiting() <= 0 {
		return false
	}
```

The 20 special-move writes. Each is the identical two-line shape:

```go
	if char.Aggro != nil {
		char.Aggro.RoundsWaiting = 1
	}
```

becomes:

```go
	if char.CombatPhase != nil {
		char.CombatPhase.SetRoundsWaiting(1)
	}
```

Enumerate them with:
```bash
grep -rn "Aggro.RoundsWaiting = 1" --include=*.go internal/ | grep -v "_test\.go"
```
Expected: `actions/` combat_drain, combat_gore, combat_hamstring, combat_helpers,
combat_maul, combat_pounce, combat_rake, combat_rally, combat_taunt (×2),
combat_throttle, combat_warcry; plus `usercommands/` stand, target, throw.

The prompt token, `internal/users/userrecord.prompt.go:707-708`:

```go
				if u.Character.CombatPhase != nil {
					promptOut.WriteString(strconv.Itoa(u.Character.CombatPhase.RoundsWaiting()))
```

- [ ] **Step 6: Verify**

Run:
```bash
gofmt -l internal/ && go build ./... && go test ./internal/... 2>&1 | grep -vE "^ok|no test files"
```
Expected: no output.

Then confirm the field has no production readers left:
```bash
grep -rn "Aggro\.RoundsWaiting" --include=*.go internal/ | grep -v "_test\.go"
```
Expected: nothing.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor(combatphase): RoundsWaiting onto the machine, with the two-counter note"
```

---

## Task 3: Dissolve `AggroType.Flee` into `Disengaging`

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:905-912, 935-938`
- Modify: `internal/hooks/combat_retarget.go:28-33`

This finishes the standing `// TODO Task 18` at
`NewRound_DoCombat_helpers.go:906`. ⚠️ The spec cites line 904; it moved to 906
in U12c-1.

- [ ] **Step 1: Read the two sites**

Run: `sed -n '900,945p' internal/hooks/NewRound_DoCombat_helpers.go`

`handlePlayerFlee` currently ORs `IsDisengaging()` with a legacy
`Aggro.Type == Flee` sentinel, and then reverts the type on the legacy path
only.

- [ ] **Step 2: Delete the legacy arm**

```go
	// U12c-2: the Aggro.Type == Flee sentinel is gone. IsDisengaging() reads
	// CombatPhase.State() == Disengaging, set by TransitionToDisengaging in
	// flee.go, and is now the only way a flee is expressed. This finishes the
	// TODO that stood here from Task 18.
	isFleeing := user.Character.IsDisengaging()
	if !isFleeing {
```

Delete the `phaseFleeing` local and every use of it; with the sentinel gone,
`phaseFleeing` and `isFleeing` are the same value. The admission block that read
`if phaseFleeing` becomes unconditional:

```go
	// Consume admission before any resolution branch. A flee can only come
	// from the command now, so missing admission means another/reentrant
	// resolver already owns it.
	includeSkill, admitted := usercommands.TakeFleeAdmission(user)
	if !admitted {
		return true
	}
```

⚠️ This is a **behaviour change and it is the intended one.** The legacy branch
granted `includeSkill = true` without consuming admission. Nothing can reach
that branch any more, because `TransitionToDisengaging` is the only way to
become `Disengaging` and it always goes through the command. Say this in the
commit message.

Then delete the legacy revert block at 935-938 entirely — it existed only to
undo the sentinel.

- [ ] **Step 3: Drop `Flee` from the ValidateAggro no-target exemption**

In `internal/hooks/combat_retarget.go`, the guard reads:

```go
	if ref.IsZero() &&
		char.Aggro.Type != characters.SpellCast &&
		char.Aggro.Type != characters.Flee {
```

`Disengaging` legitimately has no plain target. Replace the `Flee` half with the
state:

```go
	// SpellCast and Disengaging intentionally have no target — they act on
	// self or the room, not another character — so they are valid no-target
	// states. (The Flee AggroType became Disengaging in U12c-2; SpellCast
	// follows in Task 4.)
	if ref.IsZero() && !char.IsDisengaging() &&
		char.Aggro.Type != characters.SpellCast {
```

- [ ] **Step 4: Verify**

Run: `go build ./... && go test ./internal/hooks/ ./internal/usercommands/ ./internal/combat/ -count=1`
Expected: PASS. Flee tests are the ones to watch; if one fails it is asserting
on the sentinel and should be rewritten to drive `TransitionToDisengaging`, not
deleted.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(hooks): Flee aggro type becomes Disengaging, closing the Task 18 TODO"
```

---

## Task 4: Dissolve `AggroType.SpellCast`, and put `SetCast` on the seam

**Files:**
- Modify: `internal/characters/spells.go:208` (`SetCast`)
- Modify: `internal/hooks/Death_InboundAggroCleanup.go:52-59`
- Modify: `internal/characters/combat_state_compat.go:201-217` (`IsAggro`)
- Modify: `internal/targeting/engagement.go:104-113` (`EngagementOf`)
- Modify: `internal/hooks/combat_retarget.go` (the exemption from Task 3)
- Test: `internal/characters/accessor_equivalence_test.go` (invert the pinned disagreement)

Read §0.2 before starting. `SpellCast` serves ONE healing command and nothing
resolves it.

- [ ] **Step 1: Make `SetCast` write the Activity machine**

`activity.CastingData` is a strict superset of `SpellAggroInfo` (§0 table), so
the payload moves with no loss. `SetCast` becomes a thin wrapper that records
the cast where every other cast is already recorded, and stops writing `Aggro`.

Read the existing casting entry point first — do not invent one:

```bash
grep -rn "func (c \*Character)" internal/characters/spells.go
grep -rn "CastingData{" --include=*.go internal/ | grep -v "_test\.go"
```

Follow whatever `mobcommands/cast.go` already does to start a cast, and make
`SetCast` use the same call. `SetCast` keeps its name and signature so
`aid.go` does not change.

- [ ] **Step 2: Move the `Death_InboundAggroCleanup` read**

The one real consumer. It cancels an in-flight aid when its target dies:

```go
					// Spell-cast-shape aggro: abort an in-flight cast
					// targeting this player. U12c-2: reads the Activity
					// machine's CastingData, which carries the same target
					// lists SpellAggroInfo did.
					if cd, ok := mob.Character.CastingData(); ok {
						for _, tid := range cd.TargetUserIds {
							if tid == c.GetUserId() {
								targeting.Release(&mob.Character, targeting.ReasonDisengage)
								break
							}
						}
					}
```

⚠️ **Verified 2026-08-29:** `CastingData()` lives on the **Activity machine**
(`activity/activity.go:112`, returning `(CastingData, bool)`), NOT on
`Character`. `Character` exposes only `IsCasting()` (`spells.go:83`). So the
read is `mob.Character.Activity != nil` then
`mob.Character.Activity.CastingData()` — or add a `Character.CastingData()`
wrapper mirroring `Character.IsCasting()`'s nil-guard shape, which is the
tidier option given three call sites need it.

- [ ] **Step 3: Move the `IsAggro` and `EngagementOf` branches**

Both read `Aggro.Type == SpellCast` then `Aggro.SpellInfo`. Both become reads of
the Activity machine's casting data. `Engagement.SpellTargets` keeps its name,
its type and its contract; only its source moves.

- [ ] **Step 4: Invert the pinned disagreement test**

`TestAccessors_KnownDisagreement_SetCastOverALiveEngagement` exists precisely to
fail when this task lands. Its comment says so. Rewrite it as an equivalence
assertion — `SetCast` over a live engagement must now leave both stores agreeing
— and rename it to `TestAccessors_AgreeAfterSetCastOverALiveEngagement`.
**Do not delete it.**

- [ ] **Step 5: Verify**

Run: `go build ./... && go test ./internal/characters/ ./internal/hooks/ ./internal/targeting/ ./internal/mobcommands/ -count=1`
Expected: PASS, with the renamed test now asserting agreement.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(characters): SetCast onto the Activity machine, SpellCast aggro type dissolved"
```

---

## Task 5: Dissolve `AggroType.Shooting`

**Files:**
- Modify: `internal/usercommands/attack.go:130`
- Modify: `internal/usercommands/target.go:50, 154, 181`
- Modify: `internal/characters/combat_state_compat.go:108-112`

`Shooting` is already derived once, at commit time, from the weapon subtype
(`combat_state_compat.go:109`). `targeting.Engagement` derives the same fact
live: `e.Ranged = c.Equipment.Weapon.GetSpec().Subtype == items.Shooting`
(`engagement.go`). Stored state and derived state cannot disagree if only one
exists.

- [ ] **Step 1: Read the three consumer sites**

Run:
```bash
sed -n '125,135p' internal/usercommands/attack.go
sed -n '45,55p;150,190p' internal/usercommands/target.go
```

All three ask the same question: "is this an ordinary attack engagement, as
opposed to a cast or a flee?"

- [ ] **Step 2: Replace with the derived question**

`attack.go:130` and `target.go:50` both test
`Type == DefaultAttack || Type == Shooting`. With `Flee` now `Disengaging` and
`SpellCast` now `Activity`, that pair is simply "not fleeing and not casting":

```go
	// U12c-2: DefaultAttack and Shooting were the two ordinary-attack aggro
	// types; Shooting was only ever derived from the weapon subtype at commit
	// time, and Engagement.Ranged derives it live. With Flee and SpellCast
	// dissolved into Disengaging and Activity, "an ordinary attack" is exactly
	// "engaged, not fleeing, not casting".
	if user.Character.IsInCombat() && !user.Character.IsDisengaging() &&
		!user.Character.IsCasting() {
```

✅ **Verified 2026-08-29:** `Character.IsCasting()` exists at `spells.go:83`
and already nil-guards `c.Activity`, so the form above is correct as written.

`target.go:154` and `:181` hoist `aggroType` to hand to
`targeting.ReasonForAggroType`. Since `Commit` re-derives `Shooting` itself,
pass `targeting.ReasonAttack` and delete the hoist.

- [ ] **Step 3: Delete the derivation in `SetAggro`**

Remove:
```go
	if aggroType == DefaultAttack {
		if c.Equipment.Weapon.GetSpec().Subtype == items.Shooting {
			aggroType = Shooting
		}
	}
```

- [ ] **Step 4: Verify**

Run: `go build ./... && go test ./internal/usercommands/ ./internal/characters/ ./internal/targeting/ -count=1`
Expected: PASS. Pay attention to any `shoot_test.go` case asserting on the
aggro type; rewrite it against `Engagement.Ranged`.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(usercommands): Shooting aggro type derived, not stored"
```

---

## Task 6: `openingStrikeLeft` becomes engagement state

**Files:**
- Modify: `internal/state/combatphase/combatphase.go`
- Modify: `internal/combat/combat.go:406-419, 480-481`
- Modify: `internal/targeting/engagement.go:139` (`ConsumeOpeningStrike`)

`ConsumeOpeningStrike` has had **zero production callers since U12a**, on
purpose — it was built waiting for this task. `EngagingData` already carries the
opening via `Engagement.OpeningUnspent`, derived today from
`Aggro.Type == SurpriseAttack`.

- [ ] **Step 1: Add the machine-side flag**

Add `openingUnspent bool` to `Machine` beside `roundsWaiting`, set it in
`TransitionToEngaging` when the trigger is `TriggerSurpriseAttack`, and clear it
on Idle. Expose:

```go
// OpeningUnspent reports whether this engagement still has its ambush opening.
func (m *Machine) OpeningUnspent() bool { return m.openingUnspent }

// SpendOpening consumes the ambush opening and reports whether it was there to
// spend. Exactly ONE caller: the swing loop, on the swing that is THROWN.
func (m *Machine) SpendOpening() bool {
	if !m.openingUnspent {
		return false
	}
	m.openingUnspent = false
	return true
}
```

- [ ] **Step 2: Point `EngagementOf` and `ConsumeOpeningStrike` at it**

`EngagementOf` reads `c.CombatPhase.OpeningUnspent()` instead of
`c.Aggro.Type == characters.SurpriseAttack`. `ConsumeOpeningStrike` calls
`SpendOpening()`.

- [ ] **Step 3: Give `ConsumeOpeningStrike` its caller**

In `calculateCombat` (`combat.go:406`), replace:

```go
	openingStrikeLeft := false
	if sourceChar.Aggro.Type == characters.SurpriseAttack {
		openingStrikeLeft = true
		attackResult.WasSurpriseAttack = true
		targeting.Commit(sourceChar, sourceChar.CurrentCombatTarget(), targeting.ReasonAttack)
	}
```

with:

```go
	// U12c-2: the opening is engagement state now, and this is the production
	// caller ConsumeOpeningStrike was written for in U12a. The old form read
	// Aggro.Type and DEMOTED it in the same breath via a re-Commit -- which is
	// why U10d had to add AttackResult.WasSurpriseAttack to carry the fact
	// past the read. Splitting the query from the consumption removes the
	// demotion entirely.
	openingStrikeLeft := targeting.ConsumeOpeningStrike(sourceChar)
	attackResult.WasSurpriseAttack = openingStrikeLeft
```

⚠️ **`AttackResult.WasSurpriseAttack` STAYS.** `applyCombatProgression` runs
after this point and needs the fact carried on the result. Do not "simplify" it
away; U10d's spec §2.8.3 records why.

⚠️ The per-swing capture at `combat.go:480-481` is unchanged. The opening is
consumed once per ROUND here and handed to the ONE swing that is thrown — that
is U10d's contract and this task must not alter it.

- [ ] **Step 4: Verify against the U10d suite**

Run: `go test ./internal/combat/ -count=1`
Then the invariant that caught a real defect this arc:
```bash
go test ./internal/combat/ -run TestSurpriseRound_ExactlyOneSwingIsMarkedAsTheOpener -count=200
```
Expected: PASS, all 200. This test flakes on any regression to opener
consumption; a failure here means the round/swing split moved.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(combat): the ambush opening becomes engagement state"
```

---

## Task 7: Delete `EngagingData.Reason`

**Files:**
- Modify: `internal/state/combatphase/combatphase.go:41`

- [ ] **Step 1: Re-confirm it is dead**

Run:
```bash
grep -rn "EngagingData" --include=*.go internal/ modules/ | grep -v "_test\.go"
grep -rn "\.Reason\b" --include=*.go internal/state/combatphase/ internal/targeting/ | grep -v "_test\.go"
```
Expected: one production construction (`combat_state_compat.go:129`) which does
not set `Reason`, and zero readers.

- [ ] **Step 2: Delete the field**

```go
type EngagingData struct {
	Target      state.ActorRef
	RoundsUntil int // weapon WaitRounds before swing
}
```

⚠️ The `r state.TransitionReason` **parameter** on `TransitionToEngaging` is
LIVE — it reaches `m.inner.TransitionTo`. Do not touch it. And do **not**
repurpose either as a home for an engagement-kind enum: that moves the demotion
bug rather than killing it (spec §6.3.5).

- [ ] **Step 3: Verify and commit**

Run: `go build ./... && go test ./internal/... 2>&1 | grep -vE "^ok|no test files"`
Expected: no output.

```bash
git add -A
git commit -m "refactor(combatphase): delete the dead EngagingData.Reason field"
```

---

## Task 8: Migrate the test fixtures

**Files:**
- Modify: 87 test files, 524 `.Aggro` occurrences
- Test: `internal/characters/aggro_reader_guard_test.go` (extend to cover tests)

🔴 **This is the largest task in the slice and the roadmap's "Size: M" does not
account for it.** Budget accordingly. It is mechanical but it is not small.

- [ ] **Step 1: Enumerate by shape, not by file**

```bash
grep -rho "\.Aggro[.!= ]*[A-Za-z]*" --include=*_test.go internal/ | sed 's/[0-9]//g' | sort | uniq -c | sort -rn
```

Expected shapes and their replacements:

| Shape | Count | Becomes |
|---|---|---|
| `.Aggro = nil` | 94 | `c.EndAggro()` |
| `.Aggro = &characters.Aggro{...}` | ~12 | `c.SetAggro(userId, mobId, characters.DefaultAttack)` |
| `.Aggro.UserId` | 45 | `c.CurrentCombatTarget().UserId` |
| `.Aggro.RoundsWaiting` | 38 | `c.CombatPhase.RoundsWaiting()` |
| `.Aggro == nil` | 29 | `!c.IsInCombat()` |
| `.Aggro.MobInstanceId` | 28 | `c.CurrentCombatTarget().MobInstanceId` |
| `.Aggro != nil` | 22 | `c.IsInCombat()` |
| `.Aggro.Type` | 13 | per Tasks 3-6: `IsDisengaging()`, casting, `Engagement.Ranged`, `OpeningUnspent()` |

⚠️ **`.Aggro = nil` is NOT always `EndAggro()`.** Where a test nils the field to
assert a *nil-guard* rather than to reset combat state, `EndAggro()` also clears
taunt hold, grapple state and `RangedEngagedCueSpoken`. Read each one. Where the
test wants "not in combat", `EndAggro()` is right; where it wants "this specific
pointer is nil", the test is asserting on the field's existence and should be
deleted with it.

- [ ] **Step 2: Work file by file, largest first**

Order: `usercommands/usercommands_test.go` (46), `usercommands/shoot_test.go`
(30), `actions/rhetoric_progression_test.go` (30),
`mobcommands/predator_test.go` (29), `usercommands/throw_engage_test.go` (17),
`hooks/pinnacle_tick_test.go` (15), `characters/taunt_hold_test.go` (14), then
the tail.

Commit every 8-10 files. A single 87-file commit is unreviewable, and CI lint
goes **red on any PR over 300 files** (the diff API 406 kills `only-new-issues`),
so keep the whole slice under that.

- [ ] **Step 3: Handle `accessor_equivalence_test.go` last**

It reads `.Aggro` 14 times **on purpose** — it is the test that proves the
accessors match the field. Once the field is gone the test has nothing to
compare against. Do not migrate it: **delete it in Task 9**, and move its
`SetCast` case (renamed in Task 4) into
`internal/state/combatphase/` as a machine-level test first, so the coverage
survives the file.

- [ ] **Step 4: Extend the guard to see test files**

`aggro_reader_guard_test.go` currently skips `_test.go`. Flip that, so the
allowlist tracks the test migration the same way it tracked production:

```go
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
```

Populate the allowlist from the guard's own failure output, exactly as U12c-1
did — **not from a grep**, which sees comments the AST parser drops.

- [ ] **Step 5: Verify**

Run: `go test ./internal/... 2>&1 | grep -vE "^ok|no test files"`
Expected: no output, and the allowlist empty except `characters/` itself.

---

## Task 9: Delete the field

**Files:**
- Modify: `internal/characters/character.go:150, 746-751, 783-797`
- Delete: `internal/characters/combat_state_compat.go` (221 lines)
- Delete: `internal/characters/accessor_equivalence_test.go`
- Modify: `internal/characters/spells.go` (remove `SpellAggroInfo`)
- Modify: `internal/characters/aggro_reader_guard_test.go` (delete — the guard's job is done)

- [ ] **Step 1: Delete the fallback branches**

`IsInCombat` becomes:

```go
// IsInCombat returns true if the character is in any non-Idle combat state.
func (c *Character) IsInCombat() bool {
	return c.CombatPhase != nil && c.CombatPhase.IsInCombat()
}
```

`CurrentCombatTarget` becomes:

```go
// CurrentCombatTarget returns the current combat target across all non-Idle
// states (Engaging.Target, Engaged.Target, or Disengaging.LastTarget).
// Returns zero ActorRef when Idle.
func (c *Character) CurrentCombatTarget() state.ActorRef {
	if c.CombatPhase == nil {
		return state.ActorRef{}
	}
	return c.CombatPhase.CurrentTarget()
}
```

⚠️ **A nil `CombatPhase` now means "not in combat" with no second opinion.**
Before deleting, confirm every production path Validates the character:
```bash
grep -rn "CombatPhase = " --include=*.go internal/ | grep -v "_test\.go"
```

- [ ] **Step 2: Delete the field and the types**

Remove `Aggro *Aggro` from `character.go:150`. Move `SetAggro` and `EndAggro`
(which SURVIVE, per spec §6.3.7) into `character.go` or a new
`internal/characters/targeting_storage.go`, writing `CombatPhase` alone, then
delete `combat_state_compat.go` and `SpellAggroInfo`.

⚠️ **`SetAggro`/`EndAggro` are NOT deleted.** Spec §5 said they were and the
spec was wrong; the correction is recorded in §6.3.7. They are the storage
primitives. The enforced rule is a **caller** restriction: everything outside
`internal/characters` and `internal/targeting` goes through the seam.

- [ ] **Step 3: Delete the guards that have done their job**

`aggro_reader_guard_test.go` and `accessor_equivalence_test.go` both exist to
police a field that no longer exists. Delete both. The U12b **write** guard
stays — it still enforces the caller restriction.

- [ ] **Step 4: Verify**

```bash
gofmt -l internal/ modules/ && go build ./... && go test ./internal/... 2>&1 | grep -vE "^ok|no test files"
grep -rn "\bAggro\b" --include=*.go internal/ modules/ | grep -v "SetAggro\|EndAggro\|IsAggro\|AggroTarget\|AggroPulled\|NoAggroTarget\|InboundAggro\|ValidateAggro\|clearAggroIfTargetAbsent"
```
Expected: no output from either.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(characters): delete Aggro, AggroType, SpellAggroInfo and combat_state_compat.go"
```

---

## Task 10: Verify, patch notes, boot, adversarial playtest, PR

🔴 **This slice owns the arc's adversarial playtest** (spec §6.3, done-when 8,
and the content SOP). U11 is the arc's closing gate and no code slice may land
after it, so U12c-2's playtest cannot be deferred into U11.

- [ ] **Step 1: Full local gate**

```bash
gofmt -l internal/ modules/ main.go && go build ./... && go test ./internal/...
```

- [ ] **Step 2: Patch notes**

Add a dated entry to `docs/PATCH_NOTES.md`. Player-facing framing, 80-column
wrap, no raw numbers, **no en or em dashes**. ⚠️ The file is **CRLF**; an edit
that assumes `\n` will fail on it.

Unlike U12a/U12b/U12c-1, this slice **does** change behaviour: flee admission is
now consumed on every flee, and the ambush opening is consumed through a
different seam. Say what a player would notice, and do not claim "nothing
changes".

- [ ] **Step 3: Boot test in an isolated detached worktree**

Exit code 124 is the SUCCESS case. Do not grep for the bare word `panic`: the
config key `GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Clean up with `git worktree remove --force`; if Windows holds a lock, `rm -rf`
then `git worktree prune`.

- [ ] **Step 4: Adversarial playtest**

```text
/playtest local --checkout <abs> bug-finder 2026-08-03-prepush-sweep.yaml
```

Drive the real player flow end to end. The behaviours this slice touched, all of
which need a mob that **survives more than one round**:

- engage, switch targets mid wind-up, confirm the prompt follows
- flee successfully, and fail a flee, confirming you are put back in combat
- an ambush from hiding: exactly ONE opener line per round
- a special move, confirming the round budget still costs a round
- a companion casting `aid` on a downed player in a calm room
- an MvM fight (two mobs) reaching a defender that had no prior engagement

- [ ] **Step 5: PR**

```bash
git push -u origin feature/u12c-2-the-collapse
gh pr create --repo pruuk/DOGMud --base master --head feature/u12c-2-the-collapse --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

⚠️ `gh pr checks --watch` can return on **stale** runs. Confirm the run's
`headSha` matches HEAD before believing it:
```bash
gh run list --repo pruuk/DOGMud --branch feature/u12c-2-the-collapse --limit 5 \
  --json databaseId,headSha,status,conclusion,workflowName \
  --jq '.[] | "\(.workflowName) \(.status)/\(.conclusion) sha=\(.headSha[0:9])"'
```

- [ ] **Step 6: After merge, watch `Build and release` on master**

PR #83 was green on the PR and master went red.

---

## Done when

1. `grep -rn "\bAggro\b"` over `internal/` and `modules/` returns only
   `SetAggro`, `EndAggro`, `IsAggro`, `ValidateAggro` and the unrelated
   `AggroTarget` / `AggroPulled` / `NoAggroTarget` identifiers.
2. `Character.Aggro`, `AggroType`, `SpellAggroInfo` and
   `combat_state_compat.go` are gone; `SetAggro` and `EndAggro` survive as
   storage primitives writing `CombatPhase` alone.
3. `RoundsWaiting` lives on the combat phase machine, is cleared on Idle, and
   the two-counter comment block states all five facts from spec §6.3.1.
4. `ConsumeOpeningStrike` has exactly one production caller.
5. `EngagingData.Reason` is deleted; the `TransitionReason` parameter is not.
6. `go test ./internal/...` green, boot clean, PR green, **and master green
   after merge**.
7. The adversarial playtest ran, its findings are fixed or recorded, and its
   report's findings are extracted to memory (playtest reports are gitignored).
8. `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`'s U12 row is marked done with
   merge evidence, leaving **U11 as the only open row in the arc**.

## Do not

- Unify `RoundsWaiting` with `RoundsUntil`. Deferred with a written reason.
- Adopt `IsAggro()` in `combat_retarget.go`. See §0.3.
- Delete `AttackResult.WasSurpriseAttack`. Progression reads it after the
  consumption point.
- Delete `SetAggro`/`EndAggro`. The spec's §5 said to; the spec was wrong.
- Repurpose `EngagingData.Reason` or the `TransitionReason` parameter as an
  engagement-kind enum.
