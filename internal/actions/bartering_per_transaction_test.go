package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bartering must award progression ONCE per command, not once per unit.
//
// Both faucets used to award inside their per-unit loop with no cooldown, so
// `sell all` on a 200-item stack fired 200 progression rolls from a single
// command. That made bartering unbounded in TIME, which means no uses/hour
// figure -- and therefore no progression multiplier -- could be fitted to it at
// all (U10b-0 Phase D Task 3).
//
// These assert on SkillUseCount rather than on a rank change. Progression is
// probabilistic, so an outcome-based test would be flaky; the use counter is
// incremented deterministically by TrackSkillUse on every award and is exactly
// the quantity under test.

func TestSell_MultiUnit_AwardsBarteringOnce(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 100000)()

	// Four identical sellable items, sold with one command.
	seller := newSellerActor(t, true,
		sellTestItemId, sellTestItemId, sellTestItemId, sellTestItemId)
	char := seller.GetCharacter()
	char.SkillUseCount = map[string]int{}
	char.StatUseCount = map[string]int{}

	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 4})
	require.Equal(t, 4, res.Sold, "all four should sell, or this proves nothing")

	assert.Equal(t, 1, char.SkillUseCount["bartering"],
		"a 4-unit sale must award bartering ONCE, not once per unit")
}

func TestSellAll_Sweep_AwardsBarteringOnce(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 100000)()

	seller := newSellerActor(t, true,
		sellTestItemId, sellTestItemId, sellTestItemId)
	char := seller.GetCharacter()
	char.SkillUseCount = map[string]int{}

	res := Sell(seller, SellOptions{SellAllSellable: true})
	require.Greater(t, res.Sold, 1, "the sweep must sell more than one item")

	assert.Equal(t, 1, char.SkillUseCount["bartering"],
		"a sell-all sweep is ONE transaction and must award bartering once")
}

// The merchant-side charisma roll rides the same guard, so it collapses too.
// Asserted separately because it is a different actor and a different stat, and
// because a future change could plausibly want to split them.
func TestSell_MultiUnit_AwardsMerchantCharismaOnce(t *testing.T) {
	defer seedSellItemSpecs()()
	defer seedSellRoom(t)()
	defer seedSellMerchant(t, 100000)()

	seller := newSellerActor(t, true,
		sellTestItemId, sellTestItemId, sellTestItemId)
	merchant := merchantInstance()
	require.NotNil(t, merchant, "merchant fixture must exist")
	merchant.Character.StatUseCount = map[string]int{}

	res := Sell(seller, SellOptions{ItemName: "iron sword", Quantity: 3})
	require.Equal(t, 3, res.Sold)

	assert.Equal(t, 1, merchant.Character.StatUseCount["charisma"],
		"the merchant's charisma must be rolled once per transaction")
}
