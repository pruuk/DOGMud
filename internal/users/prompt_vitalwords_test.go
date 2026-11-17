package users

import (
	"strings"
	"testing"
)

// TestVitalWord checks each pool's four buckets at the 75, 50 and 25 percent
// edges, with and without a reservation, and that odd input does not panic.
func TestVitalWord(t *testing.T) {
	cases := []struct {
		pool                  string
		current, max, reserve int
		want                  string
	}{}
	edges := []struct {
		pct  int
		slot int
	}{{100, 0}, {75, 0}, {74, 1}, {50, 1}, {49, 2}, {25, 2}, {24, 3}, {0, 3}}
	for pool, words := range vitalWords {
		for _, e := range edges {
			cases = append(cases, struct {
				pool                  string
				current, max, reserve int
				want                  string
			}{pool, e.pct, 100, 0, words[e.slot]})
			// The same percentage of a reachable pool of 80 (20 reserved).
			cases = append(cases, struct {
				pool                  string
				current, max, reserve int
				want                  string
			}{pool, e.pct * 80 / 100, 100, 20, words[e.slot]})
		}
	}
	for _, c := range cases {
		if got := vitalWord(c.pool, c.current, c.max, c.reserve); got != c.want {
			t.Errorf("vitalWord(%s, %d, %d, %d) = %q, want %q", c.pool, c.current, c.max, c.reserve, got, c.want)
		}
	}

	if got := vitalWord(`hp`, 60, 100, 20); got != `unhurt` {
		t.Errorf("60 of 100 with 20 reserved = %q, want unhurt", got)
	}
	// Must not panic.
	_ = vitalWord(`hp`, 5, 0, 0)
	_ = vitalWord(`sp`, -5, 100, 0)
	_ = vitalWord(`cp`, 5, 100, -3)
	_ = vitalWord(`hp`, 5, 10, 50)
}

// TestPromptVitalWordsForScreenReader: a screen reader account hears the
// status as words and no bar glyphs; any other account still gets the bars.
func TestPromptVitalWordsForScreenReader(t *testing.T) {
	defer SetPromptSightCheck(nil)
	fx := newTankFixture(t)

	fx.player.ScreenReader = true
	got := promptVisible(fx.player.ProcessPromptString(`{hpbar} {stbar} {cvbar}`))
	if got != `unhurt fresh strong` {
		t.Errorf("screen reader status = %q, want %q", got, `unhurt fresh strong`)
	}
	for _, glyph := range []string{`█`, `░`, `▒`} {
		if strings.Contains(got, glyph) {
			t.Errorf("screen reader status %q contains %q", got, glyph)
		}
	}

	// The tank at 40 of 100 is badly hurt; the label stays.
	tankOut := promptVisible(fx.player.ProcessPromptString(`{tankbar}`))
	if tankOut != ` Thp:badly hurt` {
		t.Errorf("screen reader {tankbar} = %q, want %q", tankOut, ` Thp:badly hurt`)
	}

	fx.player.ScreenReader = false
	if got := promptVisible(fx.player.ProcessPromptString(`{hpbar}`)); !strings.Contains(got, `█`) {
		t.Errorf("sighted {hpbar} = %q, want a bar", got)
	}
}
