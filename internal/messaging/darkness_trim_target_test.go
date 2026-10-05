package messaging

import "testing"

// The point a darkness trims to (lighting plan 5d, Rule 3): one point inside
// the bottom of an observer's usable range. The blind edge plus one for normal
// eyes, the shifted blind edge floored at windowFloor plus one for
// nightvision, minus the reach plus one for infravision.
func TestDarknessTrimTarget(t *testing.T) {
	cases := []struct {
		name            string
		strength, reach int
		want            float64
	}{
		{"normal eyes", 0, 0, 26},
		{"nightvision 12", 12, 0, 14},
		{"nightvision 24: the window floor", 24, 0, 2},
		{"nightvision past the cap clamps", 40, 0, 2},
		{"infravision 30", 12, 30, -29},
		{"infravision 50, no nightvision", 0, 50, -49},
	}
	for _, c := range cases {
		if got := DarknessTrimTarget(c.strength, c.reach, 25); got != c.want {
			t.Errorf("%s: DarknessTrimTarget(%d, %d, 25) = %v, want %v", c.name, c.strength, c.reach, got, c.want)
		}
	}
	// A blind edge low enough that the shift passes the window floor: the
	// clamp, not the shift, sets the edge, and the margin sits above it.
	if got := DarknessTrimTarget(24, 0, 10); got != windowFloor+1 {
		t.Errorf("DarknessTrimTarget(24, 0, 10) = %v, want one above the window floor %v", got, windowFloor+1)
	}
}

// The target sits one point inside the usable range: the bearer reads shapes
// at the target AND at one below it (the usable edge itself), so only a drift
// of more than one point, past the edge, tips the bearer into the dark.
func TestDarknessTrimTargetIsTheUsableEdge(t *testing.T) {
	for _, c := range []struct{ strength, reach int }{{0, 0}, {24, 0}, {0, 50}} {
		target := int(DarknessTrimTarget(c.strength, c.reach, 25))
		if got := SightThroughWindow(target, c.strength, c.reach, 25, 50); got != SightShapes {
			t.Errorf("strength %d reach %d: the target %d reads %v, want shapes", c.strength, c.reach, target, got)
		}
		edge := target - 1
		if got := SightThroughWindow(edge, c.strength, c.reach, 25, 50); got != SightShapes {
			t.Errorf("strength %d reach %d: the edge %d (one below the target) reads %v, want shapes", c.strength, c.reach, edge, got)
		}
		if got := SightThroughWindow(edge-1, c.strength, c.reach, 25, 50); got != SightNone {
			t.Errorf("strength %d reach %d: one past the edge (%d) still reads %v", c.strength, c.reach, edge-1, got)
		}
	}
}
