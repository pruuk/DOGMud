package rooms

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestZoneMultipliers_UnsetZeroAndSet(t *testing.T) {
	defer SeedRoomsForTest(map[int]*Room{}, map[string]*ZoneConfig{
		"Plain":  {Name: "Plain"},
		"Hushed": {Name: "Hushed", AmbientMultiplier: MultiplierPtr(0), WanderMultiplier: MultiplierPtr(0)},
		"Lively": {Name: "Lively", AmbientMultiplier: MultiplierPtr(2), WanderMultiplier: MultiplierPtr(0.5)},
	})()
	for _, c := range []struct {
		zone          string
		ambient, walk float64
	}{
		{"Plain", 1, 1},
		{"NoSuchZone", 1, 1},
		{"Hushed", 0, 0},
		{"Lively", 2, 0.5},
	} {
		if got := ZoneAmbientMultiplier(c.zone); got != c.ambient {
			t.Errorf("ZoneAmbientMultiplier(%q) = %v, want %v", c.zone, got, c.ambient)
		}
		if got := ZoneWanderMultiplier(c.zone); got != c.walk {
			t.Errorf("ZoneWanderMultiplier(%q) = %v, want %v", c.zone, got, c.walk)
		}
	}
}

func TestZoneMultipliers_YAMLTellsZeroFromUnset(t *testing.T) {
	var z ZoneConfig
	if err := yaml.Unmarshal([]byte("name: Z\nambientmultiplier: 0\n"), &z); err != nil {
		t.Fatal(err)
	}
	if z.AmbientMultiplier == nil || *z.AmbientMultiplier != 0 {
		t.Fatalf("ambientmultiplier: 0 must load as a set 0, got %v", z.AmbientMultiplier)
	}
	if z.WanderMultiplier != nil {
		t.Fatalf("an absent wandermultiplier must stay unset, got %v", *z.WanderMultiplier)
	}
}

func TestZoneMultipliers_NegativeRefusesBoot(t *testing.T) {
	z := &ZoneConfig{Name: "Bad", WanderMultiplier: MultiplierPtr(-1)}
	if err := z.Validate(); err == nil {
		t.Fatal("a negative wandermultiplier must fail Validate")
	}
	z = &ZoneConfig{Name: "Bad", AmbientMultiplier: MultiplierPtr(-0.1)}
	if err := z.Validate(); err == nil {
		t.Fatal("a negative ambientmultiplier must fail Validate")
	}
}
