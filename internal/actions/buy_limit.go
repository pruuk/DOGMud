package actions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The per-character limit on buying Bloom Wafers (owner ruling 2026-11-04). A
// real drug market is accepted, so a wafer may be given and auctioned. What is
// limited is how often one character can buy at the source: at most
// Balance.BloomDealerWafersPerDay within a rolling Balance.BloomDealerWindowHours.
//
// The rule belongs to the item, whoever sells it: a limit on the seller would be
// doubled by a second dealer, and a limit on the door fails because a player can
// stand inside and buy each refill. So both purchase paths ask it.
//
// Each purchase is one Character.Timers entry (gametime.RoundTimer, a saved start
// round and a period), the mechanism that already survives a relog and a restart
// and reads a period in real hours. A Cooldowns tag cannot do it: it holds one
// number per tag and counts down only while its holder is online, so logging out
// would wait out nothing. The window counts in rounds, so it does not run while
// the server is down.
const bloomDealerTimerPrefix = `bloom-dealer-`

// bloomDealerSayEnough is the seller's refusal, in his own voice, with no numbers.
const bloomDealerSayEnough = `say That is enough from me for one day. Come back tomorrow, if you still want to.`

// buyLimited reports whether a per-character buying limit applies to this item.
func buyLimited(itemId int) bool {
	return itemId == bloomWaferItemId
}

// bloomDealerBuysHeld counts the purchases still inside the window, dropping the
// ones past it.
func bloomDealerBuysHeld(c *characters.Character) int {
	held := 0
	for name := range c.Timers {
		// TimerExpired deletes an expired entry, which is safe during a range.
		if strings.HasPrefix(name, bloomDealerTimerPrefix) && !c.TimerExpired(name) {
			held++
		}
	}
	return held
}

// BuyLimitReached reports whether this character may buy no more of the item now.
func BuyLimitReached(c *characters.Character, itemId int) bool {
	return buyLimited(itemId) && bloomDealerBuysHeld(c) >= int(configs.GetBalanceConfig().BloomDealerWafersPerDay)
}

// recordLimitedBuy starts the window for one bought unit.
func recordLimitedBuy(c *characters.Character, itemId int) {
	if !buyLimited(itemId) {
		return
	}
	bloomDealerBuysHeld(c) // frees the slots of what has expired
	period := fmt.Sprintf(`%d real hours`, int(configs.GetBalanceConfig().BloomDealerWindowHours))
	for i := 0; ; i++ {
		if name := bloomDealerTimerPrefix + strconv.Itoa(i); !c.TimerExists(name) {
			c.TimerSet(name, period)
			return
		}
	}
}

// refuseIfBuyLimited has the seller refuse a purchase past the limit, before
// anything moves or is charged, and reports whether it did.
func refuseIfBuyLimited(buyer Actor, shopMob *mobs.Mob, shopUser *users.UserRecord, itemId int) bool {
	if !BuyLimitReached(buyer.GetCharacter(), itemId) {
		return false
	}
	sellerName := ``
	if shopUser != nil {
		sellerName = shopUser.Character.Name
	}
	sendMerchantMessage(buyer, shopMob, shopUser, bloomDealerSayEnough,
		fmt.Sprintf(`<ansi fg="username">%s</ansi> will not sell you any more of that today.`, sellerName))
	return true
}

// BuyLimitNote is the line under a merchant's list for a viewer who has bought
// all they may of something on it, or "". The list still shows the stock, so
// without it a player could read a wafer on the shelf as one they can buy.
func BuyLimitNote(c *characters.Character, stock characters.Shop, sellerName string) string {
	for _, entry := range stock {
		if entry.ItemId > 0 && BuyLimitReached(c, entry.ItemId) {
			itm := items.New(entry.ItemId)
			return fmt.Sprintf(`<ansi fg="mobname">%s</ansi> will not sell you any more of the <ansi fg="itemname">%s</ansi> today.`,
				sellerName, itm.DisplayName())
		}
	}
	return ``
}
