# U12c-0b — Load-Bearing Vetoes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a refused combat-phase transition actually refuse the commit, so `Aggro` and `CombatPhase` can never disagree.

**Architecture:** Three changes in dependency order. First finish the transition table, so a commit can land from every state it legitimately should. Then fix the one content file that would otherwise make a veto harmful. Only then invert `SetAggro`: attempt the transition first, and write `Aggro` only if it succeeded.

**Tech Stack:** Go, `testify/assert` + `require`, `internal/state` machine framework, mob YAML.

**Spec:** [`2026-08-29-u12-unified-targeting-design`](../specs/2026-08-29-u12-unified-targeting-design.md) §6.1b

**Branch:** `feature/u12c-0b-load-bearing-vetoes` (already exists, rebased on master, holds the spec commits).

---

## 0. Facts verified against source

Read at master `32813de28` on 2026-08-29.

| Fact | Value |
|---|---|
| Commit lands from | **Idle and Engaged only.** `Engaging` and `Disengaging` are refused by the table — probed directly |
| U12c-0's gap | It added `Engaged → Engaging` but not `Engaging → Engaging`, and its regression test covered only the `Engaged` case |
| Why that is harmless today | `SetAggro` discards the transition error (`combat_state_compat.go:133`, "Errors are intentionally ignored") |
| The six vetoes on this transition | self non-combatant · self crafting/salvaging · self dead · target non-combatant · target dead · target presence (incl. `NoAggroTarget` respawn grace). **Position is NOT among them** |
| Where they are registered | `internal/hooks/CombatPhase_Vetoes.go`, via `characters.OnCharacterCreated` |
| `SetAggro` already refuses to write | Yes, twice: the grace-period guard (`:85`) and the taunt-hold guard (`:94`). Refusing on a veto is consistent with its own shape |
| Craft-schedule NPCs | **24**, of which **23** carry `non_combatant: true` |
| The exception | `_datafiles/world/dogmud/mobs/ashwick/259-delia.yaml` — has `hostile: false`, `charm_immune: true`, `behavior_archetype: noncombat_questgiver`, but NOT `non_combatant: true` |
| Damage already frees a craft | `cancelCraftOrSalvageOnDamage`, wired at `NewRound_DoCombat_helpers.go:1158` |
| Flee authority | `handlePlayerFlee` reads `IsDisengaging()` FIRST; its `Aggro.Type == Flee` branch is the documented *"legacy path: Aggro-only set (no CombatPhase wired)"* fixture fallback |
| Circular assumption | `Activity_Cascades.go:14-19` deleted its combat-entry cancel because "the veto handles it"; `activity.TriggerCombatInterrupt` has **zero** production uses |

---

## Task 1: Finish the transition table

**Files:**
- Modify: `internal/state/combatphase/transitions.go`
- Modify: `internal/state/combatphase/retarget_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/state/combatphase/retarget_test.go`:

```go
// TestRetarget_WhileStillWindingUp closes the gap U12c-0 left. That slice made
// Engaged -> Engaging legal and tested only that case; a retarget during the
// WIND-UP is the same situation one state earlier and was still refused.
//
// It is harmless while SetAggro discards the error, and fatal the moment
// U12c-0b makes refusals real: a mistyped target would lock the actor in until
// the wind-up finished.
func TestRetarget_WhileStillWindingUp(t *testing.T) {
	m := NewMachine()
	require.NoError(t, m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(100), RoundsUntil: 5},
		state.TransitionReason{Trigger: TriggerAttackCommand}))
	require.Equal(t, Engaging, m.State(), "fixture must still be winding up")

	err := m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(200), RoundsUntil: 5},
		state.TransitionReason{Trigger: TriggerAttackCommand})

	require.NoError(t, err, "a retarget during the wind-up must be legal")
	require.Equal(t, retargetTestActor(200), m.CurrentTarget())
}

// Fleeing stays a commitment: Disengaging -> Engaging is deliberately NOT in
// the table. This is a bug fix rather than a behaviour change, because
// handlePlayerFlee reads IsDisengaging() first and authoritatively -- a
// re-aggro mid-flee never affected whether the flee succeeded, it only
// overwrote Aggro, which silently defeats combat_retarget.go's
// `Aggro.Type != Flee` guard.
func TestRetarget_WhileDisengagingIsRefused(t *testing.T) {
	m := NewMachine()
	engageFully(t, m, retargetTestActor(100), 1)
	require.NoError(t, m.TransitionToDisengaging(
		state.TransitionReason{Trigger: TriggerFleeCommand}))
	require.Equal(t, Disengaging, m.State())

	err := m.TransitionToEngaging(
		EngagingData{Target: retargetTestActor(200), RoundsUntil: 1},
		state.TransitionReason{Trigger: TriggerAttackCommand})

	require.Error(t, err, "fleeing is a commitment; re-engaging must be refused")
}
```

- [ ] **Step 2: Run to verify the first fails and the second passes**

Run: `go test ./internal/state/combatphase/ -run 'TestRetarget_WhileStillWindingUp|TestRetarget_WhileDisengagingIsRefused' -v`
Expected: `WhileStillWindingUp` FAILS with `state: transition not allowed by table`; `WhileDisengagingIsRefused` PASSES already (it pins behaviour we are keeping).

- [ ] **Step 3: Add the transition**

In `internal/state/combatphase/transitions.go`, change the `Engaging` row:

```go
	// Engaging on RETARGET during the wind-up (U12c-0b). U12c-0 added the
	// Engaged case and missed this one; both are "switching targets takes a
	// moment", one state apart.
	Engaging: {Engaged, Idle, Engaging}, // Idle on cancel/target-died
```

Leave the `Disengaging` row alone. Fleeing is a commitment; the existing
`Disengaging: {Idle, Engaged}` already provides the way back on flee failure.

- [ ] **Step 4: Run both tests**

Run: `go test ./internal/state/combatphase/ -run TestRetarget -v`
Expected: all six PASS (the four from U12c-0 plus these two).

- [ ] **Step 5: Commit**

```bash
git add internal/state/combatphase/
git commit -m "fix(combatphase): allow retarget during the wind-up too"
```

---

## Task 2: Fix the one attackable crafting NPC

Must land before Task 3. Delia is what makes the activity veto harmful: an attackable NPC who crafts, whose retaliation would be refused.

**Files:**
- Modify: `_datafiles/world/dogmud/mobs/ashwick/259-delia.yaml`

- [ ] **Step 1: Add the flag**

Insert after the `charm_immune: true` line (line 7), matching the placement her correctly-flagged siblings use (e.g. `greenford/9590-aldo_the_smith.yaml:11`):

```yaml
non_combatant: true
```

The surrounding lines become:

```yaml
hostile: false
charm_immune: true
non_combatant: true
maxwander: 0
```

- [ ] **Step 2: Verify she is now consistent with every other craft-schedule NPC**

```bash
for s in $(grep -rl "activity: craft" _datafiles/world/dogmud/schedules/); do
  sid=$(basename $s .yaml)
  f=$(grep -rl "schedule_id: *$sid\$" _datafiles/world/dogmud/mobs/ | head -1)
  [ -n "$f" ] && ! grep -q 'non_combatant: true' "$f" && echo "STILL ATTACKABLE: $f"
done
echo "(no output = all 24 craft-schedule NPCs are non-combatant)"
```

Expected: no output.

- [ ] **Step 3: Boot test — YAML changes only fail at startup**

A mob YAML edit cannot be validated by `go build`. Use the isolated worktree; exit code 124 is the success case.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Clean up with `git worktree remove --force C:/tmp/dogmud-boot-check`; if Windows holds a lock, `rm -rf` then `git worktree prune`.

- [ ] **Step 4: Commit**

```bash
git add _datafiles/world/dogmud/mobs/ashwick/259-delia.yaml
git commit -m "fix(content): Delia is a crafting NPC and must be non-combatant"
```

---

## Task 3: Make the refusal real

**Files:**
- Modify: `internal/characters/combat_state_compat.go:82-149` (`SetAggro`)
- Create: `internal/characters/aggro_veto_test.go`

- [ ] **Step 1: Write the failing test**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/require"
)

// A refused transition must refuse the whole commit. Before U12c-0b, SetAggro
// wrote Aggro and then discarded the transition error, so a vetoed commit left
// Aggro holding a target the machine had rejected -- the two stores
// disagreeing by construction.
func TestSetAggro_RefusedTransitionWritesNothing(t *testing.T) {
	c := New()
	c.SetAggro(0, 100, DefaultAttack)
	require.NotNil(t, c.Aggro, "the first commit must land")
	require.Equal(t, 100, c.Aggro.MobInstanceId)

	// Veto every subsequent target, as a dead or non-combatant target would.
	c.CombatPhase.RegisterTargetLifeCheck(func(state.ActorRef) bool { return false })

	c.SetAggro(0, 200, DefaultAttack)

	require.Equal(t, 100, c.Aggro.MobInstanceId,
		"a refused commit must leave the previous target intact, not overwrite it")
	require.Equal(t, 100, c.CurrentCombatTarget().MobInstanceId,
		"and the two stores must still agree")
}

// The nil-CombatPhase path is the legacy/fixture one and must keep writing,
// or every test that builds a bare Character loses the ability to set a target.
func TestSetAggro_NilCombatPhaseStillWrites(t *testing.T) {
	c := New()
	c.CombatPhase = nil

	c.SetAggro(0, 100, DefaultAttack)

	require.NotNil(t, c.Aggro, "with no machine there is nothing to refuse")
	require.Equal(t, 100, c.Aggro.MobInstanceId)
}

```

**No test for the grapple-clear ordering, deliberately.** The move is still
correct — a commit that was refused must not clear grapple for a switch that
never happened — but `ClearGrappleState()` is currently a **no-op**
(`combat_state_compat.go`: *"FSM owns position state; no legacy fields to
clear"*). A test asserting it was not called would assert nothing at all. The
ordering is defensive against the day it does something again, and the code
comment says so; that is the honest amount of coverage for it.

- [ ] **Step 2: Run to verify the first fails**

Run: `go test ./internal/characters/ -run TestSetAggro_ -v`
Expected: `TestSetAggro_RefusedTransitionWritesNothing` FAILS — `Aggro.MobInstanceId` is 200, because the write happens before the discarded transition. `TestSetAggro_NilCombatPhaseStillWrites` PASSES.

- [ ] **Step 3: Invert `SetAggro`**

Replace everything from the grapple-clear comment to the end of the function. The two early guards at the top (grace period, taunt hold) stay exactly as they are.

```go
	var combatAddlWaitRounds int = 0

	if len(roundsWaitTime) > 0 {
		for _, waitAmt := range roundsWaitTime {
			combatAddlWaitRounds += waitAmt
		}
	} else {
		combatAddlWaitRounds = c.Equipment.Weapon.GetSpec().WaitRounds + c.Equipment.Offhand.GetSpec().WaitRounds
	}

	if aggroType == DefaultAttack {
		if c.Equipment.Weapon.GetSpec().Subtype == items.Shooting {
			aggroType = Shooting
		}
	}

	// U12c-0b: the transition decides. It used to run AFTER the Aggro write
	// with its error discarded, so a vetoed commit left Aggro holding a target
	// the machine had rejected and the two stores disagreed by construction.
	//
	// Refusing to write here is consistent with this function's own shape: it
	// already returns without writing for the grace-period guard and the
	// taunt-hold guard above.
	//
	// A nil CombatPhase is the legacy/fixture path — there is nothing to
	// refuse, so the write proceeds as it always did.
	if c.CombatPhase != nil {
		trigger := combatphase.TriggerAttackCommand
		if aggroType == SurpriseAttack {
			trigger = combatphase.TriggerSurpriseAttack
		}
		if err := c.CombatPhase.TransitionToEngaging(combatphase.EngagingData{
			Target: state.ActorRef{
				UserId:        userId,
				MobInstanceId: mobInstanceId,
			},
			RoundsUntil: combatAddlWaitRounds,
		}, state.TransitionReason{
			Trigger: trigger,
			Actor:   state.ActorRef{UserId: c.userId},
			Target:  state.ActorRef{UserId: userId, MobInstanceId: mobInstanceId},
		}); err != nil {
			return
		}
	}

	// Clear grapple state if switching targets. AFTER the transition, so a
	// commit that was refused does not clear grapple for a switch that never
	// happened.
	if c.Aggro != nil {
		if c.Aggro.UserId != userId || c.Aggro.MobInstanceId != mobInstanceId {
			c.ClearGrappleState()
		}
	}

	c.Aggro = &Aggro{
		UserId:        userId,
		MobInstanceId: mobInstanceId,
		Type:          aggroType,
		RoundsWaiting: combatAddlWaitRounds,
	}
}
```

Delete the original grapple-clear block from its old position near the top of the function, and delete the old "Dual-write ... Errors are intentionally ignored" comment along with the trailing `_ = c.CombatPhase.TransitionToEngaging(...)` call.

- [ ] **Step 4: Run the new tests**

Run: `go test ./internal/characters/ -run TestSetAggro_ -v`
Expected: both PASS.

- [ ] **Step 5: Run every suite that exercises combat**

Run: `go build ./... && go test ./internal/characters/ ./internal/targeting/ ./internal/combat/ ./internal/actions/ ./internal/hooks/ ./internal/behaviortree/ ./internal/mobcommands/ ./internal/usercommands/`
Expected: all PASS.

⚠️ **If a test now fails because a commit is refused, STOP and report which veto fired.** That is the whole risk of this slice, and the answer is a decision about that veto, not a test edit.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/
git commit -m "fix(characters): a refused transition now refuses the commit"
```

---

## Task 4: Full verification, patch notes, boot, PR

**Files:**
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Full sweep**

Run: `gofmt -l internal/ modules/ main.go && go build ./... && go test ./internal/...`
Expected: gofmt silent, build OK, everything green.

- [ ] **Step 2: Add a dated patch-notes entry**

Insert at the top of `docs/PATCH_NOTES.md`, under the `# DOGMud Patch Notes` heading:

```markdown
## 2026-08-29: Fights start only when they are allowed to

The rules about who can be drawn into a fight are now actually enforced.

The game has always had a list of reasons a fight should not start: the target
is already dead, one of you is not a fighter, someone just respawned and is
still protected. Those rules were written down but never consulted, so a fight
could begin anyway and then behave oddly, because half the game thought it had
started and half did not.

They are consulted now. In almost every case you will notice nothing, because
the situations they cover are ones the game already refused for other reasons.

Two related changes come with it. Switching targets while you are still winding
up a blow now works, where before it quietly did nothing until the blow landed.
And fleeing is now a commitment: you cannot be dragged back into a fight you
are running from, though a failed escape still puts you back in it.

One villager in Ashwick was mistakenly marked as a valid target despite being a
crafter and no fighter. She is not any more.
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

- [ ] **Step 4: Push and open the PR**

```bash
git push -u origin feature/u12c-0b-load-bearing-vetoes
gh pr create --repo pruuk/DOGMud --base master --head feature/u12c-0b-load-bearing-vetoes --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

⚠️ After merging, **watch `Build and release` on master**, not just the PR checks. PR #83 was green and master went red, because `run-tests.yml` runs on PRs and `build-and-release.yml` runs on master.

---

## Done when

1. A commit lands from `Idle`, `Engaging` and `Engaged`, and is refused from `Disengaging`.
2. A vetoed commit writes nothing: `Aggro` keeps the previous target and the two stores agree.
3. A nil `CombatPhase` still writes, so fixtures keep working.
4. Grapple state is cleared only after a commit is accepted (untested by design; the call is currently a no-op).
5. All 24 craft-schedule NPCs are `non_combatant: true`.
6. `go test ./internal/...` green, boot clean, PR green, **and master green after merge**.

## Deliberately NOT in this slice

- **No collapsing the six vetoes into two.** Restructuring a mechanism in the same slice that makes it load-bearing means a failure could be either the policy or the plumbing. The collapse actually worth doing is spec §8.1: `combatphase`'s target vetoes and `mobs.CheckPlayerHarm` are two incomplete answers to "is this a legal target", and neither covers charmed companions plus dead targets.
- **No implementing `activity.TriggerCombatInterrupt`.** It has zero production uses and is the residue of the cascade `Activity_Cascades.go` deleted. Whether combat entry should cancel a craft is a gameplay question; damage already cancels it, which is what makes the activity veto safe here.
- **No playtest.** The arc's adversarial playtest belongs to U12c-2.
