package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Broadcast is an admin announcement (owner 2026-11-07): every online player
// gets it, deafened players included, in the game text and in the web
// client's Broadcasts tab, which only admins and the tips timer post to.
// Player chat is the chat command (its shortcut is `).
func Broadcast(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	rest = strings.TrimSpace(rest)
	if rest == `` {
		user.SendText(messaging.CategoryUsage, `Usage: <ansi fg="command">broadcast</ansi> <message>`)
		return true, nil
	}
	rest = util.EscapeAnsiTags(rest)

	events.AddToQueue(events.Broadcast{
		Text: fmt.Sprintf(`<ansi fg="black-bold">(announcement)</ansi> <ansi fg="username">%s</ansi>: <ansi fg="yellow">%s</ansi>%s`,
			user.Character.Name, rest, term.CRLFStr),
		IsCommunication: true,
		SourceIsMod:     true,
	})

	// One Comm per player, aimed at them, as the tips are: an untargeted
	// broadcast would reach the web client of everyone online, deafened or not,
	// but this keeps the two lanes built the same way.
	for _, u := range users.GetAllActiveUsers() {
		events.AddToQueue(events.Communication{
			SourceUserId: user.UserId,
			TargetUserId: u.UserId,
			CommType:     `broadcast`,
			Name:         `(announcement) ` + user.Character.Name,
			Message:      rest,
		})
	}
	return true, nil
}
