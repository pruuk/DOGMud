package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// #285: every write that raises a pool stops at PoolCeiling (raw max less the
// reservation), so the prompt, status and GMCP never show a pool fuller than
// the reservation clamp in RecalculateStats would leave it.

const reservingNeckItemId = 999285

// newReservedChar returns a character with 400 health, 200 stamina and 200
// conviction, a worn neck item reserving 25% of each, and pools emptied.
// Ceilings: health 300, stamina 150, conviction 150.
func newReservedChar(t *testing.T) *Character {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		reservingNeckItemId: {
			ItemId:               reservingNeckItemId,
			Name:                 "grasping torc",
			Type:                 items.Neck,
			ReserveHealthPct:     0.25,
			ReserveStaminaPct:    0.25,
			ReserveConvictionPct: 0.25,
		},
	}))

	c := New()
	c.HealthMax.Base = 400
	c.StaminaMax.Base = 200
	c.ConvictionMax.Base = 200
	c.Equipment.Neck = items.New(reservingNeckItemId)
	c.Validate()
	c.Health, c.Stamina, c.Conviction = 10, 10, 10

	for _, tc := range []struct {
		p    Pool
		want int
	}{{PoolHealth, 300}, {PoolStamina, 150}, {PoolConviction, 150}} {
		if got := c.PoolCeiling(tc.p); got != tc.want {
			t.Fatalf("setup: PoolCeiling(%s) = %d, want %d", tc.p, got, tc.want)
		}
	}
	return c
}

// A restore larger than the room left (a regen tick, a potion's condition
// tick, a heal, lifesteal) fills to the ceiling and no further, and reports
// only what it applied.
func TestApplyRestore_StopsAtReservedCeiling(t *testing.T) {
	c := newReservedChar(t)
	for _, p := range []Pool{PoolHealth, PoolStamina, PoolConviction} {
		ceiling := c.PoolCeiling(p)
		applied := c.ApplyRestore(p, 10000)
		if got := c.PoolValue(p); got != ceiling {
			t.Errorf("%s after a large restore = %d, want the reserved ceiling %d (raw max %d)", p, got, ceiling, c.poolMax(p))
		}
		if applied != ceiling-10 {
			t.Errorf("%s restore reported %d applied, want %d", p, applied, ceiling-10)
		}
	}

	// The clamp agrees with the ceiling: recalculating moves nothing.
	h, s, cv := c.Health, c.Stamina, c.Conviction
	c.Validate()
	if c.Health != h || c.Stamina != s || c.Conviction != cv {
		t.Errorf("Validate moved pools a restore filled: %d/%d/%d became %d/%d/%d", h, s, cv, c.Health, c.Stamina, c.Conviction)
	}
}

// A pool already above the ceiling (a reservation that grew since the last
// clamp) is not lowered by a restore: a restore never harms.
func TestApplyRestore_NeverLowersAPoolAboveTheCeiling(t *testing.T) {
	c := newReservedChar(t)
	c.Health = 350
	if applied := c.ApplyRestore(PoolHealth, 20); applied != 0 || c.Health != 350 {
		t.Errorf("restore over the ceiling: applied %d, health %d; want 0 and 350", applied, c.Health)
	}
}

// A companion's conviction reserve is a reservation like any other, and it
// can hold the whole pool: the restore ceiling is then 0, not the 1 that
// EffectivePoolMax floors its denominator at.
func TestApplyRestore_FullyReservedConvictionStaysEmpty(t *testing.T) {
	c := New()
	c.ConvictionMax.Base = 100
	c.Validate()
	c.Companions = []CompanionInfo{{ConvictionReserve: 150}}
	c.Conviction = 0
	if applied := c.ApplyRestore(PoolConviction, 5); applied != 0 || c.Conviction != 0 {
		t.Errorf("fully reserved conviction: applied %d, conviction %d; want 0 and 0", applied, c.Conviction)
	}
}

// Spawn, respawn, revive and admin restore set a pool to a fraction of its raw
// max; "full" for a reserved actor is the ceiling.
func TestSetPoolFraction_CapsAtReservedCeiling(t *testing.T) {
	c := newReservedChar(t)
	if got := c.SetPoolFraction(PoolHealth, 1.0); got != 300 || c.Health != 300 {
		t.Errorf("full health = %d (wrote %d), want the ceiling 300", c.Health, got)
	}
	if got := c.SetPoolFraction(PoolStamina, 0.5); got != 100 {
		t.Errorf("half stamina = %d, want 100 (half the raw 200, under the 150 ceiling)", got)
	}
	if got := c.SetPoolFraction(PoolConviction, 0.05); got != 10 {
		t.Errorf("5%% conviction = %d, want 10", got)
	}
}

// With no reservation the ceiling is the raw max, so unreserved actors see no
// change at all.
func TestPoolCeiling_UnreservedIsRawMax(t *testing.T) {
	c := New()
	c.HealthMax.Base = 400
	c.Validate()
	c.Health = 390
	if got := c.PoolCeiling(PoolHealth); got != 400 {
		t.Fatalf("unreserved ceiling = %d, want 400", got)
	}
	if applied := c.ApplyRestore(PoolHealth, 50); applied != 10 || c.Health != 400 {
		t.Errorf("unreserved restore: applied %d, health %d; want 10 and 400", applied, c.Health)
	}
}
