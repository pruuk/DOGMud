package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/lightscale"
)

// TestPublicLightTermsMatchesLightLevel drives the public entry points, which
// each read config, the clock and the mutators themselves, across shipped
// biomes and the clock.
func TestPublicLightTermsMatchesLightLevel(t *testing.T) {
	withShippedBiomesAndClock(t)
	for _, biome := range []string{"city_thoroughfare", "city_backstreet", "interior", "cave", "forest", "plains"} {
		requireBiome(t, biome)
		room := Room{Biome: biome}
		for _, hour := range []float64{0, 6, 12, 18} {
			setClock(172, hour)
			if got, want := room.LightTerms().Level, room.LightLevel(); got != want {
				t.Errorf("%s at %v: LightTerms().Level = %d, LightLevel() = %d", biome, hour, got, want)
			}
		}
	}
}

func TestLightTermsReportEachTerm(t *testing.T) {
	cfg := modelCfg()
	open, zero := 1.0, 0.0
	lamp := 40

	lit := Room{SkyLight: &open, Lamp: &lamp}
	got := lit.composeLight(cfg, 60, 0.25)
	if !got.HasLamp || got.Lamp != 40 {
		t.Errorf("lamp = (%v, %d), want (true, 40)", got.HasLamp, got.Lamp)
	}
	if got.SkyFilter != 0.25 {
		t.Errorf("SkyFilter = %v, want 0.25", got.SkyFilter)
	}
	wantSky := lightscale.Attenuate(cfg.DoublingStep, 60, 0.25)
	if math.Abs(got.Sky-wantSky) > 1e-9 {
		t.Errorf("sky after a 0.25 filter = %v, want %v", got.Sky, wantSky)
	}
	if got.Carried {
		t.Error("Carried = true with nobody in the room")
	}

	cave := Room{SkyLight: &zero}
	if c := cave.composeLight(cfg, 60, 1); c.Sky != 0 || c.HasLamp {
		t.Errorf("a cave has no sky light (0) and no lamp, got Sky=%v HasLamp=%v", c.Sky, c.HasLamp)
	}
}
