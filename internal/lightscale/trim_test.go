package lightscale

import (
	"math"
	"testing"
)

func TestTrimLightLandsExactlyOnTarget(t *testing.T) {
	for _, others := range []float64{0, 20, 50, 60, 73} {
		out := Trim(8, others, 100, 74)
		if got := Combine(8, others, out); math.Abs(got-74) > 1e-9 {
			t.Errorf("others %v: Combine(others, Trim) = %v, want 74", others, got)
		}
	}
}

func TestTrimLightIsCappedAtFullStrength(t *testing.T) {
	if got := Trim(8, 0, 54, 74); got != 54 {
		t.Errorf("a weak lantern in a faint room runs at %v, want its full 54", got)
	}
}

func TestTrimLightInAnUnlitRoomIsTheTarget(t *testing.T) {
	if got := Trim(8, Absent(), 90, 74); got != 74 {
		t.Errorf("a strong glow in a cave trims to %v, want 74", got)
	}
	if got := Trim(8, Absent(), 54, 74); got != 54 {
		t.Errorf("a weak lantern in a cave runs at %v, want 54", got)
	}
}

func TestTrimLightGoesDarkWhenTheRoomIsAlreadyBright(t *testing.T) {
	for _, others := range []float64{74, 80} {
		if got := Trim(8, others, 90, 74); !math.IsInf(got, -1) {
			t.Errorf("others %v: Trim = %v, want Absent", others, got)
		}
	}
}

// A room within a hair of the target needs a term below zero, the darkest
// natural light, so the source is not needed at all rather than "lit" at a
// meaningless negative value.
func TestTrimLightJustBelowTargetIsAbsent(t *testing.T) {
	for _, d := range []float64{1e-3, 1e-9, 1e-14, 1e-15} {
		if got := Trim(8, 74-d, 90, 74); !math.IsInf(got, -1) {
			t.Errorf("others 74-%v: Trim = %v, want Absent", d, got)
		}
	}
	// Just far enough below that a real (non-negative) term is needed.
	if got := Trim(8, 70, 90, 74); !(got >= 0 && got < 74) {
		t.Errorf("others 70: Trim = %v, want a term in [0, 74)", got)
	}
}

func TestTrimNaNTargetOrMaxIsNeverNaN(t *testing.T) {
	if got := Trim(8, 20, 90, math.NaN()); !math.IsInf(got, -1) {
		t.Errorf("NaN target: Trim = %v, want Absent", got)
	}
	if got := Trim(8, 20, math.NaN(), 74); !math.IsInf(got, -1) {
		t.Errorf("NaN max: Trim = %v, want Absent", got)
	}
}

// Mirrors TestNonPositiveStepDoesNotPanicOrNaN in lightscale_test.go: a
// non-positive step must not panic or hand back NaN or +Inf. -Inf (Absent) is
// a legitimate result and is not checked against.
func TestTrimNonPositiveStepDoesNotPanicOrNaN(t *testing.T) {
	for _, step := range []float64{0, -4} {
		if got := Trim(step, 20, 90, 74); math.IsNaN(got) || math.IsInf(got, 1) {
			t.Errorf("step %v, light: Trim = %v", step, got)
		}
		if got := TrimDarkness(step, 50, 20, 40, 25); math.IsNaN(got) || math.IsInf(got, 1) {
			t.Errorf("step %v, darkness: TrimDarkness = %v", step, got)
		}
	}
}

func TestTrimNaNOthersBehavesAsAbsent(t *testing.T) {
	if got, want := Trim(8, math.NaN(), 90, 74), Trim(8, Absent(), 90, 74); got != want {
		t.Errorf("NaN others: Trim = %v, want %v (same as Absent others)", got, want)
	}
}

// The Rule 3 trim table of the 5d spec: an Umbral Lantern (full 50) entering
// alone, or beside other darkness. Every row is checked twice: the output,
// and the room it leaves, recomputed with Combine the way the room composes
// it (light, 0 when Absent, minus the combined darkness).
func TestTrimDarknessKeepsTheRoomOnTheFloor(t *testing.T) {
	absent := Absent()
	cases := []struct {
		name                    string
		light, otherDark, floor float64
		wantOut                 float64 // Absent() for off
		wantRoom                float64
	}{
		{"normal eyes, cave: off", absent, absent, 25, absent, 0},
		{"normal eyes, tavern 50", 50, absent, 25, 25, 25},
		{"normal eyes, noon 70", 70, absent, 25, 45, 25},
		{"normal eyes, light 90: full", 90, absent, 25, 50, 40},
		{"nightvision 24, light 50", 50, absent, 1, 49, 1},
		{"infravision 30, cave", absent, absent, -30, 30, -30},
		{"infravision 50, cave: full", absent, absent, -50, 50, -50},
		{"infravision 50, light 50: full", 50, absent, -50, 50, 0},
		{"normal eyes, light 50, other darkness 30: off", 50, 30, 25, absent, 20},
	}
	for _, c := range cases {
		out := TrimDarkness(8, c.light, c.otherDark, 50, c.floor)
		if math.IsInf(c.wantOut, -1) {
			if !math.IsInf(out, -1) {
				t.Errorf("%s: TrimDarkness = %v, want Absent (off)", c.name, out)
			}
		} else if math.Abs(out-c.wantOut) > 1e-9 {
			t.Errorf("%s: TrimDarkness = %v, want %v", c.name, out, c.wantOut)
		}
		light := c.light
		if math.IsInf(light, -1) {
			light = 0
		}
		dark := Combine(8, c.otherDark, out)
		if math.IsInf(dark, -1) {
			dark = 0
		}
		if room := light - dark; math.Abs(room-c.wantRoom) > 1e-9 {
			t.Errorf("%s: room after = %v, want %v", c.name, room, c.wantRoom)
		}
	}
}

// Other darkness below the budget: the source solves the halving rule, not a
// linear cut. Two darknesses of 20 and 12.9 combine to 25, not 32.9.
func TestTrimDarknessSolvesTheDarknessCombine(t *testing.T) {
	out := TrimDarkness(8, 50, 20, 50, 25)
	if !(out > 12.9 && out < 13.0) {
		t.Fatalf("TrimDarkness(light 50, other 20, floor 25) = %v, want about 12.9", out)
	}
	if got := 50 - Combine(8, 20, out); math.Abs(got-25) > 1e-9 {
		t.Errorf("room after = %v, want exactly the floor 25", got)
	}
}

func TestTrimDarknessIsCappedAtFullStrength(t *testing.T) {
	if got := TrimDarkness(8, 100, Absent(), 50, 25); got != 50 {
		t.Errorf("light 100, floor 25: TrimDarkness = %v, want its full 50", got)
	}
}

func TestTrimDarknessNaNFloorOrMaxIsAbsent(t *testing.T) {
	if got := TrimDarkness(8, 50, Absent(), 50, math.NaN()); !math.IsInf(got, -1) {
		t.Errorf("NaN floor: TrimDarkness = %v, want Absent", got)
	}
	if got := TrimDarkness(8, 50, Absent(), math.NaN(), 25); !math.IsInf(got, -1) {
		t.Errorf("NaN max: TrimDarkness = %v, want Absent", got)
	}
}

// A NaN light is read as Absent, which reads 0, exactly as an unlit room.
func TestTrimDarknessNaNLightReadsAsAnUnlitRoom(t *testing.T) {
	if got, want := TrimDarkness(8, math.NaN(), Absent(), 50, -30), TrimDarkness(8, Absent(), Absent(), 50, -30); got != want {
		t.Errorf("NaN light: TrimDarkness = %v, want %v", got, want)
	}
}
