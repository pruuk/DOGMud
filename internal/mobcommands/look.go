package mobcommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Look(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	secretLook := false
	if strings.HasPrefix(rest, "secretly") {
		secretLook = true
		rest = strings.TrimSpace(strings.TrimPrefix(rest, "secretly"))
	}

	isSneaking := mob.Character.IsHidden()

	// trim off some fluff
	if len(rest) > 2 {
		if rest[0:3] == `at ` {
			rest = rest[3:]
		}
	}
	if len(rest) > 3 {
		if rest[0:4] == `the ` {
			rest = rest[4:]
		}
	}

	name := mob.Character.Name

	// The player's sight rules, shared (actions.ResolveLook, slice 5a). A mob
	// is silent on every refusal; its room lines hide names by each
	// observer's sight, as the player's do.
	res := actions.ResolveLook(actions.NewMobActorInRoom(mob, room), rest)
	switch res.Kind {
	case actions.LookBlind, actions.LookTooDark, actions.LookExitTooDark, actions.LookExitLocked, actions.LookExitShapes:
		return true, nil

	case actions.LookRoom:
		if !secretLook && !isSneaking {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is looking around.`, name), []string{name})
			// Make it a "secret looks" now because we don't want another look message sent out by the lookRoom() func
			secretLook = true
		}
		lookRoom(mob, room.RoomId, secretLook || isSneaking)
		return true, nil

	case actions.LookExit:
		if !isSneaking {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> peers toward the %s.`, name, res.ExitName), []string{name})
		}
		lookRoom(mob, res.ExitRoomId, secretLook || isSneaking)
		return true, nil

	case actions.LookCreature:
		if isSneaking {
			return true, nil
		}
		if res.Target.IsPlayer() {
			u := res.Target.(*actions.UserActor).User
			u.SendText(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is looking at you.`, name))
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is looking at <ansi fg="username">%s</ansi>.`, name, u.Character.Name),
				[]string{name, u.Character.Name}, u.UserId)
			return true, nil
		}
		m := res.Target.(*actions.MobActor).Mob
		room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is looking at %s.`, name, m.Character.GetMobName(0).String()),
			[]string{name})
		return true, nil
	}

	// LookOther: the mob's own objects, in its order.
	if lookItem, found := mob.Character.FindInBackpack(rest); found {
		if !isSneaking {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is admiring their <ansi fg="item">%s</ansi>.`, name, lookItem.DisplayName()), []string{name})
		}
		return true, nil
	}
	if lookItem, found := mob.Character.FindOnBody(rest); found {
		if !isSneaking {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is admiring their <ansi fg="item">%s</ansi>.`, name, lookItem.DisplayName()), []string{name})
		}
		return true, nil
	}
	if foundNoun, _ := room.FindNoun(rest); len(foundNoun) > 0 {
		if !isSneaking {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> is examining the <ansi fg="noun">%s</ansi>.`, name, foundNoun), []string{name})
		}
		return true, nil
	}
	if res.PetUserId > 0 {
		if petUser := users.GetByUserId(res.PetUserId); petUser != nil {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is looking at %s.`, name, petUser.Character.Pet.DisplayName()), []string{name})
		}
	}
	return true, nil
}

func lookRoom(mob *mobs.Mob, roomId int, secretLook bool) {

	room := rooms.LoadRoom(roomId)

	if mob == nil || room == nil {
		return
	}

	// Make sure to prepare the room before anyone looks in if this is the first time someone has dealt with it in a while.
	if room.PlayerCt() < 1 {
		room.Prepare(true)
	}

	if !secretLook {
		// Find the exit back
		lookFromName := room.FindExitTo(mob.Character.RoomId)
		if lookFromName == "" {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is looking into the room from somewhere...`, mob.Character.Name),
				[]string{mob.Character.Name},
			)
		} else {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is looking into the room from the <ansi fg="exit">%s</ansi> exit`, mob.Character.Name, lookFromName),
				[]string{mob.Character.Name},
			)
		}
	}

}
