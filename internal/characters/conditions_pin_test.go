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
	// Base, not just Value: RecalculateStats (validate.go:29-249) only ever
	// writes .Mods and then calls StatInfo.Recalculate() (Value = Base +
	// Training + Mods), so a fixture that sets only .Value gets overwritten
	// on the first Validate() call. In a test binary the balance config is
	// all zeros, so Mods computes to 0 for every pool — without a real Base
	// here, Validate() would floor every pool at 1 and every withdrawal
	// fraction below would multiply against 1, not against a real pool.
	c.HealthMax.Base = 200
	c.HealthMax.Value = 200
	c.Health = 200
	c.StaminaMax.Base = 100
	c.StaminaMax.Value = 100
	c.Stamina = 100
	c.ConvictionMax.Base = 80
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
// every call (RecalculateStats, validate.go:29-249, called from Validate at
// validate.go:682), so the fixture reads the maximum Validate itself computes
// as the base (with a real .Base now set above, that base is exactly 200 /
// 100 / 80) and checks the withdrawal fraction against THAT base rather than
// a hardcoded number Validate could otherwise silently redefine out from
// under the test. The withdrawal application itself lives at
// validate.go:186-221.
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
	t.Run("stamina", func(t *testing.T) {
		c := pinCharacter()
		c.Validate()
		baseHealth := c.HealthMax.Value
		baseStamina := c.StaminaMax.Value

		c.AddCondition(ConditionEnchantWithdrawal, 50, 0.5, "stamina") // SETUP: Task 10
		c.Validate()

		wantStamina := baseStamina - int(math.Floor(float64(baseStamina)*0.5))
		if c.StaminaMax.Value != wantStamina {
			t.Fatalf("stamina max %d with a 0.5 withdrawal must read %d, got %d", baseStamina, wantStamina, c.StaminaMax.Value)
		}
		if c.HealthMax.Value != baseHealth {
			t.Fatalf("a stamina withdrawal must not touch health max: base %d, got %d", baseHealth, c.HealthMax.Value)
		}
	})

	t.Run("conviction", func(t *testing.T) {
		c := pinCharacter()
		c.Validate()
		baseHealth := c.HealthMax.Value
		baseConviction := c.ConvictionMax.Value

		c.AddCondition(ConditionEnchantWithdrawal, 50, 0.5, "conviction") // SETUP: Task 10
		c.Validate()

		wantConviction := baseConviction - int(math.Floor(float64(baseConviction)*0.5))
		if c.ConvictionMax.Value != wantConviction {
			t.Fatalf("conviction max %d with a 0.5 withdrawal must read %d, got %d", baseConviction, wantConviction, c.ConvictionMax.Value)
		}
		if c.HealthMax.Value != baseHealth {
			t.Fatalf("a conviction withdrawal must not touch health max: base %d, got %d", baseHealth, c.HealthMax.Value)
		}
	})
}

// TestPin_OnlyTheFirstWithdrawalApplies pins a today-behavior quirk in the
// withdrawal loop (validate.go:189-221): it iterates c.Conditions and
// `break`s after the FIRST entry whose Type is ConditionEnchantWithdrawal, so
// a second withdrawal condition on the same character is silently ignored.
// AddCondition itself collapses same-type conditions to a single entry (see
// conditions.go's overwrite-in-place loop), so reaching two live entries
// requires appending to c.Conditions directly, the way two independently
// timed disenchants overlapping would eventually produce. Task 10 must
// decide whether "only the first counts" is deliberate (a character cannot
// have two Chrysalis withdrawal penalties at once by design) or a bug worth
// fixing when withdrawal becomes a buff record instead of a single-slot
// condition list entry.
func TestPin_OnlyTheFirstWithdrawalApplies(t *testing.T) {
	c := pinCharacter()
	c.Conditions = append(c.Conditions,
		CombatCondition{Type: ConditionEnchantWithdrawal, Duration: 50, Magnitude: 0.25, Source: "health"},
		CombatCondition{Type: ConditionEnchantWithdrawal, Duration: 50, Magnitude: 0.5, Source: "stamina"},
	)
	c.Validate()

	if c.HealthMax.Value != 150 {
		t.Fatalf("first withdrawal (health, 0.25 of base 200) must read 150, got %d", c.HealthMax.Value)
	}
	if c.StaminaMax.Value != 100 {
		t.Fatalf("second withdrawal (stamina) must be silently ignored today, stamina max must stay 100, got %d", c.StaminaMax.Value)
	}
}
