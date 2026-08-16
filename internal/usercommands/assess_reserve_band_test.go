package usercommands

import "testing"

// assess discloses how much Conviction a raised companion will reserve —
// as a descriptive band, never a raw number (owner request 2026-08-02:
// companions' CP reservation was judged fair but undisclosed).

func TestConvictionReserveBand(t *testing.T) {
	// The ladder is measured against the reservable CEILING, not the pool, so
	// every edge below is a fraction of 660 (the cap on a pool of 1000 at the
	// shipped and defaulted 0.66) rather than of 1000.
	const maxPool = 1000
	cases := []struct {
		reserve int
		want    string
	}{
		{50, "a slight part"},   // under 15% of the ceiling
		{98, "a slight part"},   // just under 15%
		{99, "a modest share"},  // 15%
		{230, "a modest share"}, // just under 35%
		{231, "a notable share"},
		{362, "a notable share"},
		{363, "a heavy share"}, // 55%
		{494, "a heavy share"},
		{495, "nearly all you can set aside"}, // 75%
		{659, "nearly all you can set aside"},
		{660, "all you can set aside"}, // the ceiling itself
		{999, "all you can set aside"},
		{1000, "more than your spirit could hold"}, // >= the whole pool
		{1500, "more than your spirit could hold"},
	}
	for _, tc := range cases {
		if got := convictionReserveBand(tc.reserve, maxPool); got != tc.want {
			t.Errorf("convictionReserveBand(%d, %d) = %q, want %q", tc.reserve, maxPool, got, tc.want)
		}
	}
	// Degenerate pool never divides by zero.
	if got := convictionReserveBand(10, 0); got != "more than your spirit could hold" {
		t.Errorf("zero max pool: got %q", got)
	}
}

func TestJoinRaiseForms(t *testing.T) {
	cases := []struct {
		forms []string
		want  string
	}{
		{[]string{"skeleton"}, "a skeleton"},
		{[]string{"skeleton", "zombie"}, "a skeleton or zombie"},
		{[]string{"skeleton", "zombie", "wraith"}, "a skeleton, zombie, or wraith"},
	}
	for _, tc := range cases {
		if got := joinRaiseForms(tc.forms); got != tc.want {
			t.Errorf("joinRaiseForms(%v) = %q, want %q", tc.forms, got, tc.want)
		}
	}
}
