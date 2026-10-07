package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

// Rule 10 and X5: a lit fixture is one term in the light combine, a darkness
// fixture one in the darkness combine; LightTerms.Fixture reports the light
// fixtures' combine and never sets Carried; CarriedLight is the carried
// light alone.
func TestFixtureComposition(t *testing.T) {
	cfg := modelCfg()
	zero := 0.0
	cave := Room{SkyLight: &zero}
	tavern := Room{SkyLight: &zero, Lamp: LampPtr(38)}

	cases := []struct {
		name             string
		room             Room
		carried, dark    []float64
		fxLight, fxDark  []float64
		want             int
		wantFixture      float64 // 0 when none (lighting plan 6: Combine of nothing reads 0)
		wantCarriedLight float64 // 0 when none (lighting plan 6: Combine of nothing reads 0)
	}{
		{"a fixture alone", cave, nil, nil, []float64{52}, nil, 52, 52, 0},
		{"5000: lamp 38, stones at their trough", tavern, nil, nil, []float64{20}, nil, 40, 20, 0},
		{"5000: lamp 38, stones at their crest", tavern, nil, nil, []float64{36}, nil, 45, 36, 0},
		{"a fixture and a carried torch", cave, []float64{56}, nil, []float64{52}, nil, 62, 52, 56},
		{"two fixtures", cave, nil, nil, []float64{52, 52}, nil, 60, 60, 0},
		{"a darkness fixture under a lamp", tavern, nil, nil, nil, []float64{30}, 8, 0, 0},
		{"a darkness fixture and a carried darkness", cave, nil, []float64{50}, nil, []float64{50}, -58, 0, 0},
	}
	for _, c := range cases {
		got := c.room.composeWithFixtures(cfg, 60, true, 1, c.carried, c.dark, c.fxLight, c.fxDark)
		if got.Level != c.want {
			t.Errorf("%s: Level = %d, want %d", c.name, got.Level, c.want)
		}
		if !sameTerm(got.Fixture, c.wantFixture) {
			t.Errorf("%s: Fixture = %v, want %v", c.name, got.Fixture, c.wantFixture)
		}
		if !sameTerm(got.CarriedLight, c.wantCarriedLight) {
			t.Errorf("%s: CarriedLight = %v, want %v", c.name, got.CarriedLight, c.wantCarriedLight)
		}
		if got.Carried != (len(c.carried) > 0) {
			t.Errorf("%s: Carried = %v, want %v: a fixture is never a carried light", c.name, got.Carried, len(c.carried) > 0)
		}
	}
}

// composeWith is composeWithFixtures with no fixtures: every existing
// caller and test reads the same terms as before.
func TestComposeWithIsFixtureFree(t *testing.T) {
	cfg := modelCfg()
	zero := 0.0
	cave := Room{SkyLight: &zero, Lamp: LampPtr(50)}
	a := cave.composeWith(cfg, 60, 1, []float64{56}, []float64{20})
	b := cave.composeWithFixtures(cfg, 60, true, 1, []float64{56}, []float64{20}, nil, nil)
	if a != b {
		t.Errorf("composeWith %+v != composeWithFixtures with none %+v", a, b)
	}
}

// LightLevel reads a room's fixtures from internal/itemlight by its id.
func TestLightLevelReadsTheRoomsFixtures(t *testing.T) {
	t.Cleanup(itemlight.ResetForTest())
	zero := 0.0
	cave := &Room{RoomId: 7831, SkyLight: &zero}
	before := cave.LightTerms()
	itemlight.Set(7831, uuid.UUID{1}, itemlight.Light, 52)
	after := cave.LightTerms()
	if before.Level != 0 || after.Level != 52 || after.Carried {
		t.Errorf("before %d, after %d (Carried %v); want 0, 52, false", before.Level, after.Level, after.Carried)
	}
}

func sameTerm(a, b float64) bool {
	if math.IsInf(a, -1) || math.IsInf(b, -1) {
		return math.IsInf(a, -1) && math.IsInf(b, -1)
	}
	return math.Abs(a-b) < 0.5
}
