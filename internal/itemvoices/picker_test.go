package itemvoices

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

// TestLineWithHonoursThePicker is what makes this store snapshottable at all.
// Before it, Line() called util.Rand directly and no test could pin its output,
// which is why itemvoices was the one message store with no golden.
func TestLineWithHonoursThePicker(t *testing.T) {
	v := &VoiceSpec{
		VoiceId: "test-voice",
		Lines:   map[string][]string{"on_equip": {"first", "second", "third"}},
	}

	if got := v.LineWith(narration.SequencePicker(), "on_equip"); got != "first" {
		t.Errorf("fresh SequencePicker should yield index 0, got %q", got)
	}

	always2 := func(n int) int { return 2 }
	if got := v.LineWith(always2, "on_equip"); got != "third" {
		t.Errorf("picker index 2 should yield the third variant, got %q", got)
	}
}

// TestLineWithNilPickerIsProduction pins the contract every other store in the
// repo uses: a nil Picker means narration.DefaultPicker, never a panic.
func TestLineWithNilPickerIsProduction(t *testing.T) {
	v := &VoiceSpec{
		VoiceId: "test-voice",
		Lines:   map[string][]string{"on_equip": {"only"}},
	}
	if got := v.LineWith(nil, "on_equip"); got != "only" {
		t.Errorf("nil picker should still render, got %q", got)
	}
}

// TestLineWithUnknownEventIsEmpty pins the existing contract Line() has, so the
// migration onto the narration core cannot quietly change it.
func TestLineWithUnknownEventIsEmpty(t *testing.T) {
	v := &VoiceSpec{VoiceId: "test-voice", Lines: map[string][]string{}}
	if got := v.LineWith(narration.SequencePicker(), "on_equip"); got != "" {
		t.Errorf("unknown event should render empty, got %q", got)
	}
}

// TestLineDelegatesToLineWith guards the wrapper: Line is what production
// calls, and a migration that changed LineWith while leaving Line pointing at
// its own copy of the old logic would pass every other test in this file.
func TestLineDelegatesToLineWith(t *testing.T) {
	v := &VoiceSpec{
		VoiceId: "test-voice",
		Lines:   map[string][]string{"on_equip": {"only"}},
	}
	if got := v.Line("on_equip"); got != "only" {
		t.Errorf("Line should render the single variant, got %q", got)
	}
	if got := v.Line("on_kill"); got != "" {
		t.Errorf("Line should render empty for an event with no pool, got %q", got)
	}
}
