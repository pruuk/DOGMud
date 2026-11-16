package costs

import (
	"math"
	"testing"
)

// The smooth curve at the shipped defaults (#401, owner 2026-11-16):
// 1 + (5.5 - 1) x r^2.5, flat at and above capacity. Expected values are
// worked by hand from that formula, not read back from the code.
func TestEncumbranceMultiplierCurve(t *testing.T) {
	cases := []struct {
		name              string
		carried, capacity float64
		want              float64
	}{
		{"empty", 0, 100, 1.0},
		{"quarter load", 25, 100, 1.1406},      // 0.25^2.5 = 0.03125
		{"realistic load", 40, 100, 1.4554},    // 0.40^2.5 = 0.10119
		{"three quarters", 75, 100, 3.1921},    // 0.75^2.5 = 0.48714
		{"most of the way", 87.5, 100, 4.2228}, // 0.875^2.5 = 0.71618
		{"at capacity", 100, 100, 5.5},
		{"above capacity clamps", 250, 100, 5.5},
	}
	for _, c := range cases {
		got := EncumbranceMultiplier(c.carried, c.capacity)
		if math.Abs(got-c.want) > 0.001 {
			t.Errorf("%s: got %.4f, want %.4f", c.name, got, c.want)
		}
	}
}

// A zero or negative capacity must not divide by zero or hand out a discount.
func TestEncumbranceMultiplierZeroCapacity(t *testing.T) {
	if got := EncumbranceMultiplier(10, 0); got != 1.0 {
		t.Fatalf("zero capacity: got %.4f, want 1.0", got)
	}
	if got := EncumbranceMultiplier(10, -5); got != 1.0 {
		t.Fatalf("negative capacity: got %.4f, want 1.0", got)
	}
}

// A non-finite input must come out neutral, not propagate.
//
// NaN fails EVERY comparison, so it slips past the `capacity <= 0` guard and
// both range clamps and would leave the function as a NaN multiplier. That
// multiplier reaches characters.ApplyCostFloat, poisons the pool's cost carry
// permanently (int(NaN) is the minimum int64, which reads as non-positive and
// therefore free), and makes the pool cost-free for the rest of the session with
// no log and no panic. Failing neutral at the source is deliberate.
func TestEncumbranceMultiplierNonFiniteIsNeutral(t *testing.T) {
	nan := math.NaN()
	inf := math.Inf(1)
	negInf := math.Inf(-1)

	cases := []struct {
		name              string
		carried, capacity float64
	}{
		{"NaN carried", nan, 100},
		{"NaN capacity", 40, nan},
		{"both NaN", nan, nan},
		{"+Inf carried", inf, 100},
		{"-Inf carried", negInf, 100},
		{"+Inf capacity", 40, inf},
		{"-Inf capacity", 40, negInf},
	}
	for _, c := range cases {
		got := EncumbranceMultiplier(c.carried, c.capacity)
		if got != 1.0 {
			t.Errorf("%s: got %v, want exactly 1.0", c.name, got)
		}
	}
}

// Monotonically increasing: carrying more must never cost less.
func TestEncumbranceMultiplierIsMonotonic(t *testing.T) {
	prev := EncumbranceMultiplier(0, 100)
	for w := 1; w <= 150; w++ {
		got := EncumbranceMultiplier(float64(w), 100)
		if got < prev-0.0001 {
			t.Fatalf("weight %d: multiplier fell from %.4f to %.4f", w, prev, got)
		}
		prev = got
	}
}

// Carrying nothing is never penalised, whatever the capacity.
func TestEncumbranceMultiplierEmptyIsNeutral(t *testing.T) {
	for _, cap := range []float64{1, 50, 105.3, 1000} {
		if got := EncumbranceMultiplier(0, cap); math.Abs(got-1.0) > 0.0001 {
			t.Fatalf("capacity %.1f: empty load gave %.4f, want 1.0", cap, got)
		}
	}
}
