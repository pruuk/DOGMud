package gametime

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The lamplighter's rule (lighting plan 6, owner ruling O4 as amended): the
// street lamps burn at night, and by day while the clear sky reads below the
// faces edge.
func TestLampsLitAt(t *testing.T) {
	for _, c := range []struct {
		name      string
		night     bool
		celestial float64
		dim       int
		want      bool
	}{
		{"night under a bright sky", true, 70, 50, true},
		{"night with no sky at all", true, lightscale.Absent(), 50, true},
		{"a day sky below the faces edge", false, 40, 50, true},
		{"a day sky just below the edge", false, 49.999, 50, true},
		{"a day sky exactly at the edge", false, 50, 50, false},
		{"a day sky above the edge", false, 70, 50, false},
		{"an Absent sky by day", false, lightscale.Absent(), 50, true},
		{"a NaN sky leaves it to the night: day", false, math.NaN(), 50, false},
		{"a NaN sky leaves it to the night: night", true, math.NaN(), 50, true},
		{"the edge is the configured one", false, 55, 60, true},
	} {
		if got := LampsLitAt(c.night, c.celestial, c.dim); got != c.want {
			t.Errorf("%s: LampsLitAt(%v, %v, %d) = %v, want %v", c.name, c.night, c.celestial, c.dim, got, c.want)
		}
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
		if got, want := LampsLit(), LampsLitAt(night, cel, dim); got != want {
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
