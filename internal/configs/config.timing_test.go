package configs

import "testing"

// AutosaveWritesPerTick bounds how many prepared files autosave commits per
// turn (roadmap chunk 3.6b-1). Zero is the dangerous value: the pending set
// never drains, so NOTHING is ever persisted while every other signal says the
// game is healthy. That is a worse failure than the pause the knob exists to
// smooth out, and it is one typo in config.yaml away, so the floor is enforced
// here as well as in savequeue.Drain.
func TestTimingValidate_AutosaveWritesPerTickIsClampedNotTrusted(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   ConfigInt
		want ConfigInt
	}{
		{"zero would never drain", 0, 3},
		{"negative would never drain", -5, 3},
		{"absent from config.yaml", 0, 3},
		{"a legitimate low value survives", 1, 1},
		{"a legitimate high value survives", 25, 25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tm := Timing{AutosaveWritesPerTick: tc.in}
			tm.Validate()
			if tm.AutosaveWritesPerTick != tc.want {
				t.Errorf("AutosaveWritesPerTick = %d, want %d", tm.AutosaveWritesPerTick, tc.want)
			}
		})
	}
}

// Validate() derives cached values from the raw ones, and a zero TurnMs would
// divide by zero. Pinning the defaults keeps the autosave arithmetic in the
// spec (3 writes x 3.46ms inside a 50ms turn) tied to what the code does.
func TestTimingValidate_DefaultsSupportTheAutosaveBudget(t *testing.T) {
	tm := Timing{}
	tm.Validate()

	if tm.TurnMs < 10 {
		t.Errorf("TurnMs = %d, want a sane default", tm.TurnMs)
	}
	if tm.TurnsPerRound() <= 0 {
		t.Errorf("TurnsPerRound() = %d, want > 0", tm.TurnsPerRound())
	}
	if tm.TurnsPerAutoSave() <= 0 {
		t.Errorf("TurnsPerAutoSave() = %d, want > 0", tm.TurnsPerAutoSave())
	}
}
