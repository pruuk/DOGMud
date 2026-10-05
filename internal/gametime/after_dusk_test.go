package gametime

import (
	"math"
	"testing"
)

// duskRound scans one day for the first round at or after noon that reads
// Night: the round dusk falls in.
func duskRound(t *testing.T, dayOfYear int) uint64 {
	t.Helper()
	for r := roundFor(dayOfYear, 12); r < roundFor(dayOfYear+1, 0); r++ {
		if GetDate(r).Night {
			return r
		}
	}
	t.Fatalf("day %d has no dusk after noon", dayOfYear)
	return 0
}

// HoursAfterDusk reads the same unrounded boundary Night does, so it is
// near 0 in the round dusk falls in and grows one game hour per 37.5 rounds.
func TestHoursAfterDuskStartsAtTheNightBoundary(t *testing.T) {
	pinTiming(t, 46.5)
	for _, doy := range []int{356, 81, 172} {
		dusk := duskRound(t, doy)
		if h := GetDate(dusk).HoursAfterDusk(); h < 0 || h >= 1.0/37.5 {
			t.Errorf("day %d: at the dusk round HoursAfterDusk = %v, want within one round of 0", doy, h)
		}
		if h := GetDate(dusk + 75).HoursAfterDusk(); math.Abs(h-2) > 1.0/37.5 {
			t.Errorf("day %d: 75 rounds after dusk HoursAfterDusk = %v, want about 2", doy, h)
		}
		// The round before dusk is day, and reads nearly a whole day since
		// the boundary: never "just after dusk".
		if gd := GetDate(dusk - 1); gd.Night || gd.HoursAfterDusk() < 23 {
			t.Errorf("day %d: the round before dusk reads Night=%v HoursAfterDusk=%v, want day and over 23",
				doy, gd.Night, gd.HoursAfterDusk())
		}
	}
}

// Past midnight it keeps counting from the evening's dusk.
func TestHoursAfterDuskWrapsPastMidnight(t *testing.T) {
	pinTiming(t, 46.5)
	dusk := duskRound(t, 356)
	want := 24 - (float64(dusk%900) / 37.5) // hours from dusk to midnight
	if h := GetDate(roundFor(357, 0)).HoursAfterDusk(); math.Abs(h-want) > 1.0/37.5 {
		t.Errorf("midnight after midwinter dusk: HoursAfterDusk = %v, want about %v", h, want)
	}
}
