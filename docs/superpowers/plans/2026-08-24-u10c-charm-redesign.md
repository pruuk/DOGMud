# U10c Charm Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make charm a companion with a clock — it joins the unified contest seam, its duration is bought with the margin of victory, and when the bond ends the creature turns on the caster.

**Architecture:** Charm stops calling `combat.RunContest` with hand-built scores and routes through `ResolveChannelAttack(ChannelSocial)`, answered by defy. The seam gains one additive field so an attack-win can report its opposed margin. The ~60-line dead resist ladder is deleted and its break-free branch is promoted to the unconditional expiry outcome.

**Tech Stack:** Go, `internal/combat` (contest seam), `internal/hooks` (spell + round tick), `internal/characters` (companion state), `internal/configs` (balance knobs), YAML data files and ANSI help templates.

**Spec:** `docs/superpowers/specs/2026-08-24-u10c-charm-redesign-design.md` — read sections 3 (decisions and their rationale) and 7 (traps) before starting.

---

## Read this before Task 1

Four facts that are load-bearing and non-obvious. Each was verified in source; do not re-derive them.

1. **`AttackSide.score()` computes the score for you.**
   ```go
   return (float64(s.Stat) + float64(s.SkillRank)*float64(configs.GetBalanceConfig().SkillWeight)) * m
   ```
   The uniform skill weight is read *inside* the seam. You supply `Stat`, `Skill`, `SkillRank` and `Mult`. **The old ×25 becomes impossible to express** — that is the point, not an accident.

2. **The seam does not surface the opposed margin on an attack win.** `NormalizedDefenceMargin` is assigned *below* `if res.Success { return out }`, so it is zero exactly when the attacker won. Task 1 fixes this.

3. **The margin sign is a documented footgun.** `contest.Result.Margin` is ATTACK-positive; `bestDefenseResult.margin` in `internal/combat` is DEFENCE-positive. The contest package's own docs say mixing them "compiles cleanly and silently puts crit on the losing side."

4. **The conviction reservation is derived, not held.** `GetPoolReservation` sums each live companion's `ConvictionReserve` (`internal/characters/reservation.go:252`). `RemoveCompanion` therefore releases it automatically — **do not add an explicit release step.**

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `internal/combat/defence_multiplier.go` | The unified contest seam | Modify — add `AttackerNormalizedMargin`, populate on the win path |
| `internal/combat/channel_attacker_margin_test.go` | Guards the new field's sign, zeroing and scaling | Create |
| `internal/configs/config.balance.go` | Balance field declarations | Modify — two new `ConfigInt` fields |
| `internal/configs/config.balance.mobs.go` | Companion/charm defaulting | Modify — defaults beside `CompanionReserveDefault` |
| `internal/hooks/charm_duration.go` | Pure margin→rounds function | Create |
| `internal/hooks/charm_duration_test.go` | Monotonicity, clamping, floored-takes-Min | Create |
| `internal/hooks/charm_spell.go` | The cast: contest, duration, companion registration | Modify — route through the seam |
| `internal/hooks/charm_spell_test.go` | Cast-path behaviour | Create |
| `internal/hooks/NewRound_MobRoundTick.go` | Round tick: charm expiry | Modify — delete the ladder, promote the grudge |
| `internal/hooks/charm_expiry_test.go` | Grudge, absent caster, reservation release | Create |
| `internal/characters/companions.go` | `CompanionInfo` | Modify — delete `CharmDuration`, `CharmRerolls` |
| `_datafiles/world/dogmud/spells/charm.yaml` | Spell description | Modify — §10.1 |
| `_datafiles/world/dogmud/templates/help/charm.template` | Player help | Modify — §10.2 |
| `_datafiles/config.yaml` | Shipped knob values | Modify — **via the `git show HEAD:` blob** |

---

### Task 1: Surface the attack-win margin on the seam

**Files:**
- Modify: `internal/combat/defence_multiplier.go`
- Test: `internal/combat/channel_attacker_margin_test.go` (create)

This is an additive change to shared combat code. It must not alter any existing field's behaviour.

- [ ] **Step 1: Write the failing test**

```go
package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
)

// AttackerNormalizedMargin is the attack-positive twin of
// NormalizedDefenceMargin. The two are populated on OPPOSITE paths and neither
// is a substitute for the other: mixing them compiles cleanly and inverts the
// result, which is why contest.Result.Margin carries a warning in its own docs.
func TestAttackerNormalizedMargin_PopulatedOnlyOnAttackWin(t *testing.T) {
	attacker, defender := newContestPair(t)

	// Attack wins decisively.
	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			return contest.Result{
				Success:     true,
				Contested:   true,
				Margin:      30,
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)

	want := 30.0 / (10.0 * math.Sqrt2)
	if math.Abs(out.AttackerNormalizedMargin-want) > 1e-9 {
		t.Errorf("AttackerNormalizedMargin = %v, want %v", out.AttackerNormalizedMargin, want)
	}
	if out.AttackerNormalizedMargin <= 0 {
		t.Error("a decisive ATTACK win must produce a POSITIVE margin; the sign is inverted")
	}
	if out.NormalizedDefenceMargin != 0 {
		t.Errorf("NormalizedDefenceMargin = %v, want 0 on an attack win", out.NormalizedDefenceMargin)
	}
}

func TestAttackerNormalizedMargin_ZeroWhenDefenceWon(t *testing.T) {
	attacker, defender := newContestPair(t)

	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			return contest.Result{
				Success:     false,
				Contested:   true,
				Margin:      -30,
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)

	if out.AttackerNormalizedMargin != 0 {
		t.Errorf("AttackerNormalizedMargin = %v, want 0 when the defence won", out.AttackerNormalizedMargin)
	}
}

// A floored outcome stamps a +-1 SENTINEL margin, not a roll. Reporting it
// would let a mercy-granted win read as dominance.
func TestAttackerNormalizedMargin_ZeroWhenFloored(t *testing.T) {
	attacker, defender := newContestPair(t)

	restore := SetChannelAttackContestRunnerForTest(
		func(atk float64, entries []contest.Entry) contest.Result {
			return contest.Result{
				Success:     true,
				Contested:   true,
				Floored:     true,
				Margin:      1,
				DefenseRoll: dice.RollResult{StdDev: 10},
			}
		})
	defer restore()

	out := ResolveChannelAttack(ChannelSocial, AttackSide{Stat: 100}, attacker, defender)

	if out.AttackerNormalizedMargin != 0 {
		t.Errorf("AttackerNormalizedMargin = %v, want 0 on a floored win", out.AttackerNormalizedMargin)
	}
}
```

**Before writing this, find the existing helper that builds a contest pair.** Run:

```bash
grep -rn "func newContestPair\|func newChannelPair\|attacker, defender :=" internal/combat/*_test.go | head -5
```

If no such helper exists, write one in the new file that returns two `*characters.Character` with `Validate()` called, mirroring whatever the nearest existing channel test does.

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/combat/ -run TestAttackerNormalizedMargin -v
```

Expected: FAIL — `out.AttackerNormalizedMargin undefined`.

- [ ] **Step 3: Add the field**

In `ChannelDefenceResult`, directly beneath `NormalizedDefenceMargin`:

```go
	// AttackerNormalizedMargin is the ATTACK-POSITIVE opposed margin, populated
	// ONLY when the attacker won. NormalizedDefenceMargin is its
	// defence-positive counterpart and is populated only when the defence won,
	// so neither is a substitute for the other and neither is meaningful on the
	// other's path. See contest.Result.Margin's own warning: mixing the two
	// compiles cleanly and silently puts the outcome on the losing side.
	//
	// Zero on a floored outcome — the margin is then a +-1 sentinel rather than
	// a roll, and a mercy-granted win must not read as dominance.
	//
	// Added by U10c so charm can buy its duration with the margin of victory.
	// Nothing else consumes it yet; it is a general gap rather than a
	// charm-specific one, since any effect scaled by how decisively the attack
	// won hits the same wall.
	AttackerNormalizedMargin float64
```

- [ ] **Step 4: Populate it on the win path**

Locate the early return:

```bash
grep -n "if res.Success {" internal/combat/defence_multiplier.go
```

Replace that block so the margin is stamped before returning:

```go
	if res.Success {
		if !res.Floored && res.DefenseRoll.StdDev > 0 {
			out.AttackerNormalizedMargin = res.Margin / (res.DefenseRoll.StdDev * math.Sqrt2)
		}
		return out
	}
```

Note the guards mirror the existing `DefenseRollZScore` treatment three lines above: floored outcomes expose zero, and a zero StdDev (uncontested) is skipped.

- [ ] **Step 5: Run the tests to verify they pass**

```bash
go test ./internal/combat/ -run TestAttackerNormalizedMargin -v
```

Expected: PASS, all three.

- [ ] **Step 6: Prove nothing else changed**

```bash
go test ./internal/combat/ ./internal/hooks/ ./internal/actions/ -count=1
```

Expected: all `ok`. The field is additive; any failure here means the win-path return was altered rather than extended.

- [ ] **Step 7: Commit**

```bash
git add internal/combat/defence_multiplier.go internal/combat/channel_attacker_margin_test.go
git commit -m "feat(combat): surface the attack-positive margin on a channel win

ResolveChannelAttack assigned NormalizedDefenceMargin below the
'if res.Success { return out }' early return, so the opposed margin was zero
exactly when the ATTACKER won. Any effect that wants to scale with how
decisively an attack landed had nothing to read.

Additive: one field, populated on the win path with the same guards the
neighbouring DefenseRollZScore already uses -- zero when floored, because the
margin is then a sentinel rather than a roll, and skipped on a zero StdDev.

U10c consumes it for charm's duration. The gap is general, not charm-specific."
```

---

### Task 2: The duration knobs

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.mobs.go`
- Modify: `_datafiles/config.yaml` — **via the `git show HEAD:` blob, see Step 4**

- [ ] **Step 1: Declare the fields**

In `internal/configs/config.balance.go`, beside `CompanionReserveDefault`:

```go
	CharmDurationMinRounds ConfigInt `yaml:"CharmDurationMinRounds"` // Rounds a barely-won charm holds, and what a floored win takes (default 30)
	CharmDurationMaxRounds ConfigInt `yaml:"CharmDurationMaxRounds"` // Rounds a charm won at or beyond the crit bar holds (default 450)
```

- [ ] **Step 2: Default them**

In `internal/configs/config.balance.mobs.go`, immediately after the
`CompanionReserveDefault` block:

```go
	if b.CharmDurationMinRounds < 1 {
		// At RoundSeconds 4 this is ~3.4 minutes. It is what a scraped win buys,
		// and what a FLOORED win buys, since a mercy-granted success is by
		// definition not a dominant one.
		b.CharmDurationMinRounds = 30
	}
	if b.CharmDurationMaxRounds < 1 {
		// ~30 minutes. Chosen so a strong (1 sigma) win lands near 240 rounds,
		// reproducing the dead resist ladder's own 50 + Cha/2 + manifestation*3
		// (~235 rounds for a veteran) -- the only prior art for what a charm
		// duration should be.
		b.CharmDurationMaxRounds = 450
	}
```

- [ ] **Step 3: Verify the defaults load**

```bash
go test ./internal/configs/ -count=1
```

Expected: `ok`.

- [ ] **Step 4: Add the shipped values to `config.yaml` from the committed blob**

`_datafiles/config.yaml` has `skip-worktree` set and the working copy carries dev overrides (HttpPort, LogLevel, an uncommitted `Playtest:` block). Editing it on disk and staging it would ship those. Build the change from the committed blob instead:

```bash
git show HEAD:_datafiles/config.yaml > /tmp/cfg.yaml
# insert the two keys next to CompanionReserveDefault (or the companion block)
#   CharmDurationMinRounds: 30
#   CharmDurationMaxRounds: 450
python -c "import yaml;yaml.safe_load(open(r'<windows path to /tmp/cfg.yaml>',encoding='utf-8'));print('parses')"
H=$(git hash-object -w /tmp/cfg.yaml)
git update-index --cacheinfo 100644,$H,_datafiles/config.yaml
git diff --cached _datafiles/config.yaml   # MUST show only the two added lines
git update-index --skip-worktree _datafiles/config.yaml   # cacheinfo CLEARS this flag
git ls-files -v _datafiles/config.yaml     # MUST print "S"
```

The `skip-worktree` restore is not optional — `git update-index --cacheinfo` clears the bit, and leaving it clear makes every later branch switch fail on this file.

- [ ] **Step 5: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.mobs.go
git commit -m "feat(balance): charm duration knobs

Min 30 rounds (~3.4 min) is what a scraped or floored win buys. Max 450 (~30
min) is tuned so a 1-sigma win lands near 240 rounds, reproducing the dead
resist ladder's own formula -- the only prior art for a charm duration.

config.yaml built from the git show HEAD: blob; the working copy carries dev
overrides that must not ship."
```

---

### Task 3: The duration function

**Files:**
- Create: `internal/hooks/charm_duration.go`
- Test: `internal/hooks/charm_duration_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func charmDurationTestConfig(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.CharmDurationMinRounds = 30
	cfg.Balance.CharmDurationMaxRounds = 450
	configs.SetConfigForTest(t, cfg)
}

// A bigger margin must NEVER buy a shorter bond. If the sign is inverted this
// test is the one that catches it, so assert the direction explicitly rather
// than only asserting the endpoints.
func TestCharmDurationFor_IsMonotonicInMargin(t *testing.T) {
	charmDurationTestConfig(t)

	prev := 0
	for _, m := range []float64{0, 0.1, 0.25, 0.5, 1.0, 1.5, 1.9} {
		got := charmDurationFor(m)
		if got < prev {
			t.Errorf("margin %v gave %d rounds, less than the %d a smaller margin gave", m, got, prev)
		}
		prev = got
	}
}

func TestCharmDurationFor_ClampsBothEnds(t *testing.T) {
	charmDurationTestConfig(t)

	if got := charmDurationFor(0); got != 30 {
		t.Errorf("margin 0 = %d rounds, want the 30 minimum", got)
	}
	// A floored win reports margin 0 and must take exactly Min.
	if got := charmDurationFor(-5); got != 30 {
		t.Errorf("negative margin = %d rounds, want the 30 minimum", got)
	}
	if got := charmDurationFor(2.0); got != 450 {
		t.Errorf("margin at the crit bar = %d rounds, want the 450 maximum", got)
	}
	if got := charmDurationFor(50); got != 450 {
		t.Errorf("absurd margin = %d rounds, want the 450 maximum", got)
	}
}

// The shipped tuning must reproduce the dead ladder's ~235 rounds at a strong
// win. That anchor is why Max is 450 and not something rounder.
func TestCharmDurationFor_StrongWinMatchesThePriorArt(t *testing.T) {
	charmDurationTestConfig(t)

	if got := charmDurationFor(1.0); got != 240 {
		t.Errorf("a 1-sigma win = %d rounds, want 240 (the ladder's ~235 anchor)", got)
	}
}

// CharmPermanent is -1 and means never expires. charmDurationFor must never
// return it or 0, either of which would make the bond permanent or instant.
func TestCharmDurationFor_NeverReturnsSentinelOrZero(t *testing.T) {
	charmDurationTestConfig(t)

	for _, m := range []float64{-100, -1, 0, 0.001, 1, 100} {
		if got := charmDurationFor(m); got < 1 {
			t.Errorf("margin %v returned %d; must always be >= 1 round", m, got)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/hooks/ -run TestCharmDurationFor -v
```

Expected: FAIL — `undefined: charmDurationFor`.

- [ ] **Step 3: Write the implementation**

```go
package hooks

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// charmCritBar is the normalized margin at which a charm buys its full
// duration. It is the crit bar at parity, so "as decisive as a critical
// success" and "the longest possible hold" are the same threshold.
const charmCritBar = 2.0

// charmDurationFor converts the ATTACK-POSITIVE normalized margin of the
// winning contest into the rounds a charm holds.
//
//	duration = Min + (Max - Min) * clamp(margin / 2.0, 0, 1)
//
// The player is never told this number, and never told how long is left. That
// uncertainty is the mechanic (spec 3.3): a bond you cannot plan around is the
// whole risk of charming something dangerous.
//
// A floored win arrives here as margin 0 and therefore takes Min, which is
// correct -- a mercy-granted success is not a dominant one.
func charmDurationFor(normalizedMargin float64) int {
	bal := configs.GetBalanceConfig()

	min := int(bal.CharmDurationMinRounds)
	max := int(bal.CharmDurationMaxRounds)
	if min < 1 {
		min = 1
	}
	if max < min {
		max = min
	}

	ratio := normalizedMargin / charmCritBar
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	return min + int(math.Round(float64(max-min)*ratio))
}
```

- [ ] **Step 4: Run to verify it passes**

```bash
go test ./internal/hooks/ -run TestCharmDurationFor -v
```

Expected: PASS, all four.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/charm_duration.go internal/hooks/charm_duration_test.go
git commit -m "feat(charm): duration is bought with the margin of victory

duration = Min + (Max-Min) * clamp(margin/2.0, 0, 1), where 2.0 is the crit bar
at parity, so 'as decisive as a crit' and 'the longest hold' coincide.

A floored win arrives as margin 0 and takes Min, which is right: a
mercy-granted success is not a dominant one.

The monotonicity test exists to catch an inverted margin sign, which the
contest package warns compiles cleanly and silently inverts outcomes."
```

---

### Task 4: Route the cast through the seam

**Files:**
- Modify: `internal/hooks/charm_spell.go`
- Test: `internal/hooks/charm_spell_test.go` (create)

- [ ] **Step 1: Read the current cast path**

```bash
sed -n '1,120p' internal/hooks/charm_spell.go
```

Note what must be **preserved**: the `CharmImmune` gate, the companion-count gate, `WouldBreachReservationCap`, the reservation calculation, `EndAggro`, `TrackCharmed`, and the `CompanionInfo` registration.

- [ ] **Step 2: Write the failing test**

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// The attack score must come from the seam, which reads SkillWeight from
// config. Before U10c charm multiplied manifestation by a hardcoded 25, which
// is how it survived the U6 flip.
func TestCharmAttackSide_UsesUniformSkillWeight(t *testing.T) {
	charmDurationTestConfig(t)

	side := charmAttackSide(charmTestCaster(t), false, false)

	if side.Skill != skills.Manifestation {
		t.Errorf("Skill = %v, want manifestation", side.Skill)
	}
	if side.Mult != 1.0 && side.Mult != 0 {
		t.Errorf("Mult = %v, want unset/1.0 outside combat", side.Mult)
	}
	// The seam multiplies SkillRank by SkillWeight itself. A charm-side
	// pre-multiplication would show up as an inflated SkillRank.
	if side.SkillRank != charmTestCaster(t).GetSkillLevel(skills.Manifestation) {
		t.Errorf("SkillRank = %d, want the raw skill level with no pre-multiplication", side.SkillRank)
	}
}

func TestCharmAttackSide_InCombatPenaltiesPreserved(t *testing.T) {
	charmDurationTestConfig(t)
	c := charmTestCaster(t)

	if got := charmAttackSide(c, true, true).Mult; got != 0.75 {
		t.Errorf("Mult vs a creature fighting YOU = %v, want 0.75", got)
	}
	if got := charmAttackSide(c, true, false).Mult; got != 0.85 {
		t.Errorf("Mult vs a creature fighting someone else = %v, want 0.85", got)
	}
}

// A fumbled attack aborts even when the roll won -- the seam documents this and
// every other channel honours it.
func TestCharmOutcome_FumbleAbortsEvenOnAWinningRoll(t *testing.T) {
	out := combat.ChannelDefenceResult{Defended: false, AttackerFumble: true}
	if charmSucceeded(out) {
		t.Error("a fumbled charm must fail even though the defence did not win")
	}
}

func TestCharmOutcome_SuccessIsAnUndefendedNonFumble(t *testing.T) {
	if !charmSucceeded(combat.ChannelDefenceResult{Defended: false}) {
		t.Error("an undefended, unfumbled charm must succeed")
	}
	if charmSucceeded(combat.ChannelDefenceResult{Defended: true}) {
		t.Error("a defended charm must fail")
	}
}

var _ = contest.Result{}
var _ = dice.RollResult{}
```

Write `charmTestCaster(t)` in the same file: a `*characters.Character` with `Validate()` called, a known Charisma, and a known manifestation level.

- [ ] **Step 3: Run to verify it fails**

```bash
go test ./internal/hooks/ -run "TestCharmAttackSide|TestCharmOutcome" -v
```

Expected: FAIL — `undefined: charmAttackSide`, `undefined: charmSucceeded`.

- [ ] **Step 4: Extract the two helpers**

Add to `internal/hooks/charm_spell.go`:

```go
// charmAttackSide builds the attacker's half of the ONE contest.
//
// The score itself is the seam's job: AttackSide.score() computes
// (Stat + SkillRank*SkillWeight) * Mult, reading SkillWeight from config. That
// is why this returns components rather than a number, and why charm can no
// longer express the hardcoded x25 it carried through the U6 flip.
//
// Mult carries the in-combat penalty, which is preserved unchanged: a creature
// already fighting is harder to reach, and hardest of all when it is fighting
// YOU.
func charmAttackSide(ch *characters.Character, targetInCombat, targetFightingCaster bool) combat.AttackSide {
	mult := 1.0
	if targetInCombat {
		mult = 0.85
		if targetFightingCaster {
			mult = 0.75
		}
	}
	return combat.AttackSide{
		Stat:      ch.Stats.Charisma.ValueAdj,
		StatName:  "charisma",
		Skill:     skills.Manifestation,
		SkillRank: ch.GetSkillLevel(skills.Manifestation),
		Mult:      mult,
	}
}

// charmSucceeded reads the seam's verdict.
//
// Fumble is resolved BEFORE the win: the seam documents that a fumbled attack
// aborts even when the roll won, uniformly across channels.
func charmSucceeded(out combat.ChannelDefenceResult) bool {
	if out.AttackerFumble {
		return false
	}
	return !out.Defended
}
```

- [ ] **Step 5: Run to verify the helpers pass**

```bash
go test ./internal/hooks/ -run "TestCharmAttackSide|TestCharmOutcome" -v
```

Expected: PASS.

- [ ] **Step 6: Replace the hand-rolled contest**

Delete the attack-score, defence-score, aggro-penalty and `RunContest` blocks (spec 2.1, 2.7 — sections 3, 4, 5 and 6 of the current function) and replace with:

```go
	// ── 3. One contest, through the seam ───────────────────────────────
	// Charm was the one attack channel the U6b flip missed: it built its own
	// attack and defence scores and called RunContest directly. It now resolves
	// like every other channel. Defy answers it -- charm is an act of social
	// domination whose attack side is already Charisma.
	//
	// The StatPoolTotal*0.10 defence term is GONE and is not to be restored.
	// There is deliberately no power gate on what may be charmed: a charmed
	// elite trains and keeps the gear you give it, then turns on you at a
	// moment you cannot predict. The risk is the balance (spec 3.6).
	targetInCombat := targetMob.Character.IsInCombat()
	targetFightingCaster := targetInCombat &&
		targetMob.Character.CurrentCombatTarget().UserId == user.UserId

	out := combat.ResolveChannelAttack(
		combat.ChannelSocial,
		charmAttackSide(ch, targetInCombat, targetFightingCaster),
		ch,
		&targetMob.Character,
	)

	success := charmSucceeded(out)
```

Then replace the permanent charm with a real duration:

```go
	if success {
		// The bond has a clock, and the player is never told how long it is.
		rounds := charmDurationFor(out.AttackerNormalizedMargin)
		targetMob.Character.Charm(user.UserId, rounds, "")
```

**Delete the `99999`.** Do not substitute `characters.CharmPermanent`: that sentinel means "never expires" and would restore exactly the bug this slice removes.

- [ ] **Step 7: Record that the flat reservation is deliberate**

Spec 3.8. The reservation call is unchanged, but it now looks like an oversight
to anyone who reads it after this slice — a rat and an Elemental King reserving
identically is exactly the shape of a bug. Comment it at the call site so nobody
"fixes" it:

```go
	// ── 2. Reservation + budget gate ───────────────────────────────────
	// The reserve is FLAT and does not scale with what you charmed: a sewer rat
	// and an Elemental King tie up the same conviction. That is a deliberate
	// decision (U10c, spec 3.8), not an oversight, and not a leftover from the
	// pet-multiplier path.
	//
	// Charm is already a risky game -- the creature trains while it serves you,
	// keeps the gear you hand it, and turns on you at a moment you cannot
	// predict. Juggling several charmed NPCs is challenge enough on its own. A
	// power-scaled price would add bookkeeping without adding tension, so the
	// cost of charming something enormous is the DANGER, not the invoice.
	reserve := ch.CalcCompanionReserve(characters.CompanionReserveBase(0))
```

- [ ] **Step 8: Build and run the package**

```bash
gofmt -l internal/ && go build ./... && go test ./internal/hooks/ -count=1
```

Expected: no gofmt output, build clean, tests `ok`. Remove any import left unused by the deleted score maths (`math` is a likely casualty — let the compiler tell you).

- [ ] **Step 9: Commit**

```bash
git add internal/hooks/charm_spell.go internal/hooks/charm_spell_test.go
git commit -m "feat(charm): resolve through the unified seam, not a private contest

Charm was the one attack channel the U6b flip missed. It built its own attack
and defence scores and called RunContest directly, which is how a x25 skill
weight survived a project-wide move to a uniform 5.0.

It now goes through ResolveChannelAttack(ChannelSocial) and is answered by defy.
The weight is not corrected so much as made inexpressible: AttackSide.score()
reads SkillWeight from config, so a caller cannot supply its own.

The StatPoolTotal*0.10 defence term is deleted with no replacement. There is
deliberately no power gate -- a charmed elite trains, keeps the gear you hand
it, and turns on you unpredictably. The risk is the balance.

Duration now comes from the margin of victory. The 99999 magic number is gone."
```

---

### Task 5: Expiry becomes the grudge; the ladder dies

**Files:**
- Modify: `internal/hooks/NewRound_MobRoundTick.go`
- Test: `internal/hooks/charm_expiry_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package hooks

import (
	"testing"
)

// When the clock runs out the creature breaks free and turns on the caster.
// The companion entry goes, which is also what releases the conviction
// reservation -- GetPoolReservation SUMS live companions, so there is nothing
// separate to release.
func TestCharmExpiry_PresentCaster_ProducesTheGrudge(t *testing.T) {
	owner, mob, room := charmedPairInRoom(t)

	mob.Character.Charmed.RoundsRemaining = 0
	tickMobCharmState(mob)

	if mob.Character.IsCharmed() {
		t.Error("charm must be removed at expiry")
	}
	if owner.Character.GetCompanionByInstanceId(mob.InstanceId) != nil {
		t.Error("companion entry must be removed, which is what frees the reservation")
	}
	if mob.Character.Aggro == nil || mob.Character.Aggro.UserId != owner.UserId {
		t.Error("the creature must turn on the caster who was standing there")
	}
	_ = room
}

// Anti-grief: a bond that lapses while you are elsewhere must not create a
// hunter. Patrols and pathto mean a hostile elite could otherwise follow a
// player across zones indefinitely.
func TestCharmExpiry_AbsentCaster_NoGrudge(t *testing.T) {
	owner, mob, _ := charmedPairInRoom(t)
	owner.Character.RoomId = mob.Character.RoomId + 1 // caster is elsewhere

	mob.Character.Charmed.RoundsRemaining = 0
	tickMobCharmState(mob)

	if mob.Character.IsCharmed() {
		t.Error("charm must still be removed at expiry")
	}
	if mob.Character.Aggro != nil {
		t.Error("a creature whose bond lapsed while the caster was away must NOT hunt them")
	}
}

// A permanent charm (the -1 sentinel) must never expire. tickMobCharmDuration
// only decrements above zero, so -1 must stay -1 forever.
func TestCharmExpiry_PermanentSentinelNeverExpires(t *testing.T) {
	_, mob, _ := charmedPairInRoom(t)
	mob.Character.Charmed.RoundsRemaining = -1

	for i := 0; i < 50; i++ {
		tickMobCharmDuration(mob)
		tickMobCharmState(mob)
	}

	if !mob.Character.IsCharmed() {
		t.Error("a permanent charm expired; the -1 sentinel was decremented or compared wrong")
	}
}
```

Write `charmedPairInRoom(t)` in the same file: seed a room, a user and a charmed mob whose `CompanionInfo` is registered on the user with `SourceType: characters.CompanionCharmed`, both in the same room. Follow the seeding pattern in the nearest existing `internal/hooks` test.

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/hooks/ -run TestCharmExpiry -v
```

Expected: FAIL — no companion removal, no aggro.

- [ ] **Step 3: Delete the ladder**

Remove the entire `// Re-roll contested Charisma vs Willpower on CharmDuration tick.` block from `tickMobCharmState` — roughly lines 394-450, ending at the closing braces of the `if mob.Character.IsCharmed()` wrapper that follows the expiry-cleanup block.

That deletes: the periodic re-roll, the duplicated `Charisma + manifestation*25` scoring, the `CharmRerolls` effectiveness decay, and the "control is slipping" / "eyes flash with defiance" warnings.

- [ ] **Step 4: Promote the grudge into the expiry block**

Extend the existing expiry cleanup so it reads:

```go
	// Charm expiry cleanup, and the grudge.
	if mob.Character.IsCharmed() && mob.Character.Charmed.RoundsRemaining == 0 {
		cmd := mob.Character.Charmed.ExpiredCommand
		charmedUserId := mob.Character.RemoveCharm()

		if charmedUserId > 0 {
			if charmedUser := users.GetByUserId(charmedUserId); charmedUser != nil {
				charmedUser.Character.TrackCharmed(mob.InstanceId, false)

				// Removing the companion is ALSO what releases the conviction
				// reservation: GetPoolReservation sums live companions rather
				// than holding separate state. Do not add a release call.
				charmedUser.Character.RemoveCompanion(mob.InstanceId)

				// The grudge only bites if you are there to receive it. A bond
				// that lapses while the caster is elsewhere must not create a
				// creature that hunts them -- with patrols and pathto that is
				// griefing, not risk (spec 3.10).
				if charmedUser.Character.RoomId == mob.Character.RoomId {
					charmedUser.SendText(messaging.CategorySpellMental, fmt.Sprintf(
						`<ansi fg="red-bold">%s breaks free of your control!</ansi>`,
						mob.Character.Name))
					if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
						sendVisualRoomText(room, messaging.CategoryMobEmote, fmt.Sprintf(
							`<ansi fg="red">%s snarls and turns on %s!</ansi>`,
							mob.Character.Name, charmedUser.Character.Name),
							charmedUser.UserId)
					}
					mob.Character.SetAggro(charmedUser.UserId, 0, characters.DefaultAttack)
				}
			}
		}

		if cmd != `` {
			cmds := strings.Split(cmd, `;`)
			for _, cmd := range cmds {
				cmd = strings.TrimSpace(cmd)
				if len(cmd) > 0 {
					mob.Command(cmd)
				}
			}
		}
	}
```

The two messages are carried over verbatim from the deleted ladder — they were written for exactly this moment and never fired.

- [ ] **Step 5: Run to verify it passes**

```bash
go test ./internal/hooks/ -run TestCharmExpiry -v
```

Expected: PASS, all three.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/NewRound_MobRoundTick.go internal/hooks/charm_expiry_test.go
git commit -m "feat(charm): expiry is the grudge; delete the dead resist ladder

The ladder was gated on CompanionInfo.CharmDuration > 0, and the only
assignment to that field lived INSIDE the ladder, so it could never bootstrap.
Roughly 60 lines that never ran, including a second copy of the charm scoring
expression -- the same duplication class that let the progression dashboard
drift from production for months.

Its break-free branch was the one good thing in it, so that is promoted to the
unconditional expiry outcome, messages and all. They were written for this
moment and never fired.

The grudge only bites if the caster is present. A bond lapsing while they are
elsewhere must not create a creature that hunts them across zones."
```

---

### Task 6: The grudge dies with the player

**Files:**
- Modify: the player-death cleanup path — locate it first
- Test: add to `internal/hooks/charm_expiry_test.go`

- [ ] **Step 1: Find the death cleanup**

```bash
grep -rn "func.*Death_PlayerCleanup\|PlayerDeath" --include=*.go internal/hooks/ | head -5
grep -rn "func.*ClearAggroAgainst\|Aggro.UserId ==" --include=*.go internal/mobs/ internal/rooms/ | head -5
```

Prefer an existing helper that clears mob aggro against a user. If none exists, write one in `internal/mobs`.

- [ ] **Step 2: Write the failing test**

```go
// A player who dies must not be hunted afterwards by something they charmed.
func TestCharmGrudge_ClearedOnPlayerDeath(t *testing.T) {
	owner, mob, _ := charmedPairInRoom(t)

	mob.Character.Charmed.RoundsRemaining = 0
	tickMobCharmState(mob)
	if mob.Character.Aggro == nil {
		t.Fatal("precondition: the grudge should have been set")
	}

	clearCharmGrudgesAgainst(owner.UserId)

	if mob.Character.Aggro != nil && mob.Character.Aggro.UserId == owner.UserId {
		t.Error("a dead player must not still be hunted by a creature that broke free")
	}
}
```

- [ ] **Step 3: Run to verify it fails**

```bash
go test ./internal/hooks/ -run TestCharmGrudge -v
```

Expected: FAIL — `undefined: clearCharmGrudgesAgainst`.

- [ ] **Step 4: Implement and wire it**

```go
// clearCharmGrudgesAgainst drops aggro that charmed creatures took against a
// player. Called on player death.
//
// Without it, a creature that broke free keeps hunting: with patrol and pathto
// behaviour it can follow the player across zones indefinitely, which is
// griefing rather than the risk charm is meant to carry (spec 3.9).
func clearCharmGrudgesAgainst(userId int) {
	for _, mobInstanceId := range mobs.GetAllMobInstanceIds() {
		mob := mobs.GetInstance(mobInstanceId)
		if mob == nil {
			continue
		}
		if mob.Character.Aggro != nil && mob.Character.Aggro.UserId == userId {
			mob.Character.EndAggro()
		}
	}
}
```

Verify `mobs.GetAllMobInstanceIds` exists before using it:

```bash
grep -rn "func GetAllMobInstanceIds\|func GetAllMobInstances" internal/mobs/*.go | head -3
```

Call it from the player-death cleanup found in Step 1.

- [ ] **Step 5: Run to verify it passes, then commit**

```bash
go test ./internal/hooks/ -run TestCharmGrudge -v
git add internal/hooks/
git commit -m "fix(charm): the grudge dies with the player

A creature that broke free keeps its aggro, and with patrol and pathto
behaviour it can follow the player across zones indefinitely. That is griefing,
not the risk charm is meant to carry. Cleared on death."
```

---

### Task 7: Delete the dead companion fields

**Files:**
- Modify: `internal/characters/companions.go`

- [ ] **Step 1: Confirm nothing reads them**

```bash
grep -rn "CharmDuration\|CharmRerolls" --include=*.go internal/ modules/
```

Expected after Tasks 4-5: **no hits outside the struct declaration.** If anything else appears, stop and handle it before deleting.

- [ ] **Step 2: Check no save carries them**

```bash
grep -rl "charm_duration\|charm_rerolls" _datafiles/world/dogmud/users/ 2>/dev/null | head
```

Expected: no output. They are `omitempty` and were never assigned, so no save should contain them. If any do, confirm the user-save loader ignores unknown keys before proceeding.

- [ ] **Step 3: Delete the fields**

Remove `CharmDuration` and `CharmRerolls` from `CompanionInfo`.

- [ ] **Step 4: Verify and commit**

```bash
go build ./... && go test ./internal/characters/ ./internal/hooks/ -count=1
git add internal/characters/companions.go
git commit -m "chore(charm): delete CharmDuration and CharmRerolls

Both existed only for the resist ladder deleted in the previous commit.
CharmDuration was never assigned outside it and CharmRerolls counted re-rolls
that never happened. Both are omitempty and absent from every save."
```

---

### Task 8: Guard that charm cannot leave the seam again

**Files:**
- Test: `internal/hooks/charm_seam_guard_test.go` (create)

The arc already guards its seams this way; charm needs one because a private contest is exactly how it drifted for so long.

- [ ] **Step 1: Find the existing guard pattern to copy**

```bash
grep -rln "contest_floor_guard_test\|contest_site_guard_test\|readSource(t," internal/ | head -5
```

- [ ] **Step 2: Write the guard**

```go
package hooks

import (
	"strings"
	"testing"
)

// Charm was the one attack channel U6b's flip missed: it called RunContest with
// hand-built scores, which is how a x25 skill weight outlived a project-wide
// move to a uniform 5.0, and how the same expression came to exist in two
// files. It resolves through ResolveChannelAttack now. Keep it there.
func TestCharmDoesNotRunItsOwnContest(t *testing.T) {
	src := readSource(t, "charm_spell.go")

	for _, banned := range []string{"RunContest", "OpposedRollStat"} {
		if strings.Contains(src, banned) {
			t.Errorf("charm_spell.go calls %s directly; it must resolve through "+
				"combat.ResolveChannelAttack(ChannelSocial) so the uniform SkillWeight "+
				"and the defy defence set apply", banned)
		}
	}
	if !strings.Contains(src, "ResolveChannelAttack") {
		t.Error("charm_spell.go no longer routes through the unified seam")
	}
	if strings.Contains(src, "* 25") || strings.Contains(src, "*25") {
		t.Error("charm_spell.go appears to multiply a skill by 25 again")
	}
}
```

Reuse the existing `readSource` helper if `internal/hooks` has one; otherwise copy the implementation from wherever Step 1 found it.

- [ ] **Step 3: Run, then commit**

```bash
go test ./internal/hooks/ -run TestCharmDoesNotRunItsOwnContest -v
git add internal/hooks/charm_seam_guard_test.go
git commit -m "test(charm): guard that charm cannot leave the unified seam again"
```

---

### Task 9: Player-facing copy — REQUIRED FOR COMPLETION

Owner ruling: the slice is not done until both land. Read spec §10 before writing either.

**Files:**
- Modify: `_datafiles/world/dogmud/spells/charm.yaml`
- Modify: `_datafiles/world/dogmud/templates/help/charm.template`

- [ ] **Step 1: Rewrite the spell description**

The shipped text promises "Stronger creatures resist more fiercely", which Task 4 makes false. Convey instead that a strong-*willed* creature resists where a strong-*bodied* one may not, and that the bond does not last forever. Keep it to the existing block's length. **No numbers.**

- [ ] **Step 2: Rewrite the helpfile**

Read it first — it already describes this design and has been documenting behaviour that never ran. Four lines are now wrong and must change:

| Line | Fix |
|---|---|
| `Defense: Mental (opposed by target's willpower)` | Social channel, answered by defy. Still Willpower-based. |
| "…against the creature's willpower and **mental fortitude**" | The defending skill is **rhetoric**. |
| `Duration: Scales with charisma and manifestation skill` | Duration comes from **how decisively you won**. |
| "The stronger your charisma and the higher your manifestation skill, the longer the charm endures." | Replace with the margin framing. |

Preserve, because all remain true: charm cannot target players; immune creatures are beyond reach; creatures already in combat resist more strongly; the companion limit and `dismiss`.

**Never state the formula or the rounds remaining** — the uncertainty is the mechanic (spec 3.3).

Consider adding one line reflecting spec 3.6 honestly: a powerful creature is no harder to charm, but it is far more dangerous when the bond breaks.

- [ ] **Step 3: Check the mechanics**

```bash
# 80-char wrap on rendered prose
awk 'length($0)>80 {print FILENAME": "FNR": "length($0)}' _datafiles/world/dogmud/templates/help/charm.template
# balanced ansi tags
o=$(grep -o '<ansi ' _datafiles/world/dogmud/templates/help/charm.template | wc -l)
c=$(grep -o '</ansi>' _datafiles/world/dogmud/templates/help/charm.template | wc -l)
echo "open=$o close=$c"   # must match
python -c "import yaml;yaml.safe_load(open(r'_datafiles/world/dogmud/spells/charm.yaml',encoding='utf-8'));print('yaml ok')"
```

Long lines are acceptable ONLY where the excess is ansi markup, not rendered text.

- [ ] **Step 4: Verify it is SERVED, not just written**

A boot-clean check does not prove a template reaches the player. Boot an isolated worktree on non-default ports and fetch it over telnet:

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check
sed -i 's/^  TelnetPort: \[33333, 44444\]/  TelnetPort: [33397, 44497]/; s/^  LocalPort: 9999/  LocalPort: 9997/; s/^  HttpPort: 8090/  HttpPort: 8097/; s/^  AIPort: 55555/  AIPort: 55597/' _datafiles/config.yaml
go build -o boot-check.exe . && timeout 300 ./boot-check.exe > boot.log 2>&1 &
```

Wait for `Server Ready`, connect to `127.0.0.1:33397`, create a throwaway character (choose the veteran start), and run `help charm`. Confirm the rendered output no longer says the duration scales with charisma and manifestation skill. **Templates are parsed per request**, so a copy-in and re-fetch needs no rebuild.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/spells/charm.yaml _datafiles/world/dogmud/templates/help/charm.template
git commit -m "docs(charm): player copy matches the redesign

The helpfile has been describing this design all along -- 'your hold gradually
loosens over time', 'when the charm finally breaks, the creature reverts and
may turn hostile', even 'if the charm breaks while the creature is NEAR you'.
None of it ran. This slice makes it true, and corrects the four lines that
became wrong: the Mental defence label, mental fortitude, and both
duration-scales-with-your-stats claims.

The spell description no longer promises that stronger creatures resist more
fiercely, which the deleted size term made false.

Neither states the duration formula or the rounds remaining. Not knowing is the
mechanic."
```

---

### Task 10: Gates and ship

- [ ] **Step 1: Full local gates**

```bash
gofmt -l internal/ modules/          # must print nothing
go build ./...
go test ./internal/combat/ ./internal/hooks/ ./internal/characters/ ./internal/configs/ ./internal/actions/ ./internal/usercommands/ -count=1
GOTMPDIR=C:/gotmp golangci-lint run ./internal/hooks/... ./internal/combat/... ./internal/characters/...
```

Lint must show **no new** finding on a touched file; the repo carries pre-existing ones that are not this slice's to fix.

- [ ] **Step 2: Boot test**

Isolated detached worktree, non-default ports, fixed `boot-check.exe` path.

```bash
grep -c "Server Ready" boot.log                                          # want 1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
```

**Exit code 124 is the success case** — the timeout fired because the server stayed up. Never grep the bare word `panic`: `GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`.

- [ ] **Step 3: Patch notes**

Add a dated entry to `docs/PATCH_NOTES.md`. Player-facing framing, **no raw numbers**, no en or em dashes, 80-char wrap. The story: a charmed creature now serves for a time rather than forever, you are never told how long, and when the hold breaks it turns on you. Winning the contest more decisively buys a longer hold.

- [ ] **Step 4: Adversarial playtest gate**

Required — this changes player-facing behaviour and copy.

```text
/playtest local --checkout <abs> bug-finder <goals>.yaml
```

Write a goals file with an `ephemeral:` block that drives: casting charm and reading every line; keeping a charmed creature until it turns; `help charm` read as a player would; and `consider`/`status` on a charmed companion. Fix what it finds and re-run if needed.

- [ ] **Step 5: PR**

```bash
git push -u origin feature/u10c-charm-redesign
gh pr create --repo pruuk/DOGMud --base master --head feature/u10c-charm-redesign --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

**`--repo pruuk/DOGMud` is mandatory on every `gh` command** — this repo is a fork and `gh` defaults to the parent. Confirm each job actually ran and carries zero annotations before merging with `--merge` (not `--squash`).

- [ ] **Step 6: Update the roadmap**

Mark U10c ✅ in `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md` with the merge SHA, and record the two things this slice found that the roadmap did not know: charm never joined the U6b seam, and the seam did not surface an attack-win margin.

---

## Notes for whoever executes this

- **No migration is needed.** The owner confirms no veteran character uses charm.
- **U10b is still unshipped** and is a different slice — progression firing consistency. Do not fold any of it in here.
- If a step's grep finds something the plan did not predict, **stop and report** rather than improvising. Three prior slices in this arc were re-planned because reality differed from the roadmap, and every time the discovery was worth more than the guess.
