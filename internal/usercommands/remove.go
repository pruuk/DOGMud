package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/lightnotice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Remove(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	// "remove ring from chest" takes from a container, the way get does. Only
	// when the words after "from" name a container here; anything else is
	// taking off equipment, as always.
	if i := strings.LastIndex(strings.ToLower(rest), ` from `); i > 0 {
		if room.FindContainerByName(strings.TrimSpace(rest[i+len(` from `):])) != `` {
			return Get(rest, user, room, flags)
		}
	}


	actor := &actions.UserActor{User: user, Room: room}

	// Snapshot reservation BEFORE anything leaves the body, so the disclosure
	// below can tell the player when taking that off gave capacity back.
	// Taken up here so the `all` branch is covered by the same snapshot.
	beforeReservation := user.Character.ReservationTotals()

	cursedLine := func(it items.Item) string {
		return fmt.Sprintf(`You can't seem to remove your <ansi fg="item">%s</ansi>... It's <ansi fg="red-bold">CURSED!</ansi>`, it.DisplayName())
	}

	if rest == "all" {
		// The busy, curse and per-item rules live in actions.RemoveAllEquipment
		// (slice 5a), which fires one EquipmentChange per item.
		res := actions.RemoveAllEquipment(actor)
		if res.Busy {
			user.SendText(messaging.CategorySystem, busyRefusalText(`change equipment`))
			return true, nil
		}
		for _, it := range res.Cursed {
			user.SendText(messaging.CategorySystem, cursedLine(it))
		}
		sendReservationReturnDisclosure(user, beforeReservation)
		for _, item := range res.Removed {
			if item.GetSpec().Type == items.Light {
				lightnotice.Check(user, lightnotice.TriggerCommand)
				break
			}
		}
		return true, nil
	}

	result := actions.RemoveEquipment(actor, rest)
	switch {
	case result.Busy:
		user.SendText(messaging.CategorySystem, busyRefusalText(`change equipment`))
	case !result.Found:
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You don't appear to be using a "%s".`, rest))
	case result.Cursed:
		user.SendText(messaging.CategorySystem, cursedLine(result.Item))
	default:
		if result.CursedOverridden {
			user.SendText(messaging.CategorySystem,
				`It's <ansi fg="red-bold">CURSED</ansi> but luckily your <ansi fg="skillname">enchant</ansi> skill level allows you to remove it.`,
			)
		}
		if result.Removed {
			user.SendText(messaging.CategorySystem,
				fmt.Sprintf(`You remove your <ansi fg="item">%s</ansi> and return it to your backpack.`, result.Item.DisplayName()),
			)
			room.SendTextVisual(messaging.CategoryEquipment,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> removes their <ansi fg="item">%s</ansi> and stores it away.`, user.Character.Name, result.Item.DisplayName()),
				user.UserId,
			)
			sendReservationReturnDisclosure(user, beforeReservation)
			// Taking off a light changes the band at once; the notice rides
			// this command rather than waiting for the next one.
			if result.Item.GetSpec().Type == items.Light {
				lightnotice.Check(user, lightnotice.TriggerCommand)
			}
		} else {
			user.SendText(messaging.CategorySystem,
				fmt.Sprintf(`You can't seem to remove your <ansi fg="item">%s</ansi>.`, result.Item.DisplayName()),
			)
		}
	}
	return true, nil
}
