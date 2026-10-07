package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/lightscale"
)

// Every carried source is its own term: two equal torches are one doubling
// step brighter than one, not the flat single term plan 1 shipped.
func TestCarriedSourcesEachJoinTheCombine(t *testing.T) {
	cfg := modelCfg()
	zero := 0.0
	cave := Room{SkyLight: &zero}

	one := cave.composeWith(cfg, 60, 1, []float64{56}, nil)
	two := cave.composeWith(cfg, 60, 1, []float64{56, 56}, nil)
	if one.Level != 56 {
		t.Errorf("one torch in a cave = %d, want 56", one.Level)
	}
	if two.Level != 64 {
		t.Errorf("two torches in a cave = %d, want 64 (one step of 8 brighter)", two.Level)
	}
	if !one.Carried || cave.composeWith(cfg, 60, 1, nil, nil).Carried {
		t.Error("Carried must be true exactly when a carried term is present")
	}
	if want := lightscale.Combine(cfg.DoublingStep, 56, 56); math.Abs(two.Raw-want) > 1e-9 {
		t.Errorf("Raw = %v, want %v", two.Raw, want)
	}
	// Since lighting plan 5d, Raw is the net light; since plan 6 no light reads 0, and
	// the light combine alone is Light.
	if empty := cave.composeWith(cfg, 60, 1, nil, nil); empty.Light != 0 || empty.Raw != 0 || empty.Level != 0 {
		t.Errorf("an empty cave: Light %v Raw %v Level %d, want 0, 0 and 0", empty.Light, empty.Raw, empty.Level)
	}
}

// A carried term of exactly 0 is a present term, not an absent one: 0 is the
// darkest natural light and lightscale.Trim can produce it.
func TestCarriedTermOfZeroIsPresent(t *testing.T) {
	cfg := modelCfg()
	zero := 0.0
	cave := Room{SkyLight: &zero}

	got := cave.composeWith(cfg, 60, 1, []float64{0}, nil)
	if got.Raw != 0 || got.Level != 0 || !got.Carried {
		t.Errorf("a zero carried term in a cave: Raw %v Level %d Carried %v, want 0, 0, true", got.Raw, got.Level, got.Carried)
	}
}
