package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/refusal"
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
		user.SendText(messaging.CategoryRefusal, actions.ShopSightRefusalText)
		return true, nil
	}

	if rest == "" {
		refusal.Prompt(user, "offer", refusal.What, "offer <item>")
		return true, nil
	}

	// Shut for the night, as for list, buy and sell: a sleeping keeper
	// quotes nothing sell would then refuse.
	if asleep := actions.ShopClosedForSleep(room); asleep != nil {
		actions.RefuseMobIfAsleep(asleep, user)
		return true, nil
	}

	item, found := user.Character.FindInBackpack(rest)
	if !found {
		refusal.NotCarrying(user, rest)
		return true, nil
	}

	if item.GetSpec().ItemId < 1 {
		return true, nil
	}

	// The merchant sell would deal with, and the quote sell would pay
	// (actions.QuoteSale), so offer never names a price sell then refuses.
	mob, shopInv := actions.SaleMerchant(room, item, true)
	if mob == nil {
		user.SendText(messaging.CategoryRefusal, "There's no merchant here.")
		return true, nil
	}

	quote := actions.QuoteSale(user.Character, room, mob, shopInv, item, true)
	if quote.Tell != "" {
		user.SendText(messaging.CategoryRefusal, quote.Tell)
		return true, nil
	}

	user.Character.CancelConditionsWithFlag(conditions.Hidden)

	if quote.Price <= 0 {
		merchantSay(room, mob, quote.Say)
		return true, nil
	}

	merchantSay(room, mob, fmt.Sprintf(`I can give you <ansi fg="gold">%d gold</ansi> for that <ansi fg="itemname">%s</ansi>.`, quote.Price, item.DisplayName()))

	return true, nil
}
