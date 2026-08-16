package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// The two reserve vocabularies used to be keyed differently, and a 2026-08-16
// re-check playtest caught them contradicting each other at the same instant:
// with health at roughly two fifths of the pool and conviction at roughly two
// thirds, the equip line said "a significant portion" of health and "a heavy
// share" of conviction while the status sheet said `heavy` and `near limit`.
// One measured the pool, the other measured the ceiling.
//
// These tests exist so they cannot drift apart again.

// rungOfProse and rungOfShort recover a rung index from a returned string
// WITHOUT asking either band function which rung it meant. That is the whole
// point: if one function is re-keyed off the pool again, the two indices stop
// agreeing and this test says so.
func rungOfProse(t *testing.T, s string) int {
	t.Helper()
	for i, rung := range reserveLadder {
		if rung.prose == s {
			return i
		}
	}
	t.Fatalf("prose band %q is not a rung of reserveLadder at all", s)
	return -1
}

func rungOfShort(t *testing.T, s string) int {
	t.Helper()
	for i, rung := range reserveLadder {
		if rung.short == s {
			return i
		}
	}
	t.Fatalf("short band %q is not a rung of reserveLadder at all", s)
	return -1
}

// The core guard: one reserved state, one rung, whichever vocabulary asks.
func TestReserveBands_ShareAndNameAgreeOnEveryRung(t *testing.T) {
	const maxPool = 1000
	cap := reservationCapFor(maxPool)
	if cap <= 0 || cap >= maxPool {
		t.Fatalf("fixture: a ceiling of %d on a pool of %d proves nothing", cap, maxPool)
	}

	seen := map[int]bool{}
	for reserve := 0; reserve <= maxPool; reserve++ {
		prose := ReserveShareBand(reserve, maxPool)
		short := reservationBand(reserve, maxPool, cap)

		pRung := rungOfProse(t, prose)
		sRung := rungOfShort(t, short)
		if pRung != sRung {
			t.Fatalf("at a reservation of %d of %d the two vocabularies disagree: "+
				"prose %q is rung %d, status word %q is rung %d. They must be two "+
				"spellings of ONE ladder",
				reserve, maxPool, prose, pRung, short, sRung)
		}
		seen[pRung] = true
	}

	// Every rung has to be reachable, or an agreement test could pass by never
	// exercising the rungs that disagreed.
	for i := range reserveLadder {
		if !seen[i] {
			t.Errorf("rung %d (%q / %q) is unreachable on a pool of %d",
				i, reserveLadder[i].short, reserveLadder[i].prose, maxPool)
		}
	}
}

// Both vocabularies must be keyed to the CEILING, not the pool. Pinned as an
// absolute so a re-key back to pool fractions cannot pass by moving both halves
// together.
func TestReserveBands_AreMeasuredAgainstTheCeiling(t *testing.T) {
	const maxPool = 100
	cap := reservationCapFor(maxPool)

	// A reservation of exactly the ceiling is the top rung in both spellings,
	// even though it is nowhere near the whole pool.
	if got := ReserveShareBand(cap, maxPool); got != reserveLadder[rungAtLimit].prose {
		t.Errorf("prose band at exactly the ceiling = %q, want %q (keyed to the pool again?)",
			got, reserveLadder[rungAtLimit].prose)
	}
	if got := reservationBand(cap, maxPool, cap); got != reserveLadder[rungAtLimit].short {
		t.Errorf("status band at exactly the ceiling = %q, want %q",
			got, reserveLadder[rungAtLimit].short)
	}

	// And half the POOL is well up the ladder, because it is three quarters of
	// the ceiling. Under the old pool-keyed prose this was only the middle rung.
	if got, want := ReserveShareBand(maxPool/2, maxPool), reserveLadder[rungNearLimit].prose; got != want {
		t.Errorf("prose band at half the pool = %q, want %q", got, want)
	}
}

// The playtest's exact instant, driven through a real character: the equip line
// and the status row must name the same rung for each pool at the same moment.
func TestReserveBands_EquipLineAgreesWithTheStatusRow(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		// Roughly the observed state: health a little under half the pool,
		// conviction most of the way to the ceiling.
		999952: {
			ItemId:               999952,
			Name:                 "playtest collar",
			Type:                 items.Neck,
			ReserveHealthPct:     0.40,
			ReserveConvictionPct: 0.63,
		},
	})()

	c := newReservationNoticeChar()
	before := c.ReservationTotals()

	c.Equipment.Neck = items.New(999952)
	c.Validate()

	notice := c.ReservationIncreaseNotice(before)
	if notice == `` {
		t.Fatal("fixture: the collar reserved nothing, so there is no line to compare")
	}

	for _, pool := range []string{"health", "conviction"} {
		short := c.ReservationBandName(pool)
		wantProse := reserveLadder[rungOfShort(t, short)].prose

		fragment := wantProse + ` of your ` + pool
		if !strings.Contains(notice, fragment) {
			t.Errorf("status says %s is %q, so the equip line must read %q. It says:\n%s",
				pool, short, fragment, notice)
		}
	}
}
