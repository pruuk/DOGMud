# U2: Spell Channel onto the Contest Core — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the spell channel's six resolution sites onto `internal/contest`, changing no behaviour.

**Architecture:** The core gains contest-floor support (`RunWithFloors`) reproducing `dice.OpposedRollStatWithFloors` exactly, plus a `Success` field. The four spell attack sites, `resolveCharmSpell`, and `TrySpellDeflection` then call the core instead of `dice` directly.

**Tech Stack:** Go, `internal/contest`, `internal/dice`, `testify/assert`.

---

## Prerequisite state (verified 2026-08-12)

U1 shipped `internal/contest` with `Entry`, `Result{AttackRoll, DefenseRoll, Margin, Winner, Contested}`, `Run` and `AgainstDifficulty`. `Margin` is **attack-positive**.

The six sites this plan migrates:

| Site | File | Call |
|---|---|---|
| `resolveAgainstMob` | `internal/hooks/spell_resolution.go:300` | `OpposedRollStatWithFloors(spellAttack, defVal, spellHitFloor(), spellResistFloor())` |
| `resolveAgainstPlayer` | `internal/hooks/spell_resolution.go:730` | same |
| `resolveMobSpellAgainstMob` | `internal/hooks/spell_resolution.go:1292` | same |
| `resolveMobSpellAgainstPlayer` | `internal/hooks/spell_resolution.go:1311` | same |
| `resolveCharmSpell` | `internal/hooks/charm_spell.go:75` | `OpposedRollStatWithFloors(attackScore, defenseScore, spellHitFloor(), spellResistFloor())` |
| `TrySpellDeflection` | `internal/combat/avoidance.go` | `OpposedRollStatWithFloors(attackScore, defenseScore, floorHit, floorResist)` |

**Six sites, not five.** `resolveCharmSpell` was missed by the first draft of this
plan because its verification grepped only `spell_resolution.go` — true, but
misleading, since charm lives in a sibling file and its own comment says *"Charm
is a spell, so it takes the spell floors, not the maneuver pair."* It is called
from the live cast dispatch (`spell_resolution.go:231`) and uses the same
`spellHitFloor()`/`spellResistFloor()` accessors. Leaving it would have stranded a
spell-channel site owned by no plan at all, to be discovered at U10's audit.

Verified complete: `grep -rn "spellHitFloor()\|spellResistFloor()"` across
`internal/` returns exactly these five call sites plus the two accessor
definitions.

`TryStoicResolve` in `avoidance.go` is the **conviction** channel and belongs to
U3. Do not touch it here beyond the breadcrumb comment in Task 3.

**Also not U2's, flagged so it is not lost:**
`internal/hooks/NewRound_MobRoundTick.go:398` (the periodic charm-duration
reroll) uses the **maneuver** floors, not the spell pair, so it belongs with U3's
taunt/maneuver work. U3's stated scope does not currently claim it. Whoever
writes the U3 plan must add it.

## Why the core needs floors

All six sites use `dice.OpposedRollStatWithFloors`, which does three things `contest.Run` does not:

1. **Flips the outcome** with probability `floorSuccess` (when the attack lost) or `floorResist` (when it won) — the 5.9 last-resort guarantee.
2. **Stamps a ±1 sentinel margin** on a flipped outcome, so a floor-granted hit carries margin `1` rather than its real (losing) margin. That sentinel is load-bearing: `ContestCrit` normalises it to a near-zero z, which is why a floor-granted hit can never also be a crit.
3. **Clamps each floor to [0, 0.5]**, above which a floor stops being a last resort and becomes the dominant term.

Leaving floors at the call sites would mean six copies of that logic, which is the fragmentation this arc exists to remove.

**Note for U6:** melee applies its floors *after* the contest, in `resolveDefenseOutcomeCore`, while spell applies them *inside* the roll. Two floor styles. U6 must unify them; U2 deliberately preserves both.

## Two traps specific to this plan

**TRAP 1 — `contest.Run` does not populate `RollResult.Margin` or `.Success`.**
`dice.OpposedRoll` sets those fields on the returned rolls; `dice.Roll`, which the core uses, does not. The four spell attack sites only ever read `atkRoll.ZScore`, so they are unaffected. **`TrySpellDeflection` is affected** — it currently passes `defRoll.Margin` to `DefenseContestCrit`. Under the core that field is zero. Task 3 must pass the negated `Result.Margin` instead.

**TRAP 2 — the defence-side sign, again.**
`Result.Margin` is attack-positive. `TrySpellDeflection` needs a **defence-positive** margin. Negate exactly once, at the call, and comment it. Getting it backwards means a well-defended spell is treated as a decisive attacker win, and it compiles.

## File Structure

| File | Responsibility |
|---|---|
| `internal/contest/contest.go` (modify) | add `Success` to `Result`; add `RunWithFloors` |
| `internal/contest/contest_test.go` (modify) | tests for both |
| `internal/hooks/spell_resolution.go` (modify) | four attack sites call the core |
| `internal/hooks/charm_spell.go` (modify) | `resolveCharmSpell` calls the core |
| `internal/combat/avoidance.go` (modify) | `TrySpellDeflection` calls the core |
| `internal/contest/context.md` (modify) | document floors, `Success`, `Floored`, and the transitional status |

## Success criteria

1. `go test ./...` passes with **no existing test file modified**.
2. `internal/contest` still imports only stdlib and `internal/dice`.
3. `grep -c "OpposedRollStatWithFloors" internal/hooks/spell_resolution.go` returns `0`.
4. `TryStoicResolve` is untouched (it is U3's).

---

### Task 1: Add `Success` and `RunWithFloors` to the core

**Files:**
- Modify: `internal/contest/contest.go`
- Test: `internal/contest/contest_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/contest/contest_test.go`:

```go
// TestRun_SuccessTracksMargin — Success is simply "the attacker won", which is
// margin > 0. It exists so floored contests can report an outcome that no
// longer matches their margin.
func TestRun_SuccessTracksMargin(t *testing.T) {
	win := Run(10000, []Entry{{Name: "d", Score: 1}})
	assert.True(t, win.Success, "a decisive attacker win must report Success")
	assert.Greater(t, win.Margin, 0.0)

	lose := Run(1, []Entry{{Name: "d", Score: 10000}})
	assert.False(t, lose.Success, "a decisive attacker loss must not report Success")
	assert.Less(t, lose.Margin, 0.0)
}

// TestRunWithFloors_ZeroFloorsMatchRun — with both floors off, the floored
// variant must behave exactly like Run. This is the property that makes the
// migration safe for any caller passing 0.
func TestRunWithFloors_ZeroFloorsMatchRun(t *testing.T) {
	for i := 0; i < 500; i++ {
		res := RunWithFloors(10000, []Entry{{Name: "d", Score: 1}}, 0, 0)
		assert.True(t, res.Success, "attacker should win on merit with no floors")
		assert.Greater(t, res.Margin, 0.0, "margin must be the real one, not a sentinel")
	}
}

// TestRunWithFloors_SuccessFloorRescuesAHopelessAttack pins the 5.9 guarantee:
// a hopelessly outclassed attacker still lands roughly floorSuccess of the time.
func TestRunWithFloors_SuccessFloorRescuesAHopelessAttack(t *testing.T) {
	const samples = 20000
	wins := 0
	for i := 0; i < samples; i++ {
		// Attacker cannot win on merit at these scores.
		if RunWithFloors(1, []Entry{{Name: "d", Score: 100000}}, 0.15, 0).Success {
			wins++
		}
	}
	rate := float64(wins) / samples
	assert.InDelta(t, 0.15, rate, 0.02, "success floor should rescue ~15%% of hopeless attacks")
}

// TestRunWithFloors_ResistFloorSavesADoomedDefender is the mirror.
func TestRunWithFloors_ResistFloorSavesADoomedDefender(t *testing.T) {
	const samples = 20000
	saves := 0
	for i := 0; i < samples; i++ {
		if !RunWithFloors(100000, []Entry{{Name: "d", Score: 1}}, 0, 0.15).Success {
			saves++
		}
	}
	rate := float64(saves) / samples
	assert.InDelta(t, 0.15, rate, 0.02, "resist floor should save ~15%% of doomed defences")
}

// TestRunWithFloors_StampsTheSentinelMargin is load-bearing. A floored outcome
// carries margin +-1, NOT its real margin. Downstream, ContestCrit normalises
// that sentinel to a near-zero z, which is the only reason a floor-granted hit
// cannot also be a critical hit. If the real margin leaked through here, a
// hopeless attacker rescued by the floor would crit.
func TestRunWithFloors_StampsTheSentinelMargin(t *testing.T) {
	// 0.5 is the clamp CEILING, not a certainty, so each call is a coin flip on
	// whether the floor fires at all. This must therefore loop AND prove it
	// actually observed both branches. A single draw per branch would execute
	// no assertions at all on roughly a quarter of runs and still report PASS —
	// measured at 502/2000 — which is precisely how a "load-bearing" test
	// silently stops guarding anything while staying green.
	sawSuccess, sawResist := false, false

	for i := 0; i < 200; i++ {
		granted := RunWithFloors(1, []Entry{{Name: "d", Score: 100000}}, 0.5, 0)
		if granted.Success {
			sawSuccess = true
			assert.Equal(t, 1.0, granted.Margin, "a floor-granted success carries the +1 sentinel")
		}

		resisted := RunWithFloors(100000, []Entry{{Name: "d", Score: 1}}, 0, 0.5)
		if !resisted.Success {
			sawResist = true
			assert.Equal(t, -1.0, resisted.Margin, "a floor-granted resist carries the -1 sentinel")
		}
	}

	assert.True(t, sawSuccess, "success floor never fired in 200 draws — the test asserted nothing")
	assert.True(t, sawResist, "resist floor never fired in 200 draws — the test asserted nothing")
}

// TestRunWithFloors_ClampsFloors — above 0.5 a floor stops being a last resort
// and becomes the dominant term, so dice.OpposedRollStatWithFloors clamps to
// [0, 0.5]. The core must clamp identically or migrated callers change
// behaviour.
func TestRunWithFloors_ClampsFloors(t *testing.T) {
	const samples = 20000
	wins := 0
	for i := 0; i < samples; i++ {
		// 5.0 must be clamped to 0.5, not treated as "always".
		if RunWithFloors(1, []Entry{{Name: "d", Score: 100000}}, 5.0, 0).Success {
			wins++
		}
	}
	rate := float64(wins) / samples
	assert.InDelta(t, 0.5, rate, 0.03, "a floor above 0.5 must clamp to 0.5")

	// A negative floor must clamp to 0, never rescue anything.
	for i := 0; i < 500; i++ {
		assert.False(t, RunWithFloors(1, []Entry{{Name: "d", Score: 100000}}, -1.0, 0).Success,
			"a negative floor must clamp to 0")
	}
}

// TestRunWithFloors_UncontestedIsUntouched — no entries means no contest, so
// there is nothing for a floor to flip.
func TestRunWithFloors_UncontestedIsUntouched(t *testing.T) {
	res := RunWithFloors(100, nil, 0.5, 0.5)
	assert.False(t, res.Contested)
	assert.Equal(t, 0.0, res.Margin, "an uncontested result must not be stamped with a sentinel")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/contest/ -count=1`
Expected: build failure — `res.Success undefined` and `undefined: RunWithFloors`.

- [ ] **Step 3: Write the implementation**

In `internal/contest/contest.go`, add a `Success` field to `Result`, immediately after `Contested`:

```go
	// Success reports whether the ATTACKER won. Ordinarily this is just
	// Margin > 0, but RunWithFloors can flip it without changing the sign of
	// the sentinel margin it stamps, so callers that care about the outcome
	// must read this rather than re-deriving it from Margin.
	Success bool

	// Floored reports whether a contest floor CHANGED this outcome.
	//
	// Without it, the only way to ask is comparing Margin against the +-1
	// sentinel: that means knowing an internal constant, is ambiguous against a
	// genuine roll landing exactly there, and breaks silently if the sentinel is
	// ever retuned. Roadmap section 8 names floor-reliance rate as something
	// that must be MODELLED before U6 flips the defence model, so it has to be
	// answerable cheaply.
	Floored bool
```

At the end of `Run`, immediately before `return res`, set it:

```go
	res.Success = res.Contested && res.Margin > 0
```

Then append this function:

```go
// RunWithFloors is Run plus the 5.9 contest floors: a last-resort probability
// that an outcome is flipped, so a hopelessly outclassed actor is never simply
// incapable and an overwhelming one is never simply guaranteed.
//
// TRANSITIONAL. This exists so U2-U5 can be provable no-ops. The codebase has
// TWO floor styles and this reproduces only one: melee applies its floors AFTER
// the contest, in resolveDefenseOutcomeCore, flipping a hit with no margin
// involved; spell and maneuver apply theirs INSIDE the roll and need the
// sentinel margin to stop a floored hit from also critting. Roadmap section 8
// lists reconciling the two as an OPEN question for U6, which may delete or
// reshape this function entirely. Do not build new permanent behaviour on it
// without checking where that landed.
//
// It reproduces dice.OpposedRollStatWithFloors exactly, because callers are
// being migrated onto it and must not change behaviour:
//
//   - At most ONE floor is rolled per call. If the attack lost, only
//     floorSuccess is considered; if it won, only floorResist. Drawing both
//     would change the outcome distribution.
//   - A flipped outcome carries a SENTINEL margin of +1 or -1, not its real
//     margin. This is load-bearing: ContestCrit normalises that sentinel to a
//     near-zero z, which is the only reason a floor-granted hit cannot also be
//     a critical hit. Leak the real margin here and a hopeless attacker rescued
//     by the floor would crit.
//   - Both floors clamp to [0, 0.5]. Above that a floor stops being a last
//     resort and becomes the dominant term.
//
// An uncontested result is returned untouched — there is no outcome to flip.
func RunWithFloors(atkScore float64, entries []Entry, floorSuccess, floorResist float64) Result {
	res := Run(atkScore, entries)
	if !res.Contested {
		return res
	}

	floorSuccess = clampFloor(floorSuccess)
	floorResist = clampFloor(floorResist)

	switch {
	case !res.Success && floorSuccess > 0 && rand.Float64() < floorSuccess:
		res.Success, res.Margin, res.Floored = true, 1, true
	case res.Success && floorResist > 0 && rand.Float64() < floorResist:
		res.Success, res.Margin, res.Floored = false, -1, true
	}

	return res
}

// clampFloor bounds a floor to [0, 0.5], matching dice.clampFloor. Duplicated
// rather than imported because dice keeps it unexported, and this package takes
// its tunables as parameters by design.
func clampFloor(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 0.5 {
		return 0.5
	}
	return v
}
```

Add `"math/rand"` to the import block.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/contest/ -count=1 -v`
Expected: PASS, all sixteen tests (nine from U1 plus these seven).

- [ ] **Step 5: Confirm the package is still a leaf**

Run: `go list -deps ./internal/contest/ | grep GoMudEngine`
Expected: exactly `.../internal/dice` and `.../internal/contest`.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/contest/
go vet ./internal/contest/
git add internal/contest/contest.go internal/contest/contest_test.go
git commit -m "feat(contest): contest floors and an explicit Success outcome (U2)

RunWithFloors reproduces dice.OpposedRollStatWithFloors exactly, because
six sites are about to be migrated onto it and must not change
behaviour: at most one floor rolled per call, a flipped outcome stamped
with the +-1 sentinel margin rather than its real one, and both floors
clamped to [0, 0.5].

The sentinel is load-bearing. ContestCrit normalises it to a near-zero z,
which is the only reason a floor-granted hit cannot also be a crit. Leak
the real margin and a hopeless attacker rescued by the floor would crit.

Result gains Success because a floored outcome no longer agrees with the
sign of its margin, so callers cannot re-derive it.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Migrate the five spell attack sites

**Files:**
- Modify: `internal/hooks/spell_resolution.go` (lines 300, 730, 1292, 1311)

All four sites are byte-identical:

```go
	success, atkMargin, atkRoll, _ := dice.OpposedRollStatWithFloors(spellAttack, defVal, spellHitFloor(), spellResistFloor())
```

- [ ] **Step 1: Replace all four call sites**

Each becomes:

```go
	spellContest := contest.RunWithFloors(spellAttack, []contest.Entry{{Score: defVal}}, spellHitFloor(), spellResistFloor())
	success, atkMargin, atkRoll := spellContest.Success, spellContest.Margin, spellContest.AttackRoll
```

The local variable names `success`, `atkMargin` and `atkRoll` are kept deliberately so the rest of each function is untouched and the diff stays readable as a no-op.

**A spell has one defence, so the entry is unnamed** — `spellDefenseValue` returns a bare number, not a named defence type. That is exactly the static-difficulty shape, and `Winner` is correspondingly empty. Nothing downstream reads `Winner` here.

- [ ] **Step 2: Add the import**

Add `"github.com/GoMudEngine/GoMud/internal/contest"` to the import block of `internal/hooks/spell_resolution.go` in alphabetical position.

- [ ] **Step 3: Remove the now-unused `dice` import**

Verified 2026-08-12: `spell_resolution.go` contains exactly four `dice.`
references, and they are precisely the four calls you just replaced. So after
Step 1 the import is unused and the file will not compile until it is removed.

Run `grep -n "dice\." internal/hooks/spell_resolution.go` to confirm zero
matches, then delete `"github.com/GoMudEngine/GoMud/internal/dice"` from the
import block. If the grep unexpectedly returns matches, leave the import and say
so in your report.

- [ ] **Step 4: Migrate the fifth site, `resolveCharmSpell`**

In `internal/hooks/charm_spell.go` around line 75, replace:

```go
	success, _, _, _ := dice.OpposedRollStatWithFloors(
		float64(attackScore), float64(defenseScore),
		spellHitFloor(), spellResistFloor(),
	)
```

with:

```go
	success := contest.RunWithFloors(
		float64(attackScore), []contest.Entry{{Score: float64(defenseScore)}},
		spellHitFloor(), spellResistFloor(),
	).Success
```

Charm reads **only** the success boolean — it discards all three other returns
today — so this is the simplest of the five migrations. Keep the comment above it
(`// Charm is a spell, so it takes the spell floors, not the maneuver pair.`);
it is still true and it is the reason this site belongs to U2 rather than U3.

Add the `contest` import, and check whether `dice` is still used in that file:
run `grep -n "dice\." internal/hooks/charm_spell.go` and remove the import only
if there are zero matches.

- [ ] **Step 5: Verify no behaviour changed**

```bash
go build ./...
go test ./internal/hooks/ ./internal/contest/ -count=1
grep -rc "OpposedRollStatWithFloors" internal/hooks/spell_resolution.go internal/hooks/charm_spell.go
```
Expected: build clean, tests PASS with no test file edited, and BOTH greps return `0`.

- [ ] **Step 6: Run the full suite**

Run: `go test ./... 2>&1 | grep -E "^(FAIL|---)"`
Expected: only `internal/relationships` (a known Windows Defender quarantine of the test binary, unrelated). Any other failure is a real regression — report it, do not edit the test.

- [ ] **Step 7: Commit**

```bash
gofmt -l internal/
go vet ./internal/hooks/
git add internal/hooks/spell_resolution.go
git commit -m "refactor(spells): four spell attack sites use the contest core (U2)

resolveAgainstMob, resolveAgainstPlayer, resolveMobSpellAgainstMob and
resolveMobSpellAgainstPlayer now call contest.RunWithFloors instead of
dice.OpposedRollStatWithFloors. Local variable names are unchanged so the
rest of each function is untouched and the diff reads as the no-op it is.

A spell contests one unnamed defence, which is the static-difficulty
shape: spellDefenseValue returns a bare number, not a named defence type,
so Result.Winner is empty and nothing here reads it.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Migrate `TrySpellDeflection`

This is the delicate one. Read both traps at the top of this plan before starting.

**Files:**
- Modify: `internal/combat/avoidance.go`

The current code is:

```go
	floorHit, floorResist := SpellFloors()
	success, _, _, defRoll := dice.OpposedRollStatWithFloors(attackScore, defenseScore, floorHit, floorResist)

	defender.OnSkillUse(string(skills.Spellcasting), defenderUserId)
	defender.OnStatUse("perception", defenderUserId)

	if !success {
		// A full negation is the defensive mirror of a crit, so it derives from
		// the same normalized margin. defRoll.Margin is ALREADY defence-positive
		// (dice.OpposedRoll negates it for the defender), so it must not be
		// negated again here.
		if DefenseContestCrit(defRoll.Margin, defRoll) {
			return 0.0
		}
		return float64(cfg.SpellAvoidanceDamageMultiplier)
	}

	return 1.0
```

- [ ] **Step 1: Replace the roll and the crit check**

```go
	floorHit, floorResist := SpellFloors()
	res := contest.RunWithFloors(attackScore, []contest.Entry{{Score: defenseScore}}, floorHit, floorResist)

	defender.OnSkillUse(string(skills.Spellcasting), defenderUserId)
	defender.OnStatUse("perception", defenderUserId)

	if !res.Success {
		// A full negation is the defensive mirror of a crit, so it derives from
		// the same normalized margin.
		//
		// SIGN: contest.Result.Margin is ATTACK-positive, and this is the
		// DEFENDER's crit check, so it is negated exactly here. Previously this
		// read defRoll.Margin, which dice.OpposedRoll had already negated for
		// the defender; contest.Run uses dice.Roll, which does NOT populate
		// RollResult.Margin at all, so reading that field now would silently
		// pass zero and no defence would ever crit.
		if DefenseContestCrit(-res.Margin, res.DefenseRoll) {
			return 0.0
		}
		return float64(cfg.SpellAvoidanceDamageMultiplier)
	}

	return 1.0
```

Note the entry type is `[]contest.Entry`, qualified — `avoidance.go` is in package
`combat`, not package `contest`.

- [ ] **Step 2: Add the import**

Add `"github.com/GoMudEngine/GoMud/internal/contest"` to the import block of
`internal/combat/avoidance.go`, in alphabetical position.

`internal/combat` already imports `internal/contest` (U1 did it in
`combat_helpers.go`), so there is no cycle to worry about.

- [ ] **Step 3: Leave a breadcrumb above `TryStoicResolve`**

After this task, `avoidance.go` contains two adjacent near-identical functions
resolved by different mechanisms — `TrySpellDeflection` on the contest core,
`TryStoicResolve` still on `dice`. That is exactly the drift this arc removes,
reproduced in miniature, and a reader editing this file in isolation has no way
to know it is deliberate and temporary. Add directly above `TryStoicResolve`:

```go
// TODO(U3): still on dice.OpposedRollStatWithFloors while its sibling
// TrySpellDeflection above has moved to internal/contest. Deliberate, not an
// oversight: this is the conviction channel and roadmap U3 owns it. Note that
// U6 removes BOTH as parallel mechanisms, folding them into the defence
// multiplier, so do not invest in unifying them here.
```

This is a comment only. Do not change `TryStoicResolve`'s code.

- [ ] **Step 4: Confirm `TryStoicResolve`'s CODE is untouched**

Run: `git diff internal/combat/avoidance.go`
The diff must show only the added TODO comment above `TryStoicResolve` — no change to its body. That function is the conviction channel and belongs to U3.

- [ ] **Step 5: Verify**

```bash
go build ./...
go test ./internal/combat/ ./internal/contest/ ./internal/hooks/ -count=1
grep -n "dice\." internal/combat/avoidance.go
```
Expected: build clean, tests PASS with no test file edited. The `dice` import must remain — `TryStoicResolve` still uses it.

- [ ] **Step 6: Run the full suite**

Run: `go test ./... 2>&1 | grep -E "^(FAIL|---)"`
Expected: only `internal/relationships`.

- [ ] **Step 7: Commit**

```bash
gofmt -l internal/
go vet ./internal/combat/
git add internal/combat/avoidance.go
git commit -m "refactor(combat): TrySpellDeflection uses the contest core (U2)

The defensive spell site now calls contest.RunWithFloors. TryStoicResolve
is deliberately untouched -- it is the conviction channel and belongs to
U3.

One real hazard handled here: this previously passed defRoll.Margin,
which dice.OpposedRoll populates and had already negated for the
defender. contest.Run uses dice.Roll, which does NOT populate
RollResult.Margin at all, so reading that field would have silently
passed zero and no spell deflection would ever have critted. The negated
Result.Margin is used instead, negated exactly once because
Result.Margin is attack-positive and this is the defender's check.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Documentation

**Files:**
- Modify: `internal/contest/context.md`

- [ ] **Step 1: Update the Public API section**

Replace the Public API list with:

```markdown
## Public API

- `Run(atkScore float64, entries []Entry) Result` — one attack roll contested by
  every entry, keeping the widest defensive margin.
- `RunWithFloors(atkScore float64, entries []Entry, floorSuccess, floorResist float64) Result`
  — `Run` plus the 5.9 last-resort floors. Reproduces
  `dice.OpposedRollStatWithFloors` exactly.
- `AgainstDifficulty(score, difficulty float64) Result` — the same path against a
  fixed number instead of an opponent.
```

- [ ] **Step 2: Add `Success` and `Floored` to the Core types block**

In the `Result` struct in the Core types section, add:

```go
	Success     bool            // the ATTACKER won; read this, not Margin's sign, after floors
	Floored     bool            // a contest floor CHANGED this outcome
```

- [ ] **Step 3: Add four gotchas**

Append to the Gotchas section:

```markdown
- **After a floor fires, `Success` and `Margin` disagree by design.** A
  floor-granted success carries margin `+1` and a floor-granted resist carries
  `-1` — sentinels, not real margins. Read `Success` for the outcome; never
  re-derive it from the sign of `Margin`.
- **The sentinel is what stops a floor-granted hit from also critting.**
  `ContestCrit` normalises `±1` to a near-zero z. If a future change leaked the
  real margin through a floored outcome, a hopelessly outclassed attacker
  rescued by the floor would crit.
- **`RollResult.Margin` and `.Success` are NOT populated on the rolls this
  package returns.** `dice.OpposedRoll` sets them; `dice.Roll`, which this
  package uses, does not. Read `Result.Margin` and `Result.Success` instead.
  `TrySpellDeflection` was reading `defRoll.Margin` before U2 and would have
  silently received zero.
```

- [ ] **Step 4: Verify and commit**

```bash
python tools/context_md_audit.py 2>&1 | grep -i contest
go build ./...
git add internal/contest/context.md
git commit -m "docs(contest): floors, Success, and the unpopulated-roll-fields trap (U2)

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Self-review notes

- **Spec coverage.** U2's roadmap row is "spell channel onto the core (4 sites), preserving ×15 attack / ×0 defence as parameters." The four attack sites are Task 2; the fifth (defensive) site is Task 3. The ×15 and ×0 weights need no parameterisation — they live in `CalcSpellAttack` and `spellDefenseValue`, which compute the scores the core receives and are untouched.
- **Deliberately NOT in U2.** `TryStoicResolve` (U3), any uniform skill weight (U6), the two floor styles being unified (U6).
- **Riskiest step is Task 3**, for the `defRoll.Margin` trap. It is the one place in this plan where the obvious translation is silently wrong.
