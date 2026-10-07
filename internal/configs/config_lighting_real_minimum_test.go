package configs

import "testing"

// LightRealMinimum (lighting plan 6, owner ruling O3) is the least a room with
// any real light reads. A test binary never loads config.yaml, so a zero must
// land on the default 3, and the knob must stay below LightBlindBelow so a
// sliver of light can never grant sight.
func TestLightRealMinimumValidation(t *testing.T) {
	cases := []struct {
		name  string
		blind ConfigInt
		dim   ConfigInt
		set   ConfigInt
		want  ConfigInt
	}{
		{"unset takes the default", 0, 0, 0, 3},
		{"negative takes the default", 0, 0, -4, 3},
		{"an authored 1 survives", 0, 0, 1, 1},
		{"an authored 24 survives under blind 25", 0, 0, 24, 24},
		{"at blind it clamps one below", 0, 0, 25, 24},
		{"above blind it clamps one below", 30, 60, 90, 29},
		{"blind 2 leaves 1", 2, 50, 3, 1},
		{"blind 1 leaves no floor", 1, 50, 3, 0},
		{"a negative blind leaves no floor", -10, 50, 3, 0},
	}
	for _, c := range cases {
		b := Balance{LightBlindBelow: c.blind, LightDimBelow: c.dim, LightRealMinimum: c.set}
		b.Validate()
		if b.LightRealMinimum != c.want {
			t.Errorf("%s: LightRealMinimum = %v, want %v (blind %v)", c.name, b.LightRealMinimum, c.want, b.LightBlindBelow)
		}
	}
}
