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

// PowerScore rates the build, not current condition (#301): a hurt character
// scores the same as a healthy one, and the caller's copy is left alone.
func TestPowerScore_RatesTheBuildAtFullPools(t *testing.T) {
	pinParityBalance(t)
	full := parityCombatant(t, "fighter")
	full.Health = full.HealthMax.Value
	full.Stamina = full.StaminaMax.Value

	hurt := *full
	hurt.Health = full.HealthMax.Value / 10
	hurt.Stamina = full.StaminaMax.Value / 10
	wantHealth, wantStamina := hurt.Health, hurt.Stamina

	if got, want := PowerScore(hurt), PowerScore(*full); got != want {
		t.Errorf("hurt PowerScore %v, full-pools PowerScore %v", got, want)
	}
	if hurt.Health != wantHealth || hurt.Stamina != wantStamina {
		t.Errorf("caller's character modified: Health %d (want %d), Stamina %d (want %d)",
			hurt.Health, wantHealth, hurt.Stamina, wantStamina)
	}
}
