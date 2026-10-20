package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/colorpatterns"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"

	"github.com/GoMudEngine/GoMud/internal/users"
)

/*
* Role Permissions:
* paz 				(All)
 */
func Paz(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	beamOfLight := colorpatterns.ApplyColorPattern(`beam of light`, `rainbow`)

	if rest != `` {

		target, err := actions.ResolveTargetActor(room, rest)
		if err == actions.ErrTargetVanished {
			user.SendText(messaging.CategoryAdmin, "Paz target not found.")
			return true, nil
		}
		if err == nil {
			if !target.IsPlayer() {
				mob := target.(*actions.MobActor).Mob
				user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`You illuminate <ansi fg="mobname">%s</ansi> with a %s!`, mob.Character.Name, beamOfLight))
				room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> illuminates <ansi fg="mobname">%s</ansi> with a %s!`, user.Character.Name, mob.Character.Name, beamOfLight), user.UserId)

				mob.Character.SetPoolFraction(characters.PoolHealth, 1.0)
				mob.Character.SetPoolFraction(characters.PoolConviction, 1.0)

				return true, nil
			}

			u := target.(*actions.UserActor).User
			user.SendText(messaging.CategoryAdmin, fmt.Sprintf(`You illuminate <ansi fg="username">%s</ansi> with a %s!`, u.Character.Name, beamOfLight))
			room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> illuminates <ansi fg="username">%s</ansi> with a %s!`, user.Character.Name, u.Character.Name, beamOfLight), user.UserId, u.UserId)
			u.SendText(messaging.CategoryMobEmote, messaging.CapitalizeStart(fmt.Sprintf(`<ansi fg="username">%s</ansi> illuminates you with a %s!`, user.Character.Name, beamOfLight)))

			u.Character.SetPoolFraction(characters.PoolHealth, 1.0)
			u.Character.SetPoolFraction(characters.PoolConviction, 1.0)

			events.AddToQueue(events.CharacterVitalsChanged{UserId: u.UserId})

			return true, nil
		}
		// err == ErrTargetNotFound → fall through to self-paz path

	}

	user.SendText(messaging.CategoryAdmin, `You paz yourself with a `+beamOfLight+`!`)
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> illuminates <ansi fg="username">%s</ansi> with a %s!`, user.Character.Name, user.Character.Name, beamOfLight), user.UserId)

	if user.Character.Health != user.Character.PoolCeiling(characters.PoolHealth) || user.Character.Conviction != user.Character.PoolCeiling(characters.PoolConviction) {
		user.Character.SetPoolFraction(characters.PoolHealth, 1.0)
		user.Character.SetPoolFraction(characters.PoolConviction, 1.0)

		events.AddToQueue(events.CharacterVitalsChanged{UserId: user.UserId})
	}

	return true, nil
}
