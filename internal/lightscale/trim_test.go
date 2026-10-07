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

// A room within a hair of the target needs only a sliver of light. On the
// linear sum that sliver is a small positive term, never a negative one (the
// old log-domain solve needed a term below zero here and switched the source
// off). Either way the combine lands on the target, and when float rounding
// leaves no brightness to supply at all, the source is off.
func TestTrimLightJustBelowTargetNeedsOnlyASliver(t *testing.T) {
	slivers := 0
	for _, d := range []float64{1e-3, 1e-9, 1e-14, 1e-15} {
		got := Trim(8, 74-d, 90, 74)
		if math.IsInf(got, -1) {
			continue
		}
		slivers++
		if !(got > 0 && got < 1) {
			t.Errorf("others 74-%v: Trim = %v, want a sliver in (0, 1) or Absent", d, got)
		}
		if c := Combine(8, 74-d, got); math.Abs(c-74) > 1e-6 {
			t.Errorf("others 74-%v: Combine(others, Trim) = %v, want 74", d, c)
		}
	}
	// The near misses well clear of float rounding must solve to a sliver,
	// or the loop above proved nothing.
	if slivers < 2 {
		t.Errorf("only %d of the near misses solved to a sliver; want at least the 1e-3 and 1e-9 ones", slivers)
	}
	if got := Trim(8, 74-1e-3, 90, 74); math.IsInf(got, -1) {
		t.Errorf("others 74-1e-3: Trim is Absent, want a sliver")
	}
	if got := Trim(8, 70, 90, 74); !(got > 0 && got < 74) {
		t.Errorf("others 70: Trim = %v, want a term in (0, 74)", got)
	}
}

// The solve round-trips on the linear sum for every other light and target.
func TestTrimRoundTripsOnTheLinearSum(t *testing.T) {
	for others := 1.0; others < 90; others += 3 {
		for target := others + 0.5; target <= 100; target += 7 {
			out := Trim(8, others, 200, target)
			if math.IsInf(out, -1) {
				t.Errorf("others %v target %v: Trim is Absent, want a term", others, target)
				continue
			}
			if out < 0 {
				t.Errorf("others %v target %v: Trim = %v, below 0", others, target, out)
			}
			if got := Combine(8, others, out); math.Abs(got-target) > 1e-9 {
				t.Errorf("others %v target %v: Combine(others, Trim) = %v", others, target, got)
			}
		}
	}
}

// TrimDarkness round-trips the same way: the room it leaves sits on the floor.
//
// A case is Absent exactly when the room already sits at or below the floor
// without this source (light - Combine(otherDark) <= floor): that is asserted
// too, so a solve that went Absent everywhere fails rather than skipping.
func TestTrimDarknessRoundTripsOnTheFloor(t *testing.T) {
	solved := 0
	for light := 0.0; light <= 90; light += 10 {
		for _, otherDark := range []float64{0, 5, 12} {
			for _, floor := range []float64{-50, -30, 1, 25} {
				out := TrimDarkness(8, light, otherDark, 200, floor)
				alreadyAtFloor := light-Combine(8, otherDark) <= floor
				if math.IsInf(out, -1) {
					if !alreadyAtFloor {
						t.Errorf("light %v otherDark %v floor %v: Absent, but the room sits above the floor", light, otherDark, floor)
					}
					continue
				}
				if alreadyAtFloor {
					t.Errorf("light %v otherDark %v floor %v: Trim = %v, want Absent (already at the floor)", light, otherDark, floor, out)
				}
				solved++
				if got := light - Combine(8, otherDark, out); math.Abs(got-floor) > 1e-9 {
					t.Errorf("light %v otherDark %v floor %v: room = %v", light, otherDark, floor, got)
				}
			}
		}
	}
	if solved == 0 {
		t.Fatal("every case was Absent: the round trip tested nothing")
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

// Other darkness below the budget: the source solves the darkness combine, not
// a point cut. Two darknesses of 20 and 16.2 combine to 25, not 36.2.
func TestTrimDarknessSolvesTheDarknessCombine(t *testing.T) {
	out := TrimDarkness(8, 50, 20, 50, 25)
	if !(out > 16.1 && out < 16.3) {
		t.Fatalf("TrimDarkness(light 50, other 20, floor 25) = %v, want about 16.2", out)
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

// Trim never hands back a negative light: its result is Absent (off) or a
// term in (0, max]. A target at or below 0 needs no light, and a
// strengthless source (max at or below 0) has none to give, whatever the
// other light.
func TestTrimNeverReturnsANegativeLight(t *testing.T) {
	for _, others := range []float64{Absent(), 0, 10, 50, 80} {
		for _, max := range []float64{-20, 0, 30, 90} {
			for _, target := range []float64{-30, 0, 20, 74, math.Inf(1)} {
				got := Trim(8, others, max, target)
				if math.IsInf(got, -1) {
					continue
				}
				if !(got > 0 && got <= max) || math.IsNaN(got) {
					t.Errorf("Trim(others %v, max %v, target %v) = %v, want Absent or a term in (0, max]",
						others, max, target, got)
				}
			}
		}
	}
	for _, c := range []struct{ others, max, target float64 }{
		{Absent(), 90, 0},
		{Absent(), 90, -5},
		{0, 90, -5},
		{Absent(), -5, 74},
		{20, -5, 74},
		{80, 0, 74},
	} {
		if got := Trim(8, c.others, c.max, c.target); !math.IsInf(got, -1) {
			t.Errorf("Trim(others %v, max %v, target %v) = %v, want Absent", c.others, c.max, c.target, got)
		}
	}
}

// An unbounded target (or an unbounded budget for a darkness) needs the
// source at full strength, other light or not.
func TestTrimToAnUnboundedTargetRunsAtFullStrength(t *testing.T) {
	for _, others := range []float64{Absent(), 0, 20, 99} {
		if got := Trim(8, others, 90, math.Inf(1)); got != 90 {
			t.Errorf("others %v, target +Inf: Trim = %v, want the full 90", others, got)
		}
	}
	for _, otherDark := range []float64{Absent(), 20} {
		if got := TrimDarkness(8, math.Inf(1), otherDark, 50, 25); got != 50 {
			t.Errorf("light +Inf, other dark %v: TrimDarkness = %v, want the full 50", otherDark, got)
		}
	}
}

// A target whose brightness overflows still solves on the linear sum, rather
// than reading as "unbounded" and running the source at full strength.
func TestTrimSolvesAnOverflowingTarget(t *testing.T) {
	out := Trim(8, 50, 1e4, 9000)
	if math.Abs(out-9000) > 1e-6 {
		t.Fatalf("Trim(others 50, max 1e4, target 9000) = %v, want about 9000", out)
	}
	if got := Combine(8, 50, out); math.Abs(got-9000) > 1e-6 {
		t.Errorf("Combine(50, Trim) = %v, want 9000", got)
	}
	out = Trim(8, 8990, 1e4, 9000)
	if got := Combine(8, 8990, out); math.Abs(got-9000) > 1e-6 {
		t.Errorf("Combine(8990, Trim = %v) = %v, want 9000", out, got)
	}
}
