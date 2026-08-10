package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/moderation"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Unban lifts an account ban (by name) or an IP ban.
// Usage: unban <name>  |  unban ip <ip>
func Unban(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		user.SendText(messaging.CategorySystem, `<ansi fg="yellow">Usage: unban <name>  |  unban ip <ip></ansi>`)
		return true, nil
	}

	if strings.ToLower(fields[0]) == "ip" && len(fields) >= 2 {
		// An unban that did not persist must not be reported as done: the ban
		// returns at the next restart while staff believe it was lifted.
		if err := moderation.UnbanIP(fields[1]); err != nil {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="alert-5">Unban FAILED to save:</ansi> %s is STILL banned. %s`, fields[1], err))
			return true, nil
		}
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="green">IP unbanned:</ansi> %s`, fields[1]))
		return true, nil
	}

	if err := moderation.Unban(fields[0]); err != nil {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="alert-5">Unban FAILED to save:</ansi> <ansi fg="username">%s</ansi> is STILL banned. %s`, fields[0], err))
		return true, nil
	}
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="green"><ansi fg="username">%s</ansi> has been unbanned.</ansi>`, fields[0]))
	return true, nil
}
