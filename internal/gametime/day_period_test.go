package gametime

import "testing"

// #382 playtest: `time` said "It is daytime" at 4:12PM after the dusk notice
// had already dimmed the street. While the lamps are lit and it is not yet
// night, the period is dusk (afternoon) or dawn (morning).
func TestDayPeriod(t *testing.T) {
	cases := []struct {
		name     string
		night    bool
		lampsLit bool
		hour24   int
		want     string
	}{
		{"midday", false, false, 12, "day"},
		{"lamps lit in the afternoon", false, true, 16, "dusk"},
		{"lamps lit in the morning", false, true, 7, "dawn"},
		{"night", true, true, 22, "night"},
		{"night before dawn", true, true, 4, "night"},
		{"noon boundary counts as afternoon", false, true, 12, "dusk"},
	}
	for _, c := range cases {
		if got := DayPeriod(c.night, c.lampsLit, c.hour24); got != c.want {
			t.Errorf("%s: DayPeriod(%v, %v, %d) = %q, want %q", c.name, c.night, c.lampsLit, c.hour24, got, c.want)
		}
	}
}
