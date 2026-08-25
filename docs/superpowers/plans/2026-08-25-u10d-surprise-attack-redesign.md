# U10d Surprise-Attack Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the uncontested surprise-attack burst with a single contested opening strike that crits on a win, add the same for a same-room ranged shot, and rebalance the ranged economy around an unengaged-damage bonus.

**Architecture:** Delete `actions.SurpriseAttack`. Exactly one attack per engagement is special — the opening strike — flagged by `Aggro.Type == SurpriseAttack` and consumed once, exactly as `backstabCrit` is today. It contests normally; a won contest crits and carries `CritDamageMultiplier(skullduggery)` on top of the combat-skill term. Stealth breaks immediately, achieved by *deleting* the `Awareness_Cascades` branch that preserves `Hidden`. Ranged gets the same via `AttackSide.CritOnWin`, plus a detuned bow line compensated by a bonus when the shooter has no inbound attackers.

**Tech Stack:** Go 1.x, GoMud engine. Packages: `internal/combat`, `internal/hooks`, `internal/actions`, `internal/usercommands`, `internal/mobcommands`, `internal/behaviortree`, `internal/state/combatphase`, `internal/state/awareness`, `internal/configs`, plus eight item YAMLs.

**Spec:** `docs/superpowers/specs/2026-08-25-u10d-surprise-attack-redesign-design.md`

---

## Before you start

Read the spec. This plan is the mechanics; the spec is the contract and records
why several things that look wrong are deliberate.

Five things that will bite you:

1. **`forceCrit` and `critOnWin` are NOT the same.** `forceCrit` forces the WIN — the
   defender never rolls (sleeping victim). `critOnWin` respects the contest and only
   upgrades a win. Never merge them.
2. **Crits already bypass mitigation completely.** `calcHitDamage` rolls off
   `sdp.rawDmgForCrit`, the unmitigated mean. Add no mitigation bypass.
3. **Multipliers go on the MEAN, before `dice.RollStat`.** `RollStat` derives its
   spread from the mean it is handed. Scaling the rolled result instead stretches
   variance and makes high-skill crits wildly swingy.
4. **A defensive win is NOT a miss.** Since U6 Task 10 it lands partial damage:
   `res.hit == true`, `res.defended == true`, `damageMult` in 0.0–0.5. Any test
   asserting "zero damage on a lost contest" is asserting behaviour that has not
   existed for two slices.
5. **Do NOT build a round snapshot.** An earlier draft did. The bonus applies to one
   attack and is consumed by it, so nothing needs to persist across the round.

**Run tests with:** `go test ./internal/combat/... ./internal/hooks/... ./internal/actions/... ./internal/state/...`

---

## File Structure

**Modified:**

| File | Responsibility after this change |
|---|---|
| `internal/configs/config.balance.go` | declares 2 new knobs; loses 5 `SurpriseAttack*Penalty` |
| `internal/configs/config.balance.combat.go` | defaults/validates both new knobs |
| `internal/configs/config.balance.misc.go` | loses the 5 penalty validators |
| `internal/combat/combat_helpers.go` | `resolveDefenseOutcome*` gains a `critOnWin` **parameter** + the four-condition guard; `calcHitDamage` opening-strike stack. **No `combatContext` field** — see Task 3 Step 3. |
| `internal/combat/combat.go` | `backstabCrit` → `openingStrikeLeft`, captured and cleared per-swing at `:466`. **The four `Attack*Vs*` entry points are untouched.** |
| `internal/combat/crit_damage.go` | `OpeningStrikeMultiplier`, `CritOrMitigatedDamageScaled` |
| `internal/combat/skill_moves.go` | `SkillMoveParams.BonusCritMultiplier` |
| `internal/combat/defence_multiplier.go` | `AttackSide.CritOnWin` |
| `internal/hooks/Awareness_Cascades.go` | **loses** the surprise-preservation branch and the round-end registration |
| `internal/state/combatphase/combatphase.go` | **loses** `SurpriseLeft`, `OnCombatRoundEnd`, `OnEndOfRoundIfSurprise` |
| `internal/hooks/NewRound_DoCombat_unified.go` | skullduggery progression |
| `internal/actions/combat_fire.go` | surprise shot; unengaged bonus |
| `internal/actions/combat_attack.go` | `EngageAggroType` relocated |
| `_datafiles/world/dogmud/items/weapons-10000/` | 8 bow multipliers detuned |

**Deleted:** `internal/actions/surprise_attack.go`

---

## Task 1: Two config knobs

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.combat.go`
- Modify: `_datafiles/config.yaml`
- Test: `internal/configs/config_balance_combat_test.go` (create)

**⚠ `_datafiles/config.yaml` has `skip-worktree`.** Edit locally, but build the
committed content from `git show HEAD:_datafiles/config.yaml` plus your addition —
**never from disk**, which carries dev-only overrides.

- [ ] **Step 1: Write the failing test**

```go
package configs

import "testing"

func TestU10dKnobs_DefaultAndValidate(t *testing.T) {
	cases := []struct {
		name string
		get  func(*Balance) float64
		set  func(*Balance, float64)
	}{
		{"SurpriseOpeningStrikeMultiplier",
			func(b *Balance) float64 { return float64(b.SurpriseOpeningStrikeMultiplier) },
			func(b *Balance, v float64) { b.SurpriseOpeningStrikeMultiplier = ConfigFloat(v) }},
		{"SurpriseRangedStrikeMultiplier",
			func(b *Balance) float64 { return float64(b.SurpriseRangedStrikeMultiplier) },
			func(b *Balance, v float64) { b.SurpriseRangedStrikeMultiplier = ConfigFloat(v) }},
		{"RangedUnengagedDamageMultiplier",
			func(b *Balance) float64 { return float64(b.RangedUnengagedDamageMultiplier) },
			func(b *Balance, v float64) { b.RangedUnengagedDamageMultiplier = ConfigFloat(v) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b Balance

			// Absent (zero) must default to 1.0, NOT stay at 0.
			tc.set(&b, 0)
			b.Validate()
			if got := tc.get(&b); got != 1.0 {
				t.Fatalf("zero must default to 1.0, got %v", got)
			}

			tc.set(&b, -3)
			b.Validate()
			if got := tc.get(&b); got != 1.0 {
				t.Fatalf("negative must reset to 1.0, got %v", got)
			}

			tc.set(&b, 2.5)
			b.Validate()
			if got := tc.get(&b); got != 2.5 {
				t.Fatalf("a legal value must survive validation, got %v", got)
			}
		})
	}
}
```

**The zero case is the important one.** The five knobs this slice deletes were
validated as `if x < 0 || x > 1.0 { x = default }`, so an absent key unmarshalled
to 0, passed both tests, and stayed at 0 forever. Do not repeat that shape.

- [ ] **Step 2: Run it and watch it fail**

Run: `go test ./internal/configs/ -run TestU10dKnobs -v`
Expected: compile failure, fields undefined.

- [ ] **Step 3: Declare both fields**

`internal/configs/config.balance.go`, beside the crit knobs:

```go
	SurpriseOpeningStrikeMultiplier ConfigFloat `yaml:"SurpriseOpeningStrikeMultiplier"` // Extra multiplier on a MELEE surprise opening strike (default 1.0)
	SurpriseRangedStrikeMultiplier  ConfigFloat `yaml:"SurpriseRangedStrikeMultiplier"`  // Extra multiplier on a RANGED surprise opening shot (default 1.0)
	RangedUnengagedDamageMultiplier ConfigFloat `yaml:"RangedUnengagedDamageMultiplier"` // Ranged damage multiplier when nothing in the room targets the shooter (default 1.0)
```

- [ ] **Step 4: Default and validate**

`internal/configs/config.balance.combat.go`, in `validateCombat()`:

```go
	// Both default to 1.0 on ANY non-positive value, including the zero an
	// absent YAML key unmarshals to. Deliberately NOT the `< 0 || > 1.0` shape
	// the deleted SurpriseAttack*Penalty knobs used: that shape leaves an
	// absent key at 0 forever, which is exactly why those five were silently
	// inert. 0 is nonsense for both of these; disabling is 1.0.
	if b.SurpriseOpeningStrikeMultiplier <= 0 {
		b.SurpriseOpeningStrikeMultiplier = 1.0
	}
	if b.SurpriseRangedStrikeMultiplier <= 0 {
		b.SurpriseRangedStrikeMultiplier = 1.0
	}
	if b.RangedUnengagedDamageMultiplier <= 0 {
		b.RangedUnengagedDamageMultiplier = 1.0
	}
```

- [ ] **Step 5: Run** — `go test ./internal/configs/ -run TestU10dKnobs -v` → PASS

- [ ] **Step 6: Shipped values in `_datafiles/config.yaml`**

```yaml
  # SurpriseOpeningStrikeMultiplier: extra multiplier on the opening strike of a
  # surprise attack, on top of the stacked combat-skill and skullduggery crit
  # multipliers. 1.0 = no change. Exists so the ambush can be retuned WITHOUT
  # moving CritDamageBase / CritDamagePerSkill, which are global.
  SurpriseOpeningStrikeMultiplier: 1.0

  # RangedUnengagedDamageMultiplier: ranged damage multiplier applied when the
  # shooter has NO inbound attackers. 1.0 = no change. This is the archer's
  # compensation for firing once where melee swings up to four times per weapon,
  # and for reload burning the shared special-move cooldown. It replaces the flat
  # inflation the bow damage_multiplier line used to carry. Raising it restores
  # sustained archery but also grows the surprise opener, which compounds with it.
  RangedUnengagedDamageMultiplier: 2.75

  # SurpriseRangedStrikeMultiplier: the ranged counterpart of
  # SurpriseOpeningStrikeMultiplier, touching the ranged opening shot ALONE.
  # Deliberately below the melee value: a shot answers one fewer defence, and the
  # opener already inherits RangedUnengagedDamageMultiplier because it is
  # unengaged by definition. Without this counterweight, raising that knob to
  # restore sustained archery would push the ambush to roughly 18,000.
  SurpriseRangedStrikeMultiplier: 0.5
```

- [ ] **Step 7: Commit**

```bash
git add internal/configs/ && git commit -m "feat(u10d): add the opening-strike and unengaged-ranged knobs"
```

---

## Task 2: `critOnWin` in the melee resolution core

**Files:**
- Modify: `internal/combat/combat_helpers.go:897`, `:914`
- Test: `internal/combat/surprise_critonwin_test.go` (create)

Two fixtures already exist and you MUST reuse them:
`defenceFixture(defenderStamina int)` (`internal/combat/defense_affordability_test.go:19`)
and `defenceWinBest(rawMargin, defStdDev float64)` (`defence_multiplier_test.go:97`).

**The margin sign is DEFENCE-POSITIVE.** A *negative* margin is an attack win. The
established idiom, used verbatim in two existing tests, is
`defenceWinBest(-15*math.Sqrt2, 15)`; its `hitRoll.ZScore` is 0, well under the 2.0
crit bar. Get the sign backwards and your tests silently exercise a defence win.

- [ ] **Step 1: Write the failing test**

```go
package combat

import (
	"math"
	"testing"
)

// attackWinBest is a settled ATTACK win below the crit bar, so only critOnWin
// can make it crit.
func attackWinBest() bestDefenseResult { return defenceWinBest(-15*math.Sqrt2, 15) }

func TestCritOnWin_UpgradesWinButNeverRescuesALoss(t *testing.T) {
	src, tgt := defenceFixture(1000)

	t.Run("clean attack win becomes a crit", func(t *testing.T) {
		res := resolveDefenseOutcomeCore(&AttackResult{}, attackWinBest(), src, tgt, 2.0, false, false, true)
		if !res.hit || res.defended {
			t.Fatalf("precondition: expected a clean attack win")
		}
		if !res.crit {
			t.Fatalf("critOnWin must upgrade a won contest")
		}
	})

	t.Run("defence win stays a defence win", func(t *testing.T) {
		best := defenceWinBest(15*math.Sqrt2, 15) // positive == defence won
		res := resolveDefenseOutcomeCore(&AttackResult{}, best, src, tgt, 2.0, false, false, true)
		if res.crit {
			t.Fatalf("critOnWin must never rescue a lost contest")
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
			t.Fatalf("a sentinel margin must never be promoted to a crit")
		}
	})
}
```

The floored subtest is not optional. `crit_floor.go:122-130` states the rule:
*"A floored outcome carries the +-1 sentinel margin and represents an outcome the
contest did not actually produce. Promoting it would hand a decisive result to the
side that lost the roll."* Without it, `ContestFloor` (0.125) hands a rank-1
ambusher a one-in-eight maximum-damage assassination on anything in the game.

Confirm `floored` is the real field name on `bestDefenseResult` before writing it.

- [ ] **Step 2: Run it** — expect `too many arguments in call to resolveDefenseOutcomeCore`.

- [ ] **Step 3: Add the parameter to both functions**

```go
// critOnWin upgrades a WON contest to a crit. It does NOT decide the contest.
//
// Deliberately NOT forceCrit. forceCrit forces the WIN outright and the defender
// never answers it (the sleeping-victim contract). critOnWin respects the contest
// in full: the defender rolls and may win, and on a defender win nothing is
// upgraded because there is no clean hit to upgrade.
//
// Both may be true — an ambush against a sleeping target — in which case
// forceCrit decides the outcome and critOnWin is redundant. Do not merge them.
func resolveDefenseOutcome(result *AttackResult, best bestDefenseResult, sourceChar, targetChar *characters.Character, critThreshold float64, isThirdParty, forceCrit, critOnWin bool) hitResolution {
	res := resolveDefenseOutcomeCore(result, best, sourceChar, targetChar, critThreshold, isThirdParty, forceCrit, critOnWin)
	// ... rest unchanged
}
```

- [ ] **Step 4: Split the core so there IS a single exit, then apply the guard**

**There is no "end of the function" to append to.** `resolveDefenseOutcomeCore`
has **seven** `return res` exits (`:990, :1001, :1014, :1064, :1074, :1083,
:1112`). Appending after the last one is unreachable code and leaves the function
without a terminating return. Inlining the guard at the two *obvious* branches
instead silently misses the **defence-fumble** exit at `:1004-1014`, which carries
`res.hit = true, res.defended = false` — a clean attack win by the very idiom the
guard uses.

Rename the existing body and wrap it:

```go
// resolveDefenseOutcomeCore applies the U10d opening-strike upgrade to the
// verdict the inner resolver produced. Split out because the inner function has
// seven exits and the upgrade must apply to all of them uniformly.
func resolveDefenseOutcomeCore(result *AttackResult, best bestDefenseResult, sourceChar, targetChar *characters.Character, critThreshold float64, isThirdParty, forceCrit, critOnWin bool) hitResolution {
	res := resolveDefenseOutcomeInner(result, best, sourceChar, targetChar, critThreshold, isThirdParty, forceCrit)
	// ... the guard from below goes here ...
	return res
}
```

Keeping the exported-to-package name on the wrapper matters: the Step 1 tests call
`resolveDefenseOutcomeCore` directly and assert `res.crit`, so putting the guard in
the outer `resolveDefenseOutcome` wrapper instead would make them fail.

The guard, with all four conditions:

```go
	// U10d: a surprise opening strike crits on a CLEANLY won contest. Placed
	// last so it can only upgrade an outcome the ordinary ordering already
	// settled. Every guard corresponds to a rule this package states elsewhere:
	//
	//   res.hit && !res.defended -- the documented "attack won the contest"
	//     idiom (hitResolution.defended, combat_helpers.go:798-803). hit alone
	//     is TRUE for a deflected partial hit, which is a DEFENCE win.
	//   !best.floored -- crit_floor.go:122: a sentinel margin must never be
	//     promoted. A mercy save must not become a maximum-damage ambush.
	//   !res.fumble -- a fumble aborts even a winning roll.
	if critOnWin && res.hit && !res.defended && !best.floored && !res.fumble &&
		best.margin <= 0 {
		res.crit = true
	}
```

Do not drop a guard to "simplify". Each produces a bug visible only as an
occasional absurd damage number in play.

> **Why `best.margin <= 0` is the fifth guard.** Of the inner function's seven
> exits, six are handled by the first four conditions. The seventh — the
> **defence-fumble** exit at `:1004-1014` — carries
> `res.hit = true, res.defended = false, res.fumble = false` and returns *before*
> `attackWon := best.margin <= 0` is ever computed at `:1055`. So without this
> condition the guard fires on a swing where **the attack may have lost the
> margin** and only won because the defender fumbled.
>
> That would be a genuine semantic divergence from the channel seam, whose guard
> is gated on `res.Success` (Task 9 Step 3) — on roughly the 2.3% of defended
> swings that fumble. And `TestCritOnWin_MeleeAndChannelAgree` (Task 16) asserts
> the two paths agree, so the divergence would sit behind a test that claims it
> cannot exist, because the sketched cases never construct a defence fumble.
>
> Adding `best.margin <= 0` makes the melee guard literally "won the contest",
> matching the channel path. Confirm `margin` is defence-positive on
> `bestDefenseResult` before relying on the sign — it is the same convention as
> `defenceWinBest` in Task 2 Step 1.

- [ ] **Step 5: Fix call sites**

`combat.go:466` gets a **per-swing** value (see the warning in Task 3 Step 6 —
passing `ctx.critOnWin` bare there is a round-scoped bug). For now pass `false`
with a `// Task 3` comment. Update every test caller found by:

```bash
grep -rn "resolveDefenseOutcomeCore(\|resolveDefenseOutcome(" internal/combat/
```

with a trailing `, false`.

**Three of those calls wrap onto two lines**, and the argument list closes on the
*second* one — `defence_multiplier_test.go:143-144`,
`resolution_order_test.go:76-77` and `:116-117`. Appending blindly to the grep hit
puts the argument after a trailing comma mid-list and produces three compile
errors. Append to the **closing** argument line.

- [ ] **Step 6: Run** — `go test ./internal/combat/...` all green.

- [ ] **Step 7: Commit**

```bash
git add internal/combat/ && git commit -m "feat(u10d): critOnWin in the melee core, distinct from forceCrit"
```

---

## Task 3: The opening strike and its skullduggery stack

Exactly **one** attack per engagement is special. Reuse the consume-once mechanism
`backstabCrit` already has.

**Files:**
- Modify: `internal/combat/combat_helpers.go` (`combatContext`, `swingDamageParams`, `buildDamageParams`, `calcHitDamage`)
- Modify: `internal/combat/combat.go:403-407`, `:466`, `:483`
- Modify: `internal/combat/crit_damage.go`
- Test: `internal/combat/surprise_opening_strike_test.go` (create)

- [ ] **Step 1: Write the failing tests**

`dice.RollStat` is stochastic. Assert on a **sampled mean** over at least 2000
iterations. Never assert an exact rolled value.

```go
package combat

import "testing"

// The opening strike stacks skullduggery's crit worth on top of the combat
// skill's, and is consumed by the swing that lands it.
func TestOpeningStrike_StacksOnceThenStops(t *testing.T) {
	sdp := swingDamageParams{rawDmgForCrit: 100, critDmgMult: 4.0, openingStrikeMult: 3.0}

	_, remaining := calcHitDamage(&AttackResult{}, true, true, sdp)
	if remaining {
		t.Fatalf("the opening strike must be consumed by the swing that lands it")
	}

	stacked := meanOver(2000, func() int { d, _ := calcHitDamage(&AttackResult{}, true, true, sdp); return d })
	plain := meanOver(2000, func() int { d, _ := calcHitDamage(&AttackResult{}, true, false, sdp); return d })

	if ratio := stacked / plain; ratio < 2.7 || ratio > 3.3 {
		t.Fatalf("stacked/plain mean ratio %.2f, want ~3.0 (openingStrikeMult)", ratio)
	}
}

// THE REGRESSION TEST for the retired every-swing design: a swing that is not
// the opening strike gets no stack, even when it crits.
func TestOpeningStrike_LaterCritsAreOrdinary(t *testing.T) {
	sdp := swingDamageParams{rawDmgForCrit: 100, critDmgMult: 4.0, openingStrikeMult: 3.0}
	plain := meanOver(2000, func() int { d, _ := calcHitDamage(&AttackResult{}, true, false, sdp); return d })
	if plain < 350 || plain > 450 {
		t.Fatalf("an ordinary crit must roll around rawDmgForCrit*critDmgMult=400, got %.0f", plain)
	}
}

// A DEFENDED opening strike must not pay the stack, and must not consume the
// flag. calcHitDamage is called on every res.hit, deflections included.
func TestOpeningStrike_DefendedSwingDoesNotPayTheStack(t *testing.T) {
	sdp := swingDamageParams{rawDmgForCrit: 100, critDmgMult: 4.0, openingStrikeMult: 3.0}

	// isCrit false is what a deflected hit carries (the Task 2 guard excludes
	// res.defended), so the crit branch must NOT be entered by openingStrike
	// alone -- otherwise a defended opener rolls the full stacked mean and only
	// then gets scaled down by damageMult, delivering roughly half a maximum
	// ambush on a swing the defender won.
	dmg, _ := calcHitDamage(&AttackResult{}, false, true, sdp)
	if dmg > 200 {
		t.Fatalf("a defended swing rolled %d — it took the stacked crit branch", dmg)
	}
}
```

Write `meanOver(n int, f func() int) float64` as a local helper.

- [ ] **Step 2: Run it** — expect `unknown field openingStrikeMult`.

- [ ] **Step 3: Add the context field and the multiplier**

**Do NOT add a `critOnWin` field to `combatContext`.** An earlier draft did, and
it was dead weight: `openingStrikeLeft` (Step 6) is itself derived from
`sourceChar.Aggro.Type == characters.SurpriseAttack` at `:403`, so
`ctx.critOnWin && openingStrikeLeft` is identical to `openingStrikeLeft` alone.
Carrying both creates a second source of truth that someone will later read bare
— which, being round-scoped, is precisely the every-swing bug the boxed warning in
Step 6 exists to prevent. One signal, one place.

`swingDamageParams`:

```go
	// openingStrikeMult is the skullduggery stack for the ONE opening strike of
	// a surprise attack. 1.0 otherwise. Applied to the crit MEAN before the roll
	// -- dice.RollStat takes its spread from the mean it is handed.
	openingStrikeMult float64
```

New helper in `crit_damage.go`:

```go
// OpeningStrikeMultiplier is the extra crit worth carried by the single opening
// strike of a surprise attack: the attacker's skullduggery expressed through the
// same crit-worth curve their combat skill already uses, times the ambush-only
// tuning knob.
//
// channelKnob is the per-channel ambush multiplier, passed by the caller rather
// than read here: melee uses SurpriseOpeningStrikeMultiplier, ranged uses
// SurpriseRangedStrikeMultiplier, and the two ship at different values because a
// shot answers one fewer defence and already inherits the unengaged bonus.
//
// Returns 1.0 for a nil attacker so callers can multiply unconditionally.
func OpeningStrikeMultiplier(attacker *characters.Character, channelKnob float64) float64 {
	if attacker == nil {
		return 1.0
	}
	return CritDamageMultiplier(attacker.GetSkillLevel(skills.Skullduggery)) * channelKnob
}
```

In `buildDamageParams`:

```go
		openingStrikeMult: OpeningStrikeMultiplier(sourceChar,
			float64(configs.GetBalanceConfig().SurpriseOpeningStrikeMultiplier)),
```

- [ ] **Step 4: Rewrite `calcHitDamage`**

```go
func calcHitDamage(result *AttackResult, isCrit bool, openingStrike bool, sdp swingDamageParams) (int, bool) {
	// U10d: the crit branch is selected by the CRIT VERDICT alone. It used to be
	// `isCrit || backstab`, which forced the branch from the flag itself -- under
	// this design that would let a DEFENDED opening strike roll the full stacked
	// mean and consume the flag, then merely scale the result by damageMult,
	// delivering roughly half a maximum ambush on a swing the defender won.
	if isCrit {
		result.Crit = true
		result.BuffTarget = sdp.critBuffs

		critMean := sdp.rawDmgForCrit * sdp.critDmgMult
		if openingStrike {
			critMean *= sdp.openingStrikeMult
		}

		damageResult := dice.RollStat(critMean)
		dmg := int(math.Round(math.Max(0, damageResult.Value)))
		return dmg, false
	}
	damageResult := dice.RollStat(sdp.dmgMean)
	return int(math.Round(math.Max(0, damageResult.Value))), openingStrike
}
```

> **The second return value is now vestigial.** The single production caller
> (`combat.go:483`) discards it, because Step 6 consumes the flag on the throw.
> It is kept only so the five existing test call sites in `crit_damage_test.go`
> and `hitroll_test.go` compile unchanged. An earlier draft computed a `consumed`
> local here and returned `openingStrike && !consumed`, which is a compile-time
> constant `false` — no linter in `.golangci.yml` catches that, and it reads as if
> it encodes a decision it cannot encode. If you prefer, drop the second return
> entirely and update those five call sites; do not leave a fake one.
```

- [ ] **Step 5: Nothing to do in the four entry points**

They are untouched. The signal is read inside `calculateCombat` at `:403` (Step 6),
not threaded through `combatContext`.

**Why `Aggro.Type` and not a snapshot:** the bonus applies to one attack and is
consumed by it, so nothing must persist across the round. This is the same signal
`backstabCrit` uses and it demonstrably fires in production, unlike
`combatphase.SurpriseLeft` which never has (spec 1.1).

- [ ] **Step 6: Rename `backstabCrit` at the call site**

`combat.go:403-407`:

```go
	attackMessagePrefix := ``
	openingStrikeLeft := false
	if sourceChar.Aggro.Type == characters.SurpriseAttack {
		openingStrikeLeft = true
		attackMessagePrefix = `<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `
		sourceChar.SetAggro(sourceChar.Aggro.UserId, sourceChar.Aggro.MobInstanceId, characters.DefaultAttack)
	}
```

**Keep the `SetAggro` demotion.** `combat.go:393` documents that it only works
through the pointer and that reverting it silently disables three separate things.

`:483` → `attackTargetDamage, openingStrikeLeft = calcHitDamage(&attackResult, res.crit, openingStrikeLeft, sdp)`

**`:466` is the one line in this plan most likely to be got wrong. Read this.**

```go
			// U10d: the opening strike is ONE swing, consumed by the swing that
			// is THROWN -- not by the first one that happens to land.
			openingStrikeThisSwing := openingStrikeLeft
			openingStrikeLeft = false

			res := resolveDefenseOutcome(&attackResult, best, sourceChar, targetChar,
				critThreshold, isThirdParty, ctx.forceCrit, openingStrikeThisSwing)
```

and at `:483`, pass the per-swing local and **discard** the returned flag (the
round-level variable is already cleared above):

```go
			attackTargetDamage, _ = calcHitDamage(&attackResult, res.crit, openingStrikeThisSwing, sdp)
```

> **Passing bare `ctx.critOnWin` here is a silent, serious bug.** Line 466 sits
> inside `for j := 0; j < swingCount; j++`, itself inside `for _, ws := range
> plan.weapons`. `ctx` is round-scoped, built once in the four `Attack*Vs*` entry
> points. `ctx.forceCrit` is round-scoped **on purpose** — the sleeping-victim
> contract is "every swing this round crits", which the comment directly above
> that line states.
>
> Copy that wiring for `critOnWin` and you give it the same scope, so **every
> winning swing of the round crits** — reinstating exactly the every-swing design
> spec 2.1 records as retired. It compiles, and the Task 3 unit tests still pass,
> because they exercise `calcHitDamage` in isolation where the stack really is
> consumed once. Only a round-level test catches it. At the owner's ranks with
> Blackrazor the difference is roughly **9,970 intended against 20,600 shipped**.
>
> `openingStrikeThisSwing` captures the flag and `openingStrikeLeft` is cleared
> **immediately, before the contest runs**, so every later swing reads false
> regardless of what the opener did — landed, missed, fumbled or was deflected.

> **Why "thrown" and not "the first swing that lands".** An earlier draft of this
> plan let the flag survive a miss, reasoning that a parried opener has not spent
> the ambush. That is indefensible once you count the swings.
>
> `calcHitDamage` runs only under `res.hit`, and on a non-crit it returns the flag
> unchanged — so a miss, a fumble *and* a deflection all leave it set
> (`combat_helpers.go:996`, `:1106-1112`). The gate would then be true for **every
> remaining swing of the round**, giving the ambush one re-roll per swing.
>
> `calcSwingCount` issues up to 4 swings per weapon, so a main-hand + offhand +
> extra-arm build throws 3 to 12. The probability the ambush lands is
> `1 - (1-p)^N`, not `p`:
>
> | | 1 swing | 6 swings |
> |---|---|---|
> | p = 0.50 | 50% | **98.4%** |
> | p = 0.125 (mercy floor only) | 12.5% | **55.4%** |
>
> A defender good enough to win every honest roll would still eat the full stacked
> ambush more than half the time. That makes spec 2.2's "the defender can deny it"
> and 2.4's "zero if the contest is lost" false, and it would have invalidated the
> melee-versus-ranged comparison outright — the archer genuinely gets one roll,
> the melee ambusher would have got twelve.
>
> Consuming on the throw restores the single roll the design is built on.

- [ ] **Step 6a: Add the round-level test**

The Task 3 Step 1 tests are necessary but NOT sufficient: they call `calcHitDamage`
directly and cannot see the scope bug above. Add one that drives a full
multi-swing round:

```go
// The scope guard. calcHitDamage-level tests pass even when critOnWin is wired
// round-scoped, so this must exercise calculateCombat.
func TestSurpriseRound_ExactlyOneSwingIsUpgraded(t *testing.T) {
	// Build an attacker with a multi-swing weapon setup and
	// Aggro.Type == SurpriseAttack, a defender who reliably loses the contest,
	// then assert across the resolved round that exactly ONE swing carried the
	// stacked multiplier -- and that later swings rolled around the MITIGATED
	// mean, not the crit mean.
}
```

- [ ] **Step 7: Run and commit**

```bash
go test ./internal/combat/...
git add internal/combat/ && git commit -m "feat(u10d): one contested opening strike, stacking skullduggery crit worth"
```

---

## Task 4: Delete the burst

**Files:**
- Delete: `internal/actions/surprise_attack.go`
- Modify: `internal/actions/combat_attack.go`, `internal/usercommands/attack.go` (`:189`, `:198`, `:307`), `internal/mobcommands/attack.go`, `internal/behaviortree/actions_combat.go`

- [ ] **Step 1: Relocate `EngageAggroType` into `combat_attack.go`**

```go
// EngageAggroType reports the Aggro type a new engagement should carry, and
// claims the surprise opener's cost.
//
// A hidden attacker whose special-move cooldown is unavailable opens as an
// ORDINARY attack. That contract predates U10d: callers must not pre-check
// IsHidden and must not assume hidden implies surprise.
//
// U10d: the pre-combat burst this used to fire is gone. The opening strike of
// the ordinary combat round is the surprise now.
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

- [ ] **Step 2: Delete the file AND its two test files**

```bash
git rm internal/actions/surprise_attack.go \
       internal/actions/surprise_attack_test.go \
       internal/actions/surprise_aggro_test.go
```

Both test files call `SurpriseAttack(...)` / `SurpriseAttackOpts{...}`
(`surprise_aggro_test.go:42,52`; `surprise_attack_test.go:63,84,108,137`). Leaving
them breaks `go test ./internal/actions/...` at Step 6 — the commit would not
compile. Read them first: `surprise_aggro_test.go` asserts the `EngageAggroType`
contract (hidden-but-on-cooldown opens as an ordinary attack), which **survives**
this slice. Port that assertion into a new test rather than losing the coverage.

- [ ] **Step 3: `usercommands/attack.go:189`** — delete the direct
`actions.SurpriseAttack(...)` call *and its enclosing `if targetMob := ...` block*.
The party member's own `attack` reaches `EngageAggroType` normally. Keep
`partyUser.Command(...)`.

- [ ] **Step 4: `behaviortree/actions_combat.go:44-49`** — it sets
`characters.SurpriseAttack` straight from `IsHidden()`, **bypassing the cooldown
gate** the player and `mobcommands` paths both respect. Route it through the seam:

```go
	// U10d: through EngageAggroType so a btree ambush respects the special-move
	// cooldown exactly as the player and mobcommands paths do. Setting
	// SurpriseAttack straight from IsHidden() let btree mobs ambush on a
	// cooldown the other two honoured.
	aggroType := characters.DefaultAttack
	if room := rooms.LoadRoom(ctx.RoomId); room != nil {
		var target actions.Actor
		if targetUserId > 0 {
			if u := users.GetByUserId(targetUserId); u != nil {
				target = actions.NewUserActorInRoom(u, room)
			}
		} else if targetMobId > 0 {
			if m := mobs.GetInstance(targetMobId); m != nil {
				target = actions.NewMobActorInRoom(m, room)
			}
		}
		if target != nil {
			aggroType = actions.EngageAggroType(actions.NewMobActorInRoom(mob, room), target)
		}
	}
	mob.Character.SetAggro(targetUserId, targetMobId, aggroType)
```

Two traps here, both from an earlier draft:

- **`room` is not in scope at the aggro assignment.** In `actions_combat.go` the
  existing `room := rooms.LoadRoom(ctx.RoomId)` is declared **inside** the
  `if targetUserId == 0 && targetMobId == 0` block at `:34`, while the assignment
  sits at `:44-50` outside it. Load it locally as above, or hoist the existing one
  with a nil guard.
- **Assign `target` only when non-nil.** `EngageAggroType`'s `target == nil` guard
  does not catch a typed-nil `*mobs.Mob` wrapped in an `Actor` interface.

**Add the import.** `internal/behaviortree/actions_combat.go:8-16` imports
`characters`, `combat`, `mobs`, `rooms`, `state`, `state/activity`, `users` and
`util` — but **not** `actions`. Go imports are per-file, so "the package already
imports it elsewhere" does not help; the snippet will not compile until
`"github.com/GoMudEngine/GoMud/internal/actions"` is added to this file's block.

There is **no import-cycle risk**: ten files in `behaviortree` import `actions`,
and `actions` imports `behaviortree` nowhere.

- [ ] **Step 5:** Rewrite the stale comments in `mobcommands/attack.go` and
`usercommands/attack.go` that describe "the burst". Do not leave prose documenting
deleted code.

- [ ] **Step 6: Build, test, commit**

```bash
go build ./... && go test ./internal/actions/... ./internal/usercommands/... ./internal/mobcommands/... ./internal/behaviortree/...
git add -u && git commit -m "refactor(u10d): delete the uncontested surprise burst"
```

---

## Task 5: Stealth breaks immediately, by deleting code

**Files:**
- Modify: `internal/hooks/Awareness_Cascades.go:36-38`, `:47-52`
- Modify: `internal/state/combatphase/combatphase.go`
- Test: `internal/hooks/surprise_reveal_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// A hidden attacker is revealed the moment they engage, whether or not anyone
// retaliates. Regression test for the latent bug where an ambusher stayed Hidden
// until somebody attacked them back.
func TestHiddenAttacker_IsRevealedOnEngage_WithNoRetaliation(t *testing.T) {
	atk, def, _ := newHiddenAttackerAndTarget(t)

	atk.Character.SetAggro(def.UserId, 0, characters.SurpriseAttack)

	if atk.Character.IsHidden() {
		t.Fatalf("a surprise engagement must reveal the attacker immediately")
	}
}

// ...and they still get their opening strike in that same round, because the
// bonus keys off Aggro.Type, not IsHidden(). Get this ordering wrong and the
// whole feature silently does nothing.
func TestRevealedAmbusher_StillGetsTheOpeningStrike(t *testing.T) {
	atk, def, _ := newHiddenAttackerAndTarget(t)
	atk.Character.SetAggro(def.UserId, 0, characters.SurpriseAttack)

	if atk.Character.Aggro.Type != characters.SurpriseAttack {
		t.Fatalf("revealing must not clear the SurpriseAttack aggro type")
	}
}
```

- [ ] **Step 2: Run it** — the first test fails; the cascade currently preserves `Hidden`.

- [ ] **Step 3: Delete the preservation branch**

`Awareness_Cascades.go:36-38`, remove:

```go
			if r.Trigger == combatphase.TriggerSurpriseAttack {
				return // preserve Hidden through Engaging for surprise
			}
```

A hidden attacker now falls through to the ordinary `Idle → Engaging` cascade every
other attacker already takes. **"Stealth breaks immediately" is implemented by
removing code.**

Ordering is safe: `SetAggro` writes `Aggro.Type` **before** the FSM transition that
fires the cascade (`combat_state_compat.go:123-149`), and the opening strike reads
`Aggro.Type`, not `IsHidden()`.

- [ ] **Step 4: Delete the dead round-boundary machinery**

None of it has ever executed in production (spec 1.1 mechanic 3):

- `Awareness_Cascades.go:47-52` — the `OnEndOfRoundIfSurprise` registration
- `combatphase.go` — `EngagedData.SurpriseLeft`, the `advanceToEngaged` line that
  sets it, `OnCombatRoundEnd`, `OnEndOfRoundIfSurprise`,
  `endOfRoundIfSurpriseCallbacks`, and the `=== STUBS ===` banner
- `awareness.TriggerSurpriseRoundEnd` — **`internal/state/awareness/awareness_test.go:221`
  references it** (`TestAW_018_SurpriseRoundEndReveals`). Delete that test and
  `TestAW_017` alongside it; both assert the surprise-round preservation contract
  this task removes. The Step 5 grep will otherwise print "STILL REFERENCED" with
  nothing to do about it.
- the `combatphase_test.go` tests exercising the hand-filled `Reason` path

**Do NOT "fix" `TransitionToEngaging` to carry its `TransitionReason`.** It does
silently drop it, and that is a real latent trap — but the only consumer of
`EngagingData.Reason` is the `SurpriseLeft` line being deleted here. Fixing a
producer whose sole consumer is going away adds a live path nothing uses. Record it
for U11 instead.

- [ ] **Step 5: Verify nothing references the deleted surface**

```bash
grep -rn "SurpriseLeft\|OnCombatRoundEnd\|OnEndOfRoundIfSurprise\|TriggerSurpriseRoundEnd" --include=*.go . && echo "STILL REFERENCED" || echo clean
```

- [ ] **Step 6: Run and commit**

```bash
go test ./internal/hooks/... ./internal/state/...
git add -u && git commit -m "refactor(u10d): stealth breaks immediately; delete the dead surprise-round machinery"
```

---

## Task 6: Delete the five inert penalty knobs

**Files:** `internal/configs/config.balance.go:272-276`, `internal/configs/config.balance.misc.go:259-273`

They are absent from `config.yaml` **and running at 0.0, not their advertised
defaults** — the validators only reject `< 0 || > 1.0`, and an absent key
unmarshals to 0, which passes both. That is why today's burst auto-hits on every
limb, not just the primary.

- [ ] **Step 1:** Delete the five declarations and their five validator blocks.

- [ ] **Step 2: Verify**

```bash
grep -rn "SurpriseAttack.*Penalty" --include=*.go . && echo "STILL REFERENCED" || echo clean
grep -in "surprise" _datafiles/config.yaml   # only SurpriseOpeningStrikeMultiplier
```

- [ ] **Step 3: Commit**

```bash
go build ./... && go test ./internal/configs/...
git add -u && git commit -m "refactor(u10d): delete five SurpriseAttack penalty knobs that were inert at 0.0"
```

---

## Task 7: Skullduggery progression, melee

**Files:** `internal/hooks/NewRound_DoCombat_unified.go` (`applyCombatProgression`), test `internal/hooks/surprise_progression_test.go` (create)

Assert on the **use counter** (`GetSkillUseCount`), never on whether a rank moved —
progression is probabilistic and a rank assertion will flake.

- [ ] **Step 1: Write the failing tests**

```go
// Once for the engagement, not once per weapon hit.
func TestSurpriseAttack_AwardsSkullduggeryOnce(t *testing.T) {
	atk, def := newSurpriseAttackerPair(t)
	before := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery))

	res := combat.AttackResult{CleanHit: true, WeaponHits: twoCleanWeaponHits()}
	applyCombatProgression(atk, def, &res)

	if got := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery)) - before; got != 1 {
		t.Fatalf("want exactly 1 skullduggery award, got %d", got)
	}
}

// Success-only.
func TestSurpriseAttack_NoSkullduggeryWithoutACleanHit(t *testing.T) {
	atk, def := newSurpriseAttackerPair(t)
	before := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery))

	res := combat.AttackResult{CleanHit: false}
	applyCombatProgression(atk, def, &res)

	if atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery)) != before {
		t.Fatalf("a surprise attack that lands nothing must award no skullduggery")
	}
}

// The combat skill still progresses: Outcome holds only ONE AttackerSkill, so
// the second Outcome must not have displaced the first.
func TestSurpriseAttack_StillAwardsTheCombatSkill(t *testing.T) {
	atk, def := newSurpriseAttackerPair(t)
	before := atk.GetCharacter().GetSkillUseCount(string(skills.WeaponCombat))

	res := combat.AttackResult{CleanHit: true, WeaponHits: twoCleanWeaponHits()}
	applyCombatProgression(atk, def, &res)

	if atk.GetCharacter().GetSkillUseCount(string(skills.WeaponCombat)) <= before {
		t.Fatalf("the combat skill must still progress during a surprise attack")
	}
}

// An ordinary round trains no skullduggery.
func TestOrdinaryRound_AwardsNoSkullduggery(t *testing.T) {
	atk, def := newOrdinaryAttackerPair(t)
	before := atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery))

	res := combat.AttackResult{CleanHit: true, WeaponHits: twoCleanWeaponHits()}
	applyCombatProgression(atk, def, &res)

	if atk.GetCharacter().GetSkillUseCount(string(skills.Skullduggery)) != before {
		t.Fatalf("an ordinary melee round must not train skullduggery")
	}
}
```

**Every `AttackResult` above that expects an award must carry
`WasSurpriseAttack: true`** — that is what Step 3 reads. Setting only
`Aggro.Type` on the fixture proves nothing, because Step 3's boxed warning says
`Aggro.Type` is already demoted by the time `applyCombatProgression` runs. So:

```go
	res := combat.AttackResult{
		WasSurpriseAttack: true,          // <- the signal Step 3 actually reads
		CleanHit:          true,
		WeaponHits:        twoCleanWeaponHits(),
	}
```

and `newOrdinaryAttackerPair`'s case leaves it `false`.

**Do not seed `Aggro.Type` in either fixture.** It lends false confidence: with it
set, two of these tests pass whether or not the feature exists, which is the exact
class of dead test this plan keeps warning about.

`twoCleanWeaponHits` returns two `combat.WeaponHitInfo` with `CleanHit: true` and
a real `SkillTag`. Read `internal/hooks/NewRound_DoCombat_parity_test.go` before
writing these.

- [ ] **Step 3: Add the event**, after the per-weapon loop and **outside** it:

```go
	// U10d: a landed surprise attack trains skullduggery, once for the round.
	//
	// A SECOND Outcome is structurally required: progression.Outcome carries
	// exactly one AttackerSkill and the loop above already spent it on the
	// combat skill.
	//
	// AttackerStat is deliberately empty. ApplyProgression calls
	// OnSkillUseScaled, which already rolls the skill's primary stat, and only
	// rolls ev.Stat separately when it names a DIFFERENT one.
	if res.WasSurpriseAttack && res.CleanHit {
		atkChar.ApplyProgression(
			progression.OrdinaryEvents(progression.Outcome{
				AttackerSkill: string(skills.Skullduggery),
			}),
			progression.SideAttacker, atkUid, round)
	}
```

> **Do NOT read `atkChar.Aggro.Type` here. It is already gone.** Verified against
> the code: `applyCombatProgression` is **Phase 5** of `handleCombatRound`
> (`NewRound_DoCombat_unified.go:189`), which runs *after* the attack resolves —
> and `calculateCombat` demotes `Aggro.Type` to `DefaultAttack` the moment it
> arms the opening strike (`combat.go:403-407`). By Phase 5 the attacker always
> reads `DefaultAttack`, so an `Aggro.Type` condition here would **never fire and
> nothing would fail**: no compile error, no test failure unless one is written
> for it, just a surprise attack that silently trains no skullduggery.
>
> The signal must therefore be carried **out** of the attack on `AttackResult`.

- [ ] **Step 2: Carry the flag out on `AttackResult`** (do this BEFORE Step 3, which consumes it)

In `internal/combat/attackresult.go` (or wherever `AttackResult` is declared):

```go
	// WasSurpriseAttack records that this round armed a surprise opening
	// strike. Carried out because calculateCombat DEMOTES Aggro.Type to
	// DefaultAttack while resolving, so every consumer running after the attack
	// -- progression at Phase 5, messaging, analytics -- would otherwise see no
	// trace that the round was an ambush at all.
	WasSurpriseAttack bool
```

Set it in `calculateCombat` in the same block that arms the strike (Task 3 Step 6):

```go
	if sourceChar.Aggro.Type == characters.SurpriseAttack {
		openingStrikeLeft = true
		attackResult.WasSurpriseAttack = true
		attackMessagePrefix = `<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `
		sourceChar.SetAggro(sourceChar.Aggro.UserId, sourceChar.Aggro.MobInstanceId, characters.DefaultAttack)
	}
```

Add a test asserting `WasSurpriseAttack` survives to the progression phase — that
is the regression guard for this whole class of "the demotion ate my signal" bug.

- [ ] **Step 4: Run and commit**

```bash
go test ./internal/hooks/...
git add internal/hooks/ && git commit -m "feat(u10d): a landed surprise attack trains skullduggery on the U9 seam"
```

---

## Task 8: `CritOrMitigatedDamageScaled`

**Files:** `internal/combat/crit_damage.go`, test `internal/combat/crit_damage_test.go`

- [ ] **Step 1: Write the failing test** — the bonus applies on a crit, not on a
normal hit, and 0 reads as 1.0 rather than annihilating. Use a sampled mean.

- [ ] **Step 2: Implement**

```go
// CritOrMitigatedDamageScaled is CritOrMitigatedDamage with an extra crit-only
// multiplier, used by the U10d ranged surprise shot.
//
// bonusCritMult 0 reads as "unset" and means 1.0, matching AttackSide.Mult's
// convention. It applies ONLY on the crit branch: a surprise shot that lands as
// an ordinary hit is an ordinary hit.
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

// CritOrMitigatedDamage rolls damage for one spell, conviction or ranged hit with
// no bonus multiplier. See CritOrMitigatedDamageScaled.
func CritOrMitigatedDamage(rawDmg float64, skillRank int, isCrit bool, mitigPct, mitigCap float64) int {
	return CritOrMitigatedDamageScaled(rawDmg, skillRank, isCrit, mitigPct, mitigCap, 1.0)
}
```

Keep the existing docstring's warning that the melee channel deliberately does not
use this. The other four callers are untouched.

- [ ] **Step 3: Run and commit**

```bash
go test ./internal/combat/ -run TestCritOrMitigated -v
git add internal/combat/ && git commit -m "feat(u10d): CritOrMitigatedDamageScaled for the ranged surprise bonus"
```

---

## Task 9: `AttackSide.CritOnWin` on the channel seam

**Files:** `internal/combat/defence_multiplier.go`, `internal/combat/skill_moves.go`, test extends `surprise_critonwin_test.go`

- [ ] **Step 1: Write the failing test** using
`SetChannelAttackContestRunnerForTest` to make the contest deterministic: assert
`AttackerCrit` on an attack win with `CritOnWin: true`, and **not** on a defence win.

- [ ] **Step 2: Add the field**

```go
	// CritOnWin upgrades a WON contest to a crit without deciding it. NOT
	// ForceCrit: that forces the win and returns before a margin is computed.
	// Set by the U10d ranged surprise shot. The melee path carries the same
	// semantics as a parameter on resolveDefenseOutcomeCore; the two are pinned
	// equivalent by TestCritOnWin_MeleeAndChannelAgree.
	CritOnWin bool
```

- [ ] **Step 3: Apply it inside the attack-win branch**

**Placement matters.** `Defended` is a plain `bool` **field** (`:213`), not a
method, and is not assigned until `:437` — after the attack-win branch has already
returned. A `!out.Defended` test placed beside the verdict block reads the zero
value and filters nothing.

**It must go BESIDE the `ForceCrit` block at `:388`, not inside `if res.Success`
at `:416`.** The crit/fumble progression tier is paid by
`awardChannelDefenceBonus` at **`:408`** — eight lines *before* `:416` — and it
receives `out.AttackerCrit` **by value**. A crit set at `:416` is invisible to it,
so the ranged surprise shot would earn no crit progression at all, silently.
`ForceCrit` is set at `:388` for precisely this reason.

```go
	if side.ForceCrit {
		out.AttackerCrit = true
		out.AttackerFumble = false
	}
	// U10d: a surprise shot crits on a won contest. Placed HERE, beside
	// ForceCrit and before awardChannelDefenceBonus at :408, because that
	// function takes out.AttackerCrit BY VALUE -- a crit set after it is
	// invisible to the progression tier.
	//
	// res.Success is the attack win. !res.Floored mirrors the gate the
	// AttackerCrit line above already applies: a sentinel margin cannot be a
	// crit. !out.AttackerFumble because a fumbled attack aborts even a winning
	// roll.
	if side.CritOnWin && res.Success && !res.Floored && !out.AttackerFumble {
		out.AttackerCrit = true
	}
```

**Also apply it at the two early returns**, at `:316` (empty defence set) and
`:362` (uncontested roll):

```go
		out.AttackerCrit = side.ForceCrit || side.CritOnWin
```

Both paths are attack wins by the code's own comment at `:313-315` — *"Uncontested
is an attack win, which is what a full multiplier says."* Leaving them out would
mean a defender with **no available defence** denies the ambush bonus while a
fully-defended one grants it, which inverts the fiction and reads as a bug in
play. An earlier draft of this plan excluded them by misreading
`AttackerNormalizedMargin`'s docstring: that warns the *margin* is not meaningful
on those exits, which is a statement about decisiveness scaling, not about whether
the attack won.

- [ ] **Step 4: Thread the bonus through `SkillMoveParams`**

```go
	// BonusCritMultiplier scales the crit mean only. 0 means 1.0.
	BonusCritMultiplier float64
```

`skill_moves.go:156`:

```go
	dmg := CritOrMitigatedDamageScaled(rawDmg, p.Attack.SkillRank, result.Crit, mitig, cap, p.BonusCritMultiplier)
```

- [ ] **Step 5: Run and commit**

```bash
go test ./internal/combat/...
git add internal/combat/ && git commit -m "feat(u10d): AttackSide.CritOnWin and a bonus crit multiplier on the channel seam"
```

---

## Task 10: The ranged surprise shot

**Files:** `internal/actions/combat_fire.go`, test `internal/actions/combat_fire_surprise_test.go` (create)

Four behaviours: same-room stealth shot crits and stacks; it **reveals** the
shooter; it burns the shared cooldown while an ordinary shot does not; cross-room
gets none of it.

- [ ] **Step 1: Write the failing tests**

Read `internal/actions/combat_fire_test.go` first for how this package builds a
shooter, target and room. Use `combat.SetChannelAttackContestRunnerForTest` for
determinism. Seed skills by **direct map assignment** —
`shooter.Character.Skills[string(skills.Skullduggery)] = 50` — there is no
`SetSkillLevel` method; `characters.New()` already populates `Skills` via
`initAllSkills()`, so the map is non-nil.

```go
func TestSurpriseShot_SameRoomCritsAndStacks(t *testing.T) { /* crit true; sampled mean materially above an unhidden shot */ }
func TestSurpriseShot_RevealsTheShooter(t *testing.T)      { /* res.Revealed && !IsHidden() */ }
func TestSurpriseShot_BurnsSpecialMoveCooldown(t *testing.T) { /* TryCooldown now fails */ }
func TestOrdinaryShot_DoesNotBurnCooldown(t *testing.T)      { /* TryCooldown still succeeds */ }
func TestCrossRoomShot_FromStealthIsOrdinary(t *testing.T)   { /* no crit, no stack, no reveal, no charge */ }
```

Fill each body with real assertions on the same pattern as Task 3.

- [ ] **Step 2: Decide the surprise shot before resolution**

After `crossRoom` is known and admission passes, before `ExecuteSkillMove`.
**Capture hidden state early** — `SetAggro` at `:243` reveals a same-room shooter
via the Awareness cascade, so reading `IsHidden()` after it is too late:

```go
	// U10d. NOTE the ordering: a same-room shot calls SetAggro below, which
	// cascades Hidden -> Revealing. Capture the state BEFORE that happens.
	//
	// Cross-room is excluded deliberately: it never SetAggro's, is reach-gated
	// out of counterattacks (the one uncounterable attack), and narrates
	// anonymously. A stacked crit on top of all three would be a boss killed
	// from the next room at no risk and with no way to learn who did it.
	surpriseShot := !crossRoom && result.IsSneaking
	if surpriseShot && !char.TryCooldown("special-move",
		fmt.Sprintf("%d rounds", cfg.SpecialMoveCooldown)) {
		// DENIED, and the player must be told. Reload burns this same timer
		// (combat_reload.go:133), so "reload, sneak, shoot" -- the ordinary way
		// an archer sets up an ambush -- silently produces an ordinary shot.
		// Without a message this reads as the feature being broken.
		surpriseShot = false
		result.SurpriseOnCooldown = true
	}

	bonusCrit := 1.0
	if surpriseShot {
		// The RANGED knob, not the melee one. It ships lower (0.5 against 1.0)
		// because a shot answers one fewer defence and the opener already
		// inherits RangedUnengagedDamageMultiplier -- it is unengaged by
		// definition. Passing the melee knob here would put the ambush near
		// 18,000 instead of ~9,080.
		bonusCrit = combat.OpeningStrikeMultiplier(char,
			float64(cfg.SurpriseRangedStrikeMultiplier))
	}
```

**Use `result.IsSneaking`, not a new local.** `combat_fire.go:153` is the struct
field `IsSneaking: char.IsHidden()`, not a variable — an earlier draft referred to
a `wasHidden` local that does not exist. `result.IsSneaking` is captured at `:153`,
before `SetAggro` at `:244` reveals a same-room shooter, so it is the correct
pre-reveal value.

Add `SurpriseOnCooldown bool` to `FireResult` alongside `Revealed`.

**This denial is not rare.** A loaded bow implies a recent reload, and reload burns
the same `special-move` timer. So the natural setup — reload, sneak, shoot —
denies the opener whenever the reload was within `SpecialMoveCooldown` rounds.
That is a legitimate cost, but it must be visible (Task 14) and tested (below).

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

- [ ] **Step 4: Reveal the shooter explicitly**

```go
	if surpriseShot {
		// U10d: firing from stealth gives away your position.
		//
		// Belt and braces. A same-room shot with no prior aggro is ALREADY
		// revealed indirectly, via SetAggro -> TransitionToEngaging -> the
		// Awareness cascade. But that path does not fire when the shooter is
		// already engaged (Aggro != nil), which is exactly the case that would
		// otherwise let a re-hidden archer fire repeated maximum-bonus shots.
		// This call is a no-op in the first case and load-bearing in the second.
		// errcheck is enabled (.golangci.yml) and TransitionToRevealing returns
		// error, so discard it explicitly or the lint gate flags a new issue.
		_ = char.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger: awareness.TriggerRangedSurpriseShot,
		})
		result.Revealed = true
	}
```

Add `Revealed bool` to `FireResult`, and `TriggerRangedSurpriseShot` to
`internal/state/awareness/transitions.go`. **Do not reuse
`TriggerSurpriseRoundEnd`** (a shot is not a round, and Task 5 deletes it) or
`TriggerCombatEntered` (a cross-room shooter never enters combat, so it would be
misleading in exactly the case this is most likely debugged against).

- [ ] **Step 5: Run and commit**

```bash
go test ./internal/actions/...
git add internal/actions/ internal/state/awareness/ && git commit -m "feat(u10d): the same-room ranged surprise shot, which reveals the shooter"
```

---

## Task 11: Skullduggery progression, ranged

**Files:** `internal/actions/combat_fire.go`, extends the Task 10 test file

- [ ] **Step 1: Write the failing test** — a landed surprise shot increments the
skullduggery use counter once; a missed one does not; an ordinary shot never does.

- [ ] **Step 2: Add the award**, beside `RecordAndWait`:

```go
	if surpriseShot && result.MoveResult.Hit {
		char.ApplyProgression(
			progression.OrdinaryEvents(progression.Outcome{
				AttackerSkill: string(skills.Skullduggery),
			}),
			progression.SideAttacker, actor.GetUserId(), util.GetRoundCount())
	}
```

**Do not touch** the ranged-combat award at `usercommands/shoot.go:199`, and do not
add a mob equivalent. Both are assigned to U10b. Changing what an ordinary shot
awards would contaminate this slice's playtest with archer-mob scaling.

- [ ] **Step 3: Commit**

```bash
git add internal/actions/ && git commit -m "feat(u10d): a landed surprise shot trains skullduggery"
```

---

## Task 12: Detune the bow line

**Files:** eight YAMLs in `_datafiles/world/dogmud/items/weapons-10000/`, test `internal/items/ranged_multiplier_table_test.go` (create)

The ranged line (2.00–7.50) sits entirely above the melee line (0.85–1.50, with
Blackrazor a deliberate 3.75 legendary outlier). A Training Bow worth 5 gold is
4.00 — above every melee weapon except Blackrazor.

- [ ] **Step 1: Write the table test FIRST**

Eight hand edits is exactly the shape of change that silently goes wrong.

```go
// The U10d bow detune (spec 2.8.3). Pinned as a table because eight hand edits
// to YAML are easy to get wrong and impossible to notice.
func TestRangedWeaponMultipliers_MatchTheU10dTable(t *testing.T) {
	want := map[string]float64{
		"Ironhorn Warbow":   2.75,
		"Arbalest":          2.55,
		"Relic Sidearm":     2.20,
		"Hunting Bow":       2.00,
		"Training Bow":      1.45,
		"Primitive Pistol":  1.30,
		"Hand Crossbow":     1.10,
		"Sling":             0.75,
	}
	// Load every Shooting-subtype item spec and compare DamageMultiplier.
	// Fail on any shooting weapon missing from the table too -- a NEW bow added
	// later must be a deliberate decision against this line, not a silent
	// return to 7.5.
}
```

Anchor the data path on `runtime.Caller`, **not** the working directory: test
binaries in this repo do not reliably run in the package directory, and
`internal/actions/economy_test.go` chdirs to the repo root, so relative paths pass
or fail by test order.

- [ ] **Step 2: Run it** — expect eight failures.

- [ ] **Step 3: Apply the table**

| File | `damage_multiplier` |
|---|---|
| `10046-ironhorn_warbow.yaml` | 7.50 → **2.75** |
| `10042-arbalest.yaml` | 7.00 → **2.55** |
| `10049-relic_sidearm.yaml` | 6.00 → **2.20** |
| `10041-hunting_bow.yaml` | 5.50 → **2.00** |
| `10004-training_bow.yaml` | 4.00 → **1.45** |
| `10040-primitive_pistol.yaml` | 3.50 → **1.30** |
| `10039-hand_crossbow.yaml` | 3.00 → **1.10** |
| `10038-sling.yaml` | 2.00 → **0.75** |

Change **only** `damage_multiplier`. Leave value, speed, and everything else alone
— repricing is not in scope.

- [ ] **Step 4: Migrate `EnchantBaseline`, or the detune does not reach existing bows**

**Editing the template YAML does not reach items that already exist.**
`Item.GetSpec()` returns `*i.Spec` whenever it is non-nil
(`internal/items/items.go:314-317`), and `Item.Spec` is persisted into player
saves under the yaml key **`overrides:`** (`items.go:56`).
`_datafiles/world/dogmud/users/3.yaml` already carries several `overrides:` blocks
containing `damage_multiplier`. So any existing bow with an override keeps **7.50
forever**, and the Step 1 table test — which reads templates — cannot see it.

Two distinct populations, and an earlier draft of this step got both wrong:

1. **Enchanted items.** `EnchantBaseline` (`spec_baseline.go:28/39/60`) is restored
   by `enchantments.go:171`. **Do not "clear" it**: `RestoreInto` does an
   unconditional `spec.DamageMultiplier = b.DamageMultiplier`, so zeroing the
   baseline writes **0** into the spec and `ApplyTier` then adds only the tier
   bonus — an enchanted Ironhorn Warbow would land near **0.10**, a 96% nerf
   rather than the intended 63%.
2. **Affixed / renamed / admin-set items**, which carry `Item.Spec` with **no**
   `EnchantBaseline`. `migrateEnchantedItem` early-returns on `EnchantType == ""`
   (`migrate_enchantments.go:56-58`), so an enchantment-only migration skips them
   entirely — and per Step 5, ids 10046 and 10049 exist *only* in loot pools,
   which is exactly this class.

**RESCALE proportionally. Do NOT assign the template value.**

> **Assigning the template value re-lands a documented production regression.**
> `internal/items/spec_baseline.go:5-24` explains that `SpecBaseline` exists
> *because* the baseline used to be the bare template, which *"silently destroyed
> everything an instance had earned above it: affix scaling from instanced loot,
> whose budget is bought with the gold paid to enter the instance
> (`items.CalcLootBudget`)… **Observed on prod: about a 16% damage drop on a set
> of affixed claws.**"*
>
> `affixgen.go:262-265` writes `spec.DamageMultiplier += 0.05` per affix rank, so
> an instanced Ironhorn Warbow might sit at 7.50 + 0.35. An absolute assignment
> flattens it to 2.75 and deletes the 0.35 the player **paid gold** for, with no
> message and no refund — on exactly the item class this migration targets, since
> ids 10046 and 10049 exist only in loot pools.

```go
// preDetuneBowMultipliers are the values the eight Shooting templates carried
// BEFORE the U10d detune. Hardcoded because the templates no longer hold them,
// and the ratio is what lets an affixed instance keep the scaling it bought.
var preDetuneBowMultipliers = map[int]float64{
	10046: 7.50, 10042: 7.00, 10049: 6.00, 10041: 5.50,
	10004: 4.00, 10040: 3.50, 10039: 3.00, 10038: 2.00,
}

func migrateDetunedBow(item *items.Item) bool {
	old, ok := preDetuneBowMultipliers[item.ItemId]
	if !ok || old <= 0 {
		return false
	}
	tmpl := items.GetItemSpec(item.ItemId)
	if tmpl == nil || tmpl.DamageMultiplier <= 0 {
		return false
	}
	ratio := tmpl.DamageMultiplier / old
	changed := false
	// Scale, never assign: an affixed instance sitting above template keeps its
	// earned delta in proportion.
	if item.Spec != nil && item.Spec.DamageMultiplier > 0 {
		item.Spec.DamageMultiplier *= ratio
		changed = true
	}
	if item.EnchantBaseline != nil && item.EnchantBaseline.DamageMultiplier > 0 {
		item.EnchantBaseline.DamageMultiplier *= ratio
		changed = true
	}
	return changed
}
```

Keying on `ItemId` rather than subtype also makes the migration **idempotent**:
once rescaled, running it again multiplies by the same ratio, so it must be
guarded by a save-version bump or a one-shot marker exactly as the other
migrations are. **Check how `MigrateEnchantments` avoids re-running and copy
that mechanism** — do not invent a new one.

Verified signatures: `items.GetItemSpec(itemId int) *ItemSpec` (`itemspec.go:696`),
`ItemSpec.DamageMultiplier float64` (`:282`), `ItemSpec.Subtype ItemSubType`
(`:323`), `items.Shooting ItemSubType = "shooting"` (`:159`).

**Home, and ordering.** `internal/users/users.go:554-566` already runs
`MigrateLegacyPotions()` `:560`, `MigrateEnchantments()` `:561`,
`MigrateNewbieAwakening()` `:564` and `ItemStorage.MigrateStorageSlots()` `:565`
on every load. `migrate_enchantments.go` is the structural precedent. **Run the
bow migration BEFORE `MigrateEnchantments()`** — `ApplyTier` does an
unconditional `item.EnchantBaseline.RestoreInto(&newSpec)`
(`enchantments.go:171`), so a later pass would re-install the stale value over
your fix.

- [ ] **Step 5: Cover the populations a character sweep does NOT reach**

`MigrateEnchantments` walks only `c.Items`, `c.ComponentItems`, `c.PotionItems`
and `c.Equipment.GetAllItemPtrs()` (`migrate_enchantments.go:17-49`). Copying its
shape leaves **four** populations holding 7.50 forever:

| Population | Where | Why it matters |
|---|---|---|
| **Mob equipment** | `mobs.instances/**`, `instance_save.go:50` persists `Equipment *characters.Worn` with `overrides:` | **A live damage source the player fights.** Worst of the four. |
| Bank | `loadedUser.ItemStorage` — not a `Character` collection | Player-owned, comes back into circulation |
| Shop resale stock | `shops/**`, `shopinventory.go:72-75` persists the full instance so "the exact affixes survive" | Buyable at the old value |
| Room containers / corpses | `rooms.instances/**` | Lootable |

Handle them explicitly rather than by omission:

- **`ItemStorage`** — extend the sweep; it is right there at `users.go:565`.
- **Mob instances and shops** — add a boot-time pass, or state in the plan and the
  patch note that they are deliberately not migrated and why. Note that
  `mobs.instances/` and `rooms.instances/` are wiped by the smoke-test SOP and are
  not deployed, so the exposure is **prod long-lived instances only** — real, but
  bounded, and it decays as mobs respawn from templates.

**Do not leave "a user-save migration" implying world coverage.** Whatever is not
migrated must be named.

- [ ] **Step 6: Tests**

Three, not two:

1. an **enchanted** bow at 7.50 → effective 2.75
2. an **affixed** bow at 7.85 (template + 0.35 of paid affix) → **2.88**, i.e. it
   keeps its earned delta in proportion. This is the regression test for the
   16%-damage-drop bug above; an assignment-based migration fails it.
3. running the migration **twice** leaves the value unchanged

- [ ] **Step 7: Note the two knock-on effects in the patch note**

- **One mob is affected.** Of the eight ids only `10004` (Training Bow) is
  *equipped* by a mob — `mobs/test_arena/62-sparring_archer.yaml:34` — a 64%
  damage cut on a test-arena dummy. `10046` and `10049` appear only in loot pools,
  `10038` only as a dialogue `givesItem`. No quest, recipe or starting-equipment
  reference.
- **Item pricing shifts.** `itemvalue/score.go:25` prices
  `DamageMultiplier x 100 x PhysicalDamageWeight`, which feeds mob gear-up and the
  shop-upgrade planner. "Leave `value:` alone" does not make pricing static; it
  just means we are not *re-authoring* it. Say so rather than being surprised.

The melee-with-a-bow path is safe: `combat_helpers.go:393` clamps Shooting
subtypes to `unloadedMeleeDamageCap` (0.30) regardless of the multiplier.

- [ ] **Step 8: Run, boot, commit**

The boot test matters here: item YAML errors panic at startup, not compile time.

```bash
go test ./internal/items/...
git add -u && git commit -m "balance(u10d): detune the bow line onto the melee scale"
```

---

## Task 13: The unengaged ranged bonus

**Files:** `internal/actions/combat_fire.go`, test extends the Task 10 file

- [ ] **Step 1: Write the failing tests**

```go
func TestUnengagedBonus_AppliesWithNoInboundAttackers(t *testing.T) {}
func TestUnengagedBonus_DropsOnceEngaged(t *testing.T)              {} // same shooter, same fight, no re-equip
func TestUnengagedBonus_CrossRoomKeepsIt(t *testing.T)              {} // never engages the shooter
func TestUnengagedBonus_CompoundsWithTheSurpriseShot(t *testing.T)  {} // owner decision, spec 2.8.3
func TestUnengagedBonus_DoesNotApplyToMelee(t *testing.T)           {}
func TestUnengagedBonus_KnobAtOneIsANoOp(t *testing.T)              {}
```

Fill each with real assertions on sampled means. Test 4 must pin the **product**
explicitly so a later "simplification" into alternatives is caught.

- [ ] **Step 2: Write the "is anything targeting me" helper**

> **DO NOT use `Character.Attackers()`.** It always returns empty in production.
> `RecordInboundAttacker` is reachable only through `lookupMachine`, which reads
> `machineRegistry`, which is written only by `combatphase.RegisterMachine` —
> and that has **zero production callers** (verify:
> `grep -rn "RegisterMachine(" --include=*.go . | grep -v "_test.go"` returns only
> the five declarations). `recoveryContest` is already silently inert for this
> reason. Using it would apply the bonus unconditionally and leave the entire
> situational design dead, with nothing failing.

Add to `internal/actions/combat_fire.go`, modelled on the existing scan in
`internal/hooks/combat_retarget.go:80-122`:

```go
// shooterIsUnengaged reports whether nothing in the room currently has the
// shooter as its aggro target.
//
// A room scan rather than Character.Attackers(): that list is never populated
// in production (combatphase.RegisterMachine has no production callers), so it
// always reads empty and would hand out the bonus unconditionally.
//
// It scans the SHOOTER's room, never the target's. So a cross-room sniper who is
// himself in melee IS engaged and loses the bonus -- that is the point of the
// rule, not an edge case. Pass actor.GetRoom(), NOT targetRoom.
func shooterIsUnengaged(char *characters.Character, room *rooms.Room) bool {
	if room == nil {
		return true
	}
	uid, mid := char.GetUserId(), char.MobInstanceId

	for _, instId := range room.GetMobs(rooms.FindFighting) {
		m := mobs.GetInstance(instId)
		// IsInCombat() as well as Aggro != nil, matching combat_retarget.go:80-111
		// exactly. Stale non-nil aggro on an out-of-combat actor would otherwise
		// suppress the bonus.
		if m == nil || m.Character.Aggro == nil || !m.Character.IsInCombat() {
			continue
		}
		if (uid > 0 && m.Character.Aggro.UserId == uid) ||
			(mid > 0 && m.Character.Aggro.MobInstanceId == mid) {
			return false
		}
	}
	for _, pId := range room.GetPlayers(rooms.FindFighting) {
		u := users.GetByUserId(pId)
		if u == nil || u.Character.Aggro == nil || pId == uid || !u.Character.IsInCombat() {
			continue
		}
		if (uid > 0 && u.Character.Aggro.UserId == uid) ||
			(mid > 0 && u.Character.Aggro.MobInstanceId == mid) {
			return false
		}
	}
	return true
}
```

Check `rooms.FindFighting` and the `GetMobs`/`GetPlayers` signatures against
`combat_retarget.go` before writing — match what that file actually does rather
than this sketch.

- [ ] **Step 3: Apply the multiplier**

`shotMult` is already declared at `combat_fire.go:250`. **Modify that line; do not
redeclare it** — a second `shotMult :=` is either a compile error or a shadow
depending on placement.

```go
	// U10d: the archer's compensation is situational, not flat. It pays for
	// firing ONCE where melee swings up to four times per weapon, and for reload
	// burning the shared special-move cooldown. The bow damage_multiplier line
	// used to carry this as flat inflation (Task 12 removed it), which made an
	// archer equally strong whether or not anything was hitting them.
	//
	// Compounds with the surprise stack -- owner decision, spec 2.8.3. A
	// surprise shot is unengaged by definition, so this is deliberate.
	unengagedMult := 1.0
	if shooterIsUnengaged(char, room) {
		unengagedMult = float64(cfg.RangedUnengagedDamageMultiplier)
	}
	shotMult := weapon.GetSpec().DamageMultiplier * float64(cfg.RangedShotScale) * unengagedMult
```

- [ ] **Step 4: The player-facing cue**

Set a flag on `FireResult` when the shot was taken **while engaged**, so the
wrapper can speak one line the first time it happens per engagement. Damage that
silently halves reads as a bug. Do not repeat it every round.

- [ ] **Step 5: Run and commit**

```bash
go test ./internal/actions/...
git add -u && git commit -m "feat(u10d): ranged damage bonus when the shooter has no inbound attackers"
```

---

## Task 14: Player-facing copy

**Files:** melee narration in `internal/combat/`, `internal/usercommands/shoot.go`, `internal/mobcommands/shoot.go`

Rules: **80-character wrap**, **no raw numbers** (use
`combat.GetDamageDescription(amount, targetMaxHP)`), **no en or em dashes**,
ESL-clear.

- [ ] **Step 1:** The melee **opening strike** needs its own line, distinct from the
ordinary swings that follow it. The `*[SURPRISE ATTACK]*` prefix already exists.

- [ ] **Step 2:** A **defended** opening strike must narrate as the defence that
won — **dodge, parry or block only**. `DefenceSetFor(ChannelMelee)` offers exactly
those three; quell is spell-mental and defy is social, so copy for them would be an
unreachable branch. A defended **shot** can only be dodged or blocked.

- [ ] **Step 3:** The ranged surprise shot needs its own narration plus a line
telling the shooter they are revealed. Check it reads coherently with the existing
`IsSneaking` anonymity: the shot is anonymous, then the shooter is exposed.

- [ ] **Step 4:** The **unengaged-bonus cue** from Task 13 Step 3, phrased as the
archer being unable to steady the shot. Once per engagement.

- [ ] **Step 5:** Verify colours over telnet on port 33333 — the harness strips colour.

- [ ] **Step 6:** `git add -u && git commit -m "feat(u10d): player-facing copy for the openers and the unengaged bonus"`

---

## Task 15: Helpfiles

- [ ] **Step 1:** Write the helpfile covering **both** openers and the unengaged
bonus. Say plainly that a shot from stealth gives away your position, that the big
bonus is same-room only, and that shooting is far more effective when nothing is
attacking you — that last one is why archers want a companion or a front-line.
Describe the feel, never the numbers.

- [ ] **Step 2:** Register it in `_datafiles/world/dogmud/keywords.yaml`. The help
index is hand-maintained with **no fallback to the command registry** — an
unregistered helpfile never appears in the topic list. That is how `stow` became
invisible.

- [ ] **Step 3:** Cross-link both directions from `sneak`, `hide`, `skullduggery`,
`combat`, and the ranged/`shoot` topic. A trigger word appearing in no other
helpfile is undiscoverable.

- [ ] **Step 4:** `git add _datafiles/world/dogmud/ && git commit -m "docs(u10d): helpfiles for the stealth openers and the unengaged bonus"`

---

## Task 16: Guards

- [ ] **Step 1: The two `critOnWin` paths must agree**

```go
// The arc has two attack paths, so crit-on-win exists twice: a parameter on
// resolveDefenseOutcomeCore for the melee scoring loop, a field on AttackSide for
// the channel seam. This is the test that catches someone "simplifying" one later.
func TestCritOnWin_MeleeAndChannelAgree(t *testing.T) {
	cases := []struct{ name string; attackWon, critOnWin, wantCrit bool }{
		{"win with critOnWin crits", true, true, true},
		{"win without critOnWin does not", true, false, false},
		{"loss with critOnWin does not", false, true, false},
		{"loss without critOnWin does not", false, false, false},
	}
	// Drive both paths with the same inputs; assert the same verdict.
	// If they disagree, the bug is real. Do NOT "fix" the test by asserting
	// each path's own behaviour separately.
}
```

- [ ] **Step 2: Ambush parity across all three engagement paths**

`usercommands/attack.go`, `mobcommands/attack.go` and
`behaviortree/actions_combat.go` must produce the same aggro type for the same
situation, and all three must respect the cooldown. If wiring the behaviortree path
into a test is disproportionate, an AST assertion that `actAttack` calls
`EngageAggroType` and never assigns `characters.SurpriseAttack` directly is
acceptable and still catches the regression.

- [ ] **Step 3: The auto-hit must not come back.** Extend the site guard in the
style of the existing U6b Task 18 guards: assert no production path produces an
uncontested surprise hit.

- [ ] **Step 4:** `git add internal/combat/ && git commit -m "test(u10d): guard the two critOnWin paths, ambush parity, and the deleted auto-hit"`

---

## Task 17: Documentation

- [ ] **Step 1:** `docs/PATCH_NOTES.md` — dated entry, player-facing framing, no raw
numbers, no em dashes. Cover the bow detune: players own these weapons and will
notice.
- [ ] **Step 2:** `context.md` for every touched package. Verify every symbol you
name exists — a `context.md` describing an invented API is worse than none.
- [ ] **Step 3:** Mark the **U10d** roadmap row in the style of the U10c row.
- [ ] **Step 4:** Record for U11: `TransitionToEngaging` silently drops its
`TransitionReason` (spec 3.3), and the `< 0 || > 1.0` validator shape can never
default a knob whose legitimate range includes 0.
- [ ] **Step 5:** `git add docs/ && git commit -m "docs(u10d): patch notes, context.md sweep, roadmap"`

---

## Task 18: Pre-push gates

- [ ] **Step 1:** `gofmt -l internal/ modules/` — must print nothing.
- [ ] **Step 2:** `go build ./...`
- [ ] **Step 3:** `go test ./...`
- [ ] **Step 4:** Confirm `Logging.LogToFile: false` in `_datafiles/config.yaml`.
- [ ] **Step 5: Isolated boot test.** Item YAML changes (Task 12) panic at startup,
not compile time, so this gate is load-bearing for this slice.

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

**Exit code 124 is the success case.** Do **not** grep for the bare word `panic`:
`GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`. Clean up with
`git worktree remove --force`.

---

## Task 19: The adversarial playtest gate

Mandatory. New player-facing copy, and boot-clean verifies the system, never the
experience.

- [ ] **Step 1: Wipe instance saves**

```bash
rm -rf _datafiles/world/dogmud/mobs.instances/* _datafiles/world/dogmud/rooms.instances/*
```

Do **not** wipe `shops/`, `guilds/` or `moderation/` — persistent living state.

- [ ] **Step 2:** `/playtest local --checkout <abs> bug-finder <goals>.yaml` with an
explicitly adversarial mandate. The goals file must carry `ephemeral:`.

- [ ] **Step 3: Probe these specifically**

- **A fresh character walking into a hidden hostile mob.** The owner accepted that
  mobs get the full mechanic; crits bypass mitigation so armour is not a counter.
  Walk it with a NEW character, not an established one.
- **Is the melee opener worth taking, versus just using a bow?** The two land near
  parity on paper (~9,760 vs ~9,080); confirm it in play.
- **Does the unengaged bonus read?** Does a player notice their damage halving, and
  does the cue explain it?
- **Sustained archery at the detuned multipliers** — does free-firing feel thin?
  The lever is `RangedUnengagedDamageMultiplier`, not the bow table.
- **A sleeping target**: `ForceCrit` plus the stacked opening strike is an
  uncontested maximum hit. Confirm it reads as an assassination, not a bug.
- **Mid-fight re-hiding**: can a second opening strike be produced? `Sneak`'s only
  combat gate is `Aggro != nil`, and a successful sneak sets no cooldown.
- **The reveal**: fires on a same-room shot, not on cross-room.

- [ ] **Step 4:** Playtest reports are gitignored — extract findings to memory. Fix
what it finds, re-run if needed, and only then hand to the user.

---

## Notes for the reviewer

Deliberate decisions that will look wrong without the spec:

- **`forceCrit` and `critOnWin` both exist and can both be true.** Not redundant.
- **No mitigation bypass for surprise.** Crits already bypass mitigation entirely.
- **No round snapshot, and `SurpriseLeft` is DELETED not fixed.** It has never been
  true in production (spec 1.1); its only consumer is being removed.
- **Stealth breaks by deleting a special case**, not by adding a reveal.
- **The unengaged bonus compounds with the surprise stack.** Owner decision,
  spec 2.8.3, taken with the number stated.
- **The bow detune is not a nerf in isolation** — it is paid back by the unengaged
  bonus in the intended playstyle. Judge them together.
- **Ranged-combat's off-seam award and the absent mob award are left alone.**
  Assigned to U10b.
- **The `SetAggro` demotion in `calculateCombat` stays.** `combat.go:393` documents
  three things that silently break without it.
