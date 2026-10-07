package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

func modelCfg() configs.Lighting {
	return configs.Lighting{
		BlindBelow: 25, DimBelow: 50, ExitsAbove: 65, RealMinimum: 3,
		DoublingStep: 8, WorldLatitude: 46.5, EquinoxNoon: 70,
		Starlight: 10, MoonsFull: 35,
		MoonWeightSwiftmoon: 4, MoonWeightWanderer: 1, MoonWeightEye: 0.5,
	}
}

// A room with no sky and no lamp is light 0: the darkest natural light, which
// the scale defines as an unlit cave. It is NOT negative; negative is magical
// darkness, which plan 5 introduces.
func TestUnlitCaveIsZeroNotNegative(t *testing.T) {
	zero := 0.0
	r := Room{SkyLight: &zero}
	if got := r.lightLevel(modelCfg(), 70, true); got != 0 {
		t.Errorf("unlit cave = %d, want 0", got)
	}
}

// The sky fraction is an attenuation, so 0.5 costs exactly one doubling step.
func TestSkyFractionCostsOneStepPerHalving(t *testing.T) {
	half := 0.5
	r := Room{SkyLight: &half}
	if got := r.lightLevel(modelCfg(), 60, true); got != 52 {
		t.Errorf("half sky under a 60 sky = %d, want 52", got)
	}
}

// A lamp joins the same combine as the sky rather than acting as a floor, so a
// lamp equal to the ambient reads one step above it, not double.
func TestLampJoinsTheCombineRatherThanFlooring(t *testing.T) {
	open := 1.0
	lamp := 36
	r := Room{SkyLight: &open, Lamp: &lamp}
	if got := r.lightLevel(modelCfg(), 36, true); got != 44 {
		t.Errorf("lamp 36 under a 36 sky = %d, want 44", got)
	}
}

// 🔑 The property that made the log combine necessary: a lantern is nearly
// irrelevant at noon. Under the spec's original halving rule this read 125.
func TestLanternIsNearlyIrrelevantAtNoon(t *testing.T) {
	open := 1.0
	lamp := 55
	r := Room{SkyLight: &open, Lamp: &lamp}
	got := r.lightLevel(modelCfg(), 70, true)
	if got < 70 || got > 74 {
		t.Errorf("lantern 55 at noon 70 = %d, want 70 to 74", got)
	}
}

func TestLightIsClampedToTheScale(t *testing.T) {
	open := 1.0
	lamp := 100
	r := Room{SkyLight: &open, Lamp: &lamp}
	if got := r.lightLevel(modelCfg(), 100, true); got > 100 {
		t.Errorf("light = %d, above the scale ceiling", got)
	}
}
