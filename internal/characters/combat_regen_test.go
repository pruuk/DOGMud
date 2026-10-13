package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
)

// With no Regenerating record nothing heals in combat (NewRound_AutoHeal has
// no base regen in combat), and with one the record multiplies base regen.
func TestCombatHealthRegenPerRound(t *testing.T) {
	c := New()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.HealthMax.Value = 1000
	if got := c.CombatHealthRegenPerRound(); got != 0 {
		t.Fatalf("no record: got %v, want 0", got)
	}

	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		9201: {ConditionId: 9201, Name: "test regen", TriggerCount: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectRegenMult: {Literal: 3}}},
	}))
	c.Conditions.List = append(c.Conditions.List, &conditions.Condition{ConditionId: 9201, TriggersLeft: 1000})
	c.Conditions.Validate(true)

	want := float64(c.HealthPerRound()) * 3
	if got := c.CombatHealthRegenPerRound(); got != want {
		t.Fatalf("with record: got %v, want %v", got, want)
	}
}
