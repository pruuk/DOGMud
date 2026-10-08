package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Appraise(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// Below the faces band you can't make out the goods (lighting plan 5b,
	// #272): appraising is dealing, as list, buy and sell are.
	if actions.ShopSightRefusal(user.Character, room) {
		user.SendText(messaging.CategorySystem, actions.ShopSightRefusalText)
		return true, nil
	}

	merchantMobs := room.GetMobs(rooms.FindMerchant)
	if len(merchantMobs) == 0 {
		user.SendText(messaging.CategorySystem, `You need to be at a merchant to appraise items.`)
		return true, nil
	}

	for _, mobId := range merchantMobs {

		mob := mobs.GetInstance(mobId)
		if mob == nil {
			continue
		}

		if rest == "" {

			mob.Command(`say I will appraise items for 20 gold.`)

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

		// Baubles: a merchant looks one over for free and says what it is
		// and what they would pay (docs/baubles). A 20 gold fee on an object
		// worth 1 to 6 gold would make appraising them pointless.
		if item.IsBauble() {
			appraiseBauble(item, user, room, mob)
			return true, nil
		}

		type identifyDetails struct {
			Item     *items.Item
			ItemSpec *items.ItemSpec
		}

		details := identifyDetails{
			Item:     &item,
			ItemSpec: &itemSpec,
		}

		appraisePrice := 20

		if appraisePrice > user.Character.Gold {

			mob.Command(fmt.Sprintf("say That costs %d gold to appraise, which you don't seem to have.", appraisePrice))

			return true, nil
		}

		user.Character.Gold -= appraisePrice
		mob.Character.Gold += appraisePrice

		events.AddToQueue(events.EquipmentChange{
			UserId:     user.UserId,
			GoldChange: appraisePrice,
		})

		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You give <ansi fg="mobname">%s</ansi> %d gold to appraise <ansi fg="itemname">%s</ansi>.`, mob.Character.Name, appraisePrice, itemSpec.Name))
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> appraises <ansi fg="itemname">%s</ansi>.`, user.Character.Name, itemSpec.Name), user.UserId)

		inspectTxt, _ := templates.Process("descriptions/identify", details, user.UserId)
		user.SendText(messaging.CategorySystem, inspectTxt)

		break
	}

	return true, nil
}

// appraiseBauble is the free appraisal of a bauble: what it is made of, how
// heavy it is, where it turned up, and what this merchant would pay. It never
// mentions whether the bauble was stolen; that is Phase 6's to decide.
func appraiseBauble(item items.Item, user *users.UserRecord, room *rooms.Room, mob *mobs.Mob) {
	rec, ok := baubles.Get(item.Bauble)
	if !ok {
		merchantSay(room, mob, "I can't make head or tail of that.")
		return
	}

	spec := item.GetSpecFor(user.UserId) // the appraisal reaches this player alone
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> turns <ansi fg="itemname">%s</ansi> over in their hands.`, mob.Character.Name, item.DisplayNameFor(user.UserId)))
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> looks over something for <ansi fg="username">%s</ansi>.`, mob.Character.Name, user.Character.Name), user.UserId)

	var b strings.Builder
	fmt.Fprintf(&b, "<ansi fg=\"itemname\">%s</ansi>\r\n", item.DisplayNameFor(user.UserId))
	if spec.Description != `` {
		fmt.Fprintf(&b, "  %s\r\n", spec.Description)
	}
	if material := rec.MaterialFor(user.UserId); material != `` {
		fmt.Fprintf(&b, "  Made of:  %s\r\n", material)
	}
	fmt.Fprintf(&b, "  Weight:   %.1f lb\r\n", spec.Weight)
	fmt.Fprintf(&b, "  Worth:    <ansi fg=\"gold\">%d gold</ansi>\r\n", spec.Value)
	if rec.Region != `` {
		fmt.Fprintf(&b, "  Found in: %s\r\n", rec.Region)
	}
	user.SendText(messaging.CategorySystem, b.String())

	offer := actions.BaubleOfferFrom(item, mob)
	if offer.Price > 0 {
		merchantSay(room, mob, fmt.Sprintf(`I'd give you <ansi fg="gold">%d gold</ansi> for it.`, offer.Price))
	} else {
		merchantSay(room, mob, offer.Refusal)
	}
}
