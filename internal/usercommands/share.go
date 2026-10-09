package usercommands

import (
	"math"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Share(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	party := parties.Get(user.UserId)
	if party == nil {
		user.SendText(messaging.CategorySystem, "You can only share in a party.")
		return true, nil
	}

	args := util.SplitButRespectQuotes(strings.ToLower(rest))

	if len(args) == 2 && strings.ToLower(args[1]) == "gold" {

		giveGoldAmount := 0

		if args[0] == "all" {
			giveGoldAmount = user.Character.Gold
		} else {
			giveGoldAmount, _ = strconv.Atoi(args[0])
		}

		if giveGoldAmount < 0 {
			user.SendText(messaging.CategorySystem, "You can't share a negative amount of gold.")
			return true, nil
		}

		if giveGoldAmount > user.Character.Gold {
			user.SendText(messaging.CategorySystem, "You don't have that much gold to share.")
			return true, nil
		}

		partyMembersInRoom := []int{user.UserId} // make sure party leader gets first share
		for _, uid := range room.GetPlayers(rooms.FindAll) {
			if uid == user.UserId {
				continue
			}
			if party.IsMember(uid) {
				partyMembersInRoom = append(partyMembersInRoom, uid)
			}
		}

		split := int(math.Floor(float64(giveGoldAmount) / float64(len(partyMembersInRoom))))
		leftOver := giveGoldAmount - split*len(partyMembersInRoom)

		// Each share is paid directly, not typed as `give N gold to @uid`:
		// the give command's sight gate refuses an id form in the dark, and
		// party membership is already known, so paying a member tells no one
		// who is there (#454). giveGoldToUser hides each name at its reader's
		// sight.
		shares := map[int]int{}
		for _, uid := range partyMembersInRoom {
			shares[uid] += split
		}
		if leftOver > 0 {
			shares[partyMembersInRoom[util.Rand(len(partyMembersInRoom))]] += leftOver
		}

		for _, uid := range partyMembersInRoom {
			if shares[uid] <= 0 {
				continue
			}
			if member := users.GetByUserId(uid); member != nil {
				giveGoldToUser(user, member, room, shares[uid])
			}
		}

	} else {

		user.SendText(messaging.CategorySystem, `You can share gold by typing <ansi fg="command">share [amt] gold</ansi>?`)
	}

	return true, nil
}
