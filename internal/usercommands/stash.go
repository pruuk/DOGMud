package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Stash(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if rest == `` {
		refusal.Prompt(user, `stash`, refusal.What, `stash <item>`)
		return true, nil
	}

	// Check whether the user has an item in their inventory that matches
	// The let-go lookup: an ordinary copy before quest gear (#940).
	matchItem, found := user.Character.FindInBackpackToLetGo(rest)

	if !found {
		refusal.NotCarrying(user, rest)
	} else if actions.VanishQuestGiven(user.Character, user.UserId, 0, matchItem, nil) {
		// Quest gear fades instead of being stashed (#940). Quietly: a stash
		// shows the room nothing about the item.
	} else {
		// Swap the item location

		matchItem.StashedBy = user.UserId

		room.AddItem(matchItem, true)
		user.Character.RemoveItem(matchItem)

		events.AddToQueue(events.ItemOwnership{
			UserId: user.UserId,
			Item:   matchItem,
			Gained: false,
		})

		isSneaking := user.Character.IsHidden()

		user.SendText(messaging.CategorySystem,
			fmt.Sprintf(`You stash the <ansi fg="itemname">%s</ansi>. To get it back, try <ansi fg="command">get %s from stash</ansi>`, matchItem.DisplayName(), matchItem.DisplayName()))

		if !isSneaking {
			room.SendTextVisual(messaging.CategoryLoot,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> is attempting to look unsuspicious.`, user.Character.Name),
				user.UserId)
		}

	}

	return true, nil
}
