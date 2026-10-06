package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Who(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// Refused exactly where look is, with look's words: a viewer who makes
	// out nothing here has no roster to read. The untargeted look resolution
	// decides it, so blinded and too dark are told apart as look tells them.
	// At shapes GetDetails lists anonymous figures.
	res := actions.ResolveLook(&actions.UserActor{User: user, Room: room}, ``)
	if line, refused := noSightRefusal(res.Kind); refused {
		user.SendText(messaging.CategorySystem, line)
		return true, nil
	}

	details := rooms.GetDetails(room, user)

	whoTxt, _ := templates.Process("descriptions/who", details, user.UserId)
	user.SendText(messaging.CategorySystem, whoTxt)

	return true, nil
}
