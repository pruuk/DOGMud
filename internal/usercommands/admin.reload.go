package usercommands

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/language"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

/*
* Role Permissions:
* reload <items|biomes|translations|mapcache>   (Admin)
 */

// Reload reloads a server data file in place. This is the ADMIN command, and
// now the only meaning of the word.
//
// It used to share its name, its file and its dispatch with a player-facing
// ranged-weapon reload, separated only by a role check: an admin typing bare
// `reload` got the weapon behaviour, and everyone else got it too. That player
// command is gone, because firing chambers its own next round, so `reload`
// means exactly one thing again.
func Reload(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if !user.HasRolePermission("reload", true) {
		// A player who types `reload` is almost certainly holding a ranged
		// weapon and expecting the retired command. Point them at the verb that
		// replaced it rather than at a permissions error.
		user.SendText(messaging.CategorySystem,
			`You do not need to reload. Firing readies the next round by itself. See <ansi fg="command">help fire</ansi>.`)
		return true, nil
	}

	switch strings.ToLower(rest) {
	case `items`:
		items.LoadDataFiles()
		user.SendText(messaging.CategorySystem, `Items reloaded.`)
		return true, nil
	case `biomes`:
		rooms.LoadBiomeDataFiles()
		user.SendText(messaging.CategorySystem, `Biomes reloaded.`)
		return true, nil
	case `translations`:
		ok := language.ReloadTranslation()
		if !ok {
			user.SendText(messaging.CategorySystem, `Translations reload failed.`)
		} else {
			user.SendText(messaging.CategorySystem, `Translations reloaded.`)
		}
		return true, nil
	case `mapcache`:
		mapper.ClearCache()
		user.SendText(messaging.CategorySystem, `Mapper cache cleared. Next 'map' command will rebuild from current room data.`)
		return true, nil
	case `help`:
		infoOutput, _ := templates.Process("admincommands/help/command.reload", nil, user.UserId)
		user.SendText(messaging.CategorySystem, infoOutput)
		return true, nil
	default:
		user.SendText(messaging.CategorySystem, `Unknown reload command. See <ansi fg="command">reload help</ansi>.`)
		return true, nil
	}
}
