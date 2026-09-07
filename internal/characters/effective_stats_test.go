package characters

import (
	"testing"
)

// TestEffectiveStats_ToxicityPenalty verifies that GetEffectivePerception and
// GetEffectiveDexterity return the raw ValueAdj at zero toxicity and apply
// the configured penalty multipliers at high toxicity.
//
// Band values confirmed from GetToxicityPenalties (resources.go):
//
//	ratio >= 0.90 → (regenMult=0.60, perMult=0.80, dexMult=0.90)
//
// At max toxicity: ratio = 1.0 ≥ 0.90 → perMult=0.80, dexMult=0.90.
// With base stats 100: effective Per = int(100*0.80) = 80, Dex = int(100*0.90) = 90.
func TestEffectiveStats_ToxicityPenalty(t *testing.T) {
	c := &Character{}
	// ToxicityBaseMax now ships at 0 -- a bare Character has a toxicity max of
	// 0, so "c.Toxicity = c.GetToxicityMax()" below would set toxicity to 0
	// and GetToxicityPenalties would treat that as "no toxicity system" (all
	// multipliers 1.0) rather than "at max". Vitality 300 / VitalityScale 3
	// gives a real max of 100 so the ratio=1.0 band this test targets exists.
	c.Stats.Vitality.Base = 300
	c.Stats.Vitality.Recalculate()
	c.Stats.Perception.ValueAdj = 100
	c.Stats.Dexterity.ValueAdj = 100

	// At zero toxicity: no penalty; raw stats pass through unchanged.
	if got := c.GetEffectivePerception(); got != 100 {
		t.Errorf("GetEffectivePerception at tox=0: want 100, got %d", got)
	}
	if got := c.GetEffectiveDexterity(); got != 100 {
		t.Errorf("GetEffectiveDexterity at tox=0: want 100, got %d", got)
	}

	// At max toxicity (ratio = 1.0 ≥ 0.90): perMult=0.80, dexMult=0.90.
	c.Toxicity = c.GetToxicityMax()
	if got := c.GetEffectivePerception(); got != 80 {
		t.Errorf("GetEffectivePerception at max tox: want 80, got %d", got)
	}
	if got := c.GetEffectiveDexterity(); got != 90 {
		t.Errorf("GetEffectiveDexterity at max tox: want 90, got %d", got)
	}
}
