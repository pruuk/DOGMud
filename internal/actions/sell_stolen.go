package actions

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
)

// Selling stolen goods from merchant chests (internal/merchantchests), on
// the same rules as a stolen bauble (sell_bauble.go, docs/baubles Phase 6c):
//
//   - Taken out of a chest, a merchant's goods are stolen
//     (items.Item.IsStolen) and hot for Balance.BaubleStolenHeatHours, in
//     the heat area they were taken in only (baubles.GoodsHotIn). There an
//     honest merchant refuses them; anywhere else, or once they have
//     cooled, an honest merchant buys them as ordinary goods, at the
//     ordinary price.
//   - A fence (IsFence) buys them anywhere, hot or cold, at FencePrice
//     (Balance.BaubleFenceBuyPct of the value, the same cut as a stolen
//     bauble), and never shelves them: the network moves them on. When a
//     room has a fence, stolen goods go to it.
//   - Given back to the merchant they were taken from, they are no longer
//     stolen (StolenGoodsGiven, stolen_bauble.go).

// StolenGoodsHotHere reports whether itm is stolen goods hot in room's area
// (the rule offer quotes by).
func StolenGoodsHotHere(itm items.Item, room *rooms.Room) bool {
	return stolenGoodsHotHere(itm, room)
}

// StolenGoodsHotNow reports whether itm is hot stolen goods anywhere, on the
// stolen-goods clock (the salvage command asks).
func StolenGoodsHotNow(itm items.Item) bool {
	return baubles.GoodsHot(itm, stolenNow())
}

// MarkChestGoodsTaken records a merchant's goods leaving its chest in room,
// taken by userId, on the heat clock (items.Item.MarkTaken). Anything that
// is not a merchant's goods is left alone.
func MarkChestGoodsTaken(itm *items.Item, userId int, room *rooms.Room) {
	zone := ``
	if room != nil {
		zone = room.Zone
	}
	itm.MarkTaken(userId, zone, stolenNow())
}

// stolenGoodsHotHere reports whether itm is stolen goods hot in room's area.
func stolenGoodsHotHere(itm items.Item, room *rooms.Room) bool {
	if room == nil {
		return false
	}
	return baubles.GoodsHotIn(itm, room.Zone, baubleNowForSale())
}

// StolenGoodsRefusal is what an honest merchant says to stolen goods that
// are hot where it trades, and what the merchant they were taken from says
// on seeing its own.
func StolenGoodsRefusal(mob *mobs.Mob, item items.Item) string {
	if mob != nil && item.StolenFromMob > 0 && int(mob.MobId) == item.StolenFromMob {
		return "That's mine! Put it back where you found it, thief, before I call the watch."
	}
	return fmt.Sprintf("That was taken from %s, and not long ago. I won't touch it. Try someone less particular about where things come from.", ownerPhrase(item.StolenFrom))
}

// StolenGoodsNote is the line a thief reads on lifting goods from a chest.
func StolenGoodsNote(item items.Item) string {
	return fmt.Sprintf(`It is %s's. For a few days no honest merchant around here will touch it; a fence will take it anywhere.`, ownerPhrase(item.StolenFrom))
}

// ownerPhrase puts a merchant's name mid-sentence: a leading "A " or "The "
// becomes "the " ("A Market Hawker" reads "the market hawker").
func ownerPhrase(name string) string {
	for _, article := range []string{`A `, `An `, `The `} {
		if strings.HasPrefix(name, article) {
			return `the ` + strings.ToLower(name[len(article):])
		}
	}
	return name
}

// FenceFor returns the fence in room that would buy itm, with its
// living-economy shop (nil for a legacy merchant), or (nil, nil) when there
// is none. offer quotes through it, so it names the buyer sell would use.
func FenceFor(room *rooms.Room, itm items.Item) (*mobs.Mob, *shops.ShopInventory) {
	return fenceInRoom(room, itm)
}

// fenceInRoom returns the first fence in the room that is not the merchant
// itm was taken from, with its living-economy shop (nil for a legacy
// merchant), or (nil, nil) when there is none.
func fenceInRoom(room *rooms.Room, itm items.Item) (*mobs.Mob, *shops.ShopInventory) {
	for _, mobId := range room.GetMobs(rooms.FindMerchant) {
		mob := mobs.GetInstance(mobId)
		if mob == nil || !IsFence(mob) {
			continue
		}
		if itm.StolenFromMob > 0 && int(mob.MobId) == itm.StolenFromMob {
			continue // it will not buy back its own goods
		}
		return mob, shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId)
	}
	return nil, nil
}

// sellStolenToFence sells one stolen item to a fence at FencePrice. Players
// draw down the fence's gold like any sale; a mob seller's payout is minted,
// as in sellOneToMerchant.
func sellStolenToFence(seller Actor, item items.Item, room *rooms.Room,
	mob *mobs.Mob, shopInv *shops.ShopInventory, awardProgression bool) (int, SellStopReason) {

	// A fence will not buy back what was lifted from its own chest.
	if item.StolenFromMob > 0 && int(mob.MobId) == item.StolenFromMob {
		merchantSay(room, mob, StolenGoodsRefusal(mob, item))
		return 0, SellStopRejected
	}

	price := FencePrice(item.GetSpec().Value)

	// The living-economy reserve, as for a stolen bauble (baubleOfferFor).
	if shopInv != nil {
		ratio := float64(configs.GetBalanceConfig().ShopGoldReserveRatio)
		if ratio <= 0 {
			ratio = 0.50
		}
		if !shopInv.CanAfford(price, shopInv.GoldReserve(ratio)) {
			merchantSay(room, mob, baubleSayCantAfford)
			return 0, SellStopMerchantBroke
		}
	}

	if seller.IsPlayer() {
		gold := mob.Character.Gold
		if shopInv != nil {
			gold = shopInv.Gold
		}
		if gold < price {
			merchantSay(room, mob, baubleSayCantAfford)
			return 0, SellStopMerchantBroke
		}
		if shopInv != nil {
			shopInv.Gold -= price
		} else {
			mob.Character.Gold -= price
		}
	}

	char := seller.GetCharacter()
	char.Gold += price
	char.RemoveItem(item)

	if seller.IsPlayer() {
		events.AddToQueue(events.ItemOwnership{UserId: seller.GetUserId(), Item: item, Gained: false})
		events.AddToQueue(events.EquipmentChange{UserId: seller.GetUserId(), GoldChange: price})
	} else {
		events.AddToQueue(events.ItemOwnership{MobInstanceId: seller.GetMobInstanceId(), Item: item, Gained: false})
	}

	if shopInv != nil {
		shopInv.BuysCount++
		if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
			mudlog.Error("SELL", "msg", "SaveShop failed", "error", err)
		}
	}

	if awardProgression {
		saleProgression(seller, mob)
	}
	return price, SellStopSoldAll
}
