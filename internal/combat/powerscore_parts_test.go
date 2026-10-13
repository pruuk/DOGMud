package combat

import "testing"

// Drift guard 2 (#301): PowerScore's melee offense and avoidance ARE the
// fight read's layer 1 terms, and PowerScore is the sum of its parts.
func TestPowerScoreParts_AreTheSharedTerms(t *testing.T) {
	pinParityBalance(t)
	c := parityCombatant(t, "fighter")

	parts := powerScoreParts(*c)
	if parts.meleeOffense != meleeOffenseRaw(c, User) {
		t.Errorf("meleeOffense %v, layer 1 %v", parts.meleeOffense, meleeOffenseRaw(c, User))
	}
	if parts.avoidance != meleeAvoidance(c) {
		t.Errorf("avoidance %v, layer 1 %v", parts.avoidance, meleeAvoidance(c))
	}
	if got, want := PowerScore(*c), parts.total(); got != want {
		t.Errorf("PowerScore %v, sum of parts %v", got, want)
	}
}
