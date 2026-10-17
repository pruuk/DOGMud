package gmcp

import "testing"

// TestWorldTimeLabelAgreesWithIsNight is the 2026-10-17 playtest payload
// `"time_of_day": "Afternoon", "is_night": true` at a midwinter 4:12PM: the
// label came from fixed clock hours while night is seasonal. The label must
// read "Night" exactly when is_night does, at every hour and lamp state.
func TestWorldTimeLabelAgreesWithIsNight(t *testing.T) {
	if got := timeOfDayLabel(true, true, 16); got != "Night" {
		t.Errorf("midwinter 4PM night reads %q, want Night", got)
	}
	for hour := 0; hour < 24; hour++ {
		for _, night := range []bool{false, true} {
			for _, lamps := range []bool{false, true} {
				if night && !lamps {
					continue // the lamps always burn at night (gametime.LampsLitAt)
				}
				got := timeOfDayLabel(night, lamps, hour)
				if (got == "Night") != night {
					t.Errorf("hour %d night=%v lamps=%v reads %q; it must say Night exactly when is_night is true",
						hour, night, lamps, got)
				}
			}
		}
	}
	if got := timeOfDayLabel(false, true, 16); got != "Evening" {
		t.Errorf("a lamplit winter afternoon before night reads %q, want Evening", got)
	}
	if got := timeOfDayLabel(false, true, 7); got != "Dawn" {
		t.Errorf("a lamplit morning before full light reads %q, want Dawn", got)
	}
	if got := timeOfDayLabel(false, false, 15); got != "Afternoon" {
		t.Errorf("a clear afternoon reads %q, want Afternoon", got)
	}
}
