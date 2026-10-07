package main

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestShippedConfigSurfacesEveryLightingKnob is lighting plan 6's config
// surfacing guard (spec section 7). Every lighting knob is present in the
// shipped _datafiles/config.yaml, so none of them silently runs its Go
// default, and the two the plan retunes ship at their new values:
// LightExitsAbove 55 (owner ruling O5) and LightRealMinimum 3 (O3).
//
// "Every lighting knob" is read off configs.Balance itself: each field whose
// yaml key starts with Light (the edges, the celestial knobs, and the
// Light*Spell* scaling for glow, night vision, heat sight and darkness),
// plus the three lighting knobs named otherwise. A knob added later is
// guarded without editing this list.
func TestShippedConfigSurfacesEveryLightingKnob(t *testing.T) {
	raw, err := os.ReadFile("_datafiles/config.yaml")
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	keys := []string{"WorldLatitude", "DarknessCombatPenalty", "DazzleCap"}
	bt := reflect.TypeOf(configs.Balance{})
	for i := 0; i < bt.NumField(); i++ {
		name, _, _ := strings.Cut(bt.Field(i).Tag.Get("yaml"), ",")
		if strings.HasPrefix(name, "Light") {
			keys = append(keys, name)
		}
	}
	spells := 0
	for _, key := range keys {
		if strings.Contains(key, "Spell") {
			spells++
		}
	}
	// 33 Light* keys at plan 6, 18 of them spell scaling: a walk that found
	// far fewer is not reading the struct.
	if len(keys) < 30 || spells < 18 {
		t.Fatalf("found %d lighting keys (%d spell scaling): the Balance walk is not seeing the knobs", len(keys), spells)
	}
	for _, key := range keys {
		if !regexp.MustCompile(`(?m)^\s+` + key + `:\s*\S`).Match(raw) {
			t.Errorf("config.yaml does not set %s: it runs its Go default unseen", key)
		}
	}

	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	cfg := configs.GetLightingConfig()
	for _, c := range []struct {
		name      string
		got, want float64
	}{
		{"LightBlindBelow", float64(cfg.BlindBelow), 25},
		{"LightDimBelow", float64(cfg.DimBelow), 50},
		{"LightExitsAbove", float64(cfg.ExitsAbove), 55},
		{"LightRealMinimum", float64(cfg.RealMinimum), 3},
		{"LightDazzleAbove", float64(cfg.DazzleAbove), 75},
		{"LightDefaultVisionStrength", float64(cfg.DefaultVisionStrength), 12},
		{"LightDoublingStep", cfg.DoublingStep, 8},
		{"WorldLatitude", cfg.WorldLatitude, 46.5},
		{"LightEquinoxNoon", cfg.EquinoxNoon, 70},
		{"LightStarlight", cfg.Starlight, 10},
		{"LightMoonsFull", cfg.MoonsFull, 35},
		{"LightMoonWeightSwiftmoon", cfg.MoonWeightSwiftmoon, 4},
		{"LightMoonWeightWanderer", cfg.MoonWeightWanderer, 1},
		{"LightMoonWeightEye", cfg.MoonWeightEye, 0.5},
	} {
		if c.got != c.want {
			t.Errorf("shipped %s = %v, want %v", c.name, c.got, c.want)
		}
	}
}
