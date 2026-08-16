package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Equipping something that sets part of you aside used to say NOTHING.
//
// A 2026-08-15 playtest put on an item that took health, stamina and conviction
// into reserve, was told nothing at all, and then hit a refusal several actions
// later with no way to connect the two. The equip is the only moment where
// cause and effect sit next to each other, so it is the only place a disclosure
// is worth anything.

func seedReservingGear(t *testing.T) func() {
	t.Helper()
	return items.SeedItemsForTest(map[int]*items.ItemSpec{
		// Takes a slice of all three pools, like the playtest's item.
		999950: {
			ItemId:               999950,
			Name:                 "draining collar",
			Type:                 items.Neck,
			ReserveHealthPct:     0.20,
			ReserveStaminaPct:    0.10,
			ReserveConvictionPct: 0.30,
		},
		// Reserves nothing at all. The ordinary case, and the one the
		// disclosure has to stay silent through.
		999951: {
			ItemId: 999951,
			Name:   "plain collar",
			Type:   items.Neck,
		},
		// A second, lighter reserving item on a different slot, so a test can
		// take ONE thing off and still be holding something.
		999953: {
			ItemId:               999953,
			Name:                 "clinging ring",
			Type:                 items.Ring,
			ReserveHealthPct:     0.05,
			ReserveStaminaPct:    0.05,
			ReserveConvictionPct: 0.05,
		},
	})
}

func newReservationNoticeChar() *Character {
	c := New()
	c.HealthMax.Base = 400
	c.StaminaMax.Base = 200
	c.ConvictionMax.Base = 500
	c.Validate()
	return c
}

func TestReservationIncreaseNotice_SpeaksWhenGearTakesAShare(t *testing.T) {
	defer seedReservingGear(t)()

	c := newReservationNoticeChar()
	before := c.ReservationTotals()

	c.Equipment.Neck = items.New(999950)
	c.Validate()

	notice := c.ReservationIncreaseNotice(before)
	if notice == `` {
		t.Fatal("equipping an item that reserves health, stamina and conviction said nothing")
	}

	for _, pool := range []string{"health", "stamina", "conviction"} {
		if !strings.Contains(notice, pool) {
			t.Errorf("the notice does not name %s, which the item reserves: %q", pool, notice)
		}
	}

	// Player copy: descriptive bands only. A digit here leaks a balance value.
	if strings.ContainsAny(notice, "0123456789") {
		t.Errorf("the notice contains a number: %q", notice)
	}
	if strings.ContainsAny(notice, "–—") {
		t.Errorf("the notice contains an en or em dash: %q", notice)
	}

	// The share words come from the shared prose vocabulary, so the equip line
	// and the refusal cannot describe the same reservation differently.
	if !strings.Contains(notice, ReserveShareBand(c.GetPoolReservation("conviction", c.poolMax(PoolConviction)), c.poolMax(PoolConviction))) {
		t.Errorf("the notice does not use ReserveShareBand's wording: %q", notice)
	}
}

// The half that keeps it off every other equip.
func TestReservationIncreaseNotice_SilentWhenNothingIsSetAside(t *testing.T) {
	defer seedReservingGear(t)()

	c := newReservationNoticeChar()
	before := c.ReservationTotals()

	c.Equipment.Neck = items.New(999951)
	c.Validate()

	if notice := c.ReservationIncreaseNotice(before); notice != `` {
		t.Errorf("a plain item that reserves nothing produced a disclosure: %q", notice)
	}
}

// Nothing changed at all is the commonest call of the three: most equips do not
// touch the neck slot, the pools, or anything else the disclosure watches.
func TestReservationIncreaseNotice_SilentOnAnUnchangedCharacter(t *testing.T) {
	defer seedReservingGear(t)()

	c := newReservationNoticeChar()
	c.Equipment.Neck = items.New(999950)
	c.Validate()

	// Already wearing the reserving item. Snapshot AFTER it is on, then change
	// nothing.
	before := c.ReservationTotals()
	if notice := c.ReservationIncreaseNotice(before); notice != `` {
		t.Errorf("an equip that changed no reservation produced a disclosure: %q", notice)
	}
}

// The trap that makes a points comparison the wrong test.
//
// reserve_*_pct is a percentage of the pool max, so ANY item that raises a pool
// raises the reserved POINTS on gear already worn without the wearer setting a
// larger share of themselves aside. Comparing points would announce a
// reservation increase for a plain +Vitality helmet.
func TestReservationIncreaseNotice_SilentWhenOnlyThePoolGrew(t *testing.T) {
	c := newReservationNoticeChar()

	// Same share of every pool, twice the pool. Points doubled; nothing was
	// actually set aside.
	before := ReservationTotals{
		Health: 40, HealthMax: 400,
		Stamina: 20, StaminaMax: 200,
		Conviction: 150, ConvictionMax: 500,
	}
	c.HealthMax.Base = 800
	c.StaminaMax.Base = 400
	c.ConvictionMax.Base = 1000
	c.Validate()
	c.Companions = []CompanionInfo{{ConvictionReserve: 300}}

	if got := c.GetPoolReservation("conviction", c.poolMax(PoolConviction)); got != 300 {
		t.Fatalf("fixture: conviction reservation = %d, want 300", got)
	}
	if notice := c.ReservationIncreaseNotice(before); notice != `` {
		t.Errorf("a bigger pool at an unchanged share produced a disclosure: %q", notice)
	}

	// And one more point of share genuinely is an increase.
	c.Companions = []CompanionInfo{{ConvictionReserve: 301}}
	if notice := c.ReservationIncreaseNotice(before); notice == `` {
		t.Error("a genuinely larger conviction share said nothing")
	}
}
