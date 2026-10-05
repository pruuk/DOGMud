package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// merchantSay makes the merchant speak a line to the room synchronously. The
// canonical implementation now lives in actions.Sell (unexported); this thin
// helper keeps the offer command — which previously shared usercommands'
// package-local merchantSay — working after the sell lift. See the long-form
// rationale on the async-pipeline pitfall in internal/actions/sell.go.
func merchantSay(room *rooms.Room, mob *mobs.Mob, line string) {
	if mob == nil || room == nil {
		return
	}
	// actions.Say sends the room line itself (sight gates slice 5b).
	actions.Say(&actions.MobActor{Mob: mob, Room: room}, line)
}

func Offer(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// The copy sell would sell: a clean one before a stolen twin.
	item, found := actions.SellFindItemInChar(user.Character, rest)
	if !found {
		user.SendText(messaging.CategorySystem, "You don't have that item.")
		return true, nil
	}

	itemSpec := item.GetSpec()
	if itemSpec.ItemId < 1 {
		return true, nil
	}

	// Stolen goods go to a fence when the room has one, as sell sends them
	// (actions.FenceFor), so offer names that buyer and its cut.
	if item.IsStolen() && !item.IsBauble() {
		if fence, _ := actions.FenceFor(room, item); fence != nil {
			user.Character.CancelConditionsWithFlag(conditions.Hidden)
			merchantSay(room, fence, fmt.Sprintf(`I can give you <ansi fg="gold">%d gold</ansi> for that <ansi fg="itemname">%s</ansi>, and no questions asked.`, actions.FencePrice(itemSpec.Value), item.DisplayName()))
			return true, nil
		}
	}

	for _, mobId := range room.GetMobs(rooms.FindMerchant) {

		mob := mobs.GetInstance(mobId)
		if mob == nil {
			continue
		}

		user.Character.CancelConditionsWithFlag(conditions.Hidden)

		// Baubles are priced from their catalog record (docs/baubles).
		if item.IsBauble() {
			offer := actions.BaubleOfferFrom(item, mob)
			if offer.Price <= 0 {
				merchantSay(room, mob, offer.Refusal)
				continue
			}
			merchantSay(room, mob, fmt.Sprintf(`I can give you <ansi fg="gold">%d gold</ansi> for that <ansi fg="itemname">%s</ansi>.`, offer.Price, item.DisplayName()))
			break
		}

		// Stolen goods (a merchant chest's) with no fence to take them (a
		// fence is quoted above; one robbed of them refuses): an honest
		// merchant refuses them while they are hot here and otherwise quotes
		// them like anything else.
		if item.IsStolen() {
			if actions.IsFence(mob) || actions.StolenGoodsHotHere(item, room) {
				merchantSay(room, mob, actions.StolenGoodsRefusal(mob, item))
				continue
			}
		}

		if item.IsSpecial() {

			merchantSay(room, mob, "I'm afraid I don't buy those.")

			continue
		}

		sellValue := mob.GetSellPrice(item)

		if sellValue <= 0 {

			merchantSay(room, mob, "I'm not interested in that.")

			continue
		}

		merchantSay(room, mob, fmt.Sprintf(`I can give you <ansi fg="gold">%d gold</ansi> for that <ansi fg="itemname">%s</ansi>.`, sellValue, item.DisplayName()))

		break
	}

	return true, nil
}
