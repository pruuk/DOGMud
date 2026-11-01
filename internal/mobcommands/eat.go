package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Eat is the mob food command; the rules live in actions.ConsumeFood, the same
// body a player eats through (#375 phase 7).
func Eat(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	actions.ConsumeFood(actions.NewMobActorInRoom(mob, room), `eat`, rest)
	return true, nil
}
