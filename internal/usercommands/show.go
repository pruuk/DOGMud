package usercommands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func Show(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	rest = util.StripPrepositions(rest)

	args := util.SplitButRespectQuotes(strings.ToLower(rest))

	if len(args) < 2 {
		refusal.Prompt(user, "show", refusal.What, "show <item> <name>")
		return true, nil
	}

	var showItem items.Item = items.Item{}
	var found bool = false

	var targetName string = args[len(args)-1]
	args = args[:len(args)-1]
	var objectName string = strings.Join(args, " ")

	// Check whether the user has an item in their inventory that matches
	showItem, found = user.Character.FindInBackpack(objectName)

	if !found {
		refusal.NotCarrying(user, objectName)
		return true, nil
	}

	// #454: a typed name resolves only at full sight.
	name, aimRefusal := actions.AimBySight(user.Character, user.UserId, room, targetName, `show `+objectName)
	if aimRefusal != `` {
		user.SendText(messaging.CategoryRefusal, aimRefusal)
		return true, nil
	}
	targetName = name
	showerSight := messaging.ParticipantSight(user.Character, room)

	target, err := actions.ResolveTargetActor(room, targetName, actions.ResolveTargetOptions{Viewer: user.Character})
	if err != nil {
		refusal.NotHereCreature(user)
		return true, nil
	}

	user.Character.CancelConditionsWithFlag(conditions.Hidden)

	if showItem.ItemId == 0 {
		refusal.Failed(user, "", errors.New("show: found item has no id"), "userId", user.UserId)
		return true, nil
	}

	if target.IsPlayer() {

		targetUser := target.(*actions.UserActor).User

		// Tell the shower
		user.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`You show the <ansi fg="item">%s</ansi> to <ansi fg="username">%s</ansi>.`, showItem.DisplayName(), targetUser.Character.Name),
			[]string{targetUser.Character.Name}, showerSight),
		)

		// Tell the Showee, at the showee's own sight.
		targetUser.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`<ansi fg="username">%s</ansi> shows you their <ansi fg="item">%s</ansi>.`, user.Character.Name, showItem.DisplayName()),
			[]string{user.Character.Name}, messaging.ParticipantSight(targetUser.Character, room)),
		)

		targetUser.SendText(messaging.CategorySystem,
			"\n"+showItem.GetLongDescription()+"\n",
		)

		// Tell the rest of the room
		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> shows their <ansi fg="item">%s</ansi> to <ansi fg="username">%s</ansi>.`, user.Character.Name, showItem.DisplayName(), targetUser.Character.Name),
			targetUser.UserId,
			user.UserId)

	} else {

		targetMob := target.(*actions.MobActor).Mob

		user.SendText(messaging.CategorySystem, messaging.HideNames(
			fmt.Sprintf(`You show the <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.`, showItem.DisplayName(), targetMob.Character.Name),
			[]string{targetMob.Character.Name}, showerSight),
		)

		room.SendTextVisual(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="username">%s</ansi> shows their <ansi fg="item">%s</ansi> to <ansi fg="mobname">%s</ansi>.`, user.Character.Name, showItem.DisplayName(), targetMob.Character.Name),
			user.UserId,
		)
	}

	return true, nil
}
