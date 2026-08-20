package configs

import "testing"

// CritBarCeiling 0 is LEGAL and means UNCAPPED -- the documented off-switch
// (U6b). A validator that "corrects" 0 back to 3.0 would make uncapping
// impossible; this test pins that it survives. Only a negative ceiling is
// corrected.
func TestCritBarCeiling_ZeroIsLegalUncapped(t *testing.T) {
	b := Balance{CritBarCeiling: 0}
	b.Validate()
	if b.CritBarCeiling != 0 {
		t.Fatalf("CritBarCeiling 0 is the legal uncapped off-switch and must survive validation, got %v", b.CritBarCeiling)
	}
}

func TestCritBarCeiling_NegativeIsRejected(t *testing.T) {
	b := Balance{CritBarCeiling: -1}
	b.Validate()
	if b.CritBarCeiling != 3.0 {
		t.Fatalf("a negative CritBarCeiling must be replaced by the 3.0 default, got %v", b.CritBarCeiling)
	}
}

func TestCritBarSlopeAndFloor_Defaults(t *testing.T) {
	b := Balance{CritBarSkillSlope: -0.05, CritBarFloor: 0}
	b.Validate()
	if b.CritBarSkillSlope != 0.05 {
		t.Fatalf("a negative CritBarSkillSlope must be replaced by the 0.05 default, got %v", b.CritBarSkillSlope)
	}
	if b.CritBarFloor != 1.5 {
		t.Fatalf("a non-positive CritBarFloor must be replaced by the 1.5 default, got %v", b.CritBarFloor)
	}
}

// Slope 0 is legal (a deliberately flat bar) -- only negatives are corrected.
func TestCritBarSlope_ZeroIsLegal(t *testing.T) {
	b := Balance{CritBarSkillSlope: 0}
	b.Validate()
	if b.CritBarSkillSlope != 0 {
		t.Fatalf("CritBarSkillSlope 0 (flat bar) must survive validation, got %v", b.CritBarSkillSlope)
	}
}
