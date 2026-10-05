package configs

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

// A zero Balance is what a test binary sees; every 5d knob must default to
// glow's value (owner decision 3: shipped at glow's values).
func TestLighting5dKnobDefaults(t *testing.T) {
	var b Balance
	b.validateLighting()
	checks := []struct {
		name      string
		got, want float64
	}{
		{"LightDarknessSpellStrengthBase", float64(b.LightDarknessSpellStrengthBase), 40},
		{"LightDarknessSpellStrengthStatDivisor", float64(b.LightDarknessSpellStrengthStatDivisor), 10},
		{"LightDarknessSpellStrengthSkillDivisor", float64(b.LightDarknessSpellStrengthSkillDivisor), 2},
		{"LightDarknessSpellDurationBase", float64(b.LightDarknessSpellDurationBase), 2},
		{"LightDarknessSpellDurationStatDivisor", float64(b.LightDarknessSpellDurationStatDivisor), 50},
		{"LightDarknessSpellDurationSkillDivisor", float64(b.LightDarknessSpellDurationSkillDivisor), 20},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// A zero or negative divisor would divide by zero; it reverts.
func TestLighting5dKnobsRevertWhenNotPositive(t *testing.T) {
	b := Balance{LightDarknessSpellStrengthStatDivisor: -1, LightDarknessSpellDurationSkillDivisor: 0}
	b.validateLighting()
	if b.LightDarknessSpellStrengthStatDivisor != 10 || b.LightDarknessSpellDurationSkillDivisor != 20 {
		t.Errorf("divisors = %v / %v, want 10 / 20", b.LightDarknessSpellStrengthStatDivisor, b.LightDarknessSpellDurationSkillDivisor)
	}
}

func TestLightingAccessorCarries5dKnobs(t *testing.T) {
	cfg := GetConfig()
	cfg.Balance.LightDarknessSpellStrengthBase = 33
	cfg.Balance.LightDarknessSpellDurationStatDivisor = 44
	SetConfigForTest(t, cfg)
	l := GetLightingConfig()
	if l.DarknessSpellStrengthBase != 33 || l.DarknessSpellDurationStatDivisor != 44 ||
		l.DarknessSpellStrengthStatDivisor != 10 || l.DarknessSpellStrengthSkillDivisor != 2 ||
		l.DarknessSpellDurationBase != 2 || l.DarknessSpellDurationSkillDivisor != 20 {
		t.Errorf("accessor = %+v", l)
	}
}

// The six keys ship in config.yaml at glow's values, so the live value never
// silently rides the Go default (dogmud-balance-config).
func TestLighting5dKnobsShipInConfigYaml(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	// A map, not the Config struct: an ABSENT key must fail here, and a
	// struct field cannot tell absent from a shipped 0.
	var doc struct {
		Balance map[string]any `yaml:"Balance"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse config.yaml: %v", err)
	}
	want := map[string]float64{
		"LightDarknessSpellStrengthBase":         40,
		"LightDarknessSpellStrengthStatDivisor":  10,
		"LightDarknessSpellStrengthSkillDivisor": 2,
		"LightDarknessSpellDurationBase":         2,
		"LightDarknessSpellDurationStatDivisor":  50,
		"LightDarknessSpellDurationSkillDivisor": 20,
	}
	for k, v := range want {
		raw, ok := doc.Balance[k]
		if !ok {
			t.Errorf("config.yaml Balance has no %s", k)
			continue
		}
		var got float64
		switch n := raw.(type) {
		case int:
			got = float64(n)
		case float64:
			got = n
		default:
			t.Errorf("config.yaml Balance.%s = %v (%T), want a number", k, raw, raw)
			continue
		}
		if got != v {
			t.Errorf("config.yaml Balance.%s = %v, want %v", k, got, v)
		}
	}
}
