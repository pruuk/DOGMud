package actions

import (
	"fmt"
	"math"

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
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/shops"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
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
				seller.SendText(messaging.CategoryRefusal, ShopSightRefusalText)
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
	for _, mobId := range room.GetMobs(rooms.FindMerchant) {
		mob := mobs.GetInstance(mobId)
		if mob == nil {
			continue
		}
		shopInv := shops.GetShopInventory(mob.Zone, int(mob.MobId), mob.HomeRoomId)
		// Interest only: no seller (no barter bonus, which cannot turn a
		// refusal into a price) and no purse check (a broke merchant still
		// takes the sale and says it cannot pay, as it always has).
		if QuoteSale(nil, room, mob, shopInv, probe, false).Price > 0 {
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
			refusal.NotCarrying(seller, itemName)
		}
		return SellResult{Reason: SellStopNoItem}
	}
	if probe.GetSpec().QuestToken != "" {
		if seller.IsPlayer() {
			seller.SendText(messaging.CategoryRefusal, "Quest items cannot be sold!")
		}
		return SellResult{Reason: SellStopRejected}
	}
	// No WILLING merchant still routes through sellOneToMerchant when a
	// merchant is present (SaleMerchant), so the merchant says why it won't
	// buy and we return SellStopRejected / SellStopMerchantBroke rather than
	// the misleading SellStopNoMerchant ("There's no merchant here.").
	mob, shopInv := SaleMerchant(room, probe, seller.IsPlayer())
	if mob == nil {
		return SellResult{Reason: SellStopNoMerchant}
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
			if out.Sold > 0 && (next.IsBauble() || baubleSold) {
				// SaleMerchant falls back to the first merchant present,
				// who says why nobody will buy it.
				if m, inv := SaleMerchant(room, next, seller.IsPlayer()); m != nil {
					mob, shopInv = m, inv
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
		value, res := sellOneToMerchant(seller, itm.Name(), room, mob, shopInv, !soldAny)
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
	// The one quote offer and appraise also say (QuoteSale): price, the
	// merchant's refusal line, and for a player the purse check.
	quote := QuoteSale(char, room, mob, shopInv, item, seller.IsPlayer())
	if quote.Tell != "" {
		if seller.IsPlayer() {
			seller.SendText(messaging.CategoryRefusal, quote.Tell)
		}
		return 0, SellStopRejected
	}
	if quote.Price <= 0 && quote.Say == "" {
		return 0, SellStopRejected // no spec: nothing to deal over
	}

	char.CancelConditionsWithFlag(conditions.Hidden)
	if quote.Price <= 0 {
		merchantSay(room, mob, quote.Say)
		if quote.Broke {
			return 0, SellStopMerchantBroke
		}
		return 0, SellStopRejected
	}
	sellValue := quote.Price
	buyReason := quote.buyReason

	// Baubles (docs/baubles): catalog-priced; a player's average or rare one
	// goes on a living shop's shelf, the rest leave the world. See
	// sell_bauble.go.
	if item.IsBauble() {
		return sellBaubleToMerchant(seller, item, sellValue, mob, shopInv, awardProgression)
	}

	// Only players draw down the merchant's gold (QuoteSale checked they
	// can pay). Mob sales mint the seller's payout.
	if seller.IsPlayer() {
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

	notifySaleToQuests(seller, item, mob)
	return sellValue, SellStopSoldAll
}

// notifySaleToQuests tells the quest engine a player's sale completed, as
// postSuccessBookkeeping does for a purchase, but with the item sold and the
// shop mob that bought it, so a trigger can key on either (R22, #375: selling
// Smith Rusk the crafted dagger turns in First Heat). It runs only on the
// success paths, once per unit, so a refused sale never reaches it. Mob sellers
// have no quest log and are skipped.
func notifySaleToQuests(seller Actor, item items.Item, mob *mobs.Mob) {
	if !seller.IsPlayer() {
		return
	}
	userId := seller.GetUserId()
	u := users.GetByUserId(userId)
	room := seller.GetRoom()
	if u == nil || room == nil {
		return
	}
	questengine.NotifyCommand(u, room.RoomId, "sell", questengine.TargetItem(item.ItemId).WithMob(mob))
}

// sellFindItemInChar searches backpack → potions → components for a match.
func sellFindItemInChar(char *characters.Character, name string) (items.Item, bool) {
	item, found := char.FindInBackpack(name)
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
