package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// calculateCombat must not copy its combatants. A copy silently discards every
// in-place mutation its callees make, which is how melee defence came to cost
// nothing: runBestOfAllDefense charged a discarded struct for years.
func TestCalculateCombatDoesNotDiscardDefenderCharges(t *testing.T) {
	pinContestFloorOff(t)

	atk := characters.New()
	atk.Stats.Strength.Base = 200
	atk.Stats.Dexterity.Base = 200
	atk.Stats.Strength.Recalculate()
	atk.Stats.Dexterity.Recalculate()
	atk.Validate()
	atk.HealthMax.Value = 1000
	atk.Health = 1000
	atk.StaminaMax.Value = 1000
	atk.Stamina = 1000
	// calculateCombat reads sourceChar.Aggro.Type unguarded; Aggro is a nil
	// pointer on a fresh character and every production caller is mid-combat.
	// DefaultAttack, not SurpriseAttack: a surprise attack crits every swing
	// and would not exercise the defence path we are measuring.
	atk.SetAggro(0, 0, characters.DefaultAttack)

	def := characters.New()
	def.Stats.Dexterity.Base = 100
	def.Stats.Vitality.Base = 100
	def.Stats.Dexterity.Recalculate()
	def.Stats.Vitality.Recalculate()
	def.Validate()
	def.HealthMax.Value = 5000
	def.Health = 5000
	def.StaminaMax.Value = 500
	def.Stamina = 500

	before := def.Stamina
	for i := 0; i < 10; i++ {
		_ = calculateCombat(atk, def, User, Mob, combatContext{
			sourceCanSee: true, targetCanSee: true,
		})
	}

	if def.Stamina >= before {
		t.Fatalf("defender paid nothing across 10 rounds: stamina %d -> %d", before, def.Stamina)
	}
}
