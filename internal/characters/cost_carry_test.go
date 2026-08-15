package characters

import "testing"

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
