# #514 Every Attack Interrupts a Craft Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Any real health damage from an attack ends a craft or salvage in progress, from one seam (`Character.ApplyHarm`), while attrition does not.

**Architecture:** `ApplyHarm` delegates to an unexported `harm(..., interrupt bool)`; a new `ApplyAttrition` passes `interrupt` false. The interrupt is an Activity transition with `TriggerDamageInterrupt`, so the existing craft-ended hook narrates the room; a new salvage-ended callback lets the crafter's and salvager's own lines move out of the hand-placed helper, which is deleted.

**Tech Stack:** Go; `internal/characters`, `internal/state/activity`, `internal/hooks`.

**Spec:** `docs/superpowers/specs/completed/2026-10-13-514-damage-interrupts-crafting-design.md` (facts table at the top; read it first).

---

## Rules for every task (restate in every subagent brief)

- Edit/Write tools only for text edits. No `sed -i`, no shell heredoc appends, no Python read-modify-write, no `gofmt -w`.
- Named-path `git add` only. No em or en dashes, no spaced ` - ` or `--`.
- After every task run the touched packages' tests AND `go test .` (root guards).
- Worktree `C:/tmp/dogmud-366-facts`, branch `fix/514-damage-interrupts-crafting`. Do not push. Do not start or stop servers or processes.
- Commit messages end with a blank line then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

### Task 1: The seam in `characters` (TDD)

**Files:** `internal/characters/pools.go`; test `internal/characters/harm_interrupt_test.go` (new).

- [ ] **Step 1: Failing tests.** Read existing `internal/characters/*_test.go` for how a test `Character` is built (`New()` gives a fresh `Activity` machine) and how health is set. Tests:
  - crafting character, `ApplyHarm(PoolHealth, 5, state.ActorRef{})` with health well above 5: activity is Free afterwards, and an `Activity.OnCraftEnded` handler registered in the test saw trigger `activity.TriggerDamageInterrupt`;
  - same for a salvaging character (assert Free; the salvage callback does not exist yet, so assert state only);
  - `ApplyAttrition(PoolHealth, 5, state.ActorRef{})`: still Crafting, health reduced by 5 exactly as `ApplyHarm` would;
  - `ApplyHarm(PoolConviction, 5, ...)`: still Crafting;
  - `ApplyHarm(PoolHealth, 0, ...)`: still Crafting.
  Run: FAIL (`ApplyAttrition` undefined).
- [ ] **Step 2: Implement.** Move `ApplyHarm`'s body into `func (c *Character) harm(pool Pool, amount int, source state.ActorRef, interrupt bool) int`. `ApplyHarm` returns `c.harm(pool, amount, source, true)`; add `ApplyAttrition` returning `c.harm(pool, amount, source, false)` with a doc comment citing #514 R2 and the pinnacle-toll rule. In `harm`, after the vital change and after the existing death-queue block, add:

```go
	// #514: an attack ends any work in hand. Health only, and only harm that
	// actually landed. It runs after the vital change, so a killing blow is
	// already at zero health when the craft-ended hook reads it.
	if interrupt && pool == PoolHealth && applied > 0 {
		c.interruptWork()
	}
```

  and

```go
// interruptWork ends a craft or salvage in progress because the character was
// hurt (#514). The Activity machine's craft-ended and salvage-ended callbacks
// narrate it.
func (c *Character) interruptWork() {
	if c.Activity == nil {
		return
	}
	switch c.Activity.State() {
	case activity.Crafting, activity.Salvaging:
		_ = c.Activity.TransitionToFree(state.TransitionReason{
			Trigger: activity.TriggerDamageInterrupt,
			Actor:   c.ActorRef(),
		})
	}
}
```

  Confirm `internal/characters` already imports `internal/state/activity` (the `Activity` field's type); if not, add it and confirm no import cycle with `go build ./...`. Keep `ApplyHarm`'s existing doc comment on `ApplyHarm`. Run Step 1 tests: PASS. `go test ./internal/characters/`: PASS.
- [ ] **Step 3: Commit** `internal/characters/pools.go`, `internal/characters/harm_interrupt_test.go`.

### Task 2: Salvage-ended callback in `activity` (TDD)

**Files:** `internal/state/activity/activity.go`, `rules.go`; test `internal/state/activity/craft_ended_test.go` (extend).

- [ ] **Step 1: Failing tests,** mirroring the four existing `OnCraftEnded` tests plus the vetoed case: `OnSalvageEnded` fires once with the `SalvagingData` snapshot and the reason on `TransitionToFree` and on `ForceFree` out of Salvaging; never for Crafting or Casting; never on a vetoed transition; never when already Free. Run: FAIL.
- [ ] **Step 2: Implement** exactly as `OnCraftEnded` is built (read it): a `salvageEnded` handler slice, `OnSalvageEnded`, `fireSalvageEnded`, and the capture-then-fire in `TransitionToFree` and `ForceFree` (capture `m.salvaging` and `wasSalvaging` alongside the craft capture). Run: PASS; `go test ./internal/state/...`: PASS.
- [ ] **Step 3: Commit.**

### Task 3: Hooks: crafter and salvager lines move; helper deleted; attrition callers switch

**Files:** `internal/hooks/Activity_CraftEnded.go`, new `internal/hooks/Activity_SalvageEnded.go`, `internal/hooks/combat_shared_helpers.go`, `NewRound_DoCombat_helpers.go`, `NewRound_UserRoundTick.go`, `NewRound_MobRoundTick.go`, `pinnacle_tick.go`, `NewRound_AutoHeal.go`, `internal/mobcommands/pathto.go`; tests `internal/hooks/activity_craft_ended_test.go`.

- [ ] **Step 1: Tests first.** In `activity_craft_ended_test.go`, change every call of `cancelCraftOrSalvageOnDamage(x)` to `x.ApplyHarm(characters.PoolHealth, 1, state.ActorRef{})` (health is above 1 in those fixtures; check), keeping each test's expectations: a player crafter reads exactly "The pain interrupts your crafting." and the watcher the stop line; a salvaging player reads "The pain interrupts your salvaging." and the watcher nothing; a mob crafter is told nothing; a killing blow (health 1, harm 5) gives NO room line and NO crafter line. Add a test that a skill-move hit interrupts: a crafting player is the target of `combat.ExecuteSkillMove` (read `internal/combat/skill_moves.go` `SkillMoveParams` and an existing test that drives it, e.g. grep `ExecuteSkillMove(` in `*_test.go`) with a move that deals damage; afterwards the player's Activity is Free. If driving `ExecuteSkillMove` in a hooks test needs a heavy fixture, put that test in `internal/combat` and assert only the Activity state. Run: FAIL (crafter line still in the helper; skill move does not interrupt yet only if Task 1 is not in, so after Task 1 the skill-move test may already pass: note it).
- [ ] **Step 2: Move the crafter line** into `wireCraftEndedNarration`'s handler: after the existing dying check, if `r.Trigger == activity.TriggerDamageInterrupt` and the crafter is a player (`ref.UserId > 0` and the user resolves), send `<ansi fg="red">The pain interrupts your crafting.</ansi>` on `messaging.CategorySystem` to that user, in addition to the room line. Keep the room line exactly as today.
- [ ] **Step 3: Salvage hook.** Create `internal/hooks/Activity_SalvageEnded.go`: `wireSalvageEndedNarration(c)` registered via `characters.OnCharacterCreated` (as `Activity_CraftEnded.go` does), calling `c.Activity.OnSalvageEnded(...)`; on trigger `TriggerDamageInterrupt`, for a player who is not dying (`c.Health >= 1 && c.IsAlive()`), send `<ansi fg="red">The pain interrupts your salvaging.</ansi>` on `CategorySystem`. Nothing for any other trigger (movement already tells the salvager in `usercommands/go.go`), nothing to the room (salvage has no room lines yet).
- [ ] **Step 4: Delete** `cancelCraftOrSalvageOnDamage` and its three calls (melee helper `if roundResult.DamageToTarget > 0 { ... }` block, the player tick, the mob tick). Update the `handlePlayerConcentrationBreak` doc comment, which says it "hard-cancels any in-progress crafting or salvaging": it no longer does; the damage seam does.
- [ ] **Step 5: Attrition callers** switch `ApplyHarm(characters.PoolHealth, ...)` to `ApplyAttrition(characters.PoolHealth, ...)` with the same arguments: `pinnacle_tick.go` (blade toll), `NewRound_AutoHeal.go` (toxicity sickness), `mobcommands/pathto.go` (stuck-mob decay). Add a one-line comment at each: attrition, not an attack (#514 R2). Add a test that a pinnacle-toll-style `ApplyAttrition` on a crafting player leaves the craft running (Task 1 covers the primitive; one hooks-level assertion via the toxicity or toll function is enough if a test seam exists; otherwise skip and say so).
- [ ] **Step 6: Run** `go test ./internal/hooks/ ./internal/mobcommands/ ./internal/characters/ ./internal/combat/ .` and fix any viewpoint-registry or other root-guard fallout (read each report against source). Commit.

### Task 4: Root guard, docs, gate

- [ ] **Step 1: Guard** `damage_interrupt_seam_guard_test.go` (repo root), modelled on `craft_phase_narration_guard_test.go`: walk `internal` and `modules` non-test Go files; any line containing `TriggerDamageInterrupt` outside `internal/characters/` and `internal/state/activity/` fails, except reads in `internal/hooks/Activity_CraftEnded.go` and `internal/hooks/Activity_SalvageEnded.go` (comparisons, not transitions: allow those two files). Prove it fails by temporarily adding a `_ = activity.TriggerDamageInterrupt` line to `internal/hooks/combat_shared_helpers.go`, then revert.
- [ ] **Step 2: Docs.** `internal/characters/context.md`: `ApplyAttrition`, the interrupt in `ApplyHarm`. `internal/state/activity/context.md`: `OnSalvageEnded`. `internal/hooks/context.md`: `Activity_SalvageEnded.go`; the damage-cancel helper is gone. `docs/PATCH_NOTES.md`: "Any attack now interrupts crafting or salvaging, not just an ordinary swing: a charge, a kick, a spell or a thrown blade all break your concentration on the work." File a follow-up issue (search first, `gh issue list --repo pruuk/DOGMud`) for `cancelDamageConditions` (wake a sleeper, drop cancel-on-damage conditions) having the same hand-placed shape. Add this spec and plan to `docs/README.md`.
- [ ] **Step 3: Gate**, each standalone: `gofmt -l internal/ modules/` and `gofmt -l *.go` from Bash; `go build ./...`; `go vet ./...`; `go test ./... > <scratch>/gotest.log 2>&1` then grep `--- FAIL` (report names verbatim; rerun a failing package once); `golangci-lint run --new-from-merge-base=origin/master ./...`. Commit.

### Task 5: Review, boot, PR, playtest, merge

- [ ] Whole-branch review against the spec; fix findings.
- [ ] Isolated boot per `dogmud-shipping` (stop by PID).
- [ ] Push; `gh pr create --repo pruuk/DOGMud`, body "Fixes #514", rulings R1 R2, the follow-up issue number.
- [ ] Playtest: a crafter gored by a Tusked Forest-Boar (9539) mid-craft: room reads the stop line, crafter reads "The pain interrupts your crafting."; a crafter hit by a damaging spell, same; normal completion unchanged.
- [ ] Merge on green with the owner's go-ahead.
