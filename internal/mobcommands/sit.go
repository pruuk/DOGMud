package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Sit is the mob sit verb, registered as both `sit` and `rest` (#637).
// Delegates to actions.Sit, shared with players; a refusal is silent for a
// mob.
func Sit(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	actions.Sit(actions.NewMobActorInRoom(mob, room))
	return true, nil
}
