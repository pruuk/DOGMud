package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
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

// A blinding room must read blinding. Before the overflow fix a light whose
// brightness overflowed (a huge Glow, or a tiny authored doubling step) came
// back +Inf, int(math.Round(+Inf)) is the most negative int, and the clamp
// turned the brightest room in the world into maximal magical darkness.
func TestAnOverflowingLightReadsBlindingNotDark(t *testing.T) {
	zero := 0.0
	cave := Room{SkyLight: &zero}
	tiny := modelCfg()
	tiny.DoublingStep = 1e-3
	cases := []struct {
		name    string
		cfg     configs.Lighting
		carried []float64
		dark    []float64
		want    int
	}{
		{"a huge carried light", modelCfg(), []float64{9000}, nil, 100},
		{"two huge carried lights", modelCfg(), []float64{9000, 9000}, nil, 100},
		{"a huge light against a huge darkness", modelCfg(), []float64{9000}, []float64{9000}, 0},
		{"a huge darkness", modelCfg(), []float64{30}, []float64{9000}, -100},
		{"a lantern at a tiny step", tiny, []float64{52}, nil, 52},
		{"two lanterns at a tiny step", tiny, []float64{52, 52}, nil, 52},
	}
	for _, c := range cases {
		got := cave.composeWith(c.cfg, 0, 1, c.carried, c.dark)
		if got.Level != c.want {
			t.Errorf("%s: Level = %d, want %d (Raw %v)", c.name, got.Level, c.want, got.Raw)
		}
	}
}

// The int conversion itself is safe for every float, so no future term that
// slips past lightscale can wrap around to -100.
func TestLevelOfRawIsSafeForEveryFloat(t *testing.T) {
	cases := []struct {
		raw  float64
		want int
	}{
		{math.Inf(1), 100},
		{math.Inf(-1), -100},
		{math.NaN(), 0},
		{1e300, 100},
		{-1e300, -100},
		{float64(math.MaxInt64), 100},
		{100.4, 100},
		{-100.4, -100},
		{49.5, 50},
		{-0.4, 0},
	}
	for _, c := range cases {
		if got := levelOfRaw(c.raw); got != c.want {
			t.Errorf("levelOfRaw(%v) = %d, want %d", c.raw, got, c.want)
		}
	}
}
