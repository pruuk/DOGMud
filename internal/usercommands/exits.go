package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Exits(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	details := rooms.GetDetails(room, user)

	// The exits line lists nothing at no sight (rooms.KnownExits, #781), and
	// "Exits: None" would not be true: answer as look does. ResolveLook only
	// words it (dark or blinded); ExitsDark decides.
	if details.ExitsDark {
		if line, refused := actions.NoSightLine(actions.ResolveLook(actions.NewUserActorInRoom(user, room), ``).Kind); refused {
			user.SendText(messaging.CategoryRefusal, line)
			return true, nil
		}
	}

	exitTxt, _ := templates.Process("descriptions/exits", details, user.UserId)
	user.SendText(messaging.CategoryRoomDescription, exitTxt)

	return true, nil
}
