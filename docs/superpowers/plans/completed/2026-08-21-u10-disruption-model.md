# U10 Disruption Model Implementation Plan (rev 3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Concentration, the throttle cast-interrupt, knockdown, and prone
recovery become real contests on the arc's seam; knockdown ships as a NAMED
rebalance to its intended rates.

**Architecture:** A new `combat.RunConcentrationContest` (the ONE reader of
`ConcentrationFloor`) resolves every concentration contest with the caster
as the attack side — static difficulties for the damage/position paths
(damagePct×10 with a 10% threshold; chunk-4f lattice ×10), the throttler's
live score for the throttle interrupt. Knockdown and prone recovery run
through `combat.RunContest` (standard floor); knockdown thresholds become
parity-anchored score factors targeting the RATIFIED intended rates.
Progression fires success-only. An adversarial playtest gate closes the
slice.

**Tech Stack:** Go; `internal/contest` core; TWO guards
(`internal/combat/contest_site_guard_test.go` allowlist + root
`contest_floor_guard_test.go` exemptions); playtest harness.

**Spec:** `docs/superpowers/specs/completed/2026-08-21-u10-disruption-model-design.md`
rev 3 (§7 owner decisions binding). Branch: `feature/u10-disruption-model`.

**Verified facts (all checked 2026-08-21 against source; do not re-derive):**
- `dice.RollStat(mean)` is **Normal(mean, mean×RollSpread)**
  (`dice.go:451-453`), so `RollStat(50)` = Normal(50, 7.5). The old
  knockdown knobs are thresholds on that curve: TRUE live rates are
  bash 50%, trip ~91%, kick ~2.3% — NOT the 50/60/35 the comments claim.
  Owner ratified anchoring the new contest to the INTENDED 50/60/35 as a
  named rebalance (spec §4).
- `contest.RunWithFloors(atkScore, entries, floor)` (`contest.go:151`):
  floor FLIPS with probability `floor` → observed `p + floor·(1−2p)`. Both
  sides roll with the ATTACKER's stdDev (`contest.go:97-103`).
- Config type `configs.Balance` (`config.balance.go:7`),
  `func (b *Balance) Validate()` (`:900`); `Balance{}` + `Validate()` is
  the package's test pattern (`config.balance.flee_test.go:10-11`).
  ⚠️ `config.balance.go:239-243` INTERLEAVES the knockdown knobs with
  `TripDamagePercent` (:240) and `KickDamagePercent` (:242), and
  `config.balance.misc.go:230-244` interleaves their defaulting blocks the
  same way — replace ONLY the knockdown lines, never the quoted ranges
  wholesale (deleting the DamagePercent defaults compiles silently).
  `ThrottleInterruptChance`: field `:248`, defaults
  `config.balance.misc.go:251-256`, NOT in config.yaml.
- HEAD `config.yaml` anchors: `ContestFloor: 0.125` :546 (and :540-541
  carries a "concentration ... are not floored" comment that becomes false
  — fix it in Task 2); knockdown block :663-670; the dead concentration
  pair + its comment block :1247-1255.
- `AttackSide.score()` (`defence_multiplier.go:80`), unexported, same
  package as `skill_moves.go`. **The channel seam ALREADY fires the
  defender's winning-defence skill once per contested swing, win or lose**
  (`AwardDefenceProgression`, `defence_multiplier.go:363-373`; a bare
  defender's winning candidate is dodge, which trains unarmed-combat) —
  knockdown progression tests must expect `N + resists`, not `resists`.
- `characters.Character`: `GetUserId()` (`character.go:465`, 0 for mobs),
  `MobInstanceId`, `GetSkillLevel(skills.SkillTag) int`,
  `OnSkillUse(skillName string, userId int) bool` (nil-map-safe),
  `GetSkillUseCount(skillName string) int` (`progression.go:522`),
  `RecalculateStats()` (`validate.go:29`), `New()` (`character.go:351`),
  **`Attackers() []state.ActorRef`** (`character.go:795`, "snapshot of
  inbound attacker list") — the prescribed accessor;
  `characters/context.md:961-964` FORBIDS new direct `Aggro` reads.
- Skill tags: `skills.Spellcasting`, `skills.UnarmedCombat`
  (`skills.go:30,32`).
- Concentration callers: `checkConcentrationBreak`
  (`combat_shared_helpers.go:123`; SIX tests at `hooks_test.go:347-415`),
  sole production caller `handlePlayerConcentrationBreak`
  (`NewRound_DoCombat_helpers.go:857`, players only). Position block
  `:549-581`, callers `:288` (user) / `:498` (mob). `util.` appears in
  this file at exactly :142-143 (deleted by Task 4) and :559-560 (deleted
  by Task 5) — **the `util` import must be removed in Task 5** or its
  commit does not compile. Hooks scaffolds: `newCastingChar` (:337),
  `newProneCastingChar` (:420); the seeded cast has tiny FoldsNeeded, so
  loop-tests must seed a big-fold spell via `spells.SeedSpellsForTest`
  (model: `TestProcessFoldRound_NoDamageInterrupt_SkipsPositionBreak`,
  hooks_test.go:431-439) or they go ~2% flaky.
- Throttle: interrupt at `combat_throttle.go:151` inside `ExecuteThrottle`
  (`cfg` in scope at :96; `interrupted`/`target.Char` real;
  `InterruptTargetCast(*characters.Character, state.ActorRef) bool`).
  `NoDamageInterrupt` is NOT currently checked there. **An existing test
  file pins the coin**: `internal/actions/combat_throttle_test.go:165-266`
  (`TestThrottle_CastInterrupt`) forces the deleted knob via string-keyed
  `configs.AddOverlayOverrides` and asserts a guaranteed interrupt — it
  must be reworked in Task 6b (its scaffold — `setCastingForTest`,
  `newStubActor`, fanged-species seeding, retry-until-hit loop — is
  exactly what the new test needs; do NOT write a scaffold from scratch).
  `util` stays imported there (`GetRoundCount` :176). Throttle is mob-only
  by anatomy (`:92` requires handless + fanged).
- `AttemptRecovery` (`skills.go:47`): exactly two callers
  (`NewRound_UserRoundTick.go:245`, `tickMobProneRecovery`
  `NewRound_MobRoundTick.go:182`); NO existing tests; `math` stays used
  elsewhere in the file (:302), `dice` goes. The manual `stand` command
  (`usercommands/stand.go:88`) stands unconditionally after a stamina gate
  — deliberate paid bypass (spec §5), untouched by this plan.
- `KnockdownChance` sites: **14 struct-literal setters + 2 local-var
  feeds** — `internal/actions`: `combat_bash.go:116`, `combat_gore.go:113`,
  `combat_pounce.go:128`, `combat_trip.go:144` (fed by local :111),
  `combat_kick.go:159` (fed by local :119), `combat_throttle.go:125`,
  `combat_drain.go:131`, `combat_fire.go:271`, `combat_hamstring.go:122`,
  `combat_maul.go:118`, `combat_rake.go:118`;
  `internal/combat/counter.go:119`;
  `internal/hooks/combat_shared_helpers.go:299` (auto-trip, reads
  `cfg.TripKnockdownChance`) and `:358` (auto-bash, reads
  `cfg.BashKnockdownChance`). Five test files construct the field:
  `skill_moves_seam_test.go:37`, `skill_moves_partial_test.go:38`,
  `skill_moves_grapple_test.go:16,34`, `knockdown_nil_test.go:66`,
  `control_immune_test.go:37` — several mean GUARANTEED knockdown; under a
  floored contest that needs `pinContestFloorOff(t)`
  (`run_contest_test.go:25`) + an overwhelming factor.
- Knockdown roll: `skill_moves.go:175-176` inside
  `executeSkillMoveWithRunner` (:113); immunity gate :174. The file
  imports neither `skills` nor `configs` — Task 7 adds both.
- Root guard `contest_floor_guard_test.go`: fails any file calling
  `contest.RunWithFloors` not in `guardedRollExemptions["contest"]`
  (today: `internal/combat/combat_helpers.go`,
  `internal/combat/run_contest.go`). `combat.RunConcentrationContest` is
  NOT a guarded selector in the site guard, so its callers (hooks,
  actions) need no allowlist rows — only the defining file needs the two
  guard entries from Task 3.
- `configs.SetConfigForTest(t, cfg)` (`testing_support.go:30`);
  `pinContestFloorOff` exists only in `internal/combat` tests — other
  packages inline the SetConfigForTest pattern.
- `_datafiles/config.yaml` is skip-worktree; yaml assertions read
  `git show HEAD:_datafiles/config.yaml`. YAML decode is non-strict —
  stale keys in the dev disk copy are harmless.

---

### Task 1: New balance knobs + the knockdown field rename (everything compiles)

**Files:**
- Modify: `internal/configs/config.balance.go` (:91 area; the three
  knockdown lines within :239-243 ONLY — the DamagePercent lines between
  them stay)
- Modify: `internal/configs/config.balance.misc.go` (the three knockdown
  defaulting blocks within :230-244 ONLY; leave `ThrottleInterruptChance`
  :251-256 for Task 6b)
- Modify: `internal/configs/config.balance.spells.go` (new concentration
  defaults)
- Modify: `internal/combat/skill_moves.go` (field rename + bridge)
- Modify: ALL sites in the Verified-facts inventory (14 setters + 2 local
  feeds + 5 test files)
- Test: `internal/configs/configs_balance_u10_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package configs

import "testing"

// U10 knobs: concentration floor/threshold and the three knockdown factors
// (anchored to the RATIFIED intended rates — see the spec's section 4; the
// old threshold knobs never delivered the rates their comments claimed).
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

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTMPDIR=C:/gotmp go test ./internal/configs/ -run TestU10KnobDefaults -count=1`
Expected: FAIL (undefined fields).

- [ ] **Step 3: Add fields and defaults**

`config.balance.go`, next to `ContestFloor` (:91):

```go
	// ConcentrationFloor is the symmetric last-resort flip probability for
	// concentration contests ONLY (all three triggers: damage, position,
	// throttle). The standard ContestFloor (0.125) would break a master's
	// concentration one disruption in eight; concentration gets its own,
	// much smaller mercy band. Read in exactly one place:
	// combat.RunConcentrationContest.
	ConcentrationFloor ConfigFloat `yaml:"ConcentrationFloor"` // default 0.02

	// ConcentrationDamageThresholdPct: damage below this percent of the
	// caster's health pool does not roll for concentration at all. Chip
	// damage should not generate rolls. Values below 1 are rewritten to
	// the default; "roll on any hit" is expressed as 1.
	ConcentrationDamageThresholdPct ConfigInt `yaml:"ConcentrationDamageThresholdPct"` // default 10
```

Replace ONLY the three knockdown lines inside :239-243 (leave
`TripDamagePercent`/`KickDamagePercent` untouched between them):

```go
	BashKnockdownFactor ConfigFloat `yaml:"BashKnockdownFactor"` // Knockdown score factor; intended-rate anchor 50% at parity (default 1.0)
	TripKnockdownFactor ConfigFloat `yaml:"TripKnockdownFactor"` // Knockdown score factor; intended-rate anchor 60% at parity (default 1.057)
	KickKnockdownFactor ConfigFloat `yaml:"KickKnockdownFactor"` // Knockdown score factor; intended-rate anchor 35% at parity (default 0.924)
```

`config.balance.misc.go`: replace ONLY the three knockdown defaulting
blocks inside :230-244 (the DamagePercent blocks interleaved there stay):

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

`config.balance.spells.go`:

```go
	if b.ConcentrationFloor <= 0 || b.ConcentrationFloor > 0.5 {
		b.ConcentrationFloor = 0.02
	}
	if b.ConcentrationDamageThresholdPct < 1 {
		b.ConcentrationDamageThresholdPct = 10
	}
```

- [ ] **Step 4: Rename the struct field with a TRANSITIONAL bridge**

`skill_moves.go`: `KnockdownChance int` → `KnockdownFactor float64`. The
roll at :175-176 becomes the bridge **that Task 7 deletes**:

```go
		if !mutations.IsControlImmune(p.Defender.Mutations) {
			// TRANSITIONAL (U10 Task 1→7): threshold roll against
			// factor*50. NOT behavior-preserving for trip/kick — the old
			// thresholds sat on a normal curve and delivered ~91%/~2.3%;
			// the ratified target is 60%/35%, which the Task 7 contest
			// delivers. This bridge only keeps the tree green in between.
			knockdownRoll := dice.RollStat(50)
			if knockdownRoll.Value < p.KnockdownFactor*50.0 {
				result.KnockedDown = true
			}
		}
```

Update the fifteen production sites and two local feeds per the
Verified-facts inventory (`float64(cfg.BashKnockdownFactor)` etc.; hooks
:299/:358 switch to the new knob names; zero sites →
`KnockdownFactor: 0`). Update the five test files: `KnockdownChance: 100`
→ `KnockdownFactor: 2.0` (guaranteed under the bridge; Task 7 revisits).

Verify: `grep -rn "KnockdownChance" internal/ modules/ --include="*.go"` →
zero rows.

- [ ] **Step 5: Run the affected packages**

Run: `GOTMPDIR=C:/gotmp go test ./internal/configs/ ./internal/combat/ ./internal/actions/ ./internal/hooks/ -count=1`
Expected: PASS. (Bash behavior identical; trip/kick sit at the bridge's
interim ~65%/~31% until Task 7 — named in the commit.)

- [ ] **Step 6: Commit**

```bash
git add internal/configs/ internal/combat/ internal/actions/ internal/hooks/
git commit -m "config(u10): concentration knobs; knockdown thresholds become factors (transitional bridge, rebalance lands in the contest task)"
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

Fix the now-false comment at :540-541 (it says static-difficulty rolls
including concentration are not floored) to name the exception:

```yaml
  # Static-difficulty rolls -- search, track, forage -- are not floored.
  # Concentration IS floored, by its own ConcentrationFloor below, through
  # combat.RunConcentrationContest.
```

Next to `ContestFloor: 0.125` (:546):

```yaml
  # ConcentrationFloor: the symmetric mercy band for concentration
  #   contests only (damage, position, and throttle triggers). The
  #   standard ContestFloor would break a master one-in-eight per
  #   disruption; concentration holds get a much smaller band.
  ConcentrationFloor: 0.02
  # ConcentrationDamageThresholdPct: damage below this percent of the pool
  #   never rolls for concentration. Chip damage should not generate
  #   rolls. Minimum 1; there is no roll-on-any-hit zero setting.
  ConcentrationDamageThresholdPct: 10
```

Replace the knockdown block (:663-670):

```yaml
  # KnockdownFactor: multiplier on the attacker's score in the knockdown
  # contest (attack score x factor vs defender Dex + unarmed x SkillWeight).
  # Anchored to the INTENDED per-move rates at score parity (the old
  # threshold knobs claimed these numbers but never delivered them). The
  # contest floor pulls observed parity rates slightly toward even.
  BashKnockdownFactor: 1.0       # intended half-and-half at parity
  TripKnockdownFactor: 1.057     # intended strong-favorite at parity
  KickKnockdownFactor: 0.924     # intended underdog at parity
  ```

Delete the dead concentration pair and its whole comment block
(:1247-1255, including the "SURVIVES despite its name" comment).

- [ ] **Step 3: Commit clean, restore disk copy**

```bash
git add _datafiles/config.yaml
git diff --cached _datafiles/config.yaml   # REVIEW: only the U10 lines
git commit -m "config(u10): ship the disruption-model knobs; retire the dead concentration pair"
cp /tmp/config.yaml.diskbackup _datafiles/config.yaml
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml     # expect leading 'S'
```

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

// RunConcentrationContest resolves a caster's hold on a forming spell. The
// caster is always the attack side; disruption is a static difficulty
// (damagePct x10, position lattice x10) or a live opposing score (the
// throttle interrupt). It is the ONE place Balance.ConcentrationFloor is
// read, exactly as RunContest is for ContestFloor — concentration gets its
// own, much smaller mercy band because the standard floor would break a
// master one disruption in eight.
//
// Success = the caster HELD.
func RunConcentrationContest(casterScore, disruption float64) contest.Result {
	return contest.RunWithFloors(casterScore,
		[]contest.Entry{{Score: disruption}},
		float64(configs.GetBalanceConfig().ConcentrationFloor))
}
```

- [ ] **Step 4: Feed BOTH guards**

`contestSiteOwners` (`contest_site_guard_test.go:47`), under "The seams
themselves":

```go
	"internal/combat/run_concentration_contest.go:RunConcentrationContest": "the concentration floor seam — the ONE place Balance.ConcentrationFloor is read (U10)",
```

Root `contest_floor_guard_test.go`, `guardedRollExemptions["contest"]`:

```go
		"internal/combat/run_concentration_contest.go": "defines combat.RunConcentrationContest — the one ConcentrationFloor reader (U10)",
```

(Match each map's actual row shape — read the existing rows first.)

- [ ] **Step 5: Run package AND root guards**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ . -count=1`
Expected: PASS including the root `TestOpposedContestsAreFloored`.

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
  `NoDamageInterrupt_NeverBreaks`, `NormalSpell_StillBreaks`) — keep all
  six cases, update curve-specific assertions only

- [ ] **Step 1: Update the six tests + add threshold/progression pins**

Use the existing `newCastingChar(spellId)` scaffold (:337). Add:

```go
func TestCheckConcentrationBreak_BelowThresholdNeverRolls(t *testing.T) {
	ch := newCastingChar("mind-spike") // adapt spellId to the scaffold's users
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

(Stat setup rule where needed: `.Base` then `RecalculateStats()`.)

- [ ] **Step 2: Run to verify failure**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -run TestCheckConcentrationBreak -count=1`
Expected: FAIL.

- [ ] **Step 3: Rewrite the body**

Replace from `maxHealth :=` down (:135-144):

```go
	maxHealth := ch.HealthMax.Value
	damagePct := damage * 100 / maxHealth
	if damagePct < int(configs.GetBalanceConfig().ConcentrationDamageThresholdPct) {
		// Chip damage does not generate rolls at all (U10).
		return false
	}
	res := combat.RunConcentrationContest(concentrationScore(ch), float64(damagePct*10))
	if res.Success {
		// Success-only progression (U10b's success half, adopted from
		// birth): one spellcasting event per HELD contest.
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

(The file already imports `combat`, `configs`, `skills` — verify, add only
what's missing. Do NOT touch the `util` import yet; :559-560 still uses it
until Task 5.)

- [ ] **Step 4: Run + commit**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -count=1` → PASS.

```bash
git add internal/hooks/combat_shared_helpers.go internal/hooks/hooks_test.go
git commit -m "feat(u10): damage-path concentration is a contest with a chip-damage threshold"
```

---

### Task 5: Convert the position path (and remove the `util` import HERE)

**Files:**
- Modify: `internal/hooks/combat_shared_helpers.go:549-581` + its imports
- Test: extend the fold-round coverage; scaffold `newProneCastingChar`
  (:420); big-fold seeding model at hooks_test.go:431-439

- [ ] **Step 1: Pin behavior across the swap (flake-proofed)**

```go
func TestProcessFoldRound_ProneHopelessCasterBreaks(t *testing.T) {
	// Seed a many-fold spell so a lucky early hold cannot complete the
	// cast and end the loop early (the scaffold's default has tiny folds;
	// see TestProcessFoldRound_NoDamageInterrupt_SkipsPositionBreak for
	// the SeedSpellsForTest pattern).
	defer spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"u10-longcast": {SpellId: "u10-longcast", Name: "Longcast",
			Type: spells.HarmSingle, Cost: 10, BaseFolds: 512},
	})()
	ch := newProneCastingChar("u10-longcast")
	ch.Stats.Willpower.Base = 1
	ch.RecalculateStats()
	broke := false
	for i := 0; i < 200 && !broke; i++ {
		broke = processFoldRound(ch).ProneBroke
	}
	if !broke {
		t.Fatal("a Wil-1 caster folding while prone should break within 200 rounds")
	}
}
```

(Adapt the seed-map shape to what `SeedSpellsForTest` actually takes —
mirror hooks_test.go:431-439.)

- [ ] **Step 2: Compile-check**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -run TestProcessFoldRound -count=1`
Expected: PASS already — it pins the contract across the swap.

- [ ] **Step 3: Swap the roll**

Replace :555-561 (through the `if roll >= chance {` line — the break-branch
body and the passed-roll comment stay byte-identical):

```go
		dmgPctEquiv := position.PositionDisruptionDmgEquiv(posState, ctrlState)
		if dmgPctEquiv > 0 {
			// Chunk 4f's lattice keeps its full granularity; the x10
			// conversion is the design (owner 2026-08-21, re-ratified over
			// the corrected table — prone 300, deep holds 600-700).
			res := combat.RunConcentrationContest(concentrationScore(char), float64(dmgPctEquiv*10))
			if res.Success {
				// Success-only progression: one spellcasting event per
				// HELD round (melee fires per combat round on the same
				// basis; farming requires a live aggressor).
				char.OnSkillUse(string(skills.Spellcasting), char.GetUserId())
			}
			if !res.Success {
```

Delete the `util.LogRoll` line, **and remove the now-unused `util` import
from this file in this task** (its last two uses were :559-560). Update the
:539-548 comment to describe the contest, keeping the layering +
NoDamageInterrupt sentences.

- [ ] **Step 4: Run + commit**

Run: `GOTMPDIR=C:/gotmp go test ./internal/hooks/ -count=1` → PASS.

```bash
git add internal/hooks/combat_shared_helpers.go internal/hooks/hooks_test.go
git commit -m "feat(u10): position-path concentration contests the chunk-4f lattice at x10"
```

---

### Task 6: Delete the dead curve and its knobs

**Files:**
- Modify: `internal/characters/cast_helpers.go` (delete
  `CalcConcentrationChance` + :41-43 comment)
- Modify: `internal/characters/casting_test.go` (delete its curve test)
- Modify: `internal/hooks/combat_shared_helpers_test.go:20-46` (delete the
  two curve tests)
- Modify: `internal/configs/config.balance.go:487-488`,
  `config.balance.spells.go:18-23` (delete the two dead fields + defaults)
- Modify: `internal/state/position/disruption.go` lines 1-13 and 20-22
  (comments name the dead function — reword to "the concentration contest
  difficulty conversion in processFoldRound (x10)")

- [ ] **Step 1: Delete, compiler-first**

Delete the config FIELDS first, then every red site. Exit check (code
only; context.md prose is Task 10's):

```bash
grep -rn "SpellConcentrationBase\|SpellInitiationWillpowerDivisor\|CalcConcentrationChance" internal/ modules/ --include="*.go"
```
→ zero rows.

- [ ] **Step 2: Build + test + commit**

Run: `GOTMPDIR=C:/gotmp go build ./... && GOTMPDIR=C:/gotmp go test ./internal/characters/ ./internal/hooks/ ./internal/configs/ ./internal/state/... -count=1` → PASS.

```bash
git add internal/characters/ internal/hooks/ internal/configs/ internal/state/
git commit -m "refactor(u10): delete CalcConcentrationChance and its two knobs — the contest replaced the curve"
```

---

### Task 6b: The throttle interrupt through the concentration seam

**Files:**
- Modify: `internal/actions/combat_throttle.go:148-159` (+ the :69 doc
  comment)
- Modify: `internal/actions/combat_throttle_test.go:165-266` — the
  existing `TestThrottle_CastInterrupt` pins the deleted knob and MUST be
  reworked (its scaffold is reused, not rewritten)
- Modify: `internal/configs/config.balance.go:248`,
  `config.balance.misc.go:251-256` (delete `ThrottleInterruptChance`)

No guard rows: `RunConcentrationContest` is not a guarded selector, and
this file never calls `RunWithFloors`.

- [ ] **Step 1: Rework the existing test, add the contest pins**

In `TestThrottle_CastInterrupt`: drop the
`"Balance.ThrottleInterruptChance": 1.0` overlay entry; for a
deterministic guaranteed interrupt, pin the concentration floor to 0 via
the overlay/`SetConfigForTest` pattern the file already uses
(`"Balance.ConcentrationFloor": 0`... match the overlay key style at
:170-178) and leave the casting target at Wil-1/skill-0 against the
fanged attacker. Add a companion test: a caster with overwhelming
Wil/spellcasting (set `.Base`, `RecalculateStats()`) and floor pinned to 0
is NEVER interrupted across the retry loop, and
`GetSkillUseCount(spellcasting)` grows by exactly the number of held
interrupt contests (success-only rule).

- [ ] **Step 2: Swap the coin for the seam call**

Replace :148-159:

```go
		// Cast interrupt (U10): an opposed contest through the
		// concentration seam — the caster's hold (attack side, as in every
		// concentration contest) against the throttler's grip, floored at
		// ConcentrationFloor per the owner's 2% ruling. Telegraphed
		// NoDamageInterrupt casts are not contestable, matching both
		// concentration paths (deliberate small fix: the old flat coin
		// ignored the flag; throttle is mob-only by anatomy, so no player
		// counter-play is lost).
		if target.Char.IsCasting() {
			interruptible := true
			if cs, ok := target.Char.Activity.CastingData(); ok {
				if sd := spells.GetSpell(cs.SpellId); sd != nil && sd.NoDamageInterrupt {
					interruptible = false
				}
			}
			if interruptible {
				w := float64(cfg.SkillWeight)
				grip := float64(actor.GetCharacter().Stats.Dexterity.ValueAdj) +
					float64(actor.GetCharacter().GetSkillLevel(skills.UnarmedCombat))*w
				hold := float64(target.Char.Stats.Willpower.ValueAdj) +
					float64(target.Char.GetSkillLevel(skills.Spellcasting))*w
				res := combat.RunConcentrationContest(hold, grip)
				if !res.Success {
					var attackerRef state.ActorRef
					if actor.IsPlayer() {
						attackerRef = state.ActorRef{UserId: actor.GetUserId()}
					} else {
						attackerRef = state.ActorRef{MobInstanceId: actor.GetMobInstanceId()}
					}
					interrupted = InterruptTargetCast(target.Char, attackerRef)
				} else {
					// Held: success-only progression for the defence.
					target.Char.OnSkillUse(string(skills.Spellcasting), target.Char.GetUserId())
				}
			}
		}
```

(`cfg` is in scope at :96; `combat` and `skills` already imported; add
`spells`.) Update the :69 doc comment. Delete the `ThrottleInterruptChance`
field and defaulting block. Exit check (code only; the
`internal/actions/context.md:184` prose reference is Task 10's):

```bash
grep -rn "ThrottleInterruptChance" internal/ modules/ --include="*.go"
```
→ zero rows.

- [ ] **Step 3: Run + commit**

Run: `GOTMPDIR=C:/gotmp go test ./internal/actions/ ./internal/configs/ -count=1` → PASS.

```bash
git add internal/actions/ internal/configs/
git commit -m "feat(u10): throttle cast-interrupt contests the caster's hold through the concentration seam"
```

---

### Task 7: Knockdown becomes an opposed contest (bridge deleted; the rebalance lands)

**Files:**
- Modify: `internal/combat/skill_moves.go` (bridge → contest; add `skills`
  and `configs` imports; drop `dice` if unused)
- Modify: `internal/combat/contest_site_guard_test.go` (allowlist row)
- Modify: the five certainty tests (named in Verified facts)
- Test: `internal/combat/skill_moves_knockdown_test.go` (create)

- [ ] **Step 1: Calibration + progression tests**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
)

// Parity calibration (spec section 4): observed = p + F(1-2p) under
// RunWithFloors' one-symmetric-flip semantics. These are the RATIFIED
// intended rates — deliberately not the old accidental live rates.
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

Progression test: drive `ExecuteSkillMove` N=200 times with the seam
tests' forced-hit runner (`skill_moves_seam_test.go:63-80` pattern) and
`pinContestFloorOff(t)`. **Expected delta for the defender's
unarmed-combat count is `N + resists`, not `resists`:** the channel seam's
`AwardDefenceProgression` already fires the winning defence's skill once
per contested swing win-or-lose (a bare defender's winner is dodge →
unarmed-combat), and the knockdown resist event is additional. Assert
`delta == N + resists` with a comment saying exactly that.

- [ ] **Step 2: Replace the bridge**

```go
		// Knockdown is an opposed contest (U10): the move's attack score
		// times its intended-rate factor, against the defender's
		// Dex + unarmed-combat x SkillWeight. This is a NAMED REBALANCE:
		// the old thresholds delivered ~91% trip / ~2.3% kick despite
		// claiming 60/35 (normal-curve thresholds, not percentages); the
		// contest delivers the intended rates at parity. Control-immune
		// defenders are immovable and never contest. Factor 0 = no
		// knockdown component (no contest, no progression).
		if !mutations.IsControlImmune(p.Defender.Mutations) && p.KnockdownFactor > 0 {
			defScore := float64(p.Defender.Stats.Dexterity.ValueAdj) +
				float64(p.Defender.GetSkillLevel(skills.UnarmedCombat))*float64(configs.GetBalanceConfig().SkillWeight)
			kd := RunContest(p.Attack.score()*p.KnockdownFactor, []contest.Entry{{Score: defScore}})
			if kd.Success {
				result.KnockedDown = true
			} else {
				// Success-only progression: the defender fires one
				// unarmed-combat event only on a RESIST — on top of the
				// seam's ordinary per-contest defence award, which is
				// unchanged.
				p.Defender.OnSkillUse(string(skills.UnarmedCombat), p.Defender.GetUserId())
			}
		}
```

Add `skills` + `configs` imports; drop `dice` if now unused.

- [ ] **Step 3: Fix the five certainty tests**

`control_immune_test.go:37`, `knockdown_nil_test.go:66`,
`skill_moves_partial_test.go:38`, `skill_moves_grapple_test.go:16,34`,
`skill_moves_seam_test.go:37` — for guaranteed knockdown:
`pinContestFloorOff(t)` + `KnockdownFactor: 1000.0`; for guaranteed none:
`KnockdownFactor: 0`. No single-shot `KnockedDown` assert may run with the
floor on.

- [ ] **Step 4: Guard row**

```go
	"internal/combat/skill_moves.go:executeSkillMoveWithRunner": "U10 (knockdown opposed contest after a landed special move)",
```

- [ ] **Step 5: Run + commit**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ ./internal/actions/ ./internal/hooks/ -count=1` → PASS.

```bash
git add internal/combat/ internal/actions/ internal/hooks/
git commit -m "feat(u10): knockdown is an opposed contest at the intended rates — a named rebalance"
```

---

### Task 8: Prone recovery becomes an opposed contest

**Files:**
- Modify: `internal/characters/skills.go:47-100` (`AttemptRecovery`)
- Create: `internal/hooks/recovery_contest.go`
- Modify: `internal/hooks/NewRound_UserRoundTick.go:245`,
  `internal/hooks/NewRound_MobRoundTick.go:182`
- Modify: `internal/combat/contest_site_guard_test.go` (allowlist row)
- Modify: `internal/contest/contest.go` `AgainstDifficulty` doc comment
  (drop the "recovering from prone with nobody holding you down" example —
  that case is now a free stand with no roll)
- Test: `internal/characters/skills_recovery_test.go` (create — no
  AttemptRecovery tests exist anywhere; scaffold below is new)

- [ ] **Step 1: Write the failing tests (scaffold included)**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// newProneCharForTest: validated character, prone, with the given
// MinRecoveryRounds. Validate() attaches the Position FSM.
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
// AttemptRecovery tries the FREE automatic stand once per round for a
// prone/supine character. contestWin is the opposed recovery contest
// against whoever is holding the character down, built by the caller
// (internal/hooks/recovery_contest.go) — nil means nobody is attacking the
// recoverer and the stand is automatic once MinRecoveryRounds is consumed
// (owner 2026-08-21). The old solo Dex curve is gone: free recovery is
// either contested or automatic. The manual `stand` command is the
// separate, deliberate PAID exit: stamina buys an uncontested stand.
func (c *Character) AttemptRecovery(contestWin func() bool) (bool, bool) {
```

Replace the curve+roll (:71-85):

```go
	success := true
	if contestWin != nil {
		success = contestWin()
		if success {
			// Success-only progression (U10b's success half): one
			// unarmed-combat event per WON recovery contest. Free stands
			// and lost contests fire nothing.
			c.OnSkillUse(string(skills.UnarmedCombat), c.GetUserId())
		}
	}
```

Tail (:87-99) unchanged. Drop the `dice` import (`math` stays — used at
:302).

- [ ] **Step 4: The hooks-side contest builder — via Attackers()**

`internal/hooks/recovery_contest.go` (direct `Aggro` reads are forbidden;
`Attackers()` is the prescribed accessor and answers exactly "who has
aggro ON the recoverer"):

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

// recoveryContest builds the opposed prone-recovery roll for
// AttemptRecovery. Contested only when someone is actually holding the
// recoverer down: a living, same-room actor from the recoverer's inbound
// attacker set (Character.Attackers() — the accessor context.md
// prescribes over direct Aggro reads). Opponent = the strongest holder by
// recovery score. Returns nil — a free stand — when nobody qualifies.
// Standard ContestFloor applies, so nobody is pinned forever and no stand
// is certain against live opposition. (A player can always take the paid
// exit instead: the manual stand command is uncontested by design.)
func recoveryContest(ch *characters.Character) func() bool {
	w := float64(configs.GetBalanceConfig().SkillWeight)
	score := func(c *characters.Character) float64 {
		return float64(c.Stats.Dexterity.ValueAdj) +
			float64(c.GetSkillLevel(skills.UnarmedCombat))*w
	}

	var best *characters.Character
	for _, ref := range ch.Attackers() {
		var opp *characters.Character
		if ref.UserId > 0 {
			if u := users.GetByUserId(ref.UserId); u != nil {
				opp = u.Character
			}
		} else if ref.MobInstanceId > 0 {
			if m := mobs.GetInstance(ref.MobInstanceId); m != nil {
				opp = &m.Character
			}
		}
		if opp == nil || opp.Health < 1 || opp.RoomId != ch.RoomId {
			continue
		}
		if best == nil || score(opp) > score(best) {
			best = opp
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

Run: `GOTMPDIR=C:/gotmp go test ./internal/characters/ ./internal/hooks/ ./internal/combat/ ./internal/contest/ -count=1` → PASS.
Caller audit: `grep -rn "AttemptRecovery(" internal/ --include="*.go"` —
expect the definition + doc mentions in `skills.go`, the comment at
`internal/state/position/position.go:223`, the two production callers, and
the new tests; nothing else.

```bash
git add internal/characters/ internal/hooks/ internal/combat/contest_site_guard_test.go internal/contest/contest.go
git commit -m "feat(u10): free prone recovery contests whoever holds you down; the stand command stays the paid exit"
```

---

### Task 9: The Done-when guard (HEAD blob; Go files only)

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
// copy is skip-worktree and deliberately stale on dev machines. The
// single-reader scan is restricted to Go files so documentation naming the
// knob (context.md) cannot trip it.
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
		"knockdown is an opposed contest, not a threshold roll")
	mustNotContain(readFile("internal/characters/skills.go"),
		"skills.go", "RollStat(50",
		"free recovery is contested or automatic, never a solo roll")
	mustNotContain(readFile("internal/actions/combat_throttle.go"),
		"combat_throttle.go", "ThrottleInterruptChance",
		"the throttle interrupt goes through the concentration seam")

	// ConcentrationFloor single-reader rule (Done-when 2), Go files only.
	out, err := exec.Command("git", "-C", repoRoot, "grep", "-l",
		"ConcentrationFloor", "--", "internal/*.go", "internal/**/*.go",
		"modules/**/*.go").Output()
	if err != nil && len(out) == 0 {
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
		"threshold knobs became intended-rate factors")
	for _, key := range []string{"ConcentrationFloor:", "ConcentrationDamageThresholdPct:",
		"BashKnockdownFactor:", "TripKnockdownFactor:", "KickKnockdownFactor:"} {
		if !strings.Contains(yaml, key) {
			t.Errorf("HEAD config.yaml missing shipped knob %s", key)
		}
	}
}
```

(Verify the two-pattern pathspec actually matches nested Go files on this
git version — `git grep -l ConcentrationFloor -- 'internal/**/*.go'` from
a shell first; adjust to `-- internal modules` plus an in-loop
`strings.HasSuffix(f, ".go")` filter if the pathspec misbehaves. The
in-loop suffix filter is the sturdier form; prefer it.)

- [ ] **Step 2: Run + commit**

Run: `GOTMPDIR=C:/gotmp go test ./internal/combat/ -run "TestU10DoneWhen|TestKnockdownFactor|TestRunConcentrationContest" -count=1` → PASS.

```bash
git add internal/combat/u10_done_when_test.go
git commit -m "test(u10): the Done-when list as a guard — dead paths stay dead, one floor reader"
```

---

### Task 10: Docs (context.md, helpfiles, roadmap, backlog, patch notes)

**Files:**
- Modify context.md: `internal/combat/`, `internal/characters/`,
  `internal/hooks/`, `internal/actions/` (:184 documents the deleted
  throttle knob), `internal/configs/` (if it lists knobs),
  `internal/state/position/`, `internal/state/control/`,
  `internal/state/activity/` (all reference `CalcConcentrationChance`)
- Modify helpfiles (dogmud template layer, no numbers, no em dashes):
  - `grapple.template:116`, `cast.template:16`, `spell.template:18` —
    Willpower-only claims → Willpower AND spellcasting training contest
    the hold
  - `throttle.template:27` — "high chance to shatter their concentration"
    → the victim's training contests the grip
  - `prone.template:40,53-54` — free recovery is a contest against
    whoever is on you; with nobody attacking you, you simply stand
  - `stand.template:20,29` — reframe as the paid certain exit versus the
    free contested recovery; no percent claims
- Modify: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`:
  - U10 section: shipped shape (lattice ×10 with the corrected harsher
    table, throttle through the seam at the concentration floor,
    knockdown as a NAMED rebalance to intended rates with the true old
    rates recorded, success-only progression, stand-command framing)
  - unowned-sites table (~:115): throttle row → claimed by U10;
    surprise-attack row (~:121) → **U10d**; the U4-era note (~:173) and
    the follow-up note (~:526) → U10d
  - **add the U10d row** (surprise-attack redesign; brainstorm → spec →
    plan; sequenced with U10b/U10c before U12)
  - U10b row: U10's sites ship the success half of its convention; the
    crit/critical-failure bonus layer for them still lands with U10b
- Modify: `docs/roadmaps/CURRENT_BACKLOG.md:26-34` (U10 scope + U10d in
  the sequencing line)
- Modify: `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md` — add the U10 rows:
  hold a cast under fire at journeyman and adept scores; prone-cast into a
  grapple; get knocked down across a skill gap (trip should feel
  resistible now, kick knockdowns should exist); pinned recovery vs the
  paid stand; a fanged mob throttling a casting character
- Modify: `docs/PATCH_NOTES.md`
- Modify: `CLAUDE.md` "Dice & Rolling System": one line — concentration
  contests (damage, position, throttle) run through
  `combat.RunConcentrationContest`, floored by `ConcentrationFloor`, not
  `ContestFloor`

- [ ] **Step 1: Update each file; verify every named symbol exists**

Patch notes draft:

```markdown
## 2026-08-21: Holding a spell is a skill

Keeping a spell together while someone hits you is now a real contest, and
your spellcasting training is half of it. A practiced caster holds through
pain that would shatter a novice. Small scrapes no longer threaten your
casting at all. Being wrestled to the ground is still the surest way to
stop a caster, and a beast at your throat now tests your training against
its grip instead of rolling plain luck.

Knocking someone down now has to beat them. A trip or bash measures your
skill against their footwork, so clumsy attackers cannot floor a master by
luck alone, and some moves that almost never took anyone down now can.
Getting back up for free is a contest against whoever is standing over
you. Standing by command still works as it always has: you spend the
effort, you get up. If nobody is on you, you simply rise.
```

- [ ] **Step 2: Commit**

```bash
git add internal/ _datafiles/world/dogmud/templates/help/ docs/ CLAUDE.md
git commit -m "docs(u10): context.md + helpfile sweep, roadmap (throttle claimed, U10d filed), cribsheet rows, patch notes"
```

---

### Task 11: The adversarial playtest gate

**Files:**
- Create: `tools/playtest/goals/2026-08-XX-u10-disruption.yaml` (date of
  the run; `ephemeral:` block with `profile: mid` or `specialist-caster`,
  start room near a camp with casters and grapplers)

- [ ] **Step 1: Author the goals file**

Focus lanes: (1) cast under melee fire and quote every
concentration-related line — chip hits must produce NO break text, big
hits must resolve visibly; (2) get knocked prone mid-cast and confirm the
break narration; (3) fight a caster and watch trip/kick knockdowns — kick
knockdowns should now occur, trip should sometimes fail; (4) get knocked
down by a strong mob and compare the free recovery grind against the paid
`stand`; (5) find a fanged beast, cast at it, and observe throttle
interrupts and holds; (6) general adversarial pass. Follow the format of
`tools/playtest/goals/2026-08-21-bandit-caster-1on1.yaml`.

- [ ] **Step 2: Run it and disposition findings**

`/playtest local --checkout <abs> feature-tester <goals>.yaml` per the
CLAUDE.md SOP (wipe instance saves is handled by the ephemeral env). Fix
what it finds, re-run if needed, extract findings to memory (reports are
gitignored), and bank the goals file:

```bash
git add tools/playtest/goals/
git commit -m "docs(u10): bank the disruption playtest goals file"
```

---

### Task 12: Gates and PR

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
become real contests on the arc's seam. Spec rev 3 has the full design and
the owner decision log (docs/superpowers/specs/2026-08-21-u10-disruption-
model-design.md section 7). Named behaviour changes: chip damage below a
tenth of the pool no longer threatens casts; prone/grapple holds land
harsher than the old curve (lattice x10, re-ratified over corrected
numbers); throttle interrupts are opposed through the concentration seam
at its 2% floor (was a flat 75% coin that also ignored NoDamageInterrupt);
KNOCKDOWN IS A NAMED REBALANCE — the old threshold knobs claimed 50/60/35
but delivered 50/~91/~2.3, and the contest ships the intended rates, so
trip stops being near-certain and kick knockdowns start existing; free
prone recovery contests whoever is on you while the stand command stays
the paid uncontested exit; progression at the new sites is success-only
(the U10b convention's success half). Surprise attack is split out as
U10d. Adversarial playtest ran as the closing gate.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
gh pr checks <n> --repo pruuk/DOGMud --watch
```

Then verify annotations on the validate runs
(`gh api repos/pruuk/DOGMud/check-runs/<id>/annotations --jq length` → 0).
Do not merge without the owner's word; the arc no-deploy policy holds
regardless.
