package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Eat is the food command. eat and drink are one command (#375 phase 7, owner
// ruling 2026-10-30); the rules live in actions.ConsumeFood, shared with mobs.
// Potions are drunk with quaff (quaff.go, actions.Quaff).
func Eat(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	actions.ConsumeFood(actions.NewUserActorInRoom(user, room), `eat`, rest)
	return true, nil
}
