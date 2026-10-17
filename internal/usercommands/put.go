package usercommands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Put(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	args := util.SplitButRespectQuotes(strings.ToLower(rest))

	if len(args) < 2 {
		refusal.Prompt(user, "put", refusal.What, "put <item> in <container>")
		return true, nil
	}

	containerName := ``
	nameSearch := ``
	for i := len(args) - 1; i >= 1; i-- {
		if len(nameSearch) > 0 {
			nameSearch = ` ` + nameSearch
		}
		nameSearch = args[i] + nameSearch

		if room.MatchesSealedCrate(nameSearch) {
			user.SendText(messaging.CategoryRefusal, `The shipping crate is sealed. Only the caravan can put things in it.`)
			return true, nil
		}

		containerName = room.FindContainerByName(nameSearch)
		if containerName != `` {
			if c, exists := room.Containers[containerName]; exists && c.Hidden {
				if user == nil || !user.Character.HasDiscovery(room.RoomId, containerName) {
					containerName = ``
				}
			}
		}
		if containerName != `` {
			args = args[:i]
			break
		}
	}

	if containerName == `` {
		refusal.NotHereThing(user)
		return true, nil
	}

	container := room.Containers[containerName]

	if container.Lock.IsLocked() {
		// Named by no container: it was found by looking (spec section 2).
		user.SendText(messaging.CategoryRefusal, ``)
		user.SendText(messaging.CategoryRefusal, `That is locked.`)
		user.SendText(messaging.CategoryRefusal, ``)
		return true, nil
	}

	if len(args) < 1 {
		refusal.Prompt(user, "put", refusal.What, "put <item> in <container>")
		return true, nil
	}

	var item items.Item
	var itemFound bool
	goldAmt := 0

	if len(args) >= 2 && args[1] == `gold` {

		g, _ := strconv.ParseInt(args[0], 10, 32)
		goldAmt = int(g)
		if goldAmt < 0 {
			goldAmt = -1 * goldAmt
		}

	} else {

		item, itemFound = user.Character.FindInBackpack(strings.Join(args, ` `))
		if !itemFound && len(args) > 1 {
			item, itemFound = user.Character.FindInBackpack(args[0])
		}

	}

	if !itemFound && goldAmt == 0 {
		// The item is the player's own, so the refusal echoes it. A trailing
		// "in" or "into" is the joining word before the container, not the item.
		typed := args
		if len(typed) > 1 && (typed[len(typed)-1] == `in` || typed[len(typed)-1] == `into`) {
			typed = typed[:len(typed)-1]
		}
		refusal.NotCarrying(user, strings.Join(typed, ` `))
		return true, nil
	}

	if goldAmt > user.Character.Gold {
		user.SendText(messaging.CategoryRefusal, `You don't have that much gold.`)
		return true, nil
	}

	if goldAmt > 0 {
		user.Character.Gold -= goldAmt

		events.AddToQueue(events.EquipmentChange{
			UserId:     user.UserId,
			GoldChange: goldAmt,
		})

		container.Gold += goldAmt
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You place <ansi fg="gold">%s gold</ansi> into the <ansi fg="container">%s</ansi>`, util.FormatNumber(goldAmt), containerName))
		room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(`<ansi fg="username">%s</ansi> places some <ansi fg="gold">gold</ansi> into the <ansi fg="container">%s</ansi>`, user.Character.Name, containerName), user.UserId)
	}

	if itemFound {

		container.AddItem(item)
		user.Character.RemoveItem(item)

		events.AddToQueue(events.ItemOwnership{
			UserId: user.UserId,
			Item:   item,
			Gained: false,
		})

		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You place your <ansi fg="itemname">%s</ansi> into the <ansi fg="container">%s</ansi>`, item.DisplayName(), containerName))
		room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(`<ansi fg="username">%s</ansi> places their <ansi fg="itemname">%s</ansi> into the <ansi fg="container">%s</ansi>`, user.Character.Name, item.DisplayName(), containerName), user.UserId)

		// Enforce container size limits

		if len(container.Items) > int(configs.GetGamePlayConfig().ContainerSizeMax) {

			randItemToRemove := util.Rand(len(container.Items))
			oopsItem := container.Items[randItemToRemove]

			// get all items that spawn in chests
			for _, spn := range room.SpawnInfo {
				if spn.Container == containerName && oopsItem.ItemId == spn.ItemId {
					// Don't let this one pop out
					oopsItem = item
					break
				}
			}

			container.RemoveItem(oopsItem)
			room.SendTextVisual(messaging.CategoryLoot, fmt.Sprintf(`The <ansi fg="container">%s</ansi> is too full and %s <ansi fg="itemname">%s</ansi> falls out and onto the floor.`, containerName, items.IndefiniteArticle(oopsItem.DisplayName()), oopsItem.DisplayName()))
			room.AddItem(oopsItem, false)
		}
	}

	room.Containers[containerName] = container

	return true, nil
}
