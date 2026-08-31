# U3: Ranged, Taunt and the Maneuver Family onto the Contest Core — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move every remaining **floored** opposed roll — the whole `ManeuverFloors()` family, ranged, taunt and conviction defence — onto `internal/contest`, changing no behaviour.

**Architecture:** `internal/combat` gains two thin wrappers, `RunWithManeuverFloors` and `RunWithSpellFloors`, that pair the existing floor accessors with `contest.RunWithFloors`. Eight maneuver/taunt/ranged sites and the six already-migrated spell sites then read the floors from one place instead of nine, and `internal/hooks`'s four private floor accessors are deleted.

**Tech Stack:** Go, `internal/contest`, `internal/combat`, `internal/dice`, `testify/assert`.

**Status:** amended 2026-08-12 after a three-reviewer adversarial pass (behavioural-equivalence, executability, scope/design). Their corrections are folded in below; where a finding changed a decision it is marked **[review]**.

---

## Prerequisite state (verified 2026-08-12 against `f36a44423`)

U1 shipped `internal/contest`; U2 added `RunWithFloors`, `Success` and `Floored`. `Result.Margin` is **attack-positive**. Melee (U1) and the six spell sites (U2) are migrated.

### The scope decision the roadmap deferred — RESOLVED

The roadmap flagged `ExecuteSkillMove` as "the awkward one": `combat_fire.go` folds ranged defence into a scalar and calls it, so migrating ranged appeared to force either touching melee's bash/kick/trip or forking ranged out.

**Neither. Migrate the single roll inside `ExecuteSkillMove` and every caller comes with it.** The function has **fourteen** production call sites, not three (verified: `grep -rn "ExecuteSkillMove(combat.SkillMoveParams{" internal/ --include=*.go | grep -v _test.go | wc -l` → 14):

`actions/`: `combat_bash.go:92`, `combat_drain.go:95` + `:242`, `combat_fire.go:190`, `combat_gore.go:81`, `combat_hamstring.go:86`, `combat_kick.go:135`, `combat_maul.go:87`, `combat_pounce.go:96`, `combat_rake.go:87`, `combat_throttle.go:94`, `combat_trip.go:114`; `hooks/combat_shared_helpers.go:239` + `:282`. One is ranged, thirteen are melee.

Forking ranged out would duplicate the damage pipeline, knockdown roll, control-immunity check and FSM transition block — about 90 lines — to change one roll that is **identical** on both paths. The roll reads only `attackSuccess`; it reads no roll field at all. So one edit migrates ranged and the entire special-move family together, as a provable no-op, with nothing forked and nothing left half-converted.

**[review] This does not make U6's job harder.** U6's ranged/melee split is a split in *score construction*, which lives in the fourteen callers — `combat_fire.go:193-201` folds `rangedDefenseScore` into `DefenseSkill` with `DefenseStat: 0`, while `combat_bash.go:95-98` passes stat and skill separately. It is not a split in the roll. So the correct U3 preparation is neither a fork nor widening `SkillMoveResult`; it is making the parameter contract explicit in a doc comment (Task 6 Step 1).

`rangedDefenseScore`'s ×1 skill weight, its flat `RangedShieldDefenseBonus`, and its `DefenseStat: 0` are **preserved exactly**. U6 fixes them; U3 must not.

### The eight sites this plan migrates

| # | Site | Fields read beyond `success` | Risk |
|---|---|---|---|
| 1 | `combat/skill_moves.go:62` (`ExecuteSkillMove`) | none | none — covers ranged + 13 melee callers |
| 2 | `usercommands/throw.go:154` | `atkRoll.ZScore` (`:157`) | none — `ZScore` is populated by `dice.Roll` |
| 3 | `actions/combat_taunt.go:129` | `atkRoll.ZScore` (`:142`), `attackSuccess` (`:169`), `atkMargin` + `atkRoll` (`:184`) | margin → `res.Margin`, unnegated |
| 4 | `combat/avoidance.go:81` (`TryStoicResolve`) | **`defRoll.Margin`** (`:89`) | **TRAP 1 — silently reads zero** |
| 5 | `combat/grapple.go:80` | `margin`, both `.Value`, both `.ZScore` | none |
| 6 | `combat/submission.go:80` | `margin`, both `.ZScore` | none |
| 7 | `hooks/Position_GrappleTick.go:270` | `margin`, `atkRoll.ZScore`, `atkRoll.StdDev`, `defRoll.ZScore` | none — `StdDev` is populated |
| 8 | `hooks/NewRound_MobRoundTick.go:398` | none | none |

Sites 5–8 are the four the roadmap listed as **UNOWNED**, plus the charm-duration reroll it listed as unclaimed. U3 claims all five. Site 4 carries a `TODO(U3)` breadcrumb left by U2.

**Verified complete:** `grep -rn "ManeuverFloors()\|maneuverHitFloor()\|maneuverResistFloor()" internal/ --include=*.go` returns 13 lines: these eight call sites, the `combat.ManeuverFloors` definition at `contest_floors.go:18` (`SpellFloors` does not match this pattern), the two `hooks` accessor definitions at `spell_resolution.go:276`/`:280`, and `flee.go:84`/`:107` — which is **U4's**, and must not be touched here. All three reviewers re-ran this sweep independently and found nothing missed.

## Why two wrappers rather than eight inline pairs

Today nine sites each write the same two lines: fetch a floor pair, pass it to a roll. Two of them fetch it through a *different, private* accessor in `hooks` that reads the same config keys — which is precisely why the roadmap's first sweep missed `Position_GrappleTick.go`: a grep for `ManeuverFloors()` does not find `maneuverHitFloor()`.

```go
// internal/combat/contest_floors.go
func RunWithManeuverFloors(attackScore, defenseScore float64) contest.Result
func RunWithSpellFloors(attackScore, defenseScore float64) contest.Result
```

Each is two lines: read the floor pair, call `contest.RunWithFloors` with a single unnamed entry. That kills the grep-blindness permanently and gives U6 **one** place to change the floor model per pair instead of nine.

**[review] They are named for the FLOOR PAIR, not for a channel — deliberately.** An earlier draft called them `RunManeuverContest`/`RunSpellContest` and claimed they made "which channel is this contest?" a property of the call. That claim was false and the name was actively misleading: `TryStoicResolve` is the **conviction** channel and uses the *maneuver* floor pair, and the maneuver pair's other callers span five different skill-weight regimes (see the table in Task 6 Step 2). A U6 implementer reading `RunManeuverContest` would reasonably assume its callers share a channel and can be retuned together. They cannot. `RunWithManeuverFloors` says exactly what it does and no more.

**`RunWithSpellFloors` also re-homes U2's six sites.** That is deliberate scope, not creep: leaving the spell channel inline while the maneuver channel gets a wrapper would ship exactly the half-converted asymmetry this arc exists to remove, and it is six one-line edits that delete two more dead accessors. All six read only `.Success`, `.Margin` and `.AttackRoll`, every one of which `RunWithSpellFloors` returns identically.

The wrappers live in `combat`, not `contest`, because `contest` is a leaf that deliberately reads no config — that purity is what makes it testable without `config.yaml`. Verified there is no cycle or new edge: `go list -deps ./internal/combat/` shows `combat` importing none of `actions`, `hooks`, `usercommands`; all three already import `combat` (`throw.go:153` already calls `combat.ManeuverFloors()`); `combat` already imports `contest`.

> The comment above `maneuverHitFloor` claims the accessors exist because "hooks does not import internal/combat". That has been false since `combat_shared_helpers.go` landed. Deleting the accessors deletes the stale claim with them.

## Three traps specific to this plan

**TRAP 1 — `TryStoicResolve` reads `defRoll.Margin`, which the core does not populate.**
`dice.OpposedRoll` stamps `.Margin`/`.Success` onto the returned rolls; `dice.Roll`, which the core uses, does not. Under the core that field is **zero**, `ContestCrit` sees z ≈ 0, and stoic resolve silently never fully negates again — no test fails, no compile error. This is the same defect that nearly shipped in `TrySpellDeflection` during U2. Read `Result.Margin`; never a roll's `.Margin`.

**TRAP 2 — the sign differs between sites 3 and 4, and both compile.**
`Result.Margin` is attack-positive.
- Site 3 (taunt) is the **attacker's** crit check: pass `res.Margin` **unnegated** to `AttackContestCrit`. It currently passes `atkMargin`, which is already attack-positive — so this is a rename, not a sign change.
- Site 4 (stoic resolve) is the **defender's** crit check: pass `-res.Margin` to `DefenseContestCrit`, exactly as migrated `TrySpellDeflection` does. It currently passes `defRoll.Margin`, and `dice.OpposedRoll:115` sets `defenseRoll.Margin = -margin` in every branch including both floor flips, so `-res.Margin` reproduces it exactly.

Get either backwards and crit lands on the losing side, silently.

**[review] TRAPS 1 AND 2 HAVE NO EXISTING TEST COVERAGE.** This is the single most dangerous fact about this plan. `internal/combat/avoidance_test.go` asserts only `mult >= 0.0` (`:22`, `:57`) — nothing asserts a full negation (`0.0`) ever occurs. `internal/actions` has **no taunt test at all**. `internal/usercommands/throw_test.go` covers only `maybeInterruptOnThrow`, never the roll. So a migration that reads the zero field, or flips a sign, leaves `go test ./...` **fully green**. Task 1b exists solely to close this, and success criterion 1 is worded to permit it.

**TRAP 3 — `ExecuteSkillMove` is load-bearing for fourteen callers.**
Its roll currently discards margin and both rolls. Do not "improve" it by surfacing them; that is U6's decision and widening `SkillMoveResult` here invites callers to depend on fields whose semantics are about to change. Change the two lines that roll, nothing else.

## File Structure

| File | Responsibility |
|---|---|
| `internal/combat/contest_floors.go` (modify) | add `RunWithManeuverFloors` + `RunWithSpellFloors` |
| `internal/combat/contest_floors_test.go` (new) | wrappers delegate correctly and read the right knobs |
| `internal/combat/contest_sign_test.go` (new) | **TRAP 1 + TRAP 2 regression coverage** |
| `internal/combat/skill_moves.go` (modify) | site 1 |
| `internal/usercommands/throw.go` (modify) | site 2 |
| `internal/actions/combat_taunt.go` (modify) | site 3 |
| `internal/combat/avoidance.go` (modify) | site 4 + `TrySpellDeflection` onto the wrapper; delete the `TODO(U3)` |
| `internal/combat/grapple.go` (modify) | site 5 |
| `internal/combat/submission.go` (modify) | site 6 |
| `internal/hooks/Position_GrappleTick.go` (modify) | site 7 |
| `internal/hooks/NewRound_MobRoundTick.go` (modify) | site 8 |
| `internal/hooks/spell_resolution.go` (modify) | **four** spell sites onto the wrapper; **delete all four floor accessors** |
| `internal/hooks/charm_spell.go` (modify) | `resolveCharmSpell` (the fifth spell site) onto the wrapper |
| `internal/combat/margin_crit.go` (modify) | **comment only** — it currently teaches TRAP 1 |
| `internal/combat/context.md` (modify) | wrappers + correct two stale claims |
| `internal/hooks/context.md` (modify) | drop the deleted accessors if named |
| `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md` (modify) | ownership discharge + a new U6 modelling gate |
| `docs/superpowers/specs/completed/2026-08-12-unified-contest-resolution-design.md` (modify) | correct the false ranged skill-weight row |

## Success criteria

1. `go test ./...` passes with **no *existing* test file modified**. New test files are expected — `contest_floors_test.go` and `contest_sign_test.go`. If an *existing* test needs changing, stop: a modifier moved. **[review] The earlier wording forbade all test changes, which would have blocked the only coverage that catches TRAP 1.**
2. `gofmt -l internal/` prints nothing; `go build ./...` clean.
3. **[review]** `grep -rn "dice\.OpposedRollStatWithFloors" internal/ --include=*.go | grep -v _test.go` returns **only** `combat/flee.go:85`, `combat/flee.go:108` (U4's) and comment lines in `avoidance.go`, `margin_crit.go`, `contest/contest.go`. *The earlier criterion grepped the substring `dice.OpposedRoll`, which also matches `dice.OpposedRollStat` — it would still return ~17 sites that this plan explicitly assigns to U4, and would push an executor straight into U4's scope. Line numbers corrected too: 84/107 are the `ManeuverFloors()` fetch lines; the calls are at 85/108.*
4. No **declaration or call** of `maneuverHitFloor`, `maneuverResistFloor`, `spellHitFloor` or `spellResistFloor` survives anywhere in `internal/`.

   **[amended during execution]** The original wording demanded the grep return *nothing at all*. Task 5 deliberately kept the deleted names in the wrappers' prose — "U3 deleted that duplicate pair; do not reintroduce one" — so the grep returns two comment hits in `internal/combat/contest_floors.go`. That is **correct and is to be preserved**. Grep-blindness on `maneuverHitFloor` is exactly how the roadmap's first sweep missed `Position_GrappleTick`; a future search for the old name should land on a warning, not on silence. Verify with the build (a surviving call cannot compile once the funcs are gone) rather than with a bare name grep.
5. `internal/contest` still imports only stdlib and `internal/dice`; `internal/hooks` imports `internal/contest` **not at all** (it reaches the core through `combat`).
6. `internal/actions/combat_fire.go`'s `rangedDefenseScore` function body is unchanged (only its file's `ExecuteSkillMove` *call* is in scope, and even that is unchanged — site 1 is in `internal/combat`).
7. Boot test clean (exit 124 = success).

---

### Task 1: The two floor-pair wrappers

**Files:** Modify `internal/combat/contest_floors.go`; new `internal/combat/contest_floors_test.go`

- [ ] **Step 1: Write the failing tests**

**[review] Config mechanism, verified — do not improvise.** `configs.GetBalanceConfig()` returns `Balance` **by value**, so mutating what it returns is a no-op. `setBalanceForTest` is unexported. The correct exported hook is `configs.SetConfigForTest(t, c Config)` (`internal/configs/testing_support.go:30`), used as:

```go
c := configs.GetConfig()
c.Balance.MinManeuverHitChance = 0.5
configs.SetConfigForTest(t, c)   // self-registers t.Cleanup restore
```

It snapshots and restores via `t.Cleanup` itself, so it cannot leak into sibling tests. **Do NOT use `configs.AddOverlayOverrides`** — it has no clear/reset API (verified: no `ClearOverlay`/`ResetOverlay` exists), is package-global, and would leak a pinned floor into `grapple_test.go`'s `TestAttemptGrapple_Statistical` and the submission tests in this same package.

**[review] All four floor knobs default to `0` under test.** A Go test binary never loads `_datafiles/config.yaml`, and `validateMisc` (`internal/configs/config.balance.misc.go:167-172`) only substitutes a default when the value is `< 0 || > 0.50` — `0` is in range and stays `0`. So by default the two wrappers are **behaviourally identical** and any "assert the channels differ" test passes vacuously. The floors must be injected explicitly. This is the same class of defect as the U2 test that executed zero assertions on ~25% of runs while reporting PASS.

```go
// TestRunWithManeuverFloors_DelegatesToCore — a decisive attacker wins and the
// result is a real contest with an attack-positive margin. This is the shape
// contract every migrated site depends on.
//
// Floors are pinned to zero explicitly rather than relying on them defaulting
// that way: config.yaml ships MinManeuverResistChance 0.05, so the day someone
// adds a config-loading TestMain to this package (internal/characters already
// has one) an unpinned version of this test would flip to Success=false with
// the -1 sentinel about 1 run in 20.
func TestRunWithManeuverFloors_DelegatesToCore(t *testing.T) {
	c := configs.GetConfig()
	c.Balance.MinManeuverHitChance = 0
	c.Balance.MinManeuverResistChance = 0
	configs.SetConfigForTest(t, c)

	res := RunWithManeuverFloors(10000, 1)
	assert.True(t, res.Contested, "one entry must produce a contest")
	assert.True(t, res.Success)
	assert.Greater(t, res.Margin, 0.0, "Margin must be attack-positive")
	assert.Equal(t, "", res.Winner, "the single entry is deliberately unnamed")
	assert.Greater(t, res.AttackRoll.StdDev, 0.0, "crit normalisation needs StdDev")
}

// TestRunWithSpellFloors_DelegatesToCore — mirror of the above, pinning the
// spell pair to zero.

// TestWrappers_ReadTheirOwnFloorPair — the two wrappers must not collapse onto
// the same knob pair. Pin the maneuver hit floor to 0.5 and the spell hit floor
// to 0, run a HOPELESS attacker (score 1 vs 10000) ~300 times through each, and
// assert res.Floored fires for maneuver and never for spell.
//
// Guards a copy-paste that points both wrappers at ManeuverFloors() — which
// compiles, passes every other test, and silently retunes the spell channel.
func TestWrappers_ReadTheirOwnFloorPair(t *testing.T) { /* per the above */ }
```

- [ ] **Step 2: Implement**

```go
// RunWithManeuverFloors resolves one attack score against one defence score
// using the MANEUVER contest floor pair.
//
// It is named for the FLOOR PAIR, not for a channel, because its callers do not
// share one: grapples, submissions, skill moves, thrown weapons and taunts all
// use this pair, and so does TryStoicResolve, which is the CONVICTION channel.
// Nor do they share a skill-weight convention — see the roadmap's special-move
// weight table. Do not assume these callers can be retuned together.
//
// The single entry is deliberately unnamed — there is one defence, so there is
// nothing to name. Read Result.Contested, never Result.Winner, to ask whether a
// contest happened.
//
// TRANSITIONAL, mirroring contest.RunWithFloors: U6 may delete or reshape both.
// It exists so the floor pair is fetched in ONE place. Before it, nine sites
// fetched it themselves and two used a private duplicate in internal/hooks,
// which is how the roadmap's first sweep missed Position_GrappleTick entirely.
func RunWithManeuverFloors(attackScore, defenseScore float64) contest.Result {
	hit, resist := ManeuverFloors()
	return contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, hit, resist)
}

// RunWithSpellFloors is the spell-pair equivalent. Same transitional status and
// the same warning about callers not sharing a convention.
func RunWithSpellFloors(attackScore, defenseScore float64) contest.Result {
	hit, resist := SpellFloors()
	return contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, hit, resist)
}
```

- [ ] **Step 3: Verify** — `go test ./internal/combat/ -run 'Floors'` passes; `go build ./...` clean. Run the package's full suite too, to prove the config injection did not leak.

---

### Task 1b: Regression coverage for TRAP 1 and TRAP 2 — **write this BEFORE migrating**

**Files:** new `internal/combat/contest_sign_test.go`

**[review] This task did not exist in the first draft.** Without it, both traps ship green. Write it against the **pre-migration** code so it passes now, then confirm it still passes after Task 4 — that is what makes it a regression test rather than a restatement of the new code.

- [ ] **Step 1:** Test that `TryStoicResolve` can fully negate. Pin the maneuver floors to zero (Task 1's mechanism), build an overwhelming defender against a hopeless attacker, and assert that over N iterations the returned multiplier is `0.0` at least once — a full negation requires a defensive crit, which requires a correctly-signed non-zero margin. If the migration reads the unpopulated `.Margin` field, or negates the wrong way, this count goes to zero.

- [ ] **Step 2:** The mirror for `TrySpellDeflection` (already migrated in U2, so this also locks in U2's fix).

- [ ] **Step 3:** Pick N and the score gap from the *defensive crit* rate, not by guessing. `DefenseCritFloor()` may floor the rate; check `crit_floor.go` before choosing, and assert against a bound the test can actually meet. **A test whose assertion sits inside an `if` that is a coin flip reports PASS while executing nothing — that shipped in this arc once already.** Every assertion must execute on every run.

- [ ] **Step 4: Verify** — passes against unmodified `master` code. If it passes when it should not, the test is not exercising the sign; fix the test before proceeding.

---

### Task 2: Sites 1, 2, 5, 6 — the mechanical four (`combat` + `usercommands`)

**Files:** Modify `internal/combat/skill_moves.go`, `internal/combat/grapple.go`, `internal/combat/submission.go`, `internal/usercommands/throw.go`

These read no field the core fails to populate. Each is a two-lines-to-one replacement.

- [ ] **Step 1: `skill_moves.go:61-62`** — replace the floor fetch + roll with:

```go
	res := RunWithManeuverFloors(attackerScore, defenderScore)
	attackSuccess := res.Success
```

Change nothing else in the function. Do not widen `SkillMoveResult` (TRAP 3). **Keep the `dice` import** — `dice.RollStat` is still used at `:72` and `:87`.

- [ ] **Step 2: `grapple.go:79-80`** — `res := RunWithManeuverFloors(result.AttackScore, result.DefenseScore)`, then populate `result` from `res.Success`, `res.Margin`, `res.AttackRoll.Value`, `res.DefenseRoll.Value`, `res.AttackRoll.ZScore`, `res.DefenseRoll.ZScore`.

  **[review] `GrappleResult.AttackRoll` is a `float64` (`grapple.go:19-20`), while `contest.Result.AttackRoll` is a `dice.RollResult`.** `result.AttackRoll = res.AttackRoll` reads naturally and is wrong; it must be `.Value`. The compiler catches it, but do not waste a cycle on it. Drop the now-unused `dice` import.

- [ ] **Step 3: `submission.go:79-80`** — same shape; `Tier: ClassifySubmissionTier(res.Success, res.AttackRoll.ZScore)`, `Margin: res.Margin`, `AttackerZScore: res.AttackRoll.ZScore`, `DefenderZScore: res.DefenseRoll.ZScore`. Drop the now-unused `dice` import.

- [ ] **Step 4: `throw.go:153-154`** — `res := combat.RunWithManeuverFloors(attackerScore, defenderScore)`; the fumble check at `:157` becomes `res.AttackRoll.ZScore <= -2.0`; the branch at **`:201` is `if !attackSuccess { continue }`** — a *miss* branch, not a hit branch — and becomes `if !res.Success`. **Keep the `dice` import** — `dice.RollStat` is still used at `:213`.

- [ ] **Step 5: Verify** — `go build ./...`; `go test ./internal/combat/ ./internal/actions/ ./internal/usercommands/`. Import changes are itemised per step above; do not apply a blanket rule.

---

### Task 3: Site 3 — taunt

**Files:** Modify `internal/actions/combat_taunt.go`

The three variables from the roll are used at exactly `:142` (`atkRoll.ZScore`), `:169` (`attackSuccess`) and `:184` (`atkMargin`, `atkRoll`). Miss one and it will not compile — but know where they are.

- [ ] **Step 1:** Replace lines 128-129 with `res := combat.RunWithManeuverFloors(attackScore, defenseScore)`.

- [ ] **Step 2:** `:142` → `res.AttackRoll.ZScore <= -2.0`; `:169` → `if res.Success`.

- [ ] **Step 3:** `:184` becomes, with the comment:

```go
	// SIGN: contest.Result.Margin is ATTACK-positive and this is the ATTACKER's
	// crit check, so it is passed unnegated. The defensive mirror
	// (TryStoicResolve) negates. Note this reads Result.Margin and NOT
	// AttackRoll.Margin: the core rolls via dice.Roll, which never populates a
	// roll's Margin field, so the latter would silently be zero and no taunt
	// would ever crit.
	isCrit := combat.AttackContestCrit(res.Margin, res.AttackRoll)
```

- [ ] **Step 4:** Drop the now-unused `dice` import (`:9` — this was its only use).

- [ ] **Step 5: Verify** — `go test ./internal/actions/`.

---

### Task 4: Site 4 — `TryStoicResolve`, and `TrySpellDeflection` onto the wrapper

**Files:** Modify `internal/combat/avoidance.go`

- [ ] **Step 1:** Delete the `TODO(U3)` block above `TryStoicResolve` (lines 55-59) — keep the sentence noting U6 folds both into the defence multiplier, since that is still true and still worth knowing.

- [ ] **Step 2:** Replace lines 80-81 with `res := RunWithManeuverFloors(attackScore, defenseScore)`, and `if !success` with `if !res.Success`.

- [ ] **Step 3:** The crit call, with the comment corrected — the existing one says "used unnegated", which becomes **wrong** the moment the core is used:

```go
		// SIGN: contest.Result.Margin is ATTACK-positive and this is the
		// DEFENDER's crit check, so it is negated exactly here — the mirror of
		// TrySpellDeflection above. And it is Result.Margin, not
		// DefenseRoll.Margin: the core rolls via dice.Roll, which does not
		// populate that field, so reading it would pass zero and stoic resolve
		// would never fully negate again.
		if DefenseContestCrit(-res.Margin, res.DefenseRoll) {
```

- [ ] **Step 4:** Collapse `TrySpellDeflection`'s floor fetch + `contest.RunWithFloors` into `res := RunWithSpellFloors(attackScore, defenseScore)`. Its sign comment already documents the negation; keep it.

- [ ] **Step 5:** Drop **both** the `dice` and the `contest` imports — Step 4 removes the file's last `contest.` reference. Confirm with `grep -n "dice\.\|contest\." internal/combat/avoidance.go` returning nothing.

- [ ] **Step 6: Verify** — `go test ./internal/combat/`, including Task 1b's sign tests, which must still pass.

---

### Task 5: Sites 7, 8 — the `hooks` maneuver pair, and deleting the private accessors

**Files:** Modify `internal/hooks/Position_GrappleTick.go`, `internal/hooks/NewRound_MobRoundTick.go`, `internal/hooks/spell_resolution.go`, `internal/hooks/charm_spell.go`

**[review] Import bookkeeping, verified per file.** `Position_GrappleTick.go`, `NewRound_MobRoundTick.go` and `charm_spell.go` do **not** currently import `internal/combat` at file level (only `spell_resolution.go:11` does), so each needs it **added**. `configs` stays in `spell_resolution.go` (still used at `:361`, `:948`, `:1262`) and in `Position_GrappleTick.go` — do not over-delete.

- [ ] **Step 1: `Position_GrappleTick.go:269-270`** — `res := combat.RunWithManeuverFloors(ctrlScore, cdScore)`. Then `MarginAttacker: res.Margin`, `AttackerZScore: res.AttackRoll.ZScore`, `DefenderZScore: res.DefenseRoll.ZScore`, and the signed-z block reads `res.AttackRoll.StdDev` and `res.Margin`. All four fields are populated by `dice.Roll`. Add the `combat` import; drop `dice` (this was its only use).

  **[review] While you are in these four lines, add a breadcrumb — do not fix it.** `:284-286` computes `z = margin / atkRoll.StdDev`, dividing by `stdDev` **without** the `√2` that `ContestCrit` uses. Spec trap 5 says that inflates the margin ~41% and roughly triples crit rates. Preserve it exactly (this plan is a no-op) and mark it:
  ```go
	// NOTE(U6): normalised by stdDev alone, without the sqrt(2) that
	// ContestCrit applies. Both sides roll with the attacker's stdDev, so their
	// difference has stdDev*sqrt(2); see spec trap 5. Preserved as-is here
	// because U3 is a provable no-op. U6 owns the correction.
  ```

- [ ] **Step 2: `NewRound_MobRoundTick.go:397-398`** — `res := combat.RunWithManeuverFloors(attackScore, defenseScore)`; `if success` → `if res.Success`. Add the `combat` import; drop `dice` (`:15`, its only use).

- [ ] **Step 3:** Migrate the **five** spell sites — `spell_resolution.go:300`, `:731`, `:1294`, `:1314` (four) and `charm_spell.go:77` (the fifth) — from `contest.RunWithFloors(a, []contest.Entry{{Score: d}}, spellHitFloor(), spellResistFloor())` to `combat.RunWithSpellFloors(a, d)`. Read each carefully first: this is a pure call-shape change and **no site's field reads may move**. Then drop the now-unused `contest` import from **both** files and add `combat` to `charm_spell.go`; confirm with `grep -n "contest\." internal/hooks/spell_resolution.go internal/hooks/charm_spell.go` returning nothing.

- [ ] **Step 4:** Delete the four accessors. **[review] Verified layout — do not delete the whole block:** lines `265-272` are a doc comment carrying the *why* for the spell pair ("a fizzle burns the caster's round… the resist floor keeps an outmatched TARGET from being auto-hit with no agency, which matters because mobs cast at players too"); `273-275` is the false "hooks does not import internal/combat" claim; the four funcs are at `276-290`.

  **Delete `273-290`. Move the substance of `265-272` into `combat.SpellFloors`' doc comment** (`contest_floors.go:23-25`), which today explains only why `TrySpellDeflection` uses the spell pair — a narrower and different point. The mob-casts-at-players justification is exactly what U6 will need and it has no other home.

  The compiler is the dead-code sweep: if an accessor is still referenced, the build fails and that reference is a site this plan missed — find it, do not restore the accessor.

- [ ] **Step 5: Verify** — `go build ./...`; `go test ./internal/hooks/`; success criteria 4 and 5.

---

### Task 6: Documentation, in this PR

Standing rule 2: `context.md` ships with the change, never as a follow-up. Roadmap completion gate 6: *every comment describing the per-channel resolution this arc removes has been corrected or deleted.*

**[review] The first draft's Task 6 only ADDED documentation. Three of these steps are corrections of things U3 makes false — rule 4 ("delete as you migrate") applies to comments, not just code.**

- [ ] **Step 0: retire the site counts in `contest_floors.go`'s doc comments.** Task 1 documented that the maneuver pair is read at TEN places and the spell pair at SIX, correctly qualified as true "at this SHA". Every migration task invalidates those numbers, and by the end of U3 both collapse to one reader each. Replace the counts with the durable statement — that each pair is fetched in exactly one place, and that `internal/hooks` no longer has private duplicates — rather than leaving a number that is wrong the moment U6 touches anything. Left alone, this becomes precisely the kind of stale claim Step 3 exists to delete.

- [ ] **Step 1: `internal/combat/context.md`.** Add `RunWithManeuverFloors`/`RunWithSpellFloors` to the Public API with verified signatures, and a Gotchas entry: **read `Result.Margin`, never a roll's `.Margin`** — attack-positive, negate for a defender's crit check. Then `grep -n "dice.OpposedRoll" internal/combat/context.md` and correct **both** hits: `:752` ("Opposed roll: `dice.OpposedRollStat(attackScore, defenseScore)`" in the `runBestOfAllDefense` walkthrough — stale since **U1**, which fixed `:144` and `:1236` but missed this one) and `:1113` ("Both sides roll via `dice.OpposedRollStat`" in the submission section — U3 makes this false). Also document the `SkillMoveParams` contract: `AttackSkill`/`DefenseSkill` are **raw skill levels with no `SkillWeight` applied**, and `DefenseStat: 0` is how ranged folds its defence into a scalar.

- [ ] **Step 2: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`.** Four edits:

  a. Mark U3 done; mark the four **UNOWNED** rows and the unclaimed charm-reroll row as claimed and shipped by U3.

  b. **Add a new pre-U6 modelling gate: the special-move family's skill weight.** The roadmap's flip table says `skillWeight` is "per-channel 5 / 1 / 0 / 15 → uniform 5.0". That is wrong for everything U3 touches. Verified — `grep -rn "SkillWeight" internal/actions/ internal/combat/` returns **zero** hits in any of the fourteen `ExecuteSkillMove` callers:

  | Site | Attack weight | Defence weight |
  |---|---|---|
  | `ExecuteSkillMove` (bash/kick/trip/gore/hamstring/maul/pounce/rake/throttle/drain ×2, riposte-trip, auto-bash) | **×1** | **×1** |
  | `ExecuteFire` (ranged) | **×1** | ×1 + flat shield bonus |
  | `AttemptGrapple` | **×1** | **×1** |
  | `RollSubmissionAttempt` | **×`SubSkillWeight` (1.5)** | ×1.5 |
  | `processGrapplePair` | **×2.2 aggressor / ×2.0 defender** | same |
  | taunt / `TryStoicResolve` | ×5 | ×5 |

  Five distinct regimes, not one. U6's stated action is "uniform ×5, parameter deleted"; applied naively that moves fourteen sites from ×1 to ×5 on both sides. Against mobs — which all carry combat skill 1 — a weapon-combat-30 player's bash goes from `130 vs 101` to `250 vs 105`. Nobody has modelled that, because it is not in the table the modelling was done against. **Gate it before U6, do not fix it in U3.**

  c. **Discharge the roadmap's own unexecuted "Name them" instruction.** It says `actions/shadow.go`, `usercommands/skill.skullduggery.shadow.go` and the four `usercommands/go.go` hidden-detection checks are "plausibly part of U4's sneak, but not named. **Name them.**" Add them to U4's row in the Plans table. Until that is done, this plan's claim that "everything remaining belongs to U4" is an assertion, not a fact — the same failure mode as the `resolveCharmSpell` miss, one level up.

  d. **Add three newly-found unowned uncertain outcomes to the Ownership gaps table** (found by searching the concepts, not the function names — U3's own sweep was scoped to the floor accessors and was blind to these):
  - `actions/combat_throttle.go:126` — a flat `util.Rand(100)` cast-interrupt hanging off a maneuver, bypassing concentration entirely, so U9's "concentration becomes a contest" does not reach it → **assign U9**.
  - `actions/surprise_attack.go:222-225` — a hand-rolled per-weapon `util.Rand(100)` hit resolution that never contests the defender at all; fits none of the sweep's categories A–D → **assign U4**.
  - `hooks/Position_GrappleTick.go:284-286` — the missing `√2` (Task 5 Step 1's breadcrumb) → **assign U6**.

  Also note for U4: after U3, `contest.Run`/`AgainstDifficulty` are exported, unfloored and **invisible** to `contest_floor_guard_test.go`, whose AST list covers only `dice.OpposedRollStatRaw`/`dice.OpposedRoll`. U4 migrates the unfloored sites and is where extending that guard belongs.

- [ ] **Step 3: `internal/combat/margin_crit.go:92-115` — comment only, no code change.** After U3, **every** `ContestCrit` caller passes `contest.Result.Margin`; not one passes a `dice.OpposedRoll*` margin. The comment currently instructs the reader to pass `defRoll.Margin` for a defender — which is precisely TRAP 1, the silent-zero defect. Rewrite it to name `contest.Result.Margin` as the input, `contest.RunWithFloors` as the ±1 sentinel's source (`:113` currently credits `dice.OpposedRollStatWithFloors`), and to say explicitly: **never pass a `dice.RollResult`'s `.Margin`; the core does not populate it.** Keep the `normalizedAttackMargin` warning and the `√2` paragraph — both still true. Sweep `internal/combat/avoidance.go:42` and `internal/characters/cast_helpers.go:63` for the same stale framing.

- [ ] **Step 4: `internal/hooks/context.md`.** Check for the four deleted accessors and any claim that hooks resolves its own contests; correct or delete. Reviewers found no current mention, so this may be a no-op — confirm rather than assume.

- [ ] **Step 5: The spec.** `docs/superpowers/specs/completed/2026-08-12-unified-contest-resolution-design.md` §1.1's drift table lists ranged as `Perception + skill×5`. Ranged attack is **×1** (`combat_fire.go` passes `AttackSkill: rangedRank` raw). Correct the row and cross-reference the new modelling gate.

- [ ] **Step 6:** Verify every symbol named in the touched `context.md` files still exists (`tools/context_md_audit.py`, or `Select-String -Path internal\<pkg>\*.go -Pattern '^(func|type|const|var)\s'`).

- [ ] **Step 7: No `docs/PATCH_NOTES.md` entry.** **[review] Verified consistent with precedent, not a deviation from the SOP.** `git log 63f21e8ee..f36a44423 -- docs/PATCH_NOTES.md` returns exactly one commit — `5ea0c8980`, U0, the only arc item marked "Behaviour change? **Yes**". U1's three code commits carry no entry and PR #32 (U2) did not touch the file at all. A provable no-op has nothing player-facing to announce; the arc's player-facing note lands with U6. Recorded here so the next reviewer does not re-litigate it.

---

### Task 7: Whole-implementation verification

- [ ] **Step 1:** `gofmt -l internal/` prints nothing.
- [ ] **Step 2:** `go build ./...` and `go test ./...` clean. Confirm with `git diff --stat` that the only `_test.go` files touched are the two **new** ones (`contest_floors_test.go`, `contest_sign_test.go`) — no *existing* test file modified.
- [ ] **Step 3:** Success criteria 3, 4 and 5 by grep, as worded above.
- [ ] **Step 4:** Re-run Task 1b's sign tests specifically and confirm they still pass post-migration. They are the only thing standing between TRAP 1 and production.
- [ ] **Step 5:** Boot test in an isolated detached worktree per the pre-push SOP; exit 124 is the success case. Do not grep the bare word `panic`.
- [ ] **Step 6:** `git diff master --stat` reviewed line by line for any numeric literal introduced under `internal/` (standing rule 1) and any behaviour change (standing rule 3).

---

## What U3 deliberately does NOT do

- **Touch `flee.go`.** Its two `ManeuverFloors()` sites are U4's, named in the roadmap. Migrating them here would look tidy and would steal a chunk's scope.
- **Fix `rangedDefenseScore`, or any skill weight.** The ×1 attack weight, the flat shield bonus and `DefenseStat: 0` are the drift this arc exists to remove — at **U6**, in one reviewable commit, not smuggled into a no-op refactor. U3's contribution is documenting the five regimes so U6 flips against a true table.
- **Fix the missing `√2` in `Position_GrappleTick.go`.** Breadcrumbed for U6; correcting it would roughly triple grapple-drift crit rates, which is a behaviour change.
- **Extend `contest_floor_guard_test.go` to cover `contest.Run`.** Real gap, but U4 is where unfloored core usage actually appears.
- **Reconcile the two floor styles.** Melee floors after the contest; maneuver and spell floor inside the roll. That is U6's open question and `RunWithFloors` is documented as transitional.
- **Surface margin from `ExecuteSkillMove`.** See TRAP 3.
