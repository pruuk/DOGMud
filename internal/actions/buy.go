package actions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// BuyOptions controls how a purchase is attempted.
type BuyOptions struct {
	// Request is the raw "rest" string, e.g. "5 iron ingot from marko".
	// Quantity prefix and "from <merchant>" suffix are parsed inside Buy,
	// so wrappers pass through whatever the player typed.
	Request string

	// TargetMerchantUserId, when > 0, restricts merchant selection to a
	// specific user merchant. Wrappers may set this directly for cases
	// where the caller has already resolved the target.
	TargetMerchantUserId int

	// TargetMerchantMobInstanceId, when > 0, restricts merchant selection
	// to a specific mob merchant.
	TargetMerchantMobInstanceId int
}

// BuyResult is the outcome of an attempted purchase.
type BuyResult struct {
	Success   bool   // at least one unit purchased
	Purchased int    // actual units purchased (may be < Requested)
	Requested int    // requested quantity (1 if unspecified)
	SaleType  string // "item" | "condition" | "" on failure
	Reason    string // populated on failure
}

// Failure-reason vocabulary returned in BuyResult.Reason.
const (
	BuyReasonNoRequest        = "no_request"
	BuyReasonNoMerchant       = "no_merchant"
	BuyReasonNoMatch          = "no_match"
	BuyReasonOutOfStock       = "out_of_stock"
	BuyReasonInsufficientGold = "insufficient_gold"
	BuyReasonMissingTradeItem = "missing_trade_item"
	BuyReasonOverburdened     = "overburdened"
	BuyReasonSelfTarget       = "self_target"
	BuyReasonNoSight          = "no_sight"
)

// legacyShopCatalog enumerates the in-stock items + conditions offered by a
// legacy Character.Shop. Merc and pet sale types are intentionally NOT
// surfaced (spec 2.1 drops them).
type legacyShopCatalog struct {
	nameToShopItem  map[string]characters.ShopItem
	itemNames       []string
	itemNamesFancy  []string
	itemPrices      map[int]int
	conditionNames  []string
	conditionPrices map[int]int
}

func buildLegacyCatalog(saleItems characters.Shop) legacyShopCatalog {
	cat := legacyShopCatalog{
		nameToShopItem:  map[string]characters.ShopItem{},
		itemPrices:      map[int]int{},
		conditionPrices: map[int]int{},
	}

	for _, saleItem := range saleItems {
		if saleItem.ItemId > 0 {
			item := items.New(saleItem.ItemId)
			if item.ItemId == 0 {
				continue
			}
			cat.itemNames = append(cat.itemNames, item.GetSpec().Name)
			cat.itemNamesFancy = append(cat.itemNamesFancy, item.DisplayName())
			cat.nameToShopItem[item.GetSpec().Name] = saleItem

			price := saleItem.Price
			if price == 0 {
				price = item.GetSpec().Value
			} else if price < 0 {
				price = 0
			}
			cat.itemPrices[saleItem.ItemId] = price
			continue
		}
		if saleItem.ConditionId > 0 {
			conditionInfo := conditions.GetConditionSpec(saleItem.ConditionId)
			if conditionInfo == nil {
				continue
			}
			cat.conditionNames = append(cat.conditionNames, conditionInfo.Name)
			cat.nameToShopItem[conditionInfo.Name] = saleItem

			price := saleItem.Price
			if price == 0 {
				price = 1000
			} else if price < 0 {
				price = 0
			}
			cat.conditionPrices[saleItem.ConditionId] = price
			continue
		}
		// Merc / pet entries on legacy shops are skipped — see spec 2.1.
	}
	return cat
}

// allNames returns the union of item + condition display names in the catalog
// for fuzzy matching. Merc/pet names are intentionally excluded.
func (c *legacyShopCatalog) allNames() []string {
	all := make([]string, 0, len(c.itemNames)+len(c.conditionNames))
	all = append(all, c.itemNames...)
	all = append(all, c.conditionNames...)
	return all
}

// purchaseContext holds the validated state of a purchase ready
// for execution.
type purchaseContext struct {
	matchedShopItem characters.ShopItem
	price           int
	tradeInString   string
}

// validatePurchase runs every pre-side-effect check, then applies
// side effects (destock, gold deduction, trade-item consume) only
// when all checks pass. Returns ok=false plus a populated Reason
// when any check fails; on failure no state is mutated.
func validatePurchase(
	buyer Actor,
	shopMob *mobs.Mob,
	shopUser *users.UserRecord,
	matchedShopItem characters.ShopItem,
	itemPrices map[int]int,
	conditionPrices map[int]int,
) (purchaseContext, string, bool) {

	char := buyer.GetCharacter()

	// (1) Encumbrance gate — item purchases only.
	if matchedShopItem.ItemId > 0 {
		newItm := items.New(matchedShopItem.ItemId)
		weight := newItm.GetSpec().Weight
		if char.GetCarriedWeight()+weight > char.CarryCapacity() {
			if buyer.IsPlayer() {
				buyer.SendText(messaging.CategoryError, "You can't carry any more.")
			}
			return purchaseContext{}, BuyReasonOverburdened, false
		}
	}

	// (2) Stock check.
	if !matchedShopItem.Available() {
		if shopMob != nil {
			shopMob.Command(`say I don't have that for sale right now.`)
		} else if shopUser != nil && buyer.IsPlayer() {
			buyer.SendText(messaging.CategoryError, fmt.Sprintf(`<ansi fg="username">%s</ansi> doesn't have that for sale right now.`, shopUser.Character.Name))
		}
		return purchaseContext{}, BuyReasonOutOfStock, false
	}

	// (3) Price lookup.
	price := 0
	if matchedShopItem.ItemId > 0 {
		price = itemPrices[matchedShopItem.ItemId]
	} else if matchedShopItem.ConditionId > 0 {
		price = conditionPrices[matchedShopItem.ConditionId]
	}

	// (4) Gold check.
	if char.Gold < price {
		sendMerchantMessage(buyer, shopMob, shopUser,
			`say You don't have enough gold for that.`,
			`You don't have enough gold for that.`)
		return purchaseContext{}, BuyReasonInsufficientGold, false
	}

	// (5) Trade-item check.
	tradeItemName := ""
	if matchedShopItem.TradeItemId > 0 {
		tradeItm := items.New(matchedShopItem.TradeItemId)
		tradeItemName = tradeItm.Name()
		if _, found := char.FindInBackpack(tradeItemName); !found {
			if buyer.IsPlayer() {
				buyer.SendText(messaging.CategoryError, fmt.Sprintf(`You must have a <ansi fg="itemname">%s</ansi> to trade for that.`, tradeItm.DisplayName()))
			}
			return purchaseContext{}, BuyReasonMissingTradeItem, false
		}
	}

	// All checks passed — apply side effects.
	if shopMob != nil {
		if !shopMob.Character.Shop.Destock(matchedShopItem) {
			shopMob.Command(`say I don't have that item right now.`)
			return purchaseContext{}, BuyReasonOutOfStock, false
		}
	} else if shopUser != nil {
		if !shopUser.Character.Shop.Destock(matchedShopItem) {
			if buyer.IsPlayer() {
				buyer.SendText(messaging.CategoryError, `That's not for sale.`)
			}
			return purchaseContext{}, BuyReasonOutOfStock, false
		}
	}

	if buyer.IsPlayer() {
		events.AddToQueue(events.EquipmentChange{
			UserId:     buyer.GetUserId(),
			GoldChange: -price,
		})
	}

	char.Gold -= price
	if shopMob != nil {
		shopMob.Character.Gold += 1 // legacy +1 cheat preserved
	} else if shopUser != nil {
		shopUser.Character.Gold += price
		events.AddToQueue(events.EquipmentChange{
			UserId:     shopUser.UserId,
			GoldChange: price,
		})
	}

	tradeInString := ""
	if price > 0 {
		tradeInString = fmt.Sprintf(`<ansi fg="gold">%s gold</ansi>`, util.FormatNumber(price))
	}
	if tradeItemName != "" {
		if itm, found := char.FindInBackpack(tradeItemName); found {
			char.RemoveItem(itm)
			if buyer.IsPlayer() {
				events.AddToQueue(events.ItemOwnership{
					UserId: buyer.GetUserId(),
					Item:   itm,
					Gained: false,
				})
			} else {
				events.AddToQueue(events.ItemOwnership{
					MobInstanceId: buyer.GetMobInstanceId(),
					Item:          itm,
					Gained:        false,
				})
			}

			if tradeInString != "" {
				tradeInString += fmt.Sprintf(` and a <ansi fg="itemname">%s</ansi>`, itm.DisplayName())
			} else {
				tradeInString = fmt.Sprintf(`a <ansi fg="itemname">%s</ansi>`, itm.DisplayName())
			}
		}
	}
	if tradeInString == "" {
		tradeInString = "nothing"
	}

	return purchaseContext{
		matchedShopItem: matchedShopItem,
		price:           price,
		tradeInString:   tradeInString,
	}, "", true
}

// sendMerchantMessage delivers a message to the buyer, branching on
// mob vs player merchant. Mob merchants speak; player merchants send
// to the buyer directly.
func sendMerchantMessage(buyer Actor, shopMob *mobs.Mob, shopUser *users.UserRecord, mobMsg string, userMsg string) {
	if shopMob != nil {
		shopMob.Command(mobMsg)
	} else if shopUser != nil && buyer.IsPlayer() {
		buyer.SendText(messaging.CategoryError, userMsg)
	}
}

// Buy executes a purchase on behalf of buyer. See package context for
// the full flow.
func Buy(buyer Actor, opts BuyOptions) BuyResult {
	req := strings.TrimSpace(opts.Request)
	if req == "" {
		return BuyResult{Reason: BuyReasonNoRequest, Requested: 1}
	}

	// Parse leading quantity: "buy 5 iron ingot".
	quantity := 1
	args0 := strings.SplitN(strings.TrimSpace(req), " ", 2)
	if len(args0) == 2 {
		if n, err := strconv.Atoi(args0[0]); err == nil && n >= 1 {
			quantity = n
			req = args0[1]
		}
	}

	room := buyer.GetRoom()
	if room == nil {
		return BuyResult{Reason: BuyReasonNoMerchant, Requested: quantity}
	}

	// Parse trailing "from <name>" clause to resolve a specific merchant.
	itemRequest := req
	targetUserId := opts.TargetMerchantUserId
	targetMobInstanceId := opts.TargetMerchantMobInstanceId

	args := util.SplitButRespectQuotes(strings.ToLower(req))
	if len(args) >= 3 && args[len(args)-2] == "from" {
		mercName := args[len(args)-1]
		exclude := ResolveTargetOptions{}
		if buyer.IsPlayer() {
			exclude.ExcludeUserId = buyer.GetUserId()
			exclude.Viewer = buyer.GetCharacter()
		}
		target, terr := ResolveTargetActor(room, mercName, exclude)
		if terr == nil {
			if target.IsPlayer() {
				targetUserId = target.(*UserActor).User.UserId
			} else {
				targetMobInstanceId = target.(*MobActor).Mob.InstanceId
			}
			itemRequest = strings.Join(args[:len(args)-2], " ")
		} else if buyer.IsPlayer() {
			// Self-targeting collapses to NotFound under ExcludeUserId;
			// check explicitly so we can return BuyReasonSelfTarget.
			if pId, _ := room.FindByName(mercName); pId == buyer.GetUserId() {
				buyer.SendText(messaging.CategoryError, "You can't buy from yourself.")
				return BuyResult{Reason: BuyReasonSelfTarget, Requested: quantity}
			}
			buyer.SendText(messaging.CategoryRefusal, "Visit a merchant to purchase objects or services.")
			return BuyResult{Reason: BuyReasonNoMerchant, Requested: quantity}
		} else {
			return BuyResult{Reason: BuyReasonNoMerchant, Requested: quantity}
		}
	}

	merchantPlayers := room.GetPlayers(rooms.FindMerchant)
	merchantMobs := room.GetMobs(rooms.FindMerchant)

	if len(merchantPlayers) == 0 && len(merchantMobs) == 0 {
		if buyer.IsPlayer() {
			buyer.SendText(messaging.CategoryRefusal, "Visit a merchant to purchase objects or services.")
		}
		return BuyResult{Reason: BuyReasonNoMerchant, Requested: quantity}
	}

	// Below the faces band you can't make out the goods (lighting plan 5b).
	// Checked only once a merchant is confirmed present, matching every other
	// check in this function that reads buyer.GetCharacter(): a nil-Mob
	// MobActor (the "no merchant here" test fixture) never reaches this line.
	if ShopSightRefusal(buyer.GetCharacter(), room) {
		if buyer.IsPlayer() {
			buyer.SendText(messaging.CategoryRefusal, ShopSightRefusalText)
		}
		return BuyResult{Reason: BuyReasonNoSight, Requested: quantity}
	}

	// tryMerchant wraps a per-merchant attempt closure to support retries up to quantity.
	// The closure-passed attempt callback returns the result of a single purchase.
	// If successful, tryMerchant loops up to quantity times, calling attempt()
	// for each additional unit purchase.
	// attempt receives isFirst so the callback can award bartering progression
	// exactly once per command rather than once per unit (Phase D Task 3).
	tryMerchant := func(attempt func(isFirst bool) BuyResult) (BuyResult, bool) {
		first := attempt(true)
		if !first.Success {
			return first, false
		}
		purchased := 1
		for purchased < quantity {
			next := attempt(false)
			if !next.Success {
				break
			}
			purchased++
		}
		if quantity > 1 && purchased < quantity {
			if buyer.IsPlayer() {
				buyer.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="yellow">Purchased %d of %d before running short.</ansi>`, purchased, quantity))
			}
		}
		return BuyResult{
			Success:   true,
			Purchased: purchased,
			Requested: quantity,
			SaleType:  first.SaleType,
		}, true
	}

	// Iterate merchants — players first, then mobs.
	for _, uid := range merchantPlayers {
		if targetUserId > 0 && uid != targetUserId {
			continue
		}
		shopUser := users.GetByUserId(uid)
		if shopUser == nil {
			continue
		}
		result, sold := tryMerchant(func(isFirst bool) BuyResult {
			r := tryPurchaseLegacy(buyer, itemRequest, nil, shopUser)
			if r.Success {
				postSuccessBookkeeping(buyer, nil, shopUser, isFirst)
			}
			return r
		})
		if sold {
			return result
		}
	}

	for _, miid := range merchantMobs {
		if targetMobInstanceId > 0 && miid != targetMobInstanceId {
			continue
		}
		shopMob := mobs.GetInstance(miid)
		if shopMob == nil {
			continue
		}

		shopInv := shops.GetShopInventory(shopMob.Zone, int(shopMob.MobId), shopMob.HomeRoomId)
		if shopInv != nil {
			result, sold := tryMerchant(func(isFirst bool) BuyResult {
				r := tryPurchaseFromInventory(buyer, itemRequest, shopMob, shopInv)
				if r.Success {
					postSuccessBookkeeping(buyer, shopMob, nil, isFirst)
				}
				return r
			})
			if sold {
				return result
			}
		} else {
			result, sold := tryMerchant(func(isFirst bool) BuyResult {
				shopMob.Character.Shop.Restock()
				r := tryPurchaseLegacy(buyer, itemRequest, shopMob, nil)
				if r.Success {
					postSuccessBookkeeping(buyer, shopMob, nil, isFirst)
				}
				return r
			})
			if sold {
				return result
			}
		}
	}

	return BuyResult{Reason: BuyReasonNoMatch, Requested: quantity}
}

// tryPurchaseLegacy attempts a single purchase against a merchant
// backed by the legacy Character.Shop. Returns the populated
// BuyResult on either success or failure; non-empty Reason on
// failure means the outer Buy loop should try the next merchant.
func tryPurchaseLegacy(buyer Actor, request string, shopMob *mobs.Mob, shopUser *users.UserRecord) BuyResult {
	var saleItems characters.Shop
	if shopMob != nil {
		saleItems = shopMob.Character.Shop.GetInstock()
	} else if shopUser != nil {
		saleItems = shopUser.Character.Shop.GetInstock()
	}

	cat := buildLegacyCatalog(saleItems)

	match, closeMatch := util.FindMatchIn(request, cat.allNames()...)
	if match == "" {
		match = closeMatch
	}
	if match == "" {
		if shopMob != nil {
			extraSay := ""
			if len(cat.itemNamesFancy) > 0 {
				randSelection := util.Rand(len(cat.itemNamesFancy))
				extraSay = fmt.Sprintf(` Any interest in this <ansi fg="itemname">%s</ansi>?`, cat.itemNamesFancy[randSelection])
			} else if len(cat.conditionNames) > 0 {
				randSelection := util.Rand(len(cat.conditionNames))
				extraSay = fmt.Sprintf(` Maybe you would enjoy this %s enchantment?`, cat.conditionNames[randSelection])
			}
			shopMob.Command(`say Sorry, I can't offer that right now.`+extraSay, 1)
		}
		return BuyResult{Reason: BuyReasonNoMatch}
	}

	ctx, reason, ok := validatePurchase(buyer, shopMob, shopUser, cat.nameToShopItem[match], cat.itemPrices, cat.conditionPrices)
	if !ok {
		return BuyResult{Reason: reason}
	}

	if ctx.matchedShopItem.ItemId > 0 {
		executePurchaseItem(buyer, shopMob, shopUser, ctx.matchedShopItem, ctx.price, ctx.tradeInString)
		return BuyResult{Success: true, Purchased: 1, SaleType: "item"}
	}
	if ctx.matchedShopItem.ConditionId > 0 {
		executePurchaseCondition(buyer, shopMob, shopUser, ctx.matchedShopItem, ctx.price, ctx.tradeInString)
		return BuyResult{Success: true, Purchased: 1, SaleType: "condition"}
	}

	// Merc/pet sale types are filtered out at catalog build time;
	// reaching this branch means malformed data.
	return BuyResult{Reason: BuyReasonNoMatch}
}

// tryPurchaseFromInventory attempts a single purchase against a
// ShopInventory-backed mob merchant. Condition/merc/pet purchases are
// NOT handled here — ShopInventory only carries items.
func tryPurchaseFromInventory(buyer Actor, request string, shopMob *mobs.Mob, shopInv *shops.ShopInventory) BuyResult {
	cfg := shops.PricingConfigFromBalance()

	// The shelf cap is enforced lazily (baubles slice D): a hold that ended
	// since the last trade can push the listed entries over it. The trim is
	// saved before returning unless a purchase below saves the shop anyway.
	now := shops.ShelfNow()
	shopSaved := false
	if shopInv.EnforceAffixedCap(int(configs.GetBalanceConfig().ShopAffixedStockCap), now) > 0 {
		defer func() {
			if !shopSaved {
				if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
					mudlog.Error("PURCHASE", "msg", "SaveShop failed", "error", err)
				}
			}
		}()
	}

	type invEntry struct {
		entry      *shops.StockEntry
		item       items.Item
		price      int
		affixedIdx int // index into shopInv.AffixedStock, or -1 for base-ItemId stock
	}

	var available []invEntry
	var itemNames []string
	var itemNamesFancy []string

	char := buyer.GetCharacter()
	// Balance numbers come from config.yaml, never a Go literal: the buy-side
	// bartering cap is Balance.BarterMaxDiscount (sell.go reads BarterMaxBonus
	// instead, see sellOneToMerchant).
	barterMaxDiscount := float64(configs.GetBalanceConfig().BarterMaxDiscount)

	for i := range shopInv.Stock {
		entry := &shopInv.Stock[i]
		if entry.Current <= 0 {
			continue
		}
		itm := items.New(entry.ItemId)
		if itm.ItemId == 0 {
			continue
		}
		spec := itm.GetSpec()
		restock := shops.PricingBaseline(entry, cfg)
		basePrice := shops.CalcSellPrice(spec.Value, entry.Current, restock, cfg)

		// Bartering discount, works symmetrically for both buyer types. A
		// dazzled buyer bargains worse (lighting plan 5b): barterDiscount
		// folds SightMult into the same BarterMaxDiscount-at-skill-50 cap.
		if discount := barterDiscount(char, buyer.GetRoom(), barterMaxDiscount); discount > 0 {
			basePrice = shops.ApplyBarterSellDiscount(basePrice, discount)
		}

		available = append(available, invEntry{
			entry:      entry,
			item:       itm,
			price:      basePrice,
			affixedIdx: -1,
		})
		itemNames = append(itemNames, spec.Name)
		itemNamesFancy = append(itemNamesFancy, itm.DisplayName())
	}

	// The secondhand shelf (Stage 3; baubles slice D): bought-back affixed
	// gear and baubles, in ListedIndexes order, the order list shows, each at
	// its stored relist price less any barter. A held entry (a bauble still
	// hot when shelved) is not offered, so no name, no match and no "Any
	// interest" line can reach it. affixedIdx is the raw AffixedStock index,
	// not the row's position among the listed ones.
	for _, idx := range shopInv.ListedIndexes(now) {
		e := &shopInv.AffixedStock[idx]
		name := e.Item.GetSpec().Name
		if e.Item.IsBauble() {
			// The buyer's own view, the one list showed them (owner ruling
			// 4): a finder-only bauble is Trinket to everyone but its finder,
			// so only its finder's own name holds its hidden words and nobody
			// else can confirm them by typing. A mob buyer (user 0) gets the
			// generic view.
			name = e.Item.NameFor(buyer.GetUserId())
		}
		price := e.Price
		if discount := barterDiscount(char, buyer.GetRoom(), barterMaxDiscount); discount > 0 {
			price = shops.ApplyBarterSellDiscount(price, discount)
		}
		available = append(available, invEntry{
			item:       e.Item,
			price:      price,
			affixedIdx: idx,
		})
		itemNames = append(itemNames, name)
		itemNamesFancy = append(itemNamesFancy, e.Item.DisplayName())
	}

	// Select by position, not by name (baubles slice D): two rows can share
	// a name (two shelved trinkets), and `buy 2.trinket` must take the second.
	// Counted in available order: stock first, then the shelf in list order.
	// A full match anywhere outranks a close match, as FindMatchIn rules.
	pick, closePick := util.FindMatchIndexIn(request, itemNames...)
	if pick < 0 {
		pick = closePick
	}
	if pick < 0 {
		if shopMob != nil {
			extraSay := ""
			if len(itemNamesFancy) > 0 {
				randSelection := util.Rand(len(itemNamesFancy))
				extraSay = fmt.Sprintf(` Any interest in this <ansi fg="itemname">%s</ansi>?`, itemNamesFancy[randSelection])
			}
			shopMob.Command(`say Sorry, I can't offer that right now.` + extraSay)
		}
		return BuyResult{Reason: BuyReasonNoMatch}
	}

	matched := &available[pick]

	// Encumbrance gate — same pre-side-effect check as the legacy
	// path, since ShopInventory bypasses validatePurchase.
	if char.GetCarriedWeight()+matched.item.GetSpec().Weight > char.CarryCapacity() {
		if buyer.IsPlayer() {
			buyer.SendText(messaging.CategoryError, "You can't carry any more.")
		}
		return BuyResult{Reason: BuyReasonOverburdened}
	}

	// Affixed resale purchase (Stage 3): hand the exact stored item with a fresh
	// UUID and remove the entry. Bypasses the base-ItemId destock path below
	// (matched.entry is nil for affixed rows).
	if matched.affixedIdx >= 0 {
		if char.Gold < matched.price {
			if shopMob != nil {
				shopMob.Command(`say You don't have enough gold for that.`)
			} else if buyer.IsPlayer() {
				buyer.SendText(messaging.CategoryError, `You don't have enough gold for that.`)
			}
			return BuyResult{Reason: BuyReasonInsufficientGold}
		}
		if matched.affixedIdx >= len(shopInv.AffixedStock) {
			return BuyResult{Reason: BuyReasonOutOfStock}
		}
		shelved := shopInv.AffixedStock[matched.affixedIdx] // the whole entry, for a rollback
		bought, ok := shopInv.RemoveAffixedStock(matched.affixedIdx)
		if !ok {
			return BuyResult{Reason: BuyReasonOutOfStock}
		}
		bought.UUID = items.NewItemUUID()
		if !char.StoreItem(bought) {
			// Roll back on carry failure: the same entry in the same place,
			// its relist price and listing time unchanged (a re-add would
			// stamp a new time at the barter-discounted price). Defensive:
			// the encumbrance gate above refuses first.
			shopInv.RestoreAffixedStock(matched.affixedIdx, shelved)
			if buyer.IsPlayer() {
				buyer.SendText(messaging.CategoryError, "You can't carry any more.")
			}
			return BuyResult{Reason: BuyReasonOverburdened}
		}
		shopInv.SalesCount++
		char.Gold -= matched.price
		shopInv.Gold += matched.price
		if buyer.IsPlayer() {
			events.AddToQueue(events.ItemOwnership{UserId: buyer.GetUserId(), Item: bought, Gained: true})
			events.AddToQueue(events.EquipmentChange{UserId: buyer.GetUserId(), GoldChange: -matched.price})
		}
		if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
			mudlog.Error("PURCHASE", "msg", "SaveShop failed", "error", err)
		}
		shopSaved = true
		if bought.IsBauble() {
			// Back in a pack: its record returns to its unsold status
			// (baubles slice D, ruling 2).
			baubles.MarkBought(bought.Bauble, buyer.GetUserId())
		}
		buyer.SendText(messaging.CategoryLoot, fmt.Sprintf(`You buy the <ansi fg="itemname">%s</ansi>.`, bought.DisplayNameFor(buyer.GetUserId())))
		if room := buyer.GetRoom(); room != nil {
			room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(`<ansi fg="username">%s</ansi> buys the <ansi fg="itemname">%s</ansi>.`, buyer.GetName(), bought.DisplayName()), buyer.GetUserId())
		}
		return BuyResult{Success: true, Purchased: 1, SaleType: "item"}
	}

	if matched.entry.Current <= 0 {
		if shopMob != nil {
			shopMob.Command(`say I don't have that for sale right now.`)
		}
		return BuyResult{Reason: BuyReasonOutOfStock}
	}

	if char.Gold < matched.price {
		if shopMob != nil {
			shopMob.Command(`say You don't have enough gold for that.`)
		} else if buyer.IsPlayer() {
			buyer.SendText(messaging.CategoryError, `You don't have enough gold for that.`)
		}
		return BuyResult{Reason: BuyReasonInsufficientGold}
	}

	if shopInv.RemoveStockAtRound(matched.entry.ItemId, 1, util.GetRoundCount()) == 0 {
		if shopMob != nil {
			shopMob.Command(`say I don't have that item right now.`)
		}
		return BuyResult{Reason: BuyReasonOutOfStock}
	}
	shopInv.SalesCount++

	if buyer.IsPlayer() {
		events.AddToQueue(events.EquipmentChange{
			UserId:     buyer.GetUserId(),
			GoldChange: -matched.price,
		})
	}
	char.Gold -= matched.price
	shopInv.Gold += matched.price

	if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
		mudlog.Error("PURCHASE", "msg", "SaveShop failed", "error", err)
	}
	shopSaved = true

	tradeInString := fmt.Sprintf(`<ansi fg="gold">%s gold</ansi>`, util.FormatNumber(matched.price))
	if matched.price == 0 {
		tradeInString = "nothing"
	}

	executePurchaseItem(buyer, shopMob, nil,
		characters.ShopItem{ItemId: matched.entry.ItemId},
		matched.price, tradeInString)

	return BuyResult{Success: true, Purchased: 1, SaleType: "item"}
}

// executePurchaseItem lands a purchased item in the buyer's
// inventory and emits the buyer + room messages.
func executePurchaseItem(buyer Actor, shopMob *mobs.Mob, shopUser *users.UserRecord, matchedShopItem characters.ShopItem, price int, tradeInString string) {
	newItm := items.New(matchedShopItem.ItemId)

	if buyer.IsPlayer() {
		userId := buyer.GetUserId()
		if u := users.GetByUserId(userId); u != nil {
			u.PlaySound(`purchase`, `other`)
		}
		events.AddToQueue(events.ItemOwnership{
			UserId: userId,
			Item:   newItm,
			Gained: true,
		})
	} else {
		events.AddToQueue(events.ItemOwnership{
			MobInstanceId: buyer.GetMobInstanceId(),
			Item:          newItm,
			Gained:        true,
		})
	}

	buyerName := buyer.GetName()

	if shopMob != nil {
		if buyer.IsPlayer() {
			if u := users.GetByUserId(buyer.GetUserId()); u != nil {
				u.EventLog.Add(`shop`, fmt.Sprintf(`Purchased a <ansi fg="itemname">%s</ansi> from <ansi fg="mobname">%s</ansi> for %s`, newItm.DisplayName(), shopMob.Character.Name, tradeInString))
			}
		}
		buyer.SendText(messaging.CategoryLoot, fmt.Sprintf(`You purchase the <ansi fg="itemname">%s</ansi> from <ansi fg="mobname">%s</ansi> for %s.`, newItm.DisplayName(), shopMob.Character.Name, tradeInString))
		if room := buyer.GetRoom(); room != nil {
			room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(`<ansi fg="username">%s</ansi> purchases the <ansi fg="itemname">%s</ansi> from <ansi fg="mobname">%s</ansi>.`, buyerName, newItm.DisplayName(), shopMob.Character.Name), buyer.GetUserId())
		}
	} else if shopUser != nil {
		if buyer.IsPlayer() {
			if u := users.GetByUserId(buyer.GetUserId()); u != nil {
				u.EventLog.Add(`shop`, fmt.Sprintf(`Purchased a <ansi fg="itemname">%s</ansi> from <ansi fg="username">%s</ansi> for %s.`, newItm.DisplayName(), shopUser.Character.Name, tradeInString))
			}
		}
		buyer.SendText(messaging.CategoryLoot, fmt.Sprintf(`You purchase the <ansi fg="itemname">%s</ansi> from <ansi fg="username">%s</ansi> for %s.`, newItm.DisplayName(), shopUser.Character.Name, tradeInString))
		shopUser.SendText(messaging.CategoryLoot, fmt.Sprintf(`<ansi fg="username">%s</ansi> purchased the <ansi fg="itemname">%s</ansi> you were selling for %s.`, buyerName, newItm.DisplayName(), tradeInString))
		if room := buyer.GetRoom(); room != nil {
			room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(`<ansi fg="username">%s</ansi> purchases the <ansi fg="itemname">%s</ansi> from <ansi fg="username">%s</ansi>.`, buyerName, newItm.DisplayName(), shopUser.Character.Name), buyer.GetUserId())
		}
	}

	buyer.GetCharacter().StoreItem(newItm)
}

// executePurchaseCondition applies the bought condition to the buyer and
// emits the merchant emote follow-up.
func executePurchaseCondition(buyer Actor, shopMob *mobs.Mob, shopUser *users.UserRecord, matchedShopItem characters.ShopItem, price int, tradeInString string) {
	conditionSpec := conditions.GetConditionSpec(matchedShopItem.ConditionId)
	buyerName := buyer.GetName()

	if shopMob != nil {
		if buyer.IsPlayer() {
			if u := users.GetByUserId(buyer.GetUserId()); u != nil {
				u.EventLog.Add(`shop`, fmt.Sprintf(`Purchased a <ansi fg="condition">%s</ansi> enchantment from <ansi fg="mobname">%s</ansi> for %s`, conditionSpec.Name, shopMob.Character.Name, tradeInString))
			}
		}
		buyer.SendText(messaging.CategoryLoot, fmt.Sprintf(`You pay %s to <ansi fg="mobname">%s</ansi>.`, tradeInString, shopMob.Character.Name))
		if room := buyer.GetRoom(); room != nil {
			room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(`<ansi fg="username">%s</ansi> pays %s to <ansi fg="mobname">%s</ansi>.`, buyerName, tradeInString, shopMob.Character.Name), buyer.GetUserId())
		}
		shopMob.Command(`emote mutters a soft incantation.`, 1)
	} else if shopUser != nil {
		if buyer.IsPlayer() {
			if u := users.GetByUserId(buyer.GetUserId()); u != nil {
				u.EventLog.Add(`shop`, fmt.Sprintf(`Purchased a <ansi fg="condition">%s</ansi> enchantment from  <ansi fg="username">%s</ansi> for %s`, conditionSpec.Name, shopUser.Character.Name, tradeInString))
			}
		}
		buyer.SendText(messaging.CategoryLoot, fmt.Sprintf(`You pay %s to <ansi fg="username">%s</ansi>.`, tradeInString, shopUser.Character.Name))
		shopUser.SendText(messaging.CategoryLoot, fmt.Sprintf(`<ansi fg="username">%s</ansi> pays you %s for an enchantment.`, buyerName, tradeInString))
		if room := buyer.GetRoom(); room != nil {
			room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(`<ansi fg="username">%s</ansi> pays to <ansi fg="username">%s</ansi> for an enchantment.`, buyerName, shopUser.Character.Name), buyer.GetUserId())
		}
		// player-merchant doesn't emote (matches existing behavior).
	}

	buyer.AddCondition(matchedShopItem.ConditionId, "shop")

	if shopMob != nil {
		shopMob.Command(`say I've done what I can.`, 1)
	}
}

// postSuccessBookkeeping runs after any successful purchase: skill
// progression for the buyer, charisma stat-use on the merchant mob,
// and quest-engine notification gated by buyer.IsPlayer().
// awardProgression is true only for the FIRST unit of a multi-buy. Bartering
// used to award per unit with no cooldown, so `buy 200 x` fired 200 progression
// rolls from one command, which made bartering unbounded in time -- no
// uses/hour could be fitted to it (U10b-0 Phase D Task 3). The merchant-side
// charisma roll and the quest notification stay PER UNIT: quest steps count
// items, so collapsing them would break "buy N of X" objectives.
func postSuccessBookkeeping(buyer Actor, shopMob *mobs.Mob, shopUser *users.UserRecord,
	awardProgression bool) {
	// U10b-1 Task 18c: won is unconditionally true. This runs inside
	// postSuccessBookkeeping -- every refusal path (no stock, cannot afford,
	// carry capacity) returns before it -- so a completed trade is a success by
	// construction and there is no losing branch to pay a fraction on. Haggling
	// is not a contest in this economy; it is a price lookup.
	//
	// The awardProgression gate is UNCHANGED and is not the firing rule: it is
	// true only for the FIRST unit of a multi-buy, so `buy 200 x` fires one
	// award rather than 200. See the doc comment above.
	if awardProgression {
		buyer.AwardResolved(true, buyer.GetCharacter().CandidateFor("bartering"))
	}

	if shopMob != nil {
		shopMob.Character.OnStatUse("charisma", 0)
	}

	if buyer.IsPlayer() {
		userId := buyer.GetUserId()
		if u := users.GetByUserId(userId); u != nil {
			room := buyer.GetRoom()
			bridge := questengine.NewGameBridge(u, room.RoomId)
			questengine.GetEngine().Notify("command", questengine.EventDetails{
				UserId:  userId,
				RoomId:  room.RoomId,
				Command: "buy",
			}, bridge, bridge)
		}
	}
}
