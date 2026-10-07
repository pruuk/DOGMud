package gametime

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The lamplighter's rule (lighting plan 6, owner ruling O4 as amended, and
// the review fix after it): the street lamps burn at night, and by day while
// the clear sky, seen through the dimmest street-lamp street's sky fraction,
// would read a level below the faces edge.
func TestLampsLitAt(t *testing.T) {
	cfg := configs.Lighting{DimBelow: 50, DoublingStep: 8}
	for _, c := range []struct {
		name      string
		night     bool
		celestial float64
		streetSky float64
		dim       int
		want      bool
	}{
		{"night under a bright sky", true, 70, 1, 50, true},
		{"night with no sky at all", true, lightscale.Absent(), 1, 50, true},
		{"a day sky below the faces edge", false, 40, 1, 50, true},
		{"an open-sky street just under the edge rounds like a room", false, 49.4, 1, 50, true},
		{"an open-sky street that rounds to the edge", false, 49.5, 1, 50, false},
		{"an open-sky street exactly at the edge", false, 50, 1, 50, false},
		{"a day sky above the edge", false, 70, 1, 50, false},
		// 50 through 0.95 reads 49.41, a level of 49: the dip the first
		// rule left at every lamp change.
		{"a 0.95 street under a sky of 50 keeps its lamps", false, 50, 0.95, 50, true},
		// 51 through 0.95 reads 50.40, a level of 50.
		{"a 0.95 street under a sky of 51 reads faces", false, 51, 0.95, 50, false},
		{"a street with no sky keeps its lamps all day", false, 90, 0, 50, true},
		{"an Absent sky by day", false, lightscale.Absent(), 1, 50, true},
		{"a NaN sky leaves it to the night: day", false, math.NaN(), 1, 50, false},
		{"a NaN sky leaves it to the night: night", true, math.NaN(), 1, 50, true},
		{"the edge is the configured one", false, 55, 1, 60, true},
	} {
		cfg.DimBelow = c.dim
		if got := LampsLitAt(c.night, c.celestial, c.streetSky, cfg); got != c.want {
			t.Errorf("%s: LampsLitAt(%v, %v, %v, dim %d) = %v, want %v",
				c.name, c.night, c.celestial, c.streetSky, c.dim, got, c.want)
		}
	}
}

// The registered fraction defaults to the open sky, clamps out-of-range
// values, and hands back what it replaced.
func TestStreetLampSkyFractionRegistration(t *testing.T) {
	orig := SetStreetLampSkyFraction(0.95)
	t.Cleanup(func() { SetStreetLampSkyFraction(orig) })
	if got := StreetLampSkyFraction(); got != 0.95 {
		t.Fatalf("registered 0.95, read %v", got)
	}
	for _, c := range []struct{ in, want float64 }{
		{math.NaN(), 1}, {1.5, 1}, {-0.2, 0}, {0, 0}, {0.4, 0.4},
	} {
		SetStreetLampSkyFraction(c.in)
		if got := StreetLampSkyFraction(); got != c.want {
			t.Errorf("registered %v, read %v, want %v", c.in, got, c.want)
		}
	}
	if prev := SetStreetLampSkyFraction(0.7); prev != 0.4 {
		t.Errorf("Set returned %v, want the replaced 0.4", prev)
	}
}

// LampsLit reads the live clock: midwinter 08:00 is day (IsNight false) but
// its clear sky is below the faces edge, so the lamps still burn; noon puts
// them out; midnight lights them.
func TestLampsLitOnTheClock(t *testing.T) {
	pinTiming(t, 46.5)
	ClearCelestialMemoForTest()
	t.Cleanup(ClearCelestialMemoForTest)
	original := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCount(original) })
	dim := configs.GetLightingConfig().DimBelow

	at := func(doy int, hour float64) (night bool, celestial float64, lit bool) {
		util.SetRoundCount(roundFor(doy, hour))
		ClearDateCacheForTest()
		ClearCelestialMemoForTest()
		return IsNight(), CelestialLight(), LampsLit()
	}

	if night, cel, lit := at(356, 8); night || !(cel < float64(dim)) || !lit {
		t.Errorf("midwinter 08:00: night=%v celestial=%.2f lit=%v, want day, a sky below %d, lamps lit",
			night, cel, lit, dim)
	}
	for _, doy := range []int{356, 81, 172} {
		if _, _, lit := at(doy, 12); lit {
			t.Errorf("day %d noon: lamps lit, want out", doy)
		}
		if _, _, lit := at(doy, 0); !lit {
			t.Errorf("day %d midnight: lamps out, want lit", doy)
		}
	}

	// Every round of midwinter day: LampsLit is exactly the pure rule on
	// the clock's own reads, and there is dim daylight where it differs from
	// IsNight alone (else this test proves nothing about the new half).
	dimDay := 0
	for r := roundFor(356, 0); r < roundFor(357, 0); r++ {
		util.SetRoundCount(r)
		night, cel := IsNight(), CelestialLight()
		if got, want := LampsLit(), LampsLitAt(night, cel, StreetLampSkyFraction(), configs.GetLightingConfig()); got != want {
			t.Fatalf("round %d: LampsLit %v, LampsLitAt on the same reads %v", r, got, want)
		}
		if !night && LampsLit() {
			dimDay++
		}
	}
	if dimDay == 0 {
		t.Errorf("midwinter has no dim daylight round with the lamps lit; the sky-dim half is untested")
	}
}
