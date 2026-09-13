package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
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
	defer buffs.SeedConditionRecordsForTest()()
	c := pinCharacter()
	before := c.GetPhysicalMitigation()
	_ = c.AddBuffMagnitude(buffs.BuffIdMinorShield, 10, 12, "pin") // SETUP
	c.Buffs.Validate(true)
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
	defer buffs.SeedConditionRecordsForTest()()
	c := pinCharacter()
	c.Validate()
	baseHealth := c.HealthMax.Value
	baseStamina := c.StaminaMax.Value

	_ = c.AddBuffMagnitude(buffs.BuffIdEnchantWithdrawal, 50, 0.25, "health") // SETUP: Task 10
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
		defer buffs.SeedConditionRecordsForTest()()
		c := pinCharacter()
		c.Validate()
		baseHealth := c.HealthMax.Value
		baseStamina := c.StaminaMax.Value

		_ = c.AddBuffMagnitude(buffs.BuffIdEnchantWithdrawal, 50, 0.5, "stamina") // SETUP: Task 10
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
		defer buffs.SeedConditionRecordsForTest()()
		c := pinCharacter()
		c.Validate()
		baseHealth := c.HealthMax.Value
		baseConviction := c.ConvictionMax.Value

		_ = c.AddBuffMagnitude(buffs.BuffIdEnchantWithdrawal, 50, 0.5, "conviction") // SETUP: Task 10
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

// TestPin_ASecondWithdrawalReplacesTheFirst used to pin a condition-list
// quirk: the old enum path iterated c.Conditions and broke after the first
// entry whose Type was ConditionEnchantWithdrawal, so a second entry
// appended directly to that slice was silently ignored (AddCondition itself
// never produced two live entries; only a direct append could). That
// history is gone now that withdrawal is buff record 123: a Buffs list
// holds one entry per buff id, and AddBuffMagnitude on a held id refreshes
// it in place (buffs.go's AddBuffScaled early-return branch), overwriting
// Magnitude, TriggersLeft and, via Character.AddBuffMagnitude, Source. So a
// second disenchant does not silently no-op alongside the first, it REPLACES
// it: the record that used to read "health" now reads "stamina", and only
// the stamina penalty applies.
func TestPin_ASecondWithdrawalReplacesTheFirst(t *testing.T) {
	defer buffs.SeedConditionRecordsForTest()()
	c := pinCharacter()
	c.Validate()

	_ = c.AddBuffMagnitude(buffs.BuffIdEnchantWithdrawal, 50, 0.25, "health")
	if c.HealthMax.Value != 150 {
		t.Fatalf("first withdrawal (health, 0.25 of base 200) must read 150, got %d", c.HealthMax.Value)
	}

	_ = c.AddBuffMagnitude(buffs.BuffIdEnchantWithdrawal, 50, 0.5, "stamina")
	if c.HealthMax.Value != 200 {
		t.Fatalf("a second withdrawal on the same record must replace the first: health max must return to base 200, got %d", c.HealthMax.Value)
	}
	if c.StaminaMax.Value != 50 {
		t.Fatalf("the replaced record must apply as stamina (0.5 of base 100): want 50, got %d", c.StaminaMax.Value)
	}
}
