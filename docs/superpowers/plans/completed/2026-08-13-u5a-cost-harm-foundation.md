# U5a — Cost and Harm Foundation: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **REVISION 2 (2026-08-13), after a three-reviewer adversarial pass.** Revision 1
> contained four factual claims that were false and would have shipped into
> permanent docstrings, plus docstrings that contradicted the plan's own decision
> table. Do not consult revision 1 in git history for guidance.

**Goal:** Build the pool primitives -- `ApplyCost`, `ApplyCostPartial`,
`ApplyHarm`, `ApplyRestore` -- move the hardcoded defence base costs into config
at their current effective values, and clear the orphaned docstrings in
`internal/characters/character.go`. **No call site moves in this slice.**

**Architecture:** Costs split into refuse vs partial-charge by whether the actor
retains a meaningful alternative action. Harm and restore are **one signed
pipeline** with two positive-argument wrappers, so no call site can invert a
sign. `ApplyHarm` carries a `state.ActorRef` from the outset so U5c can add
attributed death without re-touching every site.

**Tech Stack:** Go, `internal/characters`, `internal/configs`.

---

## Context

U5 was originally one chunk contracted as a provable no-op. An inventory found
**137 raw pool-mutation statements across ~78 logical sites** in `internal/`
(`modules/` never mutates a pool -- verified), and found the code inconsistent
rather than merely unrouted. The user released the no-op contract:

> "Rule 1 is a discipline rule, not a rule you follow when we're finding
> obviously broken, scattered, and inconsistent crap."

U5 is therefore three slices. **U5a is still a strict no-op** -- it only adds
functions and moves constants into config. U5b routes sites; U5c adds attributed
death.

### The three floor rules

1. **A cost may never drive any pool below 0.**
2. **Harm floors stamina and conviction at 0.**
3. **Harm may drive health below 0, and MUST be allowed to.**

> **The reason for rule 3 is NOT "death detection reads the negative value".**
> Revision 1 claimed that four times and it is false: every death gate tests
> `< 1` or `<= 0` (`buffs.go:16` `IsDisabled`, `NewRound_AutoHeal.go:57`,
> `combat_shared_helpers.go:512/525`, `combat_retarget.go:34/43`). Flooring at 0
> would not break death.
>
> **The real reason is overkill magnitude**, which U6's margin-scaled work needs,
> plus `validatePoolClamps` (`internal/characters/validate.go:349`) carrying an
> explicit "No lower Health clamp" comment that a floor would contradict.
> Verified: nothing re-clamps health downward -- the reservation clamp
> (`validate.go:146-172`) and the enchant clamp (`:175-211`) are upper-bound only.

### Cost semantics: refuse vs partial charge

**THE RULE: an exhausted actor still acts, it just loses the skill term** --
for actions where refusal would leave them helpless.

> **Death from exhaustion was tried in this game and players hated it.** An actor
> at 0 stamina who cannot attack "may as well just be dead". Refusal also removes
> them from the contest entirely, which removes them from the reach of the
> 5.9/5.10 contest floors. Firing at no skill is punishing but leaves a puncher's
> chance.

**The dividing line is whether the actor retains a meaningful alternative
action.** Revision 1 said the line was "the cooldown channel". **That is false in
both directions** and was disproved: movement, stand and spellcasting have **no**
cooldown, while `reload` does. The cooldown framing also rested on a false
premise -- at ~15 sites `Cooldowns.Try("special-move", ...)` fires *before* the
resource check, so the cooldown is already spent, and only
`internal/actions/mutation_helpers.go:106-116` rolls it back.

| Primitive | On insufficient resource |
|---|---|
| `ApplyCostPartial(pool, amount) CostResult` | charges what is there, reports `Short`, action **fires with no skill term** |
| `ApplyCost(pool, amount) bool` | **refuses**, takes nothing, returns false |

**The assignment. This table is the authority.**

| Action | Primitive | Has a cost TODAY? |
|---|---|---|
| Auto-attack | **Partial** | Yes -- `DeductAttackStamina` |
| Dodge / parry / block | **Partial** | Yes -- `DeductDefenseStamina` |
| Grapple **upkeep** (per round) | **Partial** | Yes -- `Position_GrappleTick.go:733-739` |
| **Flee** | **Partial** | Yes -- `flee.go:41`, and it REFUSES today |
| Movement (`go`) | **Refuse** | Yes -- `go.go:182` |
| Stand from prone | **Refuse** | Yes -- `stand.go:66` |
| Spellcasting | **Refuse** | Yes -- `combat_shared_helpers.go:558`, `mobcommands/cast.go:116` |
| Mutation special moves | **Refuse** | Yes -- `mutation_helpers.go:116` |
| **quell** (mental spell defence) | Partial | **NO -- new in U6** |
| **defy** (social defence) | Partial | **NO -- new in U6** |
| Taunt / rally / warcry | Refuse | **NO -- new in U8** |
| Special moves (bash/kick/trip/gore/...) | Refuse | **NO -- new in U8** |
| Ranged (`shoot`) | Partial | **NO -- new in U8** |

**The right-hand column matters.** Rows marked "NO" are design decisions for a
later chunk, **not U5b routing instructions**. Verified: `combat_bash.go` and
`combat_taunt.go` contain no stamina or conviction *cost* at all, and cost spec
section 2 says taunt/rally/warcry cost nothing today. A U5b implementer hunting
for bash's deduction will find nothing.

### Harm and restore are ONE pipeline

The user's framing: *"restore being part of harm/damage pipeline, it is just the
inverse."* Correct, and it removes a whole primitive.

Internally there is one signed core. Externally there are **two wrappers that
both take a POSITIVE amount**:

```go
func (c *Character) ApplyHarm(pool Pool, amount int, source state.ActorRef) int
func (c *Character) ApplyRestore(pool Pool, amount int) int
```

**Why not one exported signed function.** Sign inversion is this codebase's
signature failure mode -- the arc has already lost time to it twice
(`contest.Result.Margin` attack-positive vs `bestDefenseResult.margin`
defence-positive, documented as trap 1 in the roadmap). Handing ~78 call sites a
signed delta invites exactly that. Positive-only wrappers make the direction
unmissable at the call site while keeping one implementation.

Restore matters more than it looks: **~27 of the 137 statements are additions** --
regen (`NewRound_AutoHeal.go` x6), buff ticks, lifesteal, CP refunds (four
independent copies), respawn. Without `ApplyRestore`, U5b's "no direct pool
mutation" guard cannot pass without a hand-maintained exemption list -- the same
blind spot the floor guard burned a chunk fixing.

### Do NOT build the harm helper on `ApplyHealthChange`

It calls `CancelCombatBuffs()` → `Validate(true)` → `RecalculateStats` +
`validatePoolClamps`, and can emit `events.CharacterStatsChanged`. That side
effect reaches exactly **8 call sites**, all in `internal/combat/combat.go`
(`:59, 63, 137, 142, 215, 218, 264, 265` -- verified). Building on it would add a
full stat recalculation to the ~19 other damage sites in U5b, including every
spell path.

### Do NOT emit events

**Correction to revision 1**, which said "exactly one pool mutation emits an
event" and would have shipped that into a docstring. At least three do:
`Life_Cascades.go:79` emits `CharacterVitalsChanged` directly; `ApplyHealthChange`
emits `CharacterStatsChanged` indirectly via `Validate`; `NewRound_AutoHeal`
emits once at loop end gated on a net change.

The design conclusion still holds, for a stateable reason: **no direct pool
mutation emits**, and the two indirect emitters are deliberate and stay where
they are. A helper that emitted would add ~78 emissions in U5b.

### Do NOT read the pool reservation

**Correction to revision 1**, which said `GetPoolReservation` is consulted in
"only four places". It is ~20 production sites across 6 files, including
`modules/gmcp/gmcp.Char.go`. The conclusion survives: **none of them is an
affordability check**, so U5a reads the raw pool, matching today's behaviour.
Making costs reserve-aware would change costs for every companion holder -- that
is U7/U8, and it is a named behaviour change when it happens.

Worth knowing: `validate.go:146-172` already clamps *current* pools down to the
reserve-excluded ceiling, so a raw-reading cost helper and a reserve-enforcing
validator are already in mild tension. Not U5a's problem; do not "fix" it here.

### Out of scope, deliberately: ActionPoints

Movement is a **two-pool transaction**: `go.go:147` charges `ActionPoints`, then
`:182` charges stamina, and `:185` hand-rolls an AP refund if stamina fails.
`ActionPoints` is a fourth pool with its own `DeductActionPoints` helper.

**It is an inherited GoMud movement throttle**, redundant with DOGMud's
terrain-and-encumbrance-scaled stamina cost, undocumented in any helpfile, and
carrying `ActionPointsMax.Mods = 200 // hard coded for now` (`validate.go:101`).
It is a deletion candidate on its own, so **U5a does not add it to `Pool`**.
Movement stays half-routed until that is decided.

---

## File Structure

**Created:**
- `internal/characters/pools.go` -- `Pool`, `CostResult`, `PoolValue`,
  `CanAfford`, `ApplyCost`, `ApplyCostPartial`, `ApplyHarm`, `ApplyRestore`.
- `internal/characters/pools_test.go`

**Modified:**
- `internal/characters/character.go` -- orphaned docstring cleanup
- `internal/characters/resources.go` -- `GetDefenseStaminaCost` reads config;
  `Deprecated:` markers on the three `Deduct*` functions
- `internal/configs/config.balance.go` -- three new fields
- `internal/configs/config.balance.combat.go` -- their validation defaults
- `_datafiles/config.yaml` -- three knobs, replacing the stale comment block
- `internal/characters/character_test.go` -- extend the existing defence-cost test
- `internal/characters/context.md`
- `docs/PATCH_NOTES.md`

---

### Task 0: Branch

- [ ] **Step 1**

```bash
git checkout -b feature/u5a-cost-harm-foundation
git status
```

Expected: `On branch feature/u5a-cost-harm-foundation`. (If the branch already
exists from planning work, check it out instead.)

---

### Task 1: Clear the orphaned docstrings

Eleven husks, not eight -- revision 1 undercounted and truncated several quotes.
**This task has no compiler safety net**: deleting comments cannot break the
build, so a mistake is silent. Work by line range, and verify before deleting.

**Files:** Modify `internal/characters/character.go`

- [ ] **Step 1: Confirm the ranges are comment-only**

```bash
sed -n '435,455p' internal/characters/character.go
sed -n '654,660p' internal/characters/character.go
sed -n '689,705p' internal/characters/character.go
```

You are looking for runs of `//` lines with **no `func`** between them. If any
range contains a `func`, STOP and report -- the file has changed since this plan
was written.

- [ ] **Step 2: Delete lines 437-451 inclusive**

Five husks in one run. The live functions are at `resources.go:26` (`DeductStamina`),
`:41` (`GetMovementStaminaCost`), `:92` (`GetAttackStaminaCost`), `:111`
(`DeductAttackStamina`), each with its own correct docstring:

```
437  // returns description unless description is a hash
439-440  // DeductStamina attempts to deduct... / // Returns false if...
442-445  // GetMovementStaminaCost calculates... (4 lines)
447-448  // GetAttackStaminaCost calculates... (2 lines)
450-451  // DeductAttackStamina deducts... / // If character doesn't have enough...
```

Delete the whole 437-451 block including its blank lines.

- [ ] **Step 3: Delete lines 691-703 inclusive**

Six husks. Live at `resources.go:126`, `:154`, `:261`, `:269`, `:305`, `:365`.

Note the `AddToxicity` husk is also **factually wrong** -- it says "Returns false
if it would exceed max", but the live function always returns `true` and clamps
to `[0, max]`. Deleting it removes a lie.

- [ ] **Step 4: Fix the mislabelled live function at 656-657**

Worse than an orphan: a wrong docstring on a real function, which reads as
authoritative.

```go
// AttemptRecovery tries to recover from a condition using a stat-based chance
// Formula: min(90, 25 + 20 * ln(statValue/25))
func (c *Character) SetSetting(...
```

The real `AttemptRecovery` formula lives at `internal/characters/skills.go:72-75`,
so nothing is lost. Read `SetSetting`'s body and write a one-line docstring
describing what it actually does.

- [ ] **Step 5: Verify**

```bash
git diff --stat internal/characters/character.go
go build ./... && go test ./internal/characters/
gofmt -l internal/characters/
```

Expected: deletions plus one docstring rewrite; build succeeds; tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/character.go
git commit -m "docs(characters): delete eleven orphaned docstrings and fix a mislabelled one (U5a)"
```

---

### Task 2: The pool primitives

**Files:** Create `internal/characters/pools.go` and `pools_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/characters/pools_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
)

// poolChar builds a character with known pool values.
//
// Note StatInfo.Recalculate sets Value = Base + Training + Mods and does NOT
// derive from Vitality/Strength -- RecalculateStats does that, and this helper
// deliberately never calls it. So Base then Recalculate yields exactly what we
// asked for on a fresh character.
func poolChar(hp, sp, cp int) *Character {
	c := New()
	c.HealthMax.Base = hp
	c.StaminaMax.Base = sp
	c.ConvictionMax.Base = cp
	c.HealthMax.Recalculate()
	c.StaminaMax.Recalculate()
	c.ConvictionMax.Recalculate()
	c.Health, c.Stamina, c.Conviction = hp, sp, cp
	return c
}

// --- costs -----------------------------------------------------------------

// TestApplyCost_RefusesAndTakesNothing is floor rule 1 for refusing costs.
// Treating unaffordability as anything except refusal here is the bug this
// primitive exists to stop: it must not scrape the pool out.
func TestApplyCost_RefusesAndTakesNothing(t *testing.T) {
	for _, pool := range []Pool{PoolHealth, PoolStamina, PoolConviction} {
		c := poolChar(10, 10, 10)
		if c.ApplyCost(pool, 25) {
			t.Errorf("pool %s: an unaffordable cost reported success", pool)
		}
		if got := c.PoolValue(pool); got != 10 {
			t.Errorf("pool %s after a REFUSED cost: got %d, want 10 (untouched)", pool, got)
		}
	}
}

func TestApplyCost_PaysInFullWhenAffordable(t *testing.T) {
	c := poolChar(10, 10, 10)
	if !c.ApplyCost(PoolStamina, 4) {
		t.Error("affordable cost reported failure")
	}
	if c.Stamina != 6 {
		t.Errorf("stamina after paying 4: got %d want 6", c.Stamina)
	}
	// Exact affordability legitimately lands the pool on 0. The rule forbids
	// being scraped out, not reaching empty.
	if !c.ApplyCost(PoolStamina, 6) {
		t.Error("exactly-affordable cost reported failure")
	}
	if c.Stamina != 0 {
		t.Errorf("stamina after paying the exact remainder: got %d want 0", c.Stamina)
	}
}

// TestApplyCostPartial_ChargesWhatItCanAndReportsShort pins the primitive used
// wherever refusal would leave the actor helpless -- auto-attack, defence,
// grapple upkeep, flee. Short is what U8 reads to strip the skill term.
func TestApplyCostPartial_ChargesWhatItCanAndReportsShort(t *testing.T) {
	c := poolChar(10, 7, 10)

	res := c.ApplyCostPartial(PoolStamina, 25)
	if res.Charged != 7 {
		t.Errorf("partial charge: got %d want 7 (all that was there)", res.Charged)
	}
	if !res.Short {
		t.Error("charging less than requested must report Short")
	}
	if c.Stamina != 0 {
		t.Errorf("stamina after a partial charge: got %d want 0", c.Stamina)
	}

	res = c.ApplyCostPartial(PoolStamina, 5)
	if res.Charged != 0 || !res.Short {
		t.Errorf("charge from an empty pool: got %+v want {0 true}", res)
	}
	if c.Stamina != 0 {
		t.Errorf("partial charge drove the pool below zero: got %d", c.Stamina)
	}
}

func TestApplyCostPartial_FullPaymentIsNotShort(t *testing.T) {
	c := poolChar(10, 10, 10)
	res := c.ApplyCostPartial(PoolStamina, 4)
	if res.Charged != 4 || res.Short {
		t.Errorf("affordable partial charge: got %+v want {4 false}", res)
	}
}

func TestCostsIgnoreNonPositive(t *testing.T) {
	c := poolChar(10, 10, 10)
	if !c.ApplyCost(PoolStamina, -5) {
		t.Error("a non-positive cost is free, so it should succeed")
	}
	if res := c.ApplyCostPartial(PoolStamina, -5); res.Charged != 0 || res.Short {
		t.Errorf("negative partial cost: got %+v want {0 false}", res)
	}
	if c.Stamina != 10 {
		t.Errorf("a negative cost changed the pool: got %d want 10", c.Stamina)
	}
}

func TestCanAffordReadsRawPool(t *testing.T) {
	c := poolChar(10, 10, 10)
	if !c.CanAfford(PoolStamina, 10) {
		t.Error("exact affordability should be true")
	}
	if c.CanAfford(PoolStamina, 11) {
		t.Error("over-affordability should be false")
	}
}

// --- harm and restore ------------------------------------------------------

// TestApplyHarm_FloorsStaminaAndConviction is floor rule 2.
func TestApplyHarm_FloorsStaminaAndConviction(t *testing.T) {
	for _, pool := range []Pool{PoolStamina, PoolConviction} {
		c := poolChar(10, 10, 10)
		if applied := c.ApplyHarm(pool, 25, state.ActorRef{}); applied != 10 {
			t.Errorf("pool %s: floored harm returned %d, want 10 (what actually landed)", pool, applied)
		}
		if got := c.PoolValue(pool); got != 0 {
			t.Errorf("pool %s after overkill harm: got %d, want 0", pool, got)
		}
	}
}

// TestApplyHarm_LeavesHealthNegative is floor rule 3, and it is the one that
// matters. Health must be allowed below zero so overkill magnitude survives for
// U6's margin-scaled work, and because validatePoolClamps carries an explicit
// "No lower Health clamp" comment. Do NOT add a health floor.
func TestApplyHarm_LeavesHealthNegative(t *testing.T) {
	c := poolChar(10, 10, 10)
	if applied := c.ApplyHarm(PoolHealth, 25, state.ActorRef{}); applied != 25 {
		t.Errorf("unfloored health harm returned %d, want 25 (all of it landed)", applied)
	}
	if c.Health != -15 {
		t.Errorf("health after overkill: got %d, want -15 (NOT floored)", c.Health)
	}
}

func TestApplyRestore_ClampsAtMax(t *testing.T) {
	for _, pool := range []Pool{PoolHealth, PoolStamina, PoolConviction} {
		c := poolChar(10, 10, 10)
		c.ApplyHarm(pool, 6, state.ActorRef{})
		if applied := c.ApplyRestore(pool, 100); applied != 6 {
			t.Errorf("pool %s: restore returned %d, want 6 (clamped at max)", pool, applied)
		}
		if got := c.PoolValue(pool); got != 10 {
			t.Errorf("pool %s after restore: got %d, want 10", pool, got)
		}
	}
}

// TestApplyRestore_LiftsNegativeHealth: a heal on a character below zero must
// work normally. Nothing special-cases the negative start.
func TestApplyRestore_LiftsNegativeHealth(t *testing.T) {
	c := poolChar(10, 10, 10)
	c.ApplyHarm(PoolHealth, 25, state.ActorRef{}) // -15
	if applied := c.ApplyRestore(PoolHealth, 20); applied != 20 {
		t.Errorf("restore from negative: got %d want 20", applied)
	}
	if c.Health != 5 {
		t.Errorf("health after restoring 20 from -15: got %d want 5", c.Health)
	}
}

func TestHarmAndRestoreIgnoreNonPositive(t *testing.T) {
	c := poolChar(10, 10, 10)
	if applied := c.ApplyHarm(PoolHealth, -5, state.ActorRef{}); applied != 0 {
		t.Errorf("negative harm: got %d want 0", applied)
	}
	if applied := c.ApplyRestore(PoolHealth, -5); applied != 0 {
		t.Errorf("negative restore: got %d want 0", applied)
	}
	if c.Health != 10 {
		t.Errorf("a non-positive amount changed the pool: got %d want 10", c.Health)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/characters/ -run 'TestApplyCost|TestApplyCostPartial|TestCosts|TestCanAfford|TestApplyHarm|TestApplyRestore|TestHarmAndRestore' -v
```

Expected: FAIL, `undefined: Pool` and friends.

> The `-run` pattern must include `TestCosts` and `TestHarmAndRestore` -- an
> earlier draft used a shorter pattern that silently skipped two tests while
> reporting PASS.

- [ ] **Step 3: Write the primitives**

Create `internal/characters/pools.go`:

```go
package characters

import "github.com/GoMudEngine/GoMud/internal/state"

// Pool identifies one of the three resource pools.
//
// Deliberately a STRING, matching the vocabulary already used by
// GetPoolReservation(pool string, ...) and BuffSpec.TickPool. An int enum would
// have made this the third pool vocabulary in the package and forced a
// translation function the moment U7 makes costs reserve-aware.
//
// The pools themselves are plain int fields on Character. Only the MAX values
// are StatInfo, read via .Value.
type Pool string

const (
	PoolHealth     Pool = "health"
	PoolStamina    Pool = "stamina"
	PoolConviction Pool = "conviction"
)

// CostResult reports what a partial cost charge actually did.
//
// Returned by value rather than as a bare int so later chunks can add fields
// without re-touching the ~78 call sites U5b routes. U8 in particular needs
// Short to know it must strip the skill term from the action's roll, and U7
// will change how Short is computed when affordability becomes reserve-aware --
// that belongs inside this helper, once, not at every site.
type CostResult struct {
	Charged int  // amount actually taken from the pool
	Short   bool // the actor could not pay in full
}

// PoolValue reads the current value of a pool.
func (c *Character) PoolValue(p Pool) int {
	switch p {
	case PoolHealth:
		return c.Health
	case PoolStamina:
		return c.Stamina
	case PoolConviction:
		return c.Conviction
	}
	return 0
}

// poolMax reads a pool's ceiling.
func (c *Character) poolMax(p Pool) int {
	switch p {
	case PoolHealth:
		return c.HealthMax.Value
	case PoolStamina:
		return c.StaminaMax.Value
	case PoolConviction:
		return c.ConvictionMax.Value
	}
	return 0
}

// setPool writes a pool. Unexported so every caller goes through a primitive
// and cannot bypass the floor rules.
func (c *Character) setPool(p Pool, v int) {
	switch p {
	case PoolHealth:
		c.Health = v
	case PoolStamina:
		c.Stamina = v
	case PoolConviction:
		c.Conviction = v
	}
}

// CanAfford reports whether the character could pay amount from pool.
//
// Reads the RAW pool. GetPoolReservation is deliberately NOT consulted: no
// affordability check in the codebase consults it today, and making costs
// reserve-aware would change the cost of every action for every companion
// holder. That is U7/U8 work and a named behaviour change when it lands.
func (c *Character) CanAfford(pool Pool, amount int) bool {
	if amount <= 0 {
		return true
	}
	return c.PoolValue(pool) >= amount
}

// ApplyCost pays amount from pool IN FULL or not at all, reporting whether it
// paid.
//
// Use this where refusing is safe because the actor retains a meaningful
// alternative action: movement, standing, spellcasting, mutation special moves.
// They can still auto-attack, so declining costs them a short wait rather than
// their participation.
//
// Use ApplyCostPartial instead wherever refusal would leave the actor helpless.
// See the assignment table in the U5a plan; it is the authority, and the split
// is NOT "volitional vs involuntary" and NOT "uses a cooldown" -- both framings
// were tried and both are wrong.
//
// Floor rule 1: never drives a pool below 0. Paying a cost you exactly afford
// legitimately lands the pool on 0; the rule forbids being scraped out.
//
// A non-positive amount is free, so it succeeds and changes nothing. That stops
// a negative "cost" being used as a backdoor heal.
//
// Emits no event and performs no validation, deliberately. No direct pool
// mutation in this codebase emits; adding one here would add an emission at
// every site U5b routes.
func (c *Character) ApplyCost(pool Pool, amount int) bool {
	if amount <= 0 {
		return true
	}
	current := c.PoolValue(pool)
	if current < amount {
		return false
	}
	c.setPool(pool, current-amount)
	return true
}

// ApplyCostPartial charges up to amount from pool, taking whatever is there when
// the actor cannot pay in full, and reports what happened.
//
// Use this wherever refusal would leave the actor helpless: auto-attacks,
// defensive avoidance, grapple upkeep, and flee. Death from exhaustion was tried
// in this game and players hated it -- an actor who cannot act may as well be
// dead. Refusal also removes them from the contest entirely, and therefore from
// the reach of the 5.9/5.10 contest floors, which is the mechanical reason this
// primitive exists.
//
// The punishment for being short is losing the SKILL TERM from the action's
// roll, not losing the action. Applying that penalty is chunk U8; this helper
// only reports CostResult.Short so U8 has something to read.
//
// Never drives the pool below 0. A non-positive amount charges nothing and is
// not Short.
func (c *Character) ApplyCostPartial(pool Pool, amount int) CostResult {
	if amount <= 0 {
		return CostResult{}
	}
	current := c.PoolValue(pool)
	if current <= 0 {
		return CostResult{Charged: 0, Short: true}
	}
	charged := amount
	if charged > current {
		charged = current
	}
	c.setPool(pool, current-charged)
	return CostResult{Charged: charged, Short: charged < amount}
}

// applyVitalChange is the single signed pipeline behind ApplyHarm and
// ApplyRestore. Negative delta is harm, positive is restore.
//
// It is unexported on purpose. Sign inversion is this codebase's signature
// failure mode -- the resolution arc has already lost time to attack-positive
// versus defence-positive margins -- so call sites get positive-only wrappers
// and cannot get the direction wrong.
func (c *Character) applyVitalChange(pool Pool, delta int) int {
	current := c.PoolValue(pool)
	next := current + delta

	if delta > 0 {
		if max := c.poolMax(pool); next > max {
			next = max
		}
	} else if pool != PoolHealth && next < 0 {
		// Floor rule 2: stamina and conviction stop at empty.
		// Floor rule 3: health does NOT, deliberately.
		next = 0
	}

	c.setPool(pool, next)
	return next - current
}

// ApplyHarm applies amount of harm from source to pool and returns the amount
// ACTUALLY applied.
//
// Floor rule 2: stamina and conviction floor at 0.
//
// Floor rule 3: health is NOT floored. It must be allowed below zero so overkill
// magnitude survives -- U6's margin-scaled work needs it -- and because
// validatePoolClamps carries an explicit "No lower Health clamp" comment.
// Note the reason is NOT that death detection reads the negative value: every
// death gate tests < 1 or <= 0, so a floor would not break death. Do not add one
// anyway, however wrong a negative number looks in a debugger.
//
// The return is the APPLIED delta, not the requested amount; they differ when a
// floor bites. Callers keeping a result struct in sync with the pool --
// NewRound_DoCombat_unified does -- must add the returned value.
//
// source is unused in U5a and is present ON PURPOSE, so U5b routes each call site
// once and U5c can add attributed death without a second pass. Be realistic about
// its reach: the direct combat, spell and maneuver sites have an actor in hand,
// but damage-over-time, toxicity and attrition sites do NOT -- buffs.Buff has no
// applier field, so those pass the zero value and stay anonymous until the buff
// system carries one. That is not U5 work.
//
// Deliberately does not call Die, cancel buffs, validate, or emit. In particular
// this is NOT built on ApplyHealthChange, whose buff-cancel and full stat
// recalculation reach 8 call sites today and must not spread silently.
func (c *Character) ApplyHarm(pool Pool, amount int, source state.ActorRef) int {
	if amount <= 0 {
		return 0
	}
	return -c.applyVitalChange(pool, -amount)
}

// ApplyRestore restores amount to pool and returns the amount ACTUALLY restored,
// which is less than requested when the pool hits its maximum.
//
// Restore is the same pipeline as harm with the sign flipped, which is why the
// two share applyVitalChange. Restoring a character whose health is negative
// works normally and is not special-cased.
//
// Takes no source: nothing downstream attributes healing today, and inventing a
// parameter nobody fills is how ApplyHarm's source would have become decoration.
func (c *Character) ApplyRestore(pool Pool, amount int) int {
	if amount <= 0 {
		return 0
	}
	return c.applyVitalChange(pool, amount)
}
```

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/characters/ -run 'TestApplyCost|TestApplyCostPartial|TestCosts|TestCanAfford|TestApplyHarm|TestApplyRestore|TestHarmAndRestore' -v
go build ./... && go test ./internal/characters/
gofmt -l internal/characters/
```

Expected: all PASS, build succeeds, gofmt silent.

- [ ] **Step 5: Commit**

```bash
git add internal/characters/pools.go internal/characters/pools_test.go
git commit -m "feat(characters): add the pool cost, harm and restore primitives (U5a)"
```

---

### Task 3: Move the defence base costs into config

The base costs `2 / 4 / 5` are hardcoded at `resources.go:134/137/140` while only
their multipliers are config -- the "no balance number inside `internal/`"
anti-pattern sitting in the code this arc is rewriting.

**The arithmetic is truncate-then-floor, in that order.** `int(2 x 0.9) = 1`,
`int(4 x 0.9) = 3`, `int(5 x 0.9) = 4`. All three multipliers default AND ship at
0.9, so live costs today are **dodge 1, parry 3, block 4**. Rounding instead of
truncating would make dodge cost 2 -- a 100% rise on the most-used defence.

**Files:** `internal/configs/config.balance.go`,
`internal/configs/config.balance.combat.go`, `_datafiles/config.yaml`,
`internal/characters/resources.go`, `internal/characters/character_test.go`

- [ ] **Step 1: Add the three fields**

In `internal/configs/config.balance.go`, adjacent to the existing
`DodgeMultiplier` / `ParryMultiplier` / `BlockMultiplier` declarations. Use the
names the cost spec already uses, so U7 does not rename them again:

```go
	DodgeBaseStaminaCost ConfigInt `yaml:"DodgeBaseStaminaCost"` // Base stamina cost for dodge, before multiplier (default 2)
	ParryBaseStaminaCost ConfigInt `yaml:"ParryBaseStaminaCost"` // Base stamina cost for parry, before multiplier (default 4)
	BlockBaseStaminaCost ConfigInt `yaml:"BlockBaseStaminaCost"` // Base stamina cost for block, before multiplier (default 5)
```

- [ ] **Step 2: Add the validation defaults in `config.balance.combat.go`**

**NOT `config.balance.misc.go`.** The three multipliers' defaults live in
`internal/configs/config.balance.combat.go`, in `validateCombat()`, under a block
headed `── COMBAT: DEFENSE COSTS ──`. Put the new defaults immediately after the
`BlockMultiplier` default so the five-value family stays together, matching the
surrounding idiom:

```go
	if b.DodgeBaseStaminaCost < 1 {
		b.DodgeBaseStaminaCost = 2
	}
	if b.ParryBaseStaminaCost < 1 {
		b.ParryBaseStaminaCost = 4
	}
	if b.BlockBaseStaminaCost < 1 {
		b.BlockBaseStaminaCost = 5
	}
```

A floor of 1, not 0: a zero base would make the defence free, and the final cost
is already floored at 1.

> `GetBalanceConfig()` calls `ensureConfigValidated()`, so these defaults apply
> even in a test binary that never loads `config.yaml`. That is what makes the
> Step 5 test meaningful. If you put them anywhere `Balance.Validate()` does not
> reach, they silently stay 0.

- [ ] **Step 3: Read the knobs in `GetDefenseStaminaCost`**

In `internal/characters/resources.go`, replace:

```go
	case DefenseDodge:
		baseCost = 2
		multiplier = float64(bal.DodgeMultiplier)
	case DefenseParry:
		baseCost = 4
		multiplier = float64(bal.ParryMultiplier)
	case DefenseBlock:
		baseCost = 5
		multiplier = float64(bal.BlockMultiplier)
```

with:

```go
	case DefenseDodge:
		baseCost = int(bal.DodgeBaseStaminaCost)
		multiplier = float64(bal.DodgeMultiplier)
	case DefenseParry:
		baseCost = int(bal.ParryBaseStaminaCost)
		multiplier = float64(bal.ParryMultiplier)
	case DefenseBlock:
		baseCost = int(bal.BlockBaseStaminaCost)
		multiplier = float64(bal.BlockMultiplier)
```

**Leave everything else alone** -- the `int(float64(baseCost) * multiplier)`
truncation and the `if cost < 1` floor must not move or change form.

- [ ] **Step 4: Extend the EXISTING test, do not add a new one**

`internal/characters/character_test.go:2576` already has
`TestGetDefenseStaminaCost` asserting dodge 1 / parry 3 / block 4, plus an
`"unknown" -> 0` case. Adding a second test would be a strictly weaker duplicate.

Add a comment to that test recording what it now also protects:

```go
	// Also pins U5a's move of the base costs 2/4/5 out of Go and into config.
	// A test binary never loads config.yaml, but GetBalanceConfig runs
	// ensureConfigValidated, so these are the VALIDATION defaults -- which is
	// exactly the point: it proves the defaults match the literals that were
	// deleted. Truncation, not rounding: int(2*0.9)=1, int(4*0.9)=3,
	// int(5*0.9)=4.
	//
	// Note the dodge row is the weakest of the three: int(0*0.9)=0 also floors
	// to 1, so dodge=1 is produced both by a correct move and by the knob being
	// entirely unwired. Parry and block are the rows that actually detect a
	// botched move.
```

- [ ] **Step 5: Edit `config.yaml` -- REPLACE the stale comment block**

`_datafiles/config.yaml` around lines 421-432 **already documents these base
costs in prose**, including three inline `# (base: N SP)` comments and an example
(`BlockMultiplier 1.5 -> floor(5 x 1.5) = 7`) that is already stale against the
shipped 0.9. Adding a second block beside it leaves the value documented in four
places, three of which rot.

**Replace** that block, delete the three inline `# (base: N SP)` notes, and write:

```yaml
  # Defence stamina costs. Final cost is int(base * multiplier), TRUNCATED, then
  # floored at 1. With the multipliers at 0.9 the live costs are dodge 1,
  # parry 3, block 4. Rounding instead of truncating would take dodge to 2, a
  # 100% rise on the most-used defence in the game.
  #
  # The base costs were hardcoded in Go until U5a moved them here; only the
  # multipliers were ever configurable, which is the anti-pattern the resolution
  # arc exists to remove.
  #
  # U7 replaces this shape entirely: base cost becomes one term in a unified
  # formula with an encumbrance multiplier and an inverse-skill multiplier, and
  # these values become modifiers rather than bases. Do not retune them here in
  # the meantime -- that retune is a modelled change, not a nudge.
  DodgeBaseStaminaCost: 2
  ParryBaseStaminaCost: 4
  BlockBaseStaminaCost: 5
  DodgeMultiplier: 0.9
  ParryMultiplier: 0.9
  BlockMultiplier: 0.9
```

(Keep whatever the existing multiplier lines say if they differ; do not retune.)

- [ ] **Step 6: Stage `config.yaml` safely -- NON-INTERACTIVE**

This file has `skip-worktree` set and the working copy **genuinely diverges from
HEAD in three hunks** (`HttpPort`, `LogToFile`, a `LogLevel` comment). A blind
`git add` leaks local dev settings to master, which happened on 2026-08-11.
`git add -p` is interactive and **will not work** in this environment.

```bash
git update-index --no-skip-worktree _datafiles/config.yaml
cp _datafiles/config.yaml /c/tmp/config.local.yaml     # keep the local settings
git checkout -- _datafiles/config.yaml                 # clean copy from HEAD
```

Now apply **only** the Step 5 edit to the clean copy, then:

```bash
git add _datafiles/config.yaml
git diff --cached _datafiles/config.yaml
```

**Read that diff. It is the only real gate.** It must show only the defence-cost
comment block and the three new knobs. If `HttpPort` or `LogToFile` appear,
`git restore --staged` and start over.

```bash
git commit -m "feat(config): move the defence base costs out of Go and into config (U5a)"
cp /c/tmp/config.local.yaml _datafiles/config.yaml     # restore local settings
```

Re-apply the same three knobs to the restored local copy, then **always**:

```bash
git update-index --skip-worktree _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml
```

Expected: the second command prints a line beginning with lowercase `S`,
confirming skip-worktree is back on. Do this even if the commit failed.

- [ ] **Step 7: Verify**

```bash
go build ./... && go test ./internal/characters/ ./internal/configs/
gofmt -l internal/ modules/
grep -n "baseCost = [0-9]" internal/characters/resources.go
```

Expected: build and tests pass, gofmt silent, grep prints nothing.

- [ ] **Step 8: Commit the code half**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.combat.go \
        internal/characters/resources.go internal/characters/character_test.go
git commit -m "refactor(characters): GetDefenseStaminaCost reads its base costs from config (U5a)"
```

---

### Task 4: Deprecate the superseded helpers

Standing rule 4 says delete as you migrate. U5a cannot -- deleting `Deduct*`
means routing call sites, which is U5b. The arc already has a convention for this
gap: `dice.OpposedRollStat*` carried `Deprecated:` markers through U4 for exactly
the same reason.

**Files:** `internal/characters/resources.go`

- [ ] **Step 1: Add markers**

As the last line of each docstring, after a blank `//` line:

- `DeductStamina`: `// Deprecated: use ApplyCost. U5b routes the remaining callers.`
- `DeductDefenseStamina`: `// Deprecated: use ApplyCostPartial -- defence charges what it can. U5b routes the remaining callers.`
- `DeductAttackStamina`: `// Deprecated: use ApplyCostPartial. Its pay-what-you-can behaviour is correct but nothing downstream strips the skill term yet; U8 adds that.`

Leave `ApplyHealthChange` unmarked -- U5b decides its fate along with the buff-cancel side effect.

- [ ] **Step 2: Verify and commit**

```bash
go build ./... && go vet ./internal/characters/ && gofmt -l internal/characters/
git add internal/characters/resources.go
git commit -m "docs(characters): deprecate the superseded Deduct helpers (U5a)"
```

---

### Task 5: Documentation

**Files:** `internal/characters/context.md`, `docs/PATCH_NOTES.md`

- [ ] **Step 1: Document the API**

`internal/characters/context.md` has **no `## Public API` or `## Gotchas`
section** -- do not go looking for them. Its structure is `## Overview`,
`## Key Components`, `### Resource Pools`, `### Combat and Interaction Systems`,
then per-chunk sections.

Extend **`### Resource Pools`** with a `#### Pool mutation API` subsection
carrying the verified signatures of `Pool`, `CostResult`, `PoolValue`,
`CanAfford`, `ApplyCost`, `ApplyCostPartial`, `ApplyHarm`, `ApplyRestore`, then a
`#### Gotchas` subsection:

```markdown
- **Three floor rules, and they are not symmetric.** A cost never drives a pool
  below 0. Harm floors stamina and conviction at 0. Harm does **not** floor
  health. The health rule exists to preserve overkill magnitude for margin-scaled
  work and because `validatePoolClamps` carries an explicit "No lower Health
  clamp" comment -- **not** because death detection reads the negative value,
  which it does not (every gate tests `< 1` or `<= 0`).
- **`ApplyCost` refuses; `ApplyCostPartial` charges what it can.** The split is
  whether the actor retains a meaningful alternative action, NOT volition and NOT
  whether a cooldown is involved -- both framings were tried and both are wrong.
  Refuse: movement, stand, spellcasting, mutation moves. Partial: auto-attack,
  dodge/parry/block, grapple upkeep, flee. Death from exhaustion was tried in
  this game and players hated it.
- **`CostResult.Short` is what a later chunk reads to strip the skill term.**
  The penalty for being short is a worse roll, not a lost action.
- **`CanAfford` reads the RAW pool, not reserve-excluded.** No affordability
  check in the codebase consults `GetPoolReservation` today. Note
  `validate.go:146` already clamps current pools to the reserve-excluded ceiling,
  so a raw-reading cost helper and a reserve-enforcing validator are in mild
  tension. Do not resolve that here.
- **Harm and restore are one signed pipeline** (`applyVitalChange`) behind two
  positive-only wrappers. Sign inversion is this codebase's signature failure
  mode; the wrappers exist so no call site can get the direction wrong.
- **Both return the APPLIED delta**, which differs from the requested amount when
  a floor or ceiling bites. A caller keeping a result struct in sync must add the
  return value.
- **`ApplyHarm`'s source is not universally available.** Direct combat, spell and
  maneuver sites have an actor; damage-over-time, toxicity and attrition sites do
  not, because `buffs.Buff` has no applier field. Those pass the zero value.
- **Do not build on `ApplyHealthChange`.** It cancels combat buffs and triggers a
  full stat recalculation via `Validate(true)` when health crosses zero, reaching
  8 call sites today.
- **No direct pool mutation emits an event**, and neither primitive does. The two
  indirect emitters (`ApplyHealthChange` via `Validate`, and `Life_Cascades`'
  respawn set) are deliberate.
- **ActionPoints is a fourth pool and is NOT in `Pool`.** It is an inherited
  GoMud movement throttle, redundant with stamina movement costs, and a deletion
  candidate. Movement is a two-pool transaction with a hand-rolled refund.
```

- [ ] **Step 2: Audit -- SCOPED**

```bash
python tools/context_md_audit.py internal/characters
```

Expected: `All documented symbols resolve.`

> Run it scoped. A bare invocation audits the whole repo and reports
> **pre-existing** phantoms in other packages while still exiting 0, so "zero
> phantom symbols" is unmeetable as a whole-repo expectation.

- [ ] **Step 3: Patch notes**

The Pre-Push SOP requires a dated entry. `docs/PATCH_NOTES.md` is **newest-first**
with `## YYYY-MM-DD: Title` headings and short prose -- follow that, prepend, do
not append. U5a changes nothing a player can see, so keep it honest and short:

```markdown
## 2026-08-13: Groundwork

- Internal work on how actions spend stamina and conviction. Nothing about
  costs or damage has changed. This is preparation for the combat rebalance.
```

- [ ] **Step 4: Full verification**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/characters/ ./internal/configs/ ./internal/combat/
```

**Known noise:** the `internal/relationships` test binary is quarantined by
Windows Defender. Expected and unrelated.

- [ ] **Step 5: Confirm nothing was routed**

```bash
grep -rn "ApplyCost\|ApplyHarm\|ApplyRestore" internal/ modules/ --include=*.go \
  | grep -v _test | grep -v "^internal/characters/pools.go"
```

Expected: **no output.** U5a adds primitives and moves nothing.

- [ ] **Step 6: Boot test on NON-DEFAULT ports**

The user runs the local server continuously; `config.yaml` holds the live ports.
Copying it verbatim makes the test either fail to bind or silently target the
running server.

```bash
git worktree add --detach C:/tmp/dogmud-u5a-boot HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-u5a-boot/_datafiles/config.yaml
```

Edit the worktree copy with these exact values. **Note `TelnetPort` is a YAML
list, not a scalar:**

```yaml
  TelnetPort: [43333]
  LocalPort: 19999
  HttpPort: 18090
  AIPort: 15555
```

and set `Logging.LogToFile: false` -- a fresh worktree has no `_datafiles/logs`
directory and the server exits 1 if file logging is on.

```bash
cd C:/tmp/dogmud-u5a-boot && timeout 150 go run . > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log  # want 0
grep -c "Server Ready" boot.log                                         # want 1
grep -icE "bind:|address already in use" boot.log                       # want 0
```

**Exit code 124 is the success case.** Do not grep for the bare word `panic`:
`GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`.

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git worktree remove --force C:/tmp/dogmud-u5a-boot || rm -rf C:/tmp/dogmud-u5a-boot
git worktree prune
```

- [ ] **Step 7: Commit**

```bash
git add internal/characters/context.md docs/PATCH_NOTES.md
git commit -m "docs(characters): document the pool primitives and the three floor rules (U5a)"
```

---

## Ship it

```bash
git push -u origin feature/u5a-cost-harm-foundation
gh pr create --repo pruuk/DOGMud --base master --head feature/u5a-cost-harm-foundation --fill
```

`gh pr create` prints the PR URL; the trailing number is the PR number. Then:

```bash
gh pr checks <PR-number> --repo pruuk/DOGMud --watch
gh pr merge  <PR-number> --repo pruuk/DOGMud --merge --delete-branch
```

**Always pass `--repo pruuk/DOGMud`** -- `gh` defaults to the fork parent. Use
`--merge`, not `--squash`. A green `gh pr checks --watch` can return early, so
confirm which runs actually re-ran.

**Do not propose a deploy.** The arc is under a deploy gate until U0-U11 are
complete and playtested.

---

## Carried forward to U5b — decide these explicitly, do not let them slip

1. **Defence does not work the intended way today.** `combat_helpers.go:561`
   uses the cost as an affordability **gate** that `continue`s an unaffordable
   defence out of the best-of-N candidate set, so an exhausted character does not
   get to dodge at all. (Revision 1 also claimed the discarded bool at `:665`
   meant defences went uncharged -- **that was wrong**: candidates are gated
   before the contest, so the winner is affordable by construction.)
2. **Flee changes behaviour.** It costs a hardcoded **10** stamina and REFUSES
   today, printing "You're too exhausted to flee!". Moving it to partial charge
   means a 0-stamina player can always disengage, the message becomes dead code,
   and the hardcoded 10 is a standing-rule-1 violation sitting in a routed site.
3. **`stand` uses two independent knobs.** `StandMinStamina` gates and
   `StandStaminaCost` charges, and the charge floors at 0 today. Both default to
   0.15, so `ApplyCost` is equivalent only while they are equal. Needs
   `CanAfford(gate)` + `ApplyCost(cost)`, not a straight swap.
4. **Grapple upkeep** floors at 0 today, so an exhausted grappler grapples free.
   Partial charging fixes the free part; whether the grapple *breaks* is
   undecided.
5. **`mobcommands/cast.go:116`** debits mob CP with no guard and no clamp, so a
   mob can cast at 0 CP into negative. Any handling changes behaviour.
6. **The prone/stand interaction is a measured death spiral, not a hypothetical.**
   In-combat stamina regen floors at **1 per round**; stand needs 15% of max, so
   a 300-max character needs ~45 rounds prone under `ProneVulnerabilityMultiplier`
   and the prone defence penalties, unable to stand and (movement refuses) unable
   to walk away. Flee is the only exit, which is an argument for its Partial
   assignment. Revision 1's "a brief wait, not a lock" was quantitatively false.
7. **Buff applier attribution is unowned.** `buffs.Buff` has no applier field, so
   U5c cannot attribute DoT kills. File it or accept anonymous DoT deaths.
8. **Movement stays half-routed** while ActionPoints is undecided.

---

## Definition of done

1. `Pool`, `CostResult`, `PoolValue`, `CanAfford`, `ApplyCost`,
   `ApplyCostPartial`, `ApplyHarm`, `ApplyRestore` exist and are tested.
2. `ApplyHarm` leaves health negative on overkill, pinned by a test asserting the
   exact value.
3. `ApplyCost` refuses without taking anything; `ApplyCostPartial` reports
   `Short`. Both pinned.
4. `grep -n "baseCost = [0-9]" internal/characters/resources.go` returns nothing.
   `DodgeBaseStaminaCost` / `ParryBaseStaminaCost` / `BlockBaseStaminaCost` exist
   in config with the stale prose block **replaced**, not duplicated.
5. `TestGetDefenseStaminaCost` still returns dodge 1 / parry 3 / block 4.
6. **No call site routed** -- the Task 5 Step 5 grep is empty.
7. Eleven orphaned docstrings deleted; the `SetSetting` mislabel corrected.
8. The three `Deduct*` helpers carry `Deprecated:` markers.
9. `context.md` accurate; `context_md_audit.py internal/characters` clean.
10. `git diff --cached _datafiles/config.yaml` contained only the defence-cost
    change; skip-worktree confirmed re-set via `git ls-files -v`.
11. Boot test clean on non-default ports; `gofmt -l internal/ modules/` silent;
    `docs/PATCH_NOTES.md` has a dated entry.
