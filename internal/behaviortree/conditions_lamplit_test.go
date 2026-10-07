package behaviortree

import (
	"testing"

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

	// Every round of the day it is exactly gametime.LampsLit, the test the
	// biome street lamps read, so a lamplit fixture changes in the same round.
	for r := uint64(midwinter); r < midwinter+900; r++ {
		util.SetRoundCountForTest(r)
		want := Failure
		if gametime.LampsLit() {
			want = Success
		}
		if got := condTimeOfDay(lamplit, nil); got != want {
			t.Fatalf("round %d: lamplit %v, gametime.LampsLit says %v", r, got, want)
		}
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
