package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Read(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if rest == `` {
		refusal.Prompt(user, `read`, refusal.What, `read <item>`)
		return true, nil
	}

	// Check whether the user has an item in their inventory that matches

	foundItemName := ""
	foundItemLongName := ""
	foundItemDescription := ""
	carriedWithoutText := false
	// Search for an exact match first
	if readItem, found := user.Character.FindInBackpack(rest); found {
		iSpec := readItem.GetSpec()
		// Readable items (books/scrolls) carry their text in the blob.
		desc := ""
		if iSpec.Type == items.Readable {
			desc = string(readItem.GetBlob())
		}
		// Fall back to the item's description for anything else. Quest
		// notices/letters/signs are frequently authored as `object`/`holdable`
		// yet the game explicitly tells players to "read the notice on the
		// post" — restricting `read` to the Readable type dead-ends that beat.
		// Any held item that carries text can be read.
		if len(desc) == 0 {
			desc = iSpec.Description
		}
		if len(desc) > 0 {
			foundItemName = readItem.DisplayName()
			foundItemLongName = readItem.DisplayName()
			foundItemDescription = desc
		} else {
			carriedWithoutText = true
		}
	}

	// Nothing carried by that name: read is in the look family (#375 phase
	// 3a, owner ruling 5), so it answers a room feature ("read the
	// inscription") as look would, quest triggers first.
	if !carriedWithoutText && len(foundItemName) == 0 {
		if notifyRoomInteract(user, room, quests.VerbLook, rest, room.NounNamedBy(rest)) {
			return true, nil
		}
		if featureNamedBy(user, room, rest) != `` {
			return Look(rest, user, room, flags)
		}
	}

	isSneaking := user.Character.IsHidden()

	if carriedWithoutText {
		user.SendText(messaging.CategoryRefusal, `There is nothing written on it.`)
	} else if len(foundItemName) == 0 {
		refusal.NotCarrying(user, rest)
	} else {
		user.SendText(messaging.CategorySystem,
			fmt.Sprintf(`You look at <ansi fg="item">%s</ansi>...`, foundItemLongName),
		)

		if !isSneaking {
			room.SendTextVisual(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> looks at their <ansi fg="item">%s</ansi>...`, user.Character.Name, foundItemName),
				user.UserId,
			)
		}

		user.SendText(messaging.CategorySystem, foundItemDescription)
	}

	return true, nil
}
