package rooms

import "testing"

// TestRoomOverridesBiomeSkyAndLamp pins Task 7's override precedence: a
// room's own SkyLight/Lamp win over its biome's when set, and the biome's
// apply when the room has no override at all.
//
// GetBiome() reads the package-level `biomes` map (see lighting_test.go's
// doc comment on TestLightLevelMapsTheOldModel for the full explanation), so
// this seeds a "cave" biome with SkyLight forced to 0 via SeedBiomesForTest
// rather than relying on the real _datafiles/biomes tree.
func TestRoomOverridesBiomeSkyAndLamp(t *testing.T) {
	cleanupBiomes := SeedBiomesForTest(map[string]*BiomeInfo{
		"default": {BiomeId: "default", Name: "Default"},
		"cave":    {BiomeId: "cave", Name: "Cave", SkyLight: SkyLightPtr(0.0)},
	})
	defer cleanupBiomes()

	open := 1.0
	lamp := 55

	r := Room{Biome: "cave"}
	if got := r.skyLightFraction(); got != 0 {
		t.Errorf("cave room without override = %v, want 0", got)
	}

	r.SkyLight = &open
	if got := r.skyLightFraction(); got != 1.0 {
		t.Errorf("overridden skylight = %v, want 1.0", got)
	}

	if got, ok := r.lampValue(true); ok {
		t.Errorf("cave room lamp = (%v,%v), want none", got, ok)
	}
	r.Lamp = &lamp
	got, ok := r.lampValue(true)
	if !ok || got != 55 {
		t.Errorf("overridden lamp = (%v,%v), want (55,true)", got, ok)
	}
}

// TestRoomZeroOverridesAreHonoured pins that an authored zero override
// survives and is distinguishable from "no override, use the biome". A
// buried vault inside an otherwise sunlit fort is exactly this case: the
// biome says open sky, the room says none.
func TestRoomZeroOverridesAreHonoured(t *testing.T) {
	cleanupBiomes := SeedBiomesForTest(map[string]*BiomeInfo{
		"default": {BiomeId: "default", Name: "Default"},
		"land":    {BiomeId: "land", Name: "Land", SkyLight: SkyLightPtr(1.0)},
	})
	defer cleanupBiomes()

	zero := 0.0
	r := Room{Biome: "land", SkyLight: &zero}
	if got := r.skyLightFraction(); got != 0 {
		t.Errorf("zero override = %v, want 0", got)
	}
}
