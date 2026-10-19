package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Cooldowns(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	cdTxt, _ := templates.Process("character/cooldowns", cooldownsForDisplay(user.Character.GetAllCooldowns()), user.UserId)
	user.SendText(messaging.CategorySystem, cdTxt)

	return true, nil
}

// cooldownsForDisplay renames the shared special-move timer to its player
// word, so the list reads "special moves" rather than the raw "special-move"
// key (#620). all is a copy (GetAllCooldowns) and is changed in place.
func cooldownsForDisplay(all map[string]int) map[string]int {
	if rounds, ok := all[actions.SpecialMoveCooldownTag]; ok {
		delete(all, actions.SpecialMoveCooldownTag)
		all[actions.SpecialMoveCooldownLabel] = rounds
	}
	return all
}
