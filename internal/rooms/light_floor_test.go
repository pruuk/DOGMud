package rooms

import (
	"math"
	"testing"
)

// The floor (lighting plan 6, owner ruling O3): any real light reads at least
// LightRealMinimum before darkness is subtracted, and a room with none reads
// exactly 0.
func TestRealLightReadsAtLeastTheFloor(t *testing.T) {
	cfg := modelCfg() // RealMinimum 3
	zero := 0.0
	tenth := 0.1
	cave := Room{SkyLight: &zero}
	cell := Room{SkyLight: &tenth} // 5105's sky fraction

	cases := []struct {
		name      string
		room      Room
		celestial float64
		carried   []float64
		dark      []float64
		wantLevel int
		wantRaw   float64 // NaN: not checked
	}{
		{"a sealed cave reads 0", cave, 10, nil, nil, 0, 0},
		{"a cell under starlight alone is floored to 3", cell, 10, nil, nil, 3, 3},
		{"a carried sliver is floored to 3", cave, 10, []float64{0.4}, nil, 3, 3},
		{"light at the floor is untouched", cave, 10, []float64{3}, nil, 3, 3},
		{"light above the floor is untouched", cave, 10, []float64{4.6}, nil, 5, 4.6},
		{"darkness subtracts from the floored light", cell, 10, nil, []float64{10}, -7, -7},
		{"darkness alone in a cave reads below 0", cave, 10, nil, []float64{10}, -10, -10},
	}
	for _, c := range cases {
		got := c.room.composeWith(cfg, c.celestial, 1, c.carried, c.dark)
		if got.Level != c.wantLevel {
			t.Errorf("%s: Level = %d, want %d", c.name, got.Level, c.wantLevel)
		}
		if !math.IsNaN(c.wantRaw) && math.Abs(got.Raw-c.wantRaw) > 1e-9 {
			t.Errorf("%s: Raw = %v, want %v", c.name, got.Raw, c.wantRaw)
		}
	}
}

// Light carries the floored value: it is what darkness subtracts from, so a
// darkness trim solving against it leaves the room exactly on its floor.
func TestTheFloorIsTheLightDarknessSubtractsFrom(t *testing.T) {
	tenth := 0.1
	cell := Room{SkyLight: &tenth}
	got := cell.composeWith(modelCfg(), 10, 1, nil, []float64{2})
	if got.Light != 3 {
		t.Errorf("Light = %v, want the floored 3", got.Light)
	}
	if math.Abs(got.Raw-(got.Light-got.Dark)) > 1e-9 {
		t.Errorf("Raw = %v, want Light - Dark = %v", got.Raw, got.Light-got.Dark)
	}
}

// With the knob at 0 (only reachable with LightBlindBelow at 1 or below) there
// is no floor.
func TestAFloorOfZeroIsNoFloor(t *testing.T) {
	cfg := modelCfg()
	cfg.RealMinimum = 0
	zero := 0.0
	cave := Room{SkyLight: &zero}
	if got := cave.composeWith(cfg, 10, 1, []float64{0.4}, nil); math.Abs(got.Raw-0.4) > 1e-9 {
		t.Errorf("Raw = %v, want 0.4 with no floor", got.Raw)
	}
}

// Cell 5105 at night with no moons: the old arithmetic read -16.6, by
// construction a natural negative. Now the sky reads about 1.49, a tenth of
// starlight's brightness B(10) = 2^(10/8) - 1 read back in points, and is
// floored to 3.
func TestTheHoldingCellAtANewMoonMidnightIsBarelyLit(t *testing.T) {
	tenth := 0.1
	cell := Room{SkyLight: &tenth}
	got := cell.composeWith(modelCfg(), 10, 1, nil, nil)
	if math.Abs(got.Sky-1.49) > 0.01 {
		t.Errorf("Sky = %v, want about 1.49", got.Sky)
	}
	if got.Level != 3 {
		t.Errorf("Level = %d, want 3", got.Level)
	}
}
