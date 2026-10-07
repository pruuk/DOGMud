package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// withShippedBiomesAndClock loads the REAL biome YAMLs from
// _datafiles/world/dogmud/biomes (so a wrong data file fails these tests) and
// pins the clock config the light model needs. It follows loadBiomesForTest's
// rule of never calling ReloadConfig, for the reason documented there.
func withShippedBiomesAndClock(t *testing.T) {
	t.Helper()

	origBiomes := biomes
	t.Cleanup(func() { biomes = origBiomes; registerStreetLampSky() })

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = `../../_datafiles/world/dogmud`
	cfg.Timing.RoundsPerDay = 900
	cfg.Timing.RoundSeconds = 4
	cfg.Timing.Validate()
	cfg.Balance.Validate() // shipped latitude, deliberately not overridden
	configs.SetConfigForTest(t, cfg)

	LoadBiomeDataFiles()

	gametime.ClearDateCacheForTest()
	t.Cleanup(gametime.ClearDateCacheForTest)
	gametime.ClearCelestialMemoForTest()
	t.Cleanup(gametime.ClearCelestialMemoForTest)

	original := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCountForTest(original) })
}

// setClock moves the world to a day of year and an hour. 900 rounds per day,
// so one hour is 37.5 rounds.
func setClock(doy int, hour float64) {
	util.SetRoundCountForTest(uint64(float64(doy-1)*900 + hour*37.5))
	gametime.ClearDateCacheForTest()
	gametime.ClearCelestialMemoForTest()
}

func requireBiome(t *testing.T, id string) {
	t.Helper()
	if _, ok := GetBiome(id); !ok {
		t.Fatalf("biome %q is not shipped in _datafiles/world/dogmud/biomes", id)
	}
}

// TestThoroughfaresAreFaceReadableAllYear is the promise the patch note makes:
// a main street can be walked after dark with faces readable. Sampled across
// the year and the clock, because the failure mode is midwinter midnight.
func TestThoroughfaresAreFaceReadableAllYear(t *testing.T) {
	withShippedBiomesAndClock(t)
	requireBiome(t, "city_thoroughfare")

	dim := configs.GetLightingConfig().DimBelow
	room := Room{Biome: "city_thoroughfare"}
	for _, doy := range []int{356, 81, 172} {
		for _, hour := range []float64{0, 3, 6, 12, 18, 21} {
			setClock(doy, hour)
			if got := room.LightLevel(); got < dim {
				t.Errorf("day %d hour %v: city_thoroughfare reads %d, below DimBelow %d; "+
					"a main street must show faces at any hour", doy, hour, got, dim)
			}
		}
	}
}

// TestBackstreetsHideFacesAtMidnight keeps the two tiers from collapsing into
// one. If someone raises the backstreet lamp to match, this goes red.
func TestBackstreetsHideFacesAtMidnight(t *testing.T) {
	withShippedBiomesAndClock(t)
	requireBiome(t, "city_backstreet")

	cfg := configs.GetLightingConfig()
	room := Room{Biome: "city_backstreet"}
	for _, doy := range []int{356, 81, 172} {
		setClock(doy, 0)
		got := room.LightLevel()
		if got >= cfg.DimBelow {
			t.Errorf("day %d midnight: city_backstreet reads %d, at or above DimBelow %d; "+
				"a back lane must hide faces after dark", doy, got, cfg.DimBelow)
		}
		if got < cfg.BlindBelow {
			t.Errorf("day %d midnight: city_backstreet reads %d, below BlindBelow %d; "+
				"a lane is dim, not blind", doy, got, cfg.BlindBelow)
		}
	}
}

// TestRuinsAreOutdoorAndFollowTheSky pins what makes a ruin a ruin: weather
// reaches it, faces read at noon, and nothing lights it at night.
func TestRuinsAreOutdoorAndFollowTheSky(t *testing.T) {
	withShippedBiomesAndClock(t)
	requireBiome(t, "ruins")

	b, _ := GetBiome("ruins")
	if b.Indoor {
		t.Fatal("ruins is indoor; a roofless room must take the weather")
	}
	if _, ok := b.LampValue(); ok {
		t.Fatal("ruins declares a lamp; nothing lights a ruin")
	}

	dim := configs.GetLightingConfig().DimBelow
	room := Room{Biome: "ruins"}
	for _, doy := range []int{356, 81, 172} {
		setClock(doy, 12)
		if got := room.LightLevel(); got < dim {
			t.Errorf("day %d noon: ruins reads %d, below DimBelow %d; a roofless room is lit by day",
				doy, got, dim)
		}
		setClock(doy, 0)
		if got := room.LightLevel(); got >= dim {
			t.Errorf("day %d midnight: ruins reads %d, at or above DimBelow %d; a ruin is dark at night",
				doy, got, dim)
		}
	}
}
