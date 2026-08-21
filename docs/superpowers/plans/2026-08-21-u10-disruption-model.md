# U10 Disruption Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Concentration, knockdown, and prone recovery become real contests on
the arc's seam, killing the last three ×0-skill resolution sites.

**Architecture:** A new `combat.RunConcentrationContest` (the ONE reader of a
new `ConcentrationFloor` knob, mirroring `RunContest`/`ContestFloor`) resolves
both concentration paths against static difficulties (damagePct×10 with a 10%
threshold; the chunk-4f position lattice ×10). Knockdown and prone recovery
become `combat.RunContest` sites (standard floor), with per-move knockdown
percentages recalibrated into parity-anchored score factors. Progression fires
once per contest through the U9 seam.

**Tech Stack:** Go; existing `internal/contest` core; guard tests in
`internal/combat/contest_site_guard_test.go`.

**Spec:** `docs/superpowers/specs/2026-08-21-u10-disruption-model-design.md`
(owner decisions §6 are binding). Branch: `feature/u10-disruption-model`.

**Verified facts the plan builds on (do not re-derive):**
- `contest.RunWithFloors(atkScore, entries, floor)` takes the floor as a
  parameter (`internal/contest/contest.go:151`); `combat.RunContest`
  (`internal/combat/run_contest.go:23`) is the one `ContestFloor` reader.
- `AttackSide.score()` = `(Stat + SkillRank×SkillWeight) × Mult`
  (`internal/combat/defence_multiplier.go:80`), unexported, same package as
  `skill_moves.go`.
- `characters.Character` has `GetUserId()` and `MobInstanceId` (see the
  `ApplyHarm` call at `skill_moves.go` ~line 165) and
  `GetSkillLevel(skills.SkillTag) int` (`internal/characters/skills.go:166`).
- Skill tags: `skills.Spellcasting` = `spellcasting`, `skills.UnarmedCombat`
  = `unarmed-combat` (`internal/skills/skills.go:30,32`).
- `Character.OnSkillUse(skillName string, userId int) bool`
  (`internal/characters/progression.go:255`) — pass `GetUserId()` (0 for
  mobs).
- Concentration callers: `checkConcentrationBreak`
  (`internal/hooks/combat_shared_helpers.go:123`, called only from
  `handlePlayerConcentrationBreak` at `NewRound_DoCombat_helpers.go:857` —
  **players only; mobs have no damage-path break today and U10 preserves
  that**) and the position block in `processFoldRound`
  (`combat_shared_helpers.go:549-581`, called for users at
  `NewRound_DoCombat_helpers.go:288` and mobs at `:498`).
- `AttemptRecovery` (`internal/characters/skills.go:47`) is called from
  `internal/hooks/NewRound_UserRoundTick.go:245` and
  `tickMobProneRecovery` (`internal/hooks/NewRound_MobRoundTick.go:182`),
  both passing `Stats.Dexterity.ValueAdj`.
- Knockdown roll: `internal/combat/skill_moves.go:175` inside
  `executeSkillMoveWithRunner`; `SkillMoveParams.KnockdownChance int` set by
  14 callers in `internal/actions/combat_*.go` (bash/gore/pounce use
  `cfg.BashKnockdownChance`, trip `TripKnockdownChance`, kick
  `KickKnockdownChance`; all others pass 0).
- `_datafiles/config.yaml` has **skip-worktree** with four permanent local
  overrides. Any commit touching it MUST follow the blob procedure in each
  task that edits it (never commit the disk copy wholesale).

---

### Task 1: New balance knobs (Go side)

**Files:**
- Modify: `internal/configs/config.balance.go` (near `ContestFloor` at :91,
  and the knockdown knobs at :239-243)
- Modify: `internal/configs/config.balance.combat.go` (validation defaults)
- Modify: `internal/configs/config.balance.spells.go` (validation defaults)
- Test: `internal/configs/configs_balance_u10_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package configs

import "testing"

// U10 knobs: the concentration floor/threshold and the three parity-anchored
// knockdown factors. Defaults must apply when config.yaml omits the keys.
func TestU10KnobDefaults(t *testing.T) {
	b := BalanceConfig{}
	b.Validate()

	if got := float64(b.ConcentrationFloor); got != 0.02 {
		t.Errorf("ConcentrationFloor default = %v, want 0.02", got)
	}
	if got := int(b.ConcentrationDamageThresholdPct); got != 10 {
		t.Errorf("ConcentrationDamageThresholdPct default = %d, want 10", got)
	}
	if got := float64(b.BashKnockdownFactor); got != 1.0 {
		t.Errorf("BashKnockdownFactor default = %v, want 1.0", got)
	}
	if got := float64(b.TripKnockdownFactor); got != 1.055 {
		t.Errorf("TripKnockdownFactor default = %v, want 1.055", got)
	}
	if got := float64(b.KickKnockdownFactor); got != 0.921 {
		t.Errorf("KickKnockdownFactor default = %v, want 0.921", got)
	}
}
```

Note: check how the balance validation entry point is actually named before
running (`grep -n "func (b \*BalanceConfig)" internal/configs/config.balance*.go`
— the sibling files hold `Validate`-family methods). Use the same call the
existing default tests use; if a `TestBalanceDefaults`-style test exists,
mirror its setup exactly.

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTMPDIR=C:/gotmp go test ./internal/configs/ -run TestU10KnobDefaults -count=1`
Expected: FAIL (undefined fields).

- [ ] **Step 3: Add the fields**

In `config.balance.go`, next to `ContestFloor` (keep the comment style of its
neighbors):

```go
	// ConcentrationFloor is the symmetric last-resort probability for
	// concentration contests ONLY. The standard ContestFloor (0.125) would
	// break a master's concentration one disruption in eight; concentration
	// gets its own, much smaller mercy band. Read in exactly one place:
	// combat.RunConcentrationContest.
	ConcentrationFloor ConfigFloat `yaml:"ConcentrationFloor"` // default 0.02

	// ConcentrationDamageThresholdPct: damage below this percent of the
	// caster's health pool does not roll for concentration at all. Chip
	// damage should not generate rolls.
	ConcentrationDamageThresholdPct ConfigInt `yaml:"ConcentrationDamageThresholdPct"` // default 10
```

Replace the three knockdown knobs in place (`:239-243`) — a knob whose
semantics change must not keep its old name:

```go
	BashKnockdownFactor ConfigFloat `yaml:"BashKnockdownFactor"` // Knockdown score factor, parity-anchored to the old 50% (default 1.0)
	TripKnockdownFactor ConfigFloat `yaml:"TripKnockdownFactor"` // Knockdown score factor, parity-anchored to the old 60% (default 1.055)
	KickKnockdownFactor ConfigFloat `yaml:"KickKnockdownFactor"` // Knockdown score factor, parity-anchored to the old 35% (default 0.921)
```

In `config.balance.combat.go` (with the other combat defaults):

```go
	if b.BashKnockdownFactor <= 0 {
		b.BashKnockdownFactor = 1.0
	}
	if b.TripKnockdownFactor <= 0 {
		b.TripKnockdownFactor = 1.055
	}
	if b.KickKnockdownFactor <= 0 {
		b.KickKnockdownFactor = 0.921
	}
```

In `config.balance.spells.go` (this file owns concentration):

```go
	if b.ConcentrationFloor <= 0 || b.ConcentrationFloor > 0.5 {
		b.ConcentrationFloor = 0.02
	}
	if b.ConcentrationDamageThresholdPct < 1 {
		b.ConcentrationDamageThresholdPct = 10
	}
```

Delete the old `BashKnockdownChance`/`TripKnockdownChance`/
`KickKnockdownChance` defaulting blocks wherever they live (grep for them;
`config.balance.combat.go` is the likely home). Compilation of the three
`combat_*.go` callers breaks now — that is expected and fixed in Task 6; to
keep this task's test runnable, update the three call sites mechanically in
this task (`int(cfg.BashKnockdownChance)` → the field rename only, keeping
`KnockdownChance` int by casting `int(...)` is WRONG — instead leave the
callers broken until Step 4 of Task 6 if running the configs test alone, or
preferably fold the mechanical rename in here:
`internal/actions/combat_bash.go:116`, `combat_gore.go:113`,
`combat_pounce.go:128` → `KnockdownFactor: float64(cfg.BashKnockdownFactor)`,
`combat_trip.go:111,144` → `float64(cfg.TripKnockdownFactor)`,
`combat_kick.go:119,159` → `float64(cfg.KickKnockdownFactor)`, and in
`internal/combat/skill_moves.go` rename the field:
`KnockdownChance int` → `KnockdownFactor float64`, changing the roll line to
`if knockdownRoll.Value < p.KnockdownFactor*50.0` as a TEMPORARY bridge that
Task 6 deletes — this keeps every commit green).

Also update the zero-knockdown callers (`combat_drain.go:131`,
`combat_fire.go:271`, `combat_hamstring.go:122`, `combat_maul.go:118`,
`combat_rake.go:118`, and any other `KnockdownChance: 0` found by
`grep -rn "KnockdownChance" internal/actions/`) to `KnockdownFactor: 0`.

- [ ] **Step 4: Run the tests**

Run: `GOTMPDIR=C:/gotmp go test ./internal/configs/ ./internal/combat/ ./internal/actions/ -count=1`
Expected: PASS (configs test green; combat/actions compile with the bridge).

- [ ] **Step 5: Commit**

```bash
git add internal/configs/ internal/combat/skill_moves.go internal/actions/
git commit -m "config(u10): concentration floor/threshold knobs; knockdown chances become parity-anchored factors"
```

---

### Task 2: config.yaml wiring (skip-worktree procedure)

**Files:**
- Modify: `_datafiles/config.yaml` (via the blob procedure — NEVER commit the
  disk copy)

- [ ] **Step 1: Follow the skip-worktree procedure exactly**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
cp _datafiles/config.yaml /tmp/config.yaml.diskbackup
git update-index --no-skip-worktree _datafiles/config.yaml
git checkout -- _datafiles/config.yaml
```

- [ ] **Step 2: Edit the CLEAN (HEAD) copy**

In the `Balance:` block: next to `ContestFloor: 0.125` add:

```yaml
  # ConcentrationFloor: the symmetric mercy band for concentration contests
  #   only. The standard ContestFloor would break a master one-in-eight per
  #   disruption; concentration holds get a much smaller band.
  ConcentrationFloor: 0.02
  # ConcentrationDamageThresholdPct: damage below this percent of the pool
  #   never rolls for concentration. Chip damage should not generate rolls.
  ConcentrationDamageThresholdPct: 10
```

Replace the knockdown lines (search `KnockdownChance`, currently ~line
626-633) with:

```yaml
  # KnockdownFactor: multiplier on the attacker's score in the knockdown
  # contest (attacker stat+skill*weight*factor vs defender Dex+unarmed*weight).
  # Parity-anchored to the old flat chances: 1.0 = 50%, 1.055 = 60%,
  # 0.921 = 35% at equal scores. Away from parity the rate now scales with
  # the matchup, which is the point.
  BashKnockdownFactor: 1.0       # Bash/gore/pounce (was 50% flat)
  TripKnockdownFactor: 1.055     # Trip (was 60% flat)
  KickKnockdownFactor: 0.921     # Kick (was 35% flat)
```

Delete the `SpellConcentrationBase: 50` and
`SpellInitiationWillpowerDivisor: 4` lines and their comment block (~lines
1201-1209) — Task 5 deletes the Go reader, and a knob left in YAML with no
field is a lie.

- [ ] **Step 3: Commit the clean edit, restore the disk copy**

```bash
git add _datafiles/config.yaml
git diff --cached _datafiles/config.yaml   # REVIEW: only the U10 lines may appear
git commit -m "config(u10): ship the disruption-model knobs; retire the dead concentration pair"
cp /tmp/config.yaml.diskbackup _datafiles/config.yaml
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml     # expect leading 'S'
```

Note the disk copy now lacks the new keys — that is fine (absence falls back
to the Go defaults, which are identical).

---

### Task 3: The concentration seam

**Files:**
- Create: `internal/combat/run_concentration_contest.go`
- Test: `internal/combat/run_concentration_contest_test.go` (create)
- Modify: `internal/combat/contest_site_guard_test.go:47` (allowlist row)

- [ ] **Step 1: Write the failing test**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// The concentration floor must clamp both tails: a hopeless novice still
// holds 2% of the time, a master still breaks 2% of the time.
func TestRunConcentrationContest_FloorClampsBothTails(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.ConcentrationFloor = 0.02
	configs.SetConfigForTest(t, cfg)

	const n = 20000
	holdsHopeless, holdsMaster := 0, 0
	for i := 0; i < n; i++ {
		if RunConcentrationContest(50, 700).Success {
			holdsHopeless++
		}
		if RunConcentrationContest(700, 50).Success {
			holdsMaster++
		}
	}
	// 2% of 20000 = 400; allow wide statistical slop (5 sigma ~ +-70).
	if holdsHopeless < 300 || holdsHopeless > 500 {
		t.Errorf("hopeless caster held %d/20000, want ~400 (floor 0.02)", holdsHopeless)
	}
	if breaks := n - holdsMaster; breaks < 300 || breaks > 500 {
		t.Errorf("master broke %d/20000, want ~400 (ceiling 0.98)", breaks)
	}
}
```

(Check how sibling tests in `internal/combat` override balance config — if
they use a different helper than `configs.SetConfigForTest`, mirror that
exactly; `run_contest_test.go:34` is the model.)

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ -run TestRunConcentrationContest -count=1`
Expected: FAIL — `RunConcentrationContest` undefined.

- [ ] **Step 3: Write the seam**

`internal/combat/run_concentration_contest.go`:

```go
package combat

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// RunConcentrationContest resolves a caster's hold on a forming spell against
// a static difficulty (damagePct x10, or the position lattice x10). It is the
// ONE place Balance.ConcentrationFloor is read, exactly as RunContest is for
// ContestFloor — concentration gets its own, much smaller mercy band because
// the standard floor would break a master one disruption in eight.
//
// Success = the caster HELD. The result carries Run's usual crit/margin
// semantics but concentration consumes only Success today.
func RunConcentrationContest(casterScore, difficulty float64) contest.Result {
	return contest.RunWithFloors(casterScore,
		[]contest.Entry{{Score: difficulty}},
		float64(configs.GetBalanceConfig().ConcentrationFloor))
}
```

- [ ] **Step 4: Add the guard allowlist row**

In `contestSiteOwners` (`contest_site_guard_test.go:47`), under "The seams
themselves":

```go
	"internal/combat/run_concentration_contest.go:RunConcentrationContest": "the concentration floor seam — the ONE place Balance.ConcentrationFloor is read (U10)",
```

- [ ] **Step 5: Run the package tests (guard included)**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ -count=1`
Expected: PASS, including the site guard.

- [ ] **Step 6: Commit**

```bash
git add internal/combat/run_concentration_contest.go internal/combat/run_concentration_contest_test.go internal/combat/contest_site_guard_test.go
git commit -m "feat(u10): the concentration contest seam, floored at ConcentrationFloor"
```

---

### Task 4: Convert the damage path

**Files:**
- Modify: `internal/hooks/combat_shared_helpers.go:120-145`
  (`checkConcentrationBreak`) plus a new shared helper in the same file
- Test: `internal/hooks/hooks_test.go` (the existing
  `checkConcentrationBreak` tests at :344-410 change expectations)

- [ ] **Step 1: Update the existing tests to the new contract**

The tests at `hooks_test.go:344-410` currently pin the old curve. Rewrite
them to pin: (a) no roll below the threshold, (b) a roll above it, (c) the
NoDamageInterrupt skip, (d) zero/negative damage no-ops. Keep the existing
test scaffolding (whatever builds `ch` there today) and replace the
assertions:

```go
func TestCheckConcentrationBreak_BelowThresholdNeverRolls(t *testing.T) {
	ch := newCastingCharForTest(t) // reuse/extract the file's existing setup
	ch.HealthMax.Value = 1000
	// 5% damage with threshold 10: no contest, never breaks, no progression.
	for i := 0; i < 200; i++ {
		if checkConcentrationBreak(ch, 50) {
			t.Fatal("sub-threshold damage must never break concentration")
		}
	}
}

func TestCheckConcentrationBreak_AboveThresholdContests(t *testing.T) {
	ch := newCastingCharForTest(t)
	ch.HealthMax.Value = 100
	ch.Stats.Willpower.Base = 1 // hopeless caster
	ch.Recalculate()            // .ValueAdj is derived — never set it directly
	broke := false
	for i := 0; i < 200; i++ {
		if checkConcentrationBreak(ch, 90) { // 90% hit, difficulty 900
			broke = true
			break
		}
	}
	if !broke {
		t.Fatal("a hopeless caster taking 90%-pool hits should break within 200 tries")
	}
}
```

(Adapt names/setup to what the file actually has — the four existing tests
there show the working character construction; keep their NoDamageInterrupt
and non-positive-damage cases, updated only where the curve was asserted.
Remember the stat rule: set `.Base` then `Recalculate()`.)

- [ ] **Step 2: Run to verify the new expectations fail**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -run TestCheckConcentrationBreak -count=1`
Expected: FAIL (threshold not implemented; old curve still rolls sub-10%).

- [ ] **Step 3: Rewrite checkConcentrationBreak**

Replace the body from the `maxHealth :=` line down (`:135-144`) with:

```go
	maxHealth := ch.HealthMax.Value
	damagePct := damage * 100 / maxHealth
	if damagePct < int(configs.GetBalanceConfig().ConcentrationDamageThresholdPct) {
		// Chip damage does not generate rolls at all (U10).
		return false
	}
	res := combat.RunConcentrationContest(concentrationScore(ch), float64(damagePct*10))
	// One spellcasting event per contest, win or lose — the channel-defence
	// convention (U9/U10). The initiation cast progressed separately.
	ch.OnSkillUse(string(skills.Spellcasting), ch.GetUserId())
	return !res.Success
```

Add the shared score helper in the same file (both paths use it):

```go
// concentrationScore is the caster's side of every concentration contest:
// Wil + spellcasting x SkillWeight, the arc's standard shape.
func concentrationScore(ch *characters.Character) float64 {
	return float64(ch.Stats.Willpower.ValueAdj) +
		float64(ch.GetSkillLevel(skills.Spellcasting))*float64(configs.GetBalanceConfig().SkillWeight)
}
```

Add imports as needed (`combat`, `skills`, `configs` — check what the file
already imports; `internal/hooks` already imports `combat` heavily).

- [ ] **Step 4: Run the tests**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/combat_shared_helpers.go internal/hooks/hooks_test.go
git commit -m "feat(u10): damage-path concentration is a contest with a chip-damage threshold"
```

---

### Task 5: Convert the position path

**Files:**
- Modify: `internal/hooks/combat_shared_helpers.go:549-581`
  (`processFoldRound` position block)
- Test: extend the fold-round tests in `internal/hooks` (find them:
  `grep -rn "processFoldRound" internal/hooks/*_test.go`)

- [ ] **Step 1: Write/adjust the failing test**

Pin: a Standing caster never rolls; a prone hopeless caster breaks quickly;
a prone overwhelming caster (score 5000 vs difficulty 300) essentially always
holds (allow the 2% floor). Model on the existing fold-round tests' setup:

```go
func TestProcessFoldRound_ProneHopelessCasterBreaks(t *testing.T) {
	ch := newFoldingCharForTest(t) // the file's existing scaffold
	forceProneForTest(t, ch)       // however sibling tests do it
	ch.Stats.Willpower.Base = 1
	ch.Recalculate()
	broke := false
	for i := 0; i < 200 && !broke; i++ {
		res := processFoldRound(ch)
		broke = res.ProneBroke
		if broke {
			break
		}
		restartCastForTest(t, ch) // re-enter casting if the fold completed
	}
	if !broke {
		t.Fatal("a Wil-1 caster folding while prone should break within 200 rounds")
	}
}
```

(Adapt to the file's real helpers; if no scaffold exists, extract one from
whatever test currently covers the position block — `grep "PositionDisruption"
internal/hooks/*_test.go` finds it.)

- [ ] **Step 2: Run to verify state**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -run TestProcessFoldRound -count=1`
Expected: the new test may PASS already (the old curve also breaks a Wil-1
caster) — that is fine; it pins the behavior across the swap. The compile
must be green before the swap.

- [ ] **Step 3: Swap the block**

Replace `combat_shared_helpers.go:555-560` (the `dmgPctEquiv` roll) with:

```go
		dmgPctEquiv := position.PositionDisruptionDmgEquiv(posState, ctrlState)
		if dmgPctEquiv > 0 {
			// Chunk 4f's lattice keeps its full granularity; the x10
			// conversion is the design (owner 2026-08-21). Difficulties run
			// 250 (guard-bottom/supine) to 700 (crucifix-controlled).
			res := combat.RunConcentrationContest(concentrationScore(char), float64(dmgPctEquiv*10))
			char.OnSkillUse(string(skills.Spellcasting), char.GetUserId())
			if !res.Success {
```

The break-branch body (`clearCastingActivity` through the `return result`)
and the passed-roll comment stay exactly as they are; only the
chance-computation and `roll >= chance` test are replaced by `!res.Success`.
Delete the now-unused `util.LogRoll("Position Concentration", ...)` line.
Update the comment block above (`:539-548`) to describe the contest, not the
curve — keep the layering and NoDamageInterrupt sentences.

- [ ] **Step 4: Run the tests**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/combat_shared_helpers.go internal/hooks/*_test.go
git commit -m "feat(u10): position-path concentration contests the chunk-4f lattice at x10"
```

---

### Task 6: Delete the dead curve and its knobs

**Files:**
- Modify: `internal/characters/cast_helpers.go` (delete
  `CalcConcentrationChance` + the `:41` comment block)
- Modify: `internal/characters/casting_test.go` (delete
  `TestCalcConcentrationChance`)
- Modify: `internal/hooks/combat_shared_helpers_test.go:20-46` (delete the
  two curve tests)
- Modify: `internal/configs/config.balance.go:487-488` (delete the two
  fields)
- Modify: `internal/configs/config.balance.spells.go:18-22` (delete the two
  defaulting blocks)

- [ ] **Step 1: Delete, compiler-first**

Delete the config FIELDS first (the compiler is the dead-code sweep), then
every red site: `CalcConcentrationChance`, its two test files' curve tests,
the stale comment. `grep -rn "SpellConcentrationBase\|SpellInitiationWillpowerDivisor\|CalcConcentrationChance" internal/ modules/`
must return zero rows when done.

- [ ] **Step 2: Build and test**

Run: `GOTMPDIR=C:/gotmp go build ./... && GOTMPDIR=C:/gotmp go test ./internal/characters/ ./internal/hooks/ ./internal/configs/ -count=1`
Expected: PASS, no references anywhere.

- [ ] **Step 3: Commit**

```bash
git add internal/characters/ internal/hooks/ internal/configs/
git commit -m "refactor(u10): delete CalcConcentrationChance and its two knobs — the contest replaced the curve"
```

---

### Task 7: Knockdown becomes an opposed contest

**Files:**
- Modify: `internal/combat/skill_moves.go` (the temporary bridge from Task 1
  becomes the real contest)
- Modify: `internal/combat/contest_site_guard_test.go` (allowlist row)
- Test: `internal/combat/skill_moves_knockdown_test.go` (create)

- [ ] **Step 1: Write the failing calibration test**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// Parity calibration: at equal scores the knockdown contest must reproduce
// the old flat chances the factors were anchored to (spec section 3):
// 1.0 -> 50%, 1.055 -> 60%, 0.921 -> 35%. Note the standard ContestFloor
// (0.125) flips symmetrically at parity-ish rates, so the observed rate is
// pulled toward 50% by floor*(1-2p); the expectations below bake that in.
func TestKnockdownFactor_ParityCalibration(t *testing.T) {
	floor := float64(configs.GetBalanceConfig().ContestFloor)
	const n = 200000
	cases := []struct {
		name   string
		factor float64
		want   float64 // pre-floor target
	}{
		{"bash", 1.0, 0.50},
		{"trip", 1.055, 0.60},
		{"kick", 0.921, 0.35},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wins := 0
			for i := 0; i < n; i++ {
				res := RunContest(100*tc.factor, []contest.Entry{{Score: 100}})
				if res.Success {
					wins++
				}
			}
			got := float64(wins) / n
			want := tc.want + floor*(1-2*tc.want) // one symmetric flip per call
			if got < want-0.02 || got > want+0.02 {
				t.Errorf("%s: knockdown rate at parity = %.3f, want %.3f +-0.02", tc.name, got, want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ -run TestKnockdownFactor -count=1`
Expected: PASS immediately (it tests RunContest + the factor maths, not the
site). It exists to pin the calibration forever; commit it with the site
change below.

- [ ] **Step 3: Replace the bridge with the contest**

In `executeSkillMoveWithRunner`, replace the knockdown block (Task 1 left it
as `knockdownRoll.Value < p.KnockdownFactor*50.0`) with:

```go
		// Knockdown is an opposed contest (U10): the move's attack score
		// times its parity-anchored factor, against the defender's
		// Dex + unarmed-combat x SkillWeight. Control-immune defenders
		// (Ironhide's Living Carapace, Colossus's Ossified Frame) are
		// immovable and never contest — the blow still lands and deals
		// damage, it just doesn't take them off their feet. Factor 0 means
		// the move has no knockdown component at all (no contest, no
		// defender progression).
		if !mutations.IsControlImmune(p.Defender.Mutations) && p.KnockdownFactor > 0 {
			defScore := float64(p.Defender.Stats.Dexterity.ValueAdj) +
				float64(p.Defender.GetSkillLevel(skills.UnarmedCombat))*float64(configs.GetBalanceConfig().SkillWeight)
			kd := RunContest(p.Attack.score()*p.KnockdownFactor, []contest.Entry{{Score: defScore}})
			// One unarmed-combat event for the defender per contest, win or
			// lose (U9 convention). The attacker side fires nothing new: the
			// move's hit already progressed it, and a second event per swing
			// is the duplication U9 deleted.
			p.Defender.OnSkillUse(string(skills.UnarmedCombat), p.Defender.GetUserId())
			if kd.Success {
				result.KnockedDown = true
			}
		}
```

Delete the `dice.RollStat(50)` line; drop the `dice` import if now unused in
the file. Add `skills`/`configs` imports if missing.

- [ ] **Step 4: Add the guard row**

```go
	"internal/combat/skill_moves.go:executeSkillMoveWithRunner": "U10 (knockdown opposed contest after a landed special move)",
```

- [ ] **Step 5: Run the package**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ ./internal/actions/ -count=1`
Expected: PASS. If an existing skill-move test pinned flat knockdown rates,
update it to the contest contract (find with
`grep -rln "KnockdownChance\|KnockedDown" internal/combat/*_test.go internal/actions/*_test.go`).

- [ ] **Step 6: Commit**

```bash
git add internal/combat/ internal/actions/
git commit -m "feat(u10): knockdown is an opposed contest — factor-scaled attack vs Dex+unarmed"
```

---

### Task 8: Prone recovery becomes an opposed contest

**Files:**
- Modify: `internal/characters/skills.go:47-100` (`AttemptRecovery`
  signature + roll)
- Create: `internal/hooks/recovery_contest.go`
- Modify: `internal/hooks/NewRound_UserRoundTick.go:245`,
  `internal/hooks/NewRound_MobRoundTick.go:182` (callers)
- Modify: `internal/combat/contest_site_guard_test.go` (allowlist row)
- Test: `internal/characters/skills_recovery_test.go` (create or extend
  existing AttemptRecovery tests — find them:
  `grep -rln "AttemptRecovery" internal/characters/*_test.go internal/hooks/*_test.go`)

- [ ] **Step 1: Write the failing tests**

```go
package characters

import "testing"

// With no opponent (nil contest), a character past MinRecoveryRounds stands
// automatically — no roll against nobody (owner 2026-08-21).
func TestAttemptRecovery_NoOpponentAutoStands(t *testing.T) {
	c := newProneCharForTest(t, 0 /* MinRecoveryRounds */) // reuse the file's scaffold
	attempted, success := c.AttemptRecovery(nil)
	if !attempted || !success {
		t.Fatalf("free recovery: attempted=%v success=%v, want true/true", attempted, success)
	}
	if !c.IsStanding() {
		t.Fatal("character should be standing after free recovery")
	}
}

// With an opponent closure, the closure decides.
func TestAttemptRecovery_ContestLossStaysDown(t *testing.T) {
	c := newProneCharForTest(t, 0)
	attempted, success := c.AttemptRecovery(func() bool { return false })
	if !attempted || success {
		t.Fatalf("lost contest: attempted=%v success=%v, want true/false", attempted, success)
	}
	if c.IsStanding() {
		t.Fatal("character must stay down on a lost contest")
	}
}
```

(Adapt the scaffold: existing AttemptRecovery tests show how to build a prone
character; keep their MinRecoveryRounds coverage.)

- [ ] **Step 2: Run to verify failure**

Run: `GOTMPDIR=C:/gotmp go test ./internal/characters/ -run TestAttemptRecovery -count=1`
Expected: FAIL (signature mismatch).

- [ ] **Step 3: Change AttemptRecovery**

Replace the stat-curve roll (`skills.go:71-85`) and the signature:

```go
// AttemptRecovery tries to stand a prone/supine character once per round.
// contestWin is the opposed recovery contest against the character's living
// aggro opponent, built by the caller (internal/hooks/recovery_contest.go) —
// nil means nobody is holding them down and the stand is automatic once
// MinRecoveryRounds is consumed (owner 2026-08-21). The old solo Dex curve
// is gone: recovery is either contested or free.
func (c *Character) AttemptRecovery(contestWin func() bool) (bool, bool) {
```

The position gate and MinRecoveryRounds block (`:48-69`) stay byte-identical.
Then:

```go
	success := true
	if contestWin != nil {
		success = contestWin()
		// One unarmed-combat event per contested recovery, win or lose
		// (U9 convention). A free stand is not a contest and fires nothing.
		c.OnSkillUse(string(skills.UnarmedCombat), c.GetUserId())
	}
```

The FSM-transition/penalty tail (`:87-99`) stays as-is, driven by `success`.
Delete the `math` import if now unused; drop the `dice` import if unused.

- [ ] **Step 4: Build the hooks-side contest**

`internal/hooks/recovery_contest.go`:

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// recoveryContest builds the opposed prone-recovery roll for AttemptRecovery:
// recoverer Dex + unarmed-combat x SkillWeight against the same shape for the
// character's current aggro opponent. Returns nil — a free stand — when there
// is no living opponent in the character's room (owner 2026-08-21). The
// standard ContestFloor applies, so nobody is pinned forever (12.5% per round
// after the minimum) and no stand is ever certain against live opposition.
func recoveryContest(ch *characters.Character) func() bool {
	var opp *characters.Character
	if ch.Aggro != nil {
		if ch.Aggro.UserId > 0 {
			if u := users.GetByUserId(ch.Aggro.UserId); u != nil {
				opp = u.Character
			}
		} else if ch.Aggro.MobInstanceId > 0 {
			if m := mobs.GetInstance(ch.Aggro.MobInstanceId); m != nil {
				opp = &m.Character
			}
		}
	}
	if opp == nil || opp.Health < 1 || opp.RoomId != ch.RoomId {
		return nil
	}
	w := float64(configs.GetBalanceConfig().SkillWeight)
	self := float64(ch.Stats.Dexterity.ValueAdj) + float64(ch.GetSkillLevel(skills.UnarmedCombat))*w
	other := float64(opp.Stats.Dexterity.ValueAdj) + float64(opp.GetSkillLevel(skills.UnarmedCombat))*w
	return func() bool {
		return combat.RunContest(self, []contest.Entry{{Score: other}}).Success
	}
}
```

Callers:

`NewRound_UserRoundTick.go:245`:
```go
			if attemptMade, success := user.Character.AttemptRecovery(recoveryContest(user.Character)); attemptMade {
```

`NewRound_MobRoundTick.go:182`:
```go
	if attemptMade, success := mob.Character.AttemptRecovery(recoveryContest(&mob.Character)); attemptMade {
```

- [ ] **Step 5: Add the guard row**

```go
	"internal/hooks/recovery_contest.go:recoveryContest": "U10 (prone-recovery opposed contest; nil-opponent recoveries are free and never contest)",
```

- [ ] **Step 6: Run the packages**

Run: `GOTMPDIR=C:/gotmp go test ./internal/characters/ ./internal/hooks/ ./internal/combat/ -count=1`
Expected: PASS. Update any other `AttemptRecovery` callers/tests the compiler
finds (`grep -rn "AttemptRecovery(" internal/ --include="*.go"`).

- [ ] **Step 7: Commit**

```bash
git add internal/characters/ internal/hooks/ internal/combat/contest_site_guard_test.go
git commit -m "feat(u10): prone recovery contests the aggro opponent; free stand when nobody holds you down"
```

---

### Task 9: The Done-when guard

**Files:**
- Create: `internal/combat/u10_done_when_test.go`

- [ ] **Step 1: Write the guard (passes only when U10 is complete)**

```go
package combat

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// U10's "Done when" list as a test (the U6b lesson: prose criteria fail
// silently). Source-level assertions that the dead resolution paths stay dead.
func TestU10DoneWhen_DeadPathsStayDead(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))

	mustNotContain := func(rel, needle, why string) {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if strings.Contains(string(b), needle) {
			t.Errorf("%s still contains %q — %s", rel, needle, why)
		}
	}

	mustNotContain("internal/characters/cast_helpers.go", "CalcConcentrationChance",
		"the flat concentration curve was deleted by U10")
	mustNotContain("internal/combat/skill_moves.go", "RollStat(50",
		"knockdown is an opposed contest, not a flat roll")
	mustNotContain("internal/characters/skills.go", "RollStat(50",
		"prone recovery is an opposed contest or free, never a solo roll")
	mustNotContain("_datafiles/config.yaml", "SpellInitiationWillpowerDivisor",
		"the knob died with the curve")
	mustNotContain("_datafiles/config.yaml", "KnockdownChance",
		"percent knobs were replaced by parity-anchored factors")
}
```

Note: the config.yaml assertions read the DISK copy, which is skip-worktree
and may lag HEAD. If the disk copy still carries deleted keys after Task 2's
restore step, the correct fix is updating the disk copy's stale lines by hand
(it is the user's live dev config — coordinate, don't clobber), or reading
the blob via `git show HEAD:_datafiles/config.yaml` in the test instead.
Prefer the `git show` form if `exec.Command("git", ...)` is acceptable in
this test suite; otherwise document the coordination step in the PR.

- [ ] **Step 2: Run the full done-when set**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ -run "TestU10DoneWhen|TestKnockdownFactor|TestRunConcentrationContest" -count=1`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/combat/u10_done_when_test.go
git commit -m "test(u10): the Done-when list as a guard — dead paths stay dead"
```

---

### Task 10: Docs

**Files:**
- Modify: `internal/combat/context.md` (RunConcentrationContest, the
  knockdown contest, KnockdownFactor semantics)
- Modify: `internal/characters/context.md` (AttemptRecovery's new contract)
- Modify: `internal/hooks/context.md` (recoveryContest, concentrationScore,
  the two converted paths) — create the section if the file lacks one
- Modify: `internal/configs/context.md` if it enumerates balance knobs
- Modify: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md` (mark U10's section
  with the shipped shape: lattice kept, factors, floor 0.02, threshold 10,
  progression convention; note the roadmap's 3-row table was superseded by
  the lattice conversion)
- Modify: `docs/PATCH_NOTES.md`
- Modify: `CLAUDE.md` — the "Dice & Rolling System" section gains one line:
  concentration contests run through `combat.RunConcentrationContest`
  (floored by `ConcentrationFloor`, not `ContestFloor`)

- [ ] **Step 1: Update each file**

Verify every symbol named in context.md prose exists (codegraph or
`Select-String`). Patch notes entry (player-facing, no numbers, no em
dashes, 80-char wrap):

```markdown
## 2026-08-21: Holding a spell is a skill

Keeping a spell together while someone hits you is now a contest your
training wins, not a coin your willpower weights. A practiced caster holds
through pain that would shatter a novice, small scrapes no longer threaten
your casting at all, and being wrestled to the ground remains the surest way
to stop a caster: deep grapple holds will silence anyone.

Knocking someone down now has to beat them, not the dice. A trip or bash
measures your skill against their footwork, so a master cannot be tripped by
a flailing novice, and getting back up is a contest against whoever is
standing over you, not a private struggle with your own boots. With nobody
holding you down, you simply stand.
```

- [ ] **Step 2: Commit**

```bash
git add internal/combat/context.md internal/characters/context.md internal/hooks/context.md internal/configs/context.md docs/
git commit -m "docs(u10): context.md sweep, roadmap annotation, patch notes"
```

---

### Task 11: Gates and PR

- [ ] **Step 1: Format and full tests**

```bash
gofmt -l internal/ modules/        # must print nothing
GOTMPDIR=C:/gotmp go build ./...
GOTMPDIR=C:/gotmp go test ./... 2>&1 | tail -20   # exits 0; a failure is REAL
```

- [ ] **Step 2: Isolated-worktree boot test**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git worktree remove --force C:/tmp/dogmud-boot-check 2>/dev/null; git worktree prune
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
mkdir -p C:/tmp/dogmud-boot-check/_datafiles/logs
cd C:/tmp/dogmud-boot-check && GOTMPDIR=C:/gotmp go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1   # exit 124 = SUCCESS
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
cd "C:/Users/Calabe Davis/workspace/DOGMud" && git worktree remove --force C:/tmp/dogmud-boot-check
```

- [ ] **Step 3: Push and open the PR**

```bash
git push -u origin feature/u10-disruption-model
gh pr create --repo pruuk/DOGMud --base master --head feature/u10-disruption-model \
  --title "U10: the disruption model" \
  --body "<summary of the slice; end with the Claude Code footer>"
gh pr checks <n> --repo pruuk/DOGMud --watch
```

Verify annotations on the validate runs via
`gh api repos/pruuk/DOGMud/check-runs/<id>/annotations --jq length` (want 0)
— a green check alone is not proof. Do not merge without the owner's word;
the arc no-deploy policy holds regardless.
