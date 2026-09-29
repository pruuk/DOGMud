package actions

import (
	"math"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
)

// Selling baubles (docs/baubles, Phase 2).
//
// A bauble is item 900 plus a catalog id, so none of the ItemId-keyed sale
// machinery can price it or stock it: EvaluateBuyRules would refuse it (the
// carrier has no vendor_categories), and the legacy path would stock item 900
// and resell it as a generic "Curious Trinket". Baubles therefore take their
// own branch, like affixed loot does: priced from the catalog value times the
// shop buy ratio, never added to stock, and the record marked sold.

// baubleShopBuys reports whether a living-economy shop buys baubles: its
// craft_support is listed in Balance.BaubleBuyerCraftSupports (by default
// general stores and jewellers). Legacy merchants, which have no
// ShopInventory, all buy them; they are general traders by construction.
func baubleShopBuys(shopInv *shops.ShopInventory) bool {
	for _, cs := range configs.GetBalanceConfig().BaubleBuyerCraftSupports {
		if strings.EqualFold(strings.TrimSpace(cs), shopInv.CraftSupport) {
			return true
		}
	}
	return false
}

// Merchant lines for bauble refusals.
const (
	baubleSayUnknown    = "I'm afraid I don't buy those."
	baubleSayNotBuyer   = "I'm not interested in trinkets. Try a general store or a jeweller."
	baubleSayCantAfford = "I can't afford that right now."
	baubleSayHot        = "That was stolen, and not long ago. I won't touch it. Try someone less particular about where things come from."
)

// Stolen baubles (docs/baubles Phase 6c). A stolen bauble is hot for
// BaubleStolenHeatHours after its latest theft, and only in the area it was
// stolen in (baubles.Record.HotIn: its zone, or the group of zones in
// BaubleHeatAreas it belongs to). An honest merchant there will not buy a
// hot one; one anywhere else will. A fence (a merchant mob in one of
// BaubleFenceGroups) buys every bauble, and pays BaubleFenceBuyPct percent
// of the value for any stolen one not given back since
// (baubles.Record.StolenGoods), hot or cold: better than an honest
// merchant's ShopBuyRatio, which is why thieves seek fences out. An honest
// bauble sells to a fence at the ordinary price. With several merchants in
// a room a bauble goes to the best offer (resolveMerchant).

// baubleNowForSale is the clock for heat at a sale. A variable for tests.
var baubleNowForSale = time.Now

// IsFence reports whether mob is a fence: one of its groups is listed in
// Balance.BaubleFenceGroups (mobs.Mob.IsFence).
func IsFence(mob *mobs.Mob) bool {
	return mob.IsFence()
}

// FencePrice is the gold a fence pays for stolen goods worth value (a
// stolen bauble, or goods from a merchant's chest, sell_stolen.go):
// BaubleFenceBuyPct percent of it, rounded up, at least 1.
func FencePrice(value int) int {
	pct := float64(configs.GetBalanceConfig().BaubleFenceBuyPct)
	price := int(math.Ceil(float64(value) * pct / 100))
	if price < 1 {
		price = 1
	}
	return price
}

// BaubleOffer is what one merchant would pay for one bauble. Price is 0 when
// the merchant will not buy it, and Refusal is the line it says instead.
// Broke marks a refusal for lack of gold rather than lack of interest.
type BaubleOffer struct {
	Price   int
	Refusal string
	Broke   bool
}

// BaublePrice is the gold a merchant pays for a bauble worth value: the
// catalog value times the shop buy ratio, rounded up, at least 1. Same
// spread as affixed loot, and like it, no scarcity curve and no barter bonus.
func BaublePrice(value int) int {
	price := int(math.Ceil(float64(value) * shops.PricingConfigFromBalance().BuyRatio))
	if price < 1 {
		price = 1
	}
	return price
}

// baubleOfferFor decides what this merchant would pay for this bauble.
// shopInv is the merchant's living-economy shop, nil for a legacy merchant;
// fence is whether the merchant is a fence (IsFence); zone is where the
// sale is, for heat (a bauble is hot only in the area it was stolen in,
// baubles.Record.HotIn). The gold check here
// is the living-economy reserve (the same one EvaluateBuyRules applies);
// whether the merchant has the gold at all is checked at the sale, as for
// every other item.
func baubleOfferFor(item items.Item, shopInv *shops.ShopInventory, fence bool, zone string) BaubleOffer {
	rec, ok := baubles.Get(item.Bauble)
	if !ok {
		return BaubleOffer{Refusal: baubleSayUnknown}
	}

	var price int
	switch {
	case fence && rec.StolenGoods():
		price = FencePrice(rec.Value)
	case fence:
		price = BaublePrice(rec.Value) // a fence buys honest goods too, at the honest price
	case shopInv != nil && !baubleShopBuys(shopInv):
		return BaubleOffer{Refusal: baubleSayNotBuyer} // not a trinket buyer at all
	case rec.HotIn(zone, baubleNowForSale()):
		return BaubleOffer{Refusal: baubleSayHot}
	default:
		price = BaublePrice(rec.Value)
	}

	if shopInv != nil {
		ratio := float64(configs.GetBalanceConfig().ShopGoldReserveRatio)
		if ratio <= 0 {
			ratio = 0.50
		}
		if !shopInv.CanAfford(price, shopInv.GoldReserve(ratio)) {
			return BaubleOffer{Refusal: baubleSayCantAfford, Broke: true}
		}
	}
	return BaubleOffer{Price: price}
}

// baubleMerchantGold is the gold a merchant has to pay a player with: its
// living-economy shop's gold, or its own purse for a legacy merchant. The
// sale draws down the same gold (sellBaubleToMerchant).
func baubleMerchantGold(mob *mobs.Mob, shopInv *shops.ShopInventory) int {
	if shopInv != nil {
		return shopInv.Gold
	}
	return mob.Character.Gold
}

// bestBaubleMerchant is resolveMerchant for a bauble: the merchant in the
// room paying the most for it (a fence over an honest merchant for stolen
// goods), or nil when none will buy it. For a player's sale (playerSale) a
// merchant who cannot pay its offer is skipped, so a broke fence does not
// hide an honest merchant who can. Ties go to the first.
func bestBaubleMerchant(room *rooms.Room, probe items.Item, playerSale bool) (*mobs.Mob, *shops.ShopInventory) {
	var best *mobs.Mob
	var bestInv *shops.ShopInventory
	bestPrice := 0
	for _, mobId := range room.GetMobs(rooms.FindMerchant) {
		mob := mobs.GetInstance(mobId)
		if mob == nil {
			continue
		}
		shopInv := shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId)
		price := baubleOfferFor(probe, shopInv, IsFence(mob), room.Zone).Price
		if price <= bestPrice {
			continue
		}
		if playerSale && baubleMerchantGold(mob, shopInv) < price {
			continue
		}
		best, bestInv, bestPrice = mob, shopInv, price
	}
	return best, bestInv
}

// merchantZone is the zone a merchant is trading in: the zone of the room
// it stands in, or its own zone when that room cannot be loaded.
func merchantZone(mob *mobs.Mob) string {
	if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
		return room.Zone
	}
	return mob.Zone
}

// BaubleOfferFrom is baubleOfferFor for a merchant mob, resolving its shop.
// Used by the offer and appraise commands.
func BaubleOfferFrom(item items.Item, mob *mobs.Mob) BaubleOffer {
	if mob == nil || !item.IsBauble() {
		return BaubleOffer{Refusal: baubleSayUnknown}
	}
	shopInv := shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId)
	return baubleOfferFor(item, shopInv, IsFence(mob), merchantZone(mob))
}

// sellBaubleToMerchant is sellOneToMerchant's branch for a bauble. item has
// already been found in the seller's inventory and is known to be a bauble.
func sellBaubleToMerchant(seller Actor, item items.Item, room *rooms.Room,
	mob *mobs.Mob, shopInv *shops.ShopInventory,
	awardProgression bool) (soldValue int, res SellStopReason) {

	char := seller.GetCharacter()

	offer := baubleOfferFor(item, shopInv, IsFence(mob), room.Zone)
	if offer.Price <= 0 {
		merchantSay(room, mob, offer.Refusal)
		if offer.Broke {
			return 0, SellStopMerchantBroke
		}
		return 0, SellStopRejected
	}
	sellValue := offer.Price

	// Gold-model gate, exactly as for every other item: only players are
	// constrained by, and draw down, the merchant's gold. A fence is a
	// shopkeeper like any other and pays from its shop's gold.
	if seller.IsPlayer() {
		merchantGold := baubleMerchantGold(mob, shopInv)
		if merchantGold < sellValue {
			merchantSay(room, mob, baubleSayCantAfford)
			return 0, SellStopMerchantBroke
		}
		if shopInv != nil {
			shopInv.Gold -= sellValue
		} else {
			mob.Character.Gold -= sellValue
		}
	}

	char.Gold += sellValue
	char.RemoveItem(item)

	if seller.IsPlayer() {
		events.AddToQueue(events.ItemOwnership{UserId: seller.GetUserId(), Item: item, Gained: false})
		events.AddToQueue(events.EquipmentChange{UserId: seller.GetUserId(), GoldChange: sellValue})
	} else {
		events.AddToQueue(events.ItemOwnership{MobInstanceId: seller.GetMobInstanceId(), Item: item, Gained: false})
	}

	// The bauble is not stocked: it leaves the world, and its gold is the
	// only trace. The shop's gold changed, so a living-economy shop is saved.
	if shopInv != nil {
		shopInv.BuysCount++
		if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
			mudlog.Error("SELL", "msg", "SaveShop failed", "error", err)
		}
	}
	baubles.MarkSold(item.Bauble, sellValue, seller.GetUserId())

	// Progression: first sale of the command only, as for every other item
	// (see sellOneToMerchant).
	if awardProgression {
		saleProgression(seller, mob)
	}

	return sellValue, SellStopSoldAll
}
