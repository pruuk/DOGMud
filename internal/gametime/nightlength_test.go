package gametime

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func pinTiming(t *testing.T, latitude float64) {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.NightHours = 8
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	c.Balance.WorldLatitude = configs.ConfigFloat(latitude)
	c.Balance.Validate()
	configs.SetConfigForTest(t, c)

	// 🚨 MANDATORY, not hygiene. roundDateCache is keyed on the round number
	// alone and carries no config fingerprint, so without this a test inherits
	// whatever an earlier test cached for the same round under a DIFFERENT
	// latitude, and asserts on arithmetic it never performed. Clear on the way
	// out as well, so this test cannot poison the next one.
	ClearDateCacheForTest()
	t.Cleanup(ClearDateCacheForTest)
}

// roundFor returns the round number for a given day of the year and hour, at
// the pinned RoundsPerDay of 900.
func roundFor(dayOfYear int, hour float64) uint64 {
	return uint64(float64(dayOfYear-1)*900 + hour*37.5)
}

// Midwinter night is 15h37m, so 05:00 is still night; midsummer night is
// 8h23m, so 05:00 is broad day. Under the old flat model both were day,
// because night ended at 04:00 all year.
func TestNightLengthVariesWithTheSeason(t *testing.T) {
	pinTiming(t, 46.5)
	original := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCount(original) })

	if gd := GetDate(roundFor(356, 5)); !gd.Night {
		t.Errorf("midwinter 05:00 reported day; at 46.5 degrees night runs to about 07:49")
	}
	if gd := GetDate(roundFor(172, 5)); gd.Night {
		t.Errorf("midsummer 05:00 reported night; at 46.5 degrees night ends about 04:11")
	}
}

// 🔴 REPLACES TestZeroLatitudeFallsBackToNightHours, deleted 2026-09-23.
// There is no NightHours fallback any more: zero is coerced to the default in
// validation, because a config without WorldLatitude reads zero, and honouring
// zero would have shipped DOGMud with a flat night and no seasons.
//
// A near-equatorial latitude is the replacement for that behaviour, and gives
// a flat twelve-hour night all year, which is what an operator asking for "no
// seasons" actually wants.
func TestNearEquatorialLatitudeGivesAFlatTwelveHourNight(t *testing.T) {
	pinTiming(t, 0.001)
	original := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCount(original) })

	for _, doy := range []int{1, 81, 172, 356} {
		// Night runs 18:00 to 06:00 at twelve hours, so 03:00 is night and
		// 09:00 is day, on every day of the year.
		if gd := GetDate(roundFor(doy, 3)); !gd.Night {
			t.Errorf("day %d 03:00 reported day under a flat twelve-hour night", doy)
		}
		if gd := GetDate(roundFor(doy, 9)); gd.Night {
			t.Errorf("day %d 09:00 reported night under a flat twelve-hour night", doy)
		}
	}
}

// 🔴 The load-bearing test of this task. A config without WorldLatitude (an
// older config.yaml, or this test binary, which loads Go defaults; the shipped
// file has set it since lighting plan 6) reads zero, so if validation ever
// stops coercing a zero latitude, such a world runs with no seasons and every
// other test here still passes because they all pin a latitude explicitly.
// This one deliberately does not.
func TestShippedConfigHasASeasonalNight(t *testing.T) {
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	c.Balance.Validate() // no latitude authored: a config that omits the key
	configs.SetConfigForTest(t, c)

	// 🔴 THIS LINE IS THE TEST. Without it this test is a fraud, and was
	// one: TestNightLengthVariesWithTheSeason runs first, pins latitude 46.5,
	// and asks about these exact two rounds, so roundDateCache already holds
	// the correct seasonal answer. This test would then get a cache hit and
	// pass on the other test's arithmetic, never once exercising the shipped
	// default it exists to guard. Proven, not theorised: with the zero-latitude
	// coercion deliberately broken, this test failed in isolation and PASSED in
	// the normal package run.
	ClearDateCacheForTest()
	t.Cleanup(ClearDateCacheForTest)

	original := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCount(original) })

	winter := GetDate(roundFor(356, 5)).Night
	summer := GetDate(roundFor(172, 5)).Night
	if winter == summer {
		t.Fatalf("05:00 reads the same in midwinter and midsummer (both night=%v); "+
			"the shipped config has no seasonal night, so WorldLatitude defaulted to zero", winter)
	}
}

// Midnight is night and midday is day at every latitude on every day. If this
// ever fails the hour-of-day arithmetic has drifted.
func TestMidnightIsAlwaysNightAndNoonAlwaysDay(t *testing.T) {
	pinTiming(t, 46.5)
	original := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCount(original) })

	for doy := 1; doy <= 365; doy += 7 {
		if gd := GetDate(roundFor(doy, 0)); !gd.Night {
			t.Fatalf("day %d midnight reported day", doy)
		}
		if gd := GetDate(roundFor(doy, 12)); gd.Night {
			t.Fatalf("day %d noon reported night", doy)
		}
	}
}
