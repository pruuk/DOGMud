package configs

import "testing"

// TestToxicityBaseMaxAcceptsExplicitZero pins the fix for a guard whose shape
// made a legal value unreachable. ToxicityBaseMax: 0 is the SHIPPED value --
// tolerance is earned entirely from alchemy and vitality -- but the old
// `if b.ToxicityBaseMax <= 0 { b.ToxicityBaseMax = 100 }` rewrote it to 100 on
// every load, restoring the flat ceiling and reverting the tolerance model with
// no error anywhere. Nothing failed; the numbers were simply wrong.
func TestToxicityBaseMaxAcceptsExplicitZero(t *testing.T) {
	b := Balance{ToxicityBaseMax: 0}
	b.validateCombat()
	if b.ToxicityBaseMax != 0 {
		t.Errorf("ToxicityBaseMax = %v, want 0 -- an explicit zero must survive validation", b.ToxicityBaseMax)
	}
}

// TestToxicityBaseMaxRejectsNegative verifies the guard still rejects nonsense.
func TestToxicityBaseMaxRejectsNegative(t *testing.T) {
	b := Balance{ToxicityBaseMax: -5}
	b.validateCombat()
	if b.ToxicityBaseMax != 0 {
		t.Errorf("ToxicityBaseMax = %v, want 0 for a negative input", b.ToxicityBaseMax)
	}
}

// TestToxicityDivisorsStillDefault verifies the <= 0 guard is KEPT on the two
// divisors. These are denominators in GetToxicityMax and must never be zero, so
// their guard shape is correct and deliberately differs from ToxicityBaseMax's.
func TestToxicityDivisorsStillDefault(t *testing.T) {
	b := Balance{}
	b.validateCombat()
	if b.ToxicityAlchemyScale != 2.5 {
		t.Errorf("ToxicityAlchemyScale = %v, want 2.5", b.ToxicityAlchemyScale)
	}
	if b.ToxicityVitalityScale != 3 {
		t.Errorf("ToxicityVitalityScale = %v, want 3", b.ToxicityVitalityScale)
	}
}
