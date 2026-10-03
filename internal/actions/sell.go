package actions

import (
	"fmt"
	"math"
	"regexp"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// UnlimitedSell is the Quantity sentinel meaning "sell every match".
const UnlimitedSell = math.MaxInt

type SellOptions struct {
	ItemName        string // ignored when SellAllSellable
	Quantity        int    // 1, N, or UnlimitedSell
	SellAllSellable bool   // mob inventory-sweep mode (every sellable item)
	MerchantName    string // optional target merchant name; "" = first willing
}

type SellStopReason int

const (
	SellStopSoldAll       SellStopReason = iota // ran out of matching items (normal)
	SellStopNoItem                              // seller never had the item
	SellStopNoMerchant                          // no willing merchant in room
	SellStopMerchantBroke                       // merchant ran out of gold (player path only)
	SellStopRejected                            // merchant declined the item type
	SellStopNoSight                             // a merchant is here but the seller can't see well enough to deal
)

type SellResult struct {
	Sold         int
	TotalGold    int
	Reason       SellStopReason
	LastItemName string
	// Mixed is set when the items sold were not all the same thing, so the
	// caller must not pluralise LastItemName. Every bauble has its own name:
	// `sell all bauble` sells three different objects, not three "Tarnished
	// Copper Buttons".
	Mixed bool
}

// Sell is the shared seller entry point for players and mobs. The seller is
// abstracted via Actor; the merchant is a shopkeeper mob resolved from the
// seller's room. Player sells draw down shop gold; mob sells credit the seller
// but leave shop gold intact (see internal/shops/context.md "two sell models").
//
// NOTE: forager.SellToVendor is a different path — a free supply handoff, not a
// sale. See internal/forager/vendor_sell.go.
func Sell(seller Actor, opts SellOptions) SellResult {
	room := seller.GetRoom()
	if room == nil {
		return SellResult{Reason: SellStopNoMerchant}
	}

	// Below the faces band you can't make out the goods (lighting plan 5b).
	// Gated on a merchant actually being in the room, same as
	// ShopClosedForSleep beside it: "there's no merchant here" still wins
	// over a sight refusal when there is truly nobody to deal with.
	if len(room.GetPlayers(rooms.FindMerchant)) > 0 || len(room.GetMobs(rooms.FindMerchant)) > 0 {
		if ShopSightRefusal(seller.GetCharacter(), room) {
			if seller.IsPlayer() {
				seller.SendText(messaging.CategorySystem, ShopSightRefusalText)
			}
			return SellResult{Reason: SellStopNoSight}
		}
	}

	if opts.SellAllSellable {
		return sellSweep(seller, room)
	}
	if opts.Quantity < 1 {
		opts.Quantity = 1
	}
	return sellNamed(seller, room, opts.ItemName, opts.Quantity)
}

// merchantSay makes the merchant speak a line to the room immediately.
//
// Merchant refusals ("I'm not interested in that.", "I can't afford that
// right now.", etc.) were previously delivered via mob.Command("say ..."),
// which enqueues the line on the mob's ASYNC command pipeline (events.Input,
// gated on the mob's next turn). On busy shopkeepers — scheduled townsfolk and
// autonomous crafters like Kerra and Voss — that pipeline can defer the line
// for many turns, long enough that the seller never associates it with their
// sell attempt and the sale appears to fail silently. Every other sell outcome
// (success, no-item, no-merchant, quest-item) reports synchronously, so the
// refusal path was the lone async outlier. Speak synchronously instead — the
// same broadcast mobcommands.Say performs — so the seller always gets
// immediate, reliable feedback.
func merchantSay(room *rooms.Room, mob *mobs.Mob, line string) {
	if mob == nil || room == nil {
		return
	}
	// Say sends the room line itself (sight gates slice 5b).
	Say(&MobActor{Mob: mob, Room: room}, line)
}

// affixedSellPrice is the fixed-spread price a shop pays for an affix-scaled
// instance item: its (Stage-1 stamped) value times the buy/sell spread. It
// deliberately bypasses the scarcity curve and the legacy 25% cap — unique
// affixed gear is priced on value, not commodity stock levels.
func affixedSellPrice(item items.Item, cfg shops.PricingConfig) int {
	price := int(math.Ceil(float64(item.GetSpec().Value) * cfg.BuyRatio))
	if price < 1 {
		price = 1
	}
	return price
}

// resolveMerchant finds the first merchant in the room willing to buy probe,
// returning the merchant mob and its living-economy ShopInventory (nil for
// legacy-shop merchants). A bauble goes to the best offer the merchant can
// pay instead (bestBaubleMerchant); playerSale says whether the seller is a
// player, the only sellers a merchant's gold constrains.
func resolveMerchant(room *rooms.Room, probe items.Item, playerSale bool) (*mobs.Mob, *shops.ShopInventory) {
	if probe.IsBauble() {
		return bestBaubleMerchant(room, probe, playerSale)
	}
	if probe.IsStolen() {
		// Stolen goods go to a fence when there is one (it pays its cut,
		// hot or cold); without one, only where they are not hot.
		if mob, inv := fenceInRoom(room, probe); mob != nil {
			return mob, inv
		}
		if stolenGoodsHotHere(probe, room) {
			return nil, nil
		}
	}
	for _, mobId := range room.GetMobs(rooms.FindMerchant) {
		mob := mobs.GetInstance(mobId)
		if mob == nil {
			continue
		}
		shopInv := shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId)
		var probeValue int
		if probe.Affixed {
			probeValue = affixedSellPrice(probe, shops.PricingConfigFromBalance())
		} else if shopInv != nil {
			cfg := shops.PricingConfigFromBalance()
			wornItems := mob.Character.Equipment.GetAllItemsWithEmptySlots()
			offer := shops.EvaluateBuyRules(probe, shopInv, mob.CrafterSkill, mob.BuysGeneral, cfg, wornItems)
			probeValue = offer.Price
		} else {
			probeValue = mob.GetSellPrice(probe)
		}
		if probeValue > 0 {
			return mob, shopInv
		}
	}
	return nil, nil
}

// firstMerchantInRoom returns the first merchant present in the room and its
// ShopInventory (nil for legacy-shop merchants), regardless of whether it will
// buy any particular item. Returns (nil, nil) only when no merchant is present
// at all. Used by the named-sell path to distinguish "no merchant here" from
// "a merchant is here but won't buy this", so the latter routes through
// sellOneToMerchant for the proper spoken refusal instead of the misleading
// "There's no merchant here." message.
func firstMerchantInRoom(room *rooms.Room) (*mobs.Mob, *shops.ShopInventory) {
	for _, mobId := range room.GetMobs(rooms.FindMerchant) {
		mob := mobs.GetInstance(mobId)
		if mob == nil {
			continue
		}
		shopInv := shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId)
		return mob, shopInv
	}
	return nil, nil
}

func sellNamed(seller Actor, room *rooms.Room, itemName string, quantity int) SellResult {
	char := seller.GetCharacter()
	probe, found := sellFindItemInChar(char, itemName)
	if !found {
		if seller.IsPlayer() {
			seller.SendText(messaging.CategorySystem, "You don't have that item.")
		}
		return SellResult{Reason: SellStopNoItem}
	}
	if probe.GetSpec().QuestToken != "" {
		if seller.IsPlayer() {
			seller.SendText(messaging.CategorySystem, "Quest items cannot be sold!")
		}
		return SellResult{Reason: SellStopRejected}
	}
	mob, shopInv := resolveMerchant(room, probe, seller.IsPlayer())
	if mob == nil {
		// No WILLING merchant. Distinguish "no merchant present at all" from
		// "a merchant is here but won't buy this item." For the latter, route
		// through sellOneToMerchant so the merchant gives the right spoken
		// refusal ("I'm not interested in that." / "I can't afford that right
		// now.") and we return SellStopRejected / SellStopMerchantBroke —
		// rather than the misleading SellStopNoMerchant ("There's no merchant
		// here.").
		mob, shopInv = firstMerchantInRoom(room)
		if mob == nil {
			return SellResult{Reason: SellStopNoMerchant}
		}
	}
	var out SellResult
	out.Reason = SellStopSoldAll
	baubleSold := false // the buyer may since have changed from the probe's
	for out.Sold < quantity {
		// The item this iteration sells is the first match left, which is not
		// always the probe: it differs for baubles, which share a keyword but
		// each have their own name.
		soldName := probe.GetSpec().Name
		soldBauble := probe.IsBauble()
		if next, ok := sellFindItemInChar(char, itemName); ok {
			soldBauble = next.IsBauble()
			soldName = next.GetSpec().Name
			// Each bauble goes to the best offer for it, which is not
			// always the merchant who took the one before (a fence pays
			// more for stolen goods only; a merchant runs out of gold).
			// A real item after a bauble gets its own merchant too, not
			// the bauble's buyer; until then real items keep the probe's.
			// Clean copies sell before stolen ones (sellFindItemInChar), so
			// once the next is stolen and the buyer is honest, the stolen
			// ones go to a fence if one is here (resolveMerchant), else an
			// honest merchant takes them where they are not hot and
			// refuses them where they are.
			needsFence := next.IsStolen() && !IsFence(mob)
			if out.Sold > 0 && (next.IsBauble() || baubleSold || needsFence) {
				if m, inv := resolveMerchant(room, next, seller.IsPlayer()); m != nil {
					mob, shopInv = m, inv
				} else if m, inv := firstMerchantInRoom(room); m != nil {
					mob, shopInv = m, inv // says why nobody will buy it
				}
			}
		}
		value, res := sellOneToMerchant(seller, itemName, room, mob, shopInv, out.Sold == 0)
		if res != SellStopSoldAll {
			// Running out of matching items mid-loop is a NORMAL completion
			// (the "sold all I had" case) — only surface it as SellStopNoItem
			// when nothing was sold at all (the seller never had the item).
			// Merchant-side stops (broke / rejected) always surface.
			if res == SellStopNoItem && out.Sold > 0 {
				out.Reason = SellStopSoldAll
			} else {
				out.Reason = res
			}
			break
		}
		if out.Sold > 0 && soldName != out.LastItemName {
			out.Mixed = true
		}
		out.Sold++
		out.TotalGold += value
		out.LastItemName = soldName
		baubleSold = baubleSold || soldBauble
	}
	return out
}

func sellSweep(seller Actor, room *rooms.Room) SellResult {
	char := seller.GetCharacter()
	var out SellResult
	out.Reason = SellStopSoldAll
	snapshot := append([]items.Item{}, char.Items...)
	soldAny := false
	for _, itm := range snapshot {
		spec := itm.GetSpec()
		if spec.ItemId < 1 || spec.QuestToken != "" || spec.Value <= 0 || spec.IsComponent {
			continue
		}
		mob, shopInv := resolveMerchant(room, itm, seller.IsPlayer())
		if mob == nil {
			continue
		}
		// Sell exactly this item: by name, the clean-copy preference could
		// hand a fence the clean twin of a stolen item it was resolved for.
		name := itm.Name()
		if !itm.UUID.IsNil() {
			name = characters.ItemHandleSigil + itm.UUID.String()
		}
		value, res := sellOneToMerchant(seller, name, room, mob, shopInv, !soldAny)
		if res == SellStopSoldAll {
			if soldAny && spec.Name != out.LastItemName {
				out.Mixed = true
			}
			soldAny = true
			out.Sold++
			out.TotalGold += value
			out.LastItemName = spec.Name
		}
	}
	if !soldAny {
		out.Reason = SellStopRejected
	}
	return out
}

// sellOneToMerchant sells a single matching item from seller to the given
// merchant. Mirrors the player trySellOne, with two changes:
//   - seller-side access via Actor (GetCharacter / Gold / RemoveItem).
//   - shop-gold drain + merchant-broke check apply only to player sellers
//     (decision 2: NPC selling never bankrupts a shop).
//
// awardProgression is true only for the FIRST sale of a command. Bartering used
// to award per unit with no cooldown, so `sell all` on a 200-item stack fired
// 200 progression rolls from one command, which made bartering unbounded in
// time -- no uses/hour could be fitted to it (U10b-0 Phase D Task 3).
func sellOneToMerchant(seller Actor, itemName string, room *rooms.Room,
	mob *mobs.Mob, shopInv *shops.ShopInventory,
	awardProgression bool) (soldValue int, res SellStopReason) {

	char := seller.GetCharacter()
	item, found := sellFindItemInChar(char, itemName)
	if !found {
		return 0, SellStopNoItem
	}
	itemSpec := item.GetSpec()
	if itemSpec.ItemId < 1 {
		return 0, SellStopRejected
	}
	if itemSpec.QuestToken != "" {
		if seller.IsPlayer() {
			seller.SendText(messaging.CategorySystem, "Quest items cannot be sold!")
		}
		return 0, SellStopRejected
	}
	// Housing deeds, vouchers and keys: what was paid for them is gone for
	// good, and no merchant gives any of it back (items.IsNeverBought).
	if items.IsNeverBought(itemSpec.ItemId) {
		merchantSay(room, mob, "That's lodging-house paper. It's no good to me, and I'll not give you a coin for it.")
		return 0, SellStopRejected
	}

	char.CancelConditionsWithFlag(conditions.Hidden)
	// Baubles (docs/baubles): catalog-priced; a player's average or rare one
	// goes on a living shop's shelf, the rest leave the world. See
	// sell_bauble.go.
	if item.IsBauble() {
		return sellBaubleToMerchant(seller, item, room, mob, shopInv, awardProgression)
	}
	// Stolen goods (a merchant chest's, internal/merchantchests): a fence
	// buys them at its cut; an honest merchant refuses them while they are
	// hot here, and otherwise buys them as ordinary goods below. See
	// sell_stolen.go.
	if item.IsStolen() {
		if IsFence(mob) {
			return sellStolenToFence(seller, item, room, mob, shopInv, awardProgression)
		}
		if stolenGoodsHotHere(item, room) {
			merchantSay(room, mob, StolenGoodsRefusal(mob, item))
			return 0, SellStopRejected
		}
	}
	// Affixed instance loot is sellable despite carrying a per-instance Spec;
	// every other custom-spec item (enchanted / blob / uses) stays blocked.
	if item.IsSpecial() && !item.Affixed {
		merchantSay(room, mob, "I'm afraid I don't buy those.")
		return 0, SellStopRejected
	}

	var sellValue int
	var buyReason string
	if item.Affixed {
		// Unique affix-scaled loot: fixed spread off its stamped value, bypassing
		// scarcity pricing and the legacy 25% cap.
		sellValue = affixedSellPrice(item, shops.PricingConfigFromBalance())
	} else if shopInv != nil {
		cfg := shops.PricingConfigFromBalance()
		wornItems := mob.Character.Equipment.GetAllItemsWithEmptySlots()
		offer := shops.EvaluateBuyRules(item, shopInv, mob.CrafterSkill, mob.BuysGeneral, cfg, wornItems)
		sellValue = offer.Price
		buyReason = offer.Reason
		if sellValue > 0 {
			// A dazzled seller bargains worse (lighting plan 5b): barterDiscount
			// folds SightMult into the same BarterMaxBonus-at-skill-50 cap.
			// Balance numbers come from config.yaml, never a Go literal: the
			// sell-side cap is Balance.BarterMaxBonus (buy.go reads
			// BarterMaxDiscount instead, see tryPurchaseFromInventory).
			barterMaxBonus := float64(configs.GetBalanceConfig().BarterMaxBonus)
			if bonus := barterDiscount(char, room, barterMaxBonus); bonus > 0 {
				sellValue = shops.ApplyBarterBuyBonus(sellValue, bonus)
			}
		}
	} else {
		sellValue = mob.GetSellPrice(item)
	}

	if sellValue <= 0 {
		merchantSay(room, mob, "I'm not interested in that.")
		return 0, SellStopRejected
	}

	// Gold-model gate: only players are constrained by — and draw down —
	// the merchant's gold. Mob sales mint the seller's payout.
	if seller.IsPlayer() {
		merchantGold := mob.Character.Gold
		if shopInv != nil {
			merchantGold = shopInv.Gold
		}
		if merchantGold < sellValue {
			merchantSay(room, mob, "I can't afford that right now.")
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

	// Stock update (merchant side). Living-economy shops store the exact affixed
	// item for resale; legacy shops (shopInv == nil) melt it.
	if item.Affixed {
		if shopInv != nil {
			c := int(configs.GetBalanceConfig().ShopAffixedStockCap)
			now := shops.ShelfNow()
			shopInv.AddAffixedStock(item, item.GetSpec().Value, c, baubles.ShelfHoldUntil(item, now), now)
			shopInv.BuysCount++
			if err := shops.SaveShop(shopInv.Zone, shopInv.MobId, shopInv.RoomId); err != nil {
				mudlog.Error("SELL", "msg", "SaveShop failed", "error", err)
			}
		}
	} else if shopInv != nil {
		shopInv.BuysCount++
		if buyReason == "gear_upgrade" {
			newItem := items.New(item.ItemId)
			if newItem.ItemId > 0 {
				returnedItems, wore, wearFailure := mob.Character.Wear(newItem)
				if wore {
					for _, old := range returnedItems {
						if old.ItemId > 0 {
							shopInv.AddStockAtRound(old.ItemId, 1, util.GetRoundCount())
						}
					}
					room.SendTextVisual(messaging.CategoryLoot,
						fmt.Sprintf(`<ansi fg="mobname">%s</ansi> examines the <ansi fg="itemname">%s</ansi> and puts it on.`, mob.Character.Name, newItem.DisplayName()),
						seller.GetUserId(),
					)
				} else {
					// The gear-upgrade path calls Wear directly, so it is the one
					// place a refusal (the U7b reservation ceiling among them)
					// would otherwise vanish: the merchant shelves the item and
					// the seller sees nothing to explain why the upgrade did not
					// take.
					if wearFailure != "" {
						room.SendTextVisual(messaging.CategoryLoot,
							fmt.Sprintf(`<ansi fg="mobname">%s</ansi> considers the <ansi fg="itemname">%s</ansi>, then shelves it instead.`,
								mob.Character.Name, newItem.DisplayName()))
					}
					shopInv.AddStockAtRound(item.ItemId, 1, util.GetRoundCount())
				}
			} else {
				shopInv.AddStockAtRound(item.ItemId, 1, util.GetRoundCount())
			}
		} else {
			shopInv.AddStockAtRound(item.ItemId, 1, util.GetRoundCount())
		}
		if err := shops.SaveShop(mob.Zone, int(mob.MobId), mob.HomeRoomId); err != nil {
			mudlog.Error("SELL", "msg", "SaveShop failed", "error", err)
		}
	} else {
		mob.Character.Shop.StockItem(item.ItemId)
	}

	// Progression. FIRST sale of the command only -- see the doc comment.
	// U10b-1 Task 18c: won unconditionally true -- see the identical note in
	// actions/buy.go. A completed sale is a success by construction; the
	// refusal paths return earlier.
	//
	// ⚠️ The shop mob's charisma roll below is NOT a stray stat roll beside
	// this award: it belongs to a DIFFERENT character (the merchant), and it is
	// the merchant's only progression from trading. It is not the
	// emitAttackerStatGain pattern Task 22 deletes.
	if awardProgression {
		saleProgression(seller, mob)
	}

	return sellValue, SellStopSoldAll
}

// explicitItemPick matches a name that already says which copy it means:
// an ordinal (`2.ingot`, `ingot#2`) or an item handle (`@<uuid>`).
var explicitItemPick = regexp.MustCompile(`^\s*(\d+\.|` + regexp.QuoteMeta(characters.ItemHandleSigil) + `)|#\d+\s*$`)

// findCleanIn matches name among the pool's items that are not stolen.
func findCleanIn(name string, pool []items.Item) (items.Item, bool) {
	clean := make([]items.Item, 0, len(pool))
	for _, it := range pool {
		if !it.IsStolen() {
			clean = append(clean, it)
		}
	}
	close, full := items.FindMatchIn(name, clean...)
	if full.ItemId != 0 {
		return full, true
	}
	if close.ItemId != 0 {
		return close, true
	}
	return items.Item{}, false
}

// SellFindItemInChar searches backpack → potions → components for a match.
// In the backpack a clean copy is preferred over a stolen one, so `sell
// ingot` sells your own iron before the stolen bar that happens to sit
// first. A name that picks its copy explicitly (explicitItemPick) is taken
// at its word: `sell 2.ingot` is the second ingot in the pack, as ever.
// offer resolves through this too, so it quotes the copy sell would sell.
func SellFindItemInChar(char *characters.Character, name string) (items.Item, bool) {
	return sellFindItemInChar(char, name)
}

func sellFindItemInChar(char *characters.Character, name string) (items.Item, bool) {
	var item items.Item
	found := false
	if !explicitItemPick.MatchString(name) {
		// A clean copy anywhere the seller carries goods, before any stolen
		// one: the backpack, then the bandolier and the component bag.
		item, found = char.FindInBackpackWhere(name, func(i items.Item) bool { return !i.IsStolen() })
		if !found {
			item, found = findCleanIn(name, char.PotionItems)
		}
		if !found {
			item, found = findCleanIn(name, char.ComponentItems)
		}
	}
	if !found {
		item, found = char.FindInBackpack(name)
	}
	if !found {
		item, found = char.FindInPotions(name)
	}
	if !found {
		item, found = char.FindInComponents(name)
	}
	return item, found
}

// saleProgression is a completed sale's progression: the seller's bartering
// award, and the merchant's charisma roll (a different character's, and the
// merchant's only progression from trading; allowlisted for this file in
// progression/seam_guard_test.go). Shared by every item sold and a bauble
// sold (sell_bauble.go), so the one allowlisted site covers both.
func saleProgression(seller Actor, mob *mobs.Mob) {
	seller.AwardResolved(true, seller.GetCharacter().CandidateFor(string(skills.Bartering)))
	mob.Character.OnStatUse("charisma", 0)
}
