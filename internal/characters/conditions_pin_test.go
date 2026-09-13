package characters

import (
	"math"
	"testing"
)

// Pins for the conditions unification (slice 1). Each literal below is what
// the OLD enum path produces at master 230041292; the migration tasks change
// only the setup lines and must leave every literal untouched.

func pinCharacter() *Character {
	c := &Character{}
	c.Buffs.Validate(true)
	c.Name = "Pin"
	c.HealthMax.Value = 200
	c.Health = 200
	c.StaminaMax.Value = 100
	c.Stamina = 100
	c.ConvictionMax.Value = 80
	c.Conviction = 80
	return c
}

func TestPin_ShieldAddsFlatPhysicalMitigation(t *testing.T) {
	c := pinCharacter()
	before := c.GetPhysicalMitigation()
	c.AddCondition(ConditionShield, 10, 12, "pin") // SETUP: migrates in Task 6
	got := c.GetPhysicalMitigation() - before
	if math.Abs(got-0.12) > 1e-9 {
		t.Fatalf("shield 12 must add exactly 0.12 mitigation, got %v", got)
	}
}

// Validate() recomputes HealthMax/StaminaMax/ConvictionMax from stats on
// every call (RecalculateStats, validate.go:29-223, called from Validate at
// validate.go:682), so the fixture's manually assigned 200/100/80 above never
// survives a Validate() call. This is the fallback shape the task calls for:
// read the maximum Validate itself computes as the base, then check the
// withdrawal fraction against THAT base rather than against a hardcoded
// number that Validate would immediately overwrite.
func TestPin_WithdrawalCutsThePoolMaximumByTheFraction(t *testing.T) {
	c := pinCharacter()
	c.Validate()
	baseHealth := c.HealthMax.Value
	baseStamina := c.StaminaMax.Value

	c.AddCondition(ConditionEnchantWithdrawal, 50, 0.25, "health") // SETUP: migrates in Task 10
	c.Validate()

	wantHealth := baseHealth - int(math.Floor(float64(baseHealth)*0.25))
	if c.HealthMax.Value != wantHealth {
		t.Fatalf("health max %d with a 0.25 withdrawal must read %d, got %d", baseHealth, wantHealth, c.HealthMax.Value)
	}
	if c.StaminaMax.Value != baseStamina {
		t.Fatalf("a health withdrawal must not touch stamina: base %d, got %d", baseStamina, c.StaminaMax.Value)
	}
}

func TestPin_WithdrawalOnStaminaAndConviction(t *testing.T) {
	c := pinCharacter()
	c.Validate()
	baseStamina := c.StaminaMax.Value

	c.AddCondition(ConditionEnchantWithdrawal, 50, 0.5, "stamina") // SETUP: Task 10
	c.Validate()

	wantStamina := baseStamina - int(math.Floor(float64(baseStamina)*0.5))
	if c.StaminaMax.Value != wantStamina {
		t.Fatalf("stamina max %d with a 0.5 withdrawal must read %d, got %d", baseStamina, wantStamina, c.StaminaMax.Value)
	}

	c2 := pinCharacter()
	c2.Validate()
	baseConviction := c2.ConvictionMax.Value

	c2.AddCondition(ConditionEnchantWithdrawal, 50, 0.5, "conviction") // SETUP: Task 10
	c2.Validate()

	wantConviction := baseConviction - int(math.Floor(float64(baseConviction)*0.5))
	if c2.ConvictionMax.Value != wantConviction {
		t.Fatalf("conviction max %d with a 0.5 withdrawal must read %d, got %d", baseConviction, wantConviction, c2.ConvictionMax.Value)
	}
}
