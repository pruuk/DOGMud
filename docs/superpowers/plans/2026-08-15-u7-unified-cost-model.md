# U7 Unified Cost Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every action that costs a resource prices it through one formula — flat config base, encumbrance for physical actions, inverse-skill for any action with a skill, per-action modifier — with a clamped product and no lost fractions.

**Architecture:** A new `internal/costs` package holds the three pure multiplier functions and the action registry. `Character` gains a fractional-remainder carry so sub-integer costs are neither erased nor rounded up. `calculateCombat` switches to pointer parameters first, because until it does, every defence charge is written to a discarded struct copy and any cost work on that path is unobservable.

**Tech Stack:** Go, `internal/characters` (pools, progression), `internal/combat` (melee resolution), `internal/configs` (balance knobs), `_datafiles/config.yaml`, `_datafiles/world/dogmud/biomes/`.

**Design authority:** `docs/superpowers/specs/2026-08-12-unified-cost-and-harm-design.md` as corrected by `docs/superpowers/specs/2026-08-15-u7-cost-model-prebuild-findings.md`. **Where the two disagree, the 2026-08-15 document wins.**

---

## Numbers this plan ships

| Knob | Value | Why |
|---|---|---|
| `AttackBaseStaminaCost` | 1.0 | Per swing. Duels last 34-41 rounds, four-mob rooms 15-19. At 0.5 the mechanism is unfelt; at 2.0 a newbie empties in 6-8 rounds against any four-mob room. |
| `DefenceBaseStaminaCost` | 1.0 | Per defence mounted, shared by dodge/parry/block. |
| `DodgeCostModifier` | 1.25 | Dodging moves the whole body. Deliberately the dearest defence. |
| `ParryCostModifier` | 1.10 | |
| `BlockCostModifier` | 1.15 | |
| `AttackCostModifier` | 1.00 | |
| `CostEncumbranceKnee` | 0.75 | Gentle to 75% of capacity, punishing above it. |
| `CostEncumbranceKneeMult` | **1.5** | Owner-tuned down from 2.0. A realistically laden character lands near 1.25 rather than 1.5, so the whole gentle segment is flatter and the steep one from the knee to capacity is steeper. |
| `CostEncumbranceMax` | 5.0 | Clamped at and above capacity. |
| `CostSkillMultAtZero` | **1.10** | Owner-tuned from 1.25. A rank-1 newcomer is barely penalised. |
| `CostSkillMultAtMid` | 1.00 | |
| `CostSkillMidRank` | **25** | Owner-tuned from 35. Neutral arrives sooner. |
| `CostSkillMultAtCap` | **0.40** | Owner-tuned from 0.75. Mastery is worth far more: Meirok pays 0.65x, so his swing costs 0.81 against a newcomer's 1.35. |
| `CostSkillCapRank` | 100 | |
| `PlayerStaminaRegenPct` | **0.05** | Owner-tuned from the shipped 0.02. Raised deliberately to offset defence costing something for the first time. |
| `StaminaCombatRegenDivisor` | **2** | **New knob.** Today the in-combat quarter is a hardcoded `/4` in both the player and mob branches of `AutoHeal`. Halving it, with the raised percentage, lifts in-combat regen from about 0.67 per round to 3.3-4.7. |
| `CostTotalMultiplierMax` | 6.0 | Unclamped worst case is 5.0 x 1.25 x 1.25 = 7.81. |
| `MovementBaseStaminaCost` | 0.5 | Was 2.0. With the shared curve this makes ordinary travel cheaper and near-capacity travel dearer. |
| `MovementCostFloor` | 1 | Every move costs at least 1. |
| `MovementSearchTrainChance` | 0.005 | 1-in-200 gate. At 1-in-50 walking becomes the dominant way to train search. |

### What this tuning produces

| Character | Duel | Four mobs | Wolf den | Combat regen | Refill |
|---|---|---|---|---|---|
| Newcomer | 85 rounds | 23 | 14 | 3.33/rd | 4 min |
| Mid | 76 | 25 | 16 | 4.00/rd | 4 min |
| Meirok | 88 | 40 | 26 | 4.67/rd | 4 min |

Nothing falls below 14 rounds anywhere in the world. Duels are never a stamina
race, a four-mob room bites, and the worst room in the game bites hard without
being a death sentence. Meirok's mastery shows: his swing costs 0.81 where a
newcomer's costs 1.35.

### Regen consolidation (owner, 2026-08-15)

**The player and mob regen knobs collapse into one set.** Mobs and players fight
the same combat under the same cost model, so `MobHealthRegenPct`,
`MobStaminaRegenPct` and `MobConvictionRegenPct` are deleted and the three
`Player*` knobs are renamed to drop the prefix. `StaminaPerRound` and its two
siblings stop branching on `c.IsMob`.

Two things to keep straight while doing it:

- The **combat divisor is stamina-only**. `AutoHeal` quarters stamina in combat
  but applies conviction and health at the full tick, with an explicit
  "not affected by combat state" comment on conviction. Consolidating the
  percentages must not accidentally quarter conviction, or every caster loses
  four fifths of their in-combat recovery.
- Mob regen currently reaches the mob branch of `AutoHeal` on the same
  three-round cadence, so the cadence needs no change; only the knob it reads.

### Open questions this tuning raises, to settle before Task 12

1. **Health and conviction stay at 0.02 while stamina rises to 0.05.** That is
   defensible because stamina is the pool U7 puts under pressure, but it is now
   the odd one out among three knobs that were deliberately matched. Note the
   interaction with the point above: conviction is already effectively four
   times stronger than stamina in combat because it is never quartered.
2. **Faster regen slightly slows stat progression.** Regen-based progression
   fires per tick with a chance of `RegenProgressionBase x (1 - currentPct)^3`,
   so a fuller pool progresses more slowly. Faster stamina regen therefore damps
   Vitality and Willpower gain a little. The new defence costs push the other
   way; net effect unknown and worth watching in the playtest rather than
   pre-compensating.

**Rounding is fractional carry, never `math.Ceil` per action.** Verified: ceiling rounds dodge, parry and block to the *same* integer at every base from 0.5 to 3.0 for a rank-1 character, erasing the three modifiers above, and overcharges by roughly 10% besides.

---

## File structure

| File | Responsibility |
|---|---|
| `internal/costs/skill.go` (new) | `SkillMultiplier(rank int) float64` — the inverse-skill band |
| `internal/costs/encumbrance.go` (new) | `EncumbranceMultiplier(carried, capacity float64) float64` — the two-segment curve |
| `internal/costs/action.go` (new) | The action registry: base knob, pool, skill, physical flag |
| `internal/costs/cost.go` (new) | `Calc(...)` — composes the factors, applies the product clamp |
| `internal/costs/context.md` (new) | Package documentation, per project convention |
| `internal/characters/pools.go` | Fractional-remainder carry: `ApplyCostFloat` |
| `internal/characters/resources.go` | `GetDefenseCost`, `GetMovementStaminaCost`, `EffectivePoolMax` |
| `internal/combat/combat.go` | Pointer signature; per-swing attack charging |
| `internal/combat/combat_helpers.go` | Defence charge now reaches the real character |
| `internal/usercommands/go.go` | Movement cost + rare search training |
| `internal/usercommands/stand.go` | Effective-max gate (live lockout fix) |
| `_datafiles/config.yaml` | All knobs above |
| `_datafiles/world/dogmud/biomes/*.yaml` | `movementcost` values (currently absent world-wide) |

---

### Task 1: Make `calculateCombat` take pointers

**This is a prerequisite, not a cleanup.** `calculateCombat` takes its combatants by value and all four wrappers pass copies, so `runBestOfAllDefense`'s defence charge writes to a discarded struct. Wiring any cost formula into that path before this lands produces a beautifully tuned cost that charges nothing, and every test asserting `ApplyCostPartial` was called still passes.

**Files:**
- Modify: `internal/combat/combat.go` (signature at :430, call sites at :54, :137, :220, :271)
- Test: `internal/combat/cost_seam_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/combat/cost_seam_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// The defence charge must reach the REAL defender. Before this task,
// calculateCombat took its combatants by value, so runBestOfAllDefense
// charged a discarded copy and melee defence was free in production.
func TestDefenceChargeReachesTheRealDefender(t *testing.T) {
	def := characters.New()
	def.Stats.Vitality.Base = 100
	def.Stats.Vitality.Recalculate()
	def.Validate()
	def.StaminaMax.Value = 100
	def.Stamina = 100

	// Charge through the same pair runBestOfAllDefense uses, via a pointer
	// obtained the way calculateCombat will hold it after this task.
	charge := func(target *characters.Character) {
		_ = target.ApplyCostPartial(
			characters.DefensePool(characters.DefenseParry),
			target.GetDefenseCost(characters.DefenseParry))
	}
	charge(def)

	if def.Stamina == 100 {
		t.Fatalf("defence charge did not reach the real defender: stamina still %d", def.Stamina)
	}
}
```

- [ ] **Step 2: Run test to verify it passes for the helper but the seam is still broken**

Run: `go test ./internal/combat/ -run TestDefenceChargeReachesTheRealDefender -v`
Expected: PASS. This test pins the *primitive*; the seam itself is proven by Step 3's test.

- [ ] **Step 3: Add the seam regression test**

Append to the same file:

```go
// calculateCombat must not copy its combatants. A copy silently discards every
// in-place mutation its callees make, which is how melee defence came to cost
// nothing. This test drives the real function and asserts the defender's pool
// moved.
func TestCalculateCombatDoesNotDiscardDefenderCharges(t *testing.T) {
	pinContestFloorOff(t)

	atk := characters.New()
	atk.Stats.Strength.Base = 200
	atk.Stats.Dexterity.Base = 200
	atk.Stats.Strength.Recalculate()
	atk.Stats.Dexterity.Recalculate()
	atk.Validate()
	atk.HealthMax.Value = 1000
	atk.Health = 1000
	atk.StaminaMax.Value = 1000
	atk.Stamina = 1000

	def := characters.New()
	def.Stats.Dexterity.Base = 100
	def.Stats.Vitality.Base = 100
	def.Stats.Dexterity.Recalculate()
	def.Stats.Vitality.Recalculate()
	def.Validate()
	def.HealthMax.Value = 5000
	def.Health = 5000
	def.StaminaMax.Value = 500
	def.Stamina = 500

	before := def.Stamina
	for i := 0; i < 10; i++ {
		_ = calculateCombat(atk, def, User, Mob, combatContext{
			sourceCanSee: true, targetCanSee: true,
		})
	}

	if def.Stamina >= before {
		t.Fatalf("defender paid nothing across 10 rounds: stamina %d -> %d", before, def.Stamina)
	}
}
```

- [ ] **Step 4: Run it to verify it fails to compile**

Run: `go test ./internal/combat/ -run TestCalculateCombatDoesNotDiscardDefenderCharges -v`
Expected: FAIL to build — `cannot use atk (variable of type *characters.Character) as characters.Character value`. That compile error is the point: the signature is wrong.

- [ ] **Step 5: Change the signature and the four call sites**

In `internal/combat/combat.go`, change the declaration at line 430 from:

```go
func calculateCombat(sourceChar characters.Character, targetChar characters.Character, sourceType SourceTarget, targetType SourceTarget, ctx combatContext) AttackResult {
```

to:

```go
// Takes POINTERS deliberately. Value parameters silently discarded every
// in-place mutation the callees make -- most importantly the defence stamina
// charge in runBestOfAllDefense, which meant melee defence cost nothing in
// production from the day it was written. Do not "simplify" these back to
// values: the compiler cannot catch what that breaks, and the tests that
// assert a charge was requested keep passing while nothing is charged.
func calculateCombat(sourceChar *characters.Character, targetChar *characters.Character, sourceType SourceTarget, targetType SourceTarget, ctx combatContext) AttackResult {
```

Inside the function body, remove the `&` from every `&sourceChar` and `&targetChar` (they are already pointers now). Then update the four call sites:

- `:54` `calculateCombat(*user.Character, mob.Character, ...)` becomes `calculateCombat(user.Character, &mob.Character, ...)`
- `:137` `calculateCombat(*userAtk.Character, *userDef.Character, ...)` becomes `calculateCombat(userAtk.Character, userDef.Character, ...)`
- `:220` `calculateCombat(mob.Character, *user.Character, ...)` becomes `calculateCombat(&mob.Character, user.Character, ...)`
- `:271` `calculateCombat(mobAtk.Character, mobDef.Character, ...)` becomes `calculateCombat(&mobAtk.Character, &mobDef.Character, ...)`

`Mob.Character` is a value field, so the two mob-side arguments need `&`. `UserRecord.Character` is already a pointer.

- [ ] **Step 6: Run the tests**

Run: `go test -count=1 ./internal/combat/ ./internal/hooks/`
Expected: PASS. If a test fails asserting a character was NOT modified, it was encoding the bug; read it, and update it only if it is genuinely asserting the old broken behaviour — say so in the report.

- [ ] **Step 7: Commit**

```bash
git add internal/combat/combat.go internal/combat/cost_seam_test.go
git commit -m "fix(combat): calculateCombat takes pointers, so charges reach the real character

Value parameters meant every in-place mutation the callees made was
written to a discarded copy. The defence stamina charge in
runBestOfAllDefense was the significant one: melee dodge, parry and block
have cost nothing in production since they were written, while the
attacker's cost survived only because the wrappers charge it outside this
function. Damage was unaffected, travelling back in AttackResult.

This also reactivates two other writes that were being discarded:
cross-round momentum, and the SurpriseAttack-to-DefaultAttack demotion in
SetAggro. The latter is a suspected live crit-lock and is worth its own
look.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: The inverse-skill cost multiplier

**Files:**
- Create: `internal/costs/skill.go`
- Create: `internal/costs/skill_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/costs/skill_test.go`:

```go
package costs

import (
	"math"
	"testing"
)

// The band is deliberately narrow: a wide one drains a new player in their
// first exchange, which is the failure mode this curve exists to avoid.
func TestSkillMultiplierBand(t *testing.T) {
	cases := []struct {
		rank int
		want float64
	}{
		{0, 1.250},
		{35, 1.000},
		{100, 0.750},
		{150, 0.750}, // clamped beyond the cap rank
	}
	for _, c := range cases {
		got := SkillMultiplier(c.rank)
		if math.Abs(got-c.want) > 0.0005 {
			t.Errorf("rank %d: got %.4f, want %.4f", c.rank, got, c.want)
		}
	}
}

// Monotonically decreasing: more skill never costs more.
func TestSkillMultiplierIsMonotonic(t *testing.T) {
	prev := SkillMultiplier(0)
	for r := 1; r <= 120; r++ {
		got := SkillMultiplier(r)
		if got > prev+0.0001 {
			t.Fatalf("rank %d: multiplier rose from %.4f to %.4f", r, prev, got)
		}
		prev = got
	}
}

// A negative rank cannot be cheaper than rank zero.
func TestSkillMultiplierNegativeRankClampsToZero(t *testing.T) {
	if SkillMultiplier(-5) != SkillMultiplier(0) {
		t.Fatalf("negative rank must clamp to the rank-0 penalty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/costs/ -run TestSkillMultiplier -v`
Expected: FAIL — the package does not exist yet.

- [ ] **Step 3: Implement**

Create `internal/costs/skill.go`:

```go
// Package costs prices every action in the game through one formula:
//
//	cost = base x encumbrance(actor) x skill(actor) x modifier(action)
//
// Encumbrance applies to physical actions only; the skill multiplier applies to
// any action with an associated skill, mental and social included.
package costs

import "github.com/GoMudEngine/GoMud/internal/configs"

// SkillMultiplier returns the cost multiplier for a character at the given rank
// in the action's governing skill. It runs INVERSE to skill: a practised
// fighter spends less stamina on the same parry.
//
// Two linear segments joined at the neutral rank. This is NOT
// combat.SkillMultiplier, which is a sqrt curve scaling damage upward -- same
// name, opposite direction, different job. Keep them apart.
//
// The band is deliberately narrow. A wide band drains a new player in their
// first exchange, which is the exact failure this curve exists to avoid.
func SkillMultiplier(rank int) float64 {
	b := configs.GetBalanceConfig()

	atZero := float64(b.CostSkillMultAtZero)
	atMid := float64(b.CostSkillMultAtMid)
	atCap := float64(b.CostSkillMultAtCap)
	midRank := int(b.CostSkillMidRank)
	capRank := int(b.CostSkillCapRank)

	if rank <= 0 {
		return atZero
	}
	if rank >= capRank {
		return atCap
	}
	if rank <= midRank {
		if midRank == 0 {
			return atMid
		}
		t := float64(rank) / float64(midRank)
		return atZero + (atMid-atZero)*t
	}
	span := capRank - midRank
	if span <= 0 {
		return atCap
	}
	t := float64(rank-midRank) / float64(span)
	return atMid + (atCap-atMid)*t
}
```

- [ ] **Step 4: Add the config knobs**

In `internal/configs/config.balance.go`, add to the `Balance` struct alongside the other cost fields:

```go
	CostSkillMultAtZero ConfigFloat `yaml:"CostSkillMultAtZero"`
	CostSkillMultAtMid  ConfigFloat `yaml:"CostSkillMultAtMid"`
	CostSkillMultAtCap  ConfigFloat `yaml:"CostSkillMultAtCap"`
	CostSkillMidRank    ConfigInt   `yaml:"CostSkillMidRank"`
	CostSkillCapRank    ConfigInt   `yaml:"CostSkillCapRank"`
```

In `internal/configs/config.balance.progression.go` (the sibling that defaults progression-shaped knobs), add to its defaulting function:

```go
	if b.CostSkillMultAtZero == 0 {
		b.CostSkillMultAtZero = 1.25
	}
	if b.CostSkillMultAtMid == 0 {
		b.CostSkillMultAtMid = 1.0
	}
	if b.CostSkillMultAtCap == 0 {
		b.CostSkillMultAtCap = 0.75
	}
	if b.CostSkillMidRank == 0 {
		b.CostSkillMidRank = 35
	}
	if b.CostSkillCapRank == 0 {
		b.CostSkillCapRank = 100
	}
```

Verify the exact field-type names (`ConfigFloat`, `ConfigInt`) and the defaulting function's name by reading the neighbouring knobs in those two files before writing. Do not invent a type.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/costs/ -run TestSkillMultiplier -v`
Expected: PASS, all three tests.

- [ ] **Step 6: Commit**

```bash
git add internal/costs/skill.go internal/costs/skill_test.go internal/configs/
git commit -m "feat(costs): inverse-skill cost multiplier

A practised fighter spends less stamina on the same parry. Narrow band by
design, 1.25 at rank 0 through 1.00 at 35 to 0.75 at 100, because a wide
band drains a new player in their first exchange.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: The encumbrance cost multiplier

**Files:**
- Create: `internal/costs/encumbrance.go`
- Create: `internal/costs/encumbrance_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/costs/encumbrance_test.go`:

```go
package costs

import (
	"math"
	"testing"
)

// Gentle to the knee, punishing from the knee to capacity, clamped above it.
func TestEncumbranceMultiplierCurve(t *testing.T) {
	cases := []struct {
		name             string
		carried, capacity float64
		want             float64
	}{
		{"empty", 0, 100, 1.0},
		{"quarter load", 25, 100, 1.3333},
		{"at the knee", 75, 100, 2.0},
		{"at capacity", 100, 100, 5.0},
		{"above capacity clamps", 250, 100, 5.0},
		{"halfway up the steep segment", 87.5, 100, 3.5},
	}
	for _, c := range cases {
		got := EncumbranceMultiplier(c.carried, c.capacity)
		if math.Abs(got-c.want) > 0.001 {
			t.Errorf("%s: got %.4f, want %.4f", c.name, got, c.want)
		}
	}
}

// A zero or negative capacity must not divide by zero or hand out a discount.
func TestEncumbranceMultiplierZeroCapacity(t *testing.T) {
	if got := EncumbranceMultiplier(10, 0); got != 1.0 {
		t.Fatalf("zero capacity: got %.4f, want 1.0", got)
	}
	if got := EncumbranceMultiplier(10, -5); got != 1.0 {
		t.Fatalf("negative capacity: got %.4f, want 1.0", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/costs/ -run TestEncumbranceMultiplier -v`
Expected: FAIL — `undefined: EncumbranceMultiplier`.

- [ ] **Step 3: Implement**

Create `internal/costs/encumbrance.go`:

```go
package costs

import "github.com/GoMudEngine/GoMud/internal/configs"

// EncumbranceMultiplier prices what the actor is carrying into every PHYSICAL
// action. Mental and social actions never take it.
//
// Two segments: gentle from empty to the knee, steep from the knee to capacity,
// clamped at and above capacity. The shape is deliberate -- a normally equipped
// adventurer should feel almost nothing, and the cost should bite hard only as
// they approach what they can actually carry.
//
// It is NOT the curve GetMovementStaminaCost used before U7, which was flat
// until the actor EXCEEDED capacity and then ramped to double capacity. That
// one left every realistically loaded character at exactly 1.0, which is to say
// it did nothing at all for anybody who was not deliberately overloaded.
func EncumbranceMultiplier(carried, capacity float64) float64 {
	if capacity <= 0 {
		return 1.0
	}
	b := configs.GetBalanceConfig()
	knee := float64(b.CostEncumbranceKnee)
	kneeMult := float64(b.CostEncumbranceKneeMult)
	max := float64(b.CostEncumbranceMax)

	r := carried / capacity
	if r <= 0 {
		return 1.0
	}
	if r >= 1.0 {
		return max
	}
	if r <= knee {
		if knee <= 0 {
			return kneeMult
		}
		return 1.0 + (kneeMult-1.0)*(r/knee)
	}
	span := 1.0 - knee
	if span <= 0 {
		return max
	}
	return kneeMult + (max-kneeMult)*((r-knee)/span)
}
```

- [ ] **Step 4: Add the config knobs**

In `internal/configs/config.balance.go`:

```go
	CostEncumbranceKnee     ConfigFloat `yaml:"CostEncumbranceKnee"`
	CostEncumbranceKneeMult ConfigFloat `yaml:"CostEncumbranceKneeMult"`
	CostEncumbranceMax      ConfigFloat `yaml:"CostEncumbranceMax"`
```

Defaults 0.75, 2.0 and 5.0 in the same sibling defaulting function used in Task 2.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/costs/ -run TestEncumbranceMultiplier -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/costs/encumbrance.go internal/costs/encumbrance_test.go internal/configs/
git commit -m "feat(costs): two-segment encumbrance cost multiplier

Gentle to 75% of carry capacity, steep from there to capacity, clamped
above. Replaces a curve that was flat until the actor exceeded capacity,
which left every realistically loaded character at exactly 1.0.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Fractional cost carry

Without this the whole plan is decoration. Verified: rounding each action to a whole number collapses dodge, parry and block onto the *same* integer at every base from 0.5 to 3.0 for a rank-1 character, so the 1.25 / 1.10 / 1.15 modifiers become invisible to exactly the players they matter most to, and the bill is inflated by roughly 10% on top.

**Files:**
- Modify: `internal/characters/pools.go`
- Test: `internal/characters/cost_carry_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/characters/cost_carry_test.go`:

```go
package characters

import "testing"

// A 1.5-per-action cost must average 1.5, not 2.0. Rounding every action up
// erases the per-action modifiers and overcharges; truncating every action
// erases them and undercharges. Carrying the remainder does neither.
func TestApplyCostFloatCarriesTheRemainder(t *testing.T) {
	c := New()
	c.StaminaMax.Value = 1000
	c.Stamina = 1000

	for i := 0; i < 10; i++ {
		c.ApplyCostFloat(PoolStamina, 1.5)
	}

	spent := 1000 - c.Stamina
	if spent != 15 {
		t.Fatalf("ten actions at 1.5 should cost 15, cost %d", spent)
	}
}

// The dial must survive: three costs that differ by 14% must produce
// different totals over a run of actions.
func TestApplyCostFloatPreservesSmallModifierDifferences(t *testing.T) {
	spend := func(each float64) int {
		c := New()
		c.StaminaMax.Value = 10000
		c.Stamina = 10000
		for i := 0; i < 100; i++ {
			c.ApplyCostFloat(PoolStamina, each)
		}
		return 10000 - c.Stamina
	}
	dodge := spend(2.5)  // base 2 x 1.25
	parry := spend(2.2)  // base 2 x 1.10
	block := spend(2.3)  // base 2 x 1.15

	if !(parry < block && block < dodge) {
		t.Fatalf("modifier ordering lost: parry=%d block=%d dodge=%d", parry, block, dodge)
	}
}

// Carry must never let a pool go negative, and a short charge must report it.
func TestApplyCostFloatNeverGoesNegative(t *testing.T) {
	c := New()
	c.StaminaMax.Value = 100
	c.Stamina = 3

	res := c.ApplyCostFloat(PoolStamina, 10.0)
	if c.Stamina != 0 {
		t.Fatalf("pool went to %d, want 0", c.Stamina)
	}
	if !res.Short {
		t.Fatalf("a charge larger than the pool must report Short")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/characters/ -run TestApplyCostFloat -v`
Expected: FAIL — `c.ApplyCostFloat undefined`.

- [ ] **Step 3: Implement**

In `internal/characters/pools.go`, add the carry field to `Character` (find the struct in `internal/characters/character.go` and add it there, marked so it is never persisted):

```go
	// costCarry banks the sub-integer remainder of every cost so small
	// per-action modifiers survive. Pools are ints; a cost of 2.5 charged
	// action-by-action must average 2.5 rather than rounding to 2 or 3 each
	// time. Not persisted: an in-flight fraction is worth less than a byte in
	// the save file, and a stale one after a reload would be indistinguishable
	// from a rounding bug.
	costCarry map[Pool]float64 `yaml:"-"`
```

Then in `pools.go`:

```go
// ApplyCostFloat charges a fractional cost, carrying the remainder to the next
// call so the average charged converges exactly on the cost asked for.
//
// This exists because the per-action modifiers are small. Dodge, parry and
// block differ by 14%, and rounding each action to a whole number collapses all
// three onto the same integer for a low-skill character at every base value
// this game would ship, which makes the modifiers decoration. Rounding up also
// overcharges by about 10%.
//
// The carry is per pool and per character, and is deliberately not persisted.
func (c *Character) ApplyCostFloat(pool Pool, amount float64) CostResult {
	if amount <= 0 {
		return CostResult{}
	}
	if c.costCarry == nil {
		c.costCarry = make(map[Pool]float64, 3)
	}
	debt := c.costCarry[pool] + amount
	whole := math.Floor(debt)
	c.costCarry[pool] = debt - whole

	if whole <= 0 {
		return CostResult{}
	}
	return c.ApplyCostPartial(pool, int(whole))
}
```

Add `"math"` to the file's imports if it is not already there.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/characters/ -run TestApplyCostFloat -v`
Expected: PASS, all three.

- [ ] **Step 5: Confirm the pool-mutation guard still passes**

Run: `go test -count=1 -run TestPoolMutationGoesThroughThePrimitives .`
Expected: `ok`. `ApplyCostFloat` delegates to `ApplyCostPartial` rather than touching a pool directly, so the AST guard at the repo root stays satisfied.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/pools.go internal/characters/character.go internal/characters/cost_carry_test.go
git commit -m "feat(characters): fractional cost carry

Pools are integers and the per-action cost modifiers are small. Rounding
each action to a whole number collapses dodge, parry and block onto the
same integer for a low-skill character at every base this game would
ship, so the modifiers become decoration; rounding up also overcharges by
about a tenth. Banking the remainder makes the average charged converge
on the real cost and keeps the dials meaningful.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: The action registry and the composed cost

**Files:**
- Create: `internal/costs/action.go`
- Create: `internal/costs/cost.go`
- Create: `internal/costs/cost_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/costs/cost_test.go`:

```go
package costs

import (
	"math"
	"testing"
)

func TestCalcComposesEveryFactor(t *testing.T) {
	// base 1.0, encumbrance at the knee (2.0), skill rank 35 (1.0),
	// modifier 1.25  ->  2.5
	got := Calc(Input{
		Base:     1.0,
		Carried:  75,
		Capacity: 100,
		Physical: true,
		SkillRank: 35,
		HasSkill: true,
		Modifier: 1.25,
	})
	if math.Abs(got-2.5) > 0.001 {
		t.Fatalf("got %.4f, want 2.5", got)
	}
}

// Mental and social actions never take encumbrance, however laden the actor is.
func TestCalcSkipsEncumbranceForNonPhysical(t *testing.T) {
	laden := Input{Base: 10, Carried: 100, Capacity: 100, Physical: false,
		SkillRank: 35, HasSkill: true, Modifier: 1.0}
	empty := laden
	empty.Carried = 0

	if Calc(laden) != Calc(empty) {
		t.Fatalf("a mental action must cost the same laden or empty")
	}
}

// An action with no associated skill takes no skill multiplier.
func TestCalcSkipsSkillWhenTheActionHasNone(t *testing.T) {
	in := Input{Base: 10, Physical: false, HasSkill: false, SkillRank: 0, Modifier: 1.0}
	if math.Abs(Calc(in)-10.0) > 0.001 {
		t.Fatalf("got %.4f, want 10.0 with no skill multiplier applied", Calc(in))
	}
}

// The product of the multipliers is clamped. Worst legal stack is
// encumbrance 5.0 x skill 1.25 x dodge 1.25 = 7.8125, which the clamp bounds.
func TestCalcClampsTheProduct(t *testing.T) {
	got := Calc(Input{
		Base: 1.0, Carried: 100, Capacity: 100, Physical: true,
		SkillRank: 0, HasSkill: true, Modifier: 1.25,
	})
	if got > 6.0001 {
		t.Fatalf("product not clamped: got %.4f, want <= 6.0", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/costs/ -run TestCalc -v`
Expected: FAIL — `undefined: Calc`, `undefined: Input`.

- [ ] **Step 3: Implement the composed cost**

Create `internal/costs/cost.go`:

```go
package costs

import "github.com/GoMudEngine/GoMud/internal/configs"

// Input is everything Calc needs to price one action. Callers assemble it from
// the actor and the action registry rather than passing a Character, which
// keeps this package a config-only leaf with no dependency on internal/characters.
type Input struct {
	Base      float64 // flat config base for the action
	Carried   float64 // actor's carried weight
	Capacity  float64 // actor's carry capacity
	Physical  bool    // physical actions take the encumbrance multiplier
	SkillRank int     // rank in the action's governing skill
	HasSkill  bool    // false for actions with no associated skill
	Modifier  float64 // per-action tuning knob
}

// Calc returns the cost of one action, before it is charged.
//
// The multipliers are clamped as a PRODUCT, not individually. Clamping each
// factor separately still allows encumbrance 5.0 and a rank-0 penalty and a
// dodge premium to stack into something no laden novice can pay, which silently
// becomes autofail-everything.
func Calc(in Input) float64 {
	mult := 1.0
	if in.Physical {
		mult *= EncumbranceMultiplier(in.Carried, in.Capacity)
	}
	if in.HasSkill {
		mult *= SkillMultiplier(in.SkillRank)
	}
	if in.Modifier > 0 {
		mult *= in.Modifier
	}

	if maxMult := float64(configs.GetBalanceConfig().CostTotalMultiplierMax); maxMult > 0 && mult > maxMult {
		mult = maxMult
	}
	return in.Base * mult
}
```

- [ ] **Step 4: Implement the action registry**

Create `internal/costs/action.go`:

```go
package costs

import "github.com/GoMudEngine/GoMud/internal/skills"

// Action names one priced action. Adding a cost to something that is free today
// should be a registry entry plus a config base, never a change at the call site.
type Action string

const (
	ActionAttack Action = "attack"
	ActionDodge  Action = "dodge"
	ActionParry  Action = "parry"
	ActionBlock  Action = "block"
	ActionMove   Action = "move"
)

// Spec describes how one action is priced: which skill governs it, and whether
// it is physical (and therefore pays encumbrance).
type Spec struct {
	Skill    skills.SkillTag
	HasSkill bool
	Physical bool
}

// registry is the single table. Everything the arc still owes a cost to --
// ranged, taunt, rally, warcry, the thirteen special moves, grapple initiation,
// sneak -- gets an entry here and a config base, and needs no new plumbing.
var registry = map[Action]Spec{
	ActionAttack: {Skill: skills.WeaponCombat, HasSkill: true, Physical: true},
	ActionDodge:  {Skill: skills.UnarmedCombat, HasSkill: true, Physical: true},
	ActionParry:  {Skill: skills.WeaponCombat, HasSkill: true, Physical: true},
	ActionBlock:  {Skill: skills.WeaponCombat, HasSkill: true, Physical: true},
	ActionMove:   {Skill: skills.Search, HasSkill: true, Physical: true},
}

// SpecFor returns the pricing spec for an action. An unregistered action is
// priced with no skill and no encumbrance rather than panicking, so a caller
// that adds an action without a registry entry gets a flat base cost instead of
// a crash in combat.
func SpecFor(a Action) Spec {
	if s, ok := registry[a]; ok {
		return s
	}
	return Spec{}
}
```

Verify the `skills.SkillTag` constant names (`WeaponCombat`, `UnarmedCombat`, `Search`) against `internal/skills/skills.go` before writing.

- [ ] **Step 5: Add the clamp knob**

In `internal/configs/config.balance.go` add `CostTotalMultiplierMax ConfigFloat`, defaulting to `6.0` in the same sibling function used in Task 2.

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/costs/ -v`
Expected: PASS, every test in the package.

- [ ] **Step 7: Write the package context.md**

Create `internal/costs/context.md` following the project convention (Purpose, Files, core types in a `go` block, Public API with verified signatures, Gotchas, Dependencies, Consumers). The Gotchas section must record: the product clamp rather than per-factor clamps; that `SkillMultiplier` here runs opposite to `combat.SkillMultiplier`; and that this package must stay free of any `internal/characters` import so it remains a leaf.

- [ ] **Step 8: Commit**

```bash
git add internal/costs/ internal/configs/
git commit -m "feat(costs): action registry and the composed cost

One table says which skill governs an action and whether it is physical;
one function composes base, encumbrance, inverse skill and the per-action
modifier, clamping the PRODUCT so a laden novice cannot be priced out of
acting entirely. Pricing something that is free today becomes a registry
entry plus a config base rather than a change at the call site.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Defence costs onto the formula

**Files:**
- Modify: `internal/characters/resources.go` (`GetDefenseCost`, `GetDefenseStaminaCost`)
- Modify: `internal/combat/combat_helpers.go` (the charge site)
- Test: `internal/characters/defence_cost_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/characters/defence_cost_test.go`:

```go
package characters

import "testing"

// Dodge is deliberately the most expensive defence: moving the whole body is
// more tiring than interposing a weapon or a shield.
func TestDefenceCostOrderingDodgeIsDearest(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 100
	c.Stats.Strength.Recalculate()
	c.Validate()

	dodge := c.GetDefenseCostFloat(DefenseDodge)
	parry := c.GetDefenseCostFloat(DefenseParry)
	block := c.GetDefenseCostFloat(DefenseBlock)

	if !(parry < block && block < dodge) {
		t.Fatalf("want parry < block < dodge, got parry=%.3f block=%.3f dodge=%.3f",
			parry, block, dodge)
	}
}

// A laden character pays more to defend than an unladen one.
func TestDefenceCostRisesWithLoad(t *testing.T) {
	light := New()
	light.Stats.Strength.Base = 100
	light.Stats.Strength.Recalculate()
	light.Validate()

	heavy := New()
	heavy.Stats.Strength.Base = 100
	heavy.Stats.Strength.Recalculate()
	heavy.Validate()
	// Put the heavy character at the knee by giving it a carried weight the
	// test can control; see GetCarriedWeight for what contributes.
	heavy.testSetCarriedWeightForCost(heavy.CarryCapacity() * 0.75)

	if heavy.GetDefenseCostFloat(DefenseParry) <= light.GetDefenseCostFloat(DefenseParry) {
		t.Fatalf("a laden defender must pay more than an unladen one")
	}
}
```

If no test seam for carried weight exists, add one in a `_test.go` file in the package rather than production code, or equip the heavy character with real items via the existing item test helpers — read `internal/characters/inventory.go` and the package's existing tests and pick whichever the package already does. Say in your report which you used.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/characters/ -run TestDefenceCost -v`
Expected: FAIL — `GetDefenseCostFloat undefined`.

- [ ] **Step 3: Implement**

In `internal/characters/resources.go`, add the float-returning entry point and make the int one delegate:

```go
// GetDefenseCostFloat prices a defence through the unified cost model. Callers
// that charge should use this with ApplyCostFloat so the sub-integer part is
// carried rather than lost -- the three defence modifiers differ by 14%, which
// per-action rounding erases entirely.
func (c *Character) GetDefenseCostFloat(defenseType string) float64 {
	bal := configs.GetBalanceConfig()

	var action costs.Action
	var modifier float64
	switch defenseType {
	case DefenseDodge:
		action, modifier = costs.ActionDodge, float64(bal.DodgeCostModifier)
	case DefenseParry:
		action, modifier = costs.ActionParry, float64(bal.ParryCostModifier)
	case DefenseBlock:
		action, modifier = costs.ActionBlock, float64(bal.BlockCostModifier)
	case DefenseQuell:
		return float64(atLeastOneCost(int(bal.QuellBaseConvictionCost)))
	case DefenseDefy:
		return float64(atLeastOneCost(int(bal.DefyBaseConvictionCost)))
	default:
		return 0
	}

	spec := costs.SpecFor(action)
	return costs.Calc(costs.Input{
		Base:      float64(bal.DefenceBaseStaminaCost),
		Carried:   c.GetCarriedWeight(),
		Capacity:  c.CarryCapacity(),
		Physical:  spec.Physical,
		SkillRank: c.GetSkillLevel(spec.Skill),
		HasSkill:  spec.HasSkill,
		Modifier:  modifier,
	})
}
```

Keep `GetDefenseCost` as the integer entry point for callers that have not moved, implemented as `int(math.Ceil(c.GetDefenseCostFloat(defenseType)))` with a comment saying which callers still use it and that they should migrate.

Quell and defy keep their flat U6 costs: converting them to a fraction of the incoming action's cost needs a signature change through seven call sites and neither defence has been observed in live play. That is explicitly deferred (see the prebuild findings, §5).

- [ ] **Step 4: Charge through the carry at the melee site**

In `internal/combat/combat_helpers.go`, change the charge at the end of `runBestOfAllDefense` from `ApplyCostPartial(..., GetDefenseCost(...))` to:

```go
	if best.defenseType != "" {
		_ = targetChar.ApplyCostFloat(
			characters.DefensePool(best.defenseType),
			targetChar.GetDefenseCostFloat(best.defenseType))
	}
```

- [ ] **Step 5: Retune the knobs**

Add to `internal/configs/config.balance.go` and default in the combat sibling: `DefenceBaseStaminaCost` 1.0, `DodgeCostModifier` 1.25, `ParryCostModifier` 1.10, `BlockCostModifier` 1.15. The old `DodgeBaseStaminaCost`/`ParryBaseStaminaCost`/`BlockBaseStaminaCost` and `DodgeMultiplier`/`ParryMultiplier`/`BlockMultiplier` are now unread — delete the fields and their `config.yaml` entries in the same commit, and let the compiler prove nothing else reads them.

- [ ] **Step 6: Run the tests**

Run: `go test -count=1 ./internal/characters/ ./internal/combat/ ./internal/hooks/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/characters/ internal/combat/combat_helpers.go internal/configs/ _datafiles/config.yaml
git commit -m "feat(combat): defence costs run through the unified formula

Dodge, parry and block are priced by base x encumbrance x inverse skill x
per-action modifier and charged through the fractional carry. Dodge is
deliberately dearest: moving the whole body is more tiring than
interposing a weapon. The old per-defence bases and their 0.9 multipliers
are deleted rather than left unread.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Attack charged per swing

**Files:**
- Modify: `internal/combat/combat.go` (swing loop and the four wrapper charge sites)
- Modify: `internal/characters/resources.go` (`GetAttackStaminaCost`, `DeductAttackStamina`)
- Test: `internal/combat/attack_cost_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/combat/attack_cost_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// Attack cost scales with the number of swings thrown. Before U7 it was a
// single flat charge per round however many weapons and swings resolved, so a
// twelve-swing build attacked twelve times for the price of one.
func TestAttackCostScalesWithSwingCount(t *testing.T) {
	spend := func(dex int) int {
		c := characters.New()
		c.Stats.Dexterity.Base = dex
		c.Stats.Strength.Base = 100
		c.Stats.Dexterity.Recalculate()
		c.Stats.Strength.Recalculate()
		c.Validate()
		c.StaminaMax.Value = 1000
		c.Stamina = 1000

		before := c.Stamina
		ChargeAttackCost(c, 4) // four swings
		four := before - c.Stamina

		c.Stamina = 1000
		before = c.Stamina
		ChargeAttackCost(c, 1) // one swing
		one := before - c.Stamina

		if four <= one {
			t.Fatalf("four swings (%d) must cost more than one (%d)", four, one)
		}
		return four
	}
	spend(100)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/combat/ -run TestAttackCostScalesWithSwingCount -v`
Expected: FAIL — `undefined: ChargeAttackCost`.

- [ ] **Step 3: Implement**

Add to `internal/combat/combat.go`:

```go
// ChargeAttackCost charges an attacker for the swings they actually threw.
//
// Before U7 this was one flat DeductAttackStamina() per round regardless of
// weapon count or swing count, so a twelve-swing build paid the same as a
// one-swing build and offence was effectively free relative to defence. Cost
// now scales with swings, which is what makes the defender's per-swing bill
// proportionate.
//
// The per-weapon `staminacost` item field is deliberately not consulted. It was
// authored against a per-round charge, so multiplying it by a swing count
// inflates it by that count; and heavy weapons already cost more through their
// weight feeding the encumbrance multiplier, so reading it here would
// double-count.
func ChargeAttackCost(attacker *characters.Character, swings int) {
	if attacker == nil || swings <= 0 {
		return
	}
	bal := configs.GetBalanceConfig()
	spec := costs.SpecFor(costs.ActionAttack)

	per := costs.Calc(costs.Input{
		Base:      float64(bal.AttackBaseStaminaCost),
		Carried:   attacker.GetCarriedWeight(),
		Capacity:  attacker.CarryCapacity(),
		Physical:  spec.Physical,
		SkillRank: attacker.GetCombatSkillLevel(),
		HasSkill:  spec.HasSkill,
		Modifier:  float64(bal.AttackCostModifier),
	})
	_ = attacker.ApplyCostFloat(characters.PoolStamina, per*float64(swings))
}
```

Add a `SwingsThrown int` field to `AttackResult` (in `internal/combat/attackresult.go`), incremented once per swing inside the swing loop in `calculateCombat`, so the wrappers know how many to charge for.

Then replace each of the four `DeductAttackStamina()` calls in the wrappers with:

```go
	ChargeAttackCost(user.Character, attackResult.SwingsThrown)
```

using the correct character for each quadrant (`user.Character`, `userAtk.Character`, `&mob.Character`, `&mobAtk.Character`).

Finally, mark `DeductAttackStamina` and `GetAttackStaminaCost` deprecated with a comment pointing at `ChargeAttackCost`, and leave them until the compiler shows no production callers, then delete them in the same commit.

- [ ] **Step 4: Add the knobs**

`AttackBaseStaminaCost` 1.0 and `AttackCostModifier` 1.0 in `internal/configs/config.balance.go` plus the combat sibling defaults.

- [ ] **Step 5: Run tests**

Run: `go test -count=1 ./internal/combat/ ./internal/characters/ ./internal/hooks/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/combat/ internal/characters/resources.go internal/configs/ _datafiles/config.yaml
git commit -m "feat(combat): attack charged per swing, not once per round

A twelve-swing build paid the same as a one-swing build, which is what
made offence effectively free next to a defender charged on every
incoming swing. Cost now scales with swings thrown. The per-weapon
staminacost field is dropped rather than rescaled: it was authored
against a per-round charge, and weapon weight already prices heavy
weapons through encumbrance.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Movement onto the shared curve

**Files:**
- Modify: `internal/characters/resources.go` (`GetMovementStaminaCost`)
- Test: `internal/characters/movement_cost_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/characters/movement_cost_test.go`:

```go
package characters

import "testing"

// Ordinary travel gets cheaper; travel near carry capacity gets dearer. The
// floor keeps every normal move at a whole stamina point.
func TestMovementCostFloorAndCurve(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 100
	c.Stats.Strength.Recalculate()
	c.Validate()

	normal := c.GetMovementStaminaCost(1.0)
	if normal < 1 {
		t.Fatalf("every move must cost at least 1, got %d", normal)
	}

	rough := c.GetMovementStaminaCost(2.0)
	if rough <= normal {
		t.Fatalf("rough terrain (%d) must cost more than normal (%d)", rough, normal)
	}
}
```

- [ ] **Step 2: Run test to verify it fails or passes trivially**

Run: `go test ./internal/characters/ -run TestMovementCostFloorAndCurve -v`
Expected: PASS today (the old curve also floors at 1), so this is a guard, not a red test. The behavioural change is proven by Step 4's assertion instead.

- [ ] **Step 3: Implement**

Rewrite the encumbrance section of `GetMovementStaminaCost` to use the shared curve and the new base, keeping terrain, the mutation speed modifier, the hidden multiplier and the max cap exactly as they are:

```go
	cost := float64(b.MovementBaseStaminaCost) * terrainMultiplier
	cost *= costs.EncumbranceMultiplier(c.GetCarriedWeight(), c.CarryCapacity())
	cost *= costs.SkillMultiplier(c.GetSkillLevel(skills.Search))
```

Delete the inline `1.0 + math.Min(overRatio*4.0, 4.0)` block it replaces. Keep the `MovementMaxStaminaCost` cap and replace the hardcoded minimum with `MovementCostFloor`.

- [ ] **Step 4: Add a test pinning the direction of the change**

```go
// A normally laden traveller pays less than they used to; the saving is what
// pays for the steep cost near capacity.
func TestOrdinaryTravelIsCheaperThanTheOldFlatTwo(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 100
	c.Stats.Strength.Recalculate()
	c.Validate()

	if got := c.GetMovementStaminaCost(1.0); got > 2 {
		t.Fatalf("ordinary travel should not cost more than the old flat 2, got %d", got)
	}
}
```

- [ ] **Step 5: Add the knobs and run**

`MovementBaseStaminaCost` 0.5 (replacing 2.0 in `config.yaml`) and `MovementCostFloor` 1.

Run: `go test -count=1 ./internal/characters/ ./internal/usercommands/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/resources.go internal/characters/movement_cost_test.go internal/configs/ _datafiles/config.yaml
git commit -m "feat(movement): movement adopts the shared encumbrance curve

One curve for every physical action. The base drops from 2.0 to 0.5 with
a floor of 1, so ordinary travel gets slightly cheaper while travel near
carry capacity gets markedly dearer, and the old flat-until-overloaded
curve is gone. Terrain stays its own separate multiplier.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Author the biome movement costs

Terrain is a multiplier in the formula but **no DOGMud biome sets it**: all 17 files under `_datafiles/world/dogmud/biomes/` omit `movementcost`, so every biome loads at the 1.0 default and the terrain half of the movement design does nothing. The 17 files under `_datafiles/world/default/biomes/` do carry values, but that directory is not loaded.

**Files:**
- Modify: every `.yaml` under `_datafiles/world/dogmud/biomes/`

- [ ] **Step 1: Read both sets**

Run: `ls _datafiles/world/dogmud/biomes/` and `grep -h "biomeid\|movementcost" _datafiles/world/default/biomes/*.yaml`
Record which DOGMud biome ids exist and what the default world assigns to the same or nearest-equivalent id.

- [ ] **Step 2: Add `movementcost` to each DOGMud biome**

Use the default world's values where the biome matches (road 0.5, city and house 0.7, mountains and swamp 2.0, cliffs 2.5, everything ordinary 1.0). For any DOGMud biome with no equivalent, pick from that same scale and say why in your report. Match each file's existing key ordering and comment style.

- [ ] **Step 3: Verify every file still parses and the values load**

Run: `python -c "import yaml,glob,sys; [yaml.safe_load(open(f)) for f in glob.glob('_datafiles/world/dogmud/biomes/*.yaml')]" && echo YAML_OK`
Expected: `YAML_OK`.

Then confirm the loader picks them up rather than trusting the file: add a temporary `t.Log` in a test, or grep `internal/rooms/biomes.go` for the load path and confirm the directory it reads is the one you edited.

- [ ] **Step 4: Commit**

```bash
git add _datafiles/world/dogmud/biomes/
git commit -m "content(biomes): author movement costs

Terrain has been a multiplier in the movement cost formula with no data
behind it: every DOGMud biome omitted movementcost and loaded at the 1.0
default, so every room in the world cost the same to walk into. Values
follow the scale the unused default-world biomes already use.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: Movement trains `search`, rarely

**Files:**
- Modify: `internal/usercommands/go.go`
- Test: `internal/usercommands/movement_search_test.go` (create)

⚠️ **The naive implementation is not merely slow, it is destructive.** `CheckSkillProgression` derives its decay from the skill's use count, and `TrackSkillUse` increments it. Recording a use on every move piles up tens of thousands of uses, exhausts the decay curve, and cuts forage-based search training to a fraction of its normal rate. **Gate whether the use is recorded at all.**

- [ ] **Step 1: Write the failing test**

Create `internal/usercommands/movement_search_test.go`:

```go
package usercommands

import "testing"

// Movement trains search rarely. The gate decides whether a use is RECORDED,
// which keeps the use counter honest -- scaling the odds on a use that is still
// counted every step would exhaust the decay curve and poison forage training.
func TestMovementSearchTrainingIsGatedNotScaled(t *testing.T) {
	hits := 0
	const trials = 100000
	for i := 0; i < trials; i++ {
		if movementTrainsSearch() {
			hits++
		}
	}
	rate := float64(hits) / float64(trials)
	if rate > 0.02 || rate < 0.001 {
		t.Fatalf("gate rate %.4f is outside the intended rare band", rate)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/usercommands/ -run TestMovementSearchTraining -v`
Expected: FAIL — `undefined: movementTrainsSearch`.

- [ ] **Step 3: Implement**

In `internal/usercommands/go.go`:

```go
// movementTrainsSearch reports whether this move should record a search use.
//
// The rarity is in whether the use is RECORDED, not in the odds attached to it.
// CheckSkillProgression derives its decay from the skill's use count, so
// recording a use every step would bury the counter and leave forage, search
// and track training almost worthless. Walking should be an occasional
// bonus toward an eye for the road, never the fastest way to train the skill.
func movementTrainsSearch() bool {
	chance := float64(configs.GetBalanceConfig().MovementSearchTrainChance)
	if chance <= 0 {
		return false
	}
	return util.Rand(10000) < int(chance*10000)
}
```

At the point in `Go` where a move has succeeded and stamina has been charged, add:

```go
	if movementTrainsSearch() {
		user.Character.OnSkillUse(string(skills.Search), user.UserId)
	}
```

Read the surrounding code for the exact success point and the right `util.Rand` idiom before writing; match what the file already does.

- [ ] **Step 4: Add the knob**

`MovementSearchTrainChance` 0.005 (a 1-in-200 gate) in `internal/configs/config.balance.go` plus the sibling default. Document in `config.yaml` that raising it toward 0.02 makes walking the dominant way to train search, and that the skill also drives hidden-creature detection, so the rate controls how quickly stealth degrades world-wide.

- [ ] **Step 5: Run tests**

Run: `go test -count=1 ./internal/usercommands/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/usercommands/go.go internal/usercommands/movement_search_test.go internal/configs/ _datafiles/config.yaml
git commit -m "feat(movement): walking rarely trains search

Movement is priced against search, so the discount needs a path that
travel itself can earn. The gate decides whether a use is recorded rather
than scaling the odds on a use counted every step: the decay curve is
driven by the use count, and recording one per move would bury it and
make forage-based training nearly worthless. Search also feeds
hidden-creature detection, so this slowly sharpens a well-travelled
character's eye.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: `EffectivePoolMax` and the stand-up lockout

A companion or enchantment holder should simply **have less**, not **pay more**. Today several percentage-of-max consumers read the raw max while the current value is reserve-clamped, so they are taxed twice. The `stand` gate is the worst: at high stamina reservation it becomes unreachable and the player can never stand, and the message blames exhaustion, which resting cannot fix.

**Files:**
- Modify: `internal/characters/resources.go` (add `EffectivePoolMax`)
- Modify: `internal/usercommands/stand.go`
- Test: `internal/characters/effective_pool_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package characters

import "testing"

// A reserved pool's percentage-of-max thresholds must measure the pool the
// character can actually reach, or a heavily reserved character faces a gate
// they can never satisfy.
func TestEffectivePoolMaxExcludesReservation(t *testing.T) {
	c := New()
	c.Validate()
	c.StaminaMax.Value = 100

	full := c.EffectivePoolMax(PoolStamina)
	if full != 100 {
		t.Fatalf("with no reservation, effective max should equal max, got %d", full)
	}
}
```

Extend it with a reserved case using whatever seam the package already has for `GetPoolReservation` (read `internal/characters/validate.go` and the existing reservation tests first; if the only source is equipped items, equip one in the test).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/characters/ -run TestEffectivePoolMax -v`
Expected: FAIL — `EffectivePoolMax undefined`.

- [ ] **Step 3: Implement**

```go
// EffectivePoolMax returns the maximum the character can actually reach, with
// reservation excluded.
//
// Use it for every percentage-OF-MAX threshold. Do NOT use it for
// affordability: RecalculateStats already clamps the CURRENT pool to
// max - reserve every round, so a cost that subtracted the reservation again
// would charge the reserve twice.
func (c *Character) EffectivePoolMax(pool Pool) int {
	max := c.PoolMax(pool)
	res := c.GetPoolReservation(string(pool), max)
	if eff := max - res; eff > 0 {
		return eff
	}
	return 0
}
```

Check the real name of the max accessor (`PoolMax` or equivalent) in `pools.go` before writing.

- [ ] **Step 4: Fix the stand gate**

In `internal/usercommands/stand.go`, change both computations to measure the reachable pool:

```go
	effMax := user.Character.EffectivePoolMax(characters.PoolStamina)
	staminaCost := int(float64(effMax) * float64(cfg.StandStaminaCost))
	minStamina := int(float64(effMax) * float64(cfg.StandMinStamina))
```

and change the refusal message so it names reservation when that is the cause rather than blaming exhaustion. Follow `internal/usercommands/assess.go`, which already discloses reservation in descriptive bands with no raw numbers.

- [ ] **Step 5: Run tests**

Run: `go test -count=1 ./internal/characters/ ./internal/usercommands/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/ internal/usercommands/stand.go
git commit -m "fix(characters): percentage-of-max thresholds measure the reachable pool

A companion or enchantment holder should have less, not pay more. Both
stand-up thresholds were computed from the raw maximum and then tested
against a current value already clamped to max minus reservation, so a
heavily reserved character faced a gate they could never satisfy and was
told they were too exhausted, which resting could not fix.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: Config, documentation and the arc's paperwork

**Files:**
- Modify: `_datafiles/config.yaml`
- Modify: `internal/combat/context.md`, `internal/characters/context.md`
- Create: `internal/costs/context.md` (if Task 5 did not)
- Modify: `docs/PATCH_NOTES.md`
- Modify: `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`

- [ ] **Step 1: Document every knob in `config.yaml` itself**

Each new knob gets a comment above it saying what it does and what changing it costs. The arc's standing rule is that a knob is documented where its value lives, not only in a design document.

- [ ] **Step 2: Update the package docs**

`internal/combat/context.md` must record that `calculateCombat` takes pointers and why, and that attack cost is per swing. `internal/characters/context.md` must record `ApplyCostFloat` and its carry, `EffectivePoolMax` and the rule that costs never use it. Verify every symbol you name exists.

- [ ] **Step 3: Add the patch note**

Player-facing framing, no raw numbers, no em dashes. Cover: defending now costs stamina, attacking costs more the more blows you throw, carrying a heavy load makes both dearer, skill makes them cheaper, and ordinary travel is slightly easier while travelling near your limit is much harder.

- [ ] **Step 4: Add a U7 section to the crib sheet**

Add checks to `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md` section 10: a long fight watching stamina, a four-mob room in Ironwind Steppe (3078 Wolf Den Approach is the harshest in the world), a heavily laden character defending, a well-skilled character noticing the discount, and travel at ordinary versus near-capacity load.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/config.yaml internal/*/context.md docs/
git commit -m "docs(u7): knob documentation, package docs and the playtest checks

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: Verify, boot, playtest

- [ ] **Step 1: Formatting, build, full tests**

```bash
gofmt -l internal/ modules/     # must print nothing
go build ./...
go test -count=1 ./...
```
Known pre-existing failure, not yours: `internal/relationships` is quarantined by Windows Defender.

- [ ] **Step 2: Boot test in an isolated worktree**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
# set TelnetPort [43333], LocalPort 19999, HttpPort 18090, AIPort 15555, LogToFile false
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1   # exit 124 is SUCCESS
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

- [ ] **Step 3: Adversarial playtest**

The content gate. Author a goals file focused on stamina: a long duel, a four-mob room, a heavily laden defender, and travel at both ordinary and near-capacity load. Drive it with `/playtest local --checkout <abs> bug-finder <goals>.yaml`. **Re-kit the playtest profile first** — the shipped profiles carry three to five pounds, which sits at the bottom of the encumbrance curve and would report the whole model as free.

- [ ] **Step 4: Open the PR**

```bash
git push -u origin feature/u7-unified-cost-model
gh pr create --repo pruuk/DOGMud --base master --head feature/u7-unified-cost-model --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```
A green check is not proof; confirm with `gh run view <id> --repo pruuk/DOGMud --log-failed`.

---

## Deliberately not in this slice

- **The skill-strip on insufficient resource.** U8 owns it, and it must not ship before the reservation ceiling (below) or a zero-reserved pool becomes a permanent triple-digit defence penalty.
- **Ranged, taunt, rally, warcry, the thirteen special moves, grapple initiation, sneak.** They stay free. The registry in Task 5 is what they wire into, so each becomes an entry plus a config base.
- **Quell and defy as a fraction of the incoming action's cost.** Needs a signature change through seven call sites, and neither defence has been observed in live play. Revisit after the Elemental Queen playtest.
- **The reservation ceiling** (cap total reservation at 50-75% of a pool, refuse the breaching wield/enchant/summon). Arc-scoped, slice to be decided, and it must precede U8.
- **Reserve-aware regeneration.** Deliberately left reading the raw max; changing it is a nerf to reserved characters and it currently offsets a penalty they already pay.
