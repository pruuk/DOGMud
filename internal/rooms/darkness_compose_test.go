package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/lightscale"
)

// The Rule 2 table of the lighting plan 5d spec: lights combine, darknesses
// combine among themselves by the same halving rule, and the net is the
// light (0 when none) minus the darkness (0 when none), clamped at -100.
func TestDarknessComposition(t *testing.T) {
	cfg := modelCfg()
	zero, open := 0.0, 1.0
	cave := Room{SkyLight: &zero}
	tavern := Room{SkyLight: &zero, Lamp: LampPtr(50)}
	field := Room{SkyLight: &open}

	cases := []struct {
		name        string
		room        Room
		celestial   float64
		light, dark []float64
		want        int
	}{
		{"508 today, nobody carrying", cave, 60, nil, nil, 0},
		{"508 with the Phantom's lantern", cave, 60, nil, []float64{50}, -50},
		{"508, lantern, a torch", cave, 60, []float64{56}, []float64{50}, 6},
		{"508, lantern, endgame glow", cave, 60, []float64{90}, []float64{50}, 40},
		{"508, lantern, glow 90 and torch 56", cave, 60, []float64{90, 56}, []float64{50}, 41},
		{"tavern lamp 50, one darkness 50", tavern, 60, nil, []float64{50}, 0},
		{"open ground at noon, new caster's pall", field, 70, nil, []float64{50}, 20},
		{"open ground at noon, endgame pall", field, 70, nil, []float64{90}, -20},
		{"cave, two darkness 50", cave, 60, nil, []float64{50, 50}, -58},
		{"cave, darkness 50 and 40", cave, 60, nil, []float64{50, 40}, -54},
		{"cave, three endgame palls: clamped", cave, 60, nil, []float64{90, 90, 90}, -100},
	}
	for _, c := range cases {
		got := c.room.composeWith(cfg, c.celestial, 1, c.light, c.dark)
		if got.Level != c.want {
			t.Errorf("%s: Level = %d, want %d", c.name, got.Level, c.want)
		}
	}
}

// Light, Dark and Raw carry the two combines and the net; Carried and
// Darkened are independent.
func TestDarknessTermsAreReported(t *testing.T) {
	cfg := modelCfg()
	zero := 0.0
	cave := Room{SkyLight: &zero}

	dark := cave.composeWith(cfg, 60, 1, nil, []float64{50, 50})
	if !math.IsInf(dark.Light, -1) || dark.Carried {
		t.Errorf("darkness alone: Light %v Carried %v, want -Inf and false", dark.Light, dark.Carried)
	}
	if want := lightscale.Combine(cfg.DoublingStep, 50, 50); math.Abs(dark.Dark-want) > 1e-9 || !dark.Darkened {
		t.Errorf("darkness alone: Dark %v Darkened %v, want %v and true", dark.Dark, dark.Darkened, want)
	}
	if math.Abs(dark.Raw-(-dark.Dark)) > 1e-9 {
		t.Errorf("darkness alone: Raw %v, want %v (Absent light reads 0)", dark.Raw, -dark.Dark)
	}

	lit := cave.composeWith(cfg, 60, 1, []float64{56}, nil)
	if !lit.Carried || lit.Darkened || !math.IsInf(lit.Dark, -1) || lit.Light != 56 || lit.Raw != 56 {
		t.Errorf("a torch alone: %+v, want Carried, not Darkened, Dark -Inf, Light and Raw 56", lit)
	}

	both := cave.composeWith(cfg, 60, 1, []float64{56}, []float64{50})
	if !both.Carried || !both.Darkened || both.Light != 56 || both.Dark != 50 || both.Raw != 6 {
		t.Errorf("a torch and a darkness: %+v, want both flags, Light 56, Dark 50, Raw 6", both)
	}
}

// Darkness takes from sky, lamp and carried light alike: it subtracts from
// the combined light, not from any one term.
func TestDarknessSubtractsFromTheWholeCombine(t *testing.T) {
	cfg := modelCfg()
	open := 1.0
	room := Room{SkyLight: &open, Lamp: LampPtr(50)}
	light := lightscale.Combine(cfg.DoublingStep, 60, 50, 56)
	got := room.composeWith(cfg, 60, 1, []float64{56}, []float64{40})
	if want := int(math.Round(light - 40)); got.Level != want {
		t.Errorf("sky 60, lamp 50, torch 56, darkness 40: Level %d, want %d", got.Level, want)
	}
}
