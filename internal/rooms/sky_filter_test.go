package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mutators"
)

// A filter multiplies the sky fraction, which on the log scale is a fixed
// subtraction: 0.7 about 4 points, 0.5 one step, 0.35 about 12. A lamp is
// untouched.
func TestSkyFilterSubtractsFromTheSkyOnly(t *testing.T) {
	cfg := modelCfg()
	open, zero := 1.0, 0.0
	lamp := 40

	sky := Room{SkyLight: &open}
	clear := sky.lightLevelWithSkyFilter(cfg, 60, true, 1)
	for _, c := range []struct {
		filter float64
		drop   int
	}{{0.7, 4}, {0.5, 8}, {0.35, 12}} {
		if got := clear - sky.lightLevelWithSkyFilter(cfg, 60, true, c.filter); got != c.drop {
			t.Errorf("filter %v dropped the sky by %d, want %d", c.filter, got, c.drop)
		}
	}

	lampOnly := Room{SkyLight: &zero, Lamp: &lamp}
	if a, b := lampOnly.lightLevelWithSkyFilter(cfg, 60, true, 1), lampOnly.lightLevelWithSkyFilter(cfg, 60, true, 0.35); a != b {
		t.Errorf("a filter moved a lamp: %d clear, %d filtered", a, b)
	}
}

// Active mutators multiply; one without a skylight changes nothing; an
// outdoor-only mutator never reaches an indoor biome.
func TestMutatorSkyFilterMultipliesAndStaysOutdoors(t *testing.T) {
	cleanup := seedRegistry()
	defer cleanup()
	defer SeedBiomesForTest(map[string]*BiomeInfo{
		"testfield": {BiomeId: "testfield", Name: "Field", Symbol: "."},
		"testhouse": {BiomeId: "testhouse", Name: "House", Symbol: "H", Indoor: true},
	})()
	half, most := 0.5, 0.7
	defer mutators.SeedSpecsForTest(
		mutators.MutatorSpec{MutatorId: "test-storm", OutdoorOnly: true, SkyLight: &half},
		mutators.MutatorSpec{MutatorId: "test-fog", OutdoorOnly: true, SkyLight: &most},
		mutators.MutatorSpec{MutatorId: "test-sanctuary"},
	)()

	outdoor, indoor := roomManager.rooms[1], roomManager.rooms[2]
	outdoor.Biome, indoor.Biome = "testfield", "testhouse"
	zc := GetZoneConfig("TestZone")

	if got := outdoor.mutatorSkyFilter(); got != 1 {
		t.Errorf("clear weather filter = %v, want 1", got)
	}
	zc.Mutators.Add("test-sanctuary")
	if got := outdoor.mutatorSkyFilter(); got != 1 {
		t.Errorf("a mutator with no skylight changed the filter to %v", got)
	}
	zc.Mutators.Add("test-storm")
	zc.Mutators.Add("test-fog")
	if got := outdoor.mutatorSkyFilter(); math.Abs(got-0.35) > 1e-9 {
		t.Errorf("storm and fog = %v, want 0.35", got)
	}
	if got := indoor.mutatorSkyFilter(); got != 1 {
		t.Errorf("weather reached an indoor biome: filter %v", got)
	}
}
