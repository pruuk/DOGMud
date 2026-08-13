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

// TestSignedTickSplit_NegativeStillHarms guards the U5b-1 trap: a signed tick
// amount routed only through ApplyRestore would silently delete every
// damage-over-time buff, because ApplyRestore no-ops on non-positive input.
//
// The body mirrors the sign split now live in NewRound_UserRoundTick.go and
// NewRound_MobRoundTick.go. buffs.ComputeTickAmount returns -amount for
// TickPercent < 0, so both directions arrive at the same switch.
func TestSignedTickSplit_NegativeStillHarms(t *testing.T) {
	c := poolChar(10, 10, 10)
	tick := -4
	if tick > 0 {
		c.ApplyRestore(PoolStamina, tick)
	} else if tick < 0 {
		c.ApplyHarm(PoolStamina, -tick, state.ActorRef{})
	}
	if c.Stamina != 6 {
		t.Errorf("negative tick: stamina %d, want 6 -- the DoT path is broken", c.Stamina)
	}
	if got := c.ApplyRestore(PoolStamina, -4); got != 0 {
		t.Errorf("ApplyRestore must no-op on negative input, got %d -- if this "+
			"changes, the sign split above is no longer load-bearing", got)
	}
}
