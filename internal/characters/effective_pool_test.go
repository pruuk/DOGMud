package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// EffectivePoolMax is the denominator for every percentage-OF-MAX threshold in
// the game. Measuring one against the RAW max compares it to a current value
// that RecalculateStats has already clamped to max - reservation, which charges
// the reserve twice -- the `stand` lockout being the case where the double
// charge made the gate literally unsatisfiable.
//
// Setup note (same as pool_reservation_pinnacle_test.go): no config file is
// loaded in this package's tests, so every Balance multiplier feeding
// RecalculateStats' pool-max derivation is 0. RecalculateStats only writes
// .Mods; setting .Base gives a deterministic post-Validate() pool max.

// With nothing reserving the pool, the effective max IS the max. This is the
// no-op case that must stay free of surprises: the overwhelming majority of
// characters have no reservation at all, and routing them through this helper
// must not move a single number for them.
func TestEffectivePoolMax_NoReservationEqualsMax(t *testing.T) {
	c := New()
	c.StaminaMax.Base = 100
	c.HealthMax.Base = 400
	c.ConvictionMax.Base = 200
	c.Validate()

	for _, tc := range []struct {
		pool Pool
		want int
	}{
		{PoolStamina, 100},
		{PoolHealth, 400},
		{PoolConviction, 200},
	} {
		if got := c.EffectivePoolMax(tc.pool); got != tc.want {
			t.Errorf("EffectivePoolMax(%s) with no reservation = %d, want %d (the raw max)", tc.pool, got, tc.want)
		}
	}
}

// With a reservation the effective max is max - reserve, and is strictly less
// than the raw max. "Strictly less" is the assertion that actually matters: it
// is the difference between a threshold the character can reach and one they
// cannot.
func TestEffectivePoolMax_ExcludesReservation(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999920: {
			ItemId:            999920,
			Name:              "leeching harness",
			Type:              items.Neck,
			ReserveStaminaPct: 0.30,
		},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999920)
	c.Validate()

	if c.StaminaMax.Value != 100 {
		t.Fatalf("test setup invariant broken: StaminaMax.Value = %d, want 100", c.StaminaMax.Value)
	}
	if res := c.GetPoolReservation("stamina", c.StaminaMax.Value); res != 30 {
		t.Fatalf("test setup invariant broken: reservation = %d, want 30", res)
	}

	eff := c.EffectivePoolMax(PoolStamina)
	if eff != 70 {
		t.Errorf("EffectivePoolMax(stamina) = %d, want 70 (100 max - 30 reserved)", eff)
	}
	if eff >= c.StaminaMax.Value {
		t.Errorf("EffectivePoolMax(stamina) = %d must be strictly less than the raw max %d", eff, c.StaminaMax.Value)
	}

	// The pool the character can actually fill is exactly the effective max --
	// which is the whole reason a threshold must be measured against it.
	c.Stamina = 999999
	c.Validate()
	if c.Stamina != eff {
		t.Errorf("reserve-clamped current stamina = %d, want %d (the effective max)", c.Stamina, eff)
	}
}

// Reservation meeting or exceeding the max floors the result at 0 rather than
// going negative. A negative denominator would flow straight into
// ResourceMultiplier and the stand thresholds and produce nonsense (a negative
// threshold is trivially "affordable", which is the opposite of the bug being
// fixed but just as wrong).
func TestEffectivePoolMax_FloorsAtZero(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999921: {ItemId: 999921, Name: "greedy collar", Type: items.Neck, ReserveStaminaPct: 0.60},
		999922: {ItemId: 999922, Name: "greedy belt", Type: items.Belt, ReserveStaminaPct: 0.60},
	})()

	t.Run("reservation_equals_max", func(t *testing.T) {
		c := New()
		c.StaminaMax.Base = 100
		c.Equipment.Neck = items.New(999921)
		c.Equipment.Belt = items.New(999922)
		// 60 + 60 = 120 reserved against a 100 max.
		c.Validate()

		if res := c.GetPoolReservation("stamina", c.StaminaMax.Value); res <= c.StaminaMax.Value {
			t.Fatalf("test setup invariant broken: reservation %d must exceed max %d", res, c.StaminaMax.Value)
		}
		if got := c.EffectivePoolMax(PoolStamina); got != 0 {
			t.Errorf("EffectivePoolMax(stamina) = %d with over-full reservation, want 0 (floored, never negative)", got)
		}
	})

	// A zero pool. Deliberately NOT Validate()d: validatePoolClamps floors a
	// pool max at 1, so validating here would test the clamp rather than this
	// function's own floor.
	t.Run("no_pool_at_all", func(t *testing.T) {
		c := New()
		c.StaminaMax.Value = 0
		if got := c.EffectivePoolMax(PoolStamina); got != 0 {
			t.Errorf("EffectivePoolMax(stamina) = %d on a zero pool, want 0", got)
		}
	})
}
