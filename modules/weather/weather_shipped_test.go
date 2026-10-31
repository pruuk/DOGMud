package weather

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v2"
)

// The shipped Modules.weather block reads as the #317 retune: a pass every
// 48 rounds and a seasonal line on 1 pass in 4.
func TestShippedWeatherAmbientKnobs(t *testing.T) {
	_, self, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(self), "..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read shipped config: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	modules, _ := doc["Modules"].(map[any]any)
	block, _ := modules["weather"].(map[any]any)
	if block == nil {
		t.Fatal("no Modules.weather block in config.yaml")
	}
	c := buildConfig(func(k string) any { return block[k] })
	if c.EmoteEveryRounds != 48 || c.SeasonalEmoteOneIn != 4 {
		t.Fatalf("shipped weather cadence %d seasonal 1 in %d, want 48 and 4", c.EmoteEveryRounds, c.SeasonalEmoteOneIn)
	}
}
