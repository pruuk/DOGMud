package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Sit is the player sit verb (#637); `rest` reaches it through the
// command-aliases in keywords.yaml. Delegates to actions.Sit, shared with
// mobs, which moves the Position machine to Sitting and narrates it.
func Sit(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	// The busy refusal lives in actions.Sit, shared with mobs.
	actions.Sit(actions.NewUserActorInRoom(user, room))
	return true, nil
}
