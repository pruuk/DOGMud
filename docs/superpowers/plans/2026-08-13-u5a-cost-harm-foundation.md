# U5a — Cost and Harm Foundation: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the two pool helpers (`ApplyCost`, `ApplyHarm`) with the three
floor rules, move the hardcoded defence costs `2 / 4 / 5` into config at their
current effective values, and clear the orphaned docstrings in
`internal/characters/character.go`. **No call site moves in this slice.**

**Architecture:** `ApplyHarm` carries a `state.ActorRef` source from the outset,
before any call site moves, so U5c can route attributed death without touching
all ~78 sites a second time. Cost gets two primitives split by the KIND of
action, never by caller convenience: `ApplyCost` refuses for volitional actions,
`ApplyCostPartial` charges what it can for involuntary defensive avoidance.

**Tech Stack:** Go, `internal/characters`, `internal/configs`.

---

## Context: why this slice exists and what it must not do

U5 was originally one chunk contracted as a provable no-op. The inventory found
**137 raw pool-mutation statements across ~78 logical sites** in `internal/`
(`modules/` never mutates a pool), and found the code inconsistent rather than
merely unrouted. The user released the no-op contract:

> "Rule 1 is a discipline rule, not a rule you follow when we're finding
> obviously broken, scattered, and inconsistent crap."

U5 is therefore three slices. **This is U5a and it is still a strict no-op**,
because it only adds functions and moves constants into config. Nothing calls
the new helpers yet. U5b routes the sites; U5c adds attributed death.

### The three floor rules (user-corrected, do not "improve" them)

1. **A cost may never drive any pool below 0.** Refined below: a *volitional*
   cost pays in full or takes nothing, and only *defensive* costs may charge a
   partial amount down to exactly 0.
2. **Harm floors stamina and conviction at 0.**
3. **Harm may drive health below 0, and MUST be allowed to. That is how death
   works.** `validatePoolClamps` (`internal/characters/validate.go:349`) carries
   an explicit "No lower Health clamp" comment, and the per-round death checks
   read the negative value. Clamping health at 0 destroys overkill magnitude.

The spec's success criterion previously said "No pool can be driven negative from
any path". That was wrong and is now corrected in the spec itself.

### Unaffordability is REFUSAL. Partial payment is a bug.

Verified in `internal/characters/resources.go`:

| Function | Behaviour when the actor cannot pay |
|---|---|
| `DeductStamina(amount) bool` | **Refuses.** Returns false, mutates nothing. |
| `DeductDefenseStamina(type) bool` | **Refuses.** Returns false, mutates nothing. |
| `DeductAttackStamina() int` | **Pays what it can.** Zeroes the pool, returns the amount taken. |

**User decision, 2026-08-13, arrived at over three messages. The final shape
inverts the default, so read this rather than the intermediate versions.**

**THE RULE: an exhausted actor still acts. It just loses the skill term.**
Combat actions -- auto-attacks, defensive avoidance (dodge / parry / block /
spell-resist) and grapple -- **always fire**, charge **the maximum the actor can
pay**, and take their punishment as the **loss of the skill contribution to that
action's side of the calculation**.

The reasoning is a design constraint learned the hard way, not a preference:

> **Death from exhaustion was tried in this game and players hated it.** An
> actor at 0 stamina who cannot attack "may as well just be dead". Lying there
> only defending is realistic and actively anti-fun.

An exhausted fighter still swings. They will rarely hit -- losing `skill x 5` is
a swing of hundreds of points -- but they are still in the fight, and the 5.9/5.10
contest floors still guarantee a puncher's chance. **Refusal removes them from
the contest entirely, which also removes them from the reach of those floors.**
That is the mechanical reason the rule is what it is, and it applies just as much
to attacking as to defending. Spec section 3.4 reached the same conclusion for
defence; the user extended it to attacks and grapple.

**Refusal survives only as the narrow exception**, for discrete opt-in abilities
where declining is not disabling: the player still has their auto-attack, so
nothing is taken away from them by a refusal.

So there are two cost primitives, and **which one a site uses is determined by
the kind of action, never by caller convenience**:

| Primitive | On insufficient resource |
|---|---|
| `ApplyCostPartial(pool, amount) int` | charges what is there, returns the actual amount, action **fires with no skill term** |
| `ApplyCost(pool, amount) bool` | **refuses**, takes nothing, returns false |

**The assignment, decided by the user 2026-08-13. This table is the authority;
route U5b sites from it, do not re-derive it from the principle.**

| Action | Primitive | Why |
|---|---|---|
| Auto-attack | **Partial** | Refusing is the exhaustion-death failure |
| Dodge / parry / block / spell-resist | **Partial** | You do not choose to be attacked |
| Grapple **upkeep** (per round, once engaged) | **Partial** | Involuntary once you are in it |
| **Flee** | **Partial** | Movement, but being unable to escape is the free-target problem again |
| Movement (`go`) | **Refuse** | Stamina regenerates; a brief wait, not a lock |
| Stand up from prone | **Refuse** | As above |
| Taunt / rally / warcry | **Refuse** | Cooldown channel |
| Special moves (bash / kick / trip / gore / ...) | **Refuse** | Cooldown channel |
| Grapple actions the player **inputs** | **Refuse** | Cooldown channel |
| Spellcasting | **Refuse** | Cooldown channel |

**The organising rule is the cooldown channel, not volition.** Anything that
consumes a cooldown refuses, because firing it with no skill term *and* burning
the cooldown is a trap: the actor loses the ability for several rounds and got
nothing for it. Refusal there costs them nothing they cannot retry shortly, and
their auto-attack is still available. Everything continuous or involuntary --
being swung at, already being in a grapple, trying to get away -- fires with the
penalty instead.

Note this **resolves the grapple-upkeep question** the earlier draft left open:
inputted grapple actions refuse; the per-round upkeep charges partially. So an
exhausted grappler keeps holding on at no skill rather than the grapple silently
becoming free.

> **Flag for U5b, worth one look before it ships.** Refusing `stand` interacts
> with prone: a prone character takes `ProneVulnerabilityMultiplier` and the
> prone dodge/parry/block penalties, so an exhausted prone character in combat
> cannot get up *and* is being hit harder while their stamina regenerates. That
> may be an acceptable pressure, or it may be a death spiral. It is a real
> interaction rather than a hypothetical, so measure it rather than assuming.

Floor rule 1 splits to match: a refusing cost pays in full or takes nothing; a
partial cost may drain a pool to exactly 0 but never below.

`DeductAttackStamina`'s existing pay-what-you-can behaviour turns out to be
**right for the wrong reason** -- it charges what it can, but nothing downstream
strips the skill term, so an exhausted attacker currently swings at full skill
for free. U5b routes it; U8 adds the penalty.

**What happens to the ACTION is a separate question that U5a does not answer.**
Dropping the skill term is chunk U8 (spec §3.4). U5a supplies the primitives;
U5b routes each site preserving its current consequence and names every change.

> **Flags for U5b, both real behaviour changes to decide explicitly:**
>
> 1. **Defence does not work the user's way today.** `combat_helpers.go:561`
>    uses `GetDefenseStaminaCost` as an affordability gate that `continue`s an
>    unaffordable defence **out of the best-of-N candidate set** -- so an
>    exhausted character does not get to dodge at all -- and then
>    `combat_helpers.go:665` **discards** `DeductDefenseStamina`'s bool, so a
>    defence that did get selected is never actually charged when it cannot pay.
>    Moving to "always fires, charges what it can, loses the skill term" changes
>    both.
> 2. **Grapple upkeep has no decided consequence.**
>    `Position_GrappleTick.go:733-739` floors at 0, so an exhausted grappler
>    keeps grappling free. Upkeep is volitional, so refusal applies -- but that
>    leaves "does the grapple break?" open. Decide it, do not let refusal quietly
>    become free upkeep.

### Do NOT build the harm helper on `ApplyHealthChange`

`ApplyHealthChange` (`resources.go:163`) looks like the obvious base and is a
trap. On crossing below zero it calls `CancelCombatBuffs()`, which calls
`Validate(true)`, which re-runs `RecalculateStats`, the reservation clamp and
`validatePoolClamps`, and can emit `events.CharacterStatsChanged`. That side
effect currently reaches exactly **8 call sites**, all in
`internal/combat/combat.go`. Building `ApplyHarm` on top of it would add a full
stat recalculation to the ~19 other damage sites in U5b, including every spell
path. Build a bare primitive; leave `ApplyHealthChange` alone in this slice.

### Do NOT emit events from the helpers

Exactly **one** pool mutation in the codebase emits an event
(`Life_Cascades.go:79`, after the respawn set). Everything else is silent, with
`NewRound_AutoHeal` emitting once at loop end gated on a net change. A helper
that emits would add ~78 new event emissions in U5b.

### Do NOT read the pool reservation

`GetPoolReservation(pool, max)` is consulted in only four places, none of them an
affordability check. Every cost site today compares against the **raw** pool. If
`CanAfford` subtracted reserve, every companion-holding character's costs would
change. That is U7/U8 work. U5a reads raw, deliberately, with a comment saying so.

---

## File Structure

**Created:**
- `internal/characters/pools.go` — `Pool`, `PoolValue`, `CanAfford`, `ApplyCost`,
  `ApplyCostPartial`, `ApplyHarm`.
- `internal/characters/pools_test.go` — the three floor rules, refusal vs
  partial-charge, and the defence-cost no-op.

**Modified:**
- `internal/characters/resources.go` — `GetDefenseStaminaCost` reads config.
- `internal/configs/config.balance.go` — three new base-cost fields.
- `internal/configs/config.balance.misc.go` — their validation defaults.
- `_datafiles/config.yaml` — the three knobs, documented in place.
- `internal/characters/character.go` — orphaned docstring cleanup.
- `internal/characters/context.md` — the new API and its gotchas.

---

### Task 0: Branch

- [ ] **Step 1: Create the branch**

```bash
git checkout -b feature/u5a-cost-harm-foundation
git status
```

Expected: `On branch feature/u5a-cost-harm-foundation`.

---

### Task 1: Clear the orphaned docstrings

Do this first: it is eight comment blocks in one file, zero risk, and it makes
the rest of the slice's diff readable. These are husks left by the 2026-04-17
split that moved the code to `resources.go` -- doc comments with **no function
under them**.

**Files:**
- Modify: `internal/characters/character.go`

- [ ] **Step 1: Delete the six husks at 691-703**

Open `internal/characters/character.go` and find the run of comments after the
`DefenseNone`/`DefenseDodge`/`DefenseParry`/`DefenseBlock` const block (around
line 691). Delete these comment lines **only** -- there is no code between them:

```go
// GetDefenseStaminaCost returns stamina cost for a defense type (Stage 7.1)

// DeductDefenseStamina deducts stamina for a defense and returns true if successful (Stage 7.1)

// GetToxicityMax returns the maximum toxicity this character can handle.
// Formula: BaseMax + Vitality / VitalityScale

// AddToxicity attempts to add toxicity. Returns false if it would exceed max.

// GetToxicityPenalties returns stat multipliers based on toxicity threshold.

// Where 1000 = a full round
```

The live functions are at `resources.go:126`, `:154`, `:261`, `:269`, `:305`,
`:365`. Note the `AddToxicity` husk is also **factually wrong**: the live
function always returns `true` and clamps to `[0, max]`.

- [ ] **Step 2: Delete the two husks at 439 and 450**

```go
// DeductStamina attempts to deduct the specified amount of stamina.
```

```go
// DeductAttackStamina deducts stamina for an attack and returns the actual cost deducted.
```

Both live at `resources.go:25` and `:111` with their own correct docstrings.

- [ ] **Step 3: Fix the mislabelled live function at 656-657**

This one is worse than an orphan: a wrong docstring sitting on a real function,
which reads as authoritative. Find:

```go
// AttemptRecovery tries to recover from a condition using a stat-based chance
// Formula: min(90, 25 + 20 * ln(statValue/25))
func (c *Character) SetSetting(...
```

Replace the two comment lines with a correct one for `SetSetting`. Read the
function body first and describe what it actually does.

- [ ] **Step 4: Verify nothing else was removed**

```bash
git diff --stat internal/characters/character.go
go build ./... && go test ./internal/characters/
gofmt -l internal/characters/
```

Expected: deletions only (plus the one docstring rewrite), build succeeds, tests
pass, gofmt silent.

- [ ] **Step 5: Commit**

```bash
git add internal/characters/character.go
git commit -m "docs(characters): delete eight orphaned docstrings and fix a mislabelled one (U5a)"
```

---

### Task 2: The pool helpers

**Files:**
- Create: `internal/characters/pools.go`
- Test: `internal/characters/pools_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/characters/pools_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
)

// poolChar builds a character with known pools. Base is set then Recalculate is
// called, because Value/ValueAdj are DERIVED -- assigning Value directly is
// silently wiped.
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

// TestApplyCost_RefusesAndTakesNothing is floor rule 1 for volitional actions.
// An unaffordable cost must not scrape the pool out -- treating unaffordability
// as anything except refusal is the bug this helper exists to stop.
func TestApplyCost_RefusesAndTakesNothing(t *testing.T) {
	for _, pool := range []Pool{PoolHealth, PoolStamina, PoolConviction} {
		c := poolChar(10, 10, 10)
		if c.ApplyCost(pool, 25) {
			t.Errorf("pool %v: an unaffordable cost reported success", pool)
		}
		if got := c.PoolValue(pool); got != 10 {
			t.Errorf("pool %v after a REFUSED cost: got %d, want 10 (untouched)", pool, got)
		}
	}
}

// TestApplyCost_PaysInFullWhenAffordable, including the exact-affordability
// boundary, which may legitimately land the pool on 0.
func TestApplyCost_PaysInFullWhenAffordable(t *testing.T) {
	c := poolChar(10, 10, 10)
	if !c.ApplyCost(PoolStamina, 4) {
		t.Error("affordable cost reported failure")
	}
	if c.Stamina != 6 {
		t.Errorf("stamina after paying 4: got %d want 6", c.Stamina)
	}
	if !c.ApplyCost(PoolStamina, 6) {
		t.Error("exactly-affordable cost reported failure")
	}
	if c.Stamina != 0 {
		t.Errorf("stamina after paying the exact remainder: got %d want 0", c.Stamina)
	}
}

// TestApplyCostPartial_ChargesWhatItCan pins the DEFENSIVE exception: dodge,
// parry, block and spell-resist always fire and charge the maximum available,
// because you do not choose to be attacked. A defence that simply did not
// happen would make an exhausted character a free target and would also remove
// them from the reach of the 5.9 contest floors.
func TestApplyCostPartial_ChargesWhatItCan(t *testing.T) {
	c := poolChar(10, 7, 10)
	if spent := c.ApplyCostPartial(PoolStamina, 25); spent != 7 {
		t.Errorf("partial charge: got %d want 7 (all that was there)", spent)
	}
	if c.Stamina != 0 {
		t.Errorf("stamina after a partial charge: got %d want 0", c.Stamina)
	}
	if spent := c.ApplyCostPartial(PoolStamina, 5); spent != 0 {
		t.Errorf("partial charge from an empty pool: got %d want 0", spent)
	}
	if c.Stamina != 0 {
		t.Errorf("partial charge drove the pool below zero: got %d want 0", c.Stamina)
	}
}

// TestCostsIgnoreNonPositive guards against a negative "cost" being used as a
// backdoor heal through either primitive.
func TestCostsIgnoreNonPositive(t *testing.T) {
	c := poolChar(10, 10, 10)
	if !c.ApplyCost(PoolStamina, -5) {
		t.Error("a non-positive cost is free, so it should succeed")
	}
	if spent := c.ApplyCostPartial(PoolStamina, -5); spent != 0 {
		t.Errorf("negative partial cost: got %d want 0", spent)
	}
	if c.Stamina != 10 {
		t.Errorf("a negative cost changed the pool: got %d want 10", c.Stamina)
	}
}

// TestCanAfford pins the refuse-if-poor semantics that DeductStamina and
// DeductDefenseStamina rely on. It reads the RAW pool, not reserve-excluded.
func TestCanAfford(t *testing.T) {
	c := poolChar(10, 10, 10)
	if !c.CanAfford(PoolStamina, 10) {
		t.Error("exact affordability should be true")
	}
	if c.CanAfford(PoolStamina, 11) {
		t.Error("over-affordability should be false")
	}
}

// TestApplyHarm_FloorsStaminaAndConviction is floor rule 2.
func TestApplyHarm_FloorsStaminaAndConviction(t *testing.T) {
	for _, pool := range []Pool{PoolStamina, PoolConviction} {
		c := poolChar(10, 10, 10)
		c.ApplyHarm(pool, 25, state.ActorRef{})
		if got := c.PoolValue(pool); got != 0 {
			t.Errorf("pool %v after overkill harm: got %d, want 0 (floored)", pool, got)
		}
	}
}

// TestApplyHarm_LeavesHealthNegative is floor rule 3, and it is the one that
// matters most. Health MUST be allowed below zero: that is how death works.
// validatePoolClamps carries an explicit "No lower Health clamp" comment, and
// the per-round death checks read the negative value. Clamping here destroys
// overkill magnitude and breaks anything measuring how far past zero a blow
// landed.
func TestApplyHarm_LeavesHealthNegative(t *testing.T) {
	c := poolChar(10, 10, 10)
	c.ApplyHarm(PoolHealth, 25, state.ActorRef{})
	if c.Health != -15 {
		t.Errorf("health after overkill: got %d, want -15 (NOT floored)", c.Health)
	}
}

// TestApplyHarm_ReturnsAppliedDelta pins the return contract. Callers such as
// DoCombat_unified keep a result struct in sync with the pool, so a floored
// harm must report what actually landed, not what was requested.
func TestApplyHarm_ReturnsAppliedDelta(t *testing.T) {
	c := poolChar(10, 10, 10)
	if applied := c.ApplyHarm(PoolStamina, 25, state.ActorRef{}); applied != 10 {
		t.Errorf("floored harm: returned %d, want 10 (the amount actually applied)", applied)
	}
	c2 := poolChar(10, 10, 10)
	if applied := c2.ApplyHarm(PoolHealth, 25, state.ActorRef{}); applied != 25 {
		t.Errorf("unfloored health harm: returned %d, want 25 (all of it landed)", applied)
	}
}

// TestApplyHarm_IgnoresNonPositive guards against negative harm healing.
func TestApplyHarm_IgnoresNonPositive(t *testing.T) {
	c := poolChar(10, 10, 10)
	if applied := c.ApplyHarm(PoolHealth, -5, state.ActorRef{}); applied != 0 {
		t.Errorf("negative harm: got %d want 0", applied)
	}
	if c.Health != 10 {
		t.Errorf("negative harm changed the pool: got %d want 10", c.Health)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/characters/ -run 'TestApplyCost|TestApplyHarm|TestCanAfford' -v
```

Expected: FAIL, `undefined: Pool` / `undefined: ApplyCost` and friends.

- [ ] **Step 3: Write the helpers**

Create `internal/characters/pools.go`:

```go
package characters

import "github.com/GoMudEngine/GoMud/internal/state"

// Pool identifies one of the three resource pools.
//
// The pools are plain int fields on Character (Health, Stamina, Conviction).
// Only the MAX values are StatInfo, read via .Value.
type Pool int

const (
	PoolHealth Pool = iota
	PoolStamina
	PoolConviction
)

// String makes Pool printable in test failures and log lines.
func (p Pool) String() string {
	switch p {
	case PoolHealth:
		return "health"
	case PoolStamina:
		return "stamina"
	case PoolConviction:
		return "conviction"
	}
	return "unknown"
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

// setPool writes a pool. Unexported on purpose: every caller should be going
// through ApplyCost or ApplyHarm so the floor rules cannot be bypassed.
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

// CanAfford reports whether the character could pay `amount` from `pool`.
//
// It reads the RAW pool, deliberately. GetPoolReservation is NOT consulted --
// no affordability check in the codebase consults it today, and making costs
// reserve-aware would change the cost of every action for every companion
// holder. That is U7/U8 work; doing it here would be an unannounced balance
// change.
//
// Pair this with ApplyCost to express refuse-if-poor semantics, as
// DeductStamina and DeductDefenseStamina do. Callers that should pay what they
// can, as DeductAttackStamina does, call ApplyCost alone.
func (c *Character) CanAfford(pool Pool, amount int) bool {
	if amount <= 0 {
		return true
	}
	return c.PoolValue(pool) >= amount
}

// ApplyCost pays `amount` from `pool` IN FULL or not at all, reporting whether
// it paid. This is the primitive for every VOLITIONAL action: attacking,
// casting, moving, standing, mutation moves.
//
// FLOOR RULE 1: a cost may never drive any pool below 0, including health. An
// action's price is not a way to kill someone. Harm is the only thing that may
// take health below zero -- see ApplyHarm.
//
// UNAFFORDABILITY IS REFUSAL. It does not scrape the pool out. The codebase used
// to do both -- DeductStamina refused, DeductAttackStamina zeroed the pool and
// reported what it managed to take -- and the second is a defect, not a
// semantic worth preserving. You chose to act; if you cannot pay, the action
// does not happen and nothing is taken.
//
// Paying a cost you can exactly afford legitimately lands the pool on 0. The
// rule forbids being scraped out, not reaching empty.
//
// A non-positive amount is free, so it succeeds and changes nothing. That stops
// a negative "cost" being used as a backdoor heal.
//
// If you are reaching for this to charge a DEFENCE, you want ApplyCostPartial.
//
// Deliberately emits no event and performs no validation. Exactly one pool
// mutation in the codebase emits an event today; making this one emit would add
// an emission at every site U5b touches.
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

// ApplyCostPartial charges up to `amount` from `pool`, taking whatever is there
// when the actor cannot pay in full, and returns the amount ACTUALLY charged.
//
// THIS IS FOR INVOLUNTARY DEFENSIVE AVOIDANCE ONLY -- dodge, parry, block,
// spell-resist. Every volitional action must use ApplyCost and refuse instead.
//
// The exception is deliberate and is the reason two primitives exist. You do not
// choose to be attacked. A defence that simply did not happen would make an
// exhausted character a free target, and worse, would remove them from the
// contest altogether and therefore from the reach of the 5.9 contest floors.
// Charging what they have and stripping the SKILL TERM from the roll is
// punishing without being hopeless. See spec section 3.4; applying that penalty
// is chunk U8, not this one.
//
// Never drives the pool below 0. A non-positive amount charges nothing.
func (c *Character) ApplyCostPartial(pool Pool, amount int) int {
	if amount <= 0 {
		return 0
	}

	current := c.PoolValue(pool)
	if current <= 0 {
		return 0
	}

	charged := amount
	if charged > current {
		charged = current
	}

	c.setPool(pool, current-charged)
	return charged
}

// ApplyHarm applies `amount` of harm from `source` to `pool` and returns the
// amount ACTUALLY applied.
//
// FLOOR RULE 2: harm floors stamina and conviction at 0.
//
// FLOOR RULE 3: harm does NOT floor health. Health must be allowed below zero,
// because that is how death works. validatePoolClamps carries an explicit
// "No lower Health clamp" comment, the per-round death checks read the value,
// and the magnitude past zero is real information -- U6's margin-scaled work
// wants it. Do not add a health floor here, however wrong the negative number
// looks in a debugger.
//
// The return value is the APPLIED delta, not the requested one. They differ
// whenever a floor bites. Callers that keep a result struct in sync with the
// pool -- NewRound_DoCombat_unified does -- must add the returned value, not
// the requested one, or reported damage diverges from damage dealt.
//
// `source` is unused in U5a and is present ON PURPOSE. U5c routes attributed
// death from the harm site, replacing the deferred round-tick sweep that today
// calls Die with an EMPTY ActorRef -- so grenades, damage-over-time and pathto
// attrition currently kill without credit. Taking the source now means U5b can
// route ~78 call sites once instead of twice.
//
// Deliberately does NOT call Die, cancel buffs, validate, or emit. In
// particular this is NOT built on ApplyHealthChange, which cancels combat buffs
// and triggers a full stat recalculation on crossing zero; that side effect
// reaches 8 call sites today and must not silently spread to the rest in U5b.
func (c *Character) ApplyHarm(pool Pool, amount int, source state.ActorRef) int {
	_ = source // U5c reads this; see the docstring above.

	if amount <= 0 {
		return 0
	}

	current := c.PoolValue(pool)

	if pool == PoolHealth {
		c.setPool(pool, current-amount)
		return amount
	}

	applied := amount
	if applied > current {
		applied = current
	}
	if applied < 0 {
		applied = 0
	}

	c.setPool(pool, current-applied)
	return applied
}
```

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/characters/ -run 'TestApplyCost|TestApplyHarm|TestCanAfford' -v
```

Expected: all PASS.

- [ ] **Step 5: Full package + gofmt**

```bash
go build ./... && go test ./internal/characters/
gofmt -l internal/characters/
```

Expected: build succeeds, whole package passes, gofmt silent.

> If `New()` in the test helper needs arguments, or `StatInfo.Recalculate` is
> spelled differently, read the real signatures and adjust the **test helper**
> only. Do not change the assertions.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/pools.go internal/characters/pools_test.go
git commit -m "feat(characters): add ApplyCost and ApplyHarm with the three floor rules (U5a)"
```

---

### Task 3: Move the defence base costs into config

The base costs `2 / 4 / 5` are hardcoded in Go at `resources.go:134/137/140`
while only their multipliers are config. That is the "no balance number inside
`internal/`" anti-pattern sitting in the code this arc is rewriting.

**This must stay a no-op**, so the shipped values are the current ones and the
arithmetic order is preserved exactly.

**The arithmetic is truncate-then-floor, in that order**, and it matters:
`int(2 × 0.9) = 1`, `int(4 × 0.9) = 3`, `int(5 × 0.9) = 4`. All three
multipliers ship at 0.9, so today's effective costs are **dodge 1, parry 3,
block 4**. Rounding instead of truncating would make dodge cost 2 -- a 100%
increase on the most-used defence in the game.

**Files:**
- Modify: `internal/configs/config.balance.go`
- Modify: `internal/configs/config.balance.misc.go`
- Modify: `_datafiles/config.yaml`
- Modify: `internal/characters/resources.go`

- [ ] **Step 1: Add the three fields**

In `internal/configs/config.balance.go`, next to the existing
`DodgeMultiplier` / `ParryMultiplier` / `BlockMultiplier` declarations:

```go
	DodgeBaseCost ConfigInt `yaml:"DodgeBaseCost"` // Base stamina cost for dodge before multiplier (default 2)
	ParryBaseCost ConfigInt `yaml:"ParryBaseCost"` // Base stamina cost for parry before multiplier (default 4)
	BlockBaseCost ConfigInt `yaml:"BlockBaseCost"` // Base stamina cost for block before multiplier (default 5)
```

Match the surrounding declarations' type spelling exactly -- if neighbours use a
different int config type, use theirs.

- [ ] **Step 2: Add the validation defaults**

In `internal/configs/config.balance.misc.go`, following the idiom already used
by the `Min*Chance` knobs in that file:

```go
	if b.DodgeBaseCost < 1 {
		b.DodgeBaseCost = 2
	}
	if b.ParryBaseCost < 1 {
		b.ParryBaseCost = 4
	}
	if b.BlockBaseCost < 1 {
		b.BlockBaseCost = 5
	}
```

A floor of 1 rather than 0: a zero base cost would make the defence free, and
the existing code already floors the final cost at 1.

- [ ] **Step 3: Read the config knob back out in `GetDefenseStaminaCost`**

In `internal/characters/resources.go`, replace the hardcoded literals:

```go
	switch defenseType {
	case DefenseDodge:
		baseCost = 2
		multiplier = float64(bal.DodgeMultiplier)
	case DefenseParry:
		baseCost = 4
		multiplier = float64(bal.ParryMultiplier)
	case DefenseBlock:
		baseCost = 5
		multiplier = float64(bal.BlockMultiplier)
	default:
		return 0
	}
```

with:

```go
	switch defenseType {
	case DefenseDodge:
		baseCost = int(bal.DodgeBaseCost)
		multiplier = float64(bal.DodgeMultiplier)
	case DefenseParry:
		baseCost = int(bal.ParryBaseCost)
		multiplier = float64(bal.ParryMultiplier)
	case DefenseBlock:
		baseCost = int(bal.BlockBaseCost)
		multiplier = float64(bal.BlockMultiplier)
	default:
		return 0
	}
```

**Leave the rest of the function exactly as it is.** The
`cost := int(float64(baseCost) * multiplier)` truncation and the `if cost < 1`
floor must not move or change form.

- [ ] **Step 4: Add the knobs to `config.yaml`, documented in place**

`_datafiles/config.yaml` has `skip-worktree` set. **Unset it, edit, stage, re-set
it**, and diff the staged result before committing -- committing this file has
leaked local dev settings to master before.

```bash
git update-index --no-skip-worktree _datafiles/config.yaml
```

Add next to `DodgeMultiplier` / `ParryMultiplier` / `BlockMultiplier`:

```yaml
  # DodgeBaseCost / ParryBaseCost / BlockBaseCost: the stamina a defence costs
  # BEFORE its multiplier. Moved out of Go by U5a; they were hardcoded in
  # GetDefenseStaminaCost, which is the exact anti-pattern the resolution arc
  # exists to remove.
  #
  # The arithmetic is int(base * multiplier), TRUNCATED, then floored at 1. With
  # the multipliers all shipping at 0.9 that makes the live costs dodge 1,
  # parry 3, block 4. Rounding instead of truncating would take dodge to 2, a
  # 100% rise on the most-used defence in the game.
  #
  # U7 replaces this whole shape: base cost becomes one term in a unified
  # formula with an encumbrance multiplier and an inverse-skill multiplier, and
  # these values become modifiers rather than bases. Do not retune them here in
  # the meantime; the retune is a modelled change, not a nudge.
  DodgeBaseCost: 2
  ParryBaseCost: 4
  BlockBaseCost: 5
```

- [ ] **Step 5: Prove the no-op numerically**

Add to `internal/characters/pools_test.go`:

```go
// TestDefenceBaseCostsAreUnchangedByConfigMove pins the exact costs across
// U5a's move of 2/4/5 out of Go and into config.
//
// A Go test binary never loads config.yaml, so these read the VALIDATION
// DEFAULTS from config.balance.misc.go -- which is precisely what makes this
// test meaningful: it proves the defaults match the literals that were deleted.
// The multipliers default to 0.9, and int(2*0.9)=1, int(4*0.9)=3, int(5*0.9)=4.
// Truncation, not rounding: rounding would make dodge cost 2.
func TestDefenceBaseCostsAreUnchangedByConfigMove(t *testing.T) {
	c := poolChar(10, 10, 10)
	for _, tc := range []struct {
		defense string
		want    int
	}{
		{DefenseDodge, 1},
		{DefenseParry, 3},
		{DefenseBlock, 4},
	} {
		if got := c.GetDefenseStaminaCost(tc.defense); got != tc.want {
			t.Errorf("%s cost: got %d, want %d -- the config move changed a live value",
				tc.defense, got, tc.want)
		}
	}
}
```

> If the shipped multipliers in `config.yaml` are not 0.9, this test still holds,
> because it runs against Go defaults. Confirm the defaults in
> `config.balance.misc.go` are 0.9 before trusting the expected values; if they
> differ, recompute `int(base × multiplier)` and use those numbers, stating the
> real defaults in the comment.

- [ ] **Step 6: Verify**

```bash
go build ./... && go test ./internal/characters/
gofmt -l internal/ modules/
grep -n "baseCost = [0-9]" internal/characters/resources.go
```

Expected: build succeeds, tests pass, gofmt silent, and the grep prints
**nothing** -- no hardcoded base cost survives.

- [ ] **Step 7: Stage carefully, then re-set skip-worktree**

```bash
git add internal/configs/config.balance.go internal/configs/config.balance.misc.go \
        internal/characters/resources.go internal/characters/pools_test.go
git add -p _datafiles/config.yaml
git diff --cached _datafiles/config.yaml
```

**Read that diff.** It must contain ONLY the three new knobs and their comment
block. If it shows `HttpPort`, `LogToFile`, or any other local setting, unstage
and redo -- that exact leak reached master on 2026-08-11.

```bash
git commit -m "feat(config): move the defence base costs out of Go and into config (U5a)"
git update-index --skip-worktree _datafiles/config.yaml
git status --short _datafiles/config.yaml
```

Expected: the final `git status` prints nothing.

---

### Task 4: Documentation

**Files:**
- Modify: `internal/characters/context.md`

- [ ] **Step 1: Document the new API**

Add `Pool`, `PoolValue`, `CanAfford`, `ApplyCost` and `ApplyHarm` to the Public
API section with their verified signatures, then add these Gotchas:

```markdown
- **Three floor rules, and they are not symmetric.** A COST may never drive any
  pool below 0. HARM floors stamina and conviction at 0. HARM does **not** floor
  health, because health going negative is how death works -- `validatePoolClamps`
  carries an explicit "No lower Health clamp" comment and the per-round death
  checks read the value. Do not add a health floor to `ApplyHarm`.
- **Unaffordability is REFUSAL, except for defence.** `ApplyCost` pays in full or
  takes nothing, and is the primitive for every volitional action. Scraping a
  pool out is a bug, not a semantic -- `DeductAttackStamina` used to do it.
- **`ApplyCostPartial` is for involuntary defensive avoidance ONLY** (dodge,
  parry, block, spell-resist). Those always fire and charge the maximum
  available, because you do not choose to be attacked: a defence that did not
  happen would make an exhausted character a free target and would remove them
  from the reach of the 5.9 contest floors. Their punishment is losing the skill
  term from the roll (spec 3.4, chunk U8), not losing the defence.
- **`CanAfford` reads the RAW pool, not reserve-excluded.** No affordability
  check in the codebase consults `GetPoolReservation` today. Making costs
  reserve-aware would change costs for every companion holder, so it is U7/U8
  work, not a tidy-up.
- **Both helpers return the APPLIED delta, not the requested amount.** They
  differ whenever a floor bites. A caller keeping a result struct in sync with
  the pool must add the return value.
- **`ApplyHarm` takes a source actor that U5a does not use.** U5c routes
  attributed death from the harm site; today the deferred round-tick sweep calls
  `Die` with an empty `ActorRef`, so grenades, DoTs and `pathto` attrition kill
  without credit. The parameter exists now so U5b routes each call site once.
- **Do not build on `ApplyHealthChange`.** It cancels combat buffs and triggers
  a full stat recalculation via `Validate(true)` when health crosses zero. That
  reaches 8 call sites today; spreading it to the rest would be a large silent
  change.
- **Neither helper emits an event.** Exactly one pool mutation in the codebase
  does (`Life_Cascades.go`, after the respawn set).
```

- [ ] **Step 2: Audit**

```bash
python tools/context_md_audit.py
```

Expected: zero phantom symbols for `internal/characters`.

- [ ] **Step 3: Full verification**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/characters/ ./internal/configs/ ./internal/combat/
```

Expected: gofmt silent, build succeeds, all pass.

**Known noise:** the `internal/relationships` test binary is quarantined by
Windows Defender. Expected and unrelated.

- [ ] **Step 4: Confirm nothing was routed**

```bash
grep -rn "ApplyCost\|ApplyHarm" internal/ modules/ --include=*.go | grep -v _test | grep -v "^internal/characters/pools.go"
```

Expected: **no output.** U5a adds the helpers and moves nothing. If a call site
appears here, it belongs to U5b.

- [ ] **Step 5: Boot test on NON-DEFAULT ports**

The user runs the local server continuously, and `_datafiles/config.yaml` holds
the live ports. Copying it verbatim makes the test either fail to bind or
silently target the running server.

```bash
git worktree add --detach C:/tmp/dogmud-u5a-boot HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-u5a-boot/_datafiles/config.yaml
```

Edit the worktree copy: shift `TelnetPort`, `LocalPort`, `HttpPort` and `AIPort`
to unused values, and set `Logging.LogToFile: false` (a fresh worktree has no
`_datafiles/logs` directory, and the server exits 1 if file logging is on).

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

- [ ] **Step 6: Commit**

```bash
git add internal/characters/context.md
git commit -m "docs(characters): document the pool helpers and the three floor rules (U5a)"
```

---

## Ship it

```bash
git push -u origin feature/u5a-cost-harm-foundation
gh pr create --repo pruuk/DOGMud --base master --head feature/u5a-cost-harm-foundation --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
gh pr merge  <n> --repo pruuk/DOGMud --merge --delete-branch
```

**Always pass `--repo pruuk/DOGMud`** -- `gh` defaults to the fork parent. Use
`--merge`, not `--squash`. A green `gh pr checks --watch` can return early, so
confirm which runs actually re-ran.

**Do not propose a deploy.** The arc is under a deploy gate until U0-U11 are
complete and playtested.

---

## Definition of done

1. `ApplyCost`, `ApplyHarm`, `CanAfford`, `PoolValue` and `Pool` exist, tested,
   with the three floor rules pinned by name.
2. `ApplyHarm` leaves health negative on overkill. There is a test asserting the
   exact negative value.
3. `grep -n "baseCost = [0-9]" internal/characters/resources.go` returns nothing;
   `DodgeBaseCost` / `ParryBaseCost` / `BlockBaseCost` exist in config with
   documentation next to their values.
4. `GetDefenseStaminaCost` still returns dodge 1, parry 3, block 4 under Go
   defaults, pinned by test. Truncate-then-floor order unchanged.
5. **No call site was routed.** The Task 4 Step 4 grep is empty.
6. Eight orphaned docstrings deleted; the `SetSetting` mislabel corrected.
7. `internal/characters/context.md` accurate; `context_md_audit.py` clean.
8. `config.yaml`'s staged diff contained only the three new knobs.
9. Boot test clean on non-default ports; `gofmt -l internal/ modules/` silent.
