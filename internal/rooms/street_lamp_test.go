package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The street lamp (lighting plan 6, owner ruling O4 as amended): a biome
// marked streetlamp lights only while the street lamps are lit
// (gametime.LampsLit); every other lamp, and every room's own lamp override,
// burns at all hours.
func TestStreetLampJoinsOnlyWhileLampsAreLit(t *testing.T) {
	cleanup := SeedBiomesForTest(map[string]*BiomeInfo{
		"default":           {BiomeId: "default", Name: "Default"},
		"city_thoroughfare": {BiomeId: "city_thoroughfare", Name: "Street", SkyLight: SkyLightPtr(0.95), Lamp: LampPtr(52), StreetLamp: true},
		"interior":          {BiomeId: "interior", Name: "Interior", SkyLight: SkyLightPtr(0.15), Lamp: LampPtr(50)},
	})
	t.Cleanup(cleanup)
	cfg := modelCfg()

	street := Room{Biome: "city_thoroughfare"}
	inn := Room{Biome: "interior"}
	lampPost := Room{Biome: "city_thoroughfare", Lamp: LampPtr(40)}

	for _, c := range []struct {
		name     string
		room     Room
		lit      bool
		wantLamp bool
		lamp     int
	}{
		{"a street with the lamps out reads its daylight alone", street, false, false, 0},
		{"a street with the lamps lit has its lamp", street, true, true, 52},
		{"an inn's lamp burns with the lamps out", inn, false, true, 50},
		{"an inn's lamp burns with the lamps lit", inn, true, true, 50},
		{"a room's own lamp burns on a street with the lamps out", lampPost, false, true, 40},
		{"a room's own lamp burns on a street with the lamps lit", lampPost, true, true, 40},
	} {
		got := c.room.composeWithFixtures(cfg, 70, c.lit, 1, nil, nil, nil, nil)
		if got.HasLamp != c.wantLamp || got.Lamp != c.lamp {
			t.Errorf("%s: lamp (%v, %d), want (%v, %d)", c.name, got.HasLamp, got.Lamp, c.wantLamp, c.lamp)
		}
	}

	// With the lamps out the street is its sky alone; lit, the lamp joins it.
	day := street.composeWithFixtures(cfg, 70, false, 1, nil, nil, nil, nil)
	if day.Light != day.Sky {
		t.Errorf("street with the lamps out: Light %v, want the sky alone %v", day.Light, day.Sky)
	}
	night := street.composeWithFixtures(cfg, 10, true, 1, nil, nil, nil, nil)
	if night.Level != 52 {
		t.Errorf("street at a new-moon midnight = %d, want 52", night.Level)
	}
}

// setRound moves the world to an exact round, clearing the per-round memos.
func setRound(r uint64) {
	util.SetRoundCountForTest(r)
	gametime.ClearDateCacheForTest()
	gametime.ClearCelestialMemoForTest()
}

// The street lamp on the real clock. Every round of three sample days: the
// lamp is lit exactly while it is night or the clear sky reads below
// LightDimBelow, so it is lit all night. At each end of the day the lamp
// changes in the round the clear sky crosses the faces edge, not the round
// night ends or begins: lit one round before the morning crossing and out at
// it, out one round before the evening crossing and lit at it, and both
// crossings fall in daylight (IsNight false), which is the lamplighter's
// half of the rule that IsNight alone would get wrong.
func TestStreetLampFollowsTheDimSkyAtTheBoundary(t *testing.T) {
	withShippedBiomesAndClock(t)
	requireBiome(t, "city_thoroughfare")
	dim := float64(configs.GetLightingConfig().DimBelow)
	street := Room{Biome: "city_thoroughfare"}

	const roundsPerDay = 900
	for _, doy := range []int{356, 81, 172} {
		base := uint64(doy-1) * roundsPerDay
		type reading struct {
			night, dimSky, lamp bool
		}
		day := make([]reading, roundsPerDay)
		for i := range day {
			setRound(base + uint64(i))
			day[i] = reading{
				night:  gametime.IsNight(),
				dimSky: gametime.CelestialLight() < dim,
				lamp:   street.LightTerms().HasLamp,
			}
			if want := day[i].night || day[i].dimSky; day[i].lamp != want {
				t.Errorf("day %d round %d: lamp lit %v, want %v (night %v, clear sky below %v: %v)",
					doy, i, day[i].lamp, want, day[i].night, dim, day[i].dimSky)
			}
			if day[i].night && !day[i].lamp {
				t.Errorf("day %d round %d: night with the street lamp out", doy, i)
			}
		}

		// The morning crossing: the first round of the day the clear sky
		// reaches the faces edge. The evening one: the first round after
		// noon it is below the edge again.
		dawn, dusk := -1, -1
		for i := 1; i < roundsPerDay/2; i++ {
			if day[i-1].dimSky && !day[i].dimSky {
				dawn = i
				break
			}
		}
		for i := roundsPerDay / 2; i < roundsPerDay; i++ {
			if !day[i-1].dimSky && day[i].dimSky {
				dusk = i
				break
			}
		}
		if dawn < 0 || dusk < 0 {
			t.Fatalf("day %d: no crossing of the faces edge found (dawn %d, dusk %d)", doy, dawn, dusk)
		}
		if !day[dawn-1].lamp || day[dawn].lamp {
			t.Errorf("day %d morning: lamp %v one round before the crossing and %v at it, want lit then out",
				doy, day[dawn-1].lamp, day[dawn].lamp)
		}
		if day[dusk-1].lamp || !day[dusk].lamp {
			t.Errorf("day %d evening: lamp %v one round before the crossing and %v at it, want out then lit",
				doy, day[dusk-1].lamp, day[dusk].lamp)
		}
		if day[dawn-1].night || day[dusk].night {
			t.Errorf("day %d: a crossing fell in the night (morning %v, evening %v); the sky-dim half is untested",
				doy, day[dawn-1].night, day[dusk].night)
		}
	}

	// The case that forced the ruling: midwinter 08:00 is day, its clear sky
	// too dim to read a face, and its street lamps burn.
	setClock(356, 8)
	if gametime.IsNight() {
		t.Fatalf("midwinter 08:00 reads as night; the probe is off")
	}
	if !street.LightTerms().HasLamp {
		t.Errorf("midwinter 08:00: street lamp out, want lit (clear sky %.2f, faces edge %v)",
			gametime.CelestialLight(), dim)
	}
}

// The shipped biome files: the two street biomes carry street lamps, and the
// indoor and magical lamps do not (spec section 2).
func TestShippedStreetLamps(t *testing.T) {
	withShippedBiomesAndClock(t)
	for id, want := range map[string]bool{
		"city_thoroughfare": true, "city_backstreet": true,
		"interior": false, "ether": false, "spiderweb": false,
	} {
		b, ok := GetBiome(id)
		if !ok {
			t.Fatalf("biome %q is not shipped", id)
		}
		if b.StreetLamp != want {
			t.Errorf("%s: streetlamp = %v, want %v", id, b.StreetLamp, want)
		}
		if _, has := b.LampValue(); !has {
			t.Errorf("%s: declares no lamp at all", id)
		}
	}
}
