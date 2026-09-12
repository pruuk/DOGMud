package narration

import "testing"

// TestDefaultPickerStaysInRange pins the production picker's contract: a valid
// index for any positive n, and a safe 0 for a degenerate pool.
func TestDefaultPickerStaysInRange(t *testing.T) {
	for _, n := range []int{1, 2, 5, 50} {
		for i := 0; i < 200; i++ {
			got := DefaultPicker(n)
			if got < 0 || got >= n {
				t.Fatalf("DefaultPicker(%d) = %d, out of range", n, got)
			}
		}
	}
	if got := DefaultPicker(0); got != 0 {
		t.Errorf("DefaultPicker(0) = %d, want 0", got)
	}
	if got := DefaultPicker(-3); got != 0 {
		t.Errorf("DefaultPicker(-3) = %d, want 0", got)
	}
}

// TestSequencePickerIsDeterministic pins what the harness needs: the SAME
// sequence every run, independent of any global seed.
//
// A global seed would be process-wide, and this repo runs all tests in ONE
// binary, so a seeded run's output would depend on which other tests ran
// first. That order-dependence is the trap this injected picker exists to
// avoid, and it is why the harness must never reach for rand.Seed.
func TestSequencePickerIsDeterministic(t *testing.T) {
	first := SequencePicker()
	second := SequencePicker()

	for i := 0; i < 20; i++ {
		a, b := first(7), second(7)
		if a != b {
			t.Fatalf("call %d: two fresh sequence pickers diverged (%d vs %d)", i, a, b)
		}
		if a < 0 || a >= 7 {
			t.Fatalf("call %d: %d out of range for n=7", i, a)
		}
	}
}

// TestSequencePickerCoversThePool pins that snapshots see EVERY entry rather
// than the same one repeatedly. A picker that always returned 0 would satisfy
// determinism while freezing only a fraction of the text.
func TestSequencePickerCoversThePool(t *testing.T) {
	pick := SequencePicker()
	seen := map[int]bool{}
	for i := 0; i < 12; i++ {
		seen[pick(4)] = true
	}
	for want := 0; want < 4; want++ {
		if !seen[want] {
			t.Errorf("index %d never selected; the sequence must cover the pool", want)
		}
	}
}

// FirstPicker is for a single-variant store. It must return 0 for every n and
// must not be DefaultPicker in disguise: DefaultPicker(1) still calls
// util.Rand(1), which still calls rand.Intn, so routing ~750 single-string
// fields through it would add one global draw per narrated phase.
func TestFirstPickerAlwaysReturnsZero(t *testing.T) {
	for _, n := range []int{0, 1, 2, 5, 100} {
		if got := FirstPicker(n); got != 0 {
			t.Fatalf("FirstPicker(%d) = %d, want 0", n, got)
		}
	}
}

func TestRenderWithFirstPickerTakesTheOnlyVariant(t *testing.T) {
	roles := Render(Variants{Actee: []string{"You feel {x}."}, Observer: []string{"A glow."}},
		map[string]string{"{x}": "warm"}, FirstPicker)
	if roles.Actee != "You feel warm." || roles.Observer != "A glow." || roles.Actor != "" {
		t.Fatalf("unexpected roles: %+v", roles)
	}
}
