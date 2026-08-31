# U12a — Targeting Seam Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `internal/targeting` and prove its API on four real call sites, two on each side of the player/mob divide, without changing any observable behaviour.

**Architecture:** One new package holding five members: `Select` (who to fight), `Commit` / `Release` (enter and leave combat), `EngagementOf` (a pure query composing the authoritative sources) and `ConsumeOpeningStrike` (the one deliberate side effect). In this slice `Commit` and `Release` **delegate to the existing `SetAggro` / `EndAggro`**, so the dual-write and every guard are untouched and behaviour is provably unchanged. The remaining ~86 write sites keep calling the old methods until U12b.

**Tech Stack:** Go, `testify/assert` + `testify/require`, existing `internal/state` machines (`combatphase`, `activity`), behaviour trees driven from YAML via `actionRegistry`.

**Spec:** [`2026-08-29-u12-unified-targeting-design`](../specs/2026-08-29-u12-unified-targeting-design.md)

---

## 0. Facts verified against source

Read from source at HEAD `f07edb248` on 2026-08-29. Re-verify anything you are about to depend on; this table is evidence, not memory.

| Fact | Value |
|---|---|
| `state.ActorRef` | `struct { UserId int; MobInstanceId int }`, `internal/state/transition.go`; methods `IsZero()`, `IsPlayer()`, `IsMob()` at `:13`, `:18`, `:23` |
| `SetAggro` | `func (c *Character) SetAggro(userId int, mobInstanceId int, aggroType AggroType, roundsWaitTime ...int)` — `internal/characters/combat_state_compat.go:82` |
| `SetAggro` guards, in order | grace-period `:85` · taunt-hold `:94` · grapple clear on target change `:99` · wait-rounds `:105` · Shooting inference `:115` · `Aggro` write `:121` · dual-write `TransitionToEngaging` `:129` |
| `EndAggro` | `func (c *Character) EndAggro()` — `:153`. Nils `Aggro`, `clearTauntHold()`, `ClearGrappleState()`, `RangedEngagedCueSpoken = false`, `CombatPhase.ForceIdle(...)` |
| Injection precedent | `userUntargetableFn func(userId int) bool` declared `:49`, registered `:59` |
| `combat.PowerScore` | `func PowerScore(char characters.Character) float64` — `internal/combat/calculations.go:19`. **Takes a value, not a pointer.** |
| `internal/characters` imports | does **NOT** import `rooms`, `mobs`, `users` or `combat` (`go list -deps`) |
| `internal/combat` imports | **does** import `rooms`, `mobs`, `users` |
| `EvalContext` | `Event`, `MobState`, `MobId`, `InstanceId`, `RoomId`, `MobName`, `Intercepted`, `SoftTarget state.ActorRef` — `internal/behaviortree/types.go` |
| `StageMeleeTarget` | `func StageMeleeTarget(user *users.UserRecord, room *rooms.Room, rest string, opts MeleeTargetOpts) (Actor, bool)` — `internal/actions/melee_target.go:150` |
| `ResolveTargetActor` | `func ResolveTargetActor(r *rooms.Room, name string, opts ...ResolveTargetOptions) (Actor, error)` — `internal/actions/target_resolution.go:73` |
| Proof-set actions | `actTargetRandomPlayerInRoom` `:159`, `actTargetWeakestMobInRoom` `:194`, `actAttack` `:19` (inline picker at `:38-46`) — `internal/behaviortree/actions_combat.go` |
| Registry | `actionRegistry["attack"]` `:38`, `["target_weakest_mob_in_room"]` `:92`, `["target_random_player_in_room"]` `:93` — `internal/behaviortree/actions.go` |
| `delayedActions` | `internal/behaviortree/actions.go:143-155`. The three target-setters are **deliberately absent** |
| `Character.Activity` | `*activity.Machine` — `internal/characters/character.go:214`; `IsCasting()` at `internal/state/activity/activity.go:100` |
| `Character.CombatPhase` | `*combatphase.Machine` — `character.go:182`, constructed in `New()` at `:396` |
| Test libs | `github.com/stretchr/testify/assert` and `/require` |

### 0.1 The two layering decisions this plan locks in

Both follow from the import facts above and **must not be quietly reversed later**.

1. **`internal/targeting` must NOT import `internal/combat`.** `Select`'s weakest-mob strategy needs `combat.PowerScore`, but `internal/combat/combat.go:409` is itself a `SetAggro` site that U12b has to migrate onto `targeting`. Importing `combat` would make that migration a cycle. The score function is therefore **injected**, following the `userUntargetableFn` precedent already in `characters`.

2. **There is NO exemption.** An earlier draft of this plan carved out `internal/characters/taunt_hold.go:22` as permanent, reasoning that `targeting` imports `characters` so `characters` can never import `targeting`. That was wrong, and it mattered: **taunt is the most frequent retargeting mechanic in the game**, so the exemption would have put the hole in the seam exactly where the traffic is.

   `ForceTauntAggro` has **zero callers inside `internal/characters`**. Its three production callers are `actions/combat_taunt.go:311`, `:317` and `hooks/pinnacle_tick.go:481`, all in packages that import `targeting` freely. The `SetAggro` at `taunt_hold.go:22` is not an independent call site; it is the body of a **targeting operation that lives in the storage package**.

   Task 7b splits it: `characters` keeps the three lock fields and gains exported accessors; `targeting` gains `CommitTaunt`; `ForceTauntAggro` is deleted. `characters/taunt_hold.go` then holds no commit at all, and **U12b's AST guard needs no whitelist**.

---

## Task 1: Package skeleton and the import-graph guard

Prove the layering before writing anything that depends on it.

**Files:**
- Create: `internal/targeting/doc.go`
- Create: `internal/targeting/imports_guard_test.go`

- [ ] **Step 1: Write the failing guard test**

```go
package targeting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

// TestTargetingDoesNotImportCombat pins layering decision 1 from the U12a
// plan. internal/combat contains a SetAggro site (combat.go:409) that U12b
// migrates onto this package. If targeting ever imports combat, that
// migration becomes an import cycle and U12b stalls.
//
// The weakest-mob strategy gets its score function by injection instead;
// see RegisterPowerScoreFn.
func TestTargetingDoesNotImportCombat(t *testing.T) {
	cfg := &packages.Config{Mode: packages.NeedImports | packages.NeedDeps | packages.NeedName}
	pkgs, err := packages.Load(cfg, "github.com/GoMudEngine/GoMud/internal/targeting")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}

	var offenders []string
	var walk func(p *packages.Package, seen map[string]bool)
	walk = func(p *packages.Package, seen map[string]bool) {
		for path, dep := range p.Imports {
			if seen[path] {
				continue
			}
			seen[path] = true
			if path == "github.com/GoMudEngine/GoMud/internal/combat" {
				offenders = append(offenders, path)
			}
			walk(dep, seen)
		}
	}
	walk(pkgs[0], map[string]bool{})

	assert.Empty(t, offenders,
		"internal/targeting must not depend on internal/combat: %s",
		strings.Join(offenders, ", "))
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/targeting/ -run TestTargetingDoesNotImportCombat -v`
Expected: FAIL — the package does not exist yet (`no Go files in .../internal/targeting`).

- [ ] **Step 3: Create the package doc**

```go
// Package targeting is the single seam for choosing and committing combat
// targets, for players and mobs alike.
//
// Two verbs, deliberately separate, because the codebase discovered the
// distinction three times independently before it had a name for it:
//
//   - Select answers "who should I fight?" and has NO combat consequence.
//     A thief archetype selects a victim without starting a fight
//     (behaviortree's SoftTarget), and StageMeleeTarget resolves a target
//     before the action has been paid for.
//   - Commit enters combat with a selected target. It is the only door
//     external packages use.
//
// LAYERING (see docs/superpowers/plans/completed/2026-08-29-u12a-targeting-seam.md):
//
//   - This package MUST NOT import internal/combat. internal/combat is
//     itself a Commit call site, so importing it creates a cycle. The
//     weakest-mob score arrives through RegisterPowerScoreFn instead.
//   - internal/characters can never import this package, because this
//     package imports it. That is a constraint on where targeting LOGIC may
//     live, not a licence for characters to keep committing: ForceTauntAggro
//     moved here as CommitTaunt, and characters kept only the lock state.
//     There are no exemptions from this seam.
//
// In U12a, Commit and Release delegate to characters.SetAggro and
// characters.EndAggro, so the Aggro/CombatPhase dual-write and every guard
// are untouched and behaviour is unchanged. U12b migrates the remaining
// write sites; U12c collapses the stores and deletes Aggro.
package targeting
```

- [ ] **Step 4: Run the guard to verify it passes**

Run: `go test ./internal/targeting/ -run TestTargetingDoesNotImportCombat -v`
Expected: PASS.

If `golang.org/x/tools/go/packages` is not already a dependency, run `go get golang.org/x/tools/go/packages` first and include `go.mod`/`go.sum` in the commit.

- [ ] **Step 5: Commit**

```bash
git add internal/targeting/doc.go internal/targeting/imports_guard_test.go go.mod go.sum
git commit -m "feat(targeting): package skeleton with the no-combat-import guard"
```

---

## Task 2: `Engagement` and `EngagementOf`

The pure query. Build it first because it has no dependencies beyond `characters` and is the easiest thing to test.

**Files:**
- Create: `internal/targeting/engagement.go`
- Create: `internal/targeting/engagement_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/assert"
)

func TestEngagementOf_IdleCharacterIsZero(t *testing.T) {
	c := characters.New()

	e := EngagementOf(c)

	assert.Equal(t, combatphase.Idle, e.Phase)
	assert.True(t, e.Target.IsZero())
	assert.False(t, e.OpeningUnspent)
	assert.False(t, e.Casting)
}

func TestEngagementOf_ReportsTargetAfterAggro(t *testing.T) {
	c := characters.New()
	c.SetAggro(0, 77, characters.DefaultAttack)

	e := EngagementOf(c)

	assert.Equal(t, 77, e.Target.MobInstanceId)
	assert.Equal(t, 0, e.Target.UserId)
}

func TestEngagementOf_OpeningUnspentTracksSurpriseAttack(t *testing.T) {
	c := characters.New()
	c.SetAggro(0, 77, characters.SurpriseAttack)

	assert.True(t, EngagementOf(c).OpeningUnspent)

	c.SetAggro(0, 77, characters.DefaultAttack)

	assert.False(t, EngagementOf(c).OpeningUnspent)
}

// TestEngagementOf_IsPure is the guard for the design's central rule: today
// the read IS the write (calculateCombat reads Aggro.Type and demotes it in
// the same breath). If EngagementOf ever inherits that, every caller asking
// "is this an ambush?" silently spends the ambush.
func TestEngagementOf_IsPure(t *testing.T) {
	c := characters.New()
	c.SetAggro(0, 77, characters.SurpriseAttack)

	for i := 0; i < 5; i++ {
		assert.True(t, EngagementOf(c).OpeningUnspent,
			"EngagementOf must not consume the opening strike (call %d)", i+1)
	}
	assert.Equal(t, characters.SurpriseAttack, c.Aggro.Type)
}

func TestEngagementOf_NilCharacterIsZero(t *testing.T) {
	e := EngagementOf(nil)

	assert.Equal(t, combatphase.Idle, e.Phase)
	assert.True(t, e.Target.IsZero())
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/targeting/ -run TestEngagementOf -v`
Expected: FAIL — `undefined: EngagementOf`.

- [ ] **Step 3: Implement**

```go
package targeting

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
)

// Engagement is the one place to ask "what kind of engagement is this?".
//
// The STORED/DERIVED split below is load-bearing and must not be "optimised".
//
//   - Ranged is DERIVED, so it is never stored: SetAggro already re-infers it
//     from the weapon subtype on every single call.
//   - OpeningUnspent is STORED and is NOT derivable from anything. U10d made
//     stealth break immediately, so "this engagement opened from concealment"
//     survives only as remembered state. Deriving it from IsHidden() would
//     reintroduce the bug U10d fixed.
type Engagement struct {
	Phase          combatphase.State // STORED
	Target         state.ActorRef    // STORED
	OpeningUnspent bool              // STORED  ambush opening not yet thrown
	Casting        bool              // DERIVED from the activity machine
	Ranged         bool              // DERIVED from the equipped weapon subtype
}

// EngagementOf composes the authoritative sources at read time.
//
// It is PURE. It stores nothing and consumes nothing, so there is no value to
// go stale and nothing to demote. Spending the opening strike is
// ConsumeOpeningStrike's job and has exactly one caller.
//
// U12a NOTE: Target and OpeningUnspent read Character.Aggro, because that is
// what the 294 production read sites do today and this slice changes no
// behaviour. U12c repoints them at CombatPhase and deletes Aggro.
func EngagementOf(c *characters.Character) Engagement {
	e := Engagement{Phase: combatphase.Idle}
	if c == nil {
		return e
	}

	if c.CombatPhase != nil {
		e.Phase = c.CombatPhase.State()
	}
	if c.Aggro != nil {
		e.Target = state.ActorRef{
			UserId:        c.Aggro.UserId,
			MobInstanceId: c.Aggro.MobInstanceId,
		}
		e.OpeningUnspent = c.Aggro.Type == characters.SurpriseAttack
	}
	if c.Activity != nil {
		e.Casting = c.Activity.IsCasting()
	}
	e.Ranged = c.Equipment.Weapon.GetSpec().Subtype == items.Shooting

	return e
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/targeting/ -run TestEngagementOf -v`
Expected: PASS, five tests.

- [ ] **Step 5: Commit**

```bash
git add internal/targeting/engagement.go internal/targeting/engagement_test.go
git commit -m "feat(targeting): Engagement and the pure EngagementOf query"
```

---

## Task 3: `Reason`, `Commit` and `Release`

**Files:**
- Create: `internal/targeting/commit.go`
- Create: `internal/targeting/commit_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommit_SetsTheTarget(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)

	require.NotNil(t, c.Aggro)
	assert.Equal(t, 42, c.Aggro.MobInstanceId)
	assert.Equal(t, 42, EngagementOf(c).Target.MobInstanceId)
}

func TestCommit_SurpriseReasonArmsTheOpening(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonSurprise)

	assert.True(t, EngagementOf(c).OpeningUnspent)
}

// TestCommit_DualWriteAgrees pins the invariant that SetAggro maintains by
// convention today: after any commit, the two stores describe the same
// engagement. U12c deletes one of them; until then this is what stops them
// drifting.
func TestCommit_DualWriteAgrees(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)

	require.NotNil(t, c.Aggro)
	require.NotNil(t, c.CombatPhase)
	assert.True(t, c.CombatPhase.IsInCombat(),
		"CombatPhase must agree that a commit started a fight")
}

func TestRelease_ClearsTheTarget(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)

	Release(c, ReasonDisengage)

	assert.Nil(t, c.Aggro)
	assert.True(t, EngagementOf(c).Target.IsZero())
}

func TestCommitAndRelease_NilCharacterDoNotPanic(t *testing.T) {
	assert.NotPanics(t, func() { Commit(nil, state.ActorRef{MobInstanceId: 1}, ReasonAttack) })
	assert.NotPanics(t, func() { Release(nil, ReasonDisengage) })
}

// TestCommit_ZeroRefIsRefused: a zero ActorRef means "nobody". Committing to
// nobody would set an engagement with no target, which every downstream
// consumer then has to defend against.
func TestCommit_ZeroRefIsRefused(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{}, ReasonAttack)

	assert.Nil(t, c.Aggro)
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/targeting/ -run 'TestCommit|TestRelease' -v`
Expected: FAIL — `undefined: Commit`, `undefined: ReasonAttack`.

- [ ] **Step 3: Implement**

```go
package targeting

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
)

// Reason says WHY a commit or release happened. It replaces the
// characters.AggroType enum at the seam boundary.
//
// It deliberately does NOT describe what kind of engagement resulted. That is
// Engagement's job, and conflating the two is exactly how Aggro.Type ended up
// being demoted mid-round. A Reason is a fact about a moment; an Engagement is
// a fact about a state.
type Reason int

const (
	ReasonAttack    Reason = iota // ordinary engagement
	ReasonSurprise                // opened from concealment
	ReasonRetaliate               // answering an incoming attack
	ReasonDisengage               // leaving combat
)

// Commit enters combat with ref.
//
// U12a: delegates to characters.SetAggro, so every guard (grace period,
// taunt-hold, grapple clearing, wait rounds, ranged inference) and the
// Aggro/CombatPhase dual-write are untouched. U12b migrates the remaining
// callers here; U12c moves the guard bodies in and deletes SetAggro.
func Commit(c *characters.Character, ref state.ActorRef, r Reason) {
	if c == nil || ref.IsZero() {
		return
	}
	c.SetAggro(ref.UserId, ref.MobInstanceId, aggroTypeFor(r))
}

// CommitAfter is Commit with an explicit extra wait, replacing SetAggro's
// overloaded roundsWaitTime variadic. Only two production sites pass one;
// everything else takes weapon speed, which is what Commit does.
func CommitAfter(c *characters.Character, ref state.ActorRef, r Reason, waitRounds int) {
	if c == nil || ref.IsZero() {
		return
	}
	c.SetAggro(ref.UserId, ref.MobInstanceId, aggroTypeFor(r), waitRounds)
}

// Release leaves combat.
func Release(c *characters.Character, r Reason) {
	if c == nil {
		return
	}
	c.EndAggro()
}

// aggroTypeFor is the ONLY translation between Reason and the legacy enum,
// and it exists only until U12c deletes AggroType. Keeping it in one function
// means the deletion is a single edit rather than a sweep.
//
// Shooting is deliberately absent: SetAggro infers it from the weapon subtype,
// and duplicating that inference here would let the two disagree.
func aggroTypeFor(r Reason) characters.AggroType {
	switch r {
	case ReasonSurprise:
		return characters.SurpriseAttack
	default:
		return characters.DefaultAttack
	}
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/targeting/ -v`
Expected: PASS, all tests including Task 2's.

- [ ] **Step 5: Commit**

```bash
git add internal/targeting/commit.go internal/targeting/commit_test.go
git commit -m "feat(targeting): Reason, Commit, CommitAfter and Release"
```

---

## Task 4: `ConsumeOpeningStrike`

**Note:** this member has **no production caller until U12c**, when `calculateCombat` stops doing the read-and-demote itself. It is built and tested here because U12a's whole purpose is to get the API reviewed as a whole. Do not wire it into `internal/combat` in this slice; that is a behavioural change and `internal/combat` is not in the proof set.

**Files:**
- Modify: `internal/targeting/engagement.go`
- Modify: `internal/targeting/engagement_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestConsumeOpeningStrike_SpendsExactlyOnce(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 9}, ReasonSurprise)

	assert.True(t, ConsumeOpeningStrike(c), "first call spends the opening")
	assert.False(t, ConsumeOpeningStrike(c), "second call must find it spent")
	assert.False(t, EngagementOf(c).OpeningUnspent)
}

func TestConsumeOpeningStrike_KeepsTheTarget(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 9}, ReasonSurprise)

	ConsumeOpeningStrike(c)

	assert.Equal(t, 9, EngagementOf(c).Target.MobInstanceId,
		"spending the opening must not end the engagement")
}

func TestConsumeOpeningStrike_FalseWhenNothingArmed(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 9}, ReasonAttack)

	assert.False(t, ConsumeOpeningStrike(c))
	assert.False(t, ConsumeOpeningStrike(nil))
}
```

Add `"github.com/GoMudEngine/GoMud/internal/state"` to the test file's imports if it is not already there.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/targeting/ -run TestConsumeOpeningStrike -v`
Expected: FAIL — `undefined: ConsumeOpeningStrike`.

- [ ] **Step 3: Implement, appending to `engagement.go`**

```go
// ConsumeOpeningStrike spends the ambush opening and reports whether it was
// there to spend. It is the ONE deliberate side effect in this package and
// must have exactly one caller: the swing loop, on the swing that is THROWN.
//
// It exists as a separate call precisely because EngagementOf is pure. Today
// calculateCombat reads Aggro.Type and demotes it in the same breath, which is
// why U10d had to add AttackResult.WasSurpriseAttack to carry the fact past
// the read. Splitting the query from the consumption is what stops a casual
// reader spending an ambush by asking about it.
//
// The engagement itself survives: only the opening is spent.
func ConsumeOpeningStrike(c *characters.Character) bool {
	if c == nil || c.Aggro == nil || c.Aggro.Type != characters.SurpriseAttack {
		return false
	}
	c.SetAggro(c.Aggro.UserId, c.Aggro.MobInstanceId, characters.DefaultAttack)
	return true
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/targeting/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/targeting/engagement.go internal/targeting/engagement_test.go
git commit -m "feat(targeting): ConsumeOpeningStrike, split from the pure query"
```

---

## Task 5: `Criteria`, `Scope` and `Select` for `RandomPlayer`

**Files:**
- Create: `internal/targeting/select.go`
- Create: `internal/targeting/select_test.go`

- [ ] **Step 1: Write the failing tests**

`rooms.Room` has unexported `players` and `mobs` fields, so a room literal is empty. Seed it with `room.AddPlayer(userId int) int` and `room.AddMob(mobInstanceId int)`, which is what `internal/actions/combat_drain_test.go:309` and its neighbours do. `&rooms.Room{RoomId: 1}` as the literal matches `internal/actions/aggression_test.go:49`.

```go
package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelect_RandomPlayerFindsAPlayerInTheRoom(t *testing.T) {
	room := &rooms.Room{RoomId: 1}
	room.AddPlayer(5)

	ref, ok := Select(Criteria{Kind: RandomPlayer}, Scope{Room: room})

	require.True(t, ok)
	assert.Equal(t, 5, ref.UserId)
	assert.True(t, ref.IsPlayer())
}

func TestSelect_RandomPlayerFailsInAnEmptyRoom(t *testing.T) {
	room := &rooms.Room{RoomId: 1}

	_, ok := Select(Criteria{Kind: RandomPlayer}, Scope{Room: room})

	assert.False(t, ok)
}

func TestSelect_NilRoomFails(t *testing.T) {
	_, ok := Select(Criteria{Kind: RandomPlayer}, Scope{})

	assert.False(t, ok)
}

// TestSelect_HasNoCombatConsequence is the point of the whole verb split.
// Selecting a victim must never start a fight; that is the chunk-2.7 bug
// class SoftTarget was invented to prevent.
func TestSelect_HasNoCombatConsequence(t *testing.T) {
	room := &rooms.Room{RoomId: 1}
	room.AddPlayer(5)
	c := characters.New()

	Select(Criteria{Kind: RandomPlayer}, Scope{Room: room, Self: c})

	assert.Nil(t, c.Aggro, "Select must not commit")
	assert.Equal(t, combatphase.Idle, EngagementOf(c).Phase)
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/targeting/ -run TestSelect -v`
Expected: FAIL — `undefined: Select`.

- [ ] **Step 3: Implement**

```go
package targeting

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Kind names a selection strategy.
type Kind int

const (
	RandomPlayer     Kind = iota // any player in the room
	WeakestHatedMob              // the weakest mob this actor hates
)

// Criteria says WHAT to look for.
type Criteria struct {
	Kind Kind

	// RatioBelow caps WeakestHatedMob: only candidates whose power ratio
	// against Self is strictly below this are eligible. Zero means 1.0,
	// matching the behaviour tree's ratio_below default.
	RatioBelow float64
}

// Scope says WHERE to look, and on whose behalf.
type Scope struct {
	Room *rooms.Room
	Self *characters.Character

	// SelfMobInstanceId is set when Self is a mob, so strategies that must
	// skip the actor itself can do so. Zero for players.
	SelfMobInstanceId int
}

// Select answers "who should I fight?" and has NO combat consequence.
//
// It never writes state. Committing to what it returns is Commit's job, and
// keeping the two apart is what lets a thief archetype pick a victim without
// starting a fight.
//
// Returns ok=false when nothing matches. Callers must treat that as a normal
// outcome, not an error.
func Select(c Criteria, s Scope) (state.ActorRef, bool) {
	if s.Room == nil {
		return state.ActorRef{}, false
	}
	switch c.Kind {
	case RandomPlayer:
		return selectRandomPlayer(s)
	case WeakestHatedMob:
		return selectWeakestHatedMob(c, s)
	}
	return state.ActorRef{}, false
}

func selectRandomPlayer(s Scope) (state.ActorRef, bool) {
	playerIds := s.Room.GetPlayers()
	if len(playerIds) == 0 {
		return state.ActorRef{}, false
	}
	return state.ActorRef{UserId: playerIds[util.Rand(len(playerIds))]}, true
}

// selectWeakestHatedMob is implemented in Task 6. This stub keeps the package
// compiling and makes the strategy fail closed until then: an unimplemented
// strategy must pick nobody, never pick arbitrarily.
func selectWeakestHatedMob(c Criteria, s Scope) (state.ActorRef, bool) {
	return state.ActorRef{}, false
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/targeting/ -run TestSelect -v`
Expected: PASS, four tests. `Scope.Self` is unused by `RandomPlayer` and that is fine; Task 6 gives it a consumer.

- [ ] **Step 5: Commit**

```bash
git add internal/targeting/select.go internal/targeting/select_test.go
git commit -m "feat(targeting): Select, Criteria and Scope with the RandomPlayer strategy"
```

---

## Task 6: `WeakestHatedMob` and the injected score

**Files:**
- Modify: `internal/targeting/select.go`
- Create: `internal/targeting/score.go`
- Modify: `internal/targeting/select_test.go`
- Modify: wherever boot registers cross-package functions (find it with `grep -rn "RegisterUserUntargetableFn" --include=*.go internal/ | grep -v _test`)

- [ ] **Step 1: Write the failing tests**

```go
// An empty room must not reach the scorer at all: there is nobody to score,
// and calling out to injected code on an empty scan is wasted work on the
// mob idle tick.
func TestSelect_WeakestHatedMobDoesNotScoreAnEmptyRoom(t *testing.T) {
	called := false
	RegisterPowerScoreFn(func(c characters.Character) float64 {
		called = true
		return 10
	})
	t.Cleanup(func() { RegisterPowerScoreFn(nil) })

	room := &rooms.Room{RoomId: 1}
	self := characters.New()

	_, ok := Select(Criteria{Kind: WeakestHatedMob},
		Scope{Room: room, Self: self, SelfMobInstanceId: 1})

	assert.False(t, ok)
	assert.False(t, called, "an empty room should not need to score anybody")
}

// TestSelect_WeakestHatedMobFailsWithoutAScorer is the safety net for the
// injection. If boot forgets to register the score function, selection must
// fail closed (pick nobody) rather than pick arbitrarily.
func TestSelect_WeakestHatedMobFailsWithoutAScorer(t *testing.T) {
	RegisterPowerScoreFn(nil)
	room := &rooms.Room{RoomId: 1}

	_, ok := Select(Criteria{Kind: WeakestHatedMob},
		Scope{Room: room, Self: characters.New(), SelfMobInstanceId: 1})

	assert.False(t, ok)
}

func TestCriteria_RatioBelowDefaultsToOne(t *testing.T) {
	assert.Equal(t, 1.0, effectiveRatio(Criteria{}))
	assert.Equal(t, 0.5, effectiveRatio(Criteria{RatioBelow: 0.5}))
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/targeting/ -run 'TestRegisterPowerScore|TestSelect_Weakest|TestCriteria' -v`
Expected: FAIL — `undefined: RegisterPowerScoreFn`, `undefined: effectiveRatio`.

- [ ] **Step 3: Implement `score.go`**

```go
package targeting

import "github.com/GoMudEngine/GoMud/internal/characters"

// powerScoreFn is injected at boot rather than imported, because
// internal/combat is itself a Commit call site (combat.go:409) and importing
// it here would make U12b's migration an import cycle. This mirrors
// characters.RegisterUserUntargetableFn, which exists for the same reason.
//
// Note the VALUE receiver: combat.PowerScore takes characters.Character, not
// a pointer.
var powerScoreFn func(c characters.Character) float64

// RegisterPowerScoreFn wires the scoring function. Call it once at boot.
// Passing nil unregisters, which is what the tests use.
func RegisterPowerScoreFn(fn func(c characters.Character) float64) {
	powerScoreFn = fn
}
```

- [ ] **Step 4: Implement the strategy, replacing the Task 5 stub**

```go
func effectiveRatio(c Criteria) float64 {
	if c.RatioBelow <= 0 {
		return 1.0
	}
	return c.RatioBelow
}

// selectWeakestHatedMob mirrors actTargetWeakestMobInRoom's rules exactly:
// skip self, dead mobs, non-combatants, mobs HatesMob rejects, and (when the
// caller is itself charmed) fellow companions of the same owner. Players are
// never scanned; predation is a mob-vs-mob action.
//
// Fails closed when no score function is registered. Picking arbitrarily
// would silently change which mob gets eaten.
func selectWeakestHatedMob(c Criteria, s Scope) (state.ActorRef, bool) {
	if powerScoreFn == nil || s.Self == nil {
		return state.ActorRef{}, false
	}
	self := mobs.GetInstance(s.SelfMobInstanceId)
	if self == nil || self.IsNonCombatant() {
		return state.ActorRef{}, false
	}
	selfPower := powerScoreFn(*s.Self)
	if selfPower <= 0 {
		return state.ActorRef{}, false
	}

	callerCharmedBy := s.Self.GetCharmedUserId()
	bestId := 0
	bestRatio := effectiveRatio(c)

	for _, otherId := range s.Room.GetMobs() {
		if otherId == s.SelfMobInstanceId {
			continue
		}
		other := mobs.GetInstance(otherId)
		if other == nil || other.IsNonCombatant() || other.Character.Health <= 0 {
			continue
		}
		if callerCharmedBy > 0 && other.Character.IsCharmed(callerCharmedBy) {
			continue
		}
		if !self.HatesMob(other) {
			continue
		}
		targetPower := powerScoreFn(other.Character)
		if targetPower <= 0 {
			continue
		}
		if ratio := targetPower / selfPower; ratio < bestRatio {
			bestRatio = ratio
			bestId = otherId
		}
	}
	if bestId == 0 {
		return state.ActorRef{}, false
	}
	return state.ActorRef{MobInstanceId: bestId}, true
}
```

Add `"github.com/GoMudEngine/GoMud/internal/mobs"` to `select.go`'s imports.

- [ ] **Step 5: Register at boot**

Find the boot wiring next to `RegisterUserUntargetableFn` and add:

```go
targeting.RegisterPowerScoreFn(combat.PowerScore)
```

- [ ] **Step 6: Run the full package plus a build**

Run: `go build ./... && go test ./internal/targeting/ -v`
Expected: build clean, all tests PASS, and `TestTargetingDoesNotImportCombat` still passes (the boot file imports both; the package itself does not).

- [ ] **Step 7: Commit**

```bash
git add internal/targeting/ && git add -u
git commit -m "feat(targeting): WeakestHatedMob strategy with an injected power score"
```

---

## Task 7: Convert the three behaviour-tree actions

**Files:**
- Modify: `internal/behaviortree/actions_combat.go:159-181` (`actTargetRandomPlayerInRoom`)
- Modify: `internal/behaviortree/actions_combat.go:194-249` (`actTargetWeakestMobInRoom`)
- Modify: `internal/behaviortree/actions_combat.go:19-81` (`actAttack`)
- Create: `internal/behaviortree/targeting_adapters_test.go`

**Do not touch** `internal/behaviortree/actions.go`. Registry names, parameters and `delayedActions` membership must be byte-identical.

- [ ] **Step 1: Write the failing tests**

```go
package behaviortree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRegistryNamesUnchanged pins the authored surface. Behaviour YAML
// references these names; renaming one silently breaks every tree using it.
func TestRegistryNamesUnchanged(t *testing.T) {
	for _, name := range []string{
		"attack",
		"target_random_player_in_room",
		"target_weakest_mob_in_room",
	} {
		_, ok := actionRegistry[name]
		assert.True(t, ok, "actionRegistry must still contain %q", name)
	}
}

// TestTargetSettersStayUndelayed pins actions.go:87-91. The target-setters are
// deliberately absent from delayedActions: a perception delay would open a
// window where idle ticks re-fire before the target takes effect.
func TestTargetSettersStayUndelayed(t *testing.T) {
	for _, name := range []string{
		"target_random_player_in_room",
		"target_weakest_mob_in_room",
	} {
		assert.False(t, delayedActions[name],
			"%q must not be perception-delayed", name)
	}
	assert.True(t, delayedActions["attack"], "attack stays delayed")
}
```

- [ ] **Step 2: Run to verify the state before conversion**

Run: `go test ./internal/behaviortree/ -run 'TestRegistryNames|TestTargetSetters' -v`
Expected: PASS already. These are regression pins, not red-first tests — their job is to fail if the conversion moves something it must not.

- [ ] **Step 3: Convert `actTargetRandomPlayerInRoom`**

```go
func actTargetRandomPlayerInRoom(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil {
		return Failure
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return Failure
	}

	// Select, never Commit. This archetype picks a victim for skullduggery
	// WITHOUT entering combat; committing here is the chunk-2.7 bug class.
	ref, ok := targeting.Select(targeting.Criteria{Kind: targeting.RandomPlayer},
		targeting.Scope{Room: room, Self: &mob.Character, SelfMobInstanceId: ctx.InstanceId})
	if !ok {
		return Failure
	}
	ctx.SoftTarget = ref
	return Success
}
```

- [ ] **Step 4: Convert `actTargetWeakestMobInRoom`**

```go
func actTargetWeakestMobInRoom(params map[string]any, ctx *EvalContext) Result {
	mob := mobs.GetInstance(ctx.InstanceId)
	if mob == nil || mob.IsNonCombatant() {
		return Failure
	}
	room := rooms.LoadRoom(ctx.RoomId)
	if room == nil {
		return Failure
	}

	ref, ok := targeting.Select(
		targeting.Criteria{
			Kind:       targeting.WeakestHatedMob,
			RatioBelow: getFloatParam(params, "ratio_below", 1.0),
		},
		targeting.Scope{Room: room, Self: &mob.Character, SelfMobInstanceId: ctx.InstanceId})
	if !ok {
		return Failure
	}

	// Predation DOES commit: this archetype wants the fight.
	targeting.Commit(&mob.Character, ref, targeting.ReasonAttack)
	return Success
}
```

- [ ] **Step 5: Convert `actAttack`'s inline picker and its commit**

Replace the inline fallback block at `actions_combat.go:38-46` with a `Select`, and the trailing `SetAggro` at `:79` with a `Commit`. Everything between — the `EngageAggroType` call and its comments — stays exactly as it is.

```go
	room := rooms.LoadRoom(ctx.RoomId)
	if targetUserId == 0 && targetMobId == 0 {
		if room == nil {
			return Failure
		}
		ref, ok := targeting.Select(targeting.Criteria{Kind: targeting.RandomPlayer},
			targeting.Scope{Room: room, Self: &mob.Character, SelfMobInstanceId: ctx.InstanceId})
		if !ok {
			return Failure
		}
		targetUserId = ref.UserId
	}
```

and at the end of the function:

```go
	reason := targeting.ReasonAttack
	if aggroType == characters.SurpriseAttack {
		reason = targeting.ReasonSurprise
	}
	targeting.Commit(&mob.Character,
		state.ActorRef{UserId: targetUserId, MobInstanceId: targetMobId}, reason)
	return Success
```

- [ ] **Step 6: Run the behaviour-tree suite**

Run: `go build ./... && go test ./internal/behaviortree/ -v`
Expected: PASS, including the two pins from Step 1.

- [ ] **Step 7: Commit**

```bash
git add internal/behaviortree/
git commit -m "refactor(behaviortree): three target actions onto the targeting seam"
```

---

## Task 7b: Move taunt onto the seam

Taunt is the highest-frequency retargeting mechanic in the game, so it is the best available proof of the API, and leaving it off the seam would put the hole where the traffic is.

**Files:**
- Modify: `internal/characters/taunt_hold.go` (delete `ForceTauntAggro`, export three accessors)
- Modify: `internal/characters/combat_state_compat.go:94` (call site of the renamed gate)
- Modify: `internal/characters/taunt_hold_test.go` (five existing tests call `ForceTauntAggro`)
- Modify: `internal/targeting/commit.go` (add `ReasonTaunt` and `CommitTaunt`)
- Modify: `internal/targeting/commit_test.go`
- Modify: `internal/actions/combat_taunt.go:311,317`
- Modify: `internal/hooks/pinnacle_tick.go:481`

- [ ] **Step 1: Write the failing tests in `internal/targeting/commit_test.go`**

```go
func TestCommitTaunt_PinsTheTargetOntoTheTaunter(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 50}, ReasonAttack)

	CommitTaunt(c, state.ActorRef{UserId: 7}, 4)

	assert.Equal(t, 7, EngagementOf(c).Target.UserId)
	assert.Equal(t, 0, EngagementOf(c).Target.MobInstanceId)
}

// TestCommitTaunt_HoldSurvivesReaggro is the whole point of the mechanic: an
// ally swinging at the taunted mob must not flip it back off the taunter.
func TestCommitTaunt_HoldSurvivesReaggro(t *testing.T) {
	c := characters.New()
	CommitTaunt(c, state.ActorRef{UserId: 7}, 4)

	Commit(c, state.ActorRef{MobInstanceId: 50}, ReasonAttack)

	assert.Equal(t, 7, EngagementOf(c).Target.UserId,
		"a basic re-aggro must not break an active taunt hold")
}

// TestCommitTaunt_OrderIsLoadBearing: the hold is set BEFORE the commit so
// the gate sees the new taunter as the locked target and lets this very set
// through. If the two lines are reversed, a taunt cannot override an existing
// hold and silently no-ops.
func TestCommitTaunt_NewerTauntOverridesActiveHold(t *testing.T) {
	c := characters.New()
	CommitTaunt(c, state.ActorRef{MobInstanceId: 50}, 4)

	CommitTaunt(c, state.ActorRef{MobInstanceId: 60}, 4)

	assert.Equal(t, 60, EngagementOf(c).Target.MobInstanceId)
}

func TestCommitTaunt_NilAndZeroAreSafe(t *testing.T) {
	assert.NotPanics(t, func() { CommitTaunt(nil, state.ActorRef{UserId: 7}, 4) })

	c := characters.New()
	CommitTaunt(c, state.ActorRef{}, 4)
	assert.Nil(t, c.Aggro, "a taunt with no taunter must not engage anybody")
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/targeting/ -run TestCommitTaunt -v`
Expected: FAIL — `undefined: CommitTaunt`.

- [ ] **Step 3: Export the lock accessors in `internal/characters/taunt_hold.go`**

Delete `ForceTauntAggro` entirely and replace the three unexported helpers with exported ones. The lock fields at `character.go:155-157` stay exactly as they are.

```go
// SetTauntHold applies a taunt-hold lock onto the given taunter for
// holdRounds. It does NOT engage: committing is targeting.CommitTaunt's job,
// and the split is what keeps internal/characters free of targeting logic.
func (c *Character) SetTauntHold(userId, mobInstanceId, holdRounds int) {
	if holdRounds < 1 {
		holdRounds = 1
	}
	c.tauntHoldUserId = userId
	c.tauntHoldMobInstanceId = mobInstanceId
	c.tauntHoldUntilRound = util.GetRoundCount() + uint64(holdRounds)
}

// TauntHoldBlocks reports whether an incoming target set should be ignored
// because a taunt hold pins this character onto a different taunter. Only
// basic attack-type aggro (DefaultAttack/Shooting/SurpriseAttack) is pinned;
// SpellCast and Flee always pass, as does a set matching the locked taunter.
//
// Exported so targeting.Commit can consult it once U12c deletes SetAggro and
// the guard bodies move out of this package.
func (c *Character) TauntHoldBlocks(userId, mobInstanceId int, aggroType AggroType) bool {
	if !c.tauntHoldActive() {
		return false
	}
	switch aggroType {
	case DefaultAttack, Shooting, SurpriseAttack:
		return userId != c.tauntHoldUserId || mobInstanceId != c.tauntHoldMobInstanceId
	default:
		return false
	}
}

// ClearTauntHold drops any active lock. Called from EndAggro so a dead or
// fled taunter doesn't leave the enemy pinned and unable to re-acquire.
func (c *Character) ClearTauntHold() {
	c.tauntHoldUntilRound = 0
	c.tauntHoldUserId = 0
	c.tauntHoldMobInstanceId = 0
}
```

Keep `tauntHoldActive` unexported. Update the two internal callers: `combat_state_compat.go:94` becomes `c.TauntHoldBlocks(...)` and `:155` becomes `c.ClearTauntHold()`.

- [ ] **Step 4: Add `ReasonTaunt` and `CommitTaunt` to `internal/targeting/commit.go`**

Add `ReasonTaunt` to the `Reason` const block, after `ReasonRetaliate`.

```go
// CommitTaunt pins c onto ref for holdRounds, then commits.
//
// ORDER IS LOAD-BEARING. The hold is set BEFORE the commit so the taunt-hold
// gate sees the new taunter as the locked target and lets this very set
// through. It is also why a newer taunt cleanly overrides an older hold.
// Reversing the two lines makes every taunt silently no-op against an
// existing hold, and nothing would fail loudly.
func CommitTaunt(c *characters.Character, ref state.ActorRef, holdRounds int) {
	if c == nil || ref.IsZero() {
		return
	}
	c.SetTauntHold(ref.UserId, ref.MobInstanceId, holdRounds)
	Commit(c, ref, ReasonTaunt)
}
```

In `aggroTypeFor`, `ReasonTaunt` must fall through to `DefaultAttack`. Do **not** give it a case of its own: the hold gate pins exactly `DefaultAttack`/`Shooting`/`SurpriseAttack`, so any other value would make a taunt unable to hold itself. Add this comment to the `default` branch:

```go
	// ReasonTaunt lands here deliberately. The taunt-hold gate pins only
	// DefaultAttack/Shooting/SurpriseAttack, so a taunt that committed as
	// anything else could not hold its own target.
```

- [ ] **Step 5: Migrate the three call sites**

`internal/actions/combat_taunt.go:311` and `:317`, preserving the surrounding conditionals exactly:

```go
					targeting.CommitTaunt(&targetMob.Character,
						state.ActorRef{UserId: attackerUserId}, holdRounds)
```

```go
					targeting.CommitTaunt(&targetMob.Character,
						state.ActorRef{MobInstanceId: attackerMobId}, holdRounds)
```

`internal/hooks/pinnacle_tick.go:481`:

```go
	targeting.CommitTaunt(&mob.Character, state.ActorRef{UserId: user.UserId}, holdRounds)
```

- [ ] **Step 6: Update the five existing taunt-hold tests**

`internal/characters/taunt_hold_test.go` calls `ForceTauntAggro` at `:26`, `:44`, `:59`, `:62`, `:73` and `:89`. Those tests live in `characters`, which cannot import `targeting`, so rewrite each to the two-line equivalent:

```go
	c.SetTauntHold(0, 50, 4)
	c.SetAggro(0, 50, characters.DefaultAttack)
```

They keep testing the gate, which is what they were always about. Do not delete any of the five.

- [ ] **Step 7: Verify nothing still calls the deleted method**

Run: `grep -rn "ForceTauntAggro" --include=*.go internal/`
Expected: no hits. Then `go build ./... && go test ./internal/characters/ ./internal/targeting/ ./internal/actions/ ./internal/hooks/ -v`
Expected: PASS, including all five pre-existing taunt-hold tests.

- [ ] **Step 8: Commit**

```bash
git add internal/characters/ internal/targeting/ internal/actions/combat_taunt.go internal/hooks/pinnacle_tick.go
git commit -m "refactor(targeting): taunt commits through CommitTaunt, no seam exemption"
```

---

## Task 8: Convert `StageMeleeTarget`

`StageMeleeTarget` already separates selection from commitment — it resolves a target and defers the engagement to `commitMeleeEngagement` after the action is paid for. That makes it the player-side proof that the two verbs are right.

**Files:**
- Modify: `internal/actions/melee_target.go:119-127` (`commitMeleeEngagement`)
- Modify: `internal/actions/melee_target.go:205-240` (the two `SetAggro` calls at `:213` and `:236`)
- Create: `internal/actions/melee_target_seam_test.go`

- [ ] **Step 1: Read the existing melee-target coverage first**

Run: `ls internal/actions/ | grep melee` and read whatever turns up, plus `grep -rn "StageMeleeTarget" --include=*_test.go internal/actions/`.

This task deliberately writes **no new unit test**. The behaviour it changes is "which function performs the write", and the honest pin for that is the AST-level parity guard in Task 9, not a hand-built fixture that would need a user, a room, a target mob and a live combat state to assert something the guard proves directly. Writing a `characters.New()` test here would pass without exercising `StageMeleeTarget` at all, which is worse than no test.

The verification for this task is: the existing `internal/actions` suite still passes, and Task 9's guard goes green.

- [ ] **Step 2: Confirm the current call sites before editing**

Run: `grep -n "SetAggro(" internal/actions/melee_target.go`
Expected: exactly two hits, at `:213` and `:236`.

- [ ] **Step 3: Replace the two `SetAggro` calls with `Commit`**

At `melee_target.go:213`:

```go
			targeting.Commit(&user.Character,
				state.ActorRef{MobInstanceId: mob.InstanceId}, targeting.ReasonAttack)
```

At `melee_target.go:236`:

```go
		targeting.Commit(&user.Character,
			state.ActorRef{UserId: p.UserId}, targeting.ReasonAttack)
```

Read both sites in full before editing — the surrounding aggression-seeding and party checks stay exactly as they are.

- [ ] **Step 4: Build and run the actions suite**

Run: `go build ./... && go test ./internal/actions/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/actions/
git commit -m "refactor(actions): StageMeleeTarget commits through the targeting seam"
```

---

## Task 9: Extend the parity guard

**Files:**
- Modify: `internal/actions/ambush_parity_guard_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestTargetingParity_BothSidesUseTheSeam asserts that the player path and the
// behaviour-tree path both reach internal/targeting rather than writing aggro
// themselves. The two drifted before (U10d had to route btree ambush through
// EngageAggroType after it had been setting SurpriseAttack straight from
// IsHidden), and a divergence here is invisible at runtime.
func TestTargetingParity_BothSidesUseTheSeam(t *testing.T) {
	for _, f := range []string{
		"melee_target.go",
		"combat_taunt.go",
		"../behaviortree/actions_combat.go",
		"../hooks/pinnacle_tick.go",
		"../characters/taunt_hold.go",
	} {
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		assert.NotContains(t, string(src), ".SetAggro(",
			"%s is on the U12a proof set and must commit through internal/targeting", f)
		assert.NotContains(t, string(src), "ForceTauntAggro",
			"%s must use targeting.CommitTaunt; ForceTauntAggro is deleted", f)
	}
}
```

- [ ] **Step 2: Run to verify it passes after Tasks 7 and 8**

Run: `go test ./internal/actions/ -run TestTargetingParity -v`
Expected: PASS. If it fails, a `SetAggro` call was missed in Task 7 or 8 — find it with `grep -n "SetAggro(" internal/actions/melee_target.go internal/behaviortree/actions_combat.go`.

- [ ] **Step 3: Commit**

```bash
git add internal/actions/ambush_parity_guard_test.go
git commit -m "test(actions): parity guard covers the targeting seam, not only ambush"
```

---

## Task 10: Measure `EngagementOf` on the hot path

Spec risk 1. `EngagementOf` will be called per actor per round once U12c lands. Measure before anything depends on it.

**Files:**
- Create: `internal/targeting/engagement_bench_test.go`

- [ ] **Step 1: Write the benchmark**

```go
package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
)

func BenchmarkEngagementOf(b *testing.B) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EngagementOf(c)
	}
}
```

- [ ] **Step 2: Run it and record the result**

Run: `go test ./internal/targeting/ -bench BenchmarkEngagementOf -benchmem -run '^$'`
Expected: a result in the tens of nanoseconds with **0 allocs/op**. `GetSpec()` is the only call that could surprise; if allocs are non-zero or the time exceeds ~200ns/op, stop and report rather than proceeding — that is a finding U12c needs, not a number to shrug at.

- [ ] **Step 3: Record the number in the spec**

Append the measured figure to the spec's risk 1 so U12c inherits it rather than re-deriving it.

- [ ] **Step 4: Commit**

```bash
git add internal/targeting/engagement_bench_test.go docs/superpowers/specs/completed/2026-08-29-u12-unified-targeting-design.md
git commit -m "test(targeting): benchmark EngagementOf and record the hot-path cost"
```

---

## Task 11: `context.md`, patch notes and the pre-push gate

**Files:**
- Create: `internal/targeting/context.md`
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Write `internal/targeting/context.md`**

Follow the project convention in CLAUDE.md: `## Purpose` (say what it deliberately does not do), `## Files`, core types with real field names, `## Public API` with verified signatures, `## Gotchas`, `## Dependencies`, `## Consumers`. No "Future Enhancements" or "Scalability" filler.

The Gotchas section must carry all four of these:

1. `EngagementOf` is pure; `ConsumeOpeningStrike` is the only side effect, and it has exactly one intended caller.
2. This package must never import `internal/combat`; the score arrives by injection. There is a test that fails if this is violated.
3. `CommitTaunt` sets the hold BEFORE committing, and reversing those two lines makes every taunt silently no-op against an existing hold. `ReasonTaunt` maps to `DefaultAttack`, not to a value of its own, because that is what the hold gate pins.
4. `Engagement.Ranged` is derived and `Engagement.OpeningUnspent` is stored, and neither may swap.

- [ ] **Step 2: Add a dated patch-notes entry**

Player-facing framing, no raw numbers, no em dashes. This slice changes nothing a player can see, so the entry should say so plainly rather than inventing an improvement.

- [ ] **Step 3: Run the pre-push gate**

```bash
gofmt -l internal/ modules/          # must print nothing
go build ./...
go test ./internal/targeting/ ./internal/behaviortree/ ./internal/actions/ ./internal/characters/ ./internal/combat/
```

- [ ] **Step 4: Boot test in an isolated detached worktree**

Per CLAUDE.md's pre-push SOP. Exit code 124 is the success case.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Clean up with `git worktree remove --force C:/tmp/dogmud-boot-check`.

- [ ] **Step 5: Commit and open the PR**

```bash
git add internal/targeting/context.md docs/PATCH_NOTES.md
git commit -m "docs(targeting): context.md and patch notes for the U12a seam"
git push -u origin feature/u12a-targeting-seam
gh pr create --repo pruuk/DOGMud --base master --head feature/u12a-targeting-seam --fill
```

---

## Done when

1. `internal/targeting` exists with `Select`, `Commit`, `CommitAfter`, `CommitTaunt`, `Release`, `EngagementOf`, `ConsumeOpeningStrike` and a `context.md`.
2. `grep -n "SetAggro(" internal/behaviortree/actions_combat.go internal/actions/melee_target.go internal/actions/combat_taunt.go internal/characters/taunt_hold.go internal/hooks/pinnacle_tick.go` returns nothing.
2b. `grep -rn "ForceTauntAggro" --include=*.go internal/` returns nothing, and all five pre-existing taunt-hold tests still pass. **There are no seam exemptions**, so U12b's AST guard will need no whitelist.
3. Registry names, action parameters and `delayedActions` membership are unchanged, asserted by test.
4. The no-combat-import guard passes.
5. `EngagementOf`'s cost is measured and written into the spec.
6. Boot is clean and the PR is green.
7. The remaining ~86 `SetAggro` / `EndAggro` sites are **untouched**. That is correct, not unfinished: U12b sweeps them.

> **Correction, added 2026-08-29 during U12b:** an earlier version of this plan and of spec §5 said U12b would DELETE `SetAggro` and `EndAggro`. It cannot. `(*Character).Charm` calls `EndAggro` from inside `internal/characters`, which can never import `internal/targeting`. They survive as the package-internal storage primitives, and U12b enforces a caller restriction instead. See the U12b plan §0.1.
