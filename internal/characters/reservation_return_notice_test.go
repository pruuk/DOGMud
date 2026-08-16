package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// U7b told the player when capacity was TAKEN and never when it came back.
//
// A 2026-08-16 re-check playtest put on a reserving item, was told "your gear
// and bonds now hold a share of your health in reserve", took it straight off
// again and heard only "You remove your Blackrayor and return it to your
// backpack." Half a feature: the whole point of the disclosure is to teach what
// drives the ceiling, and a lesson with no second half does not teach it.

func assertReturnCopyClean(t *testing.T, notice string) {
	t.Helper()
	if strings.ContainsAny(notice, "0123456789") {
		t.Errorf("the notice contains a number: %q", notice)
	}
	if strings.Contains(notice, "%") {
		t.Errorf("the notice contains a percentage: %q", notice)
	}
	if strings.ContainsAny(notice, "–—") {
		t.Errorf("the notice contains an en or em dash: %q", notice)
	}
}

// A partial release: one reserving item comes off, another stays on. The line
// must name every pool that got something back and where each now stands.
func TestReservationDecreaseNotice_SpeaksWhenGearGivesAShareBack(t *testing.T) {
	defer seedReservingGear(t)()

	c := newReservationNoticeChar()
	c.Equipment.Neck = items.New(999950)
	c.Equipment.Ring = items.New(999953)
	c.Validate()

	before := c.ReservationTotals()
	if before.Conviction <= 0 || before.Health <= 0 {
		t.Fatal("fixture: the gear reserved nothing, so there is nothing to give back")
	}

	c.Equipment.Neck = items.Item{}
	c.Validate()

	notice := c.ReservationDecreaseNotice(before)
	if notice == `` {
		t.Fatal("taking off an item that reserved health, stamina and conviction said nothing")
	}

	for _, pool := range []string{"health", "stamina", "conviction"} {
		if !strings.Contains(notice, pool) {
			t.Errorf("the notice does not name %s, which the collar released: %q", pool, notice)
		}
	}

	// The band quoted is where each pool NOW stands, in the shared vocabulary.
	for _, pool := range []Pool{PoolHealth, PoolStamina, PoolConviction} {
		max := c.poolMax(pool)
		want := ReserveShareBand(c.GetPoolReservation(string(pool), max), max) +
			` of your ` + poolDisplayName(pool)
		if !strings.Contains(notice, want) {
			t.Errorf("the notice does not read %q: %s", want, notice)
		}
	}

	assertReturnCopyClean(t, notice)
}

// Everything comes off. Reciting the bottom rung once per pool buries the one
// thing the player wants to hear, so the line collapses.
func TestReservationDecreaseNotice_SaysSoWhenNothingIsHeldAtAll(t *testing.T) {
	defer seedReservingGear(t)()

	c := newReservationNoticeChar()
	c.Equipment.Neck = items.New(999950)
	c.Validate()

	before := c.ReservationTotals()
	c.Equipment.Neck = items.Item{}
	c.Validate()

	notice := c.ReservationDecreaseNotice(before)
	if !strings.Contains(notice, "Nothing you carry holds any part of you in reserve now") {
		t.Errorf("a character holding nothing at all must be told plainly: %q", notice)
	}
	assertReturnCopyClean(t, notice)
}

// The half that keeps it off every other remove.
func TestReservationDecreaseNotice_SilentWhenNothingWasSetAside(t *testing.T) {
	defer seedReservingGear(t)()

	c := newReservationNoticeChar()
	c.Equipment.Neck = items.New(999951)
	c.Validate()

	before := c.ReservationTotals()
	c.Equipment.Neck = items.Item{}
	c.Validate()

	if notice := c.ReservationDecreaseNotice(before); notice != `` {
		t.Errorf("taking off a plain item that reserved nothing produced a disclosure: %q", notice)
	}
}

// Nothing changed at all: most removes touch neither the reserving slot nor the
// pools.
func TestReservationDecreaseNotice_SilentOnAnUnchangedCharacter(t *testing.T) {
	defer seedReservingGear(t)()

	c := newReservationNoticeChar()
	c.Equipment.Neck = items.New(999950)
	c.Validate()

	before := c.ReservationTotals()
	if notice := c.ReservationDecreaseNotice(before); notice != `` {
		t.Errorf("a remove that changed no reservation produced a disclosure: %q", notice)
	}
}

// The mirror of the trap that makes a points comparison wrong on the equip side.
//
// reserve_*_pct is a percentage of the pool max, so taking off a plain
// +Vitality helmet SHRINKS the reserved points on gear still worn without
// giving the wearer any share of themselves back. Comparing points would
// announce a return that did not happen.
func TestReservationDecreaseNotice_SilentWhenOnlyThePoolShrank(t *testing.T) {
	c := newReservationNoticeChar()

	// Same share of every pool, twice the pool, before. Points halve; nothing
	// was actually given back.
	// Health and stamina hold nothing before or after, so conviction is the
	// only pool under test and a false positive there cannot hide behind them.
	before := ReservationTotals{
		Health: 0, HealthMax: 800,
		Stamina: 0, StaminaMax: 400,
		Conviction: 600, ConvictionMax: 1000,
	}
	c.HealthMax.Base = 400
	c.StaminaMax.Base = 200
	c.ConvictionMax.Base = 500
	c.Validate()
	c.Companions = []CompanionInfo{{ConvictionReserve: 300}}

	if got := c.GetPoolReservation("conviction", c.poolMax(PoolConviction)); got != 300 {
		t.Fatalf("fixture: conviction reservation = %d, want 300", got)
	}
	if notice := c.ReservationDecreaseNotice(before); notice != `` {
		t.Errorf("a smaller pool at an unchanged share produced a disclosure: %q", notice)
	}

	// And one point less of share genuinely is a return.
	c.Companions = []CompanionInfo{{ConvictionReserve: 299}}
	if notice := c.ReservationDecreaseNotice(before); notice == `` {
		t.Error("a genuinely smaller conviction share said nothing")
	}
}

// An INCREASE must never come out of the decrease notice, and vice versa. The
// two share movedPoolShares, so a mirrored guard is the thing that could be got
// backwards.
func TestReservationNotices_DoNotFireInEachOthersDirection(t *testing.T) {
	defer seedReservingGear(t)()

	c := newReservationNoticeChar()
	before := c.ReservationTotals()

	c.Equipment.Neck = items.New(999950)
	c.Validate()

	if notice := c.ReservationDecreaseNotice(before); notice != `` {
		t.Errorf("PUTTING ON a reserving item produced a return disclosure: %q", notice)
	}

	after := c.ReservationTotals()
	c.Equipment.Neck = items.Item{}
	c.Validate()

	if notice := c.ReservationIncreaseNotice(after); notice != `` {
		t.Errorf("TAKING OFF a reserving item produced an increase disclosure: %q", notice)
	}
}
