package configs

import "testing"

// A Go test binary never loads config.yaml, so validation defaults are what
// tests see. ContestFloor MUST NOT be allowed to validate to zero: a zero floor
// silently disables the whole mechanism in every Go test, which would make the
// bound property test pass vacuously while the real behaviour was untested.
func TestContestFloor_ZeroIsRejected(t *testing.T) {
	b := Balance{ContestFloor: 0}
	b.Validate()
	if b.ContestFloor != 0.125 {
		t.Fatalf("ContestFloor 0 must be replaced by the 0.125 default, got %v", b.ContestFloor)
	}
}

func TestContestFloor_AboveHalfIsRejected(t *testing.T) {
	b := Balance{ContestFloor: 0.6}
	b.Validate()
	if b.ContestFloor != 0.125 {
		t.Fatalf("ContestFloor above 0.5 stops being a last resort, got %v", b.ContestFloor)
	}
}

func TestContestFloor_LegalValueSurvives(t *testing.T) {
	b := Balance{ContestFloor: 0.2}
	b.Validate()
	if b.ContestFloor != 0.2 {
		t.Fatalf("a legal ContestFloor must survive validation, got %v", b.ContestFloor)
	}
}
