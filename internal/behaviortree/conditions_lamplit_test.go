package behaviortree

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// period: lamplit is true while the street lamps burn (gametime.LampsLit):
// at night and while the clear sky is too dim to read a face. Midwinter
// 08:00 is day, so `period: night` fails there, but its sky is dim and the
// lamps (and so a lamplit lantern) still burn.
func TestCondTimeOfDay_LampLit(t *testing.T) {
	pinAfterDuskClock(t)
	lamplit := map[string]any{"period": "lamplit"}
	night := map[string]any{"period": "night"}

	const midwinter = 355 * 900 // the first round of day 356
	for _, c := range []struct {
		name      string
		round     uint64
		lit       Result
		nightWant Result
	}{
		{"midnight", midwinter, Success, Success},
		{"08:00, day but a dim sky", midwinter + 300, Success, Failure},
		{"noon", midwinter + 450, Failure, Failure},
	} {
		util.SetRoundCountForTest(c.round)
		if got := condTimeOfDay(lamplit, nil); got != c.lit {
			t.Errorf("%s: lamplit got %v, want %v", c.name, got, c.lit)
		}
		if got := condTimeOfDay(night, nil); got != c.nightWant {
			t.Errorf("%s: night got %v, want %v; the probe is off", c.name, got, c.nightWant)
		}
	}

	// Every round of the day it follows the lamplighter's rule, written out
	// here from its definition rather than read back from gametime.LampsLit
	// (which would compare the condition with itself): night, or the clear
	// sky seen through the registered street fraction, p = step*log2(1 +
	// f*(2^(sky/step) - 1)), rounding below the faces edge. A 0.95 street is
	// registered so the fraction matters. It must also change exactly twice,
	// so a lantern on it is lit and snuffed once a day.
	prev := gametime.SetStreetLampSkyFraction(0.95)
	t.Cleanup(func() { gametime.SetStreetLampSkyFraction(prev) })
	cfg := configs.GetLightingConfig()
	changes, last := 0, Result(0)
	for r := uint64(midwinter); r < midwinter+900; r++ {
		util.SetRoundCountForTest(r)
		sky := gametime.CelestialLight()
		street := 0.0
		if sky > 0 {
			street = cfg.DoublingStep * math.Log2(1+0.95*(math.Pow(2, sky/cfg.DoublingStep)-1))
		}
		want := Failure
		if gametime.IsNight() || math.Round(street) < float64(cfg.DimBelow) {
			want = Success
		}
		got := condTimeOfDay(lamplit, nil)
		if got != want {
			t.Fatalf("round %d: lamplit %v, want %v (sky %.3f, street %.3f)", r, got, want, sky, street)
		}
		if r > midwinter && got != last {
			changes++
		}
		last = got
	}
	if changes != 2 {
		t.Errorf("lamplit changed %d times across midwinter, want 2", changes)
	}
}

// The period name is case-insensitive, as day and night are.
func TestCondTimeOfDay_LampLitIgnoresCase(t *testing.T) {
	pinAfterDuskClock(t)
	util.SetRoundCountForTest(355 * 900)
	if got := condTimeOfDay(map[string]any{"period": "LampLit"}, nil); got != Success {
		t.Errorf("period LampLit at midnight: got %v, want Success", got)
	}
}
