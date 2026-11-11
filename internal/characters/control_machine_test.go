package characters

import "testing"

// Every character gets a Control machine, mob instances included (bug, owner
// 2026-11-11). A mob instance is a shallow copy of its template whose
// machines ResetForMobInstance clears so Validate can build fresh ones;
// Validate rebuilt every machine except Control. With Control nil on the
// mob, a player-versus-mob grapple never set either side's held or
// in-control state at entry, the mob's side never moved on the drift roll,
// and the mob could never be eligible for a submission.
func TestMobInstanceGetsControlMachine(t *testing.T) {
	c := New()
	c.ResetForMobInstance()
	if c.Control != nil {
		t.Fatal("setup: ResetForMobInstance should clear Control")
	}
	c.Validate()
	if c.Control == nil {
		t.Fatal("Validate left a mob instance without a Control machine")
	}
}

// A character loaded from YAML (players and mob templates) has no Control,
// since the field is yaml:"-"; Validate must build it.
func TestValidateBuildsControlForLoadedCharacter(t *testing.T) {
	c := &Character{}
	c.Validate()
	if c.Control == nil {
		t.Fatal("Validate left a loaded character without a Control machine")
	}
}
