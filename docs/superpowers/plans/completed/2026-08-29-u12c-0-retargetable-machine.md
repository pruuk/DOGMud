# U12c-0 — Retargetable Combat Phase Machine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a mid-combat retarget actually update `CombatPhase`, so `CurrentCombatTarget()` stops returning the previous enemy.

**Architecture:** One entry in `validTransitions` makes `Engaged → Engaging` legal, so `SetAggro`'s existing (currently failing) `TransitionToEngaging` call succeeds on a retarget. A retarget becomes a fresh engagement: new target, fresh wind-up, back to `Engaged`. Two pieces of housekeeping ride along — clearing the superseded `Engaged` data, and moving the inbound-attacker entry off the old target.

**Tech Stack:** Go, `testify/assert` + `require`, `internal/state` machine framework.

**Spec:** [`2026-08-29-u12-unified-targeting-design`](../specs/2026-08-29-u12-unified-targeting-design.md) §6.1

**Branch:** `feature/u12c-0-retargetable-machine`. The spec commits currently sit on `feature/u12c1-targeting-reads`; rename it (`git branch -m`) rather than branching again, since U12c-0 must land first.

---

## 0. Facts verified against source

Read at merged HEAD `5f1ca6b99` on 2026-08-29.

| Fact | Value |
|---|---|
| The bug | `validTransitions` has `Engaged: {Disengaging, Idle}`; `Engaged → Engaging` is illegal — `internal/state/combatphase/transitions.go:9` |
| Why it is silent | `SetAggro` calls `_ = c.CombatPhase.TransitionToEngaging(...)`, discarding the error — `internal/characters/combat_state_compat.go:138` |
| Other retarget paths | **None.** Every `func (m *Machine)` was enumerated; only `TransitionToEngaging` sets a target |
| Why the fallback misses | `CurrentCombatTarget()` falls back to `Aggro` only when CombatPhase's target is **zero** — `characters/character.go:783` |
| Consumers already affected | ~40 `CurrentCombatTarget()` sites, incl. `users/userrecord.prompt.go:541`/`:557` (the `{target}` and `{targethealth}` tokens), `mobcommands/attack.go:84`, `behaviortree/actions_party.go:302` |
| `mob_engaged` listeners | **Zero authored behaviour trees.** Verified against `_datafiles/world/dogmud/behaviors/`, where 14 other event names do appear |
| `IsEngaged()` production consumers | **Zero** beyond its own `Character` wrapper (`character.go:740`) |
| Data accessors | `EngagingData()`, `EngagedData()`, `DisengagingData()` are all **state-gated**, so stale data in a non-current state is invisible through the public API |
| Existing tests asserting the old behaviour | **None.** No test in `combatphase_test.go` asserts `Engaged → Engaging` fails |
| Inbound attacker bookkeeping | `TransitionToEngaging` calls `target.RecordInboundAttacker`; `lookupMachine` returns nil in production because `combatphase.RegisterMachine` has no production callers, so this is **inert today** |

---

## Task 1: The regression test

Write it first and watch it fail — this is the bug, reproduced.

**Files:**
- Create: `internal/state/combatphase/retarget_test.go`

- [ ] **Step 1: Write the failing test**

```go
package combatphase

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/require"
)

func retargetTestActor(id int) state.ActorRef {
	return state.ActorRef{MobInstanceId: id}
}

// engageFully drives a machine from Idle to Engaged against target, the way
// the round driver does: transition, then tick until the wind-up expires.
func engageFully(t *testing.T, m *Machine, target state.ActorRef, roundsUntil int) {
	t.Helper()
	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: target, RoundsUntil: roundsUntil},
		state.TransitionReason{Trigger: TriggerAttackCommand}))
	for i := 0; i < roundsUntil+2 && m.State() != Engaged; i++ {
		m.OnRoundTick()
	}
	require.Equal(t, Engaged, m.State(), "fixture must reach Engaged")
}

// TestRetarget_WhileEngagedUpdatesTheTarget is the U12c-0 regression.
//
// Before this slice, validTransitions declared Engaged: {Disengaging, Idle},
// so TransitionToEngaging returned ErrInvalidTransition on a retarget and
// SetAggro discarded it. CurrentTarget kept returning the PREVIOUS enemy,
// which is what the {target} and {targethealth} prompt tokens render.
func TestRetarget_WhileEngagedUpdatesTheTarget(t *testing.T) {
	m := NewMachine()
	engageFully(t, m, retargetTestActor(100), 1)
	require.Equal(t, retargetTestActor(100), m.CurrentTarget())

	err := m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(200), RoundsUntil: 1},
		state.TransitionReason{Trigger: TriggerAttackCommand})

	require.NoError(t, err, "a retarget while Engaged must be a legal transition")
	require.Equal(t, retargetTestActor(200), m.CurrentTarget(),
		"CurrentTarget must follow the retarget, not keep the old enemy")
}

// A retarget re-imposes the wind-up. That is the intended behaviour change:
// switching targets mid-fight takes a moment, and SetAggro already reseeds
// RoundsWaiting on every retarget, so the moment was already being paid.
func TestRetarget_ReimposesTheWindUp(t *testing.T) {
	m := NewMachine()
	engageFully(t, m, retargetTestActor(100), 1)

	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(200), RoundsUntil: 2},
		state.TransitionReason{Trigger: TriggerAttackCommand}))

	require.Equal(t, Engaging, m.State(),
		"a retarget re-enters Engaging rather than staying Engaged")

	d, ok := m.EngagingData()
	require.True(t, ok)
	require.Equal(t, 2, d.RoundsUntil, "the new wind-up is the one supplied")

	m.OnRoundTick()
	m.OnRoundTick()
	require.Equal(t, Engaged, m.State(), "and it advances back to Engaged")
	require.Equal(t, retargetTestActor(200), m.CurrentTarget())
}

// The superseded Engaged data must not survive the retarget. The public
// accessors are state-gated so a stale value is invisible today, but leaving
// it sets a trap for any future accessor that is not.
func TestRetarget_ClearsTheSupersededEngagedData(t *testing.T) {
	m := NewMachine()
	engageFully(t, m, retargetTestActor(100), 1)

	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(200), RoundsUntil: 1},
		state.TransitionReason{Trigger: TriggerAttackCommand}))

	require.Nil(t, m.engaged,
		"the superseded Engaged data must be cleared, not left behind")
}

// A retarget still runs the target vetoes. This is why the fix allows the
// transition rather than mutating m.engaged in place: an in-place setter would
// skip the vetoes and could leave the machine pointing at a corpse.
func TestRetarget_StillHonoursTargetVetoes(t *testing.T) {
	m := NewMachine()
	engageFully(t, m, retargetTestActor(100), 1)

	m.RegisterTargetLifeCheck(func(state.ActorRef) bool { return false })

	err := m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(200), RoundsUntil: 1},
		state.TransitionReason{Trigger: TriggerAttackCommand})

	require.Error(t, err, "a retarget onto a dead target must be refused")
	require.Equal(t, retargetTestActor(100), m.CurrentTarget(),
		"a refused retarget leaves the existing engagement intact")
}
```

- [ ] **Step 2: Run to verify the bug reproduces**

Run: `go test ./internal/state/combatphase/ -run TestRetarget -v`
Expected: `TestRetarget_WhileEngagedUpdatesTheTarget`, `TestRetarget_ReimposesTheWindUp` and `TestRetarget_ClearsTheSupersededEngagedData` all FAIL with an invalid-transition error. `TestRetarget_StillHonoursTargetVetoes` may PASS already (the transition fails for the wrong reason) — that is expected and it becomes meaningful after Step 3.

- [ ] **Step 3: Commit the red test**

Commit the failing test on its own so the bug is recorded in history independently of the fix.

```bash
git add internal/state/combatphase/retarget_test.go
git commit -m "test(combatphase): reproduce the stale-target retarget bug"
```

---

## Task 2: Allow the transition

**Files:**
- Modify: `internal/state/combatphase/transitions.go:9`
- Modify: `internal/state/combatphase/combatphase.go` (inside `TransitionToEngaging`)

- [ ] **Step 1: Make `Engaged → Engaging` legal**

In `internal/state/combatphase/transitions.go`, replace the `Engaged` row:

```go
var validTransitions = state.TransitionTable[State]{
	Idle:     {Engaging},
	Engaging: {Engaged, Idle}, // Idle on cancel/target-died
	// Engaging on RETARGET (U12c-0). Switching targets mid-fight is a fresh
	// engagement: new target, fresh wind-up, then back to Engaged. Without
	// this, TransitionToEngaging failed on every retarget and SetAggro
	// discarded the error, so CurrentTarget kept returning the PREVIOUS
	// enemy — which the {target} and {targethealth} prompt tokens render.
	Engaged:     {Disengaging, Idle, Engaging},
	Disengaging: {Idle, Engaged}, // Engaged on flee failure
}
```

- [ ] **Step 2: Clear the superseded state data and move the inbound entry**

In `TransitionToEngaging` (`combatphase.go`), replace the block after the successful inner transition:

```go
	if err := m.inner.TransitionTo(Engaging, r); err != nil {
		return err
	}

	// U12c-0: this transition is now reachable from Engaged (a retarget), so
	// the superseded state data must go. The public accessors are state-gated
	// and would hide a stale value, which is exactly why leaving it would be a
	// trap for the next accessor that is not.
	prevTarget := m.CurrentTarget()
	m.engaged = nil
	m.disengaging = nil

	m.engaging = &d

	// Move our inbound-attacker entry off the previous target. Inert today —
	// lookupMachine returns nil because combatphase.RegisterMachine has no
	// production callers — but without it a retarget would leak an entry on
	// the old target the day that registry is wired up.
	selfRef := r.Actor
	if selfRef.IsZero() {
		selfRef = m.self
	}
	if !prevTarget.IsZero() && prevTarget != d.Target {
		if prev := lookupMachine(prevTarget); prev != nil && !selfRef.IsZero() {
			prev.RemoveInboundAttacker(selfRef)
		}
	}

	if target := lookupMachine(d.Target); target != nil {
		target.RecordInboundAttacker(r.Actor)
	}
	return nil
}
```

Note `prevTarget` is captured **before** `m.engaged` is cleared, because `CurrentTarget()` reads the state data.

- [ ] **Step 3: Run the regression tests**

Run: `go test ./internal/state/combatphase/ -run TestRetarget -v`
Expected: all four PASS.

- [ ] **Step 4: Run the whole combatphase suite**

Run: `go test ./internal/state/combatphase/ -v`
Expected: PASS. If an existing test fails, STOP and report it — the facts table says none asserts the old behaviour, so a failure means that check was wrong.

- [ ] **Step 5: Commit**

```bash
git add internal/state/combatphase/
git commit -m "fix(combatphase): allow Engaged to Engaging so a retarget lands"
```

---

## Task 3: Verify the fix reaches the seam and the prompt

The unit tests prove the machine. This proves the thing players actually see.

**Files:**
- Create: `internal/targeting/retarget_test.go`

- [ ] **Step 1: Write the end-to-end test**

```go
package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/require"
)

// TestCommit_RetargetKeepsTheStoresInAgreement is the U12c-0 fix seen from
// where it matters. CurrentCombatTarget is what the {target} and
// {targethealth} prompt tokens render (users/userrecord.prompt.go:541, :557),
// and before this slice it kept naming the PREVIOUS enemy after a successful
// target switch.
func TestCommit_RetargetKeepsTheStoresInAgreement(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{MobInstanceId: 100}, ReasonAttack)
	for i := 0; i < 10; i++ {
		c.CombatPhase.OnRoundTick()
	}
	require.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId)

	Commit(c, state.ActorRef{MobInstanceId: 200}, ReasonAttack)

	require.Equal(t, 200, c.Aggro.MobInstanceId, "Aggro holds the new target")
	require.Equal(t, 200, c.CurrentCombatTarget().MobInstanceId,
		"CurrentCombatTarget must follow the retarget; this is the prompt bug")
	require.Equal(t, 200, EngagementOf(c).Target.MobInstanceId,
		"and the seam's own query must agree")
}

// CommitTaunt takes the same path, and taunt is the game's most frequent
// retargeting mechanic, so it gets its own assertion rather than being
// assumed to follow.
func TestCommitTaunt_RetargetKeepsTheStoresInAgreement(t *testing.T) {
	c := characters.New()

	Commit(c, state.ActorRef{UserId: 7}, ReasonAttack)
	for i := 0; i < 10; i++ {
		c.CombatPhase.OnRoundTick()
	}
	require.Equal(t, 7, c.CurrentCombatTarget().UserId)

	CommitTaunt(c, state.ActorRef{UserId: 9}, 4)

	require.Equal(t, 9, c.Aggro.UserId)
	require.Equal(t, 9, c.CurrentCombatTarget().UserId,
		"a taunt must pull the CombatPhase target too, not just Aggro")
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/targeting/ -run Retarget -v`
Expected: both PASS. If either fails, the machine fix is not reaching through `SetAggro`'s dual-write — STOP and report rather than adjusting the test.

- [ ] **Step 3: Commit**

```bash
git add internal/targeting/retarget_test.go
git commit -m "test(targeting): pin retarget agreement through the seam"
```

---

## Task 4: Full verification, patch notes, boot, PR

**Files:**
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Run the affected suites**

Run: `gofmt -l internal/ modules/ main.go && go build ./... && go test ./internal/...`
Expected: gofmt silent, build OK, everything green.

⚠️ `TestCheckConcentrationBreak_ProgressionFiresOnEveryResolvedContest` is known-flaky and its subject lives in `hooks/combat_shared_helpers.go`, untouched by this slice. If it fails, re-run before investigating.

- [ ] **Step 2: Add a dated patch-notes entry**

Insert at the top of `docs/PATCH_NOTES.md`, directly under the `# DOGMud Patch Notes` heading. Unlike the two entries below it, this one **is** visible in play, and the entry says so:

```markdown
## 2026-08-29: Switching targets now sticks

Change who you are fighting and the game now agrees with you about it.

Until now, switching targets updated the fight itself but not the part of the
game that answers the question "who am I fighting?". Your prompt would keep
naming the previous enemy, and showing that enemy's health, even though your
blows were landing on the new one. Anything else that asked the same question
got the same stale answer, which included creatures deciding whether they were
already fighting you, and companions choosing who to help.

Taunts were affected too, and they are the most common way a fight changes
target, so this was easiest to notice in a group.

Nothing about how quickly you switch has changed. There is still a moment of
repositioning when you turn on someone new, exactly as before.
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
git push -u origin feature/u12c-0-retargetable-machine
gh pr create --repo pruuk/DOGMud --base master --head feature/u12c-0-retargetable-machine --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

---

## Done when

1. All four `combatphase` retarget tests pass, and the two `targeting` ones do.
2. `CurrentCombatTarget()` equals the committed target after a retarget, for both `Commit` and `CommitTaunt`.
3. A retarget onto a dead target is still refused, and leaves the existing engagement intact.
4. The superseded `Engaged` data is cleared on retarget.
5. `go test ./internal/...` green, boot clean, PR green.
6. Nothing else changed. This slice does NOT migrate any read, delete any field, or touch `Aggro` — U12c-1 and U12c-2 own those.

## Deliberately NOT in this slice

- **No adversarial playtest.** This is a small, well-pinned bug fix and the arc's playtest belongs to U12c-2, which is where the store actually changes. If the owner wants combat feel checked earlier, that is a separate call.
- **No change to `SetAggro`'s discarded error.** It now succeeds on the path that mattered. Whether an ignored transition error is acceptable at all is a U12c-2 question, once `SetAggro` writes only one store.
