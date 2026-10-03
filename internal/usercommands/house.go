package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/housing"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// House manages player housing access from anywhere:
//
//	house                  your lodging, its guests, and lodgings you may visit
//	house guests           the same
//	house revoke <name>    take a guest's access away (they are put outside if in)
//	house leave <owner>    give up your own access to someone else's lodging
func House(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	send := func(s string) { user.SendText(messaging.CategorySystem, util.SplitStringNL(s, 80)) }

	args := strings.Fields(rest)
	sub := ``
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}
	target := ``
	if len(args) > 1 {
		target = strings.Join(args[1:], ` `)
	}

	switch sub {
	case ``, `info`, `guests`, `list`:
		houseInfo(user, send)

	case `revoke`, `kick`, `remove`:
		if target == `` {
			send(`Revoke whom? Type <ansi fg="command">house revoke [name]</ansi>. <ansi fg="command">house guests</ansi> lists them.`)
			return true, nil
		}
		g, where, err := housing.Revoke(user.UserId, target)
		if err != nil {
			send(capitaliseFirst(err.Error()) + `.`)
			return true, nil
		}
		send(fmt.Sprintf(`The lock of your lodging in %s no longer knows <ansi fg="username">%s</ansi>. If they were inside, they are outside now.`, strings.Join(where, ` and in `), g.Name))

	case `leave`:
		if target == `` {
			send(`Whose lodging do you want to give up? Type <ansi fg="command">house leave [owner]</ansi>.`)
			return true, nil
		}
		h, where, err := housing.Leave(user.UserId, target)
		if err != nil {
			send(capitaliseFirst(err.Error()) + `.`)
			return true, nil
		}
		send(fmt.Sprintf(`The lock of <ansi fg="username">%s</ansi>'s lodging in %s will not know your palm any more.`, h.OwnerName, strings.Join(where, ` and in `)))

	default:
		send(`Usage: <ansi fg="command">house</ansi>, <ansi fg="command">house revoke [name]</ansi> or <ansi fg="command">house leave [owner]</ansi>. Type <ansi fg="command">help house</ansi> for more.`)
	}
	return true, nil
}

func houseInfo(user *users.UserRecord, send func(string)) {
	owned := housing.HousesOwnedBy(user.UserId)
	visiting := housing.GuestOf(user.UserId)

	if len(owned) == 0 && len(visiting) == 0 {
		send(`You have no lodging, and nobody has given you a key to theirs.`)
		for _, b := range housing.AllBuildings() {
			send(fmt.Sprintf(`%s lets rooms in %s: %s. Type <ansi fg="command">list</ansi> beside them.`, housing.LandlordName(b), b.Name, b.Location))
		}
		return
	}
	for _, h := range owned {
		b, _ := housing.GetBuilding(h.BuildingId)
		rooms := `one room`
		if n := len(h.RoomIds); n != 1 {
			rooms = fmt.Sprintf(`%d rooms`, n)
		}
		send(fmt.Sprintf(`Your lodging in %s (%s) has %s.`, b.Name, b.Location, rooms))
		if len(h.Guests) == 0 {
			send(fmt.Sprintf(`Nobody else can come in. Buy a guest key from the landlord to let a friend in. Up to %d guests.`, b.MaxGuests))
			continue
		}
		names := make([]string, 0, len(h.Guests))
		for _, g := range h.Guests {
			names = append(names, `<ansi fg="username">`+g.Name+`</ansi>`)
		}
		send(fmt.Sprintf(`Guests who can come in (%d of %d): %s. Type <ansi fg="command">house revoke [name]</ansi> to take a key back.`, len(h.Guests), b.MaxGuests, strings.Join(names, `, `)))
	}
	// Guest access, one line per building, since each door only knows its
	// own lodgings.
	for _, b := range housing.AllBuildings() {
		names := []string{}
		for _, h := range visiting {
			if h.BuildingId == b.BuildingId {
				names = append(names, `<ansi fg="username">`+h.OwnerName+`</ansi>`)
			}
		}
		if len(names) == 0 {
			continue
		}
		send(fmt.Sprintf(`In %s you may visit the lodgings of: %s. Use the door in %s, or type <ansi fg="command">visit [name]</ansi> there. <ansi fg="command">house leave [name]</ansi> gives one up.`, b.Name, strings.Join(names, `, `), b.Location))
	}
}

// Visit goes through a housing door to one lodging by name ("home" or an
// owner the player is a guest of), skipping the door's menu.
func Visit(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	b, atDoor := housing.BuildingForDoor(room.RoomId)
	if !atDoor {
		user.SendText(messaging.CategorySystem, `There is no lodging-house door here to visit through.`)
		return true, nil
	}
	name := strings.ToLower(strings.TrimSpace(rest))
	if name == `` {
		// No name: the door itself asks, or goes straight in.
		return Go(b.DoorExit, user, room, flags)
	}
	route, _ := rooms.RouteExit(user.UserId, room.RoomId, b.DoorExit)
	pick := 0
	for _, c := range route.Choices {
		if strings.ToLower(c.Label) == name {
			pick = c.RoomId
			break
		}
	}
	if pick == 0 {
		var matches []int
		for _, c := range route.Choices {
			if strings.HasPrefix(strings.ToLower(c.Label), name) {
				matches = append(matches, c.RoomId)
			}
		}
		if len(matches) == 1 {
			pick = matches[0]
		}
	}
	if pick == 0 {
		user.SendText(messaging.CategorySystem, util.SplitStringNL(fmt.Sprintf(`The lock does not know you for anyone called %q. Type <ansi fg="command">house</ansi> to see whose lodgings you may visit.`, strings.TrimSpace(rest)), 80))
		return true, nil
	}
	user.SetTempData(routedPickKey, pick)
	handled, err := Go(b.DoorExit, user, room, flags)
	user.SetTempData(routedPickKey, nil)
	return handled, err
}

func capitaliseFirst(s string) string {
	if s == `` {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
