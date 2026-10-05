package usercommands

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Repair handles `repair` (list what is worn and where to mend it) and
// `repair <item>` (wilderness trades gear wear).
func Repair(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actor := &actions.UserActor{User: user, Room: room}
	name := strings.TrimSpace(rest)
	if name == `` {
		actions.ListRepairs(actor)
		return true, nil
	}
	actions.Repair(actor, name)
	return true, nil
}
