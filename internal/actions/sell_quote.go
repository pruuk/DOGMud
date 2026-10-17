package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
)

// Merchant lines for a sale the merchant turns down. Each says why and, where
// there is one, what the seller can do instead.
const (
	sellSayQuestItem   = "Quest items cannot be sold!"
	sellSaySpecial     = "I'm afraid I don't buy those."
	sellSayNoTrade     = "That has no value to a trader. No shop will buy it."
	sellSaySpoiled     = "That potion has gone bad. No shop will buy it now."
	sellSayWrongShop   = "I don't deal in that sort of thing. Another trader might buy it."
	sellSayOverstock   = "I have all of those I can use right now. Another trader might buy it."
	sellSayCantAfford  = "I can't afford that right now."
	sellSayNotInterest = "I'm not interested in that."
)

// SaleQuote is what one merchant would pay one seller for one item, right
// now. Price is 0 when the merchant will not buy it; then Say is the line the
// merchant speaks, or Tell a line for the seller alone (a quest item), and
// Broke marks a refusal for lack of gold rather than lack of interest.
type SaleQuote struct {
	Price int
	Say   string
	Tell  string
	Broke bool

	buyReason string // shops.BuyOffer.Reason, for the sale's gear-upgrade path
}

// QuoteSale is the ONE price a merchant puts on an item a seller holds: sell
// pays it, and offer and appraise say it. Before it, offer priced every item
// through the legacy mob.GetSellPrice while sell asked the living-economy
// shop (shops.EvaluateBuyRules), so offer quoted gold for an item sell then
// refused (2026-10-17 playtest: the Smooth Grey Token).
//
// mob is the merchant and shopInv its living-economy shop (nil for a legacy
// merchant). char is the seller, read for the bartering bonus; nil skips the
// bonus, which only resolveMerchant does, since it needs only whether the
// price is above zero. playerSale adds the purse check: a player's sale draws
// down the merchant's gold, a mob's sale never does.
func QuoteSale(char *characters.Character, room *rooms.Room, mob *mobs.Mob,
	shopInv *shops.ShopInventory, item items.Item, playerSale bool) SaleQuote {

	spec := item.GetSpec()
	if spec.ItemId < 1 || mob == nil {
		return SaleQuote{}
	}
	if spec.QuestToken != "" {
		return SaleQuote{Tell: sellSayQuestItem}
	}

	// Baubles (docs/baubles) are priced from their catalog record; see
	// sell_bauble.go.
	if item.IsBauble() {
		zone := mob.Zone
		if room != nil {
			zone = room.Zone
		}
		offer := baubleOfferFor(item, shopInv, IsFence(mob), zone)
		if offer.Price <= 0 {
			return SaleQuote{Say: offer.Refusal, Broke: offer.Broke}
		}
		if playerSale && baubleMerchantGold(mob, shopInv) < offer.Price {
			return SaleQuote{Say: baubleSayCantAfford, Broke: true}
		}
		return SaleQuote{Price: offer.Price}
	}

	// Affixed instance loot is sellable despite carrying a per-instance Spec;
	// every other custom-spec item (enchanted / blob / uses) stays blocked.
	if item.IsSpecial() && !item.Affixed {
		return SaleQuote{Say: sellSaySpecial}
	}

	var q SaleQuote
	switch {
	case item.Affixed:
		// Unique affix-scaled loot: fixed spread off its stamped value,
		// bypassing scarcity pricing and the legacy 25% cap.
		q.Price = affixedSellPrice(item, shops.PricingConfigFromBalance())
	case shopInv != nil:
		cfg := shops.PricingConfigFromBalance()
		wornItems := mob.Character.Equipment.GetAllItemsWithEmptySlots()
		offer := shops.EvaluateBuyRules(item, shopInv, mob.CrafterSkill, mob.BuysGeneral, cfg, wornItems)
		if offer.Price <= 0 {
			return SaleQuote{Say: buyRefusalLine(offer.Refused), Broke: offer.Refused == shops.BuyRefusedReserve}
		}
		q.Price = offer.Price
		q.buyReason = offer.Reason
		// A dazzled seller bargains worse (lighting plan 5b): barterDiscount
		// folds SightMult into the same BarterMaxBonus-at-skill-50 cap.
		// Balance numbers come from config.yaml, never a Go literal: the
		// sell-side cap is Balance.BarterMaxBonus (buy.go reads
		// BarterMaxDiscount instead, see tryPurchaseFromInventory).
		if char != nil {
			barterMaxBonus := float64(configs.GetBalanceConfig().BarterMaxBonus)
			if bonus := barterDiscount(char, room, barterMaxBonus); bonus > 0 {
				q.Price = shops.ApplyBarterBuyBonus(q.Price, bonus)
			}
		}
	default:
		q.Price = mob.GetSellPrice(item)
	}

	if q.Price <= 0 {
		return SaleQuote{Say: sellSayNotInterest}
	}

	// Gold-model gate: only players are constrained by, and draw down, the
	// merchant's gold. Mob sales mint the seller's payout.
	if playerSale {
		merchantGold := mob.Character.Gold
		if shopInv != nil {
			merchantGold = shopInv.Gold
		}
		if merchantGold < q.Price {
			return SaleQuote{Say: sellSayCantAfford, Broke: true}
		}
	}
	return q
}

// buyRefusalLine is the merchant's line for a living-economy shop's refusal.
func buyRefusalLine(r shops.BuyRefusal) string {
	switch r {
	case shops.BuyRefusedNoTrade:
		return sellSayNoTrade
	case shops.BuyRefusedSpoiled:
		return sellSaySpoiled
	case shops.BuyRefusedWrongShop:
		return sellSayWrongShop
	case shops.BuyRefusedOverstock:
		return sellSayOverstock
	case shops.BuyRefusedReserve:
		return sellSayCantAfford
	}
	return sellSayNotInterest
}

// SaleMerchant is the merchant a seller's item goes to: the first merchant in
// the room willing to buy it (the best offer, for a bauble), or, when none
// will, the first merchant present, whose quote then says why. nil only when
// no merchant is in the room. sell and offer both use it, so offer always
// answers for the merchant sell would deal with.
func SaleMerchant(room *rooms.Room, item items.Item, playerSale bool) (*mobs.Mob, *shops.ShopInventory) {
	if mob, shopInv := resolveMerchant(room, item, playerSale); mob != nil {
		return mob, shopInv
	}
	return firstMerchantInRoom(room)
}
