# U10d Surprise-Attack Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the uncontested surprise-attack burst with a contested surprise round on the existing combat seams, and add a single same-room ranged surprise shot.

**Architecture:** Delete `actions.SurpriseAttack` entirely. Being `Hidden` when combat joins marks round 1 as a *surprise round*, published as a round snapshot mirroring the existing sleeping snapshot. Every swing that round contests normally and crits on a won contest (`critOnWin`, distinct from `forceCrit`, which forces the win). The first landed swing additionally stacks `CritDamageMultiplier(skullduggery)`. Ranged gets the same treatment through `AttackSide.CritOnWin`, same-room only, and reveals the shooter.

**Tech Stack:** Go 1.x, GoMud engine. Packages touched: `internal/combat`, `internal/hooks`, `internal/actions`, `internal/usercommands`, `internal/mobcommands`, `internal/behaviortree`, `internal/state/combatphase`, `internal/configs`.

**Spec:** `docs/superpowers/specs/2026-08-25-u10d-surprise-attack-redesign-design.md`

---

## Before you start

Read the spec. It is the contract; this plan is the mechanics.

Three things that will bite you if you skip them:

1. **`forceCrit` and `critOnWin` are NOT the same.** `forceCrit` forces the WIN — the defender never rolls (sleeping victim). `critOnWin` respects the contest and only upgrades a win. Never merge them, never pass one where the other belongs.
2. **Crits already bypass mitigation completely.** `calcHitDamage` rolls off `sdp.rawDmgForCrit`, the unmitigated mean. Do not add any mitigation bypass for surprise; it would be strictly weaker and would double-count.
3. **Multipliers go on the MEAN, before `dice.RollStat`.** `RollStat` derives its spread from the mean it is handed (`stdDev = mean * RollSpread`). Scaling the rolled result instead stretches the variance and makes high-skill crits wildly swingy. Every crit path in this codebase is careful about this. Be careful too.

**Run tests with:** `go test ./internal/combat/... ./internal/hooks/... ./internal/actions/...`

---

## File Structure

**Modified:**

| File | Responsibility after this change |
|---|---|
| `internal/configs/config.balance.go` | declares `SurpriseOpeningStrikeMultiplier`; loses 5 `SurpriseAttack*Penalty` knobs |
| `internal/configs/config.balance.combat.go` | defaults/validates the new knob |
| `internal/configs/config.balance.misc.go` | loses the 5 penalty validators |
| `internal/combat/situational.go` | gains the surprise-round snapshot beside the sleeping one |
| `internal/combat/combat_helpers.go` | `combatContext.critOnWin`; `resolveDefenseOutcome*` critOnWin param; `calcHitDamage` opening-strike consumption |
| `internal/combat/combat.go` | four `Attack*Vs*` entry points thread `critOnWin`; `backstabCrit` removed |
| `internal/combat/crit_damage.go` | `CritOrMitigatedDamageScaled` |
| `internal/combat/skill_moves.go` | `SkillMoveParams.BonusCritMultiplier` |
| `internal/combat/defence_multiplier.go` | `AttackSide.CritOnWin` and its branches |
| `internal/hooks/NewRound_DoCombat.go` | publishes the surprise snapshot; consumes the round boundary |
| `internal/hooks/NewRound_DoCombat_unified.go` | skullduggery progression event |
| `internal/state/combatphase/combatphase.go` | STUBS comment removed |
| `internal/actions/combat_fire.go` | ranged surprise: gate, bonus, reveal, cooldown |
| `internal/actions/combat_attack.go` | `EngageAggroType` relocated here |
| `internal/usercommands/attack.go`, `internal/mobcommands/attack.go`, `internal/behaviortree/actions_combat.go` | call sites |

**Deleted:** `internal/actions/surprise_attack.go`

---

## Task 1: The config knob

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.combat.go`
- Modify: `_datafiles/config.yaml`
- Test: `internal/configs/config_balance_combat_test.go` (create)

**⚠ `_datafiles/config.yaml` has `skip-worktree` set.** Editing it locally is correct and necessary, but when you commit, build the committed content from `git show HEAD:_datafiles/config.yaml` plus your addition — **never from disk**, which legitimately carries dev-only overrides (HttpPort, LogLevel, an uncommitted `Playtest:` block).

- [ ] **Step 1: Write the failing test**

In `internal/configs/config_balance_combat_test.go`:

```go
func TestSurpriseOpeningStrikeMultiplier_DefaultsAndValidates(t *testing.T) {
	var b Balance
	b.SurpriseOpeningStrikeMultiplier = 0
	b.Validate()
	if got := float64(b.SurpriseOpeningStrikeMultiplier); got != 1.0 {
		t.Fatalf("zero must default to 1.0, got %v", got)
	}

	b.SurpriseOpeningStrikeMultiplier = -3
	b.Validate()
	if got := float64(b.SurpriseOpeningStrikeMultiplier); got != 1.0 {
		t.Fatalf("negative must reset to 1.0, got %v", got)
	}

	b.SurpriseOpeningStrikeMultiplier = 2.5
	b.Validate()
	if got := float64(b.SurpriseOpeningStrikeMultiplier); got != 2.5 {
		t.Fatalf("a legal value must survive validation, got %v", got)
	}
}
```

`Validate()` is the real entry point (`internal/configs/config.balance.go:936`); it calls the unexported `validateCombat()` internally, which is where your defaulting block goes.

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/configs/ -run TestSurpriseOpeningStrikeMultiplier -v`
Expected: compile failure, `b.SurpriseOpeningStrikeMultiplier undefined`.

- [ ] **Step 3: Declare the field**

In `internal/configs/config.balance.go`, beside the other crit knobs:

```go
	SurpriseOpeningStrikeMultiplier ConfigFloat `yaml:"SurpriseOpeningStrikeMultiplier"` // Extra multiplier on a surprise round's opening strike (default 1.0)
```

- [ ] **Step 4: Default and validate it**

In `internal/configs/config.balance.combat.go`, next to `CritDamageBase` / `CritDamagePerSkill`:

```go
	// SurpriseOpeningStrikeMultiplier tunes the ambush ALONE. The stacked
	// opening strike already multiplies two CritDamageMultiplier terms, and
	// those knobs are global -- retuning them to fix the ambush would move
	// every crit in the game. 0 or negative is nonsense, not "disabled":
	// disabling is 1.0.
	if b.SurpriseOpeningStrikeMultiplier <= 0 {
		b.SurpriseOpeningStrikeMultiplier = 1.0
	}
```

- [ ] **Step 5: Run the test**

Run: `go test ./internal/configs/ -run TestSurpriseOpeningStrikeMultiplier -v`
Expected: PASS

- [ ] **Step 6: Add the shipped value**

In `_datafiles/config.yaml`, beside `CritDamageBase` / `CritDamagePerSkill`:

```yaml
  # SurpriseOpeningStrikeMultiplier: extra multiplier on the opening strike of
  # a surprise round, applied on top of the stacked combat-skill and
  # skullduggery crit multipliers. 1.0 = no change. Exists so the ambush can be
  # retuned WITHOUT moving CritDamageBase / CritDamagePerSkill, which are global
  # and would move every crit in the game.
  SurpriseOpeningStrikeMultiplier: 1.0
```

- [ ] **Step 7: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.combat.go internal/configs/config_balance_combat_test.go
git commit -m "feat(u10d): add SurpriseOpeningStrikeMultiplier"
```

Commit `_datafiles/config.yaml` separately, built from the `git show HEAD:` blob (see the warning above).

---

## Task 2: `critOnWin` in the melee resolution core

`resolveDefenseOutcomeCore` already takes `forceCrit`. Add `critOnWin` beside it. `forceCrit` forces the win; `critOnWin` upgrades a win that already happened.

**Files:**
- Modify: `internal/combat/combat_helpers.go:897`, `:914`
- Test: `internal/combat/surprise_critonwin_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/combat/surprise_critonwin_test.go`:

Two fixtures already exist and you MUST reuse them rather than inventing new
ones: `defenceFixture(defenderStamina int) (*characters.Character, *characters.Character)`
in `internal/combat/defense_affordability_test.go:19`, and
`defenceWinBest(rawMargin, defStdDev float64) bestDefenseResult` in
`internal/combat/defence_multiplier_test.go:97`.

**The margin sign is DEFENCE-POSITIVE.** A *negative* margin is an attack win.
The established idiom, used verbatim in two existing tests, is
`defenceWinBest(-15*math.Sqrt2, 15)`. Its `hitRoll.ZScore` is 0, comfortably
below the 2.0 crit bar, so nothing but `critOnWin` can produce a crit. Get this
sign backwards and your tests silently exercise a defence win and fail in a
confusing way.

```go
package combat

import (
	"math"
	"testing"
)

// attackWinBest is a settled ATTACK win below the crit bar. Negative margin ==
// attack win (defenceWinBest's margin is DEFENCE-positive), and ZScore 0 is far
// under a 2.0 bar, so only critOnWin can make this crit.
func attackWinBest() bestDefenseResult { return defenceWinBest(-15*math.Sqrt2, 15) }

// critOnWin must upgrade a WON contest to a crit, and must NOT rescue a lost
// one. That distinction is the whole point: forceCrit forces the win,
// critOnWin respects the contest.
func TestCritOnWin_UpgradesWinButNeverRescuesALoss(t *testing.T) {
	src, tgt := defenceFixture(1000)

	t.Run("attack win becomes a crit", func(t *testing.T) {
		res := resolveDefenseOutcomeCore(&AttackResult{}, attackWinBest(), src, tgt, 2.0, false, false, true)
		if !res.hit || res.defended {
			t.Fatalf("precondition: the attack should have won cleanly")
		}
		if !res.crit {
			t.Fatalf("critOnWin must upgrade a won contest to a crit")
		}
	})

	t.Run("defence win stays a defence win", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2, 15) // positive == defence won
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, true)
		if res.crit {
			t.Fatalf("critOnWin must never turn a lost contest into a crit")
		}
	})

	t.Run("critOnWin false is unchanged behaviour", func(t *testing.T) {
		res := resolveDefenseOutcomeCore(&AttackResult{}, attackWinBest(), src, tgt, 2.0, false, false, false)
		if res.crit {
			t.Fatalf("an ordinary win must not crit merely from winning")
		}
	})

	t.Run("a FLOORED win must not crit", func(t *testing.T) {
		best := attackWinBest()
		best.floored = true
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, true)
		if res.crit {
			t.Fatalf("a floored outcome carries a sentinel margin and must never be promoted")
		}
	})
}
```

That last subtest is not optional. `crit_floor.go:122-130` states the rule
explicitly: *"A floored outcome carries the +-1 sentinel margin and represents an
outcome the contest did not actually produce. Promoting it would hand a decisive
result to the side that lost the roll."* A mercy-granted save must not become a
maximum-damage ambush crit.

Confirm `floored` is the actual field name on `bestDefenseResult` before writing
that subtest — read the struct.

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/combat/ -run TestCritOnWin -v`
Expected: compile failure — `too many arguments in call to resolveDefenseOutcomeCore`.

- [ ] **Step 3: Add the parameter**

`internal/combat/combat_helpers.go`, both functions:

```go
// critOnWin upgrades a WON contest to a crit. It does NOT decide the contest.
//
// Deliberately NOT forceCrit. forceCrit forces the WIN outright and the
// defender never gets to answer it (the sleeping-victim contract). critOnWin
// respects the contest in full: the defender rolls and may win, and on a
// defender win nothing is upgraded, because there is no hit to upgrade.
//
// The two are independent and may both be true -- an ambush against a sleeping
// target -- in which case forceCrit decides the outcome and critOnWin is
// redundant. That is fine; do not "simplify" it into one flag.
func resolveDefenseOutcome(result *AttackResult, best bestDefenseResult, sourceChar *characters.Character, targetChar *characters.Character, critThreshold float64, isThirdParty bool, forceCrit bool, critOnWin bool) hitResolution {
	res := resolveDefenseOutcomeCore(result, best, sourceChar, targetChar, critThreshold, isThirdParty, forceCrit, critOnWin)
	// ... rest unchanged
}

func resolveDefenseOutcomeCore(result *AttackResult, best bestDefenseResult, sourceChar *characters.Character, targetChar *characters.Character, critThreshold float64, isThirdParty bool, forceCrit bool, critOnWin bool) hitResolution {
```

- [ ] **Step 4: Apply it, after the existing crit determination**

Find where `res.crit` is finally settled in `resolveDefenseOutcomeCore` (after the fumble → crit → normal → floors ordering, near the `forceCrit` handling at ~`:962`). Add, as the **last** thing that can set `crit`:

```go
	// U10d: a surprise round crits on any CLEANLY won contest. Placed last so
	// it can only upgrade an outcome the ordinary ordering already settled.
	//
	// Every term is load-bearing:
	//   res.hit && !res.defended -- the documented idiom for "the attack won
	//     the contest" (see hitResolution.defended, combat_helpers.go:798-803).
	//     hit alone is TRUE for a deflected partial hit, which is a defence
	//     win and must not be promoted.
	//   !best.floored -- crit_floor.go:122 declares that a floored outcome
	//     carries a sentinel margin and must never be promoted. A mercy save
	//     must not become a maximum-damage ambush crit.
	//   !res.fumble -- a fumble is the attacker's own blunder and aborts even
	//     a winning roll (applyCritFloors returns early on it).
	if critOnWin && res.hit && !res.defended && !best.floored && !res.fumble {
		res.crit = true
	}
```

Do not drop any of the four guards to "simplify". Each one corresponds to a rule
this package states in a comment somewhere, and dropping one produces a bug that
only shows up as an occasional absurd damage number in play.

- [ ] **Step 5: Fix the existing call sites**

`internal/combat/combat.go:466` passes `ctx.forceCrit`. Add `, ctx.critOnWin` — the field lands in Task 3, so for now pass `false` and leave a `// Task 3` comment. Update every `_test.go` caller listed by:

```bash
grep -rn "resolveDefenseOutcomeCore(\|resolveDefenseOutcome(" internal/combat/
```

with a trailing `, false`.

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/combat/ -run TestCritOnWin -v` → PASS
Run: `go test ./internal/combat/...` → all green (the existing suite must be unaffected; `critOnWin=false` is the old behaviour).

- [ ] **Step 7: Commit**

```bash
git add internal/combat/combat_helpers.go internal/combat/combat.go internal/combat/surprise_critonwin_test.go
git commit -m "feat(u10d): critOnWin in the melee resolution core, distinct from forceCrit"
```

---

## Task 3: The surprise-round snapshot

Mirror the sleeping snapshot exactly. A snapshot, not a live `IsHidden()` read — `Hidden` breaks mid-round through several paths, and a live read would let the attacker's own first swing cancel the surprise for their second weapon, making multi-weapon behaviour depend on iteration order.

**Files:**
- Modify: `internal/combat/situational.go`
- Test: `internal/combat/surprise_snapshot_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

func TestSurpriseSnapshot_StableAcrossTheRound(t *testing.T) {
	round := util.GetRoundCount()
	attacker := newTestCharacterWithUserId(t, 7)

	PublishSurpriseSnapshot(round, map[int]bool{7: true}, map[int]bool{})

	if !SurpriseRoundActive(attacker) {
		t.Fatalf("a snapshotted attacker must be in a surprise round")
	}

	// The whole point: losing Hidden mid-round must NOT downgrade later swings.
	attacker.CancelBuffsWithFlag(buffs.Hidden)
	if !SurpriseRoundActive(attacker) {
		t.Fatalf("the snapshot must survive a mid-round Hidden break")
	}
}

func TestSurpriseSnapshot_StaleRoundSaysNothing(t *testing.T) {
	attacker := newTestCharacterWithUserId(t, 7)
	PublishSurpriseSnapshot(util.GetRoundCount()+999, map[int]bool{7: true}, map[int]bool{})
	if SurpriseRoundActive(attacker) {
		t.Fatalf("a snapshot from another round must not apply")
	}
}
```

Reuse whatever character-construction helper `internal/combat`'s tests already use instead of `newTestCharacterWithUserId` if one exists — check `situational_test.go` first.

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/combat/ -run TestSurpriseSnapshot -v`
Expected: `undefined: PublishSurpriseSnapshot`.

- [ ] **Step 3: Implement, beside the sleeping snapshot**

In `internal/combat/situational.go`:

```go
// surpriseSnapshot records who began this round in a surprise round.
//
// A snapshot rather than a live IsHidden() read, for the same reason the
// sleeping snapshot exists: Hidden breaks mid-round through several paths (the
// defender's CancelCombatBuffs, ForceVisible on retaliation, cancel-on-damage).
// A live read would let an attacker's own first swing cancel the surprise for
// their second weapon, making a multi-weapon ambush depend on iteration order.
var surpriseSnapshot struct {
	round          uint64
	userIds        map[int]bool
	mobInstanceIds map[int]bool
}

// PublishSurpriseSnapshot records the surprise-round set for one round. Called
// once at the top of DoCombat, before either combat pass runs.
func PublishSurpriseSnapshot(round uint64, userIds map[int]bool, mobInstanceIds map[int]bool) {
	surpriseSnapshot.round = round
	surpriseSnapshot.userIds = userIds
	surpriseSnapshot.mobInstanceIds = mobInstanceIds
}

// SurpriseRoundActive reports whether attacker began the current round in a
// surprise round. A stale snapshot (any other round) says no. Nil says no.
func SurpriseRoundActive(attacker *characters.Character) bool {
	if attacker == nil {
		return false
	}
	if surpriseSnapshot.round != util.GetRoundCount() {
		return false
	}
	if attacker.MobInstanceId > 0 {
		return surpriseSnapshot.mobInstanceIds[attacker.MobInstanceId]
	}
	if uid := attacker.GetUserId(); uid > 0 {
		return surpriseSnapshot.userIds[uid]
	}
	return false
}
```

Note the asymmetry with `SleepingForceCrit`, and keep it: that one also honours a **live** `Sleeping` flag so a command-driven attack catches a sleeper. Surprise deliberately does not, because the live flag is exactly what goes stale mid-round.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/combat/ -run TestSurpriseSnapshot -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/combat/situational.go internal/combat/surprise_snapshot_test.go
git commit -m "feat(u10d): surprise-round snapshot, mirroring the sleeping snapshot"
```

---

## Task 4: Thread `critOnWin` from the snapshot into melee

**Files:**
- Modify: `internal/combat/combat_helpers.go` (`combatContext`)
- Modify: `internal/combat/combat.go` (four entry points, `:37`, `:99`, `:143`, `:184`)
- Modify: `internal/hooks/NewRound_DoCombat.go`
- Test: `internal/combat/surprise_critonwin_test.go` (extend)

- [ ] **Step 1: Add the context field**

`internal/combat/combat_helpers.go`, in `combatContext` beside `forceCrit`:

```go
	// U10d: when true, EVERY landed swing this round crits, because the
	// attacker opened from stealth. Set from the DoCombat surprise snapshot.
	// Distinct from forceCrit: this does not decide the contest, it upgrades a
	// win. See resolveDefenseOutcomeCore.
	critOnWin bool
```

- [ ] **Step 2: Populate it in all four entry points**

In each of `AttackPlayerVsMob`, `AttackPlayerVsPlayer`, `AttackMobVsPlayer`, `AttackMobVsMob`, beside `forceCrit: forceCrit`:

```go
		critOnWin:    SurpriseRoundActive(sourceChar),
```

Use whatever the attacker's `*characters.Character` is named in each function — read each one; they differ (`user.Character`, `&mob.Character`).

- [ ] **Step 3: Use it at the resolution site**

`internal/combat/combat.go:466` — replace the `false` placeholder from Task 2:

```go
			res := resolveDefenseOutcome(&attackResult, best, sourceChar, targetChar, critThreshold, isThirdParty, ctx.forceCrit, ctx.critOnWin)
```

- [ ] **Step 4: Publish the snapshot at the top of `DoCombat`**

`internal/hooks/NewRound_DoCombat.go`, immediately after the existing `snapshotSleepingVictims()` publish:

```go
	surpriseUserIds, surpriseMobInstanceIds := snapshotSurpriseRounds()
	combat.PublishSurpriseSnapshot(evt.RoundNumber, surpriseUserIds, surpriseMobInstanceIds)
```

And add the collector beside `snapshotSleepingVictims`:

```go
// snapshotSurpriseRounds records which users and mob instances begin this round
// in a surprise round -- Engaged with the combat-phase SurpriseLeft flag still
// set. Published to internal/combat so every swing of the round reads one
// stable value, including channel attacks (a ranged surprise shot) resolving
// later in the same round.
//
// Chunk 3.3's docstring on snapshotSleepingVictims invited exactly this:
// "Future first-hit-crit triggers (surprise attack, backstab, etc.) can add
// parallel snapshot checks at this same site."
func snapshotSurpriseRounds() (userIds map[int]bool, mobInstanceIds map[int]bool) {
	userIds = map[int]bool{}
	mobInstanceIds = map[int]bool{}
	for _, uid := range users.GetOnlineUserIds() {
		if u := users.GetByUserId(uid); u != nil && u.Character.InSurpriseRound() {
			userIds[uid] = true
		}
	}
	for _, mobId := range mobs.GetAllMobInstanceIds() {
		if m := mobs.GetInstance(mobId); m != nil && m.Character.InSurpriseRound() {
			mobInstanceIds[mobId] = true
		}
	}
	return userIds, mobInstanceIds
}
```

- [ ] **Step 5: Add `Character.InSurpriseRound()`**

`internal/characters/` — read `combat_state_compat.go` and the `CombatPhase` accessor first. It must report: state is `Engaged` **and** the engaged data's `SurpriseLeft` is true. If `combatphase.Machine` exposes no reader for `SurpriseLeft`, add one there (`func (m *Machine) SurpriseLeft() bool`) rather than reaching into the struct from another package.

- [ ] **Step 6: Extend the test**

Add to `surprise_critonwin_test.go` an end-to-end assertion that a snapshotted attacker's landed swing crits, and that the same attacker crits on **every** weapon, not just the first. This is the behaviour change over `backstabCrit`, which consumed after one swing.

- [ ] **Step 7: Run and commit**

Run: `go test ./internal/combat/... ./internal/hooks/...`

```bash
git add internal/combat/ internal/hooks/NewRound_DoCombat.go internal/characters/
git commit -m "feat(u10d): every landed swing of a surprise round crits"
```

---

## Task 5: The opening strike and its skullduggery stack

One swing per surprise round carries the stacked multiplier. Reuse the consume-once mechanism `backstabCrit` already had, but the flag now scales damage rather than forcing a crit — Task 4 already made every swing crit.

**Files:**
- Modify: `internal/combat/combat_helpers.go` (`swingDamageParams`, `buildDamageParams`, `calcHitDamage`)
- Modify: `internal/combat/combat.go` (`:403-407`, `:483`)
- Test: `internal/combat/surprise_opening_strike_test.go` (create)

**Which swing is the opening strike?** The **first landed swing of the round**, consumed exactly like `backstabCrit` was. Do **not** try to identify "the primary weapon" by slot: `collectAttackWeapons` appends the main-hand fist *after* offhand and extra-arm weapons when the main hand is empty, so `weapons[0]` is not reliably the primary. First-landed-swing is deterministic, matches the spec's intent, and needs no slot logic.

- [ ] **Step 1: Write the failing test**

```go
package combat

import (
	"math"
	"testing"
)

// The opening strike stacks CritDamageMultiplier(skullduggery) on top of the
// ordinary combat-skill crit term. Later swings that round do not.
func TestOpeningStrike_StacksOnceThenStops(t *testing.T) {
	sdp := swingDamageParams{
		rawDmgForCrit:     100,
		critDmgMult:       4.0,
		openingStrikeMult: 3.0,
	}

	first, remaining := calcHitDamage(&AttackResult{}, true, true, sdp)
	if remaining {
		t.Fatalf("the opening strike must be consumed by the swing that lands it")
	}

	second, _ := calcHitDamage(&AttackResult{}, true, remaining, sdp)

	// Both are dice rolls around their means, so compare the means via a large
	// sample rather than one roll.
	if !meanRatioNear(t, 100*4.0*3.0, 100*4.0, first, second) {
		t.Fatalf("opening strike %d vs ordinary crit %d: stack not applied", first, second)
	}
}

func TestOpeningStrike_NoStackOutsideASurpriseRound(t *testing.T) {
	sdp := swingDamageParams{rawDmgForCrit: 100, critDmgMult: 4.0, openingStrikeMult: 1.0}
	got, _ := calcHitDamage(&AttackResult{}, true, false, sdp)
	if math.Abs(float64(got)-400) > 400*0.6 {
		t.Fatalf("an ordinary crit must roll around rawDmgForCrit*critDmgMult, got %d", got)
	}
}
```

`dice.RollStat` is stochastic, so assert on a **sampled mean** across at least 2000 iterations rather than a single roll. Write `meanRatioNear` as a local helper that re-rolls both cases and compares averages within a few percent. Do not assert exact equality on a rolled value — that test will flake and someone will delete it.

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/combat/ -run TestOpeningStrike -v`
Expected: `unknown field openingStrikeMult`.

- [ ] **Step 3: Add the field and compute it**

`swingDamageParams` gains:

```go
	// openingStrikeMult is the skullduggery stack applied to the ONE opening
	// strike of a surprise round. 1.0 outside a surprise round. Applied to the
	// crit MEAN before the roll -- dice.RollStat takes its spread from the mean
	// it is handed, so scaling the rolled result instead would stretch the
	// variance and make a high-skullduggery ambush wildly swingy.
	openingStrikeMult float64
```

In `buildDamageParams`, after `critDmgMult`:

```go
		openingStrikeMult: openingStrikeMultiplier(sourceChar),
```

And a new helper in `crit_damage.go`:

```go
// OpeningStrikeMultiplier is the extra crit worth carried by the single
// opening strike of a surprise round: the attacker's skullduggery expressed
// through the same crit-worth curve their combat skill already uses, times the
// ambush-only tuning knob.
//
// Returns 1.0 when the attacker is not in a surprise round, so the caller can
// multiply unconditionally.
func OpeningStrikeMultiplier(attacker *characters.Character) float64 {
	if attacker == nil || !SurpriseRoundActive(attacker) {
		return 1.0
	}
	rank := attacker.GetSkillLevel(skills.Skullduggery)
	return CritDamageMultiplier(rank) *
		float64(configs.GetBalanceConfig().SurpriseOpeningStrikeMultiplier)
}
```

- [ ] **Step 4: Rename `backstab` to `openingStrike` and apply the multiplier**

`calcHitDamage`:

```go
func calcHitDamage(result *AttackResult, isCrit bool, openingStrike bool, sdp swingDamageParams) (int, bool) {
	if isCrit || openingStrike {
		result.Crit = true
		result.BuffTarget = sdp.critBuffs

		critMean := sdp.rawDmgForCrit * sdp.critDmgMult
		if openingStrike {
			// U10d: the ambush's opening strike stacks skullduggery's crit
			// worth on top of the combat skill's. Applied to the mean, before
			// the roll -- see the openingStrikeMult docstring.
			critMean *= sdp.openingStrikeMult
		}

		damageResult := dice.RollStat(critMean)
		dmg := int(math.Round(math.Max(0, damageResult.Value)))
		return dmg, false // consume the opening strike
	}
	damageResult := dice.RollStat(sdp.dmgMean)
	return int(math.Round(math.Max(0, damageResult.Value))), openingStrike
}
```

- [ ] **Step 5: Replace `backstabCrit` at the call site**

`internal/combat/combat.go:403-407` — delete the `backstabCrit` block and the `SetAggro` demotion it carried, replacing it with:

```go
	attackMessagePrefix := ``
	openingStrikeLeft := false
	if sourceChar.Aggro.Type == characters.SurpriseAttack {
		openingStrikeLeft = true
		attackMessagePrefix = `<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `
		sourceChar.SetAggro(sourceChar.Aggro.UserId, sourceChar.Aggro.MobInstanceId, characters.DefaultAttack)
	}
```

**Keep the `SetAggro` demotion.** `combat.go:393` documents that this write only works because `sourceChar` is a pointer, and reverting it silently disables three separate things. Do not touch it.

At `:483`:

```go
				attackTargetDamage, openingStrikeLeft = calcHitDamage(&attackResult, res.crit, openingStrikeLeft, sdp)
```

- [ ] **Step 6: Run tests and commit**

Run: `go test ./internal/combat/...`

```bash
git add internal/combat/
git commit -m "feat(u10d): the opening strike stacks skullduggery crit worth"
```

---

## Task 6: The round boundary

`Machine.OnCombatRoundEnd()` is called from exactly one place today: a test. Wire it.

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat.go`
- Modify: `internal/state/combatphase/combatphase.go` (STUBS comment)
- Test: `internal/hooks/surprise_round_boundary_test.go` (create)

- [ ] **Step 1: Write the failing test**

Assert two things:
1. After one full `DoCombat` round, a surprise attacker's `SurpriseLeft` is false.
2. The attacker has left `Hidden` — **even when nobody retaliated**. This is the regression test for the latent bug: today `Hidden` only breaks when the ambusher is themselves attacked.

```go
func TestSurpriseRound_EndsAfterOneRound_EvenWithNoRetaliation(t *testing.T) {
	// Build an ambusher Engaged via TriggerSurpriseAttack against a target
	// that never attacks back.
	// ... construct per the package's existing DoCombat test helpers ...

	if !ambusher.Character.InSurpriseRound() {
		t.Fatalf("precondition: should start the round in a surprise round")
	}

	DoCombat(events.NewRound{RoundNumber: util.GetRoundCount()})

	if ambusher.Character.InSurpriseRound() {
		t.Fatalf("SurpriseLeft must be consumed after exactly one round")
	}
	if ambusher.Character.IsHidden() {
		t.Fatalf("the ambusher must leave stealth after their surprise round, " +
			"even though nobody attacked them back")
	}
}
```

Read `internal/hooks/NewRound_DoCombat_parity_test.go` for how this package builds a combat scenario. Do not invent a harness.

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/hooks/ -run TestSurpriseRound_Ends -v`
Expected: FAIL — `SurpriseLeft` is still set and the ambusher is still hidden.

- [ ] **Step 3: Consume the boundary at the end of `DoCombat`**

At the end of `DoCombat`, after both combat passes and after `handleAffected`:

```go
	// U10d: close the surprise round. This fires the OnEndOfRoundIfSurprise
	// callback that Awareness_Cascades has been registering since chunk 1 and
	// that NOTHING in production has ever called -- Machine.OnCombatRoundEnd
	// had exactly one caller, a test. Without this the flag is never consumed,
	// so an ambusher leaves stealth only incidentally, when someone attacks
	// them back.
	for _, uid := range users.GetOnlineUserIds() {
		if u := users.GetByUserId(uid); u != nil && u.Character.CombatPhase != nil {
			u.Character.CombatPhase.OnCombatRoundEnd()
		}
	}
	for _, mobId := range mobs.GetAllMobInstanceIds() {
		if m := mobs.GetInstance(mobId); m != nil && m.Character.CombatPhase != nil {
			m.Character.CombatPhase.OnCombatRoundEnd()
		}
	}
```

`OnCombatRoundEnd` already no-ops unless the machine is `Engaged` with `SurpriseLeft` set, so this is cheap and safe to call broadly. Verify that before shipping the loop — read `combatphase.go:459`.

- [ ] **Step 4: Remove the stub marker**

`internal/state/combatphase/combatphase.go:439` — delete the `=== STUBS — Implementations land in Tasks 6-8. ===` banner. Replace the `OnCombatRoundEnd` docstring's forward-looking language with a statement of what now calls it (`internal/hooks.DoCombat`, at end of round).

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/hooks/... ./internal/state/combatphase/...`

```bash
git add internal/hooks/ internal/state/combatphase/
git commit -m "fix(u10d): wire the surprise-round boundary, which production never called"
```

---

## Task 7: Delete the burst

**Files:**
- Delete: `internal/actions/surprise_attack.go`
- Modify: `internal/actions/combat_attack.go` (new home for `EngageAggroType`)
- Modify: `internal/usercommands/attack.go` (`:189`, `:198`, `:307`)
- Modify: `internal/behaviortree/actions_combat.go` (`:44-49`)
- Modify: `internal/mobcommands/attack.go`

- [ ] **Step 1: Move `EngageAggroType` into `combat_attack.go`**

It survives; only its body changes. It no longer fires a burst — it decides the aggro type and owns the cooldown try:

```go
// EngageAggroType reports the Aggro type a new engagement should carry, and
// claims the surprise opener's cost.
//
// A hidden attacker whose special-move cooldown is unavailable opens as an
// ORDINARY attack. That contract predates U10d and is preserved: callers must
// not pre-check IsHidden and must not assume hidden implies surprise.
//
// U10d: the pre-combat burst this used to fire is gone. Round 1 IS the
// surprise round now, resolved by the ordinary melee path.
func EngageAggroType(actor Actor, target Actor) characters.AggroType {
	if target == nil || actor.GetCharacter() == nil {
		return characters.DefaultAttack
	}
	if !actor.GetCharacter().IsHidden() {
		return characters.DefaultAttack
	}
	cfg := configs.GetBalanceConfig()
	if !actor.GetCharacter().TryCooldown("special-move",
		fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		return characters.DefaultAttack
	}
	return characters.SurpriseAttack
}
```

- [ ] **Step 2: Delete the file**

```bash
git rm internal/actions/surprise_attack.go
```

- [ ] **Step 3: Fix `usercommands/attack.go:189`**

The party-member path calls `actions.SurpriseAttack(...)` directly. Delete that call **and its enclosing `if targetMob := ...` block**; the party member's own `attack` command reaches `EngageAggroType` through the normal path. Leave `partyUser.Command(...)` in place.

- [ ] **Step 4: Fix `behaviortree/actions_combat.go`**

It sets `characters.SurpriseAttack` directly from `IsHidden()`, **bypassing the cooldown gate** that the player and `mobcommands` paths respect. That is a live inconsistency, not just dead wording. Route it through the same seam:

```go
	// U10d: through EngageAggroType so a btree ambush respects the
	// special-move cooldown exactly as the player and mobcommands paths do.
	// Setting SurpriseAttack straight from IsHidden() let btree mobs ambush on
	// a cooldown the other two paths honour.
	aggroType := actions.EngageAggroType(
		actions.NewMobActorInRoom(mob, room),
		targetActorFor(targetUserId, targetMobId, room),
	)
	mob.Character.SetAggro(targetUserId, targetMobId, aggroType)
```

Build the target actor with whatever constructor this file already has in scope; if it has none, resolve the user or mob and use `actions.NewUserActorInRoom` / `actions.NewMobActorInRoom`. Watch the import graph — if `internal/behaviortree` importing `internal/actions` creates a cycle, add a small accessor rather than duplicating the cooldown logic.

- [ ] **Step 5: Update the stale comments**

`internal/mobcommands/attack.go` and `internal/usercommands/attack.go` both carry comments describing "the burst" and "whether the burst actually fired". Rewrite them to describe the surprise round. Do not leave prose that documents deleted code.

- [ ] **Step 6: Build, test, commit**

Run: `go build ./... && go test ./internal/actions/... ./internal/usercommands/... ./internal/mobcommands/... ./internal/behaviortree/...`

```bash
git add -u
git commit -m "refactor(u10d): delete the uncontested surprise burst"
```

---

## Task 8: Delete the five orphaned knobs

They are absent from `config.yaml`, so they have always run on Go defaults, and the per-weapon self-penalty concept dies with the volley.

**Files:**
- Modify: `internal/configs/config.balance.go:272-276`
- Modify: `internal/configs/config.balance.misc.go:259-273`

- [ ] **Step 1: Delete the declarations and validators**

Remove `SurpriseAttackOffhandPenalty`, `SurpriseAttackExtraArm1Penalty` through `...ExtraArm4Penalty`, and their five validator blocks.

- [ ] **Step 2: Confirm nothing references them**

```bash
grep -rn "SurpriseAttack.*Penalty" --include=*.go . && echo "STILL REFERENCED" || echo "clean"
```

Expected: `clean`. Also confirm `_datafiles/config.yaml` never mentioned them:

```bash
grep -in "surprise" _datafiles/config.yaml
```

Expected: only your new `SurpriseOpeningStrikeMultiplier`.

- [ ] **Step 3: Build and commit**

Run: `go build ./... && go test ./internal/configs/...`

```bash
git add -u
git commit -m "refactor(u10d): delete five orphaned SurpriseAttack penalty knobs"
```

---

## Task 9: Skullduggery progression on the seam

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_unified.go` (`applyCombatProgression`)
- Test: `internal/hooks/surprise_progression_test.go` (create)

- [ ] **Step 1: Write the failing test**

Progression is probabilistic, so assert on the **use counter**
(`GetSkillUseCount`, as `internal/usercommands/throw_test.go:93` does), never on
whether a rank actually went up. A rank-based assertion will flake and be
deleted.

```go
// Once for the ROUND, not once per weapon hit. A dual-wielder with extra arms
// lands up to six swings in a surprise round; skullduggery must move by one.
func TestSurpriseRound_AwardsSkullduggeryOncePerRound(t *testing.T) {
	atk, def := newSurpriseRoundPair(t)
	equipTwoWeapons(t, atk) // at least two weapons, so >1 clean hit is possible

	before := atk.Character.GetSkillUseCount(string(skills.Skullduggery))
	res := combat.AttackResult{CleanHit: true, WeaponHits: twoCleanWeaponHits()}
	applyCombatProgression(atk, def, &res)
	after := atk.Character.GetSkillUseCount(string(skills.Skullduggery))

	if got := after - before; got != 1 {
		t.Fatalf("skullduggery must be awarded exactly once per surprise round, got %d", got)
	}
}

// Success-only, matching the convention U10's new sites adopted.
func TestSurpriseRound_NoSkullduggeryWithoutACleanHit(t *testing.T) {
	atk, def := newSurpriseRoundPair(t)

	before := atk.Character.GetSkillUseCount(string(skills.Skullduggery))
	res := combat.AttackResult{CleanHit: false}
	applyCombatProgression(atk, def, &res)

	if after := atk.Character.GetSkillUseCount(string(skills.Skullduggery)); after != before {
		t.Fatalf("a surprise round that lands nothing must award no skullduggery")
	}
}

// The combat skill is still awarded alongside it. progression.Outcome holds
// only ONE AttackerSkill, so the second Outcome must not have displaced the
// first.
func TestSurpriseRound_StillAwardsTheCombatSkill(t *testing.T) {
	atk, def := newSurpriseRoundPair(t)

	before := atk.Character.GetSkillUseCount(string(skills.WeaponCombat))
	res := combat.AttackResult{CleanHit: true, WeaponHits: twoCleanWeaponHits()}
	applyCombatProgression(atk, def, &res)

	if after := atk.Character.GetSkillUseCount(string(skills.WeaponCombat)); after <= before {
		t.Fatalf("the combat skill must still progress during a surprise round")
	}
}

// Outside a surprise round nothing changes.
func TestOrdinaryRound_AwardsNoSkullduggery(t *testing.T) {
	atk, def := newOrdinaryRoundPair(t)

	before := atk.Character.GetSkillUseCount(string(skills.Skullduggery))
	res := combat.AttackResult{CleanHit: true, WeaponHits: twoCleanWeaponHits()}
	applyCombatProgression(atk, def, &res)

	if after := atk.Character.GetSkillUseCount(string(skills.Skullduggery)); after != before {
		t.Fatalf("an ordinary melee round must not train skullduggery")
	}
}
```

`newSurpriseRoundPair` publishes a surprise snapshot containing the attacker
(Task 3) and returns actors the way this package's existing DoCombat tests build
them; `newOrdinaryRoundPair` does not publish one. `twoCleanWeaponHits` returns
two `combat.WeaponHitInfo` values with `CleanHit: true` and a real `SkillTag`.
Read `internal/hooks/NewRound_DoCombat_parity_test.go` before writing these.

- [ ] **Step 2: Add the event**

In `applyCombatProgression`, after the per-weapon ordinary-events loop and **outside** it:

```go
	// U10d: a landed surprise round trains skullduggery, once for the round.
	//
	// A SECOND Outcome is structurally required: progression.Outcome carries
	// exactly one AttackerSkill and the loop above already spent it on the
	// combat skill.
	//
	// AttackerStat is deliberately empty. ApplyProgression calls
	// OnSkillUseScaled, which already rolls the skill's primary stat, and only
	// rolls ev.Stat separately when it names a DIFFERENT one. Setting
	// "dexterity" here would be a no-op at best.
	//
	// Success-only, matching the convention U10's new sites adopted.
	if combat.SurpriseRoundActive(atkChar) && res.CleanHit {
		atkChar.ApplyProgression(
			progression.OrdinaryEvents(progression.Outcome{
				AttackerSkill: string(skills.Skullduggery),
			}),
			progression.SideAttacker, atkUid, round)
	}
```

- [ ] **Step 3: Run and commit**

Run: `go test ./internal/hooks/...`

```bash
git add internal/hooks/
git commit -m "feat(u10d): a landed surprise round trains skullduggery, on the U9 seam"
```

---

## Task 10: `CritOrMitigatedDamageScaled`

Ranged needs the same stacked multiplier. Add a scaled sibling rather than a sixth parameter on a function with five call sites.

**Files:**
- Modify: `internal/combat/crit_damage.go`
- Test: `internal/combat/crit_damage_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestCritOrMitigatedDamageScaled_MultipliesTheCritMeanOnly(t *testing.T) {
	// bonus applies on a crit
	// bonus does NOT apply on a normal (mitigated) hit
	// bonus of 0 is treated as 1.0, not as an annihilator
}
```

Fill in real assertions using a sampled mean, as in Task 5.

- [ ] **Step 2: Implement**

```go
// CritOrMitigatedDamageScaled is CritOrMitigatedDamage with an extra crit-only
// multiplier, used by the U10d ranged surprise shot.
//
// bonusCritMult 0 reads as "unset" and means 1.0, matching AttackSide.Mult's
// convention. It applies ONLY on the crit branch: a surprise shot that lands
// as an ordinary hit is an ordinary hit.
func CritOrMitigatedDamageScaled(rawDmg float64, skillRank int, isCrit bool, mitigPct, mitigCap, bonusCritMult float64) int {
	mean := rawDmg
	if isCrit {
		if bonusCritMult == 0 {
			bonusCritMult = 1.0
		}
		mean *= CritDamageMultiplier(skillRank) * bonusCritMult
	} else {
		mean = ApplyMitigation(rawDmg, mitigPct, mitigCap)
	}
	dmg := int(math.Round(dice.RollStat(mean).Value))
	if dmg < 1 {
		dmg = 1
	}
	return dmg
}

// CritOrMitigatedDamage rolls the damage for one spell, conviction or ranged
// hit with no bonus multiplier. See CritOrMitigatedDamageScaled.
func CritOrMitigatedDamage(rawDmg float64, skillRank int, isCrit bool, mitigPct, mitigCap float64) int {
	return CritOrMitigatedDamageScaled(rawDmg, skillRank, isCrit, mitigPct, mitigCap, 1.0)
}
```

Keep the existing `CritOrMitigatedDamage` docstring's warning that the melee channel deliberately does not use it.

- [ ] **Step 3: Run and commit**

Run: `go test ./internal/combat/ -run TestCritOrMitigated -v`

```bash
git add internal/combat/crit_damage.go internal/combat/crit_damage_test.go
git commit -m "feat(u10d): CritOrMitigatedDamageScaled for the ranged surprise bonus"
```

---

## Task 11: `AttackSide.CritOnWin` on the channel seam

**Files:**
- Modify: `internal/combat/defence_multiplier.go` (`AttackSide`, `:316`, `:362`, and the contested path)
- Modify: `internal/combat/skill_moves.go` (`SkillMoveParams`, `:156`)
- Test: `internal/combat/surprise_critonwin_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Assert `ResolveChannelAttack` with `CritOnWin: true` produces `AttackerCrit` on an attack win and **not** on a defence win, using `SetChannelAttackContestRunnerForTest` to make the contest deterministic. That helper exists precisely for this.

- [ ] **Step 2: Add the field**

```go
	// CritOnWin upgrades a WON contest to a crit without deciding it. NOT
	// ForceCrit: that forces the win and returns before a margin is computed.
	// Set by the U10d ranged surprise shot. The melee path carries the same
	// semantics as a parameter on resolveDefenseOutcomeCore -- the two are
	// pinned equivalent by TestCritOnWin_MeleeAndChannelAgree.
	CritOnWin bool
```

- [ ] **Step 3: Apply it inside the attack-win branch**

**Placement matters and is easy to get wrong.** `Defended` is a plain `bool`
**field** (`defence_multiplier.go:213`), not a method, and it is not assigned
until `:437` — which is *after* the attack-win branch has already returned. So a
`!out.Defended` test placed up beside the verdict block reads the zero value and
filters nothing.

The attack win is `if res.Success { ... return out }` at roughly `:416-424`.
Put it inside that block, immediately before its `return out`:

```go
	if res.Success {
		if !res.Floored && res.DefenseRoll.StdDev > 0 {
			out.AttackerNormalizedMargin = res.Margin / (res.DefenseRoll.StdDev * math.Sqrt2)
		}
		// U10d: a surprise shot crits on a won contest. Inside this branch
		// because res.Success IS the attack win -- the defended path below
		// sets out.Defended and returns separately.
		//
		// !res.Floored mirrors the gate the AttackerCrit line above already
		// applies: a floored outcome carries the +-1 sentinel margin and
		// cannot be a crit. !out.AttackerFumble because "a fumbled attack
		// aborts even a winning roll" (the verdict block's own comment).
		if side.CritOnWin && !res.Floored && !out.AttackerFumble {
			out.AttackerCrit = true
		}
		return out
	}
```

Do **not** apply it in the two early-return branches at `:316` (empty defence
set) and `:362` (uncontested roll). Those are already attack wins, and
`AttackerNormalizedMargin`'s docstring explicitly warns that those exits are not
decisive outcomes. A surprise shot against an undefendable target still crits via
`ForceCrit` when it is asleep; otherwise leave those paths alone.

- [ ] **Step 4: Thread the bonus multiplier through `SkillMoveParams`**

```go
	// BonusCritMultiplier scales the crit mean only. 0 means 1.0. Used by the
	// U10d ranged surprise shot.
	BonusCritMultiplier float64
```

`skill_moves.go:156` becomes:

```go
	dmg := CritOrMitigatedDamageScaled(rawDmg, p.Attack.SkillRank, result.Crit, mitig, cap, p.BonusCritMultiplier)
```

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/combat/...`

```bash
git add internal/combat/
git commit -m "feat(u10d): AttackSide.CritOnWin and a bonus crit multiplier on the channel seam"
```

---

## Task 12: The ranged surprise shot

**Files:**
- Modify: `internal/actions/combat_fire.go`
- Test: `internal/actions/combat_fire_surprise_test.go` (create)

Four behaviours, all new:

1. Same-room shot from stealth → `CritOnWin` + skullduggery stack.
2. Firing a surprise shot **reveals** the shooter.
3. A surprise shot burns the shared `special-move` cooldown; an ordinary shot still does not.
4. Cross-room shots from stealth get **none** of it.

- [ ] **Step 1: Write the failing tests**

Read `internal/actions/combat_fire_test.go` first for how this package builds a
shooter, a target and a room, and reuse those helpers. Use
`combat.SetChannelAttackContestRunnerForTest` to make the contest deterministic
so these assert behaviour, not luck.

```go
// A same-room shot from stealth crits on a won contest and carries the
// skullduggery stack.
func TestSurpriseShot_SameRoomCritsAndStacks(t *testing.T) {
	restore := combat.SetChannelAttackContestRunnerForTest(alwaysAttackWins)
	t.Cleanup(restore)

	shooter, target, room := newArmedShooterAndTarget(t)
	// No SetSkillLevel method exists. Every test in this repo seeds a skill
	// by direct map assignment -- see behaviortree/actions_skullduggery_test.go.
	shooter.Character.Skills[string(skills.Skullduggery)] = 50
	hide(t, shooter)

	res := ExecuteFire(NewUserActorInRoom(shooter, room), target.Character.Name)

	if !res.MoveResult.Crit {
		t.Fatalf("a won surprise shot must crit")
	}
	// Compare against the same shot fired unhidden: the stacked shot must be
	// materially larger. Sample both, do not compare single rolls.
	if !surpriseShotIsLarger(t, shooter, target, room) {
		t.Fatalf("the surprise shot must carry the skullduggery stack")
	}
}

// THE ANTI-SNIPING REGRESSION. Before U10d nothing in the fire path ever
// revealed a shooter, so a stacked crit would have been repeatable forever.
func TestSurpriseShot_RevealsTheShooter(t *testing.T) {
	shooter, target, room := newArmedShooterAndTarget(t)
	hide(t, shooter)

	res := ExecuteFire(NewUserActorInRoom(shooter, room), target.Character.Name)

	if !res.Revealed {
		t.Fatalf("a surprise shot must report that it revealed the shooter")
	}
	if shooter.Character.IsHidden() {
		t.Fatalf("the shooter must not still be hidden after a surprise shot")
	}
}

func TestSurpriseShot_BurnsSpecialMoveCooldown(t *testing.T) {
	shooter, target, room := newArmedShooterAndTarget(t)
	hide(t, shooter)

	ExecuteFire(NewUserActorInRoom(shooter, room), target.Character.Name)

	if shooter.Character.TryCooldown("special-move", "4 rounds") {
		t.Fatalf("a surprise shot must have consumed the shared special-move cooldown")
	}
}

// Today's behaviour, preserved: "Fire never burns the special-move cooldown."
// Only the SURPRISE shot does.
func TestOrdinaryShot_DoesNotBurnCooldown(t *testing.T) {
	shooter, target, room := newArmedShooterAndTarget(t)
	// deliberately NOT hidden

	ExecuteFire(NewUserActorInRoom(shooter, room), target.Character.Name)

	if !shooter.Character.TryCooldown("special-move", "4 rounds") {
		t.Fatalf("an ordinary shot must leave the special-move cooldown free")
	}
}

// Cross-room gets none of it: no crit upgrade, no stack, no reveal, no charge.
func TestCrossRoomShot_FromStealthIsOrdinary(t *testing.T) {
	restore := combat.SetChannelAttackContestRunnerForTest(alwaysAttackWins)
	t.Cleanup(restore)

	shooter, target, room, exitName := newCrossRoomShooterAndTarget(t)
	hide(t, shooter)

	res := ExecuteFire(NewUserActorInRoom(shooter, room), exitName+" "+target.Character.Name)

	if !res.CrossRoom {
		t.Fatalf("precondition: this must be a cross-room shot")
	}
	if res.MoveResult.Crit {
		t.Fatalf("a cross-room shot from stealth must not crit merely from winning")
	}
	if res.Revealed || !shooter.Character.IsHidden() {
		t.Fatalf("a cross-room shot must not reveal the shooter")
	}
	if !shooter.Character.TryCooldown("special-move", "4 rounds") {
		t.Fatalf("a cross-room shot must not burn the special-move cooldown")
	}
}
```

`alwaysAttackWins` is a `func(float64, []contest.Entry) contest.Result` returning
a contested attack win below the crit bar, so the only thing that can produce a
crit is `CritOnWin`. `hide`, `newArmedShooterAndTarget`,
`newCrossRoomShooterAndTarget` and `surpriseShotIsLarger` are local helpers —
build them on whatever the package's existing fixtures provide rather than
inventing a parallel harness.

The exact argument string for a cross-room shot (`"<exit> <target>"`) must match
what `ExecuteFire`'s own parser accepts — read the `crossRoom` resolution at the
top of `ExecuteFire` and match it.

- [ ] **Step 2: Decide the surprise shot before the shot resolves**

In `ExecuteFire`, after `crossRoom` is known and after the admission gate passes, before the `ExecuteSkillMove` call:

```go
	// U10d: a same-room shot from stealth is a surprise shot.
	//
	// Cross-room is excluded deliberately. A cross-room shot never SetAggro's,
	// counterSkillMoveExit is reach-gated so it is the one uncounterable
	// attack, and IsSneaking already makes the narration anonymous. A stacked
	// crit on top of all three would be a boss killed from the next room at no
	// risk and with no way for the target to learn who did it.
	surpriseShot := !crossRoom && char.IsHidden()
	if surpriseShot {
		// Matches the melee ambush: the opener costs the shared special-move
		// timer. An ORDINARY shot still does not -- fire deliberately never
		// burned it, and the ranged rotation must stay untouched.
		if !char.TryCooldown("special-move", fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
			surpriseShot = false
		}
	}

	bonusCrit := 1.0
	if surpriseShot {
		bonusCrit = combat.CritDamageMultiplier(char.GetSkillLevel(skills.Skullduggery)) *
			float64(cfg.SurpriseOpeningStrikeMultiplier)
	}
```

- [ ] **Step 3: Feed the seam**

```go
		Attack: combat.AttackSide{
			Stat: char.GetEffectivePerception(), StatName: "perception",
			Skill: skills.RangedCombat, SkillRank: rangedRank,
			Mult:      combat.SituationalAttackMult(char, combat.ChannelRanged),
			ForceCrit: combat.SleepingForceCrit(defChar),
			CritOnWin: surpriseShot,
		},
		BonusCritMultiplier: bonusCrit,
```

- [ ] **Step 4: Reveal the shooter**

After the shot resolves, and only for a surprise shot:

```go
	if surpriseShot {
		// U10d: firing from stealth gives away your position.
		//
		// Before this, NOTHING in the fire path ever revealed a shooter --
		// combat_fire.go, usercommands/shoot.go and mobcommands/shoot.go
		// contain no TransitionToRevealing, no CancelBuffsWithFlag(Hidden) and
		// no ForceVisible. IsHidden() was read exactly once, for narration
		// anonymity. Harmless while a hidden shot was an ordinary shot;
		// with a stacked crit attached it would let a hidden archer fire a
		// maximum-bonus shot every round, forever, unseen.
		char.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger: awareness.TriggerCombatEntered,
		})
		result.Revealed = true
	}
```

Add `Revealed bool` to `FireResult` so the wrappers can speak the line (Task 14). Use a NEW constant, `awareness.TriggerRangedSurpriseShot`, added to `internal/state/awareness/transitions.go` beside the existing triggers. Do not reuse `TriggerSurpriseRoundEnd`: a shot is not a round, and the trigger name is what shows up in transition logs. Do not reuse `TriggerCombatEntered` either -- a cross-room shooter never enters combat at all, so that name would be actively misleading for the one case this feature is most likely to be debugged against.

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/actions/...`

```bash
git add internal/actions/combat_fire.go internal/actions/combat_fire_surprise_test.go
git commit -m "feat(u10d): the same-room ranged surprise shot, which reveals the shooter"
```

---

## Task 13: Skullduggery progression for the ranged shot

**Files:**
- Modify: `internal/actions/combat_fire.go`
- Test: `internal/actions/combat_fire_surprise_test.go`

- [ ] **Step 1: Write the failing test**

A landed surprise shot increments the skullduggery use counter exactly once; a missed one does not; an ordinary shot never does.

- [ ] **Step 2: Add the award**

Beside the existing `RecordAndWait` call:

```go
	if surpriseShot && result.MoveResult.Hit {
		char.ApplyProgression(
			progression.OrdinaryEvents(progression.Outcome{
				AttackerSkill: string(skills.Skullduggery),
			}),
			progression.SideAttacker, actor.GetUserId(), util.GetRoundCount())
	}
```

**Do not touch the ranged-combat award** at `usercommands/shoot.go:199`, and do not add one for mobs. Both are assigned to U10b (see the roadmap row). Changing what an ordinary shot awards would contaminate this slice's playtest with archer-mob scaling.

- [ ] **Step 3: Run and commit**

```bash
git add internal/actions/
git commit -m "feat(u10d): a landed surprise shot trains skullduggery"
```

---

## Task 14: Player-facing copy

**Files:**
- Modify: `internal/combat/` melee narration (the `attackMessagePrefix` path)
- Modify: `internal/usercommands/shoot.go`, `internal/mobcommands/shoot.go`

Rules, non-negotiable: **80-character wrap**, **no raw numbers** (use `combat.GetDamageDescription(amount, targetMaxHP)`), **no en or em dashes**, ESL-clear phrasing.

- [ ] **Step 1: The melee opening strike needs its own line**

Distinct from the other swings of the surprise round, so the player can tell which swing was the big one. The `*[SURPRISE ATTACK]*` prefix already exists and stays.

- [ ] **Step 2: A defended surprise round must name the defence**

New situation: the ambush could not previously be defended at all. It must narrate as the defence that won (dodge, parry, block, quell, defy), not as a generic whiff.

- [ ] **Step 3: The ranged surprise shot**

Its own narration, plus a line telling the shooter they have been revealed. A player who loses stealth silently will file it as a bug. Check that the reveal reads coherently with the existing `IsSneaking` anonymity in the same round: the shot is anonymous, then the shooter is exposed.

- [ ] **Step 4: Verify colours over telnet**

The harness strips colour. Check on port 33333.

- [ ] **Step 5: Commit**

```bash
git add -u
git commit -m "feat(u10d): player-facing copy for the surprise round and shot"
```

---

## Task 15: Helpfiles

**Files:**
- Create: `_datafiles/world/dogmud/templates/help/<topic>.template`
- Modify: `_datafiles/world/dogmud/keywords.yaml`
- Modify: existing `sneak` / `hide` / `skullduggery` / `combat` helpfiles

- [ ] **Step 1: Write the helpfile**

Cover **both** openers. Say plainly that a shot from stealth gives away your position, and that the big bonus applies only in the same room. Describe the feel, never the numbers.

- [ ] **Step 2: Register it in `keywords.yaml`**

The help index is hand-maintained with **no fallback to the command registry**. A helpfile that is not in that YAML never appears in the topic list — that is exactly how `stow` became invisible.

- [ ] **Step 3: Cross-link both directions**

A trigger word appearing in no other helpfile is undiscoverable.

- [ ] **Step 4: Commit**

```bash
git add _datafiles/world/dogmud/
git commit -m "docs(u10d): helpfile for the stealth openers, registered and cross-linked"
```

---

## Task 16: Guards

**Files:**
- Modify: `internal/combat/contest_site_guard_test.go`
- Create: the melee/channel parity test

- [ ] **Step 1: The two `critOnWin` paths must agree**

```go
// The arc has two attack paths, so crit-on-win exists twice: a parameter on
// resolveDefenseOutcomeCore for the melee scoring loop, a field on AttackSide
// for the channel seam. They must not drift.
//
// This is the test that catches someone "simplifying" one of them later.
func TestCritOnWin_MeleeAndChannelAgree(t *testing.T) {
	cases := []struct {
		name      string
		attackWon bool
		critOnWin bool
		wantCrit  bool
	}{
		{"win with critOnWin crits", true, true, true},
		{"win without critOnWin does not", true, false, false},
		{"loss with critOnWin does not", false, true, false},
		{"loss without critOnWin does not", false, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, tgt := testAttackerDefender(t)

			best := defenceWinBest(100, 10)
			if tc.attackWon {
				best = attackWinBest()
			}
			melee := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt,
				2.0, false, false, tc.critOnWin)

			restore := SetChannelAttackContestRunnerForTest(fixedContest(tc.attackWon))
			t.Cleanup(restore)
			channel := ResolveChannelAttack(ChannelRanged, AttackSide{
				Stat: 100, StatName: "perception",
				Skill: skills.RangedCombat, SkillRank: 10,
				CritOnWin: tc.critOnWin,
			}, src, tgt)

			if melee.crit != tc.wantCrit {
				t.Fatalf("melee path: got crit=%v want %v", melee.crit, tc.wantCrit)
			}
			if channel.AttackerCrit != tc.wantCrit {
				t.Fatalf("channel path: got crit=%v want %v", channel.AttackerCrit, tc.wantCrit)
			}
		})
	}
}
```

`fixedContest(attackWon bool)` returns a runner producing a **contested** result
whose margin sits below the crit bar, so neither path can crit for any reason
other than `critOnWin`. Reuse `attackWinBest` / `defenceWinBest` /
`testAttackerDefender` from Task 2.

If the two paths disagree, the bug is real — do not "fix" the test by asserting
each path's own behaviour separately. That defeats the entire point of the test.

- [ ] **Step 2: Mob and player ambushers must resolve identically**

```go
// Spec test 22. The three engagement paths (usercommands/attack.go,
// mobcommands/attack.go, behaviortree/actions_combat.go) must all produce the
// same aggro type for the same situation. Before U10d the btree path set
// SurpriseAttack straight from IsHidden(), bypassing the special-move cooldown
// the other two honoured.
func TestAmbushParity_AllPathsRespectTheCooldown(t *testing.T) {
	for _, path := range []string{"player", "mobcommand", "behaviortree"} {
		t.Run(path, func(t *testing.T) {
			atk, def, room := newHiddenAttackerFor(t, path)

			first := engageVia(t, path, atk, def, room)
			if first != characters.SurpriseAttack {
				t.Fatalf("a hidden opener must be typed SurpriseAttack, got %v", first)
			}

			// Cooldown is now spent; a second immediate opener must be ordinary.
			second := engageVia(t, path, atk, def, room)
			if second != characters.DefaultAttack {
				t.Fatalf("a hidden-but-on-cooldown opener must be ordinary, got %v", second)
			}
		})
	}
}
```

`engageVia` calls the real entry point for each path. If wiring the behaviortree
path into a test is disproportionate, assert instead that
`actions_combat.go`'s `actAttack` calls `EngageAggroType` and never assigns
`characters.SurpriseAttack` directly — an AST assertion in the style of the
existing guards is acceptable and still catches the regression.

- [ ] **Step 3: The auto-hit must not come back**

Extend the site guard in the style of the existing U6b Task 18 guards: assert no production path produces an uncontested surprise hit. The guard walks the AST — read how the existing ones are written and follow the pattern.

- [ ] **Step 4: Commit**

```bash
git add internal/combat/
git commit -m "test(u10d): guard the two critOnWin paths and the deleted auto-hit"
```

---

## Task 17: Documentation

- [ ] **Step 1: `docs/PATCH_NOTES.md`**

A dated entry, player-facing framing, no raw numbers, no em dashes.

- [ ] **Step 2: `context.md` for every touched package**

`internal/combat`, `internal/actions`, `internal/hooks`, `internal/state/combatphase`. Verify every symbol you name actually exists — a `context.md` describing an invented API is worse than none.

- [ ] **Step 3: Mark the roadmap row**

`docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`, the **U10d** row. Record what shipped and what was found, in the style of the U10c row.

- [ ] **Step 4: Commit**

```bash
git add docs/
git commit -m "docs(u10d): patch notes, context.md sweep, roadmap"
```

---

## Task 18: Pre-push gates

- [ ] **Step 1:** `gofmt -l internal/ modules/` — must print nothing.
- [ ] **Step 2:** `go build ./...`
- [ ] **Step 3:** `go test ./...`
- [ ] **Step 4:** Confirm `Logging.LogToFile: false` in `_datafiles/config.yaml`.
- [ ] **Step 5: Isolated boot test.**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

**Exit code 124 is the success case** — the timeout fired because the server stayed up. Do **not** grep for the bare word `panic`: `GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`. Clean up with `git worktree remove --force`.

---

## Task 19: The adversarial playtest gate

Mandatory. This ships new player-facing copy, and boot-clean verifies the system, never the experience.

- [ ] **Step 1: Wipe instance saves**

```bash
rm -rf _datafiles/world/dogmud/mobs.instances/* _datafiles/world/dogmud/rooms.instances/*
```

Do **not** wipe `shops/`, `guilds/` or `moderation/` — those are persistent living state.

- [ ] **Step 2: Run the harness with an explicitly adversarial mandate**

```
/playtest local --checkout <abs> bug-finder <goals>.yaml
```

The goals file must carry `ephemeral:`.

- [ ] **Step 3: Probe these specifically**

- A **sleeping** target: `ForceCrit` and the stacked opening strike compound into an uncontested maximum hit. Spec 2.7 accepts this; confirm it feels like an assassination and not a bug.
- **Mid-fight re-hiding**: can a second ambush be produced against the same opponent?
- **Multi-weapon and Extra Arms**: every swing crits; check the total is exciting rather than absurd.
- **The ranged shot**: confirm the reveal fires, that cross-room gets nothing, and that the anonymity and the reveal read coherently in one round.
- **The comparison the design rests on**: does the ambush feel competitive with a companion build, rather than merely lethal?

- [ ] **Step 4: Extract findings to memory**

Playtest reports are gitignored. Fix what it finds, re-run if needed, and only then hand to the user.

---

## Notes for the reviewer

Deliberate decisions that will look wrong without the spec:

- **`forceCrit` and `critOnWin` both exist and can both be true.** Not redundant. See spec 2.3.
- **No mitigation bypass for surprise.** Crits already bypass mitigation entirely; the old half-bypass was strictly weaker.
- **The ranged opener is easier to land than the melee one.** Intended, not an oversight — `ChannelRanged` answers with dodge, plus block only for a shielded defender. The melee ambusher is paid in volume instead. Spec 2.8.2 says explicitly: do not "fix" this.
- **Ranged-combat's off-seam award and the total absence of mob ranged-combat progression are left alone.** Assigned to U10b by the owner on 2026-08-25.
- **The `SetAggro` demotion in `calculateCombat` stays.** `combat.go:393` documents three things that silently break without it.
