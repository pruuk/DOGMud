package characters

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// "Your gear and bonds already hold..." was a FIXED phrase.
//
// A 2026-08-16 playtest held exactly one companion, wore nothing at all, and
// was told its gear was holding a share of its conviction. Naming a source the
// player does not have sends them hunting for it, and the remedy list did the
// same thing from the other end: it told a gearless player to remove a draining
// item.
//
// Same class as the two falsehoods this slice already fixed: the refusal that
// claimed a holding at zero reservation, and the remedy that named the opposite
// of the action it meant.

func seedSourceGear(t *testing.T) func() {
	t.Helper()
	return items.SeedItemsForTest(map[int]*items.ItemSpec{
		999970: {
			ItemId:               999970,
			Name:                 "greedy torc",
			Type:                 items.Neck,
			ReserveConvictionPct: 0.60,
		},
	})
}

// A companion and no gear at all. The message must not mention gear, and must
// not tell the player to take anything off.
func TestReservationRefusal_CompanionOnly_NeverNamesGear(t *testing.T) {
	c := New()
	c.ConvictionMax.Base = 100
	c.Validate()
	c.Companions = []CompanionInfo{{ConvictionReserve: 60}}

	if got := c.GetPoolReservation("conviction", c.ConvictionMax.Value); got != 60 {
		t.Fatalf("fixture: reservation = %d, want 60 (all of it from the companion)", got)
	}

	msg := c.ReservationRefusal(PoolConviction, 10)

	if !strings.Contains(msg, "Your bonds already hold ") {
		t.Errorf("a player with a companion and no gear must be told about bonds alone:\n%s", msg)
	}
	if strings.Contains(msg, "gear") {
		t.Errorf("the refusal names gear the player does not have:\n%s", msg)
	}
	if !strings.Contains(msg, "dismiss a companion") {
		t.Errorf("the only remedy that could help is missing:\n%s", msg)
	}
	for _, useless := range []string{"remove a draining item", "disenchant"} {
		if strings.Contains(msg, useless) {
			t.Errorf("the remedy offers %q to a player wearing nothing:\n%s", useless, msg)
		}
	}
	assertPlayerCopyClean(t, msg)
}

// The mirror: gear and no companion must not mention bonds.
func TestReservationRefusal_GearOnly_NeverNamesBonds(t *testing.T) {
	defer seedSourceGear(t)()

	c := New()
	c.ConvictionMax.Base = 100
	c.Equipment.Neck = items.New(999970)
	c.Validate()

	msg := c.ReservationRefusal(PoolConviction, 10)

	if !strings.Contains(msg, "Your gear already holds ") {
		t.Errorf("a player with gear and no companion must be told about gear alone:\n%s", msg)
	}
	if strings.Contains(msg, "bond") {
		t.Errorf("the refusal names bonds the player does not have:\n%s", msg)
	}
	assertPlayerCopyClean(t, msg)
}

// Both sources really present is the only case that may say both.
func TestReservationRefusal_GearAndBonds_NamesBoth(t *testing.T) {
	defer seedSourceGear(t)()

	c := New()
	c.ConvictionMax.Base = 100
	c.Equipment.Neck = items.New(999970)
	c.Validate()
	c.Companions = []CompanionInfo{{ConvictionReserve: 5}}

	msg := c.ReservationRefusal(PoolConviction, 5)

	if !strings.Contains(msg, "Your gear and bonds already hold ") {
		t.Errorf("both sources are loaded, so both must be named:\n%s", msg)
	}
	for _, remedy := range []string{"remove a draining item", "dismiss a companion"} {
		if !strings.Contains(msg, remedy) {
			t.Errorf("the remedy must offer %q when it would help:\n%s", remedy, msg)
		}
	}
	assertPlayerCopyClean(t, msg)
}

// Stamina and health have no companion source at all, so a refusal on those
// pools can never speak of bonds however many companions are fielded.
func TestReservationRefusal_NonConvictionPool_NeverNamesBonds(t *testing.T) {
	defer items.SeedItemsForTest(map[int]*items.ItemSpec{
		999971: {
			ItemId:            999971,
			Name:              "famished cuirass",
			Type:              items.Body,
			ReserveStaminaPct: 0.60,
		},
	})()

	c := New()
	c.StaminaMax.Base = 100
	c.Equipment.Body = items.New(999971)
	c.Validate()
	c.Companions = []CompanionInfo{{ConvictionReserve: 300}}

	msg := c.ReservationRefusal(PoolStamina, 10)

	if strings.Contains(msg, "bond") || strings.Contains(msg, "companion") {
		t.Errorf("companions reserve conviction only, so a stamina refusal must not name them:\n%s", msg)
	}
	assertPlayerCopyClean(t, msg)
}

// The equip and remove lines carry the same subject, so the fixed phrase cannot
// come back through the disclosure instead of the refusal.
func TestReservationNotices_NameOnlyTheSourcesInPlay(t *testing.T) {
	defer seedReservingGear(t)()

	c := newReservationNoticeChar()
	before := c.ReservationTotals()

	c.Equipment.Neck = items.New(999950)
	c.Validate()

	notice := c.ReservationIncreaseNotice(before)
	if !strings.Contains(notice, "Your gear now holds ") {
		t.Errorf("the equip line must name gear alone for a player with no companion:\n%s", notice)
	}
	if strings.Contains(notice, "bond") {
		t.Errorf("the equip line names bonds the player does not have:\n%s", notice)
	}

	after := c.ReservationTotals()
	c.Equipment.Neck = items.Item{}
	c.Validate()

	back := c.ReservationDecreaseNotice(after)
	if strings.Contains(back, "bond") {
		t.Errorf("the remove line names bonds the player does not have:\n%s", back)
	}
}

// The subject and its verb must agree, or the fix ships "Your gear now hold".
func TestReserveSources_SubjectAndVerbAgree(t *testing.T) {
	cases := []struct {
		name    string
		sources reserveSources
		subject string
		verb    string
	}{
		{"gear only", reserveSources{drainingItem: true}, `Your gear`, `holds`},
		{"enchantment only", reserveSources{enchantment: true}, `Your gear`, `holds`},
		{"companion only", reserveSources{companion: true}, `Your bonds`, `hold`},
		{"both", reserveSources{drainingItem: true, companion: true}, `Your gear and bonds`, `hold`},
		{"neither", reserveSources{}, `What you carry`, `holds`},
	}
	for _, tc := range cases {
		if got := tc.sources.subject(); got != tc.subject {
			t.Errorf("%s: subject = %q, want %q", tc.name, got, tc.subject)
		}
		if got := tc.sources.verb(); got != tc.verb {
			t.Errorf("%s: verb = %q, want %q", tc.name, got, tc.verb)
		}
	}
}

// With nothing recorded, the fallback must not assert either source. This case
// is unreachable from today's callers; the guard is what keeps it that way.
func TestReserveSources_EmptyFallbackAssertsNothing(t *testing.T) {
	var none reserveSources
	if subject := none.subject(); strings.Contains(subject, "gear") || strings.Contains(subject, "bond") {
		t.Errorf("the empty-source subject asserts a source it has no evidence for: %q", subject)
	}
	// The remedy falls back to the full list, which is the honest answer when
	// nothing is known: every avenue, none of them claimed to apply.
	for _, remedy := range []string{"remove a draining item", "disenchant", "dismiss a companion"} {
		if !strings.Contains(none.remedies(), remedy) {
			t.Errorf("the empty-source remedy list dropped %q: %q", remedy, none.remedies())
		}
	}
}
