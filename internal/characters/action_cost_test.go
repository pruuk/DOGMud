package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

func actionCostCharacter(stamina, conviction int) *Character {
	c := New()
	c.Stamina = stamina
	c.Conviction = conviction
	c.carryCapacityOverride = 100
	c.Items = nil
	c.Equipment = Worn{}
	c.Skills[string(skills.UnarmedCombat)] = 25
	c.Skills[string(skills.WeaponCombat)] = 25
	c.Skills[string(skills.RangedCombat)] = 25
	c.Skills[string(skills.Rhetoric)] = 25
	return c
}

func TestQuoteActionCostDoesNotMutatePoolOrCarry(t *testing.T) {
	c := actionCostCharacter(20, 20)
	if c.costCarry != nil {
		t.Fatal("fixture unexpectedly has carry")
	}

	q := c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 2.5, Modifier: 1, Units: 1,
	})

	if !q.Affordable() {
		t.Fatal("quote should be affordable")
	}
	if c.Stamina != 20 || c.Conviction != 20 {
		t.Fatalf("quote changed pools: stamina=%d conviction=%d", c.Stamina, c.Conviction)
	}
	if c.costCarry != nil {
		t.Fatalf("quote initialized or changed carry: %#v", c.costCarry)
	}
}

func TestActionCostSelectsFixedAndEquippedCombatSkills(t *testing.T) {
	c := actionCostCharacter(100, 100)
	c.Skills[string(skills.UnarmedCombat)] = 25
	c.Skills[string(skills.RangedCombat)] = 100

	shoot := c.CommitCost(c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 10, Modifier: 1, Units: 1,
	}), CostFullOrRefuse)
	attack := c.CommitCost(c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionAttack, Pool: PoolStamina, Base: 10, Modifier: 1, Units: 1,
	}), CostFullOrRefuse)

	if shoot.Charged != 4 {
		t.Fatalf("fixed ranged rank 100 charged %d, want 4", shoot.Charged)
	}
	if attack.Charged != 10 {
		t.Fatalf("equipped unarmed rank 25 charged %d, want 10", attack.Charged)
	}
}

func TestActionCostAppliesLoadOnlyToPhysicalActions(t *testing.T) {
	quoteAndCommit := func(action costs.Action, laden bool) int {
		c := actionCostCharacter(100, 100)
		if laden {
			c.Items = []items.Item{{ItemId: 900001, Spec: &items.ItemSpec{ItemId: 900001, Weight: 100}}}
		}
		pool := PoolStamina
		if action == costs.ActionTaunt {
			pool = PoolConviction
		}
		return c.CommitCost(c.QuoteActionCost(ActionCostRequest{
			Action: action, Pool: pool, Base: 10, Modifier: 1, Units: 1,
		}), CostFullOrRefuse).Charged
	}

	if empty, laden := quoteAndCommit(costs.ActionShoot, false), quoteAndCommit(costs.ActionShoot, true); empty != 10 || laden != 50 {
		t.Fatalf("physical shoot charged empty/laden %d/%d, want 10/50", empty, laden)
	}
	if empty, laden := quoteAndCommit(costs.ActionTaunt, false), quoteAndCommit(costs.ActionTaunt, true); empty != 10 || laden != 10 {
		t.Fatalf("mental taunt charged empty/laden %d/%d, want 10/10", empty, laden)
	}
}

func TestActionCostUnitsMultiplyTheUnroundedAmount(t *testing.T) {
	c := actionCostCharacter(20, 20)
	got := c.CommitCost(c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 1.25, Modifier: 1, Units: 4,
	}), CostFullOrRefuse)
	if got.Status != CostPaid || got.Charged != 5 || c.Stamina != 15 {
		t.Fatalf("four units at 1.25 = status %v charged %d pool %d, want paid/5/15", got.Status, got.Charged, c.Stamina)
	}
}

func TestCommitCostFullPaymentUpdatesCarryAndPoolOnce(t *testing.T) {
	c := actionCostCharacter(10, 10)
	c.costCarry = map[Pool]float64{PoolStamina: 0.25}
	q := c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 1.5, Modifier: 1, Units: 1,
	})

	got := c.CommitCost(q, CostFullOrRefuse)
	if got.Status != CostPaid || got.Pool != PoolStamina || got.Charged != 1 || c.Stamina != 9 {
		t.Fatalf("first commit = %+v pool=%d, want paid 1 and pool 9", got, c.Stamina)
	}
	if carry := c.costCarry[PoolStamina]; math.Abs(carry-0.75) > 1e-9 {
		t.Fatalf("carry = %v, want 0.75", carry)
	}

	replay := c.CommitCost(q, CostFullOrRefuse)
	if replay.Status != CostRefused || replay.Charged != 0 || c.Stamina != 9 || math.Abs(c.costCarry[PoolStamina]-0.75) > 1e-9 {
		t.Fatalf("replay = %+v pool=%d carry=%v, want refused and unchanged", replay, c.Stamina, c.costCarry[PoolStamina])
	}
}

func TestCommitCostFullRefusalMutatesNothing(t *testing.T) {
	c := actionCostCharacter(1, 10)
	c.costCarry = map[Pool]float64{PoolStamina: 0.25}
	q := c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 2, Modifier: 1, Units: 1,
	})
	if q.Affordable() {
		t.Fatal("quote should be unaffordable")
	}

	got := c.CommitCost(q, CostFullOrRefuse)
	if got.Status != CostRefused || got.Charged != 0 || c.Stamina != 1 || math.Abs(c.costCarry[PoolStamina]-0.25) > 1e-9 {
		t.Fatalf("refusal = %+v pool=%d carry=%v, want unchanged", got, c.Stamina, c.costCarry[PoolStamina])
	}
}

func TestCommitCostPartialChargesAvailableWholeAndWritesOffShortage(t *testing.T) {
	c := actionCostCharacter(1, 10)
	c.costCarry = map[Pool]float64{PoolStamina: 0.25}
	got := c.CommitCost(c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 3.5, Modifier: 1, Units: 1,
	}), CostPartial)

	if got.Status != CostPartiallyPaid || !got.Short() || got.Charged != 1 || c.Stamina != 0 {
		t.Fatalf("partial commit = %+v pool=%d, want partially paid 1 and pool 0", got, c.Stamina)
	}
	if carry := c.costCarry[PoolStamina]; math.Abs(carry-0.75) > 1e-9 {
		t.Fatalf("carry = %v, want fractional remainder 0.75", carry)
	}
}

func TestCommitCostZeroWholeDueIsNoChargeNotShort(t *testing.T) {
	c := actionCostCharacter(0, 10)
	c.costCarry = map[Pool]float64{PoolStamina: 0.2}
	q := c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 0.4, Modifier: 1, Units: 1,
	})
	if !q.Affordable() {
		t.Fatal("a zero-whole quote must be affordable even on an empty pool")
	}
	got := c.CommitCost(q, CostPartial)
	if got.Status != CostNoCharge || got.Short() || got.Charged != 0 || c.Stamina != 0 {
		t.Fatalf("zero-whole commit = %+v pool=%d, want no charge and not short", got, c.Stamina)
	}
	if carry := c.costCarry[PoolStamina]; math.Abs(carry-0.6) > 1e-9 {
		t.Fatalf("carry = %v, want 0.6", carry)
	}
}

func TestActionCostInvalidAmountsBankNothingAndNonPositiveModifierIsNeutral(t *testing.T) {
	invalid := []struct {
		name  string
		base  float64
		units int
	}{
		{"zero base", 0, 1},
		{"negative base", -1, 1},
		{"nan base", math.NaN(), 1},
		{"positive infinity base", math.Inf(1), 1},
		{"negative infinity base", math.Inf(-1), 1},
		{"non-finite calculated amount", math.MaxFloat64, 2},
		{"zero units", 2, 0},
		{"negative units", 2, -1},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			c := actionCostCharacter(20, 20)
			c.costCarry = map[Pool]float64{PoolStamina: 0.25}
			got := c.CommitCost(c.QuoteActionCost(ActionCostRequest{
				Action: costs.ActionShoot, Pool: PoolStamina, Base: tt.base, Modifier: 1, Units: tt.units,
			}), CostFullOrRefuse)
			if got.Status != CostNoCharge || got.Charged != 0 || c.Stamina != 20 || math.Abs(c.costCarry[PoolStamina]-0.25) > 1e-9 {
				t.Fatalf("invalid amount commit = %+v pool=%d carry=%v, want harmless no-charge", got, c.Stamina, c.costCarry[PoolStamina])
			}
		})
	}

	for _, modifier := range []float64{0, -2, math.NaN()} {
		c := actionCostCharacter(20, 20)
		got := c.CommitCost(c.QuoteActionCost(ActionCostRequest{
			Action: costs.ActionShoot, Pool: PoolStamina, Base: 2, Modifier: modifier, Units: 1,
		}), CostFullOrRefuse)
		if got.Status != CostPaid || got.Charged != 2 {
			t.Errorf("modifier %v commit = %+v, want neutral paid charge 2", modifier, got)
		}
	}
}

func TestCommitCostRejectsStaleQuoteWithoutRepricing(t *testing.T) {
	c := actionCostCharacter(10, 10)
	c.costCarry = map[Pool]float64{PoolStamina: 0.25}
	q := c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 2, Modifier: 1, Units: 1,
	})
	c.Stamina = 9

	got := c.CommitCost(q, CostFullOrRefuse)
	if got.Status != CostRefused || got.Charged != 0 || c.Stamina != 9 || math.Abs(c.costCarry[PoolStamina]-0.25) > 1e-9 {
		t.Fatalf("stale commit = %+v pool=%d carry=%v, want refused and unchanged", got, c.Stamina, c.costCarry[PoolStamina])
	}

	// Restoring the quoted snapshot must not revive a same-owner quote after
	// its first commit attempt. Otherwise a caller can retry an admission whose
	// freshness check already rejected it.
	c.Stamina = 10
	restored := c.CommitCost(q, CostFullOrRefuse)
	if restored.Status != CostRefused || restored.Charged != 0 || c.Stamina != 10 || math.Abs(c.costCarry[PoolStamina]-0.25) > 1e-9 {
		t.Fatalf("restored stale quote = %+v pool=%d carry=%v, want consumed refusal and unchanged", restored, c.Stamina, c.costCarry[PoolStamina])
	}
}

func TestCommitCostRejectsQuoteWithChangedCarry(t *testing.T) {
	c := actionCostCharacter(10, 10)
	c.costCarry = map[Pool]float64{PoolStamina: 0.25}
	q := c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 2, Modifier: 1, Units: 1,
	})
	c.costCarry[PoolStamina] = 0.75

	got := c.CommitCost(q, CostFullOrRefuse)
	if got.Status != CostRefused || got.Charged != 0 || c.Stamina != 10 || math.Abs(c.costCarry[PoolStamina]-0.75) > 1e-9 {
		t.Fatalf("carry-stale commit = %+v pool=%d carry=%v, want refused and unchanged", got, c.Stamina, c.costCarry[PoolStamina])
	}
}

func TestCommitCostRejectsDifferentOwner(t *testing.T) {
	owner := actionCostCharacter(10, 10)
	other := actionCostCharacter(10, 10)
	q := owner.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 2, Modifier: 1, Units: 1,
	})

	got := other.CommitCost(q, CostFullOrRefuse)
	if got.Status != CostRefused || got.Charged != 0 || owner.Stamina != 10 || other.Stamina != 10 {
		t.Fatalf("cross-owner commit = %+v owner=%d other=%d, want refused and unchanged", got, owner.Stamina, other.Stamina)
	}

	ownerCommit := owner.CommitCost(q, CostFullOrRefuse)
	if ownerCommit.Status != CostPaid || ownerCommit.Charged != 2 || owner.Stamina != 8 || other.Stamina != 10 {
		t.Fatalf("owner after cross-owner attempt = %+v owner=%d other=%d, want paid/8/10", ownerCommit, owner.Stamina, other.Stamina)
	}
}

func TestCommitCostDoesNotSubtractReservationTwice(t *testing.T) {
	c := actionCostCharacter(20, 10)
	c.StaminaMax.Value = 100
	c.Equipment.Neck = items.Item{
		ItemId: 900002,
		Spec:   &items.ItemSpec{ItemId: 900002, Type: items.Neck, ReserveStaminaPct: 0.5},
	}
	if reserved := c.GetPoolReservation(string(PoolStamina), c.StaminaMax.Value); reserved != 50 {
		t.Fatalf("fixture reservation = %d, want 50", reserved)
	}

	got := c.CommitCost(c.QuoteActionCost(ActionCostRequest{
		Action: costs.ActionShoot, Pool: PoolStamina, Base: 15, Modifier: 1, Units: 1,
	}), CostFullOrRefuse)
	if got.Status != CostPaid || got.Charged != 15 || c.Stamina != 5 {
		t.Fatalf("reserved commit = %+v pool=%d, want paid 15 from current pool", got, c.Stamina)
	}
}

func TestActionCostCooldownReadyIsReadOnly(t *testing.T) {
	t.Run("nil map stays nil", func(t *testing.T) {
		c := &Character{}
		if !c.CooldownReady("special") {
			t.Fatal("missing cooldown should be ready")
		}
		if c.Cooldowns != nil {
			t.Fatalf("read-only query initialized cooldowns: %#v", c.Cooldowns)
		}
	})

	t.Run("expired entries are not pruned", func(t *testing.T) {
		c := &Character{Cooldowns: Cooldowns{"expired": 0, "past": -2, "busy": 3}}
		if !c.CooldownReady("expired") || !c.CooldownReady("past") || c.CooldownReady("busy") {
			t.Fatalf("unexpected readiness for map %#v", c.Cooldowns)
		}
		if len(c.Cooldowns) != 3 || c.Cooldowns["expired"] != 0 || c.Cooldowns["past"] != -2 || c.Cooldowns["busy"] != 3 {
			t.Fatalf("read-only query changed cooldowns: %#v", c.Cooldowns)
		}
	})
}
