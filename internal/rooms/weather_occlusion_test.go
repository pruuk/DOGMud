package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/mutators"
)

// openSkyUnder loads the shipped biomes, clock and mutators and returns an
// open-sky plains room whose zone carries the given weather (none for clear).
func openSkyUnder(t *testing.T, weather ...string) *Room {
	t.Helper()
	withShippedBiomesAndClock(t)
	t.Cleanup(mutators.SeedSpecsForTest())
	mutators.LoadDataFiles()
	t.Cleanup(seedRegistry())
	requireBiome(t, "plains")

	room := roomManager.rooms[1]
	room.Biome, room.SkyLight, room.Lamp = "plains", nil, nil
	setWeather(t, room, weather...)
	return room
}

// setWeather replaces the room's zone weather.
func setWeather(t *testing.T, room *Room, weather ...string) {
	t.Helper()
	zc := GetZoneConfig(room.Zone)
	if zc == nil {
		t.Fatalf("room %d has no zone config", room.RoomId)
	}
	zc.Mutators = mutators.MutatorList{}
	for _, w := range weather {
		if !zc.Mutators.Add(w) {
			t.Fatalf("weather %q is not a shipped mutator", w)
		}
	}
}

var sampleDays = []int{172, 81, 356}

// brightestMidnight is the day, within 30 of doy, whose midnight sky is the
// brightest: the fullest moons.
func brightestMidnight(doy int) int {
	best, bestV := doy, -1e9
	for d := doy - 30; d <= doy+30; d++ {
		setClock(d, 0)
		if v := gametime.CelestialLight(); v > bestV {
			best, bestV = d, v
		}
	}
	return best
}

// Light cloud must leave the brightest moonlit night readable as shapes.
func TestFogLeavesAMoonlitNightReadable(t *testing.T) {
	room := openSkyUnder(t)
	blind := configs.GetLightingConfig().BlindBelow
	for _, doy := range sampleDays {
		d := brightestMidnight(doy)
		setClock(d, 0)
		setWeather(t, room)
		clear := room.LightLevel()
		setWeather(t, room, "weather-fog")
		foggy := room.LightLevel()
		t.Logf("day %d midnight: clear %d, fog %d", d, clear, foggy)
		if foggy < blind {
			t.Errorf("day %d midnight under fog = %d, below BlindBelow %d (clear %d)", d, foggy, blind, clear)
		}
		if drop := clear - foggy; drop < 3 || drop > 5 {
			t.Errorf("day %d midnight: fog took %d points, want about 4", d, drop)
		}
	}
}

// Heavy weather never hides faces at noon, at any season.
func TestStormNeverHidesFacesAtNoon(t *testing.T) {
	room := openSkyUnder(t, "weather-storm")
	dim := configs.GetLightingConfig().DimBelow
	for _, doy := range sampleDays {
		setClock(doy, 12)
		got := room.LightLevel()
		t.Logf("day %d noon under a storm: %d", doy, got)
		if got < dim {
			t.Errorf("day %d noon under a storm = %d, below DimBelow %d", doy, got, dim)
		}
	}
}

// Heavy weather halves the night sky, so a night readable in clear weather
// goes blind under a storm unless the moons are near their brightest.
//
// Since lighting plan 6 a halving is a whole doubling step only well above the
// dim end: on the linear sum it takes less as the light nears 0, because there
// is less light left to take (a clear 20 halves to about 14, a clear 10 to
// about 6). So a night storm takes between 3 and 8 points, never more.
func TestStormTakesAStepOffTheNight(t *testing.T) {
	room := openSkyUnder(t)
	blind := configs.GetLightingConfig().BlindBelow
	flipped := 0
	for _, doy := range sampleDays {
		for d := doy - 30; d <= doy+30; d++ {
			setClock(d, 0)
			setWeather(t, room)
			clear := room.LightLevel()
			setWeather(t, room, "weather-storm")
			stormy := room.LightLevel()
			if drop := clear - stormy; clear > 8 && (drop < 3 || drop > 8) {
				t.Errorf("day %d midnight: storm took %d points from %d, want 3 to 8", d, drop, clear)
			}
			if clear >= blind && stormy < blind {
				flipped++
			}
		}
	}
	t.Logf("%d sampled nights went from readable to blind under a storm", flipped)
	if flipped == 0 {
		t.Error("no sampled night went from readable to blind under a storm")
	}
}
