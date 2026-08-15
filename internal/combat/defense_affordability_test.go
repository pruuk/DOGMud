package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// defenceFixture builds an attacker and a defender with known stats and a
// defender stamina pool the caller controls.
//
// ValueAdj is set directly rather than via Base+Recalculate because stat
// compression was removed on 2026-08-02: ValueAdj == Value always, and the
// combat scoring path reads ValueAdj. This matches avoidance_test.go's idiom.
// characters.New() initialises Position, Buffs, Cooldowns and Stats and calls
// Validate(), so the fixture survives the positional checks in the loop.
func defenceFixture(defenderStamina int) (*characters.Character, *characters.Character) {
	attacker := characters.New()
	attacker.Stats.Strength.ValueAdj = 100
	attacker.Stats.Dexterity.ValueAdj = 100

	defender := characters.New()
	defender.Stats.Dexterity.ValueAdj = 100
	defender.StaminaMax.Base = 100
	defender.StaminaMax.Recalculate()
	defender.Stamina = defenderStamina

	return attacker, defender
}

// U5b-2: an exhausted defender must still get to defend.
//
// Before this chunk the affordability gate `continue`d an unaffordable defence
// out of the candidate set. With every defence unaffordable the entry list came
// out empty, contest.Run reported uncontested, and the swing fell through to the
// MinDefenseChance last-resort branch -- a flat 15% save, always narrated as a
// dodge, with no possibility of a defence crit. After this change the defender
// rolls a real contest.
func TestRunBestOfAllDefense_ExhaustedDefenderStillEntersTheContest(t *testing.T) {
	attacker, defender := defenceFixture(0)

	result := &AttackResult{}
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	best := runBestOfAllDefense(result, attacker, defender,
		[]string{characters.DefenseDodge}, 100.0, false, ctx)

	if best.defenseType == "" {
		t.Fatal("an exhausted defender was dropped from the candidate set; " +
			"they must enter the contest and be charged partially instead")
	}
	if math.IsInf(best.margin, -1) {
		t.Error("margin is the uncontested sentinel; the contest did not run")
	}
}

// The winner is charged whatever is actually there. A defence must never come
// out free just because the defender could not pay in full.
//
// U7 Task 6 rewrote the setup. Under the old base x multiplier formula block
// cost a flat int(5 * 0.9) = 4, so a 1-stamina fixture was short by
// construction. It is now DefenceBaseStaminaCost x encumbrance x inverse skill
// x BlockCostModifier, which for an unladen rank-0 defender is about 1.27 --
// and ApplyCostFloat floors that to a charge of 1, which the fixture can pay in
// full, leaving the assertion vacuous. Loading the defender to capacity puts
// the encumbrance multiplier on it and prices block well past what they hold.
// Capacity is overridden rather than derived from Strength so the ratio does
// not move when the stat defaults do.
func TestRunBestOfAllDefense_PartiallyChargesAnExhaustedWinner(t *testing.T) {
	attacker, defender := defenceFixture(1)

	const heavyItemId = 99801
	items.RegisterTestItemSpec(&items.ItemSpec{
		ItemId: heavyItemId,
		Name:   "test lead pig",
		Weight: 40.0,
	})
	characters.ApplyMobOverrides(defender, 0, 0, 40.0)
	defender.Items = append(defender.Items, items.Item{ItemId: heavyItemId})

	cost := defender.GetDefenseCostFloat(characters.DefenseBlock)
	if cost <= 1 {
		t.Fatalf("block costs %.3f; this test needs a cost above the 1 stamina on hand", cost)
	}

	result := &AttackResult{}
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	best := runBestOfAllDefense(result, attacker, defender,
		[]string{characters.DefenseBlock}, 100.0, false, ctx)

	if best.defenseType == "" {
		t.Fatal("exhausted defender was dropped from the candidate set")
	}
	if defender.Stamina != 0 {
		t.Errorf("stamina after a partial defence charge = %d, want 0 (charged down to empty, not refused)", defender.Stamina)
	}
}
