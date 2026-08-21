# U10 Disruption Model Implementation Plan (rev 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Concentration, the throttle cast-interrupt, knockdown, and prone
recovery become real contests on the arc's seam — the last four ×0-skill
resolution sites this side of the U10d surprise-attack redesign.

**Architecture:** A new `combat.RunConcentrationContest` (the ONE reader of
`ConcentrationFloor`, mirroring `RunContest`/`ContestFloor`) resolves both
concentration paths against static difficulties (damagePct×10 with a 10%
threshold; the chunk-4f position lattice ×10). The throttle interrupt,
knockdown, and prone recovery become `combat.RunContest` sites (standard
floor); knockdown percentages become parity-anchored score factors.
Progression fires **success-only** (U10b's convention from birth).

**Tech Stack:** Go; `internal/contest` core; TWO guards: the site allowlist
in `internal/combat/contest_site_guard_test.go` AND the root-level
`contest_floor_guard_test.go` exemption map.

**Spec:** `docs/superpowers/specs/2026-08-21-u10-disruption-model-design.md`
rev 2 (§7 owner decisions binding). Branch: `feature/u10-disruption-model`.

**Verified facts (do not re-derive; all checked 2026-08-21 against source):**
- `contest.RunWithFloors(atkScore, entries, floor)` (`contest.go:151`); the
  floor FLIPS the outcome with probability `floor` — observed rate is
  `p + floor·(1−2p)`. Both sides roll with the ATTACKER's stdDev
  (`contest.go:97-103`) → margin σ = `RollSpread × atkScore × √2`.
- Config type is `configs.Balance` (`config.balance.go:7`) with
  `func (b *Balance) Validate()` (`:900`). `ConfigFloat`/`ConfigInt` are the
  field types. Old knockdown defaults live in
  `config.balance.misc.go:230-243` (bash Go default 40; shipped yaml is 50);
  `ThrottleInterruptChance` field `config.balance.go:248`, defaults
  `config.balance.misc.go:251-256`, NOT in config.yaml.
- `AttackSide.score()` (`defence_multiplier.go:80`), unexported, same
  package as `skill_moves.go`.
- `characters.Character`: `GetUserId()` (`character.go:465`, 0 for mobs),
  `MobInstanceId`, `GetSkillLevel(skills.SkillTag) int` (`skills.go:166`),
  `OnSkillUse(skillName string, userId int) bool` (`progression.go:255`,
  nil-map-safe), `GetSkillUseCount(skillName string) int`
  (`progression.go:522`), `RecalculateStats()` (`validate.go:29`). Stat
  setup in tests: set `.Base`, then `RecalculateStats()`.
- Skill tags: `skills.Spellcasting`=`spellcasting`,
  `skills.UnarmedCombat`=`unarmed-combat` (`skills.go:30,32`).
- Concentration callers: `checkConcentrationBreak`
  (`internal/hooks/combat_shared_helpers.go:123`; SIX existing tests at
  `hooks_test.go:347-415`), sole production caller
  `handlePlayerConcentrationBreak` (`NewRound_DoCombat_helpers.go:857`,
  players only — mobs have no damage path; preserved). Position block:
  `processFoldRound` `combat_shared_helpers.go:549-581`, callers `:288`
  (user) / `:498` (mob). Hooks test scaffolds that EXIST: `newCastingChar`
  (`hooks_test.go:337`), `newProneCastingChar` (`:420`).
- `AttemptRecovery` (`characters/skills.go:47`): exactly two callers —
  `NewRound_UserRoundTick.go:245`, `tickMobProneRecovery`
  (`NewRound_MobRoundTick.go:182`). **No existing AttemptRecovery tests
  anywhere** — Task 8's scaffold is built from scratch.
- `KnockdownChance` production setters — **fifteen**: `internal/actions/`
  `combat_bash.go:116`, `combat_gore.go:113`, `combat_pounce.go:128`,
  `combat_trip.go:111,144`, `combat_kick.go:119,159`,
  `combat_throttle.go:125`, `combat_drain.go:131`, `combat_fire.go:271`,
  `combat_hamstring.go:122`, `combat_maul.go:118`, `combat_rake.go:118`;
  `internal/combat/counter.go:119`; **`internal/hooks/
  combat_shared_helpers.go:299` (auto-trip) and `:358` (auto-bash)** — the
  last two read `cfg.TripKnockdownChance`/`cfg.BashKnockdownChance` directly.
  Plus five `internal/combat` TEST files constructing the field:
  `skill_moves_seam_test.go:37`, `skill_moves_partial_test.go:38`,
  `skill_moves_grapple_test.go:16,34`, `knockdown_nil_test.go:66`,
  `control_immune_test.go:37` — several rely on `KnockdownChance: 100`
  meaning GUARANTEED, which no floored contest reproduces; they need
  `pinContestFloorOff(t)` (`run_contest_test.go:25`) plus an overwhelming
  factor and hopeless defender.
- Knockdown roll: `skill_moves.go:175-176` inside
  `executeSkillMoveWithRunner`; control-immunity gate at `:174`.
- Throttle interrupt: `internal/actions/combat_throttle.go:151` inside
  `ExecuteThrottle`; `NoDamageInterrupt` is NOT currently checked there.
- Root guard: `contest_floor_guard_test.go` (package main) fails any file
  calling `contest.RunWithFloors` that is not in
  `guardedRollExemptions["contest"]` (today: `internal/combat/
  combat_helpers.go`, `internal/combat/run_contest.go`).
- `configs.SetConfigForTest(t, cfg)` exists (`testing_support.go:30`);
  floor-pinning model: `pinContestFloorOff` + `run_contest_test.go:34`.
- `_datafiles/config.yaml` is **skip-worktree** with four permanent local
  overrides — every edit uses the blob procedure (Task 2), and source-level
  yaml assertions must read `git show HEAD:_datafiles/config.yaml`, never
  the disk copy.
- `internal/characters/context.md:961-964` forbids NEW direct `Aggro` field
  reads — Task 8's room scan must use the accessor/predicate that section
  prescribes (read it first; mirror the nearest compliant site if only
  field access exists in practice, and say so in the commit).

---

### Task 1: New balance knobs + the knockdown field rename (everything compiles)

**Files:**
- Modify: `internal/configs/config.balance.go` (:91 area, :239-243, :248)
- Modify: `internal/configs/config.balance.misc.go:230-243` (knockdown
  defaults) — leave `ThrottleInterruptChance` alone until Task 6b
- Modify: `internal/configs/config.balance.spells.go` (new concentration
  defaults)
- Modify: `internal/combat/skill_moves.go` (field rename + bridge)
- Modify: ALL fifteen production setters listed in Verified facts
- Modify: the five `internal/combat` test files listed in Verified facts
- Test: `internal/configs/configs_balance_u10_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package configs

import "testing"

// U10 knobs: concentration floor/threshold and the three parity-anchored
// knockdown factors. Defaults must apply when config.yaml omits the keys.
func TestU10KnobDefaults(t *testing.T) {
	b := Balance{}
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
	if got := float64(b.TripKnockdownFactor); got != 1.057 {
		t.Errorf("TripKnockdownFactor default = %v, want 1.057", got)
	}
	if got := float64(b.KickKnockdownFactor); got != 0.924 {
		t.Errorf("KickKnockdownFactor default = %v, want 0.924", got)
	}
}
```

(If `Balance.Validate` panics on an otherwise-zero struct, mirror whatever
setup the existing defaults tests in this package use instead — check
`grep -rln "Validate()" internal/configs/*_test.go`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTMPDIR=C:/gotmp go test ./internal/configs/ -run TestU10KnobDefaults -count=1`
Expected: FAIL (undefined fields).

- [ ] **Step 3: Add fields and defaults**

`config.balance.go`, next to `ContestFloor` (:91), matching neighbor
comment style:

```go
	// ConcentrationFloor is the symmetric last-resort flip probability for
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

Replace the three knockdown knobs (:239-243):

```go
	BashKnockdownFactor ConfigFloat `yaml:"BashKnockdownFactor"` // Knockdown score factor, parity-anchored to the shipped 50% (default 1.0)
	TripKnockdownFactor ConfigFloat `yaml:"TripKnockdownFactor"` // Knockdown score factor, parity-anchored to the shipped 60% (default 1.057)
	KickKnockdownFactor ConfigFloat `yaml:"KickKnockdownFactor"` // Knockdown score factor, parity-anchored to the shipped 35% (default 0.924)
```

`config.balance.misc.go:230-243` — replace the three old defaulting blocks:

```go
	if b.BashKnockdownFactor <= 0 {
		b.BashKnockdownFactor = 1.0
	}
	if b.TripKnockdownFactor <= 0 {
		b.TripKnockdownFactor = 1.057
	}
	if b.KickKnockdownFactor <= 0 {
		b.KickKnockdownFactor = 0.924
	}
```

`config.balance.spells.go` (this file owns concentration):

```go
	if b.ConcentrationFloor <= 0 || b.ConcentrationFloor > 0.5 {
		b.ConcentrationFloor = 0.02
	}
	if b.ConcentrationDamageThresholdPct < 1 {
		b.ConcentrationDamageThresholdPct = 10
	}
```

- [ ] **Step 4: Rename the struct field with a temporary bridge**

`internal/combat/skill_moves.go`: `KnockdownChance int` →
`KnockdownFactor float64` (keep the field's comment, updated). The roll at
:175-176 becomes the TEMPORARY bridge **that Task 7 deletes**:

```go
		if !mutations.IsControlImmune(p.Defender.Mutations) {
			knockdownRoll := dice.RollStat(50)
			if knockdownRoll.Value < p.KnockdownFactor*50.0 {
				result.KnockedDown = true
			}
		}
```

Update ALL fifteen production setters:
- `combat_bash.go:116`, `combat_gore.go:113`, `combat_pounce.go:128` →
  `KnockdownFactor: float64(cfg.BashKnockdownFactor)`
- `combat_trip.go:111,144` → `float64(cfg.TripKnockdownFactor)`
- `combat_kick.go:119,159` → `float64(cfg.KickKnockdownFactor)`
- `internal/hooks/combat_shared_helpers.go:299` →
  `KnockdownFactor: float64(configs.GetBalanceConfig().TripKnockdownFactor)`
  and `:358` → `...BashKnockdownFactor)` (match the surrounding cfg
  variable if one is in scope)
- zero sites (`combat_drain.go:131`, `combat_fire.go:271`,
  `combat_hamstring.go:122`, `combat_maul.go:118`, `combat_rake.go:118`,
  `combat_throttle.go:125`, `internal/combat/counter.go:119`) →
  `KnockdownFactor: 0`

Update the five test files: replace `KnockdownChance: 100` with
`KnockdownFactor: 2.0` (under the bridge, `2.0*50 = 100` keeps them
guaranteed; Task 7 revisits them properly).

Verify completeness: `grep -rn "KnockdownChance" internal/ modules/` → zero
rows.

- [ ] **Step 5: Run the affected packages**

Run: `GOTMPDIR=C:/gotmp go test ./internal/configs/ ./internal/combat/ ./internal/actions/ ./internal/hooks/ -count=1`
Expected: PASS — the bridge preserves old behavior exactly (factor·50 = old
percent at the shipped values).

- [ ] **Step 6: Commit**

```bash
git add internal/configs/ internal/combat/ internal/actions/ internal/hooks/
git commit -m "config(u10): concentration knobs; knockdown chances become parity-anchored factors (bridge)"
```

---

### Task 2: config.yaml wiring (skip-worktree blob procedure)

**Files:**
- Modify: `_datafiles/config.yaml` — NEVER commit the disk copy

- [ ] **Step 1: The procedure, exactly**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
cp _datafiles/config.yaml /tmp/config.yaml.diskbackup
git update-index --no-skip-worktree _datafiles/config.yaml
git checkout -- _datafiles/config.yaml
```

- [ ] **Step 2: Edit the CLEAN (HEAD) copy**

Next to `ContestFloor: 0.125` (~line 546):

```yaml
  # ConcentrationFloor: the symmetric mercy band for concentration contests
  #   only. The standard ContestFloor would break a master one-in-eight per
  #   disruption; concentration holds get a much smaller band.
  ConcentrationFloor: 0.02
  # ConcentrationDamageThresholdPct: damage below this percent of the pool
  #   never rolls for concentration. Chip damage should not generate rolls.
  ConcentrationDamageThresholdPct: 10
```

Replace the knockdown block (~lines 626-633):

```yaml
  # KnockdownFactor: multiplier on the attacker's score in the knockdown
  # contest (attack score x factor vs defender Dex + unarmed x SkillWeight).
  # Parity-anchored to the old flat chances: 1.0 = 50%, 1.057 = 60%,
  # 0.924 = 35% at equal scores. Away from parity the rate scales with the
  # matchup, which is the point.
  BashKnockdownFactor: 1.0       # Bash/gore/pounce (was 50% flat)
  TripKnockdownFactor: 1.057     # Trip (was 60% flat)
  KickKnockdownFactor: 0.924     # Kick (was 35% flat)
```

Delete `SpellConcentrationBase: 50`, `SpellInitiationWillpowerDivisor: 4`,
and their comment block (~lines 1201-1209). (`ThrottleInterruptChance` was
never in this file — nothing to delete for it.)

- [ ] **Step 3: Commit clean, restore disk copy**

```bash
git add _datafiles/config.yaml
git diff --cached _datafiles/config.yaml   # REVIEW: only the U10 lines
git commit -m "config(u10): ship the disruption-model knobs; retire the dead concentration pair"
cp /tmp/config.yaml.diskbackup _datafiles/config.yaml
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml     # expect leading 'S'
```

The restored disk copy lacks the new keys (falls back to identical Go
defaults) and still contains the deleted ones (harmless: yaml decode is
non-strict). Task 9's assertions therefore read the HEAD blob, never disk.

---

### Task 3: The concentration seam (BOTH guards)

**Files:**
- Create: `internal/combat/run_concentration_contest.go`
- Test: `internal/combat/run_concentration_contest_test.go` (create)
- Modify: `internal/combat/contest_site_guard_test.go:47` (allowlist row)
- Modify: `contest_floor_guard_test.go` (repo root — exemption row)

- [ ] **Step 1: Write the failing test**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// The concentration floor must flip both tails: a hopeless caster still
// holds ~2% of the time, a master still breaks ~2% of the time.
func TestRunConcentrationContest_FloorFlipsBothTails(t *testing.T) {
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
	// 2% of 20000 = 400; allow ~5 sigma (+-100).
	if holdsHopeless < 300 || holdsHopeless > 500 {
		t.Errorf("hopeless caster held %d/20000, want ~400 (floor 0.02)", holdsHopeless)
	}
	if breaks := n - holdsMaster; breaks < 300 || breaks > 500 {
		t.Errorf("master broke %d/20000, want ~400 (flip 0.02)", breaks)
	}
}
```

(Mirror `run_contest_test.go:34` if its config-override shape differs.)

- [ ] **Step 2: Run to verify failure**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ -run TestRunConcentrationContest -count=1`
Expected: FAIL — undefined.

- [ ] **Step 3: Write the seam**

`internal/combat/run_concentration_contest.go`:

```go
package combat

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// RunConcentrationContest resolves a caster's hold on a forming spell
// against a static difficulty (damagePct x10, or the position lattice x10).
// It is the ONE place Balance.ConcentrationFloor is read, exactly as
// RunContest is for ContestFloor — concentration gets its own, much smaller
// mercy band because the standard floor would break a master one
// disruption in eight.
//
// Success = the caster HELD.
func RunConcentrationContest(casterScore, difficulty float64) contest.Result {
	return contest.RunWithFloors(casterScore,
		[]contest.Entry{{Score: difficulty}},
		float64(configs.GetBalanceConfig().ConcentrationFloor))
}
```

- [ ] **Step 4: Feed BOTH guards**

`contest_site_guard_test.go` `contestSiteOwners`, under "The seams
themselves":

```go
	"internal/combat/run_concentration_contest.go:RunConcentrationContest": "the concentration floor seam — the ONE place Balance.ConcentrationFloor is read (U10)",
```

Root `contest_floor_guard_test.go`, in `guardedRollExemptions["contest"]`
next to the `run_contest.go` row:

```go
		"internal/combat/run_concentration_contest.go": "defines combat.RunConcentrationContest — the one ConcentrationFloor reader (U10)",
```

(Match the map's actual shape — read the two existing rows first.)

- [ ] **Step 5: Run package AND root guards**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ . -count=1`
Expected: PASS including `TestOpposedContestsAreFloored` in the root
package.

- [ ] **Step 6: Commit**

```bash
git add internal/combat/run_concentration_contest.go internal/combat/run_concentration_contest_test.go internal/combat/contest_site_guard_test.go contest_floor_guard_test.go
git commit -m "feat(u10): the concentration contest seam, floored at ConcentrationFloor"
```

---

### Task 4: Convert the damage path (threshold + contest + success-only progression)

**Files:**
- Modify: `internal/hooks/combat_shared_helpers.go:120-145`
  (`checkConcentrationBreak`) + new `concentrationScore` helper
- Test: `internal/hooks/hooks_test.go:347-415` — SIX existing tests
  (`NotCasting`, `ZeroDamage`, `NegativeDamage`, `WithDamage`,
  `NoDamageInterrupt_NeverBreaks`, `NormalSpell_StillBreaks`)

- [ ] **Step 1: Update the six tests + add threshold/progression pins**

Keep all six cases. The scaffold that exists is `newCastingChar(spellId)`
(`hooks_test.go:337`) — use it, not an invented helper. Replace
curve-specific assertions and add:

```go
func TestCheckConcentrationBreak_BelowThresholdNeverRolls(t *testing.T) {
	ch := newCastingChar("mind-spike") // adapt spellId to what :337's callers use
	ch.HealthMax.Value = 1000
	before := ch.GetSkillUseCount(string(skills.Spellcasting))
	for i := 0; i < 200; i++ {
		if checkConcentrationBreak(ch, 50) { // 5% of pool
			t.Fatal("sub-threshold damage must never break concentration")
		}
	}
	if got := ch.GetSkillUseCount(string(skills.Spellcasting)); got != before {
		t.Fatalf("sub-threshold damage fired progression: %d -> %d", before, got)
	}
}

func TestCheckConcentrationBreak_ProgressionFiresOnlyOnHolds(t *testing.T) {
	ch := newCastingChar("mind-spike")
	ch.HealthMax.Value = 100
	before := ch.GetSkillUseCount(string(skills.Spellcasting))
	holds := 0
	for i := 0; i < 200; i++ {
		if !checkConcentrationBreak(ch, 30) { // 30% hit, difficulty 300
			holds++
		}
	}
	if got := ch.GetSkillUseCount(string(skills.Spellcasting)) - before; got != holds {
		t.Fatalf("progression events %d != holds %d (success-only rule)", got, holds)
	}
}
```

(Adapt construction details to the real scaffold; stat setup rule: `.Base`
then `RecalculateStats()`.)

- [ ] **Step 2: Run to verify failure**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -run TestCheckConcentrationBreak -count=1`
Expected: FAIL (threshold and progression not implemented).

- [ ] **Step 3: Rewrite the body**

Replace `checkConcentrationBreak`'s body from `maxHealth :=` down
(:135-144):

```go
	maxHealth := ch.HealthMax.Value
	damagePct := damage * 100 / maxHealth
	if damagePct < int(configs.GetBalanceConfig().ConcentrationDamageThresholdPct) {
		// Chip damage does not generate rolls at all (U10).
		return false
	}
	res := combat.RunConcentrationContest(concentrationScore(ch), float64(damagePct*10))
	if res.Success {
		// Success-only progression (U10b convention, adopted from birth):
		// one spellcasting event per HELD contest. The initiation cast
		// progressed separately; a lost hold fires nothing.
		ch.OnSkillUse(string(skills.Spellcasting), ch.GetUserId())
	}
	return !res.Success
```

Add the shared helper in the same file:

```go
// concentrationScore is the caster's side of every concentration contest:
// Wil + spellcasting x SkillWeight, the arc's standard shape.
func concentrationScore(ch *characters.Character) float64 {
	return float64(ch.Stats.Willpower.ValueAdj) +
		float64(ch.GetSkillLevel(skills.Spellcasting))*float64(configs.GetBalanceConfig().SkillWeight)
}
```

Imports: the file already uses `characters`/`configs`/`spells`; add
`combat` and `skills` if missing (hooks already imports both elsewhere —
compiler decides).

- [ ] **Step 4: Run the package**

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
- Test: whatever covers the block today — find with
  `grep -rln "PositionDisruption\|ProneBroke" internal/hooks/*_test.go`;
  `newProneCastingChar` (`hooks_test.go:420`) is the existing scaffold

- [ ] **Step 1: Pin behavior across the swap**

```go
func TestProcessFoldRound_ProneHopelessCasterBreaks(t *testing.T) {
	ch := newProneCastingChar("mind-spike") // adapt to the real scaffold
	ch.Stats.Willpower.Base = 1
	ch.RecalculateStats()
	broke := false
	for i := 0; i < 200 && !broke; i++ {
		res := processFoldRound(ch)
		if res.ProneBroke {
			broke = true
		}
		// If the fold completed instead, re-enter casting the way the
		// scaffold does.
	}
	if !broke {
		t.Fatal("a Wil-1 caster folding while prone should break within 200 rounds")
	}
}
```

- [ ] **Step 2: Compile-check**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -run TestProcessFoldRound -count=1`
Expected: PASS already (the old curve also breaks a Wil-1 caster) — it pins
the contract across the swap.

- [ ] **Step 3: Swap the roll**

Replace :555-560 (`dmgPctEquiv` computation stays; the chance+roll goes):

```go
		dmgPctEquiv := position.PositionDisruptionDmgEquiv(posState, ctrlState)
		if dmgPctEquiv > 0 {
			// Chunk 4f's lattice keeps its full granularity; the x10
			// conversion is the design (owner 2026-08-21, re-ratified over
			// the corrected table — prone 300, deep holds 600-700).
			res := combat.RunConcentrationContest(concentrationScore(char), float64(dmgPctEquiv*10))
			if res.Success {
				// Success-only progression: one spellcasting event per HELD
				// round (melee fires per combat round on the same basis).
				char.OnSkillUse(string(skills.Spellcasting), char.GetUserId())
			}
			if !res.Success {
```

The break branch (`clearCastingActivity` … `return result`) and the
passed-roll comment stay byte-identical. Delete the
`util.LogRoll("Position Concentration", ...)` line. **After Task 6 the
`util` import may become unused in this file — the compiler will say so;
delete it then.** Update the :539-548 comment to describe the contest,
keeping the layering + NoDamageInterrupt sentences.

- [ ] **Step 4: Run and commit**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -count=1` → PASS.

```bash
git add internal/hooks/combat_shared_helpers.go internal/hooks/hooks_test.go
git commit -m "feat(u10): position-path concentration contests the chunk-4f lattice at x10"
```

---

### Task 6: Delete the dead curve and its knobs

**Files:**
- Modify: `internal/characters/cast_helpers.go` (delete
  `CalcConcentrationChance` + the :41-43 comment)
- Modify: `internal/characters/casting_test.go` (delete its curve test)
- Modify: `internal/hooks/combat_shared_helpers_test.go:20-46` (delete the
  two curve tests)
- Modify: `internal/configs/config.balance.go:487-488` (delete two fields)
- Modify: `internal/configs/config.balance.spells.go:18-22` (delete two
  defaulting blocks)
- Modify: `internal/state/position/disruption.go` lines 1-13 and 20-22
  (comments reference the dead function — reword to "the concentration
  contest difficulty conversion in processFoldRound (x10)")
- Modify: `internal/hooks/combat_shared_helpers.go` (drop the `util` import
  if now unused)

- [ ] **Step 1: Delete, compiler-first**

Delete the two config FIELDS first, then every red site. Exit check
(CODE only — context.md prose is Task 10's):

```bash
grep -rn "SpellConcentrationBase\|SpellInitiationWillpowerDivisor\|CalcConcentrationChance" internal/ modules/ --include="*.go"
```
→ zero rows.

- [ ] **Step 2: Build + test + commit**

Run: `GOTMPDIR=C:/gotmp go build ./... && GOTMPDIR=C:/gotmp go test ./internal/characters/ ./internal/hooks/ ./internal/configs/ ./internal/state/... -count=1`
Expected: PASS.

```bash
git add internal/characters/ internal/hooks/ internal/configs/ internal/state/
git commit -m "refactor(u10): delete CalcConcentrationChance and its two knobs — the contest replaced the curve"
```

---

### Task 6b: The throttle interrupt becomes an opposed contest

**Files:**
- Modify: `internal/actions/combat_throttle.go:148-159` (+ the :69 doc
  comment)
- Modify: `internal/configs/config.balance.go:248`,
  `config.balance.misc.go:251-256` (delete `ThrottleInterruptChance`)
- Modify: `internal/combat/contest_site_guard_test.go` (allowlist row)
- Test: `internal/actions/combat_throttle_test.go` (extend or create —
  check for an existing file first)

- [ ] **Step 1: Write the failing test**

```go
package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/skills"
)

// The throttle interrupt is an opposed contest (owner 2026-08-21):
// throttler Dex+unarmed vs caster Wil+spellcasting. A hopeless throttler
// cannot reliably strip a master's cast, and a held cast fires exactly one
// spellcasting event (success-only progression).
func TestThrottleInterrupt_OpposedAndSuccessOnlyProgression(t *testing.T) {
	// Build a throttler and a casting target using this package's existing
	// combat-action test scaffolding (see the other combat_*_test.go files
	// for actor construction; the target must be mid-cast via the Activity
	// machine the same way hooks' newCastingChar does it).
	// Master caster: Wil.Base high + spellcasting rank high; RecalculateStats.
	// Novice throttler: low Dex, unarmed 1.
	// Run ExecuteThrottle enough times (re-entering the cast each time) and
	// assert: interrupts happen at most ~ContestFloor-often (allow 3x slack),
	// and GetSkillUseCount(spellcasting) delta equals the number of HELD casts.
}
```

The scaffold here is genuinely new — build it from the nearest existing
`internal/actions` combat test (grep for tests exercising `ExecuteThrottle`
or siblings like `ExecuteBash`); if the package has no runnable throttle
test, write the contest-shape assertions at whatever level the existing
tests exercise moves. Do not skip the progression-delta assertion.

- [ ] **Step 2: Swap the coin for the contest**

Replace :148-159:

```go
		// Cast interrupt (U10): an opposed contest — the throttler's grip
		// against the caster's hold. Telegraphed NoDamageInterrupt casts are
		// not contestable, matching both concentration paths (small
		// deliberate fix: the old flat coin ignored the flag).
		if target.Char.IsCasting() {
			interruptible := true
			if cs, ok := target.Char.Activity.CastingData(); ok {
				if sd := spells.GetSpell(cs.SpellId); sd != nil && sd.NoDamageInterrupt {
					interruptible = false
				}
			}
			if interruptible {
				w := float64(cfg.SkillWeight)
				atk := float64(actor.GetCharacter().Stats.Dexterity.ValueAdj) +
					float64(actor.GetCharacter().GetSkillLevel(skills.UnarmedCombat))*w
				def := float64(target.Char.Stats.Willpower.ValueAdj) +
					float64(target.Char.GetSkillLevel(skills.Spellcasting))*w
				res := combat.RunContest(atk, []contest.Entry{{Score: def}})
				if res.Success {
					var attackerRef state.ActorRef
					if actor.IsPlayer() {
						attackerRef = state.ActorRef{UserId: actor.GetUserId()}
					} else {
						attackerRef = state.ActorRef{MobInstanceId: actor.GetMobInstanceId()}
					}
					interrupted = InterruptTargetCast(target.Char, attackerRef)
				} else {
					// Held: success-only progression for the defence. The
					// throttler's unarmed use already progressed via the
					// move's own hit resolution.
					target.Char.OnSkillUse(string(skills.Spellcasting), target.Char.GetUserId())
				}
			}
		}
```

(`cfg` is the balance config already in scope; add `contest` and `spells`
imports — `combat` and `skills` are already imported.) Update the :69 doc
comment. Delete the `ThrottleInterruptChance` field and its defaulting
block; `grep -rn "ThrottleInterruptChance" internal/ modules/` → zero rows.

- [ ] **Step 3: Guard row**

```go
	"internal/actions/combat_throttle.go:ExecuteThrottle": "U10 (throttle cast-interrupt opposed contest)",
```

- [ ] **Step 4: Run + commit**

Run: `GOTMPDIR=C:/gotmp go test ./internal/actions/ ./internal/combat/ ./internal/configs/ -count=1` → PASS.

```bash
git add internal/actions/ internal/combat/contest_site_guard_test.go internal/configs/
git commit -m "feat(u10): throttle cast-interrupt is an opposed contest — grip vs hold"
```

---

### Task 7: Knockdown becomes an opposed contest (bridge deleted)

**Files:**
- Modify: `internal/combat/skill_moves.go` (the Task 1 bridge → contest)
- Modify: `internal/combat/contest_site_guard_test.go` (allowlist row)
- Modify: the five certainty tests (named below)
- Test: `internal/combat/skill_moves_knockdown_test.go` (create)

- [ ] **Step 1: Write the calibration + progression tests**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// Parity calibration (spec section 4): observed = p + F(1-2p) under
// RunWithFloors' one-symmetric-flip semantics.
func TestKnockdownFactor_ParityCalibration(t *testing.T) {
	floor := float64(configs.GetBalanceConfig().ContestFloor)
	const n = 200000
	cases := []struct {
		name   string
		factor float64
		want   float64
	}{
		{"bash", 1.0, 0.50},
		{"trip", 1.057, 0.60},
		{"kick", 0.924, 0.35},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wins := 0
			for i := 0; i < n; i++ {
				if RunContest(100*tc.factor, []contest.Entry{{Score: 100}}).Success {
					wins++
				}
			}
			got := float64(wins) / n
			want := tc.want + floor*(1-2*tc.want)
			if got < want-0.02 || got > want+0.02 {
				t.Errorf("%s: parity knockdown rate = %.3f, want %.3f +-0.02", tc.name, got, want)
			}
		})
	}
}
```

Progression (success-only, defender fires only on a RESIST): drive
`ExecuteSkillMove` with the seam tests' forced-hit runner
(`skill_moves_seam_test.go` shows the pattern), N=200 iterations, assert
`defender.GetSkillUseCount(unarmed)` delta equals the count of iterations
where `result.KnockedDown` was false (contest happened, defender won). Use
`pinContestFloorOff(t)` where determinism matters.

- [ ] **Step 2: Replace the bridge**

```go
		// Knockdown is an opposed contest (U10): the move's attack score
		// times its parity-anchored factor, against the defender's
		// Dex + unarmed-combat x SkillWeight. Control-immune defenders are
		// immovable and never contest — the blow still lands and deals
		// damage. Factor 0 = the move has no knockdown component (no
		// contest, no progression).
		if !mutations.IsControlImmune(p.Defender.Mutations) && p.KnockdownFactor > 0 {
			defScore := float64(p.Defender.Stats.Dexterity.ValueAdj) +
				float64(p.Defender.GetSkillLevel(skills.UnarmedCombat))*float64(configs.GetBalanceConfig().SkillWeight)
			kd := RunContest(p.Attack.score()*p.KnockdownFactor, []contest.Entry{{Score: defScore}})
			if kd.Success {
				result.KnockedDown = true
			} else {
				// Success-only progression: the defender fires one
				// unarmed-combat event only when they RESIST. The attacker's
				// hit already progressed via the seam.
				p.Defender.OnSkillUse(string(skills.UnarmedCombat), p.Defender.GetUserId())
			}
		}
```

Drop the `dice` import if now unused in the file.

- [ ] **Step 3: Fix the five certainty tests — named, with the pattern**

Each of `control_immune_test.go:37`, `knockdown_nil_test.go:66`,
`skill_moves_partial_test.go:38`, `skill_moves_grapple_test.go:16,34`,
`skill_moves_seam_test.go:37` currently means "guaranteed knockdown". Under
a floored contest nothing is guaranteed; convert each to:

```go
	pinContestFloorOff(t) // run_contest_test.go:25 — floor 0 for determinism
	// ...and in the SkillMoveParams:
	KnockdownFactor: 1000.0, // overwhelming: certain knockdown with floor off
```

with the defender left at scaffold-default stats. Where a test needs
guaranteed NO knockdown, use `KnockdownFactor: 0`. Do not leave any
single-shot `result.KnockedDown` assert running with the floor on.

- [ ] **Step 4: Guard row**

```go
	"internal/combat/skill_moves.go:executeSkillMoveWithRunner": "U10 (knockdown opposed contest after a landed special move)",
```

- [ ] **Step 5: Run + commit**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ ./internal/actions/ ./internal/hooks/ -count=1` → PASS.

```bash
git add internal/combat/ internal/actions/ internal/hooks/
git commit -m "feat(u10): knockdown is an opposed contest — factor-scaled attack vs Dex+unarmed"
```

---

### Task 8: Prone recovery becomes an opposed contest

**Files:**
- Modify: `internal/characters/skills.go:47-100` (`AttemptRecovery`)
- Create: `internal/hooks/recovery_contest.go`
- Modify: `internal/hooks/NewRound_UserRoundTick.go:245`,
  `internal/hooks/NewRound_MobRoundTick.go:182`
- Modify: `internal/combat/contest_site_guard_test.go` (allowlist row)
- Test: `internal/characters/skills_recovery_test.go` (create — NO existing
  AttemptRecovery tests exist anywhere; the scaffold below is built from
  scratch)

- [ ] **Step 1: Write the failing tests (scaffold included)**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// newProneCharForTest builds a validated character and puts it prone with
// the given MinRecoveryRounds. (No prior AttemptRecovery tests existed;
// this scaffold is new. Validate() is what attaches the Position FSM.)
func newProneCharForTest(t *testing.T, minRounds int) *Character {
	t.Helper()
	c := New()
	c.Validate()
	if err := c.Position.TransitionToProne(
		position.ProneData{MinRecoveryRounds: minRounds},
		state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward},
	); err != nil {
		t.Fatalf("prone transition: %v", err)
	}
	return c
}
```

(Verify the exact `New()`/`Validate()`/transition trigger names against
`internal/characters` and `internal/state/position` before running — the
hooks-package scaffold at `hooks_test.go:420-429` shows a working
prone+casting construction to crib from, adjusted for package.)

```go
func TestAttemptRecovery_NoOpponentAutoStands(t *testing.T) {
	c := newProneCharForTest(t, 0)
	before := c.GetSkillUseCount(string(skills.UnarmedCombat))
	attempted, success := c.AttemptRecovery(nil)
	if !attempted || !success {
		t.Fatalf("free recovery: attempted=%v success=%v, want true/true", attempted, success)
	}
	if !c.IsStanding() {
		t.Fatal("should be standing after a free recovery")
	}
	if c.GetSkillUseCount(string(skills.UnarmedCombat)) != before {
		t.Fatal("a free stand is not a contest and must fire no progression")
	}
}

func TestAttemptRecovery_ContestOutcomesAndProgression(t *testing.T) {
	c := newProneCharForTest(t, 0)
	before := c.GetSkillUseCount(string(skills.UnarmedCombat))
	if attempted, success := c.AttemptRecovery(func() bool { return false }); !attempted || success {
		t.Fatalf("lost contest: attempted=%v success=%v, want true/false", attempted, success)
	}
	if c.IsStanding() {
		t.Fatal("must stay down on a lost contest")
	}
	if c.GetSkillUseCount(string(skills.UnarmedCombat)) != before {
		t.Fatal("a LOST recovery fires nothing (success-only rule)")
	}
	if attempted, success := c.AttemptRecovery(func() bool { return true }); !attempted || !success {
		t.Fatalf("won contest: attempted=%v success=%v, want true/true", attempted, success)
	}
	if got := c.GetSkillUseCount(string(skills.UnarmedCombat)); got != before+1 {
		t.Fatalf("a WON recovery fires exactly one unarmed event: %d -> %d", before, got)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `GOTMPDIR=C:/gotmp go test ./internal/characters/ -run TestAttemptRecovery -count=1`
Expected: FAIL (signature).

- [ ] **Step 3: Change AttemptRecovery**

New signature + doc; the position gate and MinRecoveryRounds block
(:48-69) stay byte-identical:

```go
// AttemptRecovery tries to stand a prone/supine character once per round.
// contestWin is the opposed recovery contest against whoever is holding the
// character down, built by the caller (internal/hooks/recovery_contest.go)
// — nil means nobody in the room has aggro on this character and the stand
// is automatic once MinRecoveryRounds is consumed (owner 2026-08-21). The
// old solo Dex curve is gone: recovery is either contested or free.
func (c *Character) AttemptRecovery(contestWin func() bool) (bool, bool) {
```

Replace the curve+roll (:71-85):

```go
	success := true
	if contestWin != nil {
		success = contestWin()
		if success {
			// Success-only progression (U10b convention): one
			// unarmed-combat event per WON recovery contest. Free stands
			// and lost contests fire nothing.
			c.OnSkillUse(string(skills.UnarmedCombat), c.GetUserId())
		}
	}
```

Tail (:87-99) unchanged. Drop `math` and `dice` imports if unused.

- [ ] **Step 4: The hooks-side contest builder**

`internal/hooks/recovery_contest.go`. FIRST read
`internal/characters/context.md:961-964` (direct `Aggro` reads are
forbidden for new code — use the accessor/predicate it prescribes; if the
prescribed accessor does not cover "who is this actor targeting", mirror
the nearest compliant site and note it in the commit message):

```go
package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// recoveryContest builds the opposed prone-recovery roll for
// AttemptRecovery. Contested only when someone is actually holding the
// recoverer down: any LIVING actor in the room whose aggro is ON the
// recoverer (spec section 5 — keying on the recoverer's own aggro would
// free-stand a passive victim mid-fight and invite a drop-target exploit).
// Opponent = the strongest such holder by recovery score. Returns nil — a
// free stand — when nobody qualifies. Standard ContestFloor applies, so
// nobody is pinned forever and no stand is certain against live opposition.
func recoveryContest(ch *characters.Character) func() bool {
	room := rooms.LoadRoom(ch.RoomId)
	if room == nil {
		return nil
	}
	selfUser, selfMob := ch.GetUserId(), ch.MobInstanceId
	w := float64(configs.GetBalanceConfig().SkillWeight)
	score := func(c *characters.Character) float64 {
		return float64(c.Stats.Dexterity.ValueAdj) +
			float64(c.GetSkillLevel(skills.UnarmedCombat))*w
	}

	var best *characters.Character
	consider := func(c *characters.Character) {
		if c == nil || c.Health < 1 || c.Aggro == nil {
			return
		}
		targetsRecoverer := (selfUser > 0 && c.Aggro.UserId == selfUser) ||
			(selfMob > 0 && c.Aggro.MobInstanceId == selfMob)
		if !targetsRecoverer {
			return
		}
		if best == nil || score(c) > score(best) {
			best = c
		}
	}
	for _, mid := range room.GetMobs() {
		if m := mobs.GetInstance(mid); m != nil {
			consider(&m.Character)
		}
	}
	for _, uid := range room.GetPlayers() {
		if u := users.GetByUserId(uid); u != nil {
			consider(u.Character)
		}
	}
	if best == nil {
		return nil
	}

	self, other := score(ch), score(best)
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

- [ ] **Step 5: Guard row**

```go
	"internal/hooks/recovery_contest.go:recoveryContest": "U10 (prone-recovery opposed contest; free stands never contest)",
```

- [ ] **Step 6: Run + commit**

Run: `GOTMPDIR=C:/gotmp go test ./internal/characters/ ./internal/hooks/ ./internal/combat/ -count=1` → PASS.
Confirm no other caller: `grep -rn "AttemptRecovery(" internal/ --include="*.go"` → exactly the two callers + the new tests.

```bash
git add internal/characters/ internal/hooks/ internal/combat/contest_site_guard_test.go
git commit -m "feat(u10): prone recovery contests whoever holds you down; free stand otherwise"
```

---

### Task 9: The Done-when guard (HEAD blob, not disk)

**Files:**
- Create: `internal/combat/u10_done_when_test.go`

- [ ] **Step 1: Write the guard**

```go
package combat

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// U10's "Done when" list as a test (the U6b lesson: prose criteria fail
// silently). Yaml assertions read the HEAD blob via git show — the working
// copy is skip-worktree and deliberately stale on dev machines.
func TestU10DoneWhen_DeadPathsStayDead(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))

	readFile := func(rel string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}
	mustNotContain := func(content, name, needle, why string) {
		t.Helper()
		if strings.Contains(content, needle) {
			t.Errorf("%s still contains %q — %s", name, needle, why)
		}
	}

	mustNotContain(readFile("internal/characters/cast_helpers.go"),
		"cast_helpers.go", "CalcConcentrationChance",
		"the flat concentration curve was deleted by U10")
	mustNotContain(readFile("internal/combat/skill_moves.go"),
		"skill_moves.go", "RollStat(50",
		"knockdown is an opposed contest, not a flat roll")
	mustNotContain(readFile("internal/characters/skills.go"),
		"skills.go", "RollStat(50",
		"prone recovery is contested or free, never a solo roll")
	mustNotContain(readFile("internal/actions/combat_throttle.go"),
		"combat_throttle.go", "ThrottleInterruptChance",
		"the throttle interrupt is an opposed contest, not a coin")

	// ConcentrationFloor single-reader rule (Done-when 2): the only
	// production reference outside the config package is the seam file.
	out, err := exec.Command("git", "-C", repoRoot, "grep", "-l",
		"ConcentrationFloor", "--", "internal/", "modules/").Output()
	if err != nil {
		t.Fatalf("git grep: %v", err)
	}
	for _, f := range strings.Fields(string(out)) {
		if strings.Contains(f, "configs/") || strings.HasSuffix(f, "_test.go") {
			continue
		}
		if f != "internal/combat/run_concentration_contest.go" {
			t.Errorf("unexpected ConcentrationFloor reader: %s (the seam must stay the only one)", f)
		}
	}

	// Yaml criteria against the HEAD blob (Done-when 4).
	blob, err := exec.Command("git", "-C", repoRoot, "show",
		"HEAD:_datafiles/config.yaml").Output()
	if err != nil {
		t.Fatalf("git show config.yaml: %v", err)
	}
	yaml := string(blob)
	mustNotContain(yaml, "HEAD:config.yaml", "SpellInitiationWillpowerDivisor",
		"the knob died with the curve")
	mustNotContain(yaml, "HEAD:config.yaml", "KnockdownChance",
		"percent knobs became parity-anchored factors")
	for _, key := range []string{"ConcentrationFloor:", "ConcentrationDamageThresholdPct:",
		"BashKnockdownFactor:", "TripKnockdownFactor:", "KickKnockdownFactor:"} {
		if !strings.Contains(yaml, key) {
			t.Errorf("HEAD config.yaml missing shipped knob %s", key)
		}
	}
}
```

(If `git` is unavailable in some CI context, the test fails loudly rather
than silently passing — acceptable; CI has git.)

- [ ] **Step 2: Run + commit**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ -run "TestU10DoneWhen|TestKnockdownFactor|TestRunConcentrationContest" -count=1` → PASS.

```bash
git add internal/combat/u10_done_when_test.go
git commit -m "test(u10): the Done-when list as a guard — dead paths stay dead, one floor reader"
```

---

### Task 10: Docs (context.md, helpfiles, roadmap, patch notes)

**Files:**
- Modify: `internal/combat/context.md`, `internal/characters/context.md`,
  `internal/hooks/context.md`, `internal/configs/context.md` (if it lists
  knobs), and `internal/state/position/context.md`,
  `internal/state/control/context.md`, `internal/state/activity/context.md`
  (all three reference `CalcConcentrationChance` — reword to the contest)
- Modify: **helpfiles** (dogmud template layer):
  - `_datafiles/world/dogmud/templates/help/grapple.template:116` —
    "Willpower decides how often your concentration holds" → Willpower AND
    spellcasting skill contest the hold
  - `cast.template:16` — same correction
  - `prone.template:40,53-54` — recovery is now a contest against whoever
    is holding you down; with nobody on you, you simply stand
  - `stand.template:20,29` — the "unlike automatic recovery" percent claim
    is half-wrong out of combat now; reword without numbers
- Modify: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`:
  - U10 section: shipped shape (lattice ×10 as-is with the corrected
    harsher table, opposed throttle, success-only progression, factors)
  - the unowned-sites table (~line 115): throttle row → claimed by U10
  - **add the U10d row** (surprise-attack redesign; brainstorm → spec →
    plan; sequenced with U10b/U10c before U12) and update the ~line 526
    follow-up note to point at U10d
  - U10b row: note U10's three sites already fire on its convention
- Modify: `docs/PATCH_NOTES.md`
- Modify: `CLAUDE.md` "Dice & Rolling System": one line — concentration
  contests run through `combat.RunConcentrationContest` (floored by
  `ConcentrationFloor`, not `ContestFloor`)

- [ ] **Step 1: Update each file; verify every named symbol exists**

Patch notes (player-facing, no numbers, no em dashes, ESL-clear, wrap ~78):

```markdown
## 2026-08-21: Holding a spell is a skill

Keeping a spell together while someone hits you is now a real contest, and
your spellcasting training is half of it. A practiced caster holds through
pain that would shatter a novice. Small scrapes no longer threaten your
casting at all. Being wrestled to the ground is still the surest way to
stop a caster, and a hand on your throat now tests your training against
your attacker's grip instead of rolling plain luck.

Knocking someone down now has to beat them. A trip or bash measures your
skill against their footwork, so clumsy attackers cannot floor a master by
luck alone. Getting back up is a contest against whoever is standing over
you. If nobody is on you, you simply stand.
```

- [ ] **Step 2: Commit**

```bash
git add internal/ _datafiles/world/dogmud/templates/help/ docs/ CLAUDE.md
git commit -m "docs(u10): context.md + helpfile sweep, roadmap annotation (throttle claimed, U10d filed), patch notes"
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

- [ ] **Step 3: Push, PR, verify checks AND annotations**

```bash
git push -u origin feature/u10-disruption-model
gh pr create --repo pruuk/DOGMud --base master --head feature/u10-disruption-model \
  --title "U10: the disruption model" --body "$(cat <<'EOF'
Concentration, the throttle cast-interrupt, knockdown, and prone recovery
become real contests on the arc's seam. Details in the rev-2 spec
(docs/superpowers/specs/2026-08-21-u10-disruption-model-design.md); owner
decisions in its section 7. Named behaviour changes: chip damage below 10%
of the pool no longer threatens casts; prone/grapple holds land harsher
than the old curve (lattice x10, re-ratified); throttle interrupts are
opposed (was a flat 75% coin); knockdown and recovery scale with the
matchup; NoDamageInterrupt now also covers throttle; progression at the new
sites is success-only (U10b's convention from birth). Surprise attack is
explicitly split out as U10d.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
gh pr checks <n> --repo pruuk/DOGMud --watch
```

Then verify annotations on the validate runs
(`gh api repos/pruuk/DOGMud/check-runs/<id>/annotations --jq length` → 0).
Do not merge without the owner's word; the arc no-deploy policy holds
regardless.
