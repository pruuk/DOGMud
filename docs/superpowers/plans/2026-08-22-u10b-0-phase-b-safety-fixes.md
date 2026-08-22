# U10b-0 Phase B: Safety Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix two live production bugs that do not depend on the U10b-0 re-key, so they ship even if the rest of the arc stalls.

**Architecture:** Two independent changes. First, progression rolls move from
1-in-10,000 to 1-in-1,000,000 resolution and gain a configurable chance floor at
the two rank-driven roll sites, so a stat can become asymptotically slow but
never sealed. Second, `OnRegenTick` derives its own effective pool from a
`characters.Pool` rather than trusting a caller-supplied `max`, so the
reserved-pool progression faucet cannot be reopened by a future caller.

**Tech Stack:** Go (`internal/characters`, `internal/configs`, `internal/hooks`),
plus one new knob in `_datafiles/config.yaml`.

**Spec:** `docs/superpowers/specs/2026-08-21-u10b-0-progression-rank-from-training-design.md`
sections **13.1** (fix the truncation, do not relocate it), **14.3** (regen must
measure the reachable pool), and **14.5** (the floor does NOT apply to the regen
faucet). **Phase index:** `2026-08-21-u10b-0-README.md`.

**Branch:** `feature/u10b-0-phase-b-safety-fixes`, cut from `master`
(at or after `1c5d10fd7`, the Phase A merge).

---

## Why this phase exists, in one table

Verified 2026-08-22 against `_datafiles/world/dogmud/users/3.yaml` (Meirok) and
the shipped `config.yaml`, by evaluating the production expression directly:

| stat | uses | virtual rank | chance | `int(chance*10000)` | |
|---|---|---|---|---|---|
| strength | 11,671 | 466 | 3.978e-05 | **0** | **dead** |
| dexterity | 39,772 | 1,590 | 9.249e-12 | **0** | **dead** |
| charisma | 3,754 | 150 | 2.957e-03 | 29 | alive, ~6,300 uses from the same cliff |
| perception | 2,679 | 152 | 1.309e-02 | 130 | alive |
| willpower | 3,523 | 140 | 1.642e-02 | 164 | alive |

Two of a live character's six stats can never progress again, and the
`config.yaml:1006` comment about dexterity being "the slowest stat even at 0.10"
is this bug being mistaken for a tuning problem.

**Resolution alone is not enough.** At 1e6 the death point moves rather than
disappearing (spec 13.1 measures it at `Training 247`). The floor is what makes
the guarantee real.

---

## Standing rules

1. **No absolute line numbers for code an earlier task shifts.** Locate with
   `grep` at execution time, and verify the grep matched the symbol you meant.
2. **Safety defaults use `<= 0`; only genuine off-switches use `< 0`.** This is
   not a style preference. `ObservedCritProgressionBonus` uses `< 0` and is
   therefore sitting at 0 on any config that omits it — which is exactly what
   the owner's local `config.yaml` does today. `ProgressionChanceFloor` is a
   safety default, so it uses `<= 0`.
3. **Go defaults move with shipped values.** A test binary never loads
   `config.yaml`, so a knob added to one and not the other produces a test suite
   validating a curve production never runs.
4. **`grep --include=*.go` cannot see `config.yaml` or `AGENTS.md`.** Every
   prior round of this arc was bitten by this.
5. **Nothing in this phase changes a progression *rate*.** It changes what
   happens at the bottom of the curve, where the rate is currently exactly zero.
   Any change to a chance that is not already truncating to zero is a bug in
   this phase.

---

## File Structure

**Modified:**
- `internal/configs/config.balance.go` — the `ProgressionChanceFloor` field
- `internal/configs/config.balance.progression.go` — its default and validation
- `_datafiles/config.yaml` — the shipped value
- `internal/characters/progression.go` — three roll sites, one floor helper,
  and `OnRegenTick`'s signature
- `internal/hooks/NewRound_AutoHeal.go` — six `OnRegenTick` call sites lose
  their hand-rolled max arithmetic
- `internal/characters/context.md` — the progression seam's contract

**Created:**
- `internal/characters/progression_floor_test.go` — the truncation and floor gates
- `internal/characters/progression_regen_pool_test.go` — the reserved-pool gate

---

## Task B1: `ProgressionChanceFloor`, the knob

**Files:** Modify `internal/configs/config.balance.go`,
`internal/configs/config.balance.progression.go`. Test: modify
`internal/configs/config.balance_test.go`.

- [ ] **Step 1: Write the failing test**

Find the existing balance-validation test and its helper:

```bash
grep -n "func Test" internal/configs/config.balance_test.go | head -20
grep -n "setBalanceForTest" internal/configs/config.balance_test.go | head -3
```

Append to `internal/configs/config.balance_test.go`:

```go
// ProgressionChanceFloor is a SAFETY default, so its validator uses `<= 0`:
// a config that omits the key, or sets it to 0, gets the default back. The
// `< 0` idiom used by the deliberate off-switches (CritProgressionBonus,
// ObservedCritProgressionBonus) would leave an omitted key at zero, which is
// precisely the failure this knob exists to prevent — a chance that can reach
// zero and seal a stat forever.
func TestProgressionChanceFloor_AbsentOrZeroGetsTheDefault(t *testing.T) {
	for _, in := range []float64{0, -1, -0.5} {
		b := &Balance{ProgressionChanceFloor: ConfigFloat(in)}
		b.Validate()
		if float64(b.ProgressionChanceFloor) != 1e-5 {
			t.Errorf("ProgressionChanceFloor %v validated to %v, want 1e-05",
				in, float64(b.ProgressionChanceFloor))
		}
	}
}

func TestProgressionChanceFloor_ExplicitValueSurvives(t *testing.T) {
	b := &Balance{ProgressionChanceFloor: ConfigFloat(2e-4)}
	b.Validate()
	if float64(b.ProgressionChanceFloor) != 2e-4 {
		t.Errorf("an explicit floor was overwritten: got %v, want 0.0002",
			float64(b.ProgressionChanceFloor))
	}
}
```

- [ ] **Step 2: Verify it fails**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
GOTMPDIR=C:/gotmp go test ./internal/configs/ -run TestProgressionChanceFloor -v
```

Expected: FAIL, `unknown field ProgressionChanceFloor in struct literal`.

- [ ] **Step 3: Add the field**

```bash
grep -n "ProgressionDecayAboveCap ConfigFloat" internal/configs/config.balance.go
```

Add immediately after that line:

```go
	// ProgressionChanceFloor is the smallest chance a rank-driven progression
	// roll may present. Below it, a stat or skill is not merely slow, it is
	// sealed: the roll quantises to zero and can never fire again. Two of a
	// live character's six stats were in that state before this knob existed.
	//
	// Applied ONLY to the two rank-driven sites (CheckStatProgression,
	// CheckSkillProgression). CheckRegenProgression is deliberately excluded:
	// its chance is proportional to pool depletion and is SUPPOSED to vanish as
	// the pool fills (spec 14.5).
	ProgressionChanceFloor ConfigFloat `yaml:"ProgressionChanceFloor"` // Smallest chance a rank-driven progression roll may present (default 0.00001)
```

- [ ] **Step 4: Add the default**

```bash
grep -n "b.ObservedCritProgressionBonus = 0.5" internal/configs/config.balance.progression.go
```

Add after the closing brace of that `if` block:

```go
	// `<= 0`, not `< 0`: this is a safety floor, not an off-switch. A config
	// that omits the key must get the floor, not lose it — the `< 0` idiom two
	// lines above is why ObservedCritProgressionBonus sits at 0 in production.
	if b.ProgressionChanceFloor <= 0 {
		b.ProgressionChanceFloor = 1e-5
	}
```

- [ ] **Step 5: Verify it passes**

```bash
GOTMPDIR=C:/gotmp go test ./internal/configs/ -run TestProgressionChanceFloor -v
```

Expected: both tests PASS.

- [ ] **Step 6: Ship the value in config.yaml**

**`grep --include=*.go` cannot see this file.** Locate the anchor:

```bash
grep -n "ProgressionDecayAboveCap: 2.0" _datafiles/config.yaml
```

Insert immediately after that line:

```yaml
  # Smallest chance a stat or skill roll may present. Below this a stat is not
  # slow, it is SEALED: the roll quantises to zero and can never fire again.
  # Two of Meirok's six stats were in that state (strength 3.98e-05, dexterity
  # 9.25e-12) before this existed. Does NOT apply to regen progression, whose
  # chance is meant to vanish as the pool fills.
  ProgressionChanceFloor: 0.00001
```

**Note:** `_datafiles/config.yaml` carries the git skip-worktree bit, so
`git status` shows it clean and `git add` fails with a misleading
"sparse-checkout" error. Follow the procedure in
`reference_config_yaml_skip_worktree` memory: `git update-index
--no-skip-worktree`, stage only this hunk, commit, then re-set the bit. Do NOT
commit the local `HttpPort: 8090` / `LogLevel` / `LogToFile` overrides.

- [ ] **Step 7: Commit**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.progression.go internal/configs/config.balance_test.go
git commit -m "feat(u10b-0): ProgressionChanceFloor, so a stat can never seal

Progression is designed to become asymptotically slow, never impossible. The
integer roll quantises the chance, so below a threshold it becomes exactly
impossible instead.

Safety default, so the validator uses <= 0 rather than the < 0 idiom the
deliberate off-switches use. A config that omits the key must get the floor
back, not lose it."
```

---

## Task B2: Raise roll resolution and apply the floor

The truncation lives at three sites. **Two get both changes; the third gets
resolution only.**

| function | resolution 1e4 -> 1e6 | floor |
|---|---|---|
| `CheckSkillProgression` | yes | **yes** |
| `CheckStatProgression` | yes | **yes** |
| `CheckRegenProgression` | yes | **no** (spec 14.5) |

`CheckRegenProgression`'s chance is `RegenProgressionBase * (1-ratio)^curve`.
At 99% of pool that is around 1e-8 **by design** — the term is supposed to
vanish as the pool fills. Flooring it would lift it roughly 1000x and would also
fight the regen damper that Phase D has to tune. Give it the finer resolution
and nothing else.

**Files:** Modify `internal/characters/progression.go`. Test: create
`internal/characters/progression_floor_test.go`.

- [ ] **Step 1: Confirm the three sites and which function each is in**

```bash
grep -n "10000" internal/characters/progression.go
for L in $(grep -n "int(chance \* 10000)" internal/characters/progression.go | cut -d: -f1); do
  awk -v L=$L 'NR<=L && /^func /{f=$0} NR==L{print L": "f}' internal/characters/progression.go
done
```

Expected: exactly three, in `CheckSkillProgression`, `CheckStatProgression`,
`CheckRegenProgression`. **If there are more or fewer, stop and re-cut this
task** rather than editing what you find.

- [ ] **Step 2: Write the failing tests**

Create `internal/characters/progression_floor_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// progressionRollDenominator is the integer resolution the roll sites use. At
// 10,000 any chance below 0.01% truncates to a threshold of 0 and can never
// fire. Meirok's strength sits at 3.98e-05 and his dexterity at 9.25e-12.
const progressionRollDenominator = 1000000

// A chance far below the old 1-in-10,000 quantum must still produce a nonzero
// threshold. This is the arithmetic the two rank-driven roll sites perform.
func TestRollResolution_SmallChanceIsNotQuantisedAway(t *testing.T) {
	// The floor is what guarantees this, but the resolution is what makes the
	// floor expressible: 1e-5 x 10,000 = 0.1, which truncates to 0.
	const floor = 1e-5
	if got := int(floor * progressionRollDenominator); got == 0 {
		t.Fatalf("the shipped floor still quantises to zero at resolution %d",
			progressionRollDenominator)
	}
	if got := int(floor * 10000); got != 0 {
		t.Fatalf("sanity: the old resolution was supposed to quantise 1e-5 away, got %d", got)
	}
}

// The floor applies to the two rank-driven chances. A character whose virtual
// rank has run away must still have a live, if tiny, chance.
func TestStatProgressionChance_IsFloored(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "FlooredStat"
	// Meirok's real dexterity position: 39,772 uses at UsesPerRank 25.
	c.StatUseCount["dexterity"] = 39772

	got := c.statProgressionChance("dexterity", 1.0)
	floor := float64(configs.GetBalanceConfig().ProgressionChanceFloor)
	if got < floor {
		t.Errorf("statProgressionChance = %v, below the floor %v", got, floor)
	}
	if int(got*progressionRollDenominator) == 0 {
		t.Errorf("chance %v still quantises to a zero threshold", got)
	}
}

// The floor must not LIFT a chance that was already above it. This phase is
// only allowed to change what happens at the bottom of the curve.
func TestStatProgressionChance_HealthyChanceIsUntouched(t *testing.T) {
	withRepoRoot(t)

	fresh := New()
	fresh.Name = "FreshStat"
	fresh.StatUseCount["perception"] = 0

	got := fresh.statProgressionChance("perception", 1.0)
	floor := float64(configs.GetBalanceConfig().ProgressionChanceFloor)
	if got <= floor*10 {
		t.Fatalf("fixture is not above the floor; got %v, floor %v", got, floor)
	}

	// Recompute without the floor and confirm they agree.
	b := configs.GetBalanceConfig()
	rank := 0
	want := CalculateProgressionChance(rank, int(b.StatProgressionSoftCap)) *
		1.0 * 1.0 * b.GetStatProgressionMultiplier("perception") *
		float64(b.StatProgressionRate)
	if want > 1.0 {
		want = 1.0
	}
	if got != want {
		t.Errorf("floor altered a healthy chance: got %v, want %v", got, want)
	}
}

// Regen progression is deliberately excluded (spec 14.5). Its chance is
// proportional to depletion and is SUPPOSED to vanish as the pool fills; a
// floor there would lift a near-full-pool chance by roughly a thousand times
// and fight the regen damper Phase D has to tune.
func TestRegenProgression_IsNotFloored(t *testing.T) {
	withRepoRoot(t)

	b := configs.GetBalanceConfig()
	base := float64(b.RegenProgressionBase)
	curve := float64(b.RegenProgressionCurve)

	// A pool at 99% full.
	ratio := 0.99
	chance := base * pow(1.0-ratio, curve)
	floor := float64(b.ProgressionChanceFloor)
	if chance >= floor {
		t.Skipf("fixture no longer sits below the floor (chance %v, floor %v); "+
			"pick a fuller pool", chance, floor)
	}
	// The point of the test: production must NOT lift this to the floor.
	// CheckRegenProgression takes the chance as an argument and must roll it
	// as given, so this asserts the contract rather than a return value.
	if got := regenRollThresholdForTest(chance); got != int(chance*progressionRollDenominator) {
		t.Errorf("regen threshold %d does not match its unfloored chance %v", got, chance)
	}
}
```

That test file needs two small helpers. Add them to the same file:

```go
// pow avoids importing math into the test for one call.
func pow(x, y float64) float64 {
	r := 1.0
	for i := 0; i < int(y); i++ {
		r *= x
	}
	return r
}

// regenRollThresholdForTest mirrors the threshold arithmetic
// CheckRegenProgression performs, so the test pins production's expression
// rather than asserting on a side effect that needs a full character.
func regenRollThresholdForTest(chance float64) int {
	return int(chance * progressionRollDenominator)
}
```

- [ ] **Step 3: Verify they fail**

```bash
GOTMPDIR=C:/gotmp go test ./internal/characters/ -run "TestRollResolution|TestStatProgressionChance_IsFloored|TestStatProgressionChance_Healthy|TestRegenProgression_IsNotFloored" -v
```

Expected: `TestStatProgressionChance_IsFloored` FAILS (chance is
9.249e-12, far below the floor). The others should pass already — they are
guard rails, not the change.

- [ ] **Step 4: Apply the floor in `statProgressionChance`**

```bash
grep -n "func (c \*Character) statProgressionChance" internal/characters/progression.go
```

Find the clamp at the end of that function:

```go
	if chance > 1.0 {
		chance = 1.0
	}
	return chance
```

Replace with:

```go
	if chance > 1.0 {
		chance = 1.0
	}
	// Floor last, after every multiplier. Progression is meant to become
	// asymptotically slow, never impossible, and the integer roll quantises
	// anything smaller than one part in the denominator down to "never".
	// Applied here rather than at the roll site so every caller of this
	// expression — CheckStatProgression, OnCritReceived, the faucet test —
	// sees the same guarantee.
	if floor := float64(b.ProgressionChanceFloor); chance > 0 && chance < floor {
		chance = floor
	}
	return chance
```

**`chance > 0` matters.** The mob gates above return a hard `0` for "this mob
may not progress at all", and the floor must not resurrect that.

- [ ] **Step 5: Apply the floor in the skill path**

```bash
grep -n "func (c \*Character) CheckSkillProgression" internal/characters/progression.go
```

Find, inside it:

```go
	if chance > 1.0 {
		chance = 1.0
	}
```

Replace with:

```go
	if chance > 1.0 {
		chance = 1.0
	}
	// Same guarantee as the stat path: a skill may become vanishingly slow but
	// never sealed. `chance > 0` preserves the mob hard-cap short-circuits
	// above, which return a genuine zero.
	if floor := float64(b.ProgressionChanceFloor); chance > 0 && chance < floor {
		chance = floor
	}
```

- [ ] **Step 6: Raise the resolution at all three sites**

Each site currently reads:

```go
	threshold := int(chance * 10000)
	roll := util.Rand(10000)
```

Replace **all three** with:

```go
	threshold := int(chance * progressionRollDenominator)
	roll := util.Rand(progressionRollDenominator)
```

and add the constant near the top of `progression.go`, after the imports:

```go
// progressionRollDenominator is the integer resolution every progression roll
// uses. It was 10,000, which quantised any chance below 0.01% to a threshold of
// zero — a stat in that state could never progress again, and two of a live
// character's six were there. util.Rand is already integer, so the finer
// resolution costs nothing.
//
// Resolution alone does not fix the seal, it only moves it; ProgressionChanceFloor
// is what removes it. Both are needed: at 10,000 the shipped floor of 1e-5
// would itself quantise to zero.
const progressionRollDenominator = 1000000
```

**The test file declares a constant of the same name.** Delete the one in
`progression_floor_test.go` once production exports it, or the package will not
compile.

- [ ] **Step 7: Verify**

```bash
gofmt -l internal/
GOTMPDIR=C:/gotmp go test ./internal/characters/ -count=1
```

Expected: all pass, including the pre-existing `progression_test.go`,
`progression_apply_test.go` and `progression_faucet_test.go`. **If the faucet
test fails, read it before touching it** — it calls `statProgressionChance`
directly precisely so it pins production's expression, so a failure there means
the floor changed a chance it should not have.

- [ ] **Step 8: Re-verify the real character**

The whole justification for this task is a specific claim about a specific save.
Confirm it is now false:

```bash
python C:/tmp/truncheck.py
```

If that scratch script is gone, the check is: strength and dexterity must now
report a nonzero threshold. Record the new numbers in the commit message.

- [ ] **Step 9: Commit**

```bash
git add internal/characters/progression.go internal/characters/progression_floor_test.go
git commit -m "fix(u10b-0): a stat can no longer be sealed by roll quantisation

progression.go rolled int(chance * 10000), so any chance below 0.01% became a
threshold of exactly 0 and could never fire. Verified against users/3.yaml with
the shipped config: strength 3.98e-05 and dexterity 9.25e-12 both truncate to 0.
Two of that character's six stats have been unable to progress at all, and the
config.yaml comment about dexterity being the slowest stat records the bug being
mistaken for a tuning problem.

Two halves, because neither is sufficient alone. Resolution moves to 1,000,000
(util.Rand is already integer, so this costs nothing) and ProgressionChanceFloor
is applied after every multiplier. At the old resolution the floor itself would
have quantised to zero; with resolution alone the seal moves rather than lifting.

Scoped to the two rank-driven sites. CheckRegenProgression gets the finer
resolution but NO floor: its chance is proportional to pool depletion and is
meant to vanish as the pool fills, so flooring it would lift a near-full-pool
chance by about a thousand times and fight the regen damper (spec 14.5)."
```

---

## Task B3: `OnRegenTick` derives its own effective pool

`OnRegenTick(current, max int, ...)` trusts its `max` argument. Six call sites
compute it by hand as `Max.Value - GetPoolReservation(...)`, and **no test pins
that**. The arithmetic is correct today; the exposure is structural. Once Phase C
makes companions mirror players, a reserved pet would farm it too.

**The trap, and it is a real one.** `EffectivePoolMax` is **floored at 1 and
never returns 0** (`pools.go:313-318`). The current call sites yield `max <= 0`
at total reservation, and `OnRegenTick` returns early on that. Swapping in
`EffectivePoolMax` naively gives `max = 1`, `current = 0`, `ratio = 0` — the
*maximum* progression chance, forever. The fully-reserved case must be handled
explicitly.

**Second trap, from `EffectivePoolMax`'s own doc comment:** "Regeneration is a
deliberate exception and still reads the raw max." That refers to the regen
**amount** (`HealthPerRound` / `StaminaPerRound` / `ConvictionPerRound`) and
stays correct. Only the progression **ratio** moves. Do not touch the amount.

**Files:** Modify `internal/characters/progression.go`,
`internal/hooks/NewRound_AutoHeal.go`. Test: create
`internal/characters/progression_regen_pool_test.go`.

- [ ] **Step 1: Write the failing test**

```go
package characters

import (
	"testing"
)

// OnRegenTick must derive its own effective pool. A caller cannot be trusted to
// subtract the reservation: six call sites do it by hand today, correctly, and
// nothing pins that. The fyttyn vitality exploit (2026-04-16) was exactly this
// faucet — a character at their reserved cap, unable to heal higher, counting as
// permanently "depleted".
//
// These tests assert on the chance OnRegenTick would roll, via the same helper
// production uses, rather than on a stat actually gaining: a progression roll is
// probabilistic and a test that waits for one is flaky by construction.
func TestRegenTickChance_FullEffectivePoolWithLargeReservationIsZero(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "Reserved"
	c.HealthMax.Base = 400
	c.HealthMax.Recalculate()

	// Half the pool reserved; the character is at the top of what they can reach.
	setPoolReservationForTest(t, c, "health", 200)
	c.Health = 200

	if got := c.regenTickChance(PoolHealth); got != 0 {
		t.Errorf("a character at their effective cap has regen progression chance %v, want 0", got)
	}
}

// The complement: genuinely depleted must still progress.
func TestRegenTickChance_DepletedPoolStillProgresses(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "Hurt"
	c.HealthMax.Base = 400
	c.HealthMax.Recalculate()
	setPoolReservationForTest(t, c, "health", 200)
	c.Health = 20 // deep into the reachable 200

	if got := c.regenTickChance(PoolHealth); got <= 0 {
		t.Errorf("a genuinely depleted character has chance %v, want > 0", got)
	}
}

// The trap this task exists to avoid. EffectivePoolMax is floored at 1 and never
// returns 0, so at total reservation a naive implementation sees max=1,
// current=0, ratio=0 — the MAXIMUM chance, forever. It must be zero.
func TestRegenTickChance_TotallyReservedPoolIsZeroNotMaximum(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "FullyReserved"
	c.HealthMax.Base = 400
	c.HealthMax.Recalculate()
	setPoolReservationForTest(t, c, "health", 400) // the entire pool
	c.Health = 0

	if got := c.regenTickChance(PoolHealth); got != 0 {
		t.Errorf("a fully reserved pool has chance %v, want 0 "+
			"(EffectivePoolMax floors at 1, which reads as ratio 0 = maximum chance)", got)
	}
}
```

The reservation helper depends on how reservations are stored. Find it:

```bash
grep -n "func (c \*Character) GetPoolReservation" -A 25 internal/characters/validate.go
```

Write `setPoolReservationForTest` in the test file to populate whatever that
function reads — if it sums equipment `ReservePool` fields, equip a fixture item;
if it reads a map, set the map. **Do not add a production setter for this.**

- [ ] **Step 2: Verify it fails**

```bash
GOTMPDIR=C:/gotmp go test ./internal/characters/ -run TestRegenTickChance -v
```

Expected: FAIL, `c.regenTickChance undefined`.

- [ ] **Step 3: Extract the chance, then change the signature**

In `internal/characters/progression.go`, replace `OnRegenTick`'s body with a
chance helper plus a thin driver:

```go
// regenTickChance is the progression chance one regen tick presents for a pool,
// or 0 when no roll should happen. Extracted from OnRegenTick so it can be
// tested without waiting on a probabilistic roll, and so the reserved-pool rule
// lives in exactly one place.
//
// The ratio measures the pool the character can actually REACH — raw max minus
// reservation — because a character sitting at their reserved cap is not
// depleted, they are full. Reading the raw max there turns a large reservation
// into a permanent progression farm (the fyttyn vitality exploit, 2026-04-16).
//
// Note this is the progression RATIO only. The regen AMOUNT deliberately keeps
// the raw max; see EffectivePoolMax's doc comment.
func (c *Character) regenTickChance(p Pool) float64 {
	if !configs.GetGamePlayConfig().UseSkillProgression {
		return 0
	}

	rawMax := c.poolMax(p)
	if rawMax <= 0 {
		return 0
	}
	// Deliberately NOT EffectivePoolMax: that floors at 1 and never returns 0,
	// so a fully reserved pool would present max=1, current=0, ratio=0 — the
	// maximum chance, permanently. A pool with nothing reachable in it offers
	// no progression at all.
	effMax := rawMax - c.GetPoolReservation(string(p), rawMax)
	if effMax <= 0 {
		return 0
	}

	ratio := float64(c.PoolValue(p)) / float64(effMax)
	if ratio >= 1.0 {
		return 0 // at the top of what they can reach: not depleted
	}
	if ratio < 0 {
		ratio = 0
	}

	b := configs.GetBalanceConfig()
	chance := float64(b.RegenProgressionBase) *
		math.Pow(1.0-ratio, float64(b.RegenProgressionCurve))
	if chance <= 0 {
		return 0
	}
	return chance
}

// OnRegenTick is called every regen tick (every 3 rounds) for each resource
// pool, and rolls regen-driven stat progression for the stats that pool
// exercises.
//
// It takes a Pool rather than a (current, max) pair on purpose: six call sites
// used to compute the reservation-adjusted max by hand, correctly but with
// nothing pinning it, and the next caller to get it wrong reopens a progression
// faucet. There is no longer a max to pass.
func (c *Character) OnRegenTick(p Pool, relatedStats []string, userId int) {
	chance := c.regenTickChance(p)
	if chance <= 0 {
		return
	}
	for _, statName := range relatedStats {
		c.CheckRegenProgression(statName, userId, chance)
	}
}
```

**Preserve the existing comment block** inside the `relatedStats` loop (the one
explaining that `CheckRegenProgression` rolls but never records the use) — copy
it across rather than dropping it.

Confirm `math` is already imported:

```bash
grep -n '"math"' internal/characters/progression.go
```

- [ ] **Step 4: Verify the tests pass**

```bash
GOTMPDIR=C:/gotmp go test ./internal/characters/ -run TestRegenTickChance -v
```

Expected: all three PASS.

- [ ] **Step 5: Update the six call sites**

```bash
grep -n "OnRegenTick" internal/hooks/NewRound_AutoHeal.go
```

On the player side, the three hand-rolled max locals
(`healthMax`, `staminaMax`, `convictionMax`) become dead. Delete them **only if
nothing else uses them** — check first:

```bash
grep -n "healthMax\|staminaMax\|convictionMax" internal/hooks/NewRound_AutoHeal.go
```

Then:

```go
		user.Character.OnRegenTick(characters.PoolHealth,
			[]string{"vitality", "willpower"}, user.UserId)
		user.Character.OnRegenTick(characters.PoolStamina,
			[]string{"strength", "vitality"}, user.UserId)
		user.Character.OnRegenTick(characters.PoolConviction,
			[]string{"willpower", "charisma"}, user.UserId)
```

And the mob side, dropping `mobHealthMax` / `mobStaminaMax` / `mobConvictionMax`
under the same check:

```go
		mob.Character.OnRegenTick(characters.PoolHealth,
			[]string{"vitality", "willpower"}, 0)
		mob.Character.OnRegenTick(characters.PoolStamina,
			[]string{"strength", "vitality"}, 0)
		mob.Character.OnRegenTick(characters.PoolConviction,
			[]string{"willpower", "charisma"}, 0)
```

Replace the now-stale comments above each block. The old ones explain the manual
subtraction that no longer happens there; say instead that `OnRegenTick` derives
the reachable pool itself, and keep the fyttyn reference so the reason survives.

- [ ] **Step 6: Verify and commit**

```bash
gofmt -l internal/ modules/
GOTMPDIR=C:/gotmp go build ./... && GOTMPDIR=C:/gotmp go test ./... -count=1 2>&1 | grep -E "^(FAIL|--- FAIL)"
```

Expected: no output. Note `internal/usercommands` is independently flaky on
master (`TestTaunt_StalePlayerIdInRoom_StillMessages`,
`TestShoot_SameRoomLoaded_DamageAndAggro`) — re-run before assuming either is
yours.

```bash
git add internal/characters/progression.go internal/characters/progression_regen_pool_test.go internal/hooks/NewRound_AutoHeal.go
git commit -m "refactor(u10b-0): OnRegenTick derives its own reachable pool

OnRegenTick took (current, max) and trusted the max. Six call sites computed it
by hand as Max.Value - GetPoolReservation, correctly, with nothing pinning it.
The arithmetic was right; the exposure was structural, and Phase C makes
companions mirror players, so a reserved pet would inherit it.

It now takes a Pool and derives both, so there is no max to get wrong.

Deliberately NOT via EffectivePoolMax, despite that being the obvious helper:
it floors at 1 and never returns 0, so a fully reserved pool would present
max=1, current=0, ratio=0 — the MAXIMUM progression chance, permanently. That
is the same faucet inverted. A pool with nothing reachable in it now yields
exactly zero, pinned by a test.

The regen AMOUNT still reads the raw max, which is deliberate and unchanged;
only the progression ratio moved."
```

---

## Task B4: Bring the arc's spec and phase index onto master

The spec, the phase index and the Phase A plan live only on
`feature/u10b-progression-firing`, which was never merged. The index carries the
corrections that phases C through F must apply before reusing the v3 task
bodies. Leaving that on an unmerged branch is a single point of failure.

**Files:** Copy from `feature/u10b-progression-firing` into `docs/superpowers/`.

- [ ] **Step 1: Confirm what is missing**

```bash
git log --oneline -1 -- docs/superpowers/specs/2026-08-21-u10b-0-progression-rank-from-training-design.md
git show feature/u10b-progression-firing --stat --oneline | head -20
```

- [ ] **Step 2: Bring the three documents across**

```bash
git checkout feature/u10b-progression-firing -- \
  docs/superpowers/specs/2026-08-21-u10b-0-progression-rank-from-training-design.md \
  docs/superpowers/plans/2026-08-21-u10b-0-README.md \
  docs/superpowers/plans/2026-08-21-u10b-0-phase-a-groundwork.md
```

- [ ] **Step 3: Mark Phase A done in the index**

In `docs/superpowers/plans/2026-08-21-u10b-0-README.md`, under
`## Phase A — Groundwork`, add a line recording that it shipped in PR #55
(merge `1c5d10fd7`), and that spec section 15 already covers the authored-
`training:` discovery so the "spec correction owed" item is closed.

- [ ] **Step 4: Index the new docs**

Per project convention, add the spec and both plans to `docs/README.md` if
`docs/superpowers/` content is indexed there:

```bash
grep -n "superpowers" docs/README.md | head
```

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/ docs/README.md
git commit -m "docs(u10b-0): bring the arc spec and phase index onto master

Both lived only on feature/u10b-progression-firing, which was never merged. The
index carries the per-phase corrections that C through F must apply to the v3
task bodies before reusing them, so keeping it off master was a single point of
failure for the rest of the arc."
```

---

## Task B5: Phase gates

- [ ] **Step 1: Formatting** — `gofmt -l internal/ modules/` must print nothing.

- [ ] **Step 2: Build and full suite**

```bash
GOTMPDIR=C:/gotmp go build ./... && GOTMPDIR=C:/gotmp go test ./... -count=1 2>&1 | grep -E "^(FAIL|--- FAIL)"
```

- [ ] **Step 3: Confirm the scope of the change**

This phase must not alter a chance that was not already truncating to zero.

```bash
git diff --name-only master...HEAD
```

Expected: only the files this plan lists. **No mob YAML, no rank site, no
`StatProgressionSoftCap` change** — those are Phase C.

- [ ] **Step 4: Patch notes**

Add a dated entry to `docs/PATCH_NOTES.md`, above the most recent heading.
Player-facing framing, no raw numbers, no em dashes, wrapped at 80 columns. The
honest player-side statement is that a few very well-practised abilities had
stopped improving entirely and can improve again.

- [ ] **Step 5: Boot test** in an isolated detached worktree:

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
mkdir -p C:/tmp/dogmud-boot-check/_datafiles/logs
cd C:/tmp/dogmud-boot-check && GOTMPDIR=C:/gotmp go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Exit code **124 is the success case**. Do not grep for the bare word `panic` —
`GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`. The
`mkdir` for `_datafiles/logs` is required: without it the server exits 1 before
reaching Server Ready. Clean up with `git worktree remove --force`, and if
Windows holds a lock, `rm -rf` then `git worktree prune`.

- [ ] **Step 6: Adversarial playtest gate**

This phase changes no content, but it does change a player-facing progression
guarantee, so the SOP gate applies. A `veteran` profile is the right instrument:
its skills are high enough to sit in the part of the curve this fixes.

```text
/playtest local --checkout <abs> bug-finder 2026-08-03-prepush-sweep.yaml
```

Drive skill and stat use heavily and watch for progression messages appearing at
all on a high-rank skill. Extract findings to memory afterwards — reports are
gitignored.

- [ ] **Step 7: Ship**

```bash
git push -u origin feature/u10b-0-phase-b-safety-fixes
gh pr create --repo pruuk/DOGMud --base master --head feature/u10b-0-phase-b-safety-fixes --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

`gh` defaults to the fork **parent** — `--repo pruuk/DOGMud` is mandatory on
every command. This phase touches far fewer than 300 files, so the lint gate's
`only-new-issues` will work normally (Phase A's red lint was a >300-file API
limit, not a code problem).

---

## Self-Review

**Spec coverage:** 13.1 (resolution + floor) → B1, B2 · 14.5 (floor excludes the
regen faucet) → B2 Step 6 and `TestRegenProgression_IsNotFloored` · 14.3
(`OnRegenTick` derives its own effective pool) → B3. The index's three
corrections are all applied: `PoolValue` is used rather than the non-existent
`GetPool`; `EffectivePoolMax`'s floor-at-1 is handled explicitly rather than
swapped in naively; and the fixture concern about the old value floor is moot
here because this phase does not move `StatProgressionSoftCap`.

**Not in this phase, deliberately:** every rank site, the spawn-pool move, the
caps, and all balance retuning. Phase B must not change a chance that was not
already zero.

**Known-weak points, stated rather than hidden:**
- B3 Step 1 leaves `setPoolReservationForTest` to be written against whatever
  `GetPoolReservation` actually reads. That is deliberate — inventing the
  fixture here without reading that function is how a plan ships an API that
  does not exist — but it is the one step that needs judgement rather than
  transcription.
- `TestRegenProgression_IsNotFloored` asserts on a mirrored expression rather
  than on production's return value, because `CheckRegenProgression` takes the
  chance as an argument and returns nothing. If Phase E extracts
  `ProgressionChanceForSkill`/`ForStat` as the spec suggests, this test should
  be re-pointed at the real function.
- The floor's shipped value (1e-5) is a *suggested* default from spec 13.1, not
  a solved one. Phase D owns pace; if 1e-5 turns out to be too generous a
  long-tail faucet, that is a Phase D retune, not a Phase B bug.
