package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Quaff is the mob command for potions; the rules live in actions.Quaff,
// the same body a player quaffs through (#375 phase 7).
func Quaff(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	actions.Quaff(actions.NewMobActorInRoom(mob, room).(actions.DrinkActor), rest)
	return true, nil
}
