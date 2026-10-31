package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Say speaks for a mob through actions.Say, which sends the room line with
// the mob's name hidden by each listener's sight and no deafen filter (NPC
// lines are authored content, owner ruling 6).
func Say(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Don't bother if no players are present
	if room.PlayerCt() < 1 {
		return true, nil
	}

	return sayAs(&actions.MobActor{Mob: mob, Room: room}, rest)
}

// sayAs is Say's body for a given actor, so an ambient say (#317,
// ambient.go) sends the same line with its Skip list.
func sayAs(actor *actions.MobActor, rest string) (bool, error) {
	actions.Say(actor, rest)
	return true, nil
}
