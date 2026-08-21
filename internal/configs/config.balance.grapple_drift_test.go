package configs

import "testing"

// U6b Task 14: the grapple-drift aggressor edge used to be an accidental
// 2.2-vs-2.0 skill-coefficient gap hardcoded in hooks/Position_GrappleTick.go.
// The reweight to SkillWeight deleted it; this knob restores it deliberately.
// The default 1.038 is the modelling solve that restores parity E[drift]
// ≈ +0.196 steps/round under the √2-fixed + reweighted maths
// (tools/balance/u6b_model_counters_family_costs.py). Pin it so a future
// edit cannot quietly change the shipped aggressor edge.
func TestGrappleAggressorDriftBonus_DefaultsToModelledValue(t *testing.T) {
	b := Balance{}
	b.Validate()
	if float64(b.GrappleAggressorDriftBonus) != 1.038 {
		t.Fatalf("GrappleAggressorDriftBonus default = %v, want 1.038 (modelling solve)",
			float64(b.GrappleAggressorDriftBonus))
	}
}

// A zero or negative multiplier would zero (or invert) the aggressor's whole
// drift score — a config error, not a balance decision. Validation must
// reject it back to the default.
func TestGrappleAggressorDriftBonus_RejectsNonPositive(t *testing.T) {
	b := Balance{GrappleAggressorDriftBonus: -1}
	b.Validate()
	if float64(b.GrappleAggressorDriftBonus) != 1.038 {
		t.Fatalf("GrappleAggressorDriftBonus after validating -1 = %v, want 1.038",
			float64(b.GrappleAggressorDriftBonus))
	}
}
