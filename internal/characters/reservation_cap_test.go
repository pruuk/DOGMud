package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Setup note (mirrors effective_pool_test.go): no config file is loaded in this
// package's tests, so every Balance multiplier feeding RecalculateStats' pool-max
// derivation is 0. RecalculateStats only writes .Mods, so setting .Base gives a
// deterministic post-Validate() pool max.

// The cap is a flat fraction of the pool's max, per pool.
func TestReservationCap_IsAFractionOfMax(t *testing.T) {
	c := New()
	c.StaminaMax.Base = 100
	c.HealthMax.Base = 400
	c.ConvictionMax.Base = 500
	c.Validate()

	for _, tc := range []struct {
		pool Pool
		want int
	}{
		{PoolStamina, 66},
		{PoolHealth, 264},
		{PoolConviction, 330},
	} {
		if got := c.ReservationCap(tc.pool); got != tc.want {
			t.Errorf("ReservationCap(%s) = %d, want %d", tc.pool, got, tc.want)
		}
	}
}

// An addition that would carry total reservation past the cap must be reported
// as a breach; one that lands inside it must not.
func TestWouldBreachReservationCap_Boundary(t *testing.T) {
	c := New()
	c.StaminaMax.Base = 100
	c.Validate()

	if c.WouldBreachReservationCap(PoolStamina, 66) {
		t.Errorf("adding exactly the cap must be allowed, not refused")
	}
	if !c.WouldBreachReservationCap(PoolStamina, 67) {
		t.Errorf("adding one past the cap must be refused")
	}
	// A zero or negative addition is never a breach: removing gear must never
	// be blocked by the ceiling, however far over the character already is.
	if c.WouldBreachReservationCap(PoolStamina, 0) {
		t.Errorf("a zero addition must never breach")
	}
	if c.WouldBreachReservationCap(PoolStamina, -50) {
		t.Errorf("a negative addition must never breach")
	}
}

// D4 grandfathering. A character ALREADY past the cap keeps everything they
// have, and is refused only ADDITIONS. Nothing here may force a removal.
func TestWouldBreachReservationCap_GrandfathersTheAlreadyOver(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999930: {ItemId: 999930, Name: "greedy collar", Type: items.Neck, ReserveStaminaPct: 0.80},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999930)
	c.Validate()

	if res := c.GetPoolReservation("stamina", 100); res != 80 {
		t.Fatalf("fixture: reservation = %d, want 80", res)
	}
	if !c.WouldBreachReservationCap(PoolStamina, 1) {
		t.Errorf("an over-cap character must be refused every addition")
	}
	if c.WouldBreachReservationCap(PoolStamina, 0) {
		t.Errorf("an over-cap character must NOT be forced to shed anything")
	}
}

// The overage snapshot is the seam the equip path uses. It must report zero for
// a pool inside the cap, the excess for one outside it, and Worsened must fire
// only when a pool's overage GREW -- which is what lets an over-cap character
// swap one reserving item for an equal one.
func TestReservationOverages_WorsenedOnlyOnGrowth(t *testing.T) {
	before := ReservationSnapshot{Health: 0, Stamina: 10, Conviction: 0}

	same := ReservationSnapshot{Health: 0, Stamina: 10, Conviction: 0}
	if _, worse := before.Worsened(same); worse {
		t.Errorf("an unchanged overage must not be reported as worse")
	}

	better := ReservationSnapshot{Health: 0, Stamina: 4, Conviction: 0}
	if _, worse := before.Worsened(better); worse {
		t.Errorf("a shrinking overage must not be reported as worse")
	}

	worseSnap := ReservationSnapshot{Health: 0, Stamina: 11, Conviction: 0}
	p, worse := before.Worsened(worseSnap)
	if !worse || p != PoolStamina {
		t.Errorf("a growing stamina overage must report (PoolStamina, true), got (%s, %v)", p, worse)
	}

	// A pool that goes from inside the cap to outside it is the ordinary case.
	fresh := ReservationSnapshot{}
	p, worse = fresh.Worsened(ReservationSnapshot{Health: 3})
	if !worse || p != PoolHealth {
		t.Errorf("a newly-breached health pool must report (PoolHealth, true), got (%s, %v)", p, worse)
	}
}

// The short band vocabulary used by the status sheet. Every label must fit the
// 13-character column the template reserves for it, and the top band must key
// off the CAP rather than a fixed fraction, so a player can see when they have
// no room left to add.
func TestReservationBandName_ShortVocabulary(t *testing.T) {
	c := New()
	c.StaminaMax.Base = 100
	c.Validate()

	if got := c.ReservationBandName("stamina"); got != "none" {
		t.Errorf("an unreserved pool = %q, want \"none\"", got)
	}

	for _, tc := range []struct {
		reserve int
		want    string
	}{
		{0, "none"},
		{10, "slight"},
		{20, "modest"},
		{40, "notable"},
		{60, "heavy"},
		{66, "at limit"},
		{90, "at limit"},
	} {
		if got := reservationBand(tc.reserve, 100, 66); got != tc.want {
			t.Errorf("reservationBand(%d, 100, 66) = %q, want %q", tc.reserve, got, tc.want)
		}
		if len(tc.want) > 13 {
			t.Errorf("band %q is %d chars and will break the status box", tc.want, len(tc.want))
		}
	}
}
