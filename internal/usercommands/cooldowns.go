package usercommands

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Cooldowns(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	cdTxt, _ := templates.Process("character/cooldowns", cooldownsForDisplay(user.Character.GetAllCooldowns()), user.UserId)
	user.SendText(messaging.CategoryScreen, cdTxt)

	return true, nil
}

// cooldownsForDisplay names every timer in player words. The shared
// special-move timer reads "special moves" (#620); any other key reads as its
// last colon-separated part with hyphens and underscores as spaces, so
// "skullduggery:steal" is "steal" and "homunculus-respawn" is "homunculus
// respawn" (#755). Two keys that land on one word keep the longer wait.
func cooldownsForDisplay(all map[string]int) map[string]int {
	shown := make(map[string]int, len(all))
	for key, rounds := range all {
		name := key
		if key == actions.SpecialMoveCooldownTag {
			name = actions.SpecialMoveCooldownLabel
		} else {
			if i := strings.LastIndex(key, `:`); i >= 0 {
				name = key[i+1:]
			}
			name = strings.NewReplacer(`-`, ` `, `_`, ` `).Replace(name)
		}
		if rounds > shown[name] {
			shown[name] = rounds
		}
	}
	return shown
}
