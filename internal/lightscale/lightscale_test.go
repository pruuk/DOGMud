package lightscale

import (
	"math"
	"testing"
)

// oldCombine and oldAttenuate are the pre-plan-6 log-domain operators, pinned
// as literals from master 48007844c, so the property tests can measure how far
// the rebuild moved each reading.
func oldCombine(step float64, terms ...float64) float64 {
	best := math.Inf(-1)
	for _, t := range terms {
		if t > best {
			best = t
		}
	}
	sum := 0.0
	for _, t := range terms {
		sum += math.Exp2((t - best) / step)
	}
	return best + step*math.Log2(sum)
}

func oldAttenuate(step, light, fraction float64) float64 {
	return light + step*math.Log2(fraction)
}

func TestCombineOfNothingReadsZero(t *testing.T) {
	if got := Combine(8); got != 0 {
		t.Fatalf("Combine of no terms = %v, want 0", got)
	}
	if got := Combine(8, Absent(), Absent()); got != 0 {
		t.Fatalf("Combine of two Absent terms = %v, want 0", got)
	}
}

// B(0) is 0, so two zeros are still zero. The old log-domain combine read
// them one step brighter, which is why the Absent sentinel had to exist.
func TestTwoZerosAreStillZero(t *testing.T) {
	if got := Combine(8, 0, 0); got != 0 {
		t.Fatalf("Combine(0, 0) = %v, want 0", got)
	}
}

// A single term reads its own value: p(B(x)) = x for every x >= 0.
func TestASingleTermReadsItself(t *testing.T) {
	for x := 0.0; x <= 100; x += 0.5 {
		if got := Combine(8, x); math.Abs(got-x) > 1e-9 {
			t.Errorf("Combine(%v) = %v", x, got)
		}
		if got := Combine(8, x, Absent()); math.Abs(got-x) > 1e-9 {
			t.Errorf("Combine(%v, Absent) = %v", x, got)
		}
		if got := level(8, brightness(8, x)); math.Abs(got-x) > 1e-9 {
			t.Errorf("p(B(%v)) = %v", x, got)
		}
	}
}

// Two oil lanterns: 52 and 52 read 59.9, which rounds to 60 (spec section 1).
func TestTwoLanternsReadSixty(t *testing.T) {
	got := Combine(8, 52, 52)
	if math.Abs(got-59.94) > 0.01 {
		t.Fatalf("Combine(52, 52) = %v, want about 59.94", got)
	}
	if math.Round(got) != 60 {
		t.Fatalf("Combine(52, 52) rounds to %v, want 60", math.Round(got))
	}
}

// Two equal sources read a little under one doubling step brighter, closing to
// a whole step as the sources grow.
func TestTwoEqualSourcesApproachOneStep(t *testing.T) {
	prevGap := 0.0
	for _, x := range []float64{5, 25, 50, 75} {
		gap := Combine(8, x, x) - x
		if !(gap < 8) || !(gap > prevGap) {
			t.Errorf("x %v: two equal terms add %v, want under 8 and more than at the smaller x (%v)", x, gap, prevGap)
		}
		prevGap = gap
	}
}

// Combine never reads below its brightest term, and never above the old
// log-domain combine.
func TestCombineNeverBelowTheBrightestTerm(t *testing.T) {
	sets := [][]float64{
		{0, 0}, {3, 0}, {10, 10}, {5, 5}, {70, 30}, {52, 35, 12}, {1, 2, 3, 4, 5},
		{100, 100}, {24, 24, 24, 24},
	}
	for _, s := range sets {
		got := Combine(8, s...)
		best := 0.0
		for _, v := range s {
			best = math.Max(best, v)
		}
		if got < best-1e-9 {
			t.Errorf("Combine%v = %v, below the brightest term %v", s, got, best)
		}
		if old := oldCombine(8, s...); got > old+1e-9 {
			t.Errorf("Combine%v = %v, above the old combine %v", s, got, old)
		}
	}
}

func TestCombineIsOrderIndependent(t *testing.T) {
	a := Combine(8, 12, 55, 31)
	b := Combine(8, 55, 31, 12)
	if math.Abs(a-b) > 1e-9 {
		t.Fatalf("%v != %v", a, b)
	}
}

// A source five doublings weaker than the brightest is negligible.
func TestFarWeakerSourceBarelyContributes(t *testing.T) {
	got := Combine(8, 70, 30)
	if got <= 70 || got-70 >= 0.5 {
		t.Fatalf("Combine(70, 30) = %v, want within (70, 70.5)", got)
	}
}

// The two reaches of 5 the spec quotes: 13 under the old combine, 8.5 now.
func TestTwoSmallReachesReadLower(t *testing.T) {
	if got := oldCombine(8, 5, 5); math.Abs(got-13) > 1e-9 {
		t.Fatalf("old Combine(5, 5) = %v, want 13", got)
	}
	if got := Combine(8, 5, 5); math.Abs(got-8.47) > 0.01 {
		t.Fatalf("Combine(5, 5) = %v, want about 8.47", got)
	}
}

// Well above the dim end the rebuild is invisible: the gap to the old reading
// shrinks as 2^(-p/step). The bound is step/ln2 * (n-1) * 2^(-p/step) for n
// terms, which is under 0.5 for a two-term reading at or above 37 at step 8.
//
// Below 37 the gap can pass half a point: at 25 two equal terms move about
// 1.4 points (17 and 17 read 25 before, 23.6 now).
func TestCombineMatchesTheOldScaleWellAboveTheDimEnd(t *testing.T) {
	for a := 0.0; a <= 100; a += 1 {
		for b := 0.0; b <= a; b += 1 {
			got, old := Combine(8, a, b), oldCombine(8, a, b)
			if old < 37 {
				continue
			}
			if math.Abs(got-old) >= 0.5 {
				t.Errorf("Combine(%v, %v) = %v, old %v: moved %v", a, b, got, old, old-got)
			}
		}
	}
}

func TestAttenuateNeverReadsBelowZero(t *testing.T) {
	for light := 0.0; light <= 100; light += 5 {
		for _, f := range []float64{1e-9, 0.001, 0.01, 0.1, 0.15, 0.25, 0.5, 0.95, 1} {
			if got := Attenuate(8, light, f); got < 0 {
				t.Errorf("Attenuate(%v, %v) = %v, below 0", light, f, got)
			}
		}
	}
}

// Well above the dim end a halving is very nearly one step down, and a window
// at noon reads what it did before to within half a point.
func TestAttenuateMatchesTheOldScaleWellAboveTheDimEnd(t *testing.T) {
	for light := 0.0; light <= 100; light += 1 {
		for _, f := range []float64{0.01, 0.1, 0.15, 0.25, 0.35, 0.45, 0.5, 0.75, 0.95} {
			old := oldAttenuate(8, light, f)
			if old < 37 {
				continue
			}
			if got := Attenuate(8, light, f); math.Abs(got-old) >= 0.5 {
				t.Errorf("Attenuate(%v, %v) = %v, old %v", light, f, got, old)
			}
		}
	}
	// Equinox noon (72) through an interior's 0.15 window.
	if got, old := Attenuate(8, 72, 0.15), oldAttenuate(8, 72, 0.15); math.Abs(got-old) >= 0.5 {
		t.Errorf("window at noon: %v, old %v", got, old)
	}
}

func TestAttenuateByOneIsIdentity(t *testing.T) {
	if got := Attenuate(8, 60, 1); got != 60 {
		t.Fatalf("want 60, got %v", got)
	}
}

// No sky and no light are the same statement now: both read 0.
func TestAttenuateByZeroReadsZero(t *testing.T) {
	if got := Attenuate(8, 60, 0); got != 0 {
		t.Fatalf("want 0, got %v", got)
	}
	if got := Attenuate(8, Absent(), 0.5); got != 0 {
		t.Fatalf("Attenuate of Absent = %v, want 0", got)
	}
}

// A negative light term is not light: it reads 0 and adds nothing.
func TestNegativeLightTermsAddNothing(t *testing.T) {
	if got := Combine(8, -20, 10); math.Abs(got-10) > 1e-9 {
		t.Fatalf("Combine(-20, 10) = %v, want 10", got)
	}
	if got := Attenuate(8, -20, 0.5); got != 0 {
		t.Fatalf("Attenuate(-20, 0.5) = %v, want 0", got)
	}
}

func TestNaNTermsAreSkipped(t *testing.T) {
	if got := Combine(8, math.NaN(), 10); math.Abs(got-10) > 1e-9 {
		t.Fatalf("Combine(NaN, 10) = %v, want 10", got)
	}
}

func TestNonPositiveStepDoesNotPanicOrNaN(t *testing.T) {
	if got := Combine(0, 10, 10); math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("Combine at step 0 = %v", got)
	}
	if got := Attenuate(-4, 10, 0.5); math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("Attenuate at step -4 = %v", got)
	}
}

// finiteAndAtLeast fails unless got is a finite number at or above floor.
func finiteAndAtLeast(t *testing.T, what string, got, floor float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Errorf("%s = %v, want a finite reading", what, got)
		return
	}
	if got < floor-1e-9 {
		t.Errorf("%s = %v, want at least %v", what, got, floor)
	}
}

// A term so bright its brightness overflows a float64 (p/step past 1024, an
// uncapped Glow or a tiny authored LightDoublingStep) must still read as a
// bright light, never +Inf, and never the NaN that +Inf minus +Inf gives.
func TestCombineSurvivesHugeTerms(t *testing.T) {
	for _, c := range []struct {
		step  float64
		terms []float64
	}{
		{8, []float64{9000}},
		{8, []float64{9000, 9000}},
		{8, []float64{9000, 50}},
		{8, []float64{1e300, 1e300, 3}},
		{1e-3, []float64{52}},
		{1e-3, []float64{52, 35}},
		{1e-300, []float64{52, 52}},
		{5e-324, []float64{10, 20}},
	} {
		got := Combine(c.step, c.terms...)
		best := 0.0
		for _, v := range c.terms {
			best = math.Max(best, v)
		}
		finiteAndAtLeast(t, "Combine", got, best)
		// Two terms add at most one step to the brightest.
		if got > best+c.step*math.Log2(float64(len(c.terms)))+1e-9*best {
			t.Errorf("Combine(step %v, %v) = %v, more than log2(n) steps above %v", c.step, c.terms, got, best)
		}
	}
	// Two equal huge terms read exactly one step brighter than one.
	if got := Combine(8, 9000, 9000); math.Abs(got-9008) > 1e-9 {
		t.Errorf("Combine(9000, 9000) = %v, want 9008", got)
	}
	// At a tiny step a weaker term is invisible beside a brighter one.
	if got := Combine(1e-3, 52, 35); math.Abs(got-52) > 1e-9 {
		t.Errorf("Combine(step 1e-3, 52, 35) = %v, want 52", got)
	}
}

func TestAttenuateSurvivesHugeTerms(t *testing.T) {
	for _, c := range []struct{ step, light, fraction float64 }{
		{8, 9000, 0.5},
		{8, 9000, 0.95},
		{8, 9000, 1e-300},
		{8, 1e300, 0.5},
		{1e-3, 52, 0.95},
		{1e-3, 52, 1e-9},
		{5e-324, 52, 0.5},
	} {
		got := Attenuate(c.step, c.light, c.fraction)
		finiteAndAtLeast(t, "Attenuate", got, 0)
		if got > c.light+1e-9 {
			t.Errorf("Attenuate(step %v, %v, %v) = %v, above the light", c.step, c.light, c.fraction, got)
		}
	}
	// Far above the dim end a halving is exactly one step down.
	if got := Attenuate(8, 9000, 0.5); math.Abs(got-8992) > 1e-9 {
		t.Errorf("Attenuate(9000, 0.5) = %v, want 8992", got)
	}
}

// Near 0 the arithmetic must not lose the light to cancellation: a sliver
// still reads itself and two slivers read about twice one.
func TestTinyTermsKeepTheirPrecision(t *testing.T) {
	for _, x := range []float64{1e-12, 1e-9, 1e-6} {
		if got := Combine(8, x); math.Abs(got-x) > x*1e-9 {
			t.Errorf("Combine(%v) = %v", x, got)
		}
		if got := Combine(8, x, x); math.Abs(got-2*x) > x*1e-6 {
			t.Errorf("Combine(%v, %v) = %v, want about %v", x, x, got, 2*x)
		}
		if got := Attenuate(8, x, 0.5); math.Abs(got-x/2) > x*1e-6 {
			t.Errorf("Attenuate(%v, 0.5) = %v, want about %v", x, got, x/2)
		}
	}
}
