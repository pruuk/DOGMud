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

	// Below the faces band you can't make out the goods (lighting plan 5b,
	// #272): asking for an offer is dealing, as list, buy and sell are.
	if actions.ShopSightRefusal(user.Character, room) {
		user.SendText(messaging.CategorySystem, actions.ShopSightRefusalText)
		return true, nil
	}

	item, found := user.Character.FindInBackpack(rest)
	if !found {
		user.SendText(messaging.CategorySystem, "You don't have that item.")
		return true, nil
	}

	itemSpec := item.GetSpec()
	if itemSpec.ItemId < 1 {
		return true, nil
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
