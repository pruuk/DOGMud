package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// loadBiomesForTest loads the REAL shipped biome YAMLs from
// _datafiles/world/dogmud/biomes into the package-level biomes map.
//
// This deliberately does NOT reuse the zone_lifecycle_test.go /
// zone_rename_test.go chdir-to-repo-root-then-configs.ReloadConfig() idiom.
// A first attempt at this helper did exactly that, and go test
// ./internal/rooms/... started failing two UNRELATED tests
// (TestLoadRoomInstance_AbsentOverlayIsSilent,
// TestCreateEphemeralZone_EmptyRoomIdsFallsBackToEntryRoom, or a different
// pair depending on run) with an "index out of range" panic inside
// CreateEphemeralRoomIds. Disabling only this test made the flake vanish.
// ReloadConfig() reads the REAL _datafiles/config.yaml, which swaps in
// production Balance/Timing/every-other-knob for the remainder of the test
// binary process wherever a restore misses a package-level cache; chasing
// the exact poisoned global was not worth it when a narrower fix removes the
// whole risk. Those two zone tests get away with the same idiom only because
// both are gated behind DOGMUD_BOOT_SMOKE and are skipped by default, so
// they never actually exercise it in a normal run.
//
// Instead, this only overlays FilePaths.DataFiles to a path relative to this
// package's own directory (a test binary's CWD is its package directory, the
// same fact internal/templates/process_test.go's dataFilesRoot documents),
// with everything else left at Go zero-value defaults, and skips
// ReloadConfig() entirely so no other knob is ever touched.
func loadBiomesForTest(t *testing.T) {
	t.Helper()

	origBiomes := biomes
	t.Cleanup(func() { biomes = origBiomes })

	cfg := configs.Config{}
	cfg.FilePaths.DataFiles = `../../_datafiles/world/dogmud`
	configs.SetConfigForTest(t, cfg)

	LoadBiomeDataFiles()
}

func TestBiomeSkyLightDefaultsToFullyOpen(t *testing.T) {
	b := BiomeInfo{BiomeId: "x", Name: "X", Symbol: "."}
	if got := b.SkyLightFraction(); got != 1.0 {
		t.Errorf("unset skylight = %v, want 1.0", got)
	}
}

// 🔑 Zero must be HONOURED, not treated as unset. A cave's sky fraction is
// genuinely zero, and coercing it to the default would make every cave as
// bright as an open field.
func TestBiomeSkyLightZeroIsHonoured(t *testing.T) {
	zero := 0.0
	b := BiomeInfo{BiomeId: "cave", Name: "Cave", Symbol: ".", SkyLight: &zero}
	if got := b.SkyLightFraction(); got != 0 {
		t.Errorf("authored zero skylight = %v, want 0", got)
	}
}

func TestBiomeLampDefaultsToNone(t *testing.T) {
	b := BiomeInfo{BiomeId: "x", Name: "X", Symbol: "."}
	if got, ok := b.LampValue(); ok {
		t.Errorf("unset lamp reported %v, want none", got)
	}
}

func TestBiomeLampZeroIsHonoured(t *testing.T) {
	zero := 0
	b := BiomeInfo{BiomeId: "x", Name: "X", Symbol: ".", Lamp: &zero}
	got, ok := b.LampValue()
	if !ok || got != 0 {
		t.Errorf("authored zero lamp = (%v,%v), want (0,true)", got, ok)
	}
}

func TestBiomeValidateRejectsSkyLightOutOfRange(t *testing.T) {
	for _, v := range []float64{-0.1, 1.1} {
		f := v
		b := BiomeInfo{BiomeId: "x", Name: "X", Symbol: ".", SkyLight: &f}
		if err := b.Validate(); err == nil {
			t.Errorf("skylight %v passed validation", v)
		}
	}
}

// A lamp is light, and light is never negative (lighting plan 6, owner
// ruling O1: negative is magical darkness only). A negative lamp would
// silently read as unlit, so it is refused at load.
func TestBiomeValidateRejectsLampOffScale(t *testing.T) {
	for _, v := range []int{-101, -1, 101} {
		n := v
		b := BiomeInfo{BiomeId: "x", Name: "X", Symbol: ".", Lamp: &n}
		if err := b.Validate(); err == nil {
			t.Errorf("lamp %v passed validation", v)
		}
	}
}

// Every shipped biome must declare a sky fraction. An unset one silently reads
// as fully open sky, which is right for a meadow and catastrophically wrong for
// a cave, so the shipped set is required to be explicit.
func TestEveryShippedBiomeDeclaresASkyFraction(t *testing.T) {
	loadBiomesForTest(t)
	for _, b := range GetAllBiomes() {
		if b.BiomeId == "default" {
			continue // synthetic Go fallback, not authored; plan 3b removes its users
		}
		if b.SkyLight == nil {
			t.Errorf("biome %q declares no skylight", b.BiomeId)
		}
	}
}
