package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Quaff is the player command for potions (#375 phase 7, owner ruling
// 2026-10-30); the rules live in actions.Quaff (drink path unification),
// shared with mobs and the AI companion. Food and cooked drinks go through
// eat and drink (eat.go).
func Quaff(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actions.Quaff(actions.NewUserActorInRoom(user, room).(actions.DrinkActor), rest)
	return true, nil
}
