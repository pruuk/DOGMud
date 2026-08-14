package configs

import "testing"

// The flee stamina cost was a hardcoded 10 in usercommands/flee.go until U5b-2.
// Standing rule 2 of the unified-resolution arc: no balance number lives inside
// internal/. Pin the default so a future edit to flee.go cannot quietly
// reintroduce a literal.
func TestFleeStaminaCost_DefaultsToTheOldHardcodedValue(t *testing.T) {
	b := Balance{}
	b.Validate()
	if int(b.FleeStaminaCost) != 10 {
		t.Fatalf("FleeStaminaCost default = %v, want 10 (the value flee.go hardcoded pre-U5b-2)", int(b.FleeStaminaCost))
	}
}

// A zero or negative cost would make flee free, which is a balance decision
// nobody made. Validation must reject it back to the default.
func TestFleeStaminaCost_RejectsNonPositive(t *testing.T) {
	b := Balance{FleeStaminaCost: -5}
	b.Validate()
	if int(b.FleeStaminaCost) != 10 {
		t.Fatalf("FleeStaminaCost after validating -5 = %v, want 10", int(b.FleeStaminaCost))
	}
}
