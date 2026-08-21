package configs

import "testing"

// U10 knobs: concentration floor/threshold and the three knockdown factors
// (anchored to the RATIFIED intended rates — see the spec's section 4; the
// old threshold knobs never delivered the rates their comments claimed).
func TestU10KnobDefaults(t *testing.T) {
	b := Balance{}
	b.Validate()

	if got := float64(b.ConcentrationFloor); got != 0.02 {
		t.Errorf("ConcentrationFloor default = %v, want 0.02", got)
	}
	if got := int(b.ConcentrationDamageThresholdPct); got != 10 {
		t.Errorf("ConcentrationDamageThresholdPct default = %d, want 10", got)
	}
	if got := float64(b.BashKnockdownFactor); got != 1.0 {
		t.Errorf("BashKnockdownFactor default = %v, want 1.0", got)
	}
	if got := float64(b.TripKnockdownFactor); got != 1.057 {
		t.Errorf("TripKnockdownFactor default = %v, want 1.057", got)
	}
	if got := float64(b.KickKnockdownFactor); got != 0.924 {
		t.Errorf("KickKnockdownFactor default = %v, want 0.924", got)
	}
}

func TestU10ConcentrationFloorRejection(t *testing.T) {
	b := Balance{ConcentrationFloor: 0.6}
	b.Validate()
	if got := float64(b.ConcentrationFloor); got != 0.02 {
		t.Errorf("ConcentrationFloor 0.6 should be rejected to 0.02, got %v", got)
	}
}
