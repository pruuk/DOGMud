package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// characterOf is user's character, or nil for a nil user, so a command can
// hand it to a rooms helper that takes a nil character as "no viewer"
// (Room.ContainerVisibleTo, #442).
func characterOf(user *users.UserRecord) *characters.Character {
	if user == nil {
		return nil
	}
	return user.Character
}
