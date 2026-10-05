package messaging

import "testing"

// The bottom of an observer's usable range, the floor a darkness trims to
// (lighting plan 5d, Rule 3): the blind edge for normal eyes, the shifted
// blind edge floored at windowFloor for nightvision, minus the reach for
// infravision.
func TestDarknessTrimTarget(t *testing.T) {
	cases := []struct {
		name            string
		strength, reach int
		want            float64
	}{
		{"normal eyes", 0, 0, 25},
		{"nightvision 12", 12, 0, 13},
		{"nightvision 24: the window floor", 24, 0, 1},
		{"nightvision past the cap clamps", 40, 0, 1},
		{"infravision 30", 12, 30, -30},
		{"infravision 50, no nightvision", 0, 50, -50},
	}
	for _, c := range cases {
		if got := DarknessTrimTarget(c.strength, c.reach, 25); got != c.want {
			t.Errorf("%s: DarknessTrimTarget(%d, %d, 25) = %v, want %v", c.name, c.strength, c.reach, got, c.want)
		}
	}
	// A blind edge low enough that the shift passes the window floor: the
	// clamp, not the shift, sets the floor.
	if got := DarknessTrimTarget(24, 0, 10); got != windowFloor {
		t.Errorf("DarknessTrimTarget(24, 0, 10) = %v, want the window floor %v", got, windowFloor)
	}
}

// The floor is a usable edge: an observer standing exactly on it still reads
// shapes, and one point below reads nothing.
func TestDarknessTrimTargetIsTheUsableEdge(t *testing.T) {
	for _, c := range []struct{ strength, reach int }{{0, 0}, {24, 0}, {0, 50}} {
		floor := int(DarknessTrimTarget(c.strength, c.reach, 25))
		if got := SightThroughWindow(floor, c.strength, c.reach, 25, 50); got == SightNone {
			t.Errorf("strength %d reach %d: the floor %d reads nothing", c.strength, c.reach, floor)
		}
		if got := SightThroughWindow(floor-1, c.strength, c.reach, 25, 50); got != SightNone {
			t.Errorf("strength %d reach %d: one below the floor (%d) still reads %v", c.strength, c.reach, floor-1, got)
		}
	}
}
