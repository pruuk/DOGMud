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
// 13-character column the template reserves for it, and EVERY rung keys off the
// CAP rather than the pool, so the row reports the headroom a player has left
// rather than a share of a ceiling they can never use.
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
		{5, "slight"},      // 8% of the cap
		{15, "modest"},     // 23%
		{30, "notable"},    // 45%
		{40, "heavy"},      // 61%
		{55, "near limit"}, // 83%
		{66, "at limit"},   // the cap itself
		{90, "at limit"},   // grandfathered, well past it
	} {
		if got := reservationBand(tc.reserve, 100, 66); got != tc.want {
			t.Errorf("reservationBand(%d, 100, 66) = %q, want %q", tc.reserve, got, tc.want)
		}
		if len(tc.want) > 13 {
			t.Errorf("band %q is %d chars and will break the status box", tc.want, len(tc.want))
		}
	}
}

// The defect this exists to prevent. A 2026-08-15 playtest watched the Reserved
// row read `notable` through THREE consecutive refusals.
//
// The row's entire job is telling a player how much room they have left, so a
// character close enough to the ceiling that the next ordinary addition is
// refused must not be described in the comfortable middle of the ladder. The
// old ladder measured the middle rungs against the full POOL while the ceiling
// sat at two thirds of it, so two thirds of the way to the ceiling still read
// as under half of the pool.
func TestReservationBandName_WarnsBeforeTheCeiling(t *testing.T) {
	const (
		pool = 500
		cap  = 330 // ReservationCap: floor(500 * 0.66)
	)

	// A companion of the kind U7b prices at roughly a quarter of a 500 pool.
	const nextAddition = 126

	// The band cannot know what the player is about to add, so it is not asked
	// to predict a specific refusal. What it must never do is describe someone
	// out of room as comfortable. Walk the whole pool: every reservation big
	// enough to be refused its next companion has to sit in the crowded end of
	// the ladder, and no reservation with room to spare may claim the ceiling.
	crowded := map[string]bool{"heavy": true, "near limit": true, "at limit": true}

	for reserve := 0; reserve <= cap; reserve++ {
		band := reservationBand(reserve, pool, cap)
		refused := reserve+nextAddition > cap

		if refused && !crowded[band] {
			t.Fatalf("reserve %d of pool %d (cap %d) is refused its next companion "+
				"but the status row says %q. The row has to warn before the ceiling, "+
				"not after it.", reserve, pool, cap, band)
		}
		if !refused && band == "at limit" {
			t.Fatalf("reserve %d of pool %d (cap %d) still has room for its next "+
				"companion but the status row says %q", reserve, pool, cap, band)
		}
	}

	// And the ceiling itself must say so, which is the case the playtest never
	// once saw.
	if got := reservationBand(cap, pool, cap); got != "at limit" {
		t.Errorf("reservationBand at exactly the cap = %q, want \"at limit\"", got)
	}
}

// End to end through the real character, not just the helper: a pool two thirds
// reserved is at its ceiling and the sheet must say so. Before U7b's display
// fix this read "heavy", one rung short of the top, because 66 of 100 is only
// two thirds of the POOL.
func TestReservationBandName_AtTheCapThroughTheCharacter(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999931: {ItemId: 999931, Name: "heavy collar", Type: items.Neck, ReserveStaminaPct: 0.66},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Neck = items.New(999931)
	c.Validate()

	if res := c.GetPoolReservation("stamina", 100); res != 66 {
		t.Fatalf("fixture: reservation = %d, want 66", res)
	}
	if got := c.ReservationBandName("stamina"); got != "at limit" {
		t.Errorf("ReservationBandName(stamina) at the cap = %q, want \"at limit\"", got)
	}
}
