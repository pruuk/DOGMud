package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Drink is eat under its other name (#375 phase 7, owner ruling 2026-10-30):
// both take food and cooked drinks (actions.ConsumeFood). Potions are drunk
// with quaff (quaff.go, actions.Quaff).
func Drink(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actions.ConsumeFood(actions.NewUserActorInRoom(user, room), `drink`, rest)
	return true, nil
}
