package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Wake wakes a sleeper in the room by name (owner 2026-11-07): `wake marn`.
// Bare `wake` has nothing left to do: a sleeping player is woken by the
// dispatcher before any command but sit and rest runs (keepsSleep).
func Wake(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	name := strings.TrimSpace(rest)
	if name == `` || room == nil {
		return true, nil
	}
	playerId, mobId := room.FindByNameSeenBy(user.Character, name)
	if mobId > 0 {
		if m := mobs.GetInstance(mobId); m != nil {
			if !m.Character.HasConditionFlag(conditions.Sleeping) {
				user.SendText(messaging.CategoryRefusal, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is not asleep.`, m.Character.Name))
				return true, nil
			}
			m.Character.CancelConditionsWithFlag(conditions.Sleeping)
			mobs.OnSleeperWoken(&m.Character)
			if m.HasShop() {
				relightLantern(m, room) // a keeper's lantern went dark while they slept
			}
			user.SendText(messaging.CategoryMobEmote, fmt.Sprintf(`You wake <ansi fg="mobname">%s</ansi>.`, m.Character.Name))
			room.SendText(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> wakes <ansi fg="mobname">%s</ansi>.`, user.Character.Name, m.Character.Name), user.UserId)
			return true, nil
		}
	}
	if playerId > 0 && playerId != user.UserId {
		if other := users.GetByUserId(playerId); other != nil {
			if !other.Character.HasConditionFlag(conditions.Sleeping) {
				user.SendText(messaging.CategoryRefusal, fmt.Sprintf(`<ansi fg="username">%s</ansi> is not asleep.`, other.Character.Name))
				return true, nil
			}
			other.Character.CancelConditionsWithFlag(conditions.Sleeping)
			other.SendText(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> wakes you.`, user.Character.Name))
			user.SendText(messaging.CategoryMobEmote, fmt.Sprintf(`You wake <ansi fg="username">%s</ansi>.`, other.Character.Name))
			room.SendText(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> wakes <ansi fg="username">%s</ansi>.`, user.Character.Name, other.Character.Name), user.UserId, other.UserId)
			return true, nil
		}
	}
	refusal.NotHereCreature(user)
	return true, nil
}
