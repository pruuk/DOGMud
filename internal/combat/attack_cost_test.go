package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

func attackCostFixture(t *testing.T, stamina int) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Stats.Strength.Base = 100
	c.Stats.Dexterity.Base = 100
	c.Stats.Strength.Recalculate()
	c.Stats.Dexterity.Recalculate()
	if err := c.Validate(); err != nil {
		t.Fatalf("validate attacker: %v", err)
	}
	c.StaminaMax.Value = stamina
	c.Stamina = stamina
	return c
}

func pinLiteralAttackCost(t *testing.T, base, modifier float64) {
	t.Helper()
	cfg := configs.GetConfig()
	b := &cfg.Balance
	b.AttackBaseStaminaCost = configs.ConfigFloat(base)
	b.AttackCostModifier = configs.ConfigFloat(modifier)
	b.CostSkillMultAtZero = 1
	b.CostSkillMultAtMid = 1
	b.CostSkillMultAtCap = 1
	b.CostSkillMidRank = 25
	b.CostSkillCapRank = 100
	b.CostEncumbranceKnee = 0.75
	b.CostEncumbranceKneeMult = 1
	b.CostEncumbranceMax = 1
	b.CostTotalMultiplierMax = 100
	configs.SetConfigForTest(t, cfg)
}

// TestAttackCostUsesRawBaseModifierAndUnitsOnce catches feeding a composed
// per-swing value back into ActionCostRequest.Base (which applies the modifier
// and registry terms a second time) or losing the aggregate Units count.
// Literal due: raw base 2 * neutral actor terms 1 * modifier 3 * 4 swings = 24.
func TestAttackCostUsesRawBaseModifierAndUnitsOnce(t *testing.T) {
	pinLiteralAttackCost(t, 2, 3)
	c := attackCostFixture(t, 100)

	result := ChargeAttackCost(c, 4)

	if result.Status != characters.CostPaid || result.Short() || result.Charged != 24 {
		t.Fatalf("four-swing raw commit = %+v, want paid 24", result)
	}
	if c.Stamina != 76 {
		t.Fatalf("Stamina = %d, want 76 after exactly one 24-point commit", c.Stamina)
	}
}

// TestAttackCostScalesWithPlannedSwings catches flattening the aggregate quote
// back to once per round. With every multiplier pinned neutral, Units is the
// only possible source of the literal 1:4 charge ratio.
func TestAttackCostScalesWithPlannedSwings(t *testing.T) {
	pinLiteralAttackCost(t, 1, 1)
	one := attackCostFixture(t, 100)
	four := attackCostFixture(t, 100)

	oneResult := ChargeAttackCost(one, 1)
	fourResult := ChargeAttackCost(four, 4)

	if oneResult.Charged != 1 || fourResult.Charged != 4 {
		t.Fatalf("one/four planned swings charged %d/%d, want literal 1/4",
			oneResult.Charged, fourResult.Charged)
	}
}

// TestAttackCostPartialWritesOffUnpaidWhole catches refusal, overdraw and debt
// re-banking. Four points are due, only three exist, and autoattack must spend
// those three once while reporting CostPartiallyPaid.
func TestAttackCostPartialWritesOffUnpaidWhole(t *testing.T) {
	pinLiteralAttackCost(t, 1, 1)
	c := attackCostFixture(t, 3)

	result := ChargeAttackCost(c, 4)

	if result.Status != characters.CostPartiallyPaid || !result.Short() || result.Charged != 3 {
		t.Fatalf("short four-swing commit = %+v, want partially paid 3", result)
	}
	if c.Stamina != 0 {
		t.Fatalf("Stamina = %d, want zero without overdraw", c.Stamina)
	}

	// The unpaid point was written off. A fresh one-swing quote after recovery
	// owes exactly one, not the prior shortage plus one.
	c.Stamina = 10
	next := ChargeAttackCost(c, 1)
	if next.Status != characters.CostPaid || next.Charged != 1 || c.Stamina != 9 {
		t.Fatalf("post-recovery commit = %+v stamina=%d, want one fresh point only", next, c.Stamina)
	}
}

// TestAttackCostNonPositiveUnitsAreNoCharge catches treating a zero-due round
// as short (which would incorrectly suppress hit skill) or crediting Stamina
// for a negative caller bug.
func TestAttackCostNonPositiveUnitsAreNoCharge(t *testing.T) {
	pinLiteralAttackCost(t, 1, 1)
	for _, swings := range []int{0, -1, -12} {
		c := attackCostFixture(t, 100)
		result := ChargeAttackCost(c, swings)
		if result.Status != characters.CostNoCharge || result.Short() || result.Charged != 0 {
			t.Errorf("%d swings returned %+v, want no-charge and not short", swings, result)
		}
		if c.Stamina != 100 {
			t.Errorf("%d swings moved Stamina to %d, want 100", swings, c.Stamina)
		}
	}
}

func TestAttackCostNilAttackerIsNoCharge(t *testing.T) {
	result := ChargeAttackCost(nil, 4)
	if result.Status != characters.CostNoCharge || result.Pool != characters.PoolStamina ||
		result.Short() || result.Charged != 0 {
		t.Fatalf("nil attacker returned %+v, want Stamina no-charge", result)
	}
}

func loadAttackCostFixture(t *testing.T, c *characters.Character, itemID int, fraction float64) {
	t.Helper()
	const capacity = 100.0
	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId: itemID,
		Name:   "attack cost test weight",
		Weight: capacity * fraction,
	})
	characters.ApplyMobOverrides(c, 0, 0, capacity)
	c.Items = append(c.Items, items.Item{ItemId: itemID})
}

// TestAttackCostRawQuoteUsesEquippedSkillAndPhysicalLoad catches bypassing the
// ActionAttack registry row. The real quote must select Unarmed Combat for an
// empty-handed attacker and apply physical encumbrance before committing.
func TestAttackCostRawQuoteUsesEquippedSkillAndPhysicalLoad(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.AttackBaseStaminaCost = 20
	cfg.Balance.AttackCostModifier = 1
	cfg.Balance.CostTotalMultiplierMax = 100
	configs.SetConfigForTest(t, cfg)

	novice := attackCostFixture(t, 1000)
	veteran := attackCostFixture(t, 1000)
	veteran.Skills[string(skills.UnarmedCombat)] = 100
	laden := attackCostFixture(t, 1000)
	loadAttackCostFixture(t, laden, 99863, 0.90)

	noviceResult := ChargeAttackCost(novice, 1)
	veteranResult := ChargeAttackCost(veteran, 1)
	ladenResult := ChargeAttackCost(laden, 1)
	if veteranResult.Charged >= noviceResult.Charged {
		t.Fatalf("rank-100 unarmed charge %d is not below rank-1 charge %d",
			veteranResult.Charged, noviceResult.Charged)
	}
	if ladenResult.Charged <= noviceResult.Charged {
		t.Fatalf("laden physical charge %d is not above unladen charge %d",
			ladenResult.Charged, noviceResult.Charged)
	}
}
