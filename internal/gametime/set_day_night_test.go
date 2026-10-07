package gametime

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/util"
)

// isBoundary reports whether round r is the first round of day (sunrise) or
// of night (sunset), by the same Night predicate every reader uses.
func isBoundary(r uint64, wantNight bool) bool {
	return GetDate(r).Night == wantNight && GetDate(r-1).Night != wantNight
}

// assertNextBoundary checks that SetToDay / SetToNight(-1) from start lands
// one round before the FIRST matching boundary strictly after start (#408).
func assertNextBoundary(t *testing.T, label string, start uint64, wantNight bool, set func(...int)) {
	t.Helper()
	util.SetRoundCount(start)
	set(-1)
	got := util.GetRoundCount()
	boundary := got + 1
	if boundary <= start {
		t.Fatalf("%s: clock moved back or stood still: start %d, landed %d", label, start, got)
	}
	if !isBoundary(boundary, wantNight) {
		t.Fatalf("%s: round %d after the landing is not the start of the period (Night=%v, before=%v)",
			label, boundary, GetDate(boundary).Night, GetDate(boundary-1).Night)
	}
	for r := start + 1; r < boundary; r++ {
		if isBoundary(r, wantNight) {
			t.Fatalf("%s: skipped an earlier boundary at round %d (landed before %d)", label, r, boundary)
		}
	}
}

func pinClock(t *testing.T) {
	t.Helper()
	pinTiming(t, 46.5)
	original := util.GetRoundCount()
	originalOffset := dayResetOffset
	t.Cleanup(func() {
		util.SetRoundCount(original)
		dayResetOffset = originalOffset
		ClearDateCacheForTest()
	})
	dayResetOffset = 0
}

func TestSetToDayJumpsToTheNextSunrise(t *testing.T) {
	pinClock(t)
	for _, doy := range []int{10, 172, 356} {
		for _, hour := range []float64{2, 10, 15, 22} {
			start := roundFor(doy, hour)
			assertNextBoundary(t, "set day", start, false, SetToDay)
			landed := util.GetRoundCount() + 1
			wantDay := doy
			if !GetDate(start).Night || hour >= 12 {
				wantDay = doy + 1 // today's sunrise has passed
			}
			if gd := GetDate(landed); gd.Day != wantDay {
				t.Errorf("set day from day %d %.0f:00 landed on day %d, want %d", doy, hour, gd.Day, wantDay)
			}
		}
	}
}

func TestSetToNightJumpsToTheNextSunset(t *testing.T) {
	pinClock(t)
	for _, doy := range []int{10, 172, 356} {
		for _, hour := range []float64{2, 10, 15, 22} {
			start := roundFor(doy, hour)
			assertNextBoundary(t, "set night", start, true, SetToNight)
			landed := util.GetRoundCount() + 1
			wantDay := doy
			if GetDate(start).Night && hour >= 12 {
				wantDay = doy + 1 // after dusk: tomorrow's sunset is next
			}
			if gd := GetDate(landed); gd.Day != wantDay {
				t.Errorf("set night from day %d %.0f:00 landed on day %d, want %d", doy, hour, gd.Day, wantDay)
			}
		}
	}
}

// Repeated calls keep advancing the date instead of landing on the same
// sunset (the second half of #408).
func TestSetToNightRepeatedAdvancesTheDate(t *testing.T) {
	pinClock(t)
	util.SetRoundCount(roundFor(100, 10))
	SetToNight(-1)
	first := GetDate(util.GetRoundCount() + 1).Day
	// Step into the night, then ask again.
	util.SetRoundCount(util.GetRoundCount() + 2)
	SetToNight(-1)
	second := GetDate(util.GetRoundCount() + 1).Day
	if second != first+1 {
		t.Errorf("second set night landed on day %d, want %d", second, first+1)
	}
}

// After `settime` moves the clock offset, the jump still follows the clock
// players read, not the raw round number.
func TestSetToDayHonoursTheClockOffset(t *testing.T) {
	pinClock(t)
	util.SetRoundCount(roundFor(50, 1))
	SetTime(22) // the clock now reads 10PM
	ClearDateCacheForTest()
	start := util.GetRoundCount()
	if h := GetDate(start).Hour24; h != 22 {
		t.Fatalf("setup: clock reads hour %d, want 22", h)
	}
	assertNextBoundary(t, "set day after settime", start, false, SetToDay)
	if h := GetDate(util.GetRoundCount() + 1).Hour24; h < 4 || h > 8 {
		t.Errorf("sunrise landed at hour %d", h)
	}
}
