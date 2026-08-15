package characters

import (
	"math"
	"testing"
)

// A 1.5-per-action cost must average 1.5. Rounding every action up erases the
// per-action modifiers and overcharges; truncating every action erases them and
// undercharges. Carrying the remainder does neither.
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

// The dial must survive: three costs differing by 14% must produce different
// totals over a run of actions. This is the whole reason the carry exists.
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
	dodge := spend(2.5) // base 2 x 1.25
	parry := spend(2.2) // base 2 x 1.10
	block := spend(2.3) // base 2 x 1.15

	if !(parry < block && block < dodge) {
		t.Fatalf("modifier ordering lost: parry=%d block=%d dodge=%d", parry, block, dodge)
	}
}

// A cost may never drive a pool below zero, and a short charge must say so.
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

// The carry is per pool: spending stamina must not bleed a remainder into
// conviction.
func TestApplyCostFloatCarryIsPerPool(t *testing.T) {
	c := New()
	c.StaminaMax.Value = 1000
	c.Stamina = 1000
	c.ConvictionMax.Value = 1000
	c.Conviction = 1000

	// Bank a 0.5 remainder on stamina.
	c.ApplyCostFloat(PoolStamina, 1.5)
	// A 1.5 conviction charge must behave as a fresh 1.5, not 2.0.
	c.ApplyCostFloat(PoolConviction, 1.5)

	if spent := 1000 - c.Conviction; spent != 1 {
		t.Fatalf("first conviction charge of 1.5 should take 1, took %d", spent)
	}
}

// Zero and negative amounts are free and bank nothing.
func TestApplyCostFloatIgnoresNonPositive(t *testing.T) {
	c := New()
	c.StaminaMax.Value = 100
	c.Stamina = 100

	c.ApplyCostFloat(PoolStamina, 0)
	c.ApplyCostFloat(PoolStamina, -5)

	if c.Stamina != 100 {
		t.Fatalf("non-positive charges must be free, pool is %d", c.Stamina)
	}
}

// PINS A DELIBERATE DECISION, not an accident: a sub-integer charge takes
// nothing from the pool and is NOT Short.
//
// Until this test existed nothing exercised a sub-integer amount at all -- every
// other test in this file charges at least 1.5 -- even though banking the
// fraction is the entire mechanism ApplyCostFloat exists for. Short means "the
// actor could not pay what this action demanded", and a floored charge of zero
// demanded nothing. U8 reads Short to strip the skill term from the action's
// roll, so flipping this to true would penalise an action that cost nothing, and
// would penalise the same fraction AGAIN on the action that later floors it.
func TestApplyCostFloatSubIntegerChargesAreFreeAndNotShort(t *testing.T) {
	c := New()
	c.StaminaMax.Value = 1000
	c.Stamina = 1000

	wantCharged := []int{0, 0, 0, 1}
	for i, want := range wantCharged {
		res := c.ApplyCostFloat(PoolStamina, 0.3)
		if res.Charged != want {
			t.Errorf("charge %d of 0.3: Charged=%d, want %d", i+1, res.Charged, want)
		}
		if res.Short {
			t.Errorf("charge %d of 0.3 on a full pool must not be Short", i+1)
		}
	}

	if spent := 1000 - c.Stamina; spent != 1 {
		t.Fatalf("four charges of 0.3 should total 1, totalled %d", spent)
	}
}

// PINS A DELIBERATE DECISION: on an EMPTY pool a sub-integer charge is still not
// Short. The fraction is banked silently and reported exactly once, at the
// action that floors it to 1 and genuinely fails to pay it.
//
// This is the half of the "zero floored charge is not Short" rule that is easy
// to get wrong, because an empty pool makes reporting Short look right. It is
// not: reporting it on charges 1 and 2 would strip the skill term from two
// actions that cost the actor nothing, and then strip it a third time on the
// charge that actually fails -- three penalties for 1.2 stamina of demand.
func TestApplyCostFloatShortIsReportedOnceOnAnEmptyPool(t *testing.T) {
	c := New()
	c.StaminaMax.Value = 100
	c.Stamina = 0

	wantShort := []bool{false, false, true}
	for i, want := range wantShort {
		res := c.ApplyCostFloat(PoolStamina, 0.4)
		if res.Short != want {
			t.Errorf("charge %d of 0.4 on an empty pool: Short=%v, want %v", i+1, res.Short, want)
		}
		if res.Charged != 0 {
			t.Errorf("charge %d of 0.4 on an empty pool: Charged=%d, want 0", i+1, res.Charged)
		}
	}
}

// PINS A DELIBERATE DECISION: the unpaid part of a short charge is WRITTEN OFF,
// not re-banked into the carry.
//
// pool=1, charge 3.5 -> the carry hands 3 to the pool, the pool can only pay 1,
// and the 2 it could not pay is gone. The carry keeps only the 0.5 fraction it
// never handed over. Re-banking the 2 would give an exhausted actor an unbounded
// debt and slam them with the whole backlog on the first tick their pool
// refilled; being short is already punished by the lost skill term and must not
// also become a loan.
//
// Reads the unexported costCarry directly -- there is no accessor, and the
// difference between writing the debt off and re-banking it is invisible from
// the pool value alone.
func TestApplyCostFloatWritesOffTheUnpaidPart(t *testing.T) {
	c := New()
	c.StaminaMax.Value = 100
	c.Stamina = 1

	res := c.ApplyCostFloat(PoolStamina, 3.5)

	if res.Charged != 1 || !res.Short {
		t.Fatalf("got Charged=%d Short=%v, want Charged=1 Short=true", res.Charged, res.Short)
	}
	if c.Stamina != 0 {
		t.Fatalf("pool is %d, want 0", c.Stamina)
	}
	if got := c.costCarry[PoolStamina]; math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("carry is %v, want 0.5 -- the 2 the pool could not pay must be written off, not re-banked", got)
	}
}

// A non-finite cost must be free and must NOT touch the carry.
//
// NaN fails every comparison, so `amount <= 0` alone does not stop it, and
// `debt - math.Floor(debt)` is NaN for NaN and also NaN for +Inf (Inf-Inf). The
// carry entry would then be NaN forever: every later charge floors to NaN, fails
// `whole <= 0`, and int(NaN) converts to the minimum int64, which
// ApplyCostPartial reads as non-positive and treats as free. One non-finite
// value would make that pool cost-free for the rest of the session, with no log,
// no panic and nothing failing. A cost is a product of four config-sourced
// floats, so it is reachable.
func TestApplyCostFloatIgnoresNonFinite(t *testing.T) {
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		c := New()
		c.StaminaMax.Value = 1000
		c.Stamina = 1000

		if res := c.ApplyCostFloat(PoolStamina, bad); res.Charged != 0 || res.Short {
			t.Errorf("%v: got Charged=%d Short=%v, want free", bad, res.Charged, res.Short)
		}

		// The pool must still be chargeable afterwards: this is the part that
		// silently broke.
		if res := c.ApplyCostFloat(PoolStamina, 5.0); res.Charged != 5 {
			t.Errorf("after %v, a 5.0 charge took %d, want 5 -- the carry was poisoned", bad, res.Charged)
		}
		if c.Stamina != 995 {
			t.Errorf("after %v, pool is %d, want 995", bad, c.Stamina)
		}
	}
}
